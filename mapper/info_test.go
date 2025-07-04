package mapper

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleTable = []byte(`{
      "columns": {
        "aString": {
          "type": "string"
        },
        "aInteger": {
          "type": "integer"
        },
        "aSet": {
          "type": {
            "key": "string",
            "max": "unlimited",
            "min": 0
          }
        },
        "aMap": {
          "type": {
            "key": "string",
            "value": "string"
          }
        }
    }
}`)

func TestNewMapperInfo(t *testing.T) {
	type test struct {
		name         string
		table        []byte
		obj          any
		expectedCols []string
		err          bool
	}
	tests := []test{
		{
			name:  "no_orm",
			table: sampleTable,
			obj: &struct {
				foo string
				bar int
			}{},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("NewMapper_%s", tt.name), func(t *testing.T) {
			var table ovsdb.TableSchema
			err := json.Unmarshal(tt.table, &table)
			require.NoError(t, err)

			info, err := NewInfo("Test", &table, tt.obj)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			for _, col := range tt.expectedCols {
				assert.Truef(t, info.hasColumn(col), "Expected column should be present in Mapper Info")
			}
			assert.Equal(t, "Test", info.Metadata.TableName)

		})
	}
}

func TestMapperInfoSet(t *testing.T) {
	type obj struct {
		Ostring string            `ovsdb:"aString"`
		Oint    int               `ovsdb:"aInteger"`
		Oset    []string          `ovsdb:"aSet"`
		Omap    map[string]string `ovsdb:"aMap"`
	}

	type test struct {
		name   string
		table  []byte
		obj    any
		field  any
		column string
		err    bool
	}
	tests := []test{
		{
			name:   "string",
			table:  sampleTable,
			obj:    &obj{},
			field:  "foo",
			column: "aString",
			err:    false,
		},
		{
			name:   "set",
			table:  sampleTable,
			obj:    &obj{},
			field:  []string{"foo", "bar"},
			column: "aSet",
			err:    false,
		},
		{
			name:   "map",
			table:  sampleTable,
			obj:    &obj{},
			field:  map[string]string{"foo": "bar"},
			column: "aMap",
			err:    false,
		},
		{
			name:  "nonempty",
			table: sampleTable,
			obj: &obj{
				Omap:    map[string]string{"original": "stuff"},
				Oint:    1,
				Ostring: "foo",
				Oset:    []string{"foo"},
			},
			field:  map[string]string{"foo": "bar"},
			column: "aMap",
			err:    false,
		},
		{
			name:   "un-assignable",
			table:  sampleTable,
			obj:    &obj{},
			field:  []string{"foo"},
			column: "aMap",
			err:    true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("SetField_%s", tt.name), func(t *testing.T) {
			var table ovsdb.TableSchema
			err := json.Unmarshal(tt.table, &table)
			require.NoError(t, err)

			info, err := NewInfo("Test", &table, tt.obj)
			require.NoError(t, err)

			err = info.SetField(tt.column, tt.field)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				readBack, err := info.FieldByColumn(tt.column)
				require.NoError(t, err)
				assert.Equalf(t, tt.field, readBack, "Set field should match original")
			}

		})
	}
}

func TestMapperInfoColByPtr(t *testing.T) {
	type obj struct {
		ostring string            `ovsdb:"aString"`
		oint    int               `ovsdb:"aInteger"`
		oset    []string          `ovsdb:"aSet"`
		omap    map[string]string `ovsdb:"aMap"`
	}
	obj1 := obj{}

	type test struct {
		name   string
		table  []byte
		obj    any
		field  any
		column string
		err    bool
	}
	tests := []test{
		{
			name:   "first",
			table:  sampleTable,
			obj:    &obj1,
			field:  &obj1.ostring,
			column: "aString",
			err:    false,
		},
		{
			name:   "middle",
			table:  sampleTable,
			obj:    &obj1,
			field:  &obj1.oint,
			column: "aInteger",
			err:    false,
		},
		{
			name:   "middle2",
			table:  sampleTable,
			obj:    &obj1,
			field:  &obj1.oset,
			column: "aSet",
			err:    false,
		},
		{
			name:   "last",
			table:  sampleTable,
			obj:    &obj1,
			field:  &obj1.omap,
			column: "aMap",
			err:    false,
		},
		{
			name:  "external",
			table: sampleTable,
			obj:   &obj1,
			field: &obj{},
			err:   true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("GetFieldByPtr_%s", tt.name), func(t *testing.T) {
			var table ovsdb.TableSchema
			err := json.Unmarshal(tt.table, &table)
			require.NoError(t, err)

			info, err := NewInfo("Test", &table, tt.obj)
			require.NoError(t, err)

			col, err := info.ColumnByPtr(tt.field)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equalf(t, tt.column, col, "Column name extracted should match")
			}

		})
	}
}

func TestOrmGetIndex(t *testing.T) {
	tableSchema := []byte(`{
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
        "config": {
          "type": {
            "key": "string",
            "max": "unlimited",
            "min": 0,
            "value": "string"
          }
	}
      }
   }`)
	var table ovsdb.TableSchema
	err := json.Unmarshal(tableSchema, &table)
	require.NoError(t, err)

	type obj struct {
		ID     string            `ovsdb:"_uuid"`
		MyName string            `ovsdb:"name"`
		Config map[string]string `ovsdb:"config"`
		Comp1  string            `ovsdb:"composed_1"`
		Comp2  string            `ovsdb:"composed_2"`
	}
	type test struct {
		name     string
		obj      any
		expected [][]string
		err      bool
	}
	tests := []test{
		{
			name:     "empty",
			obj:      &obj{},
			expected: [][]string{},
			err:      false,
		},
		{
			name: "UUID",
			obj: &obj{
				ID: aUUID0,
			},
			expected: [][]string{{"_uuid"}},
			err:      false,
		},
		{
			name: "simple",
			obj: &obj{
				MyName: "foo",
			},
			expected: [][]string{{"name"}},
			err:      false,
		},
		{
			name: "additional index",
			obj: &obj{
				ID:     aUUID0,
				MyName: "foo",
			},
			expected: [][]string{{"_uuid"}, {"name"}},
			err:      false,
		},
		{
			name: "complex index",
			obj: &obj{
				Comp1: "foo",
				Comp2: "bar",
			},
			expected: [][]string{{"composed_1", "composed_2"}},
			err:      false,
		},
		{
			name: "multiple index",
			obj: &obj{
				MyName: "something",
				Comp1:  "foo",
				Comp2:  "bar",
			},
			expected: [][]string{{"name"}, {"composed_1", "composed_2"}},
			err:      false,
		},
		{
			name: "all ",
			obj: &obj{
				ID:     aUUID0,
				MyName: "something",
				Comp1:  "foo",
				Comp2:  "bar",
			},
			expected: [][]string{{"_uuid"}, {"name"}, {"composed_1", "composed_2"}},
			err:      false,
		},
		{
			name: "Error: None",
			obj: &obj{
				Config: map[string]string{"foo": "bar"},
			},
			expected: [][]string{},
			err:      false,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("GetValidIndexes_%s", tt.name), func(t *testing.T) {
			info, err := NewInfo("Test", &table, tt.obj)
			require.NoError(t, err)

			indexes, err := info.getValidIndexes()
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.expected, indexes, "Indexes must match")
			}

		})
	}
}
