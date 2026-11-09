package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/rpc2"
	"github.com/cenkalti/rpc2/jsonrpc"
	"github.com/go-logr/logr"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
)

// ConnState represents the FSM state of the connection manager.
type ConnState int32

const (
	StateDisconnected ConnState = iota
	StateConnecting
	StateConnected
	StateRestoring
)

func (s ConnState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateRestoring:
		return "Restoring"
	default:
		return "Unknown"
	}
}

// CommandType identifies the kind of command sent from client to connection manager.
type CommandType int

const (
	CmdConnect CommandType = iota
	CmdDisconnect
	CmdClose
	CmdNotLeader
	// CmdCall starts a tracked outbound RPC without blocking the FSM.
	CmdCall
	CmdUpdateOptions
	CmdUpdateEndpoints
	CmdQueryState
	CmdQueryEndpoint
	// CmdRunInactivityEcho runs one inactivity echo check immediately (test-only; used to verify disconnect on echo failure).
	CmdRunInactivityEcho
	CmdRestoreDone
)

// EventType identifies the kind of event sent from connection manager to client.
type EventType int

const (
	EventDisconnected EventType = iota
	EventReconnected
	EventSchemaData
	EventInboundRPC
	EventBarrier
)

// SchemaPayload carries schema per database name (for Connect response or Reconnected event).
type SchemaPayload map[string]ovsdb.DatabaseSchema

// InboundRPCPayload carries server-push RPC data (update/update2/update3) for the client to apply.
type InboundRPCPayload struct {
	Method string            // "update", "update2", "update3"
	Args   []json.RawMessage // client decodes and applies
}

// Command is sent by the client on the command channel.
type Command struct {
	Generation uint64
	Err        error
	Type       CommandType
	ResponseCh chan<- CommandResult // optional; must have capacity >= 1; closed after reply
	Ctx        context.Context      // for RPC timeout/cancel

	// Payloads per command type (only one is used per command)
	CallMethod      string // for CmdCall: RPC method name (e.g. "transact", "echo", "monitor_cond_since")
	CallArgs        any    // for CmdCall: request payload
	CallReply       any    // for CmdCall: pointer to reply (filled by CM)
	UpdateOptions   *options
	UpdateEndpoints []string
}

// CommandResult is sent on Command.ResponseCh for commands that need a response.
type CommandResult struct {
	// NotSubmitted is true only when the FSM rejected an RPC before starting it.
	// Other connection errors may represent an operation committed by the peer.
	NotSubmitted     bool
	Generation       uint64 // originating RPC session, including successful replies
	Err              error
	Schema           SchemaPayload           // for Connect
	Results          []ovsdb.OperationResult // for Transact
	State            ConnState               // for QueryState
	EndpointAddress  string                  // for QueryEndpoint
	EndpointServerID string                  // for QueryEndpoint
}

// Event is sent by the connection manager on the event channel.
type Event struct {
	Generation       uint64
	FullSync         bool
	Ctx              context.Context
	Type             EventType
	Schema           SchemaPayload      // for EventReconnected, EventSchemaData
	Inbound          *InboundRPCPayload // for EventInboundRPC
	EndpointServerID string             // for EventReconnected, so client can track current endpoint without blocking on CM
	Processed        chan struct{}      // closed by the consumer for EventBarrier
}

