package server

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/ovn-kubernetes/libovsdb/updates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalMonitorUsesNativeModelValues(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	monitor := monitor{dbModel: &dbModel}

	port1 := uuid.NewString()
	port2 := uuid.NewString()
	row := &test.BridgeType{
		UUID:         uuid.NewString(),
		DatapathType: "",
		ExternalIDs:  map[string]string{"owner": "test", "kind": "integration"},
		Ports:        []string{port1, port2},
	}
	externalIDs, err := ovsdb.NewOvsMap(map[string]string{"owner": "test"})
	require.NoError(t, err)
	ports, err := ovsdb.NewOvsSet([]ovsdb.UUID{{GoUUID: port1}, {GoUUID: port2}})
	require.NoError(t, err)

	tests := []struct {
		name      string
		condition ovsdb.Condition
	}{
		{
			name:      "default value",
			condition: ovsdb.NewCondition("datapath_type", ovsdb.ConditionEqual, ""),
		},
		{
			name:      "map includes",
			condition: ovsdb.NewCondition("external_ids", ovsdb.ConditionIncludes, externalIDs),
		},
		{
			name:      "multi-element set includes",
			condition: ovsdb.NewCondition("ports", ovsdb.ConditionIncludes, ports),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, monitor.rowMatchesAnyCondition("Bridge", row, []ovsdb.Condition{tt.condition}))
		})
	}

	ovsRow := monitor.rowForMonitor("Bridge", row, []string{"datapath_type", "external_ids", "ports"})
	require.NotNil(t, ovsRow)
	assert.NotContains(t, *ovsRow, "datapath_type")
	assert.Contains(t, *ovsRow, "external_ids")
	assert.Contains(t, *ovsRow, "ports")
}

func TestConditionalMonitorDeleteUsesNullRow(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)
	rowUUID := uuid.NewString()
	old := &test.BridgeType{UUID: rowUUID, Name: "bridge"}
	monitor := monitor{
		dbModel: &dbModel,
		request: map[string]*ovsdb.MonitorRequest{
			"Bridge": {
				Columns: []string{"name"},
				Where: []ovsdb.Condition{
					ovsdb.NewCondition("name", ovsdb.ConditionEqual, "bridge"),
				},
				Select: ovsdb.NewDefaultMonitorSelect(),
			},
		},
	}
	update := updates.ModelUpdates{}
	err = update.AddRowUpdate2(dbModel, "Bridge", rowUUID, old, ovsdb.RowUpdate2{Delete: &ovsdb.Row{}})
	require.NoError(t, err)

	tableUpdates := monitor.filter2(updates.NewDatabaseUpdate(update, nil))
	require.Contains(t, tableUpdates, "Bridge")
	require.Contains(t, tableUpdates["Bridge"], rowUUID)
	require.NotNil(t, tableUpdates["Bridge"][rowUUID].Delete)
	assert.Empty(t, *tableUpdates["Bridge"][rowUUID].Delete)
}

func TestMonitorFilter(t *testing.T) {
	monitor := monitor{
		request: map[string]*ovsdb.MonitorRequest{
			"Bridge": {
				Columns: []string{"name"},
				Select:  ovsdb.NewDefaultMonitorSelect(),
			},
		},
	}
	bridgeRow := ovsdb.Row{
		"name": "bar",
	}
	bridgeExternalIDs, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar"})
	bridgeRowWithIDs := ovsdb.Row{
		"name":         "bar",
		"external_ids": bridgeExternalIDs,
	}
	tests := []struct {
		name     string
		update   ovsdb.TableUpdates2
		expected ovsdb.TableUpdates2
	}{
		{
			"not filtered",
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
		},
		{
			"removed table",
			ovsdb.TableUpdates2{
				"Open_vSwitch": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
			ovsdb.TableUpdates2{},
		},
		{
			"removed column",
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRowWithIDs,
					},
				},
			},
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbModel, err := test.GetModel()
			require.NoError(t, err)
			update := updates.ModelUpdates{}
			for table, rows := range tt.update {
				for uuid, row := range rows {
					err := update.AddRowUpdate2(dbModel, table, uuid, nil, *row)
					require.NoError(t, err)
				}
			}
			tu := monitor.filter2(updates.NewDatabaseUpdate(update, nil))
			assert.Equal(t, tt.expected, tu)
		})
	}
}

func TestMonitorFilter2(t *testing.T) {
	monitor := monitor{
		request: map[string]*ovsdb.MonitorRequest{
			"Bridge": {
				Columns: []string{"name"},
				Select:  ovsdb.NewDefaultMonitorSelect(),
			},
		},
	}
	bridgeRow := ovsdb.Row{
		"name": "bar",
	}
	bridgeExternalIDs, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar"})
	bridgeRowWithIDs := ovsdb.Row{
		"name":         "bar",
		"external_ids": bridgeExternalIDs,
	}
	tests := []struct {
		name     string
		update   ovsdb.TableUpdates2
		expected ovsdb.TableUpdates2
	}{
		{
			"not filtered",
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
		},
		{
			"removed table",
			ovsdb.TableUpdates2{
				"Open_vSwitch": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
			ovsdb.TableUpdates2{},
		},
		{
			"removed column",
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRowWithIDs,
					},
				},
			},
			ovsdb.TableUpdates2{
				"Bridge": ovsdb.TableUpdate2{
					"foo": &ovsdb.RowUpdate2{
						Insert: &bridgeRow,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbModel, err := test.GetModel()
			require.NoError(t, err)
			update := updates.ModelUpdates{}
			for table, rows := range tt.update {
				for uuid, row := range rows {
					err := update.AddRowUpdate2(dbModel, table, uuid, nil, *row)
					require.NoError(t, err)
				}
			}
			tu := monitor.filter2(updates.NewDatabaseUpdate(update, nil))
			assert.Equal(t, tt.expected, tu)
		})
	}
}
