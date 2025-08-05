package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/cache"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	trueVal  = true
	falseVal = false
	one      = 1
	six      = 6
)

var discardLogger = logr.Discard()

func TestAPIListSimple(t *testing.T) {

	lscacheList := []model.Model{
		&testLogicalSwitch{
			UUID:        aUUID0,
			Name:        "ls0",
			ExternalIDs: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitch{
			UUID:        aUUID1,
			Name:        "ls1",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID2,
			Name:        "ls2",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID3,
			Name:        "ls4",
			ExternalIDs: map[string]string{"foo": "baz"},
			Ports:       []string{"port0", "port1"},
		},
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].(*testLogicalSwitch).UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch": lscache,
	}
	tcache := apiTestCache(t, testData)
	test := []struct {
		name       string
		initialCap int
		resultCap  int
		resultLen  int
		content    []model.Model
		err        bool
	}{
		{
			name:       "full",
			initialCap: 0,
			resultCap:  len(lscache),
			resultLen:  len(lscacheList),
			content:    lscacheList,
			err:        false,
		},
		{
			name:       "single",
			initialCap: 1,
			resultCap:  1,
			resultLen:  1,
			content:    lscacheList,
			err:        false,
		},
		{
			name:       "multiple",
			initialCap: 2,
			resultCap:  2,
			resultLen:  2,
			content:    lscacheList,
			err:        false,
		},
	}
	hasDups := func(a any) bool {
		l := map[string]struct{}{}
		switch v := a.(type) {
		case []testLogicalSwitch:
			for _, i := range v {
				if _, ok := l[i.Name]; ok {
					return ok
				}
			}
		case []*testLogicalSwitch:
			for _, i := range v {
				if _, ok := l[i.Name]; ok {
					return ok
				}
			}
		}
		return false
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiList: %s", tt.name), func(t *testing.T) {
			// test List with a pointer to a slice of Models
			var result []*testLogicalSwitch
			if tt.initialCap != 0 {
				result = make([]*testLogicalSwitch, 0, tt.initialCap)
			}
			api := newAPI(tcache, &discardLogger, false)
			err := api.List(context.Background(), &result)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Lenf(t, result, tt.resultLen, "Length should match expected")
				assert.Equal(t, cap(result), tt.resultCap, "Capability should match expected")
				assert.Subsetf(t, tt.content, result, "Result should be a subset of expected")
				assert.False(t, hasDups(result), "Result should have no duplicates")
			}

			// test List with a pointer to a slice of structs
			var resultWithNoPtr []testLogicalSwitch
			if tt.initialCap != 0 {
				resultWithNoPtr = make([]testLogicalSwitch, 0, tt.initialCap)
			}
			contentNoPtr := make([]testLogicalSwitch, 0, len(tt.content))
			for i := range tt.content {
				contentNoPtr = append(contentNoPtr, *tt.content[i].(*testLogicalSwitch))
			}
			err = api.List(context.Background(), &resultWithNoPtr)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Lenf(t, result, tt.resultLen, "Length should match expected")
				assert.Equal(t, cap(result), tt.resultCap, "Capability should match expected")
				assert.Subsetf(t, contentNoPtr, resultWithNoPtr, "Result should be a subset of expected")
				assert.False(t, hasDups(resultWithNoPtr), "Result should have no duplicates")
			}

		})
	}

	t.Run("ApiList: Error wrong type", func(t *testing.T) {
		var result []string
		api := newAPI(tcache, &discardLogger, false)
		err := api.List(context.Background(), &result)
		require.Error(t, err)
	})

	t.Run("ApiList: Type Selection", func(t *testing.T) {
		var result []testLogicalSwitchPort
		api := newAPI(tcache, &discardLogger, false)
		err := api.List(context.Background(), &result)
		require.NoError(t, err)
		assert.Empty(t, result, "Should be empty since cache is empty")
	})

	t.Run("ApiList: Empty List", func(t *testing.T) {
		result := []testLogicalSwitch{}
		api := newAPI(tcache, &discardLogger, false)
		err := api.List(context.Background(), &result)
		require.NoError(t, err)
		assert.Len(t, result, len(lscacheList))
	})

	t.Run("ApiList: fails if conditional is an error", func(t *testing.T) {
		result := []testLogicalSwitch{}
		api := newConditionalAPI(tcache, newErrorConditional(fmt.Errorf("error")), &discardLogger, false)
		err := api.List(context.Background(), &result)
		require.Error(t, err)
	})
}

func TestAPIListPredicate(t *testing.T) {
	lscacheList := []model.Model{
		&testLogicalSwitch{
			UUID:        aUUID0,
			Name:        "ls0",
			ExternalIDs: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitch{
			UUID:        aUUID1,
			Name:        "magicLs1",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID2,
			Name:        "ls2",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
		&testLogicalSwitch{
			UUID:        aUUID3,
			Name:        "magicLs2",
			ExternalIDs: map[string]string{"foo": "baz"},
			Ports:       []string{"port0", "port1"},
		},
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].(*testLogicalSwitch).UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch": lscache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		predicate any
		content   []model.Model
		err       bool
	}{
		{
			name: "none",
			predicate: func(_ *testLogicalSwitch) bool {
				return false
			},
			content: []model.Model{},
			err:     false,
		},
		{
			name: "all",
			predicate: func(_ *testLogicalSwitch) bool {
				return true
			},
			content: lscacheList,
			err:     false,
		},
		{
			name: "nil function must fail",
			err:  true,
		},
		{
			name: "arbitrary condition",
			predicate: func(t *testLogicalSwitch) bool {
				return strings.HasPrefix(t.Name, "magic")
			},
			content: []model.Model{lscacheList[1], lscacheList[3]},
			err:     false,
		},
		{
			name: "error wrong type",
			predicate: func(_ testLogicalSwitch) string {
				return "foo"
			},
			err: true,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiListPredicate: %s", tt.name), func(t *testing.T) {
			var result []*testLogicalSwitch
			api := newAPI(tcache, &discardLogger, false)
			cond := api.WhereCache(tt.predicate)
			err := cond.List(context.Background(), &result)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.content, result, "Content should match")
			}

		})
	}
}

func TestAPIListWhereConditions(t *testing.T) {
	lscacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID: aUUID0,
			Name: "lsp0",
			Type: "",
		},
		&testLogicalSwitchPort{
			UUID: aUUID1,
			Name: "lsp1",
			Type: "router",
		},
		&testLogicalSwitchPort{
			UUID: aUUID2,
			Name: "lsp2",
			Type: "router",
		},
		&testLogicalSwitchPort{
			UUID: aUUID3,
			Name: "lsp3",
			Type: "localnet",
		},
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].(*testLogicalSwitchPort).UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lscache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		desc       string
		matchNames []string
		matchTypes []string
		matchAll   bool
		result     []model.Model
	}{
		{
			desc:       "any conditions",
			matchNames: []string{"lsp0"},
			matchTypes: []string{"router"},
			matchAll:   false,
			result:     lscacheList[0:3],
		},
		{
			desc:       "all conditions",
			matchNames: []string{"lsp1"},
			matchTypes: []string{"router"},
			matchAll:   true,
			result:     lscacheList[1:2],
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("TestAPIListWhereConditions: %s", tt.desc), func(t *testing.T) {
			var result []*testLogicalSwitchPort
			api := newAPI(tcache, &discardLogger, false)
			testObj := &testLogicalSwitchPort{}
			conds := []model.Condition{}
			for _, name := range tt.matchNames {
				cond := model.Condition{Field: &testObj.Name, Function: ovsdb.ConditionEqual, Value: name}
				conds = append(conds, cond)
			}
			for _, atype := range tt.matchTypes {
				cond := model.Condition{Field: &testObj.Type, Function: ovsdb.ConditionEqual, Value: atype}
				conds = append(conds, cond)
			}
			var capi ConditionalAPI
			if tt.matchAll {
				capi = api.WhereAll(testObj, conds...)
			} else {
				capi = api.WhereAny(testObj, conds...)
			}
			err := capi.List(context.Background(), &result)
			require.NoError(t, err)
			assert.ElementsMatchf(t, tt.result, result, "Content should match")
		})
	}
}

