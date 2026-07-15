# Go DTO Mapping Plan

> Generated from frozen OpenAPI schema (62 schemas)

## Mapping Policy

| OpenAPI Construct | Proposed Go Type | Strategy |
|-------------------|------------------|----------|
| `string` | string | - |
| `integer_small` | int (local indices: turn_index, page_index, etc.) | - |
| `integer_large` | int64 (token/count/timeout/budget/limit/offset/id/size/length/timestamp/duration) | - |
| `number` | float64 | - |
| `boolean` | bool | - |
| `nullable_anyOf_T_null` | pointer *T | - |
| `nullable_ref` | pointer *RefName | - |
| `non_null_union_anyOf` | json.RawMessage (custom union policy needed) | - |
| `oneOf` | json.RawMessage (blocker: oneOf not supported) | - |
| `allOf` | json.RawMessage (blocker: allOf not supported) | - |
| `additionalProperties_true` | map[string]any | - |
| `additionalProperties_typed` | map[string]T where T is mapped from schema | - |
| `object_without_properties` | map[string]any (blocker: loose object) | - |
| `array_with_items` | []T where T is mapped from items | - |
| `array_without_items` | blocker | - |
| `ref` | referenced DTO type | - |
| `inline_object` | map[string]any (blocker: needs named schema) | - |
| `inline_object_with_additionalProperties` | map[string]any (blocker: fidelity loss) | - |
| `untyped` | any (blocker: untyped property) | - |

## Per-Schema Summary

| Schema | Fields | Required | Blockers | Routes |
|--------|--------|----------|----------|--------|
| ActiveScopeRequest | 2 | 1 | 0 | 1 |
| ArcGenerateRequest | 4 | 0 | 0 | 1 |
| ChapterDryRunRequest | 4 | 0 | 0 | 1 |
| ChapterGenerateRequest | 4 | 0 | 0 | 1 |
| ChapterSearchRequest | 3 | 0 | 0 | 1 |
| ChatLogRepairEntryRequest | 5 | 1 | 0 | 0 |
| ChatLogRepairReplayRequest | 3 | 0 | 0 | 1 |
| ChromaShadowAdoptionGateRequest | 2 | 0 | 0 | 1 |
| ChromaShadowBackfillBatchRequest | 4 | 0 | 0 | 1 |
| ChromaShadowBackfillDryRunRequest | 2 | 0 | 0 | 1 |
| ChromaShadowFallbackRunbookRequest | 1 | 0 | 0 | 1 |
| ChromaShadowHealthProbeRequest | 1 | 0 | 0 | 1 |
| ChromaShadowRebuildDrillRequest | 5 | 0 | 0 | 1 |
| ChromaShadowReembedAuditRequest | 2 | 0 | 0 | 1 |
| ChromaShadowReleaseHygieneRequest | 3 | 0 | 0 | 1 |
| ChromaShadowVisibilityGuardRequest | 2 | 0 | 0 | 1 |
| CompleteTurnRequest | 5 | 2 | 0 | 1 |
| CriticTestRequest | 5 | 2 | 0 | 1 |
| DirectEvidenceRevalidateRequest | 2 | 1 | 0 | 1 |
| DirectorPatchRequest | 7 | 0 | 0 | 1 |
| EpisodeGenerateRequest | 4 | 0 | 0 | 2 |
| EpisodeMergeRequest | 3 | 0 | 0 | 1 |
| EpisodeSearchRequest | 3 | 0 | 0 | 1 |
| FeedbackRequest | 5 | 4 | 0 | 1 |
| HTTPValidationError | 1 | 0 | 0 | 119 |
| HypaImportRequest | 2 | 2 | 0 | 1 |
| HypaImportSummary | 4 | 1 | 0 | 0 |
| IntentRoutingRuntimeConfigRequest | 1 | 0 | 0 | 1 |
| KGRecallRequest | 4 | 0 | 0 | 1 |
| M4CompleteTurnRequest | 9 | 2 | 0 | 1 |
| M4CompleteTurnResponse | 15 | 0 | 0 | 1 |
| MaintenanceEnqueueRequest | 7 | 1 | 0 | 1 |
| MaintenanceEnqueueResponse | 4 | 3 | 0 | 1 |
| MaintenancePassRequest | 5 | 0 | 0 | 1 |
| PatchCharacterRequest | 5 | 0 | 0 | 1 |
| PatchDirectEvidenceReviewRequest | 6 | 1 | 0 | 1 |
| PatchDirectEvidenceSupersedeRequest | 3 | 2 | 0 | 1 |
| PatchDirectEvidenceTombstoneRequest | 3 | 1 | 0 | 1 |
| PatchEpisodeRequest | 7 | 0 | 0 | 1 |
| PatchKGTripleRequest | 6 | 1 | 2 | 1 |
| PatchMemoryRequest | 5 | 1 | 0 | 1 |
| PatchPendingThreadRequest | 8 | 0 | 0 | 1 |
| PatchSpeechStyleRequest | 3 | 0 | 0 | 1 |
| PatchStorylineRequest | 11 | 0 | 0 | 1 |
| PatchWorldRuleRequest | 6 | 0 | 0 | 1 |
| PrepareTurnRequest | 9 | 1 | 0 | 1 |
| PrepareTurnSettings | 14 | 0 | 0 | 0 |
| PromptUpdateRequest | 1 | 0 | 0 | 1 |
| ProxyPluginMainRequest | 14 | 1 | 1 | 1 |
| ReindexRequest | 4 | 1 | 0 | 1 |
| RescanRequest | 3 | 1 | 0 | 1 |
| RetrievalIndexRuntimeConfigRequest | 1 | 0 | 0 | 1 |
| SagaGenerateRequest | 4 | 0 | 0 | 1 |
| SaveEffectiveInputRequest | 3 | 1 | 0 | 1 |
| SaveTurnRequest | 4 | 1 | 0 | 1 |
| SearchRequest | 4 | 1 | 0 | 1 |
| SessionMigrateRequest | 7 | 2 | 0 | 1 |
| StorylineSyncRequest | 4 | 0 | 0 | 1 |
| SupervisorRequest | 11 | 0 | 0 | 1 |
| TrustControlRequest | 3 | 0 | 0 | 3 |
| ValidationError | 5 | 3 | 3 | 0 |
| WorldRuleSyncRequest | 4 | 2 | 0 | 1 |

