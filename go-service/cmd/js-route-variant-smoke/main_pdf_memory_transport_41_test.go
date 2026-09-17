package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func archiveCenterNodeForPDFMemoryTransportTest(t *testing.T) string {
	t.Helper()
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath != "" {
		return nodePath
	}
	resolved, err := exec.LookPath("node")
	if err != nil {
		t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for PDF memory transport runtime smoke")
	}
	return resolved
}

func TestArchiveCenterPDFMemoryTransportWiringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, needle := range []string{
		`memoryTransportMode: "text"`,
		`memory_transport_mode: settings.memoryTransportMode || DEFAULT_SETTINGS.memoryTransportMode`,
		`function attachFinalConfirmationMemoryTransport(context, orchestrationResult, prepareBundle)`,
		`function applyGoogleMemoryPDFBody(body, plan, transient)`,
		`function applyLLMGatewayMemoryPDFBody(body, plan, transient)`,
		`function applyProviderManagerMemoryPDFPayload(payload, transportPlan)`,
		`function normalizeProviderManagerMemoryPDFPayload(payload, transportPlan)`,
		`const memoryTransportApplication = applyProviderManagerMemoryPDFPayload(finalPayload, memoryTransportPlan);`,
		`async function onMemoryTransportBodyInterceptor(body, type)`,
		`"provider_manager_pdf"`,
		`"<pm-pdf>\n" + selectedMemoryText + "\n</pm-pdf>"`,
		`requestType === "gemini_base" || requestType === "gemini_base_stream"`,
		`requestType === "openai_basic" || requestType === "openai_streaming"`,
		`await R.registerBodyIntercepter(onMemoryTransportBodyInterceptor)`,
		`body.client_meta.memory_transport_observation = JSON.parse(JSON.stringify(memoryTransportObservation))`,
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing PDF memory transport marker %q", needle)
		}
	}
}

