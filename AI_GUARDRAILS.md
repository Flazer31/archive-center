# Archive Center AI Coding Guardrails

## 2026-09-09 — 4.3.0 stable release and OpenCode Zen / Go

Source version is 4.3.0 (stable), published with main and tag updated; package and actual public Windows 4.1/4.2 update evidence is
tracked in [release verification](docs/archive-center-4.3.0-release-verification.md).
`proxy_provider.go::callProxyProviderWithPolicy` routes the explicit `opencode`
and `opencode-go` providers through existing model-native adapters. Explicit API endpoints retain
priority. `proxyProviderBaseURL` supplies the Zen default; OpenRouter retains its
existing default and transport. JS changes are provider options, endpoint hints
and reasoning controls, plus the existing direct proxy's optional
`chat_session_id` query observation. Go carries the existing session ID from
Publisher/Critic/preprocessing into an internal request policy and hashes it for
Go's `x-opencode-session` header. Go identifies itself as ArchiveCenter/4.3.0;
configured extra headers retain precedence. Calls without a chat use an auxiliary
identifier. These identifiers never enter memory ownership or prompts.
MiniMax/Qwen Messages routes keep the prompt JSON contract without adding
Claude-specific structured-output fields. Shared provider tests cover Publisher, Critic and
preprocessing purposes with an external HTTP fixture, not live provider proof.
No memory policy, storage schema, lifecycle or fallback is added in this slice.


Use this file as an operational checklist. Treat the active implementation and its actual callers as stronger evidence than filenames, comments, roadmaps, generated packages, or previous conversations. Treat unqualified rules below as **VERIFIED** from source or explicit architecture. Mark unresolved behavior **UNKNOWN**, current roadmap-only work **PLANNED**, and superseded/inactive surfaces **OBSOLETE**; do not guess.

## 1. Files to Read Before Editing

### 4.3 test.23 — pending branch source and cold-start merge — 2026-09-09

- Go's existing lineage resolver can use the named parent's observed message
  prefix before its last source revision is persisted. `worldline_message_origins.go`
  anchors that position to stored source coordinates or the parent's confirmed
  inherited endpoint; the existing routing calculator handles the unanchored
  root prefix. Source persistence and Critic completion are separate operations.
- Host origin observations remain optional. Preserve direct source/history
  matching and the existing user/char inherited-through boundary. Do not flush a
  parent's next-input marker from a child or add a save-on-switch/background job.
- `computeActiveChatRescanDryRunPlan()` must apply Go's existing
  `skip_pre_route_visible_pair` decision in both merge passes. A positive turn
  number on that decision is not authorization to rebuild inherited memory.
- Reuse the existing assistant persistence normalizer for second-pass text.
  Keep Host observation hashes separate from normalized persistence content.
  This repairs GigaTrans translation text re-entry; it does not add a new regex
  setting, translation policy, source admission rule or memory-selection policy.
- The real JS merge and real Go routing API are exercised together with fixture
  Host/Store boundaries. Source-save delay, reissued IDs, offsets and deeper
  rebranches have separate owner regressions. See the [test.23 record](docs/archive-center-4.3-test-build-23.md).
  Actual RisuAI/DB/provider verification remains `implemented_unverified`.

### 4.3 test.22 — bounded preprocessing finishing — 2026-09-09

The user first requested a local rollback checkpoint. It is commit `6ef8f74`,
with test.21 retained. This slice repairs the existing Go preprocessing owner.

- Normalize only the reproduced string `question`/`query` object forms inside
  search_requests. Preserve independent fields and existing partial decoding.
  Do not apply question-object decoding to selected memory references.
- Each role owns the order of its accepted fact recommendation. event_recent's
  summary list has separate ordering. Other roles' mentions retain their own
  context and cannot overwrite that order. Preserve Go no-recommendation ordering.
- Prompts prepare attributed evidence and possible relevance, with the user's
  creative direction authoritative. Source time, planned time and current scene
  time stay distinguishable. Gaps accompany usable evidence; they are not new
  rejection criteria. Saved custom prompts remain authoritative.
- Compact only the rendered provenance keys and repeated uncertainty scope.
  Preserve full diagnostic catalog/items, source text and IDs, owner/viewer scope,
  null/absence, received reasons and uncertainty. Keep Publisher support connected
  to the full catalog. Add no post-selection character cut or reinterpretation.
- Test.21 retrieval breadth, importance/recency, budgets, secret scope, existing
  empty/failed recommendation behavior and maximum two analysis rounds remain.
- Verification uses production owners and recorded provider replies at a local
  fixture endpoint, with zero external AI calls. Prompt quality requires the next
  real user test; offline reply replay cannot establish improved creative output.
  See [test.22](docs/archive-center-4.3-test-build-22.md). User owns backend startup.

### 2026-09-08 planning alignment — shared recall and optional Actor expression

