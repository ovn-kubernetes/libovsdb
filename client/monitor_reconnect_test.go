package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

type reconnectExtraRow struct {
	UUID string `ovsdb:"_uuid"`
	Name string `ovsdb:"name"`
}

func reconnectModel(t *testing.T, extra bool) (model.ClientDBModel, ovsdb.DatabaseSchema) {
	t.Helper()
	models := map[string]model.Model{"Row": &compatServerRow{}}
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Reconnect","tables":{"Row":{"isRoot":true,"columns":{"name":{"type":"string"}}}}}`), &schema))
	if extra {
		models["Extra"] = &reconnectExtraRow{}
		schema.Tables["Extra"] = schema.Tables["Row"]
	}
	m, err := model.NewClientDBModel(schema.Name, models)
	require.NoError(t, err)
	return m, schema
}

func reconnectTransact(ctx context.Context, t *testing.T, cli Client, op ovsdb.Operation) []ovsdb.OperationResult {
	t.Helper()
	results, err := cli.Transact(ctx, op)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(results, []ovsdb.Operation{op})
	require.NoError(t, err)
	require.Empty(t, opErrs)
	return results
}

// Each server returns a full snapshot (found=false). Incremental resume across
// schema changes is deliberately outside this test's contract.
func TestMonitorReconnectSchemaGrowth(t *testing.T) {
	for _, all := range []bool{true, false} {
		name := "explicit"
		if all {
			name = "all"
		}
		t.Run(name, func(t *testing.T) { testMonitorReconnectSchemaGrowth(t, all) })
	}
}

func testMonitorReconnectSchemaGrowth(t *testing.T, all bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	oldModel, oldSchema := reconnectModel(t, false)
	_, oldSocket := newOVSDBServer(t, oldModel, oldSchema)
	fullModel, fullSchema := reconnectModel(t, true)
	_, newSocket := newOVSDBServer(t, fullModel, fullSchema)

	writer, err := NewOVSDBClient(fullModel, WithEndpoint("unix:"+newSocket))
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Connect(ctx))
	results := reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Extra", Row: ovsdb.Row{"name": "initial"}})
	uuid := results[0].UUID.GoUUID
	results = reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Row", Row: ovsdb.Row{"name": "initial"}})
	rowUUID := results[0].UUID.GoUUID

	cli, err := NewOVSDBClient(fullModel, WithEndpoint("unix:"+oldSocket), WithCompatibility(true), WithReconnect(time.Second, &backoff.ConstantBackOff{Interval: 10 * time.Millisecond}))
	require.NoError(t, err)
	defer cli.Close()
	require.NoError(t, cli.Connect(ctx))
	if all {
		_, err = cli.MonitorAll(ctx)
	} else {
		_, err = cli.Monitor(ctx, cli.NewMonitor(WithTable(&compatServerRow{})))
	}
	require.NoError(t, err)
	require.Nil(t, cli.Cache().Table("Extra"))

	cli.UpdateEndpoints([]string{"unix:" + newSocket})
	require.Eventually(t, func() bool {
		if !cli.Connected() || cli.CurrentEndpoint() != "unix:"+newSocket {
			return false
		}
		return cli.Cache().Table("Row").Row(rowUUID) != nil
	}, 3*time.Second, 10*time.Millisecond)
	if !all {
		// A new table cache can exist without a subscription. Its seeded row
		// must not appear merely because the runtime schema now supports it.
		require.Empty(t, cli.Cache().Table("Extra").Rows())
		reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationUpdate, Table: "Row", Row: ovsdb.Row{"name": "updated"}})
		require.Eventually(t, func() bool {
			return cli.Cache().Table("Row").Row(rowUUID).(*compatServerRow).Name == "updated"
		}, time.Second, 10*time.Millisecond)
		require.Empty(t, cli.Cache().Table("Extra").Rows())
		return
	}
	require.Equal(t, "initial", cli.Cache().Table("Extra").Row(uuid).(*reconnectExtraRow).Name)

	reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationUpdate, Table: "Extra", Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: uuid})}, Row: ovsdb.Row{"name": "updated"}})
	require.Eventually(t, func() bool {
		return cli.Cache().Table("Extra").Row(uuid).(*reconnectExtraRow).Name == "updated"
	}, time.Second, 10*time.Millisecond)
}

