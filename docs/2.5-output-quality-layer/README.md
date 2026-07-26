# 2.5 Output Quality Layer Docs

This folder groups the 2.5 standalone MDASH/Table Read output-quality design
documents so they do not mix with unrelated Archive Center runtime docs.

## Current Authority

- `risu-recomposer-mdash-execution-plan-2026-07-24.md`

This is the canonical preparatory implementation plan for the current
`source/Risu Recomposer.js` runtime. It owns the current MDASH/Fusion/Fugu
interpretation, implementation order, 500 KB one-file limit, call budget,
Judge/Composer/Prover separation, and beta evidence gate.

If an older dated plan, operating contract, full-flow map, strong-fusion
roadmap, or R1-R7 worker prompt conflicts with the current sequence or limits,
the 2026-07-24 execution plan wins.

## Supporting Design Documents

- `2.5-standalone-output-quality-layer-plan.md`
- `feature-expansion-synthesis-2026-06-26.md`
- `2.5-role-call-catalog.md`
- `2.5-risu-runtime-context-collector-implementation-plan.md`
- `2.5-parallel-execution-layer-plan.md`
- `2.5-live-qa-hardening-2026-06-28.md`
- `2.5-fusion-orchestrator-roadmap-2026-06-28.md`
- `2.5-mdash-fusion-operating-contract-2026-06-29.md`
- `2.5-mdash-fusion-fugu-full-flow-2026-07-01.md`
- `2.5-strong-fusion-enhancement-roadmap-2026-07-02.md`
- `rescan-canonical-backfill-fix.md`
- `long-session-subjective-memory-accuracy-gate.md`
- `../provider-request-overrides-flex-paygo-contract.md`

Use these as supporting 2.5 design references. They define the standalone-first
RisuAI output quality layer, including Input Enhance MDASH, Output Check MDASH,
Table Read MDASH, Output Enhance MDASH, protected segment patching, verifier,
and trace.

Use the feature-expansion synthesis as the current MVP and sequencing decision
record for the multi-agent RP director-room design.

Use the role call catalog as the implementation contract for the plugin's
default callable items and per-role AI profile requirements.

Use the runtime context collector plan as the active step-by-step checklist for
adding read-only RisuAI character, persona, lorebook, current-chat, and
Supa/Hypa memory context to the existing reader pipeline.

Use the parallel execution layer plan as the S8 record for bounded context and
reader role concurrency, execution modes, and trace requirements.

Use the live QA hardening note as the current record for context caps, estimated
token trace, lore/memory matching improvements, image marker protection, and
reader JSON recovery.

Use the fusion orchestrator roadmap as supporting S9+ design history for moving
from audit-only readers into enhancement-first multi-model fusion: bounded
revision, fusion composition, JS verification, and verified enhanced output
return.

Use the MDASH/Fusion operating contract as supporting interpretation history.
Its deterministic-router, specialist-reader, fusion-director,
segment-composer, and JS-verifier concepts remain relevant, but the canonical
execution plan owns the current stage order.

Use the MDASH/Fusion/Fugu full flow document as historical end-to-end sequence
detail. It records the earlier operating order and alpha maturity snapshot.

Use the strong fusion enhancement roadmap as supporting evidence for the
aggressive rewrite direction. Specialist AIs still generate strong improvement
candidates; the canonical execution plan now owns how those candidates are
judged, composed, and verified.

Use the provider request overrides and Vertex Flex PayGo contract as the shared
provider-options contract with Archive Center. Archive Center should apply the
contract in the Go backend, while the standalone Output Quality Layer should
apply the same setting names and safety rules inside its own JS provider caller.

## Reference Archive

- `_reference-do-not-use-as-active/`

This folder contains older plans, raw AI opinions, and legacy Table Read notes.
Do not use those files as implementation authority unless the active anchor or
the user explicitly promotes a specific item back into the active plan.

Subfolders:

- `source-plans/`: earlier strategic plans and source drafts.
- `ai-design-reviews/`: raw AI design review responses.
- `ai-feature-expansion-notes/`: raw AI feature expansion responses.
- `legacy-table-read/`: older Table Read and polish planning notes.
