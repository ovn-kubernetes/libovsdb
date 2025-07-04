package updates

import (
	"testing"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdates_AddOperation(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)

	type fields struct {
		updates map[string]map[string]modelUpdate
	}
	type args struct {
		dbModel model.DatabaseModel
		table   string
		uuid    string
		current model.Model
		op      *ovsdb.Operation
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected fields
		wantErr  bool
	}{
		{
			name: "insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
								},
								New: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "insert after insert fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "insert after update fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "type",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "type",
								},
								Modify: &ovsdb.Row{
									"datapath_type": "type",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "insert after delete fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Delete: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "insert ignores unknown columns",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"unknown": "unknown",
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
								},
								New: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "insert with bad column type fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationInsert,
					Row: ovsdb.Row{
						"datapath_type": 0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:        "uuid",
					Name:        "bridge",
					ExternalIds: map[string]string{"key": "value", "key1": "value1"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": "type",
						"external_ids":  ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value1", "key2": "value2"}},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"key": "value", "key1": "value1"},
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "type",
								ExternalIds:  map[string]string{"key": "value1", "key2": "value2"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"_uuid":        ovsdb.UUID{GoUUID: "uuid"},
									"name":         "bridge",
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value", "key1": "value1"}},
								},
								New: &ovsdb.Row{
									"_uuid":         ovsdb.UUID{GoUUID: "uuid"},
									"name":          "bridge",
									"datapath_type": "type",
									"external_ids":  ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value1", "key2": "value2"}},
								},
								Modify: &ovsdb.Row{
									"datapath_type": "type",
									"external_ids":  ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value1", "key1": "value1", "key2": "value2"}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update no op",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:        "uuid",
					Name:        "bridge",
					ExternalIds: map[string]string{"key": "value", "key1": "value1"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value", "key1": "value1"}},
					},
				},
			},
		},
		{
			name: "update after insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": "type",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								New: &ovsdb.Row{
									"name": "bridge",
								},
								Insert: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "type",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								New: &ovsdb.Row{
									"_uuid":         ovsdb.UUID{GoUUID: "uuid"},
									"name":          "bridge",
									"datapath_type": "type",
								},
								Insert: &ovsdb.Row{
									"_uuid":         ovsdb.UUID{GoUUID: "uuid"},
									"name":          "bridge",
									"datapath_type": "type",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update after update",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "old",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": "new",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "old",
								},
								Modify: &ovsdb.Row{
									"datapath_type": "old",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "new",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"_uuid":         ovsdb.UUID{GoUUID: "uuid"},
									"name":          "bridge",
									"datapath_type": "new",
								},
								Modify: &ovsdb.Row{
									"datapath_type": "new",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update after update results in no op",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "type",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": "",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "type",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "type",
								},
								Modify: &ovsdb.Row{
									"datapath_type": "type",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: nil,
			},
		},
		{
			name: "update after delete fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": "type",
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								Delete: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update nil model fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update different type of model fails",
			args: args{
				table:   "Bridge",
				uuid:    "uuid",
				current: &test.OvsType{},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update an inmutable column fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"name": "bridge2",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "update unknown column ignored",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"unknown": "bridge",
					},
				},
			},
		},
		{
			name: "update with bad column type fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationUpdate,
					Row: ovsdb.Row{
						"datapath_type": 0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutate map multiple times",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:        "uuid",
					Name:        "bridge",
					ExternalIds: map[string]string{"key1": "value1", "key2": "value2"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "external_ids",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key3": "value3", "key1": "value2"}},
						},
						{
							Column:  "external_ids",
							Mutator: ovsdb.MutateOperationDelete,
							Value:   ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key2": "value2"}},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"key1": "value1", "key2": "value2"},
							},
							new: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"key1": "value1", "key3": "value3"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"_uuid":        ovsdb.UUID{GoUUID: "uuid"},
									"name":         "bridge",
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key2": "value2"}},
								},
								New: &ovsdb.Row{
									"_uuid":        ovsdb.UUID{GoUUID: "uuid"},
									"name":         "bridge",
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key3": "value3"}},
								},
								Modify: &ovsdb.Row{
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key2": "value2", "key3": "value3"}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mutate set multiple times",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:  "uuid",
					Name:  "bridge",
					Ports: []string{"uuid1", "uuid2"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid1"}, ovsdb.UUID{GoUUID: "uuid3"}}},
						},
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationDelete,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid3"}, ovsdb.UUID{GoUUID: "uuid1"}}},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"uuid1", "uuid2"},
							},
							new: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"uuid2"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid1"}, ovsdb.UUID{GoUUID: "uuid2"}}},
								},
								New: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid2"}}},
								},
								Modify: &ovsdb.Row{
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid1"}}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mutate can result in no op",
			args: args{
				table: "Flow_Sample_Collector_Set",
				uuid:  "uuid",
				current: &test.FlowSampleCollectorSetType{
					UUID:        "uuid",
					ID:          1,
					ExternalIDs: map[string]string{"key": "value"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "id",
							Mutator: ovsdb.MutateOperationAdd,
							Value:   1,
						},
						{
							Column:  "id",
							Mutator: ovsdb.MutateOperationSubtract,
							Value:   1,
						},
						{
							Column:  "external_ids",
							Mutator: ovsdb.MutateOperationDelete,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{"key"}},
						},
						{
							Column:  "external_ids",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key": "value"}},
						},
					},
				},
			},
		},
		{
			name: "mutate after insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
						},
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"uuid"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								New: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
								},
								Insert: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mutate after update",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge2",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
						},
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge2",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name": "bridge2",
								},
								Modify: &ovsdb.Row{
									"name": "bridge2",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge2",
								Ports: []string{"uuid"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge2",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
								},
								Modify: &ovsdb.Row{
									"name":  "bridge2",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid"}}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mutate after mutate",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:        "uuid",
					Name:        "bridge",
					ExternalIds: map[string]string{"key1": "value1"},
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "external_ids",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key2": "value2"}},
						},
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"key1": "value1"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name":         "bridge",
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1"}},
								},
								Modify: &ovsdb.Row{
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1"}},
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"key1": "value1", "key2": "value2"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"_uuid":        ovsdb.UUID{GoUUID: "uuid"},
									"name":         "bridge",
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key2": "value2"}},
								},
								Modify: &ovsdb.Row{
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"key1": "value1", "key2": "value2"}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mutate after delete fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid-2"}}},
						},
					},
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"uuid-1"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid-1"}}},
								},
								Delete: &ovsdb.Row{
									"name":  "bridge",
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid-1"}}},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutate nil model fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid-2"}}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutate different type of model fails",
			args: args{
				table:   "Bridge",
				uuid:    "uuid",
				current: &test.OvsType{},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "ports",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "uuid-2"}}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutate an inmmutable column fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "name",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   "bridge2",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutate with bad column type fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationMutate,
					Mutations: []ovsdb.Mutation{
						{
							Column:  "datapath_type",
							Mutator: ovsdb.MutateOperationInsert,
							Value:   0,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "delete",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationDelete,
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"_uuid": ovsdb.UUID{GoUUID: "uuid"},
									"name":  "bridge",
								},
								Delete: &ovsdb.Row{},
							},
						},
					},
				},
			},
		},
		{
			name: "delete after insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationDelete,
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"name": "bridge",
								},
								New: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: nil,
			},
		},
		{
			name: "delete after update",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "new",
				},
				op: &ovsdb.Operation{
					Op: ovsdb.OperationDelete,
				},
			},
			fields: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "new",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "old",
								},
								New: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "new",
								},
								Modify: &ovsdb.Row{
									"datapath_type": "new",
								},
							},
						},
					},
				},
			},
			expected: fields{
				updates: map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "old",
								},
								Delete: &ovsdb.Row{},
							},
						},
					},
				},
			},
		},
		{
			name: "delete nil model fails",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				op: &ovsdb.Operation{
					Op: ovsdb.OperationDelete,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &ModelUpdates{
				updates: tt.fields.updates,
			}
			tt.args.dbModel = dbModel
			err := u.AddOperation(tt.args.dbModel, tt.args.table, tt.args.uuid, tt.args.current, tt.args.op)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.updates, u.updates)
		})
	}
}

