'use strict';
// Exercises the production settings serializer and complete-turn transport.
// Host observation/HUD are isolated; no personal settings or provider calls.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');

function harness(sourcePath, transport, overrides = {}) {
  const source = fs.readFileSync(sourcePath, 'utf8').replaceAll('\r\n', '\n');
  const ctx = vm.createContext({console, URL, URLSearchParams, AbortController, setTimeout, clearTimeout});
  const start = source.indexOf('  const PLUGIN_ID =');
  const end = source.indexOf('  const _i18n =');
  assert(start >= 0 && end > start);
  vm.runInContext(source.slice(start, end), ctx);
  // Declarations are loaded verbatim; plugin initialization/Host registration
  // are deliberately outside this focused transport regression.
  for (const match of source.matchAll(/^  (?:async )?function \w+\(/gm)) {
    const close = source.indexOf('\n  }', match.index);
    assert(close > match.index);
    vm.runInContext(source.slice(match.index, close + 4), ctx);
  }
  vm.runInContext('var settings = {...DEFAULT_SETTINGS};', ctx);
  Object.assign(ctx.settings, {
    enabled:true, dbEnabled:true, subLlmProvider:'ollama',
    subLlmApiKey:'synthetic-critic-secret', subLlmEndpoint:'http://127.0.0.1:1/v1',
    subLlmModel:'glm-5.3:cloud', subLlmTimeoutMs:240000,
    criticReferenceMaxChars:4000, maxInputContextChars:64000,
    embeddingProvider:'ollama', embeddingEndpoint:'http://127.0.0.1:1/v1',
    embeddingApiKey:'synthetic-embedding-secret', embeddingModel:'synthetic-embedding',
    ...overrides,
  });
  Object.assign(ctx, {
    safeCall: async callback => callback(),
    bridgeFetch: transport,
    turnWorkflowHUDRequestIdFromCompleteBody: () => '',
    debugLog: () => {}, updateRuntimeState: () => {},
  });
  return ctx;
}

async function dispatchRecovery(ctx, payload, kind) {
  // Only Host re-observation and local durable storage are substituted here;
  // run the production queue/resume function through the real transport owner.
  Object.assign(ctx, {
    refreshQueuedCompleteTurnSourceObservation: async () => true,
    prunePersistedFailedQueue: () => {},
    flushQueueSave: async () => true,
    removePendingFinalConfirmationRecovery: async () => true,
  });
  if (kind === 'failed_queue') {
    ctx._failedQueue = [{type:'complete_turn', payload, attempts:0, addedAt:new Date().toISOString()}];
    await ctx.drainOneFailedQueueItem();
    return ctx._failedQueue;
  } else {
    let pending;
    ctx.queuePendingFinalConfirmation = item => {pending=item;return true;};
    assert(await ctx.queuePendingCompleteTurnPayload(payload, 'pending_confirmation', '', {persist:false}));
    assert(pending && typeof pending.resume === 'function');
    await pending.resume({observationKey:'synthetic-final-observation'});
  }
}

async function run() {
  const sourcePath = process.argv[2] || path.join(__dirname, '..', 'Archive Center.js');
  const sent = [];
  const ctx = harness(sourcePath, async (route, options) => {
    assert.equal(route, '/complete-turn');
    sent.push(structuredClone(options.body));
    return {status:'ok',save_ok:true,retryable:false,queue_action:''};
  });
  const retained = {chat_session_id:'synthetic', turn_index:1, user_input:'Open the box.', assistant_content:'The box contains a blue key.', context_messages:[], client_meta:{
    critic_input_budget_observation:{critic_reference_max_chars:1733},
    source_acceptance_observation:{observed_content_hash:'synthetic-hash'},
    turn_finalization_mode:'next_user_input',
  }};
  const before = JSON.stringify(retained);
  const states = [];
  ctx.updateRuntimeState = (...args) => states.push(args);
  for (let iteration=0; iteration<2; iteration++) {
    ctx.settings.subLlmModel = iteration ? 'changed-critic' : 'glm-5.3:cloud';
    ctx.settings.subLlmApiKey = 'synthetic-secret-'+iteration;
    await ctx.tryCompleteTurn(1, retained.user_input, retained.assistant_content, [], 'synthetic', null, retained);
    assert.equal(sent.length, iteration+1, 'complete-turn must actually be dispatched');
    const meta = sent[iteration].client_meta;
    assert.equal(meta.critic?.model, ctx.settings.subLlmModel, 'dispatch lost current Critic configuration');
    assert.equal(meta.critic.api_key, ctx.settings.subLlmApiKey);
    assert.equal(meta.critic.timeout_ms, 240000);
    assert.equal(meta.embedding.api_key, 'synthetic-embedding-secret');
    assert.equal(meta.critic_input_budget_observation.critic_reference_max_chars, 1733, 'replay must retain observed input budget');
    assert.equal(JSON.stringify(retained), before, 'transport must not put credentials in retained body');
    const queue = ctx.buildCompleteTurnQueuePayload(sent[iteration]);
    assert(queue && !queue.client_meta.critic && !queue.client_meta.embedding, 'persistent queue must omit credentials');
  }
  for (const kind of ['failed_queue', 'pending_final']) {
    for (let iteration=0; iteration<2; iteration++) {
      ctx.settings.subLlmApiKey = 'synthetic-'+kind+'-'+iteration;
      const remaining = await dispatchRecovery(ctx, retained, kind);
      if (kind === 'failed_queue') assert.equal(remaining.length, 0, 'recovered queue item was not consumed');
      const meta = sent.at(-1).client_meta;
      assert.equal(meta.critic?.api_key, ctx.settings.subLlmApiKey, kind+' lost Critic config');
      assert.equal(meta.embedding?.api_key, 'synthetic-embedding-secret');
      assert.equal(meta.critic_input_budget_observation.critic_reference_max_chars, 1733);
      assert.equal(JSON.stringify(retained), before, kind+' mutated persistent body');
    }
  }
  assert.equal(sent.length,6);
  assert(!states.some(([name,status]) => name === 'lastCompleteTurnStatus' && ['warn','fail'].includes(status)), 'successful recovery must not be shown as failure');
  console.log(JSON.stringify({passed:true, deliveries:sent.length, current_settings:true, original_budget_preserved:true, retained_body_unchanged:true, queue_credentials:false}));
}
module.exports = {harness,dispatchRecovery};
if (require.main === module) run().catch(error => {console.error(error); process.exitCode=1;});
