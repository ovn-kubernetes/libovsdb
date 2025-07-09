package cache

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/go-logr/logr"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/ovn-kubernetes/libovsdb/test"
	"github.com/ovn-kubernetes/libovsdb/updates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testModel struct {
	UUID     string   `ovsdb:"_uuid"`
	Foo      string   `ovsdb:"foo"`
	Bar      string   `ovsdb:"bar"`
	Baz      int      `ovsdb:"baz"`
	Array    []string `ovsdb:"array"`
	Datapath *string  `ovsdb:"datapath"`
}

const testSchemaFmt string = `{
  "name": "Open_vSwitch",
  "tables": {
    "Open_vSwitch": {
`

const testSchemaFmt2 string = `
      "columns": {
        "foo": {
          "type": "string"
        },
        "bar": {
          "type": "string"
        },
        "baz": {
          "type": "integer"
        },
        "array": {
          "type": {
            "key": {
              "type": "string"
            },
            "min": 0,
            "max": "unlimited"
          }
        },
        "datapath": {
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

func getTestSchema(indexes string) []byte {
	if len(indexes) > 0 {
		return []byte(testSchemaFmt + fmt.Sprintf(`"indexes": [%s],`, indexes) + testSchemaFmt2)
	}
	return []byte(testSchemaFmt + testSchemaFmt2)
}

func TestRowCache_Row(t *testing.T) {

	type fields struct {
		cache map[string]model.Model
	}
	type args struct {
		uuid string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   model.Model
	}{
		{
			"returns a row that exists",
			fields{cache: map[string]model.Model{"test": &testModel{}}},
			args{uuid: "test"},
			&testModel{},
		},
		{
			"returns a nil for a row that does not exist",
			fields{cache: map[string]model.Model{"test": &testModel{}}},
			args{uuid: "foo"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RowCache{
				cache: tt.fields.cache,
			}
			got := r.Row(tt.args.uuid)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRowCache_Rows(t *testing.T) {
	tests := []struct {
		name  string
		cache map[string]model.Model
		want  map[string]model.Model
	}{
		{
			"returns a rows that exist",
			map[string]model.Model{"test1": &testModel{}, "test2": &testModel{}, "test3": &testModel{}},
			map[string]model.Model{"test1": &testModel{}, "test2": &testModel{}, "test3": &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RowCache{
				cache: tt.cache,
			}
			got := r.Rows()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRowCacheCreate(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar"}},
	}

	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	tests := []struct {
		name       string
		uuid       string
		model      *testModel
		checkIndex bool
		wantErr    bool
	}{
		{
			"inserts a new row",
			"foo",
			&testModel{Foo: "foo"},
			true,
			false,
		},
		{
			"error duplicate uuid",
			"bar",
			&testModel{Foo: "foo"},
			true,
			true,
		},
		{
			"error duplicate index",
			"baz",
			&testModel{Foo: "bar"},
			true,
			true,
		},
		{
			"error duplicate uuid, no index check",
			"bar",
			&testModel{Foo: "bar"},
			false,
			true,
		},
		{
			"no error duplicate index",
			"baz",
			&testModel{Foo: "bar"},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Create(tt.uuid, tt.model, tt.checkIndex)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, rc.indexes["foo"][tt.model.Foo], 1)
				assert.Equal(t, tt.uuid, rc.indexes["foo"][tt.model.Foo].getAny())
			}
		})
	}
}

func TestRowCacheCreateClientIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(""), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar"}},
	}

	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected valueToUUIDs
	}{
		{
			name:    "inserts a new row",
			uuid:    "foo",
			model:   &testModel{Foo: "foo"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("foo"),
				"bar": newUUIDSet("bar"),
			},
		},
		{
			name:    "error duplicate uuid",
			uuid:    "bar",
			model:   &testModel{Foo: "foo"},
			wantErr: true,
		},
		{
			name:    "inserts duplicate index",
			uuid:    "baz",
			model:   &testModel{Foo: "bar"},
			wantErr: false,
			expected: valueToUUIDs{
				"bar": newUUIDSet("bar", "baz"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err = rc.Create(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, rc.indexes["foo"])
			}
		})
	}
}

func getStringPtr(s string) *string {
	return &s
}

var nilString *string

func TestRowCacheCreateOptionalColumnClientIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "datapath",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(""), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Datapath: getStringPtr("bar")}},
	}

	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected valueToUUIDs
	}{
		{
			name:    "inserts a new row",
			uuid:    "foo",
			model:   &testModel{Datapath: getStringPtr("foo")},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("foo"),
				"bar": newUUIDSet("bar"),
			},
		},
		{
			name:    "error duplicate uuid",
			uuid:    "bar",
			model:   &testModel{Datapath: getStringPtr("foo")},
			wantErr: true,
		},
		{
			name:    "inserts duplicate index",
			uuid:    "baz",
			model:   &testModel{Datapath: getStringPtr("bar")},
			wantErr: false,
			expected: valueToUUIDs{
				"bar": newUUIDSet("bar", "baz"),
			},
		},
		{
			name:    "inserts nil index",
			uuid:    "nil",
			model:   &testModel{Datapath: nil},
			wantErr: false,
			expected: valueToUUIDs{
				"bar":     newUUIDSet("bar"),
				nilString: newUUIDSet("nil"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err = rc.Create(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, rc.indexes["datapath"])
			}
		})
	}
}

func TestRowCacheCreateMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo", "bar",  "datapath"]`), &schema)
	require.NoError(t, err)
	index := newIndexFromColumns("foo", "bar", "datapath")
	// Note datapath purposely left empty for initial data to exercise handling of nil pointer
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar", Bar: "bar"}},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)
	fakeDatapath := "fakePath"
	tests := []struct {
		name               string
		uuid               string
		model              *testModel
		wantErr            bool
		wantIndexExistsErr bool
	}{
		{
			"inserts a new row",
			"foo",
			&testModel{Foo: "foo", Bar: "foo"},
			false,
			false,
		},
		{
			"error duplicate uuid",
			"bar",
			&testModel{Foo: "bar", Bar: "bar"},
			true,
			false,
		},
		{
			"error duplicate index",
			"baz",
			&testModel{Foo: "foo", Bar: "foo"},
			true,
			true,
		},
		{
			"new row with one duplicate value",
			"baz",
			&testModel{Foo: "foo", Bar: "bar"},
			false,
			false,
		},
		{
			"new row with other duplicate value",
			"quux",
			&testModel{Foo: "bar", Bar: "baz"},
			false,
			false,
		},
		{
			"new row with non nil pointer value, but other column indexes overlap",
			"quux2",
			&testModel{Foo: "bar", Bar: "baz", Datapath: &fakeDatapath},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Create(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantIndexExistsErr {
					assert.IsType(t, &ErrIndexExists{}, err)
				}
			} else {
				require.NoError(t, err)
				mapperInfo, err := dbModel.NewModelInfo(tt.model)
				require.NoError(t, err)
				h, err := valueFromIndex(mapperInfo, newColumnKeysFromColumns("foo", "bar", "datapath"))
				require.NoError(t, err)
				assert.Len(t, rc.indexes[index][h], 1)
				assert.Equal(t, tt.uuid, rc.indexes[index][h].getAny())
			}
		})
	}
}

