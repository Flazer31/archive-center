# Archive Center 2.4-7 Plugin Slimming and Backend Offload Inventory

Status: inventory completed; first bounded offload candidate selected, code movement not started
Target: Archive Center 2.4

## Goal

`Archive Center.js` must stay a thin RisuAI integration layer. It should observe RisuAI state, pass raw hints to the backend, mutate the live request payload at the last mile, and render UI. Long-memory retrieval, lane assembly, rollback planning, trace shaping, and durable interpretation should move toward the Go backend.

Current baseline:

```text
Archive Center.js: about 2.52 MB / 47,250 lines
```

## Non-Movable Plugin Ownership

These areas should remain in `Archive Center.js` because they depend on browser/RisuAI runtime state.

| Area | Current line range | Keep reason | Risk if moved |
| --- | ---: | --- | --- |
| Risu hook ownership | `onBeforeRequest` 32117+, `onAfterRequest` 32938+ | Only the plugin can observe and mutate the in-flight Risu payload. | High: backend cannot safely mutate browser-local payloads. |
| Payload message path detection and rewrite | 31749-32027, 30914-31077 | Depends on provider payload shape and live request body. | High: wrong rewrite can replace user input or inject into submodel calls. |
| Active chat observation/recovery | 6375-8062, 21683-22263 | Reads Risu active chat object and message references. | High: backend has no direct Risu object access. |
| Settings UI and overlay rendering | 47860+ | DOM-only behavior. | Medium: backend can provide schema later, but not render UI. |
| Minimal bridge/client/retry handoff | 11369-12112 | Browser network/storage fail-open behavior. | Medium: backend can own queue semantics later, but plugin must preserve offline handoff. |

## Offload Inventory

| Candidate | Current plugin line range | Target backend route/service | Risk | First action |
| --- | ---: | --- | --- | --- |
| Input Transparency / Effective Input render model | 14921-17460 | Extend `POST /prepare-turn` with `input_transparency_model` and `effective_input_preview` | Low-Medium | First bounded code candidate. Keep payload mutation local; backend returns read-only blocks and counters. |
| Auxiliary context block assembly and budget planning | 24185-26772, 30914-31077 | Extend `POST /prepare-turn` injection pack; later add `assembly_contract_version` | Medium | Move after trace model is stable. Must preserve user `maxInjectionChars`, topK, placement, and anchor settings. |
| Storyline/world/character/pending-thread block formatting | 20354-21441, 24185-24544 | `GET /continuity-pack/{sid}`, `/session-state/{sid}`, `/prepare-turn` support lanes | Medium | Backend already owns most data. Move formatting only after equivalent render-model tests. |
| Chroma/MariaDB hydration trace shaping | 15357-15574, 16792-17033 | `POST /search` and `POST /prepare-turn` trace objects | Low-Medium | Backend should expose found/hydrated/injected counters; plugin renders compactly. |
| Rollback/tail-delete cleanup planning | 12292-14790, 35644-36866 | New `POST /rollback/plan` plus existing `DELETE /rollback/{turn_index}` | High | Keep plugin detection; backend computes cleanup plan from submitted snapshot and canonical row state. Move only after deletion replay tests. |
| Session routing/migration planning | 8122-10298, 35007-35242 | `POST /sessions/resolve`, existing session migrate/admin routes | High | Do not move before rollback plan is stable. Incorrect migration can split or merge sessions wrongly. |
| Timeline/explorer detail shaping | 34163-41989, 43495-45047 | Existing explorer/timeline routes, later `timeline_render_model` | Medium | UI stays local; backend can return grouped display records to reduce plugin formatting. |
| Persona capsule queue/render helpers | 45403-46049 | Existing persona capsule routes | Medium | Defer; depends on UI forms and local candidate queue. |
| Table Read/output quality layer | 5440-6082 | 2.5 output-quality backend, not 2.4 | Medium | Keep out of 2.4 slimming unless requested. |

