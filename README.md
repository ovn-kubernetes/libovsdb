# libovsdb

[![libovsdb-ci](https://github.com/ovn-kubernetes/libovsdb/actions/workflows/ci.yml/badge.svg)](https://github.com/ovn-kubernetes/libovsdb/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/ovn-kubernetes/libovsdb/badge.svg?branch=main)](https://coveralls.io/github/ovn-kubernetes/libovsdb?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/ovn-kubernetes/libovsdb)](https://goreportcard.com/report/github.com/ovn-kubernetes/libovsdb)

An OVSDB client library written in Go.

## What is OVSDB?

OVSDB is the Open vSwitch Database Protocol, defined in [RFC 7047](http://tools.ietf.org/html/rfc7047).
It is used primarily to manage the configuration of Open vSwitch and OVN.

## Quick Start

### 1. Define a Model

Models are tagged Go structs that map to OVSDB tables:

```go
type MyLogicalSwitch struct {
    UUID   string            `ovsdb:"_uuid"` // required
    Name   string            `ovsdb:"name"`
    Ports  []string          `ovsdb:"ports"`
    Config map[string]string `ovsdb:"other_config"`
}
```

OVSDB types map to Go types as follows:

| OVSDB type | Go type |
|---|---|
| Set (min 0, max unlimited) | Slice |
| Set (min 0, max 1) | Pointer to scalar |
| Set (min 0, max N) | Array of N |
| Enum | Type-aliased string |
| Map | Map |
| Scalar | Equivalent Go scalar |

### 2. Create a Database Model

```go
dbModel, err := model.NewClientDBModel("OVN_Northbound", map[string]model.Model{
    "Logical_Switch": &MyLogicalSwitch{},
})
```

### 3. Connect

```go
ovs, err := client.NewOVSDBClient(dbModel, client.WithEndpoint("tcp:172.18.0.4:6641"))
if err != nil { ... }

err = ovs.Connect(context.Background())
err = ovs.MonitorAll(context.Background()) // required for cache-based operations
```

### 4. Interact with the Database

```go
// List all rows
var switches []MyLogicalSwitch
err = ovs.List(context.Background(), &switches)

// Create
ops, err := ovs.Create(&MyLogicalSwitch{Name: "foo"})
_, err = ovs.Transact(context.Background(), ops...)

// Get by index
ls := &MyLogicalSwitch{Name: "foo"}
err = ovs.Get(context.Background(), ls)

// Update
ls.Config["key"] = "value"
ops, err = ovs.Where(ls).Update(ls)
_, err = ovs.Transact(context.Background(), ops...)

// Delete
ops, err = ovs.Where(ls).Delete()
_, err = ovs.Transact(context.Background(), ops...)
```

For the full API — conditional queries (`Where`, `WhereAny`, `WhereAll`, `WhereCache`),
client-defined indexes, `Select`, and cache event handlers — see the [API guide](docs/api.md).

## Code Generation

`modelgen` generates Go model types from an OVSDB schema file, so you don't have to
write them by hand. See the [modelgen guide](docs/modelgen.md).

## Package Documentation

API reference for each sub-package is available on [pkg.go.dev](https://pkg.go.dev/):

| Package | Description | |
|---|---|---|
| `client` | OVSDB client and API | [![][clientbadge]][clientdoc] |
| `cache` | Model-based cache | [![][cachebadge]][cachedoc] |
| `model` | Model and database model | [![][modelbadge]][modeldoc] |
| `ovsdb` | Low-level OVSDB types | [![][ovsdbbadge]][ovsdbdoc] |
| `mapper` | Tagged struct ↔ OVSDB mapping | [![][mapperbadge]][mapperdoc] |
| `modelgen` | Code-generator library | [![][genbadge]][gendoc] |
| `server` | In-process test server | [![][serverbadge]][serverdoc] |
| `database` | Database types and interfaces | [![][dbbadge]][dbdoc] |
| `updates` | Model update helpers | [![][updatesbadge]][updatesdoc] |

[clientbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/client
[mapperbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/mapper
[modelbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/model
[ovsdbbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/ovsdb
[cachebadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/cache
[genbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/modelgen
[serverbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/server
[dbbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/database
[updatesbadge]: https://pkg.go.dev/badge/github.com/ovn-kubernetes/libovsdb/updates
[clientdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/client
[mapperdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/mapper
[modeldoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/model
[ovsdbdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/ovsdb
[cachedoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/cache
[gendoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/modelgen
[serverdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/server
[dbdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/database
[updatesdoc]: https://pkg.go.dev/github.com/ovn-kubernetes/libovsdb/updates

## Testing

### Unit Tests

```console
make test
```

### Integration Tests

Integration tests require Docker with a running OVS container:

```console
make integration-test
```

To test against a specific OVS version:

```console
OVS_VERSION=v3.4.0 make integration-test
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Contact

The libovsdb community is part of ovn-kubernetes. Find us in the
[#libovsdb channel](https://cloud-native.slack.com/) on the CNCF Slack server.
