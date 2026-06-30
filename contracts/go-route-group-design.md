# Go Route Group Design

> Source of truth: `Archive Center Beta 0.8(fix)\backend\main.py` and `backend\routes`
> Cross-reference: `contracts/current-route-inventory.md`, `contracts/go-route-dto-map.md`
> Date: 2026-05-21
> Status: R0 design document; not a migration plan, only a phased-risk classification.
> Note: `/ready` and `/version` are Go-skeleton-only probes not present in the 0.8 FastAPI backend.

## Phase Definitions

| Phase | Scope | Authority Requirement |
|-------|-------|----------------------|
| **R0** | Contracts, scaffolding, safe probes, read-only health checks. No DB write, no external call, no side effect. | None |
| **R1** | Shadow-safe read or idempotent write with compare capability. Can run in parallel with 0.8 backend without data loss. | Shadow mode only |
| **R2** | Live authority required. Destructive write, external provider call, session migration, batch vector backfill, turn completion, or deletion. | Explicit cutover approval |

## Group 1 — Health / Config / Prompts

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | GET | `/health` | health | - | - | **R0** | probe |
| 2 | GET | `/long-session-health/{session_id}` | get_long_session_health | - | - | **R1** | read-only |
| 3 | GET | `/prompts` | prompt_list | - | - | **R1** | read-only |
| 4 | GET | `/prompts/{prompt_name}` | prompt_get | - | - | **R1** | read-only |
| 5 | GET | `/ready` | handle_ready | - | - | **R0** | probe |
| 6 | GET | `/stats` | db_stats | - | - | **R1** | read-only |
| 7 | GET | `/version` | handle_version | - | - | **R0** | probe |
| 8 | GET | `/wakeup` | wakeup | - | - | **R0** | probe |
| 9 | POST | `/config/update` | config_update | Request | - | **R2** | mutating |
| 10 | PUT | `/prompts/{prompt_name}` | prompt_update | PromptUpdateRequest | - | **R2** | mutating |
**Group 1 Summary:** R0=4, R1=4, R2=2

---
## Group 2 — Prepare / Complete Turn

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | DELETE | `/rollback/{turn_index}` | rollback | - | - | **R2** | mutating |
| 2 | POST | `/complete-turn` | complete_turn_m4 | M4CompleteTurnRequest | M4CompleteTurnResponse | **R2** | mutating |
| 3 | POST | `/effective-inputs` | save_effective_input | SaveEffectiveInputRequest | - | **R2** | mutating |
| 4 | POST | `/prepare-turn` | prepare_turn | PrepareTurnRequest | - | **R2** | mutating |
| 5 | POST | `/turns` | save_turn | SaveTurnRequest | - | **R2** | mutating |
| 6 | POST | `/turns/complete` | complete_turn | CompleteTurnRequest | - | **R2** | mutating |
| 7 | POST | `/turns/repair-replay` | repair_turn_replay | ChatLogRepairReplayRequest | - | **R2** | mutating |
**Group 2 Summary:** R0=0, R1=0, R2=7

