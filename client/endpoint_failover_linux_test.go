//go:build linux

package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func newTestTLSConfigs(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "libovsdb-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(certificateDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(certificate)

	serverConfig := &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{certificateDER}, PrivateKey: privateKey}},
		MinVersion:   tls.VersionTLS12,
	}
	clientConfig := &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS12,
	}
	return serverConfig, clientConfig
}

func proxyConnections(left, right net.Conn) {
	defer left.Close()
	defer right.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(left, right)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(right, left)
		done <- struct{}{}
	}()
	<-done
}

func newTCPTestServer(t *testing.T, connectCounter *int32, useTLS bool, serverTLSConfig *tls.Config) int {
	t.Helper()

	var defSchema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &defSchema))
	ovsdbServer, socket := newOVSDBServer(t, defDB, defSchema)
	ovsdbServer.OnConnect(func(_ *rpc2.Client) {
		atomic.AddInt32(connectCounter, 1)
	})

	tcpListener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	port := tcpListener.Addr().(*net.TCPAddr).Port
	var listener net.Listener = tcpListener
	if useTLS {
		listener = tls.NewListener(tcpListener, serverTLSConfig)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				return
			}
			serverConn, err := net.Dial("unix", socket)
			if err != nil {
				_ = clientConn.Close()
				t.Errorf("dial OVSDB server: %v", err)
				return
			}
			go proxyConnections(clientConn, serverConn)
		}
	}()

	return port
}

func requireNetworkBlackholeTest(t *testing.T) {
	t.Helper()
	if os.Getenv("LIBOVSDB_TEST_NETWORK_BLACKHOLE") != "1" {
		t.Skip("set LIBOVSDB_TEST_NETWORK_BLACKHOLE=1 in an isolated network namespace")
	}
	currentNamespace, err := os.Readlink("/proc/self/ns/net")
	require.NoError(t, err)
	initNamespace, err := os.Readlink("/proc/1/ns/net")
	require.NoError(t, err)
	require.NotEqual(t, initNamespace, currentNamespace, "network blackhole tests require an isolated network namespace")
}

func setOutputBlackhole(t *testing.T, port int) {
	t.Helper()

	args := []string{"--wait", "-I", "OUTPUT", "-o", "lo", "-d", "127.0.0.1/32", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "DROP"}
	output, err := exec.Command("iptables", args...).CombinedOutput()
	require.NoErrorf(t, err, "iptables: %s", output)
	t.Cleanup(func() {
		args[1] = "-D"
		if output, err := exec.Command("iptables", args...).CombinedOutput(); err != nil {
			t.Errorf("remove iptables rule: %v: %s", err, output)
		}
	})
}

func newTCPConn(t *testing.T) *net.TCPConn {
	t.Helper()

	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan *net.TCPConn, 1)
	go func() {
		conn, err := listener.AcceptTCP()
		if err == nil {
			accepted <- conn
		}
	}()
	conn, err := net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	select {
	case serverConn := <-accepted:
		t.Cleanup(func() { _ = serverConn.Close() })
	case <-time.After(time.Second):
		t.Fatal("server did not accept TCP connection")
	}
	return conn
}

func getTCPUserTimeout(t *testing.T, conn *net.TCPConn) int {
	t.Helper()

	rawConn, err := conn.SyscallConn()
	require.NoError(t, err)
	var value int
	var socketErr error
	require.NoError(t, rawConn.Control(func(fd uintptr) {
		value, socketErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT)
	}))
	require.NoError(t, socketErr)
	return value
}

func TestSetTCPUserTimeout(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(net.Conn) net.Conn
	}{
		{name: "TCP", wrap: func(conn net.Conn) net.Conn { return conn }},
		{name: "TLS", wrap: func(conn net.Conn) net.Conn {
			return tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tcpConn := newTCPConn(t)
			require.NoError(t, setTCPUserTimeout(tc.wrap(tcpConn), 250*time.Millisecond))
			require.Equal(t, 250, getTCPUserTimeout(t, tcpConn))
		})
	}
}

func failoverClient(t *testing.T, useTLS bool, clientOptions ...Option) (Client, string, string, *int32, *int32) {
	t.Helper()

	connected1 := new(int32)
	connected2 := new(int32)
	var serverTLSConfig, clientTLSConfig *tls.Config
	if useTLS {
		serverTLSConfig, clientTLSConfig = newTestTLSConfigs(t)
	}
	port1 := newTCPTestServer(t, connected1, useTLS, serverTLSConfig)
	port2 := newTCPTestServer(t, connected2, useTLS, serverTLSConfig)
	scheme := TCP
	if useTLS {
		scheme = SSL
		clientOptions = append(clientOptions, WithTLSConfig(clientTLSConfig))
	}
	endpoint1 := fmt.Sprintf("%s:127.0.0.1:%d", scheme, port1)
	endpoint2 := fmt.Sprintf("%s:127.0.0.1:%d", scheme, port2)
	clientOptions = append(clientOptions, WithEndpoint(endpoint1), WithEndpoint(endpoint2))
	client, err := newOVSDBClient(defDB, clientOptions...)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, client.Connect(ctx))
	t.Cleanup(client.Close)
	require.Equal(t, int32(1), atomic.LoadInt32(connected1))
	require.Zero(t, atomic.LoadInt32(connected2))
	return client, endpoint1, endpoint2, connected1, connected2
}