## ActiveScopeRequest

- **Fields**: 2
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /session/{chat_session_id}/active-scope`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `active_scope` | ActiveScope | Yes | No | No | - | `string` | direct | - | - | - |
| `scope_name,omitempty` | ScopeName | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## ArcGenerateRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /arcs/generate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `from_turn,omitempty` | FromTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `to_turn,omitempty` | ToTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |

## ChapterDryRunRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chapters/dry-run`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `interval,omitempty` | Interval | No | No | Yes | 60 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (60): Go handler must apply default when f... |
| `turn_index,omitempty` | TurnIndex | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |

## ChapterGenerateRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chapters/generate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `from_turn,omitempty` | FromTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `to_turn,omitempty` | ToTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |

## ChapterSearchRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chapters/search`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `query,omitempty` | Query | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `top_k,omitempty` | TopK | No | No | Yes | 3 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (3): Go handler must apply default when fi... |

## ChatLogRepairEntryRequest

- **Fields**: 5
- **Required**: 1
- **Blockers**: 0
- **Routes**: none

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `assistant_content,omitempty` | AssistantContent | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `created_at,omitempty` | CreatedAt | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `source,omitempty` | Source | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |
| `user_content,omitempty` | UserContent | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## ChatLogRepairReplayRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /turns/repair-replay`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `dry_run,omitempty` | DryRun | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `entries,omitempty` | Entries | No | No | No | - | `[]ChatLogRepairEntryRequest` | array_of | - | - | - |

## ChromaShadowAdoptionGateRequest

- **Fields**: 2
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/adoption-gate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `operator_evidence,omitempty` | OperatorEvidence | No | No | No | - | `map[string]any` | map_string_any | - | - | - |

## ChromaShadowBackfillBatchRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/backfill-batch`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `batch_size_per_tier,omitempty` | BatchSizePerTier | No | No | Yes | 25 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (25): Go handler must apply default when f... |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `checkpoint,omitempty` | Checkpoint | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `retry_rows,omitempty` | RetryRows | No | No | No | - | `[]map[string]any` | array_of | - | - | - |

## ChromaShadowBackfillDryRunRequest

- **Fields**: 2
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/backfill-dry-run`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `sample_limit_per_tier,omitempty` | SampleLimitPerTier | No | No | Yes | 2 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (2): Go handler must apply default when fi... |

## ChromaShadowFallbackRunbookRequest

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/fallback-runbook`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## ChromaShadowHealthProbeRequest

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/health-probe`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## ChromaShadowRebuildDrillRequest

- **Fields**: 5
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/rebuild-drill`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `batch_size_per_tier,omitempty` | BatchSizePerTier | No | No | Yes | 25 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (25): Go handler must apply default when f... |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `checkpoint,omitempty` | Checkpoint | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `execute_rebuild_then_rollback,omitempty` | ExecuteRebuildThenRollback | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `retry_rows,omitempty` | RetryRows | No | No | No | - | `[]map[string]any` | array_of | - | - | - |

## ChromaShadowReembedAuditRequest

- **Fields**: 2
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/reembed-audit`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `queue_limit_per_tier,omitempty` | QueueLimitPerTier | No | No | Yes | 25 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (25): Go handler must apply default when f... |

## ChromaShadowReleaseHygieneRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/release-hygiene`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `adoption_operator_evidence,omitempty` | AdoptionOperatorEvidence | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `release_evidence,omitempty` | ReleaseEvidence | No | No | No | - | `map[string]any` | map_string_any | - | - | - |

## ChromaShadowVisibilityGuardRequest

