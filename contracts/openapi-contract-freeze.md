# OpenAPI Contract Freeze

> Status: R0 evidence. Schema extraction is preparatory, not a migration completion.
> Date: 2026-05-21

## Why This Exists

Before any route parity shadow work begins, the current Archive Center Beta 0.8(fix) FastAPI application must have its request/response contracts captured in a machine-readable form. The OpenAPI spec emitted by `app.openapi()` is the authoritative source of truth for:

- Route paths and HTTP methods
- Pydantic request/response schemas
- Operation IDs (used for tracing and metrics)
- Response status codes
- Tags and metadata

Without this freeze, later Go parity work would need to reverse-engineer Python types from source code, which is error-prone and incomplete.

## What Is Frozen

The `tools/extract_openapi_contract.py` script produces a **summary**, not a full schema dump. It captures:

- **Path count** — total number of distinct URL paths
- **Operation count** — total number of method+path combinations
- **Schema component count** — number of `#/components/schemas/*` entries
- **Method distribution** — counts per HTTP verb
- **Response status distribution** — counts per HTTP status code
- **Routes with request bodies** — POST/PUT/PATCH routes that accept structured input
- **Routes without response schemas** — endpoints that return plain text or empty responses
- **Duplicate operation IDs** — collisions that could break tracing
- **Per-route detail** — method, path, operationId, tags, request schema refs, response schema refs

## What Is Not Frozen

- Full JSON Schema dumps for every component (too large for R0 review; deferred to R1).
- Response body examples or validation rules.
- Authentication/security scheme details.
- Custom FastAPI dependency injection behavior (not expressible in OpenAPI).

## Tool Usage

```
PYTHONDONTWRITEBYTECODE=1 python tools/extract_openapi_contract.py --format markdown
```

Default source root is `M:\risulongmemory\Archive Center Beta 0.8(fix)`. Override with `--source-root`.

The tool:
- Sets dummy API key env vars in-process only.
- Uses `sqlite:///:memory:` to avoid creating DB files.
- Does not trigger `.env` mutation (recovery mode is explicitly disabled).
- Does not write bytecode (`sys.dont_write_bytecode = True`).

## Extraction Result

**Attempted**: 2026-05-21

**Status**: Succeeded — system Python 3.14 with `PYTHONDONTWRITEBYTECODE=1`.

**Method**: The extractor set dummy API key env vars in-process, disabled `.env` mutation, and used `sqlite:///:memory:` to avoid file creation. `app.openapi()` completed without panics.

### Summary Counts

| Metric | Value |
|--------|-------|
| Paths | 117 |
| Operations | 130 |
| Schema Components | 62 |
| Request Body Routes | 60 |

### Method Counts

| Method | Count |
|--------|-------|
| DELETE | 8 |
| GET | 59 |
| PATCH | 17 |
| POST | 45 |
| PUT | 1 |

### Response Status Counts

| Status | Count |
|--------|-------|
| 200 | 130 |
| 422 | 119 |

### Routes Without Response Schema

| Method | Path |
|--------|------|
| GET | /health |
| GET | /retrieval-index/runtime-config |
| GET | /chroma-shadow/preflight |
| POST | /chroma-shadow/bootstrap |
| GET | /intent-routing/runtime-config |
| GET | /wakeup |
| GET | /prompts |
| GET | /stats |
| GET | /sessions |
| GET | /metrics/lc1r/regression-corpus |
| GET | /metrics/lc1s/step17-bundle-closure |

### OpenAPI Warning

- **UserWarning**: Duplicate Operation ID for `get_retrieval_index_runtime_config_retrieval_index_runtime_config_get`
  - **Source**: Likely duplicate GET `/retrieval-index/runtime-config` registration in `main.py`.
  - **Classification**: Warning, not blocker. Recorded in `openapi_warnings`.

## Why Not Green

- OpenAPI extraction does not mean the Go skeleton implements any of these routes.
- The summary is read-only evidence; it is not a cutover gate.
- Schema components still need to be translated to Go structs.
- Validation behavior (custom validators, `model_validator`, `lifespan`) is not captured by OpenAPI.

## Blockers

| Blocker | Status | Detail |
|---------|--------|--------|
| FastAPI import may fail if backend has unmet side effects | known | Mitigated by dummy env vars and in-memory DB URL. If import still fails, the blocker will be recorded here. |
| Pydantic v2 custom validators | known | Not expressible in OpenAPI; must be documented separately. |
| SQLAlchemy engine creation during import | known | Uses `:memory:` URL; no file creation. |
