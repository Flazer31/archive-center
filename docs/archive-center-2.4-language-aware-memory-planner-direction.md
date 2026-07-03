# Archive Center 2.4 Direction - Language-Aware Memory + Planner Baseline

Status: 2.4 RC6 release candidate prepared; language-aware memory, identity/secret guard, plugin offload, vector integrity, reindex/orphan/dedupe maintenance implemented
Target: Archive Center 2.4

## Purpose

Archive Center 2.4 should combine the Step 25 planner/progression baseline with language-aware long-memory behavior.

The core problem is that many users do not keep input language and output language the same. A user may type Korean while asking the model to answer in English or Japanese. If Archive Center stores summaries, rules, memory capsules, and vector text in mixed languages without a contract, ChromaDB retrieval can become less stable and Effective Input can look inconsistent.

2.4 should therefore separate three ideas:

- Raw evidence language: the original user/assistant text must remain unchanged.
- Output/session language: generated summaries and injected support should follow the user's active output language.
- Cross-language retrieval: ChromaDB search text should preserve enough raw evidence and aliases to match across Korean, English, Japanese, and mixed sessions.

## Non-Goals

- Do not translate or rewrite raw chat logs, direct evidence, or quoted facts.
- Do not make planner output a canonical truth writer.
- Do not solve full user profiling here; that remains a later profile substrate concern.
- Do not add larger prompt blocks just to carry language metadata.
- Do not hardcode one language as the default memory language.

## 25th Step Carry-In

Step 25 defines the input-intent and planner/progression contract layer. For 2.4, its ideas should be applied as follows:

- Input-intent resolution decides what the user actually asked, independent of UI/meta/preset residue.
- Weak-input planner contract gives bounded initiative when the user input is thin, but never overrides the current user input.
- Planner/execution slots stay inspectable: scene mandate, required outcome, forbidden move, pacing pressure, ending requirement.
- Progression choice remains support-only: advance, callback, new scene opportunity, or hold.
- Validation must prove that planner support improved continuity without becoming truth authority.

2.4 adds one more contract to that layer: the planner and memory writer must know which language each lane is allowed to use.

## Language Contract

Each stored or indexed item should be treated as one of these lanes:

- `raw_evidence`: original text, never translated.
- `canonical_summary`: generated summary in the session output language.
- `search_text`: ChromaDB-facing text that combines canonical summary, raw evidence, and aliases.
- `display_text`: UI-facing text in UI language or session output language, depending on surface.
- `internal_key`: stable internal enum/key/category, usually language-neutral or English.
- `protected_secret`: owner-scoped or audience-scoped private knowledge such as secret crushes, mistakes, shame, guilt, small lies, private fears, hidden plans, identity twists, hidden roles, or succession/power inheritance. It must guide continuity without causing spontaneous confession or impossible discovery.
- `character_identity_accuracy`: internal identity/role/allegiance/succession mapping for cover identities, fake names, disguises, betrayal status, hidden true roles, unrevealed monster/demon-king identities, royal/heir status, secret successors, hidden power inheritance, undercover roles, or other twist identities. It must guide continuity without forcing narrator/player reveal.

Recommended shape:

```text
raw_evidence: original text
canonical_summary: output-language summary
search_text: canonical_summary + raw_evidence + aliases
raw_language: ko/en/ja/unknown
summary_language: ko/en/ja/unknown
session_output_language: ko/en/ja/auto
```

## Character Identity Accuracy Contract

2.4 must treat character identity accuracy as a first-class memory contract. A stored sentence such as "Lia is Gloria's maid cover identity" is not enough by itself; it must become an internal identity mapping that prevents split-character drift while preserving reveal boundaries.

This is broader than fake names. Fiction commonly uses hidden identity and hidden role structures: a trusted ally is actually a traitor, a harmless NPC is actually the demon king, a maid is actually a noble, a companion is undercover, a protagonist is the secret successor to a protected power or bloodline, or a character's public loyalty differs from their private allegiance. Archive Center must preserve the accurate internal identity without spoiling the reveal.

Recommended shape:

```json
{
  "contract_version": "character_identity_accuracy.v1",
  "canonical_entity_key": "gloria",
  "canonical_entity_name": "Gloria",
  "surface_identity_name": "Lia",
  "true_identity_name": "Gloria",
  "identity_kind": "cover_identity|fake_identity|disguise|stage_name|mistaken_identity|hidden_role|hidden_allegiance|traitor|undercover|true_form|hidden_heir|secret_successor|secret_inheritor|hidden_power_inheritance|hidden_monster",
  "same_entity": true,
  "protected_secret_type": "identity|role|allegiance|succession|power_inheritance|lineage|mission",
  "public_role": "maid",
  "true_role": "lord",
  "public_allegiance": "unknown",
  "true_allegiance": "gloria_house",
  "twist_sensitivity": "low|medium|high|critical",
  "reveal_policy": "explicit_user_reveal_required|current_session_confirmation_required|owner_private_until_revealed",
  "visibility": "internal_support_only|current_scene_surface_name",
  "knowledge_scope": {
    "publicly_revealed": false,
    "known_by": ["Gloria"],
    "unknown_to": ["Siwoo"],
    "suspected_by": [],
    "misinformed_by": [],
    "revealed_to": ["Gloria"],
    "reader_visible": false,
    "protagonist_visible": false
  },
  "source_evidence_turns": [1],
  "raw_evidence_rewritten": false
}
```

