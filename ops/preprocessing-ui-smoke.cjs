// Local browser fixture: no backend startup, installed RisuAI access or provider calls.
// Requires Playwright; set NODE_PATH and ARCHIVE_CENTER_UI_BROWSER for a bundled runtime.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const repo = path.resolve(__dirname, '..');
const source = fs.readFileSync(process.env.ARCHIVE_CENTER_UI_SOURCE || path.join(repo, 'Archive Center.js'), 'utf8').replace(/\r\n/g, '\n');
const go = fs.readFileSync(path.join(repo, 'go-service/internal/httpapi/prepare_turn_multi_agent.go'), 'utf8').replace(/\r\n/g, '\n');
const output = path.resolve(process.argv[2] || path.join(repo, '.tmp-preprocessing-ui'));
fs.mkdirSync(output, { recursive: true });
function owner(start, end) {
  const a = source.indexOf(start), b = source.indexOf(end, a + start.length);
  assert.ok(a >= 0 && b > a, 'production owner missing: ' + start);
  return source.slice(a, b);
}
function goMap(name) {
  const block = go.match(new RegExp('var ' + name + ' = map\\[string\\]string\\{([\\s\\S]*?)\\n\\}'))[1];
  return Object.fromEntries([...block.matchAll(/"([^"]+)":\s*("(?:\\.|[^"\\])*"|\x60[^\x60]*\x60)/g)].map(m => [m[1], m[2][0] === '`' ? m[2].slice(1, -1) : JSON.parse(m[2])]));
}
const roles = JSON.parse('[' + go.match(/var multiAgentRoles = \[\]string\{([^}]+)\}/)[1] + ']');
const jevModeSource = fs.readFileSync(path.join(repo, 'go-service/internal/httpapi/prepare_turn_jev.go'), 'utf8');
const modeViews = Object.fromEntries([...jevModeSource.matchAll(/"([01]{2})": map\[string\]string\{"mode": "([^"]+)", "label": "([^"]+)", "description": "([^"]+)"\}/g)].map(m=>[m[1], {mode:m[2],label:m[3],description:m[4]}]));
assert.equal(Object.keys(modeViews).length,4);
const jevPrompts = [...jevModeSource.matchAll(/\{Key: "([^"]+)", Label: "([^"]+)", Mode: "([^"]+)", Type: "([^"]+)",\r?\n\s*Instructions: ("(?:\\.|[^"\\])*"),\r?\n\s*Criteria: \[\]jevPromptCriterion\{([\s\S]*?)\r?\n\s*\}\}/g)].map(m => ({
  key:m[1],label:m[2],mode:m[3],type:m[4],instructions:JSON.parse(m[5]),
  criteria:[...m[6].matchAll(/\{"([^"]+)", ("(?:\\.|[^"\\])*")\}/g)].map(c=>({key:c[1],text:JSON.parse(c[2])}))
}));
assert.equal(jevPrompts.length,5,'shipped Jev question definitions missing');
const fixture = {
  mode_views: modeViews,
  jev_prompts: jevPrompts.map(p=>({defaults:p,effective:structuredClone(p)})),
  role_order: roles, role_names: goMap('multiAgentRoleNames'), default_prompts: goMap('multiAgentRolePrompts'),
  shared_prompt: go.match(/const multiAgentSharedPrompt = \x60([\s\S]*?)\x60/)[1],
  default_shared_prompt: go.match(/const multiAgentSharedPrompt = \x60([\s\S]*?)\x60/)[1],
  settings: { enabled: false, jev:{enabled:false,endpoint:'https://api.typesafe.ai/v1/systemone',model:'jev-1.13.0',api_key_set:true}, candidate_chars: 32000, shared_prompt: '', roles: Object.fromEntries(roles.map(role => [
    role, { enabled: true, use_publisher: false, provider: '', endpoint: '', model: '', api_key: 'fixture-saved-key-' + role,
      prompt: '', temperature: 0.2, max_tokens: 2048, timeout_ms: 120000, reasoning_effort: '' }
  ])) }
};
const css = source.match(/const PANEL_CSS = \x60([\s\S]*?)\x60;/)[1];
const renderer = owner('  async function renderSettingsPanel(options)', '\n  async function closeSettingsPanel()');
const loader = owner('  async function loadMemoryPreprocessingPanel()', '\n  async function savePromptEditorPrompt(');
const providers = source.match(/const LLM_PROVIDER_OPTIONS = Object.freeze\((\[[^\n]+?\])\);/)[1];
const version = source.match(/const VERSION = ("[^"]+");/)[1];
const translations = {
  'settings.tab.timeline': '세계선', 'settings.tab.explore': '기억', 'settings.tab.memoryManagement': '기억 관리',
  'settings.tab.extensions': '추가 기능', 'settings.tab.settings': '설정', 'settings.tab.reference': '원작 DB',
  'persona.tab': '페르소나 캡슐', 'settings.tab.lorebook': '로어북'
};
const runtime = [
  'const PANEL_CSS = ' + JSON.stringify(css) + ';',
  'const VERSION = ' + version + ', LLM_PROVIDER_OPTIONS = ' + providers + ';',
  'let view = ' + JSON.stringify(fixture) + '; const translations = ' + JSON.stringify(translations) + ';',
  'const t = key => translations[key] || key;',
  'const diagnosticText = ko => ko;', // Unrelated diagnostics button copy in the main shell.
  'window.fixtureCalls = []; window.fixtureWarnings = [];',
  'let panelOpen = false, _settingsPanelRenderRequestId = 0, _settingsActiveTab = "memory-preprocessing";',
  'const LOG_PREFIX = "UI fixture", _turnHistory = [], _settingsStorageStatus = {}, _lastPrepareTurnBundle = null;',
  'const runtimeState = {}, DEFAULT_SETTINGS = {memoryDeliveryBudgets:{}};',
  'const getSettings = () => DEFAULT_SETTINGS, resolveEffectiveCriticConfig = () => ({});',
  'const debugLog = () => {}, warnLog = (...args) => window.fixtureWarnings.push(args.join(" "));',
  'const escapeAttr = value => String(value ?? "").replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;");',
  // Host container and unrelated main navigation event binding are outside this fixture.
  'const R = {showContainer:async () => {}}, attachSettingsEvents = () => {};',
  // Reasoning preview is covered by settings-pair-smoke with the actual Go
  // endpoint. This fixture isolates prompt, API-key and layout/save behavior.
  'const bindLlmSettingsView = () => async () => {};',
  'async function bridgeFetch(route, options = {}) {',
  '  if (!["/config/memory-preprocessing", "/config/memory-preprocessing/jev-test"].includes(route)) throw new Error("Unexpected route: " + route);',
  '  window.fixtureCalls.push({route,method:options.method || "GET",body:options.body});',
  '  if (route.endsWith("/jev-test")) return {ok:true,model:options.body.model,duration_ms:12,usage:{input_tokens:42}};',
  '  if (options.method === "PUT") {',
  '    if (window.fixtureSaveFails) return null;',
  '    view.settings = {...view.settings, ...structuredClone(options.body)};',
  '    if (view.settings.jev && Object.hasOwn(view.settings.jev,"api_key")) { view.settings.jev.api_key_set = view.settings.jev.api_key !== ""; delete view.settings.jev.api_key; }',
  '    view.shared_prompt = view.settings.shared_prompt || view.default_shared_prompt;',
  '    view.jev_prompts = view.jev_prompts.map(({defaults:p}) => { const saved = (view.settings.jev.prompts || {})[p.key] || {}; return {defaults:p,effective:{...p,instructions:saved.instructions?.trim() ? saved.instructions : p.instructions,criteria:p.criteria.map(c=>({...c,text:saved.criteria?.[c.key]?.trim() ? saved.criteria[c.key] : c.text}))}}; });',
  '  }',
  '  return structuredClone(view);',
  '}',
  renderer, loader, 'renderSettingsPanel();'
].join('\n');

(async () => {
  const browser = await chromium.launch({
    headless: true,
    ...(process.env.ARCHIVE_CENTER_UI_BROWSER ? { executablePath: process.env.ARCHIVE_CENTER_UI_BROWSER } : {})
  });
  try {
    const page = await browser.newPage({ viewport: { width: 1600, height: 1100 } });
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.route('**/*', route => route.abort()); // This fixture must never use the network.
    await page.setContent('<!doctype html><html lang="ko"><meta charset="utf-8"><title>Archive Center 전처리 UI 검증</title><body></body></html>');
    await page.addScriptTag({ content: runtime });
    const root = page.locator('#mo-memory-preprocessing-root');
    await root.getByRole('heading', { name: '전처리 다중 에이전트', exact: true }).waitFor({ timeout: 3000 });
    assert.equal(await root.getByText('Flex는 해당 제공자가 지원하는 모델에서 사용하세요. Claude 직접 연결에는 Flex가 없습니다.', { exact: true }).count(), 0);
    assert.equal(await root.locator('.mo-ma-role').count(), roles.length);
    assert.equal(await root.locator('.mo-ma-role[open]').count(), 1);
    assert.equal(await root.locator('#mo-ma-enabled').isChecked(), false);

    assert.equal(await root.locator('#mo-jev-enabled').count(), 0, 'Jev must be a sibling page');
    assert.equal(await page.getByRole('tab', {name:'Jev',exact:true}).count(), 1);

    const first = roles[0], input = key => root.locator('#mo-ma-' + first + '-' + key);
    const desktopPrompt = await input('prompt').boundingBox();
    assert.ok(desktopPrompt.width > 400 && desktopPrompt.height >= 240, 'desktop prompt editor is too narrow');
    assert.equal(await input('provider').isEnabled(), true);
    assert.equal(await input('settings_source').inputValue(), '');
    assert.notEqual(await input('prompt').evaluate(el => getComputedStyle(el).backgroundColor), 'rgb(255, 255, 255)', 'native unstyled textarea');
    assert.equal(await input('prompt').inputValue(), fixture.default_prompts[first]);
    assert.equal(await input('api_key').inputValue(), fixture.settings.roles[first].api_key);
    assert.equal(await input('api_key').getAttribute('type'), 'password');
    assert.equal(await root.locator('[id$="-clear_api_key"]').count(), 0);
    await page.screenshot({ path: path.join(output, 'preprocessing-desktop.png') });

    // Real DOM controls and the production save handler preserve independent role values.
    await input('settings_source').selectOption('publisher');
    assert.equal(await input('provider').isDisabled(), true);
    await input('settings_source').selectOption('');
    assert.equal(await input('provider').isEnabled(), true);
    assert.equal(await input('inherited').isHidden(), true);
    await input('provider').selectOption('custom');
    assert.equal(await input('llm_gateway_service_tier').isVisible(), true);
    assert.equal(await input('vertex_flex_mode').isHidden(), true);
    await input('llm_gateway_service_tier').selectOption('flex');
    await input('provider').selectOption('vertex');
    assert.equal(await input('llm_gateway_service_tier').isHidden(), true);
    await input('vertex_flex_mode').selectOption('flex_only');
    await input('provider').selectOption('claude');
    assert.equal(await input('llm_gateway_service_tier').isHidden(), true);
    assert.equal(await input('vertex_flex_mode').isHidden(), true);
    await input('provider').selectOption('gemini');
    assert.equal(await input('llm_gateway_service_tier').isVisible(), true);
    assert.equal(await input('llm_gateway_service_tier').inputValue(), 'flex');
    await input('provider').selectOption('custom');
    assert.equal(await input('temperature').isVisible(), true);
    assert.equal(await input('max_tokens').isVisible(), true);
    await input('temperature').fill('0.6');
    await input('max_tokens').fill('3072');
    await input('endpoint').fill('https://fixture.invalid/v1');
    await input('model').fill('independent-role-model');
    await input('prompt').fill('편집한 지침');
    await input('restore').click();
    assert.equal(await input('prompt').inputValue(), fixture.default_prompts[first]);
    const editedPrompt = '이 담당의 지침만 수정합니다. <tag> & "quote"';
    await input('prompt').fill(editedPrompt);
    await input('settings_source').selectOption('publisher');
    assert.equal(await input('temperature').isEnabled(), true, 'connection sharing disabled role generation settings');
    assert.equal(await input('max_tokens').isEnabled(), true);
    await root.locator('#mo-ma-common-settings > summary').click();
    const shared = root.locator('#mo-ma-shared-prompt');
    assert.equal(await shared.inputValue(), fixture.default_shared_prompt);
    await shared.fill('임시 공통 지침');
    await root.locator('#mo-ma-shared-restore').click();
    assert.equal(await shared.inputValue(), fixture.default_shared_prompt);
    const editedShared = '모든 담당에게 적용할 공통 지침입니다. <boundary> & "quote"';
    await shared.fill(editedShared);
    await root.locator('#mo-ma-enabled').check();
    await root.locator('#mo-ma-save').click();
    await root.getByText('저장했습니다. 다음 요청부터 적용됩니다.', { exact: true }).waitFor();
    const saved = await page.evaluate(() => window.fixtureCalls.find(c => c.method === 'PUT').body);
    assert.equal(Object.hasOwn(saved, 'jev'), false, 'preprocessing save must omit Jev settings');
    assert.equal(saved.roles[first].model, 'independent-role-model', 'shared connection erased separately configured model');
    assert.equal(saved.roles[first].prompt, editedPrompt);
    assert.equal(saved.roles[first].temperature, 0.6);
    assert.equal(saved.roles[first].max_tokens, 3072);
    assert.equal(saved.roles[first].llm_gateway_service_tier, 'flex');
    assert.equal(saved.roles[first].vertex_flex_mode, 'flex_only');
    assert.equal(saved.roles[first].api_key, fixture.settings.roles[first].api_key, 'saving hidden connection lost the saved key');
    assert.equal(Object.hasOwn(saved.roles[first], 'clear_api_key'), false);
    assert.equal(saved.enabled, true);
    assert.equal(saved.shared_prompt, editedShared);
    for (const role of roles.slice(1)) {
      assert.equal(saved.roles[role].prompt, '', 'editing one role changed another role prompt override');
      assert.equal(saved.roles[role].use_publisher, false);
      assert.equal(saved.roles[role].llm_gateway_service_tier, '');
      assert.equal(saved.roles[role].vertex_flex_mode, '');
      assert.equal(saved.roles[role].api_key, fixture.settings.roles[role].api_key);
    }
    assert.equal(await input('prompt').inputValue(), editedPrompt, 'prompt escaping changed the value');
    assert.equal(await input('model').inputValue(), 'independent-role-model');
    assert.equal(await input('llm_gateway_service_tier').inputValue(), 'flex');
    assert.equal(await input('vertex_flex_mode').inputValue(), 'flex_only');
    assert.equal(await shared.inputValue(), editedShared);
    assert.equal(await input('api_key').inputValue(), fixture.settings.roles[first].api_key, 'saving emptied the password editor');
    await root.locator('#mo-ma-common-settings > summary').click();
    await root.locator('#mo-ma-shared-restore').click();
    await root.locator('#mo-ma-save').click();
    await root.getByText('저장했습니다. 다음 요청부터 적용됩니다.', { exact: true }).waitFor();
    assert.equal(await page.evaluate(() => window.fixtureCalls.filter(c => c.method === 'PUT').at(-1).body.shared_prompt), '');
    assert.equal(await shared.inputValue(), fixture.default_shared_prompt);
    await root.locator('#mo-ma-common-settings > summary').click();
    await shared.fill(editedShared);
    await page.evaluate(() => { window.fixtureSaveFails = true; });
    await root.locator('#mo-ma-save').click();
    await root.getByText('저장 실패: 백엔드에 설정을 저장하지 못했습니다.', { exact: true }).waitFor();
    assert.equal(await shared.inputValue(), editedShared, 'failed save erased the edited shared prompt');
    assert.equal(await root.locator('#mo-ma-save').isEnabled(), true);
    await page.evaluate(() => { window.fixtureSaveFails = false; });
    await input('settings_source').selectOption('');
    await input('api_key').fill('replacement-fixture-key');
    await root.locator('#mo-ma-save').click();
    await root.getByText('저장했습니다. 다음 요청부터 적용됩니다.', { exact: true }).waitFor();
    await page.evaluate(() => loadMemoryPreprocessingPanel());
    assert.equal(await input('api_key').inputValue(), 'replacement-fixture-key', 'reopening settings lost the edited key');
    await input('api_key').fill('');
    await root.locator('#mo-ma-save').click();
    await root.getByText('저장했습니다. 다음 요청부터 적용됩니다.', { exact: true }).waitFor();
    assert.equal(await input('api_key').inputValue(), '');
    assert.equal(await page.evaluate(() => window.fixtureCalls.filter(c => c.method === 'PUT').at(-1).body.roles.event_recent.api_key), '');
    // Preview ordinary configured providers, without a backend call or credentials.
    const preview = structuredClone(fixture);
    for (const [index, role] of roles.entries()) {
      Object.assign(preview.settings.roles[role], {
        provider: index % 2 ? 'vertex' : 'gemini', model: 'gemini-3.8-flash',
        llm_gateway_service_tier: 'flex', vertex_flex_mode: 'flex_only'
      });
    }
    await page.evaluate(data => { view = structuredClone(data); }, preview);
    await page.evaluate(() => loadMemoryPreprocessingPanel());

    const measurements = [];
    for (const [width, height] of [[1600, 1100], [900, 1000], [390, 844]]) {
      await page.setViewportSize({ width, height });
      await page.locator('.mo-workspace').evaluate(el => { el.scrollTop = 0; });
      const layout = await root.evaluate(el => {
        const rect = el.getBoundingClientRect(), workspace = el.closest('.mo-workspace');
        const overflow = [...el.querySelectorAll('input,textarea,select,summary,fieldset,section,.mo-ma-savebar')]
          .filter(node => node.checkVisibility())
          .filter(node => { const b = node.getBoundingClientRect(); return b.left < rect.left - 1 || b.right > rect.right + 1; })
          .map(node => node.id || node.className || node.tagName);
        return { width: rect.width, overflow, scrollWidth: workspace.scrollWidth, clientWidth: workspace.clientWidth };
      });
      assert.deepEqual(layout.overflow, [], 'controls overflow at width ' + width);
      assert.ok(layout.scrollWidth <= layout.clientWidth + 1, 'workspace scrolls sideways at width ' + width);
      assert.deepEqual(await root.locator('.mo-row input,.mo-row textarea,.mo-row select').evaluateAll(nodes => nodes.filter(el => {
        if (!el.checkVisibility()) return false;
        const box = el.getBoundingClientRect(), row = el.closest('.mo-row').getBoundingClientRect();
        return box.left < row.left - 1 || box.right > row.right + 1;
      }).map(el => el.id)), [], 'a field overlaps another column at width ' + width);
      measurements.push({ viewport: width, ...layout });
      if (width === 390) {
        await page.screenshot({ path: path.join(output, 'preprocessing-mobile.png') });
        await input('prompt').scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(output, 'preprocessing-mobile-prompt.png') });
      }
      await root.locator('.mo-ma-role,.mo-ma-advanced,#mo-ma-common-settings').evaluateAll(nodes => nodes.forEach(node => { node.open = true; }));
      for (const role of roles) await root.locator('#mo-ma-' + role + '-settings_source').selectOption('');
      assert.deepEqual(await root.locator('.mo-row input,.mo-row textarea,.mo-row select').evaluateAll(nodes => nodes.filter(el => {
        if (!el.checkVisibility()) return false;
        const box = el.getBoundingClientRect(), row = el.closest('.mo-row').getBoundingClientRect();
        return box.left < row.left - 1 || box.right > row.right + 1;
      }).map(el => el.id)), [], 'custom connection or advanced field overlaps at width ' + width);
      if (width === 1600) {
        await page.locator('.mo-workspace').evaluate(el => { el.scrollTop = 0; });
        await page.screenshot({ path: path.join(output, 'preprocessing-custom-model.png') });
        await input('temperature').scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(output, 'preprocessing-generation-settings.png') });
        await shared.scrollIntoViewIfNeeded();
        await page.screenshot({ path: path.join(output, 'preprocessing-common-prompt.png') });
      }
      await page.evaluate(() => loadMemoryPreprocessingPanel());
    }
    await page.evaluate(async () => { await bridgeFetch('/config/memory-preprocessing', {method:'PUT',body:{enabled:true}}); _settingsActiveTab = 'jev'; await renderSettingsPanel({ recompose: true }); });
    const jevRoot = page.locator('#mo-jev-root');
    await jevRoot.getByRole('heading', {name:'Jev',exact:true}).waitFor();
    assert.equal(await page.getByRole('tab',{name:'Jev',exact:true}).getAttribute('aria-selected'),'true');
    assert.equal(await page.locator('#mo-memory-preprocessing-root').count(),0);
    assert.match(await jevRoot.textContent(),/전처리를 꺼도 Jev만 사용할 수 있습니다/);
    assert.equal(await jevRoot.locator('#mo-jev-api_key').inputValue(),'');
    assert.equal(await jevRoot.locator('#mo-jev-api_key').getAttribute('type'),'password');
    assert.match(await jevRoot.locator('#mo-jev-mode').textContent(),/기존 AI 전처리/);
    await jevRoot.locator('#mo-jev-enabled').check();
    assert.match(await jevRoot.locator('#mo-jev-mode').textContent(),/Jev 검증/);
    await jevRoot.locator('#mo-jev-api_key').fill('fixture-new-jev-key');
    const savesBefore = await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').length);
    await jevRoot.locator('#mo-jev-test').click();
    assert.match(await jevRoot.locator('#mo-jev-test-result').textContent(),/연결 성공/);
    assert.equal(await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').length),savesBefore);
    const preprocessingBefore = await page.evaluate(()=>{const {jev,...other}=view.settings;return other;});
    await jevRoot.locator('#mo-jev-save').click();
    await jevRoot.getByText('Jev 설정을 저장했습니다. 다음 요청부터 적용됩니다.',{exact:true}).waitFor();
    const jevSaved = await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').at(-1).body);
    assert.deepEqual(Object.keys(jevSaved),['jev']);
    assert.equal(jevSaved.jev.api_key,'fixture-new-jev-key');
    assert.equal(await jevRoot.locator('#mo-jev-api_key').inputValue(),'');
    assert.deepEqual(await page.evaluate(()=>{const {jev,...other}=view.settings;return other;}),preprocessingBefore);
    await page.evaluate(async()=>{await bridgeFetch('/config/memory-preprocessing',{method:'PUT',body:{enabled:false}});await loadJevPanel();});
    assert.match(await jevRoot.locator('#mo-jev-mode').textContent(),/Jev 주력/);
    await jevRoot.locator('#mo-jev-enabled').uncheck();
    assert.match(await jevRoot.locator('#mo-jev-mode').textContent(),/Go 기본/);
    await jevRoot.locator('#mo-jev-enabled').check();
    await jevRoot.locator('#mo-jev-clear-key').check();
    await jevRoot.locator('#mo-jev-save').click();
    await jevRoot.getByText('Jev 설정을 저장했습니다. 다음 요청부터 적용됩니다.',{exact:true}).waitFor();
    assert.equal(await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').at(-1).body.jev.api_key),'');
    assert.equal(await page.evaluate(()=>view.settings.enabled),false);
    assert.equal(await jevRoot.locator('.mo-jev-prompt').count(),jevPrompts.length);
    const promptInput = (p,key) => jevRoot.locator('#mo-jev-'+p.key+'-'+key);
    for(const p of jevPrompts){
      await jevRoot.locator('#mo-jev-prompt-'+p.key+' > summary').click();
      assert.equal(await promptInput(p,'instructions').inputValue(),p.instructions);
      await promptInput(p,'instructions').fill('편집 '+p.key+': {item} / {role_focus}\n</textarea><b> & "quote" 100%');
      await promptInput(p,'criterion-'+p.criteria[0].key).fill('기준 '+p.key+'\n<exact> & "quote"');
    }
    await page.evaluate(()=>{window.fixtureSaveFails=true;});
    await jevRoot.locator('#mo-jev-save').click();
    await jevRoot.getByText('저장 실패: 백엔드에 설정을 저장하지 못했습니다.',{exact:true}).waitFor();
    assert.match(await promptInput(jevPrompts[0],'instructions').inputValue(),/^편집 rank/);
    assert.equal(await jevRoot.locator('#mo-jev-save').isEnabled(),true);
    await page.evaluate(()=>{window.fixtureSaveFails=false;});
    await jevRoot.locator('#mo-jev-save').click();
    await jevRoot.getByText('Jev 설정을 저장했습니다. 다음 요청부터 적용됩니다.',{exact:true}).waitFor();
    const promptSaved = await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').at(-1).body);
    assert.deepEqual(Object.keys(promptSaved),['jev']);
    for(const p of jevPrompts){
      assert.equal(await promptInput(p,'instructions').inputValue(),promptSaved.jev.prompts[p.key].instructions);
      assert.equal(await promptInput(p,'criterion-'+p.criteria[0].key).inputValue(),promptSaved.jev.prompts[p.key].criteria[p.criteria[0].key]);
    }
    await jevRoot.locator('#mo-jev-test').click();
    assert.equal(await page.evaluate(()=>Object.hasOwn(window.fixtureCalls.filter(c=>c.method==='POST').at(-1).body,'prompts')),false,'connection test sent prompt drafts');
    const restorePrompt = jevPrompts.find(p=>p.key==='time');
    await jevRoot.locator('#mo-jev-prompt-time > summary').click();
    const savesBeforeRestore = await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').length);
    await jevRoot.locator('#mo-jev-time-restore').click();
    assert.equal(await page.evaluate(()=>window.fixtureCalls.filter(c=>c.method==='PUT').length),savesBeforeRestore,'restore saved without explicit save');
    assert.equal(await promptInput(restorePrompt,'instructions').inputValue(),restorePrompt.instructions);
    for(const c of restorePrompt.criteria) assert.equal(await promptInput(restorePrompt,'criterion-'+c.key).inputValue(),c.text);
    await jevRoot.locator('#mo-jev-save').click();
    await jevRoot.getByText('Jev 설정을 저장했습니다. 다음 요청부터 적용됩니다.',{exact:true}).waitFor();
    await page.evaluate(()=>loadJevPanel());
    assert.equal(await promptInput(restorePrompt,'instructions').inputValue(),restorePrompt.instructions);
    assert.equal(await promptInput(jevPrompts[0],'instructions').inputValue(),promptSaved.jev.prompts.rank.instructions,'restoring one question changed another');
    for(const width of [1600,900,390]){
      await page.setViewportSize({width,height:1100});
      await page.locator('.mo-workspace').evaluate(el=>{el.scrollTop=0;});
      const layout = await jevRoot.evaluate(el=>{
        const root=el.getBoundingClientRect(),workspace=el.closest('.mo-workspace');
        const overflow=[...el.querySelectorAll('input,textarea,summary,button,section')].filter(n=>n.checkVisibility()).filter(n=>{const r=n.getBoundingClientRect();return r.left<root.left-1||r.right>root.right+1;}).map(n=>n.id||n.tagName);
        return {overflow,scrollWidth:workspace.scrollWidth,clientWidth:workspace.clientWidth};
      });
      assert.deepEqual(layout.overflow,[], 'Jev layout overflow at '+width);
      assert.ok(layout.scrollWidth<=layout.clientWidth+1);
      measurements.push({panel:'jev',viewport:width,...layout});
      await page.screenshot({path:path.join(output,'jev-'+width+'.png')});
      await jevRoot.locator('.mo-jev-prompt').evaluateAll(nodes=>nodes.forEach(node=>{node.open=true;}));
      const promptOverflow = await jevRoot.locator('textarea').evaluateAll(nodes=>nodes.filter(el=>{
        const b=el.getBoundingClientRect(),r=el.closest('section').getBoundingClientRect();return b.left<r.left-1||b.right>r.right+1;
      }).map(el=>el.id));
      assert.deepEqual(promptOverflow,[],'Jev prompt columns overlap at '+width);
      await promptInput(jevPrompts[0],'instructions').scrollIntoViewIfNeeded();
      await page.screenshot({path:path.join(output,'jev-prompts-'+width+'.png')});
    }
    await jevRoot.locator('#mo-jev-open-preprocessing').click();
    await root.getByRole('heading',{name:'전처리 다중 에이전트',exact:true}).waitFor();
    assert.equal(await root.locator('#mo-ma-enabled').isChecked(),false);
    assert.equal(await root.locator('#mo-jev-enabled').count(),0);

    assert.deepEqual(errors, []);
    assert.deepEqual(await page.evaluate(() => window.fixtureWarnings), []);
    const receipt = { source: process.env.ARCHIVE_CENTER_UI_SOURCE || path.join(repo, 'Archive Center.js'), measurements,
      tested: ['five Jev prompt/criteria editors, save/reopen, escaped text, per-question restore and failed-save preservation', 'independent sibling Jev page and four-mode display', 'Jev-only save, masked key and explicit clearing without changing preprocessing', 'Jev synthetic connection test without settings mutation or prompt drafts', 'responsive real DOM layout', 'scoped dark controls', 'independent role prompt editing and restore',
        'connection inheritance toggle', 'disabled input values preserved on save', 'unchanged other-role prompts',
        'editable shared prompt and default restore', 'failed save preserves edits',
        'provider-specific Flex controls and hidden mode preservation', 'visible per-role temperature and max tokens',
        'saved password remains after save and reopen; editing and explicit clearing without deletion checkbox'],
      boundary: 'Production UI owners with mocked config transport; no installed RisuAI or real backend/provider.' };
    fs.writeFileSync(path.join(output, 'receipt.json'), JSON.stringify(receipt, null, 2));
    console.log(JSON.stringify(receipt, null, 2));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
