package ovs

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// OVSIntegrationSuite runs tests against a real Open vSwitch instance
type OVSIntegrationSuite struct {
	suite.Suite
	pool     *dockertest.Pool
	resource *dockertest.Resource
	client   client.Client
}

func (suite *OVSIntegrationSuite) SetupSuite() {
	var err error
	suite.pool, err = dockertest.NewPool("")
	require.NoError(suite.T(), err)

	tag := os.Getenv("OVS_IMAGE_TAG")
	if tag == "" {
		tag = "2.15.0"
	}

	options := &dockertest.RunOptions{
		Repository:   "libovsdb/ovs",
		Tag:          tag,
		ExposedPorts: []string{"6640/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"6640/tcp": {{HostPort: "56640"}},
		},
		Tty: true,
	}
	hostConfig := func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	}

	suite.resource, err = suite.pool.RunWithOptions(options, hostConfig)
	require.NoError(suite.T(), err)

	// set expiry to 90 seconds so containers are cleaned up on test panic
	err = suite.resource.Expire(90)
	require.NoError(suite.T(), err)

	// let the container start before we attempt connection
	time.Sleep(5 * time.Second)
}

func (suite *OVSIntegrationSuite) SetupTest() {
	if suite.client != nil {
		suite.client.Close()
	}
	var err error
	err = suite.pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		endpoint := "tcp::56640"
		ovs, err := client.NewOVSDBClient(
			defDB,
			client.WithEndpoint(endpoint),
			client.WithLeaderOnly(true),
		)
		if err != nil {
			return err
		}
		err = ovs.Connect(ctx)
		if err != nil {
			suite.T().Log(err)
			return err
		}
		suite.client = ovs
		return nil
	})
	require.NoError(suite.T(), err)

	// give ovsdb-server some time to start up

	_, err = suite.client.Monitor(context.TODO(),
		suite.client.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
		suite.client = nil
	}
	err := suite.pool.Purge(suite.resource)
	require.NoError(suite.T(), err)
}

func TestOVSIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	suite.Run(t, new(OVSIntegrationSuite))
}

type BridgeFailMode = string

var (
	BridgeFailModeStandalone BridgeFailMode = "standalone"
	BridgeFailModeSecure     BridgeFailMode = "secure"
)

// bridgeType is the simplified ORM model of the Bridge table
type bridgeType struct {
	UUID           string            `ovsdb:"_uuid"`
	Name           string            `ovsdb:"name"`
	OtherConfig    map[string]string `ovsdb:"other_config"`
	ExternalIds    map[string]string `ovsdb:"external_ids"`
	Ports          []string          `ovsdb:"ports"`
	Status         map[string]string `ovsdb:"status"`
	BridgeFailMode *BridgeFailMode   `ovsdb:"fail_mode"`
	IPFIX          *string           `ovsdb:"ipfix"`
}

// ovsType is the ORM model of the OVS table
type ovsType struct {
	UUID            string            `ovsdb:"_uuid"`
	Bridges         []string          `ovsdb:"bridges"`
	CurCfg          int               `ovsdb:"cur_cfg"`
	DatapathTypes   []string          `ovsdb:"datapath_types"`
	Datapaths       map[string]string `ovsdb:"datapaths"`
	DbVersion       *string           `ovsdb:"db_version"`
	DpdkInitialized bool              `ovsdb:"dpdk_initialized"`
	DpdkVersion     *string           `ovsdb:"dpdk_version"`
	ExternalIDs     map[string]string `ovsdb:"external_ids"`
	IfaceTypes      []string          `ovsdb:"iface_types"`
	ManagerOptions  []string          `ovsdb:"manager_options"`
	NextCfg         int               `ovsdb:"next_cfg"`
	OtherConfig     map[string]string `ovsdb:"other_config"`
	OVSVersion      *string           `ovsdb:"ovs_version"`
	SSL             *string           `ovsdb:"ssl"`
	Statistics      map[string]string `ovsdb:"statistics"`
	SystemType      *string           `ovsdb:"system_type"`
	SystemVersion   *string           `ovsdb:"system_version"`
}

