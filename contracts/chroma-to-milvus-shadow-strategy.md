# Chroma to Milvus Lite Shadow Strategy ? Archive Center 2.0 R0

> Status: **R0 design contract**  
> Live Milvus read switch is **explicitly banned** until explicit approval.

---

## 1. 0.8 Chroma Shadow Current Behavior

### 1.1 Architecture
- **Truth authority**: SQLite (`memory.db`).
- **Vector shadow**: Chroma (`chroma.sqlite3` in `.chroma_shadow/`).
- **Write path**: Every `prepare-turn` and `complete-turn` triggers `_run_chroma_shadow_backfill_batch_c17` if embedding model is ready.
- **Read path**: `search` uses SQLite `LIKE` + heuristic ranking; Chroma query is **not** on the live read path.
- **Consistency model**: Best-effort backfill. Chroma lags behind SQLite by design.

### 1.2 Shadow Write (0.8)
```python
# In complete-turn path
result = _run_chroma_shadow_backfill_batch_c17(req)
# result["sample_upsert_count"] ? how many docs were upserted
# result["count_before"] / result["count_after"] ? collection size delta
```
- Backfill is **fire-and-forget** inside `background_tasks`.
- Failure to upsert is logged but does not fail the turn.

### 1.3 Shadow Read Comparison (0.8)
- 0.8 does **not** compare Chroma results vs SQLite results on the live path.
- `search` handler reads from `memories` table directly.
- Chroma shadow is only exercised by:
  - `GET /chroma-shadow/preflight` ? health probe.
  - `POST /chroma-shadow/backfill-dry-run` ? diagnostic.
  - `POST /chroma-shadow/health-probe` ? sample query sanity check.

---

## 2. Milvus Lite Shadow Strategy

### 2.1 Dual-Lane Model (R1)

```
                    ???????????????????
  User Request      ?   Prepare/Complete Turn  ?
       ?            ???????????????????
       ?                     ?
       ?                     ?
????????????????    ????????????????????
?  SQLite      ?    ?  Milvus Lite     ?
?  (truth)     ?    ?  (shadow)        ?
?  memory.db   ?    ?  .milvus_lite/   ?
????????????????    ????????????????????
       ?                     ?
       ?                     ?
????????????????    ????????????????????
?  Live Read   ?    ?  Shadow Read     ?
?  (default)   ?    ?  (compare only)  ?
????????????????    ????????????????????
```

### 2.2 Shadow Write (R1)
- Same as 0.8 Chroma: fire-and-forget backfill from MariaDB canonical truth.
- `fakeVectorStore.Upsert` records calls but does not persist until live.
- Actual `milvusStore.Upsert` is disabled (`ErrNotEnabled`).

### 2.3 Shadow Read Compare (R1)
- **Not on the live path**.
- A dedicated probe endpoint (e.g., `POST /milvus-shadow/search-compare`) will:
  1. Run the same query against MariaDB `LIKE` search.
  2. Run the same query against Milvus Lite `Search`.
  3. Compare top-k overlap (Jaccard similarity on document IDs).
  4. Log divergence metrics to `audit_logs`.
- Divergence threshold: if overlap < 0.8, flag for review.

---

## 3. Stale Vector Handling

### 3.1 Definition
A vector is **stale** when:
- The source row in MariaDB has been updated (e.g., `memories.summary_json` changed).
- The source row has been soft-deleted (e.g., `kg_triples.valid_to` set).
- The embedding model has changed (e.g., from `text-embedding-3-small` to `text-embedding-3-large`).

### 3.2 Detection Strategy
1. **Version stamp**: Every upserted vector stores `schema_version` and `embedding_model` in metadata.
2. **Re-embedding audit** (`POST /chroma-shadow/reembed-audit`):
   - Sample vectors per tier.
   - Re-compute embedding for the source text.
   - Compare cosine similarity.
   - If similarity < 0.95, flag as stale.
3. **Model change detection**: Compare current `PROJECT_EMBEDDING_MODEL` with stored `embedding_model`.

### 3.3 Remediation
- **R1**: Log stale vectors to `audit_logs`. No auto-delete.
- **R2**: Schedule rebuild for stale sessions. Rebuild is full-session backfill, not incremental.

---

## 4. Targeted Delete

### 4.1 When to Delete
- Session is purged (user requests data deletion).
- Vector is tombstoned (source row marked `tombstoned=true` in `direct_evidence_records`).
- Rebuild drill: old collection is dropped after successful swap.

### 4.2 Delete Granularity
| Granularity | Chroma (0.8) | Milvus Lite (Future) |
|-------------|-------------|---------------------|
| By session  | Not implemented | `DeleteSession(ctx, sessionID)` |
| By document ID | Not implemented | `Delete(ctx, ids...)` |
| By filter | Not implemented | `Delete(ctx, filter)` |
| Full collection | Rebuild drill (swap) | `DropCollection` + recreate |