Rules:

- Canonical storage should know that the two names point to the same entity when direct evidence supports it.
- Prompt injection should not casually reveal the true identity. It should use the current scene surface name unless the current user input, current direct evidence, or owning character dialogue/action reveals it.
- KG/entity/character-state writes must avoid creating contradictory separate-person facts when a same-entity alias is active.
- Hidden role/allegiance facts must not overwrite public-facing role/state unless the reveal is current and explicit.
- Secret succession, hidden lineage, hidden mission, and protected power inheritance facts must be treated as protected identity facts even when they do not change the character's visible name.
- ChromaDB search text may include both names as aliases, but metadata/trace must keep the reveal policy visible.
- Private recollection may use the mapping for subtext, hesitation, recognition, avoidance, or selective silence; it must not narrate the secret as public truth.
- If evidence is ambiguous, hold as an alias candidate instead of merging automatically.

### Per-Character Knowledge and Reveal Audience

Identity truth and character knowledge must be separate lanes. A secret can be canonically true while still being unknown to the protagonist, unknown to the current speaker, suspected by one NPC, and fully known by another NPC. Archive Center must not collapse that into a single global "revealed" flag.

Rules:

- `known_by`, `unknown_to`, `suspected_by`, `misinformed_by`, and `revealed_to` are scoped to characters or audience groups, not to the entire session.
- Subjective memory belongs to its owner. It can guide that owner's subtext, recognition, suspicion, avoidance, or deception, but it must not become public narration or protagonist knowledge by itself.
- If a secret is revealed only to some characters, injection must include a compact actor-aware guard: who can act on the secret, who cannot, and who only suspects it.
- Character dialogue/action may use facts known to that character, but other characters must not react as if they know unless current evidence reveals it to them.
- A protagonist or NPC must not suddenly confess a protected secret merely because Archive Center stored it. Stored truth is not permission for self-disclosure.
- A narrator/public reveal requires current user instruction, current direct evidence, or an explicit reveal event in the current scene.
- Reviewer/table-read lanes may inspect protected knowledge for consistency, but they must return leak warnings/protected-reveal markers rather than rewriting the secret into visible output.
- ChromaDB may retrieve private or subjective memories as candidates, but hydration and injection must filter them through the MariaDB knowledge scope before they reach the final support lane.

## General Protected Secret Contract

2.4 must protect all secrets, not only large twist identities. A secret crush, a hidden mistake, embarrassment, guilt, debt, fear, small lie, private promise, hidden plan, or shameful memory can be story-critical even when it is easy to discover later. Archive Center must preserve the fact for continuity while preventing impossible disclosure.

Recommended shape:

```json
{
  "contract_version": "protected_secret.v1",
  "secret_key": "stable-session-secret-key",
  "secret_kind": "romantic_feeling|mistake|embarrassment|guilt|lie|fear|weakness|debt|private_promise|hidden_plan|identity|role|allegiance|succession|power_inheritance|lineage|mission|other",
  "owner": "character_or_group",
  "subject": ["optional_target_character_or_topic"],
  "sensitivity": "low|medium|high|critical",
  "evidence_strength": "direct|inferred|rumor|suspected",
  "disclosure_policy": "owner_may_reveal_when_scene_supports|explicit_reveal_event_required|user_directed_reveal_only",
  "knowledge_scope": {
    "publicly_revealed": false,
    "known_by": [],
    "unknown_to": [],
    "suspected_by": [],
    "misinformed_by": [],
    "revealed_to": []
  },
  "source_evidence_turns": [],
  "raw_evidence_rewritten": false
}
```

Rules:

- Stored secret truth is continuity support, not permission for the owner to confess it.
- Other characters may notice, infer, suspect, or discover a secret only when current-scene evidence supports that step.
- Easy-to-discover secrets are not auto-discovered. Archive Center may mark them as low sensitivity or inferable, but discovery still needs scene evidence.
- The owner may act with subtext, hesitation, avoidance, overcompensation, nervousness, or selective silence without stating the secret.
- Secret retrieval must not put the secret as the first visible instruction to the main model unless the current scene calls for reveal. It should be compact support metadata with a reveal guard.
- ChromaDB can retrieve protected secrets semantically, but MariaDB knowledge scope decides whether the current lane may inject the fact, a masked hint, or only a leak warning.
- Debug surfaces may show counts, kinds, sensitivity, and reveal policy. They must avoid dumping protected secret text into public-facing lanes unless debug mode is explicitly inspecting protected details.