func TestRowCacheCreateMultiClientIndex(t *testing.T) {
	type testModel struct {
		UUID string            `ovsdb:"_uuid"`
		Foo  string            `ovsdb:"foo"`
		Bar  map[string]string `ovsdb:"bar"`
	}
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)

	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
					{
						Column: "bar",
						Key:    "bar",
					},
				},
			},
		},
	})
	index := newIndexFromColumnKeys(db.Indexes("Open_vSwitch")[0].Columns...)

	err = json.Unmarshal([]byte(`{
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
		    "columns": {
		      "foo": {
			    "type": "string"
			  },
			  "bar": {
				"type": {
					"key": "string",
					"value": "string",
					"min": 0, 
					"max": "unlimited"
				}
			  }
		    }
		  }
		}
	}`), &schema)
	require.NoError(t, err)

	testData := Data{
		"Open_vSwitch": map[string]model.Model{"bar": &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}}},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	type expected struct {
		index model.Model
		uuids uuidset
	}

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected []expected
	}{
		{
			name:    "inserts a new row",
			uuid:    "foo",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("foo"),
				},
			},
		},
		{
			name:    "error duplicate uuid",
			uuid:    "bar",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "bar"}},
			wantErr: true,
		},
		{
			name:    "inserts duplicate index",
			uuid:    "baz",
			model:   &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "baz"),
				},
			},
		},
		{
			name:    "new row with one duplicate value",
			uuid:    "baz",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "bar"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("baz"),
				},
			},
		},
		{
			name:    "new row with other duplicate value",
			uuid:    "baz",
			model:   &testModel{Foo: "bar", Bar: map[string]string{"bar": "foo"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("baz"),
				},
			},
		},
		{
			name:    "new row with nil map index",
			uuid:    "baz",
			model:   &testModel{Foo: "bar"},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
				{
					index: &testModel{Foo: "bar"},
					uuids: newUUIDSet("baz"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err = rc.Create(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, rc.indexes[index], len(tt.expected))
				for _, expected := range tt.expected {
					mapperInfo, err := dbModel.NewModelInfo(expected.index)
					require.NoError(t, err)
					h, err := valueFromIndex(mapperInfo, db.Indexes("Open_vSwitch")[0].Columns)
					require.NoError(t, err)
					require.Equal(t, expected.uuids, rc.indexes[index][h], expected.index)
				}
			}
		})
	}
}

func TestRowCacheUpdate(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar":    &testModel{Foo: "bar"},
			"foobar": &testModel{Foo: "foobar"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	tests := []struct {
		name       string
		uuid       string
		model      *testModel
		checkIndex bool
		wantErr    bool
	}{
		{
			"error if row does not exist",
			"foo",
			&testModel{Foo: "foo"},
			true,
			true,
		},
		{
			"update",
			"bar",
			&testModel{Foo: "baz"},
			true,
			false,
		},
		{
			"error new index would cause duplicate",
			"bar",
			&testModel{Foo: "foobar"},
			true,
			true,
		},
		{
			"no error new index would cause duplicate",
			"bar",
			&testModel{Foo: "foobar"},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			_, err := rc.Update(tt.uuid, tt.model, tt.checkIndex)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, rc.indexes["foo"][tt.model.Foo], 1)
				assert.Equal(t, tt.uuid, rc.indexes["foo"][tt.model.Foo].getAny())
			}
		})
	}
}

func TestRowCacheUpdateClientIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
				},
			},
		},
	})
	err = json.Unmarshal(getTestSchema(""), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Foo: "foo", Bar: "foo"},
			"bar":    &testModel{Foo: "bar", Bar: "bar"},
			"foobar": &testModel{Foo: "bar", Bar: "foobar"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected valueToUUIDs
	}{
		{
			name:    "error if row does not exist",
			uuid:    "baz",
			model:   &testModel{Foo: "baz"},
			wantErr: true,
		},
		{
			name:    "update non-index",
			uuid:    "foo",
			model:   &testModel{Foo: "foo", Bar: "bar"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("foo"),
				"bar": newUUIDSet("bar", "foobar"),
			},
		},
		{
			name:    "update unique index to new index",
			uuid:    "foo",
			model:   &testModel{Foo: "baz"},
			wantErr: false,
			expected: valueToUUIDs{
				"baz": newUUIDSet("foo"),
				"bar": newUUIDSet("bar", "foobar"),
			},
		},
		{
			name:    "update unique index to existing index",
			uuid:    "foo",
			model:   &testModel{Foo: "bar"},
			wantErr: false,
			expected: valueToUUIDs{
				"bar": newUUIDSet("foo", "bar", "foobar"),
			},
		},
		{
			name:    "update multi index to different index",
			uuid:    "foobar",
			model:   &testModel{Foo: "foo"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("foo", "foobar"),
				"bar": newUUIDSet("bar"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			_, err = rc.Update(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, rc.indexes["foo"])
			}
		})
	}
}

func TestRowCacheUpdateOptionalColumnClientIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "datapath",
					},
				},
			},
		},
	})
	err = json.Unmarshal(getTestSchema(""), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Datapath: getStringPtr("foo"), Bar: "foo"},
			"bar":    &testModel{Datapath: getStringPtr("bar"), Bar: "bar"},
			"foobar": &testModel{Datapath: getStringPtr("bar"), Bar: "foobar"},
			"nil":    &testModel{Datapath: nilString, Bar: "nil"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected valueToUUIDs
	}{
		{
			name:    "error if row does not exist",
			uuid:    "baz",
			model:   &testModel{Datapath: getStringPtr("baz")},
			wantErr: true,
		},
		{
			name:    "update non-index",
			uuid:    "foo",
			model:   &testModel{Datapath: getStringPtr("foo"), Bar: "bar"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo":     newUUIDSet("foo"),
				"bar":     newUUIDSet("bar", "foobar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "update unique index to new index",
			uuid:    "foo",
			model:   &testModel{Datapath: getStringPtr("baz")},
			wantErr: false,
			expected: valueToUUIDs{
				"baz":     newUUIDSet("foo"),
				"bar":     newUUIDSet("bar", "foobar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "update unique index to existing index",
			uuid:    "foo",
			model:   &testModel{Datapath: getStringPtr("bar")},
			wantErr: false,
			expected: valueToUUIDs{
				"bar":     newUUIDSet("foo", "bar", "foobar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "update multi index to different index",
			uuid:    "foobar",
			model:   &testModel{Datapath: getStringPtr("foo")},
			wantErr: false,
			expected: valueToUUIDs{
				"foo":     newUUIDSet("foo", "foobar"),
				"bar":     newUUIDSet("bar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "update nil index to new index",
			uuid:    "nil",
			model:   &testModel{Datapath: getStringPtr("foo")},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("nil", "foo"),
				"bar": newUUIDSet("bar", "foobar"),
			},
		},
		{
			name:    "update multi index to nil index",
			uuid:    "foobar",
			model:   &testModel{Datapath: nilString},
			wantErr: false,
			expected: valueToUUIDs{
				"foo":     newUUIDSet("foo"),
				"bar":     newUUIDSet("bar"),
				nilString: newUUIDSet("nil", "foobar"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			_, err = rc.Update(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, rc.indexes["datapath"])
			}
		})
	}
}

func TestRowCacheUpdateMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo", "bar", "datapath"]`), &schema)
	require.NoError(t, err)
	index := newIndexFromColumns("foo", "bar", "datapath")
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar":    &testModel{Foo: "bar", Bar: "bar"},
			"foobar": &testModel{Foo: "foobar", Bar: "foobar"},
			"baz":    &testModel{Foo: "blah", Bar: "blah"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	assert.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)
	fakeDatapath := "fakePath"
	tests := []struct {
		name    string
		uuid    string
		model   *testModel
		wantErr bool
	}{
		{
			"error if row does not exist",
			"foo",
			&testModel{Foo: "foo", Bar: "foo"},
			true,
		},
		{
			"update both index cols",
			"bar",
			&testModel{Foo: "baz", Bar: "baz"},
			false,
		},
		{
			"update single index col",
			"bar",
			&testModel{Foo: "baz", Bar: "quux"},
			false,
		},
		{
			"error updating index would cause duplicate, even with nil pointer index value",
			"baz",
			&testModel{Foo: "foobar", Bar: "foobar"},
			true,
		},
		{
			"update from nil ptr value to non-nil value for index",
			"baz",
			&testModel{Foo: "blah", Bar: "blah", Datapath: &fakeDatapath},
			false,
		},
		{
			"updating overlapping keys with different pointer index value causes no error",
			"baz",
			&testModel{Foo: "foobar", Bar: "foobar", Datapath: &fakeDatapath},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			_, err := rc.Update(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				mapperInfo, err := dbModel.NewModelInfo(tt.model)
				require.NoError(t, err)
				h, err := valueFromIndex(mapperInfo, newColumnKeysFromColumns("foo", "bar", "datapath"))
				require.NoError(t, err)
				assert.Len(t, rc.indexes[index][h], 1)
				assert.Equal(t, tt.uuid, rc.indexes[index][h].getAny())
			}
		})
	}
}

func TestRowCacheUpdateMultiClientIndex(t *testing.T) {
	type testModel struct {
		UUID string            `ovsdb:"_uuid"`
		Foo  string            `ovsdb:"foo"`
		Bar  map[string]string `ovsdb:"bar"`
		Baz  string            `ovsdb:"baz"`
	}
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)

	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
					{
						Column: "bar",
						Key:    "bar",
					},
				},
			},
		},
	})
	index := newIndexFromColumnKeys(db.Indexes("Open_vSwitch")[0].Columns...)

	err = json.Unmarshal([]byte(`{
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
		    "columns": {
		      "foo": {
			    "type": "string"
			  },
			  "bar": {
				"type": {
					"key": "string",
					"value": "string",
					"min": 0, 
					"max": "unlimited"
				}
			  },
			  "baz": {
			    "type": "string"
			  }
		    }
		  }
		}
	}`), &schema)
	require.NoError(t, err)

	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
			"bar":    &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
			"foobar": &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	type expected struct {
		index model.Model
		uuids uuidset
	}

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected []expected
	}{
		{
			name:    "error if row does not exist",
			uuid:    "baz",
			model:   &testModel{Foo: "baz", Bar: map[string]string{"bar": "baz"}},
			wantErr: true,
		},
		{
			name:  "update non-index",
			uuid:  "foo",
			model: &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}, Baz: "bar"},
			expected: []expected{
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("foo"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "foobar"),
				},
			},
		},
		{
			name:    "update one index column",
			uuid:    "foo",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "baz"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "baz"}},
					uuids: newUUIDSet("foo"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "foobar"),
				},
			},
		},
		{
			name:    "update other index column",
			uuid:    "foo",
			model:   &testModel{Foo: "baz", Bar: map[string]string{"bar": "foo"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "baz", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("foo"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "foobar"),
				},
			},
		},
		{
			name:    "update both index columns",
			uuid:    "foo",
			model:   &testModel{Foo: "baz", Bar: map[string]string{"bar": "baz"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "baz", Bar: map[string]string{"bar": "baz"}},
					uuids: newUUIDSet("foo"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "foobar"),
				},
			},
		},
		{
			name:    "update unique index to existing index",
			uuid:    "foo",
			model:   &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("foo", "bar", "foobar"),
				},
			},
		},
		{
			name:    "update multi index to different index",
			uuid:    "foobar",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("foo", "foobar"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			_, err = rc.Update(tt.uuid, tt.model, true)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, rc.indexes[index], len(tt.expected))
				for _, expectedUUID := range tt.expected {
					mapperInfo, err := dbModel.NewModelInfo(expectedUUID.index)
					require.NoError(t, err)
					h, err := valueFromIndex(mapperInfo, db.Indexes("Open_vSwitch")[0].Columns)
					require.NoError(t, err)
					require.Equal(t, expectedUUID.uuids, rc.indexes[index][h], expectedUUID.index)
				}
			}
		})
	}
}

