package ovsdb

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConditionMarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		want      string
		wantErr   bool
	}{
		{
			"test <",
			Condition{"foo", ConditionLessThan, "bar"},
			`[ "foo", "<", "bar" ]`,
			false,
		},
		{
			"test <=",
			Condition{"foo", ConditionLessThanOrEqual, "bar"},
			`[ "foo", "<=", "bar" ]`,
			false,
		},
		{
			"test >",
			Condition{"foo", ConditionGreaterThan, "bar"},
			`[ "foo", ">", "bar" ]`,
			false,
		},
		{
			"test >=",
			Condition{"foo", ConditionGreaterThanOrEqual, "bar"},
			`[ "foo", ">=", "bar" ]`,
			false,
		},
		{
			"test ==",
			Condition{"foo", ConditionEqual, "bar"},
			`[ "foo", "==", "bar" ]`,
			false,
		},
		{
			"test !=",
			Condition{"foo", ConditionNotEqual, "bar"},
			`[ "foo", "!=", "bar" ]`,
			false,
		},
		{
			"test includes",
			Condition{"foo", ConditionIncludes, "bar"},
			`[ "foo", "includes", "bar" ]`,
			false,
		},
		{
			"test excludes",
			Condition{"foo", ConditionExcludes, "bar"},
			`[ "foo", "excludes", "bar" ]`,
			false,
		},
		{
			"test uuid",
			Condition{"foo", ConditionExcludes, UUID{GoUUID: "foo"}},
			`[ "foo", "excludes", ["named-uuid", "foo"] ]`,
			false,
		},
		{
			"test set",
			Condition{"foo", ConditionExcludes, OvsSet{GoSet: []any{"foo", "bar", "baz"}}},
			`[ "foo", "excludes", ["set",["foo", "bar", "baz"]] ]`,
			false,
		},
		{
			"test map",
			Condition{"foo", ConditionExcludes, OvsMap{GoMap: map[any]any{"foo": "bar", "baz": "quux"}}},
			`[ "foo", "excludes", ["map",[["foo", "bar"], ["baz", "quux"]]]]`,
			false,
		},
		{
			"test uuid set",
			Condition{"foo", ConditionExcludes, OvsSet{GoSet: []any{UUID{GoUUID: "foo"}, UUID{GoUUID: "bar"}}}},
			`[ "foo", "excludes", ["set",[["named-uuid", "foo"], ["named-uuid", "bar"]]] ]`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.condition)
			if err != nil {
				t.Fatal(err)
			}
			// testing JSON equality is flaky for ovsdb notated maps
			// it's safe to skip this as we test from json->object later
			if tt.name != "test map" {
				assert.JSONEq(t, tt.want, string(got))
			}
			var c Condition
			if err := json.Unmarshal(got, &c); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.condition.Column, c.Column)
			assert.Equal(t, tt.condition.Function, c.Function)
			v := reflect.TypeOf(tt.condition.Value)
			vv := reflect.ValueOf(c.Value)
			if !vv.IsValid() {
				t.Fatalf("c.Value is empty: %v", c.Value)
			}
			assert.Equal(t, v, vv.Type())
			assert.Equal(t, tt.condition.Value, vv.Convert(v).Interface())
			if vv.Kind() == reflect.String {
				assert.Equal(t, tt.condition.Value, vv.String())
			}
		})
	}
}

