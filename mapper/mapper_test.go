package mapper

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	aString = "foo"
	aEnum   = "enum1"
	aSet    = []string{"a", "set", "of", "strings"}
	aUUID0  = "2f77b348-9768-4866-b761-89d5177ecda0"
	aUUID1  = "2f77b348-9768-4866-b761-89d5177ecda1"
	aUUID2  = "2f77b348-9768-4866-b761-89d5177ecda2"
	aUUID3  = "2f77b348-9768-4866-b761-89d5177ecda3"

	aUUIDSet = []string{
		aUUID0,
		aUUID1,
		aUUID2,
		aUUID3,
	}

	aIntSet = []int{
		3,
		2,
		42,
	}
	aFloat = 42.00

	aFloatSet = []float64{
		3.0,
		2.0,
		42.0,
	}

	aMap = map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
)

var testSchema = []byte(`{
  "cksum": "223619766 22548",
  "name": "TestSchema",
  "tables": {
    "TestTable": {
      "columns": {
        "aString": {
          "type": "string"
        },
        "aSet": {
          "type": {
            "key": "string",
            "max": "unlimited",
            "min": 0
          }
        },
        "aSingleSet": {
          "type": {
            "key": "string",
            "max": "unlimited",
            "min": 0,
            "max": 1
          }
        },
        "aUUIDSet": {
          "type": {
            "key": {
              "refTable": "SomeOtherTAble",
              "refType": "weak",
              "type": "uuid"
            },
            "min": 0,
            "max": "unlimited"
          }
        },
        "aUUID": {
          "type": {
            "key": {
              "refTable": "SomeOtherTAble",
              "refType": "weak",
              "type": "uuid"
            },
            "min": 1,
            "max": 1
          }
        },
        "aIntSet": {
          "type": {
            "key": {
              "type": "integer"
            },
            "min": 0,
            "max": "unlimited"
          }
        },
        "aFloat": {
          "type": {
            "key": {
              "type": "real"
            }
          }
        },
        "aFloatSet": {
          "type": {
            "key": {
              "type": "real"
            },
            "min": 0,
            "max": 10
          }
        },
        "aEmptySet": {
          "type": {
            "key": {
              "type": "string"
            },
            "min": 0,
            "max": "unlimited"
          }
        },
        "aEnum": {
          "type": {
            "key": {
              "enum": [
                "set",
                [
                  "enum1",
                  "enum2",
                  "enum3"
                ]
              ],
              "type": "string"
            }
          }
        },
        "aMap": {
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

func getOvsTestRow(t *testing.T) ovsdb.Row {
	ovsRow := ovsdb.NewRow()
	ovsRow["aString"] = aString
	ovsRow["aSet"] = testOvsSet(t, aSet)
	// Set's can hold the value if they have len == 1
	ovsRow["aSingleSet"] = aString

	us := make([]ovsdb.UUID, 0)
	for _, u := range aUUIDSet {
		us = append(us, ovsdb.UUID{GoUUID: u})
	}
	ovsRow["aUUIDSet"] = testOvsSet(t, us)

	ovsRow["aUUID"] = ovsdb.UUID{GoUUID: aUUID0}

	ovsRow["aIntSet"] = testOvsSet(t, aIntSet)

	ovsRow["aFloat"] = aFloat

	ovsRow["aFloatSet"] = testOvsSet(t, aFloatSet)

	ovsRow["aEmptySet"] = testOvsSet(t, []string{})

	ovsRow["aEnum"] = aEnum

	ovsRow["aMap"] = testOvsMap(t, aMap)

	return ovsRow
}

func TestMapperGetData(t *testing.T) {
	type ormTestType struct {
		AString             string            `ovsdb:"aString"`
		ASet                []string          `ovsdb:"aSet"`
		ASingleSet          *string           `ovsdb:"aSingleSet"`
		AUUIDSet            []string          `ovsdb:"aUUIDSet"`
		AUUID               string            `ovsdb:"aUUID"`
		AIntSet             []int             `ovsdb:"aIntSet"`
		AFloat              float64           `ovsdb:"aFloat"`
		AFloatSet           []float64         `ovsdb:"aFloatSet"`
		YetAnotherStringSet []string          `ovsdb:"aEmptySet"`
		AEnum               string            `ovsdb:"aEnum"`
		AMap                map[string]string `ovsdb:"aMap"`
		NonTagged           string
	}

	var expected = ormTestType{
		AString:             aString,
		ASet:                aSet,
		ASingleSet:          &aString,
		AUUIDSet:            aUUIDSet,
		AUUID:               aUUID0,
		AIntSet:             aIntSet,
		AFloat:              aFloat,
		AFloatSet:           aFloatSet,
		YetAnotherStringSet: []string{},
		AEnum:               aEnum,
		AMap:                aMap,
		NonTagged:           "something",
	}

	ovsRow := getOvsTestRow(t)
	/* Code under test */
	var schema ovsdb.DatabaseSchema
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Error(err)
	}

	mapper := NewMapper(schema)
	test := ormTestType{
		NonTagged: "something",
	}
	testInfo, err := NewInfo("TestTable", schema.Table("TestTable"), &test)
	require.NoError(t, err)

	err = mapper.GetRowData(&ovsRow, testInfo)
	require.NoError(t, err)
	/*End code under test*/

	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, expected, test)
}

func TestMapperGetDataWithUUID(t *testing.T) {
	type ormWithUUID struct {
		UUID    string `ovsdb:"_uuid"`
		AString string `ovsdb:"aString"`
	}
	type ormWithoutUUID struct {
		AString string `ovsdb:"aString"`
	}

	var schema ovsdb.DatabaseSchema
	err := json.Unmarshal(testSchema, &schema)
	require.NoError(t, err)
	mapper := NewMapper(schema)

	rowWithUUID := getOvsTestRow(t)
	rowWithUUID["_uuid"] = ovsdb.UUID{GoUUID: aUUID0}
	rowWithoutUUID := getOvsTestRow(t)
	delete(rowWithoutUUID, "_uuid")

	tests := []struct {
		name        string
		ovsRow      ovsdb.Row
		model       interface{}
		expected    interface{}
		shouldError bool
	}{
		{
			name:   "Model with _uuid tag and row with _uuid",
			ovsRow: rowWithUUID,
			model:  &ormWithUUID{},
			expected: &ormWithUUID{
				UUID:    aUUID0,
				AString: aString,
			},
		},
		{
			name:   "Model with _uuid tag and row without _uuid",
			ovsRow: rowWithoutUUID,
			model:  &ormWithUUID{},
			expected: &ormWithUUID{
				UUID:    "",
				AString: aString,
			},
		},
		{
			name:   "Model without _uuid tag and row with _uuid",
			ovsRow: rowWithUUID,
			model:  &ormWithoutUUID{},
			expected: &ormWithoutUUID{
				AString: aString,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testInfo, err := NewInfo("TestTable", schema.Table("TestTable"), tt.model)
			require.NoError(t, err)

			err = mapper.GetRowDataWithUUID(&tt.ovsRow, testInfo)

			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, tt.model)
			}
		})
	}
}

func TestMapperNewRow(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Error(err)
	}

	tests := []struct {
		name        string
		objInput    any
		expectedRow ovsdb.Row
		shoulderr   bool
	}{{
		name: "string",
		objInput: &struct {
			AString string `ovsdb:"aString"`
		}{
			AString: aString,
		},
		expectedRow: ovsdb.Row(map[string]any{"aString": aString}),
	}, {
		name: "set",
		objInput: &struct {
			SomeSet []string `ovsdb:"aSet"`
		}{
			SomeSet: aSet,
		},
		expectedRow: ovsdb.Row(map[string]any{"aSet": testOvsSet(t, aSet)}),
	}, {
		name: "emptySet with no column specification",
		objInput: &struct {
			EmptySet []string `ovsdb:"aSet"`
		}{
			EmptySet: []string{},
		},
		expectedRow: ovsdb.Row(map[string]any{}),
	}, {
		name: "UUID",
		objInput: &struct {
			MyUUID string `ovsdb:"aUUID"`
		}{
			MyUUID: aUUID0,
		},
		expectedRow: ovsdb.Row(map[string]any{"aUUID": ovsdb.UUID{GoUUID: aUUID0}}),
	}, {
		name: "aUUIDSet",
		objInput: &struct {
			MyUUIDSet []string `ovsdb:"aUUIDSet"`
		}{
			MyUUIDSet: []string{aUUID0, aUUID1},
		},
		expectedRow: ovsdb.Row(map[string]any{"aUUIDSet": testOvsSet(t, []ovsdb.UUID{{GoUUID: aUUID0}, {GoUUID: aUUID1}})}),
	}, {
		name: "aIntSet",
		objInput: &struct {
			MyIntSet []int `ovsdb:"aIntSet"`
		}{
			MyIntSet: []int{0, 42},
		},
		expectedRow: ovsdb.Row(map[string]any{"aIntSet": testOvsSet(t, []int{0, 42})}),
	}, {
		name: "aFloat",
		objInput: &struct {
			MyFloat float64 `ovsdb:"aFloat"`
		}{
			MyFloat: 42.42,
		},
		expectedRow: ovsdb.Row(map[string]any{"aFloat": 42.42}),
	}, {
		name: "aFloatSet",
		objInput: &struct {
			MyFloatSet []float64 `ovsdb:"aFloatSet"`
		}{
			MyFloatSet: aFloatSet,
		},
		expectedRow: ovsdb.Row(map[string]any{"aFloatSet": testOvsSet(t, aFloatSet)}),
	}, {
		name: "Enum",
		objInput: &struct {
			MyEnum string `ovsdb:"aEnum"`
		}{
			MyEnum: aEnum,
		},
		expectedRow: ovsdb.Row(map[string]any{"aEnum": aEnum}),
	}, {
		name: "untagged fields should not affect row",
		objInput: &struct {
			AString string `ovsdb:"aString"`
			MyStuff map[string]string
		}{
			AString: aString,
			MyStuff: map[string]string{"this is": "private"},
		},
		expectedRow: ovsdb.Row(map[string]any{"aString": aString}),
	}, {
		name: "Maps",
		objInput: &struct {
			MyMap map[string]string `ovsdb:"aMap"`
		}{
			MyMap: aMap,
		},
		expectedRow: ovsdb.Row(map[string]any{"aMap": testOvsMap(t, aMap)}),
	},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("NewRow: %s", test.name), func(t *testing.T) {
			mapper := NewMapper(schema)
			info, err := NewInfo("TestTable", schema.Table("TestTable"), test.objInput)
			require.NoError(t, err)
			row, err := mapper.NewRow(info)
			if test.shoulderr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equalf(t, test.expectedRow, row, "NewRow should match expected")
			}
		})
	}
}

func TestMapperNewRowFields(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Error(err)
	}

	type obj struct {
		MyMap    map[string]string `ovsdb:"aMap"`
		MySet    []string          `ovsdb:"aSet"`
		MyString string            `ovsdb:"aString"`
		MyFloat  float64           `ovsdb:"aFloat"`
	}
	testObj := obj{}

	tests := []struct {
		name        string
		prepare     func(*obj)
		expectedRow ovsdb.Row
		fields      []any
		err         bool
	}{{
		name: "string",
		prepare: func(o *obj) {
			o.MyString = aString
		},
		expectedRow: ovsdb.Row(map[string]any{"aString": aString}),
	}, {
		name: "empty string with field specification",
		prepare: func(o *obj) {
			o.MyString = ""
		},
		fields:      []any{&testObj.MyString},
		expectedRow: ovsdb.Row(map[string]any{"aString": ""}),
	}, {
		name: "empty set without field specification",
		prepare: func(_ *obj) {
		},
		expectedRow: ovsdb.Row(map[string]any{}),
	}, {
		name: "empty set without field specification",
		prepare: func(_ *obj) {
		},
		fields:      []any{&testObj.MySet},
		expectedRow: ovsdb.Row(map[string]any{"aSet": testOvsSet(t, []string{})}),
	}, {
		name: "empty maps",
		prepare: func(o *obj) {
			o.MyString = "foo"
		},
		expectedRow: ovsdb.Row(map[string]any{"aString": aString}),
	}, {
		name: "empty maps with field specification",
		prepare: func(o *obj) {
			o.MyString = "foo"
		},
		fields:      []any{&testObj.MyMap},
		expectedRow: ovsdb.Row(map[string]any{"aMap": testOvsMap(t, map[string]string{})}),
	}, {
		name: "Complex object with field selection",
		prepare: func(o *obj) {
			o.MyString = aString
			o.MyMap = aMap
			o.MySet = aSet
			o.MyFloat = aFloat
		},
		fields:      []any{&testObj.MyMap, &testObj.MySet},
		expectedRow: ovsdb.Row(map[string]any{"aMap": testOvsMap(t, aMap), "aSet": testOvsSet(t, aSet)}),
	},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("NewRow: %s", test.name), func(t *testing.T) {
			mapper := NewMapper(schema)
			// Clean the test object
			testObj.MyString = ""
			testObj.MyMap = nil
			testObj.MySet = nil
			testObj.MyFloat = 0

			test.prepare(&testObj)
			info, err := NewInfo("TestTable", schema.Table("TestTable"), &testObj)
			require.NoError(t, err)
			row, err := mapper.NewRow(info, test.fields...)
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equalf(t, test.expectedRow, row, "NewRow should match expected")
			}
		})
	}
}

func TestMapperCondition(t *testing.T) {

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
	}

	var schema ovsdb.DatabaseSchema
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Fatal(err)
	}
	mapper := NewMapper(schema)

	type Test struct {
		name     string
		prepare  func(*testType)
		expected []ovsdb.Condition
		index    []any
		err      bool
	}
	testObj := testType{}

	tests := []Test{
		{
			name: "simple index",
			prepare: func(t *testType) {
				t.ID = ""
				t.MyName = "foo"
				t.Config = nil
				t.Comp1 = ""
				t.Comp2 = ""
			},
			index:    []any{},
			expected: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
			err:      false,
		},
		{
			name: "UUID",
			prepare: func(t *testType) {
				t.ID = aUUID0
				t.MyName = "foo"
				t.Config = nil
				t.Comp1 = ""
				t.Comp2 = ""
			},
			index:    []any{},
			expected: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
			err:      false,
		},
		{
			name: "specify index",
			prepare: func(t *testType) {
				t.ID = aUUID0
				t.MyName = "foo"
				t.Config = nil
				t.Comp1 = ""
				t.Comp2 = ""
			},
			index:    []any{&testObj.MyName},
			expected: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
			err:      false,
		},
		{
			name: "complex index",
			prepare: func(t *testType) {
				t.ID = ""
				t.MyName = ""
				t.Config = nil
				t.Comp1 = "foo"
				t.Comp2 = "bar"
			},
			expected: []ovsdb.Condition{
				{Column: "composed_1", Function: ovsdb.ConditionEqual, Value: "foo"},
				{Column: "composed_2", Function: ovsdb.ConditionEqual, Value: "bar"}},
			index: []any{},
			err:   false,
		},
		{
			name: "first index",
			prepare: func(t *testType) {
				t.ID = ""
				t.MyName = "something"
				t.Config = nil
				t.Comp1 = "foo"
				t.Comp2 = "bar"
			},
			expected: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "something"}},
			index:    []any{},
			err:      false,
		},
		{
			name: "Error: None",
			prepare: func(t *testType) {
				t.ID = ""
				t.MyName = ""
				t.Config = map[string]string{"foo": "bar"}
				t.Comp1 = ""
				t.Comp2 = ""
			},
			index: []any{},
			err:   true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("newEqualityCondition_%s", tt.name), func(t *testing.T) {
			tt.prepare(&testObj)
			info, err := NewInfo("TestTable", schema.Table("TestTable"), &testObj)
			require.NoError(t, err)

			conds, err := mapper.NewEqualityCondition(info, tt.index...)
			if tt.err {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Error(err)
				}
				if !assert.ElementsMatch(t, tt.expected, conds, "Condition must match expected") {
					t.Logf("%v \n", conds)
				}
			}

		})
	}
}

func TestMapperEqualIndexes(t *testing.T) {

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
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Fatal(err)
	}
	mapper := NewMapper(schema)

	type Test struct {
		name     string
		obj1     testType
		obj2     testType
		expected bool
		indexes  []string
	}
	tests := []Test{
		{
			name: "same simple index",
			obj1: testType{
				MyName: "foo",
			},
			obj2: testType{
				MyName: "foo",
			},
			expected: true,
			indexes:  []string{},
		},
		{
			name: "diff simple index",
			obj1: testType{
				MyName: "foo",
			},
			obj2: testType{
				MyName: "bar",
			},
			expected: false,
			indexes:  []string{},
		},
		{
			name: "same uuid",
			obj1: testType{
				ID:     aUUID0,
				MyName: "foo",
			},
			obj2: testType{
				ID:     aUUID0,
				MyName: "bar",
			},
			expected: true,
			indexes:  []string{},
		},
		{
			name: "diff uuid",
			obj1: testType{
				ID:     aUUID0,
				MyName: "foo",
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "bar",
			},
			expected: false,
			indexes:  []string{},
		},
		{
			name: "same complex_index",
			obj1: testType{
				ID:     aUUID0,
				MyName: "foo",
				Comp1:  "foo",
				Comp2:  "bar",
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "bar",
				Comp1:  "foo",
				Comp2:  "bar",
			},
			expected: true,
			indexes:  []string{},
		},
		{
			name: "different",
			obj1: testType{
				ID:     aUUID0,
				MyName: "name1",
				Comp1:  "foo",
				Comp2:  "bar",
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "name2",
				Comp1:  "foo",
				Comp2:  "bar2",
			},
			expected: false,
			indexes:  []string{},
		},
		{
			name: "same additional index",
			obj1: testType{
				ID:     aUUID0,
				MyName: "name1",
				Comp1:  "foo",
				Comp2:  "bar1",
				Int1:   42,
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "name2",
				Comp1:  "foo",
				Comp2:  "bar2",
				Int1:   42,
			},
			expected: true,
			indexes:  []string{"int1"},
		},
		{
			name: "diff additional index",
			obj1: testType{
				ID:     aUUID0,
				MyName: "name1",
				Comp1:  "foo",
				Comp2:  "bar1",
				Int1:   42,
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "name2",
				Comp1:  "foo",
				Comp2:  "bar2",
				Int1:   420,
			},
			expected: false,
			indexes:  []string{"int1"},
		},
		{
			name: "same additional indexes ",
			obj1: testType{
				ID:     aUUID0,
				MyName: "name1",
				Comp1:  "foo",
				Comp2:  "bar1",
				Int1:   42,
				Int2:   25,
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "name2",
				Comp1:  "foo",
				Comp2:  "bar2",
				Int1:   42,
				Int2:   25,
			},
			expected: true,
			indexes:  []string{"int1", "int2"},
		},
		{
			name: "diff additional indexes ",
			obj1: testType{
				ID:     aUUID0,
				MyName: "name1",
				Comp1:  "foo",
				Comp2:  "bar1",
				Int1:   42,
				Int2:   50,
			},
			obj2: testType{
				ID:     aUUID1,
				MyName: "name2",
				Comp1:  "foo",
				Comp2:  "bar2",
				Int1:   42,
				Int2:   25,
			},
			expected: false,
			indexes:  []string{"int1", "int2"},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("Equal %s", test.name), func(t *testing.T) {
			info1, err := NewInfo("TestTable", schema.Table("TestTable"), &test.obj1)
			require.NoError(t, err)
			info2, err := NewInfo("TestTable", schema.Table("TestTable"), &test.obj2)
			require.NoError(t, err)
			eq, err := mapper.equalIndexes(info1, info2, test.indexes...)
			require.NoError(t, err)
			assert.Equalf(t, test.expected, eq, "equal value should match expected")
		})
	}

	// Test we can also use field pointers
	obj1 := testType{
		ID:     aUUID0,
		MyName: "name1",
		Comp1:  "foo",
		Comp2:  "bar1",
		Int1:   42,
		Int2:   25,
	}
	obj2 := testType{
		ID:     aUUID1,
		MyName: "name2",
		Comp1:  "foo",
		Comp2:  "bar2",
		Int1:   42,
		Int2:   25,
	}
	info1, err := NewInfo("TestTable", schema.Table("TestTable"), &obj1)
	require.NoError(t, err)
	info2, err := NewInfo("TestTable", schema.Table("TestTable"), &obj2)
	require.NoError(t, err)
	eq, err := mapper.EqualFields(info1, info2, &obj1.Int1, &obj1.Int2)
	require.NoError(t, err)
	assert.True(t, eq)
	// Using pointers to second value is not supported
	_, err = mapper.EqualFields(info1, info2, &obj2.Int1, &obj2.Int2)
	require.Error(t, err)

}

func TestMapperMutation(t *testing.T) {

	var testSchema = []byte(`{
  "cksum": "223619766 22548",
  "name": "TestSchema",
  "tables": {
    "TestTable": {
      "columns": {
        "string": {
          "type": "string"
        },
        "set": {
          "type": {
            "key": "string",
            "min": 0,
            "max": "unlimited"
          }
        },
        "map": {
          "type": {
            "key": "string",
            "value": "string"
          }
        },
        "unmutable": {
          "mutable": false,
          "type": {
            "key": "integer"
          }
	      },
        "int": {
          "type": {
            "key": "integer"
          }
	}
      }
    }
  }
}`)
	type testType struct {
		UUID      string            `ovsdb:"_uuid"`
		String    string            `ovsdb:"string"`
		Set       []string          `ovsdb:"set"`
		Map       map[string]string `ovsdb:"map"`
		Int       int               `ovsdb:"int"`
		UnMutable int               `ovsdb:"unmutable"`
	}

	var schema ovsdb.DatabaseSchema
	if err := json.Unmarshal(testSchema, &schema); err != nil {
		t.Fatal(err)
	}
	mapper := NewMapper(schema)

	type Test struct {
		name     string
		column   string
		obj      testType
		expected *ovsdb.Mutation
		mutator  ovsdb.Mutator
		value    any
		err      bool
	}
	tests := []Test{
		{
			name:    "string",
			column:  "string",
			obj:     testType{},
			mutator: ovsdb.MutateOperationAdd,
			err:     true,
		},
		{
			name:     "Increment integer",
			column:   "int",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationAdd,
			value:    1,
			expected: ovsdb.NewMutation("int", ovsdb.MutateOperationAdd, 1),
			err:      false,
		},
		{
			name:     "Increment integer",
			column:   "int",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationModulo,
			value:    2,
			expected: ovsdb.NewMutation("int", ovsdb.MutateOperationModulo, 2),
			err:      false,
		},
		{
			name:    "non-mutable",
			column:  "unmutable",
			obj:     testType{},
			mutator: ovsdb.MutateOperationSubtract,
			value:   2,
			err:     true,
		},
		{
			name:     "Add element to set ",
			column:   "set",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationInsert,
			value:    []string{"foo"},
			expected: ovsdb.NewMutation("set", ovsdb.MutateOperationInsert, testOvsSet(t, []string{"foo"})),
			err:      false,
		},
		{
			name:     "Delete element from set ",
			column:   "set",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationDelete,
			value:    []string{"foo"},
			expected: ovsdb.NewMutation("set", ovsdb.MutateOperationDelete, testOvsSet(t, []string{"foo"})),
			err:      false,
		},
		{
			name:     "Delete keys from map ",
			column:   "map",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationDelete,
			value:    []string{"foo", "bar"},
			expected: ovsdb.NewMutation("map", ovsdb.MutateOperationDelete, testOvsSet(t, []string{"foo", "bar"})),
			err:      false,
		},
		{
			name:     "Delete key value pairs from map ",
			column:   "map",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationDelete,
			value:    map[string]string{"foo": "bar"},
			expected: ovsdb.NewMutation("map", ovsdb.MutateOperationDelete, testOvsMap(t, map[string]string{"foo": "bar"})),
			err:      false,
		},
		{
			name:     "Insert elements in map ",
			column:   "map",
			obj:      testType{},
			mutator:  ovsdb.MutateOperationInsert,
			value:    map[string]string{"foo": "bar"},
			expected: ovsdb.NewMutation("map", ovsdb.MutateOperationInsert, testOvsMap(t, map[string]string{"foo": "bar"})),
			err:      false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("newMutation%s", test.name), func(t *testing.T) {
			info, err := NewInfo("TestTable", schema.Table("TestTable"), &test.obj)
			require.NoError(t, err)

			mutation, err := mapper.NewMutation(info, test.column, test.mutator, test.value)
			if test.err {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Error(err)
				}
			}

			assert.Equalf(t, test.expected, mutation, "Mutation must match expected")
		})
	}
}

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
