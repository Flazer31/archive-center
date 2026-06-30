# Go Shadow API Contract

> Scope: `go-service` skeleton as of slice `2.0-1-basic-structure-migration`
> Status: R0/R1 preparatory; no live traffic
> Port: `127.0.0.1:28080` (non-conflicting with 0.8 backend)

## What the Go Skeleton Provides

### Endpoints

| Method | Path | Response Shape | Purpose |
|--------|------|----------------|---------|
| GET | /health | `{"status":"ok","service":"archive-center-go","mode":"shadow","timestamp":"..."}` | Liveness probe. |
| GET | /ready | Shadow mode: HTTP 200, `{"ready":true,"mode":"shadow","checks":{"shadow_mode":"active","mariadb":"not_configured","milvus":"not_configured","live_cutover":"disabled"},"timestamp":"..."}`<br>Non-shadow mode: HTTP 503, `{"ready":false,"mode":"...","checks":{"shadow_mode":"inactive","mode_guard":"mode \"...\" is disabled in this slice","mariadb":"...","milvus":"...","live_cutover":"disabled"},"timestamp":"..."}` | Readiness probe. In shadow mode it returns `ready=true` because shadow traffic does not require live DBs. In non-shadow modes it returns HTTP 503 with `ready=false` and a mode guard, even if the caller bypassed `main` validation. |
| GET | /version | `{"version":"2.0.0-dev","commit":"unknown","build_time":"...","go_version":"unknown"}` | Build metadata. |

### Behavior Guarantees

1. **Shadow-only default**: `Config.Mode` defaults to `shadow`. Live and cutover are accepted by `Load()` but rejected by `Validate()`.
2. **Live cutover disabled**: `Config.IsLiveCutoverAllowed()` returns `false` unconditionally. The readiness payload always reports `live_cutover: disabled`. Additionally, the `/ready` handler itself returns HTTP 503 and `ready: false` for any non-shadow mode, regardless of whether `main` validation was bypassed.
3. **No secrets in config**: `Config.String()` is redacted. Secrets must be injected via environment or a future secret manager.
4. **No DB connections**: MariaDB and Milvus are not wired. Readiness reports `not_configured` when the corresponding env vars are absent.
5. **Stdlib-only**: No third-party dependencies. The module uses `net/http`, `encoding/json`, `log/slog`, and `os` only.

## What the Go Skeleton Does NOT Provide

- **No turn processing**: `/prepare-turn`, `/complete-turn`, and related turn endpoints are not implemented.
- **No retrieval**: `/search`, `/retrieval-index/*`, `/kg/recall`, `/intent-routing/*` are not implemented.
- **No explorer CRUD**: `/explorer/*` routes are not implemented.
- **No narrative generation**: `/episodes/*`, `/chapters/*`, `/arcs/*`, `/sagas/*`, `/storylines/*`, `/characters/*`, `/world-rules/*`, `/pending-threads/*` are not implemented.
- **No metrics endpoints**: `/metrics/*` and `/momentum-packet/*` are not implemented.
- **No admin endpoints**: `/admin/*`, `/maintenance/*`, `/maintenance-pass/*`, `/long-session-health/*` are not implemented.
- **No audit/feedback**: `/audit`, `/feedback/*` are not implemented.
- **No prompt store**: `/prompts/*` are not implemented.
- **No import**: `/import/*` is not implemented.
- **No session utility**: `/sessions/*`, `/session/*`, `/active-states/*`, `/canonical-state-layer/*`, `/continuity-pack/*`, `/session-state/*` are not implemented.
- **No chroma-shadow workflows**: `/chroma-shadow/*` are not implemented.
- **No provider proxy**: `/supervisor` is not implemented.
- **No plugin-local routes**: `/proxy/plugin-main`, `/config/update` are not implemented.
- **No debug/test**: `/critic/test` is not implemented.

## Trace Vocabulary Alignment

The Go skeleton preserves the following trace vocabulary so that logs and probes remain compatible with 0.8 expectations:

- `archive-center-go` as the service name in health responses.
- `shadow` as the canonical mode string.
- `live_cutover: disabled` as the explicit guard value.
- `not_configured` / `configured` as dependency readiness states.

## Configuration Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `AC_BIND_ADDR` | `127.0.0.1:28080` | HTTP listen address. |
| `AC_MODE` | `shadow` | Runtime mode (`shadow`, `live`, `cutover`). Only `shadow` is valid today. |
| `AC_BUILD_VERSION` | `2.0.0-dev` | Build version string. |
| `AC_BUILD_COMMIT` | `unknown` | Build commit SHA. |
| `AC_BUILD_TIME` | current UTC | Build timestamp. |
| `AC_MARIADB_DSN` | (empty) | If present, readiness reports `mariadb: configured`. |
| `AC_MILVUS_ENDPOINT` | (empty) | If present, readiness reports `milvus: configured`. |

## Relation to 2.0 Milestones

This slice provides **initial R0 evidence and structure** for the items below. It does **not** claim green status, cutover readiness, or feature completeness.

| Milestone | Connection |
|-----------|------------|
| 2.0-1a (Go module scaffold) | **Supported / prepares R0 evidence for.** Module, main, config, and httpapi packages exist and compile. |
| 2.0-1g (Config defaults + env override) | **Supported / prepares R0 evidence for.** `config.Default()` and `config.Load()` are implemented and tested. |
| 2.0-1j (Readiness probe with dependency states) | **Supported / prepares R0 evidence for.** `/ready` reports `not_configured` for MariaDB and Milvus, and `disabled` for live cutover. It also returns HTTP 503 for non-shadow modes as a runtime guard. |
| 2.0-2a (Route parity shadow) | **Not started**. Requires JSON schema extraction from 0.8 Pydantic models. Suggested as next step. |
| 2.0-2b (Baseline benchmark capture) | **Not started**. Suggested as next step after route inventory is accepted. |
| MariaDB truth cutover | **Blocked** until R2. Must remain a separate event. |
| Milvus live read switch | **Blocked** until R2. Must remain a separate event. |

## Risk Notes

- The `live` and `cutover` modes are parsed but explicitly rejected by `Validate()` and blocked at the HTTP handler layer (`/ready` returns HTTP 503). This is intentional: it prevents accidental activation if an operator sets `AC_MODE=live` before the stack is ready, even when `main` validation is bypassed.
- `IsLiveCutoverAllowed()` is a hard-coded `false` guard. Future R2 work should replace this with a feature-flag or explicit operator approval gate, not an environment variable alone.
- No secrets are embedded in source or config structs. The next slice that adds DB connectivity must introduce a secret-management boundary (e.g., env-only, vault, or sealed secrets).
