package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/mapper"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
	"go.opentelemetry.io/otel"
)

// Constants defined for libovsdb
const (
	SSL  = "ssl"
	TCP  = "tcp"
	UNIX = "unix"
)

const serverDB = "_Server"

// ErrNotConnected is returned when the required connection is unavailable.
// For an in-flight transaction, it does not imply that the server did not commit.
var ErrNotConnected = errors.New("not connected")

// ErrAlreadyConnected is an error returned when the client is already connected
var ErrAlreadyConnected = errors.New("already connected")

// ErrClosed is returned by Connect after the client has been permanently closed.
var ErrClosed = errors.New("client is closed")

// ErrUnsupportedRPC is an error returned when an unsupported RPC method is called
var ErrUnsupportedRPC = errors.New("unsupported rpc")

// Client represents an OVSDB Client Connection
// It provides all the necessary functionality to Connect to a server,
// perform transactions, and build your own replica of the database with
// Monitor or MonitorAll. It also provides a Cache that is populated from OVSDB
// update notifications.
//
// A Client can be reused across Disconnect and Connect calls, including endpoint
// changes through UpdateEndpoints. Close permanently ends the client's lifetime;
// use NewOVSDBClient to create a new client after Close.
type Client interface {
	// Connect establishes a connection using the current endpoints and returns
	// success only after schema validation and restoration of existing monitors.
	// It is idempotent while connected. After Disconnect it reuses the client's
	// cache and monitor registrations; cache contents may be rebuilt by recovery.
	// The attempt's context covers monitor restoration; an established session
	// outlives that context. Incompatible endpoints are skipped during selection.
	// After Close it returns ErrClosed and does not restart the client.
	Connect(context.Context) error

	// Disconnect ends the current connection without disposing of the client.
	// The cache, monitor registrations, and cache event handlers are retained.
	// When WithReconnect or WithInactivityCheck is enabled, reconnection starts
	// automatically; otherwise, call Connect explicitly. Disconnect is not a pause
	// when automatic reconnect is enabled. Use Close to permanently stop the client.
	Disconnect()

	// Close permanently shuts down the client and waits for its internal workers
	// and cache event callbacks to finish. It is safe to call more than once.
	// Subsequent Connect calls return ErrClosed; UpdateEndpoints cannot reopen it.
	// A retained Cache is no longer updated by this client. To reuse the client
	// and its cache with different endpoints, use Disconnect, UpdateEndpoints,
	// and Connect instead of Close. Do not call Close synchronously from a cache
	// event callback: Close waits for that callback to return.
	Close()

	Schema() ovsdb.DatabaseSchema

	// Cache returns the client-owned TableCache, or nil before cache initialization.
	// The TableCache and its event handlers are retained across Disconnect and
	// Connect, but recovery may replace its per-table caches and row contents.
	// Reacquire per-table caches after reconnect. Direct cache access does not wait
	// for connection readiness and can expose stale data while disconnected or
	// restoring monitors. After Close, the retained cache is no longer updated
	// by this client; creating a new client does not reactivate the old cache.
	Cache() *cache.TableCache

	// UpdateEndpoints replaces the endpoint list without recreating the client
	// or its cache. A live connection is retained if its endpoint is still listed;
	// removing it disconnects and follows the configured reconnect policy.
	// This method does not wait for connection readiness; call Connect to establish
	// a connection or wait for restoration. An empty list selects the default Unix
	// endpoint. Calls after Close are ignored and cannot reopen the client.
	UpdateEndpoints([]string)

	SetOption(Option) error

	// Connected reports readiness, including restoration of existing monitors.
	// It is always false after Close.
	Connected() bool
	DisconnectNotify() chan struct{}
	Echo(context.Context) error
	// Transact waits for preceding monitor updates to be applied in its connection
	// generation. With reconnect enabled, only requests rejected before submission
	// are automatically retried. An error after submission, including cancellation
	// or ErrNotConnected, does not prove that the server did not commit.
	Transact(context.Context, ...ovsdb.Operation) ([]ovsdb.OperationResult, error)
	// Monitor registers a monitor and populates the cache from its initial reply.
	// Existing monitors are restored automatically on reconnection; do not register
	// them again solely because the client disconnected. Overlapping monitor row
	// scopes are unsupported.
	Monitor(context.Context, *Monitor) (MonitorCookie, error)

	// MonitorAll registers a monitor for all tables and columns in the client model.
	// Like Monitor, its registration survives Disconnect and is restored on Connect.
	// Calling MonitorAll again is not an idempotent way to restore existing monitors.
	MonitorAll(context.Context) (MonitorCookie, error)
	MonitorCancel(ctx context.Context, cookie MonitorCookie) error
	NewMonitor(...MonitorOption) *Monitor
	CurrentEndpoint() string
	API
	// GetSelectResultsByIndex parses the result of the select operation indicated by
	// the 0-based index from the transaction results of the provided operations.
	GetSelectResultsByIndex(ops []ovsdb.Operation, results []ovsdb.OperationResult, target interface{}, index int) error

	// GetSelectResults parses the result of the first select operation from the
	// transaction results of the provided operations.
	GetSelectResults(ops []ovsdb.Operation, results []ovsdb.OperationResult, target interface{}) error
}

type bufferedUpdate struct {
	updates   *ovsdb.TableUpdates
	updates2  *ovsdb.TableUpdates2
	lastTxnID string
}

type epInfo struct {
	address  string
	serverID string
}

// ovsdbClient is an OVSDB client. Connection lifecycle and RPC are delegated to the connection manager.
type ovsdbClient struct {
	optionsMu      sync.RWMutex
	generation     atomic.Uint64
	leaderWatching bool // owned by the event loop
	options        *options
	metrics        metrics
	primaryDBName  string
	databases      map[string]*database

	errorCh    chan error
	stopCh     chan struct{} // closed when client is Close()d to stop cache and event loop
	disconnect chan struct{} // sent one value when CM sends EventDisconnected (for DisconnectNotify API)
	doneCh     chan struct{} // closed when client is Close()d so event loop exits

	handlerShutdown *sync.WaitGroup
	closeOnce       sync.Once
	eventLoopWg     sync.WaitGroup // waited in Close() so event loop and error handler exit before stopCh is closed

	cm          *ConnectionManager
	lifecycleCh chan Event // receives EventDisconnected/Reconnected/SchemaData; blocking send from CM
	eventCh     chan Event // receives EventInboundRPC; non-blocking send from CM
	logger      *logr.Logger

	// currentEndpointServerID is set on Connect/Reconnected so watchForLeaderChange can use it without blocking on CM.
	currentEndpointServerIDMu sync.Mutex
	currentEndpointServerID   string
}

