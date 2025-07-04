package ovsdb

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutationMarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		mutation Mutation
		want     string
		wantErr  bool
	}{
		{
			"test delete",
			Mutation{"foo", MutateOperationDelete, "bar"},
			`[ "foo", "delete", "bar" ]`,
			false,
		},
		{
			"test insert",
			Mutation{"foo", MutateOperationInsert, "bar"},
			`[ "foo", "insert", "bar" ]`,
			false,
		},
		{
			"test add",
			Mutation{"foo", MutateOperationAdd, "bar"},
			`[ "foo", "+=", "bar" ]`,
			false,
		},
		{
			"test subtract",
			Mutation{"foo", MutateOperationSubtract, "bar"},
			`[ "foo", "-=", "bar" ]`,
			false,
		},
		{
			"test multiply",
			Mutation{"foo", MutateOperationMultiply, "bar"},
			`[ "foo", "*=", "bar" ]`,
			false,
		},
		{
			"test divide",
			Mutation{"foo", MutateOperationDivide, "bar"},
			`[ "foo", "/=", "bar" ]`,
			false,
		},
		{
			"test modulo",
			Mutation{"foo", MutateOperationModulo, "bar"},
			`[ "foo", "%=", "bar" ]`,
			false,
		},
		{
			"test uuid",
			Mutation{"foo", MutateOperationInsert, UUID{GoUUID: "foo"}},
			`[ "foo", "insert", ["named-uuid", "foo"] ]`,
			false,
		},
		{
			"test set",
			Mutation{"foo", MutateOperationInsert, OvsDataSet{GoSet: []interface{}{"foo", "bar", "baz"}}},
			`[ "foo", "insert", ["set",["foo", "bar", "baz"]] ]`,
			false,
		},
		{
			"test map",
			Mutation{"foo", MutateOperationInsert, OvsMap{GoMap: map[interface{}]interface{}{"foo": "bar", "baz": "quux"}}},
			`[ "foo", "insert", ["map",[["foo", "bar"], ["baz", "quux"]]]]`,
			false,
		},
		{
			"test uuid set",
			Mutation{"foo", MutateOperationInsert, OvsDataSet{GoSet: []interface{}{UUID{GoUUID: "foo"}, UUID{GoUUID: "bar"}}}},
			`[ "foo", "insert", ["set",[["named-uuid", "foo"], ["named-uuid", "bar"]]] ]`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.mutation)
			if err != nil {
				t.Fatal(err)
			}
			// testing JSON equality is flaky for ovsdb notated maps
			// it's safe to skip this as we test from json->object later
			if tt.name != "test map" {
				assert.JSONEq(t, tt.want, string(got))
			}
			var c Mutation
			if err := json.Unmarshal(got, &c); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.mutation.Column, c.Column)
			assert.Equal(t, tt.mutation.Mutator, c.Mutator)
			v := reflect.TypeOf(tt.mutation.Value)
			vv := reflect.ValueOf(c.Value)
			if !vv.IsValid() {
				t.Fatalf("c.Value is empty: %v", c.Value)
			}
			assert.Equal(t, v, vv.Type())
			assert.Equal(t, tt.mutation.Value, vv.Convert(v).Interface())
			if vv.Kind() == reflect.String {
				assert.Equal(t, tt.mutation.Value, vv.String())
			}
		})
	}
}
