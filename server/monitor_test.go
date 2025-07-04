package server

import (
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/ovn-kubernetes/libovsdb/updates"
	"github.com/stretchr/testify/assert"
)

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
		"_uuid": "foo",
		"name":  "bar",
	}
	bridgeExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar"})
	bridgeRowWithIDs := ovsdb.Row{
		"_uuid":        "foo",
		"name":         "bar",
		"external_ids": bridgeExternalIds,
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
			assert.NoError(t, err)
			update := updates.ModelUpdates{}
			for table, rows := range tt.update {
				for uuid, row := range rows {
					err := update.AddRowUpdate2(dbModel, table, uuid, nil, *row)
					assert.NoError(t, err)
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
	bridgeExternalIds, _ := ovsdb.NewOvsMap(map[string]string{"foo": "bar"})
	bridgeRowWithIDs := ovsdb.Row{
		"name":         "bar",
		"external_ids": bridgeExternalIds,
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
			assert.NoError(t, err)
			update := updates.ModelUpdates{}
			for table, rows := range tt.update {
				for uuid, row := range rows {
					err := update.AddRowUpdate2(dbModel, table, uuid, nil, *row)
					assert.NoError(t, err)
				}
			}
			tu := monitor.filter2(updates.NewDatabaseUpdate(update, nil))
			assert.Equal(t, tt.expected, tu)
		})
	}
}
