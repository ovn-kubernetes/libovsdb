/*
Package model defines the types used to describe how OVSDB data maps to Go.

A Model is a pointer to a struct whose fields are tagged with the 'ovsdb' tag naming the
column of the OVSDB table they map to. A field tagged "_uuid" is mandatory:

	type MyLogicalRouter struct {
		UUID        string            `ovsdb:"_uuid"`
		Name        string            `ovsdb:"name"`
		ExternalIDs map[string]string `ovsdb:"external_ids"`
	}

Models are usually generated from an OVSDB schema using the modelgen tool.

# ClientDBModel

A ClientDBModel, created with NewClientDBModel, associates a database name with the Model used for
each of its tables. It can optionally carry a set of indexes (see ClientIndex) that are
maintained by the client in addition to those defined in the schema. It is the main input
needed to create a client (see the client package).

# DatabaseModel

A DatabaseModel is the combination of a ClientDBModel and the schema (ovsdb.DatabaseSchema) that
the server reports. It is created with NewDatabaseModel, which validates that the Models are
compatible with the schema, and is used to look up the Model type associated with a table.

# Conditions and Mutations

Condition and Mutation describe, in terms of pointers to Model fields, the conditions and the
mutations that can be passed to the client API (for example Client.WhereAll or ConditionalAPI.Mutate).
*/
package model
