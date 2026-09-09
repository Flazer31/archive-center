package main

import (
	"os"
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

func TestArchiveCenterJSGLMUsesVersionedThinkingContract(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`/(^|\/)glm[-_]/`,
		`function resolveGLMReasoningMode(model)`,
		`mode: "glm_reasoning_effort"`,
		`effortOptions: ["none", "high", "max"]`,
		`mode: "glm_toggle"`,
		`effortOptions: ["enable", "disable"]`,
		`function applyReasoningFieldsToPayload`,
		`payload.glm_thinking_type = (effort === "disable" || effort === "disabled") ? "disabled" : "enabled"`,
		`GLM 5.2+ thinking.type + reasoning_effort`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing versioned GLM reasoning marker %q", needle)
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
		`function resolveReasoningTransport(provider, endpoint)`,
		`transport === "ollama" && family !== "none"`,
		`["custom", "opencode", "opencode-go"].includes(transport) && family === "deepseek_v4"`,
		`mode: "gateway_reasoning_effort"`,
		`mode: "deepseek_v4_reasoning_effort"`,
		`const deepSeekV4EffortOptions = transport === "neuralwatt"`,
		`if (normalizedValue === "low" && !options.includes("low")) normalizedValue = "high"`,
		`if (normalizedValue === "medium" && controls.mode !== "ollama_reasoning_effort") normalizedValue = "high"`,
		`normalizedValue === "xhigh"`,
		`controls.mode === "deepseek_v4_reasoning_effort"`,
		`payload.reasoning_effort = effort || "none"`,
		`현재 전송 규약: DeepSeek V4 thinking.type + reasoning_effort`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing DeepSeek V4 reasoning marker %q", needle)
		}
	}
}

