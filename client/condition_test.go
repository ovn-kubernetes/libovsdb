package client

import (
	"fmt"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
)

func TestEqualityConditional(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIds: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp3",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &trueVal,
		},
	}
	lspcache := map[string]model.Model{}
	for i := range lspcacheList {
		lspcache[lspcacheList[i].(*testLogicalSwitchPort).UUID] = lspcacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspcache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		models    []model.Model
		condition [][]ovsdb.Condition
		matches   map[string]model.Model
		err       bool
	}{
		{
			name: "by uuid",
			models: []model.Model{
				&testLogicalSwitchPort{UUID: aUUID0, Name: "different"},
			},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					}}},
			matches: map[string]model.Model{aUUID0: lspcacheList[0]},
		},
		{
			name: "by uuids",
			models: []model.Model{
				&testLogicalSwitchPort{UUID: aUUID0, Name: "different"},
				&testLogicalSwitchPort{UUID: aUUID1, Name: "different2"},
			},
			condition: [][]ovsdb.Condition{
				{{
					Column:   "_uuid",
					Function: ovsdb.ConditionEqual,
					Value:    ovsdb.UUID{GoUUID: aUUID0},
				}},
				{{
					Column:   "_uuid",
					Function: ovsdb.ConditionEqual,
					Value:    ovsdb.UUID{GoUUID: aUUID1},
				}},
			},
			matches: map[string]model.Model{
				aUUID0: lspcacheList[0],
				aUUID1: lspcacheList[1],
			},
		},
		{
			name: "by index with cache",
			models: []model.Model{
				&testLogicalSwitchPort{Name: "lsp1"},
				&testLogicalSwitchPort{Name: "lsp2"},
			},
			condition: [][]ovsdb.Condition{
				{{
					Column:   "_uuid",
					Function: ovsdb.ConditionEqual,
					Value:    ovsdb.UUID{GoUUID: aUUID1},
				}},
				{{
					Column:   "_uuid",
					Function: ovsdb.ConditionEqual,
					Value:    ovsdb.UUID{GoUUID: aUUID2},
				}},
			},
			matches: map[string]model.Model{
				aUUID1: lspcacheList[1],
				aUUID2: lspcacheList[2],
			},
		},
		{
			name: "by index with no cache",
			models: []model.Model{
				&testLogicalSwitchPort{Name: "foo"},
				&testLogicalSwitchPort{Name: "123"},
			},
			condition: [][]ovsdb.Condition{
				{{
					Column:   "name",
					Function: ovsdb.ConditionEqual,
					Value:    "foo",
				}},
				{{
					Column:   "name",
					Function: ovsdb.ConditionEqual,
					Value:    "123",
				}},
			},
		},
		{
			name: "by non index",
			models: []model.Model{
				&testLogicalSwitchPort{ExternalIds: map[string]string{"foo": "baz"}},
			},
			err: true,
		},
		{
			name: "by non index multiple models",
			models: []model.Model{
				&testLogicalSwitchPort{ExternalIds: map[string]string{"foo": "baz"}},
				&testLogicalSwitchPort{ExternalIds: map[string]string{"foo": "123"}},
			},
			err: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Equality Conditional: %s", tt.name), func(t *testing.T) {
			cond, err := newEqualityConditional("Logical_Switch_Port", tcache, tt.models)
			assert.Nil(t, err)
			matches, err := cond.Matches()
			assert.Nil(t, err)
			assert.Equal(t, tt.matches, matches)
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.condition, generated)
			}
		})
	}
}

func TestPredicateConditional(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIds: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp3",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &trueVal,
		},
	}
	lspcache := map[string]model.Model{}
	for i := range lspcacheList {
		lspcache[lspcacheList[i].(*testLogicalSwitchPort).UUID] = lspcacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspcache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		predicate interface{}
		condition [][]ovsdb.Condition
		matches   map[string]model.Model
		err       bool
	}{
		{
			name: "simple value comparison",
			predicate: func(lsp *testLogicalSwitchPort) bool {
				return lsp.UUID == aUUID0
			},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					}}},
			matches: map[string]model.Model{aUUID0: lspcacheList[0]},
		},
		{
			name: "by random field",
			predicate: func(lsp *testLogicalSwitchPort) bool {
				return lsp.Enabled != nil && *lsp.Enabled == false
			},
			condition: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					}},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID2},
					}}},
			matches: map[string]model.Model{
				aUUID1: lspcacheList[1],
				aUUID2: lspcacheList[2],
			},
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Predicate Conditional: %s", tt.name), func(t *testing.T) {
			cond, err := newPredicateConditional("Logical_Switch_Port", tcache, tt.predicate)
			assert.Nil(t, err)
			matches, err := cond.Matches()
			assert.Nil(t, err)
			assert.Equal(t, tt.matches, matches)
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.condition, generated)
			}
		})
	}
}

