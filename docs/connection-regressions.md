# Connection lifecycle regression coverage

This checklist maps the connection-manager work to upstream reports. It is not
an instruction to automatically close those issues or PRs; several overlap or
include changes outside this implementation.

## Portable coverage

| Upstream report | Local coverage | Scope |
| --- | --- | --- |
| [#464](https://github.com/ovn-kubernetes/libovsdb/issues/464), [#459](https://github.com/ovn-kubernetes/libovsdb/pull/459), [#460](https://github.com/ovn-kubernetes/libovsdb/pull/460) | `TestTransactDuringAbruptDisconnectNoPanic` | TCP proxy closes real connections during concurrent transactions. Ten iterations run in short/race CI, 100 otherwise; workers are joined and timeouts fail. The original trafficSeen panic also has an upstream fix in merged #488. |
| [#497](https://github.com/ovn-kubernetes/libovsdb/pull/497) | `client/disconnect_regression_test.go` | Server-side disconnect, accurate Connected state, notification retention until readiness, prompt failure after backoff exhaustion, and explicit reconnection after exhaustion. |
| [#492](https://github.com/ovn-kubernetes/libovsdb/pull/492) | `client/endpoint_failover_test.go` | Positive timeout validation, disconnected endpoint ordering, active endpoint rotation, shared deadline budgets, silent TCP RPC and TLS-handshake peers, and retry past a silent standby. These are not kernel packet-blackhole tests. |
| [#499](https://github.com/ovn-kubernetes/libovsdb/pull/499) | `server/close_test.go` | Server Close terminates accepted sockets, including concurrent accepts; repeated/concurrent Close and Close before Serve are safe. Old disconnect tests now fail rather than skip if server shutdown goes unnoticed. |
| [#250](https://github.com/ovn-kubernetes/libovsdb/issues/250), [#283](https://github.com/ovn-kubernetes/libovsdb/pull/283) | `TestMonitorRecoveryMatrix` | Single-monitor Found=true/false, unchanged/deleted rows, multiple tables, disjoint conditional scopes, and mixed monitor methods. Multiple monitors use full snapshots and a single purge. |
| [#501](https://github.com/ovn-kubernetes/libovsdb/pull/501) | Existing `TestEchoRace` plus FSM ownership | Endpoint mutations are serialized by the FSM. The rebased branch includes upstream's rpc2 v1.0.7, covering the dependency upgrade requested by #500/#501; local codec wrappers are retained. |

### RPC and recovery boundary coverage

- `TestTransactDoesNotReplayUncertainOutcome`: a peer commits then drops its
  reply; reconnect must not replay that operation.
- `TestTransactRejectsInvalidatedBarrier`: a disconnect invalidates queued
  updates; draining their barrier must not report successful cache application.
- `TestConnectDeadlineIncludesMonitorRestoration`: a silent restored monitor
  cannot outlive the Connect deadline or strand subsequent Connect calls. A
  successfully restored session outlives its caller's context.
- `TestConnectSkipsIncompatibleSchema`: an incompatible first endpoint does not
  prevent explicit connection to a compatible second endpoint.
- `TestConnectionManagerProbeUsesConfiguredDeadline`: short and long configured
  inactivity intervals govern the echo response deadline, not a fixed duration.

### Explicitly unsupported monitor configuration

The matrix documents that overlapping row scopes fail during initial monitor
registration with a cache-inconsistency error. This is not a successful recovery
case. Merely accepting duplicate snapshot rows would not resolve duplicate set/map
deltas, row ownership, or cancellation. Overlap support, selective per-monitor
purging, and multiple-monitor incremental replay remain follow-up work under
#250/#283.

## Deferred integration work

### Linux packet blackholes — PR #492 (also related to #368)

The upstream opt-in suite uses `LIBOVSDB_TEST_NETWORK_BLACKHOLE=1`, Linux socket
options, and firewall rules. It has not been imported or run here. A follow-up
should run it only inside an isolated network namespace with appropriate network
administration privileges, not against the developer/CI host's firewall.

Required cases:

- Active TCP and TLS connections stop receiving ACKs while RPC writes are pending.
- Inactivity probes and shutdown remain bounded under that condition.
- A blackholed standby cannot starve a healthy endpoint.
- `TCP_USER_TIMEOUT` is applied to the underlying TCP socket for TLS, not just
  plain TCP; validate the actual socket option and its duration bounds.
- Endpoint updates racing with failure retain the requested ordering.

### Schema conversion / monitor cancellation — PR #456

This is not solved by successful echo probes or by transport reconnect tests.
The upstream reproduction keeps a client monitoring a real OVS database,
converts its schema with `ovsdb-client convert`, and inserts another row. Echo
may still work while monitor updates have stopped.

A follow-up real-OVS test should assert:

1. A pre-conversion row arrives through the monitor.
2. Conversion is performed on a dedicated test database.
3. The client reacts correctly whether the server disconnects or sends
   `monitor_canceled` on a still-live connection.
4. Schema and monitor restoration finish before readiness is published.
5. A post-conversion row reaches the cache, even if echo continued succeeding.

The change-awareness negotiation (`set_db_change_aware`) and cancellation
semantics need implementation and compatibility decisions before adding this as
an expected-passing test. Do not add a permanently skipped placeholder claiming
coverage. PR #424's integration-test refactoring may be useful for that work.

Adaptive reconnect-timeout growth (#389) and disabling inactivity probes for
Unix sockets (#395) are also outside this pass.

## Running the portable regressions

```sh
go test -race -short -timeout 90s ./...
go test -race ./client -run '^TestTransactDuringAbruptDisconnectNoPanic$' -count=1 -timeout 90s
go test -race ./client ./server -run 'TestMonitorRecoveryMatrix|TestDisconnectNotification|TestReconnectGivingUp|Test.*Endpoint|TestClose.*' -count=20 -timeout 90s
```

The external OVS integration suite requires its own environment and is not
exercised by `-short`.
