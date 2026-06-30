# Admin / Maintenance Route Plan

> Status: **R0/R1 design contract** — live implementation banned.
> All mutating admin/maintenance routes in Go return `shadow_guard` (503) until explicit approval.

---

## 1. 0.8 Admin / Maintenance Route Inventory

| # | Method | Path | 0.8 Handler | Request DTO | Response DTO | Side Effect | Destructive |
|---|--------|------|-------------|-------------|--------------|-------------|-------------|
| 1 | GET | `/maintenance/queue-status` | `get_maintenance_queue_status` | query `session_id` | dict | None | **No** |
| 2 | POST | `/maintenance-pass/{chat_session_id}` | `run_maintenance_pass` | `MaintenancePassRequest` | `MaintenancePassResult` | Writes `MaintenancePassState` row | **Yes** |
| 3 | POST | `/maintenance/enqueue` | `enqueue_maintenance_job` | `MaintenanceEnqueueRequest` | `MaintenanceEnqueueResponse` | Enqueues background job | **Yes** |
| 4 | POST | `/admin/reindex` | `admin_reindex` | `ReindexRequest` | dict | Re-embeds memories, marks retrieval index dirty | **Yes** |
| 5 | POST | `/admin/rescan` | `admin_rescan` | `RescanRequest` | dict | Re-runs Critic over chat logs, inserts memories | **Yes** |
| 6 | POST | `/admin/session-migrate` | `admin_session_migrate` | `SessionMigrateRequest` | dict | Rewrites session-scoped rows across tables | **Yes** |
| 7 | POST | `/turns/repair-replay` | `repair_turn_replay` | `ChatLogRepairReplayRequest` | dict | Repairs chat logs + re-runs downstream | **Yes** |
| 8 | POST | `/chroma-shadow/rebuild-drill` | `run_chroma_shadow_rebuild_drill` | `ChromaShadowRebuildDrillRequest` | dict | Rebuilds vector collection | **Yes** |
| 9 | POST | `/import/hypamemory` | `import_hypamemory` | `HypaImportRequest` | dict | Bulk-inserts summaries into memories | **Yes** |

---

## 2. Destructiveness Classification

### 2.1 Read-Only (Non-Destructive)
- `GET /maintenance/queue-status`
  - Returns queue depth and job list.
  - No state mutation.
  - Go R1 shadow: **placeholder read-only response allowed**.

### 2.2 State-Mutating (Destructive)
- `POST /maintenance-pass/{chat_session_id}`
  - Writes a `MaintenancePassState` row.
  - In 0.8 `shadow_only=True` by default; even then it writes to the DB.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /maintenance/enqueue`
  - Enqueues a background maintenance job.
  - May trigger `maintenance-pass`, `episode` generation, etc.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /admin/reindex`
  - Re-embeds session memories with the current embedding model.
  - Updates `memories.embedding`, `memories.embedding_model`.
  - Marks retrieval index dirty.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /admin/rescan`
  - Re-runs Critic over existing `chat_logs`.
  - May insert new `memories`, `direct_evidence_records`, `audit_logs` rows.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /admin/session-migrate`
  - Rewrites `chat_session_id` and related foreign keys across multiple tables.
  - Risk of partial migration and data inconsistency.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /turns/repair-replay`
  - Rewrites `chat_logs` entries and re-runs downstream pipelines.
  - `dry_run` flag exists but defaults to false in 0.8.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /chroma-shadow/rebuild-drill`
  - Drops and rebuilds Chroma collection segments.
  - Vector data loss risk if backup missing.
  - **Go shadow: blocked with `shadow_guard`**.

- `POST /import/hypamemory`
  - Bulk-inserts external memory summaries.
  - May create duplicate or conflicting `memories` rows.
  - **Go shadow: blocked with `shadow_guard`**.

---

## 3. Operator Gate Requirements (Go Shadow)

