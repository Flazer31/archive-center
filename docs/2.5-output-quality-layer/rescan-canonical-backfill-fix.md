# 2.5 Canonical Rescan and Cold-Start Backfill Fix

## Problem

- `chat_logs` raw rows could remain intact while Memory, Direct Evidence, KG, entity, and state artifacts were skipped.
- The batch rescan path applied the same content-shape `source_guard` used by live request capture.
- Legitimate roleplay containing plan, rule, template, or structured blocks could therefore be mistaken for auxiliary prompt residue.
- Turn `0` starter/prologue rows were accepted by canonical storage but excluded by repair, normalize, rescan, and raw world-rule audit paths.
- Restarting a rescan after a browser/plugin interruption could start another background job for the same session while the first job was still running.

## Contract

1. `chat_logs` is the canonical visible conversation source for repair and rescan.
2. Batch derivation must not reject canonical rows by matching words or formatting inside their content.
3. Live request capture may still protect storage from an unverified auxiliary request before it becomes canonical raw data.
4. Turn `0` is a valid assistant starter/prologue record. It is processed before positive dialogue turns.
5. Episode and hierarchy ranges still begin at turn `1`; turn `0` supplies initial world, character, location, language, and continuity evidence.
6. Only one background job of the same kind may run for the same session. A repeated request reuses the existing job id.
7. Raw logs are not rewritten by rescan. Missing derived artifacts are rebuilt through the configured Critic pipeline and existing dedupe checks.
8. Session Normalize resumes from existing artifacts by default. Re-running it skips turns that already have canonical memory and does not force raw world-rule, episode, hierarchy, or vector regeneration unless an operator explicitly requests those force flags.

## Implementation

- Added a canonical-log Critic path that performs storage-safe cleanup without the content-shape source-control rejection.
- Removed `shouldSkipDerivedIngestForSourceAwareGuard` from canonical rescan and manual regenerate paths.
- Removed the content-shape source-aware derived guard after a live complete-turn has been accepted as a canonical user/assistant pair. Auxiliary-request rejection remains a capture/routing responsibility before canonical persistence.
- Allowed turn `0` in repair replay, session-normalize inputs, rescan candidates, processed-turn progress, memory world-rule backfill, and raw world-rule audit.
- Added same-kind/same-session background job reuse in `adminJobManager`.
- Removed Session Normalize's unconditional force flags so a partial run resumes from missing turns instead of replaying every successful Critic call.

## Regression Gates

- A turn `0` starter containing structured narrative guidance still produces derived memory.
- A turn `0` assistant-only starter can be restored by repair replay.
- A repeated background rescan returns the running job id and does not invoke duplicate work.
- Existing normal rescan and background failure reporting tests continue to pass.