func TestAPIListFields(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIDs: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "magiclsp1",
			ExternalIDs: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIDs: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "magiclsp2",
			ExternalIDs: map[string]string{"foo": "baz"},
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
		name    string
		fields  []any
		prepare func(*testLogicalSwitchPort)
		content []model.Model
	}{
		{
			name:    "No match",
			prepare: func(_ *testLogicalSwitchPort) {},
			content: []model.Model{},
		},
		{
			name: "List unique by UUID",
			prepare: func(t *testLogicalSwitchPort) {
				t.UUID = aUUID0
			},
			content: []model.Model{lspcache[aUUID0]},
		},
		{
			name: "List unique by Index",
			prepare: func(t *testLogicalSwitchPort) {
				t.Name = "lsp2"
			},
			content: []model.Model{lspcache[aUUID2]},
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiListFields: %s", tt.name), func(t *testing.T) {
			var result []*testLogicalSwitchPort
			// Clean object
			testObj := testLogicalSwitchPort{}
			tt.prepare(&testObj)
			api := newAPI(tcache, &discardLogger, false)
			err := api.Where(&testObj).List(context.Background(), &result)
			require.NoError(t, err)
			assert.ElementsMatchf(t, tt.content, result, "Content should match")
		})
	}

	t.Run("ApiListFields: Wrong table", func(t *testing.T) {
		var result []testLogicalSwitchPort
		api := newAPI(tcache, &discardLogger, false)
		obj := testLogicalSwitch{
			UUID: aUUID0,
		}

		err := api.Where(&obj).List(context.Background(), &result)
		require.Error(t, err)
	})
}

func TestAPIListMulti(t *testing.T) {
	lspcacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			ExternalIDs: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "magiclsp1",
			ExternalIDs: map[string]string{"foo": "baz"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			ExternalIDs: map[string]string{"unique": "id"},
			Enabled:     &falseVal,
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "magiclsp2",
			ExternalIDs: map[string]string{"foo": "baz"},
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
		name    string
		models  []model.Model
		matches []model.Model
		err     bool
	}{
		{
			name: "No match",
			models: []model.Model{
				&testLogicalSwitchPort{UUID: "asdfasdfaf"},
				&testLogicalSwitchPort{UUID: "ghghghghgh"},
			},
			matches: []model.Model{},
			err:     false,
		},
		{
			name: "One match",
			models: []model.Model{
				&testLogicalSwitchPort{UUID: aUUID0},
				&testLogicalSwitchPort{UUID: "ghghghghgh"},
			},
			matches: []model.Model{lspcache[aUUID0]},
			err:     false,
		},
		{
			name: "Mismatched models",
			models: []model.Model{
				&testLogicalSwitchPort{UUID: aUUID0},
				&testLogicalSwitch{UUID: "ghghghghgh"},
			},
			matches: []model.Model{},
			err:     true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			var result []*testLogicalSwitchPort
			api := newAPI(tcache, &discardLogger, false)
			err := api.Where(tt.models...).List(context.Background(), &result)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.matches, result, "Content should match")
			}
		})
	}
}

func TestConditionFromFunc(t *testing.T) {
	test := []struct {
		name string
		arg  any
		err  bool
	}{
		{
			name: "wrong function must fail",
			arg: func(_ string) bool {
				return false
			},
			err: true,
		},
		{
			name: "wrong function must fail2 ",
			arg: func(_ *testLogicalSwitch) string {
				return "foo"
			},
			err: true,
		},
		{
			name: "correct func should succeed",
			arg: func(_ *testLogicalSwitch) bool {
				return true
			},
			err: false,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("conditionFromFunc: %s", tt.name), func(t *testing.T) {
			cache := apiTestCache(t, nil)
			apiIface := newAPI(cache, &discardLogger, false)
			condition := apiIface.(api).conditionFromFunc(tt.arg)
			if tt.err {
				assert.IsType(t, &errorConditional{}, condition)
			} else {
				assert.IsType(t, &predicateConditional{}, condition)
			}
		})
	}
}

func TestConditionFromModel(t *testing.T) {
	var testObj testLogicalSwitch
	test := []struct {
		name   string
		models []model.Model
		conds  []model.Condition
		err    bool
	}{
		{
			name: "wrong model must fail",
			models: []model.Model{
				&struct{ a string }{},
			},
			err: true,
		},
		{
			name: "wrong condition must fail",
			models: []model.Model{
				&struct {
					a string `ovsdb:"_uuid"`
				}{},
			},
			conds: []model.Condition{{Field: "foo"}},
			err:   true,
		},
		{
			name: "correct model must succeed",
			models: []model.Model{
				&testLogicalSwitch{},
			},
			err: false,
		},
		{
			name: "correct models must succeed",
			models: []model.Model{
				&testLogicalSwitch{},
				&testLogicalSwitch{},
			},
			err: false,
		},
		{
			name:   "correct model with valid condition must succeed",
			models: []model.Model{&testObj},
			conds: []model.Condition{
				{
					Field:    &testObj.Name,
					Function: ovsdb.ConditionEqual,
					Value:    "foo",
				},
				{
					Field:    &testObj.Ports,
					Function: ovsdb.ConditionIncludes,
					Value:    []string{"foo"},
				},
			},
			err: false,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("conditionFromModels: %s", tt.name), func(t *testing.T) {
			cache := apiTestCache(t, nil)
			apiIface := newAPI(cache, &discardLogger, false)
			var condition Conditional
			if len(tt.conds) > 0 {
				condition = apiIface.(api).conditionFromExplicitConditions(true, tt.models[0], tt.conds...)
			} else {
				condition = apiIface.(api).conditionFromModels(tt.models)
			}
			if tt.err {
				assert.IsType(t, &errorConditional{}, condition)
			} else {
				if len(tt.conds) > 0 {
					assert.IsType(t, &explicitConditional{}, condition)
				} else {
					assert.IsType(t, &equalityConditional{}, condition)
				}

			}
		})
	}
}

func TestAPIGet(t *testing.T) {
	lsCacheList := []model.Model{}
	lspCacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp0",
			Type:        "foo",
			ExternalIDs: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp1",
			Type:        "bar",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
	}
	lsCache := map[string]model.Model{}
	lspCache := map[string]model.Model{}
	for i := range lsCacheList {
		lsCache[lsCacheList[i].(*testLogicalSwitch).UUID] = lsCacheList[i]
	}
	for i := range lspCacheList {
		lspCache[lspCacheList[i].(*testLogicalSwitchPort).UUID] = lspCacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch":      lsCache,
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name    string
		prepare func(model.Model)
		result  model.Model
		err     bool
	}{
		{
			name: "empty",
			prepare: func(_ model.Model) {
			},
			err: true,
		},
		{
			name: "non_existing",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).Name = "foo"
			},
			err: true,
		},
		{
			name: "by UUID",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).UUID = aUUID3
			},
			result: lspCacheList[1],
			err:    false,
		},
		{
			name: "by name",
			prepare: func(m model.Model) {
				m.(*testLogicalSwitchPort).Name = "lsp0"
			},
			result: lspCacheList[0],
			err:    false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiGet: %s", tt.name), func(t *testing.T) {
			var result testLogicalSwitchPort
			tt.prepare(&result)
			api := newAPI(tcache, &discardLogger, false)
			err := api.Get(context.Background(), &result)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equalf(t, tt.result, &result, "Result should match")
			}
		})
	}
}

