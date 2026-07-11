# 2.5 Long-Session and Subjective Memory Accuracy Gate

## Long-Session Regression

- The regression fixture contains an assistant starter at turn `0` and complete user/assistant pairs through turn `120`.
- The first rescan stops after 53 candidates and must process turns `0..52`.
- A second rescan must resume at turn `53` and finish at turn `120`.
- A third rescan must find no candidates.
- Every turn must create exactly one memory and the Critic must be called exactly 121 times across all three runs.

This verifies ordered cold-start/rescan recovery, interrupted-run continuation, and duplicate-free replay without requiring a real 100-turn user session.

## Subjective Memory Storage

New subjective memories are skipped only when one of these high-confidence conditions is met for the same canonical owner and session:

1. The normalized memory text is identical. This applies across turns.
2. The grounded evidence text is identical, sufficiently long, and the source turns are no more than three turns apart.

The implementation does not delete or rewrite existing rows. It does not use broad semantic similarity, so a later reinterpretation or a memory with different evidence remains eligible for storage.

## Subjective Memory Injection

- Existing scene relevance remains required: the owner must appear in the current user input, the immediate chat tail, or the latest active scene state.
- At most one private recollection per owner is injected.
- At most two private recollections in total are injected for one prepare-turn request.
- Additional relevant owners are reported as `private_recollection_total_capped` in the trace instead of being injected.

## Exact Entity Name Deduplication

- One Critic extraction result saves an exact normalized entity name only once per broad entity type.
- Whitespace and letter case differences do not create another row for the same name and type.
- Unconfirmed variants such as `이시우`, `시우`, and `Siwoo` remain separate until confirmed alias evidence exists.
- The same label may remain separate across different types, such as a character and an item.
- Records from different turns are not merged because they preserve state history and rollback boundaries.

## Exact Subjective-Memory Owner Read Model

- Subjective-memory owners with the same canonical owner key are shown as one entity bundle even when older rows have inconsistent role or visibility labels.
- Every memory row remains stored with its original source turn and scope metadata.
- Memories are read with the newest source turn first; older turns remain behind it as history.
- A bundle reports `mixed_owner_scope` and `scope_variant_count` when historical role/visibility labels disagree.
- Unconfirmed display-name variants remain separate bundles.

## Alias Repair Safety Gate

- Alias Repair scans name/key variants but does not treat name similarity, romanization, edit distance, or similar memory text as proof that two owners are the same person.
- Automatic apply is limited to rows carrying both `confirmed_identity_alias_canonicalized` and a non-empty grounded evidence excerpt, or a previous explicit user repair/force-merge record.
- A confirmed-identity tag without grounded evidence remains review-only.
- Similar-name candidates without that evidence are returned as `review_required` and are not mutated when Apply is pressed.
- The response separates `repairable_count` from `review_required_count` and gives each group a decision, evidence status, candidate reasons, source turns, and bounded memory previews.
- Memory previews are review context only. They never authorize an automatic merge.
- Apply still changes only owner/persona identity fields and tags. Memory text, direct evidence, source turns, and rows remain intact.
- A user who independently knows two ambiguous owners are the same person can use the explicit Force Merge action.

## Deliberate Non-Goals

- No automatic deletion or merge of existing subjective-memory rows.
- No embedding-based fuzzy deduplication.
- No expansion of scene membership inference.
- No global merge of historical entity rows by name.
- No change to raw chat storage, turn deletion, rollback, or vector deletion behavior.

## Validation

- Targeted long-session and subjective-memory regression tests pass.
- Existing same-owner cap and same-turn duplicate tests pass.
- `go test ./internal/... -count=1` passes.
- `go build ./cmd/archive-center-go` passes.
