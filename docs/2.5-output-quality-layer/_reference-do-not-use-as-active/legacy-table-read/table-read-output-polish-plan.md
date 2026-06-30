# LEGACY / DEBUG ONLY: Table Read Output Polish Plan

> Current product boundary: use
> `_archive/future-reference/RisuAI_TableRead_OutputOnly_Boundary.md` as the
> active scope. This file contains older TR-POLISH proof notes and should not be
> used to justify live afterRequest post-processing.

## 2026-06-17 Direction Correction

The original TR-POLISH implementation proved that `afterRequest` can alter the
assistant output, but that is no longer the product direction.

The live `afterRequest` polish path is disabled in `Archive Center.js`.

Why:

- It works after the assistant output already exists.
- In streaming recovery paths, the raw output may appear before the replacement.
- Timeline pending artifacts can briefly appear and then be replaced by the real
  backend refresh, which looks like memory was saved and then disappeared.
- This does not match the desired behavior: Table Read should help produce the
  final answer before the user sees it, not rewrite it afterward.

Current status:

- `/table-read/draft`, `/table-read/simulate`, `/table-read/review`,
  `/table-read/revise`, and `/table-read/polish` remain diagnostic/debug routes.
- The old `tableReadOutputPolishEnabled` setting is forced off during settings
  sanitization.
- The next product path is pre-output orchestration: prepare a draft, run Table
  Read with relevant entity memories, revise internally, and return only the
  final answer to RisuAI.

## TR-POLISH-0 Decision

Table Read is not a candidate-suggestion UI.

The product goal is post-output improvement:

```text
original assistant output
-> Output Check
-> Table Read with relevant entity memories
-> Output Enhance
-> final assistant output returned to RisuAI
-> final assistant output stored by Archive Center
```

The user-facing behavior should be that the assistant response is improved
before it is shown/stored. Candidate copy buttons are debug tools only.

## Existing Routes Kept As Diagnostics

The current routes remain available for debugging and development:

- `POST /table-read/draft`
  - dry-run entity memory binding check
  - no LLM call
  - no DB write

- `POST /table-read/simulate`
  - pre-output scene discussion
  - useful for debugging entity memory lanes
  - not the main output-improvement path

- `POST /table-read/review`
  - read-only review of an assistant draft
  - useful for inspecting voice, leak, continuity, and emotion issues
  - no output replacement

- `POST /table-read/revise`
  - debug-only revision generation
  - no output replacement
  - no DB write
  - not the product UX

These routes should not define the final user experience.

## Product Route Direction

Add a new route:

`POST /table-read/polish`

Expected request:

- `chat_session_id`
- `turn_index`
- `user_input`
- `assistant_output_original`
- optional recent scene/context summary
- relevant entity memory bundles
- LLM configuration

Expected response:

- `assistant_output_final`
- `changed`
- `issues`
- `protected_reveals`
- `entity_review_trace`
- `fallback_reason`

Contract:

- no canonical truth writes
- no direct memory mutation
- no KG/direct evidence write from the polish route itself
- private recollections stay subtext
- the returned final output is safe for display/storage

## Plugin Integration Point

The primary integration point is `Archive Center.js` `afterRequest`.

Target flow:

```text
afterRequest(content, type)
-> skip auxiliary/module outputs
-> collect session id, turn index, raw user input
-> load relevant entity memories
-> call /table-read/polish
-> if success and assistant_output_final is non-empty:
     return assistant_output_final
   else:
     return original content
```

For native non-streaming responses, returning `assistant_output_final` should be
the preferred path.

For streaming/poller recovery paths, the response may already be displayed. In
that case the fallback path is:

```text
active chat assistant message replace
-> /turns/replace or complete-turn replace
-> timeline refresh
```

## Storage Rule

Archive Center should store the final improved assistant output, not the raw
unpolished output.

The original output may be retained only as trace/debug evidence, for example:

- `table_read_trace.original_assistant_output`
- `table_read_trace.issues`
- `table_read_trace.protected_reveals`

Derived artifacts should be generated from `assistant_output_final`:

- `chat_logs`
- `memories`
- `direct_evidence`
- `kg_triples`
- subjective entity memories