### 4.3 Safety Guard
- Delete must be idempotent.
- Delete must write to `audit_logs` before execution.
- Delete by session requires `operator_token` in R2.

---

## 5. Rebuild Parity

### 5.1 Parity Definition
Rebuild parity means:
- After a full rebuild, `Count(sessionID)` in Milvus Lite equals `COUNT(*)` in MariaDB canonical truth for that session.
- After a full rebuild, `Search(sessionID, vector, limit)` in Milvus Lite returns the same top-k documents as SQLite `LIKE` search (within Jaccard > 0.8).

### 5.2 Rebuild Steps
1. **Lock**: Mark session as `rebuilding` in `session_active_scopes`.
2. **Source**: Query all canonical truth rows for `chat_session_id` from MariaDB.
3. **Build**: Create new Milvus collection `{name}_rebuild_{timestamp}`.
4. **Upsert**: Batch upsert (25 per tier) into rebuild collection.
5. **Validate**: Run sample query sanity check (same as 0.8 health probe).
6. **Compare**: Run shadow-read compare against SQLite/MariaDB.
7. **Swap**: If parity passes, atomically rename old ? `_rollback`, rebuild ? active.
8. **Unlock**: Mark session as `active`.

### 5.3 Parity Checklist
- [ ] `Count` matches MariaDB row count per session.
- [ ] Sample query returns non-empty results.
- [ ] Top-k overlap Jaccard > 0.8.
- [ ] No duplicate IDs in new collection.
- [ ] All metadata fields present (tier, source_table, source_row_id, schema_version).

---

## 6. Live Read Switch (Explicitly Banned)

> **Rule**: Until explicit approval, `search` handler MUST use SQLite/MariaDB `LIKE` search only.
> Milvus Lite `Search` is exercised only by shadow compare probes.

### 6.1 Switch Conditions (Future)
- Divergence rate < 1% for 7 consecutive days.
- Rebuild parity passes for 100% of active sessions.
- Operator approval via `POST /milvus-shadow/adoption-gate`.
- `config.Mode` is NOT `ModeShadow` (blocked by `Validate()`).

### 6.2 Rollback Plan
- If live read switch causes regression: revert to SQLite/MariaDB within one request.
- `fakeVectorStore` is always available as emergency fallback.
- `audit_logs` records every switch attempt and rollback.

---

## 7. Trace / Audit Fields

| Event | Source | Fields |
|-------|--------|--------|
| `shadow_write_attempt` | `fakeVectorStore.Upsert` | session_id, doc_count, tier_distribution |
| `shadow_read_compare` | compare probe | session_id, jaccard, divergence_count |
| `stale_vector_detected` | re-embedding audit | session_id, doc_id, old_model, new_model, cosine_diff |
| `targeted_delete` | delete handler | session_id, doc_ids, reason, operator_token_hash |
| `rebuild_start` | rebuild handler | session_id, source_count, target_collection |
| `rebuild_parity_pass` | rebuild validator | session_id, count_match, jaccard, duration_ms |
| `rebuild_parity_fail` | rebuild validator | session_id, count_delta, jaccard, reason |
| `live_read_switch_attempt` | adoption gate | session_id, operator, approval_token_hash |
| `live_read_switch_rollback` | adoption gate | session_id, reason, fallback_duration_ms |

---

## 8. Go Handler Classification

| Route | Tier | Current Behavior | Future Behavior |
|-------|------|------------------|-----------------|
| `POST /chroma-shadow/backfill-dry-run` | R1 | `writeShadowGuard` ? 503 | Shadow write probe (R1+) |
| `POST /chroma-shadow/reembed-audit` | R1 | `writeShadowGuard` ? 503 | Stale vector detection (R1+) |
| `POST /chroma-shadow/rebuild-drill` | R2 | `writeShadowGuard` ? 503 | Rebuild parity test (R2+) |
| `POST /chroma-shadow/adoption-gate` | R2 | `writeShadowGuard` ? 503 | Live read switch approval (R2+) |
| `POST /milvus-shadow/search-compare` | R1 | **Not registered** | Shadow read compare (R1+) |

---

## 9. Verification Checklist

Before any live read switch:

- [ ] Shadow write coverage is 100% of new turns.
- [ ] Shadow read compare runs automatically every N turns.
- [ ] Stale vector detection has < 5% false positive rate.
- [ ] Targeted delete is idempotent and audited.
- [ ] Rebuild parity passes for all active sessions.
- [ ] Divergence rate is < 1% for 7 consecutive days.
- [ ] Operator approval token is validated.
- [ ] `config.Validate()` still blocks `ModeLive` and `ModeCutover`.
- [ ] H-4e release hygiene scan passes.

---

*Strategy version: R0-2026-05-22*  
*Reference: `Archive Center Beta 0.8(fix)/backend/services/chroma_shadow.py`, `contracts/milvus-lite-vector-contract.md`*
