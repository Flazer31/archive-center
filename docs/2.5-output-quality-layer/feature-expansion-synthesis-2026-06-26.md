# 2.5 Output Quality Layer - Feature Expansion Synthesis

Date: 2026-06-26

Scope: standalone RisuAI output-quality plugin based on MDASH-style multi-agent review. This document intentionally excludes Archive Center DB, long-term memory, backend storage, and migration concerns.

Reference inputs:

- `_reference-do-not-use-as-active/source-plans/multiagent-rp-supervisor-feature-plan.md`
- `_reference-do-not-use-as-active/ai-feature-expansion-notes/kimi-multiagent-rp-director-feature-plan-2026-06-26.md`
- `_reference-do-not-use-as-active/ai-feature-expansion-notes/glm-feature-expansion-2026-06-26.md`
- `_reference-do-not-use-as-active/ai-feature-expansion-notes/deepseek-feature-expansion-2026-06-26.md`
- `_archive/reference/external-examples/example/risu-multiagent.js`
- `_archive/reference/external-examples/example/risu_agents.js`
- `_archive/reference/external-examples/example/Serial_Gradation_Agents_for_RP.js`
- `_archive/reference/external-examples/example/multiagent-full-v0.8.1`

The reference files above are source material only. This synthesis and
`2.5-standalone-output-quality-layer-plan.md` are the active decision records.

## 1. Product Definition

The 2.5 output layer should be a role-play director room / writers' room, not a grammar checker and not a full rewrite bot.

It receives the main model's completed draft, reviews it through role-separated agents, applies only bounded low-risk patches, and preserves the original output whenever the system is not confident that the change improves the result.

Core sentence:

> Improve role-play output by catching continuity, character, scene, style, and protected-format failures without replacing the user's intent or rewriting the whole answer.

Distribution boundary:

- The first deliverable must be one standalone RisuAI `.js` plugin file.
- It must not be implemented by adding more code to `Archive Center.js`.
- It must run without Archive Center.
- Archive Center awareness can only be added later as an optional read-only
  adapter inside the standalone plugin.
- If any optional adapter, provider call, or reviewer call fails, the main
  RisuAI output must pass through unchanged.

Configuration boundary:

- The plugin is one file, but every AI lane must have an independent role
  profile.
- Each role profile must be able to set its own endpoint, provider, API key
  reference, model, temperature, timeout, max output, system prompt, and task
  prompt.
- Shared presets are allowed for convenience, but the runtime must resolve and
  show the exact effective profile used by each role.
- Missing role configuration must skip or downgrade that role only. It must not
  silently fall back to an unrelated provider, and it must never block or erase
  the main RisuAI output.

## 2. Non-Negotiable Rules

These rules are the boundary that prevents the feature from becoming another unsafe output replacer.

1. User input is immutable.
2. The main model draft is the source text; the plugin may annotate or patch it, not replace it wholesale.
3. Protected spans must survive unchanged.
4. If confidence is low, return the original draft.
5. If protected tags, image tags, code blocks, or structural markers are damaged, revert.
6. Streaming partials must not be treated as final output.
7. Every accepted patch must have a visible reason and trace.
8. Role agents may only act within their declared domain.
9. A patch is optional support, not canonical truth.
10. The feature must remain useful without Archive Center.

## 3. Shared Consensus From The Four Reviews

All four reviews converge on the same functional direction:

- Multi-agent review is valuable only if each agent has a strict role boundary.
- The patch unit should be a span or segment, not the entire response.
- A deterministic verifier is as important as the LLM reviewers.
- Trace/diff is required for user trust.
- Early MVP should be conservative.
- Light patching should come after audit and verification are stable.
- Full Table Read / full multi-character simulation is too heavy for the first 2.5 slice.
- The strongest RP-specific value is continuity, character voice, scene anchoring, and user-agency preservation.

## 4. Recommended Pipeline

```text
main model draft
  -> protected span masking
  -> response type classifier / triage
  -> selected role readers
  -> structured span findings
  -> director arbitration
  -> bounded patch application
  -> integrity and no-regression gate
  -> protected span restore
  -> final output + trace
```

### 4.1 Protected Span Masking

Protect before any LLM reviewer sees the output:

- image tags
- RisuAI tags
- hidden/system-like markers
- code blocks
- markdown structures that must stay balanced
- user-defined protected regex patterns
- model/tool metadata blocks

The reviewers can see placeholders but cannot modify the protected payload.