// ConnectionManager owns connection transitions. Network waits run in tracked
// workers; no RPC or lifecycle-event consumer can block the control loop.
type ConnectionManager struct {
	state              atomic.Int32
	reconnectExhausted atomic.Bool
	disconnectNotify   chan struct{} // drained by the FSM immediately before publishing readiness
	generation         atomic.Uint64
	resyncVersion      *atomic.Uint64
	restoredVersion    uint64
	restoringVersion   uint64
	opts               *options
	dbNames            []string
	primaryDBName      string
	cmdCh              chan Command
	lifecycleCh        chan Event
	eventCh            chan Event
	logger             *logr.Logger
	endpoints          []*epInfo
	rpcClient          *rpc2.Client
	ctx                context.Context
	cancel             context.CancelFunc
	done               chan struct{}
	workers            sync.WaitGroup
	results            chan connectResult
	calls              chan callResult
	overflow           chan uint64
	attemptCtx         context.Context
	attemptCancel      context.CancelFunc
	validateSchema     func(SchemaPayload) error
	sessionCancel      context.CancelFunc
	sessionCtx         context.Context
	retry              *time.Timer
	probe              *time.Timer
	probing            bool
	reconnecting       bool
	requireReady       bool
	pending            []Command
	events             []Event
}

type connectResult struct {
	generation uint64
	client     *rpc2.Client
	schema     SchemaPayload
	endpoints  []*epInfo
	serverID   string
	err        error
}
type callResult struct {
	generation uint64
	cmd        Command
	err        error
	probe      bool
}

func NewConnectionManager(opts *options, primaryDBName string, dbNames []string, lifecycleCh, eventCh chan Event) *ConnectionManager {
	copyOpts := *opts
	copyOpts.endpoints = append([]string(nil), opts.endpoints...)
	ctx, cancel := context.WithCancel(context.Background())
	cm := &ConnectionManager{opts: &copyOpts, primaryDBName: primaryDBName, dbNames: dbNames,
		cmdCh: make(chan Command, 32), lifecycleCh: lifecycleCh, eventCh: eventCh, logger: opts.logger,
		resyncVersion: &atomic.Uint64{}, ctx: ctx, cancel: cancel, done: make(chan struct{}), results: make(chan connectResult), calls: make(chan callResult), overflow: make(chan uint64, 1)}
	if cm.logger == nil {
		l := logr.Discard()
		cm.logger = &l
	}
	for _, addr := range opts.endpoints {
		cm.endpoints = append(cm.endpoints, &epInfo{address: addr})
	}
	return cm
}
func (cm *ConnectionManager) CommandChannel() chan<- Command { return cm.cmdCh }
func (cm *ConnectionManager) EventChannel() chan Event       { return cm.eventCh }
func (cm *ConnectionManager) Connected() bool                { return cm.State() == StateConnected }
func (cm *ConnectionManager) State() ConnState               { return ConnState(cm.state.Load()) }
func (cm *ConnectionManager) Done() <-chan struct{}          { return cm.done }

// Do is the shutdown- and context-aware command boundary. The raw command
// channel is retained for internal tests; production callers use Do.
func (cm *ConnectionManager) Do(ctx context.Context, cmd Command) CommandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if cm.ctx.Err() != nil {
		return CommandResult{Err: ErrNotConnected}
	}
	if err := ctx.Err(); err != nil {
		return CommandResult{Err: err}
	}
	// Once Do returns on cancellation, callers may reuse their argument/reply
	// storage. Freeze JSON arguments before handing work to another goroutine,
	// and publish reply storage only when Do itself returns success.
	var replyTarget any
	if cmd.Type == CmdCall {
		args := cmd.CallArgs
		if args == nil || reflect.TypeOf(args).Kind() != reflect.Slice {
			args = []any{args}
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return CommandResult{Err: err}
		}
		cmd.CallArgs = json.RawMessage(raw)
		replyTarget = cmd.CallReply
		cmd.CallReply = reflect.New(reflect.TypeOf(replyTarget).Elem()).Interface()
	}
	reply := make(chan CommandResult, 1)
	cmd.ResponseCh = reply
	cmd.Ctx = ctx
	select {
	case cm.cmdCh <- cmd:
	case <-cm.done:
		return CommandResult{Err: ErrNotConnected}
	case <-ctx.Done():
		return CommandResult{Err: ctx.Err()}
	}
	select {
	case r := <-reply:
		if r.Err == nil && replyTarget != nil {
			reflect.ValueOf(replyTarget).Elem().Set(reflect.ValueOf(cmd.CallReply).Elem())
		}
		return r
	case <-cm.done:
		return CommandResult{Err: ErrNotConnected}
	case <-ctx.Done():
		return CommandResult{Err: ctx.Err()}
	}
}
func (cm *ConnectionManager) finish(cmd Command, r CommandResult) {
	if cmd.ResponseCh != nil {
		cmd.ResponseCh <- r
		close(cmd.ResponseCh)
	}
}
func timerChannel(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}
func stopTimer(t **time.Timer) {
	if *t != nil {
		(*t).Stop()
		*t = nil
	}
}

