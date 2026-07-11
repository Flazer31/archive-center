package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSeq168P109StaleCallbackSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p109", Name: "a1", LastTurn: 1, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p109", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sc := seq165Map(t, resp, "stale_callback_suppression")
	if sc["version"] != "seq16_8_p109.v1" {
		t.Fatalf("version=%v, want seq16_8_p109.v1", sc["version"])
	}
	if sc["role"] != "stale_callback_suppression" {
		t.Fatalf("role=%v, want stale_callback_suppression", sc["role"])
	}
	if sc["suppression_trigger"] != true {
		t.Fatalf("suppression_trigger=%v, want true", sc["suppression_trigger"])
	}
}

func TestSeq168P113OldArcForegroundVisibility(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p113", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p113", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p113","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ov := seq165Map(t, resp, "old_arc_foreground_visibility")
	if ov["version"] != "seq16_8_p113.v1" {
		t.Fatalf("version=%v, want seq16_8_p113.v1", ov["version"])
	}
	if ov["role"] != "old_arc_foreground_visibility" {
		t.Fatalf("role=%v, want old_arc_foreground_visibility", ov["role"])
	}
	if ov["visibility_lane_ready"] != true {
		t.Fatalf("visibility_lane_ready=%v, want true", ov["visibility_lane_ready"])
	}
	vr, ok := ov["visible_reasons"].([]any)
	if !ok || len(vr) == 0 {
		t.Fatalf("visible_reasons empty or missing")
	}
}

func TestSeq168P114ReasonCodeVocabulary(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p114", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p114","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	vc := seq165Map(t, resp, "reason_code_vocabulary")
	if vc["version"] != "seq16_8_p114.v1" {
		t.Fatalf("version=%v, want seq16_8_p114.v1", vc["version"])
	}
	if vc["role"] != "reason_code_vocabulary" {
		t.Fatalf("role=%v, want reason_code_vocabulary", vc["role"])
	}
	vocab, ok := vc["vocabulary"].([]any)
	if !ok || len(vocab) == 0 {
		t.Fatalf("vocabulary empty or missing")
	}
}

func TestSeq168P115PreviewAuditTransparency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p115", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p115","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pt := seq165Map(t, resp, "preview_audit_transparency")
	if pt["version"] != "seq16_8_p115.v1" {
		t.Fatalf("version=%v, want seq16_8_p115.v1", pt["version"])
	}
	if pt["role"] != "preview_audit_transparency" {
		t.Fatalf("role=%v, want preview_audit_transparency", pt["role"])
	}
	if pt["preview_ready"] != true {
		t.Fatalf("preview_ready=%v, want true", pt["preview_ready"])
	}
	if pt["audit_ready"] != true {
		t.Fatalf("audit_ready=%v, want true", pt["audit_ready"])
	}
}

func TestSeq168P119ForegroundHijackTaxonomy(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p119", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p119", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p119","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ft := seq165Map(t, resp, "foreground_hijack_taxonomy")
	if ft["version"] != "seq16_8_p119.v1" {
		t.Fatalf("version=%v, want seq16_8_p119.v1", ft["version"])
	}
	if ft["role"] != "foreground_hijack_taxonomy" {
		t.Fatalf("role=%v, want foreground_hijack_taxonomy", ft["role"])
	}
	if _, ok := ft["taxonomy_entries"]; !ok {
		t.Fatalf("missing taxonomy_entries")
	}
}

func TestSeq168P120DelayedPayoffSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p120", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "seq168-p120", FromTurn: 1, ToTurn: 5, SummaryText: "episode one"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p120", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p120","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	dp := seq165Map(t, resp, "delayed_payoff_split")
	if dp["version"] != "seq16_8_p120.v1" {
		t.Fatalf("version=%v, want seq16_8_p120.v1", dp["version"])
	}
	if dp["role"] != "delayed_payoff_split" {
		t.Fatalf("role=%v, want delayed_payoff_split", dp["role"])
	}
	if dp["delayed_payoff_rescue_ready"] != true {
		t.Fatalf("delayed_payoff_rescue_ready=%v, want true", dp["delayed_payoff_rescue_ready"])
	}
}

