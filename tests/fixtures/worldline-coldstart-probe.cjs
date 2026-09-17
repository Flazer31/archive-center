// Offline reproduction: execute active production functions, with Host reads
// supplied as fixture data and routing delegated to a real Go test server.
const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const [sourcePath, endpoint, variant] = process.argv.slice(2);
const source = fs.readFileSync(sourcePath, 'utf8');
const functions = new Map();
for (const match of source.matchAll(/^  (?:async )?function (\w+)\(/gm)) {
  const rest = source.slice(match.index);
  const end = /^  }\s*$/m.exec(rest);
  assert(end, `missing production function end: ${match[1]}`);
  functions.set(match[1], rest.slice(0, end.index + end[0].length));
}
const original = 'The brass key opens the east gate.';
const translated = '황동 열쇠로 동쪽 문을 연다.';
const translatedBody = `<GigaTrans>${original}</GigaTrans>\n${translated}`;
const messages = [
  {role:'user', data:'Inspect the gate.', chatId:'root-u'},
  {role:'char', data:'The gate is locked.', chatId:'root-a'},
  {role:'char', data:'{{specialcomment::branchedfrom::root::Root::root-a::}}', disabled:true},
  {role:'user', data:'Inspect the key.', chatId:'child-u'},
  {role:'char', data:variant === 'plain' ? original : translatedBody, chatId:'child-a'},
];
if (variant === 'assistant_only') messages.splice(3, 1);
if (variant === 'all_inherited') messages.splice(3);
const routeCalls = [];
const external = {
  async resolveCurrentActiveChatObject(sid, host) {
    assert.equal(sid, 'B'); assert.equal(host.hostChatId, 'branch');
    return {chat:{id:'branch',message:messages},source:'fixture_host'};
  },
  async explorerFetchAllChatLogsForSession(sid) { assert.equal(sid,'B'); return {items:[],limited:false}; },
  async explorerFetchTimelineItemsForSessionDryRun(sid) { assert.equal(sid,'B'); return {items:[],limited:false}; },
  async fetchWorldRules(sid) { assert.equal(sid,'B'); return {items:[],count:0}; },
  summarizeActiveChatRawMessageShape() { return {}; },
  debugLog(...args) { throw new Error(`unexpected production error: ${args.join(' ')}`); },
  shouldSkipUserInputPersistence() { return false; },
  async requestBackendSessionRoutingTurnResolution(sid, mode, rows, options={}) {
    assert.equal(sid,'B'); assert.equal(mode,'batch');
    const observations = rows.map((row, index) => ({
      observation_index:index, risu_user_message_index:row.risuUserMessageIndex,
      risu_assistant_message_index:row.risuAssistantMessageIndex,
      observed_pair_ordinal:row.observedPairOrdinal,
      assistant_message_id:row.message_id, assistant_generation_id:row.generation_id,
      assistant_content_hash:row.content_hash, assistant_content:row.assistant_content,
      adjacent_user_present:row.adjacent_user_present, adjacent_user_content:row.adjacent_user_content,
      assistant_disabled_state:row.disabled_state, assistant_streaming_state:row.streaming_state,
      assistant_final_state:row.final_state,
    }));
    const response = await fetch(endpoint + '/session-routing/turn-resolution', {
      method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        chat_session_id:sid,mode,stable_character_id:'stable',stable_character_id_state:'observed',
        host_chat_id:'branch',host_chat_id_state:'observed',observations,
        routing_context:options.routingContext || '',
      }),
    });
    assert.equal(response.status,200);
    const body = await response.json();
    routeCalls.push({context:options.routingContext || '',request:observations,result:body.resolved_observations});
    return {status:body.status,resolvedObservations:body.resolved_observations,backendDecision:body};
  },
};
const roots = ['computeActiveChatRescanDryRunPlan','buildSessionNormalizeRepairEntriesFromDryRunPlan',
  'buildSessionNormalizeCompletedTurnPairs','buildRollbackAssistantObservations'];
const selected = new Set();
function load(name) {
  if (selected.has(name) || Object.hasOwn(external,name)) return;
  assert(functions.has(name), `missing production function: ${name}`);
  selected.add(name);
  for (const call of functions.get(name).matchAll(/\b(\w+)\s*\(/g)) {
    if (functions.has(call[1])) load(call[1]);
  }
}
roots.forEach(load);
const sandbox = vm.createContext({...external,console,TextEncoder,TextDecoder,
  AUTO_CONTINUE_USER_INPUT_MARKER:'fixture-empty-user'});
vm.runInContext([...selected].map(name => functions.get(name)).join('\n'),sandbox);
(async () => {
  sandbox.fixtureChat = {id:'branch',message:messages};
  const first = vm.runInContext('buildSessionNormalizeCompletedTurnPairs(fixtureChat)',sandbox);
  if (variant !== 'assistant_only' && variant !== 'all_inherited') {
    assert.equal(first.pairs.at(-1).assistantContent,original, 'production first pass must clean the translation');
  }
  const plan = await vm.runInContext('computeActiveChatRescanDryRunPlan("B",{hostChatId:"branch"})',sandbox);
  sandbox.plan = plan;
  const entries = vm.runInContext('buildSessionNormalizeRepairEntriesFromDryRunPlan(plan)',sandbox);
  assert.equal(routeCalls.length,2,'both routing passes must execute');
  assert(routeCalls[1].result.length > 0,'assistant merge must execute');
  console.log(JSON.stringify({variant,original,translated,first:first.pairs,routeCalls,
    pairs:plan.pairs,processableTurns:plan.processableTurns,entries}));
})().catch(error => { console.error(error.stack || error); process.exitCode=1; });
