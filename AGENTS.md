# Archive Center Codex Rules

## Task Routing

- For implementation, behavior, contract, schema, storage, packaging, or
  architecture changes, use `AI_GUARDRAILS.md` as the operational checklist.
- Use `STRUCTURE.md` as a repository map, not as a substitute for source
  verification. Read its current-status section and the sections relevant to
  the task, then inspect the cited paths, symbols, callers, and production
  tests.
- Use version work logs and roadmaps only to locate possible changes. Verify
  operational claims against the active implementation before relying on them.
- For documentation-only spelling, formatting, or link repairs that do not
  change a technical claim, inspect the affected document and nearby links;
  do not force a full runtime audit.
- Update both `STRUCTURE.md` and `AI_GUARDRAILS.md` in the same change whenever
  architecture, ownership, contracts, hook order, persistence, indexing, or
  fallback behavior changes.

- Active workspace: `C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`.
- Active runtime sources: `Archive Center.js` for the thin RisuAI host adapter and `go-service` for backend policy and persistence.
- Do not redirect current work to historical `M:\` worktrees, previous `_dist-*` packages, release copies, or reference files.
- Preserve unrelated dirty-worktree changes. Inspect the exact target and existing owner before every edit.
- Do not copy `.env`, vault keys, SQLite DB files, Chroma persist directories, caches, logs, backups, release packages, or deployment folders into this workspace.
- Keep `Archive Center 1.0`, backup folders, and deploy/release folders untouched.
- Treat `Archive Center.js` as a RisuAI host adapter, not as a second backend or the target of a language rewrite.
- The architecture target is behavior-preserving backend ownership, not a reduced rewrite or a new parallel runtime.
- MariaDB remains canonical truth storage and ChromaDB remains the product vector backend. Do not restore the retired Milvus experiment without an explicit architecture decision.
- Do not call work complete from documents, scaffolding, syntax checks, or skipped tests alone.

## Permanent RisuAI Host / Backend Boundary

This contract applies to every Archive Center version and all future work.

- `Archive Center.js` is a thin RisuAI host adapter.
- JavaScript owns only host observation, bridge transport, application of a
  backend result to the real RisuAI payload, final displayed-output
  confirmation, DOM/UI rendering, localization, and unavoidable host-local
  transient state.
- The Go backend owns policy, orchestration, selection, ranking, prompt and
  memory assembly, budget calculation, canonical decisions, turn and migration
  ranges, persistence, ViewModels, and stable error codes.
- Do not append JavaScript business logic when the backend can decide from
  observations supplied by the host adapter.
- Prefer modifying or removing an existing path over adding another guard,
  fallback, cache, watcher, or special case.
- Across the repository, do not add a file, function, state field, API, table,
  or compatibility path unless a reproduced requirement makes it necessary.
  Simplify or repair the existing owner first.
- Do not add speculative compatibility behavior without a reproduced failure
  or a versioned contract requirement.
- Do not classify requests by hard-coded plugin names, prompt prose, roleplay
  templates, or model-specific phrases.
- Backend-first feature work is mandatory. Any unavoidable JavaScript growth
  must state the concrete RisuAI host capability that requires it.
- Moving logic is not complete until the replaced JavaScript implementation is
  removed, or a time-bounded compatibility fallback is documented with a
  version gate and removal condition.
- Every completed change must report JavaScript lines added and removed.

## Absolute Ban on New Strict or Multi-Condition Rejection Gates

This is a permanent rule. Do not add new strict conditions. Do not add a
"second condition" (`2차 조건`). Do not combine multiple observations or
multiple checks to make normal output, display, persistence, reroll,
replacement, deletion, or recovery harder to accept.

- This rule governs future work and future additions only. Do not use this
  rule by itself as a reason to remove, weaken, rewrite, or disable an
  existing behavior that is already working.
- Existing behavior may be changed only for the specific reproduced problem
  the user asked to fix, after showing the exact affected behavior and the
  minimal proposed change. Do not broaden that change to adjacent working
  paths.
- If a new strict condition or multi-condition check appears genuinely
  essential, stop and ask the user first. Apply it only after the user
  explicitly approves that exact condition. Do not infer approval from the
  task, safety concerns, or prior discussion.

- Do not design, propose, recommend, plan, discuss as a candidate, implement,
  test, document, or leave a TODO for a new strict condition or
  multi-condition rejection gate. The approach itself is out of scope and
  must be discarded immediately, not presented to the user as an option.
- Do not reason from "more observed signals are safer", "all coordinates
  should agree", "uncertainty should fail closed", or similar premises when
  handling normal Archive Center output and lifecycle behavior. Do not use
  defensive strictness as a design direction.
- An already existing strict or multi-condition gate is not automatically a
  removal target. Inspect it only when it is directly implicated in the
  reproduced problem the user asked about, and do not remove or replace it
  without the user's explicit approval of that exact change.
- Do not create `all fields must match`, multi-field `AND`, unanimous-signal,
  exact-coordinate, threshold stack, or equivalent multi-condition gates.
  Adding more conditions is prohibited; it is not an accepted safety method.
- Do not deliberately route a model result or Host-observed result toward
  rejection, discard, suppression, no-save, terminal failure, or fail-closed
  behavior because one of several observations is absent, changed, delayed,
  or uncertain.
- Do not make output refusal the fallback for uncertainty. Preserve the
  existing user-visible output and the existing established lifecycle
  behavior. Record a diagnostic observation only; do not add another decision
  gate around it.
- Do not tighten an existing identity, lifecycle, acceptance, replacement,
  reroll, deletion, persistence, or recovery rule. A task to fix one path is
  not permission to make adjacent paths stricter.
- Do not make an optional Host observation mandatory. `chatId`, message index,
  pair ordinal, timestamp, content hash, generation ID, swipe ID, and similar
  coordinates remain separate evidence. They must not be accumulated into a
  stricter composite acceptance or rejection rule.
- Never reuse a transient provider-request retry fingerprint as a durable
  logical-turn, reroll, replacement, deletion, or canonical persistence
  identity. Retry correlation and persistent turn identity are separate
  contracts.
- A fix for provider retry, timeout, fallback, HUD, or transient request
  ownership must not change the completed-turn persistence or reroll/edit
  contract.
- Editing the same Host user row and regenerating after deleting its assistant
  remains a replacement of the existing logical turn. Rerolling the same Host
  user row remains a replacement even when optional coordinates drift. A
  genuinely new Host user row remains a new turn even when its text is
  identical. Do not replace these distinctions with strict matching.
- A changed or missing optional coordinate must not fall through to `append
  after canonical tail`, create a duplicate user/assistant pair, block a
  previously valid replacement, or reject an otherwise available output.
- Do not add new terminal ledgers, rejection states, validation layers,
  fallback gates, compatibility guards, or parallel decision paths as an
  unrequested safety measure.
- Before changing identity or persistence behavior, show the user the existing
  rule, the exact minimal direct change, and the production cases it affects.
  Do not include a strict or multi-condition rejection design among the
  alternatives under consideration.
- Do not broaden an approved fix. Implement only the exact requested behavior
  and leave every unrelated acceptance and output path unchanged.

## Test Integrity

- Tests must exercise the production function or API that owns the behavior. A
  duplicate test-only implementation is not evidence that the runtime works.
- Do not hard-code expected turn numbers, scores, rankings, baselines, or other
  policy results merely to match the current patch. Derive expectations from
  the fixture inputs and the documented contract.
- Do not replace the behavior under test with an always-successful stub,
  unconditional `pass`, no-op callback, or canned response and then report the
  suite as validation of that behavior.
- Stubs are allowed only for external boundaries that are not under test. They
  must record calls and fail on unexpected input so the production path cannot
  silently bypass the assertion.
- Every regression test must fail against the known broken behavior or include
  an equivalent negative assertion that proves the guard is meaningful.
- A green test that cannot detect removal or inversion of the intended runtime
  branch is invalid and must not be used as completion evidence.
- Report exactly what a test covers and what remains unverified. Never present
  syntax checks, source-string checks, mocks, or fixture-only tests as live
  end-to-end validation.
- Any change near request identity, completed-turn acceptance, reroll, edit,
  deletion, or persistence must exercise the production owner with at least
  these distinct cases: same-request provider retry; same Host user row
  reroll; same Host user row edited after assistant deletion and regenerated;
  genuinely new Host user row with identical text; and a normal new turn.
  The tests must prove which cases replace and which append. Omitting the
  edit-and-regenerate case invalidates completion evidence for that change.

## Permanent Fresh-Install Contract

- Normal-user fresh install has exactly two public entrypoints:
  - POSIX: `curl -fsSL https://raw.githubusercontent.com/Flazer31/archive-center/main/install.sh | sh`
  - Windows PowerShell: `irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex`
- These entrypoints are fresh-install only. They must not perform, stage, or
  redirect into an update, compatibility bridge, database migration, or
  existing-install replacement.
- If an existing managed install root is present, the fresh installer must stop
  before downloading a release helper or changing files and direct the user to
  the separate update path.
- The fresh-install command must not require the user to choose a timeout,
  install directory, CPU architecture, service mode, restart delay, or start
  flag. The entrypoint owns those normal-install defaults and starts the
  installed package.
- Do not add another normal-user fresh-install command. Advanced/operator
  helpers may remain internal, but README and release-facing documentation must
  keep the two commands above as the only normal fresh-install interface.
- CI must exercise the production fresh-install entrypoints, including the
  existing-install no-network/no-mutation path. A documentation-only assertion
  is not sufficient evidence.

Read `docs/permanent-risu-host-backend-boundary.md` before changing runtime
ownership or adding functionality to `Archive Center.js`.

For all 4.0 memory creation, retrieval, injection, lorebook, and edit-check work,
read `docs/4.0-memory-restoration-work-contract.md` before analysis or edits.