func TestSeq168P121RecallGainMonopolySplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p121", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p121", Name: "a2", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p121", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p121","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rs := seq165Map(t, resp, "recall_gain_monopoly_split")
	if rs["version"] != "seq16_8_p121.v1" {
		t.Fatalf("version=%v, want seq16_8_p121.v1", rs["version"])
	}
	if rs["role"] != "recall_gain_monopoly_split" {
		t.Fatalf("role=%v, want recall_gain_monopoly_split", rs["role"])
	}
	if rs["split_trace_ready"] != true {
		t.Fatalf("split_trace_ready=%v, want true", rs["split_trace_ready"])
	}
}

func TestSeq168P125StaleArcRevivalReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p125", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p125", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p125","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sr := seq165Map(t, resp, "stale_arc_revival_replay")
	if sr["version"] != "seq16_8_p125.v1" {
		t.Fatalf("version=%v, want seq16_8_p125.v1", sr["version"])
	}
	if sr["role"] != "stale_arc_revival_replay" {
		t.Fatalf("role=%v, want stale_arc_revival_replay", sr["role"])
	}
	if sr["single_incident_monopoly"] != true {
		t.Fatalf("single_incident_monopoly=%v, want true", sr["single_incident_monopoly"])
	}
	cands, ok := sr["revival_candidates"].([]any)
	if !ok || len(cands) == 0 {
		t.Fatalf("revival_candidates empty or missing")
	}
}

func TestSeq168P126TailRecallHijackGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p126", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p126", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p126","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	th := seq165Map(t, resp, "tail_recall_hijack_gate")
	if th["version"] != "seq16_8_p126.v1" {
		t.Fatalf("version=%v, want seq16_8_p126.v1", th["version"])
	}
	if th["role"] != "tail_recall_hijack_gate" {
		t.Fatalf("role=%v, want tail_recall_hijack_gate", th["role"])
	}
	if th["gate_status"] != "closed" {
		t.Fatalf("gate_status=%v, want closed", th["gate_status"])
	}
}

func TestSeq168P127NarrativeDiversityGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p127", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p127", Name: "a2", LastTurn: 2, Status: "active"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "seq168-p127", Scope: "session", Category: "law", Key: "rule1"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p127", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	nd := seq165Map(t, resp, "narrative_diversity_gate")
	if nd["version"] != "seq16_8_p127.v1" {
		t.Fatalf("version=%v, want seq16_8_p127.v1", nd["version"])
	}
	if nd["role"] != "narrative_diversity_gate" {
		t.Fatalf("role=%v, want narrative_diversity_gate", nd["role"])
	}
	if nd["diversity_gate_open"] != true {
		t.Fatalf("diversity_gate_open=%v, want true", nd["diversity_gate_open"])
	}
}

func TestSeq168P128ArcMonopolyGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p128", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p128", Name: "a2", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p128", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p128","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	am := seq165Map(t, resp, "arc_monopoly_gate")
	if am["version"] != "seq16_8_p128.v1" {
		t.Fatalf("version=%v, want seq16_8_p128.v1", am["version"])
	}
	if am["role"] != "arc_monopoly_gate" {
		t.Fatalf("role=%v, want arc_monopoly_gate", am["role"])
	}
	if am["gate_status"] != "closed" {
		t.Fatalf("gate_status=%v, want closed", am["gate_status"])
	}
}

func TestSeq168P132JSContinuityRescue(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p132", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p132", ThreadKey: "t1", CreatedTurn: 1, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p132", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	jr := seq165Map(t, resp, "js_continuity_rescue")
	if jr["version"] != "seq16_8_p132.v1" {
		t.Fatalf("version=%v, want seq16_8_p132.v1", jr["version"])
	}
	if jr["role"] != "js_continuity_rescue" {
		t.Fatalf("role=%v, want js_continuity_rescue", jr["role"])
	}
	if jr["js_owner"] != "archive_center_js" {
		t.Fatalf("js_owner=%v, want archive_center_js", jr["js_owner"])
	}
	funcs, ok := jr["js_functions"].([]any)
	if !ok || len(funcs) == 0 {
		t.Fatalf("js_functions empty or missing")
	}
}

func TestSeq168P133JSPromptAssemblyGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq168-p133", TurnIndex: 1, SummaryJSON: `{"text":"memory"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p133", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	jg := seq165Map(t, resp, "js_prompt_assembly_guard")
	if jg["version"] != "seq16_8_p133.v1" {
		t.Fatalf("version=%v, want seq16_8_p133.v1", jg["version"])
	}
	if jg["role"] != "js_prompt_assembly_guard" {
		t.Fatalf("role=%v, want js_prompt_assembly_guard", jg["role"])
	}
	if jg["js_owner"] != "archive_center_js" {
		t.Fatalf("js_owner=%v, want archive_center_js", jg["js_owner"])
	}
}

