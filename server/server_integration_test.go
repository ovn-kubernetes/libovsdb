package server

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/database/inmemory"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/libovsdb/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestServerAndClient(t *testing.T) (client.Client, func()) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	ovsDB := inmemory.NewDatabase(map[string]model.ClientDBModel{"Open_vSwitch": dbModel.Client()})
	schema := dbModel.Schema
	defDB := dbModel.Client()

	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	server, err := NewOvsdbServer(ovsDB, dbModel)
	assert.Nil(t, err)

	go func(t *testing.T, o *OvsdbServer) {
		if err := o.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}(t, server)
	defer server.Close()
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	ovs, err := client.NewOVSDBClient(defDB, client.WithEndpoint(fmt.Sprintf("unix:%s", tmpfile)))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)

	return ovs, func() {
		ovs.Disconnect()
		server.Close()
	}
}

func TestClientServerEcho(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	err := ovs.Echo(context.Background())
	assert.Nil(t, err)
}

func TestClientServerInsert(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	_, err := ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	wallace := "wallace"
	bridgeRow := &test.BridgeType{
		Name:         "foo",
		DatapathType: "bar",
		DatapathID:   &wallace,
		ExternalIds:  map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	assert.Nil(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	assert.NoErrorf(t, err, "%+v", opErr)

	uuid := reply[0].UUID.GoUUID
	require.Eventually(t, func() bool {
		br := &test.BridgeType{UUID: uuid}
		err := ovs.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	br := &test.BridgeType{UUID: uuid}
	err = ovs.Get(context.Background(), br)
	require.NoError(t, err)

	assert.Equal(t, bridgeRow.Name, br.Name)
	assert.Equal(t, bridgeRow.ExternalIds, br.ExternalIds)
	assert.Equal(t, bridgeRow.DatapathType, br.DatapathType)
	assert.Equal(t, *bridgeRow.DatapathID, wallace)
}

func TestClientServerMonitor(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	ovsRow := &test.OvsType{
		UUID: "ovs",
	}
	bridgeRow := &test.BridgeType{
		UUID:        "foo",
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	seenMutex := sync.RWMutex{}
	seenInsert := false
	seenMutation := false
	seenInitialOvs := false
	ovs.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			if table == "Bridge" {
				br := model.(*test.BridgeType)
				assert.Equal(t, bridgeRow.Name, br.Name)
				assert.Equal(t, bridgeRow.ExternalIds, br.ExternalIds)
				seenMutex.Lock()
				seenInsert = true
				seenMutex.Unlock()
			}
			if table == "Open_vSwitch" {
				seenMutex.Lock()
				seenInitialOvs = true
				seenMutex.Unlock()
			}
		},
		UpdateFunc: func(table string, old, new model.Model) {
			if table == "Open_vSwitch" {
				ov := new.(*test.OvsType)
				assert.Equal(t, 1, len(ov.Bridges))
				seenMutex.Lock()
				seenMutation = true
				seenMutex.Unlock()
			}
		},
	})

	var ops []ovsdb.Operation
	ovsOps, err := ovs.Create(ovsRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ovsOps...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ovsOps)
	require.Nil(t, err)
	require.NotEmpty(t, reply[0].UUID.GoUUID)
	ovsRow.UUID = reply[0].UUID.GoUUID

	_, err = ovs.MonitorAll(context.Background())
	require.Nil(t, err)
	require.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenInitialOvs
	}, 1*time.Second, 10*time.Millisecond)

	bridgeOps, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	ops = append(ops, bridgeOps...)

	mutateOps, err := ovs.Where(ovsRow).Mutate(ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{"foo"},
	})
	require.Nil(t, err)
	ops = append(ops, mutateOps...)

	reply, err = ovs.Transact(context.Background(), ops...)
	require.Nil(t, err)

	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Nil(t, err)
	assert.Equal(t, 1, reply[1].Count)

	assert.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenInsert
	}, 1*time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool {
		seenMutex.RLock()
		defer seenMutex.RUnlock()
		return seenMutation
	}, 1*time.Second, 10*time.Millisecond)
}