- **Fields**: 2
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /chroma-shadow/visibility-guard`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `sample_limit,omitempty` | SampleLimit | No | No | Yes | 20 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (20): Go handler must apply default when f... |

## CompleteTurnRequest

- **Fields**: 5
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /turns/complete`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `context,omitempty` | Context | No | No | No | - | `[]map[string]any` | array_of | - | - | - |
| `output_language_override,omitempty` | OutputLanguageOverride | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `turn_content` | TurnContent | Yes | No | No | - | `string` | direct | - | - | - |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |

## CriticTestRequest

- **Fields**: 5
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /critic/test`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `context,omitempty` | Context | No | No | No | - | `[]map[string]any` | array_of | - | - | - |
| `output_language_override,omitempty` | OutputLanguageOverride | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `turn_content` | TurnContent | Yes | No | No | - | `string` | direct | - | - | - |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |

## DirectEvidenceRevalidateRequest

- **Fields**: 2
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /explorer/direct-evidence/{record_id}/revalidate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `review_note,omitempty` | ReviewNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## DirectorPatchRequest

- **Fields**: 7
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /narrative-control/{chat_session_id}/director-patch`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `execution_checklist,omitempty` | ExecutionChecklist | No | Yes | No | - | `*[]string` | nullable_pointer | - | - | - |
| `forbidden_moves,omitempty` | ForbiddenMoves | No | Yes | No | - | `*[]string` | nullable_pointer | - | - | - |
| `persona_guardrails,omitempty` | PersonaGuardrails | No | Yes | No | - | `*[]string` | nullable_pointer | - | - | - |
| `pressure_level,omitempty` | PressureLevel | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `required_outcomes,omitempty` | RequiredOutcomes | No | Yes | No | - | `*[]string` | nullable_pointer | - | - | - |
| `scene_mandate,omitempty` | SceneMandate | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `world_guardrails,omitempty` | WorldGuardrails | No | Yes | No | - | `*[]string` | nullable_pointer | - | - | - |

## EpisodeGenerateRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /episodes/generate`
  - REQUEST: `POST /episodes/regenerate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `from_turn,omitempty` | FromTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `to_turn,omitempty` | ToTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |

## EpisodeMergeRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /episodes/merge`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `episode_ids,omitempty` | EpisodeIds | No | No | No | - | `[]int` | array_of | - | - | - |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |

## EpisodeSearchRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /episodes/search`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `query,omitempty` | Query | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `top_k,omitempty` | TopK | No | No | Yes | 3 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (3): Go handler must apply default when fi... |

## FeedbackRequest