// ipfixType is a simplified ORM model for the IPFIX table
type ipfixType struct {
	UUID    string   `ovsdb:"_uuid"`
	Targets []string `ovsdb:"targets"`
}

// queueType is the simplified ORM model of the Queue table
type queueType struct {
	UUID string `ovsdb:"_uuid"`
	DSCP *int   `ovsdb:"dscp"`
}

var defDB, _ = model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
	"Open_vSwitch": &ovsType{},
	"Bridge":       &bridgeType{},
	"IPFIX":        &ipfixType{},
	"Queue":        &queueType{},
})

func (suite *OVSIntegrationSuite) TestConnectReconnect() {
	assert.True(suite.T(), suite.client.Connected())
	err := suite.client.Echo(context.TODO())
	require.NoError(suite.T(), err)

	bridgeName := "br-discoreco"
	brChan := make(chan *bridgeType)
	suite.client.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			br, ok := model.(*bridgeType)
			if !ok {
				return
			}
			if br.Name == bridgeName {
				brChan <- br
			}
		},
	})

	bridgeUUID, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	<-brChan

	// make another connect call, this should return without error as we're already connected
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	disconnectNotification := suite.client.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	suite.client.Disconnect()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.client.Connected())

	err = suite.client.Echo(context.TODO())
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	br := &bridgeType{
		UUID: bridgeUUID,
	}

	// assert cache has been purged
	err = suite.client.Get(ctx, br)
	require.Error(suite.T(), err, client.ErrNotFound)

	err = suite.client.Echo(context.TODO())
	assert.NoError(suite.T(), err)

	_, err = suite.client.Monitor(context.TODO(),
		suite.client.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	require.NoError(suite.T(), err)

	// assert cache has been re-populated
	require.NoError(suite.T(), suite.client.Get(ctx, br))

}

func (suite *OVSIntegrationSuite) TestWithReconnect() {
	assert.Equal(suite.T(), true, suite.client.Connected())
	err := suite.client.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// Disconnect client
	suite.client.Disconnect()

	require.Eventually(suite.T(), func() bool {
		return !suite.client.Connected()
	}, 5*time.Second, 1*time.Second)

	// Reconfigure
	err = suite.client.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	require.NoError(suite.T(), err)

	// Connect (again)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	// make another connect call, this should return without error as we're already connected
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	// check the connection is working
	err = suite.client.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// check the cache is purged
	require.True(suite.T(), suite.client.Cache().Table("Bridge").Len() == 0)

	// set up the monitor again
	_, err = suite.client.MonitorAll(context.TODO())
	require.NoError(suite.T(), err)

	// add a bridge and verify our handler gets called
	bridgeName := "recon-b4"
	brChan := make(chan *bridgeType)
	suite.client.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			br, ok := model.(*bridgeType)
			if !ok {
				return
			}
			if strings.HasPrefix(br.Name, "recon-") {
				brChan <- br
			}
		},
	})

	_, err = suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	br := <-brChan
	require.Equal(suite.T(), bridgeName, br.Name)

	// trigger reconnect
	err = suite.pool.Client.RestartContainer(suite.resource.Container.ID, 0)
	require.NoError(suite.T(), err)

	// check that we are automatically reconnected
	require.Eventually(suite.T(), func() bool {
		return suite.client.Connected()
	}, 20*time.Second, 1*time.Second)

	err = suite.client.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// check our original bridge is in the cache
	err = suite.client.Get(ctx, br)
	require.NoError(suite.T(), err)

	// create a new bridge to ensure the monitor and cache handler is still working
	bridgeName = "recon-after"
	_, err = suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)

