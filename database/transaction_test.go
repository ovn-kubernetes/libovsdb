package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovn-org/libovsdb/mapper"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"

	. "github.com/ovn-org/libovsdb/test"
)

func TestWaitOpEquals(t *testing.T) {
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	m := mapper.NewMapper(schema)

	ovs := OpenvSwitch{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := Bridge{
		Name: "foo",
		ExternalIDs: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Open_vSwitch", ovsUUID, ovsRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	res, update2 := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	updates.Merge(update2)
	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	timeout := 0
	// Attempt to wait for row with name foo to appear
	gotResult := transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name"},
		"==",
		[]ovsdb.Row{{"name": "foo"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Attempt to wait for 2 rows, where one does not exist
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name"},
		"==",
		[]ovsdb.Row{{"name": "foo"}, {"name": "blah"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.NotNil(t, err)

	extIDs, err := ovsdb.NewOvsMap(map[string]string{
		"foo":   "bar",
		"baz":   "quux",
		"waldo": "fred",
	})
	require.Nil(t, err)
	// Attempt to wait for a row, with multiple columns specified
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name", "external_ids"},
		"==",
		[]ovsdb.Row{{"name": "foo", "external_ids": extIDs}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Attempt to wait for a row, with multiple columns, but not specified in row filtering
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name", "external_ids"},
		"==",
		[]ovsdb.Row{{"name": "foo"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Attempt to get something with a non-zero timeout that will fail
	timeout = 400
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name", "external_ids"},
		"==",
		[]ovsdb.Row{{"name": "doesNotExist"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.NotNil(t, err)

}

func TestWaitOpNotEquals(t *testing.T) {
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	m := mapper.NewMapper(schema)

	ovs := OpenvSwitch{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := Bridge{
		Name: "foo",
		ExternalIDs: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Open_vSwitch", ovsUUID, ovsRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	res, update2 := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	updates.Merge(update2)
	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	timeout := 0
	// Attempt a wait where no entry with name blah should exist
	gotResult := transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name"},
		"!=",
		[]ovsdb.Row{{"name": "blah"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Attempt another wait with multiple rows specified, one that would match, and one that doesn't
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name"},
		"!=",
		[]ovsdb.Row{{"name": "blah"}, {"name": "foo"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Attempt another wait where name would match, but ext ids would not match
	NoMatchExtIDs, err := ovsdb.NewOvsMap(map[string]string{
		"foo":   "bar",
		"baz":   "quux",
		"waldo": "is_different",
	})
	require.Nil(t, err)
	// Attempt to wait for a row, with multiple columns specified and one is not a match
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name", "external_ids"},
		"!=",
		[]ovsdb.Row{{"name": "foo", "external_ids": NoMatchExtIDs}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	require.Nil(t, err)

	// Check to see if a non match takes around the timeout
	start := time.Now()
	timeout = 200
	gotResult = transaction.Wait(
		"Bridge",
		&timeout,
		[]ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "foo")},
		[]string{"name"},
		"!=",
		[]ovsdb.Row{{"name": "foo"}},
	)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{gotResult}, []ovsdb.Operation{{Op: "wait"}})
	ts := time.Since(start)
	if ts < time.Duration(timeout)*time.Millisecond {
		t.Fatalf("Should have taken at least %d milliseconds to return, but it took %d instead", timeout, ts)
	}
	require.NotNil(t, err)
}

func TestMutateOp(t *testing.T) {
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)

	ovsUUID := uuid.NewString()
	bridgeUUID := uuid.NewString()

	m := mapper.NewMapper(schema)

	ovs := OpenvSwitch{}
	info, err := dbModel.NewModelInfo(&ovs)
	require.NoError(t, err)
	ovsRow, err := m.NewRow(info)
	require.Nil(t, err)

	bridge := Bridge{
		Name: "foo",
		ExternalIDs: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Open_vSwitch", ovsUUID, ovsRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	res, update2 := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	updates.Merge(update2)
	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	gotResult, gotUpdate := transaction.Mutate(
		"Open_vSwitch",
		[]ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: ovsUUID}),
		},
		[]ovsdb.Mutation{
			*ovsdb.NewMutation("bridges", ovsdb.MutateOperationInsert, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
	)
	assert.Equal(t, ovsdb.OperationResult{Count: 1}, gotResult)
	err = db.Commit("Open_vSwitch", uuid.New(), gotUpdate)
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
	}, gotUpdate)

	keyDelete, err := ovsdb.NewOvsSet([]string{"foo"})
	assert.Nil(t, err)
	keyValueDelete, err := ovsdb.NewOvsMap(map[string]string{"baz": "quux"})
	assert.Nil(t, err)
	gotResult, gotUpdate = transaction.Mutate(
		"Bridge",
		[]ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		[]ovsdb.Mutation{
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationDelete, keyDelete),
			*ovsdb.NewMutation("external_ids", ovsdb.MutateOperationDelete, keyValueDelete),
		},
	)
	assert.Equal(t, ovsdb.OperationResult{Count: 1}, gotResult)

	oldExternalIds, _ := ovsdb.NewOvsMap(bridge.ExternalIDs)
	newExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"waldo": "fred"})
	diffExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar", "baz": "quux"})

	assert.Nil(t, err)

	gotModify := *gotUpdate["Bridge"][bridgeUUID].Modify
	gotOld := *gotUpdate["Bridge"][bridgeUUID].Old
	gotNew := *gotUpdate["Bridge"][bridgeUUID].New
	assert.Equal(t, diffExternalIds, gotModify["external_ids"])
	assert.Equal(t, oldExternalIds, gotOld["external_ids"])
	assert.Equal(t, newExternalIds, gotNew["external_ids"])
}

func TestDiff(t *testing.T) {
	originSet, _ := ovsdb.NewOvsSet([]interface{}{"foo"})

	originSetAdd, _ := ovsdb.NewOvsSet([]interface{}{"bar"})
	setAddDiff, _ := ovsdb.NewOvsSet([]interface{}{"foo", "bar"})

	originSetDel, _ := ovsdb.NewOvsSet([]interface{}{})
	setDelDiff, _ := ovsdb.NewOvsSet([]interface{}{"foo"})

	originMap, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"foo": "bar"})

	originMapAdd, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"foo": "bar", "baz": "quux"})
	originMapAddDiff, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"baz": "quux"})

	originMapDel, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{})
	originMapDelDiff, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"foo": "bar"})

	originMapReplace, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"foo": "baz"})
	originMapReplaceDiff, _ := ovsdb.NewOvsMap(map[interface{}]interface{}{"foo": "baz"})

	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected interface{}
	}{
		{
			"add to set",
			originSet,
			originSetAdd,
			setAddDiff,
		},
		{
			"delete from set",
			originSet,
			originSetDel,
			setDelDiff,
		},
		{
			"noop set",
			originSet,
			originSet,
			nil,
		},
		{
			"add to map",
			originMap,
			originMapAdd,
			originMapAddDiff,
		},
		{
			"delete from map",
			originMap,
			originMapDel,
			originMapDelDiff,
		},
		{
			"replace in map",
			originMap,
			originMapReplace,
			originMapReplaceDiff,
		},
		{
			"noop map",
			originMap,
			originMap,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := diff(tt.a, tt.b)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestOvsdbServerInsert(t *testing.T) {
	t.Skip("need a helper for comparing rows as map elements aren't in same order")
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	m := mapper.NewMapper(schema)

	gromit := "gromit"
	bridge := Bridge{
		Name:         "foo",
		DatapathType: "bar",
		DatapathID:   &gromit,
		ExternalIDs: map[string]string{
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

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
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
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	m := mapper.NewMapper(schema)

	bridge := Bridge{
		Name: "foo",
		ExternalIDs: map[string]string{
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

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.NoError(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	assert.NoError(t, err)

	halloween, _ := ovsdb.NewOvsSet([]string{"halloween"})
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
			"update single optional field",
			ovsdb.Row{"datapath_id": "halloween"},
			&ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"datapath_id": halloween,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, updates := transaction.Update(
				"Bridge",
				[]ovsdb.Condition{{
					Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: bridgeUUID},
				}}, tt.row)
			errs, err := ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "update"}})
			require.NoErrorf(t, err, "%+v", errs)

			bridge.UUID = bridgeUUID
			row, err := db.Get("Open_vSwitch", "Bridge", bridgeUUID)
			assert.NoError(t, err)
			br := row.(*Bridge)
			assert.NotEqual(t, br, bridgeRow)
			assert.Equal(t, tt.expected.Modify, updates["Bridge"][bridgeUUID].Modify)
		})
	}
}

