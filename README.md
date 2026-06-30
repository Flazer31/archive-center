# Archive Center 2.0

Archive Center 2.0 is a new migration workspace for moving the existing Archive Center backend substrate toward:

- Go-primary backend runtime
- MariaDB canonical truth
- ChromaDB local-first vector retrieval lane in shadow/limited mode

Milvus Lite is not part of the current 2.0 backend target. It is mentioned only as a future reconsideration note in historical R1 documents, in case the project later needs to evaluate it separately.

This folder is intentionally not a copy of `Archive Center Beta 0.8(fix)`. The 0.8(fix) tree remains the current runtime baseline and compatibility reference. The goal is to migrate the completed 0.8(fix) backend feature set into the new substrate, not to reduce the feature surface. Runtime secrets, databases, vector indexes, caches, logs, backups, and release artifacts must stay outside this workspace.

`Archive Center.js` remains the RisuAI-facing JavaScript host adapter. 2.0 backend migration must preserve its HTTP payload, trace, fallback, and compatibility contract instead of rewriting it.

## User Installation Burden Kill Gate

2.0 must not require normal users to manually install and operate Go, MariaDB, ChromaDB, Python fallback tools, and RisuAI artifacts as separate products. The default local-user path must be a single packaged launcher/installer or a clearly bounded one-command bootstrap. Advanced external MariaDB/ChromaDB configuration may exist as an opt-in operator path only.

The current 2.0 backend direction is fixed as MariaDB + ChromaDB. Milvus Lite must not be treated as a normal-user install requirement, package requirement, live-read requirement, checklist target, or release blocker. It may only be reconsidered later through a separate post-checklist decision if the project explicitly needs it.

If this packaging story cannot be made practical for low-resource local users, the 2.0 migration must pause, redesign, or cancel before any R2 cutover. Passing Go/MariaDB/ChromaDB technical tests is not enough for release adoption.

## Current Phase

Status: R0/R1 foundation active. The 2.0-0 readiness floor is green, Go runtime work has R1 `implemented-shadow` evidence, MariaDB is the planned canonical truth backend, and ChromaDB is the active vector/search accelerator direction. Milvus Lite remains historical evidence plus a future-only reconsideration note. No R2/R3 cutover-ready claim is made.

The current segment is not a live migration. It establishes the workspace, migration boundaries, safety gates, Go shadow service, compare tooling, and first parity evidence before any MariaDB authority, ChromaDB live/default vector switch, or Go default runtime switch.

## First Build Order

1. Freeze 2.0 workspace rules and artifact hygiene.
2. Define current Python/FastAPI route and service contract from 0.8(fix).
3. Capture baseline metrics from the current runtime.
4. Define MariaDB truth boundary without cutover.
5. Define ChromaDB vector/search accelerator boundary without cutover.
6. Define Go service boundary without replacing Python.
7. Run low-resource feasibility/shakedown before any R2 cutover.

## Non-Goals For This Phase

- No Go service cutover.
- No MariaDB authority switch.
- No ChromaDB live/default vector switch.
- No Milvus Lite work in the current checklist; only a future-only reconsideration note is retained.
- No old path retirement.
- No `Archive Center.js` rewrite or routine 2.0 migration edits.
- No secrets or runtime data copied into the 2.0 workspace.
