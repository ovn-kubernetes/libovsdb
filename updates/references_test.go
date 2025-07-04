package updates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovn-kubernetes/libovsdb/database"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

const referencesTestSchema = `
{
    "name": "References_Test",
    "version": "0.0.1",
    "tables": {
        "Parent": {
            "columns": {
                "strong_atomic_required_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child"
                        },
                        "min": 1,
                        "max": 1
                    }
                },
                "strong_atomic_optional_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child"
                        },
                        "min": 0,
                        "max": 1
                    }
                },
                "strong_set_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                },
                "strong_map_key_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child"
                        },
                        "value": {
                            "type": "string"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                },
                "strong_map_value_reference": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "value": {
                            "type": "uuid",
                            "refTable": "Child"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "weak_atomic_required_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "min": 1,
                        "max": 1
                    }
                },
                "weak_atomic_optional_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "min": 0,
                        "max": 1
                    }
                },
                "weak_set_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "min": 2,
                        "max": "unlimited"
                    }
                },
                "weak_map_key_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "value": {
                            "type": "string"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "weak_map_value_reference": {
                    "type": {
                        "key": {
                            "type": "string"
                        },
                        "value": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "min": 1,
                        "max": "unlimited"
                    }
                },
                "map_key_value_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "weak"
                        },
                        "value": {
                            "type": "uuid",
                            "refTable": "Child",
                            "refType": "strong"
                        },
                        "min": 0,
                        "max": "unlimited"
                    }
                }
            },
            "isRoot": true
        },
        "Child": {
            "columns": {
                "name": {
                    "type": "string",
                    "mutable": false
                },
                "strong_atomic_optional_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Grandchild"
                        },
                        "min": 0,
                        "max": 1
                    }
                },
                "weak_atomic_optional_reference": {
                    "type": {
                        "key": {
                            "type": "uuid",
                            "refTable": "Grandchild",
                            "refType": "weak"
                        },
                        "min": 0,
                        "max": 1
                    }
                }
            },
            "indexes": [
                [
                    "name"
                ]
            ]
        },
        "Grandchild": {
            "columns": {
                "name": {
                    "type": "string",
                    "mutable": false
                }
            },
            "indexes": [
                [
                    "name"
                ]
            ]
        }
    }
}
`

type Parent struct {
	UUID                          string            `ovsdb:"_uuid"`
	StrongAtomicRequiredReference string            `ovsdb:"strong_atomic_required_reference"`
	StrongAtomicOptionalReference *string           `ovsdb:"strong_atomic_optional_reference"`
	StrongSetReference            []string          `ovsdb:"strong_set_reference"`
	StrongMapKeyReference         map[string]string `ovsdb:"strong_map_key_reference"`
	StrongMapValueReference       map[string]string `ovsdb:"strong_map_value_reference"`
	WeakAtomicRequiredReference   string            `ovsdb:"weak_atomic_required_reference"`
	WeakAtomicOptionalReference   *string           `ovsdb:"weak_atomic_optional_reference"`
	WeakSetReference              []string          `ovsdb:"weak_set_reference"`
	WeakMapKeyReference           map[string]string `ovsdb:"weak_map_key_reference"`
	WeakMapValueReference         map[string]string `ovsdb:"weak_map_value_reference"`
	MapKeyValueReference          map[string]string `ovsdb:"map_key_value_reference"`
}

type Child struct {
	UUID                          string  `ovsdb:"_uuid"`
	StrongAtomicOptionalReference *string `ovsdb:"strong_atomic_optional_reference"`
	WeakAtomicOptionalReference   *string `ovsdb:"weak_atomic_optional_reference"`
}

type Grandchild struct {
	UUID string `ovsdb:"_uuid"`
}

func getReferencesTestDBModel() (model.DatabaseModel, error) {
	client, err := model.NewClientDBModel(
		"References_Test",
		map[string]model.Model{
			"Parent":     &Parent{},
			"Child":      &Child{},
			"Grandchild": &Grandchild{},
		},
	)
	if err != nil {
		return model.DatabaseModel{}, err
	}
	schema, err := getReferencesTestSchema()
	if err != nil {
		return model.DatabaseModel{}, err
	}
	dbModel, errs := model.NewDatabaseModel(schema, client)
	if len(errs) > 0 {
		return model.DatabaseModel{}, fmt.Errorf("errors build model: %v", errs)
	}
	return dbModel, nil
}

