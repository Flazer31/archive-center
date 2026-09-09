# Tests

Current source: **4.3.0 stable**, release verification in progress. See [release record](../docs/archive-center-4.3.0-release-verification.md). Prior test-build entries below are historical evidence.

Previous test snapshot: 2026-09-09, `4.3.0-test.23` source. This directory is retained
from the old R0 layout; the implemented suites live alongside their owners below.

| Scope | Location |
| --- | --- |
| Go handlers, memory selection, provider transport and storage | `go-service/internal/**/` production `*_test.go` files |
| Host adapter, HUD and JavaScript routes | [js-route-variant-smoke](../go-service/cmd/js-route-variant-smoke/) |
| Cold-start JS merge through the real Go routing handler; pending-source lineage | [worldline_cold_start_reproduction_test.go](../go-service/internal/httpapi/worldline_cold_start_reproduction_test.go), [Host fixture runner](fixtures/worldline-coldstart-probe.cjs) (Node required) |
| Standalone settings browser checks | [preprocessing-ui-smoke.cjs](../ops/preprocessing-ui-smoke.cjs) |
| Current package verification and skipped live cases | [test.23 record](../docs/archive-center-4.3-test-build-23.md) |

From the active `source` directory:

```powershell
node --check "Archive Center.js"
```

From `source/go-service`:

```powershell
go test ./... -count=1
```

For a change limited to the Host adapter, the existing focused suite is:

```powershell
go test ./cmd/js-route-variant-smoke -count=1
```

Use the affected owner's existing tests first. Test.22 adds regressions for
question objects, per-role ordering, exact compact provenance and accepted second
rounds. The record distinguishes the complete Go suite from frozen reply replay
and the environment-dependent live checks. Replay makes zero external AI calls.
Source, controlled fixtures, isolated browser rendering, packaged file identity,
loaded RisuAI, real services and final displayed output are separate evidence.
No runtime or user data belongs in this directory.
