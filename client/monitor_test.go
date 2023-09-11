package client

import (
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
)

func TestWithTable(t *testing.T) {
	client, err := newOVSDBClient(defDB)
	assert.NoError(t, err)
	m := newMonitor()
	opt := WithTable(&OpenvSwitch{})

	err = opt(client, m)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(m.Tables))
}

func populateClientModel(t *testing.T, client *ovsdbClient) {
	var s ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(schema), &s)
	assert.NoError(t, err)
	clientDBModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":       &Bridge{},
		"Open_vSwitch": &OpenvSwitch{},
	})
	assert.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(s, clientDBModel)
	assert.Empty(t, errs)
	client.primaryDB().model = dbModel
	assert.NoError(t, err)
}

func TestWithTableAndFields(t *testing.T) {
	client, err := newOVSDBClient(defDB)
	assert.NoError(t, err)
	populateClientModel(t, client)

	m := newMonitor()
	ovs := OpenvSwitch{}
	opt := WithTable(&ovs, &ovs.Bridges, &ovs.CurCfg)
	err = opt(client, m)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(m.Tables))
	assert.ElementsMatch(t, []string{"bridges", "cur_cfg"}, m.Tables[0].Fields)
}
