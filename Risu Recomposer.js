//@name Risu Recomposer
//@display-name Risu Recomposer
//@author recomposer
//@api 3.0
//@version 0.1.3


/*
 * Risu Recomposer — MDASH/Fusion/Fugu output recomposition plugin.
 *
 * Product: RisuAI main LLM response (draft_zero) is received, split into
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

  const R = (typeof Risuai !== "undefined")
    ? Risuai
    : (typeof risuai !== "undefined" ? risuai : null);

  const PLUGIN_ID = "risu_recomposer";
  const VERSION = "0.1.3";
  const LOG_PREFIX = "[Recomposer]";
  const SETTINGS_KEY = `${PLUGIN_ID}_settings_v1`;
  const TRACE_KEY = `${PLUGIN_ID}_trace_v1`;
  const TRACE_LIMIT = 30;
  const DEFAULT_DEADLINE_MS = 120000;
  const COMPOSER_RESERVE_MS = 30000;
  const CONTEXT_TIMEOUT_MS = 12000;
  const MAX_REPAIR_RETRY = 1;
  const MAX_FALLBACK = 1;

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

  const DEFAULT_ROLES = [
    {
      role_id: "secret_pov_guard",
      label: "Secret / POV Constraint Ledger",
      purpose: "Detect leaks of secrets, private thoughts, hidden narrator knowledge, system/meta text, or information the current POV character should not know. Rewrite the segment to enforce knowledge boundaries.",
      priority: 10,
      default_prompt: "You are a Secret/POV Constraint specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment so that no character reveals knowledge they should not have.\n\nRules:\n- The POV character must not narrate others' private thoughts.\n- Secrets, hidden identities, and private memories must not leak.\n- System/meta/prompt text must not appear in prose.\n- Preserve the scene's action and emotional beat.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"secret_pov_guard\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"voice\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "character_reader",
      label: "Character / Voice",
      purpose: "Inspect and rewrite for character voice, emotional posture, knowledge boundary, and persona consistency.",
      priority: 9,
      default_prompt: "You are a Character/Voice specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment so the character's voice, personality, and emotional state are authentic and consistent.\n\nRules:\n- Match the character's established speech patterns and personality.\n- Do not invent new facts or backstory.\n- Do not decide the user's next action.\n- Preserve the scene's action and intent.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"character_reader\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"voice\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "plot_continuity_reader",
      label: "Plot / Continuity",
      purpose: "Inspect and rewrite for causality, scene flow, action order, and unresolved promise continuity.",
      priority: 8,
      default_prompt: "You are a Plot/Continuity specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment so causality, scene flow, and continuity are correct.\n\nRules:\n- Actions must follow logically from prior context.\n- Do not contradict established facts.\n- Preserve the user's intended direction.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"plot_continuity_reader\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"continuity\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "world_reader",
      label: "World / Scene Logic",
      purpose: "Inspect and rewrite for local world rules, social laws, magic/technology constraints, geography, and faction logic.",
      priority: 7,
      default_prompt: "You are a World/Scene Logic specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment so world rules, physical constraints, and scene logic are consistent.\n\nRules:\n- Respect established lore and world rules.\n- Physical actions must be plausible within the setting.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"world_reader\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"world\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "style_reader",
      label: "Style / Rhythm",
      purpose: "Inspect and rewrite for prose rhythm, repetition, awkward phrasing, tone, and transitions.",
      priority: 6,
      default_prompt: "You are a Style/Rhythm specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment for better prose rhythm, reduced repetition, smoother transitions, and consistent tone.\n\nRules:\n- Improve flow without changing meaning or character voice.\n- Remove awkward phrasing and redundant repetition.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"style_reader\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"style\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "agency_meta_guard",
      label: "Agency / Meta Artifact",
      purpose: "Detect and remove user agency takeover, meta text, model self-commentary, translator-like phrasing, and mechanical artifacts.",
      priority: 7,
      default_prompt: "You are an Agency/Meta Artifact specialist for roleplay.\n\nYou receive one mutable prose segment from a larger RP response.\nYour job: rewrite this segment to remove meta text, model self-commentary, translator-like phrasing, list-like summaries, and any takeover of user agency.\n\nRules:\n- Remove 'As an AI', 'Language model', summary lists, and meta commentary.\n- Do not decide or force the user's feelings, thoughts, or actions.\n- Return a FULL replacement for this segment, not advice.\n\nReturn JSON:\n{\"role\":\"agency_meta_guard\",\"candidates\":[{\"segment_id\":\"SEG_ID\",\"rewrite\":\"full rewritten text\",\"confidence\":0.0,\"tags\":[\"agency\"]}]}",
      cost_tier: "standard",
    },
    {
      role_id: "whole_scene_composer",
      label: "Whole-Scene Composer",
      purpose: "Read the full scene with all segment candidates and integrate character, secret, POV, continuity, style, and emotion into one coherent final response.",
      priority: 5,
      is_composer: true,
      default_prompt: "You are the Whole-Scene Fusion Composer for a roleplay response.\n\nYou receive the full scene with protected/inspect segments shown as [PRESERVED] and each mutable segment's best rewrite candidates.\nYour job: produce the FINAL version of every mutable segment, integrating character voice, secrets, POV, continuity, style, and emotional arc simultaneously.\n\nRules:\n- Do NOT simply concatenate candidates.\n- Weave the best elements into coherent prose.\n- Preserve the scene's action, intent, and emotional beat.\n- Each segment must be a complete, self-contained prose block.\n- Do NOT output protected or inspect-only segments — only mutable ones.\n\nReturn JSON:\n{\"segments\":{\"SEG_ID\":\"final rewritten text\",...}}",
      cost_tier: "premium",
    },
  ];

  const COMPOSER_ROLE_ID = "whole_scene_composer";

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
      label: "Quality (all roles + composer)",
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
    try {
      if (R && typeof R.getStorage === "function") {
        const v = await R.getStorage(key);
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
    try {
      if (R && typeof R.setStorage === "function") {
        await R.setStorage(key, value);
        return;
      }
    } catch (_) {}
    try {
      if (typeof localStorage !== "undefined") {
        localStorage.setItem(key, value);
      }
    } catch (_) {}
  }

  /* ── Settings ──────────────────────────────────────────── */

  function defaultRoleProfile(roleId) {
    const role = DEFAULT_ROLES.find((r) => r.role_id === roleId);
    return {
      role_id: roleId,
      enabled: true,
      provider: "openai_compatible",
      endpoint: "",
      api_key_ref: "",
      model: "",
      temperature: role && role.is_composer ? 0.4 : 0.3,
      max_output_tokens: role && role.is_composer ? 4096 : 2048,
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
    try {
      await storageSet(SETTINGS_KEY, JSON.stringify(settings));
    } catch (_) {}
  }

  function mergeSettings(base, input) {
    const merged = deepClone(base);
    if (!input || typeof input !== "object") return merged;
    merged.version = VERSION;
    merged.enabled = input.enabled !== false;
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
      });
    }
    return merged;
  }

  /* ── API Key Resolution ────────────────────────────────── */

  async function resolveApiKey(ref) {
    const r = safeString(ref).trim();
    if (!r) return "";
    if (r.indexOf("arg:") === 0 && R && typeof R.getArgument === "function") {
      return safeString(await R.getArgument(r.slice(4)));
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
      stage: safeString(stage),
      request_type: safeString(type),
      timestamp: Date.now(),
      roles: [],
      segments: { protected: 0, inspect_only: 0, mutable: 0 },
      candidates: { total: 0, by_segment: {} },
      composer: { used: false, status: "", elapsed_ms: 0 },
      final: { enhanced: false, reason: "" },
      errors: [],
      timeline: [],
    };
  }

  function traceRole(trace, entry) {
    if (!trace.roles) trace.roles = [];
    trace.roles.push({
      role_id: entry.role_id,
      provider: entry.provider,
      model: entry.model,
      status: entry.status,
      started_at: entry.started_at,
      ended_at: entry.ended_at,
      elapsed_ms: entry.elapsed_ms,
      retry: entry.retry || 0,
      fallback: entry.fallback || false,
      candidate_count: entry.candidate_count || 0,
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
    { kind: "html_tag", regex: /<\/?[a-z][^>]*>/gi, priority: 80 },
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

  async function guardedRisuApiCall(name, args) {
    if (!R || typeof R[name] !== "function") {
      return { ok: false, value: null, source: name, error: "api_unavailable" };
    }
    return Promise.race([
      (async () => {
        try {
          const value = await R[name].apply(R, Array.isArray(args) ? args : []);
          return { ok: true, value, source: name, error: "" };
        } catch (err) {
          return { ok: false, value: null, source: name, error: err && err.message ? err.message : String(err) };
        }
      })(),
      new Promise((resolve) => setTimeout(() => resolve({ ok: false, value: null, source: name, error: "timeout" }), CONTEXT_TIMEOUT_MS)),
    ]);
  }

  async function loadCharacter() {
    const direct = await guardedRisuApiCall("getCharacter");
    if (direct.ok && direct.value && typeof direct.value === "object") {
      return { value: direct.value, source: "getCharacter" };
    }
    const idx = await guardedRisuApiCall("getCurrentCharacterIndex");
    if (idx.ok && Number.isFinite(Number(idx.value))) {
      const byIdx = await guardedRisuApiCall("getCharacterFromIndex", [parseInt(idx.value, 10)]);
      if (byIdx.ok && byIdx.value) return { value: byIdx.value, source: "getCharacterFromIndex" };
    }
    return { value: null, source: "getCharacter", error: "character_unavailable" };
  }

  async function loadDatabase() {
    const keys = ["personas", "selectedPersona", "modules", "enabledModules", "globalChatVariables"];
    const keyed = await guardedRisuApiCall("getDatabase", [keys]);
    if (keyed.ok) return { value: keyed.value, source: "getDatabase" };
    const full = await guardedRisuApiCall("getDatabase");
    if (full.ok) return { value: full.value, source: "getDatabase" };
    return { value: null, source: "getDatabase", error: "database_unavailable" };
  }

  async function loadCurrentChat(character) {
    const charIdx = await guardedRisuApiCall("getCurrentCharacterIndex");
    const chatIdx = await guardedRisuApiCall("getCurrentChatIndex");
    if (charIdx.ok && chatIdx.ok && Number.isFinite(Number(charIdx.value)) && Number.isFinite(Number(chatIdx.value))) {
      const chat = await guardedRisuApiCall("getChatFromIndex", [parseInt(charIdx.value, 10), parseInt(chatIdx.value, 10)]);
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
    else if (typeof selected === "string" && personas.length) {
      persona = personas.find((p) => p && (p.id === selected || p.name === selected));
    }
    if (!persona && personas.length) persona = personas[0];
    if (!persona) return "";
    const parts = [];
    if (persona.name) parts.push(`Persona: ${persona.name}`);
    if (persona.text || persona.description) parts.push(`Detail: ${truncate(persona.text || persona.description, 500)}`);
    return parts.join("\n");
  }

  function extractChatSummary(chat) {
    if (!chat || typeof chat !== "object") return "";
    const parts = [];
    const messages = arrayFromCollection(chat.messages || chat.chats);
    const recent = messages.slice(-8);
    recent.forEach((msg) => {
      if (msg && msg.data) parts.push(`${msg.role || "unknown"}: ${truncate(msg.data, 300)}`);
    });
    return parts.join("\n");
  }

  function extractMemorySummary(chat) {
    if (!chat || typeof chat !== "object") return "";
    const parts = [];
    const fields = [
      ["summary", "Summary"],
      ["note", "Note"],
      ["supaMemoryData", "SupaMemory"],
      ["hypaMemoryData", "HypaMemory"],
      ["lastMemory", "LastMemory"],
    ];
    fields.forEach(([key, label]) => {
      if (chat[key]) parts.push(`${label}: ${truncate(typeof chat[key] === "string" ? chat[key] : JSON.stringify(chat[key]), 400)}`);
    });
    return parts.join("\n");
  }

  function extractLorebookSummary(character, db) {
    const lorebooks = arrayFromCollection(
      (character && character.character_book) || (character && character.data && character.data.character_book)
    );
    const candidates = [];
    const active = [];
    lorebooks.forEach((book) => {
      if (book && Array.isArray(book.entries)) {
        book.entries.forEach((entry) => {
          if (!entry) return;
          const keys = Array.isArray(entry.keys) ? entry.keys.map((k) => safeString(k)).filter(Boolean) : [safeString(entry.keys)].filter(Boolean);
          const content = truncate(entry.content, 200);
          const candidateEntry = { keys: keys.join(", "), content };
          candidates.push(candidateEntry);
          if (entry.constant) {
            active.push(candidateEntry);
            return;
          }
          if (keys.length && entry.keys) {
            let matched = false;
            keys.forEach((key) => {
              try {
                const re = new RegExp(key, "i");
                if (re.test(content) || re.test(safeString(character && character.name))) {
                  matched = true;
                }
              } catch (_) {}
            });
            if (matched) active.push(candidateEntry);
          }
        });
      }
    });
    if (!candidates.length) return { candidates: "", active: "" };
    return {
      candidates: candidates.slice(0, 20).map((e) => `[${e.keys}] ${e.content}`).join("\n"),
      active: active.slice(0, 10).map((e) => `[${e.keys}] ${e.content}`).join("\n"),
    };
  }

  function extractPayloadSystem(payload) {
    if (!payload || typeof payload !== "object") return "";
    const messages = arrayFromCollection(payload.messages);
    const systemParts = [];
    messages.forEach((msg) => {
      if (msg && msg.role === "system") {
        systemParts.push(truncate(safeString(msg.content), 800));
      }
    });
    return systemParts.join("\n");
  }

  function extractRecentChat(payload) {
    if (!payload || typeof payload !== "object") return "";
    const messages = arrayFromCollection(payload.messages);
    const recent = messages.slice(-10).filter((m) => m && m.role !== "system");
    return recent.map((m) => `${m.role}: ${truncate(safeString(m.content), 300)}`).join("\n");
  }

  function extractLatestUserInput(payload) {
    if (!payload || typeof payload !== "object") return "";
    const messages = arrayFromCollection(payload.messages);
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i] && messages[i].role === "user") {
        return truncate(safeString(messages[i].content), 500);
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

  async function collectContext(payload, settings, trace) {
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
      bounded_context_block: "",
      sources: {},
    };

    try {
      ctx.system_context = filterExcludedContext(extractPayloadSystem(payload));
      ctx.recent_chat = filterExcludedContext(extractRecentChat(payload));
      ctx.latest_user_input = filterExcludedContext(extractLatestUserInput(payload));
      ctx.sources.payload = { available: !!ctx.system_context || !!ctx.recent_chat };
    } catch (err) {
      ctx.sources.payload = { available: false, error: safeString(err && err.message) };
    }

    try {
      const charResult = await loadCharacter();
      if (charResult.value) {
        ctx.character = filterExcludedContext(extractCharacterSummary(charResult.value));
        ctx.sources.character = { available: true, source: charResult.source };

        try {
          const chatResult = await loadCurrentChat(charResult.value);
          if (chatResult.value) {
            ctx.current_chat = filterExcludedContext(extractChatSummary(chatResult.value));
            ctx.memory = filterExcludedContext(extractMemorySummary(chatResult.value));
            ctx.sources.current_chat = { available: true, source: chatResult.source };
            ctx.sources.memory = { available: !!ctx.memory };
          } else {
            ctx.sources.current_chat = { available: false, error: chatResult.error || "" };
          }
        } catch (err) {
          ctx.sources.current_chat = { available: false, error: safeString(err && err.message) };
        }

        try {
          const loreResult = extractLorebookSummary(charResult.value, null);
          ctx.lorebook = filterExcludedContext(loreResult.candidates || "");
          ctx.lorebook_active = filterExcludedContext(loreResult.active || "");
          ctx.sources.lorebook = { available: !!ctx.lorebook, candidate_count: candidates.length || 0, active_count: active.length || 0 };
        } catch (_) {}
      } else {
        ctx.sources.character = { available: false, error: charResult.error || "" };
      }
    } catch (err) {
      ctx.sources.character = { available: false, error: safeString(err && err.message) };
    }

    try {
      const dbResult = await loadDatabase();
      if (dbResult.value) {
        ctx.persona = filterExcludedContext(extractPersonaSummary(dbResult.value));
        ctx.sources.persona = { available: !!ctx.persona, source: dbResult.source };
      } else {
        ctx.sources.persona = { available: false };
      }
    } catch (_) {
      ctx.sources.persona = { available: false };
    }

    const parts = [];
    if (ctx.system_context) parts.push(`[System]\n${ctx.system_context}`);
    if (ctx.character) parts.push(`[Character]\n${ctx.character}`);
    if (ctx.persona) parts.push(`[Persona]\n${ctx.persona}`);
    if (ctx.lorebook) parts.push(`[Lorebook Candidates]\n${ctx.lorebook}`);
    if (ctx.lorebook_active) parts.push(`[Active Lore]\n${ctx.lorebook_active}`);
    if (ctx.current_chat) parts.push(`[Recent Chat]\n${ctx.current_chat}`);
    if (ctx.memory) parts.push(`[Memory]\n${ctx.memory}`);
    if (ctx.latest_user_input) parts.push(`[User Input]\n${ctx.latest_user_input}`);
    ctx.bounded_context_block = truncate(parts.join("\n\n"), limit);

    return ctx;
  }

  /* ── Provider Request ──────────────────────────────────── */

  function openAiChatUrl(endpoint) {
    const base = safeString(endpoint || "https://api.openai.com/v1").replace(/\/+$/, "");
    if (/\/chat\/completions$/i.test(base)) return base;
    return `${base}/chat/completions`;
  }

  function parseExtraHeaders(text) {
    const out = {};
    safeString(text).split(/\n/).forEach((line) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      const idx = trimmed.indexOf(":");
      if (idx > 0) {
        const key = trimmed.slice(0, idx).trim();
        const val = trimmed.slice(idx + 1).trim();
        if (key) out[key] = val;
      }
    });
    return out;
  }

  function parseExtraBody(text) {
    if (!text || !text.trim()) return {};
    try { return JSON.parse(text); } catch (_) { return {}; }
  }

  function applyReasoningAdapter(provider, body, profile) {
    const preset = safeString(profile.reasoning_preset, "auto").toLowerCase();
    const effort = safeString(profile.reasoning_effort, "auto").toLowerCase();
    const budget = clampNumber(profile.reasoning_budget_tokens, 0, 131072, 0);
    if (effort === "none" || effort === "disable") return;
    if (provider === "openai_compatible" || provider === "ollama_compatible") {
      if (effort !== "auto" && effort !== "enable") {
        body.reasoning_effort = effort;
      }
      if (budget > 0) body.max_completion_tokens = budget;
    } else if (provider === "anthropic") {
      if (budget > 0) {
        body.thinking = { type: "enabled", budget_tokens: budget };
      }
    } else if (provider === "gemini" || provider === "vertex") {
      const gc = body.generationConfig || (body.generationConfig = {});
      if (effort !== "auto") {
        gc.thinkingConfig = gc.thinkingConfig || {};
        gc.thinkingConfig.thinkingLevel = effort;
      }
      if (budget > 0) {
        gc.thinkingConfig = gc.thinkingConfig || {};
        gc.thinkingConfig.thinkingBudget = budget;
      }
    }
  }

  function applyVertexFlex(profile, headers) {
    const mode = safeString(profile.vertex_flex_mode, "off").toLowerCase();
    if (mode === "off") return;
    if (mode === "flex_only") {
      headers["X-Vertex-Flex"] = "true";
    } else if (mode === "provisioned_then_flex") {
      headers["X-Vertex-Flex"] = "auto";
    }
  }

  async function fetchWithAbort(url, options, timeoutMs, abortSignal) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    if (abortSignal) {
      abortSignal.addEventListener("abort", () => controller.abort(), { once: true });
    }
    try {
      const fetchFn = (typeof fetch === "function") ? fetch : null;
      if (!fetchFn) throw new Error("no_fetch_available");
      const fetchPromise = fetchFn(url, Object.assign({}, options, { signal: controller.signal }));
      const abortPromise = new Promise((_, reject) => {
        controller.signal.addEventListener("abort", () => reject(new Error("request_aborted")), { once: true });
      });
      const response = await Promise.race([fetchPromise, abortPromise]);
      return response;
    } finally {
      clearTimeout(timer);
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

  async function callProvider(profile, prompts, abortSignal) {
    const provider = sanitizeEnum(profile.provider, PROVIDERS, "openai_compatible");
    const key = await resolveApiKey(profile.api_key_ref);
    const timeoutMs = clampNumber(profile.timeout_ms, 5000, 300000, 45000);
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
      return callOllama(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    if (provider === "custom") {
      return callCustom(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
    }
    return callOpenAiCompatible(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal);
  }

  async function callOpenAiCompatible(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const url = openAiChatUrl(profile.endpoint);
    const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
    if (key) headers.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
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
    if (profile.force_json_response) {
      body.response_format = { type: "json_object" };
    }
    applyReasoningAdapter("openai_compatible", body, profile);
    body = Object.assign(body, extraBody);
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    return { content: extractOpenAiText(JSON.parse(raw)), raw, elapsed_ms: 0 };
  }

  function isOllamaCloudEndpoint(endpoint) {
    const base = safeString(endpoint).toLowerCase();
    return base.indexOf("ollama.com") >= 0 || base.indexOf("/v1/chat/completions") >= 0;
  }

  async function callOllama(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const base = safeString(profile.endpoint || "http://localhost:11434").replace(/\/+$/, "");
    if (isOllamaCloudEndpoint(base)) {
      const url = /\/chat\/completions$/i.test(base) ? base : `${base}/chat/completions`;
      const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
      if (key) headers.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
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
      if (profile.force_json_response) {
        body.response_format = { type: "json_object" };
      }
      applyReasoningAdapter("ollama_compatible", body, profile);
      body = Object.assign(body, extraBody);
      const response = await fetchWithAbort(url, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      }, timeoutMs, abortSignal);
      const raw = await readResponseText(response);
      if (!response || !response.ok) {
        throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
      }
      return { content: extractOpenAiText(JSON.parse(raw)), raw, elapsed_ms: 0 };
    }
    const url = `${base}/api/chat`;
    const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
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
    applyReasoningAdapter("ollama_compatible", body, profile);
    body = Object.assign(body, extraBody);
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
    return { content: safeString(data.message && data.message.content), raw, elapsed_ms: 0 };
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
    applyReasoningAdapter("anthropic", body, profile);
    body = Object.assign(body, extraBody);
    const headers = Object.assign({
      "Content-Type": "application/json",
      "x-api-key": key,
      "anthropic-version": "2023-06-01",
    }, extraHeaders);
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    return { content: extractAnthropicText(JSON.parse(raw)), raw, elapsed_ms: 0 };
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
    applyReasoningAdapter("gemini", body, profile);
    if (profile.force_json_response) {
      body.generationConfig.responseMimeType = "application/json";
    }
    body = Object.assign(body, extraBody);
    const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
    if (key) headers["x-goog-api-key"] = key;
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    return { content: extractGeminiText(JSON.parse(raw)), raw, elapsed_ms: 0 };
  }

  async function callVertex(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const url = vertexGenerateContentUrl(profile.endpoint, profile.model);
    const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
    if (key) headers.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
    applyVertexFlex(profile, headers);
    let body = {
      systemInstruction: { parts: [{ text: prompts.system }] },
      contents: [{ role: "user", parts: [{ text: prompts.user }] }],
      generationConfig: {
        temperature: profile.temperature,
        maxOutputTokens: profile.max_output_tokens,
      },
    };
    applyReasoningAdapter("vertex", body, profile);
    if (profile.force_json_response) {
      body.generationConfig.responseMimeType = "application/json";
    }
    body = Object.assign(body, extraBody);
    const response = await fetchWithAbort(url, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }, timeoutMs, abortSignal);
    const raw = await readResponseText(response);
    if (!response || !response.ok) {
      throw new Error(`HTTP ${response ? response.status : ""}: ${raw.slice(0, 300)}`);
    }
    return { content: extractGeminiText(JSON.parse(raw)), raw, elapsed_ms: 0 };
  }

  async function callCustom(profile, key, prompts, timeoutMs, extraHeaders, extraBody, abortSignal) {
    const url = safeString(profile.endpoint);
    if (!url) throw new Error("missing_custom_endpoint");
    const headers = Object.assign({ "Content-Type": "application/json" }, extraHeaders);
    if (key) headers.Authorization = `Bearer ${key.replace(/^Bearer\s+/i, "")}`;
    let body = {
      model: profile.model,
      system: prompts.system,
      user: prompts.user,
      temperature: profile.temperature,
      max_tokens: profile.max_output_tokens,
    };
    body = Object.assign(body, extraBody);
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
    try {
      const data = JSON.parse(raw);
      content = extractOpenAiText(data) || extractGeminiText(data) || extractAnthropicText(data) || safeString(data.content || data.text || data.output);
    } catch (_) {
      content = raw;
    }
    return { content, raw, elapsed_ms: 0 };
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

  function tryParseJson(text) {
    const raw = stripJsonFence(text);
    if (!raw) return null;
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

  /* ── Role Call (single path for all roles) ─────────────── */

  function buildRolePrompt(role, profile, mutableSegs, contextBlock, allSegments, directorInfo) {
    const systemPrompt = safeString(profile.system_prompt || role.default_prompt);
    const contextSection = contextBlock ? `\n\n--- Runtime Context (read-only) ---\n${contextBlock}\n--- End Context ---\n` : "";
    let userPrompt;
    if (role.is_composer) {
      const mutableList = mutableSegs.map((s) => {
        const candidates = (directorInfo && directorInfo.candidateBundles && directorInfo.candidateBundles[s.id]) || [];
        const candidateSection = candidates.length
          ? candidates.map((c, i) => `  Candidate ${i + 1} (${c.role}, conf ${c.confidence}):\n    ${c.rewrite}`).join("\n")
          : "  (no candidates — use original)";
        return `Segment ${s.id}:\nOriginal:\n${s.text}\nCandidates:\n${candidateSection}`;
      }).join("\n\n---\n\n");
      const preservedList = allSegments.filter((s) => s.type !== "mutable").map((s) =>
        `${s.id} [PRESERVED — do not output]: ${preview(s.text, 200)}`
      ).join("\n");
      const directorSection = directorInfo
        ? `\n--- Fusion Director ---\nConsensus: ${JSON.stringify(directorInfo.consensus || {})}\nConflict: ${JSON.stringify(directorInfo.conflict || {})}\nGaps: ${JSON.stringify(directorInfo.gap || [])}\n--- End Director ---\n`
        : "";
      userPrompt = `You are composing the FINAL version of this roleplay response.\n\nFull scene segments:\n\n${mutableList}\n\nPreserved segments (do NOT include in your output):\n${preservedList}\n${directorSection}${contextSection}\n\nProduce the final rewritten text for each mutable segment. Return JSON:\n{"segments":{"SEG_ID":"final text", ...}}`;
    } else {
      const segList = mutableSegs.map((s) =>
        `Segment ${s.id}:\n${s.text}`
      ).join("\n\n---\n\n");
      const segIds = mutableSegs.map((s) => s.id);
      userPrompt = `Rewrite ALL of the following mutable segments from a roleplay response.\n\nYou are seeing the full ordered segment list for context. Protected and inspect-only segments are shown as [PRESERVED] — do not rewrite those.\n\nFull ordered segments:\n${allSegments.map((s) => s.type === "mutable" ? `${s.id} [MUTABLE — rewrite this]:\n${s.text}` : `${s.id} [PRESERVED — do not rewrite]: ${preview(s.text, 200)}`).join("\n\n---\n\n")}\n\nMutable segment IDs to rewrite: ${segIds.join(", ")}\n${contextSection}\n\nReturn JSON with your rewrites for ALL listed mutable segments. Each rewrite must be a COMPLETE replacement for that entire segment, not partial advice.\nReturn:\n{"role":"${role.role_id}","candidates":[{"segment_id":"SEG_ID","rewrite":"full rewritten text","confidence":0.0,"tags":["voice"]}, ...]}\n\nOnly include candidates for segment IDs from the list above.`;
    }
    return { system: systemPrompt, user: userPrompt };
  }

  function validateCandidateSchema(parsed, expectedRoleId, allowedSegmentIds) {
    if (!parsed || typeof parsed !== "object") return null;
    const candidates = Array.isArray(parsed.candidates) ? parsed.candidates : [];
    const allowed = Array.isArray(allowedSegmentIds) ? new Set(allowedSegmentIds) : null;
    const valid = [];
    candidates.forEach((c) => {
      if (!c || typeof c !== "object") return;
      const segId = safeString(c.segment_id).trim();
      const rewrite = safeString(c.rewrite);
      if (!segId || !rewrite) return;
      if (allowed && !allowed.has(segId)) return;
      valid.push({
        segment_id: segId,
        rewrite,
        confidence: clampNumber(c.confidence, 0, 1, 0.5),
        tags: Array.isArray(c.tags) ? c.tags.map((t) => safeString(t)).filter(Boolean) : [],
      });
    });
    if (!valid.length) return null;
    return { role: safeString(parsed.role || expectedRoleId), candidates: valid };
  }

  function validateComposerSchema(parsed, allowedSegmentIds) {
    if (!parsed || typeof parsed !== "object") return null;
    const segments = asObject(parsed.segments);
    const allowed = Array.isArray(allowedSegmentIds) ? new Set(allowedSegmentIds) : null;
    const result = {};
    let count = 0;
    Object.keys(segments).forEach((segId) => {
      if (allowed && !allowed.has(segId)) return;
      const text = safeString(segments[segId]);
      if (text) {
        result[segId] = text;
        count++;
      }
    });
    if (!count) return null;
    return { segments: result };
  }

  async function callRole(role, profile, mutableSegs, contextBlock, allSegments, abortSignal, trace, directorInfo) {
    const allowedSegIds = mutableSegs ? mutableSegs.map((s) => s.id) : (role.is_composer ? allSegments.filter((s) => s.type === "mutable").map((s) => s.id) : []);
    const prompts = buildRolePrompt(role, profile, role.is_composer ? mutableSegs : mutableSegs, contextBlock, allSegments, directorInfo);
    const startedAt = Date.now();
    let attempt = 0;
    let lastError = "";
    let usedFallback = false;
    let activeProfile = profile;

    while (attempt <= MAX_REPAIR_RETRY) {
      try {
        const result = await callProvider(activeProfile, prompts, abortSignal);
        const content = safeString(result.content);
        if (!content) throw new Error("empty_response");
        const parsed = tryParseJson(content);
        if (!parsed) throw new Error("json_parse_failed");
        const validated = role.is_composer
          ? validateComposerSchema(parsed, allowedSegIds)
          : validateCandidateSchema(parsed, role.role_id, allowedSegIds);
        if (!validated) throw new Error("schema_validation_failed");

        const elapsed = Date.now() - startedAt;
        traceRole(trace, {
          role_id: role.role_id,
          provider: activeProfile.provider,
          model: activeProfile.model,
          status: "fulfilled",
          started_at: startedAt,
          ended_at: Date.now(),
          elapsed_ms: elapsed,
          retry: attempt,
          fallback: usedFallback,
          candidate_count: role.is_composer
            ? Object.keys(validated.segments).length
            : validated.candidates.length,
        });
        return validated;
      } catch (err) {
        lastError = safeString(err && err.message);
        if (abortSignal && abortSignal.aborted) break;
        attempt++;
        if (attempt <= MAX_REPAIR_RETRY) {
          continue;
        }
        if (!usedFallback && profile.fallback_provider && profile.fallback_model) {
          usedFallback = true;
          activeProfile = Object.assign({}, profile, {
            provider: profile.fallback_provider,
            endpoint: profile.fallback_endpoint || profile.endpoint,
            model: profile.fallback_model,
            api_key_ref: profile.fallback_api_key_ref || profile.api_key_ref,
          });
          attempt = 0;
          continue;
        }
        break;
      }
    }

    traceRole(trace, {
      role_id: role.role_id,
      provider: activeProfile.provider,
      model: activeProfile.model,
      status: "failed",
      started_at: startedAt,
      ended_at: Date.now(),
      elapsed_ms: Date.now() - startedAt,
      retry: attempt,
      fallback: usedFallback,
      error: lastError,
    });
    return null;
  }

  /* ── Router (Fugu role selection) ──────────────────────── */

  function detectSceneSignals(segments, context) {
    const signals = [];
    const mutableText = mutableSegments(segments).map((s) => s.text).join("\n");
    const combined = [mutableText, context && context.bounded_context_block].filter(Boolean).join("\n");
    const dialogueCount = (mutableText.match(/["']/g) || []).length;
    const agencyCount = (combined.match(/\b(?:you|your|decide|feel|felt|think|thought|choose|force|must)\b/gim) || []).length;
    const povCount = (combined.match(/\b(?:secret|private|hidden|pov|narrator|system|meta|memory|identity|alias)\b/gim) || []).length;
    const mechanicalCount = (mutableText.match(/as an ai|language model|firstly|secondly|in summary|sorry|redo|rewrite/gim) || []).length;
    const worldCount = (combined.match(/\b(?:lore|world|kingdom|magic|mana|law|rule|forbidden|faction|war)\b/gim) || []).length;
    const styleCount = (mutableText.match(/\b(?:again|still|suddenly|silence|wind|eyes|gaze|voice|breath|cold|quiet)\b/gim) || []).length;
    const mutableChars = mutableText.trim().length;

    if (dialogueCount >= 4) signals.push({ id: "dialogue_heavy", severity: "medium" });
    if (agencyCount >= 2) signals.push({ id: "agency_risk", severity: "high" });
    if (povCount >= 2) signals.push({ id: "pov_secret_risk", severity: "high" });
    if (mechanicalCount >= 1) signals.push({ id: "mechanical_artifact", severity: "high" });
    if (worldCount >= 3) signals.push({ id: "world_lore_risk", severity: "medium" });
    if (styleCount >= 8) signals.push({ id: "style_pressure", severity: "medium" });
    if (mutableChars >= 3000) signals.push({ id: "long_scene", severity: "medium" });
    return signals;
  }

  const SIGNAL_ROLE_MAP = Object.freeze({
    dialogue_heavy: ["character_reader"],
    agency_risk: ["character_reader", "agency_meta_guard"],
    pov_secret_risk: ["secret_pov_guard"],
    mechanical_artifact: ["agency_meta_guard", "style_reader"],
    world_lore_risk: ["world_reader"],
    style_pressure: ["style_reader"],
    long_scene: ["plot_continuity_reader", "style_reader"],
  });

  function selectRoles(roles, signals, preset, settings) {
    const presetDef = PRESETS.find((p) => p.id === preset) || PRESETS[1];
    const enabledRoleIds = presetDef.roles;
    const signalIds = signals.map((s) => s.id);
    const signalRoleIds = new Set();
    signalIds.forEach((sigId) => {
      const mapped = SIGNAL_ROLE_MAP[sigId];
      if (mapped) mapped.forEach((rid) => signalRoleIds.add(rid));
    });
    const selected = [];
    roles.forEach((role) => {
      if (role.is_composer) {
        if (enabledRoleIds.indexOf(role.role_id) >= 0) selected.push(role);
        return;
      }
      if (enabledRoleIds.indexOf(role.role_id) < 0) return;
      const profile = settings.role_profiles[role.role_id];
      if (!profile || !profile.enabled) return;
      if (signalRoleIds.size === 0) {
        selected.push(role);
        return;
      }
      if (signalRoleIds.has(role.role_id)) {
        selected.push(role);
      }
    });
    return selected.sort((a, b) => (b.priority || 0) - (a.priority || 0));
  }

  /* ── Fusion Director (single scoring function) ─────────── */

  function fusionDirector(candidatesBySegment, roles) {
    const segmentIds = Object.keys(candidatesBySegment);
    const ranked = {};
    const consensus = {};
    const conflict = {};
    const gap = [];

    segmentIds.forEach((segId) => {
      const candidates = candidatesBySegment[segId] || [];
      if (!candidates.length) {
        gap.push(segId);
        ranked[segId] = [];
        return;
      }
      const rolePriority = {};
      roles.forEach((r, i) => { rolePriority[r.role_id] = (r.priority || 0) + (roles.length - i); });

      const scored = candidates.map((c) => {
        const roleWeight = rolePriority[c.role_id] || 0;
        const confidence = clampNumber(c.confidence, 0, 1, 0.5);
        const score = confidence * 100 + roleWeight;
        return Object.assign({}, c, { score });
      }).sort((a, b) => b.score - a.score);

      ranked[segId] = scored;

      const roleIds = uniqueList(candidates.map((c) => c.role_id));
      if (roleIds.length >= 2) {
        consensus[segId] = roleIds.length >= 2 ? "multi_role" : "single_role";
        const topRewrites = scored.slice(0, 3).map((c) => c.rewrite);
        const avgLen = topRewrites.reduce((s, t) => s + t.length, 0) / topRewrites.length;
        const variance = topRewrites.reduce((s, t) => s + Math.abs(t.length - avgLen), 0) / topRewrites.length;
        if (variance > avgLen * 0.3) {
          conflict[segId] = "high_length_variance";
        }
      }
    });

    return { ranked, consensus, conflict, gap };
  }

  /* ── Composer ──────────────────────────────────────────── */

  async function runComposer(composerRole, profile, segments, directorResult, contextBlock, abortSignal, trace, mutableSegs) {
    const candidateBundles = {};
    Object.keys(directorResult.ranked).forEach((segId) => {
      const ranked = directorResult.ranked[segId];
      candidateBundles[segId] = ranked.slice(0, 3).map((c) => ({
        role: c.role_id,
        confidence: c.confidence,
        rewrite: c.rewrite,
      }));
    });
    const directorInfo = {
      candidateBundles,
      consensus: directorResult.consensus,
      conflict: directorResult.conflict,
      gap: directorResult.gap,
    };
    const composerStarted = Date.now();
    const result = await callRole(composerRole, profile, mutableSegs, contextBlock, segments, abortSignal, trace, directorInfo);
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

    segments.forEach((seg) => {
      if (seg.type === "mutable") {
        let finalText = null;
        let source = "original";
        if (composerSegments[seg.id]) {
          finalText = composerSegments[seg.id];
          source = "composer";
          changed = true;
        } else if (ranked[seg.id] && ranked[seg.id].length) {
          finalText = ranked[seg.id][0].rewrite;
          source = "top_candidate";
          changed = true;
        }
        if (finalText !== null) {
          const leadingWs = safeString(seg.leading_ws);
          const trailingWs = safeString(seg.trailing_ws);
          finalText = leadingWs + safeString(finalText) + trailingWs;
        }
        finalSegments.push({
          id: seg.id,
          type: "mutable",
          original_text: seg.text,
          final_text: finalText || seg.text,
          source: finalText !== null ? source : "original",
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
    return { output, finalSegments, changed };
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
    const mutableSegs = finalSegments.filter((f) => f.type === "mutable");
    const seenTexts = new Set();
    mutableSegs.forEach((seg) => {
      const key = safeString(seg.final_text).trim().slice(0, 100);
      if (key && seenTexts.has(key)) {
        errors.push(`duplicate_segment:${seg.id}`);
      }
      seenTexts.add(key);
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

  function createDeadline(deadlineMs) {
    const start = Date.now();
    const deadline = start + deadlineMs;
    const controller = new AbortController();
    let aborted = false;
    const timer = setTimeout(() => {
      aborted = true;
      try { controller.abort(); } catch (_) {}
    }, deadlineMs);
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

  async function scheduleRoles(roles, profiles, segments, contextBlock, allSegments, deadline, trace, maxParallel) {
    const mutableSegs = mutableSegments(segments);
    const specialistRoles = roles.filter((r) => !r.is_composer);
    const composerRole = roles.find((r) => r.is_composer);
    const candidatesBySegment = {};
    const mutableSegIds = mutableSegs.map((s) => s.id);

    const globalSem = createSemaphore(clampNumber(maxParallel, 1, 20, 5));
    const providerSems = {};
    function getProviderSem(provider) {
      const p = sanitizeEnum(provider, PROVIDERS, "openai_compatible");
      if (!providerSems[p]) {
        providerSems[p] = createSemaphore(PROVIDER_CONCURRENCY[p] || 1);
      }
      return providerSems[p];
    }
    const allSems = () => [globalSem].concat(Object.values(providerSems));

    const specialistTasks = specialistRoles.filter((role) => {
      const profile = profiles[role.role_id];
      return profile && profile.enabled;
    });

    let activeCallCount = 0;

    const specialistPromises = specialistTasks.map((role) => {
      const profile = profiles[role.role_id];
      return globalSem.acquire(() => {
        if (deadline.check()) return null;
        return getProviderSem(profile.provider).acquire(() => {
          if (deadline.check()) return null;
          activeCallCount++;
          return callRole(role, profile, mutableSegs, contextBlock, allSegments, deadline.signal, trace, null)
            .then((result) => {
              if (result && result.candidates) {
                result.candidates.forEach((c) => {
                  if (!candidatesBySegment[c.segment_id]) candidatesBySegment[c.segment_id] = [];
                  candidatesBySegment[c.segment_id].push({
                    role_id: result.role,
                    rewrite: c.rewrite,
                    confidence: c.confidence,
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
    const deadlinePromise = new Promise((resolve) => {
      const checkTimer = setInterval(() => {
        if (deadline.check()) {
          clearInterval(checkTimer);
          allSems().forEach((sem) => { try { sem.cancel(); } catch (_) {} });
          resolve("deadline");
        }
      }, 200);
      allSettled.then(() => { clearInterval(checkTimer); resolve("settled"); });
    });

    const raceResult = await Promise.race([allSettled, deadlinePromise]);
    if (raceResult === "deadline") {
      allSems().forEach((sem) => { try { sem.cancel(); } catch (_) {} });
    }
    specialistPromises.forEach((p) => { try { p.catch(() => null); } catch (_) {} });
    if (raceResult === "deadline") {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }

    trace.active_calls_after_specialists = activeCallCount;

    const directorResult = fusionDirector(candidatesBySegment, specialistRoles);
    trace.candidates.total = Object.values(candidatesBySegment).reduce((s, arr) => s + arr.length, 0);
    Object.keys(candidatesBySegment).forEach((segId) => {
      trace.candidates.by_segment[segId] = candidatesBySegment[segId].length;
    });

    let composerResult = null;
    if (composerRole && !deadline.check() && deadline.reserveComposer() > 0) {
      const composerProfile = profiles[composerRole.role_id];
      if (composerProfile && composerProfile.enabled) {
        composerResult = await runComposer(composerRole, composerProfile, segments, directorResult, contextBlock, deadline.signal, trace, mutableSegs);
      }
    } else if (composerRole) {
      trace.composer.status = "skipped_deadline";
    }

    trace.active_calls_final = activeCallCount;
    return { directorResult, composerResult };
  }

  /* ── Main Pipeline ─────────────────────────────────────── */

  function isAuxiliaryRequest(type) {
    const t = safeString(type).toLowerCase();
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
      || t.indexOf("submodel") >= 0;
  }

  let lastRequestContext = null;

  async function onBeforeRequest(payload, type) {
    try {
      if (isAuxiliaryRequest(type)) return payload;
      lastRequestContext = payload;
    } catch (err) {
      warn("beforeRequest error:", err);
    }
    return payload;
  }

  async function onAfterRequest(content, type) {
    const trace = newTrace("afterRequest", type);
    traceTimeline(trace, "start");

    try {
      if (isAuxiliaryRequest(type)) {
        trace.final.enhanced = false;
        trace.final.reason = "auxiliary_request_bypass";
        if (settings_trace_enabled()) await saveTrace(trace);
        return content;
      }

      const settings = await loadSettings();
      if (!settings.enabled) {
        trace.final.enhanced = false;
        trace.final.reason = "plugin_disabled";
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

      const originalText = safeString(content);
      if (!originalText.trim()) {
        trace.final.enhanced = false;
        trace.final.reason = "empty_input";
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

      traceTimeline(trace, "segment_start");
      const segments = buildSegmentMap(originalText, settings);
      const segSummary = summarizeSegments(segments);
      trace.segments = segSummary;
      traceTimeline(trace, "segment_done");

      if (segSummary.mutable === 0) {
        trace.final.enhanced = false;
        trace.final.reason = "no_mutable_segments";
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

      traceTimeline(trace, "context_start");
      const context = await collectContext(lastRequestContext, settings, trace);
      traceTimeline(trace, "context_done");

      traceTimeline(trace, "router_start");
      const signals = detectSceneSignals(segments, context);
      const selectedRoles = selectRoles(settings.roles, signals, settings.preset, settings);
      traceTimeline(trace, "router_done");

      if (selectedRoles.length === 0 || selectedRoles.every((r) => !r.is_composer && !settings.role_profiles[r.role_id])) {
        trace.final.enhanced = false;
        trace.final.reason = "no_roles_selected";
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

      const deadlineMs = clampNumber(settings.deadline_ms, 10000, 600000, DEFAULT_DEADLINE_MS);
      let deadline = null;

      try {
        deadline = createDeadline(deadlineMs);
        traceTimeline(trace, "schedule_start");
        const { directorResult, composerResult } = await scheduleRoles(
          selectedRoles, settings.role_profiles, segments,
          context.bounded_context_block, segments, deadline, trace, settings.max_parallel
        );
        traceTimeline(trace, "schedule_done");

        traceTimeline(trace, "assemble_start");
        const assembled = assembleOutput(segments, composerResult, directorResult);
        traceTimeline(trace, "assemble_done");

        traceTimeline(trace, "verify_start");
        const verification = verifyOutput(segments, assembled.finalSegments, assembled.output, originalText);
        traceTimeline(trace, "verify_done");

        if (!verification.pass) {
        trace.final.enhanced = false;
        trace.final.reason = `verifier_failed: ${verification.errors.join(", ")}`;
        traceError(trace, trace.final.reason);
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

      if (!assembled.changed || assembled.output === originalText) {
        trace.final.enhanced = false;
        trace.final.reason = assembled.changed ? "output_identical" : "no_candidates_applied";
        if (settings.trace_enabled) await saveTrace(trace);
        return content;
      }

        trace.final.enhanced = true;
        trace.final.reason = composerResult ? "composer_integrated" : "top_candidate_assembled";
        trace.final.original_preview = preview(originalText, 200);
        trace.final.final_preview = preview(assembled.output, 200);

        if (settings.trace_enabled) await saveTrace(trace);
        return assembled.output;
      } finally {
        if (deadline) deadline.cancel();
      }
    } catch (err) {
      traceError(trace, `pipeline_error: ${safeString(err && err.message)}`);
      trace.final.enhanced = false;
      trace.final.reason = "pipeline_error";
      if (settings.trace_enabled) await saveTrace(trace);
      return content;
    }
  }

  function settings_trace_enabled() {
    return true;
  }

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
      <div class="recomposer-section">
        <h3>Presets</h3>
        <label>Preset:
          <select class="recomposer-preset">${presetOptions}</select>
        </label>
        <label>Deadline (ms):
          <input type="number" class="recomposer-deadline" value="${settings.deadline_ms}" min="10000" max="600000" step="5000">
        </label>
        <label>Max Parallel:
          <input type="number" class="recomposer-max-parallel" value="${settings.max_parallel}" min="1" max="20">
        </label>
        <label>Protected Regex:
          <input type="text" class="recomposer-protected-regex" value="${escapeHtml(settings.protected_regex)}" placeholder="user-defined protected pattern">
        </label>
        <label>Context Char Limit:
          <input type="number" class="recomposer-context-limit" value="${settings.context_char_limit}" min="500" max="50000" step="500">
        </label>
        <label><input type="checkbox" class="recomposer-trace-enabled" ${settings.trace_enabled ? "checked" : ""}> Trace Enabled</label>
      </div>`;
  }

  function renderRolePanel(settings) {
    const rows = settings.roles.map((role) => {
      const profile = settings.role_profiles[role.role_id] || defaultRoleProfile(role.role_id);
      const providerOptions = PROVIDERS.map((p) =>
        `<option value="${p}" ${profile.provider === p ? "selected" : ""}>${p}</option>`
      ).join("");
      return `
        <div class="recomposer-role-row" data-role="${escapeHtml(role.role_id)}">
          <h4>${escapeHtml(role.label)} ${role.is_composer ? "(Composer)" : ""}</h4>
          <p class="recomposer-role-purpose">${escapeHtml(role.purpose)}</p>
          <label>Enabled: <input type="checkbox" class="recomposer-role-enabled" ${profile.enabled ? "checked" : ""}></label>
          <label>Provider:
            <select class="recomposer-role-provider">${providerOptions}</select>
          </label>
          <label>Endpoint:
            <input type="text" class="recomposer-role-endpoint" value="${escapeHtml(profile.endpoint)}" placeholder="API endpoint URL">
          </label>
          <label>Model:
            <input type="text" class="recomposer-role-model" value="${escapeHtml(profile.model)}" placeholder="model name">
          </label>
          <label>API Key / Ref:
            <input type="password" class="recomposer-role-key" value="" placeholder="${profile.api_key_ref ? maskKey(profile.api_key_ref) + ' (saved — blank to keep, clear:key to delete)' : 'direct key / arg:name / storage:key / env:KEY'}">
          </label>
          <label>Temperature:
            <input type="number" class="recomposer-role-temp" value="${profile.temperature}" min="0" max="2" step="0.1">
          </label>
          <label>Max Output Tokens:
            <input type="number" class="recomposer-role-max-tokens" value="${profile.max_output_tokens}" min="100" max="32000" step="100">
          </label>
          <label>Timeout (ms):
            <input type="number" class="recomposer-role-timeout" value="${profile.timeout_ms}" min="5000" max="300000" step="5000">
          </label>
          <label>System Prompt:
            <textarea class="recomposer-role-prompt" rows="4">${escapeHtml(profile.system_prompt)}</textarea>
          </label>
          <details>
            <summary>Advanced</summary>
            <label>Fallback Provider:
              <input type="text" class="recomposer-role-fb-provider" value="${escapeHtml(profile.fallback_provider)}" placeholder="fallback provider">
            </label>
            <label>Fallback Model:
              <input type="text" class="recomposer-role-fb-model" value="${escapeHtml(profile.fallback_model)}" placeholder="fallback model">
            </label>
            <label>Fallback Endpoint:
              <input type="text" class="recomposer-role-fb-endpoint" value="${escapeHtml(profile.fallback_endpoint)}" placeholder="fallback endpoint">
            </label>
            <label>Fallback API Key:
              <input type="password" class="recomposer-role-fb-key" value="" placeholder="${profile.fallback_api_key_ref ? maskKey(profile.fallback_api_key_ref) + ' (saved — blank to keep, clear:key to delete)' : 'fallback key ref'}">
            </label>
            <label>Extra Headers (one per line, Key: Value):
              <textarea class="recomposer-role-extra-headers" rows="3">${escapeHtml(profile.extra_headers)}</textarea>
            </label>
            <label>Extra Body (JSON):
              <textarea class="recomposer-role-extra-body" rows="3">${escapeHtml(profile.extra_body)}</textarea>
            </label>
            <label>Reasoning Preset:
              <select class="recomposer-role-reasoning">
                <option value="auto" ${profile.reasoning_preset === "auto" ? "selected" : ""}>auto</option>
                <option value="gpt" ${profile.reasoning_preset === "gpt" ? "selected" : ""}>gpt</option>
                <option value="claude" ${profile.reasoning_preset === "claude" ? "selected" : ""}>claude</option>
                <option value="gemini" ${profile.reasoning_preset === "gemini" ? "selected" : ""}>gemini</option>
                <option value="glm" ${profile.reasoning_preset === "glm" ? "selected" : ""}>glm</option>
                <option value="deepseek" ${profile.reasoning_preset === "deepseek" ? "selected" : ""}>deepseek</option>
              </select>
            </label>
            <label>Reasoning Effort:
              <select class="recomposer-role-reasoning-effort">
                <option value="auto" ${profile.reasoning_effort === "auto" ? "selected" : ""}>auto</option>
                <option value="none" ${profile.reasoning_effort === "none" ? "selected" : ""}>none</option>
                <option value="low" ${profile.reasoning_effort === "low" ? "selected" : ""}>low</option>
                <option value="medium" ${profile.reasoning_effort === "medium" ? "selected" : ""}>medium</option>
                <option value="high" ${profile.reasoning_effort === "high" ? "selected" : ""}>high</option>
              </select>
            </label>
            <label>Reasoning Budget Tokens:
              <input type="number" class="recomposer-role-reasoning-budget" value="${profile.reasoning_budget_tokens}" min="0" max="131072" step="256">
            </label>
            <label>Vertex Flex Mode:
              <select class="recomposer-role-vertex-flex">
                <option value="off" ${profile.vertex_flex_mode === "off" ? "selected" : ""}>off</option>
                <option value="provisioned_then_flex" ${profile.vertex_flex_mode === "provisioned_then_flex" ? "selected" : ""}>provisioned_then_flex</option>
                <option value="flex_only" ${profile.vertex_flex_mode === "flex_only" ? "selected" : ""}>flex_only</option>
              </select>
            </label>
            <label><input type="checkbox" class="recomposer-role-force-json" ${profile.force_json_response ? "checked" : ""}> Force JSON Response</label>
          </details>
        </div>`;
    }).join("");
    return `<div class="recomposer-section"><h3>Roles & Models</h3>${rows}</div>`;
  }

  function renderTracePanel() {
    return `
      <div class="recomposer-section">
        <h3>Trace</h3>
        <button class="recomposer-trace-refresh">Refresh Trace</button>
        <button class="recomposer-trace-clear">Clear Trace</button>
        <div class="recomposer-trace-list"></div>
      </div>`;
  }

  function renderProviderDetails(settings) {
    const roles = settings.roles;
    const rows = roles.map((role) => {
      const p = settings.role_profiles[role.role_id] || defaultRoleProfile(role.role_id);
      return `<tr>
        <td>${escapeHtml(role.role_id)}</td>
        <td>${escapeHtml(p.provider)}</td>
        <td>${escapeHtml(maskKey(p.api_key_ref))}</td>
        <td>${escapeHtml(p.endpoint)}</td>
        <td>${escapeHtml(p.model)}</td>
        <td>${p.timeout_ms}</td>
      </tr>`;
    }).join("");
    return `
      <div class="recomposer-section">
        <h3>Provider Details</h3>
        <table class="recomposer-provider-table">
          <thead><tr><th>Role</th><th>Provider</th><th>Key</th><th>Endpoint</th><th>Model</th><th>Timeout</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`;
  }

  function renderUI(settings) {
    const style = `
      <style>
        .recomposer-root { font-family: sans-serif; padding: 12px; max-width: 900px; }
        .recomposer-section { border: 1px solid #444; border-radius: 8px; padding: 12px; margin-bottom: 16px; }
        .recomposer-section h3 { margin-top: 0; }
        .recomposer-role-row { border: 1px solid #333; border-radius: 6px; padding: 10px; margin-bottom: 12px; }
        .recomposer-role-purpose { color: #aaa; font-size: 0.85em; }
        .recomposer-role-row label { display: block; margin: 4px 0; }
        .recomposer-role-row input, .recomposer-role-row select, .recomposer-role-row textarea { width: 100%; max-width: 500px; }
        .recomposer-provider-table { width: 100%; border-collapse: collapse; }
        .recomposer-provider-table th, .recomposer-provider-table td { border: 1px solid #333; padding: 4px 8px; text-align: left; }
        .recomposer-trace-entry { border: 1px solid #333; border-radius: 4px; padding: 8px; margin-bottom: 8px; font-size: 0.85em; }
        .recomposer-trace-entry pre { white-space: pre-wrap; word-break: break-all; }
        .recomposer-btn { padding: 6px 16px; cursor: pointer; }
      </style>`;
    const html = `
      <div class="recomposer-root">
        ${style}
        <h2>Risu Recomposer v${VERSION}</h2>
        ${renderPresetPanel(settings)}
        ${renderRolePanel(settings)}
        ${renderProviderDetails(settings)}
        ${renderTracePanel()}
        <button class="recomposer-save recomposer-btn">Save Settings</button>
        <button class="recomposer-close recomposer-btn">Close</button>
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
    } catch (err) {
      warn("collectSettingsFromUI error:", err);
    }
    return settings;
  }

  async function refreshTraceList(containerEl) {
    try {
      const traces = await loadTraceList();
      containerEl.innerHTML = traces.slice(0, 10).map((t) => {
        const roleLines = (t.roles || []).map((r) =>
          `${r.role_id}: ${r.status} (${r.provider}/${r.model}) ${r.elapsed_ms}ms${r.error ? " ERR:" + escapeHtml(r.error) : ""}`
        ).join("\n");
        return `<div class="recomposer-trace-entry">
          <strong>${escapeHtml(t.stage)} ${new Date(t.timestamp).toLocaleString()}</strong><br>
          Enhanced: ${t.final.enhanced} — ${escapeHtml(t.final.reason || "")}<br>
          Segments: P:${t.segments.protected} I:${t.segments.inspect_only} M:${t.segments.mutable}<br>
          Candidates: ${t.candidates.total}<br>
          Composer: ${t.composer.used} (${t.composer.status} ${t.composer.elapsed_ms}ms)<br>
          ${t.final.original_preview ? `Orig: ${escapeHtml(t.final.original_preview)}<br>` : ""}
          ${t.final.final_preview ? `Final: ${escapeHtml(t.final.final_preview)}<br>` : ""}
          <pre>${escapeHtml(roleLines)}</pre>
          ${t.errors && t.errors.length ? `<pre style="color:#e55;">${escapeHtml(t.errors.join("\n"))}</pre>` : ""}
        </div>`;
      }).join("");
    } catch (err) {
      containerEl.innerHTML = `<pre>Error loading trace: ${escapeHtml(safeString(err && err.message))}</pre>`;
    }
  }

  let uiRoot = null;

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
      const container = document.createElement("div");
      container.innerHTML = html;
      uiRoot = container.querySelector(".recomposer-root");
      if (document.body) {
        const existing = document.body.querySelector(".recomposer-root");
        if (existing) existing.remove();
        document.body.appendChild(uiRoot);
      }
    } catch (err) {
      warn("UI render error:", err);
      return;
    }
    try {
      if (!uiRoot) return;
      const saveBtn = uiRoot.querySelector(".recomposer-save");
      if (saveBtn) {
        saveBtn.addEventListener("click", async () => {
          const newSettings = await collectSettingsFromUI(uiRoot);
          await saveSettings(newSettings);
          saveBtn.textContent = "Saved!";
          setTimeout(() => { saveBtn.textContent = "Save Settings"; }, 2000);
        });
      }
      const closeBtn = uiRoot.querySelector(".recomposer-close");
      if (closeBtn) {
        closeBtn.addEventListener("click", async () => {
          try {
            if (uiRoot && uiRoot.parentNode) uiRoot.parentNode.removeChild(uiRoot);
          } catch (_) {}
          try {
            if (RR && typeof RR.hideContainer === "function") await RR.hideContainer();
          } catch (_) {}
        });
      }
      const refreshBtn = uiRoot.querySelector(".recomposer-trace-refresh");
      const traceContainer = uiRoot.querySelector(".recomposer-trace-list");
      if (refreshBtn && traceContainer) {
        refreshBtn.addEventListener("click", () => refreshTraceList(traceContainer));
        refreshTraceList(traceContainer);
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

    // Test 4: 역할 3개 후보 Fusion
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
      if (result.consensus.mutable_1 !== "multi_role") throw new Error("consensus not detected");
      return `top: ${result.ranked.mutable_1[0].role_id} score ${result.ranked.mutable_1[0].score.toFixed(1)}`;
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
      const roles = selectRoles(DEFAULT_ROLES, [{ id: "pov_secret_risk", severity: "high" }], "balanced", settings);
      const profiles = settings.role_profiles;
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

    // Test 16: router signal-based role selection — 신호별 결과 차이
    test("16_router_signal_difference", () => {
      const noSignals = [];
      const povSignals = [{ id: "pov_secret_risk", severity: "high" }];
      const worldSignals = [{ id: "world_lore_risk", severity: "medium" }];
      const selectedNone = selectRoles(DEFAULT_ROLES, noSignals, "quality", settings);
      const selectedPov = selectRoles(DEFAULT_ROLES, povSignals, "quality", settings);
      const selectedWorld = selectRoles(DEFAULT_ROLES, worldSignals, "quality", settings);
      const povHasGuard = selectedPov.some((r) => r.role_id === "secret_pov_guard");
      const worldHasReader = selectedWorld.some((r) => r.role_id === "world_reader");
      const noneHasGuard = selectedNone.some((r) => r.role_id === "secret_pov_guard");
      if (!povHasGuard) throw new Error("pov_secret_risk signal should select secret_pov_guard");
      if (!worldHasReader) throw new Error("world_lore_risk signal should select world_reader");
      if (!noneHasGuard) throw new Error("no signals should still select all preset roles (including guard)");
      const povIds = selectedPov.map((r) => r.role_id).sort().join(",");
      const worldIds = selectedWorld.map((r) => r.role_id).sort().join(",");
      if (povIds === worldIds) throw new Error("pov and world signals should produce different role sets");
      return `pov:${povHasGuard} world:${worldHasReader} none:${noneHasGuard} — sets differ`;
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
        { segment_id: "mutable_2", rewrite: "text2", confidence: 0.7, tags: ["style"] },
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
      const withForeign = { segments: { mutable_1: "text 1", foreign_id: "text" } };
      const v2 = validateComposerSchema(withForeign, allowed);
      if (!v2 || Object.keys(v2.segments).length !== 1) throw new Error("foreign segment should be filtered");
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
      const captured = { url: "", auth: "", bodyStr: "", hasReasoning: false };
      const origFetch = globalThis.fetch;
      globalThis.fetch = async (url, opts) => {
        captured.url = String(url);
        captured.auth = (opts && opts.headers && opts.headers.Authorization) || "";
        captured.bodyStr = (opts && opts.body) || "";
        try {
          const parsed = JSON.parse(captured.bodyStr);
          captured.hasReasoning = parsed.reasoning_effort === "medium";
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
          model: "llama3",
          api_key_ref: "sk-ollama-cloud-key",
          temperature: 0.3,
          max_output_tokens: 1024,
          force_json_response: true,
          reasoning_effort: "medium",
          reasoning_budget_tokens: 512,
          extra_headers: "",
          extra_body: "",
        };
        await callProvider(cloudProfile, { system: "sys", user: "usr" }, null);
        if (captured.url.indexOf("ollama.com") < 0) throw new Error(`URL should contain ollama.com, got: ${captured.url}`);
        if (captured.url.indexOf("/chat/completions") < 0) throw new Error(`URL should use /chat/completions, got: ${captured.url}`);
        if (captured.auth.indexOf("Bearer sk-ollama-cloud-key") < 0) throw new Error(`Auth should be Bearer key, got: ${captured.auth}`);
        if (!captured.hasReasoning) throw new Error("reasoning_effort should be in body");
      } finally {
        globalThis.fetch = origFetch;
      }
      return `url:${captured.url.indexOf("/chat/completions") >= 0} auth:${captured.auth.indexOf("Bearer") >= 0} reasoning:${captured.hasReasoning}`;
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
        const roles = selectRoles(DEFAULT_ROLES, [{ id: "pov_secret_risk", severity: "high" }], "balanced", settings);
        const result = await scheduleRoles(roles, settings.role_profiles, segs, "", segs, deadline, trace, 5);
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
        return {
          ok: true,
          text: async () => JSON.stringify({
            choices: [{ message: { content: '{"role":"character_reader","candidates":[{"segment_id":"mutable_1","rewrite":"rewritten","confidence":0.8,"tags":["voice"]},{"segment_id":"mutable_2","rewrite":"rewritten 2","confidence":0.7,"tags":["voice"]},{"segment_id":"mutable_3","rewrite":"rewritten 3","confidence":0.6,"tags":["voice"]}]}' } }],
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
        const roles = selectRoles(DEFAULT_ROLES, [{ id: "pov_secret_risk", severity: "high" }], "balanced", settings);
        const specialistCount = roles.filter((r) => !r.is_composer).length;
        expectedMax = specialistCount + 1;
        await scheduleRoles(roles, settings.role_profiles, segs, "", segs, deadline, trace, 5);
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
          mutable_1: [{ role_id: "character_reader", rewrite: "cand 1", confidence: 0.8, score: 80 }],
        },
        consensus: { mutable_1: "multi_role" },
        conflict: {},
        gap: ["mutable_2"],
      };
      const composerRole = DEFAULT_ROLES.find((r) => r.is_composer);
      const profile = defaultRoleProfile("whole_scene_composer");
      const directorInfo = {
        candidateBundles: { mutable_1: [{ role: "character_reader", confidence: 0.8, rewrite: "cand 1" }] },
        consensus: directorResult.consensus,
        conflict: directorResult.conflict,
        gap: directorResult.gap,
      };
      const prompts = buildRolePrompt(composerRole, profile, mutableSegs, "context block", allSegments, directorInfo);
      const hasOriginal = prompts.user.indexOf("Original:") >= 0;
      const hasCandidates = prompts.user.indexOf("Candidate 1") >= 0;
      const hasDirector = prompts.user.indexOf("Fusion Director") >= 0;
      const hasConsensus = prompts.user.indexOf("multi_role") >= 0;
      const hasGap = prompts.user.indexOf("mutable_2") >= 0;
      if (!hasOriginal) throw new Error("composer input missing original text");
      if (!hasCandidates) throw new Error("composer input missing candidates");
      if (!hasDirector) throw new Error("composer input missing director info");
      if (!hasConsensus) throw new Error("composer input missing consensus");
      if (!hasGap) throw new Error("composer input missing gap info");
      return `original:${hasOriginal} candidates:${hasCandidates} director:${hasDirector} consensus:${hasConsensus} gap:${hasGap}`;
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

    // Test 33: 모의 RisuAI에서 registerSetting/registerButton 등록 + openSettingsUI 실행 검증
    await asyncTest("33_mock_risu_ui_registration_and_open", async () => {
      const mockR = {
        _registeredSettings: [],
        _registeredButtons: [],
        _replacers: {},
        _arguments: [],
        _showContainerCalled: false,
        _hideContainerCalled: false,
        async registerSetting(name, callback, icon, type) {
          this._registeredSettings.push({ name, callback, icon, type });
        },
        async registerButton(buttonObj) {
          this._registeredButtons.push(buttonObj);
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
        const hasButtonId = buttonObj && buttonObj.id === "risu-recomposer-settings";
        const hasButtonLocation = buttonObj && buttonObj.location === "hamburger";
        const hasButtonIconType = buttonObj && buttonObj.iconType === "html";
        const hasButtonCallback = buttonObj && typeof buttonObj.onClick === "function";
        const hasBefore = !!mockR._replacers.beforeRequest;
        const hasAfter = !!mockR._replacers.afterRequest;
        if (!hasSetting) throw new Error("registerSetting not called with html type");
        if (!hasButton) throw new Error("registerButton not called");
        if (!hasButtonId) throw new Error("registerButton missing id");
        if (!hasButtonLocation) throw new Error("registerButton missing location=hamburger");
        if (!hasButtonIconType) throw new Error("registerButton missing iconType=html");
        if (!hasButtonCallback) throw new Error("registerButton missing onClick callback");
        if (!hasBefore || !hasAfter) throw new Error("addRisuReplacer not called for both events");
        const settingCallback = mockR._registeredSettings.find((s) => s.name === "Risu Recomposer").callback;
        await settingCallback();
        if (!mockR._showContainerCalled) throw new Error("showContainer not called by openSettingsUI");
        const rootEl = document.querySelector(".recomposer-root");
        if (!rootEl) throw new Error(".recomposer-root not created in DOM");
        const saveBtn = rootEl.querySelector(".recomposer-save");
        const closeBtn = rootEl.querySelector(".recomposer-close");
        if (!saveBtn) throw new Error("save button not found");
        if (!closeBtn) throw new Error("close button not found");
        closeBtn.click();
        await new Promise((resolve) => setTimeout(resolve, 50));
        if (!mockR._hideContainerCalled) throw new Error("hideContainer not called after close");
        return `setting:${hasSetting} button:${hasButton} id:${hasButtonId} loc:${hasButtonLocation} iconType:${hasButtonIconType} cb:${hasButtonCallback} show:${mockR._showContainerCalled} hide:${mockR._hideContainerCalled}`;
      } finally {
        globalThis.Risuai = origR;
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

  /* ── Plugin Registration ───────────────────────────────── */

  function getR() {
    return (typeof Risuai !== "undefined")
      ? Risuai
      : (typeof risuai !== "undefined" ? risuai : null);
  }

  async function initialize() {
    const RR = getR();
    try {
      const settings = await loadSettings();
      await saveSettings(settings);

      if (RR && typeof RR.addRisuReplacer === "function") {
        await RR.addRisuReplacer("beforeRequest", onBeforeRequest);
        await RR.addRisuReplacer("afterRequest", onAfterRequest);
      }
      try {
        if (RR && typeof RR.registerSetting === "function") {
          await RR.registerSetting("Risu Recomposer", openSettingsUI, "🔧", "html");
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
            location: "hamburger",
            id: "risu-recomposer-settings",
            onClick: openSettingsUI,
          });
        }
      } catch (err) {
        error("registerButton error:", err);
      }
      if (RR && typeof RR.addArgument === "function") {
        await RR.addArgument("recomposer_test", async () => {
          return JSON.stringify(await runInMemoryTests());
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
      DEFAULT_ROLES,
      PRESETS,
      PROVIDERS,
      SIGNAL_ROLE_MAP,
      VOID_HTML_TAGS,
    };
  }
})();