func getReferencesTestSchema() (ovsdb.DatabaseSchema, error) {
	var dbSchema ovsdb.DatabaseSchema
	err := json.Unmarshal([]byte(referencesTestSchema), &dbSchema)
	return dbSchema, err
}

type testReferenceProvider struct {
	models     map[string]model.Model
	references database.References
}

func (rp *testReferenceProvider) GetReferences(_, table, uuid string) (database.References, error) {
	return rp.references.GetReferences(table, uuid), nil
}

func (rp *testReferenceProvider) Get(_, _ string, uuid string) (model.Model, error) {
	return rp.models[uuid], nil
}

var (
	referencesTestDBModel model.DatabaseModel
)

func ptr(s string) *string {
	return &s
}

type testData struct {
	existingModels        []model.Model
	updatedModels         []model.Model
	finalModels           []model.Model
	existingReferences    database.References
	wantUpdatedReferences database.References
}

func TestProcessReferences(t *testing.T) {
	var err error
	referencesTestDBModel, err = getReferencesTestDBModel()
	if err != nil {
		t.Errorf("error building DB model: %v", err)
	}

	tests := []struct {
		name     string
		testData testData
		wantErr  bool
	}{
		{
			// when a strong reference is replaced with another in a required atomic
			// field, the referenced row should be deleted
			name:     "strong atomic required reference garbage collected when replaced",
			testData: strongAtomicRequiredReferenceTestData(),
		},
		{
			// attempting to delete a row that is strongly referenced from a
			// required atomic field should fail
			name:     "constraint violation when strongly referenced row from required field deleted",
			testData: strongAtomicRequiredReferenceDeleteConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to add a required strong reference to a nonexistent row should
			// fail
			name:     "constraint violation when strong required reference to nonexistent row added",
			testData: strongAtomicRequiredReferenceAddConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// when a strong reference is removed from an optional atomic field, the
			// referenced row should be deleted
			name:     "strong atomic optional reference garbage collected when removed",
			testData: strongAtomicOptionalReferenceTestData(),
		},
		{
			// attempting to delete a row that is strongly referenced from an
			// optional atomic field should fail
			name:     "constraint violation when strongly referenced row from optional field deleted",
			testData: strongAtomicOptionalReferenceDeleteConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to add a optional strong reference to a nonexistent
			// row should fail
			name:     "constraint violation when strong optional reference to nonexistent row added",
			testData: strongAtomicOptionalReferenceAddConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// when a strong reference is removed from a set, the referenced row should
			// be deleted
			name:     "strong reference garbage collected when removed from set",
			testData: strongSetReferenceTestData(),
		},
		{
			// attempting to remove a row that is still strongly referenced in a set should fail
			name:     "strong set reference constraint violation when row deleted error",
			testData: strongSetReferenceDeleteConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to add strong set reference to non existent row should fail
			name:     "strong set reference constraint violation when nonexistent reference added error",
			testData: strongSetReferenceAddConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// when a strong reference is removed from a map key, the
			// referenced row should be deleted
			name:     "strong reference garbage collected when removed from map key",
			testData: strongMapKeyReferenceTestData(),
		},
		{
			// attempting to remove a row that is still strongly referenced in a
			// map key should fail
			name:     "strong map key reference constraint violation when row deleted error",
			testData: strongMapKeyReferenceDeleteConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to add strong map key reference to non existent row should fail
			name:     "strong map key reference constraint violation when nonexistent reference added error",
			testData: strongMapKeyReferenceAddConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// when a strong reference is removed from a map value, the
			// referenced row should be deleted
			name:     "strong reference garbage collected when removed from map value",
			testData: strongMapValueReferenceTestData(),
		},
		{
			// attempting to remove a row that is still strongly referenced in a
			// map value should fail
			name:     "strong map value reference constraint violation when row deleted error",
			testData: strongMapValueReferenceDeleteConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to add strong map value reference to non existent row should fail
			name:     "strong map value reference constraint violation when nonexistent reference added error",
			testData: strongMapValueReferenceAddConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// when a weak referenced row is deleted, the reference on an atomic
			// optional field is also deleted
			name:     "weak atomic optional reference deleted when row deleted",
			testData: weakAtomicOptionalReferenceTestData(),
		},
		{
			// when a weak referenced row is deleted, the reference on an set is
			// also deleted
			name:     "weak reference deleted from set when row deleted",
			testData: weakSetReferenceTestData(),
		},
		{
			// when a weak referenced row is deleted, the reference on a map
			// key is also deleted
			name:     "weak reference deleted from map key when row deleted",
			testData: weakMapKeyReferenceTestData(),
		},
		{
			// when a weak referenced row is deleted, the reference on a map
			// value is also deleted
			name:     "weak reference deleted from map value when row deleted",
			testData: weakMapValueReferenceTestData(),
		},
		{
			// attempting to delete a weak referenced row when it is referenced
			// from an atomic required field will fail
			name:     "weak reference constraint violation in required atomic field when row deleted error",
			testData: weakAtomicReferenceConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to delete a weak referenced row when it is referenced
			// from an set that then becomes smaller than the minimum allowed
			// will fail
			name:     "weak reference constraint violation in set becoming smaller than allowed error",
			testData: weakSetReferenceConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to delete a weak referenced row when it is referenced
			// from a map key that then becomes smaller than the minimum
			// allowed will fail
			name:     "weak reference constraint violation in map key field becoming smaller than allowed error",
			testData: weakMapKeyReferenceConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// attempting to delete a weak referenced row when it is referenced
			// from a map value that then becomes smaller than the minimum
			// allowed will fail
			name:     "weak reference constraint violation in map value field becoming smaller than allowed error",
			testData: weakMapValueReferenceConstraintViolationErrorTestData(),
			wantErr:  true,
		},
		{
			// testing behavior with multiple combinations of references
			name:     "multiple strong and weak reference changes",
			testData: multipleReferencesTestData(),
		},
		{
			// corner case
			// inserting a row in a table that is not part of the root set and
			// is not strongly referenced is a noop
			name:     "insert unreferenced row in non root set table is a noop",
			testData: insertNoRootUnreferencedRowTestData(),
		},
		{
			// corner case
			// adding a weak reference to a nonexistent row is a noop
			name:     "insert weak reference to nonexistent row is a noop",
			testData: weakReferenceToNonExistentRowTestData(),
		},
		{
			// corner case
			// for a map holding weak key references to strong value references, when
			// the weak reference row is deleted, the map entry and the strongly
			// referenced row is also deleted
			name:     "map with key weak reference and value strong reference, weak reference and strong referenced row deleted",
			testData: mapKeyValueReferenceTestData(),
			wantErr:  false,
		},
		{
			// corner case
			// when a weak referenced row is deleted, multiple references on a map
			// value are also deleted
			name:     "multiple weak references deleted from map value when row deleted",
			testData: multipleWeakMapValueReferenceTestData(),
			wantErr:  false,
		},
		{
			// corner case when multiple rows are transitively & strongly
			// referenced, garbage collection happens transitively as well
			name:     "transitive strong references garbage collected when removed",
			testData: transitiveStrongReferenceTestData(),
		},
		{
			// corner case
			// when a strong referenced is removed, an unreferenced row will be
			// garbage collected and weak references to it removed
			name:     "transitive strong and weak references garbage collected when removed",
			testData: transitiveStrongAndWeakReferenceTestData(),
		},
		{
			// corner case
			// a row needs to have a weak reference garbage collected and
			// at the same time that row itself is garbage collected due to not
			// being strongly referenced
			name:     "strong and weak garbage collection over the same row doesn't fail",
			testData: sameRowStrongAndWeakReferenceTestData(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := tt.testData
			rp := testReferenceProvider{
				models:     indexModels(td.existingModels),
				references: td.existingReferences,
			}

			onUpdates, err := getUpdates(td.existingModels, td.updatedModels)
			require.NoError(t, err, "failed to build updates from existing and updated models")

			// need a copy easiest way to have it is generating the updates all
			// over again
			onUpdatesCopy, err := getUpdates(td.existingModels, td.updatedModels)
			require.NoError(t, err, "failed to build updates copy from existing and updated models")

			gotModelUpdates, gotReferenceModelUpdates, gotReferenceUpdates, err := ProcessReferences(referencesTestDBModel, &rp, onUpdates)
			if tt.wantErr {
				require.Errorf(t, err, "expected an error but got none")
				return
			}
			require.NoError(t, err, "got a different error than expected")

			//gotModelUpdates := gotUpdates.(modelUpdatesWithReferences).ModelUpdates
			wantModelUpdates, err := getUpdates(td.existingModels, td.finalModels)
			require.NoError(t, err, "failed to build updates from existing and final models")
			assert.Equal(t, wantModelUpdates, gotModelUpdates, "got different updates than expected")

			//gotUpdatedReferences := gotUpdates.(modelUpdatesWithReferences).references
			assert.Equal(t, td.wantUpdatedReferences, gotReferenceUpdates, "got different reference updates than expected")

			gotMergedModelUpdates := onUpdatesCopy
			err = gotMergedModelUpdates.Merge(referencesTestDBModel, gotReferenceModelUpdates)
			require.NoError(t, err)
			assert.Equal(t, gotModelUpdates, gotMergedModelUpdates,
				"the updates are not a result of merging the initial updates with the reference updates")
		})
	}
}