## Secret and Subjective Memory Guard

Entity memories are subjective memory banks, not global truth.

Rules:

- Do not narrate private recollection as objective fact.
- Do not expose NPC private memory directly through the narrator.
- Do not reveal loop/regression/reincarnation/isekai knowledge unless already
  explicit in the user input or current output.
- Reflect private knowledge through behavior, hesitation, tone, avoidance,
  misunderstanding, suspicion, or emotional drift.
- If the polish pass cannot improve safely, return the original output.

## Implementation Phases

### TR-POLISH-1: Return Replacement Proof

Goal: prove that `afterRequest` can replace the displayed/stored assistant
output.

No LLM call yet.

Test behavior:

- append or wrap a harmless marker in debug mode only
- return the modified content
- confirm RisuAI display uses the returned content
- confirm Archive Center stores the returned content

This separates replacement mechanics from LLM behavior.

Status: implemented as a debug-only one-shot proof in `Archive Center.js`.

- Debug tab button arms the next persisted assistant output.
- No LLM call is made.
- The marker `[TR-POLISH-1 PROOF: afterRequest replacement applied]` is appended
  to the next normal save-target output.
- The marked output is used for the `afterRequest` return value and the
  `/complete-turn` persistence path.
- Streaming poller recovery may already have rendered the assistant message;
  in that case this proof can still validate persistence, while UI replacement
  needs the later active-chat replace path.

### TR-POLISH-2: Backend Polish Route

Add `/table-read/polish`.

At this phase it may use a deterministic mock in tests, but the route contract
must already match the final shape:

- input original output
- output final output
- support trace
- no DB write

Status: implemented as a read-only backend route contract.

- `POST /table-read/polish` is mounted in the Go backend.
- The route accepts `chat_session_id`, `turn_index`, `user_input`,
  `assistant_output_original`, optional scene/recent context, review context,
  entities, and multi-model planning hints.
- It returns `assistant_output_final` with `changed=false` and
  `fallback_reason=tr_polish_2_route_contract_only`.
- No LLM call is attempted in TR-POLISH-2.
- No canonical truth, memory, direct evidence, KG, or subjective memory write is
  attempted.
- Relevant entity subjective memory cards are still bound into the support trace
  so TR-POLISH-3 can reuse the same route shape when live LLM polishing is
  connected.

### TR-POLISH-3: Live LLM Polish

Connect the configured LLM to `/table-read/polish`.

The prompt should ask for the final assistant response only, plus structured
trace out-of-band in JSON.

The final response must not contain analysis notes or candidate labels.

Status: implemented in the Go backend route.

- `POST /table-read/polish` now calls the configured LLM when `llm.endpoint`,
  `llm.api_key`, and `llm.model` are supplied.
- `Archive Center.js` now builds that `llm` object from dedicated
  `tableReadLlm*` settings first. If those fields are empty, it falls back to
  the Publisher LLM settings for compatibility.
- If no LLM config is supplied, the TR-POLISH-2 contract-only passthrough path
  remains available.
- The LLM must return JSON with `assistant_output_final`, `changed`, `issues`,
  `protected_reveals`, `entity_review_trace`, and `fallback_reason`.
- `assistant_output_final` is returned as the final display/storage candidate.
- The route still performs no DB write and does not mutate canonical memory,
  direct evidence, KG, or subjective entity memory.
- If the LLM output cannot be parsed or lacks `assistant_output_final`, the route
  falls back to the original assistant output.
- Subjective entity memories are bound into the prompt as support-only context;
  private/NPC recollections must remain subtext or trace, not narrator truth.

### TR-POLISH-4: Archive Center.js Integration

Wire `/table-read/polish` into the normal assistant output path.

Controls:

- Table Read Output Polish on/off
- fallback to original on failure
- skip auxiliary/module outputs
- skip if no relevant entity memories

Dashboard:

- last polish status
- changed/skipped/failed
- fallback reason

Status: implemented in `Archive Center.js`.

- A new `tableReadOutputPolishEnabled` setting gates the feature. It defaults to
  off.