func requireUsableEndpoint(t *testing.T, client Client, endpoint string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return client.CurrentEndpoint() == endpoint && client.Connected()
	}, 5*time.Second, 20*time.Millisecond, "client did not switch to endpoint %s", endpoint)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, client.Echo(ctx))
}

func TestClientRotatesActiveEndpointAfterLiveUpdate(t *testing.T) {
	client, endpoint1, endpoint2, _, _ := failoverClient(t, false,
		WithReconnect(400*time.Millisecond, &backoff.ZeroBackOff{}))
	client.UpdateEndpoints([]string{endpoint1, endpoint2})
	client.Disconnect()
	requireUsableEndpoint(t, client, endpoint2)
}

func TestClientFailsOverWhenActiveEndpointStopsAcknowledgingTraffic(t *testing.T) {
	requireNetworkBlackholeTest(t)

	for _, tc := range []struct {
		name   string
		useTLS bool
	}{{name: "TCP"}, {name: "TLS", useTLS: true}} {
		t.Run(tc.name, func(t *testing.T) {
			client, endpoint1, endpoint2, _, connected2 := failoverClient(t, tc.useTLS,
				WithReconnect(200*time.Millisecond, &backoff.ZeroBackOff{}))
			port, err := strconv.Atoi(endpoint1[strings.LastIndexByte(endpoint1, ':')+1:])
			require.NoError(t, err)
			setOutputBlackhole(t, port)

			echoDone := make(chan error, 1)
			go func() {
				echoDone <- client.Echo(context.Background())
			}()

			requireUsableEndpoint(t, client, endpoint2)
			require.Positive(t, atomic.LoadInt32(connected2))
			select {
			case <-echoDone:
			case <-time.After(time.Second):
				t.Fatal("stalled Echo did not return after failover")
			}
		})
	}
}

func TestClientFailsOverAfterActiveEndpointBlackhole(t *testing.T) {
	requireNetworkBlackholeTest(t)

	for _, tc := range []struct {
		name   string
		useTLS bool
	}{{name: "TCP"}, {name: "TLS", useTLS: true}} {
		t.Run(tc.name, func(t *testing.T) {
			client, endpoint1, endpoint2, _, _ := failoverClient(t, tc.useTLS,
				WithInactivityCheck(500*time.Millisecond, 200*time.Millisecond, &backoff.ZeroBackOff{}))
			port, err := strconv.Atoi(endpoint1[strings.LastIndexByte(endpoint1, ':')+1:])
			require.NoError(t, err)
			setOutputBlackhole(t, port)
			requireUsableEndpoint(t, client, endpoint2)
		})
	}
}

func TestClientReconnectTriesHealthyEndpointAfterBlackholedStandby(t *testing.T) {
	requireNetworkBlackholeTest(t)

	for _, tc := range []struct {
		name   string
		useTLS bool
	}{{name: "TCP"}, {name: "TLS", useTLS: true}} {
		t.Run(tc.name, func(t *testing.T) {
			client, endpoint1, endpoint2, connected1, _ := failoverClient(t, tc.useTLS,
				WithReconnect(400*time.Millisecond, &backoff.ZeroBackOff{}))
			port, err := strconv.Atoi(endpoint2[strings.LastIndexByte(endpoint2, ':')+1:])
			require.NoError(t, err)
			setOutputBlackhole(t, port)
			client.Disconnect()

			requireUsableEndpoint(t, client, endpoint1)
			require.GreaterOrEqual(t, atomic.LoadInt32(connected1), int32(2))
		})
	}
}

func TestUpdateEndpointsHonorsCallerOrderWhileDisconnected(t *testing.T) {
	endpoint1 := "tcp:127.0.0.1:6641"
	endpoint2 := "tcp:127.0.0.1:6642"
	client, err := newOVSDBClient(defDB, WithEndpoint(endpoint2), WithEndpoint(endpoint1))
	require.NoError(t, err)
	disconnectedRevision := client.endpointsRevision

	client.UpdateEndpoints([]string{endpoint1, endpoint2})
	client.rotateDisconnectedEndpoint(endpoint2, disconnectedRevision)
	require.Equal(t, endpoint1, client.endpoints[0].address)
	require.Equal(t, endpoint2, client.endpoints[1].address)
}
