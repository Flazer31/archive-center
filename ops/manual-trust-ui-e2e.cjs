// Browser -> production Explorer handlers -> real Go HTTP -> disposable MariaDB.
// RisuAI's outer plugin host and remote AI/vector services are external boundaries.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const source = fs.readFileSync(process.env.ARCHIVE_CENTER_UI_SOURCE || path.join(__dirname, '../Archive Center.js'), 'utf8').replace(/\r\n/g, '\n');
const [base, sid, output] = process.argv.slice(2);
assert(base && sid && output);
function fn(name) {
  const match = source.match(new RegExp('  (?:async )?function '+name+'\\([^\\n]*\\) \\{[\\s\\S]*?\\n  \\}'));
  assert(match, name); return match[0];
}
function between(a,b) {
  const start=source.indexOf(a),end=source.indexOf(b,start);
  assert(start>=0 && end>start,a);return source.slice(start,end);
}
const functions=['explorerIsEditing','explorerStartEdit','explorerCancelEdit','explorerFetchTrust','explorerPatchTrust','explorerPatchStorylineItem','explorerPatchWorldRuleItem','explorerPatchHookItem','explorerFetchWorldGraph','explorerFetchEntities','isLocationWorldRuleScope','renderExplorerTrust','renderExplorerEntities'];
const locationScopes=source.match(/  const WORLD_RULE_LOCATION_SCOPES = [^\n]+;/)[0];
const bindings=between('      document.querySelectorAll(".mo-trust-btn[data-trust-model]")','      // Trust tab — 항목 삭제 버튼')+
 between('      document.querySelectorAll(".mo-trust-reload")','      // I-4d: World tab')+
 between('      document.querySelectorAll("[data-ent-subtab]")','      //',source.indexOf('data-ent-subtab'));