LOOP:
	for {
		select {
		case <-time.After(5 * time.Second):
			suite.T().Fatal("timed out waiting for bridge")
		case b := <-brChan:
			if b.Name == bridgeName {
				break LOOP
			}
		}
	}

	// set up a disconnect notification
	disconnectNotification := suite.client.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	// close the connection
	suite.client.Close()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.client.Connected())

	err = suite.client.Echo(context.TODO())
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.client.Connect(ctx)
	require.NoError(suite.T(), err)

	err = suite.client.Echo(context.TODO())
	assert.NoError(suite.T(), err)

	_, err = suite.client.MonitorAll(context.TODO())
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestInsertTransactIntegration() {
	bridgeName := "gopher-br7"
	uuid, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestMultipleOpsTransactIntegration() {
	bridgeName := "a_bridge_to_nowhere"
	uuid, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	var operations []ovsdb.Operation
	ovsRow := bridgeType{}
	br := &bridgeType{UUID: uuid}

	op1, err := suite.client.Where(br).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.ExternalIds,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"one": "1"},
		})
	require.NoError(suite.T(), err)
	operations = append(operations, op1...)

	op2Mutations := []model.Mutation{
		{
			Field:   &ovsRow.ExternalIds,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"two": "2", "three": "3"},
		},
		{
			Field:   &ovsRow.ExternalIds,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{"docker"},
		},
		{
			Field:   &ovsRow.ExternalIds,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"podman": "made-for-each-other"},
		},
	}
	op2, err := suite.client.Where(br).Mutate(&ovsRow, op2Mutations...)
	require.NoError(suite.T(), err)
	operations = append(operations, op2...)

	var op3Comment = "update external ids"
	op3 := ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &op3Comment}
	operations = append(operations, op3)

	reply, err := suite.client.Transact(context.TODO(), operations...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	expectedExternalIds := map[string]string{
		"go":     "awesome",
		"podman": "made-for-each-other",
		"one":    "1",
		"two":    "2",
		"three":  "3",
	}
	require.Exactly(suite.T(), expectedExternalIds, br.ExternalIds)
}

