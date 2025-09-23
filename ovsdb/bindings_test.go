package ovsdb

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/internal/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	aUUID0       = "2f77b348-9768-4866-b761-89d5177ecda0"
	aUUID1       = "2f77b348-9768-4866-b761-89d5177ecda1"
	aUUID2       = "2f77b348-9768-4866-b761-89d5177ecda2"
	aUUID3       = "2f77b348-9768-4866-b761-89d5177ecda3"
	aString      = "foo"
	aEnum        = "enum1"
	anIntEnum    = 1
	aRealEnum    = 1.0
	aBoolEnum    = true
	aEnumSet     = []string{"enum1", "enum2", "enum3"}
	anIntEnumSet = []int{1, 2, 3}
	aRealEnumSet = []float64{1.0, 2.0, 3.0}
	aBoolEnumSet = []bool{true, false}
	aUUIDEnumSet = []UUID{{GoUUID: aUUID0}, {GoUUID: aUUID1}, {GoUUID: aUUID2}}

	aSet = []string{"a", "set", "of", "strings"}

	aSingleUUIDSet, _ = NewOvsSet(UUID{GoUUID: aUUID0})

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

	aInt = 42

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

	aUUIDMap = map[string]string{
		"key1": aUUID0,
		"key2": aUUID1,
		"key3": aUUID2,
	}

	aEmptySet = []string{}
)

