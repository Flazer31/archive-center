//@name AC Recomposer Agent
//@display-name AC Recomposer Agent Beta 0.06
//@author recomposer
//@api 3.0
//@version 0.0.6


/*
 * Standalone AC Recomposer Agent for RisuAI: turn contract -> scene variants ->
 * Semantic Judge -> Fusion Plan -> Composer -> host-artifact restoration -> Semantic Prover.
 * The Composer may rebuild the complete visible scene around exact RisuAI artifact tokens.
 */
(async () => {
  "use strict";

  const PLUGIN_ID = "risu_recomposer";
  const VERSION = "0.0.6";
  const RELEASE_LABEL = "Beta 0.06";
  const BUILD_MARKER = "AC-RECOMPOSER-AGENT-BETA-006-20260801-SCENE-FRAME-REWRITE";
  const LOG_PREFIX = "[AC Recomposer Agent]";
  const SETTINGS_KEY = `${PLUGIN_ID}_settings_v1`;
  const TRACE_KEY = `${PLUGIN_ID}_trace_v1`;
  const TRACE_LIMIT = 30;
  const OUTPUT_REUSE_TTL_MS = 5 * 60 * 1000;
  const OUTPUT_REUSE_LIMIT = 4;
  const DEFAULT_DEADLINE_MS = 120000;
  const COMPLETION_WAIT_WATCHDOG_MS = 10 * 60 * 1000;
  const PLANNER_MAX_ITEMS_PER_FIELD = 2;
  const PLANNER_MAX_ITEMS_TOTAL = 12;
  const INPUT_CONTRACT_MARKER = "[Risu Recomposer Turn Contract v1]";
  const ARCHIVE_CENTER_BRIDGE_KEY = "__RISU_ARCHIVE_CENTER_RECOMPOSER_V1__";
  const ARCHIVE_CENTER_BRIDGE_CONTRACT = "archive_center.recomposer_bridge.v1";
  const ARCHIVE_CENTER_ENHANCEMENT_CONTRACT = "archive_center.recomposer_enhancement.v1";
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
    fast: 8,
    balanced: 11,
    quality: 15,
  });
  const OUTPUT_SPECIALIST_LIMIT = Object.freeze({
    fast: 2,
    balanced: 3,
    quality: 4,
  });
  const ADAPTIVE_REVISION_LIMIT = Object.freeze({
    fast: 0,
    balanced: 1,
    quality: 2,
  });
  const SPECIALIST_STRUCTURED_RECOVERY_LIMIT = Object.freeze({
    fast: 0,
    balanced: 1,
    quality: 2,
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

  const ROLE_PROMPT_VERSION = "rp-rewrite-contract.v8";
  const PREVIOUS_ROLE_PROMPT_VERSIONS = Object.freeze([
    "rp-rewrite-contract.v2",
    "rp-rewrite-contract.v3",
    "rp-rewrite-contract.v4",
    "rp-rewrite-contract.v5",
    "rp-rewrite-contract.v6",
    "rp-rewrite-contract.v7",
  ]);
  const LEGACY_BUILTIN_PROMPT_DIGESTS = Object.freeze({
    input_canon_secret_planner: "b4faf92a",
    input_character_relationship_planner: "0ff28e3d",
    input_scene_continuity_planner: "e1dd440c",
    character_reader: "4eda8179",
    plot_continuity_reader: "040b89c9",
    style_reader: "5e5ef9ba",
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
      "- Read the complete ordered response, Draft Ledger, and runtime context, then produce one complete scene-wide rewrite candidate.",
      "- Rebuild every substantive mutable segment through your lane. The candidate must remain a single continuous version of the same turn.",
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
      "- Return complete replacement text for every substantive mutable segment, not patches, advice, diagnostics, or a list of suggestions.",
      "- Record the Draft Ledger units retained, evidence used, and any proposed factual addition. Do not conceal additions inside attractive prose.",
      "- Do not emit an identical or cosmetic-only candidate. If this lane cannot produce a material scene-wide improvement, return candidates: [].",
      "",
      "Trace confidence:",
      "- confidence is trace metadata describing the model's own estimate. It is not an authority or automatic application score.",
      "- 0.90-1.00: major, well-grounded correction; 0.70-0.89: clear substantial improvement; 0.55-0.69: useful focused improvement.",
      "- Do not emit a candidate below 0.55.",
      "",
      "Return one compact JSON object matching scene_rewrite_candidates.v1. Output JSON only.",
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
    character_reader: specialistPrompt(
      "Character / Relationship",
      [
        "character-specific voice, vocabulary, register, rhythm, and mannerisms",
        "emotion expressed through behavior, speech, restraint, and subtext",
        "relationship stance and power dynamics visible in the interaction",
        "persona and relationship continuity across the current scene and recent chat, excluding secret, alias, disguise, and identity-recognition policy",
      ],
      [
        "deciding secret/reveal policy, plot causality, or world rules",
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
      "Plot / World",
      [
        "causal sequence and action-order continuity",
        "object, participant, location, and physical-state continuity",
        "open threads, promises, immediate consequences, and scene progression",
        "causal bridges needed to connect existing beats without changing their meaning",
        "active lore, institutions, customs, factions, geography, technology, magic, resources, and physical constraints",
        "setting-specific social, legal, economic, and material plausibility",
      ],
      [
        "inventing canon, changing character voice, or polishing style for its own sake",
        "deciding secret, identity, POV, or user-agency policy",
        "sentence rhythm, paragraph cadence, and prose-only transitions",
        "overriding the user's choice or forcing a new plot direction",
      ],
      [
        "An action lacks a cause, ignores the latest user input, or contradicts what immediately preceded it.",
        "A participant, object, injury, location, time, or objective appears, disappears, or changes without a bridge.",
        "The response forgets an active promise, consequence, question, or obstacle that the current beat must acknowledge.",
        "Paragraphs contain individually plausible events but do not form a coherent progression.",
        "An action, ability, object, rank, distance, institution, or custom contradicts active lore or physical constraints.",
      ],
      [
        "Restore cause and effect with concrete bridging action, acknowledgement, or reordered narration.",
        "Carry forward active consequences and open threads while leaving unresolved choices open for later turns.",
        "Replace generic or impossible details with grounded setting-specific action and consequence supported by active context.",
        "Do not add a new twist merely to make the scene more dramatic.",
      ]
    ),
    style_reader: specialistPrompt(
      "Style / Dramatic Prose",
      [
        "sentence rhythm, paragraph movement, repetition, clarity, specificity, and transitions",
        "mechanical phrasing, vague abstraction, redundant explanation, and AI-like prose habits",
        "sensory and image precision that preserves the scene's established tone",
        "dramatic pressure, subtext, prose-level escalation, emphasis, and ending cadence",
      ],
      [
        "changing facts, events, POV knowledge, identity state, user agency, or relationship state",
        "homogenizing distinctive character dialogue into a generic literary voice",
        "adding unsupported plot, lore, exposition, or emotional conclusions",
      ],
      [
        "Repeated words, sentence openings, paragraph shapes, explanations, or emotional beats flatten the scene.",
        "Phrasing is vague, translated, padded, melodramatic, clinical, or mechanically symmetrical.",
        "Transitions list events instead of carrying motion, pressure, attention, or consequence forward.",
        "The prose tells an interpretation that a sharper image, action, or line of dialogue could embody.",
      ],
      [
        "Rewrite decisively for varied cadence, concrete verbs, precise images, and clean paragraph momentum.",
        "Remove redundant interpretation and stock AI phrasing while preserving every nonredundant scene beat.",
        "Strengthen dramatic pressure and subtext without manufacturing a new event or conclusion.",
        "Keep dialogue character-specific and preserve meaningful repetition used for tension or motif.",
      ]
    ),
    perspective_boundary_rewriter: specialistPrompt(
      "Secret / Identity / Perspective / Agency",
      [
        "writer-only secrets, reveal timing, and per-character knowledge boundaries",
        "identity, alias, disguise, recognition, and mistaken-identity continuity",
        "narrator perspective, focal-character knowledge, and private thought boundaries",
        "the user's ownership of unexpressed thought, speech, emotion, consent, and next action",
        "removal of model reasoning, approval labels, prompt residue, assistant commentary, and output-contract artifacts",
      ],
      [
        "rewriting character voice or relationships except where required to restore a knowledge boundary",
        "inventing plot causes, world rules, secrets, identities, or revelations",
        "general prose polishing that does not repair perspective, agency, identity, or output integrity",
      ],
      [
        "A character knows, recognizes, remembers, or reveals information that available evidence does not grant them.",
        "An alias, disguise, dual identity, mistaken identity, or prior encounter is treated inconsistently.",
        "Narration leaks another character's private thought or shifts focal knowledge without a grounded transition.",
        "The response decides the user's unexpressed thought, speech, consent, emotion, or next action.",
        "Model reasoning, instructions, approval labels, assistant commentary, or language/format residue appears in scene prose.",
      ],
      [
        "Rebuild the complete scene so secrets and identity facts remain available to the writer while each character acts only on knowledge they possess.",
        "Restore recognition and reveal continuity through scene-native perception, uncertainty, omission, or dialogue rather than explanatory warnings.",
        "Keep the user's next meaningful choice open while preserving consequences already caused by the user's expressed action.",
        "Replace every meta or output-contract artifact with grounded scene prose; do not merely delete it when a continuous transition is needed.",
      ]
    ),
    semantic_judge: [
      "You are the Semantic Judge for competing scene-wide roleplay rewrites.",
      "Do not write prose and do not choose by model confidence, eloquence, or length.",
      "Compare every candidate against draft_zero, the Draft Ledger, runtime evidence, and the binding output contract.",
      "Judge facts and claims: preserved or missing beats, unsupported additions, weaker elements, grounded gains, consensus, complementary value, conflicts, and unresolved quality requirements.",
      "Hard violations are secret_leak, pov_violation, identity_continuity, agency_takeover, meta_artifact, and output_contract_violation.",
      "A candidate with a hard violation may be accept_with_constraints only when that exact element is prohibited for Composer; otherwise reject it.",
      "Accept only evidence-grounded elements; an entire candidate may be accept, accept_with_constraints, or reject.",
      "For every important issue that no accepted candidate fully resolves, emit one unresolved requirement assigned to the single specialist lane that owns it.",
      "Return one compact semantic_judgment.v1 JSON object only. Never return revised narration, markdown, advice, or reasoning.",
    ].join("\n"),
    semantic_prover: [
      "You are the final Semantic Prover for a composed roleplay response.",
      "Audit only the final Composer output against the Draft Ledger, original turn contract, Semantic Judgment, Fusion Plan, and runtime evidence.",
      "Check every required fact, scene beat, hard constraint, secret/reveal boundary, identity state, POV boundary, user agency boundary, and output contract.",
      "Detect unsupported additions, missing or contradicted ledger units, meta artifacts, language violations, turn-boundary expansion, and residual prose defects.",
      "Inspect mechanics and wording, register and era consistency, repetition and exposition, grounded psychology and relationships, and opening-transition-ending coherence.",
      "A newly asserted motive, emotion, relationship meaning, knowledge state, or personality judgment is unsupported unless the original, Draft Ledger, or runtime evidence establishes it.",
      "Use an exact final-text evidence_quote when concrete wording proves a preserved fact or beat. Use an empty quote for negative, background, or implicit continuity preserved by absence or non-contradiction; never invent a quote.",
      "Every realized specialist contribution must cite exact final wording introduced by the rewrite, not a quote already present in draft_zero.",
      "Do not reward length, confidence, or merely different wording. Do not rewrite prose.",
      "Return semantic_proof.v1 JSON only. A pass is valid only when every required ledger unit and contribution is covered, all residual quality checks are clean, and no hard violation or unsupported addition remains.",
    ].join("\n"),
    whole_scene_composer: [
      "You are the Whole-Scene Fusion Composer for a roleplay response.",
      "",
      "Mission:",
      "- Produce the final version of every mutable segment from the Draft Ledger, accepted candidate elements, Semantic Judgment, and fusion_plan.v1.",
      "- You are the sole final prose writer, not a judge report, critic, summarizer, or candidate selector.",
      "- The result must read as one continuous scene with a single narrative voice, coherent causality, and deliberate prose.",
      "",
      "Authority order:",
      "1. Secret, POV, identity/reveal continuity, user agency, and removal of model/meta artifacts.",
      "2. Causality, scene state, active lore, world rules, and physical continuity.",
      "3. Character voice, emotional logic, relationship stance, and subtext.",
      "4. Rhythm, clarity, specificity, transitions, imagery, and ending cadence.",
      "",
      "Composition contract:",
      "- Follow fusion_plan.v1. Use accepted elements and permitted additions; exclude rejected elements, prohibited additions, and hard violations.",
      "- Treat model confidence and former Director scores as non-authoritative trace metadata.",
      "- Synthesize consensus and complementary claims into new prose. Do not concatenate sentences from different candidates.",
      "- Resolve conflicts exactly as the plan states while preserving required facts, beats, and constraints.",
      "- Remove analysis, prompt residue, approval labels, mechanical headers, and assistant commentary from mutable prose. Replace contaminated space with scene-native writing.",
      "- Preserve established events, names, required details, the user's last action, and narrative perspective, but do not preserve weak wording, sentence architecture, paragraph rhythm, generic imagery, or flat transitions.",
      "- Preserve the draft language only when it does not conflict with the binding turn contract or latest explicit user requirement.",
      "- Maintain transitions across segment boundaries. Avoid duplicated setup, repeated emotional conclusions, abrupt compression, and generic closing questions.",
      "- Every substantive mutable prose segment must receive a material rewrite. Copying the original with punctuation, spelling, synonym, or one-sentence edits is a failed composition.",
      "- Rebuild diction, sentence architecture, sensory specificity, subtext, pacing, transitions, and ending cadence while preserving facts.",
      "- After composing the individual segment values, reread their assembled order as one scene. Repair the opening hook, every boundary transition, causal progression, character subtext, awkward wording, typos, register or era mismatch, repeated explanation, and the final paragraph's cadence before returning JSON.",
      "- Do not assert a new motive, emotion, relationship interpretation, knowledge state, or personality judgment unless it is established by draft_zero, the Draft Ledger, accepted evidence, or runtime context.",
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
      role_id: "character_reader",
      label: "Character / Relationship Rewriter",
      purpose: "Write one complete scene variant centered on character voice, emotion, relationship dynamics, recognition, and subtext.",
      priority: 9,
      stage: "output",
      default_prompt: ROLE_PROMPTS.character_reader,
      cost_tier: "standard",
    },
    {
      role_id: "plot_continuity_reader",
      label: "Plot / World Rewriter",
      purpose: "Write one complete scene variant centered on causality, scene progression, active lore, world constraints, and physical continuity.",
      priority: 8,
      stage: "output",
      default_prompt: ROLE_PROMPTS.plot_continuity_reader,
      cost_tier: "standard",
    },
    {
      role_id: "style_reader",
      label: "Style / Dramatic Rewriter",
      purpose: "Write one complete scene variant centered on prose rhythm, imagery, subtext, dramatic pressure, transitions, and cadence.",
      priority: 6,
      stage: "output",
      default_prompt: ROLE_PROMPTS.style_reader,
      cost_tier: "standard",
    },
    {
      role_id: "perspective_boundary_rewriter",
      label: "Secret / Identity / Perspective Rewriter",
      purpose: "Write one complete scene variant centered on secrets, identity continuity, POV knowledge, user agency, and removal of model/meta artifacts.",
      priority: 10,
      stage: "output",
      default_prompt: ROLE_PROMPTS.perspective_boundary_rewriter,
      cost_tier: "standard",
    },
    {
      role_id: "semantic_judge",
      label: "Semantic Judge",
      purpose: "Compare complete scene variants against the Draft Ledger and runtime evidence, reject unsupported or violating elements, and produce semantic_judgment.v1.",
      priority: 20,
      stage: "judge",
      is_judge: true,
      default_prompt: ROLE_PROMPTS.semantic_judge,
      cost_tier: "premium",
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
    {
      role_id: "semantic_prover",
      label: "Final Semantic Prover",
      purpose: "Verify the final Composer output against facts, beats, constraints, secrets, POV, identity, agency, and output contract before return.",
      priority: 4,
      stage: "prover",
      is_prover: true,
      default_prompt: ROLE_PROMPTS.semantic_prover,
      cost_tier: "premium",
    },
  ];

  const COMPOSER_ROLE_ID = "whole_scene_composer";
  const JUDGE_ROLE_ID = "semantic_judge";
  const PROVER_ROLE_ID = "semantic_prover";

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
    character_reader: Object.freeze([
      "character_voice", "emotion",
    ]),
    plot_continuity_reader: Object.freeze([
      "plot_continuity", "scene_logic", "world_rule",
    ]),
    style_reader: Object.freeze([
      "repetition", "rhythm", "transition", "prose_clarity",
    ]),
    perspective_boundary_rewriter: Object.freeze([
      "secret_leak", "pov_violation", "identity_continuity", "agency_takeover",
      "meta_artifact", "output_contract_violation",
    ]),
  });
  const SPECIALIST_ROLE_IDS = Object.freeze([
    "character_reader",
    "plot_continuity_reader",
    "style_reader",
    "perspective_boundary_rewriter",
  ]);
  const ISSUE_OWNER_ROLE = Object.freeze({
    character_voice: "character_reader",
    emotion: "character_reader",
    plot_continuity: "plot_continuity_reader",
    scene_logic: "plot_continuity_reader",
    world_rule: "plot_continuity_reader",
    repetition: "style_reader",
    rhythm: "style_reader",
    transition: "style_reader",
    prose_clarity: "style_reader",
    secret_leak: "perspective_boundary_rewriter",
    pov_violation: "perspective_boundary_rewriter",
    identity_continuity: "perspective_boundary_rewriter",
    agency_takeover: "perspective_boundary_rewriter",
    meta_artifact: "perspective_boundary_rewriter",
    output_contract_violation: "perspective_boundary_rewriter",
  });

  function roleAllowedIssues(roleId) {
    return ROLE_ALLOWED_ISSUES[safeString(roleId)] || ISSUE_GROUPS;
  }

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

  /* ── Presets ───────────────────────────────────────────── */

  const PRESETS = Object.freeze([
    {
      id: "fast",
      label: "Fast (adaptive 2 + Judge + Composer + Prover)",
      roles: ["character_reader", "plot_continuity_reader", "style_reader", "perspective_boundary_rewriter", "semantic_judge", "whole_scene_composer", "semantic_prover"],
      deadline_ms: 60000,
    },
    {
      id: "balanced",
      label: "Balanced (adaptive 3 + 1 revision + Judge + Composer + Prover)",
      roles: ["character_reader", "plot_continuity_reader", "style_reader", "perspective_boundary_rewriter", "semantic_judge", "whole_scene_composer", "semantic_prover"],
      deadline_ms: 120000,
    },
    {
      id: "quality",
      label: "Quality (4 rewrites + up to 2 revisions + Judge + Composer + Prover)",
      roles: ["character_reader", "plot_continuity_reader", "style_reader", "perspective_boundary_rewriter", "semantic_judge", "whole_scene_composer", "semantic_prover"],
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
      .replace(/(["']?(?:authorization|x-api-key|api[_ -]?key|access[_ -]?token|token)["']?\s*[:=]\s*["']?)[^"',;\s}\]]+/gi, "$1[REDACTED]");
  }

  function redactTraceValue(value, depth) {
    const level = Math.max(0, Number(depth) || 0);
    if (level > 12) return "[REDACTED_DEPTH]";
    if (typeof value === "string") return redactSensitiveText(value);
    if (Array.isArray(value)) return value.map((item) => redactTraceValue(item, level + 1));
    if (!value || typeof value !== "object") return value;
    const output = {};
    Object.keys(value).forEach((key) => {
      output[key] = redactTraceValue(value[key], level + 1);
    });
    return output;
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

  function readArchiveCenterEnhancement(latestUserInput) {
    try {
      const envelope = asObject(globalThis[ARCHIVE_CENTER_BRIDGE_KEY]);
      const contract = asObject(envelope.enhancement_contract);
      const observation = asObject(envelope.payload_application_observation);
      const lifecycleState = "current_request_payload_applied";
      const sessionId = safeString(envelope.session_id).trim();
      const contractSessionId = safeString(contract.session_id).trim();
      const turnIndex = Math.trunc(Number(envelope.turn_index || 0));
      const contractTurnIndex = Math.trunc(Number(contract.turn_index || 0));
      const payloadPlanId = safeString(envelope.payload_plan_id).trim();
      const observedPayloadPlanId = safeString(observation.payload_plan_id).trim();
      if (envelope.contract_version !== ARCHIVE_CENTER_BRIDGE_CONTRACT
        || envelope.owner !== "archive_center_host_adapter"
        || envelope.transport_only !== true
        || envelope.lifecycle_state !== lifecycleState
        || contract.contract_version !== ARCHIVE_CENTER_ENHANCEMENT_CONTRACT
        || contract.owner !== "go"
        || contract.read_only !== true
        || contract.optional_enhancement !== true
        || contract.standalone_fallback_required !== true
        || observation.payload_application_status !== "applied"
        || observation.lifecycle_state !== lifecycleState
        || !sessionId
        || sessionId !== contractSessionId
        || turnIndex < 1
        || turnIndex !== contractTurnIndex
        || !payloadPlanId
        || payloadPlanId !== observedPayloadPlanId) {
        return null;
      }
      const input = safeString(latestUserInput);
      const expectedDigest = stableDigest(input);
      const bindings = arrayFromCollection(envelope.input_bindings);
      const expectedLifecycleDigest = stableDigest([
        sessionId,
        turnIndex,
        expectedDigest,
        input.length,
        payloadPlanId,
        lifecycleState,
      ].join("|"));
      const bound = bindings.length
        ? bindings.some((binding) => (
            safeString(binding && binding.digest) === expectedDigest
            && Number(binding && binding.chars || 0) === input.length
            && safeString(binding && binding.lifecycle_digest) === expectedLifecycleDigest
          ))
        : (
            safeString(envelope.input_digest) === expectedDigest
            && Number(envelope.input_chars || 0) === input.length
            && safeString(envelope.lifecycle_digest) === expectedLifecycleDigest
          );
      if (!bound) return null;
      return deepClone(envelope);
    } catch (_) {
      return null;
    }
  }

  function archiveCenterEnhancementTrace(envelope, lanes) {
    const contract = asObject(envelope && envelope.enhancement_contract);
    const features = asObject(contract.feature_status);
    const featureSummary = {};
    Object.keys(features).forEach((key) => {
      const feature = asObject(features[key]);
      featureSummary[key] = {
        status: safeString(feature.status, "empty"),
        selected_count: clampNumber(feature.selected_count, 0, 100000, 0),
        source_mode: safeString(feature.source_mode),
        call_status: safeString(feature.call_status),
      };
    });
    return {
      detected: true,
      mode: "archive_center_enhanced",
      contract_version: safeString(contract.contract_version),
      status: safeString(contract.status, "empty"),
      session_id: safeString(envelope && envelope.session_id),
      turn_index: clampNumber(envelope && envelope.turn_index, 0, 1000000000, 0),
      lane_count: arrayFromCollection(lanes).length,
      evidence_chars: arrayFromCollection(lanes).reduce(
        (total, lane) => total + safeString(lane && lane.text).length,
        0
      ),
      features: featureSummary,
      same_turn_critic_result_available: contract.same_turn_critic_result_available === true,
      transport: "transient_current_turn_only",
    };
  }

  function applyArchiveCenterEnhancement(ctx, envelope, settings, trace) {
    if (!ctx || !envelope) return ctx;
    const contract = asObject(envelope.enhancement_contract);
    const memoryPlan = asObject(envelope.memory_delivery_plan);
    const semantics = asObject(contract.lane_semantics);
    const lanes = [];
    arrayFromCollection(memoryPlan.classes).forEach((rawClass) => {
      const lane = asObject(rawClass);
      const key = safeString(lane.key).trim();
      const text = redactSensitiveText(filterExcludedContext(lane.text));
      const selectedCount = clampNumber(lane.selected_count, 0, 100000, 0);
      if (!key || !text.trim() || selectedCount < 1 || !safeString(semantics[key])) return;
      lanes.push({
        key,
        evidence_ref: `archive_center_${key}`,
        semantic_role: safeString(semantics[key]),
        text,
        selected_count: selectedCount,
        used_chars: clampNumber(lane.used_chars, 0, 1000000, text.length),
      });
    });
    const guidance = asObject(envelope.guidance_application_trace);
    const guidanceText = redactSensitiveText(filterExcludedContext(guidance.final_text));
    if (guidanceText.trim() && clampNumber(guidance.applied_count, 0, 100000, 0) > 0) {
      lanes.push({
        key: "output_guidance",
        evidence_ref: "archive_center_supervisor_guidance",
        semantic_role: safeString(semantics.output_guidance, "supervisor_current_turn"),
        text: guidanceText,
        selected_count: clampNumber(guidance.applied_count, 0, 100000, 0),
        used_chars: guidanceText.length,
      });
    }
    if (!lanes.length && safeString(contract.status) === "empty") {
      if (trace) trace.archive_center = archiveCenterEnhancementTrace(envelope, lanes);
      return ctx;
    }
    ctx.archive_center_context = {
      contract_version: safeString(contract.contract_version),
      status: safeString(contract.status, "ready"),
      owner: "go",
      read_only: true,
      session_id: safeString(envelope.session_id),
      turn_index: clampNumber(envelope.turn_index, 0, 1000000000, 0),
      privacy: deepClone(asObject(contract.privacy)),
      feature_status: deepClone(asObject(contract.feature_status)),
      lanes,
      memory_lineage: deepClone(asObject(envelope.memory_delivery_lineage)),
      payload_application_observation: deepClone(asObject(envelope.payload_application_observation)),
    };
    ctx.sources.archive_center = {
      available: lanes.length > 0,
      source: ARCHIVE_CENTER_ENHANCEMENT_CONTRACT,
      count: lanes.reduce((total, lane) => total + lane.selected_count, 0),
      active_count: lanes.length,
    };
    const limit = settings
      ? clampNumber(settings.context_char_limit, 500, 50000, 6000)
      : 6000;
    ctx.bounded_context_block = buildBoundedContextBlock(ctx, limit);
    ctx.manifest = buildContextManifest(ctx, settings);
    if (trace) {
      trace.archive_center = archiveCenterEnhancementTrace(envelope, lanes);
      if (trace.input_enhance) {
        trace.input_enhance.manifest_id = ctx.manifest.snapshot_id;
        trace.input_enhance.source_availability = ctx.manifest.source_availability;
      }
    }
    return ctx;
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
    const isProofRole = !!(role && (role.is_judge || role.is_prover));
    return {
      role_id: roleId,
      enabled: true,
      provider: "openai_compatible",
      endpoint: "",
      api_key_ref: "",
      model: "",
      temperature: role && role.is_composer ? 0.4 : (isProofRole ? 0 : (isInputPlanner ? 0.1 : 0.3)),
      max_output_tokens: role && (role.is_composer || isProofRole) ? 4096 : (isInputPlanner ? 1800 : 2048),
      timeout_ms: role && (role.is_composer || isProofRole) ? 60000 : 45000,
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
        is_judge: !!r.is_judge,
        is_prover: !!r.is_prover,
        is_composer: !!r.is_composer,
        default_prompt: r.default_prompt,
      })),
      role_profiles: roleProfiles,
      context_char_limit: 6000,
      protected_regex: "",
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
    merged.context_char_limit = clampNumber(input.context_char_limit, 500, 50000, 6000);
    merged.protected_regex = safeString(input.protected_regex);
    merged.trace_enabled = input.trace_enabled !== false;
    if (input.role_profiles && typeof input.role_profiles === "object") {
      const sourceProfiles = Object.assign({}, input.role_profiles);
      const legacyJudgeProfile = sourceProfiles.secret_pov_guard || sourceProfiles.agency_meta_guard;
      if (!sourceProfiles[JUDGE_ROLE_ID] && legacyJudgeProfile) {
        sourceProfiles[JUDGE_ROLE_ID] = Object.assign({}, legacyJudgeProfile, {
          role_id: JUDGE_ROLE_ID,
          system_prompt: ROLE_PROMPTS.semantic_judge,
          prompt_contract_version: ROLE_PROMPT_VERSION,
        });
      }
      const proverSourceProfile = sourceProfiles[JUDGE_ROLE_ID] || legacyJudgeProfile;
      if (!sourceProfiles[PROVER_ROLE_ID] && proverSourceProfile) {
        sourceProfiles[PROVER_ROLE_ID] = Object.assign({}, proverSourceProfile, {
          role_id: PROVER_ROLE_ID,
          temperature: 0,
          system_prompt: ROLE_PROMPTS.semantic_prover,
          prompt_contract_version: ROLE_PROMPT_VERSION,
        });
      }
      if (!sourceProfiles.plot_continuity_reader && sourceProfiles.world_reader) {
        sourceProfiles.plot_continuity_reader = Object.assign({}, sourceProfiles.world_reader, {
          role_id: "plot_continuity_reader",
          system_prompt: ROLE_PROMPTS.plot_continuity_reader,
          prompt_contract_version: ROLE_PROMPT_VERSION,
        });
      }
      if (!sourceProfiles.perspective_boundary_rewriter && legacyJudgeProfile) {
        sourceProfiles.perspective_boundary_rewriter = Object.assign({}, legacyJudgeProfile, {
          role_id: "perspective_boundary_rewriter",
          system_prompt: ROLE_PROMPTS.perspective_boundary_rewriter,
          prompt_contract_version: ROLE_PROMPT_VERSION,
        });
      }
      Object.keys(merged.role_profiles).forEach((roleId) => {
        const src = sourceProfiles[roleId];
        if (!src || typeof src !== "object") return;
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
      visible_output: {
        raw_chars: 0,
        visible_chars: 0,
        removed_block_count: 0,
        removed_chars: 0,
        ambiguous_unclosed: false,
      },
      roles: [],
      segments: { protected: 0, inspect_only: 0, mutable: 0 },
      candidates: {
        total: 0,
        segment_variant_total: 0,
        duplicate_scene_candidates: 0,
        incomplete_rejected_count: 0,
        normalized_field_count: 0,
        structured_recovery_queued: 0,
        structured_recovery_attempted: 0,
        structured_recovery_succeeded: 0,
        by_segment: {},
      },
      composer: {
        used: false,
        status: "",
        elapsed_ms: 0,
        semantic_retry: 0,
        preproof_recovery_attempted: false,
        preproof_recovery_succeeded: false,
        preproof_recovery_reason: "",
        proof_repair_attempted: false,
        direct_fallback: false,
        fallback_reason: "",
      },
      semantic_judge: {
        status: "not_run",
        accepted_candidates: 0,
        rejected_candidates: 0,
        constrained_candidates: 0,
        missing_facts: 0,
        unsupported_additions: 0,
        hard_violations: 0,
        required_contributions: 0,
        unresolved_requirements: 0,
      },
      semantic_prover: {
        status: "not_run",
        verdict: "not_run",
        attempts: 0,
        repair_attempted: false,
        repair_synthesized: false,
        facts_missing: 0,
        facts_contradicted: 0,
        beats_missing: 0,
        constraints_violated: 0,
        hard_violations: 0,
        unsupported_additions: 0,
        output_contract_failures: 0,
        quality_gains_required: 0,
        quality_gains_missing: 0,
        quality_gains_regressed: 0,
        residual_quality_checked: 0,
        residual_quality_issues: 0,
        realized_contributions: [],
        structured_recovery_attempted: false,
        structured_recovery_succeeded: false,
        quality_debt: [],
        validation_diagnostics: [],
        reason: "",
      },
      fusion_plan: {
        status: "not_run",
        direct_composer_fallback: false,
        fallback_reason: "",
        accepted_elements: 0,
        rejected_elements: 0,
        consensus_claims: 0,
        complementary_claims: 0,
        conflicts: 0,
        required_contributions: 0,
        unresolved_requirements: 0,
      },
      revision_convergence: {
        limit: 0,
        attempted: 0,
        fulfilled: 0,
        failed: 0,
        no_candidate: 0,
        remaining_requirements: 0,
        tail_reserve_ms: 0,
        last_valid_candidate_id: "",
        last_valid_draft_digest: "",
        rounds: [],
      },
      output_reuse: {
        hit: false,
        key: "",
        source_trace_timestamp: 0,
        age_ms: 0,
      },
      lineage: {
        draft_zero_digest: "",
        candidate_ids: [],
        judgment_digests: [],
        plan_ids: [],
        composer_output_digest: "",
        proof_digest: "",
        returned_output_digest: "",
      },
      router: { signals: [], selected: [], skipped: [] },
      scheduler: {
        completion_wait: false,
        endpoint_groups: [],
        stage_budgets: [],
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
      archive_center: {
        detected: false,
        mode: "standalone",
        contract_version: "",
        status: "not_detected",
        session_id: "",
        turn_index: 0,
        lane_count: 0,
        evidence_chars: 0,
        features: {},
        same_turn_critic_result_available: false,
        transport: "",
      },
      director_evidence: [],
      applied_evidence: [],
      attempted_evidence: [],
      budget: {
        http_attempt_max: OUTPUT_HTTP_ATTEMPT_BUDGET.balanced,
        http_attempt_used: 0,
        http_stopped_reason: "",
        input_attempt_max: 0,
        input_attempt_used: 0,
        composer_attempt_reserved: 0,
        composer_attempt_used: 0,
        judge_attempt_reserved: 0,
        judge_attempt_used: 0,
        prover_attempt_reserved: 0,
        prover_attempt_used: 0,
        specialist_primary_remaining: 0,
      },
      draft_ledger: {
        schema: "draft_ledger.v1",
        digest: "",
        source_contract_id: "",
        established_facts: 0,
        scene_beats: 0,
        unresolved_hooks: 0,
        response_directives: 0,
        hard_constraints: 0,
        protected_structures: 0,
        unknown_semantics: [],
      },
      summary: {
        specialist_calls: 0,
        specialist_http_calls: 0,
        successful_roles: 0,
        candidate_count: 0,
        composer_state: "",
        changed_segment_count: 0,
        material_changed_segment_count: 0,
        unchanged_segment_count: 0,
        material_rewrite: false,
        semantic_verified: "not_run",
        quality_preferred: "not_run",
        final_state: "",
        final_reason: "",
      },
      final: {
        enhanced: false,
        material_rewrite: false,
        semantic_verified: "not_run",
        quality_preferred: "not_run",
        reason: "",
      },
      errors: [],
      timeline: [],
    };
  }

  function traceRole(trace, entry) {
    if (!trace.roles) trace.roles = [];
    trace.roles.push(redactTraceValue({
      role_id: entry.role_id,
      stage: entry.stage || "output",
      provider: entry.provider,
      endpoint_group: entry.endpoint_group || "",
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
      validation_diagnostics: Array.isArray(entry.validation_diagnostics)
        ? entry.validation_diagnostics
        : [],
    }));
  }

  function traceError(trace, msg) {
    if (!trace.errors) trace.errors = [];
    trace.errors.push(redactSensitiveText(msg));
  }

  function traceTimeline(trace, label) {
    if (!trace.timeline) trace.timeline = [];
    trace.timeline.push({ label: safeString(label), t: Date.now() });
  }

  async function saveTrace(trace) {
    try {
      const raw = await storageGet(TRACE_KEY, "[]");
      const arr = JSON.parse(raw);
      arr.unshift(redactTraceValue(trace));
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
    { kind: "risu_marker", regex: /<\/?(?:risu|module|status|chatindex|regex|system|plugin|asset|background|emotion|prompt)[^>]*>/gi, priority: 92 },
    { kind: "html_tag", regex: /<\/?(?!(?:thoughts?|analysis|thinking|think)\b)[a-z][^>]*>/gi, priority: 80 },
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
    if (!raw.trim()) {
      return { leadingWs: raw, core: "", trailingWs: "" };
    }
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
    const extracted = extractVisibleAssistantOutput(value);
    return extracted.removed_block_count > 0 && !extracted.text.trim();
  }

  function normalizeEscapedReasoningTags(text) {
    return safeString(text).replace(
      /&lt;\s*(\/?)\s*(thoughts?|analysis|thinking|think)\b((?:(?!&gt;)[\s\S])*?)&gt;/gi,
      (_match, slash, name, suffix) => `<${slash || ""}${name}${suffix || ""}>`
    );
  }

  function isMetaOnlyHeadingTitle(text) {
    const title = safeString(text)
      .replace(/[*_`~]+/g, "")
      .replace(/[:：]\s*$/, "")
      .trim()
      .toLowerCase();
    return /^(?:thinking(?:\s+process)?|analysis|reasoning|chain\s+of\s+thought|approved)\b/.test(title);
  }

  function visibleResponseBoundary(text) {
    const source = safeString(text);
    const patterns = [
      /<(?:final|response|answer)\b[^>]*>/i,
      /(?:^|\n)\s*#{1,6}\s*(?:final(?:\s+answer)?|response|answer|최종(?:\s*답변)?|응답)\s*:?\s*(?:\n|$)/im,
    ];
    let selected = null;
    patterns.forEach((pattern) => {
      const match = pattern.exec(source);
      if (!match) return;
      const candidate = {
        start: match.index,
        content_start: match.index + match[0].length,
        marker: match[0],
      };
      if (!selected || candidate.start < selected.start) selected = candidate;
    });
    const headingPattern = /(?:^|\n)([ \t]*#{1,6}[ \t]+([^\n]+)(?:\n|$))/g;
    let headingMatch;
    while ((headingMatch = headingPattern.exec(source))) {
      if (isMetaOnlyHeadingTitle(headingMatch[2])) continue;
      const newlinePrefix = headingMatch[0][0] === "\n" ? 1 : 0;
      const headingStart = headingMatch.index + newlinePrefix;
      const candidate = {
        start: headingStart,
        content_start: headingStart,
        marker: headingMatch[1],
      };
      if (!selected || candidate.start < selected.start) selected = candidate;
      break;
    }
    return selected;
  }

  function extractVisibleAssistantOutput(text) {
    const source = safeString(text);
    let visible = normalizeEscapedReasoningTags(source);
    let removedBlocks = 0;
    let ambiguousUnclosed = false;
    const closedPattern = /<(thoughts?|analysis|thinking|think)\b[^>]*>[\s\S]*?<\/\1\s*>/gi;
    visible = visible.replace(closedPattern, () => {
      removedBlocks++;
      return "";
    });

    const leadingOpen = /^\s*<(thoughts?|analysis|thinking|think)\b[^>]*>/i.exec(visible);
    if (leadingOpen) {
      const remainderStart = leadingOpen.index + leadingOpen[0].length;
      const boundary = visibleResponseBoundary(visible.slice(remainderStart));
      if (boundary) {
        const visibleStart = remainderStart + boundary.content_start;
        removedBlocks++;
        visible = visible.slice(visibleStart);
      } else {
        ambiguousUnclosed = true;
        removedBlocks++;
        visible = "";
      }
    }

    const markdownHeader = /^\s*(?:\*{1,2}|_{1,2})?\s*(?:thinking(?:\s+process)?|analysis|reasoning|chain\s+of\s+thought)\s*[:：]?\s*(?:\*{1,2}|_{1,2})?\s*(?:\n|$)/i.exec(visible);
    if (markdownHeader) {
      const remainderStart = markdownHeader.index + markdownHeader[0].length;
      const boundary = visibleResponseBoundary(visible.slice(remainderStart));
      if (boundary) {
        visible = visible.slice(remainderStart + boundary.content_start);
        removedBlocks++;
      } else {
        ambiguousUnclosed = true;
        removedBlocks++;
        visible = "";
      }
    }

    if (removedBlocks > 0) {
      visible = visible.replace(/^\s*\n+/, "").replace(/\n{3,}/g, "\n\n");
    } else {
      visible = source;
    }
    return {
      text: visible,
      removed_block_count: removedBlocks,
      removed_chars: Math.max(0, source.length - visible.length),
      ambiguous_unclosed: ambiguousUnclosed,
      changed: visible !== source,
    };
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

  const SCENE_REWRITE_SEGMENT_ID = "scene_1";
  const SCENE_FRAME_TOKEN_RE = /\[\[ACR_EXACT_[a-z0-9]+_\d{4}\]\]/gi;

  function sceneFrameToken(digest, index) {
    return `[[ACR_EXACT_${safeString(digest).slice(0, 10)}_${String(index + 1).padStart(4, "0")}]]`;
  }

  function extractSceneFrameTokens(text) {
    return safeString(text).match(SCENE_FRAME_TOKEN_RE) || [];
  }

  function validateSceneFrameText(segment, text) {
    const required = arrayFromCollection(segment && segment.required_preservation_tokens)
      .map((token) => safeString(token));
    if (!required.length) return { pass: true, exact: true, repair_needed: false, code: "", expected: [], actual: [] };
    const actual = extractSceneFrameTokens(text);
    const exact = actual.length === required.length
      && actual.every((token, index) => token === required[index]);
    return {
      pass: true,
      exact,
      repair_needed: !exact,
      code: "",
      expected: required,
      actual,
    };
  }

  function rebuildSceneFrameTokens(frame, text) {
    const source = safeString(text);
    const tokens = arrayFromCollection(frame && frame.tokens).map((token) => safeString(token));
    if (!tokens.length) return source;
    const logicalSegment = frame && frame.logical_segments && frame.logical_segments[0];
    const template = mutableFullText(logicalSegment);
    const cleanSource = source.replace(SCENE_FRAME_TOKEN_RE, "");
    const cleanTemplate = template.replace(SCENE_FRAME_TOKEN_RE, "");
    const ratios = [];
    let cursor = 0;
    let visibleChars = 0;
    tokens.forEach((token) => {
      const index = template.indexOf(token, cursor);
      if (index < 0) {
        ratios.push(1);
        return;
      }
      visibleChars += template.slice(cursor, index).replace(SCENE_FRAME_TOKEN_RE, "").length;
      ratios.push(cleanTemplate.length ? visibleChars / cleanTemplate.length : 1);
      cursor = index + token.length;
    });
    let rebuilt = cleanSource;
    for (let index = tokens.length - 1; index >= 0; index--) {
      const position = Math.max(0, Math.min(
        rebuilt.length,
        Math.round(cleanSource.length * Math.max(0, Math.min(1, ratios[index])))
      ));
      rebuilt = rebuilt.slice(0, position) + tokens[index] + rebuilt.slice(position);
    }
    return rebuilt;
  }

  function buildSceneRewriteFrame(physicalSegments, originalText) {
    const source = safeString(originalText);
    let digest = stableDigest(source);
    let digestSalt = 0;
    while (source.indexOf(`[[ACR_EXACT_${digest}_`) >= 0) {
      digest = stableDigest(`${source}:${++digestSalt}`);
    }
    const tokens = [];
    const preservationManifest = [];
    const slotSegmentIds = [null];
    const maskedParts = [];
    let pendingFixedSegments = [];

    function flushFixedSegments() {
      if (!pendingFixedSegments.length) return;
      const token = sceneFrameToken(digest, tokens.length);
      const fixedText = pendingFixedSegments.map((segment) => mutableFullText(segment)).join("");
      const segmentIds = pendingFixedSegments.map((segment) => safeString(segment.id));
      const fixedTypes = uniqueList(pendingFixedSegments.map((segment) => safeString(segment.type)));
      const fixedKinds = uniqueList(pendingFixedSegments.map((segment) => safeString(segment.kind)).filter(Boolean));
      tokens.push(token);
      preservationManifest.push({
        token,
        segment_id: segmentIds[0] || "",
        segment_ids: segmentIds,
        type: fixedTypes.length === 1 ? fixedTypes[0] : "exact_group",
        kind: fixedKinds.length === 1 ? fixedKinds[0] : "structural_group",
        chars: fixedText.length,
        preview: preview(fixedText, 160),
      });
      maskedParts.push(token);
      slotSegmentIds.push(null);
      pendingFixedSegments = [];
    }

    arrayFromCollection(physicalSegments).forEach((segment) => {
      const fixedWhitespace = segment.type === "mutable"
        && !mutableCoreText(segment).trim();
      if (segment.type === "mutable" && !fixedWhitespace) {
        flushFixedSegments();
        const slotIndex = slotSegmentIds.length - 1;
        if (slotSegmentIds[slotIndex]) {
          throw new Error(`scene_frame_multiple_mutable_slot:${slotIndex}`);
        }
        slotSegmentIds[slotIndex] = segment.id;
        maskedParts.push(mutableFullText(segment));
        return;
      }
      pendingFixedSegments.push(Object.assign({}, segment, {
        type: fixedWhitespace ? "mutable_exact" : segment.type,
        kind: fixedWhitespace ? "structural_whitespace" : segment.kind,
      }));
    });
    flushFixedSegments();

    const maskedText = maskedParts.join("");
    const ws = splitMutableWhitespace(maskedText);
    const logicalSegment = {
      id: SCENE_REWRITE_SEGMENT_ID,
      type: "mutable",
      start: 0,
      end: maskedText.length,
      text: maskedText,
      leading_ws: ws.leadingWs,
      core_text: ws.core,
      trailing_ws: ws.trailingWs,
      required_preservation_tokens: tokens.slice(),
      preservation_manifest: preservationManifest,
      scene_slot_segment_ids: slotSegmentIds.slice(),
    };
    return {
      schema: "scene_rewrite_frame.v1",
      digest,
      original_text: source,
      physical_segments: arrayFromCollection(physicalSegments),
      logical_segments: [logicalSegment],
      semantic_segments: [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        start: 0,
        end: source.length,
        text: source,
        leading_ws: "",
        core_text: source,
        trailing_ws: "",
      }],
      tokens,
      preservation_manifest: preservationManifest,
      slot_segment_ids: slotSegmentIds,
    };
  }

  function splitSceneFrameText(frame, text) {
    let source = safeString(text);
    const logicalSegment = frame && frame.logical_segments && frame.logical_segments[0];
    const tokenCheck = validateSceneFrameText(logicalSegment, source);
    if (tokenCheck.repair_needed) {
      source = rebuildSceneFrameTokens(frame, source);
    }
    const chunks = [];
    let cursor = 0;
    arrayFromCollection(frame && frame.tokens).forEach((token) => {
      const index = source.indexOf(token, cursor);
      if (index < 0) return;
      chunks.push(source.slice(cursor, index));
      cursor = index + token.length;
    });
    chunks.push(source.slice(cursor));
    if (chunks.length !== arrayFromCollection(frame && frame.slot_segment_ids).length) {
      return { pass: false, code: "scene_frame_slot_count_mismatch", chunks: [], token_check: tokenCheck };
    }
    const insertedSlots = chunks.reduce((indexes, chunk, index) => {
      if (!frame.slot_segment_ids[index] && safeString(chunk).trim().length > 0) {
        indexes.push(index);
      }
      return indexes;
    }, []);
    return {
      pass: true,
      code: "",
      chunks,
      token_check: tokenCheck,
      inserted_slots: insertedSlots,
      token_repaired: tokenCheck.repair_needed === true,
    };
  }

  function foldPhysicalSegmentMapIntoScene(logicalSegment, rawSegments) {
    const slotSegmentIds = arrayFromCollection(logicalSegment && logicalSegment.scene_slot_segment_ids);
    const physicalIds = uniqueList(slotSegmentIds.filter(Boolean).map((id) => safeString(id)));
    const providedIds = physicalIds.filter((id) =>
      Object.prototype.hasOwnProperty.call(asObject(rawSegments), id)
    );
    if (!providedIds.length) return null;
    const frame = {
      logical_segments: [logicalSegment],
      tokens: arrayFromCollection(logicalSegment && logicalSegment.required_preservation_tokens),
      slot_segment_ids: slotSegmentIds,
    };
    const split = splitSceneFrameText(frame, mutableFullText(logicalSegment));
    if (!split.pass) return null;
    const chunks = split.chunks.slice();
    const knownTokens = new Set(frame.tokens);
    let strippedEmbeddedTokenCount = 0;
    slotSegmentIds.forEach((segmentId, slotIndex) => {
      if (!segmentId || !Object.prototype.hasOwnProperty.call(rawSegments, segmentId)) return;
      let value = safeString(rawSegments[segmentId]);
      const embeddedTokens = value.match(/\[\[ACR_EXACT_[^\]]+\]\]/g) || [];
      if (embeddedTokens.some((token) => !knownTokens.has(token))) {
        chunks[slotIndex] = null;
        return;
      }
      embeddedTokens.forEach((token) => {
        value = value.split(token).join("");
        strippedEmbeddedTokenCount++;
      });
      chunks[slotIndex] = value;
    });
    if (chunks.some((chunk) => chunk === null)) return null;
    let sceneText = chunks[0] || "";
    arrayFromCollection(frame.tokens).forEach((token, index) => {
      sceneText += token + (chunks[index + 1] || "");
    });
    if (!validateSceneFrameText(logicalSegment, sceneText).pass) return null;
    return {
      scene_text: sceneText,
      provided_segment_ids: providedIds,
      missing_segment_ids: physicalIds.filter((id) => providedIds.indexOf(id) < 0),
      stripped_embedded_token_count: strippedEmbeddedTokenCount,
    };
  }

  function restoreSceneRewriteFrame(frame, rewrittenMaskedText) {
    const split = splitSceneFrameText(frame, rewrittenMaskedText);
    if (!split.pass) return { pass: false, code: split.code, output: "", final_segments: [] };
    const physicalById = {};
    arrayFromCollection(frame.physical_segments).forEach((segment) => {
      physicalById[safeString(segment.id)] = segment;
    });
    const finalSegments = [];
    const manifest = arrayFromCollection(frame.preservation_manifest);
    const slotSegmentIds = arrayFromCollection(frame.slot_segment_ids);
    split.chunks.forEach((chunk, slotIndex) => {
      const segmentId = safeString(slotSegmentIds[slotIndex]);
      if (segmentId) {
        const original = physicalById[segmentId];
        if (!original) return;
        const originalFull = mutableFullText(original);
        const finalFull = safeString(original.leading_ws)
          + replacementCoreText(chunk)
          + safeString(original.trailing_ws);
        const materiality = rewriteMateriality(originalFull, finalFull);
        finalSegments.push({
          id: original.id,
          type: "mutable",
          original_text: originalFull,
          final_text: finalFull,
          source: finalFull === originalFull ? "original" : "composer",
          operation: finalFull === originalFull ? "none" : "replace",
          applied_role_id: finalFull === originalFull ? "" : COMPOSER_ROLE_ID,
          composer_unchanged: finalFull === originalFull,
          material_change: finalFull !== originalFull,
          original_meta_only: materiality.original_meta_only,
          sequence_similarity: materiality.sequence_similarity,
          changed_span_ratio: materiality.changed_span_ratio,
        });
      } else if (safeString(chunk).length > 0) {
        const materiality = rewriteMateriality("", chunk);
        finalSegments.push({
          id: `mutable_scene_insert_${slotIndex + 1}`,
          type: "mutable",
          original_text: "",
          final_text: chunk,
          source: "composer",
          operation: "insert",
          applied_role_id: COMPOSER_ROLE_ID,
          composer_unchanged: false,
          material_change: safeString(chunk).length > 0,
          original_meta_only: materiality.original_meta_only,
          sequence_similarity: materiality.sequence_similarity,
          changed_span_ratio: materiality.changed_span_ratio,
        });
      }

      const fixedGroup = manifest[slotIndex];
      arrayFromCollection(fixedGroup && fixedGroup.segment_ids).forEach((fixedId) => {
        const fixed = physicalById[safeString(fixedId)];
        if (!fixed) return;
        finalSegments.push({
          id: fixed.id,
          type: fixed.type,
          kind: fixed.kind,
          original_text: mutableFullText(fixed),
          final_text: mutableFullText(fixed),
          source: "preserved",
        });
      });
    });
    const representedIds = new Set(finalSegments.map((segment) => safeString(segment.id)));
    const missingPhysical = arrayFromCollection(frame.physical_segments)
      .filter((segment) => !representedIds.has(safeString(segment.id)));
    if (missingPhysical.length) {
      return {
        pass: false,
        code: `scene_frame_physical_segment_missing:${safeString(missingPhysical[0].id)}`,
        output: "",
        final_segments: [],
      };
    }
    return {
      pass: true,
      code: "",
      output: finalSegments.map((segment) => segment.final_text).join(""),
      final_segments: finalSegments,
      inserted_slots: arrayFromCollection(split.inserted_slots),
      token_repaired: split.token_repaired === true,
    };
  }

  function assembleSceneRewriteFrame(frame, composerResult) {
    const logicalAssembly = assembleOutput(frame.logical_segments, composerResult);
    const restored = restoreSceneRewriteFrame(frame, logicalAssembly.output);
    if (!restored.pass) {
      return Object.assign({}, logicalAssembly, {
        output: frame.original_text,
        finalSegments: [{
          id: SCENE_REWRITE_SEGMENT_ID,
          type: "mutable",
          original_text: frame.original_text,
          final_text: frame.original_text,
          source: "original",
          operation: "none",
          material_change: false,
        }],
        physicalFinalSegments: [],
        logicalFinalSegments: logicalAssembly.finalSegments,
        changed: false,
        composerApplied: 0,
        materialComposerApplied: 0,
        frame_error: restored.code,
      });
    }
    const originalText = frame.original_text;
    const finalText = restored.output;
    const logicalEvidence = logicalAssembly.finalSegments.find((segment) =>
      segment.id === SCENE_REWRITE_SEGMENT_ID
    );
    const materiality = rewriteMateriality(
      logicalEvidence ? logicalEvidence.original_text : originalText,
      logicalEvidence ? logicalEvidence.final_text : finalText
    );
    const changed = finalText !== originalText;
    return {
      output: finalText,
      finalSegments: [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        original_text: originalText,
        final_text: finalText,
        source: changed ? "composer" : "original",
        operation: changed ? "replace" : "none",
        applied_role_id: changed ? COMPOSER_ROLE_ID : "",
        composer_unchanged: !changed,
        material_change: changed,
        original_meta_only: materiality.original_meta_only,
        sequence_similarity: materiality.sequence_similarity,
        changed_span_ratio: materiality.changed_span_ratio,
      }],
      physicalFinalSegments: restored.final_segments,
      logicalFinalSegments: logicalAssembly.finalSegments,
      changed,
      composerApplied: changed ? 1 : 0,
      materialComposerApplied: changed ? 1 : 0,
      materialChanged: changed ? 1 : 0,
      metaOnlyChanged: changed && materiality.original_meta_only ? 1 : 0,
      unchangedSegments: changed ? 0 : 1,
      frame_error: "",
    };
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
    try {
      const value = await RR[name].apply(RR, Array.isArray(args) ? args : []);
      return { ok: true, value, source: name, error: "" };
    } catch (err) {
      return { ok: false, value: null, source: name, error: err && err.message ? err.message : String(err) };
    }
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
    const archiveEvidence = {};
    arrayFromCollection(ctx.archive_center_context && ctx.archive_center_context.lanes).forEach((lane) => {
      const ref = safeString(lane && lane.evidence_ref);
      const text = safeString(lane && lane.text);
      if (ref && text) archiveEvidence[ref] = text;
    });
    const evidence = {
      ...archiveEvidence,
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
      mode: Object.keys(archiveEvidence).length ? "archive_center_enhanced" : "standalone",
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
      archive_center_context: ctx.archive_center_context
        ? deepClone(ctx.archive_center_context)
        : null,
    };
    manifestCore.snapshot_id = `ctx_${Date.now()}_${stableDigest(manifestCore)}`;
    return manifestCore;
  }

  function buildBoundedContextBlock(ctx, limit) {
    const parts = [];
    arrayFromCollection(ctx.archive_center_context && ctx.archive_center_context.lanes).forEach((lane) => {
      const ref = safeString(lane && lane.evidence_ref);
      const semanticRole = safeString(lane && lane.semantic_role);
      const text = safeString(lane && lane.text);
      if (ref && text) parts.push(`[${ref}] [${semanticRole}]\n${text}`);
    });
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
    const archiveEnvelope = readArchiveCenterEnhancement(ctx.latest_user_input);
    if (archiveEnvelope) {
      applyArchiveCenterEnhancement(ctx, archiveEnvelope, settings, trace);
    }
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
      if (family === "deepseek") {
        if (provider === "ollama_compatible" && !isOllamaCloudEndpoint(profile && profile.endpoint)) {
          body.think = false;
          return { family, applied: ["think"] };
        }
        body.reasoning_effort = "none";
        return { family, applied: ["reasoning_effort"] };
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

  async function readResponseText(response, timeoutMs, abortSignal) {
    if (!response) return "";
    const boundedTimeoutMs = Math.max(0, Number(timeoutMs) || 0);
    let timer = null;
    let onAbort = null;
    try {
      const bodyPromise = Promise.resolve().then(() => response.text());
      const races = [bodyPromise];
      if (boundedTimeoutMs > 0) {
        races.push(new Promise((_, reject) => {
          timer = setTimeout(() => reject(new Error("request_timeout")), boundedTimeoutMs);
        }));
      }
      if (abortSignal) {
        races.push(new Promise((_, reject) => {
          onAbort = () => reject(new Error("deadline_aborted"));
          if (abortSignal.aborted) onAbort();
          else abortSignal.addEventListener("abort", onAbort, { once: true });
        }));
      }
      return await Promise.race(races);
    } finally {
      if (timer) clearTimeout(timer);
      if (abortSignal && onAbort) {
        try { abortSignal.removeEventListener("abort", onAbort); } catch (_) {}
      }
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
      const raw = await readResponseText(response, timeoutMs, abortSignal);
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
    if (profile.force_json_response && !(requestOptions && requestOptions.planner_tool)) {
      body.format = "json";
    }
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
    const raw = await readResponseText(response, timeoutMs, abortSignal);
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
      mode: safeString(manifest && manifest.mode, "standalone"),
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
      archive_center_context: manifest && manifest.archive_center_context
        ? deepClone(manifest.archive_center_context)
        : null,
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

  function attachArchiveCenterContextToTurnContract(turnContract, manifest) {
    const archiveContext = manifest && manifest.archive_center_context;
    if (!archiveContext) return turnContract;
    const contract = deepClone(asObject(turnContract));
    contract.schema = safeString(contract.schema, "turn_contract.v1");
    contract.mode = "archive_center_enhanced";
    contract.archive_center_context = deepClone(archiveContext);
    contract.evidence_refs = uniqueList(
      arrayFromCollection(contract.evidence_refs)
        .concat(arrayFromCollection(manifest.evidence_refs))
        .map((ref) => safeString(ref))
        .filter(Boolean)
    );
    contract.source_availability = Object.assign(
      {},
      asObject(contract.source_availability),
      asObject(manifest.source_availability)
    );
    const digestSource = Object.assign({}, contract);
    delete digestSource.contract_id;
    delete digestSource.contract_digest;
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
      mode: safeString(manifest && manifest.mode, "standalone"),
      evidence_sources: evidenceSources,
      evidence_refs: Object.keys(evidenceSources),
      source_availability: asObject(manifest && manifest.source_availability),
      collection_warnings: arrayFromCollection(manifest && manifest.collection_warnings),
      archive_center_context: manifest && manifest.archive_center_context
        ? {
            contract_version: safeString(manifest.archive_center_context.contract_version),
            status: safeString(manifest.archive_center_context.status),
            feature_status: deepClone(asObject(manifest.archive_center_context.feature_status)),
            privacy: deepClone(asObject(manifest.archive_center_context.privacy)),
            lanes: arrayFromCollection(manifest.archive_center_context.lanes).map((lane) => ({
              key: safeString(lane && lane.key),
              evidence_ref: safeString(lane && lane.evidence_ref),
              semantic_role: safeString(lane && lane.semantic_role),
              selected_count: clampNumber(lane && lane.selected_count, 0, 100000, 0),
            })),
          }
        : null,
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
    if (contract.archive_center_context) {
      projection.archive_center_context = {
        contract_version: safeString(contract.archive_center_context.contract_version),
        status: safeString(contract.archive_center_context.status),
        feature_status: deepClone(asObject(contract.archive_center_context.feature_status)),
        privacy: deepClone(asObject(contract.archive_center_context.privacy)),
        lane_refs: arrayFromCollection(contract.archive_center_context.lanes).map((lane) => ({
          key: safeString(lane && lane.key),
          evidence_ref: safeString(lane && lane.evidence_ref),
          semantic_role: safeString(lane && lane.semantic_role),
          selected_count: clampNumber(lane && lane.selected_count, 0, 100000, 0),
        })),
      };
    }
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

  function draftLedgerItems(contract, field, kind) {
    return uniqueContractItems(asObject(contract)[field]).map((item) => {
      const normalized = {
        kind: safeString(kind),
        text: safeString(item.text),
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs).map((ref) => safeString(ref))),
        evidence_quote: safeString(item.evidence_quote),
        status: "explicit_contract",
      };
      normalized.ledger_id = `ledger_${stableDigest(normalized)}`;
      return normalized;
    });
  }

  function isPayloadUserDirectiveDuplicate(item, responseDirectives) {
    const refs = uniqueList(arrayFromCollection(item && item.evidence_refs)
      .map((ref) => safeString(ref))
      .filter(Boolean));
    if (!refs.length || refs.some((ref) => ref !== "payload_user_input")) return false;
    return arrayFromCollection(responseDirectives).some((directive) =>
      contractItemsConflict(item, directive)
    );
  }

  function uniqueDraftLedgerItems(items) {
    const seen = new Set();
    const result = [];
    arrayFromCollection(items).forEach((item) => {
      if (!item || typeof item !== "object") return;
      const key = safeString(item.ledger_id)
        || stableDigest({
          kind: item.kind,
          text: item.text,
          evidence_refs: item.evidence_refs,
        });
      if (seen.has(key)) return;
      seen.add(key);
      result.push(item);
    });
    return result;
  }

  function archiveCenterLedgerItems(contract, keys, kind) {
    const archive = asObject(asObject(contract).archive_center_context);
    const allowed = new Set(arrayFromCollection(keys).map((key) => safeString(key)));
    const result = [];
    arrayFromCollection(archive.lanes).forEach((lane) => {
      const key = safeString(lane && lane.key);
      const evidenceRef = safeString(lane && lane.evidence_ref);
      if (!allowed.has(key) || !evidenceRef) return;
      const lines = safeString(lane && lane.text)
        .split(/\r?\n/)
        .map((line) => line.trim())
        .filter((line) => line && !/^\[[^\]]+\]$/.test(line))
        .slice(0, 16);
      lines.forEach((line) => {
        const text = truncate(line.replace(/^[-*]\s*/, ""), 700).trim();
        if (!text) return;
        const item = {
          kind: safeString(kind),
          text,
          evidence_refs: [evidenceRef],
          evidence_quote: truncate(line, 240),
          status: "archive_center_go_selected",
          archive_lane: key,
          semantic_role: safeString(lane && lane.semantic_role),
        };
        item.ledger_id = `ledger_${stableDigest(item)}`;
        result.push(item);
      });
    });
    return uniqueDraftLedgerItems(result);
  }

  function buildDraftLedger(draftZero, segments, turnContract) {
    const contract = asObject(turnContract);
    const archiveObjectiveFacts = archiveCenterLedgerItems(
      contract,
      ["event_recent", "character_objective", "world_state", "direct_evidence"],
      "archive_grounded_fact"
    );
    const archiveRelationshipState = archiveCenterLedgerItems(
      contract,
      ["subjective_relationship"],
      "archive_subjective_relationship"
    );
    const archiveSecrets = archiveCenterLedgerItems(
      contract,
      ["protected_secret"],
      "archive_writer_only_secret"
    );
    const archiveOpenThreads = archiveCenterLedgerItems(
      contract,
      ["unresolved_goal"],
      "archive_open_thread"
    );
    const archiveSupervisorGuidance = archiveCenterLedgerItems(
      contract,
      ["output_guidance"],
      "archive_supervisor_directive"
    );
    const responseDirectives = uniqueDraftLedgerItems(
      draftLedgerItems(contract, "turn_objectives", "response_directive")
        .concat(draftLedgerItems(contract, "prose_targets", "prose_directive"))
        .concat(archiveSupervisorGuidance)
    );
    const immutableConstraints = uniqueDraftLedgerItems(
      draftLedgerItems(contract, "immutable_constraints", "immutable_constraint")
    );
    const establishedFacts = uniqueDraftLedgerItems(
      immutableConstraints
        .concat(draftLedgerItems(contract, "character_visible_facts", "character_visible_fact"))
        .concat(draftLedgerItems(contract, "scene_state", "scene_state"))
        .concat(archiveObjectiveFacts)
    ).filter((item) => !isPayloadUserDirectiveDuplicate(item, responseDirectives));
    const unresolvedHooks = uniqueDraftLedgerItems(
      draftLedgerItems(contract, "open_threads", "unresolved_hook")
        .concat(archiveOpenThreads)
    );
    const sceneBeats = uniqueDraftLedgerItems(unresolvedHooks);
    const relationshipState = uniqueDraftLedgerItems(
      draftLedgerItems(
        contract,
        "relationship_and_emotion_state",
        "relationship_state"
      ).concat(archiveRelationshipState)
    );
    const speakerAndPov = draftLedgerItems(
      contract,
      "agency_and_pov_constraints",
      "speaker_pov_constraint"
    );
    const secretsAndReveal = uniqueDraftLedgerItems(
      draftLedgerItems(contract, "writer_only_secrets", "writer_only_secret")
        .concat(draftLedgerItems(contract, "character_knowledge_scopes", "knowledge_scope"))
        .concat(draftLedgerItems(contract, "identity_and_alias_map", "identity_reveal_state"))
        .concat(archiveSecrets)
    );
    const userOwnedDecisions = speakerAndPov.concat(responseDirectives);
    const hardConstraints = uniqueDraftLedgerItems(
      immutableConstraints
        .concat(secretsAndReveal)
        .concat(speakerAndPov)
        .concat(responseDirectives)
        .concat(draftLedgerItems(contract, "forbidden_regressions", "forbidden_regression"))
    );
    const protectedStructures = (segments || [])
      .filter((segment) => segment.type !== "mutable")
      .map((segment) => ({
        segment_id: safeString(segment.id),
        type: safeString(segment.type),
        kind: safeString(segment.kind),
        source_ref: `draft_zero:${safeString(segment.id)}`,
        digest: stableDigest(safeString(segment.text)),
        chars: safeString(segment.text).length,
        status: "exact_preservation",
      }));
    const mutableSources = (segments || [])
      .filter((segment) => segment.type === "mutable" && mutableCoreText(segment).trim())
      .map((segment) => ({
        segment_id: safeString(segment.id),
        source_ref: `draft_zero:${safeString(segment.id)}`,
        digest: stableDigest(mutableFullText(segment)),
        chars: mutableFullText(segment).length,
        paragraph_count: mutableCoreText(segment).split(/\n\s*\n/).filter((part) => part.trim()).length,
        status: "explicit_draft_source",
      }));
    const unknownSemantics = ["character_intentions"];
    if (!speakerAndPov.length) unknownSemantics.push("speaker_and_pov");
    if (!secretsAndReveal.length) unknownSemantics.push("secrets_and_reveal");
    const ledger = {
      schema: "draft_ledger.v1",
      source_contract_id: safeString(contract.contract_id),
      source_contract_digest: safeString(contract.contract_digest),
      draft_digest: stableDigest(safeString(draftZero)),
      established_facts: establishedFacts,
      scene_beats: sceneBeats,
      unresolved_hooks: unresolvedHooks,
      response_directives: responseDirectives,
      character_intentions: [],
      relationship_state: relationshipState,
      speaker_and_pov: speakerAndPov,
      secrets_and_reveal: secretsAndReveal,
      user_owned_decisions: uniqueDraftLedgerItems(userOwnedDecisions),
      hard_constraints: hardConstraints,
      protected_structures: protectedStructures,
      mutable_sources: mutableSources,
      candidate_factual_additions: [],
      unknown_semantics: uniqueList(unknownSemantics),
      archive_center: contract.archive_center_context
        ? {
            status: safeString(contract.archive_center_context.status),
            lane_count: arrayFromCollection(contract.archive_center_context.lanes).length,
            objective_fact_count: archiveObjectiveFacts.length,
            subjective_memory_count: archiveRelationshipState.length,
            writer_only_secret_count: archiveSecrets.length,
            open_thread_count: archiveOpenThreads.length,
            supervisor_directive_count: archiveSupervisorGuidance.length,
          }
        : null,
    };
    ledger.ledger_digest = stableDigest(ledger);
    return ledger;
  }

  function summarizeDraftLedger(ledger) {
    const source = asObject(ledger);
    return {
      schema: safeString(source.schema) || "draft_ledger.v1",
      digest: safeString(source.ledger_digest),
      source_contract_id: safeString(source.source_contract_id),
      established_facts: arrayFromCollection(source.established_facts).length,
      scene_beats: arrayFromCollection(source.scene_beats).length,
      unresolved_hooks: arrayFromCollection(source.unresolved_hooks).length,
      response_directives: arrayFromCollection(source.response_directives).length,
      hard_constraints: arrayFromCollection(source.hard_constraints).length,
      protected_structures: arrayFromCollection(source.protected_structures).length,
      unknown_semantics: uniqueList(arrayFromCollection(source.unknown_semantics).map((item) => safeString(item))),
      archive_center: source.archive_center ? deepClone(source.archive_center) : null,
    };
  }

  /* ── Role Call (single path for all roles) ─────────────── */

  function buildRolePrompt(role, profile, mutableSegs, contextBlock, allSegments, directorInfo) {
    const systemPrompt = safeString(profile.system_prompt || role.default_prompt);
    const draftLedger = asObject(directorInfo && directorInfo.draft_ledger);
    const archiveContext = asObject(draftLedger.archive_center);
    const archivePolicy = archiveContext.status
      ? [
          "Archive Center enhancement is active for this turn.",
          "- Treat archive_grounded_fact as selected objective or verified evidence.",
          "- Treat archive_subjective_relationship as one perspective's memory, belief, or relationship state, never universal truth.",
          "- Treat archive_writer_only_secret as writer-only knowledge. Do not expose it through a character who lacks that knowledge.",
          "- Treat archive_supervisor_directive as a current-turn composition requirement.",
          "- Archive Center Critic evidence is accepted prior-turn evidence, not a same-turn Critic verdict.",
        ].join("\n")
      : "";
    const contextSection = contextBlock
      ? `\n\n--- Runtime Context (read-only) ---\n${archivePolicy ? archivePolicy + "\n\n" : ""}${contextBlock}\n--- End Context ---\n`
      : "";
    const draftLedgerSection = draftLedger.schema === "draft_ledger.v1"
      ? `\n\n--- Draft Ledger (binding source map) ---\n${JSON.stringify(draftLedger)}\n--- End Draft Ledger ---\n`
      : "";
    const preservationManifest = arrayFromCollection(mutableSegs).flatMap((segment) =>
      arrayFromCollection(segment && segment.preservation_manifest).map((item) => ({
        token: safeString(item && item.token),
        segment_id: safeString(item && item.segment_id),
        segment_ids: arrayFromCollection(item && item.segment_ids).map((id) => safeString(id)),
        type: safeString(item && item.type),
        kind: safeString(item && item.kind),
      }))
    );
    const preservationSection = preservationManifest.length
      ? `\n\n--- RisuAI Host Artifact Hints ---\n${JSON.stringify(preservationManifest)}\nEach marker indicates the approximate location of host-rendered bytes such as an image, status field, or code container. Freely rebuild all prose before, between, and after them. Keep markers when convenient; missing, duplicated, or displaced markers are repaired mechanically after composition and never invalidate the prose.\n--- End RisuAI Host Artifact Hints ---\n`
      : "";
    let userPrompt;
    if (role.is_input_planner) {
      const manifest = asObject(directorInfo && directorInfo.context_manifest);
      const plannerOptions = plannerRequestOptions(role, profile);
      const transportInstruction = plannerOptions && plannerOptions.planner_tool
        ? "Call submit_turn_contract_fragment exactly once with the final object. Do not also emit the object as message text."
        : "Output one compact JSON object only. Do not emit XML, <tool_call>, function tags, markdown, reasoning, or commentary.";
      userPrompt = `Build one grounded turn_contract_fragment.v1 from this context_manifest.v1.\n\nAllowed evidence_refs: ${arrayFromCollection(manifest.evidence_refs).join(", ")}\nPlanner-owned fields: ${inputPlannerFields(role.role_id).join(", ")}\n\nContext manifest:\n${JSON.stringify(manifest)}\n\nRules:\n- Every item outside uncertainty MUST include at least one allowed evidence_ref and an exact short evidence_quote copied from that same evidence_sources field.\n- The item text must describe the quoted evidence; never attach an unrelated real quote to an invented claim.\n- Keep writer_only_secrets separate from character_visible_facts when those fields are assigned to this role.\n- Do not convert narrator knowledge into character knowledge.\n- Do not write RP prose, dialogue, a draft, recommendations, or markdown.\n- Return only this role's assigned fields. Use empty arrays when no grounded item exists.\n- ${transportInstruction}\n\nExpected object:\n${inputPlannerExample(role.role_id)}`;
    } else if (role.is_judge) {
      const sceneCandidates = arrayFromCollection(directorInfo && directorInfo.scene_candidates).map((candidate) => ({
        candidate_id: candidate.candidate_id,
        role_id: candidate.role_id,
        supporting_roles: candidate.supporting_roles,
        segments: candidate.segments,
        evidence_refs: candidate.evidence_refs,
        retained_beats: candidate.retained_beats,
        proposed_additions: candidate.proposed_additions,
        addressed_issues: candidate.addressed_issues,
        change_summary: candidate.change_summary,
        coverage_complete: candidate.coverage_complete === true,
        covered_segment_ids: candidate.covered_segment_ids || [],
      }));
      const originalSegments = allSegments.map((segment) => ({
        id: segment.id,
        type: segment.type,
        text: segment.type === "mutable" ? segment.text : preview(segment.text, 240),
      }));
      userPrompt = `Judge every scene-wide candidate against the original response, Draft Ledger, runtime evidence, and output contract.\n\nOriginal ordered segments:\n${JSON.stringify(originalSegments)}\n\nScene candidates:\n${JSON.stringify(sceneCandidates)}\n${draftLedgerSection}${contextSection}\n\nRules:\n- Return exactly one judgment for every candidate_id and no unknown candidate_id.\n- preserved_ledger_ids and missing_ledger_ids may contain only Draft Ledger ledger_id values.\n- A candidate with a secret, POV, identity/reveal, user-agency, model/meta, or output-contract violation must be reject, unless the exact violating element is isolated in rejected_elements and hard_violations for Composer prohibition; only then use accept_with_constraints.\n- Treat unsupported factual, relationship, location, object-state, backstory, or event additions as unsupported_additions.\n- Record accepted and rejected elements as concrete claims with candidate_ids and evidence_refs.\n- Every non-rejected candidate must expose at least one concrete accepted_elements or quality_gains item tied to its candidate_id and affected segment_ids. Describe the contribution Composer must realize, not a generic compliment.\n- Cross-candidate consensus, complementary value, and conflicts must describe claims or facts, not shared issue labels.\n- unresolved_requirements contains only important concrete deficiencies that no accepted candidate fully solves. Assign each to exactly one owner role: character_reader for voice/emotion/relationships, plot_continuity_reader for plot/continuity/world, style_reader for style/rhythm/staging, perspective_boundary_rewriter for secret/identity/POV/agency/meta/output boundaries.\n- Every unresolved requirement needs an issue_type owned by role_id, affected mutable segment_ids, and evidence_refs when evidence exists. Use [] when no important gap remains.\n- Do not use model confidence as evidence and do not write replacement prose.\n\nReturn compact JSON only:\n{"schema":"semantic_judgment.v1","candidate_judgments":[{"candidate_id":"candidate_id","verdict":"accept|accept_with_constraints|reject","preserved_ledger_ids":[],"missing_ledger_ids":[],"unsupported_additions":[{"claim":"unsupported claim","reason":"why","evidence_refs":[],"candidate_ids":["candidate_id"],"segment_ids":["mutable_1"]}],"hard_violations":[{"type":"secret_leak|pov_violation|identity_continuity|agency_takeover|meta_artifact|output_contract_violation","detail":"violation","evidence_refs":[],"segment_ids":["mutable_1"]}],"accepted_elements":[{"claim":"specific contribution to preserve","candidate_ids":["candidate_id"],"segment_ids":["mutable_1"],"evidence_refs":[]}],"rejected_elements":[],"quality_gains":[{"claim":"specific quality gain Composer must realize","candidate_ids":["candidate_id"],"segment_ids":["mutable_1"],"evidence_refs":[]}],"quality_regressions":[]}],"cross_candidate":{"consensus":[],"complementary":[],"conflicts":[]},"unresolved_requirements":[{"issue_type":"character_voice|emotion|plot_continuity|scene_logic|world_rule|repetition|rhythm|transition|prose_clarity|secret_leak|pov_violation|identity_continuity|agency_takeover|meta_artifact|output_contract_violation","role_id":"specialist_role_id","claim":"specific unresolved deficiency","segment_ids":["mutable_1"],"evidence_refs":[]}],"scene_requirements":{"target_arc":"","target_voice":"","target_pacing":""}}`;
    } else if (role.is_prover) {
      const finalSegments = arrayFromCollection(directorInfo && directorInfo.final_segments).map((segment) => ({
        id: safeString(segment.id),
        type: safeString(segment.type),
        text: safeString(segment.final_text),
      }));
      const originalSegments = allSegments.map((segment) => ({
        id: segment.id,
        type: segment.type,
        text: segment.type === "mutable" ? mutableFullText(segment) : preview(segment.text, 240),
      }));
      userPrompt = `Prove or reject the FINAL Composer output against every binding semantic unit and required Fusion contribution.\n\nOriginal ordered segments:\n${JSON.stringify(originalSegments)}\n\nFinal Composer segments:\n${JSON.stringify(finalSegments)}\n\nSemantic Judgment:\n${JSON.stringify(asObject(directorInfo && directorInfo.semantic_judgment))}\n\nFusion Plan:\n${JSON.stringify(asObject(directorInfo && directorInfo.fusion_plan))}\n${draftLedgerSection}${contextSection}\n\nRules:\n- Cover every established_facts ledger_id exactly once in fact_checks.\n- Cover every scene_beats ledger_id exactly once in beat_checks.\n- Cover every hard_constraints ledger_id exactly once in constraint_checks.\n- response_directives are execution constraints, not story facts or scene beats. Judge whether the final scene obeys them through constraint_checks; the user's command wording does not need to appear in the prose.\n- A preserved fact or beat evidence_quote must be an exact quote from the FINAL Composer prose. Never cite the user command, Draft Ledger wording, runtime context, or the original draft when that wording is absent from the final prose.\n- Cover every fusion_plan.required_contributions contribution_id exactly once in quality_gain_checks.\n- A quality gain is realized only when its exact evidence_quote appears in the required final segment and that exact wording was not already present in the corresponding original segment. Missing, unchanged, or regressed gains cannot pass.\n- Return exactly one residual_quality_checks item for each check_id: mechanics_and_wording, register_and_era, repetition_and_exposition, grounded_psychology_and_relationship, scene_coherence_and_ending.\n- Mark a residual check issue when the final scene retains an awkward or erroneous phrase, typo, register/era mismatch, repeated or flattened exposition, unsupported motive/emotion/relationship/knowledge assertion, weak opening or transition, or generic/explanatory ending. An issue requires an exact final-text quote and segment_id and cannot pass.\n- Mark missing, contradicted, violated, uncertain, regressed, or issue honestly. Uncertain is not a pass.\n- Record every secret, POV, identity, agency, meta, or output-contract failure as a hard_violation.\n- Record every new factual, relationship, location, object-state, backstory, event, motive, emotion, knowledge state, or personality assertion without original, Draft Ledger, or runtime evidence as an unsupported_addition.\n- output_contract fields must be booleans grounded in the final text.\n- Use verdict repair only when one targeted Composer repair can resolve every failure. Provide segment-scoped repair_instructions covering missing/contradicted fact ledger IDs, beat ledger IDs, violated constraint ledger IDs, missing contribution IDs, residual check IDs, hard violations, unsupported additions, and failed output_contract keys.\n- Put the covered ledger/contribution/check/output-contract IDs in repair_instructions.evidence_refs or name them exactly in instruction. Every affected segment must receive an instruction.\n- Use verdict fail when the scene cannot be repaired without replacing its grounded event structure.\n- Do not write prose, advice, markdown, or reasoning.\n\nReturn compact JSON only:\n{"schema":"semantic_proof.v1","declared_verdict":"pass|repair|fail","fact_checks":[{"ledger_id":"fact_1","status":"preserved|missing|contradicted|uncertain","detail":"","evidence_quote":""}],"beat_checks":[{"ledger_id":"beat_1","status":"preserved|missing|contradicted|uncertain","detail":"","evidence_quote":""}],"constraint_checks":[{"ledger_id":"constraint_1","status":"satisfied|violated|uncertain","detail":"","evidence_quote":""}],"quality_gain_checks":[{"contribution_id":"gain_id","status":"realized|missing|regressed","detail":"","evidence_quote":"","segment_ids":["mutable_1"]}],"residual_quality_checks":[{"check_id":"mechanics_and_wording|register_and_era|repetition_and_exposition|grounded_psychology_and_relationship|scene_coherence_and_ending","status":"clean|issue","detail":"","evidence_quote":"","segment_ids":[]}],"hard_violations":[{"type":"secret_leak|pov_violation|identity_continuity|agency_takeover|meta_artifact|output_contract_violation","detail":"","segment_ids":["mutable_1"],"evidence_refs":[]}],"unsupported_additions":[{"claim":"","reason":"","segment_ids":["mutable_1"],"evidence_refs":[]}],"output_contract":{"language_ok":true,"turn_boundary_ok":true,"user_agency_ok":true,"meta_free":true,"format_ok":true},"repair_instructions":[{"segment_id":"mutable_1","instruction":"repair fact_1 and output_contract:language_ok","evidence_refs":["fact_1","output_contract:language_ok"],"prohibited":[]}]}`;
    } else if (role.is_composer) {
      const plainSceneOutput = !!(directorInfo && directorInfo.composer_plain_scene)
        && mutableSegs.length === 1
        && mutableSegs[0].id === SCENE_REWRITE_SEGMENT_ID;
      const mutableList = mutableSegs.map((s) => {
        const candidates = (directorInfo && directorInfo.candidateBundles && directorInfo.candidateBundles[s.id]) || [];
        const candidateSection = candidates.length
          ? candidates.map((c, i) => `  Candidate ${i + 1} (id: ${c.candidate_id || "unknown"}, role: ${c.role}, Judge: ${c.judge_verdict || "unknown"}, evidence: [${(c.evidence_refs || []).join(", ")}], retained: ${JSON.stringify(c.retained_beats || [])}, proposed additions: ${JSON.stringify(c.proposed_additions || [])}, change: ${c.change_summary || "none"}):\n    ${c.rewrite}`).join("\n")
          : "  (no candidates — gap, improve original directly)";
        return `Segment ${s.id}:\nOriginal:\n${s.text}\nAccepted candidates:\n${candidateSection}`;
      }).join("\n\n---\n\n");
      const preservedList = allSegments.filter((s) => s.type !== "mutable").map((s) =>
        `${s.id} [HOST ARTIFACT — represented by token]: ${preview(s.text, 200)}`
      ).join("\n");
      const proofRepair = asObject(directorInfo && directorInfo.semantic_proof_repair);
      const preProofRecovery = proofRepair.preproof_recovery === true;
      const proofRepairSection = arrayFromCollection(proofRepair.repair_instructions).length
        ? `\n--- ${preProofRecovery ? "Pre-Proof Composition Recovery" : "Semantic Proof Repair"} ---\nThe previous composed scene failed ${preProofRecovery ? "material rewrite or structural verification" : "final semantic proof"}.\nPrevious rejected final segments:\n${JSON.stringify(proofRepair.previous_segments || {})}\nRepair instructions:\n${JSON.stringify(proofRepair.repair_instructions)}\nFailure reasons:\n${JSON.stringify(proofRepair.reason_codes || [])}\nRecompose the complete mutable scene once. Resolve every listed failure while preserving all already-correct facts, beats, constraints, exact placeholders, voice, and causality. Do not return a local phrase patch or commentary.\n--- End ${preProofRecovery ? "Pre-Proof Composition Recovery" : "Semantic Proof Repair"} ---\n`
        : "";
      const lineageSection = directorInfo && (
        arrayFromCollection(directorInfo.revision_lineage).length
        || safeString(asObject(directorInfo.last_valid_candidate).candidate_id)
      )
        ? `\n--- Revision Lineage ---\nLast Judge-accepted complete draft:\n${JSON.stringify(asObject(directorInfo.last_valid_candidate))}\nAdaptive revision rounds:\n${JSON.stringify(arrayFromCollection(directorInfo.revision_lineage))}\nUse the last accepted draft as accumulated work. Preserve its accepted gains while resolving the final Fusion Plan. Draft zero remains the factual comparison source, not the prose template.\n--- End Revision Lineage ---\n`
        : "";
      const directorSection = directorInfo
        ? `\n--- Semantic Judgment ---\n${JSON.stringify(directorInfo.semantic_judgment || {})}\n--- Fusion Plan ---\n${JSON.stringify(directorInfo.fusion_plan || {})}\n--- End Semantic Plan ---\n${lineageSection}${proofRepairSection}`
        : "";
      const revisionFeedback = asObject(directorInfo && directorInfo.composer_revision_feedback);
      const retrySection = arrayFromCollection(revisionFeedback.weak_segment_ids).length
        ? `\n--- Recomposition Retry ---\nThe previous composition needs a stronger scene-level rewrite.\nWeak segment IDs: ${revisionFeedback.weak_segment_ids.join(", ")}\nPrevious segments:\n${JSON.stringify(revisionFeedback.previous_segments || {})}\nFor every weak segment, discard the previous sentence structure and reconstruct the prose materially. Preserve facts, not wording. Do not answer with spelling, punctuation, synonym, or isolated-line edits.\n--- End Retry ---\n`
        : "";
      const composerReturnContract = plainSceneOutput
        ? `Return only the complete final ${SCENE_REWRITE_SEGMENT_ID} scene body. Do not wrap it in JSON, markdown fences, labels, analysis, or commentary. Treat RisuAI host artifact markers as optional placement hints and freely rewrite or expand all prose around them.`
        : 'Return compact JSON only:\n{"segments":{"SEG_ID":"final rewritten text", ...}}';
      userPrompt = `Compose the FINAL mutable prose from the original, accepted scene candidates, Semantic Judgment, Fusion Plan, and Draft Ledger.\n\nMutable segments:\n${mutableList}\n\nRisuAI host artifacts already represented by tokens:\n${preservedList}\n${directorSection}${retrySection}${draftLedgerSection}${preservationSection}${contextSection}\n\nRules:\n- Follow fusion_plan.v1. Preserve every required fact, beat, constraint, user-owned decision, secret/reveal state, POV boundary, and cross-segment consequence.\n- Realize every fusion_plan.required_contributions item in its specified segment_ids. Integrate compatible contributions from every accepted specialist role rather than collapsing to one top candidate.\n- Use only accepted candidate elements and permitted additions. Exclude every rejected element, prohibited addition, unsupported claim, and hard violation.\n- Resolve conflicts exactly as the Fusion Plan states; do not choose by confidence, score, verbosity, or surface elegance.\n- Synthesize compatible strengths into new prose. Never concatenate candidate passages or copy one candidate unchanged.\n- The supplied mutable text is already the visible response; do not recreate removed model reasoning, prompt residue, approval labels, assistant commentary, or output-contract violations.\n- Materially reconstruct every substantive segment through stronger scene architecture, causality, voice, subtext, dramatic pressure, imagery, pacing, transitions, and cadence.\n- After drafting all segment values, read them in assembled order and perform one final whole-scene polish. Correct awkward or erroneous wording, typos, register or era mismatch, repeated explanation, unsupported psychological interpretation, weak boundary transitions, and generic or explanatory ending cadence.\n- Do not introduce a motive, emotion, relationship interpretation, knowledge state, or personality judgment unless grounded in the original, Draft Ledger, accepted evidence, or runtime context.\n- Do not cross the current turn boundary or decide the user's unexpressed thought, speech, consent, emotion, or next action.\n- Include every requested mutable segment ID exactly once. Output token markers rather than raw host artifact bytes.\n\n${composerReturnContract}`;
    } else {
      const segIds = mutableSegs.map((s) => s.id);
      const revisionRound = asObject(directorInfo && directorInfo.revision_round);
      const revisionSection = revisionRound.round
        ? `\n\n--- Adaptive Revision Round ${revisionRound.round} ---\nThe mutable segments above are the most recent Judge-accepted complete draft, not draft_zero.\nParent candidate: ${safeString(revisionRound.parent_candidate_id)}\nUnresolved requirements owned by this lane:\n${JSON.stringify(arrayFromCollection(revisionRound.requirements))}\nCurrent Semantic Judgment:\n${JSON.stringify(asObject(revisionRound.semantic_judgment))}\nCurrent Fusion Plan:\n${JSON.stringify(asObject(revisionRound.fusion_plan))}\nDevelop this complete draft further. Preserve its valid gains and resolve every listed requirement through your lane. Do not restart from weaker draft_zero wording.\n--- End Adaptive Revision Round ---\n`
        : "";
      userPrompt = `Produce one complete scene-wide rewrite candidate through this lane.\n\nYou are seeing the full ordered visible response for continuity. Model reasoning containers were removed before this call. RisuAI-rendered artifacts are represented by exact host tokens; freely rebuild all surrounding prose.\n\nFull ordered segments:\n${allSegments.map((s) => s.type === "mutable" ? `${s.id} [MUTABLE]:\n${s.text}` : `${s.id} [HOST ARTIFACT]: ${preview(s.text, 200)}`).join("\n\n---\n\n")}\n\nAllowed mutable segment IDs: ${segIds.join(", ")}\n${revisionSection}${draftLedgerSection}${preservationSection}${contextSection}\n\nCandidate instructions:\n- Apply only your system-defined lane. Do not duplicate another specialist's ownership.\n- Return exactly one candidate whose segments object contains every allowed substantive mutable segment ID exactly once, or return an empty candidates array.\n- The candidate must be readable from the first mutable segment through the last as one complete version of this turn. Missing IDs, partial excerpts, patch fragments, and implicit passthrough are rejected as an incomplete scene.\n- Reconstruct the full scene through this lane; do not submit advice, diagnostics, fragments, patches, or cosmetic synonym swaps.\n- Preserve Draft Ledger facts, beats, secrets, POV, identity state, user-owned decisions, and the current turn boundary. Rebuild wording and paragraph architecture without limitation.\n- Do not recreate model reasoning, approval labels, assistant commentary, prompt residue, or other meta artifacts.\n- No substantive scene segment may be empty. An empty value is allowed only to delete a wholly meta-only mutable segment and must include meta_artifact in addressed_issues.\n- evidence_refs must name Draft Ledger ledger_id, source_ref, or runtime context refs actually used.\n- retained_beats must identify the Draft Ledger units preserved in the scene.\n- proposed_additions must disclose every new factual claim, relationship fact, event, location fact, object state, backstory detail, motive, emotion, knowledge state, or personality judgment introduced by the candidate. Use [] when none.\n- confidence is trace-only metadata and never guarantees selection.\n- If this lane cannot materially improve the whole scene, return {"schema":"scene_rewrite_candidates.v1","role_id":"${role.role_id}","candidates":[]}. That response is traced as no_candidate, not as a successful rewrite.\n- Identify the concrete issues addressed and summarize the actual scene-level revision.\n\nReturn one compact JSON object only:\n{"schema":"scene_rewrite_candidates.v1","role_id":"${role.role_id}","candidates":[{"segments":{"SEG_ID":"complete replacement text for every allowed ID"},"evidence_refs":["ledger_or_context_ref"],"retained_beats":[{"id":"ledger_id","text":"retained beat"}],"proposed_additions":[],"addressed_issues":["issue_code"],"confidence":0.75,"change_summary":"what materially changed across the complete scene"}]}\n\nAllowed issue values for this lane: ${roleAllowedIssues(role.role_id).join(", ")}.`;
    }
    let finalSystemPrompt = systemPrompt;
    if (role.is_composer && directorInfo && directorInfo.composer_plain_scene) {
      finalSystemPrompt += `\n\nSCENE BODY OUTPUT OVERRIDE: For ${SCENE_REWRITE_SEGMENT_ID}, return only the complete final scene body with exact placeholders. Do not return JSON or a wrapper. This instruction overrides the generic JSON sentence above.`;
    } else if (role.is_prover) {
      finalSystemPrompt += "\n\nEVIDENCE QUOTE OVERRIDE: A preserved fact or beat may use an empty evidence_quote when it is negative, background, or implicit continuity verified by absence or non-contradiction. If a quote is supplied, it must be exact final-scene text. This overrides any generic instruction requiring a quote for every preserved item.";
    } else if (!role.is_input_planner && !role.is_judge && !role.is_prover && !role.is_composer
        && mutableSegs.length === 1 && mutableSegs[0].id === SCENE_REWRITE_SEGMENT_ID) {
      finalSystemPrompt += `\n\nSCENE KEY CONTRACT: The one and only writable key is ${SCENE_REWRITE_SEGMENT_ID}. Never return mutable_1, mutable_2, or physical segment keys.`;
    }
    return { system: finalSystemPrompt, user: userPrompt };
  }

  function normalizeCandidateReferences(values, limit) {
    const result = [];
    const seen = new Set();
    arrayFromCollection(values).slice(0, limit || 24).forEach((value) => {
      const source = typeof value === "string" ? { text: value } : asObject(value);
      const normalized = {
        id: truncate(source.id, 120).trim(),
        text: truncate(source.text != null ? source.text : source.value, 320).trim(),
        evidence_refs: uniqueList(
          arrayFromCollection(source.evidence_refs)
            .map((item) => truncate(item, 120).trim())
            .filter(Boolean)
        ),
        evidence_quote: truncate(source.evidence_quote, 180).trim(),
      };
      if (!normalized.id && !normalized.text) return;
      const key = stableDigest(normalized);
      if (seen.has(key)) return;
      seen.add(key);
      result.push(normalized);
    });
    return result;
  }

  function candidateValidationDiagnostic(code, candidateIndex, segmentIds, detail) {
    return {
      code: safeString(code),
      candidate_index: Number.isFinite(candidateIndex) ? candidateIndex : -1,
      segment_ids: uniqueList(arrayFromCollection(segmentIds).map((id) => safeString(id)).filter(Boolean)),
      detail: truncate(detail, 240).trim(),
    };
  }

  function recognizableCandidateEnvelope(parsed) {
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    if (Array.isArray(parsed.candidates)) return parsed;
    if (parsed.candidate && typeof parsed.candidate === "object") {
      return Object.assign({}, parsed, { candidates: [parsed.candidate] });
    }
    if (parsed.segments && typeof parsed.segments === "object") {
      return Object.assign({}, parsed, { candidates: [parsed] });
    }
    return null;
  }

  function validateCandidateSchemaDetailed(parsed, expectedRoleId, allowedSegmentIds, mutableSegs) {
    const diagnostics = [];
    const envelope = recognizableCandidateEnvelope(parsed);
    if (!envelope) {
      return {
        value: null,
        diagnostics: [candidateValidationDiagnostic("unrecognized_candidate_envelope", -1, [], "")],
      };
    }
    const roleId = safeString(expectedRoleId);
    if (safeString(envelope.schema) !== "scene_rewrite_candidates.v1") {
      diagnostics.push(candidateValidationDiagnostic("normalized_schema", -1, [], safeString(envelope.schema)));
    }
    if (safeString(envelope.role_id) !== roleId) {
      diagnostics.push(candidateValidationDiagnostic("normalized_role_id", -1, [], safeString(envelope.role_id)));
    }
    const allowedIds = uniqueList(arrayFromCollection(allowedSegmentIds).map((id) => safeString(id)));
    if (!allowedIds.length) {
      return {
        value: null,
        diagnostics: diagnostics.concat(candidateValidationDiagnostic("no_allowed_segments", -1, [], "")),
      };
    }
    if (envelope.candidates.length > 1) {
      return {
        value: null,
        diagnostics: diagnostics.concat(candidateValidationDiagnostic(
          "candidate_count_not_one",
          -1,
          [],
          `expected zero or one complete scene candidate, received ${envelope.candidates.length}`
        )),
      };
    }
    const allowed = new Set(allowedIds);
    const laneIssues = ROLE_ALLOWED_ISSUES[roleId]
      ? new Set(roleAllowedIssues(roleId))
      : null;
    const mutableById = {};
    (mutableSegs || []).forEach((segment) => { mutableById[segment.id] = segment; });
    const valid = [];
    const seen = new Set();

    envelope.candidates.forEach((candidate, candidateIndex) => {
      if (!candidate || typeof candidate !== "object") {
        diagnostics.push(candidateValidationDiagnostic("invalid_candidate_item", candidateIndex, [], ""));
        return;
      }
      if (candidate.role_id != null && safeString(candidate.role_id) !== roleId) {
        diagnostics.push(candidateValidationDiagnostic(
          "normalized_candidate_role_id",
          candidateIndex,
          [],
          safeString(candidate.role_id)
        ));
      }
      let rawSegments = asObject(candidate.segments);
      if (!Object.keys(rawSegments).length
          && allowedIds.length === 1
          && typeof candidate.scene_rewrite === "string") {
        rawSegments = { [allowedIds[0]]: candidate.scene_rewrite };
      }
      if (allowedIds.length === 1
          && allowedIds[0] === SCENE_REWRITE_SEGMENT_ID
          && !Object.prototype.hasOwnProperty.call(rawSegments, SCENE_REWRITE_SEGMENT_ID)) {
        const folded = foldPhysicalSegmentMapIntoScene(mutableById[SCENE_REWRITE_SEGMENT_ID], rawSegments);
        if (folded) {
          rawSegments = { [SCENE_REWRITE_SEGMENT_ID]: folded.scene_text };
          diagnostics.push(candidateValidationDiagnostic(
            "folded_physical_segments_to_scene",
            candidateIndex,
            folded.provided_segment_ids,
            [
              folded.missing_segment_ids.length
                ? `filled ${folded.missing_segment_ids.length} missing physical slots from draft_zero`
                : "all physical mutable slots recovered",
              folded.stripped_embedded_token_count
                ? `stripped ${folded.stripped_embedded_token_count} embedded preservation tokens before deterministic reassembly`
                : "",
            ].filter(Boolean).join("; ")
          ));
        }
      }
      const tags = arrayFromCollection(candidate.tags).map((tag) => safeString(tag)).filter(Boolean);
      const rawIssueValues = arrayFromCollection(candidate.addressed_issues || candidate.issues)
        .map((issue) => safeString(issue).trim().toLowerCase())
        .filter(Boolean);
      const unknownIssues = uniqueList(rawIssueValues.filter(
        (issue) => ISSUE_GROUPS.indexOf(issue) < 0
      ));
      if (unknownIssues.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "filtered_unknown_issue_codes",
          candidateIndex,
          [],
          unknownIssues.join(",")
        ));
      }
      const normalizedIssues = normalizeIssues(candidate.addressed_issues || candidate.issues, tags);
      const issues = laneIssues
        ? normalizedIssues.filter((issue) => laneIssues.has(issue))
        : normalizedIssues;
      const removedIssues = normalizedIssues.filter((issue) => issues.indexOf(issue) < 0);
      if (removedIssues.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "filtered_out_of_lane_issues",
          candidateIndex,
          [],
          removedIssues.join(",")
        ));
      }
      const normalizedSegments = {};
      const segmentOperations = {};
      const changedSegmentIds = [];
      let changedSegmentCount = 0;
      const missingSegmentIds = allowedIds.filter(
        (segmentId) => !Object.prototype.hasOwnProperty.call(rawSegments, segmentId)
      );
      if (missingSegmentIds.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "candidate_missing_required_segments",
          candidateIndex,
          missingSegmentIds,
          "complete scene candidates must contain every allowed mutable segment"
        ));
        return;
      }
      const emptySubstantiveSegmentIds = allowedIds.filter((segmentId) => {
        const rewrite = safeString(rawSegments[segmentId]);
        if (rewrite.trim()) return false;
        return !(issues.indexOf("meta_artifact") >= 0
          && isWhollyMetaArtifactText(mutableFullText(mutableById[segmentId])));
      });
      if (emptySubstantiveSegmentIds.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "candidate_empty_required_segments",
          candidateIndex,
          emptySubstantiveSegmentIds,
          "complete scene candidates cannot leave substantive segments empty"
        ));
        return;
      }
      const placeholderMismatchIds = allowedIds.filter((segmentId) =>
        !validateSceneFrameText(mutableById[segmentId], safeString(rawSegments[segmentId])).pass
      );
      if (placeholderMismatchIds.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "scene_frame_placeholder_mismatch",
          candidateIndex,
          placeholderMismatchIds,
          "every exact-preservation placeholder must appear once in the original order"
        ));
        return;
      }
      const foreignSegmentIds = Object.keys(rawSegments).filter((segmentId) => !allowed.has(segmentId));
      if (foreignSegmentIds.length) {
        diagnostics.push(candidateValidationDiagnostic(
          "candidate_contains_foreign_segments",
          candidateIndex,
          foreignSegmentIds,
          "candidate segments must match the allowed mutable segment set exactly"
        ));
        return;
      }
      allowedIds.forEach((segmentId) => {
        const original = mutableCoreText(mutableById[segmentId]);
        const rewrite = safeString(rawSegments[segmentId]);
        const deleteMeta = !rewrite.trim()
          && issues.indexOf("meta_artifact") >= 0
          && isWhollyMetaArtifactText(mutableFullText(mutableById[segmentId]));
        normalizedSegments[segmentId] = deleteMeta ? "" : rewrite;
        segmentOperations[segmentId] = deleteMeta ? "delete" : "replace";
        if (normalizedSegments[segmentId] !== original) {
          changedSegmentCount++;
          if (!deleteMeta) changedSegmentIds.push(segmentId);
        }
      });
      if (!changedSegmentCount) {
        diagnostics.push(candidateValidationDiagnostic(
          "candidate_has_no_usable_rewrite",
          candidateIndex,
          allowedIds,
          ""
        ));
        return;
      }
      const evidenceRefs = uniqueList(
        arrayFromCollection(candidate.evidence_refs)
          .map((ref) => truncate(ref, 160).trim())
          .filter(Boolean)
      ).slice(0, 32);
      const retainedBeats = normalizeCandidateReferences(candidate.retained_beats, 32);
      const proposedAdditions = normalizeCandidateReferences(candidate.proposed_additions, 24);
      const sceneSignature = stableDigest({ segments: normalizedSegments });
      const candidateId = `candidate_${stableDigest({
        role_id: roleId,
        scene_signature: sceneSignature,
        addressed_issues: issues,
        evidence_refs: evidenceRefs,
      })}`;
      if (seen.has(candidateId)) return;
      seen.add(candidateId);
      valid.push({
        candidate_id: candidateId,
        scene_signature: sceneSignature,
        role_id: roleId,
        supporting_roles: [roleId],
        duplicate_count: 0,
        segments: normalizedSegments,
        segment_operations: segmentOperations,
        evidence_refs: evidenceRefs,
        retained_beats: retainedBeats,
        proposed_additions: proposedAdditions,
        addressed_issues: issues,
        confidence: clampNumber(candidate.confidence, 0, 1, 0.5),
        change_summary: truncate(candidate.change_summary, 500),
        tags,
        coverage_complete: true,
        covered_segment_ids: allowedIds.slice(),
        changed_segment_ids: changedSegmentIds,
      });
    });
    if (!envelope.candidates.length) {
      return {
        value: { schema: "scene_rewrite_candidates.v1", role: roleId, candidates: [] },
        diagnostics,
      };
    }
    if (!valid.length) {
      return { value: null, diagnostics };
    }
    return {
      value: { schema: "scene_rewrite_candidates.v1", role: roleId, candidates: valid },
      diagnostics,
    };
  }

  function validateCandidateSchema(parsed, expectedRoleId, allowedSegmentIds, mutableSegs) {
    return validateCandidateSchemaDetailed(
      parsed,
      expectedRoleId,
      allowedSegmentIds,
      mutableSegs
    ).value;
  }

  const JUDGE_VERDICTS = Object.freeze(["accept", "accept_with_constraints", "reject"]);
  const JUDGE_HARD_VIOLATIONS = Object.freeze([
    "secret_leak",
    "pov_violation",
    "identity_continuity",
    "agency_takeover",
    "meta_artifact",
    "output_contract_violation",
  ]);

  function judgmentItems(values, allowedCandidateIds, limit) {
    const result = [];
    arrayFromCollection(values).slice(0, limit || 32).forEach((value) => {
      const item = typeof value === "string" ? { claim: value } : asObject(value);
      const candidateIds = uniqueList(
        arrayFromCollection(item.candidate_ids).map((id) => safeString(id)).filter(Boolean)
      );
      if (candidateIds.some((id) => !allowedCandidateIds.has(id))) return;
      const normalized = {
        claim: truncate(item.claim != null ? item.claim : item.text, 500).trim(),
        reason: truncate(item.reason, 400).trim(),
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
          .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
        candidate_ids: candidateIds,
        segment_ids: uniqueList(arrayFromCollection(item.segment_ids)
          .map((id) => truncate(id, 120).trim()).filter(Boolean)).slice(0, 24),
      };
      if (normalized.claim || normalized.reason) result.push(normalized);
    });
    return result;
  }

  function normalizeUnresolvedRequirements(values, sceneCandidates) {
    const allowedSegmentIds = new Set();
    arrayFromCollection(sceneCandidates).forEach((candidate) => {
      Object.keys(asObject(candidate && candidate.segments)).forEach((segmentId) => {
        allowedSegmentIds.add(safeString(segmentId));
      });
    });
    const result = [];
    const seen = new Set();
    arrayFromCollection(values).slice(0, 12).forEach((value) => {
      const item = asObject(value);
      const issueType = safeString(item.issue_type).trim().toLowerCase();
      const expectedRoleId = ISSUE_OWNER_ROLE[issueType];
      const roleId = safeString(item.role_id || expectedRoleId);
      const claim = truncate(item.claim != null ? item.claim : item.detail, 500).trim();
      if (!claim || !expectedRoleId || roleId !== expectedRoleId
          || SPECIALIST_ROLE_IDS.indexOf(roleId) < 0) return;
      const segmentIds = uniqueList(arrayFromCollection(item.segment_ids)
        .map((id) => safeString(id))
        .filter((id) => allowedSegmentIds.has(id))).slice(0, 24);
      if (!segmentIds.length) return;
      const normalized = {
        issue_type: issueType,
        role_id: roleId,
        claim,
        segment_ids: segmentIds,
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
          .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
      };
      normalized.requirement_id = `need_${stableDigest(normalized)}`;
      if (seen.has(normalized.requirement_id)) return;
      seen.add(normalized.requirement_id);
      result.push(normalized);
    });
    return result;
  }

  function draftLedgerIdSet(ledger) {
    const ids = new Set();
    Object.keys(asObject(ledger)).forEach((field) => {
      arrayFromCollection(ledger[field]).forEach((item) => {
        const id = safeString(item && item.ledger_id);
        if (id) ids.add(id);
      });
    });
    return ids;
  }

  function validateSemanticJudgment(parsed, sceneCandidates, draftLedger) {
    if (!parsed || safeString(parsed.schema) !== "semantic_judgment.v1"
        || !Array.isArray(parsed.candidate_judgments)) return null;
    const candidates = arrayFromCollection(sceneCandidates);
    const allowedIds = new Set(candidates.map((candidate) => safeString(candidate.candidate_id)).filter(Boolean));
    if (!allowedIds.size || parsed.candidate_judgments.length !== allowedIds.size) return null;
    const ledgerIds = draftLedgerIdSet(draftLedger);
    const seen = new Set();
    const judgments = [];
    for (const raw of parsed.candidate_judgments) {
      const item = asObject(raw);
      const candidateId = safeString(item.candidate_id);
      const verdict = safeString(item.verdict);
      if (!allowedIds.has(candidateId) || seen.has(candidateId)
          || JUDGE_VERDICTS.indexOf(verdict) < 0) return null;
      seen.add(candidateId);
      const preserved = uniqueList(arrayFromCollection(item.preserved_ledger_ids).map((id) => safeString(id)).filter(Boolean));
      const missing = uniqueList(arrayFromCollection(item.missing_ledger_ids).map((id) => safeString(id)).filter(Boolean));
      if (preserved.concat(missing).some((id) => !ledgerIds.has(id))) return null;
      const hard = arrayFromCollection(item.hard_violations).slice(0, 24).map((value) => {
        const violation = asObject(value);
        const type = safeString(violation.type);
        if (JUDGE_HARD_VIOLATIONS.indexOf(type) < 0) return null;
        return {
          type,
          detail: truncate(violation.detail != null ? violation.detail : violation.claim, 500).trim(),
          evidence_refs: uniqueList(arrayFromCollection(violation.evidence_refs)
            .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
          segment_ids: uniqueList(arrayFromCollection(violation.segment_ids)
            .map((id) => truncate(id, 120).trim()).filter(Boolean)).slice(0, 24),
        };
      });
      if (hard.some((item2) => !item2) || (hard.length && verdict === "accept")) return null;
      judgments.push({
        candidate_id: candidateId,
        verdict,
        preserved_ledger_ids: preserved,
        missing_ledger_ids: missing,
        unsupported_additions: judgmentItems(item.unsupported_additions, allowedIds, 24),
        hard_violations: hard,
        accepted_elements: judgmentItems(item.accepted_elements, allowedIds, 32),
        rejected_elements: judgmentItems(item.rejected_elements, allowedIds, 32),
        quality_gains: judgmentItems(item.quality_gains, allowedIds, 24),
        quality_regressions: judgmentItems(item.quality_regressions, allowedIds, 24),
      });
    }
    const cross = asObject(parsed.cross_candidate);
    return {
      schema: "semantic_judgment.v1",
      candidate_judgments: judgments,
      cross_candidate: {
        consensus: judgmentItems(cross.consensus, allowedIds, 32),
        complementary: judgmentItems(cross.complementary, allowedIds, 32),
        conflicts: judgmentItems(cross.conflicts, allowedIds, 32),
      },
      unresolved_requirements: normalizeUnresolvedRequirements(
        parsed.unresolved_requirements,
        candidates
      ),
      scene_requirements: {
        target_arc: truncate(asObject(parsed.scene_requirements).target_arc, 600).trim(),
        target_voice: truncate(asObject(parsed.scene_requirements).target_voice, 600).trim(),
        target_pacing: truncate(asObject(parsed.scene_requirements).target_pacing, 600).trim(),
      },
    };
  }

  const PROOF_ITEM_STATUSES = Object.freeze(["preserved", "missing", "contradicted", "uncertain"]);
  const PROOF_CONSTRAINT_STATUSES = Object.freeze(["satisfied", "violated", "uncertain"]);
  const PROOF_QUALITY_STATUSES = Object.freeze(["realized", "missing", "regressed"]);
  const PROOF_RESIDUAL_QUALITY_STATUSES = Object.freeze(["clean", "issue"]);
  const PROOF_RESIDUAL_QUALITY_IDS = Object.freeze([
    "mechanics_and_wording",
    "register_and_era",
    "repetition_and_exposition",
    "grounded_psychology_and_relationship",
    "scene_coherence_and_ending",
  ]);
  const PROOF_OUTPUT_CONTRACT_KEYS = Object.freeze([
    "language_ok",
    "turn_boundary_ok",
    "user_agency_ok",
    "meta_free",
    "format_ok",
  ]);

  function rejectSemanticProof(diagnostics, field, reason, detail) {
    if (Array.isArray(diagnostics)) {
      diagnostics.push({
        code: `semantic_proof_${safeString(reason)}`,
        field: safeString(field),
        detail: truncate(detail, 500).trim(),
      });
    }
    return null;
  }

  function draftLedgerIdsForField(draftLedger, field) {
    return uniqueList(
      arrayFromCollection(asObject(draftLedger)[field])
        .map((item) => safeString(item && item.ledger_id))
        .filter(Boolean)
    );
  }

  function normalizeProofChecks(values, expectedIds, statuses, fieldName, diagnostics) {
    if (!Array.isArray(values)) {
      return rejectSemanticProof(diagnostics, fieldName, "expected_array", `received=${typeof values}`);
    }
    const expected = new Set(expectedIds);
    if (values.length !== expected.size) {
      return rejectSemanticProof(
        diagnostics,
        fieldName,
        "coverage_count_mismatch",
        `expected=${expected.size} actual=${values.length}`
      );
    }
    const seen = new Set();
    const normalized = [];
    for (let index = 0; index < values.length; index++) {
      const raw = values[index];
      const item = asObject(raw);
      const ledgerId = safeString(item.ledger_id);
      const status = safeString(item.status);
      if (!expected.has(ledgerId)) {
        return rejectSemanticProof(diagnostics, `${fieldName}[${index}].ledger_id`, "unknown_id", ledgerId);
      }
      if (seen.has(ledgerId)) {
        return rejectSemanticProof(diagnostics, `${fieldName}[${index}].ledger_id`, "duplicate_id", ledgerId);
      }
      if (statuses.indexOf(status) < 0) {
        return rejectSemanticProof(diagnostics, `${fieldName}[${index}].status`, "invalid_status", status);
      }
      seen.add(ledgerId);
      normalized.push({
        ledger_id: ledgerId,
        status,
        detail: truncate(item.detail, 500).trim(),
        evidence_quote: truncate(item.evidence_quote, 240).trim(),
      });
    }
    return normalized;
  }

  function normalizeQualityGainChecks(
    values,
    requiredContributions,
    allowedSegments,
    finalEvidenceText,
    finalTextBySegment,
    originalTextBySegment,
    diagnostics
  ) {
    const required = arrayFromCollection(requiredContributions);
    if (!required.length) {
      return values == null || (Array.isArray(values) && values.length === 0)
        ? []
        : rejectSemanticProof(diagnostics, "quality_gain_checks", "unexpected_items", `actual=${arrayFromCollection(values).length}`);
    }
    if (!Array.isArray(values)) {
      return rejectSemanticProof(diagnostics, "quality_gain_checks", "expected_array", `received=${typeof values}`);
    }
    if (values.length !== required.length) {
      return rejectSemanticProof(
        diagnostics,
        "quality_gain_checks",
        "coverage_count_mismatch",
        `expected=${required.length} actual=${values.length}`
      );
    }
    const expected = {};
    required.forEach((item) => {
      expected[safeString(item.contribution_id)] = item;
    });
    const seen = new Set();
    const normalized = [];
    for (let index = 0; index < values.length; index++) {
      const raw = values[index];
      const item = asObject(raw);
      const contributionId = safeString(item.contribution_id);
      const status = safeString(item.status);
      const expectedItem = expected[contributionId];
      if (!expectedItem) {
        return rejectSemanticProof(diagnostics, `quality_gain_checks[${index}].contribution_id`, "unknown_id", contributionId);
      }
      if (seen.has(contributionId)) {
        return rejectSemanticProof(diagnostics, `quality_gain_checks[${index}].contribution_id`, "duplicate_id", contributionId);
      }
      if (PROOF_QUALITY_STATUSES.indexOf(status) < 0) {
        return rejectSemanticProof(diagnostics, `quality_gain_checks[${index}].status`, "invalid_status", status);
      }
      seen.add(contributionId);
      let segmentIds = uniqueList(arrayFromCollection(item.segment_ids)
        .map((id) => safeString(id))
        .filter(Boolean));
      const expectedSegmentIds = uniqueList(arrayFromCollection(expectedItem.segment_ids)
        .map((id) => safeString(id))
        .filter(Boolean));
      if (!segmentIds.length) {
        segmentIds = expectedSegmentIds.slice();
      }
      if (segmentIds.some((id) => !allowedSegments.has(id))) {
        return rejectSemanticProof(diagnostics, `quality_gain_checks[${index}].segment_ids`, "unknown_segment", segmentIds.join(","));
      }
      if (segmentIds.slice().sort().join("\u0000")
          !== expectedSegmentIds.slice().sort().join("\u0000")) {
        return rejectSemanticProof(
          diagnostics,
          `quality_gain_checks[${index}].segment_ids`,
          "segment_coverage_mismatch",
          `expected=${expectedSegmentIds.join(",")} actual=${segmentIds.join(",")}`
        );
      }
      const evidenceQuote = truncate(item.evidence_quote, 320).trim();
      const normalizedQuote = evidenceQuote.replace(/\s+/g, " ").toLowerCase();
      const expectedSegmentEvidence = expectedSegmentIds
        .map((segmentId) => safeString(asObject(finalTextBySegment)[segmentId]))
        .join("\n")
        .replace(/\s+/g, " ")
        .toLowerCase();
      const quoteIsNewMaterialEvidence = expectedSegmentIds.some((segmentId) => {
        const originalText = safeString(asObject(originalTextBySegment)[segmentId]);
        const finalText = safeString(asObject(finalTextBySegment)[segmentId]);
        const normalizedOriginal = originalText.replace(/\s+/g, " ").toLowerCase();
        const normalizedFinal = finalText.replace(/\s+/g, " ").toLowerCase();
        return rewriteMateriality(originalText, finalText).material
          && normalizedFinal.includes(normalizedQuote)
          && !normalizedOriginal.includes(normalizedQuote);
      });
      if (status === "realized" && (!normalizedQuote
          || !finalEvidenceText.includes(normalizedQuote)
          || !expectedSegmentEvidence.includes(normalizedQuote)
          || !quoteIsNewMaterialEvidence)) {
        return rejectSemanticProof(
          diagnostics,
          `quality_gain_checks[${index}].evidence_quote`,
          "realized_quote_not_new_in_final",
          preview(evidenceQuote, 120)
        );
      }
      normalized.push({
        contribution_id: contributionId,
        role_id: safeString(expectedItem.role_id),
        candidate_id: safeString(expectedItem.candidate_id),
        status,
        detail: truncate(item.detail, 500).trim(),
        evidence_quote: evidenceQuote,
        segment_ids: segmentIds,
      });
    }
    return normalized;
  }

  function normalizeResidualQualityChecks(values, allowedSegments, finalTextBySegment, diagnostics) {
    if (!Array.isArray(values) || values.length !== PROOF_RESIDUAL_QUALITY_IDS.length) {
      return rejectSemanticProof(
        diagnostics,
        "residual_quality_checks",
        "coverage_count_mismatch",
        `expected=${PROOF_RESIDUAL_QUALITY_IDS.length} actual=${arrayFromCollection(values).length}`
      );
    }
    const expected = new Set(PROOF_RESIDUAL_QUALITY_IDS);
    const seen = new Set();
    const normalized = [];
    for (let index = 0; index < values.length; index++) {
      const raw = values[index];
      const item = asObject(raw);
      const checkId = safeString(item.check_id);
      const status = safeString(item.status);
      if (!expected.has(checkId) || seen.has(checkId)
          || PROOF_RESIDUAL_QUALITY_STATUSES.indexOf(status) < 0) {
        return rejectSemanticProof(
          diagnostics,
          `residual_quality_checks[${index}]`,
          "invalid_check",
          `check_id=${checkId} status=${status}`
        );
      }
      seen.add(checkId);
      const segmentIds = uniqueList(arrayFromCollection(item.segment_ids)
        .map((id) => safeString(id))
        .filter(Boolean));
      if (segmentIds.some((id) => !allowedSegments.has(id))) {
        return rejectSemanticProof(diagnostics, `residual_quality_checks[${index}].segment_ids`, "unknown_segment", segmentIds.join(","));
      }
      const evidenceQuote = truncate(item.evidence_quote, 320).trim();
      if (status === "issue") {
        const normalizedQuote = evidenceQuote.replace(/\s+/g, " ").toLowerCase();
        const quotedInDeclaredSegment = segmentIds.some((segmentId) =>
          safeString(asObject(finalTextBySegment)[segmentId])
            .replace(/\s+/g, " ")
            .toLowerCase()
            .includes(normalizedQuote)
        );
        if (!normalizedQuote || !segmentIds.length || !quotedInDeclaredSegment) {
          return rejectSemanticProof(
            diagnostics,
            `residual_quality_checks[${index}].evidence_quote`,
            "issue_quote_not_in_final_segment",
            preview(evidenceQuote, 120)
          );
        }
      }
      normalized.push({
        check_id: checkId,
        status,
        detail: truncate(item.detail, 600).trim(),
        evidence_quote: evidenceQuote,
        segment_ids: segmentIds,
      });
    }
    return normalized;
  }

  function validateSemanticProof(parsed, draftLedger, mutableSegs, finalSegments, fusionPlan, diagnostics) {
    if (!parsed || safeString(parsed.schema) !== "semantic_proof.v1") {
      return rejectSemanticProof(
        diagnostics,
        "schema",
        "invalid_schema",
        safeString(parsed && parsed.schema)
      );
    }
    const declaredVerdict = safeString(parsed.declared_verdict);
    if (["pass", "repair", "fail"].indexOf(declaredVerdict) < 0) {
      return rejectSemanticProof(diagnostics, "declared_verdict", "invalid_status", declaredVerdict);
    }

    const factChecks = normalizeProofChecks(
      parsed.fact_checks,
      draftLedgerIdsForField(draftLedger, "established_facts"),
      PROOF_ITEM_STATUSES,
      "fact_checks",
      diagnostics
    );
    const beatChecks = normalizeProofChecks(
      parsed.beat_checks,
      draftLedgerIdsForField(draftLedger, "scene_beats"),
      PROOF_ITEM_STATUSES,
      "beat_checks",
      diagnostics
    );
    const constraintChecks = normalizeProofChecks(
      parsed.constraint_checks,
      draftLedgerIdsForField(draftLedger, "hard_constraints"),
      PROOF_CONSTRAINT_STATUSES,
      "constraint_checks",
      diagnostics
    );
    if (!factChecks || !beatChecks || !constraintChecks) return null;
    const finalEvidenceText = arrayFromCollection(finalSegments)
      .map((segment) => safeString(segment && (segment.final_text != null ? segment.final_text : segment.text)))
      .join("\n")
      .replace(/\s+/g, " ")
      .toLowerCase();
    const finalTextBySegment = {};
    const originalTextBySegment = {};
    arrayFromCollection(finalSegments).forEach((segment) => {
      const segmentId = safeString(segment && segment.id);
      if (!segmentId) return;
      finalTextBySegment[segmentId] = safeString(
        segment && (segment.final_text != null ? segment.final_text : segment.text)
      );
    });
    arrayFromCollection(mutableSegs).forEach((segment) => {
      const segmentId = safeString(segment && segment.id);
      if (segmentId) originalTextBySegment[segmentId] = mutableFullText(segment);
    });
    const unsupportedFactIndex = factChecks.findIndex((item) => {
      if (item.status !== "preserved") return false;
      const quote = safeString(item.evidence_quote).replace(/\s+/g, " ").trim().toLowerCase();
      return !!quote && !finalEvidenceText.includes(quote);
    });
    const unsupportedBeatIndex = unsupportedFactIndex < 0
      ? beatChecks.findIndex((item) => {
        if (item.status !== "preserved") return false;
        const quote = safeString(item.evidence_quote).replace(/\s+/g, " ").trim().toLowerCase();
        return !!quote && !finalEvidenceText.includes(quote);
      })
      : -1;
    const unsupportedPreservedClaim = unsupportedFactIndex >= 0
      ? factChecks[unsupportedFactIndex]
      : (unsupportedBeatIndex >= 0 ? beatChecks[unsupportedBeatIndex] : null);
    if (unsupportedPreservedClaim) {
      return rejectSemanticProof(
        diagnostics,
        unsupportedFactIndex >= 0
          ? `fact_checks[${unsupportedFactIndex}].evidence_quote`
          : `beat_checks[${unsupportedBeatIndex}].evidence_quote`,
        "preserved_quote_not_in_final",
        `ledger_id=${unsupportedPreservedClaim.ledger_id} quote=${preview(unsupportedPreservedClaim.evidence_quote, 120)}`
      );
    }

    const allowedSegments = new Set(arrayFromCollection(mutableSegs).map((segment) => safeString(segment.id)));
    const qualityGainChecks = normalizeQualityGainChecks(
      parsed.quality_gain_checks,
      asObject(fusionPlan).required_contributions,
      allowedSegments,
      finalEvidenceText,
      finalTextBySegment,
      originalTextBySegment,
      diagnostics
    );
    if (!qualityGainChecks) return null;
    const residualQualityChecks = normalizeResidualQualityChecks(
      parsed.residual_quality_checks,
      allowedSegments,
      finalTextBySegment,
      diagnostics
    );
    if (!residualQualityChecks) return null;
    const hardViolations = [];
    if (!Array.isArray(parsed.hard_violations)) {
      return rejectSemanticProof(diagnostics, "hard_violations", "expected_array", `received=${typeof parsed.hard_violations}`);
    }
    for (let index = 0; index < parsed.hard_violations.slice(0, 32).length; index++) {
      const raw = parsed.hard_violations[index];
      const item = asObject(raw);
      const type = safeString(item.type);
      const detail = truncate(item.detail, 500).trim();
      const segmentIds = uniqueList(arrayFromCollection(item.segment_ids)
        .map((id) => safeString(id)).filter(Boolean));
      if (JUDGE_HARD_VIOLATIONS.indexOf(type) < 0 || !detail
          || segmentIds.some((id) => !allowedSegments.has(id))) {
        return rejectSemanticProof(
          diagnostics,
          `hard_violations[${index}]`,
          "invalid_violation",
          `type=${type} segments=${segmentIds.join(",")}`
        );
      }
      hardViolations.push({
        type,
        detail,
        segment_ids: segmentIds,
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
          .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
      });
    }

    const unsupportedAdditions = [];
    if (!Array.isArray(parsed.unsupported_additions)) {
      return rejectSemanticProof(diagnostics, "unsupported_additions", "expected_array", `received=${typeof parsed.unsupported_additions}`);
    }
    for (let index = 0; index < parsed.unsupported_additions.slice(0, 32).length; index++) {
      const raw = parsed.unsupported_additions[index];
      const item = asObject(raw);
      const claim = truncate(item.claim, 500).trim();
      const segmentIds = uniqueList(arrayFromCollection(item.segment_ids)
        .map((id) => safeString(id)).filter(Boolean));
      if (!claim || segmentIds.some((id) => !allowedSegments.has(id))) {
        return rejectSemanticProof(
          diagnostics,
          `unsupported_additions[${index}]`,
          "invalid_addition",
          `claim=${preview(claim, 120)} segments=${segmentIds.join(",")}`
        );
      }
      unsupportedAdditions.push({
        claim,
        reason: truncate(item.reason, 500).trim(),
        segment_ids: segmentIds,
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
          .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
      });
    }

    const outputContract = asObject(parsed.output_contract);
    if (PROOF_OUTPUT_CONTRACT_KEYS.some((key) => typeof outputContract[key] !== "boolean")) {
      const invalidKey = PROOF_OUTPUT_CONTRACT_KEYS.find((key) => typeof outputContract[key] !== "boolean");
      return rejectSemanticProof(
        diagnostics,
        `output_contract.${invalidKey}`,
        "expected_boolean",
        `received=${typeof outputContract[invalidKey]}`
      );
    }
    const normalizedOutputContract = {};
    PROOF_OUTPUT_CONTRACT_KEYS.forEach((key) => {
      normalizedOutputContract[key] = outputContract[key];
    });

    const repairInstructions = [];
    if (!Array.isArray(parsed.repair_instructions)) {
      return rejectSemanticProof(diagnostics, "repair_instructions", "expected_array", `received=${typeof parsed.repair_instructions}`);
    }
    for (let index = 0; index < parsed.repair_instructions.slice(0, 24).length; index++) {
      const raw = parsed.repair_instructions[index];
      const item = asObject(raw);
      const segmentId = safeString(item.segment_id);
      const instruction = truncate(item.instruction, 800).trim();
      if (!allowedSegments.has(segmentId) || !instruction) {
        return rejectSemanticProof(
          diagnostics,
          `repair_instructions[${index}]`,
          "invalid_instruction",
          `segment_id=${segmentId} instruction=${preview(instruction, 120)}`
        );
      }
      repairInstructions.push({
        segment_id: segmentId,
        instruction,
        evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
          .map((ref) => truncate(ref, 160).trim()).filter(Boolean)).slice(0, 24),
        prohibited: uniqueList(arrayFromCollection(item.prohibited)
          .map((value) => truncate(value, 300).trim()).filter(Boolean)).slice(0, 24),
      });
    }

    const failedFacts = factChecks.filter((item) => item.status !== "preserved");
    const failedBeats = beatChecks.filter((item) => item.status !== "preserved");
    const failedConstraints = constraintChecks.filter((item) => item.status !== "satisfied");
    const failedQualityGains = qualityGainChecks.filter((item) => item.status !== "realized");
    const failedResidualQuality = residualQualityChecks.filter((item) => item.status !== "clean");
    const failedOutputContract = PROOF_OUTPUT_CONTRACT_KEYS.filter((key) => !normalizedOutputContract[key]);
    const repairSegmentIds = new Set(repairInstructions.map((item) => item.segment_id));
    const repairCoversReference = (reference) => {
      const target = safeString(reference).toLowerCase();
      return repairInstructions.some((instruction) =>
        arrayFromCollection(instruction.evidence_refs)
          .some((ref) => safeString(ref).toLowerCase() === target)
        || safeString(instruction.instruction).toLowerCase().includes(target)
      );
    };
    const repairsCoverSegments = (segmentIds) => {
      const ids = arrayFromCollection(segmentIds);
      return ids.length
        ? ids.every((segmentId) => repairSegmentIds.has(segmentId))
        : repairInstructions.length > 0;
    };
    const uncoveredFactRepairs = failedFacts.filter((item) =>
      !repairCoversReference(item.ledger_id)
    );
    const uncoveredBeatRepairs = failedBeats.filter((item) =>
      !repairCoversReference(item.ledger_id)
    );
    const uncoveredConstraintRepairs = failedConstraints.filter((item) =>
      !repairCoversReference(item.ledger_id)
    );
    const uncoveredQualityRepairs = failedQualityGains.filter((item) =>
      !repairCoversReference(item.contribution_id)
      || !repairsCoverSegments(item.segment_ids)
    );
    const uncoveredResidualRepairs = failedResidualQuality.filter((item) =>
      !repairCoversReference(item.check_id)
      || !repairsCoverSegments(item.segment_ids)
    );
    const uncoveredHardViolationRepairs = hardViolations.filter((item) =>
      !repairsCoverSegments(item.segment_ids)
      || !repairCoversReference(item.type)
    );
    const uncoveredUnsupportedAdditionRepairs = unsupportedAdditions.filter((item) =>
      !repairsCoverSegments(item.segment_ids)
      || !repairInstructions.some((instruction) =>
        safeString(instruction.instruction).toLowerCase()
          .includes(safeString(item.claim).toLowerCase().slice(0, 48))
      )
    );
    const uncoveredOutputContractRepairs = failedOutputContract.filter((key) =>
      !repairCoversReference(`output_contract:${key}`)
      && !repairCoversReference(key)
    );
    const reasonCodes = uniqueList([]
      .concat(failedFacts.map((item) => `fact_${item.status}:${item.ledger_id}`))
      .concat(failedBeats.map((item) => `beat_${item.status}:${item.ledger_id}`))
      .concat(failedConstraints.map((item) => `constraint_${item.status}:${item.ledger_id}`))
      .concat(failedQualityGains.map((item) => `quality_gain_${item.status}:${item.contribution_id}`))
      .concat(failedResidualQuality.map((item) => `residual_quality_issue:${item.check_id}`))
      .concat(uncoveredFactRepairs.map((item) => `repair_missing_for_fact:${item.ledger_id}`))
      .concat(uncoveredBeatRepairs.map((item) => `repair_missing_for_beat:${item.ledger_id}`))
      .concat(uncoveredConstraintRepairs.map((item) => `repair_missing_for_constraint:${item.ledger_id}`))
      .concat(uncoveredQualityRepairs.map((item) => `repair_missing_for_gain:${item.contribution_id}`))
      .concat(uncoveredResidualRepairs.map((item) => `repair_missing_for_residual:${item.check_id}`))
      .concat(uncoveredHardViolationRepairs.map((item) => `repair_missing_for_hard_violation:${item.type}`))
      .concat(uncoveredUnsupportedAdditionRepairs.length ? ["repair_missing_for_unsupported_addition"] : [])
      .concat(uncoveredOutputContractRepairs.map((key) => `repair_missing_for_output_contract:${key}`))
      .concat(hardViolations.map((item) => `hard_violation:${item.type}`))
      .concat(unsupportedAdditions.length ? ["unsupported_addition"] : [])
      .concat(failedOutputContract.map((key) => `output_contract:${key}`)));
    const clean = reasonCodes.length === 0;
    let verdict = "fail";
    if (clean && declaredVerdict === "pass") {
      verdict = "pass";
    } else if (!clean && declaredVerdict !== "fail" && repairInstructions.length
        && !uncoveredFactRepairs.length
        && !uncoveredBeatRepairs.length
        && !uncoveredConstraintRepairs.length
        && !uncoveredQualityRepairs.length
        && !uncoveredResidualRepairs.length
        && !uncoveredHardViolationRepairs.length
        && !uncoveredUnsupportedAdditionRepairs.length
        && !uncoveredOutputContractRepairs.length) {
      verdict = "repair";
    } else if (clean) {
      reasonCodes.push(`declared_${declaredVerdict}_despite_clean_proof`);
    } else if (!repairInstructions.length) {
      reasonCodes.push("no_valid_repair_instruction");
    }

    return {
      schema: "semantic_proof.v1",
      declared_verdict: declaredVerdict,
      verdict,
      fact_checks: factChecks,
      beat_checks: beatChecks,
      constraint_checks: constraintChecks,
      quality_gain_checks: qualityGainChecks,
      residual_quality_checks: residualQualityChecks,
      hard_violations: hardViolations,
      unsupported_additions: unsupportedAdditions,
      output_contract: normalizedOutputContract,
      repair_instructions: repairInstructions,
      reason_codes: uniqueList(reasonCodes),
    };
  }

  function synthesizeWholeSceneRepairProof(proof, mutableSegs) {
    if (!proof || proof.verdict === "pass" || proof.declared_verdict === "fail") return proof;
    const segmentIds = uniqueList(arrayFromCollection(mutableSegs)
      .filter((segment) => segment && segment.type === "mutable")
      .map((segment) => safeString(segment.id))
      .filter(Boolean));
    if (!segmentIds.length) return proof;

    const references = [];
    const repairDetails = [];
    const addFailure = (reference, detail) => {
      const ref = truncate(reference, 160).trim();
      const text = truncate(detail, 420).trim();
      if (ref) references.push(ref);
      if (text) repairDetails.push(`${ref || "failure"}: ${text}`);
    };
    arrayFromCollection(proof.fact_checks).forEach((item) => {
      if (item.status !== "preserved") addFailure(item.ledger_id, item.detail || `fact ${item.status}`);
    });
    arrayFromCollection(proof.beat_checks).forEach((item) => {
      if (item.status !== "preserved") addFailure(item.ledger_id, item.detail || `beat ${item.status}`);
    });
    arrayFromCollection(proof.constraint_checks).forEach((item) => {
      if (item.status !== "satisfied") addFailure(item.ledger_id, item.detail || `constraint ${item.status}`);
    });
    arrayFromCollection(proof.quality_gain_checks).forEach((item) => {
      if (item.status !== "realized") addFailure(item.contribution_id, item.detail || `quality gain ${item.status}`);
    });
    arrayFromCollection(proof.residual_quality_checks).forEach((item) => {
      if (item.status === "issue") addFailure(item.check_id, item.detail || "residual quality issue");
    });
    arrayFromCollection(proof.hard_violations).forEach((item) => {
      addFailure(item.type, item.detail || "hard semantic violation");
    });
    arrayFromCollection(proof.unsupported_additions).forEach((item, index) => {
      addFailure(`unsupported_addition:${index + 1}`, item.claim || item.reason || "remove unsupported addition");
    });
    PROOF_OUTPUT_CONTRACT_KEYS.forEach((key) => {
      if (!asObject(proof.output_contract)[key]) addFailure(`output_contract:${key}`, `repair ${key}`);
    });

    const instruction = [
      "Recompose the complete scene, not a local patch.",
      "Resolve every listed semantic, contract, and prose failure while preserving all grounded facts, chronology, identity/reveal state, POV, secrets, and user agency.",
      "Remove all model reasoning, prompt residue, approval labels, output wrappers, and language/format violations.",
      "Realize the accepted specialist contributions as natural scene prose and rebuild weak exposition, transitions, rhythm, characterization, and ending where required.",
      repairDetails.length ? `Failures to resolve: ${repairDetails.join(" | ")}` : "Resolve every Semantic Prover reason code.",
    ].join(" ");
    const repairInstructions = segmentIds.map((segmentId) => ({
      segment_id: segmentId,
      instruction: truncate(instruction, 4000),
      evidence_refs: uniqueList(references).slice(0, 64),
      prohibited: uniqueList(arrayFromCollection(proof.hard_violations)
        .map((item) => truncate(item && (item.detail || item.type), 300).trim())
        .concat(arrayFromCollection(proof.unsupported_additions)
          .map((item) => truncate(item && item.claim, 300).trim()))
        .filter(Boolean)).slice(0, 32),
    }));

    return Object.assign({}, proof, {
      declared_verdict: "repair",
      verdict: "repair",
      repair_instructions: repairInstructions,
      reason_codes: uniqueList(arrayFromCollection(proof.reason_codes)
        .concat(["deterministic_whole_scene_repair_synthesized"])),
      repair_synthesized: true,
    });
  }

  function assessSemanticProofReturn(proof) {
    if (!proof) {
      return { returnable: false, degraded: false, blocking_reasons: ["semantic_proof_missing"], quality_debt: [] };
    }
    if (proof.verdict === "pass") {
      return { returnable: true, degraded: false, blocking_reasons: [], quality_debt: [] };
    }

    const blocking = [];
    if (safeString(proof.declared_verdict) === "fail") {
      blocking.push("semantic_prover_declared_fail");
    }
    arrayFromCollection(proof.hard_violations).forEach((item) => {
      blocking.push(`hard_violation:${safeString(item && item.type) || "unknown"}`);
    });
    if (arrayFromCollection(proof.unsupported_additions).length) blocking.push("unsupported_addition");
    arrayFromCollection(proof.fact_checks).forEach((item) => {
      if (item.status !== "preserved") blocking.push(`fact_${safeString(item.status)}:${safeString(item.ledger_id)}`);
    });
    arrayFromCollection(proof.beat_checks).forEach((item) => {
      if (item.status !== "preserved") blocking.push(`beat_${safeString(item.status)}:${safeString(item.ledger_id)}`);
    });
    arrayFromCollection(proof.constraint_checks).forEach((item) => {
      if (item.status !== "satisfied") blocking.push(`constraint_${safeString(item.status)}:${safeString(item.ledger_id)}`);
    });
    PROOF_OUTPUT_CONTRACT_KEYS.forEach((key) => {
      if (!asObject(proof.output_contract)[key]) blocking.push(`output_contract:${key}`);
    });

    const qualityDebt = uniqueList([]
      .concat(arrayFromCollection(proof.quality_gain_checks)
        .filter((item) => item.status !== "realized")
        .map((item) => `quality_gain_${item.status}:${item.contribution_id}`))
      .concat(arrayFromCollection(proof.residual_quality_checks)
        .filter((item) => item.status === "issue")
        .map((item) => `residual_quality_issue:${item.check_id}`)));
    if (safeString(proof.verdict) === "fail"
        && safeString(proof.declared_verdict) !== "fail"
        && blocking.length === 0
        && qualityDebt.length === 0) {
      blocking.push("semantic_proof_verdict_fail");
    }
    return {
      returnable: blocking.length === 0,
      degraded: blocking.length === 0,
      blocking_reasons: uniqueList(blocking),
      quality_debt: qualityDebt,
    };
  }

  function degradedSemanticProof(proof, reasonCodes) {
    const source = asObject(proof);
    const reasons = uniqueList(arrayFromCollection(reasonCodes)
      .concat(arrayFromCollection(source.reason_codes))
      .map((item) => safeString(item).trim())
      .filter(Boolean));
    return Object.assign({}, source, {
      schema: safeString(source.schema) || "semantic_proof.v1",
      declared_verdict: "degraded",
      original_declared_verdict: safeString(source.declared_verdict),
      verdict: "degraded",
      returnable_degraded: true,
      reason_codes: reasons,
      quality_debt: uniqueList(arrayFromCollection(source.quality_debt).concat(reasons)),
      fact_checks: arrayFromCollection(source.fact_checks),
      beat_checks: arrayFromCollection(source.beat_checks),
      constraint_checks: arrayFromCollection(source.constraint_checks),
      quality_gain_checks: arrayFromCollection(source.quality_gain_checks),
      residual_quality_checks: arrayFromCollection(source.residual_quality_checks),
      hard_violations: arrayFromCollection(source.hard_violations),
      unsupported_additions: arrayFromCollection(source.unsupported_additions),
      output_contract: asObject(source.output_contract),
      repair_instructions: arrayFromCollection(source.repair_instructions),
    });
  }

  function buildRequiredContributions(acceptedJudgments, candidateMap) {
    const contributions = [];
    const seen = new Set();
    arrayFromCollection(acceptedJudgments).forEach((judgment) => {
      const candidate = candidateMap[judgment.candidate_id];
      if (!candidate) return;
      const candidateSegmentIds = arrayFromCollection(candidate.changed_segment_ids).length
        ? uniqueList(arrayFromCollection(candidate.changed_segment_ids).map((id) => safeString(id)).filter(Boolean))
        : Object.keys(asObject(candidate.segments)).filter(
          (segmentId) => safeString(asObject(candidate.segment_operations)[segmentId]) === "replace"
        );
      if (!candidateSegmentIds.length) return;
      const rejectedClaims = new Set(arrayFromCollection(judgment.rejected_elements)
        .map((item) => safeString(item && item.claim).trim().toLowerCase())
        .filter(Boolean));
      const qualitySources = arrayFromCollection(judgment.quality_gains);
      const acceptedSources = arrayFromCollection(judgment.accepted_elements);
      const sources = (qualitySources.length ? qualitySources : acceptedSources)
        .slice(0, 2)
        .map((item) => ({
          item,
          source: qualitySources.length ? "quality_gain" : "accepted_element",
        }));
      const contributionCountBefore = contributions.length;
      sources.forEach((entry) => {
        const item = asObject(entry.item);
        const claim = truncate(item.claim != null ? item.claim : item.text, 500).trim();
        if (!claim || rejectedClaims.has(claim.toLowerCase())) return;
        const declaredSegmentIds = uniqueList(arrayFromCollection(item.segment_ids)
          .map((id) => safeString(id))
          .filter(Boolean));
        const segmentIds = declaredSegmentIds.length
          ? declaredSegmentIds.filter((id) => candidateSegmentIds.indexOf(id) >= 0)
          : candidateSegmentIds.slice();
        if (!segmentIds.length) return;
        const contribution = {
          candidate_id: candidate.candidate_id,
          role_id: candidate.role_id,
          segment_ids: segmentIds,
          claim,
          evidence_refs: uniqueList(arrayFromCollection(item.evidence_refs)
            .concat(candidate.evidence_refs || [])
            .map((ref) => truncate(ref, 160).trim())
            .filter(Boolean)).slice(0, 24),
          source: entry.source,
        };
        contribution.contribution_id = `gain_${stableDigest(contribution)}`;
        if (seen.has(contribution.contribution_id)) return;
        seen.add(contribution.contribution_id);
        contributions.push(contribution);
      });
      if (contributions.length === contributionCountBefore
          && safeString(candidate.change_summary).trim()) {
        const fallbackContribution = {
          candidate_id: candidate.candidate_id,
          role_id: candidate.role_id,
          segment_ids: candidateSegmentIds.slice(),
          claim: truncate(candidate.change_summary, 500).trim(),
          evidence_refs: uniqueList(arrayFromCollection(candidate.evidence_refs)
            .map((ref) => truncate(ref, 160).trim())
            .filter(Boolean)).slice(0, 24),
          source: "change_summary",
        };
        fallbackContribution.contribution_id = `gain_${stableDigest(fallbackContribution)}`;
        if (!seen.has(fallbackContribution.contribution_id)) {
          seen.add(fallbackContribution.contribution_id);
          contributions.push(fallbackContribution);
        }
      }
    });
    return contributions;
  }

  function buildFusionPlan(judgment, sceneCandidates, draftLedger, mutableSegs) {
    if (!judgment) return null;
    const candidateMap = {};
    arrayFromCollection(sceneCandidates).forEach((candidate) => {
      candidateMap[candidate.candidate_id] = candidate;
    });
    const acceptedJudgments = judgment.candidate_judgments.filter((item) => item.verdict !== "reject");
    const rejectedJudgments = judgment.candidate_judgments.filter((item) => item.verdict === "reject");
    const prohibited = judgment.candidate_judgments.flatMap((item) =>
      item.unsupported_additions.concat(item.verdict !== "accept" ? item.hard_violations.map((violation) => ({
        claim: violation.detail,
        reason: violation.type,
        candidate_ids: [item.candidate_id],
        evidence_refs: violation.evidence_refs,
        segment_ids: violation.segment_ids,
      })) : [])
    );
    const unsupportedKeys = new Set(prohibited.map((item) => stableDigest({
      claim: safeString(item.claim),
      candidate_ids: item.candidate_ids,
    })));
    const permittedAdditions = [];
    acceptedJudgments.forEach((item) => {
      arrayFromCollection(candidateMap[item.candidate_id] && candidateMap[item.candidate_id].proposed_additions)
        .forEach((addition) => {
          const normalized = Object.assign({}, addition, { candidate_ids: [item.candidate_id] });
          if (!unsupportedKeys.has(stableDigest({ claim: safeString(normalized.text), candidate_ids: normalized.candidate_ids }))) {
            permittedAdditions.push(normalized);
          }
        });
    });
    const requiredContributions = buildRequiredContributions(acceptedJudgments, candidateMap);
    const plan = {
      schema: "fusion_plan.v1",
      required_facts: arrayFromCollection(draftLedger && draftLedger.established_facts),
      required_beats: arrayFromCollection(draftLedger && draftLedger.scene_beats),
      must_preserve_constraints: arrayFromCollection(draftLedger && draftLedger.hard_constraints),
      accepted_candidate_ids: acceptedJudgments.map((item) => item.candidate_id),
      rejected_candidates: rejectedJudgments.map((item) => ({
        candidate_id: item.candidate_id,
        reasons: item.rejected_elements.concat(item.unsupported_additions, item.hard_violations),
      })),
      accepted_elements: acceptedJudgments.flatMap((item) =>
        item.accepted_elements.map((element) => Object.assign({}, element, { candidate_ids: [item.candidate_id] }))
      ),
      rejected_elements: judgment.candidate_judgments.flatMap((item) =>
        item.rejected_elements.map((element) => Object.assign({}, element, { candidate_ids: [item.candidate_id] }))
      ),
      required_contributions: requiredContributions,
      consensus_claims: judgment.cross_candidate.consensus,
      complementary_claims: judgment.cross_candidate.complementary,
      conflicts: judgment.cross_candidate.conflicts,
      unresolved_requirements: arrayFromCollection(judgment.unresolved_requirements),
      permitted_additions: permittedAdditions,
      prohibited_additions: prohibited,
      target_scene_arc: judgment.scene_requirements.target_arc,
      target_voice: judgment.scene_requirements.target_voice,
      target_pacing: judgment.scene_requirements.target_pacing,
      expected_segment_coverage: arrayFromCollection(mutableSegs).map((segment) => segment.id),
      semantic_ready: acceptedJudgments.length > 0,
    };
    plan.plan_id = `fusion_${stableDigest(plan)}`;
    return plan;
  }

  function buildComposerCandidatePool(sceneCandidates, judgment, fusionPlan, segments, draftLedger) {
    const acceptedOrder = new Map(arrayFromCollection(fusionPlan && fusionPlan.accepted_candidate_ids)
      .map((id, index) => [id, index]));
    const verdicts = {};
    arrayFromCollection(judgment && judgment.candidate_judgments).forEach((item) => {
      verdicts[item.candidate_id] = item.verdict;
    });
    const ranked = {};
    mutableSegments(segments).forEach((segment) => {
      ranked[segment.id] = arrayFromCollection(sceneCandidates)
        .filter((candidate) => acceptedOrder.has(candidate.candidate_id))
        .sort((left, right) => acceptedOrder.get(left.candidate_id) - acceptedOrder.get(right.candidate_id))
        .map((candidate) => ({
          candidate_id: candidate.candidate_id,
          role_id: candidate.role_id,
          supporting_roles: candidate.supporting_roles,
          rewrite: candidate.segments[segment.id],
          operation: candidate.segment_operations[segment.id] || "replace",
          issues: candidate.addressed_issues || [],
          evidence_refs: candidate.evidence_refs || [],
          retained_beats: candidate.retained_beats || [],
          proposed_additions: candidate.proposed_additions || [],
          change_summary: candidate.change_summary || "",
          judge_verdict: verdicts[candidate.candidate_id],
          revision_round: Number(candidate.revision_round) || 0,
          parent_candidate_id: safeString(candidate.parent_candidate_id),
          input_draft_digest: safeString(candidate.input_draft_digest),
          identical_to_original: replacementCoreText(candidate.segments[segment.id]) === mutableCoreText(segment),
        }))
        .filter((candidate) => candidate.rewrite != null);
    });
    return {
      ranked,
      consensus: {},
      complementary: {},
      conflict: {},
      gap: Object.keys(ranked).filter((segmentId) => !ranked[segmentId].length),
      scene_candidates: sceneCandidates,
      semantic_judgment: judgment,
      fusion_plan: fusionPlan,
      draft_ledger: draftLedger,
    };
  }

  function buildDirectComposerFallbackDirector(reason, segments, draftLedger, semanticJudgment, rejectedPlan) {
    const segmentIds = mutableSegments(segments)
      .filter((segment) => mutableCoreText(segment).trim().length > 0)
      .map((segment) => segment.id);
    const fallbackRequirement = {
      issue_type: "prose_clarity",
      role_id: "style_reader",
      claim: "No judged specialist scene is usable. Reconstruct the complete scene directly from draft_zero, Draft Ledger, runtime context, and output constraints with material improvements to scene architecture, causality, characterization, rhythm, transitions, and ending.",
      segment_ids: segmentIds,
      evidence_refs: [],
    };
    const fallbackJudgment = semanticJudgment || {
      schema: "semantic_judgment.v1",
      candidate_judgments: [],
      cross_candidate: { consensus: [], complementary: [], conflicts: [] },
      unresolved_requirements: [fallbackRequirement],
      scene_requirements: { target_arc: "", target_voice: "", target_pacing: "" },
    };
    const priorPlan = asObject(rejectedPlan);
    const plan = {
      schema: "fusion_plan.v1",
      required_facts: arrayFromCollection(draftLedger && draftLedger.established_facts),
      required_beats: arrayFromCollection(draftLedger && draftLedger.scene_beats),
      must_preserve_constraints: arrayFromCollection(draftLedger && draftLedger.hard_constraints),
      accepted_candidate_ids: [],
      rejected_candidates: arrayFromCollection(priorPlan.rejected_candidates),
      accepted_elements: [],
      rejected_elements: arrayFromCollection(priorPlan.rejected_elements),
      required_contributions: [],
      consensus_claims: [],
      complementary_claims: [],
      conflicts: [],
      unresolved_requirements: [fallbackRequirement],
      permitted_additions: [],
      prohibited_additions: arrayFromCollection(priorPlan.prohibited_additions),
      target_scene_arc: "",
      target_voice: "",
      target_pacing: "",
      expected_segment_coverage: segmentIds,
      semantic_ready: true,
      direct_composer_fallback: true,
      fallback_reason: safeString(reason),
    };
    plan.plan_id = `fusion_${stableDigest(plan)}`;
    const directorResult = buildComposerCandidatePool(
      [], fallbackJudgment, plan, segments, draftLedger
    );
    directorResult.direct_composer_fallback = true;
    directorResult.fallback_reason = safeString(reason);
    return { directorResult, semanticJudgment: fallbackJudgment, fusionPlan: plan };
  }

  function validateComposerSchema(parsed, allowedSegmentIds, mutableSegs) {
    if (!parsed || typeof parsed !== "object") return null;
    if (!Array.isArray(allowedSegmentIds) || !allowedSegmentIds.length) return null;
    let segments = asObject(parsed.segments);
    if (!Object.keys(segments).length && allowedSegmentIds.length === 1
        && typeof parsed[allowedSegmentIds[0]] === "string") {
      segments = { [allowedSegmentIds[0]]: parsed[allowedSegmentIds[0]] };
    }
    const allowed = new Set(allowedSegmentIds);
    const mutableById = {};
    arrayFromCollection(mutableSegs).forEach((segment) => { mutableById[segment.id] = segment; });
    if (allowedSegmentIds.length === 1
        && allowedSegmentIds[0] === SCENE_REWRITE_SEGMENT_ID
        && !Object.prototype.hasOwnProperty.call(segments, SCENE_REWRITE_SEGMENT_ID)) {
      const folded = foldPhysicalSegmentMapIntoScene(mutableById[SCENE_REWRITE_SEGMENT_ID], segments);
      if (folded) segments = { [SCENE_REWRITE_SEGMENT_ID]: folded.scene_text };
    }
    const result = {};
    let count = 0;
    Object.keys(segments).forEach((segId) => {
      if (!allowed.has(segId)) return;
      const text = safeString(segments[segId]);
      if (!validateSceneFrameText(mutableById[segId], text).pass) return;
      if (text || isWhollyMetaArtifactText(mutableFullText(mutableById[segId]))) {
        result[segId] = text;
        count++;
      }
    });
    if (!count || count !== allowed.size) return null;
    return { segments: result };
  }

  function decodePartialJsonString(source, startIndex) {
    const text = safeString(source);
    let value = "";
    let index = startIndex;
    while (index < text.length) {
      const ch = text[index++];
      if (ch === '"') return { value, closed: true };
      if (ch !== "\\") {
        value += ch;
        continue;
      }
      if (index >= text.length) return { value, closed: false };
      const escaped = text[index++];
      if (escaped === "n") value += "\n";
      else if (escaped === "r") value += "\r";
      else if (escaped === "t") value += "\t";
      else if (escaped === "b") value += "\b";
      else if (escaped === "f") value += "\f";
      else if (escaped === "u" && /^[0-9a-fA-F]{4}$/.test(text.slice(index, index + 4))) {
        value += String.fromCharCode(parseInt(text.slice(index, index + 4), 16));
        index += 4;
      } else {
        value += escaped;
      }
    }
    return { value, closed: false };
  }

  function recoverComposerSceneFromDamagedEnvelope(text, allowedSegmentIds, mutableSegs) {
    if (!Array.isArray(allowedSegmentIds) || allowedSegmentIds.length !== 1) return null;
    const segmentId = safeString(allowedSegmentIds[0]);
    const source = safeString(text);
    const keyPattern = new RegExp(`"${segmentId.replace(/[.*+?^${}()|[\\]\\]/g, "\\$&")}"\\s*:\\s*"`, "g");
    let match;
    let recovered = null;
    while ((match = keyPattern.exec(source))) {
      const decoded = decodePartialJsonString(source, match.index + match[0].length);
      if (!decoded.value) continue;
      const mutable = arrayFromCollection(mutableSegs).find((segment) => segment.id === segmentId);
      if (!mutable || !validateSceneFrameText(mutable, decoded.value).pass) continue;
      recovered = { segments: { [segmentId]: decoded.value } };
    }
    return recovered;
  }

  function validateComposerPlainScene(text, allowedSegmentIds, mutableSegs) {
    if (!Array.isArray(allowedSegmentIds) || allowedSegmentIds.length !== 1) return null;
    const raw = safeString(text);
    if (!raw.trim() || /^\s*[\[{]/.test(raw) || /"segments"\s*:/.test(raw)) return null;
    const visible = extractVisibleAssistantOutput(raw);
    if (!visible.text.trim() || visible.ambiguous_unclosed) return null;
    const segmentId = allowedSegmentIds[0];
    const mutable = arrayFromCollection(mutableSegs).find((segment) => segment.id === segmentId);
    if (!mutable || !validateSceneFrameText(mutable, visible.text).pass) return null;
    return { segments: { [segmentId]: visible.text } };
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
      : (role.is_judge
        ? '{"schema":"semantic_judgment.v1","candidate_judgments":[{"candidate_id":"candidate_id","verdict":"accept","preserved_ledger_ids":[],"missing_ledger_ids":[],"unsupported_additions":[],"hard_violations":[],"accepted_elements":[],"rejected_elements":[],"quality_gains":[],"quality_regressions":[]}],"cross_candidate":{"consensus":[],"complementary":[],"conflicts":[]},"unresolved_requirements":[],"scene_requirements":{"target_arc":"","target_voice":"","target_pacing":""}}'
        : (role.is_prover
        ? '{"schema":"semantic_proof.v1","declared_verdict":"pass","fact_checks":[],"beat_checks":[],"constraint_checks":[],"quality_gain_checks":[],"residual_quality_checks":[{"check_id":"mechanics_and_wording","status":"clean","detail":"","evidence_quote":"","segment_ids":[]},{"check_id":"register_and_era","status":"clean","detail":"","evidence_quote":"","segment_ids":[]},{"check_id":"repetition_and_exposition","status":"clean","detail":"","evidence_quote":"","segment_ids":[]},{"check_id":"grounded_psychology_and_relationship","status":"clean","detail":"","evidence_quote":"","segment_ids":[]},{"check_id":"scene_coherence_and_ending","status":"clean","detail":"","evidence_quote":"","segment_ids":[]}],"hard_violations":[],"unsupported_additions":[],"output_contract":{"language_ok":true,"turn_boundary_ok":true,"user_agency_ok":true,"meta_free":true,"format_ok":true},"repair_instructions":[]}'
          : (role.is_composer
            ? '{"segments":{"SEG_ID":"final rewritten text"}}'
            : `{"schema":"scene_rewrite_candidates.v1","role_id":"${role.role_id}","candidates":[{"segments":{"SEG_ID":"full rewritten text"},"evidence_refs":["source_ref"],"retained_beats":[],"proposed_additions":[],"addressed_issues":["issue_code"],"confidence":0.8,"change_summary":"brief"}]}`)));
    return {
      system: `${prompts.system}\n\nREPAIR RESPONSE CONTRACT: Output JSON only. Do not include reasoning, analysis, markdown, or code fences.`,
      user: `${prompts.user}\n\nYour previous response could not be parsed. Return one compact JSON object only, matching this shape exactly:\n${schema}`,
    };
  }

  function structuredRecoveryProfile(profile) {
    const extraBody = parseExtraBody(safeString(profile && profile.extra_body));
    [
      "reasoning_effort", "reasoning", "think", "thinking",
      "response_format", "format", "stream",
    ].forEach((key) => { delete extraBody[key]; });
    if (extraBody.generationConfig && typeof extraBody.generationConfig === "object") {
      delete extraBody.generationConfig.thinkingConfig;
    }
    return Object.assign({}, profile, {
      temperature: 0,
      force_json_response: true,
      reasoning_effort: "none",
      reasoning_budget_tokens: 0,
      extra_body: Object.keys(extraBody).length ? JSON.stringify(extraBody) : "",
    });
  }

  function mixedSegmentDeletionIds(parsed, role, allowedSegmentIds, mutableSegs) {
    if (!role || role.is_input_planner || role.is_judge || role.is_prover || role.is_composer) {
      return [];
    }
    const allowed = new Set(arrayFromCollection(allowedSegmentIds).map((id) => safeString(id)));
    const mutableById = {};
    arrayFromCollection(mutableSegs).forEach((segment) => {
      mutableById[safeString(segment.id)] = segment;
    });
    const ids = [];
    arrayFromCollection(parsed && parsed.candidates).forEach((candidate) => {
      const segments = asObject(candidate && candidate.segments);
      Object.keys(segments).forEach((segmentId) => {
        const original = mutableById[segmentId];
        if (!allowed.has(segmentId) || safeString(segments[segmentId]) || !original) return;
        if (mutableCoreText(original).trim() && !isWhollyMetaArtifactText(mutableFullText(original))) {
          ids.push(segmentId);
        }
      });
    });
    return uniqueList(ids);
  }

  function composerStructuredRecoveryPrompts(mutableSegs, previousFailure) {
    const segmentIds = arrayFromCollection(mutableSegs).map((segment) => safeString(segment.id));
    const shape = {};
    segmentIds.forEach((segmentId) => { shape[segmentId] = "final rewritten text"; });
    const placeholderContract = arrayFromCollection(mutableSegs).map((segment) => ({
      segment_id: safeString(segment.id),
      required_tokens: arrayFromCollection(segment && segment.required_preservation_tokens),
    }));
    const failedResponse = truncate(
      typeof previousFailure === "string" ? previousFailure : JSON.stringify(previousFailure || {}),
      120000
    );
    return {
      system: "You repair one already-written Composer response into valid compact JSON. Output JSON only. Do not analyze, explain, continue, shorten, or rewrite the prose.",
      user: `Recover the complete scene text already present in the failed response below.\n\nAllowed segment IDs:\n${JSON.stringify(segmentIds)}\n\nExact placeholder contract:\n${JSON.stringify(placeholderContract)}\n\nFailed Composer response:\n${failedResponse}\n\nRules:\n- Return every allowed segment ID exactly once and no other key.\n- Preserve the scene wording from the failed response; this is format recovery, not another composition pass.\n- Preserve every required placeholder exactly once in the listed order.\n- If the failed response contains a JSON wrapper, repair only its structure and escaping.\n- Output no reasoning, markdown fence, label, or commentary.\n\nReturn exactly this JSON shape:\n${JSON.stringify({ segments: shape })}`,
    };
  }

  function semanticProverStructuredRecoveryPrompts(
    role,
    previousFailure,
    diagnostics,
    mutableSegs,
    directorInfo
  ) {
    const ledger = asObject(directorInfo && directorInfo.draft_ledger);
    const fusionPlan = asObject(directorInfo && directorInfo.fusion_plan);
    const finalSegments = arrayFromCollection(directorInfo && directorInfo.final_segments)
      .filter((segment) => segment && segment.type === "mutable")
      .map((segment) => ({
        id: safeString(segment.id),
        final_text: safeString(segment.final_text != null ? segment.final_text : segment.text),
      }));
    const originalSegments = arrayFromCollection(mutableSegs).map((segment) => ({
      id: safeString(segment.id),
      original_text: mutableFullText(segment),
    }));
    const targetShape = {
      schema: "semantic_proof.v1",
      declared_verdict: "pass|repair|fail",
      fact_checks: draftLedgerIdsForField(ledger, "established_facts").map((ledgerId) => ({
        ledger_id: ledgerId,
        status: "preserved|missing|contradicted|uncertain",
        detail: "",
        evidence_quote: "",
      })),
      beat_checks: draftLedgerIdsForField(ledger, "scene_beats").map((ledgerId) => ({
        ledger_id: ledgerId,
        status: "preserved|missing|contradicted|uncertain",
        detail: "",
        evidence_quote: "",
      })),
      constraint_checks: draftLedgerIdsForField(ledger, "hard_constraints").map((ledgerId) => ({
        ledger_id: ledgerId,
        status: "satisfied|violated|uncertain",
        detail: "",
        evidence_quote: "",
      })),
      quality_gain_checks: arrayFromCollection(fusionPlan.required_contributions).map((item) => ({
        contribution_id: safeString(item && item.contribution_id),
        status: "realized|missing|regressed",
        detail: "",
        evidence_quote: "",
        segment_ids: arrayFromCollection(item && item.segment_ids),
      })),
      residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
        check_id: checkId,
        status: "clean|issue",
        detail: "",
        evidence_quote: "",
        segment_ids: [],
      })),
      hard_violations: [],
      unsupported_additions: [],
      output_contract: {
        language_ok: true,
        turn_boundary_ok: true,
        user_agency_ok: true,
        meta_free: true,
        format_ok: true,
      },
      repair_instructions: [],
    };
    const recoveryPayload = {
      validation_failures: arrayFromCollection(diagnostics),
      previous_invalid_response: previousFailure && typeof previousFailure === "object"
        ? previousFailure
        : truncate(previousFailure, 50000),
      original_segments: originalSegments,
      final_segments: finalSegments,
      draft_ledger: ledger,
      fusion_plan: fusionPlan,
    };
    return {
      system: "You are the final semantic proof structured-response recovery pass. Recheck the supplied final scene and return one valid semantic_proof.v1 JSON object. Do not write prose, markdown, code fences, or reasoning. An empty preserved fact/beat evidence_quote is valid for negative, background, or implicit continuity preserved by absence or non-contradiction; never invent quoted evidence.",
      user: `Repair the prior Semantic Prover response using this payload:\n${JSON.stringify(recoveryPayload)}\n\nRules:\n- Return every required fact, beat, constraint, contribution, and residual check exactly once using the IDs already present in TARGET SHAPE.\n- Preserved fact/beat evidence_quote must be an exact quote from final_segments. Never quote a user command or ledger wording absent from final prose.\n- response_directives are constraints, not facts or scene beats; their literal wording need not appear in final prose.\n- A realized contribution requires an exact quote in its required final segment that was absent from the corresponding original segment.\n- Every issue requires exact final evidence and segment IDs. Use repair only with segment-scoped repair instructions covering every failure.\n- Output one compact JSON object only.\n\nTARGET SHAPE:\n${JSON.stringify(targetShape)}\n\nRole: ${safeString(role && role.role_id)}`,
    };
  }

  function specialistStructuredRecoveryPrompts(prompts, role, parsed, mixedIds, mutableSegs, allSegments, directorInfo, contextBlock) {
    const allowedIds = arrayFromCollection(mutableSegs).map((segment) => safeString(segment.id));
    const shape = {};
    allowedIds.forEach((segmentId) => { shape[segmentId] = "complete replacement text"; });
    const recoveryPayload = {
      mixed_non_deletable_segment_ids: mixedIds,
      mutable_segments: arrayFromCollection(mutableSegs).map((segment) => ({
        id: safeString(segment.id),
        original: mutableFullText(segment),
      })),
      preserved_segments: arrayFromCollection(allSegments)
        .filter((segment) => segment.type !== "mutable")
        .map((segment) => ({ id: safeString(segment.id), type: safeString(segment.type), kind: safeString(segment.kind) })),
      previous_invalid_response: parsed,
      draft_ledger: asObject(directorInfo && directorInfo.draft_ledger),
      runtime_context: safeString(contextBlock),
    };
    return {
      system: `${prompts.system}\n\nSTRUCTURED RECOVERY: Return one valid scene_rewrite_candidates.v1 JSON object directly. Do not emit analysis, reasoning, markdown, or code fences.`,
      user: `The previous candidate tried to delete mixed meta+narrative segments. Repair that candidate from this payload:\n${JSON.stringify(recoveryPayload)}\n\nRules:\n- Return exactly one complete scene-wide candidate containing every allowed segment ID.\n- For mixed_non_deletable_segment_ids, remove only the model reasoning artifact and fully rewrite all substantive narrative. Never return an empty string for those IDs.\n- Keep valid improvements from the previous response where they remain grounded, but materially rebuild the scene through the ${safeString(role.role_id)} lane.\n- Preserve Draft Ledger facts, POV, identity/reveal state, user agency, and the current turn boundary.\n- Use only these issue values: ${roleAllowedIssues(role.role_id).join(", ")}.\n\nReturn exactly one compact object with this shape and nothing else:\n${JSON.stringify({ schema: "scene_rewrite_candidates.v1", role_id: role.role_id, candidates: [{ segments: shape, evidence_refs: [], retained_beats: [], proposed_additions: [], addressed_issues: ["meta_artifact"], confidence: 0.8, change_summary: "recovered complete scene rewrite" }] })}`,
    };
  }

  function specialistCompactRecoveryPrompts(role, deferredFailure, mutableSegs) {
    const allowedIds = arrayFromCollection(mutableSegs).map((segment) => safeString(segment.id));
    const segmentState = {};
    const shape = {};
    allowedIds.forEach((segmentId) => {
      const segment = arrayFromCollection(mutableSegs).find((item) => safeString(item.id) === segmentId);
      segmentState[segmentId] = {
        state: isWhollyMetaArtifactText(mutableFullText(segment))
          ? "wholly_meta"
          : "substantive",
        original_text: mutableFullText(segment),
      };
      shape[segmentId] = "complete replacement text";
    });
    const parsedPayload = deferredFailure && deferredFailure.recovery_payload;
    const rawSource = parsedPayload && typeof parsedPayload === "object"
      ? JSON.stringify(parsedPayload)
      : safeString(deferredFailure && deferredFailure.recovery_source);
    const laneInstruction = role.role_id === "style_reader"
      ? "Preserve usable supplied improvements, then complete the entire scene through the style lane without adding new story events, facts, or dialogue."
      : "Preserve usable supplied improvements, then complete the entire scene through the character lane without adding new relationships, knowledge, emotions, events, or dialogue.";
    return {
      system: "You are the complete-scene structured recovery worker for one Recomposer specialist lane. Return a complete usable scene candidate, not advice, analysis, patches, or a partial formatting repair.",
      user: `Recover the existing ${safeString(role.role_id)} attempt into one complete scene-wide candidate.\n\nAllowed segment states and original text:\n${JSON.stringify(segmentState)}\n\nSOURCE:\n${truncate(rawSource, 120000)}\n\nRules:\n- Preserve usable candidate prose already present in SOURCE.\n- ${laneInstruction}\n- The segments object must contain every allowed segment ID exactly once. Reconstruct missing segments from original_text through this role's lane; never rely on implicit passthrough.\n- The result must read as one continuous complete version of the same turn from first mutable segment to last.\n- An empty string is allowed only when its segment state is wholly_meta and addressed_issues includes meta_artifact.\n- If a complete material scene candidate cannot be recovered, return an empty candidates array.\n- Use only these issue values: ${roleAllowedIssues(role.role_id).join(", ")}.\n- Output JSON only, with no markdown, code fence, reasoning, or commentary.\n\nTarget shape:\n${JSON.stringify({ schema: "scene_rewrite_candidates.v1", role_id: role.role_id, candidates: [{ segments: shape, evidence_refs: [], retained_beats: [], proposed_additions: [], addressed_issues: [], confidence: 0.8, change_summary: "recovered complete scene rewrite" }] })}`,
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

  function setFinalTraceState(trace, enhanced, state, reason, assessment) {
    const evidence = asObject(assessment);
    const materialRewrite = evidence.material_rewrite === true;
    const semanticVerified = evidence.semantic_verified === "degraded"
      ? "degraded"
      : (evidence.semantic_verified === true
        ? "passed"
        : (evidence.semantic_verified === false ? "failed" : "not_run"));
    const qualityPreferred = evidence.quality_preferred === true
      ? "passed"
      : (evidence.quality_preferred === false ? "failed" : "not_run");
    trace.final.material_rewrite = materialRewrite;
    trace.final.semantic_verified = semanticVerified;
    trace.final.quality_preferred = qualityPreferred;
    trace.final.enhanced = !!enhanced && materialRewrite
      && (semanticVerified === "passed" || semanticVerified === "degraded");
    trace.final.reason = safeString(reason);
    trace.summary.material_rewrite = materialRewrite;
    trace.summary.semantic_verified = semanticVerified;
    trace.summary.quality_preferred = qualityPreferred;
    trace.summary.final_state = safeString(state);
    trace.summary.final_reason = safeString(reason);
  }

  function refreshTraceSummary(trace, segSummary, composerResult) {
    const specialistTraceEntries = (trace.roles || []).filter(
      (entry) => entry.stage === "output" && entry.role_id !== COMPOSER_ROLE_ID
    );
    const specialistRoleIds = uniqueList(specialistTraceEntries.map((entry) => entry.role_id));
    const successfulRoles = specialistRoleIds.filter((roleId) =>
      specialistTraceEntries.some((entry) => entry.role_id === roleId && entry.status === "fulfilled")
    ).length;
    trace.summary = Object.assign({}, trace.summary || {}, {
      specialist_calls: specialistRoleIds.length,
      specialist_http_calls: specialistTraceEntries.reduce(
        (sum, entry) => sum + Math.max(0, Number(entry.http_attempts) || 0),
        0
      ),
      successful_roles: successfulRoles,
      candidate_count: Number(trace.candidates && trace.candidates.total) || 0,
      composer_state: trace.composer.status || (composerResult ? "fulfilled" : "not_run"),
      changed_segment_count: 0,
      material_changed_segment_count: 0,
      unchanged_segment_count: Number(segSummary && segSummary.mutable) || 0,
    });
  }

  function classifyAppliedOutput(assembled, semanticProof) {
    const changedSegments = (assembled && assembled.finalSegments || []).filter((segment) =>
      segment.type === "mutable"
      && safeString(segment.final_text) !== safeString(segment.original_text)
    );
    if (assembled && changedSegments.length > 0) {
      const proofPassed = semanticProof && semanticProof.verdict === "pass";
      const proofDegraded = !proofPassed;
      const qualityChecks = arrayFromCollection(semanticProof && semanticProof.quality_gain_checks);
      const residualChecks = arrayFromCollection(semanticProof && semanticProof.residual_quality_checks);
      const qualityPreferred = residualChecks.length
        ? qualityChecks.every((item) => item.status === "realized")
          && residualChecks.every((item) => item.status === "clean")
        : null;
      return {
        enhanced: true,
        state: proofPassed ? "enhanced" : "enhanced_degraded",
        reason: proofPassed
          ? "composer_changed_scene_semantic_proof_passed"
          : "composer_changed_scene_returned_with_semantic_debt",
        material_rewrite: true,
        semantic_verified: proofPassed ? true : "degraded",
        quality_preferred: proofDegraded ? false : qualityPreferred,
      };
    }
    return {
      enhanced: false,
      state: "no_changed_scene",
      reason: "no_changed_scene_generated",
      material_rewrite: false,
      semantic_verified: null,
      quality_preferred: null,
    };
  }

  async function callRole(role, profile, mutableSegs, contextBlock, allSegments, abortSignal, trace, directorInfo, queuedAt, runtimeControl) {
    profile = expandRewriteOutputBudget(role, profile, mutableSegs);
    const allowedSegIds = mutableSegs ? mutableSegs.map((s) => s.id) : (role.is_composer ? allSegments.filter((s) => s.type === "mutable").map((s) => s.id) : []);
    const prompts = runtimeControl && runtimeControl.initialPrompts
      ? runtimeControl.initialPrompts
      : buildRolePrompt(role, profile, role.is_composer ? mutableSegs : mutableSegs, contextBlock, allSegments, directorInfo);
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
      const proverReserve = role.is_prover
        ? 0
        : Math.max(0, Number(traceBudget.prover_attempt_reserved) || 0);
      const composerReserve = role.is_composer || role.is_prover
        ? 0
        : Math.max(0, Number(traceBudget.composer_attempt_reserved) || 0);
      const judgeReserve = role.is_composer || role.is_judge || role.is_prover
        ? 0
        : Math.max(0, Number(traceBudget.judge_attempt_reserved) || 0);
      const downstreamReserve = proverReserve + composerReserve + judgeReserve;
      const primaryReserve = !role.is_input_planner && !role.is_composer
          && !role.is_judge && !role.is_prover && kind !== "primary"
        ? Math.max(0, Number(traceBudget.specialist_primary_remaining) || 0)
        : 0;
      if (!role.is_prover
          && traceBudget.http_attempt_used >= traceBudget.http_attempt_max - downstreamReserve - primaryReserve) {
        return {
          allowed: false,
          reason: primaryReserve > 0 ? "specialist_primary_and_downstream_reserved" : "downstream_attempt_reserved",
        };
      }
      if (traceBudget.http_attempt_used >= traceBudget.http_attempt_max) {
        traceBudget.http_stopped_reason = "turn_attempt_budget_exhausted";
        return { allowed: false, reason: traceBudget.http_stopped_reason };
      }
      traceBudget.http_attempt_used += 1;
      if (role.is_input_planner) {
        traceBudget.input_attempt_used = Math.max(0, Number(traceBudget.input_attempt_used) || 0) + 1;
      } else if (role.is_prover) {
        traceBudget.prover_attempt_used = Math.max(0, Number(traceBudget.prover_attempt_used) || 0) + 1;
        traceBudget.prover_attempt_reserved = 0;
      } else if (role.is_composer) {
        traceBudget.composer_attempt_used = Math.max(0, Number(traceBudget.composer_attempt_used) || 0) + 1;
        traceBudget.composer_attempt_reserved = 0;
      } else if (role.is_judge) {
        traceBudget.judge_attempt_used = Math.max(0, Number(traceBudget.judge_attempt_used) || 0) + 1;
        traceBudget.judge_attempt_reserved = 0;
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
        endpoint_group: profile && profile.endpoint ? executionGroupKey(profile) : "",
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
          endpoint_group: executionGroupKey(activeProfile),
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
          endpoint_group: executionGroupKey(activeProfile),
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
      let validationDiagnostics = [];
      try {
        const requestProfile = role.is_input_planner || role.is_judge || role.is_prover
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
        let validated;
        if (role.is_composer && !parsed) {
          const recoveredEnvelope = recoverComposerSceneFromDamagedEnvelope(
            content || structuredRaw,
            allowedSegIds,
            mutableSegs
          );
          validated = recoveredEnvelope
            || validateComposerPlainScene(content || structuredRaw, allowedSegIds, mutableSegs);
          if (validated) {
            structuredTransport = recoveredEnvelope
              ? "composer_damaged_envelope_recovered"
              : "composer_plain_scene";
          }
          else throw new Error("json_parse_failed");
        } else if (!parsed) {
          throw new Error("json_parse_failed");
        } else if (role.is_input_planner) {
          validated = validateTurnContractFragment(
            parsed,
            role.role_id,
            directorInfo && directorInfo.context_manifest
          );
        } else if (role.is_judge) {
          validated = validateSemanticJudgment(
            parsed,
            directorInfo && directorInfo.scene_candidates,
            directorInfo && directorInfo.draft_ledger
          );
        } else if (role.is_prover) {
          validated = validateSemanticProof(
            parsed,
            directorInfo && directorInfo.draft_ledger,
            mutableSegs,
            directorInfo && directorInfo.final_segments,
            directorInfo && directorInfo.fusion_plan,
            validationDiagnostics
          );
        } else if (role.is_composer) {
          validated = validateComposerSchema(parsed, allowedSegIds, mutableSegs);
        } else {
          const candidateValidation = validateCandidateSchemaDetailed(
            parsed,
            role.role_id,
            allowedSegIds,
            mutableSegs
          );
          validated = candidateValidation.value;
          validationDiagnostics = candidateValidation.diagnostics;
          const incompleteCandidateIndexes = new Set(
            validationDiagnostics
              .filter((diagnostic) =>
                diagnostic.code === "candidate_missing_required_segments"
                || diagnostic.code === "candidate_empty_required_segments"
                || diagnostic.code === "candidate_contains_foreign_segments"
              )
              .map((diagnostic) => diagnostic.candidate_index)
              .filter((index) => index >= 0)
          );
          trace.candidates.incomplete_rejected_count += incompleteCandidateIndexes.size;
          trace.candidates.normalized_field_count += validationDiagnostics.length;
        }
        if (!validated) {
          const schemaError = new Error("schema_validation_failed");
          const mixedIds = mixedSegmentDeletionIds(parsed, role, allowedSegIds, mutableSegs);
          if (mixedIds.length) {
            schemaError.structured_recovery_kind = "specialist_mixed_segment_recovery";
            schemaError.structured_recovery_segment_ids = mixedIds;
            schemaError.structured_recovery_payload = parsed;
          }
          schemaError.structured_recovery_payload = schemaError.structured_recovery_payload || parsed;
          schemaError.validation_diagnostics = validationDiagnostics;
          throw schemaError;
        }
        const specialistNoCandidate = !role.is_input_planner
          && !role.is_judge
          && !role.is_prover
          && !role.is_composer
          && validated.candidates.length === 0;
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          endpoint_group: executionGroupKey(activeProfile),
          model: activeProfile.model,
          status: specialistNoCandidate ? "no_candidate" : "fulfilled",
          started_at: attemptStarted,
          ended_at: Date.now(),
          structured_transport: structuredPayload ? "tool_call_used" : structuredTransport,
          validation_diagnostics: validationDiagnostics,
        });
        return validated;
      } catch (err) {
        lastError = safeString(err && err.message);
        const classification = classifyRoleError(err, abortSignal);
        if (err && err.structured_recovery_kind) {
          classification.recovery_kind = safeString(err.structured_recovery_kind);
          classification.recovery_segment_ids = arrayFromCollection(err.structured_recovery_segment_ids);
          classification.recovery_payload = err.structured_recovery_payload;
        }
        if (err && err.structured_recovery_payload) {
          classification.recovery_payload = err.structured_recovery_payload;
        }
        classification.recovery_source = truncate(failureSource, 120000);
        classification.validation_diagnostics = arrayFromCollection(err && err.validation_diagnostics);
        lastErrorClass = classification.code;
        attemptTrace.push({
          kind,
          provider: activeProfile.provider,
          endpoint_group: executionGroupKey(activeProfile),
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
          validation_diagnostics: classification.validation_diagnostics,
        });
        return { __error: true, classification };
      }
    }

    let outcome = await executeAttempt(
      profile,
      prompts,
      safeString(runtimeControl && runtimeControl.initialAttemptKind) || "primary"
    );
    const allowRetry = !runtimeControl || runtimeControl.allowRetry !== false;
    const allowFallback = !runtimeControl || runtimeControl.allowFallback !== false;
    const retryErrorCodes = arrayFromCollection(runtimeControl && runtimeControl.retryErrorCodes)
      .map((code) => safeString(code))
      .filter(Boolean);
    if (outcome && outcome.__error && outcome.classification.retryable && allowRetry
        && (!retryErrorCodes.length || retryErrorCodes.indexOf(outcome.classification.code) >= 0)
        && !role.is_input_planner && !(abortSignal && abortSignal.aborted)
        && (!runtimeControl || typeof runtimeControl.canContinue !== "function" || runtimeControl.canContinue())) {
      retryCount++;
      if (role.is_prover) {
        outcome = await executeAttempt(
          structuredRecoveryProfile(profile),
          semanticProverStructuredRecoveryPrompts(
            role,
            outcome.classification.recovery_payload || outcome.classification.recovery_source,
            outcome.classification.validation_diagnostics,
            mutableSegs,
            directorInfo
          ),
          "semantic_prover_json_recovery"
        );
      } else if (role.is_composer
          && /^(?:json_parse_failed|schema_validation_failed|reasoning_only_response)$/.test(outcome.classification.code)) {
        outcome = await executeAttempt(
          structuredRecoveryProfile(profile),
          composerStructuredRecoveryPrompts(
            mutableSegs,
            outcome.classification.recovery_source || outcome.classification.recovery_payload
          ),
          "composer_json_recovery"
        );
      } else if (outcome.classification.recovery_kind === "specialist_mixed_segment_recovery") {
        outcome = await executeAttempt(
          structuredRecoveryProfile(profile),
          specialistStructuredRecoveryPrompts(
            prompts,
            role,
            outcome.classification.recovery_payload,
            outcome.classification.recovery_segment_ids,
            mutableSegs,
            allSegments,
            directorInfo,
            contextBlock
          ),
          "specialist_mixed_segment_recovery"
        );
      } else {
        const retryPrompts = outcome.classification.repair ? jsonRepairPrompts(prompts, role) : prompts;
        outcome = await executeAttempt(
          profile,
          retryPrompts,
          outcome.classification.repair ? "repair_retry" : "transient_retry"
        );
      }
    }

    if (outcome && outcome.__error && allowFallback && fallbackProfile
        && !(abortSignal && abortSignal.aborted)
        && (!runtimeControl || typeof runtimeControl.canContinue !== "function" || runtimeControl.canContinue())) {
      usedFallback = true;
      outcome = await executeAttempt(fallbackProfile, jsonRepairPrompts(prompts, role), "fallback");
    }

    if (outcome && !outcome.__error) {
      const finalAttempt = attemptTrace[attemptTrace.length - 1] || {};
      const allValidationDiagnostics = attemptTrace.flatMap((attempt) =>
        arrayFromCollection(attempt && attempt.validation_diagnostics)
      );
      const specialistNoCandidate = !role.is_input_planner
        && !role.is_judge
        && !role.is_prover
        && !role.is_composer
        && outcome.candidates.length === 0;
      traceRole(trace, {
        role_id: role.role_id,
        stage: role.stage || "output",
        provider: finalAttempt.provider || profile.provider,
        endpoint_group: finalAttempt.endpoint_group || executionGroupKey(profile),
        model: finalAttempt.model || profile.model,
        status: specialistNoCandidate ? "no_candidate" : "fulfilled",
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
          : (role.is_judge
            ? outcome.candidate_judgments.length
            : (role.is_prover
              ? 1
              : (role.is_composer
                ? Object.keys(outcome.segments).length
                : outcome.candidates.length))),
        request_overrides: requestOverrides,
        validation_diagnostics: allValidationDiagnostics,
      });
      return outcome;
    }

    const deferStructuredRecovery = runtimeControl
      && runtimeControl.deferStructuredRecovery === true
      && !role.is_input_planner
      && !role.is_judge
      && !role.is_composer
      && !role.is_prover
      && outcome
      && outcome.__error
      && /^(?:json_parse_failed|schema_validation_failed|reasoning_only_response)$/.test(
        outcome.classification.code
      );
    if (deferStructuredRecovery) {
      traceRole(trace, {
        role_id: role.role_id,
        stage: role.stage || "output",
        provider: profile.provider,
        endpoint_group: executionGroupKey(profile),
        model: profile.model,
        status: "recovery_queued",
        queued_at: queuedTimestamp,
        started_at: startedAt,
        ended_at: Date.now(),
        elapsed_ms: Date.now() - startedAt,
        retry: retryCount,
        fallback: false,
        http_attempts: httpAttempts,
        attempts: attemptTrace,
        request_overrides: requestOverrides,
        error_class: lastErrorClass,
        error: lastError,
        validation_diagnostics: outcome.classification.validation_diagnostics,
      });
      return {
        __deferred_specialist_recovery: true,
        role_id: role.role_id,
        error_class: outcome.classification.code,
        recovery_source: outcome.classification.recovery_source,
        recovery_payload: outcome.classification.recovery_payload || null,
        validation_diagnostics: outcome.classification.validation_diagnostics || [],
      };
    }

    const finalProfile = usedFallback && fallbackProfile ? fallbackProfile : profile;
    const allValidationDiagnostics = attemptTrace.flatMap((attempt) =>
      arrayFromCollection(attempt && attempt.validation_diagnostics)
    );
    traceRole(trace, {
      role_id: role.role_id,
      stage: role.stage || "output",
      provider: finalProfile.provider,
      endpoint_group: executionGroupKey(finalProfile),
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
      validation_diagnostics: allValidationDiagnostics,
    });
    return null;
  }

  /* ── Adaptive role selection ───────────────────────────── */

  function detectSceneSignals(segments, context, draftLedger) {
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
    const metaPattern = /(?:<\/?(?:thoughts?|analysis|thinking|think)\b[^>]*>|(?:^|\n)\s*#{0,3}\s*(?:approved|response|processing\b)|(?:^|\n)\s*(?:as an ai|language model|i cannot|i apologize|i'm sorry|firstly|secondly|in summary|to summarize|결론적으로|요약하면|죄송합니다|먼저|둘째로))/gim;
    const metaCount = (mutableText.match(metaPattern) || []).length;

    // Context availability
    const hasLorebook = !!(context && context.lorebook);
    const hasMemory = !!(context && context.memory);
    const hasCharacter = !!(context && context.character);
    const hasBoundaryConstraints = arrayFromCollection(draftLedger && draftLedger.secrets_and_reveal).length > 0
      || arrayFromCollection(draftLedger && draftLedger.speaker_and_pov).length > 0
      || arrayFromCollection(draftLedger && draftLedger.user_owned_decisions).length > 0;
    const archiveContext = asObject(context && context.archive_center_context);
    const archiveFeatures = asObject(archiveContext.feature_status);
    const archiveFeatureCount = (key) => clampNumber(
      asObject(archiveFeatures[key]).selected_count,
      0, 100000, 0
    );

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
    if (hasBoundaryConstraints) {
      signals.push({
        id: "perspective_boundary_constraints",
        severity: "high",
        metrics: {
          secrets: arrayFromCollection(draftLedger && draftLedger.secrets_and_reveal).length,
          pov: arrayFromCollection(draftLedger && draftLedger.speaker_and_pov).length,
        },
      });
    }
    if (archiveContext.contract_version === ARCHIVE_CENTER_ENHANCEMENT_CONTRACT) {
      signals.push({
        id: "archive_center_available",
        severity: "medium",
        metrics: { lanes: arrayFromCollection(archiveContext.lanes).length },
      });
    }
    if (archiveFeatureCount("subjective_memory") > 0) {
      signals.push({
        id: "archive_subjective_memory",
        severity: "high",
        metrics: { count: archiveFeatureCount("subjective_memory") },
      });
    }
    if (archiveFeatureCount("protected_secret") > 0) {
      signals.push({
        id: "archive_protected_secret",
        severity: "high",
        metrics: { count: archiveFeatureCount("protected_secret") },
      });
    }
    if (archiveFeatureCount("supervisor_guidance") > 0) {
      signals.push({
        id: "archive_supervisor_guidance",
        severity: "high",
        metrics: { count: archiveFeatureCount("supervisor_guidance") },
      });
    }
    if (archiveFeatureCount("critic_curated_evidence") > 0) {
      signals.push({
        id: "archive_critic_evidence",
        severity: "medium",
        metrics: { count: archiveFeatureCount("critic_curated_evidence") },
      });
    }

    return signals;
  }

  const SIGNAL_ROLE_MAP = Object.freeze({
    dialogue_heavy: ["character_reader"],
    speaker_switch: ["character_reader"],
    mechanical_artifact: ["perspective_boundary_rewriter"],
    list_structure: ["style_reader"],
    repetition_detected: ["style_reader"],
    long_scene: ["plot_continuity_reader", "style_reader"],
    dense_segments: ["style_reader"],
    lorebook_available: ["plot_continuity_reader"],
    memory_available: ["plot_continuity_reader"],
    character_available: ["character_reader"],
    perspective_boundary_constraints: ["perspective_boundary_rewriter"],
    archive_center_available: ["character_reader", "plot_continuity_reader"],
    archive_subjective_memory: ["character_reader", "plot_continuity_reader"],
    archive_protected_secret: ["perspective_boundary_rewriter"],
    archive_supervisor_guidance: ["plot_continuity_reader", "style_reader"],
    archive_critic_evidence: ["plot_continuity_reader"],
  });

  function selectRoles(roles, signals, preset, settings) {
    const presetDef = PRESETS.find((p) => p.id === preset) || PRESETS[1];
    const baseRoleIds = new Set(presetDef.roles);
    const signalIds = signals.map((s) => s.id);
    const specialistLimit = OUTPUT_SPECIALIST_LIMIT[presetDef.id] || OUTPUT_SPECIALIST_LIMIT.balanced;
    const baseScores = {
      character_reader: 55,
      plot_continuity_reader: 50,
      style_reader: 45,
      perspective_boundary_rewriter: 60,
    };
    const severityScores = { high: 100, medium: 45, low: 15 };
    const candidates = [];
    let composer = null;
    let judge = null;
    let prover = null;
    const skipReasons = [];
    const selectReasons = [];

    roles.forEach((role) => {
      const profile = settings.role_profiles[role.role_id];
      if (role.is_input_planner) {
        skipReasons.push({ role_id: role.role_id, reason: "input_stage_only" });
        return;
      }
      if (role.is_judge) {
        if (!baseRoleIds.has(role.role_id)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_in_preset" });
        } else if (!profile || !profile.enabled) {
          skipReasons.push({ role_id: role.role_id, reason: "disabled" });
        } else if (!isProfileConfigured(profile)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_configured" });
        } else {
          judge = role;
        }
        return;
      }
      if (role.is_prover) {
        if (!baseRoleIds.has(role.role_id)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_in_preset" });
        } else if (!profile || !profile.enabled) {
          skipReasons.push({ role_id: role.role_id, reason: "disabled" });
        } else if (!isProfileConfigured(profile)) {
          skipReasons.push({ role_id: role.role_id, reason: "not_configured" });
        } else {
          prover = role;
        }
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
        required: baseRoleIds.has(role.role_id),
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
    if (judge) {
      selectReasons.push({ role_id: judge.role_id, reason: "semantic_judge_reserved" });
    }
    if (prover) {
      selectReasons.push({ role_id: prover.role_id, reason: "semantic_prover_reserved" });
    }

    return {
      roles: chosen.map((item) => item.role)
        .concat(judge ? [judge] : [])
        .concat(composer ? [composer] : [])
        .concat(prover ? [prover] : []),
      skipReasons,
      selectReasons,
      signals: signalIds,
    };
  }

  /* ── Rewrite materiality ─────────── */

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
      material: !originalMetaOnly
        && (shortSegment
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

  function composerMaterialityScore(assessment) {
    const details = arrayFromCollection(assessment && assessment.details);
    return (Math.max(0, Number(assessment && assessment.material_substantive_count) || 0) * 1000)
      + details.reduce((sum, item) => sum
        + Math.max(0, Number(item.changed_span_ratio) || 0) * 100
        + Math.max(0, 1 - (Number(item.sequence_similarity) || 0)) * 10, 0);
  }

  /* ── Composer ──────────────────────────────────────────── */

  function candidateEligibleForApplication(candidate) {
    if (!candidate || candidate.identical_to_original) return false;
    return safeString(candidate.judge_verdict) !== "reject";
  }

  function composerTokenPlan(mutableSegs, profile) {
    const mutableTotalChars = (mutableSegs || []).reduce((sum, segment) => sum + safeString(segment.text).length, 0);
    const estimatedOutputTokens = Math.min(32000, Math.max(1024, Math.ceil(mutableTotalChars * 1.5) + 1024));
    const configuredOutputTokens = clampNumber(profile && profile.max_output_tokens, 1024, 32000, 4096);
    return {
      mutable_total_chars: mutableTotalChars,
      estimated_output_tokens: estimatedOutputTokens,
      requested_output_tokens: Math.max(configuredOutputTokens, estimatedOutputTokens),
    };
  }

  function expandRewriteOutputBudget(role, profile, mutableSegs) {
    if (!role || role.is_input_planner || role.is_judge || role.is_prover) return profile;
    const plan = composerTokenPlan(mutableSegs, profile);
    return Object.assign({}, profile, {
      max_output_tokens: plan.requested_output_tokens,
    });
  }

  async function runComposer(
    composerRole,
    profile,
    segments,
    directorResult,
    contextBlock,
    abortSignal,
    trace,
    mutableSegs,
    completionWait,
    semanticProofRepair
  ) {
    const previousComposerSegments = asObject(
      semanticProofRepair && semanticProofRepair.previous_segments
    );
    const workingSegments = Object.keys(previousComposerSegments).length
      ? materializeCandidateSegments(segments, { segments: previousComposerSegments })
      : segments;
    const workingMutableSegs = mutableSegments(workingSegments).filter((segment) =>
      mutableCoreText(segment).trim().length > 0
    );
    const candidateBundles = {};
    Object.keys(directorResult.ranked).forEach((segId) => {
      const ranked = directorResult.ranked[segId];
      candidateBundles[segId] = ranked.filter(candidateEligibleForApplication).slice(0, 3).map((c) => ({
        candidate_id: c.candidate_id,
        role: c.role_id,
        supporting_roles: c.supporting_roles || [c.role_id],
        duplicate_count: c.duplicate_count || 0,
        judge_verdict: c.judge_verdict,
        change_summary: c.change_summary || "",
        evidence_refs: c.evidence_refs || [],
        retained_beats: c.retained_beats || [],
        proposed_additions: c.proposed_additions || [],
        operation: c.operation || "replace",
        revision_round: c.revision_round || 0,
        parent_candidate_id: c.parent_candidate_id || "",
        rewrite: c.rewrite,
      }));
    });
    const directorInfo = {
      candidateBundles,
      semantic_judgment: directorResult.semantic_judgment,
      fusion_plan: directorResult.fusion_plan,
      draft_ledger: directorResult.draft_ledger || null,
      revision_lineage: directorResult.revision_lineage || [],
      last_valid_candidate: directorResult.last_valid_candidate || null,
      semantic_proof_repair: semanticProofRepair || null,
    };
    const tokenPlan = composerTokenPlan(workingMutableSegs, profile);
    const plainSceneOutput = workingMutableSegs.length === 1
      && workingMutableSegs[0].id === SCENE_REWRITE_SEGMENT_ID;
    directorInfo.composer_plain_scene = plainSceneOutput;
    const composerProfile = Object.assign({}, profile, {
      max_output_tokens: tokenPlan.requested_output_tokens,
      force_json_response: plainSceneOutput ? false : !!profile.force_json_response,
    });
    trace.composer.estimated_output_tokens = tokenPlan.estimated_output_tokens;
    trace.composer.requested_output_tokens = tokenPlan.requested_output_tokens;
    trace.composer.mutable_total_chars = tokenPlan.mutable_total_chars;
    const composerStarted = Date.now();
    let result = await callRole(
      composerRole, composerProfile, workingMutableSegs, contextBlock, workingSegments,
      abortSignal, trace, directorInfo, composerStarted,
      {
        allowRetry: true,
        allowFallback: true,
        retryErrorCodes: [
          "json_parse_failed",
          "schema_validation_failed",
          "reasoning_only_response",
        ],
        completionWait: !!completionWait,
      }
    );
    let materiality = result ? assessComposerMateriality(workingMutableSegs, result) : null;
    trace.composer.materiality = materiality;
    if (!semanticProofRepair) trace.composer.semantic_retry = 0;
    if (!semanticProofRepair && result && materiality && !materiality.pass
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
        system_prompt: `${safeString(composerProfile.system_prompt || composerRole.default_prompt)}\n\nThe previous composition did not yet achieve the requested recomposition strength. Recompose every flagged prose segment with materially different sentence architecture and scene execution while preserving facts. Then reread the assembled scene and repair awkward wording, typos, register or era mismatch, repeated explanation, unsupported psychological interpretation, transitions, and ending cadence.`,
      });
      const retrySegments = materializeCandidateSegments(
        workingSegments,
        { segments: result.segments }
      );
      const retryMutableSegs = mutableSegments(retrySegments).filter((segment) =>
        mutableCoreText(segment).trim().length > 0
      );
      const retryResult = await callRole(
        composerRole,
        retryProfile,
        retryMutableSegs,
        contextBlock,
        retrySegments,
        abortSignal,
        trace,
        retryDirectorInfo,
        Date.now(),
        {
          allowRetry: false,
          allowFallback: false,
          completionWait: !!completionWait,
        }
      );
      if (retryResult) {
        const retryMateriality = assessComposerMateriality(retryMutableSegs, retryResult);
        if (!result || composerMaterialityScore(retryMateriality) > composerMaterialityScore(materiality)) {
          result = retryResult;
          materiality = retryMateriality;
          trace.composer.semantic_retry_adopted = true;
        } else {
          trace.composer.semantic_retry_adopted = false;
          trace.composer.semantic_retry_retained_initial = true;
        }
        trace.composer.materiality = materiality;
      }
    }
    const elapsed = Date.now() - composerStarted;
    trace.composer.used = !!result;
    trace.composer.status = result ? "fulfilled" : "failed";
    trace.composer.elapsed_ms = Math.max(0, Number(trace.composer.elapsed_ms) || 0) + elapsed;
    if (semanticProofRepair && semanticProofRepair.preproof_recovery === true) {
      trace.composer.preproof_recovery_attempted = true;
    } else if (semanticProofRepair) {
      trace.composer.proof_repair_attempted = true;
    }
    return result;
  }

  async function runPreProofComposerRecovery(options) {
    const reasonCodes = uniqueList(arrayFromCollection(options && options.reason_codes)
      .map((item) => safeString(item).trim())
      .filter(Boolean));
    const deadline = options && options.deadline;
    const trace = options && options.trace;
    if (!options || !trace || !deadline || deadline.check()
        || trace.composer.preproof_recovery_attempted === true) {
      return null;
    }
    const currentSegments = asObject(options.composer_result && options.composer_result.segments);
    trace.composer.preproof_recovery_attempted = true;
    trace.composer.preproof_recovery_reason = reasonCodes.join(",") || "composition_rejected";
    traceTimeline(trace, "preproof_composer_recovery_start");
    const composerScope = createPipelineStageScope(
      deadline, "preproof_composer_recovery", trace
    );
    let recoveredComposerResult;
    try {
      recoveredComposerResult = await runComposer(
        options.composer_role,
        options.composer_profile,
        options.segments,
        options.director_result,
        options.context_block,
        composerScope.signal,
        trace,
        options.mutable_segments,
        !!deadline.completion_wait,
        {
          preproof_recovery: true,
          previous_segments: currentSegments,
          reason_codes: reasonCodes,
          repair_instructions: [{
            segment_id: SCENE_REWRITE_SEGMENT_ID,
            instruction: "Rebuild the complete scene with materially different scene execution. Remove duplicate prose and malformed model artifacts, preserve grounded facts and beats, and return only the finished scene. Host artifacts are restored mechanically after writing.",
            evidence_refs: reasonCodes,
            prohibited: [
              "near-copy",
              "duplicate block",
              "commentary or analysis",
            ],
          }],
        }
      );
    } finally {
      composerScope.cancel();
    }
    traceTimeline(trace, "preproof_composer_recovery_done");
    if (!recoveredComposerResult) {
      trace.composer.preproof_recovery_succeeded = false;
      return null;
    }
    const assembled = assembleSceneRewriteFrame(options.rewrite_frame, recoveredComposerResult);
    const verification = assembled.frame_error
      ? { pass: false, errors: [`scene_frame_restore_failed:${assembled.frame_error}`] }
      : verifyOutput(
          options.physical_segments,
          assembled.physicalFinalSegments,
          assembled.output,
          options.original_text,
          options.settings
        );
    trace.composer.preproof_recovery_succeeded = !!(
      verification.pass && assembled.changed
    );
    if (!trace.composer.preproof_recovery_succeeded) return null;
    return { composerResult: recoveredComposerResult, assembled, verification };
  }

  function summarizeSemanticProof(trace, proof, attemptLabel) {
    const target = trace.semantic_prover;
    const proverTrace = arrayFromCollection(trace.roles)
      .filter((entry) => safeString(entry && entry.role_id) === PROVER_ROLE_ID)
      .slice(-1)[0];
    const recoveryAttempted = arrayFromCollection(proverTrace && proverTrace.attempts)
      .some((attempt) => safeString(attempt && attempt.kind) === "semantic_prover_json_recovery");
    target.attempts = Math.max(0, Number(target.attempts) || 0) + 1;
    target.status = proof ? "fulfilled" : "failed";
    target.verdict = proof ? proof.verdict : "failed";
    target.attempt_label = safeString(attemptLabel);
    target.facts_missing = proof
      ? proof.fact_checks.filter((item) => item.status !== "preserved").length
      : 0;
    target.facts_contradicted = proof
      ? proof.fact_checks.filter((item) => item.status === "contradicted").length
      : 0;
    target.beats_missing = proof
      ? proof.beat_checks.filter((item) => item.status !== "preserved").length
      : 0;
    target.constraints_violated = proof
      ? proof.constraint_checks.filter((item) => item.status !== "satisfied").length
      : 0;
    target.hard_violations = proof ? proof.hard_violations.length : 0;
    target.unsupported_additions = proof ? proof.unsupported_additions.length : 0;
    target.output_contract_failures = proof
      ? PROOF_OUTPUT_CONTRACT_KEYS.filter((key) => !proof.output_contract[key]).length
      : 0;
    target.quality_gains_required = proof ? proof.quality_gain_checks.length : 0;
    target.quality_gains_missing = proof
      ? proof.quality_gain_checks.filter((item) => item.status === "missing").length
      : 0;
    target.quality_gains_regressed = proof
      ? proof.quality_gain_checks.filter((item) => item.status === "regressed").length
      : 0;
    target.residual_quality_checked = proof ? proof.residual_quality_checks.length : 0;
    target.residual_quality_issues = proof
      ? proof.residual_quality_checks.filter((item) => item.status === "issue").length
      : 0;
    target.realized_contributions = proof
      ? proof.quality_gain_checks.filter((item) => item.status === "realized").map((item) => ({
        contribution_id: item.contribution_id,
        role_id: item.role_id,
        candidate_id: item.candidate_id,
        segment_ids: item.segment_ids,
        evidence_quote: item.evidence_quote,
      }))
      : [];
    target.structured_recovery_attempted = target.structured_recovery_attempted || recoveryAttempted;
    target.structured_recovery_succeeded = target.structured_recovery_succeeded
      || (recoveryAttempted && !!proof);
    target.validation_diagnostics = arrayFromCollection(target.validation_diagnostics)
      .concat(arrayFromCollection(proverTrace && proverTrace.validation_diagnostics))
      .slice(-20);
    target.reason = proof ? proof.reason_codes.join(",") : "semantic_prover_call_failed";
  }

  async function runSemanticProver(
    proverRole,
    profile,
    segments,
    finalSegments,
    directorResult,
    contextBlock,
    abortSignal,
    trace,
    completionWait,
    attemptLabel
  ) {
    const structuredRecoveryAvailable = !(trace.semantic_prover
      && trace.semantic_prover.structured_recovery_attempted);
    const proof = await callRole(
      proverRole,
      profile,
      mutableSegments(segments),
      contextBlock,
      segments,
      abortSignal,
      trace,
      {
        final_segments: finalSegments,
        semantic_judgment: directorResult.semantic_judgment,
        fusion_plan: directorResult.fusion_plan,
        draft_ledger: directorResult.draft_ledger,
      },
      Date.now(),
      {
        allowRetry: structuredRecoveryAvailable,
        allowFallback: true,
        retryErrorCodes: [
          "json_parse_failed",
          "schema_validation_failed",
          "reasoning_only_response",
        ],
        completionWait: !!completionWait,
      }
    );
    summarizeSemanticProof(trace, proof, attemptLabel);
    return proof;
  }

  /* ── Output Assembly ───────────────────────────────────── */

  function assembleOutput(segments, composerResult) {
    const composerSegments = composerResult ? asObject(composerResult.segments) : {};
    const finalSegments = [];
    let changed = false;
    let composerApplied = 0;
    let materialComposerApplied = 0;
    let materialChanged = 0;
    let metaOnlyChanged = 0;
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
        if (Object.prototype.hasOwnProperty.call(composerSegments, seg.id)) {
          const composerCore = replacementCoreText(composerSegments[seg.id]);
          if (composerCore === originalCore) {
            composerUnchanged = true;
          } else {
            finalCore = composerCore;
            source = "composer";
            operation = !composerCore && isWhollyMetaArtifactText(originalFull) ? "delete" : "replace";
            appliedRole = COMPOSER_ROLE_ID;
            composerApplied++;
          }
        }
        const finalFull = finalCore === null
          ? originalFull
          : safeString(seg.leading_ws) + finalCore + safeString(seg.trailing_ws);
        const segmentChanged = finalFull !== originalFull;
        const materiality = rewriteMateriality(originalFull, finalFull);
        if (segmentChanged) changed = true;
        else unchangedSegments++;
        if (segmentChanged && materiality.original_meta_only) {
          metaOnlyChanged++;
        } else if (segmentChanged) {
          materialChanged++;
        }
        if (segmentChanged && source === "composer") {
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
          material_change: segmentChanged,
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
      unchangedSegments,
    };
  }

  function buildAppliedEvidence(finalSegments, semanticProof) {
    const realized = arrayFromCollection(semanticProof && semanticProof.quality_gain_checks)
      .filter((item) => item.status === "realized");
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
      role_contributions: realized.filter((item) =>
        arrayFromCollection(item.segment_ids).indexOf(segment.id) >= 0
      ).map((item) => ({
        contribution_id: item.contribution_id,
        role_id: item.role_id,
        candidate_id: item.candidate_id,
        evidence_quote: item.evidence_quote,
      })),
      original_preview: preview(segment.original_text, 80),
      final_preview: preview(segment.final_text, 80),
    }));
  }

  function updateAppliedEvidence(trace, assembled, verification, semanticProof) {
    trace.applied_evidence = verification && verification.pass
      ? buildAppliedEvidence(assembled && assembled.finalSegments, semanticProof)
      : [];
    return trace.applied_evidence;
  }

  function holdAttemptedEvidence(trace, assembled) {
    trace.attempted_evidence = buildAppliedEvidence(assembled && assembled.finalSegments);
    trace.applied_evidence = [];
    return trace.attempted_evidence;
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

  function protectedStructureInventory(segments) {
    return arrayFromCollection(segments)
      .filter((segment) => segment.type === "protected" || segment.type === "inspect_only")
      .map((segment) => ({
        type: safeString(segment.type),
        kind: safeString(segment.kind),
        text: safeString(segment.text),
      }));
  }

  function verifyOutput(segments, finalSegments, output, originalText, settings) {
    const errors = [];
    const warnings = [];
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
      warnings.push("unbalanced_code_fence");
    }
    const allOpenTags = output.match(/<[^/][^>]*>/g) || [];
    const nonVoidOpenTags = allOpenTags.filter((tag) => !isVoidHtmlTag(tag));
    const closeTags = (output.match(/<\/[^>]*>/g) || []).length;
    if (Math.abs(nonVoidOpenTags.length - closeTags) > 2) {
      warnings.push("tag_balance_suspicious");
    }
    if (settings) {
      const originalInventory = protectedStructureInventory(segments);
      const finalInventory = protectedStructureInventory(buildSegmentMap(output, settings));
      const inventoryChanged = originalInventory.length !== finalInventory.length
        || originalInventory.some((item, index) => {
          const finalItem = finalInventory[index];
          return !finalItem
            || item.type !== finalItem.type
            || item.kind !== finalItem.kind
            || item.text !== finalItem.text;
        });
      if (inventoryChanged) warnings.push("host_artifact_inventory_changed_after_restore");
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
      ids.slice(allowedCount).forEach((id) => warnings.push(`duplicate_segment:${id}`));
    });
    return { pass: errors.length === 0, errors, warnings };
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
    function cancel() {
      if (!aborted) {
        aborted = true;
        try { controller.abort(); } catch (_) {}
      }
      clearTimeout(timer);
    }
    return { check, remaining, signal: controller.signal, aborted: () => aborted, cancel };
  }

  function createCompletionDeadline(startedAt, watchdogMs) {
    const controller = new AbortController();
    let aborted = false;
    const started = Number.isFinite(Number(startedAt)) ? Number(startedAt) : Date.now();
    const watchdog = Math.max(1, Number(watchdogMs) || COMPLETION_WAIT_WATCHDOG_MS);
    const deadline = started + watchdog;
    const timer = setTimeout(() => {
      if (aborted) return;
      aborted = true;
      try { controller.abort("completion_wait_watchdog"); } catch (_) {
        try { controller.abort(); } catch (_) {}
      }
    }, Math.max(0, deadline - Date.now()));
    function cancel() {
      if (!aborted) {
        aborted = true;
        try { controller.abort("completion_wait_cancelled"); } catch (_) {
          try { controller.abort(); } catch (_) {}
        }
      }
      clearTimeout(timer);
    }
    function check() {
      if (aborted) return true;
      if (Date.now() >= deadline) {
        aborted = true;
        clearTimeout(timer);
        try { controller.abort("completion_wait_watchdog"); } catch (_) {
          try { controller.abort(); } catch (_) {}
        }
        return true;
      }
      return false;
    }
    return {
      completion_wait: true,
      started_at: started,
      watchdog_ms: watchdog,
      check,
      remaining: () => Math.max(0, deadline - Date.now()),
      signal: controller.signal,
      aborted: () => aborted,
      cancel,
    };
  }

  function createPipelineStageScope(deadline, stage, trace, explicitMaxBudgetMs) {
    const stageId = safeString(stage) || "pipeline_stage";
    const remainingAtStart = deadline ? deadline.remaining() : 0;
    if (!deadline || remainingAtStart <= 0) {
      if (trace && trace.scheduler) {
        trace.scheduler.stage_budgets.push({
          stage: stageId,
          budget_ms: remainingAtStart,
          remaining_at_start_ms: remainingAtStart,
          completion_wait: !!(deadline && deadline.completion_wait),
        });
      }
      return { signal: deadline && deadline.signal, budget_ms: remainingAtStart, cancel() {} };
    }
    const shareByStage = {
      semantic_judge: 0.28,
      adaptive_revision: 0.3,
      adaptive_rejudge: 0.25,
      whole_scene_composer: 0.62,
      preproof_composer_recovery: 0.58,
      semantic_repair_composer: 0.6,
    };
    const share = shareByStage[stageId] || 0.6;
    const minimumStageMs = Math.min(
      1000,
      Math.max(1, Math.floor(remainingAtStart * 0.5))
    );
    let budgetMs = Math.min(
      remainingAtStart,
      Math.max(minimumStageMs, Math.floor(remainingAtStart * share))
    );
    const explicitCap = Number(explicitMaxBudgetMs);
    if (Number.isFinite(explicitCap) && explicitCap > 0) {
      budgetMs = Math.max(1, Math.min(budgetMs, Math.floor(explicitCap)));
    }
    const controller = new AbortController();
    let cancelled = false;
    const abortStage = (reason) => {
      if (controller.signal.aborted) return;
      try { controller.abort(reason); } catch (_) {
        try { controller.abort(); } catch (_) {}
      }
    };
    const onParentAbort = () => abortStage(
      deadline.signal && deadline.signal.reason
        ? deadline.signal.reason
        : `pipeline_deadline:${stageId}`
    );
    if (deadline.signal && deadline.signal.aborted) onParentAbort();
    else if (deadline.signal) deadline.signal.addEventListener("abort", onParentAbort, { once: true });
    const timer = setTimeout(() => abortStage(`stage_budget:${stageId}`), budgetMs);
    if (trace && trace.scheduler) {
      trace.scheduler.stage_budgets.push({
        stage: stageId,
        budget_ms: budgetMs,
        remaining_at_start_ms: remainingAtStart,
        completion_wait: !!deadline.completion_wait,
        explicit_cap_ms: Number.isFinite(explicitCap) && explicitCap > 0
          ? Math.floor(explicitCap)
          : 0,
      });
    }
    return {
      signal: controller.signal,
      budget_ms: budgetMs,
      cancel() {
        if (cancelled) return;
        cancelled = true;
        clearTimeout(timer);
        try {
          if (deadline.signal) deadline.signal.removeEventListener("abort", onParentAbort);
        } catch (_) {}
      },
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
      group.effective_concurrency = Math.max(1, Math.min(group.base_concurrency, group.selected_calls));
      group.reason = "provider_concurrency";
    });
    return groups;
  }

  function recordExecutionGroupPlan(trace, plan) {
    if (!trace) return;
    if (!trace.scheduler) {
      trace.scheduler = {
        completion_wait: false,
        endpoint_groups: [],
      };
    }
    Object.keys(plan || {}).forEach((key) => {
      trace.scheduler.endpoint_groups.push(Object.assign({}, plan[key]));
    });
  }

  function sceneCandidateSimilarity(left, right) {
    const grams = (candidate) => {
      let text = Object.keys(asObject(candidate && candidate.segments)).sort()
        .map((id) => safeString(candidate.segments[id])).join("\n").toLowerCase().replace(/\s+/g, "");
      try { text = text.normalize("NFKC"); } catch (_) {}
      const chars = Array.from(text);
      const set = new Set();
      const width = chars.length < 3 ? 1 : 3;
      for (let i = 0; i <= chars.length - width; i++) set.add(chars.slice(i, i + width).join(""));
      return set;
    };
    return setJaccardSimilarity(grams(left), grams(right));
  }

  function admitSceneCandidate(pool, seenSignatures, candidate) {
    if (!Array.isArray(pool) || !(seenSignatures instanceof Set)
        || !candidate || !safeString(candidate.scene_signature)) {
      return false;
    }
    const duplicate = pool.find((item) =>
      safeString(item.role_id) === safeString(candidate.role_id)
      && (
        item.scene_signature === candidate.scene_signature
        || sceneCandidateSimilarity(item, candidate) >= 0.96
      )
    );
    if (duplicate) {
      duplicate.supporting_roles = uniqueList(
        arrayFromCollection(duplicate.supporting_roles).concat(candidate.supporting_roles || [candidate.role_id])
      );
      duplicate.evidence_refs = uniqueList(
        arrayFromCollection(duplicate.evidence_refs).concat(candidate.evidence_refs || [])
      );
      duplicate.addressed_issues = uniqueList(
        arrayFromCollection(duplicate.addressed_issues).concat(candidate.addressed_issues || [])
      );
      duplicate.retained_beats = normalizeCandidateReferences(
        arrayFromCollection(duplicate.retained_beats).concat(candidate.retained_beats || []), 32
      );
      duplicate.proposed_additions = normalizeCandidateReferences(
        arrayFromCollection(duplicate.proposed_additions).concat(candidate.proposed_additions || []), 24
      );
      duplicate.duplicate_count = (duplicate.duplicate_count || 0) + 1;
      return false;
    }
    seenSignatures.add(candidate.scene_signature);
    pool.push(candidate);
    return true;
  }

  function downstreamReserveWindowMs(deadline, roleProfiles) {
    const configured = arrayFromCollection(roleProfiles).filter((profile) => profile && isProfileConfigured(profile));
    if (!deadline || !configured.length) return 0;
    const remaining = deadline.remaining();
    if (remaining <= 0) return 0;
    if (deadline.completion_wait) {
      return Math.max(1000, Math.floor(remaining * 0.5));
    }
    const requested = configured.reduce((sum, profile) => (
      sum + clampNumber(profile.timeout_ms, 5000, 300000, 60000)
        * (configuredFallbackProfile(profile) ? 2 : 1)
    ), 0);
    const dynamicShare = Math.floor(remaining * 0.72);
    return Math.max(
      0,
      Math.min(requested, dynamicShare, remaining)
    );
  }

  function chooseLastValidCandidate(sceneCandidates, semanticJudgment) {
    const candidates = {};
    arrayFromCollection(sceneCandidates).forEach((candidate) => {
      candidates[safeString(candidate.candidate_id)] = candidate;
    });
    return arrayFromCollection(semanticJudgment && semanticJudgment.candidate_judgments)
      .filter((judgment) => judgment && judgment.verdict !== "reject" && candidates[judgment.candidate_id])
      .map((judgment) => {
        const candidate = candidates[judgment.candidate_id];
        const score = arrayFromCollection(judgment.quality_gains).length * 8
          + arrayFromCollection(judgment.accepted_elements).length * 6
          + arrayFromCollection(judgment.preserved_ledger_ids).length
          - arrayFromCollection(judgment.missing_ledger_ids).length * 8
          - arrayFromCollection(judgment.quality_regressions).length * 6
          - arrayFromCollection(judgment.unsupported_additions).length * 12
          - arrayFromCollection(judgment.hard_violations).length * 100
          + Math.max(0, Number(candidate.revision_round) || 0) * 4;
        return { candidate, score };
      })
      .sort((left, right) =>
        (Number(right.candidate.revision_round) || 0) - (Number(left.candidate.revision_round) || 0)
        || right.score - left.score
        || safeString(left.candidate.candidate_id).localeCompare(safeString(right.candidate.candidate_id)))
      .map((entry) => entry.candidate)[0] || null;
  }

  function chooseFallbackSceneCandidate(sceneCandidates) {
    return arrayFromCollection(sceneCandidates).filter((candidate) =>
      candidate && Object.keys(asObject(candidate.segments)).length > 0
    ).slice().sort((left, right) =>
      (Number(right.revision_round) || 0) - (Number(left.revision_round) || 0)
      || (Number(right.confidence) || 0) - (Number(left.confidence) || 0)
      || safeString(left.candidate_id).localeCompare(safeString(right.candidate_id))
    )[0] || null;
  }

  function candidateLineageSnapshot(candidate) {
    if (!candidate) return null;
    return {
      candidate_id: safeString(candidate.candidate_id),
      role_id: safeString(candidate.role_id),
      revision_round: Math.max(0, Number(candidate.revision_round) || 0),
      parent_candidate_id: safeString(candidate.parent_candidate_id),
      input_draft_digest: safeString(candidate.input_draft_digest),
      scene_signature: safeString(candidate.scene_signature),
      segments: deepClone(asObject(candidate.segments)),
    };
  }

  function materializeCandidateSegments(baseSegments, candidate) {
    const replacements = asObject(candidate && candidate.segments);
    return arrayFromCollection(baseSegments).map((segment) => {
      if (!segment || segment.type !== "mutable"
          || !Object.prototype.hasOwnProperty.call(replacements, segment.id)) {
        return Object.assign({}, segment);
      }
      const core = safeString(replacements[segment.id]);
      return Object.assign({}, segment, {
        core_text: core,
        text: safeString(segment.leading_ws) + core + safeString(segment.trailing_ws),
      });
    });
  }

  function chooseAdaptiveRequirementRole(requirements, profiles) {
    const grouped = {};
    arrayFromCollection(requirements).forEach((requirement, index) => {
      const roleId = safeString(requirement && requirement.role_id);
      if (SPECIALIST_ROLE_IDS.indexOf(roleId) < 0) return;
      const profile = profiles && profiles[roleId];
      if (!profile || !profile.enabled || !isProfileConfigured(profile)) return;
      if (!grouped[roleId]) grouped[roleId] = { role_id: roleId, requirements: [], first: index };
      grouped[roleId].requirements.push(requirement);
    });
    return Object.values(grouped).sort((left, right) =>
      right.requirements.length - left.requirements.length
      || left.first - right.first
      || SPECIALIST_ROLE_IDS.indexOf(left.role_id) - SPECIALIST_ROLE_IDS.indexOf(right.role_id)
    )[0] || null;
  }

  function updateJudgmentTrace(trace, semanticJudgment, fusionPlan) {
    const judgeItems = arrayFromCollection(semanticJudgment && semanticJudgment.candidate_judgments);
    trace.semantic_judge.status = semanticJudgment ? "fulfilled" : "failed";
    trace.semantic_judge.accepted_candidates = judgeItems.filter((item) => item.verdict === "accept").length;
    trace.semantic_judge.constrained_candidates = judgeItems.filter(
      (item) => item.verdict === "accept_with_constraints"
    ).length;
    trace.semantic_judge.rejected_candidates = judgeItems.filter((item) => item.verdict === "reject").length;
    trace.semantic_judge.missing_facts = judgeItems.reduce(
      (sum, item) => sum + arrayFromCollection(item.missing_ledger_ids).length, 0
    );
    trace.semantic_judge.unsupported_additions = judgeItems.reduce(
      (sum, item) => sum + arrayFromCollection(item.unsupported_additions).length, 0
    );
    trace.semantic_judge.hard_violations = judgeItems.reduce(
      (sum, item) => sum + arrayFromCollection(item.hard_violations).length, 0
    );
    trace.semantic_judge.unresolved_requirements = arrayFromCollection(
      semanticJudgment && semanticJudgment.unresolved_requirements
    ).length;
    trace.fusion_plan.status = fusionPlan && fusionPlan.semantic_ready ? "ready" : "rejected_all";
    trace.fusion_plan.accepted_candidates = fusionPlan ? fusionPlan.accepted_candidate_ids.length : 0;
    trace.fusion_plan.rejected_candidates = fusionPlan ? fusionPlan.rejected_candidates.length : 0;
    trace.fusion_plan.consensus_claims = fusionPlan ? fusionPlan.consensus_claims.length : 0;
    trace.fusion_plan.complementary_claims = fusionPlan ? fusionPlan.complementary_claims.length : 0;
    trace.fusion_plan.conflicts = fusionPlan ? fusionPlan.conflicts.length : 0;
    trace.fusion_plan.prohibited_additions = fusionPlan ? fusionPlan.prohibited_additions.length : 0;
    trace.fusion_plan.required_contributions = fusionPlan
      ? fusionPlan.required_contributions.length
      : 0;
    trace.fusion_plan.unresolved_requirements = fusionPlan
      ? arrayFromCollection(fusionPlan.unresolved_requirements).length
      : 0;
    trace.semantic_judge.required_contributions = trace.fusion_plan.required_contributions;
    if (semanticJudgment) {
      trace.lineage.judgment_digests.push(stableDigest(semanticJudgment));
    }
    if (fusionPlan && fusionPlan.plan_id) {
      trace.lineage.plan_ids.push(fusionPlan.plan_id);
    }
  }

  function refreshSceneCandidateTrace(trace, sceneCandidates, mutableSegIds) {
    const counts = {};
    arrayFromCollection(mutableSegIds).forEach((segmentId) => { counts[segmentId] = 0; });
    arrayFromCollection(sceneCandidates).forEach((candidate) => {
      Object.keys(asObject(candidate && candidate.segments)).forEach((segmentId) => {
        if (Object.prototype.hasOwnProperty.call(counts, segmentId)) counts[segmentId]++;
      });
    });
    trace.candidates.total = arrayFromCollection(sceneCandidates).length;
    trace.candidates.segment_variant_total = Object.values(counts)
      .reduce((sum, count) => sum + count, 0);
    trace.candidates.by_segment = counts;
    trace.lineage.candidate_ids = arrayFromCollection(sceneCandidates)
      .map((candidate) => safeString(candidate && candidate.candidate_id))
      .filter(Boolean);
  }

  async function runAdaptiveRevisionRounds(options) {
    const sceneCandidates = options.sceneCandidates;
    const seenSceneSignatures = options.seenSceneSignatures;
    const presetId = safeString(options.presetId) || "balanced";
    const limit = ADAPTIVE_REVISION_LIMIT[presetId] || 0;
    const trace = options.trace;
    const tailReserveMs = Math.max(
      0,
      Number(options.tailReserveMs != null
        ? options.tailReserveMs
        : options.downstreamReserveMs) || 0
    );
    const tailGuardMs = Math.min(
      1000,
      Math.max(20, Math.floor(tailReserveMs * 0.02))
    );
    trace.revision_convergence.limit = limit;
    let semanticJudgment = options.semanticJudgment;
    let fusionPlan = options.fusionPlan;
    let lastValidCandidate = chooseLastValidCandidate(sceneCandidates, semanticJudgment);
    trace.revision_convergence.last_valid_candidate_id = safeString(
      lastValidCandidate && lastValidCandidate.candidate_id
    );
    trace.revision_convergence.last_valid_draft_digest = lastValidCandidate
      ? stableDigest(lastValidCandidate.segments)
      : "";

    for (let round = 1; round <= limit; round++) {
      const requirements = arrayFromCollection(fusionPlan && fusionPlan.unresolved_requirements);
      const adaptiveStartMarginMs = options.deadline.completion_wait ? 15000 : 5000;
      if (!requirements.length || !lastValidCandidate) break;
      if (options.deadline.check()
          || options.deadline.remaining() <= tailReserveMs + adaptiveStartMarginMs) break;
      const allocation = chooseAdaptiveRequirementRole(requirements, options.profiles);
      if (!allocation) break;
      const baseRole = DEFAULT_ROLES.find((role) => role.role_id === allocation.role_id);
      const profile = options.profiles[allocation.role_id];
      if (!baseRole || !profile) break;

      const workingSegments = materializeCandidateSegments(options.segments, lastValidCandidate);
      const workingMutable = mutableSegments(workingSegments).filter((segment) =>
        mutableCoreText(segment).trim().length > 0
      );
      const parentCandidateId = safeString(lastValidCandidate.candidate_id);
      const inputDraftDigest = stableDigest(lastValidCandidate.segments);
      const roundTrace = {
        round,
        role_id: allocation.role_id,
        requirement_ids: allocation.requirements.map((item) => item.requirement_id),
        parent_candidate_id: parentCandidateId,
        input_draft_digest: inputDraftDigest,
        output_candidate_id: "",
        output_draft_digest: "",
        status: "running",
        failure_reason: "",
      };
      trace.revision_convergence.rounds.push(roundTrace);
      trace.revision_convergence.attempted++;
      trace.budget.judge_attempt_reserved = Math.max(
        1,
        Number(trace.budget.judge_attempt_reserved) || 0
      );
      const revisionRole = Object.assign({}, baseRole, { stage: "revision" });
      const revisionSlackMs = Math.max(
        0,
        options.deadline.remaining() - tailReserveMs - tailGuardMs
      );
      if (revisionSlackMs <= 0) {
        trace.budget.judge_attempt_reserved = 0;
        roundTrace.status = "skipped";
        roundTrace.failure_reason = "adaptive_tail_reserve_reached_before_revision";
        break;
      }
      const revisionScope = createPipelineStageScope(
        options.deadline,
        "adaptive_revision",
        trace,
        Math.max(1, Math.floor(revisionSlackMs * 0.6))
      );
      let revisionResult;
      try {
        revisionResult = await callRole(
          revisionRole,
          profile,
          workingMutable,
          options.contextBlock,
          workingSegments,
          revisionScope.signal,
          trace,
          {
            draft_ledger: options.draftLedger,
            revision_round: {
              round,
              parent_candidate_id: parentCandidateId,
              requirements: allocation.requirements,
              semantic_judgment: semanticJudgment,
              fusion_plan: fusionPlan,
            },
          },
          Date.now(),
          {
            initialAttemptKind: "adaptive_revision",
            allowRetry: false,
            allowFallback: true,
            completionWait: !!options.deadline.completion_wait,
            canContinue: () => !options.deadline.check()
              && options.deadline.remaining() > tailReserveMs,
          }
        );
      } finally {
        revisionScope.cancel();
      }
      if (!revisionResult || !arrayFromCollection(revisionResult.candidates).length) {
        trace.budget.judge_attempt_reserved = 0;
        roundTrace.status = revisionResult ? "no_candidate" : "failed";
        roundTrace.failure_reason = revisionResult ? "no_complete_scene_candidate" : "revision_call_failed";
        if (revisionResult) trace.revision_convergence.no_candidate++;
        else trace.revision_convergence.failed++;
        continue;
      }

      const candidate = revisionResult.candidates[0];
      candidate.revision_round = round;
      candidate.parent_candidate_id = parentCandidateId;
      candidate.input_draft_digest = inputDraftDigest;
      candidate.candidate_id = `revision_${stableDigest({
        role_id: candidate.role_id,
        parent_candidate_id: parentCandidateId,
        revision_round: round,
        scene_signature: candidate.scene_signature,
      })}`;
      if (!admitSceneCandidate(sceneCandidates, seenSceneSignatures, candidate)) {
        trace.budget.judge_attempt_reserved = 0;
        trace.candidates.duplicate_scene_candidates++;
        trace.revision_convergence.no_candidate++;
        roundTrace.status = "no_candidate";
        roundTrace.failure_reason = "duplicate_or_near_duplicate_revision";
        continue;
      }
      roundTrace.output_candidate_id = candidate.candidate_id;
      roundTrace.output_draft_digest = stableDigest(candidate.segments);

      const rejudgeBudgetMs = Math.max(
        0,
        options.deadline.remaining() - tailReserveMs - tailGuardMs
      );
      if (rejudgeBudgetMs <= 0) {
        sceneCandidates.splice(sceneCandidates.indexOf(candidate), 1);
        seenSceneSignatures.delete(candidate.scene_signature);
        trace.budget.judge_attempt_reserved = 0;
        trace.revision_convergence.failed++;
        roundTrace.status = "failed";
        roundTrace.failure_reason = "adaptive_tail_reserve_reached_before_rejudge";
        break;
      }
      const rejudgeScope = createPipelineStageScope(
        options.deadline,
        "adaptive_rejudge",
        trace,
        rejudgeBudgetMs
      );
      let revisedJudgment;
      try {
        revisedJudgment = await callRole(
          options.judgeRole,
          options.judgeProfile,
          options.mutableSegs,
          options.contextBlock,
          options.allSegments,
          rejudgeScope.signal,
          trace,
          { scene_candidates: sceneCandidates, draft_ledger: options.draftLedger },
          Date.now(),
          {
            initialAttemptKind: "adaptive_rejudge",
            allowRetry: false,
            allowFallback: true,
            completionWait: !!options.deadline.completion_wait,
          }
        );
      } finally {
        rejudgeScope.cancel();
      }
      trace.budget.judge_attempt_reserved = 0;
      const revisedPlan = revisedJudgment
        ? buildFusionPlan(revisedJudgment, sceneCandidates, options.draftLedger, options.mutableSegs)
        : null;
      if (!revisedJudgment || !revisedPlan || !revisedPlan.semantic_ready) {
        sceneCandidates.splice(sceneCandidates.indexOf(candidate), 1);
        seenSceneSignatures.delete(candidate.scene_signature);
        trace.revision_convergence.failed++;
        roundTrace.status = "failed";
        roundTrace.failure_reason = revisedJudgment
          ? "adaptive_rejudge_rejected_all"
          : "adaptive_rejudge_failed";
        continue;
      }

      semanticJudgment = revisedJudgment;
      fusionPlan = revisedPlan;
      updateJudgmentTrace(trace, semanticJudgment, fusionPlan);
      const candidateJudgment = semanticJudgment.candidate_judgments.find(
        (item) => item.candidate_id === candidate.candidate_id
      );
      if (candidateJudgment && candidateJudgment.verdict !== "reject") {
        lastValidCandidate = candidate;
        trace.revision_convergence.fulfilled++;
        roundTrace.status = "fulfilled";
      } else {
        trace.revision_convergence.failed++;
        roundTrace.status = "rejected";
        roundTrace.failure_reason = "adaptive_candidate_rejected";
      }
      trace.revision_convergence.last_valid_candidate_id = safeString(
        lastValidCandidate && lastValidCandidate.candidate_id
      );
      trace.revision_convergence.last_valid_draft_digest = lastValidCandidate
        ? stableDigest(lastValidCandidate.segments)
        : "";
    }

    trace.budget.judge_attempt_reserved = 0;
    trace.revision_convergence.remaining_requirements = arrayFromCollection(
      fusionPlan && fusionPlan.unresolved_requirements
    ).length;
    return {
      semanticJudgment,
      fusionPlan,
      lastValidCandidate,
      revisionLineage: trace.revision_convergence.rounds.slice(),
    };
  }

  async function runDirectComposerFallback(options) {
    const reason = safeString(options.reason) || "specialist_or_judge_unavailable";
    if (!options.composerRole || !options.composerProfile
        || !options.composerProfile.enabled || !isProfileConfigured(options.composerProfile)) {
      return {
        directorResult: null,
        composerResult: null,
        semanticJudgment: options.semanticJudgment || null,
        fusionPlan: options.rejectedPlan || null,
        proverRole: options.proverRole,
        proverProfile: options.proverProfile,
        failureReason: "composer_not_configured",
      };
    }
    if (options.deadline.check() || options.deadline.remaining() <= 0) {
      return {
        directorResult: null,
        composerResult: null,
        semanticJudgment: options.semanticJudgment || null,
        fusionPlan: options.rejectedPlan || null,
        proverRole: options.proverRole,
        proverProfile: options.proverProfile,
        failureReason: `${reason}_deadline`,
      };
    }
    const fallback = buildDirectComposerFallbackDirector(
      reason,
      options.segments,
      options.draftLedger,
      options.semanticJudgment,
      options.rejectedPlan
    );
    options.trace.semantic_judge.status = `direct_composer_fallback:${reason}`;
    options.trace.fusion_plan.status = "direct_composer_fallback";
    options.trace.fusion_plan.direct_composer_fallback = true;
    options.trace.fusion_plan.fallback_reason = reason;
    options.trace.fusion_plan.unresolved_requirements = 1;
    options.trace.composer.direct_fallback = true;
    options.trace.composer.fallback_reason = reason;
    options.trace.budget.judge_attempt_reserved = 0;
    const composerScope = createPipelineStageScope(
      options.deadline, "whole_scene_composer", options.trace
    );
    let composerResult;
    try {
      composerResult = await runComposer(
        options.composerRole,
        options.composerProfile,
        options.segments,
        fallback.directorResult,
        options.contextBlock,
        composerScope.signal,
        options.trace,
        options.mutableSegs,
        !!options.deadline.completion_wait
      );
    } finally {
      composerScope.cancel();
    }
    options.trace.active_calls_final = Math.max(0, Number(options.activeCallCount) || 0);
    return {
      directorResult: fallback.directorResult,
      composerResult,
      semanticJudgment: fallback.semanticJudgment,
      fusionPlan: fallback.fusionPlan,
      proverRole: options.proverRole,
      proverProfile: options.proverProfile,
      failureReason: composerResult
        ? ""
        : (options.deadline.check() ? "composer_deadline" : "composer_failed"),
    };
  }

  async function scheduleRoles(roles, profiles, segments, contextBlock, allSegments, deadline, trace, maxParallel, draftLedger) {
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
    const specialistRoles = roles.filter(
      (r) => !r.is_composer && !r.is_judge && !r.is_prover && !r.is_input_planner
    );
    const judgeRole = roles.find((r) => r.is_judge);
    const composerRole = roles.find((r) => r.is_composer);
    const proverRole = roles.find((r) => r.is_prover);
    const judgeProfile = judgeRole && profiles[judgeRole.role_id];
    const composerProfile = composerRole && profiles[composerRole.role_id];
    const proverProfile = proverRole && profiles[proverRole.role_id];
    const judgeConfigured = !!(
      judgeRole && judgeProfile && judgeProfile.enabled && isProfileConfigured(judgeProfile)
    );
    if (!judgeConfigured) {
      trace.semantic_judge.status = "not_configured";
    }
    if (!composerRole || !composerProfile || !composerProfile.enabled
        || !isProfileConfigured(composerProfile)) {
      trace.composer.status = "not_configured";
    }
    if (!proverRole || !proverProfile || !proverProfile.enabled || !isProfileConfigured(proverProfile)) {
      trace.semantic_prover.status = "not_configured";
    }
    const downstreamReserveMs = downstreamReserveWindowMs(deadline, [
      judgeRole && judgeProfile && judgeProfile.enabled ? judgeProfile : null,
      composerRole && composerProfile && composerProfile.enabled ? composerProfile : null,
      proverRole && proverProfile && proverProfile.enabled ? proverProfile : null,
    ]);
    trace.composer.reserve_ms = downstreamReserveMs;
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
    if (judgeRole && judgeProfile && judgeProfile.enabled
        && isProfileConfigured(judgeProfile)) {
      recordExecutionGroupPlan(trace, {
        [executionGroupKey(judgeProfile)]: {
          stage: "semantic_judge",
          endpoint_group: executionGroupKey(judgeProfile),
          selected_calls: 1,
          base_concurrency: 1,
          effective_concurrency: 1,
          reason: "judge_after_specialists",
        },
      });
    }
    if (proverRole && proverProfile && proverProfile.enabled
        && isProfileConfigured(proverProfile)) {
      recordExecutionGroupPlan(trace, {
        [executionGroupKey(proverProfile)]: {
          stage: "semantic_prover",
          endpoint_group: executionGroupKey(proverProfile),
          selected_calls: 1,
          base_concurrency: 1,
          effective_concurrency: 1,
          reason: "prover_after_composer",
        },
      });
    }
    const candidatesBySegment = {};
    const sceneCandidates = [];
    const seenSceneSignatures = new Set();
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
    const reserveDelay = downstreamReserveMs > 0
      ? Math.max(0, deadline.remaining() - downstreamReserveMs)
      : -1;
    const reserveTimer = reserveDelay >= 0
      ? setTimeout(() => stopSpecialists(deadline.check() ? "deadline" : "judge_composer_reserve"), reserveDelay)
      : null;
    const onPipelineAbort = () => stopSpecialists("deadline");
    if (deadline.signal.aborted) onPipelineAbort();
    else deadline.signal.addEventListener("abort", onPipelineAbort, { once: true });

    if (trace && trace.budget) {
      trace.budget.composer_attempt_reserved = composerRole && composerProfile
        && composerProfile.enabled && isProfileConfigured(composerProfile)
        ? 2
        : 0;
      trace.budget.composer_attempt_used = 0;
      trace.budget.judge_attempt_reserved = judgeRole && judgeProfile
        && judgeProfile.enabled && isProfileConfigured(judgeProfile)
        ? (configuredFallbackProfile(judgeProfile) ? 2 : 1)
        : 0;
      trace.budget.judge_attempt_used = 0;
      trace.budget.prover_attempt_reserved = proverRole && proverProfile
        && proverProfile.enabled && isProfileConfigured(proverProfile)
        ? 2
        : 0;
      trace.budget.prover_attempt_used = 0;
      trace.budget.specialist_primary_remaining = specialistTasks.length;
    }

    let activeCallCount = 0;
    const specialistResultRecords = [];
    const directFallback = async (reason, semanticJudgment, rejectedPlan) => {
      const result = await runDirectComposerFallback({
        reason,
        semanticJudgment,
        rejectedPlan,
        segments,
        mutableSegs,
        contextBlock,
        deadline,
        trace,
        draftLedger,
        composerRole,
        composerProfile,
        proverRole,
        proverProfile,
        activeCallCount,
      });
      if (!result.composerResult) {
        const fallbackCandidate = chooseFallbackSceneCandidate(sceneCandidates);
        if (fallbackCandidate) {
          if (!result.directorResult) {
            const fallback = buildDirectComposerFallbackDirector(
              reason,
              segments,
              draftLedger,
              semanticJudgment,
              rejectedPlan
            );
            result.directorResult = fallback.directorResult;
            result.semanticJudgment = fallback.semanticJudgment;
            result.fusionPlan = fallback.fusionPlan;
          }
          result.composerResult = {
            schema: "scene_composition.v1",
            segments: deepClone(asObject(fallbackCandidate.segments)),
            recovered_from_candidate_id: safeString(fallbackCandidate.candidate_id),
            recovered_from_role_id: safeString(fallbackCandidate.role_id),
          };
          result.failureReason = "";
          trace.semantic_judge.status = `direct_composer_fallback:${reason}`;
          trace.fusion_plan.status = "direct_composer_fallback";
          trace.fusion_plan.direct_composer_fallback = true;
          trace.fusion_plan.fallback_reason = reason;
          trace.fusion_plan.unresolved_requirements = 1;
          trace.composer.direct_fallback = true;
          trace.composer.fallback_reason = reason;
          trace.composer.status = "specialist_scene_candidate";
          trace.composer.specialist_candidate_recovered = true;
        }
      }
      return result;
    };

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
            { draft_ledger: draftLedger },
            queuedAt,
            {
              allowRetry: false,
              allowFallback: false,
              deferStructuredRecovery: role.role_id === "character_reader"
                || role.role_id === "style_reader",
              completionWait: !!deadline.completion_wait,
              canContinue: () => !specialistController.signal.aborted
                && deadline.remaining() > downstreamReserveMs,
            }
          )
            .then((result) => {
              specialistResultRecords.push({ role, profile, result });
              if (result && result.candidates) {
                result.candidates.forEach((candidate) => {
                  if (!admitSceneCandidate(sceneCandidates, seenSceneSignatures, candidate)) {
                    trace.candidates.duplicate_scene_candidates++;
                  }
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
    if (reserveTimer) clearTimeout(reserveTimer);
    try { deadline.signal.removeEventListener("abort", onPipelineAbort); } catch (_) {}
    trace.composer.specialist_stop_reason = raceResult === "settled" ? "" : raceResult;

    trace.active_calls_after_specialists = activeCallCount;
    if (raceResult === "settled" && trace && trace.budget) {
      trace.budget.specialist_primary_remaining = 0;
    }

    const presetId = safeString(trace.router && trace.router.preset) || "balanced";
    const presetRecoveryLimit = SPECIALIST_STRUCTURED_RECOVERY_LIMIT[presetId];
    const recoveryLimit = Number.isFinite(presetRecoveryLimit)
      ? presetRecoveryLimit
      : SPECIALIST_STRUCTURED_RECOVERY_LIMIT.balanced;
    const deferredRecoveryRecords = specialistResultRecords
      .filter((record) => record.result && record.result.__deferred_specialist_recovery)
      .sort((left, right) => {
        const priority = { character_reader: 0, style_reader: 1 };
        return (priority[left.role.role_id] == null ? 9 : priority[left.role.role_id])
          - (priority[right.role.role_id] == null ? 9 : priority[right.role.role_id]);
      });
    const recoveryQueue = deferredRecoveryRecords;
    trace.candidates.structured_recovery_queued = deferredRecoveryRecords.length;
    for (const record of recoveryQueue) {
      if (trace.candidates.structured_recovery_attempted >= recoveryLimit) break;
      if (deadline.check()
          || deadline.remaining() <= downstreamReserveMs) break;
      trace.candidates.structured_recovery_attempted++;
      const recovered = await callRole(
        record.role,
        structuredRecoveryProfile(record.profile),
        mutableSegs,
        "",
        allSegments,
        deadline.signal,
        trace,
        { draft_ledger: draftLedger },
        Date.now(),
        {
          initialPrompts: specialistCompactRecoveryPrompts(
            record.role,
            record.result,
            mutableSegs
          ),
          initialAttemptKind: "specialist_compact_recovery",
          allowRetry: false,
          allowFallback: false,
          completionWait: !!deadline.completion_wait,
          canContinue: () => !deadline.check()
            && deadline.remaining() > downstreamReserveMs,
        }
      );
      if (!recovered || !Array.isArray(recovered.candidates) || !recovered.candidates.length) {
        continue;
      }
      let admittedRecoveryCandidates = 0;
      recovered.candidates.forEach((candidate) => {
        if (!admitSceneCandidate(sceneCandidates, seenSceneSignatures, candidate)) {
          trace.candidates.duplicate_scene_candidates++;
        } else {
          admittedRecoveryCandidates++;
        }
      });
      if (admittedRecoveryCandidates > 0) {
        trace.candidates.structured_recovery_succeeded++;
      }
    }

    sceneCandidates.forEach((candidate) => {
      Object.keys(candidate.segments || {}).forEach((segmentId) => {
        if (!candidatesBySegment[segmentId]) return;
        candidatesBySegment[segmentId].push({
          candidate_id: candidate.candidate_id,
          scene_signature: candidate.scene_signature,
          role_id: candidate.role_id,
          supporting_roles: candidate.supporting_roles,
          duplicate_count: candidate.duplicate_count || 0,
          rewrite: candidate.segments[segmentId],
          operation: candidate.segment_operations[segmentId] || "replace",
          confidence: candidate.confidence,
          issues: candidate.addressed_issues || [],
          change_summary: candidate.change_summary || "",
          evidence_refs: candidate.evidence_refs || [],
          retained_beats: candidate.retained_beats || [],
          proposed_additions: candidate.proposed_additions || [],
          tags: candidate.tags || [],
        });
      });
    });
    trace.candidates.total = sceneCandidates.length;
    trace.candidates.segment_variant_total = Object.values(candidatesBySegment)
      .reduce((sum, entries) => sum + entries.length, 0);
    Object.keys(candidatesBySegment).forEach((segId) => {
      trace.candidates.by_segment[segId] = candidatesBySegment[segId].length;
    });

    if (!sceneCandidates.length) {
      trace.semantic_judge.status = "no_candidates";
      return directFallback("no_scene_candidates", null, null);
    }
    if (!judgeConfigured) {
      return directFallback("semantic_judge_not_configured", null, null);
    }
    let semanticJudgment = null;
    if (!deadline.check() && deadline.remaining() > 0) {
      const judgeScope = createPipelineStageScope(deadline, "semantic_judge", trace);
      try {
        semanticJudgment = await callRole(
          judgeRole,
          judgeProfile,
          mutableSegs,
          contextBlock,
          allSegments,
          judgeScope.signal,
          trace,
          { scene_candidates: sceneCandidates, draft_ledger: draftLedger },
          Date.now(),
          {
            allowRetry: false,
            allowFallback: true,
            completionWait: !!deadline.completion_wait,
          }
        );
      } finally {
        judgeScope.cancel();
      }
    }
    if (!semanticJudgment) {
      trace.semantic_judge.status = deadline.check() ? "skipped_deadline" : "failed";
      return directFallback("semantic_judge_failed", null, null);
    }

    let fusionPlan = buildFusionPlan(semanticJudgment, sceneCandidates, draftLedger, mutableSegs);
    updateJudgmentTrace(trace, semanticJudgment, fusionPlan);
    if (!fusionPlan || !fusionPlan.semantic_ready) {
      trace.semantic_judge.status = "rejected_all";
      return directFallback("semantic_judge_rejected_all", semanticJudgment, fusionPlan);
    }

    const adaptiveTailReserveMs = Math.max(
      1000,
      Math.floor(deadline.remaining() * 0.48)
    );
    trace.revision_convergence.tail_reserve_ms = adaptiveTailReserveMs;
    const adaptiveResult = await runAdaptiveRevisionRounds({
      sceneCandidates,
      seenSceneSignatures,
      semanticJudgment,
      fusionPlan,
      profiles,
      segments,
      mutableSegs,
      contextBlock,
      allSegments,
      deadline,
      trace,
      draftLedger,
      judgeRole,
      judgeProfile,
      downstreamReserveMs,
      tailReserveMs: adaptiveTailReserveMs,
      presetId,
    });
    semanticJudgment = adaptiveResult.semanticJudgment;
    fusionPlan = adaptiveResult.fusionPlan;
    refreshSceneCandidateTrace(trace, sceneCandidates, mutableSegIds);

    const directorResult = buildComposerCandidatePool(
      sceneCandidates, semanticJudgment, fusionPlan, segments, draftLedger
    );
    directorResult.revision_lineage = adaptiveResult.revisionLineage;
    directorResult.last_valid_candidate = candidateLineageSnapshot(
      adaptiveResult.lastValidCandidate
    );
    let composerResult = null;
    if (composerRole && !deadline.check() && deadline.remaining() > 0) {
      if (composerProfile && composerProfile.enabled && isProfileConfigured(composerProfile)) {
        const composerScope = createPipelineStageScope(
          deadline, "whole_scene_composer", trace
        );
        try {
          composerResult = await runComposer(
            composerRole,
            composerProfile,
            segments,
            directorResult,
            contextBlock,
            composerScope.signal,
            trace,
            mutableSegs,
            !!deadline.completion_wait
          );
        } finally {
          composerScope.cancel();
        }
      } else {
        trace.composer.status = "not_configured";
      }
    } else if (composerRole) {
      trace.composer.status = "skipped_deadline";
    }

    trace.active_calls_final = activeCallCount;
    if (!composerResult && adaptiveResult.lastValidCandidate) {
      composerResult = {
        schema: "scene_composition.v1",
        segments: deepClone(asObject(adaptiveResult.lastValidCandidate.segments)),
        recovered_from_candidate_id: safeString(adaptiveResult.lastValidCandidate.candidate_id),
        recovered_from_role_id: safeString(adaptiveResult.lastValidCandidate.role_id),
      };
      trace.composer.status = "last_valid_scene_candidate";
      trace.composer.last_valid_candidate_recovered = true;
    }
    if (!composerResult) {
      return {
        directorResult,
        composerResult: null,
        semanticJudgment,
        fusionPlan,
        proverRole,
        proverProfile,
        failureReason: deadline.check() ? "composer_deadline" : "composer_failed",
      };
    }
    return {
      directorResult,
      composerResult,
      semanticJudgment,
      fusionPlan,
      proverRole,
      proverProfile,
      failureReason: "",
    };
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
  let completedOutputCache = [];

  function outputSettingsFingerprint(settings) {
    const roleProfiles = asObject(settings && settings.role_profiles);
    const roleConfig = Object.keys(roleProfiles).sort().map((roleId) => {
      const profile = asObject(roleProfiles[roleId]);
      return {
        role_id: roleId,
        enabled: profile.enabled !== false,
        provider: safeString(profile.provider),
        endpoint: safeString(profile.endpoint),
        model: safeString(profile.model),
        temperature: Number(profile.temperature) || 0,
        max_output_tokens: Number(profile.max_output_tokens) || 0,
        timeout_ms: Number(profile.timeout_ms) || 0,
        prompt_digest: stableDigest(safeString(profile.system_prompt)),
        fallback_provider: safeString(profile.fallback_provider),
        fallback_endpoint: safeString(profile.fallback_endpoint),
        fallback_model: safeString(profile.fallback_model),
        reasoning_preset: safeString(profile.reasoning_preset),
        reasoning_effort: safeString(profile.reasoning_effort),
        reasoning_budget_tokens: Number(profile.reasoning_budget_tokens) || 0,
        extra_headers_digest: stableDigest(safeString(profile.extra_headers)),
        extra_body_digest: stableDigest(safeString(profile.extra_body)),
        api_key_ref_digest: stableDigest(safeString(profile.api_key_ref)),
        fallback_api_key_ref_digest: stableDigest(safeString(profile.fallback_api_key_ref)),
      };
    });
    return stableDigest({
      version: VERSION,
      preset: safeString(settings && settings.preset),
      max_parallel: Number(settings && settings.max_parallel) || 0,
      context_char_limit: Number(settings && settings.context_char_limit) || 0,
      protected_regex_digest: stableDigest(safeString(settings && settings.protected_regex)),
      roles: roleConfig,
    });
  }

  function outputRequestIdentityDigest(snapshot) {
    if (!snapshot || !snapshot.turn_contract) return "";
    const contract = deepClone(asObject(snapshot.turn_contract));
    const manifest = deepClone(asObject(snapshot.context_manifest));
    delete contract.contract_id;
    delete contract.contract_digest;
    delete contract.source_snapshot_id;
    delete manifest.snapshot_id;
    return stableDigest({
      original_messages: arrayFromCollection(snapshot.original_messages),
      turn_contract: contract,
      context_manifest: manifest,
    });
  }

  function outputReuseKeys(rawOutput, type, settings, snapshot) {
    const base = {
      request_type: safeString(type).toLowerCase(),
      raw_output_digest: stableDigest(safeString(rawOutput)),
      settings_digest: outputSettingsFingerprint(settings),
    };
    const contractDigest = outputRequestIdentityDigest(snapshot);
    return {
      strict: contractDigest ? stableDigest(Object.assign({}, base, { contract_digest: contractDigest })) : "",
      contract_digest: contractDigest,
    };
  }

  function pruneCompletedOutputCache(now) {
    const timestamp = Number(now) || Date.now();
    completedOutputCache = completedOutputCache
      .filter((entry) => timestamp - entry.created_at <= OUTPUT_REUSE_TTL_MS)
      .slice(-OUTPUT_REUSE_LIMIT);
  }

  function findCompletedOutputReuse(rawOutput, type, settings, snapshot) {
    const now = Date.now();
    pruneCompletedOutputCache(now);
    const keys = outputReuseKeys(rawOutput, type, settings, snapshot);
    for (let index = completedOutputCache.length - 1; index >= 0; index--) {
      const entry = completedOutputCache[index];
      const ageMs = now - entry.created_at;
      if (keys.strict && entry.strict_key === keys.strict) {
        return Object.assign({ age_ms: ageMs, match: "strict" }, entry);
      }
    }
    return null;
  }

  function storeCompletedOutputReuse(rawOutput, type, settings, snapshot, finalOutput, trace, finalSegments) {
    const keys = outputReuseKeys(rawOutput, type, settings, snapshot);
    const entry = {
      strict_key: keys.strict,
      created_at: Date.now(),
      output: safeString(finalOutput),
      trace: cloneSnapshotValue(trace),
      final_segments: cloneSnapshotValue(finalSegments || []),
    };
    if (!entry.strict_key) return;
    completedOutputCache = completedOutputCache.filter((item) =>
      item.strict_key !== entry.strict_key
    );
    completedOutputCache.push(entry);
    pruneCompletedOutputCache(entry.created_at);
  }

  function updateLatestAppliedComparison(trace, finalSegments, originalText, returnedText) {
    const appliedBySegment = {};
    arrayFromCollection(trace && trace.applied_evidence).forEach((item) => {
      appliedBySegment[safeString(item.segment_id)] = item;
    });
    const changes = (finalSegments || []).filter((segment) =>
      segment.type === "mutable"
      && safeString(segment.final_text) !== safeString(segment.original_text)
    ).map((segment) => ({
      segment_id: segment.id,
      source: segment.source || "original",
      operation: segment.operation || "none",
      applied_role_id: segment.applied_role_id || "",
      material_change: !!segment.material_change,
      role_contributions: arrayFromCollection(
        asObject(appliedBySegment[segment.id]).role_contributions
      ),
      original_text: safeString(segment.original_text),
      final_text: safeString(segment.final_text),
    }));
    latestAppliedComparison = {
      timestamp: trace && trace.timestamp ? trace.timestamp : Date.now(),
      state: safeString(trace && trace.summary && trace.summary.final_state) || "unchanged",
      reason: safeString(trace && trace.final && trace.final.reason),
      changes,
      original_text: safeString(originalText),
      returned_text: safeString(returnedText),
      applied: changes.length > 0,
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
        ? createCompletionDeadline(Date.now(), COMPLETION_WAIT_WATCHDOG_MS)
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
    let comparisonOriginalText = "";
    let comparisonReturnedText = "";
    let lastReturnableAssembly = null;
    let lastReturnableComposerResult = null;
    let lastReturnableVerification = null;

    function rememberReturnableAssembly(assembled, composerResult, verification) {
      if (!assembled || !verification || !verification.pass
          || !assembled.changed || !safeString(assembled.output).trim()) return false;
      lastReturnableAssembly = assembled;
      lastReturnableComposerResult = composerResult;
      lastReturnableVerification = verification;
      comparisonFinalSegments = assembled.finalSegments;
      comparisonReturnedText = assembled.output;
      return true;
    }

    try {
      const settings = await loadSettings();
      _cachedTraceEnabled = settings.trace_enabled !== false;

      if (isAuxiliaryRequest(type)) {
        setFinalTraceState(trace, false, "bypassed", "auxiliary_request_bypass");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }
      comparisonTurn = true;

      trace.content_type = typeof content;
      trace.content_chars = safeString(content).length;
      if (typeof content !== "string") {
        if (content && typeof content === "object" && typeof content.content === "string") {
          content = content.content;
        } else {
          setFinalTraceState(trace, false, "bypassed", "non_string_content_bypass");
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }
      }

      const rawOriginalText = safeString(content);
      const visibleOutput = extractVisibleAssistantOutput(rawOriginalText);
      trace.visible_output = {
        raw_chars: rawOriginalText.length,
        visible_chars: visibleOutput.text.length,
        removed_block_count: visibleOutput.removed_block_count,
        removed_chars: visibleOutput.removed_chars,
        ambiguous_unclosed: visibleOutput.ambiguous_unclosed,
      };
      content = visibleOutput.text;
      const originalText = visibleOutput.text;
      comparisonOriginalText = originalText;
      comparisonReturnedText = originalText;
      if (!originalText.trim()) {
        setFinalTraceState(
          trace, false, "bypassed",
          visibleOutput.removed_block_count > 0
            ? "reasoning_only_no_visible_output"
            : "empty_input"
        );
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      const completedReuse = findCompletedOutputReuse(
        rawOriginalText,
        type,
        settings,
        pendingMainSnapshot
      );
      if (completedReuse) {
        if (pendingMainSnapshot) consumePendingSnapshot(type);
        const cachedTrace = cloneSnapshotValue(completedReuse.trace || {});
        Object.assign(trace, cachedTrace);
        trace.timestamp = Date.now();
        trace.stage = "afterRequest";
        trace.request_type = safeString(type);
        trace.output_reuse = {
          hit: true,
          key: completedReuse.match,
          source_trace_timestamp: Number(completedReuse.created_at) || 0,
          age_ms: Math.max(0, Number(completedReuse.age_ms) || 0),
        };
        traceTimeline(trace, "output_reuse_hit");
        comparisonTurn = true;
        comparisonFinalSegments = cloneSnapshotValue(completedReuse.final_segments || []);
        comparisonReturnedText = safeString(completedReuse.output);
        if (settings_trace_enabled()) await saveTrace(trace);
        return safeString(completedReuse.output);
      }

      const completionWait = presetUsesCompletionWait(settings.preset);
      const deadlineMs = completionWait
        ? COMPLETION_WAIT_WATCHDOG_MS
        : resolvePipelineDeadlineMs(settings);
      trace.deadline_ms = deadlineMs;
      trace.scheduler.completion_wait = completionWait;
      deadline = completionWait
        ? createCompletionDeadline(pipelineStartedAt, deadlineMs)
        : createDeadline(deadlineMs, pipelineStartedAt);

      const snapshot = consumePendingSnapshot(type);
      trace.streaming = snapshot.streaming;
      if (snapshot.input_trace && snapshot.input_trace.input_enhance) {
        trace.input_enhance = cloneSnapshotValue(snapshot.input_trace.input_enhance);
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

      traceTimeline(trace, "context_start");
      const context = snapshot.context
        || await collectContext(snapshot.original_messages || snapshot.messages, settings, trace, deadline);
      const snapshotUserInput = safeString(
        context && context.latest_user_input
          ? context.latest_user_input
          : snapshot.context_manifest && snapshot.context_manifest.latest_user_input
      );
      const archiveEnvelope = readArchiveCenterEnhancement(snapshotUserInput);
      if (archiveEnvelope) {
        applyArchiveCenterEnhancement(context, archiveEnvelope, settings, trace);
      }
      const effectiveTurnContract = attachArchiveCenterContextToTurnContract(
        snapshot.turn_contract,
        context && context.manifest
      );
      trace.snapshot.contract_id = safeString(effectiveTurnContract && effectiveTurnContract.contract_id);
      trace.snapshot.contract_digest = safeString(effectiveTurnContract && effectiveTurnContract.contract_digest);
      traceTimeline(trace, "context_done");

      traceTimeline(trace, "segment_start");
      const physicalSegments = buildSegmentMap(originalText, settings);
      const segSummary = summarizeSegments(physicalSegments);
      const rewriteFrame = buildSceneRewriteFrame(physicalSegments, originalText);
      const segments = rewriteFrame.logical_segments;
      trace.segments = segSummary;
      trace.rewrite_frame = {
        schema: rewriteFrame.schema,
        logical_mutable_count: segments.length,
        placeholder_count: rewriteFrame.tokens.length,
        physical_segment_count: physicalSegments.length,
      };
      const draftLedger = buildDraftLedger(originalText, physicalSegments, effectiveTurnContract);
      trace.draft_ledger = summarizeDraftLedger(draftLedger);
      trace.lineage.draft_zero_digest = safeString(draftLedger.draft_digest);
      traceTimeline(trace, "segment_done");

      if (segSummary.mutable === 0) {
        setFinalTraceState(trace, false, "bypassed", "no_mutable_segments");
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

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
      const signals = detectSceneSignals(physicalSegments, context, draftLedger);
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
        const scheduled = await scheduleRoles(
          selectedRoles, settings.role_profiles, segments,
          rewriteContextBlock, segments, deadline, trace, settings.max_parallel,
          draftLedger
        );
        const directorResult = scheduled.directorResult;
        let composerResult = scheduled.composerResult;
        const proverRole = scheduled.proverRole;
        const proverProfile = scheduled.proverProfile;
        const failureReason = scheduled.failureReason;
        traceTimeline(trace, "schedule_done");
        refreshTraceSummary(trace, segSummary, composerResult);
        if (failureReason) {
          setFinalTraceState(trace, false, "rejected", failureReason, {
            material_rewrite: false,
            semantic_verified: null,
            quality_preferred: null,
          });
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }

        traceTimeline(trace, "assemble_start");
        let assembled = assembleSceneRewriteFrame(rewriteFrame, composerResult);
        traceTimeline(trace, "assemble_done");
        traceTimeline(trace, "verify_start");
        let verification = assembled.frame_error
          ? { pass: false, errors: [`scene_frame_restore_failed:${assembled.frame_error}`], warnings: [] }
          : verifyOutput(
              physicalSegments,
              assembled.physicalFinalSegments,
              assembled.output,
              originalText,
              settings
            );
        traceTimeline(trace, "verify_done");
        rememberReturnableAssembly(assembled, composerResult, verification);

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
              judge_verdict: top.judge_verdict || "",
              top_role: top.role_id,
              candidate_count: ranked.length,
            });
          });
        }

        /* ── R4: Final summary ── */
        const preProofFailureReasons = [];
        if (assembled.frame_error) {
          preProofFailureReasons.push(`scene_frame_restore_failed:${assembled.frame_error}`);
        } else if (!verification.pass) {
          preProofFailureReasons.push(...verification.errors.map((item) => `verifier:${item}`));
        }
        preProofFailureReasons.push(...arrayFromCollection(verification.warnings)
          .map((item) => `verifier_warning:${item}`));
        if (!assembled.changed || assembled.output === originalText) {
          preProofFailureReasons.push(
            assembled.changed ? "composer_output_identical" : "composer_returned_original"
          );
        }
        if (preProofFailureReasons.length && !deadline.check()) {
          const recovered = await runPreProofComposerRecovery({
            reason_codes: preProofFailureReasons,
            deadline,
            trace,
            composer_role: DEFAULT_ROLES.find((role) => role.role_id === COMPOSER_ROLE_ID),
            composer_profile: settings.role_profiles[COMPOSER_ROLE_ID],
            composer_result: composerResult,
            segments,
            mutable_segments: mutableSegments(segments).filter(
              (segment) => mutableCoreText(segment).trim().length > 0
            ),
            director_result: directorResult,
            context_block: rewriteContextBlock,
            rewrite_frame: rewriteFrame,
            physical_segments: physicalSegments,
            original_text: originalText,
            settings,
          });
          if (recovered) {
            composerResult = recovered.composerResult;
            assembled = recovered.assembled;
            verification = recovered.verification;
          }
        }

        if ((!verification.pass || !assembled.changed || assembled.output === originalText)
            && directorResult && directorResult.last_valid_candidate) {
          const candidateFallbackResult = {
            schema: "scene_composition.v1",
            segments: deepClone(asObject(directorResult.last_valid_candidate.segments)),
            recovered_from_candidate_id: safeString(directorResult.last_valid_candidate.candidate_id),
            recovered_from_role_id: safeString(directorResult.last_valid_candidate.role_id),
          };
          const candidateFallbackAssembly = assembleSceneRewriteFrame(
            rewriteFrame,
            candidateFallbackResult
          );
          const candidateFallbackVerification = candidateFallbackAssembly.frame_error
            ? { pass: false, errors: [`scene_frame_restore_failed:${candidateFallbackAssembly.frame_error}`], warnings: [] }
            : verifyOutput(
                physicalSegments,
                candidateFallbackAssembly.physicalFinalSegments,
                candidateFallbackAssembly.output,
                originalText,
                settings
              );
          if (candidateFallbackVerification.pass
              && candidateFallbackAssembly.changed
              && candidateFallbackAssembly.output !== originalText) {
            composerResult = candidateFallbackResult;
            assembled = candidateFallbackAssembly;
            verification = candidateFallbackVerification;
            trace.composer.status = "last_valid_scene_candidate";
            trace.composer.last_valid_candidate_recovered = true;
          }
        }

        rememberReturnableAssembly(assembled, composerResult, verification);

        if (!verification.pass) {
          const reason = `verifier_failed:${verification.errors.join(",")}`;
          holdAttemptedEvidence(trace, assembled);
          setFinalTraceState(trace, false, "rejected", reason, {
            material_rewrite: assembled.materialComposerApplied > 0,
            semantic_verified: null,
            quality_preferred: null,
          });
          traceError(trace, reason);
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }

        if (!assembled.changed || assembled.output === originalText) {
          holdAttemptedEvidence(trace, assembled);
          setFinalTraceState(
            trace, false, "unchanged",
            assembled.changed ? "composer_output_identical" : "composer_returned_original",
            {
              material_rewrite: false,
              semantic_verified: null,
              quality_preferred: null,
            }
          );
          if (settings_trace_enabled()) await saveTrace(trace);
          return content;
        }

        const proverConfigured = !!(
          proverRole && proverProfile && proverProfile.enabled && isProfileConfigured(proverProfile)
        );
        let semanticProof = null;
        if (!proverConfigured || deadline.check()) {
          const reason = deadline.check() ? "semantic_prover_deadline" : "semantic_prover_not_configured";
          semanticProof = degradedSemanticProof(null, [reason]);
          trace.semantic_prover.status = "degraded";
          trace.semantic_prover.verdict = "degraded";
          trace.semantic_prover.reason = reason;
        } else {
          traceTimeline(trace, "semantic_proof_start");
          semanticProof = await runSemanticProver(
            proverRole,
            proverProfile,
            rewriteFrame.semantic_segments,
            assembled.finalSegments,
            directorResult,
            rewriteContextBlock,
            deadline.signal,
            trace,
            !!deadline.completion_wait,
            "initial"
          );
          traceTimeline(trace, "semantic_proof_done");
          if (!semanticProof) {
            semanticProof = degradedSemanticProof(null, ["semantic_prover_failed"]);
            trace.semantic_prover.status = "degraded";
            trace.semantic_prover.verdict = "degraded";
            trace.semantic_prover.reason = "semantic_prover_failed";
          }
        }

        let proofReturnAssessment = assessSemanticProofReturn(semanticProof);
        if (!semanticProof.returnable_degraded
            && semanticProof.verdict !== "pass"
            && !proofReturnAssessment.returnable) {
          semanticProof = synthesizeWholeSceneRepairProof(
            semanticProof,
            rewriteFrame.semantic_segments
          );
          if (semanticProof && semanticProof.repair_synthesized) {
            trace.semantic_prover.verdict = "repair";
            trace.semantic_prover.repair_synthesized = true;
            trace.semantic_prover.reason = arrayFromCollection(semanticProof.reason_codes).join(",");
          }
        }

        if (semanticProof.verdict === "repair") {
          trace.semantic_prover.repair_attempted = true;
          if (deadline.check()) {
            semanticProof = degradedSemanticProof(semanticProof, ["semantic_repair_deadline"]);
          } else {
            trace.budget.prover_attempt_reserved = trace.semantic_prover.structured_recovery_attempted
              ? 1
              : 2;
            const previousSegments = {};
            assembled.logicalFinalSegments.filter((segment) => segment.type === "mutable").forEach((segment) => {
              previousSegments[segment.id] = segment.final_text;
            });
            traceTimeline(trace, "semantic_repair_start");
            const repairScope = createPipelineStageScope(
              deadline, "semantic_repair_composer", trace
            );
            let repairedComposerResult;
            try {
              repairedComposerResult = await runComposer(
                DEFAULT_ROLES.find((role) => role.role_id === COMPOSER_ROLE_ID),
                settings.role_profiles[COMPOSER_ROLE_ID],
                segments,
                directorResult,
                rewriteContextBlock,
                repairScope.signal,
                trace,
                mutableSegments(segments).filter((segment) => mutableCoreText(segment).trim().length > 0),
                !!deadline.completion_wait,
                {
                  repair_instructions: semanticProof.repair_instructions,
                  previous_segments: previousSegments,
                  reason_codes: semanticProof.reason_codes,
                }
              );
            } finally {
              repairScope.cancel();
            }
            traceTimeline(trace, "semantic_repair_done");
            if (!repairedComposerResult) {
              semanticProof = degradedSemanticProof(semanticProof, ["semantic_repair_composer_failed"]);
            } else {
              const repairedAssembly = assembleSceneRewriteFrame(rewriteFrame, repairedComposerResult);
              const repairedVerification = repairedAssembly.frame_error
                ? { pass: false, errors: [`scene_frame_restore_failed:${repairedAssembly.frame_error}`], warnings: [] }
                : verifyOutput(
                    physicalSegments,
                    repairedAssembly.physicalFinalSegments,
                    repairedAssembly.output,
                    originalText,
                    settings
                  );
              if (rememberReturnableAssembly(repairedAssembly, repairedComposerResult, repairedVerification)) {
                composerResult = repairedComposerResult;
                assembled = repairedAssembly;
                verification = repairedVerification;
                trace.budget.prover_attempt_reserved = trace.semantic_prover.structured_recovery_attempted
                  ? 1
                  : 2;
                if (proverConfigured && !deadline.check()) {
                  traceTimeline(trace, "semantic_reproof_start");
                  const repairedProof = await runSemanticProver(
                    proverRole,
                    proverProfile,
                    rewriteFrame.semantic_segments,
                    assembled.finalSegments,
                    directorResult,
                    rewriteContextBlock,
                    deadline.signal,
                    trace,
                    !!deadline.completion_wait,
                    "after_repair"
                  );
                  traceTimeline(trace, "semantic_reproof_done");
                  semanticProof = repairedProof
                    || degradedSemanticProof(semanticProof, ["semantic_reproof_failed"]);
                } else {
                  semanticProof = degradedSemanticProof(semanticProof, ["semantic_reproof_unavailable"]);
                }
              } else {
                holdAttemptedEvidence(trace, repairedAssembly);
                semanticProof = degradedSemanticProof(semanticProof, [
                  repairedVerification.pass
                    ? "semantic_repair_returned_original"
                    : `semantic_repair_verifier_failed:${arrayFromCollection(repairedVerification.errors).join(",")}`,
                ]);
              }
            }
          }
        }

        proofReturnAssessment = assessSemanticProofReturn(semanticProof);
        if (!proofReturnAssessment.returnable) {
          semanticProof = degradedSemanticProof(semanticProof, proofReturnAssessment.blocking_reasons.length
            ? proofReturnAssessment.blocking_reasons
            : ["semantic_proof_not_passed"]);
          proofReturnAssessment = assessSemanticProofReturn(semanticProof);
        }

        if (proofReturnAssessment.degraded || semanticProof.returnable_degraded) {
          semanticProof.returnable_degraded = true;
          semanticProof.quality_debt = uniqueList(arrayFromCollection(semanticProof.quality_debt)
            .concat(proofReturnAssessment.quality_debt));
          trace.semantic_prover.verdict = "degraded";
          trace.semantic_prover.quality_debt = semanticProof.quality_debt;
          trace.semantic_prover.reason = semanticProof.quality_debt.length
            ? semanticProof.quality_debt.join(",")
            : "semantic_proof_degraded";
        }

        updateAppliedEvidence(trace, assembled, verification, semanticProof);
        const finalClassification = classifyAppliedOutput(assembled, semanticProof);
        setFinalTraceState(
          trace,
          finalClassification.enhanced,
          finalClassification.state,
          finalClassification.reason,
          finalClassification
        );
        trace.summary.changed_segment_count = trace.applied_evidence.filter((item) => item.changed).length;
        trace.summary.material_changed_segment_count = trace.applied_evidence.filter(
          (item) => item.changed && item.material_change && !item.original_meta_only
        ).length;
        trace.summary.unchanged_segment_count = trace.applied_evidence.length - trace.summary.changed_segment_count;
        trace.final.original_preview = preview(originalText, 200);
        trace.final.final_preview = preview(assembled.output, 200);
        trace.lineage.composer_output_digest = stableDigest(composerResult.segments);
        trace.lineage.proof_digest = stableDigest(semanticProof);
        trace.lineage.returned_output_digest = stableDigest(assembled.output);
        comparisonFinalSegments = assembled.finalSegments;
        comparisonReturnedText = assembled.output;

        try {
          storeCompletedOutputReuse(
            rawOriginalText,
            type,
            settings,
            snapshot,
            assembled.output,
            trace,
            assembled.finalSegments
          );
        } catch (cacheError) {
          traceError(trace, `output_reuse_store_failed:${safeString(cacheError && cacheError.message)}`);
        }
        if (settings_trace_enabled()) await saveTrace(trace);
        return assembled.output;
      } finally {
        if (deadline) deadline.cancel();
      }
    } catch (err) {
      warn("afterRequest pipeline error:", err);
      traceError(trace, `pipeline_error: ${safeString(err && err.message)}`);
      if (lastReturnableAssembly && lastReturnableVerification) {
        const degradedProof = degradedSemanticProof(null, ["pipeline_error_after_rewrite"]);
        updateAppliedEvidence(
          trace,
          lastReturnableAssembly,
          lastReturnableVerification,
          degradedProof
        );
        const classification = classifyAppliedOutput(lastReturnableAssembly, degradedProof);
        setFinalTraceState(
          trace,
          classification.enhanced,
          classification.state,
          "changed_scene_returned_after_pipeline_error",
          classification
        );
        trace.final.original_preview = preview(comparisonOriginalText, 200);
        trace.final.final_preview = preview(lastReturnableAssembly.output, 200);
        trace.lineage.composer_output_digest = stableDigest(
          lastReturnableComposerResult && lastReturnableComposerResult.segments
        );
        trace.lineage.returned_output_digest = stableDigest(lastReturnableAssembly.output);
        comparisonFinalSegments = lastReturnableAssembly.finalSegments;
        comparisonReturnedText = lastReturnableAssembly.output;
        if (settings_trace_enabled()) await saveTrace(trace);
        return lastReturnableAssembly.output;
      }
      setFinalTraceState(trace, false, "failed", "pipeline_error");
      if (settings_trace_enabled()) await saveTrace(trace);
      return content;
    } finally {
      if (deadline) deadline.cancel();
      if (comparisonTurn) {
        updateLatestAppliedComparison(
          trace,
          comparisonFinalSegments,
          comparisonOriginalText,
          comparisonReturnedText
        );
      }
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
            <label>이미지·상태·코드 원문 패턴 <span>Exact artifact regex</span></label>
            <input type="text" class="recomposer-protected-regex" value="${escapeHtml(settings.protected_regex)}" placeholder="원문 그대로 다시 끼워 넣을 사용자 정의 패턴">
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
      const roleStage = role.is_input_planner
        ? "input"
        : (role.is_judge ? "judge" : (role.is_composer ? "composer" : (role.is_prover ? "prover" : "specialist")));
      const roleStageLabel = role.is_input_planner
        ? "INPUT"
        : (role.is_judge ? "JUDGE" : (role.is_composer ? "COMPOSER" : (role.is_prover ? "PROVER" : "REWRITE")));
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
        <div><span>01</span><strong>입력 계획</strong><small>설정, 기억, 현재 장면을 턴 계약으로 정리</small></div>
        <div><span>02</span><strong>장면 재작성</strong><small>세 전문 역할이 완성된 장면 후보를 각각 생성</small></div>
        <div><span>03</span><strong>Semantic Judge</strong><small>근거, 누락, 충돌, 비밀과 시점 위반을 후보별 판정</small></div>
        <div><span>04</span><strong>Fusion Composer</strong><small>승인된 요소와 Fusion Plan으로 최종 장면을 재구성</small></div>
        <div><span>05</span><strong>Semantic Prover</strong><small>최종문 전체가 사실, beat, 비밀, 시점, agency 계약을 지켰는지 증명</small></div>
      </div>      <div class="recomposer-roles-list">${rows}</div>`;
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
      if (!snapshot.original_text && !snapshot.returned_text) {
        return `<div class="recomposer-compare-empty">최근 main 턴에는 실제 적용된 문장 변경이 없습니다. (${escapeHtml(snapshot.reason || snapshot.state)})</div>`;
      }
      return `
        <div class="recomposer-compare-entry">
          <div class="recomposer-compare-head">
            <strong>${new Date(snapshot.timestamp).toLocaleString()}</strong>
            <span>변경 미적용 · ${escapeHtml(snapshot.reason || snapshot.state)}</span>
          </div>
          <section class="recomposer-compare-item">
            <div class="recomposer-compare-meta">
              <span>${SCENE_REWRITE_SEGMENT_ID}</span>
              <span>변경 결과 없음</span>
              <span>실제 변경 0개</span>
            </div>
            <div class="recomposer-compare-grid">
              <div class="recomposer-compare-column">
                <span class="recomposer-compare-label">전</span>
                <pre>${escapeHtml(snapshot.original_text)}</pre>
              </div>
              <div class="recomposer-compare-column is-after">
                <span class="recomposer-compare-label">후 (반환본)</span>
                <pre>${escapeHtml(snapshot.returned_text)}</pre>
              </div>
            </div>
          </section>
        </div>`;
    }
    const items = snapshot.changes.map((change) => {
      const finalText = change.operation === "delete" && !change.final_text
        ? "(삭제됨)"
        : change.final_text;
      const contributionLabels = arrayFromCollection(change.role_contributions)
        .map((item) => safeString(item.role_id))
        .filter(Boolean);
      return `
        <section class="recomposer-compare-item">
          <div class="recomposer-compare-meta">
            <span>${escapeHtml(change.segment_id)}</span>
            <span>${escapeHtml(change.source)}</span>
            ${change.applied_role_id ? `<span>${escapeHtml(change.applied_role_id)}</span>` : ""}
            ${contributionLabels.map((roleId) => `<span>${escapeHtml(roleId)}</span>`).join("")}
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
    const providerEndpointCount = new Set(configuredRoles.map((role) => (
      executionGroupKey(settings.role_profiles[role.role_id])
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
        .stage-judge .recomposer-stage-badge {
          border-color: rgba(225,88,166,0.24);
          background: rgba(225,88,166,0.08);
          color: #ef9ac9;
        }
        .stage-prover .recomposer-stage-badge {
          border-color: rgba(143,167,255,0.28);
          background: rgba(143,167,255,0.09);
          color: #b7c5ff;
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
                <strong>AC Recomposer Agent</strong>
                <small>${RELEASE_LABEL} · ${escapeHtml(BUILD_MARKER)}</small>
              </span>
            </div>
            <div class="recomposer-trust"><span class="recomposer-trust-dot"></span>Read-only context · local trace</div>
          </div>
        </section>
        <div class="recomposer-metric-grid">
          <div class="recomposer-metric"><span>프리셋</span><strong>${escapeHtml(preset.label)}</strong></div>
          <div class="recomposer-metric"><span>준비된 역할</span><strong>${configuredRoles.length} / ${settings.roles.length}</strong></div>
          <div class="recomposer-metric"><span>Input Planner</span><strong>${configuredInputRoles.length} ready</strong></div>
          <div class="recomposer-metric"><span>Provider endpoints</span><strong>${providerEndpointCount || 0} connected</strong></div>
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
            `${attempt.kind}:${attempt.status}${attempt.error_class ? "/" + attempt.error_class : ""}${attempt.structured_transport ? "/via:" + attempt.structured_transport : ""}${attempt.endpoint_group ? " group:" + attempt.endpoint_group : ""}${attempt.transport ? "@" + attempt.transport : ""}${attempt.endpoint ? "(" + attempt.endpoint + ")" : ""}${attempt.failure_preview ? ' preview:"' + attempt.failure_preview + '"' : ""}`
          ).join(",");
          const reasoningLabel = overrides.reasoning_family
            ? ` reasoning:${overrides.reasoning_family}${(overrides.reasoning_fields || []).length ? "/" + overrides.reasoning_fields.join(",") : ""}`
            : "";
          const transportLabel = overrides.transport ? ` transport:${overrides.transport}` : "";
          const validationSummary = (r.validation_diagnostics || [])
            .map((item) => `${item.code}${item.field ? "@" + item.field : ""}${item.segment_ids && item.segment_ids.length ? ":" + item.segment_ids.join(",") : ""}${item.detail ? "(" + item.detail + ")" : ""}`)
            .join(",");
          return `[${r.stage || "output"}] ${r.role_id}: ${r.status} (${r.provider}/${r.model}${r.endpoint_group ? " @ " + r.endpoint_group : ""}) queue:${queueMs}ms run:${r.elapsed_ms}ms http:${r.http_attempts || 0}${r.retry ? " retry:" + r.retry : ""}${r.fallback ? " fallback" : ""}${r.candidate_count ? " cand:" + r.candidate_count : ""}${attemptSummary ? " [" + attemptSummary + "]" : ""}${transportLabel}${reasoningLabel}${validationSummary ? " normalized:[" + validationSummary + "]" : ""}${appliedOverrideKeys.length ? " override+:" + appliedOverrideKeys.join(",") : ""}${skippedOverrideKeys.length ? " override-skip:" + skippedOverrideKeys.join(",") : ""}${r.error_class ? " class:" + r.error_class : ""}${r.error ? " ERR:" + r.error : ""}`;
        }).join("\n");
        const directorLines = (t.director_evidence || []).map((d) =>
          `${d.segment_id}: candidate=${d.top_role} verdict=${d.judge_verdict || "accepted"} issues=[${(d.issue_groups || []).join(",")}] cand=${d.candidate_count}`
        ).join("\n");
        const appliedLines = (t.applied_evidence || []).map((a) => {
          const contributions = arrayFromCollection(a.role_contributions).map((item) =>
            `${item.role_id}/${item.contribution_id} evidence:"${item.evidence_quote}"`
          ).join(" | ");
          return `${a.segment_id}: ${a.source}${a.changed ? " [CHANGED]" : " [unchanged]"}${a.material_change ? " [MATERIAL]" : (a.changed ? " [MINOR]" : "")}${a.composer_unchanged ? " (composer:identical)" : ""} orig:"${escapeHtml(a.original_preview || "")}" → final:"${escapeHtml(a.final_preview || "")}"${contributions ? ` contributions:[${contributions}]` : ""}`;
        }).join("\n");
        const attemptedLines = (t.attempted_evidence || []).filter((a) => a.changed).map((a) =>
          `${a.segment_id}: ${a.source}${a.material_change ? " [MATERIAL]" : " [MINOR]"} orig:"${escapeHtml(a.original_preview || "")}" → attempted:"${escapeHtml(a.final_preview || "")}"`
        ).join("\n");
        const s = t.summary || {};
        const ie = t.input_enhance || {};
        const finalEvidence = t.final || {};
        const ledger = t.draft_ledger || {};
        const judge = t.semantic_judge || {};
        const prover = t.semantic_prover || {};
        const plan = t.fusion_plan || {};
        const revision = t.revision_convergence || {};
        const outputReuse = t.output_reuse || {};
        const lineage = t.lineage || {};
        const archive = t.archive_center || {};
        const sourceSummary = Object.keys(ie.source_availability || {}).map((key) => {
          const source = ie.source_availability[key] || {};
          return `${key}:${source.available ? "used" : "missing"}${source.count ? "/" + source.count : ""}${source.active_count ? "/active:" + source.active_count : ""}${source.unknown_activation_count ? "/unknown:" + source.unknown_activation_count : ""}`;
        }).join(", ");
        const inputLine = `state:${ie.status || "not_run"} contract:${ie.contract_id || "none"} digest:${ie.contract_digest || "none"} planners:${ie.planner_succeeded || 0}ok/${ie.planner_failed || 0}fail injected:${ie.injected_chars || 0} chars active:${ie.active_calls_final || 0} retry-reuse:${ie.retry_reuse_count || 0} transport-cancel:${ie.transport_cancellation || "not_requested"}${ie.fallback_reason ? " fallback:" + ie.fallback_reason : ""}`;
        const summaryLine = `specialists:${s.specialist_calls} successful:${s.successful_roles} http:${s.specialist_http_calls || 0} scene-candidates:${s.candidate_count} composer:${s.composer_state} changed:${s.changed_segment_count} material-segments:${s.material_changed_segment_count || 0} material-rewrite:${s.material_rewrite === true} semantic:${s.semantic_verified || "not_run"} quality:${s.quality_preferred || "not_run"} unchanged:${s.unchanged_segment_count} state:${s.final_state} reason:${escapeHtml(s.final_reason || "")}`;
        const ledgerLine = `digest:${ledger.digest || "none"} facts:${ledger.established_facts || 0} beats:${ledger.scene_beats || 0} hooks:${ledger.unresolved_hooks || 0} directives:${ledger.response_directives || 0} hard:${ledger.hard_constraints || 0} protected:${ledger.protected_structures || 0} unknown:[${(ledger.unknown_semantics || []).join(",")}]`;
        const judgeLine = `status:${judge.status || "not_run"} accept:${judge.accepted_candidates || 0} constrained:${judge.constrained_candidates || 0} reject:${judge.rejected_candidates || 0} contributions:${judge.required_contributions || 0} unresolved:${judge.unresolved_requirements || 0} missing:${judge.missing_facts || 0} unsupported:${judge.unsupported_additions || 0} hard:${judge.hard_violations || 0}`;
        const proverValidation = arrayFromCollection(prover.validation_diagnostics)
          .map((item) => `${item.code}${item.field ? "@" + item.field : ""}${item.detail ? "(" + item.detail + ")" : ""}`)
          .join(",");
        const proverLine = `status:${prover.status || "not_run"} verdict:${prover.verdict || "not_run"} attempts:${prover.attempts || 0} repair:${prover.repair_attempted === true} synthesized:${prover.repair_synthesized === true} structure-recovery:${prover.structured_recovery_attempted === true}/${prover.structured_recovery_succeeded === true} gains:${prover.quality_gains_required || 0}/${prover.quality_gains_missing || 0}/${prover.quality_gains_regressed || 0} residual:${prover.residual_quality_checked || 0}/${prover.residual_quality_issues || 0} debt:${arrayFromCollection(prover.quality_debt).length} facts:${prover.facts_missing || 0}/${prover.facts_contradicted || 0} beats:${prover.beats_missing || 0} constraints:${prover.constraints_violated || 0} unsupported:${prover.unsupported_additions || 0} hard:${prover.hard_violations || 0} contract:${prover.output_contract_failures || 0}${proverValidation ? " validation:[" + proverValidation + "]" : ""}${prover.reason ? " reason:" + prover.reason : ""}`;
        const planLine = `status:${plan.status || "not_run"} accepted:${plan.accepted_candidates || 0} rejected:${plan.rejected_candidates || 0} required:${plan.required_contributions || 0} unresolved:${plan.unresolved_requirements || 0} consensus:${plan.consensus_claims || 0} complement:${plan.complementary_claims || 0} conflicts:${plan.conflicts || 0} prohibited:${plan.prohibited_additions || 0}${plan.direct_composer_fallback ? " direct-composer:true" : ""}${plan.fallback_reason ? " fallback:" + plan.fallback_reason : ""}`;
        const revisionLines = arrayFromCollection(revision.rounds).map((round) =>
          `round:${round.round} role:${round.role_id} status:${round.status} parent:${round.parent_candidate_id || "none"} result:${round.output_candidate_id || "none"} needs:[${arrayFromCollection(round.requirement_ids).join(",")}]${round.failure_reason ? " reason:" + round.failure_reason : ""}`
        ).join("\n");
        const revisionLine = `limit:${revision.limit || 0} attempted:${revision.attempted || 0} fulfilled:${revision.fulfilled || 0} failed:${revision.failed || 0} no-candidate:${revision.no_candidate || 0} remaining:${revision.remaining_requirements || 0} last-valid:${revision.last_valid_candidate_id || "none"}`;
        const lineageLine = `draft:${lineage.draft_zero_digest || "none"} candidates:${arrayFromCollection(lineage.candidate_ids).length} judgments:${arrayFromCollection(lineage.judgment_digests).length} plans:${arrayFromCollection(lineage.plan_ids).length} composer:${lineage.composer_output_digest || "none"} proof:${lineage.proof_digest || "none"} returned:${lineage.returned_output_digest || "none"}`;
        const visible = t.visible_output || {};
        const visibleLine = `raw:${visible.raw_chars || 0} visible:${visible.visible_chars || 0} removed-blocks:${visible.removed_block_count || 0} removed-chars:${visible.removed_chars || 0} ambiguous:${visible.ambiguous_unclosed === true}`;
        const rewriteFrame = t.rewrite_frame || {};
        const rewriteFrameLine = rewriteFrame.schema
          ? `logical-mutable:${rewriteFrame.logical_mutable_count || 0} placeholders:${rewriteFrame.placeholder_count || 0} physical-segments:${rewriteFrame.physical_segment_count || 0}`
          : "not_run";
        const archiveFeatures = Object.keys(archive.features || {}).map((key) => {
          const feature = archive.features[key] || {};
          return `${key}:${feature.status || "empty"}/${feature.selected_count || 0}${feature.call_status ? "/" + feature.call_status : ""}`;
        }).join(", ");
        const archiveLine = archive.detected
          ? `mode:${archive.mode || "archive_center_enhanced"} status:${archive.status || "unknown"} lanes:${archive.lane_count || 0} chars:${archive.evidence_chars || 0} critic-same-turn:${archive.same_turn_critic_result_available === true} features:[${archiveFeatures}]`
          : "mode:standalone status:not_detected";
        const scheduler = t.scheduler || {};
        const schedulerLines = (scheduler.endpoint_groups || []).map((group) =>
          `${group.stage}:${group.endpoint_group} calls:${group.selected_calls} concurrency:${group.effective_concurrency}/${group.base_concurrency} reason:${group.reason}`
        ).join("\n");
        const stageBudgetLines = arrayFromCollection(scheduler.stage_budgets).map((item) =>
          `${item.stage}: budget:${item.budget_ms || 0}ms${item.explicit_cap_ms ? " cap:" + item.explicit_cap_ms + "ms" : ""} remaining-at-start:${item.remaining_at_start_ms || 0}ms mode:${item.completion_wait ? "completion_wait" : "deadline"}`
        ).join("\n");
        const routerLine = (t.router && t.router.signals && t.router.signals.length) ? `signals:[${t.router.signals.join(",")}]` : "";
        return `<div class="recomposer-trace-entry">
          <strong>${escapeHtml(t.stage)} ${new Date(t.timestamp).toLocaleString()}</strong><br>
          Enhanced: ${finalEvidence.enhanced === true} · material:${finalEvidence.material_rewrite === true} · semantic:${escapeHtml(finalEvidence.semantic_verified || "not_run")} · quality:${escapeHtml(finalEvidence.quality_preferred || "not_run")} — ${escapeHtml(finalEvidence.reason || "")}<br>
          Segments: P:${t.segments.protected} I:${t.segments.inspect_only} M:${t.segments.mutable}<br>
          Rewrite Frame: ${escapeHtml(rewriteFrameLine)}<br>
          Visible Output: ${escapeHtml(visibleLine)}<br>
          ${routerLine ? `Router: ${escapeHtml(routerLine)}<br>` : ""}
          Input Enhance: ${escapeHtml(inputLine)}<br>
          Archive Center: ${escapeHtml(archiveLine)}<br>
          Draft Ledger: ${escapeHtml(ledgerLine)}<br>
          Semantic Judge: ${escapeHtml(judgeLine)}<br>
          Fusion Plan: ${escapeHtml(planLine)}<br>
          Adaptive Revision: ${escapeHtml(revisionLine)}<br>
          Output Reuse: ${outputReuse.hit === true ? `hit/${escapeHtml(outputReuse.key || "")}/${outputReuse.age_ms || 0}ms` : "miss"}<br>
          Lineage: ${escapeHtml(lineageLine)}<br>
          Semantic Prover: ${escapeHtml(proverLine)}<br>
          ${sourceSummary ? `Sources: ${escapeHtml(sourceSummary)}<br>` : ""}
          Summary: ${escapeHtml(summaryLine)}<br>
          Scheduler: ${scheduler.completion_wait ? "completion_wait" : "deadline"} · provider concurrency<br>
          Candidates: complete-scenes:${t.candidates.total} segment-variants:${t.candidates.segment_variant_total || 0} incomplete-rejected:${t.candidates.incomplete_rejected_count || 0} normalized:${t.candidates.normalized_field_count || 0} recovery:${t.candidates.structured_recovery_succeeded || 0}/${t.candidates.structured_recovery_attempted || 0}/${t.candidates.structured_recovery_queued || 0} deduped:${t.candidates.duplicate_scene_candidates || 0}<br>
          Composer: ${t.composer.used} (${t.composer.status} ${t.composer.elapsed_ms}ms, reserve:${t.composer.reserve_ms || 0}ms, semantic-retry:${t.composer.semantic_retry || 0}, preproof-recovery:${t.composer.preproof_recovery_attempted ? (t.composer.preproof_recovery_succeeded ? "fulfilled" : "failed") : "not_run"}${t.composer.preproof_recovery_reason ? ":" + escapeHtml(t.composer.preproof_recovery_reason) : ""}${t.composer.specialist_stop_reason ? ", specialist-stop:" + escapeHtml(t.composer.specialist_stop_reason) : ""})<br>
          ${t.final.original_preview ? `Orig: ${escapeHtml(t.final.original_preview)}<br>` : ""}
          ${t.final.final_preview ? `Final: ${escapeHtml(t.final.final_preview)}<br>` : ""}
          ${directorLines ? `<pre style="color:#8FA7FF;">Accepted candidate coverage:\n${escapeHtml(directorLines)}</pre>` : ""}
          ${appliedLines ? `<pre style="color:#8A55F7;">Applied:\n${escapeHtml(appliedLines)}</pre>` : ""}
          ${attemptedLines ? `<pre style="color:#E158A6;">Rejected Composer attempt:\n${escapeHtml(attemptedLines)}</pre>` : ""}
          ${revisionLines ? `<pre style="color:#8FA7FF;">Adaptive revision lineage:\n${escapeHtml(revisionLines)}</pre>` : ""}
          ${schedulerLines ? `<pre>Endpoint groups:\n${escapeHtml(schedulerLines)}</pre>` : ""}
          ${stageBudgetLines ? `<pre>Stage budgets:\n${escapeHtml(stageBudgetLines)}</pre>` : ""}
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
    async function test(name, fn) {
      try {
        const detail = await fn();
        results.push({ name, pass: true, detail: safeString(detail) });
      } catch (err) {
        results.push({ name, pass: false, error: safeString(err && err.message) });
      }
    }
    function candidate(roleId, id, text) {
      return {
        candidate_id: id,
        role_id: roleId,
        supporting_roles: [roleId],
        duplicate_count: 0,
        segments: { mutable_1: text },
        segment_operations: { mutable_1: 'replace' },
        evidence_refs: ['fact_1'],
        retained_beats: [{ id: 'fact_1', text: 'Keep the meeting fact.' }],
        proposed_additions: [],
        addressed_issues: roleId === 'style_reader' ? ['rhythm'] : ['character_voice'],
        confidence: 0.8,
        change_summary: 'material scene rewrite',
      };
    }
    function ledger() {
      return {
        schema: 'draft_ledger.v1',
        established_facts: [{ ledger_id: 'fact_1', text: 'The characters met.', source_ref: 'current_chat' }],
        scene_beats: [{ ledger_id: 'beat_1', text: 'The scene remains in the room.', source_ref: 'draft_zero' }],
        unresolved_hooks: [],
        hard_constraints: [{ ledger_id: 'constraint_1', text: 'Do not reveal the secret.', source_ref: 'turn_contract' }],
        protected_structures: [],
        unknown_semantics: [],
      };
    }
    function judgmentObject(candidateIds, verdict) {
      return {
        schema: 'semantic_judgment.v1',
        candidate_judgments: candidateIds.map((candidateId) => ({
          candidate_id: candidateId,
          verdict: verdict || 'accept',
          preserved_ledger_ids: ['fact_1', 'beat_1', 'constraint_1'],
          missing_ledger_ids: [],
          unsupported_additions: [],
          hard_violations: [],
          accepted_elements: [{ claim: 'Grounded scene improvement', candidate_ids: [candidateId], segment_ids: ['mutable_1'] }],
          rejected_elements: [],
          quality_gains: [{ claim: 'Stronger dramatic movement', candidate_ids: [candidateId] }],
          quality_regressions: [],
        })),
        cross_candidate: { consensus: [], complementary: [], conflicts: [] },
        scene_requirements: { target_arc: 'Escalate the exchange.', target_voice: 'Character-specific.', target_pacing: 'Tight.' },
      };
    }
    function semanticProofObject(verdict) {
      return {
        schema: 'semantic_proof.v1',
        declared_verdict: verdict || 'pass',
        fact_checks: [{ ledger_id: 'fact_1', status: 'preserved', detail: 'Fact remains.', evidence_quote: 'met' }],
        beat_checks: [{ ledger_id: 'beat_1', status: 'preserved', detail: 'Beat remains.', evidence_quote: 'room' }],
        constraint_checks: [{ ledger_id: 'constraint_1', status: 'satisfied', detail: 'Secret remains hidden.', evidence_quote: '' }],
        quality_gain_checks: [],
        residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
          check_id: checkId,
          status: 'clean',
          detail: 'No residual issue found.',
          evidence_quote: '',
          segment_ids: [],
        })),
        hard_violations: [],
        unsupported_additions: [],
        output_contract: {
          language_ok: true,
          turn_boundary_ok: true,
          user_agency_ok: true,
          meta_free: true,
          format_ok: true,
        },
        repair_instructions: [],
      };
    }
    function response(content) {
      return { ok: true, status: 200, text: async () => JSON.stringify({ choices: [{ message: { content } }] }) };
    }
    function sceneCandidateJson(roleId, text, segmentId) {
      const issueByRole = {
        character_reader: 'character_voice',
        plot_continuity_reader: 'scene_logic',
        style_reader: 'rhythm',
        perspective_boundary_rewriter: 'identity_continuity',
      };
      return JSON.stringify({
        schema: 'scene_rewrite_candidates.v1',
        role_id: roleId,
        candidates: [{
          segments: { [segmentId || 'mutable_1']: text },
          evidence_refs: ['fact_1'],
          retained_beats: [{ id: 'fact_1', text: 'Keep the meeting fact.' }],
          proposed_additions: [],
          addressed_issues: [issueByRole[roleId] || 'character_voice'],
          confidence: 0.8,
          change_summary: 'material scene rewrite',
        }],
      });
    }
    function semanticProofJsonFromPrompt(prompt) {
      const ledgerMatch = /--- Draft Ledger \(binding source map\) ---\n(.+?)\n--- End Draft Ledger ---/s.exec(prompt);
      const finalMatch = /Final Composer segments:\n(.+?)\n\nSemantic Judgment:/s.exec(prompt);
      if (!ledgerMatch || !finalMatch) throw new Error('semantic proof mock missing ledger or final scene');
      const proofLedger = JSON.parse(ledgerMatch[1]);
      const finalSegments = JSON.parse(finalMatch[1]);
      const finalText = finalSegments.map((item) => safeString(item && item.text)).join('\n').trim();
      const exactQuote = finalText.slice(0, Math.min(18, finalText.length));
      return JSON.stringify({
        schema: 'semantic_proof.v1',
        declared_verdict: 'pass',
        fact_checks: arrayFromCollection(proofLedger.established_facts).map((item) => ({
          ledger_id: item.ledger_id, status: 'preserved', detail: 'Preserved.', evidence_quote: exactQuote,
        })),
        beat_checks: arrayFromCollection(proofLedger.scene_beats).map((item) => ({
          ledger_id: item.ledger_id, status: 'preserved', detail: 'Preserved.', evidence_quote: exactQuote,
        })),
        constraint_checks: arrayFromCollection(proofLedger.hard_constraints).map((item) => ({
          ledger_id: item.ledger_id, status: 'satisfied', detail: 'Satisfied.', evidence_quote: '',
        })),
        quality_gain_checks: [],
        residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
          check_id: checkId, status: 'clean', detail: 'Clean.', evidence_quote: '', segment_ids: [],
        })),
        hard_violations: [],
        unsupported_additions: [],
        output_contract: {
          language_ok: true, turn_boundary_ok: true, user_agency_ok: true, meta_free: true, format_ok: true,
        },
        repair_instructions: [],
      });
    }

    await test('specialist_incomplete_scene_candidate_is_rejected', () => {
      const mutable = [
        { id: 'mutable_1', type: 'mutable', text: 'First original.', leading_ws: '', trailing_ws: '' },
        { id: 'mutable_2', type: 'mutable', text: 'Second original.', leading_ws: '', trailing_ws: '' },
      ];
      const detailed = validateCandidateSchemaDetailed({
        schema: 'loose_candidate_wrapper',
        role_id: 'wrong_role',
        candidates: [{
          role_id: 'wrong_role',
          segments: { mutable_1: 'First sentence rebuilt with stronger cadence.' },
          addressed_issues: ['rhythm', 'character_voice', 'unknown_issue'],
          confidence: 0.8,
          change_summary: 'Rebuilt the first segment.',
        }],
      }, 'style_reader', ['mutable_1', 'mutable_2'], mutable);
      const codes = detailed.diagnostics.map((item) => item.code);
      if (detailed.value
          || !codes.includes('normalized_schema')
          || !codes.includes('normalized_role_id')
          || !codes.includes('candidate_missing_required_segments')
          || !codes.includes('filtered_unknown_issue_codes')
          || !codes.includes('filtered_out_of_lane_issues')) {
        throw new Error(`incomplete candidate was not rejected:${codes.join(',')}`);
      }
      const emptyDetailed = validateCandidateSchemaDetailed({
        schema: 'scene_rewrite_candidates.v1',
        role_id: 'style_reader',
        candidates: [{
          segments: { mutable_1: 'Rewritten opening.', mutable_2: ' \n ' },
          addressed_issues: ['rhythm'],
          confidence: 0.8,
        }],
      }, 'style_reader', ['mutable_1', 'mutable_2'], mutable);
      const emptyCodes = emptyDetailed.diagnostics.map((item) => item.code);
      if (emptyDetailed.value || !emptyCodes.includes('candidate_empty_required_segments')) {
        throw new Error(`blank substantive segment was accepted:${emptyCodes.join(',')}`);
      }
      return codes.concat(emptyCodes).join(',');
    });

    await test('specialist_complete_multi_segment_candidate_is_accepted', () => {
      const mutable = [
        { id: 'mutable_1', type: 'mutable', text: 'First original.', leading_ws: '', trailing_ws: '' },
        { id: 'mutable_2', type: 'mutable', text: 'Second original.', leading_ws: '', trailing_ws: '' },
      ];
      const detailed = validateCandidateSchemaDetailed({
        schema: 'scene_rewrite_candidates.v1',
        role_id: 'style_reader',
        candidates: [{
          segments: {
            mutable_1: 'The opening now lands with a deliberate cadence.',
            mutable_2: 'The closing answers it with a sharper dramatic turn.',
          },
          evidence_refs: ['fact_1'],
          retained_beats: [{ id: 'fact_1', text: 'Keep the established meeting.' }],
          proposed_additions: [],
          addressed_issues: ['rhythm'],
          confidence: 0.8,
          change_summary: 'Rebuilt the complete scene rhythm from opening to closing.',
        }],
      }, 'style_reader', ['mutable_1', 'mutable_2'], mutable);
      const candidate = detailed.value && detailed.value.candidates[0];
      if (!candidate
          || !candidate.coverage_complete
          || candidate.covered_segment_ids.join(',') !== 'mutable_1,mutable_2'
          || Object.keys(candidate.segments).length !== 2
          || candidate.changed_segment_ids.length !== 2) {
        throw new Error(`complete candidate rejected:${JSON.stringify(detailed)}`);
      }
      return candidate.covered_segment_ids.join(',');
    });

    await test('specialist_empty_candidate_is_not_counted_as_success', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === 'style_reader');
      const profile = settings.role_profiles.style_reader;
      profile.provider = 'openai_compatible';
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'no-candidate-model';
      profile.timeout_ms = 5000;
      globalThis.fetch = async () => response(JSON.stringify({
        schema: 'scene_rewrite_candidates.v1',
        role_id: 'style_reader',
        candidates: [],
      }));
      try {
        const trace = newTrace('test', 'test');
        trace.budget.http_attempt_max = 2;
        const deadline = createDeadline(10000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: 'Original scene.',
          leading_ws: '',
          trailing_ws: '',
        }];
        const result = await callRole(
          role,
          profile,
          segments,
          '',
          segments,
          deadline.signal,
          trace,
          { draft_ledger: ledger() },
          Date.now(),
          {
            allowRetry: false,
            allowFallback: false,
            completionWait: false,
            canContinue: () => true,
          }
        );
        deadline.cancel();
        refreshTraceSummary(trace, { mutable: 1 }, null);
        const roleTrace = trace.roles[0];
        if (!result || result.candidates.length !== 0
            || !roleTrace
            || roleTrace.status !== 'no_candidate'
            || roleTrace.attempts[0].status !== 'no_candidate'
            || trace.summary.successful_roles !== 0) {
          throw new Error(`no-candidate counted as success:${JSON.stringify(roleTrace)}`);
        }
        return 'valid empty candidate response traced as no_candidate';
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('reasoning_only_specialist_is_deferred_for_structured_recovery', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === 'character_reader');
      const profile = settings.role_profiles.character_reader;
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'reasoning-specialist-model';
      globalThis.fetch = async () => ({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({
          choices: [{
            message: {
              content: '',
              reasoning_content: sceneCandidateJson(
                'character_reader',
                'Character knowledge and reaction are rebuilt through action.'
              ),
            },
          }],
        }),
      });
      try {
        const trace = newTrace('test', 'test');
        const result = await callRole(
          role,
          profile,
          [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }],
          '',
          [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }],
          new AbortController().signal,
          trace,
          { draft_ledger: ledger() },
          Date.now(),
          {
            allowRetry: false,
            allowFallback: false,
            deferStructuredRecovery: true,
          }
        );
        if (!result || !result.__deferred_specialist_recovery
            || result.error_class !== 'reasoning_only_response'
            || !result.recovery_source.includes('scene_rewrite_candidates.v1')
            || trace.roles[0].status !== 'recovery_queued') {
          throw new Error(`reasoning-only specialist was dropped:${JSON.stringify(result)}`);
        }
        return result.error_class;
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('deferred_specialist_recovery_runs_after_primaries_with_preset_limits', async () => {
      const originalFetch = globalThis.fetch;
      async function runCase(preset, characterRecoveryFailure) {
        const settings = defaultSettings();
        const roleIds = [
          'character_reader',
          'style_reader',
          'plot_continuity_reader',
          JUDGE_ROLE_ID,
          COMPOSER_ROLE_ID,
          PROVER_ROLE_ID,
        ];
        const roles = roleIds.map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
        roles.forEach((role) => {
          const profile = settings.role_profiles[role.role_id];
          profile.endpoint = 'https://test.example.com/v1/chat/completions';
          profile.model = `${role.role_id}-model`;
          profile.timeout_ms = 5000;
        });
        const calls = [];
        globalThis.fetch = async (_url, options) => {
          const body = JSON.parse(options.body);
          const model = safeString(body.model);
          const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
          const recovery = prompt.includes('into one complete scene-wide candidate');
          calls.push({ model, recovery, prompt });
          if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
            const match = /Scene candidates:\n(.+?)\n\n--- Draft Ledger/s.exec(prompt);
            const promptCandidates = match ? JSON.parse(match[1]) : [];
            return response(JSON.stringify(judgmentObject(
              promptCandidates.map((item) => item.candidate_id)
            )));
          }
          if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
            return response(JSON.stringify({
              segments: { mutable_1: 'Composer integrated every recovered specialist contribution.' },
            }));
          }
          if (model.indexOf('plot_continuity_reader') >= 0) {
            return response(sceneCandidateJson(
              'plot_continuity_reader',
              'The scene advances through a clear causal turn.'
            ));
          }
          if (model.indexOf('character_reader') >= 0 || model.indexOf('style_reader') >= 0) {
            const roleId = model.indexOf('character_reader') >= 0
              ? 'character_reader'
              : 'style_reader';
            if (!recovery) {
              return response('```json\n{"schema":"scene_rewrite_candidates.v1","role_id":"'
                + roleId + '","candidates":[');
            }
            if (!prompt.includes('SOURCE:') || prompt.includes('Runtime Context')) {
              throw new Error(`recovery prompt not compact:${roleId}`);
            }
            if (characterRecoveryFailure === 'invalid' && roleId === 'character_reader') {
              return response('{"schema":"scene_rewrite_candidates.v1","candidates":[');
            }
            if (characterRecoveryFailure === 'empty' && roleId === 'character_reader') {
              return response(JSON.stringify({
                schema: 'scene_rewrite_candidates.v1',
                role_id: 'character_reader',
                candidates: [],
              }));
            }
            return response(sceneCandidateJson(
              roleId,
              roleId === 'character_reader'
                ? 'Character reactions now carry distinct knowledge and subtext.'
                : 'The prose now moves through varied cadence and transitions.'
            ));
          }
          throw new Error(`unexpected model:${model}`);
        };
        const trace = newTrace('test', 'test');
        trace.router.preset = preset;
        const deadline = createDeadline(60000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: 'Original scene.',
          leading_ws: '',
          trailing_ws: '',
        }];
        try {
          const result = await scheduleRoles(
            roles,
            settings.role_profiles,
            segments,
            '',
            segments,
            deadline,
            trace,
            3,
            ledger()
          );
          if (result.failureReason || !result.composerResult) {
            throw new Error(`recovery pipeline failed:${preset}:${result.failureReason}`);
          }
        } finally {
          deadline.cancel();
        }
        const specialistModels = ['character_reader-model', 'style_reader-model', 'plot_continuity_reader-model'];
        const primaryIndexes = calls.map((call, index) =>
          specialistModels.includes(call.model) && !call.recovery ? index : -1
        ).filter((index) => index >= 0);
        const recoveryIndexes = calls.map((call, index) =>
          specialistModels.includes(call.model) && call.recovery ? index : -1
        ).filter((index) => index >= 0);
        const expectedRecoveries = preset === 'quality' ? 2 : (preset === 'balanced' ? 1 : 0);
        const expectedSuccesses = characterRecoveryFailure
          ? 0
          : (preset === 'fast' ? 0 : (preset === 'quality' ? 2 : 1));
        if (primaryIndexes.length !== 3
            || recoveryIndexes.length !== expectedRecoveries
            || (recoveryIndexes.length
              && Math.min.apply(null, recoveryIndexes) <= Math.max.apply(null, primaryIndexes))
            || trace.candidates.structured_recovery_attempted !== expectedRecoveries
            || trace.candidates.structured_recovery_succeeded !== expectedSuccesses) {
          throw new Error(`deferred recovery mismatch:${preset}:${JSON.stringify(calls.map((call) => ({
            model: call.model,
            recovery: call.recovery,
          })))}`);
        }
        return {
          attempted: recoveryIndexes.length,
          succeeded: trace.candidates.structured_recovery_succeeded,
        };
      }
      try {
        const fastRecoveries = await runCase('fast');
        const balancedRecoveries = await runCase('balanced');
        const qualityRecoveries = await runCase('quality');
        const invalidFailover = await runCase('balanced', 'invalid');
        const emptyFailover = await runCase('balanced', 'empty');
        return `fast:${fastRecoveries.attempted}/${fastRecoveries.succeeded} balanced:${balancedRecoveries.attempted}/${balancedRecoveries.succeeded} quality:${qualityRecoveries.attempted}/${qualityRecoveries.succeeded} invalid-failover:${invalidFailover.attempted}/${invalidFailover.succeeded} empty-failover:${emptyFailover.attempted}/${emptyFailover.succeeded}`;
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('fusion_plan_requires_each_accepted_role_contribution', () => {
      const candidates = [
        candidate('character_reader', 'cand_character', 'Character subtext becomes explicit through action.'),
        candidate('style_reader', 'cand_style', 'Cadence and paragraph transitions are rebuilt.'),
      ];
      const judged = validateSemanticJudgment(
        judgmentObject(['cand_character', 'cand_style']),
        candidates,
        ledger()
      );
      const plan = buildFusionPlan(
        judged,
        candidates,
        ledger(),
        [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }]
      );
      const roles = uniqueList(plan.required_contributions.map((item) => item.role_id)).sort();
      if (roles.join(',') !== 'character_reader,style_reader'
          || plan.required_contributions.some((item) =>
            !item.contribution_id || !item.candidate_id || !item.segment_ids.length || !item.claim
          )) {
        throw new Error(`required contribution mismatch:${JSON.stringify(plan.required_contributions)}`);
      }
      const composerRole = DEFAULT_ROLES.find((role) => role.role_id === COMPOSER_ROLE_ID);
      const prompt = buildRolePrompt(
        composerRole,
        defaultSettings().role_profiles[COMPOSER_ROLE_ID],
        [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }],
        '',
        [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }],
        {
          candidateBundles: { mutable_1: [] },
          semantic_judgment: judged,
          fusion_plan: plan,
          draft_ledger: ledger(),
        }
      );
      if (!prompt.user.includes('Realize every fusion_plan.required_contributions')) {
        throw new Error('Composer contribution obligation missing');
      }
      const partialCandidate = candidate(
        'style_reader',
        'cand_partial',
        'Only the first segment is materially rewritten.'
      );
      partialCandidate.segments.mutable_2 = 'Second original.';
      partialCandidate.segment_operations.mutable_2 = 'passthrough';
      partialCandidate.changed_segment_ids = ['mutable_1'];
      const partialJudgmentRaw = judgmentObject(['cand_partial']);
      partialJudgmentRaw.candidate_judgments[0].quality_gains[0].segment_ids = ['mutable_2'];
      const partialJudgment = validateSemanticJudgment(
        partialJudgmentRaw,
        [partialCandidate],
        ledger()
      );
      const partialPlan = buildFusionPlan(
        partialJudgment,
        [partialCandidate],
        ledger(),
        [
          { id: 'mutable_1', type: 'mutable', text: 'Original.' },
          { id: 'mutable_2', type: 'mutable', text: 'Second original.' },
        ]
      );
      if (partialPlan.required_contributions.some((item) =>
        item.claim === 'Stronger dramatic movement'
        || item.segment_ids.includes('mutable_2')
      ) || partialPlan.required_contributions[0].source !== 'change_summary') {
        throw new Error(`coverage-gap contribution moved:${JSON.stringify(partialPlan.required_contributions)}`);
      }
      return roles.join(',');
    });

    await test('semantic_prover_enforces_quality_gain_realization', () => {
      const candidates = [candidate(
        'character_reader',
        'cand_character',
        'The meeting gains sharper character subtext.'
      )];
      const judged = validateSemanticJudgment(
        judgmentObject(['cand_character']),
        candidates,
        ledger()
      );
      const plan = buildFusionPlan(
        judged,
        candidates,
        ledger(),
        [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }]
      );
      const contribution = plan.required_contributions[0];
      const finalText = 'They met in the room. The exchange carried sharper dramatic movement.';
      const mutable = [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }];
      const finalSegments = [{ id: 'mutable_1', type: 'mutable', final_text: finalText }];
      const goodRaw = semanticProofObject('pass');
      goodRaw.quality_gain_checks = [{
        contribution_id: contribution.contribution_id,
        status: 'realized',
        detail: 'The dramatic movement is present in the exchange.',
        evidence_quote: 'sharper dramatic movement',
        segment_ids: ['mutable_1'],
      }];
      const good = validateSemanticProof(goodRaw, ledger(), mutable, finalSegments, plan);
      if (!good || good.verdict !== 'pass') throw new Error('realized gain did not pass');

      const unchangedEvidenceMutable = [{
        id: 'mutable_1',
        type: 'mutable',
        text: 'They met in the room. The exchange remained guarded.',
      }];
      const unchangedEvidenceFinal = [{
        id: 'mutable_1',
        type: 'mutable',
        final_text: 'They met in the room. The exchange carried sharper dramatic movement.',
      }];
      const unchangedEvidenceRaw = JSON.parse(JSON.stringify(goodRaw));
      unchangedEvidenceRaw.quality_gain_checks[0].evidence_quote = 'They met in the room';
      if (validateSemanticProof(
        unchangedEvidenceRaw,
        ledger(),
        unchangedEvidenceMutable,
        unchangedEvidenceFinal,
        plan
      )) {
        throw new Error('unchanged original wording was accepted as role contribution evidence');
      }

      const wrongSegmentMutable = mutable.concat({
        id: 'mutable_2',
        type: 'mutable',
        text: 'Other original.',
      });
      const wrongSegmentFinal = [
        { id: 'mutable_1', type: 'mutable', final_text: 'They met in the room.' },
        {
          id: 'mutable_2',
          type: 'mutable',
          final_text: 'An unrelated paragraph contains sharper dramatic movement.',
        },
      ];
      const wrongSegmentRaw = JSON.parse(JSON.stringify(goodRaw));
      const wrongSegment = validateSemanticProof(
        wrongSegmentRaw,
        ledger(),
        wrongSegmentMutable,
        wrongSegmentFinal,
        plan
      );
      if (wrongSegment) {
        throw new Error('quality evidence from the wrong segment was accepted');
      }

      const repairRaw = JSON.parse(JSON.stringify(goodRaw));
      repairRaw.declared_verdict = 'repair';
      repairRaw.quality_gain_checks[0].status = 'missing';
      repairRaw.quality_gain_checks[0].evidence_quote = '';
      repairRaw.repair_instructions = [{
        segment_id: 'mutable_1',
        instruction: `Realize the missing character contribution ${contribution.contribution_id} in the full scene.`,
        evidence_refs: [contribution.contribution_id],
        prohibited: [],
      }];
      const repair = validateSemanticProof(repairRaw, ledger(), mutable, finalSegments, plan);
      if (!repair || repair.verdict !== 'repair'
          || !repair.reason_codes.some((code) => code.indexOf('quality_gain_missing:') === 0)) {
        throw new Error(`missing gain did not request repair:${JSON.stringify(repair)}`);
      }

      const failedRaw = JSON.parse(JSON.stringify(repairRaw));
      failedRaw.declared_verdict = 'pass';
      failedRaw.repair_instructions = [];
      const failed = validateSemanticProof(failedRaw, ledger(), mutable, finalSegments, plan);
      if (!failed || failed.verdict !== 'fail') {
        throw new Error('missing gain incorrectly passed');
      }
      return `${good.verdict}/${repair.verdict}/${failed.verdict}`;
    });

    await test('phase_c_role_topology', () => {
      const specialists = DEFAULT_ROLES.filter((role) => role.stage === 'output' && !role.is_composer);
      const ids = specialists.map((role) => role.role_id).sort().join(',');
      if (ids !== 'character_reader,perspective_boundary_rewriter,plot_continuity_reader,style_reader') throw new Error(`unexpected specialists:${ids}`);
      if (!DEFAULT_ROLES.some((role) => role.is_judge && role.role_id === JUDGE_ROLE_ID)) throw new Error('semantic judge missing');
      if (!DEFAULT_ROLES.some((role) => role.is_prover && role.role_id === PROVER_ROLE_ID)) throw new Error('semantic prover missing');
      if (DEFAULT_ROLES.some((role) => ['secret_pov_guard', 'world_reader', 'agency_meta_guard'].indexOf(role.role_id) >= 0)) throw new Error('legacy specialist remains');
      return ids;
    });

    await test('specialist_issue_ownership_is_disjoint', () => {
      const owners = {};
      SPECIALIST_ROLE_IDS.forEach((roleId) => {
        roleAllowedIssues(roleId).forEach((issue) => {
          if (owners[issue]) throw new Error(`overlap:${issue}:${owners[issue]}:${roleId}`);
          owners[issue] = roleId;
        });
      });
      ISSUE_GROUPS.forEach((issue) => {
        if (!owners[issue]) throw new Error(`unowned:${issue}`);
        if (owners[issue] !== ISSUE_OWNER_ROLE[issue]) {
          throw new Error(`wrong_owner:${issue}:${owners[issue]}`);
        }
      });
      return `${Object.keys(owners).length} issues partitioned across 4 roles`;
    });

    await test('semantic_judge_unresolved_requirement_is_owner_bound', () => {
      const candidates = [{
        candidate_id: 'candidate_boundary',
        role_id: 'perspective_boundary_rewriter',
        segments: { mutable_1: 'Rewritten scene.' },
      }];
      const raw = {
        schema: 'semantic_judgment.v1',
        candidate_judgments: [{
          candidate_id: 'candidate_boundary',
          verdict: 'accept',
          preserved_ledger_ids: [],
          missing_ledger_ids: [],
          unsupported_additions: [],
          hard_violations: [],
          accepted_elements: [{ claim: 'POV remains bounded.', candidate_ids: ['candidate_boundary'], segment_ids: ['mutable_1'] }],
          rejected_elements: [],
          quality_gains: [{ claim: 'Reveal timing is clearer.', candidate_ids: ['candidate_boundary'], segment_ids: ['mutable_1'] }],
          quality_regressions: [],
        }],
        cross_candidate: { consensus: [], complementary: [], conflicts: [] },
        unresolved_requirements: [{
          issue_type: 'character_voice',
          role_id: 'character_reader',
          claim: 'The dialogue still lacks the established clipped register.',
          segment_ids: ['mutable_1'],
          evidence_refs: ['character'],
        }, {
          issue_type: 'secret_leak',
          role_id: 'style_reader',
          claim: 'Wrong owner must be rejected.',
          segment_ids: ['mutable_1'],
          evidence_refs: [],
        }],
        scene_requirements: { target_arc: '', target_voice: '', target_pacing: '' },
      };
      const judged = validateSemanticJudgment(raw, candidates, { schema: 'draft_ledger.v1' });
      if (!judged || judged.unresolved_requirements.length !== 1
          || judged.unresolved_requirements[0].role_id !== 'character_reader') {
        throw new Error(`unresolved_requirement_not_partitioned:${JSON.stringify(judged)}`);
      }
      return judged.unresolved_requirements[0].requirement_id;
    });

    await test('independent_roles_keep_identical_scene_candidates', () => {
      const pool = [];
      const seen = new Set();
      const base = {
        scene_signature: 'same_scene',
        segments: { mutable_1: 'Same complete scene.' },
        segment_operations: { mutable_1: 'replace' },
        evidence_refs: [],
        addressed_issues: [],
        retained_beats: [],
        proposed_additions: [],
      };
      const left = Object.assign({}, base, {
        candidate_id: 'candidate_character',
        role_id: 'character_reader',
        supporting_roles: ['character_reader'],
      });
      const right = Object.assign({}, base, {
        candidate_id: 'candidate_style',
        role_id: 'style_reader',
        supporting_roles: ['style_reader'],
      });
      if (!admitSceneCandidate(pool, seen, left) || !admitSceneCandidate(pool, seen, right)
          || pool.length !== 2) {
        throw new Error(`cross_role_candidate_collapsed:${pool.length}`);
      }
      return '2 independent role submissions retained';
    });

    await test('output_reuse_requires_contract_and_hashed_key_ref_identity', () => {
      completedOutputCache = [];
      const settingsA = defaultSettings();
      const settingsB = deepClone(settingsA);
      settingsA.role_profiles.character_reader.api_key_ref = 'sk-contract-alpha-123456789';
      settingsB.role_profiles.character_reader.api_key_ref = 'sk-contract-beta-987654321';
      const messages = [{ role: 'user', content: 'Continue the same grounded scene.' }];
      const snapshotA = makeRequestSnapshot(messages, 'model', { detected: false, reason: '' }, '', {
        original_messages: messages,
        injected_messages: messages,
        context_manifest: {
          snapshot_id: 'ctx_1000_manifest',
          evidence_refs: ['payload_user_input'],
          latest_user_input: 'Continue the same grounded scene.',
        },
        turn_contract: {
          schema: 'turn_contract.v1',
          contract_id: 'turn_1000_contract',
          contract_digest: 'contract',
          source_snapshot_id: 'ctx_1000_manifest',
          immutable_constraints: [],
          turn_objectives: [{ text: 'Continue the same grounded scene.' }],
        },
      });
      const snapshotB = makeRequestSnapshot(messages, 'model', { detected: false, reason: '' }, '', {
        original_messages: messages,
        injected_messages: messages,
        context_manifest: {
          snapshot_id: 'ctx_2000_manifest',
          evidence_refs: ['payload_user_input'],
          latest_user_input: 'Continue the same grounded scene.',
        },
        turn_contract: {
          schema: 'turn_contract.v1',
          contract_id: 'turn_2000_contract',
          contract_digest: 'contract',
          source_snapshot_id: 'ctx_2000_manifest',
          immutable_constraints: [],
          turn_objectives: [{ text: 'Continue the same grounded scene.' }],
        },
      });
      const raw = 'The same raw model draft.';
      const finalText = 'The proven recomposed scene.';
      const trace = newTrace('afterRequest', 'model');
      storeCompletedOutputReuse(raw, 'model', settingsA, snapshotA, finalText, trace, []);
      const stableHit = findCompletedOutputReuse(raw, 'model', settingsA, snapshotB);
      const noContractHit = findCompletedOutputReuse(raw, 'model', settingsA, null);
      const changedKeyRefHit = findCompletedOutputReuse(raw, 'model', settingsB, snapshotB);
      const serializedCache = JSON.stringify(completedOutputCache);
      completedOutputCache = [];
      if (!stableHit
          || stableHit.match !== 'strict'
          || noContractHit
          || changedKeyRefHit
          || serializedCache.indexOf('sk-contract-alpha-123456789') >= 0
          || serializedCache.indexOf('sk-contract-beta-987654321') >= 0) {
        throw new Error(`output reuse identity mismatch:${JSON.stringify({
          stable: stableHit && stableHit.match,
          no_contract: !!noContractHit,
          changed_key_ref: !!changedKeyRefHit,
          key_exposed: serializedCache.indexOf('sk-contract-') >= 0,
        })}`);
      }
      return 'strict contract identity; key refs hashed; no contract means no reuse';
    });

    await test('provider_endpoint_metric_and_role_trace_are_distinct', () => {
      const settings = defaultSettings();
      settings.roles.forEach((role) => {
        settings.role_profiles[role.role_id].enabled = false;
      });
      const roleIds = ['character_reader', 'style_reader'];
      roleIds.forEach((roleId, index) => {
        const profile = settings.role_profiles[roleId];
        profile.enabled = true;
        profile.provider = 'openai_compatible';
        profile.endpoint = index === 0
          ? 'https://ollama.com/v1/chat/completions'
          : 'https://api.llmgateway.io/v1/chat/completions';
        profile.model = `${roleId}-model`;
      });
      const html = renderUI(settings);
      if (html.indexOf('Provider endpoints</span><strong>2 connected') < 0) {
        throw new Error('distinct endpoint origins collapsed in UI metric');
      }
      const trace = newTrace('test', 'test');
      traceRole(trace, {
        role_id: 'character_reader',
        provider: 'openai_compatible',
        endpoint_group: 'https://ollama.com',
        model: 'character-reader-model',
        status: 'fulfilled',
        started_at: 1,
        ended_at: 2,
        elapsed_ms: 1,
      });
      if (trace.roles[0].endpoint_group !== 'https://ollama.com') {
        throw new Error('role endpoint group was not retained');
      }
      trace.candidates.total = 1;
      trace.composer.status = 'failed';
      refreshTraceSummary(trace, { mutable: 3 }, null);
      if (trace.summary.specialist_calls !== 1
          || trace.summary.successful_roles !== 1
          || trace.summary.candidate_count !== 1
          || trace.summary.composer_state !== 'failed') {
        throw new Error('failure trace summary did not retain completed work');
      }
      return '2 endpoint origins counted and failure trace retained';
    });

    await test('composer_reasoning_only_uses_one_json_recovery', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === COMPOSER_ROLE_ID);
      const profile = settings.role_profiles[COMPOSER_ROLE_ID];
      profile.provider = 'openai_compatible';
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'deepseek-v4-pro';
      profile.reasoning_preset = 'deepseek';
      profile.reasoning_effort = 'high';
      profile.force_json_response = false;
      profile.extra_body = '{"reasoning_effort":"high","stream":true,"response_format":{"type":"text"}}';
      profile.timeout_ms = 5000;
      let calls = 0;
      const bodies = [];
      globalThis.fetch = async (_url, options) => {
        calls++;
        bodies.push(JSON.parse(options.body));
        if (calls === 1) {
          return {
            ok: true,
            status: 200,
            text: async () => JSON.stringify({
              choices: [{ message: { content: '', reasoning_content: 'Long internal composition reasoning.' } }],
            }),
          };
        }
        return response(JSON.stringify({
          segments: { mutable_1: 'The rain struck the shutters while Mira rebuilt the scene through action and subtext.' },
        }));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.budget.http_attempt_max = 4;
        const deadline = createDeadline(30000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: 'Mira stood by the window.',
          leading_ws: '',
          trailing_ws: '',
        }];
        const result = await callRole(
          role,
          profile,
          segments,
          'runtime context',
          segments,
          deadline.signal,
          trace,
          {
            candidateBundles: { mutable_1: [] },
            semantic_judgment: {},
            fusion_plan: {},
            draft_ledger: ledger(),
          },
          Date.now(),
          {
            allowRetry: true,
            allowFallback: false,
            completionWait: false,
            canContinue: () => true,
          }
        );
        deadline.cancel();
        const attempts = trace.roles[0] && trace.roles[0].attempts || [];
        if (!result || calls !== 2) throw new Error(`composer recovery calls:${calls}`);
        if (attempts.length !== 2 || attempts[1].kind !== 'composer_json_recovery') {
          throw new Error('composer recovery attempt was not traced');
        }
        if (bodies[1].reasoning_effort !== 'none'
            || bodies[1].stream !== false
            || !bodies[1].response_format
            || bodies[1].response_format.type !== 'json_object') {
          throw new Error('composer recovery did not disable reasoning and force JSON');
        }
        return 'reasoning-only Composer recovered in exactly one structured retry';
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('specialist_mixed_segment_empty_uses_targeted_recovery', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === 'character_reader');
      const profile = settings.role_profiles.character_reader;
      profile.provider = 'openai_compatible';
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'glm-5.2';
      profile.reasoning_preset = 'glm';
      profile.reasoning_effort = 'high';
      profile.force_json_response = false;
      profile.extra_body = '{"think":true,"thinking":{"type":"enabled"},"stream":true}';
      profile.timeout_ms = 5000;
      let calls = 0;
      const bodies = [];
      globalThis.fetch = async (_url, options) => {
        calls++;
        bodies.push(JSON.parse(options.body));
        if (calls === 1) {
          return response(JSON.stringify({
            schema: 'scene_rewrite_candidates.v1',
            role_id: 'character_reader',
            candidates: [{
              segments: { mutable_1: '' },
              evidence_refs: ['fact_1'],
              retained_beats: [{ id: 'fact_1', text: 'Keep the meeting fact.' }],
              proposed_additions: [],
              addressed_issues: ['meta_artifact'],
              confidence: 0.8,
              change_summary: 'remove leaked reasoning',
            }],
          }));
        }
        return response(JSON.stringify({
          schema: 'scene_rewrite_candidates.v1',
          role_id: 'character_reader',
          candidates: [{
            segments: {
              mutable_1: '### Chapter 1\nMira closed the ledger and met Rowan with a guarded, character-specific answer.',
            },
            evidence_refs: ['fact_1'],
            retained_beats: [{ id: 'fact_1', text: 'Keep the meeting fact.' }],
            proposed_additions: [],
            addressed_issues: ['meta_artifact', 'character_voice'],
            confidence: 0.8,
            change_summary: 'removed reasoning and rebuilt the complete narrative segment',
          }],
        }));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.budget.http_attempt_max = 4;
        const deadline = createDeadline(30000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: '<Thoughts>Analyze the scene.</Thoughts>\n### Chapter 1\nMira met Rowan in the archive.',
          leading_ws: '',
          trailing_ws: '',
        }];
        const result = await callRole(
          role,
          profile,
          segments,
          '',
          segments,
          deadline.signal,
          trace,
          { draft_ledger: ledger() },
          Date.now(),
          {
            allowRetry: true,
            allowFallback: false,
            completionWait: false,
            canContinue: () => true,
          }
        );
        deadline.cancel();
        const attempts = trace.roles[0] && trace.roles[0].attempts || [];
        if (!result || result.candidates.length !== 1 || calls !== 2) {
          throw new Error(`specialist recovery calls:${calls}`);
        }
        if (attempts.length !== 2 || attempts[1].kind !== 'specialist_mixed_segment_recovery') {
          throw new Error('specialist mixed-segment recovery was not traced');
        }
        if (bodies[1].think !== false
            || !bodies[1].thinking
            || bodies[1].thinking.type !== 'disabled'
            || bodies[1].stream !== false
            || !bodies[1].response_format) {
          throw new Error('specialist recovery did not disable reasoning and force JSON');
        }
        if (!result.candidates[0].segments.mutable_1) {
          throw new Error('mixed narrative segment was still deleted');
        }
        return 'mixed meta+narrative deletion recovered in exactly one structured retry';
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('semantic_proof_computes_pass_and_rejects_false_pass', () => {
      const mutable = [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }];
      const finalSegments = [{ id: 'mutable_1', type: 'mutable', final_text: 'They met and remained in the room.' }];
      const passed = validateSemanticProof(semanticProofObject('pass'), ledger(), mutable, finalSegments);
      const falsePassRaw = semanticProofObject('pass');
      falsePassRaw.fact_checks[0].status = 'missing';
      const falsePass = validateSemanticProof(falsePassRaw, ledger(), mutable, finalSegments);
      const incompleteRaw = semanticProofObject('pass');
      incompleteRaw.beat_checks = [];
      const incomplete = validateSemanticProof(incompleteRaw, ledger(), mutable, finalSegments);
      const repairRaw = semanticProofObject('repair');
      repairRaw.fact_checks[0].status = 'missing';
      repairRaw.repair_instructions = [{
        segment_id: 'mutable_1',
        instruction: 'Restore the established meeting fact.',
        evidence_refs: ['fact_1'],
        prohibited: [],
      }];
      const repair = validateSemanticProof(repairRaw, ledger(), mutable, finalSegments);
      if (!passed || passed.verdict !== 'pass') throw new Error('clean proof did not pass');
      if (!falsePass || falsePass.verdict !== 'fail') throw new Error('declared pass overrode missing fact');
      if (incomplete) throw new Error('incomplete ledger coverage accepted');
      if (!repair || repair.verdict !== 'repair') throw new Error('targeted repair was not admitted');
      return 'computed verdict, exact ledger coverage, and one repair';
    });

    await test('visible_output_extraction_precedes_segmentation', () => {
      const closed = extractVisibleAssistantOutput(
        '<Thoughts>Private chain of thought.</Thoughts>\n### Chapter 1\nMira opened the ledger.'
      );
      if (closed.text.indexOf('Private chain') >= 0
          || closed.text.indexOf('### Chapter 1') !== 0
          || closed.removed_block_count !== 1) {
        throw new Error(`closed reasoning leaked:${JSON.stringify(closed)}`);
      }
      const multiple = extractVisibleAssistantOutput(
        '<Analysis>First private block.</Analysis>\nScene line.\n<Thinking>Second private block.</Thinking>\nEnding.'
      );
      if (multiple.removed_block_count !== 2
          || multiple.text.indexOf('private block') >= 0
          || !multiple.text.includes('Scene line.')
          || !multiple.text.includes('Ending.')) {
        throw new Error(`multiple reasoning extraction failed:${JSON.stringify(multiple)}`);
      }
      const unclosed = extractVisibleAssistantOutput(
        '<Thoughts>Private unfinished reasoning.\n## Response\nMira opened the ledger.'
      );
      if (!unclosed.text.startsWith('Mira opened the ledger.')
          || unclosed.removed_block_count !== 1
          || unclosed.ambiguous_unclosed) {
        throw new Error(`unclosed boundary extraction failed:${JSON.stringify(unclosed)}`);
      }
      const ambiguousSource = '<Thoughts>Possibly narrative content without a visible boundary.';
      const ambiguous = extractVisibleAssistantOutput(ambiguousSource);
      if (!ambiguous.ambiguous_unclosed || !ambiguous.changed || ambiguous.text !== '') {
        throw new Error(`ambiguous reasoning leaked:${JSON.stringify(ambiguous)}`);
      }
      const escaped = extractVisibleAssistantOutput(
        '&lt;Thoughts&gt;Escaped private reasoning.&lt;/Thoughts&gt;\n### Chapter 2\nThe gate opened.'
      );
      if (escaped.text !== '### Chapter 2\nThe gate opened.'
          || escaped.removed_block_count !== 1) {
        throw new Error(`escaped reasoning leaked:${JSON.stringify(escaped)}`);
      }
      const markdown = extractVisibleAssistantOutput(
        '**Thinking Process:**\n1. Inspect the request.\n2. Plan the response.\n\n### Chapter 3\n비가 처마를 두드렸다.'
      );
      if (markdown.text !== '### Chapter 3\n비가 처마를 두드렸다.'
          || markdown.removed_block_count !== 1) {
        throw new Error(`markdown reasoning leaked:${JSON.stringify(markdown)}`);
      }
      const narrative = '그는 문서의 **analysis:** 항목을 손가락으로 짚었다.\n\n다음 문단도 그대로 이어졌다.';
      const untouchedNarrative = extractVisibleAssistantOutput(narrative);
      if (untouchedNarrative.changed || untouchedNarrative.text !== narrative) {
        throw new Error(`narrative false positive:${JSON.stringify(untouchedNarrative)}`);
      }
      const visibleSegments = buildSegmentMap(closed.text, defaultSettings());
      if (visibleSegments.some((segment) => safeString(segment.text).indexOf('Private chain') >= 0)) {
        throw new Error('segmentation received removed reasoning');
      }
      return 'xml/escaped/markdown reasoning removed; narrative occurrence preserved';
    });

    await test('after_request_returns_visible_output_only', async () => {
      const originalRisu = globalThis.Risuai;
      const settings = defaultSettings();
      settings.trace_enabled = false;
      Object.keys(settings.role_profiles).forEach((roleId) => {
        settings.role_profiles[roleId].enabled = false;
      });
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => key === SETTINGS_KEY ? JSON.stringify(settings) : null,
          setItem: async () => true,
        },
      };
      pendingMainSnapshot = makeRequestSnapshot(
        [{ role: 'user', content: 'Continue the scene.' }],
        'model',
        { detected: false, reason: '' },
        ''
      );
      try {
        const result = await onAfterRequest(
          '<Thoughts>Private chain of thought.</Thoughts>\n### Chapter 1\nMira opened the ledger.',
          'model'
        );
        if (result !== '### Chapter 1\nMira opened the ledger.') {
          throw new Error(`afterRequest returned reasoning:${JSON.stringify(result)}`);
        }
        pendingMainSnapshot = null;
        const preflightResult = await onAfterRequest(
          '<Analysis>Private preflight analysis.</Analysis>\nVisible fallback scene.',
          'model'
        );
        if (preflightResult !== 'Visible fallback scene.') {
          throw new Error(`preflight returned reasoning:${JSON.stringify(preflightResult)}`);
        }
        return 'role bypass and ambiguous preflight returned visible prose only';
      } finally {
        pendingMainSnapshot = null;
        globalThis.Risuai = originalRisu;
      }
    });

    await test('materiality_reports_change_strength_without_controlling_return', () => {
      const original = '초여름의 푸른 그늘이 공조(工曹) 청사 앞 느릅나무 가지마다 짙게 늘어서기 시작했다. '
        + '단오를 앞둔 한양의 공기는 메말라 있었고 공조 마당은 수레와 인부들의 소리로 요란했다. '
        + '행정실 북쪽 창가의 자리는 바깥과 격리된 섬처럼 정갈한 정적을 유지하고 있었다.';
      const cosmetic = original.replace('공조(工曹)', '공조').replace('  ', ' ');
      const rebuilt = '느릅나무 그늘 아래로 수레가 연달아 밀려들었다. 마른 흙먼지가 공조 마당을 훑을 때마다 '
        + '인부들의 고함이 행정실 창호를 두드렸다. 그 소란에서 한 걸음 비껴난 북쪽 창가에서, '
        + '강한얼의 붓만 일정한 호흡으로 장부의 빈칸을 메웠다.';
      const cosmeticAssessment = rewriteMateriality(original, cosmetic);
      const rebuiltAssessment = rewriteMateriality(original, rebuilt);
      const metaAssessment = rewriteMateriality(
        '<Thoughts>Private reasoning only.</Thoughts>',
        ''
      );
      if (cosmeticAssessment.material) {
        throw new Error(`cosmetic change counted material:${JSON.stringify(cosmeticAssessment)}`);
      }
      if (!rebuiltAssessment.material) {
        throw new Error(`structural rewrite not material:${JSON.stringify(rebuiltAssessment)}`);
      }
      if (metaAssessment.material || !metaAssessment.original_meta_only) {
        throw new Error(`reasoning deletion counted material:${JSON.stringify(metaAssessment)}`);
      }
      return 'cosmetic:false structural:true reasoning:false';
    });

    await test('semantic_prover_residual_quality_blocks_false_pass', () => {
      const mutable = [{ id: 'mutable_1', type: 'mutable', text: 'Original scene.' }];
      const finalText = 'They met in the room. 권 서리가 어깨를 쫑긋 세우며 새 교지를 내밀었다.';
      const finalSegments = [{ id: 'mutable_1', type: 'mutable', final_text: finalText }];
      const clean = validateSemanticProof(
        semanticProofObject('pass'),
        ledger(),
        mutable,
        finalSegments,
        { required_contributions: [] }
      );
      if (!clean || clean.verdict !== 'pass') throw new Error('clean residual proof did not pass');

      const issueRaw = semanticProofObject('pass');
      issueRaw.residual_quality_checks[0] = {
        check_id: 'mechanics_and_wording',
        status: 'issue',
        detail: 'The body movement is awkwardly phrased.',
        evidence_quote: '어깨를 쫑긋 세우며',
        segment_ids: ['mutable_1'],
      };
      const falsePass = validateSemanticProof(
        issueRaw,
        ledger(),
        mutable,
        finalSegments,
        { required_contributions: [] }
      );
      if (!falsePass || falsePass.verdict !== 'fail'
          || !falsePass.reason_codes.includes('residual_quality_issue:mechanics_and_wording')) {
        throw new Error(`residual issue passed:${JSON.stringify(falsePass)}`);
      }

      const repairRaw = JSON.parse(JSON.stringify(issueRaw));
      repairRaw.declared_verdict = 'repair';
      repairRaw.repair_instructions = [{
        segment_id: 'mutable_1',
        instruction: 'Replace the awkward body movement with natural period-appropriate action for mechanics_and_wording.',
        evidence_refs: ['mechanics_and_wording'],
        prohibited: [],
      }];
      const repair = validateSemanticProof(
        repairRaw,
        ledger(),
        mutable,
        finalSegments,
        { required_contributions: [] }
      );
      if (!repair || repair.verdict !== 'repair') {
        throw new Error(`residual issue did not request repair:${JSON.stringify(repair)}`);
      }
      const incomplete = semanticProofObject('pass');
      incomplete.residual_quality_checks.pop();
      if (validateSemanticProof(incomplete, ledger(), mutable, finalSegments, { required_contributions: [] })) {
        throw new Error('incomplete residual quality coverage accepted');
      }
      return 'clean/pass issue/fail issue/repair incomplete/reject';
    });

    await test('semantic_prover_call_validates_final_composer_output', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === PROVER_ROLE_ID);
      const profile = settings.role_profiles[PROVER_ROLE_ID];
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'semantic-prover-model';
      profile.timeout_ms = 5000;
      globalThis.fetch = async () => response(JSON.stringify(semanticProofObject('pass')));
      try {
        const trace = newTrace('test', 'test');
        const deadline = createDeadline(30000);
        const segments = [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }];
        const finalSegments = [{
          id: 'mutable_1',
          type: 'mutable',
          original_text: 'Original.',
          final_text: 'They met and remained in the room.',
        }];
        const proof = await runSemanticProver(
          role,
          profile,
          segments,
          finalSegments,
          {
            semantic_judgment: judgmentObject(['cand_1']),
            fusion_plan: { schema: 'fusion_plan.v1' },
            draft_ledger: ledger(),
          },
          '',
          deadline.signal,
          trace,
          false,
          'test'
        );
        deadline.cancel();
        if (!proof || proof.verdict !== 'pass' || trace.semantic_prover.verdict !== 'pass') {
          throw new Error('semantic prover production call did not pass');
        }
        return 'provider response validated through semantic_proof.v1';
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('semantic_prover_repairs_schema_once_with_field_diagnostics', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === PROVER_ROLE_ID);
      const profile = settings.role_profiles[PROVER_ROLE_ID];
      profile.endpoint = 'https://test.example.com/v1/chat/completions';
      profile.model = 'semantic-prover-recovery-model';
      profile.timeout_ms = 5000;
      let calls = 0;
      const invalidProof = semanticProofObject('pass');
      invalidProof.fact_checks[0].evidence_quote = 'Go ahead.';
      globalThis.fetch = async () => {
        calls++;
        return response(JSON.stringify(calls === 1 ? invalidProof : semanticProofObject('pass')));
      };
      try {
        const trace = newTrace('test', 'test');
        const deadline = createDeadline(30000);
        const segments = [{ id: 'mutable_1', type: 'mutable', text: 'Original.', leading_ws: '', trailing_ws: '' }];
        const finalSegments = [{
          id: 'mutable_1',
          type: 'mutable',
          original_text: 'Original.',
          final_text: 'They met and remained in the room.',
        }];
        const proof = await runSemanticProver(
          role,
          profile,
          segments,
          finalSegments,
          {
            semantic_judgment: judgmentObject(['cand_1']),
            fusion_plan: { schema: 'fusion_plan.v1', required_contributions: [] },
            draft_ledger: ledger(),
          },
          '',
          deadline.signal,
          trace,
          false,
          'schema_recovery_test'
        );
        deadline.cancel();
        const roleTrace = trace.roles.filter((entry) => entry.role_id === PROVER_ROLE_ID).slice(-1)[0];
        const diagnostic = arrayFromCollection(roleTrace && roleTrace.validation_diagnostics)
          .find((item) => item.code === 'semantic_proof_preserved_quote_not_in_final'
            && item.field === 'fact_checks[0].evidence_quote');
        const recoveryAttempts = arrayFromCollection(roleTrace && roleTrace.attempts)
          .filter((attempt) => attempt.kind === 'semantic_prover_json_recovery');
        if (!proof || proof.verdict !== 'pass' || calls !== 2 || recoveryAttempts.length !== 1
            || !diagnostic || trace.semantic_prover.structured_recovery_attempted !== true
            || trace.semantic_prover.structured_recovery_succeeded !== true) {
          throw new Error(`prover recovery mismatch:${JSON.stringify({ calls, roleTrace, prover: trace.semantic_prover })}`);
        }
        return 'one schema recovery; exact failed field retained in trace';
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('composer_only_assembly_ignores_specialist_candidate', () => {
      const segments = [{ id: 'mutable_1', type: 'mutable', text: 'Original paragraph.', leading_ws: '', trailing_ws: '' }];
      const director = {
        ranked: {
          mutable_1: [{
            role_id: 'character_reader',
            rewrite: 'Specialist replacement.',
            judge_verdict: 'accept',
            identical_to_original: false,
          }],
        },
      };
      const assembled = assembleOutput(segments, null, director);
      if (assembled.changed || assembled.output !== 'Original paragraph.') {
        throw new Error('specialist candidate escaped Composer boundary');
      }
      return 'specialist output remains Composer input only';
    });

    await test('semantic_judgment_strict_candidate_ids', () => {
      const candidates = [candidate('character_reader', 'cand_1', 'Rewritten scene.')];
      const good = validateSemanticJudgment(judgmentObject(['cand_1']), candidates, ledger());
      const foreign = validateSemanticJudgment(judgmentObject(['foreign']), candidates, ledger());
      if (!good || foreign) throw new Error('candidate identity validation failed');
      return 'strict IDs accepted';
    });

    await test('fusion_plan_prohibits_constrained_addition', () => {
      const candidates = [candidate('character_reader', 'cand_1', 'Rewritten scene.')];
      const raw = judgmentObject(['cand_1'], 'accept_with_constraints');
      raw.candidate_judgments[0].unsupported_additions = [{ claim: 'Invented sibling', candidate_ids: ['cand_1'], segment_ids: ['mutable_1'], reason: 'not grounded' }];
      raw.candidate_judgments[0].hard_violations = [{
        type: 'secret_leak',
        detail: 'The candidate reveals a writer-only identity.',
        evidence_refs: ['constraint_1'],
        segment_ids: ['mutable_1'],
      }];
      raw.candidate_judgments[0].rejected_elements = [{
        claim: 'Remove the identity reveal.',
        candidate_ids: ['cand_1'],
        segment_ids: ['mutable_1'],
      }];
      const judged = validateSemanticJudgment(raw, candidates, ledger());
      const plan = buildFusionPlan(judged, candidates, ledger(), [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }]);
      if (!plan || !plan.semantic_ready || plan.prohibited_additions.length !== 2) throw new Error('constraint not carried into plan');
      return plan.plan_id;
    });

    await test('judged_candidates_replace_confidence_director', () => {
      const candidates = [candidate('character_reader', 'cand_a', 'Character rewrite.'), candidate('style_reader', 'cand_b', 'Style rewrite.')];
      const raw = judgmentObject(['cand_a', 'cand_b']);
      raw.candidate_judgments[1].verdict = 'reject';
      const judged = validateSemanticJudgment(raw, candidates, ledger());
      const plan = buildFusionPlan(judged, candidates, ledger(), [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }]);
      const result = buildComposerCandidatePool(candidates, judged, plan, [{ id: 'mutable_1', type: 'mutable', text: 'Original.' }], ledger());
      if (result.ranked.mutable_1.length !== 1 || result.ranked.mutable_1[0].candidate_id !== 'cand_a') throw new Error('rejected candidate reached composer pool');
      if ('score' in result.ranked.mutable_1[0]) throw new Error('confidence score still controls selection');
      return result.ranked.mutable_1[0].judge_verdict;
    });

    await test('scheduler_orders_specialist_judge_composer_and_reserves_prover', async () => {
      const originalFetch = globalThis.fetch;
      const calls = [];
      const settings = defaultSettings();
      const roles = ['character_reader', JUDGE_ROLE_ID, COMPOSER_ROLE_ID, PROVER_ROLE_ID]
        .map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
      roles.forEach((role) => {
        const profile = settings.role_profiles[role.role_id];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${role.role_id}-model`;
        profile.timeout_ms = 5000;
      });
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        calls.push(model);
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
          const match = /"candidate_id":"([^"]+)"/.exec(prompt);
          if (!match) throw new Error('judge prompt candidate missing');
          return response(JSON.stringify(judgmentObject([match[1]])));
        }
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) return response(JSON.stringify({ segments: { mutable_1: 'Composer rebuilt the scene with stronger causality.' } }));
        return response(sceneCandidateJson('character_reader', 'Character specialist rebuilt the scene.'));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.router.preset = 'balanced';
        const deadline = createDeadline(30000);
        const segments = [{ id: 'mutable_1', type: 'mutable', text: 'Original scene.', leading_ws: '', trailing_ws: '' }];
        const result = await scheduleRoles(roles, settings.role_profiles, segments, '', segments, deadline, trace, 2, ledger());
        deadline.cancel();
        if (result.failureReason || !result.composerResult) throw new Error(`pipeline failed:${result.failureReason}`);
        if (!result.proverRole || result.proverRole.role_id !== PROVER_ROLE_ID) throw new Error('prover not reserved');
        const judgeIndex = calls.findIndex((model) => model.indexOf(JUDGE_ROLE_ID) >= 0);
        const composerIndex = calls.findIndex((model) => model.indexOf(COMPOSER_ROLE_ID) >= 0);
        if (judgeIndex < 1 || composerIndex <= judgeIndex) throw new Error(`wrong order:${calls.join('>')}`);
        return calls.join('>');
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('adaptive_revision_uses_latest_valid_scene_and_rejudges_before_composer', async () => {
      const originalFetch = globalThis.fetch;
      const calls = [];
      const settings = defaultSettings();
      settings.preset = 'balanced';
      const selectedIds = [
        'character_reader',
        JUDGE_ROLE_ID,
        COMPOSER_ROLE_ID,
        PROVER_ROLE_ID,
      ];
      const configuredIds = selectedIds.concat(['perspective_boundary_rewriter']);
      configuredIds.forEach((roleId) => {
        const profile = settings.role_profiles[roleId];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${roleId}-model`;
        profile.timeout_ms = 5000;
      });
      const roles = selectedIds.map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
      let judgeCalls = 0;
      let adaptiveSawLatestDraft = false;
      let composerSawLineage = false;
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        calls.push(model);
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          judgeCalls++;
          const candidatesMatch = /Scene candidates:\n(.+?)\n\n--- Draft Ledger/s.exec(prompt);
          const promptCandidates = candidatesMatch ? JSON.parse(candidatesMatch[1]) : [];
          const ids = promptCandidates.map((item) => item.candidate_id);
          const judgment = judgmentObject(ids);
          if (judgeCalls === 1) {
            judgment.unresolved_requirements = [{
              issue_type: 'identity_continuity',
              role_id: 'perspective_boundary_rewriter',
              claim: 'The alias and true identity boundary still needs a coherent scene-wide treatment.',
              segment_ids: ['mutable_1'],
              evidence_refs: ['constraint_1'],
            }];
          } else {
            judgment.unresolved_requirements = [];
            judgment.candidate_judgments[0].quality_gains.push(
              { claim: 'Older draft has an additional gain A.', candidate_ids: [ids[0]], segment_ids: ['mutable_1'] },
              { claim: 'Older draft has an additional gain B.', candidate_ids: [ids[0]], segment_ids: ['mutable_1'] },
              { claim: 'Older draft has an additional gain C.', candidate_ids: [ids[0]], segment_ids: ['mutable_1'] }
            );
          }
          return response(JSON.stringify(judgment));
        }
        if (model.indexOf('perspective_boundary_rewriter') >= 0) {
          adaptiveSawLatestDraft = prompt.indexOf('Character specialist rebuilt the complete scene.') >= 0
            && prompt.indexOf('Adaptive Revision Round 1') >= 0;
          return response(sceneCandidateJson(
            'perspective_boundary_rewriter',
            'The complete scene now preserves the alias, identity boundary, and prior character gains.'
          ));
        }
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
          composerSawLineage = prompt.indexOf('Revision Lineage') >= 0
            && prompt.indexOf('perspective_boundary_rewriter') >= 0;
          return response(JSON.stringify({
            segments: {
              mutable_1: 'The Composer unified the revised identity boundary with the character scene.',
            },
          }));
        }
        return response(sceneCandidateJson(
          'character_reader',
          'Character specialist rebuilt the complete scene.'
        ));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.router.preset = 'balanced';
        const deadline = createDeadline(30000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: 'Original scene.',
          leading_ws: '',
          trailing_ws: '',
        }];
        const result = await scheduleRoles(
          roles, settings.role_profiles, segments, '', segments, deadline, trace, 1, ledger()
        );
        deadline.cancel();
        if (result.failureReason || !result.composerResult) {
          throw new Error(`adaptive pipeline failed:${result.failureReason}`);
        }
        const expectedOrder = [
          'character_reader-model',
          `${JUDGE_ROLE_ID}-model`,
          'perspective_boundary_rewriter-model',
          `${JUDGE_ROLE_ID}-model`,
          `${COMPOSER_ROLE_ID}-model`,
        ].join('>');
        if (calls.join('>') !== expectedOrder) {
          throw new Error(`adaptive call order mismatch:${calls.join('>')}`);
        }
        if (!adaptiveSawLatestDraft || !composerSawLineage) {
          throw new Error(`latest draft or lineage missing:${adaptiveSawLatestDraft}/${composerSawLineage}`);
        }
        const round = trace.revision_convergence.rounds[0];
        const adaptiveBudgetStages = arrayFromCollection(trace.scheduler.stage_budgets)
          .map((item) => item.stage);
        if (judgeCalls !== 2
            || trace.revision_convergence.fulfilled !== 1
            || trace.revision_convergence.remaining_requirements !== 0
            || adaptiveBudgetStages.indexOf('adaptive_revision') < 0
            || adaptiveBudgetStages.indexOf('adaptive_rejudge') < 0
            || !round
            || round.status !== 'fulfilled'
            || !round.parent_candidate_id
            || !round.input_draft_digest
            || !round.output_candidate_id
            || result.directorResult.last_valid_candidate.role_id !== 'perspective_boundary_rewriter') {
          throw new Error(`adaptive lineage mismatch:${JSON.stringify({
            judgeCalls,
            convergence: trace.revision_convergence,
            stage_budgets: trace.scheduler.stage_budgets,
            last: result.directorResult.last_valid_candidate,
          })}`);
        }
        return calls.join('>');
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('adaptive_revision_failure_keeps_previous_valid_scene_for_composer', async () => {
      const originalFetch = globalThis.fetch;
      const settings = defaultSettings();
      settings.preset = 'balanced';
      const selectedIds = [
        'character_reader',
        JUDGE_ROLE_ID,
        COMPOSER_ROLE_ID,
        PROVER_ROLE_ID,
      ];
      selectedIds.concat(['perspective_boundary_rewriter']).forEach((roleId) => {
        const profile = settings.role_profiles[roleId];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${roleId}-model`;
        profile.timeout_ms = 5000;
      });
      const roles = selectedIds.map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
      let composerSawPreviousCandidate = false;
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          const candidatesMatch = /Scene candidates:\n(.+?)\n\n--- Draft Ledger/s.exec(prompt);
          const promptCandidates = candidatesMatch ? JSON.parse(candidatesMatch[1]) : [];
          const judgment = judgmentObject(promptCandidates.map((item) => item.candidate_id));
          judgment.unresolved_requirements = [{
            issue_type: 'identity_continuity',
            role_id: 'perspective_boundary_rewriter',
            claim: 'Identity continuity still needs a complete rewrite.',
            segment_ids: ['mutable_1'],
            evidence_refs: ['constraint_1'],
          }];
          return response(JSON.stringify(judgment));
        }
        if (model.indexOf('perspective_boundary_rewriter') >= 0) {
          return response(JSON.stringify({
            schema: 'scene_rewrite_candidates.v1',
            role_id: 'perspective_boundary_rewriter',
            candidates: [],
          }));
        }
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
          composerSawPreviousCandidate = prompt.indexOf('Character specialist remains the last valid complete draft.') >= 0;
          return response(JSON.stringify({
            segments: {
              mutable_1: 'The Composer continued from the prior valid complete draft.',
            },
          }));
        }
        return response(sceneCandidateJson(
          'character_reader',
          'Character specialist remains the last valid complete draft.'
        ));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.router.preset = 'balanced';
        const deadline = createDeadline(30000);
        const segments = [{
          id: 'mutable_1',
          type: 'mutable',
          text: 'Original scene.',
          leading_ws: '',
          trailing_ws: '',
        }];
        const result = await scheduleRoles(
          roles, settings.role_profiles, segments, '', segments, deadline, trace, 1, ledger()
        );
        deadline.cancel();
        if (result.failureReason || !result.composerResult || !composerSawPreviousCandidate) {
          throw new Error(`previous valid scene not inherited:${result.failureReason}/${composerSawPreviousCandidate}`);
        }
        const lastValid = result.directorResult.last_valid_candidate;
        const round = trace.revision_convergence.rounds[0];
        if (!lastValid
            || lastValid.role_id !== 'character_reader'
            || trace.revision_convergence.no_candidate !== 1
            || !round
            || round.status !== 'no_candidate') {
          throw new Error(`failed revision inheritance mismatch:${JSON.stringify({
            lastValid,
            convergence: trace.revision_convergence,
          })}`);
        }
        return `${lastValid.role_id}/${round.status}`;
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('judge_reject_all_discards_candidates_and_runs_direct_composer', async () => {
      const originalFetch = globalThis.fetch;
      let composerCalls = 0;
      const settings = defaultSettings();
      const roles = ['character_reader', JUDGE_ROLE_ID, COMPOSER_ROLE_ID, PROVER_ROLE_ID]
        .map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
      roles.forEach((role) => {
        const profile = settings.role_profiles[role.role_id];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${role.role_id}-model`;
        profile.timeout_ms = 5000;
      });
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
          composerCalls++;
          const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
          if (prompt.indexOf('Candidate rewrite.') >= 0) {
            throw new Error('rejected candidate leaked into direct Composer prompt');
          }
          return response(JSON.stringify({
            segments: { mutable_1: 'Rain pressed against the archive windows while Mira rebuilt the scene from grounded facts.' },
          }));
        }
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
          const match = /"candidate_id":"([^"]+)"/.exec(prompt);
          if (!match) throw new Error('judge prompt candidate missing');
          const rejected = judgmentObject([match[1]], 'reject');
          rejected.candidate_judgments[0].accepted_elements = [];
          rejected.candidate_judgments[0].rejected_elements = [{ claim: 'Not grounded', candidate_ids: [match[1]] }];
          return response(JSON.stringify(rejected));
        }
        return response(sceneCandidateJson('character_reader', 'Candidate rewrite.'));
      };
      try {
        const trace = newTrace('test', 'test');
        trace.router.preset = 'balanced';
        const deadline = createDeadline(30000);
        const segments = [{ id: 'mutable_1', type: 'mutable', text: 'Original scene.', leading_ws: '', trailing_ws: '' }];
        const result = await scheduleRoles(roles, settings.role_profiles, segments, '', segments, deadline, trace, 2, ledger());
        deadline.cancel();
        if (result.failureReason || !result.composerResult || composerCalls !== 1
            || !result.fusionPlan.direct_composer_fallback
            || result.fusionPlan.fallback_reason !== 'semantic_judge_rejected_all'
            || trace.composer.direct_fallback !== true) {
          throw new Error(`reject fallback failed:${JSON.stringify({
            failureReason: result.failureReason,
            composerCalls,
            fusionPlan: result.fusionPlan,
            composer: trace.composer,
          })}`);
        }
        return result.fusionPlan.fallback_reason;
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('empty_specialists_and_failed_judge_both_reach_direct_composer', async () => {
      const originalFetch = globalThis.fetch;
      async function runCase(mode) {
        const settings = defaultSettings();
        const roles = ['character_reader', JUDGE_ROLE_ID, COMPOSER_ROLE_ID, PROVER_ROLE_ID]
          .map((id) => DEFAULT_ROLES.find((role) => role.role_id === id));
        roles.forEach((role) => {
          const profile = settings.role_profiles[role.role_id];
          if (mode === 'composer_unconfigured' && role.role_id === COMPOSER_ROLE_ID) return;
          profile.endpoint = 'https://test.example.com/v1/chat/completions';
          profile.model = `${role.role_id}-model`;
          profile.timeout_ms = 5000;
        });
        let composerCalls = 0;
        let judgeCalls = 0;
        globalThis.fetch = async (_url, options) => {
          const body = JSON.parse(options.body);
          const model = safeString(body.model);
          if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
            composerCalls++;
            if (mode === 'judge_failed_composer_failed') {
              return { ok: false, status: 503, text: async () => '{"error":"composer unavailable"}' };
            }
            return response(JSON.stringify({
              segments: { mutable_1: `Direct Composer rebuilt the complete ${mode} scene with action, consequence, and a stronger ending.` },
            }));
          }
          if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
            judgeCalls++;
            return response('not a semantic judgment');
          }
          if (mode === 'no_candidates') {
            return response(JSON.stringify({
              schema: 'scene_rewrite_candidates.v1',
              role_id: 'character_reader',
              candidates: [],
            }));
          }
          return response(sceneCandidateJson(
            'character_reader',
            'A complete specialist candidate reached the unavailable Judge.'
          ));
        };
        const trace = newTrace('test', 'test');
        trace.router.preset = 'balanced';
        const deadline = createDeadline(30000);
        const segments = [{
          id: 'mutable_1', type: 'mutable', text: 'Original scene.', leading_ws: '', trailing_ws: '',
        }];
        const result = await scheduleRoles(
          roles, settings.role_profiles, segments, '', segments, deadline, trace, 2, ledger()
        );
        deadline.cancel();
        const expectedReason = mode === 'no_candidates'
          ? 'no_scene_candidates'
          : 'semantic_judge_failed';
        const expectedSpecialistFallback = mode === 'judge_failed_composer_failed'
          || mode === 'composer_unconfigured';
        const expectedComposerCalls = mode === 'composer_unconfigured' ? 0 : 1;
        if (result.failureReason || !result.composerResult || composerCalls !== expectedComposerCalls
            || judgeCalls !== (mode === 'no_candidates' ? 0 : 1)
            || result.fusionPlan.fallback_reason !== expectedReason
            || trace.fusion_plan.status !== 'direct_composer_fallback'
            || !!trace.composer.specialist_candidate_recovered !== expectedSpecialistFallback) {
          throw new Error(`direct fallback case failed:${JSON.stringify({
            mode, failureReason: result.failureReason, composerCalls, judgeCalls,
            fallback: result.fusionPlan && result.fusionPlan.fallback_reason,
            tracePlan: trace.fusion_plan,
          })}`);
        }
        return expectedReason;
      }
      try {
        const reasons = [];
        reasons.push(await runCase('no_candidates'));
        reasons.push(await runCase('judge_failed'));
        reasons.push(await runCase('judge_failed_composer_failed'));
        reasons.push(await runCase('composer_unconfigured'));
        return reasons.join(',');
      } finally {
        globalThis.fetch = originalFetch;
      }
    });

    await test('phase_e_f_full_hook_repairs_and_returns_composer_output', async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      const settings = defaultSettings();
      settings.preset = 'fast';
      settings.trace_enabled = true;
      const configuredIds = [
        'character_reader',
        'style_reader',
        JUDGE_ROLE_ID,
        COMPOSER_ROLE_ID,
        PROVER_ROLE_ID,
      ];
      configuredIds.forEach((roleId) => {
        const profile = settings.role_profiles[roleId];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${roleId}-model`;
        profile.timeout_ms = 5000;
      });
      const calls = [];
      const stored = {};
      let composerCalls = 0;
      let proverCalls = 0;
      const initialComposerText = 'Rain tapped the window as Mira studied the open ledger in measured silence.';
      const finalComposerText = 'Rain tapped the window as Mira studied the open ledger, keeping the secret behind her measured silence.';
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => key === SETTINGS_KEY
            ? JSON.stringify(settings)
            : (Object.prototype.hasOwnProperty.call(stored, key) ? stored[key] : null),
          setItem: async (key, value) => { stored[key] = value; },
        },
      };
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        calls.push(model);
        if (model.indexOf(PROVER_ROLE_ID) >= 0) {
          proverCalls++;
          const ledgerMatch = /--- Draft Ledger \(binding source map\) ---\n(.+?)\n--- End Draft Ledger ---/s.exec(prompt);
          if (!ledgerMatch) throw new Error('prover prompt missing draft ledger');
          const proofLedger = JSON.parse(ledgerMatch[1]);
          const proof = {
            schema: 'semantic_proof.v1',
            declared_verdict: 'pass',
            fact_checks: arrayFromCollection(proofLedger.established_facts).map((item) => ({
              ledger_id: item.ledger_id,
              status: 'preserved',
              detail: 'Grounded in final scene.',
              evidence_quote: 'Rain tapped',
            })),
            beat_checks: arrayFromCollection(proofLedger.scene_beats).map((item) => ({
              ledger_id: item.ledger_id,
              status: 'preserved',
              detail: 'Beat retained.',
              evidence_quote: 'open ledger',
            })),
            constraint_checks: arrayFromCollection(proofLedger.hard_constraints).map((item) => ({
              ledger_id: item.ledger_id,
              status: 'satisfied',
              detail: 'Constraint preserved.',
              evidence_quote: '',
            })),
            quality_gain_checks: [],
            residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
              check_id: checkId,
              status: 'clean',
              detail: 'No residual issue found.',
              evidence_quote: '',
              segment_ids: [],
            })),
            hard_violations: [],
            unsupported_additions: [],
            output_contract: {
              language_ok: true,
              turn_boundary_ok: true,
              user_agency_ok: true,
              meta_free: true,
              format_ok: true,
            },
            repair_instructions: [],
          };
          const planMatch = /Fusion Plan:\n(.+?)\n\n--- Draft Ledger/s.exec(prompt);
          const proofPlan = planMatch ? JSON.parse(planMatch[1]) : {};
          proof.quality_gain_checks = arrayFromCollection(proofPlan.required_contributions)
            .map((item) => ({
              contribution_id: item.contribution_id,
              status: 'realized',
              detail: 'The final scene realizes the required specialist contribution.',
              evidence_quote: 'measured silence',
              segment_ids: item.segment_ids,
          }));
          if (proverCalls === 1) {
            proof.declared_verdict = 'repair';
            if (proof.fact_checks.length) {
              proof.fact_checks[0].status = 'missing';
              proof.fact_checks[0].detail = 'The first required fact is not explicit enough.';
            } else if (proof.beat_checks.length) {
              proof.beat_checks[0].status = 'missing';
              proof.beat_checks[0].detail = 'The first required beat is not explicit enough.';
            } else {
              proof.output_contract.turn_boundary_ok = false;
            }
            proof.repair_instructions = [];
          }
          return response(JSON.stringify(proof));
        }
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
          composerCalls++;
          return response(JSON.stringify({
            segments: { [SCENE_REWRITE_SEGMENT_ID]: composerCalls === 1 ? initialComposerText : finalComposerText },
          }));
        }
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          try {
            const candidatesMatch = /Scene candidates:\n(.+?)\n\n--- Draft Ledger/s.exec(prompt);
            const promptCandidates = candidatesMatch ? JSON.parse(candidatesMatch[1]) : [];
            const ids = promptCandidates.map((candidate) => candidate.candidate_id);
            if (ids.length !== 2) throw new Error(`judge expected two scene candidates:${ids.length}`);
            const ledgerMatch = /--- Draft Ledger \(binding source map\) ---\n(.+?)\n--- End Draft Ledger ---/s.exec(prompt);
            if (!ledgerMatch) throw new Error('judge prompt missing draft ledger');
            const judgmentLedger = JSON.parse(ledgerMatch[1]);
            const ledgerIds = ["established_facts", "scene_beats", "hard_constraints"]
              .flatMap((field) => arrayFromCollection(judgmentLedger[field]).map((item) => item.ledger_id));
            const judgment = judgmentObject(ids);
            judgment.candidate_judgments.forEach((item) => {
              item.preserved_ledger_ids = ledgerIds;
            });
            if (!validateSemanticJudgment(judgment, promptCandidates, judgmentLedger)) {
              calls.push('judge_validation_null');
            }
            return response(JSON.stringify(judgment));
          } catch (err) {
            calls.push(`judge_mock_error:${safeString(err && err.message)}`);
            throw err;
          }
        }
        if (model.indexOf('character_reader') >= 0) {
          return response(sceneCandidateJson(
            'character_reader',
            'Mira read the ledger while withholding the secret.',
            SCENE_REWRITE_SEGMENT_ID
          ));
        }
        if (model.indexOf('style_reader') >= 0) {
          return response(sceneCandidateJson(
            'style_reader',
            'Rain pressed softly at the window while Mira studied the ledger.',
            SCENE_REWRITE_SEGMENT_ID
          ));
        }
        throw new Error(`unexpected model:${model}`);
      };
      try {
        pendingMainSnapshot = null;
        const messages = [{ role: 'user', content: 'Continue the scene without revealing the secret.' }];
        await onBeforeRequest(messages, 'model');
        const original = 'Mira watched the rain and kept the secret while the ledger remained open on the table.';
        const output = await onAfterRequest(original, 'model');
        if (output !== finalComposerText) {
          const traces = JSON.parse(stored[TRACE_KEY] || '[]');
          const reason = traces[0] && traces[0].final && traces[0].final.reason;
          const roleErrors = arrayFromCollection(traces[0] && traces[0].roles)
            .filter((entry) => entry.status !== 'fulfilled')
            .map((entry) => `${entry.role_id}:${entry.error_class || entry.error || entry.status}`)
            .join(',');
          throw new Error(`unproven or wrong output returned:${output};calls:${calls.join('>')};reason:${reason || 'none'};roles:${roleErrors || 'none'}`);
        }
        const traceRecord = JSON.parse(stored[TRACE_KEY] || '[]')[0];
        if (!traceRecord || traceRecord.final.enhanced !== true
            || traceRecord.final.semantic_verified !== 'passed'
            || traceRecord.semantic_prover.attempts !== 2
            || traceRecord.semantic_prover.repair_attempted !== true
            || traceRecord.semantic_prover.repair_synthesized !== true
            || traceRecord.semantic_prover.residual_quality_checked !== PROOF_RESIDUAL_QUALITY_IDS.length
            || traceRecord.semantic_prover.residual_quality_issues !== 0
            || !traceRecord.applied_evidence.some((item) =>
              item.changed
              && item.source === 'composer'
              && arrayFromCollection(item.role_contributions).length > 0
            )) {
          throw new Error(`final proof trace mismatch:${JSON.stringify(traceRecord && {
            final: traceRecord.final,
            prover: traceRecord.semantic_prover,
            applied: traceRecord.applied_evidence,
          })}`);
        }
        const judgeIndex = calls.findIndex((model) => model.indexOf(JUDGE_ROLE_ID) >= 0);
        const composerIndexes = calls.map((model, index) => model.indexOf(COMPOSER_ROLE_ID) >= 0 ? index : -1)
          .filter((index) => index >= 0);
        const proverIndexes = calls.map((model, index) => model.indexOf(PROVER_ROLE_ID) >= 0 ? index : -1)
          .filter((index) => index >= 0);
        if (judgeIndex < 2 || composerIndexes.length !== 2 || proverIndexes.length !== 2
            || composerIndexes[0] <= judgeIndex
            || proverIndexes[0] <= composerIndexes[0]
            || composerIndexes[1] <= proverIndexes[0]
            || proverIndexes[1] <= composerIndexes[1]) {
          throw new Error(`wrong full hook order:${calls.join('>')}`);
        }
        const providerCallsAfterFirstRun = calls.length;
        await onBeforeRequest(messages, 'model');
        const reusedOutput = await onAfterRequest(original, 'model');
        const reusedTrace = JSON.parse(stored[TRACE_KEY] || '[]')[0];
        if (reusedOutput !== finalComposerText
            || calls.length !== providerCallsAfterFirstRun
            || !reusedTrace
            || reusedTrace.output_reuse.hit !== true
            || reusedTrace.output_reuse.key !== 'strict') {
          throw new Error(`completed output was not reused:${JSON.stringify({
            output: reusedOutput,
            calls_before: providerCallsAfterFirstRun,
            calls_after: calls.length,
            output_reuse: reusedTrace && reusedTrace.output_reuse,
          })}`);
        }
        return `${calls.join('>')} > reuse:0-provider-calls`;
      } finally {
        pendingMainSnapshot = null;
        completedOutputCache = [];
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    await test('full_hook_all_specialists_empty_uses_direct_composer_then_prover', async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      const settings = defaultSettings();
      settings.preset = 'fast';
      settings.trace_enabled = true;
      DEFAULT_ROLES.filter((role) => !role.is_input_planner).forEach((role) => {
        const profile = settings.role_profiles[role.role_id];
        profile.endpoint = 'https://test.example.com/v1/chat/completions';
        profile.model = `${role.role_id}-model`;
        profile.timeout_ms = 5000;
      });
      const stored = {};
      const calls = [];
      let judgeCalls = 0;
      const finalText = 'Rain struck the archive windows as Mira shut the ledger, crossed the room, and answered Rowan without exposing the sealed name.';
      globalThis.Risuai = {
        pluginStorage: {
          getItem: async (key) => key === SETTINGS_KEY
            ? JSON.stringify(settings)
            : (Object.prototype.hasOwnProperty.call(stored, key) ? stored[key] : null),
          setItem: async (key, value) => { stored[key] = value; },
        },
      };
      globalThis.fetch = async (_url, options) => {
        const body = JSON.parse(options.body);
        const model = safeString(body.model);
        const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
        calls.push(model);
        if (model.indexOf(PROVER_ROLE_ID) >= 0) {
          const ledgerMatch = /--- Draft Ledger \(binding source map\) ---\n(.+?)\n--- End Draft Ledger ---/s.exec(prompt);
          if (!ledgerMatch) throw new Error('fallback Prover prompt missing Draft Ledger');
          const proofLedger = JSON.parse(ledgerMatch[1]);
          return response(JSON.stringify({
            schema: 'semantic_proof.v1',
            declared_verdict: 'pass',
            fact_checks: arrayFromCollection(proofLedger.established_facts).map((item) => ({
              ledger_id: item.ledger_id, status: 'preserved', detail: 'Preserved.', evidence_quote: '',
            })),
            beat_checks: arrayFromCollection(proofLedger.scene_beats).map((item) => ({
              ledger_id: item.ledger_id, status: 'preserved', detail: 'Preserved.', evidence_quote: '',
            })),
            constraint_checks: arrayFromCollection(proofLedger.hard_constraints).map((item) => ({
              ledger_id: item.ledger_id, status: 'satisfied', detail: 'Satisfied.', evidence_quote: '',
            })),
            quality_gain_checks: [],
            residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
              check_id: checkId, status: 'clean', detail: 'Clean.', evidence_quote: '', segment_ids: [],
            })),
            hard_violations: [],
            unsupported_additions: [],
            output_contract: {
              language_ok: true, turn_boundary_ok: true, user_agency_ok: true, meta_free: true, format_ok: true,
            },
            repair_instructions: [],
          }));
        }
        if (model.indexOf(COMPOSER_ROLE_ID) >= 0) return response(finalText);
        if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
          judgeCalls++;
          throw new Error('Judge must not run without candidates');
        }
        const roleId = model.replace(/-model$/, '');
        return response(JSON.stringify({
          schema: 'scene_rewrite_candidates.v1',
          role_id: roleId,
          candidates: [],
        }));
      };
      try {
        pendingMainSnapshot = null;
        completedOutputCache = [];
        const messages = [{ role: 'user', content: 'Continue without revealing the sealed identity.' }];
        await onBeforeRequest(messages, 'model');
        const original = 'Mira sat beside the open ledger while rain touched the window.';
        const output = await onAfterRequest(original, 'model');
        const traceRecord = JSON.parse(stored[TRACE_KEY] || '[]')[0];
        if (output !== finalText || judgeCalls !== 0 || !traceRecord
            || traceRecord.final.enhanced !== true
            || traceRecord.final.semantic_verified !== 'passed'
            || traceRecord.composer.direct_fallback !== true
            || traceRecord.fusion_plan.fallback_reason !== 'no_scene_candidates'
            || calls.filter((model) => model.indexOf(COMPOSER_ROLE_ID) >= 0).length !== 1
            || calls.filter((model) => model.indexOf(PROVER_ROLE_ID) >= 0).length !== 1) {
          throw new Error(`full direct fallback failed:${JSON.stringify({
            output, judgeCalls, calls, final: traceRecord && traceRecord.final,
            composer: traceRecord && traceRecord.composer,
            fusionPlan: traceRecord && traceRecord.fusion_plan,
          })}`);
        }
        return 'empty specialists -> direct Composer -> Prover -> enhanced output';
      } finally {
        pendingMainSnapshot = null;
        completedOutputCache = [];
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    await test('full_hook_recoverable_failure_matrix_returns_changed_output', async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      async function runCase(mode, preset, original, expected) {
        const settings = defaultSettings();
        settings.preset = preset;
        settings.trace_enabled = true;
        DEFAULT_ROLES.filter((role) => !role.is_input_planner).forEach((role) => {
          const profile = settings.role_profiles[role.role_id];
          if (mode === 'judge_unconfigured' && role.role_id === JUDGE_ROLE_ID) return;
          if (mode === 'prover_unconfigured' && role.role_id === PROVER_ROLE_ID) return;
          profile.endpoint = 'https://test.example.com/v1/chat/completions';
          profile.model = `${role.role_id}-model`;
          profile.timeout_ms = 5000;
        });
        if (mode === 'prover_fallback') {
          const prover = settings.role_profiles[PROVER_ROLE_ID];
          prover.fallback_provider = 'openai_compatible';
          prover.fallback_endpoint = 'https://fallback.example.com/v1/chat/completions';
          prover.fallback_model = 'semantic-prover-fallback';
        }
        const stored = {};
        const calls = [];
        let composerCalls = 0;
        let judgeCalls = 0;
        globalThis.Risuai = {
          pluginStorage: {
            getItem: async (key) => key === SETTINGS_KEY
              ? JSON.stringify(settings)
              : (Object.prototype.hasOwnProperty.call(stored, key) ? stored[key] : null),
            setItem: async (key, value) => { stored[key] = value; },
          },
        };
        globalThis.fetch = async (_url, options) => {
          const body = JSON.parse(options.body);
          const model = safeString(body.model);
          const prompt = safeString(body.messages && body.messages[1] && body.messages[1].content);
          calls.push(model);
          if (model === 'semantic-prover-fallback') {
            return response(semanticProofJsonFromPrompt(prompt));
          }
          if (model.indexOf(PROVER_ROLE_ID) >= 0) {
            if (mode === 'prover_fallback') {
              return { ok: false, status: 503, text: async () => '{"error":"temporary unavailable"}' };
            }
            if (mode === 'prover_transport_fail') {
              return { ok: false, status: 503, text: async () => '{"error":"prover unavailable"}' };
            }
            if (mode === 'prover_declared_fail') {
              const proof = JSON.parse(semanticProofJsonFromPrompt(prompt));
              proof.declared_verdict = 'fail';
              return response(JSON.stringify(proof));
            }
            return response(semanticProofJsonFromPrompt(prompt));
          }
          if (model.indexOf(COMPOSER_ROLE_ID) >= 0) {
            composerCalls++;
            if (mode === 'weak_balanced') {
              return response(composerCalls === 1 ? original : expected);
            }
            if (mode === 'duplicate_verifier') {
              const tokenMatch = prompt.match(/\[\[ACR_EXACT_[^\]]+\]\]/);
              if (!tokenMatch) throw new Error('duplicate recovery mock missing exact token');
              const token = tokenMatch[0];
              const duplicated = 'A newly duplicated rewritten paragraph is deliberately long enough to represent a repeated prose block that the structural verifier must reject.';
              return response(composerCalls === 1
                ? `${duplicated}\n${token}\n${duplicated}`
                : expected.replace('__TOKEN__', token));
            }
            if (mode === 'terminal_artifact_expansion') {
              const tokenMatch = prompt.match(/\[\[ACR_EXACT_[^\]]+\]\]/);
              if (!tokenMatch) throw new Error('terminal artifact mock missing token');
              return response(expected.replace('__TOKEN__', tokenMatch[0]));
            }
            return response(expected);
          }
          if (model.indexOf(JUDGE_ROLE_ID) >= 0) {
            judgeCalls++;
            throw new Error(`Judge should not run in ${mode}`);
          }
          const roleId = model.replace(/-model$/, '');
          if (mode === 'judge_unconfigured') {
            return response(sceneCandidateJson(
              roleId,
              `The ${roleId} independently rebuilt the entire scene around the grounded exchange.`,
              SCENE_REWRITE_SEGMENT_ID
            ));
          }
          return response(JSON.stringify({
            schema: 'scene_rewrite_candidates.v1',
            role_id: roleId,
            candidates: [],
          }));
        };
        pendingMainSnapshot = null;
        completedOutputCache = [];
        const messages = [{ role: 'user', content: 'Continue the scene and improve it materially.' }];
        await onBeforeRequest(messages, 'model');
        const output = await onAfterRequest(original, 'model');
        const traceRecord = JSON.parse(stored[TRACE_KEY] || '[]')[0];
        return { output, traceRecord, calls, composerCalls, judgeCalls };
      }
      try {
        const weakOriginal = 'Mira sat beside the open ledger while rain touched the window and the unresolved accusation remained between her and Rowan.';
        const weakExpected = 'Rain drove silver lines across the archive glass. Mira closed the ledger before Rowan could reach it, then met his accusation with a silence that sharpened the distance between them.';
        const weak = await runCase('weak_balanced', 'balanced', weakOriginal, weakExpected);
        if (weak.output !== weakExpected || weak.composerCalls !== 2
            || !weak.traceRecord || weak.traceRecord.final.enhanced !== true
            || weak.traceRecord.composer.semantic_retry !== 1) {
          throw new Error(`balanced weak rewrite recovery failed:${JSON.stringify({
            output: weak.output, composerCalls: weak.composerCalls,
            final: weak.traceRecord && weak.traceRecord.final,
            composer: weak.traceRecord && weak.traceRecord.composer,
          })}`);
        }

        const image = '<img src="/api/asset/example.png">';
        const duplicateOriginal = `The first original paragraph carries a distinct opening action and enough length for duplicate comparison.\n${image}\nThe second original paragraph carries a different response and enough length for duplicate comparison.`;
        const duplicateExpected = 'Rain rattled against the shutters as Mira seized the ledger before the seal could be read.\n__TOKEN__\nRowan stopped at the threshold, and the question he had prepared died when he saw her hand covering the name.';
        const duplicate = await runCase(
          'duplicate_verifier', 'fast', duplicateOriginal,
          duplicateExpected
        );
        if (duplicate.output.indexOf('Mira seized the ledger') < 0
            || duplicate.output.indexOf('Rowan stopped at the threshold') < 0
            || duplicate.output.indexOf(image) < 0
            || duplicate.composerCalls !== 2
            || !duplicate.traceRecord
            || duplicate.traceRecord.final.enhanced !== true
            || duplicate.traceRecord.composer.preproof_recovery_attempted !== true
            || duplicate.traceRecord.composer.preproof_recovery_succeeded !== true) {
          throw new Error(`verifier recovery failed:${JSON.stringify({
            output: duplicate.output, composerCalls: duplicate.composerCalls,
            final: duplicate.traceRecord && duplicate.traceRecord.final,
            composer: duplicate.traceRecord && duplicate.traceRecord.composer,
          })}`);
        }

        const statusBlock = '<STATUS>[ DATE: 2024.05.01 | PT: 1500 ]</STATUS>';
        const terminalOriginal = `Mira watched the sealed door without speaking.\n${statusBlock}`;
        const terminalExpected = 'Mira crossed the room before Rowan could finish the accusation.\n__TOKEN__\nThe latch answered with a hard click, turning their silence into a decision.';
        const terminal = await runCase(
          'terminal_artifact_expansion', 'quality', terminalOriginal, terminalExpected
        );
        if (!terminal.traceRecord || terminal.traceRecord.final.enhanced !== true
            || terminal.output.indexOf(statusBlock) < 0
            || !terminal.output.endsWith('The latch answered with a hard click, turning their silence into a decision.')
            || terminal.output === terminalOriginal
            || terminal.composerCalls !== 1) {
          throw new Error(`terminal artifact expansion failed:${JSON.stringify({
            output: terminal.output, composerCalls: terminal.composerCalls,
            final: terminal.traceRecord && terminal.traceRecord.final,
          })}`);
        }

        const judgeOriginal = 'Mira watched Rowan from the archive doorway while the sealed ledger remained open behind her.';
        const judgeExpected = 'Mira stepped out of the archive and pulled the door shut with the ledger still open inside. Rowan read the warning in the gesture and lowered the question he had been about to ask.';
        const missingJudge = await runCase('judge_unconfigured', 'fast', judgeOriginal, judgeExpected);
        if (missingJudge.output !== judgeExpected || missingJudge.judgeCalls !== 0
            || !missingJudge.traceRecord || missingJudge.traceRecord.final.enhanced !== true
            || missingJudge.traceRecord.composer.direct_fallback !== true
            || missingJudge.traceRecord.fusion_plan.fallback_reason !== 'semantic_judge_not_configured') {
          throw new Error(`missing Judge direct Composer failed:${JSON.stringify({
            output: missingJudge.output, judgeCalls: missingJudge.judgeCalls,
            final: missingJudge.traceRecord && missingJudge.traceRecord.final,
            fusionPlan: missingJudge.traceRecord && missingJudge.traceRecord.fusion_plan,
          })}`);
        }

        const proverOriginal = 'Mira waited by the archive window with the ledger open and the sealed name hidden under her palm.';
        const proverExpected = 'Thunder rolled above the archive. Mira snapped the ledger closed beneath her palm, forcing Rowan to choose between pressing the forbidden name and preserving the fragile trust between them.';
        const proverFallback = await runCase('prover_fallback', 'fast', proverOriginal, proverExpected);
        const fallbackTrace = arrayFromCollection(proverFallback.traceRecord && proverFallback.traceRecord.roles)
          .find((entry) => entry.role_id === PROVER_ROLE_ID && entry.fallback === true);
        if (proverFallback.output !== proverExpected || !proverFallback.traceRecord
            || proverFallback.traceRecord.final.enhanced !== true || !fallbackTrace) {
          throw new Error(`Prover fallback failed:${JSON.stringify({
            output: proverFallback.output,
            final: proverFallback.traceRecord && proverFallback.traceRecord.final,
            roles: proverFallback.traceRecord && proverFallback.traceRecord.roles,
          })}`);
        }

        const proverUnconfigured = await runCase(
          'prover_unconfigured', 'fast', proverOriginal, proverExpected
        );
        if (proverUnconfigured.output !== proverExpected || !proverUnconfigured.traceRecord
            || proverUnconfigured.traceRecord.final.enhanced !== true
            || proverUnconfigured.traceRecord.final.semantic_verified !== 'degraded') {
          throw new Error(`unconfigured Prover discarded changed scene:${JSON.stringify({
            output: proverUnconfigured.output,
            final: proverUnconfigured.traceRecord && proverUnconfigured.traceRecord.final,
          })}`);
        }

        const proverTransportFail = await runCase(
          'prover_transport_fail', 'fast', proverOriginal, proverExpected
        );
        if (proverTransportFail.output !== proverExpected || !proverTransportFail.traceRecord
            || proverTransportFail.traceRecord.final.enhanced !== true
            || proverTransportFail.traceRecord.final.semantic_verified !== 'degraded') {
          throw new Error(`failed Prover discarded changed scene:${JSON.stringify({
            output: proverTransportFail.output,
            final: proverTransportFail.traceRecord && proverTransportFail.traceRecord.final,
          })}`);
        }

        const proverDeclaredFail = await runCase(
          'prover_declared_fail', 'fast', proverOriginal, proverExpected
        );
        if (proverDeclaredFail.output !== proverExpected || !proverDeclaredFail.traceRecord
            || proverDeclaredFail.traceRecord.final.enhanced !== true
            || proverDeclaredFail.traceRecord.final.semantic_verified !== 'degraded') {
          throw new Error(`blocking Prover verdict discarded changed scene:${JSON.stringify({
            output: proverDeclaredFail.output,
            final: proverDeclaredFail.traceRecord && proverDeclaredFail.traceRecord.final,
          })}`);
        }
        return 'changed scenes survive weak-copy recovery, artifact repair, missing Judge, and all Prover failure states';
      } finally {
        pendingMainSnapshot = null;
        completedOutputCache = [];
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
    });

    await test('archive_center_optional_enhancement_builds_typed_ledger', () => {
      const originalBridge = globalThis[ARCHIVE_CENTER_BRIDGE_KEY];
      const input = 'Continue the rain-soaked council scene.';
      const settings = defaultSettings();
      const lifecycleState = 'current_request_payload_applied';
      const sessionId = 'archive-test-session';
      const turnIndex = 9;
      const payloadPlanId = 'archive-test-plan';
      const inputDigest = stableDigest(input);
      const lifecycleDigest = stableDigest([
        sessionId, turnIndex, inputDigest, input.length, payloadPlanId, lifecycleState,
      ].join('|'));
      try {
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY] = {
          contract_version: ARCHIVE_CENTER_BRIDGE_CONTRACT,
          owner: 'archive_center_host_adapter',
          transport_only: true,
          lifecycle_state: lifecycleState,
          input_digest: inputDigest,
          input_chars: input.length,
          lifecycle_digest: lifecycleDigest,
          input_bindings: [{ digest: inputDigest, chars: input.length, lifecycle_digest: lifecycleDigest }],
          session_id: sessionId,
          turn_index: turnIndex,
          payload_plan_id: payloadPlanId,
          enhancement_contract: {
            contract_version: ARCHIVE_CENTER_ENHANCEMENT_CONTRACT,
            status: 'ready',
            owner: 'go',
            read_only: true,
            optional_enhancement: true,
            standalone_fallback_required: true,
            session_id: sessionId,
            turn_index: turnIndex,
            same_turn_critic_result_available: false,
            lane_semantics: {
              event_recent: 'objective_event_memory',
              subjective_relationship: 'perspective_scoped_subjective',
              protected_secret: 'writer_only',
              unresolved_goal: 'open_thread_or_goal',
              direct_evidence: 'accepted_or_verified_grounded_evidence',
              output_guidance: 'supervisor_current_turn',
            },
            privacy: {
              subjective_not_objective_truth: true,
              protected_secret_writer_only: true,
              no_state_write: true,
            },
            feature_status: {
              long_term_memory: { status: 'available', selected_count: 2 },
              subjective_memory: { status: 'available', selected_count: 1 },
              protected_secret: { status: 'available', selected_count: 1 },
              supervisor_guidance: { status: 'available', selected_count: 1 },
              critic_curated_evidence: {
                status: 'available',
                selected_count: 1,
                source_mode: 'prior_accepted_or_verified_direct_evidence',
                same_turn_result: false,
              },
            },
          },
          memory_delivery_plan: {
            contract_version: 'memory_delivery_plan.v1',
            classes: [
              { key: 'event_recent', selected_count: 1, text: '[Recent Event]\n- The council convened during the storm.' },
              { key: 'subjective_relationship', selected_count: 1, text: '[Subjective Relationship]\n- Mira privately distrusts the envoy.' },
              { key: 'protected_secret', selected_count: 1, text: '[Protected Secret]\n- The envoy is the missing heir.' },
              { key: 'unresolved_goal', selected_count: 1, text: '[Unresolved Goal]\n- Decide whether to open the sealed letter.' },
              { key: 'direct_evidence', selected_count: 1, text: '[Direct Evidence]\n- Mira previously hid the signet ring.' },
            ],
          },
          memory_delivery_lineage: {
            contract_version: 'memory_delivery_lineage.v1',
          },
          guidance_application_trace: {
            contract_version: 'guidance_application_trace.v1',
            applied_count: 1,
            final_text: '[Supervisor Guidance]\nEscalate the council conflict through action and dialogue.',
          },
          payload_application_observation: {
            status: 'ready',
            payload_application_status: 'applied',
            lifecycle_state: lifecycleState,
            payload_plan_id: payloadPlanId,
          },
        };
        const envelope = readArchiveCenterEnhancement(input);
        if (!envelope) throw new Error('valid Archive Center envelope was rejected');
        const context = {
          system_context: '',
          recent_chat: '',
          latest_user_input: input,
          character: '',
          persona: '',
          current_chat: '',
          lorebook: '',
          lorebook_active: '',
          memory: '',
          memory_fields: [],
          bounded_context_block: '',
          manifest: null,
          sources: {},
        };
        const trace = newTrace('afterRequest', 'model');
        applyArchiveCenterEnhancement(context, envelope, settings, trace);
        const attached = attachArchiveCenterContextToTurnContract({
          schema: 'turn_contract.v1',
          contract_id: 'standalone-contract',
          evidence_refs: [],
          source_availability: {},
        }, context.manifest);
        const segments = buildSegmentMap('The council doors opened.', settings);
        const draftLedger = buildDraftLedger('The council doors opened.', segments, attached);
        const establishedText = draftLedger.established_facts.map((item) => item.text).join('\n');
        const relationshipText = draftLedger.relationship_state.map((item) => item.text).join('\n');
        const secretText = draftLedger.secrets_and_reveal.map((item) => item.text).join('\n');
        const directiveText = draftLedger.response_directives.map((item) => item.text).join('\n');
        if (!/council convened|signet ring/i.test(establishedText)) {
          throw new Error(`objective Archive memory missing:${establishedText}`);
        }
        if (/privately distrusts/i.test(establishedText) || !/privately distrusts/i.test(relationshipText)) {
          throw new Error('subjective memory escaped its perspective-scoped ledger lane');
        }
        if (!/missing heir/i.test(secretText)) {
          throw new Error('writer-only secret missing from protected ledger lane');
        }
        if (!/action and dialogue/i.test(directiveText)) {
          throw new Error('Supervisor guidance missing from response directives');
        }
        const signalIds = detectSceneSignals(segments, context).map((signal) => signal.id);
        [
          'archive_center_available',
          'archive_subjective_memory',
          'archive_protected_secret',
          'archive_supervisor_guidance',
          'archive_critic_evidence',
        ].forEach((signalId) => {
          if (!signalIds.includes(signalId)) throw new Error(`missing Archive router signal:${signalId}`);
        });
        if (trace.archive_center.same_turn_critic_result_available !== false) {
          throw new Error('prior Critic evidence was misrepresented as a same-turn result');
        }
        return `lanes=${context.archive_center_context.lanes.length} signals=${signalIds.length}`;
      } finally {
        if (originalBridge === undefined) delete globalThis[ARCHIVE_CENTER_BRIDGE_KEY];
        else globalThis[ARCHIVE_CENTER_BRIDGE_KEY] = originalBridge;
      }
    });

    await test('archive_center_bridge_lifecycle_fences_and_long_generation', () => {
      const originalBridge = globalThis[ARCHIVE_CENTER_BRIDGE_KEY];
      const input = 'Continue this scene.';
      const lifecycleState = 'current_request_payload_applied';
      const sessionId = 'archive-long-generation-session';
      const turnIndex = 12;
      const payloadPlanId = 'archive-long-generation-plan';
      const boundDigest = stableDigest(input);
      const boundLifecycleDigest = stableDigest([
        sessionId, turnIndex, boundDigest, input.length, payloadPlanId, lifecycleState,
      ].join('|'));
      try {
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY] = {
          contract_version: ARCHIVE_CENTER_BRIDGE_CONTRACT,
          owner: 'archive_center_host_adapter',
          transport_only: true,
          lifecycle_state: lifecycleState,
          published_at_ms: Date.now() - (16 * 60 * 1000),
          input_bindings: [{ digest: stableDigest('a different turn'), chars: 16 }],
          session_id: sessionId,
          turn_index: turnIndex,
          payload_plan_id: payloadPlanId,
          enhancement_contract: {
            contract_version: ARCHIVE_CENTER_ENHANCEMENT_CONTRACT,
            owner: 'go',
            read_only: true,
            optional_enhancement: true,
            standalone_fallback_required: true,
            session_id: sessionId,
            turn_index: turnIndex,
          },
          payload_application_observation: {
            payload_application_status: 'applied',
            lifecycle_state: lifecycleState,
            payload_plan_id: payloadPlanId,
          },
        };
        if (readArchiveCenterEnhancement(input) !== null) {
          throw new Error('cross-turn Archive Center bridge was accepted');
        }
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY].input_bindings = [
          { digest: boundDigest, chars: input.length, lifecycle_digest: boundLifecycleDigest },
        ];
        if (!readArchiveCenterEnhancement(input)) {
          throw new Error('lifecycle-bound bridge was rejected after a generation longer than 15 minutes');
        }
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY].enhancement_contract.session_id = 'different-session';
        if (readArchiveCenterEnhancement(input) !== null) {
          throw new Error('cross-session Archive Center bridge was accepted');
        }
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY].enhancement_contract.session_id = sessionId;
        globalThis[ARCHIVE_CENTER_BRIDGE_KEY].lifecycle_state = 'superseded_request';
        if (readArchiveCenterEnhancement(input) !== null) {
          throw new Error('superseded Archive Center bridge was accepted');
        }
        delete globalThis[ARCHIVE_CENTER_BRIDGE_KEY];
        if (readArchiveCenterEnhancement(input) !== null) {
          throw new Error('missing Archive Center bridge changed standalone behavior');
        }
        return 'input/session/lifecycle fences enforced; long generation accepted';
      } finally {
        if (originalBridge === undefined) delete globalThis[ARCHIVE_CENTER_BRIDGE_KEY];
        else globalThis[ARCHIVE_CENTER_BRIDGE_KEY] = originalBridge;
      }
    });

    await test('protected_segments_remain_exact', () => {
      const source = 'Before <img src="x"> after.';
      const segments = buildSegmentMap(source, defaultSettings());
      const mutable = mutableSegments(segments);
      const composed = { segments: {} };
      mutable.forEach((segment) => { composed.segments[segment.id] = mutableCoreText(segment).toUpperCase(); });
      const assembled = assembleOutput(segments, composed);
      const verification = verifyOutput(segments, assembled.finalSegments, assembled.output, source);
      if (!verification.pass || assembled.output.indexOf('<img src="x">') < 0) throw new Error('protected segment changed');
      return 'protected exact';
    });

    return {
      total: results.length,
      passed: results.filter((item) => item.pass).length,
      failed: results.filter((item) => !item.pass).length,
      results,
    };
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
        if (trace.input_enhance.transport_cancellation !== "requested_unverified"
            || trace.input_enhance.active_calls_final !== 1) {
          throw new Error(
            `cancellation:${trace.input_enhance.transport_cancellation}/active_calls:${trace.input_enhance.active_calls_final}`
          );
        }
        return "deadline requested cancellation; uncooperative transport remains explicitly unverified";
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

    await check("input_13_user_directive_not_promoted_to_narrative_fact", async () => {
      const directive = {
        text: "Continue the response in concise prose.",
        evidence_refs: ["payload_user_input"],
        evidence_quote: "Continue the response in concise prose.",
      };
      const narrativeFact = {
        text: "The bridge collapsed before dawn.",
        evidence_refs: ["payload_recent_chat"],
        evidence_quote: "The bridge collapsed before dawn.",
      };
      const openThread = {
        text: "Mira still needs to cross the river.",
        evidence_refs: ["payload_recent_chat"],
        evidence_quote: "Mira still needs to cross the river.",
      };
      const contract = {
        contract_id: "directive_separation",
        contract_digest: "directive_separation_digest",
        immutable_constraints: [directive, narrativeFact],
        character_visible_facts: [],
        scene_state: [],
        open_threads: [openThread],
        turn_objectives: [directive],
        prose_targets: [],
        relationship_and_emotion_state: [],
        agency_and_pov_constraints: [],
        writer_only_secrets: [],
        character_knowledge_scopes: [],
        identity_and_alias_map: [],
        forbidden_regressions: [],
      };
      const source = "Mira watched the river from the broken bridge.";
      const ledger = buildDraftLedger(source, buildSegmentMap(source, defaultSettings()), contract);
      const texts = (items) => arrayFromCollection(items).map((item) => safeString(item.text));
      if (texts(ledger.established_facts).includes(directive.text)) {
        throw new Error("user_directive_promoted_to_fact");
      }
      if (!texts(ledger.established_facts).includes(narrativeFact.text)) {
        throw new Error("grounded_narrative_fact_removed");
      }
      if (texts(ledger.scene_beats).includes(directive.text)
          || !texts(ledger.scene_beats).includes(openThread.text)) {
        throw new Error("scene_beat_separation_failed");
      }
      if (!texts(ledger.response_directives).includes(directive.text)
          || !texts(ledger.hard_constraints).includes(directive.text)) {
        throw new Error("response_directive_contract_missing");
      }
      return "user command remains a response directive; narrative facts and hooks remain separate";
    });

    await check("input_14_planner_provider_response_fuses", async () => withMockRisu(baseMock({
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
            || traces[0].budget.input_attempt_used !== 3
            || traces[0].final.reason === "pipeline_error") {
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
          || OUTPUT_HTTP_ATTEMPT_BUDGET.fast !== 8
          || OUTPUT_HTTP_ATTEMPT_BUDGET.balanced !== 11
          || OUTPUT_HTTP_ATTEMPT_BUDGET.quality !== 15) {
        throw new Error("wrong_stage_attempt_budget");
      }
      const trace = newTrace("test", "main");
      if (trace.budget.http_attempt_max !== OUTPUT_HTTP_ATTEMPT_BUDGET.balanced) {
        throw new Error("default_output_budget_not_initialized");
      }
      return "input=0/3/4 output=8/11/15";
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

    await check("batch1_new_duplicate_block_triggers_rewrite_warning", async () => {
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
      if (!verification.pass || verification.warnings.indexOf("duplicate_segment:mutable_2") < 0) {
        throw new Error(`new_duplicate_warning_missing:${JSON.stringify(verification)}`);
      }
      return "new long duplicate requests recovery without discarding changed prose";
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

    await check("batch1_specialist_candidate_never_applied_without_composer", async () => {
      const segments = [{ id: "mutable_1", type: "mutable", text: "Original paragraph." }];
      const director = {
        ranked: {
          mutable_1: [{
            role_id: "character_reader",
            rewrite: "Inferior rewrite.",
            judge_verdict: "accept",
            identical_to_original: false,
          }],
        },
      };
      const assembled = assembleOutput(segments, null, director);
      if (assembled.output !== "Original paragraph." || assembled.changed) {
        throw new Error(`specialist_candidate_applied:${assembled.output}`);
      }
      return "specialist candidate stayed inside Composer input boundary";
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

    await check("batch1_dense_html_uses_one_logical_scene", async () => {
      const html = Array.from({ length: 48 }, (_item, index) =>
        `<span data-i="${index}">line ${index}</span>`
      ).join("\n");
      const source = `Scene start.\n${html}\nScene end.`;
      const physical = buildSegmentMap(source, defaultSettings());
      const summary = summarizeSegments(physical);
      const frame = buildSceneRewriteFrame(physical, source);
      if (summary.protected < 90 || summary.mutable < 40) {
        throw new Error(`dense_fixture_not_reproduced:${JSON.stringify(summary)}`);
      }
      if (frame.logical_segments.length !== 1
          || frame.logical_segments[0].id !== SCENE_REWRITE_SEGMENT_ID
          || frame.tokens.length <= 0
          || frame.tokens.length >= summary.protected + summary.mutable) {
        throw new Error(`logical_scene_not_compacted:${JSON.stringify({
          logical: frame.logical_segments.length,
          tokens: frame.tokens.length,
          summary,
        })}`);
      }
      return `physical=${physical.length} logical=${frame.logical_segments.length} placeholders=${frame.tokens.length}`;
    });

    await check("batch1_scene_frame_groups_structural_whitespace", async () => {
      const source = '<div class="han-image-container">\n<img src="x">\n</div>\nNarrative begins.';
      const physical = buildSegmentMap(source, defaultSettings());
      const frame = buildSceneRewriteFrame(physical, source);
      if (frame.tokens.length !== 1) {
        throw new Error(`structural_group_not_compacted:${frame.tokens.length}`);
      }
      const valid = frame.logical_segments[0].text.replace(
        "Narrative begins.",
        "The revised narrative begins with immediate pressure."
      );
      const restored = restoreSceneRewriteFrame(frame, valid);
      if (!restored.pass
          || restored.output.indexOf('<div class="han-image-container">\n<img src="x">\n</div>\n') !== 0) {
        throw new Error(`structural_group_restore_failed:${restored.code}:${JSON.stringify(restored.output)}`);
      }
      const expanded = restoreSceneRewriteFrame(frame, `A new opening beat.\n${valid}`);
      if (!expanded.pass
          || expanded.output.indexOf('A new opening beat.\n<div class="han-image-container">') !== 0
          || expanded.inserted_slots.indexOf(0) < 0) {
        throw new Error(`container_slot_rewrite_rejected:${expanded.code}`);
      }
      return "host artifact remains exact while whole-scene prose expands around it";
    });

    await check("batch1_scene_frame_accepts_prose_after_terminal_artifact", async () => {
      const source = 'Opening scene.\n<sys>Rule text</sys>\nClosing beat.\n<STATUS>[ PT: 1500 ]</STATUS>';
      const settings = defaultSettings();
      const physical = buildSegmentMap(source, settings);
      const frame = buildSceneRewriteFrame(physical, source);
      const terminalToken = frame.tokens[frame.tokens.length - 1];
      if (!terminalToken || !frame.logical_segments[0].text.endsWith(terminalToken)) {
        throw new Error("fixture_does_not_end_with_artifact_token");
      }
      const rewritten = frame.logical_segments[0].text
        .replace("Opening scene.", "The revised scene opens with immediate tension.")
        + "\nA final consequence lands after the status block.";
      const assembled = assembleSceneRewriteFrame(frame, {
        segments: { [SCENE_REWRITE_SEGMENT_ID]: rewritten },
      });
      const verification = verifyOutput(
        physical,
        assembled.physicalFinalSegments,
        assembled.output,
        source,
        settings
      );
      if (assembled.frame_error || !assembled.changed || !verification.pass
          || assembled.output.indexOf('<STATUS>[ PT: 1500 ]</STATUS>') < 0
          || !assembled.output.endsWith('A final consequence lands after the status block.')) {
        throw new Error(`terminal_scene_expansion_failed:${assembled.frame_error}:${verification.errors.join(",")}`);
      }
      return "terminal artifact and Composer ending both retained";
    });

    await check("batch1_legacy_physical_segments_fold_into_scene_1", async () => {
      const source = 'Opening <b>pressure</b> and a closing choice.';
      const physical = buildSegmentMap(source, defaultSettings());
      const frame = buildSceneRewriteFrame(physical, source);
      const legacySegments = {};
      physical.filter((segment) => segment.type === "mutable" && mutableCoreText(segment).trim())
        .forEach((segment, index) => {
          legacySegments[segment.id] = index === 0
            ? mutableFullText(segment).replace("Opening", "A sharper opening")
            : mutableFullText(segment);
        });
      const detailed = validateCandidateSchemaDetailed({
        schema: "scene_rewrite_candidates.v1",
        role_id: "style_reader",
        candidates: [{
          segments: legacySegments,
          evidence_refs: [],
          retained_beats: [],
          proposed_additions: [],
          addressed_issues: ["rhythm"],
          confidence: 0.8,
          change_summary: "legacy physical map folded into one scene",
        }],
      }, "style_reader", [SCENE_REWRITE_SEGMENT_ID], frame.logical_segments);
      if (!detailed.value || detailed.value.candidates.length !== 1
          || !detailed.diagnostics.some((item) => item.code === "folded_physical_segments_to_scene")) {
        throw new Error(`legacy_scene_fold_failed:${JSON.stringify(detailed.diagnostics)}`);
      }
      return "legacy mutable_n candidate recovered as scene_1";
    });

    await check("batch1_physical_map_embedded_tokens_recovered_without_llm_retry", async () => {
      const source = 'Opening <b>pressure</b> and a closing choice.';
      const physical = buildSegmentMap(source, defaultSettings());
      const frame = buildSceneRewriteFrame(physical, source);
      const rawSegments = {};
      physical.filter((segment) => segment.type === "mutable" && mutableCoreText(segment).trim())
        .forEach((segment, index) => {
          const rewritten = index === 0
            ? mutableFullText(segment).replace("Opening", "A decisive opening")
            : mutableFullText(segment);
          rawSegments[segment.id] = `${frame.tokens.join("")}${rewritten}`;
        });
      const detailed = validateCandidateSchemaDetailed({
        schema: "scene_rewrite_candidates.v1",
        role_id: "style_reader",
        candidates: [{
          segments: rawSegments,
          evidence_refs: [],
          retained_beats: [],
          proposed_additions: [],
          addressed_issues: ["rhythm"],
          confidence: 0.8,
          change_summary: "legacy physical response with duplicated placeholders",
        }],
      }, "style_reader", [SCENE_REWRITE_SEGMENT_ID], frame.logical_segments);
      const diagnostic = detailed.diagnostics.find((item) => item.code === "folded_physical_segments_to_scene");
      if (!detailed.value || detailed.value.candidates.length !== 1
          || !diagnostic || diagnostic.detail.indexOf("stripped") < 0) {
        throw new Error(`embedded_token_fold_failed:${JSON.stringify(detailed.diagnostics)}`);
      }
      const foreign = foldPhysicalSegmentMapIntoScene(frame.logical_segments[0], {
        [physical.find((segment) => segment.type === "mutable").id]: "[[ACR_EXACT_foreign_9999]]replacement",
      });
      if (foreign) throw new Error("foreign_placeholder_was_stripped_instead_of_rejected");
      return "physical-key response normalized and duplicated known tokens stripped locally";
    });

    await check("batch1_verifier_warns_on_new_structural_container", async () => {
      const settings = defaultSettings();
      const source = "Original narrative paragraph.";
      const segments = buildSegmentMap(source, settings);
      const finalText = '<div class="new-wrapper">Rewritten narrative paragraph.</div>';
      const finalSegments = [{
        id: segments[0].id,
        type: "mutable",
        final_text: finalText,
      }];
      const verification = verifyOutput(segments, finalSegments, finalText, source, settings);
      if (!verification.pass
          || verification.warnings.indexOf("host_artifact_inventory_changed_after_restore") < 0) {
        throw new Error(`new_container_warning_missing:${JSON.stringify(verification)}`);
      }
      return "new balanced HTML wrapper remains returnable and is traced as a warning";
    });

    await check("batch1_damaged_composer_envelope_recovers_complete_scene", async () => {
      const source = 'Before <img src="x"> after.';
      const frame = buildSceneRewriteFrame(buildSegmentMap(source, defaultSettings()), source);
      const scene = frame.logical_segments[0].text
        .replace("Before", "A stronger opening arrives before")
        .replace("after.", "the consequence lands after.");
      const encoded = JSON.stringify(scene);
      const damaged = `{"segments":{"${SCENE_REWRITE_SEGMENT_ID}":${encoded}`;
      const recovered = recoverComposerSceneFromDamagedEnvelope(
        damaged,
        [SCENE_REWRITE_SEGMENT_ID],
        frame.logical_segments
      );
      if (!recovered || recovered.segments[SCENE_REWRITE_SEGMENT_ID] !== scene) {
        throw new Error("complete_scene_not_recovered_from_damaged_envelope");
      }
      return "complete scene recovered from truncated wrapper after the scene string";
    });

    await check("batch1_scene_rewriters_expand_output_budget", async () => {
      const role = DEFAULT_ROLES.find((item) => item.role_id === "style_reader");
      const profile = defaultRoleProfile(role.role_id);
      profile.max_output_tokens = 2048;
      const expanded = expandRewriteOutputBudget(role, profile, [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        text: "가".repeat(7200),
      }]);
      if (expanded.max_output_tokens <= 4096 || expanded.max_output_tokens > 32000) {
        throw new Error(`rewrite_budget_not_expanded:${expanded.max_output_tokens}`);
      }
      return `long scene output budget expanded to ${expanded.max_output_tokens}`;
    });

    await check("batch1_scene_frame_repairs_placeholder_damage", async () => {
      const source = "Before <b>bold</b> and <i>italic</i> after.";
      const frame = buildSceneRewriteFrame(buildSegmentMap(source, defaultSettings()), source);
      const segment = frame.logical_segments[0];
      const validText = segment.text.replace("Before", "Rewritten before");
      const validCandidate = {
        schema: "scene_rewrite_candidates.v1",
        role_id: "style_reader",
        candidates: [{
          segments: { [SCENE_REWRITE_SEGMENT_ID]: validText },
          evidence_refs: [],
          retained_beats: [],
          proposed_additions: [],
          addressed_issues: ["rhythm"],
          confidence: 0.8,
          change_summary: "scene rewrite",
        }],
      };
      if (!validateCandidateSchema(
        validCandidate,
        "style_reader",
        [SCENE_REWRITE_SEGMENT_ID],
        frame.logical_segments
      )) throw new Error("valid_placeholder_candidate_rejected");
      if (!validateComposerSchema(
        { segments: { [SCENE_REWRITE_SEGMENT_ID]: validText } },
        [SCENE_REWRITE_SEGMENT_ID],
        frame.logical_segments
      )) throw new Error("valid_placeholder_composer_rejected");
      if (!validateComposerPlainScene(
        validText,
        [SCENE_REWRITE_SEGMENT_ID],
        frame.logical_segments
      )) throw new Error("valid_plain_scene_composer_rejected");
      if (validateComposerPlainScene(
        `{"segments":{"${SCENE_REWRITE_SEGMENT_ID}":`,
        [SCENE_REWRITE_SEGMENT_ID],
        frame.logical_segments
      )) throw new Error("malformed_json_accepted_as_plain_scene");

      const tokens = frame.tokens;
      const damaged = [
        validText.replace(tokens[0], ""),
        validText.replace(tokens[0], `${tokens[0]}${tokens[0]}`),
        validText.replace(tokens[0], "[[ACR_EXACT_foreign_9999]]"),
        validText.replace(tokens[0], "__SWAP__")
          .replace(tokens[1], tokens[0])
          .replace("__SWAP__", tokens[1]),
      ];
      damaged.forEach((text, index) => {
        const candidate = deepClone(validCandidate);
        candidate.candidates[0].segments[SCENE_REWRITE_SEGMENT_ID] = text;
        if (!validateCandidateSchema(
          candidate,
          "style_reader",
          [SCENE_REWRITE_SEGMENT_ID],
          frame.logical_segments
        )) throw new Error(`repairable_candidate_rejected:${index}`);
        const composer = validateComposerSchema(
          { segments: { [SCENE_REWRITE_SEGMENT_ID]: text } },
          [SCENE_REWRITE_SEGMENT_ID],
          frame.logical_segments
        );
        if (!composer) throw new Error(`repairable_composer_rejected:${index}`);
        const assembled = assembleSceneRewriteFrame(frame, composer);
        if (assembled.frame_error
            || assembled.output.indexOf("<b>") < 0
            || assembled.output.indexOf("</b>") < 0
            || assembled.output.indexOf("<i>") < 0
            || assembled.output.indexOf("</i>") < 0
            || extractSceneFrameTokens(assembled.output).length > 0) {
          throw new Error(`placeholder_damage_not_repaired:${index}:${assembled.frame_error}`);
        }
      });
      return "missing duplicate foreign and reordered hints are mechanically repaired";
    });

    await check("batch1_scene_frame_restores_exact_structures_and_whitespace", async () => {
      const source = "  Before.\n<img src=\"x\">\n```status\nHP: 10\n```\n<div class=\"box\">value</div>\n`code`\nAfter.  ";
      const physical = buildSegmentMap(source, defaultSettings());
      const frame = buildSceneRewriteFrame(physical, source);
      const rewritten = frame.logical_segments[0].text
        .replace("Before.", "The scene now opens with pressure.")
        .replace("After.", "The scene closes on a sharper choice.");
      const assembled = assembleSceneRewriteFrame(frame, {
        segments: { [SCENE_REWRITE_SEGMENT_ID]: rewritten },
      });
      const verification = verifyOutput(
        physical,
        assembled.physicalFinalSegments,
        assembled.output,
        source
      );
      const exactStructures = [
        '<img src="x">',
        '```status\nHP: 10\n```',
        '<div class="box">',
        '</div>',
        '`code`',
      ];
      if (!verification.pass || assembled.frame_error
          || !exactStructures.every((item) => assembled.output.indexOf(item) >= 0)
          || assembled.output.indexOf('\n<img src="x">\n') < 0
          || !assembled.output.startsWith("  ")
          || !assembled.output.endsWith("  ")) {
        throw new Error(`scene_frame_restore_failed:${assembled.frame_error}:${verification.errors.join(",")}`);
      }
      return `restored ${frame.tokens.length} exact structures`;
    });

    await check("batch1_composer_accepts_complete_plain_scene", async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      const source = 'Before <img src="x"> after.';
      const frame = buildSceneRewriteFrame(buildSegmentMap(source, defaultSettings()), source);
      const plainScene = frame.logical_segments[0].text
        .replace("Before", "A stronger opening arrives before")
        .replace("after.", "the consequence lands after.");
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === COMPOSER_ROLE_ID);
      const profile = settings.role_profiles[COMPOSER_ROLE_ID];
      profile.endpoint = "https://test.example.com/v1/chat/completions";
      profile.model = "plain-scene-composer";
      globalThis.Risuai = null;
      globalThis.fetch = async () => ({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ choices: [{ message: { content: plainScene } }] }),
      });
      try {
        const trace = newTrace("test", "main");
        const result = await callRole(
          role,
          profile,
          frame.logical_segments,
          "",
          frame.logical_segments,
          null,
          trace,
          { candidateBundles: {}, draft_ledger: {} },
          Date.now(),
          { allowRetry: false, allowFallback: false, completionWait: true }
        );
        const roleTrace = trace.roles[trace.roles.length - 1];
        const attempt = roleTrace && roleTrace.attempts && roleTrace.attempts[0];
        if (!result
            || result.segments[SCENE_REWRITE_SEGMENT_ID] !== plainScene
            || !attempt
            || attempt.structured_transport !== "composer_plain_scene") {
          throw new Error(`plain_scene_transport_failed:${JSON.stringify({ result, attempt })}`);
        }
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
      return "plain whole-scene Composer output accepted without JSON wrapper";
    });

    await check("batch1_scene_composer_requests_plain_body_with_dynamic_budget", async () => {
      const originalFetch = globalThis.fetch;
      const originalRisu = globalThis.Risuai;
      const source = 'Opening <img src="x"> consequence.';
      const frame = buildSceneRewriteFrame(buildSegmentMap(source, defaultSettings()), source);
      const scene = frame.logical_segments[0].text
        .replace("Opening", "The rebuilt opening")
        .replace("consequence.", "lands on a concrete consequence.");
      const settings = defaultSettings();
      const role = DEFAULT_ROLES.find((item) => item.role_id === COMPOSER_ROLE_ID);
      const profile = settings.role_profiles[COMPOSER_ROLE_ID];
      profile.endpoint = "https://test.example.com/v1/chat/completions";
      profile.model = "plain-scene-composer";
      profile.force_json_response = true;
      let requestBody = null;
      globalThis.Risuai = null;
      globalThis.fetch = async (_url, init) => {
        requestBody = JSON.parse(init.body);
        return {
          ok: true,
          status: 200,
          text: async () => JSON.stringify({ choices: [{ message: { content: scene } }] }),
        };
      };
      try {
        const trace = newTrace("test", "main");
        const result = await runComposer(
          role,
          profile,
          frame.logical_segments,
          {
            ranked: {
              [SCENE_REWRITE_SEGMENT_ID]: [{
                candidate_id: "candidate_test",
                role_id: "style_reader",
                supporting_roles: ["style_reader"],
                rewrite: scene,
                judge_verdict: "accept",
                evidence_refs: [],
                retained_beats: [],
                proposed_additions: [],
                issues: ["rhythm"],
              }],
            },
            semantic_judgment: {},
            fusion_plan: {},
            draft_ledger: {},
          },
          "",
          null,
          trace,
          frame.logical_segments,
          true
        );
        const systemMessage = requestBody && requestBody.messages
          ? safeString(requestBody.messages[0] && requestBody.messages[0].content)
          : "";
        if (!result || result.segments[SCENE_REWRITE_SEGMENT_ID] !== scene
            || !requestBody || requestBody.response_format
            || systemMessage.indexOf("SCENE BODY OUTPUT OVERRIDE") < 0
            || trace.composer.requested_output_tokens < trace.composer.estimated_output_tokens) {
          throw new Error(`plain_composer_request_contract_failed:${JSON.stringify({
            result,
            response_format: requestBody && requestBody.response_format,
            requested: trace.composer.requested_output_tokens,
            estimated: trace.composer.estimated_output_tokens,
          })}`);
        }
      } finally {
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
      return "scene Composer uses plain body transport and length-based output budget";
    });

    await check("batch1_comparison_shows_applied_and_no_candidate_states", async () => {
      const original = "Original full scene.";
      const changed = "Rewritten full scene with stronger dramatic movement.";
      const trace = newTrace("test", "main");
      trace.summary.final_state = "enhanced";
      trace.final.reason = "composer_rewrite_semantic_proof_passed";
      trace.applied_evidence = [{ segment_id: SCENE_REWRITE_SEGMENT_ID, role_contributions: [] }];
      updateLatestAppliedComparison(trace, [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        original_text: original,
        final_text: changed,
        source: "composer",
        operation: "replace",
        applied_role_id: COMPOSER_ROLE_ID,
        material_change: true,
      }], original, changed);
      if (!latestAppliedComparison.applied
          || latestAppliedComparison.changes.length !== 1
          || renderLatestComparison().indexOf(changed) < 0) {
        throw new Error("applied_whole_scene_comparison_missing");
      }

      trace.summary.final_state = "rejected";
      trace.final.reason = "composer_failed";
      updateLatestAppliedComparison(trace, null, original, original);
      const failedHtml = renderLatestComparison();
      if (latestAppliedComparison.applied
          || latestAppliedComparison.changes.length
          || failedHtml.indexOf("composer_failed") < 0
          || failedHtml.indexOf(original) < 0
          || failedHtml.indexOf("변경 미적용") < 0) {
        throw new Error("original_return_comparison_missing");
      }
      return "whole-scene applied and no-generated-candidate comparison rendered";
    });

    await check("batch1_inline_backticks_are_mutable_not_protected", async () => {
      const source = "`prompt residue`\n<img src=\"x\">\n```status\nHP: 10\n```\nNarrative body.";
      const physical = buildSegmentMap(source, defaultSettings());
      if (physical.some((segment) => segment.kind === "inline_code")) {
        throw new Error("inline_backtick_still_protected");
      }
      const frame = buildSceneRewriteFrame(physical, source);
      const logical = frame.logical_segments[0];
      if (logical.text.indexOf("`prompt residue`") < 0) {
        throw new Error("inline_backtick_not_exposed_to_rewriter");
      }
      const rewritten = logical.text
        .replace("`prompt residue`", "The scene opens on a concrete action.")
        .replace("Narrative body.", "The consequence follows in continuous prose.");
      const assembled = assembleSceneRewriteFrame(frame, {
        segments: { [SCENE_REWRITE_SEGMENT_ID]: rewritten },
      });
      if (assembled.frame_error
          || assembled.output.indexOf("prompt residue") >= 0
          || assembled.output.indexOf('<img src="x">') < 0
          || assembled.output.indexOf("```status\nHP: 10\n```") < 0) {
        throw new Error(`inline_rewrite_or_structure_preservation_failed:${assembled.frame_error}`);
      }
      return "backtick prompt residue mutable while image and status fence stay exact";
    });

    await check("batch1_ledger_separates_facts_from_constraints", async () => {
      const contract = {
        contract_id: "contract_test",
        contract_digest: "digest_test",
        immutable_constraints: [{
          text: "Do not reveal the hidden identity.",
          evidence_refs: ["payload_user_input"],
          evidence_quote: "hidden identity",
        }],
        turn_objectives: [{
          text: "Do not reveal the hidden identity.",
          evidence_refs: ["payload_user_input"],
          evidence_quote: "hidden identity",
        }],
        character_visible_facts: [{
          text: "Mira carries the sealed letter.",
          evidence_refs: ["character"],
          evidence_quote: "sealed letter",
        }],
      };
      const ledger = buildDraftLedger(
        "Mira crossed the courtyard.",
        [{ id: "mutable_1", type: "mutable", text: "Mira crossed the courtyard." }],
        contract
      );
      if (ledger.established_facts.length !== 1
          || ledger.established_facts[0].kind !== "character_visible_fact"
          || !ledger.hard_constraints.some((item) => item.kind === "immutable_constraint")
          || ledger.hard_constraints.some((item) => item.kind === "character_visible_fact")) {
        throw new Error(`ledger_fact_constraint_overlap:${JSON.stringify({
          facts: ledger.established_facts,
          hard: ledger.hard_constraints,
        })}`);
      }
      return "immutable directives no longer masquerade as prose facts";
    });

    await check("batch1_negative_fact_empty_quote_is_valid", async () => {
      const ledger = {
        established_facts: [{ ledger_id: "fact_absence", kind: "scene_state", text: "The hidden observer remains absent." }],
        scene_beats: [],
        hard_constraints: [],
      };
      const finalText = "The corridor remained quiet as Mira closed the door.";
      const proof = validateSemanticProof({
        schema: "semantic_proof.v1",
        declared_verdict: "pass",
        fact_checks: [{ ledger_id: "fact_absence", status: "preserved", detail: "No observer appears.", evidence_quote: "" }],
        beat_checks: [],
        constraint_checks: [],
        quality_gain_checks: [],
        residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
          check_id: checkId,
          status: "clean",
          detail: "",
          evidence_quote: "",
          segment_ids: [],
        })),
        hard_violations: [],
        unsupported_additions: [],
        output_contract: {
          language_ok: true,
          turn_boundary_ok: true,
          user_agency_ok: true,
          meta_free: true,
          format_ok: true,
        },
        repair_instructions: [],
      }, ledger, [{ id: "scene_1", type: "mutable", text: finalText }], [{
        id: "scene_1",
        type: "mutable",
        final_text: finalText,
      }], { required_contributions: [] }, []);
      if (!proof || proof.verdict !== "pass") throw new Error("negative_fact_empty_quote_rejected");
      return "negative and implicit preservation no longer requires invented prose quote";
    });

    await check("batch1_failed_proof_synthesizes_one_scene_repair", async () => {
      const proof = {
        verdict: "fail",
        reason_codes: ["hard_violation:meta_artifact", "quality_gain_missing:gain_1"],
        fact_checks: [],
        beat_checks: [],
        constraint_checks: [],
        quality_gain_checks: [{ contribution_id: "gain_1", status: "missing", detail: "The stronger transition is absent." }],
        residual_quality_checks: [{ check_id: "scene_coherence_and_ending", status: "issue", detail: "The ending is explanatory." }],
        hard_violations: [{ type: "meta_artifact", detail: "Prompt residue remains.", segment_ids: [SCENE_REWRITE_SEGMENT_ID] }],
        unsupported_additions: [],
        output_contract: {
          language_ok: true,
          turn_boundary_ok: true,
          user_agency_ok: true,
          meta_free: false,
          format_ok: true,
        },
      };
      const repaired = synthesizeWholeSceneRepairProof(proof, [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        text: "scene",
      }]);
      if (!repaired || repaired.verdict !== "repair" || !repaired.repair_synthesized
          || repaired.repair_instructions.length !== 1
          || repaired.repair_instructions[0].evidence_refs.indexOf("output_contract:meta_free") < 0) {
        throw new Error(`whole_scene_repair_not_synthesized:${JSON.stringify(repaired)}`);
      }
      return "Prover fail becomes one complete-scene repair pass";
    });

    await check("batch1_explicit_prover_fail_cannot_discard_changed_scene", async () => {
      const finalText = "The characters met in the room while the secret remained unspoken.";
      const proofLedger = {
        schema: "draft_ledger.v1",
        established_facts: [{
          ledger_id: "fact_1", text: "The characters met.", source_ref: "draft_zero",
        }],
        scene_beats: [{
          ledger_id: "beat_1", text: "The scene remains in the room.", source_ref: "draft_zero",
        }],
        hard_constraints: [{
          ledger_id: "constraint_1", text: "Do not reveal the secret.", source_ref: "turn_contract",
        }],
      };
      const proof = validateSemanticProof(
        {
          schema: "semantic_proof.v1",
          declared_verdict: "fail",
          fact_checks: [{
            ledger_id: "fact_1", status: "preserved", detail: "Fact remains.", evidence_quote: "met",
          }],
          beat_checks: [{
            ledger_id: "beat_1", status: "preserved", detail: "Beat remains.", evidence_quote: "room",
          }],
          constraint_checks: [{
            ledger_id: "constraint_1", status: "satisfied", detail: "Secret remains hidden.", evidence_quote: "",
          }],
          quality_gain_checks: [],
          residual_quality_checks: PROOF_RESIDUAL_QUALITY_IDS.map((checkId) => ({
            check_id: checkId, status: "clean", detail: "Clean.", evidence_quote: "", segment_ids: [],
          })),
          hard_violations: [],
          unsupported_additions: [],
          output_contract: {
            language_ok: true, turn_boundary_ok: true, user_agency_ok: true, meta_free: true, format_ok: true,
          },
          repair_instructions: [],
        },
        proofLedger,
        [{ id: SCENE_REWRITE_SEGMENT_ID, type: "mutable", text: finalText }],
        [{ id: SCENE_REWRITE_SEGMENT_ID, type: "mutable", final_text: finalText }],
        { required_contributions: [] },
        []
      );
      if (!proof || proof.verdict !== "fail" || proof.declared_verdict !== "fail") {
        throw new Error(`explicit_fail_not_validated:${JSON.stringify(proof)}`);
      }
      const result = synthesizeWholeSceneRepairProof(proof, [{
        id: SCENE_REWRITE_SEGMENT_ID,
        type: "mutable",
        text: "scene",
      }]);
      const assessment = assessSemanticProofReturn(result);
      const returnedProof = degradedSemanticProof(result, assessment.blocking_reasons);
      if (result !== proof || result.verdict !== "fail" || result.repair_synthesized
          || assessment.blocking_reasons.indexOf("semantic_prover_declared_fail") < 0
          || returnedProof.returnable_degraded !== true
          || returnedProof.reason_codes.indexOf("semantic_prover_declared_fail") < 0) {
        throw new Error(`explicit_fail_return_contract_wrong:${JSON.stringify({ result, assessment, returnedProof })}`);
      }
      return "explicit Prover failure is retained as repair evidence but cannot erase changed prose";
    });

    await check("batch1_post_repair_semantic_debt_is_traced_without_output_veto", async () => {
      const qualityOnly = {
        declared_verdict: "repair",
        verdict: "fail",
        fact_checks: [], beat_checks: [], constraint_checks: [],
        quality_gain_checks: [{ contribution_id: "gain_1", status: "missing" }],
        residual_quality_checks: [{ check_id: "rhythm", status: "issue" }],
        hard_violations: [], unsupported_additions: [],
        output_contract: {
          language_ok: true, turn_boundary_ok: true, user_agency_ok: true, meta_free: true, format_ok: true,
        },
      };
      const qualityAssessment = assessSemanticProofReturn(qualityOnly);
      const blockingAssessment = assessSemanticProofReturn(Object.assign({}, qualityOnly, {
        hard_violations: [{ type: "secret_leak" }],
      }));
      const missingFactAssessment = assessSemanticProofReturn(Object.assign({}, qualityOnly, {
        fact_checks: [{ ledger_id: "fact_1", status: "missing" }],
      }));
      const degradedBlocking = degradedSemanticProof(
        Object.assign({}, qualityOnly, { hard_violations: [{ type: "secret_leak" }] }),
        blockingAssessment.blocking_reasons
      );
      const degradedMissingFact = degradedSemanticProof(
        Object.assign({}, qualityOnly, { fact_checks: [{ ledger_id: "fact_1", status: "missing" }] }),
        missingFactAssessment.blocking_reasons
      );
      if (!qualityAssessment.returnable || !qualityAssessment.degraded
          || blockingAssessment.returnable
          || blockingAssessment.blocking_reasons.indexOf("hard_violation:secret_leak") < 0
          || missingFactAssessment.returnable
          || missingFactAssessment.blocking_reasons.indexOf("fact_missing:fact_1") < 0
          || !degradedBlocking.returnable_degraded
          || !degradedMissingFact.returnable_degraded) {
        throw new Error(`post_repair_return_policy_wrong:${JSON.stringify({
          qualityAssessment, blockingAssessment, missingFactAssessment,
          degradedBlocking, degradedMissingFact,
        })}`);
      }
      return "semantic debt drives repair and Trace but exhausted repair cannot erase changed prose";
    });

    await check("batch1_quality_debt_rewrite_is_classified_and_traced_as_enhanced", async () => {
      const assembled = {
        materialComposerApplied: 1,
        composerApplied: 1,
        metaOnlyChanged: 0,
        finalSegments: [{
          id: SCENE_REWRITE_SEGMENT_ID,
          type: "mutable",
          original_text: "The room was quiet.",
          final_text: "Rain tightened against the windows as Mira closed the ledger.",
          operation: "replace",
        }],
      };
      const proof = {
        verdict: "fail",
        returnable_degraded: true,
        quality_gain_checks: [{ contribution_id: "gain_1", status: "missing" }],
        residual_quality_checks: [{ check_id: "rhythm", status: "issue" }],
      };
      const classification = classifyAppliedOutput(assembled, proof);
      const trace = newTrace("test", "test");
      setFinalTraceState(
        trace,
        classification.enhanced,
        classification.state,
        classification.reason,
        classification
      );
      if (!classification.enhanced
          || classification.state !== "enhanced_degraded"
          || classification.semantic_verified !== "degraded"
          || trace.final.enhanced !== true
          || trace.final.semantic_verified !== "degraded"
          || trace.summary.final_state !== "enhanced_degraded") {
        throw new Error(`quality_debt_return_metadata_broken:${JSON.stringify({ classification, final: trace.final, summary: trace.summary })}`);
      }
      return "material Composer rewrite remains the returned output when only quality debt remains";
    });

    await check("batch1_completion_wait_has_hard_watchdog", async () => {
      const deadline = createCompletionDeadline(Date.now(), 20);
      await new Promise((resolve) => setTimeout(resolve, 50));
      const stopped = deadline.check();
      const reason = safeString(deadline.signal.reason);
      deadline.cancel();
      if (!stopped || !deadline.signal.aborted || deadline.remaining() !== 0
          || reason !== "completion_wait_watchdog") {
        throw new Error(`completion_wait_watchdog_failed:${JSON.stringify({
          stopped, aborted: deadline.signal.aborted, remaining: deadline.remaining(), reason,
        })}`);
      }
      return "completion wait aborts stalled provider work at its hard watchdog";
    });

    await check("batch1_completion_watchdog_aborts_stalled_response_body", async () => {
      const originalRisu = globalThis.Risuai;
      const originalFetch = globalThis.fetch;
      const deadline = createCompletionDeadline(Date.now(), 30);
      const profile = defaultRoleProfile("character_reader");
      profile.endpoint = "https://test.example.com/v1/chat/completions";
      profile.model = "stalled-body-model";
      globalThis.Risuai = null;
      globalThis.fetch = async () => ({
        ok: true,
        status: 200,
        text: () => new Promise(() => {}),
      });
      const startedAt = Date.now();
      let errorCode = "";
      try {
        await callProvider(
          profile,
          { system: "system", user: "user" },
          deadline.signal,
          { completion_wait: true }
        );
      } catch (err) {
        errorCode = safeString(err && err.message);
      } finally {
        deadline.cancel();
        globalThis.fetch = originalFetch;
        globalThis.Risuai = originalRisu;
      }
      const elapsed = Date.now() - startedAt;
      if (errorCode !== "deadline_aborted" || elapsed > 1000) {
        throw new Error(`stalled_body_not_aborted:${errorCode}:${elapsed}`);
      }
      return "completion watchdog interrupts response.text() after headers arrive";
    });

    await check("batch1_bounded_stage_budget_preserves_downstream_time", async () => {
      const deadline = createDeadline(240, Date.now());
      const trace = newTrace("test", "model");
      const scope = createPipelineStageScope(deadline, "semantic_judge", trace);
      await new Promise((resolve) => setTimeout(resolve, scope.budget_ms + 30));
      const parentExpired = deadline.check();
      const stageAborted = scope.signal.aborted;
      const remaining = deadline.remaining();
      scope.cancel();
      deadline.cancel();
      if (!stageAborted || parentExpired || remaining <= 0
          || !trace.scheduler.stage_budgets.length
          || trace.scheduler.stage_budgets[0].stage !== "semantic_judge") {
        throw new Error(`stage_budget_did_not_leave_tail:${JSON.stringify({
          stageAborted, parentExpired, remaining, budgets: trace.scheduler.stage_budgets,
        })}`);
      }
      const completionDeadline = createCompletionDeadline(Date.now(), 240);
      const completionTrace = newTrace("test", "model");
      const completionScope = createPipelineStageScope(
        completionDeadline, "semantic_judge", completionTrace
      );
      await new Promise((resolve) => setTimeout(resolve, completionScope.budget_ms + 30));
      const completionParentExpired = completionDeadline.check();
      const completionStageAborted = completionScope.signal.aborted;
      const completionRemaining = completionDeadline.remaining();
      completionScope.cancel();
      completionDeadline.cancel();
      if (!completionStageAborted || completionParentExpired || completionRemaining <= 0
          || !completionTrace.scheduler.stage_budgets[0].completion_wait) {
        throw new Error(`completion_stage_budget_did_not_leave_tail:${JSON.stringify({
          completionStageAborted,
          completionParentExpired,
          completionRemaining,
          budgets: completionTrace.scheduler.stage_budgets,
        })}`);
      }
      const adaptiveDeadline = createDeadline(500, Date.now());
      const adaptiveTrace = newTrace("test", "model");
      const tailReserveMs = 200;
      const revisionScope = createPipelineStageScope(
        adaptiveDeadline, "adaptive_revision", adaptiveTrace, 120
      );
      await new Promise((resolve) => setTimeout(resolve, revisionScope.budget_ms + 20));
      revisionScope.cancel();
      const rejudgeCap = Math.max(1, adaptiveDeadline.remaining() - tailReserveMs - 40);
      const rejudgeScope = createPipelineStageScope(
        adaptiveDeadline, "adaptive_rejudge", adaptiveTrace, rejudgeCap
      );
      await new Promise((resolve) => setTimeout(resolve, rejudgeScope.budget_ms + 20));
      rejudgeScope.cancel();
      const adaptiveRemaining = adaptiveDeadline.remaining();
      adaptiveDeadline.cancel();
      const adaptiveBudgets = adaptiveTrace.scheduler.stage_budgets;
      if (adaptiveRemaining < tailReserveMs
          || adaptiveBudgets.length !== 2
          || adaptiveBudgets[0].explicit_cap_ms !== 120
          || adaptiveBudgets[1].explicit_cap_ms !== Math.floor(rejudgeCap)) {
        throw new Error(`adaptive_stage_caps_consumed_tail:${JSON.stringify({
          adaptiveRemaining, tailReserveMs, adaptiveBudgets,
        })}`);
      }
      return "bounded, completion-wait, and adaptive scopes preserve downstream time";
    });

    await check("batch1_provider_error_trace_redacts_echoed_credentials", async () => {
      const secret = "provider-echo-secret-123456789";
      const trace = newTrace("test", "test");
      traceRole(trace, {
        role_id: "character_reader",
        provider: "custom",
        model: "error-model",
        status: "failed",
        error: `HTTP 401: {"x-api-key":"${secret}","authorization":"Bearer ${secret}"}`,
        attempts: [{ error: `token=${secret}` }],
      });
      traceError(trace, `provider_error api_key=${secret}`);
      const serialized = JSON.stringify(redactTraceValue(trace));
      if (serialized.indexOf(secret) >= 0 || serialized.indexOf("[REDACTED]") < 0) {
        throw new Error(`provider_secret_persisted_in_trace:${serialized}`);
      }
      return "provider-reflected credentials are removed before Trace persistence";
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
          await RR.registerSetting("AC Recomposer Agent", openSettingsUI, "🔧", "html", "risu-recomposer-settings");
        }
      } catch (err) {
        error("registerSetting error:", err);
      }
      try {
        if (RR && typeof RR.registerButton === "function") {
          await RR.registerButton({
            name: "AC Recomposer Agent Settings",
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
      verifyOutput,
      selectRoles,
      detectSceneSignals,
      validateCandidateSchema,
      validateSemanticJudgment,
      validateSemanticProof,
      validateComposerSchema,
      buildDraftLedger,
      buildFusionPlan,
      buildComposerCandidatePool,
      summarizeDraftLedger,
      classifyAppliedOutput,
      admitSceneCandidate,
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
      runSemanticProver,
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
      TAG_ISSUE_MAP,
      normalizeIssues,
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