// database is everything needed to map between go types and an ovsdb Database
type database struct {
	connected func() bool
	stopped   <-chan struct{}
	// model encapsulates the database schema and model of the database we're connecting to
	model model.DatabaseModel
	// modelMutex protects model from being replaced (via reconnect) while in use
	modelMutex sync.RWMutex

	// cache is used to store the updates for monitored tables
	cache *cache.TableCache
	// cacheMutex protects cache from being replaced (via reconnect) while in use
	cacheMutex sync.RWMutex

	api API

	// any ongoing monitors, so we can re-create them if we disconnect
	monitors      map[string]*Monitor
	monitorsMutex sync.Mutex

	// tracks any outstanding updates while waiting for a monitor response
	deferUpdates    bool
	deferredUpdates []*bufferedUpdate
}

// NewOVSDBClient creates a new OVSDB Client with the provided
// database model. The client can be configured using one or more Option(s),
// like WithTLSConfig. If no WithEndpoint option is supplied, the default of
// unix:/var/run/openvswitch/ovsdb.sock is used
func NewOVSDBClient(clientDBModel model.ClientDBModel, opts ...Option) (Client, error) {
	return newOVSDBClient(clientDBModel, opts...)
}

// newOVSDBClient creates a new ovsdbClient
func newOVSDBClient(clientDBModel model.ClientDBModel, opts ...Option) (*ovsdbClient, error) {
	ovs := &ovsdbClient{
		primaryDBName: clientDBModel.Name(),
		databases: map[string]*database{
			clientDBModel.Name(): {
				model:           model.NewPartialDatabaseModel(clientDBModel),
				monitors:        make(map[string]*Monitor),
				deferUpdates:    true,
				deferredUpdates: make([]*bufferedUpdate, 0),
			},
		},
		errorCh:         make(chan error),
		handlerShutdown: &sync.WaitGroup{},
		disconnect:      make(chan struct{}, 1), // buffered so one DisconnectNotify is never dropped
		doneCh:          make(chan struct{}),
		stopCh:          make(chan struct{}),
		lifecycleCh:     make(chan Event, 8),
		eventCh:         make(chan Event, 64),
	}
	var err error
	ovs.options, err = newOptions(opts...)
	if err != nil {
		return nil, err
	}

	if ovs.options.logger == nil {
		logger := logr.Discard()
		ovs.logger = &logger
	} else {
		l := ovs.options.logger.WithValues("database", ovs.primaryDBName)
		ovs.logger = &l
	}
	ovs.metrics.init(clientDBModel.Name(), ovs.options.metricNamespace, ovs.options.metricSubsystem)
	ovs.registerMetrics()

	if ovs.options.leaderOnly {
		sm, err := serverdb.FullDatabaseModel()
		if err != nil {
			return nil, fmt.Errorf("could not initialize model _Server: %w", err)
		}
		ovs.databases[serverDB] = &database{
			model:    model.NewPartialDatabaseModel(sm),
			monitors: make(map[string]*Monitor),
		}
	}

	dbNames := make([]string, 0, len(ovs.databases))
	for name := range ovs.databases {
		dbNames = append(dbNames, name)
	}
	ovs.cm = NewConnectionManager(ovs.options, ovs.primaryDBName, dbNames, ovs.lifecycleCh, ovs.eventCh)
	ovs.cm.validateSchema = ovs.validateSchema
	ovs.cm.requireReady = true
	ovs.cm.disconnectNotify = ovs.disconnect
	for _, db := range ovs.databases {
		db.connected = ovs.Connected
		db.stopped = ovs.doneCh
	}
	go ovs.cm.Run()
	ovs.eventLoopWg.Add(2)
	go func() {
		defer ovs.eventLoopWg.Done()
		ovs.runEventLoop()
	}()
	go func() {
		defer ovs.eventLoopWg.Done()
		ovs.runErrorHandler()
	}()

	return ovs, nil
}

// runEventLoop dispatches events from the connection manager.
// Lifecycle events (lifecycleCh) are given priority over inbound updates (eventCh).
func (o *ovsdbClient) runEventLoop() {
	for {
		// Drain any pending lifecycle event before blocking on updates.
		select {
		case <-o.doneCh:
			return
		case ev := <-o.lifecycleCh:
			o.dispatchEvent(ev)
			continue
		default:
		}
		select {
		case <-o.doneCh:
			return
		case ev := <-o.lifecycleCh:
			o.dispatchEvent(ev)
		case ev := <-o.eventCh:
			o.dispatchEvent(ev)
		}
	}
}

// dispatchEvent processes a single event from the connection manager.
func (o *ovsdbClient) dispatchEvent(ev Event) {
	switch ev.Type {
	case EventDisconnected:
		o.metrics.numDisconnects.Inc()
		// Retain the cache and its applied transaction cursor for incremental replay.
		// Reads remain gated until every monitor has been restored.
		for _, db := range o.databases {
			db.cacheMutex.Lock()
			db.deferUpdates = true
			db.deferredUpdates = make([]*bufferedUpdate, 0)
			db.cacheMutex.Unlock()
		}
		select {
		case o.disconnect <- struct{}{}:
		default:
			// Buffer full or no listener; leave previous value for next receiver
		}
	case EventReconnected, EventSchemaData:
		if ev.Generation != o.cm.generation.Load() {
			return
		}
		o.generation.Store(ev.Generation)
		o.setCurrentEndpointServerID(ev.EndpointServerID)
		ctx := ev.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		if timeout := o.getOptions().timeout; timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		if ev.FullSync {
			o.invalidateMonitorCursors()
		}
		err := o.applySchema(ev.Schema)
		if err == nil {
			err = o.reestablishMonitors(ctx)
		}
		if err == nil && o.getOptions().leaderOnly {
			err = o.watchForLeaderChange(ctx)
		}
		if err != nil {
			o.invalidateMonitorCursors()
		}
		o.cm.Do(context.Background(), Command{Type: CmdRestoreDone, Generation: ev.Generation, Err: err})
	case EventBarrier:
		close(ev.Processed)
	case EventInboundRPC:
		if ev.Inbound != nil && ev.Generation == o.generation.Load() && ev.Generation == o.cm.generation.Load() {
			o.applyInboundRPC(ev.Inbound)
		}
	}
}