- **Fields**: 5
- **Required**: 4
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /feedback`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `feedback_note,omitempty` | FeedbackNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `feedback_value` | FeedbackValue | Yes | No | No | - | `string` | direct | - | - | - |
| `target_id` | TargetID | Yes | No | No | - | `int64` | direct | - | - | - |
| `target_type` | TargetType | Yes | No | No | - | `string` | direct | - | - | - |

## HTTPValidationError

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - RESPONSE: `POST /proxy/plugin-main`
  - RESPONSE: `POST /config/update`
  - RESPONSE: `PATCH /narrative-control/{chat_session_id}/director-patch`
  - RESPONSE: `POST /maintenance-pass/{chat_session_id}`
  - RESPONSE: `POST /maintenance/enqueue`
  - RESPONSE: `GET /maintenance/queue-status`
  - RESPONSE: `GET /long-session-health/{session_id}`
  - RESPONSE: `POST /turns`
  - RESPONSE: `POST /turns/repair-replay`
  - RESPONSE: `POST /effective-inputs`
  - RESPONSE: `DELETE /rollback/{turn_index}`
  - RESPONSE: `POST /search`
  - RESPONSE: `POST /retrieval-index/runtime-config`
  - RESPONSE: `POST /chroma-shadow/backfill-dry-run`
  - RESPONSE: `POST /chroma-shadow/backfill-batch`
  - RESPONSE: `POST /chroma-shadow/reembed-audit`
  - RESPONSE: `POST /chroma-shadow/health-probe`
  - RESPONSE: `POST /chroma-shadow/fallback-runbook`
  - RESPONSE: `POST /chroma-shadow/rebuild-drill`
  - RESPONSE: `POST /chroma-shadow/adoption-gate`
  - RESPONSE: `POST /chroma-shadow/release-hygiene`
  - RESPONSE: `POST /chroma-shadow/visibility-guard`
  - RESPONSE: `POST /intent-routing/runtime-config`
  - RESPONSE: `GET /retrieval-index/{chat_session_id}`
  - RESPONSE: `GET /retrieval-index/{chat_session_id}/source-row`
  - RESPONSE: `POST /critic/test`
  - RESPONSE: `POST /turns/complete`
  - RESPONSE: `POST /kg/recall`
  - RESPONSE: `GET /kg/recall`
  - RESPONSE: `POST /supervisor`
  - RESPONSE: `GET /prompts/{prompt_name}`
  - RESPONSE: `PUT /prompts/{prompt_name}`
  - RESPONSE: `GET /explorer/chat_logs`
  - RESPONSE: `GET /explorer/memories`
  - RESPONSE: `GET /explorer/direct-evidence`
  - RESPONSE: `GET /explorer/kg_triples`
  - RESPONSE: `GET /explorer/chapter_summaries`
  - RESPONSE: `PATCH /explorer/memories/{memory_id}`
  - RESPONSE: `DELETE /explorer/memories/{memory_id}`
  - RESPONSE: `PATCH /explorer/kg_triples/{triple_id}`
  - RESPONSE: `DELETE /explorer/kg_triples/{triple_id}`
  - RESPONSE: `PATCH /explorer/direct-evidence/{record_id}/review`
  - RESPONSE: `PATCH /explorer/direct-evidence/{record_id}/revalidate`
  - RESPONSE: `PATCH /explorer/direct-evidence/{record_id}/tombstone`
  - RESPONSE: `PATCH /explorer/direct-evidence/{record_id}/supersede`
  - RESPONSE: `POST /explorer/memories/regenerate`
  - RESPONSE: `POST /explorer/memories/{memory_id}/delete`
  - RESPONSE: `POST /explorer/kg_triples/{triple_id}/delete`
  - RESPONSE: `GET /sessions/{chat_session_id}/export`
  - RESPONSE: `GET /sessions/{chat_session_id}/guidance-snapshot`
  - RESPONSE: `GET /sessions/{chat_session_id}/step7-health`
  - RESPONSE: `POST /admin/reindex`
  - RESPONSE: `POST /admin/rescan`
  - RESPONSE: `POST /admin/session-migrate`
  - RESPONSE: `POST /import/hypamemory`
  - RESPONSE: `GET /audit`
  - RESPONSE: `POST /feedback`
  - RESPONSE: `GET /feedback/latest`
  - RESPONSE: `GET /sessions/compare`
  - RESPONSE: `GET /active-states/{chat_session_id}`
  - RESPONSE: `GET /canonical-state-layer/{chat_session_id}`
  - RESPONSE: `GET /episodes/{chat_session_id}`
  - RESPONSE: `POST /episodes/generate`
  - RESPONSE: `POST /chapters/generate`
  - RESPONSE: `POST /arcs/generate`
  - RESPONSE: `POST /sagas/generate`
  - RESPONSE: `POST /chapters/dry-run`
  - RESPONSE: `POST /chapters/search`
  - RESPONSE: `POST /episodes/search`
  - RESPONSE: `GET /episodes/detail/{episode_id}`
  - RESPONSE: `PATCH /episodes/{episode_id}`
  - RESPONSE: `DELETE /episodes/{episode_id}`
  - RESPONSE: `POST /episodes/regenerate`
  - RESPONSE: `POST /episodes/merge`
  - RESPONSE: `GET /storylines/{chat_session_id}`
  - RESPONSE: `PATCH /storylines/{storyline_id}`
  - RESPONSE: `DELETE /storylines/{storyline_id}`
  - RESPONSE: `PATCH /storylines/{storyline_id}/trust`
  - RESPONSE: `POST /storylines/sync`
  - RESPONSE: `GET /characters/{chat_session_id}`
  - RESPONSE: `GET /characters/{chat_session_id}/{character_name}`
  - RESPONSE: `PATCH /characters/{chat_session_id}/{character_name}`
  - RESPONSE: `DELETE /characters/{chat_session_id}/{character_name}`
  - RESPONSE: `GET /characters/{chat_session_id}/{character_name}/events`
  - RESPONSE: `PATCH /characters/{chat_session_id}/{character_name}/speech`
  - RESPONSE: `GET /session/{chat_session_id}/active-scope`
  - RESPONSE: `PATCH /session/{chat_session_id}/active-scope`
  - RESPONSE: `GET /world-rules/{chat_session_id}`
  - RESPONSE: `GET /world-rules/{chat_session_id}/inherited`
  - RESPONSE: `POST /world-rules/sync`
  - RESPONSE: `PATCH /world-rules/{rule_id}`
  - RESPONSE: `DELETE /world-rules/{rule_id}`
  - RESPONSE: `PATCH /world-rules/{rule_id}/trust`
  - RESPONSE: `GET /continuity-pack/{chat_session_id}`
  - RESPONSE: `GET /pending-threads/{chat_session_id}`
  - RESPONSE: `PATCH /pending-threads/{hook_id}`
  - RESPONSE: `DELETE /pending-threads/{hook_id}`
  - RESPONSE: `PATCH /pending-threads/{hook_id}/trust`
  - RESPONSE: `GET /session-state/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1c/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1d/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1e/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1f/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1g/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1h/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1i/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1j/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1k/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1l/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1m/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1n/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1o/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1p/{chat_session_id}`
  - RESPONSE: `GET /metrics/lc1q/{chat_session_id}`
  - RESPONSE: `GET /metrics/tm1d/{chat_session_id}`
  - RESPONSE: `GET /momentum-packet/{chat_session_id}`
  - RESPONSE: `GET /narrative-control/{chat_session_id}`
  - RESPONSE: `POST /complete-turn`
  - RESPONSE: `POST /prepare-turn`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `detail,omitempty` | Detail | No | No | No | - | `[]ValidationError` | array_of | - | - | - |

## HypaImportRequest

- **Fields**: 2
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /import/hypamemory`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `summaries` | Summaries | Yes | No | No | - | `[]HypaImportSummary` | array_of | - | - | - |