func getUUID(model model.Model) string {
	return reflect.ValueOf(model).Elem().FieldByName("UUID").Interface().(string)
}

func indexModels(models []model.Model) map[string]model.Model {
	indexed := map[string]model.Model{}
	for _, model := range models {
		indexed[getUUID(model)] = model
	}
	return indexed
}

// getUpdates returns the updates needed to go from existing to updated
func getUpdates(existing, updated []model.Model) (ModelUpdates, error) {
	// index the models by uuid
	existingModels := indexModels(existing)
	updatedModels := indexModels(updated)

	// helpers
	tables := map[string]string{}
	getRow := func(model model.Model, fields ...any) (ovsdb.Row, error) {
		info, err := referencesTestDBModel.NewModelInfo(model)
		if err != nil {
			return nil, err
		}
		row, err := referencesTestDBModel.Mapper.NewRow(info, fields...)
		if err != nil {
			return nil, err
		}
		tables[getUUID(model)] = info.Metadata.TableName
		return row, nil
	}

	getUpdateOp := func(old, newModel model.Model) (ovsdb.Operation, error) {
		var err error
		var row ovsdb.Row

		// insert
		if old == nil {
			row, err := getRow(newModel)
			return ovsdb.Operation{
				Op:    ovsdb.OperationInsert,
				Table: tables[getUUID(newModel)],
				Row:   row,
			}, err
		}

		// delete
		if newModel == nil {
			// lazy, just to cache the table of the row
			_, err := getRow(old)

			return ovsdb.Operation{
				Op:    ovsdb.OperationDelete,
				Table: tables[getUUID(old)],
				Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: getUUID(old)})},
			}, err
		}

		// update, just with the fields that have been changed
		fields := []any{}
		xv := reflect.ValueOf(newModel).Elem()
		xt := xv.Type()
		for i := 0; i < xt.NumField(); i++ {
			if !reflect.DeepEqual(xv.Field(i).Interface(), reflect.ValueOf(old).Elem().Field(i).Interface()) {
				fields = append(fields, xv.Field(i).Addr().Interface())
			}
		}

		row, err = getRow(newModel, fields...)
		return ovsdb.Operation{
			Op:    ovsdb.OperationUpdate,
			Table: tables[getUUID(newModel)],
			Row:   row,
			Where: []ovsdb.Condition{ovsdb.NewCondition("_uuid", ovsdb.ConditionEqual, ovsdb.UUID{GoUUID: getUUID(newModel)})},
		}, err

	}

	// generate updates
	updates := ModelUpdates{}
	for uuid, updatedModel := range updatedModels {
		op, err := getUpdateOp(existingModels[uuid], updatedModel)
		if err != nil {
			return updates, err
		}
		err = updates.AddOperation(referencesTestDBModel, tables[uuid], uuid, existingModels[uuid], &op)
		if err != nil {
			return updates, err
		}
	}

	// deletes
	for uuid := range existingModels {
		if updatedModels[uuid] != nil {
			continue
		}
		op, err := getUpdateOp(existingModels[uuid], nil)
		if err != nil {
			return updates, err
		}
		err = updates.AddOperation(referencesTestDBModel, tables[uuid], uuid, existingModels[uuid], &op)
		if err != nil {
			return updates, err
		}
	}

	return updates, nil
}