## 2.4-0 Contract: Field Names and Fallbacks

2.4-0 is contract-only. It must not change runtime behavior, write new DB rows, reindex ChromaDB, or rewrite existing memories. Its job is to freeze names and debug surfaces so later implementation slices do not invent incompatible shapes.

### Language Context Object

Every turn-level trace that participates in memory write, ChromaDB indexing, Effective Input assembly, or planner support should be able to expose this object:

```json
{
  "contract_version": "language_memory.v1",
  "session_output_language": "ko|en|ja|auto|unknown",
  "output_language_source": "explicit_override|plugin_setting|recent_assistant|ui_language|auto_unknown",
  "ui_language": "ko|en|ja|unknown",
  "raw_user_language": "ko|en|ja|mixed|unknown",
  "assistant_output_language": "ko|en|ja|mixed|unknown",
  "summary_language": "ko|en|ja|auto|unknown",
  "search_text_policy": "summary_plus_raw_plus_aliases",
  "locked_for_turn": true,
  "confidence": 0.0
}
```

Field rules:

- `session_output_language` is the target language for generated memory summaries and injected support text.
- `output_language_source` records why that language was selected.
- `ui_language` is only the settings/dashboard display language. It must not silently become memory language unless no better signal exists.
- `raw_user_language` describes the current user input only; it must not override an explicit output language.
- `assistant_output_language` can be used as a fallback after the current turn has completed.
- `summary_language` should normally equal `session_output_language`.
- `search_text_policy` must stay descriptive, not a hidden behavior switch.
- `locked_for_turn` means the selected output language cannot drift during one request/complete-turn pair.
- `confidence` is diagnostic only. It must not block saving by itself.

### Output Language Fallback Order

Resolve `session_output_language` in this order:

1. `output_language_override` from the current request, if explicit and valid.
2. A future dedicated plugin setting for response/memory output language, if present.
3. Recent assistant output language from the same active chat, if confident.
4. `uiLanguage`, but only as a weak fallback and only when no output signal exists.
5. `auto` / `unknown`.

Do not resolve output language from a single short user input when a stronger output-language signal exists. Korean input with English output should remain English for summaries and injected support.

### Lane-to-Storage Contract

```text
chat_logs.content              -> raw_evidence, original language only
effective_input_logs           -> final assembled input, may contain mixed lanes but must label them
direct_evidence_records        -> raw_evidence, original language only
memories.summary_json          -> canonical_summary plus language metadata
kg/rule/status display summary -> output-language display/support text
internal category/key fields   -> stable internal keys, not translated per turn
ChromaDB document text         -> search_text
ChromaDB metadata              -> language_context subset + canonical row id
```

Existing rows must not be rewritten just because the user changes output language later. New language-aware summaries are additive or generated on the next write/reindex path only.

## ChromaDB Direction

ChromaDB should not be left to guess from one mixed-language summary. It should receive an intentional `search_text`:

```text
search_text =
  canonical_summary in output language
  + raw evidence in original language
  + key aliases for names, relationships, places, and major actions
```

This lets Korean input retrieve English/Japanese session memories, and English/Japanese output still receive memories in the user's requested response language.

MariaDB remains the canonical row authority. ChromaDB is the semantic candidate finder. A vector hit still needs MariaDB hydration before injection.

## 2.4-0 Debug Surface Shape

The first implementation-visible result of 2.4 should be traceability, not behavior change. Debug surfaces must let the user answer: "which language did Archive Center use, why, and what reached ChromaDB/Effective Input?"

### Dashboard

Add a compact language row when 2.4 language tracing exists:

```text
Language
session: en / source: explicit_override / raw user: ko / summary: en
```

If ChromaDB language-aware indexing is active in a later slice, the existing vector counters should expand without replacing current counters:

```text
검색
정상 [10건] — vec found:10 hydrated:8 injected:5 / lang summary:en raw:ko alias:on
```

### Effective Input

Effective Input should show a read-only language section before `Related Memories` once implemented:

```text
━━ Language Context ━━
session_output_language: en
output_language_source: explicit_override
raw_user_language: ko
summary_language: en
search_text_policy: summary_plus_raw_plus_aliases
```

This section is diagnostic/support text. It must not replace the user input or raw evidence.

### Input Transparency

Input Transparency should expose the same facts in a compact checklist:

```text
Language Context
- output: en (explicit_override)
- raw user: ko
- summaries: en
- Chroma search text: summary + raw + aliases
- raw evidence rewrite: no
```

Related Memories should be allowed to show per-item language tags:

```text
[Memory] turn 12 · summary:en · raw:ko · vector:hydrated
```

### Debug Tab

Debug mode should get a read-only `Language Contract Trace` block:

