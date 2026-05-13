# API Guide

- [Conditional API](#conditional-api)
  - [Where](#where)
  - [WhereAny](#whereany)
  - [WhereAll](#whereall)
  - [WhereCache](#wherecache)
- [Client Indexes](#client-indexes)
- [Examples](#examples)
  - [List](#list)
  - [Create](#create)
  - [Get](#get)
  - [Update](#update)
  - [Mutate](#mutate)
  - [Delete](#delete)
- [Select](#select)
  - [Selecting Rows](#selecting-rows)
  - [Selecting Specific Columns](#selecting-specific-columns)
  - [Multiple Select Queries in One Transaction](#multiple-select-queries-in-one-transaction)
- [Cache Event Handlers](#cache-event-handlers)

## Conditional API

Operations that affect specific rows (`Update`, `Delete`, `Mutate`, `Select`) must
be called on a `ConditionalAPI`. There are four ways to create one.

### Where

`Where()` creates a `ConditionalAPI` based on the index fields of the provided models.
It checks `_uuid` first, then schema-defined indexes, then client-defined indexes, in
that order, and uses the first available index to generate a condition.

```go
ls := &LogicalSwitch{UUID: "foo"}
ls2 := &LogicalSwitch{UUID: "foo2"}
ops, err := ovs.Where(ls, ls2).Delete()
```

### WhereAny

`WhereAny()` creates a `ConditionalAPI` from an explicit list of `Condition` objects.
It generates one operation per condition, so rows matching **any** condition are affected.

```go
ls := &LogicalSwitch{}
ops, err := ovs.WhereAny(ls, client.Condition{
    Field:    &ls.Config,
    Function: ovsdb.ConditionIncludes,
    Value:    map[string]string{"foo": "bar"},
}).Delete()
```

### WhereAll

`WhereAll()` behaves like `WhereAny()` but with **AND** semantics. All conditions are
combined into a single operation, so only rows satisfying every condition are affected.

```go
ops, err := ovs.WhereAll(ls,
    client.Condition{
        Field:    &ls.Config,
        Function: ovsdb.ConditionIncludes,
        Value:    map[string]string{"foo": "bar"},
    },
    client.Condition{
        Field:    &ls.Config,
        Function: ovsdb.ConditionIncludes,
        Value:    map[string]string{"bar": "baz"},
    },
).Delete()
```

### WhereCache

`WhereCache()` filters rows using a predicate on the local cache. It is primarily
used for `List()`, but can also drive server-side operations — in that case it
generates one equality condition on `_uuid` per matching cached row.

```go
var lsList []LogicalSwitch
err = ovs.WhereCache(func(ls *LogicalSwitch) bool {
    return strings.HasPrefix(ls.Name, "ext_")
}).List(context.Background(), &lsList)
```

The table is inferred from the type accepted by the predicate function.

## Client Indexes

By default the client tracks schema indexes and uses them in `Get`, `Where`, and
`WhereAll`. You can declare additional indexes for a client instance — for example,
to index on a specific map key or on an optional column:

```go
dbModel, err := nbdb.FullDatabaseModel()
dbModel.SetIndexes(map[string][]model.ClientIndex{
    "Load_Balancer": {
        {Columns: []model.ColumnKey{{Column: "external_ids", Key: "myIdKey"}}},
    },
})
// connect ...
```

With this index, instead of a full cache scan:

```go
// slow — predicate runs against every row in the cache
ovs.WhereCache(func(lb *LoadBalancer) bool {
    return lb.ExternalIds["myIdKey"] == "myIdValue"
}).List(ctx, &results)
```

you can use an indexed lookup:

```go
lb := &LoadBalancer{
    ExternalIds: map[string]string{"myIdKey": "myIdValue"},
}
ovs.Where(lb).List(ctx, &results)
```

Unlike schema indexes, client indexes do not enforce uniqueness, so conditions based
on them may match multiple rows.

## Examples

### List

```go
var switches []MyLogicalSwitch
err = ovs.List(context.Background(), &switches)
for _, ls := range switches {
    fmt.Printf("%+v\n", ls)
}
```

### Create

```go
ops, err := ovs.Create(&MyLogicalSwitch{Name: "foo"})
_, err = ovs.Transact(context.Background(), ops...)
```

### Get

```go
ls := &MyLogicalSwitch{Name: "foo"} // "name" is in the index list
err = ovs.Get(context.Background(), ls)
```

### Update

```go
ls.Config["foo"] = "bar"
ops, err := ovs.Where(ls).Update(ls)
_, err = ovs.Transact(context.Background(), ops...)
```

### Mutate

```go
ops, err := ovs.Where(ls).Mutate(ls, client.Mutation{
    Field:   &ls.Config,
    Mutator: ovsdb.MutateOperationInsert,
    Value:   map[string]string{"foo": "bar"},
})
_, err = ovs.Transact(context.Background(), ops...)
```

### Delete

```go
// By index
ls := &LogicalSwitch{UUID: "myUUID"}
ops, err := ovs.Where(ls).Delete()
_, err = ovs.Transact(context.Background(), ops...)

// With a condition
ops, err = ovs.WhereAny(ls, client.Condition{
    Field:    &ls.Config,
    Function: ovsdb.ConditionIncludes,
    Value:    map[string]string{"foo": "bar"},
}).Delete()
_, err = ovs.Transact(context.Background(), ops...)
```

## Select

`Select` builds an OVSDB `select` query. Unlike `List` (which reads from the local
cache), `Select` generates `ovsdb.Operation`s that must be executed via `Transact`.
Use `GetSelectResults` to parse the reply.

Core patterns:

```
// All rows
ovs.Select(model, fields...) → Transact → GetSelectResults

// Filtered rows
ovs.WhereXxx(...).Select(model, fields...) → Transact → GetSelectResults
```

### Selecting Rows

**All rows:**

```go
var allSwitches []*MyLogicalSwitch
selectOps, err := ovs.Select(&MyLogicalSwitch{})
reply, err := ovs.Transact(ctx, selectOps...)
err = ovs.GetSelectResults(selectOps, reply, &allSwitches)
```

**By index:**

```go
ls := &MyLogicalSwitch{Name: "switch1"} // "Name" is indexed
var results []*MyLogicalSwitch
selectOps, err := ovs.Where(ls).Select(ls)
reply, err := ovs.Transact(ctx, selectOps...)
err = ovs.GetSelectResults(selectOps, reply, &results)
```

**AND conditions:**

```go
lsModel := &MyLogicalSwitch{}
selectOps, err := ovs.WhereAll(lsModel,
    client.Condition{Field: &lsModel.Name, Function: ovsdb.ConditionEqual, Value: "sw1"},
    client.Condition{Field: &lsModel.Ports, Function: ovsdb.ConditionIncludes, Value: "port-uuid"},
).Select(lsModel)
reply, err := ovs.Transact(ctx, selectOps...)
err = ovs.GetSelectResults(selectOps, reply, &results)
```

**OR conditions:**

`WhereAny` generates one `select` operation per condition; `GetSelectResults` aggregates
all results into a single slice.

```go
selectOps, err := ovs.WhereAny(lsModel,
    client.Condition{Field: &lsModel.Name, Function: ovsdb.ConditionEqual, Value: "sw1"},
    client.Condition{Field: &lsModel.Name, Function: ovsdb.ConditionEqual, Value: "sw2"},
).Select(lsModel)
reply, err := ovs.Transact(ctx, selectOps...)
err = ovs.GetSelectResults(selectOps, reply, &results)
```

**Cache-based filtering:**

```go
selectOps, err := ovs.WhereCache(func(ls *MyLogicalSwitch) bool {
    return strings.HasPrefix(ls.Name, "ext_")
}).Select(&MyLogicalSwitch{})
reply, err := ovs.Transact(ctx, selectOps...)
err = ovs.GetSelectResults(selectOps, reply, &results)
```

### Selecting Specific Columns

Pass field pointers to `Select` to retrieve only those columns. `_uuid` is always included.

```go
ls := &MyLogicalSwitch{}
selectOps, err := ovs.Select(ls, &ls.Name, &ls.Ports)

// or conditionally:
selectOps, err = ovs.Where(&MyLogicalSwitch{Name: "sw1"}).Select(ls, &ls.Name, &ls.Ports)
```

### Multiple Select Queries in One Transaction

Use `GetSelectResultsByIndex` to retrieve results for a specific `Select` call when
multiple are bundled into one transaction.

```go
ls1 := &MyLogicalSwitch{Name: "sw1"}
selectOps1, _ := ovs.Where(ls1).Select(ls1)

ls2 := &MyLogicalSwitch{Name: "sw2"}
selectOps2, _ := ovs.Where(ls2).Select(ls2)

allOps := append(selectOps1, selectOps2...)
reply, _ := ovs.Transact(ctx, allOps...)

var results1, results2 []*MyLogicalSwitch
err = ovs.GetSelectResultsByIndex(allOps, reply, &results1, 0)
err = ovs.GetSelectResultsByIndex(allOps, reply, &results2, 1)
```

## Cache Event Handlers

Register a handler to be notified when rows are added, updated, or deleted:

```go
handler := &cache.EventHandlerFuncs{
    AddFunc: func(table string, model model.Model) {
        if table == "Logical_Switch" {
            fmt.Printf("New switch: %s\n", model.(*MyLogicalSwitch).Name)
        }
    },
    UpdateFunc: func(table string, old, new model.Model) { ... },
    DeleteFunc: func(table string, model model.Model) { ... },
}
ovs.Cache().AddEventHandler(handler)
```
