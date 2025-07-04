package updates

import (
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_merge(t *testing.T) {
	oldDatapathID := "old"
	newDatapathID := "new"
	type args struct {
		a modelUpdate
		b modelUpdate
	}
	tests := []struct {
		name    string
		args    args
		want    modelUpdate
		wantErr bool
	}{
		{
			name: "no op",
		},
		{
			name: "insert",
			args: args{
				b: modelUpdate{
					new: &test.BridgeType{
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
			want: modelUpdate{
				new: &test.BridgeType{
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
		{
			name: "update",
			args: args{
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				new: &test.BridgeType{
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
		{
			name: "delete",
			args: args{
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					Delete: &ovsdb.Row{},
				},
			},
		},
		{
			name: "no op after insert",
			args: args{
				a: modelUpdate{
					new: &test.BridgeType{
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
			want: modelUpdate{
				new: &test.BridgeType{
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
		{
			name: "no op after update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				new: &test.BridgeType{
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
		{
			name: "no op after delete",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					Delete: &ovsdb.Row{},
				},
			},
		},
		{
			name: "insert after insert fails",
			args: args{
				a: modelUpdate{
					new: &test.BridgeType{
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
				b: modelUpdate{
					new: &test.BridgeType{
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
			wantErr: true,
		},
		{
			name: "insert after update fails",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
				b: modelUpdate{
					new: &test.BridgeType{
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
			wantErr: true,
		},
		{
			name: "insert after delete fails",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
				b: modelUpdate{
					new: &test.BridgeType{
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
			wantErr: true,
		},
		{
			name: "update after insert",
			args: args{
				a: modelUpdate{
					new: &test.BridgeType{
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
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
			want: modelUpdate{
				new: &test.BridgeType{
					Name: "bridge2",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Insert: &ovsdb.Row{
						"name": "bridge2",
					},
					New: &ovsdb.Row{
						"name": "bridge2",
					},
				},
			},
		},
		{
			name: "update after update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge2",
					},
					new: &test.BridgeType{
						Name: "bridge3",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge2",
						},
						New: &ovsdb.Row{
							"name": "bridge3",
						},
						Modify: &ovsdb.Row{
							"name": "bridge3",
						},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				new: &test.BridgeType{
					Name: "bridge3",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					New: &ovsdb.Row{
						"name": "bridge3",
					},
					Modify: &ovsdb.Row{
						"name": "bridge3",
					},
				},
			},
		},
		{
			name: "update after delete fails",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge2",
					},
					new: &test.BridgeType{
						Name: "bridge3",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge2",
						},
						New: &ovsdb.Row{
							"name": "bridge3",
						},
						Modify: &ovsdb.Row{
							"name": "bridge3",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "delete after insert",
			args: args{
				a: modelUpdate{
					new: &test.BridgeType{
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
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
			},
		},
		{
			name: "delete after update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge2",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge2",
						},
						Delete: &ovsdb.Row{},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					Delete: &ovsdb.Row{},
				},
			},
		},
		{
			name: "delete after delete",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						Delete: &ovsdb.Row{},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					Delete: &ovsdb.Row{},
				},
			},
		},
		{
			name: "update atomic field to original value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
						Name:         "bridge2",
						DatapathType: "type",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						New: &ovsdb.Row{
							"name":          "bridge2",
							"datapath_type": "type",
						},
						Modify: &ovsdb.Row{
							"name":          "bridge2",
							"datapath_type": "type",
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:         "bridge2",
						DatapathType: "type",
					},
					new: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":          "bridge2",
							"datapath_type": "type",
						},
						New: &ovsdb.Row{
							"name": "bridge",
						},
						Modify: &ovsdb.Row{
							"name":          "bridge",
							"datapath_type": "",
						},
					},
				},
			},
		},
		{
			name: "update atomic field to same updated value after update results in original update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
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
				b: modelUpdate{
					old: &test.BridgeType{
						Name:         "bridge",
						DatapathType: "type",
					},
					new: &test.BridgeType{
						Name:         "bridge",
						DatapathType: "type",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":          "bridge",
							"datapath_type": "type",
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
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				new: &test.BridgeType{
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
		{
			name: "update optional field to same value after update results in original update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name: "bridge",
				},
				new: &test.BridgeType{
					Name:       "bridge",
					DatapathID: &newDatapathID,
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name": "bridge",
					},
					New: &ovsdb.Row{
						"name":        "bridge",
						"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
					},
					Modify: &ovsdb.Row{
						"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
					},
				},
			},
		},
		{
			name: "update optional field to original value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &oldDatapathID,
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &oldDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
					},
				},
			},
		},
		{
			name: "update optional field to original empty value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name: "bridge",
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name": "bridge",
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					new: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						New: &ovsdb.Row{
							"name": "bridge",
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{}},
						},
					},
				},
			},
		},
		{
			name: "update optional field to empty value after update",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &oldDatapathID,
					},
					new: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
						New: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:       "bridge",
						DatapathID: &newDatapathID,
					},
					new: &test.BridgeType{
						Name: "bridge",
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":        "bridge",
							"datapath_id": ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						New: &ovsdb.Row{
							"name": "bridge",
						},
						Modify: &ovsdb.Row{
							"datapath_id": ovsdb.OvsSet{GoSet: []any{}},
						},
					},
				},
			},
			want: modelUpdate{
				old: &test.BridgeType{
					Name:       "bridge",
					DatapathID: &oldDatapathID,
				},
				new: &test.BridgeType{
					Name: "bridge",
				},
				rowUpdate2: &ovsdb.RowUpdate2{
					Old: &ovsdb.Row{
						"name":        "bridge",
						"datapath_id": ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
					},
					New: &ovsdb.Row{
						"name": "bridge",
					},
					Modify: &ovsdb.Row{
						"datapath_id": ovsdb.OvsSet{GoSet: []any{}},
					},
				},
			},
		},
		{
			name: "update set field to original value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name:  "bridge",
						Ports: []string{"port1", "port2"},
					},
					new: &test.BridgeType{
						Name:  "bridge",
						Ports: []string{"port1", "port3"},
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port1", "port2"}},
						},
						New: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port1", "port3"}},
						},
						Modify: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port2", "port3"}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:  "bridge",
						Ports: []string{"port1", "port3"},
					},
					new: &test.BridgeType{
						Name:  "bridge",
						Ports: []string{"port1", "port2"},
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port1", "port3"}},
						},
						New: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port1", "port2"}},
						},
						Modify: &ovsdb.Row{
							"ports": ovsdb.OvsSet{GoSet: []any{"port2", "port3"}},
						},
					},
				},
			},
		},
		{
			name: "update map field to original value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name:        "bridge",
						ExternalIDs: map[string]string{"key": "value", "key1": "value1", "key2": "value2"},
					},
					new: &test.BridgeType{
						Name:        "bridge",
						ExternalIDs: map[string]string{"key": "value1", "key1": "value1", "key3": "value3"},
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key1": "value1", "key2": "value2"}},
						},
						New: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key1": "value1", "key3": "value3"}},
						},
						Modify: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key2": "value2", "key3": "value3"}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:        "bridge",
						ExternalIDs: map[string]string{"key": "value1", "key1": "value1", "key3": "value3"},
					},
					new: &test.BridgeType{
						Name:        "bridge",
						ExternalIDs: map[string]string{"key": "value", "key1": "value1", "key2": "value2"},
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key1": "value1", "key3": "value3"}},
						},
						New: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key1": "value1", "key2": "value2"}},
						},
						Modify: &ovsdb.Row{
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key2": "value2", "key3": "value3"}},
						},
					},
				},
			},
		},
		{
			name: "update multiple fields to original value after update results in no op",
			args: args{
				a: modelUpdate{
					old: &test.BridgeType{
						Name:        "bridge",
						Ports:       []string{"port1", "port2"},
						ExternalIDs: map[string]string{"key": "value", "key1": "value1", "key2": "value2"},
						DatapathID:  &oldDatapathID,
					},
					new: &test.BridgeType{
						Name:        "bridge2",
						Ports:       []string{"port1", "port3"},
						ExternalIDs: map[string]string{"key": "value1", "key1": "value1", "key3": "value3"},
						DatapathID:  &newDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":         "bridge",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port1", "port2"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key1": "value1", "key2": "value2"}},
							"datapath_id":  ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
						New: &ovsdb.Row{
							"name":         "bridge2",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port1", "port3"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key1": "value1", "key3": "value3"}},
							"datapath_id":  ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
						Modify: &ovsdb.Row{
							"name":         "bridge2",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port2", "port3"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key2": "value2", "key3": "value3"}},
							"datapath_id":  ovsdb.OvsSet{GoSet: []any{newDatapathID}},
						},
					},
				},
				b: modelUpdate{
					old: &test.BridgeType{
						Name:        "bridge2",
						Ports:       []string{"port1", "port3"},
						ExternalIDs: map[string]string{"key": "value1", "key1": "value1", "key3": "value3"},
						DatapathID:  &newDatapathID,
					},
					new: &test.BridgeType{
						Name:        "bridge",
						Ports:       []string{"port1", "port2"},
						ExternalIDs: map[string]string{"key": "value", "key1": "value1", "key2": "value2"},
						DatapathID:  &oldDatapathID,
					},
					rowUpdate2: &ovsdb.RowUpdate2{
						Old: &ovsdb.Row{
							"name":         "bridge2",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port1", "port3"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value1", "key1": "value1", "key3": "value3"}},
						},
						New: &ovsdb.Row{
							"name":         "bridge",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port1", "port2"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key1": "value1", "key2": "value2"}},
						},
						Modify: &ovsdb.Row{
							"name":         "bridge",
							"ports":        ovsdb.OvsSet{GoSet: []any{"port2", "port3"}},
							"external_ids": ovsdb.OvsMap{GoMap: map[any]any{"key": "value", "key2": "value2", "key3": "value3"}},
							"datapath_id":  ovsdb.OvsSet{GoSet: []any{oldDatapathID}},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbModel, err := test.GetModel()
			require.NoError(t, err)
			ts := dbModel.Schema.Table("Bridge")
			got, err := merge(ts, tt.args.a, tt.args.b)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