### 4.2 Triage / Router

The system should avoid calling every role on every turn.

Examples:

- dialogue-heavy: Character + Direction/Style
- exposition-heavy: World + Plot + Style
- action scene: Plot + Direction + Scene Anchor
- short output: Character + Style only
- streaming incomplete: audit-only or skip until final
- protected-heavy output: Guardian first, then conservative review

### 4.3 Structured Finding Schema

Each role reader should return findings, not full rewrites.

Minimum schema:

```json
{
  "role": "character",
  "domain_tag": "voice",
  "span": {
    "start_hint": "text before...",
    "end_hint": "...text after",
    "quote": "problematic phrase"
  },
  "severity": "low|medium|high",
  "confidence": 0.0,
  "problem": "why this may harm the output",
  "patch": "bounded replacement candidate",
  "patch_type": "replace|insert_before|insert_after|delete",
  "requires_director": true
}
```

The exact schema can evolve, but all agents must stay structured.

### 4.4 Director Arbitration

The Director decides which findings are accepted, rejected, or deferred.

Recommended priority:

1. Protected span integrity
2. User input preservation
3. Character voice / knowledge boundary
4. World rule / physical consistency
5. Plot continuity / scene goal
6. Emotional direction / pacing
7. Style polish

If two findings conflict and neither is clearly superior, keep the original text.

### 4.5 No-Regression Gate

After patching, run deterministic checks:

- protected spans restored exactly
- output length did not swing beyond configured threshold
- markdown/code fences balanced
- image tags preserved
- no user input rewrite
- no obvious duplicate block insertion
- no dropped final paragraph
- no OOC/meta leakage introduced by the plugin

If any hard check fails, return the original draft and record the reason.

## 5. MVP Recommendation

The first practical 2.5 MVP should not implement the full dream system. It should prove the safe architecture.

MVP includes:

1. Protected Envelope Guardian
2. Span Annotation Protocol
3. Structured Finding Schema
4. Domain Boundary Manifest
5. Deterministic Patch Applier
6. No-Regression Gate
7. Audit-only mode
8. Light Patch mode behind explicit toggle
9. Character Reader
10. Plot/Continuity Reader
11. Style Reader
12. Director Arbiter
13. Trace/Diff UI
14. Response Type Classifier for routing

MVP does not include:

- full per-character LLM Table Read
- always-on 5+ agent calls
- mid-stream rewrite
- automatic whole-output rewrite
- Archive Center memory dependency
- image-generation quality judgement beyond protected tag preservation

## 6. Role Readers

### 6.1 Character Reader

Purpose:

- preserve character voice
- prevent personality drift
- prevent a character from knowing things they should not know
- detect relationship posture mismatch

MVP status: required.

Patch authority: limited. It may suggest small voice or knowledge-boundary fixes.

### 6.2 Plot / Continuity Reader

Purpose:

- detect ignored user action
- detect current scene goal being dropped
- detect sudden contradiction with the immediate prior scene
- detect unresolved promise being skipped

MVP status: required.

Patch authority: limited. It should prefer annotations and small continuity patches.

### 6.3 Style Reader

Purpose:

- detect repetition
- detect register mismatch
- smooth clumsy phrasing
- preserve the original tone rather than beautifying everything

MVP status: required.

Patch authority: safest initial patch domain.

### 6.4 World Reader

Purpose:

- check local world rules
- check physical/environmental consistency
- detect faction/social rule contradictions

MVP status: check-only or deferred.

Reason: world rule quality requires context input. It should be added after the core verifier is stable.

### 6.5 Direction / Performance Reader

Purpose:

- emotional arc
- pacing
- scene blocking
- sensory balance
- tension curve

MVP status: deferred or check-only.

Reason: high value, but easy to over-edit.

### 6.6 Scene Anchor Guard

Purpose:

- keep who/where/when/participants/physical posture stable
- catch speaker confusion in multi-character dialogue

MVP status: late MVP or first expansion.

This is one of the strongest RP-specific differentiators and should not be forgotten.

## 7. Strongest Feature Candidates

### 7.1 Continuity Guard

Protects the user's latest input and the immediate scene state from being ignored.

Why it matters:

- This is the most common RP frustration.
- It directly improves perceived intelligence.

### 7.2 Scene Anchor Guard

Tracks the active scene anchors:

- location
- participants
- speaker attribution
- physical state
- recent action
- current pressure

Why it matters:

