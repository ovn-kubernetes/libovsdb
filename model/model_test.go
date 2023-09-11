package model

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/stretchr/testify/assert"
)

type modelA struct {
	UUID string `ovsdb:"_uuid"`
}

type modelB struct {
	UID string `ovsdb:"_uuid"`
	Foo string `ovsdb:"bar"`
	Bar string `ovsdb:"baz"`
}

type modelInvalid struct {
	Foo string
}

func TestClientDBModel(t *testing.T) {
	type Test struct {
		name  string
		obj   map[string]Model
		valid bool
	}

	tests := []Test{
		{
			name:  "valid",
			obj:   map[string]Model{"Test_A": &modelA{}},
			valid: true,
		},
		{
			name: "valid_multiple",
			obj: map[string]Model{"Test_A": &modelA{},
				"Test_B": &modelB{}},
			valid: true,
		},
		{
			name:  "invalid",
			obj:   map[string]Model{"INVALID": &modelInvalid{}},
			valid: false,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("TestNewModel_%s", tt.name), func(t *testing.T) {
			db, err := NewClientDBModel(tt.name, tt.obj)
			if tt.valid {
				assert.Nil(t, err)
				assert.Len(t, db.types, len(tt.obj))
				assert.Equal(t, tt.name, db.Name())
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestNewModel(t *testing.T) {
	db, err := NewClientDBModel("testTable", map[string]Model{"Test_A": &modelA{}, "Test_B": &modelB{}})
	assert.Nil(t, err)
	_, err = db.newModel("Unknown")
	assert.NotNilf(t, err, "Creating model from unknown table should fail")
	model, err := db.newModel("Test_A")
	assert.Nilf(t, err, "Creating model from valid table should succeed")
	assert.IsTypef(t, model, &modelA{}, "model creation should return the appropriate type")
}

func TestSetUUID(t *testing.T) {
	var err error
	a := modelA{}
	err = modelSetUUID(&a, "foo")
	assert.Nilf(t, err, "Setting UUID should succeed")
	assert.Equal(t, "foo", a.UUID)
	b := modelB{}
	err = modelSetUUID(&b, "foo")
	assert.Nilf(t, err, "Setting UUID should succeed")
	assert.Equal(t, "foo", b.UID)

}

func TestValidate(t *testing.T) {
	model, err := NewClientDBModel("TestDB", map[string]Model{
		"TestTable": &struct {
			aUUID   string            `ovsdb:"_uuid"`
			aString string            `ovsdb:"aString"`
			aInt    int               `ovsdb:"aInt"`
			aFloat  float64           `ovsdb:"aFloat"`
			aSet    []string          `ovsdb:"aSet"`
			aMap    map[string]string `ovsdb:"aMap"`
		}{},
	})
	assert.Nil(t, err)

	tests := []struct {
		name   string
		schema []byte
		err    bool
	}{
		{
			name: "wrong name",
			schema: []byte(`{
			    "name": "Wrong"
			}`),
			err: true,
		},
		{
			name: "correct",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "aString": { "type": "string" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: false,
		},
		{
			name: "extra column should be OK",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "ExtraCol": { "type": "real" } ,
  			          "aString": { "type": "string" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: false,
		},
		{
			name: "extra table should be OK",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "ExtraTable": {
  			        "columns": {
  			          "foo": { "type": "real" }
				}
			      },
  			      "TestTable": {
  			        "columns": {
  			          "ExtraCol": { "type": "real" } ,
  			          "aString": { "type": "string" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: false,
		},
		{
			name: "Less columns should fail",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "aString": { "type": "string" },
  			          "aSet": { "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: true,
		},
		{
			name: "Wrong simple type should fail",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "aString": { "type": "integer" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: true,
		},
		{
			name: "Wrong set type should fail",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "aString": { "type": "string" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "integer",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "string"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: true,
		},
		{
			name: "Wrong map type should fail",
			schema: []byte(`{
			    "name": "TestDB",
  			    "tables": {
  			      "TestTable": {
  			        "columns": {
  			          "aString": { "type": "string" },
  			          "aInt": { "type": "integer" },
  			          "aFloat": { "type": "real" } ,
  			          "aSet": { "type": {
  			              "key": "integer",
  			              "max": "unlimited",
  			              "min": 0
  			            } },
  			          "aMap": {
  			            "type": {
  			              "key": "string",
  			              "max": "unlimited",
  			              "min": 0,
  			              "value": "boolean"
  			            }
  			          }
  			        }
  			      }
  			    }
			}`),
			err: true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("TestValidate %s", tt.name), func(t *testing.T) {
			var schema ovsdb.DatabaseSchema
			err := json.Unmarshal(tt.schema, &schema)
			assert.Nil(t, err)
			errors := model.validate(schema)
			if tt.err {
				assert.Greater(t, len(errors), 0)
			} else {
				assert.Len(t, errors, 0)
			}
		})
	}

}

type modelC struct {
	modelB
	NoClone string
}

func (a *modelC) CloneModel() Model {
	return &modelC{
		modelB: a.modelB,
	}
}

func (a *modelC) CloneModelInto(b Model) {
	c := b.(*modelC)
	c.modelB = a.modelB
}

func (a *modelC) EqualsModel(b Model) bool {
	c := b.(*modelC)
	return reflect.DeepEqual(a.modelB, c.modelB)
}

func TestCloneViaMarshalling(t *testing.T) {
	a := &modelB{UID: "foo", Foo: "bar", Bar: "baz"}
	b := Clone(a).(*modelB)
	assert.Equal(t, a, b)
	a.UID = "baz"
	assert.NotEqual(t, a, b)
	b.UID = "quux"
	assert.NotEqual(t, a, b)
}

func TestCloneIntoViaMarshalling(t *testing.T) {
	a := &modelB{UID: "foo", Foo: "bar", Bar: "baz"}
	b := &modelB{}
	CloneInto(a, b)
	assert.Equal(t, a, b)
	a.UID = "baz"
	assert.NotEqual(t, a, b)
	b.UID = "quux"
	assert.NotEqual(t, a, b)
}

func TestCloneViaCloneable(t *testing.T) {
	a := &modelC{modelB: modelB{UID: "foo", Foo: "bar", Bar: "baz"}, NoClone: "noClone"}
	func(a interface{}) {
		_, ok := a.(CloneableModel)
		assert.True(t, ok, "is not cloneable")
	}(a)
	// test that Clone() uses the cloneable interface, in which
	// case modelC.NoClone won't be copied
	b := Clone(a).(*modelC)
	assert.NotEqual(t, a, b)
	b.NoClone = a.NoClone
	assert.Equal(t, a, b)
	a.UID = "baz"
	assert.NotEqual(t, a, b)
	b.UID = "quux"
	assert.NotEqual(t, a, b)
}

func TestCloneIntoViaCloneable(t *testing.T) {
	a := &modelC{modelB: modelB{UID: "foo", Foo: "bar", Bar: "baz"}, NoClone: "noClone"}
	func(a interface{}) {
		_, ok := a.(CloneableModel)
		assert.True(t, ok, "is not cloneable")
	}(a)
	// test that CloneInto() uses the cloneable interface, in which
	// case modelC.NoClone won't be copied
	b := &modelC{}
	CloneInto(a, b)
	assert.NotEqual(t, a, b)
	b.NoClone = a.NoClone
	assert.Equal(t, a, b)
	a.UID = "baz"
	assert.NotEqual(t, a, b)
	b.UID = "quux"
	assert.NotEqual(t, a, b)
}

func TestEqualViaDeepEqual(t *testing.T) {
	a := &modelB{UID: "foo", Foo: "bar", Bar: "baz"}
	b := &modelB{UID: "foo", Foo: "bar", Bar: "baz"}
	assert.True(t, Equal(a, b))
	a.UID = "baz"
	assert.False(t, Equal(a, b))
}

func TestEqualViaComparable(t *testing.T) {
	a := &modelC{modelB: modelB{UID: "foo", Foo: "bar", Bar: "baz"}, NoClone: "noClone"}
	func(a interface{}) {
		_, ok := a.(ComparableModel)
		assert.True(t, ok, "is not comparable")
	}(a)
	b := a.CloneModel().(*modelC)
	// test that Equal() uses the comparable interface, in which
	// case the difference on modelC.NoClone won't be noticed
	assert.True(t, Equal(a, b))
	a.UID = "baz"
	assert.False(t, Equal(a, b))
}
