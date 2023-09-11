package json_test

import (
	"testing"

	"github.com/ovn-kubernetes/libovsdb/internal/json"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func benchmarkSetMarshalJSON(s []ovsdb.Operation, b *testing.B) {
	for n := 0; n < b.N; n++ {
		_, err := json.Marshal(s)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func operationDataset(n int) []ovsdb.Operation {
	var result []ovsdb.Operation
	oSet, _ := ovsdb.NewOvsSet([]string{"1.1.1.1"})
	var testOperations = []ovsdb.Operation{
		{
			Op:        ovsdb.OperationMutate,
			Table:     "Logical_Switch_Port",
			Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: oSet}},
			Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: "9ea1d0f7-e389-48a3-9563-fcf1a80fdb1e"}}},
		},
	}
	for range n {
		result = append(result, testOperations...)
	}
	return result
}

func BenchmarkSetMarshalJSONOperations1(b *testing.B) {
	benchmarkSetMarshalJSON(operationDataset(1), b)
}
func BenchmarkSetMarshalJSONOperations2(b *testing.B) {
	benchmarkSetMarshalJSON(operationDataset(2), b)
}
func BenchmarkSetMarshalJSONOperations3(b *testing.B) {
	benchmarkSetMarshalJSON(operationDataset(3), b)
}
func BenchmarkSetMarshalJSONOperations5(b *testing.B) {
	benchmarkSetMarshalJSON(operationDataset(5), b)
}
func BenchmarkSetMarshalJSONOperations8(b *testing.B) {
	benchmarkSetMarshalJSON(operationDataset(8), b)
}

func benchmarkSetUnmarshalJSON(data []byte, b *testing.B) {
	for n := 0; n < b.N; n++ {
		var s ovsdb.TableUpdate2
		err := json.Unmarshal(data, &s)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetUnmarshalJSONTableUpdate1(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`{"Port_Binding":{"77189bff-b4e3-4c15-9967-4518036b9a11":{"modify":{"virtual_parent":"06:73:24:4c:a3:97~2024-09-25 08:02:04.295"}}}}`), b)
}
func BenchmarkSetUnmarshalJSONTableUpdate2(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`{"Port_Binding":{"77189bff-b4e3-4c15-9967-4518036b9a11":{"modify":{"virtual_parent":"06:73:24:4c:a3:97~2024-09-25 08:02:04.295","tag":["set",[]]}}}}`), b)
}