func (cm *ConnectionManager) Run() {
	defer func() {
		cm.cancel()
		cm.disconnect(false)
		cm.workers.Wait()
		cm.completeConnect(ErrNotConnected)
		close(cm.done)
	}()
	for {
		var server <-chan struct{}
		if cm.rpcClient != nil {
			server = cm.rpcClient.DisconnectNotify()
		}
		var events chan Event
		var event Event
		if len(cm.events) > 0 {
			events = cm.lifecycleCh
			event = cm.events[0]
		}
		var attemptDone <-chan struct{}
		if cm.attemptCtx != nil {
			attemptDone = cm.attemptCtx.Done()
		}
		select {
		case <-cm.ctx.Done():
			return
		case <-attemptDone:
			err := cm.attemptCtx.Err()
			cm.disconnect(true)
			cm.retryConnect(err)
		case events <- event:
			cm.events[0] = Event{}
			cm.events = cm.events[1:]
		case <-server:
			cm.rotateActiveEndpoint()
			cm.disconnect(true)
			cm.beginReconnect()
		case generation := <-cm.overflow:
			if generation == cm.generation.Load() {
				cm.rotateActiveEndpoint()
				cm.disconnect(true)
				cm.beginReconnect()
			}
		case <-timerChannel(cm.retry):
			cm.retry = nil
			cm.startAttempt(context.Background())
		case <-timerChannel(cm.probe):
			cm.probe = nil
			if !cm.probing && cm.rpcClient != nil {
				cm.startCall(Command{Type: CmdRunInactivityEcho}, true)
			}
		case result := <-cm.results:
			cm.connectedResult(result)
		case result := <-cm.calls:
			if result.generation != cm.generation.Load() {
				result.err = ErrNotConnected
			}
			if result.probe && result.generation == cm.generation.Load() {
				cm.probing = false
				if result.err != nil {
					cm.rotateActiveEndpoint()
					cm.disconnect(true)
					cm.beginReconnect()
				} else {
					cm.scheduleProbe()
				}
			}
			cm.finish(result.cmd, CommandResult{Err: result.err, Generation: result.generation})
		case cmd, ok := <-cm.cmdCh:
			if !ok {
				return
			}
			if cmd.Type == CmdClose {
				cm.finish(cmd, CommandResult{})
				return
			}
			cm.handleCommand(cmd)
		}
	}
}

func (cm *ConnectionManager) markReady() {
	cm.finishAttempt()
	// No obsolete disconnect can be observed after Connect returns success.
	select {
	case <-cm.disconnectNotify:
	default:
	}
	cm.reconnectExhausted.Store(false)
	cm.state.Store(int32(StateConnected))
}

func (cm *ConnectionManager) completeConnect(err error) {
	for _, cmd := range cm.pending {
		cm.finish(cmd, CommandResult{Err: err})
	}
	cm.pending = nil
}
func (cm *ConnectionManager) closeRPCClient() {
	if cm.rpcClient != nil {
		cm.rpcClient.Close()
		cm.rpcClient = nil
	}
}

// Only rotate an actual live connection. Disconnected endpoint updates must
// retain the caller's order, and removing an endpoint must not rotate its successor.
func (cm *ConnectionManager) rotateActiveEndpoint() {
	if cm.rpcClient != nil && len(cm.endpoints) > 1 {
		cm.endpoints = append(cm.endpoints[1:], cm.endpoints[0])
	}
}

