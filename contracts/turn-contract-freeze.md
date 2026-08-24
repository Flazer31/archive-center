# Turn Contract Freeze ? Archive Center 2.0 R0

> Status: **R0 contract freeze**  
> Live turn implementation is **explicitly banned** in R0/R1. This document freezes the payload/response/fail-open contracts for `/complete-turn` and `/prepare-turn` based on 0.8 behavior analysis.

---

## 1. Route Inventory

| Route | Method | 0.8 Handler | Current Go Tier | Current Behavior |
|-------|--------|-------------|-----------------|------------------|
| `/complete-turn` | POST | `complete_turn_m4` ? `handle_complete_turn_m4` | R2 (write) | `writeShadowGuard` ? 503 |
| `/prepare-turn` | POST | `prepare_turn` ? `handle_prepare_turn` | R2 (write) | `writeShadowGuard` ? 503 |

Both routes are registered in `group_turn.go` and MUST remain R2 guards until live turn processing is approved.

---

## 2. `/complete-turn` Contract (M-4b)

### 2.1 Request DTO: `M4CompleteTurnRequest`

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `chat_session_id` | str | ? | Required |
| `turn_index` | int | ? | Required |
| `user_input` | str | `""` | User message text |
| `assistant_content` | str | `""` | Assistant message text |
| `context_messages` | list[dict] | `[]` | Full conversation context |
| `improvement_trace` | dict | null | Optional trace metadata |
| `output_language_override` | dict | null | Optional language override |
| `request_type` | str | `"model"` | `"model"` or `"system"` |
| `client_meta` | dict | `{}` | Client metadata |

**Go DTO**: `internal/dto.M4CompleteTurnRequest` (auto-generated from OpenAPI)

### 2.2 Response DTO: `M4CompleteTurnResponse`

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `status` | str | `"ok"` | `"ok"` or `"error"` |
| `chat_session_id` | str | `""` | |
| `turn_index` | int | `0` | |
| `generated_at` | str | `""` | ISO-8601 timestamp |
| `save_ok` | bool | `false` | DB persistence result |
| `save_error` | str | null | Error message if save failed |
| `critic_triggered` | bool | `false` | True if critic ran |
| `critic_result` | dict | null | Critic output |
| `episode_result` | dict | null | Episode generation result |
| `chapter_result` | dict | null | Chapter generation result |
| `summary_fallback` | dict | null | Summary fallback envelope |
| `maintenance_enqueued` | bool | `false` | Async maintenance job queued |
| `fail_reasons` | list[str] | `[]` | Non-fatal failure reasons |
| `trace_handoff` | dict | null | Handoff trace metadata |
| `warnings` | list[str] | `[]` | Non-fatal warnings |

**Go DTO**: `internal/dto.M4CompleteTurnResponse` (auto-generated from OpenAPI)

### 2.3 Fail-Open Behavior
- If `save_ok` is `false`, the client MUST NOT block chat flow. The turn may proceed with a degraded save.
- If `critic_triggered` is `true` but `critic_result` is null, the client falls back to the original assistant content.
- `fail_reasons` is advisory; HTTP status remains `200` unless a fatal exception occurs.
- `maintenance_enqueued` is fire-and-forget; failure to enqueue does not fail the turn.

---

## 3. `/prepare-turn` Contract (M-2b / P-2c)

### 3.1 Request DTO: `PrepareTurnRequest`

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `chat_session_id` | str | ? | Required |
| `request_type` | str | `"model"` | `"model"` or `"system"` |
| `raw_user_input` | str | `""` | Raw user message |
| `messages` | list[dict] | `[]` | Conversation history |
| `continuity_trigger_mode` | str | `"none"` | `"none"`, `"query"`, `"auto"` |
| `continuity_query` | str | `""` | Query string when mode is `"query"` |
| `turn_index` | int | null | Optional explicit turn index |
| `settings` | `PrepareTurnSettings` | factory | See ?3.2 |
| `client_meta` | dict | `{}` | Client metadata |

**Go DTO**: `internal/dto.PrepareTurnRequest` (auto-generated from OpenAPI)

