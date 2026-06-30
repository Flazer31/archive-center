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

func TestTableReadOutputEnhanceWithoutLLMConfigReturnsOriginalDraft(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-enhance",
		SourceTurn:          3,
		MemoryText:          "Chloe's private recollection should stay as subtext.",
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
		"chat_session_id":"sess-enhance",
		"turn_index":4,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo checks Chloe's expression.",
		"assistant_draft":"Chloe smiled without explaining herself.",
		"output_check_context":{"verdict":"minor_revise","active_entities":["Chloe"]},
		"mini_read_context":{"moderator_summary":"Keep Chloe's intent as subtext."},
		"max_memories_per_entity":4,
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-enhance", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputEnhance3ContractVersion ||
		resp["llm_call_attempted"] != false ||
		resp["write_attempted"] != false ||
		resp["replaces_output"] != true ||
		resp["route_can_replace_output"] != true ||
		resp["candidate_generation"] != false ||
		resp["assistant_output_final"] != "Chloe smiled without explaining herself." ||
		resp["changed"] != false ||
		resp["fallback_reason"] != "llm_not_configured" {
		t.Fatalf("output-enhance fail-open mismatch: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	guards := tableRead["guards"].(map[string]any)
	if guards["output_replacement"] != true || guards["candidate_generation"] != false || guards["fallback_to_original"] != true {
		t.Fatalf("guards mismatch: %+v", guards)
	}
}

func TestTableReadOutputEnhanceReturnsFinalOutputWithoutWriting(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	for _, memory := range []*store.ProtagonistEntityMemory{
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "sess-enhance-live",
			SourceTurn:          3,
			MemoryText:          "Chloe privately enjoys teasing Siwoo, but her intent must remain ambiguous.",
			SecretGuard:         true,
			Portability:         "npc_private_recollection",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        8,
			EmotionalWeight:     0.8,
		},
		{
			OwnerEntityKey:      "siwoo",
			OwnerEntityName:     "Siwoo",
			OwnerEntityRole:     "protagonist",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "sess-enhance-live",
			SourceTurn:          3,
			MemoryText:          "Siwoo is embarrassed, but he still tries to take care of Chloe.",
			SecretGuard:         true,
			Portability:         "persona_recollection",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        7,
			EmotionalWeight:     0.7,
		},
		{
			OwnerEntityKey:      "saori",
			OwnerEntityName:     "Saori",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "sess-enhance-live",
			SourceTurn:          9,
			MemoryText:          "Saori's unrelated memory should not enter this Chloe scene.",
			SecretGuard:         true,
			Portability:         "npc_private_recollection",
			TargetRevealPolicy:  "owner_private_until_revealed",
			Importance10:        10,
			EmotionalWeight:     1.0,
		},
	} {
		if _, err := fake.CreateProtagonistEntityMemory(context.Background(), memory); err != nil {
			t.Fatalf("seed subjective memory: %v", err)
		}
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
			"model": "table-read-enhance-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"assistant_output_final":"Siwoo set the cup down carefully. Chloe answered with a faint smile, leaving him unsure whether she was amused or merely tired.","changed":true,"issues_repaired":["private_memory_leak","scene_flow"],"protected_reveals":["Chloe's private teasing intent remains subtext."],"entity_review_trace":[{"entity_name":"Chloe","concern":"Do not expose private intent as narrator fact.","applied_change":"Changed direct intent into ambiguous smile.","private_memory_reveal_blocked":true},{"entity_name":"Siwoo","concern":"Preserve his awkward care.","applied_change":"Restored the cup action before interpretation.","private_memory_reveal_blocked":true}],"fallback_reason":""}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 512},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-enhance-live",
		"turn_index":6,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo sets the cup down and studies Chloe's expression.",
		"assistant_draft":"Chloe intentionally teased Siwoo because she knew he liked her.",
		"recent_context_summary":"Kitchen scene, quiet tension.",
		"output_check_context":{"verdict":"major_revise","active_entities":["Chloe","Siwoo"],"issues":["private_memory_leak"]},
		"mini_read_context":{"moderator_summary":"Repair direct intent into subtext while preserving Siwoo's action.","output_enhance_notes":["Restore user action first.","Keep Chloe's inner motive ambiguous."]},
		"max_memories_per_entity":4,
		"max_entities":3,
		"entities":[
			{"entity_key":"chloe","entity_name":"Chloe","role":"npc"},
			{"entity_key":"siwoo","entity_name":"Siwoo","role":"protagonist"},
			{"entity_key":"saori","entity_name":"Saori","role":"npc"}
		],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-enhance-model","provider":"openai","max_tokens":800,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-enhance", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputEnhance3ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["replaces_output"] != true ||
		resp["route_can_replace_output"] != true ||
		resp["candidate_generation"] != false ||
		resp["changed"] != true {
		t.Fatalf("output-enhance live guard mismatch: %+v", resp)
	}
	wantFinal := "Siwoo set the cup down carefully. Chloe answered with a faint smile, leaving him unsure whether she was amused or merely tired."
	if resp["assistant_output_final"] != wantFinal {
		t.Fatalf("assistant_output_final mismatch: %+v", resp["assistant_output_final"])
	}
	selected := resp["selected_entities"].([]any)
	if len(selected) != 2 {
		t.Fatalf("selected entities = %#v", selected)
	}
	tableRead := resp["table_read"].(map[string]any)
	outputEnhance := tableRead["output_enhance"].(map[string]any)
	if outputEnhance["mode"] != "pre_output_final_rewrite" ||
		outputEnhance["truth_authority"] != false ||
		outputEnhance["write_attempted"] != false ||
		outputEnhance["replaces_output"] != true ||
		outputEnhance["candidate_generation"] != false ||
		outputEnhance["parse_status"] != "ok" {
		t.Fatalf("output_enhance surface mismatch: %+v", outputEnhance)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	content := messages[0].(map[string]any)["content"].(string) + "\n" + messages[1].(map[string]any)["content"].(string)
	for _, want := range []string{"TR-OUT-3", "assistant_output_final", "output_check_context", "mini_read_context", "Chloe privately enjoys teasing", "Siwoo is embarrassed", "Do not treat mini_discussion as scene dialogue", "moderator_summary and output_enhance_notes as constraints", "Do not add new events"} {
		if !strings.Contains(content, want) {
			t.Fatalf("output-enhance prompt missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, "Saori's unrelated memory") {
		t.Fatalf("output-enhance prompt included unrelated Saori memory: %s", content)
	}
}

func TestTableReadOutputEnhanceMalformedLLMResponseReturnsOriginalDraft(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "table-read-enhance-model",
			"choices": []any{map[string]any{
				"message": map[string]any{"content": "Here is a better version, but this is not JSON."},
			}},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = newPersonaRouteFakeStore()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-enhance-bad-json",
		"turn_index":7,
		"scene_text":"Chloe and Siwoo are here.",
		"user_input":"Siwoo waits.",
		"assistant_draft":"Chloe watches Siwoo.",
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-enhance-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/output-enhance", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadOutputEnhance3ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["assistant_output_final"] != "Chloe watches Siwoo." ||
		resp["changed"] != false ||
		resp["fallback_reason"] != "llm_response_not_json" {
		t.Fatalf("malformed output-enhance should return original: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	outputEnhance := tableRead["output_enhance"].(map[string]any)
	if outputEnhance["parse_status"] != "raw_text_only" || outputEnhance["replaces_output"] != true || outputEnhance["write_attempted"] != false {
		t.Fatalf("malformed output-enhance surface mismatch: %+v", outputEnhance)
	}
}
