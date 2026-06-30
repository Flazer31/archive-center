# Current Route Inventory

> Source of truth: `Archive Center Beta 0.8(fix)\backend\main.py` and `backend\routes`
> Date: 2026-05-21
> Status: R0 inventory; not a migration plan, only a classification.

## Method

Routes were extracted via static inspection of `@app.<method>("...")` decorators in `main.py` plus `routes/proxy_plugin_main.py` and `routes/config_update.py`.

## Tier Definitions

| Tier | Description |
|------|-------------|
| public/readiness | Exposed to external health checks, operators, or orchestrators. No auth required in local mode. |
| plugin-local runtime | Called by the Archive Center.js host adapter or local plugins. Tight coupling to current transport. |
| admin-control | Maintenance, reindex, session migration, queue management. Operator-only. |
| provider-proxy | Calls to external LLM/provider surfaces (supervisor, proxy plugin). |
| debug-test | Critic tests, diagnostic endpoints. Not for production traffic. |
| explorer/data | CRUD and search over memories, evidence, KG triples, summaries. |
| narrative/domain | Episodes, chapters, arcs, sagas, storylines, characters, world rules, pending threads. |
| metrics/diagnostic | LC1* bundle metrics, momentum, step17 telemetry. |
| session/utility | Session export, guidance, scope, state, continuity pack. |
| turn/core | Prepare-turn and complete-turn; the primary user-facing surface. |
| retrieval/config | Search, retrieval index, intent routing, KG recall. |
| chroma-shadow | Chroma shadow backfill, audit, hygiene, rebuild. Migration-specific lane. |
| prompts | Prompt store read/write. |
| audit/feedback | Audit logs and critic feedback. |
| import | External data import (hypamemory). |

## Route Inventory Table