func TestAPICreate(t *testing.T) {
	lsCacheList := []model.Model{}
	lspCacheList := []model.Model{
		&testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp0",
			Type:        "foo",
			ExternalIDs: map[string]string{"foo": "bar"},
		},
		&testLogicalSwitchPort{
			UUID:        aUUID3,
			Name:        "lsp1",
			Type:        "bar",
			ExternalIDs: map[string]string{"foo": "baz"},
		},
	}
	lsCache := map[string]model.Model{}
	lspCache := map[string]model.Model{}
	for i := range lsCacheList {
		lsCache[lsCacheList[i].(*testLogicalSwitch).UUID] = lsCacheList[i]
	}
	for i := range lspCacheList {
		lspCache[lspCacheList[i].(*testLogicalSwitchPort).UUID] = lspCacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch":      lsCache,
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	rowFoo := ovsdb.Row(map[string]any{"name": "foo"})
	rowBar := ovsdb.Row(map[string]any{"name": "bar"})
	test := []struct {
		name   string
		input  []model.Model
		result []ovsdb.Operation
		err    bool
	}{
		{
			name:  "empty",
			input: []model.Model{&testLogicalSwitch{}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      ovsdb.Row{},
				UUIDName: "",
			}},
			err: false,
		},
		{
			name: "With some values",
			input: []model.Model{&testLogicalSwitch{
				Name: "foo",
			}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowFoo,
				UUIDName: "",
			}},
			err: false,
		},
		{
			name: "With named UUID ",
			input: []model.Model{&testLogicalSwitch{
				UUID: "foo",
			}},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      ovsdb.Row{},
				UUIDName: "foo",
			}},
			err: false,
		},
		{
			name: "Multiple",
			input: []model.Model{
				&testLogicalSwitch{
					UUID: "fooUUID",
					Name: "foo",
				},
				&testLogicalSwitch{
					UUID: "barUUID",
					Name: "bar",
				},
			},
			result: []ovsdb.Operation{{
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowFoo,
				UUIDName: "fooUUID",
			}, {
				Op:       "insert",
				Table:    "Logical_Switch",
				Row:      rowBar,
				UUIDName: "barUUID",
			}},
			err: false,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiCreate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache, &discardLogger, false)
			op, err := api.Create(tt.input...)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equalf(t, tt.result, op, "ovsdb.Operation should match")
			}
		})
	}
}

func TestAPIMutate(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	testObj := testLogicalSwitchPort{}
	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		model     model.Model
		mutations []model.Mutation
		init      map[string]model.Model
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "select by UUID addElement to set",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					UUID: aUUID0,
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.Addresses,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   []string{"1.1.1.1"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: testOvsSet(t, []string{"1.1.1.1"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select multiple by UUID addElement to set",
			condition: func(a API) ConditionalAPI {
				return a.Where(
					&testLogicalSwitchPort{UUID: aUUID0},
					&testLogicalSwitchPort{UUID: aUUID1},
					&testLogicalSwitchPort{UUID: aUUID2},
				)
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.Addresses,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   []string{"2.2.2.2"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: testOvsSet(t, []string{"2.2.2.2"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: testOvsSet(t, []string{"2.2.2.2"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "addresses", Mutator: ovsdb.MutateOperationInsert, Value: testOvsSet(t, []string{"2.2.2.2"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
			},
			err: false,
		},
		{
			name: "select by name delete element from map with cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp2",
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIDs,
					Mutator: ovsdb.MutateOperationDelete,
					Value:   []string{"foo"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationDelete, Value: testOvsSet(t, []string{"foo"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
			},
			err: false,
		},
		{
			name: "select by name delete element from map with no cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "foo",
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIDs,
					Mutator: ovsdb.MutateOperationDelete,
					Value:   []string{"foo"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationDelete, Value: testOvsSet(t, []string{"foo"})}},
					Where:     []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
				},
			},
			err: false,
		},
		{
			name: "select single by predicate name insert element in map",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Name == "lsp2"
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIDs,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   map[string]string{"bar": "baz"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
			},
			err: false,
		},
		{
			name: "select many by predicate name insert element in map",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Type == "someType"
				})
			},
			mutations: []model.Mutation{
				{
					Field:   &testObj.ExternalIDs,
					Mutator: ovsdb.MutateOperationInsert,
					Value:   map[string]string{"bar": "baz"},
				},
			},
			result: []ovsdb.Operation{
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:        ovsdb.OperationMutate,
					Table:     "Logical_Switch_Port",
					Mutations: []ovsdb.Mutation{{Column: "external_ids", Mutator: ovsdb.MutateOperationInsert, Value: testOvsMap(t, map[string]string{"bar": "baz"})}},
					Where:     []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "No mutations should error",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(lsp *testLogicalSwitchPort) bool {
					return lsp.Type == "someType"
				})
			},
			mutations: []model.Mutation{},
			err:       true,
		},
		{
			name: "multiple different selected models must fail",
			condition: func(a API) ConditionalAPI {
				return a.Where(
					&testLogicalSwitchPort{UUID: aUUID0},
					&testLogicalSwitchPort{UUID: aUUID1},
					&testLogicalSwitch{UUID: aUUID2},
				)
			},
			err: true,
		},
		{
			name: "fails if conditional is an error",
			condition: func(_ API) ConditionalAPI {
				return newConditionalAPI(nil, newErrorConditional(fmt.Errorf("error")), &discardLogger, false)
			},
			err: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiMutate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache, &discardLogger, false)
			cond := tt.condition(api)
			ops, err := cond.Mutate(&testObj, tt.mutations...)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func TestAPIUpdate(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
			Enabled:     &trueVal,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	testObj := testLogicalSwitchPort{}
	testRow := ovsdb.Row(map[string]any{"type": "somethingElse", "tag": testOvsSet(t, []int{6})})
	tagRow := ovsdb.Row(map[string]any{"tag": testOvsSet(t, []int{6})})
	var nilInt *int
	testNilRow := ovsdb.Row(map[string]any{"type": "somethingElse", "tag": testOvsSet(t, nilInt)})
	typeRow := ovsdb.Row(map[string]any{"type": "somethingElse"})
	fields := []any{&testObj.Tag, &testObj.Type}

	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		prepare   func(t *testLogicalSwitchPort)
		result    []ovsdb.Operation
		fields    []any
		err       bool
	}{
		{
			name: "select by UUID change multiple field",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by UUID change multiple field with nil pointer/empty set",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = nilInt
			},
			fields: fields,
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testNilRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by UUID with no fields does not change multiple field with nil pointer/empty set",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = nilInt
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   typeRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by index change multiple field with no cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "foo",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
				},
			},
			err: false,
		},
		{
			name: "select by index change multiple field with cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp1",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "select by field change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Type:    "sometype",
					Enabled: &trueVal,
				}
				return a.WhereAny(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionEqual,
					Value:    "sometype",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
			},
			err: false,
		},
		{
			name: "multiple select any by field change multiple field with cache hits",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{}
				return a.WhereAny(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "someOtherType",
					},
					model.Condition{
						Field:    &t.Enabled,
						Function: ovsdb.ConditionEqual,
						Value:    &trueVal,
					})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "multiple select all by field change multiple field",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{}
				return a.WhereAll(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					},
					model.Condition{
						Field:    &t.Enabled,
						Function: ovsdb.ConditionIncludes,
						Value:    &trueVal,
					})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{
						{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"},
						{Column: "enabled", Function: ovsdb.ConditionIncludes, Value: testOvsSet(t, &trueVal)},
					},
				},
			},
			err: false,
		},
		{
			name: "select by field inequality change multiple field with cache",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Type:    "someType",
					Enabled: &trueVal,
				}
				return a.WhereAny(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionNotEqual,
					Value:    "someType",
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID2}}},
				},
			},
			err: false,
		},
		{
			name: "select by field inequality change multiple field with no cache",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Type:    "sometype",
					Enabled: &trueVal,
				}
				return a.WhereAny(&t, model.Condition{
					Field:    &t.Tag,
					Function: ovsdb.ConditionNotEqual,
					Value:    &one,
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   tagRow,
					Where: []ovsdb.Condition{{Column: "tag", Function: ovsdb.ConditionNotEqual, Value: testOvsSet(t, &one)}},
				},
			},
			err: false,
		},
		{
			name: "select multiple by predicate change multiple field",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(t *testLogicalSwitchPort) bool {
					return t.Enabled != nil && *t.Enabled == true
				})
			},
			prepare: func(t *testLogicalSwitchPort) {
				t.Type = "somethingElse"
				t.Tag = &six
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:    ovsdb.OperationUpdate,
					Table: "Logical_Switch_Port",
					Row:   testRow,
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "multiple different selected models must fail",
			condition: func(a API) ConditionalAPI {
				return a.Where(
					&testLogicalSwitchPort{UUID: aUUID0},
					&testLogicalSwitchPort{UUID: aUUID1},
					&testLogicalSwitch{UUID: aUUID2},
				)
			},
			err: true,
		},
		{
			name: "fails if conditional is an error",
			condition: func(_ API) ConditionalAPI {
				return newConditionalAPI(tcache, newErrorConditional(fmt.Errorf("error")), &discardLogger, false)
			},
			prepare: func(_ *testLogicalSwitchPort) {
			},
			err: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiUpdate: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache, &discardLogger, false)
			cond := tt.condition(api)
			// clean test Object
			testObj = testLogicalSwitchPort{}
			if tt.prepare != nil {
				tt.prepare(&testObj)
			}
			ops, err := cond.Update(&testObj, tt.fields...)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func TestAPIDelete(t *testing.T) {
	lspCache := map[string]model.Model{
		aUUID0: &testLogicalSwitchPort{
			UUID:        aUUID0,
			Name:        "lsp0",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "bar"},
			Enabled:     &trueVal,
			Tag:         &one,
		},
		aUUID1: &testLogicalSwitchPort{
			UUID:        aUUID1,
			Name:        "lsp1",
			Type:        "someType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
			Enabled:     &trueVal,
		},
		aUUID2: &testLogicalSwitchPort{
			UUID:        aUUID2,
			Name:        "lsp2",
			Type:        "someOtherType",
			ExternalIDs: map[string]string{"foo": "baz"},
			Tag:         &one,
		},
	}
	testData := cache.Data{
		"Logical_Switch_Port": lspCache,
	}
	tcache := apiTestCache(t, testData)

	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "select by UUID",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitch{
					UUID: aUUID0,
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
			},
			err: false,
		},
		{
			name: "select by index with cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp1",
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "select by index with no cache",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "foo",
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
				},
			},
			err: false,
		},
		{
			name: "select by field equality",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.WhereAny(&t, model.Condition{
					Field:    &t.Type,
					Function: ovsdb.ConditionEqual,
					Value:    "sometype",
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
			},
			err: false,
		},
		{
			name: "select any by field ",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.WhereAny(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					}, model.Condition{
						Field:    &t.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "foo",
					})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"}},
				},
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"}},
				},
			},
			err: false,
		},
		{
			name: "select all by field ",
			condition: func(a API) ConditionalAPI {
				t := testLogicalSwitchPort{
					Enabled: &trueVal,
				}
				return a.WhereAll(&t,
					model.Condition{
						Field:    &t.Type,
						Function: ovsdb.ConditionEqual,
						Value:    "sometype",
					}, model.Condition{
						Field:    &t.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "foo",
					})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{
						{Column: "type", Function: ovsdb.ConditionEqual, Value: "sometype"},
						{Column: "name", Function: ovsdb.ConditionEqual, Value: "foo"},
					},
				},
			},
			err: false,
		},
		{
			name: "select multiple by predicate",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(t *testLogicalSwitchPort) bool {
					return t.Enabled != nil && *t.Enabled == true
				})
			},
			result: []ovsdb.Operation{
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID0}}},
				},
				{
					Op:    ovsdb.OperationDelete,
					Table: "Logical_Switch_Port",
					Where: []ovsdb.Condition{{Column: "_uuid", Function: ovsdb.ConditionEqual, Value: ovsdb.UUID{GoUUID: aUUID1}}},
				},
			},
			err: false,
		},
		{
			name: "multiple different selected models must fail",
			condition: func(a API) ConditionalAPI {
				return a.Where(
					&testLogicalSwitchPort{UUID: aUUID0},
					&testLogicalSwitchPort{UUID: aUUID1},
					&testLogicalSwitch{UUID: aUUID2},
				)
			},
			err: true,
		},
		{
			name: "fails if conditional is an error",
			condition: func(_ API) ConditionalAPI {
				return newConditionalAPI(nil, newErrorConditional(fmt.Errorf("error")), &discardLogger, false)
			},
			err: true,
		},
	}
	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiDelete: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache, &discardLogger, false)
			cond := tt.condition(api)
			ops, err := cond.Delete()
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func BenchmarkAPIList(b *testing.B) {
	const numRows = 10000

	lscacheList := make([]*testLogicalSwitchPort, 0, numRows)

	for i := 0; i < numRows; i++ {
		lscacheList = append(lscacheList,
			&testLogicalSwitchPort{
				UUID:        uuid.New().String(),
				Name:        fmt.Sprintf("ls%d", i),
				ExternalIDs: map[string]string{"foo": "bar"},
			})
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lscache,
	}
	tcache := apiTestCache(b, testData)

	r := rand.New(rand.NewSource(int64(b.N)))
	var index int

	test := []struct {
		name      string
		predicate any
	}{
		{
			name: "predicate returns none",
			predicate: func(_ *testLogicalSwitchPort) bool {
				return false
			},
		},
		{
			name: "predicate returns all",
			predicate: func(_ *testLogicalSwitchPort) bool {
				return true
			},
		},
		{
			name: "predicate on an arbitrary condition",
			predicate: func(t *testLogicalSwitchPort) bool {
				return strings.HasPrefix(t.Name, "ls1")
			},
		},
		{
			name: "predicate matches name",
			predicate: func(t *testLogicalSwitchPort) bool {
				return t.Name == lscacheList[index].Name
			},
		},
		{
			name: "by index, no predicate",
		},
	}

	for _, tt := range test {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				index = r.Intn(numRows)
				var result []*testLogicalSwitchPort
				api := newAPI(tcache, &discardLogger, false)
				var cond ConditionalAPI
				if tt.predicate != nil {
					cond = api.WhereCache(tt.predicate)
				} else {
					cond = api.Where(lscacheList[index])
				}
				err := cond.List(context.Background(), &result)
				assert.NoError(b, err)
			}
		})
	}
}