```json
{
  "contract_version": "language_memory.v1",
  "fallback_chain": [
    {"source": "explicit_override", "value": "en", "used": true},
    {"source": "plugin_setting", "value": null, "used": false},
    {"source": "recent_assistant", "value": "en", "used": false},
    {"source": "ui_language", "value": "ko", "used": false}
  ],
  "violations": [],
  "raw_evidence_rewritten": false
}
```

The same debug area should include a small world-rule count trace when rule counts are present, because users can confuse category count with total count:

```text
World Rule Count Trace
header_count_source: active_category
active_category: global
active_category_count: 1
total_count: 17
display_warning: header is category count, not total count
```

This is only a visibility contract in 2.4-0. The actual UI count correction can be handled as a small follow-up slice.

### Backend Trace Shape

Backend responses that already return prepare/search/complete-turn traces may add a `language_context` object later. 2.4-0 freezes the intended location:

```json
{
  "trace": {
    "language_context": {},
    "vector_counts": {
      "found": 0,
      "hydrated": 0,
      "injected": 0,
      "language_matched": 0,
      "alias_matched": 0
    }
  }
}
```

`language_context` must be read-only trace data. It must not be a new authority lane.

## 2.4 Work Order

### 2.4-0 Contract Document

Define the lane contract for raw evidence, canonical summary, search text, display text, and internal keys.

Acceptance:

- Raw evidence preservation is explicit.
- Output-language summaries are support/injection text, not replacements for evidence.
- Cross-language search text is defined without hardcoded language assumptions.
- Debug surface shape is fixed for Dashboard, Effective Input, Input Transparency, and Debug tab.
- No runtime behavior, DB migration, or ChromaDB reindex is required in this slice.

### 2.4-1 Language Detection and Source of Truth

Define where output language comes from:

- explicit output language override if present,
- plugin/user setting if present,
- recent assistant output language as fallback,
- unknown/auto if no reliable signal exists.

Acceptance:

- The system does not infer language from a single short user input when an explicit output language exists.
- Debug surfaces show resolved `session_output_language`.

Implementation notes:

- `Archive Center.js` builds a turn-level `language_context` object using the 2.4-0 fallback order.
- The current user input is recorded as `raw_user_language` only; it is not used to override the session output language.
- `/prepare-turn` and `/complete-turn` carry the same read-only language context through `client_meta.language_context`.
- Last Turn Trace, Input Transparency, Effective Input, and debug preview can show the resolved language contract.
- Effective Input shows the language contract as trace-only metadata, not as injected auxiliary prompt text.
- This slice does not translate raw evidence, rewrite existing DB rows, reindex ChromaDB, or change memory extraction output.

### 2.4-2 Memory Write Contract

Apply the lane contract to memory extraction outputs.

Acceptance:

- Chat logs and direct evidence keep original language.
- Memory summaries follow session output language when generated.
- KG/rule/status display summaries follow session output language, while internal keys remain stable.
- No existing evidence row is rewritten just because output language changed later.

Implementation notes:

- `/complete-turn` reads `client_meta.language_context` and forwards it to the critic extraction path.
- Critic prompts include a read-only `Language_Context_JSON` block.
- The prompt tells the critic to use summary/session language for generated summaries and display/support fields, while keeping `evidence_excerpts` as exact source text.
- Backend normalization attaches `language_context` and `memory_write_contract` to `memories.summary_json`.
- Direct evidence rows keep `evidence_text` unchanged and record `lane=raw_evidence`, `raw_evidence_rewritten=false`, and `language_context` inside `lineage_json`.
- ChromaDB cross-language `search_text` is handled by `2.4-3`, not by this slice.
- This slice does not rewrite or backfill existing rows.

### 2.4-3 ChromaDB Cross-Language Indexing

Index ChromaDB with a composed search text instead of only one summary field.

Acceptance:

- Search text contains canonical summary, raw evidence, and aliases.
- Vector metadata records raw/summary/session language.
- Effective Input / dashboard can show vector found, hydrated, injected, and language lane counts.

Implementation notes:

- New memory vector upserts build ChromaDB `documentText` from labeled `Canonical Summary`, `Raw Evidence`, and `Aliases` sections.
- New memory embeddings are generated from the composed search text, not from `turn_summary` alone.
- MariaDB remains canonical. `memories.summary_json` and `memories.evidence` are read to reconstruct search text; no new memory DB column is required for this slice.
- ChromaDB metadata records `search_text_policy`, `raw_language`, `summary_language`, `session_output_language`, and `alias_count`.
- Search previews expose the same metadata so debug surfaces can confirm whether vector hits came from language-aware rows.
- Vector hydration trace and memory lane counters now expose hit/hydrated language-context and alias-ready counts.
- Admin reindex uses the same composed search text. Existing embeddings are only regenerated when the existing reindex force/missing-embedding policy requests it; this avoids surprise bulk embedding cost.
- This slice does not translate existing rows, backfill old language metadata, or inject status/planner text into Effective Input.

