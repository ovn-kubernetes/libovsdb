package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"github.com/stretchr/testify/require"
)

// Load unmodified release schemas, not reduced copies: validation and monitoring
// must account for every table, column, reference and index in the real schema.
func ovnReleaseSchema(t *testing.T, release, database string) ovsdb.DatabaseSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "ovn", release, "ovn-"+database+".ovsschema"))
	require.NoError(t, err)
	var schema ovsdb.DatabaseSchema
	require.NoError(t, json.Unmarshal(data, &schema))
	return schema
}

// Build tagged Go models for all schema columns without checking in hundreds of
// generated Go files. This tests client/schema compatibility, not modelgen. Each
// field uses the schema's native Go type, just as generated models do.
func ovnReleaseModel(t *testing.T, schema ovsdb.DatabaseSchema) model.ClientDBModel {
	t.Helper()
	models := make(map[string]model.Model, len(schema.Tables))
	for table, tableSchema := range schema.Tables {
		// Distinguish tables even if they happen to have identical columns.
		fields := []reflect.StructField{{Name: "Table", Type: reflect.TypeOf(false), Tag: reflect.StructTag(fmt.Sprintf("table:%q", table))}}
		columns := []string{"_uuid"}
		for column := range tableSchema.Columns {
			columns = append(columns, column)
		}
		sort.Strings(columns)
		for _, column := range columns {
			fields = append(fields, reflect.StructField{
				Name: "Column_" + column,
				Type: ovsdb.NativeType(tableSchema.Column(column)),
				Tag:  reflect.StructTag(fmt.Sprintf("ovsdb:%q", column)),
			})
		}
		models[table] = reflect.New(reflect.StructOf(fields)).Interface()
	}
	db, err := model.NewClientDBModel(schema.Name, models)
	require.NoError(t, err)
	return db
}

func ovnReleaseRow(db model.ClientDBModel, table string) model.Model {
	return reflect.New(db.Types()[table].Elem()).Interface()
}

func ovnColumn(row model.Model, column string) reflect.Value {
	return reflect.ValueOf(row).Elem().FieldByName("Column_" + column)
}

func TestOVNReleaseCompatibility(t *testing.T) {
	for _, database := range []string{"nb", "sb"} {
		t.Run(database, func(t *testing.T) {
			schemas := map[string]ovsdb.DatabaseSchema{
				"25.09": ovnReleaseSchema(t, "25.09", database),
				"26.03": ovnReleaseSchema(t, "26.03", database),
			}
			if database == "nb" {
				require.Equal(t, "7.12.0", schemas["25.09"].Version)
				require.Equal(t, "7.18.0", schemas["26.03"].Version)
				require.Len(t, schemas["25.09"].Tables, 35)
				require.Len(t, schemas["26.03"].Tables, 39)
			} else {
				require.Equal(t, "21.5.0", schemas["25.09"].Version)
				require.Equal(t, "21.8.0", schemas["26.03"].Version)
				require.Len(t, schemas["25.09"].Tables, 38)
				require.Len(t, schemas["26.03"].Tables, 39)
			}
			for _, serverRelease := range []string{"25.09", "26.03"} {
				t.Run("server_"+serverRelease, func(t *testing.T) {
					serverSchema := schemas[serverRelease]
					_, socket := newOVSDBServer(t, ovnReleaseModel(t, serverSchema), serverSchema)
					for _, clientRelease := range []string{"25.09", "26.03"} {
						for _, compatible := range []bool{false, true} {
							t.Run(fmt.Sprintf("client_%s/compatibility_%t", clientRelease, compatible), func(t *testing.T) {
								clientSchema := schemas[clientRelease]
								clientModel := ovnReleaseModel(t, clientSchema)
								ovs, err := NewOVSDBClient(clientModel, WithEndpoint("unix:"+socket), WithCompatibility(compatible))
								require.NoError(t, err)
								defer ovs.Close()
								ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer cancel()
								err = ovs.Connect(ctx)
								if clientRelease == "26.03" && serverRelease == "25.09" && !compatible {
									require.Error(t, err, "strict validation must reject the newer model")
									return
								}
								require.NoError(t, err)
								_, err = ovs.MonitorAll(ctx)
								require.NoError(t, err)

								// Check every modeled table and column, including the implicit UUID.
								for table, tableSchema := range clientSchema.Tables {
									row := ovnReleaseRow(clientModel, table)
									runtimeTable := serverSchema.Table(table)
									require.Equal(t, runtimeTable != nil, ovs.IsTableSupported(row), table)
									if runtimeTable == nil {
										require.Nil(t, ovs.Cache().Table(table), table)
									} else {
										require.NotNil(t, ovs.Cache().Table(table), table)
									}
									columns := []string{"_uuid"}
									for column := range tableSchema.Columns {
										columns = append(columns, column)
									}
									for _, column := range columns {
										supported := runtimeTable != nil && runtimeTable.Column(column) != nil
										require.Equal(t, supported, ovs.IsColumnSupported(row, ovnColumn(row, column).Addr().Interface()), table+"."+column)
									}
								}

								if clientRelease == "26.03" {
									// Exercise actual additions from these releases, not synthetic fields.
									table, column := "Logical_Switch_Port", "health_checks"
									var value any = []string{"00000000-0000-0000-0000-000000000001"}
									if database == "sb" {
										table, column, value = "Service_Monitor", "mac", "00:00:00:00:00:01"
									}
									row := ovnReleaseRow(clientModel, table)
									ovnColumn(row, column).Set(reflect.ValueOf(value))
									ops, err := ovs.Create(row)
									require.NoError(t, err)
									require.Len(t, ops, 1)
									if serverRelease == "25.09" {
										require.NotContains(t, ops[0].Row, column)
										require.NotEmpty(t, ovs.NewMonitor(WithTable(row, ovnColumn(row, column).Addr().Interface())).Errors)
									} else {
										require.Contains(t, ops[0].Row, column)
										require.Empty(t, ovs.NewMonitor(WithTable(row, ovnColumn(row, column).Addr().Interface())).Errors)
									}
								}

								// Complete a real transaction and observe its monitor/cache update.
								// Both tables are roots and require no references to other rows.
								table, column := "Logical_Switch", "name"
								if database == "sb" {
									table, column = "Service_Monitor", "logical_port"
								}
								row := ovnReleaseRow(clientModel, table)
								ovnColumn(row, column).SetString(t.Name())
								ops, err := ovs.Create(row)
								require.NoError(t, err)
								results, err := ovs.Transact(ctx, ops...)
								require.NoError(t, err)
								opErrs, err := ovsdb.CheckOperationResults(results, ops)
								require.NoError(t, err)
								require.Empty(t, opErrs)
								uuid := results[0].UUID.GoUUID
								require.NotEmpty(t, uuid)
								require.Eventually(t, func() bool {
									cached := ovs.Cache().Table(table).Row(uuid)
									return cached != nil && ovnColumn(cached, column).String() == t.Name()
								}, time.Second, 10*time.Millisecond)
							})
						}
					}
				})
			}
		})
	}
}