### 3.1 Definition
An **operator gate** is a bearer-token or `X-RisuAI-Operator-Token` check that:
1. Requires a non-empty operator token in the request header.
2. Compares the token against `SharedState.OperatorToken` (or `Config.Auth.BearerToken`).
3. Rejects with `403 forbidden` / `CodeForbidden` if missing or mismatched.
4. Is **independent of the generic auth envelope** so that admin routes can have stricter rules.

### 3.2 Route-to-Gate Mapping

| Route | Go Handler | Current Gate | Required Gate in R2 |
|-------|------------|--------------|---------------------|
| `GET /maintenance/queue-status` | `handleMaintenanceQueueStatus` | None (placeholder) | Operator read gate |
| `POST /maintenance-pass/...` | `handleMaintenancePass` | `shadow_guard` | Operator write gate + audit log |
| `POST /maintenance/enqueue` | `handleMaintenanceEnqueue` | `shadow_guard` | Operator write gate + audit log |
| `POST /admin/reindex` | `handleAdminReindex` | `shadow_guard` | Operator write gate + audit log |
| `POST /admin/rescan` | `handleAdminRescan` | `shadow_guard` | Operator write gate + audit log |
| `POST /admin/session-migrate` | `handleAdminSessionMigrate` | `shadow_guard` | Operator write gate + audit log |

### 3.3 Audit Log Parity
In R2 every gated admin write should produce an `audit_logs`-equivalent trace:
- `event_type`: `"admin_reindex"`, `"admin_rescan"`, `"session_migrate"`, etc.
- `chat_session_id`: target session (or empty for global ops).
- `details_json`: request payload (with secrets redacted).
- Go skeleton: `trace.go` placeholder only; no live audit DB writes in R0/R1.

---

## 4. Go Implementation Status

### 4.1 Current Skeleton (`internal/httpapi/group_admin.go`)
```go
mux.HandleFunc("POST /admin/reindex", s.handleAdminReindex)
// ... etc
```

All mutating handlers currently call:
```go
writeShadowGuard(w, "POST /admin/reindex")
```

This returns:
- HTTP 503
- `{"status":"error","error":"... is not available in R0/R1 shadow mode","code":"shadow_guard"}`

### 4.2 R1 Additions (Allowed)
- `GET /maintenance/queue-status` placeholder with fake queue data.
- DTO decode helpers for `ReindexRequest`, `RescanRequest`, `SessionMigrateRequest`.
- Operator gate middleware skeleton (returns `403` if token missing, but does not block if `Enforce=false`).

### 4.3 R2 Additions (Requires Explicit Approval)
- Live side-effect implementations (reindex, rescan, migrate, enqueue, pass).
- Real audit log writes.
- Actual operator token enforcement.

---

## 5. Blockers & Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Accidental R2 activation | Admin route mutates live data before approval | All handlers call `writeShadowGuard`; `Config.Validate()` blocks live mode |
| Missing operator token | Unauthorized admin access in R2 | Skeleton middleware exists; enforcement is off (`Enforce=false`) until approval |
| Audit gap | Admin action not traceable | `trace.go` placeholders ready; live audit requires R2 |
| Reindex without backup | Embeddings lost if model changes | Not a Go concern yet; 0.8 already handles this |
| Session-migrate partial failure | Data inconsistency across tables | Go shadow does not run the migration; 0.8 handles rollback |

---

## 6. Evidence Checklist

- [x] 0.8 admin/maintenance routes inventoried (`main.py` + `services/admin.py` + `services/maintenance_pass.py`).
- [x] Destructive vs read-only classification defined.
- [x] Go `group_admin.go` skeleton registers all routes with `shadow_guard`.
- [x] Operator gate requirements documented for R2.
- [x] Audit log parity requirements documented for R2.
- [ ] R2 live implementation (explicitly banned until approval).

---

*Contract version: R0-2026-05-22*  
*Reference: `Archive Center Beta 0.8(fix)/backend/main.py`, `backend/services/admin.py`, `backend/services/maintenance_pass.py`, `backend/services/explorer.py`, `internal/httpapi/group_admin.go`*