- Prevents "wrong person/wrong place/wrong scene" failures.
- More RP-specific than generic prose polishing.

### 7.3 Character Voice Fingerprint

Builds a lightweight per-character voice profile from current context or user-provided examples.

Why it matters:

- Users notice character voice drift immediately.
- It can be useful even without long-term memory.

### 7.4 Before/After Diff With Reasoning

Shows:

- original span
- replacement span
- role that proposed it
- Director decision
- accepted/rejected reason

Why it matters:

- Users can trust and tune the system.

### 7.5 No-Regression Promise

The system must be able to say:

> If the plugin is unsure, it will not make the output worse.

This is not just marketing. It is an implementation requirement.

## 8. Mode Design

Recommended initial modes:

| Mode | Behavior |
|---|---|
| Off | Plugin does nothing. |
| Audit Only | Finds issues and shows trace, but does not change final output. |
| Light Patch | Applies low-risk accepted patches only. |
| Standard | Character + Plot + Style with Director and verifier. |
| Deep / Rehearsal | Future heavier mode with World, Direction, and more agents. |

Do not make Deep/Rehearsal the default.

## 9. Resolved Design Conflicts

### 9.1 Should MVP include all five roles?

Decision: no.

Start with Character, Plot/Continuity, and Style. Add World and Direction as check-only or expansion roles after the core patch/verifier path is stable.

### 9.2 Should Light Patch be in MVP?

Decision: yes, but only behind an explicit toggle and only after Protected Guardian + No-Regression Gate are working.

Audit-only must exist first.

### 9.3 Should the plugin rewrite the final output?

Decision: no.

It may patch bounded spans. Whole-output rewrite belongs to a future experimental mode, not the normal path.

### 9.4 Should it depend on Archive Center?

Decision: no.

The plugin must be standalone. If a backend or memory source exists later, it can provide optional context, but the product cannot require it.

### 9.5 Should streaming be modified live?

Decision: no for MVP.

Streaming outputs should be reviewed after completion, or audit-only while incomplete.

## 10. Implementation Order

Recommended build sequence:

1. Product shell and settings
2. Protected span parser/masker/restorer
3. Trace ledger structure
4. Structured finding schema
5. Audit-only mode
6. Character Reader
7. Plot/Continuity Reader
8. Style Reader
9. Director Arbiter
10. Patch Applier
11. No-Regression Gate
12. Before/After Diff UI
13. Response Type Classifier / Router
14. Light Patch mode
15. Scene Anchor Guard
16. World Reader check-only
17. Direction Reader check-only
18. Voice Fingerprint Cache
19. Promise Hook Tracker
20. Full Table Read / Rehearsal mode

## 11. UI Requirements

Minimum UI:

- mode selector
- role toggles
- per-role AI profile editor
- per-role endpoint / provider / model / temperature settings
- per-role system prompt and task prompt editor
- shared preset editor with effective-config preview
- protected pattern list
- severity threshold
- max calls / budget setting
- trace panel
- before/after diff
- accepted/rejected findings
- revert-to-original button
- copy original / copy patched

Trace should answer:

- What did the plugin check?
- Which agents ran?
- Which endpoint/model/profile did each role actually use?
- Which patches were proposed?
- Which were accepted?
- Which were rejected?
- Did the verifier revert anything?
- Was the final output original or patched?

## 12. Cost Control

The system must not call every agent every turn.

Cost controls:

- response type classifier
- role routing
- short-output fast path
- audit-only default during streaming
- max agent count
- max findings per role
- max patch count
- confidence threshold
- cache voice fingerprints where possible

Recommended default:

- Audit-only: 1-3 calls depending on enabled roles.
- Light Patch: Character + Plot/Continuity + Style + Director.
- Deep/Rehearsal: future only.

## 13. Future Expansion

Good later features:

- Scene Anchor Guard as first expansion after MVP
- World Surface Rule Validator
- Emotional Arc Director
- Pacing Curve visualization
- Dialogue Attribution Resolver
- Promise Hook Tracker
- Cross-Examination Round for high-risk findings
- Character-specific "What would this character say?" inspection
- Full Table Read with per-character agents
- Genre presets
- user preference profiles

Experimental features:

- image tag consistency auditor
- whole-output rewrite mode
- live streaming intervention
- multi-model character panel

## 14. Things To Avoid

Avoid these failure modes:

- turning output support into output replacement
- hiding changes from the user
- damaging image or markup tags
- calling too many LLMs by default
- treating style preference as correctness
- letting one role rewrite another role's domain
- making Archive Center required
- allowing patch failure to block the main output
- applying patches when span matching is uncertain

