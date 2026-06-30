# Codex Design Review: 2.5 Standalone Output Quality Layer (2026-06-26)

Status: design review (read-only analysis). No runtime change.

Scope reviewed:

- `README.md`
- `2.5-standalone-output-quality-layer-plan.md` (active anchor)
- `2.5-mdash-output-harness-plan.md` (strategic reference)
- cross-checked against legacy `table-read-output-polish-plan.md` and
  `table-read-tr1-agent-planning.md`

This review answers the eight requested questions and emits the requested
output format. It does not modify 0.8 or the runtime.

---

## 1. 문서 이해 요약 (Document Understanding)

The plan defines a **standalone-first RisuAI output quality layer**, MDASH-
inspired but not Archive Center-dependent. The product is a controlled
quality-control pipeline, not a memory system and not a hidden second main
model.

Core flow:

```text
RisuAI request
-> context reader
-> input preprocessing (support-only guidance, raw input untouched)
-> main model draft
-> output review (Output Check MDASH: role-separated readers)
-> Table Read MDASH (review the existing draft, find problems, no new prose)
-> patch planning (Output Enhance MDASH: bounded patches on mutable segments)
-> protected segment guard + verifier (veto only)
-> patched output OR original fallback
-> RisuAI display/save
```

Key invariants the document fixes:

- Raw user input is never rewritten; input lane is support-only guidance.
- Output is segmented into **protected / mutable / inspect-only** before any
  edit; only mutable prose can be patched, via bounded patch objects (not whole-
  response replacement).
- Auxiliary LLM failure must fail open to the original draft.
- Table Read does not write memory/DB and does not author new scenes; it
  reviews a draft and produces editorial constraints.
- Verifier has veto power, not creative rewrite power.
- Execution modes scale cost: `off`, `guidance_only`, `check_only`,
  `light_patch`, `single_panel`, `full_table_read`, with explicit call budgets
  and runtime downgrade.
- Architecture is modular (`StandaloneOutputCore`, `RisuAIAdapter`,
  `ProviderRegistry`, `OptionalArchiveAdapter`), explicitly avoiding another
  monolithic `Archive Center.js`. The harness doc proposes independent satellite
  plugins (`Guidance`, `Output Check`, `Table Read`, `Patch Guard`).

The strategic reference doc (`2.5-mdash-output-harness-plan.md`) keeps the
broader connected-Archive Center vision and the full Table Read math
(`2C + 7~9`), but the README correctly subordinates it to the standalone anchor.

The legacy docs confirm a hard lesson already learned: live `afterRequest`
whole-output polish was implemented, proven, and then **disabled** because
streaming recovery showed the raw output first and timeline pending artifacts
flickered. The 2.5 anchor is essentially the corrected design built on that
scar.

---

## 2. 현재 설계의 강점 (Strengths) / 약점 (Weaknesses)

### Well-captured (강점)

- **Correct core invariants.** "Don't rewrite user input", "support-only
  guidance", "segment before patch", "patch mutable prose only", "fail open to
  original", "verifier vetoes, does not create" are all present and stated as
  non-negotiable. These are the right load-bearing rules.
- **Learned from the real failure.** The whole-response `afterRequest` polish
  was already tried and burned the team (streaming flicker, phantom pending
  artifacts). The bounded-patch design is a direct, justified correction, not a
  theoretical preference.
- **Table Read is correctly re-scoped.** It is review of an existing draft +
  editorial constraints, explicitly NOT new-scene authoring and NOT memory
  write. This matches the user's stated intent exactly.
- **Cost is treated as a first-class constraint.** Explicit mode ladder + call
  budgets + runtime downgrade is mature. `full_table_read` (`2C + 7~9`) is
  correctly flagged as opt-in only, not a per-turn default.
- **Anti-monolith architecture stance.** Naming `StandaloneOutputCore` /
  `RisuAIAdapter` / `ProviderRegistry` / `OptionalArchiveAdapter` and the
  satellite split is a deliberate guard against re-creating a giant
  `Archive Center.js`.
