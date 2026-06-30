# SQLite -> MariaDB Export/Import Dry-Run Plan

> Status: **R0/R1 design contract** - no live authority switch permitted.
> Actual DB migration is **explicitly banned** until explicit approval.

---

## 1. Scope & Goals

This document defines a **dry-run** (no-op) export/import pipeline from the 0.8 SQLite runtime (`memory.db`, `bundle_validation.db`) to the MariaDB canonical truth skeleton.

Goals:
- Identify every table and row that must move.
- Define row-count, checksum, and replay parity checks.
- Keep the pipeline runnable in R0/R1 **without** switching write authority to MariaDB.
- Produce evidence that the MariaDB schema can absorb the entire 0.8 dataset.

---

## 2. 0.8 SQLite Source Analysis

### 2.1 DB Files
| File | Role | Schema | Current Size | WAL |
|------|------|--------|--------------|-----|
| `memory.db` | Primary runtime | 23 tables (see `models.py`) | ~1.3 MB | WAL active (`*.db-wal`, `*.db-shm`) |
| `bundle_validation.db` | Validation mirror | Same schema, smaller subset | ~400 KB | WAL active |
| `.chroma_shadow/chroma.sqlite3` | Vector store | Chroma internal | ~168 KB | N/A |

### 2.2 Table Classification (memory.db)

| # | Table | Canonical? | Write Pattern | Export Priority |
|---|-------|------------|---------------|-----------------|
| 1 | `chat_logs` | **Yes** | Append-only | P0 |
| 2 | `effective_input_logs` | **Yes** | Append-only | P0 |
| 3 | `memories` | **Yes** | Append-only | P0 |
| 4 | `direct_evidence_records` | **Yes** | Insert + state transition | P0 |
| 5 | `kg_triples` | **Yes** | Insert + soft-delete | P0 |
| 6 | `audit_logs` | **Yes** | Append-only | P0 |
| 7 | `critic_feedback` | **Yes** | Append-only | P0 |
| 8 | `character_events` | **Yes** | Append-only | P0 |
| 9 | `active_states` | No (derived) | Insert per turn | P1 |
| 10 | `canonical_state_layers` | No (derived) | Insert per turn | P1 |
| 11 | `character_states` | No (derived) | Upsert snapshot | P1 |
| 12 | `world_rules` | No (derived) | Upsert + trust | P1 |
| 13 | `storylines` | No (derived) | Upsert + trust | P1 |
| 14 | `pending_threads` | No (derived) | Upsert + trust | P1 |
| 15 | `session_active_scopes` | No (derived) | Single-row upsert | P2 |
| 16 | `guidance_plan_states` | No (derived) | Single-row upsert | P2 |
| 17 | `guidance_compact_records` | No (derived) | Insert on compaction | P2 |
| 18 | `maintenance_pass_states` | No (derived) | Insert per pass | P2 |
| 19-22 | `episode/chapter/arc/saga_summaries` | No (summary) | Insert on generation | P2 |

> **Chroma vector data** (`.chroma_shadow/chroma.sqlite3`) is **out of scope** for MariaDB truth; it is covered by the Milvus shadow contract instead.

---

## 3. Export Phase (SQLite -> Intermediates)

### 3.1 Method
Read-only `SELECT` from SQLite; no writes to source DB.

- Open SQLite in **WAL-aware read-only mode** (`?mode=ro` or copy to temp file to avoid WAL lock contention).
- Stream rows as newline-delimited JSON (NDJSON) or batched CSV.
- One intermediate file per table.
- Include `_export_meta` header in each file: `table_name`, `export_timestamp`, `source_db_path`, `row_count`.

### 3.2 Canonical Truth Export Order
1. `chat_logs`
2. `effective_input_logs`
3. `memories`
4. `direct_evidence_records`
5. `kg_triples`
6. `audit_logs`
7. `critic_feedback`
8. `character_events`

### 3.3 Derived Table Export Order
9. `active_states`
10. `canonical_state_layers`
11. `character_states`
12. `world_rules`
13. `storylines`
14. `pending_threads`
15. `session_active_scopes`
16. `guidance_plan_states`
17. `guidance_compact_records`
18. `maintenance_pass_states`
19-22. Summary tables

> **Reason for order**: Foreign keys are soft (no DB-level FK in R0), but logical dependencies exist. Audit logs reference rows that must exist first; summaries reference turns.

---

## 4. Import Phase (Intermediates -> MariaDB Skeleton)

### 4.1 Target
The Go `internal/store` MariaDB skeleton (`mariadbStore`) currently returns `ErrNotEnabled` for every operation. In R1 the dry-run importer will:

- Parse intermediate NDJSON/CSV.
- Validate against MariaDB schema constraints (type widths, nullable rules).
- **Log** the intended `INSERT`/`UPSERT` statements.
- **Do not execute** live SQL (no open connection, no DSN required).
- Produce a `dry_run_report.json` containing:
  - `table_name`
  - `rows_discovered`
  - `rows_accepted`
  - `rows_rejected` (with reason)
  - `checksum_expected` (from export)
  - `checksum_calculated` (from parsed values)

### 4.2 Upsert Strategy
- **Append-only tables**: `INSERT` only; duplicate key becomes a rejection logged in dry-run report.
- **Upsert tables**: `INSERT ... ON DUPLICATE KEY UPDATE` (MariaDB syntax). Dry-run reports what the update would look like.
- **Soft-delete tables**: Import all rows including logically deleted ones; `valid_to` / `tombstoned` fields preserve deletion state.

---

## 5. Verification Plan

### 5.1 Row Count Parity

| Check | Source | Target (dry-run) | Pass Criteria |
|-------|--------|------------------|---------------|
| Per-table count | `SELECT COUNT(*) FROM <table>` | Rows parsed from intermediate | Equal |
| Per-session count | `SELECT chat_session_id, COUNT(*) FROM <table> GROUP BY chat_session_id` | Same grouping in dry-run report | Equal per session |

### 5.2 Content Checksum Parity

- **Per-row checksum**: SHA-256 of stable canonical JSON representation of row fields (excluding `id` auto-increment).
- **Per-table checksum**: XOR or sorted-join of per-row checksums.
- **Dry-run report** includes both expected (from export) and calculated (from parse) checksums.
- Discrepancy = schema mismatch or data truncation risk.

### 5.3 Replay / Round-Trip Validation

1. **Canonical replay**: Read all canonical truth rows from MariaDB dry-run parse log, feed them into the Go shadow `fakeTurnSideeffects`, and verify that the resulting derived table shape matches the SQLite derived table shape (row counts, key presence).
2. **Audit completeness**: Verify that every write to canonical truth has a corresponding `audit_logs` row in the export.
3. **Bundle validation replay**: Run the same export against `bundle_validation.db` and compare row counts with `memory.db` to confirm schema parity.

---

## 6. Dry-Run Procedure (R0/R1 Safe)

### Step 1 - Snapshot (read-only)
```bash
# Copy SQLite files while 0.8 backend is running (WAL-safe)
cp memory.db memory.db.snapshot
cp bundle_validation.db bundle_validation.db.snapshot
```

### Step 2 - Export
```bash
python tools/export_sqlite_to_ndjson.py \
  --db memory.db.snapshot \
  --out ./dry-run/exports/ \
  --canonical-only  # or --all for full dry-run
```

### Step 3 - Validate Schema
```bash
go run ./cmd/dry-run-validator/... \
  --export-dir ./dry-run/exports/ \
  --report ./dry-run/dry_run_report.json
```

> This uses the `mariadbStore` skeleton to parse and validate without any live connection.

### Step 4 - Compare Reports
```bash
go run ./cmd/compare-dry-run/... \
  --sqlite-db memory.db.snapshot \
  --dry-run-report ./dry-run/dry_run_report.json
```

### Step 5 - Decision Gate
- If row-count parity == 100% AND checksum parity == 100% -> **dry-run passed** (still no authority switch).
- If any discrepancy -> fix schema or exporter, repeat Steps 2-4.

---

## 7. Blockers & Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| WAL lock during export | Cannot read `memory.db` live | Use snapshot copy or SQLite backup API |
| JSON column truncation | MariaDB `JSON` vs SQLite `TEXT` | Validate max length during dry-run parse |
| `DateTime` precision loss | SQLite sub-second vs MariaDB `DATETIME(3)` | Round-trip test with `time.Now().UTC()` |
| Auto-increment `id` collision | SQLite `id` may not match MariaDB sequence | Exclude `id` from checksum; re-map in import |
| Derived table rebuild drift | Go shadow rebuild may not match Python heuristic | Accept bounded non-parity for derived tables; canonical truth is the gate |
| Authority switch accident | Live cutover before approval | `mariadbStore` returns `ErrNotEnabled`; `Config.Validate()` blocks live mode |

---

## 8. Go Skeleton Integration

The existing `internal/store` package already contains:
- `Store` interface with all canonical truth methods.
- `mariadbStore` returning `ErrNotEnabled`.
- `noopStore` for R0 shadow reads.

For the dry-run importer, a new `DryRunImporter` struct can be added in R1 that:
- Satisfies the `Store` interface.
- Logs every call instead of executing SQL.
- Emits the `dry_run_report.json` on `Close()`.