func TestRowCacheDelete(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"bar": &testModel{Foo: "bar"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		uuid    string
		model   *testModel
		wantErr bool
	}{
		{
			"deletes a row",
			"bar",
			&testModel{Foo: "bar"},
			false,
		},
		{
			"error if row does not exist",
			"foobar",
			&testModel{Foo: "bar"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err := rc.Delete(tt.uuid)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Nil(t, rc.indexes["foo"][tt.model.Foo])
			}
		})
	}
}

func TestRowCacheDeleteClientIndex(t *testing.T) {
	type testModel struct {
		UUID string            `ovsdb:"_uuid"`
		Foo  string            `ovsdb:"foo"`
		Bar  map[string]string `ovsdb:"bar"`
	}
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)

	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
					{
						Column: "bar",
						Key:    "bar",
					},
				},
			},
		},
	})
	index := newIndexFromColumnKeys(db.Indexes("Open_vSwitch")[0].Columns...)

	err = json.Unmarshal([]byte(`{
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
		    "columns": {
		      "foo": {
			    "type": "string"
			  },
			  "bar": {
				"type": {
					"key": "string",
					"value": "string",
					"min": 0, 
					"max": "unlimited"
				}
			  }
		    }
		  }
		}
	}`), &schema)
	require.NoError(t, err)

	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
			"bar":    &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
			"foobar": &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	type expected struct {
		index model.Model
		uuids uuidset
	}

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected []expected
	}{
		{
			name:    "error if row does not exist",
			uuid:    "baz",
			model:   &testModel{Foo: "baz", Bar: map[string]string{"bar": "baz"}},
			wantErr: true,
		},
		{
			name:    "delete a row with unique index",
			uuid:    "foo",
			model:   &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar", "foobar"),
				},
			},
		},
		{
			name:    "delete a row with duplicated index",
			uuid:    "foobar",
			model:   &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
			wantErr: false,
			expected: []expected{
				{
					index: &testModel{Foo: "foo", Bar: map[string]string{"bar": "foo"}},
					uuids: newUUIDSet("foo"),
				},
				{
					index: &testModel{Foo: "bar", Bar: map[string]string{"bar": "bar"}},
					uuids: newUUIDSet("bar"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err = rc.Delete(tt.uuid)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, rc.indexes[index], len(tt.expected))
				for _, expected := range tt.expected {
					mapperInfo, err := dbModel.NewModelInfo(expected.index)
					require.NoError(t, err)
					h, err := valueFromIndex(mapperInfo, db.Indexes("Open_vSwitch")[0].Columns)
					require.NoError(t, err)
					require.Equal(t, expected.uuids, rc.indexes[index][h], expected.index)
				}
			}
		})
	}
}

func TestRowCacheDeleteOptionalColumnClientIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)

	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "datapath",
					},
				},
			},
		},
	})
	err = json.Unmarshal(getTestSchema(""), &schema)
	require.NoError(t, err)

	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Datapath: getStringPtr("foo"), Bar: "foo"},
			"bar":    &testModel{Datapath: getStringPtr("bar"), Bar: "bar"},
			"foobar": &testModel{Datapath: getStringPtr("bar"), Bar: "foobar"},
			"nil":    &testModel{Datapath: nilString, Bar: "nil"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name     string
		uuid     string
		model    *testModel
		wantErr  bool
		expected valueToUUIDs
	}{
		{
			name:    "error if row does not exist",
			uuid:    "baz",
			model:   &testModel{Datapath: getStringPtr("baz")},
			wantErr: true,
		},
		{
			name:    "delete a row with unique index",
			uuid:    "foo",
			model:   &testModel{Datapath: getStringPtr("foo"), Bar: "foo"},
			wantErr: false,
			expected: valueToUUIDs{
				"bar":     newUUIDSet("bar", "foobar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "delete a row with duplicated index",
			uuid:    "bar",
			model:   &testModel{Datapath: getStringPtr("bar"), Bar: "bar"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo":     newUUIDSet("foo"),
				"bar":     newUUIDSet("foobar"),
				nilString: newUUIDSet("nil"),
			},
		},
		{
			name:    "delete a row with nil index",
			uuid:    "nil",
			model:   &testModel{Datapath: nilString, Bar: "nil"},
			wantErr: false,
			expected: valueToUUIDs{
				"foo": newUUIDSet("foo"),
				"bar": newUUIDSet("bar", "foobar"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rc := tc.Table("Open_vSwitch")
			require.NotNil(t, rc)
			err = rc.Delete(tt.uuid)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, rc.indexes["datapath"])
			}
		})
	}
}