func (cm *ConnectionManager) finishAttempt() {
	if cm.attemptCancel != nil {
		cm.attemptCancel()
		cm.attemptCancel = nil
	}
	cm.attemptCtx = nil
}

func (cm *ConnectionManager) disconnect(notify bool) {
	old := cm.generation.Load()
	cm.generation.Add(1) // invalidate completions and queued notifications immediately
	cm.finishAttempt()
	if cm.sessionCancel != nil {
		cm.sessionCancel()
		cm.sessionCancel = nil
	}
	cm.closeRPCClient()
	stopTimer(&cm.retry)
	stopTimer(&cm.probe)
	cm.probing = false
	cm.state.Store(int32(StateDisconnected))
	if notify {
		cm.events = append(cm.events, Event{Type: EventDisconnected, Generation: old})
	}
}
func (cm *ConnectionManager) beginReconnect() {
	if !cm.opts.reconnect || cm.opts.backoff == nil {
		cm.completeConnect(ErrNotConnected)
		return
	}
	cm.reconnecting = true
	cm.reconnectExhausted.Store(false)
	cm.opts.backoff.Reset()
	cm.startAttempt(context.Background())
}
func (cm *ConnectionManager) startAttempt(parent context.Context) {
	cm.state.Store(int32(StateConnecting))
	gen := cm.generation.Add(1)
	ctx, cancel := context.WithCancel(parent)
	if cm.reconnecting {
		cancel()
		ctx, cancel = context.WithTimeout(parent, cm.opts.timeout)
	}
	cm.attemptCtx, cm.attemptCancel = ctx, cancel
	stop := context.AfterFunc(cm.ctx, cancel)
	opts := *cm.opts
	opts.endpoints = nil
	for _, ep := range cm.endpoints {
		opts.endpoints = append(opts.endpoints, ep.address)
	}
	cm.workers.Add(1)
	go func() {
		defer cm.workers.Done()
		defer stop()
		defer cancel()
		worker := NewConnectionManager(&opts, cm.primaryDBName, cm.dbNames, nil, cm.eventCh)
		defer worker.cancel()
		worker.generation.Store(gen)
		worker.overflow = cm.overflow
		worker.resyncVersion = cm.resyncVersion
		worker.validateSchema = cm.validateSchema
		schema, sid, err := worker.runConnectSequence(ctx)
		rpc := worker.rpcClient
		result := connectResult{generation: gen, client: rpc, schema: schema, serverID: sid, endpoints: worker.endpoints, err: err}
		select {
		case cm.results <- result:
		case <-cm.ctx.Done():
			worker.closeRPCClient()
		}
		// This worker owns the reader's lifetime even after handing off the connection.
		if rpc != nil {
			select {
			case <-rpc.DisconnectNotify():
			case <-cm.ctx.Done():
				rpc.Close()
			}
		}
		worker.workers.Wait()
	}()
}
func (cm *ConnectionManager) connectedResult(r connectResult) {
	if r.generation != cm.generation.Load() {
		if r.client != nil {
			r.client.Close()
		}
		return
	}
	if r.err == nil && cm.attemptCtx != nil {
		r.err = cm.attemptCtx.Err()
		if r.err != nil && r.client != nil {
			r.client.Close()
		}
	}
	if r.err != nil {
		cm.retryConnect(r.err)
		return
	}
	cm.rpcClient = r.client
	cm.endpoints = r.endpoints
	cm.sessionCtx, cm.sessionCancel = context.WithCancel(cm.ctx)
	if cm.requireReady {
		cm.restoringVersion = cm.resyncVersion.Load()
		cm.state.Store(int32(StateRestoring))
		cm.events = append(cm.events, Event{Type: EventSchemaData, Generation: r.generation, FullSync: cm.restoringVersion != cm.restoredVersion, Schema: r.schema, EndpointServerID: r.serverID, Ctx: cm.attemptCtx})
	} else {
		cm.markReady()
		for _, cmd := range cm.pending {
			cm.finish(cmd, CommandResult{Schema: r.schema, EndpointServerID: r.serverID})
		}
		cm.pending = nil
		if cm.reconnecting {
			cm.events = append(cm.events, Event{Type: EventReconnected, Generation: r.generation, Schema: r.schema, EndpointServerID: r.serverID, Ctx: cm.sessionCtx})
		}
	}
	cm.scheduleProbe()
}

