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
	compactPack := mapFromAny(compact["injection_pack"])
	if _, ok := compactPack["temporal_packet"]; !ok ||
		strings.TrimSpace(extractionStringFromAny(compactPack["temporal_packet_text"])) == "" {
		t.Fatalf("compact projection dropped the Go-owned temporal packet: %#v", compactPack)
	}
	trace := mapFromAny(compact["trace_preview"])
	orchestration := mapFromAny(trace["compact_orchestration"])
	if orchestration["contract_version"] != "prepare_turn.compact_orchestration.v1" ||
		extractionStringFromAny(mapFromAny(orchestration["supervisor"])["status"]) != "disabled" {
		t.Fatalf("compact orchestration projection is not Go-owned or truthful: %#v", orchestration)
	}
	if len(compactRec.Body.Bytes()) >= len(legacyRec.Body.Bytes()) {
		t.Fatalf("compact response bytes=%d, legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
	}
	if len(compactRec.Body.Bytes())*2 >= len(legacyRec.Body.Bytes()) {
		t.Fatalf("compact response did not remove enough legacy material: compact=%d legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
	}
	t.Logf("prepare-turn response bytes compact=%d legacy=%d", compactRec.Body.Len(), legacyRec.Body.Len())
}

func TestPrepareTurnCompactOrchestrationProjectionOwnsCountsAndSupervisorStatus(t *testing.T) {
	projection := buildPrepareTurnCompactOrchestrationProjection(
		"valid_empty",
		0,
		map[string]any{"final_delivered_count": 3},
	)
	search := mapFromAny(projection["search_result"])
	supervisor := mapFromAny(projection["supervisor"])
	activity := mapFromAny(projection["activity"])
	if intFromAny(search["memoryCount"], 0) != 3 ||
		supervisor["status"] != "valid_empty" ||
		boolFromAny(supervisor["hasDirective"]) ||
		intFromAny(mapFromAny(activity["llmCalls"])["supervisor"], 0) != 1 {
		t.Fatalf("compact orchestration facts mismatch: %#v", projection)
	}

	failedTraceOnly := []prepareTurnGuidanceItem{{
		Key:        "supervisor_scene_proposal",
		Status:     "failed",
		ReasonCode: "supervisor_llm_failed_open",
	}}
	failedProjection := buildPrepareTurnCompactOrchestrationProjection(
		"failed_open",
		countPrepareTurnSupervisorDirectiveItems(failedTraceOnly),
		nil,
	)
	if boolFromAny(mapFromAny(failedProjection["supervisor"])["hasDirective"]) {
		t.Fatalf("provider failure trace was presented as an accepted directive: %#v", failedProjection)
	}
}

func TestPrepareTurnRecomposerEnhancementContractUsesExistingPlans(t *testing.T) {
	memoryPlan := map[string]any{
		"contract_version": "memory_delivery_plan.v1",
		"classes": []map[string]any{
			{"key": "event_recent", "selected_count": 2, "text": "[Event]\n- objective"},
			{"key": "subjective_relationship", "selected_count": 3, "text": "[Subjective]\n- private"},
			{"key": "protected_secret", "selected_count": 1, "text": "[Secret]\n- hidden"},
			{"key": "direct_evidence", "selected_count": 4, "text": "[Evidence]\n- verified"},
		},
	}
	lineage := map[string]any{"contract_version": "memory_delivery_lineage.v1"}
	payloadPlan := map[string]any{
		"guidance_application_trace": map[string]any{"applied_count": 2},
	}
	contract := buildPrepareTurnRecomposerEnhancementContract(
		"recomposer-session",
		12,
		memoryPlan,
		lineage,
		payloadPlan,
		"applied",
	)

	if contract["contract_version"] != "archive_center.recomposer_enhancement.v1" ||
		contract["owner"] != "go" ||
		!boolFromAny(contract["read_only"]) ||
		!boolFromAny(contract["optional_enhancement"]) ||
		!boolFromAny(contract["standalone_fallback_required"]) {
		t.Fatalf("invalid Recomposer enhancement contract: %#v", contract)
	}
	features := mapFromAny(contract["feature_status"])
	if intFromAny(mapFromAny(features["subjective_memory"])["selected_count"], 0) != 3 {
		t.Fatalf("subjective feature=%#v", features["subjective_memory"])
	}
	if intFromAny(mapFromAny(features["protected_secret"])["selected_count"], 0) != 1 ||
		extractionStringFromAny(mapFromAny(contract["lane_semantics"])["protected_secret"]) != "writer_only" {
		t.Fatalf("protected secret semantics=%#v", contract)
	}
	if intFromAny(mapFromAny(features["supervisor_guidance"])["selected_count"], 0) != 2 ||
		extractionStringFromAny(mapFromAny(features["supervisor_guidance"])["call_status"]) != "applied" {
		t.Fatalf("supervisor feature=%#v", features["supervisor_guidance"])
	}
	critic := mapFromAny(features["critic_curated_evidence"])
	if intFromAny(critic["selected_count"], 0) != 4 ||
		boolFromAny(critic["same_turn_result"]) ||
		boolFromAny(contract["same_turn_critic_result_available"]) ||
		extractionStringFromAny(critic["source_mode"]) != "prior_accepted_or_verified_direct_evidence" {
		t.Fatalf("critic feature misrepresented: %#v", critic)
	}
}

func TestPrepareTurnProductionProjectionExposesRecomposerEnhancementContract(t *testing.T) {
	_, compact := prepareTurnPerfRequest(t, setupTestServer(), `{
		"chat_session_id":"perf-recomposer-contract",
		"raw_user_input":"Continue the current scene.",
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"guide_strength":"none"}
	}`)
	plan := mapFromAny(compact["payload_application_plan"])
	contract := mapFromAny(plan["recomposer_enhancement_contract"])
	if contract["contract_version"] != "archive_center.recomposer_enhancement.v1" ||
		contract["owner"] != "go" {
		t.Fatalf("compact response omitted Recomposer contract: %#v", contract)
	}
	pack := mapFromAny(compact["injection_pack"])
	packPlan := mapFromAny(pack["payload_application_plan"])
	if !reflect.DeepEqual(packPlan["recomposer_enhancement_contract"], contract) {
		t.Fatalf("compact injection pack contract drifted: pack=%#v top=%#v", packPlan["recomposer_enhancement_contract"], contract)
	}
}

type prepareTurnPerfRangeStore struct {
	*turnRecordingStore
	latestTurn          int
	latestTurnCalls     int
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
	s.latestTurnCalls++
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
	if (fromTurn > 0 && 25 < fromTurn) || (toTurn > 0 && 25 > toTurn) {
		return nil, nil
	}
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
		{ChatSessionID: sid, TurnIndex: s.latestTurn, Role: "user", Content: "Mina checked the observatory map."},
		{ChatSessionID: sid, TurnIndex: s.latestTurn, Role: "assistant", Content: "The brass key mark remained beside the old vow."},
	}, nil
}

