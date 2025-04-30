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
	"github.com/google/uuid"
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
	pool                        *dockertest.Pool
	resource                    *dockertest.Resource
	clientWithoutInactvityCheck client.Client
	clientWithInactivityCheck   client.Client
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
	if suite.clientWithoutInactvityCheck != nil {
		suite.clientWithoutInactvityCheck.Close()
	}
	if suite.clientWithInactivityCheck != nil {
		suite.clientWithInactivityCheck.Close()
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
		suite.clientWithoutInactvityCheck = ovs

		ovs2, err := client.NewOVSDBClient(
			defDB,
			client.WithEndpoint(endpoint),
			client.WithInactivityCheck(2*time.Second, 1*time.Second, &backoff.ZeroBackOff{}),
			client.WithLeaderOnly(true),
		)
		if err != nil {
			return err
		}
		err = ovs2.Connect(ctx)
		if err != nil {
			suite.T().Log(err)
			return err
		}
		suite.clientWithInactivityCheck = ovs2

		return nil
	})
	require.NoError(suite.T(), err)

	// give ovsdb-server some time to start up

	_, err = suite.clientWithoutInactvityCheck.Monitor(context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TearDownSuite() {
	if suite.clientWithoutInactvityCheck != nil {
		suite.clientWithoutInactvityCheck.Close()
		suite.clientWithoutInactvityCheck = nil
	}
	if suite.clientWithInactivityCheck != nil {
		suite.clientWithInactivityCheck.Close()
		suite.clientWithInactivityCheck = nil
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
	DatapathID     *string           `ovsdb:"datapath_id"`
	Mirrors        []string          `ovsdb:"mirrors"`
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

type portType struct {
	UUID       string   `ovsdb:"_uuid"`
	Name       string   `ovsdb:"name"`
	Interfaces []string `ovsdb:"interfaces"`
}

type interfaceType struct {
	UUID string `ovsdb:"_uuid"`
	Name string `ovsdb:"name"`
}

type mirrorType struct {
	UUID          string   `ovsdb:"_uuid"`
	Name          string   `ovsdb:"name"`
	SelectSrcPort []string `ovsdb:"select_src_port"`
}

var defDB, _ = model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
	"Open_vSwitch": &ovsType{},
	"Bridge":       &bridgeType{},
	"IPFIX":        &ipfixType{},
	"Queue":        &queueType{},
	"Port":         &portType{},
	"Mirror":       &mirrorType{},
	"Interface":    &interfaceType{},
})

func (suite *OVSIntegrationSuite) TestConnectReconnect() {
	assert.True(suite.T(), suite.clientWithoutInactvityCheck.Connected())
	err := suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)

	bridgeName := "br-discoreco"
	brChan := make(chan *bridgeType)
	suite.clientWithoutInactvityCheck.Cache().AddEventHandler(&cache.EventHandlerFuncs{
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
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	disconnectNotification := suite.clientWithoutInactvityCheck.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	suite.clientWithoutInactvityCheck.Disconnect()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	br := &bridgeType{
		UUID: bridgeUUID,
	}

	// assert cache has been purged
	err = suite.clientWithoutInactvityCheck.Get(ctx, br)
	require.Error(suite.T(), err, client.ErrNotFound)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	assert.NoError(suite.T(), err)

	_, err = suite.clientWithoutInactvityCheck.Monitor(context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	require.NoError(suite.T(), err)

	// assert cache has been re-populated
	require.NoError(suite.T(), suite.clientWithoutInactvityCheck.Get(ctx, br))

}

func (suite *OVSIntegrationSuite) TestWithInactivityCheck() {
	assert.Equal(suite.T(), true, suite.clientWithInactivityCheck.Connected())
	err := suite.clientWithInactivityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// Disconnect client
	suite.clientWithInactivityCheck.Disconnect()

	// Ensure Disconnect doesn't have any impact to the connection.
	require.Eventually(suite.T(), func() bool {
		return suite.clientWithInactivityCheck.Connected()
	}, 5*time.Second, 1*time.Second)
	// Try to reconfigure client which already have an established connection.
	err = suite.clientWithInactivityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	require.Error(suite.T(), err)

	// Ensure Connect doesn't purge the cache.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithInactivityCheck.Connect(ctx)
	require.NoError(suite.T(), err)
	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)
	require.True(suite.T(), suite.clientWithoutInactvityCheck.Cache().Table("Bridge").Len() != 0)

	// set up a disconnect notification
	disconnectNotification := suite.clientWithoutInactvityCheck.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	// close the connection
	suite.clientWithoutInactvityCheck.Close()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	assert.NoError(suite.T(), err)

	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestWithReconnect() {
	assert.Equal(suite.T(), true, suite.clientWithoutInactvityCheck.Connected())
	err := suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// Disconnect client
	suite.clientWithoutInactvityCheck.Disconnect()

	require.Eventually(suite.T(), func() bool {
		return !suite.clientWithoutInactvityCheck.Connected()
	}, 5*time.Second, 1*time.Second)

	// Reconfigure
	err = suite.clientWithoutInactvityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	require.NoError(suite.T(), err)

	// Connect (again)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	// make another connect call, this should return without error as we're already connected
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	// check the connection is working
	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// check the cache is purged
	require.True(suite.T(), suite.clientWithoutInactvityCheck.Cache().Table("Bridge").Len() == 0)

	// set up the monitor again
	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	require.NoError(suite.T(), err)

	// add a bridge and verify our handler gets called
	bridgeName := "recon-b4"
	brChan := make(chan *bridgeType)
	suite.clientWithoutInactvityCheck.Cache().AddEventHandler(&cache.EventHandlerFuncs{
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
		return suite.clientWithoutInactvityCheck.Connected()
	}, 20*time.Second, 1*time.Second)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.NoError(suite.T(), err)

	// check our original bridge is in the cache
	err = suite.clientWithoutInactvityCheck.Get(ctx, br)
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
	disconnectNotification := suite.clientWithoutInactvityCheck.DisconnectNotify()
	notified := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		<-disconnectNotification
		notified <- struct{}{}
	}()

	<-ready
	// close the connection
	suite.clientWithoutInactvityCheck.Close()

	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		suite.T().Fatal("expected a disconnect notification but didn't receive one")
	}

	assert.Equal(suite.T(), false, suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	require.EqualError(suite.T(), err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	require.NoError(suite.T(), err)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	assert.NoError(suite.T(), err)

	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	require.NoError(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestInsertTransactIntegration() {
	bridgeName := "gopher-br7"
	uuid, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestMultipleOpsTransactIntegration() {
	bridgeName := "a_bridge_to_nowhere"
	uuid, err := suite.createBridge(bridgeName)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	var operations []ovsdb.Operation
	ovsRow := bridgeType{}
	br := &bridgeType{UUID: uuid}

	op1, err := suite.clientWithoutInactvityCheck.Where(br).
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
	op2, err := suite.clientWithoutInactvityCheck.Where(br).Mutate(&ovsRow, op2Mutations...)
	require.NoError(suite.T(), err)
	operations = append(operations, op2...)

	var op3Comment = "update external ids"
	op3 := ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &op3Comment}
	operations = append(operations, op3)

	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
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
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	deleteOp, err := suite.clientWithoutInactvityCheck.Where(&bridgeType{Name: bridgeName}).Delete()
	require.NoError(suite.T(), err)

	ovsRow := ovsType{}
	delMutateOp, err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{bridgeUUID},
		})

	require.NoError(suite.T(), err)

	delOperations := append(deleteOp, delMutateOp...)
	delReply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), delOperations...)
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
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err != nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestTableSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "InvalidTable",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs"}),
	}
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInRowValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Row:   ovsdb.Row(map[string]interface{}{"name": "docker-ovs", "invalid_column": "invalid_column"}),
	}

	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
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
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:      "select",
		Table:   "Bridge",
		Columns: []string{"name", "invalidColumn"},
	}
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) TestMonitorCancelIntegration() {
	monitorID, err := suite.clientWithoutInactvityCheck.Monitor(
		context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&queueType{}),
		),
	)
	require.NoError(suite.T(), err)

	uuid, err := suite.createQueue("test1", 0)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	err = suite.clientWithoutInactvityCheck.MonitorCancel(context.TODO(), monitorID)
	assert.NoError(suite.T(), err)

	uuid, err = suite.createQueue("test2", 1)
	require.NoError(suite.T(), err)
	assert.Never(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestMonitorConditionIntegration() {
	// Monitor table Queue rows with dscp == 1 or 2.
	queue := queueType{}
	dscp1 := 1
	dscp2 := 2
	conditions := []model.Condition{
		{
			Field:    &queue.DSCP,
			Function: ovsdb.ConditionEqual,
			Value:    &dscp1,
		},
		{
			Field:    &queue.DSCP,
			Function: ovsdb.ConditionEqual,
			Value:    &dscp2,
		},
	}

	_, err := suite.clientWithoutInactvityCheck.Monitor(
		context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithConditionalTable(&queue, conditions),
		),
	)
	require.NoError(suite.T(), err)

	uuid, err := suite.createQueue("test1", 1)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	uuid, err = suite.createQueue("test2", 3)
	require.NoError(suite.T(), err)
	assert.Never(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	uuid, err = suite.createQueue("test3", 2)
	require.NoError(suite.T(), err)
	require.Eventually(suite.T(), func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestInsertDuplicateTransactIntegration() {
	uuid, err := suite.createBridge("br-dup")
	require.NoError(suite.T(), err)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
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
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow := &bridgeType{UUID: uuid}
	err = suite.clientWithoutInactvityCheck.Get(context.Background(), bridgeRow)
	require.NoError(suite.T(), err)

	// try to modify immutable field
	bridgeRow.Name = "br-update2"
	_, err = suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow, &bridgeRow.Name)
	require.Error(suite.T(), err)
	bridgeRow.Name = "br-update"
	// update many fields
	bridgeRow.ExternalIds["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err := suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow)
	require.NoError(suite.T(), err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%+v", opErrs)

	require.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)

	newExternalIds := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIds = newExternalIds
	ops, err = suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIds)
	require.NoError(suite.T(), err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%Populate2+v", opErr)

	assert.Eventually(suite.T(), func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), br)
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

	insertOp, err := suite.clientWithoutInactvityCheck.Create(&br)
	require.NoError(suite.T(), err)

	// Inserting a Bridge row in Bridge table requires mutating the open_vswitch table.
	ovsRow := ovsType{}
	mutateOp, err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{namedUUID},
		})
	require.NoError(suite.T(), err)

	operations := append(insertOp, mutateOp...)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
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
	insertOp, err := suite.clientWithoutInactvityCheck.Create(&ipfix)
	require.NoError(suite.T(), err)

	bridge := bridgeType{
		UUID:  uuid,
		IPFIX: &namedUUID,
	}
	updateOps, err := suite.clientWithoutInactvityCheck.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	require.NoError(suite.T(), err)
	operations := append(insertOp, updateOps...)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
	require.NoError(suite.T(), err)
	opErrs, err := ovsdb.CheckOperationResults(reply, operations)
	if err != nil {
		for _, oe := range opErrs {
			suite.T().Error(oe)
		}
	}

	// Delete the IPFIX row by removing it's strong reference
	bridge.IPFIX = nil
	updateOps, err = suite.clientWithoutInactvityCheck.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	require.NoError(suite.T(), err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.TODO(), updateOps...)
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
	err = suite.clientWithoutInactvityCheck.List(context.Background(), &ipfixes)
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
	ops, err := suite.clientWithoutInactvityCheck.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionNotEqual, &timeout, bridgeRow, &bridgeRow.Name)
	require.NoError(suite.T(), err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
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
	ops, err = suite.clientWithoutInactvityCheck.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionEqual, &timeout, bridgeRow, &bridgeRow.BridgeFailMode)
	require.NoError(suite.T(), err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	opErrs, err = ovsdb.CheckOperationResults(reply, ops)
	require.NoErrorf(suite.T(), err, "%+v", opErrs)

	// Use wait to get a txn error due to until condition that is not happening
	timeout = 222 // milliseconds
	ops, err = suite.clientWithoutInactvityCheck.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionNotEqual, &timeout, bridgeRow, &bridgeRow.BridgeFailMode)
	require.NoError(suite.T(), err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	assert.Error(suite.T(), err)
}

