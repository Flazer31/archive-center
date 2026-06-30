# OpenAPI Schema Inventory

> Status: R0 evidence. Schema extraction is preparatory, not a cutover completion.
> This document lists all schema components and their usage, plus Go struct mapping blockers.
> Go struct mapping itself is NOT done yet.

- **OpenAPI Version**: 3.1.0
- **API Title**: RisuAI Memory Backend
- **API Version**: 0.1.0
- **Schema Component Count**: 62

## Schema Components

### ActiveScopeRequest
- PATCH /session/{chat_session_id}/active-scope

### ArcGenerateRequest
- POST /arcs/generate

### ChapterDryRunRequest
- POST /chapters/dry-run

### ChapterGenerateRequest
- POST /chapters/generate

### ChapterSearchRequest
- POST /chapters/search

### ChatLogRepairReplayRequest
- POST /turns/repair-replay

### ChromaShadowAdoptionGateRequest
- POST /chroma-shadow/adoption-gate

### ChromaShadowBackfillBatchRequest
- POST /chroma-shadow/backfill-batch

### ChromaShadowBackfillDryRunRequest
- POST /chroma-shadow/backfill-dry-run

### ChromaShadowFallbackRunbookRequest
- POST /chroma-shadow/fallback-runbook

### ChromaShadowHealthProbeRequest
- POST /chroma-shadow/health-probe

### ChromaShadowRebuildDrillRequest
- POST /chroma-shadow/rebuild-drill

### ChromaShadowReembedAuditRequest
- POST /chroma-shadow/reembed-audit

### ChromaShadowReleaseHygieneRequest
- POST /chroma-shadow/release-hygiene

### ChromaShadowVisibilityGuardRequest
- POST /chroma-shadow/visibility-guard

### CompleteTurnRequest
- POST /turns/complete

### CriticTestRequest
- POST /critic/test

### DirectEvidenceRevalidateRequest
- PATCH /explorer/direct-evidence/{record_id}/revalidate

### DirectorPatchRequest
- PATCH /narrative-control/{chat_session_id}/director-patch

### EpisodeGenerateRequest
- POST /episodes/generate
- POST /episodes/regenerate

### EpisodeMergeRequest
- POST /episodes/merge

### EpisodeSearchRequest
- POST /episodes/search

### FeedbackRequest
- POST /feedback

