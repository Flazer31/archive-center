"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const runtimePath = path.join(__dirname, "Archive Center.js");
const runtimeSource = fs.readFileSync(runtimePath, "utf8").replace(
  /\r?\nawait init\(\);\r?\n/,
  "\nglobalThis.__archiveCenterRuntimeTest = { sanitizeSettings, renderSessionNormalizeResultHtml, ensureActiveChatCompletedTurnsBackfilled };\nawait init();\n",
);

function jsonResponse(data, status = 200) {
  const text = JSON.stringify(data);
  return {
    status,
    ok: status >= 200 && status < 300,
    async json() { return data; },
    async text() { return text; },
  };
}

function makeStorage(initial = {}) {
  const values = new Map(Object.entries(initial));
  return {
    getItem(key) { return values.has(String(key)) ? values.get(String(key)) : null; },
    setItem(key, value) { values.set(String(key), String(value)); },
    removeItem(key) { values.delete(String(key)); },
  };
}

function makeDeferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

async function waitFor(predicate, label) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  throw new Error("timed out waiting for " + label);
}

async function loadRuntime(options = {}) {
  const settingsRaw = Object.assign({
    uiLanguage: "en",
    turnWorkflowHUDEnabled: false,
  }, options.settingsRaw || {});
  const settingsKey = "risu_memory_orchestrator_settings";
  const settingsJson = JSON.stringify(settingsRaw);
  const localPluginStorage = makeStorage({ [settingsKey]: settingsJson });
  const pluginStorage = makeStorage();
  const localStorage = makeStorage({ [settingsKey]: settingsJson });
  const hooks = {};
  const requests = [];
  const activeChat = options.activeChat || { id: "chat-1", name: "Test Chat", message: [] };
  const worldlineGate = options.worldlineGate || null;
  const worldlineState = options.worldlineState || "confirmed";

  const Risuai = {
    pluginStorage,
    async getLocalPluginStorage() { return localPluginStorage; },
    async addRisuScriptHandler(name, callback) { hooks[name] = callback; },
    async addRisuReplacer(name, callback) { hooks[name] = callback; },
    async addRisuChatListener(name, callback) { hooks[name] = callback; },
    async onUnload(callback) { hooks.unload = callback; },
    async getCurrentCharacterIndex() { return 0; },
    async getCurrentChatIndex() { return 0; },
    async getCharacter() { return { chaId: "character-1", name: "Character", chats: [activeChat] }; },
    async getChatFromIndex() { return activeChat; },
    async nativeFetch(url, init = {}) {
      const parsedUrl = new URL(url);
      const body = init.body ? JSON.parse(init.body) : null;
      const request = { path: parsedUrl.pathname, method: init.method || "GET", body };
      requests.push(request);

      if (request.path === "/config/update") {
        return jsonResponse({
          status: "ok",
          backend_instance_id: "backend-test",
          runtime_config_trace: { synced: true },
        });
      }
      if (request.path === "/session-routing/turn-resolution") {
        if (body && body.worldline_observation && worldlineGate) {
          await worldlineGate.promise;
        }
        return jsonResponse({
          status: "ok",
          contract_version: "session-routing.turn-resolution.v1",
          resolution: "normal",
          chat_session_id: String(body && body.chat_session_id || ""),
          binding_required: false,
          binding_acknowledged: true,
          turn_index: 1,
          completed_turns: 0,
          local_turn_index: 1,
          worldline: body && body.worldline_observation
            ? { state: worldlineState, reason: worldlineState === "confirmed" ? "" : "branch_parent_unresolved" }
            : null,
        });
      }
      if (request.path === "/prepare-turn") {
        return jsonResponse({
          status: "ok",
          source: "database",
          backend_instance_id: "backend-test",
          current_input_decision: {
            status: "deferred",
            reason_code: "runtime_test_stop_after_first_prepare",
          },
        });
      }
      return jsonResponse({ status: "ok" });
    },
  };

  const quietConsole = { log() {}, warn() {}, error() {}, debug() {} };
  const context = vm.createContext({
    AbortController,
    ArrayBuffer,
    TextDecoder,
    TextEncoder,
    URL,
    URLSearchParams,
    Uint8Array,
    clearInterval,
    clearTimeout,
    console: quietConsole,
    localStorage,
    performance,
    setInterval,
    setTimeout,
    Risuai,
  });
  await vm.runInContext(runtimeSource, context, { filename: runtimePath });
  return { context, hooks, requests, activeChat };
}

function requestBodies(runtime, requestPath) {
  return runtime.requests.filter((request) => request.path === requestPath).map((request) => request.body);
}

