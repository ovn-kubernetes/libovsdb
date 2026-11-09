package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/cenkalti/rpc2/jsonrpc"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestCloseIsTerminalConcurrentConnect(t *testing.T) {
	o, err := newOVSDBClient(defDB, WithEndpoint("unix:/nonexistent-lifecycle.sock"))
	require.NoError(t, err)
	o.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- o.Connect(context.Background()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.ErrorIs(t, err, ErrClosed)
	}
	require.False(t, o.Connected())
}

func TestConnectionManagerBlockedRPCDoesNotBlockClose(t *testing.T) {
	opts, _ := newOptions()
	cm := NewConnectionManager(opts, "", nil, make(chan Event, 8), make(chan Event, 64))
	a, b := net.Pipe()
	peer := rpc2.NewClientWithCodec(jsonrpc.NewJSONCodec(b))
	entered, release := make(chan struct{}), make(chan struct{})
	peer.Handle("hold", func(_ *rpc2.Client, _ []any, _ *[]any) error { close(entered); <-release; return nil })
	go peer.Run()
	defer func() { close(release); peer.Close() }()
	cm.rpcClient = rpc2.NewClientWithCodec(jsonrpc.NewJSONCodec(a))
	cm.state.Store(int32(StateConnected))
	go cm.rpcClient.Run()
	go cm.Run()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cm.CommandChannel() <- Command{Type: CmdCall, CallMethod: "hold", CallArgs: []any{}, CallReply: new([]any), Ctx: ctx, ResponseCh: make(chan CommandResult, 1)}
	<-entered
	closed := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: closed}
	select {
	case <-closed:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("Close blocked behind RPC")
	}
}

type observedBackoff struct {
	backoff.BackOff
	entered chan struct{}
	once    sync.Once
}

func (b *observedBackoff) NextBackOff() time.Duration {
	b.once.Do(func() { close(b.entered) })
	return b.BackOff.NextBackOff()
}

func TestEndpointUpdateRestartsBackoff(t *testing.T) {
	var s ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &s))
	srv, sock := newOVSDBServer(t, defDB, s)
	defer srv.Close()
	require.Eventually(t, srv.Ready, time.Second, time.Millisecond)
	b := &observedBackoff{BackOff: backoff.NewConstantBackOff(time.Hour), entered: make(chan struct{})}
	opts, _ := newOptions(WithEndpoint("unix:/nonexistent-lifecycle.sock"), WithReconnect(time.Second, b))
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, make(chan Event, 8), make(chan Event, 64))
	go cm.Run()
	defer func() { cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)} }()
	resp := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdDisconnect, ResponseCh: resp}
	<-resp
	// Synchronize with the failed attempt, rather than relying on a sleep.
	select {
	case <-b.entered:
	case <-time.After(time.Second):
		t.Fatal("attempt did not enter backoff")
	}
	resp = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdUpdateEndpoints, UpdateEndpoints: []string{fmt.Sprintf("unix:%s", sock)}, ResponseCh: resp}
	<-resp
	require.Eventually(t, cm.Connected, time.Second, time.Millisecond, "new endpoint must be attempted without waiting for old backoff")
}