// Failed handshakes and monitor restorations share the same retry budget.
func (cm *ConnectionManager) retryConnect(err error) {
	cm.finishAttempt()
	if cm.reconnecting && cm.opts.backoff != nil {
		if next := cm.opts.backoff.NextBackOff(); next >= 0 {
			cm.state.Store(int32(StateConnecting))
			cm.retry = time.NewTimer(next)
			return
		}
	}
	if cm.reconnecting {
		cm.reconnectExhausted.Store(true)
	}
	cm.state.Store(int32(StateDisconnected))
	cm.completeConnect(err)
}

func (cm *ConnectionManager) scheduleProbe() {
	if cm.opts.inactivityTimeout > 0 {
		cm.probe = time.NewTimer(cm.opts.inactivityTimeout)
	}
}
func (cm *ConnectionManager) handleCommand(cmd Command) {
	switch cmd.Type {
	case CmdConnect:
		if cm.Connected() {
			cm.finish(cmd, CommandResult{})
			return
		}
		cm.pending = append(cm.pending, cmd)
		if cm.State() == StateDisconnected {
			cm.reconnecting = false
			ctx := cmd.Ctx
			if ctx == nil {
				ctx = context.Background()
			}
			cm.startAttempt(ctx)
		}
	case CmdRestoreDone:
		cm.restoreDone(cmd)
	case CmdDisconnect, CmdNotLeader:
		cm.completeConnect(ErrNotConnected)
		cm.rotateActiveEndpoint()
		cm.disconnect(true)
		cm.beginReconnect()
		cm.finish(cmd, CommandResult{})
	case CmdUpdateEndpoints:
		cm.updateEndpoints(cmd.UpdateEndpoints)
		cm.finish(cmd, CommandResult{})
	case CmdUpdateOptions:
		if cm.State() != StateDisconnected {
			cm.finish(cmd, CommandResult{Err: fmt.Errorf("cannot set option when client is connected")})
			return
		}
		if cmd.UpdateOptions != nil {
			opts := *cmd.UpdateOptions
			cm.opts = &opts
			cm.endpoints = nil
			for _, addr := range opts.endpoints {
				cm.endpoints = append(cm.endpoints, &epInfo{address: addr})
			}
		}
		cm.finish(cmd, CommandResult{})
	case CmdQueryState:
		cm.finish(cmd, CommandResult{State: cm.State()})
	case CmdQueryEndpoint:
		r := CommandResult{}
		if cm.rpcClient != nil && len(cm.endpoints) > 0 {
			r.EndpointAddress = cm.endpoints[0].address
			r.EndpointServerID = cm.endpoints[0].serverID
		}
		cm.finish(cmd, r)
	case CmdCall, CmdRunInactivityEcho:
		if cm.rpcClient == nil || (cmd.Generation != 0 && cmd.Generation != cm.generation.Load()) || (cm.requireReady && !cm.Connected() && cmd.Generation == 0) {
			cm.finish(cmd, CommandResult{Err: ErrNotConnected, NotSubmitted: true})
			return
		}
		cm.startCall(cmd, cmd.Type == CmdRunInactivityEcho)
	default:
		cm.finish(cmd, CommandResult{Err: ErrNotConnected})
	}
}

