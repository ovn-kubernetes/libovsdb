package inmemory

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovn-kubernetes/libovsdb/database"
	"github.com/ovn-kubernetes/libovsdb/mapper"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"

	. "github.com/ovn-kubernetes/libovsdb/test"
)

func TestWaitOpEquals(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	ovs := OvsType{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := BridgeType{
		Name: "foo",
		ExternalIds: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := db.NewTransaction("Open_vSwitch")

	operations := []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Open_vSwitch",
			UUIDName: ovsUUID,
			Row:      ovsRow,
		},
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Bridge",
			UUIDName: bridgeUUID,
			Row:      bridgeRow,
		},
	}
	res, updates := transaction.Transact(operations...)
	_, err = checkOperationResults(res, operations...)
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	timeout := 0
	// Attempt to wait for row with name foo to appear
	operation := ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name"},
		Until:   "==",
		Rows:    []ovsdb.Row{{"name": "foo"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Attempt to wait for 2 rows, where one does not exist
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name"},
		Until:   "==",
		Rows:    []ovsdb.Row{{"name": "foo"}, {"name": "blah"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.Error(t, err)

	extIDs, err := ovsdb.NewOvsMap(map[string]string{
		"foo":   "bar",
		"baz":   "quux",
		"waldo": "fred",
	})
	require.Nil(t, err)
	// Attempt to wait for a row, with multiple columns specified
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name", "external_ids"},
		Until:   "==",
		Rows:    []ovsdb.Row{{"name": "foo", "external_ids": extIDs}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Attempt to wait for a row, with multiple columns, but not specified in row filtering
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name", "external_ids"},
		Until:   "==",
		Rows:    []ovsdb.Row{{"name": "foo"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Attempt to get something with a non-zero timeout that will fail
	timeout = 400
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name", "external_ids"},
		Until:   "==",
		Rows:    []ovsdb.Row{{"name": "doesNotExist"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.Error(t, err)
}

func TestWaitOpNotEquals(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	ovs := OvsType{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := BridgeType{
		Name: "foo",
		ExternalIds: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := db.NewTransaction("Open_vSwitch")

	operations := []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Open_vSwitch",
			UUIDName: ovsUUID,
			Row:      ovsRow,
		},
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Bridge",
			UUIDName: bridgeUUID,
			Row:      bridgeRow,
		},
	}
	res, updates := transaction.Transact(operations...)
	_, err = checkOperationResults(res, operations...)
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	timeout := 0
	// Attempt a wait where no entry with name blah should exist
	operation := ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name"},
		Until:   "!=",
		Rows:    []ovsdb.Row{{"name": "blah"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Attempt another wait with multiple rows specified, one that would match, and one that doesn't
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name"},
		Until:   "!=",
		Rows:    []ovsdb.Row{{"name": "blah"}, {"name": "foo"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Attempt another wait where name would match, but ext ids would not match
	NoMatchExtIDs, err := ovsdb.NewOvsMap(map[string]string{
		"foo":   "bar",
		"baz":   "quux",
		"waldo": "is_different",
	})
	require.NoError(t, err)

	// Attempt to wait for a row, with multiple columns specified and one is not a match
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name", "external_ids"},
		Until:   "!=",
		Rows:    []ovsdb.Row{{"name": "foo", "external_ids": NoMatchExtIDs}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	// Check to see if a non match takes around the timeout
	start := time.Now()
	timeout = 200
	operation = ovsdb.Operation{
		Op:      ovsdb.OperationWait,
		Table:   "Bridge",
		Timeout: &timeout,
		Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		Columns: []string{"name"},
		Until:   "!=",
		Rows:    []ovsdb.Row{{"name": "foo"}},
	}
	res, _ = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.Error(t, err)

	ts := time.Since(start)
	if ts < time.Duration(timeout)*time.Millisecond {
		t.Fatalf("Should have taken at least %d milliseconds to return, but it took %d instead", timeout, ts)
	}
	require.NotNil(t, err)
}

func TestMutateOp(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	bridgeUUID := uuid.NewString()

	ovs := OvsType{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := BridgeType{
		Name: "foo",
		ExternalIds: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := db.NewTransaction("Open_vSwitch")

	operations := []ovsdb.Operation{
		{
			Op:    ovsdb.OperationInsert,
			Table: "Open_vSwitch",
			Row:   ovsRow,
		},
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  bridgeUUID,
			Row:   bridgeRow,
		},
	}
	res, updates := transaction.Transact(operations...)
	_, err = checkOperationResults(res, operations...)
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	ovsUUID := res[0].UUID.GoUUID
	operation := ovsdb.Operation{
		Op:        ovsdb.OperationMutate,
		Table:     "Open_vSwitch",
		Where:     []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: ovsUUID})},
		Mutations: []ovsdb.Mutation{*ovsdb.NewMutation("bridges", ovsdb.MutateOperationInsert, ovsdb.UUID{GoUUID: bridgeUUID})},
	}
	res, updates = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)
	assert.Equal(t, []*ovsdb.OperationResult{{Count: 1}}, res)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	bridgeSet, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: bridgeUUID}})
	assert.Nil(t, err)
	assert.Equal(t, ovsdb.TableUpdates2{
		"Open_vSwitch": ovsdb.TableUpdate2{
			ovsUUID: &ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"bridges": bridgeSet,
				},
				Old: &ovsdb.Row{
					// TODO: _uuid should be filtered
					"_uuid": ovsdb.UUID{GoUUID: ovsUUID},
				},
				New: &ovsdb.Row{
					// TODO: _uuid should be filtered
					"_uuid":   ovsdb.UUID{GoUUID: ovsUUID},
					"bridges": bridgeSet,
				},
			},
		},
	}, getTableUpdates(updates))

	keyDelete, err := ovsdb.NewOvsSet([]string{"foo"})
	assert.Nil(t, err)
	keyValueDelete, err := ovsdb.NewOvsMap(map[string]string{"baz": "quux"})
	assert.Nil(t, err)

	operation = ovsdb.Operation{
		Op:    ovsdb.OperationMutate,
		Table: "Bridge",
		Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID})},
		Mutations: []ovsdb.Mutation{
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationDelete, keyDelete),
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationDelete, keyValueDelete),
		},
	}
	res, updates = transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)
	assert.Equal(t, []*ovsdb.OperationResult{{Count: 1}}, res)

	oldExternalIds, _ := ovsdb.NewOvsMap(bridge.ExternalIds)
	newExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"waldo": "fred"})
	diffExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar", "baz": "quux"})

	assert.Nil(t, err)

	gotModify := *getTableUpdates(updates)["Bridge"][bridgeUUID].Modify
	gotOld := *getTableUpdates(updates)["Bridge"][bridgeUUID].Old
	gotNew := *getTableUpdates(updates)["Bridge"][bridgeUUID].New
	assert.Equal(t, diffExternalIds, gotModify["external_ids"])
	assert.Equal(t, oldExternalIds, gotOld["external_ids"])
	assert.Equal(t, newExternalIds, gotNew["external_ids"])
}

