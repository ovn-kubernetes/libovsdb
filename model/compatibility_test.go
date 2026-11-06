package model

import (
	"encoding/json"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/mapper"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

type compatibilityRow struct {
	UUID   string `ovsdb:"_uuid"`
	Name   string `ovsdb:"name"`
	Future string `ovsdb:"future"`
}
type compatibilityFuture struct {
	UUID string `ovsdb:"_uuid"`
}

func compatibilitySchema(t *testing.T, columns string, futureTable bool) ovsdb.DatabaseSchema {
	t.Helper()
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Compat","tables":{"Row":{"columns":`+columns+`}}}`), &schema))
	if futureTable {
		schema.Tables["Future"] = ovsdb.TableSchema{Columns: map[string]*ovsdb.ColumnSchema{}}
	}
	return schema
}

func TestAutomaticCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name, columns                  string
		futureTable, compatible, valid bool
	}{
		{"strict full", `{"name":{"type":"string"},"future":{"type":"string"}}`, true, false, true},
		{"strict missing table", `{"name":{"type":"string"},"future":{"type":"string"}}`, false, false, false},
		{"strict missing column", `{"name":{"type":"string"}}`, true, false, false},
		{"compatible missing table", `{"name":{"type":"string"},"future":{"type":"string"}}`, false, true, true},
		{"compatible missing column", `{"name":{"type":"string"}}`, true, true, true},
		{"compatible missing both", `{"name":{"type":"string"}}`, false, true, true},
		{"compatible full", `{"name":{"type":"string"},"future":{"type":"string"}}`, true, true, true},
		{"compatible wrong type", `{"name":{"type":"integer"}}`, false, true, false},
		{"server extra column", `{"name":{"type":"string"},"future":{"type":"string"},"extra":{"type":"boolean"}}`, true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientDBModel("Compat", map[string]Model{"Row": &compatibilityRow{}, "Future": &compatibilityFuture{}})
			require.NoError(t, err)
			client.SetCompatibility(tt.compatible)
			schema := compatibilitySchema(t, tt.columns, tt.futureTable)
			db, errs := NewDatabaseModel(schema, client)
			if !tt.valid {
				require.NotEmpty(t, errs)
				return
			}
			require.Empty(t, errs)
			require.Equal(t, tt.futureTable, db.Types()["Future"] != nil)
			row := &compatibilityRow{Name: "present", Future: "ignored"}
			info, err := db.NewModelInfo(row)
			require.NoError(t, err)
			wire, err := db.Mapper.NewRow(info)
			require.NoError(t, err)
			require.Equal(t, "present", wire["name"])
			if schema.Table("Row").Column("future") == nil {
				require.NotContains(t, wire, "future")
				_, err = info.ColumnByPtr(&row.Future)
				require.ErrorIs(t, err, mapper.ErrOmitted)
				_, err = db.Mapper.NewRow(info, &row.Future)
				require.ErrorIs(t, err, mapper.ErrOmitted)
				_, err = db.Mapper.NewRow(info, &row.UUID, &row.Name, &row.Future)
				require.ErrorIs(t, err, mapper.ErrOmitted)
				_, err = db.Mapper.NewCondition(info, &row.Future, ovsdb.ConditionEqual, "x")
				require.ErrorIs(t, err, mapper.ErrOmitted)
				_, err = db.Mapper.NewMutation(info, "future", ovsdb.MutateOperationInsert, "x")
				require.Error(t, err)
			} else {
				col, err := info.ColumnByPtr(&row.Future)
				require.NoError(t, err)
				require.Equal(t, "future", col)
			}
		})
	}
}

func TestCompatibilityRebuild(t *testing.T) {
	client, err := NewClientDBModel("Compat", map[string]Model{"Row": &compatibilityRow{}, "Future": &compatibilityFuture{}})
	require.NoError(t, err)
	client.SetCompatibility(true)
	for _, full := range []bool{false, true, false, true} {
		columns := `{"name":{"type":"string"}}`
		if full {
			columns = `{"name":{"type":"string"},"future":{"type":"string"}}`
		}
		db, errs := NewDatabaseModel(compatibilitySchema(t, columns, full), client)
		require.Empty(t, errs)
		require.Equal(t, full, db.Types()["Future"] != nil)
		info, err := db.NewModelInfo(&compatibilityRow{})
		require.NoError(t, err)
		require.Equal(t, !full, len(info.Metadata.OmittedFields) > 0)
		client = db.Client() // reconnect must retain the complete, unmodified model
	}
	require.Len(t, client.Types(), 2)
	wrong := compatibilitySchema(t, `{"name":{"type":"string"}}`, false)
	wrong.Name = "Wrong"
	_, errs := NewDatabaseModel(wrong, client)
	require.NotEmpty(t, errs)
	client.SetCompatibility(false)
	_, errs = NewDatabaseModel(compatibilitySchema(t, `{"name":{"type":"string"}}`, false), client)
	require.NotEmpty(t, errs)
}

func TestOptionalTableDatabaseModel(t *testing.T) {
	client, err := NewClientDBModel("Compat", map[string]Model{"Future": &compatibilityFuture{}}, map[string]bool{"Future": true})
	require.NoError(t, err)
	schema := compatibilitySchema(t, `{}`, false)
	db, errs := NewDatabaseModel(schema, client)
	require.Empty(t, errs)
	require.Empty(t, db.Types())
	require.Len(t, db.Client().Types(), 1)
	schema.Tables["Future"] = ovsdb.TableSchema{Columns: map[string]*ovsdb.ColumnSchema{}}
	db, errs = NewDatabaseModel(schema, db.Client())
	require.Empty(t, errs)
	require.Contains(t, db.Types(), "Future")
}

func TestCompatibilityInvalidIndexTable(t *testing.T) {
	client, err := NewClientDBModel("Compat", map[string]Model{"Row": &compatibilityRow{}})
	require.NoError(t, err)
	client.SetCompatibility(true)
	client.SetIndexes(map[string][]ClientIndex{"Typo": {{Columns: []ColumnKey{{Column: "_uuid"}}}}})
	_, errs := NewDatabaseModel(compatibilitySchema(t, `{}`, false), client)
	require.NotEmpty(t, errs)
}

func TestOptionalColumnPresent(t *testing.T) {
	row := &struct {
		UUID string `ovsdb:"_uuid"`
		Name string `ovsdb:"name,omitunsupported"`
	}{Name: "present"}
	client, err := NewClientDBModel("Compat", map[string]Model{"Row": row})
	require.NoError(t, err)
	db, errs := NewDatabaseModel(compatibilitySchema(t, `{"name":{"type":"string"}}`, false), client)
	require.Empty(t, errs)
	info, err := db.NewModelInfo(row)
	require.NoError(t, err)
	col, err := info.ColumnByPtr(&row.Name)
	require.NoError(t, err)
	require.Equal(t, "name", col)
	wire, err := db.Mapper.NewRow(info, &row.Name)
	require.NoError(t, err)
	require.Equal(t, "present", wire["name"])
}