// validateSchema rejects incompatible endpoints before selecting a connection.
// Do not publish models or mutate retained caches while evaluating candidates.
func (o *ovsdbClient) validateSchema(schemas SchemaPayload) error {
	for name, schema := range schemas {
		db := o.databases[name]
		db.modelMutex.RLock()
		_, errs := model.NewDatabaseModel(schema, db.model.Client())
		db.modelMutex.RUnlock()
		if len(errs) > 0 {
			return fmt.Errorf("schema validation failed for %s: %v", name, errs)
		}
	}
	return nil
}

// applySchema validates models before monitor restoration. Existing cache data
// remains available internally for incremental replay; public reads are gated.
func (o *ovsdbClient) applySchema(schemaPayload SchemaPayload) error {
	for dbName, schema := range schemaPayload {
		db, ok := o.databases[dbName]
		if !ok {
			continue
		}
		db.modelMutex.Lock()
		dbModel, errs := model.NewDatabaseModel(schema, db.model.Client())
		if len(errs) == 0 {
			db.model = dbModel
		}
		db.modelMutex.Unlock()
		if len(errs) > 0 {
			return fmt.Errorf("schema validation failed for %s: %v", dbName, errs)
		}
		db.cacheMutex.Lock()
		if db.cache == nil {
			var err error
			db.cache, err = cache.NewTableCache(dbModel, nil, o.logger)
			if err != nil {
				db.cacheMutex.Unlock()
				return err
			}
			dbNameForWait := dbName
			dbForWait := db
			db.api = newAPI(db.cache, o.logger, o.getOptions().validateModel, func(ctx context.Context) func() {
				waitForCacheConsistent(ctx, dbForWait, o.logger, dbNameForWait)
				return dbForWait.cacheMutex.RUnlock
			})
			o.handlerShutdown.Add(1)
			go func() {
				defer o.handlerShutdown.Done()
				db.cache.Run(o.stopCh)
			}()
		}
		db.deferUpdates = true
		db.deferredUpdates = nil
		db.cacheMutex.Unlock()
	}
	return nil
}

// reestablishMonitors re-sends monitor RPCs for all known monitors (after reconnect).
// ctx should be cancelled when the client is closing so the RPC calls are unblocked.
func (o *ovsdbClient) reestablishMonitors(ctx context.Context) error {
	for dbName, db := range o.databases {
		db.monitorsMutex.Lock()
		// Multiple monitors require snapshots. Purge once, not once per monitor.
		if len(db.monitors) != 1 {
			db.modelMutex.RLock()
			m := db.model
			db.modelMutex.RUnlock()
			db.cacheMutex.Lock()
			db.cache.Purge(m)
			db.cacheMutex.Unlock()
		}
		for id, mon := range db.monitors {
			if err := o.monitor(ctx, MonitorCookie{DatabaseName: dbName, ID: id}, true, mon); err != nil {
				db.monitorsMutex.Unlock()
				return err
			}
		}
		db.monitorsMutex.Unlock()
	}
	// Publish consistency only after all databases/monitors succeeded. Leave
	// unmonitored databases buffering until their first Monitor response is applied.
	for _, db := range o.databases {
		db.monitorsMutex.Lock()
		monitored := len(db.monitors) > 0
		db.monitorsMutex.Unlock()
		db.cacheMutex.Lock()
		db.deferUpdates = !monitored
		db.cacheMutex.Unlock()
	}
	return nil
}

func (o *ovsdbClient) invalidateMonitorCursors() {
	for _, db := range o.databases {
		db.monitorsMutex.Lock()
		for _, mon := range db.monitors {
			mon.LastTransactionID = emptyUUID
		}
		db.monitorsMutex.Unlock()
	}
}

// applyInboundRPC applies server-push update/update2/update3 to the cache.
func (o *ovsdbClient) applyInboundRPC(in *InboundRPCPayload) {
	var reply []any
	switch in.Method {
	case "update":
		_ = o.update(in.Args, &reply)
	case "update2":
		_ = o.update2(in.Args, &reply)
	case "update3":
		if err := o.update3(in.Args, &reply); err != nil {
			select {
			case o.errorCh <- err:
			case <-o.doneCh:
			}
		}
	}
}

// Connect establishes readiness using the current endpoints and restores existing
// monitors. It is idempotent while connected and returns ErrClosed after Close.
func (o *ovsdbClient) Connect(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "Connect")
	defer span.End()
	select {
	case <-o.doneCh:
		return ErrClosed
	default:
	}
	r := o.cm.Do(ctx, Command{Type: CmdConnect})
	select {
	case <-o.doneCh:
		return ErrClosed
	default:
	}
	return r.Err
}

func (o *ovsdbClient) primaryDB() *database {
	return o.databases[o.primaryDBName]
}

// Schema returns the DatabaseSchema that is being used by the client
// it will be nil until a connection has been established
func (o *ovsdbClient) Schema() ovsdb.DatabaseSchema {
	db := o.primaryDB()
	db.modelMutex.RLock()
	defer db.modelMutex.RUnlock()
	return db.model.Schema
}

// Cache returns the client-owned TableCache without waiting for readiness, or nil
// before initialization. It survives reconnect, though its tables and rows may be
// replaced. After Close, the retained cache is no longer updated by this client.
func (o *ovsdbClient) Cache() *cache.TableCache {
	db := o.primaryDB()
	db.cacheMutex.RLock()
	defer db.cacheMutex.RUnlock()
	return db.cache
}

// UpdateEndpoints replaces the endpoint list without recreating the client or
// cache. It does not wait for connection readiness and is ignored after Close.
// See Client.UpdateEndpoints for connection replacement behavior.
func (o *ovsdbClient) UpdateEndpoints(endpoints []string) {
	if len(endpoints) == 0 {
		endpoints = []string{defaultUnixEndpoint}
	}
	o.cm.Do(context.Background(), Command{Type: CmdUpdateEndpoints, UpdateEndpoints: append([]string(nil), endpoints...)})
}

func (o *ovsdbClient) getOptions() options {
	o.optionsMu.RLock()
	defer o.optionsMu.RUnlock()
	return *o.options
}

