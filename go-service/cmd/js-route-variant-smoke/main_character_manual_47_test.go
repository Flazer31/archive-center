package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestArchiveCenterJSManualVoiceRendersAfterRollback(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node unavailable")
		}
	}
	src := readArchiveCenterJS(t)
	render := extractJSFunctionBlockForTest(t, src, "function renderExplorerEntities()")
	script := `
const assert=require('assert');
const escapeAttr=x=>String(x??'').replaceAll('<','&lt;').replaceAll('>','&gt;');
const t=x=>x, formatDisplayEntityLabel=x=>x;
const renderExplorerBatchDeleteToolbar=()=>'',renderExplorerCharacterIdentityMergePanel=()=>'',renderExplorerItemIdentityMergePanel=()=>'',renderExplorerBatchDeleteCheckbox=()=>'',explorerBuildBatchDeleteKey=()=>'';
const explorerIsEditing=()=>false;
const _explorer={selectedSessionId:'manual-ui',entities:{characters:[],locations:[],items:[],memoryBundles:[]}};
` + render + `
for(const speech of [
 {manual_overrides:{speech_notes:'manual-note'},principles:[{principle_key:'manual-principle'}]},
 {contract_version:'voice_behavior_projection.v1',manual_overrides:{speech_notes:'manual-note'},principles:[{principle_key:'manual-principle'}]},
 {speech_notes:'manual-note'}
]) {
 _explorer.entities.characters=[{character_name:'Mira',speech_style_json:JSON.stringify(speech)}];
 const html=renderExplorerEntities();
 assert(html.includes('manual-note'),'saved manual note must remain visible');
 if(speech.principles) assert(html.includes('manual-principle'),'manual-only principle must remain visible after rollback');
}
_explorer.entities.characters=[{character_name:'Mira',speech_style_json:'null',user_corrected:true}];
assert(renderExplorerEntities().includes('explorer.edit.userCorrected'),'persisted user edits must show a label');
_explorer.entities.characters[0].user_corrected=false;
assert(!renderExplorerEntities().includes('explorer.edit.userCorrected'),'automatic character data must not be labelled as user edits');
assert(renderExplorerEntities().includes('Mira'),'empty voice must not break the character panel');
_explorer.entities.activeSubTab='locations';
_explorer.entities.locations=[{id:7,key:'Library',scope:'location',value_json:'{}',user_corrected:true}];
assert(renderExplorerEntities().includes('explorer.edit.userCorrected'),'location rule marker must render');
_explorer.entities.activeSubTab='items';
_explorer.entities.items=[{id:8,item:'Brass key',owner:'Mira',predicate:'owns',source_turn:1}];
assert(renderExplorerEntities().includes('Brass key'),'ordinary item list must still render after adding markers');
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("manual voice renderer: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSManualTrustMarksAndPartialResponse(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node unavailable")
		}
	}
	src := readArchiveCenterJS(t)
	script := `
const assert=require('assert');
const escapeAttr=x=>String(x??'');
const t=x=>x==='explorer.edit.userCorrected'?'사용자에 의해 수정됨.':x;
const renderExplorerBatchDeleteToolbar=()=>'',renderExplorerBatchDeleteCheckbox=()=>'',explorerBuildBatchDeleteKey=()=>'',explorerIsEditing=()=>false;
const explorerSessionId=()=> 'manual-ui',refreshExplorerUI=()=>{},getRequestTimeoutSettingMs=()=>1000;
const safeCall=async f=>f();
let response={status:'ok',pinned:true};
const bridgeFetch=async()=>response;
const _explorer={selectedSessionId:'manual-ui',trust:{patchStatus:{},storylines:[],worldRules:[],hooks:[]}};
` + extractJSFunctionBlockForTest(t, src, "function renderExplorerTrust()") +
		extractJSFunctionBlockForTest(t, src, "async function explorerPatchTrust(") + `
(async()=>{
for(const [group,model] of [['storylines','storyline'],['worldRules','worldRule'],['hooks','hook']]) {
  _explorer.trust[group]=[{id:1,name:'Manual item',title:'Manual item',key:'Manual item',user_corrected:true,suppressed:true},{id:2,name:'Automatic item',title:'Automatic item',key:'Automatic item'}];
  const html=renderExplorerTrust();
  assert.equal((html.match(/<span class="mo-trust-badge">사용자에 의해 수정됨\.<\/span>/g)||[]).length,1);
  await explorerPatchTrust(model,1,'pinned',true);
  assert.equal(_explorer.trust[group][0].user_corrected,true,'pin response must not clear edit marker');
  assert.equal(_explorer.trust[group][0].suppressed,true,'pin response must not clear suppression');
  response={status:'ok',user_corrected:false};
  await explorerPatchTrust(model,1,'user_corrected',false);
  assert.equal(_explorer.trust[group][0].user_corrected,false);
  assert.equal(_explorer.trust[group][0].pinned,true);
  assert(!renderExplorerTrust().includes('<span class="mo-trust-badge">사용자에 의해 수정됨.</span>'));
  _explorer.trust[group]=[];
  response={status:'ok',pinned:true};
}
})().catch(err=>{console.error(err);process.exit(1)});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("manual trust marks: %v\n%s", err, out)
	}
}