func strongAtomicRequiredReferenceTestData() testData {
	// when a strong reference is replaced with another in a required atomic
	// field, the referenced row should be deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "child",
			},
			&Child{
				UUID: "child",
			},
		},
		// newChild is added and parent reference is replaced with newChild
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "newChild",
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "newChild",
			},
		},
		// child model should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "newChild",
			},
			&Child{
				UUID: "newChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_required_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
		// child model is no longer referenced, newChild is
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_required_reference",
			}: database.Reference{
				"child":    nil,
				"newChild": []string{"parent"},
			},
		},
	}
}

func strongAtomicRequiredReferenceDeleteConstraintViolationErrorTestData() testData {
	// attempting to delete a row that is strongly referenced from a required
	// atomic field should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "child",
			},
			&Child{
				UUID: "child",
			},
		},
		// child is removed but will fail as it is still referenced
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "child",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_required_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func strongAtomicRequiredReferenceAddConstraintViolationErrorTestData() testData {
	// attempting to add a required strong reference to a nonexistent row should
	// fail
	return testData{
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicRequiredReference: "child",
			},
		},
	}
}

func strongAtomicOptionalReferenceTestData() testData {
	// when a strong reference is removed from an optional atomic field, the
	// referenced row should be deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID: "child",
			},
		},
		// parent reference to child is removed
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
			&Child{
				UUID: "child",
			},
		},
		// child model should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func strongAtomicOptionalReferenceDeleteConstraintViolationErrorTestData() testData {
	// attempting to delete a row that is strongly referenced from an optional
	// atomic field should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID: "child",
			},
		},
		// child is removed but will fail as it is still referenced
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func strongAtomicOptionalReferenceAddConstraintViolationErrorTestData() testData {
	// attempting to add a optional strong reference to a nonexistent row should
	// fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		// add reference to child but will fail as it does not exist
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
		},
	}
}

