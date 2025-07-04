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

Finally, a client object can be created:

    ovs, _ := client.Connect(context.Background(), dbModelReq, client.WithEndpoint("tcp:172.18.0.4:6641"))
    client.MonitorAll(nil) // Only needed if you want to use the built-in cache

Once the client object is created, a generic API can be used to interact with the Database. Some API calls can be performed on the generic API: `List`, `Get`, `Create`.

Others, have to be called on a `ConditionalAPI` (`Update`, `Delete`, `Mutate`). There are three ways to create a `ConditionalAPI`:

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