### HTTPValidationError
- DELETE /characters/{chat_session_id}/{character_name}
- DELETE /episodes/{episode_id}
- DELETE /explorer/kg_triples/{triple_id}
- DELETE /explorer/memories/{memory_id}
- DELETE /pending-threads/{hook_id}
- DELETE /rollback/{turn_index}
- DELETE /storylines/{storyline_id}
- DELETE /world-rules/{rule_id}
- GET /active-states/{chat_session_id}
- GET /audit
- GET /canonical-state-layer/{chat_session_id}
- GET /characters/{chat_session_id}
- GET /characters/{chat_session_id}/{character_name}
- GET /characters/{chat_session_id}/{character_name}/events
- GET /continuity-pack/{chat_session_id}
- GET /episodes/detail/{episode_id}
- GET /episodes/{chat_session_id}
- GET /explorer/chapter_summaries
- GET /explorer/chat_logs
- GET /explorer/direct-evidence
- GET /explorer/kg_triples
- GET /explorer/memories
- GET /feedback/latest
- GET /kg/recall
- GET /long-session-health/{session_id}
- GET /maintenance/queue-status
- GET /metrics/lc1c/{chat_session_id}
- GET /metrics/lc1d/{chat_session_id}
- GET /metrics/lc1e/{chat_session_id}
- GET /metrics/lc1f/{chat_session_id}
- GET /metrics/lc1g/{chat_session_id}
- GET /metrics/lc1h/{chat_session_id}
- GET /metrics/lc1i/{chat_session_id}
- GET /metrics/lc1j/{chat_session_id}
- GET /metrics/lc1k/{chat_session_id}
- GET /metrics/lc1l/{chat_session_id}
- GET /metrics/lc1m/{chat_session_id}
- GET /metrics/lc1n/{chat_session_id}
- GET /metrics/lc1o/{chat_session_id}
- GET /metrics/lc1p/{chat_session_id}
- GET /metrics/lc1q/{chat_session_id}
- GET /metrics/tm1d/{chat_session_id}
- GET /momentum-packet/{chat_session_id}
- GET /narrative-control/{chat_session_id}
- GET /pending-threads/{chat_session_id}
- GET /prompts/{prompt_name}
- GET /retrieval-index/{chat_session_id}
- GET /retrieval-index/{chat_session_id}/source-row
- GET /session-state/{chat_session_id}
- GET /session/{chat_session_id}/active-scope
- GET /sessions/compare
- GET /sessions/{chat_session_id}/export
- GET /sessions/{chat_session_id}/guidance-snapshot
- GET /sessions/{chat_session_id}/step7-health
- GET /storylines/{chat_session_id}
- GET /world-rules/{chat_session_id}
- GET /world-rules/{chat_session_id}/inherited
- PATCH /characters/{chat_session_id}/{character_name}
- PATCH /characters/{chat_session_id}/{character_name}/speech
- PATCH /episodes/{episode_id}
- PATCH /explorer/direct-evidence/{record_id}/revalidate
- PATCH /explorer/direct-evidence/{record_id}/review
- PATCH /explorer/direct-evidence/{record_id}/supersede
- PATCH /explorer/direct-evidence/{record_id}/tombstone
- PATCH /explorer/kg_triples/{triple_id}
- PATCH /explorer/memories/{memory_id}
- PATCH /narrative-control/{chat_session_id}/director-patch
- PATCH /pending-threads/{hook_id}
- PATCH /pending-threads/{hook_id}/trust
- PATCH /session/{chat_session_id}/active-scope
- PATCH /storylines/{storyline_id}
- PATCH /storylines/{storyline_id}/trust
- PATCH /world-rules/{rule_id}
- PATCH /world-rules/{rule_id}/trust
- POST /admin/reindex
- POST /admin/rescan
- POST /admin/session-migrate
- POST /arcs/generate
- POST /chapters/dry-run
- POST /chapters/generate
- POST /chapters/search
- POST /chroma-shadow/adoption-gate
- POST /chroma-shadow/backfill-batch
- POST /chroma-shadow/backfill-dry-run
- POST /chroma-shadow/fallback-runbook
- POST /chroma-shadow/health-probe
- POST /chroma-shadow/rebuild-drill
- POST /chroma-shadow/reembed-audit
- POST /chroma-shadow/release-hygiene
- POST /chroma-shadow/visibility-guard
- POST /complete-turn
- POST /config/update
- POST /critic/test
- POST /effective-inputs
- POST /episodes/generate
- POST /episodes/merge
- POST /episodes/regenerate
- POST /episodes/search
- POST /explorer/kg_triples/{triple_id}/delete
- POST /explorer/memories/regenerate
- POST /explorer/memories/{memory_id}/delete
- POST /feedback
- POST /import/hypamemory
- POST /intent-routing/runtime-config
- POST /kg/recall
- POST /maintenance-pass/{chat_session_id}
- POST /maintenance/enqueue
- POST /prepare-turn
- POST /proxy/plugin-main
- POST /retrieval-index/runtime-config
- POST /sagas/generate
- POST /search
- POST /storylines/sync
- POST /supervisor
- POST /turns
- POST /turns/complete
- POST /turns/repair-replay
- POST /world-rules/sync
- PUT /prompts/{prompt_name}

### HypaImportRequest
- POST /import/hypamemory

### IntentRoutingRuntimeConfigRequest
- POST /intent-routing/runtime-config

### KGRecallRequest
- POST /kg/recall

