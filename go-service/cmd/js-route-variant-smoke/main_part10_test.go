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
		`["persona", t("persona.tab")]`,
		`data-tab-jump="' + id + '"`,
		`data-tab-panel="persona"`,
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

func TestArchiveCenterJSGLM52ReasoningEffortMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function resolveGLMThinkingMode`,
		`glm_52_reasoning_effort`,
		`["none", "minimal", "low", "medium", "high", "xhigh", "max"]`,
		`function applyReasoningFieldsToPayload`,
		`payload.glm_thinking_type = "disabled"`,
		`payload.glm_thinking_type = "enabled"`,
		`payload.reasoning_effort = effort`,
		`GLM-5.2 thinking.type + reasoning_effort`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing GLM-5.2 reasoning marker %q", needle)
		}
	}
}
