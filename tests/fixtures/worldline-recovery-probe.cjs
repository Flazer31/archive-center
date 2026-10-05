// Production JS Host adapter against the production Go routing/recovery API.
const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const [sourcePath, endpoint] = process.argv.slice(2);
const source = fs.readFileSync(sourcePath, 'utf8');
const functions = new Map();
for (const match of source.matchAll(/^  (?:async )?function (\w+)\(/gm)) {
  const rest = source.slice(match.index), end = /^  }\s*$/m.exec(rest);
  functions.set(match[1], rest.slice(0, end.index + end[0].length));
}
const original = 'The brass key opens the east gate.';
const messages = [
  {role:'user',data:'Inspect the key.',chatId:'u'},
  {role:'char',data:`<GigaTrans>${original}</GigaTrans>\n번역 화면`,chatId:'a'},
  {role:'char',data:'{{specialcomment::branchedfrom::parent::Parent::a::}}',disabled:true,chatId:'marker'},
];
const updates = [], calls = [];
const external = {
  async resolveCurrentActiveChatObject(sid) { assert.equal(sid,'B'); return {chat:{id:'child',message:messages},charIdx:0}; },
  R: {async getCharacterFromIndex(index) { assert.equal(index,0); return {chaId:'stable',chats:[{id:'parent',message:[
    {role:'user',data:'Inspect the key.',chatId:'u'},
    {role:'char',data:'Parent text edited after branching; NEVER recover this.',chatId:'a'},
    {role:'user',data:'Future user input.',chatId:'future-u'},
    {role:'char',data:'Future response.',chatId:'future-a'},
  ]}]}; }},
  serializeSessionRoutingBaselineForBackend() { return null; },
  getSessionRoutingTurnBaseline() { return null; },
  getRequestTimeoutSettingMs() { return 15000; },
  buildAdminRuntimeClientMeta() { return {critic_input_budget_observation:{contract_version:'critic_input_budget_observation.v1',max_input_context_chars:2468}}; },
  updateRuntimeState(...args) { updates.push(args); },
  shouldSkipUserInputPersistence() { return false; },
  debugLog(...args) { throw new Error(`unexpected production diagnostic: ${args.join(' ')}`); },
  async bridgeFetch(path, options) {
    assert.equal(path,'/session-routing/turn-resolution');
    calls.push(options.body);
    const response = await fetch(endpoint+path,{method:options.method,headers:{'Content-Type':'application/json'},body:JSON.stringify(options.body)});
    assert.equal(response.status,200); return response.json();
  },
};
const selected = new Set();
function load(name) {
  if (selected.has(name) || Object.hasOwn(external,name)) return;
  assert(functions.has(name),`production function missing: ${name}`); selected.add(name);
  for (const call of functions.get(name).matchAll(/\b(\w+)\s*\(/g)) if (functions.has(call[1])) load(call[1]);
}
load('preflightActiveChatBackfillIdentity');
const sandbox = vm.createContext({...external,console,TextEncoder,TextDecoder,
  SESSION_FALLBACK:'unknown',AUTO_CONTINUE_USER_INPUT_MARKER:'fixture-empty-user',
  _sessionCache:{sessionId:'B',stableCharacterId:'stable',observedChatUniqueId:'child'}});
vm.runInContext([...selected].map(name=>functions.get(name)).join('\n'),sandbox);
(async()=>{
  const result=await vm.runInContext('preflightActiveChatBackfillIdentity("B")',sandbox);
  assert.equal(result.status,'ok');
  const sent=calls.flatMap(call=>call.inherited_recovery || []);
  assert(sent.length>0,'recovery observations were never submitted');
  assert(sent.every(item=>item.assistant_content===original),'must use translated child snapshot original, never parent future/edited text');
  const update=updates.find(item=>item[0]==='lastWorldlineSourceRecovery');
  assert(update); assert.equal(update[1],'ok');
  console.log(JSON.stringify({submitted:sent,items:update[2].items}));
})().catch(error=>{console.error(error.stack || error);process.exitCode=1;});