func strongSetReferenceTestData() testData {
	// when a strong reference is removed from a set, the referenced row should
	// be deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"child", "otherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child reference is removed from the set
		updatedModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"otherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child model should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"otherChild"},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func strongMapKeyReferenceTestData() testData {
	// when a strong reference is removed from a map key, the referenced row
	// should be deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapKeyReference: map[string]string{
					"child": "value",
				},
			},
			&Child{
				UUID: "child",
			},
		},
		// child reference is removed from the map
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
			&Child{
				UUID: "child",
			},
		},
		// child model should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_key_reference",
				FromValue:  false,
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_key_reference",
				FromValue:  false,
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func strongMapKeyReferenceDeleteConstraintViolationErrorTestData() testData {
	// attempting to remove a row that is still strongly referenced in a map key
	// should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapKeyReference: map[string]string{
					"child": "value",
				},
			},
			&Child{
				UUID: "child",
			},
		},
		// child is removed but will fail as it is still referenced
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapKeyReference: map[string]string{
					"child": "value",
				},
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_key_reference",
				FromValue:  false,
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func strongMapKeyReferenceAddConstraintViolationErrorTestData() testData {
	// attempting to add a map key strong reference to a nonexistent row should
	// fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		// child reference is added to the map but wil fail as child does not
		// exist
		updatedModels: []model.Model{
			&Parent{
				UUID:                  "parent",
				StrongMapKeyReference: map[string]string{"child": "value"},
			},
		},
	}
}