func TestOvsdbServerInsert(t *testing.T) {
	t.Skip("need a helper for comparing rows as map elements aren't in same order")
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	gromit := "gromit"
	bridge := BridgeType{
		Name:         "foo",
		DatapathType: "bar",
		DatapathID:   &gromit,
		ExternalIds: map[string]string{
			"foo":   "bar",
			"baz":   "qux",
			"waldo": "fred",
		},
	}
	bridgeUUID := uuid.NewString()
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := db.NewTransaction("Open_vSwitch")

	operation := ovsdb.Operation{
		Op:    ovsdb.OperationInsert,
		Table: "Bridge",
		UUID:  bridgeUUID,
		Row:   bridgeRow,
	}
	res, updates := transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	assert.NoError(t, err)

	bridge.UUID = bridgeUUID
	br, err := db.Get("Open_vSwitch", "Bridge", bridgeUUID)
	assert.NoError(t, err)
	assert.Equal(t, &bridge, br)
	assert.Equal(t, ovsdb.TableUpdates2{
		"Bridge": {
			bridgeUUID: &ovsdb.RowUpdate2{
				Insert: &bridgeRow,
				New:    &bridgeRow,
			},
		},
	}, updates)
}

func TestOvsdbServerUpdate(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	christmas := "christmas"
	bridge := BridgeType{
		Name:       "foo",
		DatapathID: &christmas,
		ExternalIds: map[string]string{
			"foo":   "bar",
			"baz":   "qux",
			"waldo": "fred",
		},
	}
	bridgeUUID := uuid.NewString()
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := db.NewTransaction("Open_vSwitch")

	operation := ovsdb.Operation{
		Op:    ovsdb.OperationInsert,
		Table: "Bridge",
		UUID:  bridgeUUID,
		Row:   bridgeRow,
	}
	res, updates := transaction.Transact(operation)
	_, err = checkOperationResults(res, operation)
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	assert.NoError(t, err)

	halloween, _ := ovsdb.NewOvsSet([]string{"halloween"})
	emptySet, _ := ovsdb.NewOvsSet([]string{})
	tests := []struct {
		name     string
		row      ovsdb.Row
		expected *ovsdb.RowUpdate2
	}{
		{
			"update single field",
			ovsdb.Row{"datapath_type": "waldo"},
			&ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"datapath_type": "waldo",
				},
			},
		},
		{
			"update single optional field, with direct value",
			ovsdb.Row{"datapath_id": "halloween"},
			&ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"datapath_id": halloween,
				},
			},
		},
		{
			"update single optional field, with set",
			ovsdb.Row{"datapath_id": halloween},
			&ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"datapath_id": halloween,
				},
			},
		},
		{
			"unset single optional field",
			ovsdb.Row{"datapath_id": emptySet},
			&ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"datapath_id": emptySet,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transaction := db.NewTransaction("Open_vSwitch")
			op := ovsdb.Operation{
				Op:    ovsdb.OperationUpdate,
				Table: "Bridge",
				Where: []ovsdb.Condition{{
					Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: bridgeUUID},
				}},
				Row: tt.row,
			}
			res, updates := transaction.Transact(op)
			errs, err := checkOperationResults(res, op)
			require.NoErrorf(t, err, "%+v", errs)

			bridge.UUID = bridgeUUID
			row, err := db.Get("Open_vSwitch", "Bridge", bridgeUUID)
			assert.NoError(t, err)
			br := row.(*BridgeType)
			assert.NotEqual(t, br, bridgeRow)
			assert.Equal(t, tt.expected.Modify, getTableUpdates(updates)["Bridge"][bridgeUUID].Modify)
		})
	}
}

