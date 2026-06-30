# JS Adapter Route Coverage Audit — R1

> **Status**: R1 shadow evidence artifact for progress items `2.0-4k` and `2.0-4m`.
> This is **not** a product-readiness or green-gate claim. It documents which
> 0.8 JS adapter bridge routes have corresponding Go surface in 2.0 and at
> what implementation level.

## Method

- **0.8 source** (read-only reference): `Archive Center Beta 0.8(fix)/Archive Center.js`
  — all `bridgeFetch` / `bridgeFetchWithRetry` call targets extracted.
- **2.0 source**: `go-service/internal/httpapi/*.go` — all `mux.HandleFunc`
  registrations and handler implementations inspected.
- Each route family is classified by current 2.0 Go status.

## Status Legend

| Tag | Meaning |
|---|---|
| **R1-read** | Handler runs Store/Vector-backed reads; returns real evidence under R1 constraints. |
| **R1-shadow** | Handler returns shadow/plan shape; no real mutation or upstream call. |
| **R1-degraded** | Handler returns safe empty/degraded shape when Store/Vector is ErrNotEnabled. |
| **R2-guarded** | Handler returns `writeShadowGuard` 503; mutation blocked until R2 cutover. |
| **R2-plan** | Handler returns a shadow plan object (e.g. rollback_plan, repair_replay_plan); no mutations. |
| **missing** | No Go route registered; needs follow-up. |

## Route Family Coverage

