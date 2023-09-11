package ovsdb

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpRowSerialization(t *testing.T) {
	var commentString = "this is a comment"
	tests := []struct {
		name     string
		op       Operation
		expected string
	}{
		{
			"insert",
			Operation{
				Op:    "insert",
				Table: "Bridge",
			},
			`{"op":"insert","table":"Bridge"}`,
		},
		{
			"insert with row",
			Operation{
				Op:    "insert",
				Table: "Bridge",
				Row:   Row(map[string]interface{}{"name": "docker-ovs"}),
			},
			`{"op":"insert","table":"Bridge","row":{"name":"docker-ovs"}}`,
		},
		{
			"comment",
			Operation{
				Op:      "comment",
				Comment: &commentString,
			},
			fmt.Sprintf(`{"op":"comment","comment":"%s"}`, commentString),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			str, err := json.Marshal(test.op)
			if err != nil {
				log.Fatal("serialization error:", err)
			}
			if string(str) != test.expected {
				t.Error("Expected: ", test.expected, "Got", string(str))
			}
		})
	}
}

func TestOpRowsSerialization(t *testing.T) {
	operation := Operation{
		Op:    "insert",
		Table: "Interface",
	}

	iface1 := Row(map[string]interface{}{
		"name":   "test-iface1",
		"mac":    "0000ffaaaa",
		"ofport": 1,
	})

	iface2 := Row(map[string]interface{}{
		"name":   "test-iface2",
		"mac":    "0000ffaabb",
		"ofport": 2,
	})
	operation.Rows = []Row{iface1, iface2}

	str, err := json.Marshal(operation)

	if err != nil {
		log.Fatal("serialization error:", err)
	}

	expected := `{"op":"insert","table":"Interface","rows":[{"mac":"0000ffaaaa","name":"test-iface1","ofport":1},{"mac":"0000ffaabb","name":"test-iface2","ofport":2}]}`

	if string(str) != expected {
		t.Error("Expected: ", expected, "Got", string(str))
	}
}

func TestValidateOvsSet(t *testing.T) {
	goSlice := []int{1, 2, 3, 4}
	oSet, err := NewOvsSet(goSlice)
	if err != nil {
		t.Error("Error creating OvsSet ", err)
	}
	data, err := json.Marshal(oSet)
	if err != nil {
		t.Error("Error Marshalling OvsSet", err)
	}
	expected := `["set",[1,2,3,4]]`
	if string(data) != expected {
		t.Error("Expected: ", expected, "Got", string(data))
	}
	// Negative condition test
	oSet, err = NewOvsSet(struct{ foo string }{})
	if err == nil {
		t.Error("OvsSet must fail for anything other than Slices and atomic types")
		t.Error("Got", oSet)
	}
}

func TestValidateOvsMap(t *testing.T) {
	myMap := make(map[int]string)
	myMap[1] = "hello"
	myMap[2] = "world"
	oMap, err := NewOvsMap(myMap)
	if err != nil {
		t.Error("Error creating OvsMap ", err)
	}
	data, err := json.Marshal(oMap)
	if err != nil {
		t.Error("Error Marshalling OvsMap", err)
	}
	expected1 := `["map",[[1,"hello"],[2,"world"]]]`
	expected2 := `["map",[[2,"world"],[1,"hello"]]]`
	if string(data) != expected1 && string(data) != expected2 {
		t.Error("Expected: ", expected1, "Got", string(data))
	}
	// Negative condition test
	integer := 5
	_, err = NewOvsMap(integer)
	if err == nil {
		t.Error("OvsMap must fail for anything other than Maps")
	}
}

func TestValidateUuid(t *testing.T) {
	uuid1 := UUID{"this is a bad uuid"}                   // Bad
	uuid2 := UUID{"alsoabaduuid"}                         // Bad
	uuid3 := UUID{"550e8400-e29b-41d4-a716-446655440000"} // Good
	uuid4 := UUID{"thishoul-dnot-pass-vali-dationchecks"} // Bad

	err := uuid1.validateUUID()

	if err == nil {
		t.Error(uuid1, " is not a valid UUID")
	}

	err = uuid2.validateUUID()

	if err == nil {
		t.Error(uuid2, " is not a valid UUID")
	}

	err = uuid3.validateUUID()

	if err != nil {
		t.Error(uuid3, " is a valid UUID")
	}

	err = uuid4.validateUUID()

	if err == nil {
		t.Error(uuid4, " is not a valid UUID")
	}
}

