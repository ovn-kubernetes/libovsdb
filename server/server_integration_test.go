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

	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/database"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/ovn-org/libovsdb/test"
)

func TestClientServerEcho(t *testing.T) {
	defDB, err := FullDatabaseModel()
	require.Nil(t, err)

	schema, err := Schema()
	require.Nil(t, err)

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})

	rand.Seed(time.Now().UnixNano())
	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	defer os.Remove(tmpfile)
	dbModel, errs := model.NewDatabaseModel(schema, defDB)
	require.Empty(t, errs)
	server, err := NewOvsdbServer(ovsDB, dbModel)
	require.Nil(t, err)

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
	err = ovs.Echo(context.Background())
	assert.Nil(t, err)
}

func TestClientServerInsert(t *testing.T) {
	defDB, err := FullDatabaseModel()
	require.Nil(t, err)

	schema, err := Schema()
	require.Nil(t, err)

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
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
	_, err = ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	wallace := "wallace"
	bridgeRow := &Bridge{
		Name:         "foo",
		DatapathType: "bar",
		DatapathID:   &wallace,
		ExternalIDs:  map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	assert.Nil(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	assert.NoErrorf(t, err, "%+v", opErr)

	uuid := reply[0].UUID.GoUUID
	require.Eventually(t, func() bool {
		br := &Bridge{UUID: uuid}
		err := ovs.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	br := &Bridge{UUID: uuid}
	err = ovs.Get(context.Background(), br)
	require.NoError(t, err)

	assert.Equal(t, bridgeRow.Name, br.Name)
	assert.Equal(t, bridgeRow.ExternalIDs, br.ExternalIDs)
	assert.Equal(t, bridgeRow.DatapathType, br.DatapathType)
	assert.Equal(t, *bridgeRow.DatapathID, wallace)
}

func TestClientServerMonitor(t *testing.T) {
	defDB, err := FullDatabaseModel()
	if err != nil {
		t.Fatal(err)
	}

	schema, err := Schema()
	if err != nil {
		t.Fatal(err)
	}

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
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

	ovsRow := &OpenvSwitch{
		UUID: "ovs",
	}
	bridgeRow := &Bridge{
		UUID:        "foo",
		Name:        "foo",
		ExternalIDs: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	seenMutex := sync.RWMutex{}
	seenInsert := false
	seenMutation := false
	seenInitialOvs := false
	ovs.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			if table == "Bridge" {
				br := model.(*Bridge)
				assert.Equal(t, bridgeRow.Name, br.Name)
				assert.Equal(t, bridgeRow.ExternalIDs, br.ExternalIDs)
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
				ov := new.(*OpenvSwitch)
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
	defDB, err := FullDatabaseModel()
	require.Nil(t, err)

	schema, err := Schema()
	require.Nil(t, err)

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
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
	_, err = ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	bridgeRow := &Bridge{
		Name:        "foo",
		ExternalIDs: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.Nil(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	require.Nil(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.Nil(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &Bridge{UUID: uuid}
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
	defDB, err := FullDatabaseModel()
	require.Nil(t, err)

	schema, err := Schema()
	require.Nil(t, err)

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
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

	bridgeRow := &Bridge{
		Name:        "foo",
		ExternalIDs: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
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
	defDB, err := FullDatabaseModel()
	require.Nil(t, err)

	schema, err := Schema()
	require.Nil(t, err)

	ovsDB := database.NewInMemoryDatabase(map[string]model.ClientDBModel{"Open_vSwitch": defDB})
	rand.Seed(time.Now().UnixNano())
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
	defer ovs.Disconnect()

	_, err = ovs.MonitorAll(context.Background())
	require.NoError(t, err)

	bridgeRow := &Bridge{
		Name:        "br-update",
		ExternalIDs: map[string]string{"go": "awesome", "docker": "made-for-each-other"},
	}

	ops, err := ovs.Create(bridgeRow)
	require.NoError(t, err)
	reply, err := ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoError(t, err)

	uuid := reply[0].UUID.GoUUID
	assert.Eventually(t, func() bool {
		br := &Bridge{UUID: uuid}
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
	bridgeRow.ExternalIDs["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow)
	require.NoError(t, err)
	reply, err = ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(t, err, "%+v", opErrs)

	require.Eventually(t, func() bool {
		br := &Bridge{UUID: uuid}
		err = ovs.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIDs = newExternalIds
	ops, err = ovs.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIDs)
	require.NoError(t, err)
	reply, err = ovs.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(t, err, "%+v", opErr)

	assert.Eventually(t, func() bool {
		br := &Bridge{UUID: uuid}
		err = ovs.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br.ExternalIDs, bridgeRow.ExternalIDs)
	}, 2*time.Second, 500*time.Millisecond)

	br := &Bridge{UUID: uuid}
	err = ovs.Get(context.Background(), br)
	assert.NoError(t, err)

	assert.Equal(t, bridgeRow, br)
}
