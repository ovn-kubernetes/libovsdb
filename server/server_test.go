package server

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ovn-org/libovsdb/internal/json"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandNamedUUID(t *testing.T) {
	testUUID := uuid.NewString()
	testUUID1 := uuid.NewString()
	tests := []struct {
		name       string
		namedUUIDs map[string]ovsdb.UUID
		value      interface{}
		expected   interface{}
	}{
		{
			"uuid",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}},
			ovsdb.UUID{GoUUID: "foo"},
			ovsdb.UUID{GoUUID: testUUID},
		},
		{
			"set",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}},
			ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "foo"}}},
			ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: testUUID}}},
		},
		{
			"set multiple",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}, "bar": {GoUUID: testUUID1}},
			ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "foo"}, ovsdb.UUID{GoUUID: "bar"}, ovsdb.UUID{GoUUID: "baz"}}},
			ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: testUUID}, ovsdb.UUID{GoUUID: testUUID1}, ovsdb.UUID{GoUUID: "baz"}}},
		},
		{
			"map key",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{ovsdb.UUID{GoUUID: "foo"}: "foo"}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{ovsdb.UUID{GoUUID: testUUID}: "foo"}},
		},
		{
			"map values",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"foo": ovsdb.UUID{GoUUID: "foo"}}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"foo": ovsdb.UUID{GoUUID: testUUID}}},
		},
		{
			"map key and values",
			map[string]ovsdb.UUID{"foo": {GoUUID: testUUID}, "bar": {GoUUID: testUUID1}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{ovsdb.UUID{GoUUID: "foo"}: ovsdb.UUID{GoUUID: "bar"}}},
			ovsdb.OvsMap{GoMap: map[interface{}]interface{}{ovsdb.UUID{GoUUID: testUUID}: ovsdb.UUID{GoUUID: testUUID1}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandNamedUUID(tt.value, tt.namedUUIDs)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestOvsdbServerMonitor(t *testing.T) {
	defDB, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &ovsType{},
		"Bridge":       &bridgeType{}})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := getSchema()
	if err != nil {
		t.Fatal(err)
	}
	ovsDB := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	o, err := NewOvsdbServer(ovsDB, dbModel)
	require.Nil(t, err)
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

	transaction := o.NewTransaction(dbModel, "Open_vSwitch", o.db)

	_, updates := transaction.Insert("Bridge", fooUUID, ovsdb.Row{"name": "foo"})
	_, update2 := transaction.Insert("Bridge", barUUID, ovsdb.Row{"name": "bar"})
	updates.Merge(update2)
	_, update3 := transaction.Insert("Bridge", bazUUID, ovsdb.Row{"name": "baz"})
	updates.Merge(update3)
	_, update4 := transaction.Insert("Bridge", quuxUUID, ovsdb.Row{"name": "quux"})
	updates.Merge(update4)
	err = o.db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	db, err := json.Marshal("Open_vSwitch")
	require.Nil(t, err)
	value, err := json.Marshal("foo")
	require.Nil(t, err)
	rJSON, err := json.Marshal(requests)
	require.Nil(t, err)
	args := []json.RawMessage{db, value, rJSON}
	reply := &ovsdb.TableUpdates{}
	err = o.Monitor(nil, args, reply)
	require.Nil(t, err)
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