func (cm *ConnectionManager) restoreDone(cmd Command) {
	if cmd.Generation != cm.generation.Load() || cm.State() != StateRestoring {
		cm.finish(cmd, CommandResult{Err: ErrNotConnected})
		return
	}
	if cmd.Err == nil && cm.attemptCtx != nil {
		cmd.Err = cm.attemptCtx.Err()
	}
	if cmd.Err != nil {
		cm.completeConnect(cmd.Err)
		cm.rotateActiveEndpoint()
		cm.disconnect(true)
		if !cm.reconnecting && cm.opts.reconnect && cm.opts.backoff != nil {
			cm.reconnecting = true
			cm.opts.backoff.Reset()
		}
		cm.retryConnect(cmd.Err)
	} else {
		cm.restoredVersion = cm.restoringVersion
		cm.markReady()
		cm.completeConnect(nil)
	}
	cm.finish(cmd, CommandResult{})
}

// updateEndpoints runs on the FSM, preserving a live endpoint when possible.
func (cm *ConnectionManager) updateEndpoints(addresses []string) {
	old := ""
	if len(cm.endpoints) > 0 {
		old = cm.endpoints[0].address
	}
	current := -1
	eps := make([]*epInfo, 0, len(addresses))
	for i, addr := range addresses {
		ep := &epInfo{address: addr}
		if addr == old {
			current = i
			if len(cm.endpoints) > 0 {
				ep.serverID = cm.endpoints[0].serverID
			}
		}
		eps = append(eps, ep)
	}
	if len(eps) > 0 {
		cm.endpoints = eps
		if current > 0 && cm.rpcClient != nil {
			ep := eps[current]
			cm.endpoints = append([]*epInfo{ep}, append(eps[:current], eps[current+1:]...)...)
		}
		if cm.State() == StateConnecting || (cm.rpcClient != nil && current < 0) {
			cm.disconnect(true)
			cm.beginReconnect()
		}
	}
}

