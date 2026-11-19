package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
	"github.com/ovn-kubernetes/libovsdb/server"
	"github.com/stretchr/testify/require"
)

// newServerWithConnections starts a server and returns, in addition to its
// socket, a channel receiving the server side of every client connection, so
// that a test can close a connection the way ovsdb-server does when it stops.
func newServerWithConnections(t *testing.T) (*server.OvsdbServer, string, <-chan *rpc2.Client) {
	t.Helper()
	var defSchema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &defSchema))
	s, sock := newOVSDBServer(t, defDB, defSchema)
	conns := make(chan *rpc2.Client, 10)
	s.OnConnect(func(c *rpc2.Client) { conns <- c })
	return s, sock, conns
}

func nextServerConnection(t *testing.T, conns <-chan *rpc2.Client) *rpc2.Client {
	t.Helper()
	select {
	case c := <-conns:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no client connection reached the server")
		return nil
	}
}

// connectServerDBClient returns a client connected to the _Server database.
func connectServerDBClient(t *testing.T, sock string, opts ...Option) *ovsdbClient {
	t.Helper()
	serverDBModel, err := serverdb.FullDatabaseModel()
	require.NoError(t, err)
	ovs, err := newOVSDBClient(serverDBModel, append([]Option{WithEndpoint("unix:" + sock)}, opts...)...)
	require.NoError(t, err)
	require.NoError(t, ovs.Connect(context.Background()))
	t.Cleanup(ovs.Close)
	return ovs
}

// The disconnect handler waits for the other connection handlers. It must not
// start waiting before they are added, or a server closing the connection
// right away races with Connect.
func TestServerClosingConnectionTearsDownClient(t *testing.T) {
	_, sock, conns := newServerWithConnections(t)
	ovs := connectServerDBClient(t, sock)

	nextServerConnection(t, conns).Close()

	require.Eventually(t, func() bool {
		ovs.rpcMutex.RLock()
		defer ovs.rpcMutex.RUnlock()
		return ovs.rpcClient == nil
	}, 5*time.Second, 10*time.Millisecond)
}

func commentOperation() ovsdb.Operation {
	comment := "libovsdb test"
	return ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &comment}
}

// Connected must not keep reporting true once the server closed the
// connection: every call fails with ErrNotConnected from then on.
func TestConnectedIsFalseAfterServerClosesConnection(t *testing.T) {
	_, sock, conns := newServerWithConnections(t)
	ovs := connectServerDBClient(t, sock)
	require.True(t, ovs.Connected())

	nextServerConnection(t, conns).Close()

	require.Eventually(t, func() bool {
		return !ovs.Connected()
	}, 5*time.Second, 10*time.Millisecond, "Connected() still true after the server closed the connection")
	_, err := ovs.Transact(context.Background(), commentOperation())
	require.ErrorIs(t, err, ErrNotConnected)
}

// waitDisconnected waits until the client saw the server close its
// connection and had time to send the disconnect notification.
func waitDisconnected(t *testing.T, ovs *ovsdbClient) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !ovs.Connected()
	}, 5*time.Second, 10*time.Millisecond)
	// The notification is sent after the connection state is cleared.
	time.Sleep(200 * time.Millisecond)
}

// The disconnect notification must reach a caller that was not receiving at
// the moment the connection dropped, and must not be left pending on a later
// connection.
func TestDisconnectNotificationIsKeptUntilReceived(t *testing.T) {
	_, sock, conns := newServerWithConnections(t)
	ovs := connectServerDBClient(t, sock)

	nextServerConnection(t, conns).Close()
	waitDisconnected(t, ovs)
	select {
	case <-ovs.DisconnectNotify():
	case <-time.After(5 * time.Second):
		t.Fatal("the disconnect notification was dropped because nobody was receiving")
	}

	require.NoError(t, ovs.Connect(context.Background()))
	nextServerConnection(t, conns).Close()
	waitDisconnected(t, ovs)
	require.NoError(t, ovs.Connect(context.Background()))
	select {
	case <-ovs.DisconnectNotify():
		t.Fatal("the notification of the previous connection is pending on the new one")
	default:
	}
}

// When reconnection gives up, the client must neither panic nor keep calls
// waiting for a reconnection that is not coming: it is disconnected, says so,
// and fails calls until Connect is called again.
func TestReconnectGivingUpLeavesClientDisconnected(t *testing.T) {
	s, sock, conns := newServerWithConnections(t)
	ovs := connectServerDBClient(t, sock,
		WithReconnect(100*time.Millisecond, backoff.WithMaxRetries(&backoff.ZeroBackOff{}, 2)))

	// Stop listening first, so that every reconnection attempt fails.
	s.Close()
	nextServerConnection(t, conns).Close()

	select {
	case <-ovs.DisconnectNotify():
	case <-time.After(5 * time.Second):
		t.Fatal("no disconnect notification after reconnection gave up")
	}
	require.False(t, ovs.Connected())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := ovs.Transact(ctx, commentOperation())
	require.ErrorIs(t, err, ErrNotConnected)
	require.Less(t, time.Since(start), time.Second, "Transact waited for a reconnection that is not coming")
}
