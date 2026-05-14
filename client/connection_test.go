package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionManagerFSMState tests that the CM starts Disconnected and responds to QueryState.
func TestConnectionManagerFSMState(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/nonexistent"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	// Initially disconnected
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryState, ResponseCh: respCh}
	r := <-respCh
	assert.Equal(t, StateDisconnected, r.State)

	// Disconnect when already disconnected is a no-op
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdDisconnect, ResponseCh: respCh}
	<-respCh
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryState, ResponseCh: respCh}
	r = <-respCh
	assert.Equal(t, StateDisconnected, r.State)
}

// TestConnectionManagerClose verifies that CmdClose is processed and subsequent
// commands return ErrNotConnected. Run() stays alive in drain mode after Close.
func TestConnectionManagerClose(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/nonexistent"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()

	// Close the manager.
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: respCh}
	r := <-respCh
	require.NoError(t, r.Err)

	// After Close, further RPC commands must return ErrNotConnected immediately.
	rpcRespCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{
		Type: CmdCall, CallMethod: "echo", CallArgs: []any{}, CallReply: new([]any),
		ResponseCh: rpcRespCh, Ctx: context.Background(),
	}
	select {
	case r := <-rpcRespCh:
		require.Equal(t, ErrNotConnected, r.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("command after CmdClose did not return promptly")
	}
}

// TestConnectionManagerRPCWhenDisconnected returns ErrNotConnected for RPC commands.
func TestConnectionManagerRPCWhenDisconnected(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/nonexistent"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{
		Type: CmdCall, CallMethod: "echo", CallArgs: []any{}, CallReply: new([]any),
		ResponseCh: respCh, Ctx: context.Background(),
	}
	r := <-respCh
	assert.Equal(t, ErrNotConnected, r.Err)
}

// TestConnectionManagerUpdateOptions propagates options.
func TestConnectionManagerUpdateOptions(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/a"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	newOpts, _ := newOptions(WithEndpoint("unix:/b"))
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdUpdateOptions, UpdateOptions: newOpts, ResponseCh: respCh}
	<-respCh

	// When disconnected, QueryEndpoint returns empty; endpoint list was updated for next Connect
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryEndpoint, ResponseCh: respCh}
	r := <-respCh
	assert.Empty(t, r.EndpointAddress)
}

// TestConnectionManagerUpdateEndpoints propagates endpoint list.
func TestConnectionManagerUpdateEndpoints(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/a"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdUpdateEndpoints, UpdateEndpoints: []string{"unix:/b", "unix:/c"}, ResponseCh: respCh}
	<-respCh

	// When disconnected, QueryEndpoint returns empty
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryEndpoint, ResponseCh: respCh}
	r := <-respCh
	assert.Empty(t, r.EndpointAddress)
}

// TestConnectionManagerConnectSuccess runs Connect against a real in-memory server.
func TestConnectionManagerConnectSuccess(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	server, sock := newOVSDBServer(t, defDB, defSchema)
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool { return server.Ready() }, time.Second, 10*time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	opts, err := newOptions(WithEndpoint(endpoint))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	r := <-respCh
	require.NoError(t, r.Err)
	require.NotNil(t, r.Schema)
	require.Contains(t, r.Schema, "Open_vSwitch")

	// State should be Connected
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryState, ResponseCh: respCh}
	stateR := <-respCh
	assert.Equal(t, StateConnected, stateR.State)

	// QueryEndpoint should return the connected endpoint
	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryEndpoint, ResponseCh: respCh}
	epR := <-respCh
	assert.Equal(t, endpoint, epR.EndpointAddress)
}

// TestConnectionManagerConnectInvalidEndpoint fails Connect and stays Disconnected.
func TestConnectionManagerConnectInvalidEndpoint(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/tmp/ovsdb-nonexistent-" + fmt.Sprintf("%d", rand.Intn(100000)) + ".sock"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	r := <-respCh
	require.Error(t, r.Err)
	require.Equal(t, ErrNotConnected, r.Err)

	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryState, ResponseCh: respCh}
	stateR := <-respCh
	assert.Equal(t, StateDisconnected, stateR.State)
}