func (suite *OVSIntegrationSuite) createQueue(queueName string, dscp int) (string, error) {
	q := queueType{
		DSCP: &dscp,
	}

	insertOp, err := suite.clientWithoutInactvityCheck.Create(&q)
	require.NoError(suite.T(), err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), insertOp...)
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
	suite.clientWithoutInactvityCheck.Disconnect()

	require.Eventually(suite.T(), func() bool {
		return !suite.clientWithoutInactvityCheck.Connected()
	}, 5*time.Second, 1*time.Second)

	err := suite.clientWithoutInactvityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	require.NoError(suite.T(), err)
	var insertOp []ovsdb.Operation
	insertOp, err = suite.clientWithoutInactvityCheck.Create(&ipfix)
	require.NoError(suite.T(), err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	// delay reconnecting for 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		err := suite.clientWithoutInactvityCheck.Connect(context.Background())
		require.NoError(suite.T(), err)
		wg.Done()
	}()

	// execute the transaction, should not fail and execute after reconnection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reply, err := suite.clientWithoutInactvityCheck.Transact(ctx, insertOp...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(reply, insertOp)
	require.NoError(suite.T(), err)

	wg.Wait()

}

func (suite *OVSIntegrationSuite) TestUnsetOptional() {
	// Create the default bridge which has an optional BridgeFailMode set
	uuid, err := suite.createBridge("br-with-optional-unset")
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	br := bridgeType{
		UUID: uuid,
	}

	// verify the bridge has BridgeFailMode set
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), br.BridgeFailMode)

	// modify bridge to unset BridgeFailMode
	br.BridgeFailMode = nil
	ops, err := suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.BridgeFailMode)
	require.NoError(suite.T(), err)
	r, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(suite.T(), err)

	// verify the bridge has BridgeFailMode unset
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), br.BridgeFailMode)
}