func (cm *ConnectionManager) startCall(cmd Command, probe bool) {
	if cmd.Ctx != nil && cmd.Ctx.Err() != nil {
		cm.finish(cmd, CommandResult{Err: cmd.Ctx.Err()})
		return
	}
	rpc, gen := cm.rpcClient, cm.generation.Load()
	parent := cmd.Ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	session := cm.sessionCtx
	if session == nil {
		session = cm.ctx
	}
	stop := context.AfterFunc(session, cancel)
	if probe {
		cancel()
		ctx, cancel = context.WithTimeout(session, cm.opts.inactivityTimeout)
		cm.probing = true
		cmd.CallMethod = "echo"
		cmd.CallArgs = ovsdb.NewEchoArgs()
		cmd.CallReply = new([]any)
	}
	cm.workers.Add(1)
	go func() {
		defer cm.workers.Done()
		defer cancel()
		defer stop()
		// rpc2 can decode a late response after cancellation. Never give it the
		// caller's reply storage, which may already have gone back into use.
		reply := reflect.New(reflect.TypeOf(cmd.CallReply).Elem()).Interface()
		// Go writes synchronously. A cancelled blocked write must close the transport.
		abortWrite := context.AfterFunc(ctx, func() { rpc.Close() })
		call := rpc.Go(cmd.CallMethod, cmd.CallArgs, reply, make(chan *rpc2.Call, 1))
		abortWrite()
		var err error
		select {
		case <-call.Done:
			err = call.Error
			if err == nil {
				reflect.ValueOf(cmd.CallReply).Elem().Set(reflect.ValueOf(reply).Elem())
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err == rpc2.ErrShutdown {
			err = ErrNotConnected
		}
		result := callResult{generation: gen, cmd: cmd, err: err, probe: probe}
		select {
		case cm.calls <- result:
		case <-cm.ctx.Done():
			cm.finish(cmd, CommandResult{Err: ErrNotConnected})
		}
	}()
}

// serialClientCodec also serializes replies to server-initiated requests with
// ordinary outbound writes; rpc2's sending lock only covers the latter.
type serialClientCodec struct {
	rpc2.Codec
	mu sync.Mutex
}

func (c *serialClientCodec) WriteRequest(r *rpc2.Request, v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Codec.WriteRequest(r, v)
}
func (c *serialClientCodec) WriteResponse(r *rpc2.Response, v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Codec.WriteResponse(r, v)
}

func (cm *ConnectionManager) registerInboundHandlers() {
	rpc, gen := cm.rpcClient, cm.generation.Load()
	rpc.Handle("echo", func(_ *rpc2.Client, args []any, reply *[]any) error { *reply = args; return nil })
	for _, method := range []string{"update", "update2", "update3"} {
		rpc.Handle(method, func(_ *rpc2.Client, args []json.RawMessage, reply *[]any) error {
			*reply = []any{}
			ev := Event{Type: EventInboundRPC, Generation: gen, Inbound: &InboundRPCPayload{Method: method, Args: args}}
			select {
			case cm.eventCh <- ev:
			default:
				// Stop this generation and invalidate its cursor so the next
				// restoration requests a full snapshot instead of accepting lost updates.
				cm.resyncVersion.Add(1)
				select {
				case cm.overflow <- gen:
				default:
				}
				rpc.Close()
			}
			return nil
		})
	}
}

// runConnectSequence runs only on an isolated connector worker. Each endpoint
// receives a share of the remaining deadline, including dial/TLS and all RPCs.
func (cm *ConnectionManager) runConnectSequence(ctx context.Context) (SchemaPayload, string, error) {
	ctx, span := tracer.Start(ctx, "connect")
	defer span.End()
	endpoints := append([]*epInfo(nil), cm.endpoints...)
	lastErr := ErrNotConnected
	for i, ep := range endpoints {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		attemptCtx, cancel := context.WithCancel(ctx)
		if deadline, ok := ctx.Deadline(); ok {
			cancel()
			attemptCtx, cancel = context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(endpoints)-i))
		}
		schema, sid, err := cm.tryEndpoint(attemptCtx, ep)
		cancel()
		if err != nil {
			cm.closeRPCClient()
			lastErr = err
			cm.logger.V(3).Info("endpoint failed", "endpoint", ep.address, "err", err)
			continue
		}
		ep.serverID = sid
		if i > 0 {
			cm.endpoints = append([]*epInfo{ep}, append(endpoints[:i], endpoints[i+1:]...)...)
		}
		return schema, sid, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return nil, "", lastErr
}

func (cm *ConnectionManager) tryEndpoint(ctx context.Context, ep *epInfo) (SchemaPayload, string, error) {
	ctx, span := tracer.Start(ctx, "tryEndpoint")
	defer span.End()
	u, err := url.Parse(ep.address)
	if err != nil {
		return nil, "", ErrNotConnected
	}
	conn, err := cm.dial(ctx, u, cm.opts.tlsConfig)
	if err != nil {
		return nil, "", ErrNotConnected
	}
	// rpc2 writes synchronously before selecting on ctx.Done. Closing the
	// transport is necessary to bound a stalled write as well as a stalled reply.
	stopCancel := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopCancel()
	cm.rpcClient = rpc2.NewClientWithCodec(&serialClientCodec{Codec: jsonrpc.NewJSONCodec(conn)})
	cm.rpcClient.SetBlocking(true)
	cm.registerInboundHandlers()
	rpc := cm.rpcClient
	cm.workers.Add(1)
	go func() { defer cm.workers.Done(); rpc.Run() }()
	names, err := cm.listDbs(ctx)
	if err != nil {
		return nil, "", err
	}
	dbSet := make(map[string]bool, len(names))
	for _, name := range names {
		dbSet[name] = true
	}
	schemas := make(SchemaPayload)
	for _, name := range cm.dbNames {
		if !dbSet[name] {
			return nil, "", fmt.Errorf("target database %s not found", name)
		}
		schema, err := cm.getSchema(ctx, name)
		if err != nil {
			return nil, "", err
		}
		schemas[name] = schema
	}
	if cm.validateSchema != nil {
		if err := cm.validateSchema(schemas); err != nil {
			return nil, "", err
		}
	}
	sid := ""
	if cm.opts.leaderOnly {
		leader, id, err := cm.isEndpointLeader(ctx)
		if err != nil {
			return nil, "", err
		}
		if !leader {
			return nil, "", ErrNotConnected
		}
		sid = id
	}
	// Do not hand off a connection whose deadline callback has already fired.
	if !stopCancel() {
		return nil, "", ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return schemas, sid, nil
}

func (cm *ConnectionManager) dial(ctx context.Context, u *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	switch u.Scheme {
	case UNIX:
		var d net.Dialer
		return d.DialContext(ctx, u.Scheme, u.Path)
	case TCP:
		var d net.Dialer
		return d.DialContext(ctx, u.Scheme, u.Opaque)
	case SSL:
		d := tls.Dialer{Config: tlsConfig}
		return d.DialContext(ctx, "tcp", u.Opaque)
	default:
		return nil, fmt.Errorf("unknown scheme %s", u.Scheme)
	}
}

func (cm *ConnectionManager) listDbs(ctx context.Context) ([]string, error) {
	ctx, span := tracer.Start(ctx, "listDbs")
	defer span.End()
	var dbs []string
	err := cm.rpcClient.CallWithContext(ctx, "list_dbs", nil, &dbs)
	if err != nil {
		if err == rpc2.ErrShutdown {
			return nil, ErrNotConnected
		}
		return nil, err
	}
	return dbs, nil
}

func (cm *ConnectionManager) getSchema(ctx context.Context, dbName string) (ovsdb.DatabaseSchema, error) {
	ctx, span := tracer.Start(ctx, "getSchema")
	defer span.End()
	args := ovsdb.NewGetSchemaArgs(dbName)
	var reply ovsdb.DatabaseSchema
	err := cm.rpcClient.CallWithContext(ctx, "get_schema", args, &reply)
	if err != nil {
		if err == rpc2.ErrShutdown {
			return ovsdb.DatabaseSchema{}, ErrNotConnected
		}
		return ovsdb.DatabaseSchema{}, err
	}
	return reply, nil
}

func (cm *ConnectionManager) isEndpointLeader(ctx context.Context) (bool, string, error) {
	ctx, span := tracer.Start(ctx, "isEndpointLeader")
	defer span.End()
	op := ovsdb.Operation{
		Op:      ovsdb.OperationSelect,
		Table:   "Database",
		Columns: []string{"name", "model", "leader", "sid"},
	}
	var results []ovsdb.OperationResult
	args := ovsdb.NewTransactArgs(serverDB, op)
	err := cm.rpcClient.CallWithContext(ctx, "transact", args, &results)
	if err != nil {
		return false, "", err
	}
	if len(results) != 1 {
		return true, "", nil
	}
	result := results[0]
	if len(result.Rows) == 0 {
		return true, "", nil
	}
	for _, row := range result.Rows {
		dbName, ok := row["name"].(string)
		if !ok {
			return false, "", fmt.Errorf("could not parse name")
		}
		if dbName != cm.primaryDBName {
			continue
		}
		model, ok := row["model"].(string)
		if !ok {
			return false, "", fmt.Errorf("could not parse model")
		}
		if model != serverdb.DatabaseModelClustered {
			return true, "", nil
		}
		sid, ok := row["sid"].(ovsdb.UUID)
		if !ok {
			return false, "", fmt.Errorf("could not parse server id")
		}
		leader, ok := row["leader"].(bool)
		if !ok {
			return false, "", fmt.Errorf("could not parse leader")
		}
		return leader, sid.GoUUID, nil
	}
	return true, "", nil
}