func TestEventHandlerFuncs_OnAdd(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, newModel model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		row   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}},
		},
		{
			"calls onadd function",
			fields{func(string, model.Model) {
				calls++
			}, nil, nil},
			args{"testTable", &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnAdd(tt.args.table, tt.args.row)
			if e.AddFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestEventHandlerFuncs_OnUpdate(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, newModel model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		old   model.Model
		new   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}, &testModel{}},
		},
		{
			"calls onupdate function",
			fields{nil, func(string, model.Model, model.Model) {
				calls++
			}, nil},
			args{"testTable", &testModel{}, &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnUpdate(tt.args.table, tt.args.old, tt.args.new)
			if e.UpdateFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestEventHandlerFuncs_OnDelete(t *testing.T) {
	calls := 0
	type fields struct {
		AddFunc    func(table string, row model.Model)
		UpdateFunc func(table string, old model.Model, newModel model.Model)
		DeleteFunc func(table string, row model.Model)
	}
	type args struct {
		table string
		row   model.Model
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"doesn't call nil function",
			fields{nil, nil, nil},
			args{"testTable", &testModel{}},
		},
		{
			"calls ondelete function",
			fields{nil, nil, func(string, model.Model) {
				calls++
			}},
			args{"testTable", &testModel{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventHandlerFuncs{
				AddFunc:    tt.fields.AddFunc,
				UpdateFunc: tt.fields.UpdateFunc,
				DeleteFunc: tt.fields.DeleteFunc,
			}
			e.OnDelete(tt.args.table, tt.args.row)
			if e.DeleteFunc != nil {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestTableCacheTable(t *testing.T) {
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	rc, err := newRowCache("Open_vSwitch", dbModel, nil)
	require.NoError(t, err)
	tests := []struct {
		name  string
		cache map[string]*RowCache
		table string
		want  *RowCache
	}{
		{
			"returns nil for an empty table",
			map[string]*RowCache{"Open_vSwitch": rc},
			"foo",
			nil,
		},
		{
			"returns valid row cache for valid table",
			map[string]*RowCache{"Open_vSwitch": rc},
			"Open_vSwitch",
			rc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TableCache{
				cache: tt.cache,
			}
			got := tr.Table(tt.table)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTableCacheTables(t *testing.T) {
	db, err := model.NewClientDBModel("TestDB",
		map[string]model.Model{
			"test1": &testModel{},
			"test2": &testModel{},
			"test3": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(`
		 {"name": "TestDB",
		  "tables": {
		    "test1": {
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
			  "type": "string"
			},
			"baz": {
			  "type": "integer"
			},
			"array": {
			  "type": {
			    "key": {
			      "type": "string"
			    },
			    "min": 0,
			    "max": "unlimited"
			  }
            },
            "datapath": {
              "type": {
                "key": {
                  "type": "string"
                },
                "min": 0,
                "max": 1
              }
            }
		      }
		    },
		    "test2": {
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
			  "type": "string"
			},
			"baz": {
			  "type": "integer"
			},
			"array": {
			  "type": {
			    "key": {
			      "type": "string"
			    },
			    "min": 0,
			    "max": "unlimited"
			  }
            },
            "datapath": {
              "type": {
                "key": {
                  "type": "string"
                },
                "min": 0,
                "max": 1
              }
            }
		      }
		    },
		    "test3": {
		      "columns": {
		        "foo": {
			  "type": "string"
			},
			"bar": {
			  "type": "string"
			},
			"baz": {
			  "type": "integer"
			},
			"array": {
			  "type": {
			    "key": {
			      "type": "string"
			    },
			    "min": 0,
			    "max": "unlimited"
			  }
            },
            "datapath": {
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
	     }
	`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	rc1, err := newRowCache("test1", dbModel, nil)
	require.NoError(t, err)
	rc2, err := newRowCache("test2", dbModel, nil)
	require.NoError(t, err)
	rc3, err := newRowCache("test3", dbModel, nil)
	require.NoError(t, err)
	tests := []struct {
		name  string
		cache map[string]*RowCache
		want  []string
	}{
		{
			"returns a table that exists",
			map[string]*RowCache{
				"test1": rc1,
				"test2": rc2,
				"test3": rc3,
			},
			[]string{"test1", "test2", "test3"},
		},
		{
			"returns an empty slice if no tables exist",
			map[string]*RowCache{},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &TableCache{
				cache: tt.cache,
			}
			got := tr.Tables()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestTableCache_populate(t *testing.T) {
	t.Log("Create")
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	testRow := ovsdb.Row(map[string]any{"_uuid": ovsdb.UUID{GoUUID: "test"}, "foo": "bar"})
	testRowModel := &testModel{UUID: "test", Foo: "bar"}
	updates := ovsdb.TableUpdates{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate{
				Old: nil,
				New: &testRow,
			},
		},
	}
	err = tc.Populate(updates)
	require.NoError(t, err)

	got := tc.Table("Open_vSwitch").Row("test")
	assert.Equal(t, testRowModel, got)

	t.Log("Update")
	updatedRow := ovsdb.Row(map[string]any{"_uuid": ovsdb.UUID{GoUUID: "test"}, "foo": "quux"})
	updatedRowModel := &testModel{UUID: "test", Foo: "quux"}
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &testRow,
		New: &updatedRow,
	}
	err = tc.Populate(updates)
	require.NoError(t, err)

	got = tc.cache["Open_vSwitch"].cache["test"]
	assert.Equal(t, updatedRowModel, got)

	t.Log("Delete")
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &updatedRow,
		New: nil,
	}

	err = tc.Populate(updates)
	require.NoError(t, err)

	_, ok := tc.cache["Open_vSwitch"].cache["test"]
	assert.False(t, ok)
}

func TestTableCachePopulate(t *testing.T) {
	t.Log("Create")
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	testRow := ovsdb.Row(map[string]any{"_uuid": ovsdb.UUID{GoUUID: "test"}, "foo": "bar"})
	testRowModel := &testModel{UUID: "test", Foo: "bar"}
	updates := ovsdb.TableUpdates{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate{
				Old: nil,
				New: &testRow,
			},
		},
	}
	err = tc.Populate(updates)
	require.NoError(t, err)

	got := tc.Table("Open_vSwitch").Row("test")
	assert.Equal(t, testRowModel, got)

	t.Log("Update")
	updatedRow := ovsdb.Row(map[string]any{"_uuid": ovsdb.UUID{GoUUID: "test"}, "foo": "quux"})
	updatedRowModel := &testModel{UUID: "test", Foo: "quux"}
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &testRow,
		New: &updatedRow,
	}
	err = tc.Populate(updates)
	require.NoError(t, err)

	got = tc.cache["Open_vSwitch"].cache["test"]
	assert.Equal(t, updatedRowModel, got)

	t.Log("Delete")
	updates["Open_vSwitch"]["test"] = &ovsdb.RowUpdate{
		Old: &updatedRow,
		New: nil,
	}

	err = tc.Populate(updates)
	require.NoError(t, err)

	_, ok := tc.cache["Open_vSwitch"].cache["test"]
	assert.False(t, ok)
}

func TestTableCachePopulate2(t *testing.T) {
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	testRow := ovsdb.Row(map[string]any{"_uuid": "test", "foo": "bar"})
	testRowModel := &testModel{UUID: "test", Foo: "bar"}
	updates := ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate2{
				Initial: &testRow,
			},
		},
	}

	t.Log("Initial")
	err = tc.Populate2(updates)
	require.NoError(t, err)
	got := tc.Table("Open_vSwitch").Row("test")
	assert.Equal(t, testRowModel, got)

	t.Log("Insert")
	testRow2 := ovsdb.Row(map[string]any{"_uuid": "test2", "foo": "bar2"})
	testRowModel2 := &testModel{UUID: "test2", Foo: "bar2"}
	updates = ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test2": &ovsdb.RowUpdate2{
				Insert: &testRow2,
			},
		},
	}
	err = tc.Populate2(updates)
	require.NoError(t, err)
	got = tc.Table("Open_vSwitch").Row("test2")
	assert.Equal(t, testRowModel2, got)

	t.Log("Update")
	updatedRow := ovsdb.Row(map[string]any{"foo": "quux"})
	updatedRowModel := &testModel{UUID: "test", Foo: "quux"}
	updates = ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate2{
				Modify: &updatedRow,
			},
		},
	}
	err = tc.Populate2(updates)
	require.NoError(t, err)
	got = tc.cache["Open_vSwitch"].cache["test"]
	assert.Equal(t, updatedRowModel, got)

	t.Log("Delete")
	deletedRow := ovsdb.Row(map[string]any{"_uuid": "test", "foo": "quux"})
	updates = ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test": &ovsdb.RowUpdate2{
				Delete: &deletedRow,
			},
		},
	}
	err = tc.Populate2(updates)
	require.NoError(t, err)
	_, ok := tc.cache["Open_vSwitch"].cache["test"]
	assert.False(t, ok)
}

// ovsdb-server can break index uniqueness inside a monitor update
// the cache needs to be able to recover from this
func TestTableCachePopulate2BrokenIndexes(t *testing.T) {
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)

	t.Log("Insert")
	testRow := ovsdb.Row(map[string]any{"_uuid": "test1", "foo": "bar"})
	testRowModel := &testModel{UUID: "test1", Foo: "bar"}
	updates := ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test1": &ovsdb.RowUpdate2{
				Insert: &testRow,
			},
		},
	}
	err = tc.Populate2(updates)
	require.NoError(t, err)
	got := tc.Table("Open_vSwitch").Row("test1")
	assert.Equal(t, testRowModel, got)

	t.Log("Insert Duplicate Index")
	testRow2 := ovsdb.Row(map[string]any{"_uuid": "test2", "foo": "bar"})
	testRowModel2 := &testModel{UUID: "test2", Foo: "bar"}
	updates = ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test2": &ovsdb.RowUpdate2{
				Insert: &testRow2,
			},
		},
	}

	err = tc.Populate2(updates)
	require.NoError(t, err)
	got = tc.Table("Open_vSwitch").Row("test2")
	assert.Equal(t, testRowModel2, got)

	t.Log("Delete")
	deletedRow := ovsdb.Row(map[string]any{"_uuid": "test1", "foo": "bar"})
	updates = ovsdb.TableUpdates2{
		"Open_vSwitch": {
			"test1": &ovsdb.RowUpdate2{
				Delete: &deletedRow,
			},
		},
	}
	err = tc.Populate2(updates)
	require.NoError(t, err)
	_, ok := tc.cache["Open_vSwitch"].cache["test1"]
	assert.False(t, ok)

	t.Log("Lookup Original Insert By Index")
	_, result, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "bar"})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEventProcessor_AddEvent(t *testing.T) {
	logger := logr.Discard()
	ep := newEventProcessor(16, &logger)
	var events []event
	for i := 0; i < 17; i++ {
		events = append(events, event{
			table:     "bridge",
			eventType: addEvent,
			new: &testModel{
				UUID: "unique",
				Foo:  "bar",
			},
		})
	}
	// overfill channel so event 16 is dropped
	for _, e := range events {
		ep.AddEvent(e.eventType, e.table, nil, e.new)
	}
	// assert channel is full of events
	assert.Len(t, ep.events, 16)

	// read events and ensure they are in FIFO order
	for i := 0; i < 16; i++ {
		event := <-ep.events
		assert.Equal(t, &testModel{UUID: "unique", Foo: "bar"}, event.new)
	}

	// assert channel is empty
	assert.Empty(t, ep.events)
}