---
## Group 3 — Memory / Search / Retrieval / Chroma Shadow

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | DELETE | `/explorer/kg_triples/{triple_id}` | delete_kg_triple | - | - | **R2** | mutating |
| 2 | DELETE | `/explorer/memories/{memory_id}` | delete_memory | - | - | **R2** | mutating |
| 3 | GET | `/chroma-shadow/preflight` | get_chroma_shadow_preflight | - | - | **R0** | probe |
| 4 | GET | `/explorer/chapter_summaries` | get_chapter_summaries | - | - | **R2** | mutating |
| 5 | GET | `/explorer/chat_logs` | get_chat_logs | - | - | **R2** | mutating |
| 6 | GET | `/explorer/direct-evidence` | get_direct_evidence | - | - | **R2** | mutating |
| 7 | GET | `/explorer/kg_triples` | get_kg_triples | - | - | **R2** | mutating |
| 8 | GET | `/explorer/memories` | get_memories | - | - | **R2** | mutating |
| 9 | GET | `/intent-routing/runtime-config` | get_intent_routing_runtime_config_api | - | - | **R1** | read-only |
| 10 | GET | `/kg/recall` | kg_recall_get | - | - | **R1** | read-only |
| 11 | GET | `/retrieval-index/runtime-config` | get_retrieval_index_runtime_config | - | - | **R1** | read-only |
| 12 | GET | `/retrieval-index/{chat_session_id}` | get_retrieval_index_snapshot | - | - | **R1** | read-only |
| 13 | GET | `/retrieval-index/{chat_session_id}/source-row` | get_retrieval_index_source_row | - | - | **R1** | read-only |
| 14 | PATCH | `/explorer/direct-evidence/{record_id}/revalidate` | patch_direct_evidence_revalidate | DirectEvidenceRevalidateRequest | - | **R2** | mutating |
| 15 | PATCH | `/explorer/direct-evidence/{record_id}/review` | patch_direct_evidence_review | PatchDirectEvidenceReviewRequest | - | **R2** | mutating |
| 16 | PATCH | `/explorer/direct-evidence/{record_id}/supersede` | patch_direct_evidence_supersede | PatchDirectEvidenceSupersedeRequest | - | **R2** | mutating |
| 17 | PATCH | `/explorer/direct-evidence/{record_id}/tombstone` | patch_direct_evidence_tombstone | PatchDirectEvidenceTombstoneRequest | - | **R2** | mutating |
| 18 | PATCH | `/explorer/kg_triples/{triple_id}` | patch_kg_triple | PatchKGTripleRequest | - | **R2** | mutating |
| 19 | PATCH | `/explorer/memories/{memory_id}` | patch_memory | PatchMemoryRequest | - | **R2** | mutating |
| 20 | POST | `/chroma-shadow/adoption-gate` | post_chroma_shadow_adoption_gate | ChromaShadowAdoptionGateRequest | - | **R2** | chroma shadow op |
| 21 | POST | `/chroma-shadow/backfill-batch` | post_chroma_shadow_backfill_batch | ChromaShadowBackfillBatchRequest | - | **R2** | chroma shadow op |
| 22 | POST | `/chroma-shadow/backfill-dry-run` | post_chroma_shadow_backfill_dry_run | ChromaShadowBackfillDryRunRequest | - | **R1** | chroma shadow op |
| 23 | POST | `/chroma-shadow/bootstrap` | post_chroma_shadow_bootstrap | - | - | **R2** | chroma shadow op |
| 24 | POST | `/chroma-shadow/fallback-runbook` | post_chroma_shadow_fallback_runbook | ChromaShadowFallbackRunbookRequest | - | **R1** | chroma shadow op |
| 25 | POST | `/chroma-shadow/health-probe` | post_chroma_shadow_health_probe | ChromaShadowHealthProbeRequest | - | **R1** | health probe; reads vector state |
| 26 | POST | `/chroma-shadow/rebuild-drill` | post_chroma_shadow_rebuild_drill | ChromaShadowRebuildDrillRequest | - | **R2** | chroma shadow op |
| 27 | POST | `/chroma-shadow/reembed-audit` | post_chroma_shadow_reembed_audit | ChromaShadowReembedAuditRequest | - | **R1** | chroma shadow op |
| 28 | POST | `/chroma-shadow/release-hygiene` | post_chroma_shadow_release_hygiene | ChromaShadowReleaseHygieneRequest | - | **R1** | chroma shadow op |
| 29 | POST | `/chroma-shadow/visibility-guard` | post_chroma_shadow_visibility_guard | ChromaShadowVisibilityGuardRequest | - | **R1** | chroma shadow op |
| 30 | POST | `/explorer/kg_triples/{triple_id}/delete` | delete_kg_triple_via_post | - | - | **R2** | mutating |
| 31 | POST | `/explorer/memories/regenerate` | regenerate_memory | - | - | **R2** | mutating |
| 32 | POST | `/explorer/memories/{memory_id}/delete` | delete_memory_via_post | - | - | **R2** | mutating |
| 33 | POST | `/intent-routing/runtime-config` | update_intent_routing_runtime_config | IntentRoutingRuntimeConfigRequest | - | **R2** | mutating |
| 34 | POST | `/kg/recall` | kg_recall | KGRecallRequest | - | **R2** | mutating |
| 35 | POST | `/retrieval-index/runtime-config` | update_retrieval_index_runtime_config | RetrievalIndexRuntimeConfigRequest | - | **R2** | mutating |
| 36 | POST | `/search` | search | SearchRequest | - | **R1** | mutating |
**Group 3 Summary:** R0=1, R1=12, R2=23

