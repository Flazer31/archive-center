# Archive Center Repository Structure

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


## 1. Document Status

| Field | Value |
| --- | --- |
| Review date | 2026-09-09: 4.3.0 stable source and release verification; test.23 behavior retained |
| Branch | `work/4.2.0` |
| Commit | 4.1 public parent `574c2d5b`; 4.2 source/regression checkpoints `43bc20a1` and `c2f1a2d5`; public `v4.2.0` release source `4257081c217e57b7e570592fb1090b484255c013`, including the fact-semantic relevance correction. |
| Repository root | active `source/` worktree |
| Prior local test build | [`4.3.0-test.23`](docs/archive-center-4.3-test-build-23.md): branch coordinates while parent source is pending; inherited-prefix and canonical-text preservation in cold start. Retains test.21–22 memory/preprocessing work. Plugin and Go backend are packaged together; user owns startup. |
| Current work summary | 4.3.0 stable published; source tag commit `51d901b`. OpenCode Zen and Go added. [4.3 history through test.23](docs/archive-center-4.3-status-summary.md). Source checkpoint: `6ef8f74239af065d2810761dd94c846b34e99826`; previous test.22 package retained. |
| Next-version plan | [Reliable recall plan](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#good-memory-plan): 4.4 deduplication → 4.5 context bundles → 4.6 time/state → 4.7 retrieval → 4.8–4.9 related recall → 5.1-A–5.2 shared reactivation. [4.4 execution plan](docs/archive-center-4.4-refactoring-plan.md) and [state-time handoff](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#temporal-state-44647) retain their scopes. Actor expression is optional. All follow-up work is **PLANNED**; runtime ownership and behavior are unchanged. |
| Inspection scope | Pending-source and cold-start red-to-green regressions, existing lifecycle suite, and source/package verification recorded in test.23. Loaded test.23 and actual user DB/provider behavior remain open. |
| Intentionally excluded | Dependency caches, compiled-binary internals, database data, logs, bulk traversal of generated packages beyond targeted manifest/hash/symbol checks, and unrelated dirty-worktree contents |
| Evidence level | Source/regression, seven public 4.2.0 packages, tagged-source Windows/Ubuntu/macOS CI, and an isolated Windows public 4.1-to-4.2 managed update with real MariaDB/Chroma fixture preservation are verified within the [release record](docs/archive-center-4.2.0-release-verification.md). Earlier bounded PDF/Provider Manager observations remain limited to their stated artifacts. Loaded-RisuAI verification of the release, live recall quality, Google AI Studio/LLM Gateway behavior, usage comparison, long-session generalization, and full native-device coverage remain open. |
| Confidence | **VERIFIED** within each explicitly cited source/regression/package/backend-live/public-release tier; **UNKNOWN** for the remaining loaded-host, provider body/usage/display, and native-device behavior in section 20. |

### 2026-09-09 test.23 pending-source lineage and cold-start merge

- `group_turn_range_decision.go::resolveRisuWorldlineObservation()` and
  `worldline_message_origins.go::risuWorldlineObservedSourceTurn()` resolve the
  copied prefix's coordinate from the already named parent observation when the
  fork source revision is still pending. Stored source coordinates account for
  offsets; a confirmed parent's inherited endpoint anchors its first owned turn.
  Unanchored root observations reuse the existing Go turn calculator. The existing
  inherited-through user/char distinction and ancestor scope traversal remain.
- This records lineage, not the missing response's memory. JS
  `queueNextInputFinalization()`/`beginNextInputFinalizationPipeline()` retain
  per-session ownership; a child's input does not finalize the parent's marker.
- JS `computeActiveChatRescanDryRunPlan()` now consumes Go's inherited-prefix
  exclusion in its second assistant merge and reuses the existing persistence
  normalizer before producing repair entries. Host hashes remain observation
  identities. `buildSessionNormalizeRepairEntriesFromDryRunPlan()` consequently
  receives owned canonical entries instead of reintroduced inherited/translated text.
- `worldline_cold_start_reproduction_test.go` runs active JS functions against the
  actual routing handler; only Host/Store reads are fixtures. Separate owner tests
  cover source delay, preserved/reissued IDs, user/char endpoints, saved offsets
  and five-session ancestry. Earlier source fails these new assertions. See
  [test.23](docs/archive-center-4.3-test-build-23.md) for results and upstream commits.

### 2026-09-09 test.22 preprocessing finishing scope

- `prepare_turn_multi_agent.go::parseMultiAgentRecommendation()` normalizes
  recorded `search_requests` array items with string `question`/`query` fields.
  Memory IDs keep their existing decoder; valid later fields remain usable when
  a question is malformed. Existing one-search/two-analysis limits remain in force.
- `multiAgentOrderCandidates()` takes each fact lane's order from its own role,
  and summary order exclusively from event_recent's summary list. Another role's
  mention does not rank the owning lane. Go fallback lanes retain their order.
- Common and five role defaults distinguish the recorded detail, source time,
  possible relevance and missing information. User creative direction and useful
  associative/peripheral evidence remain supported. Saved prompt overrides win.
- `buildPrepareTurnPreprocessingNotes()` keeps diagnostic items and source_catalog
  intact. The joined text uses documented short provenance keys and one scope
  heading for adjacent uncertainties. Source IDs, values, null/absent fields,
  owner/viewers, original reasons and unresolved text are retained. The Publisher
  support projection continues receiving the full diagnostic catalog.
- New owner regressions fail at the checkpoint and pass after repair. Offline
  replay covers 20 supplied replies and both frozen accepted selections. It
  verifies representation and delivery, not newly generated AI judgments. No
  external AI calls, database changes or backend startup were performed.
- JavaScript changes only four version identifiers (+4/-4). Full evidence and
  prompt-reset instructions are in the [test.22 record](docs/archive-center-4.3-test-build-22.md).

### 2026-09-08 planning alignment — reliable recall before optional expression

The integrated roadmap now distinguishes 5.1-A–5.2 shared recall/accessibility
from 5.1-B and 5.3–5.7 optional Actor memory expression. Shared recall returns
useful evidence through the existing Go selection/budget and host application
owners with preprocessing and Publisher OFF as well as ON. Actor dormancy and
partial-expression states are not extra filters on ordinary memory delivery.

The six common cases cover old important events, small details, similar events,
state changes, alternative cues and ordinary RP. Follow candidate retrieval,
selection, actual input and output use separately; compare all four specialist/
Publisher ON/OFF combinations and existing no-recommendation/failure behavior.
This is a planning-only update: test.21 remains implemented_unverified in live
RisuAI, and no new runtime field, API, selection policy or package is introduced.

### 2026-09-08 test.21 common memory delivery repair

- `character_perspective.go::buildCharacterPerspectivePacket()` retains already
  scoped ordinary `subjective_memory` as typed request-local fact seeds, carrying
  holder, source unit and turn. `group_turn_prepare.go` passes these to
  `prepare_turn_assembly.go`, which places them in normal `subjective_relationship`
  selection. Existing actual protected knowledge retains its own guidance path.
  `finalizeCharacterPerspectivePacket()` observes only delivered records and removes
  the internal seed field before exporting the packet.
- `prepare_turn_priority_memory.go::buildPrepareTurnPriorityMemoryDeliveryPlan()`
  uses the existing integer setting as a core priority target per summary/fact group.
  Groups take core turns; remaining group heads use score order within existing
  global/lane character budgets. AI groups retain their explicit order. The legacy
  wire key is unchanged; `core_priority_memory_delivery.v4` describes the new meaning.
- `prepareTurnBuildPriorityCandidates()` resolves an existing current-field group
  by source turn before relevance score. Current-field identity and evidence ID are
  separate: source ref, turn and value distinguish updated candidates, allowing the
  existing supplemental merge to retain both sources for round two. This is not
  per-field temporal extraction or a historical-state graph.
- Preprocessing consumes the same source pool; no recommendation/failed roles use
  the captured Go selection. Empty recommendations retain the ordinary rendering
  order. Accepted AI text/order stays intact, including prior recommendations after
  supplemental failure. Character budgets are declared to AI before selection;
  Go does not add a post-hoc cut to accepted AI recommendations.
- Validation: [test.21](docs/archive-center-4.3-test-build-21.md), new four-group
  regressions that failed before repair, all 35 Go packages and frozen supplied
  content across OFF/AI/empty/partial failure. Frozen source metadata is explicitly
  synthetic; live MariaDB/Chroma/provider/RisuAI quality remains unverified.

Earlier dated sections describe their respective builds. The test.21 policy above
supersedes their hard-K/no-backfill wording; their other boundaries remain intact.

### 2026-09-08 test.20 preprocessing presentation and evidence references

`prepare_turn_multi_agent.go::multiAgentModelInput()` projects the existing canonical
input at `callMultiAgent()` only. Current input/recent conversation come first.
Repeated complete provenance becomes P entries; unique metadata stays inline.
Source time, private owner/viewers, original text/IDs and candidate order are retained.
The original `input` still owns canonical reference resolution; `model_input` and
character counts describe the actual serialized user message. This does not change
candidate admission, K, provider configuration, call count or failure behavior.

`multiAgentSelectionReferences()` retains the calls' F/S/L mapping. The existing
priority renderer labels AI-selected facts/complete summaries with F/S references;
`buildPrepareTurnPreprocessingNotes()` uses the same references for interpretations
and shares their scope catalog. Go-baseline memory text remains unchanged.
`prepare_turn_planner.go::supervisorDeliveredContextItems()` maps the actual rendered
fact and complete-summary lines to original sources. `group_turn_prepare.go` attaches
notes/catalog; `group_proxy.go::publisherModelSupportPacket()` carries their references
and catalog to the real Publisher input. No recommendation is re-ranked or rewritten.
The shared default prompt reviews exact evidence in the existing second round;
saved custom prompts keep their priority. There is no new narrative restriction.

The [test.20 record](docs/archive-center-4.3-test-build-20.md) separates red/green
production regressions, ten frozen input-preservation comparisons and package proof
from actual model behavior. External replay is pending explicit transmission approval;
loaded-RisuAI and displayed RP quality remain unverified. User settings/data were
not changed and the user's backend was not started. JS changed four version lines only.

### 2026-09-08 test.19 memory editor prompts and handoff purpose

`prepare_turn_multi_agent.go::multiAgentSharedPrompt` and `multiAgentRolePrompts`
define five complementary memory editors: event causality, character condition,
perspective/relationship/secrets, setting/objects and ongoing threads. Reasons
connect recorded evidence and supplied transitions to the present scene; the user
owns creative direction. The main writer receives context even with Publisher OFF.

`runMultiAgent()` sends the existing accepted public handoff with a separate
`request_reason` attributed by `from_role`. Original `text`, ID/ref and source
metadata remain canonical; the request purpose is AI interpretation. The recipient
examines its own category in the existing second round. Selection lanes, source
scope, search limits, no-recommendation Go selection and final note assembly are
unchanged. There is no added reply round, persistence path or prose validation gate.

The [test.19 record](docs/archive-center-4.3-test-build-19.md) covers production-owner
regressions, the new Windows backend/package and settings-save observations. Saved
custom prompts still override defaults; the user's six fields matched old defaults
and were saved empty through the UI to follow backend defaults after restart.
Loaded test.19, real provider behavior and RP quality remain unverified.

### 2026-09-08 test.18 packaging

The [test.18 record](docs/archive-center-4.3-test-build-18.md) packages the following
source changes. Their original source-only notes below describe evidence at repair
time; test.18 adds package verification, not a backend launch or loaded-RisuAI claim.
The existing `ops/build-full-package.ps1` builds the active Go service and helpers,
copies the active plugin/prompts/migrations and sets launcher/runtime-template version.
The new backend hash differs from test.17. Existing packages and user data are preserved.

### 2026-09-08 narrow HUD and preparation tiles

Active `Archive Center.js` uses a 224px viewport-clamped HUD and 10px card padding.
`turnWorkflowHUDTimingHTML()` keeps preparation details collapsed, with the total
above a two-column grid of small timing cards. The existing count-ledger styles
also render generated/stored categories as matching two-column label/value tiles,
with a bordered total summary. Count values, order and phase visibility are unchanged.
`attachTurnWorkflowHUDDismiss()` observes native detail-open count changes through
the official SafeElement API before applying card-coordinate dismissal. Normal
completed cards follow existing card/X policy, including the previous-slot X;
warning/error cards retain X-only dismissal. No listener, observer or timer is added.
Existing stage
values, AI round table, elapsed timer, Host observation and backend states retain
their owners. This is UI rendering only; no policy, provider or storage changes.
See the [UI record](docs/archive-center-4.3-preprocessing-work-log.md) for isolated
Edge viewport/zoom checks and the existing production UI regressions. The test.17
package still contains its earlier 268px layout; loaded RisuAI proof is separate.

### 2026-09-08 retrieval score retention

`prepare_turn_priority_memory.go` is the direct owner of the v4 scoring change.
`appendPrepareTurnPriorityMemoryFactSeeds()` carries existing aggregate-vector
provenance into candidates; `prepareTurnBuildPriorityTurnSummaries()` compares
the aggregate-vector score with the existing best-child score. The chosen score
and observed source vector are exposed by `prepareTurnPrioritySummaryMap()`.
Sibling facts retain their own relevance. `prepareTurnPriorityScore()` is shared
by fact and summary ranking and keeps stored importance separate from RP recency.
There is no new retrieval, storage, provider, Host or fallback path.

See the [repair record](docs/archive-center-4.3-feedback-work-log.md#retrieval-score-retention-repair)
for the failing-before/passing-after source regressions, unchanged historical
comparison fixtures and full Go suite. K, budgets and received AI selection order
are preserved. This change is packaged in test.18; live DB/vector,
loaded RisuAI and displayed RP evidence remain separate. State-time/commitment
resolution and paraphrased knowledge links are still planned follow-up work.

### 2026-09-08 knowledge continuity first repair

`turn_extraction_private.go` retains supplied secret/identity IDs and identity
evidence/transition metadata. `turn_entity_identity.go` resolves explicit
identity-scope holders through the existing identity projection.
`turn_precise_memory.go::protectedSecretPerspectiveMemoryCandidates()` feeds
identity mappings into the same evidence-bound per-holder observation writer as
protected secrets. The default Critic prompt describes the matching output fields.

`character_perspective.go::buildCharacterPerspectivePacket()` separates independent
secret and subjective claims within category slots. Supplied secret IDs or the
recorded claim distinguish those items; ordinary single-valued belief slots retain
latest-state behavior. Known/revealed states of one claim are compatible in both
candidate creation and reading, while original labels remain intact.

`prepare_turn_memory.go` distinguishes identity owners from informed observers
and no longer matches an arbitrary POV against itself. The assembly passes its
canonical memory context into protected-guidance grouping so an available later
disclosure can release the same secret's past guidance. Delivery lineage records
`released_by_later_disclosure` with the disclosure row/turn. Separate secrets,
other sessions and later private states retain their guidance; canonical rows and
their public event projections are not rewritten.

See the [repair and reinspection record](docs/archive-center-4.3-feedback-work-log.md#knowledge-continuity-first-repair).
Production-owner regressions pass and test.18 includes the repair. The running
backend and user DB were not changed by the build. Absent historical holder observations are not backfilled. Legacy
paraphrases without a shared ID are not semantically merged by this repair.

### 2026-09-08 HypaMemory original import

`Archive Center.js::importHypaMemory()` reads the current Host chat's
`hypaV3Data.summaries` and uses the existing `POST /import/hypamemory` route.
`group_audit_feedback_import.go::handleImportHypamemory()` stores each nonempty
original as one memory using `turn_extraction_persist.go::saveCriticExtractionArtifacts()`.
The canonical JSON keeps the exact original in `turn_summary` and
`hypamemory_import.original_text`, plus source order/index/tags/category and
analysis status. The Critic's shorter `turn_summary` moves to
`hypamemory_import.critic_summary`; its other analysis fields remain supplemental.
No table, background job, extra LLM pass or raw RP chat-log row was added.

Go allocates an unused negative import number when the supplied number is occupied.
Reimport matches stored original occurrences, preserving multiple equal entries
without adding them again. Existing imports lacking original metadata are not
guessed, rewritten or deleted. Failed/unconfigured Critic analysis still submits
the actual original to the existing memory writer. Counts distinguish new saves,
existing records, storage failures, empty inputs and analysis outcomes.

`group_turn_prepare.go` memory/evidence/KG readers and
`group_memory_explorer_read.go` retain negative external-import rows inside the
existing session history scope and keep positive turn bounds.
`store/mariadb_chat_memory.go` includes negative import rows in the existing range
queries without widening the positive branch range. Explorer's
`source=hypamemory` filter runs before pagination, recognizing both new metadata
and the old `hypamemory_import_score` marker. JS provides All/HypaMemory controls
and an expandable original. `memory_search_text.go` excludes the provenance copy
from public projection so it cannot bypass the memory body's visibility handling.
Source preservation does not mean every imported summary is injected each turn.

See the [implementation and verification log](docs/archive-center-4.3-feedback-work-log.md#hypa-original-import).
The actual split-chat data, loaded Host, MariaDB/Chroma import and final recall
remain unverified for this change. The repair is packaged in test.18; no backend
was started by packaging.

### 2026-09-07 test.17 compact HUD

`projectTurnWorkflowHUDPhaseView()` projects existing stages for
`next_user_input`: the current generation has no storage count ledger; the
previous finalization retains its counts without repeating generation timings.
Accepted Host response timing renders the current waiting stage as received,
while the original backend view and pending-save lifecycle remain unchanged.

`turnWorkflowHUDTimingHTML()` renders total/preparation/response metrics and a
five-row role table (round 1 / round 2 / AI or Go). Search and backend timings,
`turnWorkflowHUDStageLedgerHTML()` and `turnWorkflowHUDCountLedgerHTML()` use
collapsed details. Completed/error cards dismiss through X so expanding details
does not dismiss them; informational notice dismissal is unchanged. Existing
timer, stream, event cleanup and request-slot owners remain in use.

The root width is 268px with a viewport clamp. Removed three timing explanations
and the preprocessing Flex helper paragraph. No provider capability, model
prompt, Go schema, storage or inference behavior changed. Source/browser fixture
checks and package evidence are in [test.17](docs/archive-center-4.3-test-build-17.md);
actual loaded RisuAI remains a separate check.

### 2026-09-07 test.16 public memory handoff

The existing `appendPrepareTurnPriorityMemoryFactSeeds()` producer in
`prepare_turn_priority_memory.go` supplies `public_projection` source visibility.
`runMultiAgent()` in `prepare_turn_multi_agent.go` now includes that value in the
existing public handoff branch; owner/viewer/subjective restrictions are unchanged.
The receiving role gets canonical ID/ref/text/source table/source ref/source turn
in `related_evidence`. These references remain context, not a transfer of lane
selection ownership. No persistence, source-time or retrieval changes are involved.

`Test43MultiAgentHTTPPrepareDeliversSelectedCanonicalMemoryAndPublisherSupport`
now runs memory rows through production assembly and the registered prepare route,
then observes the second specialist HTTP input and selected memory in the final
injection and Publisher support. It failed before the repair in all three
Publisher modes. `Test43MultiAgentGeneralPublicHandoffKeepsPrivateScope` retains
public/general behavior and checks projection-labelled owner/viewer/subjective
scope. Source/fixture/package evidence is recorded in [test.16](docs/archive-center-4.3-test-build-16.md);
loaded RisuAI, real providers and final story behavior remain unverified.
JS is unchanged except four version identifiers; backend startup is user-owned.

### 2026-09-07 test.15 state-time observation and planned ownership

The supplied trace gives both `cash: 51냥 5푼` and the `61냥` asset description
the same `character_states:2709` source and turn 115; recent conversation records
46냥 5푼. The existing state writer merges prior fields and stamps the new row
turn. `prepare_turn_assembly.go` supplies that row turn as source metadata;
`prepare_turn_priority_memory.go` propagates it to fact seeds and uses it for
recency. Increasing the recency weight alone cannot distinguish their ages.
This is a storage/source-time/candidate issue, not resolved by the implemented
snapshot caveat or independent specialist-note delivery.

The canonical roadmap assigns duplicate reduction and comparison cases to 4.4,
per-field change/effective time and current state (including character funds,
assets and inventory) to 4.6, and consumption by retrieval/ranking/AI input to
4.7. New-write correctness and evidence-based legacy recovery are separate;
unavailable old field times remain unknown. These are plans, with no new schema,
runtime acceptance rule, migration or implementation in this documentation update.
See the [current observations](docs/archive-center-4.3-status-summary.md) for evidence limits.

### 2026-09-07 test.15 independent specialist interpretations

`buildPrepareTurnPreprocessingNotes()` in `prepare_turn_multi_agent.go` projects
received `reasons` and `unresolved` from the existing accepted analysis. Memory
reasons follow selected fact/summary order and reference actual delivered items;
lore reasons follow the separately retained lore assessment and its round.
Failed supplements retain matching first-round interpretations. Go baseline
selection does not invent AI reasons. Source references, visibility, perspective
owners and allowed viewers accompany interpretations; these remain advisory AI
text rather than canonical facts or a semantic secrecy guarantee.

After final memory/lore assembly and before Publisher, `handlePrepareTurn()`
attaches `memory_preprocessing_notes.v1` to the memory plan and its items to
`supervisor_support_packet.delivered_preprocessing_notes`. `group_proxy.go`
includes these items in the actual Publisher input and observed input size.
`prepare_turn_render.go` adds a separate `preprocessing_notes` payload lane,
independent of Publisher availability and narrative budget. Extra characters
are observed using `budget_mode: additional_observed`; original memory budgets
and text are unchanged. OFF adds no specialist lane.

JavaScript only localizes and renders the lane and its observed extra input.
Existing Host application and effective-input observation carry its text.
There are no additional AI calls, rounds, storage/schema changes, new acceptance
rules or postprocessing. Default prompts explain this use while saved edited
specialist prompts remain intact. This bounded slice ends with a user test build;
live RisuAI delivery, model interpretation and displayed story quality remain open.

### 2026-09-07 test.14 shared response recovery

`repairJSONCandidate()` in `turn_extraction_critic.go` is shared by Critic,
Publisher (`parsePublisherJSONObject`) and preprocessing. Existing quote,
literal and trailing-comma handling is reused; `repairJSONMissingArrayClosers`
recovers a missing `]` before an explicit sibling object field or enclosing `}`.
Quoted source content and original provider responses are retained. EOF does
not synthesize missing text or values. Publisher/Critic contract interpretation,
ordinary failure handling and canonical persistence retain their existing owners.

`parseMultiAgentRecommendation()` decodes each field independently, accepts a
single string in a string-list field and preserves complete IDs from interrupted
lists. A type error in reasons no longer hides later selections/search requests.
Local recovery adds no model calls or search rounds. A parsed lore selection
survives an unrelated field error; an actual provider failure retains the earlier
lore assessment. Existing failed-supplement memory selection behavior is retained.

The existing preprocessing HUD records `repaired`, `partial` and
`no_recommendation` call outcomes and the final memory `selection_source`.
JavaScript renders these backend decisions, keeping receipt separate from final
selection and from actual payload delivery. Lore candidates carry their observed
comment/key/heading beside their L ref; original text, IDs and order are unchanged.
Default response instructions request concise explanations and short references,
without changing saved prompts, model choice or user narrative authority.

The pending-thread support lane now uses the existing configured Host conversation
query set as context before scoring. Previously an implicit current instruction
could exclude an explicitly named ongoing goal from all specialist candidates.
It accepts the in-process `[]string` representation via `stringsFromAny` and keeps
other lanes, stored rows, suppression, scope and scoring owners unchanged.
Source/provider-fixture evidence and live gaps are in the preprocessing work log.

### 2026-09-07 test.13 source-time accuracy

`saveCharacterAndStateArtifacts()` merges existing character fields, so a
`character_states` row turn dates the snapshot, not every retained field's event.
`multiAgentInput()` and `runMultiAgent()` public handoffs now expose the existing
candidate `source_table` alongside unchanged IDs/text/turns. The input's existing
`reference_format` describes snapshot time and completed recent conversations.
`buildPrepareTurnPriorityMemoryDeliveryPlan()` labels these selected rows
`[state snapshot turn N; fields may be older]`; source facts, scores and selection
remain with their existing owners. The label reaches the existing Publisher
support packet and memory payload without a new state resolver.

`handlePrepareTurn()` uses `RecentConversationMessages` and the existing
`recent_conversation_reference_count` for the retrieval request and both
specialist rounds. Each completed conversation keeps observed user/assistant
content together. Default specialist/Publisher advice distinguishes known
completion from unknown details, with user revisions and custom prompts retained.
Regressions cover source/quantity preservation, both rounds with a configured
count, public/private handoff, and the actual Publisher request. This is source
and fixture evidence. The test.13 package includes these changes; loaded Host
and real-provider output verification remain separate.

### 2026-09-07 test.12 supplemental-search latency and observation

`runMultiAgent()` in `prepare_turn_multi_agent.go` runs the already-requested
supplemental searches concurrently after round one. Each active role retains its
existing one-query limit. Indexed result slots merge facts/summaries in original
role order, preserving duplicate ownership and request-local aliases before round
two. AI selection, source scope, search filters/limits and existing partial-failure
behavior stay with their existing owners.

The `handlePrepareTurn()` callback in `group_turn_prepare.go` uses the same scoped
retrieval path. External retrieval overlaps; a request-local mutex serializes
hydration and assembly of the existing shared request inputs. No service-wide
lock, persistent cache or new selection policy is introduced.
`buildPrepareTurnPriorityMemoryDeliveryPlan()` captures typed, unexported pristine
candidate/summary snapshots before rendering metadata or AI selection changes.
`multiAgentCandidatePool()` copies these snapshots rather than resolving the same
assembly again. Private text and nested slices remain request-scoped; these
internal fields are excluded from JSON.

`prepareTurnVectorShadowWithPreciseCandidateLimits()` records local `health`,
`embedding`, `vector_search` and `revision_checks` intervals. The callback adds
`hydration`, `assembly_wait` and `assembly`. Each query's `breakdown_ms` is distinct
from the whole supplemental phase's `search_duration_ms`; the latter alone supplies
the existing `backend_timing.stages_ms.preprocessing_search` stage. That stage
remains inside `injection_assembly`, not an additive sibling of it.

The existing `turn_workflow_hud.v3` ledger has an optional `preprocessing_search`
ViewModel: status, UTC started_at, duration_ms, query_count, completed_count and
ordered queries (role, status, duration_ms, breakdown_ms). It is omitted for OFF
or no searches. Snapshots deep-copy query timing maps and contain no question,
prompt, key or memory text. Partial/failed search states are diagnostics only.
`turnWorkflowHUDTimingHTML()` and `applyTurnWorkflowHUDStack()` render it through
the existing DOM/timer owners, without changing Host lifecycle or persistence.

See [test.12 evidence](docs/archive-center-4.3-test-build-12.md) and the
[preprocessing-only example comparison](docs/archive-center-4.3-preprocessing-comparison.md).

### 2026-09-07 test.11 user-directed guide strength

The user's subsequent clarification permits response execution priorities from
`strong` upward. The user retains authority over story direction, revisions,
pacing and their character's choices. Weak gives optional hints; Medium gives
connected recommendations; Strong asks for concrete enactment; Extreme connects
action, reaction and consequence; Maximum gives a current-response execution brief.

`publisherStrengthProfile()` owns the model-facing application descriptions.
`runSupervisorLLM()` sends the selected policy to the Publisher, and
`supervisorSceneProposalGuidanceItems()` carries it into the same single guidance
block in compact, standard and explicit formats. Stronger guidance changes the
requested depiction, not accepted items, source order, privacy, model parameters,
call count, budgets, output acceptance or persistence. Pressure remains independent.
The default four-field Publisher contract and saved user prompt files are retained.
See [test.11](docs/archive-center-4.3-test-build-11.md) for verification scope.

### 2026-09-07 test.10 preprocessing and creative guidance (prior checkpoint)

`prompts/supervisor_system.txt` and `publisherModelExecutionContract()` now present
historical memory as context and new developments as optional creative ideas.
User direction, including explicit changes to prior setting or relationships,
takes precedence. The default asks for four advisory fields, while existing
Publisher parsing continues to accept earlier response contracts. This changes
model-facing guidance, not completed-turn acceptance or canonical writers.

`multiAgentInput()` shares candidate text space between facts and turn summaries
(and between world facts and lorebook references). Request-local F/S/L aliases
remain stable across the two rounds and resolve to exact canonical references.
`runMultiAgent()` records candidate availability, unresolved references, partial
search and actual dispatch separately. Received valid memory selections retain
their original order and text; absent recommendations retain Go baseline behavior.

`handlePrepareTurn()` supplies existing scoped, scene-matched lorebook candidates
to the world specialist. `finalizePrepareTurnLorebookReference()` consumes its
optional selection directly, including explicit empty selection and selected
source order. A missing assessment retains ordinary Go reference selection.
First-round lore assessment survives a failed/unassessed supplement. Host lorebook
content and canonical stores are unchanged. Source-turn labels survive fact
rendering, so a historical “tomorrow” retains a visible origin.

`chromaStore.doJSON()` reads complete successful JSON responses instead of cutting
them at 1 MiB; existing bounded HTTP error reads remain. Search diagnostics now
retain scrubbed errors and distinguish partial results from hydration. The Host
HUD renders existing backend stage durations for the matching request without
adding overlapping intervals. Effective Input identifies backend preview,
pre-request observations and mismatch reasons separately, with no new output or
persistence rejection condition. See [test.10](docs/archive-center-4.3-test-build-10.md)
for source/fixture/package evidence and unverified live outcomes.

Status words used in this document have strict meanings:

- **VERIFIED** — directly confirmed in the evidence tier stated with the claim; an omitted tier is not implied.
- **INFERRED** — strongly supported by code but not proven end to end.
- **UNKNOWN** — the repository does not provide enough evidence.
- **PLANNED** — present only in roadmap/design material, not the active implementation.
- **OBSOLETE** — present in an old, copied, generated, or unreferenced surface rather than the active runtime.

**VERIFIED.** The worktree was clean at the audit baseline. The recorded HEAD is the source-evidence boundary for this revision. A clean tree, generated package, branch name, or previous copy is not evidence of a built, loaded, released, or live state. The permanent ownership rules were checked against [AGENTS.md](AGENTS.md) and [the host/backend boundary](docs/permanent-risu-host-backend-boundary.md), but implementation files and their actual callers remain the primary evidence. Line anchors are convenience links; path and symbol identity are the durable citation.

### 2026-09-07 Vertex retained service-tier correction

`proxyApplyLLMGatewayServiceTier()` leaves a valid retained service-tier setting
unapplied for Vertex, with `llm_gateway_service_tier_applied=false` and the existing
skip-reason field set to `vertex_uses_vertex_flex_mode`. `VertexFlexMode` remains
the owner of Vertex Flex headers. Stored settings are not changed, and no tier
is silently substituted. Other provider validation, explicit extra-body handling,
upstream errors and retry behavior remain unchanged.

The same Go request builder serves `/proxy/plugin-main?connection_test=critic`,
runtime-configured Critic calls, Publisher calls and shared preprocessing
connections. No additional JavaScript provider policy was introduced. The new
`TestVertexRetainedServiceTierAcrossCriticPaths` exercises the registered config
and connection-test routes and `runCompleteTurnCritic()` against HTTP boundary
fixtures. See the test.9 record for the reproduced error and verification limits.

### 2026-09-07 HUD preprocessing and response timing

`callMultiAgent()` publishes each enabled role's call start/result to the existing
request-scoped `turnWorkflowHUDLedger`. Its optional `preprocessing` ViewModel
contains ordered roles, round status/duration and each role's summed call time;
it contains no credentials, prompts or memory text. The existing event stream and
12 workflow stages remain the owners; no additional status request or stage is
created. OFF produces no preprocessing field. Snapshots copy nested timing arrays.

JavaScript renders/localizes this Go projection and updates running clocks through
the existing HUD timer. Host-only `host_timing` records HUD priming, first
`beforeRequest` return and the accepted `afterRequest` response observation using
one Host clock. Its display durations include request/retry waiting, not merely
provider computation, and stop at response receipt, excluding later Critic/save
and next-input waiting. They do not accept, persist or finalize a turn.
The current card retains its response timings until dismissal or the next request;
in next-input mode this is a finished generation presentation of the pending
backend workflow. Existing previous-turn finalization remains unchanged.

Source/provider-boundary/Host-fixture checks are recorded in the test.8 document;
loaded RisuAI verification of these timing fields remains open.

### 2026-09-07 preprocessing role prompts and password editing

The five Go-bundled role prompts now specify scene analysis, domain-specific
evidence distinctions, missing-evidence search, cross-role boundaries and complete
supplemental recommendations. Existing shared instructions, stored user overrides,
candidate access, selection behavior and round count are unchanged. The updated
defaults are used for empty role overrides or after an explicit restore/save.
Their delivery to both analysis rounds is tested; model selection quality is not
established by these transport tests.

At the user's request, `GET/PUT /config/memory-preprocessing` returns stored API
keys to the password editors instead of blanking them after every save. These
configuration responses contain credentials and use `Cache-Control: no-store`.
The separate `has_api_key`/`clear_api_key` fields and deletion checkbox are removed.
An explicit `api_key` value replaces the key, including an empty string to clear
it; an omitted key preserves the existing value. The UI labels the address
`Endpoint`, preserves saved keys on reopen, and continues to use password inputs.
Keys are not added to specialist prompts or inputs. Go remains the persistence
owner. See the test.7 record for tests and packaging evidence.

### 2026-09-06 preprocessing Flex and provider controls

Role settings persist `llm_gateway_service_tier` and `vertex_flex_mode`. Independent
connections pass their applicable options through `completeTurnLLMConfig` and the
existing proxy override owner; sharing keeps the Publisher's processing options.
Changing provider hides irrelevant controls while preserving their stored values.
Go only applies the independent service-tier value to a supported transport, so
a retained OpenAI-compatible tier does not break a later Vertex/Claude selection.
`proxyApplyLLMGatewayServiceTier()` now also supports AI Studio (`gemini`) using
top-level `serviceTier`, with `standard` for the default tier; OpenAI-compatible
requests retain `service_tier` and Vertex retains its existing Flex headers.
No model whitelist, model-capability lookup, extra AI retry or tier downgrade was
added. Exact model/account eligibility remains with the provider; UI visibility
follows the existing settings convention of provider-level controls.

`loadMemoryPreprocessingPanel()` uses the standard provider list, provider-specific
Flex selectors and always-visible per-role temperature/max-output-token fields.
Both analysis rounds retain each role's generation controls even during Publisher
connection sharing. HTTP-boundary and responsive browser checks are recorded in
the test.6 document; live provider billing and loaded RisuAI remain unverified.

### 2026-09-06 editable common prompt and independent AI connections

`multiAgentSettings.SharedPrompt` stores the user's common prompt override in the
existing preprocessing settings file. An empty value uses the bundled Go default;
GET exposes the effective `shared_prompt` and `default_shared_prompt` separately.
PUT omission preserves the saved override for older clients, while an explicit
empty value restores the default. `callMultiAgent()` uses the same settings snapshot
for first and supplemental rounds: chosen common prompt, assigned role prompt,
then the existing round task. A custom common prompt replaces the bundled common
text rather than appending another hidden copy. Prompt assembly remains Go-owned.

New role settings default to independent provider/address/model/key connections.
Explicitly saved Publisher sharing remains preserved and optional; it shares the
connection while retaining the specialist prompt. The UI edits and restores both
common and individual prompts. Failed saves preserve unsaved edits and report the
failure. HTTP-boundary tests verify five separate connections and both prompt
rounds; browser fixtures verify editing, restore, save, failure and responsive
layout. See the test.5 record for evidence and remaining live validation.

### 2026-09-06 preprocessing UI correction

The previous preprocessing form used undefined `mo-input`, `mo-label` and `mo-card`
classes. `loadMemoryPreprocessingPanel()` now renders styled role cards using scoped
CSS in `PANEL_CSS`, existing form conventions, separate connection and prompt sections,
and stacked mobile layout. Shared Publisher connections show the configured model;
custom fields remain in the DOM with their values preserved while hidden/disabled.
Each role retains its own prompt editor and restore action. Go still constructs
each call from shared rules plus that role's prompt; connection sharing does not
share the Publisher's prompt. `ops/preprocessing-ui-smoke.cjs` executes the production
panel and loader in a local browser with mocked config transport, covering layout
and independent edit/save behavior. It does not access an installed RisuAI or start
the backend. See the test.4 record for screenshots and verification boundaries.

### 2026-09-06 settings loading correction

`renderSettingsPanel()` mounts and binds the editable form before requesting the
Go dashboard ViewModel. Completion updates only `#mo-dashboard`, using the existing
render request identity; settings are not recomposed or overwritten. Its queue
action uses the existing dashboard container for event delegation. Failed status
requests retain the existing unavailable presentation. `bridgeFetch()` continues
to use the saved `settings.bridgeUrl` without host/domain substitution. Listener
binding belongs to the server launcher, not the browser's destination setting.
The user's configured Tailscale address became unreachable after the preceding
localhost-only restart; the explicitly approved Tailscale binding restored the
loaded PocketRisu preprocessing panel. See the test.3 record for evidence scope.

### 2026-09-06 preprocessing implementation checkpoint

**Implemented; loaded-Host/provider validation open.** Five optional specialists
now run stage-wise in parallel through `prepare_turn_multi_agent.go`, called by
`handlePrepareTurn()` after existing scoped candidate assembly and before final
memory/Publisher preparation. Existing retrieval performs at most one requested
search per active role; relevant roles receive one supplemental analysis. Canonical
recommendations retain their order, with explicit over-budget observation. An area
without AI recommendations retains this request's ordinary Go selection. The
existing `prepare_turn_priority_memory.go` renderer, payload and Publisher consume
the result; no new memory writer or output editing path was added.

`GET/PUT /config/memory-preprocessing` stores backend-wide user settings and prompt
overrides in the stable data directory. Defaults are compiled into Go; key values
are returned only as part of the editable configuration, with the current test.7
password-editing contract described above. `loadMemoryPreprocessingPanel()` renders the independent
Extensions tab and transports edits. The POSIX launcher exports its data root for
the same persistence contract. See [implementation and verification details](docs/archive-center-4.3-preprocessing-work-log.md).
The PLANNED 4.3 descriptions below are retained design requirements; this checkpoint
supersedes their earlier statements that no runtime/UI exists. Live release proof
must still be recorded separately.

### 2026-09-06 shared-host lineage repair checkpoint

**Implemented; loaded-host verification open.** The 4.3 feedback work now includes
the RisuAI/PocketRisu message-ID difference. `resolveRisuWorldlineObservation()`
keeps `branchedfrom`'s parent source ID and no longer equates it with a copied
child ID. `worldline_message_origins.go` consumes optional
`risu_message_origins.v1` metadata, resolves the parent's user anchor, records
child→parent message IDs and follows those mappings for nested branches.
`resolvePrepareTurnHistoryScope()` preserves the earlier cut when a child forks
inside its parent's inherited prefix.

The initial `onRisuOutput()` / `preflightActiveChatBackfillIdentity()` observation
remains 2–3 rows. Go can request a named parent prefix through the existing
session-routing response; `observeRisuWorldlineMessageOrigins()` reads the official
Host `getCharacterFromIndex` snapshot, freezes/transports ID/role/index metadata
and existing branch markers, and applies the returned routing result. Older
ancestor maps are filled through the same resolver and named reads, at most the
existing 32-level lineage scope. Once recorded, ordinary observations do not
request the prefix again. Optional read/transport failures retain the prior route.

Persistence uses a versioned origin envelope in the existing
`session_fork_lineage.inherited_items_json`. `saveAutomaticForkLineageRecord()` can
enrich empty metadata transactionally without rewriting a confirmed lineage tuple.
No memory body, vector, Host message ID or marker is rewritten by this operation.
Ordered-prefix association is based on the inspected official clone operations;
arbitrarily edited or unavailable old snapshots are not fully reconstructed.
Ordinary copy routing and explicit session-copy operations retain their existing
meaning; independent inherited-memory editing is separate work.

Evidence and upstream SHAs: [4.3 feedback work log](docs/archive-center-4.3-feedback-work-log.md).
Production Go/JS regressions and an isolated real MariaDB HTTP/storage roundtrip
passed. These do not establish that the patched artifact is loaded in either Host.

### 2026-09-06 Gemini 3.8 Flash medium checkpoint

**Implemented; loaded-host/provider verification open.**
`resolveGeminiThinkingLevelOptions()` and Go `proxyGeminiThinkingLevel()` now
recognize `gemini-3.8-flash` as supporting medium. Publisher/Critic settings expose
`none/low/medium/high`; native and gateway request owners retain selected medium.
Only the existing model lists changed; `none` and preset defaults remain unchanged.
Production JS and Go outbound-body regressions passed with mocked external
boundaries. See the [4.3 feedback work log](docs/archive-center-4.3-feedback-work-log.md).

## 2. System Purpose

**VERIFIED source/regression.** The active source identifies itself as Archive Center 4.3.0-test.18 source: a RisuAI plugin plus a Go HTTP service. It preserves the 4.1 request lifecycle and adds Go-owned priority selection plus a user-selectable turn-finalization policy. Already admitted source projections create request-local `PriorityFactSeed` units before final memory-section rendering; each unit carries its own text, relevance, source identity, and visibility/perspective lineage. Recalled `memories.turn_summary` values form a separate complete-summary group scored by the higher of their best child fact and observed aggregate-vector score. The UI core-memory maximum is applied independently to that group and each scored fact lane, while existing per-class character budgets and the final character envelope remain Go-owned. One Go-owned request query set—current continuity/input plus recent completed user/final-assistant conversation pairs up to UI `recent_conversation_reference_count`—is used by both Chroma retrieval and fact scoring. This conversation-reference depth is independent of Chroma result `top_k` and final per-group core-memory K. Aggregate Memory candidates remain bounded by the existing `tier=memory` recall result instead of all loaded session rows. In-scope canonical `precise_memory_units` determine the atomic scoring search count; their existing Chroma documents are queried by `source_table` with the same query vectors, then canonically hydrated from MariaDB so each similarity contributes only to its matching fact rather than every child of a parent Memory row. The plugin observes RisuAI lifecycle and Host coordinates, applies a backend-produced payload plan, and transports accepted finality; it does not calculate memory rank or own canonical persistence. The backend resolves the current input and session route, retrieves and assembles memory/context, produces the priority delivery/payload plan, optionally obtains a bounded `publisher_plan.v2`, validates completed-turn source lineage, writes through the selected Store, and maintains a derived Chroma search index. MariaDB remains canonical only in `mariadb_authority`; Chroma remains derived. Evidence: [plugin metadata](Archive%20Center.js), [`registerRisuLifecycleHooks()`](Archive%20Center.js), [`prepareTurnLoadGeneralPreciseMemoryUnits()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnRetrievalQueries()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnEffectiveContinuityQuery()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnHydratePreciseMemoryVectorFacts()`](go-service/internal/httpapi/prepare_turn_recall.go), [`appendPrepareTurnPriorityFactSeeds()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnBuildPriorityTurnSummaries()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`buildPrepareTurnPriorityMemoryDeliveryPlan()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), and [`handleCompleteTurn()`](go-service/internal/httpapi/group_turn_complete.go). Package and live-host/provider proof are separate.

**VERIFIED.** The system also exposes store-backed administration, memory explorer, session migration, narrative/persona, original-work reference-library, Host lorebook-reference, timeline/dashboard, status projection, source-discovery/canon-pack, and managed-update routes. These are registered by [`Server.RegisterRoutes()`](go-service/internal/httpapi/server.go#L203-L233). Their presence in source proves route implementation, not plugin invocation, production enablement, canonical-truth authority, or live data quality.

## 3. Current Architecture Summary

**VERIFIED.** Target boundary: `Archive Center.js` is intended to be the RisuAI host adapter. Go owns policy, selection, final budgets, prompt/lane text and ordering, persistence, ViewModels, and stable error decisions. MariaDB is the canonical product store in `mariadb_authority`. The same MariaDB implementation can also hold the deliberately separate, non-canonical Host lorebook observation ledger. Chroma is an optional, derived selector/index and never the authority for a memory or lorebook entry. External LLM and embedding providers are optional backend dependencies configured at runtime.

**VERIFIED.** Normal application path: a successful compact preparation returns `payload_application_plan.v1`; JavaScript validates its version/owner/apply rule, applies the exact Go-produced `auxiliary_text`, and records `payload_application_observation.v1`. The base auxiliary order is `original_work` → `long_term_memory` → `output_guidance`; `reference_assist` inserts `lorebook_reference` immediately before `output_guidance`. The plan intentionally carries an empty `input_context_text`: recent input remains internal to Go's Publisher/turn analysis and is not duplicated into the RisuAI payload. `applyContextInjection()` is active host mutation, not evidence that JavaScript owns lane contents.

**VERIFIED source/regression/package/backend-live; user-observed loaded-RisuAI Provider Manager and direct Vertex transport paths.** The opt-in PDF Preview changes only the representation of Go's already-selected `long_term_memory`. [`buildPrepareTurnMemoryTransport()`](go-service/internal/httpapi/prepare_turn_memory_transport.go) emits `memory_transport_plan.v1`; direct Google/Gateway modes also receive a current-response-only `memory_transport_payload.v1`, and [`pdfmemory.Generate()`](go-service/internal/pdfmemory/generator.go) creates their searchable A4 PDF with embedded Noto Sans KR. [`onMemoryTransportBodyInterceptor()`](Archive%20Center.js) applies Google `inlineData` or an OpenAI-compatible `file` block. The explicit `provider_manager_pdf` experiment instead applies one `<pm-pdf>` range to the selected lane in the Risu payload and lets Yumi Provider Manager create the PDF, without editing that plugin or building a second Go PDF. Both representations remove the same selected memory from the ordinary auxiliary Text while preserving the other lanes and the request-owned retry lifecycle. The existing `payload_application_plan.v1`, selection/order/budgets and complete-turn persistence remain unchanged. A packaged real MariaDB session produced identical direct-mode logical content and valid PDFs. In loaded RisuAI, Provider Manager converted one marker range to `application/pdf` and reached a model response; a direct Vertex Gemini request entered the body interceptor, applied `google_pdf`, and removed one matching Text block. Google AI Studio, LLM Gateway, total provider-side PDF block count, live semantic-recall quality and usage comparison remain **UNKNOWN**.

**VERIFIED source/regression.** The inspected RisuAI request loop can invoke `beforeRequest(messages, type)` again for a retryable provider attempt of the same logical turn, while `afterRequest(content, type)` is invoked only after a successful response. The callback still exposes neither a Host request ID nor a retry cause. Archive Center therefore reuses an existing prepared context only when the session/Host-chat coordinates, user message index/ordinal/chat ID/time/content/hash, request message count, baseline assistant generation/time/content hash, and request type all match exactly and the original Go payload plan was already marked replay-safe. That path preserves the Archive request ID, `/prepare-turn` result, payload plan, persistence owner, and HUD stage; it does not rerun preparation, Publisher/search, raw-input binding, or create a new HUD. Exact auxiliary blocks are reused, exact duplicates are collapsed to one, and a conflicting Archive block remains ambiguous without overwrite. Loaded-RisuAI behavior remains **UNKNOWN** until the generated test package is imported and traced.

**VERIFIED.** Current exceptions: the thin-adapter migration remains incomplete. Active JavaScript still calculates local turn observations with `nextTurnIndex()`, derives budget observations with `estimateAdaptiveInjectionBudgetParts()`, chooses the host insertion position with `resolveAuxiliaryInjectionPlacement()`, and can execute legacy orchestration/read/trace work after an eligible full response when a compatible compact plan is unavailable. Transport failure or an ineligible source decision returns the original payload before that legacy path. `applyProtectionOnlyInjection()` remains a JavaScript-owned exception only for the intentional-orchestration-skip branch. Rollback policy is no longer decided by a `checkAndAutoRollback()` helper: JavaScript transports observed host deletion, while Go validates route ownership, complete assistant observations, deletion evidence, token/digest/revision fences, and the delete range. These remaining adapter calculations and the protection-only path are boundary debts. Evidence: [`onBeforeRequest()`](Archive%20Center.js), [`reconcileActiveChatTailDeletionWithBackend()`](Archive%20Center.js), [`handleRollbackDecision()`](go-service/internal/httpapi/group_turn_range_decision.go), and [`applyProtectionOnlyInjection()`](Archive%20Center.js).

**OBSOLETE.** The deleted legacy `assembleInjectionWithBudget()` JavaScript assembly surface is not active evidence. The Go-owned payload application plan remains the active injection budget and assembly owner. `extractMemoryItems()` is called only to build UI/input-transparency inspection data, not to select final delivered memory.

**VERIFIED.** The removed 4.0.7 `replacement_pending` experiment is not part of the active contract. The current rollback decision starts blocked and restores the 4.0.2 exact-prefix safety shape: automatic deletion requires the Host assistant observations to match the durable active sources in Risu message-index order, with only a contiguous missing suffix admitted as deletion evidence. A middle revision/content mismatch is `historical_revision_conflict` and performs no canonical mutation. A separately authorized manual candidate remains distinct; admitted rollback binds a one-use token to route revision and assistant-observation digest and performs canonical tail invalidation before queueing derived-vector cleanup. Post-output replacement remains an explicit `superseded` lifecycle, distinct from deletion. Source-level behavior is verified; loaded-host and live-database behavior remain **UNKNOWN**.

**VERIFIED.** Complete-turn durable logical identity uses the stable observed Risu user-message `chatId` within the existing session, Host-chat, and branch scope. Editing or rerolling the same Host user row therefore replaces its logical turn even when text, timestamp, or other observations change; a genuinely new Host user row appends even when its text is identical. When that user-message `chatId` is unavailable, the existing message-index/time/content fallback remains. A non-committed replacement failure restores the previous active source state; a nonretryable failure such as `logical_turn_not_current_tail` also records a terminal revision failure so a new idempotency key cannot re-enter replacement. The MariaDB current-tail transaction check remains unchanged. These are source/regression claims; real MariaDB behavior remains **UNKNOWN**.

**VERIFIED.** Go emits the observation-only `memory_injection_baseline.v1` for the nine 4.1 surfaces and keeps `candidate`, `selected`, `rendered`, `payload-applied`, and `displayed effect` as separate stages. It reports counts, estimated tokens, provenance/order, same-row lane duplication, cross-surface normalized-text duplicate candidates, and current-input/recent-context duplicate candidates without suppressing anything or mutating canonical/vector state. JavaScript only reports whether the exact planned payload block was applied; displayed effect remains `unobserved`. Package, loaded-RisuAI, provider, actual-payload, and displayed-final evidence remain **UNKNOWN**.

**VERIFIED source/regression (4.2-G).** Archive Center exposes one `저장 확정 시점` setting. `응답 직후` is the unchanged default. With explicit `다음 사용자 입력 시`, `afterRequest` stores only a minimal pending Host marker; the next genuinely new user row starts the previous final pair through the existing `/complete-turn` path without awaiting its Critic. A same-row reroll/edit replaces the marker, an identical-text new row still advances, a different branch does not consume it, and restart restores it. Go confirms the mode through `turn_finalization_policy.v1`, and `orchestrateTurnHelpers()` now preserves that request-owned policy in both compact and legacy results consumed by `onAfterRequest()`; previously the policy was dropped at this handoff and the adapter silently used the immediate default. JavaScript falls back to the 4.1 immediate behavior only when the Go policy is actually absent. A workflow created in next-input mode uses `beginForNextInputFinalization()` and records that single Go-confirmed timing on its own ledger entry, so the following request does not incorrectly invalidate that still-running previous ledger as `superseded_by_new_request`, even if the user changes the setting before the following input. An ordinary immediate-mode previous entry retains the existing same-session supersession behavior. The UI projects the current request's backend stages 1–6 as `현재 턴 생성 1/6~6/6` and the previous request's stages 7–12 as `직전 턴 평론가·저장 1/6~6/6`. After an accepted delayed response, the finished current-generation card closes while its pending marker remains available for the next input; a terminal previous card can be dismissed without clearing a still-running current card. One additional request-scoped event stream renders the exact previous `/complete-turn` workflow below the current card; the split is presentation only and does not create a second stage ledger, persistence owner, or Critic. A missing/mismatched previous row leaves the marker pending while the current request continues, so it is not an output rejection gate. No second persistence API, Critic, scheduler, hidden retry, broad chat sweep, or shutdown/session-switch auto-save was added. Loaded RisuAI has shown both concurrent cards and real previous-turn completion; the latest close/dismiss correction still requires re-import verification. See [`archive-center-4.2-work-log.md`](docs/archive-center-4.2-work-log.md).

```mermaid
flowchart LR
    U["User"] --> R["RisuAI"]
    R --> J["Archive Center.js\nHost adapter"]
    J -->|"HTTP contracts"| G["Go HTTP service\nPolicy and orchestration"]
    G -->|"canonical Store reads/writes"| M[("MariaDB canonical records")]
    G -->|"separate LorebookReferenceStore"| L[("MariaDB Host-reference ledger")]
    G -->|"semantic selector / direct derived mutations"| C[("Chroma index")]
    G -->|"Publisher, Critic, embeddings"| P["Optional providers"]
    G --> W["Reprocessing and vector workers"]
    W --> M
    W --> C
    G -->|"payload_application_plan.v1 text/order"| J
    J -->|"host placement + exceptional protection fallback"| J
    J -->|"mutated request / displayed output"| R
```

**VERIFIED.** The diagram describes source ownership, not deployment topology. The two MariaDB nodes are logical authority boundaries, not necessarily different servers. The Host-reference interface is exposed by the direct MariaDB authority Store, not the noop/fixture/dual-write/read-only wrappers. Chroma can be off or provided through several runtime profiles.

## 4. Repository Map

```text
source/
├── Archive Center.js              active RisuAI adapter source (4.3.0-test.1 local test)
├── go-service/                    active Go backend source
│   ├── cmd/                       service, package, operator, audit, and smoke executables
│   ├── internal/config/           environment parsing and mode validation
│   ├── internal/dto/              versioned/request DTOs
│   ├── internal/httpapi/          routes, policy, orchestration, workers
│   ├── internal/store/            store interfaces and MariaDB implementation
│   ├── internal/vector/           Chroma/fake vector clients
│   └── internal/packageupdate/    managed-update implementation
├── migrations/                    canonical fresh schema and additive migrations
├── prompts/                       active Critic and Supervisor prompt sources
├── ops/                           package builders, package templates, runbooks
├── scripts/                       developer/start/validation helpers
├── tools/                         installer and operational tooling
├── contracts/                     historical/frozen contract documentation
├── docs/                          architecture, audit, and roadmap documents
├── tests/ and testdata/           non-Go fixtures and test guidance
├── .github/                       CI workflows
├── install-windows.ps1, install.sh public installer entry points
├── _dist*, _release*, _test-builds/ generated/package/test outputs
├── .tmp*, .gocache/               generated caches and scratch data
├── Archive Center 3.4-C.js and Archive Center.js.codex-backup-* inactive copies
└── standalone Recomposer/quality-layer JS files     separate manual plugins
```

| Path | Responsibility and status | Main consumers |
| --- | --- | --- |
| [`Archive Center.js`](Archive%20Center.js) | **VERIFIED.** Active RisuAI hooks, backend transport, exact payload mutation, displayed-output handling, settings UI, and local transient/retry state. | RisuAI; package builders copy this file. |
| [`go-service/cmd/archive-center-go/main.go`](go-service/cmd/archive-center-go/main.go) | **VERIFIED.** Active backend executable startup. | Windows/POSIX packages and local runs. |
| [`go-service/internal/httpapi`](go-service/internal/httpapi) | **VERIFIED.** Active HTTP handlers, prepare/complete orchestration, retrieval, workers, runtime ViewModels. | Backend `main()` through `RegisterRoutes()`/`StartMemoryWorkers()`, plus tests. |
| [`go-service/internal/store`](go-service/internal/store) | **VERIFIED.** Active Store contracts plus MariaDB/noop/fixture implementations. | HTTP policy and workers. |
| [`go-service/internal/vector`](go-service/internal/vector) | **VERIFIED.** Active Chroma and fake vector adapters. | Retrieval, vector outbox worker, and direct projection/admin/migration/reference index paths. |
| [`migrations`](migrations) | **VERIFIED.** The directory contains `001_schema.sql` plus numbered migrations through `013_precise_memory_text_fields.sql`. `013` widens subtype, relationship key and reveal condition to LONGTEXT without changing identity indexes. New and upgraded installations use the same sorted complete inventory; later tables such as the four lorebook-reference tables remain owned by `010` and need not be duplicated into `001`. The updater accepts managed migration files that are added, changed, or removed. | `cmd/mariadb-schema`, installers, updater, package builders. |
| [`go-service/internal/httpapi/group_lorebook_reference.go`](go-service/internal/httpapi/group_lorebook_reference.go), [`prepare_turn_lorebook_reference.go`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go), [`go-service/internal/store/lorebook_reference.go`](go-service/internal/store/lorebook_reference.go), [`mariadb_lorebook_reference.go`](go-service/internal/store/mariadb_lorebook_reference.go) | **VERIFIED.** Active Host lorebook snapshot route, non-canonical Store contract, lifecycle, exact/key/lexical search, budget, and optional lane delivery. | `RegisterRoutes()`, `tryPrepareTurn()`, direct MariaDB authority Store. |
| [`prompts/critic_system.txt`](prompts/critic_system.txt), [`prompts/supervisor_system.txt`](prompts/supervisor_system.txt) | **VERIFIED.** Backend prompt inputs. Files containing `복사본` or backup names are not active defaults. | Go prompt loader and package builders. |
| [`archive-center-runtime.test.cjs`](archive-center-runtime.test.cjs) | **VERIFIED** test-only manual harness; **OBSOLETE** as active-runtime evidence. It is not imported by the plugin/backend and is not selected by current CI/core-regression scripts. | Explicit manual Node invocation only. |
| [`ops/build-release-assets.ps1`](ops/build-release-assets.ps1), [`ops/build-full-package.ps1`](ops/build-full-package.ps1), [`ops/build-posix-managed-packages.ps1`](ops/build-posix-managed-packages.ps1) | **VERIFIED.** Source tooling that builds binaries, copies plugin/schema/prompts/templates, writes manifests, and assembles the seven public ZIPs plus one aggregate SHA-256 list under `_release-builds/<version>`. | Release/package operators. |
| [`contracts`](contracts) | **INFERRED.** Reference material useful for compatibility history; no active service import makes it runtime authority. | Maintainers and tests that intentionally freeze contracts. |
| [`docs`](docs) | Documentation, audits, version work logs, and future roadmaps. [`archive-center-4.0.8-work-log.md`](docs/archive-center-4.0.8-work-log.md) and [`archive-center-4.0.8-to-4.0.9-work-log.md`](docs/archive-center-4.0.8-to-4.0.9-work-log.md) are change/evidence indexes; later entries can supersede earlier entries. A claim found only in documentation is **PLANNED**, **OBSOLETE**, or **UNKNOWN**, not implementation proof. | Maintainers and auditors. |
| `_dist*`, `_release*`, `_test-builds`, `_runtime*`, `.tmp*`, `.gocache` | **OBSOLETE.** Generated, packaged, cached, or test output as active source; some are dirty/untracked. | Build/test tools only. |
| `Archive Center 3.4-C.js`, `Archive Center.js.codex-backup-*` | **OBSOLETE.** Copies relative to the active 4.1.0 source. They can run only if someone separately installs them. | No active source import was found. |
| `AC Recomposer Agent.js` | **VERIFIED.** Optional separately installed consumer of the currently implemented `archive_center.recomposer_bridge.v1` contract; it is not copied or auto-loaded by Archive Center package builders. Its Recomposer product identity is historical and is not the current `AC Ensemble Agent` product identity. | Manual installation only; live installation is **UNKNOWN**. Any approved replacement/removal requires an explicit versioned migration and removal condition. |
| `Risu Output Quality Layer 2.5.js`, `Risu Recomposer - 복사본.js` | **INFERRED.** Standalone/legacy plugins with no active package-copy or import path found. | Separate manual installation, if any. |

## 5. Runtime and Explicit Tool Entry Points

| Component | Entry file | Entry symbol | Activation method | Responsibility | Evidence | Status |
| --- | --- | --- | --- | --- | --- | --- |
| RisuAI adapter | `Archive Center.js` | async IIFE and `init()` | Plugin load evaluates the file; bottom-level `await init()` | Register hooks/UI, restore local state, sync backend config | [`init()` and call](Archive%20Center.js) | VERIFIED |
| RisuAI input observation | `Archive Center.js` | `onInputHook()` | `addRisuScriptHandler("input", ...)` | Cache raw input and observe the Risu history-trim command; do not inject | [`onInputHook()` and registration](Archive%20Center.js) | VERIFIED |
| Pre-model adapter | `Archive Center.js` | `onBeforeRequest()` | `addRisuReplacer("beforeRequest", ...)` | Capture immutable request/finality coordinates, make source-decision/full `/prepare-turn` calls, attach the request's orchestration result, and apply the backend plan to the writable payload | [`onBeforeRequest()`](Archive%20Center.js), [`captureFinalConfirmationRequestContext()`](Archive%20Center.js), [`applyGoPayloadApplicationPlan()`](Archive%20Center.js) | VERIFIED |
| Yumi Translator 1.4.2 read compatibility | `Archive Center.js` | `getCurrentActiveChatSourceObservationMessages()`, `buildYumiV1ArchiveReadContext()` | The existing `beforeRequest` path observes an active-chat assistant message containing a complete `yumi-tr:v1` marker range | Build a non-mutating Archive-only message copy from Yumi's matching `$__yumi_tr.<id>` `scriptstate` record so Publisher, continuity, language trace, and `/prepare-turn` read the model original while Risu display and outbound payload keep their existing behavior. Plain JSON, `u:` JSON, and `z:` gzip records are supported; missing/malformed metadata retains the visible translation text without blocking the turn. | [`buildYumiV1ArchiveReadContext()`](Archive%20Center.js), [`onBeforeRequest()`](Archive%20Center.js) | VERIFIED source/regression against the inspected Yumi Translator 1.4.2 reference; loaded plugin order/behavior UNKNOWN |
| PDF transport adapters | `Archive Center.js` | `applyProviderManagerMemoryPDFPayload()`, `registerMemoryTransportBodyInterceptor()`, `onMemoryTransportBodyInterceptor()` | Provider Manager marker mode runs inside the existing `beforeRequest` payload application; direct modes use `Risuai.registerBodyIntercepter(...)` during `init()` and unregister during unload | Replace only the Go-selected long-term-memory Text with one explicit Provider Manager manual PDF range or one provider-specific document block; observe application without deciding selection or persistence | [`applyGoPayloadApplicationPlan()`](Archive%20Center.js), [`applyProviderManagerMemoryPDFPayload()`](Archive%20Center.js), [`onMemoryTransportBodyInterceptor()`](Archive%20Center.js) | VERIFIED source/regression/package/backend-readiness; loaded Host/provider UNKNOWN |
| Priority Memory (4.3 static.v4) | `go-service/internal/httpapi/prepare_turn_priority_memory.go` | `appendPrepareTurnPriorityFactSeeds()`, `prepareTurnBuildPriorityTurnSummaries()`, `buildPrepareTurnPriorityMemoryDeliveryPlan()` | Priority-enabled auto/custom-budget `/prepare-turn` assembly after existing source eligibility/projection | Form request-local atomic facts before final section rendering; consume the same effective continuity query as Chroma; apply a canonically hydrated precise-unit vector score only to its matching fact; retain per-fact score/source/visibility/perspective lineage; use small speaker/location/storyline rank biases, RP-turn-distance recency, and independently weighted stored importance; keep identity metadata outside fact K; use an observed aggregate-vector score for its complete summary without copying it into sibling fact relevance; keep lifecycle diagnostic-only; resolve non-lifecycle current identity per fact while retaining source occurrence for facts whose structured path contains an array ordinal such as `item_1`; score each recalled complete turn summary by the higher of its best child fact and observed aggregate-vector score; apply the same K independently to the complete-summary group and each scored fact lane without slot transfer; apply existing per-class and final character budgets; and render `memory_delivery_plan.v2`. An unseeded source remains readable through `legacy_rendered_line` fallback. | [`prepareTurnEffectiveContinuityQuery()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnHydratePreciseMemoryVectorFacts()`](go-service/internal/httpapi/prepare_turn_recall.go), [`appendPrepareTurnPriorityFactSeeds()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnBuildPriorityCandidates()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnBuildPriorityTurnSummaries()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`buildPrepareTurnPriorityMemoryDeliveryPlan()`](go-service/internal/httpapi/prepare_turn_priority_memory.go) | VERIFIED source/regression/package; loaded live quality UNKNOWN |
| Selectable turn finalization | `Archive Center.js`, `group_turn_prepare.go` | `onAfterRequest()`, `maybeStartNextInputFinalizationPipeline()`, `buildTurnFinalizationPolicy()` | Go-confirmed setting selects the default immediate path or the explicit next-user-input path | Preserve immediate 4.1 completion or carry the exact previous Host row to the same non-blocking `/complete-turn` owner on the next new user row | [`onAfterRequest()`](Archive%20Center.js), [`buildTurnFinalizationPolicy()`](go-service/internal/httpapi/prepare_turn_priority_memory.go) | VERIFIED source/regression; loaded Host/real DB UNKNOWN |
| Optional Host lorebook sync | `Archive Center.js` | `observeLorebookReferenceScope()`, `syncCurrentLorebookReference()`, `postLorebookReferenceSnapshot()` | `tryPrepareTurn()` when mode is not `off`, manual refresh, scope/settings change | Observe the official Host API and transport one scoped snapshot; do not decide recall/delivery | [`tryPrepareTurn()`](Archive%20Center.js#L15735), [`syncCurrentLorebookReference()`](Archive%20Center.js#L15661) | VERIFIED source; UNKNOWN loaded Host |
| Host output observation | `Archive Center.js` | `onRisuOutput()` | `addRisuChatListener("output", ...)` | Observe bounded `branchedfrom` markers, freeze stable host coordinates, and transport `risu_worldline_observation.v2`; never accepts finality or completes a turn | [`onRisuOutput()` and registration](Archive%20Center.js) | VERIFIED source; UNKNOWN loaded Host |
| Post-model adapter | `Archive Center.js` | `onAfterRequest()` | `addRisuReplacer("afterRequest", ...)` | Atomically detach the exact context captured at `beforeRequest`, normalize visible text, accept official finality, queue async `/complete-turn`, and return display text without waiting | [`onAfterRequest()`](Archive%20Center.js), [`continueAcceptedFinalPersistence()`](Archive%20Center.js) | VERIFIED |
| Backend service | `go-service/cmd/archive-center-go/main.go` | `main()` | Built executable or `go run` | Load/validate config, build server, preflight dependencies, start workers/routes, serve HTTP | [`main()`](go-service/cmd/archive-center-go/main.go#L23-L116) | VERIFIED |
| Schema tool | `go-service/cmd/mariadb-schema/main.go` | `main()` | Windows/POSIX package launchers or an operator invoke it with `--execute` and a DSN | Apply fresh schema and additive compatibility statements; it is not called by the HTTP service startup | [`main()`](go-service/cmd/mariadb-schema/main.go#L96), [Windows launcher](ops/full-package/scripts/start-full-windows.ps1#L1358), [POSIX launcher](ops/full-package-posix/start-full-posix.sh#L481) | VERIFIED |
| Managed updater | `go-service/cmd/archive-center-updater/main.go` | `main()` | Managed package launchers copy/invoke a recovery runner; `/update/apply` only stages the request and asks an authorized service to exit | Verify/apply/commit/rollback managed package state | [`main()`](go-service/cmd/archive-center-updater/main.go#L23), [Windows launcher](ops/full-package/scripts/start-full-windows.ps1#L984-L1091), [POSIX launcher](ops/full-package-posix/start-full-posix.sh#L305-L340), [`handleUpdateApply()`](go-service/internal/httpapi/group_update.go#L243) | VERIFIED |
| Public fresh installers | `install-windows.ps1`, `install.sh`, `scripts/install-github-release.ps1`, `scripts/install-github-release.sh` | public bootstrap plus release helper | New users run one fixed public command; direct ZIP users enter the Windows package through `01_start_archive_center_windows.bat` | Select the current OS/CPU release asset, verify its exact SHA-256 record before extraction, preserve an install-level data root, and start the platform launcher | [`install-windows.ps1`](install-windows.ps1), [`install.sh`](install.sh), [`install-github-release.ps1`](scripts/install-github-release.ps1), [`install-github-release.sh`](scripts/install-github-release.sh) | VERIFIED production-entrypoint contract CI on Windows/Ubuntu/macOS; 4.2 assets published; full native runtime installation remains separately scoped |
| Windows package launcher | `ops/full-package/01_start_archive_center_windows.bat`, `scripts/start-full-windows.ps1`, `scripts/windows-console-control.ps1` | BAT entry plus PowerShell process-lifetime functions | User launches the public BAT; PowerShell starts and owns the managed process group | Start/update/recover the Windows package; isolate managed children from Ctrl+C, confirm `N`/`Y`, then perform bounded cleanup only after confirmation or parent/launcher loss | [`Start-ArchiveChildProcess()`](ops/full-package/scripts/start-full-windows.ps1), [`Wait-ArchiveBackendLifetime()`](ops/full-package/scripts/start-full-windows.ps1), [`Wait-ArchiveProcessWithCtrlCConfirmation()`](ops/full-package/scripts/windows-console-control.ps1) | VERIFIED source/process/package-live regression |
| Import/migration operators | `go-service/cmd/mariadb-import`, `go-service/cmd/legacy10-migrate` | `main()` | Explicit manual/tool invocation with `--execute` and a DSN | Directly populate MariaDB from validated legacy/export inputs; not mounted service entry points or packaged runtime binaries | [`mariadb-import`](go-service/cmd/mariadb-import/main.go#L210), [`legacy10-migrate`](go-service/cmd/legacy10-migrate/main.go#L70) | VERIFIED |
| Package builders | `ops/*.ps1` | script entry | Operator invocation | Compile Go tools and copy active source payloads to generated output | [`build-full-package.ps1`](ops/build-full-package.ps1#L446-L704), [`build-posix-managed-packages.ps1`](ops/build-posix-managed-packages.ps1#L306-L516) | VERIFIED |

**VERIFIED.** Only the adapter IIFE/hooks and `archive-center-go` are normal application-runtime entries. `mariadb-schema` and `archive-center-updater` are launcher/operator entries. Import, migration, builder, audit, and smoke commands execute only when explicitly invoked and must not be described as active service paths.

## 6. Component Responsibility Matrix

| Component | Owns | Must not own | Inputs | Outputs | Persistent side effects | Relevant paths |
| --- | --- | --- | --- | --- | --- | --- |
| RisuAI adapter | Host lifecycle, branch/lorebook/deletion observations, transport, actual payload mutation, displayed-output replacement, DOM/localization, local retry/UI state | Ranking, canonical turn/range or rollback calculation, memory/reference selection, final budgets, prompt prose, persistence policy, ViewModel composition | Risu callbacks, host chat/lorebook snapshots, settings, Go responses | Versioned host observations, HTTP requests, mutated Risu payload, visible status | Plugin storage for settings/queues; no direct DB writes. **VERIFIED:** debt remains in local turn/budget observations, placement, legacy compatibility work, and exceptional protection prose. | [`Archive Center.js`](Archive%20Center.js), [boundary](docs/permanent-risu-host-backend-boundary.md) |
| Go HTTP/policy layer | Validation, current-input/source decisions, canonical and lorebook-reference retrieval, eligibility, dedupe, ranking, budgets, prompt assembly, Publisher/Critic orchestration, stable reason codes | Risu DOM, native lorebook mutation/activation, or direct host payload mechanics | DTOs, canonical/reference reads, vector candidates, runtime config | Versioned plans/ViewModels, persistence commands | Through Store/vector interfaces only | [`internal/httpapi`](go-service/internal/httpapi), [`internal/dto`](go-service/internal/dto) |
| MariaDB store | Canonical raw and structured records, transaction/fence/idempotency enforcement | Semantic relevance policy or UI | Store calls and transactions | Canonical rows, job/outbox rows | Yes; authoritative when `mariadb_authority` is selected. Transaction scope is per store operation, not automatically the whole turn pipeline. | [`internal/store`](go-service/internal/store), [`mariadb_memory_admission.go`](go-service/internal/store/mariadb_memory_admission.go) |
| Host lorebook reference store | Exact Host-scope snapshot ledger and current entry projection | Canonical memory/evidence/entity/relationship/world truth, native Host activation, or Chroma indexing | `lorebook_reference_snapshot.v1` observations | `lorebook_reference_recall.v1` candidates/optional delivered reference text | Yes, in a separate MariaDB transaction and separate `LorebookReferenceStore`; exposed only by direct MariaDB authority in the current factory | [`LorebookReferenceStore`](go-service/internal/store/lorebook_reference.go#L107), [`ApplyLorebookReferenceSnapshot()`](go-service/internal/store/mariadb_lorebook_reference.go) |
| Worldline resolver, retrieval scope, and store | Go validates bounded host branch observations, parent route/source revision, conflicts, and backfill boundary; composes confirmed parent history only through each fork boundary; MariaDB stores route/fork lineage | Treat a marker alone as narrative truth, read parent post-fork or sibling history, rewrite accepted parent history, or let JavaScript choose canonical lineage | `risu_worldline_observation.v2`, stable host coordinates, source history | `session-routing.turn-resolution.v1`, `session_fork_lineage.v2`, bounded prepare-turn history segments, topology ViewModels | Session route/fork-lineage rows; these are lineage/ownership records, not narrative canonical truth | [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go), [`resolvePrepareTurnHistoryScope()`](go-service/internal/httpapi/group_turn_prepare.go#L31), [`prepareTurnVectorShadow()`](go-service/internal/httpapi/prepare_turn_recall.go), [`group_step23_fork_lineage.go`](go-service/internal/httpapi/group_step23_fork_lineage.go) |
| Manual entity-identity routes | Go previews and applies explicitly reviewed character/item equivalence links | Infer equivalence from a label, rewrite stored source rows, invoke Critic, or imply an atomic multi-link batch | Operator-selected source IDs and expected canonical identity | `character_identity_manual_merge.v1` or `item_identity_manual_merge.v1` results | Individual reviewed/revoked `entity_identity_links`; sequential writes can partially succeed | [`group_characters.go`](go-service/internal/httpapi/group_characters.go), [`group_items.go`](go-service/internal/httpapi/group_items.go) |
| Chroma adapter | Derived vector document storage and semantic candidate selection | Canonical truth, source acceptance, final eligibility | Embeddings/documents and scoped queries | Candidate IDs/scores; exact-query/list/delete capabilities where the concrete store supports them | Derived index only. It is mutated both by the core-memory outbox worker and by explicitly coded direct index-maintenance/projection paths. | [`internal/vector`](go-service/internal/vector), [`prepare_turn_recall.go`](go-service/internal/httpapi/prepare_turn_recall.go), [`turn_extraction_vector.go`](go-service/internal/httpapi/turn_extraction_vector.go) |
| Reprocessing worker | Retry Critic/derived admission for an accepted source revision | Change raw accepted turn text | Leased MariaDB job, runtime provider config, configured Critic reprocessing interval, and a longer provider `Retry-After`/`retry_after` hint when present | Core admission plus the same separate post-admission projection writes, or retry/permanent state | Job states, durable next retry time restored into one-shot worker wake after restart, provider termination/token diagnostics, core structured memory, typed projections | [`memory_reprocessing_worker.go`](go-service/internal/httpapi/memory_reprocessing_worker.go), [`mariadb_memory_derivation.go`](go-service/internal/store/mariadb_memory_derivation.go), [`proxy_provider.go`](go-service/internal/httpapi/proxy_provider.go), [`saveCriticExtractionArtifacts()`](go-service/internal/httpapi/turn_extraction_persist.go#L97) |
| Vector outbox worker | Materialize embeddings, upsert/delete Chroma, exact-readback verification, retry/compensation for outbox-managed documents | Make Chroma authoritative or imply that it covers every vector mutation | Leased outbox rows and provider config | Verified index mutation and canonical outbox completion | Chroma plus MariaDB outbox status; world-rule/status/admin/migration/reference and compatibility helpers can bypass this worker | [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go#L37-L312), [`group_reference_vectors.go`](go-service/internal/httpapi/group_reference_vectors.go#L366-L515) |
| Publisher/Supervisor provider | Return one source-backed, response-scoped `publisher_plan.v2` object containing the required `book_author` and `director` role shapes; either role can have no accepted item | Persist truth, invent defaults/facts, force user action/relationship change/event closure, or directly mutate the main payload | `response_execution_contract.v1` and `supervisor_support_packet.v2` | `supervisor_scene_proposal.v3` carrying `publisher_plan.v2` | None directly; exactly one provider request per Publisher run | [`runSupervisorLLM()`](go-service/internal/httpapi/group_proxy.go#L220), [`buildBoundedSupervisorResult()`](go-service/internal/httpapi/group_proxy.go#L835), [prepare consumer](go-service/internal/httpapi/group_turn_prepare.go) |
| Critic/extractor provider | Propose structured artifacts from an accepted raw turn | Declare canonical facts without backend validation/admission | Go-built Critic input snapshot | Parsed extraction proposal | None directly | [`turn_extraction_critic.go`](go-service/internal/httpapi/turn_extraction_critic.go), [`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go#L670-L778) |
| Build/update tooling | Produce checksummed/manifested packages from active sources | Become implementation source of truth | Source tree and toolchain | Binaries, copied plugin/schema/prompts, manifests | Generated `_dist`/package contents | [`ops`](ops), [`internal/packageupdate`](go-service/internal/packageupdate) |

## 7. Source-of-Truth Matrix

| Item | Canonicality | Source of truth / owner | Derived or consuming forms | Notes |
| --- | --- | --- | --- | --- |
| Plugin implementation | Canonical source | [`Archive Center.js`](Archive%20Center.js) | Copied plugin in package directories | Package copies are generated. |
| Backend implementation | Canonical source | [`go-service`](go-service) | Compiled executables | Binaries are not editable source. |
| Fresh/upgrade DB schema | Canonical source inputs | The complete sorted [`migrations`](migrations) set through `013_precise_memory_text_fields.sql`, interpreted by the schema loader | Installed MariaDB tables | New and upgraded installations run the resulting complete inventory. Managed migration files may be added, changed, or removed by an update; later contents do not need to be copied back into `001_schema.sql`. |
| Generated artifacts | Generated | Package/build scripts | `_dist*`, `_release*`, `_test-builds` | Rebuild; do not patch in place. |
| Package manifests | Generated release evidence | Build scripts | `PACKAGE_FILE_MANIFEST.json`, `PACKAGE_MIGRATION_UPDATE.json`, release status | Existence does not prove a package was tested or released. |
| Configuration defaults | Canonical code defaults | [`config.Default()`](go-service/internal/config/config.go#L172-L206) and plugin defaults | Environment, package-rewritten examples, runtime `/config/update` snapshot | Runtime provider config is memory-only; response says `persisted: false`. |
| Raw turn evidence | Canonical runtime record | MariaDB chat logs plus accepted source revision | Critic input, derived memories, audit rows | Assistant text becomes raw canonical evidence only after host/source acceptance and durable save. |
| Effective input | Canonical runtime record | MariaDB effective-input row | Retrieval query/trace | Not interchangeable with arbitrary payload text. |
| Source acceptance/revision | Canonical lifecycle fence | MariaDB `memory_source_revisions` | Worker and vector eligibility | The JavaScript and Go source-acceptance ledgers are correlation/decision state; the durable canonical fence is the MariaDB revision row. Superseded/inactive revisions must not feed current recall. |
| Core admitted memory/evidence/precise units | Canonical accepted derived records | MariaDB `CommitMemoryAdmission()` | Prepare-turn candidates and vector-outbox work | One source-fenced transaction includes the memory row, reconciled direct evidence, precise units, outbox rows, and admission state. |
| Turn-derived KG, subjective/narrative/character/status and other typed projections | Canonical stored projections, subject to each contract's truth flags | Specialized MariaDB Store writers | Prepare-turn inputs, dashboards, maintenance | Written after core admission by separate operations; they are not part of the common-admission atomic transaction. |
| Direct route projections, reference/canon/discovery, and Step-23 records | Persistent MariaDB product records with contract-specific authority | Their route-specific Store writers | UI/read models, reference recall, validation | They can be written without common admission. Step-23 responses explicitly say those auxiliary records are not canonical truth writers. |
| Vector embeddings/documents | Derived index | Chroma. Core memory/direct-evidence/precise admission enqueues MariaDB outbox rows; world-rule extraction, status-schema indexing, admin reindex/orphan cleanup, session migration/rollback/delete, explorer deletion, reference reindex, and non-common-admission compatibility helpers can mutate the relevant Chroma collection directly. | Semantic candidate IDs/scores | MariaDB hydration and active-revision filters re-establish authority. Outbox retry/exact-readback guarantees apply only to outbox-managed documents. |
| In-process caches/ledgers | Temporary/cache | Go server and plugin process | Status/debug/UI | Loss must not redefine canonical history. |
| Plugin storage | Persistent host-local adapter state | Risu plugin storage/local storage | Settings, queues, counters, confirmation observations | Not canonical memory policy. |
| Host lorebook source | Host-owned observed source | RisuAI/PocketRisu official current-scope APIs | MariaDB snapshot ledger/current projection and optional reference lane | Archive Center does not mutate Host lorebook content or infer unobserved scope values. Stored copies remain non-canonical reference data. |
| Host lorebook snapshot/current projection | Persistent non-canonical reference | `LorebookReferenceStore` in direct MariaDB authority mode | `lorebook_reference_recall.v1`, optional Publisher support marked `reference_only` | It is neither the original-work reference library nor a Chroma collection and cannot satisfy canonical `Store`. |
| Session route and fork lineage | Persistent ownership/lineage state, not narrative truth | MariaDB route binding plus `session_fork_lineage.v2` written by the Go resolver/manual repair route | Timeline topology, session routing, and confirmed prepare-turn history scoping | A valid Host branch observation can create lineage. Confirmed lineage permits only ancestor history through the fork boundary plus each descendant-owned segment; it does not create character, relationship, or story facts. |
| Manual character/item equivalence | Reviewed canonical-equivalence link | MariaDB `entity_identity_links` via the Go manual merge/unmerge routes | Canonicalized character/item read models | Preview is read-only. Merge/unmerge links identities without rewriting source rows or automatically reindexing. |
| Documentation | Descriptive or normative | Current source and actual call graph are authoritative for implemented behavior; explicit architecture can constrain future changes without proving implementation | Roadmaps/contracts | Classify current proposals as **PLANNED**, superseded proposals as **OBSOLETE**, and conflicting roadmap intent as **UNKNOWN**. |

## 8. Startup and Initialization Flow

```mermaid
flowchart TD
    A["archive-center-go main"] --> B["config.Load and Validate"]
    B --> C{"live or cutover?"}
    C -->|"yes"| D["require authority mode and DSN"]
    C -->|"no"| E["continue selected mode"]
    D --> F["httpapi.NewServer"]
    E --> F
    F --> G["Open store adapter"]
    F --> H["Create Chroma or fake vectors"]
    G --> I["ValidateRuntimeDependencies"]
    H --> I
    I --> J["StartMemoryWorkers"]
    J --> K["RegisterRoutes and middleware"]
    K --> L["ListenAndServe"]
    L --> M["GET /ready on demand"]
```

1. **VERIFIED:** [`main()`](go-service/cmd/archive-center-go/main.go#L23-L116) loads and validates environment-derived configuration. Live/cutover has an additional guard requiring MariaDB authority configuration.
2. **VERIFIED:** [`NewServer()`](go-service/internal/httpapi/server.go#L84-L172) selects noop/fixture/MariaDB stores and Chroma/fake vector adapters. A reference vector collection is separate and its failure is designed to be non-blocking.
3. **VERIFIED:** [`ValidateRuntimeDependencies()`](go-service/internal/httpapi/server.go#L60-L80) blocks on configured main Chroma incompatibility/unavailability and, in authority mode, a store-construction error. It does not perform a MariaDB `Ping()`; `sql.Open()` alone does not establish connectivity.
4. **VERIFIED:** [`StartMemoryWorkers()`](go-service/internal/httpapi/memory_reprocessing_worker.go#L42-L68) starts only when the store advertises source lifecycle, job, outbox, and common-admission capabilities. Processing also waits for a synced runtime provider configuration.
5. **VERIFIED:** [`RegisterRoutes()`](go-service/internal/httpapi/server.go#L203-L233) mounts every active route group, including `registerLorebookReferenceRoutes()`, on a private sub-mux, constructs the handler as `CORS(auth(reverse-proxy-base-path(sub-mux)))`, and mounts it at `/` on the main mux before `ListenAndServe`. Request-side execution is therefore CORS → auth → reverse-proxy-base-path → sub-mux.
6. **VERIFIED:** Limitation: `store.OpenMariaDB()` uses `sql.Open()` without a connectivity check; `Ping()` exists separately ([`mariadb.go`](go-service/internal/store/mariadb.go#L24-L57)). The current `/ready` handler treats absence of `StoreOpenError` as store-ready, so an unreachable database can false-green until an actual query. See [`handleReady()`](go-service/internal/httpapi/group_health.go#L99-L129) and the readiness decision ([lines 230–316](go-service/internal/httpapi/group_health.go#L230-L316)).

**VERIFIED.** The Windows package has an outer launcher flow before `archive-center-go main`. `Start-ArchiveCtrlCIsolatedProcess()` temporarily enables the process-local Ctrl+C-ignore attribute while each managed backend/MariaDB/ChromaDB child is created, then restores Ctrl+C handling in the PowerShell launcher. During backend lifetime, `ConsoleCancelGate` consumes the launcher's Ctrl+C request and `Wait-ArchiveProcessWithCtrlCConfirmation()` asks for `Y/N`; `N` resumes waiting for the same child and `Y` returns into the existing bounded `Stop-ArchiveChildProcess()` plus Job Object cleanup. Closing or losing the BAT parent still closes the kill-on-close Job Object. Source wiring, the isolated two-signal `N`→`Y` regression, and a user-driven 4.1.0 Windows test package with real MariaDB/ChromaDB are verified: `N` preserved all three service PIDs and live readiness, while confirmed `Y` removed the BAT/process tree and all three listeners.

**VERIFIED.** The plugin has its own initialization: [`init()`](Archive%20Center.js#L51920) registers Risu hooks early, loads persistent settings, syncs runtime configuration to the backend, restores failed/confirmation/persona queues, registers UI controls, and starts non-fatal health/backfill work. Hook registration being requested is not proof that a loaded RisuAI instance accepted or invoked the callbacks.

## 9. End-to-End Turn Flow

```mermaid
sequenceDiagram
    actor User
    participant Risu as RisuAI
    participant JS as Plugin adapter
    participant Go as Go backend
    participant DB as MariaDB
    participant Vec as Chroma
    participant LLM as Optional providers
    participant W as Workers

    User->>Risu: Submit input
    alt Non-empty input
        Risu->>JS: input hook
        JS->>JS: Observe raw input / history-trim command
    else Blank input with Host Say Nothing enabled
        Risu->>Risu: Store synthetic user row before request
        Note over Risu,JS: inspected Host does not call editinput/input handler here
    end
    opt Host output contains a bounded branch marker
        Risu->>JS: output listener
        JS->>Go: POST /session-routing/turn-resolution
        Go->>DB: Validate route/source and persist fork lineage
    end
    Risu->>JS: beforeRequest(payload)
    JS->>JS: Freeze request/session/user-message coordinates
    opt Lorebook mode is not off and scope is new or forced
        JS->>Risu: Observe exact Host scope and current lorebook snapshot
        JS->>Go: POST /sessions/{chat_session_id}/lorebook-reference/snapshots
        Go->>DB: Transactionally record snapshot/current reference projection
    end
    JS->>Go: POST /prepare-turn (source decision)
    Go-->>JS: current_input_decision
    JS->>Go: POST /prepare-turn (full observations)
    Go->>DB: Read canonical state and candidates
    Go->>Vec: Optional semantic search
    Vec-->>Go: Candidate IDs and scores
    Go->>DB: Hydrate and fence candidates
    Go->>DB: Optional exact-scope lorebook reference read
    opt Publisher enabled and eligible
        Go->>LLM: Bounded Publisher request
        LLM-->>Go: publisher_plan.v2 proposal
    end
    Go-->>JS: payload_application_plan.v1 and lineage
    JS->>Risu: Apply exact auxiliary_text to request payload
    Risu->>LLM: Main model request
    LLM-->>Risu: Model response
    Risu->>JS: afterRequest(content)
    JS->>JS: Detach frozen request context and accept official finality
    JS-->>Risu: Return sanitized displayed result
    JS->>Go: Async POST /complete-turn
    Go->>DB: Save user raw row
    Go->>DB: Save assistant raw row
    Go->>DB: Register accepted source revision
    Go->>LLM: Optional Critic extraction
    Go->>DB: Atomic core memory/evidence/precise/outbox admission
    Go->>DB: Separate KG/narrative/character/status projection writes
    opt A saved world-rule has embedding configuration
        Go->>Vec: Direct world-rule index upsert (not outbox-backed)
    end
    Go->>W: Wake workers
    W->>Vec: Outbox-managed verified upsert or delete
    W->>DB: Complete or retry job/outbox state
```

**VERIFIED.** **Input stage.** `onInputHook()` is an observation stage, not an injection stage. `onBeforeRequest()` freezes the request/session/user-message/finality coordinates in `captureFinalConfirmationRequestContext()`, then calls `/prepare-turn` in source-decision-only mode. Each `tryPrepareTurn()` first attempts lorebook synchronization when the mode is not `off`; the second same-scope call normally reuses the attempt state instead of rereading the Host. If Go does not return an eligible current-input decision, JavaScript preserves the original payload. The later persistence path must use this request-owned context rather than re-resolving the current chat.

**VERIFIED limitation.** In inspected official RisuAI commit `72ce721878d65b09baf4339638dfd221d1788261`, a blank non-group submit with `useSayNothing` enabled stores a synthetic user row containing `*says nothing*` and does not call the normal `editinput`/plugin input handler. The 4.1.0 adapter/backend therefore observes that stored row as ordinary non-empty user content; its explicit `[auto-continue]` metadata is available only when an actual empty input was observed. The literal is not an official origin field and must not become a runtime classifier. Which exact RisuAI version the affected user loaded remains **UNKNOWN**.

**VERIFIED.** **Preparation stage.** Before the source-decision/full `/prepare-turn` reads, the adapter creates an Archive-only copy of Yumi Translator 1.4.2 assistant messages when complete `yumi-tr:v1` marker ranges and their matching `$__yumi_tr.<id>` `scriptstate` records are available. This makes Publisher, continuity, language trace, and recent-context analysis read Yumi's stored model original regardless of which plugin's `beforeRequest` callback runs first. It does not rewrite the active chat, displayed translation, raw Host observations, or outgoing Risu payload; unavailable metadata leaves the translated inner text readable and does not reject the request. The full `/prepare-turn` call then provides the eligible input and host facts. Go performs Store/vector reads, optional exact-scope lorebook lookup, policy, budgeting, and optional Publisher work. Complete candidate text reaches the final Go-owned `buildPrepareTurnMemoryDeliveryPlan()` without an arbitrary per-item pre-cap; diagnostic text and internal context rendering can still be bounded later. The plugin applies the returned auxiliary plan at `beforeRequest`, the last supported stage where the outbound payload is writable. Lorebook synchronization/read failure is lane-local and does not abort the main turn.

**VERIFIED.** **Output stage.** `onAfterRequest()` atomically detaches and clears `_activeFinalConfirmationRequestContext` before asynchronous work. If that context is missing, canonical persistence does not start. Official RisuAI source for the pinned inspection commit serializes top-level main-chat generation, while the replacer callback itself exposes no request ID. If an unexpected second `beforeRequest` nevertheless overlaps the active single-slot owner, the adapter terminalizes and detaches both contexts so either completion order fails closed instead of mixing ownership. For a normal save type it normalizes the text, calls `acceptRisuAfterRequestFinal()`, accepts `risu_afterRequest` itself as an official finality source, schedules `continueAcceptedFinalPersistence()` with `Promise.resolve().then(...)`, and immediately returns the display text. A later active-chat host signal can supply recovery finality; the separate `output` listener never accepts finality and never calls `/complete-turn`. `/complete-turn` transport failures enter the existing retry/status path. Evidence: [`installFinalConfirmationRequestContext()`](Archive%20Center.js), [`captureFinalConfirmationRequestContext()`](Archive%20Center.js), [`onAfterRequest()`](Archive%20Center.js), and [`continueAcceptedFinalPersistence()`](Archive%20Center.js). Loaded-host serialization and callbacks remain **UNKNOWN**.

## 10. Memory Retrieval and Injection Flow

**VERIFIED.** The active primary path is `/prepare-turn`; `/search` remains a separate retrieval endpoint and uses the same canonical-vector boundary.

| Stage | Current behavior and owner | Evidence | Status |
| --- | --- | --- | --- |
| Query/current input | Go resolves one request query set. An explicit Host-observed `continuity_query`, otherwise `raw_user_input` or the latest non-assistant message, supplies the current query. Go also reads recent completed user/final-assistant conversation pairs already supplied by the RisuAI adapter, up to UI `recent_conversation_reference_count`; a value of 5 therefore uses five completed conversations. Each pair remains a separate semantic query, and fact lexical scoring also evaluates each query separately and keeps the strongest score. Conversation originals are retrieval context only and are not restored as another final payload block. No continuation-phrase list or special `이어서 적어주세요` classifier exists. JavaScript must not infer query meaning or eligibility on its own. This restores and extends the retrieval role removed in 3.4.0-dev commit `594e00a` without restoring duplicate Recent Raw Turn delivery. | [`prepareTurnRetrievalQueries()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnPriorityQuerySetRelevance()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`handlePrepareTurn()`](go-service/internal/httpapi/group_turn_prepare.go) | VERIFIED source/regression; loaded RisuAI quality UNKNOWN |
| Canonical read | Go first resolves confirmed worldline history segments. It reads each ancestor only through its fork boundary and each child from its owned start; unresolved/conflicting lineage does not invent parent inheritance. It then reads memories, evidence, KG, narrative, persona/private, status, and other projections through the Store. | [`resolvePrepareTurnHistoryScope()`](go-service/internal/httpapi/group_turn_prepare.go#L31), [prepare Store reads](go-service/internal/httpapi/group_turn_prepare.go) | VERIFIED |
| Semantic search | When configured, Go embeds every member of the request query set and searches each confirmed worldline history session. Results from current input/continuity and recent completed user/final-assistant conversation pairs are merged by document identity, retaining the strongest similarity. UI `top_k` controls only the Chroma aggregate-Memory result limit; `recent_conversation_reference_count` independently controls recent conversation depth, and the core-memory maximum independently controls final fact delivery. The separately retained `tier=memory` search protects aggregate Memory recall from precise/evidence crowding. Atomic fact scoring reuses the same query vectors in a `source_table=precise_memory_units` search whose per-session count is the exact active, public, in-scope canonical unit count, so final delivery K cannot truncate the facts before they receive scores. No second selector or persistence path is created. A supplied single client query vector preserves its existing primary-query behavior and reports that additional history embeddings were unavailable. The compact production response keeps full `recall_result` omitted but exposes the six non-content query/count observations under `trace_preview.vector_recall_query`, so a loaded RisuAI request can be inspected without exposing query text or vector values. | [`prepareTurnVectorShadowWithPreciseCandidateLimits()`](go-service/internal/httpapi/prepare_turn_recall.go), [`handlePrepareTurn()`](go-service/internal/httpapi/group_turn_prepare.go), [`chromaWhere()`](go-service/internal/vector/chroma.go), [`selectPrepareTurnMemoryLanesWithVector()`](go-service/internal/httpapi/prepare_turn_recall.go#L880-L1293) | VERIFIED source/regression; loaded RisuAI diagnostics UNKNOWN |
| Canonical hydration | Aggregate Memory hits and precise fact hits are hydrated through the selected Store; in authority mode that is MariaDB. Aggregate Memory facts enter the priority pool only from existing recall-selected rows; loading a session no longer adds every Memory row as another candidate source. A precise vector score is used only after its `precise_memory_units` row is found, source-active, public/general-memory eligible, and inside the confirmed history segment. The private vector-hit handoff is consumed and removed before the public prepare-turn response. Missing, wrong-session, stale, inactive, superseded, or out-of-segment material is not promoted from Chroma alone, but missing relevance also does not reject the normal recalled-memory path. | [`filterPrepareTurnActiveSourceRevisionVectors()`](go-service/internal/httpapi/prepare_turn_recall.go#L156-L241), [`prepareTurnHydrateVectorMemoryHits()`](go-service/internal/httpapi/prepare_turn_recall.go#L1956-L2086), [`prepareTurnLoadGeneralPreciseMemoryUnits()`](go-service/internal/httpapi/prepare_turn_recall.go), [`prepareTurnHydratePreciseMemoryVectorFacts()`](go-service/internal/httpapi/prepare_turn_recall.go), [`ListGeneralVectorPreciseMemoryUnits()`](go-service/internal/store/mariadb_precise_memory.go) | VERIFIED source/regression |
| Eligibility | Go scopes canonical reads to the requested session and excludes future-turn, tombstoned/superseded, scope-ineligible, and private-perspective-ineligible material before assembly. | [prepare Store reads and assembly](go-service/internal/httpapi/group_turn_prepare.go#L219-L569) | VERIFIED |
| Duplicate suppression | Go deduplicates repeated selection of the same stored source occurrence/row while preserving distinct occurrences and provenance, then records delivery lineage/status. Exact text alone is not sufficient identity. | [`prepareTurnMemorySourceOccurrenceKey()`](go-service/internal/httpapi/prepare_turn_recall.go#L1648), [`prepareTurnMemoryLaneLines()`](go-service/internal/httpapi/prepare_turn_memory.go#L12) | VERIFIED |
| 4.1 observation baseline | Go emits `memory_injection_baseline.v1` for memory, direct evidence, KG, state, persona, relationship, storyline, pending thread, and hierarchy summary. It distinguishes current prepare candidates, selections, renders, adapter-observed payload application, and unobserved display effect. Duplicate fingerprints are candidate telemetry only and never authorize suppression. | [`buildMemoryInjectionBaseline41()`](go-service/internal/httpapi/output_fidelity_lineage.go), [`observeGoPayloadApplication()`](Archive%20Center.js) | VERIFIED source/regression; live payload/display UNKNOWN |
| Optional PDF transport | After the existing plan is complete, Go exposes the exact selected `long_term_memory` and remaining auxiliary Text. Direct Google/Gateway modes additionally receive a transient searchable Go PDF. Explicit Provider Manager mode emits no PDF bytes; JavaScript wraps only the selected lane in one manual `<pm-pdf>` range and Provider Manager creates the document. Retry normalizes and reapplies that range once. | [`buildPrepareTurnMemoryTransport()`](go-service/internal/httpapi/prepare_turn_memory_transport.go), [`pdfmemory.Generate()`](go-service/internal/pdfmemory/generator.go), [`applyProviderManagerMemoryPDFPayload()`](Archive%20Center.js), [`onMemoryTransportBodyInterceptor()`](Archive%20Center.js) | VERIFIED source/regression/package/backend-readiness; loaded Host/provider UNKNOWN |
| Ranking/coverage | Query eligibility requires exact phrase or sufficient lexical overlap, with a protected structured-anchor exception. Exact phrase/relevance outrank importance; noneligible, nonprotected rows do not deep/recent-fill. Lexical evaluation still runs after vector success. | [`prepareTurnMemoryRecallEvidence()`](go-service/internal/httpapi/prepare_turn_recall.go#L592), [`selectPrepareTurnMemoryLanesWithVector()`](go-service/internal/httpapi/prepare_turn_recall.go#L880-L1293) | VERIFIED |
| Budgeting | JavaScript supplies settings/runtime-token observations, while Go remains the final owner. Priority-enabled auto and custom modes both build `memory_delivery_plan.v2`. The UI core-memory maximum is applied independently to complete recalled turn summaries and to each scored fact lane, and unused item slots do not transfer. A turn summary and event facts share the existing `event_recent` character budget; every other lane uses its existing UI class budget in custom mode. Auto mode and zero custom values retain the final global envelope as the available class cap. Reviewed names, aliases, and identity evidence remain attributable metadata and can be attached once to a selected fact for the same canonical entity without taking another K slot; inability to fit metadata never blocks that fact. K is a ceiling rather than a target. Direct evidence and protected secret authority remain outside K. Every item is delivered whole or deferred. | [`estimateAdaptiveInjectionBudgetParts()`](Archive%20Center.js), [`buildPrepareTurnMemoryDeliveryPlan()`](go-service/internal/httpapi/prepare_turn_memory_budget.go), [`prepareTurnPriorityDeliveryCaps()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`buildPrepareTurnPriorityMemoryDeliveryPlan()`](go-service/internal/httpapi/prepare_turn_priority_memory.go) | VERIFIED source/regression |
| Priority score lineage | `priority_score.static.v4` consumes `PriorityFactSeed` units built from existing admitted projections. It retains fact text, the request query-set source, per-fact relevance, stored source importance, the retained importance_after_turn_decay field (equal to importance in v4), RP-turn-distance recency, continuity bonus, small independent speaker/location/storyline score biases, final score, deterministic rank, source occurrence, visibility/perspective scope, projection source, lifecycle diagnostics, and selected/deferred/superseded reason. Each recalled `memories.turn_summary` is also traced as a complete summary candidate with the higher of its best child-fact score and observed aggregate-vector score, score origin, representative fact ID, and all member fact IDs. When a matching canonical `precise_memory_unit` vector hit exists, that fact's strongest similarity across the request query set is the relevance owner; one hit is consumed by one matching fact and is never copied to sibling facts from the same parent Memory row. Speaker `0.04`, location `0.05`, and storyline `0.06` biases have a combined `0.12` ceiling and only adjust rank—they do not admit, reject, or suppress a memory. Recency uses only distance from the current RP turn with a 32-turn half-life and `0.20` floor; wall-clock pauses never age story memory. Stored importance receives its `0.25` weight independently of turn recency; equally relevant and important recent facts benefit only from the separate recency term. AI-produced lifecycle metadata does not replace fact identity, change final score, or select a canonical winner: plan, progress, completion, and follow-up expressions remain independent candidates. Completion-like wording supplies no hidden lifecycle rank or continuity bonus. A parent row's aggregate-vector score contributes to its complete summary and remains diagnostic for individual fact relevance. Relevance zero or an unavailable semantic score remains a sortable score observation rather than a `no_current_context_affinity` rejection. Unseeded source text is sentence-split only as a compatibility fallback and remains deliverable. Request-scoped current resolution changes only non-lifecycle fact-level delivery projection and never deletes source rows. Plan diagnostics distinguish summary and fact candidate/selection counts and expose per-group K outcomes. | [`prepareTurnPrioritySemanticFactFromPreciseUnit()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnPriorityStructuredBias()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnPriorityTurnDistanceRecency()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnBuildPriorityCandidates()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`prepareTurnBuildPriorityTurnSummaries()`](go-service/internal/httpapi/prepare_turn_priority_memory.go), [`buildPrepareTurnPriorityMemoryDeliveryPlan()`](go-service/internal/httpapi/prepare_turn_priority_memory.go) | VERIFIED source/regression; loaded payload/display effect live UNKNOWN |
| Lorebook reference search/delivery | In non-off modes, Go searches only the exact persisted Host scope by exact phrase, key, and lexical overlap. `search_only` traces candidates without delivery. `reference_assist` requires a fully observed scope, direct key/always-active activation, remaining reference budget, and exact-duplicate suppression; delivered support is `reference_only`. | [`prepareTurnLorebookReferenceSearch()`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go#L403), [`finalizePrepareTurnLorebookReference()`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go#L103) | VERIFIED |
| Ordering/render | Go fixes base auxiliary order as `original_work`, `long_term_memory`, `output_guidance`; `reference_assist` inserts `lorebook_reference` immediately before `output_guidance`. Go concatenates exact lane text and emits hashes. It intentionally renders `input_context_text` as empty because RisuAI already carries recent chat; the internal input-context value remains available only for Publisher/turn analysis. Go does **not** choose the host message index. | [`buildPrepareTurnPayloadApplicationPlan()`](go-service/internal/httpapi/prepare_turn_render.go), [`attachPrepareTurnLorebookReferenceLane()`](go-service/internal/httpapi/prepare_turn_render.go) | VERIFIED |
| Plugin application | JavaScript rejects a missing/mismatched plan. For a valid plan it inserts only the single `auxiliary_text` system block at a JavaScript-selected host position and records `payload_application_observation.v1`. There is no active `injectInputContextBeforeUser()` path. | [`applyGoPayloadApplicationPlan()`](Archive%20Center.js), [`injectAuxiliaryBlock()`](Archive%20Center.js) | VERIFIED |

**VERIFIED.** The exact base injection order is Go lane text `original_work` → `long_term_memory` → `output_guidance`. In `reference_assist`, it is `original_work` → `long_term_memory` → `lorebook_reference` → `output_guidance`; outside that mode the lorebook lane is omitted. JavaScript inserts the resulting auxiliary block at its resolved host index and does not add a second input-context block. Go owns inner lane order; host mutation is JavaScript-owned, while auxiliary index policy remains boundary debt.

**VERIFIED.** The separate [`handleSearch()`](go-service/internal/httpapi/group_memory_search.go) is registered at `POST /search`. It also treats vector results as selectors, hydrates them from MariaDB, and uses lexical fallback. It must not be interpreted as a second canonical memory store.

**VERIFIED.** Fallback: source-decision or full `/prepare-turn` transport/ineligibility failure returns the original payload before legacy orchestration. After an eligible response, `orchestrateTurnHelpers()` can use the legacy compatibility reads only when a compatible compact plan is unavailable; `applyGoPayloadApplicationPlan()` itself preserves the payload when no valid plan reaches application. Separately, the intentional-orchestration-skip branch can call JavaScript-owned `applyProtectionOnlyInjection()`, so “no Go plan always means no mutation” would be false. No broader overlap/ambiguity fallback was verified in the current call graph.

**VERIFIED.** Suppression is inspectable through `memory_delivery_lineage`, selection/deferred states, counts, reason codes, and payload observations. `memory_injection_baseline.v1` adds observation-only surface accounting and duplicate candidates; it does not change current selection, suppression, retrieval, canonical admission, or vector policy. Any change that preserves final text but drops lineage is a contract regression. The current `trace_preview` contains a telemetry inconsistency: `/prepare-turn` can call the Publisher, but the preview writes `would_call_llm: false` ([Publisher call](go-service/internal/httpapi/group_turn_prepare.go#L1057), [trace preview](go-service/internal/httpapi/group_turn_prepare.go#L1278-L1289)).

## 11. Output Processing and Commit Flow

1. **VERIFIED:** A model response is not structured truth. `onAfterRequest()` normalizes visible content, binds it to the request/session captured at `beforeRequest`, and accepts official `risu_afterRequest` finality before scheduling persistence.
2. **VERIFIED:** `/complete-turn` validates lifecycle observations, source/session/turn alignment, reroll/replacement state, idempotency key, and conflicting raw text before the persistence boundary. See [`handleCompleteTurn()`](go-service/internal/httpapi/group_turn_complete.go#L117-L605), [`complete_turn_source_acceptance.go`](go-service/internal/httpapi/complete_turn_source_acceptance.go), and [`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go).
3. **VERIFIED:** The raw path is non-atomic. After `context.WithoutCancel`, `persistCompleteTurnRaw()` calls `SaveChatLog(user)` and `SaveChatLog(assistant)` separately. Only after both are durable does `registerCompleteTurnSourceRevision()` run, also separately. A failure can therefore leave a recoverable partial raw pair; idempotent duplicate checks/retry are relied on rather than one enclosing transaction. [`persistCompleteTurnRaw()`](go-service/internal/httpapi/group_turn_complete.go#L1812-L1862), [call and source registration](go-service/internal/httpapi/group_turn_complete.go#L645-L658).
4. **VERIFIED:** Effective input, critic feedback, and audit rows are separate Store writes outside the raw pair and common-admission transaction. `save_ok` marks the handler's raw durability result; it does not prove every derived projection or vector is complete.
5. **VERIFIED:** Critic output is a proposal. The backend builds a bounded Critic input after raw durability, parses/validates the response, and may enqueue a durable reprocessing job on failure rather than fabricate projections ([`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go#L670-L1020)).
6. **VERIFIED:** Only the core is atomic. `commitAcceptedMemoryAdmission()` requires an accepted current source revision. MariaDB `CommitMemoryAdmission()` atomically reconciles the core memory, direct evidence, precise units, vector-outbox rows, and admission-state update in one source-fenced `READ COMMITTED` transaction ([`commitMemoryAdmissionOnce()`](go-service/internal/store/mariadb_memory_admission.go#L116-L233)).
7. **VERIFIED:** The post-admission path is non-atomic. After core admission, `saveCriticExtractionArtifacts()` separately writes precise-memory projections, subjective entity memories, narrative state, story clock, KG triples, character/state artifacts, reversible states, and pruning effects. Any of these can fail after the core transaction commits. Administrative `WithMemoryAdmissionVectorReplay` is the explicit exception: it returns immediately after canonical memory/evidence/precise-memory vector admission and does not replay those post-admission state writers. [`saveCriticExtractionArtifacts()`](go-service/internal/httpapi/turn_extraction_persist.go#L97-L339), [`memory_derivation.go`](go-service/internal/store/memory_derivation.go).
8. **VERIFIED:** A compatibility path exists. If the Store does not expose common admission, the same function falls back to individual `SaveMemory`, `SaveEvidence`, precise-unit, vector, and projection operations. `mariadb_authority` normally exposes common admission, so this is callable compatibility behavior, not the normal authority path.
9. **VERIFIED:** Failure reporting distinguishes `atomic_rollback` for a failed `CommitMemoryAdmission`, `partial_commit` when some separate derived writes succeeded before another failed, and `no_commit` when none succeeded ([`completeTurnPersistenceRollbackState()`](go-service/internal/httpapi/group_turn_complete.go#L64-L76)).
10. **VERIFIED:** Ownership: the plugin has no direct MariaDB/Chroma client. Active HTTP runtime mutations go through Store/vector interfaces. Operator executables `mariadb-schema`, `mariadb-import`, and `legacy10-migrate` are explicit direct-SQL exceptions outside the service call graph.

### Canonical and persistent write-surface inventory

**VERIFIED.** This inventory covers every mounted handler family or background/operator entry point found capable of mutating MariaDB product state. “Persistent projection” does not automatically mean “canonical truth”; Step-23 DTOs deliberately report their truth-writer flags as false. Preview/search/view-model/provider-only POST routes are excluded because their code does not persist product state.

| Writer location | Mutating entry points or calls | State affected | Status |
| --- | --- | --- | --- |
| Normal completed-turn path | `POST /complete-turn`; `persistCompleteTurnRaw()`, `registerCompleteTurnSourceRevision()`, `SaveEffectiveInput`, feedback/audit, common admission, post-admission writers | Raw turn/effective input, source lifecycle, core admitted memory, typed projections, jobs/outbox/audits | VERIFIED |
| Turn repair/lifecycle routes | `POST /turns/repair-replay`, `POST /effective-inputs`, `DELETE /rollback/{turn_index}`, `POST /turn-workflow/recovery`, and `POST /session-routing/turn-resolution` when it persists route binding or validated worldline lineage | Raw repair, effective input/audit, canonical tail/source invalidation and queued vector cleanup, job/source recovery, session route bindings and fork lineage | VERIFIED — [`group_turn.go`](go-service/internal/httpapi/group_turn.go), [`group_turn_rollback.go`](go-service/internal/httpapi/group_turn_rollback.go), [`turn_workflow_hud.go`](go-service/internal/httpapi/turn_workflow_hud.go), [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go) |
| Direct canonical compatibility routes | `POST /canonical/{chat_session_id}/chat-logs`, `effective-inputs`, `memories`, `evidence`, `kg-triples`, `audit-logs`, `critic-feedback`, `character-events` | Direct canonical table rows; guarded by `usesShadowWriteStore()` | VERIFIED — [`registerCanonicalRoutes()`](go-service/internal/httpapi/group_canonical.go#L13-L36) |
| Explorer mutations | Memory/KG/evidence PATCH, review/revalidate/tombstone/supersede, regenerate, and DELETE/POST-delete routes | Existing memory/evidence/KG rows and regeneration outputs | VERIFIED — [`registerMemoryRoutes()`](go-service/internal/httpapi/group_memory.go#L8-L68) |
| Narrative/session/import routes | Manual session DELETE (`req_source=timeline_manual_delete` required); active-scope/director PATCH; storyline PATCH/trust/DELETE; character PATCH/speech/DELETE; world-rule PATCH/trust/DELETE; episode/chapter/arc/saga generation and episode PATCH/DELETE/regenerate/merge; pending-thread mutations; `POST /feedback`; `POST /import/hypamemory` | Session and narrative/persona-adjacent canonical rows, summaries, feedback, imports. Session deletion commits the MariaDB cleanup transaction first and then queues durable vector cleanup when lifecycle/outbox support exists. `/storylines/sync` and `/world-rules/sync` reject `apply` and are dry-run proposal validators. | VERIFIED — [`registerNarrativeRoutes()`](go-service/internal/httpapi/group_narrative.go), [`group_session_control.go`](go-service/internal/httpapi/group_session_control.go), [`mariadb_status.go`](go-service/internal/store/mariadb_status.go) |
| Manual identity routes | Character/item merge preview, merge, and unmerge under the narrative route group | Reviewed/revoked `canonical_equivalence` rows in `entity_identity_links`; preview writes nothing. Writes are per selected source and can partially succeed. Existing canonical rows and vectors are not rewritten or reindexed automatically. | VERIFIED — [`group_characters.go`](go-service/internal/httpapi/group_characters.go), [`group_items.go`](go-service/internal/httpapi/group_items.go), [`entity_identity.go`](go-service/internal/store/entity_identity.go) |
| Persona routes | Subjective/persona-memory create/patch/delete/capsule/alias-repair/force-merge and persona-capsule create/delete/attach/detach | Persona capsules, attachments, subjective entity memories and owner identity | VERIFIED — [`registerPersonaRoutes()`](go-service/internal/httpapi/group_persona.go#L15-L33), [`group_persona_capsules.go`](go-service/internal/httpapi/group_persona_capsules.go) |
| Admin/maintenance routes | Database reset, maintenance enqueue/pass, rescan, `POST /admin/session-normalize`, session migrate, and dedupe cleanup; reindex/vector-orphan operations directly upsert/delete derived Chroma documents | Broad canonical rows and audits; some operations mutate only the derived vector index. Session normalization inspects, replays missing non-conflicting raw roles, rescans, repairs only missing unambiguous character/item identities/surfaces, then reindexes when eligible. The admin job manager is in-process; `deferred` is terminal but not completed. | VERIFIED — [`registerAdminRoutes()`](go-service/internal/httpapi/group_admin.go), [`group_admin_session_normalize.go`](go-service/internal/httpapi/group_admin_session_normalize.go), [`admin_jobs.go`](go-service/internal/httpapi/admin_jobs.go) |
| Session migration routes | `migrate-complete`, `migrate-reindex`, `migrate-lock-source`, `migrate-rollback`, `migrate-cleanup-source` (`migrate-preview` is read-only) | Copied/moved session state, migration baselines/locks, cleanup and direct derived-vector upsert/delete. `session-migration.manifest.v4` includes identity, route/worldline, lifecycle/outbox, and lorebook scope/lock families; lorebook snapshots/entries move through their declared indirect ownership. | VERIFIED — [`registerSessionMigrationRoutes()`](go-service/internal/httpapi/group_session_migration.go), [`session_migration_manifest.go`](go-service/internal/store/session_migration_manifest.go) |
| Status-schema routes | Proposal create/review, registry import, current-value/event/effect writes and effect-state PATCH; proposal/definition routes directly index successfully saved rows when embedding input/config is available | Status proposal/registry/current/history/effect rows plus derived status vectors | VERIFIED — [`registerStatusSchemaRoutes()`](go-service/internal/httpapi/group_status_schema.go#L391-L405), [`group_status_vector.go`](go-service/internal/httpapi/group_status_vector.go#L12-L167) |
| Step-23 projection routes | Consequence, psychology, fork-lineage, theme/offscreen, capture-verification create/status/repair routes | Persistent auxiliary projection/verification records; response contracts mark them as non-canonical-truth writers. Fork-lineage manual repair validates child/parent/source history and writes a deterministic confirmed lineage record. | VERIFIED — [step-23 registrations](go-service/internal/httpapi/server.go), [`group_step23_fork_lineage.go`](go-service/internal/httpapi/group_step23_fork_lineage.go) |
| Reference-library routes | Work/continuity/document creation and update/delete, extraction/status, timeline normalization, library review/exclusion, auto-review, direct reference-vector reindex/stale deletion, and session binding apply/update/delete | Canonical reference corpus, jobs/review decisions, session bindings; the separately configured Chroma reference collection is derived and its reindex verifies by listing documents, not through the memory outbox | VERIFIED — [`registerReferenceLibraryRoutes()`](go-service/internal/httpapi/group_reference_library.go#L89-L114), [`runReferenceVectorReindex()`](go-service/internal/httpapi/group_reference_vectors.go#L366-L515) |
| Host lorebook snapshot route | `POST /sessions/{chat_session_id}/lorebook-reference/snapshots`; `ApplyLorebookReferenceSnapshot()` | Persistent, non-canonical Host observation ledger plus exact-scope current projection. One default-isolation transaction uses a per-session lock; it has no complete-turn idempotency/source-revision fence, retry queue, outbox, or Chroma mutation. | VERIFIED — [`registerLorebookReferenceRoutes()`](go-service/internal/httpapi/group_lorebook_reference.go#L49), [`ApplyLorebookReferenceSnapshot()`](go-service/internal/store/mariadb_lorebook_reference.go#L198) |
| Canon-pack/source-discovery routes | Canon-pack install/lifecycle, overlay create; discovery job create/resume/complete/admit/repair | Canon registry/pack/overlay, discovery ledgers, admitted reference documents/candidates | VERIFIED — [`registerCanonPackPreviewRoutes()`](go-service/internal/httpapi/group_canon_pack_preview.go#L19-L28), [`registerSourceDiscoveryRoutes()`](go-service/internal/httpapi/group_source_discovery.go#L36-L44) |
| Background workers | Reprocessing job claim/fail/complete and repeated `saveCriticExtractionArtifacts()`; vector-outbox claim/fail/complete | Derived canonical projections/job state; MariaDB outbox status plus derived Chroma documents | VERIFIED — [`memory_reprocessing_worker.go`](go-service/internal/httpapi/memory_reprocessing_worker.go), [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go) |
| Explicit operator tools | `mariadb-schema --execute`, `mariadb-import --execute`, `legacy10-migrate --execute` | Schema and direct imported canonical rows | VERIFIED; not active HTTP runtime and not all are packaged binaries |

**VERIFIED.** Physical runtime SQL mutation implementations are concentrated under [`go-service/internal/store/mariadb_*.go`](go-service/internal/store); active `internal/httpapi` code calls Store interfaces rather than importing `database/sql`. Outside that layer, `mariadb-schema` and `mariadb-import` execute SQL directly, while `legacy10-migrate` drives the guarded migration pipeline. SQL-using compare/export/audit and managed-E2E commands are read-only or disposable validation paths, not active product writers.

**VERIFIED.** Other non-canonical mutations: the lorebook snapshot route writes persistent reference observations; `PUT /prompts/{prompt_name}` writes prompt files; `POST /config/update` changes process memory; Chroma mutations change only derived indexes; `/update/download` and `/update/apply` change staged/package files and shutdown state. Chroma is not mutated solely by the outbox worker: direct callers include normal world-rule extraction, status-schema indexing, admin reindex/orphan cleanup, session migration/rollback/delete, explorer document deletion, reference reindex, and the non-common-admission memory/evidence compatibility path ([`turn_extraction_vector.go`](go-service/internal/httpapi/turn_extraction_vector.go#L19-L159)). None of these automatically writes conversational canonical truth. `POST /turns` and `POST /turns/complete` are guard-only compatibility stubs; `/storylines/sync` and `/world-rules/sync` are proposal dry-runs; `POST /rollback/decision`, turn-workflow notices, and admin job cancellation update in-process ledgers only.

## 12. Storage and Indexing Model

```mermaid
flowchart TD
    A["Host-accepted complete turn"] --> B["Validate source and idempotency"]
    B --> C["MariaDB raw turn and source revision"]
    C --> D{"Critic result valid?"}
    D -->|"yes"| E["MemoryAdmission transaction"]
    D -->|"no"| F["Audit and reprocessing job"]
    E --> G["Core memory, evidence, precise units"]
    E --> H["Vector outbox rows"]
    E --> I["Commit core transaction"]
    I --> S["Separate KG, narrative, character, status projections"]
    S --> T["Optional direct typed-vector mutation"]
    T --> U["Chroma without memory-outbox completion"]
    F --> J["Reprocessing worker lease/retry"]
    J --> E
    H --> K["Vector worker lease"]
    K --> L["Embed and Chroma upsert/delete"]
    L --> M["Exact readback verification"]
    M -->|"verified"| N["Complete MariaDB outbox"]
    M -->|"failed"| O["Retry or permanent audit state"]
    N --> P{"source still active?"}
    P -->|"no"| Q["Compensating vector delete"]
    P -->|"yes"| R["Derived index consistent"]
```

**VERIFIED.** **Canonical store.** In `mariadb_authority`, MariaDB owns accepted raw chat/effective input, source revisions, admitted memories/evidence/precise units, KG and typed projections, identity links, session route/fork lineage, audits, job ledgers, and vector outbox state. The current 4.3 migration directory extends through `013_precise_memory_text_fields.sql`; `010` creates four non-canonical Host-reference tables, `011`/`012` add worldline lineage, and `013` widens the precise-memory text fields. `mariadb-schema` treats either the migration directory or `001_schema.sql` as the entry point for loading every sibling SQL file in sorted order, so the four tables do not need to be duplicated into `001`.

**VERIFIED.** **Transaction boundary.** [`CommitMemoryAdmission()`](go-service/internal/store/mariadb_memory_admission.go#L26-L53) retries deadlock-class transaction errors. [`commitMemoryAdmissionOnce()`](go-service/internal/store/mariadb_memory_admission.go#L116-L233) starts a `READ COMMITTED` transaction, locks/checks the source revision, preserves an already committed result for idempotent replay, writes the core memory/evidence/precise-unit projections and outbox rows, updates admission state, and commits those operations atomically. It does not include the separately saved raw pair, effective input/feedback/audits, KG, narrative, character, status, or other typed projections.

**VERIFIED.** **Asynchronous derivation.** Reprocessing and the core memory/evidence/precise vector-outbox operations are MariaDB-backed, leased jobs. They are woken after complete-turn work and drain eligible items while runtime provider configuration is synced. A stale source revision is rejected rather than re-derived into current state. This statement does not cover the direct typed/admin/migration/reference vector calls listed above.

**VERIFIED.** **Index consistency.** For outbox-managed documents, the vector worker embeds if necessary, mutates Chroma, requires exact readback, then completes the MariaDB outbox row. If an upsert succeeded but the source fence became stale before completion, it attempts a compensating delete ([`processClaimedMemoryVectorOperation()`](go-service/internal/httpapi/memory_vector_outbox_processor.go#L186-L312)). Direct vector helpers such as normal world-rule extraction and status-schema indexing report an immediate result/warning but have no durable memory-outbox retry or exact-readback step; admin/reference reindex paths implement their own verification, while migration and delete paths have route-specific status handling. Temporary MariaDB/Chroma divergence is therefore possible on more than the outbox retry path, but Chroma must never promote itself to canonical truth.

**VERIFIED.** **Recovery.** The common writer freezes the first committed extraction/result hash for deterministic replay; a failed core admission can stage the successful Critic result, and workers retry without re-sampling an already committed extraction. This recovery guarantee does not prove automatic reconciliation of every post-admission typed projection after a partial commit. Representative evidence is [`resolveCommittedMemoryAdmissionExtraction()`](go-service/internal/httpapi/turn_memory_admission.go#L16-L79) and [`mariadb_memory_admission_test.go`](go-service/internal/store/mariadb_memory_admission_test.go#L345-L409).

**VERIFIED.** **Host-reference persistence.** `ApplyLorebookReferenceSnapshot()` atomically creates/finds the exact scope, appends a snapshot, records entry revisions, and updates that scope's current projection. A complete observed snapshot replaces the current projection; a complete empty snapshot clears it; consent revocation clears it; partial/unavailable or older authoritative observations are retained without replacing a newer current projection. This path has no automatic freshness/TTL gate, complete-turn idempotency ledger, durable retry queue, source-revision fence, or vector update. `DeleteSession()` now deletes lorebook entries, snapshots, scopes, and locks inside its session transaction. `session-migration.manifest.v4` declares lorebook scopes/locks directly and snapshot/entry ownership indirectly. Source support is **VERIFIED**; live delete/migration behavior remains **UNKNOWN**.

## 13. API and Hook Contracts

### RisuAI hooks

| Hook | Exact registration | Contract |
| --- | --- | --- |
| Input | `addRisuScriptHandler("input", onInputHook)` | Observation/correlation only; not the outbound payload mutation point. A confirmed new input terminalizes an unconsumed stale request context before the new request begins. |
| Before request | `addRisuReplacer("beforeRequest", onBeforeRequest)` | Only supported main-request payload application point. Capture the exact request/session/user-message context here and bind the resulting orchestration object to that request. An exact same-turn reentry may replay only the already-ready Go plan; otherwise preserve the normal new-request or fail-closed overlap path. |
| After request | `addRisuReplacer("afterRequest", onAfterRequest)` | Detach the captured request context, normalize visible output, accept official finality, and schedule persistence without blocking display. The adapter does not equate an existing raw pair with completed derived processing: it routes the accepted pair to Go, where raw-only turns resume Critic work and raw-plus-derived replays end idempotently. Missing captured context means no canonical persistence. |
| Output listener | `addRisuChatListener("output", onRisuOutput)` | Observe only bounded worldline branch facts and route them to Go; never accept finality, complete a turn, or decide lineage in JavaScript. |
| Unload | `onUnload(removeRegisteredRisuHooksOnUnload)` | Removes registered handlers and local lifecycle state. |

**VERIFIED.** Source registration awaits `input`, `beforeRequest`, `afterRequest`, then the `output` listener, and finally unload cleanup. Normal request flow is input observation → before-request capture/preparation/mutation → main model → after-request finality/display/persistence scheduling. A provider retry of that same logical request is a bounded loop from main-model failure back to the same request-owned payload application, not a second Archive turn. HUD stage 6 (`awaiting_final_output`) remains visible with an attempt notice; the successful `afterRequest`/`/complete-turn` path advances the same request to stage 7 (`final_output_accepted`). The output listener is an independent host signal that may run when branch facts appear; registration order does not make it a normal finality stage. Loaded-host invocation order remains **UNKNOWN** without a current lifecycle trace. Evidence: [`registerRisuLifecycleHooks()`](Archive%20Center.js), [`renderTurnWorkflowHUDSameRequestRetry()`](Archive%20Center.js), [`handleCompleteTurn()`](go-service/internal/httpapi/group_turn_complete.go).

### Important backend route groups

| Routes | Owner and purpose | Primary evidence |
| --- | --- | --- |
| `GET /health`, `GET /ready`, `GET /version`, `POST /config/update` | Liveness, dependency readiness, build/runtime identity, in-memory provider settings | [`group_health.go`](go-service/internal/httpapi/group_health.go) |
| `POST /prepare-turn` | Current-input decision, canonical/vector recall, policy, budget, optional Publisher, final application plan | [`group_turn_prepare.go`](go-service/internal/httpapi/group_turn_prepare.go) |
| `POST /complete-turn`, `GET /complete-turn/request-status` | Source acceptance, raw durability, Critic/admission handoff, transport idempotency | [`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go), [`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go) |
| `POST /rollback/decision`, `DELETE /rollback/{turn_index}` | Go-owned observed-deletion/manual rollback decision, route/digest/revision fencing, canonical tail mutation, queued derived-index cleanup | [`group_turn_rollback.go`](go-service/internal/httpapi/group_turn_rollback.go), [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go) |
| `POST /session-routing/turn-resolution`, `/step23/fork-lineage` | Host route/worldline resolution and validated manual lineage repair; not narrative-truth admission | [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go), [`group_step23_fork_lineage.go`](go-service/internal/httpapi/group_step23_fork_lineage.go) |
| Character/item identity preview, merge, unmerge; `GET /items/{session}` | Explicitly reviewed equivalence links and canonicalized read projections | [`group_characters.go`](go-service/internal/httpapi/group_characters.go), [`group_items.go`](go-service/internal/httpapi/group_items.go) |
| `POST /admin/session-normalize` | In-process staged repair/replay/identity repair/reindex job with explicit deferred/partial outcomes | [`group_admin_session_normalize.go`](go-service/internal/httpapi/group_admin_session_normalize.go), [`admin_jobs.go`](go-service/internal/httpapi/admin_jobs.go) |
| `POST /search` (`handleSearch()`) and retrieval/explorer reads | Canonical-hydrated search and read models | [`group_memory_search.go`](go-service/internal/httpapi/group_memory_search.go) |
| `/canonical/{chat_session_id}/...` | Direct Store-backed read/write compatibility surfaces; write handlers are guarded by `usesShadowWriteStore()` | [`registerCanonicalRoutes()`](go-service/internal/httpapi/group_canonical.go#L13-L36), [`usesShadowWriteStore()`](go-service/internal/httpapi/server.go#L175-L193) |
| `POST /sessions/{chat_session_id}/lorebook-reference/snapshots` | Validate and persist an exact Host lorebook observation through the separate `LorebookReferenceStore`; not a canonical-memory or vector writer | [`registerLorebookReferenceRoutes()`](go-service/internal/httpapi/group_lorebook_reference.go#L49), [`LorebookReferenceStore`](go-service/internal/store/lorebook_reference.go#L107) |
| `/admin/...`, `/sessions/...`, narrative/persona/reference/status routes | Maintenance, migration, read models, and the distinct write families inventoried in section 11 | [`RegisterRoutes()`](go-service/internal/httpapi/server.go#L203-L232) |
| `/step22/...` | Read-only adoption/validation preview in the mounted code | [`group_step22_adoption_gate.go`](go-service/internal/httpapi/group_step22_adoption_gate.go#L33) |
| `/step23/...` | Persistent auxiliary projection/verification records; their response contracts do not claim canonical-truth-writer authority | [step-23 registrations](go-service/internal/httpapi/server.go#L221-L227) |
| `POST /turns`, `POST /turns/complete` | **OBSOLETE.** Compatibility stubs that call `writeShadowGuard()` and do not persist; the active completed-turn entry is `POST /complete-turn` | [`group_turn.go`](go-service/internal/httpapi/group_turn.go#L57-L95) |
| `/proxy/plugin-main` | Provider-neutral proxy/Publisher request handling | [`group_proxy.go`](go-service/internal/httpapi/group_proxy.go) |

### Version, hash, ordering, and error constraints

- DTO owners are [`dto.PrepareTurnContractRequest`](go-service/internal/dto/prepare_source_contract.go#L57), [`dto.PrepareTurnRequest`](go-service/internal/dto/types_gen.go#L960), and [`dto.M4CompleteTurnRequest`](go-service/internal/dto/types_gen.go#L659). Do not recreate these contracts in JavaScript.
- Current memory contracts are `memory_recall_plan.v1`, priority-disabled legacy `memory_delivery_plan.v1`, and priority-enabled auto/custom-budget `memory_delivery_plan.v2` ([`prepare_turn_memory_budget.go`](go-service/internal/httpapi/prepare_turn_memory_budget.go), [`prepare_turn_priority_memory.go`](go-service/internal/httpapi/prepare_turn_priority_memory.go)); final host application remains `payload_application_plan.v1` ([`prepare_turn_render.go`](go-service/internal/httpapi/prepare_turn_render.go#L30-L149)). The nested K trace is `core_priority_memory_delivery.v3`, which reports the complete-turn-summary group and each scored fact lane separately. The opt-in representation contract is `memory_transport_plan.v1`; direct Google/Gateway modes also use transient `memory_transport_payload.v1`, while `provider_manager_pdf` intentionally has no Archive Center PDF payload because the external adapter creates it ([`prepare_turn_memory_transport.go`](go-service/internal/httpapi/prepare_turn_memory_transport.go)). These consume the completed application plan and do not replace its authority. JavaScript rejects an incompatible application version or a direct plan/payload ID mismatch.
- Current optional guidance uses `supervisor_support_packet.v2`, `response_execution_contract.v1`, `supervisor_scene_proposal.v3`, and `publisher_plan.v2`. Go accepts only `ready`/`partial` source-backed items, renders one Go-owned block in `compact`, `standard`, or `explicit` form, and gives the plan no truth/write authority ([`buildBoundedSupervisorResult()`](go-service/internal/httpapi/group_proxy.go#L835), [`supervisorSceneProposalGuidanceItems()`](go-service/internal/httpapi/prepare_turn_render.go#L596)).
- Current Host-reference contracts are `lorebook_reference_scope.v1`, `lorebook_reference_snapshot.v1`, and `lorebook_reference_recall.v1` ([`prepare_turn_lorebook_reference.go`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go#L15-L20), [`lorebook_reference.go`](go-service/internal/store/lorebook_reference.go#L9-L17)). They convey reference-only observations, not canonical admission.
- Persistence/index lifecycle contracts include `memory_source_revision.v1` and `memory_vector_outbox.v1` ([`memory_derivation.go`](go-service/internal/store/memory_derivation.go#L12-L13)).
- Rollback and worldline contracts include `rollback.decision.v2`, `session-routing.turn-resolution.v1`, `risu_worldline_observation.v2`, `risu_branchedfrom.v1`, `session_fork_lineage.v2`, and `worldline_topology.viewmodel.v2`. JavaScript supplies bounded observations; Go owns route, deletion, lineage, and topology decisions ([`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go), [`group_step23_fork_lineage.go`](go-service/internal/httpapi/group_step23_fork_lineage.go), [`store.go`](go-service/internal/store/store.go)).
- Manual identity and normalization contracts include `character_identity_manual_merge.v1`, `item_identity_manual_merge.v1`, `session-normalize.v1`, and `session-migration.manifest.v4` ([`group_characters.go`](go-service/internal/httpapi/group_characters.go), [`group_items.go`](go-service/internal/httpapi/group_items.go), [`group_admin_session_normalize.go`](go-service/internal/httpapi/group_admin_session_normalize.go), [`session_migration_manifest.go`](go-service/internal/store/session_migration_manifest.go)).
- Complete-turn uses an idempotency key/recorded-response ledger. A key conflict is non-retryable; unknown or retryable outcomes can be queried without blindly duplicating writes ([`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go#L257-L410)).
- Memory lane order, item text, source refs, budgets, serialization, and hashes are compatibility-sensitive. A change must update backend producer, adapter consumer, DTO/contract tests, and payload-observation tests together.
- HTTP errors use stable code/message JSON helpers and middleware. CORS defaults to `*`; bearer enforcement is optional and must be explicitly enabled ([`server.go`](go-service/internal/httpapi/server.go#L291-L367)).

## 14. Configuration and Runtime Modes

**VERIFIED.** Configuration is loaded from environment variables by [`config.Load()`](go-service/internal/config/config.go#L210-L346), validated by [`Config.Validate()`](go-service/internal/config/config.go#L413-L454), and supplemented by the plugin's runtime-only `/config/update` call. No secret values are reproduced here.

| Area | Current options / representative variables | Behavior |
| --- | --- | --- |
| Bind and browser access | `AC_BIND_ADDR`, `AC_ALLOWED_ORIGINS` | Default bind is loopback `127.0.0.1:28080`; default allowed origins are `*`. |
| Authority mode | `AC_MODE` = `shadow`, `live`, `cutover` | `live`/`cutover` require the explicit authority guard. |
| Store mode | `AC_STORE_MODE` = `noop`, `dual_shadow`, `mariadb_shadow`, `fixture_shadow`, `mariadb_read_shadow`, `mariadb_authority` | Only `mariadb_authority` is product authority. `mariadb_shadow` is a noop-primary/MariaDB-shadow dual writer; `mariadb_read_shadow` wraps MariaDB read-only; fixture/noop are not authority. |
| Runtime profile | `AC_RUNTIME_PROFILE` = `client_only`, `core_lite`, `vector_external`, `vector_local_native`, `full_local` | Validated against vector mode. |
| Vector mode | `AC_VECTOR_MODE` = `off`, `fallback`, `external`, `local_native`, `local_proot`, `bundled` | Chroma endpoint/collections are separate for session and reference data. |
| MariaDB | `AC_MARIADB_DSN` | Required for MariaDB authority; value is secret and must not be logged or documented. |
| Chroma | `AC_CHROMA_*` | Controls enablement, endpoint, session collection, reference collection, and timeouts. |
| Providers | runtime plugin settings plus `AC_EMBED*`, Critic/Publisher settings | `/config/update` stores provider keys/endpoints/models only in process memory; response reports `persisted: false`. |
| Publisher presentation | `none`, `weak`, `medium`, `strong`, `extreme`, `maximum`; formats `compact`, `standard`, `explicit` | Strength changes current-response explicitness only. `none` disables the Publisher call, not memory/secret guards. Format changes Go rendering only. |
| Host lorebook reference | Plugin setting `off` (default), `search_only`, `reference_assist` | Only direct `mariadb_authority` currently exposes `LorebookReferenceStore`; other Store modes return lane-local `unavailable`. |
| Prompts | `AC_PROMPT_DIR` and prompt filenames | Packages copy `critic_system.txt` and `supervisor_system.txt`. |
| Lifecycle/retry | prune mode, Critic ledger flags, embedding/Critic timeouts, vector retry limit | Controls admission and worker eligibility/retry behavior. |
| Auth | bearer token and enforcement variables | Disabled by default; required for some managed/update deployments. |
| Updates | `AC_UPDATE_*` | Stable channel and update enablement defaults are set in Go config. |
| Build identity | `AC_BUILD_VERSION` and plugin constants | Go, plugin, and package-builder defaults are `4.2.0`; the plugin channel is `release` for 4.2 publication; this marker does not establish loaded-host or native-device verification. |

**VERIFIED source.** The repository-root [`.env.example`](.env.example), Go default, Windows/POSIX package-builder defaults, and packaged env template use `4.2.0`. A built package must still be checked independently because builders stamp copied metadata and `AC_BUILD_VERSION` at build time.

## 15. Generated, Packaged, Legacy, and Experimental Files

**VERIFIED.** Do not normally edit any `_dist*`, `_release*`, `_test-builds`, `_runtime*`, cache, binary, package-manifest, or copied package file. The Windows builder defaults to `_dist`, compiles only `archive-center-go`, `archive-center-updater`, and `mariadb-schema`, copies the entire active migration directory plus the active plugin/prompts/templates, then generates package/migration/release manifests ([migration inventory](ops/build-full-package.ps1#L278-L322), [copy](ops/build-full-package.ps1#L530-L534)). The POSIX builder performs the corresponding cross-build/copy process. Other `go-service/cmd` programs are operator/audit/smoke/import tools and are not active service code merely because they contain `main()`.

**OBSOLETE.** Smoke/E2E executables and test fixtures are not active runtime entry points. Some can mutate explicitly provisioned disposable databases or vectors when run with execution flags; that is test activity, not evidence that they can write the deployed product's canonical state through the normal service.

Source-to-output rules:

| Output | Rebuild from |
| --- | --- |
| Packaged `Archive Center.js` | root active [`Archive Center.js`](Archive%20Center.js) |
| `bin/archive-center-go*` | [`go-service/cmd/archive-center-go`](go-service/cmd/archive-center-go) |
| `bin/archive-center-updater*` | [`go-service/cmd/archive-center-updater`](go-service/cmd/archive-center-updater) |
| `bin/mariadb-schema*` | [`go-service/cmd/mariadb-schema`](go-service/cmd/mariadb-schema) |
| Packaged SQL | [`migrations`](migrations) |
| Packaged prompts | active files in [`prompts`](prompts) |
| Package manifests/checksums | `ops/build-*.ps1` scripts |

**VERIFIED.** `Archive Center 3.4-C.js`, timestamped `Archive Center.js.codex-backup-*`, and `prompts/critic_system.pre-first-compression-20260808.txt` are **OBSOLETE** historical copies for active runtime defaults. The inert `_step18MarkerSurface` historical evidence object has been removed. `AC Recomposer Agent.js` is an optional, separately installed consumer of the active transient Recomposer bridge, but no package-copy/autoload step was found; live installation is **UNKNOWN**. Other standalone Recomposer/quality-layer copies remain **INFERRED** separate/legacy.

**VERIFIED.** A newly run package builder inventories and copies all active migrations, currently through `013`. No existing `_test-builds`, `_dist`, `_release`, or packaged copy is source authority, even if a version log records its hashes. `build-full-package.ps1` always rewrites the copied display label and `AC_BUILD_VERSION`, but rewrites plugin `//@version` and `const VERSION` only for strict `x.y.z`; verify source, package, loaded plugin, runtime endpoint, and published release identity separately.

## 16. Failure and Fallback Behavior

| Failure | Current behavior | Visibility / safety | Status and evidence |
| --- | --- | --- | --- |
| Backend unavailable/timeout | `bridgeFetch()` returns `null`. Source-decision or full `/prepare-turn` failure/ineligibility preserves the original payload before legacy orchestration. Legacy reads are reachable only after an eligible response without a compatible compact plan. Complete-turn transport has its own persistent retry/status path. | Main generation fails open. The intentional-orchestration-skip branch can still inject the JavaScript protection-only block. | VERIFIED — [`bridgeFetch()`](Archive%20Center.js), [`onBeforeRequest()`](Archive%20Center.js), [`applyProtectionOnlyInjection()`](Archive%20Center.js) |
| Windows Ctrl+C shutdown canceled with `N` | Managed service children ignore the console Ctrl+C inherited at creation. The PowerShell gate records the request and prompts in its normal wait loop. `N` resumes waiting for the same backend process; it does not run cleanup or restart a replacement. After confirmed `Y`, `cmd.exe` can show its separate generic batch-termination prompt; that second prompt controls only the BAT window and cannot undo service cleanup. A successful PowerShell return exits the BAT without the legacy unconditional `pause`; only nonzero exits pause to expose the error code. | The backend, managed MariaDB, and managed ChromaDB remain running after the launcher's `N`. `Y`, BAT-parent loss, or abnormal launcher exit still reaches bounded/Job Object cleanup, and a completed test launcher cannot later attach to a refreshed package path. | VERIFIED source/process/package regression — [`windows-console-control.ps1`](ops/full-package/scripts/windows-console-control.ps1), [`Wait-ArchiveBackendLifetime()`](ops/full-package/scripts/start-full-windows.ps1), [`windows-launcher-ctrl-c-smoke.ps1`](ops/windows-launcher-ctrl-c-smoke.ps1) |
| Fresh-install ZIP or checksum is missing/tampered | The release helper requires one published `SHA256SUMS*.txt` asset, finds the exact selected ZIP filename record, hashes the downloaded ZIP, and extracts only after equality. | The install pointer and package launcher are not created from an unverified archive. Existing-install refusal still occurs before this network path. | VERIFIED source/regression — [`install-github-release.ps1`](scripts/install-github-release.ps1), [`install-github-release.sh`](scripts/install-github-release.sh), [`test-simple-fresh-install.ps1`](scripts/test-simple-fresh-install.ps1), [`test-simple-fresh-install.sh`](scripts/test-simple-fresh-install.sh) |
| MariaDB open/connectivity failure | `sql.Open` errors are caught at construction, but unreachable DB connectivity is not probed at startup/readiness. Later route queries fail. | Open errors block authority startup; unreachable-host false-green is not safely visible in `/ready`. | VERIFIED limitation — [`mariadb.go`](go-service/internal/store/mariadb.go#L24-L57), [`handleReady()`](go-service/internal/httpapi/group_health.go#L99-L129) |
| Main Chroma unavailable/incompatible | If enabled, preflight/readiness checks health and collection compatibility; startup fails or readiness degrades/blocks. Prepare/search can use lexical/canonical fallback depending on mode/path. | Visible in readiness and retrieval trace; must not widen eligibility. | VERIFIED — [`ValidateRuntimeDependencies()`](go-service/internal/httpapi/server.go#L60-L80), [`prepareTurnVectorShadow()`](go-service/internal/httpapi/prepare_turn_recall.go#L18-L154) |
| Reference Chroma unavailable | Separate reference health degrades without blocking main readiness or normal turn persistence. | Explicit degraded fields; tested as non-blocking. | VERIFIED — [`server_preflight_test.go`](go-service/internal/httpapi/server_preflight_test.go#L90-L179) |
| Malformed Publisher/Critic response | Publisher makes exactly one provider request. Empty/container/strict-JSON/schema/no-valid-item states mark the Publisher HUD stage `failed` with the exact reason while the turn completes `completed_with_warning`; `valid_empty` retains its successful empty meaning. No fabricated guidance is rendered. Critic failure does not fabricate projections. | Publisher fails open while base memory remains; Critic can leave raw evidence durable and enqueue reprocessing. | VERIFIED — [`runSupervisorLLM()`](go-service/internal/httpapi/group_proxy.go#L214-L318), [prepare statuses](go-service/internal/httpapi/group_turn_prepare.go#L1057-L1143) |
| Lorebook Host API, snapshot Store, or read unavailable | Synchronization records a warning when possible and the turn continues. Go returns lane-local `unavailable`; non-authority Store modes lack `LorebookReferenceStore`. The adapter marks a same scope attempted before transport and has no durable retry queue, so non-forced automatic attempts are suppressed until scope/settings/process state changes. | No lorebook lane/Publisher support is delivered; last confirmed current projection can remain stored after unavailable/partial observation. An ambiguous retry could append another snapshot because the route generates a fresh ID. | VERIFIED — [`syncCurrentLorebookReference()`](Archive%20Center.js#L15661), [`prepareTurnLorebookReferenceSearch()`](go-service/internal/httpapi/prepare_turn_lorebook_reference.go#L403), [`handleLorebookReferenceSnapshot()`](go-service/internal/httpapi/group_lorebook_reference.go#L193) |
| Missing/incompatible payload application plan | Normal application returns the original payload with a skipped/error observation; the deleted legacy assembler is not used. | Fail-open generation without Go memory/guidance. The separate intentional-orchestration-skip branch can still use protection-only injection. | VERIFIED — [`applyGoPayloadApplicationPlan()`](Archive%20Center.js), [`applyProtectionOnlyInjection()`](Archive%20Center.js) |
| PDF not selected, empty, unsupported, generation failed, plan mismatch, or provider body shape unavailable | Go retains the existing Text mode for empty/failing PDF generation. JavaScript mutates only matching opt-in modes and exact plan/payload IDs; otherwise it returns the original provider body. It does not create another provider request or another `/prepare-turn`. | Normal Text delivery and existing output/save lifecycle remain available. The reason/application observation contains no base64 or selected memory body. | VERIFIED source/regression; loaded provider UNKNOWN — [`buildPrepareTurnMemoryTransport()`](go-service/internal/httpapi/prepare_turn_memory_transport.go), [`onMemoryTransportBodyInterceptor()`](Archive%20Center.js) |
| Provider Manager PDF experiment selected without matching Provider Manager conversion settings | Archive Center still sends the exact selected memory inside one `<pm-pdf>` range and does not inspect or modify external plugin settings. Provider Manager owns whether that range becomes a PDF. | `Gemini PDF` and `manual selection` must both be enabled in the tested Provider Manager model/settings. Without loaded-Host evidence, final PDF conversion and lane preservation remain UNKNOWN; this is not an Archive Center output/save rejection condition. | VERIFIED Archive Center source/regression; loaded Provider Manager UNKNOWN — [`applyProviderManagerMemoryPDFPayload()`](Archive%20Center.js) |
| Missing captured `beforeRequest` context at `afterRequest` | The adapter cannot prove which request/session/user-message owns the visible output, so canonical persistence does not start. | Display remains available; no global session/orchestration reconstruction is allowed to guess ownership. | VERIFIED — [`captureFinalConfirmationRequestContext()`](Archive%20Center.js), [`onAfterRequest()`](Archive%20Center.js) |
| Same logical request invokes `beforeRequest` again | The inspected RisuAI provider retry loop re-enters the replacer with no request ID or retry-cause argument. | Only an exact Host-turn identity with a ready replay-safe Go plan reuses the prior request ID/context. `/prepare-turn`, Publisher/search, raw-input binding, and HUD priming stay at one call; payload application is idempotent. The HUD remains at 6/12 and says only that a Risu request retry was observed, never that a timeout was proven. | VERIFIED source/regression; loaded Host UNKNOWN — [`finalConfirmationRequestContextRetryIdentityMatches()`](Archive%20Center.js), [`reapplyFinalConfirmationRetryPayload()`](Archive%20Center.js) |
| Different or incomplete `beforeRequest` context overlaps an active response | The replacer exposes no correlation ID, so two distinct or insufficiently identified completions cannot be assigned safely. | Both contexts are terminalized and detached; display can continue, but neither completion is persisted under the other owner. No fabricated correlation ID, completion-order binding, queue, watcher, or latest-context fallback is introduced. | VERIFIED source/regression; loaded Host UNKNOWN — [`installFinalConfirmationRequestContext()`](Archive%20Center.js) |
| Host Say Nothing user row is deleted alone | The inspected Host can preserve the generated assistant and later messages after single-message deletion. The 4.1.0 active-chat pair builder selects the last assistant before the next user, so a missing middle user can shift an assistant to the previous user and leave another assistant-only observation. Existing DB role pairs then conflict with the active chat. | Repair/normalize preserves conflicted rows rather than overwriting them, but the current sequence can surface `assistant_only`, conflict and downstream `completed_turn_gap`. Source mechanics are verified; the reported 61-chat turn mapping/worldline count is inferred until a redacted support bundle is available. Do not classify the literal, fabricate a user row, silently renumber, or auto-mutate canonical/worldline state. | VERIFIED source limitation; affected live case INFERRED — [`buildCompletedTurnPairsFromActiveChatMessages()`](Archive%20Center.js), [`group_admin_session_normalize.go`](go-service/internal/httpapi/group_admin_session_normalize.go), inspected RisuAI `DefaultChatScreen.svelte`/`Chat.svelte` |
| Rollback requested without verified deletion | `rollback.decision.v2` remains blocked unless current route ownership and complete assistant observations prove an active source disappeared, or an explicit manual candidate passes its separate guards. Execution rechecks one-use token, observation digest, and route revision. | No canonical mutation; post-output replacement uses `superseded`, not deletion. | VERIFIED — [`group_turn_rollback.go`](go-service/internal/httpapi/group_turn_rollback.go), [`group_turn_range_decision.go`](go-service/internal/httpapi/group_turn_range_decision.go) |
| Session delete after MariaDB commit | `DeleteSession()` removes canonical/session-owned rows in one transaction. With lifecycle/outbox support, the HTTP response reports `canonical_committed: true` and `vector_cleanup: queued` and wakes the worker without inline draining. | Canonical deletion can precede derived-index convergence; audit is best effort. | VERIFIED — [`group_session_control.go`](go-service/internal/httpapi/group_session_control.go), [`mariadb_status.go`](go-service/internal/store/mariadb_status.go) |
| Mixed raw repair conflict | Repair Replay preserves the conflicting existing role, records the conflict, and may still insert the other missing non-conflicting role. | No overwrite or all-or-nothing loss of the valid role; conflict/failed-turn details remain visible. | VERIFIED — [`group_admin_rescan.go`](go-service/internal/httpapi/group_admin_rescan.go) |
| Critic/provider output exhaustion | Provider termination such as `length`, `max_tokens`, `max_output_tokens`, or context-window exhaustion maps to retryable token-exhausted classification. Partial/truncated content is not synthesized into canonical Critic JSON. | Raw evidence and independent termination/token diagnostics remain available for retry/HUD. | VERIFIED — [`proxy_provider.go`](go-service/internal/httpapi/proxy_provider.go), [`turn_extraction_critic.go`](go-service/internal/httpapi/turn_extraction_critic.go) |
| Client disconnect during accepted complete-turn | Backend detaches canonical work from request cancellation with `context.WithoutCancel`. | Protects accepted writes; idempotency/status endpoint handles ambiguous transport outcomes. | VERIFIED — [`group_turn_complete.go`](go-service/internal/httpapi/group_turn_complete.go#L605-L610), [`complete_turn_idempotency.go`](go-service/internal/httpapi/complete_turn_idempotency.go#L380-L410) |
| Raw or post-admission write failure | Raw user/assistant/source writes and post-admission typed projections are separate operations. A later failure can leave a durable prefix. | Complete-turn reports diagnostics and `partial_commit`/`no_commit`; core admission alone reports `atomic_rollback` on its transaction failure. Recovery coverage for every typed projection is not proven. | VERIFIED — [`persistCompleteTurnRaw()`](go-service/internal/httpapi/group_turn_complete.go#L1812-L1862), [`completeTurnPersistenceRollbackState()`](go-service/internal/httpapi/group_turn_complete.go#L64-L76), [`saveCriticExtractionArtifacts()`](go-service/internal/httpapi/turn_extraction_persist.go#L97-L339) |
| Worker/provider failure | Lease is failed to retryable or permanent state according to configured limits; Critic reprocessing uses the greater of the configured base interval and a valid provider retry hint. A malformed hint alone is ignored without discarding the provider error, job, raw turn, derived history, or other diagnostics. Critic reprocessing and vector outbox remain in MariaDB. An automatically recovering HUD remains nonterminal but uses the existing `x_only` dismissal policy; closing it hides the Host status stream without cancelling the durable job. | Traceable job/outbox state and HUD attempt/next-time/termination/token details; terminal states restore manual recovery. | VERIFIED — [`memory_reprocessing_worker.go`](go-service/internal/httpapi/memory_reprocessing_worker.go), [`proxy_provider.go`](go-service/internal/httpapi/proxy_provider.go), [`turn_workflow_hud.go`](go-service/internal/httpapi/turn_workflow_hud.go), [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go#L186-L312) |
| Direct Chroma mutation failure | Normal world-rule/status and compatibility vector helpers record an immediate skip/failure/warning after the canonical row may already exist; they do not enqueue a durable memory-outbox retry or require exact readback. Admin/reference/migration/delete routes expose their own failure/status behavior. | MariaDB remains authoritative, but a derived document can be missing or stale until an explicit maintenance/reindex path repairs it. | VERIFIED — [`upsertDerivedArtifactVector()`](go-service/internal/httpapi/turn_extraction_vector.go#L81-L159), [`indexStatusSchemaProposal()`](go-service/internal/httpapi/group_status_vector.go#L12-L89), [`runReferenceVectorReindex()`](go-service/internal/httpapi/group_reference_vectors.go#L366-L515) |
| Stale reroll/edit source | Source revision fence rejects derivation; vector upsert completion can compensate with delete. | Prevents stale derived memory from becoming current. | VERIFIED — [`complete_turn_source_revision.go`](go-service/internal/httpapi/complete_turn_source_revision.go), [`memory_vector_outbox_processor.go`](go-service/internal/httpapi/memory_vector_outbox_processor.go#L300-L312) |
| Missing runtime provider sync | Workers defer processing; prepare Publisher/Critic features degrade rather than inventing output. | Runtime status should expose config sync; queues remain durable. | VERIFIED — [`processMemoryWorkerWake()`](go-service/internal/httpapi/memory_reprocessing_worker.go#L118-L157) |

## 17. Common Mistakes and Architectural Guardrails

Only the document-wide evidence labels are used here. **VERIFIED** followed by “risk” means the risky surface or behavior is directly present; **VERIFIED** followed by “guard” means the active path contains the stated protection; **INFERRED** followed by “risk” means the failure is plausible from the connected surfaces but not reproduced end to end.

| # | Title and classification | Affected paths / why dangerous | Safe rule | Required validation |
| --- | --- | --- | --- | --- |
| 1 | Editing generated output — **VERIFIED** risk | `_dist*`, `_release*`, `_test-builds`, package copies exist beside sources; an edit can disappear on rebuild. | Edit active root/plugin, Go, migration, prompt, or build source only. | Rebuild and compare manifest/source identity. |
| 2 | Multiple implementation copies drift — **VERIFIED** risk | `Archive Center 3.4-C.js`, backups, version-log package paths, and packaged copies can be mistaken for active 4.1.0 source. | Treat root `Archive Center.js` and `go-service` as active. | Check package copy hash/version against active source. |
| 3 | JavaScript duplicates backend policy — **VERIFIED** risk | Active adapter still calculates local turn and budget observations, placement, legacy compatibility reads, and exceptional protection prose. Rollback range/authorization is Go-owned. | Move/repair policy in Go and remove replaced JS in the same bounded slice. | JS line delta, `node --check`, backend contract and payload fidelity tests. |
| 4 | Adapter bypasses Go plan — **VERIFIED** exception | Normal `applyContextInjection()` requires `payload_application_plan.v1`, but the intentional-orchestration-skip branch can call `applyProtectionOnlyInjection()` and inject JavaScript-owned prose. | Preserve exact Go-plan application and migrate/remove the exceptional policy path. | Output-fidelity/lineage tests plus live intentional-skip payload inspection. |
| 5 | Vector data treated as canonical — **VERIFIED** guard; **VERIFIED** consistency gap | Prepare/search hydrate Chroma candidates through the selected Store and fence source revisions. Core admitted documents use an outbox, but several typed/admin/migration/reference paths mutate Chroma directly without the same durable retry/exact-readback contract. | Chroma selects; the authority Store decides existence, scope, lifecycle, and text. Reconcile and observe direct index writers independently. | Stale/missing/wrong-session vector tests, direct-writer failure/reindex tests, and real Chroma test. |
| 6 | Unverified text promoted to truth — **VERIFIED** guard | Source acceptance, Critic parsing, evidence, and common admission separate raw evidence from structured proposals. | Preserve raw accepted evidence; require typed validation and provenance. | Reroll/correction/conflicting-output tests. |
| 7 | Evidence or correction provenance lost — **VERIFIED** guard | Evidence rows and source revisions are carried into core admission/recall. | Never write memory without source/session/revision/evidence identity. | Direct evidence, user-correction, supersession tests. |
| 8 | Writes bypass the common commit path — **VERIFIED** risk | Direct canonical, explorer, admin, narrative, persona, status, Step-23, original-work reference, Host-reference snapshot, canon-pack, source-discovery, migration, and operator-tool writes coexist with `/complete-turn`. The Host-reference route is persistent but deliberately non-canonical. | Normal turn-derived memory must use accepted `/complete-turn` and common admission; keep Host-reference observations in their separate Store; narrowly authorize/test every other writer family. | Route auth, authority-mode, source-fence, audit, reference-boundary, and direct-tool tests. |
| 9 | Duplicate native and Archive Center injection — **INFERRED** risk | Native context plus Archive Center lanes can overlap despite Go dedupe and source refs. | Dedupe on stable provenance/text and record actual payload hash/lineage. | Live Risu payload capture with native context enabled. |
| 10 | Wrong Risu hook stage — **VERIFIED** guard | Input observes, beforeRequest captures and mutates, afterRequest consumes request context and accepts visible-output finality, and output observes branch facts only. | Keep those stage responsibilities fixed; never let output complete a turn. | Loaded-plugin hook observation and actual payload/display/worldline lineage. |
| 11 | Fallback reported as normal success — **VERIFIED** risk | MariaDB connectivity is not pinged, so `/ready` can report ready before the first failed query. | Readiness must verify the authority DB, and degraded states must remain explicit. | Unreachable-MariaDB preflight/readiness regression. |
| 12 | Suppression hides valid memory — **VERIFIED** guard | Go emits delivery lineage, reasons, counts, and deferred/suppressed state. | Never dedupe/suppress without stable, inspectable reason/source refs. | Coverage, protected-memory, dedupe, and lineage tests. |
| 13 | Async race, duplicate, or partial commit — **VERIFIED** mixed boundary | Source fences, idempotency, terminal replacement-failure records, the core-admission transaction, leases, and outbox-managed vector exact-readback provide guards; raw pair/source registration, post-admission projections, and direct vector writes remain separate and can diverge or report `partial_commit`. | Never leave a failed non-committed replacement as the active pending revision; do not describe the whole turn or whole index as atomic; preserve durable diagnostics and add reconciliation for separate writers where required. | Raw-row failure, non-tail replacement, new-key replay of a terminal failed revision, deadlock, stale source, post-admission/direct-vector failure, and worker-crash tests. |
| 14 | Configuration drift — **VERIFIED** risk | The source config default and `.env.example` retain 4.2.0; active JS is test.18. The package builder sets the runtime template/launcher to test.18 while `.env.source.example` remains a source reference. A displayed version alone does not identify executable contents. | Validate source defaults, templates, package rewrite, and runtime status together. | Package smoke test and `/version`/`/ready` identity comparison. |
| 15 | Roadmap described as implementation — **VERIFIED** risk | Active source uses `memory_recall_plan.v1`, `memory_delivery_plan.v1`, `payload_application_plan.v1`, and `publisher_plan.v2`. Older integrated roadmap prose still proposes recall/injection v2, while the current 4.0 roadmap retains v1 delivery/application. | Classify each contract independently from its active producer, validator, consumer, and caller; do not infer a shared version. | Contract-version grep, negative compatibility tests, and end-to-end payload evidence. |
| 16 | Obsolete code mistaken as active — **VERIFIED** risk | Old plugin copies and inert marker surfaces can be mistaken for active runtime; the unreferenced JavaScript budget assembler has now been removed. | Prove activation/import/caller/package copy before using a file or symbol as evidence. | Entry-point, call-site, and build-input inventory. |
| 17 | Schema/API/plugin/docs inconsistency — **VERIFIED** risk | `trace_preview.would_call_llm` is hard-coded false even when Publisher may have run; some route comments still say shadow while authority mode is included. | Version and test observable contracts; update comments/docs with code. | Publisher-called trace test and authority route response test. |
| 18 | Ordering, budgets, hashes, serialization changed casually — **INFERRED** risk | Payload application, memory delivery, idempotency, source revision, and outbox use versioned hashes/order. | Treat these fields as compatibility contracts; version intentional changes. | Golden DTO, hash, lane order, replay, and payload parity tests. |
| 19 | Sensitive content logged — **VERIFIED** risk | Debug mode logs message previews/input previews; bridge failure diagnostics retain a bounded response body. This proves content exposure to local diagnostics, not a secret leak in normal mode. | Default debug off; mask credentials and minimize/expire user-content diagnostics. | Log review with synthetic secrets and debug on/off. |
| 20 | Errors silently discard memory/state — **VERIFIED** guards; **UNKNOWN** completeness | Primary complete-turn uses status lookup/retry queue; backend uses durable reprocessing/outbox retries for their covered units. Recovery of every separate post-admission projection and directly written vector after partial failure was not established. | Never drop a current-version item without terminal reason, audit, and operator visibility. | Transport ambiguity, restart recovery, retry-limit, legacy-queue migration, typed-projection reconciliation, and direct-index repair tests. |
| 21 | Host lorebook reference promoted or replayed incorrectly — **VERIFIED** risk | Snapshot rows are persistent but non-canonical, scoped, not Chroma-indexed, and lack complete-turn idempotency. The adapter suppresses a same-scope retry after an attempted transport. | Never promote a Host-reference hit to canonical truth or blindly retry an ambiguous snapshot POST. Preserve exact scope, observation status, provenance, and `reference_only` authority. | Complete/empty/partial/unavailable/revoked/out-of-order/concurrent snapshot tests, ambiguous-retry tests, and loaded-Host scope capture. |

## 18. Fragile Areas

- **UNKNOWN.** Live Risu host finality and reroll replacement: source shows official `risu_afterRequest` acceptance and later-host-signal recovery, but loaded RisuAI tests are still required. Relevant paths: `onAfterRequest()`, pending-final recovery code, `complete_turn_source_acceptance.go`, and source revision tests.
- **VERIFIED.** Fragile payload-fidelity boundary: Go owns auxiliary lane text/order/hashes; JavaScript owns actual message insertion and currently the auxiliary index policy. Any change to placement, role, text, lane order, hashes, native-context interaction, or effective-input rewrite needs backend unit tests and actual outbound-payload capture. Representative tests: [`output_fidelity_lineage_test.go`](go-service/internal/httpapi/output_fidelity_lineage_test.go) and [`output_fidelity_guide_efficacy_test.go`](go-service/internal/httpapi/output_fidelity_guide_efficacy_test.go).
- **VERIFIED.** **Memory lane selection and budgets.** Protected coverage, vector/lexical refill, private scope, hierarchy escalation, and final text dedupe are coupled across `prepare_turn_memory*.go`, `prepare_turn_recall.go`, and render code.
- **VERIFIED.** Fragile raw-to-derived durability boundary: the raw user row, raw assistant row, source registration, core admission, and post-admission projections span multiple operations. Test recoverable prefixes and `partial_commit`, not only common-admission rollback, with disconnection, deadlock, reroll, duplicate key, and partial provider/write failure.
- **VERIFIED.** **MariaDB schema compatibility.** Migrations extend through `013`, and package launchers invoke the directory-loading schema tool so both new and upgraded installations receive the complete sorted inventory. `001_schema.sql` is not required to duplicate every later additive table. Verify fresh install, upgrade, and package application through the full inventory; never rewrite historical numbered migrations merely to collapse them into `001`.
- **VERIFIED.** Fragile vector lifecycle: outbox-managed core documents have durable lease/retry, exact-readback, and stale compensation, while normal world-rule/status plus admin/migration/reference/compatibility mutations use direct or route-specific paths. Test real Chroma compatibility, every direct writer's failure/reconciliation behavior, private-memory exclusion, deletion, and contextualized embedding batching without treating the outbox guarantee as global.
- **VERIFIED.** **Runtime config synchronization.** Provider config is process-memory state and workers defer until synced. Restart/rebind behavior needs live tests.
- **VERIFIED.** **Readiness limitation.** Main/reference Chroma semantics are checked separately, but MariaDB authority readiness does not prove connectivity. Add/maintain a real database connectivity check.
- **VERIFIED.** Fragile alternate-writer boundary: the full section-11 inventory can mutate persistent state outside the normal turn UI, including status/Step-23/reference/canon-pack/source-discovery and explicit import tools in addition to explorer/admin/canonical/session-migration. Require auth, audit, source fences, and authority-mode tests according to each contract.
- **VERIFIED.** Fragile Host-reference lifecycle boundary: snapshot admission still lacks durable idempotency/retry/freshness. Session deletion now removes its lorebook rows transactionally, and `session-migration.manifest.v4` includes direct/indirect lorebook ownership. Test real delete/move/copy/rollback behavior rather than reviving the former omission claim.
- **VERIFIED.** Fragile rollback boundary: Timeline/opening logic may observe deletion, but Go must prove disappearance against the captured route and complete assistant observation before mutation. Never reintroduce `beforeRequest` deletion guesses, message-count drift, or UI-selection session coupling.
- **VERIFIED.** Fragile request-ownership boundary: `beforeRequest` owns the immutable final-confirmation context and `afterRequest` consumes it once. A global latest orchestration result is UI/diagnostic state only; using it as persistence authority can mix sessions or drop a displayed turn. The inspected top-level main-chat path is serialized, but its provider retry loop can re-enter `beforeRequest` and still supplies no replacer request ID. Exact same-turn reentry therefore reuses only an already-ready request-owned plan; incomplete or different identity fails closed. Confirm both paths in the actually loaded Host before making a live claim.
- **VERIFIED.** Fragile normalization/identity boundary: repair replay preserves conflicted roles, missing identity repair skips ambiguity, manual identity links are user-reviewed and per-source writes can partially succeed, and deferred normalization is terminal but incomplete.
- **INFERRED.** **Packaging/update recovery risk.** Validate compiled binaries, plugin copy, schema tool, migration manifest, checksums/manifests, rollback, OS-specific startup, and reported version independently.
- **VERIFIED.** **Large single-file adapter.** Small changes can touch unrelated host/UI/policy paths. Avoid broad formatting and report JavaScript lines added/removed.

## 19. Implemented vs Planned Features

### Currently implemented

- **4.3 preceding feedback checkpoint — implemented_unverified:** [GitHub #5/#6/#4/#10/#7 work log](docs/archive-center-4.3-feedback-work-log.md). Existing Go delete coalescing now advances through committed ID batches using existing indexes and releases its writer between batches; its admin-maintenance caller and original source/lease/causal eligibility remain unchanged. Precise subtype/relationship/reveal text is LONGTEXT through fresh SQL, `013` and Go compatibility. Pending/committed Critic result preservation and stored-JSON hash replay remain Go-owned. HUD recovery returns/refreshes the current Go ViewModel; Go corrects estimated display turns using the canonical owner's result and transient creation order, while JS removes click handlers through their registering SafeElement. Source regressions and isolated real MariaDB/Chroma tests passed. Later test packages and multi-agent implementation are indexed in the [current 4.3 summary](docs/archive-center-4.3-status-summary.md); this earlier checkpoint does not establish patched loaded-PocketRisu behavior.

- **4.3 optional preprocessing — implemented_unverified:** `prepare_turn_multi_agent.go` owns `handleMultiAgentSettings()`, persisted common/role prompts and role connections, `callMultiAgent()` and `runMultiAgent()`. `group_turn_prepare.go` connects scoped candidates, first-round parallel analysis, bounded supplemental retrieval/analysis, received recommendations and the existing Publisher/payload owners. `loadMemoryPreprocessingPanel()` in `Archive Center.js` exposes the independent, default-OFF Extensions entry. New settings default to independent connections; saved Publisher sharing remains optional. OFF preserves existing Go preparation without feature-specific calls. See the [implementation record](docs/archive-center-4.3-preprocessing-work-log.md) and the [test.18 package record](docs/archive-center-4.3-test-build-18.md); actual Host/provider/output verification remains separately scoped.

- **4.3 result fidelity — VERIFIED source/regression, live delivery UNKNOWN:** Go preserves received canonical recommendation text/order, uses its ordinary selection for categories without recommendations, retains first-round results after supplemental failure, and distinguishes a successful empty final selection. Source/perspective/privacy scope and predeclared budgets remain with existing owners. The [result-fidelity contract](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#multi-agent-result-fidelity-43) is implemented in source; the original plan wording is not proof of final Host application or displayed effect.

- **VERIFIED:** 4.1.0 RisuAI `input`, `beforeRequest`, `afterRequest`, bounded worldline `output`, and unload hook registration.
- **VERIFIED:** Go `/prepare-turn` current-input decision, canonical/vector retrieval, exact/lexical eligibility, source-revision hydration, provenance-aware suppression, `memory_recall_plan.v1`, priority-disabled legacy `memory_delivery_plan.v1`, priority-enabled auto/custom-budget `memory_delivery_plan.v2`, lane/priority budgeting and ordering, lineage, and `payload_application_plan.v1` rendering.
- **VERIFIED:** Source/regression implementation of observation-only `memory_injection_baseline.v1`, with nine surface summaries, duplicate-candidate diagnostics, baseline-to-payload linkage, adapter payload-application observation, and explicitly unobserved displayed effect. No 4.1 token reduction or suppression/retrieval change is implemented.
- **VERIFIED:** Opt-in PDF transport source/regression/package/backend-live implementation for the already-selected `long_term_memory`: searchable Noto Sans KR PDF, `memory_transport_plan.v1`, transient `memory_transport_payload.v1`, Google `inlineData`, LLM Gateway `file`, exact Text removal, request-owned retry reuse, and Text fail-open. This does not change retrieval/selection/budget/persistence and does not prove loaded RisuAI or provider reading/usage.
- **VERIFIED:** Optional bounded Publisher using `publisher_plan.v2`, `response_execution_contract.v1`, `supervisor_support_packet.v2`, and `supervisor_scene_proposal.v3`; it makes one provider request and renders source-backed accepted items from one or both required role shapes without truth/write authority.
- **VERIFIED:** Runtime Publisher/Critic timeout settings are converted to actual millisecond call limits. NeuralWatt standard/Flex transport and provider-specific DeepSeek V4 `low` reasoning behavior are implemented in `proxy_provider.go`; real provider-account behavior remains **UNKNOWN** in this audit.
- **VERIFIED:** Default-off Host lorebook reference observation/snapshot route, separate MariaDB ledger/current projection, exact/key/lexical search, `search_only` diagnostics, and conditional `reference_assist` lane with `reference_only` authority. This is source-level implementation, not live-Host proof.
- **VERIFIED:** Optional source integration for `archive_center.recomposer_enhancement.v1` and the transient `archive_center.recomposer_bridge.v1`; `AC Recomposer Agent.js` is a separately installed historical product-identity consumer of that currently implemented optional Recomposer contract, not the current `AC Ensemble Agent` identity or an auto-loaded package component. The separate `workbench/ac-ensemble-agent` source is standalone-only and outside this active source repository; Archive Center integration remains planned.
- **VERIFIED:** `/complete-turn` source acceptance, stable observed user-message `chatId` logical-turn anchoring with existing absent-ID fallback, reroll/source revisions, terminal non-committed replacement-failure handling, idempotency ledger, separately persisted raw pair, Critic proposal parsing, atomic core memory admission, separately persisted typed projections, reprocessing jobs, and vector outbox.
- **VERIFIED source/regression/package:** A Critic parse failure with durable `reprocessing=queued` remains a nonterminal `recovering` workflow, but its existing dismissal policy is `x_only`. The RisuAI adapter renders and binds that X for both single and previous-turn HUD slots. Closing it cancels only the Host HUD stream and leaves the durable MariaDB reprocessing job unchanged. Loaded-RisuAI confirmation of the refreshed package remains open.
- **VERIFIED:** Subjective-memory `importance_10` and `emotional_weight` values are preserved per item; canonical scene/world layers are labelled `latest_observed` or `historical` by observed turn instead of treating every open record as current.
- **VERIFIED:** MariaDB store/schema tooling and optional Chroma session/reference collections.
- **VERIFIED:** Reference library/recall, narrative/persona/status/character projection, timeline/dashboard/presentation ViewModel, session migration, maintenance, canon-pack/source-discovery, and managed-update route families are mounted in source. This does not prove that the plugin invokes every family or that every persistent projection is canonical truth.
- **VERIFIED:** Request-scoped `beforeRequest` → `afterRequest` persistence ownership; observed-deletion-gated `rollback.decision.v2`; source-level Risu branch observation, Go worldline resolution, `session_fork_lineage.v2`, confirmed fork-boundary retrieval scope for Store/vector reads, topology ViewModel, and manual lineage repair.
- **VERIFIED:** Source/regression same-logical-turn Risu retry reuse: exact Host identity, one Archive request ID, one preparation/Publisher/search cycle, idempotent Go-plan payload application, one `afterRequest`/`/complete-turn` consumption, stale-context termination on new input, Say Nothing no-write termination, and HUD 6/12 retention followed by backend-owned 7/12 acceptance.
- **VERIFIED:** Default `nativeFetch` backend transport plus opt-in experimental Web Risu JSON bridge transport using request-scoped `risuFetch(..., {plainFetchForce: true})`. The experimental path rejects raw/binary bodies and disables live HUD streaming; current browser/HTTPS deployment behavior remains **UNKNOWN**.
- **VERIFIED:** Explicit reviewed character/item identity preview/merge/unmerge; staged `session-normalize.v1` with mixed-conflict repair and missing unambiguous identity repair; `session-migration.manifest.v4`; session deletion with transactional lorebook/identity/child-fork-lineage cleanup and queued vector convergence.
- **VERIFIED:** Active-chat/backfill resolution classifies `paired`, `stored_pair_recovered`, and `assistant_only`; it reuses an existing stored user row when available and does not fabricate user input for assistant-only history. Explicit normalization reports assistant-only candidates separately.
- **VERIFIED:** The long-memory candidate renderers retained by 4.1.0 no longer apply their former fixed per-item caps before the final Go delivery plan; successful contextualized embeddings discard bulky context chunks before outbox persistence, while failed/empty embeddings keep them for retry.
- **VERIFIED source/Go regression; package evidence is version-specific:** Current priority-enabled auto/custom-budget preparation emits `memory_delivery_plan.v2` with `priority_score.static.v4`. Existing eligible source projections produce request-local atomic facts with per-fact relevance and source/visibility/perspective lineage. One effective query set is shared with Chroma; its recent completed-conversation depth is independently configurable from Chroma result `top_k` and final per-group fact K. A canonically hydrated precise-unit vector hit scores only its matching fact; aggregate-vector score contributes to its complete summary while staying diagnostic for individual facts; speaker/location/storyline matches are bounded score-only biases, and recency uses only RP-turn distance with a floor. Stored importance contributes independently of RP-turn recency; age no longer reduces both terms. Story/RP memory does not age while the user is away. Recalled `memories.turn_summary` rows form a separate complete-summary group scored by the higher of the best child fact and observed aggregate-vector score. The same UI K is applied independently to that group and every scored fact lane without unused-slot transfer; one-sentence exact summary/fact duplicates render once without consuming fact K. Existing custom per-class character budgets apply in the same Go selector, and summaries share `event_recent` with event facts. Identity metadata stays outside fact K, while current resolution operates on facts; array ordinals retain source occurrence so unrelated rows cannot supersede one another merely because both rendered as `item_1`. Lifecycle-bearing plan/progress/completion/follow-up facts remain independent candidates; transitions are diagnostic only and do not affect score. Protected recollection guards remain attached to their memory, unseeded sources keep a non-rejecting rendered-line fallback, and diagnostics distinguish summary/fact candidates and group outcomes. The existing Publisher/Text/PDF paths consume the same final selection, while recent conversation remains search context rather than duplicate payload content. This changes neither source projection, canonical source rows, Chroma authority, JavaScript lifecycle, nor persistence. The 4.2 package records its earlier static.v3 behavior; the 4.3 static.v4 correction is included in test.18. Loaded/live quality verification remains open.
- **VERIFIED source/Go regression:** Critic thread/state JSON now carries one stable `lifecycle_key`. A structured completion emits `complete`/`resolve`, and `state_deltas.resolved_threads` updates the matching stored `pending_threads` row to `resolved`. Delivery deliberately does not let that AI-produced key collapse different stage facts or let a terminal transition preempt relevance, importance, or RP-turn recency. The key is hashed for the physical thread key so non-Latin titles do not collapse through the legacy ASCII `stableKey`. Legacy records without the key retain their exact-title compatibility path; this is a bounded persistence lifecycle repair plus low-confidence delivery lineage, not the planned 4.5 general lifecycle ontology.
- **VERIFIED source/Go regression:** Administrative canonical vector replay is vector-only after core admission. It does not append active states, canonical state layers, pending threads, storylines, or other post-admission projections. A repeated resolved-thread save is idempotent. Existing polluted rows are not silently deleted; rebuild requires an explicit affected-session reset/cold start.
- **VERIFIED source/regression:** 4.2 selectable finalization preserves `응답 직후` by default and implements explicit `다음 사용자 입력 시` through a Go-confirmed policy carried across the actual compact/legacy orchestration-result handoff plus the existing `/complete-turn`; the previous Critic is non-blocking and no parallel persistence owner was added.

### Partially implemented

- **VERIFIED:** Partial thin-adapter boundary. Normal final application consumes a Go plan, but active JavaScript local turn/budget observations, placement, legacy compatibility reads, and protection-only prompt debt remain. The old JavaScript budget assembler and `checkAndAutoRollback()` surface are **OBSOLETE**.
- **VERIFIED:** Partial operational dependency proof. Chroma has preflight/readiness coverage; MariaDB authority lacks a startup/readiness `Ping()`.
- **VERIFIED:** Partial durable asynchronous derivation. The ledgers/workers exist, but processing depends on runtime provider sync and provider availability; source alone does not prove a live queue drains or that every separate typed projection is reconciled after partial commit.
- **VERIFIED:** Partial Host-reference lifecycle. Snapshot application and session delete/migration ownership are implemented, but durable snapshot idempotency, automatic retry, freshness/TTL, wrapper-Store exposure, and current package/live-Host proof remain absent or **UNKNOWN**.
- **VERIFIED:** Source-level worldline flow is connected from the bounded Risu output observation through Go route/source validation to fork-lineage persistence and topology/manual repair. Current loaded-host behavior remains **UNKNOWN**.
- **VERIFIED:** Partial empty/continuation lifecycle. Explicitly observed empty input maps to `[auto-continue]`, but the inspected RisuAI Say Nothing branch bypasses the input handler and exposes only an ordinary stored user row. Generic middle-role deletion currently lacks a stable no-repair/review contract that prevents adjacency-based assistant reassignment before normalization.
- **VERIFIED:** Partial PDF Preview validation. Source/regression, rebuilt Windows package, live MariaDB/Chroma readiness and real-session Go PDF generation are verified. Loaded RisuAI interceptor execution, Google AI Studio/Vertex/LLM Gateway final request bodies, provider acceptance/usage, memory recovery quality and displayed-final effect remain **UNKNOWN**.

### Planned

- **PLANNED — 4.4 behavior-preserving refactoring:** Follow the [file/function plan](docs/archive-center-4.4-refactoring-plan.md) and [integrated 4.4 scope](../_archive/future-reference/4.1-9.0-integrated-roadmap.md#refactoring-consolidation-44). Record the actual starting 4.3 dirty baseline; consolidate existing provider-option mapping and repeated UI/HUD mechanics, type internal assembly inputs and private retrieval results, and measure unused supplemental rendering/repeated input work. Strengthen independent SQL test expectations before any related storage-function move. Shared UI settings and fake-vector read recording are unreproduced concurrency risks, not confirmed data contamination. Preserve active compatibility callers and existing acceptance, privacy, recommendation, settings and lifecycle behavior.

- **PLANNED — 4.4 semantic consolidation:** The original cross-surface claim/event grouping remains 4.4-D, after the refactoring slices. Its intended changes to representative delivery and source coverage require separate cases from refactoring parity. Define the connection to 4.3 before implementation; already-received AI recommendations must not be silently rewritten or replaced. Source-linked compact bundles remain 4.5, with typed relations/local-graph work assigned later in the integrated roadmap.

- **UNKNOWN — remaining 4.3 validation:** Role prompts, UI and calls are no longer merely planned. Their loaded-Host application, actual managed-update setting preservation, provider/Flex acceptance, output effect and timing remain limited to the evidence in the [4.3 summary](docs/archive-center-4.3-status-summary.md). The original `world_state` first-call failure is unresolved without its error record; race-detector and 40M performance proof remain open.

- **OBSOLETE:** `memory_recall_plan.v2` remains only in the older [integrated 3.6–4.1 roadmap](docs/3.6-4.1-precision-long-term-memory-roadmap.md). The reconciled planning index preserves the active `memory_recall_plan.v1`, `memory_delivery_plan.v1`, and `payload_application_plan.v1` contracts; it does not authorize a v2 migration.
- **OBSOLETE:** The “lineage-aware retrieval and invalidation are deferred” statement in [`4.0-risu-worldline-observation-design.md`](docs/4.0-risu-worldline-observation-design.md) is outdated for retrieval: current prepare-turn Store/vector reads use confirmed fork-boundary history segments. Do not infer that every invalidation or live-host case is complete from that retrieval implementation.
- **PLANNED:** The 4.0 goal of zero JavaScript policy calculations and a fully Go-owned final assembly is not complete while the active helpers listed in sections 3 and 17 remain.

### Obsolete, inactive, or abandoned

- **OBSOLETE:** `Archive Center 3.4-C.js` and timestamped adapter backups relative to the active source.
- **OBSOLETE:** `_step18MarkerSurface`, an unreferenced marker object describing removed Python paths, has been removed.
- **OBSOLETE:** The deleted legacy JavaScript budget assembler and `checkAndAutoRollback()` helper are not evidence of current injection or rollback behavior.
- **OBSOLETE:** `publisher_plan.v1` as the active Publisher contract; production source now requires `publisher_plan.v2` and retains v1 only in negative compatibility coverage.
- **OBSOLETE:** The older `memory_injection_plan.v2` proposal as the current 4.0 delivery/application target; the current 4.0 roadmap explicitly retains `memory_delivery_plan.v1` and `payload_application_plan.v1`.
- **OBSOLETE:** Historical `_dist*`, `_release*`, `_runtime*`, and `_test-builds` trees as active source.
- **UNKNOWN:** Whether `AC Recomposer Agent.js` or `AC Ensemble Agent` is currently installed in a live host. The active transient Recomposer bridge proves only the historical optional source integration, not Ensemble integration, installation, package autoload, or displayed-final admission.

## 20. Open Questions and Unverified Areas

1. **UNKNOWN:** Which exact plugin artifact is currently loaded in a real RisuAI instance, and whether the current input/beforeRequest/afterRequest/output callbacks exhibit the source-defined lifecycle there.
2. **UNKNOWN:** Whether the intended `mariadb_authority` deployment has applied the current 4.3 migration inventory through `013` and preserves canonical, identity, worldline, and Host-reference data across restart, rollback, reroll, deletion, migration, and recovery. Earlier 4.2 release evidence remains scoped to its then-current inventory.
3. **UNKNOWN:** Whether the configured Chroma version, session/reference collections, embedder, and outbox exact-readback behavior pass against the intended deployment OS/provider, and whether direct world-rule/status/admin/migration/reference/compatibility mutations reconcile after failure.
4. **UNKNOWN:** Whether the active `publisher_plan.v2` and Critic provider combinations respect timeouts, malformed-response handling, and bounded prompt contracts in production.
5. **UNKNOWN:** Whether complete-turn and vector/reprocessing queues drain correctly during long sessions, process crashes, network partitions, and restart.
6. **UNKNOWN:** Whether native RisuAI context plus Archive Center injection has zero semantic duplicates in real payloads for all supported Risu versions.
7. **UNKNOWN:** Whether all direct maintenance/write routes have the intended deployment authentication and operator audit policy.
8. **UNKNOWN:** Which plugin and backend artifacts are currently loaded in RisuAI. The public 4.2.0 packages match tagged release source `4257081c`; package evidence does not identify the artifact loaded by the Host.
9. **UNKNOWN:** Whether the verified source-level branch observer, Go worldline resolver, and topology/manual-repair UI work end to end in the currently loaded RisuAI/PocketRisu version.
10. **UNKNOWN:** Whether every raw-prefix or post-admission `partial_commit` state is automatically reconciled, especially for KG/narrative/character/status projections outside common admission.
11. **UNKNOWN:** The intended removal milestone for each remaining active JavaScript local turn/budget/placement/legacy helper and the protection-only exception.
12. **UNKNOWN:** Whether the loaded RisuAI/PocketRisu implementation exposes the official lorebook API with the observed shapes, and whether `search_only`/`reference_assist` behaves correctly against real scoped entries.
13. **UNKNOWN:** The intended durable idempotency, retry, and freshness policy for Host lorebook snapshots. Session deletion/migration source ownership is now verified and should not be listed as unresolved.
14. **VERIFIED public package / isolated Windows live update:** The seven 4.2.0 ZIPs passed manifests/checksums and 4.1 preflight. A real public 4.1 backend accepted the same managed-update API request used by the UI, restarted as 4.2.0 and committed, preserving MariaDB chat/memory fixtures, Chroma documents/embeddings/query results, and local settings. See the [release record](docs/archive-center-4.2.0-release-verification.md). Actual RisuAI UI invocation/loading, the user's original-data deployment, and full native installation/update on other devices remain **UNKNOWN**.
15. **UNKNOWN:** Whether the actually loaded RisuAI version preserves the inspected top-level serialization plus inner provider-retry callback loop, whether fallback model switching supplies the same payload shape, and whether an unexpected overlap presents different official correlation evidence.
16. **UNKNOWN:** Whether `memory_injection_baseline.v1` matches a captured real provider payload and whether any listed surface changes the displayed final output; source/regression payload observation is not displayed-final evidence.
17. **UNKNOWN:** Whether the affected user's exact RisuAI build exposes any official blank-submit or synthetic-message origin beyond the inspected stored row, and the exact redacted role/index/hash mapping around the reported Say Nothing deletion. Until then, the literal text is not authoritative Host provenance and the reported worldline count is not source-verified.
18. **UNKNOWN:** Whether the loaded RisuAI body interceptor sends exactly one PDF and zero duplicate selected-memory Text through Google AI Studio, Vertex and LLM Gateway, whether the target Gemini models read the beginning/middle/end and relation direction correctly, and how actual latency/usage compares with Text mode.
19. **UNKNOWN:** Whether the source/regression-verified precise fact similarity, structured score biases, turn-distance recency, complete-turn-summary group, per-fact-lane K, existing UI class budgets and identity-metadata K separation produce the intended memory choice and displayed-final continuity in the user's real long-session MariaDB/ChromaDB state.

## 21. Documentation Maintenance Rules

Update this document in the same bounded change whenever any of the following changes:

- executable, plugin, installer, or worker entry points;
- component ownership or the RisuAI-host/Go-backend boundary;
- RisuAI hook names, stages, callback semantics, or payload mutation timing;
- API routes, DTOs, contract versions, hashes, ordering, reason/error codes, or compatibility windows;
- source acceptance, complete-turn, any section-11 persistent writer, auxiliary non-canonical writer, transaction boundary, idempotency, partial-commit reporting, or recovery path;
- MariaDB schema, migration policy, canonical/reference table ownership, Host-reference lifecycle, or source-revision fences;
- vector query, hydration, index document, outbox, readback, retry, or compensation behavior;
- retrieval eligibility, duplicate suppression, lane ordering, coverage, context/token/character budgets, or payload rendering;
- queue/job types, leases, retry limits, terminal states, wakeup/config-sync rules, or crash recovery;
- runtime/store/vector profiles, feature flags, auth, dependency readiness, or fallback/degraded semantics;
- active prompts or external provider ownership;
- generated package layout, build inputs, manifests, updater/schema sequencing, or release verification;
- an implemented feature supersedes a roadmap contract, or an active path becomes obsolete.

Maintenance procedure:

1. Inspect active implementation first; do not promote roadmap prose or copied outputs to current behavior.
2. Update diagrams, matrices, flows, contract versions, and risk classifications together.
3. Link every important claim to a current path and symbol/route/hook when possible.
4. Keep MariaDB canonical records separate from Chroma indexes and local caches.
5. State source, automated test, built package, loaded RisuAI, live dependency, provider/OS, and release evidence as separate claims.
6. Re-run the relevant contract/unit/integration tests, `node --check "Archive Center.js"` for adapter changes, package validation for packaging changes, and live Risu/dependency checks when the claim requires them.
7. Review the final diff and confirm generated outputs and unrelated dirty files were not edited.

### 4.2 release validation expansion (2026-09-05)

The existing CI now runs the production POSIX fresh-install contract and native
updater transaction tests on Ubuntu and macOS, in addition to the existing
Windows fresh-install/generated-package contract. These tests isolate external
download/start boundaries; they are not proof of full native MariaDB/Chroma
installation on every device. All seven release archives remain owned by
`ops/build-release-assets.ps1`. The RisuAI plugin update and backend managed
package update remain separate operations.

All four jobs passed for tagged release source `4257081c`. The seven ZIPs and
checksum list are public as latest stable `v4.2.0`; uploaded asset digests match
the final clean-source build. A separate real public 4.1-to-4.2 Windows managed
update reached readiness and commit while preserving real MariaDB/Chroma test
data and local configuration. See [the release evidence](docs/archive-center-4.2.0-release-verification.md)
for exact boundaries; this does not close loaded-RisuAI or all-device questions.
