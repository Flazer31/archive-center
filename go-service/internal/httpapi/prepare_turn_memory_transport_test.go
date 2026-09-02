package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/pdfmemory"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func prepareTurnMemoryTransportFixture() map[string]any {
	return map[string]any{
		"contract_version": "payload_application_plan.v1",
		"auxiliary_text":   "[Original Work]\ncanon\n\n[Long-term Memory]\n기억 문장\n\n[Output Guidance]\ncontinue",
		"lanes": []map[string]any{
			{"key": "original_work", "applied": true, "text": "[Original Work]\ncanon"},
			{"key": "long_term_memory", "applied": true, "text": "[Long-term Memory]\n기억 문장"},
			{"key": "output_guidance", "applied": true, "text": "[Output Guidance]\ncontinue"},
		},
	}
}

func TestBuildPrepareTurnMemoryTransportTextPreservesBaseline(t *testing.T) {
	payloadPlan := prepareTurnMemoryTransportFixture()
	called := false
	plan, transient := buildPrepareTurnMemoryTransport("text", payloadPlan, "request-text", func(string) (pdfmemory.Document, error) {
		called = true
		return pdfmemory.Document{}, nil
	})
	if called || transient != nil {
		t.Fatalf("text mode invoked PDF generation: called=%v transient=%#v", called, transient)
	}
	if plan["selected_mode"] != "text" || plan["body_format"] != "text" || plan["transport_status"] != "text_mode" {
		t.Fatalf("unexpected text plan: %#v", plan)
	}
	if plan["auxiliary_text"] != payloadPlan["auxiliary_text"] || plan["text_baseline_retained"] != true {
		t.Fatal("text mode changed the existing auxiliary baseline")
	}
}

func TestBuildPrepareTurnMemoryTransportProviderFormatsUseSameSelectedMemory(t *testing.T) {
	payloadPlan := prepareTurnMemoryTransportFixture()
	wantMemory := "[Long-term Memory]\n기억 문장"
	wantWithout := "[Original Work]\ncanon\n\n[Output Guidance]\ncontinue"
	wantFormats := map[string]string{
		"google_pdf":      "gemini_inline_data",
		"llm_gateway_pdf": "openai_file",
	}
	var logicalHashes []string
	for mode, wantFormat := range wantFormats {
		t.Run(mode, func(t *testing.T) {
			var generatedText string
			plan, transient := buildPrepareTurnMemoryTransport(mode, payloadPlan, "request-provider", func(text string) (pdfmemory.Document, error) {
				generatedText = text
				return pdfmemory.Generate(text)
			})
			if generatedText != wantMemory {
				t.Fatalf("generated text=%q, want exact selected lane %q", generatedText, wantMemory)
			}
			if plan["selected_mode"] != mode || plan["body_format"] != wantFormat || plan["transport_status"] != "pdf_ready" {
				t.Fatalf("provider format mismatch: %#v", plan)
			}
			if plan["auxiliary_without_long_term_memory"] != wantWithout {
				t.Fatalf("non-memory auxiliary changed: %q", plan["auxiliary_without_long_term_memory"])
			}
			if transient == nil || transient["plan_id"] != plan["plan_id"] || extractionStringFromAny(transient["pdf_base64"]) == "" {
				t.Fatalf("transient PDF missing: plan=%#v transient=%#v", plan, transient)
			}
			if intFromAny(plan["page_count"], 0) < 1 || intFromAny(plan["pdf_bytes"], 0) < 1 || plan["mime_type"] != "application/pdf" {
				t.Fatalf("PDF metadata incomplete: %#v", plan)
			}
			logicalHashes = append(logicalHashes, extractionStringFromAny(plan["logical_text_hash"]))
			encoded := extractionStringFromAny(transient["pdf_base64"])
			publicJSON, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(publicJSON), encoded) || strings.Contains(string(publicJSON), "pdf_base64") {
				t.Fatal("public plan leaked transient PDF data")
			}
		})
	}
	if len(logicalHashes) != 2 || logicalHashes[0] != logicalHashes[1] {
		t.Fatalf("provider routes did not share one logical memory selection: %v", logicalHashes)
	}
}