func (suite *OVSIntegrationSuite) TestInsertAndDeleteTransactIntegration() {
	bridgeName := "gopher-br5"
	bridgeUUID, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: bridgeUUID}
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	deleteOp, err := suite.client.Where(&bridgeType{Name: bridgeName}).Delete()
	require.NoError(suite.T(), err)

	ovsRow := ovsType{}
	delMutateOp, err := suite.client.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{bridgeUUID},
		})

	require.NoError(suite.T(), err)

	delOperations := append(deleteOp, delMutateOp...)
	delReply, err := suite.client.Transact(context.TODO(), delOperations...)
	require.NoError(suite.T(), err)

	delOperationErrs, err := ovsdb.CheckOperationResults(delReply, delOperations)
	if err != nil {
		for _, oe := range delOperationErrs {
			suite.T().Error(oe)
		}
		suite.T().Fatal(err)
	}

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: bridgeUUID}
		err := suite.client.Get(context.Background(), br)
		return err != nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestTableSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "InvalidTable",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs"}),
	}
	_, err := suite.client.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInRowValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs", "invalid_column": "invalid_column"}),
	}

	_, err := suite.client.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInMultipleRowsValidationIntegration() {
	invalidBridge := ovsdb.Row(map[string]interface{}{"invalid_column": "invalid_column"})
	bridge := ovsdb.Row(map[string]interface{}{"name": "docker-ovs"})
	rows := []ovsdb.Row{invalidBridge, bridge}

	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Rows:  rows,
	}
	_, err := suite.client.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:      "select",
		Table:   "Bridge",
		Columns: []string{"name", "invalidColumn"},
	}
	_, err := suite.client.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestMonitorCancelIntegration() {
	monitorID, err := suite.client.Monitor(
		context.TODO(),
		suite.client.NewMonitor(
			client.WithTable(&queueType{}),
		),
	)
	require.NoError(suite.T(), err)

	uuid, err := suite.createQueue("test1", 0)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.client.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	err = suite.client.MonitorCancel(context.TODO(), monitorID)
	assert.NoError(suite.T(), err)

	uuid, err = suite.createQueue("test2", 1)
	require.NoError(suite.T(), err)
	assert.Never(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.client.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestInsertDuplicateTransactIntegration() {
	uuid, err := suite.createBridge("br-dup")
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	_, err = suite.createBridge("br-dup")
	assert.Error(suite.T(), err)
	assert.IsType(suite.T(), &ovsdb.ConstraintViolation{}, err)
}

func (suite *OVSIntegrationSuite) TestUpdate() {
	uuid, err := suite.createBridge("br-update")
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.client.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow := &bridgeType{UUID: uuid}
	err = suite.client.Get(context.Background(), bridgeRow)
	require.NoError(suite.T(), err)

	// try to modify immutable field
	bridgeRow.Name = "br-update2"
	_, err = suite.client.Where(bridgeRow).Update(bridgeRow, &bridgeRow.Name)
	require.Error(suite.T(), err)
	bridgeRow.Name = "br-update"
	// update many fields
	bridgeRow.ExternalIds["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err := suite.client.Where(bridgeRow).Update(bridgeRow)
	require.NoError(suite.T(), err)
	reply, err := suite.client.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%+v", opErrs)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.client.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIds = newExternalIds
	ops, err = suite.client.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIds)
	require.NoError(suite.T(), err)
	reply, err = suite.client.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%Populate2+v", opErr)

	assert.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.client.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) createBridge(bridgeName string) (string, error) {
	// NamedUUID is used to add multiple related Operations in a single Transact operation
	namedUUID := "gopher"
	br := bridgeType{
		UUID: namedUUID,
		Name: bridgeName,
		ExternalIds: map[string]string{
			"go":     "awesome",
			"docker": "made-for-each-other",
		},
		BridgeFailMode: &BridgeFailModeSecure,
	}

	insertOp, err := suite.client.Create(&br)
	require.NoError(suite.T(), err)

	// Inserting a Bridge row in Bridge table requires mutating the open_vswitch table.
	ovsRow := ovsType{}
	mutateOp, err := suite.client.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{namedUUID},
		})
	require.NoError(suite.T(), err)

	operations := append(insertOp, mutateOp...)
	reply, err := suite.client.Transact(context.TODO(), operations...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	return reply[0].UUID.GoUUID, err
}

func (suite *OVSIntegrationSuite) TestCreateIPFIX() {
	// Create a IPFIX row and update the bridge in the same transaction
	uuid, err := suite.createBridge("br-ipfix")
	require.NoError(suite.T(), err)
	namedUUID := "gopher"
	ipfix := ipfixType{
		UUID:    namedUUID,
		Targets: []string{"127.0.0.1:6650"},
	}
	insertOp, err := suite.client.Create(&ipfix)
	require.NoError(suite.T(), err)

	bridge := bridgeType{
		UUID:  uuid,
		IPFIX: &namedUUID,
	}
	updateOps, err := suite.client.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	require.NoError(suite.T(), err)
	operations := append(insertOp, updateOps...)
	reply, err := suite.client.Transact(context.TODO(), operations...)
	require.NoError(suite.T(), err)
	opErrs, err := ovsdb.CheckOperationResults(reply, operations)
	if err != nil {
		for _, oe := range opErrs {
			suite.T().Error(oe)
		}
	}

	// Delete the IPFIX row by removing it's strong reference
	bridge.IPFIX = nil
	updateOps, err = suite.client.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	require.NoError(suite.T(), err)
	reply, err = suite.client.Transact(context.TODO(), updateOps...)
	require.NoError(suite.T(), err)
	opErrs, err = ovsdb.CheckOperationResults(reply, updateOps)
	if err != nil {
		for _, oe := range opErrs {
			suite.T().Error(oe)
		}
	}
	require.NoError(suite.T(), err)

	//Assert the IPFIX table is empty
	ipfixes := []ipfixType{}
	err = suite.client.List(context.Background(), &ipfixes)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), ipfixes)

}