## First Bounded Offload Candidate

Selected candidate: Input Transparency / Effective Input render model.

Why this should be first:

- It is read-only debug/preview material and does not mutate the Risu payload.
- It directly addresses the user's need to see what was injected without making `Archive Center.js` format every lane itself.
- It can be tested with backend route tests against `/prepare-turn`.
- If it fails, the plugin can keep its existing local renderer as fallback without changing main model behavior.
- It prepares the next offload: moving more of `assembleInjectionWithBudget` without hiding where the memory went.

Proposed backend response shape:

```json
{
  "input_transparency_model": {
    "contract_version": "input_transparency_render.v1",
    "session_id": "char_x_cid_y",
    "turn_index": 12,
    "blocks": [
      {
        "key": "user_input",
        "title": "User Input",
        "status": "included",
        "text": "raw current user input"
      },
      {
        "key": "related_memories",
        "title": "Related Memories",
        "status": "included",
        "count": 4,
        "text": "backend-prepared memory lane"
      }
    ],
    "counts": {
      "vector_found": 4,
      "vector_hydrated": 4,
      "memory_injected": 4,
      "protected_secret_count": 1,
      "identity_accuracy_count": 1
    },
    "secret_display_policy": "counts_only_no_secret_text"
  },
  "effective_input_preview": {
    "contract_version": "effective_input_preview.v1",
    "payload_apply_mode": "shadow",
    "final_user_source": "input_hook",
    "auxiliary_context_chars": 2228,
    "raw_user_rewritten": false
  }
}
```

Plugin role after offload:

- Render the backend blocks.
- Keep a local fallback when the backend does not return the model.
- Continue last-mile payload injection with existing placement logic.
- Do not rebuild semantic meaning locally when backend trace exists.

Backend role after offload:

- Build block order, status, counts, and display-safe text from the same data used for injection.
- Redact protected-secret and identity-accuracy text unless the lane is explicitly safe for display.
- Expose vector found/hydrated/injected counters and language context counters in one place.
- Preserve all budgets from request settings; no new hardcoded topK, timeout, token, or language values.

## Code-Move Acceptance for the First Candidate

- `Archive Center.js` line count must decrease or stay stable after fallback removal; temporary bridge code must be smaller than removed local trace code.
- The backend must have focused tests for the render model contract.
- Existing `TestArchiveCenter24ReplayRegressionGate` must still pass.
- `node --check "Archive Center.js"` must pass.
- Effective Input visible output must remain equivalent, except for intentional addition of protected-secret/identity counts.
- Protected secret body text must not appear in the main narrative lane or general debug block unless explicitly allowed by a safe debug-only surface.

## 2.4-7a Backend Render Model Implementation

Implemented backend side:

- `POST /prepare-turn` now returns `input_transparency_model`.
- `POST /prepare-turn` now returns `effective_input_preview`.
- The render model is read-only: no writes and no LLM calls are attempted.
- The model reuses the same prepared injection assembly used for prompt support instead of reselecting memory.
- Vector visibility is surfaced as `vector_found`, `vector_hydrated`, `vector_selected`, and `vector_injected`.
- Protected secret and identity accuracy visibility is surfaced as counts only; the render model does not expose protected detail keys or secret body text.

Still intentionally pending:

- The plugin now renders this backend model when present.
- The local renderer remains as backend-off fallback because the plugin must stay usable when `/prepare-turn` is unreachable.
- Further deletion of old local trace-formatting code should happen only after live parity confirms no debug lane was lost.

## Later Offload Sequence

1. Input Transparency / Effective Input render model. Implemented with backend response and plugin adoption.
2. Chroma/MariaDB hydration trace shaping and counters.
3. Storyline/world/character/pending-thread formatting.
4. Auxiliary context budget planning and block assembly.
5. Rollback/tail-delete cleanup plan.
6. Session routing/migration planning.

This order keeps risky payload mutation and session identity logic local until the backend has enough read-only trace parity to prove behavior.