### 2.4-4 Planner + Language-Aware Injection

Connect Step 25 planner/execution contracts to language-aware memory.

Acceptance:

- Planner support uses the session output language.
- Current user input remains highest priority regardless of language.
- Related memories injected into the main model are readable in the output language, while retaining direct evidence when needed.

Implemented in this slice:

- `/prepare-turn` consumes `client_meta.language_context` and returns it at the top level, in `supervisor_input_pack`, and in `injection_pack`.
- `supervisor_input_pack.planner_language_contract` exposes `planner_support_language`, language source, raw-input priority, and raw-evidence preservation without hardcoded character or story examples.
- `injection_pack.language_injection_trace` exposes session output language, summary language target, current-user priority, raw evidence preservation, and translation-call status.
- Related memory lines prefer the stored canonical/output-language summary. When stored summary language and raw evidence language differ, raw evidence is attached as raw evidence, not translated or rewritten.
- Injection counts expose `language_aware_injection`, memory summary language match/mismatch, and raw evidence attachment counts.

### 2.4-5 Character Identity Accuracy + Protected Secret Guard

Prevent cover identities, fake names, disguises, hidden roles, hidden allegiance, betrayal status, secret succession, hidden power inheritance, private feelings, hidden mistakes, shame, guilt, small lies, and unrevealed true identities from becoming separate characters, wrong public facts, spontaneous confessions, impossible discoveries, or leaked narrator truth.

Example failure this slice must prevent:

```text
Gloria and Lia are the same person.
Lia is Gloria's temporary maid cover identity / reality-escape persona.
Archive Center must not later inject them as two separate people or let the main model output them as separate characters.
Archive Center must also not reveal "Lia is Gloria" as public narration unless current evidence/user instruction allows it.
```

Other covered examples:

```text
The trusted knight is secretly a traitor.
The harmless merchant is actually the demon king.
The maid identity is a noble's cover identity.
The companion is undercover and their public allegiance differs from their true allegiance.
A protagonist is the secret successor to a protected power, bloodline, or mission.
A character has a secret crush and should not confess it or have others know it without scene evidence.
A character hides a mistake, embarrassment, fear, debt, or small lie.
```

Acceptance:

- Direct evidence such as "Lia talked about when she was Gloria" can create a `character_identity_accuracy` candidate.
- Direct evidence such as "the knight is the traitor" can create a hidden role/allegiance candidate without making that fact public narration.
- Direct evidence such as "the protagonist inherited the protected power" can create a secret succession/power-inheritance candidate without making the protagonist confess it or other characters know it.
- Direct evidence such as "she secretly likes him" or "he hid his mistake" can create a `protected_secret` candidate without forcing confession, discovery, or public narration.
- A confirmed identity map links aliases/roles/allegiances to one canonical internal entity for memory, KG, character state, subjective memory owner, and ChromaDB alias search.
- Prompt injection uses the current scene surface identity by default, while carrying an internal same-entity guard.
- The injected guard must say "same entity / hidden role / reveal blocked" rather than exposing the true identity as ordinary public fact.
- Existing private-memory guard remains support-only and cannot turn the secret into narrator truth.
- Partial reveal is represented by actor/audience scope, not by one global revealed/unrevealed boolean.
- `known_by`, `unknown_to`, `suspected_by`, and `misinformed_by` affect dialogue, action, and subtext without granting the same knowledge to every character.
- Subjective entity memories remain owner-scoped and are never promoted to public/canonical narration unless current reveal evidence allows it.
- Low-sensitivity secrets may be easier to infer, but Archive Center must still require current-scene evidence before another character knows or exposes them.
- Owner behavior may show subtext, hesitation, avoidance, or nervousness, but the secret itself must remain guarded until reveal conditions are met.
- If evidence is ambiguous, Archive Center stores an alias candidate and blocks automatic merge until stronger current-session evidence or user confirmation exists.
- Effective Input / debug trace can show alias counts and reveal policy without dumping the secret into the main narrative lane.
- No hardcoded character-specific aliases such as `Gloria -> Lia`; the mechanism must be evidence-driven and session-specific.

Implementation notes:

- Add a backend-owned identity alias lane instead of expanding `Archive Center.js`.
- Add a backend-owned knowledge audience lane for identity facts, hidden roles, hidden allegiances, and subjective memories.
- Add a backend-owned protected secret lane for minor and major secrets alike.
- Treat secret successor, hidden lineage, hidden mission, and protected power inheritance as protected knowledge facts even when no alias merge is required.
- Use MariaDB as canonical authority for confirmed/candidate alias mappings.
- Reuse ChromaDB as a candidate finder, but hydrate to MariaDB before injecting.
- Extend canonicalization so `canonicalCharacterName`, entity save, KG save, character state save, and subjective entity owner save can consult confirmed aliases.
- Extend injection assembly so identity/role/allegiance accuracy is compact support metadata, not visible story exposition.
- Extend private recollection and input-context assembly so they carry owner visibility and reveal audience, not a generic private-memory block.

