# Go Route to DTO Mapping Table

> Source of truth: `Archive Center Beta 0.8(fix)\backend\main.py` and `backend\routes`
> Generated from static extraction of FastAPI route decorators and function signatures.
> Date: 2026-05-21
> Status: R0 inventory; not a migration plan, only a classification.

## Legend

| Column | Meaning |
|--------|---------|
| Method / Path | FastAPI route identifier |
| Func | Python handler function name |
| Request DTO (Go) | Go struct generated from OpenAPI schema; empty means path/query params only |
| Response DTO (Go) | Go struct or `map[string]any` / `any` for untyped responses |
| Blockers | DTO mapping blockers from `go-dto-mapping-plan.json` (e.g., `untyped`, `oneOf`, `inline_object`) |
| Side Effect Risk | `read` for safe GETs; `write` for mutating endpoints; `write/chroma` for Chroma shadow ops |

## Route to DTO Mapping Table

| # | Method | Path | Func | Request DTO (Go) | Response DTO (Go) | Blockers | Side Effect |
|---|--------|------|------|-----------------|-------------------|----------|-------------|
| 1 | DELETE | `/characters/{chat_session_id}/{character_name}` | delete_character | - | - | none | write |
| 2 | DELETE | `/episodes/{episode_id}` | delete_episode | - | - | none | write |
| 3 | DELETE | `/explorer/kg_triples/{triple_id}` | delete_kg_triple | - | - | none | write |
| 4 | DELETE | `/explorer/memories/{memory_id}` | delete_memory | - | - | none | write |
| 5 | DELETE | `/pending-threads/{hook_id}` | delete_continuity_hook | - | - | none | write |
| 6 | DELETE | `/rollback/{turn_index}` | rollback | - | - | none | write |
| 7 | DELETE | `/storylines/{storyline_id}` | delete_storyline | - | - | none | write |
| 8 | DELETE | `/world-rules/{rule_id}` | delete_world_rule | - | - | none | write |
| 9 | GET | `/active-states/{chat_session_id}` | get_active_states | - | - | none | read |
| 10 | GET | `/audit` | get_audit_logs | - | - | none | read |
| 11 | GET | `/canonical-state-layer/{chat_session_id}` | get_canonical_state_layer | - | - | none | read |
| 12 | GET | `/characters/{chat_session_id}` | get_characters | - | - | none | read |
| 13 | GET | `/characters/{chat_session_id}/{character_name}` | get_character_detail | - | - | none | read |
| 14 | GET | `/characters/{chat_session_id}/{character_name}/events` | get_character_events | - | - | none | read |
| 15 | GET | `/chroma-shadow/preflight` | get_chroma_shadow_preflight | - | - | none | read/chroma |
| 16 | GET | `/continuity-pack/{chat_session_id}` | get_continuity_pack | - | - | none | read |
| 17 | GET | `/episodes/detail/{episode_id}` | get_episode_detail | - | - | none | read |
| 18 | GET | `/episodes/{chat_session_id}` | get_episodes | - | - | none | read |
| 19 | GET | `/explorer/chapter_summaries` | get_chapter_summaries | - | - | none | read |
| 20 | GET | `/explorer/chat_logs` | get_chat_logs | - | - | none | read |
| 21 | GET | `/explorer/direct-evidence` | get_direct_evidence | - | - | none | read |
| 22 | GET | `/explorer/kg_triples` | get_kg_triples | - | - | none | read |
| 23 | GET | `/explorer/memories` | get_memories | - | - | none | read |
| 24 | GET | `/feedback/latest` | get_latest_feedback | - | - | none | read |
| 25 | GET | `/health` | health | - | - | none | read |
| 26 | GET | `/intent-routing/runtime-config` | get_intent_routing_runtime_config_api | - | - | none | read |
| 27 | GET | `/kg/recall` | kg_recall_get | - | - | none | read |
| 28 | GET | `/long-session-health/{session_id}` | get_long_session_health | - | - | none | read |
| 29 | GET | `/maintenance/queue-status` | get_maintenance_queue_status | - | - | none | read |
| 30 | GET | `/metrics/lc1c/{chat_session_id}` | get_lc1c_memory_footprint | - | - | none | read |
| 31 | GET | `/metrics/lc1d/{chat_session_id}` | get_lc1d_integrity_replay | - | - | none | read |
| 32 | GET | `/metrics/lc1e/{chat_session_id}` | get_lc1e_context_budget_comparison | - | - | none | read |
| 33 | GET | `/metrics/lc1f/{chat_session_id}` | get_lc1f_non_regression_check | - | - | none | read |
| 34 | GET | `/metrics/lc1g/{chat_session_id}` | get_lc1g_accuracy_replay | - | - | none | read |
| 35 | GET | `/metrics/lc1h/{chat_session_id}` | get_lc1h_forgetting_hallucination_replay | - | - | none | read |
| 36 | GET | `/metrics/lc1i/{chat_session_id}` | get_lc1i_ablation_compare | - | - | none | read |
| 37 | GET | `/metrics/lc1j/{chat_session_id}` | get_lc1j | - | - | none | read |
| 38 | GET | `/metrics/lc1k/{chat_session_id}` | get_lc1k | - | - | none | read |
| 39 | GET | `/metrics/lc1l/{chat_session_id}` | get_lc1l | - | - | none | read |
| 40 | GET | `/metrics/lc1m/{chat_session_id}` | get_lc1m | - | - | none | read |
| 41 | GET | `/metrics/lc1n/{chat_session_id}` | get_lc1n | - | - | none | read |
| 42 | GET | `/metrics/lc1o/{chat_session_id}` | get_lc1o | - | - | none | read |
| 43 | GET | `/metrics/lc1p/{chat_session_id}` | get_lc1p_evaluation_split_summary | - | - | none | read |
| 44 | GET | `/metrics/lc1q/{chat_session_id}` | get_lc1q_freshness_lag_summary | - | - | none | read |
| 45 | GET | `/metrics/lc1r/regression-corpus` | get_lc1r_regression_corpus_manifest | - | - | none | read |
| 46 | GET | `/metrics/lc1s/step17-bundle-closure` | get_lc1s_step17_bundle_closure | - | - | none | read |
| 47 | GET | `/metrics/tm1d/{chat_session_id}` | get_tm1d_truth_maintenance_audit_replay | - | - | none | read |
| 48 | GET | `/momentum-packet/{chat_session_id}` | get_momentum_packet | - | - | none | read |
| 49 | GET | `/narrative-control/{chat_session_id}` | get_narrative_control | - | - | none | read |
| 50 | GET | `/pending-threads/{chat_session_id}` | get_pending_threads | - | - | none | read |
| 51 | GET | `/prompts` | prompt_list | - | - | none | read |
| 52 | GET | `/prompts/{prompt_name}` | prompt_get | - | - | none | read |
| 53 | GET | `/ready` | handle_ready | - | - | none | read |
| 54 | GET | `/retrieval-index/runtime-config` | get_retrieval_index_runtime_config | - | - | none | read |
| 55 | GET | `/retrieval-index/{chat_session_id}` | get_retrieval_index_snapshot | - | - | none | read |
| 56 | GET | `/retrieval-index/{chat_session_id}/source-row` | get_retrieval_index_source_row | - | - | none | read |
| 57 | GET | `/session-state/{chat_session_id}` | get_session_state | - | - | none | read |
| 58 | GET | `/session/{chat_session_id}/active-scope` | get_active_scope | - | - | none | read |
| 59 | GET | `/sessions` | list_sessions | - | - | none | read |
| 60 | GET | `/sessions/compare` | compare_sessions | - | - | none | read |
| 61 | GET | `/sessions/{chat_session_id}/export` | export_session | - | - | none | read |
| 62 | GET | `/sessions/{chat_session_id}/guidance-snapshot` | get_guidance_snapshot | - | - | none | read |
| 63 | GET | `/sessions/{chat_session_id}/step7-health` | get_step7_health | - | - | none | read |
| 64 | GET | `/stats` | db_stats | - | - | none | read |
| 65 | GET | `/storylines/{chat_session_id}` | get_storylines | - | - | none | read |
| 66 | GET | `/version` | handle_version | - | - | none | read |
| 67 | GET | `/wakeup` | wakeup | - | - | none | read |
| 68 | GET | `/world-rules/{chat_session_id}` | get_world_rules | - | - | none | read |
| 69 | GET | `/world-rules/{chat_session_id}/inherited` | get_inherited_world_rules | - | - | none | read |
| 70 | PATCH | `/characters/{chat_session_id}/{character_name}` | patch_character | PatchCharacterRequest | - | none | write |
| 71 | PATCH | `/characters/{chat_session_id}/{character_name}/speech` | patch_character_speech | PatchSpeechStyleRequest | - | none | write |
| 72 | PATCH | `/episodes/{episode_id}` | patch_episode | PatchEpisodeRequest | - | none | write |
| 73 | PATCH | `/explorer/direct-evidence/{record_id}/revalidate` | patch_direct_evidence_revalidate | DirectEvidenceRevalidateRequest | - | none | write |
| 74 | PATCH | `/explorer/direct-evidence/{record_id}/review` | patch_direct_evidence_review | PatchDirectEvidenceReviewRequest | - | none | write |
| 75 | PATCH | `/explorer/direct-evidence/{record_id}/supersede` | patch_direct_evidence_supersede | PatchDirectEvidenceSupersedeRequest | - | none | write |
| 76 | PATCH | `/explorer/direct-evidence/{record_id}/tombstone` | patch_direct_evidence_tombstone | PatchDirectEvidenceTombstoneRequest | - | none | write |
| 77 | PATCH | `/explorer/kg_triples/{triple_id}` | patch_kg_triple | PatchKGTripleRequest | - | PatchKGTripleRequest.properties.valid_from: non-null union anyOf; PatchKGTripleRequest.properties.valid_to: non-null union anyOf | write |
| 78 | PATCH | `/explorer/memories/{memory_id}` | patch_memory | PatchMemoryRequest | - | none | write |
| 79 | PATCH | `/narrative-control/{chat_session_id}/director-patch` | patch_director_state | DirectorPatchRequest | - | none | write |
| 80 | PATCH | `/pending-threads/{hook_id}` | patch_continuity_hook | PatchPendingThreadRequest | - | none | write |
| 81 | PATCH | `/pending-threads/{hook_id}/trust` | patch_continuity_hook_trust | TrustControlRequest | - | none | write |
| 82 | PATCH | `/session/{chat_session_id}/active-scope` | set_active_scope | ActiveScopeRequest | - | none | write |
| 83 | PATCH | `/storylines/{storyline_id}` | patch_storyline | PatchStorylineRequest | - | none | write |
| 84 | PATCH | `/storylines/{storyline_id}/trust` | patch_storyline_trust | TrustControlRequest | - | none | write |
| 85 | PATCH | `/world-rules/{rule_id}` | patch_world_rule | PatchWorldRuleRequest | - | none | write |
| 86 | PATCH | `/world-rules/{rule_id}/trust` | patch_world_rule_trust | TrustControlRequest | - | none | write |
| 87 | POST | `/admin/reindex` | admin_reindex | ReindexRequest | - | none | write |
| 88 | POST | `/admin/rescan` | admin_rescan | RescanRequest | - | none | write |
| 89 | POST | `/admin/session-migrate` | admin_session_migrate | SessionMigrateRequest | - | none | write |
| 90 | POST | `/arcs/generate` | generate_arc | ArcGenerateRequest | - | none | write |
| 91 | POST | `/chapters/dry-run` | chapter_generation_dry_run | ChapterDryRunRequest | - | none | write |
| 92 | POST | `/chapters/generate` | generate_chapter | ChapterGenerateRequest | - | none | write |
| 93 | POST | `/chapters/search` | search_chapters | ChapterSearchRequest | - | none | write |
| 94 | POST | `/chroma-shadow/adoption-gate` | post_chroma_shadow_adoption_gate | ChromaShadowAdoptionGateRequest | - | none | write/chroma |
| 95 | POST | `/chroma-shadow/backfill-batch` | post_chroma_shadow_backfill_batch | ChromaShadowBackfillBatchRequest | - | none | write/chroma |
| 96 | POST | `/chroma-shadow/backfill-dry-run` | post_chroma_shadow_backfill_dry_run | ChromaShadowBackfillDryRunRequest | - | none | write/chroma |
| 97 | POST | `/chroma-shadow/bootstrap` | post_chroma_shadow_bootstrap | - | - | none | write/chroma |
| 98 | POST | `/chroma-shadow/fallback-runbook` | post_chroma_shadow_fallback_runbook | ChromaShadowFallbackRunbookRequest | - | none | write/chroma |
| 99 | POST | `/chroma-shadow/health-probe` | post_chroma_shadow_health_probe | ChromaShadowHealthProbeRequest | - | none | write/chroma |
| 100 | POST | `/chroma-shadow/rebuild-drill` | post_chroma_shadow_rebuild_drill | ChromaShadowRebuildDrillRequest | - | none | write/chroma |
| 101 | POST | `/chroma-shadow/reembed-audit` | post_chroma_shadow_reembed_audit | ChromaShadowReembedAuditRequest | - | none | write/chroma |
| 102 | POST | `/chroma-shadow/release-hygiene` | post_chroma_shadow_release_hygiene | ChromaShadowReleaseHygieneRequest | - | none | write/chroma |
| 103 | POST | `/chroma-shadow/visibility-guard` | post_chroma_shadow_visibility_guard | ChromaShadowVisibilityGuardRequest | - | none | write/chroma |
| 104 | POST | `/complete-turn` | complete_turn_m4 | M4CompleteTurnRequest | M4CompleteTurnResponse | none | write |
| 105 | POST | `/config/update` | config_update | Request | - | none | write |
| 106 | POST | `/critic/test` | critic_test | CriticTestRequest | - | none | write |
| 107 | POST | `/effective-inputs` | save_effective_input | SaveEffectiveInputRequest | - | none | write |
| 108 | POST | `/episodes/generate` | generate_episode | EpisodeGenerateRequest | - | none | write |
| 109 | POST | `/episodes/merge` | merge_episodes_endpoint | EpisodeMergeRequest | - | none | write |
| 110 | POST | `/episodes/regenerate` | regenerate_episode | EpisodeGenerateRequest | - | none | write |
| 111 | POST | `/episodes/search` | search_episodes | EpisodeSearchRequest | - | none | write |
| 112 | POST | `/explorer/kg_triples/{triple_id}/delete` | delete_kg_triple_via_post | - | - | none | write |
| 113 | POST | `/explorer/memories/regenerate` | regenerate_memory | - | - | none | write |
| 114 | POST | `/explorer/memories/{memory_id}/delete` | delete_memory_via_post | - | - | none | write |
| 115 | POST | `/feedback` | post_feedback | FeedbackRequest | - | none | write |
| 116 | POST | `/import/hypamemory` | import_hypamemory | HypaImportRequest | - | none | write |
| 117 | POST | `/intent-routing/runtime-config` | update_intent_routing_runtime_config | IntentRoutingRuntimeConfigRequest | - | none | write |
| 118 | POST | `/kg/recall` | kg_recall | KGRecallRequest | - | none | write |
| 119 | POST | `/maintenance-pass/{chat_session_id}` | run_maintenance_pass | MaintenancePassRequest | - | none | write |
| 120 | POST | `/maintenance/enqueue` | enqueue_maintenance_job | MaintenanceEnqueueRequest | MaintenanceEnqueueResponse | none | write |
| 121 | POST | `/prepare-turn` | prepare_turn | PrepareTurnRequest | - | none | write |
| 122 | POST | `/proxy/plugin-main` | proxy_plugin_main | ProxyPluginMainRequest | - | ProxyPluginMainRequest.properties.messages.items: untyped property | write |
| 123 | POST | `/retrieval-index/runtime-config` | update_retrieval_index_runtime_config | RetrievalIndexRuntimeConfigRequest | - | none | write |
| 124 | POST | `/sagas/generate` | generate_saga | SagaGenerateRequest | - | none | write |
| 125 | POST | `/search` | search | SearchRequest | - | none | write |
| 126 | POST | `/storylines/sync` | sync_storylines | StorylineSyncRequest | - | none | write |
| 127 | POST | `/supervisor` | supervisor_directive | SupervisorRequest | - | none | write |
| 128 | POST | `/turns` | save_turn | SaveTurnRequest | - | none | write |
| 129 | POST | `/turns/complete` | complete_turn | CompleteTurnRequest | - | none | write |
| 130 | POST | `/turns/repair-replay` | repair_turn_replay | ChatLogRepairReplayRequest | - | none | write |
| 131 | POST | `/world-rules/sync` | sync_world_rules | WorldRuleSyncRequest | - | none | write |
| 132 | PUT | `/prompts/{prompt_name}` | prompt_update | PromptUpdateRequest | - | none | write |