| # | Method | Path | Tier | Migration Owner | Notes / Blockers |
|---|--------|------|------|-----------------|------------------|
| 1 | GET | /health | public/readiness | @kimi_coder | Minimal; Go skeleton already provides equivalent. |
| 2 | GET | /wakeup | public/readiness | @kimi_coder | Simple wakeup probe. |
| 3 | GET | /ready | public/readiness | @kimi_coder | **New in Go skeleton**; readiness probe with dependency checks. |
| 4 | GET | /version | public/readiness | @kimi_coder | **New in Go skeleton**; build metadata. |
| 5 | POST | /turns | turn/core | @kimi_coder | Legacy turn ingestion. |
| 6 | POST | /turns/repair-replay | turn/core | @kimi_coder | Repair replay logic. |
| 7 | POST | /turns/complete | turn/core | @kimi_coder | M4 complete-turn response model. |
| 8 | POST | /complete-turn | turn/core | @kimi_coder | Primary complete-turn with `M4CompleteTurnResponse`. |
| 9 | POST | /prepare-turn | turn/core | @kimi_coder | Primary prepare-turn surface. |
| 10 | POST | /effective-inputs | turn/core | @kimi_coder | Effective input logging. |
| 11 | DELETE | /rollback/{turn_index} | turn/core | @kimi_coder | Rollback by turn index. |
| 12 | POST | /search | retrieval/config | @kimi_coder | General search endpoint. |
| 13 | GET | /retrieval-index/runtime-config | retrieval/config | @kimi_coder | Runtime config read. |
| 14 | POST | /retrieval-index/runtime-config | retrieval/config | @kimi_coder | Runtime config write. |
| 15 | GET | /retrieval-index/{chat_session_id} | retrieval/config | @kimi_coder | Per-session retrieval index. |
| 16 | GET | /retrieval-index/{chat_session_id}/source-row | retrieval/config | @kimi_coder | Source row detail. |
| 17 | GET | /intent-routing/runtime-config | retrieval/config | @kimi_coder | Intent routing config read. |
| 18 | POST | /intent-routing/runtime-config | retrieval/config | @kimi_coder | Intent routing config write. |
| 19 | POST | /kg/recall | retrieval/config | @kimi_coder | KG recall (POST). |
| 20 | GET | /kg/recall | retrieval/config | @kimi_coder | KG recall (GET). |
| 21 | GET | /chroma-shadow/preflight | chroma-shadow | @kimi_coder | Shadow preflight check. |
| 22 | POST | /chroma-shadow/bootstrap | chroma-shadow | @kimi_coder | Bootstrap shadow collection. |
| 23 | POST | /chroma-shadow/backfill-dry-run | chroma-shadow | @kimi_coder | Dry-run backfill. |
| 24 | POST | /chroma-shadow/backfill-batch | chroma-shadow | @kimi_coder | Batch backfill. |
| 25 | POST | /chroma-shadow/reembed-audit | chroma-shadow | @kimi_coder | Re-embed audit. |
| 26 | POST | /chroma-shadow/health-probe | chroma-shadow | @kimi_coder | Shadow health probe. |
| 27 | POST | /chroma-shadow/fallback-runbook | chroma-shadow | @kimi_coder | Fallback runbook. |
| 28 | POST | /chroma-shadow/rebuild-drill | chroma-shadow | @kimi_coder | Rebuild drill. |
| 29 | POST | /chroma-shadow/adoption-gate | chroma-shadow | @kimi_coder | Adoption gate. |
| 30 | POST | /chroma-shadow/release-hygiene | chroma-shadow | @kimi_coder | Release hygiene. |
| 31 | POST | /chroma-shadow/visibility-guard | chroma-shadow | @kimi_coder | Visibility guard. |
| 32 | POST | /critic/test | debug-test | @glm_subcoder | Critic test endpoint. |
| 33 | POST | /supervisor | provider-proxy | @kimi_coder | Supervisor/proxy bridge. |
| 34 | GET | /prompts | prompts | @glm_subcoder | List prompts. |
| 35 | GET | /prompts/{prompt_name} | prompts | @glm_subcoder | Read prompt. |
| 36 | PUT | /prompts/{prompt_name} | prompts | @glm_subcoder | Write prompt. |
| 37 | GET | /stats | admin-control | @glm_subcoder | Basic stats. |
| 38 | GET | /sessions | session/utility | @kimi_coder | List sessions. |
| 39 | GET | /explorer/chat_logs | explorer/data | @kimi_coder | Chat log explorer. |
| 40 | GET | /explorer/memories | explorer/data | @kimi_coder | Memory explorer. |
| 41 | GET | /explorer/direct-evidence | explorer/data | @kimi_coder | Direct evidence explorer. |
| 42 | GET | /explorer/kg_triples | explorer/data | @kimi_coder | KG triple explorer. |
| 43 | GET | /explorer/chapter_summaries | explorer/data | @kimi_coder | Chapter summary explorer. |
| 44 | PATCH | /explorer/memories/{memory_id} | explorer/data | @kimi_coder | Patch memory. |
| 45 | PATCH | /explorer/kg_triples/{triple_id} | explorer/data | @kimi_coder | Patch KG triple. |
| 46 | PATCH | /explorer/direct-evidence/{record_id}/review | explorer/data | @kimi_coder | Review evidence. |
| 47 | PATCH | /explorer/direct-evidence/{record_id}/revalidate | explorer/data | @kimi_coder | Revalidate evidence. |
| 48 | PATCH | /explorer/direct-evidence/{record_id}/tombstone | explorer/data | @kimi_coder | Tombstone evidence. |
| 49 | PATCH | /explorer/direct-evidence/{record_id}/supersede | explorer/data | @kimi_coder | Supersede evidence. |
| 50 | POST | /explorer/memories/regenerate | explorer/data | @kimi_coder | Regenerate memory embedding. |
| 51 | DELETE | /explorer/memories/{memory_id} | explorer/data | @kimi_coder | Delete memory. |
| 52 | POST | /explorer/memories/{memory_id}/delete | explorer/data | @kimi_coder | Soft-delete memory. |
| 53 | DELETE | /explorer/kg_triples/{triple_id} | explorer/data | @kimi_coder | Delete KG triple. |
| 54 | POST | /explorer/kg_triples/{triple_id}/delete | explorer/data | @kimi_coder | Soft-delete KG triple. |
| 55 | GET | /sessions/{chat_session_id}/export | session/utility | @kimi_coder | Session export. |
| 56 | GET | /sessions/{chat_session_id}/guidance-snapshot | session/utility | @kimi_coder | Guidance snapshot. |
| 57 | GET | /sessions/{chat_session_id}/step7-health | session/utility | @kimi_coder | Step7 health check. |
| 58 | POST | /admin/reindex | admin-control | @glm_subcoder | Reindex operation. |
| 59 | POST | /admin/rescan | admin-control | @glm_subcoder | Rescan operation. |
| 60 | POST | /admin/session-migrate | admin-control | @glm_subcoder | Session migration. |
| 61 | POST | /import/hypamemory | import | @kimi_coder | Import hypamemory data. |
| 62 | GET | /audit | audit/feedback | @glm_subcoder | Audit log query. |
| 63 | POST | /feedback | audit/feedback | @glm_subcoder | Submit feedback. |
| 64 | GET | /feedback/latest | audit/feedback | @glm_subcoder | Latest feedback. |
| 65 | GET | /sessions/compare | session/utility | @kimi_coder | Session compare view. |
| 66 | GET | /active-states/{chat_session_id} | session/utility | @kimi_coder | Active states. |
| 67 | GET | /canonical-state-layer/{chat_session_id} | session/utility | @kimi_coder | Canonical state layer. |
| 68 | GET | /episodes/{chat_session_id} | narrative/domain | @kimi_coder | List episodes. |
| 69 | POST | /episodes/generate | narrative/domain | @kimi_coder | Generate episode. |
| 70 | GET | /episodes/detail/{episode_id} | narrative/domain | @kimi_coder | Episode detail. |
| 71 | PATCH | /episodes/{episode_id} | narrative/domain | @kimi_coder | Patch episode. |
| 72 | DELETE | /episodes/{episode_id} | narrative/domain | @kimi_coder | Delete episode. |
| 73 | POST | /episodes/regenerate | narrative/domain | @kimi_coder | Regenerate episode. |
| 74 | POST | /episodes/merge | narrative/domain | @kimi_coder | Merge episodes. |
| 75 | POST | /episodes/search | narrative/domain | @kimi_coder | Search episodes. |
| 76 | POST | /chapters/generate | narrative/domain | @kimi_coder | Generate chapter. |
| 77 | POST | /chapters/dry-run | narrative/domain | @kimi_coder | Chapter dry-run. |
| 78 | POST | /chapters/search | narrative/domain | @kimi_coder | Search chapters. |
| 79 | POST | /arcs/generate | narrative/domain | @kimi_coder | Generate arc. |
| 80 | POST | /sagas/generate | narrative/domain | @kimi_coder | Generate saga. |
| 81 | GET | /storylines/{chat_session_id} | narrative/domain | @kimi_coder | List storylines. |
| 82 | PATCH | /storylines/{storyline_id} | narrative/domain | @kimi_coder | Patch storyline. |
| 83 | PATCH | /storylines/{storyline_id}/trust | narrative/domain | @kimi_coder | Trust storyline. |
| 84 | DELETE | /storylines/{storyline_id} | narrative/domain | @kimi_coder | Delete storyline. |
| 85 | POST | /storylines/sync | narrative/domain | @kimi_coder | Sync storylines. |
| 86 | GET | /characters/{chat_session_id} | narrative/domain | @kimi_coder | List characters. |
| 87 | GET | /characters/{chat_session_id}/{character_name} | narrative/domain | @kimi_coder | Character detail. |
| 88 | GET | /characters/{chat_session_id}/{character_name}/events | narrative/domain | @kimi_coder | Character events. |
| 89 | PATCH | /characters/{chat_session_id}/{character_name} | narrative/domain | @kimi_coder | Patch character. |
| 90 | PATCH | /characters/{chat_session_id}/{character_name}/speech | narrative/domain | @kimi_coder | Patch character speech. |
| 91 | DELETE | /characters/{chat_session_id}/{character_name} | narrative/domain | @kimi_coder | Delete character. |
| 92 | GET | /world-rules/{chat_session_id} | narrative/domain | @kimi_coder | List world rules. |
| 93 | GET | /world-rules/{chat_session_id}/inherited | narrative/domain | @kimi_coder | Inherited world rules. |
| 94 | POST | /world-rules/sync | narrative/domain | @kimi_coder | Sync world rules. |
| 95 | PATCH | /world-rules/{rule_id} | narrative/domain | @kimi_coder | Patch world rule. |
| 96 | PATCH | /world-rules/{rule_id}/trust | narrative/domain | @kimi_coder | Trust world rule. |
| 97 | DELETE | /world-rules/{rule_id} | narrative/domain | @kimi_coder | Delete world rule. |
| 98 | GET | /pending-threads/{chat_session_id} | narrative/domain | @kimi_coder | List pending threads. |
| 99 | PATCH | /pending-threads/{hook_id} | narrative/domain | @kimi_coder | Patch pending thread. |
| 100 | PATCH | /pending-threads/{hook_id}/trust | narrative/domain | @kimi_coder | Trust pending thread. |
| 101 | DELETE | /pending-threads/{hook_id} | narrative/domain | @kimi_coder | Delete pending thread. |
| 102 | GET | /continuity-pack/{chat_session_id} | session/utility | @kimi_coder | Continuity pack. |
| 103 | GET | /session-state/{chat_session_id} | session/utility | @kimi_coder | Full session state. |
| 104 | GET | /session/{chat_session_id}/active-scope | session/utility | @kimi_coder | Active scope. |
| 105 | PATCH | /session/{chat_session_id}/active-scope | session/utility | @kimi_coder | Patch active scope. |
| 106 | GET | /metrics/lc1c/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1C memory footprint. |
| 107 | GET | /metrics/lc1d/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1D integrity replay. |
| 108 | GET | /metrics/lc1e/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1E context budget. |
| 109 | GET | /metrics/lc1f/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1F non-regression. |
| 110 | GET | /metrics/lc1g/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1G accuracy replay. |
| 111 | GET | /metrics/lc1h/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1H forgetting/hallucination. |
| 112 | GET | /metrics/lc1i/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1I ablation compare. |
| 113 | GET | /metrics/lc1j/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1J imported idea gate. |
| 114 | GET | /metrics/lc1k/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1K priority budget hybrid. |
| 115 | GET | /metrics/lc1l/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1L imported idea contract. |
| 116 | GET | /metrics/lc1m/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1M single vs split. |
| 117 | GET | /metrics/lc1n/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1N rebuild backfill rehearsal. |
| 118 | GET | /metrics/lc1o/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1O preview ledger. |
| 119 | GET | /metrics/lc1p/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1P completeness. |
| 120 | GET | /metrics/lc1q/{chat_session_id} | metrics/diagnostic | @glm_subcoder | LC1Q freshness lag. |
| 121 | GET | /metrics/lc1r/regression-corpus | metrics/diagnostic | @glm_subcoder | LC1R regression corpus. |
| 122 | GET | /metrics/lc1s/step17-bundle-closure | metrics/diagnostic | @glm_subcoder | LC1S step17 bundle closure. |
| 123 | GET | /metrics/tm1d/{chat_session_id} | metrics/diagnostic | @glm_subcoder | TM1D momentum. |
| 124 | GET | /momentum-packet/{chat_session_id} | metrics/diagnostic | @glm_subcoder | Momentum packet. |
| 125 | GET | /narrative-control/{chat_session_id} | narrative/domain | @kimi_coder | Narrative control read. |
| 126 | PATCH | /narrative-control/{chat_session_id}/director-patch | narrative/domain | @kimi_coder | Director patch. |
| 127 | POST | /maintenance/enqueue | admin-control | @glm_subcoder | Maintenance enqueue. |
| 128 | GET | /maintenance/queue-status | admin-control | @glm_subcoder | Maintenance queue status. |
| 129 | POST | /maintenance-pass/{chat_session_id} | admin-control | @glm_subcoder | Maintenance pass. |
| 130 | GET | /long-session-health/{session_id} | admin-control | @glm_subcoder | Long-session health. |
| 131 | POST | /proxy/plugin-main | plugin-local runtime | @kimi_coder | Proxy plugin main (router). |
| 132 | POST | /config/update | plugin-local runtime | @glm_subcoder | Config update (router). |

