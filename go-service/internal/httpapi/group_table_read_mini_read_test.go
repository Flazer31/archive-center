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

func TestTableReadMiniReadSelectsOnlyCurrentSceneEntities(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	for _, memory := range []*store.ProtagonistEntityMemory{
		{
			OwnerEntityKey:      "chloe",
			OwnerEntityName:     "Chloe",
			OwnerEntityRole:     "npc",
			OwnerVisibility:     "owner_private",
			SourceChatSessionID: "sess-mini",
			SourceTurn:          3,
			MemoryText:          "Chloe privately enjoys teasing Siwoo, but it must remain subtext.",
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
			SourceChatSessionID: "sess-mini",
			SourceTurn:          3,
			MemoryText:          "Siwoo is embarrassed and tries to hide his concern for Chloe.",
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
			SourceChatSessionID: "sess-mini",
			SourceTurn:          9,
			MemoryText:          "Saori's unrelated memory must not be pulled into this Chloe scene.",
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
			"model": "table-read-mini-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"participant_notes":[{"entity_name":"Chloe","perspective":"private","concern":"Do not expose her teasing intent.","safe_direction":"Keep it as a smile."},{"entity_name":"Siwoo","perspective":"persona","concern":"His concern should be awkward but sincere.","safe_direction":"Show action before exposition."}],"mini_discussion":[{"speaker":"Chloe","stance":"private_memory_guard","comment":"Keep Chloe's teasing intent ambiguous instead of making it narrator fact."},{"speaker":"Siwoo","stance":"voice_fit_review","comment":"Keep Siwoo's embarrassment in gesture and pacing, not direct labels."}],"moderator_summary":"Repair direct intent into subtext while preserving Siwoo's action.","protected_reveals":["Chloe's private teasing intent"],"story_risks":["private_memory_leak"],"output_enhance_notes":["Restore user action first.","Keep Chloe's inner motive ambiguous."],"safe_to_enhance":true,"fallback_reason":""}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 432},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-mini",
		"turn_index":6,
		"scene_text":"Chloe and Siwoo stand in the kitchen.",
		"user_input":"Siwoo sets the cup down and studies Chloe's expression.",
		"assistant_draft":"Chloe intentionally teased Siwoo because she knew he liked her.",
		"recent_context_summary":"Kitchen scene, quiet tension.",
		"output_check_context":{"verdict":"major_revise","active_entities":["Chloe","Siwoo"],"issues":["private_memory_leak"]},
		"max_memories_per_entity":4,
		"max_entities":3,
		"entities":[
			{"entity_key":"chloe","entity_name":"Chloe","role":"npc"},
			{"entity_key":"siwoo","entity_name":"Siwoo","role":"protagonist"},
			{"entity_key":"saori","entity_name":"Saori","role":"npc"}
		],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-mini-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/mini-read", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadMiniRead2ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["replaces_output"] != false ||
		resp["route_can_replace_output"] != false ||
		resp["candidate_generation"] != false {
		t.Fatalf("mini-read guard mismatch: %+v", resp)
	}
	selected := resp["selected_entities"].([]any)
	if len(selected) != 2 {
		t.Fatalf("selected entities = %#v", selected)
	}
	tableRead := resp["table_read"].(map[string]any)
	agents := tableRead["agents"].([]any)
	if len(agents) != 2 {
		t.Fatalf("agents = %#v", agents)
	}
	miniRead := tableRead["mini_read"].(map[string]any)
	if miniRead["mode"] != "selected_entities_private_review_meeting" ||
		miniRead["truth_authority"] != false ||
		miniRead["write_attempted"] != false ||
		miniRead["replaces_output"] != false ||
		miniRead["candidate_generation"] != false ||
		miniRead["parse_status"] != "ok" {
		t.Fatalf("mini_read surface mismatch: %+v", miniRead)
	}
	if got := resp["moderator_summary"]; got != "Repair direct intent into subtext while preserving Siwoo's action." {
		t.Fatalf("moderator_summary = %#v", got)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	content := messages[0].(map[string]any)["content"].(string) + "\n" + messages[1].(map[string]any)["content"].(string)
	for _, want := range []string{"TR-OUT-2", "selected_agents", "Chloe privately enjoys teasing", "Siwoo is embarrassed", "deliberation, not roleplay", "Do not write in-character dialogue", "voice fit"} {
		if !strings.Contains(content, want) {
			t.Fatalf("mini-read prompt missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, "Saori's unrelated memory") {
		t.Fatalf("mini-read prompt included unrelated Saori memory: %s", content)
	}
	if strings.Contains(content, "revised_draft\":\"the suggested replacement") || strings.Contains(content, "assistant_output_final\":\"the final assistant response") {
		t.Fatalf("mini-read prompt leaked output replacement schema: %s", content)
	}
}

func TestTableReadMiniReadUsesOutputCheckActiveEntitiesWhenNamesAreNotInDraft(t *testing.T) {
	srv := NewServer(config.Config{})
	srv.Store = newPersonaRouteFakeStore()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-mini-active",
		"turn_index":2,
		"scene_text":"The room is quiet.",
		"user_input":"그는 조심스럽게 물컵을 내려놓았다.",
		"assistant_draft":"그녀는 작게 웃었다.",
		"output_check_context":{"active_entities":["클로에"]},
		"entities":[
			{"entity_key":"chloe","entity_name":"클로에","role":"npc"},
			{"entity_key":"saori","entity_name":"사오리","role":"npc"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/table-read/mini-read", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadMiniRead2ContractVersion ||
		resp["llm_call_attempted"] != false ||
		resp["fallback_reason"] != "llm_not_configured" {
		t.Fatalf("mini-read active entity fail-open mismatch: %+v", resp)
	}
	selected := resp["selected_entities"].([]any)
	if len(selected) != 1 {
		t.Fatalf("selected entities = %#v", selected)
	}
	first := selected[0].(map[string]any)
	if first["entity_name"] != "클로에" {
		t.Fatalf("selected entity = %+v", first)
	}
	trace := resp["relevance_trace"].([]any)
	if len(trace) != 2 {
		t.Fatalf("relevance_trace = %#v", trace)
	}
}

func TestTableReadMiniReadMalformedLLMResponseFailsOpen(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "table-read-mini-model",
			"choices": []any{map[string]any{
				"message": map[string]any{"content": "Chloe and Siwoo discuss the scene, but this is not JSON."},
			}},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = newPersonaRouteFakeStore()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-mini-bad-json",
		"turn_index":4,
		"scene_text":"Chloe and Siwoo are here.",
		"user_input":"Siwoo waits.",
		"assistant_draft":"Chloe watches Siwoo.",
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-mini-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/mini-read", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadMiniRead2ContractVersion ||
		resp["llm_call_attempted"] != true ||
		resp["write_attempted"] != false ||
		resp["fallback_reason"] != "llm_response_not_json" ||
		resp["safe_to_enhance"] != false {
		t.Fatalf("malformed mini-read should fail open: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	miniRead := tableRead["mini_read"].(map[string]any)
	if miniRead["parse_status"] != "raw_text_only" || miniRead["replaces_output"] != false {
		t.Fatalf("malformed mini-read surface mismatch: %+v", miniRead)
	}
}
