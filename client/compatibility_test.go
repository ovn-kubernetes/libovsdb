package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

type compatServerRow struct {
	UUID string `ovsdb:"_uuid"`
	Name string `ovsdb:"name"`
}
type compatClientRow struct {
	UUID   string `ovsdb:"_uuid"`
	Name   string `ovsdb:"name"`
	Future string `ovsdb:"future"`
}
type compatMissingTable struct {
	UUID string `ovsdb:"_uuid"`
}

func TestCompatibilityClientServer(t *testing.T) {
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Compat","tables":{"Row":{"isRoot":true,"columns":{"name":{"type":"string"}}}}}`), &schema))
	serverModel, err := model.NewClientDBModel("Compat", map[string]model.Model{"Row": &compatServerRow{}})
	require.NoError(t, err)
	_, socket := newOVSDBServer(t, serverModel, schema)
	clientModel, err := model.NewClientDBModel("Compat", map[string]model.Model{"Row": &compatClientRow{}, "Future": &compatMissingTable{}})
	require.NoError(t, err)
	clientModel.SetIndexes(map[string][]model.ClientIndex{
		"Row":    {{Columns: []model.ColumnKey{{Column: "name"}}}, {Columns: []model.ColumnKey{{Column: "name"}, {Column: "future"}}}},
		"Future": {{Columns: []model.ColumnKey{{Column: "_uuid"}}}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	strict, err := NewOVSDBClient(clientModel, WithEndpoint("unix:"+socket))
	require.NoError(t, err)
	defer strict.Close()
	require.Error(t, strict.Connect(ctx))
	compatible, err := NewOVSDBClient(clientModel, WithEndpoint("unix:"+socket), WithCompatibility(true))
	require.NoError(t, err)
	defer compatible.Close()
	row := &compatClientRow{Name: "hello", Future: "not sent"}
	require.False(t, compatible.IsTableSupported(row))
	require.False(t, compatible.IsColumnSupported(row, &row.Name))
	require.NoError(t, compatible.Connect(ctx))
	require.True(t, compatible.IsTableSupported(row))
	require.False(t, compatible.IsTableSupported(&compatMissingTable{}))
	require.True(t, compatible.IsColumnSupported(row, &row.Name))
	require.True(t, compatible.IsColumnSupported(row, &row.UUID))
	require.False(t, compatible.IsColumnSupported(row, &row.Future))
	unrelated := &compatClientRow{}
	require.False(t, compatible.IsColumnSupported(row, &unrelated.Name))
	require.False(t, compatible.IsColumnSupported(row, 42))
	require.False(t, compatible.IsColumnSupported(nil, nil))
	require.False(t, compatible.IsColumnSupported((*compatClientRow)(nil), (*string)(nil)))
	require.NotEmpty(t, compatible.NewMonitor(WithTable(&compatMissingTable{})).Errors)
	invalidMonitor := newMonitor()
	invalidMonitor.Tables = []TableMonitor{{Table: "Future"}}
	_, err = compatible.Monitor(ctx, invalidMonitor)
	require.Error(t, err)
	_, err = compatible.MonitorAll(ctx)
	require.NoError(t, err)
	require.Nil(t, compatible.Cache().Table("Future"))
	_, err = compatible.Create(&compatMissingTable{})
	require.Error(t, err)
	require.NotEmpty(t, compatible.NewMonitor(WithTable(row, &row.Future)).Errors)
	ops, err := compatible.Create(row)
	require.NoError(t, err)
	require.NotContains(t, ops[0].Row, "future")
	results, err := compatible.Transact(ctx, ops...)
	require.NoError(t, err)
	opErrs, err := ovsdb.CheckOperationResults(results, ops)
	require.NoError(t, err)
	require.Empty(t, opErrs)
	require.Eventually(t, func() bool { return len(compatible.Cache().Table("Row").Rows()) == 1 }, time.Second, 10*time.Millisecond)
	var rows []compatClientRow
	require.NoError(t, compatible.List(ctx, &rows))
	require.Len(t, rows, 1)
	require.Equal(t, "hello", rows[0].Name)
	require.Empty(t, rows[0].Future)
	// A client index on supported columns remains usable; a partially supported
	// composite index must not cause cache insertion failures or be shortened.
	var byName compatClientRow
	byName.Name = "hello"
	matches, err := compatible.Cache().Table("Row").RowsByModels([]model.Model{&byName})
	require.NoError(t, err)
	require.Contains(t, matches, rows[0].UUID)
	disconnected := compatible.DisconnectNotify()
	compatible.Disconnect()
	select {
	case <-disconnected:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, compatible.Connect(ctx))
	_, err = compatible.MonitorAll(ctx)
	require.NoError(t, err)
	require.False(t, compatible.IsColumnSupported(row, &row.Future))
	// Enabling compatibility on one client must not mutate the caller's model.
	_, errs := model.NewDatabaseModel(schema, clientModel)
	require.NotEmpty(t, errs)
}