func TestMultipleOps(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)

	var ops []ovsdb.Operation
	var op ovsdb.Operation

	bridgeUUID := uuid.NewString()
	op = ovsdb.Operation{
		Op:    ovsdb.OperationInsert,
		Table: "Bridge",
		UUID:  bridgeUUID,
		Row: ovsdb.Row{
			"name": "a_bridge_to_nowhere",
		},
	}
	ops = append(ops, op)

	op = ovsdb.Operation{
		Op:    ovsdb.OperationUpdate,
		Table: "Bridge",
		Where: []ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		Row: ovsdb.Row{
			"ports":        ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port1"}, ovsdb.UUID{GoUUID: "port10"}}},
			"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key10": "value10"}},
		},
	}
	ops = append(ops, op)

	transaction := db.NewTransaction("Open_vSwitch")
	results, _ := transaction.Transact(ops...)
	assert.Len(t, results, len(ops))
	assert.NotNil(t, results[0])
	assert.Empty(t, results[0].Error)
	assert.Equal(t, 0, results[0].Count)
	assert.Equal(t, bridgeUUID, results[0].UUID.GoUUID)
	assert.NotNil(t, results[1])
	assert.Equal(t, 1, results[1].Count)
	assert.Empty(t, results[1].Error)

	ops = ops[:0]
	op = ovsdb.Operation{
		Table: "Bridge",
		Where: []ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		Op: ovsdb.OperationMutate,
		Mutations: []ovsdb.Mutation{
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationInsert, ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"keyA": "valueA"}}),
			*ovsdb.NewMutation("ports", ovsdb.MutateOperationDelete, ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port1"}, ovsdb.UUID{GoUUID: "port10"}}}),
		},
	}
	ops = append(ops, op)

	op2 := ovsdb.Operation{
		Table: "Bridge",
		Where: []ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		Op: ovsdb.OperationMutate,
		Mutations: []ovsdb.Mutation{
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationDelete, ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key10": "value10"}}),
			*ovsdb.NewMutation("ports", ovsdb.MutateOperationInsert, ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port1"}}}),
		},
	}
	ops = append(ops, op2)

	results, updates := transaction.Transact(ops...)
	require.Len(t, results, len(ops))
	for _, result := range results {
		assert.Empty(t, result.Error)
		assert.Equal(t, 1, result.Count)
	}

	assert.Equal(t, ovsdb.TableUpdates2{
		"Bridge": ovsdb.TableUpdate2{
			bridgeUUID: &ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"keyA": "valueA", "key10": "value10"}},
					"ports":        ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port10"}}},
				},
				Old: &ovsdb.Row{
					"_uuid":        ovsdb.UUID{GoUUID: bridgeUUID},
					"name":         "a_bridge_to_nowhere",
					"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key10": "value10"}},
					"ports":        ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port1"}, ovsdb.UUID{GoUUID: "port10"}}},
				},
				New: &ovsdb.Row{
					"_uuid":        ovsdb.UUID{GoUUID: bridgeUUID},
					"name":         "a_bridge_to_nowhere",
					"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "keyA": "valueA"}},
					"ports":        ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "port1"}}},
				},
			},
		},
	}, getTableUpdates(updates))

}