func strongMapValueReferenceTestData() testData {
	// when a strong reference is removed from a map value, the referenced row
	// should be deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapValueReference: map[string]string{
					"key1": "child",
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child reference is removed from the map
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapValueReference: map[string]string{
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child model should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapValueReference: map[string]string{
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func strongMapValueReferenceDeleteConstraintViolationErrorTestData() testData {
	// attempting to remove a row that is still strongly referenced in a map value
	// should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapKeyReference: map[string]string{
					"key": "child",
				},
			},
			&Child{
				UUID: "child",
			},
		},
		// child is removed but will fail as it is still referenced
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				StrongMapKeyReference: map[string]string{
					"key": "child",
				},
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func strongMapValueReferenceAddConstraintViolationErrorTestData() testData {
	// attempting to add a map key strong reference to a nonexistent row should
	// fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		// child reference is added to the map but wil fail as is it doesn't exist
		updatedModels: []model.Model{
			&Parent{
				UUID:                    "parent",
				StrongMapValueReference: map[string]string{"key": "child"},
			},
		},
	}
}

func strongSetReferenceDeleteConstraintViolationErrorTestData() testData {
	// attempting to remove a row that is still strongly referenced should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"child", "otherChild"},
			},
			&Parent{
				UUID:               "otherParent",
				StrongSetReference: []string{"child"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child is deleted from parent but will fail as it is still referenced
		// from other parent
		updatedModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"otherChild"},
			},
			&Parent{
				UUID:               "otherParent",
				StrongSetReference: []string{"child"},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child":      []string{"parent", "otherParent"},
				"otherChild": []string{"parent"},
			},
		},
	}
}

func strongSetReferenceAddConstraintViolationErrorTestData() testData {
	// attempting to add strong reference to non existent row should fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"child"},
			},
			&Child{
				UUID: "child",
			},
		},
		// otherChild reference is added to parent but will fail as otherChild
		// does not exist
		updatedModels: []model.Model{
			&Parent{
				UUID:               "parent",
				StrongSetReference: []string{"child", "otherChild"},
			},
			&Child{
				UUID: "child",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func weakAtomicOptionalReferenceTestData() testData {
	// when a weak referenced row is deleted, the reference on an atomic
	// optional field is also deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID: "child",
			},
		},
		// child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicOptionalReference: ptr("child"),
			},
		},
		// the reference to child should be removed from parent
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func weakAtomicReferenceConstraintViolationErrorTestData() testData {
	// an attempt to delete a weak referenced row when it is referenced from an
	// atomic required field will fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicRequiredReference: "child",
			},
			&Child{
				UUID: "child",
			},
		},
		// child is deleted, but will fail because that would leave a mandatory
		// field empty
		updatedModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicRequiredReference: "child",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_required_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func weakSetReferenceTestData() testData {
	// when a weak referenced row is deleted, the reference on an set is also
	// deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:             "parent",
				WeakSetReference: []string{"child", "otherChild", "thirdChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
			&Child{
				UUID: "thirdChild",
			},
		},
		// child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID:             "parent",
				WeakSetReference: []string{"child", "otherChild", "thirdChild"},
			},
			&Child{
				UUID: "otherChild",
			},
			&Child{
				UUID: "thirdChild",
			},
		},
		// the reference to child should be removed from parent
		finalModels: []model.Model{
			&Parent{
				UUID:             "parent",
				WeakSetReference: []string{"otherChild", "thirdChild"},
			},
			&Child{
				UUID: "otherChild",
			},
			&Child{
				UUID: "thirdChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_set_reference",
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
				"thirdChild": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_set_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func weakSetReferenceConstraintViolationErrorTestData() testData {
	// an attempt to delete a weak referenced row when it is referenced from a
	// set that then becomes smaller than the minimum allowed will fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:             "parent",
				WeakSetReference: []string{"child", "otherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child is deleted but will fail because the set becomes empty and
		// that is not allowed by the schema
		updatedModels: []model.Model{
			&Parent{
				UUID:             "parent",
				WeakSetReference: []string{"child", "otherChild"},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_set_reference",
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
	}
}

func weakMapKeyReferenceTestData() testData {
	// when a weak referenced row is deleted, the reference on a map
	// value is also deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapKeyReference: map[string]string{
					"child":      "value1",
					"otherChild": "value2",
				},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapKeyReference: map[string]string{
					"child":      "value1",
					"otherChild": "value2",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// the reference to child should be removed from parent
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapKeyReference: map[string]string{
					"otherChild": "value2",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_key_reference",
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_key_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func weakMapValueReferenceTestData() testData {
	// when a weak referenced row is deleted, the reference on a map
	// value is also deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// the reference to child should be removed from parent
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
		// child model is no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child": nil,
			},
		},
	}
}
func weakMapKeyReferenceConstraintViolationErrorTestData() testData {
	// an attempt to delete a weak referenced row when it is referenced from a
	// map key that then becomes smaller than the minimum allowed will fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapKeyReference: map[string]string{
					"child": "value",
				},
			},
			&Child{
				UUID: "child",
			},
		},
		// child is deleted but will fail because the map becomes empty and
		// that is not allowed by the schema
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapKeyReference: map[string]string{
					"child": "value",
				},
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_key_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func weakMapValueReferenceConstraintViolationErrorTestData() testData {
	// an attempt to delete a weak referenced row when it is referenced from a
	// map value that then becomes smaller than the minimum allowed will fail
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
				},
			},
			&Child{
				UUID: "child",
			},
		},
		// child is deleted but will fail because the map becomes empty and
		// that is not allowed by the schema
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
				},
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
	}
}