func TestModelUpdates_AddRowUpdate2(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)

	oldDatapathID := "old"
	newDatapathID := "new"

	type fields struct {
		updates map[string]map[string]modelUpdate
	}
	type args struct {
		dbModel model.DatabaseModel
		table   string
		uuid    string
		current model.Model
		ru2     ovsdb.RowUpdate2
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected fields
		wantErr  bool
	}{
		{
			name: "insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				ru2: ovsdb.RowUpdate2{
					Insert: &ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Insert: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "old",
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_type": "new",
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "new",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"datapath_type": "new",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify, add and remove from set",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:  "uuid",
					Name:  "bridge",
					Ports: []string{"foo"},
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "foo"}, ovsdb.UUID{GoUUID: "bar"}}},
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"foo"},
							},
							new: &test.BridgeType{
								UUID:  "uuid",
								Name:  "bridge",
								Ports: []string{"bar"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"ports": ovsdb.OvsDataSet{GoSet: []interface{}{ovsdb.UUID{GoUUID: "foo"}, ovsdb.UUID{GoUUID: "bar"}}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify, add, update and remove from map",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:        "uuid",
					Name:        "bridge",
					ExternalIds: map[string]string{"foo": "bar", "baz": "qux"},
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"foo": "bar", "bar": "baz", "baz": "quux"}},
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"foo": "bar", "baz": "qux"},
							},
							new: &test.BridgeType{
								UUID:        "uuid",
								Name:        "bridge",
								ExternalIds: map[string]string{"bar": "baz", "baz": "quux"},
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"external_ids": ovsdb.OvsMap{GoMap: map[interface{}]interface{}{"foo": "bar", "bar": "baz", "baz": "quux"}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify optional",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:       "uuid",
					Name:       "bridge",
					DatapathID: &oldDatapathID,
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{newDatapathID}},
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:       "uuid",
								Name:       "bridge",
								DatapathID: &oldDatapathID,
							},
							new: &test.BridgeType{
								UUID:       "uuid",
								Name:       "bridge",
								DatapathID: &newDatapathID,
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{newDatapathID}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify add optional",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{newDatapathID}},
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							new: &test.BridgeType{
								UUID:       "uuid",
								Name:       "bridge",
								DatapathID: &newDatapathID,
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{newDatapathID}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify remove optional",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:       "uuid",
					Name:       "bridge",
					DatapathID: &oldDatapathID,
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{}},
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:       "uuid",
								Name:       "bridge",
								DatapathID: &oldDatapathID,
							},
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"datapath_id": ovsdb.OvsDataSet{GoSet: []interface{}{}},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modify no op",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "type",
					DatapathID:   &oldDatapathID,
					Ports:        []string{"foo", "bar"},
					ExternalIds:  map[string]string{"foo": "bar", "baz": "qux"},
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_type": "type",
						"datapath_id":   ovsdb.OvsDataSet{GoSet: []interface{}{oldDatapathID}},
						"ports":         ovsdb.OvsDataSet{GoSet: []interface{}{}},
						"external_ids":  ovsdb.OvsMap{GoMap: map[interface{}]interface{}{}},
					},
				},
			},
		},
		{
			name: "modify unknown colum",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "old",
				},
				ru2: ovsdb.RowUpdate2{
					Modify: &ovsdb.Row{
						"datapath_type": "new",
						"unknown":       "column",
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "new",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Modify: &ovsdb.Row{
									"datapath_type": "new",
									"unknown":       "column",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "delete",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &ModelUpdates{
				updates: tt.fields.updates,
			}
			tt.args.dbModel = dbModel
			err := u.AddRowUpdate2(tt.args.dbModel, tt.args.table, tt.args.uuid, tt.args.current, tt.args.ru2)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.updates, u.updates)
		})
	}
}

