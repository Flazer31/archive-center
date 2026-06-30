package httpapi

import (
	"bytes"
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

func TestTableReadDraftBindsSubjectiveEntityMemoriesWithoutCallingLLM(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-tr1",
		SourceTurn:          4,
		MemoryText:          "Chloe privately suspects Siwoo knows more than he admits.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        8,
		EmotionalWeight:     0.7,
	})
	if err != nil {
		t.Fatalf("seed subjective memory: %v", err)
	}

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{
		"chat_session_id":"sess-tr1",
		"scene_text":"Siwoo pauses before the locked drawer while Chloe watches.",
		"user_input":"시우는 클로에의 반응을 살핀다.",
		"max_memories_per_entity":4,
		"multi_model":{"enabled":true,"mode":"parallel_agents_dry_run","max_parallel":2,"require_consensus":true},
		"entities":[
			{"entity_key":"chloe","entity_name":"Chloe","role":"npc","provider":"anthropic","model":"claude-opus"},
			{"entity_key":"siwoo","entity_name":"Siwoo","role":"protagonist","provider":"openai","model":"gpt-4.1"}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/table-read/draft", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadTR1ContractVersion || resp["dry_run_only"] != true || resp["would_call_llm"] != false {
		t.Fatalf("TR-1 response guard mismatch: %+v", resp)
	}
	tableRead, ok := resp["table_read"].(map[string]any)
	if !ok {
		t.Fatalf("table_read missing: %+v", resp)
	}
	agents, ok := tableRead["agents"].([]any)
	if !ok || len(agents) != 2 {
		t.Fatalf("agents = %#v", tableRead["agents"])
	}
	chloe := agents[0].(map[string]any)
	if chloe["entity_key"] != "chloe" || chloe["memory_count"] != float64(1) {
		t.Fatalf("chloe agent did not bind memory: %+v", chloe)
	}
	policy := chloe["private_memory_policy"].(map[string]any)
	if policy["lane"] != "character_private_recollection" || policy["truth_authority"] != false {
		t.Fatalf("npc private policy mismatch: %+v", policy)
	}
	orchestration := tableRead["orchestration"].(map[string]any)
	if orchestration["multi_model_supported"] != true || orchestration["multi_model_enabled"] != true {
		t.Fatalf("multi-model plan not surfaced: %+v", orchestration)
	}
	if orchestration["tr1_execution_guard"] != "no_llm_call_in_tr1" {
		t.Fatalf("TR-1 should not execute LLM calls: %+v", orchestration)
	}
}