- **Standalone vs connected honesty.** The harness doc requires the UI to show
  `standalone / Archive memory unavailable` so users are not misled.
- **Done/acceptance criteria exist** and are mostly testable (input unchanged,
  protected blocks survive, check-only changes nothing, verifier can veto).

### Weak / underspecified (약점)

- **Segmentation is named but not specified.** "Protected / mutable / inspect-
  only" is the load-bearing primitive of the entire design, yet there is no
  grammar, no precedence rule, and no statement of what happens when a protected
  marker is *malformed or unterminated* (e.g. a half-streamed `<img` or an
  unclosed code fence). If the segmenter is wrong, every downstream safety claim
  is void. This is the single biggest gap.
- **Streaming is acknowledged but not decided.** The docs say streaming "must
  degrade safely" and "must not pretend", but never state the concrete default:
  does `light_patch` simply *disable patching* under streaming, or attempt a
  post-display replace? The legacy doc shows post-display replace causes the
  exact flicker/phantom problem. The anchor leaves this as prose, not a rule.
- **"Suspiciously shorter" / "length ratio" is undefined.** Verifier rules
  reference length heuristics with no threshold and no handling of legitimate
  shortening (e.g. removing a leaked secret legitimately shortens text).
- **OOC / meta detection is asserted, not designed.** `ooc_or_meta_separation`
  and "meta leakage" appear in both input and secret-guard lanes, but there is
  no false-positive policy. In RP, in-character text frequently *looks* like OOC
  (system-like dialogue, characters discussing rules, parentheticals as action
  beats). Misclassification here either patches away real prose or skips real
  leaks.
- **Helper / submodel interference is listed but not bounded.** "Auxiliary
  request bypass" is named, but the doc does not define how the layer *detects*
  that the current request is itself an auxiliary/helper/submodel call (Risu
  translation, image prompt, OtherAx, Lightboard, module sub-calls). The 0.8
  AGENTS note warns against "hardcoded module names as the only routing
  authority", yet no alternative detection contract is given.
- **Provider/auth for the auxiliary calls is unspecified.** The layer needs its
  own model calls. Does it reuse Risu's provider, require a separate key, or use
  presets? Cost attribution, key storage, and rate-limit behavior are open.
- **No explicit idempotency / re-entrancy rule.** If `afterRequest` runs, the
  layer patches, and then a regenerate/swipe happens, can the layer re-patch an
  already-patched output? The legacy ledger handled this in connected mode;
  standalone has no equivalent statement.
- **Two competing architectures coexist.** The anchor describes one
  `StandaloneOutputCore` plugin; the harness doc describes 4-5 independent
  satellite plugins. The README subordinates the harness, but the anchor never
  explicitly says "MVP = single plugin, satellites are post-MVP". A reader could
  build the wrong shape.
- **Empty/degenerate drafts undefined.** What if the draft is empty, is pure
  markup (only an image tag + status block), or has zero mutable segments? The
  pipeline should short-circuit, but this is not stated.

---

## 3. MDASH 핵심 기능 반영 평가 (MDASH Coverage)

MDASH's functional value is role-separated review of a *finished* draft, where
multiple specialized readers find specific problem classes, and the result is
applied as constraints rather than a free rewrite. Coverage:

| MDASH function | Reflected? | Notes |
| --- | --- | --- |
| Role-separated readers (continuity/voice/pacing/etc.) | Partial | Output Check / Table Read name roles but the role catalog is not fixed. |
| Review an existing draft, not author a new one | Yes | Strong, explicit, repeated. |
| Findings become bounded constraints, not rewrites | Yes | Patch objects on mutable segments. |
| Cost/complexity dial | Yes | Mode ladder + budgets. |
| Single-panel vs full-table escalation | Yes | `single_panel` / `full_table_read`. |
| Cross-character / multi-actor reasoning | **Missing for standalone** | The harness doc keeps multi-character math, but standalone MVP does not say which subset survives without Archive context. |
| Verifier as veto | Yes | Good. |

