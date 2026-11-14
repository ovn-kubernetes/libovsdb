package client

import (
	"context"
	"testing"
	"time"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/updates"
	"github.com/stretchr/testify/require"
)

// Reproduce #448's 23.03 client / 25.09 server combination. Initial rows
// already ignore unknown model fields; subsequent updates must do so too.
func TestOVNForwardCompatibilityUpdates(t *testing.T) {
	clientSchema := ovnReleaseSchema(t, "23.03", "nb")
	serverSchema := ovnReleaseSchema(t, "25.09", "nb")
	require.Equal(t, "7.0.0", clientSchema.Version)
	require.Nil(t, clientSchema.Table("Logical_Router_Port").Column("status"))
	require.NotNil(t, serverSchema.Table("Logical_Router_Port").Column("status"))
	clientModel := ovnReleaseModel(t, clientSchema)
	db, errs := model.NewDatabaseModel(serverSchema, clientModel)
	require.Empty(t, errs)
	status, err := ovsdb.NewOvsMap(map[string]string{"hosting-chassis": "chassis-1"})
	require.NoError(t, err)
	const table = "Logical_Router_Port"
	const uuid = "00000000-0000-0000-0000-000000000001"
	for _, kind := range []string{"initial", "insert", "modify", "update"} {
		for _, onlyUnknown := range []bool{false, true} {
			name := kind + "/mixed"
			if onlyUnknown {
				name = kind + "/unknown_only"
			}
			t.Run(name, func(t *testing.T) {
				current := ovnReleaseRow(clientModel, table)
				ovnColumn(current, "_uuid").SetString(uuid)
				ovnColumn(current, "name").SetString("before")
				change := ovsdb.Row{"status": status}
				if !onlyUnknown {
					change["name"] = "after"
				}
				var result updates.ModelUpdates
				switch kind {
				case "initial":
					err = result.AddRowUpdate2(db, table, uuid, nil, ovsdb.RowUpdate2{Initial: &change})
				case "insert":
					err = result.AddRowUpdate2(db, table, uuid, nil, ovsdb.RowUpdate2{Insert: &change})
				case "modify":
					err = result.AddRowUpdate2(db, table, uuid, current, ovsdb.RowUpdate2{Modify: &change})
				case "update":
					old := ovsdb.Row{"name": "before"}
					err = result.AddRowUpdate(db, table, uuid, current, ovsdb.RowUpdate{Old: &old, New: &change})
				}
				require.NoError(t, err)
				if onlyUnknown && (kind == "modify" || kind == "update") {
					require.Empty(t, result.GetUpdatedTables(), "unknown-only changes must not generate model events")
				} else {
					row := result.GetModel(table, uuid)
					require.NotNil(t, row)
					expected := "after"
					if onlyUnknown {
						expected = ""
					}
					require.Equal(t, expected, ovnColumn(row, "name").String())
				}
				require.Equal(t, "before", ovnColumn(current, "name").String(), "the original cache model must not be mutated")
			})
		}
	}
}

func TestOVNForwardCompatibilityMonitorColumns(t *testing.T) {
	serverSchema := ovnReleaseSchema(t, "25.09", "nb")
	clientModel := ovnReleaseModel(t, ovnReleaseSchema(t, "23.03", "nb"))
	db, errs := model.NewDatabaseModel(serverSchema, clientModel)
	require.Empty(t, errs)
	info, err := db.NewModelInfo(ovnReleaseRow(clientModel, "Logical_Router_Port"))
	require.NoError(t, err)
	request, err := newMonitorRequest(info, nil, nil)
	require.NoError(t, err)
	require.Contains(t, request.Columns, "name")
	require.NotContains(t, request.Columns, "status")
	for _, column := range request.Columns {
		require.Contains(t, info.Metadata.Fields, column)
	}

	// An empty column list is omitted on the wire, meaning "all columns".
	// UUID-only models must explicitly request UUID instead.
	minimal, err := model.NewClientDBModel(serverSchema.Name, map[string]model.Model{
		"Logical_Router_Port": &struct {
			UUID string `ovsdb:"_uuid"`
		}{},
	})
	require.NoError(t, err)
	db, errs = model.NewDatabaseModel(serverSchema, minimal)
	require.Empty(t, errs)
	row, err := db.NewModel("Logical_Router_Port")
	require.NoError(t, err)
	info, err = db.NewModelInfo(row)
	require.NoError(t, err)
	request, err = newMonitorRequest(info, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"_uuid"}, request.Columns)
}