func TestIndex(t *testing.T) {
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "bar",
					},
				},
			},
			{
				Columns: []model.ColumnKey{
					{
						Column: "foo",
					},
					{
						Column: "baz",
					},
				},
			},
		},
	})
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal(getTestSchema(`["foo"], ["bar","baz"]`), &schema)
	require.NoError(t, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	assert.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)
	table := tc.Table("Open_vSwitch")

	obj := &testModel{
		UUID: "test1",
		Foo:  "foo",
		Bar:  "bar",
		Baz:  42,
	}
	err = table.Create(obj.UUID, obj, true)
	require.NoError(t, err)

	obj2 := &testModel{
		UUID: "test2",
		Foo:  "foo2",
		Bar:  "bar",
		Baz:  78,
	}
	err = table.Create(obj2.UUID, obj2, true)
	require.NoError(t, err)

	t.Run("Index by single column", func(t *testing.T) {
		idx, err := table.Index("foo")
		require.NoError(t, err)
		info, err := dbModel.NewModelInfo(obj)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("foo"))
		require.NoError(t, err)
		assert.ElementsMatch(t, idx[v], []string{obj.UUID})
	})
	t.Run("Index by single column miss", func(t *testing.T) {
		idx, err := table.Index("foo")
		require.NoError(t, err)
		obj3 := *obj
		obj3.Foo = "wrong"
		require.NoError(t, err)
		info, err := dbModel.NewModelInfo(&obj3)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("foo"))
		require.NoError(t, err)
		_, ok := idx[v]
		assert.False(t, ok)
	})
	t.Run("Index by single column wrong", func(t *testing.T) {
		_, err := table.Index("wrong")
		require.Error(t, err)
	})
	t.Run("Index by multi-column wrong", func(t *testing.T) {
		_, err := table.Index("bar", "wrong")
		require.Error(t, err)
	})
	t.Run("Index by multi-column", func(t *testing.T) {
		idx, err := table.Index("bar", "baz")
		require.NoError(t, err)
		info, err := dbModel.NewModelInfo(obj)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("bar", "baz"))
		require.NoError(t, err)
		assert.ElementsMatch(t, idx[v], []string{obj.UUID})
	})
	t.Run("Index by multi-column miss", func(t *testing.T) {
		idx, err := table.Index("bar", "baz")
		require.NoError(t, err)
		obj3 := *obj
		obj3.Baz++
		info, err := dbModel.NewModelInfo(&obj3)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("bar", "baz"))
		require.NoError(t, err)
		_, ok := idx[v]
		assert.False(t, ok)
	})
	t.Run("Client index by single column", func(t *testing.T) {
		idx, err := table.Index("bar")
		require.NoError(t, err)
		info, err := dbModel.NewModelInfo(obj)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("bar"))
		require.NoError(t, err)
		assert.ElementsMatch(t, idx[v], []string{obj.UUID, obj2.UUID})
	})
	t.Run("Client index by multiple column", func(t *testing.T) {
		idx, err := table.Index("foo", "baz")
		require.NoError(t, err)
		info, err := dbModel.NewModelInfo(obj)
		require.NoError(t, err)
		v, err := valueFromIndex(info, newColumnKeysFromColumns("foo", "baz"))
		require.NoError(t, err)
		assert.ElementsMatch(t, idx[v], []string{obj.UUID})
	})
}

func setupRowByModelSingleIndex(t require.TestingT) (*testModel, *TableCache) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo"]`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo": myFoo,
			"bar": &testModel{Foo: "bar", Bar: "bar"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	return myFoo, tc
}

func TestTableCacheRowByModelSingleIndex(t *testing.T) {
	myFoo, tc := setupRowByModelSingleIndex(t)

	t.Run("get foo by index", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		require.NoError(t, err)
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		_, baz, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz"})
		require.NoError(t, err)
		assert.Nil(t, baz)
	})

	t.Run("no index data", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Bar: "foo"})
		require.NoError(t, err)
		assert.Nil(t, foo)
	})

	t.Run("wrong model type", func(t *testing.T) {
		type badModel struct {
			UUID string `ovsdb:"_uuid"`
			Baz  string `ovsdb:"baz"`
		}
		_, _, err := tc.Table("Open_vSwitch").RowByModel(&badModel{Baz: "baz"})
		require.Error(t, err)
	})
}

func benchmarkDoCreate(b *testing.B, numRows int) (*TableCache, *RowCache) {
	_, tc := setupRowByModelSingleIndex(b)

	rc := tc.Table("Open_vSwitch")
	for i := 0; i < numRows; i++ {
		uuid := fmt.Sprintf("%d", i)
		model := &testModel{Foo: uuid}
		err := rc.Create(uuid, model, true)
		require.NoError(b, err)
	}

	return tc, rc
}

const numRows int = 10000

func BenchmarkSingleIndexCreate(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, _ = benchmarkDoCreate(b, numRows)
	}
}

func BenchmarkSingleIndexUpdate(b *testing.B) {
	_, rc := benchmarkDoCreate(b, numRows)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < numRows; i++ {
			uuid := fmt.Sprintf("%d", i)
			model := &testModel{Foo: fmt.Sprintf("%d-%d", n, i)}
			_, err := rc.Update(uuid, model, true)
			require.NoError(b, err)
		}
	}
}

func BenchmarkSingleIndexUpdateArray(b *testing.B) {
	const numRows int = 1500
	_, rc := benchmarkDoCreate(b, numRows)

	array := make([]string, 0, 500)
	for i := 0; i < cap(array); i++ {
		array = append(array, fmt.Sprintf("value%d", i))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < numRows; i++ {
			uuid := fmt.Sprintf("%d", i)
			model := &testModel{Foo: fmt.Sprintf("%d-%d", n, i), Array: array}
			_, err := rc.Update(uuid, model, true)
			require.NoError(b, err)
		}
	}
}