func TestNewUUID(t *testing.T) {
	uuid := UUID{"550e8400-e29b-41d4-a716-446655440000"}
	uuidStr, _ := json.Marshal(uuid)
	expected := `["uuid","550e8400-e29b-41d4-a716-446655440000"]`
	if string(uuidStr) != expected {
		t.Error("uuid is not correctly formatted")
	}
}

func TestNewNamedUUID(t *testing.T) {
	uuid := UUID{"test-uuid"}
	uuidStr, _ := json.Marshal(uuid)
	expected := `["named-uuid","test-uuid"]`
	if string(uuidStr) != expected {
		t.Error("uuid is not correctly formatted")
	}
}

func TestNewMutation(t *testing.T) {
	mutation := NewMutation("column", "+=", 1)
	mutationStr, _ := json.Marshal(mutation)
	expected := `["column","+=",1]`
	if string(mutationStr) != expected {
		t.Error("mutation is not correctly formatted")
	}
}

func TestOperationsMarshalUnmarshalJSON(t *testing.T) {
	in := []byte(`{"op":"mutate","table":"Open_vSwitch","mutations":[["bridges","insert",["named-uuid","foo"]]],"where":[["_uuid","==",["named-uuid","ovs"]]]}`)
	var op Operation
	err := json.Unmarshal(in, &op)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, OperationMutate, op.Op)
	assert.Equal(t, "Open_vSwitch", op.Table)
	assert.Equal(t, 1, len(op.Mutations))
	assert.Equal(t, Mutation{
		Column:  "bridges",
		Mutator: OperationInsert,
		Value:   UUID{GoUUID: "foo"},
	}, op.Mutations[0])
}

func TestOvsSliceToGoNotation(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    interface{}
		wantErr bool
	}{
		{
			"scalar value",
			"foo",
			"foo",
			false,
		},
		{
			"empty set",
			[]interface{}{"set", []interface{}{}},
			OvsSet{GoSet: []interface{}{}},
			false,
		},
		{
			"set",
			[]interface{}{"set", []interface{}{"foo", "bar", "baz"}},
			OvsSet{GoSet: []interface{}{"foo", "bar", "baz"}},
			false,
		},
		{
			"uuid set",
			[]interface{}{"set", []interface{}{[]interface{}{"named-uuid", "foo"}, []interface{}{"named-uuid", "bar"}}},
			OvsSet{GoSet: []interface{}{UUID{GoUUID: "foo"}, UUID{GoUUID: "bar"}}},
			false,
		},
		{
			"empty map",
			[]interface{}{"map", []interface{}{}},
			OvsMap{GoMap: map[interface{}]interface{}{}},
			false,
		},
		{
			"map",
			[]interface{}{"map", []interface{}{[]interface{}{"foo", "bar"}, []interface{}{"baz", "quux"}}},
			OvsMap{GoMap: map[interface{}]interface{}{"foo": "bar", "baz": "quux"}},
			false,
		},
		{
			"map uuid values",
			[]interface{}{"map", []interface{}{[]interface{}{"foo", []interface{}{"named-uuid", "bar"}}, []interface{}{"baz", []interface{}{"named-uuid", "quux"}}}},
			OvsMap{GoMap: map[interface{}]interface{}{"foo": UUID{GoUUID: "bar"}, "baz": UUID{GoUUID: "quux"}}},
			false,
		},
		{
			"map uuid keys",
			[]interface{}{"map", []interface{}{[]interface{}{[]interface{}{"named-uuid", "bar"}, "foo"}, []interface{}{[]interface{}{"named-uuid", "quux"}, "baz"}}},
			OvsMap{GoMap: map[interface{}]interface{}{UUID{GoUUID: "bar"}: "foo", UUID{GoUUID: "quux"}: "baz"}},
			false,
		},
		{
			"map uuid keys and values",
			[]interface{}{"map", []interface{}{[]interface{}{[]interface{}{"named-uuid", "bar"}, "foo"}, []interface{}{[]interface{}{"named-uuid", "quux"}, "baz"}}},
			OvsMap{GoMap: map[interface{}]interface{}{UUID{GoUUID: "bar"}: "foo", UUID{GoUUID: "quux"}: "baz"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ovsSliceToGoNotation(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			wantValue := reflect.ValueOf(tt.want)
			gotValue := reflect.ValueOf(got)
			assert.Equal(t, wantValue.Type(), gotValue.Type())
			assert.Equal(t, wantValue.Interface(), gotValue.Interface())
		})
	}
}