func (suite *OVSIntegrationSuite) TestUpdateOptional() {
	// Create the default bridge which has an optional BridgeFailMode set
	uuid, err := suite.createBridge("br-with-optional-update")
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	br := bridgeType{
		UUID: uuid,
	}

	// verify the bridge has BridgeFailMode set
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), &BridgeFailModeSecure, br.BridgeFailMode)

	// modify bridge to update BridgeFailMode
	br.BridgeFailMode = &BridgeFailModeStandalone
	ops, err := suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.BridgeFailMode)
	require.NoError(suite.T(), err)
	r, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	require.NoError(suite.T(), err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	require.NoError(suite.T(), err)

	// verify the bridge has BridgeFailMode updated
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), &BridgeFailModeStandalone, br.BridgeFailMode)
}

func (suite *OVSIntegrationSuite) TestMultipleOpsSameRow() {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	var ops []ovsdb.Operation
	var op []ovsdb.Operation

	// Use raw ops for the tables we don't have in the model, they are not the
	// target of the test and are just used to comply with the schema
	// referential integrity
	iface1UUID := "iface1"
	op = []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Interface",
			UUIDName: iface1UUID,
			Row: ovsdb.Row{
				"name": iface1UUID,
			},
		},
	}
	ops = append(ops, op...)
	port1InsertOp := len(ops)
	port1UUID := "port1"
	op = []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Port",
			UUIDName: port1UUID,
			Row: ovsdb.Row{
				"name":       port1UUID,
				"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: iface1UUID}}},
			},
		},
	}
	ops = append(ops, op...)

	iface10UUID := "iface10"
	op = []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Interface",
			UUIDName: iface10UUID,
			Row: ovsdb.Row{
				"name": iface10UUID,
			},
		},
	}
	ops = append(ops, op...)
	port10InsertOp := len(ops)
	port10UUID := "port10"
	op = []ovsdb.Operation{
		{
			Op:       ovsdb.OperationInsert,
			Table:    "Port",
			UUIDName: port10UUID,
			Row: ovsdb.Row{
				"name":       port10UUID,
				"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: iface10UUID}}},
			},
		},
	}
	ops = append(ops, op...)

	// Insert a bridge and register it in the OVS table
	bridgeInsertOp := len(ops)
	bridgeUUID := "bridge_multiple_ops_same_row"
	datapathID := "datapathID"
	br := bridgeType{
		UUID:        bridgeUUID,
		Name:        bridgeUUID,
		DatapathID:  &datapathID,
		Ports:       []string{port10UUID, port1UUID},
		ExternalIds: map[string]string{"key1": "value1"},
	}
	op, err := suite.clientWithoutInactvityCheck.Create(&br)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	ovs := ovsType{}
	op, err = suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).Mutate(&ovs, model.Mutation{
		Field:   &ovs.Bridges,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{bridgeUUID},
	})
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	results, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	require.NoError(suite.T(), err)

	_, err = ovsdb.CheckOperationResults(results, ops)
	require.NoError(suite.T(), err)

	// find out the real UUIDs
	port1UUID = results[port1InsertOp].UUID.GoUUID
	port10UUID = results[port10InsertOp].UUID.GoUUID
	bridgeUUID = results[bridgeInsertOp].UUID.GoUUID

	ops = []ovsdb.Operation{}

	// Do several ops with the bridge in the same transaction
	br.Ports = []string{port10UUID}
	br.ExternalIds = map[string]string{"key1": "value1", "key10": "value10"}
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.Ports, &br.ExternalIds)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	op, err = suite.clientWithoutInactvityCheck.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIds,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"keyA": "valueA"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{port1UUID},
		},
	)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	op, err = suite.clientWithoutInactvityCheck.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIds,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   map[string]string{"key10": "value10"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{port10UUID},
		},
	)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	datapathID = "datapathID_updated"
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	br.DatapathID = nil
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.DatapathID)
	require.NoError(suite.T(), err)
	ops = append(ops, op...)

	results, err = suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	require.NoError(suite.T(), err)

	errors, err := ovsdb.CheckOperationResults(results, ops)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), errors)
	require.Len(suite.T(), results, len(ops))

	br = bridgeType{
		UUID: bridgeUUID,
	}
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), []string{port1UUID}, br.Ports)
	require.Equal(suite.T(), map[string]string{"key1": "value1", "keyA": "valueA"}, br.ExternalIds)
	require.Nil(suite.T(), br.DatapathID)
}

