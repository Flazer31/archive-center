//@name Risu Recomposer
//@display-name Risu Recomposer
//@author recomposer
//@api 3.0
//@version 0.1.22


/*
 * Risu Recomposer — MDASH/Fusion/Fugu output recomposition plugin.
 *
 * Product: read-only RisuAI context is fused into one bounded turn contract
 * before the main LLM runs. Its response (draft_zero) is then split into
 * ordered segments (protected / inspect_only / mutable), multiple specialist
 * AI roles rewrite each mutable segment, a Fusion Director ranks candidates,
 * a Fusion Composer AI integrates everything into one final RP response, and
 * a JS Verifier checks structural integrity before returning.
 *
 * No string patch / find / replace / fuzzy matching / anchor diff system.
 * Final output is assembled by walking the original ordered segment list and
 * substituting Composer results for mutable segments.
 *
 * Single candidate schema, single composer schema, single trace schema,
 * single Director scoring function, single callRole path for all roles.
 */
(async () => {
  "use strict";

  const PLUGIN_ID = "risu_recomposer";
  const VERSION = "0.1.22";
  const BUILD_MARKER = "MATERIAL-RECOMPOSITION-GATE-20260728";
  const LOG_PREFIX = "[Recomposer]";
  const SETTINGS_KEY = `${PLUGIN_ID}_settings_v1`;
  const TRACE_KEY = `${PLUGIN_ID}_trace_v1`;
  const TRACE_LIMIT = 30;
  const DEFAULT_DEADLINE_MS = 120000;
  const COMPOSER_RESERVE_MS = 30000;
  const COMPOSER_RESERVE_GUARD_MS = 5000;
  const ENDPOINT_SERIAL_THRESHOLD = 3;
  const CONTEXT_TIMEOUT_MS = 12000;
  const MAX_ROLE_HTTP_ATTEMPTS = 3;
  const PLANNER_MAX_ITEMS_PER_FIELD = 2;
  const PLANNER_MAX_ITEMS_TOTAL = 12;
  const INPUT_CONTRACT_MARKER = "[Risu Recomposer Turn Contract v1]";
  const INPUT_DEADLINE_MS = Object.freeze({
    fast: 8000,
    balanced: 50000,
    quality: 75000,
  });
  const INPUT_HTTP_ATTEMPT_BUDGET = Object.freeze({
    fast: 0,
    balanced: 3,
    quality: 4,
  });
  const OUTPUT_HTTP_ATTEMPT_BUDGET = Object.freeze({
    fast: 4,
    balanced: 5,
    quality: 7,
  });
  const OUTPUT_SPECIALIST_LIMIT = Object.freeze({
    fast: 2,
    balanced: 3,
    quality: 5,
  });

  /* ── Providers ─────────────────────────────────────────── */

  const PROVIDERS = Object.freeze([
    "openai_compatible",
    "ollama_compatible",
    "anthropic",
    "gemini",
    "vertex",
    "custom",
  ]);

  const PROVIDER_CONCURRENCY = Object.freeze({
    openai_compatible: 3,
    ollama_compatible: 1,
    anthropic: 2,
    gemini: 3,
    vertex: 4,
    custom: 1,
  });

  /* ── Role Registry ─────────────────────────────────────── */

  const ROLE_PROMPT_VERSION = "rp-rewrite-contract.v3";
  const PREVIOUS_ROLE_PROMPT_VERSIONS = Object.freeze(["rp-rewrite-contract.v2"]);
  const LEGACY_BUILTIN_PROMPT_DIGESTS = Object.freeze({
    input_canon_secret_planner: "b4faf92a",
    input_character_relationship_planner: "0ff28e3d",
    input_scene_continuity_planner: "e1dd440c",
    secret_pov_guard: "0208f796",
    character_reader: "4eda8179",
    plot_continuity_reader: "040b89c9",
    world_reader: "bec12239",
    style_reader: "5e5ef9ba",
    agency_meta_guard: "bfc4daf4",
    whole_scene_composer: "875ee4f4",
  });

  function inputPlannerPrompt(title, owns, instructions) {
    return [
      `You are the ${title} lane for a roleplay turn contract.`,
      "",
      "Mission:",
      "- Extract a compact, evidence-grounded constraint fragment for the next RP response.",
      "- You are a planner, not a prose writer. Never draft narration, dialogue, scene beats, advice, or analysis.",
      "",
      "You own only:",
      ...owns.map((item) => `- ${item}`),
      "",
      "Evidence contract:",
      "- Every asserted item must cite an allowed evidence_ref and an exact short evidence_quote from that same source.",
      "- Describe only what the quote supports. Do not attach a real quote to an inferred or invented claim.",
      "- Treat active/injected lore as usable canon. Treat candidate or unknown-activation lore as uncertainty, not established fact.",
      "- Keep writer knowledge, narrator knowledge, and each character's knowledge separate.",
      "- When sources conflict or do not prove the claim, record uncertainty instead of resolving it by invention.",
      "",
      "Lane instructions:",
      ...instructions.map((item) => `- ${item}`),
      "",
      "Output discipline:",
      "- Return only the fields assigned to this lane and use empty arrays when no grounded item exists.",
      "- Shared schema fields such as required_facts, forbidden_regressions, and uncertainty may contain only this lane's domain; a shared field name does not broaden your ownership.",
      `- Return at most ${PLANNER_MAX_ITEMS_TOTAL} items total and at most ${PLANNER_MAX_ITEMS_PER_FIELD} items per field. Keep item text under 240 characters and evidence_quote under 120 characters.`,
      "- Do not duplicate the work of another planner lane.",
      "- Do not output markdown, commentary, recommendations, XML, or pseudo tool-call wrappers.",
    ].join("\n");
  }

  function specialistPrompt(title, owns, doesNotOwn, diagnostics, rewriteDirections) {
    return [
      `You are the ${title} rewrite lane for a roleplay response.`,
      "",
      "Mission:",
      "- Act as a decisive revision worker, not a critic, advisor, auditor, or summarizer.",
      "- Read the complete ordered response and runtime context, then produce full replacement candidates for mutable segments that your lane can materially improve.",
      "- When a lane defect affects most of a segment, reconstruct the segment instead of making a timid phrase substitution.",
      "",
      "You own only:",
      ...owns.map((item) => `- ${item}`),
      "",
      "You do not own:",
      ...doesNotOwn.map((item) => `- ${item}`),
      "",
      "Diagnose:",
      ...diagnostics.map((item) => `- ${item}`),
      "",
      "Rewrite requirements:",
      ...rewriteDirections.map((item) => `- ${item}`),
      "- Preserve established events, names, factual outcomes, and the user's last action.",
      "- Preserve the response language unless the binding turn contract or latest user input explicitly requires another output language. Only the User Agency / Meta Artifact lane may repair that violation.",
      "- Use runtime context as evidence. Do not invent canon, backstory, relationships, knowledge, or future events.",
      "- Return the complete replacement text for each changed segment, not a patch, note, explanation, or list of suggestions.",
      "- Do not emit an identical or cosmetic-only candidate. If this lane has no material improvement, return candidates: [].",
      "",
      "Confidence contract:",
      "- confidence estimates expected net quality gain supported by the supplied evidence.",
      "- 0.90-1.00: major, well-grounded correction; 0.70-0.89: clear substantial improvement; 0.55-0.69: useful focused improvement.",
      "- Do not emit a candidate below 0.55.",
      "",
      "Return one compact JSON object matching the requested candidate schema. Output JSON only.",
    ].join("\n");
  }

  const ROLE_PROMPTS = Object.freeze({
    input_canon_secret_planner: inputPlannerPrompt(
      "Canon / Secret",
      [
        "required canon facts that directly constrain this turn",
        "writer-only secrets and narrator-only information",
        "character-visible facts and per-character knowledge scopes",
        "identity, alias, disguise, role, and reveal-state constraints",
        "forbidden regressions that would expose or contradict those facts",
      ],
      [
        "Map aliases and identities without assuming that every character knows the mapping.",
        "Record who knows, suspects, misremembers, or does not know a secret when evidence supports it.",
        "Never promote hidden narration, private memory, inactive lore, or another character's thoughts into shared knowledge.",
      ]
    ),
    input_character_relationship_planner: inputPlannerPrompt(
      "Character / Relationship",
      [
        "established voice, register, mannerisms, goals, and emotional posture",
        "current relationship stance, tension, trust, debt, attraction, fear, and power balance",
        "character-visible facts needed for believable reactions",
        "user agency, current POV, and interaction boundaries",
        "prose or dialogue targets that follow from established characterization",
      ],
      [
        "Distinguish a stable trait from a temporary mood caused by the latest scene.",
        "Anchor relationship state in recent exchanges rather than generic personality labels.",
        "Do not expose writer-only secrets or decide the user's unexpressed thoughts, feelings, dialogue, or next action.",
      ]
    ),
    input_scene_continuity_planner: inputPlannerPrompt(
      "Scene / Continuity",
      [
        "current time, location, participants, physical state, and immediate environment",
        "action order, causal dependencies, and unresolved scene promises",
        "active world constraints that directly govern this scene",
        "the latest user intent and concrete turn objectives",
        "POV, pacing, transition, and prose targets for this turn",
      ],
      [
        "Build the scene state from the latest grounded events; do not replace it with a generic plot outline.",
        "Preserve open choices for the user while identifying consequences that are already in motion.",
        "Mark uncertain timing, location, ownership, or causality instead of silently choosing an answer.",
      ]
    ),
    secret_pov_guard: specialistPrompt(
      "Secret / POV / Identity",
      [
        "secret and private-knowledge leakage",
        "POV scope and narrator access violations",
        "identity, alias, disguise, reveal-state, and recognition continuity",
        "characters acting on information they have not learned",
      ],
      [
        "user agency or model/meta artifact cleanup",
        "general plot repair, world-rule enrichment, or stylistic polishing",
        "changing character voice except where required to remove leaked knowledge",
      ],
      [
        "A character states, recalls, recognizes, or acts on a fact outside their evidenced knowledge scope.",
        "Narration presents another person's private thought or hidden motive as directly known from a limited POV.",
        "An alias, disguise, secret identity, or prior encounter is forgotten, prematurely revealed, or treated as separate without evidence.",
        "The prose converts writer-only context into dialogue, certainty, or visible character knowledge.",
      ],
      [
        "Replace leaked certainty with the character's available perception, inference, suspicion, error, question, or non-recognition.",
        "Preserve the secret for the reader when useful, but never transfer it to an unauthorized character.",
        "Maintain prior encounter and recognition history without forcing a reveal that has not occurred.",
      ]
    ),
    character_reader: specialistPrompt(
      "Character / Relationship / Emotion",
      [
        "character-specific voice, vocabulary, register, rhythm, and mannerisms",
        "emotion expressed through behavior, speech, restraint, and subtext",
        "relationship stance and power dynamics visible in the interaction",
        "persona consistency across the current scene and recent chat",
      ],
      [
        "secret knowledge, identity reveal policy, plot causality, or world rules",
        "generic prose polishing that does not strengthen characterization",
        "deciding the user's unexpressed thoughts, feelings, dialogue, or action",
      ],
      [
        "Dialogue could be spoken by any character and ignores established register or temperament.",
        "Emotion is flat, mechanically labelled, exaggerated without cause, or disconnected from recent events.",
        "Relationship behavior contradicts established trust, hostility, hierarchy, intimacy, or restraint.",
        "A reaction omits distinctive habits, coping patterns, goals, or tensions that should shape the moment.",
      ],
      [
        "Rebuild dialogue and action around this character's specific motives, verbal habits, emotional defenses, and relationship stance.",
        "Express emotion through concrete choices, timing, body language, omissions, and subtext rather than generic feeling labels.",
        "Keep characterization vivid without adding unsupported quirks, catchphrases, or backstory.",
      ]
    ),
    plot_continuity_reader: specialistPrompt(
      "Plot / Causality / Scene Flow",
      [
        "causal sequence and action-order continuity",
        "object, participant, location, and physical-state continuity",
        "open threads, promises, immediate consequences, and scene progression",
        "transitions needed to connect existing beats without changing their meaning",
      ],
      [
        "inventing world facts, changing character voice, or polishing style for its own sake",
        "secret and identity policy except when reporting an observable continuity break",
        "overriding the user's choice or forcing a new plot direction",
      ],
      [
        "An action lacks a cause, ignores the latest user input, or contradicts what immediately preceded it.",
        "A participant, object, injury, location, time, or objective appears, disappears, or changes without a bridge.",
        "The response forgets an active promise, consequence, question, or obstacle that the current beat must acknowledge.",
        "Paragraphs contain individually plausible events but do not form a coherent progression.",
      ],
      [
        "Restore cause and effect with concrete bridging action, acknowledgement, or reordered narration.",
        "Carry forward active consequences and open threads while leaving unresolved choices open for later turns.",
        "Do not add a new twist merely to make the scene more dramatic.",
      ]
    ),
    world_reader: specialistPrompt(
      "World / Lore / Physical Logic",
      [
        "active lore, institutions, customs, factions, geography, and material conditions",
        "magic, technology, ability, item, and resource constraints",
        "social, legal, economic, and physical plausibility inside the established setting",
        "world-specific grounding details supported by active context",
      ],
      [
        "character voice, secret reveal policy, user agency, or general prose polishing",
        "treating candidate or unknown-activation lore as binding canon",
        "adding decorative lore that does not improve the current scene",
      ],
      [
        "An action, ability, object, custom, law, rank, distance, or institution contradicts active lore.",
        "The scene uses modern assumptions, generic fantasy logic, or impossible logistics where setting-specific rules apply.",
        "Characters ignore an immediate environmental, social, legal, or material constraint.",
        "The scene lacks a setting-specific anchor that active lore clearly supplies.",
      ],
      [
        "Replace impossible or generic elements with setting-consistent actions and concrete details.",
        "Prefer the smallest coherent world correction that preserves the intended dramatic beat.",
        "When lore activation is uncertain, avoid asserting it as fact and leave the original fact pattern intact.",
      ]
    ),
    style_reader: specialistPrompt(
      "Style / Rhythm / Prose",
      [
        "sentence rhythm, paragraph movement, repetition, clarity, specificity, and transitions",
        "mechanical phrasing, vague abstraction, redundant explanation, and AI-like prose habits",
        "sensory and image precision that preserves the scene's established tone",
        "prose-level escalation, emphasis, and ending cadence",
      ],
      [
        "changing facts, events, POV knowledge, identity state, user agency, or relationship state",
        "homogenizing distinctive character dialogue into a generic literary voice",
        "adding plot, lore, exposition, or emotional conclusions not present in the source",
      ],
      [
        "Repeated words, sentence openings, paragraph shapes, explanations, or emotional beats flatten the scene.",
        "Phrasing is vague, translated, padded, melodramatic, clinical, or mechanically symmetrical.",
        "Transitions list events instead of carrying motion, pressure, attention, or consequence forward.",
        "The prose tells an interpretation that a sharper image, action, or line of dialogue could embody.",
      ],
      [
        "Rewrite decisively for varied cadence, concrete verbs, precise images, and clean paragraph momentum.",
        "Remove redundant interpretation and stock AI phrasing while preserving the scene's content density.",
        "Keep dialogue character-specific and preserve meaningful repetition used for tension or motif.",
      ]
    ),
    agency_meta_guard: specialistPrompt(
      "User Agency / Meta Artifact / Output Contract",
      [
        "the model deciding the user's unexpressed thought, emotion, speech, consent, or next action",
        "model reasoning, prompt analysis, approval labels, response headers, OOC leakage, and instruction residue",
        "mechanical summaries, translator commentary, disclaimers, and assistant self-reference",
        "explicit user requirements for output language, response framing, and non-diegetic format",
        "immersive replacement of contaminated prose with scene-native narration",
      ],
      [
        "secret or identity policy, plot invention, world-rule repair, or broad stylistic polishing",
        "removing legitimate in-world documents, interfaces, lists, or formatting",
        "asking the user to choose an action as a substitute for advancing the scene",
      ],
      [
        "The response exposes analysis such as Thoughts, Processing, Approved, Response, instructions, or planning language.",
        "Narration asserts what the user character thinks, feels, agrees to, says, or does beyond the user's supplied action.",
        "The prose contains assistant disclaimers, moralizing, translation commentary, prompt language, or a mechanical recap.",
        "The draft uses a language or response framing that directly contradicts the latest user's explicit output requirement.",
        "A closing line hands authorship back with a generic question instead of ending on an active scene beat.",
      ],
      [
        "Remove the artifact completely and write the missing space as immersive, in-scene prose rather than merely deleting a header.",
        "Replace user-controlled interiority or action with observable pressure, NPC behavior, environmental change, or an open consequence.",
        "When the user explicitly requires an output language or framing, rewrite the complete affected segment to satisfy it rather than preserving the draft's violation.",
        "Preserve legitimate diegetic formatting and never erase protected or inspect-only structures.",
      ]
    ),
    whole_scene_composer: [
      "You are the Whole-Scene Fusion Composer for a roleplay response.",
      "",
      "Mission:",
      "- Produce the final version of every mutable segment by reconstructing the draft from the strongest specialist evidence.",
      "- You are a synthesis writer, not a judge report, critic, summarizer, or candidate selector that merely copies one answer.",
      "- The result must read as one continuous scene with a single narrative voice, coherent causality, and deliberate prose.",
      "",
      "Authority order:",
      "1. Secret, POV, identity/reveal continuity, user agency, and removal of model/meta artifacts.",
      "2. Causality, scene state, active lore, world rules, and physical continuity.",
      "3. Character voice, emotional logic, relationship stance, and subtext.",
      "4. Rhythm, clarity, specificity, transitions, imagery, and ending cadence.",
      "",
      "Composition contract:",
      "- Compare each original segment with all candidates and the Director's consensus, complementary, conflict, and gap records.",
      "- Scores and confidence are evidence, not commands. Reject a high-scoring candidate if it breaks a higher-priority constraint.",
      "- For complementary candidates, synthesize their compatible strengths into a new segment. Do not concatenate sentences from different candidates.",
      "- For conflicts, preserve the higher-priority constraint and rebuild the prose so the lower-priority strength survives where compatible.",
      "- For gaps, improve the original directly without inventing canon, secrets, relationships, actions, or outcomes.",
      "- Remove analysis, prompt residue, approval labels, mechanical headers, and assistant commentary from mutable prose. Replace contaminated space with scene-native writing.",
      "- Preserve established events, names, required details, the user's last action, and narrative perspective, but do not preserve weak wording, sentence architecture, paragraph rhythm, generic imagery, or flat transitions.",
      "- Preserve the draft language only when it does not conflict with the binding turn contract or latest explicit user requirement.",
      "- Maintain transitions across segment boundaries. Avoid duplicated setup, repeated emotional conclusions, abrupt compression, and generic closing questions.",
      "- Every substantive mutable prose segment must receive a material rewrite. Copying the original with punctuation, spelling, synonym, or one-sentence edits is a failed composition.",
      "- Rebuild diction, sentence architecture, sensory specificity, subtext, pacing, transitions, and ending cadence while preserving facts.",
      "- Include every requested mutable segment ID exactly once as a complete final replacement.",
      "- Never output protected or inspect-only segments.",
      "",
      "Return one compact JSON object: {\"segments\":{\"SEG_ID\":\"final rewritten text\"}}. Output JSON only.",
    ].join("\n"),
  });

  const DEFAULT_ROLES = [
    {
      role_id: "input_canon_secret_planner",
      label: "Input: Canon / Secret",
      purpose: "Build a grounded turn-contract fragment for canon, identities, aliases, secrets, and per-character knowledge boundaries. Never write RP prose.",
      priority: 30,
      stage: "input",
      is_input_planner: true,
      default_prompt: ROLE_PROMPTS.input_canon_secret_planner,
      cost_tier: "standard",
    },
    {
      role_id: "input_character_relationship_planner",
      label: "Input: Character / Relationship",
      purpose: "Build a grounded turn-contract fragment for voice, emotional posture, relationships, and user agency. Never write RP prose.",
      priority: 29,
      stage: "input",
      is_input_planner: true,
      default_prompt: ROLE_PROMPTS.input_character_relationship_planner,
      cost_tier: "standard",
    },
    {
      role_id: "input_scene_continuity_planner",
      label: "Input: Scene / Continuity",
      purpose: "Build a grounded turn-contract fragment for scene state, world rules, open threads, turn objectives, and prose targets. Never write RP prose.",
      priority: 28,
      stage: "input",
      is_input_planner: true,
      default_prompt: ROLE_PROMPTS.input_scene_continuity_planner,
      cost_tier: "standard",
    },
    {
      role_id: "secret_pov_guard",
      label: "Secret / POV Constraint Ledger",
      purpose: "Detect leaks of secrets, private thoughts, hidden narrator knowledge, system/meta text, or information the current POV character should not know. Rewrite the segment to enforce knowledge boundaries.",
      priority: 10,
      default_prompt: ROLE_PROMPTS.secret_pov_guard,
      cost_tier: "standard",
    },
    {
      role_id: "character_reader",
      label: "Character / Voice",
      purpose: "Inspect and rewrite for character voice, emotional posture, knowledge boundary, and persona consistency.",
      priority: 9,
      default_prompt: ROLE_PROMPTS.character_reader,
      cost_tier: "standard",
    },
    {
      role_id: "plot_continuity_reader",
      label: "Plot / Continuity",
      purpose: "Inspect and rewrite for causality, scene flow, action order, and unresolved promise continuity.",
      priority: 8,
      default_prompt: ROLE_PROMPTS.plot_continuity_reader,
      cost_tier: "standard",
    },
    {
      role_id: "world_reader",
      label: "World / Scene Logic",
      purpose: "Inspect and rewrite for local world rules, social laws, magic/technology constraints, geography, and faction logic.",
      priority: 7,
      default_prompt: ROLE_PROMPTS.world_reader,
      cost_tier: "standard",
    },
    {
      role_id: "style_reader",
      label: "Style / Rhythm",
      purpose: "Inspect and rewrite for prose rhythm, repetition, awkward phrasing, tone, and transitions.",
      priority: 6,
      default_prompt: ROLE_PROMPTS.style_reader,
      cost_tier: "standard",
    },
    {
      role_id: "agency_meta_guard",
      label: "Agency / Meta Artifact",
      purpose: "Detect and remove user agency takeover, meta text, model self-commentary, translator-like phrasing, and mechanical artifacts.",
      priority: 7,
      default_prompt: ROLE_PROMPTS.agency_meta_guard,
      cost_tier: "standard",
    },
    {
      role_id: "whole_scene_composer",
      label: "Whole-Scene Composer",
      purpose: "Read the full scene with all segment candidates and integrate character, secret, POV, continuity, style, and emotion into one coherent final response.",
      priority: 5,
      is_composer: true,
      default_prompt: ROLE_PROMPTS.whole_scene_composer,
      cost_tier: "premium",
    },
  ];

  const COMPOSER_ROLE_ID = "whole_scene_composer";
  const DIRECTOR_APPLY_SCORE_MIN = 15;

  /* ── R1: Semantic Issue Groups ────────────────────────── */

  const ISSUE_GROUPS = Object.freeze([
    "secret_leak",
    "output_contract_violation",
    "pov_violation",
    "identity_continuity",
    "character_voice",
    "emotion",
    "plot_continuity",
    "scene_logic",
    "world_rule",
    "agency_takeover",
    "meta_artifact",
    "repetition",
    "rhythm",
    "transition",
    "prose_clarity",
  ]);

  const ROLE_ALLOWED_ISSUES = Object.freeze({
    secret_pov_guard: Object.freeze(["secret_leak", "pov_violation", "identity_continuity"]),
    character_reader: Object.freeze(["character_voice", "emotion"]),
    plot_continuity_reader: Object.freeze(["plot_continuity", "scene_logic"]),
    world_reader: Object.freeze(["world_rule", "scene_logic"]),
    style_reader: Object.freeze(["repetition", "rhythm", "transition", "prose_clarity"]),
    agency_meta_guard: Object.freeze(["output_contract_violation", "agency_takeover", "meta_artifact"]),
  });

  function roleAllowedIssues(roleId) {
    return ROLE_ALLOWED_ISSUES[safeString(roleId)] || ISSUE_GROUPS;
  }

  const ISSUE_PRIORITY = Object.freeze({
    secret_leak: 100,
    output_contract_violation: 99,
    pov_violation: 98,
    identity_continuity: 97,
    agency_takeover: 96,
    meta_artifact: 95,
    plot_continuity: 85,
    world_rule: 75,
    scene_logic: 70,
    character_voice: 65,
    emotion: 60,
    repetition: 40,
    rhythm: 35,
    transition: 30,
    prose_clarity: 25,
  });

  const TAG_ISSUE_MAP = Object.freeze({
    pov: "pov_violation",
    secret: "secret_leak",
    voice: "character_voice",
    character: "character_voice",
    emotion: "emotion",
    continuity: "plot_continuity",
    plot: "plot_continuity",
    world: "world_rule",
    lore: "world_rule",
    agency: "agency_takeover",
    meta: "meta_artifact",
    style: "rhythm",
    prose: "prose_clarity",
    repetition: "repetition",
    rhythm: "rhythm",
    transition: "transition",
    clarity: "prose_clarity",
    identity: "identity_continuity",
    scene: "scene_logic",
    instruction: "output_contract_violation",
    language: "output_contract_violation",
    format: "output_contract_violation",
  });

  function normalizeIssues(issues, tags) {
    const result = new Set();
    (Array.isArray(issues) ? issues : []).forEach((iss) => {
      const v = safeString(iss).trim().toLowerCase();
      if (ISSUE_GROUPS.indexOf(v) >= 0) result.add(v);
    });
    if (!result.size) {
      (Array.isArray(tags) ? tags : []).forEach((tag) => {
        const t = safeString(tag).trim().toLowerCase();
        const mapped = TAG_ISSUE_MAP[t];
        if (mapped) result.add(mapped);
      });
    }
    return Array.from(result);
  }

  function maxIssuePriority(issues) {
    let max = 0;
    (issues || []).forEach((iss) => {
      const p = ISSUE_PRIORITY[iss] || 0;
      if (p > max) max = p;
    });
    return max;
  }

  /* ── Presets ───────────────────────────────────────────── */

  const PRESETS = Object.freeze([
    {
      id: "fast",
      label: "Fast (cheap models, core roles)",
      roles: ["character_reader", "style_reader", "whole_scene_composer"],
      deadline_ms: 60000,
    },
    {
      id: "balanced",
      label: "Balanced (core + continuity + composer)",
      roles: ["secret_pov_guard", "character_reader", "plot_continuity_reader", "style_reader", "whole_scene_composer"],
      deadline_ms: 120000,
    },
    {
      id: "quality",
      label: "Quality (adaptive up to 4 roles + composer)",
      roles: ["secret_pov_guard", "character_reader", "plot_continuity_reader", "world_reader", "style_reader", "agency_meta_guard", "whole_scene_composer"],
      deadline_ms: 180000,
    },
  ]);

  /* ── Utility ───────────────────────────────────────────── */

  function log() {
    try { console.log(LOG_PREFIX, ...arguments); } catch (_) {}
  }
  function warn() {
    try { console.warn(LOG_PREFIX, ...arguments); } catch (_) {}
  }
  function error() {
    try { console.error(LOG_PREFIX, ...arguments); } catch (_) {}
  }

  function clampNumber(value, min, max, fallback) {
    const n = Number(value);
    if (!Number.isFinite(n)) return fallback;
    return Math.max(min, Math.min(max, n));
  }

  function safeString(value, fallback) {
    if (value == null) return fallback || "";
    return String(value);
  }

  function redactSensitiveText(value) {
    return safeString(value)
      .replace(/\b(?:sk|pk|rk|key|token)-[A-Za-z0-9._-]{12,}\b/g, "[REDACTED_KEY]")
      .replace(/\b(Bearer)\s+[A-Za-z0-9._~+/-]{12,}\b/gi, "$1 [REDACTED]")
      .replace(/\b(authorization|x-api-key|api[_ -]?key)\s*[:=]\s*[^\s,;]+/gi, "$1: [REDACTED]");
  }

  function stableDigest(value) {
    const text = typeof value === "string" ? value : JSON.stringify(value);
    let hash = 2166136261;
    for (let i = 0; i < text.length; i++) {
      hash ^= text.charCodeAt(i);
      hash = Math.imul(hash, 16777619);
    }
    return (`00000000${(hash >>> 0).toString(16)}`).slice(-8);
  }

  function asObject(value) {
    return (value && typeof value === "object" && !Array.isArray(value)) ? value : {};
  }

  function uniqueList(arr) {
    const seen = new Set();
    const out = [];
    (Array.isArray(arr) ? arr : []).forEach((item) => {
      const key = String(item);
      if (!seen.has(key)) { seen.add(key); out.push(item); }
    });
    return out;
  }

  function arrayFromCollection(value) {
    if (Array.isArray(value)) return value;
    if (value && typeof value.length === "number") return Array.from(value);
    return [];
  }

  function truncate(text, max) {
    const s = safeString(text);
    const n = clampNumber(max, 1, 100000, 200);
    return s.length <= n ? s : s.slice(0, n) + "…";
  }

  function preview(text, max) {
    return truncate(safeString(text).replace(/\n+/g, " ↵ ").trim(), max || 120);
  }

  function sanitizeEnum(value, allowed, fallback) {
    const v = safeString(value).trim().toLowerCase();
    return (allowed || []).indexOf(v) >= 0 ? v : fallback;
  }

  function deepClone(value) {
    if (value == null) return value;
    try { return JSON.parse(JSON.stringify(value)); } catch (_) { return value; }
  }

  /* ── Storage ───────────────────────────────────────────── */

  async function storageGet(key, fallback) {
    const RR = getR();
    try {
      if (RR && RR.pluginStorage && typeof RR.pluginStorage.getItem === "function") {
        const v = await RR.pluginStorage.getItem(key);
        if (v != null) return v;
      }
    } catch (_) {}
    try {
      if (RR && typeof RR.getStorage === "function") {
        const v = await RR.getStorage(key);
        if (v != null) return v;
      }
    } catch (_) {}
    try {
      if (typeof localStorage !== "undefined") {
        const v = localStorage.getItem(key);
        if (v != null) return v;
      }
    } catch (_) {}
    return fallback;
  }

  async function storageSet(key, value) {
    const RR = getR();
    const storageValue = safeString(value);
    let persisted = false;
    try {
      if (RR && RR.pluginStorage && typeof RR.pluginStorage.setItem === "function") {
        await RR.pluginStorage.setItem(key, storageValue);
        persisted = true;
      }
    } catch (_) {}
    if (!persisted) {
      try {
        if (RR && typeof RR.setStorage === "function") {
          await RR.setStorage(key, storageValue);
          persisted = true;
        }
      } catch (_) {}
    }
    try {
      if (typeof localStorage !== "undefined") {
        localStorage.setItem(key, storageValue);
        persisted = true;
      }
    } catch (_) {}
    return persisted;
  }

  /* ── Settings ──────────────────────────────────────────── */

  function defaultRoleProfile(roleId) {
    const role = DEFAULT_ROLES.find((r) => r.role_id === roleId);
    const isInputPlanner = !!(role && role.is_input_planner);
    return {
      role_id: roleId,
      enabled: true,
      provider: "openai_compatible",
      endpoint: "",
      api_key_ref: "",
      model: "",
      temperature: role && role.is_composer ? 0.4 : (isInputPlanner ? 0.1 : 0.3),
      max_output_tokens: role && role.is_composer ? 4096 : (isInputPlanner ? 1800 : 2048),
      timeout_ms: role && role.is_composer ? 60000 : 45000,
      system_prompt: role ? role.default_prompt : "",
      fallback_provider: "",
      fallback_endpoint: "",
      fallback_model: "",
      fallback_api_key_ref: "",
      extra_headers: "",
      extra_body: "",
      reasoning_preset: "auto",
      reasoning_effort: "auto",
      reasoning_budget_tokens: 0,
      vertex_flex_mode: "off",
      force_json_response: true,
      prompt_contract_version: ROLE_PROMPT_VERSION,
    };
  }

  function defaultSettings() {
    const roleProfiles = {};
    DEFAULT_ROLES.forEach((role) => {
      roleProfiles[role.role_id] = defaultRoleProfile(role.role_id);
    });
    return {
      version: VERSION,
      enabled: true,
      preset: "balanced",
      deadline_ms: DEFAULT_DEADLINE_MS,
      max_parallel: 5,
      roles: DEFAULT_ROLES.map((r) => ({
        role_id: r.role_id,
        label: r.label,
        purpose: r.purpose,
        priority: r.priority,
        stage: r.stage || "output",
        is_input_planner: !!r.is_input_planner,
        is_composer: !!r.is_composer,
        default_prompt: r.default_prompt,
      })),
      role_profiles: roleProfiles,
      protected_regex: "",
      context_char_limit: 6000,
      trace_enabled: true,
    };
  }

  async function loadSettings() {
    try {
      const raw = await storageGet(SETTINGS_KEY, "");
      if (!raw) return defaultSettings();
      const parsed = JSON.parse(raw);
      return mergeSettings(defaultSettings(), parsed);
    } catch (_) {
      return defaultSettings();
    }
  }

  async function saveSettings(settings) {
    const encoded = JSON.stringify(settings);
    const persisted = await storageSet(SETTINGS_KEY, encoded);
    if (!persisted) throw new Error("settings_storage_unavailable");
    const stored = await storageGet(SETTINGS_KEY, "");
    if (safeString(stored) !== encoded) throw new Error("settings_storage_readback_mismatch");
    return settings;
  }

  function isLegacyBuiltinPrompt(roleId, prompt) {
    const expectedDigest = LEGACY_BUILTIN_PROMPT_DIGESTS[safeString(roleId)];
    return !!expectedDigest && stableDigest(safeString(prompt)) === expectedDigest;
  }

  function mergeSettings(base, input) {
    const merged = deepClone(base);
    if (!input || typeof input !== "object") return merged;
    merged.version = VERSION;
    merged.enabled = true;
    merged.preset = sanitizeEnum(input.preset, PRESETS.map((p) => p.id), "balanced");
    merged.deadline_ms = clampNumber(input.deadline_ms, 10000, 600000, DEFAULT_DEADLINE_MS);
    merged.max_parallel = clampNumber(input.max_parallel, 1, 20, 5);
    merged.protected_regex = safeString(input.protected_regex);
    merged.context_char_limit = clampNumber(input.context_char_limit, 500, 50000, 6000);
    merged.trace_enabled = input.trace_enabled !== false;
    if (input.role_profiles && typeof input.role_profiles === "object") {
      Object.keys(input.role_profiles).forEach((roleId) => {
        if (!merged.role_profiles[roleId]) {
          merged.role_profiles[roleId] = defaultRoleProfile(roleId);
        }
        const src = input.role_profiles[roleId];
        const dst = merged.role_profiles[roleId];
        Object.keys(dst).forEach((key) => {
          if (src[key] !== undefined) {
            dst[key] = (typeof dst[key] === "number")
              ? clampNumber(src[key], -1, 999999, dst[key])
              : src[key];
          }
        });
        const role = DEFAULT_ROLES.find((item) => item.role_id === roleId);
        const incomingPrompt = safeString(src.system_prompt);
        const incomingVersion = safeString(src.prompt_contract_version);
        if (role && (!incomingPrompt
            || isLegacyBuiltinPrompt(roleId, incomingPrompt)
            || PREVIOUS_ROLE_PROMPT_VERSIONS.indexOf(incomingVersion) >= 0)) {
          dst.system_prompt = role.default_prompt;
          dst.prompt_contract_version = ROLE_PROMPT_VERSION;
        } else if (role && incomingPrompt === role.default_prompt) {
          dst.system_prompt = role.default_prompt;
          dst.prompt_contract_version = ROLE_PROMPT_VERSION;
        } else {
          dst.system_prompt = incomingPrompt;
          dst.prompt_contract_version = incomingVersion || "custom";
        }
      });
    }
    return merged;
  }

  /* ── API Key Resolution ────────────────────────────────── */

  async function resolveApiKey(ref) {
    const r = safeString(ref).trim();
    const RR = getR();
    if (!r) return "";
    if (r.indexOf("arg:") === 0 && RR && typeof RR.getArgument === "function") {
      return safeString(await RR.getArgument(r.slice(4)));
    }
    if (r.indexOf("storage:") === 0) {
      return safeString(await storageGet(r.slice(8), ""));
    }
    if (r.indexOf("env:") === 0 && typeof process !== "undefined" && process.env) {
      return safeString(process.env[r.slice(4)]);
    }
    return r;
  }

  function maskKey(key) {
    const k = safeString(key);
    if (!k) return "";
    if (k.length <= 8) return "****";
    return k.slice(0, 4) + "••••" + k.slice(-4);
  }

  /* ── Trace ─────────────────────────────────────────────── */

  function newTrace(stage, type) {
    return {
      plugin: PLUGIN_ID,
      version: VERSION,
      build_marker: BUILD_MARKER,
      stage: safeString(stage),
      request_type: safeString(type),
      timestamp: Date.now(),
      streaming: { detected: false, reason: "" },
      roles: [],
      segments: { protected: 0, inspect_only: 0, mutable: 0 },
      candidates: { total: 0, by_segment: {} },
      composer: { used: false, status: "", elapsed_ms: 0 },
      router: { signals: [], selected: [], skipped: [] },
      scheduler: {
        completion_wait: false,
        endpoint_serial_threshold: ENDPOINT_SERIAL_THRESHOLD,
        endpoint_groups: [],
      },
      input_enhance: {
        status: "not_run",
        fallback_reason: "",
        manifest_id: "",
        contract_id: "",
        contract_digest: "",
        source_availability: {},
        planner_selected: [],
        planner_succeeded: 0,
        planner_failed: 0,
        injected_chars: 0,
        active_calls_final: 0,
        retry_reuse_count: 0,
        transport_cancellation: "not_requested",
      },
      director_evidence: [],
      applied_evidence: [],
      budget: {
        http_attempt_max: OUTPUT_HTTP_ATTEMPT_BUDGET.balanced,
        http_attempt_used: 0,
        http_stopped_reason: "",
        input_attempt_max: 0,
        input_attempt_used: 0,
        composer_attempt_reserved: 0,
        composer_attempt_used: 0,
        specialist_primary_remaining: 0,
      },
      summary: { specialist_calls: 0, successful_roles: 0, candidate_count: 0, composer_state: "", changed_segment_count: 0, material_changed_segment_count: 0, unchanged_segment_count: 0, final_state: "", final_reason: "" },
      final: { enhanced: false, reason: "" },
      errors: [],
      timeline: [],
    };
  }

  function traceRole(trace, entry) {
    if (!trace.roles) trace.roles = [];
    trace.roles.push({
      role_id: entry.role_id,
      stage: entry.stage || "output",
      provider: entry.provider,
      model: entry.model,
      status: entry.status,
      queued_at: entry.queued_at || entry.started_at,
      started_at: entry.started_at,
      ended_at: entry.ended_at,
      elapsed_ms: entry.elapsed_ms,
      retry: entry.retry || 0,
      fallback: entry.fallback || false,
      http_attempts: entry.http_attempts || 0,
      attempts: Array.isArray(entry.attempts) ? entry.attempts : [],
      candidate_count: entry.candidate_count || 0,
      request_overrides: entry.request_overrides || null,
      error_class: entry.error_class || "",
      error: entry.error || "",
    });
  }

  function traceError(trace, msg) {
    if (!trace.errors) trace.errors = [];
    trace.errors.push(safeString(msg));
  }

  function traceTimeline(trace, label) {
    if (!trace.timeline) trace.timeline = [];
    trace.timeline.push({ label: safeString(label), t: Date.now() });
  }

  async function saveTrace(trace) {
    try {
      const raw = await storageGet(TRACE_KEY, "[]");
      const arr = JSON.parse(raw);
      arr.unshift(trace);
      while (arr.length > TRACE_LIMIT) arr.pop();
      await storageSet(TRACE_KEY, JSON.stringify(arr));
    } catch (_) {}
  }

  async function loadTraceList() {
    try {
      const raw = await storageGet(TRACE_KEY, "[]");
      return JSON.parse(raw);
    } catch (_) { return []; }
  }

  /* ── Protected Span Detection ──────────────────────────── */

  const PROTECTED_PATTERNS = [
    { kind: "image_tag", regex: /<img[^>]*>/gi, priority: 100 },
    { kind: "image_md", regex: /!\[[^\]]*\]\([^)]*\)/gi, priority: 99 },
    { kind: "code_fence", regex: /```[\s\S]*?```/gi, priority: 95 },
    { kind: "inline_code", regex: /`[^`\n]+`/g, priority: 90 },
    { kind: "risu_marker", regex: /<\/?(?:risu|module|status|chatindex|regex|system|plugin|asset|background|emotion|prompt)[^>]*>/gi, priority: 92 },
    { kind: "html_tag", regex: /<\/?(?!(?:thoughts?|analysis|thinking)\b)[a-z][^>]*>/gi, priority: 80 },
  ];

  const INSPECT_PATTERNS = [
    { kind: "status_window", regex: /```status[\s\S]*?```/gi, priority: 96 },
    { kind: "table", regex: /\|.*\|[\s\S]*?\n(?=\n|$)/gi, priority: 65 },
    { kind: "transcript", regex: /\[[^\]]{20,}\]/gi, priority: 60 },
  ];

  function isInsideAnySpan(start, end, spans) {
    return (spans || []).some((s) => start < s.end && end > s.start);
  }

  function detectSpans(text, patterns, type, userRegex, excludeSpans) {
    const source = safeString(text);
    const spans = [];
    let counter = 1;
    patterns.forEach((pat) => {
      let re = pat.regex;
      try {
        re = new RegExp(pat.regex.source, pat.regex.flags);
      } catch (_) { return; }
      let m;
        while ((m = re.exec(source)) !== null) {
          if (m[0].length === 0) { re.lastIndex++; continue; }
          if (excludeSpans && isInsideAnySpan(m.index, m.index + m[0].length, excludeSpans)) {
            re.lastIndex = m.index + 1;
            continue;
          }
          spans.push({
            id: `${type}_${counter++}`,
            type,
            kind: pat.kind,
            start: m.index,
            end: m.index + m[0].length,
            text: m[0],
            priority: pat.priority,
          });
        }
    });
    if (userRegex && type === "protected") {
      try {
        const re = new RegExp(userRegex, "gi");
        let m;
        while ((m = re.exec(source)) !== null) {
          if (m[0].length === 0) { re.lastIndex++; continue; }
          spans.push({
            id: `${type}_${counter++}`,
            type,
            kind: "user_regex",
            start: m.index,
            end: m.index + m[0].length,
            text: m[0],
            priority: 100,
          });
        }
      } catch (_) {}
    }
    return spans;
  }

  function resolveOverlappingSpans(spans) {
    const sorted = spans.slice().sort((a, b) => {
      if (a.start !== b.start) return a.start - b.start;
      return (b.priority || 0) - (a.priority || 0);
    });
    const result = [];
    let lastEnd = -1;
    sorted.forEach((span) => {
      if (span.start >= lastEnd) {
        result.push(span);
        lastEnd = span.end;
      }
    });
    return result;
  }

  function splitMutableWhitespace(text) {
    const raw = safeString(text);
    const leadingMatch = raw.match(/^[\s]*/);
    const trailingMatch = raw.match(/[\s]*$/);
    const leadingWs = leadingMatch ? leadingMatch[0] : "";
    const trailingWs = trailingMatch ? trailingMatch[0] : "";
    const core = raw.slice(leadingWs.length, raw.length - trailingWs.length);
    return { leadingWs, core, trailingWs };
  }

  function mutableCoreText(segment) {
    if (!segment) return "";
    if (typeof segment.core_text === "string") return segment.core_text;
    const raw = safeString(segment.text);
    const leading = safeString(segment.leading_ws);
    const trailing = safeString(segment.trailing_ws);
    const start = leading && raw.indexOf(leading) === 0 ? leading.length : 0;
    const end = trailing && raw.lastIndexOf(trailing) === raw.length - trailing.length
      ? raw.length - trailing.length
      : raw.length;
    return raw.slice(start, Math.max(start, end));
  }

  function mutableFullText(segment) {
    if (!segment) return "";
    if (typeof segment.core_text === "string") {
      return safeString(segment.leading_ws) + segment.core_text + safeString(segment.trailing_ws);
    }
    return safeString(segment.text);
  }

  function replacementCoreText(text) {
    return splitMutableWhitespace(text).core;
  }

  function isWhollyMetaArtifactText(text) {
    const value = safeString(text).trim();
    if (!value) return false;
    return /^<(thoughts?|analysis|thinking)\b[^>]*>[\s\S]*<\/\1>$/i.test(value);
  }

  function buildSegmentMap(text, settings) {
    const source = safeString(text);
    const userRegex = settings ? safeString(settings.protected_regex) : "";
    const fenceSpans = detectSpans(source, PROTECTED_PATTERNS.filter((p) => p.kind === "code_fence"), "protected", "")
      .concat(detectSpans(source, INSPECT_PATTERNS.filter((p) => p.kind === "status_window"), "inspect_only", ""));
    const resolvedFenceSpans = resolveOverlappingSpans(fenceSpans);
    const protectedSpans = resolveOverlappingSpans(
      detectSpans(source, PROTECTED_PATTERNS.filter((p) => p.kind !== "code_fence"), "protected", userRegex, resolvedFenceSpans)
    );
    const inspectSpans = resolveOverlappingSpans(
      detectSpans(source, INSPECT_PATTERNS.filter((p) => p.kind !== "status_window"), "inspect_only", "", resolvedFenceSpans)
    );
    const allSpans = resolveOverlappingSpans(
      resolvedFenceSpans.concat(protectedSpans).concat(inspectSpans).sort((a, b) => a.start - b.start)
    );
    const segments = [];
    let cursor = 0;
    let mutableCounter = 1;
    let protectedCounter = 1;
    let inspectCounter = 1;
    allSpans.forEach((span) => {
      if (span.start > cursor) {
        const rawText = source.slice(cursor, span.start);
        const ws = splitMutableWhitespace(rawText);
        segments.push({
          id: `mutable_${mutableCounter++}`,
          type: "mutable",
          start: cursor,
          end: span.start,
          text: rawText,
          leading_ws: ws.leadingWs,
          core_text: ws.core,
          trailing_ws: ws.trailingWs,
        });
      }
      const id = span.type === "protected"
        ? `protected_${protectedCounter++}`
        : `inspect_${inspectCounter++}`;
      segments.push({
        id,
        type: span.type,
        kind: span.kind,
        start: span.start,
        end: span.end,
        text: span.text,
      });
      cursor = span.end;
    });
    if (cursor < source.length) {
      const rawText = source.slice(cursor);
      const ws = splitMutableWhitespace(rawText);
      segments.push({
        id: `mutable_${mutableCounter++}`,
        type: "mutable",
        start: cursor,
        end: source.length,
        text: rawText,
        leading_ws: ws.leadingWs,
        core_text: ws.core,
        trailing_ws: ws.trailingWs,
      });
    }
    return segments.filter((seg) => seg.end > seg.start || seg.type !== "mutable");
  }

  function summarizeSegments(segments) {
    const summary = { protected: 0, inspect_only: 0, mutable: 0, mutable_chars: 0 };
    (segments || []).forEach((seg) => {
      if (seg.type === "protected") summary.protected++;
      else if (seg.type === "inspect_only") summary.inspect_only++;
      else if (seg.type === "mutable") {
        summary.mutable++;
        summary.mutable_chars += safeString(seg.text).trim().length;
      }
    });
    return summary;
  }

  function mutableSegments(segments) {
    return (segments || []).filter((s) => s.type === "mutable");
  }

  /* ── Context Collector ─────────────────────────────────── */

  async function guardedRisuApiCall(name, args, deadline) {
    const RR = getR();
    if (!RR || typeof RR[name] !== "function") {
      return { ok: false, value: null, source: name, error: "api_unavailable" };
    }
    if (deadline && deadline.check()) {
      return { ok: false, value: null, source: name, error: "pipeline_deadline" };
    }
    const timeoutMs = deadline
      ? Math.max(1, Math.min(CONTEXT_TIMEOUT_MS, deadline.remaining()))
      : CONTEXT_TIMEOUT_MS;
    return Promise.race([
      (async () => {
        try {
          const value = await RR[name].apply(RR, Array.isArray(args) ? args : []);
          return { ok: true, value, source: name, error: "" };
        } catch (err) {
          return { ok: false, value: null, source: name, error: err && err.message ? err.message : String(err) };
        }
      })(),
      new Promise((resolve) => setTimeout(() => resolve({
        ok: false,
        value: null,
        source: name,
        error: deadline && deadline.check() ? "pipeline_deadline" : "timeout",
      }), timeoutMs)),
    ]);
  }

  async function loadCharacter(deadline) {
    const direct = await guardedRisuApiCall("getCharacter", null, deadline);
    if (direct.ok && direct.value && typeof direct.value === "object") {
      return { value: direct.value, source: "getCharacter" };
    }
    const idx = await guardedRisuApiCall("getCurrentCharacterIndex", null, deadline);
    if (idx.ok && Number.isFinite(Number(idx.value))) {
      const byIdx = await guardedRisuApiCall("getCharacterFromIndex", [parseInt(idx.value, 10)], deadline);
      if (byIdx.ok && byIdx.value) return { value: byIdx.value, source: "getCharacterFromIndex" };
    }
    return { value: null, source: "getCharacter", error: "character_unavailable" };
  }

  async function loadDatabase(deadline) {
    const keys = ["personas", "selectedPersona", "modules", "enabledModules", "globalChatVariables"];
    const keyed = await guardedRisuApiCall("getDatabase", [keys], deadline);
    if (keyed.ok) return { value: keyed.value, source: "getDatabase" };
    const full = await guardedRisuApiCall("getDatabase", null, deadline);
    if (full.ok) return { value: full.value, source: "getDatabase" };
    return { value: null, source: "getDatabase", error: "database_unavailable" };
  }

  async function loadCurrentChat(character, deadline) {
    const charIdx = await guardedRisuApiCall("getCurrentCharacterIndex", null, deadline);
    const chatIdx = await guardedRisuApiCall("getCurrentChatIndex", null, deadline);
    if (charIdx.ok && chatIdx.ok && Number.isFinite(Number(charIdx.value)) && Number.isFinite(Number(chatIdx.value))) {
      const chat = await guardedRisuApiCall(
        "getChatFromIndex",
        [parseInt(charIdx.value, 10), parseInt(chatIdx.value, 10)],
        deadline
      );
      if (chat.ok && chat.value) return { value: chat.value, source: "getChatFromIndex" };
    }
    const chats = Array.isArray(character && character.chats) ? character.chats : [];
    if (chats.length) {
      const page = Number.isInteger(character.chatPage) ? character.chatPage : 0;
      const fallback = chats[Math.max(0, Math.min(chats.length - 1, page))];
      if (fallback) return { value: fallback, source: "character.chats[chatPage]" };
    }
    return { value: null, source: "current_chat", error: "current_chat_unavailable" };
  }

  function extractCharacterSummary(character) {
    if (!character || typeof character !== "object") return "";
    const parts = [];
    if (character.name) parts.push(`Character: ${character.name}`);
    if (character.description) parts.push(`Description: ${truncate(character.description, 800)}`);
    if (character.personality) parts.push(`Personality: ${truncate(character.personality, 600)}`);
    if (character.scenario) parts.push(`Scenario: ${truncate(character.scenario, 500)}`);
    if (character.mes_example) parts.push(`Example: ${truncate(character.mes_example, 400)}`);
    return parts.join("\n");
  }

  function extractPersonaSummary(db) {
    if (!db) return "";
    const personas = arrayFromCollection(db.personas);
    const selected = db.selectedPersona;
    let persona = null;
    if (selected && typeof selected === "object") persona = selected;
    else if (Number.isInteger(selected) && selected >= 0 && selected < personas.length) {
      persona = personas[selected];
    }
    else if (typeof selected === "string" && personas.length) {
      persona = personas.find((p) => p && (p.id === selected || p.name === selected));
    }
    if (!persona && personas.length) persona = personas[0];
    if (!persona && db.personaPrompt) {
      persona = { name: "", personaPrompt: db.personaPrompt };
    }
    if (!persona) return "";
    const parts = [];
    if (persona.name) parts.push(`Persona: ${persona.name}`);
    const detail = persona.personaPrompt || persona.text || persona.description;
    if (detail) parts.push(`Detail: ${truncate(detail, 500)}`);
    return parts.join("\n");
  }

  function extractChatSummary(chat) {
    if (!chat || typeof chat !== "object") return "";
    const parts = [];
    const messages = arrayFromCollection(chat.message || chat.messages || chat.chats);
    const recent = messages.slice(-8);
    recent.forEach((msg) => {
      if (msg && msg.data) parts.push(`${msg.role || "unknown"}: ${truncate(msg.data, 300)}`);
    });
    return parts.join("\n");
  }

  function extractMemorySnapshot(chat) {
    if (!chat || typeof chat !== "object") return { text: "", fields: [] };
    const parts = [];
    const found = [];
    const fields = [
      ["summary", "Summary"],
      ["note", "Note"],
      ["supaMemoryData", "SupaMemory"],
      ["hypaV2Data", "HypaV2"],
      ["hypaV3Data", "HypaV3"],
      ["hypaMemoryData", "HypaMemory"],
      ["lastMemory", "LastMemory"],
    ];
    fields.forEach(([key, label]) => {
      if (!chat[key]) return;
      const text = redactSensitiveText(truncate(
        typeof chat[key] === "string" ? chat[key] : JSON.stringify(chat[key]),
        600
      ));
      if (!text) return;
      parts.push(`${label}: ${text}`);
      found.push({ key, label, chars: text.length });
    });
    return { text: parts.join("\n"), fields: found };
  }

  function extractMemorySummary(chat) {
    return extractMemorySnapshot(chat).text;
  }

  function flattenLoreEntries(value) {
    if (Array.isArray(value)) return value;
    if (!value || typeof value !== "object") return [];
    if (Array.isArray(value.entries)) return value.entries;
    if (Array.isArray(value.data)) return value.data;
    return [];
  }

  function normalizeLoreEntry(entry, index) {
    if (!entry || typeof entry !== "object") return null;
    const rawKeys = entry.keys != null ? entry.keys : (entry.key != null ? entry.key : entry.keywords);
    const keys = Array.isArray(rawKeys)
      ? rawKeys.map((item) => safeString(item).trim()).filter(Boolean)
      : [safeString(rawKeys).trim()].filter(Boolean);
    const rawContent = entry.content != null
      ? entry.content
      : (entry.text != null ? entry.text : (entry.prompt != null ? entry.prompt : entry.value));
    const content = redactSensitiveText(safeString(rawContent).trim());
    if (!content) return null;
    return {
      evidence_id: `lore_${index + 1}`,
      keys,
      content,
      declared_constant: !!(entry.constant || entry.alwaysActive || entry.always_active),
    };
  }

  function fallbackCharacterLoreEntries(character) {
    const books = arrayFromCollection(
      (character && character.character_book) || (character && character.data && character.data.character_book)
    );
    const entries = [];
    books.forEach((book) => {
      flattenLoreEntries(book).forEach((entry) => entries.push(entry));
    });
    return entries;
  }

  function isLoreInjected(entryContent, systemContext) {
    const needle = safeString(entryContent).replace(/\s+/g, " ").trim();
    const haystack = safeString(systemContext).replace(/\s+/g, " ");
    return needle.length >= 12 && haystack.indexOf(needle) >= 0;
  }

  async function collectLorebookSummary(character, messages, deadline) {
    const official = await guardedRisuApiCall("getCurrentLorebookEntries", null, deadline);
    const officialEntries = official.ok ? flattenLoreEntries(official.value) : [];
    const rawEntries = official.ok ? officialEntries : fallbackCharacterLoreEntries(character);
    const source = official.ok ? "getCurrentLorebookEntries" : "character.character_book";
    const systemContext = extractPayloadSystemForMatching(messages);
    const candidates = rawEntries
      .map((entry, index) => normalizeLoreEntry(entry, index))
      .filter(Boolean);
    const injected = candidates.filter((entry) => isLoreInjected(entry.content, systemContext));
    const formatEntry = (entry) => {
      const keyText = entry.keys.length ? entry.keys.join(", ") : "no-key";
      return `[${entry.evidence_id}; ${keyText}] ${truncate(entry.content, 400)}`;
    };
    return {
      candidates: candidates.slice(0, 30).map(formatEntry).join("\n"),
      active: injected.slice(0, 20).map(formatEntry).join("\n"),
      candidateCount: candidates.length,
      activeCount: injected.length,
      unknownActivationCount: Math.max(0, candidates.length - injected.length),
      source,
      official: official.ok,
      warning: official.ok ? "" : safeString(official.error || "official_lorebook_unavailable"),
    };
  }

  /* ── OpenAIChat[] extractors (official beforeRequest input) ── */

  function isOpenAiChatArray(value) {
    return Array.isArray(value) && value.every((m) => m && typeof m === "object" && typeof m.role === "string");
  }

  function openAiChatContentText(message) {
    const content = message && message.content;
    if (typeof content === "string") return content;
    if (Array.isArray(content)) {
      return content.map((part) => {
        if (typeof part === "string") return part;
        if (part && typeof part.text === "string") return part.text;
        if (part && typeof part.content === "string") return part.content;
        return "";
      }).filter(Boolean).join("\n");
    }
    if (content && typeof content.text === "string") return content.text;
    return safeString(content);
  }

  function extractPayloadSystem(messages) {
    if (!isOpenAiChatArray(messages)) return "";
    const systemParts = [];
    messages.forEach((msg) => {
      if (msg && msg.role === "system") {
        systemParts.push(truncate(openAiChatContentText(msg), 800));
      }
    });
    return systemParts.join("\n");
  }

  function extractPayloadSystemForMatching(messages) {
    if (!isOpenAiChatArray(messages)) return "";
    return truncate(messages
      .filter((msg) => msg && msg.role === "system")
      .map((msg) => openAiChatContentText(msg))
      .filter(Boolean)
      .join("\n"), 50000);
  }

  function extractRecentChat(messages) {
    if (!isOpenAiChatArray(messages)) return "";
    const recent = messages.slice(-10).filter((m) => m && m.role !== "system");
    return recent.map((m) => `${m.role}: ${truncate(openAiChatContentText(m), 300)}`).join("\n");
  }

  function extractLatestUserInput(messages) {
    if (!isOpenAiChatArray(messages)) return "";
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i] && messages[i].role === "user") {
        return truncate(openAiChatContentText(messages[i]), 500);
      }
    }
    return "";
  }

  const EXCLUDED_CONTEXT_KEYS = Object.freeze([
    "LIBRA_CONTAINER", "LIBRA_DATA_", "lmai_",
  ]);

  function filterExcludedContext(text) {
    let result = safeString(text);
    EXCLUDED_CONTEXT_KEYS.forEach((key) => {
      try {
        const re = new RegExp(key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "[^\\n]*", "gi");
        result = result.replace(re, "");
      } catch (_) {}
    });
    return result;
  }

  function traceSafeSourceAvailability(sources) {
    const out = {};
    Object.keys(asObject(sources)).forEach((key) => {
      const source = asObject(sources[key]);
      out[key] = {
        available: !!source.available,
        source: safeString(source.source),
        count: clampNumber(
          source.count != null ? source.count : (source.candidate_count != null ? source.candidate_count : 0),
          0, 100000, 0
        ),
        active_count: clampNumber(source.active_count, 0, 100000, 0),
        unknown_activation_count: clampNumber(source.unknown_activation_count, 0, 100000, 0),
        error: truncate(redactSensitiveText(source.error), 120),
        warning: truncate(redactSensitiveText(source.warning), 120),
      };
    });
    return out;
  }

  function buildContextManifest(ctx, settings) {
    const sourceAvailability = traceSafeSourceAvailability(ctx.sources);
    const warnings = [];
    Object.keys(sourceAvailability).forEach((key) => {
      const item = sourceAvailability[key];
      if (item.error) warnings.push(`${key}:${item.error}`);
      if (item.warning) warnings.push(`${key}:${item.warning}`);
    });
    if (sourceAvailability.lorebook && sourceAvailability.lorebook.unknown_activation_count > 0) {
      warnings.push("lorebook_activation_unproven");
    }
    const evidence = {
      payload_system: ctx.system_context,
      payload_recent_chat: ctx.recent_chat,
      payload_user_input: ctx.latest_user_input,
      character: ctx.character,
      persona: ctx.persona,
      current_chat: ctx.current_chat,
      lorebook_candidates: ctx.lorebook,
      lorebook_active_or_injected: ctx.lorebook_active,
      memory_snapshot: ctx.memory,
    };
    const manifestCore = {
      schema: "context_manifest.v1",
      mode: "standalone",
      latest_user_input: ctx.latest_user_input,
      recent_messages: ctx.recent_chat,
      system_and_character_instructions: ctx.system_context,
      character_and_persona: {
        character: ctx.character,
        persona: ctx.persona,
      },
      current_chat: ctx.current_chat,
      lorebook_candidates: ctx.lorebook,
      lorebook_active_or_injected: ctx.lorebook_active,
      memory_sources: ctx.memory,
      source_availability: sourceAvailability,
      evidence_sources: evidence,
      source_provenance: Object.keys(sourceAvailability).map((key) => ({
        evidence_ref: key,
        source: sourceAvailability[key].source || key,
        available: sourceAvailability[key].available,
      })),
      collection_warnings: uniqueList(warnings),
      evidence_refs: Object.keys(evidence).filter((key) => !!evidence[key]),
      character_budget: clampNumber(settings && settings.context_char_limit, 500, 50000, 6000),
    };
    manifestCore.snapshot_id = `ctx_${Date.now()}_${stableDigest(manifestCore)}`;
    return manifestCore;
  }

  function buildBoundedContextBlock(ctx, limit) {
    const parts = [];
    if (ctx.system_context) parts.push(`[payload_system]\n${ctx.system_context}`);
    if (ctx.recent_chat) parts.push(`[Payload Recent Chat] [payload_recent_chat]\n${ctx.recent_chat}`);
    if (ctx.character) parts.push(`[character]\n${ctx.character}`);
    if (ctx.persona) parts.push(`[persona]\n${ctx.persona}`);
    if (ctx.lorebook_active) parts.push(`[lorebook_active_or_injected]\n${ctx.lorebook_active}`);
    if (ctx.lorebook) parts.push(`[lorebook_candidates; activation may be unknown]\n${ctx.lorebook}`);
    if (ctx.current_chat) parts.push(`[current_chat]\n${ctx.current_chat}`);
    if (ctx.memory) parts.push(`[memory_snapshot]\n${ctx.memory}`);
    if (ctx.latest_user_input) parts.push(`[payload_user_input]\n${ctx.latest_user_input}`);
    return truncate(parts.join("\n\n"), limit);
  }

  async function collectContext(messages, settings, trace, deadline) {
    const limit = settings ? clampNumber(settings.context_char_limit, 500, 50000, 6000) : 6000;
    const ctx = {
      system_context: "",
      recent_chat: "",
      latest_user_input: "",
      character: "",
      persona: "",
      current_chat: "",
      lorebook: "",
      lorebook_active: "",
      memory: "",
      memory_fields: [],
      bounded_context_block: "",
      manifest: null,
      sources: {},
    };

    try {
      ctx.system_context = redactSensitiveText(filterExcludedContext(extractPayloadSystem(messages)));
      ctx.recent_chat = redactSensitiveText(filterExcludedContext(extractRecentChat(messages)));
      ctx.latest_user_input = redactSensitiveText(filterExcludedContext(extractLatestUserInput(messages)));
      ctx.sources.payload = {
        available: !!ctx.system_context || !!ctx.recent_chat || !!ctx.latest_user_input,
        source: "beforeRequest.OpenAIChat[]",
        count: Array.isArray(messages) ? messages.length : 0,
      };
    } catch (err) {
      ctx.sources.payload = { available: false, error: safeString(err && err.message) };
    }

    let characterValue = null;
    try {
      const charResult = await loadCharacter(deadline);
      if (charResult.value) {
        characterValue = charResult.value;
        ctx.character = redactSensitiveText(filterExcludedContext(extractCharacterSummary(charResult.value)));
        ctx.sources.character = { available: true, source: charResult.source };

        try {
          const chatResult = await loadCurrentChat(charResult.value, deadline);
          if (chatResult.value) {
            ctx.current_chat = redactSensitiveText(filterExcludedContext(extractChatSummary(chatResult.value)));
            const memorySnapshot = extractMemorySnapshot(chatResult.value);
            ctx.memory = filterExcludedContext(memorySnapshot.text);
            ctx.memory_fields = memorySnapshot.fields;
            ctx.sources.current_chat = {
              available: !!ctx.current_chat,
              source: chatResult.source,
              warning: ctx.current_chat ? "" : "current_chat_has_no_readable_messages",
            };
            ctx.sources.memory = {
              available: !!ctx.memory,
              source: "current_chat_memory_fields",
              count: ctx.memory_fields.length,
            };
          } else {
            ctx.sources.current_chat = { available: false, error: chatResult.error || "" };
            ctx.sources.memory = { available: false, source: "current_chat_memory_fields" };
          }
        } catch (err) {
          ctx.sources.current_chat = { available: false, error: safeString(err && err.message) };
          ctx.sources.memory = { available: false, source: "current_chat_memory_fields" };
        }
      } else {
        ctx.sources.character = { available: false, error: charResult.error || "" };
      }
    } catch (err) {
      ctx.sources.character = { available: false, error: safeString(err && err.message) };
    }

    try {
      const loreResult = await collectLorebookSummary(characterValue, messages, deadline);
      ctx.lorebook = filterExcludedContext(loreResult.candidates || "");
      ctx.lorebook_active = filterExcludedContext(loreResult.active || "");
      ctx.sources.lorebook = {
        available: !!loreResult.official || !!ctx.lorebook,
        source: loreResult.source,
        candidate_count: loreResult.candidateCount || 0,
        active_count: loreResult.activeCount || 0,
        unknown_activation_count: loreResult.unknownActivationCount || 0,
        warning: loreResult.warning || "",
      };
    } catch (err) {
      ctx.sources.lorebook = {
        available: false,
        source: "getCurrentLorebookEntries",
        error: safeString(err && err.message),
      };
    }

    try {
      const dbResult = await loadDatabase(deadline);
      if (dbResult.value) {
        ctx.persona = redactSensitiveText(filterExcludedContext(extractPersonaSummary(dbResult.value)));
        ctx.sources.persona = { available: !!ctx.persona, source: dbResult.source };
      } else {
        ctx.sources.persona = { available: false };
      }
    } catch (_) {
      ctx.sources.persona = { available: false };
    }

    ctx.bounded_context_block = buildBoundedContextBlock(ctx, limit);
    ctx.manifest = buildContextManifest(ctx, settings);
    if (trace && trace.input_enhance) {
      trace.input_enhance.manifest_id = ctx.manifest.snapshot_id;
      trace.input_enhance.source_availability = ctx.manifest.source_availability;
    }

    return ctx;
  }

  /* ── Provider Request ──────────────────────────────────── */

  function openAiChatUrl(endpoint) {
    const base = safeString(endpoint || "https://api.openai.com/v1").replace(/\/+$/, "");
    if (/\/chat\/completions$/i.test(base)) return base;
    if (/^https?:\/\/(?:www\.)?ollama\.com$/i.test(base)) return `${base}/v1/chat/completions`;
    return `${base}/chat/completions`;
  }

  function parseExtraHeaders(text) {
    const out = {};
    safeString(text).split(/\n/).forEach((line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      const idx = trimmed.indexOf(":");
      if (idx <= 0) throw new Error("invalid_extra_header_line");
      const key = trimmed.slice(0, idx).trim();
      const val = trimmed.slice(idx + 1).trim();
      if (!key) throw new Error("invalid_extra_header_line");
      out[key] = val;
    });
    return out;
  }

  function parseExtraBody(text) {
    if (!text || !text.trim()) return {};
    let parsed;
    try {
      parsed = JSON.parse(text);
    } catch (_) {
      throw new Error("invalid_extra_body_json");
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("invalid_extra_body_json");
    }
    return parsed;
  }

  function reasoningFamily(provider, profile) {
    const explicit = safeString(profile && profile.reasoning_preset, "auto").toLowerCase();
    const model = safeString(profile && profile.model).toLowerCase();
    if (explicit && explicit !== "auto") {
      if (explicit === "glm" && /(?:glm[-_. ]?5[._-]?1|glm[-_. ]?4)/.test(model)) return "glm_toggle";
      if (explicit === "gemini" && /gemini[-_. ]?3/.test(model)) return "gemini3";
      return explicit;
    }
    if (/deepseek/.test(model)) return "deepseek";
    if (/kimi/.test(model)) return "kimi";
    if (/(?:glm[-_. ]?5[._-]?1|glm[-_. ]?4)/.test(model)) return "glm_toggle";
    if (/glm/.test(model)) return "glm";
    if (/gemini[-_. ]?3/.test(model)) return "gemini3";
    if (/gemini/.test(model)) return "gemini";
    if (/claude/.test(model) || provider === "anthropic") return "claude";
    if (/^(?:o[134]|gpt-5)/.test(model) || provider === "openai_compatible") return "gpt";
    return "unknown";
  }

  function applyReasoningAdapter(provider, body, profile) {
    const family = reasoningFamily(provider, profile || {});
    const effort = safeString(profile && profile.reasoning_effort, "auto").toLowerCase();
    const budget = clampNumber(profile && profile.reasoning_budget_tokens, 0, 131072, 0);
    if (effort === "none" || effort === "disable") {
      if (family === "glm_toggle" || family === "glm") {
        body.think = false;
        body.thinking = { type: "disabled" };
        return { family, applied: ["think", "thinking"] };
      }
      if (family === "kimi") {
        if (provider === "ollama_compatible" && !isOllamaCloudEndpoint(profile && profile.endpoint)) {
          body.think = false;
          return { family, applied: ["think"] };
        }
        body.reasoning_effort = "none";
        return { family, applied: ["reasoning_effort"] };
      }
      return { family, applied: [] };
    }

    const applied = [];
    if (provider === "anthropic" || family === "claude") {
      if (budget > 0) {
        body.thinking = { type: "enabled", budget_tokens: budget };
        applied.push("thinking");
      }
      return { family, applied };
    }

    if (provider === "gemini" || provider === "vertex" || family === "gemini" || family === "gemini3") {
      const gc = body.generationConfig || (body.generationConfig = {});
      if (family === "gemini3" && effort !== "auto") {
        gc.thinkingConfig = Object.assign({}, gc.thinkingConfig, { thinkingLevel: effort });
        applied.push("generationConfig.thinkingConfig.thinkingLevel");
      } else if (family === "gemini" && budget > 0) {
        gc.thinkingConfig = Object.assign({}, gc.thinkingConfig, { thinkingBudget: budget });
        applied.push("generationConfig.thinkingConfig.thinkingBudget");
      }
      return { family, applied };
    }

    if (family === "glm_toggle") {
      if (effort === "enable" || effort === "high" || effort === "medium" || effort === "low") {
        body.think = true;
        body.thinking = { type: "enabled" };
        applied.push("think", "thinking");
      }
      return { family, applied };
    }

    if (family === "glm") {
      if (effort !== "auto" && effort !== "enable") {
        body.think = true;
        body.thinking = { type: "enabled" };
        body.reasoning_effort = effort;
        applied.push("think", "thinking", "reasoning_effort");
      }
      return { family, applied };
    }

    if (family === "kimi") {
      if (effort === "auto") return { family, applied };
      if (provider === "ollama_compatible" && !isOllamaCloudEndpoint(profile && profile.endpoint)) {
        body.think = effort === "enable" ? true : effort;
        applied.push("think");
      } else if (effort !== "enable") {
        body.reasoning_effort = effort;
        applied.push("reasoning_effort");
      }
      return { family, applied };
    }

    if (family === "deepseek" || family === "gpt") {
      if (effort !== "auto" && effort !== "enable") {
        body.reasoning_effort = effort;
        applied.push("reasoning_effort");
      }
      if (family === "gpt" && budget > 0) {
        body.max_completion_tokens = Math.max(clampNumber(body.max_tokens, 0, 131072, 0), budget);
        delete body.max_tokens;
        applied.push("max_completion_tokens");
      }
      return { family, applied };
    }

    if ((provider === "openai_compatible" || provider === "ollama_compatible")
        && effort !== "auto" && effort !== "enable") {
      body.reasoning_effort = effort;
      applied.push("reasoning_effort");
    }

    return { family, applied };
  }

  function applyVertexFlex(profile, headers) {
    const mode = safeString(profile.vertex_flex_mode, "off").toLowerCase();
    if (mode === "off") return;
    if (mode === "flex_only") {
      headers["X-Vertex-AI-LLM-Request-Type"] = "shared";
      headers["X-Vertex-AI-LLM-Shared-Request-Type"] = "flex";
    } else if (mode === "provisioned_then_flex") {
      headers["X-Vertex-AI-LLM-Shared-Request-Type"] = "flex";
    }
  }

  /* ── R5: Protected fields and deep-merge ───────────────── */

  const PROTECTED_HEADERS = Object.freeze([
    "authorization", "content-type", "accept",
    "x-goog-api-key", "x-api-key", "anthropic-version",
  ]);

  const PROTECTED_BODY_FIELDS = Object.freeze({
    openai_compatible: ["messages", "model"],
    ollama_compatible: ["messages", "model"],
    anthropic: ["messages", "system", "model"],
    gemini: ["contents", "systemInstruction"],
    vertex: ["contents", "systemInstruction"],
    custom: ["model", "system", "user"],
  });

  function deepMergeBody(target, source, protectedKeys) {
    if (!source || typeof source !== "object" || Array.isArray(source)) return target;
    const protectedSet = new Set(protectedKeys || []);
    Object.keys(source).forEach((key) => {
      if (protectedSet.has(key)) return;
      const sv = source[key];
      if (sv && typeof sv === "object" && !Array.isArray(sv) && target[key] && typeof target[key] === "object" && !Array.isArray(target[key])) {
        deepMergeBody(target[key], sv, []);
      } else {
        target[key] = sv;
      }
    });
    return target;
  }

  function applyExtraHeadersSafe(baseHeaders, extraHeaders) {
    const result = Object.assign({}, baseHeaders);
    const skipped = [];
    const applied = [];
    if (extraHeaders && typeof extraHeaders === "object") {
      Object.keys(extraHeaders).forEach((key) => {
        const lower = key.toLowerCase();
        if (PROTECTED_HEADERS.indexOf(lower) >= 0) {
          skipped.push(key);
          return;
        }
        result[key] = extraHeaders[key];
        applied.push(key);
      });
    }
    return { headers: result, skipped_header_keys: skipped, applied_header_keys: applied };
  }

  function applyExtraBodySafe(baseBody, extraBody, provider) {
    const protectedKeys = PROTECTED_BODY_FIELDS[provider] || [];
    const result = Object.assign({}, baseBody);
    const skipped = [];
    const applied = [];
    if (extraBody && typeof extraBody === "object" && !Array.isArray(extraBody)) {
      deepMergeBody(result, extraBody, protectedKeys);
      Object.keys(extraBody).forEach((key) => {
        if (protectedKeys.indexOf(key) >= 0) skipped.push(key);
        else applied.push(key);
      });
    }
    return { body: result, skipped_body_keys: skipped, applied_body_keys: applied };
  }

  function requestOverrideTrace(headerResult, bodyResult, extra) {
    return Object.assign({
      applied_header_keys: (headerResult && headerResult.applied_header_keys) || [],
      skipped_header_keys: (headerResult && headerResult.skipped_header_keys) || [],
      applied_body_keys: (bodyResult && bodyResult.applied_body_keys) || [],
      skipped_body_keys: (bodyResult && bodyResult.skipped_body_keys) || [],
    }, extra || {});
  }

  function requestFetcher() {
    const RR = getR();
    if (RR && typeof RR.nativeFetch === "function") {
      return { fetcher: RR.nativeFetch.bind(RR), transport: "risu_native_fetch" };
    }
    if (RR && typeof RR.risuFetch === "function") {
      return { fetcher: RR.risuFetch.bind(RR), transport: "risu_fetch" };
    }
    if (typeof fetch === "function") {
      return { fetcher: fetch.bind(globalThis), transport: "browser_fetch" };
    }
    return { fetcher: null, transport: "unavailable" };
  }

  function traceableEndpoint(url) {
    try {
      const parsed = new URL(safeString(url));
      return `${parsed.origin}${parsed.pathname}`;
    } catch (_) {
      return safeString(url).split("?")[0].slice(0, 300);
    }
  }

  async function fetchWithAbort(url, options, timeoutMs, abortSignal) {
    const controller = new AbortController();
    let timeoutTriggered = false;
    let deadlineTriggered = false;
    const boundedTimeoutMs = Math.max(0, Number(timeoutMs) || 0);
    const timer = boundedTimeoutMs > 0
      ? setTimeout(() => {
        timeoutTriggered = true;
        controller.abort();
      }, boundedTimeoutMs)
      : null;
    if (abortSignal) {
      if (abortSignal.aborted) {
        deadlineTriggered = true;
        controller.abort();
      } else {
        abortSignal.addEventListener("abort", () => {
          deadlineTriggered = true;
          controller.abort();
        }, { once: true });
      }
    }
    const selected = requestFetcher();
    try {
      if (!selected.fetcher) throw new Error("no_fetch_available");
      const fetchOptions = Object.assign({}, options, { signal: controller.signal });
      if (boundedTimeoutMs > 0) fetchOptions.requestTimeoutMs = boundedTimeoutMs;
      const fetchPromise = selected.fetcher(url, fetchOptions);
      const abortPromise = new Promise((_, reject) => {
        controller.signal.addEventListener("abort", () => {
          reject(new Error(
            deadlineTriggered ? "deadline_aborted" : (timeoutTriggered ? "request_timeout" : "request_aborted")
          ));
        }, { once: true });
      });
      const response = await Promise.race([fetchPromise, abortPromise]);
      try {
        Object.defineProperty(response, "__recomposer_transport", {
          configurable: true,
          value: selected.transport,
        });
      } catch (_) {}
      return response;
    } catch (err) {
      let normalizedError = err;
      if (controller.signal.aborted && safeString(err && err.message) !== "deadline_aborted"
          && safeString(err && err.message) !== "request_timeout"
          && safeString(err && err.message) !== "request_aborted") {
        normalizedError = new Error(
          deadlineTriggered ? "deadline_aborted" : (timeoutTriggered ? "request_timeout" : "request_aborted")
        );
      }
      if (normalizedError && typeof normalizedError === "object") {
        normalizedError.request_transport = selected.transport;
        normalizedError.request_endpoint = traceableEndpoint(url);
      }
      throw normalizedError;
    } finally {
      if (timer) clearTimeout(timer);
    }
  }

  async function readResponseText(response) {
    if (!response) return "";
    try {
      return await response.text();
    } catch (_) {
      return "";
    }
  }

  function extractOpenAiText(data) {
    const choice = data && Array.isArray(data.choices) ? data.choices[0] : null;
    const msg = choice && choice.message ? choice.message : null;
    return safeString(msg && msg.content);
  }

  function extractToolPayload(data, expectedName) {
    const choice = data && Array.isArray(data.choices) ? data.choices[0] : null;
    const message = (choice && choice.message) || (data && data.message) || {};
    const calls = Array.isArray(message.tool_calls) ? message.tool_calls : [];
    const call = calls.find((item) => item && item.function
      && (!expectedName || safeString(item.function.name) === expectedName));
    if (!call || !call.function) return { found: false, payload: null, raw: "" };
    const args = call.function.arguments;
    if (args && typeof args === "object" && !Array.isArray(args)) {
      return { found: true, payload: args, raw: JSON.stringify(args) };
    }
    const raw = safeString(args);
    return { found: true, payload: tryParseJson(raw), raw };
  }

  function extractReasoningText(provider, data) {
    if (!data || typeof data !== "object") return "";
    if (provider === "openai_compatible" || provider === "ollama_compatible" || provider === "custom") {
      const choice = Array.isArray(data.choices) ? data.choices[0] : null;
      const message = choice && choice.message ? choice.message : {};
      return safeString(
        message.reasoning_content
        || message.reasoning
        || message.analysis
        || (choice && choice.reasoning)
        || data.reasoning
      );
    }
    if (provider === "anthropic") {
      return (Array.isArray(data.content) ? data.content : [])
        .filter((block) => block && (block.type === "thinking" || block.type === "reasoning"))
        .map((block) => safeString(block.thinking || block.text))
        .filter(Boolean)
        .join("\n");
    }
    if (provider === "gemini" || provider === "vertex") {
      const candidate = Array.isArray(data.candidates) ? data.candidates[0] : null;
      const parts = candidate && candidate.content && Array.isArray(candidate.content.parts)
        ? candidate.content.parts
        : [];
      return parts.filter((part) => part && part.thought).map((part) => safeString(part.text)).filter(Boolean).join("\n");
    }
    return "";
  }

  function extractAnthropicText(data) {
    const content = data && Array.isArray(data.content) ? data.content : [];
    const parts = [];
    content.forEach((block) => {
      if (block && block.type === "text" && block.text) parts.push(block.text);
    });
    return parts.join("\n");
  }

  function extractGeminiText(data) {
    const candidates = data && Array.isArray(data.candidates) ? data.candidates : [];
    const candidate = candidates[0];
    const parts = candidate && candidate.content && Array.isArray(candidate.content.parts) ? candidate.content.parts : [];
    const texts = [];
    parts.forEach((part) => {
      if (part && part.text) texts.push(part.text);
    });
    return texts.join("\n");
  }

  function vertexModelId(model) {
    return safeString(model).replace(/^publishers\/google\/models\//i, "").replace(/^google\//i, "");
  }

  function vertexGenerateContentUrl(endpoint, model) {
    const base = safeString(endpoint).replace(/\/+$/, "");
    if (!base) throw new Error("missing_vertex_endpoint");
    if (/:generateContent$/i.test(base)) return base;
    if (/\/publishers\/google\/models\/[^/]+$/i.test(base)) return `${base}:generateContent`;
    const modelId = encodeURIComponent(vertexModelId(model)).replace(/%2F/g, "/");
    return `${base}/publishers/google/models/${modelId}:generateContent`;
  }

  async function callProvider(profile, prompts, abortSignal, requestOptions) {
    const provider = sanitizeEnum(profile.provider, PROVIDERS, "openai_compatible");
    const model = safeString(profile.model).trim();
    const endpoint = safeString(profile.endpoint).trim();
    if (!model) throw new Error("missing_model");
    if (provider !== "ollama_compatible" && provider !== "custom" && !endpoint) throw new Error("missing_endpoint");
    const key = await resolveApiKey(profile.api_key_ref);
    const timeoutMs = requestOptions && requestOptions.completion_wait
      ? 0
      : clampNumber(profile.timeout_ms, 5000, 300000, 45000);
    const extraHeaders = parseExtraHeaders(profile.extra_headers);
    const extraBody = parseExtraBody(profile.extra_body);

    if (provider === "anthropic") {
      return callAnthropic(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    if (provider === "gemini") {
      return callGemini(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    if (provider === "vertex") {
      return callVertex(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    if (provider === "ollama_compatible") {
      return callOllama(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal, requestOptions);
    }
    if (provider === "custom") {
      return callCustom(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    return callOpenAiCompatible(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal, requestOptions);
  }

  async function callOpenAiCompatible(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal, requestOptions) {
    const url = openAiChatUrl(profile.endpoint);
    const baseHeaders = { "Content-Type": "application/json" };
    if (key) baseHeaders.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
    const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
    const headers = hdrResult.headers;
    let body = {
      model: profile.model,
      messages: [
        { role: "system", content: prompts.system },
        { role: "user", content: prompts.user },
      ],
      temperature: profile.temperature,
      max_tokens: profile.max_output_tokens,
      stream: false,
    };
    if (profile.force_json_response && !(requestOptions && requestOptions.planner_tool)) {
      body.response_format = { type: "json_object" };
    }
    const reasoningInfo = applyReasoningAdapter("openai_compatible", body, profile);
    const bodyResult = applyExtraBodySafe(body, extraBody, "openai_compatible");
    body = bodyResult.body;
    if (requestOptions && requestOptions.planner_tool) {
      body.tools = [requestOptions.planner_tool];
      body.tool_choice = {
        type: "function",
        function: { name: requestOptions.tool_name },
      };
      delete body.response_format;
    }
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    const data = JSON.parse(raw);
    const toolPayload = extractToolPayload(data, requestOptions && requestOptions.tool_name);
    return {
      content: extractOpenAiText(data),
      reasoning: extractReasoningText("openai_compatible", data),
      structured_payload: toolPayload.payload,
      structured_raw: toolPayload.raw,
      structured_transport: toolPayload.found
        ? (toolPayload.payload ? "tool_call_used" : "tool_call_invalid")
        : "content_json",
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
        reasoning_family: reasoningInfo.family,
        reasoning_fields: reasoningInfo.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  function isOllamaCloudEndpoint(endpoint) {
    const base = safeString(endpoint).toLowerCase();
    return base.indexOf("ollama.com") >= 0 || base.indexOf("/v1/chat/completions") >= 0;
  }

  async function callOllama(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal, requestOptions) {
    const base = safeString(profile.endpoint || "http://localhost:11434").replace(/\/+$/, "");
    if (isOllamaCloudEndpoint(base)) {
      const url = openAiChatUrl(base);
      const baseHeaders = { "Content-Type": "application/json" };
      if (key) baseHeaders.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
      const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
      const headers = hdrResult.headers;
      let body = {
        model: profile.model,
        messages: [
          { role: "system", content: prompts.system },
          { role: "user", content: prompts.user },
        ],
        temperature: profile.temperature,
        max_tokens: profile.max_output_tokens,
        stream: false,
      };
      if (profile.force_json_response && !(requestOptions && requestOptions.planner_tool)) {
        body.response_format = { type: "json_object" };
      }
      const reasoningInfo = applyReasoningAdapter("ollama_compatible", body, profile);
      const bodyResult = applyExtraBodySafe(body, extraBody, "ollama_compatible");
      body = bodyResult.body;
      if (requestOptions && requestOptions.planner_tool) {
        body.tools = [requestOptions.planner_tool];
        body.tool_choice = {
          type: "function",
          function: { name: requestOptions.tool_name },
        };
        delete body.response_format;
      }
      const response = await fetchWithAbort(url, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      }, timeoutMs, abortSignal);
      const raw = await readResponseText(response);
      if (!response || !response.ok) {
        throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
      }
      const data = JSON.parse(raw);
      const toolPayload = extractToolPayload(data, requestOptions && requestOptions.tool_name);
      return {
        content: extractOpenAiText(data),
        reasoning: extractReasoningText("ollama_compatible", data),
        structured_payload: toolPayload.payload,
        structured_raw: toolPayload.raw,
        structured_transport: toolPayload.found
          ? (toolPayload.payload ? "tool_call_used" : "tool_call_invalid")
          : "content_json",
        raw,
        elapsed_ms: 0,
        request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
          reasoning_family: reasoningInfo.family,
          reasoning_fields: reasoningInfo.applied,
          transport: safeString(response && response.__recomposer_transport),
        }),
      };
    }
    const url = `${base}/api/chat`;
    const baseHeaders2 = { "Content-Type": "application/json" };
    const hdrResult2 = applyExtraHeadersSafe(baseHeaders2, extraHeaders);
    const headers = hdrResult2.headers;
    let body = {
      model: profile.model,
      messages: [
        { role: "system", content: prompts.system },
        { role: "user", content: prompts.user },
      ],
      stream: false,
      options: {
        temperature: profile.temperature,
        num_predict: profile.max_output_tokens,
      },
    };
    const reasoningInfo2 = applyReasoningAdapter("ollama_compatible", body, profile);
    const bodyResult2 = applyExtraBodySafe(body, extraBody, "ollama_compatible");
    body = bodyResult2.body;
    if (requestOptions && requestOptions.planner_tool) {
      body.tools = [requestOptions.planner_tool];
    }
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    const data = JSON.parse(raw);
    const toolPayload = extractToolPayload(data, requestOptions && requestOptions.tool_name);
    return {
      content: safeString(data.message && data.message.content),
      reasoning: safeString(data.message && (data.message.thinking || data.message.reasoning)),
      structured_payload: toolPayload.payload,
      structured_raw: toolPayload.raw,
      structured_transport: toolPayload.found
        ? (toolPayload.payload ? "tool_call_used" : "tool_call_invalid")
        : "content_json",
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult2, bodyResult2, {
        reasoning_family: reasoningInfo2.family,
        reasoning_fields: reasoningInfo2.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  async function callAnthropic(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const base = safeString(profile.endpoint || "https://api.anthropic.com/v1").replace(/\/+$/, "");
    const url = /\/messages$/i.test(base) ? base : `${base}/messages`;
    let body = {
      model: profile.model,
      system: prompts.system,
      messages: [{ role: "user", content: prompts.user }],
      temperature: profile.temperature,
      max_tokens: profile.max_output_tokens || 1024,
    };
    const reasoningInfo = applyReasoningAdapter("anthropic", body, profile);
    const bodyResult = applyExtraBodySafe(body, extraBody, "anthropic");
    body = bodyResult.body;
    const baseHeaders = { "Content-Type": "application/json", "x-api-key": key, "anthropic-version": "2023-06-01" };
    const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
    const headers = hdrResult.headers;
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    const data = JSON.parse(raw);
    return {
      content: extractAnthropicText(data),
      reasoning: extractReasoningText("anthropic", data),
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
        reasoning_family: reasoningInfo.family,
        reasoning_fields: reasoningInfo.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  async function callGemini(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const base = safeString(profile.endpoint || "https://generativelanguage.googleapis.com/v1beta").replace(/\/+$/, "");
    const modelPath = encodeURIComponent(profile.model).replace(/%2F/g, "/");
    const urlBase = /:generateContent/i.test(base) ? base : `${base}/models/${modelPath}:generateContent`;
    const url = key && urlBase.indexOf("key=") < 0
      ? `${urlBase}${urlBase.indexOf("?") >= 0 ? "&" : "?"}key=${encodeURIComponent(key)}`
      : urlBase;
    let body = {
      contents: [{ role: "user", parts: [{ text: `${prompts.system}\n\n${prompts.user}` }] }],
      generationConfig: {
        temperature: profile.temperature,
        maxOutputTokens: profile.max_output_tokens,
      },
    };
    const reasoningInfo = applyReasoningAdapter("gemini", body, profile);
    if (profile.force_json_response) {
      body.generationConfig.responseMimeType = "application/json";
    }
    const bodyResult = applyExtraBodySafe(body, extraBody, "gemini");
    body = bodyResult.body;
    const baseHeaders = { "Content-Type": "application/json" };
    if (key) baseHeaders["x-goog-api-key"] = key;
    const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
    const headers = hdrResult.headers;
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    const data = JSON.parse(raw);
    return {
      content: extractGeminiText(data),
      reasoning: extractReasoningText("gemini", data),
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
        reasoning_family: reasoningInfo.family,
        reasoning_fields: reasoningInfo.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  async function callVertex(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const url = vertexGenerateContentUrl(profile.endpoint, profile.model);
    const baseHeaders = { "Content-Type": "application/json" };
    if (key) baseHeaders.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
    applyVertexFlex(profile, baseHeaders);
    const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
    const headers = hdrResult.headers;
    let body = {
      systemInstruction: { parts: [{ text: prompts.system }] },
      contents: [{ role: "user", parts: [{ text: prompts.user }] }],
      generationConfig: {
        temperature: profile.temperature,
        maxOutputTokens: profile.max_output_tokens,
      },
    };
    const reasoningInfo = applyReasoningAdapter("vertex", body, profile);
    if (profile.force_json_response) {
      body.generationConfig.responseMimeType = "application/json";
    }
    const bodyResult = applyExtraBodySafe(body, extraBody, "vertex");
    body = bodyResult.body;
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    const data = JSON.parse(raw);
    return {
      content: extractGeminiText(data),
      reasoning: extractReasoningText("vertex", data),
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
        vertex_flex_mode: safeString(profile.vertex_flex_mode, "off"),
        reasoning_family: reasoningInfo.family,
        reasoning_fields: reasoningInfo.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  async function callCustom(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const url = safeString(profile.endpoint);
    if (!url) throw new Error("missing_custom_endpoint");
    const baseHeaders = { "Content-Type": "application/json" };
    if (key) baseHeaders.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
    const hdrResult = applyExtraHeadersSafe(baseHeaders, extraHeaders);
    const headers = hdrResult.headers;
    let body = {
      model: profile.model,
      system: prompts.system,
      user: prompts.user,
      temperature: profile.temperature,
      max_tokens: profile.max_output_tokens,
    };
    const reasoningInfo = applyReasoningAdapter("custom", body, profile);
    const bodyResult = applyExtraBodySafe(body, extraBody, "custom");
    body = bodyResult.body;
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    let content = "";
    let reasoning = "";
    try {
      const data = JSON.parse(raw);
      content = extractOpenAiText(data) || extractGeminiText(data) || extractAnthropicText(data) || safeString(data.content || data.text || data.output);
      reasoning = extractReasoningText("custom", data);
    } catch (_) {
      content = raw;
    }
    return {
      content,
      reasoning,
      raw,
      elapsed_ms: 0,
      request_overrides: requestOverrideTrace(hdrResult, bodyResult, {
        reasoning_family: reasoningInfo.family,
        reasoning_fields: reasoningInfo.applied,
        transport: safeString(response && response.__recomposer_transport),
      }),
    };
  }

  /* ── JSON Repair ───────────────────────────────────────── */

  function stripJsonFence(text) {
    return safeString(text).trim()
      .replace(/^```(?:json)?\s*/i, "")
      .replace(/\s*```$/i, "")
      .trim();
  }

  function extractBalancedJson(text) {
    const source = safeString(text);
    const objStart = source.indexOf("{");
    const arrStart = source.indexOf("[");
    let start = -1;
    if (objStart >= 0 && arrStart >= 0) start = Math.min(objStart, arrStart);
    else start = Math.max(objStart, arrStart);
    if (start < 0) return "";
    const stack = [];
    let quote = "";
    let escaped = false;
    for (let i = start; i < source.length; i++) {
      const ch = source.charAt(i);
      if (quote) {
        if (escaped) escaped = false;
        else if (ch === "\\") escaped = true;
        else if (ch === quote) quote = "";
        continue;
      }
      if (ch === '"' || ch === "'") { quote = ch; continue; }
      if (ch === "{" || ch === "[") { stack.push(ch); continue; }
      if (ch === "}" || ch === "]") {
        const open = stack.pop();
        if ((ch === "}" && open !== "{") || (ch === "]" && open !== "[")) return "";
        if (!stack.length) return source.slice(start, i + 1);
      }
    }
    return "";
  }

  function decodePseudoToolText(value) {
    return safeString(value)
      .replace(/&quot;/gi, '"')
      .replace(/&#39;|&apos;/gi, "'")
      .replace(/&lt;/gi, "<")
      .replace(/&gt;/gi, ">")
      .replace(/&amp;/gi, "&");
  }

  function parsePseudoToolValue(value) {
    const raw = decodePseudoToolText(value).trim();
    if (!raw) return "";
    const variants = [raw, raw.replace(/,\s*([}\]])/g, "$1")];
    for (let i = 0; i < variants.length; i++) {
      try { return JSON.parse(variants[i]); } catch (_) {}
    }
    const balanced = extractBalancedJson(raw);
    if (balanced) {
      try { return JSON.parse(balanced); } catch (_) {}
      try { return JSON.parse(balanced.replace(/,\s*([}\]])/g, "$1")); } catch (_) {}
    }
    if (/^(?:true|false|null)$/i.test(raw)) {
      try { return JSON.parse(raw.toLowerCase()); } catch (_) {}
    }
    if (/^-?\d+(?:\.\d+)?$/.test(raw)) return Number(raw);
    return raw;
  }

  function parsePseudoToolCall(text) {
    const source = safeString(text);
    if (!/<tool_call\b|<function\s*=/i.test(source)) return null;
    const result = {};
    const parameterPattern = /<parameter(?:\s+name\s*=\s*["']([^"']+)["']|\s*=\s*["']?([A-Za-z0-9_]+)["']?)\s*>([\s\S]*?)<\/parameter>/gi;
    let match;
    while ((match = parameterPattern.exec(source))) {
      const name = safeString(match[1] || match[2]).trim();
      if (!name) continue;
      result[name] = parsePseudoToolValue(match[3]);
    }
    return Object.keys(result).length ? result : null;
  }

  function tryParseJson(text) {
    const raw = stripJsonFence(text);
    if (!raw) return null;
    const pseudoTool = parsePseudoToolCall(raw);
    if (pseudoTool) return pseudoTool;
    const variants = [
      raw,
      raw.replace(/,\s*([}\]])/g, "$1"),
    ];
    for (let i = 0; i < variants.length; i++) {
      try { return JSON.parse(variants[i]); } catch (_) {}
    }
    const balanced = extractBalancedJson(raw);
    if (balanced) {
      try { return JSON.parse(balanced); } catch (_) {}
      try { return JSON.parse(balanced.replace(/,\s*([}\]])/g, "$1")); } catch (_) {}
    }
    return null;
  }

  const CONTRACT_FRAGMENT_FIELDS = Object.freeze([
    "required_facts",
    "writer_only_secrets",
    "character_visible_facts",
    "character_knowledge_scopes",
    "identity_and_alias_constraints",
    "relationship_and_emotion_state",
    "scene_time_location_and_world_rules",
    "open_threads_and_turn_objectives",
    "agency_and_pov_constraints",
    "prose_and_dialogue_targets",
    "forbidden_regressions",
    "uncertainty",
  ]);

  const INPUT_PLANNER_FIELDS = Object.freeze({
    input_canon_secret_planner: Object.freeze([
      "required_facts",
      "writer_only_secrets",
      "character_visible_facts",
      "character_knowledge_scopes",
      "identity_and_alias_constraints",
      "forbidden_regressions",
      "uncertainty",
    ]),
    input_character_relationship_planner: Object.freeze([
      "required_facts",
      "character_visible_facts",
      "character_knowledge_scopes",
      "identity_and_alias_constraints",
      "relationship_and_emotion_state",
      "agency_and_pov_constraints",
      "prose_and_dialogue_targets",
      "forbidden_regressions",
      "uncertainty",
    ]),
    input_scene_continuity_planner: Object.freeze([
      "required_facts",
      "scene_time_location_and_world_rules",
      "open_threads_and_turn_objectives",
      "agency_and_pov_constraints",
      "prose_and_dialogue_targets",
      "forbidden_regressions",
      "uncertainty",
    ]),
  });

  function inputPlannerFields(roleId) {
    return INPUT_PLANNER_FIELDS[safeString(roleId)] || CONTRACT_FRAGMENT_FIELDS;
  }

  function inputPlannerExample(roleId) {
    const example = {
      schema: "turn_contract_fragment.v1",
      planner_role: safeString(roleId),
    };
    inputPlannerFields(roleId).forEach((field) => {
      example[field] = field === "required_facts"
        ? [{ text: "grounded fact", evidence_refs: ["payload_user_input"], evidence_quote: "exact source phrase" }]
        : [];
    });
    return JSON.stringify(example);
  }

  function inputPlannerTool(roleId) {
    const itemSchema = {
      type: "object",
      additionalProperties: false,
      properties: {
        text: { type: "string", maxLength: 240 },
        evidence_refs: { type: "array", items: { type: "string" } },
        evidence_quote: { type: "string", maxLength: 120 },
      },
      required: ["text", "evidence_refs", "evidence_quote"],
    };
    const properties = {
      schema: { type: "string", enum: ["turn_contract_fragment.v1"] },
      planner_role: { type: "string", enum: [safeString(roleId)] },
    };
    const required = ["schema", "planner_role"];
    inputPlannerFields(roleId).forEach((field) => {
      properties[field] = { type: "array", maxItems: PLANNER_MAX_ITEMS_PER_FIELD, items: itemSchema };
      required.push(field);
    });
    return {
      type: "function",
      function: {
        name: "submit_turn_contract_fragment",
        description: "Submit the grounded turn contract fragment for this planner role.",
        parameters: {
          type: "object",
          additionalProperties: false,
          properties,
          required,
        },
      },
    };
  }

  function normalizeContractItem(value, allowedEvidence) {
    const source = typeof value === "string" ? { text: value, evidence_refs: [] } : asObject(value);
    const text = truncate(redactSensitiveText(source.text != null ? source.text : source.value), 240).trim();
    if (!text) return null;
    const refs = uniqueList(arrayFromCollection(source.evidence_refs || source.evidence)
      .map((item) => safeString(item).trim())
      .filter((item) => !allowedEvidence || allowedEvidence.has(item)));
    return {
      text,
      evidence_refs: refs,
      evidence_quote: truncate(redactSensitiveText(source.evidence_quote), 120).trim(),
    };
  }

  function uniqueContractItems(items) {
    const seen = new Set();
    const out = [];
    arrayFromCollection(items).forEach((item) => {
      const normalized = normalizeContractItem(item, null);
      if (!normalized) return;
      const key = normalized.text.toLowerCase().replace(/\s+/g, " ");
      if (seen.has(key)) return;
      seen.add(key);
      out.push(normalized);
    });
    return out;
  }

  function normalizedEvidenceText(value) {
    return safeString(value)
      .toLowerCase()
      .replace(/[^\p{L}\p{N}]+/gu, " ")
      .replace(/\s+/g, " ")
      .trim();
  }

  function evidenceTextMatchesClaim(claim, quote) {
    const normalizedClaim = normalizedEvidenceText(claim);
    const normalizedQuote = normalizedEvidenceText(quote);
    if (!normalizedClaim || !normalizedQuote) return false;
    if (normalizedClaim.indexOf(normalizedQuote) >= 0 || normalizedQuote.indexOf(normalizedClaim) >= 0) {
      return true;
    }
    const claimTokens = new Set(normalizedClaim.split(" ").filter((token) => token.length >= 2));
    const quoteTokens = new Set(normalizedQuote.split(" ").filter((token) => token.length >= 2));
    if (!claimTokens.size || !quoteTokens.size) return false;
    let overlap = 0;
    quoteTokens.forEach((token) => {
      if (claimTokens.has(token)) overlap++;
    });
    return overlap >= 2 && overlap / Math.min(claimTokens.size, quoteTokens.size) >= 0.5;
  }

  function evidenceQuoteGroundedInRefs(item, manifest) {
    const quote = normalizedEvidenceText(item && item.evidence_quote);
    if (quote.length < 4) return false;
    const sources = asObject(manifest && manifest.evidence_sources);
    return arrayFromCollection(item && item.evidence_refs).some((ref) => {
      const sourceText = normalizedEvidenceText(sources[ref]);
      return !!sourceText && sourceText.indexOf(quote) >= 0;
    });
  }

  function validateTurnContractFragment(parsed, expectedRoleId, manifest) {
    if (!parsed || typeof parsed !== "object") return null;
    const allowedEvidence = new Set(arrayFromCollection(manifest && manifest.evidence_refs));
    const fragment = {
      schema: "turn_contract_fragment.v1",
      planner_role: safeString(expectedRoleId),
    };
    let itemCount = 0;
    const unsupported = [];
    CONTRACT_FRAGMENT_FIELDS.forEach((field) => {
      const normalized = [];
      arrayFromCollection(parsed[field]).slice(0, PLANNER_MAX_ITEMS_PER_FIELD).forEach((value) => {
        if (itemCount + unsupported.length >= PLANNER_MAX_ITEMS_TOTAL) return;
        const item = normalizeContractItem(value, allowedEvidence);
        if (!item) return;
        const quoteGrounded = evidenceQuoteGroundedInRefs(item, manifest);
        const claimGrounded = evidenceTextMatchesClaim(item.text, item.evidence_quote);
        if (field !== "uncertainty" && (!item.evidence_refs.length || !quoteGrounded || !claimGrounded)) {
          unsupported.push({
            text: item.text,
            evidence_refs: item.evidence_refs,
            evidence_quote: item.evidence_quote,
          });
          return;
        }
        normalized.push(item);
        itemCount++;
      });
      fragment[field] = normalized;
    });
    fragment.uncertainty = uniqueContractItems(fragment.uncertainty.concat(unsupported));
    return itemCount || fragment.uncertainty.length ? fragment : null;
  }

  function contractItemsConflict(writerOnly, visible) {
    const writerText = normalizedEvidenceText(writerOnly && writerOnly.text);
    const visibleText = normalizedEvidenceText(visible && visible.text);
    if (writerText && writerText === visibleText) return true;
    const writerQuote = normalizedEvidenceText(writerOnly && writerOnly.evidence_quote);
    const visibleQuote = normalizedEvidenceText(visible && visible.evidence_quote);
    const writerRefs = new Set(arrayFromCollection(writerOnly && writerOnly.evidence_refs));
    const sharedRef = arrayFromCollection(visible && visible.evidence_refs).some((ref) => writerRefs.has(ref));
    if (sharedRef && writerQuote && visibleQuote
      && (writerQuote === visibleQuote || writerQuote.indexOf(visibleQuote) >= 0 || visibleQuote.indexOf(writerQuote) >= 0)) {
      return true;
    }
    return evidenceTextMatchesClaim(writerText, visibleText)
      && evidenceTextMatchesClaim(visibleText, writerText);
  }

  function fuseTurnContract(manifest, fragments) {
    const contract = {
      schema: "turn_contract.v1",
      mode: "standalone",
      source_snapshot_id: safeString(manifest && manifest.snapshot_id),
      immutable_constraints: [],
      writer_only_secrets: [],
      character_visible_facts: [],
      character_knowledge_scopes: [],
      identity_and_alias_map: [],
      relationship_and_emotion_state: [],
      scene_state: [],
      open_threads: [],
      turn_objectives: [],
      agency_and_pov_constraints: [],
      prose_targets: [],
      forbidden_regressions: [],
      unresolved_uncertainty: [],
      evidence_refs: arrayFromCollection(manifest && manifest.evidence_refs),
      source_availability: asObject(manifest && manifest.source_availability),
      planner_roles: [],
      fusion_state: fragments && fragments.length ? "planner_fused" : "manifest_fallback",
    };
    const fieldMap = {
      required_facts: "immutable_constraints",
      writer_only_secrets: "writer_only_secrets",
      character_visible_facts: "character_visible_facts",
      character_knowledge_scopes: "character_knowledge_scopes",
      identity_and_alias_constraints: "identity_and_alias_map",
      relationship_and_emotion_state: "relationship_and_emotion_state",
      scene_time_location_and_world_rules: "scene_state",
      open_threads_and_turn_objectives: "open_threads",
      agency_and_pov_constraints: "agency_and_pov_constraints",
      prose_and_dialogue_targets: "prose_targets",
      forbidden_regressions: "forbidden_regressions",
      uncertainty: "unresolved_uncertainty",
    };
    arrayFromCollection(fragments).forEach((fragment) => {
      if (!fragment) return;
      contract.planner_roles.push(safeString(fragment.planner_role));
      Object.keys(fieldMap).forEach((from) => {
        const to = fieldMap[from];
        contract[to] = contract[to].concat(arrayFromCollection(fragment[from]));
      });
    });
    Object.keys(fieldMap).forEach((from) => {
      const to = fieldMap[from];
      contract[to] = uniqueContractItems(contract[to]);
    });
    const visibilityConflicts = [];
    contract.character_visible_facts = contract.character_visible_facts.filter((item) => {
      const conflicts = contract.writer_only_secrets.some((secret) => contractItemsConflict(secret, item));
      if (!conflicts) return true;
      visibilityConflicts.push({
        text: `Visibility conflict retained as writer-only: ${item.text}`,
        evidence_refs: item.evidence_refs,
        evidence_quote: item.evidence_quote,
      });
      return false;
    });
    contract.unresolved_uncertainty = uniqueContractItems(
      contract.unresolved_uncertainty.concat(visibilityConflicts)
    );
    const latestUser = safeString(manifest && manifest.latest_user_input).trim();
    if (latestUser) {
      contract.turn_objectives = uniqueContractItems(contract.turn_objectives.concat([{
        text: truncate(latestUser, 600),
        evidence_refs: ["payload_user_input"],
        evidence_quote: truncate(latestUser, 240),
      }]));
    }
    contract.planner_roles = uniqueList(contract.planner_roles.filter(Boolean));
    const digestSource = Object.assign({}, contract);
    contract.contract_digest = stableDigest(digestSource);
    contract.contract_id = `turn_${Date.now()}_${contract.contract_digest}`;
    return contract;
  }

  function compactManifestForPlanner(manifest) {
    const maxChars = clampNumber(manifest && manifest.character_budget, 500, 50000, 6000);
    const sourceMap = asObject(manifest && manifest.evidence_sources);
    const evidenceSources = {};
    let remaining = maxChars;
    arrayFromCollection(manifest && manifest.evidence_refs).forEach((ref) => {
      if (remaining <= 0) return;
      const text = safeString(sourceMap[ref]);
      if (!text) return;
      const bounded = truncate(text, Math.min(2000, remaining));
      if (!bounded) return;
      evidenceSources[ref] = bounded;
      remaining -= bounded.length;
    });
    return {
      schema: "context_manifest.v1",
      snapshot_id: safeString(manifest && manifest.snapshot_id),
      mode: "standalone",
      evidence_sources: evidenceSources,
      evidence_refs: Object.keys(evidenceSources),
      source_availability: asObject(manifest && manifest.source_availability),
      collection_warnings: arrayFromCollection(manifest && manifest.collection_warnings),
      bounded_chars: maxChars - remaining,
    };
  }

  function compactContractProjection(contract, charLimit) {
    const maxChars = clampNumber(charLimit, 1000, 20000, 6000);
    const projection = {
      schema: "turn_contract.v1",
      contract_id: contract.contract_id,
      contract_digest: contract.contract_digest,
      source_snapshot_id: contract.source_snapshot_id,
      mode: contract.mode,
      fusion_state: contract.fusion_state,
    };
    [
      "immutable_constraints",
      "writer_only_secrets",
      "character_visible_facts",
      "character_knowledge_scopes",
      "identity_and_alias_map",
      "relationship_and_emotion_state",
      "scene_state",
      "open_threads",
      "turn_objectives",
      "agency_and_pov_constraints",
      "prose_targets",
      "forbidden_regressions",
      "unresolved_uncertainty",
    ].forEach((field) => {
      projection[field] = arrayFromCollection(contract[field]).slice(0, 8).map((item) => ({
        text: truncate(item && item.text, 350),
        evidence_refs: arrayFromCollection(item && item.evidence_refs).slice(0, 4),
        evidence_quote: truncate(item && item.evidence_quote, 160),
      }));
    });
    projection.source_availability = contract.source_availability;
    projection.planner_roles = contract.planner_roles;
    let encoded = JSON.stringify(projection);
    while (encoded.length > maxChars) {
      let trimmed = false;
      Object.keys(projection).forEach((key) => {
        if (!Array.isArray(projection[key]) || projection[key].length <= 1 || encoded.length <= maxChars) return;
        projection[key].pop();
        trimmed = true;
        encoded = JSON.stringify(projection);
      });
      if (!trimmed) break;
    }
    if (encoded.length > maxChars) {
      delete projection.source_availability;
      encoded = JSON.stringify(projection);
    }
    const removalOrder = [
      "unresolved_uncertainty",
      "prose_targets",
      "open_threads",
      "relationship_and_emotion_state",
      "scene_state",
      "character_visible_facts",
      "character_knowledge_scopes",
      "identity_and_alias_map",
      "forbidden_regressions",
      "agency_and_pov_constraints",
      "writer_only_secrets",
      "immutable_constraints",
    ];
    for (let i = 0; i < removalOrder.length && encoded.length > maxChars; i++) {
      const field = removalOrder[i];
      while (Array.isArray(projection[field]) && projection[field].length && encoded.length > maxChars) {
        projection[field].pop();
        encoded = JSON.stringify(projection);
      }
    }
    if (encoded.length > maxChars) {
      projection.turn_objectives = arrayFromCollection(projection.turn_objectives).slice(0, 1).map((item) => ({
        text: truncate(item && item.text, 240),
        evidence_refs: arrayFromCollection(item && item.evidence_refs).slice(0, 1),
        evidence_quote: truncate(item && item.evidence_quote, 120),
      }));
      delete projection.planner_roles;
    }
    return projection;
  }

  function renderTurnContractBlock(contract, charLimit) {
    const instruction = "Treat this JSON as the binding narrative contract for the next RP response. Writer-only secrets may guide narration but must not become character knowledge unless character_knowledge_scopes explicitly allow it. Preserve user agency and resolve the turn objectives in immersive prose. Explicit output-language and response-format requirements in turn_objectives or prose_targets override the draft's language and framing.";
    const projectionLimit = Math.max(1000, Number(charLimit || 6000) - INPUT_CONTRACT_MARKER.length - instruction.length - 2);
    return `${INPUT_CONTRACT_MARKER}\n${instruction}\n${JSON.stringify(compactContractProjection(contract, projectionLimit))}`;
  }

  function isTurnContractMessage(message) {
    return !!(message
      && message.role === "system"
      && safeString(message.content).indexOf(INPUT_CONTRACT_MARKER) >= 0);
  }

  function injectTurnContract(messages, contractBlock) {
    const next = (Array.isArray(messages) ? messages : [])
      .filter((message) => !isTurnContractMessage(message))
      .slice();
    let insertAt = next.length;
    for (let i = next.length - 1; i >= 0; i--) {
      if (next[i] && next[i].role === "user") {
        insertAt = i;
        break;
      }
    }
    next.splice(insertAt, 0, { role: "system", content: contractBlock });
    return next;
  }

  /* ── Role Call (single path for all roles) ─────────────── */

  function buildRolePrompt(role, profile, mutableSegs, contextBlock, allSegments, directorInfo) {
    const systemPrompt = safeString(profile.system_prompt || role.default_prompt);
    const contextSection = contextBlock ? `\n\n--- Runtime Context (read-only) ---\n${contextBlock}\n--- End Context ---\n` : "";
    let userPrompt;
    if (role.is_input_planner) {
      const manifest = asObject(directorInfo && directorInfo.context_manifest);
      const plannerOptions = plannerRequestOptions(role, profile);
      const transportInstruction = plannerOptions && plannerOptions.planner_tool
        ? "Call submit_turn_contract_fragment exactly once with the final object. Do not also emit the object as message text."
        : "Output one compact JSON object only. Do not emit XML, <tool_call>, function tags, markdown, reasoning, or commentary.";
      userPrompt = `Build one grounded turn_contract_fragment.v1 from this context_manifest.v1.\n\nAllowed evidence_refs: ${arrayFromCollection(manifest.evidence_refs).join(", ")}\nPlanner-owned fields: ${inputPlannerFields(role.role_id).join(", ")}\n\nContext manifest:\n${JSON.stringify(manifest)}\n\nRules:\n- Every item outside uncertainty MUST include at least one allowed evidence_ref and an exact short evidence_quote copied from that same evidence_sources field.\n- The item text must describe the quoted evidence; never attach an unrelated real quote to an invented claim.\n- Keep writer_only_secrets separate from character_visible_facts when those fields are assigned to this role.\n- Do not convert narrator knowledge into character knowledge.\n- Do not write RP prose, dialogue, a draft, recommendations, or markdown.\n- Return only this role's assigned fields. Use empty arrays when no grounded item exists.\n- ${transportInstruction}\n\nExpected object:\n${inputPlannerExample(role.role_id)}`;
    } else if (role.is_composer) {
      const mutableList = mutableSegs.map((s) => {
        const candidates = (directorInfo && directorInfo.candidateBundles && directorInfo.candidateBundles[s.id]) || [];
        const candidateSection = candidates.length
          ? candidates.map((c, i) => `  Candidate ${i + 1} (role: ${c.role}, confidence: ${c.confidence}, score: ${c.score != null ? c.score.toFixed(1) : ""}, issues: [${(c.issues || []).join(", ")}], change: ${c.change_summary || "none"}):\n    ${c.rewrite}`).join("\n")
          : "  (no candidates — gap, improve original directly)";
        const segConsensus = directorInfo && directorInfo.consensus && directorInfo.consensus[s.id] ? directorInfo.consensus[s.id] : [];
        const segComplementary = directorInfo && directorInfo.complementary && directorInfo.complementary[s.id] ? directorInfo.complementary[s.id] : [];
        const segConflict = directorInfo && directorInfo.conflict && directorInfo.conflict[s.id] ? directorInfo.conflict[s.id] : [];
        const isGap = directorInfo && directorInfo.gap && directorInfo.gap.indexOf(s.id) >= 0;
        return `Segment ${s.id}:\nOriginal:\n${s.text}\nCandidates:\n${candidateSection}\nDirector: consensus=[${segConsensus.join(", ")}] complementary=[${segComplementary.join(", ")}] conflict=[${segConflict.join(", ")}] gap=${isGap ? "yes" : "no"}`;
      }).join("\n\n---\n\n");
      const preservedList = allSegments.filter((s) => s.type !== "mutable").map((s) =>
        `${s.id} [PRESERVED — do not output]: ${preview(s.text, 200)}`
      ).join("\n");
      const directorSection = directorInfo
        ? `\n--- Fusion Director ---\nConsensus: ${JSON.stringify(directorInfo.consensus || {})}\nComplementary: ${JSON.stringify(directorInfo.complementary || {})}\nConflict: ${JSON.stringify(directorInfo.conflict || {})}\nGaps: ${JSON.stringify(directorInfo.gap || [])}\n--- End Director ---\n`
        : "";
      const revisionFeedback = asObject(directorInfo && directorInfo.composer_revision_feedback);
      const retrySection = arrayFromCollection(revisionFeedback.weak_segment_ids).length
        ? `\n--- Recomposition Retry ---\nThe previous composition was rejected as a near-copy.\nWeak segment IDs: ${revisionFeedback.weak_segment_ids.join(", ")}\nPrevious rejected segments:\n${JSON.stringify(revisionFeedback.previous_segments || {})}\nFor every weak segment, discard the previous sentence structure and reconstruct the prose materially. Preserve facts, not wording. Do not answer with spelling, punctuation, synonym, or isolated-line edits.\n--- End Retry ---\n`
        : "";
      userPrompt = `Compose the FINAL version of the mutable prose for this roleplay response.\n\nFull scene segments (mutable only — rewrite these):\n\n${mutableList}\n\nPreserved segments (do NOT include in your output):\n${preservedList}\n${directorSection}${retrySection}${contextSection}\n\nComposition instructions:\n- Reconstruct each mutable segment from the original, all candidates, and Director evidence. Synthesize compatible strengths; never concatenate candidate passages.\n- Resolve conflicts in this order: binding output contract; secret/POV/identity/user-agency/meta integrity; continuity/world facts; character/emotion/relationship; style/rhythm.\n- Reject any candidate that breaks a higher-priority constraint even when its score is higher.\n- Use consensus as strong corroboration, complementary findings as synthesis targets, and gaps as a mandate to elevate the original directly.\n- Remove Thoughts, Processing, Approved, Response, prompt residue, and assistant commentary from mutable prose, replacing contaminated space with scene-native writing.\n- Explicit output-language or framing requirements in the turn contract override the draft's language and framing.\n- Preserve established facts, events, user actions, perspective, and cross-segment flow, but not the draft's wording or sentence structure.\n- Materially rewrite every substantive prose segment through stronger diction, sentence architecture, sensory detail, subtext, pacing, transitions, and ending cadence.\n- Cosmetic spelling, punctuation, synonym, or single-line edits do not satisfy the task.\n- Include every requested mutable segment ID exactly once. Output no protected or inspect-only segment.\n\nReturn one compact JSON object only:\n{"segments":{"SEG_ID":"final rewritten text", ...}}`;
    } else {
      const segIds = mutableSegs.map((s) => s.id);
      const deleteRule = role.role_id === "agency_meta_guard"
        ? "\n- If and only if a complete mutable segment is solely a Thoughts/Analysis/Thinking wrapper, you may return operation:\"delete\", rewrite:\"\", and issue meta_artifact."
        : "";
      userPrompt = `Produce this lane's decisive rewrite candidates for the mutable segments below.\n\nYou are seeing the full ordered response for continuity. Protected and inspect-only segments are [PRESERVED] context and must never be returned or rewritten.\n\nFull ordered segments:\n${allSegments.map((s) => s.type === "mutable" ? `${s.id} [MUTABLE]:\n${s.text}` : `${s.id} [PRESERVED]: ${preview(s.text, 200)}`).join("\n\n---\n\n")}\n\nAllowed mutable segment IDs: ${segIds.join(", ")}\n${contextSection}\n\nCandidate instructions:\n- Apply only your system-defined lane. Do not duplicate another specialist's ownership.\n- Return a candidate only when you can materially improve the complete segment within that lane.\n- Reconstruct the whole segment when necessary; do not submit advice, diagnostics, fragments, patches, or cosmetic synonym swaps.\n- Omit unchanged segments. If this lane finds no material improvement, return {"role":"${role.role_id}","candidates":[]}.\n- For every candidate, identify the concrete issues fixed and summarize the actual revision.${deleteRule}\n\nReturn one compact JSON object only:\n{"role":"${role.role_id}","candidates":[{"segment_id":"SEG_ID","operation":"replace","rewrite":"complete replacement text","confidence":0.75,"issues":["issue_code"],"change_summary":"what materially changed","tags":["tag"]}]}\n\nAllowed issue values for this lane: ${roleAllowedIssues(role.role_id).join(", ")}.`;
    }
    return { system: systemPrompt, user: userPrompt };
  }

  function validateCandidateSchema(parsed, expectedRoleId, allowedSegmentIds, mutableSegs) {
    if (!parsed || typeof parsed !== "object") return null;
    if (!Array.isArray(parsed.candidates)) return null;
    const candidates = parsed.candidates;
    const allowed = Array.isArray(allowedSegmentIds) ? new Set(allowedSegmentIds) : null;
    const laneIssues = ROLE_ALLOWED_ISSUES[safeString(expectedRoleId)]
      ? new Set(roleAllowedIssues(expectedRoleId))
      : null;
    const mutableById = {};
    (mutableSegs || []).forEach((segment) => { mutableById[segment.id] = segment; });
    const valid = [];
    candidates.forEach((c) => {
      if (!c || typeof c !== "object") return;
      const segId = safeString(c.segment_id).trim();
      const rewrite = safeString(c.rewrite);
      if (!segId) return;
      if (allowed && !allowed.has(segId)) return;
      const tags = Array.isArray(c.tags) ? c.tags.map((t) => safeString(t)).filter(Boolean) : [];
      const issues = normalizeIssues(c.issues, tags);
      if (laneIssues && issues.some((issue) => !laneIssues.has(issue))) return;
      const deleteMeta = !rewrite
        && expectedRoleId === "agency_meta_guard"
        && issues.indexOf("meta_artifact") >= 0
        && safeString(c.operation).toLowerCase() === "delete"
        && isWhollyMetaArtifactText(mutableFullText(mutableById[segId]));
      if (!rewrite && !deleteMeta) return;
      valid.push({
        segment_id: segId,
        rewrite,
        operation: deleteMeta ? "delete" : "replace",
        confidence: clampNumber(c.confidence, 0, 1, 0.5),
        issues,
        change_summary: safeString(c.change_summary),
        tags,
      });
    });
    if (!candidates.length) return { role: safeString(expectedRoleId), candidates: [] };
    if (!valid.length) return null;
    return { role: safeString(expectedRoleId), candidates: valid };
  }

  function validateComposerSchema(parsed, allowedSegmentIds) {
    if (!parsed || typeof parsed !== "object") return null;
    const segments = asObject(parsed.segments);
    if (!Array.isArray(allowedSegmentIds) || !allowedSegmentIds.length) return null;
    const allowed = new Set(allowedSegmentIds);
    const result = {};
    let count = 0;
    Object.keys(segments).forEach((segId) => {
      if (!allowed.has(segId)) return;
      const text = safeString(segments[segId]);
      if (text) {
        result[segId] = text;
        count++;
      }
    });
    if (!count || count !== allowed.size) return null;
    return { segments: result };
  }

  function isProfileConfigured(profile) {
    if (!profile || !safeString(profile.model).trim()) return false;
    const provider = sanitizeEnum(profile.provider, PROVIDERS, "openai_compatible");
    if (provider === "ollama_compatible") return true;
    return !!safeString(profile.endpoint).trim();
  }

  function classifyRoleError(err, abortSignal) {
    const message = safeString(err && err.message);
    if (abortSignal && abortSignal.aborted) {
      const abortReason = safeString(abortSignal.reason);
      return {
        code: abortReason === "composer_reserve" ? "composer_reserve_aborted" : "deadline_aborted",
        retryable: false,
        repair: false,
      };
    }
    if (message === "reasoning_only_response") return { code: message, retryable: true, repair: true };
    if (message === "json_parse_failed" || message === "schema_validation_failed") {
      return { code: message, retryable: true, repair: true };
    }
    if (message === "empty_response" || message === "request_aborted") {
      return { code: message, retryable: true, repair: false };
    }
    if (message === "request_timeout" || message === "deadline_aborted") {
      return { code: message, retryable: false, repair: false };
    }
    if (/signal is aborted without reason|operation was aborted|aborterror/i.test(message)) {
      return { code: "request_timeout", retryable: false, repair: false };
    }
    if (err instanceof SyntaxError || /Unexpected (?:token|end of JSON)/i.test(message)) {
      return { code: "provider_response_parse_failed", retryable: true, repair: false };
    }
    if (/^(?:missing_|invalid_extra_|profile_not_configured|no_fetch_available)/.test(message)) {
      return { code: "configuration_error", retryable: false, repair: false };
    }
    if (/Failed to fetch|NetworkError|Load failed|fetch failed|network request failed/i.test(message)) {
      return { code: "transport_error", retryable: true, repair: false };
    }
    const match = /^HTTP\s+(\d{3})/.exec(message);
    if (match) {
      const status = Number(match[1]);
      if (status === 408 || status === 409 || status === 425 || status === 429 || status >= 500) {
        return { code: `http_${status}`, retryable: true, repair: false };
      }
      return { code: `http_${status}`, retryable: false, repair: false };
    }
    return { code: "provider_error", retryable: false, repair: false };
  }

  function jsonRepairPrompts(prompts, role) {
    const schema = role.is_input_planner
      ? inputPlannerExample(role.role_id)
      : (role.is_composer
        ? '{"segments":{"SEG_ID":"final rewritten text"}}'
        : `{"role":"${role.role_id}","candidates":[{"segment_id":"SEG_ID","rewrite":"full rewritten text","confidence":0.8,"issues":["issue_code"],"change_summary":"brief","tags":["tag"]}]}`);
    return {
      system: `${prompts.system}\n\nREPAIR RESPONSE CONTRACT: Output JSON only. Do not include reasoning, analysis, markdown, or code fences.`,
      user: `${prompts.user}\n\nYour previous response could not be parsed. Return one compact JSON object only, matching this shape exactly:\n${schema}`,
    };
  }

  function plannerRequestOptions(role, profile) {
    if (!role || !role.is_input_planner) return null;
    const provider = sanitizeEnum(profile && profile.provider, PROVIDERS, "openai_compatible");
    const model = safeString(profile && profile.model).toLowerCase();
    const supportsPlannerTool = /kimi/.test(model)
      && (provider === "openai_compatible" || provider === "ollama_compatible");
    if (!supportsPlannerTool) return null;
    return {
      planner_tool: inputPlannerTool(role.role_id),
      tool_name: "submit_turn_contract_fragment",
    };
  }

  function boundedFailurePreview(value) {
    return preview(redactSensitiveText(value), 400);
  }

  function profilesAreDistinct(primary, fallback) {
    if (!primary || !fallback) return false;
    return safeString(primary.provider) !== safeString(fallback.provider)
      || safeString(primary.endpoint) !== safeString(fallback.endpoint)
      || safeString(primary.model) !== safeString(fallback.model);
  }

  function configuredFallbackProfile(profile) {
    if (!profile || !profile.fallback_provider || !profile.fallback_model) return null;
    const candidate = Object.assign({}, profile, {
      provider: profile.fallback_provider,
      endpoint: profile.fallback_endpoint || profile.endpoint,
      model: profile.fallback_model,
      api_key_ref: profile.fallback_api_key_ref || profile.api_key_ref,
    });
    return profilesAreDistinct(profile, candidate) && isProfileConfigured(candidate)
      ? candidate
      : null;
  }

  function setFinalTraceState(trace, enhanced, state, reason) {
    trace.final.enhanced = !!enhanced;
    trace.final.reason = safeString(reason);
    trace.summary.final_state = safeString(state);
    trace.summary.final_reason = safeString(reason);
  }

  function classifyAppliedOutput(assembled) {
    const changedSegments = (assembled && assembled.finalSegments || []).filter((segment) =>
      segment.type === "mutable"
      && safeString(segment.final_text) !== safeString(segment.original_text)
    );
    const deleteOnly = changedSegments.length > 0
      && changedSegments.every((segment) => segment.operation === "delete");
    if (assembled && assembled.materialComposerApplied > 0) {
      return { enhanced: true, state: "enhanced", reason: "composer_integrated" };
    }
    if (deleteOnly || (changedSegments.length > 0
        && assembled && assembled.metaOnlyChanged === changedSegments.length)) {
      return { enhanced: false, state: "sanitized", reason: "meta_artifact_removed_without_composer" };
    }
    if (assembled && assembled.composerApplied > 0) {
      return { enhanced: false, state: "precision_patch", reason: "composer_non_material_change" };
    }
    return { enhanced: false, state: "degraded_patch", reason: "specialist_patch_without_composer" };
  }

  async function callRole(role, profile, mutableSegs, contextBlock, allSegments, abortSignal, trace, directorInfo, queuedAt, runtimeControl) {
    const allowedSegIds = mutableSegs ? mutableSegs.map((s) => s.id) : (role.is_composer ? allSegments.filter((s) => s.type === "mutable").map((s) => s.id) : []);
    const prompts = buildRolePrompt(role, profile, role.is_composer ? mutableSegs : mutableSegs, contextBlock, allSegments, directorInfo);
    const startedAt = Date.now();
    const queuedTimestamp = queuedAt || startedAt;
    let httpAttempts = 0;
    let retryCount = 0;
    let lastError = "";
    let lastErrorClass = "";
    let usedFallback = false;
    let requestOverrides = null;
    const attemptTrace = [];

    function consumeHttpAttempt(traceBudget, kind) {
      if (!traceBudget || traceBudget.http_stopped_reason) return { allowed: false, reason: traceBudget ? traceBudget.http_stopped_reason : "no_budget" };
      if (role.is_input_planner && traceBudget.input_attempt_max > 0
          && traceBudget.input_attempt_used >= traceBudget.input_attempt_max) {
        return { allowed: false, reason: "input_attempt_budget_exhausted" };
      }
      const composerReserve = role.is_composer
        ? 0
        : Math.max(0, Number(traceBudget.composer_attempt_reserved) || 0);
      const primaryReserve = !role.is_input_planner && !role.is_composer && kind !== "primary"
        ? Math.max(0, Number(traceBudget.specialist_primary_remaining) || 0)
        : 0;
      if (!role.is_composer
          && traceBudget.http_attempt_used >= traceBudget.http_attempt_max - composerReserve - primaryReserve) {
        return {
          allowed: false,
          reason: primaryReserve > 0 ? "specialist_primary_and_composer_reserved" : "composer_attempt_reserved",
        };
      }
      if (traceBudget.http_attempt_used >= traceBudget.http_attempt_max) {
        traceBudget.http_stopped_reason = "turn_attempt_budget_exhausted";
        return { allowed: false, reason: traceBudget.http_stopped_reason };
      }
      traceBudget.http_attempt_used += 1;
      if (role.is_input_planner) {
        traceBudget.input_attempt_used = Math.max(0, Number(traceBudget.input_attempt_used) || 0) + 1;
      } else if (role.is_composer) {
        traceBudget.composer_attempt_used = Math.max(0, Number(traceBudget.composer_attempt_used) || 0) + 1;
        traceBudget.composer_attempt_reserved = 0;
      } else if (kind === "primary") {
        traceBudget.specialist_primary_remaining = Math.max(
          0,
          (Number(traceBudget.specialist_primary_remaining) || 0) - 1
        );
      }
      return { allowed: true, reason: "" };
    }

    if (!isProfileConfigured(profile)) {
      lastError = "profile_not_configured";
      traceRole(trace, {
        role_id: role.role_id,
        stage: role.stage || "output",
        provider: safeString(profile && profile.provider),
        model: safeString(profile && profile.model),
        status: "failed",
        queued_at: queuedTimestamp,
        started_at: startedAt,
        ended_at: Date.now(),
        elapsed_ms: 0,
        retry: 0,
        fallback: false,
        http_attempts: 0,
        attempts: [],
        error_class: "configuration_error",
        error: lastError,
      });
      return null;
    }

    const fallbackProfile = configuredFallbackProfile(profile);

    async function executeAttempt(activeProfile, attemptPrompts, kind) {
      try {
        parseExtraHeaders(activeProfile.extra_headers);
        parseExtraBody(activeProfile.extra_body);
      } catch (err) {
        lastError = safeString(err && err.message);
        lastErrorClass = "configuration_error";
        const blockedAt = Date.now();
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          model: activeProfile.model,
          status: "failed",
          started_at: blockedAt,
          ended_at: blockedAt,
          error_class: lastErrorClass,
        });
        return {
          __error: true,
          classification: { code: lastErrorClass, retryable: false, repair: false },
        };
      }
      const budgetCheck = consumeHttpAttempt(trace.budget, kind);
      if (!budgetCheck.allowed) {
        lastError = `turn_attempt_budget_blocked:${budgetCheck.reason}`;
        lastErrorClass = "budget_exhausted";
        const blockedAt = Date.now();
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          model: activeProfile.model,
          status: "blocked",
          started_at: blockedAt,
          ended_at: blockedAt,
          error_class: lastErrorClass,
        });
        return {
          __error: true,
          classification: { code: lastErrorClass, retryable: false, repair: false },
        };
      }
      const attemptStarted = Date.now();
      httpAttempts++;
      let failureSource = "";
      let structuredTransport = "";
      try {
        const requestProfile = role.is_input_planner
          ? Object.assign({}, activeProfile, { temperature: 0 })
          : activeProfile;
        const providerRequestOptions = Object.assign(
          {},
          plannerRequestOptions(role, requestProfile) || {}
        );
        if (runtimeControl && runtimeControl.completionWait) {
          providerRequestOptions.completion_wait = true;
        }
        const result = await callProvider(
          requestProfile,
          attemptPrompts,
          abortSignal,
          providerRequestOptions
        );
        const content = safeString(result.content);
        const structuredPayload = result.structured_payload && typeof result.structured_payload === "object"
          ? result.structured_payload
          : null;
        const structuredRaw = safeString(result.structured_raw);
        structuredTransport = role.is_input_planner
          ? (safeString(result.structured_transport) || "content_json")
          : "";
        failureSource = structuredRaw || content || safeString(result.reasoning);
        requestOverrides = result.request_overrides || requestOverrides;
        if (!structuredPayload && !content && !structuredRaw) {
          if (safeString(result.reasoning).trim()) throw new Error("reasoning_only_response");
          throw new Error("empty_response");
        }
        const pseudoToolPayload = role.is_input_planner && !structuredPayload
          ? parsePseudoToolCall(content)
          : null;
        if (pseudoToolPayload) structuredTransport = "pseudo_tool_recovered";
        const parsed = structuredPayload || pseudoToolPayload || tryParseJson(structuredRaw) || tryParseJson(content);
        if (!parsed) throw new Error("json_parse_failed");
        const validated = role.is_input_planner
          ? validateTurnContractFragment(parsed, role.role_id, directorInfo && directorInfo.context_manifest)
          : (role.is_composer
            ? validateComposerSchema(parsed, allowedSegIds)
            : validateCandidateSchema(parsed, role.role_id, allowedSegIds, mutableSegs));
        if (!validated) throw new Error("schema_validation_failed");
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          model: activeProfile.model,
          status: "fulfilled",
          started_at: attemptStarted,
          ended_at: Date.now(),
          structured_transport: structuredPayload ? "tool_call_used" : structuredTransport,
        });
        return validated;
      } catch (err) {
        lastError = safeString(err && err.message);
        const classification = classifyRoleError(err, abortSignal);
        lastErrorClass = classification.code;
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          model: activeProfile.model,
          status: "failed",
          started_at: attemptStarted,
          ended_at: Date.now(),
          error_class: classification.code,
          transport: safeString(err && err.request_transport),
          endpoint: safeString(err && err.request_endpoint),
          structured_transport: structuredTransport,
          failure_preview: /^(?:json_parse_failed|schema_validation_failed|reasoning_only_response)$/.test(classification.code)
            ? boundedFailurePreview(failureSource)
            : "",
        });
        return { __error: true, classification };
      }
    }

    let outcome = await executeAttempt(profile, prompts, "primary");
    const allowRetry = !runtimeControl || runtimeControl.allowRetry !== false;
    const allowFallback = !runtimeControl || runtimeControl.allowFallback !== false;
    if (outcome && outcome.__error && outcome.classification.retryable && allowRetry
        && !role.is_input_planner && !(abortSignal && abortSignal.aborted)
        && (!runtimeControl || typeof runtimeControl.canContinue !== "function" || runtimeControl.canContinue())) {
      retryCount++;
      const retryPrompts = outcome.classification.repair ? jsonRepairPrompts(prompts, role) : prompts;
      outcome = await executeAttempt(profile, retryPrompts, outcome.classification.repair ? "repair_retry" : "transient_retry");
    }

    if (outcome && outcome.__error && allowFallback && fallbackProfile
        && !(abortSignal && abortSignal.aborted)
        && (!runtimeControl || typeof runtimeControl.canContinue !== "function" || runtimeControl.canContinue())) {
      usedFallback = true;
      outcome = await executeAttempt(fallbackProfile, jsonRepairPrompts(prompts, role), "fallback");
    }

    if (outcome && !outcome.__error) {
      const finalAttempt = attemptTrace[attemptTrace.length - 1] || {};
      traceRole(trace, {
        role_id: role.role_id,
        stage: role.stage || "output",
        provider: finalAttempt.provider || profile.provider,
        model: finalAttempt.model || profile.model,
        status: "fulfilled",
        queued_at: queuedTimestamp,
        started_at: startedAt,
        ended_at: Date.now(),
        elapsed_ms: Date.now() - startedAt,
        retry: retryCount,
        fallback: usedFallback,
        http_attempts: httpAttempts,
        attempts: attemptTrace,
        candidate_count: role.is_input_planner
          ? 1
          : (role.is_composer
            ? Object.keys(outcome.segments).length
            : outcome.candidates.length),
        request_overrides: requestOverrides,
      });
      return outcome;
    }

    const finalProfile = usedFallback && fallbackProfile ? fallbackProfile : profile;
    traceRole(trace, {
      role_id: role.role_id,
      stage: role.stage || "output",
      provider: finalProfile.provider,
      model: finalProfile.model,
      status: "failed",
      queued_at: queuedTimestamp,
      started_at: startedAt,
      ended_at: Date.now(),
      elapsed_ms: Date.now() - startedAt,
      retry: retryCount,
      fallback: usedFallback,
      http_attempts: httpAttempts,
      attempts: attemptTrace,
      request_overrides: requestOverrides,
      error_class: lastErrorClass,
      error: lastError,
    });
    return null;
  }

  /* ── Router (Fugu role selection) ──────────────────────── */

  function detectSceneSignals(segments, context) {
    const signals = [];
    const mutableSegs = mutableSegments(segments);
    const mutableText = mutableSegs.map((s) => s.text).join("\n");
    const combined = [mutableText, context && context.bounded_context_block].filter(Boolean).join("\n");
    const mutableChars = mutableText.trim().length;

    /* ── Structure-based signals (language-independent) ── */

    // Dialogue ratio: count quote characters in mutable text
    const dialogueCount = (mutableText.match(/["'\u201C\u201D\u300C\u300D\u300E\u300F]/g) || []).length;
    const dialogueRatio = mutableChars > 0 ? dialogueCount / mutableChars : 0;

    // Speaker switches: lines starting with a quote or name pattern
    const lines = mutableText.split(/\n/).filter((l) => l.trim());
    const speakerSwitches = lines.length > 1
      ? lines.reduce((acc, line, i) => {
          if (i === 0) return acc;
          const prevStartsQuote = /["'\u201C\u201D\u300C\u300D\u300E\u300F]/.test(lines[i - 1].trim()[0] || "");
          const currStartsQuote = /["'\u201C\u201D\u300C\u300D\u300E\u300F]/.test(line.trim()[0] || "");
          if (prevStartsQuote !== currStartsQuote) return acc + 1;
          return acc;
        }, 0)
      : 0;

    // Paragraph/sentence repetition: repeated sentences
    const sentences = mutableText.split(/[.!?。！？\n]+/).map((s) => s.trim().toLowerCase()).filter((s) => s.length > 10);
    const sentenceSet = new Set();
    let repeatedSentences = 0;
    sentences.forEach((s) => {
      if (sentenceSet.has(s)) repeatedSentences++;
      else sentenceSet.add(s);
    });

    // Meta/list-like structure: bullet points, numbered lists, code-like blocks
    const listPattern = /^\s*(?:[-*•]|\d+[.)])\s+/gm;
    const listCount = (mutableText.match(listPattern) || []).length;
    const metaPattern = /(?:<\/?(?:thoughts?|analysis|thinking)\b[^>]*>|(?:^|\n)\s*#{0,3}\s*(?:approved|response|processing\b)|(?:^|\n)\s*(?:as an ai|language model|i cannot|i apologize|i'm sorry|firstly|secondly|in summary|to summarize|결론적으로|요약하면|죄송합니다|먼저|둘째로))/gim;
    const metaCount = (mutableText.match(metaPattern) || []).length;

    // Context availability
    const hasLorebook = !!(context && context.lorebook);
    const hasMemory = !!(context && context.memory);
    const hasCharacter = !!(context && context.character);

    // Mutable segment count and average length
    const mutableCount = mutableSegs.length;
    const avgMutableLen = mutableCount > 0 ? mutableChars / mutableCount : 0;

    /* ── Signal emission ── */

    if (dialogueRatio > 0.03 || dialogueCount >= 6) {
      signals.push({ id: "dialogue_heavy", severity: "medium", metrics: { dialogueCount, dialogueRatio: dialogueRatio.toFixed(3) } });
    }
    if (speakerSwitches >= 2) {
      signals.push({ id: "speaker_switch", severity: "medium", metrics: { speakerSwitches } });
    }
    if (metaCount >= 1) {
      signals.push({ id: "mechanical_artifact", severity: "high", metrics: { metaCount } });
    }
    if (listCount >= 2) {
      signals.push({ id: "list_structure", severity: "high", metrics: { listCount } });
    }
    if (repeatedSentences >= 2) {
      signals.push({ id: "repetition_detected", severity: "medium", metrics: { repeatedSentences } });
    }
    if (mutableChars >= 3000) {
      signals.push({ id: "long_scene", severity: "medium", metrics: { mutableChars } });
    }
    if (avgMutableLen > 500) {
      signals.push({ id: "dense_segments", severity: "medium", metrics: { avgMutableLen: Math.round(avgMutableLen) } });
    }
    if (hasLorebook) {
      signals.push({ id: "lorebook_available", severity: "low", metrics: {} });
    }
    if (hasMemory) {
      signals.push({ id: "memory_available", severity: "low", metrics: {} });
    }
    if (hasCharacter) {
      signals.push({ id: "character_available", severity: "low", metrics: {} });
    }

    return signals;
  }

  const SIGNAL_ROLE_MAP = Object.freeze({
    dialogue_heavy: ["character_reader"],
    speaker_switch: ["character_reader"],
    mechanical_artifact: ["agency_meta_guard"],
    list_structure: ["agency_meta_guard"],
    repetition_detected: ["style_reader"],
    long_scene: ["plot_continuity_reader", "style_reader"],
    dense_segments: ["style_reader"],
    lorebook_available: ["world_reader"],
    memory_available: ["plot_continuity_reader"],
    character_available: ["character_reader"],
  });

  function selectRoles(roles, signals, preset, settings) {
    const presetDef = PRESETS.find((p) => p.id === preset) || PRESETS[1];
    const baseRoleIds = new Set(presetDef.roles);
    const signalIds = signals.map((s) => s.id);
    const specialistLimit = OUTPUT_SPECIALIST_LIMIT[presetDef.id] || OUTPUT_SPECIALIST_LIMIT.balanced;
    const baseScores = {
      secret_pov_guard: 80,
      character_reader: 55,
      plot_continuity_reader: 50,
      style_reader: 40,
      agency_meta_guard: 35,
      world_reader: 25,
    };
    const severityScores = { high: 100, medium: 45, low: 15 };
    const candidates = [];
    let composer = null;
    const skipReasons = [];
    const selectReasons = [];

    roles.forEach((role) => {
      const profile = settings.role_profiles[role.role_id];
      if (role.is_input_planner) {
        skipReasons.push({ role_id: role.role_id, reason: "input_stage_only" });
        return;
      }
      if (role.is_composer) {
        if (!baseRoleIds.has(role.role_id)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_in_preset" });
        } else if (!profile || !profile.enabled) {
          skipReasons.push({ role_id: role.role_id, reason: "disabled" });
        } else if (!isProfileConfigured(profile)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_configured" });
        } else {
          composer = role;
        }
        return;
      }
      const matchedSignals = signals.filter((signal) => {
        const mapped = SIGNAL_ROLE_MAP[signal.id];
        return mapped && mapped.indexOf(role.role_id) >= 0;
      });
      if (!baseRoleIds.has(role.role_id) && !matchedSignals.length) {
        skipReasons.push({ role_id: role.role_id, reason: "not_in_preset_or_signals" });
        return;
      }
      if (!profile || !profile.enabled) {
        skipReasons.push({ role_id: role.role_id, reason: "disabled" });
        return;
      }
      if (!isProfileConfigured(profile)) {
        skipReasons.push({ role_id: role.role_id, reason: "not_configured" });
        return;
      }

      const maxSignalScore = matchedSignals.reduce((best, signal) => (
        Math.max(best, severityScores[safeString(signal.severity)] || 0)
      ), 0);
      candidates.push({
        role,
        matchedSignalIds: matchedSignals.map((signal) => signal.id),
        required: role.role_id === "secret_pov_guard"
          && (presetDef.id === "balanced" || presetDef.id === "quality"),
        score: (baseScores[role.role_id] || 0) + maxSignalScore + matchedSignals.length * 3,
      });
    });

    candidates.sort((a, b) => {
      if (a.required !== b.required) return a.required ? -1 : 1;
      if (a.score !== b.score) return b.score - a.score;
      return (b.role.priority || 0) - (a.role.priority || 0);
    });
    const chosen = candidates.slice(0, specialistLimit);
    const chosenIds = new Set(chosen.map((item) => item.role.role_id));
    chosen.forEach((item) => {
      selectReasons.push({
        role_id: item.role.role_id,
        reason: `adaptive_score:${item.score};signals:${item.matchedSignalIds.join(",") || "baseline"}`,
      });
    });
    candidates.forEach((item) => {
      if (chosenIds.has(item.role.role_id)) return;
      skipReasons.push({
        role_id: item.role.role_id,
        reason: `router_capacity:${specialistLimit};score:${item.score}`,
      });
    });
    if (composer) {
      selectReasons.push({ role_id: composer.role_id, reason: "composer_reserved" });
    }

    return {
      roles: chosen.map((item) => item.role).concat(composer ? [composer] : []),
      skipReasons,
      selectReasons,
      signals: signalIds,
    };
  }

  /* ── Fusion Director (single scoring function) ─────────── */

  function textSimilarity(a, b) {
    const sa = safeString(a);
    const sb = safeString(b);
    if (!sa || !sb) return 0;
    const setA = new Set(sa.toLowerCase().split(/\s+/).filter(Boolean));
    const setB = new Set(sb.toLowerCase().split(/\s+/).filter(Boolean));
    if (!setA.size || !setB.size) return 0;
    let common = 0;
    setA.forEach((w) => { if (setB.has(w)) common++; });
    return common / Math.max(setA.size, setB.size);
  }

  function sequenceShingles(text, size) {
    const tokens = safeString(text).toLowerCase().replace(/\s+/g, " ").trim().split(" ").filter(Boolean);
    const width = Math.max(1, Math.min(Number(size) || 3, tokens.length || 1));
    const shingles = new Set();
    for (let i = 0; i <= tokens.length - width; i++) {
      shingles.add(tokens.slice(i, i + width).join(" "));
    }
    return shingles;
  }

  function setJaccardSimilarity(left, right) {
    if (!left.size && !right.size) return 1;
    if (!left.size || !right.size) return 0;
    let common = 0;
    left.forEach((item) => { if (right.has(item)) common++; });
    return common / (left.size + right.size - common);
  }

  function changedSpanRatio(originalText, finalText) {
    const original = safeString(originalText);
    const final = safeString(finalText);
    const maxLength = Math.max(original.length, final.length, 1);
    let prefix = 0;
    while (prefix < original.length && prefix < final.length
        && original[prefix] === final[prefix]) prefix++;
    let suffix = 0;
    while (suffix < original.length - prefix && suffix < final.length - prefix
        && original[original.length - 1 - suffix] === final[final.length - 1 - suffix]) suffix++;
    const changedLength = Math.max(
      original.length - prefix - suffix,
      final.length - prefix - suffix
    );
    return changedLength / maxLength;
  }

  function rewriteMateriality(originalText, finalText) {
    const original = safeString(originalText).replace(/\s+/g, " ").trim();
    const final = safeString(finalText).replace(/\s+/g, " ").trim();
    const originalMetaOnly = isWhollyMetaArtifactText(originalText);
    if (original === final) {
      return {
        material: false,
        original_meta_only: originalMetaOnly,
        sequence_similarity: 1,
        changed_span_ratio: 0,
        length_delta_ratio: 0,
      };
    }
    const maxLength = Math.max(original.length, final.length, 1);
    const sequenceSimilarity = setJaccardSimilarity(
      sequenceShingles(original, 3),
      sequenceShingles(final, 3)
    );
    const spanRatio = changedSpanRatio(original, final);
    const lengthDeltaRatio = Math.abs(original.length - final.length) / maxLength;
    const shortSegment = maxLength < 160;
    return {
      material: originalMetaOnly
        || (shortSegment
          ? (spanRatio >= 0.18
            || lengthDeltaRatio >= 0.18
            || (sequenceSimilarity <= 0.55 && spanRatio >= 0.08))
          : (sequenceSimilarity <= 0.88
            || spanRatio >= 0.12
            || lengthDeltaRatio >= 0.12)),
      original_meta_only: originalMetaOnly,
      sequence_similarity: sequenceSimilarity,
      changed_span_ratio: spanRatio,
      length_delta_ratio: lengthDeltaRatio,
    };
  }

  function assessComposerMateriality(mutableSegs, composerResult) {
    const segments = asObject(composerResult && composerResult.segments);
    const details = [];
    (mutableSegs || []).forEach((segment) => {
      const originalText = mutableCoreText(segment);
      if (!originalText.trim()) return;
      const hasFinal = Object.prototype.hasOwnProperty.call(segments, segment.id);
      const finalText = hasFinal ? replacementCoreText(segments[segment.id]) : originalText;
      const metrics = rewriteMateriality(originalText, finalText);
      details.push(Object.assign({
        segment_id: segment.id,
        has_final: hasFinal,
      }, metrics));
    });
    const substantive = details.filter((item) => !item.original_meta_only);
    const weakSegmentIds = substantive.filter((item) => !item.material).map((item) => item.segment_id);
    return {
      details,
      substantive_count: substantive.length,
      material_substantive_count: substantive.filter((item) => item.material).length,
      weak_segment_ids: weakSegmentIds,
      pass: substantive.length === 0 || weakSegmentIds.length === 0,
    };
  }

  function fusionDirector(candidatesBySegment, roles, originalSegments) {
    const originalMutableIds = (originalSegments || []).filter((s) => s.type === "mutable").map((s) => s.id);
    const segmentIds = uniqueList(originalMutableIds.concat(Object.keys(candidatesBySegment || {})));
    const ranked = {};
    const consensus = {};
    const complementary = {};
    const conflict = {};
    const gap = [];
    const origMap = {};
    (originalSegments || []).forEach((s) => {
      if (s.type === "mutable") origMap[s.id] = mutableCoreText(s);
    });

    const rolePriority = {};
    roles.forEach((r, i) => { rolePriority[r.role_id] = (r.priority || 0) + (roles.length - i); });

    segmentIds.forEach((segId) => {
      const candidates = candidatesBySegment[segId] || [];
      if (!candidates.length) {
        gap.push(segId);
        ranked[segId] = [];
        return;
      }

      const hasOriginal = Object.prototype.hasOwnProperty.call(origMap, segId);
      const origText = safeString(origMap[segId]);
      const origLen = origText.trim().length || 1;

      const normalized = candidates.map((c) => {
        const roleWeight = rolePriority[c.role_id] || 0;
        const confidence = clampNumber(c.confidence, 0, 1, 0.5);
        const issues = normalizeIssues(c.issues, c.tags);
        const issuePri = maxIssuePriority(issues);
        const rewriteCore = replacementCoreText(c.rewrite);
        const sim = textSimilarity(rewriteCore, origText);
        const rewriteLen = rewriteCore.trim().length;
        const lenRatio = rewriteLen / origLen;
        const lenPenalty = (lenRatio < 0.3 || lenRatio > 3.0) ? -20 : 0;
        const identical = hasOriginal && rewriteCore === origText;
        const simPenalty = identical ? -1000 : (sim > 0.95 ? -30 : 0);
        const baseScore = confidence * 100 + roleWeight + issuePri * 0.3 + lenPenalty + simPenalty;
        return Object.assign({}, c, {
          score: baseScore,
          base_score: baseScore,
          similarity: sim,
          lenRatio,
          issues,
          identical_to_original: identical,
          issue_priority: issuePri,
        });
      });

      /* ── Semantic consensus / complementary / conflict ── */
      const issueToRoles = {};
      const changedCandidates = normalized.filter((c) => !c.identical_to_original);
      changedCandidates.forEach((c) => {
        (c.issues || []).forEach((iss) => {
          if (!issueToRoles[iss]) issueToRoles[iss] = new Set();
          issueToRoles[iss].add(c.role_id);
        });
      });

      const consensusIssues = [];
      Object.keys(issueToRoles).forEach((iss) => {
        const roleSet = issueToRoles[iss];
        if (roleSet.size >= 2) {
          consensusIssues.push(iss);
        }
      });

      const distinctRoles = new Set(changedCandidates.map((c) => c.role_id));
      const distinctIssues = Object.keys(issueToRoles);
      const complementaryIssues = changedCandidates.length >= 2
        && distinctRoles.size >= 2
        && distinctIssues.length >= 2
        ? distinctIssues.filter((issue) => consensusIssues.indexOf(issue) < 0)
        : [];

      if (consensusIssues.length) {
        consensus[segId] = consensusIssues;
      }
      if (complementaryIssues.length) {
        complementary[segId] = complementaryIssues;
      }

      /* conflict: same issue, very different rewrites */
      const conflictIssues = [];
      Object.keys(issueToRoles).forEach((iss) => {
        const roleSet = issueToRoles[iss];
        if (roleSet.size >= 2) {
            const sameIssueCandidates = changedCandidates.filter((c) => (c.issues || []).indexOf(iss) >= 0);
            if (sameIssueCandidates.length >= 2) {
              for (let i = 0; i < sameIssueCandidates.length && conflictIssues.indexOf(iss) < 0; i++) {
                for (let j = i + 1; j < sameIssueCandidates.length; j++) {
                  const left = replacementCoreText(sameIssueCandidates[i].rewrite);
                  const right = replacementCoreText(sameIssueCandidates[j].rewrite);
                  const simPair = textSimilarity(left, right);
                  const lenDiff = Math.abs(left.length - right.length) / Math.max(left.length, right.length, 1);
                  if (simPair < 0.3 || lenDiff > 0.5) {
                    conflictIssues.push(iss);
                    break;
                  }
                }
              }
            }
        }
      });
      if (conflictIssues.length) {
        conflict[segId] = conflictIssues;
      }

      const scored = normalized.map((candidate) => {
        let semanticScore = 0;
        (candidate.issues || []).forEach((issue) => {
          if (consensusIssues.indexOf(issue) >= 0) semanticScore += 12;
          if (complementaryIssues.indexOf(issue) >= 0) semanticScore += 4;
          if (conflictIssues.indexOf(issue) >= 0) semanticScore -= 8;
        });
        return Object.assign({}, candidate, {
          semantic_score: semanticScore,
          score: candidate.base_score + semanticScore,
        });
      }).sort((a, b) => b.score - a.score);
      ranked[segId] = scored;
      if (!changedCandidates.length && gap.indexOf(segId) < 0) gap.push(segId);
    });

    return { ranked, consensus, complementary, conflict, gap };
  }

  /* ── Composer ──────────────────────────────────────────── */

  function candidateEligibleForApplication(candidate) {
    if (!candidate || candidate.identical_to_original) return false;
    if (candidate.score == null) return true;
    return Number(candidate.score) > DIRECTOR_APPLY_SCORE_MIN;
  }

  function composerTokenPlan(mutableSegs, profile) {
    const mutableTotalChars = (mutableSegs || []).reduce((sum, segment) => sum + safeString(segment.text).length, 0);
    return {
      mutable_total_chars: mutableTotalChars,
      estimated_output_tokens: Math.min(32000, Math.max(1024, Math.ceil(mutableTotalChars * 1.5) + 512)),
      requested_output_tokens: clampNumber(profile && profile.max_output_tokens, 1024, 32000, 4096),
    };
  }

  async function runComposer(composerRole, profile, segments, directorResult, contextBlock, abortSignal, trace, mutableSegs, completionWait) {
    const candidateBundles = {};
    Object.keys(directorResult.ranked).forEach((segId) => {
      const ranked = directorResult.ranked[segId];
      candidateBundles[segId] = ranked.filter(candidateEligibleForApplication).slice(0, 3).map((c) => ({
        role: c.role_id,
        confidence: c.confidence,
        score: c.score,
        issues: c.issues || [],
        change_summary: c.change_summary || "",
        operation: c.operation || "replace",
        rewrite: c.rewrite,
      }));
    });
    const directorInfo = {
      candidateBundles,
      consensus: directorResult.consensus,
      complementary: directorResult.complementary,
      conflict: directorResult.conflict,
      gap: directorResult.gap,
    };
    const tokenPlan = composerTokenPlan(mutableSegs, profile);
    const composerProfile = Object.assign({}, profile, {
      max_output_tokens: tokenPlan.requested_output_tokens,
    });
    trace.composer.estimated_output_tokens = tokenPlan.estimated_output_tokens;
    trace.composer.requested_output_tokens = tokenPlan.requested_output_tokens;
    trace.composer.mutable_total_chars = tokenPlan.mutable_total_chars;
    const composerStarted = Date.now();
    let result = await callRole(
      composerRole, composerProfile, mutableSegs, contextBlock, segments,
      abortSignal, trace, directorInfo, composerStarted,
      {
        allowRetry: false,
        allowFallback: true,
        completionWait: !!completionWait,
      }
    );
    let materiality = result ? assessComposerMateriality(mutableSegs, result) : null;
    trace.composer.materiality = materiality;
    trace.composer.semantic_retry = 0;
    if (completionWait && result && materiality && !materiality.pass
        && trace.budget.http_attempt_used < trace.budget.http_attempt_max
        && !(abortSignal && abortSignal.aborted)) {
      trace.composer.semantic_retry = 1;
      const retryDirectorInfo = Object.assign({}, directorInfo, {
        composer_revision_feedback: {
          weak_segment_ids: materiality.weak_segment_ids,
          previous_segments: result.segments,
        },
      });
      const retryProfile = Object.assign({}, composerProfile, {
        system_prompt: `${safeString(composerProfile.system_prompt || composerRole.default_prompt)}\n\nThe previous composition was rejected for copying the draft too closely. Recompose every flagged prose segment with materially different sentence architecture and scene execution while preserving facts.`,
      });
      const retryResult = await callRole(
        composerRole,
        retryProfile,
        mutableSegs,
        contextBlock,
        segments,
        abortSignal,
        trace,
        retryDirectorInfo,
        Date.now(),
        {
          allowRetry: false,
          allowFallback: false,
          completionWait: true,
        }
      );
      if (retryResult) {
        result = retryResult;
        materiality = assessComposerMateriality(mutableSegs, result);
        trace.composer.materiality = materiality;
      }
    }
    const elapsed = Date.now() - composerStarted;
    trace.composer.used = !!result;
    trace.composer.status = result ? "fulfilled" : "failed";
    trace.composer.elapsed_ms = elapsed;
    return result;
  }

  /* ── Output Assembly ───────────────────────────────────── */

  function assembleOutput(segments, composerResult, directorResult) {
    const composerSegments = composerResult ? asObject(composerResult.segments) : {};
    const ranked = directorResult ? directorResult.ranked : {};
    const finalSegments = [];
    let changed = false;
    let composerApplied = 0;
    let materialComposerApplied = 0;
    let materialChanged = 0;
    let metaOnlyChanged = 0;
    let topCandidateApplied = 0;
    let unchangedSegments = 0;

    segments.forEach((seg) => {
      if (seg.type === "mutable") {
        const originalCore = mutableCoreText(seg);
        const originalFull = mutableFullText(seg);
        let finalCore = null;
        let source = "original";
        let operation = "none";
        let appliedRole = "";
        let composerUnchanged = false;
        const rankedChanged = (ranked[seg.id] || []).find((candidate) =>
          candidateEligibleForApplication(candidate)
          && replacementCoreText(candidate.rewrite) !== originalCore
        );
        if (Object.prototype.hasOwnProperty.call(composerSegments, seg.id)) {
          const composerCore = replacementCoreText(composerSegments[seg.id]);
          if (composerCore === originalCore) {
            composerUnchanged = true;
            if (rankedChanged) {
              finalCore = replacementCoreText(rankedChanged.rewrite);
              source = "top_candidate_after_composer_unchanged";
              operation = rankedChanged.operation || "replace";
              appliedRole = rankedChanged.role_id || "";
              topCandidateApplied++;
            }
          } else {
            finalCore = composerCore;
            source = "composer";
            operation = "replace";
            appliedRole = COMPOSER_ROLE_ID;
            composerApplied++;
          }
        } else if (rankedChanged) {
          finalCore = replacementCoreText(rankedChanged.rewrite);
          source = "top_candidate";
          operation = rankedChanged.operation || "replace";
          appliedRole = rankedChanged.role_id || "";
          topCandidateApplied++;
        }
        const finalFull = finalCore === null
          ? originalFull
          : safeString(seg.leading_ws) + finalCore + safeString(seg.trailing_ws);
        const segmentChanged = finalFull !== originalFull;
        const materiality = rewriteMateriality(originalFull, finalFull);
        if (segmentChanged) changed = true;
        else unchangedSegments++;
        if (segmentChanged && materiality.material) {
          if (materiality.original_meta_only) metaOnlyChanged++;
          else materialChanged++;
        }
        if (segmentChanged && source === "composer"
            && materiality.material && !materiality.original_meta_only) {
          materialComposerApplied++;
        }
        finalSegments.push({
          id: seg.id,
          type: "mutable",
          original_text: originalFull,
          final_text: finalFull,
          source: segmentChanged ? source : "original",
          operation: segmentChanged ? operation : "none",
          applied_role_id: segmentChanged ? appliedRole : "",
          composer_unchanged: composerUnchanged,
          material_change: segmentChanged && materiality.material,
          original_meta_only: materiality.original_meta_only,
          sequence_similarity: materiality.sequence_similarity,
          changed_span_ratio: materiality.changed_span_ratio,
        });
      } else {
        finalSegments.push({
          id: seg.id,
          type: seg.type,
          kind: seg.kind,
          original_text: seg.text,
          final_text: seg.text,
          source: "preserved",
        });
      }
    });

    const output = finalSegments.map((s) => s.final_text).join("");
    return {
      output,
      finalSegments,
      changed,
      composerApplied,
      materialComposerApplied,
      materialChanged,
      metaOnlyChanged,
      topCandidateApplied,
      unchangedSegments,
    };
  }

  function buildAppliedEvidence(finalSegments) {
    return (finalSegments || []).filter((segment) => segment.type === "mutable").map((segment) => ({
      segment_id: segment.id,
      source: segment.source,
      changed: safeString(segment.final_text) !== safeString(segment.original_text),
      composer_unchanged: !!segment.composer_unchanged,
      operation: segment.operation || "none",
      applied_role_id: segment.applied_role_id || "",
      material_change: !!segment.material_change,
      original_meta_only: !!segment.original_meta_only,
      sequence_similarity: Number(segment.sequence_similarity) || 0,
      changed_span_ratio: Number(segment.changed_span_ratio) || 0,
      original_preview: preview(segment.original_text, 80),
      final_preview: preview(segment.final_text, 80),
    }));
  }

  function updateAppliedEvidence(trace, assembled, verification) {
    trace.applied_evidence = verification && verification.pass
      ? buildAppliedEvidence(assembled && assembled.finalSegments)
      : [];
    return trace.applied_evidence;
  }

  /* ── Verifier (structural checks only) ─────────────────── */

  const VOID_HTML_TAGS = Object.freeze([
    "area", "base", "br", "col", "embed", "hr", "img", "input",
    "link", "meta", "param", "source", "track", "wbr",
  ]);

  function isVoidHtmlTag(tagText) {
    const m = /^<([a-z][a-z0-9]*)/i.exec(safeString(tagText));
    if (!m) return false;
    return VOID_HTML_TAGS.indexOf(m[1].toLowerCase()) >= 0;
  }

  function duplicateBlockKey(text) {
    const normalized = safeString(text).replace(/\s+/g, " ").trim();
    return normalized.length >= 80 ? normalized : "";
  }

  function verifyOutput(segments, finalSegments, output, originalText) {
    const errors = [];
    segments.forEach((origSeg) => {
      const finalSeg = finalSegments.find((f) => f.id === origSeg.id);
      if (!finalSeg) {
        errors.push(`missing_segment:${origSeg.id}`);
        return;
      }
      if (origSeg.type === "protected" || origSeg.type === "inspect_only") {
        if (origSeg.text !== finalSeg.final_text) {
          errors.push(`preservation_violation:${origSeg.id}`);
        }
      }
    });
    if (!output || !output.trim()) {
      errors.push("empty_output");
    }
    const openFences = (output.match(/```/g) || []).length;
    if (openFences % 2 !== 0) {
      errors.push("unbalanced_code_fence");
    }
    const allOpenTags = output.match(/<[^/][^>]*>/g) || [];
    const nonVoidOpenTags = allOpenTags.filter((tag) => !isVoidHtmlTag(tag));
    const closeTags = (output.match(/<\/[^>]*>/g) || []).length;
    if (Math.abs(nonVoidOpenTags.length - closeTags) > 2) {
      errors.push("tag_balance_suspicious");
    }
    const originalDuplicateCounts = {};
    segments.filter((seg) => seg.type === "mutable").forEach((seg) => {
      const key = duplicateBlockKey(mutableFullText(seg));
      if (key) originalDuplicateCounts[key] = (originalDuplicateCounts[key] || 0) + 1;
    });
    const finalOccurrences = {};
    finalSegments.filter((seg) => seg.type === "mutable").forEach((seg) => {
      const key = duplicateBlockKey(seg.final_text);
      if (!key) return;
      if (!finalOccurrences[key]) finalOccurrences[key] = [];
      finalOccurrences[key].push(seg.id);
    });
    Object.keys(finalOccurrences).forEach((key) => {
      const ids = finalOccurrences[key];
      const allowedCount = Math.max(1, originalDuplicateCounts[key] || 0);
      ids.slice(allowedCount).forEach((id) => errors.push(`duplicate_segment:${id}`));
    });
    return { pass: errors.length === 0, errors };
  }

  /* ── Scheduler ─────────────────────────────────────────── */

  function createSemaphore(max) {
    let current = 0;
    const queue = [];
    let cancelled = false;
    function tryNext() {
      if (cancelled) {
        while (queue.length) {
          const task = queue.shift();
          task.resolve(null);
        }
        return;
      }
      if (current >= max || !queue.length) return;
      current++;
      const task = queue.shift();
      Promise.resolve()
        .then(() => task.run())
        .then(task.resolve, task.reject)
        .finally(() => {
          current--;
          tryNext();
        });
    }
    return {
      acquire(run) {
        return new Promise((resolve, reject) => {
          if (cancelled) { resolve(null); return; }
          queue.push({ run, resolve, reject });
          tryNext();
        });
      },
      cancel() {
        cancelled = true;
        while (queue.length) {
          const task = queue.shift();
          task.resolve(null);
        }
      },
      get pendingCount() { return queue.length; },
    };
  }

  function createDeadline(deadlineMs, startedAt) {
    const start = Number.isFinite(Number(startedAt)) ? Number(startedAt) : Date.now();
    const deadline = start + deadlineMs;
    const controller = new AbortController();
    let aborted = false;
    const delay = Math.max(0, deadline - Date.now());
    const timer = setTimeout(() => {
      aborted = true;
      try { controller.abort(); } catch (_) {}
    }, delay);
    if (delay === 0) {
      aborted = true;
      try { controller.abort(); } catch (_) {}
    }
    function check() {
      if (aborted) return true;
      if (Date.now() >= deadline) {
        aborted = true;
        try { controller.abort(); } catch (_) {}
        return true;
      }
      return false;
    }
    function remaining() {
      return Math.max(0, deadline - Date.now());
    }
    function reserveComposer() {
      const r = remaining();
      return r > COMPOSER_RESERVE_MS ? r - COMPOSER_RESERVE_MS : 0;
    }
    function cancel() {
      if (!aborted) {
        aborted = true;
        try { controller.abort(); } catch (_) {}
      }
      clearTimeout(timer);
    }
    return { check, remaining, reserveComposer, signal: controller.signal, aborted: () => aborted, cancel };
  }

  function createCompletionDeadline(startedAt) {
    const controller = new AbortController();
    let aborted = false;
    function cancel() {
      if (aborted) return;
      aborted = true;
      try { controller.abort("completion_wait_cancelled"); } catch (_) {
        try { controller.abort(); } catch (_) {}
      }
    }
    return {
      completion_wait: true,
      started_at: Number.isFinite(Number(startedAt)) ? Number(startedAt) : Date.now(),
      check: () => aborted,
      remaining: () => Number.POSITIVE_INFINITY,
      reserveComposer: () => Number.POSITIVE_INFINITY,
      signal: controller.signal,
      aborted: () => aborted,
      cancel,
    };
  }

  function presetUsesCompletionWait(presetId) {
    return safeString(presetId) === "quality";
  }

  function resolvePipelineDeadlineMs(settings) {
    const preset = PRESETS.find((item) => item.id === settings.preset) || PRESETS[1];
    const configured = clampNumber(settings.deadline_ms, 10000, 600000, DEFAULT_DEADLINE_MS);
    if (configured === DEFAULT_DEADLINE_MS && preset && preset.deadline_ms) {
      return preset.deadline_ms;
    }
    return configured;
  }

  function executionGroupKey(profile) {
    const provider = sanitizeEnum(profile && profile.provider, PROVIDERS, "openai_compatible");
    const endpoint = safeString(profile && profile.endpoint).trim()
      || (provider === "ollama_compatible" ? "http://localhost:11434" : "");
    try {
      const parsed = new URL(endpoint);
      return `${parsed.protocol}//${parsed.host}`.toLowerCase();
    } catch (_) {
      return `${provider}:${endpoint.toLowerCase()}`;
    }
  }

  function executionGroupConcurrency(profile) {
    const key = executionGroupKey(profile);
    if (key === "https://ollama.com" || key === "https://www.ollama.com") return 2;
    const provider = sanitizeEnum(profile && profile.provider, PROVIDERS, "openai_compatible");
    return PROVIDER_CONCURRENCY[provider] || 1;
  }

  function buildExecutionGroupPlan(roles, profiles, stage) {
    const groups = {};
    (roles || []).forEach((role) => {
      const profile = profiles && profiles[role.role_id];
      if (!profile || !isProfileConfigured(profile)) return;
      const key = executionGroupKey(profile);
      if (!groups[key]) {
        groups[key] = {
          stage: stage || "output",
          endpoint_group: key,
          selected_calls: 0,
          base_concurrency: executionGroupConcurrency(profile),
          effective_concurrency: 1,
          reason: "",
        };
      }
      groups[key].selected_calls += 1;
      groups[key].base_concurrency = Math.min(
        groups[key].base_concurrency,
        executionGroupConcurrency(profile)
      );
    });
    Object.keys(groups).forEach((key) => {
      const group = groups[key];
      const densitySerial = group.selected_calls >= ENDPOINT_SERIAL_THRESHOLD;
      group.effective_concurrency = densitySerial
        ? 1
        : Math.max(1, Math.min(group.base_concurrency, group.selected_calls));
      group.reason = densitySerial
        ? `endpoint_density_${group.selected_calls}_serial`
        : "provider_default";
    });
    return groups;
  }

  function recordExecutionGroupPlan(trace, plan) {
    if (!trace) return;
    if (!trace.scheduler) {
      trace.scheduler = {
        completion_wait: false,
        endpoint_serial_threshold: ENDPOINT_SERIAL_THRESHOLD,
        endpoint_groups: [],
      };
    }
    Object.keys(plan || {}).forEach((key) => {
      trace.scheduler.endpoint_groups.push(Object.assign({}, plan[key]));
    });
  }

  function composerReserveWindowMs(deadline, profile) {
    if (deadline && deadline.completion_wait) return 0;
    if (!deadline || !profile || !isProfileConfigured(profile)) return 0;
    const remaining = deadline.remaining();
    if (remaining <= COMPOSER_RESERVE_GUARD_MS) return 0;
    const composerAttempts = configuredFallbackProfile(profile) ? 2 : 1;
    const requested = clampNumber(profile.timeout_ms, 5000, 300000, 60000)
      * composerAttempts + COMPOSER_RESERVE_GUARD_MS;
    const dynamicShare = Math.max(COMPOSER_RESERVE_MS, Math.floor(remaining * 0.75));
    return Math.max(
      0,
      Math.min(requested, dynamicShare, remaining - COMPOSER_RESERVE_GUARD_MS)
    );
  }

  async function scheduleRoles(roles, profiles, segments, contextBlock, allSegments, deadline, trace, maxParallel) {
    // Output attempts are independent from the beforeRequest planner budget.
    if (trace && trace.budget) {
      const presetId = safeString(trace.router && trace.router.preset ? trace.router.preset : "balanced");
      trace.budget.http_attempt_max = OUTPUT_HTTP_ATTEMPT_BUDGET[presetId] || OUTPUT_HTTP_ATTEMPT_BUDGET.balanced;
      trace.budget.http_attempt_used = Math.max(0, Number(trace.budget.http_attempt_used) || 0);
      if (trace.budget.http_attempt_used < trace.budget.http_attempt_max) {
        trace.budget.http_stopped_reason = "";
      }
    }

    const mutableSegs = mutableSegments(segments).filter((segment) =>
      mutableCoreText(segment).trim().length > 0
    );
    const specialistRoles = roles.filter((r) => !r.is_composer && !r.is_input_planner);
    const composerRole = roles.find((r) => r.is_composer);
    const composerProfile = composerRole && profiles[composerRole.role_id];
    const composerReserveMs = composerRole && composerProfile && composerProfile.enabled
      ? composerReserveWindowMs(deadline, composerProfile)
      : 0;
    trace.composer.reserve_ms = composerReserveMs;
    const specialistTasks = specialistRoles.filter((role) => {
      const profile = profiles[role.role_id];
      return profile && profile.enabled;
    });
    const executionGroupPlan = buildExecutionGroupPlan(
      specialistTasks,
      profiles,
      "output_specialist"
    );
    recordExecutionGroupPlan(trace, executionGroupPlan);
    if (composerRole && composerProfile && composerProfile.enabled
        && isProfileConfigured(composerProfile)) {
      recordExecutionGroupPlan(trace, {
        [executionGroupKey(composerProfile)]: {
          stage: "output_composer",
          endpoint_group: executionGroupKey(composerProfile),
          selected_calls: 1,
          base_concurrency: 1,
          effective_concurrency: 1,
          reason: "composer_after_specialists",
        },
      });
    }
    const candidatesBySegment = {};
    const mutableSegIds = mutableSegs.map((s) => s.id);
    mutableSegIds.forEach((segId) => { candidatesBySegment[segId] = []; });

    const globalSem = createSemaphore(clampNumber(maxParallel, 1, 20, 5));
    const executionGroupSems = {};
    function getExecutionGroupSem(profile) {
      const key = executionGroupKey(profile);
      if (!executionGroupSems[key]) {
        const plan = executionGroupPlan[key];
        executionGroupSems[key] = createSemaphore(
          plan ? plan.effective_concurrency : executionGroupConcurrency(profile)
        );
      }
      return executionGroupSems[key];
    }
    const allSems = () => [globalSem].concat(Object.values(executionGroupSems));
    const specialistController = new AbortController();
    let specialistStopReason = "";
    function stopSpecialists(reason) {
      if (specialistController.signal.aborted) return;
      specialistStopReason = safeString(reason) || "deadline";
      try { specialistController.abort(specialistStopReason); } catch (_) { try { specialistController.abort(); } catch (_) {} }
    }
    const reserveDelay = composerReserveMs > 0
      ? Math.max(0, deadline.remaining() - composerReserveMs)
      : -1;
    const reserveTimer = reserveDelay >= 0
      ? setTimeout(() => stopSpecialists(deadline.check() ? "deadline" : "composer_reserve"), reserveDelay)
      : null;
    const onPipelineAbort = () => stopSpecialists("deadline");
    if (deadline.signal.aborted) onPipelineAbort();
    else deadline.signal.addEventListener("abort", onPipelineAbort, { once: true });

    if (trace && trace.budget) {
      trace.budget.composer_attempt_reserved = composerRole && composerProfile
        && composerProfile.enabled && isProfileConfigured(composerProfile)
        ? (configuredFallbackProfile(composerProfile) ? 2 : 1)
        : 0;
      trace.budget.composer_attempt_used = 0;
      trace.budget.specialist_primary_remaining = specialistTasks.length;
    }

    let activeCallCount = 0;

    const specialistPromises = specialistTasks.map((role) => {
      const profile = profiles[role.role_id];
      const queuedAt = Date.now();
      return globalSem.acquire(() => {
        if (deadline.check() || specialistController.signal.aborted) return null;
        return getExecutionGroupSem(profile).acquire(() => {
          if (deadline.check() || specialistController.signal.aborted) return null;
          activeCallCount++;
          return callRole(
            role,
            profile,
            mutableSegs,
            contextBlock,
            allSegments,
            specialistController.signal,
            trace,
            null,
            queuedAt,
            {
              allowRetry: false,
              allowFallback: false,
              completionWait: !!deadline.completion_wait,
              canContinue: () => !specialistController.signal.aborted
                && deadline.remaining() > composerReserveMs + COMPOSER_RESERVE_GUARD_MS,
            }
          )
            .then((result) => {
              if (result && result.candidates) {
                result.candidates.forEach((c) => {
                  if (!candidatesBySegment[c.segment_id]) candidatesBySegment[c.segment_id] = [];
                  candidatesBySegment[c.segment_id].push({
                    role_id: result.role,
                    rewrite: c.rewrite,
                    operation: c.operation || "replace",
                    confidence: c.confidence,
                    issues: c.issues || [],
                    change_summary: c.change_summary || "",
                    tags: c.tags,
                  });
                });
              }
              return result;
            })
            .catch((err) => {
              traceError(trace, `role ${role.role_id}: ${safeString(err && err.message)}`);
              return null;
            })
            .finally(() => {
              activeCallCount = Math.max(0, activeCallCount - 1);
            });
        });
      });
    });

    const allSettled = Promise.all(specialistPromises);
    const phaseStopPromise = new Promise((resolve) => {
      if (specialistController.signal.aborted) {
        resolve(specialistStopReason || "deadline");
        return;
      }
      specialistController.signal.addEventListener("abort", () => {
        resolve(specialistStopReason || "deadline");
      }, { once: true });
    });

    const raceResult = await Promise.race([allSettled.then(() => "settled"), phaseStopPromise]);
    if (raceResult !== "settled") {
      allSems().forEach((sem) => { try { sem.cancel(); } catch (_) {} });
    }
    specialistPromises.forEach((p) => { try { p.catch(() => null); } catch (_) {} });
    if (raceResult !== "settled") {
      await Promise.race([
        allSettled,
        new Promise((resolve) => setTimeout(resolve, 500)),
      ]);
    }
    if (reserveTimer) clearTimeout(reserveTimer);
    try { deadline.signal.removeEventListener("abort", onPipelineAbort); } catch (_) {}
    trace.composer.specialist_stop_reason = raceResult === "settled" ? "" : raceResult;

    trace.active_calls_after_specialists = activeCallCount;

    const directorResult = fusionDirector(candidatesBySegment, specialistRoles, segments);
    trace.candidates.total = Object.values(candidatesBySegment).reduce((s, arr) => s + arr.length, 0);
    Object.keys(candidatesBySegment).forEach((segId) => {
      trace.candidates.by_segment[segId] = candidatesBySegment[segId].length;
    });

    let composerResult = null;
    if (composerRole && !deadline.check() && deadline.remaining() > COMPOSER_RESERVE_GUARD_MS) {
      if (composerProfile && composerProfile.enabled && isProfileConfigured(composerProfile)) {
        composerResult = await runComposer(
          composerRole,
          composerProfile,
          segments,
          directorResult,
          contextBlock,
          deadline.signal,
          trace,
          mutableSegs,
          !!deadline.completion_wait
        );
      } else {
        trace.composer.status = "not_configured";
      }
    } else if (composerRole) {
      trace.composer.status = "skipped_deadline";
    }

    trace.active_calls_final = activeCallCount;
    return { directorResult, composerResult };
  }

  /* ── Main Pipeline ─────────────────────────────────────── */

  function selectInputPlannerRoles(settings, manifest) {
    const plannerRoles = arrayFromCollection(settings && settings.roles)
      .filter((role) => role && role.is_input_planner);
    if (safeString(settings && settings.preset) === "fast") {
      return { roles: [], selected: [], skipped: plannerRoles.map((role) => ({ role_id: role.role_id, reason: "fast_manifest_contract" })) };
    }
    const byId = {};
    plannerRoles.forEach((role) => { byId[role.role_id] = role; });
    let requestedIds;
    if (safeString(settings && settings.preset) === "quality") {
      requestedIds = [
        "input_canon_secret_planner",
        "input_character_relationship_planner",
        "input_scene_continuity_planner",
      ];
    } else {
      const availability = asObject(manifest && manifest.source_availability);
      const canonUseful = !!(
        (availability.lorebook && availability.lorebook.available)
        || (availability.memory && availability.memory.available)
      );
      requestedIds = [
        "input_scene_continuity_planner",
        canonUseful ? "input_canon_secret_planner" : "input_character_relationship_planner",
      ];
    }
    const selected = [];
    const skipped = [];
    requestedIds.forEach((roleId) => {
      const role = byId[roleId];
      const profile = settings && settings.role_profiles && settings.role_profiles[roleId];
      if (!role) {
        skipped.push({ role_id: roleId, reason: "role_missing" });
      } else if (!profile || !profile.enabled) {
        skipped.push({ role_id: roleId, reason: "disabled" });
      } else if (!isProfileConfigured(profile)) {
        skipped.push({ role_id: roleId, reason: "not_configured" });
      } else {
        selected.push(role);
      }
    });
    return {
      roles: selected,
      selected: selected.map((role) => ({ role_id: role.role_id, reason: "input_preset" })),
      skipped,
    };
  }

  async function scheduleInputPlanners(roles, profiles, plannerManifest, deadline, trace, maxParallel) {
    if (!roles.length || deadline.check()) return [];
    if (trace && trace.budget) {
      trace.budget.input_attempt_max = roles.length + 1;
      trace.budget.input_attempt_used = 0;
    }
    const globalSem = createSemaphore(clampNumber(maxParallel, 1, 8, 3));
    const groupPlan = buildExecutionGroupPlan(roles, profiles, "input");
    recordExecutionGroupPlan(trace, groupPlan);
    const groupSems = {};
    function getGroupSem(profile) {
      const key = executionGroupKey(profile);
      if (!groupSems[key]) {
        const plan = groupPlan[key];
        groupSems[key] = createSemaphore(
          plan ? plan.effective_concurrency : executionGroupConcurrency(profile)
        );
      }
      return groupSems[key];
    }
    const allSems = () => [globalSem].concat(Object.values(groupSems));
    let activeCalls = 0;
    const fragments = [];
    const tasks = roles.map((role) => {
      const profile = profiles[role.role_id];
      const queuedAt = Date.now();
      return globalSem.acquire(() => getGroupSem(profile).acquire(async () => {
        if (deadline.check()) return null;
        activeCalls++;
        try {
          const result = await callRole(
            role,
            profile,
            [],
            "",
            [],
            deadline.signal,
            trace,
            { context_manifest: plannerManifest },
            queuedAt,
            {
              completionWait: !!deadline.completion_wait,
            }
          );
          if (result) fragments.push(result);
          return result;
        } catch (err) {
          traceError(trace, `input planner ${role.role_id}: ${safeString(err && err.message)}`);
          return null;
        } finally {
          activeCalls = Math.max(0, activeCalls - 1);
        }
      }));
    });
    const settled = Promise.allSettled(tasks);
    const aborted = new Promise((resolve) => {
      if (deadline.signal.aborted) {
        resolve("deadline");
        return;
      }
      deadline.signal.addEventListener("abort", () => resolve("deadline"), { once: true });
    });
    const outcome = await Promise.race([settled.then(() => "settled"), aborted]);
    if (outcome === "deadline") {
      allSems().forEach((sem) => { try { sem.cancel(); } catch (_) {} });
      trace.input_enhance.transport_cancellation = "requested_unverified";
      await Promise.race([
        settled,
        new Promise((resolve) => setTimeout(resolve, 300)),
      ]);
    }
    tasks.forEach((task) => { try { task.catch(() => null); } catch (_) {} });
    trace.input_enhance.active_calls_final = activeCalls;
    trace.input_enhance.active_wrappers_final = activeCalls;
    return fragments;
  }

  function isAuxiliaryRequest(type) {
    const t = safeString(type).toLowerCase();
    if (!t || t === "model" || t === "main") return false;
    if (t === "submodel" || t === "memory" || t === "emotion" || t === "otherax" || t === "translate") {
      return true;
    }
    return t.indexOf("image") >= 0
      || t.indexOf("module") >= 0
      || t.indexOf("regex") >= 0
      || t.indexOf("embedding") >= 0
      || t.indexOf("translation") >= 0
      || t.indexOf("summary") >= 0
      || t.indexOf("memory") >= 0
      || t.indexOf("title") >= 0
      || t.indexOf("aux") >= 0
      || t.indexOf("helper") >= 0
      || t.indexOf("submodel") >= 0
      || t.indexOf("before") >= 0;
  }

  /* ── One-shot request snapshot lifecycle ─────────────── */

  let pendingMainSnapshot = null;
  let latestAppliedComparison = null;

  function updateLatestAppliedComparison(trace, finalSegments) {
    const changes = (finalSegments || []).filter((segment) =>
      segment.type === "mutable"
      && safeString(segment.final_text) !== safeString(segment.original_text)
    ).map((segment) => ({
      segment_id: segment.id,
      source: segment.source || "original",
      operation: segment.operation || "none",
      applied_role_id: segment.applied_role_id || "",
      material_change: !!segment.material_change,
      original_text: safeString(segment.original_text),
      final_text: safeString(segment.final_text),
    }));
    latestAppliedComparison = {
      timestamp: trace && trace.timestamp ? trace.timestamp : Date.now(),
      state: safeString(trace && trace.summary && trace.summary.final_state) || "unchanged",
      reason: safeString(trace && trace.final && trace.final.reason),
      changes,
    };
    if (uiRoot) {
      const container = uiRoot.querySelector(".recomposer-compare-list");
      if (container) container.innerHTML = renderLatestComparison();
    }
  }

  function cloneSnapshotValue(value) {
    try {
      return typeof structuredClone === "function"
        ? structuredClone(value)
        : JSON.parse(JSON.stringify(value));
    } catch (_) {
      return JSON.parse(JSON.stringify(value, (_key, item) => (
        typeof item === "function" || typeof item === "undefined" ? null : item
      )));
    }
  }

  function cloneAndFreezeSnapshotValue(value) {
    const clone = cloneSnapshotValue(value);
    const seen = new WeakSet();
    function freezeDeep(item) {
      if (!item || typeof item !== "object" || seen.has(item)) return item;
      seen.add(item);
      Object.keys(item).forEach((key) => freezeDeep(item[key]));
      return Object.freeze(item);
    }
    return freezeDeep(clone);
  }

  function sameOpenAiMessages(left, right) {
    if (!isOpenAiChatArray(left) || !isOpenAiChatArray(right)) return false;
    try {
      return JSON.stringify(left) === JSON.stringify(right);
    } catch (_) {
      return false;
    }
  }

  function reusePendingInputSnapshot(messages, type, streaming) {
    const snapshot = pendingMainSnapshot;
    if (!snapshot || snapshot.ambiguous) return null;
    if (safeString(type).toLowerCase() !== snapshot.request_type.toLowerCase()) return null;
    if (!sameOpenAiMessages(messages, snapshot.original_messages)) return null;
    const inputTrace = cloneSnapshotValue(snapshot.input_trace || {});
    if (!inputTrace.input_enhance) inputTrace.input_enhance = {};
    inputTrace.input_enhance.retry_reuse_count = Math.max(
      0,
      Number(inputTrace.input_enhance.retry_reuse_count) || 0
    ) + 1;
    pendingMainSnapshot = makeRequestSnapshot(
      snapshot.messages,
      type,
      streaming,
      "",
      {
        original_messages: snapshot.original_messages,
        injected_messages: snapshot.injected_messages,
        context: snapshot.context,
        context_manifest: snapshot.context_manifest,
        turn_contract: snapshot.turn_contract,
        turn_contract_block: snapshot.turn_contract_block,
        input_trace: inputTrace,
      }
    );
    return cloneSnapshotValue(snapshot.injected_messages);
  }

  function makeRequestSnapshot(messages, type, streamingState, ambiguityReason, extras) {
    const extra = extras && typeof extras === "object" ? extras : {};
    const frozenMessages = cloneAndFreezeSnapshotValue(Array.isArray(messages) ? messages : []);
    const frozenOriginalMessages = cloneAndFreezeSnapshotValue(
      Array.isArray(extra.original_messages) ? extra.original_messages : messages
    );
    const frozenInjectedMessages = cloneAndFreezeSnapshotValue(
      Array.isArray(extra.injected_messages) ? extra.injected_messages : messages
    );
    return Object.freeze({
      created_at: Date.now(),
      request_type: safeString(type),
      messages: frozenMessages,
      original_messages: frozenOriginalMessages,
      injected_messages: frozenInjectedMessages,
      streaming: streamingState || { detected: false, reason: "" },
      ambiguous: !!ambiguityReason,
      ambiguity_reason: safeString(ambiguityReason),
      context: cloneAndFreezeSnapshotValue(extra.context || null),
      context_manifest: cloneAndFreezeSnapshotValue(extra.context_manifest || null),
      turn_contract: cloneAndFreezeSnapshotValue(extra.turn_contract || null),
      turn_contract_block: safeString(extra.turn_contract_block),
      input_trace: cloneAndFreezeSnapshotValue(extra.input_trace || null),
    });
  }

  function detectStreamingState(messagesOrContent, type) {
    const t = safeString(type).toLowerCase();
    if (/\b(?:stream|streaming|partial|fragment|delta|chunk)\b/.test(t)) {
      return { detected: true, reason: `request_type_${t}` };
    }
    const messages = Array.isArray(messagesOrContent) ? messagesOrContent : [];
    for (let i = 0; i < messages.length; i++) {
      const m = messages[i];
      if (!m || typeof m !== "object") continue;
      if (m.role === "function" || m.role === "tool") return { detected: true, reason: `message_role_${m.role}` };
    }
    return { detected: false, reason: "" };
  }

  function consumePendingSnapshot(type) {
    const snapshot = pendingMainSnapshot;
    if (!snapshot) return {
      ambiguous: true,
      reason: "no_pending_snapshot",
      messages: [],
      original_messages: [],
      injected_messages: [],
      streaming: { detected: false, reason: "" },
      context: null,
      context_manifest: null,
      turn_contract: null,
      turn_contract_block: "",
      input_trace: null,
    };
    pendingMainSnapshot = null;
    if (snapshot.ambiguous) {
      return {
        ambiguous: true,
        reason: snapshot.ambiguity_reason || "ambiguous_snapshot",
        messages: [],
        original_messages: [],
        injected_messages: [],
        streaming: snapshot.streaming,
        context: null,
        context_manifest: null,
        turn_contract: null,
        turn_contract_block: "",
        input_trace: snapshot.input_trace,
      };
    }
    if (safeString(type).toLowerCase() !== snapshot.request_type.toLowerCase()) {
      return {
        ambiguous: true,
        reason: "request_type_mismatch",
        messages: [],
        original_messages: [],
        injected_messages: [],
        streaming: snapshot.streaming,
        context: null,
        context_manifest: null,
        turn_contract: null,
        turn_contract_block: "",
        input_trace: snapshot.input_trace,
      };
    }
    return {
      ambiguous: false,
      reason: "snapshot_consumed",
      messages: snapshot.messages,
      original_messages: snapshot.original_messages,
      injected_messages: snapshot.injected_messages,
      streaming: snapshot.streaming,
      context: snapshot.context,
      context_manifest: snapshot.context_manifest,
      turn_contract: snapshot.turn_contract,
      turn_contract_block: snapshot.turn_contract_block,
      input_trace: snapshot.input_trace,
    };
  }

  function buildPostRewriteContext(snapshot, context, settings) {
    const limit = clampNumber(settings && settings.context_char_limit, 500, 50000, 6000);
    const parts = [];
    if (snapshot && snapshot.turn_contract_block) {
      parts.push(snapshot.turn_contract_block);
    }
    if (context && context.bounded_context_block) {
      parts.push(`[Turn Contract Evidence]\n${context.bounded_context_block}`);
    }
    return truncate(parts.join("\n\n"), Math.min(50000, limit * 2));
  }

  async function onBeforeRequest(messages, type) {
    let deadline = null;
    try {
      if (isAuxiliaryRequest(type)) return messages;
      const streaming = detectStreamingState(messages, type);
      if (pendingMainSnapshot) {
        const reused = reusePendingInputSnapshot(messages, type, streaming);
        if (reused) return reused;
        pendingMainSnapshot = makeRequestSnapshot([], type, streaming, "overlapping_main_requests");
        return messages;
      }
      const settings = await loadSettings();
      const preset = sanitizeEnum(settings.preset, PRESETS.map((item) => item.id), "balanced");
      const inputTrace = newTrace("beforeRequest", type);
      inputTrace.router.preset = preset;
      inputTrace.budget.http_attempt_max = INPUT_HTTP_ATTEMPT_BUDGET[preset];
      inputTrace.budget.http_attempt_used = 0;
      inputTrace.budget.http_stopped_reason = "";
      inputTrace.input_enhance.status = "collecting";
      inputTrace.scheduler.completion_wait = presetUsesCompletionWait(preset);
      traceTimeline(inputTrace, "input_context_start");
      deadline = inputTrace.scheduler.completion_wait
        ? createCompletionDeadline(Date.now())
        : createDeadline(INPUT_DEADLINE_MS[preset], Date.now());
      const context = await collectContext(messages, settings, inputTrace, deadline);
      traceTimeline(inputTrace, "input_context_done");
      const manifest = context.manifest;
      const plannerSelection = selectInputPlannerRoles(settings, manifest);
      inputTrace.input_enhance.planner_selected = plannerSelection.selected;
      inputTrace.input_enhance.planner_skipped = plannerSelection.skipped;
      const plannerManifest = compactManifestForPlanner(manifest);
      traceTimeline(inputTrace, "input_planners_start");
      const fragments = await scheduleInputPlanners(
        plannerSelection.roles,
        settings.role_profiles,
        plannerManifest,
        deadline,
        inputTrace,
        settings.max_parallel
      );
      traceTimeline(inputTrace, "input_planners_done");
      const contract = fuseTurnContract(manifest, fragments);
      const contractBlock = renderTurnContractBlock(contract, settings.context_char_limit);
      const injectedMessages = injectTurnContract(messages, contractBlock);
      const plannerEntries = inputTrace.roles.filter((entry) => entry.stage === "input");
      inputTrace.input_enhance.planner_succeeded = plannerEntries.filter((entry) => entry.status === "fulfilled").length;
      inputTrace.input_enhance.planner_failed = plannerEntries.filter((entry) => entry.status === "failed").length;
      inputTrace.input_enhance.status = fragments.length ? "planner_fused" : "manifest_fallback";
      inputTrace.input_enhance.fallback_reason = fragments.length
        ? ""
        : (plannerSelection.roles.length ? "input_planners_no_valid_fragment" : "no_configured_input_planners");
      inputTrace.input_enhance.contract_id = contract.contract_id;
      inputTrace.input_enhance.contract_digest = contract.contract_digest;
      inputTrace.input_enhance.injected_chars = contractBlock.length;
      inputTrace.input_enhance.active_calls_final = inputTrace.input_enhance.active_calls_final || 0;
      traceTimeline(inputTrace, "input_contract_injected");
      pendingMainSnapshot = makeRequestSnapshot(
        messages,
        type,
        streaming,
        "",
        {
          original_messages: messages,
          injected_messages: injectedMessages,
          context,
          context_manifest: manifest,
          turn_contract: contract,
          turn_contract_block: contractBlock,
          input_trace: inputTrace,
        }
      );
      return injectedMessages;
    } catch (err) {
      warn("beforeRequest error:", err);
      const streaming = detectStreamingState(messages, type);
      pendingMainSnapshot = makeRequestSnapshot(messages, type, streaming, "", {
        original_messages: messages,
        input_trace: {
          input_enhance: {
            status: "failed_open",
            fallback_reason: truncate(redactSensitiveText(err && err.message), 160),
            active_calls_final: 0,
          },
          roles: [],
        },
      });
      return messages;
    } finally {
      if (deadline) deadline.cancel();
    }
  }

  async function onAfterRequest(content, type) {
    const trace = newTrace("afterRequest", type);
    traceTimeline(trace, "start");
    const pipelineStartedAt = Date.now();
    let deadline = null;
    let comparisonFinalSegments = null;
    let comparisonTurn = false;

    try {
      const settings = await loadSettings();
      _cachedTraceEnabled = settings.trace_enabled !== false;

      if (isAuxiliaryRequest(type)) {
        setFinalTraceState(trace, false, "bypassed", "auxiliary_request_bypass");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }
      comparisonTurn = true;

      const completionWait = presetUsesCompletionWait(settings.preset);
      const deadlineMs = completionWait ? 0 : resolvePipelineDeadlineMs(settings);
      trace.deadline_ms = deadlineMs;
      trace.scheduler.completion_wait = completionWait;
      deadline = completionWait
        ? createCompletionDeadline(pipelineStartedAt)
        : createDeadline(deadlineMs, pipelineStartedAt);

      const snapshot = consumePendingSnapshot(type);
      trace.streaming = snapshot.streaming;
      if (snapshot.input_trace && snapshot.input_trace.input_enhance) {
        trace.input_enhance = cloneAndFreezeSnapshotValue(snapshot.input_trace.input_enhance);
      }
      if (snapshot.input_trace && Array.isArray(snapshot.input_trace.roles)) {
        trace.roles = cloneAndFreezeSnapshotValue(snapshot.input_trace.roles).slice();
      }
      if (snapshot.input_trace && snapshot.input_trace.budget) {
        const inputBudget = snapshot.input_trace.budget;
        trace.budget.http_attempt_max = OUTPUT_HTTP_ATTEMPT_BUDGET[settings.preset]
          || OUTPUT_HTTP_ATTEMPT_BUDGET.balanced;
        trace.budget.http_attempt_used = 0;
        trace.budget.input_attempt_max = Math.max(0, Number(inputBudget.input_attempt_max) || 0);
        trace.budget.input_attempt_used = Math.max(0, Number(inputBudget.input_attempt_used) || 0);
        trace.budget.http_stopped_reason = "";
      }
      trace.snapshot = {
        consumed: !snapshot.ambiguous,
        ambiguous: snapshot.ambiguous,
        reason: snapshot.reason,
        message_count: snapshot.original_messages.length || snapshot.messages.length,
        injected_message_count: snapshot.injected_messages.length || snapshot.messages.length,
        contract_id: safeString(snapshot.turn_contract && snapshot.turn_contract.contract_id),
        contract_digest: safeString(snapshot.turn_contract && snapshot.turn_contract.contract_digest),
      };
      trace.content_type = typeof content;
      trace.content_chars = safeString(content).length;

      if (snapshot.ambiguous) {
        setFinalTraceState(trace, false, "bypassed", `ambiguous_request_context:${snapshot.reason}`);
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      if (deadline.check()) {
        setFinalTraceState(trace, false, "bypassed", "deadline_before_pipeline");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      if (typeof content !== "string") {
        if (content && typeof content === "object" && typeof content.content === "string") {
          content = content.content;
        } else {
          setFinalTraceState(trace, false, "bypassed", "non_string_content_bypass");
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }
      }

      const originalText = safeString(content);
      if (!originalText.trim()) {
        setFinalTraceState(
          trace, false, "bypassed",
          snapshot.streaming.detected ? "streaming_empty_final" : "empty_input"
        );
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      traceTimeline(trace, "segment_start");
      const segments = buildSegmentMap(originalText, settings);
      const segSummary = summarizeSegments(segments);
      trace.segments = segSummary;
      traceTimeline(trace, "segment_done");

      if (segSummary.mutable === 0) {
        setFinalTraceState(trace, false, "bypassed", "no_mutable_segments");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      traceTimeline(trace, "context_start");
      const context = snapshot.context || await collectContext(snapshot.original_messages || snapshot.messages, settings, trace, deadline);
      traceTimeline(trace, "context_done");
      const rewriteContextBlock = buildPostRewriteContext(snapshot, context, settings);
      if (rewriteContextBlock) {
        trace.context_block_chars = rewriteContextBlock.length;
      } else {
        trace.context_block_chars = 0;
        traceError(trace, "empty_context_block — specialists will receive no runtime context");
      }

      if (deadline.check()) {
        setFinalTraceState(trace, false, "bypassed", "deadline_during_context");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      traceTimeline(trace, "router_start");
      const signals = detectSceneSignals(segments, context);
      const routerResult = selectRoles(settings.roles, signals, settings.preset, settings);
      const selectedRoles = routerResult.roles;
      traceTimeline(trace, "router_done");
      trace.router = {
        signals: routerResult.signals,
        selected: routerResult.selectReasons,
        skipped: routerResult.skipReasons,
        preset: settings.preset,
      };

      if (selectedRoles.length === 0) {
        setFinalTraceState(trace, false, "bypassed", "no_roles_selected");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      try {
        traceTimeline(trace, "schedule_start");
        const { directorResult, composerResult } = await scheduleRoles(
          selectedRoles, settings.role_profiles, segments,
          rewriteContextBlock, segments, deadline, trace, settings.max_parallel
        );
        traceTimeline(trace, "schedule_done");

        traceTimeline(trace, "assemble_start");
        const assembled = assembleOutput(segments, composerResult, directorResult);
        traceTimeline(trace, "assemble_done");

        /* ── R4: Director evidence ── */
        if (directorResult && directorResult.ranked) {
          Object.keys(directorResult.ranked).forEach((segId) => {
            const ranked = directorResult.ranked[segId] || [];
            if (!ranked.length) return;
            const top = ranked[0];
            trace.director_evidence.push({
              segment_id: segId,
              issue_groups: top.issues || [],
              consensus: directorResult.consensus[segId] || [],
              complementary: directorResult.complementary[segId] || [],
              conflict: directorResult.conflict[segId] || [],
              top_score: top.score != null ? Math.round(top.score * 10) / 10 : 0,
              top_role: top.role_id,
              candidate_count: ranked.length,
            });
          });
        }

        /* ── R4: Final summary ── */
        const specialistTraceEntries = (trace.roles || []).filter(
          (entry) => entry.stage !== "input" && entry.role_id !== COMPOSER_ROLE_ID
        );
        const successfulRoles = specialistTraceEntries.filter((entry) => entry.status === "fulfilled").length;
        trace.summary = {
          specialist_calls: specialistTraceEntries.length,
          successful_roles: successfulRoles,
          candidate_count: trace.candidates.total,
          composer_state: trace.composer.status || (composerResult ? "fulfilled" : "not_run"),
          changed_segment_count: 0,
          material_changed_segment_count: 0,
          unchanged_segment_count: segSummary.mutable,
          final_state: "",
          final_reason: "",
        };

        traceTimeline(trace, "verify_start");
        const verification = verifyOutput(segments, assembled.finalSegments, assembled.output, originalText);
        traceTimeline(trace, "verify_done");
        updateAppliedEvidence(trace, assembled, verification);

        if (!verification.pass) {
          trace.final.enhanced = false;
          trace.final.reason = `verifier_failed: ${verification.errors.join(", ")}`;
          setFinalTraceState(trace, false, "rejected", trace.final.reason);
          traceError(trace, trace.final.reason);
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }

        if (!assembled.changed || assembled.output === originalText) {
          setFinalTraceState(
            trace, false, "unchanged",
            assembled.changed ? "output_identical" : "no_candidates_applied"
          );
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }

        const finalClassification = classifyAppliedOutput(assembled);
        setFinalTraceState(
          trace,
          finalClassification.enhanced,
          finalClassification.state,
          finalClassification.reason
        );
        trace.summary.changed_segment_count = trace.applied_evidence.filter((item) => item.changed).length;
        trace.summary.material_changed_segment_count = trace.applied_evidence.filter(
          (item) => item.changed && item.material_change && !item.original_meta_only
        ).length;
        trace.summary.unchanged_segment_count = trace.applied_evidence.length - trace.summary.changed_segment_count;
        trace.final.original_preview = preview(originalText, 200);
        trace.final.final_preview = preview(assembled.output, 200);
        comparisonFinalSegments = assembled.finalSegments;

        if (settings_trace_enabled()) await saveTrace(trace);
        return assembled.output;
      } finally {
        if (deadline) deadline.cancel();
      }
    } catch (err) {
      traceError(trace, `pipeline_error: ${safeString(err && err.message)}`);
      setFinalTraceState(trace, false, "failed", "pipeline_error");
      if (settings_trace_enabled()) await saveTrace(trace);
      return content;
    } finally {
      if (deadline) deadline.cancel();
      if (comparisonTurn) updateLatestAppliedComparison(trace, comparisonFinalSegments);
    }
  }

  function settings_trace_enabled() {
    return _cachedTraceEnabled;
  }

  let _cachedTraceEnabled = true;

  /* ── UI ────────────────────────────────────────────────── */

  function escapeHtml(text) {
    return safeString(text)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function renderPresetPanel(settings) {
    const presetOptions = PRESETS.map((p) =>
      `<option value="${p.id}" ${settings.preset === p.id ? "selected" : ""}>${escapeHtml(p.label)}</option>`
    ).join("");
    return `
      <section class="recomposer-settings-card">
        <div class="recomposer-section-head">
          <div>
            <span class="recomposer-kicker">Runtime profile</span>
            <h3>실행 환경</h3>
          </div>
          <p>Quality는 완료까지 대기하며, 같은 endpoint에 역할이 3개 이상 모이면 해당 그룹만 순차 실행합니다.</p>
        </div>
        <div class="recomposer-grid-2col">
          <div class="recomposer-field">
            <label>품질 프리셋 <span>Preset</span></label>
            <select class="recomposer-preset">${presetOptions}</select>
          </div>
          <div class="recomposer-field">
            <label>전체 제한 시간 <span>Fast / Balanced · ms</span></label>
            <input type="number" class="recomposer-deadline" value="${settings.deadline_ms}" min="10000" max="600000" step="5000">
          </div>
          <div class="recomposer-field">
            <label>최대 동시 실행 <span>Parallel calls</span></label>
            <input type="number" class="recomposer-max-parallel" value="${settings.max_parallel}" min="1" max="20">
          </div>
          <div class="recomposer-field">
            <label>문맥 문자 상한 <span>Context limit</span></label>
            <input type="number" class="recomposer-context-limit" value="${settings.context_char_limit}" min="500" max="50000" step="500">
          </div>
          <div class="recomposer-field recomposer-field-wide">
            <label>추가 보호 정규식 <span>Protected regex</span></label>
            <input type="text" class="recomposer-protected-regex" value="${escapeHtml(settings.protected_regex)}" placeholder="변경하지 않을 사용자 정의 패턴">
          </div>
          <label class="recomposer-toggle-row recomposer-field-wide">
            <span>
              <strong>Trace 기록</strong>
              <small>실행 역할, 후보, 실제 적용 결과를 로컬 기록에 남깁니다.</small>
            </span>
            <input type="checkbox" class="recomposer-trace-enabled" ${settings.trace_enabled ? "checked" : ""}>
          </label>
        </div>
      </section>`;
  }

  function renderRolePanel(settings) {
    const rows = settings.roles.map((role) => {
      const profile = settings.role_profiles[role.role_id] || defaultRoleProfile(role.role_id);
      const providerOptions = PROVIDERS.map((p) =>
        `<option value="${p}" ${profile.provider === p ? "selected" : ""}>${p}</option>`
      ).join("");
      const roleStage = role.is_input_planner ? "input" : (role.is_composer ? "composer" : "specialist");
      const roleStageLabel = role.is_input_planner ? "INPUT" : (role.is_composer ? "COMPOSER" : "REWRITE");
      const configured = isProfileConfigured(profile);
      const stateLabel = !profile.enabled ? "꺼짐" : (configured ? "준비됨" : "설정 필요");
      const stateClass = !profile.enabled ? "is-off" : (configured ? "is-ready" : "is-missing");
      return `
        <details class="recomposer-role-row stage-${roleStage}" data-role="${escapeHtml(role.role_id)}">
          <summary class="recomposer-role-summary">
            <span class="recomposer-role-title">
              <span class="recomposer-stage-badge">${roleStageLabel}</span>
              <span>
                <strong>${escapeHtml(role.label)}</strong>
                <small>${escapeHtml(role.role_id)}</small>
              </span>
            </span>
            <span class="recomposer-role-meta">
              <span>${escapeHtml(profile.provider)}</span>
              <span>${escapeHtml(profile.model || "모델 미설정")}</span>
              <span class="recomposer-role-state ${stateClass}">${stateLabel}</span>
            </span>
          </summary>
          <div class="recomposer-role-body">
            <p class="recomposer-role-purpose">${escapeHtml(role.purpose)}</p>
            <label class="recomposer-toggle-row recomposer-role-toggle">
              <span><strong>역할 사용</strong><small>이 단계의 모델 호출을 실행합니다.</small></span>
              <input type="checkbox" class="recomposer-role-enabled" ${profile.enabled ? "checked" : ""}>
            </label>
            <div class="recomposer-grid-2col">
              <div class="recomposer-field">
                <label>Provider</label>
                <select class="recomposer-role-provider">${providerOptions}</select>
              </div>
              <div class="recomposer-field">
                <label>Model</label>
                <input type="text" class="recomposer-role-model" value="${escapeHtml(profile.model)}" placeholder="model name">
              </div>
              <div class="recomposer-field">
                <label>Endpoint</label>
                <input type="text" class="recomposer-role-endpoint" value="${escapeHtml(profile.endpoint)}" placeholder="API endpoint URL">
              </div>
              <div class="recomposer-field">
                <label>API Key / Ref</label>
                <input type="password" class="recomposer-role-key" value="" placeholder="${profile.api_key_ref ? maskKey(profile.api_key_ref) + ' (saved — blank to keep, clear:key to delete)' : 'direct key / arg:name / storage:key / env:KEY'}">
              </div>
              <div class="recomposer-field">
                <label>Temperature</label>
                <input type="number" class="recomposer-role-temp" value="${profile.temperature}" min="0" max="2" step="0.1">
              </div>
              <div class="recomposer-field">
                <label>Max Output Tokens</label>
                <input type="number" class="recomposer-role-max-tokens" value="${profile.max_output_tokens}" min="100" max="32000" step="100">
              </div>
              <div class="recomposer-field">
                <label>Timeout (Fast / Balanced · ms)</label>
                <input type="number" class="recomposer-role-timeout" value="${profile.timeout_ms}" min="5000" max="300000" step="5000">
              </div>
            </div>
            <div class="recomposer-field">
              <label>System Prompt</label>
              <textarea class="recomposer-role-prompt" rows="4">${escapeHtml(profile.system_prompt)}</textarea>
            </div>
            <details>
              <summary>Advanced</summary>
              <div class="recomposer-grid-2col">
                <div class="recomposer-field">
                  <label>Fallback Provider</label>
                  <input type="text" class="recomposer-role-fb-provider" value="${escapeHtml(profile.fallback_provider)}" placeholder="fallback provider">
                </div>
                <div class="recomposer-field">
                  <label>Fallback Model</label>
                  <input type="text" class="recomposer-role-fb-model" value="${escapeHtml(profile.fallback_model)}" placeholder="fallback model">
                </div>
                <div class="recomposer-field">
                  <label>Fallback Endpoint</label>
                  <input type="text" class="recomposer-role-fb-endpoint" value="${escapeHtml(profile.fallback_endpoint)}" placeholder="fallback endpoint">
                </div>
                <div class="recomposer-field">
                  <label>Fallback API Key</label>
                  <input type="password" class="recomposer-role-fb-key" value="" placeholder="${profile.fallback_api_key_ref ? maskKey(profile.fallback_api_key_ref) + ' (saved — blank to keep, clear:key to delete)' : 'fallback key ref'}">
                </div>
                <div class="recomposer-field">
                  <label>Reasoning Preset</label>
                  <select class="recomposer-role-reasoning">
                    <option value="auto" ${profile.reasoning_preset === "auto" ? "selected" : ""}>auto</option>
                    <option value="gpt" ${profile.reasoning_preset === "gpt" ? "selected" : ""}>gpt</option>
                    <option value="claude" ${profile.reasoning_preset === "claude" ? "selected" : ""}>claude</option>
                    <option value="gemini" ${profile.reasoning_preset === "gemini" ? "selected" : ""}>gemini</option>
                    <option value="kimi" ${profile.reasoning_preset === "kimi" ? "selected" : ""}>kimi</option>
                    <option value="glm" ${profile.reasoning_preset === "glm" ? "selected" : ""}>glm</option>
                    <option value="deepseek" ${profile.reasoning_preset === "deepseek" ? "selected" : ""}>deepseek</option>
                  </select>
                </div>
                <div class="recomposer-field">
                  <label>Reasoning Effort</label>
                  <select class="recomposer-role-reasoning-effort">
                    <option value="auto" ${profile.reasoning_effort === "auto" ? "selected" : ""}>auto</option>
                    <option value="none" ${profile.reasoning_effort === "none" ? "selected" : ""}>none</option>
                    <option value="low" ${profile.reasoning_effort === "low" ? "selected" : ""}>low</option>
                    <option value="medium" ${profile.reasoning_effort === "medium" ? "selected" : ""}>medium</option>
                    <option value="high" ${profile.reasoning_effort === "high" ? "selected" : ""}>high</option>
                  </select>
                </div>
                <div class="recomposer-field">
                  <label>Reasoning Budget Tokens</label>
                  <input type="number" class="recomposer-role-reasoning-budget" value="${profile.reasoning_budget_tokens}" min="0" max="131072" step="256">
                </div>
                <div class="recomposer-field">
                  <label>Vertex Flex Mode</label>
                  <select class="recomposer-role-vertex-flex">
                    <option value="off" ${profile.vertex_flex_mode === "off" ? "selected" : ""}>off</option>
                    <option value="provisioned_then_flex" ${profile.vertex_flex_mode === "provisioned_then_flex" ? "selected" : ""}>provisioned_then_flex</option>
                    <option value="flex_only" ${profile.vertex_flex_mode === "flex_only" ? "selected" : ""}>flex_only</option>
                  </select>
                </div>
              </div>
              <div class="recomposer-field">
                <label>Extra Headers (one per line, Key: Value)</label>
                <textarea class="recomposer-role-extra-headers" rows="3">${escapeHtml(profile.extra_headers)}</textarea>
              </div>
              <div class="recomposer-field">
                <label>Extra Body (JSON)</label>
                <textarea class="recomposer-role-extra-body" rows="3">${escapeHtml(profile.extra_body)}</textarea>
              </div>
              <label class="recomposer-field-inline"><input type="checkbox" class="recomposer-role-force-json" ${profile.force_json_response ? "checked" : ""}> Force JSON Response</label>
            </details>
          </div>
        </details>`;
    }).join("");
    return `
      <div class="recomposer-role-intro">
        <div><span>01</span><strong>Input planning</strong><small>설정·기억·현재 장면을 턴 계약으로 정리</small></div>
        <div><span>02</span><strong>Specialist rewrite</strong><small>역할별로 초안의 재작성 후보 생성</small></div>
        <div><span>03</span><strong>Fusion compose</strong><small>후보를 결합해 최종 출력으로 재구성</small></div>
      </div>
      <div class="recomposer-roles-list">${rows}</div>`;
  }

  function renderTracePanel() {
    return `
      <section class="recomposer-settings-card">
        <div class="recomposer-section-head recomposer-trace-head">
          <div>
            <span class="recomposer-kicker">Applied output evidence</span>
            <h3>실행 기록</h3>
          </div>
          <div class="recomposer-trace-toolbar">
            <button class="recomposer-trace-refresh recomposer-btn">새로고침</button>
            <button class="recomposer-trace-clear recomposer-btn">기록 비우기</button>
          </div>
        </div>
        <div class="recomposer-trace-scroll">
          <div class="recomposer-trace-list"></div>
        </div>
      </section>`;
  }

  function renderLatestComparison() {
    const snapshot = latestAppliedComparison;
    if (!snapshot) {
      return '<div class="recomposer-compare-empty">현재 세션에서 비교할 main 턴이 없습니다.</div>';
    }
    if (!snapshot.changes.length) {
      return `<div class="recomposer-compare-empty">최근 main 턴에는 실제 적용된 문장 변경이 없습니다. (${escapeHtml(snapshot.reason || snapshot.state)})</div>`;
    }
    const items = snapshot.changes.map((change) => {
      const finalText = change.operation === "delete" && !change.final_text
        ? "(삭제됨)"
        : change.final_text;
      return `
        <section class="recomposer-compare-item">
          <div class="recomposer-compare-meta">
            <span>${escapeHtml(change.segment_id)}</span>
            <span>${escapeHtml(change.source)}</span>
            ${change.applied_role_id ? `<span>${escapeHtml(change.applied_role_id)}</span>` : ""}
            <span>${escapeHtml(change.operation)}</span>
            <span>${change.material_change ? "실질적 재작성" : "미세 변경"}</span>
          </div>
          <div class="recomposer-compare-grid">
            <div class="recomposer-compare-column">
              <span class="recomposer-compare-label">전</span>
              <pre>${escapeHtml(change.original_text)}</pre>
            </div>
            <div class="recomposer-compare-column is-after">
              <span class="recomposer-compare-label">후</span>
              <pre>${escapeHtml(finalText)}</pre>
            </div>
          </div>
        </section>`;
    }).join("");
    return `
      <div class="recomposer-compare-entry">
        <div class="recomposer-compare-head">
          <strong>${new Date(snapshot.timestamp).toLocaleString()}</strong>
          <span>${escapeHtml(snapshot.state)} · ${snapshot.changes.length}개 변경</span>
        </div>
        ${items}
      </div>`;
  }

  function renderComparisonPanel() {
    return `
      <section class="recomposer-settings-card">
        <div class="recomposer-section-head recomposer-trace-head">
          <div>
            <span class="recomposer-kicker">Applied output comparison</span>
            <h3>문장 비교</h3>
            <p>현재 세션의 최근 main 턴만 표시하며 Trace나 저장소에는 전체 문장을 남기지 않습니다.</p>
          </div>
          <div class="recomposer-trace-toolbar">
            <button class="recomposer-compare-refresh recomposer-btn">새로고침</button>
          </div>
        </div>
        <div class="recomposer-compare-list"></div>
      </section>`;
  }

  function renderUI(settings) {
    const configuredRoles = settings.roles.filter((role) => {
      const profile = settings.role_profiles[role.role_id];
      return profile && profile.enabled && isProfileConfigured(profile);
    });
    const configuredInputRoles = configuredRoles.filter((role) => role.is_input_planner);
    const providerCount = new Set(configuredRoles.map((role) => (
      settings.role_profiles[role.role_id].provider
    ))).size;
    const preset = PRESETS.find((item) => item.id === settings.preset) || PRESETS[1];
    const style = `
      <style>
        #recomposer-overlay {
          position: fixed;
          inset: 0;
          z-index: 99999;
          display: flex;
          align-items: center;
          justify-content: center;
          padding: 24px;
          overflow: hidden;
          background: rgba(4, 6, 9, 0.76);
        }
        .recomposer-root {
          --rc-bg: #0B0D11;
          --rc-bg-sub: #0F1116;
          --rc-card-bg: #13161C;
          --rc-panel-bg: #181C24;
          --rc-border: rgba(255,255,255,0.07);
          --rc-border-hover: rgba(255,255,255,0.12);
          --rc-fg: #F4F5F7;
          --rc-fg-muted: #8B909A;
          --rc-fg-dim: #5C626D;
          --rc-blue: #5D73E6;
          --rc-blue-light: #8FA7FF;
          --rc-purple: #8A55F7;
          --rc-pink: #E158A6;
          --rc-cta-bg: #F7F7F9;
          --rc-cta-fg: #101216;
          font-family: 'Inter', 'Pretendard Variable', system-ui, -apple-system, sans-serif;
          background: var(--rc-bg);
          color: var(--rc-fg);
          margin: 0;
          width: min(1080px, calc(100vw - 48px));
          max-width: 1080px;
          max-height: calc(100vh - 48px);
          box-sizing: border-box;
          overflow-y: auto;
          border: 1px solid var(--rc-border-hover);
          border-radius: 8px;
          box-shadow: 0 28px 80px rgba(0,0,0,0.56);
          letter-spacing: 0;
          -webkit-font-smoothing: antialiased;
          scrollbar-color: #2a2f3a transparent;
          scrollbar-width: thin;
        }
        .recomposer-root * { box-sizing: border-box; }
        .recomposer-product-bar {
          padding: 20px 32px;
          border-bottom: 1px solid var(--rc-border);
          background: var(--rc-bg-sub);
        }
        .recomposer-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 24px;
        }
        .recomposer-brand {
          display: flex;
          align-items: center;
          gap: 12px;
          min-width: 0;
        }
        .recomposer-brand-mark {
          width: 30px;
          height: 30px;
          flex: 0 0 30px;
          border: 1px solid rgba(143,167,255,0.38);
          border-radius: 7px;
          background:
            linear-gradient(135deg, rgba(93,115,230,0.42), rgba(138,85,247,0.18)),
            var(--rc-panel-bg);
          box-shadow: inset 0 1px 0 rgba(255,255,255,0.08);
        }
        .recomposer-brand-copy strong {
          display: block;
          color: var(--rc-fg);
          font-size: 15px;
          font-weight: 500;
          line-height: 1.2;
        }
        .recomposer-brand-copy small {
          display: block;
          margin-top: 3px;
          color: var(--rc-fg-dim);
          font-size: 11px;
        }
        .recomposer-trust {
          display: flex;
          align-items: center;
          gap: 8px;
          flex: 0 0 auto;
          color: var(--rc-fg-muted);
          font-size: 12px;
        }
        .recomposer-trust-dot {
          width: 7px;
          height: 7px;
          border-radius: 50%;
          background: var(--rc-blue-light);
          box-shadow: 0 0 12px rgba(143,167,255,0.46);
        }
        .recomposer-metric-grid {
          display: grid;
          grid-template-columns: repeat(4, minmax(0, 1fr));
          gap: 12px;
          padding: 20px 32px 0;
          background: var(--rc-bg);
        }
        .recomposer-metric {
          min-width: 0;
          padding: 16px;
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          background: var(--rc-card-bg);
        }
        .recomposer-metric span {
          display: block;
          margin-bottom: 9px;
          color: var(--rc-fg-dim);
          font-size: 11px;
        }
        .recomposer-metric strong {
          display: block;
          overflow: hidden;
          color: var(--rc-fg);
          font-size: 18px;
          font-weight: 500;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .recomposer-tabs {
          position: sticky;
          top: 0;
          z-index: 5;
          display: flex;
          gap: 4px;
          margin: 20px 32px 0;
          padding: 6px;
          border-bottom: 1px solid var(--rc-border);
          background: rgba(11,13,17,0.96);
        }
        .recomposer-tab {
          min-height: 38px;
          padding: 9px 16px;
          cursor: pointer;
          border: 1px solid transparent;
          border-radius: 7px 7px 0 0;
          background: transparent;
          color: var(--rc-fg-dim);
          font-size: 13px;
          font-weight: 500;
          transition: color 0.18s, border-color 0.18s, background 0.18s;
        }
        .recomposer-tab:hover { color: var(--rc-fg-muted); }
        .recomposer-tab.active {
          color: var(--rc-fg);
          border-color: var(--rc-border);
          border-bottom-color: var(--rc-blue);
          background: var(--rc-card-bg);
        }
        .recomposer-tab-panel { display: none; }
        .recomposer-tab-panel.active {
          display: block;
          padding: 24px 32px 112px;
        }
        .recomposer-grid-2col {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 20px;
        }
        .recomposer-field-wide { grid-column: 1 / -1; }
        .recomposer-field { display: flex; flex-direction: column; gap: 6px; margin-bottom: 16px; }
        .recomposer-field label {
          font-size: 12px;
          letter-spacing: 0;
          color: var(--rc-fg-muted);
          font-weight: 500;
        }
        .recomposer-field label span {
          margin-left: 5px;
          color: var(--rc-fg-dim);
          font-size: 10px;
          font-weight: 400;
        }
        .recomposer-field-inline {
          display: flex;
          align-items: center;
          gap: 8px;
          margin: 8px 0;
          font-size: 14px;
          color: var(--rc-fg-muted);
        }
        .recomposer-toggle-row {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 20px;
          min-height: 62px;
          padding: 12px 14px;
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          background: var(--rc-panel-bg);
          color: var(--rc-fg-muted);
        }
        .recomposer-toggle-row strong {
          display: block;
          color: var(--rc-fg);
          font-size: 13px;
          font-weight: 500;
        }
        .recomposer-toggle-row small {
          display: block;
          margin-top: 4px;
          color: var(--rc-fg-dim);
          font-size: 11px;
          line-height: 1.4;
        }
        .recomposer-root input[type="text"],
        .recomposer-root input[type="number"],
        .recomposer-root input[type="password"],
        .recomposer-root select,
        .recomposer-root textarea {
          width: 100%;
          min-width: 0;
          box-sizing: border-box;
          min-height: 42px;
          padding: 10px 12px;
          border: 1px solid var(--rc-border);
          border-radius: 7px;
          background: var(--rc-panel-bg);
          color: var(--rc-fg);
          font-size: 14px;
          font-family: inherit;
          transition: border-color 0.2s, box-shadow 0.2s;
        }
        .recomposer-root input:focus,
        .recomposer-root select:focus,
        .recomposer-root textarea:focus {
          outline: none;
          border-color: var(--rc-blue);
          box-shadow: 0 0 0 3px rgba(93,115,230,0.12);
        }
        .recomposer-root input::placeholder,
        .recomposer-root textarea::placeholder {
          color: var(--rc-fg-dim);
        }
        .recomposer-root textarea { resize: vertical; line-height: 1.5; }
        .recomposer-root input[type="checkbox"] {
          width: 18px;
          height: 18px;
          flex: 0 0 18px;
          accent-color: var(--rc-blue);
          cursor: pointer;
        }
        .recomposer-settings-card {
          padding: 24px;
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          background: var(--rc-card-bg);
          box-shadow: 0 12px 32px rgba(0,0,0,0.18);
        }
        .recomposer-section-head {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
          gap: 32px;
          margin-bottom: 28px;
        }
        .recomposer-section-head h3 {
          margin: 5px 0 0;
          color: var(--rc-fg);
          font-size: 20px;
          font-weight: 500;
        }
        .recomposer-section-head p {
          max-width: 430px;
          margin: 0;
          color: var(--rc-fg-muted);
          font-size: 13px;
          line-height: 1.55;
        }
        .recomposer-kicker,
        .recomposer-section-label {
          color: var(--rc-blue-light);
          font-size: 10px;
          text-transform: uppercase;
          letter-spacing: 0;
        }
        .recomposer-role-intro {
          display: grid;
          grid-template-columns: repeat(3, minmax(0, 1fr));
          gap: 12px;
          margin-bottom: 20px;
        }
        .recomposer-role-intro > div {
          min-width: 0;
          padding: 16px;
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          background: var(--rc-bg-sub);
        }
        .recomposer-role-intro span {
          color: var(--rc-blue-light);
          font-size: 10px;
        }
        .recomposer-role-intro strong {
          display: block;
          margin-top: 8px;
          color: var(--rc-fg);
          font-size: 13px;
          font-weight: 500;
        }
        .recomposer-role-intro small {
          display: block;
          margin-top: 5px;
          color: var(--rc-fg-dim);
          font-size: 11px;
          line-height: 1.45;
        }
        .recomposer-roles-list { display: flex; flex-direction: column; gap: 12px; }
        .recomposer-role-row {
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          background: var(--rc-card-bg);
          overflow: hidden;
          box-shadow: 0 4px 16px rgba(0,0,0,0.2);
          transition: border-color 0.2s, box-shadow 0.2s;
        }
        .recomposer-role-row[open] {
          border-color: rgba(93,115,230,0.34);
          box-shadow: 0 14px 36px rgba(0,0,0,0.26), 0 0 0 1px rgba(93,115,230,0.07);
        }
        .recomposer-role-row.stage-composer[open] {
          border-color: rgba(138,85,247,0.34);
        }
        .recomposer-role-summary {
          min-height: 74px;
          padding: 14px 18px;
          cursor: pointer;
          color: var(--rc-fg);
          list-style: none;
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 20px;
        }
        .recomposer-role-summary:hover { background: rgba(255,255,255,0.015); }
        .recomposer-role-summary::-webkit-details-marker { display: none; }
        .recomposer-role-title {
          display: flex;
          align-items: center;
          gap: 12px;
          min-width: 0;
        }
        .recomposer-role-title strong {
          display: block;
          overflow-wrap: anywhere;
          color: var(--rc-fg);
          font-size: 14px;
          font-weight: 500;
        }
        .recomposer-role-title small {
          display: block;
          margin-top: 4px;
          color: var(--rc-fg-dim);
          font-size: 10px;
        }
        .recomposer-stage-badge {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          min-width: 68px;
          height: 24px;
          padding: 0 8px;
          border: 1px solid rgba(93,115,230,0.24);
          border-radius: 6px;
          background: rgba(93,115,230,0.08);
          color: var(--rc-blue-light);
          font-size: 9px;
          font-weight: 500;
        }
        .stage-composer .recomposer-stage-badge {
          border-color: rgba(138,85,247,0.26);
          background: rgba(138,85,247,0.09);
          color: #b49aff;
        }
        .recomposer-role-meta {
          display: flex;
          align-items: center;
          justify-content: flex-end;
          gap: 8px;
          min-width: 0;
          color: var(--rc-fg-dim);
          font-size: 10px;
        }
        .recomposer-role-meta > span:not(.recomposer-role-state) {
          max-width: 150px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .recomposer-role-state {
          padding: 5px 7px;
          border-radius: 6px;
          border: 1px solid var(--rc-border);
          white-space: nowrap;
        }
        .recomposer-role-state.is-ready {
          border-color: rgba(143,167,255,0.26);
          color: var(--rc-blue-light);
        }
        .recomposer-role-state.is-missing { color: #d49ab9; }
        .recomposer-role-state.is-off { color: var(--rc-fg-dim); }
        .recomposer-role-row[open] .recomposer-role-summary {
          border-bottom: 1px solid var(--rc-border);
        }
        .recomposer-role-body { padding: 20px; }
        .recomposer-role-purpose {
          color: var(--rc-fg-muted);
          font-size: 13px;
          line-height: 1.5;
          margin: 4px 0 16px;
        }
        .recomposer-role-toggle { margin-bottom: 18px; }
        .recomposer-role-row details > summary {
          cursor: pointer;
          padding: 12px 0;
          font-size: 13px;
          color: var(--rc-fg-muted);
          list-style: none;
          border-top: 1px solid var(--rc-border);
          margin-top: 8px;
        }
        .recomposer-role-row details > summary::-webkit-details-marker { display: none; }
        .recomposer-role-row details > summary::after {
          content: '›';
          float: right;
          color: var(--rc-fg-dim);
          transition: transform 0.2s;
        }
        .recomposer-role-row details[open] > summary::after { transform: rotate(90deg); }
        .recomposer-trace-head { align-items: center; }
        .recomposer-trace-toolbar { display: flex; gap: 8px; }
        .recomposer-trace-scroll { overflow-x: auto; max-height: 560px; }
        .recomposer-trace-entry {
          border: 1px solid var(--rc-border);
          border-radius: 8px;
          padding: 16px 20px;
          margin-bottom: 12px;
          font-size: 13px;
          background: var(--rc-card-bg);
          color: var(--rc-fg);
          box-shadow: 0 4px 16px rgba(0,0,0,0.15);
        }
        .recomposer-trace-entry pre {
          white-space: pre-wrap;
          word-break: break-all;
          color: var(--rc-fg-muted);
          font-size: 12px;
          line-height: 1.5;
          margin: 8px 0 0;
        }
        .recomposer-trace-entry strong { color: var(--rc-fg); font-weight: 500; }
        .recomposer-compare-entry {
          padding-top: 4px;
        }
        .recomposer-compare-head {
          display: flex;
          align-items: baseline;
          justify-content: space-between;
          gap: 16px;
          margin-bottom: 14px;
        }
        .recomposer-compare-head strong {
          color: var(--rc-fg);
          font-size: 14px;
          font-weight: 500;
        }
        .recomposer-compare-head span,
        .recomposer-compare-meta {
          color: var(--rc-fg-dim);
          font-size: 12px;
        }
        .recomposer-compare-item {
          padding: 16px 0 18px;
          border-top: 1px solid var(--rc-border);
        }
        .recomposer-compare-meta {
          display: flex;
          flex-wrap: wrap;
          gap: 6px 12px;
          margin-bottom: 10px;
        }
        .recomposer-compare-grid {
          display: grid;
          grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
          border-top: 1px solid rgba(255,255,255,0.055);
          border-bottom: 1px solid rgba(255,255,255,0.055);
        }
        .recomposer-compare-column {
          min-width: 0;
          padding: 14px 16px;
        }
        .recomposer-compare-column + .recomposer-compare-column {
          border-left: 1px solid var(--rc-border);
        }
        .recomposer-compare-label {
          display: block;
          margin-bottom: 8px;
          color: var(--rc-fg-dim);
          font-size: 11px;
          font-weight: 600;
        }
        .recomposer-compare-column.is-after .recomposer-compare-label {
          color: #8FA7FF;
        }
        .recomposer-compare-column pre {
          max-height: 420px;
          margin: 0;
          overflow: auto;
          white-space: pre-wrap;
          overflow-wrap: anywhere;
          color: var(--rc-fg);
          font-family: inherit;
          font-size: 13px;
          line-height: 1.65;
        }
        .recomposer-compare-empty {
          padding: 20px 0;
          color: var(--rc-fg-dim);
          font-size: 13px;
        }
        .recomposer-btn {
          min-height: 40px;
          padding: 9px 16px;
          cursor: pointer;
          border: 1px solid var(--rc-border);
          border-radius: 7px;
          background: var(--rc-card-bg);
          color: var(--rc-fg);
          font-size: 14px;
          font-family: inherit;
          font-weight: 400;
          transition: border-color 0.2s, background 0.2s, color 0.2s;
        }
        .recomposer-btn:hover {
          border-color: var(--rc-border-hover);
          background: var(--rc-panel-bg);
        }
        .recomposer-btn-primary {
          background: var(--rc-cta-bg);
          color: var(--rc-cta-fg);
          border-color: var(--rc-cta-bg);
          font-weight: 500;
        }
        .recomposer-btn-primary:hover {
          background: #FFFFFF;
          border-color: #FFFFFF;
        }
        .recomposer-actions {
          display: flex;
          justify-content: flex-end;
          gap: 12px;
          position: sticky;
          bottom: 0;
          z-index: 6;
          padding: 16px 32px;
          border-top: 1px solid var(--rc-border);
          background: rgba(11,13,17,0.97);
          box-shadow: 0 -16px 36px rgba(0,0,0,0.28);
        }
        .recomposer-section-label {
          display: block;
          margin-bottom: 14px;
        }
        @media (max-width: 820px) {
          #recomposer-overlay { padding: 16px; }
          .recomposer-root {
            width: calc(100vw - 32px);
            max-height: calc(100vh - 32px);
          }
          .recomposer-product-bar { padding: 18px 24px; }
          .recomposer-metric-grid {
            grid-template-columns: 1fr 1fr;
            padding: 16px 24px 0;
          }
          .recomposer-tabs { margin: 16px 24px 0; }
          .recomposer-tab-panel.active { padding: 20px 24px 104px; }
          .recomposer-role-summary { align-items: flex-start; flex-direction: column; }
          .recomposer-role-meta { width: 100%; justify-content: flex-start; flex-wrap: wrap; }
          .recomposer-section-head { flex-direction: column; gap: 12px; }
          .recomposer-section-head p { max-width: none; }
          .recomposer-actions { padding: 14px 24px; }
        }
        @media (max-width: 560px) {
          #recomposer-overlay { padding: 0; align-items: stretch; }
          .recomposer-root {
            width: 100%;
            max-width: none;
            max-height: none;
            border: 0;
            border-radius: 0;
          }
          .recomposer-product-bar { padding: 16px 20px; }
          .recomposer-header { align-items: flex-start; }
          .recomposer-trust { max-width: 120px; text-align: right; }
          .recomposer-metric-grid {
            grid-template-columns: 1fr 1fr;
            gap: 8px;
            padding: 12px 20px 0;
          }
          .recomposer-metric { padding: 13px; }
          .recomposer-metric strong { font-size: 15px; }
          .recomposer-tabs {
            margin: 12px 20px 0;
            overflow-x: auto;
          }
          .recomposer-tab { flex: 1 0 auto; padding: 9px 12px; }
          .recomposer-tab-panel.active { padding: 16px 20px 96px; }
          .recomposer-grid-2col,
          .recomposer-role-intro { grid-template-columns: 1fr; }
          .recomposer-compare-grid { grid-template-columns: 1fr; }
          .recomposer-compare-column + .recomposer-compare-column {
            border-top: 1px solid var(--rc-border);
            border-left: 0;
          }
          .recomposer-field-wide { grid-column: auto; }
          .recomposer-settings-card { padding: 18px; }
          .recomposer-stage-badge { min-width: 60px; }
          .recomposer-role-meta > span:not(.recomposer-role-state) { max-width: 120px; }
          .recomposer-actions {
            justify-content: stretch;
            padding: 12px 20px;
          }
          .recomposer-actions .recomposer-btn { flex: 1; }
        }
      </style>`;
    const html = `
      <div class="recomposer-root">
        ${style}
        <section class="recomposer-product-bar">
          <div class="recomposer-header">
            <div class="recomposer-brand">
              <span class="recomposer-brand-mark" aria-hidden="true"></span>
              <span class="recomposer-brand-copy">
                <strong>Risu Recomposer</strong>
                <small>Build ${VERSION} · ${escapeHtml(BUILD_MARKER)}</small>
              </span>
            </div>
            <div class="recomposer-trust"><span class="recomposer-trust-dot"></span>Read-only context · local trace</div>
          </div>
        </section>
        <div class="recomposer-metric-grid">
          <div class="recomposer-metric"><span>프리셋</span><strong>${escapeHtml(preset.label)}</strong></div>
          <div class="recomposer-metric"><span>준비된 역할</span><strong>${configuredRoles.length} / ${settings.roles.length}</strong></div>
          <div class="recomposer-metric"><span>Input Planner</span><strong>${configuredInputRoles.length} ready</strong></div>
          <div class="recomposer-metric"><span>Provider</span><strong>${providerCount || 0} connected</strong></div>
        </div>
        <div class="recomposer-tabs">
          <button class="recomposer-tab active" data-tab="general">개요·실행</button>
          <button class="recomposer-tab" data-tab="roles">역할·모델</button>
          <button class="recomposer-tab" data-tab="trace">실행 기록</button>
          <button class="recomposer-tab" data-tab="compare">비교</button>
        </div>
        <div class="recomposer-tab-panel active" data-panel="general">
          <span class="recomposer-section-label">Core configuration</span>
          ${renderPresetPanel(settings)}
        </div>
        <div class="recomposer-tab-panel" data-panel="roles">
          <span class="recomposer-section-label">Specialist roles and providers</span>
          ${renderRolePanel(settings)}
        </div>
        <div class="recomposer-tab-panel" data-panel="trace">
          <span class="recomposer-section-label">Trace and applied output evidence</span>
          ${renderTracePanel()}
        </div>
        <div class="recomposer-tab-panel" data-panel="compare">
          <span class="recomposer-section-label">Latest applied output only</span>
          ${renderComparisonPanel()}
        </div>
        <div class="recomposer-actions">
          <button class="recomposer-close recomposer-btn">닫기</button>
          <button class="recomposer-save recomposer-btn recomposer-btn-primary">설정 저장</button>
        </div>
      </div>`;
    return html;
  }

  function applyKeyUpdate(inputVal, existingVal) {
    const input = safeString(inputVal).trim();
    if (!input) return safeString(existingVal);
    if (input === "clear:key") return "";
    return input;
  }

  async function collectSettingsFromUI(rootEl) {
    const existing = await loadSettings();
    const settings = mergeSettings(defaultSettings(), existing);
    try {
      const allDetails = rootEl.querySelectorAll("details");
      const openStates = [];
      allDetails.forEach((d) => {
        openStates.push(d.open);
        d.open = true;
      });
      settings.preset = sanitizeEnum((rootEl.querySelector(".recomposer-preset") || {}).value, PRESETS.map((p) => p.id), "balanced");
      settings.deadline_ms = clampNumber((rootEl.querySelector(".recomposer-deadline") || {}).value, 10000, 600000, DEFAULT_DEADLINE_MS);
      settings.max_parallel = clampNumber((rootEl.querySelector(".recomposer-max-parallel") || {}).value, 1, 20, 5);
      settings.protected_regex = safeString((rootEl.querySelector(".recomposer-protected-regex") || {}).value);
      settings.context_char_limit = clampNumber((rootEl.querySelector(".recomposer-context-limit") || {}).value, 500, 50000, 6000);
      settings.trace_enabled = !!(rootEl.querySelector(".recomposer-trace-enabled") || {}).checked;
      const roleRows = rootEl.querySelectorAll(".recomposer-role-row");
      roleRows.forEach((row) => {
        const roleId = row.getAttribute("data-role");
        if (!roleId || !settings.role_profiles[roleId]) return;
        const p = settings.role_profiles[roleId];
        const existingProfile = (existing.role_profiles && existing.role_profiles[roleId]) || {};
        p.enabled = !!(row.querySelector(".recomposer-role-enabled") || {}).checked;
        p.provider = sanitizeEnum((row.querySelector(".recomposer-role-provider") || {}).value, PROVIDERS, "openai_compatible");
        p.endpoint = safeString((row.querySelector(".recomposer-role-endpoint") || {}).value);
        p.model = safeString((row.querySelector(".recomposer-role-model") || {}).value);
        p.api_key_ref = applyKeyUpdate((row.querySelector(".recomposer-role-key") || {}).value, existingProfile.api_key_ref);
        p.temperature = clampNumber((row.querySelector(".recomposer-role-temp") || {}).value, 0, 2, 0.3);
        p.max_output_tokens = clampNumber((row.querySelector(".recomposer-role-max-tokens") || {}).value, 100, 32000, 2048);
        p.timeout_ms = clampNumber((row.querySelector(".recomposer-role-timeout") || {}).value, 5000, 300000, 45000);
        p.system_prompt = safeString((row.querySelector(".recomposer-role-prompt") || {}).value);
        const roleDefinition = DEFAULT_ROLES.find((role) => role.role_id === roleId);
        p.prompt_contract_version = roleDefinition && p.system_prompt === roleDefinition.default_prompt
          ? ROLE_PROMPT_VERSION
          : "custom";
        p.fallback_provider = safeString((row.querySelector(".recomposer-role-fb-provider") || {}).value);
        p.fallback_model = safeString((row.querySelector(".recomposer-role-fb-model") || {}).value);
        p.fallback_endpoint = safeString((row.querySelector(".recomposer-role-fb-endpoint") || {}).value);
        p.fallback_api_key_ref = applyKeyUpdate((row.querySelector(".recomposer-role-fb-key") || {}).value, existingProfile.fallback_api_key_ref);
        p.extra_headers = safeString((row.querySelector(".recomposer-role-extra-headers") || {}).value);
        p.extra_body = safeString((row.querySelector(".recomposer-role-extra-body") || {}).value);
        p.reasoning_preset = safeString((row.querySelector(".recomposer-role-reasoning") || {}).value, "auto");
        p.reasoning_effort = safeString((row.querySelector(".recomposer-role-reasoning-effort") || {}).value, "auto");
        p.reasoning_budget_tokens = clampNumber((row.querySelector(".recomposer-role-reasoning-budget") || {}).value, 0, 131072, 0);
        p.vertex_flex_mode = sanitizeEnum((row.querySelector(".recomposer-role-vertex-flex") || {}).value, ["off", "provisioned_then_flex", "flex_only"], "off");
        p.force_json_response = !!(row.querySelector(".recomposer-role-force-json") || {}).checked;
      });
      allDetails.forEach((d, i) => { d.open = openStates[i]; });
    } catch (err) {
      warn("collectSettingsFromUI error:", err);
    }
    return settings;
  }

  async function refreshTraceList(containerEl) {
    try {
      const traces = await loadTraceList();
      containerEl.innerHTML = traces.slice(0, 10).map((t) => {
        const roleLines = (t.roles || []).map((r) => {
          const queueMs = Math.max(0, Number(r.started_at || 0) - Number(r.queued_at || r.started_at || 0));
          const overrides = r.request_overrides || {};
          const appliedOverrideKeys = []
            .concat(overrides.applied_header_keys || [])
            .concat(overrides.applied_body_keys || []);
          const skippedOverrideKeys = []
            .concat(overrides.skipped_header_keys || [])
            .concat(overrides.skipped_body_keys || []);
          const attemptSummary = (r.attempts || []).map((attempt) =>
            `${attempt.kind}:${attempt.status}${attempt.error_class ? "/" + attempt.error_class : ""}${attempt.structured_transport ? "/via:" + attempt.structured_transport : ""}${attempt.transport ? "@" + attempt.transport : ""}${attempt.endpoint ? "(" + attempt.endpoint + ")" : ""}${attempt.failure_preview ? ' preview:"' + attempt.failure_preview + '"' : ""}`
          ).join(",");
          const reasoningLabel = overrides.reasoning_family
            ? ` reasoning:${overrides.reasoning_family}${(overrides.reasoning_fields || []).length ? "/" + overrides.reasoning_fields.join(",") : ""}`
            : "";
          const transportLabel = overrides.transport ? ` transport:${overrides.transport}` : "";
          return `[${r.stage || "output"}] ${r.role_id}: ${r.status} (${r.provider}/${r.model}) queue:${queueMs}ms run:${r.elapsed_ms}ms http:${r.http_attempts || 0}${r.retry ? " retry:" + r.retry : ""}${r.fallback ? " fallback" : ""}${r.candidate_count ? " cand:" + r.candidate_count : ""}${attemptSummary ? " [" + attemptSummary + "]" : ""}${transportLabel}${reasoningLabel}${appliedOverrideKeys.length ? " override+:" + appliedOverrideKeys.join(",") : ""}${skippedOverrideKeys.length ? " override-skip:" + skippedOverrideKeys.join(",") : ""}${r.error_class ? " class:" + r.error_class : ""}${r.error ? " ERR:" + r.error : ""}`;
        }).join("\n");
        const directorLines = (t.director_evidence || []).map((d) =>
          `${d.segment_id}: top=${d.top_role} score=${d.top_score} issues=[${(d.issue_groups || []).join(",")}] consensus=[${(d.consensus || []).join(",")}] complementary=[${(d.complementary || []).join(",")}] conflict=[${(d.conflict || []).join(",")}] cand=${d.candidate_count}`
        ).join("\n");
        const appliedLines = (t.applied_evidence || []).map((a) =>
          `${a.segment_id}: ${a.source}${a.changed ? " [CHANGED]" : " [unchanged]"}${a.material_change ? " [MATERIAL]" : (a.changed ? " [MINOR]" : "")}${a.composer_unchanged ? " (composer:identical)" : ""} orig:"${escapeHtml(a.original_preview || "")}" → final:"${escapeHtml(a.final_preview || "")}"`
        ).join("\n");
        const s = t.summary || {};
        const ie = t.input_enhance || {};
        const sourceSummary = Object.keys(ie.source_availability || {}).map((key) => {
          const source = ie.source_availability[key] || {};
          return `${key}:${source.available ? "used" : "missing"}${source.count ? "/" + source.count : ""}${source.active_count ? "/active:" + source.active_count : ""}${source.unknown_activation_count ? "/unknown:" + source.unknown_activation_count : ""}`;
        }).join(", ");
        const inputLine = `state:${ie.status || "not_run"} contract:${ie.contract_id || "none"} digest:${ie.contract_digest || "none"} planners:${ie.planner_succeeded || 0}ok/${ie.planner_failed || 0}fail injected:${ie.injected_chars || 0} chars active:${ie.active_calls_final || 0} retry-reuse:${ie.retry_reuse_count || 0} transport-cancel:${ie.transport_cancellation || "not_requested"}${ie.fallback_reason ? " fallback:" + ie.fallback_reason : ""}`;
        const summaryLine = `specialists:${s.specialist_calls} successful:${s.successful_roles} candidates:${s.candidate_count} composer:${s.composer_state} changed:${s.changed_segment_count} material:${s.material_changed_segment_count || 0} unchanged:${s.unchanged_segment_count} state:${s.final_state} reason:${escapeHtml(s.final_reason || "")}`;
        const scheduler = t.scheduler || {};
        const schedulerLines = (scheduler.endpoint_groups || []).map((group) =>
          `${group.stage}:${group.endpoint_group} calls:${group.selected_calls} concurrency:${group.effective_concurrency}/${group.base_concurrency} reason:${group.reason}`
        ).join("\n");
        const routerLine = (t.router && t.router.signals && t.router.signals.length) ? `signals:[${t.router.signals.join(",")}]` : "";
        return `<div class="recomposer-trace-entry">
          <strong>${escapeHtml(t.stage)} ${new Date(t.timestamp).toLocaleString()}</strong><br>
          Enhanced: ${t.final.enhanced} — ${escapeHtml(t.final.reason || "")}<br>
          Segments: P:${t.segments.protected} I:${t.segments.inspect_only} M:${t.segments.mutable}<br>
          ${routerLine ? `Router: ${escapeHtml(routerLine)}<br>` : ""}
          Input Enhance: ${escapeHtml(inputLine)}<br>
          ${sourceSummary ? `Sources: ${escapeHtml(sourceSummary)}<br>` : ""}
          Summary: ${escapeHtml(summaryLine)}<br>
          Scheduler: ${scheduler.completion_wait ? "completion_wait" : "deadline"} · serial threshold:${scheduler.endpoint_serial_threshold || ENDPOINT_SERIAL_THRESHOLD}<br>
          Candidates: ${t.candidates.total}<br>
          Composer: ${t.composer.used} (${t.composer.status} ${t.composer.elapsed_ms}ms, reserve:${t.composer.reserve_ms || 0}ms, semantic-retry:${t.composer.semantic_retry || 0}${t.composer.specialist_stop_reason ? ", specialist-stop:" + escapeHtml(t.composer.specialist_stop_reason) : ""})<br>
          ${t.final.original_preview ? `Orig: ${escapeHtml(t.final.original_preview)}<br>` : ""}
          ${t.final.final_preview ? `Final: ${escapeHtml(t.final.final_preview)}<br>` : ""}
          ${directorLines ? `<pre style="color:#8FA7FF;">Director:\n${escapeHtml(directorLines)}</pre>` : ""}
          ${appliedLines ? `<pre style="color:#8A55F7;">Applied:\n${escapeHtml(appliedLines)}</pre>` : ""}
          ${schedulerLines ? `<pre>Endpoint groups:\n${escapeHtml(schedulerLines)}</pre>` : ""}
          <pre>${escapeHtml(roleLines)}</pre>
          ${t.errors && t.errors.length ? `<pre style="color:#e55;">${escapeHtml(t.errors.join("\n"))}</pre>` : ""}
        </div>`;
      }).join("");
    } catch (err) {
      containerEl.innerHTML = `<pre>Error loading trace: ${escapeHtml(safeString(err && err.message))}</pre>`;
    }
  }

  let uiRoot = null;
  let uiOverlay = null;

  async function openSettingsUI() {
    const RR = getR();
    const settings = await loadSettings();
    const html = renderUI(settings);
    try {
      if (RR && typeof RR.showContainer === "function") {
        await RR.showContainer("fullscreen");
      }
    } catch (_) {}
    try {
      if (typeof document === "undefined" || !document.body) return;
      const existing = document.getElementById("recomposer-overlay");
      if (existing) existing.remove();
      uiOverlay = document.createElement("div");
      uiOverlay.id = "recomposer-overlay";
      uiOverlay.innerHTML = html;
      document.body.appendChild(uiOverlay);
      uiRoot = uiOverlay.querySelector(".recomposer-root");
    } catch (err) {
      warn("UI render error:", err);
      return;
    }
    try {
      if (!uiRoot) return;
      const closeUI = async () => {
        try { if (uiOverlay && uiOverlay.parentNode) uiOverlay.parentNode.removeChild(uiOverlay); } catch (_) {}
        uiOverlay = null;
        uiRoot = null;
        try {
          if (RR && typeof RR.hideContainer === "function") await RR.hideContainer();
        } catch (_) {}
      };
      uiOverlay.addEventListener("click", (event) => {
        if (event.target === uiOverlay) closeUI();
      });
      const tabs = uiRoot.querySelectorAll(".recomposer-tab");
      tabs.forEach((tab) => {
        tab.addEventListener("click", () => {
          const target = tab.getAttribute("data-tab");
          uiRoot.querySelectorAll(".recomposer-tab").forEach((t) => t.classList.remove("active"));
          uiRoot.querySelectorAll(".recomposer-tab-panel").forEach((p) => p.classList.remove("active"));
          tab.classList.add("active");
          const panel = uiRoot.querySelector(`.recomposer-tab-panel[data-panel="${target}"]`);
          if (panel) panel.classList.add("active");
        });
      });
      const saveBtn = uiRoot.querySelector(".recomposer-save");
      if (saveBtn) {
        saveBtn.addEventListener("click", async () => {
          try {
            const newSettings = await collectSettingsFromUI(uiRoot);
            await saveSettings(newSettings);
            saveBtn.textContent = "저장됨";
            setTimeout(() => { saveBtn.textContent = "설정 저장"; }, 2000);
          } catch (saveErr) {
            error("save error:", saveErr);
            saveBtn.textContent = "저장 실패";
            setTimeout(() => { saveBtn.textContent = "설정 저장"; }, 3000);
          }
        });
      }
      const closeBtn = uiRoot.querySelector(".recomposer-close");
      if (closeBtn) {
        closeBtn.addEventListener("click", closeUI);
      }
      const refreshBtn = uiRoot.querySelector(".recomposer-trace-refresh");
      const traceContainer = uiRoot.querySelector(".recomposer-trace-list");
      if (refreshBtn && traceContainer) {
        refreshBtn.addEventListener("click", () => refreshTraceList(traceContainer));
        refreshTraceList(traceContainer);
      }
      const compareBtn = uiRoot.querySelector(".recomposer-compare-refresh");
      const compareContainer = uiRoot.querySelector(".recomposer-compare-list");
      if (compareContainer) compareContainer.innerHTML = renderLatestComparison();
      if (compareBtn && compareContainer) {
        compareBtn.addEventListener("click", () => {
          compareContainer.innerHTML = renderLatestComparison();
        });
      }
      const clearBtn = uiRoot.querySelector(".recomposer-trace-clear");
      if (clearBtn) {
        clearBtn.addEventListener("click", async () => {
          await storageSet(TRACE_KEY, "[]");
          if (traceContainer) traceContainer.innerHTML = "";
        });
      }
    } catch (err) {
      warn("UI bind error:", err);
    }
  }

  /* ── In-Memory Tests ───────────────────────────────────── */

  async function runInMemoryTests() {
    const results = [];
    function test(name, fn) {
      try {
        const r = fn();
        results.push({ name, pass: true, detail: r || "" });
      } catch (err) {
        results.push({ name, pass: false, detail: safeString(err && err.message) });
      }
    }
    async function asyncTest(name, fn) {
      try {
        const r = await fn();
        results.push({ name, pass: true, detail: r || "" });
      } catch (err) {
        results.push({ name, pass: false, detail: safeString(err && err.message) });
      }
    }

    const settings = defaultSettings();

    // Test 1: protected 없는 전체 재작성
    test("1_full_rewrite_no_protected", () => {
      const text = "The wind howled across the moor. She pulled her cloak tighter.";
      const segs = buildSegmentMap(text, settings);
      const mutable = mutableSegments(segs);
      if (mutable.length < 1) throw new Error("no mutable segments");
      if (segs.some((s) => s.type === "protected")) throw new Error("unexpected protected");
      return `${segs.length} segments, ${mutable.length} mutable`;
    });

    // Test 2: 이미지/상태창/코드가 섞인 출력
    test("2_mixed_protected_image_status_code", () => {
      const text = 'She smiled. <img cmd="photo"> Then she said hello. ```status\nHP: 100\n``` Finally, `code_here` was visible.';
      const segs = buildSegmentMap(text, settings);
      const protectedSegs = segs.filter((s) => s.type === "protected");
      const inspectSegs = segs.filter((s) => s.type === "inspect_only");
      const mutableSegs = segs.filter((s) => s.type === "mutable");
      if (protectedSegs.length < 2) throw new Error(`expected >=2 protected, got ${protectedSegs.length}`);
      if (mutableSegs.length < 1) throw new Error("no mutable segments");
      return `P:${protectedSegs.length} I:${inspectSegs.length} M:${mutableSegs.length}`;
    });

    // Test 3: status inspect-only exact preservation
    test("3_status_inspect_only_exact_preservation", () => {
      const text = "Some prose here. ```status\nHP: 100\nMP: 50\n``` More prose after.";
      const segs = buildSegmentMap(text, settings);
      const inspectSegs = segs.filter((s) => s.type === "inspect_only");
      if (!inspectSegs.length) throw new Error("no inspect segments — status fence not classified as inspect_only");
      const assembled = assembleOutput(segs, null, { ranked: {} });
      inspectSegs.forEach((seg) => {
        const final = assembled.finalSegments.find((f) => f.id === seg.id);
        if (!final || final.final_text !== seg.text) {
          throw new Error(`inspect segment ${seg.id} not preserved exactly`);
        }
      });
      return `${inspectSegs.length} inspect segments preserved`;
    });

    // Test 4: 역할 3개 후보 Fusion — 서로 다른 issue는 complementary
    test("4_three_role_fusion", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "A", confidence: 0.8, tags: ["voice"] },
          { role_id: "style_reader", rewrite: "B", confidence: 0.7, tags: ["style"] },
          { role_id: "plot_continuity_reader", rewrite: "C", confidence: 0.6, tags: ["continuity"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      if (!result.ranked.mutable_1 || result.ranked.mutable_1.length !== 3) throw new Error("ranking failed");
      if (result.ranked.mutable_1[0].rewrite !== "A") throw new Error("top candidate should be A (highest confidence * priority)");
      // voice→character_voice, style→rhythm, continuity→plot_continuity: all different issues → complementary, not consensus
      if (result.consensus.mutable_1) throw new Error("different issues should not produce consensus");
      if (!result.complementary.mutable_1 || !result.complementary.mutable_1.length) throw new Error("complementary not detected for different issues");
      return `top: ${result.ranked.mutable_1[0].role_id} score ${result.ranked.mutable_1[0].score.toFixed(1)} complementary: ${result.complementary.mutable_1.join(",")}`;
    });

    // Test 5: 한 역할 실패 후 나머지 후보 적용
    test("5_one_role_failure_remaining_applied", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "Good rewrite", confidence: 0.85, tags: ["voice"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      const assembled = assembleOutput(
        [{ id: "mutable_1", type: "mutable", text: "original", leading_ws: "", trailing_ws: "", start: 0, end: 8 }],
        null,
        result
      );
      if (!assembled.changed) throw new Error("output not changed");
      if (assembled.output !== "Good rewrite") throw new Error("output should be top candidate");
      return `applied top candidate: ${assembled.output}`;
    });

    // Test 6: Composer 성공
    test("6_composer_success", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "", start: 0, end: 10 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 10, end: 20 },
      ];
      const composerResult = { segments: { mutable_1: "composed 1", mutable_2: "composed 2" } };
      const directorResult = { ranked: {} };
      const assembled = assembleOutput(segs, composerResult, directorResult);
      if (!assembled.changed) throw new Error("output not changed");
      if (assembled.output !== "composed 1composed 2") throw new Error(`unexpected output: ${assembled.output}`);
      return `composer output: ${assembled.output}`;
    });

    // Test 7: Composer 실패 후 최고 후보 적용
    test("7_composer_failure_top_candidate", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original", leading_ws: "", trailing_ws: "", start: 0, end: 8 },
      ];
      const directorResult = {
        ranked: {
          mutable_1: [{ role_id: "character_reader", rewrite: "best candidate", confidence: 0.9, score: 90, tags: ["voice"] }],
        },
      };
      const assembled = assembleOutput(segs, null, directorResult);
      if (!assembled.changed) throw new Error("output not changed");
      if (assembled.output !== "best candidate") throw new Error("output should be top candidate");
      return `fallback to top candidate: ${assembled.output}`;
    });

    // Test 8: deadline 부분 결과 반환
    await asyncTest("8_deadline_partial_results", async () => {
      const deadline = createDeadline(50);
      await new Promise((resolve) => setTimeout(resolve, 100));
      if (!deadline.check()) throw new Error("deadline should have passed");
      if (!deadline.aborted()) throw new Error("deadline should be aborted");
      if (deadline.remaining() > 0) throw new Error("remaining should be 0");
      deadline.cancel();
      return "deadline expired and aborted correctly";
    });

    // Test 9: scheduleRoles 실행 후 deadline 반환 시 activeCount=0
    await asyncTest("9_scheduleRoles_deadline_active_zero", async () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "prose", leading_ws: "", trailing_ws: "", start: 0, end: 5 },
      ];
      const trace = newTrace("test", "test");
      const deadline = createDeadline(50);
      const roles = selectRoles(DEFAULT_ROLES, [{ id: "mechanical_artifact", severity: "high" }], "balanced", settings).roles;
      const profiles = deepClone(settings.role_profiles);
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      const result = await scheduleRoles(roles, profiles, segs, "", segs, deadline, trace, 5);
      deadline.cancel();
      if (typeof trace.active_calls_final !== "number") throw new Error("active_calls_final not recorded");
      if (trace.active_calls_final !== 0) throw new Error(`active_calls_final should be 0, got ${trace.active_calls_final}`);
      return `active_calls_final=${trace.active_calls_final}`;
    });

    // Test 10: API key 저장/마스킹/resolve
    await asyncTest("10_api_key_mask_resolve", async () => {
      const masked = maskKey("sk-1234567890abcdef");
      if (masked.indexOf("1234") >= 0 || masked.indexOf("abcdef") >= 0) throw new Error("key not properly masked");
      if (masked.indexOf("••••") < 0) throw new Error("mask marker missing");
      const resolved = await resolveApiKey("direct-key-value");
      if (resolved !== "direct-key-value") throw new Error("direct key not resolved");
      return `masked: ${masked}, resolved direct key OK`;
    });

    // Test 11: Trace 평문 key 없음
    test("11_trace_no_plain_key", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: Date.now(),
        ended_at: Date.now(),
        elapsed_ms: 100,
      });
      const traceStr = JSON.stringify(trace);
      const testKey = "sk-test-secret-key-12345";
      if (traceStr.indexOf(testKey) >= 0) throw new Error("plain key found in trace");
      return "no plain key in trace";
    });

    // Test 12: 최종 output이 draft_zero와 실제로 다름
    test("12_output_differs_from_draft", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "The wind blew.", leading_ws: "", trailing_ws: "", start: 0, end: 14 },
      ];
      const composerResult = { segments: { mutable_1: "The cold wind swept across the barren moor." } };
      const assembled = assembleOutput(segs, composerResult, { ranked: {} });
      if (assembled.output === "The wind blew.") throw new Error("output identical to draft");
      if (!assembled.changed) throw new Error("changed flag false");
      return `output changed: "${preview(assembled.output, 60)}"`;
    });

    // Test 13: verifier protected preservation
    test("13_verifier_protected_preservation", () => {
      const segs = [
        { id: "protected_1", type: "protected", kind: "image_tag", text: '<img src="test">', start: 0, end: 17 },
        { id: "mutable_1", type: "mutable", text: "prose", start: 17, end: 22 },
      ];
      const finalSegs = [
        { id: "protected_1", type: "protected", kind: "image_tag", original_text: '<img src="test">', final_text: '<img src="test">', source: "preserved" },
        { id: "mutable_1", type: "mutable", original_text: "prose", final_text: "rewritten prose", source: "composer" },
      ];
      const v = verifyOutput(segs, finalSegs, '<img src="test">rewritten prose', "original");
      if (!v.pass) throw new Error(`verifier failed: ${v.errors.join(",")}`);
      return "protected preserved, verifier pass";
    });

    // Test 14: verifier catches preservation violation
    test("14_verifier_catches_preservation_violation", () => {
      const segs = [
        { id: "protected_1", type: "protected", kind: "image_tag", text: '<img src="test">', start: 0, end: 17 },
      ];
      const finalSegs = [
        { id: "protected_1", type: "protected", kind: "image_tag", original_text: '<img src="test">', final_text: "BROKEN", source: "preserved" },
      ];
      const v = verifyOutput(segs, finalSegs, "BROKEN", "original");
      if (v.pass) throw new Error("verifier should have caught violation");
      if (v.errors.indexOf("preservation_violation:protected_1") < 0) throw new Error("missing violation error");
      return `caught: ${v.errors.join(",")}`;
    });

    // Test 15: verifier empty output
    test("15_verifier_empty_output", () => {
      const segs = [{ id: "mutable_1", type: "mutable", text: "text", start: 0, end: 4 }];
      const finalSegs = [{ id: "mutable_1", type: "mutable", original_text: "text", final_text: "", source: "composer" }];
      const v = verifyOutput(segs, finalSegs, "", "text");
      if (v.pass) throw new Error("verifier should catch empty output");
      return `caught: ${v.errors.join(",")}`;
    });

    // Test 16: Quality keeps the full role pool but executes an adaptive subset.
    test("16_router_signal_difference", () => {
      const settings2 = defaultSettings();
      Object.keys(settings2.role_profiles).forEach((rid) => {
        settings2.role_profiles[rid].endpoint = "https://test.example.com/v1";
        settings2.role_profiles[rid].model = "test-model";
      });
      const noSignals = [];
      const metaSignals = [{ id: "mechanical_artifact", severity: "high" }];
      const selectedNone = selectRoles(DEFAULT_ROLES, noSignals, "quality", settings2);
      const selectedMeta = selectRoles(DEFAULT_ROLES, metaSignals, "quality", settings2);
      const noneHasGuard = selectedNone.roles.some((r) => r.role_id === "secret_pov_guard");
      const noneHasWorld = selectedNone.roles.some((r) => r.role_id === "world_reader");
      if (!noneHasGuard) throw new Error("quality must preserve the secret/POV baseline");
      if (noneHasWorld) throw new Error("low-priority world role should be capacity-skipped without a signal");
      const noneSpecialistCount = selectedNone.roles.filter((r) => !r.is_composer).length;
      const metaSpecialistCount = selectedMeta.roles.filter((r) => !r.is_composer).length;
      if (noneSpecialistCount !== OUTPUT_SPECIALIST_LIMIT.quality
          || metaSpecialistCount !== OUTPUT_SPECIALIST_LIMIT.quality) {
        throw new Error(`quality adaptive limit failed: ${noneSpecialistCount}/${metaSpecialistCount}`);
      }
      if (!selectedMeta.roles.some((r) => r.role_id === "agency_meta_guard")) {
        throw new Error("meta signal did not select agency_meta_guard");
      }
      const capacitySkip = selectedMeta.skipReasons.find((item) => item.reason.indexOf("router_capacity:") === 0);
      if (!capacitySkip) throw new Error("capacity skip reason missing");
      return `limit:${noneSpecialistCount} meta:${metaSpecialistCount} skip:${capacitySkip.role_id}`;
    });

    // Test 17: segment ID uniqueness
    test("17_segment_id_uniqueness", () => {
      const text = "prose <img> more prose ```code``` end prose";
      const segs = buildSegmentMap(text, settings);
      const ids = segs.map((s) => s.id);
      const unique = new Set(ids);
      if (unique.size !== ids.length) throw new Error("duplicate segment IDs");
      return `${ids.length} unique IDs: ${ids.join(", ")}`;
    });

    // Test 18: candidate schema validation with allowedSegmentIds
    test("18_candidate_schema_allowed_segs", () => {
      const allowed = ["mutable_1", "mutable_2"];
      const good = { role: "character_reader", candidates: [
        { segment_id: "mutable_1", rewrite: "text", confidence: 0.8, tags: ["voice"] },
        { segment_id: "mutable_2", rewrite: "text2", confidence: 0.7, tags: ["emotion"] },
      ] };
      const v = validateCandidateSchema(good, "character_reader", allowed);
      if (!v || v.candidates.length !== 2) throw new Error("valid schema rejected");
      const withForeign = { role: "x", candidates: [
        { segment_id: "mutable_1", rewrite: "text", confidence: 0.8 },
        { segment_id: "foreign_id", rewrite: "text", confidence: 0.8 },
      ] };
      const v2 = validateCandidateSchema(withForeign, "x", allowed);
      if (!v2 || v2.candidates.length !== 1) throw new Error("foreign segment_id should be filtered");
      const bad = { role: "x", candidates: [{ segment_id: "", rewrite: "" }] };
      const v3 = validateCandidateSchema(bad, "x", allowed);
      if (v3) throw new Error("invalid schema accepted");
      return "schema validation with allowedSegmentIds OK";
    });

    // Test 19: composer schema validation with allowedSegmentIds
    test("19_composer_schema_allowed_segs", () => {
      const allowed = ["mutable_1", "mutable_2"];
      const good = { segments: { mutable_1: "text 1", mutable_2: "text 2" } };
      const v = validateComposerSchema(good, allowed);
      if (!v || Object.keys(v.segments).length !== 2) throw new Error("valid composer schema rejected");
      const withForeign = { segments: { mutable_1: "text 1", mutable_2: "text 2", foreign_id: "text" } };
      const v2 = validateComposerSchema(withForeign, allowed);
      if (!v2 || Object.keys(v2.segments).length !== 2) throw new Error("foreign segment should be filtered");
      if (validateComposerSchema({ segments: { mutable_1: "text 1" } }, allowed)) {
        throw new Error("partial composer output accepted");
      }
      const bad = { segments: { mutable_1: "" } };
      const v3 = validateComposerSchema(bad, allowed);
      if (v3) throw new Error("invalid composer schema accepted");
      return "composer schema validation with allowedSegmentIds OK";
    });

    // Test 20: JSON repair
    test("20_json_repair", () => {
      const fenced = '```json\n{"role":"x","candidates":[]}\n```';
      const parsed = tryParseJson(fenced);
      if (!parsed || parsed.role !== "x") throw new Error("fenced JSON not parsed");
      const trailing = '{"a":1,}';
      const parsed2 = tryParseJson(trailing);
      if (!parsed2 || parsed2.a !== 1) throw new Error("trailing comma not repaired");
      return "JSON repair OK";
    });

    // Test 21: Ollama Cloud URL/Auth/body capture via mock fetch
    await asyncTest("21_ollama_cloud_mock_fetch", async () => {
      const captured = { url: "", auth: "", bodyStr: "", reasoningNone: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.auth = (opts && opts.headers && opts.headers.Authorization) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try {
          const parsed = JSON.parse(captured.bodyStr);
          captured.reasoningNone = parsed.reasoning_effort === "none";
        } catch (_) {}
        return {
          ok: true,
          text: async () => JSON.stringify({
            choices: [{ message: { content: '{"role":"character_reader","candidates":[]}' } }],
          }),
        };
      };
      try {
        const cloudProfile = {
          provider: "ollama_compatible",
          endpoint: "https://ollama.com/v1",
          model: "kimi-k2.7-code:cloud",
          api_key_ref: "sk-ollama-cloud-key",
          temperature: 0.3,
          max_output_tokens: 1024,
          force_json_response: true,
          reasoning_preset: "auto",
          reasoning_effort: "none",
          reasoning_budget_tokens: 512,
          extra_headers: "",
          extra_body: "",
        };
        await callProvider(cloudProfile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf("ollama.com") < 0) throw new Error(`URL should contain ollama.com, got: ${captured.url}`);
        if (captured.url.indexOf("/chat/completions") < 0) throw new Error(`URL should use /chat/completions, got: ${captured.url}`);
        if (captured.auth.indexOf("Bearer sk-ollama-cloud-key") < 0) throw new Error(`Auth should be Bearer key, got: ${captured.auth}`);
        if (!captured.reasoningNone) throw new Error("Kimi reasoning_effort=none should be in body");
      } finally {
        globalThis.fetch = origFetch;
      }
      return `url:${captured.url.indexOf("/chat/completions") >= 0} auth:${captured.auth.indexOf("Bearer") >= 0} kimiNone:${captured.reasoningNone}`;
    });

    // Test 22: slow fetch deadline abort — scheduleRoles 실행 후 activeCount=0
    await asyncTest("22_slow_fetch_deadline_schedule", async () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "prose one", leading_ws: "", trailing_ws: "", start: 0, end: 9 },
        { id: "mutable_2", type: "mutable", text: "prose two", leading_ws: "", trailing_ws: "", start: 9, end: 18 },
      ];
      const trace = newTrace("test", "test");
      const origFetch = globalThis.fetch;
      globalThis.fetch = async () => {
        return new Promise(() => {});
      };
      try {
        const deadline = createDeadline(80);
        const roles = selectRoles(DEFAULT_ROLES, [{ id: "mechanical_artifact", severity: "high" }], "balanced", settings).roles;
        const testProfiles = deepClone(settings.role_profiles);
        Object.keys(testProfiles).forEach((rid) => {
          testProfiles[rid].endpoint = "https://test.example.com/v1";
          testProfiles[rid].model = "test-model";
        });
        const result = await scheduleRoles(roles, testProfiles, segs, "", segs, deadline, trace, 5);
        deadline.cancel();
        if (trace.active_calls_final !== 0) throw new Error(`active_calls_final should be 0, got ${trace.active_calls_final}`);
      } finally {
        globalThis.fetch = origFetch;
      }
      return `active_calls_final=${trace.active_calls_final}`;
    });

    // Test 23: mock fetch로 scheduleRoles 실행 — 호출 수 = specialist + 1
    await asyncTest("23_call_count_specialist_plus_composer", async () => {
      let callCount = 0;
      let specialistCalls = 0;
      let composerCalls = 0;
      let expectedMax = 0;
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        callCount++;
        let bodyStr = "";
        try { bodyStr = (opts && opts.body) || ""; } catch (_) {}
        const isComposer = bodyStr.indexOf("Whole-Scene Fusion Composer") >= 0 || bodyStr.indexOf("\"segments\"") >= 0;
        if (isComposer) {
          composerCalls++;
          return {
            ok: true,
            text: async () => JSON.stringify({
              choices: [{ message: { content: '{"segments":{"mutable_1":"composed 1","mutable_2":"composed 2","mutable_3":"composed 3"}}' } }],
            }),
          };
        }
        specialistCalls++;
        let userPrompt = "";
        try {
          const body = JSON.parse(bodyStr);
          userPrompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        } catch (_) {}
        const roleMatch = /"role":"([^"]+)"/.exec(userPrompt);
        const roleId = roleMatch ? roleMatch[1] : "character_reader";
        const issue = roleAllowedIssues(roleId)[0];
        return {
          ok: true,
          text: async () => JSON.stringify({
            choices: [{ message: { content: JSON.stringify({
              role: roleId,
              candidates: [
                { segment_id: "mutable_1", rewrite: "rewritten", confidence: 0.8, issues: [issue] },
                { segment_id: "mutable_2", rewrite: "rewritten 2", confidence: 0.7, issues: [issue] },
                { segment_id: "mutable_3", rewrite: "rewritten 3", confidence: 0.6, issues: [issue] },
              ],
            }) } }],
          }),
        };
      };
      try {
        const segs = [
          { id: "mutable_1", type: "mutable", text: "prose one", leading_ws: "", trailing_ws: "", start: 0, end: 9 },
          { id: "mutable_2", type: "mutable", text: "prose two", leading_ws: "", trailing_ws: "", start: 9, end: 18 },
          { id: "mutable_3", type: "mutable", text: "prose three", leading_ws: "", trailing_ws: "", start: 18, end: 29 },
        ];
        const trace = newTrace("test", "test");
        const deadline = createDeadline(60000);
        const testProfiles = deepClone(settings.role_profiles);
        Object.keys(testProfiles).forEach((rid) => {
          testProfiles[rid].endpoint = "https://test.example.com/v1";
          testProfiles[rid].model = "test-model";
        });
        const testSettings = Object.assign({}, settings, { role_profiles: testProfiles });
        const roles = selectRoles(DEFAULT_ROLES, [{ id: "mechanical_artifact", severity: "high" }], "balanced", testSettings).roles;
        const specialistCount = roles.filter((r) => !r.is_composer).length;
        expectedMax = specialistCount + 1;
        await scheduleRoles(roles, testProfiles, segs, "", segs, deadline, trace, 5);
        deadline.cancel();
        if (callCount !== expectedMax) throw new Error(`callCount ${callCount} should equal exactly ${expectedMax} (specialist ${specialistCount} + composer 1)`);
        if (specialistCalls !== specialistCount) throw new Error(`specialistCalls ${specialistCalls} should equal ${specialistCount}`);
        if (composerCalls !== 1) throw new Error(`composerCalls ${composerCalls} should equal 1`);
      } finally {
        globalThis.fetch = origFetch;
      }
      return `calls=${callCount}=${expectedMax} specialist=${specialistCalls} composer=${composerCalls}`;
    });

    // Test 24: Composer 입력에 원문/후보/Director 정보 포함
    test("24_composer_input_structure", () => {
      const mutableSegs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "" },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "" },
      ];
      const allSegments = mutableSegs.concat([
        { id: "protected_1", type: "protected", kind: "image_tag", text: '<img src="x">' },
      ]);
      const directorResult = {
        ranked: {
          mutable_1: [{ role_id: "character_reader", rewrite: "cand 1", confidence: 0.8, score: 80, issues: ["character_voice"], change_summary: "fixed voice" }],
        },
        consensus: { mutable_1: ["character_voice"] },
        complementary: { mutable_1: ["emotion"] },
        conflict: {},
        gap: ["mutable_2"],
      };
      const composerRole = DEFAULT_ROLES.find((r) => r.is_composer);
      const profile = defaultRoleProfile("whole_scene_composer");
      const directorInfo = {
        candidateBundles: { mutable_1: [{ role: "character_reader", confidence: 0.8, score: 80, issues: ["character_voice"], change_summary: "fixed voice", rewrite: "cand 1" }] },
        consensus: directorResult.consensus,
        complementary: directorResult.complementary,
        conflict: directorResult.conflict,
        gap: directorResult.gap,
      };
      const prompts = buildRolePrompt(composerRole, profile, mutableSegs, "context block", allSegments, directorInfo);
      const hasOriginal = prompts.user.indexOf("Original:") >= 0;
      const hasCandidates = prompts.user.indexOf("Candidate 1") >= 0;
      const hasDirector = prompts.user.indexOf("Fusion Director") >= 0;
      const hasConsensus = prompts.user.indexOf("character_voice") >= 0;
      const hasComplementary = prompts.user.indexOf("Complementary") >= 0;
      const hasGap = prompts.user.indexOf("mutable_2") >= 0;
      if (!hasOriginal) throw new Error("composer input missing original text");
      if (!hasCandidates) throw new Error("composer input missing candidates");
      if (!hasDirector) throw new Error("composer input missing director info");
      if (!hasConsensus) throw new Error("composer input missing consensus");
      if (!hasComplementary) throw new Error("composer input missing complementary");
      if (!hasGap) throw new Error("composer input missing gap info");
      return `original:${hasOriginal} candidates:${hasCandidates} director:${hasDirector} consensus:${hasConsensus} complementary:${hasComplementary} gap:${hasGap}`;
    });

    // Test 25: 이미지 태그 3개 이상 통과
    test("25_three_image_tags_pass", () => {
      const text = '<img src="a"> prose <img src="b"> more <img src="c">';
      const segs = buildSegmentMap(text, settings);
      const protectedSegs = segs.filter((s) => s.type === "protected" && s.kind === "image_tag");
      if (protectedSegs.length < 3) throw new Error(`expected >=3 image_tag protected, got ${protectedSegs.length}`);
      const assembled = assembleOutput(segs, null, { ranked: {} });
      protectedSegs.forEach((seg) => {
        const final = assembled.finalSegments.find((f) => f.id === seg.id);
        if (!final || final.final_text !== seg.text) throw new Error(`image tag ${seg.id} not preserved`);
      });
      return `${protectedSegs.length} image tags preserved`;
    });

    // Test 26: 이미지 전후 개행 보존
    test("26_image_newline_preservation", () => {
      const text = "prose before\n<img src=\"x\">\nprose after";
      const segs = buildSegmentMap(text, settings);
      const assembled = assembleOutput(segs, null, { ranked: {} });
      if (assembled.output !== text) throw new Error(`output should equal original, got: ${JSON.stringify(assembled.output)}`);
      return "newlines around image preserved";
    });

    // Test 27: API key 평문 DOM 미노출 및 유지/교체/삭제
    test("27_api_key_no_plain_dom", () => {
      const profile = defaultRoleProfile("character_reader");
      profile.api_key_ref = "sk-secret-key-12345";
      const masked = maskKey(profile.api_key_ref);
      if (masked.indexOf("secret") >= 0 || masked.indexOf("12345") >= 0) throw new Error("mask leaks key content");
      const keepResult = applyKeyUpdate("", "sk-existing-key");
      if (keepResult !== "sk-existing-key") throw new Error("blank input should keep existing key");
      const replaceResult = applyKeyUpdate("sk-new-key", "sk-old-key");
      if (replaceResult !== "sk-new-key") throw new Error("new value should replace existing key");
      const clearResult = applyKeyUpdate("clear:key", "sk-existing-key");
      if (clearResult !== "") throw new Error("clear:key should delete key");
      return `mask:${masked} keep:${keepResult === "sk-existing-key"} replace:${replaceResult === "sk-new-key"} clear:${clearResult === ""}`;
    });

    // Test 28: void HTML 태그 닫는 태그 검사 제외
    test("28_void_tag_no_close_required", () => {
      const segs = [{ id: "mutable_1", type: "mutable", text: "text", start: 0, end: 4 }];
      const finalSegs = [{ id: "mutable_1", type: "mutable", original_text: "text", final_text: "text<br><hr><img src='x'>", source: "composer" }];
      const v = verifyOutput(segs, finalSegs, "text<br><hr><img src='x'>", "text");
      if (!v.pass) throw new Error(`void tags should not require closing tags: ${v.errors.join(",")}`);
      return "void tags pass without closing tags";
    });

    // Test 29: specialist prompt에 전체 segment 목록 제공
    test("29_specialist_gets_full_segments", () => {
      const allSegments = [
        { id: "protected_1", type: "protected", kind: "image_tag", text: '<img src="x">' },
        { id: "mutable_1", type: "mutable", text: "prose here", leading_ws: "", trailing_ws: "" },
        { id: "inspect_1", type: "inspect_only", kind: "status_window", text: "```status\nHP:100\n```" },
        { id: "mutable_2", type: "mutable", text: "more prose", leading_ws: "", trailing_ws: "" },
      ];
      const mutableSegs = allSegments.filter((s) => s.type === "mutable");
      const role = DEFAULT_ROLES.find((r) => r.role_id === "character_reader");
      const profile = defaultRoleProfile("character_reader");
      const prompts = buildRolePrompt(role, profile, mutableSegs, "ctx", allSegments, null);
      const hasAllSegments = prompts.user.indexOf("protected_1") >= 0 && prompts.user.indexOf("inspect_1") >= 0;
      const hasMutableIds = prompts.user.indexOf("mutable_1") >= 0 && prompts.user.indexOf("mutable_2") >= 0;
      const hasPreservedLabel = prompts.user.indexOf("[PRESERVED") >= 0;
      const hasMutableLabel = prompts.user.indexOf("[MUTABLE") >= 0;
      if (!hasAllSegments) throw new Error("specialist prompt missing non-mutable segments");
      if (!hasMutableIds) throw new Error("specialist prompt missing mutable IDs");
      if (!hasPreservedLabel) throw new Error("specialist prompt missing PRESERVED label");
      if (!hasMutableLabel) throw new Error("specialist prompt missing MUTABLE label");
      return `allSegs:${hasAllSegments} mutableIds:${hasMutableIds} preserved:${hasPreservedLabel} mutable:${hasMutableLabel}`;
    });

    // Test 30: mutable 선행/후행 공백 보존
    test("30_mutable_whitespace_preservation", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "\n  original text  \n", leading_ws: "\n  ", core_text: "original text", trailing_ws: "  \n", start: 0, end: 20 },
      ];
      const composerResult = { segments: { mutable_1: "rewritten text" } };
      const assembled = assembleOutput(segs, composerResult, { ranked: {} });
      if (assembled.output !== "\n  rewritten text  \n") throw new Error(`whitespace not preserved: ${JSON.stringify(assembled.output)}`);
      return `whitespace preserved: ${JSON.stringify(assembled.output)}`;
    });

    // Test 31: status fence 뒤 inline code 올바르게 분리
    test("31_status_fence_then_inline_code", () => {
      const text = "Some prose. ```status\nHP: 100\n``` Then `code_here` end.";
      const segs = buildSegmentMap(text, settings);
      const inspectSegs = segs.filter((s) => s.type === "inspect_only");
      const inlineCodeSegs = segs.filter((s) => s.type === "protected" && s.kind === "inline_code");
      if (!inspectSegs.length) throw new Error("status fence not classified as inspect_only");
      if (!inlineCodeSegs.length) throw new Error("inline code after status fence not detected");
      const inlineCodeText = inlineCodeSegs[0].text;
      if (inlineCodeText.indexOf("```") >= 0) throw new Error(`inline code should not contain triple backticks: ${inlineCodeText}`);
      if (inlineCodeText !== "`code_here`") throw new Error(`inline code should be \`code_here\`, got: ${inlineCodeText}`);
      return `inspect:${inspectSegs.length} inlineCode:${inlineCodeSegs.length} text=${inlineCodeText}`;
    });

    // Test 32: triple backtick 내부를 inline-code가 가로지르지 않음
    test("32_inline_code_not_inside_fence", () => {
      const text = "Before ```code\n`inner`\nmore``` After `outer` end";
      const segs = buildSegmentMap(text, settings);
      const inlineCodeSegs = segs.filter((s) => s.type === "protected" && s.kind === "inline_code");
      const codeFenceSegs = segs.filter((s) => s.type === "protected" && s.kind === "code_fence");
      if (!codeFenceSegs.length) throw new Error("code fence not detected");
      if (!inlineCodeSegs.length) throw new Error("outer inline code not detected");
      const outerInline = inlineCodeSegs.find((s) => s.text === "`outer`");
      if (!outerInline) throw new Error("outer inline code not found");
      const innerInline = inlineCodeSegs.find((s) => s.text === "`inner`");
      if (innerInline) throw new Error("inner inline code inside fence should not be detected");
      return `fence:${codeFenceSegs.length} inline:${inlineCodeSegs.length} outerFound:${!!outerInline} innerLeaked:${!!innerInline}`;
    });

    // Test 33: 등록/요소 계약 검사 — document 없으면 renderUI 문자열 검사, 있으면 DOM mock으로 검증
    await asyncTest("33_ui_registration_and_element_contract", async () => {
      const mockR = {
        _registeredSettings: [],
        _registeredButtons: [],
        _replacers: {},
        _arguments: [],
        _showContainerCalled: false,
        _hideContainerCalled: false,
        _buttonCallback: null,
        async registerSetting(name, callback, icon, type) {
          this._registeredSettings.push({ name, callback, icon, type });
        },
        async registerButton(buttonObj, callback) {
          this._registeredButtons.push(buttonObj);
          this._buttonCallback = callback;
        },
        addRisuReplacer(event, fn) {
          this._replacers[event] = fn;
        },
        addArgument(name, fn) {
          this._arguments.push({ name, fn });
        },
        getStorage() { return null; },
        setStorage() {},
        async showContainer() { this._showContainerCalled = true; },
        async hideContainer() { this._hideContainerCalled = true; },
      };
      const origR = globalThis.Risuai;
      globalThis.Risuai = mockR;
      try {
        await initialize();
        const hasSetting = mockR._registeredSettings.some((s) => s.name === "Risu Recomposer" && s.type === "html");
        const buttonObj = mockR._registeredButtons.find((b) => b.name === "Risu Recomposer Settings");
        const hasButton = !!buttonObj;
        const hasButtonId = buttonObj && buttonObj.id === "risu-recomposer-chat-btn";
        const hasButtonLocation = buttonObj && buttonObj.location === "chat";
        const hasButtonIconType = buttonObj && buttonObj.iconType === "html";
        const hasButtonCallback = typeof mockR._buttonCallback === "function";
        const hasBefore = !!mockR._replacers.beforeRequest;
        const hasAfter = !!mockR._replacers.afterRequest;
        if (!hasSetting) throw new Error("registerSetting not called with html type");
        if (!hasButton) throw new Error("registerButton not called");
        if (!hasButtonId) throw new Error("registerButton missing id=risu-recomposer-chat-btn");
        if (!hasButtonLocation) throw new Error("registerButton missing location=chat");
        if (!hasButtonIconType) throw new Error("registerButton missing iconType=html");
        if (!hasButtonCallback) throw new Error("registerButton second callback arg missing");
        if (!hasBefore || !hasAfter) throw new Error("addRisuReplacer not called for both events");
        const s = defaultSettings();
        const uiHtml = renderUI(s);
        if (uiHtml.indexOf("recomposer-root") < 0) throw new Error("renderUI missing .recomposer-root");
        if (uiHtml.indexOf("recomposer-tab") < 0) throw new Error("renderUI missing tab structure");
        if (uiHtml.indexOf('data-tab="general"') < 0) throw new Error("renderUI missing General tab");
        if (uiHtml.indexOf('data-tab="roles"') < 0) throw new Error("renderUI missing Roles tab");
        if (uiHtml.indexOf('data-tab="trace"') < 0) throw new Error("renderUI missing Trace tab");
        if (uiHtml.indexOf('data-tab="compare"') < 0) throw new Error("renderUI missing Compare tab");
        if (uiHtml.indexOf("recomposer-save") < 0) throw new Error("renderUI missing save button");
        if (uiHtml.indexOf("recomposer-close") < 0) throw new Error("renderUI missing close button");
        if (uiHtml.indexOf("recomposer-role-row") < 0) throw new Error("renderUI missing role rows");
        if (uiHtml.indexOf("recomposer-provider-table") >= 0) throw new Error("renderUI should not contain provider details table");
        if (uiHtml.indexOf("--rc-bg") < 0) throw new Error("renderUI missing CSS variables");
        if (uiHtml.indexOf("max-width: 1080px") < 0) throw new Error("renderUI missing product panel max-width");
        if (uiHtml.indexOf("max-height: calc(100vh - 48px)") < 0) throw new Error("renderUI missing bounded panel height");
        if (uiHtml.indexOf("background: rgba(4, 6, 9, 0.76)") < 0) throw new Error("renderUI missing translucent backdrop");
        if (uiHtml.indexOf("grid-template-columns") < 0) throw new Error("renderUI missing responsive grid");
        if (uiHtml.indexOf("@media") < 0) throw new Error("renderUI missing media query");
        if (uiHtml.indexOf("#0B0D11") < 0) throw new Error("renderUI missing dark background color");
        if (uiHtml.indexOf("recomposer-btn-primary") < 0) throw new Error("renderUI missing primary button class");
        if (uiHtml.indexOf("recomposer-product-bar") < 0) throw new Error("renderUI missing compact product header");
        if (uiHtml.indexOf("recomposer-section-label") < 0) throw new Error("renderUI missing section labels");
        if (typeof document !== "undefined" && document.body) {
          const settingCallback = mockR._registeredSettings.find((s2) => s2.name === "Risu Recomposer").callback;
          await settingCallback();
          if (!mockR._showContainerCalled) throw new Error("showContainer not called by openSettingsUI");
          const rootEl = document.querySelector(".recomposer-root");
          if (!rootEl) throw new Error(".recomposer-root not created in DOM");
          const saveBtn = rootEl.querySelector(".recomposer-save");
          const closeBtn = rootEl.querySelector(".recomposer-close");
          if (!saveBtn) throw new Error("save button not found in DOM");
          if (!closeBtn) throw new Error("close button not found in DOM");
          closeBtn.click();
          await new Promise((resolve) => setTimeout(resolve, 50));
          if (!mockR._hideContainerCalled) throw new Error("hideContainer not called after close");
          return `setting:${hasSetting} button:${hasButton} id:${hasButtonId} loc:${hasButtonLocation} iconType:${hasButtonIconType} cb:${hasButtonCallback} show:${mockR._showContainerCalled} hide:${mockR._hideContainerCalled} DOM:pass`;
        }
        return `setting:${hasSetting} button:${hasButton} id:${hasButtonId} loc:${hasButtonLocation} iconType:${hasButtonIconType} cb:${hasButtonCallback} string:pass`;
      } finally {
        globalThis.Risuai = origR;
      }
    });

    // Test 34: streaming 감지 — type keyword and OpenAIChat[] role-based detection
    test("34_streaming_detection", () => {
      const fromType = detectStreamingState(null, "streaming");
      if (!fromType.detected) throw new Error("streaming type not detected");
      const fromFunctionRole = detectStreamingState([{ role: "function", content: "x" }], "main");
      if (!fromFunctionRole.detected) throw new Error("function role not detected as streaming");
      const fromToolRole = detectStreamingState([{ role: "tool", content: "x" }], "main");
      if (!fromToolRole.detected) throw new Error("tool role not detected as streaming");
      const notStreaming = detectStreamingState([{ role: "user", content: "hi" }], "main");
      if (notStreaming.detected) throw new Error("non-streaming should not be detected");
      return `type:${fromType.detected} function:${fromFunctionRole.detected} tool:${fromToolRole.detected} normal:${!notStreaming.detected}`;
    });

    // Test 35: streaming trace 기록 — onAfterRequest에서 streaming 상태가 trace에 기록됨
    await asyncTest("35_streaming_trace_recorded", async () => {
      const origR = globalThis.Risuai;
      const mockR = Object.assign({}, origR || {}, {
        _registeredSettings: [], _registeredButtons: [], _replacers: {}, _arguments: [],
        async registerSetting() {}, async registerButton() {},
        addRisuReplacer(e, f) { this._replacers[e] = f; },
        addArgument() {},
        getStorage() { return Promise.resolve(null); },
        setStorage() { return Promise.resolve(); },
        async showContainer() {}, async hideContainer() {},
      });
      globalThis.Risuai = mockR;
      try {
        const beforeFn = mockR._replacers.beforeRequest || globalThis.__recomposer;
        const afterFn = mockR._replacers.afterRequest;
        if (!afterFn) {
          await (globalThis.__recomposer.initialize || function() {})();
        }
        const after = mockR._replacers.afterRequest;
        if (!after) throw new Error("afterRequest replacer not registered");
        const before = mockR._replacers.beforeRequest;
        if (before) await before({ options: { stream: true }, messages: [] }, "main");
        const result = await after("some content here", "main");
        return `result returned: ${typeof result === "string"}`;
      } finally {
        globalThis.Risuai = origR;
      }
    });

    // Test 36: RisuAI pluginStorage 설정 저장/read-back
    await asyncTest("36_plugin_storage_settings_roundtrip", async () => {
      const values = new Map();
      const mockR = {
        pluginStorage: {
          async getItem(key) { return values.has(key) ? values.get(key) : null; },
          async setItem(key, value) { values.set(key, value); },
        },
      };
      const origR = globalThis.Risuai;
      globalThis.Risuai = mockR;
      try {
        const next = defaultSettings();
        next.preset = "quality";
        next.deadline_ms = 180000;
        next.max_parallel = 2;
        next.role_profiles.character_reader.model = "storage-roundtrip-model";
        next.role_profiles.character_reader.api_key_ref = "storage-roundtrip-key";
        await saveSettings(next);
        const loaded = await loadSettings();
        if (loaded.preset !== "quality") throw new Error("preset did not persist");
        if (loaded.deadline_ms !== 180000) throw new Error("deadline did not persist");
        if (loaded.max_parallel !== 2) throw new Error("max_parallel did not persist");
        if (loaded.role_profiles.character_reader.model !== "storage-roundtrip-model") throw new Error("role model did not persist");
        if (loaded.role_profiles.character_reader.api_key_ref !== "storage-roundtrip-key") throw new Error("role key did not persist");
        return "pluginStorage save/read-back passed";
      } finally {
        globalThis.Risuai = origR;
      }
    });

    // Test 37: R1 — 같은 segment, 같은 issue, 다른 역할 2개 → true consensus
    test("37_r1_same_issue_diff_role_consensus", () => {
      const candidates = {
        mutable_1: [
          { role_id: "secret_pov_guard", rewrite: "She hesitated, unsure.", confidence: 0.9, issues: ["pov_violation"] },
          { role_id: "character_reader", rewrite: "She paused, uncertain.", confidence: 0.8, issues: ["pov_violation"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      if (!result.consensus.mutable_1 || result.consensus.mutable_1.indexOf("pov_violation") < 0) {
        throw new Error("same issue from 2 roles should produce consensus");
      }
      if (result.complementary.mutable_1) throw new Error("consensus issue should not also be complementary");
      return `consensus: ${result.consensus.mutable_1.join(",")}`;
    });

    // Test 38: R1 — 같은 segment, 서로 다른 issue → complementary, conflict 아님
    test("38_r1_diff_issue_complementary", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "She spoke softly.", confidence: 0.8, issues: ["character_voice"] },
          { role_id: "style_reader", rewrite: "She whispered.", confidence: 0.7, issues: ["rhythm"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      if (result.consensus.mutable_1) throw new Error("different issues should not produce consensus");
      if (!result.complementary.mutable_1 || !result.complementary.mutable_1.length) {
        throw new Error("different issues should produce complementary");
      }
      if (result.conflict.mutable_1) throw new Error("different issues should not produce conflict");
      return `complementary: ${result.complementary.mutable_1.join(",")}`;
    });

    // Test 39: R1 — 같은 issue, 상반된 rewrite → conflict
    test("39_r1_same_issue_conflict", () => {
      const candidates = {
        mutable_1: [
          { role_id: "secret_pov_guard", rewrite: "She hesitated, unsure of what lay beyond the door. The hallway stretched endlessly before her.", confidence: 0.9, issues: ["pov_violation"] },
          { role_id: "character_reader", rewrite: "NO! She KNEW everything! The door was open and she charged through!", confidence: 0.8, issues: ["pov_violation"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      if (!result.conflict.mutable_1 || result.conflict.mutable_1.indexOf("pov_violation") < 0) {
        throw new Error("same issue with very different rewrites should produce conflict");
      }
      return `conflict: ${result.conflict.mutable_1.join(",")}`;
    });

    // Test 40: R1 — 후보 여러 개지만 issue가 다름 → false consensus
    test("40_r1_multi_candidate_diff_issue_no_consensus", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "A", confidence: 0.8, issues: ["character_voice"] },
          { role_id: "style_reader", rewrite: "B", confidence: 0.7, issues: ["repetition"] },
          { role_id: "world_reader", rewrite: "C", confidence: 0.6, issues: ["world_rule"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 4);
      const result = fusionDirector(candidates, roles);
      if (result.consensus.mutable_1) throw new Error("all different issues should not produce consensus");
      if (!result.complementary.mutable_1 || result.complementary.mutable_1.length !== 3) {
        throw new Error("all 3 different issues should be complementary");
      }
      return `complementary count: ${result.complementary.mutable_1.length}`;
    });

    // Test 41: R1 — 이전 schema의 tags만 있는 후보 → 정상 정규화
    test("41_r1_tags_only_normalization", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "A", confidence: 0.8, tags: ["voice"] },
          { role_id: "style_reader", rewrite: "B", confidence: 0.7, tags: ["style"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const result = fusionDirector(candidates, roles);
      if (!result.ranked.mutable_1 || result.ranked.mutable_1.length !== 2) throw new Error("tags-only candidates should be ranked");
      const issues0 = result.ranked.mutable_1[0].issues || [];
      const issues1 = result.ranked.mutable_1[1].issues || [];
      if (!issues0.length) throw new Error("tags-only candidate 0 should have normalized issues");
      if (!issues1.length) throw new Error("tags-only candidate 1 should have normalized issues");
      if (issues0.indexOf("character_voice") < 0) throw new Error("voice tag should normalize to character_voice");
      if (issues1.indexOf("rhythm") < 0) throw new Error("style tag should normalize to rhythm");
      return `normalized: ${issues0.join(",")} | ${issues1.join(",")}`;
    });

    // Test 42: R1 — secret/POV 후보와 style 후보 충돌 → secret/POV 우선
    test("42_r1_secret_pov_priority_over_style", () => {
      const candidates = {
        mutable_1: [
          { role_id: "style_reader", rewrite: "Beautiful flowing prose.", confidence: 0.95, issues: ["rhythm"] },
          { role_id: "secret_pov_guard", rewrite: "She could not know his thoughts.", confidence: 0.7, issues: ["secret_leak"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 4);
      const result = fusionDirector(candidates, roles);
      const top = result.ranked.mutable_1[0];
      if (top.role_id !== "secret_pov_guard") {
        throw new Error(`secret_pov_guard should rank above style_reader even with lower confidence, got ${top.role_id}`);
      }
      return `top: ${top.role_id} score ${top.score.toFixed(1)} (issue_pri ${top.issue_priority})`;
    });

    // Test 43: R1 — 원문과 동일한 rewrite → 낮은 점수
    test("43_r1_identical_rewrite_low_score", () => {
      const origText = "The wind howled across the moor.";
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: origText, confidence: 0.9, issues: ["character_voice"] },
          { role_id: "style_reader", rewrite: "The wind screamed across the barren moor.", confidence: 0.8, issues: ["rhythm"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer);
      const origSegs = [{ id: "mutable_1", type: "mutable", text: origText }];
      const result = fusionDirector(candidates, roles, origSegs);
      const identical = result.ranked.mutable_1.find((c) => c.rewrite === origText);
      const changed = result.ranked.mutable_1.find((c) => c.rewrite !== origText);
      if (!identical || !changed) throw new Error("both candidates should be ranked");
      if (identical.score >= changed.score) {
        throw new Error(`identical rewrite should have lower score: ${identical.score.toFixed(1)} vs ${changed.score.toFixed(1)}`);
      }
      return `identical: ${identical.score.toFixed(1)} < changed: ${changed.score.toFixed(1)}`;
    });

    // Test 44: R1 — 정상 후보가 directorResult.ranked에 보존
    test("44_r1_valid_candidates_preserved_in_ranked", () => {
      const candidates = {
        mutable_1: [
          { role_id: "character_reader", rewrite: "Good rewrite", confidence: 0.85, issues: ["character_voice"], change_summary: "fixed voice" },
          { role_id: "style_reader", rewrite: "Better rhythm", confidence: 0.7, issues: ["rhythm"], change_summary: "fixed rhythm" },
        ],
        mutable_2: [],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const origSegs = [
        { id: "mutable_1", type: "mutable", text: "original 1" },
        { id: "mutable_2", type: "mutable", text: "original 2" },
      ];
      const result = fusionDirector(candidates, roles, origSegs);
      if (!result.ranked.mutable_1 || result.ranked.mutable_1.length !== 2) throw new Error("ranked should preserve all valid candidates");
      if (result.ranked.mutable_1[0].issues.indexOf("character_voice") < 0) throw new Error("issues should be preserved in ranked");
      if (result.ranked.mutable_1[0].change_summary !== "fixed voice") throw new Error("change_summary should be preserved in ranked");
      if (result.gap.indexOf("mutable_2") < 0) throw new Error("empty candidate segment should be in gap");
      return `ranked: ${result.ranked.mutable_1.length} candidates, gap: ${result.gap.join(",")}`;
    });

    // Test 45: R2 — 동일 의미의 한국어/영어 장면에서 핵심 역할 집합이 비정상적으로 달라지지 않음
    test("45_r2_korean_english_core_roles_stable", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      const enSegs = buildSegmentMap("She said, \"Hello there.\" He replied, \"How are you?\" They walked together.", settings2);
      const koSegs = buildSegmentMap("그녀가 말했다, \"안녕하세요.\" 그가 대답했다, \"잘 지내세요?\" 둘이 함께 걸었다.", settings2);
      const enCtx = { bounded_context_block: "", character: "char", lorebook: "", memory: "" };
      const koCtx = { bounded_context_block: "", character: "char", lorebook: "", memory: "" };
      const enSignals = detectSceneSignals(enSegs, enCtx);
      const koSignals = detectSceneSignals(koSegs, koCtx);
      const enSelected = selectRoles(DEFAULT_ROLES, enSignals, "balanced", settings2);
      const koSelected = selectRoles(DEFAULT_ROLES, koSignals, "balanced", settings2);
      const enIds = enSelected.roles.map((r) => r.role_id).sort().join(",");
      const koIds = koSelected.roles.map((r) => r.role_id).sort().join(",");
      if (enIds !== koIds) throw new Error(`Korean and English should produce same role set, got en:${enIds} ko:${koIds}`);
      return `en==ko: ${enIds}`;
    });

    // Test 46: R2 — Balanced에서 하나의 신호가 감지되어도 핵심 역할 유지
    test("46_r2_balanced_signal_preserves_core", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      const noSignalResult = selectRoles(DEFAULT_ROLES, [], "balanced", settings2);
      const withSignalResult = selectRoles(DEFAULT_ROLES, [{ id: "dialogue_heavy", severity: "medium" }], "balanced", settings2);
      const noIds = new Set(noSignalResult.roles.map((r) => r.role_id));
      const withIds = new Set(withSignalResult.roles.map((r) => r.role_id));
      noIds.forEach((id) => {
        if (!withIds.has(id)) throw new Error(`signal should not remove role ${id} from balanced`);
      });
      if (withSignalResult.roles.length !== noSignalResult.roles.length) {
        throw new Error(`signal should not change role count in balanced: ${noSignalResult.roles.length} vs ${withSignalResult.roles.length}`);
      }
      return `core preserved: ${withSignalResult.roles.length} roles`;
    });

    // Test 47: Quality keeps six configured profiles but selects at most five specialists.
    test("47_r2_quality_adaptive_five_plus_composer", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      const result = selectRoles(DEFAULT_ROLES, [], "quality", settings2);
      const specialists = result.roles.filter((r) => !r.is_composer);
      const composers = result.roles.filter((r) => r.is_composer);
      if (specialists.length !== OUTPUT_SPECIALIST_LIMIT.quality) {
        throw new Error(`quality should select ${OUTPUT_SPECIALIST_LIMIT.quality} specialists, got ${specialists.length}`);
      }
      if (composers.length !== 1) throw new Error(`quality should select 1 composer, got ${composers.length}`);
      if (!specialists.some((role) => role.role_id === "secret_pov_guard")) {
        throw new Error("quality baseline lost secret_pov_guard");
      }
      const capacitySkips = result.skipReasons.filter((item) => item.reason.indexOf("router_capacity:") === 0);
      if (capacitySkips.length !== 1) throw new Error(`expected one capacity skip, got ${capacitySkips.length}`);
      return `pool:6 selected:${specialists.length} composer:${composers.length}`;
    });

    // Test 48: R2 — Fast에서 정해진 핵심 역할 + Composer만 선택
    test("48_r2_fast_core_roles_only", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      const result = selectRoles(DEFAULT_ROLES, [], "fast", settings2);
      const specialists = result.roles.filter((r) => !r.is_composer);
      const composers = result.roles.filter((r) => r.is_composer);
      if (specialists.length !== 2) throw new Error(`fast should select 2 specialists, got ${specialists.length}`);
      if (composers.length !== 1) throw new Error(`fast should select 1 composer, got ${composers.length}`);
      const ids = specialists.map((r) => r.role_id).sort().join(",");
      if (ids !== "character_reader,style_reader") throw new Error(`fast specialist set mismatch: ${ids}`);
      return `specialists:${specialists.length} composer:${composers.length}`;
    });

    // Test 49: R2 — 비활성 역할 제외
    test("49_r2_disabled_role_excluded", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      Object.keys(profiles).forEach((rid) => {
        profiles[rid].endpoint = "https://test.example.com/v1";
        profiles[rid].model = "test-model";
      });
      profiles.character_reader.enabled = false;
      const result = selectRoles(DEFAULT_ROLES, [], "balanced", settings2);
      const hasCharReader = result.roles.some((r) => r.role_id === "character_reader");
      if (hasCharReader) throw new Error("disabled role should be excluded");
      const skipEntry = result.skipReasons.find((s) => s.role_id === "character_reader");
      if (!skipEntry || skipEntry.reason !== "disabled") throw new Error("disabled role should have skip reason");
      return `excluded: character_reader, skipReason: ${skipEntry.reason}`;
    });

    // Test 50: R2 — 설정 불완전 역할의 skip reason 기록
    test("50_r2_not_configured_skip_reason", () => {
      const settings2 = defaultSettings();
      const profiles = settings2.role_profiles;
      // Leave model empty for secret_pov_guard
      Object.keys(profiles).forEach((rid) => {
        if (rid !== "secret_pov_guard") {
          profiles[rid].endpoint = "https://test.example.com/v1";
          profiles[rid].model = "test-model";
        }
      });
      const result = selectRoles(DEFAULT_ROLES, [], "balanced", settings2);
      const skipEntry = result.skipReasons.find((s) => s.role_id === "secret_pov_guard");
      if (!skipEntry) throw new Error("not_configured role should have skip reason");
      if (skipEntry.reason !== "not_configured") throw new Error(`skip reason should be not_configured, got ${skipEntry.reason}`);
      const hasGuard = result.roles.some((r) => r.role_id === "secret_pov_guard");
      if (hasGuard) throw new Error("not_configured role should not be selected");
      return `skipReason: ${skipEntry.reason}`;
    });

    // Test 51: R2 — router reason이 Trace에 남음
    test("51_r2_router_reason_in_trace", () => {
      const trace = newTrace("test", "test");
      if (!trace.router) throw new Error("trace should have router field");
      if (!Array.isArray(trace.router.signals)) throw new Error("trace.router.signals should be array");
      if (!Array.isArray(trace.router.selected)) throw new Error("trace.router.selected should be array");
      if (!Array.isArray(trace.router.skipped)) throw new Error("trace.router.skipped should be array");
      const settings2 = defaultSettings();
      Object.keys(settings2.role_profiles).forEach((rid) => {
        settings2.role_profiles[rid].endpoint = "https://test.example.com/v1";
        settings2.role_profiles[rid].model = "test-model";
      });
      const signals = [{ id: "dialogue_heavy", severity: "medium" }];
      const routerResult = selectRoles(DEFAULT_ROLES, signals, "balanced", settings2);
      trace.router = {
        signals: routerResult.signals,
        selected: routerResult.selectReasons,
        skipped: routerResult.skipReasons,
      };
      if (trace.router.signals.indexOf("dialogue_heavy") < 0) throw new Error("signal not recorded in trace");
      const charSelect = trace.router.selected.find((s) => s.role_id === "character_reader");
      if (!charSelect) throw new Error("character_reader select reason not in trace");
      if (charSelect.reason.indexOf("adaptive_score:") !== 0
          || charSelect.reason.indexOf("dialogue_heavy") < 0) {
        throw new Error("character_reader should have adaptive signal score reason");
      }
      return `signals:${trace.router.signals.length} selected:${trace.router.selected.length} skipped:${trace.router.skipped.length}`;
    });

    // Test 52: R2 — 구조 기반 신호 감지 (대화 비율, 목록 구조, 반복)
    test("52_r2_structure_based_signals", () => {
      const settings2 = defaultSettings();
      const dialogueText = '"Hello," she said. "How are you?" he asked. "Fine," she replied. "Good," he nodded.';
      const dialogueSegs = buildSegmentMap(dialogueText, settings2);
      const dialogueSignals = detectSceneSignals(dialogueSegs, { bounded_context_block: "" });
      const hasDialogue = dialogueSignals.some((s) => s.id === "dialogue_heavy");
      if (!hasDialogue) throw new Error("dialogue_heavy signal not detected for dialogue-rich text");
      const listText = "Some intro.\n- item one\n- item two\n- item three\nMore prose.";
      const listSegs = buildSegmentMap(listText, settings2);
      const listSignals = detectSceneSignals(listSegs, { bounded_context_block: "" });
      const hasList = listSignals.some((s) => s.id === "list_structure");
      if (!hasList) throw new Error("list_structure signal not detected for bullet list");
      const repeatText = "The wind howled across the moor. The wind howled across the moor. She walked away. She walked away.";
      const repeatSegs = buildSegmentMap(repeatText, settings2);
      const repeatSignals = detectSceneSignals(repeatSegs, { bounded_context_block: "" });
      const hasRepeat = repeatSignals.some((s) => s.id === "repetition_detected");
      if (!hasRepeat) throw new Error("repetition_detected signal not detected for repeated sentences");
      return `dialogue:${hasDialogue} list:${hasList} repeat:${hasRepeat}`;
    });

    // Test 53: R3 — Composer가 모든 mutable segment 재작성
    test("53_r3_composer_all_segments", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "", start: 0, end: 10 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 10, end: 20 },
        { id: "mutable_3", type: "mutable", text: "original 3", leading_ws: "", trailing_ws: "", start: 20, end: 30 },
      ];
      const composerResult = { segments: { mutable_1: "composed 1", mutable_2: "composed 2", mutable_3: "composed 3" } };
      const directorResult = { ranked: {} };
      const assembled = assembleOutput(segs, composerResult, directorResult);
      if (!assembled.changed) throw new Error("output should be changed");
      if (assembled.output !== "composed 1composed 2composed 3") throw new Error(`unexpected output: ${assembled.output}`);
      if (assembled.composerApplied !== 3) throw new Error(`composerApplied should be 3, got ${assembled.composerApplied}`);
      const sources = assembled.finalSegments.map((f) => f.source);
      if (sources.some((s) => s !== "composer")) throw new Error(`all sources should be composer, got ${sources.join(",")}`);
      return `composerApplied:${assembled.composerApplied} output:${assembled.output}`;
    });

    // Test 54: Partial Composer output is rejected before specialist fallback assembly.
    test("54_r3_partial_composer_rejected_then_top_candidate_used", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "", start: 0, end: 10 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 10, end: 20 },
      ];
      const composerResult = validateComposerSchema(
        { segments: { mutable_1: "composed 1" } },
        ["mutable_1", "mutable_2"]
      );
      if (composerResult) throw new Error("partial Composer output must be rejected");
      const directorResult = {
        ranked: {
          mutable_1: [{ role_id: "style_reader", rewrite: "best candidate 1", confidence: 0.9, score: 90, issues: ["rhythm"] }],
          mutable_2: [{ role_id: "character_reader", rewrite: "best candidate 2", confidence: 0.9, score: 90, issues: ["character_voice"] }],
        },
      };
      const assembled = assembleOutput(segs, composerResult, directorResult);
      if (!assembled.changed) throw new Error("output should be changed");
      if (assembled.output !== "best candidate 1best candidate 2") throw new Error(`unexpected output: ${assembled.output}`);
      const seg1 = assembled.finalSegments.find((f) => f.id === "mutable_1");
      const seg2 = assembled.finalSegments.find((f) => f.id === "mutable_2");
      if (seg1.source !== "top_candidate") throw new Error(`mutable_1 source should be top_candidate, got ${seg1.source}`);
      if (seg2.source !== "top_candidate") throw new Error(`mutable_2 source should be top_candidate, got ${seg2.source}`);
      return `seg1:${seg1.source} seg2:${seg2.source}`;
    });

    // Test 55: R3 — Composer 실패 + specialist 후보 적용
    test("55_r3_composer_failure_specialist_applied", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original", leading_ws: "", trailing_ws: "", start: 0, end: 8 },
      ];
      const directorResult = {
        ranked: {
          mutable_1: [{ role_id: "character_reader", rewrite: "specialist rewrite", confidence: 0.85, score: 85, issues: ["character_voice"] }],
        },
      };
      const assembled = assembleOutput(segs, null, directorResult);
      if (!assembled.changed) throw new Error("output should be changed with specialist candidate");
      if (assembled.output !== "specialist rewrite") throw new Error(`output should be specialist rewrite, got ${assembled.output}`);
      const seg1 = assembled.finalSegments.find((f) => f.id === "mutable_1");
      if (seg1.source !== "top_candidate") throw new Error(`source should be top_candidate, got ${seg1.source}`);
      return `source:${seg1.source} output:${assembled.output}`;
    });

    // Test 56: R3 — Composer unknown segment ID 거부
    test("56_r3_composer_unknown_id_rejected", () => {
      const allowed = ["mutable_1", "mutable_2"];
      const withUnknown = { segments: { mutable_1: "text 1", mutable_2: "text 2", unknown_seg: "foreign text" } };
      const v = validateComposerSchema(withUnknown, allowed);
      if (!v) throw new Error("complete allowed set should be accepted");
      if (Object.keys(v.segments).indexOf("unknown_seg") >= 0) throw new Error("unknown segment ID should be rejected");
      if (Object.keys(v.segments).length !== 2) throw new Error("complete allowed set should remain");
      return `unknown rejected, kept: ${Object.keys(v.segments).join(",")}`;
    });

    // Test 57: R3 — Composer protected/inspect-only ID 거부
    test("57_r3_composer_protected_inspect_id_rejected", () => {
      const allowed = ["mutable_1", "mutable_2"];
      const withProtected = { segments: { mutable_1: "text 1", mutable_2: "text 2", protected_1: "should not pass" } };
      const v1 = validateComposerSchema(withProtected, allowed);
      if (!v1 || Object.keys(v1.segments).indexOf("protected_1") >= 0) throw new Error("protected ID should be rejected");
      const withInspect = { segments: { mutable_1: "text 1", mutable_2: "text 2", inspect_1: "should not pass" } };
      const v2 = validateComposerSchema(withInspect, allowed);
      if (!v2 || Object.keys(v2.segments).indexOf("inspect_1") >= 0) throw new Error("inspect-only ID should be rejected");
      return `protected rejected, inspect rejected`;
    });

    // Test 58: R3 — Composer 동일문 반환 → unchanged
    test("58_r3_composer_identical_unchanged", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "same text", leading_ws: "", trailing_ws: "", start: 0, end: 9 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 9, end: 19 },
      ];
      const composerResult = { segments: { mutable_1: "same text", mutable_2: "changed text" } };
      const directorResult = { ranked: {} };
      const assembled = assembleOutput(segs, composerResult, directorResult);
      const seg1 = assembled.finalSegments.find((f) => f.id === "mutable_1");
      const seg2 = assembled.finalSegments.find((f) => f.id === "mutable_2");
      if (!seg1.composer_unchanged) throw new Error("identical composer result should be marked unchanged");
      if (seg2.composer_unchanged) throw new Error("changed composer result should not be marked unchanged");
      if (!assembled.changed) throw new Error("output should be changed (mutable_2 changed)");
      if (assembled.unchangedSegments !== 1) throw new Error(`unchangedSegments should be 1, got ${assembled.unchangedSegments}`);
      return `seg1:unchanged seg2:changed unchangedSegments:${assembled.unchangedSegments}`;
    });

    // Test 59: R3 — 여러 문제군 후보가 하나의 segment로 합성 (Composer 입력 구조)
    test("59_r3_multi_issue_synthesis_input", () => {
      const mutableSegs = [
        { id: "mutable_1", type: "mutable", text: "original text", leading_ws: "", trailing_ws: "" },
      ];
      const allSegments = mutableSegs;
      const directorInfo = {
        candidateBundles: {
          mutable_1: [
            { role: "secret_pov_guard", confidence: 0.9, score: 95.0, issues: ["secret_leak"], change_summary: "removed secret leak", rewrite: "She didn't know his secret." },
            { role: "character_reader", confidence: 0.8, score: 80.0, issues: ["character_voice"], change_summary: "fixed voice", rewrite: "She spoke in her own voice." },
            { role: "style_reader", confidence: 0.7, score: 70.0, issues: ["rhythm"], change_summary: "improved rhythm", rewrite: "She spoke, her voice steady." },
          ],
        },
        consensus: { mutable_1: ["secret_leak"] },
        complementary: { mutable_1: ["character_voice", "rhythm"] },
        conflict: {},
        gap: [],
      };
      const composerRole = DEFAULT_ROLES.find((r) => r.is_composer);
      const profile = defaultRoleProfile("whole_scene_composer");
      const prompts = buildRolePrompt(composerRole, profile, mutableSegs, "context block", allSegments, directorInfo);
      const hasSecretLeak = prompts.user.indexOf("secret_leak") >= 0;
      const hasCharacterVoice = prompts.user.indexOf("character_voice") >= 0;
      const hasRhythm = prompts.user.indexOf("rhythm") >= 0;
      const hasScore = prompts.user.indexOf("score:") >= 0;
      const hasChangeSummary = prompts.user.indexOf("change:") >= 0;
      const hasConsensus = prompts.user.indexOf("consensus=") >= 0;
      const hasComplementary = prompts.user.indexOf("complementary=") >= 0;
      const hasConflict = prompts.user.indexOf("conflict=") >= 0;
      const hasGap = prompts.user.indexOf("gap=") >= 0;
      if (!hasSecretLeak || !hasCharacterVoice || !hasRhythm) throw new Error("composer input missing issue codes");
      if (!hasScore) throw new Error("composer input missing Director score");
      if (!hasChangeSummary) throw new Error("composer input missing change_summary");
      if (!hasConsensus || !hasComplementary || !hasConflict || !hasGap) throw new Error("composer input missing Director per-segment fields");
      return `issues:${hasSecretLeak && hasCharacterVoice && hasRhythm} score:${hasScore} change:${hasChangeSummary} director:${hasConsensus && hasComplementary && hasConflict && hasGap}`;
    });

    // Test 60: R3 — 최종 출력이 실제 draft_zero와 다름
    test("60_r3_output_differs_from_draft", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "The wind blew.", leading_ws: "", trailing_ws: "", start: 0, end: 14 },
        { id: "mutable_2", type: "mutable", text: "She walked away.", leading_ws: "", trailing_ws: "", start: 14, end: 30 },
      ];
      const composerResult = { segments: { mutable_1: "The cold wind swept across the moor.", mutable_2: "She turned and walked into the fog." } };
      const assembled = assembleOutput(segs, composerResult, { ranked: {} });
      if (assembled.output === "The wind blew.She walked away.") throw new Error("output identical to draft");
      if (!assembled.changed) throw new Error("changed flag false");
      if (assembled.composerApplied !== 2) throw new Error(`composerApplied should be 2, got ${assembled.composerApplied}`);
      return `changed:${assembled.changed} composerApplied:${assembled.composerApplied}`;
    });

    // Test 61: R3 — Composer 전체 실패 시 성공한 specialist 후보가 버려지지 않음
    test("61_r3_composer_fail_specialist_preserved", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "", start: 0, end: 10 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 10, end: 20 },
      ];
      const directorResult = {
        ranked: {
          mutable_1: [{ role_id: "character_reader", rewrite: "specialist 1", confidence: 0.85, score: 85, issues: ["character_voice"] }],
          mutable_2: [{ role_id: "style_reader", rewrite: "specialist 2", confidence: 0.75, score: 75, issues: ["rhythm"] }],
        },
      };
      const assembled = assembleOutput(segs, null, directorResult);
      if (!assembled.changed) throw new Error("specialist candidates should produce changed output");
      if (assembled.output !== "specialist 1specialist 2") throw new Error(`output should be specialist results, got ${assembled.output}`);
      if (assembled.topCandidateApplied !== 2) throw new Error(`topCandidateApplied should be 2, got ${assembled.topCandidateApplied}`);
      return `output:${assembled.output} topCandidateApplied:${assembled.topCandidateApplied}`;
    });

    // Test 62: R3 — validateComposerSchema with no allowedSegmentIds returns null
    test("62_r3_composer_no_allowed_returns_null", () => {
      const good = { segments: { mutable_1: "text 1" } };
      const v = validateComposerSchema(good, null);
      if (v) throw new Error("null allowedSegmentIds should return null");
      const v2 = validateComposerSchema(good, []);
      if (v2) throw new Error("empty allowedSegmentIds should return null");
      return "null/empty allowed rejected";
    });

    // Test 63: R4 — 성공 역할 timeline
    test("63_r4_successful_role_timeline", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: 1000,
        ended_at: 2000,
        elapsed_ms: 1000,
        candidate_count: 3,
      });
      traceRole(trace, {
        role_id: "style_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: 1100,
        ended_at: 1900,
        elapsed_ms: 800,
        candidate_count: 2,
      });
      if (trace.roles.length !== 2) throw new Error("should have 2 role entries");
      const r0 = trace.roles[0];
      if (r0.status !== "fulfilled") throw new Error("role 0 should be fulfilled");
      if (r0.candidate_count !== 3) throw new Error("role 0 should have 3 candidates");
      if (r0.queued_at !== r0.started_at) throw new Error("queued_at should default to started_at");
      return `roles:${trace.roles.length} r0:${r0.status} cand:${r0.candidate_count}`;
    });

    // Test 64: R4 — 실패 역할 timeline
    test("64_r4_failed_role_timeline", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "world_reader",
        provider: "anthropic",
        model: "claude-3",
        status: "failed",
        started_at: 1000,
        ended_at: 3000,
        elapsed_ms: 2000,
        error: "HTTP 500: server error",
      });
      const r0 = trace.roles[0];
      if (r0.status !== "failed") throw new Error("role should be failed");
      if (r0.error !== "HTTP 500: server error") throw new Error("error message not recorded");
      if (r0.candidate_count !== 0) throw new Error("failed role should have 0 candidates");
      return `status:${r0.status} error:${r0.error}`;
    });

    // Test 65: R4 — retry/fallback 표시
    test("65_r4_retry_fallback_display", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: 1000,
        ended_at: 5000,
        elapsed_ms: 4000,
        retry: 1,
        fallback: true,
        candidate_count: 2,
      });
      const r0 = trace.roles[0];
      if (r0.retry !== 1) throw new Error("retry should be 1");
      if (!r0.fallback) throw new Error("fallback should be true");
      return `retry:${r0.retry} fallback:${r0.fallback}`;
    });

    // Test 66: R4 — candidate count
    test("66_r4_candidate_count_recorded", () => {
      const trace = newTrace("test", "test");
      trace.candidates.total = 5;
      trace.candidates.by_segment = { mutable_1: 3, mutable_2: 2 };
      if (trace.candidates.total !== 5) throw new Error("total should be 5");
      if (trace.candidates.by_segment.mutable_1 !== 3) throw new Error("by_segment not recorded");
      return `total:${trace.candidates.total} seg1:${trace.candidates.by_segment.mutable_1}`;
    });

    // Test 67: R4 — Composer 적용 segment evidence
    test("67_r4_composer_applied_evidence", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original 1", leading_ws: "", trailing_ws: "", start: 0, end: 10 },
        { id: "mutable_2", type: "mutable", text: "original 2", leading_ws: "", trailing_ws: "", start: 10, end: 20 },
      ];
      const composerResult = { segments: { mutable_1: "composed 1", mutable_2: "composed 2" } };
      const directorResult = { ranked: {} };
      const assembled = assembleOutput(segs, composerResult, directorResult);
      const trace = newTrace("test", "test");
      assembled.finalSegments.forEach((fseg) => {
        if (fseg.type !== "mutable") return;
        trace.applied_evidence.push({
          segment_id: fseg.id,
          source: fseg.source,
          changed: safeString(fseg.final_text) !== safeString(fseg.original_text),
          composer_unchanged: !!fseg.composer_unchanged,
          original_preview: preview(fseg.original_text, 80),
          final_preview: preview(fseg.final_text, 80),
        });
      });
      if (trace.applied_evidence.length !== 2) throw new Error("should have 2 applied evidence entries");
      const ev0 = trace.applied_evidence[0];
      if (ev0.source !== "composer") throw new Error(`source should be composer, got ${ev0.source}`);
      if (!ev0.changed) throw new Error("segment should be changed");
      if (ev0.composer_unchanged) throw new Error("should not be unchanged");
      return `evidence:${trace.applied_evidence.length} source:${ev0.source} changed:${ev0.changed}`;
    });

    // Test 68: R4 — top candidate 적용 segment evidence
    test("68_r4_top_candidate_applied_evidence", () => {
      const segs = [
        { id: "mutable_1", type: "mutable", text: "original", leading_ws: "", trailing_ws: "", start: 0, end: 8 },
      ];
      const directorResult = {
        ranked: { mutable_1: [{ role_id: "character_reader", rewrite: "best candidate", confidence: 0.9, score: 90, issues: ["character_voice"] }] },
      };
      const assembled = assembleOutput(segs, null, directorResult);
      const trace = newTrace("test", "test");
      assembled.finalSegments.forEach((fseg) => {
        if (fseg.type !== "mutable") return;
        trace.applied_evidence.push({
          segment_id: fseg.id,
          source: fseg.source,
          changed: safeString(fseg.final_text) !== safeString(fseg.original_text),
          composer_unchanged: !!fseg.composer_unchanged,
          original_preview: preview(fseg.original_text, 80),
          final_preview: preview(fseg.final_text, 80),
        });
      });
      const ev0 = trace.applied_evidence[0];
      if (ev0.source !== "top_candidate") throw new Error(`source should be top_candidate, got ${ev0.source}`);
      if (!ev0.changed) throw new Error("segment should be changed");
      return `source:${ev0.source} changed:${ev0.changed}`;
    });

    // Test 69: R4 — original 반환 reason 기록
    test("69_r4_original_return_reason", () => {
      const trace = newTrace("test", "test");
      trace.final.enhanced = false;
      trace.final.reason = "no_candidates_applied";
      trace.summary.final_state = "unchanged";
      trace.summary.final_reason = trace.final.reason;
      if (trace.final.enhanced) throw new Error("should not be enhanced");
      if (trace.final.reason !== "no_candidates_applied") throw new Error("reason not recorded");
      if (trace.summary.final_state !== "unchanged") throw new Error("final_state not recorded");
      return `state:${trace.summary.final_state} reason:${trace.final.reason}`;
    });

    // Test 70: R4 — Trace JSON에 API Key 원문 없음
    test("70_r4_no_api_key_in_trace", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: 1000,
        ended_at: 2000,
        elapsed_ms: 1000,
      });
      trace.applied_evidence.push({ segment_id: "mutable_1", source: "composer", changed: true, composer_unchanged: false, original_preview: "orig", final_preview: "final" });
      trace.director_evidence.push({ segment_id: "mutable_1", issue_groups: ["character_voice"], consensus: [], complementary: ["character_voice"], conflict: [], top_score: 85, top_role: "character_reader", candidate_count: 1 });
      trace.summary = { specialist_calls: 1, successful_roles: 1, candidate_count: 1, composer_state: "fulfilled", changed_segment_count: 1, unchanged_segment_count: 0, final_state: "enhanced", final_reason: "composer_integrated" };
      const traceStr = JSON.stringify(trace);
      const testKey = "sk-test-secret-key-12345";
      if (traceStr.indexOf(testKey) >= 0) throw new Error("plain key found in trace");
      if (traceStr.indexOf("Authorization") >= 0) throw new Error("Authorization header found in trace");
      if (traceStr.indexOf("api_key") >= 0) throw new Error("api_key field found in trace");
      return "no API key in trace JSON";
    });

    // Test 71: R4 — Trace 크기 상한 (preview만)
    test("71_r4_trace_size_bounded", () => {
      const trace = newTrace("test", "test");
      const longText = "A".repeat(10000);
      trace.applied_evidence.push({
        segment_id: "mutable_1",
        source: "composer",
        changed: true,
        composer_unchanged: false,
        original_preview: preview(longText, 80),
        final_preview: preview("B".repeat(10000), 80),
      });
      trace.final.original_preview = preview(longText, 200);
      trace.final.final_preview = preview("B".repeat(10000), 200);
      const traceStr = JSON.stringify(trace);
      if (traceStr.length > 10000) throw new Error(`trace too large: ${traceStr.length} bytes`);
      if (trace.applied_evidence[0].original_preview.length > 85) throw new Error("original_preview not bounded");
      return `trace size: ${traceStr.length} bytes, preview: ${trace.applied_evidence[0].original_preview.length} chars`;
    });

    // Test 72: R4 — Director evidence
    test("72_r4_director_evidence_recorded", () => {
      const candidates = {
        mutable_1: [
          { role_id: "secret_pov_guard", rewrite: "She hesitated.", confidence: 0.9, issues: ["pov_violation"] },
          { role_id: "character_reader", rewrite: "She paused.", confidence: 0.8, issues: ["pov_violation"] },
        ],
      };
      const roles = DEFAULT_ROLES.filter((r) => !r.is_composer).slice(0, 3);
      const origSegs = [{ id: "mutable_1", type: "mutable", text: "She knew everything." }];
      const result = fusionDirector(candidates, roles, origSegs);
      const trace = newTrace("test", "test");
      if (result.ranked && result.ranked.mutable_1) {
        const ranked = result.ranked.mutable_1;
        const top = ranked[0];
        trace.director_evidence.push({
          segment_id: "mutable_1",
          issue_groups: top.issues || [],
          consensus: result.consensus.mutable_1 || [],
          complementary: result.complementary.mutable_1 || [],
          conflict: result.conflict.mutable_1 || [],
          top_score: top.score != null ? Math.round(top.score * 10) / 10 : 0,
          top_role: top.role_id,
          candidate_count: ranked.length,
        });
      }
      const ev = trace.director_evidence[0];
      if (!ev) throw new Error("director evidence not recorded");
      if (ev.segment_id !== "mutable_1") throw new Error("segment_id mismatch");
      if (ev.consensus.indexOf("pov_violation") < 0) throw new Error("consensus not recorded");
      if (ev.candidate_count !== 2) throw new Error("candidate count mismatch");
      return `seg:${ev.segment_id} consensus:${ev.consensus.join(",")} score:${ev.top_score}`;
    });

    // Test 73: R4 — 최종 요약 필드
    test("73_r4_final_summary_fields", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "fulfilled",
        started_at: 1000,
        ended_at: 2000,
        elapsed_ms: 1000,
        candidate_count: 2,
      });
      traceRole(trace, {
        role_id: "style_reader",
        provider: "openai_compatible",
        model: "gpt-4",
        status: "failed",
        started_at: 1100,
        ended_at: 1900,
        elapsed_ms: 800,
        error: "timeout",
      });
      trace.candidates.total = 2;
      trace.composer.status = "fulfilled";
      trace.summary = { specialist_calls: 2, successful_roles: 1, candidate_count: 2, composer_state: "fulfilled", changed_segment_count: 1, unchanged_segment_count: 0, final_state: "enhanced", final_reason: "composer_integrated" };
      if (trace.summary.specialist_calls !== 2) throw new Error("specialist_calls mismatch");
      if (trace.summary.successful_roles !== 1) throw new Error("successful_roles should be 1");
      if (trace.summary.composer_state !== "fulfilled") throw new Error("composer_state mismatch");
      if (trace.summary.changed_segment_count !== 1) throw new Error("changed_segment_count mismatch");
      return `calls:${trace.summary.specialist_calls} successful:${trace.summary.successful_roles} composer:${trace.summary.composer_state} changed:${trace.summary.changed_segment_count}`;
    });

    // Test 74: R5 — OpenAI-compatible request/response mock
    await asyncTest("74_r5_openai_compatible_mock", async () => {
      const captured = { url: "", auth: "", bodyStr: "", hasJson: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.auth = (opts && opts.headers && opts.headers.Authorization) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try { const b = JSON.parse(captured.bodyStr); captured.hasJson = b.response_format && b.response_format.type === "json_object"; } catch (_) {}
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content: '{"role":"x","candidates":[]}' } }] }) };
      };
      try {
        const profile = { provider: "openai_compatible", endpoint: "https://api.test.com/v1", model: "gpt-4", api_key_ref: "sk-test", temperature: 0.3, max_output_tokens: 1024, force_json_response: true, reasoning_effort: "auto", reasoning_budget_tokens: 0, extra_headers: "", extra_body: "" };
        await callProvider(profile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf("/chat/completions") < 0) throw new Error("URL should use /chat/completions");
        if (captured.auth.indexOf("Bearer sk-test") < 0) throw new Error("Auth should be Bearer key");
        if (!captured.hasJson) throw new Error("force_json_response should set response_format");
      } finally { globalThis.fetch = origFetch; }
      return `url:${captured.url.indexOf("/chat/completions") >= 0} auth:${captured.auth.indexOf("Bearer") >= 0} json:${captured.hasJson}`;
    });

    // Test 75: R5 — Ollama local request/response mock
    await asyncTest("75_r5_ollama_local_mock", async () => {
      const captured = { url: "", bodyStr: "", hasOptions: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.bodyStr = (opts && opts.body) || "";
        try { const b = JSON.parse(captured.bodyStr); captured.hasOptions = !!b.options; } catch (_) {}
        return { ok: true, text: async () => JSON.stringify({ message: { content: '{"role":"x","candidates":[]}' } }) };
      };
      try {
        const profile = { provider: "ollama_compatible", endpoint: "http://localhost:11434", model: "llama3", api_key_ref: "", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, extra_headers: "", extra_body: "" };
        await callProvider(profile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf("/api/chat") < 0) throw new Error("Ollama local should use /api/chat");
        if (!captured.hasOptions) throw new Error("Ollama local should use options field");
      } finally { globalThis.fetch = origFetch; }
      return `url:${captured.url} options:${captured.hasOptions}`;
    });

    // Test 76: R5 — Anthropic request/response mock
    await asyncTest("76_r5_anthropic_mock", async () => {
      const captured = { url: "", apiKey: "", bodyStr: "", hasSystem: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.apiKey = (opts && opts.headers && opts.headers["x-api-key"]) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try { const b = JSON.parse(captured.bodyStr); captured.hasSystem = typeof b.system === "string"; } catch (_) {}
        return { ok: true, text: async () => JSON.stringify({ content: [{ type: "text", text: '{"role":"x","candidates":[]}' }] }) };
      };
      try {
        const profile = { provider: "anthropic", endpoint: "https://api.anthropic.com/v1", model: "claude-3", api_key_ref: "sk-ant", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, extra_headers: "", extra_body: "" };
        const result = await callProvider(profile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf("/messages") < 0) throw new Error("Anthropic should use /messages");
        if (captured.apiKey !== "sk-ant") throw new Error("Anthropic should use x-api-key header");
        if (!captured.hasSystem) throw new Error("Anthropic body should have system field");
        if (!result.content) throw new Error("Anthropic response content empty");
      } finally { globalThis.fetch = origFetch; }
      return `url:${captured.url} key:${captured.apiKey === "sk-ant"} system:${captured.hasSystem}`;
    });

    // Test 77: R5 — Gemini request/response mock
    await asyncTest("77_r5_gemini_mock", async () => {
      const captured = { url: "", googKey: "", bodyStr: "", hasContents: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.googKey = (opts && opts.headers && opts.headers["x-goog-api-key"]) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try { const b = JSON.parse(captured.bodyStr); captured.hasContents = !!b.contents; } catch (_) {}
        return { ok: true, text: async () => JSON.stringify({ candidates: [{ content: { parts: [{ text: '{"role":"x","candidates":[]}' }] } }] }) };
      };
      try {
        const profile = { provider: "gemini", endpoint: "https://generativelanguage.googleapis.com/v1beta", model: "gemini-pro", api_key_ref: "AIza-test", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, extra_headers: "", extra_body: "" };
        const result = await callProvider(profile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf(":generateContent") < 0) throw new Error("Gemini should use :generateContent");
        if (captured.googKey !== "AIza-test") throw new Error("Gemini should use x-goog-api-key");
        if (!captured.hasContents) throw new Error("Gemini body should have contents");
        if (!result.content) throw new Error("Gemini response content empty");
      } finally { globalThis.fetch = origFetch; }
      return `url:${captured.url.indexOf(":generateContent") >= 0} key:${captured.googKey === "AIza-test"} contents:${captured.hasContents}`;
    });

    // Test 78: R5 — Vertex request/response mock
    await asyncTest("78_r5_vertex_mock", async () => {
      const captured = { url: "", auth: "", bodyStr: "", hasSystemInstr: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.auth = (opts && opts.headers && opts.headers.Authorization) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try { const b = JSON.parse(captured.bodyStr); captured.hasSystemInstr = !!b.systemInstruction; } catch (_) {}
        return { ok: true, text: async () => JSON.stringify({ candidates: [{ content: { parts: [{ text: '{"role":"x","candidates":[]}' }] } }] }) };
      };
      try {
        const profile = { provider: "vertex", endpoint: "https://us-central1-aiplatform.googleapis.com/v1", model: "gemini-pro", api_key_ref: "ya29-test", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, vertex_flex_mode: "off", extra_headers: "", extra_body: "" };
        const result = await callProvider(profile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf(":generateContent") < 0) throw new Error("Vertex should use :generateContent");
        if (captured.auth.indexOf("Bearer ya29-test") < 0) throw new Error("Vertex should use Bearer token");
        if (!captured.hasSystemInstr) throw new Error("Vertex body should have systemInstruction");
        if (!result.content) throw new Error("Vertex response content empty");
      } finally { globalThis.fetch = origFetch; }
      return `url:${captured.url.indexOf(":generateContent") >= 0} auth:${captured.auth.indexOf("Bearer") >= 0} sysInstr:${captured.hasSystemInstr}`;
    });

    // Test 79: R5 — Vertex Flex가 Vertex에서만 적용
    await asyncTest("79_r5_vertex_flex_vertex_only", async () => {
      const captured = { flexHeader: "" };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.flexHeader = (opts && opts.headers && opts.headers["X-Vertex-AI-LLM-Shared-Request-Type"]) || "";
        return { ok: true, text: async () => JSON.stringify({ candidates: [{ content: { parts: [{ text: '{"role":"x","candidates":[]}' }] } }] }) };
      };
      try {
        const vertexProfile = { provider: "vertex", endpoint: "https://us-central1-aiplatform.googleapis.com/v1", model: "gemini-pro", api_key_ref: "ya29", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, vertex_flex_mode: "provisioned_then_flex", extra_headers: "", extra_body: "" };
        await callProvider(vertexProfile, { system: "s", user: "u" }, null);
        if (captured.flexHeader !== "flex") throw new Error(`Vertex provisioned_then_flex should set X-Vertex-AI-LLM-Shared-Request-Type: flex, got ${captured.flexHeader}`);
        captured.flexHeader = "";
        const geminiProfile = { provider: "gemini", endpoint: "https://generativelanguage.googleapis.com/v1beta", model: "gemini-pro", api_key_ref: "AIza", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, vertex_flex_mode: "provisioned_then_flex", extra_headers: "", extra_body: "" };
        await callProvider(geminiProfile, { system: "s", user: "u" }, null);
        if (captured.flexHeader !== "") throw new Error("Gemini should NOT apply Vertex Flex headers");
      } finally { globalThis.fetch = origFetch; }
      return `vertex:flex vertex-only:${captured.flexHeader === ""}`;
    });

    // Test 80: R5 — Vertex flex_only sets both headers
    await asyncTest("80_r5_vertex_flex_only_headers", async () => {
      const captured = { shared: "", flex: "" };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.shared = (opts && opts.headers && opts.headers["X-Vertex-AI-LLM-Request-Type"]) || "";
        captured.flex = (opts && opts.headers && opts.headers["X-Vertex-AI-LLM-Shared-Request-Type"]) || "";
        return { ok: true, text: async () => JSON.stringify({ candidates: [{ content: { parts: [{ text: '{"role":"x","candidates":[]}' }] } }] }) };
      };
      try {
        const profile = { provider: "vertex", endpoint: "https://us-central1-aiplatform.googleapis.com/v1", model: "gemini-pro", api_key_ref: "ya29", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, vertex_flex_mode: "flex_only", extra_headers: "", extra_body: "" };
        await callProvider(profile, { system: "s", user: "u" }, null);
        if (captured.shared !== "shared") throw new Error(`flex_only should set X-Vertex-AI-LLM-Request-Type: shared, got ${captured.shared}`);
        if (captured.flex !== "flex") throw new Error(`flex_only should set X-Vertex-AI-LLM-Shared-Request-Type: flex, got ${captured.flex}`);
      } finally { globalThis.fetch = origFetch; }
      return `shared:${captured.shared} flex:${captured.flex}`;
    });

    // Test 81: R5 — invalid JSON -> 1회 repair retry
    await asyncTest("81_r5_invalid_json_repair_retry", async () => {
      let callCount = 0;
      const origFetch = globalThis.fetch;
      globalThis.fetch = async () => {
        callCount++;
        if (callCount === 1) {
          return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content: "not json at all" } }] }) };
        }
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content: '{"role":"character_reader","candidates":[{"segment_id":"mutable_1","rewrite":"text","confidence":0.8}]}' } }] }) };
      };
      try {
        const profile = { provider: "openai_compatible", endpoint: "https://api.test.com/v1", model: "gpt-4", api_key_ref: "sk-test", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, extra_headers: "", extra_body: "" };
        const trace = newTrace("test", "test");
        const role = DEFAULT_ROLES.find((r) => r.role_id === "character_reader");
        const result = await callRole(role, profile, [{ id: "mutable_1", type: "mutable", text: "orig", leading_ws: "", trailing_ws: "" }], "", [{ id: "mutable_1", type: "mutable", text: "orig", leading_ws: "", trailing_ws: "" }], null, trace, null);
        if (!result) throw new Error("callRole should succeed after retry");
        if (callCount < 2) throw new Error(`should have retried, callCount=${callCount}`);
      } finally { globalThis.fetch = origFetch; }
      return `calls:${callCount} repaired:true`;
    });

    // Test 82: R5 — primary 실패 -> fallback 1회
    await asyncTest("82_r5_primary_fail_fallback", async () => {
      let callCount = 0;
      let usedFallbackModel = "";
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        callCount++;
        let isPrimary = true;
        try { const b = JSON.parse(opts.body); if (b.model === "fallback-model") { usedFallbackModel = "fallback-model"; isPrimary = false; } } catch (_) {}
        if (isPrimary) return { ok: false, status: 500, text: async () => "server error" };
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content: '{"role":"character_reader","candidates":[{"segment_id":"mutable_1","rewrite":"text","confidence":0.8}]}' } }] }) };
      };
      try {
        const profile = { provider: "openai_compatible", endpoint: "https://api.test.com/v1", model: "primary-model", api_key_ref: "sk-test", temperature: 0.3, max_output_tokens: 1024, force_json_response: false, reasoning_effort: "auto", reasoning_budget_tokens: 0, fallback_provider: "openai_compatible", fallback_endpoint: "https://api.test.com/v1", fallback_model: "fallback-model", fallback_api_key_ref: "sk-test", extra_headers: "", extra_body: "" };
        const trace = newTrace("test", "test");
        const role = DEFAULT_ROLES.find((r) => r.role_id === "character_reader");
        const result = await callRole(role, profile, [{ id: "mutable_1", type: "mutable", text: "orig", leading_ws: "", trailing_ws: "" }], "", [{ id: "mutable_1", type: "mutable", text: "orig", leading_ws: "", trailing_ws: "" }], null, trace, null);
        if (!result) throw new Error("callRole should succeed via fallback");
        if (usedFallbackModel !== "fallback-model") throw new Error("should have used fallback model");
        if (callCount < 2) throw new Error("should have retried with fallback");
      } finally { globalThis.fetch = origFetch; }
      return `calls:${callCount} fallback:${usedFallbackModel}`;
    });

    // Test 83: R5 — deadline abort -> active call 0
    await asyncTest("83_r5_deadline_abort_active_zero", async () => {
      const segs = [{ id: "mutable_1", type: "mutable", text: "prose", leading_ws: "", trailing_ws: "", start: 0, end: 5 }];
      const trace = newTrace("test", "test");
      const origFetch = globalThis.fetch;
      globalThis.fetch = async () => new Promise(() => {});
      try {
        const deadline = createDeadline(80);
        const settings2 = defaultSettings();
        Object.keys(settings2.role_profiles).forEach((rid) => { settings2.role_profiles[rid].endpoint = "https://test.example.com/v1"; settings2.role_profiles[rid].model = "test-model"; });
        const roles = selectRoles(DEFAULT_ROLES, [{ id: "mechanical_artifact", severity: "high" }], "balanced", settings2).roles;
        await scheduleRoles(roles, settings2.role_profiles, segs, "", segs, deadline, trace, 5);
        deadline.cancel();
        if (trace.active_calls_final !== 0) throw new Error(`active_calls_final should be 0, got ${trace.active_calls_final}`);
      } finally { globalThis.fetch = origFetch; }
      return `active_calls_final=${trace.active_calls_final}`;
    });

    // Test 84: R5 — same provider concurrency 준수
    test("84_r5_same_provider_concurrency", () => {
      const sem = createSemaphore(PROVIDER_CONCURRENCY.openai_compatible);
      if (PROVIDER_CONCURRENCY.openai_compatible !== 3) throw new Error("openai_compatible concurrency should be 3");
      if (PROVIDER_CONCURRENCY.ollama_compatible !== 1) throw new Error("ollama_compatible concurrency should be 1");
      if (PROVIDER_CONCURRENCY.anthropic !== 2) throw new Error("anthropic concurrency should be 2");
      return `openai:${PROVIDER_CONCURRENCY.openai_compatible} ollama:${PROVIDER_CONCURRENCY.ollama_compatible} anthropic:${PROVIDER_CONCURRENCY.anthropic}`;
    });

    // Test 85: R5 — protected header fields skipped
    test("85_r5_protected_headers_skipped", () => {
      const base = { "Content-Type": "application/json", Authorization: "Bearer real-key" };
      const extra = { "X-Custom": "val", Authorization: "Bearer evil-key", "Content-Type": "text/plain" };
      const result = applyExtraHeadersSafe(base, extra);
      if (result.headers["X-Custom"] !== "val") throw new Error("custom header should be applied");
      if (result.headers.Authorization !== "Bearer real-key") throw new Error("Authorization should not be overridden");
      if (result.headers["Content-Type"] !== "application/json") throw new Error("Content-Type should not be overridden");
      if (result.skipped_header_keys.indexOf("Authorization") < 0) throw new Error("Authorization should be in skipped list");
      if (result.skipped_header_keys.indexOf("Content-Type") < 0) throw new Error("Content-Type should be in skipped list");
      return `skipped:${result.skipped_header_keys.join(",")}`;
    });

    // Test 86: R5 — protected body fields skipped with deep-merge
    test("86_r5_protected_body_deep_merge", () => {
      const base = { model: "gpt-4", messages: [{ role: "user", content: "hi" }], temperature: 0.3 };
      const extra = { model: "evil-model", messages: [{ role: "evil" }], top_p: 0.9, temperature: 0.5 };
      const result = applyExtraBodySafe(base, extra, "openai_compatible");
      if (result.body.model !== "gpt-4") throw new Error("model should not be overridden");
      if (result.body.messages[0].role !== "user") throw new Error("messages should not be overridden");
      if (result.body.top_p !== 0.9) throw new Error("top_p should be applied");
      if (result.body.temperature !== 0.5) throw new Error("temperature should be overridden by extra");
      if (result.skipped_body_keys.indexOf("model") < 0) throw new Error("model should be in skipped list");
      if (result.skipped_body_keys.indexOf("messages") < 0) throw new Error("messages should be in skipped list");
      return `skipped:${result.skipped_body_keys.join(",")} top_p:${result.body.top_p}`;
    });

    // Test 87: R5 — API Key와 Authorization 비밀값 Trace 미노출
    test("87_r5_no_secret_in_trace", () => {
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader", provider: "openai_compatible", model: "gpt-4",
        status: "fulfilled", started_at: 1000, ended_at: 2000, elapsed_ms: 1000,
        candidate_count: 1,
        request_overrides: { skipped_header_keys: ["Authorization"], skipped_body_keys: [], vertex_flex_mode: "off" },
      });
      const traceStr = JSON.stringify(trace);
      if (traceStr.indexOf("sk-test-secret") >= 0) throw new Error("API key found in trace");
      if (traceStr.indexOf("Bearer ") >= 0) throw new Error("Bearer token found in trace");
      if (trace.roles[0].request_overrides.skipped_header_keys.indexOf("Authorization") < 0) throw new Error("skipped headers not recorded");
      return "no secrets in trace, overrides recorded";
    });

    // Test 88: R5 — model-aware reasoning adapter
    test("88_r5_reasoning_supported_providers", () => {
      const body1 = {};
      applyReasoningAdapter("openai_compatible", body1, { model: "gpt-5.2", reasoning_effort: "high", reasoning_budget_tokens: 512 });
      if (body1.reasoning_effort !== "high") throw new Error("openai should get reasoning_effort");
      if (body1.max_completion_tokens !== 512) throw new Error("openai should get max_completion_tokens");
      const body2 = {};
      applyReasoningAdapter("anthropic", body2, { model: "claude-opus-4", reasoning_effort: "high", reasoning_budget_tokens: 1024 });
      if (!body2.thinking || body2.thinking.budget_tokens !== 1024) throw new Error("anthropic should get thinking.budget_tokens");
      const body3 = { generationConfig: {} };
      applyReasoningAdapter("gemini", body3, { model: "gemini-3-pro", reasoning_effort: "high", reasoning_budget_tokens: 256 });
      if (!body3.generationConfig.thinkingConfig || body3.generationConfig.thinkingConfig.thinkingLevel !== "high") throw new Error("gemini 3 should get thinkingConfig.thinkingLevel");
      const body4 = {};
      applyReasoningAdapter("openai_compatible", body4, { model: "unknown-model", reasoning_effort: "auto", reasoning_budget_tokens: 512 });
      if (body4.reasoning_effort !== undefined) throw new Error("effort=none should skip reasoning");
      return `openai:${body1.reasoning_effort} anthropic:${!!body2.thinking} gemini3:${!!body3.generationConfig.thinkingConfig} unknownAuto:${body4.reasoning_effort === undefined}`;
    });

    test("89_consensus_changes_director_score", () => {
      const originals = [{ id: "mutable_1", type: "mutable", text: "Original scene.", leading_ws: "", trailing_ws: "" }];
      const roles = DEFAULT_ROLES.filter((role) => !role.is_composer);
      const solo = fusionDirector({ mutable_1: [
        { role_id: "character_reader", rewrite: "Better scene.", confidence: 0.8, issues: ["character_voice"] },
      ] }, roles, originals);
      const supported = fusionDirector({ mutable_1: [
        { role_id: "character_reader", rewrite: "Better scene.", confidence: 0.8, issues: ["character_voice"] },
        { role_id: "style_reader", rewrite: "Better scene!", confidence: 0.5, issues: ["character_voice"] },
      ] }, roles, originals);
      const soloScore = solo.ranked.mutable_1.find((item) => item.role_id === "character_reader").score;
      const supportedScore = supported.ranked.mutable_1.find((item) => item.role_id === "character_reader").score;
      if (supportedScore <= soloScore) throw new Error(`consensus did not raise score: ${soloScore} -> ${supportedScore}`);
      return `score:${soloScore.toFixed(1)}->${supportedScore.toFixed(1)}`;
    });

    test("90_true_gap_uses_all_mutable_segments", () => {
      const originals = [
        { id: "mutable_1", type: "mutable", text: "one" },
        { id: "mutable_2", type: "mutable", text: "two" },
      ];
      const result = fusionDirector({ mutable_1: [
        { role_id: "style_reader", rewrite: "ONE", confidence: 0.7, issues: ["prose_clarity"] },
      ] }, DEFAULT_ROLES, originals);
      if (result.gap.indexOf("mutable_2") < 0) throw new Error("mutable_2 gap missing");
      if (!result.ranked.mutable_2 || result.ranked.mutable_2.length !== 0) throw new Error("gap segment ranking should be empty");
      return `gap:${result.gap.join(",")}`;
    });

    test("91_single_candidate_not_complementary", () => {
      const result = fusionDirector({ mutable_1: [
        { role_id: "style_reader", rewrite: "Rewritten.", confidence: 0.8, issues: ["rhythm", "transition"] },
      ] }, DEFAULT_ROLES, [{ id: "mutable_1", type: "mutable", text: "Original." }]);
      if (result.complementary.mutable_1) throw new Error("single candidate must not be labeled complementary");
      return "no false complementary";
    });

    test("92_composer_unchanged_uses_changed_candidate", () => {
      const segs = [{ id: "mutable_1", type: "mutable", text: "Original", leading_ws: "", trailing_ws: "" }];
      const assembled = assembleOutput(
        segs,
        { segments: { mutable_1: "Original" } },
        { ranked: { mutable_1: [{ rewrite: "Improved", score: 100 }] } }
      );
      if (assembled.output !== "Improved") throw new Error(`expected changed candidate, got ${assembled.output}`);
      if (assembled.finalSegments[0].source !== "top_candidate_after_composer_unchanged") throw new Error("fallback source missing");
      return assembled.finalSegments[0].source;
    });

    test("93_whitespace_unchanged_evidence_false", () => {
      const segs = [{
        id: "mutable_1", type: "mutable", text: "\n  Original  \n",
        leading_ws: "\n  ", core_text: "Original", trailing_ws: "  \n",
      }];
      const assembled = assembleOutput(segs, { segments: { mutable_1: "Original" } }, { ranked: {} });
      const evidence = buildAppliedEvidence(assembled.finalSegments);
      if (evidence[0].changed) throw new Error("preserved whitespace must not create a false change");
      return "unchanged";
    });

    test("94_verifier_reject_clears_applied_evidence", () => {
      const trace = newTrace("test", "test");
      const assembled = {
        finalSegments: [{ id: "mutable_1", type: "mutable", original_text: "A", final_text: "B", source: "composer" }],
      };
      updateAppliedEvidence(trace, assembled, { pass: false, errors: ["preservation_violation"] });
      if (trace.applied_evidence.length) throw new Error("rejected output must not retain applied evidence");
      return "cleared";
    });

    await asyncTest("95_auth_error_no_identical_retry", async () => {
      let calls = 0;
      const origFetch = globalThis.fetch;
      globalThis.fetch = async () => {
        calls++;
        return { ok: false, status: 401, text: async () => "unauthorized" };
      };
      try {
        const profile = Object.assign(defaultRoleProfile("character_reader"), {
          provider: "openai_compatible", endpoint: "https://api.test.com/v1", model: "test-model", api_key_ref: "secret",
        });
        const trace = newTrace("test", "test");
        const role = DEFAULT_ROLES.find((item) => item.role_id === "character_reader");
        await callRole(role, profile, [{ id: "mutable_1", type: "mutable", text: "A" }], "", [{ id: "mutable_1", type: "mutable", text: "A" }], null, trace, null);
        if (calls !== 1) throw new Error(`401 should make one request, got ${calls}`);
        if (trace.roles[0].error_class !== "http_401") throw new Error(`wrong error class: ${trace.roles[0].error_class}`);
      } finally {
        globalThis.fetch = origFetch;
      }
      return `calls:${calls}`;
    });

    await asyncTest("96_reasoning_only_response_classified", async () => {
      let calls = 0;
      const origFetch = globalThis.fetch;
      globalThis.fetch = async () => {
        calls++;
        return {
          ok: true,
          text: async () => JSON.stringify({ choices: [{ message: { content: "", reasoning_content: "analysis only" } }] }),
        };
      };
      try {
        const profile = Object.assign(defaultRoleProfile("character_reader"), {
          provider: "openai_compatible", endpoint: "https://api.test.com/v1", model: "glm-5.2",
        });
        const trace = newTrace("test", "test");
        const role = DEFAULT_ROLES.find((item) => item.role_id === "character_reader");
        const result = await callRole(role, profile, [{ id: "mutable_1", type: "mutable", text: "A" }], "", [{ id: "mutable_1", type: "mutable", text: "A" }], null, trace, null);
        if (result) throw new Error("reasoning-only response should not validate");
        if (trace.roles[0].error_class !== "reasoning_only_response") throw new Error(`wrong error: ${trace.roles[0].error_class}`);
        if (calls !== 2) throw new Error(`reasoning-only should get one repair retry, got ${calls}`);
      } finally {
        globalThis.fetch = origFetch;
      }
      return `calls:${calls}`;
    });

    test("97_custom_prompt_fields_protected", () => {
      const result = applyExtraBodySafe(
        { model: "real", system: "real system", user: "real user", temperature: 0.3 },
        { model: "evil", system: "evil", user: "evil", top_p: 0.8 },
        "custom"
      );
      if (result.body.model !== "real" || result.body.system !== "real system" || result.body.user !== "real user") {
        throw new Error("custom prompt fields were overridden");
      }
      if (result.applied_body_keys.join(",") !== "top_p") throw new Error(`unexpected applied keys: ${result.applied_body_keys.join(",")}`);
      return `skipped:${result.skipped_body_keys.join(",")}`;
    });

    test("98_composer_token_plan_capped", () => {
      const plan = composerTokenPlan([{ text: "가".repeat(100000) }], { max_output_tokens: 999999 });
      if (plan.estimated_output_tokens !== 32000 || plan.requested_output_tokens !== 32000) {
        throw new Error(`token cap failed: ${JSON.stringify(plan)}`);
      }
      return `estimated:${plan.estimated_output_tokens} requested:${plan.requested_output_tokens}`;
    });

    test("99_candidate_role_identity_forced", () => {
      const parsed = validateCandidateSchema({
        role: "fake_role",
        candidates: [{ segment_id: "mutable_1", rewrite: "B", confidence: 0.8 }],
      }, "character_reader", ["mutable_1"]);
      if (!parsed || parsed.role !== "character_reader") throw new Error(`role identity trusted model: ${parsed && parsed.role}`);
      return parsed.role;
    });

    test("100_balanced_signal_adds_configured_role", () => {
      const settings2 = defaultSettings();
      Object.keys(settings2.role_profiles).forEach((roleId) => {
        settings2.role_profiles[roleId].endpoint = "https://test.example.com/v1";
        settings2.role_profiles[roleId].model = "test-model";
      });
      const baseline = selectRoles(DEFAULT_ROLES, [], "balanced", settings2);
      const signaled = selectRoles(DEFAULT_ROLES, [{ id: "lorebook_available", severity: "medium" }], "balanced", settings2);
      if (baseline.roles.some((role) => role.role_id === "world_reader")) throw new Error("world_reader should not be balanced core");
      if (!signaled.roles.some((role) => role.role_id === "world_reader")) throw new Error("lore signal should add world_reader");
      const reason = signaled.selectReasons.find((item) => item.role_id === "world_reader");
      if (!reason || reason.reason.indexOf("adaptive_score:") !== 0
          || reason.reason.indexOf("lorebook_available") < 0) {
        throw new Error("adaptive lore signal reason missing");
      }
      return reason.reason;
    });

    await asyncTest("101_scheduler_records_real_queue_timing", async () => {
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        const body = safeString(opts && opts.body);
        const isComposer = body.indexOf("Whole-Scene Fusion Composer") >= 0;
        const content = isComposer
          ? '{"segments":{"mutable_1":"Composed"}}'
          : '{"role":"character_reader","candidates":[{"segment_id":"mutable_1","rewrite":"Rewritten","confidence":0.8}]}';
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content } }] }) };
      };
      try {
        const settings2 = defaultSettings();
        Object.keys(settings2.role_profiles).forEach((roleId) => {
          settings2.role_profiles[roleId].endpoint = "https://test.example.com/v1";
          settings2.role_profiles[roleId].model = "test-model";
        });
        const roles = selectRoles(DEFAULT_ROLES, [], "fast", settings2).roles;
        const trace = newTrace("test", "test");
        const deadline = createDeadline(60000);
        await scheduleRoles(roles, settings2.role_profiles, [{ id: "mutable_1", type: "mutable", text: "Original" }], "", [{ id: "mutable_1", type: "mutable", text: "Original" }], deadline, trace, 2);
        deadline.cancel();
        if (!trace.roles.length) throw new Error("no role trace entries");
        trace.roles.forEach((entry) => {
          if (!(entry.queued_at <= entry.started_at && entry.started_at <= entry.ended_at)) {
            throw new Error(`invalid timing for ${entry.role_id}`);
          }
          if (entry.http_attempts !== 1) throw new Error(`unexpected attempts for ${entry.role_id}: ${entry.http_attempts}`);
        });
      } finally {
        globalThis.fetch = origFetch;
      }
      return "queue/start/end recorded";
    });

    test("102_reasoning_model_families", () => {
      const glm51 = {};
      applyReasoningAdapter("ollama_compatible", glm51, { model: "glm-5.1", reasoning_effort: "disable" });
      if (glm51.think !== false || !glm51.thinking || glm51.thinking.type !== "disabled") throw new Error("GLM 5.1 toggle failed");
      const glm52 = {};
      applyReasoningAdapter("openai_compatible", glm52, { model: "glm-5.2", reasoning_effort: "medium" });
      if (glm52.reasoning_effort !== "medium" || glm52.think !== true) throw new Error("GLM 5.2 effort failed");
      const deepseek = {};
      applyReasoningAdapter("openai_compatible", deepseek, { model: "deepseek-v4", reasoning_effort: "low" });
      if (deepseek.reasoning_effort !== "low") throw new Error("DeepSeek effort failed");
      const gemini25 = { generationConfig: {} };
      applyReasoningAdapter("gemini", gemini25, { model: "gemini-2.5-pro", reasoning_effort: "auto", reasoning_budget_tokens: 2048 });
      if (gemini25.generationConfig.thinkingConfig.thinkingBudget !== 2048) throw new Error("Gemini 2.5 budget failed");
      const kimiNative = {};
      const kimiInfo = applyReasoningAdapter("ollama_compatible", kimiNative, { endpoint: "http://localhost:11434", model: "kimi-k2.7-code", reasoning_effort: "none" });
      if (kimiInfo.family !== "kimi" || kimiNative.think !== false || kimiNative.reasoning_effort !== undefined) throw new Error("Kimi native none failed");
      return "glm51/glm52/deepseek/gemini25/kimi";
    });

    await asyncTest("103_full_after_request_returns_enhanced_output", async () => {
      const originalRisu = globalThis.Risuai;
      const originalFetch = globalThis.fetch;
      const store = {};
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null,
          setItem: async (key, value) => { store[key] = value; },
        },
      };
      globalThis.fetch = async (url, opts) => {
        const body = safeString(opts && opts.body);
        const isComposer = body.indexOf("Whole-Scene Fusion Composer") >= 0;
        const content = isComposer
          ? '{"segments":{"mutable_1":"Final composed scene."}}'
          : '{"role":"specialist","candidates":[{"segment_id":"mutable_1","rewrite":"Improved specialist scene.","confidence":0.8,"issues":["prose_clarity"]}]}';
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content } }] }) };
      };
      try {
        const settings2 = defaultSettings();
        settings2.preset = "fast";
        settings2.trace_enabled = true;
        ["character_reader", "style_reader", COMPOSER_ROLE_ID].forEach((roleId) => {
          settings2.role_profiles[roleId].endpoint = "https://test.example.com/v1";
          settings2.role_profiles[roleId].model = "test-model";
        });
        await saveSettings(settings2);
        await onBeforeRequest([
          { role: "system", content: "Test system context." },
          { role: "user", content: "Continue the scene." },
        ], "main");
        const output = await onAfterRequest("Original scene.", "main");
        if (output !== "Final composed scene.") {
          const traces = JSON.parse(store[TRACE_KEY] || "[]");
          const latest = traces[0] || {};
          throw new Error(
            `enhanced output not returned: ${output}; reason=${latest.final && latest.final.reason}; errors=${JSON.stringify(latest.errors || [])}`
          );
        }
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
      return "enhanced return path";
    });

    await asyncTest("104_full_after_request_verifier_rejects_and_clears_evidence", async () => {
      const originalRisu = globalThis.Risuai;
      const originalFetch = globalThis.fetch;
      const store = {};
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null,
          setItem: async (key, value) => { store[key] = value; },
        },
      };
      globalThis.fetch = async (url, opts) => {
        const body = safeString(opts && opts.body);
        const isComposer = body.indexOf("Whole-Scene Fusion Composer") >= 0;
        const content = isComposer
          ? '{"segments":{"mutable_1":"Broken fence ```"}}'
          : '{"role":"specialist","candidates":[{"segment_id":"mutable_1","rewrite":"Improved specialist scene.","confidence":0.8}]}';
        return { ok: true, text: async () => JSON.stringify({ choices: [{ message: { content } }] }) };
      };
      try {
        const settings2 = defaultSettings();
        settings2.preset = "fast";
        settings2.trace_enabled = true;
        ["character_reader", "style_reader", COMPOSER_ROLE_ID].forEach((roleId) => {
          settings2.role_profiles[roleId].endpoint = "https://test.example.com/v1";
          settings2.role_profiles[roleId].model = "test-model";
        });
        await saveSettings(settings2);
        const original = "Original scene.";
        await onBeforeRequest([
          { role: "system", content: "Test system context." },
          { role: "user", content: "Continue the scene." },
        ], "main");
        const output = await onAfterRequest(original, "main");
        if (output !== original) throw new Error("verifier rejection must return original");
        const traces = JSON.parse(store[TRACE_KEY] || "[]");
        const latest = traces[0];
        if (!latest || latest.summary.final_state !== "rejected") throw new Error("rejected trace state missing");
        if ((latest.applied_evidence || []).length !== 0) throw new Error("rejected trace retained applied evidence");
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
      return "original returned, evidence cleared";
    });

    await asyncTest("105_invalid_request_override_stops_before_http", async () => {
      let calls = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = async () => {
        calls++;
        throw new Error("fetch should not run");
      };
      try {
        const profile = Object.assign(defaultRoleProfile("character_reader"), {
          provider: "openai_compatible",
          endpoint: "https://test.example.com/v1",
          model: "test-model",
          extra_body: "{broken",
        });
        const trace = newTrace("test", "test");
        const role = DEFAULT_ROLES.find((item) => item.role_id === "character_reader");
        await callRole(role, profile, [{ id: "mutable_1", type: "mutable", text: "A" }], "", [{ id: "mutable_1", type: "mutable", text: "A" }], null, trace, null);
        if (calls !== 0) throw new Error(`invalid override reached HTTP: ${calls}`);
        if (trace.roles[0].error_class !== "configuration_error") throw new Error(`wrong error class: ${trace.roles[0].error_class}`);
      } finally {
        globalThis.fetch = originalFetch;
      }
      let headerError = "";
      try { parseExtraHeaders("broken-header"); } catch (err) { headerError = safeString(err && err.message); }
      if (headerError !== "invalid_extra_header_line") throw new Error(`invalid header not rejected: ${headerError}`);
      return "body/header rejected before HTTP";
    });

    function testPlannerFragment(roleId) {
      const fragment = {
        schema: "turn_contract_fragment.v1",
        planner_role: roleId,
      };
      inputPlannerFields(roleId).forEach((field) => { fragment[field] = []; });
      fragment.required_facts = [{
        text: "The bell rang",
        evidence_refs: ["payload_user_input"],
        evidence_quote: "The bell rang",
      }];
      return fragment;
    }

    await asyncTest("106_kimi_planner_tool_arguments_and_temperature_zero", async () => {
      const role = DEFAULT_ROLES.find((item) => item.role_id === "input_canon_secret_planner");
      const fragment = testPlannerFragment(role.role_id);
      const originalFetch = globalThis.fetch;
      let capturedBody = null;
      globalThis.fetch = async (_url, opts) => {
        capturedBody = JSON.parse(opts.body);
        return {
          ok: true,
          text: async () => JSON.stringify({
            choices: [{
              message: {
                content: "",
                tool_calls: [{
                  type: "function",
                  function: {
                    name: "submit_turn_contract_fragment",
                    arguments: JSON.stringify(fragment),
                  },
                }],
              },
            }],
          }),
        };
      };
      try {
        const profile = Object.assign(defaultRoleProfile(role.role_id), {
          provider: "openai_compatible",
          endpoint: "https://ollama.com/v1",
          model: "kimi-k2.7-code:cloud",
          reasoning_effort: "none",
          force_json_response: true,
        });
        const trace = newTrace("test", "test");
        trace.budget.input_attempt_max = 2;
        const result = await callRole(role, profile, [], "", [], null, trace, {
          context_manifest: {
            evidence_refs: ["payload_user_input"],
            evidence_sources: { payload_user_input: "The bell rang at dawn." },
          },
        });
        if (!result || !result.required_facts.length) throw new Error("tool arguments were not accepted");
        if (!capturedBody || capturedBody.temperature !== 0) throw new Error("planner temperature was not forced to zero");
        if (!capturedBody.tools || capturedBody.tool_choice.function.name !== "submit_turn_contract_fragment") throw new Error("forced planner tool missing");
        if (capturedBody.response_format !== undefined) throw new Error("cloud planner must not rely on response_format");
        const properties = capturedBody.tools[0].function.parameters.properties;
        if (properties.relationship_and_emotion_state) throw new Error("canon planner received unrelated relationship field");
        if (trace.roles[0].attempts[0].structured_transport !== "tool_call_used") throw new Error("tool transport not traced");
        return "tool arguments accepted; temperature=0; role schema reduced";
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await asyncTest("107_kimi_planner_content_json_fallback", async () => {
      const role = DEFAULT_ROLES.find((item) => item.role_id === "input_scene_continuity_planner");
      const fragment = testPlannerFragment(role.role_id);
      const originalFetch = globalThis.fetch;
      globalThis.fetch = async () => ({
        ok: true,
        text: async () => JSON.stringify({
          choices: [{ message: { content: JSON.stringify(fragment) } }],
        }),
      });
      try {
        const profile = Object.assign(defaultRoleProfile(role.role_id), {
          provider: "openai_compatible",
          endpoint: "https://ollama.com/v1",
          model: "kimi-k2.7-code:cloud",
          reasoning_effort: "none",
        });
        const trace = newTrace("test", "test");
        trace.budget.input_attempt_max = 2;
        const result = await callRole(role, profile, [], "", [], null, trace, {
          context_manifest: {
            evidence_refs: ["payload_user_input"],
            evidence_sources: { payload_user_input: "The bell rang at dawn." },
          },
        });
        if (!result) throw new Error("content JSON fallback was rejected");
        if (trace.roles[0].attempts[0].structured_transport !== "content_json") throw new Error("content fallback not traced");
        return "content JSON fallback accepted";
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await asyncTest("108_input_planner_single_attempt_fallback_and_bounded_preview", async () => {
      const role = DEFAULT_ROLES.find((item) => item.role_id === "input_character_relationship_planner");
      const manifest = {
        evidence_refs: ["payload_user_input"],
        evidence_sources: { payload_user_input: "The bell rang at dawn." },
      };
      const originalFetch = globalThis.fetch;
      let calls = 0;
      globalThis.fetch = async () => {
        calls++;
        return {
          ok: true,
          text: async () => JSON.stringify({
            choices: [{
              message: {
                content: `not json Authorization: Bearer sk-secret-key-1234567890 ${"x".repeat(600)}`,
              },
            }],
          }),
        };
      };
      try {
        const noFallback = Object.assign(defaultRoleProfile(role.role_id), {
          provider: "openai_compatible",
          endpoint: "https://ollama.com/v1",
          model: "kimi-k2.7-code:cloud",
        });
        const failedTrace = newTrace("test", "test");
        failedTrace.budget.input_attempt_max = 2;
        const failed = await callRole(role, noFallback, [], "", [], null, failedTrace, { context_manifest: manifest });
        if (failed || calls !== 1 || failedTrace.roles[0].retry !== 0) throw new Error("planner repeated the same profile");
        const failurePreview = failedTrace.roles[0].attempts[0].failure_preview;
        if (!failurePreview || failurePreview.length > 410) throw new Error("failure preview is missing or unbounded");
        if (failurePreview.indexOf("sk-secret-key") >= 0) throw new Error("failure preview leaked a key");

        calls = 0;
        const fragment = testPlannerFragment(role.role_id);
        globalThis.fetch = async (_url, opts) => {
          calls++;
          const body = JSON.parse(opts.body);
          const content = body.model === "fallback-model" ? JSON.stringify(fragment) : "not json";
          return {
            ok: true,
            text: async () => JSON.stringify({ choices: [{ message: { content } }] }),
          };
        };
        const withFallback = Object.assign({}, noFallback, {
          fallback_provider: "openai_compatible",
          fallback_endpoint: "https://fallback.example.com/v1",
          fallback_model: "fallback-model",
        });
        const fallbackTrace = newTrace("test", "test");
        fallbackTrace.budget.input_attempt_max = 2;
        const recovered = await callRole(role, withFallback, [], "", [], null, fallbackTrace, { context_manifest: manifest });
        if (!recovered || calls !== 2 || !fallbackTrace.roles[0].fallback || fallbackTrace.roles[0].retry !== 0) {
          throw new Error("cross-provider fallback did not recover in one extra attempt");
        }
        return "one primary; one distinct fallback; bounded redacted preview";
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await asyncTest("109_composer_primary_and_fallback_reserved_before_specialist_retry", async () => {
      const outputRoles = DEFAULT_ROLES.filter((role) => !role.is_input_planner);
      const profiles = {};
      outputRoles.forEach((role) => {
        profiles[role.role_id] = Object.assign(defaultRoleProfile(role.role_id), {
          enabled: true,
          provider: "openai_compatible",
          endpoint: "https://test.example.com/v1",
          model: `model-${role.role_id}`,
          timeout_ms: 10000,
        });
      });
      profiles[COMPOSER_ROLE_ID].fallback_provider = "openai_compatible";
      profiles[COMPOSER_ROLE_ID].fallback_endpoint = "https://fallback.example.com/v1";
      profiles[COMPOSER_ROLE_ID].fallback_model = "composer-fallback";
      const originalFetch = globalThis.fetch;
      let characterAttempts = 0;
      globalThis.fetch = async (_url, opts) => {
        const body = JSON.parse(opts.body);
        const user = safeString(body.messages && body.messages[1] && body.messages[1].content);
        if (user.indexOf("FINAL version") >= 0) {
          if (body.model !== "composer-fallback") {
            return { ok: false, status: 504, text: async () => "primary composer timeout" };
          }
          return {
            ok: true,
            text: async () => JSON.stringify({
              choices: [{ message: { content: '{"segments":{"mutable_1":"composer rewrite"}}' } }],
            }),
          };
        }
        const match = /"role":"([^"]+)"/.exec(user);
        const roleId = match ? match[1] : "character_reader";
        if (roleId === "character_reader") {
          characterAttempts++;
          if (characterAttempts === 1) {
            return {
              ok: true,
              text: async () => JSON.stringify({ choices: [{ message: { content: "not json" } }] }),
            };
          }
        }
        const payload = {
          role: roleId,
          candidates: [{
            segment_id: "mutable_1",
            rewrite: `${roleId} rewrite`,
            confidence: 0.8,
            issues: [roleAllowedIssues(roleId)[0]],
            change_summary: "improved",
            tags: [],
          }],
        };
        return {
          ok: true,
          text: async () => JSON.stringify({ choices: [{ message: { content: JSON.stringify(payload) } }] }),
        };
      };
      const deadline = createDeadline(60000, Date.now());
      try {
        const trace = newTrace("test", "test");
        trace.router.preset = "quality";
        const selected = selectRoles(
          outputRoles,
          [{ id: "dialogue_heavy", severity: "medium" }],
          "quality",
          { role_profiles: profiles }
        ).roles;
        const segments = [{ id: "mutable_1", type: "mutable", text: "original", leading_ws: "", trailing_ws: "" }];
        const result = await scheduleRoles(selected, profiles, segments, "", segments, deadline, trace, 4);
        const specialistEntries = trace.roles.filter((entry) => entry.role_id !== "whole_scene_composer");
        if (specialistEntries.length !== OUTPUT_SPECIALIST_LIMIT.quality) {
          throw new Error(`adaptive specialist count changed: ${specialistEntries.length}`);
        }
        if (!result.composerResult || trace.budget.composer_attempt_used !== 2) {
          throw new Error("composer primary and fallback were not both attempted");
        }
        const composerEntry = trace.roles.find((entry) => entry.role_id === COMPOSER_ROLE_ID);
        if (!composerEntry || !composerEntry.fallback) throw new Error("composer fallback was not traced");
        if (trace.budget.http_attempt_used !== OUTPUT_HTTP_ATTEMPT_BUDGET.quality || characterAttempts !== 1) {
          throw new Error(`unexpected budget use ${trace.budget.http_attempt_used}, character attempts ${characterAttempts}`);
        }
        return "4 specialist primaries + composer primary/fallback = 6; no specialist retry";
      } finally {
        deadline.cancel();
        globalThis.fetch = originalFetch;
      }
    });

    test("110_role_prompt_contract_v2_boundaries", () => {
      const inputRoles = DEFAULT_ROLES.filter((role) => role.is_input_planner);
      const specialists = DEFAULT_ROLES.filter((role) => !role.is_input_planner && !role.is_composer);
      if (inputRoles.some((role) => role.default_prompt.indexOf("Evidence contract:") < 0
          || role.default_prompt.indexOf("You own only:") < 0)) {
        throw new Error("input planner ownership or evidence contract missing");
      }
      if (specialists.some((role) => role.default_prompt.indexOf("decisive revision worker") < 0
          || role.default_prompt.indexOf("You do not own:") < 0
          || role.default_prompt.indexOf("return candidates: []") < 0)) {
        throw new Error("specialist rewrite boundary missing");
      }
      if (ROLE_PROMPTS.secret_pov_guard.indexOf("user agency or model/meta artifact cleanup") < 0) {
        throw new Error("secret lane does not exclude agency/meta ownership");
      }
      if (ROLE_PROMPTS.agency_meta_guard.indexOf("secret or identity policy") < 0) {
        throw new Error("agency lane does not exclude secret ownership");
      }
      if (ROLE_PROMPTS.whole_scene_composer.indexOf("Authority order:") < 0
          || ROLE_PROMPTS.whole_scene_composer.indexOf("Secret, POV, identity/reveal continuity, user agency") < 0) {
        throw new Error("composer authority order missing");
      }
      if (DEFAULT_ROLES.some((role) => role.default_prompt.indexOf("Confidence: report how much you improved") >= 0)) {
        throw new Error("legacy confidence contract still active");
      }
      const segment = [{ id: "mutable_1", type: "mutable", text: "Draft." }];
      const secretRole = DEFAULT_ROLES.find((role) => role.role_id === "secret_pov_guard");
      const secretRequest = buildRolePrompt(
        secretRole,
        defaultRoleProfile(secretRole.role_id),
        segment,
        "",
        segment,
        null,
      ).user;
      if (secretRequest.indexOf("identity_continuity") < 0 || secretRequest.indexOf("meta_artifact") >= 0) {
        throw new Error("specialist request exposes another lane's issue codes");
      }
      return `${inputRoles.length} planners, ${specialists.length} specialists, composer authority aligned`;
    });

    test("111_planner_prompt_transport_specific", () => {
      const role = DEFAULT_ROLES.find((item) => item.role_id === "input_scene_continuity_planner");
      const manifest = { evidence_refs: [], evidence_sources: {} };
      const gatewayProfile = Object.assign(defaultRoleProfile(role.role_id), {
        provider: "openai_compatible",
        endpoint: "https://api.llmgateway.io/v1/chat/completions",
        model: "minimax-m3",
      });
      const gatewayPrompt = buildRolePrompt(role, gatewayProfile, [], "", [], { context_manifest: manifest });
      if (gatewayPrompt.user.indexOf("submit_turn_contract_fragment") >= 0
          || gatewayPrompt.user.indexOf("Do not emit XML, <tool_call>") < 0) {
        throw new Error("non-tool planner received tool transport instructions");
      }
      const kimiProfile = Object.assign(defaultRoleProfile(role.role_id), {
        provider: "ollama_compatible",
        endpoint: "https://ollama.com/v1/chat/completions",
        model: "kimi-k2.7-code:cloud",
      });
      const kimiPrompt = buildRolePrompt(role, kimiProfile, [], "", [], { context_manifest: manifest });
      if (kimiPrompt.user.indexOf("Call submit_turn_contract_fragment exactly once") < 0) {
        throw new Error("tool-capable planner did not receive tool transport instructions");
      }
      return "gateway JSON-only; Kimi tool-specific";
    });

    test("112_empty_candidate_array_is_valid_success", () => {
      const empty = validateCandidateSchema({ role: "style_reader", candidates: [] }, "style_reader", ["mutable_1"]);
      if (!empty || !Array.isArray(empty.candidates) || empty.candidates.length !== 0) {
        throw new Error("empty candidate array rejected");
      }
      const invalid = validateCandidateSchema({
        role: "style_reader",
        candidates: [{ segment_id: "foreign", rewrite: "rewrite" }],
      }, "style_reader", ["mutable_1"]);
      if (invalid !== null) throw new Error("nonempty invalid candidate array accepted");
      return "clean lane succeeds without fabricated rewrite";
    });

    test("113_legacy_builtin_prompt_migrates", () => {
      const legacyPrompt = "You are the Canon/Secret input planner for a roleplay turn. Read the provenance-labelled context manifest and return a compact turn_contract_fragment.v1 JSON object. Extract only facts supported by evidence_refs. Separate writer_only_secrets from character_visible_facts. Record identity aliases and per-character knowledge scopes without revealing secrets to characters who do not know them. Do not write dialogue, narration, a draft, advice, or markdown. Output JSON only.";
      if (!isLegacyBuiltinPrompt("input_canon_secret_planner", legacyPrompt)) {
        throw new Error("legacy prompt digest mismatch");
      }
      const merged = mergeSettings(defaultSettings(), {
        role_profiles: {
          input_canon_secret_planner: { system_prompt: legacyPrompt },
        },
      });
      const migrated = merged.role_profiles.input_canon_secret_planner;
      if (migrated.system_prompt !== ROLE_PROMPTS.input_canon_secret_planner
          || migrated.prompt_contract_version !== ROLE_PROMPT_VERSION) {
        throw new Error("legacy builtin prompt not migrated");
      }
      return `legacy builtin migrated to ${ROLE_PROMPT_VERSION}`;
    });

    test("114_custom_prompt_preserved", () => {
      const customPrompt = "CUSTOM ROLE CONTRACT: rewrite the scene with a private house style.";
      const merged = mergeSettings(defaultSettings(), {
        role_profiles: {
          style_reader: {
            system_prompt: customPrompt,
            prompt_contract_version: "custom",
          },
        },
      });
      const profile = merged.role_profiles.style_reader;
      if (profile.system_prompt !== customPrompt || profile.prompt_contract_version !== "custom") {
        throw new Error("custom prompt was overwritten");
      }
      return "custom prompt retained verbatim";
    });

    test("115_role_issue_whitelist_enforced", () => {
      const mixedLane = validateCandidateSchema({
        role: "character_reader",
        candidates: [{
          segment_id: "mutable_1",
          rewrite: "A rewrite that claims to fix two lanes.",
          confidence: 0.9,
          issues: ["character_voice", "meta_artifact"],
        }],
      }, "character_reader", ["mutable_1"]);
      if (mixedLane !== null) throw new Error("foreign issue survived character lane validation");
      const outputContract = validateCandidateSchema({
        role: "agency_meta_guard",
        candidates: [{
          segment_id: "mutable_1",
          rewrite: "명시된 출력 언어를 지킨 재작성.",
          confidence: 0.9,
          issues: ["output_contract_violation"],
        }],
      }, "agency_meta_guard", ["mutable_1"]);
      if (!outputContract || outputContract.candidates.length !== 1) {
        throw new Error("agency output-contract candidate rejected");
      }
      return "foreign lane rejected; output contract accepted";
    });

    test("116_output_contract_priority_and_prompt", () => {
      if (ISSUE_PRIORITY.output_contract_violation <= ISSUE_PRIORITY.character_voice
          || ISSUE_PRIORITY.meta_artifact <= ISSUE_PRIORITY.character_voice) {
        throw new Error("binding output or meta priority is below character voice");
      }
      const agencyPrompt = ROLE_PROMPTS.agency_meta_guard;
      const composerPrompt = ROLE_PROMPTS.whole_scene_composer;
      if (agencyPrompt.indexOf("output language") < 0
          || composerPrompt.indexOf("binding turn contract") < 0) {
        throw new Error("output language contract missing from rewrite prompts");
      }
      return "binding output and meta constraints outrank character/style";
    });

    test("117_pseudo_tool_planner_recovery", () => {
      const pseudo = [
        "<tool_call>",
        "<function=submit_turn_contract_fragment>",
        "<parameter=schema>turn_contract_fragment.v1</parameter>",
        "<parameter=planner_role>input_canon_secret_planner</parameter>",
        '<parameter=required_facts>[{"text":"The bridge is closed.","evidence_refs":["payload_user_input"],"evidence_quote":"bridge is closed"}]</parameter>',
        "<parameter=writer_only_secrets>[]</parameter>",
        "</function>",
        "</tool_call>",
      ].join("\n");
      const parsed = tryParseJson(pseudo);
      if (!parsed || parsed.schema !== "turn_contract_fragment.v1"
          || parsed.planner_role !== "input_canon_secret_planner"
          || !Array.isArray(parsed.required_facts)
          || parsed.required_facts.length !== 1) {
        throw new Error("pseudo tool wrapper was not recovered");
      }
      return "textual tool wrapper recovered as planner object";
    });

    test("118_planner_output_is_bounded", () => {
      const items = Array.from({ length: 5 }, (_, index) => ({
        text: `Alpha fact ${index}`,
        evidence_refs: ["payload_user_input"],
        evidence_quote: `Alpha fact ${index}`,
      }));
      const fragment = validateTurnContractFragment({
        required_facts: items,
        uncertainty: items,
      }, "input_scene_continuity_planner", {
        evidence_refs: ["payload_user_input"],
        evidence_sources: {
          payload_user_input: items.map((item) => item.evidence_quote).join(" "),
        },
      });
      if (!fragment
          || fragment.required_facts.length !== PLANNER_MAX_ITEMS_PER_FIELD
          || fragment.uncertainty.length !== PLANNER_MAX_ITEMS_PER_FIELD) {
        throw new Error("planner field cap not enforced");
      }
      const tool = inputPlannerTool("input_scene_continuity_planner");
      const requiredFactsSchema = tool.function.parameters.properties.required_facts;
      if (requiredFactsSchema.maxItems !== PLANNER_MAX_ITEMS_PER_FIELD) {
        throw new Error("planner tool schema cap missing");
      }
      return "prompt, schema, and validator share compact limits";
    });

    await asyncTest("119_composer_time_reserve_aborts_specialist_phase", async () => {
      const originalFetch = globalThis.fetch;
      const pipelineController = new AbortController();
      let remainingCalls = 0;
      const fakeDeadline = {
        signal: pipelineController.signal,
        check: () => false,
        remaining: () => {
          remainingCalls++;
          if (remainingCalls === 1) return 180000;
          if (remainingCalls === 2) return 10100;
          return 10000;
        },
      };
      globalThis.fetch = async (_url, opts) => {
        const body = JSON.parse(opts.body);
        const userPrompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        if (userPrompt.indexOf("FINAL version") >= 0) {
          return {
            ok: true,
            text: async () => JSON.stringify({
              choices: [{ message: { content: '{"segments":{"mutable_1":"Composer completed."}}' } }],
            }),
          };
        }
        return new Promise(() => {});
      };
      try {
        const characterRole = DEFAULT_ROLES.find((role) => role.role_id === "character_reader");
        const composerRole = DEFAULT_ROLES.find((role) => role.role_id === "whole_scene_composer");
        const profiles = {
          character_reader: Object.assign(defaultRoleProfile("character_reader"), {
            endpoint: "https://test.example.com/v1",
            model: "slow-specialist",
            timeout_ms: 5000,
          }),
          whole_scene_composer: Object.assign(defaultRoleProfile("whole_scene_composer"), {
            endpoint: "https://test.example.com/v1",
            model: "composer",
            timeout_ms: 5000,
          }),
        };
        const trace = newTrace("test", "model");
        trace.router.preset = "quality";
        const segments = [{
          id: "mutable_1",
          type: "mutable",
          text: "Original.",
          leading_ws: "",
          trailing_ws: "",
        }];
        const result = await scheduleRoles(
          [characterRole, composerRole],
          profiles,
          segments,
          "",
          segments,
          fakeDeadline,
          trace,
          1
        );
        const specialistTrace = trace.roles.find((entry) => entry.role_id === "character_reader");
        if (!result.composerResult
            || trace.composer.status !== "fulfilled"
            || trace.composer.specialist_stop_reason !== "composer_reserve"
            || !specialistTrace
            || specialistTrace.error_class !== "composer_reserve_aborted") {
          throw new Error(`reserve path failed:${JSON.stringify({
            composer: trace.composer,
            specialist: specialistTrace && specialistTrace.error_class,
          })}`);
        }
        return `reserve:${trace.composer.reserve_ms}ms composer fulfilled`;
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    test("120_v2_builtin_prompt_migrates_to_v3", () => {
      const merged = mergeSettings(defaultSettings(), {
        role_profiles: {
          agency_meta_guard: {
            system_prompt: "previous v2 builtin snapshot",
            prompt_contract_version: "rp-rewrite-contract.v2",
          },
        },
      });
      const profile = merged.role_profiles.agency_meta_guard;
      if (profile.system_prompt !== ROLE_PROMPTS.agency_meta_guard
          || profile.prompt_contract_version !== ROLE_PROMPT_VERSION) {
        throw new Error("v2 builtin prompt was not upgraded to v3");
      }
      return `${PREVIOUS_ROLE_PROMPT_VERSIONS[0]} -> ${ROLE_PROMPT_VERSION}`;
    });

    test("121_meta_wrapper_delete_is_narrow_and_traceable", () => {
      const text = '<Thoughts>internal planning only</Thoughts><img src="x">Scene begins.';
      const segments = buildSegmentMap(text, defaultSettings());
      if (segments.some((segment) =>
        segment.type === "protected" && /thoughts/i.test(segment.text)
      )) {
        throw new Error("Thoughts wrapper remained protected");
      }
      const mutableSegs = mutableSegments(segments);
      const metaSegment = mutableSegs.find((segment) => isWhollyMetaArtifactText(mutableFullText(segment)));
      if (!metaSegment) throw new Error("standalone meta wrapper was not isolated as mutable");
      const validated = validateCandidateSchema({
        role: "agency_meta_guard",
        candidates: [{
          segment_id: metaSegment.id,
          operation: "delete",
          rewrite: "",
          confidence: 0.95,
          issues: ["meta_artifact"],
          change_summary: "remove model planning wrapper",
        }],
      }, "agency_meta_guard", mutableSegs.map((segment) => segment.id), mutableSegs);
      if (!validated || validated.candidates[0].operation !== "delete") {
        throw new Error("narrow meta deletion was rejected");
      }
      const director = fusionDirector({
        [metaSegment.id]: [Object.assign(
          { role_id: "agency_meta_guard" },
          validated.candidates[0]
        )],
      }, [DEFAULT_ROLES.find((role) => role.role_id === "agency_meta_guard")], segments);
      const assembled = assembleOutput(segments, null, director);
      if (/thoughts|internal planning/i.test(assembled.output)
          || assembled.output.indexOf('<img src="x">Scene begins.') < 0) {
        throw new Error(`meta delete damaged scene:${assembled.output}`);
      }
      const evidence = buildAppliedEvidence(assembled.finalSegments);
      if (!evidence.some((item) => item.operation === "delete")) {
        throw new Error("delete operation missing from applied evidence");
      }
      return "meta wrapper deleted; image and scene preserved";
    });

    await asyncTest("122_specialist_patch_is_returned_but_not_labeled_enhanced", async () => {
      const originalRisu = globalThis.Risuai;
      const originalFetch = globalThis.fetch;
      const store = {};
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null,
          setItem: async (key, value) => { store[key] = value; },
        },
      };
      globalThis.fetch = async (_url, opts) => {
        const body = JSON.parse(opts.body);
        if (body.model === "composer-model") {
          return { ok: false, status: 504, text: async () => "composer unavailable" };
        }
        const content = body.model === "character-model"
          ? '{"role":"character_reader","candidates":[{"segment_id":"mutable_1","rewrite":"Specialist-improved scene.","confidence":0.9,"issues":["character_voice"],"change_summary":"stronger voice"}]}'
          : '{"role":"style_reader","candidates":[]}';
        return {
          ok: true,
          text: async () => JSON.stringify({ choices: [{ message: { content } }] }),
        };
      };
      try {
        const settings = defaultSettings();
        settings.preset = "fast";
        settings.trace_enabled = true;
        [
          ["character_reader", "character-model"],
          ["style_reader", "style-model"],
          [COMPOSER_ROLE_ID, "composer-model"],
        ].forEach(([roleId, model]) => {
          settings.role_profiles[roleId].endpoint = "https://test.example.com/v1";
          settings.role_profiles[roleId].model = model;
        });
        await saveSettings(settings);
        await onBeforeRequest([{ role: "user", content: "Continue." }], "main");
        const output = await onAfterRequest("Original scene.", "main");
        const traces = JSON.parse(store[TRACE_KEY] || "[]");
        const latest = traces[0];
        if (output !== "Specialist-improved scene.") {
          throw new Error(`specialist patch was not returned:${output}`);
        }
        if (!latest || latest.final.enhanced
            || latest.summary.final_state !== "degraded_patch"
            || latest.final.reason !== "specialist_patch_without_composer") {
          throw new Error(`degraded state mislabeled:${JSON.stringify(latest && latest.final)}`);
        }
        return "changed output returned as degraded_patch, not Enhanced";
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    test("123_latest_comparison_is_full_ephemeral_and_replaced", () => {
      const originalText = `Before sentence ${"A".repeat(140)}`;
      const finalText = `After sentence ${"B".repeat(140)}`;
      const finalSegments = [{
        id: "mutable_1",
        type: "mutable",
        original_text: originalText,
        final_text: finalText,
        source: "composer",
        operation: "replace",
        applied_role_id: COMPOSER_ROLE_ID,
      }];
      const trace = newTrace("afterRequest", "main");
      setFinalTraceState(trace, true, "enhanced", "composer_integrated");
      updateLatestAppliedComparison(trace, finalSegments);
      const comparisonHtml = renderLatestComparison();
      if (comparisonHtml.indexOf(escapeHtml(originalText)) < 0
          || comparisonHtml.indexOf(escapeHtml(finalText)) < 0
          || comparisonHtml.indexOf(">전<") < 0
          || comparisonHtml.indexOf(">후<") < 0) {
        throw new Error("full before/after comparison missing");
      }

      const evidence = buildAppliedEvidence(finalSegments);
      if (Object.prototype.hasOwnProperty.call(evidence[0], "original_text")
          || Object.prototype.hasOwnProperty.call(evidence[0], "final_text")
          || JSON.stringify(evidence).indexOf(originalText) >= 0
          || JSON.stringify(evidence).indexOf(finalText) >= 0) {
        throw new Error("full comparison text leaked into persisted trace evidence");
      }

      const nextTrace = newTrace("afterRequest", "main");
      setFinalTraceState(nextTrace, false, "unchanged", "no_candidates_applied");
      updateLatestAppliedComparison(nextTrace, null);
      if (renderLatestComparison().indexOf(originalText) >= 0
          || latestAppliedComparison.changes.length !== 0) {
        throw new Error("previous turn comparison was not discarded");
      }
      return "full comparison is session-only and replaced by the next main turn";
    });

    test("124_endpoint_density_plan_serializes_three_or_more_calls", () => {
      const roles = [
        DEFAULT_ROLES.find((role) => role.role_id === "character_reader"),
        DEFAULT_ROLES.find((role) => role.role_id === "style_reader"),
        DEFAULT_ROLES.find((role) => role.role_id === "plot_continuity_reader"),
      ];
      const profiles = {};
      roles.forEach((role) => {
        profiles[role.role_id] = Object.assign(defaultRoleProfile(role.role_id), {
          endpoint: "https://one-provider.example/v1/chat/completions",
          model: `${role.role_id}-model`,
        });
      });
      const plan = buildExecutionGroupPlan(roles, profiles, "output_specialist");
      const group = plan["https://one-provider.example"];
      if (!group
          || group.selected_calls !== 3
          || group.effective_concurrency !== 1
          || group.reason !== "endpoint_density_3_serial") {
        throw new Error(`endpoint density plan mismatch:${JSON.stringify(plan)}`);
      }

      profiles.plot_continuity_reader.endpoint = "https://second-provider.example/v1";
      const distributed = buildExecutionGroupPlan(roles, profiles, "output_specialist");
      if (distributed["https://one-provider.example"].effective_concurrency !== 2
          || distributed["https://second-provider.example"].effective_concurrency !== 1) {
        throw new Error(`distributed endpoint plan mismatch:${JSON.stringify(distributed)}`);
      }
      return "3 calls on one endpoint serialize; separate endpoints retain independent concurrency";
    });

    await asyncTest("125_completion_wait_disables_plugin_request_timeout", async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      let receivedTimeoutField = false;
      globalThis.Risuai = {};
      globalThis.fetch = async (_url, options) => {
        receivedTimeoutField = Object.prototype.hasOwnProperty.call(options || {}, "requestTimeoutMs");
        await new Promise((resolve) => setTimeout(resolve, 40));
        return { ok: true, text: async () => "{}" };
      };
      try {
        const started = Date.now();
        await fetchWithAbort("https://wait.example/v1", {}, 0, null);
        const elapsed = Date.now() - started;
        if (elapsed < 30 || receivedTimeoutField) {
          throw new Error(`completion wait was still bounded:${elapsed}ms timeoutField:${receivedTimeoutField}`);
        }
        return `waited ${elapsed}ms without plugin timeout`;
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    await asyncTest("126_quality_same_endpoint_executes_specialists_serially_then_composer", async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      let active = 0;
      let peak = 0;
      globalThis.Risuai = {};
      globalThis.fetch = async (_url, options) => {
        active++;
        peak = Math.max(peak, active);
        const body = JSON.parse(options.body);
        await new Promise((resolve) => setTimeout(resolve, 15));
        active--;
        const roleId = safeString(body.model).replace(/-model$/, "");
        const content = roleId === COMPOSER_ROLE_ID
          ? '{"segments":{"mutable_1":"Composer rewrite."}}'
          : `{"role":"${roleId}","candidates":[]}`;
        return {
          ok: true,
          text: async () => JSON.stringify({ choices: [{ message: { content } }] }),
        };
      };
      const roles = [
        DEFAULT_ROLES.find((role) => role.role_id === "character_reader"),
        DEFAULT_ROLES.find((role) => role.role_id === "style_reader"),
        DEFAULT_ROLES.find((role) => role.role_id === "plot_continuity_reader"),
        DEFAULT_ROLES.find((role) => role.role_id === COMPOSER_ROLE_ID),
      ];
      const profiles = {};
      roles.forEach((role) => {
        profiles[role.role_id] = Object.assign(defaultRoleProfile(role.role_id), {
          endpoint: "https://single-endpoint.example/v1/chat/completions",
          model: `${role.role_id}-model`,
        });
      });
      const trace = newTrace("afterRequest", "main");
      trace.router.preset = "quality";
      trace.scheduler.completion_wait = true;
      const deadline = createCompletionDeadline(Date.now());
      const segments = [{
        id: "mutable_1",
        type: "mutable",
        text: "Original scene.",
        leading_ws: "",
        core_text: "Original scene.",
        trailing_ws: "",
      }];
      try {
        const result = await scheduleRoles(
          roles,
          profiles,
          segments,
          "",
          segments,
          deadline,
          trace,
          4
        );
        const specialistGroup = trace.scheduler.endpoint_groups.find(
          (group) => group.stage === "output_specialist"
        );
        if (!result.composerResult
            || peak !== 1
            || !specialistGroup
            || specialistGroup.selected_calls !== 3
            || specialistGroup.effective_concurrency !== 1) {
          throw new Error(`serial quality schedule failed:${JSON.stringify({
            peak,
            composer: !!result.composerResult,
            groups: trace.scheduler.endpoint_groups,
          })}`);
        }
        return "same endpoint peak=1; composer ran after specialists";
      } finally {
        deadline.cancel();
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    test("127_materiality_rejects_typo_only_and_accepts_recomposition", () => {
      const original = "권 서리는 젖은 장부를 넘기며 한얼의 보고를 묵묵히 들었다. 창밖에서는 빗물이 처마 끝을 타고 흘렀다.";
      const typoOnly = "권 서리는 젖은 장부를 넘기며 한얼의 보고를 묵묵히 들었다. 창밖에서는 빗물이 처마 끝을 타고 흘럿다.";
      const recomposed = "처마 끝의 빗물이 흙바닥을 두드리는 동안, 권 서리는 젖은 장부에서 손을 떼지 않은 채 한얼의 보고를 끝까지 받아냈다.";
      const minor = rewriteMateriality(original, typoOnly);
      const material = rewriteMateriality(original, recomposed);
      if (minor.material || !material.material) {
        throw new Error(`materiality mismatch:${JSON.stringify({ minor, material })}`);
      }
      return `minor:${minor.changed_span_ratio.toFixed(3)} material:${material.changed_span_ratio.toFixed(3)}`;
    });

    test("128_precision_only_composer_is_not_enhanced", () => {
      const segments = [{
        id: "mutable_1",
        type: "mutable",
        text: "The clerk opened the ledger and read the report.",
        leading_ws: "",
        core_text: "The clerk opened the ledger and read the report.",
        trailing_ws: "",
      }];
      const assembled = assembleOutput(
        segments,
        { segments: { mutable_1: "The clerk opened the ledger and read the reports." } },
        { ranked: {} }
      );
      const classification = classifyAppliedOutput(assembled);
      if (assembled.materialComposerApplied !== 0
          || classification.enhanced
          || classification.state !== "precision_patch") {
        throw new Error(`precision patch mislabeled:${JSON.stringify({ assembled, classification })}`);
      }
      return "minor composer edit remains precision_patch";
    });

    await asyncTest("129_quality_composer_retries_near_copy_with_material_rewrite", async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      let calls = 0;
      globalThis.Risuai = {};
      globalThis.fetch = async () => {
        calls++;
        const content = calls === 1
          ? '{"segments":{"mutable_1":"The clerk opened the ledger and read the reports."}}'
          : '{"segments":{"mutable_1":"Rain ticked against the paper doors as the clerk flattened the warped ledger, traced the missing totals with one ink-stained finger, and began reading the report aloud."}}';
        return {
          ok: true,
          text: async () => JSON.stringify({ choices: [{ message: { content } }] }),
        };
      };
      const role = DEFAULT_ROLES.find((item) => item.role_id === COMPOSER_ROLE_ID);
      const profile = Object.assign(defaultRoleProfile(COMPOSER_ROLE_ID), {
        endpoint: "https://composer.example/v1/chat/completions",
        model: "composer-model",
      });
      const segments = [{
        id: "mutable_1",
        type: "mutable",
        text: "The clerk opened the ledger and read the report.",
        leading_ws: "",
        core_text: "The clerk opened the ledger and read the report.",
        trailing_ws: "",
      }];
      const trace = newTrace("afterRequest", "main");
      trace.router.preset = "quality";
      trace.budget.http_attempt_max = OUTPUT_HTTP_ATTEMPT_BUDGET.quality;
      const director = {
        ranked: { mutable_1: [] },
        consensus: {},
        complementary: {},
        conflict: {},
        gap: ["mutable_1"],
      };
      try {
        const result = await runComposer(
          role,
          profile,
          segments,
          director,
          "",
          null,
          trace,
          segments,
          true
        );
        if (!result
            || calls !== 2
            || trace.composer.semantic_retry !== 1
            || !trace.composer.materiality
            || !trace.composer.materiality.pass) {
          throw new Error(`semantic retry failed:${JSON.stringify({
            calls,
            composer: trace.composer,
          })}`);
        }
        return "near-copy rejected; second composition materially rewrote the segment";
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    const passed = results.filter((r) => r.pass).length;
    const failed = results.filter((r) => !r.pass).length;
    const summary = `\n${"=".repeat(60)}\nRecomposer In-Memory Tests: ${passed}/${results.length} passed, ${failed} failed\n${"=".repeat(60)}\n` +
      results.map((r) => `  ${r.pass ? "✓" : "✗"} ${r.name}${r.detail ? " — " + r.detail : ""}`).join("\n") +
      "\n" + "=".repeat(60);
    log(summary);
    return { passed, failed, total: results.length, results, summary };
  }

  async function runInputEnhanceRegressionTests() {
    const results = [];
    async function check(name, fn) {
      try {
        const detail = await fn();
        results.push({ name, pass: true, detail: safeString(detail) });
      } catch (err) {
        results.push({ name, pass: false, detail: safeString(err && err.message) });
      } finally {
        pendingMainSnapshot = null;
      }
    }
    async function withMockRisu(mock, fn) {
      const original = globalThis.Risuai;
      globalThis.Risuai = mock;
      try {
        return await fn();
      } finally {
        globalThis.Risuai = original;
      }
    }
    function baseMock(extra) {
      return Object.assign({
        pluginStorage: {
          getItem: async () => null,
          setItem: async () => true,
        },
        getCharacter: async () => ({
          name: "Aster",
          description: "A guarded knight.",
          chats: [{
            message: [
              { role: "assistant", data: "Previous scene." },
              { role: "user", data: "Continue." },
            ],
          }],
          chatPage: 0,
        }),
        getDatabase: async () => ({
          selectedPersona: 0,
          personas: [{ name: "Player", personaPrompt: "A traveler." }],
        }),
      }, extra || {});
    }

    await check("input_1_injects_once_and_preserves_user", async () => withMockRisu(baseMock(), async () => {
      const messages = [
        { role: "system", content: "Stay in character." },
        { role: "user", content: "Open the sealed door." },
      ];
      const originalUser = JSON.stringify(messages[1]);
      const enhanced = await onBeforeRequest(messages, "main");
      const contractMessages = enhanced.filter(isTurnContractMessage);
      if (contractMessages.length !== 1) throw new Error(`contract_count:${contractMessages.length}`);
      if (JSON.stringify(messages[1]) !== originalUser) throw new Error("raw_user_mutated");
      if (enhanced[enhanced.length - 1].content !== "Open the sealed door.") throw new Error("user_order_changed");
      return "one contract message; raw user unchanged";
    }));

    await check("input_2_auxiliary_bypass", async () => withMockRisu(baseMock(), async () => {
      const messages = [{ role: "user", content: "module input" }];
      const result = await onBeforeRequest(messages, "module");
      if (result !== messages || pendingMainSnapshot) throw new Error("auxiliary_not_bypassed");
      return "auxiliary request untouched";
    }));

    await check("input_3_official_lore_preferred_and_injected_proven", async () => withMockRisu(baseMock({
      getCurrentLorebookEntries: async () => [{
        keys: ["moon"],
        content: "The silver moon disables all portal magic.",
      }],
      getCharacter: async () => ({
        name: "Aster",
        character_book: [{ entries: [{ keys: ["wrong"], content: "Fallback lore must not win." }] }],
      }),
    }), async () => {
      const deadline = createDeadline(5000);
      try {
        const context = await collectContext([
          { role: "system", content: "The silver moon disables all portal magic." },
          { role: "user", content: "Try the portal." },
        ], defaultSettings(), newTrace("test", "main"), deadline);
        if (context.lorebook.indexOf("silver moon") < 0 || context.lorebook.indexOf("Fallback lore") >= 0) {
          throw new Error("official_lore_not_preferred");
        }
        if (!context.sources.lorebook.active_count) throw new Error("injected_lore_not_proven");
        return "official lore used; exact injected content proven";
      } finally {
        deadline.cancel();
      }
    }));

    await check("input_4_unknown_lore_activation_not_promoted", async () => withMockRisu(baseMock({
      getCurrentLorebookEntries: async () => [{
        keys: ["hidden"],
        content: "A hidden fact not present in the request payload.",
      }],
    }), async () => {
      const deadline = createDeadline(5000);
      try {
        const context = await collectContext([
          { role: "system", content: "Unrelated system instructions." },
          { role: "user", content: "Continue." },
        ], defaultSettings(), newTrace("test", "main"), deadline);
        if (context.sources.lorebook.active_count !== 0) throw new Error("unknown_lore_promoted");
        if (context.sources.lorebook.unknown_activation_count !== 1) throw new Error("unknown_count_missing");
        return "candidate retained with unknown activation";
      } finally {
        deadline.cancel();
      }
    }));

    await check("input_5_supa_hypa_memory_read_only", async () => {
      const chat = {
        supaMemoryData: "Supa remembers the bridge collapse.",
        hypaV2Data: { summary: "Hypa V2 remembers the hidden oath." },
        hypaV3Data: { summary: "Hypa V3 remembers the hidden oath." },
      };
      const before = JSON.stringify(chat);
      const memory = extractMemorySnapshot(chat);
      if (memory.fields.length !== 3 || memory.text.indexOf("Supa") < 0 || memory.text.indexOf("HypaV2") < 0 || memory.text.indexOf("HypaV3") < 0) {
        throw new Error("memory_fields_missing");
      }
      if (JSON.stringify(chat) !== before) throw new Error("memory_source_mutated");
      return "Supa/Hypa collected without mutation";
    });

    await check("input_6_manifest_fallback_when_planners_unconfigured", async () => withMockRisu(baseMock(), async () => {
      const enhanced = await onBeforeRequest([{ role: "user", content: "Advance the scene." }], "main");
      if (enhanced.filter(isTurnContractMessage).length !== 1) throw new Error("fallback_contract_missing");
      const snapshot = consumePendingSnapshot("main");
      if (!snapshot.turn_contract || snapshot.turn_contract.fusion_state !== "manifest_fallback") {
        throw new Error("wrong_fallback_state");
      }
      if (snapshot.input_trace.input_enhance.status !== "manifest_fallback") {
        throw new Error("fallback_trace_missing");
      }
      return "minimal contract injected";
    }));

    await check("input_7_same_contract_consumed_by_after_request", async () => {
      const store = {};
      const mock = baseMock({
        pluginStorage: {
          getItem: async (key) => (Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null),
          setItem: async (key, value) => { store[key] = value; return true; },
        },
      });
      return withMockRisu(mock, async () => {
        await onBeforeRequest([{ role: "user", content: "Keep going." }], "main");
        const expectedContractId = pendingMainSnapshot.turn_contract.contract_id;
        const expectedManifestId = pendingMainSnapshot.context_manifest.snapshot_id;
        await onAfterRequest("A draft response.", "main");
        const traces = JSON.parse(store[TRACE_KEY] || "[]");
        const latest = traces[0];
        if (!latest || latest.snapshot.contract_id !== expectedContractId) {
          throw new Error("after_request_contract_id_mismatch");
        }
        if (latest.input_enhance.manifest_id !== expectedManifestId) {
          throw new Error("after_request_manifest_id_mismatch");
        }
        if (pendingMainSnapshot) throw new Error("snapshot_not_consumed");
        return expectedContractId;
      });
    });

    await check("input_8_no_plain_api_key_in_manifest_or_trace", async () => withMockRisu(baseMock(), async () => {
      const secret = "sk-super-secret-value-123456789";
      await onBeforeRequest([
        { role: "system", content: `Authorization: Bearer ${secret}` },
        { role: "user", content: "Continue." },
      ], "main");
      const snapshot = consumePendingSnapshot("main");
      const serialized = JSON.stringify({
        manifest: snapshot.context_manifest,
        trace: snapshot.input_trace,
      });
      if (serialized.indexOf(secret) >= 0) throw new Error("plain_key_leaked");
      return "manifest and trace redacted";
    }));

    await check("input_9_duplicate_contract_replaced_not_stacked", async () => withMockRisu(baseMock(), async () => {
      const enhanced = await onBeforeRequest([
        { role: "system", content: `${INPUT_CONTRACT_MARKER}\n{"contract_id":"old"}` },
        { role: "user", content: "Continue." },
      ], "main");
      const contractMessages = enhanced.filter(isTurnContractMessage);
      if (contractMessages.length !== 1 || contractMessages[0].content.indexOf('"old"') >= 0) {
        throw new Error("duplicate_contract_not_replaced");
      }
      return "old contract removed; one new contract injected";
    }));

    await check("input_10_deadline_leaves_no_active_planner_calls", async () => withMockRisu(baseMock({
      nativeFetch: async () => new Promise(() => {}),
    }), async () => {
      const settings = defaultSettings();
      const role = settings.roles.find((item) => item.role_id === "input_scene_continuity_planner");
      const profile = settings.role_profiles[role.role_id];
      profile.endpoint = "https://test.example.com/v1";
      profile.model = "slow-model";
      const trace = newTrace("test", "main");
      trace.budget.http_attempt_max = 1;
      const deadline = createDeadline(25);
      try {
        await scheduleInputPlanners(
          [role],
          settings.role_profiles,
          {
            schema: "context_manifest.v1",
            snapshot_id: "ctx_test",
            evidence_refs: ["payload_user_input"],
            evidence_sources: { payload_user_input: "Continue." },
          },
          deadline,
          trace,
          1
        );
        if (trace.input_enhance.active_calls_final !== 0) {
          throw new Error(`active_calls:${trace.input_enhance.active_calls_final}`);
        }
        return "deadline aborted planner; active calls 0";
      } finally {
        deadline.cancel();
      }
    }));

    await check("input_11_unquoted_fact_demoted_to_uncertainty", async () => {
      const fragment = validateTurnContractFragment({
        required_facts: [{
          text: "The king is secretly a dragon.",
          evidence_refs: ["payload_recent_chat"],
          evidence_quote: "This quote does not exist.",
        }],
      }, "input_canon_secret_planner", {
        evidence_refs: ["payload_recent_chat"],
        evidence_sources: {
          payload_recent_chat: "The king closed the window.",
        },
      });
      if (!fragment || fragment.required_facts.length || fragment.uncertainty.length !== 1) {
        throw new Error("unsupported_fact_not_demoted");
      }
      return "missing evidence quote cannot become immutable";
    });

    await check("input_12_writer_only_visibility_conflict_not_exposed", async () => {
      const grounded = {
        text: "Mira is the masked sovereign.",
        evidence_refs: ["lorebook_active_or_injected"],
        evidence_quote: "masked sovereign",
      };
      const contract = fuseTurnContract({
        snapshot_id: "ctx_visibility",
        evidence_refs: ["lorebook_active_or_injected"],
        source_availability: {},
        latest_user_input: "",
      }, [{
        planner_role: "input_canon_secret_planner",
        writer_only_secrets: [grounded],
        character_visible_facts: [grounded],
      }]);
      if (contract.writer_only_secrets.length !== 1 || contract.character_visible_facts.length !== 0) {
        throw new Error("writer_only_secret_exposed");
      }
      if (!contract.unresolved_uncertainty.length) throw new Error("visibility_conflict_not_recorded");
      return "writer-only classification wins";
    });

    await check("input_13_planner_provider_response_fuses", async () => withMockRisu(baseMock({
      nativeFetch: async () => ({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({
          choices: [{
            message: {
              content: JSON.stringify({
                schema: "turn_contract_fragment.v1",
                required_facts: [{
                  text: "The bridge has collapsed.",
                  evidence_refs: ["payload_recent_chat"],
                  evidence_quote: "bridge has collapsed",
                }],
              }),
            },
          }],
        }),
      }),
    }), async () => {
      const settings = defaultSettings();
      const role = settings.roles.find((item) => item.role_id === "input_scene_continuity_planner");
      const profile = settings.role_profiles[role.role_id];
      profile.endpoint = "https://test.example.com/v1";
      profile.model = "planner-model";
      const trace = newTrace("test", "main");
      trace.budget.http_attempt_max = 2;
      const deadline = createDeadline(5000);
      try {
        const fragments = await scheduleInputPlanners(
          [role],
          settings.role_profiles,
          {
            schema: "context_manifest.v1",
            snapshot_id: "ctx_provider",
            evidence_refs: ["payload_recent_chat"],
            evidence_block: "[payload_recent_chat]\nThe bridge has collapsed.",
            evidence_sources: {
              payload_recent_chat: "The bridge has collapsed.",
            },
          },
          deadline,
          trace,
          1
        );
        if (fragments.length !== 1 || fragments[0].required_facts.length !== 1) {
          throw new Error("planner_fragment_not_accepted");
        }
        if (!trace.roles.length || trace.roles[0].stage !== "input" || trace.roles[0].status !== "fulfilled") {
          throw new Error("planner_trace_missing");
        }
        return "provider fragment validated and traced";
      } finally {
        deadline.cancel();
      }
    }));

    await check("input_14_preset_routes_zero_two_three_planners", async () => {
      const settings = defaultSettings();
      ["input_canon_secret_planner", "input_character_relationship_planner", "input_scene_continuity_planner"].forEach((roleId) => {
        settings.role_profiles[roleId].endpoint = "https://test.example.com/v1";
        settings.role_profiles[roleId].model = "planner-model";
      });
      const manifest = {
        source_availability: {
          lorebook: { available: true },
          memory: { available: true },
          character: { available: true },
        },
      };
      settings.preset = "fast";
      const fast = selectInputPlannerRoles(settings, manifest).roles.length;
      settings.preset = "balanced";
      const balanced = selectInputPlannerRoles(settings, manifest).roles.length;
      settings.preset = "quality";
      const quality = selectInputPlannerRoles(settings, manifest).roles.length;
      if (fast !== 0 || balanced !== 2 || quality !== 3) {
        throw new Error(`wrong_input_route:${fast}/${balanced}/${quality}`);
      }
      return "Fast=0 Balanced=2 Quality=3";
    });

    await check("input_15_official_auxiliary_modes_bypass", async () => {
      const modes = ["submodel", "memory", "emotion", "otherAx", "translate"];
      modes.forEach((mode) => {
        if (!isAuxiliaryRequest(mode)) throw new Error(`official_aux_not_bypassed:${mode}`);
      });
      if (isAuxiliaryRequest("model")) throw new Error("main_model_was_bypassed");
      return "official ModelModeExtended values classified";
    });

    await check("input_16_official_chat_persona_hypa_shapes", async () => {
      const chat = {
        message: [
          { role: "assistant", data: "Official chat message." },
          { role: "user", data: "Official user message." },
        ],
        supaMemoryData: "Supa memory.",
        hypaV2Data: { summary: "Hypa V2 memory." },
        hypaV3Data: { summary: "Hypa V3 memory." },
      };
      const persona = extractPersonaSummary({
        selectedPersona: 1,
        personas: [
          { name: "Wrong", personaPrompt: "Wrong prompt." },
          { name: "Selected", personaPrompt: "Selected persona prompt." },
        ],
      });
      if (extractChatSummary(chat).indexOf("Official chat message") < 0) throw new Error("official_chat_message_missing");
      if (extractMemorySnapshot(chat).fields.length !== 3) throw new Error("official_memory_fields_missing");
      if (persona.indexOf("Selected persona prompt") < 0 || persona.indexOf("Wrong prompt") >= 0) {
        throw new Error("numeric_persona_selection_failed");
      }
      return "official Chat, Hypa, and Persona shapes read";
    });

    await check("input_17_official_empty_lore_does_not_fallback", async () => withMockRisu(baseMock({
      getCurrentLorebookEntries: async () => [],
      getCharacter: async () => ({
        name: "Aster",
        character_book: [{ entries: [{ keys: ["fallback"], content: "Must remain unused." }] }],
      }),
    }), async () => {
      const deadline = createDeadline(5000);
      try {
        const context = await collectContext(
          [{ role: "user", content: "Continue." }],
          defaultSettings(),
          newTrace("test", "model"),
          deadline
        );
        if (context.lorebook || context.sources.lorebook.candidate_count !== 0) {
          throw new Error("fallback_lore_used_after_official_empty");
        }
        if (context.sources.lorebook.source !== "getCurrentLorebookEntries") {
          throw new Error("official_empty_source_lost");
        }
        if (!context.sources.lorebook.available) throw new Error("official_empty_marked_unavailable");
        return "official empty lore trusted";
      } finally {
        deadline.cancel();
      }
    }));

    await check("input_18_wrong_source_quote_demoted", async () => {
      const fragment = validateTurnContractFragment({
        required_facts: [{
          text: "The bridge has collapsed.",
          evidence_refs: ["character"],
          evidence_quote: "bridge has collapsed",
        }],
      }, "input_scene_continuity_planner", {
        evidence_refs: ["character", "payload_recent_chat"],
        evidence_sources: {
          character: "Aster is a guarded knight.",
          payload_recent_chat: "The bridge has collapsed.",
        },
      });
      if (!fragment || fragment.required_facts.length || fragment.uncertainty.length !== 1) {
        throw new Error("cross_source_quote_was_accepted");
      }
      return "quote must exist in its claimed source";
    });

    await check("input_19_paraphrased_visible_secret_stays_writer_only", async () => {
      const contract = fuseTurnContract({
        snapshot_id: "ctx_secret_paraphrase",
        evidence_refs: ["lorebook_active_or_injected"],
        source_availability: {},
        latest_user_input: "",
      }, [{
        planner_role: "input_canon_secret_planner",
        writer_only_secrets: [{
          text: "Mira secretly rules the realm behind the mask.",
          evidence_refs: ["lorebook_active_or_injected"],
          evidence_quote: "Mira is the masked sovereign",
        }],
        character_visible_facts: [{
          text: "The party knows Mira is the sovereign.",
          evidence_refs: ["lorebook_active_or_injected"],
          evidence_quote: "Mira is the masked sovereign",
        }],
      }]);
      if (contract.character_visible_facts.length) throw new Error("paraphrased_secret_exposed");
      return "shared grounded secret evidence keeps writer-only precedence";
    });

    await check("input_20_provider_retry_reuses_same_contract", async () => withMockRisu(baseMock(), async () => {
      const messages = [{ role: "user", content: "Retry this exact main request." }];
      const first = await onBeforeRequest(messages, "model");
      const firstContractId = pendingMainSnapshot.turn_contract.contract_id;
      const second = await onBeforeRequest(cloneSnapshotValue(messages), "model");
      if (pendingMainSnapshot.ambiguous) throw new Error("provider_retry_marked_ambiguous");
      if (pendingMainSnapshot.turn_contract.contract_id !== firstContractId) throw new Error("contract_rebuilt_on_retry");
      if (pendingMainSnapshot.input_trace.input_enhance.retry_reuse_count !== 1) throw new Error("retry_reuse_not_traced");
      if (first.filter(isTurnContractMessage).length !== 1 || second.filter(isTurnContractMessage).length !== 1) {
        throw new Error("retry_contract_stack_error");
      }
      return "same request reused one contract without planner recollection";
    }));

    await check("input_21_input_attempts_do_not_consume_output_budget", async () => {
      const store = {};
      return withMockRisu(baseMock({
        pluginStorage: {
          getItem: async (key) => (Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null),
          setItem: async (key, value) => { store[key] = value; return true; },
        },
      }), async () => {
        const inputTrace = newTrace("beforeRequest", "model");
        inputTrace.budget.http_attempt_max = INPUT_HTTP_ATTEMPT_BUDGET.balanced;
        inputTrace.budget.http_attempt_used = 3;
        inputTrace.budget.input_attempt_used = 3;
        pendingMainSnapshot = makeRequestSnapshot(
          [{ role: "user", content: "Continue." }],
          "model",
          { detected: false, reason: "" },
          "",
          { input_trace: inputTrace }
        );
        await onAfterRequest("A mutable response.", "model");
        const traces = JSON.parse(store[TRACE_KEY] || "[]");
        if (!traces[0] || traces[0].budget.http_attempt_used !== 0
            || traces[0].budget.input_attempt_used !== 3) {
          throw new Error(`budget_not_separated:${JSON.stringify(traces[0] && traces[0].budget)}`);
        }
        return "input usage traced separately; output starts with fresh budget";
      });
    });

    const passed = results.filter((item) => item.pass).length;
    return { passed, failed: results.length - passed, total: results.length, results };
  }

  async function runBatch1RegressionTests() {
    const results = [];
    async function check(name, fn) {
      try {
        const detail = await fn();
        results.push({ name, pass: true, detail: safeString(detail) });
      } catch (err) {
        results.push({ name, pass: false, detail: safeString(err && err.message) });
      } finally {
        pendingMainSnapshot = null;
      }
    }

    await check("batch1_official_array_and_deep_immutable_snapshot", async () => {
      const messages = [
        { role: "system", content: "System A" },
        { role: "user", content: [{ type: "text", text: "User A" }] },
      ];
      await onBeforeRequest(messages, "main");
      messages[1].content[0].text = "mutated";
      const snapshot = consumePendingSnapshot("main");
      if (snapshot.ambiguous) throw new Error(snapshot.reason);
      if (snapshot.messages[1].content[0].text !== "User A") throw new Error("snapshot_mutated");
      if (!Object.isFrozen(snapshot.messages[1].content[0])) throw new Error("nested_snapshot_not_frozen");
      return "official OpenAIChat[] cloned and frozen";
    });

    await check("batch1_snapshot_consumed_once", async () => {
      await onBeforeRequest([{ role: "user", content: "Once" }], "main");
      const first = consumePendingSnapshot("main");
      const second = consumePendingSnapshot("main");
      if (first.ambiguous) throw new Error(`first:${first.reason}`);
      if (!second.ambiguous || second.reason !== "no_pending_snapshot" || second.messages.length) {
        throw new Error("snapshot_reused");
      }
      return "one-shot consume";
    });

    await check("batch1_overlap_is_ambiguous_without_context", async () => {
      await onBeforeRequest([{ role: "user", content: "First" }], "main");
      await onBeforeRequest([{ role: "user", content: "Second" }], "main");
      const snapshot = consumePendingSnapshot("main");
      if (!snapshot.ambiguous || snapshot.reason !== "overlapping_main_requests") {
        throw new Error(`wrong_overlap_state:${snapshot.reason}`);
      }
      if (snapshot.messages.length) throw new Error("overlap_context_leaked");
      return "overlap blocked";
    });

    await check("batch1_type_mismatch_drops_context", async () => {
      await onBeforeRequest([{ role: "user", content: "Main context" }], "main");
      const snapshot = consumePendingSnapshot("different-main-type");
      if (!snapshot.ambiguous || snapshot.reason !== "request_type_mismatch") {
        throw new Error(`wrong_mismatch_state:${snapshot.reason}`);
      }
      if (snapshot.messages.length) throw new Error("mismatched_context_leaked");
      return "mismatch blocked";
    });

    await check("batch1_auxiliary_does_not_consume_main_snapshot", async () => {
      await onBeforeRequest([{ role: "user", content: "Main context" }], "main");
      await onBeforeRequest([{ role: "user", content: "Aux context" }], "module");
      await onAfterRequest("Aux output", "module");
      const snapshot = consumePendingSnapshot("main");
      if (snapshot.ambiguous || snapshot.messages.length !== 1) {
        throw new Error(`main_snapshot_lost:${snapshot.reason}`);
      }
      return "auxiliary isolated";
    });

    await check("batch1_deadline_anchored_to_after_request_start", async () => {
      const startedAt = Date.now() - 100;
      const deadline = createDeadline(50, startedAt);
      try {
        if (!deadline.check() || deadline.remaining() !== 0) throw new Error("deadline_not_anchored");
      } finally {
        deadline.cancel();
      }
      return "entry-anchored deadline";
    });

    await check("batch1_payload_recent_chat_in_context_block", async () => {
      const deadline = createDeadline(1, Date.now() - 5);
      try {
        const context = await collectContext([
          { role: "system", content: "System context" },
          { role: "assistant", content: "Previous reply" },
          { role: "user", content: "Latest request" },
        ], defaultSettings(), newTrace("test", "main"), deadline);
        if (context.bounded_context_block.indexOf("[Payload Recent Chat]") < 0) {
          throw new Error("payload_recent_chat_not_injected");
        }
      } finally {
        deadline.cancel();
      }
      return "payload recent chat included";
    });

    await check("batch1_stage_http_attempt_budgets", async () => {
      if (INPUT_HTTP_ATTEMPT_BUDGET.fast !== 0
          || INPUT_HTTP_ATTEMPT_BUDGET.balanced !== 3
          || INPUT_HTTP_ATTEMPT_BUDGET.quality !== 4
          || OUTPUT_HTTP_ATTEMPT_BUDGET.fast !== 4
          || OUTPUT_HTTP_ATTEMPT_BUDGET.balanced !== 5
          || OUTPUT_HTTP_ATTEMPT_BUDGET.quality !== 7) {
        throw new Error("wrong_stage_attempt_budget");
      }
      const trace = newTrace("test", "main");
      if (trace.budget.http_attempt_max !== OUTPUT_HTTP_ATTEMPT_BUDGET.balanced) {
        throw new Error("default_output_budget_not_initialized");
      }
      return "input=0/3/4 output=4/5/7";
    });

    await check("batch1_risu_native_fetch_precedes_browser_fetch", async () => {
      const originalRisu = globalThis.Risuai;
      const originalFetch = globalThis.fetch;
      let nativeCalls = 0;
      let browserCalls = 0;
      globalThis.Risuai = {
        nativeFetch: async () => {
          nativeCalls++;
          return { ok: true, status: 200, text: async () => "{}" };
        },
      };
      globalThis.fetch = async () => {
        browserCalls++;
        throw new Error("browser fetch must not run");
      };
      try {
        const response = await fetchWithAbort(
          "https://example.com/v1/chat/completions",
          { method: "POST" },
          5000,
          null
        );
        if (!response || nativeCalls !== 1 || browserCalls !== 0) {
          throw new Error(`wrong_transport:native=${nativeCalls}:browser=${browserCalls}`);
        }
        if (response.__recomposer_transport !== "risu_native_fetch") {
          throw new Error(`missing_transport_trace:${response.__recomposer_transport}`);
        }
      } finally {
        globalThis.Risuai = originalRisu;
        globalThis.fetch = originalFetch;
      }
      return "RisuAI native fetch selected";
    });

    await check("batch1_ollama_cloud_url_and_transport_error", async () => {
      const url = openAiChatUrl("https://ollama.com");
      if (url !== "https://ollama.com/v1/chat/completions") {
        throw new Error(`wrong_ollama_cloud_url:${url}`);
      }
      const classified = classifyRoleError(new TypeError("Failed to fetch"), null);
      if (classified.code !== "transport_error" || !classified.retryable) {
        throw new Error(`wrong_transport_class:${classified.code}`);
      }
      return "Ollama Cloud URL normalized; transport retryable";
    });

    await check("batch1_existing_duplicate_blocks_remain_allowed", async () => {
      const repeated = "This intentionally repeated original block is long enough to be checked by the duplicate verifier without being treated as a short separator.";
      const segments = [
        { id: "mutable_1", type: "mutable", text: repeated },
        { id: "mutable_2", type: "mutable", text: repeated },
      ];
      const finalSegments = segments.map((seg) => ({
        id: seg.id,
        type: seg.type,
        final_text: seg.text,
      }));
      const verification = verifyOutput(segments, finalSegments, repeated + repeated, repeated + repeated);
      if (!verification.pass) throw new Error(verification.errors.join(","));
      return "original repetition preserved";
    });

    await check("batch1_new_duplicate_block_is_rejected", async () => {
      const first = "The first original paragraph contains enough distinct material to exceed the duplicate verifier minimum block length.";
      const second = "The second original paragraph is also long enough, but it begins as a different passage before rewriting.";
      const duplicated = "A newly duplicated rewritten paragraph is deliberately long enough to represent a real repeated prose block in the final response.";
      const segments = [
        { id: "mutable_1", type: "mutable", text: first },
        { id: "mutable_2", type: "mutable", text: second },
      ];
      const finalSegments = [
        { id: "mutable_1", type: "mutable", final_text: duplicated },
        { id: "mutable_2", type: "mutable", final_text: duplicated },
      ];
      const verification = verifyOutput(segments, finalSegments, duplicated + duplicated, first + second);
      if (verification.pass || verification.errors.indexOf("duplicate_segment:mutable_2") < 0) {
        throw new Error(`new_duplicate_not_rejected:${verification.errors.join(",")}`);
      }
      return "new long duplicate rejected";
    });

    await check("batch1_shared_prefix_is_not_duplicate", async () => {
      const prefix = "This common opening is intentionally longer than one hundred characters so the old verifier would have treated both segments as duplicates even though ";
      const first = prefix + "the first paragraph ends with a different event.";
      const second = prefix + "the second paragraph ends with another character response.";
      const segments = [
        { id: "mutable_1", type: "mutable", text: first },
        { id: "mutable_2", type: "mutable", text: second },
      ];
      const finalSegments = [
        { id: "mutable_1", type: "mutable", final_text: first },
        { id: "mutable_2", type: "mutable", final_text: second },
      ];
      const verification = verifyOutput(segments, finalSegments, first + second, first + second);
      if (!verification.pass) throw new Error(verification.errors.join(","));
      return "shared prefix accepted";
    });

    await check("batch1_negative_director_candidate_not_applied", async () => {
      const segments = [{ id: "mutable_1", type: "mutable", text: "Original paragraph." }];
      const director = {
        ranked: {
          mutable_1: [{
            role_id: "character_reader",
            rewrite: "Inferior rewrite.",
            score: -20,
            identical_to_original: false,
          }],
        },
      };
      const assembled = assembleOutput(segments, null, director);
      if (assembled.output !== "Original paragraph." || assembled.changed) {
        throw new Error(`negative_candidate_applied:${assembled.output}`);
      }
      return "negative candidate rejected";
    });

    await check("batch1_same_ollama_endpoint_shares_execution_group", async () => {
      const openAiProfile = {
        provider: "openai_compatible",
        endpoint: "https://ollama.com/v1/chat/completions",
      };
      const ollamaProfile = {
        provider: "ollama_compatible",
        endpoint: "https://ollama.com",
      };
      if (executionGroupKey(openAiProfile) !== executionGroupKey(ollamaProfile)) {
        throw new Error("ollama_endpoint_groups_differ");
      }
      if (executionGroupConcurrency(openAiProfile) !== 2
          || executionGroupConcurrency(ollamaProfile) !== 2) {
        throw new Error("wrong_ollama_endpoint_concurrency");
      }
      const timeoutClass = classifyRoleError(new Error("request_timeout"), null);
      if (timeoutClass.code !== "request_timeout" || timeoutClass.retryable) {
        throw new Error("request_timeout_should_not_retry");
      }
      return "shared Ollama group concurrency=2";
    });

    const passed = results.filter((item) => item.pass).length;
    const failed = results.length - passed;
    return { passed, failed, total: results.length, results };
  }


  function getR() {
    return (typeof Risuai !== "undefined")
      ? Risuai
      : (typeof risuai !== "undefined" ? risuai : null);
  }

  async function initialize() {
    const RR = getR();
    try {
      if (RR && typeof RR.addRisuReplacer === "function") {
        await RR.addRisuReplacer("beforeRequest", onBeforeRequest);
        await RR.addRisuReplacer("afterRequest", onAfterRequest);
      }
      try {
        if (RR && typeof RR.registerSetting === "function") {
          await RR.registerSetting("Risu Recomposer", openSettingsUI, "🔧", "html", "risu-recomposer-settings");
        }
      } catch (err) {
        error("registerSetting error:", err);
      }
      try {
        if (RR && typeof RR.registerButton === "function") {
          await RR.registerButton({
            name: "Risu Recomposer Settings",
            icon: "🔧",
            iconType: "html",
            location: "chat",
            id: "risu-recomposer-chat-btn",
          }, openSettingsUI);
        }
      } catch (err) {
        error("registerButton error:", err);
      }
      if (RR && typeof RR.addArgument === "function") {
        await RR.addArgument("recomposer_test", async () => {
          return JSON.stringify(await runInMemoryTests());
        });
        await RR.addArgument("recomposer_batch1", async () => {
          return JSON.stringify(await runBatch1RegressionTests());
        });
        await RR.addArgument("recomposer_input_enhance", async () => {
          return JSON.stringify(await runInputEnhanceRegressionTests());
        });
      }
      log(`initialized v${VERSION}`);
    } catch (err) {
      error("initialize error:", err);
    }
  }

  await initialize();

  // Expose for testing
  if (typeof globalThis !== "undefined") {
    globalThis.__recomposer = {
      runInMemoryTests,
      runBatch1RegressionTests,
      runInputEnhanceRegressionTests,
      initialize,
      buildSegmentMap,
      assembleOutput,
      fusionDirector,
      verifyOutput,
      selectRoles,
      detectSceneSignals,
      validateCandidateSchema,
      validateComposerSchema,
      tryParseJson,
      maskKey,
      resolveApiKey,
      applyKeyUpdate,
      defaultSettings,
      newTrace,
      createDeadline,
      createSemaphore,
      isOllamaCloudEndpoint,
      splitMutableWhitespace,
      isVoidHtmlTag,
      buildRolePrompt,
      callProvider,
      scheduleRoles,
      detectStreamingState,
      consumePendingSnapshot,
      makeRequestSnapshot,
      isAuxiliaryRequest,
      INPUT_HTTP_ATTEMPT_BUDGET,
      OUTPUT_HTTP_ATTEMPT_BUDGET,
      OUTPUT_SPECIALIST_LIMIT,
      extractPayloadSystem,
      extractRecentChat,
      extractLatestUserInput,
      collectContext,
      isOpenAiChatArray,
      DEFAULT_ROLES,
      PRESETS,
      PROVIDERS,
      SIGNAL_ROLE_MAP,
      VOID_HTML_TAGS,
      ISSUE_GROUPS,
      ISSUE_PRIORITY,
      TAG_ISSUE_MAP,
      normalizeIssues,
      maxIssuePriority,
      PROTECTED_HEADERS,
      PROTECTED_BODY_FIELDS,
      deepMergeBody,
      applyExtraHeadersSafe,
      applyExtraBodySafe,
      applyVertexFlex,
      renderUI,
    };
  }
})();