- The normal `afterRequest` save path calls `/table-read/polish` only after the
  main narrative persistence gate has accepted the assistant output.
- The plugin loads subjective entity memory bundles for the current session and
  selects only entities whose names/keys appear in the user input, assistant
  output, or recent context.
- If there is no Table Read LLM config or Publisher fallback config, no relevant
  entity memory, an HTTP failure, or a malformed polish result, the original
  output is kept.
- When changed, the returned `assistant_output_final` replaces the display
  return value and the `/complete-turn` persisted assistant content.
- Streaming/poller recovery can still improve the storage path, but the already
  rendered screen may not visibly change until the host refreshes the message.

### TR-POLISH-5: Storage and Replace Consistency

Ensure the improved output is the single source for the turn:

- complete-turn uses improved output
- regeneration replaces the same turn
- deletion removes artifacts derived from that turn
- no duplicate raw/improved rows

Status: implemented in `Archive Center.js`.

- A small persistent polish storage ledger records the original/final assistant
  hashes for a turn when `/table-read/polish` changes the output.
- `/complete-turn` now carries `client_meta.table_read_output_polish` plus
  `preserve_requested_turn_index=true` when the assistant output matches the
  polished turn ledger.
- Failed complete-turn queue payloads preserve the table-read polish metadata,
  so retry does not silently fall back to an untagged/raw turn.
- Active-chat backfill checks the polish ledger before saving a recovered pair;
  if it sees the original raw assistant hash, it replaces it with the polished
  final output before sending `/complete-turn`.
- Rollback and session deletion clear affected polish ledger rows, preventing
  deleted/replaced turns from reappearing through later repair/backfill passes.
- The route still does not write memory/KG/evidence directly. Derived artifacts
  remain generated by the normal complete-turn path from the final assistant
  text.
- Same-turn regeneration is currently consistent through the existing
  rollback/delete invalidation path: clear the old turn first, then save the
  polished final output at the preserved requested turn index. The backend still
  intentionally refuses to overwrite a conflicting already-persisted raw pair
  without that explicit invalidation step.

Limitation:

- Existing rows already saved before TR-POLISH-5 are not automatically migrated.
  They should be repaired through the existing turn replace/rebuild tools if
  needed.
- A dedicated force-replace endpoint is not part of TR-POLISH-5. If we need
  direct overwrite semantics later, it should be implemented as a separate,
  explicit replace path with artifact cleanup and audit evidence.

### TR-POLISH-6: UI Debug View

Keep the diagnostic routes available under debug:

- raw review
- revision debug output
- original vs final compare
- protected reveals
- entity review trace

Do not make candidate selection the main UX.

Status: implemented in `Archive Center.js`.

- Debug mode now shows a read-only `TR-POLISH-6 output compare` block.
- The block renders the latest `/table-read/polish` result as:
  - original assistant output preview
  - final assistant output preview
  - issues
  - protected reveals
  - entity review trace
  - session/turn metadata and changed/kept status
- The comparison snapshot is bounded and stored only in runtime state. It is not
  a candidate-selection workflow and does not write canonical memory, evidence,
  KG, or subjective entity memories.
- Candidate-style TR-Review/TR-Revise screens remain diagnostic/debug surfaces;
  the product path is still automatic output polish before display/storage.

## Acceptance Criteria

The feature is not complete until all of these are true:

1. A non-streaming assistant response is visibly replaced by the polished final
   output.
2. A streaming/poller-recovered response is either replaced or clearly falls
   back with a visible reason.
3. `chat_logs` stores the final output, not a separate candidate.
4. derived artifacts are generated from the final output.
5. auxiliary outputs such as Lightboard, NPC list, scoring modules, or helper
   module outputs are not polished/stored as main assistant turns.
6. private entity memories influence behavior and tone without becoming
   narrator-exposed truth.
7. debug routes remain available but are not the main workflow.

## Non-Goals For TR-POLISH-0

TR-POLISH-0 does not implement the route or change runtime behavior.

It only fixes the design boundary:

- existing TR-2/TR-Review routes stay
- candidate-style TR-Review-2 is debug-only
- the next implementation target is output replacement, not candidate display