func TestBuildPrepareTurnMemoryTransportProviderManagerDelegatesPDFCreation(t *testing.T) {
	payloadPlan := prepareTurnMemoryTransportFixture()
	called := false
	plan, transient := buildPrepareTurnMemoryTransport("provider_manager_pdf", payloadPlan, "request-provider-manager", func(string) (pdfmemory.Document, error) {
		called = true
		return pdfmemory.Document{}, nil
	})
	if called {
		t.Fatal("Provider Manager mode must not build a second Go PDF")
	}
	if transient != nil {
		t.Fatalf("Provider Manager mode unexpectedly returned Go PDF bytes: %#v", transient)
	}
	if plan["selected_mode"] != "provider_manager_pdf" ||
		plan["body_format"] != "provider_manager_manual_pdf" ||
		plan["transport_status"] != "provider_manager_marker_ready" ||
		plan["generation_status"] != "delegated_to_provider_manager" {
		t.Fatalf("unexpected Provider Manager plan: %#v", plan)
	}
	if plan["long_term_memory_text"] != "[Long-term Memory]\n기억 문장" ||
		plan["auxiliary_without_long_term_memory"] != "[Original Work]\ncanon\n\n[Output Guidance]\ncontinue" ||
		plan["text_baseline_retained"] != true {
		t.Fatalf("Provider Manager mode changed the selected memory or Text baseline: %#v", plan)
	}
}

func TestBuildPrepareTurnMemoryTransportLegacyPDFMapsToGoogle(t *testing.T) {
	plan, _ := buildPrepareTurnMemoryTransport("pdf", prepareTurnMemoryTransportFixture(), "legacy", pdfmemory.Generate)
	if plan["requested_mode"] != "google_pdf" || plan["selected_mode"] != "google_pdf" {
		t.Fatalf("legacy PDF compatibility mapping failed: %#v", plan)
	}
}

func TestBuildPrepareTurnMemoryTransportFailureKeepsText(t *testing.T) {
	payloadPlan := prepareTurnMemoryTransportFixture()
	plan, transient := buildPrepareTurnMemoryTransport("google_pdf", payloadPlan, "request-fail", func(string) (pdfmemory.Document, error) {
		return pdfmemory.Document{}, pdfmemory.ErrFontLoad
	})
	if transient != nil || plan["selected_mode"] != "text" || plan["transport_status"] != "pdf_build_failed" {
		t.Fatalf("PDF failure did not preserve text mode: plan=%#v transient=%#v", plan, transient)
	}
	if plan["error_code"] != "memory_pdf_font_load_failed" || plan["auxiliary_text"] != payloadPlan["auxiliary_text"] {
		t.Fatalf("failure lost the text baseline: %#v", plan)
	}
}

func TestBuildPrepareTurnMemoryTransportEmptyAndPlanIdentity(t *testing.T) {
	payloadPlan := prepareTurnMemoryTransportFixture()
	lanes := payloadPlan["lanes"].([]map[string]any)
	lanes[1]["applied"] = false
	lanes[1]["text"] = ""
	plan, transient := buildPrepareTurnMemoryTransport("llm_gateway_pdf", payloadPlan, "request-empty", pdfmemory.Generate)
	if transient != nil || plan["selected_mode"] != "text" || plan["generation_status"] != "empty" || plan["error_code"] != "memory_pdf_empty" {
		t.Fatalf("unexpected empty plan: %#v transient=%#v", plan, transient)
	}

	baseline := prepareTurnMemoryTransportFixture()
	first, _ := buildPrepareTurnMemoryTransport("google_pdf", baseline, "request-stable", pdfmemory.Generate)
	second, _ := buildPrepareTurnMemoryTransport("google_pdf", baseline, "request-stable", pdfmemory.Generate)
	third, _ := buildPrepareTurnMemoryTransport("google_pdf", baseline, "request-other", pdfmemory.Generate)
	if !reflect.DeepEqual(first["plan_id"], second["plan_id"]) || reflect.DeepEqual(first["plan_id"], third["plan_id"]) {
		t.Fatalf("plan identity mismatch: first=%v second=%v third=%v", first["plan_id"], second["plan_id"], third["plan_id"])
	}
}

func TestPrepareTurnMemoryTransportErrorCodes(t *testing.T) {
	tests := map[error]string{
		pdfmemory.ErrEmptyText:       "memory_pdf_empty",
		pdfmemory.ErrInvalidUTF8:     "memory_pdf_invalid_utf8",
		pdfmemory.ErrUnsupportedText: "memory_pdf_unsupported_text",
		pdfmemory.ErrFontLoad:        "memory_pdf_font_load_failed",
		pdfmemory.ErrRender:          "memory_pdf_render_failed",
		errors.New("other"):          "memory_pdf_build_failed",
	}
	for err, want := range tests {
		if got := prepareTurnMemoryTransportPDFErrorCode(err); got != want {
			t.Fatalf("error %v => %q, want %q", err, got, want)
		}
	}
}

