/*
Package ovsdb implements the data types and wire format of the OVSDB protocol (RFC7047).

It contains:

  - the schema types (DatabaseSchema, TableSchema, ColumnSchema) that describe a database
  - the OVSDB atomic and complex types (UUID, OvsSet, OvsMap) and their JSON encoding
  - the types used to build transactions (Operation, Row, Condition, Mutation) and to report
    and check their results (OperationResult, CheckOperationResults)
  - the RPC request and reply types used for monitoring (MonitorRequest, TableUpdates,
    TableUpdates2)

Most users should not need to build these types by hand. The client package builds Operations
from Models (see Client.Create, ConditionalAPI.Update and so on) and decodes updates into
Models. OvsSet and OvsMap are only needed when handling raw ovsdb.Row data.
*/
package ovsdb
