package client

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/rpc2"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestTransactWaitsForPrecedingMonitorUpdates(t *testing.T) {
	for _, method := range []string{"update", "update2", "update3"} {
		t.Run(method, func(t *testing.T) {
			cookies := make(chan MonitorCookie, 1)
			notified := make(chan struct{})
			endpoint := lifecycleServer(t, func(p *rpc2.Client) {
				p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, args []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
					var cookie MonitorCookie
					if err := json.Unmarshal(args[1], &cookie); err != nil {
						return err
					}
					cookies <- cookie
					*reply = ovsdb.MonitorCondSinceReply{LastTransactionID: aUUID1, Updates: bridgeSnapshot("before")}
					return nil
				})
				p.Handle("transact", func(_ *rpc2.Client, _ []json.RawMessage, reply *[]ovsdb.OperationResult) error {
					cookie := <-cookies
					old, row := ovsdb.Row{"datapath_type": ""}, ovsdb.Row{"datapath_type": "netdev"}
					updates := ovsdb.TableUpdates2{"Bridge": {aUUID0: &ovsdb.RowUpdate2{Modify: &row}}}
					args := []any{cookie, updates}
					switch method {
					case "update":
						args[1] = ovsdb.TableUpdates{"Bridge": {aUUID0: &ovsdb.RowUpdate{Old: &old, New: &row}}}
					case "update3":
						args = []any{cookie, aUUID2, updates}
					}
					if err := p.Notify(method, args); err != nil {
						return err
					}
					close(notified)
					*reply = []ovsdb.OperationResult{{}}
					return nil
				})
			})
			o := lifecycleClient(t, endpoint, false)
			monitorBridge(t, o)

			// Hold cache application, not transport receipt: the peer sends an
			// update before its transaction reply, as an OVSDB server must.
			db := o.primaryDB()
			db.cacheMutex.Lock()
			var once sync.Once
			release := func() { once.Do(db.cacheMutex.Unlock) }
			defer release()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := o.Transact(ctx, ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"})
				done <- err
			}()
			select {
			case <-notified:
			case <-ctx.Done():
				t.Fatal("peer did not send the update")
			}
			select {
			case err := <-done:
				t.Fatalf("Transact returned before its preceding monitor update was applied: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			require.NoError(t, o.Echo(ctx), "pending cache application must not block other RPCs")
			release()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("Transact did not complete after applying the update")
			}
			row := &Bridge{UUID: aUUID0}
			require.NoError(t, o.Get(ctx, row))
			require.Equal(t, "netdev", row.DatapathType)
		})
	}
}

func TestInboundBarrierCancellation(t *testing.T) {
	for _, phase := range []string{"enqueue", "apply"} {
		for _, stop := range []string{"context", "shutdown"} {
			t.Run(phase+"/"+stop, func(t *testing.T) {
				// An unbuffered queue lets the test hold either enqueueing or
				// processing without a worker or scheduler-dependent sleeps.
				o := &ovsdbClient{eventCh: make(chan Event), doneCh: make(chan struct{})}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- o.waitForInboundUpdates(ctx, 0) }()
				if phase == "apply" {
					select {
					case ev := <-o.eventCh:
						// Completion after cancellation must remain safe.
						defer o.dispatchEvent(ev)
					case <-time.After(time.Second):
						t.Fatal("barrier was not enqueued")
					}
				}
				want := context.Canceled
				if stop == "context" {
					cancel()
				} else {
					close(o.doneCh)
					want = ErrNotConnected
				}
				select {
				case err := <-done:
					require.ErrorIs(t, err, want)
				case <-time.After(time.Second):
					t.Fatal("barrier ignored cancellation")
				}
			})
		}
	}
}