func TestOvsToNativeAndNativeToOvs(t *testing.T) {
	s, _ := NewOvsSet(aSet)
	s1, _ := NewOvsSet([]string{aString})

	us := make([]UUID, 0)
	for _, u := range aUUIDSet {
		us = append(us, UUID{GoUUID: u})
	}
	uss, _ := NewOvsSet(us)

	us1 := []UUID{{GoUUID: aUUID0}}
	uss1, _ := NewOvsSet(us1)

	is, _ := NewOvsSet(aIntSet)
	fs, _ := NewOvsSet(aFloatSet)

	sis, _ := NewOvsSet([]int{aInt})
	sfs, _ := NewOvsSet([]float64{aFloat})

	es, _ := NewOvsSet(aEmptySet)
	ens, _ := NewOvsSet(aEnumSet)
	ies, _ := NewOvsSet(anIntEnumSet)
	bes, _ := NewOvsSet(aBoolEnumSet)
	ues, _ := NewOvsSet(aUUIDEnumSet)
	res, _ := NewOvsSet(aRealEnumSet)

	m, _ := NewOvsMap(aMap)

	um, _ := NewOvsMap(map[string]UUID{
		"key1": {GoUUID: aUUID0},
		"key2": {GoUUID: aUUID1},
		"key3": {GoUUID: aUUID2},
	})

	singleStringSet, _ := NewOvsSet([]string{"foo"})

	tests := []struct {
		name   string
		schema []byte
		input  any
		native any
		ovs    any
	}{
		{
			name:   "String",
			schema: []byte(`{"type":"string"}`),
			input:  aString,
			native: aString,
			ovs:    aString,
		},
		{
			name:   "Float",
			schema: []byte(`{"type":"real"}`),
			input:  aFloat,
			native: aFloat,
			ovs:    aFloat,
		},
		{
			name:   "Integers with float ovs type",
			schema: []byte(`{"type":"integer"}`),
			input:  aFloat,
			native: aInt,
			ovs:    aInt,
		},
		{
			name:   "Integers",
			schema: []byte(`{"type":"integer"}`),
			input:  aInt,
			native: aInt,
			ovs:    aInt,
		},
		{
			name:   "Integer set with float ovs type ",
			schema: []byte(`{"type":"integer", "min":0}`),
			input:  aFloat,
			native: aInt,
			ovs:    aInt,
		},
		{
			name: "String Set",
			schema: []byte(`{"type": {
            "key": "string",
            "max": "unlimited",
            "min": 0
            }}`),
			input:  s,
			native: aSet,
			ovs:    s,
		},
		{
			// string with exactly one element can also be represented
			// as the element itself. On ovs2native, we keep the slice representation
			name: "String Set with exactly one field",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input:  aString,
			native: []string{aString},
			ovs:    s1,
		},
		{
			name: "UUID Set",
			schema: []byte(`{
			"type":{
				"key": {
					"refTable": "SomeOtherTAble",
					"refType": "weak",
					"type": "uuid"
				},
				"min": 0,
				"max": "unlimited"
				}
			}`),
			input:  uss,
			native: aUUIDSet,
			ovs:    uss,
		},
		{
			name: "UUID Set with exactly one field",
			schema: []byte(`{
			"type":{
					"key": {
						"refTable": "SomeOtherTAble",
						"refType": "weak",
						"type": "uuid"
					},
					"min": 0,
					"max": "unlimited"
					}
			}`),
			input:  UUID{GoUUID: aUUID0},
			native: []string{aUUID0},
			ovs:    uss1,
		},
		{
			name: "Integer Set",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "integer"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  is,
			native: aIntSet,
			ovs:    is,
		},
		{
			name: "Integer Set single with float ovs input",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "integer"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  fs,
			native: aIntSet,
			ovs:    is,
		},
		{
			// A single-value integer set with integer ovs input
			name: "Integer Set single",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "integer"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  sis,
			native: []int{aInt},
			ovs:    sis,
		},
		{
			// A single-value integer set with float ovs input
			name: "Integer Set single",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "integer"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  sfs,
			native: []int{aInt},
			ovs:    sis,
		},

		{
			// A float set
			name: "Float Set",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "real"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  fs,
			native: aFloatSet,
			ovs:    fs,
		},
		{
			// A empty string set
			name: "Empty String Set",
			schema: []byte(`{
			"type":{
					"key": {
					"type": "string"
					},
					"min": 0,
					"max": "unlimited"
				}
			}`),
			input:  es,
			native: aEmptySet,
			ovs:    es,
		},
		{
			// Enum
			name: "Enum (string)",
			schema: []byte(`{
			"type":{
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
			}`),
			input:  aEnum,
			native: aEnum,
			ovs:    aEnum,
		},
		{
			// Enum set
			name: "Enum Set (string)",
			schema: []byte(`{
			"type":{
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
				},
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input:  ens,
			native: aEnumSet,
			ovs:    ens,
		},
		{
			name: "Enum (integer)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							1,
							2,
							3
						]
					],
					"type": "integer"
				}
			}
		}`),
			input:  anIntEnum,
			native: anIntEnum,
			ovs:    anIntEnum,
		},
		{
			name: "Enum Set (integer)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							1,
							2,
							3
						]
					],
					"type": "integer"
				},
				"max": "unlimited",
				"min": 0
			}
		}`),
			input:  ies,
			native: anIntEnumSet,
			ovs:    ies,
		},
		{
			name: "Enum (real)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							1.0,
							2.0,
							3.0
						]
					],
					"type": "real"
				}
			}
		}`),
			input:  aRealEnum,
			native: aRealEnum,
			ovs:    aRealEnum,
		},
		{
			name: "Enum Set (real)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							1.0,
							2.0,
							3.0
						]
					],
					"type": "real"
				},
				"max": "unlimited",
				"min": 0
			}
		}`),
			input:  res,
			native: aRealEnumSet,
			ovs:    res,
		},
		{
			name: "Enum (bool)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							true,
							false
						]
					],
					"type": "boolean"
				}
			}
		}`),
			input:  aBoolEnum,
			native: aBoolEnum,
			ovs:    aBoolEnum,
		},
		{
			name: "Enum Set (bool)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							true,
							false
						]
					],
					"type": "boolean"
				},
				"min": 0,
				"max": "unlimited"
			}
		}`),
			input:  bes,
			native: aBoolEnumSet,
			ovs:    bes,
		},
		{
			name: "Enum (uuid)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda0"],
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda1"],
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda2"]
						]
					],
					"type": "uuid"
				}
			}
		}`),
			input:  UUID{GoUUID: aUUID0},
			native: aUUID0,
			ovs:    UUID{GoUUID: aUUID0},
		},
		{
			name: "Enum Set (uuid)",
			schema: []byte(`{
			"type": {
				"key": {
					"enum": [
						"set",
						[
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda0"],
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda1"],
							["uuid", "2f77b348-9768-4866-b761-89d5177ecda2"]
						]
					],
					"type": "uuid"
				},
				"min": 0,
				"max": "unlimited"
			}
		}`),
			input:  ues,
			native: []string{aUUID0, aUUID1, aUUID2},
			ovs:    ues,
		},
		{
			name: "Map (string->string)",
			schema: []byte(`{
			"type": {
				"key": "string",
				"max": "unlimited",
				"min": 0,
				"value": "string"
			}
			}`),
			input:  m,
			native: aMap,
			ovs:    m,
		},
		{
			name: "Map (string->uuid)",
			schema: []byte(`{
			"type": {
				"key": "string",
				"max": "unlimited",
				"min": 0,
				"value": "uuid"
			}
			}`),
			input:  um,
			native: aUUIDMap,
			ovs:    um,
		},
		{
			name: "String set with min 0 max 1",
			schema: []byte(`{
			"type":{
				"key": {
				"type": "string"
				},
				"min": 0,
				"max": 1
			}
			}`),
			input:  singleStringSet,
			native: &aString,
			ovs:    singleStringSet,
		},
		{
			name: "UUID set with min 0 max 1",
			schema: []byte(`{
			"type":{
				"key": {
					"refTable": "SomeOtherTAble",
					"refType": "weak",
					"type": "uuid"
				},
				"min": 0,
				"max": 1
			}
			}`),
			input:  aSingleUUIDSet,
			native: &aUUID0,
			ovs:    aSingleUUIDSet,
		},
		{
			name: "null UUID set with min 0 max 1",
			schema: []byte(`{
			"type":{
				"key": {
					"refTable": "SomeOtherTAble",
					"refType": "weak",
					"type": "uuid"
				},
				"min": 0,
				"max": 1
			}
			}`),
			input:  es,
			native: (*string)(nil),
			ovs:    es,
		},
		{
			name: "A string with min 0 max 1",
			schema: []byte(`{
			"type":{
				"key": {
				"type": "string"
				},
				"min": 0,
				"max": 1
			}
			}`),
			input:  aString,
			native: &aString,
			ovs:    singleStringSet,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var column ColumnSchema
			err := json.Unmarshal(tt.schema, &column)
			require.NoError(t, err)

			native, err := OvsToNative(&column, tt.input)
			require.NoError(t, err)

			require.Equalf(t, tt.native, native,
				"fail to convert ovs2native. input: %v(%s). expected %v(%s). got %v (%s)",
				tt.input, reflect.TypeOf(tt.input),
				tt.native, reflect.TypeOf(tt.native),
				native, reflect.TypeOf(native),
			)

			ovs, err := NativeToOvs(&column, native)
			require.NoErrorf(t, err, "failed to convert %s: %s", tt, err)

			assert.Equalf(t, tt.ovs, ovs,
				"fail to convert native2ovs. native: %v(%s). expected %v(%s). got %v (%s)",
				native, reflect.TypeOf(native),
				tt.ovs, reflect.TypeOf(tt.ovs),
				ovs, reflect.TypeOf(ovs),
			)
		})
	}
}