### 3.2 Sub-DTO: `PrepareTurnSettings`

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `guide_mode` | str | `"off"` | `"auto"`, `"off"`, `"standard"`, `"romantic"`, `"action"`, `"mature_soft"`, `"mature_direct"`; `auto` resolves to language-neutral `standard` in Go |
| `narrative_stance` | str | `"balanced"` | `"balanced"`, `"authoritarian"`, `"permissive"` |
| `apply_mode` | str | `"shadow"` | `"shadow"`, `"live"` (live blocked in R0) |
| `takeover_mode` | str | `"off"` | `"off"`, `"prompt"`, `"auto"` |
| `injection_enabled` | bool | `true` | Enable memory injection |
| `input_context_enabled` | bool | `true` | Enable input context |
| `max_injection_chars` | int | `9000` | Independent main memory, world, and relationship injection cap |
| `core_objective_memory_max_items` | int | none | Optional final-delivery ceiling for distinct objective event summaries; absent preserves legacy delivery; minimum 1; does not reinterpret `top_k` or count separately budgeted support lanes |
| `reference_injection_budget_basis_chars` | int | `3000` | Independent original-work reference cap; does not borrow from memory or lorebook lanes |
| `lorebook_reference_max_chars` | int | `3000` | Independent Host lorebook reference cap; does not borrow from memory or original-work lanes |
| `reference_recall_limit` | int | none | Candidate limit used only by original-work recall; absent inherits `top_k`, explicit 0 disables reference candidates, negative clamps to 0 |
| `reference_injection_enabled` | bool | none | Controls only the independent reference lane; absent callers inherit `injection_enabled` |
| `primary_canon_base_max_chars` | int | none | Primary Canon Base subbudget inside the resolved reference total; absent/0 disables the base |
| `max_input_context_chars` | int | `800` | Context length cap |
| `episode_interval_turns` | int | `10` | Episode generation interval |
| `supervisor_enabled` | bool | `true` | Enable supervisor pass |
| `top_k` | int | `5` | Retrieval top-k |

**Go DTO**: `internal/dto.PrepareTurnSettings` (auto-generated from OpenAPI)

### 3.3 Response DTO: `PrepareTurnResponse`

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `status` | str | `"ok"` | |
| `source` | str | `"skeleton"` | Source label for debugging |
| `chat_session_id` | str | `""` | |
| `generated_at` | str | `""` | ISO-8601 timestamp |
| `effective_user_input` | str | null | Processed user input |
| `injection_text` | str | null | Memory injection text |
| `input_context_text` | str | null | Input context text |
| `supervisor_result` | dict | null | Supervisor decision |
| `narrative_control_summary` | dict | null | Narrative control state |
| `autonomy_plan` | dict | null | Autonomy proposal |
| `progression_ledger` | dict | null | Progression tracking |
| `micro_beat_proposal` | dict | null | Micro-beat proposal |
| `scene_step_proposal` | dict | null | Scene-step proposal |
| `combined_proposal` | dict | null | Combined planner output |
| `generation_packet` | `GenerationPacket` | null | See ?3.4 |
| `writeback_preview` | dict | null | DB writeback preview |
| `trace_preview` | dict | null | Trace preview |
| `session_state` | dict | null | Session state bundle |
| `narrative_control` | dict | null | Narrative control bundle |
| `continuity_pack` | dict | null | Continuity pack |
| `resume_pack` | dict | null | Resume pack |
| `canonical_ledger` | dict | null | Canonical ledger |
| `recall_result` | dict | null | Recall search result |
| `supervisor_input_pack` | dict | null | Supervisor input |
| `publisher_call_budget_ledger` | dict | null | Go-owned Publisher call prompt/token observation; present only when a Publisher call was prepared |
| `injection_pack` | dict | null | Injection assembly |
| `packet_composition` | dict | null | Packet composition metadata |
| `long_session_health` | dict | null | Long-session health snapshot |

`reference_injection.budget_policy` uses contract `reference_injection_budget.v2`.
Let `R = max(0, reference_injection_budget_basis_chars)`, defaulting to `3000`.
`R` is the independent original-work reference cap for both supplement and
primary bindings. If any primary binding is present (including mixed binding
sets), primary still wins for authority and rendering, but it does not change
the cap. The original-work lane is additive and non-displacing: it never
reduces or retrims the main memory or lorebook lanes. Primary Canon Base is
assembled first with effective subbudget
`min(max(0, primary_canon_base_max_chars), R)`; scene reference uses the
remaining `R-primary_used`. Unused base capacity remains reusable only inside
this original-work lane. The invariant is
`primary_used + scene_used <= R`.

Let `L = max(0, lorebook_reference_max_chars)`, defaulting to `3000`. The Host
lorebook lane uses only `L`; original-work usage never reduces it and unused
original-work capacity never increases it. Likewise, unused lorebook capacity
is not transferred to original work or main memory. First-turn main-memory
suppression does not alter either independent reference cap.

