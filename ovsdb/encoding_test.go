package ovsdb

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var validUUIDStr0 = `00000000-0000-0000-0000-000000000000`
var validUUIDStr1 = `11111111-1111-1111-1111-111111111111`
var validUUID0 = UUID{GoUUID: validUUIDStr0}
var validUUID1 = UUID{GoUUID: validUUIDStr1}

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected string
	}{
		{
			"empty map",
			map[string]string{},
			`["map",[]]`,
		},
		{
			"single element map",
			map[string]string{`v0`: `k0`},
			`["map",[["v0","k0"]]]`,
		},
		{
			"multiple element map",
			map[string]string{`v0`: `k0`, `v1`: `k1`},
			`["map",[["v0","k0"],["v1","k1"]]]`,
		},
	}
	for _, tt := range tests {
		m, err := NewOvsMap(tt.input)
		require.NoError(t, err)
		jsonStr, err := json.Marshal(m)
		require.NoError(t, err)
		// Compare unmarshalled data since the order of the elements of the map might not
		// have been preserved
		var expectedSlice []any
		var jsonSlice []any
		err = json.Unmarshal([]byte(tt.expected), &expectedSlice)
		require.NoError(t, err)
		err = json.Unmarshal(jsonStr, &jsonSlice)
		require.NoError(t, err)
		assert.Equal(t, expectedSlice[0], jsonSlice[0], "they should both start with 'map'")
		assert.ElementsMatch(t, expectedSlice[1].([]any), jsonSlice[1].([]any), "they should have the same elements\n")

		var res OvsMap
		err = json.Unmarshal(jsonStr, &res)
		require.NoError(t, err)
		assert.Equal(t, m, res, "they should be equal\n")
	}
}

func TestSet(t *testing.T) {
	var x *int
	var y *string
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			"empty set",
			[]string{},
			`["set",[]]`,
		},
		{
			"string",
			`aa`,
			`"aa"`,
		},
		{
			"bool",
			false,
			`false`,
		},
		{
			"float 64",
			float64(10),
			`10`,
		},
		{
			"float",
			10.2,
			`10.2`,
		},
		{
			"string slice",
			[]string{`aa`},
			`"aa"`,
		},
		{
			"string slice with multiple elements",
			[]string{`aa`, `bb`},
			`["set",["aa","bb"]]`,
		},
		{
			"float slice",
			[]float64{10.2, 15.4},
			`["set",[10.2,15.4]]`,
		},
		{
			"empty uuid",
			[]UUID{},
			`["set",[]]`,
		},
		{
			"uuid",
			UUID{GoUUID: `aa`},
			`["named-uuid","aa"]`,
		},
		{
			"uuid slice single element",
			[]UUID{{GoUUID: `aa`}},
			`["named-uuid","aa"]`,
		},
		{
			"uuid slice multiple elements",
			[]UUID{{GoUUID: `aa`}, {GoUUID: `bb`}},
			`["set",[["named-uuid","aa"],["named-uuid","bb"]]]`,
		},
		{
			"valid uuid",
			validUUID0,
			fmt.Sprintf(`["uuid","%v"]`, validUUIDStr0),
		},
		{
			"valid uuid set single element",
			[]UUID{validUUID0},
			fmt.Sprintf(`["uuid","%v"]`, validUUIDStr0),
		},
		{
			"valid uuid set multiple elements",
			[]UUID{validUUID0, validUUID1},
			fmt.Sprintf(`["set",[["uuid","%v"],["uuid","%v"]]]`, validUUIDStr0, validUUIDStr1),
		},
		{
			name:     "nil pointer of valid *int type",
			input:    x,
			expected: `["set",[]]`,
		},
		{
			name:     "nil pointer of valid *string type",
			input:    y,
			expected: `["set",[]]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := NewOvsSet(tt.input)
			require.NoError(t, err)
			jsonStr, err := json.Marshal(set)
			require.NoError(t, err)
			assert.JSONEqf(t, tt.expected, string(jsonStr), "they should be equal\n")

			var res OvsSet
			err = json.Unmarshal(jsonStr, &res)
			require.NoError(t, err)
			assert.Equal(t, set.GoSet, res.GoSet, "they should have the same elements\n")
		})
	}
}