func TestTableReadSimulateCallsSingleModelAndKeepsSupportOnly(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-tr2",
		SourceTurn:          7,
		MemoryText:          "Chloe keeps her suspicion private and avoids direct disclosure.",
		SecretGuard:         true,
		Portability:         "npc_private_recollection",
		TargetRevealPolicy:  "owner_private_until_revealed",
		Importance10:        7,
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
			"model": "table-read-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"agent_notes":[{"entity_name":"Chloe","private_read":"She is wary.","concern":"Do not expose the secret.","desired_direction":"Keep it subtle."}],"discussion":["Chloe should react cautiously."],"moderator_summary":"Use the private memory only as subtext.","story_hints":["Let Chloe hesitate."],"blocked_reveals":["Do not narrate the private suspicion as fact."]}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 123},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-tr2",
		"scene_text":"Chloe watches Siwoo near the drawer.",
		"user_input":"시우는 클로에가 왜 망설이는지 살핀다.",
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/simulate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadTR2ContractVersion || resp["llm_call_attempted"] != true || resp["write_attempted"] != false {
		t.Fatalf("TR-2 response guard mismatch: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	simulation := tableRead["simulation"].(map[string]any)
	if simulation["mode"] != "single_model_table_read" || simulation["truth_authority"] != false || simulation["parse_status"] != "ok" {
		t.Fatalf("simulation guard mismatch: %+v", simulation)
	}
	if _, ok := simulation["parsed_json"].(map[string]any); !ok {
		t.Fatalf("parsed_json missing: %+v", simulation)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	userMsg := messages[1].(map[string]any)
	if !strings.Contains(userMsg["content"].(string), "subjective_memory") && !strings.Contains(userMsg["content"].(string), "Chloe keeps her suspicion") {
		t.Fatalf("upstream prompt did not include subjective memory context: %s", userMsg["content"])
	}
}

func TestTableReadSimulateRequiresExplicitProvider(t *testing.T) {
	srv := NewServer(config.Config{})
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-tr-provider",
		"scene_text":"A quiet scene.",
		"user_input":"continue",
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":"https://api.example.com/v1","model":"table-read-model","max_tokens":512,"temperature":0.2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/table-read/simulate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "llm.provider") {
		t.Fatalf("provider missing error not surfaced: %s", rec.Body.String())
	}
}

func TestTableReadReviewCallsSingleModelButDoesNotReplaceOutput(t *testing.T) {
	fake := newPersonaRouteFakeStore()
	_, err := fake.CreateProtagonistEntityMemory(context.Background(), &store.ProtagonistEntityMemory{
		OwnerEntityKey:      "chloe",
		OwnerEntityName:     "Chloe",
		OwnerEntityRole:     "npc",
		OwnerVisibility:     "owner_private",
		SourceChatSessionID: "sess-review",
		SourceTurn:          5,
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
			"model": "table-read-review-model",
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": `{"verdict":"minor_revise","character_reviews":[{"entity_name":"Chloe","voice_fit":"ok","knowledge_leak":false,"private_memory_leak":true,"emotion_fit":"too_direct","concern":"The draft exposes Chloe's private amusement too plainly."}],"story_continuity":{"location_ok":true,"time_ok":true,"relationship_ok":true,"scene_flow_ok":true,"notes":"Keep the reaction as subtext."},"protected_reveals":["Do not state Chloe enjoyed teasing as narrator fact."],"revision_notes":["Make Chloe's amusement visible through gesture, not exposition."],"safe_to_publish":false}`,
				},
			}},
			"usage": map[string]any{"total_tokens": 234},
		})
	}))
	defer upstream.Close()

	srv := NewServer(config.Config{})
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := fmt.Sprintf(`{
		"chat_session_id":"sess-review",
		"scene_text":"Chloe and Siwoo stand near the kitchen counter.",
		"user_input":"시우는 물컵을 내려놓으며 클로에를 살폈다.",
		"assistant_draft":"클로에는 시우를 일부러 놀리고 있다는 사실을 즐기며 웃었다.",
		"entities":[{"entity_key":"chloe","entity_name":"Chloe","role":"npc"}],
		"llm":{"api_key":"sk-test","endpoint":%q,"model":"table-read-review-model","provider":"openai","max_tokens":512,"temperature":0.2}
	}`, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/table-read/review", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != tableReadReview1ContractVersion || resp["llm_call_attempted"] != true || resp["write_attempted"] != false || resp["replaces_output"] != false {
		t.Fatalf("TR-Review-1 response guard mismatch: %+v", resp)
	}
	tableRead := resp["table_read"].(map[string]any)
	guards := tableRead["guards"].(map[string]any)
	if guards["review_only"] != true || guards["output_replacement"] != false || guards["private_memory_reveal"] != "forbidden_in_final_output" {
		t.Fatalf("review guards mismatch: %+v", guards)
	}
	review := tableRead["review"].(map[string]any)
	if review["mode"] != "assistant_draft_read_only_table_read_review" || review["truth_authority"] != false || review["review_only"] != true || review["replaces_output"] != false {
		t.Fatalf("review surface mismatch: %+v", review)
	}
	if _, ok := review["parsed_json"].(map[string]any); !ok {
		t.Fatalf("parsed_json missing: %+v", review)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("upstream messages missing: %+v", upstreamBody)
	}
	userMsg := messages[1].(map[string]any)
	content := userMsg["content"].(string)
	if !strings.Contains(content, "assistant_draft") || !strings.Contains(content, "클로에는 시우를 일부러") || !strings.Contains(content, "Chloe privately enjoys teasing") {
		t.Fatalf("review prompt did not include draft and subjective memory context: %s", content)
	}
}
