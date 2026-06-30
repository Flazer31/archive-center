# LEGACY / DEBUG ONLY: Table Read TR-1/TR-2 Agent Planning

> Status: legacy diagnostic document.
>
> This document describes early TR-1/TR-2/TR-Review diagnostic routes. It is
> not the current product direction for Table Read.
>
> Current active implementation boundary:
>
> - Use `_archive/future-reference/RisuAI_TableRead_OutputOnly_Boundary.md` as
>   the active design source.
> - Table Read should improve the final assistant output before display/storage.
> - Candidate-style review/revise flows are debug-only and must not define the
>   main UX.
>
> Do not implement new product behavior from this document unless it is
> explicitly scoped as diagnostic/debug tooling.

## Goal

TR-1 introduces a safe Table Read planning surface for Archive Center 2.0.
It lets story-relevant entities read the current scene through their own
subjective memory banks, then prepares a support-only discussion plan.

This phase is intentionally dry-run only:

- no LLM call
- no DB write
- no prepare-turn mutation
- no canonical truth promotion

## Why Not Parallel Models Immediately

Parallel character agents are useful, but they need queueing, timeout,
cost, merge, and contradiction handling. TR-1 therefore exposes multi-model
slots without executing them. This keeps the route useful for UI/debug work
while leaving TR-2/TR-3 room to run real agents safely.

## Route

`POST /table-read/draft`

The route accepts:

- `chat_session_id`
- `scene_text`
- `user_input`
- `entities[]`
- optional `multi_model`
- optional `max_memories_per_entity`

The route returns:

- `table_read.agents[]`
- subjective memory cards per entity
- private-memory policy per entity
- orchestration plan
- multi-model support metadata

## Memory Boundary

The memory source is `subjective_entity_memories`. Each entity receives only
its own memory cards for the requested source session.

NPC/private memories are treated as:

- support-only
- character-private recollection
- interpretation, suspicion, or misunderstanding
- not objective truth
- not narrator disclosure

## Next Steps

TR-2 can add a single-model execution mode that asks one configured model to
simulate the table read.

TR-3 can add parallel agent execution where each entity may use a separate
provider/model, followed by a moderator synthesis pass.

## TR-2 Route

`POST /table-read/simulate`

TR-2 uses one configured model to simulate the Table Read. It receives the same
scene/entity shape as TR-1 plus an `llm` config object compatible with the
existing provider proxy.

TR-2 still does not write to MariaDB, ChromaDB, memories, KG triples, direct
evidence, or prepare-turn state. The output is returned as a support-only
candidate:

- `llm_call_attempted=true`
- `write_attempted=false`
- `table_read.simulation.mode=single_model_table_read`
- `truth_authority=false`
- `prepare_turn_role=support_only_candidate`

The model is instructed to return JSON with:

- per-entity private notes
- discussion comments
- moderator summary
- subtle story hints
- blocked reveals

Loop/regression/reincarnation/isekai secrets remain protected unless current
user input explicitly reveals them.

## TR-Review-1 Route

`POST /table-read/review`

TR-Review-1 is the post-output Table Read pass. It receives an already
generated `assistant_draft` and asks the involved entity viewpoints to inspect
whether the draft fits voice, private knowledge boundaries, emotional pacing,
and time/location/relationship continuity.

This route may call the configured LLM, but it remains read-only:

- `write_attempted=false`
- `replaces_output=false`
- `table_read.review.mode=assistant_draft_read_only_table_read_review`
- `truth_authority=false`
- no DB write
- no automatic final-output replacement

The returned review is for user/editor inspection first. Later phases may add
optional revision suggestions or guarded auto-polish, but TR-Review-1 only
evaluates and reports.

## TR-Review-2 Route

`POST /table-read/revise`

TR-Review-2 receives the same scene, entity memory cards, and `assistant_draft`
as TR-Review-1, plus optional `review_context` copied from the review result.
It asks the configured LLM for one revised assistant draft candidate.

This route is intentionally suggestion-only:

- `write_attempted=false`
- `replaces_output=false`
- `table_read.revision.mode=assistant_draft_revision_suggestion`
- `copy_only=true`
- `auto_apply=false`
- no DB write
- no automatic final-output replacement

This route is now classified as a debug/diagnostic route for inspecting how a
revision would be formed. It is not the product-level Table Read behavior.

The product direction is `Table Read Output Polish`: the generated assistant
output should be improved and returned as the final RisuAI response, not merely
shown as a copyable candidate. See `table-read-output-polish-plan.md`.