// SetOption is only valid while disconnected.
func (o *ovsdbClient) SetOption(opt Option) error {
	o.optionsMu.Lock()
	defer o.optionsMu.Unlock()
	next := *o.options
	if err := opt(&next); err != nil {
		return err
	}
	r := o.cm.Do(context.Background(), Command{Type: CmdUpdateOptions, UpdateOptions: &next})
	if r.Err == nil {
		o.options = &next
	}
	return r.Err
}

// Connected reports readiness, including restoration of all existing monitors.
func (o *ovsdbClient) Connected() bool {
	select {
	case <-o.doneCh:
		return false
	default:
		return o.cm.Connected()
	}
}

func (o *ovsdbClient) CurrentEndpoint() string {
	return o.cm.Do(context.Background(), Command{Type: CmdQueryEndpoint}).EndpointAddress
}

func (o *ovsdbClient) getCurrentEndpointInfo() (string, string) {
	r := o.cm.Do(context.Background(), Command{Type: CmdQueryEndpoint})
	return r.EndpointAddress, r.EndpointServerID
}

// DisconnectNotify retains a disconnect notification until received or until
// a new connection is fully ready (including monitor restoration).
func (o *ovsdbClient) DisconnectNotify() chan struct{} {
	return o.disconnect
}

// RFC 7047 : Section 4.1.6 : Echo
func (o *ovsdbClient) echo(args []any, reply *[]any) error {
	*reply = args
	return nil
}

// RFC 7047 : Update Notification Section 4.1.6
// params is an array of length 2: [json-value, table-updates]
// - json-value: the arbitrary json-value passed when creating the Monitor, i.e. the "cookie"
// - table-updates: map of table name to table-update. Table-update is a map of uuid to (old, new) row paris
func (o *ovsdbClient) update(params []json.RawMessage, reply *[]any) error {
	ctx, span := tracer.Start(context.Background(), "update")
	defer span.End()
	cookie := MonitorCookie{}
	*reply = []any{}
	if len(params) > 2 {
		return fmt.Errorf("update requires exactly 2 args")
	}
	err := json.Unmarshal(params[0], &cookie)
	if err != nil {
		return err
	}
	var updates ovsdb.TableUpdates
	err = json.Unmarshal(params[1], &updates)
	if err != nil {
		return err
	}
	db := o.databases[cookie.DatabaseName]
	if db == nil {
		return fmt.Errorf("update: invalid database name: %s unknown", cookie.DatabaseName)
	}
	o.metrics.numUpdates.WithLabelValues(cookie.DatabaseName).Inc()
	for tableName := range updates {
		o.metrics.numTableUpdates.WithLabelValues(cookie.DatabaseName, tableName).Inc()
	}

	db.cacheMutex.Lock()
	if db.deferUpdates {
		db.deferredUpdates = append(db.deferredUpdates, &bufferedUpdate{&updates, nil, ""})
		db.cacheMutex.Unlock()
		return nil
	}
	db.cacheMutex.Unlock()

	// Update the local DB cache with the tableUpdates
	db.cacheMutex.RLock()
	err = db.cache.Update(ctx, cookie.ID, updates)
	db.cacheMutex.RUnlock()

	if err != nil {
		select {
		case o.errorCh <- err:
		case <-o.doneCh:
		}
	}

	return err
}

// update2 handling from ovsdb-server.7
func (o *ovsdbClient) update2(params []json.RawMessage, reply *[]any) error {
	ctx, span := tracer.Start(context.Background(), "update2")
	defer span.End()
	cookie := MonitorCookie{}
	*reply = []any{}
	if len(params) > 2 {
		return fmt.Errorf("update2 requires exactly 2 args")
	}
	err := json.Unmarshal(params[0], &cookie)
	if err != nil {
		return err
	}
	var updates ovsdb.TableUpdates2
	err = json.Unmarshal(params[1], &updates)
	if err != nil {
		return err
	}
	db := o.databases[cookie.DatabaseName]
	if db == nil {
		return fmt.Errorf("update: invalid database name: %s unknown", cookie.DatabaseName)
	}

	db.cacheMutex.Lock()
	if db.deferUpdates {
		db.deferredUpdates = append(db.deferredUpdates, &bufferedUpdate{nil, &updates, ""})
		db.cacheMutex.Unlock()
		return nil
	}
	db.cacheMutex.Unlock()

	// Update the local DB cache with the tableUpdates
	db.cacheMutex.RLock()
	err = db.cache.Update2(ctx, cookie, updates)
	db.cacheMutex.RUnlock()

	if err != nil {
		select {
		case o.errorCh <- err:
		case <-o.doneCh:
		}
	}

	return err
}

// update3 handling from ovsdb-server.7
func (o *ovsdbClient) update3(params []json.RawMessage, reply *[]any) error {
	ctx, span := tracer.Start(context.Background(), "update3")
	defer span.End()
	cookie := MonitorCookie{}
	*reply = []any{}
	if len(params) > 3 {
		return fmt.Errorf("update requires exactly 3 args")
	}
	err := json.Unmarshal(params[0], &cookie)
	if err != nil {
		return err
	}
	var lastTransactionID string
	err = json.Unmarshal(params[1], &lastTransactionID)
	if err != nil {
		return err
	}
	var updates ovsdb.TableUpdates2
	err = json.Unmarshal(params[2], &updates)
	if err != nil {
		return err
	}
	db := o.databases[cookie.DatabaseName]
	if db == nil {
		return fmt.Errorf("update: invalid database name: %s unknown", cookie.DatabaseName)
	}

	db.cacheMutex.Lock()
	if db.deferUpdates {
		db.deferredUpdates = append(db.deferredUpdates, &bufferedUpdate{nil, &updates, lastTransactionID})
		db.cacheMutex.Unlock()
		return nil
	}
	db.cacheMutex.Unlock()

	// Update the local DB cache with the tableUpdates
	db.cacheMutex.RLock()
	err = db.cache.Update2(ctx, cookie, updates)
	db.cacheMutex.RUnlock()

	if err == nil {
		db.monitorsMutex.Lock()
		mon := db.monitors[cookie.ID]
		if mon != nil {
			mon.LastTransactionID = lastTransactionID
		}
		db.monitorsMutex.Unlock()
	}

	return err
}

// logFromContext returns a Logger from ctx or return the default logger
func (o *ovsdbClient) logFromContext(ctx context.Context) *logr.Logger {
	if logger, err := logr.FromContext(ctx); err == nil {
		return &logger
	}
	return o.logger
}