func TestOvsdbServerDbDoesNotExist(t *testing.T) {
	defDB, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Open_vSwitch": &OvsType{},
		"Bridge":       &BridgeType{}})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := GetSchema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)

	ops := []ovsdb.Operation{
		{
			Op:    ovsdb.OperationInsert,
			Table: "Bridge",
			UUID:  uuid.NewString(),
			Row: ovsdb.Row{
				"name": "bridge",
			},
		},
		{
			Op:    ovsdb.OperationUpdate,
			Table: "Bridge",
			Where: []ovsdb.Condition{
				{
					Column:   "name",
					Function: ovsdb.ConditionEqual,
					Value:    "bridge",
				},
			},
			Row: ovsdb.Row{
				"datapath_type": "type",
			},
		},
	}

	transaction := db.NewTransaction("nonexsitent_db")
	res, _ := transaction.Transact(ops...)
	assert.Len(t, res, len(ops))
	assert.Equal(t, "database does not exist", res[0].Error)
	assert.Nil(t, res[1])
}

func TestCheckIndexes(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)

	bridgeUUID := uuid.NewString()
	fscsUUID := uuid.NewString()
	fscsUUID2 := uuid.NewString()
	fscsUUID3 := uuid.NewString()
	ops := []ovsdb.Operation{
		{
			Table: "Bridge",
			Op:    ovsdb.OperationInsert,
			UUID:  bridgeUUID,
			Row: ovsdb.Row{
				"name": "a_bridge_to_nowhere",
			},
		},
		{
			Table: "Flow_Sample_Collector_Set",
			Op:    ovsdb.OperationInsert,
			UUID:  fscsUUID,
			Row: ovsdb.Row{
				"id":     1,
				"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
			},
		},
		{
			Table: "Flow_Sample_Collector_Set",
			Op:    ovsdb.OperationInsert,
			UUID:  fscsUUID2,
			Row: ovsdb.Row{
				"id":     2,
				"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
			},
		},
	}

	transaction := db.NewTransaction("Open_vSwitch")
	results, updates := transaction.Transact(ops...)
	require.Len(t, results, len(ops))
	for _, result := range results {
		assert.Equal(t, "", result.Error)
	}
	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	tests := []struct {
		desc        string
		ops         func() []ovsdb.Operation
		expectedErr string
	}{
		{
			"Inserting an existing database index should fail",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationInsert,
						UUID:  fscsUUID3,
						Row: ovsdb.Row{
							"id":     1,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
					},
				}
			},
			"constraint violation",
		},
		{
			"Updating an index to an existing database index should fail",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationUpdate,
						Row: ovsdb.Row{
							"id":     2,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: fscsUUID}),
						},
					},
				}
			},
			"constraint violation",
		},
		{
			"Updating an index to an existing transaction index should fail",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationInsert,
						UUID:  fscsUUID3,
						Row: ovsdb.Row{
							"id":     3,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
					},
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationUpdate,
						Row: ovsdb.Row{
							"id":     3,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: fscsUUID}),
						},
					},
				}
			},
			"constraint violation",
		},
		{
			"Updating an index to an old index that is updated in the same transaction should succeed",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationInsert,
						UUID:  fscsUUID3,
						Row: ovsdb.Row{
							"id":     1,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
					},
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationUpdate,
						Row: ovsdb.Row{
							"id":     3,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: fscsUUID}),
						},
					},
				}
			},
			"",
		},
		{
			"Updating an index to a old index that is deleted in the same transaction should succeed",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationInsert,
						UUID:  fscsUUID3,
						Row: ovsdb.Row{
							"id":     1,
							"bridge": ovsdb.UUID{GoUUID: bridgeUUID},
						},
					},
					{
						Table: "Flow_Sample_Collector_Set",
						Op:    ovsdb.OperationDelete,
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: fscsUUID}),
						},
					},
				}
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			transaction := db.NewTransaction("Open_vSwitch")
			ops := tt.ops()
			results, _ := transaction.Transact(ops...)
			var err string
			for _, result := range results {
				if result.Error != "" {
					err = result.Error
					break
				}
			}
			require.Equal(t, tt.expectedErr, err, "got a different error than expected")
			if tt.expectedErr != "" {
				require.Len(t, results, len(ops)+1)
			} else {
				require.Len(t, results, len(ops))
			}
		})
	}
}