func TestArchiveCenterJSReasoningControlsUseProviderAndEndpointRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for reasoning controls runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "function sanitizeEnumValue("),
		extractJSFunctionBlockForTest(t, src, "function normalizeLlmProvider("),
		extractJSFunctionBlockForTest(t, src, "function normalizeReasoningPreset("),
		extractJSFunctionBlockForTest(t, src, "function detectReasoningFamily("),
		extractJSFunctionBlockForTest(t, src, "function resolveReasoningTransport("),
		extractJSFunctionBlockForTest(t, src, "function normalizeReasoningModelIdentifier("),
		extractJSFunctionBlockForTest(t, src, "function resolveGLMReasoningMode("),
		extractJSFunctionBlockForTest(t, src, "function resolveGeminiThinkingMode("),
		extractJSFunctionBlockForTest(t, src, "function resolveGeminiThinkingLevelOptions("),
		extractJSFunctionBlockForTest(t, src, "function resolveClaudeThinkingMode("),
		extractJSFunctionBlockForTest(t, src, "function resolveGPTReasoningEffortOptions("),
		extractJSFunctionBlockForTest(t, src, "function resolveReasoningControls("),
		extractJSFunctionBlockForTest(t, src, "function normalizeReasoningEffortForControls("),
		extractJSFunctionBlockForTest(t, src, "function applyReasoningFieldsToPayload("),
	}
	script := `
const LLM_PROVIDER_OPTIONS = ["openai","claude","gemini","openrouter","llmgateway","vercel","neuralwatt","vertex","copilot","ollama","opencode","opencode-go","custom"];
const REASONING_PRESET_OPTIONS = ["auto","gpt","gemini","claude","glm","custom"];
` + strings.Join(blocks, "\n") + `
function assert(value, message) { if (!value) throw new Error(message); }
for (const [provider, endpoint, model] of [
  ["gemini", "https://generativelanguage.googleapis.com/v1beta", "gemini-3.8-flash"],
  ["opencode", "https://opencode.ai/zen/v1", "gemini-3.8-flash"],
  ["vertex", "https://aiplatform.googleapis.com/v1/projects/project/locations/global/publishers/google/models", "gemini-3.8-flash"],
  ["llmgateway", "https://api.llmgateway.io/v1", "gemini-3.8-flash"],
  ["openrouter", "https://openrouter.ai/api/v1", "google/gemini-3.8-flash"],
]) {
  const controls = resolveReasoningControls(provider, "auto", model, endpoint);
  assert(JSON.stringify(controls.effortOptions) === JSON.stringify(["none","low","medium","high"]), provider + ": " + JSON.stringify(controls));
  const effort = normalizeReasoningEffortForControls("medium", controls);
  assert(effort === "medium", provider + " lost selected medium");
  const payload = {};
  applyReasoningFieldsToPayload(payload, controls, "auto", effort, 0);
  assert(payload.reasoning_effort === "medium", provider + ": " + JSON.stringify(payload));
  const nonePayload = {};
  applyReasoningFieldsToPayload(nonePayload, controls, "auto", "none", 0);
  assert(nonePayload.reasoning_effort === (controls.mode === "gateway_reasoning_effort" ? "none" : undefined), provider + " changed none semantics");
}
assert(JSON.stringify(resolveGeminiThinkingLevelOptions("gemini-3-pro-preview")) === JSON.stringify(["none","low","high"]), "3 Pro options changed");
assert(resolveGeminiThinkingLevelOptions("gemini-3.6-flash").includes("minimal"), "3.6 Flash minimal disappeared");
assert(resolveGeminiThinkingLevelOptions("gemini-3.7-flash").includes("medium"), "3.7 Flash medium disappeared");
const ollama = resolveReasoningControls("ollama", "auto", "deepseek-v4-pro:0813-cloud", "http://127.0.0.1:11434");
assert(ollama.mode === "ollama_reasoning_effort", JSON.stringify(ollama));
assert(JSON.stringify(ollama.effortOptions) === JSON.stringify(["none","low","medium","high"]), JSON.stringify(ollama));
assert(normalizeReasoningEffortForControls("medium", ollama) === "medium", "Ollama medium was promoted");
assert(normalizeReasoningEffortForControls("max", ollama) === "none", "unsupported Ollama max survived");
const gatewayLuna = resolveReasoningControls("llmgateway", "auto", "gpt-5.6-luna", "https://api.llmgateway.io/v1");
assert(gatewayLuna.mode === "gateway_reasoning_effort" && gatewayLuna.effortOptions.includes("low"), JSON.stringify(gatewayLuna));
const gatewayDeepSeek = resolveReasoningControls("openrouter", "auto", "deepseek/deepseek-v4-pro", "https://openrouter.ai/api/v1");
assert(gatewayDeepSeek.mode === "gateway_reasoning_effort", JSON.stringify(gatewayDeepSeek));
assert(gatewayDeepSeek.effortOptions.includes("low"), JSON.stringify(gatewayDeepSeek));
const directDeepSeek = resolveReasoningControls("custom", "auto", "deepseek-v4-pro", "https://api.deepseek.com/v1");
assert(directDeepSeek.mode === "deepseek_v4_reasoning_effort", JSON.stringify(directDeepSeek));
assert(JSON.stringify(directDeepSeek.effortOptions) === JSON.stringify(["none","low","high","max"]), JSON.stringify(directDeepSeek));
assert(normalizeReasoningEffortForControls("low", directDeepSeek) === "low", "DeepSeek direct low was promoted");
assert(normalizeReasoningEffortForControls("medium", directDeepSeek) === "high", "DeepSeek direct medium compatibility changed");
const directDeepSeekPayload = {};
applyReasoningFieldsToPayload(directDeepSeekPayload, directDeepSeek, "auto", "low", 0);
assert(directDeepSeekPayload.reasoning_effort === "low", JSON.stringify(directDeepSeekPayload));
const neuralWattPro = resolveReasoningControls("neuralwatt", "auto", "deepseek-v4-pro", "https://api.neuralwatt.com/v1");
assert(neuralWattPro.effortOptions.includes("low"), JSON.stringify(neuralWattPro));
const neuralWattFlash = resolveReasoningControls("neuralwatt", "auto", "deepseek-v4-flash-flex", "https://api.neuralwatt.com/v1");
assert(!neuralWattFlash.effortOptions.includes("low"), JSON.stringify(neuralWattFlash));
assert(normalizeReasoningEffortForControls("low", neuralWattFlash) === "high", "NeuralWatt Flash low was not mapped to its documented high tier");
const openCodeClaude = resolveReasoningControls("opencode", "auto", "claude-sonnet-4-6", "");
assert(openCodeClaude.mode === "claude_adaptive" && openCodeClaude.effortOptions.includes("medium"), JSON.stringify(openCodeClaude));
const goDeepSeek = resolveReasoningControls("opencode-go", "auto", "deepseek-v4-pro", "");
assert(goDeepSeek.effortOptions.includes("low"), JSON.stringify(goDeepSeek));
const openCodeDeepSeek = resolveReasoningControls("opencode", "auto", "deepseek-v4-pro", "");
assert(openCodeDeepSeek.mode === "gateway_reasoning_effort" && openCodeDeepSeek.effortOptions.includes("low"), JSON.stringify(openCodeDeepSeek));
const customGatewayDeepSeek = resolveReasoningControls("custom", "auto", "deepseek-v4-pro", "https://opencode.ai/zen/v1");
assert(customGatewayDeepSeek.mode === "gateway_reasoning_effort" && customGatewayDeepSeek.effortOptions.includes("low"), JSON.stringify(customGatewayDeepSeek));
const customGatewayPayload = {};
applyReasoningFieldsToPayload(customGatewayPayload, customGatewayDeepSeek, "auto", "low", 0);
assert(customGatewayPayload.reasoning_effort === "low", JSON.stringify(customGatewayPayload));
const conflict = resolveReasoningControls("ollama", "auto", "deepseek-v4-pro", "https://api.deepseek.com/v1");
assert(conflict.mode === "unsupported" && !conflict.showEffort, JSON.stringify(conflict));
const directGLM52 = resolveReasoningControls("custom", "auto", "glm-5.2", "https://api.z.ai/api/paas/v4");
assert(directGLM52.mode === "glm_reasoning_effort", JSON.stringify(directGLM52));
assert(JSON.stringify(directGLM52.effortOptions) === JSON.stringify(["none","high","max"]), JSON.stringify(directGLM52));
assert(resolveGLMReasoningMode("z-ai/glm-6.0") === "effort", "future GLM version did not inherit the 5.2+ contract");
assert(resolveGLMReasoningMode("glm-4.7") === "toggle", "GLM-4.7 did not keep the toggle contract");
const directGLM52Payload = {};
applyReasoningFieldsToPayload(directGLM52Payload, directGLM52, "auto", "max", 0);
assert(directGLM52Payload.glm_thinking_type === "enabled" && directGLM52Payload.reasoning_effort === "max", JSON.stringify(directGLM52Payload));
const directGLM51 = resolveReasoningControls("custom", "auto", "glm-5.1", "https://api.z.ai/api/paas/v4");
assert(directGLM51.mode === "glm_toggle", JSON.stringify(directGLM51));
assert(JSON.stringify(directGLM51.effortOptions) === JSON.stringify(["enable","disable"]), JSON.stringify(directGLM51));
const directGLM51Payload = {};
applyReasoningFieldsToPayload(directGLM51Payload, directGLM51, "auto", "enable", 0);
assert(directGLM51Payload.glm_thinking_type === "enabled" && !directGLM51Payload.reasoning_effort, JSON.stringify(directGLM51Payload));
const ollamaGLM52 = resolveReasoningControls("ollama", "auto", "glm-5.2:cloud", "http://127.0.0.1:11434");
assert(JSON.stringify(ollamaGLM52.effortOptions) === JSON.stringify(["none","high"]), JSON.stringify(ollamaGLM52));
assert(normalizeReasoningEffortForControls("max", ollamaGLM52) === "high", "Ollama GLM-5.2 max was not bounded to high");
const ollamaGLM51 = resolveReasoningControls("ollama", "auto", "glm-5.1:cloud", "http://127.0.0.1:11434");
assert(JSON.stringify(ollamaGLM51.effortOptions) === JSON.stringify(["enable","disable"]), JSON.stringify(ollamaGLM51));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reasoning controls runtime failed: %v\n%s", err, output)
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
