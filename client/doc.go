/*
Package client connects to, monitors and interacts with OVSDB servers (RFC7047).

The package works with Models: pointers to structs whose fields are tagged with the 'ovsdb'
tag naming the column they map to. Example:

	type MyLogicalSwitch struct {
		UUID   string            `ovsdb:"_uuid"` // _uuid tag is mandatory
		Name   string            `ovsdb:"name"`
		Ports  []string          `ovsdb:"ports"`
		Config map[string]string `ovsdb:"other_config"`
	}

Based on these Models a ClientDBModel (see the model package) is built to represent
the entire OVSDB, or the part of it we're interested in:

	clientDBModel, err := model.NewClientDBModel("OVN_Northbound",
	    map[string]model.Model{
		"Logical_Switch": &MyLogicalSwitch{},
	})

Using it, the client package is able to properly encode and decode OVSDB messages
and store them in Model instances.
A Client is created by specifying the database model and any number of Options
(such as WithEndpoint). It then needs to be connected, and it must monitor the tables
whose content should be available through the cache-based API (List and Get):

	ovs, err := client.NewOVSDBClient(clientDBModel, client.WithEndpoint("tcp:127.0.0.1:6641"))
	err = ovs.Connect(context.Background())
	_, err = ovs.MonitorAll(context.Background())

Monitor can be used instead of MonitorAll to monitor only some tables, columns or rows.

# Main API

Once a Client is connected, a number of CRUD-like functions can be used to interact with the database:
List(), Get(), Create(), Update(), Mutate(), Delete().

The specific database table that the operation targets is automatically determined based on the type
of the parameter.

In terms of return values, some of these functions like Create(), Update(), Mutate() and Delete(),
interact with the database so they return a list of ovsdb.Operation objects that can be grouped together
and passed to Transact().

Others, such as List() and Get(), interact with the client's internal cache and are able to
return Model instances (or a list thereof) directly.

# Conditions

Some API functions (Create(), Get() and List()) can be run directly. Others require using
a ConditionalAPI. The ConditionalAPI injects RFC7047 Conditions into ovsdb Operations as well as
uses the Conditions to search the internal cache.

The ConditionalAPI is created using the Where(), WhereAny(), WhereAll(), WhereCache() and
WhereCacheByUUIDs() functions.

Where() accepts one or more Models and uses the values of the fields that correspond to
schema indexes to build the conditions. For example, the following deletes the Logical Switch
whose index (here, the UUID) is "myUUID":

	ls := &MyLogicalSwitch{UUID: "myUUID"}
	ops, err := ovs.Where(ls).Delete()

If more than one Model is provided, the client will generate multiple operations, each
matching one Model.

WhereAny() and WhereAll() accept a Model, which determines the table, and a number of
model.Condition instances. Conditions must refer to fields of the provided Model (via pointer to
fields). WhereAny() matches the rows that satisfy any of the conditions (OR semantics), while
WhereAll() matches the rows that satisfy all of them simultaneously (AND semantics). Example:

	ls := &MyLogicalSwitch{}
	ops, err := ovs.WhereAll(ls, model.Condition{
		Field:    &ls.Name,
		Function: ovsdb.ConditionEqual,
		Value:    "foo",
	}, model.Condition{
		Field:    &ls.Ports,
		Function: ovsdb.ConditionIncludes,
		Value:    []string{"portUUID"},
	}).Delete()

Where(), WhereAny() and WhereAll() evaluate the provided index values or explicit conditions against the cache
and generate conditions based on the UUIDs of matching models. If no matches are found in the cache, the
generated conditions will be based on the index or condition fields themselves.

A more flexible mechanism to search the cache is available: WhereCache()

WhereCache() accepts a function that takes a Model as argument and returns a boolean.
It is used to search the cache so commonly used with List() function. For example:

	lsList := []*MyLogicalSwitch{}
	err := ovs.WhereCache(
	    func(ls *MyLogicalSwitch) bool {
	    	return strings.HasPrefix(ls.Name, "ext_")
	}).List(context.Background(), &lsList)

Server side operations can be executed using WhereCache() conditions but it's not recommended. For each matching
cache element, an operation will be created matching on the "_uuid" column. The number of operations can be
quite large depending on the cache size and the provided function. Most likely there is a way to express the
same condition using Where(), WhereAny() or WhereAll() which will be more efficient.

WhereCacheByUUIDs() applies a WhereCache()-style predicate only to the supplied cached UUIDs. Use it when
the candidate UUIDs are already known to avoid scanning unrelated rows. Missing UUIDs are ignored, duplicate
UUIDs are evaluated once, and an empty UUID list matches no rows.

	lsList := []*MyLogicalSwitch{}
	err := ovs.WhereCacheByUUIDs(
	func(ls *MyLogicalSwitch) bool {
		return strings.HasPrefix(ls.Name, "ext_")
	}, candidateUUIDs...).List(context.Background(), &lsList)

# Get

Get() is a simple operation capable of retrieving one Model from the cache based on some of its
schema indexes. E.g:

	ls := &MyLogicalSwitch{UUID: "myUUID"}
	err := ovs.Get(context.Background(), ls)
	fmt.Printf("Name of the switch is: %s", ls.Name)

# List

List() searches the cache and populates a slice of Models. It can be used directly or using WhereCache()

	lsList := []*MyLogicalSwitch{}
	err := ovs.List(context.Background(), &lsList) // List all elements

	err = ovs.WhereCache(
	    func(ls *MyLogicalSwitch) bool {
	    	return strings.HasPrefix(ls.Name, "ext_")
	}).List(context.Background(), &lsList)

# Create

Create returns a list of operations to create the models provided. E.g:

	ops, err := ovs.Create(&MyLogicalSwitch{Name: "foo"}, &MyLogicalSwitch{Name: "bar"})

# Update

Update returns a list of operations to update the matching rows to match the values of the provided model.
By default all the non-default values of the model are updated; fields to update can be selected
by passing pointers to them. E.g:

	ls := &MyLogicalSwitch{Config: map[string]string{"foo": "bar"}}
	ops, err := ovs.Where(...).Update(ls, &ls.Config)

# Mutate

Mutate returns a list of operations needed to mutate the matching rows as described by the list of
model.Mutation objects. E.g:

	ls := &MyLogicalSwitch{}
	ops, err := ovs.Where(...).Mutate(ls, model.Mutation{
		Field:   &ls.Config,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   map[string]string{"foo": "bar"},
	})

# Delete

Delete returns a list of operations needed to delete the matching rows. E.g:

	ops, err := ovs.Where(...).Delete()

# Transact

The operations returned by the functions above are sent to the server with Transact(). All the
operations passed to a single call are executed as one atomic transaction. The returned
ovsdb.OperationResult values can be checked with ovsdb.CheckOperationResults:

	results, err := ovs.Transact(context.Background(), ops...)
	if err == nil {
		_, err = ovsdb.CheckOperationResults(results, ops)
	}
*/
package client
