package ovsdb

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var testUUIDs = []string{
	"38d9fa08-8e97-4402-9347-a610773b91cb",
	"aab50e87-1410-4c44-8c43-58aed178c833",
	"445d365f-1e5b-44ee-86e7-41605858df83",
	"a132ac6f-8b95-483b-8595-5453703e0617",
	"5e617059-c157-47ff-a4ea-2bc3f163b198",
	"faceebeb-4b52-4721-a879-c9f70e3f58a6",
	"1ff23dbb-41d1-423f-acbc-94b06c508926",
	"7e191fdb-228d-4bf3-9db4-883c8705ac7e",
}

func benchmarkSetMarshalJSON(s interface{}, b *testing.B) {
	testSet, err := NewOvsSet(s)
	if err != nil {
		b.Fatal(err)
	}
	for n := 0; n < b.N; n++ {
		_, err := json.Marshal(testSet)
		if err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkSetMarshalJSONString1(b *testing.B) { benchmarkSetMarshalJSON("foo", b) }
func BenchmarkSetMarshalJSONString2(b *testing.B) {
	benchmarkSetMarshalJSON([]string{"foo", "bar"}, b)
}
func BenchmarkSetMarshalJSONString3(b *testing.B) {
	benchmarkSetMarshalJSON([]string{"foo", "bar", "baz"}, b)
}
func BenchmarkSetMarshalJSONString5(b *testing.B) {
	benchmarkSetMarshalJSON([]string{"foo", "bar", "baz", "quux", "foofoo"}, b)
}
func BenchmarkSetMarshalJSONString8(b *testing.B) {
	benchmarkSetMarshalJSON([]string{"foo", "bar", "baz", "quux", "foofoo", "foobar", "foobaz", "fooquux"}, b)
}

func BenchmarkSetMarshalJSONInt1(b *testing.B) { benchmarkSetMarshalJSON(1, b) }
func BenchmarkSetMarshalJSONInt2(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1, 2}, b)
}
func BenchmarkSetMarshalJSONInt3(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1, 2, 3}, b)
}
func BenchmarkSetMarshalJSONInt5(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1, 2, 3, 4, 5}, b)
}
func BenchmarkSetMarshalJSONInt8(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1, 2, 3, 4, 5, 6, 7, 8}, b)
}

func BenchmarkSetMarshalJSONFloat1(b *testing.B) { benchmarkSetMarshalJSON(1.0, b) }
func BenchmarkSetMarshalJSONFloat2(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1.0, 2.0}, b)
}
func BenchmarkSetMarshalJSONFloat3(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1.0, 2.0, 3.0}, b)
}
func BenchmarkSetMarshalJSONFloat5(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1.0, 2.0, 3.0, 4.0, 5.0}, b)
}
func BenchmarkSetMarshalJSONFloat8(b *testing.B) {
	benchmarkSetMarshalJSON([]int{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}, b)
}

func BenchmarkSetMarshalJSONUUID1(b *testing.B) { benchmarkSetMarshalJSON(testUUIDs[0], b) }
func BenchmarkSetMarshalJSONUUID2(b *testing.B) {
	benchmarkSetMarshalJSON(testUUIDs[0:2], b)
}
func BenchmarkSetMarshalJSONUUID3(b *testing.B) {
	benchmarkSetMarshalJSON(testUUIDs[0:3], b)
}
func BenchmarkSetMarshalJSONUUID5(b *testing.B) {
	benchmarkSetMarshalJSON(testUUIDs[0:5], b)
}
func BenchmarkSetMarshalJSONUUID8(b *testing.B) {
	benchmarkSetMarshalJSON(testUUIDs, b)
}

func benchmarkSetUnmarshalJSON(data []byte, b *testing.B) {
	for n := 0; n < b.N; n++ {
		var s OvsDataSet
		err := json.Unmarshal(data, &s)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetUnmarshalJSONString1(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`"foo"`), b)
}

func BenchmarkSetUnmarshalJSONString2(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`[ "set", ["foo","bar"] ]`), b)
}

func BenchmarkSetUnmarshalJSONString3(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`[ "set", ["foo","bar","baz"] ]`), b)
}

func BenchmarkSetUnmarshalJSONString5(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`[ "set", ["foo","bar","baz","quuz","foofoo"] ]`), b)
}

func BenchmarkSetUnmarshalJSONString8(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`[ "set", ["foo","bar","baz","quuz","foofoo","foobar","foobaz","fooquuz"] ]`), b)
}

func BenchmarkSetUnmarshalJSONInt1(b *testing.B) { benchmarkSetUnmarshalJSON([]byte("1"), b) }
func BenchmarkSetUnmarshalJSONInt2(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1, 2]]`), b)
}
func BenchmarkSetUnmarshalJSONInt3(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1, 2, 3]]`), b)
}
func BenchmarkSetUnmarshalJSONInt5(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1, 2, 3, 4, 5]]`), b)
}
func BenchmarkSetUnmarshalJSONInt8(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1, 2, 3, 4, 5, 6, 7, 8]]`), b)
}

func BenchmarkSetUnmarshalJSONFloat1(b *testing.B) { benchmarkSetUnmarshalJSON([]byte(`1.0`), b) }
func BenchmarkSetUnmarshalJSONFloat2(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1.0, 2.0]]`), b)
}
func BenchmarkSetUnmarshalJSONFloat3(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1.0, 2.0, 3.0]]`), b)
}
func BenchmarkSetUnmarshalJSONFloat5(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1.0, 2.0, 3.0, 4.0, 5.0]]`), b)
}
func BenchmarkSetUnmarshalJSONFloat8(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`["set", [1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0]]`), b)
}

func BenchmarkSetUnmarshalJSONUUID1(b *testing.B) {
	benchmarkSetUnmarshalJSON([]byte(`"`+testUUIDs[0]+`"`), b)
}
func BenchmarkSetUnmarshalJSONUUID2(b *testing.B) {
	benchmarkSetUnmarshalJSON(setify(testUUIDs[0:2]), b)
}
func BenchmarkSetUnmarshalJSONUUID3(b *testing.B) {
	benchmarkSetUnmarshalJSON(setify(testUUIDs[0:3]), b)
}
func BenchmarkSetUnmarshalJSONUUID5(b *testing.B) {
	benchmarkSetUnmarshalJSON(setify(testUUIDs[0:5]), b)
}
func BenchmarkSetUnmarshalJSONUUID8(b *testing.B) {
	benchmarkSetUnmarshalJSON(setify(testUUIDs), b)
}

func setify(i interface{}) []byte {
	var s []string
	iv := reflect.ValueOf(i)
	for j := 0; j < iv.Len(); j++ {
		s = append(s, fmt.Sprintf("%v", iv.Index(j)))
	}
	return []byte(fmt.Sprintf(`[ "set", [ "%s" ]]`, strings.Join(s, `","`)))
}