#### 3.3.1 Main-model payload budget ledger

`payload_application_plan.budget_ledger` uses contract
`payload_budget_ledger.v1` and is owned entirely by Go. It accounts only for
the auxiliary system message prepared for the main-model request. Publisher
and Critic provider calls are separate calls and MUST NOT be combined into this
ledger.

The ledger exposes independent lanes for `long_term_memory`, `original_work`,
`lorebook_reference`, and `output_guidance`. Each lane reports its configured
and effective cap, candidate, selected, and final-delivery counts and chars,
plus exclusion or truncation reason counts. The top level reports the sum of
configured and effective caps, candidate and selected chars, lane-content
chars, outer title/separator assembly chars, and the exact final auxiliary
payload chars. Internal lane titles are included in their lane chars; only the
outer auxiliary title and separators are counted as top-level assembly cost.

`final_delivery_chars` is the exact prepared payload length, not by itself
proof that RisuAI applied the message. Actual host application is confirmed by
the existing `payload_application_observation.v1`; the UI may label the backend
number as delivered only when that observation is ready and applied (or empty),
and otherwise labels it as planned. JavaScript MUST NOT recompute caps, lane
totals, candidates, exclusions, or assembly cost.

#### 3.3.2 Lorebook selection observation

`lorebook_reference` keeps the selection policy under the existing
`lorebook_reference_recall.v1` owner and adds
`selection_observation_contract=lorebook_selection_observation.v1`. It reports
catalog, candidate, selected, deferred, and delivered counts; `Always Active`
counts at candidate and delivery boundaries; matched keys and context overlap;
final candidate dispositions; and exact-text duplicate suppression classified
as current user input, Risu host message, or Archive Center context.

After exact-payload suppression and same-content coalescing, direct-key groups
form the relevance frontier when any exist. Otherwise, only groups tied at the
highest positive context overlap form the frontier. Context overlap uses the
stored normalized search text, falling back to key, secondary key, comment, and
content when that stored text is absent. Lower relevance groups remain excluded
even when the 3000-character cap has spare capacity. JavaScript may render the
backend observation but MUST NOT infer activation, recalculate dispositions, or
select additional entries. A matching Risu host message proves that the Host request already
contains the exact text; the observation does not guess which native Host
feature produced that message.

#### 3.3.3 Publisher and Critic call budget ledger

Every actual Publisher or Critic provider call uses
`provider_call_budget_ledger.v1`. This ledger is separate from the main-model
`payload_budget_ledger.v1` and reports exact Unicode character counts for the
assembled call: system prompt, current turn, auxiliary memory, original-work
reference, lorebook reference, language context, JSON/output requirement,
assembly, user prompt, and final prompt. A lane that is not part of that call's
contract is recorded as `0` with status `not_in_call_contract`; it is not
silently reclassified from another lane.

Provider tokens are copied only from normalized provider response metadata when
`usage_reported=true`. Otherwise `provider_usage_status=unreported`; chars MUST
NOT be converted into estimated tokens. The ledger also records HTTP status,
termination kind, call status, and the exact failure stage such as
`provider_call`, `provider_response`, `json_parse`, or `schema_validation`.
Critic keeps its existing detailed `input_budget` trace and adds this common
ledger. Because the Critic JSON/output contract is embedded in the single
system prompt file, its separate text length is reported as
`embedded_in_system_prompt_not_separable` instead of being guessed by parsing
prompt prose.

**Go DTO**: `internal/dto.PrepareTurnResponse` (auto-generated from OpenAPI)

### 3.4 Sub-DTO: `GenerationPacket` (Fail-Open Core)

| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `packet_mode` | str | `"off"` | `"off"`, `"injection"`, `"guidance"`, `"full"` |
| `effective_user_input` | str | null | Rewritten user input |
| `injection_text` | str | null | Injected memory text |
| `guidance_metadata` | dict | `{}` | Guidance decisions |
| `safety_metadata` | dict | `{}` | Safety check results |
| `degraded` | bool | `false` | **True if packet is incomplete/missing** |
| `fallback_reason` | str | `""` | Reason for degradation |
| `trace_summary` | dict | `{}` | Trace summary |
| `shadow_compare_record` | dict | `{}` | Shadow comparison record |

**Fail-Open Rule**: If `generation_packet` is missing, `packet_mode` is `"off"`, or `degraded` is `true`, the plugin MUST keep its local path and the chat flow MUST NOT stop.