| # | JS Adapter Family (0.8 bridge targets) | 2.0 Go Routes | Status |
|---|---|---|---|
| 1 | `POST /prepare-turn` | `POST /prepare-turn` | R1-shadow |
| 2 | `POST /complete-turn` | `POST /complete-turn` | R1-shadow (fake side-effects) |
| 3 | `POST /effective-inputs` | `POST /effective-inputs` | R1-shadow (save disabled in R0/R1) |
| 4 | `POST /turns/complete` | `POST /turns/complete` | R2-guarded |
| 5 | `POST /turns` | `POST /turns` | R2-guarded |
| 6 | `POST /turns/repair-replay` | `POST /turns/repair-replay` | R2-plan (shadow plan only) |
| 7 | `DELETE /rollback/{turn_index}` | `DELETE /rollback/{turn_index}` | R2-plan (shadow plan only) |
| 8 | `GET /kg/recall` | `GET /kg/recall` | R1-read (Store-backed) |
| 9 | `POST /kg/recall` | `POST /kg/recall` | R1-read (Store-backed) |
| 10 | `POST /search` | `POST /search` | R1-read (Store-backed) |
| 11 | `GET /explorer/memories` | `GET /explorer/memories` | R1-read |
| 12 | `GET /explorer/direct-evidence` | `GET /explorer/direct-evidence` | R1-read |
| 13 | `GET /explorer/kg_triples` | `GET /explorer/kg_triples` | R1-read |
| 14 | `GET /explorer/chat_logs` | `GET /explorer/chat_logs` | R1-read |
| 15 | `GET /explorer/chapter_summaries` | `GET /explorer/chapter_summaries` | R1-read |
| 16 | `PATCH /explorer/memories/{id}` | `PATCH /explorer/memories/{memory_id}` | R2-guarded |
| 17 | `PATCH /explorer/kg_triples/{id}` | `PATCH /explorer/kg_triples/{triple_id}` | R2-guarded |
| 18 | `PATCH /explorer/direct-evidence/{id}/*` | 4 PATCH routes | R2-guarded |
| 19 | `DELETE /explorer/memories/{id}` | `DELETE` + `POST …/delete` | R2-guarded |
| 20 | `DELETE /explorer/kg_triples/{id}` | `DELETE` + `POST …/delete` | R2-guarded |
| 21 | `POST /explorer/memories/regenerate` | `POST /explorer/memories/regenerate` | R2-guarded |
| 22 | `GET /storylines/{sid}` | `GET /storylines/{chat_session_id}` | R1-read |
| 23 | `POST /storylines/sync` | `POST /storylines/sync` | R2-guarded |
| 24 | `PATCH /storylines/{id}` | `PATCH /storylines/{storyline_id}` | R2-guarded |
| 25 | `PATCH /storylines/{id}/trust` | `PATCH /storylines/{storyline_id}/trust` | R2-guarded |
| 26 | `DELETE /storylines/{id}` | `DELETE /storylines/{storyline_id}` | R2-guarded |
| 27 | `GET /world-rules/{sid}` | `GET /world-rules/{chat_session_id}` | R1-read |
| 28 | `GET /world-rules/{sid}/inherited` | `GET /world-rules/{chat_session_id}/inherited` | R1-read |
| 29 | `POST /world-rules/sync` | `POST /world-rules/sync` | R2-guarded |
| 30 | `PATCH /world-rules/{id}` | `PATCH /world-rules/{rule_id}` | R2-guarded |
| 31 | `PATCH /world-rules/{id}/trust` | `PATCH /world-rules/{rule_id}/trust` | R2-guarded |
| 32 | `DELETE /world-rules/{id}` | `DELETE /world-rules/{rule_id}` | R2-guarded |
| 33 | `GET /session-state/{sid}` | `GET /session-state/{chat_session_id}` | R1-read |
| 34 | `GET /continuity-pack/{sid}` | `GET /continuity-pack/{chat_session_id}` | R1-read |
| 35 | `GET /pending-threads/{sid}` | `GET /pending-threads/{chat_session_id}` | R1-read |
| 36 | `PATCH /pending-threads/{id}` | `PATCH /pending-threads/{hook_id}` | R2-guarded |
| 37 | `PATCH /pending-threads/{id}/trust` | `PATCH /pending-threads/{hook_id}/trust` | R2-guarded |
| 38 | `DELETE /pending-threads/{id}` | `DELETE /pending-threads/{hook_id}` | R2-guarded |
| 39 | `GET /active-states/{sid}` | `GET /active-states/{chat_session_id}` | R1-read |
| 40 | `GET /narrative-control/{sid}` | `GET /narrative-control/{chat_session_id}` | R1-read |
| 41 | `PATCH /narrative-control/{sid}/director-patch` | `PATCH /narrative-control/{chat_session_id}/director-patch` | R2-guarded |
| 42 | `GET /momentum-packet/{sid}` | `GET /momentum-packet/{chat_session_id}` | R1-read |
| 43 | `GET /characters/{sid}` | `GET /characters/{chat_session_id}` | R1-read |
| 44 | `PATCH /characters/{sid}/{name}` | `PATCH /characters/{chat_session_id}/{character_name}` | R2-guarded |
| 45 | `PATCH /characters/{sid}/{name}/speech` | `PATCH /characters/{sid}/{name}/speech` | R2-guarded |
| 46 | `DELETE /characters/{sid}/{name}` | `DELETE /characters/{chat_session_id}/{character_name}` | R2-guarded |
| 47 | `GET /episodes/{sid}` | `GET /episodes/{chat_session_id}` | R1-read |
| 48 | `POST /episodes/generate` | `POST /episodes/generate` | R2-guarded |
| 49 | `POST /episodes/search` | `POST /episodes/search` | R1-read (Store-backed) |
| 50 | `POST /episodes/regenerate` | `POST /episodes/regenerate` | R2-guarded |
| 51 | `POST /episodes/merge` | `POST /episodes/merge` | R2-guarded |
| 52 | `POST /chapters/generate` | `POST /chapters/generate` | R2-guarded |
| 53 | `POST /chapters/dry-run` | `POST /chapters/dry-run` | R1-read (Store-backed) |
| 54 | `POST /chapters/search` | `POST /chapters/search` | R1-read (Store-backed) |
| 55 | `PATCH /episodes/{id}` | `PATCH /episodes/{episode_id}` | R2-guarded |
| 56 | `DELETE /episodes/{id}` | `DELETE /episodes/{episode_id}` | R2-guarded |
| 57 | `POST /arcs/generate` | `POST /arcs/generate` | R2-guarded |
| 58 | `POST /sagas/generate` | `POST /sagas/generate` | R2-guarded |
| 59 | `POST /admin/rescan` | `POST /admin/rescan` | R2-guarded |
| 60 | `POST /admin/reindex` | `POST /admin/reindex` | R2-guarded |
| 61 | `POST /admin/session-migrate` | `POST /admin/session-migrate` | R2-guarded |
| 62 | `POST /proxy/plugin-main` | `POST /proxy/plugin-main` | R1-shadow (503, endpoint validated) |
| 63 | `POST /supervisor` | `POST /supervisor` | R1-shadow (prompt trace, no real LLM call) |
| 64 | `POST /critic/test` | `POST /critic/test` | R1-shadow (prompt trace, no real LLM call) |
| 65 | `GET /prompts/{name}` | `GET /prompts/{prompt_name}` | R1-read (filesystem) |
| 66 | `PUT /prompts/{name}` | `PUT /prompts/{prompt_name}` | R2-guarded |
| 67 | `POST /config/update` | `POST /config/update` | R2-guarded |
| 68 | `GET /health` | `GET /health` | R1-read |
| 69 | `GET /stats` | `GET /stats` | R1-read |
| 70 | `GET /wakeup` | `GET /wakeup` | R1-read |
| 71 | `GET /sessions` | `GET /sessions` | R1-read |
| 72 | `GET /sessions/{sid}/guidance-snapshot` | `GET /sessions/{sid}/guidance-snapshot` | R1-read |
| 73 | `GET /sessions/compare` | `GET /sessions/compare` | R1-read |
| 74 | `POST /feedback` | `POST /feedback` | R2-guarded |
| 75 | `POST /import/hypamemory` | `POST /import/hypamemory` | R2-guarded |
| 76 | `GET /metrics/lc1q/{sid}` | `GET /metrics/lc1q/{chat_session_id}` | R1-read |
| 77 | `GET /metrics/lc1r/regression-corpus` | `GET /metrics/lc1r/regression-corpus` | R1-read |
| 78 | `GET /metrics/lc1s/step17-bundle-closure` | `GET /metrics/lc1s/step17-bundle-closure` | R1-read |
| 79 | `POST /chroma-shadow/*` (6 R1 probes) | 6 POST routes | R1-read (Store/Vector-backed evidence) |
| 80 | `POST /chroma-shadow/*` (4 R2 routes) | 4 POST routes | R2-guarded |
| 81 | `POST /milvus-shadow/*` (2 drill routes) | 2 POST routes | R1-shadow/R1-degraded |

