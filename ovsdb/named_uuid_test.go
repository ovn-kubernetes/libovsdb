package ovsdb

import (
	"fmt"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/internal/json"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const uuidTestSchema = `
{
    "name": "UUID_Test",
    "version": "0.0.1",
    "tables": {
        "UUID_Test": {
            "columns": {
                "_uuid": {
                    "type": "uuid"
                },
                "real_uuid": {
                    "type": "uuid"
                },
                "str": {
                    "type": "string"
                },
                "int": {
                    "type": "integer"
                },
                "uuidset": {
                    "type": {
                        "key": {
                            "type": "uuid"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                },
                "real_uuidset": {
                    "type": {
                        "key": {
                            "type": "uuid"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                },
                "strset": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                },
                "uuidmap": {
                    "type": {
                        "key": {
                            "type": "uuid"
                        },
                        "value": {
                            "type": "uuid"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "real_uuidmap": {
                    "type": {
                        "key": {
                            "type": "uuid"
                        },
                        "value": {
                            "type": "uuid"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "struuidmap": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "value": {
                            "type": "uuid"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "real_struuidmap": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "value": {
                            "type": "uuid"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "strmap": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "value": {
                            "type": "string"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                }
            },
            "isRoot": true
        }
    }
}
`

type UUIDTestType struct {
	UUID           string            `ovsdb:"_uuid"`
	RealUUID       UUID              `ovsdb:"real_uuid"`
	String         string            `ovsdb:"str"`
	Int            string            `ovsdb:"int"`
	UUIDSet        []string          `ovsdb:"uuidset"`
	RealUUIDSet    []UUID            `ovsdb:"real_uuidset"`
	StrSet         []string          `ovsdb:"strset"`
	UUIDMap        map[string]string `ovsdb:"uuidmap"`
	RealUUIDMap    map[UUID]UUID     `ovsdb:"real_uuidmap"`
	StrUUIDMap     map[string]string `ovsdb:"struuidmap"`
	RealStrUUIDMap map[string]UUID   `ovsdb:"real_struuidmap"`
	StrMap         map[string]string `ovsdb:"strmap"`
}

func getUUIDTestSchema() (DatabaseSchema, error) {
	var dbSchema DatabaseSchema
	err := json.Unmarshal([]byte(uuidTestSchema), &dbSchema)
	return dbSchema, err
}

func TestStandaloneExpandNamedUUID(t *testing.T) {
	testUUID := uuid.NewString()
	testUUID1 := uuid.NewString()
	tests := []struct {
		name       string
		namedUUIDs map[string]string
		column     string
		value      any
		expected   any
	}{
		{
			"uuid",
			map[string]string{"foo": testUUID},
			"_uuid",
			"foo",
			testUUID,
		},
		{
			"real uuid",
			map[string]string{"foo": testUUID},
			"real_uuid",
			UUID{GoUUID: "foo"},
			UUID{GoUUID: testUUID},
		},
		{
			"string (no replace)",
			map[string]string{"foo": testUUID},
			"str",
			"foo",
			"foo",
		},
		{
			"int (no replace)",
			map[string]string{"foo": testUUID},
			"int",
			15,
			15,
		},
		// OVS []UUID == Go []string
		{
			"UUID set",
			map[string]string{"foo": testUUID},
			"uuidset",
			OvsSet{GoSet: []any{"foo"}},
			OvsSet{GoSet: []any{testUUID}},
		},
		// OVS []UUID == Go []UUID
		{
			"real UUID set",
			map[string]string{"foo": testUUID},
			"real_uuidset",
			OvsSet{GoSet: []any{UUID{GoUUID: "foo"}}},
			OvsSet{GoSet: []any{UUID{GoUUID: testUUID}}},
		},
		{
			"set multiple",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"uuidset",
			OvsSet{GoSet: []any{"foo", "bar", "baz"}},
			OvsSet{GoSet: []any{testUUID, testUUID1, "baz"}},
		},
		// OVS [UUID]UUID == Go [string]string
		{
			"map key",
			map[string]string{"foo": testUUID},
			"uuidmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{testUUID: "bar"}},
		},
		{
			"map values",
			map[string]string{"bar": testUUID1},
			"uuidmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{"foo": testUUID1}},
		},
		{
			"map key and values",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"uuidmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{testUUID: testUUID1}},
		},
		// OVS [UUID]UUID == Go [UUID]UUID
		{
			"real UUID map key",
			map[string]string{"foo": testUUID},
			"real_uuidmap",
			OvsMap{GoMap: map[any]any{UUID{GoUUID: "foo"}: UUID{GoUUID: "bar"}}},
			OvsMap{GoMap: map[any]any{UUID{GoUUID: testUUID}: UUID{GoUUID: "bar"}}},
		},
		{
			"real UUID map values",
			map[string]string{"bar": testUUID1},
			"real_uuidmap",
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: "bar"}}},
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: testUUID1}}},
		},
		{
			"real UUID map key and values",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"real_uuidmap",
			OvsMap{GoMap: map[any]any{UUID{GoUUID: "foo"}: UUID{GoUUID: "bar"}}},
			OvsMap{GoMap: map[any]any{UUID{GoUUID: testUUID}: UUID{GoUUID: testUUID1}}},
		},
		// OVS [string]UUID == Go [string]string
		{
			"string UUID map key (no replace)",
			map[string]string{"foo": testUUID},
			"struuidmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
		},
		{
			"string UUID map values (replace)",
			map[string]string{"foo": testUUID},
			"struuidmap",
			OvsMap{GoMap: map[any]any{"foo": "foo"}},
			OvsMap{GoMap: map[any]any{"foo": testUUID}},
		},
		{
			"string UUID map key (no replace) and values (replace)",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"struuidmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{"foo": testUUID1}},
		},
		// OVS [string]UUID == Go [string]UUID
		{
			"real string UUID map key (no replace)",
			map[string]string{"foo": testUUID},
			"real_struuidmap",
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: "bar"}}},
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: "bar"}}},
		},
		{
			"real string UUID map values (replace)",
			map[string]string{"foo": testUUID},
			"real_struuidmap",
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: "foo"}}},
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: testUUID}}},
		},
		{
			"real string UUID map key (no replace) and values (replace)",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"real_struuidmap",
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: "bar"}}},
			OvsMap{GoMap: map[any]any{"foo": UUID{GoUUID: testUUID1}}},
		},
		// OVS [string]string == Go [string]string
		{
			"string map key and values (no replace)",
			map[string]string{"foo": testUUID, "bar": testUUID1},
			"strmap",
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
			OvsMap{GoMap: map[any]any{"foo": "bar"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := getUUIDTestSchema()
			require.NoError(t, err)
			ts := schema.Table("UUID_Test")
			require.NotNil(t, ts)
			cs := ts.Column(tt.column)
			require.NotNil(t, cs)

			got := expandNamedUUID(cs, tt.value, tt.namedUUIDs)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func makeOp(table, uuid, uuidName string, rows ...Row) Operation {
	op := Operation{
		Op:       OperationInsert,
		Table:    table,
		UUID:     uuid,
		UUIDName: uuidName,
	}
	if len(rows) == 1 {
		op.Row = rows[0]
	} else {
		op.Rows = rows
	}
	return op
}

func makeOpWhere(table, uuid, uuidName string, row Row, w ...Condition) Operation {
	op := makeOp(table, uuid, uuidName, row)
	op.Where = w
	return op
}

func makeOpMutation(table, uuid, uuidName string, row Row, m ...Mutation) Operation {
	op := makeOp(table, uuid, uuidName, row)
	op.Mutations = m
	return op
}

func TestOperationExpandNamedUUID(t *testing.T) {
	testUUID := uuid.NewString()
	testUUID1 := uuid.NewString()
	testUUID2 := uuid.NewString()
	namedUUID := "adsfasdfadsf"
	namedUUID1 := "142124521551"
	badUUID := "asdfadsfasdfasf"

	namedUUIDSet, _ := NewOvsSet([]UUID{{GoUUID: namedUUID}})
	testUUIDSet, _ := NewOvsSet([]UUID{{GoUUID: testUUID}})

	namedUUID1Map, _ := NewOvsMap(map[string]string{"foo": namedUUID1})
	testUUID1Map, _ := NewOvsMap(map[string]string{"foo": testUUID1})

	tests := []struct {
		name        string
		ops         []Operation
		expected    []Operation
		expectedErr string
	}{
		{
			"simple replace",
			[]Operation{
				makeOp("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"uuidset": []string{namedUUID}})),
			},
			[]Operation{
				makeOp("UUID_Test", testUUID, "",
					Row(map[string]any{"uuidset": []string{testUUID}})),
			},
			"",
		},
		{
			"simple replace multiple rows",
			[]Operation{
				makeOp("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"uuidset": []string{namedUUID}}),
					Row(map[string]any{"real_uuidset": namedUUIDSet}),
				),
			},
			[]Operation{
				makeOp("UUID_Test", testUUID, "",
					Row(map[string]any{"uuidset": []string{testUUID}}),
					Row(map[string]any{"real_uuidset": testUUIDSet}),
				),
			},
			"",
		},
		{
			"chained ops",
			[]Operation{
				makeOp("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"uuidset": []string{namedUUID}})),
				makeOp("UUID_Test", testUUID1, namedUUID1,
					Row(map[string]any{"real_uuid": UUID{GoUUID: namedUUID}})),
				makeOp("UUID_Test", testUUID2, "",
					Row(map[string]any{"struuidmap": namedUUID1Map})),
			},
			[]Operation{
				makeOp("UUID_Test", testUUID, "",
					Row(map[string]any{"uuidset": []string{testUUID}})),
				makeOp("UUID_Test", testUUID1, "",
					Row(map[string]any{"real_uuid": UUID{GoUUID: testUUID}})),
				makeOp("UUID_Test", testUUID2, "",
					Row(map[string]any{"struuidmap": testUUID1Map})),
			},
			"",
		},
		{
			"reverse ordered ops",
			[]Operation{
				makeOp("UUID_Test", testUUID1, namedUUID1,
					Row(map[string]any{"real_uuid": UUID{GoUUID: namedUUID}})),
				makeOp("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"uuidset": []string{namedUUID}})),
			},
			[]Operation{
				makeOp("UUID_Test", testUUID1, "",
					Row(map[string]any{"real_uuid": UUID{GoUUID: testUUID}})),
				makeOp("UUID_Test", testUUID, "",
					Row(map[string]any{"uuidset": []string{testUUID}})),
			},
			"",
		},
		{
			"where ops",
			[]Operation{
				makeOpWhere("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"_uuid": namedUUID}),
					NewCondition("_uuid", ConditionEqual, namedUUID),
				),
				makeOpWhere("UUID_Test", testUUID1, namedUUID1,
					Row(map[string]any{"real_uuid": UUID{GoUUID: namedUUID}}),
					NewCondition("_uuid", ConditionEqual, namedUUID),
				),
			},
			[]Operation{
				makeOpWhere("UUID_Test", testUUID, "",
					Row(map[string]any{"_uuid": testUUID}),
					NewCondition("_uuid", ConditionEqual, testUUID),
				),
				makeOpWhere("UUID_Test", testUUID1, "",
					Row(map[string]any{"real_uuid": UUID{GoUUID: testUUID}}),
					NewCondition("_uuid", ConditionEqual, testUUID),
				),
			},
			"",
		},
		{
			"mutation ops",
			[]Operation{
				makeOpMutation("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"_uuid": namedUUID}),
					*NewMutation("_uuid", MutateOperationAdd, namedUUID),
				),
				makeOpMutation("UUID_Test", testUUID1, namedUUID1,
					Row(map[string]any{"real_uuid": UUID{GoUUID: namedUUID}}),
					*NewMutation("_uuid", MutateOperationAdd, namedUUID),
				),
			},
			[]Operation{
				makeOpMutation("UUID_Test", testUUID, "",
					Row(map[string]any{"_uuid": testUUID}),
					*NewMutation("_uuid", MutateOperationAdd, testUUID),
				),
				makeOpMutation("UUID_Test", testUUID1, "",
					Row(map[string]any{"real_uuid": UUID{GoUUID: testUUID}}),
					*NewMutation("_uuid", MutateOperationAdd, testUUID),
				),
			},
			"",
		},
		{
			"invalid UUID",
			[]Operation{
				makeOp("UUID_Test", badUUID, "",
					Row(map[string]any{"uuidset": []string{namedUUID}})),
			},
			[]Operation{},
			fmt.Sprintf("operation UUID %q invalid", badUUID),
		},
		{
			"mismatched UUID for named UUID",
			[]Operation{
				makeOp("UUID_Test", testUUID, namedUUID,
					Row(map[string]any{"uuidset": []string{namedUUID}})),
				makeOp("UUID_Test", testUUID1, namedUUID,
					Row(map[string]any{"real_uuid": UUID{GoUUID: namedUUID}})),
			},
			[]Operation{},
			fmt.Sprintf("named UUID %q maps to UUID %q but found existing UUID %q", namedUUID, testUUID, testUUID1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := getUUIDTestSchema()
			require.NoError(t, err)

			got, err := ExpandNamedUUIDs(tt.ops, &schema)
			if tt.expectedErr != "" {
				require.Error(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
