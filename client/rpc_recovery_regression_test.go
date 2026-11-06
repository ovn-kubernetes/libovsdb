package client

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/rpc2"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestTransactDoesNotReplayUncertainOutcome(t *testing.T) {
	var committed atomic.Int32
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle("transact", func(_ *rpc2.Client, _ []json.RawMessage, reply *[]ovsdb.OperationResult) error {
			if committed.Add(1) == 1 {
				// Commit the mutation, then lose its reply. Replaying is unsafe.
				p.Close()
			}
			*reply = []ovsdb.OperationResult{{Count: 1}}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := o.Transact(ctx, ovsdb.Operation{Op: ovsdb.OperationMutate, Table: "Bridge", Mutations: []ovsdb.Mutation{{Column: "flood_vlans", Mutator: ovsdb.MutateOperationInsert, Value: ovsdb.OvsSet{GoSet: []any{1}}}}})
	require.Error(t, err, "an unacknowledged commit must not be replayed")
	require.EqualValues(t, 1, committed.Load())
	require.Eventually(t, o.Connected, time.Second, time.Millisecond)
	// A later, explicit transaction remains usable.
	_, err = o.Transact(ctx, ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"})
	require.NoError(t, err)
	require.EqualValues(t, 2, committed.Load())
}

func TestTransactRejectsInvalidatedBarrier(t *testing.T) {
	peers := make(chan *rpc2.Client, 1)
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		peers <- p
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, _ []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("before")}
			return nil
		})
		p.Handle("transact", func(_ *rpc2.Client, _ []json.RawMessage, reply *[]ovsdb.OperationResult) error {
			*reply = []ovsdb.OperationResult{{}}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, false)
	cookie := monitorBridge(t, o)
	peer := <-peers
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	db := o.primaryDB()
	db.cacheMutex.Lock()
	var once sync.Once
	release := func() { once.Do(db.cacheMutex.Unlock) }
	defer release()
	// The first update occupies the consumer; the second must remain queued.
	var ignored any
	require.NoError(t, peer.CallWithContext(ctx, "update2", []any{cookie, ovsdb.TableUpdates2{}}, &ignored))
	require.Eventually(t, func() bool { return len(o.eventCh) == 0 }, time.Second, time.Millisecond)
	row := ovsdb.Row{"datapath_type": "netdev"}
	updates := ovsdb.TableUpdates2{"Bridge": {aUUID0: &ovsdb.RowUpdate2{Modify: &row}}}
	require.NoError(t, peer.CallWithContext(ctx, "update2", []any{cookie, updates}, &ignored))
	done := make(chan error, 1)
	go func() {
		_, err := o.Transact(ctx, ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"})
		done <- err
	}()
	require.Eventually(t, func() bool { return len(o.eventCh) == 2 }, time.Second, time.Millisecond)
	o.Disconnect()
	release()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrNotConnected)
	case <-ctx.Done():
		t.Fatal("transaction barrier did not finish")
	}
	require.Empty(t, o.Cache().Table("Bridge").Row(aUUID0).(*Bridge).DatapathType)
}

func TestConnectDeadlineIncludesMonitorRestoration(t *testing.T) {
	var monitors atomic.Int32
	entered := make(chan struct{})
	endpoint := lifecycleServer(t, func(p *rpc2.Client) {
		p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, _ []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
			if monitors.Add(1) == 2 {
				close(entered)
				<-p.DisconnectNotify()
				return nil
			}
			*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("restored")}
			return nil
		})
	})
	o := lifecycleClient(t, endpoint, false)
	monitorBridge(t, o)
	o.Disconnect()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, o.Connect(ctx), context.DeadlineExceeded)
	select {
	case <-entered:
	default:
		t.Fatal("restoration was not attempted")
	}
	require.Eventually(t, func() bool { return o.cm.State() == StateDisconnected }, time.Second, time.Millisecond)
	next, stop := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, o.Connect(next))
	stop()
	require.NoError(t, o.Echo(context.Background()), "ready session must outlive Connect context")
	require.True(t, o.Connected())
}

func TestConnectSkipsIncompatibleSchema(t *testing.T) {
	bad := lifecycleServerWithSchema(t, ovsdb.DatabaseSchema{Name: "Open_vSwitch", Tables: map[string]ovsdb.TableSchema{}}, func(_ *rpc2.Client) {})
	good := lifecycleServer(t, func(_ *rpc2.Client) {})
	o, err := newOVSDBClient(defDB, WithEndpoint(bad), WithEndpoint(good))
	require.NoError(t, err)
	defer o.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, o.Connect(ctx))
	require.Equal(t, good, o.CurrentEndpoint())
	require.NoError(t, o.Echo(ctx))
}
