// Runs the complete packaged JS against the packaged Go executable.
// Only RisuAI host capabilities are supplied by the browser harness. Settings
// rendering, binding, sanitization, persistence, transport and Go routes are real.
// The isolated backend uses its existing shadow/noop mode; no user DB or LLM.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { spawn } = require('node:child_process');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const packageRoot = path.resolve(process.argv[2]);
const evidenceRoot = path.resolve(process.argv[3]);
const pluginBytes=fs.readFileSync(path.join(packageRoot,'Archive Center.js'));
const packageVersion=pluginBytes.toString('utf8').match(/const VERSION = "([^"]+)"/)[1];
fs.mkdirSync(evidenceRoot, { recursive: true });
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const close = server => new Promise(resolve => server.close(resolve));

(async () => {
  const reserve = http.createServer(); await listen(reserve);
  const backendURL = `http://127.0.0.1:${reserve.address().port}`; await close(reserve);
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !/^(AC_|PROJECT_|CHROMA|MARIADB)/i.test(k)));
  Object.assign(env, {AC_BIND_ADDR: backendURL.slice(7), AC_MODE:'shadow', AC_STORE_MODE:'noop', AC_RUNTIME_PROFILE:'core_lite', AC_VECTOR_MODE:'off', AC_BUILD_VERSION:packageVersion, AC_UPDATE_ENABLED:'false', AC_LOG_DIR:path.join(evidenceRoot,'diagnostic-logs'), ARCHIVE_CENTER_DATA_DIR:path.join(evidenceRoot,'isolated-data')});
  const backendLog = fs.openSync(path.join(evidenceRoot,'backend.log'),'w');
  const backend = spawn(path.join(packageRoot,'bin/archive-center-go.exe'),[],{env,cwd:evidenceRoot,windowsHide:true,stdio:['ignore',backendLog,backendLog]});
  let browser, host, page; const errors=[]; const warnings=[];
  try {
    let health;
    for(let i=0;i<100;i++) {
      if(backend.exitCode!==null) throw new Error('Isolated backend exited: '+backend.exitCode);
      try { const r=await fetch(backendURL+'/health'); if(r.ok){health=await r.json();break;} } catch {}
      await new Promise(r=>setTimeout(r,100));
    }
    assert.ok(health,'isolated backend ready');
    const seed={enabled:false,dbEnabled:false,turnWorkflowHUDEnabled:false,bridgeUrl:backendURL,uiLanguage:'ko',pluginMainProvider:'openai',pluginMainModel:'gpt-5.6-luna',pluginMainApiKey:'synthetic-publisher-key',pluginMainReasoningEffort:'max',pluginMainTemperature:0,pluginMainMaxCompletionTokens:4096,subLlmProvider:'claude',subLlmModel:'claude-sonnet-4-5',subLlmApiKey:'synthetic-critic-key',subLlmReasoningBudgetTokens:2048,subLlmMaxCompletionTokens:3072};
    const bootstrap=`
      const key='risu_memory_orchestrator_settings';
      if(!localStorage.getItem('host-local:'+key)) localStorage.setItem('host-local:'+key,JSON.stringify(${JSON.stringify(seed)}));
      window.transport=[]; window.hostStorageWrites=[]; window.registeredHooks=[];
      const storage=prefix=>({getItem:async k=>localStorage.getItem(prefix+k),setItem:async(k,v)=>{localStorage.setItem(prefix+k,v);window.hostStorageWrites.push(k);},removeItem:async k=>localStorage.removeItem(prefix+k)});
      window.Risuai={
        getLocalPluginStorage:async()=>storage('host-local:'),pluginStorage:storage('host-plugin:'),
        nativeFetch:async(url,options)=>{
          if(!url.startsWith(${JSON.stringify(backendURL+'/')})) throw new Error('Unexpected destination');
          if(/\\/(proxy|prepare-turn|process-turn)/.test(url)) throw new Error('LLM or turn request is outside settings verification');
          const response=await fetch(url,options);let result=null;try{result=await response.clone().json();}catch{}
          window.transport.push({path:new URL(url).pathname,method:options.method,status:response.status,body:options.body?JSON.parse(options.body):null,result});return response;
        },
        registerSetting:async(name,callback)=>{window.openSettings=callback;},
        registerButton:async()=>{},showContainer:async()=>{},hideContainer:async()=>{},
        addRisuReplacer:async(...args)=>window.registeredHooks.push(args[0]),
        addRisuScriptHandler:async(...args)=>window.registeredHooks.push(args[0]),
        getCharacter:async()=>null,getCurrentChatIndex:async()=>0,getArgument:async()=>null
      };
    `;
    host=http.createServer((req,res)=>{
      if(req.url==='/plugin.js'){res.setHeader('Content-Type','text/javascript; charset=utf-8');res.end(pluginBytes);return;}
      res.setHeader('Content-Type','text/html; charset=utf-8');res.end('<!doctype html><meta charset="utf-8"><body><script>'+bootstrap+'</script><script src="/plugin.js"></script></body>');
    }); await listen(host);
    browser=await chromium.launch({headless:true,executablePath:process.env.ARCHIVE_CENTER_UI_BROWSER});
    const context=await browser.newContext({viewport:{width:1440,height:1050}});
    page=await context.newPage();
    page.on('pageerror',e=>errors.push(e.message));
    page.on('console',m=>{if(m.type()==='warning'||m.type()==='error')warnings.push(m.text());});
    const open=async()=>{await page.waitForFunction(()=>typeof window.openSettings==='function');await page.evaluate(()=>window.openSettings());await page.locator('button[data-tab="settings"]').click();await page.locator('#mo-pluginMainProvider').waitFor({state:'visible'});await page.waitForFunction(()=>window.transport.filter(x=>x.path==='/config/view-model'&&x.status===200).length>=2);};
    await page.goto(`http://127.0.0.1:${host.address().port}`); await open();
    assert.equal(await page.locator('#mo-pluginMainReasoningEffort').inputValue(),'max');
    assert.equal(await page.locator('#mo-subLlmReasoningBudgetTokens').inputValue(),'2048');
    assert.equal(await page.evaluate(()=>window.hostStorageWrites.filter(k=>k!=='archive_center_host_diagnostics_v1').length),0,'read-only view wrote settings/recovery storage');
    await page.locator('#mo-pluginMainModel').fill('gpt-5.2');
    await page.waitForFunction(()=>window.transport.some(x=>x.path==='/config/view-model'&&x.body.model==='gpt-5.2'));
    await page.locator('#mo-pluginMainTemperature').fill('0.15');
    await page.locator('#mo-subLlmReasoningBudgetTokens').fill('4096');
    await page.locator('#mo-pluginMainEndpoint').fill('');
    await page.locator('#mo-save-btn').click();
    await page.waitForFunction(()=>window.transport.some(x=>x.path==='/config/update'&&x.status===200&&x.body.mainModel==='gpt-5.2'&&x.body.mainTemperature===0.15));
    const sync=await page.evaluate(()=>window.transport.filter(x=>x.path==='/config/update'&&x.body.mainModel==='gpt-5.2'&&x.body.mainTemperature===0.15).at(-1));
    assert.equal(sync.result.runtime_config_trace.synced,true);
    assert.equal(sync.body.mainModel,'gpt-5.2');assert.equal(sync.body.mainTemperature,0.15);
    assert.equal(sync.body.criticReasoningBudgetTokens,4096);
    assert.equal(sync.body.criticApiKey,'synthetic-critic-key');
    assert.equal(sync.body.mainApiKey,'synthetic-publisher-key');
    const saved=await page.evaluate(()=>localStorage.getItem('host-local:risu_memory_orchestrator_settings'));
    const parsed=JSON.parse(saved);assert.equal(parsed.pluginMainModel,'gpt-5.2');assert.equal(parsed.subLlmReasoningBudgetTokens,4096);
    await page.reload(); await open();
    assert.equal(await page.locator('#mo-pluginMainModel').inputValue(),'gpt-5.2');
    assert.equal(await page.locator('#mo-pluginMainTemperature').inputValue(),'0.15');
    assert.equal(await page.locator('#mo-subLlmReasoningBudgetTokens').inputValue(),'4096');
    assert.equal(await page.locator('#mo-pluginMainEndpoint').inputValue(),'');
    assert.equal(await page.locator('#mo-pluginMainApiKey').inputValue(),'synthetic-publisher-key');
    assert.equal(await page.locator('#mo-subLlmApiKey').inputValue(),'synthetic-critic-key');
    assert.equal(await page.evaluate(()=>localStorage.getItem('host-local:risu_memory_orchestrator_settings')),saved,'reopen changed persistence');
    const views=await page.evaluate(()=>window.transport.filter(x=>x.path==='/config/view-model'));
    for(const view of views){assert.equal(view.status,200);assert.equal(view.result.contract_version,'llm_settings_view.v1');assert.ok(!JSON.stringify(view.body).includes('synthetic-'));}
    assert.deepEqual(errors,[]);
    await page.screenshot({path:path.join(evidenceRoot,'settings-reopened.png')});
    if(process.argv.includes('--preprocessing-reasoning')) {
      const openPreprocessing=async()=>{
        await page.locator('button[data-tab="reference"]').first().click();
        await page.locator('button[data-tab-jump="memory-preprocessing"]').click();
        await page.locator('#mo-ma-save').waitFor();
      };
      await openPreprocessing();
      const roles=['event_recent','character_objective','subjective_relationship','world_state','unresolved_goal'];
      const models=['glm-5.3','kimi-k2.6','kimi-k2.7-code','deepseek-v4.1-flash','gemini-3.8-flash'];
      const choices=[['','low','high','max'],['','enable','disable'],[],['','none','low','high','max'],['','none','low','medium','high']];
      for(let i=0;i<roles.length;i++) {
        const p='#mo-ma-'+roles[i]+'-';
        await page.locator(p+'enabled').evaluate(el=>{el.closest('details.mo-ma-role').open=true;});
        await page.locator(p+'provider').selectOption(i===4?'gemini':'llmgateway');
        await page.locator(p+'model').fill(models[i]);
        await page.waitForFunction(({role,model})=>window.transport.some(x=>x.path==='/config/view-model'&&x.body.purpose==='memory_preprocessing'&&x.body.model===model),{role:roles[i],model:models[i]});
        if(i===2) {
          await page.waitForFunction(sel=>document.querySelector(sel).style.display==='none',p+'reasoning_effort-row');
          assert.equal(await page.locator(p+'temperature').isDisabled(),true);
        } else {
          await page.waitForFunction(({sel,options})=>JSON.stringify(Array.from(document.querySelector(sel).options,o=>o.value))===JSON.stringify(options),{sel:p+'reasoning_effort',options:choices[i]});
          await page.locator(p+'reasoning_effort').selectOption(['low','disable','','max','medium'][i]);
        }
      }
      await page.locator('#mo-ma-save').click();
      await page.waitForFunction(()=>document.querySelector('#mo-ma-status')?.textContent.includes('저장했습니다'));
      await page.reload();await open();await openPreprocessing();
      for(let i=0;i<roles.length;i++) {
        assert.equal(await page.locator('#mo-ma-'+roles[i]+'-model').inputValue(),models[i]);
        if(i!==2) assert.equal(await page.locator('#mo-ma-'+roles[i]+'-reasoning_effort').inputValue(),['low','disable','','max','medium'][i]);
      }
      // Saved Publisher is GPT-5.2. Sharing the connection resolves that model
      // in Go despite the independent GLM field still containing glm-5.3.
      await page.locator('#mo-ma-event_recent-enabled').evaluate(el=>{el.closest('details.mo-ma-role').open=true;});
      await page.locator('#mo-ma-event_recent-settings_source').selectOption('publisher');
      await page.waitForFunction(()=>Array.from(document.querySelector('#mo-ma-event_recent-reasoning_effort').options,o=>o.value).includes('xhigh'));
      await page.locator('#mo-ma-event_recent-reasoning_effort').selectOption('');
      await page.locator('#mo-ma-save').click();
      await page.waitForFunction(()=>document.querySelector('#mo-ma-status')?.textContent.includes('저장했습니다'));
      assert.equal(await page.locator('#mo-ma-event_recent-reasoning_effort').inputValue(),'');
      await page.screenshot({path:path.join(evidenceRoot,'preprocessing-reasoning-reopened.png'),fullPage:true});
      const setSource=async(role,source)=>{
        await page.locator('#mo-ma-'+role+'-enabled').evaluate(el=>{el.closest('details.mo-ma-role').open=true;});
        await page.locator('#mo-ma-'+role+'-settings_source').selectOption(source);
      };
      await setSource('event_recent','');
      await page.locator('#mo-ma-event_recent-model').fill('gpt-5.6-luna');
      await page.locator('#mo-ma-event_recent-temperature').fill('0.25');
      await setSource('character_objective','');
      await page.locator('#mo-ma-character_objective-provider').selectOption('ollama');
      await page.locator('#mo-ma-character_objective-model').fill('deepseek-v4.1-flash:cloud');
      await page.waitForFunction(()=>window.transport.some(x=>x.path==='/config/view-model'&&x.body.model==='deepseek-v4.1-flash:cloud'));
      await page.locator('#mo-ma-character_objective-reasoning_effort').selectOption('');
      const localPrompt=await page.locator('#mo-ma-subjective_relationship-prompt').inputValue();
      await setSource('subjective_relationship','character_objective');
      await setSource('world_state','character_objective');
      await setSource('unresolved_goal','event_recent');
      assert.equal(await page.locator('#mo-ma-subjective_relationship-generation').isVisible(),false);
      assert.equal(await page.locator('#mo-ma-subjective_relationship-connection').isVisible(),false);
      assert.equal(await page.locator('#mo-ma-subjective_relationship-prompt').inputValue(),localPrompt);
      await page.locator('#mo-ma-save').click();
      await page.waitForFunction(()=>document.querySelector('#mo-ma-status')?.textContent.includes('저장했습니다'));
      await page.reload();await open();await openPreprocessing();
      for(const [role,source] of [['subjective_relationship','character_objective'],['world_state','character_objective'],['unresolved_goal','event_recent']]) {
        assert.equal(await page.locator('#mo-ma-'+role+'-settings_source').inputValue(),source);
      }
      const sharedView=await page.evaluate(()=>window.transport.filter(x=>x.path==='/config/memory-preprocessing'&&x.method==='GET').at(-1).result);
      assert.equal(sharedView.role_connections.subjective_relationship.model,'deepseek-v4.1-flash:cloud');
      assert.equal(sharedView.role_connections.world_state.source_role,'character_objective');
      assert.equal(sharedView.role_connections.unresolved_goal.model,'gpt-5.6-luna');
      assert.equal(await page.locator('#mo-ma-subjective_relationship-prompt').inputValue(),localPrompt);
      await setSource('character_objective','');
      await page.locator('#mo-ma-character_objective-model').fill('deepseek-v4-pro:cloud');
      await page.locator('#mo-ma-save').click();
      await page.waitForFunction(()=>document.querySelector('#mo-ma-status')?.textContent.includes('저장했습니다'));
      const updated=await page.evaluate(()=>window.transport.filter(x=>x.path==='/config/memory-preprocessing').at(-1).result);
      assert.equal(updated.role_connections.subjective_relationship.model,'deepseek-v4-pro:cloud');
      assert.equal(updated.role_connections.world_state.model,'deepseek-v4-pro:cloud');
      await setSource('subjective_relationship','');
      assert.equal(await page.locator('#mo-ma-subjective_relationship-model').inputValue(),models[2],'original independent draft overwritten by sharing');
      await page.screenshot({path:path.join(evidenceRoot,'preprocessing-sharing-reopened.png'),fullPage:true});
      assert.deepEqual(errors,[]);
    }
    if(process.argv.includes('--critic-only-config')) {
      const saveAndCheck=async(mode,missing)=>{
        const start=await page.evaluate(()=>window.transport.length);
        await page.locator('#mo-pluginMainApplyMode').selectOption(mode);
        await page.locator('#mo-save-btn').click();
        await page.waitForFunction(({start,mode})=>window.transport.slice(start).some(x=>x.path==='/config/update'&&x.status===200&&x.body.publisherApplyMode===mode),{start,mode});
        const ack=await page.evaluate(({start})=>window.transport.slice(start).filter(x=>x.path==='/config/update').at(-1).result,{start});
        assert.equal(ack.runtime_config_trace.synced,true);
        assert.equal(ack.runtime_config_trace.main.configured,false);
        assert.equal(ack.runtime_config_trace.critic.configured,true);
        assert.equal(ack.runtime_config_trace.main.required_for_sync,missing);
        assert.equal(ack.runtime_config_trace.supervisor.required_for_sync,missing);
        await page.waitForFunction(()=>document.querySelector('#mo-save-btn')?.disabled===false);
        const reportStart=await page.evaluate(()=>window.transport.length);
        await page.locator('#mo-diagnostics-btn').click();
        await page.waitForFunction(()=>document.querySelector('#mo-diagnostics-modal [data-download]')?.disabled===false);
        const report=await page.evaluate(({reportStart})=>window.transport.slice(reportStart).filter(x=>x.path==='/diagnostics/report').at(-1).result,{reportStart});
        assert.ok(!JSON.stringify(report).includes('synthetic-publisher-key'));
        await page.locator('#mo-diagnostics-modal [data-close]').click();
      };
      await page.locator('#mo-pluginMainModel').fill('');
      const warningStart=warnings.length;
      await saveAndCheck('off',false);
      assert.ok(!warnings.slice(warningStart).some(x=>x.includes('runtime_config_incomplete:main')),'disabled Publisher produced sync warning');
      await page.reload();await open();
      assert.equal(await page.locator('#mo-pluginMainApplyMode').inputValue(),'off');
      assert.equal(await page.locator('#mo-pluginMainModel').inputValue(),'');
      assert.equal(await page.locator('#mo-pluginMainApiKey').inputValue(),'synthetic-publisher-key');
      assert.ok(!warnings.slice(warningStart).some(x=>x.includes('runtime_config_incomplete:main')),'restart of UI produced sync warning');
      const enabledWarningStart=warnings.length;
      await saveAndCheck('shadow',true);
      assert.ok(warnings.slice(enabledWarningStart).some(x=>x.includes('runtime_config_incomplete:main[model];supervisor[model]')),'enabled incomplete Publisher warning disappeared');
      const disabledAgainStart=warnings.length;
      await saveAndCheck('off',false);
      assert.ok(!warnings.slice(disabledAgainStart).some(x=>x.includes('runtime_config_incomplete:main')));
      console.log('PASS: disabled Publisher with retained key/blank model; Critic ready; save/reload; enabled warning restored; keys retained.');
    }
    if(process.argv.includes('--diagnostics')) {
      const missing=await fetch(backendURL+'/fixture-missing-route');assert.equal(missing.status,404);
      const savedBeforeLogging=await page.evaluate(()=>localStorage.getItem('host-local:risu_memory_orchestrator_settings'));
      await page.locator('#mo-diagnostics-btn').click();
      const toggle=page.locator('#mo-diagnostics-modal [data-logging-toggle]');
      await page.waitForFunction(()=>document.querySelector('#mo-diagnostics-modal [data-logging-toggle]')?.disabled===false);
      assert.ok((await toggle.innerText()).includes('켜기'));
      await toggle.click();
      await page.waitForFunction(()=>document.querySelector('#mo-diagnostics-modal [data-logging-toggle]').textContent.includes('끄기'));
      assert.equal((await (await fetch(backendURL+'/diagnostics/report')).json()).logging.level,'debug');
      await page.locator('#mo-diagnostics-modal [data-close]').click();
      await page.locator('#mo-diagnostics-btn').click();
      await page.waitForFunction(()=>document.querySelector('#mo-diagnostics-modal [data-logging-toggle]')?.textContent.includes('끄기'));
      await page.screenshot({path:path.join(evidenceRoot,'detailed-logging-reopened.png')});
      // A request after opening the dialog must appear in the downloaded report.
      await fetch(backendURL+'/fixture-after-dialog-open');
      let pending=page.waitForEvent('download');await page.locator('#mo-diagnostics-modal [data-download]').click();
      let download=await pending;const freshPath=path.join(evidenceRoot,'detailed-report.json');await download.saveAs(freshPath);
      const fresh=JSON.parse(fs.readFileSync(freshPath,'utf8'));
      assert.equal(fresh.backend.logging.level,'debug');
      assert.ok(JSON.stringify(fresh).includes('/fixture-after-dialog-open'));
      assert.ok(JSON.stringify(fresh).includes('http request started'));
      await toggle.click();
      await page.waitForFunction(()=>document.querySelector('#mo-diagnostics-modal [data-logging-toggle]').textContent.includes('켜기'));
      assert.equal((await (await fetch(backendURL+'/diagnostics/report')).json()).logging.level,'info');
      assert.equal(await page.evaluate(()=>localStorage.getItem('host-local:risu_memory_orchestrator_settings')),savedBeforeLogging,'logging wrote saved model settings');
      await page.locator('#mo-diagnostics-modal [data-close]').click();
      const downloadReport=async(name)=>{
        await page.locator('#mo-diagnostics-btn').click();
        const button=page.locator('#mo-diagnostics-modal [data-download]');
        await button.waitFor();await page.waitForFunction(()=>!document.querySelector('#mo-diagnostics-modal [data-download]').disabled);
        const pending=page.waitForEvent('download');await button.click();const download=await pending;
        const destination=path.join(evidenceRoot,name);await download.saveAs(destination);
        const report=JSON.parse(fs.readFileSync(destination,'utf8'));
        assert.ok(!JSON.stringify(report).includes('synthetic-publisher-key'));assert.ok(!JSON.stringify(report).includes('synthetic-critic-key'));
        if(!report.backend_report_available) assert.equal(await page.locator('#mo-diagnostics-modal [data-logging-toggle]').isDisabled(),true);
        await page.screenshot({path:path.join(evidenceRoot,name+'.png')});
        await page.locator('#mo-diagnostics-modal [data-close]').click();return report;
      };
      const online=await downloadReport('online-report.json');assert.equal(online.backend_report_available,true);
      assert.ok(JSON.stringify(online).includes('/fixture-missing-route'));
      const exited=new Promise(resolve=>backend.once('exit',resolve));backend.kill();await exited;
      const offline=await downloadReport('offline-report.json');assert.equal(offline.backend_report_available,false);assert.ok(offline.host_errors.some(e=>e.stage==='bridge'));
      const persisted=await page.evaluate(()=>JSON.parse(localStorage.getItem('host-local:archive_center_host_diagnostics_v1')));
      assert.ok(persisted.some(e=>e.stage==='bridge'),'offline errors did not persist');
      await page.reload();await page.waitForFunction(()=>typeof window.openSettings==='function');
      const restored=await page.evaluate(()=>JSON.parse(localStorage.getItem('host-local:archive_center_host_diagnostics_v1')));
      assert.ok(restored.some(e=>e.at===persisted.at(-1).at),'offline error lost across reload');
      assert.deepEqual(errors,[]);
      console.log('PASS: diagnostic toggle/reopen, fresh report, unchanged saved settings, online/offline downloads, key redaction, persisted host errors after reload.');
    }
    const checks=['read-only draft','provider/model view','role-specific save','real config/update ack','page reload persistent readback','blank endpoint','separate keys'];
    if(process.argv.includes('--critic-only-config')) checks.push('Publisher off partial config saves/reopens without false sync failure','Publisher on still reports missing model','Critic readiness and stored keys preserved');
    if(process.argv.includes('--preprocessing-reasoning')) checks.push('five preprocessing role model controls','preprocessing effort save/reopen','Kimi fixed sampling','Publisher connection inheritance and blank effort','two peer connection groups save/reopen','source change follows all peers','own prompt and independent draft preserved');
    fs.writeFileSync(path.join(evidenceRoot,'result.json'),JSON.stringify({scope:'packaged full JS + real Go executable; isolated Risu host API, no live Risu/provider/user DB',health,checks,sync,views,errors,warnings},null,2));
    console.log('PASS: full packaged JS + real packaged Go, query/save/page-reload/reopen; isolated host API, no user data or LLM.');
  } catch(error) {
    if(page){fs.writeFileSync(path.join(evidenceRoot,'failure.json'),JSON.stringify({error:String(error),errors,warnings,transport:await page.evaluate(()=>window.transport),text:await page.locator('body').innerText()},null,2));await page.screenshot({path:path.join(evidenceRoot,'failure.png')});}
    throw error;
  } finally {if(browser)await browser.close();if(host)await close(host);backend.kill();fs.closeSync(backendLog);}
})().catch(e=>{console.error(e);process.exitCode=1;});
