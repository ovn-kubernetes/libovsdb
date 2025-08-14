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
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
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
	suite.Require().NoError(err)

	tag := os.Getenv("OVS_VERSION")
	if tag == "" {
		tag = "latest"
	}

	options := &dockertest.RunOptions{
		Repository:   "ghcr.io/ovn-kubernetes/ovs",
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
	suite.Require().NoError(err)

	// set expiry to 90 seconds so containers are cleaned up on test panic
	err = suite.resource.Expire(90)
	suite.Require().NoError(err)

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
	suite.Require().NoError(err)

	// give ovsdb-server some time to start up

	_, err = suite.clientWithoutInactvityCheck.Monitor(context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	suite.Require().NoError(err)
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
	suite.Require().NoError(err)
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
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	OtherConfig map[string]string `ovsdb:"other_config"`
	ExternalIDs map[string]string `ovsdb:"external_ids"`
	Ports       []string          `ovsdb:"ports"`
	Status      map[string]string `ovsdb:"status"`
	FailMode    *BridgeFailMode   `ovsdb:"fail_mode" validate:"omitempty,oneof='standalone' 'secure'"`
	IPFIX       *string           `ovsdb:"ipfix"`
	DatapathID  *string           `ovsdb:"datapath_id"`
	Mirrors     []string          `ovsdb:"mirrors"`
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
	Targets []string `ovsdb:"targets" validate:"min=0"`
}

// queueType is the simplified ORM model of the Queue table
type queueType struct {
	UUID string `ovsdb:"_uuid"`
	DSCP *int   `ovsdb:"dscp" validate:"omitempty,min=0,max=63"`
}

type portType struct {
	UUID       string   `ovsdb:"_uuid"`
	Name       string   `ovsdb:"name"`
	Interfaces []string `ovsdb:"interfaces" validate:"min=1"`
}

type interfaceType struct {
	UUID string `ovsdb:"_uuid"`
	Name string `ovsdb:"name"`
}

type mirrorType struct {
	UUID          string   `ovsdb:"_uuid"`
	Name          string   `ovsdb:"name"`
	SelectSrcPort []string `ovsdb:"select_src_port" validate:"min=0"`
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
	suite.True(suite.clientWithoutInactvityCheck.Connected())
	err := suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	bridgeName := "br-discoreco"
	brChan := make(chan *bridgeType)
	suite.clientWithoutInactvityCheck.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(_ string, model model.Model) {
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
	suite.Require().NoError(err)
	<-brChan

	// make another connect call, this should return without error as we're already connected
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

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

	suite.False(suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().EqualError(err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

	br := &bridgeType{
		UUID: bridgeUUID,
	}

	// assert cache has been purged
	err = suite.clientWithoutInactvityCheck.Get(ctx, br)
	suite.Require().ErrorIs(err, client.ErrNotFound)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	_, err = suite.clientWithoutInactvityCheck.Monitor(context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&ovsType{}),
			client.WithTable(&bridgeType{}),
		),
	)
	suite.Require().NoError(err)

	// assert cache has been re-populated
	suite.Require().NoError(suite.clientWithoutInactvityCheck.Get(ctx, br))

}

func (suite *OVSIntegrationSuite) TestWithInactivityCheck() {
	suite.True(suite.clientWithInactivityCheck.Connected())
	err := suite.clientWithInactivityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	// Disconnect client
	suite.clientWithInactivityCheck.Disconnect()

	// Ensure Disconnect doesn't have any impact to the connection.
	suite.Eventually(func() bool {
		return suite.clientWithInactivityCheck.Connected()
	}, 5*time.Second, 1*time.Second)
	// Try to reconfigure client which already have an established connection.
	err = suite.clientWithInactivityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	suite.Require().Error(err)

	// Ensure Connect doesn't purge the cache.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithInactivityCheck.Connect(ctx)
	suite.Require().NoError(err)
	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)
	suite.NotEqual(0, suite.clientWithoutInactvityCheck.Cache().Table("Bridge").Len())

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

	suite.False(suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().EqualError(err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	suite.Require().NoError(err)
}

func (suite *OVSIntegrationSuite) TestWithReconnect() {
	suite.T().Skip("On container restart client is connected but rpc2 connection is shutdown, so Echo fails with ErrNotConnected")
	suite.True(suite.clientWithoutInactvityCheck.Connected())
	err := suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	// Disconnect client
	suite.clientWithoutInactvityCheck.Disconnect()

	suite.Eventually(func() bool {
		return !suite.clientWithoutInactvityCheck.Connected()
	}, 5*time.Second, 1*time.Second)

	// Reconfigure
	err = suite.clientWithoutInactvityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	suite.Require().NoError(err)

	// Connect (again)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

	// make another connect call, this should return without error as we're already connected
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

	// check the connection is working
	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	// check the cache is purged
	suite.Equal(0, suite.clientWithoutInactvityCheck.Cache().Table("Bridge").Len())

	// set up the monitor again
	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	suite.Require().NoError(err)

	// add a bridge and verify our handler gets called
	bridgeName := "recon-b4"
	brChan := make(chan *bridgeType)
	suite.clientWithoutInactvityCheck.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(_ string, model model.Model) {
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
	suite.Require().NoError(err)
	br := <-brChan
	suite.Equal(bridgeName, br.Name)

	// trigger reconnect
	err = suite.pool.Client.RestartContainer(suite.resource.Container.ID, 0)
	suite.Require().NoError(err)

	// check that we are automatically reconnected
	suite.Eventually(func() bool {
		return suite.clientWithoutInactvityCheck.Connected()
	}, 20*time.Second, 1*time.Second)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	// check our original bridge is in the cache
	err = suite.clientWithoutInactvityCheck.Get(ctx, br)
	suite.Require().NoError(err)

	// create a new bridge to ensure the monitor and cache handler is still working
	bridgeName = "recon-after"
	_, err = suite.createBridge(bridgeName)
	suite.Require().NoError(err)

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

	suite.False(suite.clientWithoutInactvityCheck.Connected())

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().EqualError(err, client.ErrNotConnected.Error())

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = suite.clientWithoutInactvityCheck.Connect(ctx)
	suite.Require().NoError(err)

	err = suite.clientWithoutInactvityCheck.Echo(context.TODO())
	suite.Require().NoError(err)

	_, err = suite.clientWithoutInactvityCheck.MonitorAll(context.TODO())
	suite.Require().NoError(err)
}

func (suite *OVSIntegrationSuite) TestInsertTransactIntegration() {
	bridgeName := "gopher-br7"
	uuid, err := suite.createBridge(bridgeName)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestMultipleOpsTransactIntegration() {
	bridgeName := "a_bridge_to_nowhere"
	uuid, err := suite.createBridge(bridgeName)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	var operations []ovsdb.Operation
	ovsRow := bridgeType{}
	br := &bridgeType{UUID: uuid}

	op1, err := suite.clientWithoutInactvityCheck.Where(br).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.ExternalIDs,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"one": "1"},
		})
	suite.Require().NoError(err)
	operations = append(operations, op1...)

	op2Mutations := []model.Mutation{
		{
			Field:   &ovsRow.ExternalIDs,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"two": "2", "three": "3"},
		},
		{
			Field:   &ovsRow.ExternalIDs,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{"docker"},
		},
		{
			Field:   &ovsRow.ExternalIDs,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"podman": "made-for-each-other"},
		},
	}
	op2, err := suite.clientWithoutInactvityCheck.Where(br).Mutate(&ovsRow, op2Mutations...)
	suite.Require().NoError(err)
	operations = append(operations, op2...)

	var op3Comment = "update external ids"
	op3 := ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &op3Comment}
	operations = append(operations, op3)

	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
	suite.Require().NoError(err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	suite.Require().NoError(err)

	suite.Eventually(func() bool {
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	expectedExternalIDs := map[string]string{
		"go":     "awesome",
		"podman": "made-for-each-other",
		"one":    "1",
		"two":    "2",
		"three":  "3",
	}
	suite.Exactly(expectedExternalIDs, br.ExternalIDs)
}

func (suite *OVSIntegrationSuite) TestInsertAndDeleteTransactIntegration() {
	bridgeName := "gopher-br5"
	bridgeUUID, err := suite.createBridge(bridgeName)
	suite.Require().NoError(err)

	suite.Eventually(func() bool {
		br := &bridgeType{UUID: bridgeUUID}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	deleteOp, err := suite.clientWithoutInactvityCheck.Where(&bridgeType{Name: bridgeName}).Delete()
	suite.Require().NoError(err)

	ovsRow := ovsType{}
	delMutateOp, err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{bridgeUUID},
		})

	suite.Require().NoError(err)

	delOperations := append(deleteOp, delMutateOp...)
	delReply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), delOperations...)
	suite.Require().NoError(err)

	delOperationErrs, err := ovsdb.CheckOperationResults(delReply, delOperations)
	if err != nil {
		for _, oe := range delOperationErrs {
			suite.T().Error(oe)
		}
		suite.T().Fatal(err)
	}

	suite.Eventually(func() bool {
		br := &bridgeType{UUID: bridgeUUID}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err != nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestTableSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "InvalidTable",
		Row:   ovsdb.Row(map[string]any{"name": "docker-ovs"}),
	}
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	suite.Require().Error(err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInRowValidationIntegration() {
	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Row:   ovsdb.Row(map[string]any{"name": "docker-ovs", "invalid_column": "invalid_column"}),
	}

	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	suite.Require().Error(err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaInMultipleRowsValidationIntegration() {
	invalidBridge := ovsdb.Row(map[string]any{"invalid_column": "invalid_column"})
	bridge := ovsdb.Row(map[string]any{"name": "docker-ovs"})
	rows := []ovsdb.Row{invalidBridge, bridge}

	operation := ovsdb.Operation{
		Op:    "insert",
		Table: "Bridge",
		Rows:  rows,
	}
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	suite.Require().Error(err)
}

func (suite *OVSIntegrationSuite) TestColumnSchemaValidationIntegration() {
	operation := ovsdb.Operation{
		Op:      "select",
		Table:   "Bridge",
		Columns: []string{"name", "invalidColumn"},
	}
	_, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operation)
	suite.Require().Error(err)
}

func (suite *OVSIntegrationSuite) TestMonitorCancelIntegration() {
	monitorID, err := suite.clientWithoutInactvityCheck.Monitor(
		context.TODO(),
		suite.clientWithoutInactvityCheck.NewMonitor(
			client.WithTable(&queueType{}),
		),
	)
	suite.Require().NoError(err)

	uuid, err := suite.createQueue("test1", 0)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	err = suite.clientWithoutInactvityCheck.MonitorCancel(context.TODO(), monitorID)
	suite.Require().NoError(err)

	uuid, err = suite.createQueue("test2", 1)
	suite.Require().NoError(err)
	suite.Never(func() bool {
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
	suite.Require().NoError(err)

	uuid, err := suite.createQueue("test1", 1)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	uuid, err = suite.createQueue("test2", 3)
	suite.Require().NoError(err)
	suite.Never(func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	uuid, err = suite.createQueue("test3", 2)
	suite.Require().NoError(err)
	suite.Eventually(func() bool {
		q := &queueType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), q)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
}

func (suite *OVSIntegrationSuite) TestInsertDuplicateTransactIntegration() {
	uuid, err := suite.createBridge("br-dup")
	suite.Require().NoError(err)

	suite.Eventually(func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	_, err = suite.createBridge("br-dup")
	suite.Require().Error(err)
	suite.IsType(&ovsdb.ConstraintViolation{}, err)
}

func (suite *OVSIntegrationSuite) TestUpdate() {
	uuid, err := suite.createBridge("br-update")
	suite.Require().NoError(err)

	suite.Eventually(func() bool {
		br := &bridgeType{UUID: uuid}
		err := suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)

	bridgeRow := &bridgeType{UUID: uuid}
	err = suite.clientWithoutInactvityCheck.Get(context.Background(), bridgeRow)
	suite.Require().NoError(err)

	// try to modify immutable field
	bridgeRow.Name = "br-update2"
	_, err = suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow, &bridgeRow.Name)
	suite.Require().Error(err)
	bridgeRow.Name = "br-update"
	// update many fields
	bridgeRow.ExternalIDs["baz"] = "foobar"
	bridgeRow.OtherConfig = map[string]string{"foo": "bar"}
	ops, err := suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow)
	suite.Require().NoError(err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	suite.Require().NoError(err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	suite.Require().NoErrorf(err, "%+v", opErrs)

	suite.Eventually(func() bool {
		br := &bridgeType{UUID: uuid}
		err = suite.clientWithoutInactvityCheck.Get(context.Background(), br)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(br, bridgeRow)
	}, 2*time.Second, 50*time.Millisecond)

	newExternalIDs := map[string]string{"foo": "bar"}
	bridgeRow.ExternalIDs = newExternalIDs
	ops, err = suite.clientWithoutInactvityCheck.Where(bridgeRow).Update(bridgeRow, &bridgeRow.ExternalIDs)
	suite.Require().NoError(err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	suite.Require().NoError(err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	suite.Require().NoErrorf(err, "Populate2: %+v", opErr)

	suite.Eventually(func() bool {
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
		ExternalIDs: map[string]string{
			"go":     "awesome",
			"docker": "made-for-each-other",
		},
		FailMode: &BridgeFailModeSecure,
	}

	insertOp, err := suite.clientWithoutInactvityCheck.Create(&br)
	suite.Require().NoError(err)

	// Inserting a Bridge row in Bridge table requires mutating the open_vswitch table.
	ovsRow := ovsType{}
	mutateOp, err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).
		Mutate(&ovsRow, model.Mutation{
			Field:   &ovsRow.Bridges,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{namedUUID},
		})
	suite.Require().NoError(err)

	operations := append(insertOp, mutateOp...)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
	suite.Require().NoError(err)

	_, err = ovsdb.CheckOperationResults(reply, operations)
	return reply[0].UUID.GoUUID, err
}

func (suite *OVSIntegrationSuite) TestCreateIPFIX() {
	// Create a IPFIX row and update the bridge in the same transaction
	uuid, err := suite.createBridge("br-ipfix")
	suite.Require().NoError(err)
	namedUUID := "gopher"
	ipfix := ipfixType{
		UUID:    namedUUID,
		Targets: []string{"127.0.0.1:6650"},
	}
	insertOp, err := suite.clientWithoutInactvityCheck.Create(&ipfix)
	suite.Require().NoError(err)

	bridge := bridgeType{
		UUID:  uuid,
		IPFIX: &namedUUID,
	}
	updateOps, err := suite.clientWithoutInactvityCheck.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	suite.Require().NoError(err)
	operations := append(insertOp, updateOps...)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), operations...)
	suite.Require().NoError(err)
	opErrs, err := ovsdb.CheckOperationResults(reply, operations)
	if err != nil {
		for _, oe := range opErrs {
			suite.T().Error(oe)
		}
	}

	// Delete the IPFIX row by removing it's strong reference
	bridge.IPFIX = nil
	updateOps, err = suite.clientWithoutInactvityCheck.Where(&bridge).Update(&bridge, &bridge.IPFIX)
	suite.Require().NoError(err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.TODO(), updateOps...)
	suite.Require().NoError(err)
	opErrs, err = ovsdb.CheckOperationResults(reply, updateOps)
	if err != nil {
		for _, oe := range opErrs {
			suite.T().Error(oe)
		}
	}
	suite.Require().NoError(err)

	//Assert the IPFIX table is empty
	ipfixes := []ipfixType{}
	err = suite.clientWithoutInactvityCheck.List(context.Background(), &ipfixes)
	suite.Require().NoError(err)
	suite.Empty(ipfixes)

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
	suite.Require().NoError(err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	suite.Require().NoError(err)
	opErrs, err := ovsdb.CheckOperationResults(reply, ops)
	suite.Require().NoErrorf(err, "%+v", opErrs)

	// Now, create the bridge
	_, err = suite.createBridge(brName)
	suite.Require().NoError(err)

	// Use wait to verify bridge's existence
	bridgeRow = &bridgeType{
		Name:     brName,
		FailMode: &BridgeFailModeSecure,
	}
	conditions = []model.Condition{
		{
			Field:    &bridgeRow.FailMode,
			Function: ovsdb.ConditionEqual,
			Value:    &BridgeFailModeSecure,
		},
	}
	timeout = 2 * 1000 // 2 seconds (in milliseconds)
	ops, err = suite.clientWithoutInactvityCheck.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionEqual, &timeout, bridgeRow, &bridgeRow.FailMode)
	suite.Require().NoError(err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	suite.Require().NoError(err)
	opErrs, err = ovsdb.CheckOperationResults(reply, ops)
	suite.Require().NoErrorf(err, "%+v", opErrs)

	// Use wait to get a txn error due to until condition that is not happening
	timeout = 222 // milliseconds
	ops, err = suite.clientWithoutInactvityCheck.WhereAny(bridgeRow, conditions...).Wait(
		ovsdb.WaitConditionNotEqual, &timeout, bridgeRow, &bridgeRow.FailMode)
	suite.Require().NoError(err)
	reply, err = suite.clientWithoutInactvityCheck.Transact(context.Background(), ops...)
	suite.Require().NoError(err)
	_, err = ovsdb.CheckOperationResults(reply, ops)
	suite.Require().Error(err)
}

func (suite *OVSIntegrationSuite) createQueue(uuid string, dscp int) (string, error) {
	q := queueType{
		UUID: uuid,
		DSCP: &dscp,
	}

	insertOp, err := suite.clientWithoutInactvityCheck.Create(&q)
	suite.Require().NoError(err)
	reply, err := suite.clientWithoutInactvityCheck.Transact(context.TODO(), insertOp...)
	suite.Require().NoError(err)

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

	suite.Eventually(func() bool {
		return !suite.clientWithoutInactvityCheck.Connected()
	}, 5*time.Second, 1*time.Second)

	err := suite.clientWithoutInactvityCheck.SetOption(
		client.WithReconnect(2*time.Second, &backoff.ZeroBackOff{}),
	)
	suite.Require().NoError(err)
	var insertOp []ovsdb.Operation
	insertOp, err = suite.clientWithoutInactvityCheck.Create(&ipfix)
	suite.Require().NoError(err)

	errCh := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	// delay reconnecting for 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		err := suite.clientWithoutInactvityCheck.Connect(context.Background())
		errCh <- err
		wg.Done()
	}()

	// execute the transaction, should not fail and execute after reconnection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reply, err := suite.clientWithoutInactvityCheck.Transact(ctx, insertOp...)
	suite.Require().NoError(err)

	_, err = ovsdb.CheckOperationResults(reply, insertOp)
	suite.Require().NoError(err)

	wg.Wait()
	err = <-errCh
	suite.Require().NoError(err)

}

func (suite *OVSIntegrationSuite) TestUnsetOptional() {
	// Create the default bridge which has an optional FailMode set
	uuid, err := suite.createBridge("br-with-optional-unset")
	suite.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	br := bridgeType{
		UUID: uuid,
	}

	// verify the bridge has FailMode set
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	suite.Require().NoError(err)
	suite.NotNil(br.FailMode)

	// modify bridge to unset BridgeFailMode
	br.FailMode = nil
	ops, err := suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.FailMode)
	suite.Require().NoError(err)
	r, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	suite.Require().NoError(err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	suite.Require().NoError(err)

	// verify the bridge has FailMode unset
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	suite.Require().NoError(err)
	suite.Nil(br.FailMode)
}

func (suite *OVSIntegrationSuite) TestUpdateOptional() {
	// Create the default bridge which has an optional FailMode set
	uuid, err := suite.createBridge("br-with-optional-update")
	suite.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	br := bridgeType{
		UUID: uuid,
	}

	// verify the bridge has FailMode set
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	suite.Require().NoError(err)
	suite.Equal(&BridgeFailModeSecure, br.FailMode)

	// modify bridge to update BridgeFailMode
	br.FailMode = &BridgeFailModeStandalone
	ops, err := suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.FailMode)
	suite.Require().NoError(err)
	r, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	suite.Require().NoError(err)
	_, err = ovsdb.CheckOperationResults(r, ops)
	suite.Require().NoError(err)

	// verify the bridge has FailMode updated
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	suite.Require().NoError(err)
	suite.Equal(&BridgeFailModeStandalone, br.FailMode)
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
				"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: iface1UUID}}},
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
				"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: iface10UUID}}},
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
		ExternalIDs: map[string]string{"key1": "value1"},
	}
	op, err := suite.clientWithoutInactvityCheck.Create(&br)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	ovs := ovsType{}
	op, err = suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).Mutate(&ovs, model.Mutation{
		Field:   &ovs.Bridges,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{bridgeUUID},
	})
	suite.Require().NoError(err)
	ops = append(ops, op...)

	results, err := suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	suite.Require().NoError(err)

	_, err = ovsdb.CheckOperationResults(results, ops)
	suite.Require().NoError(err)

	// find out the real UUIDs
	port1UUID = results[port1InsertOp].UUID.GoUUID
	port10UUID = results[port10InsertOp].UUID.GoUUID
	bridgeUUID = results[bridgeInsertOp].UUID.GoUUID

	ops = []ovsdb.Operation{}

	// Do several ops with the bridge in the same transaction
	br.Ports = []string{port10UUID}
	br.ExternalIDs = map[string]string{"key1": "value1", "key10": "value10"}
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.Ports, &br.ExternalIDs)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	op, err = suite.clientWithoutInactvityCheck.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIDs,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   map[string]string{"keyA": "valueA"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{port1UUID},
		},
	)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	op, err = suite.clientWithoutInactvityCheck.Where(&br).Mutate(&br,
		model.Mutation{
			Field:   &br.ExternalIDs,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   map[string]string{"key10": "value10"},
		},
		model.Mutation{
			Field:   &br.Ports,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{port10UUID},
		},
	)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	datapathID = "datapathID_updated"
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.DatapathID)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	br.DatapathID = nil
	op, err = suite.clientWithoutInactvityCheck.Where(&br).Update(&br, &br.DatapathID)
	suite.Require().NoError(err)
	ops = append(ops, op...)

	results, err = suite.clientWithoutInactvityCheck.Transact(ctx, ops...)
	suite.Require().NoError(err)

	errors, err := ovsdb.CheckOperationResults(results, ops)
	suite.Require().NoError(err)
	suite.Nil(errors)
	suite.Len(results, len(ops))

	br = bridgeType{
		UUID: bridgeUUID,
	}
	err = suite.clientWithoutInactvityCheck.Get(ctx, &br)
	suite.Require().NoError(err)

	suite.Equal([]string{port1UUID}, br.Ports)
	suite.Equal(map[string]string{"key1": "value1", "keyA": "valueA"}, br.ExternalIDs)
	suite.Nil(br.DatapathID)
}

func (suite *OVSIntegrationSuite) TestReferentialIntegrity() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// fetch the OVS UUID
	var ovs []*ovsType
	err := suite.clientWithoutInactvityCheck.WhereCache(func(*ovsType) bool { return true }).List(ctx, &ovs)
	suite.Require().NoError(err)
	suite.Len(ovs, 1)

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
	suite.Require().NoError(err)

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
						"ports":   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: interfaceUUID}}},
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
						"select_src_port": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}}},
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
						"ports":   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}, ovsdb.UUID{GoUUID: port2UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: interfaceUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port2UUID,
					Row: ovsdb.Row{
						"name":       port2UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: interfaceUUID}}},
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
						"select_src_port": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}, ovsdb.UUID{GoUUID: port2UUID}}},
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
						"ports":   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}}},
						"mirrors": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: mirrorUUID}}},
					},
				},
				{
					Op:    ovsdb.OperationInsert,
					Table: "Port",
					UUID:  port1UUID,
					Row: ovsdb.Row{
						"name":       port1UUID,
						"interfaces": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: interfaceUUID}}},
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
						"select_src_port": ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: port1UUID}}},
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
		suite.Run(tt.name, func() {
			c := suite.clientWithoutInactvityCheck

			// add the bridge reference to the initial ops
			ops := append(tt.initialOps, ovsdb.Operation{
				Op:    ovsdb.OperationMutate,
				Table: "Open_vSwitch",
				Mutations: []ovsdb.Mutation{
					{
						Mutator: ovsdb.MutateOperationInsert,
						Column:  "bridges",
						Value:   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: bridgeUUID}}},
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
			suite.Require().NoError(err)
			suite.Len(results, len(ops))

			errors, err := ovsdb.CheckOperationResults(results, ops)
			suite.Nil(errors)
			suite.Require().NoError(err)

			ops, err = tt.testOps(c)
			suite.Require().NoError(err)

			results, err = c.Transact(ctx, ops...)
			suite.Require().NoError(err)

			errors, err = ovsdb.CheckOperationResults(results, ops)
			suite.Nil(errors)
			if tt.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}

			for _, m := range tt.expectModels {
				actual := model.Clone(m)
				err := c.Get(ctx, actual)
				suite.Require().NoError(err, "when expecting model %v", m)
				suite.Equal(m, actual)
			}

			for _, m := range tt.dontExpectModels {
				err := c.Get(ctx, m)
				suite.Require().ErrorIs(err, client.ErrNotFound, "when expecting model %v", m)
			}

			ops = []ovsdb.Operation{}
			for _, m := range tt.expectModels {
				op, err := c.Where(m).Delete()
				suite.Require().NoError(err)
				suite.Len(op, 1)
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
						Value:   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: bridgeUUID}}},
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
			suite.Require().NoError(err)
			suite.Len(results, len(ops))

			errors, err = ovsdb.CheckOperationResults(results, ops)
			suite.Nil(errors)
			suite.Require().NoError(err)
		})
	}
}
