// Isolated browser fixture: production panel, role bindings, save/reset collectors
// and connection-test buttons; storage/host/HTTP are explicit external boundaries.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const repo = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(repo, 'Archive Center.js'), 'utf8').replace(/\r\n/g, '\n');
function fn(name) {
  const match = source.match(new RegExp('  (?:async )?function ' + name + '\\([^\\n]*\\) \\{[\\s\\S]*?\\n  \\}'));
  assert.ok(match, name); return match[0];
}
function between(a,b) { const start=source.indexOf(a); assert.ok(start>=0,a);const end=source.indexOf(b,start);assert.ok(end>=0,b);return source.slice(start,end); }
const defaults=between('  const DEFAULT_SETTINGS = Object.freeze({','\n  });')+'\n  });';
const policies=['LLM_PROVIDER_OPTIONS','REASONING_PRESET_OPTIONS','REASONING_EFFORT_OPTIONS','VERTEX_FLEX_MODE_OPTIONS','LLM_GATEWAY_SERVICE_TIER_OPTIONS','CLAUDE_PROMPT_CACHE_MODE_OPTIONS'];
const constants=policies.map(n=>source.match(new RegExp('  const '+n+' = Object.freeze\\([^\\n]+\\);'))[0]).join('\n');
const fixture=JSON.parse(fs.readFileSync(path.join(repo,'go-service/internal/httpapi/testdata/llm-settings-44-before.json'),'utf8'));
// Reuse the full-panel fixture's declarations, but use a real browser DOM here.
const loading=fs.readFileSync(path.join(repo,'go-service/cmd/js-route-variant-smoke/main_settings_loading_43_test.go'),'utf8');
let declarations=loading.split('script := `')[1].split('// DOM and unrelated subsection renderers')[0];
declarations=declarations.replace("const assert = require('node:assert/strict');",'');
declarations=declarations.replace(/const settings = [^\n]+\nconst getSettings = [^\n]+\n/,'');
declarations=declarations.replace(/const attachSettingsEvents = [^\n]+\n/,'');
declarations=declarations.slice(0,declarations.indexOf('async function bridgeFetch(path)'));
declarations=declarations.replace("const PANEL_CSS = '',",'const PANEL_CSS = '+JSON.stringify(source.match(/const PANEL_CSS = `([\s\S]*?)`;/)[1])+',');
declarations=declarations.replace('const resolveEffectiveCriticConfig = () => ({})','const resolveEffectiveCriticConfig = s => ({apiKey:s.subLlmApiKey,endpoint:s.subLlmEndpoint,model:s.subLlmModel})');
const functions=['renderLlmProviderOptions','renderSettingsPanel','closeSettingsPanel','loadDashboardViewModel','bindLlmSettingsView','applyReasoningFieldsToPayload','sanitizeNumber','sanitizeEnumValue','normalizeLlmProvider','normalizeReasoningPreset','normalizeReasoningBudgetTokens','normalizeVertexFlexModeSetting','normalizeLlmGatewayServiceTierSetting','normalizeClaudePromptCacheModeSetting','sanitizeProviderOverrideJsonSetting','isValidBridgeUrlInput','sanitizeBridgeUrl'];
const events=between('      function getCurrentUiRequestTimeoutMs()', '      // /wakeup');
const runtime=[declarations, "const COMPLETION_TOKEN_PROFILE_VERSION='fixture';", defaults, constants,
`let settings={...DEFAULT_SETTINGS, pluginMainProvider:'openai',pluginMainModel:'gpt-5.6-luna',pluginMainApiKey:'publisher-fixture-key',pluginMainReasoningEffort:'max',pluginMainTemperature:0,pluginMainMaxCompletionTokens:4096,subLlmProvider:'claude',subLlmModel:'claude-sonnet-4-5',subLlmApiKey:'critic-fixture-key',subLlmReasoningBudgetTokens:2048,subLlmMaxCompletionTokens:3072};
const getSettings=()=>({...settings});
const safeSettingsForLog=x=>x, maskApiKey=x=>'fixture', pluginMainHasConfig=()=>true;
const syncMemoryDeliveryBudgetControls=()=>{},syncLorebookRefreshButton=()=>{},syncNarrativeGuideControls=()=>{},syncAllRangesFromInputs=()=>{};
const getSubLlmTimeoutSettingMs=v=>Number(v ?? settings.subLlmTimeoutMs), getPluginMainTimeoutSettingMs=v=>Number(v ?? settings.pluginMainTimeoutMs);
const getPluginMainProviderSetting=v=>v, getSubLlmProviderSetting=v=>v, getPluginMainMaxCompletionTokensSetting=v=>Number(v ?? settings.pluginMainMaxCompletionTokens), getSubLlmMaxCompletionTokensSetting=v=>Number(v ?? settings.subLlmMaxCompletionTokens);
const syncConfigToBackend=async()=>({status:'ok'}), runtimeConfigSyncRoleReady=()=>true;
window.calls=[]; window.saves=[]; window.pendingViews=[]; window.failures=failures;
window.currentSettings=()=>structuredClone(settings); window.confirm=()=>true;
async function saveSettings(){window.saves.push(structuredClone(settings));return true;}
async function updateSettings(patch){settings={...settings,...patch};return saveSettings();}
async function bridgeFetch(route,options={}) {
 window.calls.push({route,options:structuredClone(options)});
 if(route==='/dashboard/view-model') return {html:'fixture status'};
 if(route==='/config/view-model') return new Promise(resolve=>window.pendingViews.push({draft:options.body,resolve}));
 if(route.startsWith('/proxy/plugin-main')) return {choices:[{message:{content:'OK'}}],status:'ok',detail:'fixture OK'};
 throw new Error('Unexpected HTTP route '+route);
}
const formatBridgeFailureForDisplay=()=> 'fixture failure';`,
...functions.map(fn), 'function attachSettingsEvents(){const $ = id => document.getElementById(id);'+events+'}',
`window.openPanel=()=>renderSettingsPanel({recompose:true});window.resetFixture=async()=>{settings={...DEFAULT_SETTINGS};await saveSettings();};`
].join('\n');