---
## Group 4 — Proxy / Provider

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | POST | `/critic/test` | critic_test | CriticTestRequest | - | **R1** | mutating |
| 2 | POST | `/proxy/plugin-main` | proxy_plugin_main | ProxyPluginMainRequest | - | **R2** | mutating |
| 3 | POST | `/supervisor` | supervisor_directive | SupervisorRequest | - | **R2** | mutating |
**Group 4 Summary:** R0=0, R1=1, R2=2

---
## Group 5 — Admin / Maintenance

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | GET | `/maintenance/queue-status` | get_maintenance_queue_status | - | - | **R1** | read-only |
| 2 | POST | `/admin/reindex` | admin_reindex | ReindexRequest | - | **R2** | mutating |
| 3 | POST | `/admin/rescan` | admin_rescan | RescanRequest | - | **R2** | mutating |
| 4 | POST | `/admin/session-migrate` | admin_session_migrate | SessionMigrateRequest | - | **R2** | mutating |
| 5 | POST | `/maintenance-pass/{chat_session_id}` | run_maintenance_pass | MaintenancePassRequest | - | **R2** | mutating |
| 6 | POST | `/maintenance/enqueue` | enqueue_maintenance_job | MaintenanceEnqueueRequest | MaintenanceEnqueueResponse | **R2** | mutating |
**Group 5 Summary:** R0=0, R1=1, R2=5

