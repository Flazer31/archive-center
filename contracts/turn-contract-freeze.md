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
| `guide_mode` | str | `"off"` | `"off"`, `"hint"`, `"direct"` |
| `narrative_stance` | str | `"balanced"` | `"balanced"`, `"authoritarian"`, `"permissive"` |
| `apply_mode` | str | `"shadow"` | `"shadow"`, `"live"` (live blocked in R0) |
| `takeover_mode` | str | `"off"` | `"off"`, `"prompt"`, `"auto"` |
| `injection_enabled` | bool | `true` | Enable memory injection |
| `input_context_enabled` | bool | `true` | Enable input context |
| `max_injection_chars` | int | `3000` | Injection length cap |
| `reference_injection_budget_basis_chars` | int | none | Configured memory cap used as the independent reference budget basis; remains stable when a turn temporarily suppresses main memory injection |
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
| `injection_pack` | dict | null | Injection assembly |
| `packet_composition` | dict | null | Packet composition metadata |
| `long_session_health` | dict | null | Long-session health snapshot |

`reference_injection.budget_policy` uses contract `reference_injection_budget.v1`.
Let `M = max(0, reference_injection_budget_basis_chars)` when supplied, otherwise
fall back to the effective `max_injection_chars`. The host supplies the configured
memory cap and `reference_injection_enabled` separately, so first-turn main
suppression (`injection_enabled=false`, `max_injection_chars=0`) does not disable
the independent reference lane. With reference injection disabled, no
binding, an unknown-only binding set, or `M=0`, the reference total `R` is 0.
Supplement-only bindings resolve `R=floor(M/2)`. If any primary binding is
present (including mixed binding sets), primary wins and `R=M`. The reference
lane is additive and non-displacing: it never reduces or retrims the main
memory lane. Primary Canon Base is assembled first with effective subbudget
`min(max(0, primary_canon_base_max_chars), R)`; scene reference uses the
remaining `R-primary_used`, so unused base capacity is reusable. The invariant
is `primary_used + scene_used <= R`.

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
