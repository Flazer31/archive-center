# Risu Recomposer MDASH Execution Plan

Date: 2026-07-25

Status: canonical architecture contract and preparatory implementation plan

Target runtime:

- `source/Risu Recomposer.js`
- standalone one-file RisuAI plugin
- no Archive Center, MariaDB, or ChromaDB integration in this plan

This document supersedes earlier implementation order, size limits, and beta
gates in the dated 2.5 roadmaps and the R1-R7 worker handoff. Those documents
remain useful as design history and detailed reference, but this document wins
when their current sequence or limits conflict.

## 1. Product Goal

RisuAI's main model output is `draft_zero`. Before that output is generated,
the standalone plugin must turn the current RisuAI context into a compact
per-turn narrative contract. After `draft_zero` is generated, specialized
models must produce concrete rewrite candidates, compare and combine those
candidates, and return a materially improved RP response.

The normal success path is not findings-only advice:

```text
RisuAI runtime context
-> context manifest
-> turn contract
-> RisuAI main model
-> draft_zero
-> prepared scene
-> specialist rewrite candidates
-> semantic adjudication
-> whole-scene composition
-> structural and semantic verification
-> enhanced output
```

The input stage does not replace the main model with a serial first-draft
writer. It prepares the constraints, evidence, and scene objectives that the
main model and the later rewrite pipeline share.

The output stage does not stop at findings or advice:

```text
draft_zero
-> prepared scene
-> specialist rewrite candidates
-> semantic adjudication
-> whole-scene composition
-> structural and semantic verification
-> enhanced output
```

Low-cost and small models are expected to be useful for specialist work, but
they are not mandatory. Every role may use a local, commercial, distilled, or
frontier model selected by the user.

## 2. Architecture Decision

The three reference patterns have different ownership:

- MDASH defines the staged harness: Prepare, Scan, Validate, Dedupe, and Prove.
- Fusion defines candidate comparison, conflict/coverage analysis, and final
  synthesis.
- Fugu is adapted only as deterministic stage and role routing. This standalone
  plugin does not claim to implement a trained Fugu orchestrator.

MDASH owns both sides of the main model:

```text
Observe and Prepare runtime context
-> Input specialist planning (preset-dependent parallel LLM)
-> Contract Validate and Dedupe (JS)
-> Turn Contract Fusion (JS, with conditional LLM adjudication later)
-> inject one bounded contract into beforeRequest
-> RisuAI main model produces draft_zero
-> Prepare output segments (JS)
-> Specialist Scan / Rewrite (parallel LLM)
-> Candidate Dedupe (JS)
-> Semantic Fusion Judge (LLM, deferred)
-> optional Table Read evidence (later)
-> Fusion Director (JS)
-> Whole-Scene Composer (LLM)
-> Structural Verifier (JS)
-> conditional Semantic Prover (LLM, deferred)
-> at most one Repair and re-Prove cycle
-> enhanced output
```

## 3. Decisions That Are Locked

1. Judge and Composer stay separate. A model must not be the only judge of the
   response it writes.
2. `turn_contract.v1` is mandatory in standalone mode. The first implementation
   builds it from a typed context manifest and preset-selected planner
   fragments; it does not add a serial first-draft model.
3. There is no LLM router. Fugu-like routing remains deterministic JS under the
   user-selected preset.
4. All six specialist profiles remain available, but the deterministic router
   selects at most 2/3/4 output specialists in Fast/Balanced/Quality. Quality
   does not mean "call every configured role".
5. Input planner attempts and output rewrite attempts use separate budgets.
   Output reserves Composer primary plus one distinct configured fallback
   before any specialist retry. Specialists use one primary attempt.
6. Semantic Judge, Semantic Prover, and Table Read remain deferred until the
   adaptive specialist plus Composer path is stable in live RisuAI turns.
7. The one-file hard size limit is 500 KB. New behavior should replace obsolete
   paths in the same batch instead of appending parallel policy.
8. Archive Center subjective memory and backend integration remain future work.
9. Standalone input context is read-only. Recomposer may consume RisuAI
   character/scenario settings, persona, current request messages, recent chat,
   current lorebook entries, and Supa/Hypa/current-chat memory when those
   values are exposed. It must not write or mutate those sources.
10. Runtime and character settings mean narrative inputs exposed by RisuAI.
    Provider credentials, API keys, request headers, and model-account settings
    are never narrative context.
11. When a future Archive Center connected mode is enabled, Archive Center owns
    long-term-memory truth, retrieval, visibility, and persistence. Recomposer
    owns per-turn contract construction and output recomposition. Duplicate
    narrative-guide injection must not remain active in both products.

## 4. Current Baseline

Inspected runtime:

- version: `0.1.11`
- file: `source/Risu Recomposer.js`
- approximate size at inspection: 324,925 bytes
- approximate lines at inspection: 6,472
- in-memory tests present: 105

The existing tests cover syntax, schemas, provider mocks, assembly, and
structural preservation. They do not prove live RisuAI lifecycle behavior or
RP prose quality.

Known blocking mismatch:

- official `beforeRequest` receives `OpenAIChat[]`
- current payload extractors read `payload.messages`

The implementation contract needed for Batch 1 is recorded here so a worker
does not depend on the temporary upstream audit checkout:

```ts
addRisuReplacer(
    type: 'beforeRequest',
    func: (messages: OpenAIChat[], type: string) =>
        OpenAIChat[] | Promise<OpenAIChat[]>
): Promise<void>;

addRisuReplacer(
    type: 'afterRequest',
    func: (content: string, type: string) =>
        string | Promise<string>
): Promise<void>;
```

The inspected upstream copy is optional evidence, not a required worker input.
If it is available, its workspace-root-relative location is
`../_tmp-risuai-api-audit/src/ts/plugins/apiV3/risuai.d.ts` when VS Code is
opened at `source`. The canonical contract above remains sufficient when that
temporary checkout is not available.

Known lifecycle risks:

- a single global request snapshot can be stale or reused
- the snapshot is not consumed as a one-shot value
- the pipeline deadline starts after context collection
- an unavailable Composer provider can still exhaust its reserved time
- streaming state is mostly trace data and does not prove one final execution

Known fusion risks:

- candidate confidence is model self-report and dominates current scoring
- whitespace-token similarity is weak for Korean semantic comparison
- duplicate candidates are not collapsed before composition
- Composer output must cover every actionable mutable segment to be accepted
- the final verifier checks structure, not semantic RP regression

## 5. Runtime Contracts

The implementation should keep schemas small. These are logical contracts, not
permission to add a separate framework for every stage.

```text
context_manifest.v1
  snapshot_id
  mode: standalone | connected
  latest_user_input
  recent_messages
  system_and_character_instructions
  character_and_persona
  lorebook_candidates
  lorebook_active_or_injected
  memory_sources
  source_availability
  source_provenance
  collection_warnings
  character_budget

turn_contract_fragment.v1
  planner_role
  required_facts
  knowledge_boundaries
  identity_and_alias_constraints
  relationship_and_emotion_state
  scene_time_location_and_world_rules
  open_threads_and_turn_objectives
  agency_and_pov_constraints
  prose_and_dialogue_targets
  forbidden_regressions
  uncertainty
  evidence_refs

turn_contract.v1
  contract_id
  source_snapshot_id
  mode
  immutable_constraints
  writer_only_secrets
  character_visible_facts
  character_knowledge_scopes
  identity_and_alias_map
  relationship_and_emotion_state
  scene_state
  open_threads
  turn_objectives
  agency_and_pov_constraints
  prose_targets
  forbidden_regressions
  unresolved_uncertainty
  evidence_refs
  source_availability
  injection_budget

prepared_scene.v1
  run_id
  draft_zero
  ordered_segments
  protected_digest
  mutable_ids
  turn_contract
  host_observation_state
  deadline
  attempt_budget

candidate_record.v1
  candidate_id
  segment_id
  role_id
  rewrite
  issues
  evidence_quote
  change_summary
  provenance

semantic_judgment.v1
  per_candidate verdict
  accepted issues
  violations
  conflicts
  coverage gaps
  recommended elements

fusion_plan.v1
  per-segment accepted candidate IDs
  rejected candidate IDs and reasons
  mandatory fixes
  permitted direct gap rewrites

composed_scene.v1
  every mutable segment ID exactly once
  final text
  source candidate IDs
  addressed issues

semantic_proof.v1
  pass / fail
  regressions
  repair targets
  repair instructions
```

### Standalone Context Ownership

The standalone collector uses the following source order:

1. The actual `beforeRequest` `OpenAIChat[]` payload. This is the strongest
   evidence for system instructions, the latest user input, recent conversation,
   and lore or memory text already injected by RisuAI.
2. Official RisuAI APIs for current character, persona/database, current chat,
   and current lorebook entries when the supported API exposes them.
3. SupaMemory, HypaMemory, current-chat summaries, and compatible memory fields
   when they are exposed by the current chat shape.
4. Version-gated fallback fields only when an official value is unavailable.
   Every fallback must remain provenance-labelled and may not be presented as
   confirmed active context.

Lorebook handling distinguishes:

- `lorebook_candidates`: raw entries available to the current character, chat,
  or enabled module;
- `lorebook_active_or_injected`: entries that RisuAI actually injected or that
  an official API explicitly reports as active;
- `unknown_activation`: available entries whose activation cannot be proven.