func BenchmarkAPIListMultiple(b *testing.B) {
	const numRows = 500

	lscacheList := make([]*testLogicalSwitchPort, 0, numRows)

	for i := 0; i < numRows; i++ {
		lscacheList = append(lscacheList,
			&testLogicalSwitchPort{
				UUID:        uuid.New().String(),
				Name:        fmt.Sprintf("ls%d", i),
				ExternalIDs: map[string]string{"foo": "bar"},
			})
	}
	lscache := map[string]model.Model{}
	for i := range lscacheList {
		lscache[lscacheList[i].UUID] = lscacheList[i]
	}
	testData := cache.Data{
		"Logical_Switch_Port": lscache,
	}
	tcache := apiTestCache(b, testData)

	models := make([]model.Model, len(lscacheList))
	for i := 0; i < len(lscacheList); i++ {
		models[i] = &testLogicalSwitchPort{UUID: lscacheList[i].UUID}
	}

	test := []struct {
		name     string
		whereAny bool
	}{
		{
			name: "multiple results one at a time with Get",
		},
		{
			name:     "multiple results in a batch with WhereAny",
			whereAny: true,
		},
	}

	for _, tt := range test {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var results []*testLogicalSwitchPort
				api := newAPI(tcache, &discardLogger, false)
				if tt.whereAny {
					// Looking up models with WhereAny() should be fast
					cond := api.Where(models...)
					err := cond.List(context.Background(), &results)
					require.NoError(b, err)
				} else {
					// Looking up models one-at-a-time with Get() should be slow
					for j := 0; j < len(lscacheList); j++ {
						m := &testLogicalSwitchPort{UUID: lscacheList[j].UUID}
						err := api.Get(context.Background(), m)
						require.NoError(b, err)
						results = append(results, m)
					}
				}
				assert.Len(b, results, len(models))
			}
		})
	}
}