Implemented in this slice:

- The critic extraction contract now accepts `protected_secrets` and `character_identity_accuracy` without hardcoded character aliases.
- Protected secrets normalize into owner-scoped subjective memories with `secret_guard`, `owner_private`, reveal policy, and protected-secret tags.
- Character identity accuracy candidates normalize into protected owner-scoped support memory so same-entity or hidden-role continuity can be preserved without public reveal.
- Related-memory injection masks protected secret and protected identity content into a generic continuity guard instead of injecting the raw secret summary.
- Persona and character-private recollection lanes mask `secret_guard` content into owner-subtext guidance instead of exposing the secret phrase itself.
- Confirmed same-turn `character_identity_accuracy.same_entity=true` mappings now canonicalize saved character entities, KG subject/object names, character state owner names, protected-secret scope names, and subjective memory owner keys without rewriting raw direct evidence.
- Canonicalized entity saves preserve the surface identity as an alias, and memory `SummaryJSON` records a compact `confirmed_identity_alias_canonical_merge` trace for later audit/search.

Deferred before release/replay:

- A persistent confirmed-alias registry/import path is still not released; the current gate applies confirmed same-turn extraction mappings during artifact save and keeps ChromaDB alias search text available through the memory row.
- Effective Input/debug should expose protected-secret and identity-accuracy counts without dumping secret content into the main narrative lane.
- Replay coverage must prove same-entity continuity, partial reveal, and owner/audience knowledge scope before 2.4 release.

### 2.4-6 Replay and Regression Gate

Create replay cases for mixed-language users.

Minimum cases:

- Korean input, Korean output.
- Korean input, English output.
- Korean input, Japanese output.
- English input, Japanese output.
- User changes output language mid-session.
- Same event stored once, then retrieved from another language query.
- Secret identity / cover identity: Gloria is Lia, but Lia should remain the current surface name until revealed.
- Hidden role: a trusted ally is actually a traitor, but the narrator must not reveal it before current reveal evidence.
- Hidden true form: a harmless surface identity is actually the demon king, but public role and true role must not collapse into a wrong visible fact.
- Hidden allegiance: public allegiance and true allegiance differ, and Archive Center must keep both lanes without contradiction.
- Secret successor: a protagonist has inherited a protected power or lineage, but one later turn must not make the protagonist confess it or make unrelated characters know it.
- Partial reveal: Gloria knows that Lia is Gloria, Siwoo does not, and a third NPC only suspects it.
- Secret crush: one character privately likes another, but the owner does not confess and the target does not know without current evidence.
- Hidden mistake/shame: a character hides a mistake, embarrassing habit, fear, debt, or small lie, and another character does not magically discover it.
- Subjective misinformation: one character believes a false identity while canonical truth remains separate.
- Alias ambiguity: two similar names are not merged without direct evidence.

Acceptance:

- ChromaDB does not fall back to recent-only behavior when semantic hits exist.
- TopK is respected.
- Hydrated MariaDB rows match the vector hits.
- Raw evidence remains unchanged.
- Injected summaries are not language-mixed unless evidence quotation requires it.
- Confirmed character identity mappings prevent split-character output.
- Character identity mappings do not leak as narrator/public truth without current reveal evidence.
- Hidden role/allegiance facts improve continuity without overwriting public-facing role/state.
- Secret successor/power-inheritance facts remain available for continuity while staying blocked from spontaneous confession or unrelated-character knowledge.
- Partial reveal replay keeps `known_by`, `unknown_to`, `suspected_by`, and `misinformed_by` behavior distinct.
- Characters do not act on secrets they do not know; characters who know or suspect the secret may show consistent subtext without forcing public reveal.
- Minor protected secrets keep owner subtext without forcing confession or impossible discovery.
- Discovery requires current evidence; semantic recall alone is not discovery evidence.
- Ambiguous identity clues remain candidates and do not force a merge.

Implemented in this slice:

- Added `TestArchiveCenter24ReplayRegressionGate` as the 2.4 replay gate.
- Replay covers Korean/Korean, Korean/English, Korean/Japanese, English/Japanese, and mid-session output-language change contracts.
- Replay verifies raw evidence remains unchanged while generated support/summary lanes follow the session output language.
- ChromaDB memory search text now indexes protected-secret and character-identity candidate aliases, scope actors, roles, allegiances, and reveal policy as internal search text.
- Replay verifies identity aliases can feed vector search while main prompt injection masks true identity content behind `Protected continuity guard`.
- Replay verifies `client_meta.perspective_context.current_pov` can unlock a POV-scoped same-entity guard for a knowledge holder, so the viewpoint owner treats their protected surface identity as self rather than a separate character.
- Replay verifies partial reveal scope counts are preserved for `known_by` and `suspected_by` without exposing `unknown_to` or secret text as narrator truth.
- Replay verifies protected power-inheritance/succession-style secrets remain available to recall while blocked from spontaneous confession or unrelated-character discovery.
- Replay verifies vector-first topK does not fill remaining prompt slots with unrelated recent memories when semantic vector hits exist.
- Replay verifies confirmed identity alias canonical merge saves one internal character lane for Entity, KG, CharacterState, and owner-scoped subjective memory while preserving raw direct evidence and surface identity aliases.

