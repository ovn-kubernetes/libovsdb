package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/cenkalti/rpc2/jsonrpc"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

// A real JSON-RPC transport with programmable monitor responses. Unlike the
// in-memory server, this peer can exercise Found=true incremental recovery.
func lifecycleServer(t *testing.T, register func(*rpc2.Client)) string {
	t.Helper()
	var s ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &s))
	return lifecycleServerWithSchema(t, s, register)
}

func lifecycleServerWithSchema(t *testing.T, s ovsdb.DatabaseSchema, register func(*rpc2.Client)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var peers []*rpc2.Client
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			p := rpc2.NewClientWithCodec(&serialClientCodec{Codec: jsonrpc.NewJSONCodec(conn)})
			p.Handle("list_dbs", func(_ *rpc2.Client, _ []any, reply *[]string) error { *reply = []string{"Open_vSwitch"}; return nil })
			p.Handle("get_schema", func(_ *rpc2.Client, _ []any, reply *ovsdb.DatabaseSchema) error { *reply = s; return nil })
			p.Handle("echo", func(_ *rpc2.Client, args []any, reply *[]any) error { *reply = args; return nil })
			register(p)
			mu.Lock()
			peers = append(peers, p)
			mu.Unlock()
			go p.Run()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		<-done
		mu.Lock()
		defer mu.Unlock()
		for _, p := range peers {
			p.Close()
			<-p.DisconnectNotify()
		}
	})
	return "tcp:" + listener.Addr().String()
}

func lifecycleClient(t *testing.T, endpoint string, reconnect bool) *ovsdbClient {
	t.Helper()
	opts := []Option{WithEndpoint(endpoint)}
	if reconnect {
		opts = append(opts, WithReconnect(time.Second, backoff.NewConstantBackOff(5*time.Millisecond)))
	}
	o, err := newOVSDBClient(defDB, opts...)
	require.NoError(t, err)
	t.Cleanup(o.Close)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Connect(ctx))
	return o
}
func bridgeSnapshot(name string) ovsdb.TableUpdates2 {
	row := ovsdb.Row{"name": name}
	return ovsdb.TableUpdates2{"Bridge": {aUUID0: &ovsdb.RowUpdate2{Initial: &row}}}
}
func monitorBridge(t *testing.T, o *ovsdbClient) MonitorCookie {
	t.Helper()
	m := newMonitor()
	m.Method = ovsdb.ConditionalMonitorSinceRPC
	m.Tables = []TableMonitor{{Table: "Bridge"}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cookie, err := o.Monitor(ctx, m)
	require.NoError(t, err)
	return cookie
}

func TestIncrementalReconnectRetainsUnchangedRows(t *testing.T) {
	var calls atomic.Int32
	requested := make(chan string, 4)
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, args []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			var txn string
			if err := json.Unmarshal(args[3], &txn); err != nil {
				return err
			}
			requested <- txn
			if calls.Add(1) == 1 {
				*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("unchanged")}
			} else {
				*reply = ovsdb.MonitorCondSinceReply{Found: true, LastTransactionID: aUUID1, Updates: ovsdb.TableUpdates2{}}
			}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, true)
	monitorBridge(t, o)
	require.Equal(t, emptyUUID, <-requested)
	o.Disconnect()
	select {
	case txn := <-requested:
		require.Equal(t, aUUID1, txn)
	case <-time.After(time.Second):
		t.Fatal("monitor was not restored")
	}
	require.Eventually(t, o.Connected, time.Second, time.Millisecond)
	row := o.Cache().Table("Bridge").Row(aUUID0)
	require.NotNil(t, row)
	require.Equal(t, "unchanged", row.(*Bridge).Name)
}

func TestReadinessWaitsForMonitorRestorationAndRetriesErrors(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, _ []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			switch calls.Add(1) {
			case 2:
				return errors.New("temporary monitor failure")
			case 3:
				close(entered)
				<-release
			}
			*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("restored")}
			return nil
		})
	})
	defer close(release)
	o := lifecycleClient(t, endpoint, true)
	monitorBridge(t, o)
	o.Disconnect()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("failed restoration was not retried")
	}
	require.False(t, o.Connected())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, o.Connect(ctx), context.DeadlineExceeded, "Connect must not report readiness while a monitor is pending")
	// Close must cancel the outstanding restoration, even though the peer is still waiting.
	closed := make(chan struct{})
	go func() { o.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close blocked on monitor restoration")
	}
}

func TestConcurrentOutboundCallsWithBlockingInboundHandlers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle("hold", func(_ *rpc2.Client, _ []any, _ *[]any) error { close(entered); <-release; return nil })
	})
	defer close(release)
	o := lifecycleClient(t, endpoint, false)
	held := make(chan CommandResult, 1)
	go func() {
		held <- o.cm.Do(context.Background(), Command{Type: CmdCall, CallMethod: "hold", CallArgs: []any{}, CallReply: new([]any)})
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Echo(ctx), "a second call must complete while the first is awaiting its reply")
	o.Close()
	select {
	case r := <-held:
		require.Error(t, r.Err)
	case <-time.After(time.Second):
		t.Fatal("in-flight call not cancelled")
	}
}