Recomposer must not invent activation by matching an entry against its own
content or the character name. Regex failures are isolated per entry. The turn
contract may use writer-only secrets to prevent leaks, but must preserve the
difference between what the writer knows and what each character knows.

The bounded contract injected into `beforeRequest` is the only input-stage
mutation. Raw user text, RisuAI source objects, lorebook entries, and memory
records remain unchanged.

## 6. Implementation Order

### Batch 0: Canonical Contract

Goal:

- make this document the current implementation authority
- mark older sequencing documents as supporting or superseded
- keep the active target and limits unambiguous

No runtime code changes.

### Batch 1: Prepare And Lifecycle Correctness

Goal:

- make the host input and one-turn ownership correct before adding another LLM
  stage

Primary existing owners:

- `onBeforeRequest`
- `onAfterRequest`
- `extractPayloadSystem`
- `extractRecentChat`
- `extractLatestUserInput`
- `collectContext`
- `detectStreamingState`
- `createDeadline`
- `callRole` attempt accounting
- Trace initialization and finalization

Required behavior:

1. Read official `OpenAIChat[]` beforeRequest input.
2. Keep one immutable, one-shot request snapshot.
3. Consume and clear that snapshot when the matching main afterRequest starts.
4. Do not reuse stale request context.
5. If overlap cannot be correlated from an official host ID, mark it ambiguous
   and do not invent a hash-based request identity.
6. Keep auxiliary requests outside the main snapshot.
7. Prove that one final response starts the rewrite scheduler at most once.
8. Start the total deadline when main afterRequest begins, before context
   collection.
9. Include context collection and model calls in the same deadline.
10. Add a turn-wide HTTP attempt budget. Do not let every role independently
    consume primary, JSON retry, and fallback attempts without a total cap.
11. Do not add any LLM call in this batch.

Exit gate:

- official array input is collected correctly
- stale or ambiguous context is not injected
- context is consumed once
- deadline covers the full afterRequest path
- execution returns with zero active/background calls
- existing API key, provider, UI, storage, segmentation, and output behavior
  remains intact

### Batch 1A: Standalone Context Manifest

Goal:

- replace the current best-effort context string with a provenance-carrying
  `context_manifest.v1`

Required behavior:

1. Collect the actual request payload, character/scenario, persona, current
   chat, lorebook, and exposed Supa/Hypa/current-chat memory as read-only data.
2. Prefer official RisuAI APIs and label unavailable or unproven values.
3. Separate candidate lore from proven injected/active lore.
4. Record source availability, characters, entry counts, and bounded token
   estimates without storing provider credentials.
5. Keep one bounded snapshot for the matching `afterRequest`.
6. Add no rewrite call in this batch.

Exit gate:

- every contract field has source provenance
- no source is mutated
- unavailable memory or lore remains unavailable, not inferred
- API keys and provider settings are absent from context and Trace
- the existing output pipeline still receives the matching one-shot snapshot

### Batch 1B: Input Planner Fusion And Turn Contract Injection

Goal:

- use multiple specialist planners to produce one bounded `turn_contract.v1`
  before the main model runs

Planner lanes:

- Canon / Secret: identity, aliases, private/public knowledge, POV limits
- Character / Relationship: voice, emotion, relationship stance, user agency
- Scene / Continuity: time, place, world rules, open threads, turn objectives,
  dialogue and prose targets

Rules:

1. Planners return typed contract fragments, not first-draft prose.
2. The deterministic router selects planner lanes by preset and scene signals.
3. Independent planners may run in parallel within provider backpressure.
4. Contract validation rejects unsupported facts and preserves evidence refs.
5. Fusion merges agreement, records conflicts and uncertainty, and never
   silently converts a writer-only secret into character-visible knowledge.
6. Inject exactly one bounded contract block into the real `beforeRequest`
   payload while preserving the raw user message.
7. Carry the same immutable contract into `afterRequest`; do not recollect a
   different truth after the main model responds.
8. Planner failure degrades to the validated manifest-derived contract rather
   than blocking the main RisuAI request.

Exit gate:

- the main model receives exactly one `turn_contract.v1` projection
- the matching output pipeline receives the same contract ID and digest
- no planner writes final RP prose
- live Trace distinguishes source collection, planner calls, contract fusion,
  injection, and later rewrite calls

### Batch 2: Candidate Contract And Dedupe

Goal:

- make the candidate pool compact and evidence-carrying before adding Judge

Required behavior:

- stable `candidate_id`
- evidence quote and model/role provenance
- exact and normalized duplicate collapse
- Korean-compatible character n-gram similarity for near-duplicate grouping
- preserve distinct issue coverage
- record raw and unique candidate counts
- reduce model self-confidence from an acceptance decision to a minor signal

No added LLM call.