### M4CompleteTurnRequest
- POST /complete-turn

### M4CompleteTurnResponse
- POST /complete-turn

### MaintenanceEnqueueRequest
- POST /maintenance/enqueue

### MaintenanceEnqueueResponse
- POST /maintenance/enqueue

### MaintenancePassRequest
- POST /maintenance-pass/{chat_session_id}

### PatchCharacterRequest
- PATCH /characters/{chat_session_id}/{character_name}

### PatchDirectEvidenceReviewRequest
- PATCH /explorer/direct-evidence/{record_id}/review

### PatchDirectEvidenceSupersedeRequest
- PATCH /explorer/direct-evidence/{record_id}/supersede

### PatchDirectEvidenceTombstoneRequest
- PATCH /explorer/direct-evidence/{record_id}/tombstone

### PatchEpisodeRequest
- PATCH /episodes/{episode_id}

### PatchKGTripleRequest
- PATCH /explorer/kg_triples/{triple_id}

### PatchMemoryRequest
- PATCH /explorer/memories/{memory_id}

### PatchPendingThreadRequest
- PATCH /pending-threads/{hook_id}

### PatchSpeechStyleRequest
- PATCH /characters/{chat_session_id}/{character_name}/speech

### PatchStorylineRequest
- PATCH /storylines/{storyline_id}

### PatchWorldRuleRequest
- PATCH /world-rules/{rule_id}

### PrepareTurnRequest
- POST /prepare-turn

### PromptUpdateRequest
- PUT /prompts/{prompt_name}

### ProxyPluginMainRequest
- POST /proxy/plugin-main

### ReindexRequest
- POST /admin/reindex

### RescanRequest
- POST /admin/rescan

### RetrievalIndexRuntimeConfigRequest
- POST /retrieval-index/runtime-config

### SagaGenerateRequest
- POST /sagas/generate

### SaveEffectiveInputRequest
- POST /effective-inputs

### SaveTurnRequest
- POST /turns

### SearchRequest
- POST /search

### SessionMigrateRequest
- POST /admin/session-migrate

### StorylineSyncRequest
- POST /storylines/sync

### SupervisorRequest
- POST /supervisor

### TrustControlRequest
- PATCH /pending-threads/{hook_id}/trust
- PATCH /storylines/{storyline_id}/trust
- PATCH /world-rules/{rule_id}/trust

### WorldRuleSyncRequest
- POST /world-rules/sync

## Go Struct Mapping Blockers

| Blocker Type | Count | Details |
|--------------|-------|---------|
| anyOf | 102 | `ActiveScopeRequest.properties.scope_name`, `CompleteTurnRequest.properties.output_language_override`, `CriticTestRequest.properties.output_language_override`, `DirectEvidenceRevalidateRequest.properties.review_note`, `DirectorPatchRequest.properties.scene_mandate`, ... (97 more) |
| oneOf | 0 | _None_ |
| allOf | 0 | _None_ |
| additionalProperties | 30 | `ChromaShadowAdoptionGateRequest.properties.operator_evidence`, `ChromaShadowBackfillBatchRequest.properties.checkpoint`, `ChromaShadowBackfillBatchRequest.properties.retry_rows.items`, `ChromaShadowRebuildDrillRequest.properties.checkpoint`, `ChromaShadowRebuildDrillRequest.properties.retry_rows.items`, ... (25 more) |
| arrays_without_items | 0 | _None_ |
| nullable_without_type | 0 | _None_ |
| object_without_properties | 1 | `ValidationError.properties.ctx` |

## Route Schema Refs Summary