Still gated:

- Persistent confirmed identity alias registry/import and dedicated debug counters are not yet released. The current gate handles confirmed same-turn extraction mappings during save; it does not yet provide a separate operator-approved alias table or UI management surface.
- LLM-output behavior still needs scenario replay before final 2.4 packaging, because these backend gates prove prompt/support construction, not the external model's narrative compliance.

### 2.4-7 Plugin Slimming and Backend Offload

Archive Center exists to reduce RisuAI-side burden while improving long-memory recall. If `Archive Center.js` keeps absorbing orchestration, formatting, rollback, and debug logic, the plugin becomes a second backend inside the browser and can hurt responsiveness. 2.4 must therefore include an explicit slimming pass before release.

Baseline at the time this track was recorded:

```text
Archive Center.js: about 2.64 MB / 47,250 lines
```

Keep in `Archive Center.js`:

- RisuAI hook integration and request/response observation.
- Last-mile payload insertion, because only the plugin can mutate the live Risu request payload.
- Settings UI, dashboard rendering, and compact debug rendering.
- Minimal bridge/client configuration and retry queue handoff.
- Active chat observation needed to tell the backend what Risu currently shows.

Move toward backend ownership:

- Auxiliary context assembly and budget planning now handled around `buildInputContext`, `assembleInjectionWithBudget`, and related block formatters.
- Storyline/world/character/pending-thread formatting that can be returned as backend-prepared support lanes.
- Language contract normalization after the plugin sends raw hints; backend should become the source that writes memory metadata.
- Continuity pack merging, old-arc guard decisions, wake-up support text, and Chroma/MariaDB hydration traces.
- Rollback/tail-delete cleanup planning after the plugin sends the current chat snapshot; the backend should return a deletion/cleanup plan rather than forcing the plugin to own all invalidation logic.
- Debug trace shaping for Effective Input and vector lane counters; the plugin should render compact backend trace objects instead of rebuilding meaning locally.

Do not move in this slice:

- Any behavior that requires direct access to the in-flight Risu payload.
- DOM-only settings/dashboard rendering.
- Local browser storage compatibility code that protects users before the backend is reachable.
- Broad rewrites or file-splitting without a before/after size and behavior check.

Acceptance:

- Produce an offload inventory with function groups, current line ranges, target backend route/service, and risk level.
- Pick one bounded offload candidate only after the inventory is reviewed.
- The first code-moving slice must reduce or stabilize `Archive Center.js` size; it must not add another large parallel path.
- Effective Input output before/after must remain equivalent except for intentional bug fixes.
- `node --check "Archive Center.js"` must pass after every plugin change.
- Backend tests or focused route tests must cover any moved assembly/trace behavior.
- No hardcoded topK, token budget, timeout, or language behavior may be introduced while moving logic.

Implemented in this slice:

- Added `docs/archive-center-2.4-plugin-slimming-offload-inventory.md` as the 2.4-7 offload inventory.
- Classified plugin-only ownership: Risu hooks, live payload mutation, active chat observation, DOM settings/dashboard rendering, and bridge fail-open storage.
- Classified backend-offload candidates with current plugin line ranges, target backend route/service, and risk level.
- Selected the first bounded code candidate as backend-owned Input Transparency / Effective Input render model, exposed through `/prepare-turn` as read-only display-safe blocks and counters.
- Kept actual code movement out of this inventory slice so `Archive Center.js` does not gain another parallel path before the offload contract is reviewed.

2.4-7 backend offload slice:

- Implemented `input_transparency_model` and `effective_input_preview` in `/prepare-turn` response.
- Added a focused backend test for display-safe block order, vector found/hydrated/injected counts, and protected-secret/identity counts without secret-body leakage.
- `Archive Center.js` now renders the backend model when present while preserving local fallback for backend-off sessions.
- Redundant local trace formatting remains only as fail-open fallback and should be deleted gradually after live parity confirms no visibility lane was lost.

### 2.4 RC6 Vector Integrity and Maintenance Completion

This RC closes the remaining 2.4 maintenance gaps found during live testing: old rows not being present in ChromaDB, rollback leaving possible vector orphans, and injection-time duplicate collapse not cleaning duplicate DB rows.

Implemented in this slice:

- `/admin/reindex` now reports and processes eligible existing `direct_evidence_records` and `world_rules` rows in addition to `memories`.
- Reindex integrity now compares ChromaDB count against the full canonical vector candidate set, not memory rows alone.
- ChromaDB vector store now supports diagnostic document listing through `DocumentLister`.
- `/admin/vector-orphan-audit` performs full session document listing and compares ChromaDB documents against MariaDB canonical row references.
- `/admin/vector-orphan-audit` is audit-only by default. It deletes only when `delete_orphans:true` is explicitly provided and the store is mutation-enabled.
- Rollback now attaches a full post-rollback orphan audit when the vector store supports document listing; otherwise it keeps the bounded count check.
- Explorer direct-evidence deletion now attempts to remove matching `evidence:<session>:<id>` vector documents.
- World-rule deletion now attempts to remove matching `world_rule:<session>:<id>` vector documents when `chat_session_id` is provided.
- `/admin/dedupe-cleanup` audits duplicate `memories`, `storylines`, and `world_rules` rows.
- `/admin/dedupe-cleanup` is dry-run by default. It deletes DB rows only when `apply:true` is explicitly provided.
- Dedupe cleanup removes matching memory/world-rule vector documents for rows it actually deletes.
- Added regression coverage for derived-artifact reindex candidates, full orphan audit deletion, and dry-run/apply DB dedupe cleanup.

Operator contract:

```text
POST /admin/reindex
  dry_run:true  -> inspect memory/evidence/world_rule candidates
  dry_run:false -> upsert eligible vectors when embedding settings are configured

POST /admin/vector-orphan-audit
  delete_orphans omitted/false -> full audit only
  delete_orphans:true          -> delete ChromaDB documents that no longer hydrate to MariaDB canonical rows

POST /admin/dedupe-cleanup
  apply omitted/false -> report duplicate DB row candidates only
  apply:true          -> delete duplicate DB rows and matching vectors
```

Release risks and boundaries:

- These maintenance routes are operator/admin surfaces, not automatic prompt-time cleanup.
- Existing rows still need an explicit `/admin/reindex` run to populate ChromaDB after upgrading from older 2.4 RCs.
- Dedupe cleanup intentionally targets exact normalized duplicate anchors. It does not merge near-duplicates or semantic paraphrases.
- Full orphan audit requires a vector backend that supports document listing. ChromaDB supports it; unsupported stores return an unavailable diagnostic instead of guessing.

## Release Gate

2.4 should not be released until these checks pass:

- Raw evidence preservation check.
- Output-language summary check.
- Cross-language retrieval replay.
- ChromaDB found/hydrated/injected count visibility.
- Effective Input language-lane visibility.
- Character identity accuracy replay: cover identity, hidden role, hidden allegiance, and reveal-blocked true identity do not split, leak, or overwrite public-facing facts.
- Partial reveal and subjective knowledge replay: owner-scoped memories and `known_by`/`unknown_to`/`suspected_by`/`misinformed_by` do not leak into global narrator truth.
- Secret successor replay: protected power, lineage, mission, or succession facts do not cause spontaneous confession or unrelated-character knowledge.
- General protected secret replay: secret crushes, hidden mistakes, shame, guilt, fears, debts, promises, plans, and small lies do not become spontaneous confession or impossible discovery.
- Tail deletion rollback still removes MariaDB rows and matching vectors.
- No new hardcoded language or topK behavior outside user/settings policy.
- Plugin slimming inventory completed, with at least one reviewed backend-offload plan or a documented reason to defer code movement.
- Existing direct-evidence/world-rule rows can be reindexed into ChromaDB through `/admin/reindex`.
- Full vector orphan audit can compare ChromaDB documents with MariaDB canonical row references.
- DB duplicate cleanup is available as dry-run-first operator maintenance and does not run implicitly.

## Next Implementation Candidate

The next real work item should be RC6 live replay and release-gate verification before promoting the same code line to final 2.4.

2.4-3 is implemented as backend ChromaDB memory search-text composition, vector metadata, reindex alignment, and vector lane trace visibility. 2.4-4 is implemented as backend prepare-turn language contract propagation into planner support, injection trace, and related-memory language evidence. 2.4-5 has the protected-secret and identity-accuracy guard foundation: extraction contracts, owner-scoped subjective guard memories, masked injection/recollection text, POV-scoped same-entity guidance, and confirmed same-turn canonical merge during artifact save. 2.4-6 adds replay gates for language contracts, protected identity search aliases, masked prompt injection, partial reveal scope, protected power inheritance, vector-first topK behavior, and confirmed identity alias canonicalization of saved artifacts. 2.4-7 now has an offload inventory, backend Input Transparency render model, plugin backend-render adoption, and backend-off local fallback. RC6 adds derived-artifact reindex, full ChromaDB orphan audit, rollback orphan verification, and explicit DB duplicate cleanup. Before final 2.4 release, run live scenario replay and decide whether persistent alias registry/import is required for this release or deferred.