func TestClientServerInsertAndDelete(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	_, err := ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	bridgeRow := &test.BridgeType{
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &test.BridgeType{UUID: uuid}
		err := ovs.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow.UUID = uuid
	deleteOp, err := ovs.Where(bridgeRow).Delete()
	require.Nil(t, err)

	reply, err = ovs.Transact(context.Background(), deleteOp...)
	assert.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Nil(t, err)
	assert.Equal(t, 1, reply[0].Count)
}

func TestClientServerInsertDuplicate(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	bridgeRow := &test.BridgeType{
		Name:        "foo",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, err)

	// duplicate
	reply, err = ovs.Transact(context.Background(), ops...)
	require.Nil(t, err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, opErrs)
	require.Error(t, err)
	require.IsTypef(t, &ovsdb.ConstraintViolation{}, err, err.Error())
}

func TestClientServerInsertAndUpdate(t *testing.T) {
	ovs, close := buildTestServerAndClient(t)
	defer close()

	_, err := ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	bridgeRow := &test.BridgeType{
		Name:        "br-update",
		ExternalIds: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.NoError(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoError(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &test.BridgeType{UUID: uuid}
		err := ovs.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	// try to modify immutable field
	bridgeRow.UUID = uuid
	bridgeRow.Name = "br-update2"
	_, err = ovs.Where(bridgeRow).Update(bridgeRow, &bridgeRow.Name)
	require.Error(t, err)
	bridgeRow.Name = "br-update"

	// update many fields
	bridgeRow.UUID = uuid
	bridgeRow.Name = "br-update"
	bridgeRow.ExternalIds["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow)
	require.NoError(t, err)
	reply, err = ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(t, err, "%+v", opErrs)

	require.Eventually(t, func() bool {
		br := &test.BridgeType{UUID: uuid}
		err = ovs.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIds = newExternalIds
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIds)
	require.NoError(t, err)
	reply, err = ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(t, err, "%+v", opErr)

	assert.Eventually(t, func() bool {
		br := &test.BridgeType{UUID: uuid}
		err = ovs.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br.ExternalIds, bridgeRow.ExternalIds)
	}, 2*time.Second, 500*time.Millisecond)

	br := &test.BridgeType{UUID: uuid}
	err = ovs.Get(context.Background(), br)
	assert.NoError(t, err)

	assert.Equal(t, bridgeRow, br)
}

func TestUnsetOptional(t *testing.T) {
	c, close := buildTestServerAndClient(t)
	defer close()
	_, err := c.MonitorAll(context.Background())
	require.NoError(t, err)

	// Create the default bridge which has an optional DatapathID set
	optional := "optional"
	br := test.BridgeType{
		Name:       "br-with-optional",
		DatapathID: &optional,
	}
	ops, err := c.Create(&br)
	require.NoError(t, err)
	r, err := c.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(t, err)

	// verify the bridge has DatapathID set
	err = c.Get(context.Background(), &br)
	require.NoError(t, err)
	require.NotNil(t, br.DatapathID)

	// modify bridge to unset DatapathID
	br.DatapathID = nil
	ops, err = c.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(t, err)
	r, err = c.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(t, err)

	// verify the bridge has DatapathID unset
	err = c.Get(context.Background(), &br)
	require.NoError(t, err)
	require.Nil(t, br.DatapathID)
}

func TestUpdateOptional(t *testing.T) {
	c, close := buildTestServerAndClient(t)
	defer close()
	_, err := c.MonitorAll(context.Background())
	require.NoError(t, err)

	// Create the default bridge which has an optional DatapathID set
	old := "old"
	br := test.BridgeType{
		Name:       "br-with-optional",
		DatapathID: &old,
	}
	ops, err := c.Create(&br)
	require.NoError(t, err)
	r, err := c.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(t, err)

	// verify the bridge has DatapathID set
	err = c.Get(context.Background(), &br)
	require.NoError(t, err)
	require.NotNil(t, br.DatapathID)

	// modify bridge to update DatapathID
	new := "new"
	br.DatapathID = &new
	ops, err = c.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(t, err)
	r, err = c.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(t, err)

	// verify the bridge has DatapathID updated
	err = c.Get(context.Background(), &br)
	require.NoError(t, err)
	require.Equal(t, &new, br.DatapathID)
}

func TestMultipleOpsSameRow(t *testing.T) {
	c, close := buildTestServerAndClient(t)
	defer close()
	_, err := c.MonitorAll(context.Background())
	require.NoError(t, err)

	var ops []ovsdb.Operation
	var op []ovsdb.Operation

	// Insert a bridge
	bridgeInsertOp := len(ops)
	bridgeUUID := "bridge_multiple_ops_same_row"
	datapathID := "datapathID"
	br := test.BridgeType{
		UUID:        bridgeUUID,
		Name:        bridgeUUID,
		DatapathID:  &datapathID,
		Ports:       []string{"port10", "port1"},
		ExternalIds: map[string]string{"key1": "value1"},
	}
	op, err = c.Create(&br)
	require.NoError(t, err)
	ops = append(ops, op...)

	results, err := c.Transact(context.TODO(), ops...)
	require.NoError(t, err)

	_, err = ovsdb.CheckOperationResults(results, ops)
	require.NoError(t, err)

	// find out the real bridge UUID
	bridgeUUID = results[bridgeInsertOp].UUID.GoUUID

	ops = []ovsdb.Operation{}

	// Do several ops with the bridge in the same transaction
	br.Ports = []string{"port10"}
	br.ExternalIds = map[string]string{"key1": "value1", "key10": "value10"}
	op, err = c.Where(&br).Update(&br, &br.Ports, &br.ExternalIds)
	require.NoError(t, err)
	ops = append(ops, op...)

	op, err = c.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIds,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"keyA": "valueA"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{"port1"},
		},
	)
	require.NoError(t, err)
	ops = append(ops, op...)

	op, err = c.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIds,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   map[string]string{"key10": "value10"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{"port10"},
		},
	)
	require.NoError(t, err)
	ops = append(ops, op...)

	datapathID = "datapathID_updated"
	op, err = c.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(t, err)
	ops = append(ops, op...)

	br.DatapathID = nil
	op, err = c.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(t, err)
	ops = append(ops, op...)

	results, err = c.Transact(context.TODO(), ops...)
	require.NoError(t, err)
	require.Len(t, results, len(ops))

	errors, err := ovsdb.CheckOperationResults(results, ops)
	require.NoError(t, err)
	require.Nil(t, errors)

	br = test.BridgeType{
		UUID: bridgeUUID,
	}
	err = c.Get(context.TODO(), &br)
	require.NoError(t, err)
	require.Equal(t, []string{"port1"}, br.Ports)
	require.Equal(t, map[string]string{"key1": "value1", "keyA": "valueA"}, br.ExternalIds)
	require.Nil(t, br.DatapathID)
}

func TestReferentialIntegrity(t *testing.T) {
	// UUIDs to use throughout the tests
	ovsUUID := uuid.New().String()
	bridgeUUID := uuid.New().String()
	port1UUID := uuid.New().String()
	port2UUID := uuid.New().String()
	mirrorUUID := uuid.New().String()

	// the test adds an additional op to initialOps to set a reference to
	// the bridge in OVS table
	// the test deletes expectModels at the end
	tests := []struct {
		name             string
		initialOps       []ovsdb.Operation
		testOps          func(client.Client) ([]ovsdb.Operation, error)
		expectModels     []model.Model
		dontExpectModels []model.Model
		expectErr        bool
	}{
		{
			name: "strong reference is garbage collected",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name":    bridgeUUID,
						"ports":   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name": port1UUID,
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Mirror",
					UUID:  mirrorUUID,
					Row: ovsdb.Row{
						"name":            mirrorUUID,
						"select_src_port": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// remove the mirror reference
				b := &test.BridgeType{UUID: bridgeUUID}
				return c.Where(b).Update(b, &b.Mirrors)
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port1UUID}},
				&test.PortType{UUID: port1UUID, Name: port1UUID},
			},
			dontExpectModels: []model.Model{
				// mirror should have been garbage collected
				&test.MirrorType{UUID: mirrorUUID},
			},
		},
		{
			name: "adding non-root row that is not strongly reference is a noop",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name": bridgeUUID,
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// add a mirror
				m := &test.MirrorType{UUID: mirrorUUID, Name: mirrorUUID}
				return c.Create(m)
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID},
			},
			dontExpectModels: []model.Model{
				// mirror should have not been added as is not referenced from anywhere
				&test.MirrorType{UUID: mirrorUUID},
			},
		},
		{
			name: "adding non-existent strong reference fails",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name": bridgeUUID,
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// add a mirror
				b := &test.BridgeType{UUID: bridgeUUID, Mirrors: []string{mirrorUUID}}
				return c.Where(b).Update(b, &b.Mirrors)
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID},
			},
			expectErr: true,
		},
		{
			name: "weak reference is garbage collected",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name":    bridgeUUID,
						"ports":   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}, ovsdb.UUID{GoUUID: port2UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name": port1UUID,
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port2UUID,
					Row: ovsdb.Row{
						"name": port2UUID,
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Mirror",
					UUID:  mirrorUUID,
					Row: ovsdb.Row{
						"name":            mirrorUUID,
						"select_src_port": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}, ovsdb.UUID{GoUUID: port2UUID}}},
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// remove port1
				p := &test.PortType{UUID: port1UUID}
				ops, err := c.Where(p).Delete()
				if err != nil {
					return nil, err
				}
				b := &test.BridgeType{UUID: bridgeUUID, Ports: []string{port2UUID}}
				op, err := c.Where(b).Update(b, &b.Ports)
				if err != nil {
					return nil, err
				}
				return append(ops, op...), nil
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port2UUID}, Mirrors: []string{mirrorUUID}},
				&test.PortType{UUID: port2UUID, Name: port2UUID},
				// mirror reference to port1 should have been garbage collected
				&test.MirrorType{UUID: mirrorUUID, Name: mirrorUUID, SelectSrcPort: []string{port2UUID}},
			},
			dontExpectModels: []model.Model{
				&test.PortType{UUID: port1UUID},
			},
		},
		{
			name: "adding a weak reference to a non-existent row is a noop",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name":    bridgeUUID,
						"ports":   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name": port1UUID,
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Mirror",
					UUID:  mirrorUUID,
					Row: ovsdb.Row{
						"name":            mirrorUUID,
						"select_src_port": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// add reference to non-existent port2
				m := &test.MirrorType{UUID: mirrorUUID, SelectSrcPort: []string{port1UUID, port2UUID}}
				return c.Where(m).Update(m, &m.SelectSrcPort)
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port1UUID}, Mirrors: []string{mirrorUUID}},
				&test.PortType{UUID: port1UUID, Name: port1UUID},
				// mirror reference to port2 should have been garbage collected resulting in noop
				&test.MirrorType{UUID: mirrorUUID, Name: mirrorUUID, SelectSrcPort: []string{port1UUID}},
			},
		},
		{
			name: "garbage collecting a weak reference on a column lowering it below the min length fails",
			initialOps: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationInsert,
					Table: "Bridge",
					UUID:  bridgeUUID,
					Row: ovsdb.Row{
						"name":    bridgeUUID,
						"ports":   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name": port1UUID,
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Mirror",
					UUID:  mirrorUUID,
					Row: ovsdb.Row{
						"name":            mirrorUUID,
						"select_src_port": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: port1UUID}}},
					},
				},
			},
			testOps: func(c client.Client) ([]ovsdb.Operation, error) {
				// remove port 1
				return c.Where(&test.PortType{UUID: port1UUID}).Delete()
			},
			expectModels: []model.Model{
				&test.BridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port1UUID}, Mirrors: []string{mirrorUUID}},
				&test.PortType{UUID: port1UUID, Name: port1UUID},
				&test.MirrorType{UUID: mirrorUUID, Name: mirrorUUID, SelectSrcPort: []string{port1UUID}},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, close := buildTestServerAndClient(t)
			defer close()
			_, err := c.MonitorAll(context.Background())
			require.NoError(t, err)

			// add the bridge reference to the initial ops
			ops := append(tt.initialOps, ovsdb.Operation{
				Op:    ovsdb.OperationInsert,
				Table: "Open_vSwitch",
				UUID:  ovsUUID,
				Row: ovsdb.Row{
					"bridges": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: bridgeUUID}}},
				},
			})

			results, err := c.Transact(context.Background(), ops...)
			require.NoError(t, err)
			require.Len(t, results, len(ops))

			errors, err := ovsdb.CheckOperationResults(results, ops)
			require.Nil(t, errors)
			require.NoError(t, err)

			ops, err = tt.testOps(c)
			require.NoError(t, err)

			results, err = c.Transact(context.Background(), ops...)
			require.NoError(t, err)

			errors, err = ovsdb.CheckOperationResults(results, ops)
			require.Nil(t, errors)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			for _, m := range tt.expectModels {
				actual := model.Clone(m)
				err := c.Get(context.Background(), actual)
				require.NoError(t, err, "when expecting model %v", m)
				require.Equal(t, m, actual)
			}

			for _, m := range tt.dontExpectModels {
				err := c.Get(context.Background(), m)
				require.ErrorIs(t, err, client.ErrNotFound, "when not expecting model %v", m)
			}

			ops = []ovsdb.Operation{}
			for _, m := range tt.expectModels {
				op, err := c.Where(m).Delete()
				require.NoError(t, err)
				require.Len(t, op, 1)
				ops = append(ops, op...)
			}

			// remove the bridge reference
			ops = append(ops, ovsdb.Operation{
				Op:    ovsdb.OperationDelete,
				Table: "Open_vSwitch",
				Where: []ovsdb.Condition{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: ovsUUID},
					},
				},
			})

			results, err = c.Transact(context.Background(), ops...)
			require.NoError(t, err)
			require.Len(t, results, len(ops))

			errors, err = ovsdb.CheckOperationResults(results, ops)
			require.Nil(t, errors)
			require.NoError(t, err)
		})
	}
}