func BenchmarkAPICreate(b *testing.B) {
	tcache := apiTestCache(b, nil) // Create doesn't need a cache

	// newSimpleValidBridge creates a bridge with only the required fields
	// and empty collections.
	newSimpleValidBridge := func() *testBridge {
		return &testBridge{
			UUID:                uuid.NewString(),
			Name:                "valid-br-" + uuid.NewString()[:8], // Ensure unique name
			DatapathType:        "system",
			DatapathVersion:     "1.0",
			McastSnoopingEnable: false,
			RSTPEnable:          false,
			STPEnable:           false,
			// Initialize collections to avoid nil pointer issues
			Controller:  []string{},
			ExternalIDs: map[string]string{},
			FloodVLANs:  []int{},
			FlowTables:  map[int]string{},
			Mirrors:     []string{},
			OtherConfig: map[string]string{},
			Ports:       []string{},
			Protocols:   []string{},
			RSTPStatus:  map[string]string{},
			Status:      map[string]string{},
		}
	}

	// newComplexValidBridge creates a bridge with many fields populated
	// with valid data to test performance against a more complex model.
	newComplexValidBridge := func() *testBridge {
		fm := BridgeFailModeSecure
		return &testBridge{
			UUID:                uuid.NewString(),
			Name:                "complex-br-" + uuid.NewString()[:8],
			DatapathType:        "system",
			DatapathVersion:     "1.0",
			FailMode:            &fm,
			Controller:          []string{uuid.NewString(), uuid.NewString()},
			ExternalIDs:         map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
			FloodVLANs:          []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			FlowTables:          map[int]string{1: uuid.NewString(), 10: uuid.NewString(), 100: uuid.NewString()},
			Mirrors:             []string{uuid.NewString()},
			OtherConfig:         map[string]string{"cfg1": "val1", "cfg2": "val2"},
			Ports:               []string{uuid.NewString(), uuid.NewString(), uuid.NewString()},
			Protocols:           []string{BridgeProtocolsOpenflow13, BridgeProtocolsOpenflow14, BridgeProtocolsOpenflow15},
			McastSnoopingEnable: true,
			RSTPEnable:          true,
			STPEnable:           false,
			RSTPStatus:          map[string]string{"rstp_key": "rstp_val"},
			Status:              map[string]string{"status_key": "status_val"},
		}
	}

	testCases := []struct {
		name          string
		bridgeFactory func() *testBridge
		validateModel bool
	}{
		{
			name:          "simple model",
			bridgeFactory: newSimpleValidBridge,
		},
		{
			name:          "complex model",
			bridgeFactory: newComplexValidBridge,
		},
		{
			name:          "simple model with validation",
			bridgeFactory: newSimpleValidBridge,
			validateModel: true,
		},
		{
			name:          "complex model with validation",
			bridgeFactory: newComplexValidBridge,
			validateModel: true,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			api := newAPI(tcache, &discardLogger, tc.validateModel)
			for i := 0; i < b.N; i++ {
				bridgeToCreate := tc.bridgeFactory()
				ops, err := api.Create(bridgeToCreate)
				require.NoError(b, err)
				require.NotNil(b, ops)
			}
		})
	}
}

func TestAPIWait(t *testing.T) {
	tcache := apiTestCache(t, cache.Data{})
	timeout0 := 0

	test := []struct {
		name      string
		condition func(API) ConditionalAPI
		prepare   func() (model.Model, []any)
		until     ovsdb.WaitCondition
		timeout   *int
		result    []ovsdb.Operation
		err       bool
	}{
		{
			name: "timeout 0, no columns",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp0",
				})
			},
			until:   "==",
			timeout: &timeout0,
			prepare: func() (model.Model, []any) {
				testLSP := testLogicalSwitchPort{
					Name: "lsp0",
				}
				return &testLSP, nil
			},
			result: []ovsdb.Operation{
				{
					Op:      ovsdb.OperationWait,
					Table:   "Logical_Switch_Port",
					Timeout: &timeout0,
					Where:   []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lsp0"}},
					Until:   string(ovsdb.WaitConditionEqual),
					Columns: nil,
					Rows:    []ovsdb.Row{{"name": "lsp0"}},
				},
			},
			err: false,
		},
		{
			name: "no timeout",
			condition: func(a API) ConditionalAPI {
				return a.Where(&testLogicalSwitchPort{
					Name: "lsp0",
				})
			},
			until: "!=",
			prepare: func() (model.Model, []any) {
				testLSP := testLogicalSwitchPort{
					Name: "lsp0",
					Type: "someType",
				}
				return &testLSP, []any{&testLSP.Name, &testLSP.Type}
			},
			result: []ovsdb.Operation{
				{
					Op:      ovsdb.OperationWait,
					Timeout: nil,
					Table:   "Logical_Switch_Port",
					Where:   []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lsp0"}},
					Until:   string(ovsdb.WaitConditionNotEqual),
					Columns: []string{"name", "type"},
					Rows:    []ovsdb.Row{{"name": "lsp0", "type": "someType"}},
				},
			},
			err: false,
		},
		{
			name: "multiple conditions",
			condition: func(a API) ConditionalAPI {
				isUp := true
				lsp := testLogicalSwitchPort{}
				conditions := []model.Condition{
					{
						Field:    &lsp.Up,
						Function: ovsdb.ConditionNotEqual,
						Value:    &isUp,
					},
					{
						Field:    &lsp.Name,
						Function: ovsdb.ConditionEqual,
						Value:    "lspNameCondition",
					},
				}
				return a.WhereAny(&lsp, conditions...)
			},
			until: "!=",
			prepare: func() (model.Model, []any) {
				testLSP := testLogicalSwitchPort{
					Name: "lsp0",
					Type: "someType",
				}
				return &testLSP, []any{&testLSP.Name, &testLSP.Type}
			},
			result: []ovsdb.Operation{
				{
					Op:      ovsdb.OperationWait,
					Timeout: nil,
					Table:   "Logical_Switch_Port",
					Where: []ovsdb.Condition{
						{
							Column:   "up",
							Function: ovsdb.ConditionNotEqual,
							Value:    ovsdb.OvsSet{GoSet: []any{true}},
						},
					},
					Until:   string(ovsdb.WaitConditionNotEqual),
					Columns: []string{"name", "type"},
					Rows:    []ovsdb.Row{{"name": "lsp0", "type": "someType"}},
				},
				{
					Op:      ovsdb.OperationWait,
					Timeout: nil,
					Table:   "Logical_Switch_Port",
					Where:   []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: "lspNameCondition"}},
					Until:   string(ovsdb.WaitConditionNotEqual),
					Columns: []string{"name", "type"},
					Rows:    []ovsdb.Row{{"name": "lsp0", "type": "someType"}},
				},
			},
			err: false,
		},
		{
			name: "non-indexed condition error",
			condition: func(a API) ConditionalAPI {
				isUp := false
				return a.Where(&testLogicalSwitchPort{Up: &isUp})
			},
			until: "==",
			prepare: func() (model.Model, []any) {
				testLSP := testLogicalSwitchPort{Name: "lsp0"}
				return &testLSP, nil
			},
			err: true,
		},
		{
			name: "no operation",
			condition: func(a API) ConditionalAPI {
				return a.WhereCache(func(_ *testLogicalSwitchPort) bool { return false })
			},
			until: "==",
			prepare: func() (model.Model, []any) {
				testLSP := testLogicalSwitchPort{Name: "lsp0"}
				return &testLSP, nil
			},
			result: []ovsdb.Operation{},
			err:    false,
		},
		{
			name: "fails if conditional is an error",
			condition: func(_ API) ConditionalAPI {
				return newConditionalAPI(nil, newErrorConditional(fmt.Errorf("error")), &discardLogger, false)
			},
			prepare: func() (model.Model, []any) {
				return nil, nil
			},
			err: true,
		},
	}

	for _, tt := range test {
		t.Run(fmt.Sprintf("ApiWait: %s", tt.name), func(t *testing.T) {
			api := newAPI(tcache, &discardLogger, false)
			cond := tt.condition(api)
			model, fields := tt.prepare()
			ops, err := cond.Wait(tt.until, tt.timeout, model, fields...)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatchf(t, tt.result, ops, "ovsdb.Operations should match")
			}
		})
	}
}