func BenchmarkSingleIndexDelete(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, rc := benchmarkDoCreate(b, numRows)
		for i := 0; i < numRows; i++ {
			uuid := fmt.Sprintf("%d", i)
			err := rc.Delete(uuid)
			require.NoError(b, err)
		}
	}
}

func BenchmarkIndexExists(b *testing.B) {
	_, rc := benchmarkDoCreate(b, numRows)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < numRows; i++ {
			uuid := fmt.Sprintf("%d", i)
			model := &testModel{UUID: uuid, Foo: uuid}
			err := rc.IndexExists(model)
			require.NoError(b, err)
		}
	}
}

func BenchmarkPopulate2UpdateArray(b *testing.B) {
	const numRows int = 500

	_, tc := setupRowByModelSingleIndex(b)
	rc := tc.Table("Open_vSwitch")

	array := make([]string, 0, 50)
	for i := 0; i < cap(array); i++ {
		array = append(array, fmt.Sprintf("value%d", i))
	}

	for i := 0; i < numRows; i++ {
		uuid := fmt.Sprintf("%d", i)
		model := &testModel{Foo: uuid, Array: array}
		err := rc.Create(uuid, model, true)
		require.NoError(b, err)
	}

	updateSet := make([]any, 0, cap(array)/2)
	for i := cap(array); i < cap(array)+cap(updateSet); i++ {
		updateSet = append(updateSet, fmt.Sprintf("value%d", i))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < numRows; i++ {
			updatedRow := ovsdb.Row(map[string]any{"array": ovsdb.OvsSet{GoSet: updateSet}})
			err := tc.Populate2(ovsdb.TableUpdates2{
				"Open_vSwitch": {
					"foo": &ovsdb.RowUpdate2{
						Modify: &updatedRow,
					},
				},
			})
			require.NoError(b, err)
		}
	}
}

func TestTableCacheRowByModelTwoIndexes(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo"], ["bar"]`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo": myFoo,
			"bar": &testModel{Foo: "bar", Bar: "bar"},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	t.Run("get foo by Foo index", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		require.NoError(t, err)
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get foo by Bar index", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Bar: "foo"})
		require.NoError(t, err)
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		_, baz, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz"})
		require.NoError(t, err)
		assert.Nil(t, baz)
	})

}

func TestTableCacheRowByModelMultiIndex(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	err = json.Unmarshal(getTestSchema(`["foo", "bar"]`), &schema)
	require.NoError(t, err)
	myFoo := &testModel{Foo: "foo", Bar: "foo"}
	testData := Data{
		"Open_vSwitch": map[string]model.Model{"foo": myFoo, "bar": &testModel{Foo: "bar", Bar: "bar"}},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, testData, nil)
	require.NoError(t, err)

	t.Run("incomplete index", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo"})
		require.NoError(t, err)
		assert.Nil(t, foo)
	})

	t.Run("get foo by index", func(t *testing.T) {
		_, foo, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "foo", Bar: "foo"})
		require.NoError(t, err)
		assert.NotNil(t, foo)
		assert.Equal(t, myFoo, foo)
	})

	t.Run("get non-existent item by index", func(t *testing.T) {
		_, baz, err := tc.Table("Open_vSwitch").RowByModel(&testModel{Foo: "baz", Bar: "baz"})
		require.NoError(t, err)
		assert.Nil(t, baz)
	})
}

func TestTableCacheRowsByModels(t *testing.T) {
	type testModel struct {
		UUID string            `ovsdb:"_uuid"`
		Foo  string            `ovsdb:"foo"`
		Bar  string            `ovsdb:"bar"`
		Baz  map[string]string `ovsdb:"baz"`
	}
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testModel{}})
	require.NoError(t, err)
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "bar",
					},
				},
			},
			{
				Columns: []model.ColumnKey{
					{
						Column: "bar",
					},
					{
						Column: "baz",
						Key:    "baz",
					},
				},
			},
		},
	})
	err = json.Unmarshal([]byte(`{
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
			"indexes": [["foo"]],
			"columns": {	
			  "foo": {
				"type": "string"
			  },
			  "bar": {
				"type": "string"
			  },
			  "baz": {
				"type": {
					"key": "string",
					"value": "string",
					"min": 0, 
					"max": "unlimited"
				}
			  }
			}
		  }
		}
	}`), &schema)
	require.NoError(t, err)

	testData := Data{
		"Open_vSwitch": map[string]model.Model{
			"foo":    &testModel{Foo: "foo", Bar: "foo", Baz: map[string]string{"baz": "foo", "other": "other"}},
			"bar":    &testModel{Foo: "bar", Bar: "bar", Baz: map[string]string{"baz": "bar", "other": "other"}},
			"foobar": &testModel{Foo: "foobar", Bar: "bar", Baz: map[string]string{"baz": "foobar", "other": "other"}},
			"baz":    &testModel{Foo: "baz", Bar: "baz", Baz: map[string]string{"baz": "baz", "other": "other"}},
			"quux":   &testModel{Foo: "quux", Bar: "quux", Baz: map[string]string{"baz": "quux", "other": "other"}},
			"quuz":   &testModel{Foo: "quuz", Bar: "quux", Baz: map[string]string{"baz": "quux", "other": "other"}},
		},
	}
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)

	tests := []struct {
		name   string
		models []model.Model
		rows   map[string]model.Model
	}{
		{
			name: "by non index, no result",
			models: []model.Model{
				&testModel{Foo: "no", Bar: "no", Baz: map[string]string{"baz": "no"}},
			},
			rows: nil,
		},
		{
			name: "by single column client index, single result",
			models: []model.Model{
				&testModel{Bar: "foo"},
			},
			rows: map[string]model.Model{
				"foo": testData["Open_vSwitch"]["foo"],
			},
		},
		{
			name: "by single column client index, multiple models, multiple results",
			models: []model.Model{
				&testModel{Bar: "foo"},
				&testModel{Bar: "baz"},
			},
			rows: map[string]model.Model{
				"foo": testData["Open_vSwitch"]["foo"],
				"baz": testData["Open_vSwitch"]["baz"],
			},
		},
		{
			name: "by single column client index, multiple results",
			models: []model.Model{
				&testModel{Bar: "bar"},
			},
			rows: map[string]model.Model{
				"bar":    testData["Open_vSwitch"]["bar"],
				"foobar": testData["Open_vSwitch"]["foobar"],
			},
		},
		{
			name: "by multi column client index, single result",
			models: []model.Model{
				&testModel{Bar: "baz", Baz: map[string]string{"baz": "baz"}},
			},
			rows: map[string]model.Model{
				"baz": testData["Open_vSwitch"]["baz"],
			},
		},
		{
			name: "by client index, multiple results",
			models: []model.Model{
				&testModel{Bar: "quux", Baz: map[string]string{"baz": "quux"}},
			},
			rows: map[string]model.Model{
				"quux": testData["Open_vSwitch"]["quux"],
				"quuz": testData["Open_vSwitch"]["quuz"],
			},
		},
		{
			name: "by client index, multiple models, multiple results",
			models: []model.Model{
				&testModel{Bar: "quux", Baz: map[string]string{"baz": "quux"}},
				&testModel{Bar: "bar", Baz: map[string]string{"baz": "foobar"}},
			},
			rows: map[string]model.Model{
				"quux":   testData["Open_vSwitch"]["quux"],
				"quuz":   testData["Open_vSwitch"]["quuz"],
				"foobar": testData["Open_vSwitch"]["foobar"],
				"bar":    testData["Open_vSwitch"]["bar"],
			},
		},
		{
			name: "by schema index prioritized over client index",
			models: []model.Model{
				&testModel{Foo: "foo", Bar: "bar", Baz: map[string]string{"baz": "bar"}},
			},
			rows: map[string]model.Model{
				"foo": testData["Open_vSwitch"]["foo"],
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, testData, nil)
			require.NoError(t, err)
			rows, err := tc.Table("Open_vSwitch").RowsByModels(tt.models)
			require.NoError(t, err)
			require.Equal(t, tt.rows, rows)
		})
	}
}

