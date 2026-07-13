package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestPrepareTurnVectorHydrationUsesVectorIDFallbackAndFiltersNonMemory(t *testing.T) {
	memories := []store.Memory{
		{
			ID:          4,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"The shrine key was wrapped in red cloth."}`,
			Importance:  6,
		},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{
				"id":    "episode:sess-vector:99",
				"tier":  "episode",
				"score": 0.99,
			},
			{
				"id":                "memory:sess-vector:4",
				"tier":              "memory",
				"similarity":        0.84,
				"similarity_source": "cosine_from_query_and_stored_embedding",
			},
			{
				"id":                "memory:sess-vector:4",
				"tier":              "memory",
				"similarity":        0.84,
				"similarity_source": "cosine_from_query_and_stored_embedding",
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "red cloth key", 3, vectorShadow)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 4 {
		t.Fatalf("expected one hydrated memory from vector id fallback, got %#v", selection.VectorRelevant)
	}
	trace := mapFromAny(selection.Trace["vector_recall"])
	if got := intFromAny(trace["non_memory_count"], 0); got != 1 {
		t.Fatalf("non_memory_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["duplicate_count"], 0); got != 1 {
		t.Fatalf("duplicate_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["hydrated_count"], 0); got != 1 {
		t.Fatalf("hydrated_count = %d, want 1; trace=%#v", got, trace)
	}
}

func TestPrepareTurnStorylineSelectionPreventsStaleAmplification(t *testing.T) {
	fake := &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-e1f", Name: "Fresh confrontation", Status: "active", CurrentContext: "Fresh confrontation escalates near the gate", Confidence: 0.82, EvidenceCount: 3, LastEvidenceTurn: 11, LastTurn: 11},
			{ID: 2, ChatSessionID: "sess-e1f", Name: "Old corridor rumor", Status: "active", CurrentContext: "Old corridor rumor repeats without evidence", Confidence: 0.9, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 1},
			{ID: 3, ChatSessionID: "sess-e1f", Name: "Resolved apology", Status: "resolved", CurrentContext: "Resolved apology should stay compressed", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 8, LastTurn: 8},
			{ID: 4, ChatSessionID: "sess-e1f", Name: "Suppressed detour", Status: "active", CurrentContext: "Suppressed detour must not enter prompt", Confidence: 1, EvidenceCount: 5, LastEvidenceTurn: 12, LastTurn: 12, Suppressed: true},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-e1f","turn_index":12,"raw_user_input":"continue","settings":{"max_injection_chars":800,"injection_enabled":true,"input_context_enabled":false,"top_k":5}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	pack := resp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) {
		t.Fatalf("selected_count = %v, want 1: %#v", selection["selected_count"], selection)
	}
	if selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("stale_dropped_count = %v, want 1: %#v", selection["stale_dropped_count"], selection)
	}
	if selection["suppressed_count"] != float64(1) {
		t.Fatalf("suppressed_count = %v, want 1: %#v", selection["suppressed_count"], selection)
	}

	contextText, _ := pack["storylines_context"].(string)
	if !strings.Contains(contextText, "Fresh confrontation") {
		t.Fatalf("storylines_context missing fresh storyline: %q", contextText)
	}
	for _, forbidden := range []string{"Old corridor rumor repeats", "Suppressed detour must not enter prompt"} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("storylines_context contains forbidden stale/suppressed text %q: %q", forbidden, contextText)
		}
	}

	injectionPack := resp["injection_pack"].(map[string]any)
	storylineText, _ := injectionPack["storyline_text"].(string)
	if !strings.Contains(storylineText, "Fresh confrontation") {
		t.Fatalf("storyline_text missing fresh storyline: %q", storylineText)
	}
	if strings.Contains(storylineText, "Old corridor rumor") || strings.Contains(storylineText, "Suppressed detour") || strings.Contains(storylineText, "Resolved apology should stay compressed") {
		t.Fatalf("storyline_text contains stale/resolved/suppressed storyline: %q", storylineText)
	}
}

func TestPrepareTurnBundleIncludesFallbackChatLogsWhenMemoriesAreThin(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-fallback", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Only one memory is available"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-fallback", Subject: "Door", Predicate: "hides", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 100, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "user", Content: "I check the hallway."},
			{ID: 101, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "assistant", Content: "The hallway has a brass key under the rug."},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fallback","turn_index":2,"raw_user_input":"Use the key","settings":{"max_injection_chars":900,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	fallbackText, _ := injectionPack["fallback_text"].(string)
	if !strings.Contains(fallbackText, "brass key under the rug") {
		t.Fatalf("fallback_text missing recent chat fallback: %q", fallbackText)
	}
	budget, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if budget["fallback_chat_log_included"] != true {
		t.Errorf("fallback_chat_log_included = %v, want true", budget["fallback_chat_log_included"])
	}
	if budget["fallback_reason"] != "memory_below_threshold" {
		t.Errorf("fallback_reason = %v, want memory_below_threshold", budget["fallback_reason"])
	}
	packCounts, ok := injectionPack["counts"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack.counts is not an object")
	}
	if packCounts["memory_count"] != float64(1) || packCounts["fallback_count"] != float64(2) {
		t.Errorf("injection_pack counts memory/fallback = %v/%v, want 1/2", packCounts["memory_count"], packCounts["fallback_count"])
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	items, ok := recall["items"].([]any)
	if !ok {
		t.Fatalf("recall_result.items is not an array")
	}
	foundFallbackItem := false
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m["source"] == "chat_log" && strings.Contains(fmt.Sprint(m["content"]), "brass key") {
			foundFallbackItem = true
			break
		}
	}
	if !foundFallbackItem {
		t.Fatalf("recall_result.items missing source=chat_log fallback item: %#v", items)
	}
	recallCounts, ok := recall["counts"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.counts is not an object")
	}
	if recallCounts["memory_count"] != float64(1) || recallCounts["fallback_count"] != float64(2) {
		t.Errorf("recall_result counts memory/fallback = %v/%v, want 1/2", recallCounts["memory_count"], recallCounts["fallback_count"])
	}
}

func TestPrepareTurnTopKPrioritizesRelevantMemoryOverRecentTail(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, SummaryJSON: `{"turn_summary":"brass key old one"}`},
			{ID: 2, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, SummaryJSON: `{"turn_summary":"brass key old two"}`},
			{ID: 3, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, SummaryJSON: `{"turn_summary":"brass key old three"}`},
			{ID: 4, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, SummaryJSON: `{"turn_summary":"recent unrelated four"}`},
			{ID: 5, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, SummaryJSON: `{"turn_summary":"recent unrelated five"}`},
			{ID: 6, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, SummaryJSON: `{"turn_summary":"recent unrelated six"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 102, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 201, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 202, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
			{ID: 301, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "user", Content: "turn three user"},
			{ID: 302, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "assistant", Content: "turn three assistant"},
			{ID: 401, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "user", Content: "turn four user"},
			{ID: 402, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "assistant", Content: "turn four assistant"},
			{ID: 501, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "user", Content: "turn five user"},
			{ID: 502, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "assistant", Content: "turn five assistant"},
			{ID: 601, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "user", Content: "turn six user"},
			{ID: 602, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "assistant", Content: "turn six assistant"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-recall-lanes","turn_index":7,"raw_user_input":"brass key","settings":{"max_injection_chars":2000,"injection_enabled":true,"input_context_enabled":false,"top_k":3}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.lastEpisodeLimit != 3 {
		t.Fatalf("episode summary read limit = %d, want topK 3", fake.lastEpisodeLimit)
	}
	if fake.lastPersonaLimit != 3 {
		t.Fatalf("persona recollection read limit = %d, want topK 3", fake.lastPersonaLimit)
	}
	if fake.lastEntityMemoryLimit != 3 {
		t.Fatalf("character-private recollection read limit = %d, want topK 3", fake.lastEntityMemoryLimit)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionPack := resp["injection_pack"].(map[string]any)
	memoryText, _ := injectionPack["memory_text"].(string)
	for _, want := range []string{"brass key old three", "brass key old two", "brass key old one"} {
		if !strings.Contains(memoryText, want) {
			t.Fatalf("memory_text missing %q: %s", want, memoryText)
		}
	}
	for _, unwanted := range []string{"recent unrelated six", "recent unrelated five", "recent unrelated four"} {
		if strings.Contains(memoryText, unwanted) {
			t.Fatalf("unrelated recent memory was selected ahead of relevant memory %q: %s", unwanted, memoryText)
		}
	}

	recentRawTurnText, _ := injectionPack["recent_raw_turn_text"].(string)
	for _, want := range []string{"turn four user", "turn five user", "turn six user"} {
		if !strings.Contains(recentRawTurnText, want) {
			t.Fatalf("recent_raw_turn_text missing %q: %s", want, recentRawTurnText)
		}
	}
	if strings.Contains(recentRawTurnText, "turn three user") {
		t.Fatalf("recent_raw_turn_text exceeded topK recent turns: %s", recentRawTurnText)
	}

	counts := injectionPack["counts"].(map[string]any)
	if counts["top_k_memory_target"] != float64(3) || counts["recent_memory_bound"] != float64(0) || counts["relevant_memory_bound"] != float64(3) {
		t.Fatalf("topK/recent counts mismatch: %+v", counts)
	}

	recall := resp["recall_result"].(map[string]any)
	searchBundle := recall["search"].(map[string]any)
	if searchBundle["memory_count"] != float64(3) || searchBundle["fallback_count"] != float64(0) {
		t.Fatalf("recall_result.search counts mismatch: %+v", searchBundle)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	recent := recallLanes["recent"].(map[string]any)
	if recent["count"] != float64(0) {
		t.Fatalf("recent lane count = %v, want 0 when relevant lane fills topK: %+v", recent["count"], recent)
	}
	relevant := recallLanes["relevant"].(map[string]any)
	items := relevant["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("relevant lane item count = %d, want 3: %+v", len(items), items)
	}
	wantTurns := []float64{3, 2, 1}
	for i, item := range items {
		row := item.(map[string]any)
		if row["turn_index"] != wantTurns[i] {
			t.Fatalf("relevant lane item %d turn_index = %v, want %v: %+v", i, row["turn_index"], wantTurns[i], items)
		}
	}
	trace := recall["trace"].(map[string]any)
	laneTrace := trace["r2_recall_lanes"].(map[string]any)
	if laneTrace["top_k_memory_target"] != float64(3) || laneTrace["recent_memory_count"] != float64(0) || laneTrace["relevant_memory_count"] != float64(3) {
		t.Fatalf("trace topK/recent mismatch: %+v", laneTrace)
	}
}

func TestRecentPrepareTurnRawTurnFollowsTopKWithoutEightTurnCap(t *testing.T) {
	logs := []store.ChatLog{}
	for turn := 1; turn <= 10; turn++ {
		logs = append(logs,
			store.ChatLog{TurnIndex: turn, Role: "user", Content: fmt.Sprintf("turn %02d user", turn)},
			store.ChatLog{TurnIndex: turn, Role: "assistant", Content: fmt.Sprintf("turn %02d assistant", turn)},
		)
	}

	text := recentPrepareTurnRawTurn(logs, 10)
	for _, want := range []string{"turn 01 user", "turn 08 user", "turn 10 assistant"} {
		if !strings.Contains(text, want) {
			t.Fatalf("recent raw turn text missing %q: %s", want, text)
		}
	}
}

func TestPrepareTurnRecallLanesUseRawFallbackWhenVectorIndexNotReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, SummaryJSON: `{"turn_summary":"old memory one"}`},
			{ID: 2, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, SummaryJSON: `{"turn_summary":"old memory two"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "user", Content: "first raw turn"},
			{ID: 102, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "assistant", Content: "first assistant raw turn"},
			{ID: 201, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "user", Content: "second raw turn"},
			{ID: 202, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "assistant", Content: "second assistant raw turn"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-vector-degrade","turn_index":3,"raw_user_input":"Continue","settings":{"injection_enabled":true,"input_context_enabled":false,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall := resp["recall_result"].(map[string]any)
	readiness := recall["vector_readiness"].(map[string]any)
	if readiness["status"] != "embedding_model_not_ready" || readiness["fallback_recommended"] != true || readiness["reindex_recommended"] != true {
		t.Fatalf("vector readiness mismatch: %+v", readiness)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	rawFallback := recallLanes["raw_fallback"].(map[string]any)
	if rawFallback["active"] != true || rawFallback["count"] != float64(4) {
		t.Fatalf("raw_fallback lane mismatch: %+v", rawFallback)
	}
}

func TestPrepareTurnChromaRecallReadUsesClientMetaVector(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["would_call_vector"] != true {
		t.Errorf("recall_result.would_call_vector = %v, want true", recall["would_call_vector"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["recall_read_drill_enabled"] != true {
		t.Errorf("recall_read_drill_enabled = %v, want true", vectorShadow["recall_read_drill_enabled"])
	}
	if vectorShadow["engine"] != "chromadb" {
		t.Errorf("engine = %v, want chromadb", vectorShadow["engine"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["search_result"] != "not_found" {
		t.Errorf("search_result = %v, want not_found", vectorShadow["search_result"])
	}
	if vectorShadow["query_vector_dim"] != float64(2) {
		t.Errorf("query_vector_dim = %v, want 2", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["live_retrieval_enabled"] != false {
		t.Errorf("live_retrieval_enabled = %v, want false", vectorShadow["live_retrieval_enabled"])
	}
	for _, key := range []string{"milvus_required", "milvus_live_enabled", "optional_engine"} {
		if _, ok := vectorShadow[key]; ok {
			t.Errorf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vectorShadow)
		}
	}
}

func TestPrepareTurnChromaEndpointMarksR2Source(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("recall_result.source = %v, want go_r2_chromadb_product_read", recall["source"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["product_read_enabled"] != true {
		t.Errorf("product_read_enabled = %v, want true", vectorShadow["product_read_enabled"])
	}
	if vectorShadow["live_retrieval_enabled"] != true {
		t.Errorf("live_retrieval_enabled = %v, want true", vectorShadow["live_retrieval_enabled"])
	}
	if vectorShadow["chromadb_live_enabled"] != true {
		t.Errorf("chromadb_live_enabled = %v, want true", vectorShadow["chromadb_live_enabled"])
	}
	for _, key := range []string{"milvus_required", "milvus_live_enabled", "optional_engine"} {
		if _, ok := vectorShadow[key]; ok {
			t.Errorf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vectorShadow)
		}
	}
}

func TestPrepareTurnChromaRecallAutoEmbedsRawUserInput(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.test/v1/embeddings" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer embed-key" {
			t.Fatalf("Authorization = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "Find the relevant memory") {
			t.Fatalf("embedding request did not include raw user input: %s", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &turnRecordingVectorStore{}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"embedding":{
				"api_key":"embed-key",
				"endpoint":"https://api.example.test/v1",
				"model":"embed-model",
				"provider":"openai"
			}
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["query_embedding_status"] != "ok" {
		t.Errorf("query_embedding_status = %v, want ok", vectorShadow["query_embedding_status"])
	}
	if vectorShadow["query_vector_key"] != "server_query_embedding" {
		t.Errorf("query_vector_key = %v, want server_query_embedding", vectorShadow["query_vector_key"])
	}
	if vectorShadow["query_vector_dim"] != float64(3) {
		t.Errorf("query_vector_dim = %v, want 3", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("source = %v, want go_r2_chromadb_product_read", vectorShadow["source"])
	}
}

func TestPrepareTurnCharCapsAndTruncation(t *testing.T) {

	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-cap", TurnIndex: 1, SummaryJSON: `{"turn_summary":"` + strings.Repeat("x", 300) + `"}`, Importance: 0.5},
			{ID: 2, ChatSessionID: "sess-cap", TurnIndex: 2, SummaryJSON: `{"turn_summary":"` + strings.Repeat("y", 300) + `"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-cap", Subject: "A", Predicate: "B", Object: "C"},
		},
		returnEvidence:        []store.DirectEvidence{},
		returnChatLogs:        []store.ChatLog{},
		returnResumePack:      nil,
		returnStorylines:      nil,
		returnWorldRules:      nil,
		returnCharStates:      nil,
		returnPendingThreads:  nil,
		returnActiveStates:    nil,
		returnCanonicalLayers: nil,
		returnEpisodeSums:     nil,
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-cap","turn_index":3,"raw_user_input":"test","settings":{"max_injection_chars":50,"max_input_context_chars":50,"injection_enabled":true,"input_context_enabled":true,"top_k":10}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	injection, _ := gp["injection_text"].(string)
	if len(injection) > 50 {
		t.Errorf("injection_text length %d exceeds cap 50", len(injection))
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["injection_truncated"] != true {
		t.Errorf("injection_truncated = %v, want true (injection was truncated)", trace["injection_truncated"])
	}
}

// prepareTurnNotEnabledStore returns ErrNotEnabled for all narrative/read methods.
type prepareTurnNotEnabledStore struct {
	memoryFakeStore
}

func (n *prepareTurnNotEnabledStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListStorylines(ctx context.Context, sid string) ([]store.Storyline, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListWorldRules(ctx context.Context, sid string) ([]store.WorldRule, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListCharacterStates(ctx context.Context, sid string) ([]store.CharacterState, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListPendingThreads(ctx context.Context, sid, status string) ([]store.PendingThread, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListActiveStates(ctx context.Context, sid, stateType string) ([]store.ActiveState, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]store.CanonicalStateLayer, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, store.ErrNotEnabled
}

func TestPrepareTurnErrNotEnabledSafeFallback(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ne","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Errorf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Errorf("packet_mode = %v, want off", gp["packet_mode"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["source"] != "go_r1_read_shadow" {
		t.Errorf("recall_result.source = %v, want go_r1_read_shadow", rr["source"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}

	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "degraded" {
		t.Errorf("session_state.snapshot_status = %v, want degraded", ss["snapshot_status"])
	}
	ssMeta, _ := ss["section_meta"].(map[string]any)
	if ssMeta["storyline_count"] != float64(0) {
		t.Errorf("session_state.section_meta.storyline_count = %v, want 0", ssMeta["storyline_count"])
	}

	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "skeleton" {
		t.Errorf("narrative_control.state_status = %v, want skeleton", nc["state_status"])
	}
	if nc["storyline_count"] != float64(0) {
		t.Errorf("narrative_control.storyline_count = %v, want 0", nc["storyline_count"])
	}
	if nc["would_call_llm"] != false {
		t.Errorf("narrative_control.would_call_llm = %v, want false", nc["would_call_llm"])
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "degraded" {
		t.Errorf("continuity_pack.status = %v, want degraded", cp["status"])
	}
	if cp["resume_pack_present"] != false {
		t.Errorf("continuity_pack.resume_pack_present = %v, want false", cp["resume_pack_present"])
	}
	cpItems, _ := cp["items"].([]any)
	if len(cpItems) != 0 {
		t.Errorf("continuity_pack.items len = %d, want 0", len(cpItems))
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}
}

func TestPrepareTurnRespectsDisabledFlags(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-flag", TurnIndex: 1, SummaryJSON: `{"turn_summary":"summary"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-flag", Subject: "A", Predicate: "B", Object: "C"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-flag","turn_index":2,"raw_user_input":"test","settings":{"injection_enabled":false,"input_context_enabled":false}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["injection_text"] != nil {
		t.Errorf("injection_text = %v, want nil when disabled", resp["injection_text"])
	}
	if resp["input_context_text"] != nil {
		t.Errorf("input_context_text = %v, want nil when disabled", resp["input_context_text"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "ready" {
		t.Errorf("recall_result.status = %v, want ready", rr["status"])
	}
	if rr["query_preview"] != "test" {
		t.Errorf("query_preview = %v, want test", rr["query_preview"])
	}
	if _, ok := rr["counts"]; !ok {
		t.Errorf("recall_result.counts missing")
	}
	if _, ok := rr["vector_shadow"]; !ok {
		t.Errorf("recall_result.vector_shadow missing")
	}
}

// TestPrepareTurnDegradedFailOpenPreservesTruthFloor proves that a degraded store
// (timeout/failure simulation) does not block the main turn path and does not
// overwrite canonical state. It verifies the response still returns 200 OK,
// injection_text is present, progression_ledger has would_write=false, and
// trace_preview contains fallback indicators. (SEQ-12-P33 RMG-01)
func TestPrepareTurnDegradedFailOpenPreservesTruthFloor(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p33","turn_index":1,"raw_user_input":"hello","settings":{"max_injection_chars":500,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (main turn must not be blocked)", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Fatalf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Fatalf("packet_mode = %v, want off", gp["packet_mode"])
	}

	if _, ok := resp["injection_text"]; !ok {
		t.Fatalf("injection_text key must be present to preserve turn contract")
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger missing")
	}
	if pl["would_write"] != false {
		t.Fatalf("progression_ledger.would_write = %v, want false (truth floor must not be overwritten)", pl["would_write"])
	}
	if pl["status"] != "degraded" {
		t.Fatalf("progression_ledger.status = %v, want degraded", pl["status"])
	}

	tracePreview, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview missing")
	}
	if tracePreview["would_write"] != false {
		t.Fatalf("trace_preview.would_write = %v, want false", tracePreview["would_write"])
	}
	if tracePreview["would_call_llm"] != false {
		t.Fatalf("trace_preview.would_call_llm = %v, want false", tracePreview["would_call_llm"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing")
	}
	if rr["status"] != "degraded" {
		t.Fatalf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["would_write"] != false {
		t.Fatalf("recall_result.would_write = %v, want false", rr["would_write"])
	}
}
