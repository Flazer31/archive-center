# Tests

Published baseline: **4.3.0 stable**. Current local recall fixes are packaged in
[4.3.1-test.4](../docs/archive-center-4.3.1-test-build-4.md), including the context assembly cost repair; source regressions and live quality are separate.
See the [public release record](../docs/archive-center-4.3.0-release-verification.md) for that older release's evidence.

The fixed memory behavior baseline is **4.3.1 (local test.4)**; see the
[preservation criteria and remaining work](../docs/archive-center-memory-recall-restoration-plan.md#memory-baseline-431).
Future memory changes compare with both that baseline and the previous verified version.
Reuse the recall suites, version-comparison cases and long-query ordered-output checks.
Cover basic memory, Publisher only, editor only and both, plus empty/partial/failed-supplement
responses. Compare candidate arrival and final delivery separately from model prose.
Known state/interpretation errors are not golden expectations; public 4.3.0 remains the release comparison.

Previous test snapshot: 2026-09-09, `4.3.0-test.23` source. This directory is retained
from the old R0 layout; the implemented suites live alongside their owners below.

| Scope | Location |
| --- | --- |
| Go handlers, memory selection, provider transport and storage | `go-service/internal/**/` production `*_test.go` files |
| General recall restoration, per-query provenance, public evidence and ten editor/Publisher combinations | `go-service/internal/httpapi/memory_recall_restoration*_test.go` (external boundaries use local controlled responses) |
| Host adapter, HUD and JavaScript routes | [js-route-variant-smoke](../go-service/cmd/js-route-variant-smoke/) |
| Cold-start JS merge through the real Go routing handler; pending-source lineage | [worldline_cold_start_reproduction_test.go](../go-service/internal/httpapi/worldline_cold_start_reproduction_test.go), [Host fixture runner](fixtures/worldline-coldstart-probe.cjs) (Node required) |
| Standalone settings browser checks | [preprocessing-ui-smoke.cjs](../ops/preprocessing-ui-smoke.cjs) |
| Current local package verification and pending live cases | [test.4 record](../docs/archive-center-4.3.1-test-build-4.md); earlier [test.23](../docs/archive-center-4.3-test-build-23.md) remains historical |

Long-query particle expansion performance uses the production owner:
`go test ./internal/httpapi -run '^$' -bench '^BenchmarkMemoryRestorationRecallTerms$' -benchtime=1x`.
The benchmark verifies ordered output and measures increasing distinct-cue sizes;
ordinary tests have no wall-clock acceptance threshold. Test.4 includes an original/current
comparison and an actual-session snapshot assembly replay, separate from live model output.

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