Verdict: the **shape** of MDASH is well reflected, but the **substance**
(the concrete reader roles and what each one is allowed to flag) is left as a
TODO. For a standalone MVP this is acceptable *only if* the doc commits to a
minimal fixed reader set (see section 6). Otherwise "Table Read MDASH" risks
being a label with no defined behavior.

---

## 4. multi-agent / risu_agents / Serial Gradation 대비 차별점 (Differentiation)

- **risu_agents / multi-agent**: those orchestrate multiple *authoring* agents
  that each contribute or rewrite content, often pre-output. The 2.5 layer is
  explicitly **post-draft, non-authoring, veto-biased**. Differentiation is
  real and defensible: it never lets a helper agent author the visible scene.
- **Serial Gradation**: that is a graduated multi-pass *generation* pipeline
  (progressively refining the produced text). The 2.5 layer deliberately rejects
  whole-text refinement in favor of segment-scoped bounded patches and an
  original-text fallback. This is the key behavioral difference and the doc
  should state it directly.
- **Risk of collapse into the old pattern**: if patch bounds are loose (large
  mutable spans, no per-patch size cap), `light_patch` quietly becomes Serial
  Gradation with extra steps. The differentiation only holds if patch size and
  count are hard-capped. This is currently implicit.

Net: there *is* a genuine difference, but it lives entirely in the bounded-patch
+ fail-open + non-authoring constraints. The doc must make those constraints
numerically concrete or the differentiation evaporates in implementation.

---

## 5. 반드시 추가할 문서 항목 (Required Document Additions)

Concrete section titles to add to the anchor plan:

1. **"Segment Grammar and Protected-Marker Catalog"** — enumerate protected
   classes (image/asset tags, status panels/`<StatusBlock>`-style, module tags,
   code fences, HTML/markup, inline data tags, RisuAI render directives), define
   precedence, and define behavior for malformed/unterminated markers (rule:
   any uncertainty = treat as protected).
2. **"Streaming Decision Matrix"** — one table mapping each mode x
   (streaming on/off) to a concrete action. Default rule to state: under
   streaming, patching is *deferred to a single post-stream pass or disabled*;
   never post-display re-replace that re-flickers.
3. **"Auxiliary / Helper Request Detection Contract"** — how the layer decides
   the current request is itself a sub-call (translation, image prompt, OtherAx,
   Lightboard, module sub-calls) and must bypass entirely. Must not rely solely
   on hardcoded module names.
4. **"OOC / Meta False-Positive Policy"** — define what counts as OOC, the
   default bias (when uncertain, treat as in-character mutable prose, do not
   delete), and that secret/meta-leak removal is the *only* OOC action allowed.
5. **"Patch Bound Limits"** — hard numeric caps: max patches per response, max
   chars per patch, max total patched fraction, and the length-ratio veto
   threshold with its legitimate-shortening exception.
6. **"Idempotency and Re-entrancy"** — mark patched output so regenerate/swipe
   does not re-patch; define behavior on already-patched input.
7. **"Provider and Auth Model for Auxiliary Calls"** — where the aux model
   comes from, key handling, cost attribution, and rate-limit fallback.
8. **"Degenerate Input Short-Circuits"** — empty draft, markup-only draft, zero
   mutable segments => return original unchanged, no aux calls.
9. **"Failure Taxonomy and Telemetry"** — enumerate failure modes (aux timeout,
   parse fail, segmentation fail, budget exhausted) and confirm each maps to
   "return original" with a trace, not a hard error.

---

## 6. MVP 범위 제안 (MVP Scope)

Must be in the first standalone implementation:

- Single plugin (`StandaloneOutputCore`), not the 4-5 satellite split.
- Segment grammar + protected-marker catalog (section 5.1) — non-negotiable.
- Modes: `off`, `guidance_only`, `check_only`, `light_patch` only.
- One fixed, minimal reader role set (e.g. continuity + voice consistency +
  leak/secret guard). No open-ended role catalog yet.
- Bounded patch with hard caps; fail open to original on any failure.
- Streaming default = patching disabled or single post-stream pass (pick one,
  document it).