## 2.0-Only Routes (no 0.8 JS bridge target)

These Go routes have no direct 0.8 JS adapter bridge call. They are 2.0
additions for operational depth:

- Canonical CRUD: `GET/POST /canonical/{sid}/{table}` (16 routes)
- `GET /retrieval-index/{sid}`, `GET /retrieval-index/{sid}/source-row`
- `GET /intent-routing/runtime-config`, `GET /retrieval-index/runtime-config`
- `GET /canonical-state-layer/{sid}`, `GET /session/{sid}/active-scope`
- `GET /characters/{sid}/{name}/events`, `GET /characters/{sid}/{name}`
- `GET /episodes/detail/{id}`, `GET /long-session-health/{sid}`
- `GET /session/{sid}/resume-pack`, `GET /session/{sid}/step7-health`
- `GET /sessions/{sid}/export`, `GET /audit`, `GET /feedback/latest`
- `GET /metrics/lc1{c-p}/{sid}`, `GET /metrics/tm1d/{sid}`
- `GET /maintenance/queue-status`, `POST /maintenance/enqueue`, `POST /maintenance-pass/{sid}`
- `GET /ready`, `GET /version`

## Biggest Remaining Gaps (0.8 JS Adapter Perspective)

1. **Rollback/repair-replay mutation remains R2.** Both `DELETE /rollback/{turn_index}` and
   `POST /turns/repair-replay` return shadow plans only; no actual turn deletion or
   state rewind occurs in the Go service.

2. **Trust PATCH/DELETE surfaces are guarded.** All `/trust` PATCH routes
   (`storylines`, `world-rules`, `pending-threads`) and DELETE routes return 503.
   The 0.8 JS adapter calls these from settings panels and inline trust controls.

3. **Episode/chapter generation remains guarded.** `POST /episodes/generate`,
   `POST /chapters/generate`, `POST /arcs/generate`, `POST /sagas/generate`,
   `POST /episodes/regenerate`, `POST /episodes/merge` all return 503. The 0.8
   adapter triggers generation after turn completion and from explorer actions.

4. **Real live supervisor/critic remains disabled.** `POST /supervisor` and
   `POST /critic/test` return prompt-assembly traces and evidence counts but never
   call an upstream LLM. The 0.8 adapter depends on these for per-turn guidance
   and quality feedback.

5. **Real MariaDB-backed smoke/default switch still open.** All Store reads
   gracefully degrade to empty shapes on `ErrNotEnabled`, but no config flag or
   migration step currently enables live MariaDB as the default Store
   implementation. The `MariaDBEnabled` path exists in config but has no
   R1 verification against a running instance.

6. **Actual Milvus live cutover still open.** `milvus_live_enabled` is always
   `false` in HTTP surfaces. The `MilvusSDKEnabled` config flag exists but no
   R1 test verifies end-to-end vector search against a real Milvus instance.
   `milvus-shadow/*` drill endpoints exercise the boundary but never flip
   the switch.

7. **Admin operations are guarded.** `POST /admin/rescan`, `/admin/reindex`,
   `/admin/session-migrate` return 503. The 0.8 adapter calls these from
   maintenance/debug panels.

8. **Proxy passthrough is blocked.** `POST /proxy/plugin-main` returns 503
   with validated endpoint. The 0.8 adapter uses this as the backend bundle
   entry point for coprocessor orchestration.

## Summary Counts

| Category | Count |
|---|---|
| R1-read (Store/Vector-backed) | ~30 |
| R1-shadow (plan/trace, no real effect) | ~8 |
| R2-guarded (503 blocked mutation) | ~30 |
| R2-plan (shadow plan, no mutation) | 2 |
| 2.0-only additions (no 0.8 bridge target) | ~25 |

**Bottom line**: All major 0.8 JS adapter bridge route families have a
corresponding Go registration in 2.0. Read-surface coverage is substantive
(Store-backed R1 evidence). This does not prove byte-for-byte runtime
compatibility for every query-string and trailing-slash variant; exact request
smoke remains required before `2.0-4k` can be green. The structural gap is
concentrated in write/mutation surfaces: ~30 R2-guarded routes need cutover
decisions before 0.8->2.0 behavioral parity can be claimed. No
product-readiness claim is made.
