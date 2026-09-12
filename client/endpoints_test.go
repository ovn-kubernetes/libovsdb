package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb/serverdb"
	"github.com/stretchr/testify/require"
)

// ovn-remote and ovsdb-server remotes are comma-separated lists of endpoints.
// Given such a value, the client must try each endpoint in turn.
func TestConnectWithEndpointList(t *testing.T) {
	var defSchema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(schema), &defSchema))
	_, sock := newOVSDBServer(t, defDB, defSchema)
	serverDBModel, err := serverdb.FullDatabaseModel()
	require.NoError(t, err)

	live := "unix:" + sock
	ovs, err := newOVSDBClient(serverDBModel, WithEndpoint("unix:"+sock+".missing,"+live))
	require.NoError(t, err)
	require.NoError(t, ovs.Connect(context.Background()))
	t.Cleanup(ovs.Close)
	require.Equal(t, live, ovs.CurrentEndpoint())
}

func TestUpdateEndpointsWithEndpointList(t *testing.T) {
	ovs, err := newOVSDBClient(defDB, WithEndpoint("unix:/var/run/openvswitch/test.sock"))
	require.NoError(t, err)

	ovs.UpdateEndpoints([]string{"tcp:10.0.0.1:6642, tcp:10.0.0.2:6642,", "ssl:10.0.0.3:6642"})

	var addresses []string
	for _, ep := range ovs.endpoints {
		addresses = append(addresses, ep.address)
	}
	require.Equal(t, []string{"tcp:10.0.0.1:6642", "tcp:10.0.0.2:6642", "ssl:10.0.0.3:6642"}, addresses)
}