## Unknown / Blocker Summary

| Item | Status | Detail |
|------|--------|--------|
| FastAPI/Pydantic models | blocker | Many routes depend on typed request/response models. Go migration needs explicit schema contracts first. |
| SQLAlchemy session lifecycle | blocker | Turn and explorer routes assume SQLAlchemy `SessionLocal`. MariaDB driver + sql migration must precede parity. |
| ChromaDB client lifecycle | blocker | Shadow and retrieval routes depend on Chroma `Collection` objects. Milvus Lite client contract not yet defined. |
| ArchiveBridge / PalaceBridge | blocker | Narrative and session routes use Python bridge objects with complex state. These are not directly port-able without interface redesign. |
| `turn_contracts.py` packet shapes | blocker | `PrepareTurnRequest`, `M4CompleteTurnResponse`, etc. are deeply nested. JSON schema extraction is a prerequisite. |
| `retrieval_document_builder.py` | blocker | Retrieval document schema and provenance tracking must be preserved. |
| `backend/services/*` workers | unknown | Many services (lc1 builders, step17 metrics, chroma shadow) have heavy logic. Shadow replication vs rewrite decision is pending. |
| `backend/roles/*` | unknown | Critic, Librarian, Storyteller, Supervisor are LLM-agent roles. Their orchestration in Go is undefined. |

## Tier Counts

| Tier | Count |
|------|-------|
| public/readiness | 4 |
| turn/core | 7 |
| retrieval/config | 10 |
| chroma-shadow | 11 |
| debug-test | 1 |
| provider-proxy | 1 |
| prompts | 3 |
| admin-control | 7 |
| explorer/data | 16 |
| session/utility | 11 |
| narrative/domain | 35 |
| metrics/diagnostic | 19 |
| audit/feedback | 3 |
| import | 1 |
| plugin-local runtime | 2 |
| **Total** | **132** |
> **Footnote**: The inventory lists 132 rows, of which `/ready` and `/version` are new readiness probes introduced in the Go skeleton and do not exist in the 0.8 Python backend. The actual 0.8 OpenAPI operations extracted from `backend/main.py` are **130**.

