# libovsdb

[![libovsdb-ci](https://github.com/ovn-kubernetes/libovsdb/actions/workflows/ci.yml/badge.svg)](https://github.com/ovn-kubernetes/libovsdb/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/ovn-kubernetes/libovsdb/badge.svg?branch=main)](https://coveralls.io/github/ovn-kubernetes/libovsdb?branch=main) [![Go Report Card](https://goreportcard.com/badge/github.com/ovn-kubernetes/libovsdb)](https://goreportcard.com/report/github.com/ovn-kubernetes/libovsdb)

An OVSDB Library written in Go

## What is OVSDB?

OVSDB is the Open vSwitch Database Protocol.
It's defined in [RFC 7047](http://tools.ietf.org/html/rfc7047)
It's used mainly for managing the configuration of Open vSwitch and OVN, but it could also be used to manage your stamp collection. Philatelists Rejoice!

## Quick Overview

The API to interact with OVSDB is based on tagged golang structs. We call it a Model. e.g:

    type MyLogicalSwitch struct {
        UUID   string            `ovsdb:"_uuid"` // _uuid tag is mandatory
        Name   string            `ovsdb:"name"`
        Ports  []string          `ovsdb:"ports"`
        Config map[string]string `ovsdb:"other_config"`
    }

libovsdb is able to translate a Model in to OVSDB format.
To make the API use go idioms, the following mappings occur:

1. OVSDB Set with min 0 and max unlimited = Slice
1. OVSDB Set with min 0 and max 1 = Pointer to scalar type
1. OVSDB Set with min 0 and max N = Array of N
1. OVSDB Enum = Type-aliased Enum Type
1. OVSDB Map = Map
1. OVSDB Scalar Type = Equivalent scalar Go type

A Open vSwitch Database is modeled using a ClientDBModel which is a created by assigning table names to pointers to these structs:

    dbModelReq, _ := model.NewClientDBModel("OVN_Northbound", map[string]model.Model{
                "Logical_Switch": &MyLogicalSwitch{},
    })
    
You can create a client object with options such as the endpoint:

    ovs, _ := client.NewOVSDBClient(*dbModelReq, client.WithEndpoint("tcp:172.18.0.4:6641"))

Finally, the client must be connected before use:

    ovs.Connect(context.Background())
    ovs.MonitorAll(context.Background()) // Only needed if you want to use the built-in cache

Once the client object is created, a generic API can be used to interact with the Database. Some API calls can be performed on the generic API: `List`, `Get`, `Create`, `Select` (for all rows).

Others, have to be called on a `ConditionalAPI` (`Update`, `Delete`, `Mutate`, `Select` with conditions). There are three ways to create a `ConditionalAPI`:

**Where()**: `Where()` can be used to create a `ConditionalAPI` based on the index information that the provided Models contain. Example:

      ls := &LogicalSwitch{UUID: "foo"}
      ls2 := &LogicalSwitch{UUID: "foo2"}
      ops, _ := ovs.Where(ls, ls2).Delete()

It will check the field corresponding to the `_uuid` column as well as all the other schema-defined or client-defined indexes in that order of priority.
The first available index will be used to generate a condition.

**WhereAny()**: `WhereAny()` can be used to create a `ConditionalAPI` using a list of Condition objects. Each condition object specifies a field using a pointer
to a Model's field, a `ovsdb.ConditionFunction` and a value. The type of the value depends on the type of the field being mutated. Example:

      ls := &LogicalSwitch{}
      ops, _ := ovs.WhereAny(ls, client.Condition{
          Field: &ls.Config,
          Function: ovsdb.ConditionIncludes,
          Value: map[string]string{"foo": "bar"},
      }).Delete()

The resulting `ConditionalAPI` will create one operation per condition, so all the rows that match _any_ of the specified conditions will be affected.

**WhereAll()**: `WhereAll()` behaves like `WhereAny()` but with _AND_ semantics. The resulting `ConditionalAPI` will put all the
conditions into a single operation. Therefore the operation will affect the rows that satisfy _all_ the conditions.

**WhereCache()**: `WhereCache()` uses a function callback to filter on the local cache. It's primary use is to perform cache operations such as
`List()`. However, it can also be used to create server-side operations (such as `Delete()`, `Update()` or `Delete()`). If used this way, it will
create an equality condition (using `ovsdb.ConditionEqual`) on the `_uuid` field for every matching row. Example:

    lsList := []LogicalSwitch{}
    ovs.WhereCache(
        func(ls *MyLogicalSwitch) bool {
            return strings.HasPrefix(ls.Name, "ext_")
    }).List(&lsList)

The table is inferred from the type that the function accepts as only argument.

### Client indexes

The client will track schema indexes and use them when appropriate in `Get`, `Where`, and `WhereAll` as explained above.

Additional indexes can be specified for a client instance to track. Just as schema indexes, client indexes are specified in sets per table.
where each set consists of the columns that compose the index. However, unlike schema indexes, client indexes:

- can be used with columns that are maps, where specific map keys can be indexed (see example below).
- can be used with columns that are optional, where no-value columns are indexed as well.

Client indexes are leveraged through `Where`, and `WhereAll`. Since client indexes value uniqueness is not enforced as it happens with schema indexes,
conditions based on them can match multiple rows.

Indexed based operations generally provide better performance than operations based on explicit conditions.

As an example, where you would have:

    // slow predicate run on all the LB table rows...
    ovn.WhereCache(func (lb *LoadBalancer) bool {
        return lb.ExternalIds["myIdKey"] == "myIdValue"
    }).List(ctx, &results)

can now be improved with:

    dbModel, err := nbdb.FullDatabaseModel()
    dbModel.SetIndexes(map[string][]model.ClientIndex{
        "Load_Balancer": {{Columns: []model.ColumnKey{{Column: "external_ids", Key: "myIdKey"}}}},
    })

    // connect ....

    lb := &LoadBalancer{
        ExternalIds: map[string]string{"myIdKey": "myIdValue"},
    }
    // quick indexed result
    ovn.Where(lb).List(ctx, &results)

## Documentation

This package is divided into several sub-packages. Documentation for each sub-package is available at [pkg.go.dev][doc]:

- **client**: ovsdb client and API [![godoc for libovsdb/client][clientbadge]][clientdoc]
- **mapper**: mapping from tagged structs to ovsdb types [![godoc for libovsdb/mapper][mapperbadge]][mapperdoc]
- **model**: model and database model used for mapping [![godoc for libovsdb/model][modelbadge]][modeldoc]
- **ovsdb**: low level OVS types [![godoc for libovsdb/ovsdb][ovsdbbadge]][ovsdbdoc]
- **cache**: model-based cache [![godoc for libovsdb/cache][cachebadge]][cachedoc]
- **modelgen**: common code-generator functions [![godoc for libovsdb/modelgen][genbadge]][gendoc]
- **server**: ovsdb test server [![godoc for libovsdb/server][serverbadge]][serverdoc]
- **database**: database related types, interfaces and implementations [![godoc for libovsdb/database][dbbadge]][dbdoc]
- **updates**: common code to handle model updates [![godoc for libovsdb/updates][updatesbadge]][updatesdoc]

[doc]: https://pkg.go.dev/
[clientbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/client
[mapperbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/mapper
[modelbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/model
[ovsdbbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/ovsdb
[cachebadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/cache
[genbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/modelgen
[serverbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/server
[dbbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/database
[updatesbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/server
[clientdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/client
[mapperdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/mapper
[modeldoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/model
[ovsdbdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/ovsdb
[cachedoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/cache
[gendoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/modelgen
[serverdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/server
[dbdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/database
[updatesdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/updates

## Quick API Examples

List the content of the database:

    var lsList *[]MyLogicalSwitch
    ovs.List(lsList)

    for _, ls := range lsList {
        fmt.Printf("%+v\n", ls)
    }

Search the cache for elements that match a certain predicate:

    var lsList *[]MyLogicalSwitch
    ovs.WhereCache(
        func(ls *MyLogicalSwitch) bool {
            return strings.HasPrefix(ls.Name, "ext_")
    }).List(&lsList)

    for _, ls := range lsList {
        fmt.Printf("%+v\n", ls)
    }

Create a new element

    ops, _ := ovs.Create(&MyLogicalSwitch{
        Name: "foo",
    })

    ovs.Transact(ops...)

Get an element:

    ls := &MyLogicalSwitch{Name: "foo"} // "name" is in the table index list
    ovs.Get(ls)

And update it:

    ls.Config["foo"] = "bar"
    ops, _ := ovs.Where(ls).Update(&ls)
    ovs.Transact(ops...)

Or mutate an it:

    ops, _ := ovs.Where(ls).Mutate(ls, ovs.Mutation {
            Field:   &ls.Config,
            Mutator: ovsdb.MutateOperationInsert,
            Value:   map[string]string{"foo": "bar"},
        })
    ovs.Transact(ops...)

Update, Mutate and Delete operations need a condition to be specified.
Conditions can be created based on a Model's data:

    ls := &LogicalSwitch{UUID:"myUUID"}
    ops, _ := ovs.Where(ls).Delete()
    ovs.Transact(ops...)

They can also be created based on a list of Conditions:

    ops, _ := ovs.Where(ls, client.Condition{
        Field: &ls.Config,
        Function: ovsdb.ConditionIncludes,
        Value: map[string]string{"foo": "bar"},
    }).Delete()

    ovs.Transact(ops...)

    ops, _ := ovs.WhereAll(ls,
        client.Condition{
            Field: &ls.Config,
            Function: ovsdb.ConditionIncludes,
            Value: map[string]string{"foo": "bar"},
        }, client.Condition{
            Field: &ls.Config,
            Function: ovsdb.ConditionIncludes,
            Value: map[string]string{"bar": "baz"},
        }).Delete()
    ovs.Transact(ops...)

### Querying with Select

The `Select` operation is used to build an OVSDB `select` query. Unlike the `List` operation, which returns results directly from the cache, `Select` only generates `ovsdb.Operation`(s). You must pass these operations to `Transact` to execute them against the database, and then use a helper function like `GetSelectResults` to parse the reply.

The `Select` operation can be used in two ways:
- **Direct Select**: Call `Select()` directly on the API for unconditional selection (selects all rows from a table)
- **Conditional Select**: Call `Select()` on a `ConditionalAPI` created with `WhereXxx()` methods for filtered selection

The core workflows are:
- **Direct**: `ovs.Select(model, fields...) -> Transact(...) -> GetSelectResults(...)`
- **Conditional**: `ovs.WhereXxx(...).Select(model, fields...) -> Transact(...) -> GetSelectResults(...)`

#### Selecting Rows

**Select All Rows from a Table:**

To select all rows from a table, call `Select()` directly on the API. The model instance is used only to determine the target table and doesn't filter results.

    // var ovs client.Client
    // var ctx context.Context
    var allSwitches []*MyLogicalSwitch // Target must be a slice of pointers to models
    // 1. Generate Op: Direct Select selects all rows
    selectOps, err := ovs.Select(&MyLogicalSwitch{})
    // ...
    // 2. Execute transaction
    reply, err := ovs.Transact(ctx, selectOps...)
    // ...
    // 3. Parse result
    err = ovs.GetSelectResults(selectOps, reply, &allSwitches)
    // ...

**Select by Index:**

You can use `Where()` with a model instance that has indexed fields populated. This will create a condition to find matching rows based on the first available index.

    // Assuming "Name" is an indexed field
    ls := &MyLogicalSwitch{Name: "switch1"}
    var results []*MyLogicalSwitch
    // 1. Generate Op
    selectOps, err := ovs.Where(ls).Select(ls)
    // ...
    // 2. Transact and parse
    reply, err := ovs.Transact(ctx, selectOps...)
    err = ovs.GetSelectResults(selectOps, reply, &results)
    // ...

**Select with Conditions (AND):**

Use `WhereAll()` to set the table context and one or more filter conditions. All conditions are combined with an `AND` operator.

    var specificSwitches []*MyLogicalSwitch
    lsModel := &MyLogicalSwitch{}
    cond1 := model.Condition{Field: &lsModel.Name, Function: ovsdb.ConditionEqual, Value: "sw1"}
    cond2 := model.Condition{Field: &lsModel.Ports, Function: ovsdb.ConditionIncludes, Value: "some_port_uuid"}

    // 1. Generate Op: Use WhereAll for one or more AND conditions
    selectOps, err := ovs.WhereAll(lsModel, cond1, cond2).Select(lsModel)
    // ...
    // 2. Transact and parse
    reply, err := ovs.Transact(ctx, selectOps...)
    err = ovs.GetSelectResults(selectOps, reply, &specificSwitches)
    // ...

**Select with Conditions (OR):**

Use `WhereAny()` to set the table context and one or more filter conditions. `WhereAny` will generate one `select` operation per condition. All generated operations belong to the same select query, allowing `GetSelectResults` to aggregate the results into a single slice.

    var specificSwitches []*MyLogicalSwitch
    lsModel := &MyLogicalSwitch{}
    cond1 := model.Condition{Field: &lsModel.Name, Function: ovsdb.ConditionEqual, Value: "sw1"}
    cond2 := model.Condition{Field: &lsModel.Ports, Function: ovsdb.ConditionIncludes, Value: "some_port_uuid"}

    // 1. Generate Ops: Use WhereAny for one or more OR conditions
    // This generates multiple operations, one for each condition
    selectOps, err := ovs.WhereAny(lsModel, cond1, cond2).Select(lsModel)
    // ...
    // 2. Transact and parse
    reply, err := ovs.Transact(ctx, selectOps...)
    // GetSelectResults will parse all results and populate them into the target slice.
    err = ovs.GetSelectResults(selectOps, reply, &specificSwitches)
    // ...

**Select with Cache-based Filtering:**

Use `WhereCache()` to filter rows based on a predicate function that operates on cached data. This generates one `select` operation per matching cached row, using UUID-based conditions. All generated operations belong to the same select query.

    var matchingSwitches []*MyLogicalSwitch

    // 1. Generate Ops: Use WhereCache with a predicate function
    // This generates multiple operations, one for each matching row in cache
    selectOps, err := ovs.WhereCache(func(ls *MyLogicalSwitch) bool {
        return strings.HasPrefix(ls.Name, "ext_")
    }).Select(&MyLogicalSwitch{})
    // ...
    // 2. Transact and parse
    reply, err := ovs.Transact(ctx, selectOps...)
    // GetSelectResults will parse all results and populate them into the target slice.
    err = ovs.GetSelectResults(selectOps, reply, &matchingSwitches)
    // ...

#### Selecting Specific Columns

By default, `Select` queries all columns of a table. You can pass field pointers to the `Select` method to retrieve only specific columns. The `_uuid` column is always included in the result.

**For Direct Select (all rows with specific columns):**

    // Selects only the "name" and "ports" columns for all rows using field pointers
    ls := &MyLogicalSwitch{}
    selectOps, err := ovs.Select(ls, &ls.Name, &ls.Ports)

**For Conditional Select (filtered rows with specific columns):**

    // Selects only the "name" and "ports" columns for rows matching the condition
    ls := &MyLogicalSwitch{Name: "sw1"}
    selectOps, err := ovs.Where(ls).Select(ls, &ls.Name, &ls.Ports)

#### Processing Select Results with GetSelectResults and GetSelectResultsByIndex

Multiple `Select()` operations can be bundled within a transaction.
**GetSelectResults** retrieves the results for the first select operation.
**GetSelectResultsByIndex** allows you to choose which select operation to retrieve the results for:

    // Example: Multiple separate Select() calls for the same model
    var ptrResults1, ptrResults2 []*MyLogicalSwitch  // Target must be a pointer slice

    // First Select() call - creates select query #0
    ls1 := &MyLogicalSwitch{Name: "sw1"}
    selectOps1, _ := ovs.Where(ls1).Select(ls1)

    // Second Select() call - creates select query #1
    ls2 := &MyLogicalSwitch{Name: "sw2"}
    selectOps2, _ := ovs.Where(ls2).Select(ls2)

    // Combine operations for a single transaction
    allOps := append(selectOps1, selectOps2...)
    reply, _ := ovs.Transact(ctx, allOps...)

    // Get results from first Select() call (index 0)
    err = ovs.GetSelectResults(allOps, reply, &ptrResults1)
    err = ovs.GetSelectResultsByIndex(allOps, reply, &ptrResults1, 0) // Explicit index 0

    // Get results from second Select() call (index 1)
    err = ovs.GetSelectResultsByIndex(allOps, reply, &ptrResults2, 1)

## Monitor for updates

You can also register a notification handler to get notified every time an element is added, deleted or updated from the database.

    handler := &cache.EventHandlerFuncs{
        AddFunc: func(table string, model model.Model) {
            if table == "Logical_Switch" {
                fmt.Printf("A new switch named %s was added!!\n!", model.(*MyLogicalSwitch).Name)
            }
        },
    }
    ovs.Cache.AddEventHandler(handler)

## modelgen

In this repository there is also a code-generator capable of generating all the Model types for a given ovsdb schema (json) file.

It can be used as follows:

    go install github.com/ovn-kubernetes/libovsdb/cmd/modelgen

    $GOPATH/bin/modelgen -p ${PACKAGE_NAME} -o {OUT_DIR} ${OVSDB_SCHEMA}
    Usage of modelgen:
            modelgen [flags] OVS_SCHEMA
    Flags:
      -d    Dry run
      -o string
            Directory where the generated files shall be stored (default ".")
      -p string
            Package name (default "ovsmodel")

The result will be the definition of a Model per table defined in the ovsdb schema file.
Additionally, a function called `FullDatabaseModel()` that returns the `ClientDBModel` is created for convenience.

Example:

Download the schema:

    ovsdb-client get-schema "tcp:localhost:6641" > mypackage/ovs-nb.ovsschema

Run `go generate`

    cat <<EOF > mypackage/gen.go
    package mypackage

    // go:generate modelgen -p mypackage -o . ovs-nb.ovsschema
    EOF
    go generate ./...

In your application, load the ClientDBModel, connect to the server and start interacting with the database:

    import (
        "fmt"
        "github.com/ovn-kubernetes/libovsdb/client"

        generated "example.com/example/mypackage"
    )

    func main() {
        dbModelReq, _ := generated.FullDatabaseModel()
        ovs, _ := client.Connect(context.Background(), dbModelReq, client.WithEndpoint("tcp:localhost:6641"))
        ovs.MonitorAll()

        // Create a *LogicalRouter, as a pointer to a Model is required by the API
        lr := &generated.LogicalRouter{
            Name: "myRouter",
        }
        ovs.Get(lr)
        fmt.Printf("My Router has UUID: %s and %d Ports\n", lr.UUID, len(lr.Ports))
    }

## Running the tests

To run integration tests, you'll need access to docker to run an Open vSwitch container.
Mac users can use [boot2docker](http://boot2docker.io)

    export DOCKER_IP=$(boot2docker ip)

    docker-compose run test /bin/sh
    # make test-local
    ...
    # exit
    docker-compose down

By invoking the command **make**, you will automatically get the same behavior as what
is shown above. In other words, it will start the two containers and execute
**make test-local** from the test container.

## Contact

The libovsdb community is part of ovn-kubernetes and can be contacted in the _#libovsdb_ channel in
[CNCF Slack server](https://cloud-native.slack.com/)