func TestSeq168P134JSTracePreviewTransparency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p134", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p134","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	jt := seq165Map(t, resp, "js_trace_preview_transparency")
	if jt["version"] != "seq16_8_p134.v1" {
		t.Fatalf("version=%v, want seq16_8_p134.v1", jt["version"])
	}
	if jt["role"] != "js_trace_preview_transparency" {
		t.Fatalf("role=%v, want js_trace_preview_transparency", jt["role"])
	}
	if jt["trace_preview_ready"] != true {
		t.Fatalf("trace_preview_ready=%v, want true", jt["trace_preview_ready"])
	}
}

func TestSeq168P135ReplayCorpusBaseline(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p135", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p135", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p135","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rc := seq165Map(t, resp, "replay_corpus_baseline")
	if rc["version"] != "seq16_8_p135.v1" {
		t.Fatalf("version=%v, want seq16_8_p135.v1", rc["version"])
	}
	if rc["role"] != "replay_corpus_baseline" {
		t.Fatalf("role=%v, want replay_corpus_baseline", rc["role"])
	}
	if _, ok := rc["corpus_entries"]; !ok {
		t.Fatalf("missing corpus_entries")
	}
	cases, ok := rc["cases"].([]any)
	if !ok || len(cases) == 0 {
		t.Fatalf("cases empty or missing")
	}
}

func TestSeq168P136BackendMetadataAlignment(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p136", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p136", ThreadKey: "t1", CreatedTurn: 1, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p136", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p136","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	bm := seq165Map(t, resp, "backend_metadata_alignment")
	if bm["version"] != "seq16_8_p136.v1" {
		t.Fatalf("version=%v, want seq16_8_p136.v1", bm["version"])
	}
	if bm["role"] != "backend_metadata_alignment" {
		t.Fatalf("role=%v, want backend_metadata_alignment", bm["role"])
	}
	if bm["metadata_aligned"] != true {
		t.Fatalf("metadata_aligned=%v, want true", bm["metadata_aligned"])
	}
	if bm["suppression_trace_confirmed"] != true {
		t.Fatalf("suppression_trace_confirmed=%v, want true", bm["suppression_trace_confirmed"])
	}
}