// Transact performs the provided Operations on the database (RFC 7047 transact).
func (o *ovsdbClient) Transact(ctx context.Context, operation ...ovsdb.Operation) ([]ovsdb.OperationResult, error) {
	ctx, span := tracer.Start(ctx, "Transact")
	defer span.End()
	select {
	case <-o.doneCh:
		return nil, ErrNotConnected
	default:
	}
	logger := o.logFromContext(ctx)
	// For non-reconnect clients, fail fast when not connected.
	// For reconnect clients, skip the early exit and let the command go to the CM;
	// if rejected before submission, the polling loop waits for reconnection.
	if o.cm.reconnectExhausted.Load() || (!o.cm.Connected() && !o.getOptions().reconnect) {
		return nil, ErrNotConnected
	}
	db := o.primaryDB()
	db.modelMutex.RLock()
	schema := db.model.Schema
	db.modelMutex.RUnlock()
	if reflect.DeepEqual(schema, ovsdb.DatabaseSchema{}) {
		if o.getOptions().reconnect {
			// Schema may not be available yet during reconnect; wait for it.
			logger.V(5).Info("schema unknown, waiting for reconnection", "database", o.primaryDBName)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-o.doneCh:
					return nil, ErrNotConnected
				case <-ctx.Done():
					return nil, fmt.Errorf("%w: while awaiting reconnection", ctx.Err())
				case <-ticker.C:
					if o.cm.reconnectExhausted.Load() {
						return nil, ErrNotConnected
					}
					if o.cm.Connected() {
						return o.Transact(ctx, operation...)
					}
				}
			}
		}
		return nil, fmt.Errorf("cannot transact to database %s: schema unknown", o.primaryDBName)
	}
	if ok := schema.ValidateOperations(operation...); !ok {
		return nil, fmt.Errorf("validation failed for the operation")
	}
	var reply []ovsdb.OperationResult
	args := ovsdb.NewTransactArgs(o.primaryDBName, operation...)
	r := o.cm.Do(ctx, Command{Type: CmdCall, CallMethod: "transact", CallArgs: args, CallReply: &reply})
	if r.Err != nil {
		if r.NotSubmitted && r.Err == ErrNotConnected && o.getOptions().reconnect {
			logger.V(5).Info("blocking transaction until reconnected", "operations", fmt.Sprintf("%+v", operation))
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-o.doneCh:
					return nil, ErrNotConnected
				case <-ctx.Done():
					return nil, fmt.Errorf("%w: while awaiting reconnection", ctx.Err())
				case <-ticker.C:
					if o.cm.reconnectExhausted.Load() {
						return nil, ErrNotConnected
					}
					if o.cm.Connected() {
						return o.Transact(ctx, operation...)
					}
				}
			}
		}
		return nil, r.Err
	}
	if err := o.waitForInboundUpdates(ctx, r.Generation); err != nil {
		return nil, err
	}
	if logger.V(4).Enabled() {
		logger.V(4).Info("transacted operations", "database", o.primaryDBName)
	}
	return reply, nil
}