async function testCompletionTokenProfileMigration() {
  const legacy = await loadRuntime({
    settingsRaw: {
      pluginMainMaxCompletionTokens: 1024,
      subLlmMaxCompletionTokens: 1024,
    },
  });
  const legacyConfig = requestBodies(legacy, "/config/update").at(-1);
  assert.equal(legacyConfig.mainMaxCompletionTokens, 30000);
  assert.equal(legacyConfig.criticMaxCompletionTokens, 30000);
  const migratedSettings = legacy.context.__archiveCenterRuntimeTest.sanitizeSettings({
    pluginMainMaxCompletionTokens: 1024,
    subLlmMaxCompletionTokens: 1024,
  });
  assert.equal(migratedSettings.completionTokenProfileVersion, "p409_30000_v1");

  const custom = await loadRuntime({
    settingsRaw: {
      pluginMainMaxCompletionTokens: 4096,
      subLlmMaxCompletionTokens: 2048,
    },
  });
  const customConfig = requestBodies(custom, "/config/update").at(-1);
  assert.equal(customConfig.mainMaxCompletionTokens, 4096);
  assert.equal(customConfig.criticMaxCompletionTokens, 2048);

  const deliberate = await loadRuntime({
    settingsRaw: {
      completionTokenProfileVersion: "p409_30000_v1",
      pluginMainMaxCompletionTokens: 1024,
      subLlmMaxCompletionTokens: 1024,
    },
  });
  const deliberateConfig = requestBodies(deliberate, "/config/update").at(-1);
  assert.equal(deliberateConfig.mainMaxCompletionTokens, 1024);
  assert.equal(deliberateConfig.criticMaxCompletionTokens, 1024);
}

async function testDelayedBranchIdentityPreflight() {
  const gate = makeDeferred();
  const runtime = await loadRuntime({ worldlineGate: gate });
  runtime.activeChat.message.push(
    { role: "user", data: "Earlier input", chatId: "user-1" },
    { role: "char", data: "Earlier answer", chatId: "assistant-1" },
    { role: "system", data: "{{specialcomment::branchedfrom::assistant-1}}", disabled: true, chatId: "branch-1" },
    { role: "user", data: "New branch input", chatId: "user-2" },
  );
  const payload = { messages: [{ role: "user", content: "New branch input" }] };
  const pending = runtime.hooks.beforeRequest(payload, "model");
  await waitFor(
    () => requestBodies(runtime, "/session-routing/turn-resolution")
      .some((body) => body && body.worldline_observation),
    "worldline identity request",
  );
  await waitFor(
    () => requestBodies(runtime, "/prepare-turn").length === 1,
    "current prepare-turn while branch identity remains pending",
  );
  const returned = await pending;
  assert.equal(returned, payload);
  assert.equal(requestBodies(runtime, "/prepare-turn").length, 1);
  gate.resolve();
}

async function testUnresolvedBranchPreservesPayload() {
  const runtime = await loadRuntime({ worldlineState: "unresolved" });
  runtime.activeChat.message.push(
    { role: "char", data: "Earlier answer", chatId: "assistant-1" },
    { role: "system", data: "{{specialcomment::branchedfrom::assistant-1}}", disabled: true, chatId: "branch-1" },
    { role: "user", data: "New branch input", chatId: "user-2" },
  );
  const payload = { messages: [{ role: "user", content: "New branch input" }] };
  const returned = await runtime.hooks.beforeRequest(payload, "model");
  assert.equal(returned, payload);
  assert.equal(requestBodies(runtime, "/prepare-turn").length, 1);
}

async function testAssistantOnlyBackfillReportsNormalizeRequirement() {
  const runtime = await loadRuntime();
  runtime.activeChat.message.push({ role: "char", data: "Opening assistant text", chatId: "assistant-1" });
  const result = await runtime.context.__archiveCenterRuntimeTest.ensureActiveChatCompletedTurnsBackfilled(
    "char_0_cid_chat-1",
    {
      reason: "runtime_test",
      hostContext: {
        sessionId: "char_0_cid_chat-1",
        charIdx: 0,
        chatIdx: 0,
        hostChatId: "chat-1",
        hostChatIdState: "observed",
        stableCharacterId: "character-1",
        stableCharacterIdState: "observed",
      },
    },
  );
  assert.equal(result.status, "skipped");
  assert.equal(result.reason, "assistant_only_requires_session_normalize");
  assert.equal(result.assistantOnlyCount, 1);
  assert.equal(requestBodies(runtime, "/complete-turn").length, 0);
  assert.equal(requestBodies(runtime, "/proxy/plugin-main").length, 0);
}

async function testDeferredNormalizeHeading() {
  const runtime = await loadRuntime();
  const render = runtime.context.__archiveCenterRuntimeTest.renderSessionNormalizeResultHtml;
  for (const status of ["deferred", "partial_deferred"]) {
    const html = render({ status });
    assert.match(html, /Deferred/);
    assert.doesNotMatch(html, /Cold start complete/);
  }
}

async function main() {
  await testCompletionTokenProfileMigration();
  await testDelayedBranchIdentityPreflight();
  await testUnresolvedBranchPreservesPayload();
  await testAssistantOnlyBackfillReportsNormalizeRequirement();
  await testDeferredNormalizeHeading();
  console.log("archive-center runtime tests: 5 passed");
}

main().catch((error) => {
  console.error(error && error.stack || error);
  process.exitCode = 1;
});