// SEQ-16.8-P162: Decision outcome — no-user-mention stale arc ceiling is judged
// by explicit alignment / current-scene evidence / explicit redirection, not by
// turn-gap alone. Turn-gap is used only as a pressure signal.
func TestSeq168P162DecisionOutcomeCeilingNotTurnGap(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	bodyA := `{"chat_session_id":"seq168-p162-a","turn_index":10,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqA := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyA))
	reqA.Header.Set("Content-Type", "application/json")
	recA := httptest.NewRecorder()
	mux.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recA.Code, recA.Body.String())
	}
	var respA map[string]any
	if err := json.Unmarshal(recA.Body.Bytes(), &respA); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ceilingA := seq165Map(t, respA, "stale_arc_ceiling")
	if ceilingA["auto_foreground_mandate"] != false {
		t.Fatalf("case A: auto_foreground_mandate=%v, want false (no explicit alignment)", ceilingA["auto_foreground_mandate"])
	}
	if ceilingA["judged_by_turn_gap_alone"] != false {
		t.Fatalf("case A: judged_by_turn_gap_alone=%v, want false", ceilingA["judged_by_turn_gap_alone"])
	}
	if ceilingA["pressure_signal_only"] != true {
		t.Fatalf("case A: pressure_signal_only=%v, want true", ceilingA["pressure_signal_only"])
	}

	bodyB := `{"chat_session_id":"seq168-p162-b","turn_index":5,"raw_user_input":"what happened to the old arc","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqB := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyB))
	reqB.Header.Set("Content-Type", "application/json")
	recB := httptest.NewRecorder()
	mux.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recB.Code, recB.Body.String())
	}
	var respB map[string]any
	if err := json.Unmarshal(recB.Body.Bytes(), &respB); err != nil {
		t.Fatalf("decode: %v", err)
	}
	alignB := seq165Map(t, respB, "scene_alignment")
	if alignB["explicit_query_alignment"] != true {
		t.Fatalf("case B: explicit_query_alignment=%v, want true", alignB["explicit_query_alignment"])
	}
	if alignB["amplification_allowed"] != true {
		t.Fatalf("case B: amplification_allowed=%v, want true", alignB["amplification_allowed"])
	}

	bodyC := `{"chat_session_id":"seq168-p162-c","turn_index":3,"raw_user_input":"continue the forest scene","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqC := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyC))
	reqC.Header.Set("Content-Type", "application/json")
	recC := httptest.NewRecorder()
	mux.ServeHTTP(recC, reqC)
	if recC.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recC.Code, recC.Body.String())
	}
	var respC map[string]any
	if err := json.Unmarshal(recC.Body.Bytes(), &respC); err != nil {
		t.Fatalf("decode: %v", err)
	}
	alignC := seq165Map(t, respC, "scene_alignment")
	if alignC["current_scene_evidence"] != true {
		t.Fatalf("case C: current_scene_evidence=%v, want true", alignC["current_scene_evidence"])
	}
	if alignC["amplification_allowed"] != true {
		t.Fatalf("case C: amplification_allowed=%v, want true", alignC["amplification_allowed"])
	}

	bodyD := `{"chat_session_id":"seq168-p162-d","turn_index":4,"raw_user_input":"move on instead","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqD := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyD))
	reqD.Header.Set("Content-Type", "application/json")
	recD := httptest.NewRecorder()
	mux.ServeHTTP(recD, reqD)
	if recD.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recD.Code, recD.Body.String())
	}
	var respD map[string]any
	if err := json.Unmarshal(recD.Body.Bytes(), &respD); err != nil {
		t.Fatalf("decode: %v", err)
	}
	govD := seq165Map(t, respD, "input_anchor_governor")
	redirectionD := seq165Map(t, govD, "explicit_user_redirection")
	if redirectionD["detected"] != true {
		t.Fatalf("case D: explicit_user_redirection.detected=%v, want true", redirectionD["detected"])
	}
	if redirectionD["stale_arc_demotes"] != true || redirectionD["current_user_input_wins"] != true || redirectionD["support_lane_may_redirect"] != false {
		t.Fatalf("case D: explicit_user_redirection guard mismatch: %v", redirectionD)
	}
	ceilingD := seq165Map(t, respD, "stale_arc_ceiling")
	if ceilingD["judged_by_turn_gap_alone"] != false || ceilingD["pressure_signal_only"] != true {
		t.Fatalf("case D: stale_arc_ceiling turn-gap guard mismatch: %v", ceilingD)
	}
}

// SEQ-16.8-P163: current-scene evidence minimum criteria.
// active state / latest direct evidence / recent raw turn token overlap.
func TestSeq168P163CurrentSceneEvidenceMinCriteria(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq168-p163", StateType: "scene", Content: "forest dark path"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "seq168-p163", EvidenceText: "dark path lantern is visible", TurnAnchor: 5},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p163", TurnIndex: 5, Role: "user", Content: "walk along the dark path with the lantern in the forest"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p163","turn_index":5,"raw_user_input":"walk along the dark path with the lantern in the forest","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	criteria := seq165Map(t, resp, "current_scene_evidence_min_criteria")
	if criteria["version"] != "seq16_8_p163.v1" {
		t.Fatalf("version=%v, want seq16_8_p163.v1", criteria["version"])
	}
	if criteria["role"] != "current_scene_evidence_min_criteria" {
		t.Fatalf("role=%v, want current_scene_evidence_min_criteria", criteria["role"])
	}
	if criteria["active_state_count"] != float64(1) {
		t.Fatalf("active_state_count=%v, want 1", criteria["active_state_count"])
	}
	if criteria["active_state_text"] != "forest dark path" {
		t.Fatalf("active_state_text=%v, want forest dark path", criteria["active_state_text"])
	}
	if criteria["latest_direct_evidence"] != "dark path lantern is visible" {
		t.Fatalf("latest_direct_evidence=%v, want dark path lantern is visible", criteria["latest_direct_evidence"])
	}
	if criteria["recent_raw_turn"] != "walk along the dark path with the lantern in the forest" {
		t.Fatalf("recent_raw_turn=%v, want walk along the dark path with the lantern in the forest", criteria["recent_raw_turn"])
	}
	if criteria["active_state_token_overlap_count"] == float64(0) {
		t.Fatalf("active_state_token_overlap_count=%v, want > 0", criteria["active_state_token_overlap_count"])
	}
	if criteria["latest_direct_evidence_token_overlap_count"] == float64(0) {
		t.Fatalf("latest_direct_evidence_token_overlap_count=%v, want > 0", criteria["latest_direct_evidence_token_overlap_count"])
	}
	if criteria["min_criteria_met"] != true {
		t.Fatalf("min_criteria_met=%v, want true", criteria["min_criteria_met"])
	}
	if criteria["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", criteria["inspectable"])
	}
}

// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func TestSeq168P164PendingThreadsGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p164", Title: "thread1", Status: "open"},
			{ID: 2, ChatSessionID: "seq168-p164", Title: "thread2", Status: "paused"},
			{ID: 3, ChatSessionID: "seq168-p164", Title: "thread3", Status: "resolved"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	guard := seq165Map(t, resp, "pending_threads_guard")
	if guard["version"] != "seq16_8_p164.v1" {
		t.Fatalf("version=%v, want seq16_8_p164.v1", guard["version"])
	}
	if guard["role"] != "pending_threads_guard" {
		t.Fatalf("role=%v, want pending_threads_guard", guard["role"])
	}
	if guard["open_count"] != float64(1) {
		t.Fatalf("open_count=%v, want 1", guard["open_count"])
	}
	if guard["paused_count"] != float64(1) {
		t.Fatalf("paused_count=%v, want 1", guard["paused_count"])
	}
	if guard["pending_total"] != float64(2) {
		t.Fatalf("pending_total=%v, want 2", guard["pending_total"])
	}
	if guard["guard_active"] != true {
		t.Fatalf("guard_active=%v, want true", guard["guard_active"])
	}
	if guard["ceiling_family"] != "stale_arc_ceiling" {
		t.Fatalf("ceiling_family=%v, want stale_arc_ceiling", guard["ceiling_family"])
	}
	if guard["suppress_foreground"] != true {
		t.Fatalf("suppress_foreground=%v, want true", guard["suppress_foreground"])
	}
}

// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
func TestSeq168P165ReasonVisibilityLane(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p165","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	trace := seq165Map(t, resp, "reason_trace")
	if trace["version"] != "seq16_8_p101.v1" {
		t.Fatalf("version=%v, want seq16_8_p101.v1", trace["version"])
	}
	if trace["role"] != "reason_trace" {
		t.Fatalf("role=%v, want reason_trace", trace["role"])
	}
	if trace["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", trace["inspectable"])
	}
	if trace["adaptive_trace_visible"] != true {
		t.Fatalf("adaptive_trace_visible=%v, want true", trace["adaptive_trace_visible"])
	}
	if trace["continuity_trace_visible"] != true {
		t.Fatalf("continuity_trace_visible=%v, want true", trace["continuity_trace_visible"])
	}
	if trace["input_transparency_visible"] != true {
		t.Fatalf("input_transparency_visible=%v, want true", trace["input_transparency_visible"])
	}
}

// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func TestSeq168P166DiversityGateDiagnosticWarn(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p166", Name: "arc1", Status: "active"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p166","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gate := seq165Map(t, resp, "narrative_diversity_gate")
	if gate["version"] != "seq16_8_p127.v1" {
		t.Fatalf("version=%v, want seq16_8_p127.v1", gate["version"])
	}
	if gate["role"] != "narrative_diversity_gate" {
		t.Fatalf("role=%v, want narrative_diversity_gate", gate["role"])
	}
	if gate["diagnostic_warn"] != true {
		t.Fatalf("diagnostic_warn=%v, want true", gate["diagnostic_warn"])
	}
	if gate["arc_monopoly_attempt"] != true {
		t.Fatalf("arc_monopoly_attempt=%v, want true (single storyline)", gate["arc_monopoly_attempt"])
	}
	if gate["step_17_handoff_block"] != true {
		t.Fatalf("step_17_handoff_block=%v, want true", gate["step_17_handoff_block"])
	}
}

