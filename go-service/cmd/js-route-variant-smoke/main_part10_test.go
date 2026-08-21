package main

import (
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

func TestArchiveCenterJSGLMUsesDocumentedThinkingToggle(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`/(^|\/)glm[-_]/`,
		`mode: "glm_toggle"`,
		`effortOptions: ["enable", "disable"]`,
		`function applyReasoningFieldsToPayload`,
		`payload.glm_thinking_type = (effort === "disable" || effort === "disabled") ? "disabled" : "enabled"`,
		`GLM thinking.type`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing GLM-5.2 reasoning marker %q", needle)
		}
	}
	for _, forbidden := range []string{`glm_52_reasoning_effort`, `GLM-5.2 thinking.type + reasoning_effort`} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js kept undocumented GLM marker %q", forbidden)
		}
	}
}

func TestArchiveCenterJSReasoningFamilyUsesKnownModelNamesBeforeProviderFallback(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`/(^|\/)deepseek[-_]?v4($|[-_:])/`,
		`/(^|\/)gemini[-_]/`,
		`/(^|\/)glm[-_]/`,
		`/(^|\/)claude[-_]/`,
		`function resolveGPTReasoningEffortOptions(model)`,
		`return ["none", "low", "medium", "high", "xhigh", "max"]`,
		`const gptEffortOptions = resolveGPTReasoningEffortOptions(model)`,
		`family === "gpt" && gptEffortOptions.length > 0`,
		`Auto uses the model name and version first and omits reasoning fields when the contract is unknown`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing model-first reasoning marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		`gemini-(?:3(?:\D|$)|[4-9]`,
		`mo-sourceSearchPlannerReasoningEffortRow`,
		`resolveReasoningControls(value, preset ? preset.value : "auto", model ? model.value : "")`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js kept out-of-scope or speculative reasoning marker %q", forbidden)
		}
	}
}

func TestArchiveCenterJSDeepSeekV4ReasoningMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`deepseek_v4: {`,
		`/(^|\/)deepseek[-_]?v4($|[-_:])/`,
		`mode: "deepseek_v4_reasoning_effort"`,
		`effortOptions: ["none", "high", "max"]`,
		`normalizedValue === "low" || normalizedValue === "medium"`,
		`normalizedValue === "xhigh"`,
		`controls.family === "deepseek_v4"`,
		`payload.reasoning_effort = effort || "none"`,
		`DeepSeek V4 thinking.type + reasoning_effort`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing DeepSeek V4 reasoning marker %q", needle)
		}
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
