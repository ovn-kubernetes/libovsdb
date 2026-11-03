package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// Database abstracts a database that a server can use to store and transact data
type Database interface {
	CreateDatabase(ctx context.Context, database string, model ovsdb.DatabaseSchema) error
	Exists(ctx context.Context, database string) bool
	NewTransaction(ctx context.Context, database string) Transaction
	Commit(ctx context.Context, database string, id uuid.UUID, update Update) error
	CheckIndexes(ctx context.Context, database string, table string, m model.Model) error
	List(ctx context.Context, database, table string, conditions ...ovsdb.Condition) (map[string]model.Model, error)
	Get(ctx context.Context, database, table string, uuid string) (model.Model, error)
	GetReferences(ctx context.Context, database, table, row string) (References, error)
}

// Transaction abstracts a database transaction that can generate database
// updates
type Transaction interface {
	Transact(ctx context.Context, operations ...ovsdb.Operation) ([]*ovsdb.OperationResult, Update)
}

// Update abstracts an update that can be committed to a database
type Update interface {
	GetUpdatedTables() []string
	ForEachModelUpdate(table string, do func(uuid string, old, newModel model.Model) error) error
	ForEachRowUpdate(table string, do func(uuid string, row ovsdb.RowUpdate2) error) error
	ForReferenceUpdates(do func(references References) error) error
}