func (suite *OVSIntegrationSuite) TestReferentialIntegrity() {
	t := suite.T()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// fetch the OVS UUID
	var ovs []*ovsType
	err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).List(ctx, &ovs)
	require.NoError(t, err)
	require.Len(t, ovs, 1)

	// UUIDs to use throughout the tests
	ovsUUID := ovs[0].UUID
	bridgeUUID := uuid.New().String()
	port1UUID := uuid.New().String()
	port2UUID := uuid.New().String()
	interfaceUUID := uuid.New().String()
	mirrorUUID := uuid.New().String()

	// monitor additional table specific to this test
	_, err = suite.clientWithoutInactvityCheck.Monitor(ctx,
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&portType{}),
			client.WithTable(&interfaceType{}),
			client.WithTable(&mirrorType{}),
		),
	)
	require.NoError(t, err)

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
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: interfaceUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Interface",
					UUID:  interfaceUUID,
					Row: ovsdb.Row{
						"name": interfaceUUID,
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
				b := &bridgeType{UUID: bridgeUUID}
				return c.Where(b).Update(b, &b.Mirrors)
			},
			expectModels: []model.Model{
				&bridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port1UUID}},
				&portType{UUID: port1UUID, Name: port1UUID, Interfaces: []string{interfaceUUID}},
				&interfaceType{UUID: interfaceUUID, Name: interfaceUUID},
			},
			dontExpectModels: []model.Model{
				// mirror should have been garbage collected
				&mirrorType{UUID: mirrorUUID},
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
				m := &mirrorType{UUID: mirrorUUID, Name: mirrorUUID}
				return c.Create(m)
			},
			expectModels: []model.Model{
				&bridgeType{UUID: bridgeUUID, Name: bridgeUUID},
			},
			dontExpectModels: []model.Model{
				// mirror should have not been added as is not referenced from anywhere
				&mirrorType{UUID: mirrorUUID},
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
				b := &bridgeType{UUID: bridgeUUID, Mirrors: []string{mirrorUUID}}
				return c.Where(b).Update(b, &b.Mirrors)
			},
			expectModels: []model.Model{
				&bridgeType{UUID: bridgeUUID, Name: bridgeUUID},
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
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: interfaceUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port2UUID,
					Row: ovsdb.Row{
						"name":       port2UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: interfaceUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Interface",
					UUID:  interfaceUUID,
					Row: ovsdb.Row{
						"name": interfaceUUID,
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
				b := &bridgeType{UUID: bridgeUUID, Ports: []string{port2UUID}}
				return c.Where(b).Update(b, &b.Ports)
			},
			expectModels: []model.Model{
				&bridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port2UUID}, Mirrors: []string{mirrorUUID}},
				&portType{UUID: port2UUID, Name: port2UUID, Interfaces: []string{interfaceUUID}},
				// mirror reference to port1 should have been garbage collected
				&mirrorType{UUID: mirrorUUID, Name: mirrorUUID, SelectSrcPort: []string{port2UUID}},
			},
			dontExpectModels: []model.Model{
				&portType{UUID: port1UUID},
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
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: interfaceUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Interface",
					UUID:  interfaceUUID,
					Row: ovsdb.Row{
						"name": interfaceUUID,
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
				m := &mirrorType{UUID: mirrorUUID, SelectSrcPort: []string{port1UUID, port2UUID}}
				return c.Where(m).Update(m, &m.SelectSrcPort)
			},
			expectModels: []model.Model{
				&bridgeType{UUID: bridgeUUID, Name: bridgeUUID, Ports: []string{port1UUID}, Mirrors: []string{mirrorUUID}},
				&portType{UUID: port1UUID, Name: port1UUID, Interfaces: []string{interfaceUUID}},
				&interfaceType{UUID: interfaceUUID, Name: interfaceUUID},
				// mirror reference to port2 should have been garbage collected resulting in noop
				&mirrorType{UUID: mirrorUUID, Name: mirrorUUID, SelectSrcPort: []string{port1UUID}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := suite.clientWithoutInactvityCheck

			// add the bridge reference to the initial ops
			ops := append(tt.initialOps, ovsdb.Operation{
				Op:    ovsdb.OperationMutate,
				Table: "Open_vSwitch",
				Mutations: []ovsdb.Mutation{
					{
						Mutator: ovsdb.MutateOperationInsert,
						Column:  "bridges",
						Value:   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: bridgeUUID}}},
					},
				},
				Where: []ovsdb.Condition{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: ovsUUID},
					},
				},
			})

			results, err := c.Transact(ctx, ops...)
			require.NoError(t, err)
			require.Len(t, results, len(ops))

			errors, err := ovsdb.CheckOperationResults(results, ops)
			require.Nil(t, errors)
			require.NoError(t, err)

			ops, err = tt.testOps(c)
			require.NoError(t, err)

			results, err = c.Transact(ctx, ops...)
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
				err := c.Get(ctx, actual)
				require.NoError(t, err, "when expecting model %v", m)
				require.Equal(t, m, actual)
			}

			for _, m := range tt.dontExpectModels {
				err := c.Get(ctx, m)
				require.ErrorIs(t, err, client.ErrNotFound, "when expecting model %v", m)
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
				Op:    ovsdb.OperationMutate,
				Table: "Open_vSwitch",
				Mutations: []ovsdb.Mutation{
					{
						Mutator: ovsdb.MutateOperationDelete,
						Column:  "bridges",
						Value:   ovsdb.OvsSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: bridgeUUID}}},
					},
				},
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
