package ovsdb

import (
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"
)

func benchmarkMap(m map[string]string, b *testing.B) {
	testMap, err := NewOvsMap(m)
	if err != nil {
		b.Fatal(err)
	}
	for n := 0; n < b.N; n++ {
		_, err := json.Marshal(testMap)
		if err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkMapMarshalJSON1(b *testing.B) { benchmarkMap(map[string]string{"foo": "bar"}, b) }
func BenchmarkMapMarshalJSON2(b *testing.B) {
	benchmarkMap(map[string]string{"foo": "bar", "baz": "quuz"}, b)
}
func BenchmarkMapMarshalJSON3(b *testing.B) {
	benchmarkMap(map[string]string{"foo": "bar", "baz": "quuz", "foobar": "foobaz"}, b)
}
func BenchmarkMapMarshalJSON5(b *testing.B) {
	benchmarkMap(map[string]string{"foo": "bar", "baz": "quuz", "foofoo": "foobar", "foobaz": "fooquuz", "barfoo": "barbar"}, b)
}
func BenchmarkMapMarshalJSON8(b *testing.B) {
	benchmarkMap(map[string]string{"foo": "bar", "baz": "quuz", "foofoo": "foobar", "foobaz": "fooquuz", "barfoo": "barbar", "barbaz": "barquuz", "bazfoo": "bazbar", "bazbaz": "bazquux"}, b)
}

func benchmarkMapUnmarshalJSON(data []byte, b *testing.B) {
	for n := 0; n < b.N; n++ {
		var m OvsMap
		err := json.Unmarshal(data, &m)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMapUnmarshalJSON1(b *testing.B) {
	benchmarkMapUnmarshalJSON([]byte(`[ "map", [["foo","bar"]]]`), b)
}

func BenchmarkMapUnmarshalJSON2(b *testing.B) {
	benchmarkMapUnmarshalJSON([]byte(`[ "map", [["foo","bar"],["baz", "quuz"]]]`), b)
}

func BenchmarkMapUnmarshalJSON3(b *testing.B) {
	benchmarkMapUnmarshalJSON([]byte(`[ "map", [["foo","bar"],["baz", "quuz"],["foofoo", "foobar"]]]`), b)
}

func BenchmarkMapUnmarshalJSON5(b *testing.B) {
	benchmarkMapUnmarshalJSON([]byte(`[ "map", [["foo","bar"],["baz", "quuz"],["foofoo", "foobar"],["foobaz", "fooquuz"], ["barfoo", "barbar"]]]`), b)
}

func BenchmarkMapUnmarshalJSON8(b *testing.B) {
	benchmarkMapUnmarshalJSON([]byte(`[ "map", [["foo","bar"],["baz", "quuz"],["foofoo", "foobar"],["foobaz", "fooquuz"], ["barfoo", "barbar"],["barbaz", "barquux"],["bazfoo", "bazbar"], ["bazbaz", "bazquux"]]]`), b)
}
