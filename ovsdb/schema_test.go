package ovsdb

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/ovn-org/libovsdb/internal/json"

	"github.com/stretchr/testify/assert"
)

func TestSchema(t *testing.T) {
	type schemaTest struct {
		name           string
		schema         []byte
		expectedErr    bool
		expectedSchema DatabaseSchema
	}
	zero := 0
	one := 1
	two := 2
	boolFalse := false
	schemaTestSuite := []schemaTest{
		{
			name: "Simple AtomicType columns",
			schema: []byte(`
		 {"name": "AtomicDB",
		  "version": "0.0.0",
		  "tables": {
		    "atomicTable": {
		      "columns": {
		        "str": {
			  "type": "string"
			},
		        "int": {
			  "type": "integer"
			},
		        "float": {
			  "type": "real"
			},
		        "uuid": {
			  "type": "uuid",
			  "mutable": false
			}
		      }
		    }
		  }
	         }`),
			expectedErr: false,
			expectedSchema: DatabaseSchema{
				Name:    "AtomicDB",
				Version: "0.0.0",
				Tables: map[string]TableSchema{
					"atomicTable": {
						Columns: map[string]*ColumnSchema{
							"str": {
								Type:    TypeString,
								TypeObj: &ColumnType{Key: &BaseType{Type: TypeString}},
							},
							"int": {
								Type:    TypeInteger,
								TypeObj: &ColumnType{Key: &BaseType{Type: TypeInteger}},
							},
							"float": {
								Type:    TypeReal,
								TypeObj: &ColumnType{Key: &BaseType{Type: TypeReal}},
							},
							"uuid": {
								Type:    TypeUUID,
								TypeObj: &ColumnType{Key: &BaseType{Type: TypeUUID}},
								mutable: &boolFalse,
							},
						},
					},
				},
			},
		},
		{
			name: "Sets",
			schema: []byte(`
		 {"name": "SetsDB",
		  "version": "0.0.0",
		  "tables": {
		    "setTable": {
		      "columns": {
		        "single": {
			  "type": {
			    "key": {"type":"string"},
			    "max": 1,
			    "min": 1
			  }
			},
		        "oneElem": {
			  "type": {
			    "key": {"type":"uuid"},
			    "max": 1,
			    "min": 0
			  }
			},
		        "multipleElem": {
			  "type": {
			    "key": {"type":"real"},
			    "max": 2,
			    "min": 0
			  }
			},
		        "unlimitedElem": {
			  "type": {
			    "key": {"type":"integer"},
			    "max": "unlimited",
			    "min": 0
			  }
			},
		        "enumSet": {
			  "type": {
			    "key": {
			      "type": "string",
			      "enum": ["set", ["one", "two"]]
			     },
			    "max": "unlimited",
			    "min": 0
			  }
			}
		      }
		    }
		  }
	         }`),
			expectedErr: false,
			expectedSchema: DatabaseSchema{
				Name:    "SetsDB",
				Version: "0.0.0",
				Tables: map[string]TableSchema{
					"setTable": {
						Columns: map[string]*ColumnSchema{
							"single": {
								Type: TypeString,
								TypeObj: &ColumnType{
									Key: &BaseType{Type: TypeString},
									min: &one,
									max: &one,
								},
							},
							"oneElem": {
								Type: TypeSet,
								TypeObj: &ColumnType{
									Key: &BaseType{Type: "uuid"},
									max: &one,
									min: &zero,
								},
							},
							"multipleElem": {
								Type: TypeSet,
								TypeObj: &ColumnType{
									Key: &BaseType{Type: "real"},
									max: &two,
									min: &zero,
								},
							},
							"unlimitedElem": {
								Type: TypeSet,
								TypeObj: &ColumnType{
									Key: &BaseType{Type: "integer"},
									max: &Unlimited,
									min: &zero,
								},
							},
							"enumSet": {
								Type: TypeSet,
								TypeObj: &ColumnType{
									Key: &BaseType{
										Type: "string",
										Enum: []interface{}{"one", "two"},
									},
									max: &Unlimited,
									min: &zero,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Maps",
			schema: []byte(`
		 {"name": "MapsDB",
		  "version": "0.0.0",
		  "tables": {
		    "mapTable": {
		      "columns": {
		        "str_str": {
			  "type": {
			    "key": {"type":"string"},
			    "value": {"type":"string"}
			  }
			},
		        "str_int": {
			  "type": {
			    "key": {"type":"string"},
			    "value": {"type":"integer"}
			  }
			},
		        "int_real": {
			  "type": {
			    "key": {"type":"integer"},
			    "value": {"type":"real"}
			  }
			},
		        "str_uuid": {
			  "type": {
			    "key": {"type":"string"},
			    "value": {"type":"uuid"}
			  }
			},
		        "str_enum": {
			  "type": {
			    "key": {"type":"string"},
			    "value": {
			      "type": "string",
			      "enum": ["set", ["one", "two"]]
			     }
			  }
			}
		      }
		    }
		  }
	         }`),
			expectedErr: false,
			expectedSchema: DatabaseSchema{
				Name:    "MapsDB",
				Version: "0.0.0",
				Tables: map[string]TableSchema{
					"mapTable": {
						Columns: map[string]*ColumnSchema{
							"str_str": {
								Type: TypeMap,
								TypeObj: &ColumnType{
									Key:   &BaseType{Type: "string"},
									Value: &BaseType{Type: "string"},
								},
							},
							"str_int": {
								Type: TypeMap,
								TypeObj: &ColumnType{
									Key:   &BaseType{Type: "string"},
									Value: &BaseType{Type: "integer"},
								},
							},
							"int_real": {
								Type: TypeMap,
								TypeObj: &ColumnType{
									Key:   &BaseType{Type: "integer"},
									Value: &BaseType{Type: "real"},
								},
							},
							"str_uuid": {
								Type: TypeMap,
								TypeObj: &ColumnType{
									Key:   &BaseType{Type: "string"},
									Value: &BaseType{Type: "uuid"},
								},
							},
							"str_enum": {
								Type: TypeMap,
								TypeObj: &ColumnType{
									Key: &BaseType{
										Type: "string",
									},
									Value: &BaseType{
										Type: "string",
										Enum: []interface{}{"one", "two"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Invalid type",
			schema: []byte(`
		 {"name": "ErrorDB",
		  "version": "0.0.0",
		  "tables": {
		    "errorsTable": {
		      "columns": {
		        "wrongType": {
			  "type": {
			    "key": "unknown"
			  }
			}
		      }
		    }
		  }
	         }`),
			expectedErr: true,
		},
		{
			name:        "Invalid json",
			schema:      []byte(`invalid json`),
			expectedErr: true,
		},
	}

	for _, test := range schemaTestSuite {
		t.Run(fmt.Sprintf("Schema Test %s", test.name), func(t *testing.T) {
			var schema DatabaseSchema
			err := json.Unmarshal(test.schema, &schema)

			if (err != nil) != test.expectedErr {
				t.Fatalf("Expected error to be %t, but got error: %s", test.expectedErr, err.Error())
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(test.expectedSchema, schema) {
				t.Errorf("expected schema to be %+#v, but got: %+#v", test.expectedSchema, schema)
				// Struct Introspection for debugging purposes
				for tname, table := range schema.Tables {
					for n, c := range table.Columns {
						ec := test.expectedSchema.Tables[tname].Columns[n]
						t.Logf("column name %s", n)
						t.Logf("  Expected: %+#v", ec)
						t.Logf("       Got: %+#v", c)

						if ec.TypeObj != nil {
							t.Logf("  Expected.Obj: %+#v", ec.TypeObj)
							if ec.TypeObj.Key != nil {
								t.Logf("    Expected.Obj.Key: %+#v", ec.TypeObj.Key)
							}
							if ec.TypeObj.Value != nil {
								t.Logf("    Expected.Obj.Value: %+#v", ec.TypeObj.Value)
							}
						}
						if c.TypeObj != nil {
							t.Logf("       Got.Obj: %+#v", c.TypeObj)
							if c.TypeObj.Key != nil {
								t.Logf("         Got.Obj.Key: %+#v", c.TypeObj.Key)
							}
							if c.TypeObj.Value != nil {
								t.Logf("         Got.Obj.Value: %+#v", c.TypeObj.Value)
							}
						}

					}
				}
			}
			b, err := json.Marshal(schema)
			assert.Nil(t, err)
			assert.JSONEq(t, string(test.schema), string(b))
		})
	}
}

func TestTable(t *testing.T) {
	schemaJ := []byte(`{"name": "TestSchema",
		  "version": "0.0.0",
		  "tables": {
		    "test": {
		      "columns": {
		        "foo": {
			  "type": {
			    "key": "string",
			    "value": "string"
			  }
			},
		        "bar": {
			  "type": "string"
			}
		      }
		    }
		}
	    }`)

	var schema DatabaseSchema
	err := json.Unmarshal(schemaJ, &schema)
	assert.Nil(t, err)

	t.Run("GetTable_exists", func(t *testing.T) {
		table := schema.Table("test")
		assert.NotNil(t, table)
	})
	t.Run("GetTable_not_exists", func(t *testing.T) {
		table := schema.Table("notexists")
		assert.Nil(t, table)
	})
	t.Run("GetColumn_exists", func(t *testing.T) {
		table := schema.Table("test")
		assert.NotNil(t, table)
		column := table.Column("foo")
		assert.NotNil(t, column)
	})
	t.Run("GetColumn_not_exists", func(t *testing.T) {
		table := schema.Table("test")
		assert.NotNil(t, table)
		column := table.Column("notexists")
		assert.Nil(t, column)
	})
	t.Run("GetColumn_uuid", func(t *testing.T) {
		table := schema.Table("test")
		assert.NotNil(t, table)
		column := table.Column("_uuid")
		assert.NotNil(t, column)
	})
}

func TestBaseTypeMarshalUnmarshalJSON(t *testing.T) {
	datapath := "Datapath"
	zero := 0
	max := 4294967295
	strong := "strong"
	tests := []struct {
		name         string
		in           []byte
		expected     BaseType
		expectedJSON []byte
		wantErr      bool
	}{
		{
			"string",
			[]byte(`"string"`),
			BaseType{Type: TypeString},
			[]byte(`{"type":"string"}`),
			false,
		},
		{
			"integer",
			[]byte(`"integer"`),
			BaseType{Type: TypeInteger},
			[]byte(`{"type":"integer"}`),
			false,
		},
		{
			"boolean",
			[]byte(`"boolean"`),
			BaseType{Type: TypeBoolean},
			[]byte(`{"type":"boolean"}`),
			false,
		},
		{
			"real",
			[]byte(`"real"`),
			BaseType{Type: TypeReal},
			[]byte(`{"type":"real"}`),
			false,
		},
		{
			"uuid",
			[]byte(`"uuid"`),
			BaseType{Type: TypeUUID},
			[]byte(`{"type":"uuid"}`),
			false,
		},
		{
			"uuid",
			[]byte(`{"type": "uuid", "refTable": "Datapath", "refType": "strong"}`),
			BaseType{Type: TypeUUID, refTable: &datapath, refType: &strong},
			[]byte(`{"type": "uuid", "refTable": "Datapath", "refType": "strong"}`),
			false,
		},
		{
			"enum",
			[]byte(`{"type": "string","enum": ["set", ["OpenFlow10","OpenFlow11","OpenFlow12","OpenFlow13","OpenFlow14","OpenFlow15"]]}`),
			BaseType{Type: TypeString, Enum: []interface{}{"OpenFlow10", "OpenFlow11", "OpenFlow12", "OpenFlow13", "OpenFlow14", "OpenFlow15"}},
			[]byte(`{"type": "string","enum": ["set", ["OpenFlow10","OpenFlow11","OpenFlow12","OpenFlow13","OpenFlow14","OpenFlow15"]]}`),
			false,
		},
		{
			"int with min and max",
			[]byte(`{"type":"integer","minInteger":0,"maxInteger": 4294967295}`),
			BaseType{Type: TypeInteger, minInteger: &zero, maxInteger: &max},
			[]byte(`{"type":"integer","minInteger":0,"maxInteger": 4294967295}`),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b BaseType
			err := b.UnmarshalJSON(tt.in)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, b)
			raw, err := b.MarshalJSON()
			assert.Nil(t, err)
			assert.JSONEq(t, string(tt.expectedJSON), string(raw))
		})
	}
}

func TestColumnTypeMarshalUnmarshalJSON(t *testing.T) {
	one := 1
	tests := []struct {
		name         string
		in           []byte
		expected     ColumnType
		expectedJSON []byte
	}{
		{
			"string",
			[]byte(`"string"`),
			ColumnType{
				Key: &BaseType{Type: "string"},
			},
			[]byte(`"string"`),
		},
		{
			"map string string",
			[]byte(`{"value":"string","key":{"type":"string"},"min":1,"max":1}`),
			ColumnType{
				Key:   &BaseType{Type: "string"},
				Value: &BaseType{Type: "string"},
				min:   &one,
				max:   &one,
			},
			[]byte(`{"key":{"type":"string"},"value":{"type":"string"},"min":1,"max":1}`),
		},
		{
			"map str int",
			[]byte(`{"key":"string","value":"integer","min":1,"max":1}`),
			ColumnType{
				Key:   &BaseType{Type: "string"},
				Value: &BaseType{Type: "integer"},
				min:   &one,
				max:   &one,
			},
			[]byte(`{"key":{"type": "string"},"value":{"type":"integer"},"min":1,"max":1}`),
		},
		{
			"map int real",
			[]byte(`{"key":{"type":"integer"},"value":{"type":"real"},"min":1,"max":"unlimited"}`),
			ColumnType{
				Key:   &BaseType{Type: "integer"},
				Value: &BaseType{Type: "real"},
				min:   &one,
				max:   &Unlimited,
			},
			[]byte(`{"key":{"type":"integer"},"value":{"type":"real"},"min":1,"max":"unlimited"}`),
		},
		{
			"map str uuid",
			[]byte(`{"key":{"type":"string"},"value":{"type":"uuid"},"min":1,"max":"unlimited"}`),
			ColumnType{
				Key:   &BaseType{Type: "string"},
				Value: &BaseType{Type: "uuid"},
				min:   &one,
				max:   &Unlimited,
			},
			[]byte(`{"key":{"type":"string"},"value":{"type":"uuid"},"min":1,"max":"unlimited"}`),
		},
		{
			"string enum",
			[]byte(`{"key":{"type":"string"},"value":{"type":"string","enum":["set", ["one","two"]]},"min":1,"max":1}`),
			ColumnType{
				Key: &BaseType{
					Type: "string",
				},
				Value: &BaseType{
					Type: "string",
					Enum: []interface{}{"one", "two"},
				},
				min: &one,
				max: &one,
			},
			[]byte(`{"key":{"type":"string"},"value":{"type":"string","enum":["set",["one","two"]]},"min":1,"max":1}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c ColumnType
			err := c.UnmarshalJSON(tt.in)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, c)
			raw, err := c.MarshalJSON()
			assert.Nil(t, err)
			assert.JSONEq(t, string(tt.expectedJSON), string(raw))
		})
	}
}

func TestColumnSchemaMutable(t *testing.T) {
	boolTrue := true
	boolFalse := false
	m1 := ColumnSchema{mutable: nil}
	m2 := ColumnSchema{mutable: &boolTrue}
	m3 := ColumnSchema{mutable: &boolFalse}
	assert.True(t, m1.Mutable())
	assert.True(t, m2.Mutable())
	assert.False(t, m3.Mutable())
}

func TestColumnSchemaEphemeral(t *testing.T) {
	boolTrue := true
	boolFalse := false
	e1 := ColumnSchema{ephemeral: nil}
	e2 := ColumnSchema{ephemeral: &boolTrue}
	e3 := ColumnSchema{ephemeral: &boolFalse}
	assert.False(t, e1.Ephemeral())
	assert.True(t, e2.Ephemeral())
	assert.False(t, e3.Ephemeral())
}

func TestColumnSchemaMarshalUnmarshalJSON(t *testing.T) {
	datapath := "Datapath"
	unlimited := -1
	zero := 0
	one := 1
	tests := []struct {
		name         string
		in           []byte
		expected     ColumnSchema
		expectedJSON []byte
	}{
		{
			"simple string",
			[]byte(`{"type": "string"}`),
			ColumnSchema{
				Type:    TypeString,
				TypeObj: &ColumnType{Key: &BaseType{Type: TypeString}},
			},
			[]byte(`{"type": "string"}`),
		},
		{
			"map",
			[]byte(`{"type":{"key": {"type": "string"},"value": {"type": "uuid","refTable": "Datapath"},"min": 0, "max": "unlimited"}}`),
			ColumnSchema{
				Type: TypeMap,
				TypeObj: &ColumnType{
					Key:   &BaseType{Type: TypeString},
					Value: &BaseType{Type: TypeUUID, refTable: &datapath},
					min:   &zero,
					max:   &unlimited,
				},
			},
			[]byte(`{"type":{"key": {"type": "string"},"value": {"type": "uuid","refTable": "Datapath"},"min": 0, "max": "unlimited"}}`),
		},
		{
			"set",
			[]byte(`{"type": {"key": {"type": "uuid","refTable": "Datapath"},"min": 0, "max": "unlimited"}}`),
			ColumnSchema{
				Type: TypeSet,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeUUID, refTable: &datapath},
					min: &zero,
					max: &unlimited,
				}},
			[]byte(`{"type": {"key": {"type": "uuid","refTable": "Datapath"},"min": 0, "max": "unlimited"}}`),
		},
		{
			"enum",
			[]byte(`{"type": {"key": {"type": "string","enum": ["set", ["one", "two"]]},"max": 1,"min": 1}}`),
			ColumnSchema{
				Type: TypeEnum,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeString, Enum: []interface{}{"one", "two"}},
					max: &one,
					min: &one,
				},
			},
			[]byte(`{"type": {"key": {"type": "string","enum": ["set", ["one", "two"]]},"max": 1,"min": 1}}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c ColumnSchema
			err := c.UnmarshalJSON(tt.in)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, c)
			assert.True(t, c.Mutable())
			raw, err := c.MarshalJSON()
			assert.Nil(t, err)
			assert.JSONEq(t, string(tt.expectedJSON), string(raw))
		})
	}
}

func TestBaseTypeSimpleAtomic(t *testing.T) {
	b := BaseType{Type: TypeString}
	assert.True(t, b.simpleAtomic())
	max := 1024
	b1 := BaseType{Type: TypeInteger, maxInteger: &max}
	assert.False(t, b1.simpleAtomic())
}

func TestBaseTypeMinReal(t *testing.T) {
	value := float64(1024)
	tests := []struct {
		name    string
		bt      *BaseType
		want    float64
		wantErr bool
	}{
		{
			"not a real",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeReal},
			math.SmallestNonzeroFloat64,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeReal, minReal: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MinReal()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeMaxReal(t *testing.T) {
	value := float64(1024)
	tests := []struct {
		name    string
		bt      *BaseType
		want    float64
		wantErr bool
	}{
		{
			"not a real",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeReal},
			math.MaxFloat64,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeReal, maxReal: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MaxReal()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeMinInteger(t *testing.T) {
	value := 1024
	tests := []struct {
		name    string
		bt      *BaseType
		want    int
		wantErr bool
	}{
		{
			"not an int",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeInteger},
			int(math.Pow(-2, 63)),
			false,
		},
		{
			"set",
			&BaseType{Type: TypeInteger, minInteger: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MinInteger()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeMaxInteger(t *testing.T) {
	value := 1024
	tests := []struct {
		name    string
		bt      *BaseType
		want    int
		wantErr bool
	}{
		{
			"not an int",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeInteger},
			int(math.Pow(2, 63)) - 1,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeInteger, maxInteger: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MaxInteger()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeMinLength(t *testing.T) {
	value := 12
	tests := []struct {
		name    string
		bt      *BaseType
		want    int
		wantErr bool
	}{
		{
			"not a string",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeString},
			0,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeString, minLength: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MinLength()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeMaxLength(t *testing.T) {
	value := 1024
	tests := []struct {
		name    string
		bt      *BaseType
		want    int
		wantErr bool
	}{
		{
			"not a string",
			&BaseType{Type: TypeUUID},
			0,
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeString},
			int(math.Pow(2, 63)) - 1,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeString, maxLength: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.MaxLength()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeRefTable(t *testing.T) {
	value := "Bridge"
	tests := []struct {
		name    string
		bt      *BaseType
		want    string
		wantErr bool
	}{
		{
			"not a uuid",
			&BaseType{Type: TypeString},
			"",
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeUUID},
			"",
			false,
		},
		{
			"set",
			&BaseType{Type: TypeUUID, refTable: &value},
			value,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.RefTable()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBaseTypeRefType(t *testing.T) {
	value := "weak"
	tests := []struct {
		name    string
		bt      *BaseType
		want    RefType
		wantErr bool
	}{
		{
			"not a uuid",
			&BaseType{Type: TypeString},
			"",
			true,
		},
		{
			"nil",
			&BaseType{Type: TypeUUID},
			Strong,
			false,
		},
		{
			"set",
			&BaseType{Type: TypeUUID, refType: &value},
			Weak,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bt.RefType()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColumnSchema_String(t *testing.T) {
	datapath := "Connection"
	unlimited := -1
	zero := 0
	strong := "strong"
	weak := "weak"
	type fields struct {
		Type      ExtendedType
		TypeObj   *ColumnType
		ephemeral *bool
		mutable   *bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			"str",
			fields{
				Type: TypeString,
			},
			"string [M]",
		},
		{
			"str map",
			fields{
				Type: TypeMap,
				TypeObj: &ColumnType{
					Key: &BaseType{
						Type: TypeString,
					},
					Value: &BaseType{
						Type: TypeString,
					},
				},
			},
			"[string]string [M]",
		},
		{
			"ref",
			fields{
				Type: TypeSet,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeUUID, refTable: &datapath},
					min: &zero,
					max: &unlimited,
				},
			},
			"[] [Connection (strong)] (min: 0, max: -1) [M]",
		},
		{
			"ref 1",
			fields{
				Type: TypeSet,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeUUID, refTable: &datapath, refType: &strong},
					min: &zero,
					max: &unlimited,
				},
			},
			"[] [Connection (strong)] (min: 0, max: -1) [M]",
		},
		{
			"ref 2",
			fields{
				Type: TypeSet,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeUUID, refTable: &datapath, refType: &weak},
					min: &zero,
					max: &unlimited,
				},
			},
			"[] [Connection (weak)] (min: 0, max: -1) [M]",
		},
		{
			"enum",
			fields{
				Type: TypeEnum,
				TypeObj: &ColumnType{
					Key: &BaseType{Type: TypeString, Enum: []interface{}{"permit", "deny"}},
					max: &unlimited,
					min: &zero,
				},
			},
			"enum (type: string): [permit deny] [M]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := &ColumnSchema{
				Type:      tt.fields.Type,
				TypeObj:   tt.fields.TypeObj,
				ephemeral: tt.fields.ephemeral,
				mutable:   tt.fields.mutable,
			}
			if got := column.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