func TestOvsToNativeErr(t *testing.T) {
	as, _ := NewOvsSet([]string{"foo"})

	s, _ := NewOvsMap(map[string]string{"foo": "bar"})

	m, _ := NewOvsMap(map[int]string{1: "one", 2: "two"})

	tests := []struct {
		name   string
		schema []byte
		input  any
	}{
		{
			name:   "Wrong Atomic Type",
			schema: []byte(`{"type":"string"}`),
			input:  42,
		},
		{
			name:   "Wrong Atomic Numeric Type: Float",
			schema: []byte(`{"type":"real"}`),
			input:  42,
		},
		{
			name:   "Set instead of Atomic Type",
			schema: []byte(`{"type":"string"}`),
			input:  as,
		},
		{
			name: "Wrong Set Type",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input: []int{1, 2},
		},
		{
			name: "Wrong Map instead of Set",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input: s,
		},
		{
			name: "Wrong Map key type",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0,
				"value": "string"
			  }
			}`),
			input: m,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var column ColumnSchema
			err := json.Unmarshal(tt.schema, &column)
			require.NoError(t, err)
			res, err := OvsToNative(&column, tt.input)
			assert.Errorf(t, err,
				"conversion %+v should have failed, instead it has returned %v (%s)",
				tt, res, reflect.TypeOf(res),
			)
		})
	}
}

func TestNativeToOvsErr(t *testing.T) {
	tests := []struct {
		name   string
		schema []byte
		input  any
	}{
		{
			name:   "Wrong Atomic Type",
			schema: []byte(`{"type":"string"}`),
			input:  42,
		},
		{
			// OVS floats should be convertible to integers since encoding/json will use float64 as
			// the default numeric type. However, native types should match
			name:   "Wrong Atomic Numeric Type: Int",
			schema: []byte(`{"type":"integer"}`),
			input:  42.0,
		},
		{
			name:   "Wrong Atomic Numeric Type: Float",
			schema: []byte(`{"type":"real"}`),
			input:  42,
		},
		{
			name:   "Set instead of Atomic Type",
			schema: []byte(`{"type":"string"}`),
			input:  []string{"foo"},
		},
		{
			name: "Wrong Set Type",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input: []int{1, 2},
		},
		{
			name: "Wrong Map instead of Set",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0
			  }
			}`),
			input: map[string]string{"foo": "bar"},
		},
		{
			name: "Wrong Map key type",
			schema: []byte(`{
			  "type": {
				"key": "string",
				"max": "unlimited",
				"min": 0,
				"value": "string"
			  }
		}`),
			input: map[int]string{1: "one", 2: "two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var column ColumnSchema
			if err := json.Unmarshal(tt.schema, &column); err != nil {
				t.Fatal(err)
			}
			res, err := NativeToOvs(&column, tt.input)
			if err == nil {
				t.Errorf("conversion %s should have failed, instead it has returned %v (%s)", tt, res, reflect.TypeOf(res))
				t.Logf("Conversion schema %v", string(tt.schema))
			}
		})
	}
}