The user approved the [reliable recall plan](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#good-memory-plan).
It is **PLANNED**, with test.21 as the unchanged runtime/package baseline.

- Base recall is evaluated with preprocessing and Publisher OFF. Optional AI
  selection has separate evidence of benefit and keeps its accepted text/order
  and existing no-recommendation or failed-supplement behavior.
- 5.1-A–5.2 improves shared accessibility, reactivation and scene-cue evidence
  delivery. 5.1-B and 5.3–5.7 describe optional Actor memory expression. Actor
  dormancy/partial-expression states do not add rejection or truncation to the
  ordinary memory path; existing knowledge and privacy scope remains intact.
- Reuse the six documented cases across versions and the four ON/OFF combinations.
  Keep storage, retrieved candidates, selection, actual input and displayed use
  distinct. Compare necessary facts, wrong time/source mixing, irrelevant content
  and cost; item counts and successful calls alone are not recall-quality proof.
- Importance/recency separation and test.21 delivery breadth are implemented
  baselines. Future plans do not authorize new calls, gates, fallback layers,
  schemas or JavaScript policy. Each implementation still follows the active
  Go owner, approved scope and the existing test-integrity contract.

### 4.3 test.21 — delivery breadth and shared baseline — 2026-09-08

- The user authorized this four-item bundle. The existing maximum-count setting
  now means a per-group core priority target. Use existing character budgets for
  remaining details; do not restore a separate count cutoff. Wire key/value is
  retained; UI names and `core_priority_memory_delivery.v4` express the new meaning.
- Keep ordinary admitted `subjective_memory` in typed, holder-scoped normal
  candidates. Carry source unit/turn and holder/viewers through selection; actual
  protected knowledge keeps the existing guidance path. Do not classify privacy
  from prose or promote a character's recollection to objective world truth.
- Current-field grouping uses source chronology before relevance. Candidate IDs
  distinguish the source/turn/value; grouping identity remains separate. Preserve
  first-round IDs so failed supplements retain the previous recommendation, while
  new sources reach the actual second-round provider input.
- Empty/failed recommendations use the same Go baseline and rendering order.
  Preserve accepted AI original text and per-group order without a new cutoff,
  replacement, retry, third round, schema change or persistent-state mutation.
- Verify OFF, AI, empty and partial failure together, plus second-round failure,
  source time and actual final character caps. New regressions must reproduce the
  old failures. Frozen exports with synthetic metadata are delivery controls,
  not live database replay or proof of RP quality.
- See [test.21](docs/archive-center-4.3-test-build-21.md). JavaScript changes only
  setting labels/help and build identifiers: +10/-10 lines, zero policy growth.
  Source/regression/package status is verified; live status is implemented_unverified.

The test.21 policy supersedes older hard-K/no-backfill wording below. Preserve all
unrelated existing behavior and the user's saved prompt/configuration overrides.

### 4.3 test.20 — input presentation and selected-evidence references — 2026-09-08

- `multiAgentModelInput()` is a presentation projection at the existing provider call.
  Preserve the canonical `input`, original text/ID/order and all source/owner/viewer
  metadata. Only repeated identical provenance is shared; distinct knowledge holders
  remain distinct. `model_input` is the exact serialized user message, not a second
  selection owner. Candidate text limits are not total wire/system prompt limits.
- Use the same request-local F/S references in AI-selected memory and its notes.
  P entries describe provenance; they are not fact IDs. Preserve AI reason text and
  uncertainty as interpretation. Source sharing must not merge separate facts.
- When changing the rendered memory line, keep `supervisorDeliveredContextItems()`
  connected to the original fact/complete-summary source. The real Publisher
  projection must carry note evidence refs and `preprocessing_source_catalog`.
- Existing K/character budgets, accepted recommendation order/text, public handoffs,
  no-recommendation Go selection and supplemental failure behavior remain in force.
  Add no post-hoc cutoff, semantic rejection gate, retry or extra analysis round.
  The two shared default-prompt refinements keep saved user overrides authoritative.
- Validate production provider input and final assembly, including Publisher OFF,
  success/failure, separate private holders and all-empty Go behavior. Frozen
  round-trip equality is data-preservation evidence, not model-quality evidence.
- [test.20](docs/archive-center-4.3-test-build-20.md) is implemented/unverified at the
  live tier. External comparison is awaiting explicit payload/destination approval;
  do not treat a sandbox-blocked request as a successful provider test. Package
  creation does not authorize backend startup or changes to settings/chat/data.

### 4.3 test.19 — memory editors and request purpose — 2026-09-08

The five existing specialists collectively prepare memory as continuity editors.
`prepare_turn_multi_agent.go` owns the shared and role defaults: connect recorded
history, supplied transitions and present relevance; keep user-directed story
changes, pace and outcomes separate from archival evidence. Selected text/order,
independent K, empty-result Go selection and the existing two rounds are unchanged.
Notes assist the main model with or without the optional Publisher.

`runMultiAgent()` adds `request_reason` beside `from_role` in each existing accepted
public `related_evidence` item. It is the sender's AI question/interpretation,
separate from canonical text and provenance. It does not transfer candidate lanes
or become a fact. Prompts ask for reasons derived from shared public evidence.
The existing public/owner/viewer/subjective scope is unchanged; this does not prove
arbitrary AI prose is free of private content. No new filter, call, round or writer
is added. Recipient findings join final preparation, not another reply round.

The [test.19 record](docs/archive-center-4.3-test-build-19.md) separates failing-before/
passing-after production-path tests, package identity and the observed UI save.
Only the six matching old default overrides were reset through the user's UI to
use backend defaults. Other users' custom prompts remain explicit overrides.
The new backend must be started by the user before its defaults and handoff run.

### 4.3 test.18 package — 2026-09-08

The [test.18 record](docs/archive-center-4.3-test-build-18.md) packages the following
HUD, static.v4 scoring, knowledge-continuity and original-Hypa-import source repairs.
Earlier source-only notes describe evidence at their repair time. The Windows
package adds active-source/plugin/prompt/migration identity and 53-file/ZIP hash
verification. It does not establish a running backend, loaded RisuAI or live recall.
Use the existing package builder and its launcher-owned version; `.env.full.example`
is the runtime template, while `.env.source.example` is an unchanged source reference.
The backend differs from test.17, so apply plugin and backend together.
Package creation does not authorize starting the user's backend or changing their data.

### 4.3 retrieval score retention — 2026-09-08

The active source uses `priority_score.static.v4`; see the
[source repair and regression record](docs/archive-center-4.3-feedback-work-log.md#retrieval-score-retention-repair).
`prepare_turn_priority_memory.go` carries an existing aggregate Memory vector
observation into its complete-summary score. Keep the higher of the existing
best-child score and the aggregate-vector score. This is a source-level score;
it must not become every sibling fact's relevance. A lexical parent score is
not a vector observation. Existing precise-unit scoring still belongs only to
the matching fact. Summary trace records the observed vector and score origin.

Importance contributes independently of RP recency. Preserve the stored value
and use `0.60*relevance + 0.25*importance + 0.15*recency` plus existing biases.
The retained `importance_after_turn_decay` trace field equals importance in v4;
`importance_decay_policy` is `stored_importance_preserved_recency_separate`.
Do not reintroduce importance multiplied by age, invent new tiered decay, or
interpret a high score as current-state validity or an instruction to act.
K, budgets, source scope, received AI ordering and optional-call behavior stay
with their existing owners. No extra search, LLM call or canonical writer exists.
Source/fixture checks and the test.18 package record are verified at their respective
boundaries; loaded/live behavior remains open.

### 4.3 knowledge continuity first repair — 2026-09-08

The reproduced failures and repair evidence are in the
[feedback log](docs/archive-center-4.3-feedback-work-log.md#knowledge-continuity-first-repair).
`character_perspective.go` distinguishes independent secret/episodic claims
inside category slots; single-valued belief slots retain their current-state
resolution. `known` and `revealed` describe compatible knowledge of the same
claim. Original states remain in storage and display. Keep holder isolation,
actual unknown/known conflicts and source acceptance with their existing owners.

`turn_precise_memory.go` feeds supplied identity-mapping evidence through the
existing protected per-holder observation writer. `turn_extraction_private.go`
preserves the supplied identity/secret ID, identity evidence and transition;
`turn_entity_identity.go` includes explicit identity-scope holders. The default
Critic prompt describes these fields; no new LLM call or rejection rule is added.

`prepare_turn_memory.go` uses recorded owners/knowers for POV identity, separating
the owner's cover identity from an informed observer's knowledge. Canonical
memories passed by the assembly supply current disclosure context. A recorded
release retires the same secret's obsolete guidance and records the release
source in delivery lineage. Original/public event rows remain unchanged. A shared
category alone never identifies the same secret. Legacy matching uses identical
recorded content; semantic paraphrase resolution is not implemented here.

This repair is included in test.18, with production-owner regressions. It does
not establish a running backend. Missing old holder records are not automatically
recreated. Loaded Host, live storage/provider and displayed RP proof remain open.

### 4.3 HypaMemory original import — 2026-09-08

The authorized import contract is one supplied nonempty Hypa summary per memory.
`handleImportHypamemory()` keeps the original in `turn_summary` and
`hypamemory_import.original_text`; the Critic's summary is supplemental metadata.
Use the existing `saveCriticExtractionArtifacts()` writer and negative external
import namespace. Critic configuration/failure affects analysis, not whether the
available original is submitted for storage. Do not fabricate extraction rows.

Go resolves occupied import numbers and counts existing original occurrences.
Repeated equal summaries in one source list remain distinct occurrences;
reimport consumes the corresponding saved occurrences. Old rows without original
metadata remain untouched. This does not reconstruct missing originals.
Keep original metadata out of the general public projection; the memory body
uses the existing visibility projection. Positive RP source admission is unchanged.

Memory/evidence/KG history readers include negative external imports within the
existing session segments while retaining positive branch boundaries. Keep the
range-reader arguments; `mariadb_chat_memory.go` includes negative rows in SQL
instead of loading a branch's whole positive history. Explorer
`source=hypamemory` filters before pagination and includes old scoring-labelled
imports. JS only observes the active chat's `hypaV3Data.summaries`, sends the
request, renders backend counts and loads the memory filter. The request is
synchronous; completion text must not claim a background analysis is starting.

Source/fixture evidence and live gaps are recorded in the
[4.3 feedback log](docs/archive-center-4.3-feedback-work-log.md#hypa-original-import).
The import does not traverse split chats or replace cold start. It is packaged in
test.18; application to the user's running backend/RisuAI remains unverified.

### 4.3 status and 4.4 planning — 2026-09-07

Use the [4.3 status summary](docs/archive-center-4.3-status-summary.md) to locate
current owners and distinguish source/regression/package records from
loaded Host, actual provider, payload application and displayed-output evidence.
Optional role calls, prompts and UI are implemented in source; the original
world_state first-call error, race-detector and broad live/40M evidence remain open.

The [4.4 file/function plan](docs/archive-center-4.4-refactoring-plan.md) is
**PLANNED**. At implementation start, capture the then-current 4.3 baseline and
preserve unrelated dirty edits. Proceed through provider/settings, assembly/HUD,
then the original semantic-consolidation scope. Refactoring compares existing
behavior; consolidation has separately documented intended changes. Neither
comparison creates a new runtime output or persistence rejection condition.
Keep Go/Host ownership, received AI recommendation text/order, existing
no-recommendation behavior, source/privacy scope and user creative authority.
Do not treat this documentation update as a runtime refactor or verification run.

### 4.3 test.18 — compact HUD presentation

The 2026-09-08 UI follow-up, packaged in test.18, narrows the HUD to 224px with 10px padding
and renders backend preparation details as a total plus two-column timing cards.
Generated/stored counts use matching two-column label/value tiles and a bordered
total summary. Keep the existing count values, order and phase visibility owner.
Preserve native detail expansion and each source value, including zero duration.
This affects CSS, labels and existing HTML only; no timer or status calculations.
The earlier test.17 package's 268px layout and loaded Host evidence stay distinct.

Use the existing Host phase projector and renderers for HUD-only changes.
In `next_user_input`, current generation carries no storage count ledger;
previous finalization keeps its real counts and omits repeated generation
timings. Immediate-mode storage counts remain available, including an actual
zero result. Never infer a successful save or change the backend status from
`host_generation_finished`; only the projected waiting-stage display changes.

Keep total/preparation/response metrics and the five-role round table visible;
collapse storage breakdown, stage ledger, supplemental search and backend
timings. Avoid repeated explanation paragraphs. Normal completed cards follow
the existing card/X dismissal policy; native detail toggles keep the card open.
Observe the detail-open count through SafeElement, since trimmed Host clicks
omit event.target and native toggling moves the centered HUD before async handling.
Keep warning/error cards X-only and preserve the previous completed card's X.
Preserve unrelated notice
dismissal and the existing event/timer/stream owners. Provider Flex capability
handling remains unchanged when removing the explanatory settings paragraph.

See [test.18](docs/archive-center-4.3-test-build-18.md). JS is required here for
RisuAI DOM presentation; this change adds no Go workflow, contract, search,
acceptance, persistence or fallback rule. Verify source regression and isolated
desktop/mobile rendering separately from loaded RisuAI.

### 4.3 test.16 — production public-projection handoff

`appendPrepareTurnPriorityMemoryFactSeeds()` labels memory-derived public facts
as `public_projection` unless item metadata overrides it. `runMultiAgent()`
includes that label in its existing public handoff branch alongside public and
general. Preserve the existing owner/viewer/subjective scope checks, canonical
text/order/source metadata and role-specific selection ownership. Related
evidence provides context; it does not reclassify facts into the receiving lane.

Validate production memory assembly through the registered `/prepare-turn`
route and inspect the receiving specialist's second HTTP request, not just
handcrafted public/general candidates. The regression failed before the fix
and covers Publisher OFF/success/failure, canonical final payload and support.
Retain public/general and private-scope cases, including projection-labelled
owner/viewer/subjective evidence. See the [test.16 record](docs/archive-center-4.3-test-build-16.md).
This is a repair to existing Go delivery, with no new gate, prompt, model call,
schema, retrieval policy or Host behavior. JS changes are version identifiers.
Keep actual Host/provider verification distinct; backend restart is user-owned.

### Planned 4.4 / 4.6 / 4.7 state-time follow-up — 2026-09-07

Use the [canonical state-time plan](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#temporal-state-44647)
for the repeated mixed-balance case. The supplied test.15 candidates give old
asset text and a cash field the same latest snapshot turn. Trace the existing
state merge, row update time, per-field source metadata, recency and candidate
supply before attributing the result to Publisher or specialist prompting.
Existing time labels and received-note delivery do not resolve this cause.

4.4 reduces equivalent duplicates and prepares cases; distinct historical
values retain their source/time differences. Keep A-C behavior-preserving
refactoring separate from intended state semantics changes. 4.6 owns per-field
change/effective time and current-state work, explicitly including character
funds/assets/inventory. 4.7 consumes that evidence in retrieval/ranking and AI
input; a weight change over the same incorrect source time is not a repair.
Compare Go selection and AI-assisted selection separately while preserving
received AI source text/order and the existing no-recommendation behavior.

New-write improvements and legacy recovery have separate evidence. Recover old
times only from available original/history evidence; retain uncertainty and
original material otherwise. Case identifiers and balances are fixtures, not
runtime classifiers. These tests are development evidence, never additional
output/storage rejection conditions. This remains PLANNED documentation with
no new API/schema, automatic repair job, provider call, prompt change or build.

### 4.3 test.15 — independently delivered specialist interpretations

Use the existing accepted preprocessing selection as the owner of its reasons
and unresolved questions. Preserve received text/order, final accepted round,
and the independently retained lore assessment round. Attach reasons to the
selected evidence actually delivered; keep AI questions separate from transport
or search diagnostics. Existing Go baseline selection supplies memory without
fabricating specialist explanations.

`buildPrepareTurnPreprocessingNotes` is Go-owned request assembly. Preserve
source references and character visibility/owner/viewer metadata beside each
interpretation. Notes express AI interpretation, not stored truth, and metadata
does not establish perfect model compliance. The user's story choices and
revisions remain authoritative; this task adds no creative prohibitions.

Deliver the same received notes through Publisher support and a separate main
payload lane, including when Publisher is OFF or fails. Show that lane in Edit
Check and count its extra input independently of original memory and guidance
budgets. Keep existing Host application/observation; JavaScript only displays
Go results. No additional AI calls, rounds, retries, selection re-ranking, new
acceptance conditions, schema/storage writes or postprocessing belong to this
slice. Preserve saved user prompts and runtime state. Stop after the test build
so the user can evaluate the result; fixture success is not live output proof.

### 4.3 test.14 — shared response recovery and usable partial results

Use the existing `repairJSONCandidate` owner for AI-response syntax recovery in
Critic, Publisher and preprocessing. Preserve quoted narrative/source text and
the original response. A complete array followed by an explicit sibling key or
enclosing object close can supply its missing `]`; EOF is not new content.
Keep existing Publisher/Critic contracts, persistence and failure policy intact.
Do not turn syntax recovery into inferred facts, guessed IDs or rewritten plans.

Preprocessing fields decode independently. A scalar string can represent one
string-list item; already decoded items and later readable fields survive type
errors. Recovery introduces no retries, searches or analysis beyond the existing
limits. A parsed lore assessment survives unrelated field errors; real provider
failure and failed supplemental memory recommendations preserve prior behavior.
HUD call outcomes and final memory selection source are observations, never new
acceptance conditions or claims of final payload/display application.

Use existing observed lore comment/key/heading for a label next to its short ref.
Keep original source content, candidate budgets/order and edited prompts. All
providers and model sizes retain the same settings and transport paths.

The pending-thread candidate path alone uses the already configured Host recent
conversation query set via `stringsFromAny`. Keep source/session scope, suppression,
other support lanes and scoring unchanged. Confirm the production assembly and
specialist input retain an ongoing goal named in recent context and still omit an
unrelated goal. Broader state-time interpretation and old goal closure remain
separate from this change. Source and fixture tests do not establish live RP quality.

### 4.3 test.13 — source-time accuracy and recent context

`character_states` is a cumulative snapshot: its row turn can be newer than
individual retained fields. Keep the original source turn, IDs, text and scoring;
describe that provenance instead of inventing per-field dates or resolving
conflicting story facts in Go. Specialist candidates and public cross-role
handoffs carry the existing `source_table`; `reference_format.source_turn`
explains snapshot time even with user-edited task prompts. Selected snapshot
text uses `[state snapshot turn N; fields may be older]` through the existing
memory renderer and Publisher support path. Other source labels are unchanged.

The UI's existing recent-conversation count supplies completed user/assistant
conversations to both retrieval and specialist rounds. Keep current input
separate and preserve observed quantities, uncertainty and transitions. Default
prompt advice distinguishes an established action from its unknown details;
user prompt overrides and creative revisions remain authoritative. There is no
new completion verdict, AI call, search, rejection or persistence path.
Source/provider-fixture checks are recorded in the preprocessing work log.
The [test.13 package](docs/archive-center-4.3-test-build-13.md) contains these
changes with 53 managed file hashes verified; loaded Host remains unverified.

### 4.3 test.12 — supplemental-search latency and timing

Keep the existing per-role search and analysis limits, history/source/perspective
scope, filters and candidate content. Independent supplemental retrieval runs in
parallel; merge indexed results in role order and allocate references before
round two. Preserve first-role duplicate ownership, received AI order/text,
partial results and existing no-recommendation behavior. Concurrency is not
permission to add searches, retries, output rejection or a new selection path.

Hydration/assembly uses one request-local mutex around existing shared inputs.
Each retrieval owns its timing map; only the coordinator writes the final request
timing and result lists. Measure the supplemental phase by wall time, including
merge, rather than summing concurrent query durations. Per-query breakdowns are
health, embedding, vector_search, revision_checks, hydration, assembly_wait and
assembly. They explain observed work and must not change acceptance decisions.

Reuse typed pristine candidate snapshots from the original assembly, before AI
substitution/render mutation. Copy nested viewers, identity metadata and member
IDs. Keep snapshots unexported and request-local; never rebuild them from public
diagnostic maps that omit private source text. No persistent cache is involved.

The optional HUD `preprocessing_search` field uses the existing ledger/stream and
timer. Deep-copy query timing maps; omit it when the feature performs no search.
Expose only counts, role/status and timing, with wall time clearly distinguished
from overlapping query durations and enclosing backend stages. Verify actual
registered-route overlap, deterministic second-round inputs, final Go payload,
OFF, partial failure and production JavaScript rendering/timer behavior.
Source/fixture success is not live RisuAI/provider or race-detector evidence.
See [test.12](docs/archive-center-4.3-test-build-12.md).

### 4.3 test.11 — user-directed execution priorities by strength

The later 2026-09-07 user clarification allows stronger response-level direction
from Strong upward while preserving the user's creative freedom and decisions.
Keep the selected strength's policy visible to both the Publisher and the main
response through the existing profile/request/render owners. Weak/Medium remain
advisory. Strong prioritizes concrete enactment of the user's chosen direction;
Extreme connects action/reaction/consequence; Maximum organizes the current
response's development. The user's direction, explicit revisions, pacing and
decisions take precedence. A quiet scene, an attempted action or an open choice
can receive concrete depiction on its own terms.

This is prompt and rendering behavior, not an output compliance gate. Preserve
received item content/order and the existing acceptance, budgets, source scope,
model/temperature/token configuration, single call and lifecycle behavior.
Strength remains independent of pressure. Keep the four default Publisher fields
and existing user prompt overrides. Verify the real Publisher request and final
payload across strengths/formats, including unchanged item acceptance and None.
See [test.11](docs/archive-center-4.3-test-build-11.md).

### 4.3 test.10 — user-directed storytelling and preprocessing repair (prior checkpoint)

The 2026-09-07 user clarification governs model-facing guidance: Archive Center
supplies memory context and optional ideas, while the user chooses and can change
the story. Rephrasing a prohibition positively does not satisfy this requirement.
The default Publisher requests `current_arc`, `narrative_goal`, `next_beats` and
`pressure_level`; it no longer requests forbidden moves, required outcomes or a
scene mandate. Historical context can inspire new developments and user revisions.
Keep stored user prompt overrides intact. Existing parsing compatibility and
canonical persistence/Host acceptance are separate from these creative suggestions.

Verify current `prepare_turn_multi_agent.go` and `prepare_turn_lorebook_reference.go`:
fact/summary input sharing stays within the existing candidate-text character cap;
request-local F/S/L references map exactly to supplied IDs and remain stable across
the two rounds. Preserve valid selection order and source text. Public `general`
evidence uses the existing public handoff scope, with private ownership unchanged.
The world role's optional `selected_lorebook_refs` is independent of its fact
selection: explicit `[]` means no extra AC lorebook text; omission/failed assessment
keeps the existing Go selection or successful earlier assessment. Its received
source order survives finalization, including duplicate-content coalescing; native
Host lorebook text remains Host-owned. Positive configured budgets are supplied
before selection and excess received selections remain visible with actual usage.
Feature modes, zero budgets and existing scope/payload-presence handling remain.

Use the existing preprocessing trace to distinguish source candidate counts,
missing configuration fields, local request failure, actual dispatch and partial
search. Successful Chroma responses are read completely; bounded HTTP-error bodies
remain bounded. Do not label a ready hydration as successful dedicated search.
HUD stage intervals may overlap; `preprocessing_search` is inside
`injection_assembly`, and `supervisor_llm` is inside `response_assembly`.
Effective Input distinguishes backend preview from observed pre-request content;
neither proves the final provider request or displayed effect. The test8 mismatch
itself remains unconfirmed without its original parity observations.

Evidence and remaining live checks: [test.10](docs/archive-center-4.3-test-build-10.md).

Choose the smallest evidence set that can verify the requested change:

- For implementation, behavior, contract, schema, storage, packaging, or architecture changes, read [`AGENTS.md`](AGENTS.md) and use this file as the operational checklist.
- Use [`STRUCTURE.md`](STRUCTURE.md) as a routing map. Read its current-status section and the sections relevant to the task, then verify every relied-on path, symbol, caller, entry point, and failure path against the active implementation. Do not require a full cover-to-cover read when the task has a bounded owner.
- Read [`docs/permanent-risu-host-backend-boundary.md`](docs/permanent-risu-host-backend-boundary.md) before changing runtime ownership, API responsibilities, or behavior shared by `Archive Center.js` and the Go backend.
- For documentation-only spelling, formatting, or link repairs that do not change a technical claim, inspect the affected document and nearby references. For any changed technical claim, inspect the cited active source and its actual caller.
- Use [`docs/archive-center-4.0.8-work-log.md`](docs/archive-center-4.0.8-work-log.md) and [`docs/archive-center-4.0.8-to-4.0.9-work-log.md`](docs/archive-center-4.0.8-to-4.0.9-work-log.md) as change/evidence indexes. Check their later superseding entries and verify every operational claim against current code; never restore an intermediate fix merely because it is documented.
- Inspect the exact target, its callers, its interfaces, and the production tests that exercise its owner. Do not rely on a similarly named backup, package copy, fixture, or test-only implementation.

Read the applicable owner set before changing behavior:

- For RisuAI lifecycle or payload work, inspect [`Archive Center.js`](Archive%20Center.js), especially `registerRisuLifecycleHooks()`, `onInputHook()`, `onBeforeRequest()`, `onAfterRequest()`, `onRisuOutput()`, `captureFinalConfirmationRequestContext()`, `applyGoPayloadApplicationPlan()`, and the corresponding official RisuAI hook/type source for the supported version.
- For translation-plugin interoperability, inspect the exact referenced plugin version and its stored source contract before changing Archive Center. For Yumi Translator 1.4.2, keep Risu's translated display and outgoing payload untouched; use only complete `yumi-tr:v1` assistant marker ranges and matching `$__yumi_tr.<id>` `scriptstate` records to form a non-mutating Archive-only read copy. Do not edit the external plugin, guess generic XML/tag layouts, add ordering requirements, or turn missing translation metadata into a request rejection.
- For 4.1 PDF memory transport work, read [`archive-center-4.1-pdf-memory-transport-plan.md`](docs/archive-center-4.1-pdf-memory-transport-plan.md), then inspect the current Go lane producer, the current request-owned retry context and the official RisuAI body-interceptor/body-shape source. For the explicit Yumi Provider Manager experiment, also inspect the exact documented manual `<pm-pdf>` contract of the tested Provider Manager version; do not edit that external plugin or infer that it is loaded. Treat `pdf-memory-experiment` as frozen historical evidence, not active implementation. Keep memory selection in Go, use only the already-selected `long_term_memory`, preserve every other lane and the existing text mode, and do not add model-name/provider inference or another output/save acceptance path.
- For backend startup or route ownership, inspect [`go-service/cmd/archive-center-go/main.go`](go-service/cmd/archive-center-go/main.go), [`go-service/internal/httpapi/server.go`](go-service/internal/httpapi/server.go), and the registered route group.
- For retrieval or injection, inspect [`group_turn_prepare.go`](go-service/internal/httpapi/group_turn_prepare.go), [`prepare_turn_recall.go`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepare_turn_memory.go`](go-service/internal/httpapi/prepare_turn_memory.go), [`prepare_turn_assembly.go`](go-service/internal/httpapi/prepare_turn_assembly.go), [`prepare_turn_memory_budget.go`](go-service/internal/httpapi/prepare_turn_memory_budget.go), [`prepare_turn_lorebook_reference.go`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go), [`prepare_turn_render.go`](go-service/internal/httpapi/prepare_turn_render.go), and [`output_fidelity_lineage.go`](go-service/internal/httpapi/output_fidelity_lineage.go).
- For Publisher changes, inspect `runSupervisorLLM()`, `publisherStrengthProfile()`, `buildBoundedSupervisorResult()`, and their callers in [`group_proxy.go`](go-service/internal/httpapi/group_proxy.go), the consumer in [`group_turn_prepare.go`](go-service/internal/httpapi/group_turn_prepare.go), the renderer in [`prepare_turn_render.go`](go-service/internal/httpapi/prepare_turn_render.go), and [`prompts/supervisor_system.txt`](prompts/supervisor_system.txt).
- For the **implemented_unverified** 4.3 optional multi-agent feature, inspect `prepare_turn_multi_agent.go` and its registered settings route and `group_turn_prepare.go` callers alongside the [current integrated roadmap](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#multi-agent-memory-selection-43). Preserve first-round parallel analysis, explicit missing-evidence requests and at most one supplemental analysis per relevant active specialist (N+M normal analysis calls, 0≤M≤N≤5). Preserve current input authority and existing source/branch/perspective scope in searches and cross-category evidence; analysis prose is not canonical truth. Go executes received canonical recommendation text/order without discretionary replacement by its own ranking/prose; categories without recommendations retain ordinary Go selection. Search limits, unresolved requests and partial model/search failures must not become new output/persistence rejection conditions. Count retrieval, embedding, analysis and provider re-entry separately. JavaScript remains the Host adapter and the existing Publisher consumes delivered memory. Exclude drafts, output rewriting/polishing, new AI memory writers and a second Publisher; retain existing Critic persistence and leave bundle selection/compression to 4.5. Source implementation and roadmap fields do not prove loaded Host or actual provider behavior.
- For Host lorebook changes, inspect `observeLorebookReferenceScope()`, `syncCurrentLorebookReference()`, and `postLorebookReferenceSnapshot()` in [`Archive Center.js`](Archive%20Center.js), [`group_lorebook_reference.go`](go-service/internal/httpapi/group_lorebook_reference.go), [`store/lorebook_reference.go`](go-service/internal/store/lorebook_reference.go), [`store/mariadb_lorebook_reference.go`](go-service/internal/store/mariadb_lorebook_reference.go), and [`migrations/010_lorebook_reference_entries.sql`](migrations/010_lorebook_reference_entries.sql).
- For completed-turn persistence, inspect [`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go), [`complete_turn_source_acceptance.go`](go-service/internal/httpapi/complete_turn_source_acceptance.go), [`complete_turn_source_revision.go`](go-service/internal/httpapi/complete_turn_source_revision.go), [`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go), [`turn_memory_admission.go`](go-service/internal/httpapi/turn_memory_admission.go), and [`turn_extraction_persist.go`](go-service/internal/httpapi/turn_extraction_persist.go).
- For deletion, reroll, or worldline work, inspect `reconcileActiveChatTailDeletionWithBackend()`, `onRisuOutput()`, and the request-context handoff in [`Archive Center.js`](Archive%20Center.js); [`group_turn_rollback.go`](go-service/internal/httpapi/group_turn_rollback.go), [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go), `resolvePrepareTurnHistoryScope()` in [`group_turn_prepare.go`](go-service/internal/httpapi/group_turn_prepare.go), [`prepare_turn_recall.go`](go-service/internal/httpapi/prepare_turn_recall.go), [`complete_turn_source_revision.go`](go-service/internal/httpapi/complete_turn_source_revision.go), [`group_step23_fork_lineage.go`](go-service/internal/httpapi/group_step23_fork_lineage.go), and the `session_fork_lineage.v2` Store contract.
- For identity or normalization work, inspect [`group_characters.go`](go-service/internal/httpapi/group_characters.go), [`group_items.go`](go-service/internal/httpapi/group_items.go), [`turn_entity_identity.go`](go-service/internal/httpapi/turn_entity_identity.go), [`store/entity_identity.go`](go-service/internal/store/entity_identity.go), [`group_admin_session_normalize.go`](go-service/internal/httpapi/group_admin_session_normalize.go), [`group_admin_rescan.go`](go-service/internal/httpapi/group_admin_rescan.go), and [`admin_jobs.go`](go-service/internal/httpapi/admin_jobs.go).
- For storage or schema work, inspect [`go-service/internal/store/store.go`](go-service/internal/store/store.go), every affected Store extension and `mariadb_*.go` implementation, [`mariadb_memory_admission.go`](go-service/internal/store/mariadb_memory_admission.go), [`migrations/001_schema.sql`](migrations/001_schema.sql), every later numbered migration through the current maximum, [`migrations/README.md`](migrations/README.md), and the schema loader in [`cmd/mariadb-schema/main.go`](go-service/cmd/mariadb-schema/main.go).
- For vector work, inspect [`go-service/internal/vector`](go-service/internal/vector), [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go), [`turn_extraction_vector.go`](go-service/internal/httpapi/turn_extraction_vector.go), and every direct caller affected by the change.
- For configuration work, inspect `config.Default()`, `config.Load()`, and `Config.Validate()` in [`go-service/internal/config/config.go`](go-service/internal/config/config.go), plus the plugin defaults and runtime `/config/update` consumer.
- For API changes, inspect [`go-service/internal/dto/prepare_source_contract.go`](go-service/internal/dto/prepare_source_contract.go), [`go-service/internal/dto/types_gen.go`](go-service/internal/dto/types_gen.go), the registered route, the Go producer/validator, the JavaScript consumer, and production-path contract tests. Verify each contract version independently.
- For package or installer work, inspect [`ops/build-full-package.ps1`](ops/build-full-package.ps1), [`ops/build-posix-managed-packages.ps1`](ops/build-posix-managed-packages.ps1), the relevant launcher template, [`install-windows.ps1`](install-windows.ps1), and [`install.sh`](install.sh).
- The historical `4.3.0-test.7` checkpoint changed Go and the plugin for expanded specialist prompts, the Endpoint label and persistent password editors. The current documented package is [test.22](docs/archive-center-4.3-test-build-22.md); consult its record instead of reusing test.7 build arguments. When packaging is requested, plugin version, build channel and packaged backend version must describe the same artifact. Version reported by the backend is launcher configuration, not proof of a changed executable. The user owns backend startup and RisuAI installation. The normal Windows launcher uses the stable user data directory; a newly extracted package is not an isolated database.
- The user explicitly requested that saved preprocessing API keys remain populated like the Publisher/Critic password fields and that the separate deletion checkbox be removed. The config ViewModel now returns keys for editing with `Cache-Control: no-store`; do not treat this credential-bearing response as a shareable diagnostic. Explicit `api_key`, including an empty string, is the edited value; omission preserves the stored key. Test save/reopen/edit/clear and missing-field preservation. Keep keys out of AI prompt/input assembly and retain existing error scrubbing.
- For preprocessing Flex settings, retain the existing provider transport owner: OpenAI-compatible `service_tier`, AI Studio `serviceTier`, and Vertex Flex headers. Use each role's persisted processing options for independent connections and the Publisher's options when shared. Preserve hidden values across provider switches without sending an inapplicable independent service-tier option. Verify both analysis rounds, optional sharing, configured temperature/output limit, and unchanged no-recommendation Go selection. Provider-level UI visibility is not proof that every model/account supports Flex; do not invent a model whitelist or silently retry with a different tier.
- For preprocessing prompt changes, retain Go-owned assembly and persistence. `settings.shared_prompt` is the saved override; empty uses the bundled default, omitted PUT preserves the existing value, and the ViewModel exposes effective/default text separately. Verify the selected common and role prompts reach both rounds from one request settings snapshot. Independent role connections are the new default; preserve explicitly saved Publisher sharing and verify it only shares the connection. Keep the existing no-recommendation Go selection behavior.
- For preprocessing UI changes, exercise the actual `renderSettingsPanel()` and `loadMemoryPreprocessingPanel()` with `ops/preprocessing-ui-smoke.cjs`, including desktop/mobile dimensions, custom connections, prompt restore, independent role values and hidden-field preservation. Scope CSS to the preprocessing panel; do not reintroduce unstyled classes or let shared form flex rules overlap columns. The local browser fixture is not loaded-RisuAI or real config persistence evidence. Model/API connection sharing must not replace a role's prompt with the Publisher prompt.
- Settings rendering mounts editable controls before awaiting backend status. Update only the dashboard container for the existing render request, preserving unsaved inputs and its delegated queue action. All bridge requests follow the saved UI `bridgeUrl`; do not substitute localhost or the Risu page domain. Server listener binding is configured separately by the launcher and must remain reachable at the user's selected address. See `docs/archive-center-4.3-test-build-3.md` for the reproduced localhost-only restart regression and its authorized recovery.
- Fresh-install release helpers must download the published `SHA256SUMS*.txt`, match the exact selected ZIP filename, and verify SHA-256 before extraction or pointer creation. Test both accepted and tampered archives on Windows and POSIX.

- For the **implemented 4.3 UI**, preserve the independent entry under Extensions beside Persona Capsule, Original Work DB and Lorebook Reference. `loadMemoryPreprocessingPanel()` uses the existing Go `/config/memory-preprocessing` settings owner and explicit default-OFF setting. Panel access, model configuration and other extensions must not auto-enable it. OFF preserves existing memory/Publisher behavior with zero feature-specific analysis/search calls; other extensions keep their own settings and data-use scope. Local UI/source tests remain distinct from loaded-RisuAI verification.

### 4.3 Vertex processing-option correction — 2026-09-07

- Keep the provider request owner in `proxyApplyLLMGatewayServiceTier()` and
  `proxyApplyRequestOverrides()`. A valid service-tier value retained in settings
  must not block a Vertex request or be serialized as `service_tier`/`serviceTier`.
  Record `vertex_uses_vertex_flex_mode` in the existing skip-reason trace field;
  keep the saved value and apply Vertex's own Flex headers normally.
- This correction covers the reported Vertex configuration. Preserve other
  providers' validation, explicit extra-body overrides and upstream errors. Do
  not convert Priority/Flex into a different Vertex mode or add retries.
- Exercise the registered Critic connection-test route and config update through
  the actual Critic call owner, including retained standard/flex/priority values,
  ordinary Vertex and both existing Flex modes, unchanged generation settings and
  settings readback. HTTP/OAuth fixtures do not establish real Google access or
  loaded RisuAI success. See `docs/archive-center-4.3-test-build-9.md`.

### 4.3 HUD timing checkpoint — 2026-09-07

- Optional preprocessing timing is emitted by the real Go call owner into the
  existing request HUD ledger. Preserve the original 12 stages, recommendation
  decisions, parallel call behavior and workflow finalization. OFF omits the
  field; disabled roles do not acquire timing rows. Show call errors with their
  time, never turn an error into an apparently successful timing row.
- Keep prompts, keys, memory text and request bodies out of timing ViewModels.
  Copy each role's nested call array when cloning snapshots.
- JavaScript may observe local HUD-start, beforeRequest-return and accepted
  afterRequest timestamps for display. Keep that observation independent of
  source acceptance, saving and recovery. The frozen total must exclude subsequent
  Critic/save and next-input idle time, preserve the initial start over a provider
  retry, and clear with a new request. Keep the existing HUD timer/stream owners.
- In next-input mode, retain the finished current-generation card for timing
  inspection without marking the pending backend workflow completed or altering
  the previous-turn Critic/save path. Check both finalization modes, hidden OFF
  display, running and failed rounds, snapshot updates, dismissal/new request and
  simultaneous clocks. See `docs/archive-center-4.3-test-build-8.md`.

### Planned 4.3 prompt work

Follow the [role-prompt plan](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#multi-agent-memory-prompts-43)
when starting 4.3-A. Define five usable role defaults, shared and stage-specific
instructions, scoped inputs, response examples and expected recommendations/search
requests before wiring the analysis calls. Keep model assignment distinct from
role instructions. Extend the existing Go prompt/config/provider owners and
JavaScript editor; Go owns prompt assembly and recommendation use.

Design bundled defaults and user edits separately, with explicit default restore
and inspection of the prompt actually applied. Prompt editing or saving must not
enable the feature or call a model. The existing prompt API writes files and
`/config/update` is runtime-only; verify the chosen storage and application scope
instead of assuming durable settings. B/D implement editing and storage, E covers
restart and role isolation, and F checks actual package-update preservation and
provider application. Keep these planned requirements distinct from current
capabilities, and do not turn prompt/result imperfections into new output or
persistence rejection conditions. This plan adds no runtime implementation.

### Planned 4.3 result fidelity — user instruction, 2026-09-06

The [result-fidelity contract](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#multi-agent-result-fidelity-43)
prohibits discarding received AI recommendations and replacing them with Go's
own selection/prose. The user explicitly retains ordinary Go selection when an
area has no AI recommendation, including an empty recommendation or a failed
call with no received recommendation. Preserve received text, references/order
and search requests; distinguish the reason for using Go selection. Do not erase
received recommendations to manufacture a no-recommendation state. Partial role
failure must not replace other roles' recommendations or disable the feature.

Provide existing scope and count/size budgets in the input/output contract before
dispatch. Do not silently truncate a received selection, replace its meaning or
add rejection conditions after it returns. Preserve raw/partial results and expose
unresolved references, parsing failures, over-budget results and provider failures
as concrete unapplied/incomplete states. Keep received recommendations and use
ordinary Go selection for an area with none, without synthetic AI prose or false
success. After a failed supplemental call, retain and use its first-round
recommendation if present; otherwise use Go selection. A completed supplemental
response with an empty final recommendation also uses Go selection. Keep the
supplemental failure and the actual selection source visible.
Do not add new output/persistence/reroll rejection gates or a new result ledger.

4.3-A must specify the bounded delivery and failure-observation contract; C/D wire
the result to canonical source hydration, payload and Publisher without semantic
replacement; E/F verify received-result preservation and ordinary Go injection
when one or all areas have no recommendation, including provider failure.
Raw-result logging alone does not prove actual delivery fidelity. These are
planned requirements; no multi-agent runtime implementation is implied.

### 4.3 active preprocessing checkpoint — 2026-09-06

The preceding planned requirements now have an `implemented_unverified` runtime
slice; see [the implementation record](docs/archive-center-4.3-preprocessing-work-log.md).
`prepare_turn_multi_agent.go` owns five actual role defaults, durable configuration,
request-local parallel analyses and one supplemental round. `handlePrepareTurn()`
uses the existing scoped retrieval/hydration; `prepare_turn_priority_memory.go`
applies canonical AI selections and retains the request's ordinary Go selections
for roles without recommendations. The existing Publisher/payload consume that
final plan. Preserve order and report declared-budget overrun; do not silently
reapply score/K/size cuts to received recommendations. Existing OFF selection
and lifecycle owners are unchanged. No new result ledger or canonical AI writer.

`GET/PUT /config/memory-preprocessing` persists under `ARCHIVE_CENTER_DATA_DIR`,
separate from bundled Go defaults. The POSIX launcher exports its actual data root.
`loadMemoryPreprocessingPanel()` is Host UI/transport only and adds no JS selection.
Inspect production regressions in `prepare_turn_multi_agent_test.go`; actual
loaded-Host/provider/display and native OS evidence remain separate open checks.
The earlier planned wording describes the design baseline, not absence of this slice.

## 2. Active Sources of Truth

### 4.3 preceding GitHub feedback fixes (source checkpoint)

Use [the feedback work log](docs/archive-center-4.3-feedback-work-log.md) to locate
the #5/#6/#4/#10/#7 changes. These are base behavior repairs, independent of the
planned optional multi-agent feature. Preserve these owner boundaries:

- HUD click cleanup uses the same SafeElement that registered the listener.
  Retain its target/type/id/options until removal; no additional watcher is needed.
- Recovery errors carry the current Go HUD when available. The adapter may fetch
  the existing status endpoint once for the user action; it does not infer a save
  outcome from transport failure. Confirmed-turn correction and creation order
  are HUD bookkeeping, not a new persistence or recovery identity.
- The three free-text precise-memory fields are LONGTEXT in fresh SQL, migration
  `013`, and Go schema compatibility. Preserve full values and existing pending
  Critic JSON/hash replay; do not restore `staged` or add text rejection gates.
- Historical same-source/document delete coalescing advances only after a batch
  commit, releases the writer between batches, and uses existing indexes to avoid
  repeated scans. Preserve the existing lease/source/order predicates. It remains
  called by the existing admin maintenance path, not a new startup purge.

Actual loaded-PocketRisu and packaged-update evidence remains separate from the
production-function regressions and disposable MariaDB/Chroma integration results.

### 4.3 Gemini 3.8 Flash medium checkpoint

`resolveGeminiThinkingLevelOptions()` projects `none/low/medium/high`, and Go
`proxyGeminiThinkingLevel()` passes selected medium through native Gemini/Vertex
and existing gateway adapters. Keep the model-specific addition aligned in these
existing owners. `none`, preset defaults and other models retain their existing
behavior. Evidence is production-function/UI and outbound-body regression, not
a real provider call; see the [work log](docs/archive-center-4.3-feedback-work-log.md).

### 4.3 shared RisuAI / PocketRisu branch identity checkpoint

`branchedfrom` identifies a message in the parent chat. Preserve that marker and
its navigation meaning. Do not require the copied child row to retain the same
ID: inspected RisuAI preserves IDs and PocketRisu reissues them. The existing Go
`resolveRisuWorldlineObservation()` owns both shapes through optional
`risu_message_origins.v1` metadata in `risu_worldline_observation.v2`.

- Keep the initial output/pre-backfill observation bounded to its existing 2–3
  marker/source/user rows. Go's `origin_read_request` names the parent and source;
  the adapter uses official `getCharacterFromIndex` to observe only named chats
  in that captured character. It transmits IDs, roles, positions and branch
  markers; it does not transmit conversation bodies or decide lineage.
- `worldline_message_origins.go` preserves matching IDs and associates reissued
  IDs using the official inclusive ordered-prefix clone shape. A shifted prefix
  supplies preserved-ID links and the marker endpoint only. This is optional
  provenance, not a new completed-turn or output rejection gate. Do not convert
  it into broad content search or a promise to reconstruct arbitrarily edited
  historical prefixes.
- Store the first origin envelope in existing `session_fork_lineage`'s
  `inherited_items_json`; transactional enrichment preserves confirmed parent,
  fork point and source coordinates. Existing nonempty provenance remains intact.
  Ancestors predating origin recording use the same Go resolver and named reads,
  within the existing 32-level lineage scope. No new schema or background worker.
- Nested source lookup follows only confirmed parent records and their maps;
  recall composition keeps the earliest cut encountered along the chain. A user
  endpoint can identify turn T while inherited completed history ends at T−1.
- Missing Host snapshots or failed optional metadata transport keep the previous
  routing response. Do not turn these observations into mandatory output/save
  conditions. Existing confirmed lineage remains usable after its marker vanishes.
- An ordinary copy creates no new official branch marker. Keep its existing
  routing/backfill and explicit DB-copy behavior; editable independent copies of
  all inherited memories and inherited entity/subjective-memory UI are separate
  work, not part of this checkpoint.

Source tests and disposable MariaDB HTTP/persistence tests are recorded in the
[4.3 work log](docs/archive-center-4.3-feedback-work-log.md). Patched loaded-RisuAI
and loaded-PocketRisu validation remains open (`implemented_unverified`).

| Concern | Treat as authoritative | Do not treat as authoritative |
| --- | --- | --- |
| Active plugin | Root [`Archive Center.js`](Archive%20Center.js) | `Archive Center 3.4-C.js`, timestamped backups, and packaged copies. Treat `AC Recomposer Agent.js` only as an optional, separately installed consumer of the active transient bridge; it is not the base plugin source. |
| Backend | Root [`go-service`](go-service), entered through `cmd/archive-center-go/main.go` | Compiled binaries, old worktrees, smoke/audit command code as normal runtime |
| Generated/package output | Build inputs and [`ops`](ops) scripts | `_dist*`, `_release*`, `_test-builds`, `_runtime*`, manifests, copied plugin/SQL/prompts, binaries, caches |
| Canonical database | MariaDB through the selected canonical Store only when `AC_STORE_MODE=mariadb_authority` | Noop, fixture, read-shadow, shadow-write modes; Chroma; plugin storage; Host lorebook snapshot/current tables; in-process ledgers |
| Host lorebook reference | Official Host observation in `Archive Center.js`; separate `LorebookReferenceStore` and MariaDB snapshot/current projection | Canonical memory, original-work reference library, native Host activation, or a Chroma collection |
| Vector/search index | Chroma adapters under [`internal/vector`](go-service/internal/vector) | Chroma documents as canonical memory or accepted facts |
| Configuration | `config.Default()` plus `config.Load()`/`Config.Validate()`, with adapter settings supplied through `/config/update` | A generated package example, stale `.env.example`, or the in-memory update response as durable configuration |
| API contracts | Active Go producers/validators, JavaScript consumers, and contract tests. Current turn contracts include `memory_recall_plan.v1`, priority-disabled legacy `memory_delivery_plan.v1`, priority-enabled auto/custom-budget `memory_delivery_plan.v2`, `payload_application_plan.v1`, opt-in `memory_transport_plan.v1`, transient `memory_transport_payload.v1`, observation-only `memory_injection_baseline.v1`, and `publisher_plan.v2`. | A shared-version assumption, roadmap prose, comments, or a duplicate JavaScript DTO/policy implementation |
| Fresh/upgrade schema | The complete sorted source migration set through `013_precise_memory_text_fields.sql`, interpreted with [`migrations/README.md`](migrations/README.md) and the schema loader | `001_schema.sql` alone; an installed dump; generated package SQL; rewriting or duplicating later migrations into historical migration files |

Edit source inputs, not their outputs. Never edit copied plugin files, binaries, manifests, checksums, copied SQL, or copied prompts directly. Regenerate them through the build scripts.

## 3. Component Ownership Rules

| Responsibility | Required owner | Operational rule |
| --- | --- | --- |
| RisuAI observation | JavaScript adapter | Observe official host hooks, chat/session/message coordinates, streaming/display state, the actual payload, and exact official lorebook scope/entries only in [`Archive Center.js`](Archive%20Center.js). Do not synthesize an unobserved scope. |
| Transport and payload mutation | JavaScript adapter | Send minimal versioned observations, apply the exact backend plan to the real RisuAI payload, render UI, and maintain only unavoidable host-local retry/transient state. |
| Policy and orchestration | Go backend | Put request ownership, source acceptance, lifecycle meaning, provider orchestration, and stable error/retry decisions in [`internal/httpapi`](go-service/internal/httpapi). |
| Retrieval and filtering | Go backend | Read through Store/vector interfaces; apply session, source-revision, visibility, private-perspective, tombstone, supersession, and future-turn filters in Go. |
| Duplicate suppression and ranking | Go backend | Preserve provenance-aware dedupe, exact/lexical eligibility, protected exceptions, relevance ordering, and delivery lineage. With a query, do not restore noneligible nonprotected rows through deep/recent refill. Score admitted facts with relevance, stored importance independently of RP-turn-distance recency, and the separate recency term, then rank them within their delivery group. Never use wall-clock age for story/RP recency. Keep lifecycle metadata out of scoring and canonical winner selection: plan, progress, completion, and follow-up facts remain independent score candidates, while Critic transitions remain diagnostic lineage only. Treat structured array ordinals such as `item_1` as projection-local order, not a canonical field identity shared by unrelated source rows; retain source occurrence identity for those facts. Do not re-rank or re-dedupe in JavaScript. |
| Budgeting | Go backend | Let JavaScript supply host/runtime observations only. When Priority Memory is enabled, let Go form request-local `PriorityFactSeed` units from already admitted source projections before final section rendering, score each recalled `memories.turn_summary` by the higher of its best child fact and observed aggregate-vector score, and produce `memory_delivery_plan.v2` for both auto and custom modes. Apply the same UI maximum independently to complete turn summaries and to each scored fact lane; do not transfer unused item slots. Direct evidence and protected-secret guidance remain item-count exempt. In custom mode, apply the existing per-class UI character budgets; complete turn summaries and event facts share `event_recent`. Every selected item remains subject to its class budget and the final character envelope, and is included whole or deferred. Never use score or a missing score to reject the current output, display, save, reroll, or recovery. |
| Ordering and context assembly | Go backend | Preserve base order `original_work` → `long_term_memory` → `output_guidance`. In `reference_assist`, insert `lorebook_reference` immediately before `output_guidance`. Keep recent input internal to Publisher/turn analysis and emit an empty payload `input_context_text`. |
| Opt-in PDF memory representation | Go produces; JavaScript applies the selected transport representation | Use only the completed `payload_application_plan.v1` `long_term_memory` lane. Preserve its exact text/order/hash and every other lane. Direct Google/Gateway modes keep PDF/base64 current-request-only and apply only the matching `inlineData`/`file` shape. The explicit `provider_manager_pdf` experiment instead marks only that lane with one `<pm-pdf>` pair and delegates PDF creation to Provider Manager; it must not build a second Go PDF or modify the external plugin. Do not infer provider/model/plugin state, rerun selection, add a retry, persist the document, or make PDF success a condition for output/save/reroll. |
| 4.1 baseline and lineage | Go produces; JavaScript observes Host application only | Keep `candidate`, `selected`, `rendered`, `payload-applied`, and `displayed effect` distinct for all nine surfaces. Treat normalized-text matches as duplicate candidates only. Do not turn telemetry into JavaScript selection, automatic suppression, canonical mutation, vector mutation, or a displayed-final claim. |
| Host insertion | JavaScript adapter | Apply only the plan's exact `auxiliary_text` to the actual message array. Keep `input_context_text` empty in the payload; it is internal Go analysis context because RisuAI already carries recent chat. Treat the JavaScript host-index calculation as migration debt, not permission to add policy there. |
| Canonical persistence | Go Store/MariaDB layer | Route normal accepted turns through `/complete-turn` and common admission. Keep runtime SQL implementations under `internal/store/mariadb_*.go`. |
| 4.2 selectable finalization timing | Go decides; JavaScript observes the Host pair and transports | Keep `응답 직후` as the default 4.1 path. For explicit `다음 사용자 입력 시`, use Go's `turn_finalization_policy.v1`, carry that policy through the request-owned orchestration result in both compact and legacy return shapes, retain only minimal pending Host coordinates, and route the edited/rerolled previous pair through the same `/complete-turn` owner once when a genuinely new user row arrives. Do not await the previous Critic before the current request. Record that Go-confirmed timing on the workflow entry created for the delayed turn; the following request must preserve that still-running previous ledger instead of marking it `superseded_by_new_request`, even if the UI setting changed in between. An ordinary immediate-mode previous entry keeps the existing same-session supersession behavior. Render the current request's original stages 1–6 as `현재 턴 생성 1/6~6/6` and the previous request's original stages 7–12 as `직전 턴 평론가·저장 1/6~6/6`; this is a presentation projection of the same backend stages, not another workflow. Once `afterRequest` has supplied the accepted delayed response, close only the finished current-generation card; keep the pending marker so that the same request appears as the previous Critic/save card at the next input. A terminal previous card dismisses independently without clearing a still-running current card. The two request-scoped HUD cards remain UI observation only and must not create another persistence or Critic owner. If the exact previous row is temporarily unavailable, leave that marker pending and continue the current request; do not turn identity checking into an output gate. Do not add a second persistence path, Critic, scheduler, broad chat sweep, hidden retry, or shutdown/session-switch auto-save. |
| Host lorebook persistence | JavaScript observes/transports; Go route and separate MariaDB Store decide persistence/search | Keep `lorebook_reference_snapshot.v1` and its current projection non-canonical and exact-scope. Do not give the adapter admission, ranking, or truth authority. |
| Indexing | Go vector/outbox/direct route owners | Keep Chroma derived. Use the existing outbox or the existing route-specific direct mutation path; do not invent a second index lifecycle. |
| Rollback and deletion | Go decides; JavaScript observes/transports | Require verified host-observed assistant deletion, captured route ownership, complete observations, one-use token, observation digest, and route-revision fence. Never decide deletion from Timeline selection, `beforeRequest`, message-count drift, Critic failure, or a temporary missing tail alone. |
| Worldline | JavaScript observes bounded Host branch facts; Go resolves, scopes retrieval, and persists | Route `risu_worldline_observation.v2` through `/session-routing/turn-resolution`. For confirmed lineage, include ancestor history only through each fork boundary and child history only from its owned start; search each allowed history session but reapply turn/source fences after hydration. Do not let the output listener accept finality, complete a turn, choose lineage, expose sibling/parent-post-fork data, or create narrative truth. |
| Character/item identity | Go preview/read/write owners | Require explicit reviewed equivalence selection. Do not infer identity from a label, rewrite existing source rows, call Critic, or auto-reindex as a side effect of manual merge/unmerge. Preserve per-source partial-result reporting. |
| Session normalization | Go admin job owner | Preserve staged inspect → raw repair replay → rescan → missing unambiguous identity repair → eligible reindex. Treat `deferred` as terminal but incomplete; do not report it as completed. |
| Publisher policy | Go backend | Require the `publisher_plan.v2` schema with both role shapes; make one bounded provider request; validate source references and fields; render only accepted items from one or both roles. Never give Publisher truth, persistence, or forced-event authority. |
| Fallback behavior | Go for stable decisions; JavaScript for transport effects | Preserve backend reason codes. A source/full `/prepare-turn` transport failure preserves the original payload; it does not enter legacy reads. Use queues/retries only on paths that actually implement them. Do not expand `applyProtectionOnlyInjection()`; it is an active boundary exception. |
| Windows package lifetime | PowerShell launcher | Isolate managed backend, MariaDB, and ChromaDB children from inherited `CTRL_C_EVENT`; let the launcher prompt before cleanup. `N` at this first Archive Center prompt must preserve the same child processes and `Y` alone may enter bounded cleanup. A later generic `cmd.exe` batch prompt owns only the BAT window. Keep Job Object kill-on-close for confirmed shutdown, parent loss, and abnormal launcher exit. Do not restart children to simulate cancellation or remove the Job Object safety boundary. |

Do not add JavaScript business logic because it is faster. If Go can decide from supplied observations, implement the decision in Go and remove the replaced JavaScript calculation in the same bounded change.

## 4. Memory Safety Rules

1. Preserve accepted raw evidence before deriving structured state. Keep the user raw row, assistant raw row, and accepted source revision distinct; do not claim that their writes are one transaction. See `persistCompleteTurnRaw()` and `registerCompleteTurnSourceRevision()` in [`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go).
2. Preserve current user input and explicit user corrections above retrieved support or guidance. Keep the priority encoded by `buildResponseExecutionContractWithMemoryLineage()` in [`prepare_turn_planner.go`](go-service/internal/httpapi/prepare_turn_planner.go): current input, explicit correction, direct evidence, canonical state, retrieved support, then execution guidance.
3. Treat Publisher and Critic output as proposals. Require the current contract version, validation, source binding, and admission before using or storing derived state. `response_execution_contract.v1` has `truth_authority: false`; `publisher_plan.v2` is response guidance, not a truth writer.
4. Do not turn a model response into canonical state merely because RisuAI displayed it. Require official `afterRequest` acceptance, `/complete-turn` source/session/turn validation, durable raw writes, source revision registration, and the appropriate admission writer.
5. Do not turn a retrieval hit into canonical memory. Hydrate Chroma candidates through the selected Store and reject missing, wrong-session, stale, inactive, superseded, future, or visibility-ineligible rows in [`prepare_turn_recall.go`](go-service/internal/httpapi/prepare_turn_recall.go). Keep lexical evaluation active even after vector success.
6. Do not turn a Host lorebook snapshot, search hit, or delivered `reference_only` item into canonical memory, relationship state, character state, or original-work reference data. Keep its provenance, exact scope, observation status, and separate Store boundary.
7. Preserve provenance. Carry session, turn, source contract/revision, logical turn, message/generation identity, content/evidence hashes, excerpts, visibility/authority, and occurrence identity through the existing DTO and Store contracts. Do not synthesize missing host facts.
8. Preserve relationship and character-state admission guards. Require evidence binding for character deltas, keep relationship shifts out of generic character events, normalize relationship state through existing helpers, and call `canonicalStatePromotionAllowed()` before promotion. See [`turn_extraction_character_state.go`](go-service/internal/httpapi/turn_extraction_character_state.go), [`interaction_admission.go`](go-service/internal/httpapi/interaction_admission.go), and [`turn_extraction_helpers.go`](go-service/internal/httpapi/turn_extraction_helpers.go).
9. Do not use continuity correction, Publisher guidance, Host references, or retrieved support to override user intent, write truth, reveal hidden knowledge, force a relationship/character-state change, or force a new narrative event. Preserve the blocked usages in [`prepare_turn_planner.go`](go-service/internal/httpapi/prepare_turn_planner.go), the limits in `publisherStrengthProfile()`, and the “do not expand them into new events” rule in [`narrative_state_contract.go`](go-service/internal/httpapi/narrative_state_contract.go).
10. Preserve direct evidence and supersession. Do not detach a memory from its source revision or allow superseded/inactive revisions to re-enter current recall.
11. Preserve inspectable suppression. Keep `memory_delivery_lineage`, selected/deferred/suppressed states, counts, reason codes, source references, and payload observations whenever dedupe or budgeting removes material. Keep occurrence-aware identity so identical text does not erase distinct proven occurrences. `memory_injection_baseline.v1` duplicate observations are candidate telemetry only; never use them to suppress or mutate canonical/vector state without a separately versioned policy change.
12. Preserve request ownership. Carry the exact `beforeRequest` session, host coordinates, user-message identity/hash/content, request ID, and request-owned orchestration result into `afterRequest`. If the handoff is missing, do not reconstruct persistence authority from `_latestOrchResultForUI`, the current session, or another request's globals. The inspected official top-level main-chat path is serialized, but its provider retry loop can call the replacer again and the API exposes neither a request ID nor a retry cause. Reuse an active context only when every captured Host-turn coordinate matches and its Go payload plan is already replay-safe; never use model name, prompt text, elapsed time, or an error guess. If identity is incomplete/different, or the prior context is not ready, fail closed instead of inventing correlation, queueing, watching, or assigning a completion by order.
13. Preserve full candidate evidence until the final Go budget owner. Do not reintroduce arbitrary 80–720 character caps in prepare-turn candidate renderers. Let `buildPrepareTurnMemoryDeliveryPlan()` include a complete item or defer it with lineage; UI/diagnostic/search-query truncation remains a separate concern.
14. Keep reviewed identity links distinct from facts. A manual `canonical_equivalence` link changes canonicalized read projection; it does not rewrite the underlying character/item/KG/memory rows, prove a relationship/state fact, invoke Critic, or authorize automatic vector changes.
15. Keep contextual embedding retry evidence only while needed. After a successful non-empty contextualized embedding, clear bulky context chunks before persisting/outboxing the vector operation. Preserve them when embedding fails or returns empty so the worker can retry from the same evidence.
16. Preserve worldline isolation. Use only confirmed `session_fork_lineage.v2` to compose ancestor-through-fork and descendant-owned history segments. On unresolved/conflicting/cyclic lineage, fail closed to the bounded current/partial scope; never infer a parent, include parent post-fork data, or leak sibling history.
17. Preserve missing-role truth. Reuse an existing canonical user row for `stored_pair_recovered`; keep `assistant_only` explicit when no user row exists. Never fabricate a user message to make an old assistant output look paired.
18. Preserve empty/synthetic-origin truth. The inspected RisuAI Say Nothing branch can create a non-empty stored user row without invoking the input handler. Do not classify `*says nothing*`, a translation, prompt prose, missing `time`, or a plugin name as official auto-continue origin. If Host origin is not exposed, keep it `unobserved`. A missing middle user must not cause an assistant to be reassigned to the previous user or later turns to be silently renumbered; surface a no-write review/normalization state instead.
19. Preserve the existing Priority Score contract with the authorized 4.3 static.v4 changes. Keep relevance, stored importance, RP-turn-distance recency, continuity bonus, small structured score biases, canonical/source identity, visibility, perspective owner/viewers, final score, rank, and selection reason attached to each complete fact candidate until the final Go budget owner. Resolve one request query set in Go and use it for both Chroma retrieval and fact scoring. The current query is the explicit Host continuity query when present, otherwise raw user input or the latest non-assistant request message. Add the most recent completed Host conversations up to UI `recent_conversation_reference_count`; keep each conversation's user input and final assistant output together as one independent semantic query, merge duplicate document hits by strongest similarity, and calculate lexical fact relevance per query before retaining the strongest score. This history-reference depth is independent of Chroma result `top_k` and the final per-group core-memory item ceiling. These conversation originals restore retrieval context but must not restore a duplicate Recent Raw Turn block in the final model payload. Never classify continuation by a runtime phrase list or append names/places to saturate the primary query. Use an active, in-scope `precise_memory_unit` vector similarity only for the matching atomic fact, after canonical MariaDB hydration; never copy a parent row's retrieval/selection score or one precise hit to every child fact. Keep the dedicated aggregate `tier=memory` vector search so precise/evidence hits cannot crowd aggregate Memory recall. Aggregate Memory facts and full `memories.turn_summary` candidates may enter the priority pool only from that existing recall result; do not append every loaded session Memory row as a parallel candidate source. For atomic scoring, query the existing `source_table=precise_memory_units` vectors with the same already-created request query vectors and the exact in-scope canonical fact count, independently of final delivery K; this is a scoring pass within the same recall owner, not a second selector or persistence path. Consume the private hit handoff inside prepare-turn and do not expose it in the public response. Speaker, location, and storyline matches are small independent rank signals only, never admission or rejection conditions. Calculate recency from current RP-turn distance with a nonzero floor rather than wall-clock age. Keep reviewed names, aliases, and identity evidence as attributable metadata outside the event/current-state K; metadata may attach once to a selected same-entity fact but failure to attach must not block the fact. Score each recalled complete turn summary by the higher of its best child fact and its observed aggregate-vector score; keep it intact when selected. Apply the same UI K independently to the complete-turn-summary group and to each scored fact lane, without transferring unused slots; direct evidence and protected-secret guidance remain K-exempt. A one-sentence summary and its exactly identical fact are delivered once without consuming a fact slot. Apply auto/custom character budgets in this same Go selector: custom uses the existing per-class UI values, summaries and event facts share `event_recent`, and the final global envelope still applies. K is a maximum, not a request to fill the character budget. Do not reject a fact merely because lexical or semantic relevance is zero or unavailable; score components order candidates but do not form an additional eligibility gate. Do not recreate a JavaScript selector, lane-order fill, character blacklist, or second visibility/perspective gate. Request-scoped current resolution is a fact-level read/delivery projection and must not rewrite source rows. Preserve current input, explicit correction, direct-evidence authority, privacy, branch/revision, and secret guards outside this optional-memory score competition. A protected recollection's usage guard remains attached to its memory as one delivery unit rather than consuming separate K slots. If a source has not moved to `PriorityFactSeed`, keep its complete rendered-line/sentence compatibility path available and diagnose it as `legacy_rendered_line`; do not drop it for lacking a seed or score. Do not use score or missing score to reject normal output, display, persistence, reroll, or recovery. See [`docs/archive-center-4.2-priority-score-memory-plan.md`](docs/archive-center-4.2-priority-score-memory-plan.md).

For the active `priority_score.static.v4`, preserve raw stored importance and its contribution to ranking. The existing `importance_after_turn_decay` trace field equals importance; use importance directly in the `0.25` term and RP-turn recency independently in the `0.15` term. Never substitute wall-clock age. Treat every lifecycle key/transition as diagnostic-only delivery lineage with zero score influence.

## 5. Hook and Turn-Order Rules

Preserve this verified source registration order in `registerRisuLifecycleHooks()`:

1. Register `addRisuScriptHandler("input", onInputHook)` for observation and correlation.
2. Register `addRisuReplacer("beforeRequest", onBeforeRequest)` for the writable outbound payload stage.
3. Register `addRisuReplacer("afterRequest", onAfterRequest)` for displayed-output normalization, official `risu_afterRequest` finality acceptance, and non-blocking persistence scheduling.
4. Register `addRisuChatListener("output", onRisuOutput)` for bounded worldline observation only.
5. Register unload cleanup with `onUnload(removeRegisteredRisuHooksOnUnload)`.

Registration order is not callback coverage. In inspected official RisuAI commit `72ce721878d65b09baf4339638dfd221d1788261`, the blank-input Say Nothing branch stores its synthetic user row without calling the normal `editinput`/plugin input handler. Keep actual-empty metadata limited to an observed empty callback or a future official versioned Host signal; do not reconstruct it from literal content.

Preserve this backend turn sequence:

1. Observe input; do not inject at the input hook.
2. At `beforeRequest`, freeze one final-confirmation context containing the request/session/user-message/host coordinates, raw input, request ID, and a request-owned orchestration-result slot. Do not let later globals become persistence authority.
3. If the inspected RisuAI provider retry loop re-enters `beforeRequest` for the exact same Host turn, reuse only the ready context's request ID, raw input, Host coordinates, `/prepare-turn` result, payload plan, and final-save owner. Increment diagnostic attempt count and observation time only. Do not prime a new HUD, bind another raw input, call `/prepare-turn`, rerun Publisher/search, or reset stage 6/12. A confirmed new `input` callback terminalizes any unconsumed stale context before the new request begins.
4. From the first `beforeRequest`, attempt exact-scope Host lorebook synchronization before each actual `/prepare-turn` call when the mode is not `off`. Reuse the adapter's same-scope attempt state; do not assume a durable retry.
5. Call `/prepare-turn` for the source decision. Preserve the original payload immediately if transport fails or Go does not return an eligible `current_input_decision`.
6. Before Archive-owned Publisher/continuity/language/recent-context reads, derive a non-mutating Yumi Translator 1.4.2 original-source copy when its exact v1 marker and `scriptstate` record are present. Preserve the active chat, displayed translation, raw Host observations, persistence owner, and outgoing payload. If the record is absent or malformed, retain the translated inner text and continue; do not add a strict gate or plugin-order requirement.
7. On eligibility, call full `/prepare-turn`. Perform canonical reads, optional Chroma candidate selection with canonical hydration, exact/lexical filtering, provenance-aware suppression, budgeting, optional exact-scope lorebook search, delivered-support construction, optional `publisher_plan.v2` validation, and Go rendering. Emit `memory_injection_baseline.v1` only after current policy has produced the preparation assembly; do not let the baseline select or suppress content.
8. Apply `payload_application_plan.v1` in JavaScript. Insert only the exact Go-owned `auxiliary_text` block. Do not inject a second input-context message. For retry replay, insert when no exact block exists, reuse one exact block, collapse multiple identical exact blocks to one, and treat any different Archive auxiliary block as ambiguous without overwrite. Observe the whole exact plan application for the baseline without claiming displayed effect. Publish the transient Recomposer bridge only after a compatible Go plan is applied.
9. Let the main model run.
10. In `onAfterRequest()`, detach and clear the captured context before asynchronous work. If absent, do not start persistence. Otherwise normalize/sanitize, accept official finality, schedule persistence on a Promise microtask, and return display text without waiting for `/complete-turn`. Empty/Say Nothing final content terminalizes the detached context without `/complete-turn`; a duplicate callback cannot consume it again.
11. Validate source and idempotency; save user raw, save assistant raw, then register the accepted source revision.
12. Parse Critic output as a proposal; commit the atomic core admission; write post-admission projections separately; then wake reprocessing/vector workers.

Treat `onRisuOutput()` as an independent branch-observation signal, not a normal step between model response and `afterRequest`. It may route a validated `branchedfrom` observation to Go, but it must never accept finality, call `/complete-turn`, or choose lineage.

Do not move payload mutation to `input` or output persistence to `beforeRequest`. Do not bypass lorebook scope contracts when that feature is enabled, the source-decision call, `payload_application_plan.v1`, official finality acceptance, source revision fence, or complete-turn idempotency ledger without changing the versioned contracts and their production-path tests. Do not move the conditional `lorebook_reference` lane after `output_guidance`.

**UNKNOWN:** Source registration does not prove the loaded RisuAI callback lifecycle. Require an actual loaded-plugin trace before claiming current live conformance.

## 6. Persistence Rules

- Route normal turn-derived canonical changes through accepted `POST /complete-turn`, `commitAcceptedMemoryAdmission()`, and `CommitMemoryAdmission()`. Do not use the obsolete guard-only `POST /turns` or `POST /turns/complete` stubs.
- Use only an existing, registered Store-backed writer family documented in [the section 11 write inventory](STRUCTURE.md#11-output-processing-and-commit-flow). Do not add a direct route, table, worker, compatibility writer, or raw SQL path without a reproduced requirement and an explicit owner.
- Keep active HTTP runtime mutations behind Store/vector interfaces. Keep physical runtime SQL under `go-service/internal/store/mariadb_*.go`. Treat `mariadb-schema`, `mariadb-import`, and `legacy10-migrate` as explicit operator exceptions, not service call paths.
- Preserve the source-fenced `READ COMMITTED` transaction in `commitMemoryAdmissionOnce()`. Keep core memory, reconciled direct evidence, precise units, vector outbox rows, and admission state atomic together.
- Do not describe the whole turn as atomic. Raw user, raw assistant, source registration, effective input, feedback/audits, core admission, KG/narrative/character/status projections, and direct vectors have separate failure boundaries.
- Preserve complete-turn idempotency keys, recorded responses, conflict handling, request-status lookup, and retry classification in [`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go). Do not retry an ambiguous write by issuing an uncorrelated duplicate request.
- Build durable logical-turn identity from the stable observed Risu user-message `chatId` within session/Host-chat/branch scope. Editing or rerolling the same Host user row must preserve that logical turn even when its text, timestamp, or other observations change; a genuinely new Host user row must append even when its text is identical. Only when the user-message `chatId` is unavailable, preserve the existing index/time/content fallback. Preserve the MariaDB current-tail check. If a replacement returns `not_committed`, restore the prior active source state; for a nonretryable revision, persist a terminal replacement-failure record so a new idempotency key cannot re-enter canonical replacement, Critic, or vector work.
- Preserve one `lifecycle_key` across Critic `state_claims`, `pending_threads`, and `state_deltas` opened/resolved records. A grounded completion must use an explicit terminal transition and close the matching stored pending row; do not infer lifecycle identity from wording similarity, turn number, or score. Keep the legacy exact-title path for old rows without a key. In memory delivery, do not collapse or discard facts merely because they share that AI-produced key, and do not let a Critic transition change final score or canonical winner selection. Preserve lifecycle as diagnostic lineage only; never infer rank from completion-like words. Do not turn lifecycle metadata into a new output, memory-admission, or persistence rejection gate.
- Preserve `atomic_rollback`, `partial_commit`, and `no_commit` reporting. Do not convert a partial prefix into normal success or erase diagnostics.
- Keep reprocessing and core vector-outbox work durable, leased, source-revision-fenced, and retryable/permanent according to the existing worker contracts. Do not resample an already committed extraction during deterministic replay.
- Keep administrative canonical reindex vector-only after `CommitMemoryAdmission()`. When `WithMemoryAdmissionVectorReplay` is present, do not run post-admission precise projections, narrative/status, KG, active/canonical state, pending-thread, storyline, reversible-state, pruning, or direct-vector writers. Reindex must not change the count or currentness of derived state rows.
- Keep Critic automatic reprocessing scheduling in Go. Use the configured base interval and only let a valid provider `Retry-After` header or JSON `retry_after` extend that interval. If one hint is malformed, ignore only that hint; do not drop the provider error body, token diagnostics, raw turn, derived history, or job, and do not create a parallel fallback queue or hidden provider retry.
- Keep an automatically reprocessing turn nonterminal, but expose its existing HUD `dismissal_policy` as `x_only`. Closing that card hides its status stream only; it must not cancel, delete, or alter the durable Critic reprocessing job.
- Restore the existing MariaDB job's earliest `retry_after` or expired-lease wake into the worker's one-shot timer after startup. Do not replace this durable schedule with JavaScript timers, a server-lifetime polling ticker, or untracked `time.AfterFunc` callbacks.
- Preserve provider termination/token fields independently in the Critic HUD. A missing finish reason or one missing usage field must not erase other valid diagnostics, and a token-exhausted or truncated response must never be synthesized into canonical Critic JSON.
- The 4.0.7 `replacement_pending` experiment was removed after live cold-start testing. Do not restore that state, a parallel reroll guard, or JavaScript success handling for it. Keep the established logical-turn replacement transaction and do not decide sameness from user-input text.
- Start automatic rollback blocked. Require JavaScript to send an actual deletion observation and Go to verify route ownership, complete assistant observations, an exact Host-to-active-source prefix, a contiguous missing tail, the one-use decision token, observation digest, and unchanged route revision. A middle identity/content mismatch or a later matched turn after a mismatch is `historical_revision_conflict` and must not mutate canonical state. Do not call rollback from `beforeRequest`, Timeline selection alone, counter drift, Critic failure, or an unverified snapshot difference.
- Keep rollback HTTP bounded: after MariaDB invalidation and durable outbox registration, return `vector_cleanup=queued` and let the background worker process bounded fair slices. Do not drain an unbounded vector backlog inside rollback, reuse one stale wake timestamp, immediately reclaim a retryable item, or wait forever for source workers to stop. Keep replacement/supersession distinct from deletion.
- Keep Chroma derived. For outbox-managed documents, require embedding, mutation, exact readback, outbox completion, and stale-source compensation in [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go).
- Do not assume that every Chroma mutation uses the outbox. World-rule/status, admin, migration, rollback/delete, explorer, reference, and compatibility paths have direct or route-specific behavior. Preserve their explicit status handling and test reconciliation separately.
- For manual session deletion, require `req_source=timeline_manual_delete`. Keep lorebook, identity, child-owned fork-lineage, source-lifecycle, and other declared session-owned MariaDB cleanup in `DeleteSession()`'s transaction; preserve library-owned reference works and historical provenance as coded. After commit, queue durable vector cleanup when lifecycle/outbox support exists; do not report the derived index as already drained.
- Preserve Repair Replay's role-level conflict behavior. Record and retain a conflicting existing role while inserting the other missing non-conflicting role when allowed. Do not overwrite the conflict or discard the valid repair because its pair conflicted.
- Preserve session normalization stage and terminal semantics. Repair only missing exact/unambiguous identities and surfaces, defer reindex while derived reprocessing is incomplete, and report `deferred` as terminal incomplete state rather than success. Do not assume the in-process admin job survives restart.
- Persist manual character/item identity changes only through the existing reviewed link writer. Preview must write nothing. Do not promise atomicity across multiple selected sources; return and audit each partial result.
- Treat automatic recovery of every post-admission projection and every direct vector failure as **UNKNOWN**. Do not claim completion from a core-admission success alone.
- Treat `POST /sessions/{chat_session_id}/lorebook-reference/snapshots` as the only verified Host lorebook persistence route. Keep it outside canonical `Store`, memory admission, and Chroma. Do not route it through `/complete-turn` or promote its rows to canonical truth.
- Preserve the single MariaDB transaction and per-session lock in `ApplyLorebookReferenceSnapshot()`. Preserve these observed-state rules: complete replaces the exact scope, complete-empty clears it, revoked clears it, and partial/unavailable or older authoritative observations do not replace the newer current projection.
- Do not inherit complete-turn guarantees for lorebook snapshots. The route creates a fresh snapshot ID and has no durable idempotency key, source-revision fence, retry queue, outbox, or vector update. Do not blindly retry an ambiguous POST. Account for the adapter's in-process same-scope attempt suppression.
- Treat Host-reference availability outside direct `mariadb_authority` as **UNKNOWN/unavailable** until the selected Store actually implements `LorebookReferenceStore`; current wrapper/noop/read-shadow modes do not expose it.
- Treat the complete sorted migration inventory, currently through `013`, as the fresh-install and upgrade input. `001_schema.sql` is an entry point into that inventory, not a standalone copy of every later migration; absence of a later table definition from `001` is not an update defect or release blocker.
- Treat Host-reference freshness/TTL and durable snapshot retry/idempotency policy as **UNKNOWN**. Do not repeat the obsolete omission claim: `DeleteSession()` removes lorebook rows and `session-migration.manifest.v4` declares direct/indirect lorebook ownership.

## 7. Generated and Packaged File Rules

- Do not edit `_dist*`, `_release*`, `_test-builds`, `_runtime*`, `.tmp*`, `.gocache`, compiled binaries, package manifests, checksums, copied plugin files, copied migrations, copied prompts, database data, Chroma persistence directories, logs, or caches.
- Do not use `Archive Center 3.4-C.js`, timestamped `Archive Center.js.codex-backup-*`, or `prompts/critic_system.pre-first-compression-20260808.txt` as active implementation/default-prompt source. Treat `AC Recomposer Agent.js` as an optional manual consumer of the transient bridge, not a base-package source file.
- Treat [`archive-center-runtime.test.cjs`](archive-center-runtime.test.cjs) as a manual test-only harness. It is not an active entry point and is not selected by current CI/core-regression scripts; never use its passing result as proof of loaded RisuAI behavior.
- Regenerate a public release with [`ops/build-release-assets.ps1`](ops/build-release-assets.ps1); it invokes [`ops/build-full-package.ps1`](ops/build-full-package.ps1) for the two Windows archives and [`ops/build-posix-managed-packages.ps1`](ops/build-posix-managed-packages.ps1) for the five POSIX archives, then writes one aggregate release checksum list.
- Build packaged `archive-center-go`, `archive-center-updater`, and `mariadb-schema` binaries from their matching `go-service/cmd` sources. Do not treat every command under `go-service/cmd` as a packaged runtime component.
- Regenerate packaged SQL from every numbered source file in [`migrations`](migrations), packaged prompts from the active files in [`prompts`](prompts), and manifests/checksums from the build scripts. Verify that a current build includes migrations through `013`.
- Do not add updater rejection rules based on migration filenames, hashes, SQL shape, or differences from an earlier package. Managed migration files may be added, changed, or removed by a package update; after replacement, fresh and upgraded installations both run the resulting complete sorted inventory. Keep database data, runtime data, local environment files, and secrets outside managed-file replacement.
- Compare generated artifact identity with active source after rebuilding. Treat every package/hash mentioned in a version work log as evidence for that recorded snapshot only, not as current source or loaded-runtime authority.
- Keep the embedded PDF font and its retained license as matching source/package inputs. The active implementation uses the unmodified Google Fonts Noto Sans KR variable TTF under OFL 1.1; do not substitute a host-installed font or omit `licenses/NotoSansKR-OFL-1.1.txt` from the package.
- Keep [`windows-console-control.ps1`](ops/full-package/scripts/windows-console-control.ps1) in every Windows managed package and its manifest. The public BAT remains the single entrypoint, but PowerShell owns Ctrl+C confirmation; managed children must inherit Ctrl+C-ignore before they are assigned to the existing Job Object.
- After a successful PowerShell launcher return, the public BAT must propagate exit code `0` and exit without an unconditional `pause`. Retain a pause only for a nonzero exit so an error remains readable; otherwise a stopped test launcher can survive and later target a refreshed package directory.
- Before refreshing an existing named Windows test package, stop that package's Go backend, MariaDB, and ChromaDB after verifying their executable path, command line, or owned listener ports. This prevents duplicate test stacks and file locks; never broaden the stop operation to unrelated installed packages.
- Verify display/package version, plugin runtime version, backend `/version`, installed manifest, and published release separately. For the 4.2 release, keep all seven platform ZIPs bound to the tagged source and keep the RisuAI plugin update separate from backend package replacement. `build-full-package.ps1` rewrites plugin `//@version` and `const VERSION` only for strict `x.y.z`.
- Stage named source/documentation files. Do not use `git add .` while untracked `_release-builds` or `_test-builds` trees are present.

## 8. Common Failure Modes

| Symptom | Likely cause | Inspect | Prohibited shortcut | Required verification |
| --- | --- | --- | --- | --- |
| A fix disappears after rebuild | A generated/package copy was edited | Root source, [`ops`](ops), package manifests | Patch `_dist*`, `_release*`, copied JS/SQL/prompt, or a binary | Rebuild and compare source hashes/manifests |
| JavaScript and Go disagree | Policy was duplicated or a Go plan was bypassed | `onBeforeRequest()`, `applyGoPayloadApplicationPlan()`, `applyProtectionOnlyInjection()`, Go producer | Add another JS guard, parser, budget, selector, or fallback | Run syntax, contract, output-fidelity, and actual payload checks; report JS line delta |
| Displayed output is not saved or is saved to the wrong session | `afterRequest` lost or reconstructed the request owner from global/latest/current-session state | `captureFinalConfirmationRequestContext()`, `onBeforeRequest()`, `onAfterRequest()`, `continueAcceptedFinalPersistence()` | Fall back to `_latestOrchResultForUI`, `_sessionCache`, or current active chat | Interleave requests/session switches; verify one-shot detach, exact request/session/message ownership, and no write when context is missing |
| One logical main request invokes `beforeRequest` more than once | The inspected RisuAI provider retry/fallback loop re-entered the replacer, which exposes neither a request ID nor retry cause | `finalConfirmationRequestContextRetryIdentityMatches()`, `reapplyFinalConfirmationRetryPayload()`, official RisuAI request source, fail-once smoke | Label it timeout without evidence, generate a new Archive request/HUD, rerun preparation/Publisher/search, or append another Archive block | Verify exact Host identity, ready-plan reuse, attempt count, one `/prepare-turn`, one `/complete-turn`, fallback-clean payload replay, 0/1/multiple/conflicting auxiliary-block rules, and HUD 6→7; loaded-Host proof is separate |
| Two different or incompletely identified main-request contexts overlap | The replacer supplied no correlation ID and exact same-turn identity was not proven | `installFinalConfirmationRequestContext()`, official RisuAI request/callback source, overlap smoke tests | Invent a request ID, bind by completion order, add a queue/watcher, or reuse the latest context | Verify both contexts become terminal, both completion orders make no persistence call, and normal serialized/new-input handoff remains unchanged; loaded-Host proof is separate |
| Normal previous turns are deleted | Deletion was inferred from `beforeRequest`, Timeline selection, message-count drift, or an incomplete temporary observation | `reconcileActiveChatTailDeletionWithBackend()`, rollback decision/execution handlers, source revisions | Add a watcher, counter heuristic, or direct JavaScript delete | Verify actual deletion observation, complete assistant set, missing active source, token/digest/route fence, no mutation for intact/replaced output, and exact tail range |
| Stale or wrong memory is recalled | Chroma was trusted without canonical hydration or a source fence was bypassed | [`prepare_turn_recall.go`](go-service/internal/httpapi/prepare_turn_recall.go), source revision and rollback code | Promote vector text directly or infer lifecycle from score | Test wrong-session, stale, superseded, deleted, private, and future-turn candidates |
| Valid memory vanishes silently | Dedupe/budgeting changed without lineage | `prepare_turn_memory*.go`, `prepare_turn_recall.go`, `prepare_turn_render.go` | Drop by text match in JavaScript or omit reason/source refs | Test protected coverage, dedupe, deferred/suppressed states, hashes, and payload lineage |
| Publisher or continuity summarizes Yumi's translated display instead of the model original | Archive Center's `beforeRequest` ran before Yumi replaced its `yumi-tr:v1` display range from the matching `$__yumi_tr.<id>` source record | `buildYumiV1ArchiveReadContext()`, `getCurrentActiveChatSourceObservationMessages()`, `onBeforeRequest()`, and the exact Yumi Translator 1.4.2 reference | Edit Yumi, require a plugin order, mutate the active chat/outgoing payload, guess generic XML, or reject the request when metadata is absent | Execute the production helper with plain, `u:`, and `z:` records; verify Publisher/continuity/read paths receive the original, user/unmarked messages stay unchanged, missing metadata retains translation text, and inputs/outgoing payload are not mutated; loaded RisuAI proof is separate |
| PDF mode changes memory choice, duplicates Text, or blocks a normal turn | The representation layer recomputed Go policy, removed a partial/fuzzy match, guessed a provider, or treated document application as an acceptance gate | `prepare_turn_memory_transport.go`, `pdfmemory/generator.go`, `onMemoryTransportBodyInterceptor()` and the request-owned context | Add a selector/whitelist, a second retrieval/preparation, fuzzy deletion, automatic provider switch/retry, persistence rule, or output/save/reroll condition | Compare Text/PDF `payload_application_plan.v1` and logical hashes; require one document and exact selected Text removal only; test empty/font/body/ID failure returns Text/original body; then separately verify loaded RisuAI and each provider |
| Duplicate or partial canonical state appears | The whole turn was treated as one transaction, the Host adapter treated an existing raw pair as proof that derived work finished, or an ambiguous request was retried blindly | `onAfterRequest()`, complete-turn raw path, idempotency, admission, post-admission writers | Short-circuit `/complete-turn` from JavaScript merely because the raw pair exists, report core success as whole-turn success, or issue a new uncorrelated write | Verify an accepted raw-only replay reaches the existing Go `/complete-turn`, runs Critic without duplicating raw rows, while a raw-plus-derived replay remains idempotent; inject raw/core/post-write failures and verify `partial_commit` and recovery visibility |
| Repair Replay loses one valid role | A user/assistant conflict was treated as an all-or-nothing turn failure | `runChatLogRepairReplayWithProgress()`, normalization tests | Overwrite the conflicting row or skip the other missing role | Verify conflict preservation, independent missing-role insert, conflict counts, and idempotent rerun |
| Deleting one Say Nothing/middle user row shifts assistant/worldline pairing | Host single-message deletion left assistant/later messages, while adjacency pairing chose a new user anchor | `buildCompletedTurnPairsFromActiveChatMessages()`, assistant observations, session normalize/rescan, source revisions and topology | Hard-code `*says nothing*`, fabricate `[auto-continue]`, attach the assistant to the previous user, renumber later turns, or auto-mutate canonical/worldline data | Test ordinary middle user deletion, explicit auto-continue, Host-origin-unobserved synthetic input, single-delete versus tail-delete, stored-pair recovery, assistant-only review, 61+ turns, branch/restart, conflict no-write, and a privacy-safe role/index/hash support bundle |
| Identity merge rewrites facts or mixes entity kinds | A label match was treated as proof or JavaScript duplicated identity policy | `group_characters.go`, `group_items.go`, `entity_identity.go`, canonicalized readers | Auto-merge by name, rewrite existing rows, call Critic, or reindex automatically | Verify preview has no writes, explicit reviewed links only, character/item isolation, per-source partial results, unchanged underlying rows/vectors, and unmerge |
| Session normalization shows complete while work remains | `deferred` was treated as success or in-process job state as durable | `group_admin_session_normalize.go`, `admin_jobs.go`, rescan/identity repair | Force reindex, invent missing identity, or mark 100% completed | Verify stage order, ambiguity skips, pending/failed turns, deferred terminal state, retry fields, and restart limitation |
| Chroma is missing or stale after a successful DB write | A direct vector path was assumed to have outbox guarantees | Outbox processor, `turn_extraction_vector.go`, status/admin/migration/reference vector routes | Mark the index consistent without route-specific verification | Test direct-upsert/delete failure, reindex, stale cleanup, and real Chroma readback |
| `/ready` is green but the first DB query fails | MariaDB was opened with `sql.Open()` but not pinged | [`mariadb.go`](go-service/internal/store/mariadb.go), [`group_health.go`](go-service/internal/httpapi/group_health.go) | Treat `/ready` alone as real MariaDB proof | Run a real authority-mode connectivity/query and restart test |
| A roadmap feature or contract version is reported incorrectly | Contract versions were assumed to move together | Active producers, validators, consumers, callers, and current roadmaps | Call every v2 planned or every v1 active | Verify independently: recall/delivery/application are active v1; Publisher is active v2; old injection/recall v2 proposals are **OBSOLETE** unless a current roadmap explicitly reintroduces them as **PLANNED** |
| Full prepare fails and legacy memory unexpectedly runs | Transport failure was conflated with the post-response compatibility path | `tryPrepareTurn()`, `onBeforeRequest()`, `applyGoPayloadApplicationPlan()`, `orchestrateTurnHelpers()` | Enter legacy reads after a failed or ineligible `/prepare-turn` call | Verify failed source/full calls preserve the original payload; verify legacy reads occur only after an eligible full response lacks a compatible compact Go plan |
| Publisher guidance is missing, fabricated, or shown as a successful malformed call | Provider output was empty, malformed, schema-invalid, source-unbound, or treated as truth | [`group_proxy.go`](go-service/internal/httpapi/group_proxy.go), [`group_turn_prepare.go`](go-service/internal/httpapi/group_turn_prepare.go), [`prepare_turn_render.go`](go-service/internal/httpapi/prepare_turn_render.go) | Parse prose loosely, retry with altered parameters, fabricate defaults, fall back to v1, or mark malformed/schema-invalid as succeeded | Test one request, strict `publisher_plan.v2`, accepted/rejected source refs, `ready`/`partial`/`valid_empty`/error statuses, malformed Publisher stage `failed`, turn `completed_with_warning`, all strengths, and all render formats |
| Lorebook entries vanish, duplicate, leak scope, or become facts | The separate snapshot contract, exact scope, no-idempotency boundary, or `reference_only` authority was ignored | Adapter lorebook symbols, [`group_lorebook_reference.go`](go-service/internal/httpapi/group_lorebook_reference.go), [`prepare_turn_lorebook_reference.go`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go), [`mariadb_lorebook_reference.go`](go-service/internal/store/mariadb_lorebook_reference.go) | Read Host data in Go, blindly retry a POST, index it in Chroma, or save it as canonical memory | Test all modes/statuses, exact scope, duplicate text, out-of-order/concurrent snapshots, rollback, unavailable Store, no-promotion, and loaded-Host shapes |
| Fresh install or package lacks current tables/code | A current migration through `013` was omitted, `001_schema.sql` lorebook parity was assumed, or a stale package was used | [`migrations`](migrations), schema loader, build scripts, package manifest/hash | Edit packaged SQL or cite a version-log artifact as current | Build from the audited source, verify all migrations/code/plugin inclusion, and run disposable MariaDB fresh-install plus upgrade |
| `N` at the Windows Ctrl+C prompt still stops the backend | Managed children received the console signal before the launcher confirmation, or the generic BAT prompt was mistaken for service ownership | `windows-console-control.ps1`, `Start-ArchiveChildProcess()`, `Wait-ArchiveBackendLifetime()`, Job Object cleanup | Remove kill-on-close, leak children, or restart an already stopped backend after `N` | Generate Ctrl+C in an isolated console; verify the first `N` preserves the same child PID, the next `Y` enters cleanup, and parent loss still kills the managed process group |
| User content or credentials appear in diagnostics | Debug previews or bridge/provider errors were logged too broadly | Adapter debug logging, proxy/provider error scrubbing, config handling | Log full payloads, DSNs, tokens, keys, or unbounded provider bodies | Test debug on/off with synthetic secrets and verify masking/bounds |

## 9. Required Tests Before Completion

Apply only the relevant rows, but never substitute a test-only implementation for the production owner.

- [ ] For every change, run targeted production-path tests, then `go test ./...` when Go code changes. Confirm the regression fails against the broken behavior or contains an equivalent negative assertion.
- [ ] For `Archive Center.js`, run `node --check "Archive Center.js"`, the relevant production-executing tests under [`go-service/cmd/js-route-variant-smoke`](go-service/cmd/js-route-variant-smoke), and an actual supported RisuAI lifecycle/payload check when making live-host claims. Verify input observes and ends stale context, first beforeRequest captures/applies, exact same-turn reentry reuses without new preparation, afterRequest consumes once, and output only observes bounded worldline facts.
- [ ] For Yumi Translator 1.4.2 compatibility, execute `buildYumiV1ArchiveReadContext()` from production source against exact complete v1 markers and plain, `u:`, and `z:` `scriptstate` records. Verify assistant Archive-read copies use `model`, prefix/suffix remain, user and unmarked messages remain unchanged, missing/malformed records keep translated inner text, source arrays and raw observations are not mutated, and `onBeforeRequest()` uses the copy only for Archive reads. Verify actual loaded-plugin ordering separately.
- [ ] For Risu main-request retry work, use a fail-once provider fixture that drives `beforeRequest → retryable failure → beforeRequest → success → afterRequest`. Cover fallback-clean payload, exact auxiliary reuse/deduplication/conflict, `/prepare-turn` once, `/complete-turn` once, a real different overlap, identical text on the next turn, reroll, Say Nothing, session/worldline transition, final failure followed by new input, duplicate `afterRequest`, and HUD stage 6 retention followed by stage 7 acceptance. Do not require a naturally occurring timeout to make the regression deterministic.
- [ ] For API contracts, test the Go producer/validator, DTO/version/hash, JavaScript consumer, incompatible-version rejection, and actual error/retry behavior. Cover active v1 recall/delivery/application separately from active `publisher_plan.v2`. Inspect [`prepare_turn_source_contract_test.go`](go-service/internal/httpapi/prepare_turn_source_contract_test.go), [`prepare_turn_guidance_contract_test.go`](go-service/internal/httpapi/prepare_turn_guidance_contract_test.go), [`group_supervisor_boundary_test.go`](go-service/internal/httpapi/group_supervisor_boundary_test.go), and [`complete_turn_idempotency_test.go`](go-service/internal/httpapi/complete_turn_idempotency_test.go).
- [ ] For storage or commits, test source acceptance/revision, duplicate replay, transaction rollback, partial prefixes, post-admission failure, and restart recovery. Use [`mariadb_memory_admission_test.go`](go-service/internal/store/mariadb_memory_admission_test.go), [`complete_turn_source_revision_test.go`](go-service/internal/httpapi/complete_turn_source_revision_test.go), and [`memory_admission_worker_test.go`](go-service/internal/httpapi/memory_admission_worker_test.go) as production-owner coverage.
- [ ] For logical-turn replacement, test normal append, same Host user-row reroll, same Host user-row edit after assistant deletion and regeneration, a genuinely new Host user row with identical text, absent user-message chat ID fallback, observed-pair ordinal transport, existing idempotency terminal replay, MariaDB non-tail rejection, pending restoration/terminalization, new-key replay of the same failed revision, and no Critic/canonical/vector mutation after rejection.
- [ ] For deletion/rollback, test intact output, replacement/supersession, observed deletion, incomplete observation, poisoned counters, wrong route, changed route revision, digest/token replay, exact tail mutation, queued vector cleanup, and loaded-host Timeline behavior. Never treat opening Timeline as deletion proof.
- [ ] For empty/continuation and missing-role work, test explicit observed empty input, ordinary non-empty input, Host synthetic-origin `unobserved`, a complete synthetic user/assistant pair, deletion of only the middle user, deletion of the whole tail, `stored_pair_recovered`, `assistant_only`, no assistant reassignment, no silent turn renumber, no canonical/Critic/vector/worldline mutation on conflict, and idempotent dry-run/review. Never use literal Say Nothing prose as the classifier.
- [ ] For worldline changes, test ordinary turns, confirmed parent/child and nested forks, `user` versus `char` fork boundaries, unresolved/conflicting/cyclic/depth-limited lineage, no parent-post-fork/sibling leakage, per-history-session vector search, canonical hydration turn fences, topology/manual repair, and actual loaded-host branch observation.
- [ ] For schema changes, validate the complete migration inventory through `013`, the schema executable's sorted-sibling behavior, package inclusion, and disposable MariaDB fresh-install and upgrade paths. Verify that managed migration-file changes do not block the updater and that existing database data, runtime data, local environment files, and secrets remain outside managed-file replacement. Do not invent a requirement that later tables be duplicated into `001_schema.sql`.
- [ ] For retrieval, suppression, budgets, or ordering, test canonical hydration, exact/lexical eligibility, vector-success lexical evaluation, protected exceptions, occurrence-aware dedupe, world-rule collapse, multiplicity-safe character delivery, objective-memory K scope, global character budgets, complete candidate tails until final selection, whole-item deferral, lane order, empty payload `input_context_text`, hashes, and delivered payload lineage. Inspect [`prepare_turn_memory_policy_test.go`](go-service/internal/httpapi/prepare_turn_memory_policy_test.go), [`prepare_turn_memory_budget_test.go`](go-service/internal/httpapi/prepare_turn_memory_budget_test.go), [`output_fidelity_lineage_test.go`](go-service/internal/httpapi/output_fidelity_lineage_test.go), and [`output_fidelity_guide_efficacy_test.go`](go-service/internal/httpapi/output_fidelity_guide_efficacy_test.go).
- [ ] For an authorized 4.2 Priority Score change, test score propagation through the production Go source-projection→`PriorityFactSeed`→candidate→budget→render path; the Go-owned current query plus completed user/final-assistant conversation pairs up to UI `recent_conversation_reference_count`, one embedding per pair, independence from Chroma result `top_k` and final per-group core-memory K, strongest-similarity duplicate merge, and fact-scoring text parity without phrase classification; multiple facts from one source row; full recalled `memories.turn_summary` rendering with the higher of its best child score and observed aggregate-vector score; the same K applied independently to the complete-turn-summary group and every scored fact lane; no unused-slot transfer; exact one-sentence summary/fact delivery dedupe without consuming the fact lane's K; custom per-class UI character budgets on the same `memory_delivery_plan.v2` path; shared `event_recent` character budgeting for complete summaries and event facts; a canonically hydrated `precise_memory_unit` similarity reaching only its matching fact without parent/sibling score inheritance; the dedicated aggregate-memory vector search remaining present; no session-wide Memory-row bypass around aggregate recall; a dedicated existing-metadata `source_table=precise_memory_units` scoring search using the same request query vectors and exact canonical candidate count rather than final K; private hit handoff removal before response; stored importance independent of age, RP-turn-distance recency floor, independent bounded speaker/location/storyline biases, and deterministic final rank; identity metadata staying attributable but outside event/current-state K; per-fact visibility/perspective/source lineage without a new rejection gate; zero or unavailable relevance remaining rankable rather than becoming a hard rejection; an ordinary short continuation through the same query path; an explicit older-event query still outranking recency when semantically closer; protected recollection content and its guard staying one unit; Korean one-rune particle inflection; high-importance ordering among similarly relevant facts; protection against an unrelated high-importance takeover; no character-budget fill after each group's K; complete item deferral instead of raw JSON or summary truncation; unseeded-source compatibility fallback; factual candidate/count/character diagnostics; current input/direct correction/privacy parity; Text/PDF selected-order/hash parity; and score/rank lineage through payload and displayed-final observation. For compact `/prepare-turn`, verify `trace_preview.vector_recall_query` contains only the six query source/count diagnostics and does not restore the full `recall_result`, query text, embeddings, or vector values. Do not hard-code one fixture's expected numeric score into production or duplicate the ranker in tests.
- [ ] For the authorized 4.2 selectable finalization timing, freeze the current `응답 직후` production behavior first and keep it as the default. In `다음 사용자 입력 시`, test that the actual compact `/prepare-turn` policy survives the production orchestration-result handoff into `onAfterRequest()`; then test exactly-once completion of the edited/rerolled Host pair immediately before the new user row; same-row reroll and edit/regenerate replacement; identical-text new-row append; pending last response; restart; Say Nothing; provider retry; session/branch transition; Yumi Translator model-original input; generic backfill convergence; confirmed-memory horizon before the pending pair; current RisuAI recent-context visibility; and a slow/failed previous Critic that never delays the current main response. Verify that this mode alone preserves the previous nonterminal backend HUD ledger while the current prepare starts, opens one additional request-scoped event stream, and renders current original stages 1–6 above previous original stages 7–12 as separate `1/6~6/6` cards. Verify that accepted output closes only the finished current-generation card and that clicking a terminal previous card removes only that card. Verify that `응답 직후` retains both its single primary `1/12~12/12` HUD and same-session supersession behavior. Use the existing `/complete-turn`, Critic, persistence, reprocessing, and vector-outbox owners rather than a duplicate path, and keep source/regression/package/loaded-host/real-DB/provider evidence separate.
- [ ] For `memory_injection_baseline.v1`, test all nine surfaces and keep candidate/selected/rendered/payload-applied/displayed-effect stages distinct. Verify stable baseline IDs, same-row lane duplication, cross-surface and current-input/recent-context duplicate candidates, no automatic suppression, no canonical/vector mutation, and no live/display claim from source tests alone.
- [ ] For PDF memory transport, compare Text/Google/Gateway modes against the same completed `payload_application_plan.v1`; verify exact Unicode searchable extraction including required Hangul/Hanja, deterministic multipage output, one transient base64 field, exact Text removal, one provider document block, other lane/current-input preservation, request-owned retry reuse, unload cleanup, no complete-turn persistence, and Text/original-body fail-open for every generation/body/ID mismatch. Rebuild the Windows package and verify a single live backend/MariaDB/Chroma stack. Keep loaded RisuAI/provider body, actual usage and displayed-final proof as separate gates.
- [ ] For identity or normalization, test preview no-write behavior, explicit reviewed links, entity-kind/session isolation, per-source partial success, unchanged underlying rows/vectors, unmerge, mixed Repair Replay conflicts, ambiguity skips, staged progress, `deferred` terminal state, and reindex gating.
- [ ] For Host lorebook work, test `off`, `search_only`, and `reference_assist`; observed/partial/unavailable/revoked/empty/out-of-order/concurrent snapshots; exact-scope replacement; same-scope attempt suppression; ambiguous retry; transaction rollback; Store-mode availability; conditional lane order; Publisher `reference_only` support; no canonical promotion; no Chroma write; session delete/migration expectations. Use [`group_lorebook_reference_test.go`](go-service/internal/httpapi/group_lorebook_reference_test.go), [`prepare_turn_lorebook_reference_test.go`](go-service/internal/httpapi/prepare_turn_lorebook_reference_test.go), and [`mariadb_lorebook_reference_test.go`](go-service/internal/store/mariadb_lorebook_reference_test.go).
- [ ] For indexing, test outbox lease/retry/exact-readback/stale compensation, successful contextual embedding context-chunk compaction, failed/empty embedding retry evidence, and every affected direct writer separately. Run [`memory_vector_outbox_processor_test.go`](go-service/internal/httpapi/memory_vector_outbox_processor_test.go), [`group_reference_vectors_test.go`](go-service/internal/httpapi/group_reference_vectors_test.go), and a real Chroma compatibility/readback test when making live-index claims.
- [ ] For fallback modes, test failed/ineligible source decision, failed full prepare, eligible response without a compact plan, missing/incompatible application plans, intentional-skip protection-only behavior, missing captured afterRequest context, malformed Publisher/Critic output, token exhaustion, lexical fallback, lorebook unavailable/fail-open behavior, provider-config sync, unreachable MariaDB, and main/reference Chroma failure. Verify that transport failure preserves the original payload and does not start legacy reads.
- [ ] For packaging, rebuild from active source, validate only the three packaged binaries, compare plugin/schema/prompt identity, verify migrations through `013`, manifests/checksums, strict-semver versus display-label behavior, fresh-installer checksum acceptance/rejection, and package smoke/update/rollback checks. Keep package evidence separate from source, loaded-host, database, provider, and release evidence.
- [ ] For Windows launcher lifetime changes, run [`ops/windows-launcher-ctrl-c-smoke.ps1`](ops/windows-launcher-ctrl-c-smoke.ps1) in an isolated console and choose `N` then `Y`. Verify the same managed child PID survives `N`, confirmed cleanup follows `Y`, the package contains the console-control helper, and Job Object parent-loss cleanup remains present.
- [ ] Before completion, review `git diff`, preserve unrelated dirty files, confirm no generated output was hand-edited, report JavaScript lines added/removed, and list any adapter-side business logic that remains.

Do not report source tests, syntax checks, mocks, fixtures, built packages, loaded RisuAI behavior, real MariaDB/Chroma behavior, provider/OS behavior, or release status as interchangeable evidence.

## 10. Documentation Update Rule

Update [`STRUCTURE.md`](STRUCTURE.md) and [`AI_GUARDRAILS.md`](AI_GUARDRAILS.md) in the same bounded change whenever any of these changes:

- an entry point, hook stage, callback meaning, payload mutation point, or output-finality rule;
- component ownership or the RisuAI adapter/Go backend boundary;
- an API route, DTO, contract version, hash, ordering rule, error code, or compatibility window;
- retrieval, eligibility, suppression, ranking, budgeting, lane assembly, or payload application;
- a canonical or auxiliary persistent writer, transaction boundary, source fence, idempotency rule, partial-commit state, worker, retry, lifecycle, or recovery path;
- a MariaDB schema/migration, canonical/reference ownership boundary, Host lorebook scope/lifecycle, or Chroma query, hydration, outbox, direct mutation, readback, retry, or compensation path;
- a runtime/store/vector mode, auth rule, readiness check, provider owner, prompt owner, or fallback/degraded behavior;
- a build input, generated package layout, manifest, installer, updater/schema sequence, or release-validation rule;
- a planned feature becomes active, an active path becomes obsolete, or an **UNKNOWN** area becomes verified.

Update the implementation evidence first, then update both documents with current paths and symbols. Never use a documentation edit to claim that unimplemented or untested behavior now exists.

### 4.2 release CI evidence

Use the existing `.github/workflows/ci.yml` Windows and Ubuntu/macOS fresh-install
jobs plus the release archive/updater checks when publishing 4.2. Do not describe
mocked installer download/start boundaries or cross-built platform binaries as
real native-device runtime installation. Keep native updater tests distinct from
the loaded RisuAI plugin's separate update operation.

The [4.2.0 release verification](docs/archive-center-4.2.0-release-verification.md)
records public assets, the tagged-source CI run, and a real GitHub 4.1-to-4.2
managed update with MariaDB/Chroma fixture preservation on isolated Windows.
Keep that verified scope distinct from loaded RisuAI, the user's original data,
full runtime downloads, and native-device coverage on other platforms.