type rowsByConditionTestModel struct {
	UUID   string            `ovsdb:"_uuid"`
	Foo    string            `ovsdb:"foo"`
	Bar    string            `ovsdb:"bar"`
	Baz    string            `ovsdb:"baz"`
	Quux   string            `ovsdb:"quux"`
	Quuz   string            `ovsdb:"quuz"`
	FooBar map[string]string `ovsdb:"foobar"`
	Empty  string            `ovsdb:"empty"`
}

func setupRowsByConditionCache(t require.TestingT) *TableCache {
	var schema ovsdb.DatabaseSchema
	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &rowsByConditionTestModel{}})
	require.NoError(t, err)
	db.SetIndexes(map[string][]model.ClientIndex{
		"Open_vSwitch": {
			{
				Columns: []model.ColumnKey{
					{
						Column: "foobar",
						Key:    "foobar",
					},
				},
			},
			{
				Columns: []model.ColumnKey{
					{
						Column: "empty",
					},
				},
			},
		},
	})
	err = json.Unmarshal([]byte(`{
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
			"indexes": [["foo"], ["bar"], ["quux", "quuz"]],
			"columns": {	
			  "foo": {
				"type": "string"
			  },
			  "bar": {
				"type": "string"
			  },
			  "baz": {
				"type": "string"
			  },
			  "quux": {
			    "type": "string"
			  },
			  "quuz": {
			    "type": "string"
			  },
			  "foobar": {
				"type": {
					"key": "string",
					"value": "string",
					"min": 0, 
					"max": "unlimited"
				}
			  },
			  "empty": {
			    "type": "string"
			  }
			}
		  }
		}
	}`), &schema)
	require.NoError(t, err)

	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(t, errs)
	tc, err := NewTableCache(dbModel, nil, nil)
	require.NoError(t, err)
	return tc
}

func TestTableCacheRowsByCondition(t *testing.T) {
	testData := map[string]*rowsByConditionTestModel{
		"foo":  {UUID: "foo", Foo: "foo", Bar: "foo", Baz: "foo", Quux: "foo", Quuz: "quuz", FooBar: map[string]string{"foobar": "foo"}},
		"bar":  {UUID: "bar", Foo: "bar", Bar: "bar", Baz: "bar", Quux: "bar", Quuz: "quuz", FooBar: map[string]string{"foobar": "bar"}},
		"baz":  {UUID: "baz", Foo: "baz", Bar: "baz", Baz: "baz", Quux: "baz", Quuz: "quuz", FooBar: map[string]string{"foobar": "baz"}},
		"quux": {UUID: "quux", Foo: "quux", Bar: "quux", Baz: "quux", Quux: "quux", Quuz: "quuz", FooBar: map[string]string{"foobar": "baz"}},
		"quuz": {UUID: "quuz", Foo: "quuz", Bar: "quuz", Baz: "quuz", Quux: "quuz", Quuz: "quuz", FooBar: map[string]string{"foobar": "baz"}},
	}

	tests := []struct {
		name       string
		conditions []ovsdb.Condition
		// uuids that could be found evaluating conditions as indexes
		uuidsByConditionsAsIndexes uuidset
		// rows that could be found evaluating all conditions
		rowsByCondition map[string]model.Model
	}{
		{
			"by equal uuid",
			[]ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "foo"}}},
			nil,
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by includes uuid",
			[]ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionIncludes, Value: ovsdb.UUID{GoUUID: "foo"}}},
			nil,
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by non equal uuid, multiple results",
			[]ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionNotEqual, Value: ovsdb.UUID{GoUUID: "foo"}}},
			nil,
			map[string]model.Model{
				"bar":  testData["bar"],
				"baz":  testData["baz"],
				"quux": testData["quux"],
				"quuz": testData["quuz"],
			},
		},
		{
			"by excludes uuid, multiple results",
			[]ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionExcludes, Value: ovsdb.UUID{GoUUID: "foo"}}},
			nil,
			map[string]model.Model{
				"bar":  testData["bar"],
				"baz":  testData["baz"],
				"quux": testData["quux"],
				"quuz": testData["quuz"],
			},
		},
		{
			"by schema index",
			[]ovsdb.Condition{{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foo"}},
			newUUIDSet("foo"),
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by schema index, no results",
			[]ovsdb.Condition{{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foobar"}},
			newUUIDSet(),
			map[string]model.Model{},
		},
		{
			"by multi column schema index",
			[]ovsdb.Condition{
				{Column: "quux", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "quuz", Function: ovsdb.ConditionEqual, Value: "quuz"},
			},
			newUUIDSet("foo"),
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by multi column schema index, no results",
			[]ovsdb.Condition{
				{Column: "quux", Function: ovsdb.ConditionEqual, Value: "foobar"},
				{Column: "quuz", Function: ovsdb.ConditionEqual, Value: "quuz"},
			},
			newUUIDSet(),
			map[string]model.Model{},
		},
		{
			"by client index",
			[]ovsdb.Condition{{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": "bar"}}}},
			newUUIDSet("bar"),
			map[string]model.Model{"bar": testData["bar"]},
		},
		{
			"by client index, no results",
			[]ovsdb.Condition{{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": "foobar"}}}},
			newUUIDSet(),
			map[string]model.Model{},
		},
		{
			"by client index, multiple results",
			[]ovsdb.Condition{{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": "baz"}}}},
			newUUIDSet("baz", "quux", "quuz"),
			map[string]model.Model{
				"baz":  testData["baz"],
				"quux": testData["quux"],
				"quuz": testData["quuz"],
			},
		},
		{
			"by zero client index, multiple results",
			[]ovsdb.Condition{{Column: "empty", Function: ovsdb.ConditionEqual, Value: ""}},
			newUUIDSet("foo", "bar", "baz", "quux", "quuz"),
			map[string]model.Model{
				"foo":  testData["foo"],
				"bar":  testData["bar"],
				"baz":  testData["baz"],
				"quux": testData["quux"],
				"quuz": testData["quuz"],
			},
		},
		{
			"by non index",
			[]ovsdb.Condition{{Column: "baz", Function: ovsdb.ConditionEqual, Value: "baz"}},
			nil,
			map[string]model.Model{"baz": testData["baz"]},
		},
		{
			"by two uuids, no results",
			[]ovsdb.Condition{
				{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "foo"}},
				{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "bar"}},
			},
			nil,
			map[string]model.Model{},
		},
		{
			"by uuid and schema index",
			[]ovsdb.Condition{
				{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "foo"}},
				{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foo"},
			},
			newUUIDSet("foo"),
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by uuid and schema index, no results",
			[]ovsdb.Condition{
				{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "foo"}},
				{Column: "foo", Function: ovsdb.ConditionEqual, Value: "bar"},
			},
			newUUIDSet("bar"),
			map[string]model.Model{},
		},
		{
			"by schema index and non-index",
			[]ovsdb.Condition{
				{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "baz", Function: ovsdb.ConditionEqual, Value: "foo"},
			},
			newUUIDSet("foo"),
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by schema index and non-index, no results",
			[]ovsdb.Condition{
				{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "baz", Function: ovsdb.ConditionEqual, Value: "baz"},
			},
			newUUIDSet("foo"),
			map[string]model.Model{},
		},
		{
			"by uuid, schema index, and non-index",
			[]ovsdb.Condition{
				{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "foo"}},
				{Column: "foo", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "bar", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "baz", Function: ovsdb.ConditionEqual, Value: "foo"},
			},
			newUUIDSet("foo"),
			map[string]model.Model{"foo": testData["foo"]},
		},
		{
			"by client index, and non-index, multiple results",
			[]ovsdb.Condition{
				{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": "baz"}}},
				{Column: "quuz", Function: ovsdb.ConditionEqual, Value: "quuz"},
			},
			newUUIDSet("baz", "quux", "quuz"),
			map[string]model.Model{
				"baz":  testData["baz"],
				"quux": testData["quux"],
				"quuz": testData["quuz"],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := setupRowsByConditionCache(t)
			rc := tc.Table("Open_vSwitch")
			for _, m := range testData {
				err := rc.Create(m.UUID, m, true)
				require.NoError(t, err)
			}

			nativeValues := make([]any, 0, len(tt.conditions))
			for _, condition := range tt.conditions {
				cSchema := rc.dbModel.Schema.Tables["Open_vSwitch"].Column(condition.Column)
				nativeValue, err := ovsdb.OvsToNative(cSchema, condition.Value)
				require.NoError(t, err)
				nativeValues = append(nativeValues, nativeValue)
			}

			uuids, err := tc.Table("Open_vSwitch").uuidsByConditionsAsIndexes(tt.conditions, nativeValues)
			require.NoError(t, err)
			require.Equal(t, tt.uuidsByConditionsAsIndexes, uuids)

			rows, err := tc.Table("Open_vSwitch").RowsByCondition(tt.conditions)
			require.NoError(t, err)
			require.Equal(t, tt.rowsByCondition, rows)
		})
	}
}

func BenchmarkRowsByCondition(b *testing.B) {
	tc := setupRowsByConditionCache(b)
	rc := tc.Table("Open_vSwitch")

	models := []*rowsByConditionTestModel{}
	for i := 0; i < numRows; i++ {
		model := &rowsByConditionTestModel{
			UUID:   fmt.Sprintf("UUID-%d", i),
			Foo:    fmt.Sprintf("Foo-%d", i),
			Bar:    fmt.Sprintf("Bar-%d", i),
			Baz:    fmt.Sprintf("Baz-%d", i),
			Quux:   fmt.Sprintf("Quux-%d", i),
			Quuz:   fmt.Sprintf("Quuz-%d", i),
			FooBar: map[string]string{"foobar": fmt.Sprintf("FooBar-%d", i)},
		}
		err := rc.Create(model.UUID, model, true)
		require.NoError(b, err)
		models = append(models, model)
	}

	r := rand.New(rand.NewSource(int64(b.N)))

	benchmarks := []struct {
		name    string
		prepare func(int) []ovsdb.Condition
	}{
		{
			name: "by uuid",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: models[i].UUID}},
				}
			},
		},
		{
			name: "by single column squema index",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "foo", Function: ovsdb.ConditionEqual, Value: models[i].Foo},
				}
			},
		},
		{
			name: "by single column client index",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": models[i].FooBar["foobar"]}}},
				}
			},
		},
		{
			name: "by multi column squema index",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "quux", Function: ovsdb.ConditionEqual, Value: models[i].Quux},
					{Column: "quuz", Function: ovsdb.ConditionEqual, Value: models[i].Quuz},
				}
			},
		},
		{
			name: "by two squema indexes",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "foo", Function: ovsdb.ConditionEqual, Value: models[i].Foo},
					{Column: "bar", Function: ovsdb.ConditionEqual, Value: models[i].Bar},
				}
			},
		},
		{
			name: "by squema index and non-index",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "foo", Function: ovsdb.ConditionEqual, Value: models[i].Foo},
					{Column: "quuz", Function: ovsdb.ConditionEqual, Value: models[i].Quuz},
				}
			},
		},
		{
			name: "by single non index",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "quuz", Function: ovsdb.ConditionEqual, Value: models[i].Quuz},
				}
			},
		},
		{
			name: "by multiple non indexes",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "baz", Function: ovsdb.ConditionEqual, Value: models[i].Baz},
					{Column: "quuz", Function: ovsdb.ConditionEqual, Value: models[i].Quuz},
				}
			},
		},
		{
			name: "by many conditions",
			prepare: func(i int) []ovsdb.Condition {
				return []ovsdb.Condition{
					{Column: "foo", Function: ovsdb.ConditionEqual, Value: models[i].Foo},
					{Column: "bar", Function: ovsdb.ConditionEqual, Value: models[i].Bar},
					{Column: "baz", Function: ovsdb.ConditionEqual, Value: models[i].Baz},
					{Column: "quux", Function: ovsdb.ConditionEqual, Value: models[i].Quux},
					{Column: "quuz", Function: ovsdb.ConditionEqual, Value: models[i].Quuz},
					{Column: "foobar", Function: ovsdb.ConditionIncludes, Value: ovsdb.OvsMap{GoMap: map[any]any{"foobar": models[i].FooBar["foobar"]}}},
				}
			},
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				results, err := rc.RowsByCondition(bm.prepare(r.Intn(numRows)))
				require.NoError(b, err)
				require.Len(b, results, 1)
			}
		})
	}
}

