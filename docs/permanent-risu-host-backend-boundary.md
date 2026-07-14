# Permanent RisuAI Host / Backend Boundary

Status: mandatory architecture contract  
Scope: every current and future Archive Center version  
Effective date: 2026-07-13

## Purpose

`Archive Center.js` must remain a thin frontend adapter between RisuAI and the
Archive Center backend. It must not grow into a second policy engine, memory
server, or persistence runtime.

This contract is permanent. Version-specific plans may describe how to migrate
existing code, but they may not weaken this boundary.

## Minimal-Addition Rule

This rule applies to the whole repository, not only JavaScript. Do not add a
file, function, state field, API, table, worker, cache, watcher, fallback, or
compatibility path unless a reproduced requirement or versioned contract makes
it necessary. Repair, simplify, consolidate, or remove code in the existing
owner before creating another path.

## JavaScript Ownership

JavaScript may own only work that requires direct access to the RisuAI host:

- receive `input`, `beforeRequest`, `afterRequest`, streaming, display, and
  other RisuAI lifecycle hooks;
- observe the current character, CID, active chat, message indexes, message
  roles, visible deletion, reroll, chat transition, and final displayed output;
- collect the minimum host observations required by a versioned backend
  request;
- call backend APIs and correlate responses with the active RisuAI request;
- apply a backend-produced injection or mutation plan to the actual RisuAI
  payload/message array;
- confirm that the backend-selected final assistant value matches what RisuAI
  actually displays;
- render and localize settings, dashboard, timeline, explorer, and status DOM;
- keep minimal transient host state and a transport-only queue needed while the
  backend is unreachable.

Host observation does not grant JavaScript policy ownership. JavaScript may
report that a new user index appeared; the backend decides what that fact means
for turn ownership or persistence.

## Backend Ownership

The Go backend owns all work that can be decided from supplied observations and
stored state:

- request ownership and request-class decisions;
- canonical user-input and assistant-output candidate selection;
- memory, evidence, entity, state, rule, and reference-work retrieval;
- ranking, relevance, recency, visibility, contradiction, and continuity
  policy;
- supervisor and critic orchestration and interpretation;
- prompt, context, and memory block assembly;
- injection order, inclusion, compression, and character/token budgets;
- turn numbers, rollback/delete ranges, copy/move/connect baselines, and
  migration protection;
- raw, derived, vector, audit, retry, and idempotency persistence;
- dashboard, timeline, explorer, and maintenance ViewModels;
- stable error codes, severity, retryability, and operator guidance;
- provider credentials, LLM/embedding request construction, and backend jobs;
- reference-work extraction, review, timeline normalization, session binding,
  retrieval, and spoiler/time-scope filtering.

## Prohibited JavaScript Growth

Do not add the following to `Archive Center.js`:

- a second implementation of a backend calculation;
- prompt assembly or memory-selection policy;
- database range or migration calculations;
- content-based request classification using plugin names, prompt text,
  markdown markers, roleplay templates, or model-specific phrases;
- speculative guards, caches, watchers, or retries for an unverified failure;
- a compatibility fallback without a contract version gate and removal
  condition;
- backend credential ownership or provider-specific request policy;
- hidden feature logic merely because adding JavaScript is quicker than adding
  a backend contract.

When a defect is found, first identify which existing owner made the wrong
decision. Repair that owner. Do not cover the result with another independent
JavaScript decision layer.

## Admission Test For New JavaScript

Before adding JavaScript, answer all of these questions:

1. Does the work require a RisuAI API, lifecycle hook, actual message array,
   displayed-output observation, DOM, localization, or backend-unreachable
   transport queue?
2. Can the backend perform the decision if JavaScript sends a small observation
   packet?
3. Can an existing adapter path be changed or simplified instead of adding a
   new function?
4. Is the behavior required by a reproduced failure or a versioned contract?
5. Will the same work item remove at least the backend-replaced JavaScript
   calculation?

If question 1 is no, the work belongs in the backend. If question 2 is yes,
JavaScript must collect observations only. If questions 3 or 4 fail, do not add
the code.

## Migration Rule

Backend migration follows one bounded sequence:

1. Freeze the current externally visible behavior with a behavior test.
2. Define a versioned backend request and response contract.
3. Implement and test the backend owner.
4. Reduce JavaScript to observation, transport, application, and rendering.
5. Remove the replaced JavaScript calculator in the same work item.
6. Run JavaScript syntax, core regression, and full Go tests.
7. Record JavaScript lines added and removed.

A shadow comparison may exist only during an explicitly approved compatibility
window. It must state the minimum supported backend version and the exact
condition that removes the old path. An unbounded fallback is not a completed
migration.

## Exceptions

An exception is allowed only when the backend cannot physically perform the
operation, such as mutating the live RisuAI payload or observing the final
rendered output. Convenience, speed of implementation, and fear of changing a
backend contract are not exceptions.

Every exception must be narrow and documented next to the adapter code. It must
not include policy that can be expressed as backend input and output data.

## Current Migration Debt

Existing JavaScript that violates the target boundary is migration debt, not a
precedent for new code. Major examples include final injection budgeting and
parts of turn orchestration. They should move only with behavior-preserving
contracts and must not be expanded while awaiting migration.

Legacy Table Read code is a removal candidate, not a reason to add more
frontend policy. Its compatibility requirements must be audited separately
before deletion.

## Completion Gate

A feature or repair is not complete unless:

- backend-capable logic is implemented in the backend;
- JavaScript contains only the minimum host adapter change;
- no duplicate policy path remains without an approved compatibility window;
- no prompt-specific hard coding was introduced;
- tests cover the versioned contract and RisuAI adapter behavior;
- raw RisuAI message indexes are forwarded by JavaScript and logical turn or
  migration-baseline calculations are performed by the backend;
- the final report includes JavaScript lines added and removed and names any
  remaining adapter-side business logic.
