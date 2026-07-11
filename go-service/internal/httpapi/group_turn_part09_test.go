package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnGenerationPacketShadowCompareRecordIncludesChapterFields(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ch", TurnIndex: 2, SummaryJSON: `{"turn_summary":"chapter one"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume with chapter material.",
			Chapter: &store.ChapterSummary{
				ID:            1,
				ChatSessionID: "sess-ch",
				FromTurn:      1,
				ToTurn:        5,
				ChapterIndex:  1,
				ChapterTitle:  "The Beginning",
				SummaryText:   "A chapter summary.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ch","turn_index":6,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	scr, ok := gp["shadow_compare_record"].(map[string]any)
	if !ok {
		t.Fatalf("shadow_compare_record is not an object")
	}
	if scr["version"] != "p249a.v1" {
		t.Fatalf("version = %v, want p249a.v1", scr["version"])
	}
	if scr["new_has_chapter"] != true {
		t.Fatalf("new_has_chapter = %v, want true", scr["new_has_chapter"])
	}
	if scr["new_chapter_chars"].(float64) <= 0 {
		t.Fatalf("new_chapter_chars = %v, want > 0", scr["new_chapter_chars"])
	}
	if scr["new_has_chapter_input"] != true {
		t.Fatalf("new_has_chapter_input = %v, want true", scr["new_has_chapter_input"])
	}
	if scr["old_has_chapter"] != false {
		t.Fatalf("old_has_chapter = %v, want false", scr["old_has_chapter"])
	}
	if scr["old_chapter_chars"].(float64) != 0 {
		t.Fatalf("old_chapter_chars = %v, want 0", scr["old_chapter_chars"])
	}
	if scr["old_has_chapter_input"] != false {
		t.Fatalf("old_has_chapter_input = %v, want false", scr["old_has_chapter_input"])
	}
	if scr["divergence_chapter"] != true {
		t.Fatalf("divergence_chapter = %v, want true", scr["divergence_chapter"])
	}
	if scr["divergence_chapter_input"] != true {
		t.Fatalf("divergence_chapter_input = %v, want true", scr["divergence_chapter_input"])
	}
}

func TestPrepareTurnCanonicalStateHardFloorFiltersStaleLayers(t *testing.T) {
	fake := &turnRecordingStore{
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-hs-prep", LayerType: "relationship_state", Content: "Mina currently trusts Rowan.", SourceStateType: "relationship_state", TurnIndex: 20, SourceTurn: 20, LastVerifiedTurn: 20, Confidence: 0.9},
			{ID: 2, ChatSessionID: "sess-hs-prep", LayerType: "relationship_state", Content: "Stale rumor says Mina distrusts Rowan.", SourceStateType: "stale_relationship_state", TurnIndex: 10, SourceTurn: 10, LastVerifiedTurn: 10, Confidence: 0.95},
			{ID: 3, ChatSessionID: "sess-hs-prep", LayerType: "settings_state", Content: "Low confidence setting should not promote.", SourceStateType: "settings_state", TurnIndex: 21, SourceTurn: 21, LastVerifiedTurn: 21, Confidence: 0.3},
		},
		returnChatLogs: []store.ChatLog{{ID: 1, ChatSessionID: "sess-hs-prep", TurnIndex: 21, Role: "assistant", Content: "They pause at the archive gate."}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hs-prep","turn_index":22,"raw_user_input":"Continue","settings":{"max_injection_chars":900,"max_input_context_chars":300,"injection_enabled":true,"input_context_enabled":true}}`
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
	canonText, _ := injectionPack["canon_text"].(string)
	if !strings.Contains(canonText, "currently trusts Rowan") {
		t.Fatalf("canon_text missing verified canonical state: %q", canonText)
	}
	if strings.Contains(canonText, "Stale rumor") || strings.Contains(canonText, "Low confidence") {
		t.Fatalf("canon_text included stale/low-confidence layer: %q", canonText)
	}
	budget, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if budget["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical hard floor flag = %v, want true", budget["canonical_state_hard_floor_enabled"])
	}
	counts, ok := resp["counts"].(map[string]any)
	if ok && counts["canonical_state_layers_filtered_count"] != float64(2) {
		t.Fatalf("canonical_state_layers_filtered_count = %v, want 2", counts["canonical_state_layers_filtered_count"])
	}
}

func TestPrepareTurnTM1aCanonicalConsistencyInputsSurface(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{{
			ID:            1,
			ChatSessionID: "sess-tm1a",
			TurnIndex:     96,
			SummaryJSON:   `{"turn_summary":"Mina waits in the reading alcove after Rowan's promise."}`,
			Importance:    0.88,
			PlaceWing:     "North Wing",
			PlaceRoom:     "Scene Room",
		}},
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-tm1a",
			CharacterName:     "Mina",
			StatusJSON:        `{"mood":"watchful"}`,
			RelationshipsJSON: `{"Rowan":{"trust":74,"last_change":"Rowan promised to return with evidence"}}`,
			TurnIndex:         97,
		}},
		returnPendingThreads: []store.PendingThread{{
			ChatSessionID: "sess-tm1a",
			ThreadKey:     "rowan-answer",
			Description:   "Rowan still owes Mina an answer about the hidden archive key.",
			Status:        "open",
			SourceTurn:    97,
		}},
		returnCanonicalLayers: []store.CanonicalStateLayer{{
			ID:               1,
			ChatSessionID:    "sess-tm1a",
			LayerType:        "scene_state",
			Content:          `{"location":"North Wing / Scene Room","pressure":"low"}`,
			SourceStateType:  "scene_state",
			TurnIndex:        97,
			SourceTurn:       97,
			LastVerifiedTurn: 97,
			Confidence:       0.92,
		}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tm1a","turn_index":98,"raw_user_input":"Continue from the archive room.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":1200,"max_input_context_chars":500,"top_k":5}}`
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	memoryText, _ := ip["memory_text"].(string)
	if !strings.Contains(memoryText, "Mina waits") || !strings.Contains(memoryText, "archive_wing=North Wing") || !strings.Contains(memoryText, "archive_room=Scene Room") {
		t.Fatalf("memory_text did not preserve scene archive placement: %q", memoryText)
	}
	characterText, _ := ip["character_text"].(string)
	if !strings.Contains(characterText, "Mina") || !strings.Contains(characterText, "Rowan") || !strings.Contains(characterText, "trust") || !strings.Contains(characterText, "74") {
		t.Fatalf("character_text did not preserve relationships_json: %q", characterText)
	}
	pendingText, _ := ip["pending_thread_text"].(string)
	if !strings.Contains(pendingText, "status=open") || !strings.Contains(pendingText, "owes Mina an answer") {
		t.Fatalf("pending_thread_text did not preserve open unresolved thread: %q", pendingText)
	}
	canonText, _ := ip["canon_text"].(string)
	if !strings.Contains(canonText, "scene_state") || !strings.Contains(canonText, "North Wing") || !strings.Contains(canonText, "Scene Room") {
		t.Fatalf("canon_text did not preserve verified scene_state layer: %q", canonText)
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	if counts["character_state_count"] != float64(1) || counts["pending_thread_count"] != float64(1) || counts["canonical_state_scene_layers_count"] != float64(1) {
		t.Fatalf("counts missing TM-1a surfaces: %#v", counts)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("prepare-turn TM-1a verification should be read-only, writes logs=%d memories=%d canonical=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedCanonicalLayers))
	}
}

func TestPrepareTurnStoreBackedAssemblySagaEvidence(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-saga", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-saga",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-saga","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	sagaText, ok := injectionPack["saga_text"].(string)
	if !ok || sagaText == "" {
		t.Fatalf("injection_pack.saga_text missing or empty: %v", injectionPack["saga_text"])
	}
	if !strings.Contains(sagaText, "saga") && !strings.Contains(sagaText, "epic") {
		t.Fatalf("saga_text does not contain saga material: %q", sagaText)
	}

	if injectionPack["saga_delivered"] != true {
		t.Fatalf("injection_pack.saga_delivered = %v, want true", injectionPack["saga_delivered"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if traceSummary["saga_delivered"] != true {
		t.Fatalf("trace_summary.saga_delivered = %v, want true", traceSummary["saga_delivered"])
	}

	sagaTextChars, ok := traceSummary["saga_text_chars"].(float64)
	if !ok {
		t.Fatalf("trace_summary.saga_text_chars type = %T, want float64", traceSummary["saga_text_chars"])
	}
	if sagaTextChars <= 0 {
		t.Fatalf("trace_summary.saga_text_chars = %v, want > 0", sagaTextChars)
	}
}

func TestPrepareTurnEnforcedBudgetModeReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-budget", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-budget","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	if pbp["budget_mode"] != "enforced" {
		t.Fatalf("budget_mode = %v, want enforced", pbp["budget_mode"])
	}
}

func TestPrepareTurnSingleQuerySharedPreserved(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-route", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-route","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	tr, ok := rr["trace"].(map[string]any)
	if !ok {
		t.Fatalf("trace is not an object")
	}

	if tr["intent_route"] != "single_query_shared" {
		t.Fatalf("intent_route = %v, want single_query_shared", tr["intent_route"])
	}
}

func TestPrepareTurnSyntheticLongSession(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-long", TurnIndex: 99, SummaryJSON: `{"turn_summary":"near the end"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Long session resume.",
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-long","turn_index":100,"raw_user_input":"Continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["reads_ok"].(float64) <= 0 {
		t.Fatalf("reads_ok = %v, want > 0", trace["reads_ok"])
	}

	if gp["degraded"] != false {
		t.Fatalf("degraded = %v, want false", gp["degraded"])
	}
}

func TestPrepareTurnOutboundRewriteGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rewrite", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rewrite","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	org, ok := trace["outbound_rewrite_guard"].(map[string]any)
	if !ok {
		t.Fatalf("outbound_rewrite_guard is not an object")
	}

	if org["version"] != "p34a.v1" {
		t.Fatalf("version = %v, want p34a.v1", org["version"])
	}
	if org["rewrite_allowed"] != false {
		t.Fatalf("rewrite_allowed = %v, want false", org["rewrite_allowed"])
	}
}

func TestPrepareTurnSagaConsumedEvidence(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-consume", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-consume",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-consume","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["saga_delivered"] != true {
		t.Fatalf("saga_delivered = %v, want true", trace["saga_delivered"])
	}

	if trace["saga_consumed"] != true {
		t.Fatalf("saga_consumed = %v, want true (saga text should be consumed into assembly text)", trace["saga_consumed"])
	}
}

func TestBuildRecallResultPerIntentActualExecution(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-exec", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-exec","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ies, ok := rr["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("intent_execution_shadow is not an object")
	}

	ae, ok := ies["actual_execution"].(map[string]any)
	if !ok {
		t.Fatalf("actual_execution is not an object")
	}

	if ae["version"] != "p44a.v1" {
		t.Fatalf("version = %v, want p44a.v1", ae["version"])
	}
	if ae["retrieval_ran"] != true {
		t.Fatalf("retrieval_ran = %v, want true", ae["retrieval_ran"])
	}
	if ae["intents_ran"].(float64) <= 0 {
		t.Fatalf("intents_ran = %v, want > 0", ae["intents_ran"])
	}
}

func TestPrepareTurnEnforcedBudgetReasonTrace(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-budget2", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-budget2","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ies, ok := rr["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("intent_execution_shadow is not an object")
	}

	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatalf("budget_enforcement is not an object")
	}

	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v, want enforced_shadow", be["mode"])
	}

	if _, ok := be["reason_counts"]; !ok {
		t.Fatalf("reason_counts missing")
	}
}