func TestMultipleOps(t *testing.T) {
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	m := mapper.NewMapper(schema)

	bridgeUUID := uuid.NewString()
	bridge := Bridge{
		Name: "a_bridge_to_nowhere",
		Ports: []string{
			"port1",
			"port10",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	var ops []ovsdb.Operation
	var op ovsdb.Operation

	portA, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: "portA"}})
	require.NoError(t, err)
	op = ovsdb.Operation{
		Table: "Bridge",
		Where: []ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		Op: ovsdb.OperationMutate,
		Mutations: []ovsdb.Mutation{
			*ovsdb.NewMutation("ports", ovsdb.MutateOperationInsert, portA),
		},
	}
	ops = append(ops, op)

	portBC, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: "portB"}, {GoUUID: "portC"}})
	require.NoError(t, err)
	op2 := ovsdb.Operation{
		Table: "Bridge",
		Where: []ovsdb.Condition{
			ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
		},
		Op: ovsdb.OperationMutate,
		Mutations: []ovsdb.Mutation{
			*ovsdb.NewMutation("ports", ovsdb.MutateOperationInsert, portBC),
		},
	}
	ops = append(ops, op2)

	results, updates := transaction.Transact(ops)
	require.Len(t, results, len(ops))
	for _, result := range results {
		assert.Equal(t, "", result.Error)
	}

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	modifiedPorts, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: "portA"}, {GoUUID: "portB"}, {GoUUID: "portC"}})
	assert.Nil(t, err)

	oldPorts, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: "port1"}, {GoUUID: "port10"}})
	assert.Nil(t, err)

	newPorts, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: "port1"}, {GoUUID: "port10"}, {GoUUID: "portA"}, {GoUUID: "portB"}, {GoUUID: "portC"}})
	assert.Nil(t, err)

	assert.Equal(t, ovsdb.TableUpdates2{
		"Bridge": ovsdb.TableUpdate2{
			bridgeUUID: &ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{
					"ports": modifiedPorts,
				},
				Old: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: bridgeUUID},
					"name":  "a_bridge_to_nowhere",
					"ports": oldPorts,
				},
				New: &ovsdb.Row{
					"_uuid": ovsdb.UUID{GoUUID: bridgeUUID},
					"name":  "a_bridge_to_nowhere",
					"ports": newPorts,
				},
			},
		},
	}, updates)

}

