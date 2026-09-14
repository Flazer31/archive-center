package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestArchiveCenterJSPersonaCapsuleUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const _personaCapsuleState = {`,
		`function renderPersonaCapsuleSection`,
		`function attachPersonaCapsuleEvents`,
		`function createPersonaCapsuleFromForm`,
		`function attachPersonaCapsuleToCurrentSession`,
		`function detachPersonaCapsuleFromCurrentSession`,
		`function rememberPersonaCapsuleCandidatesFromCompleteTurn`,
		`function renderPersonaCapsuleCandidateReview`,
		`function approvePersonaCapsuleCandidate`,
		`function usePersonaCapsuleCandidateAsDraft`,
		`function personaCapsuleCurrentSourceSessionId`,
		`function personaCapsuleMatchesCurrentSourceSession`,
		`function loadSubjectiveEntityMemoriesForPersonaCapsule`,
		`function createPersonaCapsuleFromSelectedEntityMemories`,
		`function loadSubjectiveEntityBundlesForPersonaCapsule`,
		`function createPersonaCapsuleFromSelectedEntityBundle`,
		`function personaCapsuleApplyEntityBundle`,
		`function personaCapsuleOwnerIsNPCPrivate`,
		`params.set("source_chat_session_id", sourceSID)`,
		`params.set("owner_entity_key", ownerKey)`,
		`"/subjective-entity-memories/entities?"`,
		`queue.filter(personaCapsuleMatchesCurrentSourceSession)`,
		`function loadPersonaCapsuleAttachments`,
		`function useSelectedTimelineItemForPersonaCapsule`,
		`PERSONA_CAPSULE_CANDIDATE_QUEUE_KEY`,
		`data-persona-candidate-approve-id`,
		`persona.candidate.title`,
		`persona.candidate.approve`,
		`persona.status.candidateProposed`,
		`["persona", t('persona.tab')]`,
		`data-tab-jump="' + id + '"`,
		`class="mo-subtabs mo-settings-subtabs mo-extension-subtabs"`,
		`data-tab-panel="${_settingsActiveTab}"`,
		`id="mo-persona-capsule-root"`,
		`data-persona-capsule-create="true"`,
		`data-persona-entity-memory-load="true"`,
		`data-persona-entity-memory-create="true"`,
		`data-persona-entity-bundle-load="true"`,
		`data-persona-entity-bundle-create="true"`,
		`data-persona-entity-bundle-select-key`,
		`data-persona-capsule-attach-id`,
		`data-persona-capsule-detach-id`,
		`"/persona-capsules"`,
		`"/subjective-entity-memories?"`,
		`"/subjective-entity-memories/capsule"`,
		`"/persona-capsules/attachments?`,
		`"/persona-capsules/attached-entries?`,
		`"/persona-capsules/" + encodeURIComponent(id) + "/attach"`,
		`support_only_persona_recollection`,
		`support_only_npc_private_recollection`,
		`npc_private_recollection`,
		`lastPersonaCapsuleStatus`,
		`persona.desc`,
		`persona.secretDesc`,
		`persona.entityBundle.title`,
		`persona.advanced.title`,
		`persona.status.idle`,
		`state.message || t("persona.status.idle")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Persona Capsule UI marker %q", needle)
		}
	}
}

// The former model-policy assertions moved to the Go settings view/wire matrix.
// JS retains only the observed input envelope; it never expands model fields.
func TestArchiveCenterJSReasoningMappingIsBackendOwned(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, old := range []string{"function resolveReasoningControls(", "function detectReasoningFamily(", "function resolveReasoningSyncUiState(", "const REASONING_PRESET_GUIDE"} {
		if strings.Contains(src, old) {
			t.Fatalf("duplicate JS policy remains: %s", old)
		}
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	script := extractJSFunctionBlockForTest(t, src, "function applyReasoningFieldsToPayload(") + `
const assert=require('node:assert/strict');
for(const effort of ['none','low','medium','high','max','enable','disable']) {
 const request={model:'glm-5.3',api_key:'fixture-key'};
 applyReasoningFieldsToPayload(request,'auto',effort,2048);
 assert.deepEqual(request,{model:'glm-5.3',api_key:'fixture-key',reasoning_input:{preset:'auto',effort,budget:2048}});
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("host input observation failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSReasoningEffortInitialSelectPreservesStoredMax(t *testing.T) {
	src := readArchiveCenterJS(t)
	tests := []struct {
		selectID string
		setting  string
	}{
		{selectID: "mo-pluginMainReasoningEffort", setting: "pluginMainReasoningEffort"},
		{selectID: "mo-subLlmReasoningEffort", setting: "subLlmReasoningEffort"},
	}
	for _, tt := range tests {
		startMarker := `<select id="` + tt.selectID + `">`
		start := strings.Index(src, startMarker)
		if start < 0 {
			t.Fatalf("Archive Center.js missing reasoning effort select %q", tt.selectID)
		}
		endRelative := strings.Index(src[start:], `</select>`)
		if endRelative < 0 {
			t.Fatalf("Archive Center.js reasoning effort select %q has no closing tag", tt.selectID)
		}
		selectHTML := src[start : start+endRelative]
		for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "enable", "disable"} {
			if !strings.Contains(selectHTML, `<option value="`+effort+`"`) {
				t.Errorf("reasoning effort select %q missing initial option %q", tt.selectID, effort)
			}
		}
		maxBinding := `<option value="max"${s.` + tt.setting + ` === "max" ? " selected" : ""}>max</option>`
		if !strings.Contains(selectHTML, maxBinding) {
			t.Errorf("reasoning effort select %q does not restore stored max on initial render", tt.selectID)
		}
	}
}
