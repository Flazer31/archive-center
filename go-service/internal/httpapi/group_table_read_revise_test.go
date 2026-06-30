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

func TestTableReadReviseSuggestsDraftButDoesNotReplaceOutput(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-revise",
		SourceTurn:          6,
		MemoryText:          "Chloe privately enjoys teasing Siwoo but should not directly confess it.",
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
			"model": "table-read-revise-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"verdict":"major_revise","revision_strategy":"Keep Chloe's intention as subtext and restore Siwoo's action.","revised_draft":"Siwoo set the cup down carefully. Chloe watched him with a faint, unreadable smile.","change_notes":["Restored user action","Removed explicit private intent"],"protected_reveals_preserved":true,"remaining_risks":[]}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 345},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-revise",
		"scene_text":"Chloe and Siwoo stand near the kitchen counter.",
		"user_input":"Siwoo sets the cup down and studies Chloe's expression.",
		"assistant_draft":"Chloe intentionally teased Siwoo because she knew he liked her.",
		"review_context":{"verdict":"major_revise","revision_notes":["Restore the user action.","Keep Chloe's private intent as subtext."]},
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-revise-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/revise", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadReview2ContractVersion || resp["llm_call_attempted"] != true || resp["write_attempted"] != false || resp["replaces_output"] != false {
		t.Fatalf("TR-Review-2 response guard mismatch: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	guards := tableRead["guards"].(map[string]any)
	if guards["revision_suggestion_only"] != true || guards["output_replacement"] != false || guards["auto_apply"] != false || guards["copy_only"] != true {
		t.Fatalf("revision guards mismatch: %+v", guards)
	}
	revision := tableRead["revision"].(map[string]any)
	if revision["mode"] != "assistant_draft_revision_suggestion" || revision["truth_authority"] != false || revision["copy_only"] != true || revision["replaces_output"] != false {
		t.Fatalf("revision surface mismatch: %+v", revision)
	}
	parsed, ok := revision["parsed_json"].(map[string]any)
	if !ok {
		t.Fatalf("parsed_json missing: %+v", revision)
	}
	if parsed["revised_draft"] != "Siwoo set the cup down carefully. Chloe watched him with a faint, unreadable smile." {
		t.Fatalf("unexpected revised_draft: %+v", parsed)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	userMsg := messages[1].(map[string]any)
	content := userMsg["content"].(string)
	for _, want := range []string{"review_context", "revised_draft", "assistant_draft", "Chloe privately enjoys teasing"} {
		if !strings.Contains(content, want) {
			t.Fatalf("revision prompt missing %q: %s", want, content)
		}
	}
}

func TestTableReadPolishRouteContractReturnsFinalOutputWithoutWriteOrLLM(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-polish",
		SourceTurn:          8,
		MemoryText:          "Chloe privately worries that direct exposition will expose too much.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        8,
		EmotionalWeight:     0.6,
	})
	if err != nil {
		t.Fatalf("seed subjective memory: %v", err)
	}

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-polish",
		"turn_index":9,
		"scene_text":"Chloe reads the generated scene in silence.",
		"user_input":"Siwoo watches Chloe's face for a reaction.",
		"assistant_output_original":"Chloe smiled faintly, leaving Siwoo unsure what she really meant.",
		"recent_context_summary":"Kitchen scene, quiet tension.",
		"review_context":{"verdict":"minor_revise"},
		"max_memories_per_entity":4,
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"multi_model":{"enabled":true,"mode":"single_model_contract_only","max_parallel":1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/table-read/polish", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadPolish2ContractVersion ||
		resp["llm_call_attempted"] != false ||
		resp["write_attempted"] != false ||
		resp["route_can_replace_output"] != true ||
		resp["changed"] != false {
		t.Fatalf("TR-POLISH-2 response guard mismatch: %+v", resp)
	}
	if resp["assistant_output_final"] != "Chloe smiled faintly, leaving Siwoo unsure what she really meant." {
		t.Fatalf("assistant_output_final should pass through original output: %+v", resp)
	}
	if resp["fallback_reason"] != "tr_polish_2_route_contract_only" {
		t.Fatalf("fallback_reason mismatch: %+v", resp)
	}

	tableRead := resp["table_read"].(map[string]any)
	if tableRead["mode"] != "output_polish_route_contract" {
		t.Fatalf("table_read mode mismatch: %+v", tableRead)
	}
	agents := tableRead["agents"].([]any)
	if len(agents) != 1 {
		t.Fatalf("agents = %#v", tableRead["agents"])
	}
	chloe := agents[0].(map[string]any)
	if chloe["entity_key"] != "chloe" || chloe["memory_count"] != float64(1) {
		t.Fatalf("chloe subjective memory not bound: %+v", chloe)
	}
	guards := tableRead["guards"].(map[string]any)
	if guards["canonical_truth_write"] != false ||
		guards["memory_write"] != false ||
		guards["kg_write"] != false ||
		guards["direct_evidence_write"] != false ||
		guards["fallback_to_original"] != true {
		t.Fatalf("polish guards mismatch: %+v", guards)
	}
	trace := resp["entity_review_trace"].([]any)[0].(map[string]any)
	if trace["status"] != "not_run" || trace["truth_authority"] != false {
		t.Fatalf("entity review trace should be not_run support-only: %+v", trace)
	}
}

func TestTableReadPolishLiveLLMReturnsFinalOutputWithoutWriting(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-polish-live",
		SourceTurn:          8,
		MemoryText:          "Chloe privately wants to tease Siwoo, but the narration must keep it as subtext.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        8,
		EmotionalWeight:     0.6,
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
			"model": "table-read-polish-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"assistant_output_final":"Siwoo set the cup down carefully. Chloe answered with a faint smile, leaving him unsure whether she was amused or merely tired.","changed":true,"issues":["private_memory_exposition"],"protected_reveals":["Chloe's private teasing intent remains subtext."],"entity_review_trace":[{"entity_name":"Chloe","concern":"Do not expose private intent as narrator fact.","applied_change":"Changed direct intent into ambiguous smile.","private_memory_reveal_blocked":true}],"fallback_reason":""}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 456},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-polish-live",
		"turn_index":10,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo sets the cup down and checks Chloe's expression.",
		"assistant_output_original":"Chloe intentionally teased Siwoo because she knew he liked her.",
		"recent_context_summary":"Kitchen scene, quiet tension.",
		"review_context":{"verdict":"major_revise","protected_reveals":["private teasing intent"]},
		"max_memories_per_entity":4,
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-polish-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/polish", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadPolish2ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["route_can_replace_output"] != true ||
		resp["changed"] != true {
		t.Fatalf("TR-POLISH-3 response guard mismatch: %+v", resp)
	}
	if resp["assistant_output_final"] != "Siwoo set the cup down carefully. Chloe answered with a faint smile, leaving him unsure whether she was amused or merely tired." {
		t.Fatalf("assistant_output_final mismatch: %+v", resp)
	}
	protected := resp["protected_reveals"].([]any)
	if len(protected) != 1 {
		t.Fatalf("protected_reveals mismatch: %+v", resp["protected_reveals"])
	}
	trace := resp["entity_review_trace"].([]any)
	if len(trace) != 1 {
		t.Fatalf("entity_review_trace mismatch: %+v", resp["entity_review_trace"])
	}
	tableRead := resp["table_read"].(map[string]any)
	if tableRead["mode"] != "live_llm_output_polish" {
		t.Fatalf("table_read mode mismatch: %+v", tableRead)
	}
	guards := tableRead["guards"].(map[string]any)
	if guards["canonical_truth_write"] != false || guards["memory_write"] != false || guards["fallback_to_original"] != true {
		t.Fatalf("polish guards mismatch: %+v", guards)
	}
	polish := tableRead["polish"].(map[string]any)
	if polish["replaces_output"] != true || polish["write_attempted"] != false || polish["truth_authority"] != false || polish["parse_status"] != "ok" {
		t.Fatalf("polish surface mismatch: %+v", polish)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	content := messages[0].(map[string]any)["content"].(string) + "\n" + messages[1].(map[string]any)["content"].(string)
	for _, want := range []string{"assistant_output_final", "assistant_output_original", "Chloe privately wants to tease", "Never narrate NPC private recollection"} {
		if !strings.Contains(content, want) {
			t.Fatalf("polish prompt missing %q: %s", want, content)
		}
	}
}