func TestPrepareTurnMemoryTransportHTTPPreservesTextPlanAndScopesTransientPayload(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "memory-transport-http", TurnIndex: 1, SummaryJSON: `{"turn_summary":"The old semantic oath must remain exact."}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 1, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:memory-transport-http:1", Tier: "memory", ChatSessionID: "memory-transport-http", SourceTable: "memories", SourceRowID: "1", DocumentText: "old semantic oath", Similarity: 0.91, SimilarityAvailable: true, SimilaritySource: "fixture"},
		},
	}

	request := func(mode string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		body := `{
			"chat_session_id":"memory-transport-http",
			"response_projection":"prepare_turn.production_compact.v1",
			"turn_index":2,
			"raw_user_input":"Continue the old semantic oath.",
			"client_meta":{"chroma_query_vector":[0.1,0.2]},
			"settings":{"memory_transport_mode":"` + mode + `","guide_strength":"none","max_injection_chars":1600,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
		}`
		mux := http.NewServeMux()
		srv.RegisterRoutes(mux)
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("prepare-turn %s status=%d body=%s", mode, rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode prepare-turn %s response: %v", mode, err)
		}
		return rec, response
	}

	_, textResponse := request("text")
	googleRecorder, googleResponse := request("google_pdf")
	_, gatewayResponse := request("llm_gateway_pdf")
	_, providerManagerResponse := request("provider_manager_pdf")
	if !reflect.DeepEqual(textResponse["payload_application_plan"], googleResponse["payload_application_plan"]) ||
		!reflect.DeepEqual(textResponse["payload_application_plan"], gatewayResponse["payload_application_plan"]) ||
		!reflect.DeepEqual(textResponse["payload_application_plan"], providerManagerResponse["payload_application_plan"]) {
		t.Fatal("PDF mode changed payload_application_plan.v1 instead of retaining the text baseline")
	}
	for mode, response := range map[string]map[string]any{
		"google_pdf":      googleResponse,
		"llm_gateway_pdf": gatewayResponse,
	} {
		plan := mapFromAny(response["memory_transport_plan"])
		transient := mapFromAny(response["memory_transport_payload"])
		if plan["contract_version"] != prepareTurnMemoryTransportPlanContract || plan["selected_mode"] != mode || plan["transport_status"] != "pdf_ready" {
			t.Fatalf("HTTP %s transport plan incomplete: %#v", mode, plan)
		}
		if transient["contract_version"] != prepareTurnMemoryTransportPayloadContract || transient["plan_id"] != plan["plan_id"] {
			t.Fatalf("HTTP %s transient payload is not plan-bound", mode)
		}
		encoded := extractionStringFromAny(transient["pdf_base64"])
		if encoded == "" || len(encoded) != intFromAny(plan["base64_chars"], 0) {
			t.Fatalf("HTTP %s transient base64 missing or wrong size", mode)
		}
		for _, key := range []string{"trace_preview", "input_transparency_model", "injection_pack"} {
			data, err := json.Marshal(response[key])
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), encoded) || strings.Contains(string(data), "pdf_base64") {
				t.Fatalf("%s leaked transient PDF base64 for %s", key, mode)
			}
		}
	}
	providerManagerPlan := mapFromAny(providerManagerResponse["memory_transport_plan"])
	if providerManagerPlan["selected_mode"] != "provider_manager_pdf" ||
		providerManagerPlan["transport_status"] != "provider_manager_marker_ready" ||
		providerManagerResponse["memory_transport_payload"] != nil {
		t.Fatalf("HTTP Provider Manager transport plan was not marker-only: plan=%#v payload=%#v", providerManagerPlan, providerManagerResponse["memory_transport_payload"])
	}

	var base64Paths []string
	var walk func(any, string)
	walk = func(value any, path string) {
		switch item := value.(type) {
		case map[string]any:
			for key, child := range item {
				next := key
				if path != "" {
					next = path + "." + key
				}
				if key == "pdf_base64" {
					base64Paths = append(base64Paths, next)
				}
				walk(child, next)
			}
		case []any:
			for index, child := range item {
				walk(child, path+"["+fmt.Sprint(index)+"]")
			}
		}
	}
	walk(googleResponse, "")
	if !reflect.DeepEqual(base64Paths, []string{"memory_transport_payload.pdf_base64"}) {
		t.Fatalf("transient PDF base64 appeared outside its response envelope: %v", base64Paths)
	}
	if strings.Count(googleRecorder.Body.String(), `"pdf_base64"`) != 1 {
		t.Fatal("wire response must expose pdf_base64 exactly once")
	}
}
