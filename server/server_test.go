package server

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/database/inmemory"
	"github.com/ovn-kubernetes/libovsdb/internal/json"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOvsdbServerMonitor(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	logger := logr.Discard()
	ovsDB := inmemory.NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()}, &logger)
	schema := dbModel.Schema

	o, err := NewOvsdbServer(ovsDB, &logger, dbModel)
	require.NoError(t, err)
	requests := make(map[string]ovsdb.MonitorRequest)
	for table, tableSchema := range schema.Tables {
		var columns []string
		for column := range tableSchema.Columns {
			columns = append(columns, column)
		}
		requests[table] = ovsdb.MonitorRequest{
			Columns: columns,
			Select:  ovsdb.NewDefaultMonitorSelect(),
		}
	}

	fooUUID := uuid.NewString()
	barUUID := uuid.NewString()
	bazUUID := uuid.NewString()
	quuxUUID := uuid.NewString()

	operations := []ovsdb.Operation{
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  fooUUID,
			Row:   ovsdb.Row{"name": "foo"},
		},
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  barUUID,
			Row:   ovsdb.Row{"name": "bar"},
		},
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  bazUUID,
			Row:   ovsdb.Row{"name": "baz"},
		},
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  quuxUUID,
			Row:   ovsdb.Row{"name": "quux"},
		},
	}
	transaction := ovsDB.NewTransaction("Open_vSwitch")
	_, updates := transaction.Transact(operations...)
	err = o.db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	db, err := json.Marshal("Open_vSwitch")
	require.NoError(t, err)
	value, err := json.Marshal("foo")
	require.NoError(t, err)
	rJSON, err := json.Marshal(requests)
	require.NoError(t, err)
	args := []json.RawMessage{db, value, rJSON}
	reply := &ovsdb.TableUpdates{}
	err = o.Monitor(nil, args, reply)
	require.NoError(t, err)
	expected := &ovsdb.TableUpdates{
		"Bridge": {
			fooUUID: &ovsdb.RowUpdate{
				New: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: fooUUID},
					"name":  "foo",
				},
			},
			barUUID: &ovsdb.RowUpdate{
				New: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: barUUID},
					"name":  "bar",
				},
			},
			bazUUID: &ovsdb.RowUpdate{
				New: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: bazUUID},
					"name":  "baz",
				},
			},
			quuxUUID: &ovsdb.RowUpdate{
				New: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: quuxUUID},
					"name":  "quux",
				},
			},
		},
	}
	assert.Equal(t, expected, reply)
}
