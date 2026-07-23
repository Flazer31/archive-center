package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func prepareTurnPerfRequest(t *testing.T, srv *Server, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode prepare-turn response: %v", err)
	}
	return rec, response
}

func TestPrepareTurnProductionProjectionPreservesPlanAndShrinksResponse(t *testing.T) {
	legacyRec, legacy := prepareTurnPerfRequest(t, setupTestServer(), `{
		"chat_session_id":"perf-projection",
		"raw_user_input":"Continue the current scene.",
		"settings":{"guide_strength":"none"}
	}`)
	compactRec, compact := prepareTurnPerfRequest(t, setupTestServer(), `{
		"chat_session_id":"perf-projection",
		"raw_user_input":"Continue the current scene.",
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"guide_strength":"none"}
	}`)

	if compact["response_projection"] != prepareTurnProductionProjectionV1 {
		t.Fatalf("compact response projection=%v", compact["response_projection"])
	}
	if _, exists := compact["generation_packet"]; exists {
		t.Fatal("compact production response must omit legacy generation_packet")
	}
	if !reflect.DeepEqual(compact["payload_application_plan"], legacy["payload_application_plan"]) {
		t.Fatal("compact projection changed the Go-owned payload application plan")
	}
	if len(compactRec.Body.Bytes()) >= len(legacyRec.Body.Bytes()) {
		t.Fatalf("compact response bytes=%d, legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
	}
	if len(compactRec.Body.Bytes())*2 >= len(legacyRec.Body.Bytes()) {
		t.Fatalf("compact response did not remove enough legacy material: compact=%d legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
	}
	t.Logf("prepare-turn response bytes compact=%d legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
}

func TestPrepareTurnHistoryBoundsPreserveTwoAndThreeHundredTurnSessions(t *testing.T) {
	for _, test := range []struct {
		latest   int
		wantFrom int
		wantTo   int
	}{
		{latest: 200, wantFrom: 1, wantTo: 200},
		{latest: 300, wantFrom: 1, wantTo: 300},
		{latest: 450, wantFrom: 151, wantTo: 450},
	} {
		fromTurn, toTurn := prepareTurnHistoryBounds(test.latest)
		if fromTurn != test.wantFrom || toTurn != test.wantTo {
			t.Fatalf("latest=%d bounds=%d..%d, want %d..%d", test.latest, fromTurn, toTurn, test.wantFrom, test.wantTo)
		}
	}
}

type prepareTurnPerfRangeStore struct {
	*turnRecordingStore
	latestTurn          int
	memoryRangeCalls    int
	evidenceRangeCalls  int
	kgRangeCalls        int
	characterStateCalls int
	activeStateCalls    int
	canonicalStateCalls int
	legacyMemoryCalls   int
	legacyEvidenceCalls int
	legacyKGCalls       int
	fromTurn            int
	toTurn              int
	includedMemoryIDs   []int64
}

func (s *prepareTurnPerfRangeStore) LatestSessionTurnIndex(context.Context, string) (int, error) {
	return s.latestTurn, nil
}

func (s *prepareTurnPerfRangeStore) ListMemories(context.Context, string, int, int) ([]store.Memory, error) {
	s.legacyMemoryCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListMemoriesRange(_ context.Context, sid string, fromTurn, toTurn int, includeIDs []int64) ([]store.Memory, error) {
	s.memoryRangeCalls++
	s.fromTurn = fromTurn
	s.toTurn = toTurn
	s.includedMemoryIDs = append([]int64(nil), includeIDs...)
	return []store.Memory{{
		ID:            25,
		ChatSessionID: sid,
		TurnIndex:     25,
		SummaryJSON:   `{"turn_summary":"The old observatory vow still binds Mina to return the brass key."}`,
		Importance:    7,
	}}, nil
}

func (s *prepareTurnPerfRangeStore) ListEvidence(context.Context, string) ([]store.DirectEvidence, error) {
	s.legacyEvidenceCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListEvidenceRange(context.Context, string, int, int, []int64) ([]store.DirectEvidence, error) {
	s.evidenceRangeCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListKGTriples(context.Context, string) ([]store.KGTriple, error) {
	s.legacyKGCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListKGTriplesRange(context.Context, string, int, int) ([]store.KGTriple, error) {
	s.kgRangeCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListCharacterStatesCurrent(context.Context, string) ([]store.CharacterState, error) {
	s.characterStateCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListActiveStatesRange(context.Context, string, int, int) ([]store.ActiveState, error) {
	s.activeStateCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListCanonicalStateLayersRange(context.Context, string, int, int) ([]store.CanonicalStateLayer, error) {
	s.canonicalStateCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListChatLogs(_ context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	s.fromTurn = fromTurn
	s.toTurn = toTurn
	return []store.ChatLog{
		{ChatSessionID: sid, TurnIndex: toTurn, Role: "user", Content: "Mina checked the observatory map."},
		{ChatSessionID: sid, TurnIndex: toTurn, Role: "assistant", Content: "The brass key mark remained beside the old vow."},
	}, nil
}

type prepareTurnPerfVectorStore struct {
	*turnRecordingVectorStore
	results []vector.VectorDocument
}

func (s *prepareTurnPerfVectorStore) Search(context.Context, string, []float32, int, string) ([]vector.VectorDocument, error) {
	return append([]vector.VectorDocument(nil), s.results...), nil
}

func TestPrepareTurnBoundsHistoryAndHydratesOldVectorMemory(t *testing.T) {
	base := &turnRecordingStore{}
	rangeStore := &prepareTurnPerfRangeStore{turnRecordingStore: base, latestTurn: 450}
	vectorStore := &prepareTurnPerfVectorStore{
		turnRecordingVectorStore: &turnRecordingVectorStore{},
		results: []vector.VectorDocument{{
			ID:                  "memory:perf-window:25",
			ChatSessionID:       "perf-window",
			SourceTable:         "memories",
			SourceRowID:         "25",
			Similarity:          0.91,
			SimilarityAvailable: true,
			SimilaritySource:    "cosine_from_query_and_stored_embedding",
			DocumentText:        "The old observatory vow still binds Mina.",
		}},
	}
	srv := setupTestServer()
	srv.Store = rangeStore
	srv.Vector = vectorStore
	srv.Cfg.ChromaEndpoint = "http://configured.invalid"

	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"perf-window",
		"raw_user_input":"Mina returns to the old observatory for the brass key.",
		"response_projection":"prepare_turn.production_compact.v1",
		"client_meta":{"chroma_query_vector":[1,0]},
		"settings":{"top_k":1,"guide_strength":"none","max_injection_chars":4500}
	}`)

	if rangeStore.fromTurn != 151 || rangeStore.toTurn != 450 {
		t.Fatalf("history range=%d..%d, want 151..450", rangeStore.fromTurn, rangeStore.toTurn)
	}
	if rangeStore.memoryRangeCalls != 1 || rangeStore.evidenceRangeCalls != 1 || rangeStore.kgRangeCalls != 1 {
		t.Fatalf("bounded calls memory=%d evidence=%d kg=%d", rangeStore.memoryRangeCalls, rangeStore.evidenceRangeCalls, rangeStore.kgRangeCalls)
	}
	if rangeStore.characterStateCalls != 1 || rangeStore.activeStateCalls != 1 || rangeStore.canonicalStateCalls != 1 {
		t.Fatalf("bounded current-state calls character=%d active=%d canonical=%d", rangeStore.characterStateCalls, rangeStore.activeStateCalls, rangeStore.canonicalStateCalls)
	}
	if rangeStore.legacyMemoryCalls != 0 || rangeStore.legacyEvidenceCalls != 0 || rangeStore.legacyKGCalls != 0 {
		t.Fatalf("legacy full reads occurred: memory=%d evidence=%d kg=%d", rangeStore.legacyMemoryCalls, rangeStore.legacyEvidenceCalls, rangeStore.legacyKGCalls)
	}
	if len(rangeStore.includedMemoryIDs) != 1 || rangeStore.includedMemoryIDs[0] != 25 {
		t.Fatalf("old vector memory ids=%v, want [25]", rangeStore.includedMemoryIDs)
	}
	plan, _ := response["payload_application_plan"].(map[string]any)
	if !strings.Contains(strings.ToLower(extractionStringFromAny(plan["auxiliary_text"])), "old observatory vow") {
		t.Fatalf("old vector-selected memory missing from compact plan: %#v", plan)
	}
	trace := mapFromAny(response["trace_preview"])
	materialization := mapFromAny(trace["materialization"])
	if !boolFromAny(materialization["bounded_history_store"]) || intFromAny(materialization["history_window_turns"], 0) != prepareTurnHistoryWindowTurns {
		t.Fatalf("materialization trace=%#v", materialization)
	}
}