func TestExplicitConditionalWithNoCache(t *testing.T) {
	lspcache := map[string]model.Model{}
	testData := cache.Data{
		"Logical_Switch_Port": lspcache,
	}
	tcache := apiTestCache(t, testData)

	testObj := &testLogicalSwitchPort{}

	test := []struct {
		name   string
		args   []model.Condition
		result [][]ovsdb.Condition
		all    bool
		err    bool
	}{
		{
			name: "inequality comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "lsp0",
					}}},
		},
		{
			name: "inequality comparison all",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "lsp0",
					}}},
			all: true,
		},
		{
			name: "map comparison",
			args: []model.Condition{
				{
					Field:    &testObj.ExternalIds,
					Function: ovsdb.ConditionIncludes,
					Value:    map[string]string{"foo": "baz"},
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "external_ids",
						Function: ovsdb.ConditionIncludes,
						Value:    testOvsMap(t, map[string]string{"foo": "baz"}),
					}}},
		},
		{
			name: "set comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "enabled",
						Function: ovsdb.ConditionEqual,
						Value:    testOvsSet(t, &trueVal),
					}}},
		},
		{
			name: "multiple conditions",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "enabled",
						Function: ovsdb.ConditionEqual,
						Value:    testOvsSet(t, &trueVal),
					}},
				{
					{
						Column:   "name",
						Function: ovsdb.ConditionNotEqual,
						Value:    "foo",
					}}},
		},
		{
			name: "multiple conditions all",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{{
				{
					Column:   "enabled",
					Function: ovsdb.ConditionEqual,
					Value:    testOvsSet(t, &trueVal),
				},
				{
					Column:   "name",
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				}}},
			all: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Explicit Conditional with no cache: %s", tt.name), func(t *testing.T) {
			cond, err := newExplicitConditional("Logical_Switch_Port", tcache, tt.all, testObj, tt.args...)
			assert.Nil(t, err)
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.result, generated)
			}
		})
	}
}

func TestExplicitConditionalWithCache(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIds: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIds: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp3",
			ExternalIds: map[string]string{"foo": "baz"},
			Enabled:     &trueVal,
		},
	}
	lspcache := map[string]model.Model{}
	for i := range lspcacheList {
		lspcache[lspcacheList[i].(*testLogicalSwitchPort).UUID] = lspcacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspcache,
	}
	tcache := apiTestCache(t, testData)

	testObj := &testLogicalSwitchPort{}

	test := []struct {
		name   string
		args   []model.Condition
		result [][]ovsdb.Condition
		all    bool
		err    bool
	}{
		{
			name: "inequality comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID2},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
		},
		{
			name: "inequality comparison all",
			args: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "lsp0",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID2},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
			all: true,
		},
		{
			name: "map comparison",
			args: []model.Condition{
				{
					Field:    &testObj.ExternalIds,
					Function: ovsdb.ConditionIncludes,
					Value:    map[string]string{"foo": "baz"},
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
		},
		{
			name: "set comparison",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
		},
		{
			name: "multiple conditions",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID1},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID2},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
		},
		{
			name: "multiple conditions all",
			args: []model.Condition{
				{
					Field:    &testObj.Enabled,
					Function: ovsdb.ConditionEqual,
					Value:    &trueVal,
				},
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionNotEqual,
					Value:    "foo",
				},
			},
			result: [][]ovsdb.Condition{
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID0},
					},
				},
				{
					{
						Column:   "_uuid",
						Function: ovsdb.ConditionEqual,
						Value:    ovsdb.UUID{GoUUID: aUUID3},
					},
				},
			},
			all: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("Explicit Conditional with cache: %s", tt.name), func(t *testing.T) {
			cond, err := newExplicitConditional("Logical_Switch_Port", tcache, tt.all, testObj, tt.args...)
			assert.Nil(t, err)
			generated, err := cond.Generate()
			if tt.err {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.result, generated)
			}
		})
	}
}
