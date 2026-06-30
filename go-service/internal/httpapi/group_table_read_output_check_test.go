package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestTableReadOutputCheckWithoutLLMConfigFailsOpen(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-out-check",
		SourceTurn:          3,
		MemoryText:          "Chloe privately doubts whether direct narration should expose her intent.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        7,
		EmotionalWeight:     0.7,
	})
	if err != nil {
		t.Fatalf("seed subjective memory: %v", err)
	}

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-out-check",
		"turn_index":4,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo checks Chloe's expression.",
		"assistant_draft":"Chloe smiled without explaining herself.",
		"max_memories_per_entity":4,
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-check", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputCheck1ContractVersion ||
		resp["llm_call_attempted"] != false ||
		resp["write_attempted"] != false ||
		resp["replaces_output"] != false ||
		resp["route_can_replace_output"] != false ||
		resp["verdict"] != "accept" ||
		resp["requires_table_read"] != false ||
		resp["requires_output_enhance"] != false ||
		resp["fallback_reason"] != "llm_not_configured" {
		t.Fatalf("output-check fail-open guard mismatch: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	agents := tableRead["agents"].([]any)
	if len(agents) != 1 {
		t.Fatalf("agents = %#v", agents)
	}
	chloe := agents[0].(map[string]any)
	if chloe["entity_key"] != "chloe" || chloe["memory_count"] != float64(1) {
		t.Fatalf("subjective memory not bound: %+v", chloe)
	}
	guards := tableRead["guards"].(map[string]any)
	if guards["output_replacement"] != false || guards["candidate_generation"] != false || guards["fail_open"] != true {
		t.Fatalf("guards mismatch: %+v", guards)
	}
}

func TestTableReadOutputCheckCallsLLMButOnlyReturnsDecisionSignals(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-out-check-live",
		SourceTurn:          5,
		MemoryText:          "Chloe privately enjoys teasing Siwoo, but the final narration must keep it as subtext.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        8,
		EmotionalWeight:     0.8,
	})
	if err != nil {
		t.Fatalf("seed subjective memory: %v", err)
	}

	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("upstream path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "table-read-output-check-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"verdict":"major_revise","requires_table_read":true,"requires_output_enhance":true,"issues":["private_memory_leak","voice_mismatch"],"active_entities":["Chloe","Siwoo"],"protected_reveals":["Chloe's private teasing intent"],"fallback_reason":""}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 321},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-out-check-live",
		"turn_index":6,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo sets the cup down and studies Chloe's expression.",
		"assistant_draft":"Chloe intentionally teased Siwoo because she knew he liked her.",
		"recent_context_summary":"Kitchen scene, quiet tension.",
		"max_memories_per_entity":4,
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-output-check-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-check", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputCheck1ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["replaces_output"] != false ||
		resp["route_can_replace_output"] != false ||
		resp["verdict"] != "major_revise" ||
		resp["requires_table_read"] != true ||
		resp["requires_output_enhance"] != true {
		t.Fatalf("output-check live guard mismatch: %+v", resp)
	}
	issues := resp["issues"].([]any)
	if len(issues) != 2 {
		t.Fatalf("issues mismatch: %+v", resp["issues"])
	}
	tableRead := resp["table_read"].(map[string]any)
	outputCheck := tableRead["output_check"].(map[string]any)
	if outputCheck["mode"] != "pre_output_decision_only" ||
		outputCheck["truth_authority"] != false ||
		outputCheck["write_attempted"] != false ||
		outputCheck["replaces_output"] != false ||
		outputCheck["candidate_generation"] != false ||
		outputCheck["parse_status"] != "ok" {
		t.Fatalf("output_check surface mismatch: %+v", outputCheck)
	}
	if _, ok := outputCheck["parsed_json"].(map[string]any); !ok {
		t.Fatalf("parsed_json missing: %+v", outputCheck)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	content := messages[0].(map[string]any)["content"].(string) + "\n" + messages[1].(map[string]any)["content"].(string)
	for _, want := range []string{"TR-OUT-1", "assistant_draft", "forbidden_response_fields", "Chloe privately enjoys teasing"} {
		if !strings.Contains(content, want) {
			t.Fatalf("output-check prompt missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, "revised_draft\":\"the suggested replacement") {
		t.Fatalf("output-check prompt leaked revision schema: %s", content)
	}
}

func TestTableReadOutputCheckMalformedLLMResponseFailsOpen(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "table-read-output-check-model",
			"choices": []any{map[string]any{
				"message": map[string]any{"content": "This draft seems okay, but this is not JSON."},
			}},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = newPersonaRouteFakeStore()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-out-check-bad-json",
		"turn_index":7,
		"scene_text":"A short scene.",
		"user_input":"Continue.",
		"assistant_draft":"A short draft.",
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-output-check-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-check", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputCheck1ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["verdict"] != "accept" ||
		resp["requires_table_read"] != false ||
		resp["requires_output_enhance"] != false ||
		resp["fallback_reason"] != "llm_response_not_json" {
		t.Fatalf("malformed output-check should fail open: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	outputCheck := tableRead["output_check"].(map[string]any)
	if outputCheck["parse_status"] != "raw_text_only" || outputCheck["replaces_output"] != false {
		t.Fatalf("malformed output-check surface mismatch: %+v", outputCheck)
	}
}