## 15. Current Recommendation

The active 2.5 plan should be updated around this foundation:

1. Build a standalone director-room plugin.
2. Start audit-only.
3. Prove protected masking and deterministic no-regression.
4. Add Character / Plot / Style readers.
5. Add trace/diff.
6. Only then enable light patching.
7. Treat full Table Read and MDASH multi-agent expansion as later layers, not the first deliverable.

This keeps the product useful, safe, and visibly different from ordinary rewrite plugins.

## 16. Added Work: Risu Runtime Context Collector

Date added: 2026-06-27

The example-plugin review changes one part of the 2.5 plan: standalone does not
mean context-blind. When RisuAI exposes character, persona, lorebook, module,
current chat, or memory-like fields to the plugin API, 2.5 should read them as
source material for reviewers.

This must remain a read-only context collector, not a memory replacement and not
a hidden prompt rewriter.

### 16.1 Example Lessons

- `risu-multiagent.js` mostly reads the `beforeRequest` `messages` payload,
  extracts system context / recent chat / latest user input, and injects
  guidance. It bypasses HypaMemory, translation, and helper requests.
- `multiagent-full-v0.8.1` is a sidecar pattern. The server receives
  `chat_history`, `system_context`, `world_summary`, and `char_summary` from the
  caller; it does not read RisuAI state by itself.
- `risu_agents.js` actively probes RisuAI APIs such as `getCharacter`,
  `getDatabase`, current character/chat indexes, and current chat objects. It
  also builds lorebook candidates and catches regex failures.
- `Serial_Gradation_Agents_for_RP.js` is the closest reference for RP context:
  it has an optional shadow Risu context path for character, persona, lorebook,
  module lore, and current chat memory fields such as Supa/Hypa memory data.

The 2.5 implementation should use these as patterns only. It must not copy their
pipeline shape or allow whole-output replacement.

### 16.2 Collector Sources

The collector should build a bounded `runtime_context` snapshot from:

1. Request payload context: system messages, recent visible chat, and latest
   user input already present in `messages`.
2. Character context: `getCharacter()` first, then index-based fallback if the
   API exists.
3. Current chat context: `getCurrentCharacterIndex()`, `getCurrentChatIndex()`,
   `getChatFromIndex()`, then `character.chats[chatPage]` fallback.
4. Persona/database context: `getDatabase()` for personas, selected persona,
   enabled modules, and global variables when available.
5. Lorebook candidates: character lore, current chat lore, and enabled module
   lore. Active matching should be best-effort and must catch invalid regex.
6. Memory-like snapshots: current chat fields such as `supaMemoryData`,
   `hypaMemoryData`, `hypaV2Data`, `hypaV3Data`, `lastMemory`, `memory`,
   `memories`, `summary`, and `note`.
7. Protection context: user regex protection settings and detected RisuAI
   structural markers, used only to protect output and guide reviewers.

Each source must record availability, source path, char count, and failure
reason in trace.

### 16.3 Safety Boundary

- The collector is read-only.
- It must not write lorebook, chat, memory, module, database, or plugin-local
  canonical state.
- Collected context is inspect-only source material for AI roles.
- Missing APIs or inaccessible data must degrade to payload-only context.
- Memory request modes, regex/helper calls, image calls, module calls,
  translation, summaries, embeddings, title generation, and other auxiliary
  calls must stay pass-through.
- Context snippets must be bounded by per-source char limits before any AI call.
- Protected output spans remain protected even if the collector finds lore or
  memory that appears to request a different format.

### 16.4 Implementation Sequence

1. Add `risu_runtime_context_collector` as a deterministic local item.
2. Record context availability in trace without changing prompts or output.
3. Add payload-only extraction for system context, recent chat, and user input.
4. Add safe API wrappers for character, database, current chat index, and chat
   object access.
5. Add read-only character/persona/current-chat summary formatting.
6. Add lorebook candidate collection and active-key matching with invalid-regex
   isolation.
7. Add memory-like snapshot formatting from current chat fields.
8. Feed bounded `runtime_context` into Character Reader, Plot/Continuity Reader,
   Style Reader, Director, and Table Read prompts.
9. Extend Trace UI to show context sources used, skipped, failed, and char
   counts.
10. Verify pass-through again: output must remain unchanged in check-only mode
    and all protected spans must still be exact.