func TestBuildRecallResultLastGoodFallbackRetryEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-fb", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-fb", TurnIndex: 1, Role: "user", Content: "Hello there"},
			{ID: 2, ChatSessionID: "sess-fb", TurnIndex: 2, Role: "assistant", Content: ""},
			{ID: 3, ChatSessionID: "sess-fb", TurnIndex: 3, Role: "user", Content: "Try again"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fb","turn_index":4,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	if sfs["version"] != "p46a.v1" {
		t.Fatalf("version = %v, want p46a.v1", sfs["version"])
	}
	if sfs["retry_ready"] != true {
		t.Fatalf("retry_ready = %v, want true", sfs["retry_ready"])
	}
	if sfs["retry_count"].(float64) != 1 {
		t.Fatalf("retry_count = %v, want 1", sfs["retry_count"])
	}
	if sfs["last_retry_turn"].(float64) != 2 {
		t.Fatalf("last_retry_turn = %v, want 2", sfs["last_retry_turn"])
	}
	ce, ok := sfs["compression_evidence"].(map[string]any)
	if !ok {
		t.Fatalf("compression_evidence is not an object")
	}
	if ce["chat_log_count"].(float64) != 3 {
		t.Fatalf("chat_log_count = %v, want 3", ce["chat_log_count"])
	}
}

