package cache

import (
	"encoding/json"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestForwardCompatibilityIndexes(t *testing.T) {
	type row struct {
		UUID string `ovsdb:"_uuid"`
		Name string `ovsdb:"name"`
	}
	client, err := model.NewClientDBModel("Compat", map[string]model.Model{"Row": &row{}})
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Compat","tables":{"Row":{"columns":{"name":{"type":"string"},"future":{"type":"string"}},"indexes":[["name","future"],["name"]]}}}`), &schema))
	db, errs := model.NewDatabaseModel(schema, client)
	require.Empty(t, errs)
	cache, err := NewTableCache(db, nil, nil)
	require.NoError(t, err)
	rc := cache.Table("Row")
	require.Len(t, rc.indexSpecs, 1)
	require.Equal(t, newIndexFromColumns("name"), rc.indexSpecs[0].index)
	require.NoError(t, rc.Create("uuid", &row{UUID: "uuid", Name: "hello"}, true))
	_, found, err := rc.RowByModel(&row{Name: "hello"})
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Error(t, rc.Create("duplicate", &row{UUID: "duplicate", Name: "hello"}, true))
	require.Len(t, db.Schema.Table("Row").Indexes, 2, "the server schema must not be changed")
}

func TestCompatibilityIndexes(t *testing.T) {
	type row struct {
		UUID   string `ovsdb:"_uuid"`
		Name   string `ovsdb:"name"`
		Future string `ovsdb:"future"`
	}
	client, err := model.NewClientDBModel("Compat", map[string]model.Model{"Row": &row{}})
	require.NoError(t, err)
	client.SetCompatibility(true)
	client.SetIndexes(map[string][]model.ClientIndex{"Row": {
		{Columns: []model.ColumnKey{{Column: "name"}}},
		{Columns: []model.ColumnKey{{Column: "name"}, {Column: "future"}}},
	}})
	for _, full := range []bool{false, true, false} {
		columns := `{"name":{"type":"string"}}`
		if full {
			columns = `{"name":{"type":"string"},"future":{"type":"string"}}`
		}
		var schema ovsdb.DatabaseSchema
		require.NoError(t, json.Unmarshal([]byte(`{"name":"Compat","tables":{"Row":{"columns":`+columns+`}}}`), &schema))
		db, errs := model.NewDatabaseModel(schema, client)
		require.Empty(t, errs)
		cache, err := NewTableCache(db, nil, nil)
		require.NoError(t, err)
		rc := cache.Table("Row")
		count := 1
		if full {
			count = 2
		}
		require.Len(t, rc.indexSpecs, count)
		require.NoError(t, rc.Create("uuid", &row{UUID: "uuid", Name: "hello"}, false))
		found, err := rc.RowsByModels([]model.Model{&row{Name: "hello"}})
		require.NoError(t, err)
		require.Contains(t, found, "uuid")
	}
	require.Len(t, client.Indexes("Row"), 2)
	// Compatibility must not hide typos in index definitions that refer to fields
	// absent from the model itself.
	client.SetIndexes(map[string][]model.ClientIndex{"Row": {{Columns: []model.ColumnKey{{Column: "typo"}}}}})
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Compat","tables":{"Row":{"columns":{"name":{"type":"string"}}}}}`), &schema))
	_, errs := model.NewDatabaseModel(schema, client)
	require.NotEmpty(t, errs)
}