func TestAPIValidationCreate(t *testing.T) {
	tcache := apiTestCache(t, nil)
	api := newAPI(tcache, &discardLogger, true)

	// Helper to create a minimally valid Bridge object
	newValidBaseBridge := func() *testBridge {
		// Based on schema, Name, DatapathType, DatapathVersion seem required.
		// Others have defaults or are optional.
		return &testBridge{
			UUID:                uuid.NewString(),
			Name:                "valid-br-" + uuid.NewString()[:8], // Ensure unique name
			DatapathType:        "system",
			DatapathVersion:     "1.0",
			McastSnoopingEnable: false,
			RSTPEnable:          false,
			STPEnable:           false,
			// Initialize collections to avoid nil pointer issues if accessed
			Controller:  []string{},
			ExternalIDs: map[string]string{},
			FloodVLANs:  []int{},
			FlowTables:  map[int]string{},
			Mirrors:     []string{},
			OtherConfig: map[string]string{},
			Ports:       []string{},
			Protocols:   []string{},
			RSTPStatus:  map[string]string{},
			Status:      map[string]string{},
		}
	}

	type createTestCase struct {
		name          string
		bridge        *testBridge // Model to create
		expectedError string      // Substring expected in the error message, empty if no error expected
	}

	testCases := []createTestCase{
		// --- Valid Cases ---
		{
			name:   "minimal valid Bridge",
			bridge: newValidBaseBridge(),
			// No error expected
		},
		{
			name: "valid Bridge with standard UUID",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.UUID = aUUID0 // Use a standard UUID format
				br.Name = "valid-br-std-uuid"
				return br
			}(),
			// No error expected
		},
		{
			name: "valid Bridge with named UUID",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.UUID = "my-named-uuid-for-bridge-create" // Use a named UUID
				br.Name = "valid-br-named-uuid"
				return br
			}(),
			// No error expected
		},
		{
			name: "valid Bridge with specific FailMode",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "valid-br-failmode"
				fm := BridgeFailModeSecure
				br.FailMode = &fm
				return br
			}(),
			// No error expected
		},
		{
			name: "valid Bridge with Protocols",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "valid-br-protocols"
				br.Protocols = []string{BridgeProtocolsOpenflow13, BridgeProtocolsOpenflow15}
				return br
			}(),
			// No error expected
		},
		{
			name: "valid Bridge with FloodVLANs within range",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "valid-br-floodvlans"
				br.FloodVLANs = []int{10, 20, 4095} // Valid range 0-4095
				return br
			}(),
			// No error expected
		},
		{
			name: "valid Bridge with FlowTables within key range",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "valid-br-flowtables"
				br.FlowTables = map[int]string{0: uuid.NewString(), 254: uuid.NewString()} // Valid key range 0-254
				return br
			}(),
			// No error expected
		},

		// --- Invalid Cases (Based on current 'validate' tags) ---
		{
			name: "invalid FailMode",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-failmode"
				invalidFm := "unknown"
				br.FailMode = &invalidFm // Violates oneof='standalone' 'secure'
				return br
			}(),
			expectedError: "field 'testBridge.FailMode' (value: 'unknown') failed on rule 'oneof' (param: 'standalone' 'secure')",
		},
		{
			name: "invalid Protocol",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-protocol"
				// Protocols uses dive, so validation checks each element
				br.Protocols = []string{BridgeProtocolsOpenflow13, "InvalidProtocol"} // Violates dive, oneof
				return br
			}(),
			expectedError: "field 'testBridge.Protocols[1]' (value: 'InvalidProtocol') failed on rule 'oneof'",
		},
		{
			name: "too many FloodVLANs",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-floodvlans-len"
				br.FloodVLANs = make([]int, 4097) // Violates max=4096 slice length
				return br
			}(),
			expectedError: "field 'testBridge.FloodVLANs' (value: '[",
		},
		{
			name: "invalid FloodVLAN value (too low)",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-floodvlan-low"
				// FloodVLANs uses dive, validation checks element value
				br.FloodVLANs = []int{10, -1, 20} // Violates dive, min=0
				return br
			}(),
			expectedError: "field 'testBridge.FloodVLANs[1]' (value: '-1') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid FloodVLAN value (too high)",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-floodvlan-high"
				br.FloodVLANs = []int{10, 4096, 20} // Violates dive, max=4095
				return br
			}(),
			expectedError: "field 'testBridge.FloodVLANs[1]' (value: '4096') failed on rule 'max' (param: 4095)",
		},
		{
			name: "invalid FlowTable key (too low)",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-flowtable-keylow"
				// FlowTables uses dive, keys, validation checks map keys
				br.FlowTables = map[int]string{0: uuid.NewString(), -1: uuid.NewString()} // Violates dive, keys, min=0
				return br
			}(),
			expectedError: "field 'testBridge.FlowTables[-1]' (value: '-1') failed on rule 'min' (param: 0)",
		},
		{
			name: "invalid FlowTable key (too high)",
			bridge: func() *testBridge {
				br := newValidBaseBridge()
				br.Name = "invalid-br-flowtable-keyhigh"
				br.FlowTables = map[int]string{0: uuid.NewString(), 255: uuid.NewString()} // Violates dive, keys, max=254
				return br
			}(),
			expectedError: "field 'testBridge.FlowTables[255]' (value: '255') failed on rule 'max' (param: 254)",
		},
	}

	for _, tc := range testCases {
		t.Run("Create_"+tc.name, func(t *testing.T) {
			ops, err := api.Create(tc.bridge)

			if tc.expectedError != "" {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				// Check if the error message contains the expected substring
				require.ErrorContains(t, err, tc.expectedError, "Error message mismatch for: %s. New error: %s", tc.name, err.Error())

				// For validation errors, also check that they can be extracted
				var valErrs validator.ValidationErrors
				if errors.As(err, &valErrs) {
					// This is a validation error, which is expected for model validation failures
					errMsg := err.Error()
					assert.True(t, strings.Contains(errMsg, "model validation failed") || strings.Contains(errMsg, "mutation validation failed"), "Should be a validation error for: %s", tc.name)
				}
				// Note: Immutable field errors are not validation errors, they are different types of errors

				assert.Nil(t, ops, "Operations should be nil on error for: %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.NotNil(t, ops, "Operations should not be nil for valid case: %s", tc.name)
				assert.Len(t, ops, 1, "Expected one operation for valid case: %s", tc.name)
				// Additional checks specific to Bridge creation if needed
				if ovsdb.IsNamedUUID(tc.bridge.UUID) {
					assert.Equal(t, tc.bridge.UUID, ops[0].UUIDName, "UUIDName mismatch for named UUID case: %s", tc.name)
					_, uuidInRow := ops[0].Row["_uuid"]
					assert.False(t, uuidInRow, "_uuid should not be in Row for named UUID case: %s", tc.name)
				} else if tc.bridge.UUID != "" {
					assert.Empty(t, ops[0].UUIDName, "UUIDName should be empty for standard UUID case: %s", tc.name)
					assert.Equal(t, tc.bridge.UUID, ops[0].UUID, "UUID should be set for standard UUID case: %s", tc.name)
					_, uuidInRow := ops[0].Row["_uuid"]
					assert.False(t, uuidInRow, "_uuid should not be in Row for standard UUID case: %s", tc.name)
				} else {
					// Create doesn't auto-generate UUID, it relies on named UUID or standard UUID in row
					// If UUID is empty string, it should result in server-generated UUID, so UUIDName is empty and _uuid not in row
					assert.Empty(t, ops[0].UUIDName, "UUIDName should be empty for empty UUID case: %s", tc.name)
					assert.Empty(t, ops[0].UUID, "UUID should be empty for empty UUID case: %s", tc.name)
					_, uuidInRow := ops[0].Row["_uuid"]
					assert.False(t, uuidInRow, "_uuid should not be in Row for empty UUID case: %s", tc.name)
				}
			}
		})
	}
}

