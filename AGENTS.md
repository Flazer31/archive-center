# Archive Center 3.0 Codex Rules

- Active workspace: `C:\Users\com12\Downloads\Archive Center Clean Start 20260626-light\source`.
- Active runtime sources: `Archive Center.js` for the thin RisuAI host adapter and `go-service` for backend policy and persistence.
- Do not redirect current work to historical `M:\` worktrees, previous `_dist-*` packages, release copies, or reference files.
- Preserve unrelated dirty-worktree changes. Inspect the exact target and existing owner before every edit.
- Do not copy `.env`, vault keys, SQLite DB files, Chroma persist directories, caches, logs, backups, release packages, or deployment folders into this workspace.
- Keep `Archive Center 1.0`, backup folders, and deploy/release folders untouched.
- Treat `Archive Center.js` as a RisuAI host adapter, not as a second backend or the target of a language rewrite.
- The 3.0 migration target is behavior-preserving backend ownership, not a reduced rewrite or a new parallel runtime.
- MariaDB remains canonical truth storage and ChromaDB remains the product vector backend. Do not restore the retired Milvus experiment without an explicit architecture decision.
- Do not call work complete from documents, scaffolding, syntax checks, or skipped tests alone.
- Code implementation should be delegated to `@kimi_coder`; localized patches may use `@glm_subcoder`; final review/risk validation should use `@deepseek_validator` when available.

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

Read `docs/permanent-risu-host-backend-boundary.md` before changing runtime
ownership or adding functionality to `Archive Center.js`.