func TestModelUpdates_AddRowUpdate(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)

	type fields struct {
		updates map[string]map[string]modelUpdate
	}
	type args struct {
		dbModel model.DatabaseModel
		table   string
		uuid    string
		current model.Model
		ru      ovsdb.RowUpdate
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected fields
		wantErr  bool
	}{
		{
			name: "insert",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				ru: ovsdb.RowUpdate{
					New: &ovsdb.Row{
						"name": "bridge",
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							new: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								New: &ovsdb.Row{
									"name": "bridge",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "old",
				},
				ru: ovsdb.RowUpdate{
					Old: &ovsdb.Row{
						"name":          "bridge",
						"datapath_type": "old",
					},
					New: &ovsdb.Row{
						"name":          "bridge",
						"datapath_type": "new",
					},
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "old",
							},
							new: &test.BridgeType{
								UUID:         "uuid",
								Name:         "bridge",
								DatapathType: "new",
							},
							rowUpdate2: &ovsdb.RowUpdate2{
								Old: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "old",
								},
								New: &ovsdb.Row{
									"name":          "bridge",
									"datapath_type": "new",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update no op",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID:         "uuid",
					Name:         "bridge",
					DatapathType: "type",
				},
				ru: ovsdb.RowUpdate{
					Old: &ovsdb.Row{
						"name":          "bridge",
						"datapath_type": "type",
					},
					New: &ovsdb.Row{
						"name":          "bridge",
						"datapath_type": "type",
					},
				},
			},
		},
		{
			name: "delete",
			args: args{
				table: "Bridge",
				uuid:  "uuid",
				current: &test.BridgeType{
					UUID: "uuid",
					Name: "bridge",
				},
			},
			expected: fields{
				map[string]map[string]modelUpdate{
					"Bridge": {
						"uuid": {
							old: &test.BridgeType{
								UUID: "uuid",
								Name: "bridge",
							},
							rowUpdate2: &ovsdb.RowUpdate2{},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &ModelUpdates{
				updates: tt.fields.updates,
			}
			tt.args.dbModel = dbModel
			err := u.AddRowUpdate(tt.args.dbModel, tt.args.table, tt.args.uuid, tt.args.current, tt.args.ru)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.updates, u.updates)
		})
	}
}