This keeps the pipeline testable without a live MariaDB instance.

---

## 9. Evidence Checklist

Before this plan is considered complete:

- [x] 0.8 SQLite schema inventoried (`models.py` + `memory.db` analysis).
- [x] Table classification (canonical vs derived) defined.
- [x] Export order respects logical dependencies.
- [x] Row-count, checksum, and replay verification steps documented.
- [x] Dry-run procedure uses only read-only SQLite access and no-op MariaDB skeleton.
- [x] Export tool script implemented (`tools/export_sqlite_to_ndjson.py`).
- [x] Dry-run validator Go binary implemented (`cmd/dry-run-validator/`).
- [x] Report comparison tool implemented (`cmd/compare-dry-run/`).
- [x] MariaDB import plan summary CLI implemented (`cmd/mariadb-dry-run-import/`).
- [x] Full end-to-end dry-run executed against a copied `memory.db` snapshot.

> Unchecked items are **R1 implementation tasks**, not R0 design blockers.

Implementation note (2026-05-23):
- `tools/export_sqlite_to_ndjson.py` exports canonical-only by default, supports `--all`, opens SQLite with read-only URI mode, writes one NDJSON file per exported table plus `manifest.json`, records missing canonical tables as skipped, and uses SHA-256 row/table checksums.
- `tools/test_export_sqlite_to_ndjson.py` covers canonical-only export, all-table export, default mode, id-excluded row checksums, order-independent table checksums, mutually exclusive flags, missing DB failure, and source DB non-mutation using temporary SQLite databases only.

Implementation note (2026-05-24):
- `go-service/cmd/dry-run-validator` validates exporter `manifest.json` plus canonical table NDJSON files without opening MariaDB or the real 0.8 database.
- It validates row counts, id-excluded row checksums, order-independent table checksums, required canonical fields, JSON column syntax, ignored noncanonical tables, manifest-declared skipped canonical tables, and strict-canonical missing-table behavior.
- Manager validation included package tests, full Go tests, vet, artifact scan, and a temp SQLite end-to-end check where `tools/export_sqlite_to_ndjson.py` output was accepted by `cmd/dry-run-validator`.

Implementation note (2026-05-24):
- `go-service/cmd/compare-dry-run` compares a SQLite DB snapshot opened read-only against a `dry-run-validator` JSON report.
- It validates row-count parity per table and propagates `checksum_expected` vs `checksum_calculated` mismatch.
- A skipped table that is absent from SQLite is a warning; a skipped table that is present in SQLite is a failure; a canonical SQLite table missing from the report is a failure.
- Supports `--sqlite-db`, `--dry-run-report`, and optional `--json` output.
- Exit code is non-zero on mismatch/failure and zero on full parity.
- No real 0.8 DB access, no MariaDB connection, no import, and no authority switch.

---

*Contract version: R1-2026-05-24*
*Reference: `Archive Center Beta 0.8(fix)/memory.db`, `backend/models.py`, `contracts/mariadb-truth-schema-plan.md`*

Implementation note (2026-05-24):
- go-service/cmd/mariadb-dry-run-import reads a validated exporter directory (manifest.json + canonical table NDJSON) and produces a MariaDB import plan summary JSON without any live database connection, DSN, or SQL execution.
- It reports per-table planned_operation (insert for canonical import), row_count, checksum_expected, sample_statement_shape, and overall status (ok / degraded / failed).
- Missing canonical tables not listed in skipped_missing_tables produce status: failed. Skipped canonical tables produce status: skipped with a warning.
- Row-count and checksum mismatches between manifest and NDJSON meta produce status: degraded with warnings.
- No real 0.8 DB access, no MariaDB driver, no import execution, and no authority switch.

Implementation note (2026-05-24):
- A full temp-file E2E dry-run was executed against a copied `Archive Center Beta 0.8(fix)/memory.db` snapshot.
- Pipeline: temp snapshot copy -> `tools/export_sqlite_to_ndjson.py --canonical-only` -> `cmd/dry-run-validator --strict-canonical` -> `cmd/mariadb-dry-run-import` -> `cmd/compare-dry-run --json`.
- Result: validator status `ok`, 8 canonical tables, 263 rows, 0 failed tables; import plan status `ok`, 8 tables, 263 rows, 0 warnings, 0 errors; compare status `ok` with matching canonical row counts and checksums.
- The compare report warned about noncanonical derived tables that were present in SQLite but intentionally absent from the canonical-only report. This is expected for this R1 run and does not imply MariaDB authority readiness.