func TestPrepareTurnUltraProfileCompression(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-ultra", TurnIndex: 1, Role: "user", Content: "This is a very long chat log message that should be compressed"},
			{ID: 2, ChatSessionID: "sess-ultra", TurnIndex: 2, Role: "assistant", Content: "Response"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ultra","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
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

	ip, ok := gp["injection_text"].(string)
	if !ok {
		t.Fatalf("injection_text is not a string")
	}

	if len(ip) > 500 {
		t.Fatalf("injection_text length %d > 500", len(ip))
	}
}

func TestBuildRecallResultLongTierANNGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ann", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ann","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	ann, ok := rr["ann_default_takeover_guard"].(map[string]any)
	if !ok {
		t.Fatalf("ann_default_takeover_guard is not an object")
	}

	if ann["version"] != "p33a.v1" {
		t.Fatalf("version = %v, want p33a.v1", ann["version"])
	}

	ev, ok := ann["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("evidence is not an object")
	}
	if _, ok := ev["threshold_met"]; !ok {
		t.Fatalf("threshold_met missing")
	}
}

func TestBuildRecallResultStaleContextGuard(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-stale", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-stale", Name: "Main", Status: "active", Suppressed: true},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-stale", Key: "rule1", Scope: "session", Suppressed: false},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-stale", Description: "thread1", Suppressed: true},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-stale","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	scg, ok := rr["stale_context_guard"].(map[string]any)
	if !ok {
		t.Fatalf("stale_context_guard is not an object")
	}

	if scg["version"] != "p50a.v1" {
		t.Fatalf("version = %v, want p50a.v1", scg["version"])
	}
	if scg["forget_guard_active"] != true {
		t.Fatalf("forget_guard_active = %v, want true", scg["forget_guard_active"])
	}
	if scg["suppressed_storylines"].(float64) != 1 {
		t.Fatalf("suppressed_storylines = %v, want 1", scg["suppressed_storylines"])
	}
	if scg["suppressed_pending_threads"].(float64) != 1 {
		t.Fatalf("suppressed_pending_threads = %v, want 1", scg["suppressed_pending_threads"])
	}
	if scg["suppressed_world_rules"].(float64) != 0 {
		t.Fatalf("suppressed_world_rules = %v, want 0", scg["suppressed_world_rules"])
	}
	if scg["total_suppressed"].(float64) != 2 {
		t.Fatalf("total_suppressed = %v, want 2", scg["total_suppressed"])
	}
}