func TestOVNForwardCompatibilityUUIDOnlyMonitor(t *testing.T) {
	schema := ovnReleaseSchema(t, "26.03", "sb")
	_, socket := newOVSDBServer(t, ovnReleaseModel(t, schema), schema)
	type uuidOnly struct {
		UUID string `ovsdb:"_uuid"`
	}
	clientModel, err := model.NewClientDBModel(schema.Name, map[string]model.Model{"Service_Monitor": &uuidOnly{}})
	require.NoError(t, err)
	ovs, err := NewOVSDBClient(clientModel, WithEndpoint("unix:"+socket))
	require.NoError(t, err)
	defer ovs.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, ovs.Connect(ctx))
	_, err = ovs.MonitorAll(ctx)
	require.NoError(t, err)
	ops := []ovsdb.Operation{{Op: ovsdb.OperationInsert, Table: "Service_Monitor", Row: ovsdb.Row{"logical_port": "uuid-only"}}}
	results, err := ovs.Transact(ctx, ops...)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(results, ops)
	require.NoError(t, err)
	require.Empty(t, opErrs)
	uuid := results[0].UUID.GoUUID
	require.Eventually(t, func() bool {
		row := ovs.Cache().Table("Service_Monitor").Row(uuid)
		return row != nil && row.(*uuidOnly).UUID == uuid
	}, time.Second, 10*time.Millisecond)
}

// Real server updates must keep reaching the older client's cache even when a
// transaction changes both known and newer, unmodeled columns.
func TestOVNForwardCompatibilityMonitor(t *testing.T) {
	serverSchema := ovnReleaseSchema(t, "26.03", "sb")
	_, socket := newOVSDBServer(t, ovnReleaseModel(t, serverSchema), serverSchema)
	clientModel := ovnReleaseModel(t, ovnReleaseSchema(t, "25.09", "sb"))
	for _, compatible := range []bool{false, true} {
		name := "strict"
		if compatible {
			name = "compatible"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ovs, err := NewOVSDBClient(clientModel, WithEndpoint("unix:"+socket), WithCompatibility(compatible))
			require.NoError(t, err)
			defer ovs.Close()
			require.NoError(t, ovs.Connect(ctx))
			// Seed server-only data before monitoring to cover initial cache population.
			ops := []ovsdb.Operation{{Op: ovsdb.OperationInsert, Table: "Service_Monitor", Row: ovsdb.Row{"logical_port": name, "mac": "00:00:00:00:00:01"}}}
			results, err := ovs.Transact(ctx, ops...)
			require.NoError(t, err)
			opErrs, err := ovsdb.CheckOperationResults(results, ops)
			require.NoError(t, err)
			require.Empty(t, opErrs)
			uuid := results[0].UUID.GoUUID
			_, err = ovs.MonitorAll(ctx)
			require.NoError(t, err)
			require.NotNil(t, ovs.Cache().Table("Service_Monitor").Row(uuid))
			ops = []ovsdb.Operation{{Op: ovsdb.OperationUpdate, Table: "Service_Monitor", Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: uuid})}, Row: ovsdb.Row{"logical_port": name + "-updated", "mac": "00:00:00:00:00:02"}}}
			results, err = ovs.Transact(ctx, ops...)
			require.NoError(t, err)
			opErrs, err = ovsdb.CheckOperationResults(results, ops)
			require.NoError(t, err)
			require.Empty(t, opErrs)
			require.Eventually(t, func() bool {
				row := ovs.Cache().Table("Service_Monitor").Row(uuid)
				return row != nil && ovnColumn(row, "logical_port").String() == name+"-updated"
			}, time.Second, 10*time.Millisecond)
		})
	}
}