func TestAPIValidationMutate(t *testing.T) {
	initialBridgeUUID := uuid.NewString()
	initialBridge := &testBridge{
		UUID:            initialBridgeUUID,
		Name:            "initial-br-for-mutate-test",
		DatapathType:    "system",
		DatapathVersion: "1.0",
		Controller:      []string{uuid.NewString()},          // Start with one
		ExternalIDs:     map[string]string{"k1": "v1"},       // Start with one
		FloodVLANs:      []int{10, 20},                       // Start with some
		FlowTables:      map[int]string{5: uuid.NewString()}, // Start with one
		Protocols:       []string{BridgeProtocolsOpenflow13}, // Start with one
		// Other fields omitted for brevity, assuming they are not mutated in tests below
	}
	bridgeCacheData := map[string]model.Model{initialBridge.UUID: initialBridge}
	testCacheData := cache.Data{"Bridge": bridgeCacheData}
	tcache := apiTestCache(t, testCacheData)
	api := newAPI(tcache, &discardLogger, true)

	// Pointers to fields for use in Mutations
	// var bridgeForFieldPointers testBridge -- Removed, use initialBridge directly

	type mutateTestCase struct {
		name              string
		mutations         []model.Mutation // Mutations to apply
		expectedError     string           // Substring expected in the error message
		isValidationError bool             // Flag to indicate if the error is expected to be a ValidationError
	}

	testCases := []mutateTestCase{
		// --- Valid Mutations ---
		{
			name: "insert into Protocols (valid value)",
			mutations: []model.Mutation{
				{Field: &initialBridge.Protocols, Mutator: ovsdb.MutateOperationInsert, Value: []string{BridgeProtocolsOpenflow14}},
			},
			// No error expected from validation
		},
		{
			name: "delete from Protocols (valid value)",
			mutations: []model.Mutation{
				{Field: &initialBridge.Protocols, Mutator: ovsdb.MutateOperationDelete, Value: []string{BridgeProtocolsOpenflow13}},
			},
			// No error expected from validation
		},
		{
			name: "insert into FloodVLANs (valid value)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FloodVLANs, Mutator: ovsdb.MutateOperationInsert, Value: []int{30, 4095}},
			},
			// No error expected from validation
		},
		{
			name: "delete from FloodVLANs (valid value)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FloodVLANs, Mutator: ovsdb.MutateOperationDelete, Value: []int{10}},
			},
			// No error expected from validation
		},
		{
			name: "insert into FlowTables (valid key)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FlowTables, Mutator: ovsdb.MutateOperationInsert, Value: map[int]string{254: uuid.NewString()}},
			},
			// No error expected from validation
		},
		{
			name: "delete from FlowTables (valid key)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FlowTables, Mutator: ovsdb.MutateOperationDelete, Value: map[int]string{5: ""}}, // Key matters for delete
			},
			// No error expected from validation
		},
		{
			name: "insert into ExternalIDs",
			mutations: []model.Mutation{
				{Field: &initialBridge.ExternalIDs, Mutator: ovsdb.MutateOperationInsert, Value: map[string]string{"new_key": "new_val"}},
			},
			// No error expected from validation
		},
		{
			name: "delete from ExternalIDs",
			mutations: []model.Mutation{
				{Field: &initialBridge.ExternalIDs, Mutator: ovsdb.MutateOperationDelete, Value: map[string]string{"k1": ""}},
			},
			// No error expected from validation
		},

		// --- Invalid Mutations (Value violates constraint) ---
		// Note: Mutate validation checks the *value being mutated*, not the resulting state size/limit.
		// Immutability check happens *before* value validation.
		{
			name: "mutate immutable field (Name)",
			mutations: []model.Mutation{
				// Attempt to insert into Name (string) - fundamentally wrong mutation, but should hit immutable check first.
				{Field: &initialBridge.Name, Mutator: ovsdb.MutateOperationInsert, Value: "should-fail-immutable"},
			},
			expectedError:     "column is not mutable",
			isValidationError: false, // not wrapped in ValidationError
		},
		{
			name: "insert invalid Protocol value",
			mutations: []model.Mutation{
				{Field: &initialBridge.Protocols, Mutator: ovsdb.MutateOperationInsert, Value: []string{"InvalidProto"}}, // Violates oneof
			},
			expectedError:     "field '[0]' (value: 'InvalidProto') failed on rule 'oneof'",
			isValidationError: true,
		},
		{
			name: "insert invalid FloodVLAN value (too high)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FloodVLANs, Mutator: ovsdb.MutateOperationInsert, Value: []int{4096}}, // Violates max=4095
			},
			expectedError:     "field '[0]' (value: '4096') failed on rule 'max' (param: 4095)",
			isValidationError: true,
		},
		{
			name: "insert invalid FloodVLAN value (too low)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FloodVLANs, Mutator: ovsdb.MutateOperationInsert, Value: []int{-1}}, // Violates min=0
			},
			expectedError:     "field '[0]' (value: '-1') failed on rule 'min' (param: 0)",
			isValidationError: true,
		},
		{
			name: "insert invalid FlowTable key (too high)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FlowTables, Mutator: ovsdb.MutateOperationInsert, Value: map[int]string{255: uuid.NewString()}}, // Violates keys, max=254
			},
			expectedError:     "field '[255]' (value: '255') failed on rule 'max' (param: 254)",
			isValidationError: true,
		},
		{
			name: "insert invalid FlowTable key (too low)",
			mutations: []model.Mutation{
				{Field: &initialBridge.FlowTables, Mutator: ovsdb.MutateOperationInsert, Value: map[int]string{-1: uuid.NewString()}}, // Violates keys, min=0
			},
			expectedError:     "field '[-1]' (value: '-1') failed on rule 'min' (param: 0)",
			isValidationError: true,
		},
	}

	for _, tc := range testCases {
		t.Run("Mutate_"+tc.name, func(t *testing.T) {
			// Condition to select the object to mutate
			condAPI := api.Where(&testBridge{UUID: initialBridgeUUID})
			ops, err := condAPI.Mutate(initialBridge, tc.mutations...)

			if tc.expectedError != "" {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				// Check if the error message contains the expected substring
				require.ErrorContains(t, err, tc.expectedError, "Error message mismatch for: %s. New error: %s", tc.name, err.Error())

				// For validation errors, also check that they can be extracted
				var valErrs validator.ValidationErrors
				if errors.As(err, &valErrs) {
					// This is a validation error, which is expected for model validation failures
					errMsg := err.Error()
					assert.True(t, strings.Contains(errMsg, "model validation failed") || strings.Contains(errMsg, "mutation validation failed"), "Should be a validation error for: %s", tc.name)
				}
				// Note: Immutable field errors are not validation errors, they are different types of errors

				assert.Nil(t, ops, "Operations should be nil on error for: %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.NotNil(t, ops, "Operations should not be nil for valid case: %s", tc.name)
				require.GreaterOrEqual(t, len(ops), 1, "Need at least one operation for: %s", tc.name)
				// Check mutations format if needed
				assert.Equal(t, string(ovsdb.OperationMutate), ops[0].Op, "Operation should be mutate for: %s", tc.name)
				assert.NotNil(t, ops[0].Mutations, "Mutations should not be nil for: %s", tc.name)
				assert.Len(t, ops[0].Mutations, len(tc.mutations), "Number of mutations mismatch for: %s", tc.name)
			}
		})
	}
}