| Method | Path | Operation ID | Request Refs | Response Refs |
|--------|------|--------------|--------------|---------------|
| POST | `/proxy/plugin-main` | proxy_plugin_main_proxy_plugin_main_post | #/components/schemas/ProxyPluginMainRequest | #/components/schemas/HTTPValidationError |
| POST | `/config/update` | config_update_config_update_post | inline:application/json | #/components/schemas/HTTPValidationError |
| PATCH | `/narrative-control/{chat_session_id}/director-patch` | patch_director_state_narrative_control__chat_session_id__director_patch_patch | #/components/schemas/DirectorPatchRequest | #/components/schemas/HTTPValidationError |
| POST | `/maintenance-pass/{chat_session_id}` | run_maintenance_pass_maintenance_pass__chat_session_id__post | #/components/schemas/MaintenancePassRequest | #/components/schemas/HTTPValidationError |
| POST | `/maintenance/enqueue` | enqueue_maintenance_job_maintenance_enqueue_post | #/components/schemas/MaintenanceEnqueueRequest | #/components/schemas/MaintenanceEnqueueResponse, #/components/schemas/HTTPValidationError |
| GET | `/maintenance/queue-status` | get_maintenance_queue_status_maintenance_queue_status_get | - | #/components/schemas/HTTPValidationError |
| GET | `/long-session-health/{session_id}` | get_long_session_health_long_session_health__session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/health` | health_health_get | - | - |
| POST | `/turns` | save_turn_turns_post | #/components/schemas/SaveTurnRequest | #/components/schemas/HTTPValidationError |
| POST | `/turns/repair-replay` | repair_turn_replay_turns_repair_replay_post | #/components/schemas/ChatLogRepairReplayRequest | #/components/schemas/HTTPValidationError |
| POST | `/effective-inputs` | save_effective_input_effective_inputs_post | #/components/schemas/SaveEffectiveInputRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/rollback/{turn_index}` | rollback_rollback__turn_index__delete | - | #/components/schemas/HTTPValidationError |
| POST | `/search` | search_search_post | #/components/schemas/SearchRequest | #/components/schemas/HTTPValidationError |
| GET | `/retrieval-index/runtime-config` | get_retrieval_index_runtime_config_retrieval_index_runtime_config_get | - | - |
| POST | `/retrieval-index/runtime-config` | update_retrieval_index_runtime_config_retrieval_index_runtime_config_post | #/components/schemas/RetrievalIndexRuntimeConfigRequest | #/components/schemas/HTTPValidationError |
| GET | `/chroma-shadow/preflight` | get_chroma_shadow_preflight_chroma_shadow_preflight_get | - | - |
| POST | `/chroma-shadow/bootstrap` | post_chroma_shadow_bootstrap_chroma_shadow_bootstrap_post | - | - |
| POST | `/chroma-shadow/backfill-dry-run` | post_chroma_shadow_backfill_dry_run_chroma_shadow_backfill_dry_run_post | #/components/schemas/ChromaShadowBackfillDryRunRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/backfill-batch` | post_chroma_shadow_backfill_batch_chroma_shadow_backfill_batch_post | #/components/schemas/ChromaShadowBackfillBatchRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/reembed-audit` | post_chroma_shadow_reembed_audit_chroma_shadow_reembed_audit_post | #/components/schemas/ChromaShadowReembedAuditRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/health-probe` | post_chroma_shadow_health_probe_chroma_shadow_health_probe_post | #/components/schemas/ChromaShadowHealthProbeRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/fallback-runbook` | post_chroma_shadow_fallback_runbook_chroma_shadow_fallback_runbook_post | #/components/schemas/ChromaShadowFallbackRunbookRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/rebuild-drill` | post_chroma_shadow_rebuild_drill_chroma_shadow_rebuild_drill_post | #/components/schemas/ChromaShadowRebuildDrillRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/adoption-gate` | post_chroma_shadow_adoption_gate_chroma_shadow_adoption_gate_post | #/components/schemas/ChromaShadowAdoptionGateRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/release-hygiene` | post_chroma_shadow_release_hygiene_chroma_shadow_release_hygiene_post | #/components/schemas/ChromaShadowReleaseHygieneRequest | #/components/schemas/HTTPValidationError |
| POST | `/chroma-shadow/visibility-guard` | post_chroma_shadow_visibility_guard_chroma_shadow_visibility_guard_post | #/components/schemas/ChromaShadowVisibilityGuardRequest | #/components/schemas/HTTPValidationError |
| GET | `/intent-routing/runtime-config` | get_intent_routing_runtime_config_api_intent_routing_runtime_config_get | - | - |
| POST | `/intent-routing/runtime-config` | update_intent_routing_runtime_config_intent_routing_runtime_config_post | #/components/schemas/IntentRoutingRuntimeConfigRequest | #/components/schemas/HTTPValidationError |
| GET | `/retrieval-index/{chat_session_id}` | get_retrieval_index_snapshot_retrieval_index__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/retrieval-index/{chat_session_id}/source-row` | get_retrieval_index_source_row_retrieval_index__chat_session_id__source_row_get | - | #/components/schemas/HTTPValidationError |
| POST | `/critic/test` | critic_test_critic_test_post | #/components/schemas/CriticTestRequest | #/components/schemas/HTTPValidationError |
| POST | `/turns/complete` | complete_turn_turns_complete_post | #/components/schemas/CompleteTurnRequest | #/components/schemas/HTTPValidationError |
| POST | `/kg/recall` | kg_recall_kg_recall_post | #/components/schemas/KGRecallRequest | #/components/schemas/HTTPValidationError |
| GET | `/kg/recall` | kg_recall_legacy_get_kg_recall_get | - | #/components/schemas/HTTPValidationError |
| POST | `/supervisor` | supervisor_directive_supervisor_post | #/components/schemas/SupervisorRequest | #/components/schemas/HTTPValidationError |
| GET | `/wakeup` | wakeup_wakeup_get | - | - |
| GET | `/prompts` | prompt_list_prompts_get | - | - |
| GET | `/prompts/{prompt_name}` | prompt_get_prompts__prompt_name__get | - | #/components/schemas/HTTPValidationError |
| PUT | `/prompts/{prompt_name}` | prompt_update_prompts__prompt_name__put | #/components/schemas/PromptUpdateRequest | #/components/schemas/HTTPValidationError |
| GET | `/stats` | db_stats_stats_get | - | - |
| GET | `/sessions` | list_sessions_sessions_get | - | - |
| GET | `/explorer/chat_logs` | explorer_chat_logs_explorer_chat_logs_get | - | #/components/schemas/HTTPValidationError |
| GET | `/explorer/memories` | explorer_memories_explorer_memories_get | - | #/components/schemas/HTTPValidationError |
| GET | `/explorer/direct-evidence` | explorer_direct_evidence_explorer_direct_evidence_get | - | #/components/schemas/HTTPValidationError |
| GET | `/explorer/kg_triples` | explorer_kg_triples_explorer_kg_triples_get | - | #/components/schemas/HTTPValidationError |
| GET | `/explorer/chapter_summaries` | explorer_chapter_summaries_explorer_chapter_summaries_get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/memories/{memory_id}` | patch_memory_explorer_memories__memory_id__patch | #/components/schemas/PatchMemoryRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/explorer/memories/{memory_id}` | delete_memory_explorer_memories__memory_id__delete | - | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/kg_triples/{triple_id}` | patch_kg_triple_explorer_kg_triples__triple_id__patch | #/components/schemas/PatchKGTripleRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/explorer/kg_triples/{triple_id}` | delete_kg_triple_explorer_kg_triples__triple_id__delete | - | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/direct-evidence/{record_id}/review` | patch_direct_evidence_review_explorer_direct_evidence__record_id__review_patch | #/components/schemas/PatchDirectEvidenceReviewRequest | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/direct-evidence/{record_id}/revalidate` | patch_direct_evidence_revalidate_explorer_direct_evidence__record_id__revalidate_patch | #/components/schemas/DirectEvidenceRevalidateRequest | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/direct-evidence/{record_id}/tombstone` | patch_direct_evidence_tombstone_explorer_direct_evidence__record_id__tombstone_patch | #/components/schemas/PatchDirectEvidenceTombstoneRequest | #/components/schemas/HTTPValidationError |
| PATCH | `/explorer/direct-evidence/{record_id}/supersede` | patch_direct_evidence_supersede_explorer_direct_evidence__record_id__supersede_patch | #/components/schemas/PatchDirectEvidenceSupersedeRequest | #/components/schemas/HTTPValidationError |
| POST | `/explorer/memories/regenerate` | regenerate_memory_explorer_memories_regenerate_post | inline:application/json | #/components/schemas/HTTPValidationError |
| POST | `/explorer/memories/{memory_id}/delete` | delete_memory_via_post_explorer_memories__memory_id__delete_post | - | #/components/schemas/HTTPValidationError |
| POST | `/explorer/kg_triples/{triple_id}/delete` | delete_kg_triple_via_post_explorer_kg_triples__triple_id__delete_post | - | #/components/schemas/HTTPValidationError |
| GET | `/sessions/{chat_session_id}/export` | export_session_sessions__chat_session_id__export_get | - | #/components/schemas/HTTPValidationError |
| GET | `/sessions/{chat_session_id}/guidance-snapshot` | get_guidance_snapshot_sessions__chat_session_id__guidance_snapshot_get | - | #/components/schemas/HTTPValidationError |
| GET | `/sessions/{chat_session_id}/step7-health` | get_step7_health_sessions__chat_session_id__step7_health_get | - | #/components/schemas/HTTPValidationError |
| POST | `/admin/reindex` | admin_reindex_admin_reindex_post | #/components/schemas/ReindexRequest | #/components/schemas/HTTPValidationError |
| POST | `/admin/rescan` | admin_rescan_admin_rescan_post | #/components/schemas/RescanRequest | #/components/schemas/HTTPValidationError |
| POST | `/admin/session-migrate` | admin_session_migrate_admin_session_migrate_post | #/components/schemas/SessionMigrateRequest | #/components/schemas/HTTPValidationError |
| POST | `/import/hypamemory` | import_hypamemory_import_hypamemory_post | #/components/schemas/HypaImportRequest | #/components/schemas/HTTPValidationError |
| GET | `/audit` | get_audit_logs_audit_get | - | #/components/schemas/HTTPValidationError |
| POST | `/feedback` | post_feedback_feedback_post | #/components/schemas/FeedbackRequest | #/components/schemas/HTTPValidationError |
| GET | `/feedback/latest` | get_feedback_latest_feedback_latest_get | - | #/components/schemas/HTTPValidationError |
| GET | `/sessions/compare` | compare_sessions_sessions_compare_get | - | #/components/schemas/HTTPValidationError |
| GET | `/active-states/{chat_session_id}` | get_active_states_active_states__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/canonical-state-layer/{chat_session_id}` | get_canonical_state_layer_canonical_state_layer__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/episodes/{chat_session_id}` | get_episodes_episodes__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| POST | `/episodes/generate` | generate_episode_episodes_generate_post | #/components/schemas/EpisodeGenerateRequest | #/components/schemas/HTTPValidationError |
| POST | `/chapters/generate` | generate_chapter_chapters_generate_post | #/components/schemas/ChapterGenerateRequest | #/components/schemas/HTTPValidationError |
| POST | `/arcs/generate` | generate_arc_arcs_generate_post | #/components/schemas/ArcGenerateRequest | #/components/schemas/HTTPValidationError |
| POST | `/sagas/generate` | generate_saga_sagas_generate_post | #/components/schemas/SagaGenerateRequest | #/components/schemas/HTTPValidationError |
| POST | `/chapters/dry-run` | chapter_generation_dry_run_chapters_dry_run_post | #/components/schemas/ChapterDryRunRequest | #/components/schemas/HTTPValidationError |
| POST | `/chapters/search` | search_chapters_chapters_search_post | #/components/schemas/ChapterSearchRequest | #/components/schemas/HTTPValidationError |
| POST | `/episodes/search` | search_episodes_episodes_search_post | #/components/schemas/EpisodeSearchRequest | #/components/schemas/HTTPValidationError |
| GET | `/episodes/detail/{episode_id}` | get_episode_detail_episodes_detail__episode_id__get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/episodes/{episode_id}` | patch_episode_episodes__episode_id__patch | #/components/schemas/PatchEpisodeRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/episodes/{episode_id}` | delete_episode_episodes__episode_id__delete | - | #/components/schemas/HTTPValidationError |
| POST | `/episodes/regenerate` | regenerate_episode_episodes_regenerate_post | #/components/schemas/EpisodeGenerateRequest | #/components/schemas/HTTPValidationError |
| POST | `/episodes/merge` | merge_episodes_endpoint_episodes_merge_post | #/components/schemas/EpisodeMergeRequest | #/components/schemas/HTTPValidationError |
| GET | `/storylines/{chat_session_id}` | get_storylines_storylines__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/storylines/{storyline_id}` | patch_storyline_storylines__storyline_id__patch | #/components/schemas/PatchStorylineRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/storylines/{storyline_id}` | delete_storyline_storylines__storyline_id__delete | - | #/components/schemas/HTTPValidationError |
| PATCH | `/storylines/{storyline_id}/trust` | patch_storyline_trust_storylines__storyline_id__trust_patch | #/components/schemas/TrustControlRequest | #/components/schemas/HTTPValidationError |
| POST | `/storylines/sync` | sync_storylines_storylines_sync_post | #/components/schemas/StorylineSyncRequest | #/components/schemas/HTTPValidationError |
| GET | `/characters/{chat_session_id}` | get_characters_characters__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/characters/{chat_session_id}/{character_name}` | get_character_detail_characters__chat_session_id___character_name__get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/characters/{chat_session_id}/{character_name}` | patch_character_characters__chat_session_id___character_name__patch | #/components/schemas/PatchCharacterRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/characters/{chat_session_id}/{character_name}` | delete_character_characters__chat_session_id___character_name__delete | - | #/components/schemas/HTTPValidationError |
| GET | `/characters/{chat_session_id}/{character_name}/events` | get_character_events_characters__chat_session_id___character_name__events_get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/characters/{chat_session_id}/{character_name}/speech` | patch_character_speech_characters__chat_session_id___character_name__speech_patch | #/components/schemas/PatchSpeechStyleRequest | #/components/schemas/HTTPValidationError |
| GET | `/session/{chat_session_id}/active-scope` | get_active_scope_session__chat_session_id__active_scope_get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/session/{chat_session_id}/active-scope` | set_active_scope_session__chat_session_id__active_scope_patch | #/components/schemas/ActiveScopeRequest | #/components/schemas/HTTPValidationError |
| GET | `/world-rules/{chat_session_id}` | get_world_rules_world_rules__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/world-rules/{chat_session_id}/inherited` | get_inherited_world_rules_world_rules__chat_session_id__inherited_get | - | #/components/schemas/HTTPValidationError |
| POST | `/world-rules/sync` | sync_world_rules_world_rules_sync_post | #/components/schemas/WorldRuleSyncRequest | #/components/schemas/HTTPValidationError |
| PATCH | `/world-rules/{rule_id}` | patch_world_rule_world_rules__rule_id__patch | #/components/schemas/PatchWorldRuleRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/world-rules/{rule_id}` | delete_world_rule_world_rules__rule_id__delete | - | #/components/schemas/HTTPValidationError |
| PATCH | `/world-rules/{rule_id}/trust` | patch_world_rule_trust_world_rules__rule_id__trust_patch | #/components/schemas/TrustControlRequest | #/components/schemas/HTTPValidationError |
| GET | `/continuity-pack/{chat_session_id}` | get_continuity_pack_continuity_pack__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/pending-threads/{chat_session_id}` | get_pending_threads_pending_threads__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| PATCH | `/pending-threads/{hook_id}` | patch_continuity_hook_pending_threads__hook_id__patch | #/components/schemas/PatchPendingThreadRequest | #/components/schemas/HTTPValidationError |
| DELETE | `/pending-threads/{hook_id}` | delete_continuity_hook_pending_threads__hook_id__delete | - | #/components/schemas/HTTPValidationError |
| PATCH | `/pending-threads/{hook_id}/trust` | patch_continuity_hook_trust_pending_threads__hook_id__trust_patch | #/components/schemas/TrustControlRequest | #/components/schemas/HTTPValidationError |
| GET | `/session-state/{chat_session_id}` | get_session_state_session_state__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1c/{chat_session_id}` | get_lc1c_memory_footprint_metrics_lc1c__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1d/{chat_session_id}` | get_lc1d_integrity_replay_metrics_lc1d__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1e/{chat_session_id}` | get_lc1e_context_budget_comparison_metrics_lc1e__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1f/{chat_session_id}` | get_lc1f_non_regression_check_metrics_lc1f__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1g/{chat_session_id}` | get_lc1g_accuracy_replay_metrics_lc1g__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1h/{chat_session_id}` | get_lc1h_forgetting_hallucination_replay_metrics_lc1h__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1i/{chat_session_id}` | get_lc1i_ablation_compare_metrics_lc1i__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1j/{chat_session_id}` | get_lc1j_imported_idea_gate_metrics_lc1j__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1k/{chat_session_id}` | get_lc1k_priority_budget_hybrid_replay_metrics_lc1k__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1l/{chat_session_id}` | get_lc1l_imported_idea_contract_gate_metrics_lc1l__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1m/{chat_session_id}` | get_lc1m_single_call_vs_split_comparison_metrics_lc1m__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1n/{chat_session_id}` | get_lc1n_rebuild_backfill_rehearsal_metrics_lc1n__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1o/{chat_session_id}` | get_lc1o_deterministic_preview_ledger_lightweight_validation_metrics_lc1o__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1p/{chat_session_id}` | get_lc1p_evaluation_split_summary_metrics_lc1p__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1q/{chat_session_id}` | get_lc1q_freshness_lag_summary_metrics_lc1q__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/metrics/lc1r/regression-corpus` | get_lc1r_regression_corpus_manifest_metrics_lc1r_regression_corpus_get | - | - |
| GET | `/metrics/lc1s/step17-bundle-closure` | get_lc1s_step17_bundle_closure_metrics_lc1s_step17_bundle_closure_get | - | - |
| GET | `/metrics/tm1d/{chat_session_id}` | get_tm1d_truth_maintenance_audit_replay_metrics_tm1d__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/momentum-packet/{chat_session_id}` | get_momentum_packet_momentum_packet__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| GET | `/narrative-control/{chat_session_id}` | get_narrative_control_narrative_control__chat_session_id__get | - | #/components/schemas/HTTPValidationError |
| POST | `/complete-turn` | complete_turn_m4_complete_turn_post | #/components/schemas/M4CompleteTurnRequest | #/components/schemas/M4CompleteTurnResponse, #/components/schemas/HTTPValidationError |
| POST | `/prepare-turn` | prepare_turn_prepare_turn_post | #/components/schemas/PrepareTurnRequest | #/components/schemas/HTTPValidationError |

## OpenAPI Warnings (1)
- **UserWarning**: Duplicate Operation ID get_retrieval_index_runtime_config_retrieval_index_runtime_config_get for function get_retrieval_index_runtime_config at M:\risulongmemory\Archive Center Beta 0.8(fix)\backend\main.py
