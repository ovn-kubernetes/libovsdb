libovsdb
========

[![libovsdb-ci](https://github.com/ovn-org/libovsdb/actions/workflows/ci.yml/badge.svg)](https://github.com/ovn-org/libovsdb/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/ovn-org/libovsdb/badge.svg?branch=main)](https://coveralls.io/github/ovn-org/libovsdb?branch=main) [![Go Report Card](https://goreportcard.com/badge/github.com/ovn-org/libovsdb)](https://goreportcard.com/report/github.com/ovn-org/libovsdb)

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

The resulting `ConditionalAPI` will create one operation per condition, so all the rows that match *any* of the specified conditions will be affected.

**WhereAll()**: `WhereAll()` behaves like `WhereAny()` but with *AND* semantics. The resulting `ConditionalAPI` will put all the
conditions into a single operation. Therefore the operation will affect the rows that satisfy *all* the conditions.

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
where each set consists of the columns that compose the index. Unlike schema indexes, a key within a column can be addressed if the column
type is a map.

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

* **client**: ovsdb client and API [![godoc for libovsdb/client][clientbadge]][clientdoc]
* **mapper**: mapping from tagged structs to ovsdb types [![godoc for libovsdb/mapper][mapperbadge]][mapperdoc]
* **model**: model and database model used for mapping [![godoc for libovsdb/model][modelbadge]][modeldoc]
* **ovsdb**: low level OVS types [![godoc for libovsdb/ovsdb][ovsdbbadge]][ovsdbdoc]
* **cache**: model-based cache [![godoc for libovsdb/cache][cachebadge]][cachedoc]
* **modelgen**: common code-generator functions  [![godoc for libovsdb/modelgen][genbadge]][gendoc]

[doc]: https://pkg.go.dev/
[clientbadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/client
[mapperbadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/mapper
[modelbadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/model
[ovsdbbadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/ovsdb
[cachebadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/cache
[genbadge]: https://pkg.go.dev/badge/github.com/ovn-org/libovsdb/modelgen
[clientdoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/client
[mapperdoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/mapper
[modeldoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/model
[ovsdbdoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/ovsdb
[cachedoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/cache
[gendoc]: https://pkg.go.dev/github.com/ovn-org/libovsdb/modelgen

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

    go install github.com/ovn-org/libovsdb/cmd/modelgen

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
        "github.com/ovn-org/libovsdb/client"

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

## Drop-in json library

There are two json libraries to use as a drop-in replacement for std json library. 
[go-json](https://github.com/goccy/go-json) and [json-iterator](https://github.com/json-iterator/go)

go build your application with -tags go_json or jsoniter

    $ benchstat bench.out.std bench.out.jsoniter bench.out.go_json
    goos: linux
    goarch: amd64
    pkg: github.com/ovn-org/libovsdb/ovsdb
    cpu: Intel(R) Core(TM) i5-9600K CPU @ 3.70GHz
    │ bench.out.std │           bench.out.jsoniter           │           bench.out.go_json           │
    │    sec/op     │    sec/op      vs base                 │    sec/op     vs base                 │
    MapMarshalJSON1-4            2.234µ ± ∞ ¹    1.621µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.289µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapMarshalJSON2-4            3.109µ ± ∞ ¹    2.283µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.834µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapMarshalJSON3-4            4.165µ ± ∞ ¹    2.900µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.404µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapMarshalJSON5-4            6.058µ ± ∞ ¹    4.119µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.421µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapMarshalJSON8-4            8.978µ ± ∞ ¹    5.685µ ± ∞ ¹        ~ (p=0.100 n=3) ²   4.912µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapUnmarshalJSON1-4          3.208µ ± ∞ ¹    2.437µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.944µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapUnmarshalJSON2-4          4.525µ ± ∞ ¹    3.579µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.659µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapUnmarshalJSON3-4          6.015µ ± ∞ ¹    4.780µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.404µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapUnmarshalJSON5-4          9.121µ ± ∞ ¹    7.061µ ± ∞ ¹        ~ (p=0.100 n=3) ²   4.914µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    MapUnmarshalJSON8-4         13.912µ ± ∞ ¹   10.242µ ± ∞ ¹        ~ (p=0.100 n=3) ²   7.076µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONString1-4      668.7n ± ∞ ¹    511.9n ± ∞ ¹        ~ (p=0.100 n=3) ²   393.8n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONString2-4     1756.0n ± ∞ ¹   1243.0n ± ∞ ¹        ~ (p=0.100 n=3) ²   972.6n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONString3-4      1.977µ ± ∞ ¹    1.320µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.082µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONString5-4      2.463µ ± ∞ ¹    1.561µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.327µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONString8-4      3.310µ ± ∞ ¹    1.929µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.647µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONInt1-4         605.4n ± ∞ ¹    495.1n ± ∞ ¹        ~ (p=0.100 n=3) ²   405.5n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONInt2-4         1.612µ ± ∞ ¹    1.253µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.057µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONInt3-4         1.766µ ± ∞ ¹    1.327µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.200µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONInt5-4         2.082µ ± ∞ ¹    1.560µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.423µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONInt8-4         2.554µ ± ∞ ¹    1.906µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.772µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONFloat1-4       676.8n ± ∞ ¹    562.1n ± ∞ ¹        ~ (p=0.100 n=3) ²   489.2n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONFloat2-4       1.599µ ± ∞ ¹    1.205µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.025µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONFloat3-4       1.761µ ± ∞ ¹    1.341µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.148µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONFloat5-4       2.090µ ± ∞ ¹    1.576µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.379µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONFloat8-4       2.543µ ± ∞ ¹    1.902µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.720µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONUUID1-4       1227.0n ± ∞ ¹    570.0n ± ∞ ¹        ~ (p=0.100 n=3) ²   485.8n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONUUID2-4        2.875µ ± ∞ ¹    1.311µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.142µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONUUID3-4        3.594µ ± ∞ ¹    1.490µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.313µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONUUID5-4        5.184µ ± ∞ ¹    1.832µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.674µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetMarshalJSONUUID8-4        7.555µ ± ∞ ¹    2.333µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.252µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONString1-4    848.8n ± ∞ ¹    769.3n ± ∞ ¹        ~ (p=0.100 n=3) ²   517.0n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONString2-4    2.722µ ± ∞ ¹    2.270µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.684µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONString3-4    3.264µ ± ∞ ¹    2.790µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.940µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONString5-4    4.482µ ± ∞ ¹    3.580µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.414µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONString8-4    5.980µ ± ∞ ¹    4.519µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.010µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONInt1-4       741.6n ± ∞ ¹    942.9n ± ∞ ¹        ~ (p=0.100 n=3) ²   514.6n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONInt2-4       2.394µ ± ∞ ¹    2.236µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.709µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONInt3-4       2.839µ ± ∞ ¹    2.667µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.048µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONInt5-4       3.521µ ± ∞ ¹    3.425µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.497µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONInt8-4       4.383µ ± ∞ ¹    4.227µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.066µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONFloat1-4     845.2n ± ∞ ¹    960.0n ± ∞ ¹        ~ (p=0.100 n=3) ²   541.3n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONFloat2-4     2.606µ ± ∞ ¹    2.264µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.748µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONFloat3-4     3.141µ ± ∞ ¹    2.667µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.040µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONFloat5-4     4.118µ ± ∞ ¹    3.436µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.560µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONFloat8-4     5.149µ ± ∞ ¹    4.286µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.141µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONUUID1-4     1802.0n ± ∞ ¹    879.3n ± ∞ ¹        ~ (p=0.100 n=3) ²   613.5n ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONUUID2-4      5.697µ ± ∞ ¹    2.503µ ± ∞ ¹        ~ (p=0.100 n=3) ²   1.899µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONUUID3-4      7.469µ ± ∞ ¹    3.035µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.218µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONUUID5-4     11.604µ ± ∞ ¹    3.965µ ± ∞ ¹        ~ (p=0.100 n=3) ²   2.987µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    SetUnmarshalJSONUUID8-4     17.097µ ± ∞ ¹    5.168µ ± ∞ ¹        ~ (p=0.100 n=3) ²   3.731µ ± ∞ ¹        ~ (p=0.100 n=3) ²
    geomean                      2.982µ          2.057µ        -31.03%                   1.606µ        -46.14%
    ¹ need >= 6 samples for confidence interval at level 0.95
    ² need >= 4 samples to detect a difference at alpha level 0.05

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

The libovsdb community is part of ovn-org and can be contacted in the *#libovsdb* channel in
[ovn-org Slack server](https://ovn-org.slack.com)