func mapKeyValueReferenceTestData() testData {
	// for a map holding weak key references to strong value references, when
	// the weak reference row is deleted, the map entry and the strongly
	// referenced row is also deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				MapKeyValueReference: map[string]string{
					"weakChild": "strongChild",
				},
			},
			&Child{
				UUID: "weakChild",
			},
			&Child{
				UUID: "strongChild",
			},
		},
		// weak child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				MapKeyValueReference: map[string]string{
					"weakChild": "strongChild",
				},
			},
			&Child{
				UUID: "strongChild",
			},
		},
		// the reference to weak child should be removed from parent
		// and strong child should be deleted
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "map_key_value_reference",
				FromValue:  false,
			}: database.Reference{
				"weakChild": []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "map_key_value_reference",
				FromValue:  true,
			}: database.Reference{
				"strongChild": []string{"parent"},
			},
		},
		// neither weak or strong child are referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "map_key_value_reference",
				FromValue:  false,
			}: database.Reference{
				"weakChild": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "map_key_value_reference",
				FromValue:  true,
			}: database.Reference{
				"strongChild": nil,
			},
		},
	}
}

func multipleWeakMapValueReferenceTestData() testData {
	// when a weak referenced row is deleted, multiple references on a map
	// value are also deleted
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
					"key2": "otherChild",
					"key3": "child",
				},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// child is deleted
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key1": "child",
					"key2": "otherChild",
					"key3": "child",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		// the reference to child should be removed from parent
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
				WeakMapValueReference: map[string]string{
					"key2": "otherChild",
				},
			},
			&Child{
				UUID: "otherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"parent"},
			},
		},
		// child model is no longer referenced, newChild is
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func transitiveStrongReferenceTestData() testData {
	// when multiple rows are transitively referenced, garbage collection
	// happens transitively as well
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID:                          "child",
				StrongAtomicOptionalReference: ptr("grandchild"),
			},
			&Grandchild{
				UUID: "grandchild",
			},
		},
		// parent reference to child is removed
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
			&Child{
				UUID:                          "child",
				StrongAtomicOptionalReference: ptr("grandchild"),
			},
			&Grandchild{
				UUID: "grandchild",
			},
		},
		// child and grandchild models should be deleted as it is no longer referenced
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Grandchild",
				FromTable:  "Child",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"grandchild": []string{"child"},
			},
		},
		// child and grandchild models are no longer referenced
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Grandchild",
				FromTable:  "Child",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"grandchild": nil,
			},
		},
	}
}

func transitiveStrongAndWeakReferenceTestData() testData {
	// when a strong referenced is removed, an unreferenced row will be garbage
	// collected and transitively, weak references to it removed
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
				WeakAtomicOptionalReference:   ptr("child"),
			},
			&Child{
				UUID: "child",
			},
		},
		// parent strong reference to child is removed
		updatedModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID: "child",
			},
		},
		// as a result, child and and the weak reference to it is removed
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
		},
		// child is no longer referenced at all
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func insertNoRootUnreferencedRowTestData() testData {
	return testData{
		// new child is inserted
		updatedModels: []model.Model{
			&Child{
				UUID: "newChild",
			},
		},
		// but is removed since is not referenced from anywhere and the table is
		// not part of the root set
		finalModels:           nil,
		wantUpdatedReferences: database.References{},
	}
}

