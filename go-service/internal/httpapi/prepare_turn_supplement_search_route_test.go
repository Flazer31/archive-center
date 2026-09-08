package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// This boundary records actual production retrieval calls. The two supplemental
// queries must reach both external barriers before either is allowed to finish.
type supplementRouteVector struct {
	vector.VectorStore
	t           *testing.T
	sessionID   string
	docs        map[int]vector.VectorDocument
	started     chan int
	release     <-chan struct{}
	mu          sync.Mutex
	calls       map[int]map[string]int
	healthCalls int
}

func (v *supplementRouteVector) Health(context.Context) (vector.HealthSnapshot, error) {
	v.mu.Lock()
	v.healthCalls++
	v.mu.Unlock()
	return vector.HealthSnapshot{Status: "ok", Collection: "supplement-route", ModelReady: true}, nil
}

func (v *supplementRouteVector) Search(ctx context.Context, sid string, query []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	if sid != v.sessionID || len(query) != 2 || limit <= 0 {
		v.t.Errorf("unexpected vector request: session=%q query=%v limit=%d", sid, query, limit)
		return nil, fmt.Errorf("unexpected vector request")
	}
	index := int(query[0])
	if index < 0 || index > len(v.docs) || query[0] != float32(index) || query[1] != 1 {
		v.t.Errorf("unexpected query embedding: %v", query)
		return nil, fmt.Errorf("unexpected query embedding")
	}
	broadFilter := fmt.Sprintf("chat_session_id == %q", sid)
	if filter != broadFilter && filter != `tier == "memory"` && filter != `source_table == "precise_memory_units"` {
		v.t.Errorf("unexpected vector filter: %q", filter)
		return nil, fmt.Errorf("unexpected vector filter")
	}
	if filter == `source_table == "precise_memory_units"` && limit != len(v.docs) {
		v.t.Errorf("precise recall limit=%d, canonical fixture count=%d", limit, len(v.docs))
	}
	v.mu.Lock()
	if v.calls[index] == nil {
		v.calls[index] = map[string]int{}
	}
	v.calls[index][filter]++
	v.mu.Unlock()
	if index == 0 {
		return nil, vector.ErrNotFound
	}
	if filter == broadFilter {
		v.started <- index
		select {
		case <-v.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if filter == `tier == "memory"` {
		return nil, vector.ErrNotFound
	}
	return []vector.VectorDocument{v.docs[index]}, nil
}

func Test43PrepareTurnSupplementSearchOverlapsExternalRetrievalAndProjectsTiming(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid, requestID = "supplement-route", "supplement-route-request"
	roles := []string{"event_recent", "world_state"}
	questions := map[string]string{
		roles[0]: "Who delivered the compass at the archive gate?",
		roles[1]: "Who repaired the lantern at the archive gate?",
	}
	texts := []string{"Rook delivered the compass at the archive gate.", "Mira repaired the lantern at the archive gate."}
	embeddingStarted, vectorStarted := make(chan int, len(roles)), make(chan int, len(roles))
	releaseEmbedding, releaseVector := make(chan struct{}), make(chan struct{})
	var embeddingReleaseOnce, vectorReleaseOnce sync.Once
	var mu sync.Mutex
	llmCalls, embeddingCalls := map[string]int{}, map[string]int{}
	var srv *Server
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		if raw, embedding := body["input"]; embedding {
			encoded, _ := json.Marshal(raw)
			index := 0
			for i, role := range roles {
				if strings.Contains(string(encoded), questions[role]) {
					index = i + 1
					mu.Lock()
					embeddingCalls[role]++
					mu.Unlock()
				}
			}
			if index == 0 {
				t.Errorf("unexpected embedding input: %s", encoded)
				http.Error(w, "unexpected embedding", http.StatusBadRequest)
				return
			}
			embeddingStarted <- index
			select {
			case <-releaseEmbedding:
			case <-r.Context().Done():
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{float64(index), 1}, "index": 0}}, "model": "fixture-embedding"})
			return
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Errorf("unexpected analysis request messages: %#v", body["messages"])
			http.Error(w, "unexpected analysis request", http.StatusBadRequest)
			return
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input); err != nil {
			t.Error(err)
			http.Error(w, "invalid analysis input", http.StatusBadRequest)
			return
		}
		role := extractionStringFromAny(input["role"])
		if questions[role] == "" {
			t.Errorf("unexpected analysis role %q", role)
			http.Error(w, "unexpected analysis role", http.StatusBadRequest)
			return
		}
		_, second := input["previous_result"]
		round := 1
		if second {
			round = 2
		}
		mu.Lock()
		llmCalls[fmt.Sprintf("%s:%d", role, round)]++
		mu.Unlock()
		answer := multiAgentRecommendation{}
		candidates, _ := input["candidates"].([]any)
		if !second {
			for _, candidate := range candidates {
				for _, text := range texts {
					if strings.Contains(extractionStringFromAny(mapFromAny(candidate)["text"]), text) {
						t.Error("supplemental fact was already supplied before retrieval")
					}
				}
			}
			answer.SearchRequests = []string{questions[role]}
		} else {
			snapshot, ok := srv.TurnWorkflows.snapshot(requestID)
			if !ok || snapshot.PreprocessingSearch == nil || snapshot.PreprocessingSearch.CompletedCount != len(roles) {
				t.Errorf("second analysis started before every supplemental retrieval finished: %+v", snapshot.PreprocessingSearch)
			}
			if role == "event_recent" {
				for _, text := range texts {
					found := false
					for _, raw := range candidates {
						candidate := mapFromAny(raw)
						if strings.Contains(extractionStringFromAny(candidate["text"]), text) {
							answer.SelectedIDs = append(answer.SelectedIDs, extractionStringFromAny(candidate["ref"]))
							found = true
						}
					}
					if !found {
						t.Errorf("second analysis missing a merged canonical search result: %q", text)
					}
				}
			}
		}
		encoded, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}}})
	}))
	defer provider.Close()
	defer embeddingReleaseOnce.Do(func() { close(releaseEmbedding) })
	defer vectorReleaseOnce.Do(func() { close(releaseVector) })

	cfg := config.Default()
	cfg.StoreMode, cfg.ChromaEndpoint, cfg.Readiness.ChromaConfigured = config.StoreModeDualShadow, "http://fixture.invalid", true
	srv = NewServer(cfg)
	vec := &supplementRouteVector{t: t, sessionID: sid, docs: map[int]vector.VectorDocument{}, started: vectorStarted, release: releaseVector, calls: map[int]map[string]int{}}
	canonical := &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{{ID: 701, ChatSessionID: sid, TurnIndex: 1, Importance: 1, SummaryJSON: `{"narrative_events":[{"event":"Mira approached the archive gate.","visibility":"public"}]}`}}}}
	for index, text := range texts {
		id := fmt.Sprintf("supplement-event-%d", index+1)
		payload, _ := json.Marshal(map[string]any{"summary": text, "location": "archive gate"})
		canonical.precise = append(canonical.precise, store.PreciseMemoryUnit{UnitID: id, ChatSessionID: sid, SourceTurnStart: index + 2, SourceTurnEnd: index + 2, SourceRevision: fmt.Sprintf("revision-%d", index+2), Kind: "event", Subtype: "observed_event", PayloadJSON: string(payload), Visibility: "public", EpistemicMode: "direct", AdmissionState: "committed", ReviewState: "source_observed", LifecycleState: "active"})
		vec.docs[index+1] = vector.VectorDocument{ID: "precise_memory:" + sid + ":" + id, ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: id, SchemaVersion: store.PreciseMemoryUnitContract, Similarity: 0.9, SimilarityAvailable: true, SimilaritySource: "cosine"}
	}
	srv.Store, srv.Vector = canonical, vec
	srv.RuntimeConfig.EmbeddingProvider, srv.RuntimeConfig.EmbeddingEndpoint, srv.RuntimeConfig.EmbeddingModel, srv.RuntimeConfig.EmbeddingAPIKey = "custom", provider.URL, "fixture-embedding", "fixture-key"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 10
	settings := defaultMultiAgentSettings()
	settings.Enabled = true
	for role, value := range settings.Roles {
		value.Enabled, value.UsePublisher = questions[role] != "", false
		value.Provider, value.Endpoint, value.APIKey, value.Model = "custom", provider.URL, "fixture-key", "fixture-analysis"
		settings.Roles[role] = value
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	encoded, _ := json.Marshal(settings)
	configured := httptest.NewRecorder()
	mux.ServeHTTP(configured, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(encoded)))
	if configured.Code != http.StatusOK {
		t.Fatalf("could not configure existing preprocessing route: %d %s", configured.Code, configured.Body.String())
	}
	const userInput = "Mira checks the archive gate."
	inputHash := prepareOR1CHash(userInput)
	encoded, _ = json.Marshal(map[string]any{
		"chat_session_id": sid, "turn_index": 4, "raw_user_input": userInput,
		"client_meta": map[string]any{"chroma_query_vector": []float64{0, 1}},
		"host_observations": map[string]any{
			"contract_version": prepareHostObservationsVersion, "session_id": sid, "request_id": requestID, "request_type": "model", "payload_writable": true,
			"active_chat": []map[string]any{{
				"observation_ref": "active:0", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 0,
				"role": "user", "raw_content": userInput, "content_hash": inputHash, "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
			}},
			"payload": []map[string]any{{
				"observation_ref": "payload:0", "source_kind": "before_request_payload", "message_index": 0,
				"role": "user", "raw_content": userInput, "content_hash": inputHash, "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
			}},
		},
		"settings": map[string]any{"injection_enabled": true, "max_injection_chars": 6000, "core_objective_memory_max_items": 5, "supervisor_enabled": false},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	responseRecorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(responseRecorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)).WithContext(ctx))
	}()
	defer func() {
		cancel()
		embeddingReleaseOnce.Do(func() { close(releaseEmbedding) })
		vectorReleaseOnce.Do(func() { close(releaseVector) })
		<-done
	}()
	waitForBoth := func(phase string, started <-chan int) {
		t.Helper()
		seen := map[int]bool{}
		for len(seen) < len(roles) {
			select {
			case index := <-started:
				if seen[index] {
					t.Fatalf("%s repeated query %d before its peers arrived", phase, index)
				}
				seen[index] = true
			case <-done:
				t.Fatalf("prepare ended before parallel %s: %d %s", phase, responseRecorder.Code, responseRecorder.Body.String())
			case <-time.After(3 * time.Second):
				t.Fatalf("supplemental %s serialized: only queries %v arrived before release", phase, seen)
			}
		}
		snapshot, ok := srv.TurnWorkflows.snapshot(requestID)
		if !ok || snapshot.PreprocessingSearch == nil || snapshot.PreprocessingSearch.Status != "running" || snapshot.PreprocessingSearch.QueryCount != len(roles) || snapshot.PreprocessingSearch.CompletedCount != 0 {
			t.Fatalf("%s barrier lost running search progress: %+v", phase, snapshot.PreprocessingSearch)
		}
	}
	waitForBoth("embedding", embeddingStarted)
	embeddingReleaseOnce.Do(func() { close(releaseEmbedding) })
	waitForBoth("vector search", vectorStarted)
	// Both arrivals above prove overlap. Keep a measurable blocked interval for
	// the wall-time projection assertion even on a coarse Windows timer.
	time.Sleep(25 * time.Millisecond)
	vectorReleaseOnce.Do(func() { close(releaseVector) })
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("prepare did not complete after releasing external retrieval")
	}
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("prepare failed: %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(responseRecorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
	preprocessing := mapFromAny(plan["preprocessing"])
	searches, _ := preprocessing["searches"].([]any)
	if len(searches) != len(roles) || intFromAny(preprocessing["analysis_calls"], 0) != 2*len(roles) || intFromAny(preprocessing["analysis_attempts"], 0) != 2*len(roles) {
		t.Fatalf("search or AI call count changed: %+v", preprocessing)
	}
	hud := mapFromAny(mapFromAny(response["turn_workflow_hud"])["preprocessing_search"])
	hudQueries, _ := hud["queries"].([]any)
	if hud["status"] != "succeeded" || intFromAny(hud["query_count"], 0) != len(roles) || intFromAny(hud["completed_count"], 0) != len(roles) || len(hudQueries) != len(roles) {
		t.Fatalf("final response lost completed search HUD: %+v", hud)
	}
	for index, role := range roles {
		trace, queryHUD := mapFromAny(searches[index]), mapFromAny(hudQueries[index])
		if trace["role"] != role || trace["query"] != questions[role] || queryHUD["role"] != role || queryHUD["status"] != "succeeded" || intFromAny(trace["query_text_count"], 0) != 1 || intFromAny(trace["query_embedding_count"], 0) != 1 || trace["precise_hydration"] != "ready" {
			t.Fatalf("query lost deterministic order, its own retrieval, or hydration: trace=%+v hud=%+v", trace, queryHUD)
		}
		breakdown := mapFromAny(trace["breakdown_ms"])
		for _, stage := range []string{"health", "embedding", "vector_search", "revision_checks", "hydration", "assembly_wait", "assembly"} {
			elapsed, ok := breakdown[stage].(float64)
			if !ok || elapsed < 0 {
				t.Errorf("search %s missing non-negative %s timing: %+v", role, stage, breakdown)
			}
		}
		if !reflect.DeepEqual(breakdown, mapFromAny(queryHUD["breakdown_ms"])) {
			t.Errorf("response HUD timing differs from actual search trace: trace=%+v hud=%+v", breakdown, queryHUD)
		}
	}
	searchDuration, ok := preprocessing["search_duration_ms"].(float64)
	stages := mapFromAny(mapFromAny(response["backend_timing"])["stages_ms"])
	// HUD uses whole milliseconds while the trace rounds to three decimals.
	if !ok || searchDuration <= 0 || stages["preprocessing_search"] != searchDuration || math.Abs(float64(intFromAny(hud["duration_ms"], -1))-searchDuration) > 1 {
		t.Errorf("search wall time did not reach response timing/HUD: preprocessing=%v stages=%v hud=%v", searchDuration, stages, hud)
	}
	payload, _ := json.Marshal(response["payload_application_plan"])
	for _, text := range texts {
		if !strings.Contains(extractionStringFromAny(plan["final_text"]), text) || !strings.Contains(string(payload), text) {
			t.Errorf("selected hydrated fact did not reach final Go payload plan: %q", text)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, role := range roles {
		if embeddingCalls[role] != 1 || llmCalls[role+":1"] != 1 || llmCalls[role+":2"] != 1 {
			t.Errorf("external calls changed for %s: embeddings=%v analyses=%v", role, embeddingCalls, llmCalls)
		}
	}
	vec.mu.Lock()
	defer vec.mu.Unlock()
	if vec.healthCalls != 1+len(roles) || len(vec.calls) != 1+len(roles) {
		t.Errorf("retrieval query count changed: health=%d searches=%v", vec.healthCalls, vec.calls)
	}
	for index := 0; index <= len(roles); index++ {
		want := map[string]int{fmt.Sprintf("chat_session_id == %q", sid): 1, `tier == "memory"`: 1, `source_table == "precise_memory_units"`: 1}
		if !reflect.DeepEqual(vec.calls[index], want) {
			t.Errorf("query %d changed existing retrieval call count or filters: %v", index, vec.calls[index])
		}
	}
}
