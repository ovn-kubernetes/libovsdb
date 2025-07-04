package updates

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDifference(t *testing.T) {
	foo := "foo"
	bar := "bar"
	var null *string
	var nilMap map[string]string
	var nilSet []string
	tests := []struct {
		name     string
		a        any
		b        any
		expected any
	}{
		{
			"value, different",
			"foo",
			"bar",
			"bar",
		},
		{
			"value, equal",
			"foo",
			"foo",
			"foo",
		},
		{
			"pointer, different",
			&foo,
			&bar,
			&bar,
		},
		{
			"pointer, equal",
			&foo,
			&foo,
			&foo,
		},
		{
			"pointer, nil",
			&foo,
			null,
			null,
		},
		{
			"set, single element, different",
			[]string{"foo"},
			[]string{"bar"},
			[]string{"foo", "bar"},
		},
		{
			"set, single element, equal",
			[]string{"foo"},
			[]string{"foo"},
			nilSet,
		},
		{
			"set, different last element",
			[]string{"foo", "bar"},
			[]string{"foo", "foobar"},
			[]string{"bar", "foobar"},
		},
		{
			"set, different first element",
			[]string{"foo", "bar"},
			[]string{"foobar", "bar"},
			[]string{"foo", "foobar"},
		},
		{
			"set, multiple elements different",
			[]string{"foo", "bar", "foobar", "baz"},
			[]string{"qux", "foo", "quux", "baz", "waldo"},
			[]string{"bar", "foobar", "qux", "quux", "waldo"},
		},
		{
			"set, all elements different",
			[]string{"foo", "bar", "foobar", "baz"},
			[]string{"qux", "quux", "fred", "waldo"},
			[]string{"foo", "bar", "foobar", "baz", "qux", "quux", "fred", "waldo"},
		},
		{
			"set, multiple elements equal",
			[]string{"foo", "bar"},
			[]string{"foo", "bar"},
			nilSet,
		},
		{
			"map, different",
			map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo"},
			map[string]string{"bar": "baz", "qux": "fred", "foobar": "foobar"},
			map[string]string{"foo": "bar", "qux": "fred", "foobar": "foobar"},
		},
		{
			"map, equal",
			map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo"},
			map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo"},
			nilMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, _ := difference(tt.a, tt.b)
			switch v := tt.expected.(type) {
			case []string:
				if v != nil {
					assert.ElementsMatch(t, tt.expected, v)
					return
				}
				assert.Equal(t, tt.expected, diff)
			default:
				assert.Equal(t, tt.expected, diff)
			}
		})
	}
}

func BenchmarkSetDifference(t *testing.B) {
	l := 57000
	c, a := make([]string, l), make([]string, l)
	for i := 0; i < l; i++ {
		c[i] = fmt.Sprintf("foo%d", i)
	}
	b := []string{"bar", c[1000], c[20000], "foobar", c[55000], "baz"}
	t.StopTimer()
	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		copy(a, c)
		t.StartTimer()
		setDifference(a, b)
		t.StopTimer()
	}
}

func Test_applyDifference(t *testing.T) {
	type args struct {
		v any
		d any
	}
	tests := []struct {
		name     string
		args     args
		expected any
		changed  bool
	}{
		{
			name: "atomic, apply difference changes value",
			args: args{
				v: "foo",
				d: "bar",
			},
			expected: "bar",
			changed:  true,
		},
		{
			name: "atomic, apply difference does not change value",
			args: args{
				v: "foo",
				d: "foo",
			},
			expected: "foo",
			changed:  false,
		},
		{
			name: "set, apply difference changes value",
			args: args{
				v: []string{"foo"},
				d: []string{"bar"},
			},
			expected: []string{"foo", "bar"},
			changed:  true,
		},
		{
			name: "set, apply difference empties value",
			args: args{
				v: []string{"foo"},
				d: []string{"foo"},
			},
			expected: reflect.Zero(reflect.TypeOf([]string{})).Interface(),
			changed:  true,
		},
		{
			name: "set, apply empty difference",
			args: args{
				v: []string{"foo"},
				d: []string{},
			},
			expected: []string{"foo"},
			changed:  false,
		},
		{
			name: "map, apply difference changes value",
			args: args{
				v: map[string]string{"foo": "bar"},
				d: map[string]string{"fred": "waldo"},
			},
			expected: map[string]string{"foo": "bar", "fred": "waldo"},
			changed:  true,
		},
		{
			name: "map, apply difference empties value",
			args: args{
				v: map[string]string{"foo": "bar"},
				d: map[string]string{"foo": "bar"},
			},
			expected: reflect.Zero(reflect.TypeOf(map[string]string{})).Interface(),
			changed:  true,
		},
		{
			name: "map, apply empty difference",
			args: args{
				v: map[string]string{"foo": "bar"},
				d: map[string]string{},
			},
			expected: map[string]string{"foo": "bar"},
			changed:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := applyDifference(tt.args.v, tt.args.d)
			switch tt.expected.(type) {
			case []string:
				assert.ElementsMatch(t, tt.expected, result)
			default:
				assert.Equal(t, tt.expected, result)
			}
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func Test_mergeMapDifference(t *testing.T) {
	type args struct {
		o any
		a any
		b any
	}
	tests := []struct {
		name     string
		args     args
		expected any
		changed  bool
	}{
		{
			name: "original nil",
			args: args{
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "foobar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred", "foobar": "foobar"},
			changed:  true,
		},
		{
			name: "original empty",
			args: args{
				o: map[string]string{},
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "foobar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred", "foobar": "foobar"},
			changed:  true,
		},
		{
			name: "key value updated back to the original value",
			args: args{
				o: map[string]string{"foobar": "foobar"},
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo", "foobar": "bar"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "foobar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred"},
			changed:  true,
		},
		{
			name: "key value updated and then removed",
			args: args{
				o: map[string]string{"foobar": "foobar"},
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo", "foobar": "bar"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "bar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred", "foobar": "foobar"},
			changed:  true,
		},
		{
			name: "key value removed and then added to the original value",
			args: args{
				o: map[string]string{"foobar": "foobar"},
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo", "foobar": "foobar"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "foobar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred"},
			changed:  true,
		},
		{
			name: "key removed and then added to a different value",
			args: args{
				o: map[string]string{"foobar": "foobar"},
				a: map[string]string{"foo": "bar", "bar": "baz", "qux": "waldo", "foobar": "foobar"},
				b: map[string]string{"bar": "baz", "qux": "fred", "foobar": "bar"},
			},
			expected: map[string]string{"foo": "bar", "qux": "fred", "foobar": "bar"},
			changed:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := mergeMapDifference(tt.args.o, tt.args.a, tt.args.b)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.changed, changed)
		})
	}
}
