package client

// Recovery scenarios derived from issue #250 and PR #283:
// https://github.com/ovn-kubernetes/libovsdb/issues/250
// https://github.com/ovn-kubernetes/libovsdb/pull/283
// Multiple monitors deliberately request full snapshots; this matrix checks
// their union, not per-monitor incremental replay or selective-purge support.

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/rpc2"
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

type recoveryScope struct {
	table  string
	ids    []string
	where  []ovsdb.Condition
	method string
}

func (scope recoveryScope) validateRequest(raw json.RawMessage) error {
	var requests map[string]ovsdb.MonitorRequest
	if err := json.Unmarshal(raw, &requests); err != nil {
		return err
	}
	if len(requests) != 1 {
		return fmt.Errorf("unexpected monitor request: %v", requests)
	}
	if _, ok := requests[scope.table]; !ok {
		return fmt.Errorf("wrong monitored table")
	}
	if len(requests[scope.table].Where) != len(scope.where) {
		return fmt.Errorf("wrong monitor conditions")
	}
	for i, condition := range scope.where {
		if !reflect.DeepEqual(condition, requests[scope.table].Where[i]) {
			return fmt.Errorf("wrong monitor condition")
		}
	}
	return nil
}

func TestMonitorRecoveryMatrix(t *testing.T) {
	for _, tc := range []struct {
		name               string
		scopes             []recoveryScope
		found              bool
		unsupportedOverlap bool
	}{
		{name: "single/incremental", scopes: []recoveryScope{{table: "Bridge", ids: []string{aUUID0}}}, found: true},
		{name: "single/full_snapshot", scopes: []recoveryScope{{table: "Bridge", ids: []string{aUUID0}}}},
		{name: "multiple/different_tables", scopes: []recoveryScope{{table: "Bridge", ids: []string{aUUID0}}, {table: "Open_vSwitch", ids: []string{aUUID1}}}},
		{name: "multiple/same_table_disjoint", scopes: []recoveryScope{
			{table: "Bridge", ids: []string{aUUID0}, where: []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "a")}},
			{table: "Bridge", ids: []string{aUUID1}, where: []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "b")}},
		}},
		{name: "multiple/same_table_overlapping", unsupportedOverlap: true, scopes: []recoveryScope{
			{table: "Bridge", ids: []string{aUUID0, aUUID2}, where: []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionNotEqual, "b")}},
			{table: "Bridge", ids: []string{aUUID1, aUUID2}, where: []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionNotEqual, "a")}},
		}},
		{name: "multiple/mixed_monitor_methods", scopes: []recoveryScope{
			{table: "Bridge", ids: []string{aUUID0}},
			{table: "Open_vSwitch", ids: []string{aUUID1}, method: ovsdb.ConditionalMonitorRPC},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recovering atomic.Bool
			var mu sync.Mutex
			cookies := map[string]int{}
			cursors := make(chan string, 16)
			endpoint := lifecycleServer(t, func(p *rpc2.Client) {
				response := func(args []json.RawMessage) (ovsdb.MonitorCondSinceReply, error) {
					var cookie MonitorCookie
					if err := json.Unmarshal(args[1], &cookie); err != nil {
						return ovsdb.MonitorCondSinceReply{}, err
					}
					mu.Lock()
					idx, ok := cookies[cookie.ID]
					if !ok {
						idx = len(cookies)
						cookies[cookie.ID] = idx
					}
					mu.Unlock()
					if idx >= len(tc.scopes) {
						return ovsdb.MonitorCondSinceReply{}, fmt.Errorf("unexpected monitor")
					}
					scope := tc.scopes[idx]
					if err := scope.validateRequest(args[2]); err != nil {
						return ovsdb.MonitorCondSinceReply{}, err
					}
					if recovering.Load() && len(args) == 4 {
						var cursor string
						if err := json.Unmarshal(args[3], &cursor); err != nil {
							return ovsdb.MonitorCondSinceReply{}, err
						}
						cursors <- cursor
					}
					found := recovering.Load() && tc.found
					updates := ovsdb.TableUpdates2{scope.table: ovsdb.TableUpdate2{}}
					// A single monitor has an unchanged row plus an obsolete row. An empty
					// delta except for the deletion must keep the former and remove the latter.
					if found {
						empty := ovsdb.Row{}
						updates[scope.table][aUUID3] = &ovsdb.RowUpdate2{Delete: &empty}
					} else {
						for _, id := range scope.ids {
							row := ovsdb.Row{"name": map[string]string{aUUID0: "a", aUUID1: "b", aUUID2: "shared"}[id], "datapath_type": "system"}
							if scope.table == "Open_vSwitch" {
								row = ovsdb.Row{"cur_cfg": float64(1)}
							}
							updates[scope.table][id] = &ovsdb.RowUpdate2{Initial: &row}
						}
						if !recovering.Load() && len(tc.scopes) == 1 {
							row := ovsdb.Row{"name": "obsolete"}
							updates[scope.table][aUUID3] = &ovsdb.RowUpdate2{Initial: &row}
						}
					}
					return ovsdb.MonitorCondSinceReply{Found: found, LastTransactionID: aUUID2, Updates: updates}, nil
				}
				p.Handle(ovsdb.ConditionalMonitorSinceRPC, func(_ *rpc2.Client, args []json.RawMessage, reply *ovsdb.MonitorCondSinceReply) error {
					r, err := response(args)
					*reply = r
					return err
				})
				p.Handle(ovsdb.ConditionalMonitorRPC, func(_ *rpc2.Client, args []json.RawMessage, reply *ovsdb.TableUpdates2) error {
					r, err := response(args)
					*reply = r.Updates
					return err
				})
			})
			o := lifecycleClient(t, endpoint, true)
			for index, scope := range tc.scopes {
				mon := newMonitor()
				mon.Method = scope.method
				if mon.Method == "" {
					mon.Method = ovsdb.ConditionalMonitorSinceRPC
				}
				mon.Tables = []TableMonitor{{Table: scope.table, Conditions: scope.where}}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				_, err := o.Monitor(ctx, mon)
				cancel()
				if tc.unsupportedOverlap && index > 0 {
					// This documents the existing unsupported configuration, not a
					// successful recovery case. Shared-row/delta ownership remains #250/#283.
					var inconsistent *cache.ErrCacheInconsistent
					require.ErrorAs(t, err, &inconsistent)
					return
				}
				require.NoError(t, err)
			}
			old := o.generation.Load()
			recovering.Store(true)
			o.Disconnect()
			require.Eventually(t, func() bool { return o.Connected() && o.generation.Load() != old }, 2*time.Second, time.Millisecond)
			expectedCounts := map[string]map[string]bool{}
			for _, scope := range tc.scopes {
				if expectedCounts[scope.table] == nil {
					expectedCounts[scope.table] = map[string]bool{}
				}
				for _, id := range scope.ids {
					expectedCounts[scope.table][id] = true
					require.NotNil(t, o.Cache().Table(scope.table).Row(id), "monitor replay lost %s/%s", scope.table, id)
				}
				if scope.method == "" {
					select {
					case cursor := <-cursors:
						if len(tc.scopes) == 1 {
							require.Equal(t, aUUID2, cursor)
						} else {
							require.Equal(t, emptyUUID, cursor, "multiple monitors must not replay deltas onto a globally purged cache")
						}
					case <-time.After(time.Second):
						t.Fatal("missing monitor replay")
					}
				}
			}
			for table, ids := range expectedCounts {
				require.Len(t, o.Cache().Table(table).Rows(), len(ids))
			}
		})
	}
}