(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.ARCHIVE_CENTER_UI_BROWSER?{executablePath:process.env.ARCHIVE_CENTER_UI_BROWSER}:{})});
 try{
  const page=await browser.newPage({viewport:{width:1500,height:1000}});const errors=[];
  page.on('pageerror',e=>errors.push(e.message));await page.route('**/*',r=>r.abort());
  await page.setContent('<html><meta charset="utf-8"><body></body></html>');await page.addScriptTag({content:runtime});await page.evaluate(()=>window.openPanel());
  const answer=async(index,provider,model,effort)=>{
    const captured=fixture.find(c=>c.input.provider===provider&&c.input.model===model&&c.input.currentEffort===effort&&c.input.isFirstSync);
    assert.ok(captured, [provider,model,effort].join('/'));
    await page.evaluate(({index,view})=>window.pendingViews[index].resolve(view),{index,view:{...captured.view,allowedPresets:['auto','gpt','gemini','claude','glm','custom']}});
  };
  assert.deepEqual(await page.evaluate(()=>window.failures),[]);
  const expectedProviders=['openai','claude','gemini','openrouter','opencode','opencode-go','llmgateway','vercel','neuralwatt','vertex','copilot','ollama','custom'];
  for(const role of ['pluginMain','subLlm']) assert.deepEqual(await page.locator('#mo-'+role+'Provider option').evaluateAll(els=>els.map(el=>el.value)),expectedProviders);
  assert.equal(await page.locator('#mo-pluginMainApiKey').inputValue(),'publisher-fixture-key');
  await answer(0,'openai','gpt-5.6-luna','max');await answer(1,'claude','claude-sonnet-4-5','high');
  assert.equal(await page.locator('#mo-pluginMainReasoningEffort').inputValue(),'max');
  assert.equal(await page.locator('#mo-subLlmReasoningBudgetTokens').inputValue(),'2048');
  // Two overlapping draft changes: an older response cannot replace newer controls.
  await page.locator('#mo-pluginMainModel').fill('gpt-5.2');const stale=await page.evaluate(()=>window.pendingViews.length-1);
  await page.locator('#mo-pluginMainModel').fill('gpt-5.6-luna');const latest=await page.evaluate(()=>window.pendingViews.length-1);
  await answer(latest,'openai','gpt-5.6-luna','max');await answer(stale,'openai','gpt-5.2','max');
  assert.equal(await page.locator('#mo-pluginMainReasoningEffort').inputValue(),'max');
  assert.equal(await page.evaluate(()=>window.saves.length),0,'draft lookup wrote settings');
  // A budget edit made during an in-flight lookup remains a form edit.
  await page.locator('#mo-subLlmModel').fill('claude-sonnet-4-5');const budgetRequest=await page.evaluate(()=>window.pendingViews.length-1);
  await page.locator('#mo-subLlmReasoningBudgetTokens').fill('4096');await answer(budgetRequest,'claude','claude-sonnet-4-5','high');
  assert.equal(await page.locator('#mo-subLlmReasoningBudgetTokens').inputValue(),'4096');
  // The Critic test uses its own saved connection and observed reasoning input.
  await page.locator('#mo-test-call-critic').click();
  const criticTest=await page.evaluate(()=>window.calls.find(c=>c.route.includes('connection_test=critic')));
  assert.ok(criticTest,await page.locator('#mo-test-result').textContent());
  assert.equal(criticTest.options.body.api_key,'critic-fixture-key');
  assert.equal(criticTest.options.body.reasoning_input.budget,4096);
  // Explicit blank values and hidden provider-specific fields survive save/reopen.
  await page.locator('#mo-pluginMainEndpoint').fill('');await page.locator('#mo-subLlmApiKey').fill('');
  await page.locator('#mo-pluginMainExtraHeadersJson').fill('{"X-Test":"saved"}');
  await page.locator('#mo-pluginMainVertexFlexMode').evaluate(el=>el.value='flex_only');
  await page.locator('#mo-save-btn').click();
  assert.deepEqual(await page.evaluate(()=>window.failures),[]);
  assert.equal(await page.evaluate(()=>window.saves.length),1);
  const saved=await page.evaluate(()=>window.currentSettings());
  assert.equal(saved.pluginMainApiKey,'publisher-fixture-key');assert.equal(saved.subLlmApiKey,'');assert.equal(saved.pluginMainEndpoint,'');
  assert.equal(Number(saved.pluginMainTemperature),0);assert.equal(Number(saved.pluginMainMaxCompletionTokens),4096);assert.equal(Number(saved.subLlmMaxCompletionTokens),3072);
  assert.equal(saved.pluginMainVertexFlexMode,'flex_only');assert.equal(saved.pluginMainExtraHeadersJson,'{"X-Test":"saved"}');
  await page.evaluate(()=>window.openPanel());
  assert.equal(await page.locator('#mo-subLlmApiKey').inputValue(),'');assert.equal(await page.locator('#mo-pluginMainApiKey').inputValue(),'publisher-fixture-key');
  // A failed ViewModel query leaves saved and unsaved fields visible and intact.
  const offline=await page.evaluate(()=>window.pendingViews.length-2);await page.evaluate(i=>window.pendingViews[i].resolve(null),offline);
  assert.equal(await page.locator('#mo-pluginMainReasoningEffort').inputValue(),'max');
  assert.match(await page.locator('#mo-pluginMainReasoningGuide').textContent(),/조회 실패/);
  // Execute the real Publisher test button: no extra display/model request is needed.
  await page.locator('#mo-test-call-publisher').click();
  const test=await page.evaluate(()=>window.calls.find(c=>c.route==='/proxy/plugin-main'));
  assert.ok(test,await page.locator('#mo-test-result').textContent());
  assert.deepEqual(test.options.body.reasoning_input,{preset:'auto',effort:'max',budget:0});
  assert.equal(test.options.body.api_key,'publisher-fixture-key');assert.equal(test.options.body.reasoning_effort,undefined);
  const previews=await page.evaluate(()=>window.calls.filter(c=>c.route==='/config/view-model'));
  assert.ok(previews.length>2);for(const c of previews){assert.equal(c.options.body.api_key,undefined);assert.equal(c.options.body.apiKey,undefined);assert.ok(c.options.bridgeSettings.bridgeUrl);}
  await page.setViewportSize({width:390,height:844});
  assert.ok(await page.locator('#mo-settings-overlay').isVisible());
  await page.locator('#mo-reset-btn').click();
  assert.equal(await page.locator('#mo-pluginMainApiKey').inputValue(),'');assert.equal(await page.locator('#mo-subLlmApiKey').inputValue(),'');
  assert.equal(await page.evaluate(()=>window.saves.length),2);
  assert.deepEqual(await page.evaluate(()=>window.failures),[]);assert.deepEqual(errors,[]);
  console.log('PASS: production settings DOM and role events; overlapping drafts, in-flight edits, offline preservation, save/reopen/reset, hidden fields, empty key, Publisher test, mobile; no external requests.');
 }finally{await browser.close();}
})().catch(err=>{console.error(err);process.exitCode=1;});
