/*
Package mapper translates between OVSDB wire representations and native Go values.

A Mapper is created from an ovsdb.DatabaseSchema with NewMapper. Together with an Info, which
wraps a Model with the Metadata (table name, table schema and the field/column mapping) needed
to interpret it, it can:

  - populate a Model from an ovsdb.Row (GetRowData, GetRowDataWithUUID)
  - build an ovsdb.Row from the fields of a Model (NewRow)
  - build ovsdb.Condition and ovsdb.Mutation values from Model fields (NewEqualityCondition,
    NewCondition, NewMutation)
  - compare two Models field by field (EqualFields)

The mapping between struct fields and columns is expressed with the 'ovsdb' field tag.

Most users do not need this package directly: it is used by the client, cache and server
packages. See the client package for the user-facing API.
*/
package mapper