func TestArchiveCenterProviderManagerPDFMarksOnlySelectedMemoryAndReplaysOnce(t *testing.T) {
	nodePath := archiveCenterNodeForPDFMemoryTransportTest(t)
	src := readArchiveCenterJS(t)
	functions := []string{
		extractJSFunctionBlockForTest(t, src, "function providerManagerMemoryPDFMarkerContent(transportPlan)"),
		extractJSFunctionBlockForTest(t, src, "function normalizeProviderManagerMemoryPDFPayload(payload, transportPlan)"),
		extractJSFunctionBlockForTest(t, src, "function applyProviderManagerMemoryPDFPayload(payload, transportPlan)"),
	}
	script := strings.Join(functions, "\n") + `
function assert(condition, message) {
  if (!condition) throw new Error(message);
}
function extractMessages(payload) {
  if (!payload || !Array.isArray(payload.messages)) {
    return {messages: [], rebuild: () => payload, hasMessageSlot: false};
  }
  return {
    messages: payload.messages,
    rebuild: (messages) => ({...payload, messages}),
    hasMessageSlot: true,
  };
}
function getPayloadMessageRoleAndText(message) {
  return {
    role: String(message && message.role || "").toLowerCase(),
    text: typeof (message && message.content) === "string" ? message.content : "",
  };
}
function debugLog() {}
function count(value, needle) {
  return String(value).split(needle).length - 1;
}

const selectedMemory = "[Long-term Memory]\nMEMORY_SENTINEL\n기억 문장";
const remainingAuxiliary = "[Original Work]\ncanon\n\n[Output Guidance]\ncontinue";
const fullAuxiliary = remainingAuxiliary.split("\n\n[Output Guidance]")[0] + "\n\n" + selectedMemory + "\n\n[Output Guidance]\ncontinue";
const prefix = "[Archive Center — Auxiliary Context]\n\n";
const transportPlan = {
  plan_id: "mtp-provider-manager-test",
  selected_mode: "provider_manager_pdf",
  body_format: "provider_manager_manual_pdf",
  long_term_memory_text: selectedMemory,
  logical_memory_chars: selectedMemory.length,
  auxiliary_text: fullAuxiliary,
  auxiliary_without_long_term_memory: remainingAuxiliary,
};
const original = {
  messages: [
    {role: "system", content: "BASE_SYSTEM"},
    {role: "system", content: prefix + fullAuxiliary},
    {role: "user", content: "CURRENT_USER_SENTINEL"},
  ],
};
const snapshot = JSON.stringify(original);
const first = applyProviderManagerMemoryPDFPayload(original, transportPlan);
assert(first.applied === true, "Provider Manager marker route was not applied");
assert(JSON.stringify(original) === snapshot, "Provider Manager route mutated the source payload");
const firstText = JSON.stringify(first.payload);
assert(count(firstText, "<pm-pdf>") === 1 && count(firstText, "</pm-pdf>") === 1, "Provider Manager marker pair was not emitted exactly once");
assert(count(firstText, "MEMORY_SENTINEL") === 1, "selected memory was missing or duplicated");
assert(first.payload.messages.some((message) => message.role === "system" && message.content === prefix + remainingAuxiliary), "non-memory auxiliary lanes changed");
assert(first.payload.messages.some((message) => message.role === "user" && message.content === "CURRENT_USER_SENTINEL"), "current user input changed");

const normalized = normalizeProviderManagerMemoryPDFPayload(first.payload, transportPlan);
assert(normalized.normalized === true, "Provider Manager retry representation was not normalized");
assert(normalized.payload.messages.some((message) => message.role === "system" && message.content === prefix + fullAuxiliary), "Text baseline was not restored for retry replay");
assert(!JSON.stringify(normalized.payload).includes("<pm-pdf>"), "retry normalization retained an old marker");
const retry = applyProviderManagerMemoryPDFPayload(normalized.payload, transportPlan);
const retryText = JSON.stringify(retry.payload);
assert(retry.applied === true, "Provider Manager retry marker route was not applied");
assert(count(retryText, "<pm-pdf>") === 1 && count(retryText, "MEMORY_SENTINEL") === 1, "Provider Manager retry duplicated memory or markers");

for (const selectedMode of ["text", "google_pdf", "llm_gateway_pdf"]) {
  const unchanged = applyProviderManagerMemoryPDFPayload(original, {...transportPlan, selected_mode: selectedMode});
  assert(unchanged.payload === original && unchanged.applied === false, selectedMode + " route was changed by Provider Manager adapter");
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Provider Manager PDF payload fixture failed: %v\n%s", err, output)
	}
}

func TestArchiveCenterPDFMemoryTransportProviderBodiesAreIdempotent(t *testing.T) {
	nodePath := archiveCenterNodeForPDFMemoryTransportTest(t)
	src := readArchiveCenterJS(t)
	functions := []string{
		extractJSFunctionBlockForTest(t, src, "function cloneMemoryTransportProviderBody(body)"),
		extractJSFunctionBlockForTest(t, src, "function restoreMemoryTransportProviderBody(decoded)"),
		extractJSFunctionBlockForTest(t, src, "function replaceSelectedMemoryText(text, selectedMemoryText)"),
		extractJSFunctionBlockForTest(t, src, "function applyGoogleMemoryPDFBody(body, plan, transient)"),
		extractJSFunctionBlockForTest(t, src, "function applyLLMGatewayMemoryPDFBody(body, plan, transient)"),
		extractJSFunctionBlockForTest(t, src, "async function onMemoryTransportBodyInterceptor(body, type)"),
	}
	script := strings.Join(functions, "\n") + `
function assert(condition, message) {
  if (!condition) throw new Error(message);
}
function countGooglePDF(body) {
  return (body.contents || []).flatMap((content) => content.parts || []).filter((part) =>
    part && part.inlineData && part.inlineData.mimeType === "application/pdf" && part.inlineData.data === "JVBERTEST"
  ).length;
}
function countGatewayPDF(body) {
  return (body.messages || []).flatMap((message) => Array.isArray(message.content) ? message.content : []).filter((part) =>
    part && part.type === "file" && part.file &&
    part.file.filename === "archive-center-long-term-memory-test.pdf" &&
    part.file.file_data === "data:application/pdf;base64,JVBERTEST"
  ).length;
}
function containsText(value, needle) {
  return JSON.stringify(value).includes(needle);
}

const selectedMemory = "[Long-term Memory Context]\nMEMORY_SENTINEL";
const basePlan = {
  plan_id: "mtp-test",
  long_term_memory_text: selectedMemory,
  mime_type: "application/pdf",
  filename: "archive-center-long-term-memory-test.pdf",
  logical_memory_chars: selectedMemory.length,
  pdf_bytes: 321,
  page_count: 1,
  estimated_text_tokens: 10,
  estimated_pdf_tokens: 20,
};
const transient = {plan_id: "mtp-test", pdf_base64: "JVBERTEST"};
const logs = [];
function debugLog(...args) { logs.push(args); }

(async () => {
  const googlePlan = {...basePlan, selected_mode: "google_pdf"};
  const googleOriginal = {
    systemInstruction: {parts: [{text: "SYSTEM_PREFIX\n" + selectedMemory + "\nSYSTEM_SUFFIX"}]},
    contents: [
      {role: "user", parts: [{text: "older user"}]},
      {role: "model", parts: [{text: "older answer"}]},
      {role: "user", parts: [{text: "current user"}, {inlineData: {mimeType: "image/png", data: "IMAGE"}}]},
    ],
  };
  const googleSnapshot = JSON.stringify(googleOriginal);
  _activeFinalConfirmationRequestContext = {
    beforeRequestAttemptCount: 1,
    memoryTransportEnvelope: {plan: googlePlan, transient},
  };
  const googleFirst = await onMemoryTransportBodyInterceptor(googleOriginal, "gemini_base_stream");
  assert(JSON.stringify(googleOriginal) === googleSnapshot, "Google source body was mutated");
  assert(!containsText(googleFirst, "MEMORY_SENTINEL"), "Google Text memory remained beside PDF");
  assert(containsText(googleFirst, "SYSTEM_PREFIX") && containsText(googleFirst, "SYSTEM_SUFFIX"), "Google auxiliary text changed");
  assert(containsText(googleFirst, "IMAGE"), "Google existing media changed");
  assert(countGooglePDF(googleFirst) === 1, "Google PDF was not attached exactly once");
  assert(_activeFinalConfirmationRequestContext.memoryTransportObservation.removedTextCount === 1, "Google selected Text removal was not observed");

  _activeFinalConfirmationRequestContext.beforeRequestAttemptCount = 2;
  const googleRetry = await onMemoryTransportBodyInterceptor(googleFirst, "gemini_base_stream");
  assert(countGooglePDF(googleRetry) === 1, "Google retry duplicated the PDF");
  assert(!containsText(googleRetry, "MEMORY_SENTINEL"), "Google retry restored selected Text memory");
  assert(_activeFinalConfirmationRequestContext.memoryTransportEnvelope.plan === googlePlan, "Google retry consumed the prepared plan");
  assert(_activeFinalConfirmationRequestContext.memoryTransportObservation.attemptCount === 2, "Google retry attempt was not observed");

  const googleWrongRoute = await onMemoryTransportBodyInterceptor(googleOriginal, "openai_streaming");
  assert(googleWrongRoute === googleOriginal, "Google mode changed a non-Gemini route");
  const googleMissingText = {
    contents: [{role: "user", parts: [{text: "current user without selected memory"}]}],
  };
  const googleMissingTextResult = await onMemoryTransportBodyInterceptor(googleMissingText, "gemini_base");
  assert(googleMissingTextResult === googleMissingText, "Google mode changed the body when exact selected Text was absent");
  assert(countGooglePDF(googleMissingTextResult) === 0, "Google mode attached PDF without replacing exact selected Text");
  await onMemoryTransportBodyInterceptor({usageMetadata: {promptTokenCount: 123}, modelStatus: "ok"}, "meta_gemini");
  assert(_activeFinalConfirmationRequestContext.memoryTransportObservation.usageMetadata.promptTokenCount === 123, "Gemini usage was not retained");

  const gatewayPlan = {...basePlan, selected_mode: "llm_gateway_pdf"};
  const gatewayOriginal = {
    messages: [
      {role: "system", content: "SYSTEM_PREFIX\n" + selectedMemory + "\nSYSTEM_SUFFIX"},
      {role: "user", content: [{type: "text", text: "older user"}, {type: "file", file: {filename: "other.pdf", file_data: "data:application/pdf;base64,OTHER"}}]},
      {role: "assistant", content: "older answer"},
      {role: "user", content: "current user"},
    ],
  };
  const gatewaySnapshot = JSON.stringify(gatewayOriginal);
  _activeFinalConfirmationRequestContext = {
    beforeRequestAttemptCount: 1,
    memoryTransportEnvelope: {plan: gatewayPlan, transient},
  };
  const gatewayFirstSerialized = await onMemoryTransportBodyInterceptor(JSON.stringify(gatewayOriginal), "openai_streaming");
  const gatewayFirst = JSON.parse(gatewayFirstSerialized);
  assert(JSON.stringify(gatewayOriginal) === gatewaySnapshot, "Gateway source body was mutated");
  assert(!containsText(gatewayFirst, "MEMORY_SENTINEL"), "Gateway Text memory remained beside PDF");
  assert(containsText(gatewayFirst, "SYSTEM_PREFIX") && containsText(gatewayFirst, "SYSTEM_SUFFIX"), "Gateway auxiliary text changed");
  assert(containsText(gatewayFirst, "other.pdf"), "Gateway existing file changed");
  assert(countGatewayPDF(gatewayFirst) === 1, "Gateway PDF was not attached exactly once");
  assert(_activeFinalConfirmationRequestContext.memoryTransportObservation.removedTextCount === 1, "Gateway selected Text removal was not observed");

  _activeFinalConfirmationRequestContext.beforeRequestAttemptCount = 2;
  const gatewayRetry = JSON.parse(await onMemoryTransportBodyInterceptor(JSON.stringify(gatewayFirst), "openai_streaming"));
  assert(countGatewayPDF(gatewayRetry) === 1, "Gateway retry duplicated the PDF");
  assert(!containsText(gatewayRetry, "MEMORY_SENTINEL"), "Gateway retry restored selected Text memory");
  assert(_activeFinalConfirmationRequestContext.memoryTransportEnvelope.plan === gatewayPlan, "Gateway retry consumed the prepared plan");
  assert(_activeFinalConfirmationRequestContext.memoryTransportObservation.attemptCount === 2, "Gateway retry attempt was not observed");

  const gatewayWrongRoute = await onMemoryTransportBodyInterceptor(gatewayOriginal, "gemini_base");
  assert(gatewayWrongRoute === gatewayOriginal, "Gateway mode changed a non-OpenAI route");
  const gatewayMissingText = {messages: [{role: "user", content: "current user without selected memory"}]};
  const gatewayMissingTextResult = await onMemoryTransportBodyInterceptor(gatewayMissingText, "openai_basic");
  assert(gatewayMissingTextResult === gatewayMissingText, "Gateway mode changed the body when exact selected Text was absent");
  assert(countGatewayPDF(gatewayMissingTextResult) === 0, "Gateway mode attached PDF without replacing exact selected Text");

  const textOriginal = {messages: [{role: "system", content: selectedMemory}, {role: "user", content: "current user"}]};
  _activeFinalConfirmationRequestContext = {
    beforeRequestAttemptCount: 1,
    memoryTransportEnvelope: {plan: {...basePlan, selected_mode: "text"}, transient},
  };
  const textResult = await onMemoryTransportBodyInterceptor(textOriginal, "openai_streaming");
  assert(textResult === textOriginal, "Text mode changed the provider body");
  assert(containsText(textResult, "MEMORY_SENTINEL"), "Text mode removed the selected memory");

  assert(logs.every((entry) => !JSON.stringify(entry).includes("JVBERTEST")), "PDF base64 leaked to debug logs");
})().catch((error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PDF memory transport provider body fixture failed: %v\n%s", err, output)
	}
}