// waitForInboundUpdates preserves transaction/read ordering without blocking the
// RPC reader or the connection manager. Synchronous inbound handlers enqueue all
// updates preceding the transaction reply before that reply can complete. A
// barrier on the same FIFO waits for cache application, not cache event callbacks.
// Monitor restoration must not use this barrier: it runs in the event consumer.
func (o *ovsdbClient) waitForInboundUpdates(ctx context.Context, generation uint64) error {
	processed := make(chan struct{})
	select {
	case o.eventCh <- Event{Type: EventBarrier, Generation: generation, Processed: processed}:
	case <-ctx.Done():
		return ctx.Err()
	case <-o.doneCh:
		return ErrNotConnected
	}
	select {
	case <-processed:
		// A drained queue is not sufficient if its updates were discarded on
		// disconnect. Readiness and both generation checks bind this barrier
		// to the session that produced the transaction reply.
		if generation != o.cm.generation.Load() || generation != o.generation.Load() || !o.Connected() {
			return ErrNotConnected
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-o.doneCh:
		return ErrNotConnected
	}
}

// MonitorAll is a convenience method to monitor every table/column
func (o *ovsdbClient) MonitorAll(ctx context.Context) (MonitorCookie, error) {
	ctx, span := tracer.Start(ctx, "MonitorAll")
	defer span.End()
	m := newMonitor()
	db := o.primaryDB()
	db.modelMutex.RLock()
	types := db.model.Types()
	db.modelMutex.RUnlock()
	for name := range types {
		m.Tables = append(m.Tables, TableMonitor{Table: name})
	}
	return o.Monitor(ctx, m)
}

// MonitorCancel cancels a previously issued monitor request (RFC 7047 monitor_cancel).
func (o *ovsdbClient) MonitorCancel(ctx context.Context, cookie MonitorCookie) error {
	ctx, span := tracer.Start(ctx, "MonitorCancel")
	defer span.End()
	var reply ovsdb.OperationResult
	args := ovsdb.NewMonitorCancelArgs(cookie)
	r := o.cm.Do(ctx, Command{Type: CmdCall, CallMethod: "monitor_cancel", CallArgs: args, CallReply: &reply})
	if r.Err != nil {
		return r.Err
	}
	if reply.Error != "" {
		return fmt.Errorf("error while executing transaction: %s", reply.Error)
	}
	o.primaryDB().monitorsMutex.Lock()
	delete(o.primaryDB().monitors, cookie.ID)
	o.primaryDB().monitorsMutex.Unlock()
	o.metrics.numMonitors.Dec()
	return nil
}

// Monitor will provide updates for a given table/column
// and populate the cache with them. Subsequent updates will be processed
// by the Update Notifications
// RFC 7047 : monitor
func (o *ovsdbClient) Monitor(ctx context.Context, monitor *Monitor) (MonitorCookie, error) {
	ctx, span := tracer.Start(ctx, "Monitor")
	defer span.End()
	if !o.Connected() {
		return MonitorCookie{}, ErrNotConnected
	}
	cookie := newMonitorCookie(o.primaryDBName)
	db := o.databases[o.primaryDBName]
	db.monitorsMutex.Lock()
	defer db.monitorsMutex.Unlock()
	return cookie, o.monitor(ctx, cookie, false, monitor)
}

// If fields is provided, the request will be constrained to the provided columns
// If no fields are provided, all columns will be used
func newMonitorRequest(data *mapper.Info, fields []string, conditions []ovsdb.Condition) (*ovsdb.MonitorRequest, error) {
	var columns []string
	if len(fields) > 0 {
		columns = append(columns, fields...)
	} else {
		for c := range data.Metadata.TableSchema.Columns {
			columns = append(columns, c)
		}
	}
	return &ovsdb.MonitorRequest{Columns: columns, Where: conditions, Select: ovsdb.NewDefaultMonitorSelect()}, nil
}

// sendMonitorRPC sends a monitor RPC via the connection manager and returns tableUpdates and lastTransactionFound.
func (o *ovsdbClient) sendMonitorRPC(ctx context.Context, _ MonitorCookie, monitor *Monitor, args []any) (any, bool, error) {
	var reply any
	switch monitor.Method {
	case ovsdb.MonitorRPC:
		reply = new(ovsdb.TableUpdates)
	case ovsdb.ConditionalMonitorRPC:
		reply = new(ovsdb.TableUpdates2)
	case ovsdb.ConditionalMonitorSinceRPC:
		reply = new(ovsdb.MonitorCondSinceReply)
	default:
		return nil, false, fmt.Errorf("unsupported monitor method: %v", monitor.Method)
	}
	r := o.cm.Do(ctx, Command{Type: CmdCall, Generation: o.generation.Load(), CallMethod: monitor.Method, CallArgs: args, CallReply: reply})
	if r.Err != nil {
		return nil, false, r.Err
	}
	switch v := reply.(type) {
	case *ovsdb.TableUpdates:
		return *v, false, nil
	case *ovsdb.TableUpdates2:
		return *v, false, nil
	case *ovsdb.MonitorCondSinceReply:
		monitor.LastTransactionID = v.LastTransactionID
		return v.Updates, v.Found, nil
	}
	panic("unreachable")
}

// monitor must only be called with a lock on monitorsMutex
//
//gocyclo:ignore
func (o *ovsdbClient) monitor(ctx context.Context, cookie MonitorCookie, reconnecting bool, monitor *Monitor) error {
	ctx, span := tracer.Start(ctx, "monitor")
	defer span.End()
	if !o.cm.Connected() && o.cm.State() != StateRestoring {
		return ErrNotConnected
	}
	if len(monitor.Errors) != 0 {
		var errString []string
		for _, e := range monitor.Errors {
			errString = append(errString, e.Error())
		}
		return errors.New(strings.Join(errString, ". "))
	}
	if len(monitor.Tables) == 0 {
		return errors.New("at least one table should be monitored")
	}
	dbName := cookie.DatabaseName
	db := o.databases[dbName]
	db.modelMutex.RLock()
	typeMap := db.model.Types()
	requests := make(map[string]ovsdb.MonitorRequest)
	for _, t := range monitor.Tables {
		_, ok := typeMap[t.Table]
		if !ok {
			db.modelMutex.RUnlock()
			return fmt.Errorf("type for table %s does not exist in model", t.Table)
		}
		model, err := db.model.NewModel(t.Table)
		if err != nil {
			db.modelMutex.RUnlock()
			return err
		}
		info, err := db.model.NewModelInfo(model)
		if err != nil {
			db.modelMutex.RUnlock()
			return err
		}
		request, err := newMonitorRequest(info, t.Fields, t.Conditions)
		if err != nil {
			db.modelMutex.RUnlock()
			return err
		}
		requests[t.Table] = *request
	}
	db.modelMutex.RUnlock()

	var args []any
	if monitor.Method == ovsdb.ConditionalMonitorSinceRPC {
		transactionID := emptyUUID
		if reconnecting && len(db.monitors) == 1 {
			transactionID = monitor.LastTransactionID
		}
		args = ovsdb.NewMonitorCondSinceArgs(dbName, cookie, requests, transactionID)
	} else {
		args = ovsdb.NewMonitorArgs(dbName, cookie, requests)
	}

	tableUpdates, lastTransactionFound, err := o.sendMonitorRPC(ctx, cookie, monitor, args)
	if err != nil {
		if err.Error() == "unknown method" {
			if monitor.Method == ovsdb.ConditionalMonitorSinceRPC {
				o.logger.V(3).Error(err, "method monitor_cond_since not supported, falling back to monitor_cond")
				monitor.Method = ovsdb.ConditionalMonitorRPC
				return o.monitor(ctx, cookie, reconnecting, monitor)
			}
			if monitor.Method == ovsdb.ConditionalMonitorRPC {
				o.logger.V(3).Error(err, "method monitor_cond not supported, falling back to monitor")
				monitor.Method = ovsdb.MonitorRPC
				return o.monitor(ctx, cookie, reconnecting, monitor)
			}
		}
		return err
	}

	if !reconnecting {
		db.monitors[cookie.ID] = monitor
		o.metrics.numMonitors.Inc()
	}

	db.cacheMutex.Lock()
	defer db.cacheMutex.Unlock()

	// On reconnect, purge the cache _unless_ the only monitor is a
	// MonitorCondSince one, whose LastTransactionID was known to the
	// server. In this case the reply contains only updates to the existing
	// cache data, while otherwise it includes complete DB data so we must
	// purge to get rid of old rows.
	if reconnecting && len(db.monitors) == 1 && !lastTransactionFound {
		db.modelMutex.RLock()
		m := db.model
		db.modelMutex.RUnlock()
		db.cache.Purge(m)
	}

	if monitor.Method == ovsdb.MonitorRPC {
		u := tableUpdates.(ovsdb.TableUpdates)
		err = db.cache.Populate(ctx, u)
	} else {
		u := tableUpdates.(ovsdb.TableUpdates2)
		err = db.cache.Populate2(ctx, u)
	}

	if err != nil {
		return err
	}

	// populate any deferred updates
	if !reconnecting {
		db.deferUpdates = false
	}
	for _, update := range db.deferredUpdates {
		if update.updates != nil {
			if err = db.cache.Populate(ctx, *update.updates); err != nil {
				return err
			}
		}

		if update.updates2 != nil {
			if err = db.cache.Populate2(ctx, *update.updates2); err != nil {
				return err
			}
		}
		if len(update.lastTxnID) > 0 {
			db.monitors[cookie.ID].LastTransactionID = update.lastTxnID
		}
	}
	// clear deferred updates for next time
	db.deferredUpdates = make([]*bufferedUpdate, 0)

	return err
}

// Echo tests the liveness of the OVSDB connection.
func (o *ovsdbClient) Echo(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "Echo")
	defer span.End()
	args := ovsdb.NewEchoArgs()
	var reply []any
	r := o.cm.Do(ctx, Command{Type: CmdCall, CallMethod: "echo", CallArgs: args, CallReply: &reply})
	if r.Err != nil {
		return r.Err
	}
	if !reflect.DeepEqual(args, reply) {
		return fmt.Errorf("incorrect server response: %v, %v", args, reply)
	}
	return nil
}