func weakReferenceToNonExistentRowTestData() testData {
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		// a weak reference is added no nonexistent row
		updatedModels: []model.Model{
			&Parent{
				UUID:                        "parent",
				WeakAtomicOptionalReference: ptr("child"),
			},
		},
		// but is removed since the row does not exist
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
		},
	}
}

func sameRowStrongAndWeakReferenceTestData() testData {
	// a row needs to have a weak reference garbage collected and
	// at the same time that row itself is garbage collected due to not
	// being strongly referenced
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
			},
			&Child{
				UUID:                        "child",
				WeakAtomicOptionalReference: ptr("grandchild"),
			},
			&Grandchild{
				UUID: "grandchild",
			},
		},
		// parent strong reference to child is removed
		// grand child is removed as well
		updatedModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
			&Child{
				UUID:                        "child",
				WeakAtomicOptionalReference: ptr("grandchild"),
			},
		},
		// as a result, child is removed
		finalModels: []model.Model{
			&Parent{
				UUID: "parent",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Grandchild",
				FromTable:  "Child",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"grandchild": []string{"child"},
			},
		},
		// neither child nor grandchild are referenced at all
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Grandchild",
				FromTable:  "Child",
				FromColumn: "weak_atomic_optional_reference",
			}: database.Reference{
				"grandchild": nil,
			},
		},
	}
}

func multipleReferencesTestData() testData {
	// testing behavior with multiple combinations of references
	return testData{
		existingModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongSetReference:            []string{"child"},
				StrongAtomicOptionalReference: ptr("child"),
				WeakMapValueReference:         map[string]string{"key1": "yetAnotherChild", "key2": "otherChild"},
			},
			&Parent{
				UUID:                          "otherParent",
				StrongAtomicOptionalReference: ptr("child"),
				StrongSetReference:            []string{"otherChild"},
				WeakSetReference:              []string{"otherChild", "child", "yetAnotherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
			&Child{
				UUID: "yetAnotherChild",
			},
		},
		// all strong references to child except one are removed
		// single strong reference to otherChild is removed
		updatedModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
				WeakMapValueReference:         map[string]string{"key1": "yetAnotherChild", "key2": "otherChild"},
			},
			&Parent{
				UUID:             "otherParent",
				WeakSetReference: []string{"otherChild", "child", "yetAnotherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "otherChild",
			},
			&Child{
				UUID: "yetAnotherChild",
			},
		},
		// otherChild is garbage collected and all weak references to it removed
		finalModels: []model.Model{
			&Parent{
				UUID:                          "parent",
				StrongAtomicOptionalReference: ptr("child"),
				WeakMapValueReference:         map[string]string{"key1": "yetAnotherChild"},
			},
			&Parent{
				UUID:             "otherParent",
				WeakSetReference: []string{"child", "yetAnotherChild"},
			},
			&Child{
				UUID: "child",
			},
			&Child{
				UUID: "yetAnotherChild",
			},
		},
		existingReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child":      []string{"parent"},
				"otherChild": []string{"otherParent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent", "otherParent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"yetAnotherChild": []string{"parent"},
				"otherChild":      []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_set_reference",
			}: database.Reference{
				"otherChild":      []string{"otherParent"},
				"child":           []string{"otherParent"},
				"yetAnotherChild": []string{"otherParent"},
			},
		},
		// all strong references to child except one are removed
		// all references to otherChild are removed
		// references to yetAnotherChild are unchanged
		wantUpdatedReferences: database.References{
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_set_reference",
			}: database.Reference{
				"child":      nil,
				"otherChild": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "strong_atomic_optional_reference",
			}: database.Reference{
				"child": []string{"parent"},
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_map_value_reference",
				FromValue:  true,
			}: database.Reference{
				"otherChild": nil,
			},
			database.ReferenceSpec{
				ToTable:    "Child",
				FromTable:  "Parent",
				FromColumn: "weak_set_reference",
			}: database.Reference{
				"child":      []string{"otherParent"}, // this reference is read by the reference tracker, but not changed
				"otherChild": nil,
			},
		},
	}
}