func TestPrepareTurnSagaTextInAuxiliaryPrompt(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-saga-aux", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Saga: &store.SagaDigest{
				ID:            1,
				ChatSessionID: "sess-saga-aux",
				FromTurn:      1,
				ToTurn:        20,
				EraLabel:      "Era One",
				SagaSummary:   "An epic saga of mystery and discovery.",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-saga-aux","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["saga_delivered"] != true {
		t.Fatalf("saga_delivered = %v, want true", trace["saga_delivered"])
	}
	if trace["saga_text_chars"].(float64) <= 0 {
		t.Fatalf("saga_text_chars = %v, want > 0", trace["saga_text_chars"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	if ip["saga_delivered"] != true {
		t.Fatalf("injection_pack.saga_delivered = %v, want true", ip["saga_delivered"])
	}

	st, ok := ip["saga_text"].(string)
	if !ok || st == "" {
		t.Fatalf("injection_pack.saga_text missing or empty")
	}
	if !strings.Contains(st, "saga") {
		t.Fatalf("saga_text does not contain saga material: %q", st)
	}
}

func TestPrepareTurnChapterHierarchyEscalationConsumed(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-chapter-aux", TurnIndex: 58, SummaryJSON: `{"turn_summary":"The bridge plan is underway"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Chapter: &store.ChapterSummary{
				ID:            1,
				ChatSessionID: "sess-chapter-aux",
				FromTurn:      41,
				ToTurn:        60,
				ChapterIndex:  3,
				ChapterTitle:  "Bridge Operation",
				SummaryText:   "Luka and the group settle the demolition plan while unresolved trust tension remains.",
				OpenLoopsJSON: `[{"text":"Whether Hank accepts the final risk tradeoff"}]`,
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-chapter-aux","turn_index":61,"raw_user_input":"계속 이어가자. 이전 작전 맥락을 잊지 말아줘.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":900,"max_input_context_chars":400,"top_k":2}}`
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
	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}
	if trace["chapter_delivered"] != true {
		t.Fatalf("chapter_delivered = %v, want true", trace["chapter_delivered"])
	}
	if trace["chapter_consumed"] != true {
		t.Fatalf("chapter_consumed = %v, want true", trace["chapter_consumed"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["chapter_delivered"] != true {
		t.Fatalf("injection_pack.chapter_delivered = %v, want true", ip["chapter_delivered"])
	}
	chapterText, ok := ip["chapter_text"].(string)
	if !ok || !strings.Contains(chapterText, "Bridge Operation") {
		t.Fatalf("chapter_text missing expected chapter material: %v", ip["chapter_text"])
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	escalation, ok := counts["hierarchy_escalation"].(map[string]any)
	if !ok {
		t.Fatalf("hierarchy_escalation is not an object: %#v", counts["hierarchy_escalation"])
	}
	if escalation["chapter_selected"] != true {
		t.Fatalf("hierarchy_escalation.chapter_selected = %v, want true", escalation["chapter_selected"])
	}
}
