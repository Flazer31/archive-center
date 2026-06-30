# Go Service

Status: R0/R1 shadow skeleton active.

This directory contains the future Go-primary backend service for Archive Center 2.0. It does not implement live behavior yet.

## Structure

- `cmd/archive-center-go/` - Entry point.
- `internal/config/` - Configuration with safe defaults and live-cutover guard.
- `internal/httpapi/` - HTTP handlers (`/health`, `/ready`, `/version`).

## Endpoints (Current)

| Method | Path | Purpose |
|--------|------|---------|
| GET | /health | Liveness probe |
| GET | /ready | Readiness probe with dependency state |
| GET | /version | Build metadata |

## Port

Default bind: `127.0.0.1:28080`

## Mode

Default mode: `shadow`

Live and cutover are explicitly disabled by `config.Validate()` and `config.IsLiveCutoverAllowed()`.

## CLI Tools

### Shadow Parity Report

Run read-only probes against both Python and Go backends and emit a markdown parity report:

```bash
go run ./cmd/shadow-parity-report/main.go -python-base http://127.0.0.1:8000 -go-base http://127.0.0.1:28080 -out parity-report.md
```

Flags: `-python-base`, `-go-base`, `-out` (empty = stdout), `-timeout`.

The report is R1 evidence only. It compares status codes and top-level JSON keys for allowlisted read-only probes; it does not authorize R2 write routes, MariaDB authority, Milvus live reads, or Go default runtime switch.

## Dependencies

Stdlib only. No third-party modules.