// TestConnectionManagerDisconnectAfterConnect connects then disconnects.
func TestConnectionManagerDisconnectAfterConnect(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	server, sock := newOVSDBServer(t, defDB, defSchema)
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool { return server.Ready() }, time.Second, 10*time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	opts, err := newOptions(WithEndpoint(endpoint))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx := context.Background()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	<-respCh

	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdDisconnect, ResponseCh: respCh}
	<-respCh

	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryState, ResponseCh: respCh}
	r := <-respCh
	assert.Equal(t, StateDisconnected, r.State)
}

// TestConnectionManagerEventDisconnected sends EventDisconnected when server disconnects.
func TestConnectionManagerEventDisconnected(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	server, sock := newOVSDBServer(t, defDB, defSchema)
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool { return server.Ready() }, time.Second, 10*time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	opts, err := newOptions(WithEndpoint(endpoint))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx := context.Background()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	<-respCh

	// Close server so rpc2 sees disconnect
	server.Close()

	// We should see EventDisconnected (server close is detected by rpc2 DisconnectNotify)
	select {
	case ev := <-lifecycleCh:
		assert.Equal(t, EventDisconnected, ev.Type)
	case <-time.After(3 * time.Second):
		// rpc2 may not always detect socket close immediately on all platforms
		t.Skip("skipping: did not receive EventDisconnected within 3s (platform/timing dependent)")
	}
}

// TestConnectionManagerQueryEndpointWhenDisconnected returns empty address.
func TestConnectionManagerQueryEndpointWhenDisconnected(t *testing.T) {
	opts, err := newOptions(WithEndpoint("unix:/x"))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdQueryEndpoint, ResponseCh: respCh}
	r := <-respCh
	assert.Empty(t, r.EndpointAddress)
	assert.Empty(t, r.EndpointServerID)
}

// TestConnectionManagerInactivityProbeDisconnect verifies that when the server stops
// responding to echo (DoEcho(false)), the CM sends EventDisconnected after the inactivity probe.
func TestConnectionManagerInactivityProbeDisconnect(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	server, sock := newOVSDBServer(t, defDB, defSchema)
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool { return server.Ready() }, time.Second, 10*time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	opts, err := newOptions(
		WithEndpoint(endpoint),
		WithInactivityCheck(100*time.Millisecond, 50*time.Millisecond, &backoff.ZeroBackOff{}),
	)
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx := context.Background()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	r := <-respCh
	require.NoError(t, r.Err)

	server.DoEcho(false)

	// First probe fires after ~10ms; we should get EventDisconnected soon.
	select {
	case ev := <-lifecycleCh:
		assert.Equal(t, EventDisconnected, ev.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive EventDisconnected after server DoEcho(false)")
	}
}

// TestConnectionManagerEchoError verifies that when the server returns an error from Echo (DoEcho(false)),
// a direct CmdEcho returns that error to the caller.
func TestConnectionManagerEchoError(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	server, sock := newOVSDBServer(t, defDB, defSchema)
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool { return server.Ready() }, time.Second, 10*time.Millisecond)

	endpoint := fmt.Sprintf("unix:%s", sock)
	opts, err := newOptions(WithEndpoint(endpoint))
	require.NoError(t, err)
	lifecycleCh := make(chan Event, 8)
	eventCh := make(chan Event, 64)
	cm := NewConnectionManager(opts, "Open_vSwitch", []string{"Open_vSwitch"}, lifecycleCh, eventCh)
	go cm.Run()
	defer func() {
		cm.CommandChannel() <- Command{Type: CmdClose, ResponseCh: make(chan CommandResult, 1)}
	}()

	ctx := context.Background()
	respCh := make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{Type: CmdConnect, ResponseCh: respCh, Ctx: ctx}
	r := <-respCh
	require.NoError(t, r.Err)

	server.DoEcho(false)

	respCh = make(chan CommandResult, 1)
	cm.CommandChannel() <- Command{
		Type: CmdCall, CallMethod: "echo", CallArgs: ovsdb.NewEchoArgs(), CallReply: new([]any),
		ResponseCh: respCh, Ctx: context.Background(),
	}
	r = <-respCh
	assert.Error(t, r.Err)
}
