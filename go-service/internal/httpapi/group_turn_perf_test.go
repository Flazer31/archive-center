package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	legacyRecallPlan := mapFromAny(mapFromAny(legacy["injection_pack"])["memory_recall_plan"])
	compactRecallPlan := mapFromAny(mapFromAny(compact["injection_pack"])["memory_recall_plan"])
	if legacyRecallPlan["contract_version"] != "memory_recall_plan.v1" ||
		!reflect.DeepEqual(compactRecallPlan, legacyRecallPlan) {
		t.Fatalf("compact projection changed memory recall plan: legacy=%#v compact=%#v", legacyRecallPlan, compactRecallPlan)
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

func TestPrepareTurnProductionProjectionExposesVectorRecallQueryDiagnostics(t *testing.T) {
	srv := setupTestServer()
	srv.Vector = &turnRecordingVectorStore{}
	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"compact-vector-recall-query-diagnostics",
		"raw_user_input":"Continue from the letter on the table.",
		"recent_conversation_messages":[
			{"role":"user","content":"Open the archive."},
			{"role":"assistant","content":"The first previous scene."},
			{"role":"user","content":"Place the letter on the table."},
			{"role":"assistant","content":"The latest previous scene."},
			{"role":"user","content":"Continue from the letter on the table."}
		],
		"client_meta":{"chroma_query_vector":[1,0]},
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"top_k":1,"recent_conversation_reference_count":2,"guide_strength":"none"}
	}`)

	if _, exists := response["recall_result"]; exists {
		t.Fatal("compact production response must keep the full recall_result omitted")
	}
	diagnostics := mapFromAny(mapFromAny(response["trace_preview"])["vector_recall_query"])
	want := map[string]any{
		"query_text_source":                   "raw_user_input+recent_conversation_turns",
		"query_text_count":                    3,
		"recent_conversation_query_limit":     2,
		"recent_conversation_query_count":     2,
		"query_vector_count":                  1,
		"query_history_embedding_error_count": 0,
	}
	for key, expected := range want {
		actual, exists := diagnostics[key]
		if !exists {
			t.Fatalf("compact vector recall diagnostics missing %q: %#v", key, diagnostics)
		}
		if fmt.Sprint(actual) != fmt.Sprint(expected) {
			t.Fatalf("compact vector recall diagnostics %s=%v, want %v: %#v", key, actual, expected, diagnostics)
		}
	}
}

func TestPrepareTurnMemoryRecallPlanKeepsUnobservedRequirementsUnobserved(t *testing.T) {
	for _, tc := range []struct {
		name     string
		store    store.Store
		settings string
		status   string
	}{
		{name: "injection disabled", store: &narrativeFakeStore{}, settings: `"injection_enabled":false,"input_context_enabled":false,"max_injection_chars":9000`, status: "skipped"},
		{name: "zero budget", store: &narrativeFakeStore{}, settings: `"injection_enabled":true,"input_context_enabled":false,"max_injection_chars":0`, status: "skipped"},
		{name: "store unavailable", store: nil, settings: `"injection_enabled":true,"input_context_enabled":false,"max_injection_chars":9000`, status: "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := setupTestServer()
			srv.Store = tc.store
			_, response := prepareTurnPerfRequest(t, srv, `{
				"chat_session_id":"recall-plan-unobserved",
				"turn_index":7,
				"raw_user_input":"Mina checks the harbor signal.",
				"response_projection":"prepare_turn.production_compact.v1",
				"settings":{"guide_strength":"none",`+tc.settings+`}
			}`)
			plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_recall_plan"])
			if plan["status"] != tc.status {
				t.Fatalf("plan status=%v, want %s: %#v", plan["status"], tc.status, plan)
			}
			requirements := mapFromAny(plan["requirements"])
			for _, key := range []string{"query", "direct_entities", "active_scene_entities"} {
				value := mapFromAny(requirements[key])
				if value["status"] != "unobserved" || len(value) != 1 {
					t.Fatalf("%s invented an observation: %#v", key, value)
				}
			}
			coverage := mapFromAny(plan["coverage"])
			for _, key := range []string{"direct_entities", "core_objective_memory"} {
				value := mapFromAny(coverage[key])
				if value["status"] != "unobserved" || len(value) != 1 {
					t.Fatalf("%s coverage invented zero values: %#v", key, value)
				}
			}
		})
	}
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
	latestTurn               int
	memoryTurn               int
	memorySummary            string
	latestTurnCalls          int
	memoryRangeCalls         int
	evidenceRangeCalls       int
	kgRangeCalls             int
	evidenceItems            []store.DirectEvidence
	kgItems                  []store.KGTriple
	characterStates          []store.CharacterState
	activeStates             []store.ActiveState
	canonicalStates          []store.CanonicalStateLayer
	characterStateCalls      int
	characterStateBeforeTurn int
	activeStateCalls         int
	canonicalStateCalls      int
	legacyMemoryCalls        int
	legacyEvidenceCalls      int
	legacyKGCalls            int
	fromTurn                 int
	toTurn                   int
	includedMemoryIDs        []int64
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
	memoryTurn := s.memoryTurn
	if memoryTurn <= 0 {
		memoryTurn = 25
	}
	memorySummary := s.memorySummary
	if strings.TrimSpace(memorySummary) == "" {
		memorySummary = `{"turn_summary":"The old observatory vow still binds Mina to return the brass key."}`
	}
	if (fromTurn > 0 && memoryTurn < fromTurn) || (toTurn > 0 && memoryTurn > toTurn) {
		return nil, nil
	}
	return []store.Memory{{
		ID:            25,
		ChatSessionID: sid,
		TurnIndex:     memoryTurn,
		SummaryJSON:   memorySummary,
		Importance:    7,
	}}, nil
}

func (s *prepareTurnPerfRangeStore) ListEvidence(context.Context, string) ([]store.DirectEvidence, error) {
	s.legacyEvidenceCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListEvidenceRange(context.Context, string, int, int, []int64) ([]store.DirectEvidence, error) {
	s.evidenceRangeCalls++
	return append([]store.DirectEvidence(nil), s.evidenceItems...), nil
}

func (s *prepareTurnPerfRangeStore) ListKGTriples(context.Context, string) ([]store.KGTriple, error) {
	s.legacyKGCalls++
	return nil, nil
}

func (s *prepareTurnPerfRangeStore) ListKGTriplesRange(context.Context, string, int, int) ([]store.KGTriple, error) {
	s.kgRangeCalls++
	return append([]store.KGTriple(nil), s.kgItems...), nil
}

func (s *prepareTurnPerfRangeStore) ListCharacterStatesCurrentBefore(_ context.Context, _ string, beforeTurn int) ([]store.CharacterState, error) {
	s.characterStateCalls++
	s.characterStateBeforeTurn = beforeTurn
	latest := map[string]store.CharacterState{}
	order := []string{}
	for _, item := range s.characterStates {
		if beforeTurn > 0 && item.TurnIndex >= beforeTurn {
			continue
		}
		current, exists := latest[item.CharacterName]
		if !exists {
			order = append(order, item.CharacterName)
		}
		if !exists || item.TurnIndex > current.TurnIndex || (item.TurnIndex == current.TurnIndex && item.ID > current.ID) {
			latest[item.CharacterName] = item
		}
	}
	out := make([]store.CharacterState, 0, len(order))
	for _, name := range order {
		out = append(out, latest[name])
	}
	return out, nil
}

func (s *prepareTurnPerfRangeStore) ListActiveStatesRange(context.Context, string, int, int) ([]store.ActiveState, error) {
	s.activeStateCalls++
	return append([]store.ActiveState(nil), s.activeStates...), nil
}

func (s *prepareTurnPerfRangeStore) ListCanonicalStateLayersRange(context.Context, string, int, int) ([]store.CanonicalStateLayer, error) {
	s.canonicalStateCalls++
	return append([]store.CanonicalStateLayer(nil), s.canonicalStates...), nil
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

func TestPrepareTurnExcludesPreviousGenerationOfCurrentLogicalTurn(t *testing.T) {
	markers := []string{
		"fence memory marker at the broken bridge",
		"fence evidence marker at the broken bridge",
		"fence kg marker at the broken bridge",
		"fence state marker at the broken bridge",
		"fence subjective marker at the broken bridge",
	}
	const portablePersonaMarker = "portable persona marker: Chloe remembers Mina's broken bridge oath"
	const portablePrivateMarker = "portable private marker: Chloe privately remembers Mina's broken bridge oath"
	request := func(t *testing.T, currentMessageIndex int, active []map[string]any) map[string]any {
		t.Helper()
		currentText := "Chloe asks Mina whether the broken bridge is safe."
		body := map[string]any{
			"chat_session_id":     "same-turn-reroll",
			"raw_user_input":      currentText,
			"response_projection": "prepare_turn.production_compact.v1",
			"host_observations": map[string]any{
				"contract_version": prepareHostObservationsVersion,
				"session_id":       "same-turn-reroll",
				"request_id":       "same-turn-request",
				"request_type":     "model",
				"payload_writable": true,
				"active_chat":      active,
				"payload": []map[string]any{{
					"observation_ref": "payload:current", "source_kind": "before_request_payload",
					"message_index": currentMessageIndex, "role": "user", "raw_content": currentText,
					"content_hash": prepareOR1CHash(currentText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
				}},
			},
			"settings": map[string]any{"top_k": 5, "guide_strength": "none", "max_injection_chars": 4500},
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		storeFixture := &prepareTurnPerfRangeStore{
			turnRecordingStore: &turnRecordingStore{
				returnEntityMemories: []store.ProtagonistEntityMemory{{
					ID: 51, OwnerEntityKey: "chloe", OwnerEntityName: "Chloe", OwnerEntityRole: "npc",
					OwnerVisibility: "owner_private", SourceChatSessionID: "same-turn-reroll", SourceTurn: 1,
					MemoryText: markers[4], TargetRevealPolicy: "owner_private_until_revealed",
					Portability: "npc_private_recollection", Importance10: 9,
				}},
				returnPersonaEntries: []store.PersonaMemoryEntry{
					{ID: 71, CapsuleID: 7, SourceTurn: 8, MemoryText: portablePersonaMarker, Importance10: 9, EmotionalWeight: 0.8, Portability: "cross_world", InjectionPolicy: "support_only"},
					{ID: 72, CapsuleID: 7, SourceTurn: 9, MemoryText: portablePrivateMarker, Importance10: 9, EmotionalWeight: 0.8, Portability: "npc_private_recollection", InjectionPolicy: "character_private_recollection", TagsJSON: `["owner_entity_role:npc","owner_entity_key:chloe","owner_entity_name:Chloe","source_chat_session_id:portable-source"]`},
				},
			},
			latestTurn:    1,
			memoryTurn:    1,
			memorySummary: `{"turn_summary":"` + markers[0] + `"}`,
			evidenceItems: []store.DirectEvidence{{
				ID: 52, ChatSessionID: "same-turn-reroll", EvidenceKind: "verbatim",
				EvidenceText: markers[1], SourceTurnStart: 1, SourceTurnEnd: 1, TurnAnchor: 1,
			}},
			kgItems: []store.KGTriple{{
				ID: 53, ChatSessionID: "same-turn-reroll", Subject: "Chloe", Predicate: "remembers", Object: markers[2], SourceTurn: 1,
			}},
			characterStates: []store.CharacterState{{
				ID: 54, ChatSessionID: "same-turn-reroll", CharacterName: "Chloe",
				StatusJSON: `{"objective":"` + markers[3] + `"}`, TurnIndex: 1,
			}},
			activeStates: []store.ActiveState{{
				ID: 55, ChatSessionID: "same-turn-reroll", StateType: "scene",
				Content: `{"location":"broken bridge","condition":"` + markers[3] + `"}`, TurnIndex: 1,
			}},
		}
		srv := setupTestServer()
		srv.Store = storeFixture
		_, response := prepareTurnPerfRequest(t, srv, string(encoded))
		return response
	}

	currentText := "Chloe asks Mina whether the broken bridge is safe."
	firstTurn := request(t, 0, []map[string]any{{
		"observation_ref": "active:current", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message",
		"message_index": 0, "role": "user", "raw_content": currentText,
		"content_hash": prepareOR1CHash(currentText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
	}})
	firstPlan := mapFromAny(firstTurn["payload_application_plan"])
	firstText := strings.ToLower(extractionStringFromAny(firstPlan["auxiliary_text"]))
	for _, marker := range markers {
		if strings.Contains(firstText, marker) {
			t.Fatalf("same logical turn previous generation marker %q re-entered first-turn payload: %#v", marker, firstPlan)
		}
	}
	for _, marker := range []string{portablePersonaMarker, portablePrivateMarker} {
		if !strings.Contains(firstText, strings.ToLower(marker)) {
			t.Fatalf("attached portable memory %q was incorrectly cut by the target-session turn fence: %#v", marker, firstPlan)
		}
	}
	firstTrace := mapFromAny(mapFromAny(firstTurn["trace_preview"])["materialization"])
	if intFromAny(firstTrace["current_logical_turn"], 0) != 1 || intFromAny(firstTrace["current_turn_fence_dropped_rows"], 0) < len(markers) {
		t.Fatalf("first-turn history fence was not applied: %#v", firstTrace)
	}
	for _, rowKey := range []string{"memory_rows", "evidence_rows", "kg_rows", "character_state_rows", "active_state_rows"} {
		if intFromAny(firstTrace[rowKey], -1) != 0 {
			t.Fatalf("first-turn history fence retained %s: %#v", rowKey, firstTrace)
		}
	}

	secondTurn := request(t, 2, []map[string]any{
		{"observation_ref": "active:prior-user", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 0, "role": "user", "raw_content": "prior user", "content_hash": prepareOR1CHash("prior user"), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
		{"observation_ref": "active:prior-assistant", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 1, "role": "assistant", "raw_content": "prior assistant", "content_hash": prepareOR1CHash("prior assistant"), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
		{"observation_ref": "active:current", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message", "message_index": 2, "role": "user", "raw_content": currentText, "content_hash": prepareOR1CHash(currentText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed"},
	})
	secondPlan := mapFromAny(secondTurn["payload_application_plan"])
	secondText := strings.ToLower(extractionStringFromAny(secondPlan["auxiliary_text"]))
	for index, marker := range markers {
		if index == 3 {
			continue
		}
		if !strings.Contains(secondText, marker) {
			t.Fatalf("next logical turn lost prior completed marker %q: %#v", marker, secondPlan)
		}
	}
	secondTrace := mapFromAny(mapFromAny(secondTurn["trace_preview"])["materialization"])
	for _, rowKey := range []string{"memory_rows", "evidence_rows", "kg_rows", "character_state_rows", "active_state_rows"} {
		if intFromAny(secondTrace[rowKey], 0) != 1 {
			t.Fatalf("next logical turn did not materialize prior %s: %#v", rowKey, secondTrace)
		}
	}
}

func TestPrepareTurnCharacterStateUsesLatestSnapshotBeforeResolvedCurrentTurn(t *testing.T) {
	const (
		characterName       = "Chloe"
		previousStateMarker = "keeps watch at the old bridge"
		currentStateMarker  = "returns to the observatory"
	)
	states := []store.CharacterState{
		{ID: 81, ChatSessionID: "character-state-as-of", CharacterName: characterName, StatusJSON: `{"objective":"` + previousStateMarker + `"}`, TurnIndex: 1},
		{ID: 82, ChatSessionID: "character-state-as-of", CharacterName: characterName, StatusJSON: `{"objective":"` + currentStateMarker + `"}`, TurnIndex: 2},
	}

	request := func(t *testing.T, currentTurn int) (*prepareTurnPerfRangeStore, map[string]any) {
		t.Helper()
		currentMessageIndex := (currentTurn - 1) * 2
		active := make([]map[string]any, 0, currentMessageIndex+1)
		for messageIndex := 0; messageIndex < currentMessageIndex; messageIndex++ {
			role := "user"
			content := "prior user"
			if messageIndex%2 == 1 {
				role = "assistant"
				content = "prior assistant"
			}
			active = append(active, map[string]any{
				"observation_ref": fmt.Sprintf("active:prior:%d", messageIndex), "source_kind": "active_chat", "observation_stage": "active_chat_stored_message",
				"message_index": messageIndex, "role": role, "raw_content": content,
				"content_hash": prepareOR1CHash(content), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
			})
		}
		currentText := characterName + " checks the next objective."
		active = append(active, map[string]any{
			"observation_ref": "active:current", "source_kind": "active_chat", "observation_stage": "active_chat_stored_message",
			"message_index": currentMessageIndex, "role": "user", "raw_content": currentText,
			"content_hash": prepareOR1CHash(currentText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
		})
		body := map[string]any{
			"chat_session_id": "character-state-as-of", "raw_user_input": currentText,
			"response_projection": "prepare_turn.production_compact.v1",
			"host_observations": map[string]any{
				"contract_version": prepareHostObservationsVersion, "session_id": "character-state-as-of",
				"request_id": "character-state-as-of-request", "request_type": "model", "payload_writable": true,
				"active_chat": active,
				"payload": []map[string]any{{
					"observation_ref": "payload:current", "source_kind": "before_request_payload",
					"message_index": currentMessageIndex, "role": "user", "raw_content": currentText,
					"content_hash": prepareOR1CHash(currentText), "hash_algorithm": "or1c_utf16_djb2.v1", "evidence_state": "observed",
				}},
			},
			"settings": map[string]any{"top_k": 5, "guide_strength": "none", "max_injection_chars": 4500},
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		fixture := &prepareTurnPerfRangeStore{
			turnRecordingStore: &turnRecordingStore{},
			latestTurn:         currentTurn - 1,
			memoryTurn:         states[0].TurnIndex,
			memorySummary:      `{"turn_summary":"Chloe keeps watch over the next objective.","entities":[{"name":"Chloe"}]}`,
			characterStates:    states,
			activeStates: []store.ActiveState{{
				ID: 83, ChatSessionID: "character-state-as-of", StateType: "scene",
				Content: `{"location":"old bridge","present_entities":["Chloe"]}`, TurnIndex: currentTurn - 1,
			}},
		}
		srv := setupTestServer()
		srv.Store = fixture
		_, response := prepareTurnPerfRequest(t, srv, string(encoded))
		return fixture, response
	}

	assertState := func(t *testing.T, currentTurn int, want, unwanted string) {
		t.Helper()
		fixture, response := request(t, currentTurn)
		if fixture.characterStateBeforeTurn != currentTurn {
			t.Fatalf("character state before turn=%d, want resolved current turn %d", fixture.characterStateBeforeTurn, currentTurn)
		}
		plan := mapFromAny(response["payload_application_plan"])
		text := strings.ToLower(extractionStringFromAny(plan["auxiliary_text"]))
		if !strings.Contains(text, strings.ToLower(want)) || strings.Contains(text, strings.ToLower(unwanted)) {
			t.Fatalf("as-of character state mismatch at current turn %d: text=%q plan=%#v trace=%#v", currentTurn, text, plan, response["trace_preview"])
		}
	}

	assertState(t, states[0].TurnIndex+1, previousStateMarker, currentStateMarker)
	assertState(t, states[1].TurnIndex+1, currentStateMarker, previousStateMarker)
}