---
## Group 6 — Export / Import / Debug / Metrics / Narrative / Session

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Phase | Rationale |
|---|--------|------|------|-----------------|-------------------|-------|-----------|
| 1 | DELETE | `/characters/{chat_session_id}/{character_name}` | delete_character | - | - | **R2** | mutating |
| 2 | DELETE | `/episodes/{episode_id}` | delete_episode | - | - | **R2** | mutating |
| 3 | DELETE | `/pending-threads/{hook_id}` | delete_continuity_hook | - | - | **R2** | mutating |
| 4 | DELETE | `/storylines/{storyline_id}` | delete_storyline | - | - | **R2** | mutating |
| 5 | DELETE | `/world-rules/{rule_id}` | delete_world_rule | - | - | **R2** | mutating |
| 6 | GET | `/active-states/{chat_session_id}` | get_active_states | - | - | **R1** | read-only |
| 7 | GET | `/audit` | get_audit_logs | - | - | **R1** | read-only |
| 8 | GET | `/canonical-state-layer/{chat_session_id}` | get_canonical_state_layer | - | - | **R1** | read-only |
| 9 | GET | `/characters/{chat_session_id}` | get_characters | - | - | **R1** | read-only |
| 10 | GET | `/characters/{chat_session_id}/{character_name}` | get_character_detail | - | - | **R1** | read-only |
| 11 | GET | `/characters/{chat_session_id}/{character_name}/events` | get_character_events | - | - | **R1** | read-only |
| 12 | GET | `/continuity-pack/{chat_session_id}` | get_continuity_pack | - | - | **R1** | read-only |
| 13 | GET | `/episodes/detail/{episode_id}` | get_episode_detail | - | - | **R1** | read-only |
| 14 | GET | `/episodes/{chat_session_id}` | get_episodes | - | - | **R1** | read-only |
| 15 | GET | `/feedback/latest` | get_latest_feedback | - | - | **R1** | read-only |
| 16 | GET | `/metrics/lc1c/{chat_session_id}` | get_lc1c_memory_footprint | - | - | **R1** | read-only |
| 17 | GET | `/metrics/lc1d/{chat_session_id}` | get_lc1d_integrity_replay | - | - | **R1** | read-only |
| 18 | GET | `/metrics/lc1e/{chat_session_id}` | get_lc1e_context_budget_comparison | - | - | **R1** | read-only |
| 19 | GET | `/metrics/lc1f/{chat_session_id}` | get_lc1f_non_regression_check | - | - | **R1** | read-only |
| 20 | GET | `/metrics/lc1g/{chat_session_id}` | get_lc1g_accuracy_replay | - | - | **R1** | read-only |
| 21 | GET | `/metrics/lc1h/{chat_session_id}` | get_lc1h_forgetting_hallucination_replay | - | - | **R1** | read-only |
| 22 | GET | `/metrics/lc1i/{chat_session_id}` | get_lc1i_ablation_compare | - | - | **R1** | read-only |
| 23 | GET | `/metrics/lc1j/{chat_session_id}` | get_lc1j | - | - | **R1** | read-only |
| 24 | GET | `/metrics/lc1k/{chat_session_id}` | get_lc1k | - | - | **R1** | read-only |
| 25 | GET | `/metrics/lc1l/{chat_session_id}` | get_lc1l | - | - | **R1** | read-only |
| 26 | GET | `/metrics/lc1m/{chat_session_id}` | get_lc1m | - | - | **R1** | read-only |
| 27 | GET | `/metrics/lc1n/{chat_session_id}` | get_lc1n | - | - | **R1** | read-only |
| 28 | GET | `/metrics/lc1o/{chat_session_id}` | get_lc1o | - | - | **R1** | read-only |
| 29 | GET | `/metrics/lc1p/{chat_session_id}` | get_lc1p_evaluation_split_summary | - | - | **R1** | read-only |
| 30 | GET | `/metrics/lc1q/{chat_session_id}` | get_lc1q_freshness_lag_summary | - | - | **R1** | read-only |
| 31 | GET | `/metrics/lc1r/regression-corpus` | get_lc1r_regression_corpus_manifest | - | - | **R1** | read-only |
| 32 | GET | `/metrics/lc1s/step17-bundle-closure` | get_lc1s_step17_bundle_closure | - | - | **R1** | read-only |
| 33 | GET | `/metrics/tm1d/{chat_session_id}` | get_tm1d_truth_maintenance_audit_replay | - | - | **R1** | read-only |
| 34 | GET | `/momentum-packet/{chat_session_id}` | get_momentum_packet | - | - | **R1** | read-only |
| 35 | GET | `/narrative-control/{chat_session_id}` | get_narrative_control | - | - | **R1** | read-only |
| 36 | GET | `/pending-threads/{chat_session_id}` | get_pending_threads | - | - | **R1** | read-only |
| 37 | GET | `/session-state/{chat_session_id}` | get_session_state | - | - | **R1** | read-only |
| 38 | GET | `/session/{chat_session_id}/active-scope` | get_active_scope | - | - | **R1** | read-only |
| 39 | GET | `/sessions` | list_sessions | - | - | **R1** | read-only |
| 40 | GET | `/sessions/compare` | compare_sessions | - | - | **R1** | read-only |
| 41 | GET | `/sessions/{chat_session_id}/export` | export_session | - | - | **R1** | read-only |
| 42 | GET | `/sessions/{chat_session_id}/guidance-snapshot` | get_guidance_snapshot | - | - | **R1** | read-only |
| 43 | GET | `/sessions/{chat_session_id}/step7-health` | get_step7_health | - | - | **R1** | read-only |
| 44 | GET | `/storylines/{chat_session_id}` | get_storylines | - | - | **R1** | read-only |
| 45 | GET | `/world-rules/{chat_session_id}` | get_world_rules | - | - | **R1** | read-only |
| 46 | GET | `/world-rules/{chat_session_id}/inherited` | get_inherited_world_rules | - | - | **R1** | read-only |
| 47 | PATCH | `/characters/{chat_session_id}/{character_name}` | patch_character | PatchCharacterRequest | - | **R2** | mutating |
| 48 | PATCH | `/characters/{chat_session_id}/{character_name}/speech` | patch_character_speech | PatchSpeechStyleRequest | - | **R2** | mutating |
| 49 | PATCH | `/episodes/{episode_id}` | patch_episode | PatchEpisodeRequest | - | **R2** | mutating |
| 50 | PATCH | `/narrative-control/{chat_session_id}/director-patch` | patch_director_state | DirectorPatchRequest | - | **R2** | mutating |
| 51 | PATCH | `/pending-threads/{hook_id}` | patch_continuity_hook | PatchPendingThreadRequest | - | **R2** | mutating |
| 52 | PATCH | `/pending-threads/{hook_id}/trust` | patch_continuity_hook_trust | TrustControlRequest | - | **R2** | mutating |
| 53 | PATCH | `/session/{chat_session_id}/active-scope` | set_active_scope | ActiveScopeRequest | - | **R2** | mutating |
| 54 | PATCH | `/storylines/{storyline_id}` | patch_storyline | PatchStorylineRequest | - | **R2** | mutating |
| 55 | PATCH | `/storylines/{storyline_id}/trust` | patch_storyline_trust | TrustControlRequest | - | **R2** | mutating |
| 56 | PATCH | `/world-rules/{rule_id}` | patch_world_rule | PatchWorldRuleRequest | - | **R2** | mutating |
| 57 | PATCH | `/world-rules/{rule_id}/trust` | patch_world_rule_trust | TrustControlRequest | - | **R2** | mutating |
| 58 | POST | `/arcs/generate` | generate_arc | ArcGenerateRequest | - | **R2** | mutating |
| 59 | POST | `/chapters/dry-run` | chapter_generation_dry_run | ChapterDryRunRequest | - | **R1** | mutating |
| 60 | POST | `/chapters/generate` | generate_chapter | ChapterGenerateRequest | - | **R2** | mutating |
| 61 | POST | `/chapters/search` | search_chapters | ChapterSearchRequest | - | **R1** | mutating |
| 62 | POST | `/episodes/generate` | generate_episode | EpisodeGenerateRequest | - | **R2** | mutating |
| 63 | POST | `/episodes/merge` | merge_episodes_endpoint | EpisodeMergeRequest | - | **R2** | mutating |
| 64 | POST | `/episodes/regenerate` | regenerate_episode | EpisodeGenerateRequest | - | **R2** | mutating |
| 65 | POST | `/episodes/search` | search_episodes | EpisodeSearchRequest | - | **R1** | mutating |
| 66 | POST | `/feedback` | post_feedback | FeedbackRequest | - | **R2** | mutating |
| 67 | POST | `/import/hypamemory` | import_hypamemory | HypaImportRequest | - | **R2** | mutating |
| 68 | POST | `/sagas/generate` | generate_saga | SagaGenerateRequest | - | **R2** | mutating |
| 69 | POST | `/storylines/sync` | sync_storylines | StorylineSyncRequest | - | **R2** | mutating |
| 70 | POST | `/world-rules/sync` | sync_world_rules | WorldRuleSyncRequest | - | **R2** | mutating |
**Group 6 Summary:** R0=0, R1=44, R2=26