## HypaImportSummary

- **Fields**: 4
- **Required**: 1
- **Blockers**: 0
- **Routes**: none

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `category,omitempty` | Category | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `is_important,omitempty` | IsImportant | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `tags,omitempty` | Tags | No | No | No | - | `[]string` | array_of | - | - | - |
| `text` | Text | Yes | No | No | - | `string` | direct | - | - | - |

## IntentRoutingRuntimeConfigRequest

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /intent-routing/runtime-config`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `mode,omitempty` | Mode | No | No | Yes | "single_query_shared" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("single_query_shared"): Go handler must a... |

## KGRecallRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /kg/recall`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `current_turn,omitempty` | CurrentTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `entities,omitempty` | Entities | No | No | No | - | `[]string` | array_of | - | - | - |
| `limit,omitempty` | Limit | No | No | Yes | 20 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (20): Go handler must apply default when f... |

## M4CompleteTurnRequest

- **Fields**: 9
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /complete-turn`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `assistant_content,omitempty` | AssistantContent | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `client_meta,omitempty` | ClientMeta | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `context_messages,omitempty` | ContextMessages | No | No | No | - | `[]map[string]any` | array_of | - | - | - |
| `improvement_trace,omitempty` | ImprovementTrace | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `output_language_override,omitempty` | OutputLanguageOverride | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `request_type,omitempty` | RequestType | No | No | Yes | "model" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("model"): Go handler must apply default w... |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |
| `user_input,omitempty` | UserInput | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## M4CompleteTurnResponse

- **Fields**: 15
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - RESPONSE: `POST /complete-turn`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chapter_result,omitempty` | ChapterResult | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `critic_result,omitempty` | CriticResult | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `critic_triggered,omitempty` | CriticTriggered | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `episode_result,omitempty` | EpisodeResult | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `fail_reasons,omitempty` | FailReasons | No | No | No | - | `[]string` | array_of | - | - | - |
| `generated_at,omitempty` | GeneratedAt | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `maintenance_enqueued,omitempty` | MaintenanceEnqueued | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `save_error,omitempty` | SaveError | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `save_ok,omitempty` | SaveOk | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `status,omitempty` | Status | No | No | Yes | "ok" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("ok"): Go handler must apply default when... |
| `summary_fallback,omitempty` | SummaryFallback | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `trace_handoff,omitempty` | TraceHandoff | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `warnings,omitempty` | Warnings | No | No | No | - | `[]string` | array_of | - | - | - |

## MaintenanceEnqueueRequest

- **Fields**: 7
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /maintenance/enqueue`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `assistant_response,omitempty` | AssistantResponse | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `recent_responses,omitempty` | RecentResponses | No | No | No | - | `[]string` | array_of | - | - | - |
| `shadow_only,omitempty` | ShadowOnly | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |
| `summary_refresh,omitempty` | SummaryRefresh | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `supervisor_result,omitempty` | SupervisorResult | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | No | Yes | -1 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (-1): Go handler must apply default when f... |

## MaintenanceEnqueueResponse

- **Fields**: 4
- **Required**: 3
- **Blockers**: 0
- **Routes**:
  - RESPONSE: `POST /maintenance/enqueue`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `queue_depth,omitempty` | QueueDepth | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `status` | Status | Yes | No | No | - | `string` | direct | - | - | - |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |

## MaintenancePassRequest

- **Fields**: 5
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /maintenance-pass/{chat_session_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `assistant_response,omitempty` | AssistantResponse | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `recent_responses,omitempty` | RecentResponses | No | No | No | - | `[]string` | array_of | - | - | - |
| `shadow_only,omitempty` | ShadowOnly | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |
| `supervisor_result,omitempty` | SupervisorResult | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | No | Yes | -1 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (-1): Go handler must apply default when f... |

## PatchCharacterRequest

- **Fields**: 5
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /characters/{chat_session_id}/{character_name}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `appearance_json,omitempty` | AppearanceJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `personality_json,omitempty` | PersonalityJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `relationships_json,omitempty` | RelationshipsJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `speech_style_json,omitempty` | SpeechStyleJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `status_json,omitempty` | StatusJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchDirectEvidenceReviewRequest

- **Fields**: 6
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /explorer/direct-evidence/{record_id}/review`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `archive_state,omitempty` | ArchiveState | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `capture_verification,omitempty` | CaptureVerification | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `committed_gate,omitempty` | CommittedGate | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `repair_needed,omitempty` | RepairNeeded | No | Yes | No | - | `*bool` | nullable_pointer | - | - | - |
| `review_note,omitempty` | ReviewNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchDirectEvidenceSupersedeRequest

- **Fields**: 3
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /explorer/direct-evidence/{record_id}/supersede`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `review_note,omitempty` | ReviewNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `superseded_by_id` | SupersededByID | Yes | No | No | - | `int64` | direct | - | - | - |

## PatchDirectEvidenceTombstoneRequest

- **Fields**: 3
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /explorer/direct-evidence/{record_id}/tombstone`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `review_note,omitempty` | ReviewNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `tombstoned,omitempty` | Tombstoned | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |

## PatchEpisodeRequest

- **Fields**: 7
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /episodes/{episode_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `from_turn,omitempty` | FromTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `key_entities,omitempty` | KeyEntities | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `key_events,omitempty` | KeyEvents | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `open_loops_json,omitempty` | OpenLoopsJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `relationship_changes_json,omitempty` | RelationshipChangesJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `summary_text,omitempty` | SummaryText | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `to_turn,omitempty` | ToTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |

## PatchKGTripleRequest

- **Fields**: 6
- **Required**: 1
- **Blockers**: 2
- **Routes**:
  - REQUEST: `PATCH /explorer/kg_triples/{triple_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `object,omitempty` | Object | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `predicate,omitempty` | Predicate | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `subject,omitempty` | Subject | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `valid_from,omitempty` | ValidFrom | No | No | No | - | `json.RawMessage` | union_json_rawmessage | PatchKGTripleRequest.properties.valid_from: non-null union anyOf | - | - |
| `valid_to,omitempty` | ValidTo | No | No | No | - | `json.RawMessage` | union_json_rawmessage | PatchKGTripleRequest.properties.valid_to: non-null union anyOf | - | - |

## PatchMemoryRequest

- **Fields**: 5
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /explorer/memories/{memory_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `archive_room,omitempty` | ArchiveRoom | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `archive_wing,omitempty` | ArchiveWing | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `importance,omitempty` | Importance | No | Yes | No | - | `*float64` | nullable_pointer | - | - | - |
| `summary_json,omitempty` | SummaryJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchPendingThreadRequest

- **Fields**: 8
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /pending-threads/{hook_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `confidence,omitempty` | Confidence | No | Yes | No | - | `*float64` | nullable_pointer | - | - | - |
| `details_json,omitempty` | DetailsJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `owner,omitempty` | Owner | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `resolution_note,omitempty` | ResolutionNote | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `status,omitempty` | Status | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `target,omitempty` | Target | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `thread_type,omitempty` | ThreadType | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `title,omitempty` | Title | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchSpeechStyleRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /characters/{chat_session_id}/{character_name}/speech`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `default_tone,omitempty` | DefaultTone | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `honorific_style,omitempty` | HonorificStyle | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `speech_notes,omitempty` | SpeechNotes | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchStorylineRequest

- **Fields**: 11
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /storylines/{storyline_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `confidence,omitempty` | Confidence | No | Yes | No | - | `*float64` | nullable_pointer | - | - | - |
| `current_context,omitempty` | CurrentContext | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `entities_json,omitempty` | EntitiesJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `evidence_count,omitempty` | EvidenceCount | No | Yes | No | - | `*int64` | nullable_pointer | - | - | - |
| `first_turn,omitempty` | FirstTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `key_points_json,omitempty` | KeyPointsJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `last_evidence_turn,omitempty` | LastEvidenceTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `last_turn,omitempty` | LastTurn | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `name,omitempty` | Name | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `ongoing_tensions_json,omitempty` | OngoingTensionsJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `status,omitempty` | Status | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PatchWorldRuleRequest

- **Fields**: 6
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /world-rules/{rule_id}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `category,omitempty` | Category | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `genre,omitempty` | Genre | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `key,omitempty` | Key | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `scope,omitempty` | Scope | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `scope_name,omitempty` | ScopeName | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `value_json,omitempty` | ValueJSON | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## PrepareTurnRequest

- **Fields**: 9
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /prepare-turn`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `client_meta,omitempty` | ClientMeta | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `continuity_query,omitempty` | ContinuityQuery | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `continuity_trigger_mode,omitempty` | ContinuityTriggerMode | No | No | Yes | "none" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("none"): Go handler must apply default wh... |
| `messages,omitempty` | Messages | No | No | No | - | `[]map[string]any` | array_of | - | - | - |
| `raw_user_input,omitempty` | RawUserInput | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `request_type,omitempty` | RequestType | No | No | Yes | "model" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("model"): Go handler must apply default w... |
| `settings,omitempty` | Settings | No | No | No | - | `PrepareTurnSettings` | ref | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |

## PrepareTurnSettings

- **Fields**: 11
- **Required**: 0
- **Blockers**: 0
- **Routes**: none

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `apply_mode,omitempty` | ApplyMode | No | No | Yes | "shadow" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("shadow"): Go handler must apply default ... |
| `episode_interval_turns,omitempty` | EpisodeIntervalTurns | No | No | Yes | 10 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (10): Go handler must apply default when f... |
| `guide_mode,omitempty` | GuideMode | No | No | Yes | "off" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("off"): Go handler must apply default whe... |
| `injection_enabled,omitempty` | InjectionEnabled | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |
| `input_context_enabled,omitempty` | InputContextEnabled | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |
| `max_injection_chars,omitempty` | MaxInjectionChars | No | No | Yes | 3000 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (3000): Go handler must apply default when... |
| `reference_injection_budget_basis_chars,omitempty` | ReferenceInjectionBudgetBasisChars | No | No | No | null | `int` | direct | - | Configured memory cap used as the independent reference budget basis. | Absent callers use effective max_injection_chars; host sends the configured value across first-turn suppression. |
| `reference_recall_limit,omitempty` | ReferenceRecallLimit | No | No | No | null | `int` | direct | - | Optional candidate limit used only by original-work reference recall. | Absent callers inherit effective top_k; explicit zero is preserved and negative values are clamped to zero. |
| `reference_injection_enabled,omitempty` | ReferenceInjectionEnabled | No | No | No | null | `bool` | direct | - | Controls only the independent reference lane. | Absent callers inherit injection_enabled; host sends the user setting independently from per-turn main suppression. |
| `primary_canon_base_max_chars,omitempty` | PrimaryCanonBaseMaxChars | No | No | No | null | `int` | direct | - | Optional non-null scalar int; positive values cap a subbudget within the resolved reference total. | Absent and explicit zero disable Canon Base and never increase the reference total. |
| `max_input_context_chars,omitempty` | MaxInputContextChars | No | No | Yes | 800 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (800): Go handler must apply default when ... |
| `narrative_stance,omitempty` | NarrativeStance | No | No | Yes | "balanced" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("balanced"): Go handler must apply defaul... |
| `supervisor_enabled,omitempty` | SupervisorEnabled | No | No | Yes | true | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (true): Go handler must apply default when... |
| `takeover_mode,omitempty` | TakeoverMode | No | No | Yes | "off" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("off"): Go handler must apply default whe... |
| `top_k,omitempty` | TopK | No | No | Yes | 5 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (5): Go handler must apply default when fi... |

## PromptUpdateRequest

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PUT /prompts/{prompt_name}`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `content,omitempty` | Content | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## ProxyPluginMainRequest

- **Fields**: 14
- **Required**: 1
- **Blockers**: 1
- **Routes**:
  - REQUEST: `POST /proxy/plugin-main`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `api_key,omitempty` | APIKey | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `budget_tokens,omitempty` | BudgetTokens | No | Yes | No | - | `*int64` | nullable_pointer | - | - | - |
| `endpoint,omitempty` | Endpoint | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `glm_thinking_type,omitempty` | GlmThinkingType | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `max_completion_tokens,omitempty` | MaxCompletionTokens | No | Yes | No | - | `*int64` | nullable_pointer | - | - | - |
| `max_tokens,omitempty` | MaxTokens | No | No | Yes | 1024 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (1024): Go handler must apply default when... |
| `messages` | Messages | Yes | No | No | - | `[]any` | array_of | ProxyPluginMainRequest.properties.messages.items: untyped property | - | - |
| `model,omitempty` | Model | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `provider,omitempty` | Provider | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `reasoning_budget_tokens,omitempty` | ReasoningBudgetTokens | No | Yes | No | - | `*int64` | nullable_pointer | - | - | - |
| `reasoning_effort,omitempty` | ReasoningEffort | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `reasoning_preset,omitempty` | ReasoningPreset | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `temperature,omitempty` | Temperature | No | No | Yes | 0.7 | `float64` | direct | - | Optional non-null scalar float64: absent vs zero-value distinction req... | Optional field with default (0.7): Go handler must apply default when ... |
| `timeout_ms,omitempty` | TimeoutMs | No | Yes | No | - | `*int64` | nullable_pointer | - | - | - |

## ReindexRequest

- **Fields**: 4
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /admin/reindex`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `batch_size,omitempty` | BatchSize | No | No | Yes | 20 | `int64` | direct | - | Optional non-null scalar int64: absent vs zero-value distinction requi... | Optional field with default (20): Go handler must apply default when f... |
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `max_items,omitempty` | MaxItems | No | No | Yes | 200 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (200): Go handler must apply default when ... |

## RescanRequest

- **Fields**: 3
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /admin/rescan`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `max_items,omitempty` | MaxItems | No | No | Yes | 50 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (50): Go handler must apply default when f... |
| `turn_indices,omitempty` | TurnIndices | No | No | No | - | `[]int` | array_of | - | - | - |

## RetrievalIndexRuntimeConfigRequest

- **Fields**: 1
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /retrieval-index/runtime-config`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `mode,omitempty` | Mode | No | No | Yes | "shadow" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("shadow"): Go handler must apply default ... |

## SagaGenerateRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /sagas/generate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `force,omitempty` | Force | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `from_turn,omitempty` | FromTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |
| `to_turn,omitempty` | ToTurn | No | No | Yes | 0 | `int` | direct | - | Optional non-null scalar int: absent vs zero-value distinction require... | Optional field with default (0): Go handler must apply default when fi... |

## SaveEffectiveInputRequest

- **Fields**: 3
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /effective-inputs`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `effective_input,omitempty` | EffectiveInput | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |

## SaveTurnRequest

- **Fields**: 4
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /turns`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `assistant_content,omitempty` | AssistantContent | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `turn_index` | TurnIndex | Yes | No | No | - | `int` | direct | - | - | - |
| `user_content,omitempty` | UserContent | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## SearchRequest

- **Fields**: 4
- **Required**: 1
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /search`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `top_k,omitempty` | TopK | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `user_input` | UserInput | Yes | No | No | - | `string` | direct | - | - | - |
| `wing,omitempty` | Wing | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |

## SessionMigrateRequest

- **Fields**: 7
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /admin/session-migrate`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `dry_run,omitempty` | DryRun | No | No | Yes | false | `bool` | direct | - | Optional non-null scalar bool: absent vs zero-value distinction requir... | Optional field with default (false): Go handler must apply default whe... |
| `gate_confidence,omitempty` | GateConfidence | No | Yes | No | - | `*float64` | nullable_pointer | - | - | - |
| `gate_reads,omitempty` | GateReads | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
| `gate_reason,omitempty` | GateReason | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `gate_status,omitempty` | GateStatus | No | Yes | No | - | `*string` | nullable_pointer | - | - | - |
| `source_session_id` | SourceSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `target_session_id` | TargetSessionID | Yes | No | No | - | `string` | direct | - | - | - |

## StorylineSyncRequest

- **Fields**: 4
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /storylines/sync`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `mode,omitempty` | Mode | No | No | Yes | "dry_run" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("dry_run"): Go handler must apply default... |
| `supervisor_result,omitempty` | SupervisorResult | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |

## SupervisorRequest

- **Fields**: 11
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /supervisor`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `auto_advance_trigger,omitempty` | AutoAdvanceTrigger | No | No | Yes | "none" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("none"): Go handler must apply default wh... |
| `chat_session_id,omitempty` | ChatSessionID | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `context_messages,omitempty` | ContextMessages | No | No | No | - | `[]map[string]any` | array_of | - | - | - |
| `guide_mode,omitempty` | GuideMode | No | No | Yes | "off" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("off"): Go handler must apply default whe... |
| `guide_suffix,omitempty` | GuideSuffix | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `momentum_packet,omitempty` | MomentumPacket | No | Yes | No | - | `*map[string]any` | nullable_pointer | - | - | - |
| `narrative_stance,omitempty` | NarrativeStance | No | No | Yes | "balanced" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("balanced"): Go handler must apply defaul... |
| `narrative_stance_bounds,omitempty` | NarrativeStanceBounds | No | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `narrative_stance_suffix,omitempty` | NarrativeStanceSuffix | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `persistent_guidance,omitempty` | PersistentGuidance | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |
| `wake_up_context,omitempty` | WakeUpContext | No | No | Yes | "" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default (""): Go handler must apply default when f... |

## TrustControlRequest

- **Fields**: 3
- **Required**: 0
- **Blockers**: 0
- **Routes**:
  - REQUEST: `PATCH /storylines/{storyline_id}/trust`
  - REQUEST: `PATCH /world-rules/{rule_id}/trust`
  - REQUEST: `PATCH /pending-threads/{hook_id}/trust`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `pinned,omitempty` | Pinned | No | Yes | No | - | `*bool` | nullable_pointer | - | - | - |
| `suppressed,omitempty` | Suppressed | No | Yes | No | - | `*bool` | nullable_pointer | - | - | - |
| `user_corrected,omitempty` | UserCorrected | No | Yes | No | - | `*bool` | nullable_pointer | - | - | - |

## ValidationError

- **Fields**: 5
- **Required**: 3
- **Blockers**: 3
- **Routes**: none

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `ctx,omitempty` | Ctx | No | No | No | - | `map[string]any` | object_without_properties | ValidationError.properties.ctx: object without properties | - | - |
| `input,omitempty` | Input | No | No | No | - | `any` | untyped | ValidationError.properties.input: untyped property | - | - |
| `loc` | Loc | Yes | No | No | - | `[]json.RawMessage` | array_of | ValidationError.properties.loc.items: non-null union anyOf | - | - |
| `msg` | Msg | Yes | No | No | - | `string` | direct | - | - | - |
| `type` | Type | Yes | No | No | - | `string` | direct | - | - | - |

## WorldRuleSyncRequest

- **Fields**: 4
- **Required**: 2
- **Blockers**: 0
- **Routes**:
  - REQUEST: `POST /world-rules/sync`

| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |
|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|
| `chat_session_id` | ChatSessionID | Yes | No | No | - | `string` | direct | - | - | - |
| `mode,omitempty` | Mode | No | No | Yes | "apply" | `string` | direct | - | Optional non-null scalar string: absent vs zero-value distinction requ... | Optional field with default ("apply"): Go handler must apply default w... |
| `supervisor_response` | SupervisorResponse | Yes | No | No | - | `map[string]any` | map_string_any | - | - | - |
| `turn_index,omitempty` | TurnIndex | No | Yes | No | - | `*int` | nullable_pointer | - | - | - |
