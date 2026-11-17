# OVN release schema fixtures

These are unmodified Northbound and Southbound schemas from the official
[ovn-org/ovn](https://github.com/ovn-org/ovn) repository (Apache-2.0).
Release tags are resolved to the following immutable commits:

| Directory | Release tag | Commit | NB schema version | SB schema version |
|---|---|---|---|---|
| `23.03` | `v23.03.0` | `63e877fd81923367ca65f1002064799680eee70b` | `7.0.0` | not included |
| `25.09` | `v25.09.0` | `8890abf5a25b7b67f08aca8917c1d00cf2296a56` | `7.12.0` | `21.5.0` |
| `26.03` | `v26.03.0` | `0117988c0a9ebafd3b81befcf8ec8cc170276858` | `7.18.0` | `21.8.0` |

Source URL pattern:

```text
https://raw.githubusercontent.com/ovn-org/ovn/<commit>/ovn-nb.ovsschema
https://raw.githubusercontent.com/ovn-org/ovn/<commit>/ovn-sb.ovsschema
```

The fixtures are checked in so tests require neither network access nor an
installed OVN/OVS daemon. Do not replace them with schemas from a moving branch.

## What the tests cover

Run from the repository root:

```sh
go test ./client -run 'TestOVN(Release|Forward)Compatibility' -count=1
```

`client/ovn_compatibility_test.go` runs the full 2 × 2 × 2 matrix of client
release, server release, and compatibility mode for both databases. It checks
connection validation, every modeled table/column's support status, monitoring,
operation generation, and a transaction observed through the cache.

`client/ovn_forward_compatibility_test.go` additionally reproduces
[#448](https://github.com/ovn-kubernetes/libovsdb/issues/448): a 23.03 NB model
receiving 25.09 `Logical_Router_Port.status` updates. It covers initial rows,
inserts, legacy updates, and update2 modifications, including mixed known/unknown
fields and unknown-only changes. A live 25.09-client/26.03-server SB test verifies
that changing `Service_Monitor.mac` does not prevent known-field cache updates,
with compatibility mode both disabled and enabled. Default monitor requests are
also checked against the older model, including UUID-only models.

Tagged model structs are constructed with reflection from each complete schema
using its native Go types, avoiding hundreds of checked-in generated Go files.
This exercises the client and in-process OVSDB server, **not** modelgen or the
actual OVN release binaries.

Notable additions in 26.03:

- NB tables: `Logical_Switch_Port_Health_Check`, `Network_Function`,
  `Network_Function_Group`, `Network_Function_Health_Check`.
- NB columns: `Logical_Switch_Port.health_checks`, `ACL.network_function_group`,
  `Logical_Router_Policy.output_port`.
- SB table: `Advertised_MAC_Binding`.
- SB columns: `Service_Monitor.logical_input_port`, `Service_Monitor.mac`,
  `Service_Monitor.type`.

The SB `Service_Monitor.protocol` enum and `Logical_Flow.table_id` range also
expand. Their Go types remain unchanged, so model validation accepts them. This
is not a guarantee that values newly permitted in 26.03 are accepted by 25.09;
compatibility mode tolerates missing fields, not arbitrary constraint changes.