---
## Cross-Group Totals

| Group | R0 | R1 | R2 | Total | Notes |
|-------|----|----|----|-------|-------|
| 1 — Health / Config / Prompts | 4 | 4 | 2 | 10 | Safe probes + prompt/config writes |
| 2 — Prepare / Complete Turn | 0 | 0 | 7 | 7 | Core user surface; all R2 |
| 3 — Memory / Search / Retrieval / Chroma | 1 | 12 | 23 | 36 | Large read surface; chroma batch ops are R2 |
| 4 — Proxy / Provider | 0 | 1 | 2 | 3 | External calls = R2; critic test = R1 |
| 5 — Admin / Maintenance | 0 | 1 | 5 | 6 | All mutating ops are R2 |
| 6 — Export / Import / Debug / Metrics / Narrative / Session | 0 | 44 | 26 | 70 | Mostly read-only narrative/session surface |
| **Total** | **5** | **62** | **65** | **132** | |

## Blocker Summary by Group

| Group | Blocker Routes | Blocker Kind | Impact |
|-------|---------------|--------------|--------|
| 3 — Memory / Search / Retrieval / Chroma | `PATCH /explorer/kg_triples/{triple_id}` | `non-null union anyOf` on `valid_from`, `valid_to` | DTO decode needs custom union policy |
| 4 — Proxy / Provider | `POST /proxy/plugin-main` | `untyped property` on `messages.items` | `ProxyPluginMainRequest` uses `[]map[string]any` |

No other routes carry DTO-level blockers. All remaining routes map cleanly to generated Go structs or path/query parameters.

## Cutover Priority Recommendation

1. **R0 first** — Implement health/config probes to establish Go skeleton viability.
2. **R1 next** — Shadow-implement read-only routes (search, explorer GETs, metrics, session reads) to build confidence in DTO decode and DB read parity.
3. **R2 last** — Do not implement turn core, admin ops, chroma batch backfill, or external provider calls until explicit authority switch is approved.
