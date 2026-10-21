package inmemory

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/cache"
	dbase "github.com/ovn-kubernetes/libovsdb/database"
	"github.com/ovn-kubernetes/libovsdb/database/transaction"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
	"go.opentelemetry.io/otel"
)

type inMemoryDatabase struct {
	databases  map[string]*cache.TableCache
	models     map[string]model.ClientDBModel
	references map[string]dbase.References
	logger     *logr.Logger
	mutex      sync.RWMutex
}

func NewDatabase(models map[string]model.ClientDBModel, logger *logr.Logger) dbase.Database {
	return &inMemoryDatabase{
		databases:  make(map[string]*cache.TableCache),
		models:     models,
		references: make(map[string]dbase.References),
		mutex:      sync.RWMutex{},
		logger:     logger,
	}
}

func (db *inMemoryDatabase) NewTransaction(ctx context.Context, dbName string) dbase.Transaction {
	_, span := tracer.Start(ctx, "NewTransaction")
	defer span.End()
	db.mutex.Lock()
	defer db.mutex.Unlock()
	var model model.DatabaseModel
	if database, ok := db.databases[dbName]; ok {
		model = database.DatabaseModel()
	}
	transaction := transaction.NewTransaction(model, dbName, db, db.logger)
	return &transaction
}

func (db *inMemoryDatabase) CreateDatabase(ctx context.Context, name string, schema ovsdb.DatabaseSchema) error {
	_, span := tracer.Start(ctx, "CreateDatabase")
	defer span.End()
	db.mutex.Lock()
	defer db.mutex.Unlock()
	var mo model.ClientDBModel
	var ok bool
	if mo, ok = db.models[schema.Name]; !ok {
		return fmt.Errorf("no db model provided for schema with name %s", name)
	}
	dbModel, errs := model.NewDatabaseModel(schema, mo)
	if len(errs) > 0 {
		return fmt.Errorf("failed to create DatabaseModel: %#+v", errs)
	}
	database, err := cache.NewTableCache(dbModel, nil, nil)
	if err != nil {
		return err
	}
	db.databases[name] = database
	db.references[name] = make(dbase.References)
	return nil
}

func (db *inMemoryDatabase) Exists(ctx context.Context, name string) bool {
	_, span := tracer.Start(ctx, "Exists")
	defer span.End()
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	_, ok := db.databases[name]
	return ok
}

func (db *inMemoryDatabase) Commit(ctx context.Context, database string, _ uuid.UUID, update dbase.Update) error {
	ctx, span := tracer.Start(ctx, "Commit")
	defer span.End()
	if !db.Exists(ctx, database) {
		return fmt.Errorf("db does not exist")
	}
	db.mutex.RLock()
	targetDb := db.databases[database]
	db.mutex.RUnlock()

	err := targetDb.ApplyCacheUpdate(ctx, update)
	if err != nil {
		return err
	}

	return update.ForReferenceUpdates(func(references dbase.References) error {
		db.references[database].UpdateReferences(references)
		return nil
	})
}

func (db *inMemoryDatabase) CheckIndexes(ctx context.Context, database string, table string, m model.Model) error {
	ctx, span := tracer.Start(ctx, "CheckIndexes")
	defer span.End()
	if !db.Exists(ctx, database) {
		return nil
	}
	db.mutex.RLock()
	targetDb := db.databases[database]
	db.mutex.RUnlock()
	targetTable := targetDb.Table(table)
	return targetTable.IndexExists(m)
}

func (db *inMemoryDatabase) List(ctx context.Context, database, table string, conditions ...ovsdb.Condition) (map[string]model.Model, error) {
	ctx, span := tracer.Start(ctx, "List")
	defer span.End()
	if !db.Exists(ctx, database) {
		return nil, fmt.Errorf("db does not exist")
	}
	db.mutex.RLock()
	targetDb := db.databases[database]
	db.mutex.RUnlock()

	targetTable := targetDb.Table(table)
	if targetTable == nil {
		return nil, fmt.Errorf("table does not exist")
	}

	return targetTable.RowsByCondition(conditions)
}

func (db *inMemoryDatabase) Get(ctx context.Context, database, table string, uuid string) (model.Model, error) {
	ctx, span := tracer.Start(ctx, "Get")
	defer span.End()
	if !db.Exists(ctx, database) {
		return nil, fmt.Errorf("db does not exist")
	}
	db.mutex.RLock()
	targetDb := db.databases[database]
	db.mutex.RUnlock()

	targetTable := targetDb.Table(table)
	if targetTable == nil {
		return nil, fmt.Errorf("table does not exist")
	}
	return targetTable.Row(uuid), nil
}

func (db *inMemoryDatabase) GetReferences(ctx context.Context, database, table, row string) (dbase.References, error) {
	ctx, span := tracer.Start(ctx, "GetReferences")
	defer span.End()
	if !db.Exists(ctx, database) {
		return nil, fmt.Errorf("db does not exist")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return db.references[database].GetReferences(table, row), nil
}

var tracer = otel.Tracer("github.com/ovn-kubernetes/libovsdb/database/inmemory")