func TestAPIValidationUpdate(t *testing.T) {

	initialBridgeUUID := uuid.NewString()
	initialBridge := &testBridge{
		UUID:                initialBridgeUUID,
		Name:                "initial-br-for-update-test",
		DatapathType:        "system",
		DatapathVersion:     "1.0",
		McastSnoopingEnable: false,
		RSTPEnable:          false,
		STPEnable:           false,
		Controller:          []string{uuid.NewString()}, // Start with one controller
		ExternalIDs:         map[string]string{"initial_key": "initial_val"},
		FloodVLANs:          []int{100, 200},
		FlowTables:          map[int]string{10: uuid.NewString()},
		Mirrors:             []string{},
		OtherConfig:         map[string]string{"initial_other": "initial_other_val"},
		Ports:               []string{uuid.NewString()},
		Protocols:           []string{BridgeProtocolsOpenflow13},
		RSTPStatus:          map[string]string{},
		Status:              map[string]string{},
		// FailMode is initially nil
	}
	bridgeCacheData := map[string]model.Model{initialBridge.UUID: initialBridge}
	testCacheData := cache.Data{"Bridge": bridgeCacheData}
	tcache := apiTestCache(t, testCacheData)
	api := newAPI(tcache, &discardLogger, true)

	// Helper to get a fresh copy of the initialBridge state from cache
	getFreshBridgeStateFromCache := func(t *testing.T) *testBridge {
		br := &testBridge{UUID: initialBridgeUUID}
		err := api.Get(context.Background(), br)
		require.NoError(t, err, "Failed to get initial Bridge state from cache")
		return br
	}

	type updateTestCase struct {
		name          string
		updateModelFn func(currentBridge *testBridge) *testBridge // Fn to create the model for Update()
		expectedError string                                      // Substring expected in the error message
		// For Update, all validation errors are expected to be ValidationError
		useFieldNames []string // Optional field names to use as specific update targets
	}

	testCases := []updateTestCase{
		// --- Valid Cases ---
		{
			name: "update DatapathType and add Protocol",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current // Copy base state
				modelToUpdate.DatapathType = "netdev"
				modelToUpdate.Protocols = append(modelToUpdate.Protocols, BridgeProtocolsOpenflow14)
				return &modelToUpdate
			},
			// No error expected
		},
		{
			name: "update set FailMode",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				validFm := BridgeFailModeStandalone
				modelToUpdate.FailMode = &validFm
				return &modelToUpdate
			},
			// No error expected
		},
		{
			name: "update clear FailMode",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				// Set it first to ensure clearing works
				fm := BridgeFailModeSecure
				modelToUpdate.FailMode = &fm
				// Now create the actual update model where it's nil
				updateModel := *current
				updateModel.FailMode = nil
				return &updateModel
			},
			// No error expected
		},
		{
			name: "update FloodVLANs (valid range and size)",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.FloodVLANs = []int{0, 4095} // Valid values and count within max=4096
				return &modelToUpdate
			},
			// No error expected
		},
		{
			name: "update FlowTables (valid key)",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.FlowTables = map[int]string{254: uuid.NewString()} // Valid key
				return &modelToUpdate
			},
			// No error expected
		},

		// --- Invalid Cases (Based on current 'validate' tags) ---
		{
			name: "update invalid FailMode",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				invalidFm := "invalid-mode"
				modelToUpdate.FailMode = &invalidFm // Violates oneof
				return &modelToUpdate
			},
			expectedError: "field 'testBridge.FailMode' (value: 'invalid-mode') failed on rule 'oneof'",
		},
		{
			name: "update invalid Protocol",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.Protocols = []string{"BadProto"} // Violates dive, oneof
				return &modelToUpdate
			},
			expectedError: "field 'testBridge.Protocols[0]' (value: 'BadProto') failed on rule 'oneof'",
		},
		{
			name: "update too many FloodVLANs",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.FloodVLANs = make([]int, 4097) // Violates max=4096 length
				return &modelToUpdate
			},
			expectedError: "field 'testBridge.FloodVLANs' (value: '[",
		},
		{
			name: "update invalid FloodVLAN value (too high)",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.FloodVLANs = []int{4096} // Violates dive, max=4095
				return &modelToUpdate
			},
			expectedError: "field 'testBridge.FloodVLANs[0]' (value: '4096') failed on rule 'max' (param: 4095)",
		},
		{
			name: "update invalid FlowTable key (too high)",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.FlowTables = map[int]string{255: uuid.NewString()} // Violates dive, keys, max=254
				return &modelToUpdate
			},
			expectedError: "field 'testBridge.FlowTables[255]' (value: '255') failed on rule 'max' (param: 254)",
		},
		// Test immutable field update attempt
		{
			name: "update immutable field (Name)",
			updateModelFn: func(current *testBridge) *testBridge {
				modelToUpdate := *current
				modelToUpdate.Name = "attempt-to-change-immutable-name" // Name is mutable: false in schema
				return &modelToUpdate
			},
			// Update API call itself does not error on immutable checks during model validation step.
			// The immutable field is silently removed from the row before OVS op generation.
			// If *all* fields in the update are immutable (or non-existent), then it might error with "empty row".
			// In this specific case, other valid fields like DatapathType would still be in the model, so validateModel passes.
			// The 'Name' field will just be omitted from the actual OVSDB update operation.
			expectedError: "", // No error expected from validateModel for this specific case.
		},
		{
			name: "update with only immutable field",
			updateModelFn: func(current *testBridge) *testBridge {
				// Model with a non-default immutable field and default mutable fields
				modelToUpdate := &testBridge{
					UUID: current.UUID,                     // For Where() condition
					Name: "attempting-to-change-name-only", // This is immutable
				}
				return modelToUpdate
			},
			// When we specify Name field for Update, it will check if Name is mutable first
			// This will return a ValidationError now
			expectedError: "unable to update field name of table Bridge as it is not mutable",
			useFieldNames: []string{"Name"},
		},
	}

	for _, tc := range testCases {
		t.Run("Update_"+tc.name, func(t *testing.T) {
			currentBridgeState := getFreshBridgeStateFromCache(t)
			modelToUpdateWith := tc.updateModelFn(currentBridgeState)

			// Prepare field pointers if specific field names were specified
			var fieldPointers []interface{}
			if len(tc.useFieldNames) > 0 {
				for _, fieldName := range tc.useFieldNames {
					switch fieldName {
					case "Name":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.Name)
					case "DatapathType":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.DatapathType)
					case "DatapathVersion":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.DatapathVersion)
					case "FailMode":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.FailMode)
					case "STPEnable":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.STPEnable)
					case "FloodVLANs":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.FloodVLANs)
					case "FlowTables":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.FlowTables)
					case "Protocols":
						fieldPointers = append(fieldPointers, &modelToUpdateWith.Protocols)
						// Add other fields as needed
					}
				}
			}

			// Condition to select the object to update
			condAPI := api.Where(&testBridge{UUID: initialBridgeUUID}) // Use initial UUID for condition

			var ops []ovsdb.Operation
			var err error

			// Use fieldPointers if provided, otherwise do standard update
			if len(fieldPointers) > 0 {
				ops, err = condAPI.Update(modelToUpdateWith, fieldPointers...)
			} else {
				ops, err = condAPI.Update(modelToUpdateWith)
			}

			if tc.expectedError != "" {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				// Check if the error message contains the expected substring
				require.ErrorContains(t, err, tc.expectedError, "Error message mismatch for: %s. New error: %s", tc.name, err.Error())

				// For validation errors, also check that they can be extracted
				var valErrs validator.ValidationErrors
				if errors.As(err, &valErrs) {
					// This is a validation error, which is expected for model validation failures
					errMsg := err.Error()
					assert.True(t, strings.Contains(errMsg, "model validation failed") || strings.Contains(errMsg, "mutation validation failed"), "Should be a validation error for: %s", tc.name)
				}
				// Note: Immutable field errors are not validation errors, they are different types of errors

				assert.Nil(t, ops, "Operations should be nil on error for: %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for test case: %s", tc.name)
				assert.NotNil(t, ops, "Operations should not be nil for valid case: %s", tc.name)
				require.GreaterOrEqual(t, len(ops), 1, "Need at least one operation for: %s", tc.name)
				assert.Equal(t, string(ovsdb.OperationUpdate), ops[0].Op, "Operation should be update for: %s", tc.name)
				// Further checks for row content, ensuring immutable fields are not in ops[0].Row etc.
				// For example, Name should not be in ops[0].Row if it was in modelToUpdateWith for a valid case.
				if modelToUpdateWith.Name != initialBridge.Name { // If an attempt was made to change Name
					_, nameInOpRow := ops[0].Row["name"]
					assert.False(t, nameInOpRow, "Immutable field 'Name' should not be in the update operation row for: %s", tc.name)
				}
			}
		})
	}
}
