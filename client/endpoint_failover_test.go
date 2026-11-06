// Portable regressions adapted from https://github.com/ovn-kubernetes/libovsdb/pull/492.
package client

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/stretchr/testify/require"
)

// Taken from PR #492's timeout validation test.
func TestReconnectTimeoutValidation(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Millisecond, time.Millisecond} {
		t.Run(timeout.String(), func(t *testing.T) {
			for name, opt := range map[string]Option{
				"reconnect":  WithReconnect(timeout, &backoff.ZeroBackOff{}),
				"inactivity": WithInactivityCheck(time.Second, timeout, &backoff.ZeroBackOff{}),
			} {
				t.Run(name, func(t *testing.T) {
					_, err := newOptions(opt)
					if timeout <= 0 {
						require.EqualError(t, err, "reconnect timeout must be greater than zero")
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

// Public-API version of PR #492's disconnected endpoint ordering test.
func TestDisconnectedEndpointOrder(t *testing.T) {
	first := lifecycleServer(t, func(*rpc2.Client) {})
	second := lifecycleServer(t, func(*rpc2.Client) {})
	o, err := newOVSDBClient(defDB, WithEndpoint(second), WithEndpoint(first))
	require.NoError(t, err)
	defer o.Close()
	o.UpdateEndpoints([]string{first, second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Connect(ctx))
	require.Equal(t, first, o.CurrentEndpoint(), "a disconnected client's new endpoint order must be honored")
}

// PR #492: failover should rotate the actual live endpoint, even after
// UpdateEndpoints has refreshed the configured list.
func TestRotateActiveEndpointAfterLiveUpdate(t *testing.T) {
	first := lifecycleServer(t, func(*rpc2.Client) {})
	second := lifecycleServer(t, func(*rpc2.Client) {})
	o, err := newOVSDBClient(defDB, WithEndpoint(first), WithEndpoint(second), WithReconnect(time.Second, &backoff.ZeroBackOff{}))
	require.NoError(t, err)
	defer o.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Connect(ctx))
	require.Equal(t, first, o.CurrentEndpoint())
	o.UpdateEndpoints([]string{first, second})
	o.Disconnect()
	require.Eventually(t, func() bool { return o.Connected() && o.CurrentEndpoint() == second }, time.Second, time.Millisecond)
}

// A portable silent-peer harness, not a kernel packet blackhole. Cleanup joins
// the accept goroutine and closes the server side of the stalled connection.
func silentEndpoint(t *testing.T, scheme string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	accepted := make(chan net.Conn, 1)
	go func() {
		defer close(accepted)
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		if conn, ok := <-accepted; ok {
			conn.Close()
		}
	})
	return scheme + ":" + listener.Addr().String()
}

func TestSilentEndpointDoesNotStarveHealthyEndpoint(t *testing.T) {
	for _, scheme := range []string{TCP, SSL} {
		t.Run(scheme, func(t *testing.T) {
			silent := silentEndpoint(t, scheme)
			healthy := lifecycleServer(t, func(*rpc2.Client) {})
			o, err := newOVSDBClient(defDB, WithEndpoint(silent), WithEndpoint(healthy), WithTLSConfig(&tls.Config{InsecureSkipVerify: true})) // test-only silent TLS peer
			require.NoError(t, err)
			defer o.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, o.Connect(ctx), "a stalled dial/TLS/list_dbs must leave budget for healthy endpoints")
			require.Equal(t, healthy, o.CurrentEndpoint())
		})
	}
}

func TestReconnectTriesHealthyEndpointAfterSilentStandby(t *testing.T) {
	healthy := lifecycleServer(t, func(*rpc2.Client) {})
	silent := silentEndpoint(t, TCP)
	o, err := newOVSDBClient(defDB, WithEndpoint(healthy), WithEndpoint(silent), WithReconnect(time.Second, &backoff.ZeroBackOff{}))
	require.NoError(t, err)
	defer o.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Connect(ctx))
	old := o.generation.Load()
	o.Disconnect() // rotates to the silent standby first
	require.Eventually(t, func() bool { return o.Connected() && o.generation.Load() != old }, 2*time.Second, time.Millisecond)
	require.Equal(t, healthy, o.CurrentEndpoint())
}
