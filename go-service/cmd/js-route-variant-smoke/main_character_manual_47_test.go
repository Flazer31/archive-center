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
const renderExplorerBatchDeleteToolbar=()=>'',renderExplorerCharacterIdentityMergePanel=()=>'',renderExplorerBatchDeleteCheckbox=()=>'',explorerBuildBatchDeleteKey=()=>'';
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
_explorer.entities.characters=[{character_name:'Mira',speech_style_json:'null'}];
assert(renderExplorerEntities().includes('Mira'),'empty voice must not break the character panel');
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("manual voice renderer: %v\n%s", err, out)
	}
}
