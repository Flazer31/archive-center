package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReferenceUsageTabSettingsSaveIndependently(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node unavailable")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "renderReferenceUsageSettingsPanel") + "\n" +
		extractArchiveCenterJSFunction(t, src, "attachReferenceUsageSettingsEvents")
	script := `
const assert = require('assert');
const t = x => x, escapeAttr = x => String(x ?? '');
const DEFAULT_SETTINGS = {referenceInjectionMaxChars:3000,primaryCanonBaseMaxChars:3000,lorebookReferenceMaxChars:3000};
let settings = {...DEFAULT_SETTINGS, referenceInjectionEnabled:true,lorebookReferenceMode:'reference_assist',other:'unchanged'};
const panels = {}, patches = [];
let saveMode = 'ok', syncs = 0, refreshes = 0;
const document = {getElementById:id=>panels[id] || null};
async function updateSettings(patch) {
 patches.push({...patch});
 if(saveMode==='throw') throw new Error('synthetic storage failure');
 if(saveMode==='false') return false;
 settings={...settings,...patch};
 return true;
}
async function syncCurrentLorebookReference(options) {assert.strictEqual(options.force,true);syncs++;}
function updateLorebookReferenceManagementStatus(){refreshes++;}
function mount(kind) {
 const html=renderReferenceUsageSettingsPanel(kind), elements={};
 for(const match of html.matchAll(/<(input|button|span)\b([^>]*\bid="([^"]+)"[^>]*)>/g)) {
  const attrs=match[2];
  elements[match[3]]={type:(attrs.match(/\btype="([^"]+)"/)||[])[1]||'',value:(attrs.match(/\bvalue="([^"]*)"/)||[])[1]||'',checked:/\bchecked\b/.test(attrs),disabled:false,textContent:'',listeners:{},addEventListener(type,handler){this.listeners[type]=handler;}};
 }
 panels['mo-'+kind+'-usage-settings']={querySelector:selector=>elements[selector.slice(1)]||null};
 attachReferenceUsageSettingsEvents(kind);
 return {html,elements,save:()=>elements['mo-'+kind+'-usage-save'].listeners.click()};
}
` + functions + `
(async()=>{
 attachReferenceUsageSettingsEvents('persona');
 let original=mount('reference'), lore=mount('lorebook');
 assert(original.elements['mo-referenceInjectionEnabled'].checked);
 assert(lore.elements['mo-lorebookReferenceAssistEnabled'].checked);
 assert(!original.elements['mo-lorebookReferenceMaxChars'] && !lore.elements['mo-referenceInjectionMaxChars']);
 original.elements['mo-referenceInjectionEnabled'].checked=false;
 original.elements['mo-referenceInjectionMaxChars'].value='6500';
 original.elements['mo-primaryCanonBaseMaxChars'].value='0';
 lore.elements['mo-lorebookReferenceMaxChars'].value='9999';
 await original.save();
 assert.deepStrictEqual(Object.keys(patches.at(-1)).sort(),['primaryCanonBaseMaxChars','referenceInjectionEnabled','referenceInjectionMaxChars']);
 assert.strictEqual(settings.referenceInjectionEnabled,false);
 assert.strictEqual(settings.referenceInjectionMaxChars,'6500');
 assert.strictEqual(settings.primaryCanonBaseMaxChars,'0');
 assert.strictEqual(settings.lorebookReferenceMaxChars,3000,'saving original-work must not save another tab draft');
 assert.strictEqual(settings.other,'unchanged');
 assert.strictEqual(syncs,0);
 original=mount('reference');
 assert(!original.elements['mo-referenceInjectionEnabled'].checked,'reopen must show saved off state');
 assert.strictEqual(original.elements['mo-referenceInjectionMaxChars'].value,'6500');
 lore.elements['mo-lorebookReferenceAssistEnabled'].checked=false;
 lore.elements['mo-lorebookReferenceMaxChars'].value='0';
 await lore.save();
 assert.deepStrictEqual(Object.keys(patches.at(-1)).sort(),['lorebookReferenceMaxChars','lorebookReferenceMode']);
 assert.strictEqual(settings.lorebookReferenceMode,'search_only');
 assert.strictEqual(settings.lorebookReferenceMaxChars,'0');
 assert.strictEqual(settings.referenceInjectionMaxChars,'6500');
 assert.strictEqual(syncs,1); assert.strictEqual(refreshes,1);
 lore=mount('lorebook');
 assert(!lore.elements['mo-lorebookReferenceAssistEnabled'].checked);
 original.elements['mo-referenceInjectionEnabled'].checked=true;
 await original.save();
 lore.elements['mo-lorebookReferenceAssistEnabled'].checked=true;
 await lore.save();
 assert.strictEqual(settings.referenceInjectionEnabled,true);
 assert.strictEqual(settings.referenceInjectionMaxChars,'6500','enabling must keep saved budget');
 assert.strictEqual(settings.lorebookReferenceMode,'reference_assist');
 assert.strictEqual(syncs,2);
 for(const mode of ['false','throw']) {
  saveMode=mode;
  await original.save();
  assert.strictEqual(original.elements['mo-reference-usage-status'].textContent,'settings.status.saveFailed');
  assert.strictEqual(original.elements['mo-reference-usage-save'].disabled,false);
 }
 saveMode='ok';
 await original.save();
 assert.strictEqual(original.elements['mo-reference-usage-status'].textContent,'settings.status.saved');
})().catch(e=>{console.error(e);process.exitCode=1});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reference tab settings: %v\n%s", err, out)
	}
	// Check the real settings form mounts these owners outside the data roots,
	// so asynchronous data refresh cannot erase an unsaved settings draft.
	for _, kind := range []string{"reference", "lorebook"} {
		if !strings.Contains(src, `renderReferenceUsageSettingsPanel("`+kind+`") + '<div id="mo-`) {
			t.Fatalf("%s settings must be separate from the refreshed data root", kind)
		}
	}
	commonStart := strings.Index(src, `<!-- ▸ 공통 설정 (Common) -->`)
	if commonStart < 0 {
		t.Fatal("common settings start unavailable")
	}
	commonEnd := strings.Index(src[commonStart:], `id="mo-save-btn"`)
	if commonStart < 0 || commonEnd < 0 {
		t.Fatal("common settings range unavailable")
	}
	common := src[commonStart : commonStart+commonEnd]
	for _, id := range []string{"mo-referenceInjectionEnabled", "mo-referenceInjectionMaxChars", "mo-primaryCanonBaseMaxChars", "mo-lorebookReferenceAssistEnabled", "mo-lorebookReferenceMaxChars"} {
		if strings.Contains(common, `id="`+id+`"`) {
			t.Fatalf("moved control retained in common settings: %s", id)
		}
	}
}
