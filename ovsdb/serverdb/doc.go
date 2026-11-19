/*
Package serverdb provides the Models for the _Server database that every OVSDB server
exports. It reports the status of the server process itself, such as the databases it
serves, whether it is a clustered database and which member is the leader.

The Models are generated from the _Server schema using modelgen; use FullDatabaseModel to
create a model.ClientDBModel that can be passed to client.NewOVSDBClient.
*/
package serverdb
