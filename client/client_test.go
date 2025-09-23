package client

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cenkalti/rpc2"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/database/inmemory"
	"github.com/ovn-kubernetes/libovsdb/internal/json"
	"github.com/ovn-kubernetes/libovsdb/mapper"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
	"github.com/ovn-kubernetes/libovsdb/server"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	aUUID0 = "2f77b348-9768-4866-b761-89d5177ecda0"
	aUUID1 = "2f77b348-9768-4866-b761-89d5177ecda1"
	aUUID2 = "2f77b348-9768-4866-b761-89d5177ecda2"
	aUUID3 = "2f77b348-9768-4866-b761-89d5177ecda3"
)

type (
	BridgeFailMode  = string
	BridgeProtocols = string
)

const (
	BridgeFailModeStandalone  BridgeFailMode  = "standalone"
	BridgeFailModeSecure      BridgeFailMode  = "secure"
	BridgeProtocolsOpenflow10 BridgeProtocols = "OpenFlow10"
	BridgeProtocolsOpenflow11 BridgeProtocols = "OpenFlow11"
	BridgeProtocolsOpenflow12 BridgeProtocols = "OpenFlow12"
	BridgeProtocolsOpenflow13 BridgeProtocols = "OpenFlow13"
	BridgeProtocolsOpenflow14 BridgeProtocols = "OpenFlow14"
	BridgeProtocolsOpenflow15 BridgeProtocols = "OpenFlow15"
)

// Bridge defines an object in Bridge table
type Bridge struct {
	UUID                string            `ovsdb:"_uuid"`
	AutoAttach          *string           `ovsdb:"auto_attach"`
	Controller          []string          `ovsdb:"controller"`
	DatapathID          *string           `ovsdb:"datapath_id"`
	DatapathType        string            `ovsdb:"datapath_type"`
	DatapathVersion     string            `ovsdb:"datapath_version"`
	ExternalIDs         map[string]string `ovsdb:"external_ids"`
	FailMode            *BridgeFailMode   `ovsdb:"fail_mode"`
	FloodVLANs          []int             `ovsdb:"flood_vlans"`
	FlowTables          map[int]string    `ovsdb:"flow_tables"`
	IPFIX               *string           `ovsdb:"ipfix"`
	McastSnoopingEnable bool              `ovsdb:"mcast_snooping_enable"`
	Mirrors             []string          `ovsdb:"mirrors"`
	Name                string            `ovsdb:"name"`
	Netflow             *string           `ovsdb:"netflow"`
	OtherConfig         map[string]string `ovsdb:"other_config"`
	Ports               []string          `ovsdb:"ports"`
	Protocols           []BridgeProtocols `ovsdb:"protocols"`
	RSTPEnable          bool              `ovsdb:"rstp_enable"`
	RSTPStatus          map[string]string `ovsdb:"rstp_status"`
	Sflow               *string           `ovsdb:"sflow"`
	Status              map[string]string `ovsdb:"status"`
	STPEnable           bool              `ovsdb:"stp_enable"`
}