func (o *ovsdbClient) setCurrentEndpointServerID(sid string) {
	o.currentEndpointServerIDMu.Lock()
	defer o.currentEndpointServerIDMu.Unlock()
	o.currentEndpointServerID = sid
}

func (o *ovsdbClient) getCurrentEndpointServerID() string {
	o.currentEndpointServerIDMu.Lock()
	defer o.currentEndpointServerIDMu.Unlock()
	return o.currentEndpointServerID
}

// watchForLeaderChange sends CmdNotLeader to the connection manager when the connected endpoint loses leadership.
func (o *ovsdbClient) watchForLeaderChange(ctx context.Context) error {
	db := o.databases[serverDB]
	if !o.leaderWatching {
		db.cache.AddEventHandler(&cache.EventHandlerFuncs{UpdateFunc: func(table string, _, n model.Model) {
			info, ok := n.(*serverdb.Database)
			if !ok || table != "Database" || info.Name != o.primaryDBName || info.Model != serverdb.DatabaseModelClustered || info.Sid == nil {
				return
			}
			if *info.Sid == o.getCurrentEndpointServerID() && !info.Leader && o.Connected() {
				o.cm.Do(context.Background(), Command{Type: CmdNotLeader})
			}
		}})
		o.leaderWatching = true
	}
	db.monitorsMutex.Lock()
	defer db.monitorsMutex.Unlock()
	if len(db.monitors) > 0 {
		return nil
	}
	m := newMonitor()
	m.Method = ovsdb.ConditionalMonitorRPC
	m.Tables = []TableMonitor{{Table: "Database"}}
	return o.monitor(ctx, newMonitorCookie(serverDB), false, m)
}

// runErrorHandler processes cache errors; on inconsistency it triggers Disconnect (CM will reconnect).
func (o *ovsdbClient) runErrorHandler() {
	var errColumnNotFound *mapper.ErrColumnNotFound
	var errCacheInconsistent *cache.ErrCacheInconsistent
	var errIndexExists *cache.ErrIndexExists
	for {
		select {
		case <-o.doneCh:
			return
		case err, ok := <-o.errorCh:
			if !ok {
				return
			}
			if errors.As(err, &errColumnNotFound) {
				o.logger.V(3).Error(err, "error updating cache, DB schema may be newer than client!")
			} else if errors.As(err, &errCacheInconsistent) || errors.As(err, &errIndexExists) {
				o.logger.V(3).Error(err, "triggering reconnect to rebuild cache")
				for _, db := range o.databases {
					db.monitorsMutex.Lock()
					for _, mon := range db.monitors {
						mon.LastTransactionID = emptyUUID
					}
					db.monitorsMutex.Unlock()
				}
				o.Disconnect()
			} else {
				o.logger.V(3).Error(err, "error updating cache")
			}
		}
	}
}

// Disconnect ends the current connection but retains the cache, monitors, and
// event handlers for reuse. Clients configured for automatic reconnect immediately
// try to reconnect; otherwise callers can invoke Connect again. Unlike Close,
// Disconnect does not dispose of the client.
func (o *ovsdbClient) Disconnect() {
	o.cm.Do(context.Background(), Command{Type: CmdDisconnect})
}

// Close permanently shuts down this client. It cancels network work before
// joining event/cache workers. Connect after Close returns ErrClosed, and endpoint
// updates cannot reopen it. Use Disconnect to reuse the client and its cache.
// Do not call Close synchronously from a cache event callback.
func (o *ovsdbClient) Close() {
	o.closeOnce.Do(func() {
		close(o.doneCh)
		o.cm.cancel()
		<-o.cm.Done()
		o.eventLoopWg.Wait()
		close(o.stopCh)
		o.handlerShutdown.Wait()
		select {
		case o.disconnect <- struct{}{}:
		default:
		}
	})
}

// Ensures the cache is consistent by evaluating that the client is connected
// and the monitor is fully setup, with the cache populated. Caller must hold
// the database's cache mutex for reading.
func isCacheConsistent(db *database) bool {
	// This works because when a client is disconnected the deferUpdates variable
	// will be set to true. deferUpdates is also protected by the db.cacheMutex.
	// When the client reconnects and then re-establishes the monitor; the final step
	// is to process all deferred updates, set deferUpdates back to false, and unlock cacheMutex
	return !db.deferUpdates && (db.connected == nil || db.connected())
}

// best effort to ensure cache is in a good state for reading. RLocks the
// database's cache before returning; caller must always unlock.
func waitForCacheConsistent(ctx context.Context, db *database, logger *logr.Logger, dbName string) {
	if !hasMonitors(db) {
		db.cacheMutex.RLock()
		return
	}
	// Check immediately as a fastpath
	db.cacheMutex.RLock()
	if isCacheConsistent(db) {
		return
	}
	db.cacheMutex.RUnlock()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-db.stopped:
			db.cacheMutex.RLock()
			return
		case <-ctx.Done():
			logger.V(3).Info("warning: unable to ensure cache consistency for reading",
				"database", dbName)
			db.cacheMutex.RLock()
			return
		case <-ticker.C:
			db.cacheMutex.RLock()
			if isCacheConsistent(db) {
				return
			}
			db.cacheMutex.RUnlock()
		}
	}
}

func hasMonitors(db *database) bool {
	db.monitorsMutex.Lock()
	defer db.monitorsMutex.Unlock()
	return len(db.monitors) > 0
}

// Client API interface wrapper functions
// We add this wrapper to allow users to access the API directly on the
// client object

// Get implements the API interface's Get function
func (o *ovsdbClient) Get(ctx context.Context, model model.Model) error {
	return o.primaryDB().api.Get(ctx, model)
}

