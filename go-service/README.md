# Go Service

Current public version: **4.5.0**. See the [release verification](../docs/archive-center-4.5-release-verification.md) and [test guide](../tests/README.md).

This directory contains the Go-primary Archive Center backend. In the packaged
`live` profile it owns request planning, memory and source selection, prompt
assembly, budget calculation, canonical turn decisions, persistence,
orchestration and backend ViewModels. `Archive Center.js` remains the thin
RisuAI host adapter.

## Structure

- `cmd/archive-center-go/` - Backend entry point.
- `cmd/` - Release, migration, integrity and diagnostic tools.
- `internal/config/` - Runtime and provider configuration.
- `internal/httpapi/` - RisuAI bridge, turn lifecycle, memory, source and
  diagnostic HTTP handlers.
- `internal/store/` - MariaDB canonical persistence.
- [`../migrations/`](../migrations/README.md) - Versioned MariaDB schema, beside `go-service`.

## Runtime Boundary

- MariaDB is canonical truth.
- ChromaDB is a rebuildable vector retrieval lane, not canonical storage.
- Raw user and assistant text is preserved before critic-derived processing.
- JavaScript supplies host observations and applies backend decisions; it does
  not duplicate memory policy.
- `.env`, databases, vector collections, logs and user content stay outside the
  release package.

The permanent boundary is defined in
[`../docs/permanent-risu-host-backend-boundary.md`](../docs/permanent-risu-host-backend-boundary.md).

## Service

Default bind: `127.0.0.1:28080`

Core operational probes include:

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Process liveness |
| GET | `/ready` | Dependency and runtime readiness |
| GET | `/version` | Build metadata |

The product runtime uses `AC_STORE_MODE=mariadb_authority`. Development and
diagnostic modes are configuration-controlled and do not change the release
data ownership boundary.

## Validation

From `source/go-service`, use a writable Go cache/temp directory when needed, then run:

```powershell
go test ./... -count=1
```

See the [test map](../tests/README.md) for Host regressions and the distinction
between fixture checks and tests requiring MariaDB, ChromaDB or a real provider.