func TestStaleGenerationCannotOverwriteReconnectedCache(t *testing.T) {
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, _ []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("current")}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, true)
	cookie := monitorBridge(t, o)
	old := o.generation.Load()
	o.Disconnect()
	require.Eventually(t, func() bool { return o.Connected() && o.generation.Load() != old }, time.Second, time.Millisecond)
	cookieJSON, _ := json.Marshal(cookie)
	row := ovsdb.Row{"name": "stale"}
	updateJSON, _ := json.Marshal(ovsdb.TableUpdates2{"Bridge": {aUUID0: &ovsdb.RowUpdate2{Modify: &row}}})
	// dispatchEvent is otherwise owned by the event loop; no live updates are
	// sent by this peer. A stale generation must be rejected before decoding.
	o.dispatchEvent(Event{Type: EventInboundRPC, Generation: old, Inbound: &InboundRPCPayload{Method: "update2", Args: []json.RawMessage{cookieJSON, updateJSON}}})
	require.Equal(t, "current", o.Cache().Table("Bridge").Row(aUUID0).(*Bridge).Name)
}

func TestInboundOverflowDisconnectsAndRequestsFullSnapshot(t *testing.T) {
	// Exercise the real synchronous rpc2 inbound handler with a deliberately
	// blocked consumer. No notification is silently lost on a usable connection.
	opts, _ := newOptions()
	updates := make(chan Event, 1)
	cm := NewConnectionManager(opts, "", nil, make(chan Event, 8), updates)
	a, b := net.Pipe()
	cm.rpcClient = rpc2.NewClientWithCodec(&serialClientCodec{Codec: jsonrpc.NewJSONCodec(a)})
	cm.rpcClient.SetBlocking(true)
	cm.generation.Store(7)
	cm.registerInboundHandlers()
	rpc := cm.rpcClient
	go rpc.Run()
	peer := rpc2.NewClientWithCodec(&serialClientCodec{Codec: jsonrpc.NewJSONCodec(b)})
	go peer.Run()
	defer peer.Close()
	defer rpc.Close()
	for i := 0; i < 2; i++ {
		require.NoError(t, peer.Notify("update2", []any{}))
	}
	select {
	case <-rpc.DisconnectNotify():
	case <-time.After(time.Second):
		t.Fatal("overflow did not close transport")
	}
	require.NotZero(t, cm.resyncVersion.Load(), "overflow must invalidate incremental recovery")
	select {
	case gen := <-cm.overflow:
		require.Equal(t, uint64(7), gen)
	case <-time.After(time.Second):
		t.Fatal("overflow not reported")
	}
}

func TestOverflowDuringRestorationForcesFullSnapshot(t *testing.T) {
	var calls atomic.Int32
	nextCursor := make(chan string, 1)
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, args []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			n := calls.Add(1)
			if n == 2 {
				// Restoration occupies the event loop. Fill its queue through the real
				// rpc2 reader, then exceed capacity before returning the monitor response.
				for i := 0; i < 80; i++ {
					if err := p.Notify("update2", []any{args[1], ovsdb.TableUpdates2{}}); err != nil {
						return err
					}
				}
			}
			if n == 3 {
				var cursor string
				if err := json.Unmarshal(args[3], &cursor); err != nil {
					return err
				}
				nextCursor <- cursor
			}
			*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("complete")}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, true)
	monitorBridge(t, o)
	o.Disconnect()
	select {
	case cursor := <-nextCursor:
		require.Equal(t, emptyUUID, cursor)
	case <-time.After(2 * time.Second):
		t.Fatal("overflow did not trigger a new snapshot")
	}
	require.Eventually(t, o.Connected, time.Second, time.Millisecond)
	require.Equal(t, "complete", o.Cache().Table("Bridge").Row(aUUID0).(*Bridge).Name)
}

func TestCloseUnblocksTransactionsWaitingForReconnect(t *testing.T) {
	o, err := newOVSDBClient(defDB, WithEndpoint("unix:/nonexistent-close.sock"), WithReconnect(time.Second, backoff.NewConstantBackOff(time.Second)))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := o.Transact(context.Background(), ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"})
		done <- err
	}()
	o.Close()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrNotConnected)
	case <-time.After(time.Second):
		t.Fatal("transaction survived Close")
	}
	select {
	case <-o.cm.Done():
	default:
		t.Fatal("manager survived Close")
	}
}

func TestCloseDuringConnectHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	o, err := newOVSDBClient(defDB, WithEndpoint("tcp:"+listener.Addr().String()))
	require.NoError(t, err)
	connected := make(chan error, 1)
	go func() { connected <- o.Connect(context.Background()) }()
	conn := <-accepted
	defer conn.Close() // peer never replies to list_dbs
	closed := make(chan struct{})
	go func() { o.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close blocked during handshake")
	}
	require.ErrorIs(t, <-connected, ErrClosed)
}