func TestMonitorAllReconnectSchemaShrink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fullModel, fullSchema := reconnectModel(t, true)
	_, oldSocket := newOVSDBServer(t, fullModel, fullSchema)
	smallModel, smallSchema := reconnectModel(t, false)
	_, newSocket := newOVSDBServer(t, smallModel, smallSchema)
	writer, err := NewOVSDBClient(smallModel, WithEndpoint("unix:"+newSocket))
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Connect(ctx))
	results := reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Row", Row: ovsdb.Row{"name": "new-server"}})
	uuid := results[0].UUID.GoUUID

	cli, err := NewOVSDBClient(fullModel, WithEndpoint("unix:"+oldSocket), WithCompatibility(true), WithReconnect(time.Second, &backoff.ConstantBackOff{Interval: 10 * time.Millisecond}))
	require.NoError(t, err)
	defer cli.Close()
	require.NoError(t, cli.Connect(ctx))
	reconnectTransact(ctx, t, cli, ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Row", Row: ovsdb.Row{"name": "old-server"}})
	reconnectTransact(ctx, t, cli, ovsdb.Operation{Op: ovsdb.OperationInsert, Table: "Extra", Row: ovsdb.Row{"name": "removed"}})
	_, err = cli.MonitorAll(ctx)
	require.NoError(t, err)
	require.Len(t, cli.Cache().Table("Extra").Rows(), 1)

	cli.UpdateEndpoints([]string{"unix:" + newSocket})
	require.Eventually(t, func() bool {
		return cli.Connected() && cli.CurrentEndpoint() == "unix:"+newSocket
	}, 3*time.Second, 10*time.Millisecond)
	require.Nil(t, cli.Cache().Table("Extra"))
	require.Len(t, cli.Cache().Table("Row").Rows(), 1)
	require.Equal(t, "new-server", cli.Cache().Table("Row").Row(uuid).(*compatServerRow).Name)
	reconnectTransact(ctx, t, writer, ovsdb.Operation{Op: ovsdb.OperationUpdate, Table: "Row", Row: ovsdb.Row{"name": "updated"}})
	require.Eventually(t, func() bool {
		return cli.Cache().Table("Row").Row(uuid).(*compatServerRow).Name == "updated"
	}, time.Second, 10*time.Millisecond)
}

func TestExplicitMonitorReplayRejectsRemovedTable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fullModel, fullSchema := reconnectModel(t, true)
	_, oldSocket := newOVSDBServer(t, fullModel, fullSchema)
	oldClient, err := NewOVSDBClient(fullModel, WithEndpoint("unix:"+oldSocket))
	require.NoError(t, err)
	defer oldClient.Close()
	require.NoError(t, oldClient.Connect(ctx))
	selection := oldClient.NewMonitor(WithTable(&reconnectExtraRow{}))
	cookie, err := oldClient.Monitor(ctx, selection)
	require.NoError(t, err)

	smallModel, smallSchema := reconnectModel(t, false)
	_, newSocket := newOVSDBServer(t, smallModel, smallSchema)
	cli, err := newOVSDBClient(fullModel, WithEndpoint("unix:"+newSocket), WithCompatibility(true))
	require.NoError(t, err)
	defer cli.Close()
	require.NoError(t, cli.Connect(ctx))

	// Exercise the reconnect replay entry point directly: the automatic
	// reconnect loop retries failures indefinitely instead of returning them.
	cli.rpcMutex.Lock()
	cli.primaryDB().monitorsMutex.Lock()
	err = cli.monitor(ctx, cookie, true, selection)
	cli.primaryDB().monitorsMutex.Unlock()
	cli.rpcMutex.Unlock()
	require.EqualError(t, err, "type for table Extra does not exist in model")
	require.Equal(t, []TableMonitor{{Table: "Extra"}}, selection.Tables)
}