**Go DTO**: `internal/dto.GenerationPacket` (auto-generated from OpenAPI)

---

## 4. Fail-Open Behavior Matrix

| Condition | `/complete-turn` Behavior | `/prepare-turn` Behavior |
|-----------|---------------------------|--------------------------|
| `status != "ok"` | HTTP 200 with `save_ok=false`; client continues | HTTP 200 with `generation_packet.degraded=true`; client uses local path |
| `save_error` present | Logged; not fatal | N/A |
| `generation_packet` missing | N/A | Client treats as `"off"`; no injection |
| `degraded=true` | N/A | Client skips injection; chat continues |
| Upstream DB unavailable | `save_ok=false`; turn proceeds with warning | `degraded=true`; fallback to empty packet |
| Critic failure | `critic_triggered=true`, `critic_result=null` | N/A |

---

## 5. Go DTO ? Route Mapping

```go
// group_turn.go
mux.HandleFunc("POST /complete-turn", s.handleCompleteTurn)   // R2
mux.HandleFunc("POST /prepare-turn", s.handlePrepareTurn)       // R2
```

```go
// Future live handler skeleton (R2+)
func (s *Server) handleCompleteTurn(w http.ResponseWriter, r *http.Request) {
    var req dto.M4CompleteTurnRequest
    if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
        writeError(w, http.StatusBadRequest, "bad_request", err.Error())
        return
    }
    // ... live logic (banned in R0/R1)
}

func (s *Server) handlePrepareTurn(w http.ResponseWriter, r *http.Request) {
    var req dto.PrepareTurnRequest
    if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
        writeError(w, http.StatusBadRequest, "bad_request", err.Error())
        return
    }
    // ... live logic (banned in R0/R1)
}
```

**Go DTO Files**: `internal/dto/types_gen.go`
- `M4CompleteTurnRequest` (lines ~623)
- `M4CompleteTurnResponse` (lines ~658)
- `PrepareTurnRequest` (lines ~924)
- `PrepareTurnSettings` (lines ~965)
- `PrepareTurnResponse` (lines ~1010)
- `GenerationPacket` (lines ~850)

---

## 6. Trace / Fallback Fields

### 6.1 `/complete-turn` Trace
- `trace_handoff` ? pipeline handoff metadata (version, retention policy).
- `fail_reasons` ? enumerated non-fatal reasons (e.g., `"summary_stale"`, `"episode_skipped"`).
- `warnings` ? advisory strings (e.g., `"chapter_generation_pending"`).

### 6.2 `/prepare-turn` Trace
- `trace_preview` ? condensed trace of all sub-system calls.
- `packet_composition` ? which sub-packets contributed to the final generation packet.
- `shadow_compare_record` ? R0/R1 shadow vs live comparison data (empty in R0).

### 6.3 Fallback Vocabulary
- `"off"` ? no injection/guidance; local path only.
- `"injection"` ? memory injection only.
- `"guidance"` ? narrative guidance only.
- `"full"` ? injection + guidance + generation packet.
- `"degraded"` ? partial failure; packet delivered but flagged.

---

## 7. R0 Shadow Handler Contract

Until live turn processing is approved:

1. `POST /complete-turn` ? `503 shadow_guard`
2. `POST /prepare-turn` ? `503 shadow_guard`
3. No DB write, no upstream LLM call, no episode/chapter generation.
4. DTO decode helpers MAY be exercised in tests (as R1 shadow read-only probes) but MUST NOT trigger side effects.

---

## 8. Verification Checklist

Before live turn implementation:

- [ ] `M4CompleteTurnRequest` decode uses `DecodeWithDefaults` with validation.
- [ ] `PrepareTurnRequest` decode uses `DecodeWithDefaults` with validation.
- [ ] `PrepareTurnSettings.ApplyDefaults()` is called automatically.
- [ ] `GenerationPacket.degraded` is checked before any client-side injection.
- [ ] `save_ok=false` does not block chat flow.
- [ ] All trace fields are populated (not left null silently).
- [ ] Fail-open fallback_reason is human-readable.
- [ ] Shadow compare record is written when `Mode == ModeShadow`.
- [ ] H-4e release hygiene scan passes.

---

*Contract version: R0-2026-05-21*  
*Reference: `Archive Center Beta 0.8(fix)/backend/turn_contracts.py`, `backend/services/complete_turn.py`, `backend/services/prepare_turn.py`*