func (suite *OVSIntegrationSuite) TestWait() {
	var err error
	brName := "br-wait-for-it"

	// Use Wait to ensure bridge does not exist yet
	bridgeRow := &bridgeType{
		Name: brName,
	}
	conditions := []model.Condition{
		{
			Field:    &bridgeRow.Name,
			Function: ovsdb.ConditionEqual,
			Value:    brName,
		},
	}
	timeout := 0
	ops, err := suite.client.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionNotEqual, &timeout, bridgeRow, &bridgeRow.Name)
	require.NoError(suite.T(), err)
	reply, err := suite.client.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%+v", opErrs)

	// Now, create the bridge
	_, err = suite.createBridge(brName)
	require.NoError(suite.T(), err)

	// Use wait to verify bridge's existence
	bridgeRow = &bridgeType{
		Name:           brName,
		BridgeFailMode: &BridgeFailModeSecure,
	}
	conditions = []model.Condition{
		{
			Field:    &bridgeRow.BridgeFailMode,
			Function: ovsdb.ConditionEqual,
			Value:    &BridgeFailModeSecure,
		},
	}
	timeout = 2 * 1000 // 2 seconds (in milliseconds)
	ops, err = suite.client.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionEqual, &timeout, bridgeRow, &bridgeRow.BridgeFailMode)
	require.NoError(suite.T(), err)
	reply, err = suite.client.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErrs, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%+v", opErrs)

	// Use wait to get a txn error due to until condition that is not happening
	timeout = 222 // milliseconds
	ops, err = suite.client.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionNotEqual, &timeout, bridgeRow, &bridgeRow.BridgeFailMode)
	require.NoError(suite.T(), err)
	reply, err = suite.client.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) createQueue(queueName string, dscp int) (string, error) {
	q := queueType{
		DSCP: &dscp,
	}

	insertOp, err := suite.client.Create(&q)
	require.NoError(suite.T(), err)
	reply, err := suite.client.Transact(context.TODO(), insertOp...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, insertOp)
	return reply[0].UUID.GoUUID, err
}

func (suite *OVSIntegrationSuite) TestOpsWaitForReconnect() {
	namedUUID := "trozet"
	ipfix := ipfixType{
		UUID:    namedUUID,
		Targets: []string{"127.0.0.1:6650"},
	}

	// Shutdown client
	suite.client.Disconnect()

	require.Eventually(suite.T(), func() bool {
		return !suite.client.Connected()
	}, 5*time.Second, 1*time.Second)

	err := suite.client.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	require.NoError(suite.T(), err)
	var insertOp []ovsdb.Operation
	insertOp, err = suite.client.Create(&ipfix)
	require.NoError(suite.T(), err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	// delay reconnecting for 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		err := suite.client.Connect(context.Background())
		require.NoError(suite.T(), err)
		wg.Done()
	}()

	// execute the transaction, should not fail and execute after reconnection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reply, err := suite.client.Transact(ctx, insertOp...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, insertOp)
	require.NoError(suite.T(), err)

	wg.Wait()

}

func (suite *OVSIntegrationSuite) TestUnsetOptional() {
	// Create the default bridge which has an optional BridgeFailMode set
	uuid, err := suite.createBridge("br-with-optional")
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	br := bridgeType{
		UUID: uuid,
	}

	// verify the bridge has BridgeFailMode set
	err = suite.client.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), br.BridgeFailMode)

	// modify bridge to unset BridgeFailMode
	br.BridgeFailMode = nil
	ops, err := suite.client.Where(&br).Update(&br, &br.BridgeFailMode)
	require.NoError(suite.T(), err)
	r, err := suite.client.Transact(ctx, ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(suite.T(), err)

	// verify the bridge has BridgeFailMode unset
	err = suite.client.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), br.BridgeFailMode)
}