// Create implements the API interface's Create function
func (o *ovsdbClient) Create(models ...model.Model) ([]ovsdb.Operation, error) {
	return o.primaryDB().api.Create(models...)
}

// List implements the API interface's List function
func (o *ovsdbClient) List(ctx context.Context, result any) error {
	return o.primaryDB().api.List(ctx, result)
}

// Where implements the API interface's Where function
func (o *ovsdbClient) Where(models ...model.Model) ConditionalAPI {
	return o.primaryDB().api.Where(models...)
}

// WhereAny implements the API interface's WhereAny function
func (o *ovsdbClient) WhereAny(m model.Model, conditions ...model.Condition) ConditionalAPI {
	return o.primaryDB().api.WhereAny(m, conditions...)
}

// WhereAll implements the API interface's WhereAll function
func (o *ovsdbClient) WhereAll(m model.Model, conditions ...model.Condition) ConditionalAPI {
	return o.primaryDB().api.WhereAll(m, conditions...)
}

// WhereCache implements the API interface's WhereCache function
func (o *ovsdbClient) WhereCache(predicate any) ConditionalAPI {
	return o.primaryDB().api.WhereCache(predicate)
}

// WhereCacheByUUIDs implements the API interface's WhereCacheByUUIDs function
func (o *ovsdbClient) WhereCacheByUUIDs(predicate any, uuids ...string) ConditionalAPI {
	return o.primaryDB().api.WhereCacheByUUIDs(predicate, uuids...)
}

// Select implements the API interface's Select function
func (o *ovsdbClient) Select(m model.Model, fields ...any) ([]ovsdb.Operation, error) {
	return o.primaryDB().api.Select(m, fields...)
}

// GetSelectResultsByIndex parses the results of a transaction containing select operations
// and populates the target slice with the specified select query's results.
// The index parameter specifies which select query to retrieve (0-based).
// Use index=0 for single select queries (WhereAny, WhereCache, etc.).
func (o *ovsdbClient) GetSelectResultsByIndex(ops []ovsdb.Operation, results []ovsdb.OperationResult, target interface{}, index int) error {
	_, span := tracer.Start(context.Background(), "GetSelectResultsByIndex")
	defer span.End()
	if len(ops) != len(results) {
		return fmt.Errorf("number of operations (%d) and results (%d) must match", len(ops), len(results))
	}

	// Validate target parameter
	slicePtr := reflect.ValueOf(target)
	if slicePtr.Type().Kind() != reflect.Pointer || slicePtr.IsNil() {
		return &ErrWrongType{slicePtr.Type(), "target must be a non-nil pointer to a slice of models"}
	}

	sliceVal := reflect.Indirect(slicePtr)
	if sliceVal.Type().Kind() != reflect.Slice {
		return &ErrWrongType{slicePtr.Type(), "target must be a pointer to a slice of models"}
	}

	// GetSelectResultsByIndex only accepts a pointer to a slice of pointers to models
	modelType := sliceVal.Type().Elem()
	if modelType.Kind() != reflect.Pointer {
		return &ErrWrongType{slicePtr.Type(), "target must be a pointer to a slice of model pointers"}
	}
	modelType = modelType.Elem()

	o.primaryDB().modelMutex.RLock()
	dbModel := o.primaryDB().model
	o.primaryDB().modelMutex.RUnlock()

	// Determine the target table name from the model type
	dummyModel := reflect.New(modelType).Interface().(model.Model)
	info, err := dbModel.NewModelInfo(dummyModel)
	if err != nil {
		return fmt.Errorf("failed to get model info for target type: %w", err)
	}
	targetTable := info.Metadata.TableName

	// Create a map to store merged rows (deduplicated by UUID)
	mergedRows := make(map[string]ovsdb.Row)
	mergeRows := func(result ovsdb.OperationResult) error {
		if result.Error != "" {
			return fmt.Errorf("operation error: %s: %s", result.Error, result.Details)
		}

		for _, row := range result.Rows {
			uuidVal, ok := row["_uuid"]
			if !ok {
				return fmt.Errorf("failed to get UUID from row: %v", row)
			}
			uuid, ok := uuidVal.(ovsdb.UUID)
			if !ok {
				return fmt.Errorf("failed to cast UUID from row: %v", row)
			}
			// Deduplicate by UUID - later results overwrite earlier ones
			// Note different results may have different selected columns
			mergedRows[uuid.GoUUID] = row
		}
		return nil
	}

	// Single pass to find and collect results for the target index
	currentIndex := -1
	var currentCorrelationID string
	for i, op := range ops {
		if op.Op != ovsdb.OperationSelect || op.Table != targetTable {
			continue
		}

		correlationID := ovsdb.GetCorrelationID(op)
		if correlationID != currentCorrelationID {
			currentIndex++
			currentCorrelationID = correlationID
		}

		if currentIndex < index {
			continue
		}
		if currentIndex > index {
			break
		}

		err := mergeRows(results[i])
		if err != nil {
			return err
		}
	}

	if currentIndex < index {
		return fmt.Errorf("index %d is out of range: found %d query groups for table '%s'",
			index, currentIndex+1, targetTable)
	}

	// Populate the target slice with optimized memory allocation
	resultCount := len(mergedRows)

	// Pre-allocate slice with exact capacity to avoid repeated allocations
	if sliceVal.IsNil() || sliceVal.Cap() < resultCount {
		sliceVal.Set(reflect.MakeSlice(sliceVal.Type(), resultCount, resultCount))
	} else {
		// Reuse existing slice but set to exact length
		sliceVal.SetLen(resultCount)
	}

	// Use index-based assignment to avoid append overhead
	var i int
	for uuid, row := range mergedRows {
		model, err := model.CreateModel(dbModel, targetTable, &row, uuid)
		if err != nil {
			return fmt.Errorf("failed to create model: %w", err)
		}
		sliceVal.Index(i).Set(reflect.ValueOf(model))
		i++
	}

	return nil
}

// GetSelectResults parses select operation results from a transaction.
// Equivalent to GetSelectResultsByIndex with index 0 (first select query)
func (o *ovsdbClient) GetSelectResults(ops []ovsdb.Operation, results []ovsdb.OperationResult, target interface{}) error {
	_, span := tracer.Start(context.Background(), "GetSelectResults")
	defer span.End()
	return o.GetSelectResultsByIndex(ops, results, target, 0)
}

var tracer = otel.Tracer("github.com/ovn-kubernetes/libovsdb/client")