// SEQ-17-P230: retrieval completeness vs final answer quality split.
func TestSeq17P230EvaluationSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p230","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	split := seq165Map(t, resp, "evaluation_split")
	if split["version"] != "seq17_p230.v1" {
		t.Fatalf("version=%v, want seq17_p230.v1", split["version"])
	}
	if split["role"] != "evaluation_split" {
		t.Fatalf("role=%v, want evaluation_split", split["role"])
	}
	if _, ok := split["retrieval_completeness"]; !ok {
		t.Fatalf("retrieval_completeness missing")
	}
	if _, ok := split["final_answer_quality"]; !ok {
		t.Fatalf("final_answer_quality missing")
	}
	if split["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", split["inspectable"])
	}
}

// SEQ-17-P231: ops procedure documentation surface.
func TestSeq17P231OpsProcedureSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p231","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ops := seq165Map(t, resp, "ops_procedure_surface")
	if ops["version"] != "seq17_p231.v1" {
		t.Fatalf("version=%v, want seq17_p231.v1", ops["version"])
	}
	if ops["role"] != "ops_procedure_surface" {
		t.Fatalf("role=%v, want ops_procedure_surface", ops["role"])
	}
	if ops["documented"] != true {
		t.Fatalf("documented=%v, want true", ops["documented"])
	}
	procedures, ok := ops["procedures"].([]any)
	if !ok || len(procedures) == 0 {
		t.Fatalf("procedures missing or empty")
	}
}

// SEQ-17-P232: inspection lane boundary surface.
func TestSeq17P232InspectionLaneBoundary(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p232","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	boundary := seq165Map(t, resp, "inspection_lane_boundary")
	if boundary["version"] != "seq17_p232.v1" {
		t.Fatalf("version=%v, want seq17_p232.v1", boundary["version"])
	}
	if boundary["role"] != "inspection_lane_boundary" {
		t.Fatalf("role=%v, want inspection_lane_boundary", boundary["role"])
	}
	if boundary["boundary_clear"] != true {
		t.Fatalf("boundary_clear=%v, want true", boundary["boundary_clear"])
	}
	lanes, ok := boundary["lanes"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("lanes missing or empty")
	}
}

// SEQ-17-P233: adoption gate — replay green before default adoption value.
func TestSeq17P233AdoptionGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p233","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gate := seq165Map(t, resp, "adoption_gate")
	if gate["version"] != "seq17_p233.v1" {
		t.Fatalf("version=%v, want seq17_p233.v1", gate["version"])
	}
	if gate["role"] != "adoption_gate" {
		t.Fatalf("role=%v, want adoption_gate", gate["role"])
	}
	if gate["default_adoption"] != false {
		t.Fatalf("default_adoption=%v, want false", gate["default_adoption"])
	}
	if gate["replay_green"] != false {
		t.Fatalf("replay_green=%v, want false by default", gate["replay_green"])
	}
	if gate["adoption_blocked"] != true {
		t.Fatalf("adoption_blocked=%v, want true before replay green", gate["adoption_blocked"])
	}
}

// SEQ-17-P234: release hygiene — bundle/regression/checklist repeatability.
func TestSeq17P234ReleaseHygiene(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p234","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	hygiene := seq165Map(t, resp, "release_hygiene")
	if hygiene["version"] != "seq17_p234.v1" {
		t.Fatalf("version=%v, want seq17_p234.v1", hygiene["version"])
	}
	if hygiene["role"] != "release_hygiene" {
		t.Fatalf("role=%v, want release_hygiene", hygiene["role"])
	}
	if hygiene["bundle_repeatable"] != true {
		t.Fatalf("bundle_repeatable=%v, want true", hygiene["bundle_repeatable"])
	}
	if hygiene["regression_repeatable"] != true {
		t.Fatalf("regression_repeatable=%v, want true", hygiene["regression_repeatable"])
	}
	if hygiene["checklist_repeatable"] != true {
		t.Fatalf("checklist_repeatable=%v, want true", hygiene["checklist_repeatable"])
	}
}

// SEQ-17-P238: 17-1a retrieval completeness metric define.
func TestSeq17P238RetrievalCompletenessMetric(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p238","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	metric := seq165Map(t, resp, "retrieval_completeness_metric")
	if metric["version"] != "seq17_p238.v1" {
		t.Fatalf("version=%v, want seq17_p238.v1", metric["version"])
	}
	if metric["role"] != "retrieval_completeness_metric" {
		t.Fatalf("role=%v, want retrieval_completeness_metric", metric["role"])
	}
	if metric["metric_defined"] != true {
		t.Fatalf("metric_defined=%v, want true", metric["metric_defined"])
	}
}