func TestIsDefault(t *testing.T) {
	type Test struct {
		name     string
		column   []byte
		elem     any
		expected bool
	}
	tests := []Test{
		{
			name:     "empty string",
			column:   []byte(`{"type":"string"}`),
			elem:     "",
			expected: true,
		},
		{
			name:     "non string",
			column:   []byte(`{"type":"string"}`),
			elem:     "something",
			expected: false,
		},
		{
			name:     "empty uuid",
			column:   []byte(`{"type":"uuid"}`),
			elem:     "",
			expected: true,
		},
		{
			name:     "default uuid",
			column:   []byte(`{"type":"uuid"}`),
			elem:     "00000000-0000-0000-0000-000000000000",
			expected: true,
		},
		{
			name:     "non-empty uuid",
			column:   []byte(`{"type":"uuid"}`),
			elem:     aUUID0,
			expected: false,
		},
		{
			name:     "zero int",
			column:   []byte(`{"type":"integer"}`),
			elem:     0,
			expected: true,
		},
		{
			name:     "non-zero int",
			column:   []byte(`{"type":"integer"}`),
			elem:     42,
			expected: false,
		},
		{
			name:     "non-zero float",
			column:   []byte(`{"type":"real"}`),
			elem:     42.0,
			expected: false,
		},
		{
			name:     "zero float",
			column:   []byte(`{"type":"real"}`),
			elem:     0.0,
			expected: true,
		},
		{
			name: "empty set ",
			column: []byte(`{
					   "type": {
   					     "key": "string",
    					     "max": "unlimited",
    					     "min": 0
    					   }
    					 }`),
			elem:     []string{},
			expected: true,
		},
		{
			name: "empty set allocated",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "max": "unlimited",
    					     "min": 0
    					   }
    					 }`),
			elem:     make([]string, 0, 10),
			expected: true,
		},
		{
			name: "non-empty set",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "max": "unlimited",
    					     "min": 0
    					   }
    					 }`),
			elem:     []string{"something"},
			expected: false,
		},
		{
			name: "empty map allocated",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "value": "string"
    					   }
    					 }`),
			elem:     make(map[string]string),
			expected: true,
		},
		{
			name: "nil map",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "value": "string"
    					   }
    					 }`),
			elem:     nil,
			expected: true,
		},
		{
			name: "empty map",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "value": "string"
    					   }
    					 }`),
			elem:     map[string]string{},
			expected: true,
		},
		{
			name: "non-empty map",
			column: []byte(`{
					   "type": {
    					     "key": "string",
    					     "value": "string"
    					   }
   					 }`),
			elem:     map[string]string{"foo": "bar"},
			expected: false,
		},
		{
			name: "empty enum",
			column: []byte(`{
					"type":{
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
					}`),
			elem:     "",
			expected: true,
		},
		{
			name: "non-empty enum",
			column: []byte(`{
					"type":{
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
					}`),
			elem:     "enum1",
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("IsDefault: %s", test.name), func(t *testing.T) {
			var column ColumnSchema
			if err := json.Unmarshal(test.column, &column); err != nil {
				t.Fatal(err)
			}

			result := IsDefaultValue(&column, test.elem)
			if result != test.expected {
				t.Errorf("failed to determine if %v is default. expected %t got %t", test, test.expected, result)
				t.Logf("Conversion schema %v", string(test.column))
			}

		})
	}
}

func TestMutationValidation(t *testing.T) {
	type Test struct {
		name     string
		column   []byte
		mutators []Mutator
		value    any
		valid    bool
	}
	tests := []Test{
		{
			name:     "string",
			column:   []byte(`{"type":"string"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    "foo",
			valid:    false,
		},
		{
			name:     "string",
			column:   []byte(`{"type":"uuid"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    "foo",
			valid:    false,
		},
		{
			name:     "boolean",
			column:   []byte(`{"type":"boolean"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    true,
			valid:    false,
		},
		{
			name:     "integer",
			column:   []byte(`{"type":"integer"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    4,
			valid:    true,
		},
		{
			name:     "unmutable",
			column:   []byte(`{"type":"integer", "mutable": false}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    4,
			valid:    false,
		},
		{
			name:     "integer wrong mutator",
			column:   []byte(`{"type":"integer"}`),
			mutators: []Mutator{"some", "wrong", "mutator"},
			value:    4,
			valid:    false,
		},
		{
			name:     "integer wrong type",
			column:   []byte(`{"type":"integer"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    "foo",
			valid:    false,
		},
		{
			name:     "real",
			column:   []byte(`{"type":"real"}`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide},
			value:    4.0,
			valid:    true,
		},
		{
			name:     "real-%/",
			column:   []byte(`{"type":"real"}`),
			mutators: []Mutator{MutateOperationModulo},
			value:    4.0,
			valid:    false,
		},
		{
			name: "integer set",
			column: []byte(`{
				   "type": {
				     "key": "integer",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide, MutateOperationModulo},
			value:    4,
			valid:    true,
		},
		{
			name: "float set /",
			column: []byte(`{
				   "type": {
				     "key": "real",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply, MutateOperationDivide},
			value:    4.0,
			valid:    true,
		},
		{
			name: "string set wrong mutator",
			column: []byte(`{
				   "type": {
				     "key": "real",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationAdd, MutateOperationAdd, MutateOperationSubtract, MutateOperationMultiply},
			value:    "foo",
			valid:    false,
		},
		{
			name: "string set insert single string",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert},
			value:    "foo",
			valid:    true,
		},
		{
			name: "string set insert single int",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert},
			value:    42,
			valid:    false,
		},
		{
			name: "string set insert/delete",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert, MutateOperationDelete},
			value:    []string{"foo", "bar"},
			valid:    true,
		},
		{
			name: "integer set insert/delete",
			column: []byte(`{
				   "type": {
				     "key": "integer",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert, MutateOperationDelete},
			value:    []int{45, 11},
			valid:    true,
		},
		{
			name: "map insert, wrong type",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert},
			value:    []string{"foo"},
			valid:    false,
		},
		{
			name: "map insert",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			mutators: []Mutator{MutateOperationInsert},
			value:    map[string]string{"foo": "bar"},
			valid:    true,
		},
		{
			name: "map delete k-v",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			mutators: []Mutator{MutateOperationDelete},
			value:    map[string]string{"foo": "bar"},
			valid:    true,
		},
		{
			name: "map delete k set",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			mutators: []Mutator{MutateOperationDelete},
			value:    []string{"foo", "bar"},
			valid:    true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("MutationValidation: %s", test.name), func(t *testing.T) {
			var column ColumnSchema
			if err := json.Unmarshal(test.column, &column); err != nil {
				t.Fatal(err)
			}

			for _, m := range test.mutators {
				result := ValidateMutation(&column, m, test.value)
				if test.valid {
					require.NoError(t, result)
				} else {
					require.Error(t, result)
				}
			}
		})
	}
}

func TestConditionValidation(t *testing.T) {
	type Test struct {
		name      string
		column    []byte
		functions []ConditionFunction
		value     any
		valid     bool
	}
	tests := []Test{
		{
			name:      "string",
			column:    []byte(`{"type":"string"}`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     "foo",
			valid:     true,
		},
		{
			name:      "uuid",
			column:    []byte(`{"type":"uuid"}`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     "foo",
			valid:     true,
		},
		{
			name:      "string wrong type",
			column:    []byte(`{"type":"string"}`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     42,
			valid:     false,
		},
		{
			name:      "numeric",
			column:    []byte(`{"type":"integer"}`),
			functions: []ConditionFunction{ConditionGreaterThanOrEqual, ConditionGreaterThan, ConditionLessThan, ConditionLessThanOrEqual, ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     1000,
			valid:     true,
		},
		{
			name:      "numeric wrong type",
			column:    []byte(`{"type":"integer"}`),
			functions: []ConditionFunction{ConditionGreaterThanOrEqual, ConditionGreaterThan, ConditionLessThan, ConditionLessThanOrEqual, ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     "foo",
			valid:     false,
		},
		{
			name: "set",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     []string{"foo", "bar"},
			valid:     true,
		},
		{
			name: "set wrong type",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     32,
			valid:     false,
		},
		{
			name: "set wrong type2",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "max": "unlimited",
				     "min": 0
				   }
				 }`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     "foo",
			valid:     false,
		},
		{
			name: "map",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     map[string]string{"foo": "bar"},
			valid:     true,
		},
		{
			name: "map wrong type",
			column: []byte(`{
				   "type": {
				     "key": "string",
				     "value": "string"
				   }
				 }`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     map[string]int{"foo": 42},
			valid:     false,
		},
		{
			name: "enum",
			column: []byte(`{"type": {"key": {"type": "string",
                                            "enum": ["set", ["from-lport", "to-lport"]]}}}`),
			functions: []ConditionFunction{ConditionEqual, ConditionIncludes, ConditionNotEqual, ConditionExcludes},
			value:     "from-lport",
			valid:     true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("ConditionValidation: %s", test.name), func(t *testing.T) {
			var column ColumnSchema
			err := json.Unmarshal(test.column, &column)
			require.NoError(t, err)

			for _, f := range test.functions {
				result := ValidateCondition(&column, f, test.value)
				if test.valid {
					require.NoError(t, result)
				} else {
					require.Error(t, result)
				}
			}
		})
	}
}
