package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Only the external ranked index is replaced. The registered HTTP route owns
// hydration, eligibility, fact construction, optional editors and final packing.
type restorationRankedVector struct {
	vector.VectorStore
	t      *testing.T
	sid    string
	docs   []vector.VectorDocument
	limits []int
	mu     sync.Mutex
}

func (v *restorationRankedVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", ModelReady: true, TotalCount: len(v.docs)}, nil
}
func (v *restorationRankedVector) Search(_ context.Context, sid string, q []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	if sid != v.sid || len(q) != 3 || q[0] != 1 || limit <= 0 {
		v.t.Errorf("unexpected search: %s %v %d", sid, q, limit)
		return nil, fmt.Errorf("unexpected search")
	}
	if filter != `tier == "memory"` && filter != fmt.Sprintf("chat_session_id == %q", sid) {
		v.t.Errorf("unexpected scope: %s", filter)
		return nil, fmt.Errorf("unexpected scope")
	}
	v.mu.Lock()
	v.limits = append(v.limits, limit)
	v.mu.Unlock()
	return append([]vector.VectorDocument{}, v.docs[:minInt(limit, len(v.docs))]...), nil
}

func TestMemoryRestorationBudgetAndOptionMatrix(t *testing.T) {
	const target = "The group practiced ribbon binding together every evening."
	const query = "Let us resume our usual exercise."
	const question = "Find the established shared exercise."
	for _, editor := range []string{"off", "recommend", "empty", "failure", "supplement_failure"} {
		for _, publisher := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/publisher=%t", editor, publisher), func(t *testing.T) {
				t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
				const sid = "restoration-matrix"
				var mu sync.Mutex
				calls := map[string]int{}
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if input, ok := body["input"]; ok {
						if extractionStringFromAny(input) != question {
							t.Errorf("unexpected embedding: %v", input)
						}
						mu.Lock()
						calls["embedding"]++
						mu.Unlock()
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{1, .2, .3}, "index": 0}}})
						return
					}
					messages := sliceFromAny(body["messages"])
					if len(messages) != 2 {
						t.Errorf("unexpected messages: %v", body)
						http.Error(w, "unexpected", 400)
						return
					}
					var input map[string]any
					if err := json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input); err != nil {
						t.Error(err)
						return
					}
					if _, ok := input["supervisor_support_packet"]; ok {
						mu.Lock()
						calls["publisher"]++
						mu.Unlock()
						var refs []string
						for _, anchor := range sliceFromAny(mapFromAny(input["response_execution_contract"])["continuity_anchors"]) {
							refs = append(refs, stringsFromAny(mapFromAny(anchor)["source_refs"])...)
						}
						if len(refs) == 0 {
							t.Error("Publisher request has no continuity source refs")
							http.Error(w, "missing refs", 400)
							return
						}
						_, _ = w.Write([]byte(publisherV3OpenAIResponse(refs[0], "Continue the user's chosen activity.")))
						return
					}
					role := extractionStringFromAny(input["role"])
					_, second := input["previous_result"]
					mu.Lock()
					calls[fmt.Sprintf("%s/%t", role, second)]++
					mu.Unlock()
					if role != "event_recent" && role != "world_state" {
						t.Errorf("unexpected role %q", role)
						http.Error(w, "role", 400)
						return
					}
					if editor == "failure" && role == "event_recent" || editor == "supplement_failure" && second {
						http.Error(w, "recorded provider failure", 400)
						return
					}
					answer := multiAgentRecommendation{}
					if role == "event_recent" && editor != "empty" {
						for _, raw := range sliceFromAny(input["candidates"]) {
							c := modelEvidenceForTest(t, input, raw)
							if strings.Contains(extractionStringFromAny(c["text"]), target) {
								answer.SelectedIDs = append(answer.SelectedIDs, extractionStringFromAny(c["id"]))
							}
						}
						if len(answer.SelectedIDs) == 0 {
							t.Error("target beyond legacy K never reached the editor input")
						}
						if editor == "supplement_failure" {
							answer.SearchRequests = []string{question}
						}
					}
					encoded, _ := json.Marshal(answer)
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}}})
				}))
				defer provider.Close()
				cfg := config.Default()
				cfg.PromptDir = filepath.Join("..", "..", "..", "prompts")
				cfg.ChromaEndpoint = "http://offline-index.invalid"
				cfg.Readiness.ChromaConfigured = true
				srv := NewServer(cfg)
				mems := []store.Memory{}
				index := &restorationRankedVector{t: t, sid: sid}
				for id := 1; id <= 40; id++ {
					text := fmt.Sprintf("Stock ledger batch %d was filed.", id)
					importance := .3
					if id == 40 {
						text = target
						importance = .9
					}
					mems = append(mems, store.Memory{ID: int64(id), ChatSessionID: sid, TurnIndex: id, Importance: importance, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": text})})
					index.docs = append(index.docs, vector.VectorDocument{ID: fmt.Sprintf("memory:%s:%d", sid, id), ChatSessionID: sid, Tier: "memory", SourceTable: "memories", SourceRowID: fmt.Sprint(id), Similarity: .95 - float64(id)/1000, SimilarityAvailable: true, SimilaritySource: "cosine", Metadata: map[string]any{"source_revision": fmt.Sprintf("offline-rev-%d", id)}})
				}
				srv.Store = &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{returnMemories: mems}}}
				srv.StoreOpenError = nil
				srv.Vector = index
				srv.VectorOpenError = nil
				srv.RuntimeConfig.SupervisorProvider = "custom"
				srv.RuntimeConfig.SupervisorEndpoint = provider.URL
				srv.RuntimeConfig.SupervisorModel = "fixture"
				srv.RuntimeConfig.SupervisorAPIKey = "fixture"
				srv.RuntimeConfig.SupervisorTimeoutSec = 30
				srv.RuntimeConfig.EmbeddingProvider = "custom"
				srv.RuntimeConfig.EmbeddingEndpoint = provider.URL
				srv.RuntimeConfig.EmbeddingModel = "fixture"
				srv.RuntimeConfig.EmbeddingAPIKey = "fixture"
				srv.RuntimeConfig.EmbeddingTimeoutSec = 30
				settings := defaultMultiAgentSettings()
				settings.Enabled = editor != "off"
				for role, value := range settings.Roles {
					value.Enabled = role == "event_recent" || role == "world_state"
					value.UsePublisher = false
					value.Provider = "custom"
					value.Endpoint = provider.URL
					value.Model = "fixture"
					value.APIKey = "fixture"
					settings.Roles[role] = value
				}
				encoded, _ := json.Marshal(settings)
				rec := httptest.NewRecorder()
				srv.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(encoded)))
				if rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": sid, "turn_index": 41, "raw_user_input": query,
					"client_meta": map[string]any{"chroma_query_vector": []float64{1, .2, .3}},
					"settings":    map[string]any{"top_k": 1, "max_injection_chars": 18000, "input_context_enabled": false, "injection_enabled": true, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}})
				pack := mapFromAny(response["injection_pack"])
				plan := mapFromAny(pack["memory_delivery_plan"])
				payload, _ := json.Marshal(response["payload_application_plan"])
				for name, text := range map[string]string{"memory": extractionStringFromAny(plan["final_text"]), "injection": extractionStringFromAny(pack["injection_text"]), "payload": string(payload)} {
					if !strings.Contains(text, target) {
						t.Errorf("target lost at %s: %s", name, text)
					}
				}
				if len(index.limits) == 0 || index.limits[0] <= 1 {
					t.Errorf("legacy Top K still controls general recall: %v", index.limits)
				}
				if len([]rune(extractionStringFromAny(plan["final_text"]))) > 18000 {
					t.Error("final memory exceeded character budget")
				}
				wantPublisher := 0
				if publisher {
					wantPublisher = 1
				}
				if calls["publisher"] != wantPublisher {
					t.Errorf("wrong Publisher call count: %v", calls)
				}
				if publisher && extractionStringFromAny(mapFromAny(mapFromAny(response["payload_application_plan"])["guidance_application_trace"])["supervisor_call_status"]) != "applied" {
					t.Errorf("Publisher success path was not exercised: %v", mapFromAny(response["payload_application_plan"])["guidance_application_trace"])
				}
				if editor == "off" && calls["event_recent/false"] != 0 {
					t.Errorf("disabled editors were invoked: %v", calls)
				}
				if editor != "off" && calls["event_recent/false"] != 1 {
					t.Errorf("first analysis missing/repeated: %v", calls)
				}
				if editor == "supplement_failure" && (calls["event_recent/true"] != 1 || calls["embedding"] != 1) {
					t.Errorf("failed supplement was not exercised: %v", calls)
				}
				t.Logf("calls=%v candidate_limits=%v target_delivered=true", calls, index.limits)
			})
		}
	}
}
