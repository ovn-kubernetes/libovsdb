package cache

import (
	"context"
	"testing"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

func TestPurgeReplacesSchemaTables(t *testing.T) {
	type a struct {
		UUID string `ovsdb:"_uuid"`
		Name string `ovsdb:"name"`
	}
	type b a
	type c a
	client, err := model.NewClientDBModel("Purge", map[string]model.Model{"A": &a{}, "B": &b{}, "C": &c{}})
	require.NoError(t, err)
	client.SetCompatibility(true)
	makeModel := func(tables ...string) model.DatabaseModel {
		schema := ovsdb.DatabaseSchema{Name: "Purge", Tables: map[string]ovsdb.TableSchema{}}
		for _, table := range tables {
			schema.Tables[table] = ovsdb.TableSchema{Columns: map[string]*ovsdb.ColumnSchema{"name": {Type: ovsdb.TypeString}}}
		}
		db, errs := model.NewDatabaseModel(schema, client)
		require.Empty(t, errs)
		return db
	}
	tc, err := NewTableCache(makeModel("A", "B"), Data{
		"A": {"old-a": &a{UUID: "old-a", Name: "old"}},
		"B": {"old-b": &b{UUID: "old-b", Name: "old"}},
	}, nil)
	require.NoError(t, err)
	require.Len(t, tc.Table("A").Rows(), 1)
	require.Len(t, tc.Table("B").Rows(), 1)

	tc.Purge(makeModel("A", "C"))
	require.ElementsMatch(t, []string{"A", "C"}, tc.Tables())
	require.Nil(t, tc.Table("B"))
	for _, table := range []string{"A", "C"} {
		require.NotNil(t, tc.Table(table))
		require.Empty(t, tc.Table(table).Rows())
		row := ovsdb.Row{"name": "new"}
		require.NoError(t, tc.Populate2(context.Background(), ovsdb.TableUpdates2{
			table: {"new": &ovsdb.RowUpdate2{Initial: &row}},
		}))
	}
	require.Equal(t, &a{UUID: "new", Name: "new"}, tc.Table("A").Row("new"))
	require.Equal(t, &c{UUID: "new", Name: "new"}, tc.Table("C").Row("new"))
}