- Auxiliary/helper request bypass detection.
- Idempotency marker.
- Degenerate-draft short-circuit.
- Trace/telemetry of decisions (what was flagged, patched, vetoed).

---

## 7. 2.5 이후로 미룰 항목 (Defer Past MVP)

- `single_panel` and `full_table_read` (`2C + 7~9`) full Table Read math.
- Multi-character / cross-actor MDASH reasoning.
- Satellite multi-plugin decomposition.
- `OptionalArchiveAdapter` connected mode (memory-aware review).
- Open-ended/configurable reader role catalog.
- Input-side guidance beyond a no-op/trace stub (keep support-only, defer rich
  guidance until output lane is proven).

---

## 8. 수정된 구현 순서 (Revised Implementation Order)

1. **S0 Contracts (no model calls):** segment grammar + protected catalog +
   patch object schema + failure taxonomy + idempotency marker. Unit tests on
   segmentation, including malformed markers.
2. **S1 Pass-through harness:** plugin runs in `off`/`guidance_only`, proves it
   never alters input and never alters output; aux/helper bypass detection;
   degenerate short-circuit. (Preparatory — not "green" as a feature.)
3. **S2 check_only:** run readers, produce findings + trace, apply zero patches.
   Verify "check-only changes nothing".
4. **S3 light_patch:** bounded patch on mutable segments with hard caps +
   verifier veto + fail-open. Streaming decision enforced here.
5. **S4 Hardening:** length-ratio veto, OOC false-positive policy, budget
   downgrade under load, telemetry review.
6. **S5 (post-MVP):** single_panel, full Table Read, optional Archive adapter,
   satellites.

This reorders the work so the **segmenter and pass-through safety are built and
tested before any patching exists**, which inverts the legacy mistake of
shipping whole-output polish first and discovering segmentation/streaming
problems in production.

---

## 9. 위험 가정·스트리밍·태그 보호·OOC·헬퍼 간섭 명문화 제안 (Risk Hardening Language)

Add these explicit statements to the anchor:

- **Dangerous assumption to flag:** "the draft is well-formed and complete when
  the layer sees it." Under streaming recovery and partial generation it is not.
  Doc must state the layer treats input as possibly-truncated and protects
  unterminated markers.
- **Streaming:** state the single default behavior (defer to one post-stream
  pass OR disable patching while streaming) and forbid post-display re-replace,
  citing the legacy flicker/phantom-pending failure as the reason.
- **Image / status / module tags:** list them by class in the protected catalog,
  and state the rule "uncertain boundary => protected, never patched, never
  counted as mutable."
- **OOC false positives:** state the bias explicitly — in-character text that
  resembles OOC is preserved; only confirmed secret/system leakage is removed;
  ambiguous cases are left untouched and traced, not deleted.
- **Helper / submodel interference:** state that the layer must detect and fully
  bypass auxiliary requests (translation, image prompt, OtherAx, Lightboard,
  module sub-calls), must not double-process its own aux calls, and must not use
  hardcoded module names as the sole routing authority.

---

## 최종 권고 (Final Recommendation)

The plan's **principles are correct and hard-won** — bounded patch, fail-open,
non-authoring Table Read, no input rewrite. These should not change.

The plan's **mechanisms are under-specified** in exactly the places that
historically broke: segmentation grammar, streaming behavior, OOC detection,
helper bypass, and numeric patch bounds. Right now those are stated as
intentions, not contracts.

Recommendation: **proceed, but gate implementation behind the S0 contracts.**
Do not write any patching code until the segment grammar, protected-marker
catalog, streaming decision matrix, and patch bound limits exist as testable
specs. Add document sections 5.1–5.9 above, lock MVP to a single plugin with a
fixed minimal reader set, and defer Table Read math + satellites + Archive
adapter. With those additions the design is sound and meaningfully differentiated
from multi-agent / Serial Gradation; without them, `light_patch` will quietly
regress into the whole-output polish pattern that was already disabled once.