### Batch 3 And 4: Semantic Judge Plus Composer Contract

These are implemented in order but released as one batch. Do not ship an
intermediate adapter that keeps both the old semantic scoring path and the new
Judge path.

Judge:

- one scene-level call
- compares original, context, and unique candidates
- returns verdicts, violations, conflicts, gaps, and recommended elements
- does not write final prose

Director:

- consumes Judge evidence and deterministic issue priority
- uses explicit eligible / hold / reject decisions
- removes the old claim that word overlap is semantic judgment

Composer:

- writes every mutable segment exactly once
- rejects missing, duplicate, unknown, protected, and inspect-only IDs
- uses accepted candidates and Judge instructions
- may perform a direct gap rewrite only when the fusion plan explicitly allows
  it
- produces actual final prose, not advice

### Batch 5: Semantic Prover And One Repair Cycle

Goal:

- catch regressions introduced by composition

Order:

```text
Composer
-> Structural Verifier
-> Semantic Prover
-> if failed and budget remains: one targeted Repair
-> Structural Verifier
-> Semantic Prover
```

Semantic checks:

- secret and private-knowledge leakage
- POV knowledge boundary
- identity continuity
- user agency
- plot and world continuity
- character voice regression
- unsupported new facts

There is exactly one repair cycle. A repair that is not re-proven is not a
verified enhanced result.

### Batch 6: Deterministic Fugu-Lite Routing

Goal:

- choose existing roles and validation stages without adding an orchestrator
  model

Rules:

- Fast selects at most two specialists plus Composer.
- Balanced selects at most three specialists plus Composer.
- Quality selects at most four specialists plus Composer from the six-role
  profile pool.
- `secret_pov_guard` remains a required baseline in Balanced and Quality.
- High-severity meta/mechanical artifacts prioritize `agency_meta_guard`.
- The router may not exceed the user-selected preset.
- Capacity-skipped roles are reported with their score and skip reason.
- The router may recommend Table Read but may not create temporary roles,
  providers, models, or recursive work.

### Batch 7: Table Read Escalation

Table Read is added only after the adaptive Composer path passes live
validation.

Placement:

```text
Semantic Judge
-> unresolved important multi-character conflict
-> Table Read evidence
-> Fusion Director
-> Composer
```

Table Read supplies support, challenge, and unresolved notes. It does not write
the displayed scene directly. Archive Center per-character subjective memories
remain a future connected mode.

### Batch 8: Quality And Beta Gate

Compare the same `draft_zero` under blinded labels:

- original draft
- one strong Composer call
- grouped three-lane specialists
- adaptive specialists plus Composer with a distinct-provider fallback

Start with 20-30 pilot turns, then use 40-60 Korean and English scenes covering:

- secret and hidden identity
- POV changes
- user agency pressure
- continuity and world rules
- dialogue and character voice
- prose rhythm and repetition
- long output with protected markers

Record:

- blind preference
- hard semantic regressions
- protected-byte preservation
- logical calls and HTTP attempts
- input/output tokens
- p50 and p95 latency
- timeout, fallback, and unchanged rates

Unit tests and syntax checks are hygiene gates, not evidence of prose quality.

## 7. Call And Attempt Budget

Normal logical calls:

| Preset | Normal path | Conditional maximum |
|---|---:|---:|
| Fast | manifest contract + up to 2 specialists + Composer | Composer fallback |
| Balanced | 2 input planners + up to 3 specialists + Composer | Composer fallback |
| Quality | 3 input planners + up to 4 specialists + Composer | Composer fallback |

Stage-separated HTTP attempt caps:

| Preset | Input cap | Output cap |
|---|---:|---:|
| Fast | 0 | 4 |
| Balanced | 3 | 5 |
| Quality | 4 | 6 |

Input usage remains visible in Trace but does not consume the output budget.
Output specialists do not retry. Composer uses its primary and then a distinct
configured fallback when necessary. Logical calls and HTTP attempts remain
separate Trace values.

## 8. Implementation Discipline

- Change only `source/Risu Recomposer.js` for runtime work.
- Do not add a second runtime file, backend, package, or service.
- Preserve provider/model profiles, reasoning settings, extra headers/body,
  prompt editing, API key storage, UI persistence, and trace key redaction.
- Prefer replacing an existing owner over adding a parallel helper chain.
- No broad formatting or encoding rewrite.
- Every batch reports functions changed, tests added, total tests, syntax
  result, bytes, lines, and JavaScript lines added/removed.
- Live RisuAI behavior remains `implemented_unverified` until exercised in the
  host.

## 9. Next Worker Prompt

The next implementation task is Batch 1A only. Do not add planner LLM calls
until the official standalone context manifest, provenance, one-shot snapshot,
and source-availability Trace pass their exit gate.