// OpenvSwitch defines an object in Open_vSwitch table
type OpenvSwitch struct {
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

var defDB, _ = model.NewClientDBModel("Open_vSwitch",
	map[string]model.Model{
		"Open_vSwitch": &OpenvSwitch{},
		"Bridge":       &Bridge{},
	},
)

var schema = `{
	"name": "Open_vSwitch",
	"version": "8.2.0",
	"tables": {
	  "Bridge": {
		"columns": {
		  "auto_attach": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "AutoAttach"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "controller": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Controller"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "datapath_id": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			},
			"ephemeral": true
		  },
		  "datapath_type": {
			"type": "string"
		  },
		  "datapath_version": {
			"type": "string"
		  },
		  "external_ids": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "fail_mode": {
			"type": {
			  "key": {
				"type": "string",
				"enum": [
				  "set",
				  [
					"standalone",
					"secure"
				  ]
				]
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "flood_vlans": {
			"type": {
			  "key": {
				"type": "integer",
				"minInteger": 0,
				"maxInteger": 4095
			  },
			  "min": 0,
			  "max": 4096
			}
		  },
		  "flow_tables": {
			"type": {
			  "key": {
				"type": "integer",
				"minInteger": 0,
				"maxInteger": 254
			  },
			  "value": {
				"type": "uuid",
				"refTable": "Flow_Table"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ipfix": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "IPFIX"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "mcast_snooping_enable": {
			"type": "boolean"
		  },
		  "mirrors": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Mirror"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "name": {
			"type": "string",
			"mutable": false
		  },
		  "netflow": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "NetFlow"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "other_config": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ports": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Port"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "protocols": {
			"type": {
			  "key": {
				"type": "string",
				"enum": [
				  "set",
				  [
					"OpenFlow10",
					"OpenFlow11",
					"OpenFlow12",
					"OpenFlow13",
					"OpenFlow14",
					"OpenFlow15"
				  ]
				]
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "rstp_enable": {
			"type": "boolean"
		  },
		  "rstp_status": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "sflow": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "sFlow"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "status": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "stp_enable": {
			"type": "boolean"
		  }
		},
		"indexes": [
		  [
			"name"
		  ]
		]
	  },
	  "Open_vSwitch": {
		"columns": {
		  "bridges": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Bridge"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "cur_cfg": {
			"type": "integer"
		  },
		  "datapath_types": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "datapaths": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "uuid",
				"refTable": "Datapath"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "db_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "dpdk_initialized": {
			"type": "boolean"
		  },
		  "dpdk_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "external_ids": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "iface_types": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "manager_options": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "Manager"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "next_cfg": {
			"type": "integer"
		  },
		  "other_config": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			}
		  },
		  "ovs_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "ssl": {
			"type": {
			  "key": {
				"type": "uuid",
				"refTable": "SSL"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "statistics": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "value": {
				"type": "string"
			  },
			  "min": 0,
			  "max": "unlimited"
			},
			"ephemeral": true
		  },
		  "system_type": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  },
		  "system_version": {
			"type": {
			  "key": {
				"type": "string"
			  },
			  "min": 0,
			  "max": 1
			}
		  }
		}
	  }
	}
  }`

func testOvsSet(t *testing.T, set any) ovsdb.OvsSet {
	oSet, err := ovsdb.NewOvsSet(set)
	require.NoError(t, err)
	return oSet
}

func testOvsMap(t *testing.T, set any) ovsdb.OvsMap {
	oMap, err := ovsdb.NewOvsMap(set)
	require.NoError(t, err)
	return oMap
}

func updateBenchmark(ovs *ovsdbClient, updates []byte, b *testing.B) {
	for n := 0; n < b.N; n++ {
		params := []json.RawMessage{[]byte(`{"databaseName":"Open_vSwitch","id":"v1"}`), updates}
		var reply []any
		err := ovs.update(params, &reply)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func newBridgeRow(name string) string {
	return `{
		"connection_mode": [ "set", [] ],
		"controller": [ "set", [] ],
		"datapath_id": "blablabla",
		"datapath_type": "",
		"datapath_version": "",
		"external_ids": [ "map", [["foo","bar"]]],
		"fail_mode": [ "set", [] ],
		"flood_vlans": [ "set", [] ],
		"flow_tables": [ "map", [] ],
		"ipfix": [ "set", [] ],
		"mcast_snooping_enable": false,
		"mirrors": [ "set", [] ],
		"name": "` + name + `",
		"netflow": [ "set", [] ],
		"other_config": [ "map", [["bar","quux"]]],
		"ports": [ "set", [] ],
		"protocols": [ "set", [] ],
		"rstp_enable": false,
		"rstp_status": [ "map", [] ],
		"sflow": [ "set", [] ],
		"status": [ "map", [] ],
		"stp_enable": false
	}`
}

func newOvsRow(bridges ...string) string {
	bridgeUUIDs := []string{}
	for _, b := range bridges {
		bridgeUUIDs = append(bridgeUUIDs, `[ "uuid", "`+b+`" ]`)
	}
	return `{
		"bridges": [ "set", [` + strings.Join(bridgeUUIDs, `,`) + `]],
		"cur_cfg": 0,
		"datapath_types": [ "set", [] ],
		"datapaths": [ "map", [] ],
		"db_version":       "8.2.0",
		"dpdk_initialized": false,
		"dpdk_version":     [ "set", [] ],
		"external_ids":     [ "map", [["system-id","829f8534-94a8-468e-9176-132738cf260a"]]],
		"iface_types":      [ "set", [] ],
		"manager_options":  ["uuid", "6e4cd5fc-f51a-462a-b3d6-a696af6d7a84"],
		"next_cfg":         0,
		"other_config":     [ "map", [] ],
		"ovs_version":      "2.15.90",
		"ssl":              [ "set", [] ],
		"statistics":       [ "map", [] ],
		"system_type":      "docker-ovs",
		"system_version":   "0.1"
	}`
}

func BenchmarkUpdate1(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(b, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate2(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(b, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate3(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(b, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate5(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(b, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz", "quux", "foofoo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `},
			"quux": {"new": ` + newBridgeRow("quux") + `},
			"foofoo": {"new": ` + newBridgeRow("foofoo") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func BenchmarkUpdate8(b *testing.B) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(b, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(b, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(b, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(b, err)
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo", "bar", "baz", "quux", "foofoo", "foobar", "foobaz", "fooquux") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `},
			"bar": {"new": ` + newBridgeRow("bar") + `},
			"baz": {"new": ` + newBridgeRow("baz") + `},
			"quux": {"new": ` + newBridgeRow("quux") + `},
			"foofoo": {"new": ` + newBridgeRow("foofoo") + `},
			"foobar": {"new": ` + newBridgeRow("foobar") + `},
			"foobaz": {"new": ` + newBridgeRow("foobaz") + `},
			"fooquux": {"new": ` + newBridgeRow("fooquux") + `}
		}
	}`)
	updateBenchmark(ovs, update, b)
}

func TestEcho(t *testing.T) {
	req := []any{"hi"}
	var reply []any
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	err = ovs.echo(req, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, reply) {
		t.Error("Expected: ", req, " Got: ", reply)
	}
}

func TestUpdate(t *testing.T) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(t, errs)
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)
	var reply []any
	update := []byte(`{
		"Open_vSwitch": {
			"ovs": {"new": ` + newOvsRow("foo") + `}
		},
		"Bridge": {
			"foo": {"new": ` + newBridgeRow("foo") + `}
		}
	}`)
	params := []json.RawMessage{[]byte(`{"databaseName":"Open_vSwitch","id":"v1"}`), update}
	err = ovs.update(params, &reply)
	if err != nil {
		t.Error(err)
	}
}

func TestOperationWhenNeverConnected(t *testing.T) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"echo",
			func() error {
				return ovs.Echo(context.TODO())
			},
		},
		{
			"transact",
			func() error {
				comment := "this is only a test"
				_, err := ovs.Transact(context.TODO(), ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &comment})
				return err
			},
		},
		{
			"monitor/monitor all",
			func() error {
				_, err := ovs.MonitorAll(context.TODO())
				return err
			},
		},
		{
			"monitor cancel",
			func() error {
				return ovs.MonitorCancel(context.TODO(), newMonitorCookie(s.Name))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			assert.EqualError(t, err, ErrNotConnected.Error())
		})
	}
}

func TestTransactionLogger(t *testing.T) {
	stdr.SetVerbosity(5)

	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)
	_, sock := newOVSDBServer(t, defDB, defSchema)
	// Create client for this server's Server database
	endpoint := fmt.Sprintf("unix:%s", sock)

	var defaultBuf bytes.Buffer
	defaultL := stdr.New(log.New(&defaultBuf, "", log.LstdFlags)).WithName("default")

	// Create client to test transaction logger
	ovs, err := newOVSDBClient(defDB,
		WithEndpoint(endpoint),
		WithLogger(&defaultL))
	require.NoError(t, err)

	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(ovs.Close)

	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)

	dbModel, err := test.GetModel()
	require.NoError(t, err)
	m := mapper.NewMapper(dbModel.Schema)

	bridge1 := test.BridgeType{
		Name: "foo",
		ExternalIDs: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo1, err := dbModel.NewModelInfo(&bridge1)
	require.NoError(t, err)
	bridgeRow1, err := m.NewRow(bridgeInfo1)
	require.NoError(t, err)
	bridgeUUID1 := uuid.NewString()
	operation1 := ovsdb.Operation{
		Op:    ovsdb.OperationInsert,
		Table: "Bridge",
		UUID:  bridgeUUID1,
		Row:   bridgeRow1,
	}
	_, _ = ovs.Transact(context.TODO(), operation1)
	assert.Contains(t, defaultBuf.String(), "default")

	bridge2 := test.BridgeType{
		Name: "bar",
		ExternalIDs: map[string]string{
			"foo":   "bar",
			"baz":   "quux",
			"waldo": "fred",
		},
	}
	bridgeInfo2, err := dbModel.NewModelInfo(&bridge2)
	require.NoError(t, err)
	bridgeRow2, err := m.NewRow(bridgeInfo2)
	require.NoError(t, err)
	bridgeUUID2 := uuid.NewString()
	operation2 := ovsdb.Operation{
		Op:    ovsdb.OperationInsert,
		Table: "Bridge",
		UUID:  bridgeUUID2,
		Row:   bridgeRow2,
	}
	var customBuf bytes.Buffer
	customL := stdr.New(log.New(&customBuf, "", log.LstdFlags)).WithName("custom")
	ctx := logr.NewContext(context.TODO(), customL)
	_, _ = ovs.Transact(ctx, operation2)
	assert.Contains(t, customBuf.String(), "custom")
}

func TestOperationWhenNotConnected(t *testing.T) {
	ovs, err := newOVSDBClient(defDB)
	require.NoError(t, err)
	var s ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)
	var errs []error
	fullModel, errs := model.NewDatabaseModel(s, ovs.primaryDB().model.Client())
	require.Emptyf(t, errs, "expected no error but some occurred: %+v", errs)
	ovs.primaryDB().model = fullModel

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"echo",
			func() error {
				return ovs.Echo(context.TODO())
			},
		},
		{
			"transact",
			func() error {
				comment := "this is only a test"
				_, err := ovs.Transact(context.TODO(), ovsdb.Operation{Op: ovsdb.OperationComment, Comment: &comment})
				return err
			},
		},
		{
			"monitor/monitor all",
			func() error {
				_, err := ovs.MonitorAll(context.TODO())
				return err
			},
		},
		{
			"monitor cancel",
			func() error {
				return ovs.MonitorCancel(context.TODO(), newMonitorCookie(s.Name))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			assert.EqualError(t, err, ErrNotConnected.Error())
		})
	}
}

func TestSetOption(t *testing.T) {
	o, err := newOVSDBClient(defDB)
	require.NoError(t, err)

	o.options, err = newOptions()
	require.NoError(t, err)

	err = o.SetOption(WithEndpoint("tcp::6640"))
	require.NoError(t, err)

	o.rpcClient = &rpc2.Client{}

	err = o.SetOption(WithEndpoint("tcp::6641"))
	assert.EqualError(t, err, "cannot set option when client is connected")
}

func newOVSDBServer(t *testing.T, dbModel model.ClientDBModel, schema ovsdb.DatabaseSchema) (*server.OvsdbServer, string) {
	serverDBModel, err := serverdb.FullDatabaseModel()
	require.NoError(t, err)
	serverSchema := serverdb.Schema()
	logger := logr.Discard()

	db := inmemory.NewDatabase(map[string]model.ClientDBModel{
		schema.Name:       dbModel,
		serverSchema.Name: serverDBModel,
	}, &logger)

	dbMod, errs := model.NewDatabaseModel(schema, dbModel)
	require.Empty(t, errs)

	servMod, errs := model.NewDatabaseModel(serverSchema, serverDBModel)
	require.Empty(t, errs)

	server, err := server.NewOvsdbServer(db, &logger, dbMod, servMod)
	require.NoError(t, err)

	tmpfile := fmt.Sprintf("/tmp/ovsdb-%d.sock", rand.Intn(10000))
	t.Cleanup(func() {
		os.Remove(tmpfile)
	})
	go func() {
		if err := server.Serve("unix", tmpfile); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(server.Close)
	require.Eventually(t, func() bool {
		return server.Ready()
	}, 1*time.Second, 10*time.Millisecond)

	return server, tmpfile
}

func newClientServerPair(t *testing.T, connectCounter, disConnectCounter *int32, isLeader bool) (Client, *serverdb.Database, string) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)

	serverDBModel, err := serverdb.FullDatabaseModel()
	require.NoError(t, err)

	// Create server
	s, sock := newOVSDBServer(t, defDB, defSchema)
	s.OnConnect(func(_ *rpc2.Client) {
		atomic.AddInt32(connectCounter, 1)
	})
	s.OnDisConnect(func(_ *rpc2.Client) {
		atomic.AddInt32(disConnectCounter, 1)
	})

	// Create client for this server's Server database
	endpoint := fmt.Sprintf("unix:%s", sock)
	cli, err := newOVSDBClient(serverDBModel, WithEndpoint(endpoint))
	require.NoError(t, err)
	err = cli.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	// Populate the _Server database table
	sid := fmt.Sprintf("%04x", rand.Uint32())
	row := &serverdb.Database{
		UUID:      uuid.NewString(),
		Name:      defDB.Name(),
		Connected: true,
		Leader:    isLeader,
		Model:     serverdb.DatabaseModelClustered,
		Sid:       &sid,
	}
	ops, err := cli.Create(row)
	require.NoError(t, err)
	reply, err := cli.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	assert.NoErrorf(t, err, "%+v", opErr)

	row.UUID = reply[0].UUID.GoUUID
	return cli, row, endpoint
}

func setLeader(t *testing.T, cli Client, row *serverdb.Database, isLeader bool) {
	row.Leader = isLeader
	ops, err := cli.Where(row).Update(row, &row.Leader)
	require.NoError(t, err)
	reply, err := cli.Transact(context.Background(), ops...)
	require.NoError(t, err)
	opErr, err := ovsdb.CheckOperationResults(reply, ops)
	assert.NoErrorf(t, err, "%+v", opErr)
}

func TestClientInactiveCheck(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &defSchema)
	require.NoError(t, err)

	serverDBModel, err := serverdb.FullDatabaseModel()
	require.NoError(t, err)
	// Create server
	server, sock := newOVSDBServer(t, defDB, defSchema)

	// Create client to test inactivity check.
	endpoint := fmt.Sprintf("unix:%s", sock)
	ovs, err := newOVSDBClient(serverDBModel,
		WithInactivityCheck(2*time.Second, 1*time.Second, &backoff.ZeroBackOff{}),
		WithEndpoint(endpoint))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(ovs.Close)

	// Make server to do echo off and then on for two times.
	// Ensure this is detected by client's inactivity probe
	// each time and then reconnects to the server when it
	// is started responding to echo requests.

	// 1st test for client with making server not to respond for echo requests.
	notified := make(chan struct{})
	ready := make(chan struct{})
	disconnectNotify := ovs.rpcClient.DisconnectNotify()
	go func() {
		ready <- struct{}{}
		<-disconnectNotify
		notified <- struct{}{}
	}()
	<-ready
	server.DoEcho(false)
	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		assert.Fail(t, "client doesn't detect the echo failure")
	}

	// 2nd test for client with making server to respond for echo requests.
	server.DoEcho(true)
loop:
	for timeout := time.After(5 * time.Second); ; {
		select {
		case <-timeout:
			assert.Fail(t, "reconnect is not successful")
		default:
			if ovs.Connected() {
				break loop
			}
		}
	}

	// 3rd test for client with making server not to respond for echo requests.
	notified = make(chan struct{})
	ready = make(chan struct{})
	disconnectNotify = ovs.rpcClient.DisconnectNotify()
	go func() {
		ready <- struct{}{}
		<-disconnectNotify
		notified <- struct{}{}
	}()
	<-ready
	server.DoEcho(false)
	select {
	case <-notified:
		// got notification
	case <-time.After(5 * time.Second):
		assert.Fail(t, "client doesn't detect the echo failure")
	}

	// 4th test for client with making server to respond for echo requests.
	server.DoEcho(true)
loop1:
	for timeout := time.After(5 * time.Second); ; {
		select {
		case <-timeout:
			assert.Fail(t, "reconnect is not successful")
		default:
			if ovs.Connected() {
				break loop1
			}
		}
	}
}

func TestClientReconnectLeaderOnly(t *testing.T) {
	var connected1, connected2, disConnected1, disConnected2 int32
	cli1, row1, endpoint1 := newClientServerPair(t, &connected1, &disConnected1, true)
	cli2, row2, endpoint2 := newClientServerPair(t, &connected2, &disConnected2, false)

	// Create client to test reconnection for
	ovs, err := newOVSDBClient(defDB,
		WithLeaderOnly(true),
		WithReconnect(5*time.Second, &backoff.ZeroBackOff{}),
		WithEndpoint(endpoint1),
		WithEndpoint(endpoint2))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(ovs.Close)

	// Server1 should have 2 connections: cli1 and ovs
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected1) == 2
	}, 2*time.Second, 10*time.Millisecond)

	// Server2 should have 1 connection: cli2
	require.Never(t, func() bool {
		return atomic.LoadInt32(&connected2) > 1
	}, 2*time.Second, 10*time.Millisecond)

	// First leadership change
	setLeader(t, cli2, row2, true)
	setLeader(t, cli1, row1, false)

	// Server2 should have 2 connections: cli2 and ovs
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected2) == 2
	}, 2*time.Second, 10*time.Millisecond)

	// Server1 should still only have 2 total connections; eg the
	// client under test should not have reconnected
	require.Never(t, func() bool {
		return atomic.LoadInt32(&connected1) > 2
	}, 2*time.Second, 10*time.Millisecond)

	// Second leadership change
	setLeader(t, cli1, row1, true)
	setLeader(t, cli2, row2, false)

	// Server1 should now have 3 total connections: cli1, original ovs,
	// and second ovs
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected1) == 3
	}, 2*time.Second, 10*time.Millisecond)

	// Server2 should still only have 2 total connections; eg the
	// client under test should not have reconnected
	require.Never(t, func() bool {
		return atomic.LoadInt32(&connected2) > 2
	}, 2*time.Second, 10*time.Millisecond)
}

func TestNewMonitorRequest(t *testing.T) {
	var testSchema = []byte(`{
  "cksum": "223619766 22548",
  "name": "TestSchema",
  "tables": {
    "TestTable": {
      "indexes": [["name"],["composed_1","composed_2"]],
      "columns": {
        "name": {
          "type": "string"
        },
        "composed_1": {
          "type": {
            "key": "string"
          }
        },
        "composed_2": {
          "type": {
            "key": "string"
          }
        },
        "int1": {
          "type": {
            "key": "integer"
          }
        },
        "int2": {
          "type": {
            "key": "integer"
          }
        },
        "config": {
          "type": {
            "key": "string",
            "max": "unlimited",
            "min": 0,
            "value": "string"
          }
	}
      }
    }
  }
}`)
	type testType struct {
		ID     string            `ovsdb:"_uuid"`
		MyName string            `ovsdb:"name"`
		Config map[string]string `ovsdb:"config"`
		Comp1  string            `ovsdb:"composed_1"`
		Comp2  string            `ovsdb:"composed_2"`
		Int1   int               `ovsdb:"int1"`
		Int2   int               `ovsdb:"int2"`
	}
	var schema ovsdb.DatabaseSchema
	err := json.Unmarshal(testSchema, &schema)
	require.NoError(t, err)
	testTable := &testType{}
	info, err := mapper.NewInfo("TestTable", schema.Table("TestTable"), testTable)
	require.NoError(t, err)
	mr, err := newMonitorRequest(info, nil, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, mr.Columns, []string{"name", "config", "composed_1", "composed_2", "int1", "int2"})
	mr2, err := newMonitorRequest(info, []string{"int1", "name"}, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, mr2.Columns, []string{"int1", "name"})
}

func TestUpdateEndpoints(t *testing.T) {

	var connected1, connected2, connected3, disConnected1, disConnected2, disConnected3 int32
	_, _, endpoint1 := newClientServerPair(t, &connected1, &disConnected1, true)
	_, _, endpoint2 := newClientServerPair(t, &connected2, &disConnected2, false)
	_, _, endpoint3 := newClientServerPair(t, &connected3, &disConnected3, true)

	// Create client to test reconnection for
	ovs, err := newOVSDBClient(defDB,
		WithLeaderOnly(true),
		WithReconnect(1*time.Second, &backoff.ZeroBackOff{}),
		WithEndpoint(endpoint1))
	require.NoError(t, err)
	err = ovs.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(ovs.Close)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected1) == 2
	}, 2*time.Second, 10*time.Millisecond)

	require.Equal(t, ovs.CurrentEndpoint(), endpoint1)
	require.NotEmpty(t, ovs.endpoints[0].serverID)

	// update with same endpoints should not have a disconnect
	ovs.UpdateEndpoints([]string{endpoint1})
	require.Eventually(t, func() bool {
		// connect should not increase
		return atomic.LoadInt32(&connected1) == 2
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		// should not disconnect
		return atomic.LoadInt32(&disConnected1) == 0
	}, 2*time.Second, 10*time.Millisecond)

	ovs.UpdateEndpoints([]string{endpoint2, endpoint1})
	require.Eventually(t, func() bool {
		return ovs.CurrentEndpoint() == endpoint1
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected2) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		// server1 should still be the active
		return atomic.LoadInt32(&disConnected1) == 0
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, ovs.endpoints[0].address, endpoint1)
	require.Equal(t, ovs.endpoints[1].address, endpoint2)
	require.NotEmpty(t, ovs.endpoints[0].serverID)

	// server3 is the new leader
	ovs.UpdateEndpoints([]string{endpoint2, endpoint3})
	require.Eventually(t, func() bool {
		return ovs.CurrentEndpoint() == endpoint3
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&disConnected2) == 1
	}, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&connected3) == 2
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, ovs.endpoints[0].address, endpoint3)
	require.Equal(t, ovs.endpoints[1].address, endpoint2)
	require.NotEmpty(t, ovs.endpoints[0].serverID)
}

// TestConditionalAPISelect tests the Select method on the ConditionalAPI
func TestConditionalAPISelect(t *testing.T) {
	var connectCounter, disconnectCounter int32
	ovsClient, _, serverAddr := newClientServerPair(t, &connectCounter, &disconnectCounter, true)
	defer ovsClient.Close()

	ovsClientForDefDB, err := newOVSDBClient(defDB, WithEndpoint(serverAddr))
	require.NoError(t, err, "Failed to create OVSDB client for Open_vSwitch")
	err = ovsClientForDefDB.Connect(context.Background())
	require.NoError(t, err, "Failed to connect OVSDB client for Open_vSwitch")
	defer ovsClientForDefDB.Close()

	tableCache := ovsClientForDefDB.Cache()
	// Create some test bridges and add them to cache
	cacheBridge1 := &Bridge{
		UUID:         "cache-bridge-uuid-1",
		Name:         "cache-br-1",
		DatapathType: "system",
	}
	cacheBridge2 := &Bridge{
		UUID:         "cache-bridge-uuid-2",
		Name:         "cache-br-2",
		DatapathType: "netdev",
	}
	cacheBridge3 := &Bridge{
		UUID:         "cache-bridge-uuid-3",
		Name:         "other-bridge", // This one won't match the WhereCache filter
		DatapathType: "system",
	}
	bridgeTable := tableCache.Table("Bridge")
	require.NoError(t, bridgeTable.Create("cache-bridge-uuid-1", cacheBridge1, false))
	require.NoError(t, bridgeTable.Create("cache-bridge-uuid-2", cacheBridge2, false))
	require.NoError(t, bridgeTable.Create("cache-bridge-uuid-3", cacheBridge3, false))

	bridgeModel := &Bridge{Name: "br-selcond"}
	bridgeModelCtx := &Bridge{} // Context model for WhereAll/WhereAny

	// Expected Columns (including _uuid)
	expectedCols := []string{"_uuid", "auto_attach", "controller", "datapath_id", "datapath_type", "datapath_version", "external_ids", "fail_mode", "flood_vlans", "flow_tables", "ipfix", "mcast_snooping_enable", "mirrors", "name", "netflow", "other_config", "ports", "protocols", "rstp_enable", "rstp_status", "sflow", "status", "stp_enable"}
	sort.Strings(expectedCols) // Sort for consistent comparison

	tests := []struct {
		name           string
		conditionalAPI func(API) ConditionalAPI
		selectModel    model.Model // Model to use for Select call
		selectFields   []any
		expectError    bool
		errorContains  string
		expectedWhere  []ovsdb.Condition
		expectedTable  string
		expectedOps    []ovsdb.Operation
	}{
		{
			name: "Select after Where",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.Where(bridgeModel) // Condition based on index
			},
			selectModel:   bridgeModel,
			selectFields:  nil, // Default: all columns
			expectError:   false,
			expectedWhere: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "br-selcond"}}, // Equality condition from index
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "br-selcond")},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select specific fields after Where",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.Where(bridgeModel)
			},
			selectModel:   bridgeModel,
			selectFields:  []any{&bridgeModel.Name, &bridgeModel.ExternalIDs, &bridgeModel.STPEnable},
			expectError:   false,
			expectedWhere: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "br-selcond"}},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "br-selcond")},
						Columns: []string{"_uuid", "external_ids", "name", "stp_enable"}, // Always includes _uuid, sorted
					},
				}
				return expected
			}(),
		},
		{
			name: "Select after WhereAll",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAll(bridgeModelCtx, model.Condition{
					Field:    &bridgeModelCtx.Name,
					Function: ovsdb.ConditionEqual,
					Value:    "some-other-bridge",
				})
			},
			selectModel:   bridgeModelCtx,
			selectFields:  nil, // Default: all columns
			expectError:   false,
			expectedWhere: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "some-other-bridge"}},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "some-other-bridge")},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select single field after WhereAll",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAll(bridgeModelCtx, model.Condition{
					Field:    &bridgeModelCtx.Name,
					Function: ovsdb.ConditionEqual,
					Value:    "some-other-bridge",
				})
			},
			selectModel:   bridgeModelCtx,
			selectFields:  []any{&bridgeModelCtx.Name},
			expectError:   false,
			expectedWhere: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "some-other-bridge"}},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "some-other-bridge")},
						Columns: []string{"_uuid", "name"}, // Always includes _uuid
					},
				}
				return expected
			}(),
		},
		{
			name: "Select after WhereAll with multiple conditions (string and bool)",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAll(bridgeModelCtx, // Context model for table/fields
					model.Condition{
						Field:    &bridgeModelCtx.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "br-multi1",
					},
					model.Condition{
						Field:    &bridgeModelCtx.STPEnable, // Example using bool field
						Function: ovsdb.ConditionEqual,
						Value:    true, // Use bool value directly
					},
				)
			},
			selectModel:  bridgeModelCtx,
			selectFields: nil, // Default: all columns
			expectError:  false,
			expectedWhere: []ovsdb.Condition{
				{Column: "name", Function: ovsdb.ConditionEqual, Value: "br-multi1"},
				{Column: "stp_enable", Function: ovsdb.ConditionEqual, Value: true},
			},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:    ovsdb.OperationSelect,
						Table: "Bridge",
						Where: []ovsdb.Condition{
							{Column: "name", Function: ovsdb.ConditionEqual, Value: "br-multi1"},
							{Column: "stp_enable", Function: ovsdb.ConditionEqual, Value: true},
						},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select with duplicate fields after WhereAll",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAll(bridgeModelCtx,
					model.Condition{
						Field:    &bridgeModelCtx.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "br-dup",
					},
				)
			},
			selectModel:   bridgeModelCtx,
			selectFields:  []any{&bridgeModelCtx.Name, &bridgeModelCtx.ExternalIDs, &bridgeModelCtx.Name, &bridgeModelCtx.STPEnable, &bridgeModelCtx.ExternalIDs}, // Duplicates should be removed
			expectError:   false,
			expectedWhere: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "br-dup"}},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "br-dup")},
						Columns: []string{"_uuid", "external_ids", "name", "stp_enable"}, // Sorted, no duplicates
					},
				}
				return expected
			}(),
		},
		{
			name: "Select after WhereAll with multiple conditions (string and map includes)",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAll(bridgeModelCtx, // Context model for table/fields
					model.Condition{
						Field:    &bridgeModelCtx.Name,
						Function: ovsdb.ConditionNotEqual,
						Value:    "br-exclude",
					},
					model.Condition{
						Field:    &bridgeModelCtx.ExternalIDs, // Example using map field
						Function: ovsdb.ConditionIncludes,
						Value:    map[string]string{"key": "value"},
					},
				)
			},
			selectModel:  bridgeModelCtx,
			selectFields: nil, // Default: all columns
			expectError:  false,
			expectedWhere: []ovsdb.Condition{
				{Column: "name", Function: ovsdb.ConditionNotEqual, Value: "br-exclude"},
				{Column: "external_ids", Function: ovsdb.ConditionIncludes, Value: testOvsMap(t, map[string]string{"key": "value"})},
			},
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:    ovsdb.OperationSelect,
						Table: "Bridge",
						Where: []ovsdb.Condition{
							{Column: "name", Function: ovsdb.ConditionNotEqual, Value: "br-exclude"},
							{Column: "external_ids", Function: ovsdb.ConditionIncludes, Value: testOvsMap(t, map[string]string{"key": "value"})},
						},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select with invalid field pointer",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.Where(bridgeModel)
			},
			selectModel:   bridgeModel,
			selectFields:  []any{&bridgeModel.Name, "invalid_field_pointer"}, // String instead of field pointer
			expectError:   true,
			errorContains: "failed to get column name for field pointer",
		},
		{
			name: "Select after WhereCache",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereCache(func(br *Bridge) bool {
					// Filter for bridges with names starting with "cache-br"
					return strings.HasPrefix(br.Name, "cache-br")
				})
			},
			selectModel:   &Bridge{}, // For WhereCache we can use any Bridge model since it operates on cache
			selectFields:  nil,       // Default: all columns
			expectError:   false,
			expectedWhere: []ovsdb.Condition{}, // WhereCache generates conditions based on cache matches
			expectedTable: "Bridge",
			expectedOps: func() []ovsdb.Operation {
				// WhereCache will generate operations based on cached Bridge models
				// We expect operations with UUID conditions for each matching bridge in cache
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: "cache-bridge-uuid-1"})},
						Columns: nil, // nil means all columns per RFC 7047
					},
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: "cache-bridge-uuid-2"})},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select after WhereAny with multiple OR conditions",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAny(bridgeModelCtx,
					model.Condition{Field: &bridgeModelCtx.Name, Function: ovsdb.ConditionEqual, Value: "a"},
					model.Condition{Field: &bridgeModelCtx.OtherConfig, Function: ovsdb.ConditionIncludes, Value: map[string]string{"foo": "bar"}},
				)
			},
			selectModel:  bridgeModelCtx,
			selectFields: nil, // Default: all columns
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "a")},
						Columns: nil, // nil means all columns per RFC 7047
					},
					{
						Op:    ovsdb.OperationSelect,
						Table: "Bridge",
						Where: []ovsdb.Condition{ovsdb.NewCondition("other_config", ovsdb.ConditionIncludes,
							ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"foo": "bar"}})},
						Columns: nil, // nil means all columns per RFC 7047
					},
				}
				return expected
			}(),
		},
		{
			name: "Select specific fields after WhereAny",
			conditionalAPI: func(api API) ConditionalAPI {
				return api.WhereAny(bridgeModelCtx,
					model.Condition{Field: &bridgeModelCtx.Name, Function: ovsdb.ConditionEqual, Value: "test1"},
					model.Condition{Field: &bridgeModelCtx.Name, Function: ovsdb.ConditionEqual, Value: "test2"},
				)
			},
			selectModel:  bridgeModelCtx,
			selectFields: []any{&bridgeModelCtx.Name, &bridgeModelCtx.STPEnable},
			expectedOps: func() []ovsdb.Operation {
				expected := []ovsdb.Operation{
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "test1")},
						Columns: []string{"_uuid", "name", "stp_enable"}, // Always includes _uuid, sorted
					},
					{
						Op:      ovsdb.OperationSelect,
						Table:   "Bridge",
						Where:   []ovsdb.Condition{ovsdb.NewCondition("name", ovsdb.ConditionEqual, "test2")},
						Columns: []string{"_uuid", "name", "stp_enable"}, // Always includes _uuid, sorted
					},
				}
				return expected
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ops []ovsdb.Operation
			var err error

			// For other cases, use the conditionalAPI function
			condAPI := tt.conditionalAPI(ovsClientForDefDB)
			ops, err = condAPI.Select(tt.selectModel, tt.selectFields...)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.ErrorContains(t, err, tt.errorContains)
				}
				assert.Nil(t, ops)
			} else {
				require.NoError(t, err)
				require.Len(t, ops, len(tt.expectedOps), "Select should return %d operation", len(tt.expectedOps))
				if strings.Contains(tt.name, "WhereCache") {
					// we can not control map access order
					// sort ops by uuid
					sort.Slice(ops, func(i, j int) bool {
						return ops[i].Where[0].Value.(ovsdb.UUID).GoUUID < ops[j].Where[0].Value.(ovsdb.UUID).GoUUID
					})
				}
				for i, op := range ops {
					assert.Equal(t, tt.expectedOps[i].Op, op.Op)
					assert.Equal(t, tt.expectedOps[i].Table, op.Table)
					assert.Equal(t, tt.expectedOps[i].Where, op.Where)

					// Sort columns for comparison
					actualCols := make([]string, len(op.Columns))
					copy(actualCols, op.Columns)
					sort.Strings(actualCols)

					expectedCols := make([]string, len(tt.expectedOps[i].Columns))
					copy(expectedCols, tt.expectedOps[i].Columns)
					sort.Strings(expectedCols)

					assert.Equal(t, expectedCols, actualCols, "Selected columns mismatch")
				}
			}
		})
	}
}

func TestGetSelectResultsByIndex(t *testing.T) {
	var s ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &s)
	require.NoError(t, err)

	// Use a single ClientDBModel and DatabaseModel instance created within the test's scope
	// to ensure absolute consistency.
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	require.NoError(t, err)

	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	require.Empty(t, errs)

	theMapper := mapper.NewMapper(dbModel.Schema)

	// Helper to create a row from a model instance, using the consistent dbModel
	newRow := func(m model.Model) ovsdb.Row {
		info, err := dbModel.NewModelInfo(m)
		require.NoError(t, err)
		row, err := theMapper.NewRow(info)
		require.NoError(t, err)
		return row
	}

	// === Test Data Setup ===
	br1 := &Bridge{UUID: uuid.NewString(), Name: "br1"}
	br2 := &Bridge{UUID: uuid.NewString(), Name: "br2"}
	br3 := &Bridge{UUID: uuid.NewString(), Name: "br3"}

	ovs1 := &OpenvSwitch{UUID: uuid.NewString(), ExternalIDs: map[string]string{"test": "value"}}

	rowBr1 := newRow(br1)
	rowBr2 := newRow(br2)
	rowBr3 := newRow(br3)
	rowOvs1 := newRow(ovs1)

	queryID := "test-query"

	tests := []struct {
		name          string
		ops           func() []ovsdb.Operation
		results       []ovsdb.OperationResult
		target        interface{}
		index         int
		verify        func(t *testing.T, target interface{})
		expectError   bool
		errorContains string
	}{
		{
			name: "Single select operation with multiple results",
			ops: func() []ovsdb.Operation {
				op := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op, queryID)
				return []ovsdb.Operation{op}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1, rowBr2}},
			},
			target: &[]*Bridge{},
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 2)
				bridgeMap := make(map[string]*Bridge)
				for _, b := range bridges {
					bridgeMap[b.Name] = b
				}
				assert.Contains(t, bridgeMap, "br1")
				assert.Contains(t, bridgeMap, "br2")
			},
		},
		{
			name: "Multiple select operations with same correlation ID (deduplication)",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, queryID)
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op2, queryID)
				return []ovsdb.Operation{op1, op2}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1, rowBr3}},
				{Rows: []ovsdb.Row{rowBr2, rowBr3}}, // br3 is duplicated
			},
			target: &[]*Bridge{},
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 3, "should be 3 unique bridges after deduplication")
				bridgeMap := make(map[string]*Bridge)
				for _, b := range bridges {
					bridgeMap[b.Name] = b
				}
				assert.Contains(t, bridgeMap, "br1")
				assert.Contains(t, bridgeMap, "br2")
				assert.Contains(t, bridgeMap, "br3")
			},
		},
		{
			name: "Mixed Operations: non-select ops are ignored",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationInsert}                  // Non-select op
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"} // Select op
				ovsdb.SetCorrelationID(&op2, queryID)
				op3 := ovsdb.Operation{Op: ovsdb.OperationDelete} // Non-select op
				return []ovsdb.Operation{op1, op2, op3}
			},
			results: []ovsdb.OperationResult{
				{Count: 1},                  // Result for Insert
				{Rows: []ovsdb.Row{rowBr2}}, // Result for Select
				{Count: 1},                  // Result for Delete
			},
			target: &[]*Bridge{},
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 1)
				assert.Equal(t, "br2", bridges[0].Name)
			},
		},
		{
			name: "Multi-table select: only target table results are included (Bridge target)",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, "bridge-query")
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Open_vSwitch"}
				ovsdb.SetCorrelationID(&op2, "ovs-query")
				return []ovsdb.Operation{op1, op2}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1, rowBr2}}, // Bridge results
				{Rows: []ovsdb.Row{rowOvs1}},        // OpenvSwitch results
			},
			target: &[]*Bridge{}, // Target is Bridge slice
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 2, "should only contain Bridge results, not OpenvSwitch")
				bridgeMap := make(map[string]*Bridge)
				for _, b := range bridges {
					bridgeMap[b.Name] = b
				}
				assert.Contains(t, bridgeMap, "br1", "should contain br1")
				assert.Contains(t, bridgeMap, "br2", "should contain br2")
				// Verify it doesn't contain any OpenvSwitch data
				for _, b := range bridges {
					assert.NotEmpty(t, b.Name, "Bridge should have a name (not OpenvSwitch data)")
				}
			},
		},
		{
			name: "Multi-table select: only target table results are included (OpenvSwitch target)",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, "bridge-query")
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Open_vSwitch"}
				ovsdb.SetCorrelationID(&op2, "ovs-query")
				return []ovsdb.Operation{op1, op2}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1, rowBr2}}, // Bridge results
				{Rows: []ovsdb.Row{rowOvs1}},        // OpenvSwitch results
			},
			target: &[]*OpenvSwitch{}, // Target is OpenvSwitch slice
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				ovsRows := *target.(*[]*OpenvSwitch)
				require.Len(t, ovsRows, 1, "should only contain OpenvSwitch results, not Bridge")
				ovs := ovsRows[0]
				assert.NotEmpty(t, ovs.UUID, "OpenvSwitch should have UUID")
				assert.Equal(t, map[string]string{"test": "value"}, ovs.ExternalIDs, "should have correct ExternalIDs")
			},
		},
		{
			name:          "Error: mismatched ops and results length",
			ops:           func() []ovsdb.Operation { return []ovsdb.Operation{{}} },
			results:       []ovsdb.OperationResult{{}, {}},
			target:        &[]*Bridge{},
			index:         0,
			expectError:   true,
			errorContains: "must match",
		},
		{
			name: "Error: OVSDB error in result",
			ops: func() []ovsdb.Operation {
				op := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op, queryID)
				return []ovsdb.Operation{op}
			},
			results: []ovsdb.OperationResult{
				{Error: "some ovsdb error", Details: "details here"},
			},
			target:        &[]*Bridge{},
			index:         0,
			expectError:   true,
			errorContains: "some ovsdb error",
		},

		{
			name: "Success: same table with different correlation IDs - returns first group by default",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, "query-1")
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op2, "query-2")
				return []ovsdb.Operation{op1, op2}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1}},
				{Rows: []ovsdb.Row{rowBr2}},
			},
			target: &[]*Bridge{},
			index:  0,
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 1, "should return only first query group results")
				assert.Equal(t, "br1", bridges[0].Name)
			},
		},
		{
			name: "Error: index out of range",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, "query-1")
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op2, "query-2")
				return []ovsdb.Operation{op1, op2}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1}},
				{Rows: []ovsdb.Row{rowBr2}},
			},
			target:        &[]*Bridge{},
			index:         5, // Out of range - only 2 query groups (0, 1)
			expectError:   true,
			errorContains: "index 5 is out of range: found 2 query groups",
		},
		{
			name: "Success: same table with different correlation IDs - returns second group with index=1",
			ops: func() []ovsdb.Operation {
				op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op1, "first-query")
				op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op2, "second-query")
				op3 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op3, "third-query")
				return []ovsdb.Operation{op1, op2, op3}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1}},         // first-query results
				{Rows: []ovsdb.Row{rowBr2}},         // second-query results
				{Rows: []ovsdb.Row{rowBr3, rowBr1}}, // third-query results (with duplicate)
			},
			target: &[]*Bridge{},
			index:  1, // Request second query group (second-query)
			verify: func(t *testing.T, target interface{}) {
				bridges := *target.(*[]*Bridge)
				require.Len(t, bridges, 1, "should return only second query group results")
				assert.Equal(t, "br2", bridges[0].Name, "should return br2 from second query group")
			},
		},
		{
			name: "Error: target is not a pointer to slice of pointers",
			ops: func() []ovsdb.Operation {
				op := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
				ovsdb.SetCorrelationID(&op, queryID)
				return []ovsdb.Operation{op}
			},
			results: []ovsdb.OperationResult{
				{Rows: []ovsdb.Row{rowBr1, rowBr2}},
			},
			target:        &[]Bridge{}, // Value slice instead of pointer slice
			index:         0,
			expectError:   true,
			errorContains: "target must be a pointer to a slice of model pointers",
		},
	}

	// Create an ovsdbClient using the local, consistent ClientDBModel
	ovs, err := newOVSDBClient(clientDBModel)
	require.NoError(t, err)

	// Set up the client's internal model and cache with the correct, full database model
	ovs.primaryDB().model = dbModel
	ovs.primaryDB().cache, err = cache.NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ovs.GetSelectResultsByIndex(tt.ops(), tt.results, tt.target, tt.index)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.verify != nil {
					tt.verify(t, tt.target)
				}
			}
		})
	}

	// Test index parameter functionality
	t.Run("Index parameter test", func(t *testing.T) {
		op1 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
		ovsdb.SetCorrelationID(&op1, "query-1")
		op2 := ovsdb.Operation{Op: ovsdb.OperationSelect, Table: "Bridge"}
		ovsdb.SetCorrelationID(&op2, "query-2")
		ops := []ovsdb.Operation{op1, op2}
		results := []ovsdb.OperationResult{
			{Rows: []ovsdb.Row{rowBr1}},
			{Rows: []ovsdb.Row{rowBr2}},
		}

		// Test index 1 (second query group) with pointer slice
		var bridges2 []*Bridge
		idx := 1
		err := ovs.GetSelectResultsByIndex(ops, results, &bridges2, idx)
		require.NoError(t, err)
		require.Len(t, bridges2, 1)
		assert.Equal(t, "br2", bridges2[0].Name)

		// Test invalid index with pointer slice
		var bridges3 []*Bridge
		invalidIdx := 5
		err = ovs.GetSelectResultsByIndex(ops, results, &bridges3, invalidIdx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index 5 is out of range")
	})
}