func getTableUpdates(update database.Update) ovsdb.TableUpdates2 {
	tus := ovsdb.TableUpdates2{}
	tables := update.GetUpdatedTables()
	for _, table := range tables {
		tu := ovsdb.TableUpdate2{}
		_ = update.ForEachRowUpdate(table, func(uuid string, row ovsdb.RowUpdate2) error {
			tu[uuid] = &row
			return nil
		})
		tus[table] = tu
	}
	return tus
}

func checkOperationResults(result []*ovsdb.OperationResult, ops ...ovsdb.Operation) ([]ovsdb.OperationError, error) {
	r := make([]ovsdb.OperationResult, len(result))
	for i := range result {
		r[i] = *result[i]
	}
	return ovsdb.CheckOperationResults(r, ops)
}

func TestCheckIndexesWithReferentialIntegrity(t *testing.T) {
	dbModel, err := GetModel()
	require.NoError(t, err)
	db := NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	err = db.CreateDatabase("Open_vSwitch", dbModel.Schema)
	require.NoError(t, err)

	ovsUUID := uuid.NewString()
	managerUUID := uuid.NewString()
	managerUUID2 := uuid.NewString()
	ops := []ovsdb.Operation{
		{
			Table: "Open_vSwitch",
			Op:    ovsdb.OperationInsert,
			UUID:  ovsUUID,
			Row: ovsdb.Row{
				"manager_options": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: managerUUID}}},
			},
		},
		{
			Table: "Manager",
			Op:    ovsdb.OperationInsert,
			UUID:  managerUUID,
			Row: ovsdb.Row{
				"target": "target",
			},
		},
	}

	transaction := db.NewTransaction("Open_vSwitch")
	results, updates := transaction.Transact(ops...)
	require.Len(t, results, len(ops))
	for _, result := range results {
		assert.Equal(t, "", result.Error)
	}
	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	tests := []struct {
		desc        string
		ops         func() []ovsdb.Operation
		wantUpdates int
	}{
		{
			// As a row is deleted due to garbage collection, that row's index
			// should be available for use by a different row
			desc: "Replacing a strong reference should garbage collect and account for indexes",
			ops: func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Open_vSwitch",
						Op:    ovsdb.OperationUpdate,
						Row: ovsdb.Row{
							"manager_options": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: managerUUID2}}},
						},
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: ovsUUID}),
						},
					},
					{
						Table: "Manager",
						Op:    ovsdb.OperationInsert,
						UUID:  managerUUID2,
						Row: ovsdb.Row{
							"target": "target",
						},
					},
				}
			},
			// the update and insert above plus the delete from the garbage
			// collection
			wantUpdates: 3,
		},
		{
			desc: "A row that is not root and not strongly referenced should not cause index collisions",
			ops: func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Manager",
						Op:    ovsdb.OperationInsert,
						UUID:  managerUUID2,
						Row: ovsdb.Row{
							"target": "target",
						},
					},
				}
			},
			// no updates as the row is not strongly referenced
			wantUpdates: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			transaction := db.NewTransaction("Open_vSwitch")
			ops := tt.ops()
			results, update := transaction.Transact(ops...)
			var err string
			for _, result := range results {
				if result.Error != "" {
					err = result.Error
					break
				}
			}
			require.Empty(t, err, "got an unexpected error")

			tables := update.GetUpdatedTables()
			var gotUpdates int
			for _, table := range tables {
				_ = update.ForEachRowUpdate(table, func(uuid string, row ovsdb.RowUpdate2) error {
					gotUpdates++
					return nil
				})
			}
			assert.Equal(t, tt.wantUpdates, gotUpdates, "got a different number of updates than expected")
		})
	}
}