type prepareTurnPerfVectorStore struct {
	*turnRecordingVectorStore
	results []vector.VectorDocument
}

func (s *prepareTurnPerfVectorStore) Search(context.Context, string, []float32, int, string) ([]vector.VectorDocument, error) {
	return append([]vector.VectorDocument(nil), s.results...), nil
}

func TestPrepareTurnReadsFullSessionAndHydratesOldVectorMemory(t *testing.T) {
	base := &turnRecordingStore{}
	rangeStore := &prepareTurnPerfRangeStore{turnRecordingStore: base, latestTurn: 1_000_000}
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

	if rangeStore.fromTurn != 0 || rangeStore.toTurn != 0 {
		t.Fatalf("history range=%d..%d, want full-session 0..0", rangeStore.fromTurn, rangeStore.toTurn)
	}
	if rangeStore.latestTurnCalls != 0 {
		t.Fatalf("latest turn was queried %d times; full-session reads must not derive a bounded window", rangeStore.latestTurnCalls)
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
	if extractionStringFromAny(materialization["history_scope"]) != "full_session" ||
		boolFromAny(materialization["bounded_history_store"]) ||
		!boolFromAny(materialization["range_store_used"]) {
		t.Fatalf("materialization trace=%#v", materialization)
	}
	if _, exists := materialization["history_window_turns"]; exists {
		t.Fatalf("materialization trace still exposes a fixed history window: %#v", materialization)
	}
}
