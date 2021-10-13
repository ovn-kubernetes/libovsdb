package model

import (
	"fmt"
	"reflect"

	"github.com/ovn-org/libovsdb/mapper"
	"github.com/ovn-org/libovsdb/ovsdb"
)

// DatabaseModelRequest contains the information needed to build a DatabaseModel
type DatabaseModelRequest struct {
	name  string
	types map[string]reflect.Type
}

// NewModel returns a new instance of a model from a specific string
func (db DatabaseModelRequest) newModel(table string) (Model, error) {
	mtype, ok := db.types[table]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database model", string(table))
	}
	model := reflect.New(mtype.Elem())
	return model.Interface().(Model), nil
}

// Name returns the database name
func (db DatabaseModelRequest) Name() string {
	return db.name
}

// Validate validates the DatabaseModel against the input schema
// Returns all the errors detected
func (db DatabaseModelRequest) validate(schema *ovsdb.DatabaseSchema) []error {
	var errors []error
	if db.name != schema.Name {
		errors = append(errors, fmt.Errorf("database model name (%s) does not match schema (%s)",
			db.name, schema.Name))
	}

	for tableName := range db.types {
		tableSchema := schema.Table(tableName)
		if tableSchema == nil {
			errors = append(errors, fmt.Errorf("database model contains a model for table %s that does not exist in schema", tableName))
			continue
		}
		model, err := db.newModel(tableName)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if _, err := mapper.NewInfo(tableName, tableSchema, model); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// NewDatabaseModelRequest constructs a DatabaseModelRequest based on a database name and dictionary of models indexed by table name
func NewDatabaseModelRequest(name string, models map[string]Model) (*DatabaseModelRequest, error) {
	types := make(map[string]reflect.Type, len(models))
	for table, model := range models {
		modelType := reflect.TypeOf(model)
		if modelType.Kind() != reflect.Ptr || modelType.Elem().Kind() != reflect.Struct {
			return nil, fmt.Errorf("model is expected to be a pointer to struct")
		}
		hasUUID := false
		for i := 0; i < modelType.Elem().NumField(); i++ {
			if field := modelType.Elem().Field(i); field.Tag.Get("ovsdb") == "_uuid" &&
				field.Type.Kind() == reflect.String {
				hasUUID = true
			}
		}
		if !hasUUID {
			return nil, fmt.Errorf("model is expected to have a string field called uuid")
		}

		types[table] = reflect.TypeOf(model)
	}
	return &DatabaseModelRequest{
		types: types,
		name:  name,
	}, nil
}
