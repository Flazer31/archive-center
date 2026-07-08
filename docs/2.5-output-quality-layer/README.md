# 2.5 Output Quality Layer Docs

This folder groups the 2.5 standalone MDASH/Table Read output-quality design
documents so they do not mix with unrelated Archive Center runtime docs.

## Active Anchor

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
- `../provider-request-overrides-flex-paygo-contract.md`

Use this as the active 2.5 design anchor. It defines the standalone-first
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

Use the fusion orchestrator roadmap as the S9+ sequencing anchor for moving
from audit-only readers into enhancement-first multi-model fusion: bounded
revision, fusion composition, JS verification, and verified enhanced output
return.

Use the MDASH/Fusion operating contract as the current interpretation lock. If
older wording makes Fugu, Fusion, MDASH, or Table Read sound broader, weaker,
or more autonomous than intended, this contract wins: the plugin is a
deterministic-router, specialist-reader, fusion-director, segment-composer, and
JS-verifier pipeline for verified enhanced output.

Use the MDASH/Fusion/Fugu full flow document as the current end-to-end sequence
map. It records the full operating order, the current alpha maturity snapshot,
and the next implementation order after the first verified `fusion_enhance`
trace: patch apply stability, reasoning-only control, segment-level fusion
composition, full Table Read opt-in execution, router/cost profiles, and UI
hardening.

Use the strong fusion enhancement roadmap as the current direction lock for
the next implementation pass. If older wording makes the plugin sound
suggestion-only, too timid, or focused on weak local patches, this roadmap wins:
specialist AIs should generate strong improvement candidates, while the
Fusion Director, Segment Composer, and verifier decide what is safe enough to
return.

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