func BenchmarkPopulate2SingleModify(b *testing.B) {
	type testDBModel struct {
		UUID string   `ovsdb:"_uuid"`
		Set  []string `ovsdb:"set"`
	}
	aFooSet, _ := ovsdb.NewOvsSet([]string{"foo"})
	base := &testDBModel{Set: []string{}}
	for i := 0; i < 57000; i++ {
		base.Set = append(base.Set, fmt.Sprintf("foo%d", i))
	}

	db, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{"Open_vSwitch": &testDBModel{}})
	require.NoError(b, err)
	var schema ovsdb.DatabaseSchema
	err = json.Unmarshal([]byte(`
	  {
		"name": "Open_vSwitch",
		"tables": {
		  "Open_vSwitch": {
			"columns": {
			  "set": { "type": { "key": { "type": "string" }, "min": 0, "max": "unlimited" } }
			}
		  }
		}
	  }
	`), &schema)
	require.NoError(b, err)
	dbModel, errs := model.NewDatabaseModel(schema, db)
	require.Empty(b, errs)
	caches := make([]*TableCache, b.N)
	for n := 0; n < b.N; n++ {
		tc, err := NewTableCache(dbModel, nil, nil)
		require.NoError(b, err)
		caches[n] = tc
		rc := tc.Table("Open_vSwitch")
		err = rc.Create("uuid", base, true)
		require.NoError(b, err)
	}
	tu := ovsdb.TableUpdates2{
		"Open_vSwitch": ovsdb.TableUpdate2{
			"uuid": &ovsdb.RowUpdate2{
				Modify: &ovsdb.Row{"set": aFooSet},
			},
		},
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = caches[n].Populate2(tu)
		require.NoError(b, err)
	}
}

func TestTableCache_ApplyModelUpdates(t *testing.T) {
	dbModel, err := test.GetModel()
	require.NoError(t, err)

	tests := []struct {
		name     string
		update   ovsdb.RowUpdate
		current  model.Model
		expected model.Model
	}{
		{
			name: "create",
			update: ovsdb.RowUpdate{
				New: &ovsdb.Row{"name": "bridge"},
			},
			expected: &test.BridgeType{
				UUID: "uuid",
				Name: "bridge",
			},
		},
		{
			name: "update",
			update: ovsdb.RowUpdate{
				Old: &ovsdb.Row{"name": "bridge", "datapath_type": "old"},
				New: &ovsdb.Row{"name": "bridge", "datapath_type": "new"},
			},
			current: &test.BridgeType{
				UUID:         "uuid",
				Name:         "bridge",
				DatapathType: "old",
			},
			expected: &test.BridgeType{
				UUID:         "uuid",
				Name:         "bridge",
				DatapathType: "new",
			},
		},
		{
			name: "update noop",
			update: ovsdb.RowUpdate{
				Old: &ovsdb.Row{"name": "bridge", "datapath_type": "same"},
				New: &ovsdb.Row{"name": "bridge", "datapath_type": "same"},
			},
			current: &test.BridgeType{
				UUID:         "uuid",
				Name:         "bridge",
				DatapathType: "same",
			},
			expected: &test.BridgeType{
				UUID:         "uuid",
				Name:         "bridge",
				DatapathType: "same",
			},
		},
		{
			name: "delete",
			update: ovsdb.RowUpdate{
				Old: &ovsdb.Row{"name": "bridge"},
			},
			current: &test.BridgeType{
				UUID: "uuid",
				Name: "bridge",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTableCache(dbModel, nil, nil)
			require.NoError(t, err)
			rc := tc.Table("Bridge")
			require.NotNil(t, rc)
			if tt.current != nil {
				err = rc.Create("uuid", tt.current, false)
				require.NoError(t, err)
			}
			updates := updates.ModelUpdates{}
			require.NoError(t, err)
			err = updates.AddRowUpdate(dbModel, "Bridge", "uuid", tt.current, tt.update)
			require.NoError(t, err)
			err = tc.ApplyCacheUpdate(updates)
			require.NoError(t, err)
			model := rc.rowByUUID("uuid")
			if tt.expected != nil {
				assert.Equal(t, tt.expected, model)
			} else {
				assert.Nil(t, model)
			}
		})
	}
}
