# Tests

Reviewed: 2026-09-08, active `4.3.0-test.21` source. This directory is retained
from the old R0 layout; the implemented suites live alongside their owners below.

| Scope | Location |
| --- | --- |
| Go handlers, memory selection, provider transport and storage | `go-service/internal/**/` production `*_test.go` files |
| Host adapter, HUD and JavaScript routes | [js-route-variant-smoke](../go-service/cmd/js-route-variant-smoke/) |
| Standalone settings browser checks | [preprocessing-ui-smoke.cjs](../ops/preprocessing-ui-smoke.cjs) |
| Current package verification and skipped live cases | [test.21 record](../docs/archive-center-4.3-test-build-21.md) |

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

Use the affected owner's existing tests first. The test.21 record covers 35 Go
packages and the 422-test Host suite; 12 environment-dependent checks were skipped.
Source, controlled fixtures, isolated browser rendering, packaged file identity,
loaded RisuAI, real services and final displayed output are separate evidence.
No runtime or user data belongs in this directory.
