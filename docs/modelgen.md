# modelgen

`modelgen` generates Go model types from an OVSDB schema file, so you don't need
to write them by hand. It produces one struct per table and a `FullDatabaseModel()`
convenience function.

## Installation

```console
go install github.com/ovn-kubernetes/libovsdb/cmd/modelgen
```

## Usage

```console
modelgen [flags] OVS_SCHEMA

Flags:
  -d        Dry run (print output, don't write files)
  -o string Directory where generated files are stored (default ".")
  -p string Package name (default "ovsmodel")
```

## Example

**1. Get the schema from a running OVSDB server:**

```console
ovsdb-client get-schema "tcp:localhost:6641" > mypackage/ovs-nb.ovsschema
```

**2. Add a `go:generate` directive:**

```go
// mypackage/gen.go
package mypackage

//go:generate modelgen -p mypackage -o . ovs-nb.ovsschema
```

**3. Run code generation:**

```console
go generate ./...
```

This writes one `.go` file per table plus a `FullDatabaseModel()` function.

## Using the Generated Code

```go
import (
    "context"
    "fmt"

    "github.com/ovn-kubernetes/libovsdb/client"
    generated "example.com/example/mypackage"
)

func main() {
    dbModel, err := generated.FullDatabaseModel()
    if err != nil { ... }

    ovs, err := client.NewOVSDBClient(dbModel, client.WithEndpoint("tcp:localhost:6641"))
    if err != nil { ... }

    err = ovs.Connect(context.Background())
    err = ovs.MonitorAll(context.Background())

    lr := &generated.LogicalRouter{Name: "myRouter"}
    err = ovs.Get(context.Background(), lr)
    fmt.Printf("Router %s has %d ports\n", lr.Name, len(lr.Ports))
}
```