func TestCondition_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Column   string
		Function ConditionFunction
		Value    any
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"success",
			fields{"foo", ConditionEqual, "bar"},
			args{[]byte(`[ "foo", "==", "bar" ]`)},
			false,
		},
		{
			"bad function",
			fields{},
			args{[]byte(`[ "foo", "baz", "bar" ]`)},
			true,
		},
		{
			"too many elements",
			fields{},
			args{[]byte(`[ "foo", "bar", "baz", "quuz" ]`)},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Condition{
				Column:   tt.fields.Column,
				Function: tt.fields.Function,
				Value:    tt.fields.Value,
			}
			if err := c.UnmarshalJSON(tt.args.b); (err != nil) != tt.wantErr {
				t.Errorf("Condition.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConditionFunctionEvaluate(t *testing.T) {
	tests := []struct {
		name    string
		c       ConditionFunction
		a       any
		b       any
		want    bool
		wantErr bool
	}{
		{
			"equal string true",
			ConditionEqual,
			"foo",
			"foo",
			true,
			false,
		},
		{
			"equal string false",
			ConditionEqual,
			"foo",
			"bar",
			false,
			false,
		},
		{
			"equal int true",
			ConditionEqual,
			1024,
			1024,
			true,
			false,
		},
		{
			"equal int false",
			ConditionEqual,
			1024,
			2048,
			false,
			false,
		},
		{
			"equal real true",
			ConditionEqual,
			float64(42.0),
			float64(42.0),
			true,
			false,
		},
		{
			"equal real false",
			ConditionEqual,
			float64(42.0),
			float64(420.0),
			false,
			false,
		},
		{
			"equal map true",
			ConditionEqual,
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			true,
			false,
		},
		{
			"equal map false",
			ConditionEqual,
			map[string]string{"foo": "bar"},
			map[string]string{"bar": "baz"},
			false,
			false,
		},
		{
			"equal slice true",
			ConditionEqual,
			[]string{"foo", "bar"},
			[]string{"foo", "bar"},
			true,
			false,
		},
		{
			"equal slice false",
			ConditionEqual,
			[]string{"foo", "bar"},
			[]string{"foo", "baz"},
			false,
			false,
		},
		{
			"notequal string true",
			ConditionNotEqual,
			"foo",
			"bar",
			true,
			false,
		},
		{
			"notequal string false",
			ConditionNotEqual,
			"foo",
			"foo",
			false,
			false,
		},
		{
			"notequal int true",
			ConditionNotEqual,
			1024,
			2048,
			true,
			false,
		},
		{
			"notequal int false",
			ConditionNotEqual,
			1024,
			1024,
			false,
			false,
		},
		{
			"notequal real true",
			ConditionNotEqual,
			float64(42.0),
			float64(24.0),
			true,
			false,
		},
		{
			"notequal real false",
			ConditionNotEqual,
			float64(42.0),
			float64(42.0),
			false,
			false,
		},
		{
			"notequal map true",
			ConditionNotEqual,
			map[string]string{"foo": "bar"},
			map[string]string{"bar": "baz"},
			true,
			false,
		},
		{
			"notequal map false",
			ConditionNotEqual,
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			false,
			false,
		},
		{
			"notequal slice true",
			ConditionNotEqual,
			[]string{"foo", "bar"},
			[]string{"foo", "baz"},
			true,
			false,
		},
		{
			"notequal slice false",
			ConditionNotEqual,
			[]string{"foo", "bar"},
			[]string{"foo", "bar"},
			false,
			false,
		},
		{
			"includes string true",
			ConditionIncludes,
			"foo",
			"foo",
			true,
			false,
		},
		{
			"includes string false",
			ConditionIncludes,
			"foo",
			"bar",
			false,
			false,
		},
		{
			"incldes int true",
			ConditionIncludes,
			1024,
			1024,
			true,
			false,
		},
		{
			"includes int false",
			ConditionIncludes,
			1024,
			2048,
			false,
			false,
		},
		{
			"includes real true",
			ConditionIncludes,
			float64(42.0),
			float64(42.0),
			true,
			false,
		},
		{
			"includes real false",
			ConditionIncludes,
			float64(42.0),
			float64(420.0),
			false,
			false,
		},
		{
			"includes map true",
			ConditionIncludes,
			map[any]any{1: "bar", "bar": "baz", "baz": "quux"},
			map[any]any{1: "bar"},
			true,
			false,
		},
		{
			"includes map false",
			ConditionIncludes,
			map[string]string{"foo": "bar", "bar": "baz", "baz": "quux"},
			map[string]string{"quux": "foobar"},
			false,
			false,
		},
		{
			"includes slice true",
			ConditionIncludes,
			[]string{"foo", "bar", "baz", "quux"},
			[]string{"foo", "bar"},
			true,
			false,
		},
		{
			"includes slice false",
			ConditionIncludes,
			[]string{"foo", "bar", "baz", "quux"},
			[]string{"foobar", "quux"},
			false,
			false,
		},
		{
			"excludes string true",
			ConditionExcludes,
			"foo",
			"bar",
			true,
			false,
		},
		{
			"excludes string false",
			ConditionExcludes,
			"foo",
			"foo",
			false,
			false,
		},
		{
			"excludes int true",
			ConditionExcludes,
			1024,
			2048,
			true,
			false,
		},
		{
			"excludes int false",
			ConditionExcludes,
			1024,
			1024,
			false,
			false,
		},
		{
			"excludes real true",
			ConditionExcludes,
			float64(42.0),
			float64(24.0),
			true,
			false,
		},
		{
			"excludes real false",
			ConditionExcludes,
			float64(42.0),
			float64(42.0),
			false,
			false,
		},
		{
			"excludes map true",
			ConditionExcludes,
			map[any]any{1: "bar", "bar": "baz", "baz": "quux"},
			map[any]any{1: "foo"},
			true,
			false,
		},
		{
			"excludes map false",
			ConditionExcludes,
			map[string]string{"foo": "bar", "bar": "baz", "baz": "quux"},
			map[string]string{"foo": "bar"},
			false,
			false,
		},
		{
			"excludes slice true",
			ConditionExcludes,
			[]string{"foo", "bar", "baz", "quux"},
			[]string{"foobar"},
			true,
			false,
		},
		{
			"excludes slice false",
			ConditionExcludes,
			[]string{"foobar", "bar", "baz", "quux"},
			[]string{"foobar", "quux"},
			false,
			false,
		},
		{
			"lt unsuported",
			ConditionLessThan,
			"foo",
			"foo",
			false,
			true,
		},
		{
			"lteq unsupported",
			ConditionLessThanOrEqual,
			[]string{"foo"},
			[]string{"foo"},
			false,
			true,
		},
		{
			"gt unsupported",
			ConditionGreaterThan,
			map[string]string{"foo": "foo"},
			map[string]string{"foo": "foo"},
			false,
			true,
		},
		{
			"gteq unsupported",
			ConditionGreaterThanOrEqual,
			true,
			true,
			false,
			true,
		},
		{
			"lt true",
			ConditionLessThan,
			0,
			42,
			true,
			false,
		},
		{
			"lteq true",
			ConditionLessThanOrEqual,
			42,
			42,
			true,
			false,
		},
		{
			"gt true",
			ConditionGreaterThan,
			float64(420.0),
			float64(42.0),
			true,
			false,
		},
		{
			"gteq true",
			ConditionGreaterThanOrEqual,
			float64(420.00),
			float64(419.99),
			true,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.Evaluate(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConditionFunction.Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConditionFunction.Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSliceContains(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{
			"string slice",
			[]string{"foo", "bar", "baz"},
			[]string{"foo", "bar"},
			true,
		},
		{
			"int slice",
			[]int{1, 2, 3},
			[]int{2, 3},
			true,
		},
		{
			"real slice",
			[]float64{42.0, 42.0, 24.0},
			[]float64{42.0, 24.0},
			true,
		},
		{
			"interface slice",
			[]any{1, "bar", "baz"},
			[]any{1, "bar"},
			true,
		},
		{
			"no match",
			[]any{1, "bar", "baz"},
			[]any{2, "bar"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := reflect.ValueOf(tt.a)
			y := reflect.ValueOf(tt.b)
			if got := sliceContains(x, y); got != tt.want {
				t.Errorf("compareSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestMapContains(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{
			"string map",
			map[string]string{"foo": "bar", "bar": "baz"},
			map[string]string{"foo": "bar"},
			true,
		},
		{
			"int keys",
			map[int]string{1: "bar", 2: "baz"},
			map[int]string{1: "bar"},
			true,
		},
		{
			"interface keys",
			map[any]any{1: 1024, 2: "baz"},
			map[any]any{2: "baz"},
			true,
		},
		{
			"no key match",
			map[string]string{"foo": "bar", "bar": "baz"},
			map[string]string{"quux": "bar"},
			false,
		},
		{
			"no value match",
			map[string]string{"foo": "bar", "bar": "baz"},
			map[string]string{"foo": "quux"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := reflect.ValueOf(tt.a)
			y := reflect.ValueOf(tt.b)
			if got := mapContains(x, y); got != tt.want {
				t.Errorf("mapContains() = %v, want %v", got, tt.want)
			}
		})
	}
}