// The save binding itself collects actual DOM form fields and asks for confirmation.
const saveBindings=between('      document.querySelectorAll(".mo-ed-save-btn")','      // ── Session routing reset');
const cancelBindings=between('      document.querySelectorAll(".mo-ed-cancel-btn")','      document.querySelectorAll(".mo-ed-save-btn")');
const runtime=`
${locationScopes}
const sid=${JSON.stringify(sid)};
const _explorer={selectedSessionId:sid,activeTab:'trust',expandedItems:new Set(),editingItem:null,editFields:{},editStatus:'',editError:'',trust:{storylines:[],worldRules:[],hooks:[],patchStatus:{}},entities:{characters:[],locations:[],items:[],memoryBundles:[],memoryItems:[]},worldGraph:{_scopeNames:{},rules:[],allRules:[],scopeChain:['root']}};
const explorerSessionId=()=>sid,getRequestTimeoutSettingMs=()=>15000;
const t=x=>x==='explorer.edit.userCorrected'?'사용자에 의해 수정됨.':x;
const escapeAttr=x=>String(x??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
const formatDisplayEntityLabel=x=>x,_deduplicateCharacters=x=>x;
const renderExplorerBatchDeleteToolbar=()=>'',renderExplorerBatchDeleteCheckbox=()=>'',explorerBuildBatchDeleteKey=()=>'',renderExplorerCharacterIdentityMergePanel=()=>'',renderExplorerItemIdentityMergePanel=()=>'',explorerResetBatchDeleteTab=()=>{};
const explorerEntityMemoryBundleKey=x=>x.id,explorerFetchSelectedEntityMemoryItems=async()=>{};
window.requests=[];window.failures=[];
async function bridgeFetch(route,options={}) { const response=await fetch(route,{method:options.method||'GET',headers:{'Content-Type':'application/json'},body:options.body===undefined?undefined:JSON.stringify(options.body)});const data=await response.json();window.requests.push({route,status:response.status,method:options.method||'GET'});if(!response.ok)throw new Error(route+':'+response.status+':'+JSON.stringify(data));return data; }
async function safeCall(f,fallback,label){try{return await f()}catch(e){window.failures.push(label+':'+e.message);throw e}}
function refreshExplorerUI(){document.getElementById('panel').innerHTML=_explorer.activeTab==='trust'?renderExplorerTrust():renderExplorerEntities();bind();}
${functions.map(fn).join('\n')}
function bind(){${bindings}\n${cancelBindings}\n${saveBindings}}
window.openPanel=async tab=>{_explorer.activeTab=tab;if(tab==='trust')await explorerFetchTrust();else await explorerFetchEntities();refreshExplorerUI();};
window.inspect=()=>({trust:_explorer.trust,entities:_explorer.entities,error:_explorer.editError,editStatus:_explorer.editStatus});
`;
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.ARCHIVE_CENTER_UI_BROWSER});
 const page=await browser.newPage({viewport:{width:1320,height:900}}),errors=[],checks=[];
 page.setDefaultTimeout(5000);
 page.on('pageerror',e=>errors.push(e.message));page.on('dialog',d=>d.accept());
 try {
  await page.goto(base+'/__test/ui');await page.addScriptTag({content:runtime});
  await page.evaluate(()=>window.openPanel('entities'));
  assert.equal((await page.evaluate(()=>window.inspect())).entities.error,null);
  for(const sub of ['characters','locations','items']) {
   await page.locator('[data-ent-subtab="'+sub+'"]').click();
   await page.waitForFunction(sub=>document.querySelector('[data-ent-subtab="'+sub+'"]').classList.contains('mo-ent-subtab-active'),sub,{timeout:3000});
   assert.equal(await page.locator('.mo-ent-card').count(),1,'expected seeded '+sub+' card');
  }
  assert.deepEqual(errors,[]);checks.push('character/location/item tabs render database records');
  await page.evaluate(()=>window.openPanel('trust'));
  const marker=()=>page.locator('.mo-trust-name > .mo-trust-badge').filter({hasText:'사용자에 의해 수정됨.'});
  assert.equal(await marker().count(),0,'automatic records must not be marked');
  const edits=[['storyline','sl','current_context','User checked the shield promise.','storylines','storylines'],['worldRule','wr','category','user-confirmed access','world-rules','items'],['hook','hk','resolution_note','User checked the delivery details.','pending-threads','hooks']];
  const assertSavedEdits=async()=>{
   for(const [model,type,field,value,route,list] of edits) {
    const response=await page.request.get(base+'/'+route+'/'+sid+(model==='hook'?'?status=all':''));
    assert.equal(response.status(),200);
    const rows=(await response.json())[list];
    assert.equal(rows.length,1,model+' saved row count');
    assert.equal(rows[0][field],value,model+' actual persisted edit');
    await page.locator('[data-trust-edit-model="'+model+'"]').click();
    assert.equal(await page.locator('.mo-ed-form [data-field="'+field+'"]').inputValue(),value,model+' reopened edit form');
    await page.locator('.mo-ed-cancel-btn').click();
    assert.equal(await page.locator('.mo-ed-form').count(),0);
   }
  };
  for(const [model,type,field,value] of edits) {
   await page.locator('[data-trust-edit-model="'+model+'"]').click();
   await page.locator('.mo-ed-form [data-field="'+field+'"]').fill(value);
   await page.locator('[data-save-type="'+type+'"]').click();
   await page.waitForFunction(()=>!window.inspect().editStatus && !document.querySelector('.mo-ed-form'),null,{timeout:10000});
   assert.equal((await page.evaluate(()=>window.inspect())).error,'');
  }
  assert.equal(await marker().count(),3);checks.push('all three edit/save buttons persist and display markers');
  await assertSavedEdits();checks.push('edited values are read back from SQL-backed APIs and reopened forms');
  for(const model of ['storyline','worldRule','hook']) {
   for(const field of ['suppressed','pinned']) {
    await page.locator('[data-trust-model="'+model+'"][data-trust-field="'+field+'"]').click();
    await page.waitForFunction(({model,field})=>document.querySelector('[data-trust-model="'+model+'"][data-trust-field="'+field+'"]').classList.contains('mo-trust-btn-on'),{model,field});
   }
  }
  assert.equal(await marker().count(),3);checks.push('partial trust responses preserve all other flags');
  await page.reload();await page.addScriptTag({content:runtime});await page.evaluate(()=>window.openPanel('trust'));
  assert.equal(await marker().count(),3);checks.push('full browser reload retains saved markers');
  await assertSavedEdits();checks.push('full browser reload retains the actual edited values');
  for(const step of ['next','reroll','replay']) {
   const response=await page.request.post(base+'/__test/'+step);assert.equal(response.status(),200);
   await page.evaluate(()=>window.openPanel('trust'));
   assert.equal(await marker().count(),3,step+' markers');
   assert.equal(await page.locator('.mo-trust-btn-on').count(),9,step+' flags');
   assert.equal(await page.locator('.mo-trust-row').count(),3,step+' duplicates');
   checks.push(step+' runs canonical complete-turn and preserves flags without duplicate rows');
  }
  for(const model of ['storyline','worldRule','hook'])await page.locator('[data-trust-model="'+model+'"][data-trust-field="user_corrected"]').click();
  await page.waitForFunction(()=>document.querySelectorAll('[data-trust-field="user_corrected"].mo-trust-btn-on').length===0);
  await page.evaluate(()=>window.openPanel('trust'));
  assert.equal(await marker().count(),0);assert.equal(await page.locator('.mo-trust-btn-on').count(),6);checks.push('explicit marker reset persists without resetting pin/suppression');
  await page.screenshot({path:path.join(output,'trust-after-lifecycle.png'),fullPage:true});
  assert.deepEqual(errors,[]);assert.deepEqual(await page.evaluate(()=>window.failures),[]);
  fs.writeFileSync(path.join(output,'browser-e2e.json'),JSON.stringify({checks,errors,requests:await page.evaluate(()=>window.requests)},null,2));
  console.log(JSON.stringify({passed:checks.length,checks}));
 } catch(error) {await page.screenshot({path:path.join(output,'browser-failure.png'),fullPage:true});console.error(JSON.stringify({checks,errors,error:String(error)}));throw error;}
 finally {await browser.close();}
})().catch(error=>{console.error(error);process.exit(1)});