func TestCheckIndexes(t *testing.T) {
	clientDbModel, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}
	clientDbModel.SetIndexes(
		map[string][]model.ClientIndex{
			"Bridge": {
				model.ClientIndex{
					Type: model.PrimaryIndexType,
					Columns: []model.ColumnKey{
						{
							Column: "external_ids",
							Key:    "primary_key",
						},
					},
				},
				model.ClientIndex{
					Type: model.SecondaryIndexType,
					Columns: []model.ColumnKey{
						{
							Column: "external_ids",
							Key:    "secondary_key",
						},
					},
				},
			},
		},
	)
	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	db := NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": clientDbModel})
	err = db.CreateDatabase("Open_vSwitch", schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, clientDbModel)
	require.Empty(t, errs)
	m := mapper.NewMapper(schema)

	bridgeUUID := uuid.NewString()
	bridge := Bridge{
		Name: "a_bridge_to_nowhere",
		ExternalIDs: map[string]string{
			"primary_key":   "primary_key_1",
			"secondary_key": "secondary_key_1",
		},
	}
	bridgeInfo, err := dbModel.NewModelInfo(&bridge)
	require.NoError(t, err)
	bridgeRow, err := m.NewRow(bridgeInfo)
	require.Nil(t, err)

	transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)

	res, updates := transaction.Insert("Bridge", bridgeUUID, bridgeRow)
	_, err = ovsdb.CheckOperationResults([]ovsdb.OperationResult{res}, []ovsdb.Operation{{Op: "insert"}})
	require.Nil(t, err)

	err = db.Commit("Open_vSwitch", uuid.New(), updates)
	require.NoError(t, err)

	tests := []struct {
		desc              string
		ops               func() []ovsdb.Operation
		expectedErrorType interface{}
	}{
		{
			"Create to duplicate a schema index should fail",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Bridge",
						Op:    ovsdb.OperationInsert,
						Row: ovsdb.Row{
							"name": "a_bridge_to_nowhere",
						},
					},
				}
			},
			&ovsdb.ConstraintViolation{},
		},
		{
			"Create to a duplicate primary client index should fail",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Bridge",
						Op:    ovsdb.OperationInsert,
						Row: ovsdb.Row{
							"name": "a_bridge_to_nowhere_2",
							"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{
								"primary_key": "primary_key_1",
							}},
						},
					},
				}
			},
			&ovsdb.ConstraintViolation{},
		},
		{
			"Creating a duplicate secondary client index should succeed",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Bridge",
						Op:    ovsdb.OperationInsert,
						Row: ovsdb.Row{
							"name": "a_bridge_to_nowhere_2",
							"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{
								"secondary_key": "secondary_key_1",
							}},
						},
					},
				}
			},
			nil,
		},
		{
			"Changing an existing index and creating it again in the same transaction should succeed",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Bridge",
						Op:    ovsdb.OperationInsert,
						Row: ovsdb.Row{
							"name": "a_bridge_to_nowhere_2",
							"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{
								"primary_key": "primary_key_1",
							}},
						},
					},
					{
						Table: "Bridge",
						Op:    ovsdb.OperationUpdate,
						Row: ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{
								"primary_key": "primary_key_2",
							}},
						},
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
						},
					},
				}
			},
			nil,
		},
		{
			"Deleting an existing index and creating it again in the same transaction should succeed",
			func() []ovsdb.Operation {
				return []ovsdb.Operation{
					{
						Table: "Bridge",
						Op:    ovsdb.OperationInsert,
						Row: ovsdb.Row{
							"name": "a_bridge_to_nowhere_2",
							"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{
								"primary_key": "primary_key_1",
							}},
						},
					},
					{
						Table: "Bridge",
						Op:    ovsdb.OperationDelete,
						Where: []ovsdb.Condition{
							ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: bridgeUUID}),
						},
					},
				}
			},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			transaction := NewTransaction(dbModel, "Open_vSwitch", db, nil)
			ops := tt.ops()
			res, _ := transaction.Transact(ops)
			_, err = ovsdb.CheckOperationResults(res, ops)
			if tt.expectedErrorType != nil {
				require.Error(t, err)
				require.IsTypef(t, tt.expectedErrorType, err, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
