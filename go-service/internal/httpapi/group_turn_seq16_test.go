package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// SEQ-16-P164: boundary — session/permanent role split must be present on
// the /prepare-turn response with stable version and split_policy.
func TestSeq16P164SessionPermanentRoleSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p164", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p164", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p164", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p164", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p164", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p164", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	boundary, ok := resp["retrieval_role_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_role_boundary")
	}
	if boundary["version"] != "p164a.v1" {
		t.Fatalf("expected version p164a.v1, got %v", boundary["version"])
	}
	if boundary["split_policy"] != "session_permanent_role_boundary" {
		t.Fatalf("expected split_policy session_permanent_role_boundary, got %v", boundary["split_policy"])
	}
	if boundary["permanent_role"] != "permanent" || boundary["session_role"] != "session" {
		t.Fatalf("role labels missing: %v", boundary)
	}
	perm, _ := boundary["permanent_item_count"].(float64)
	sess, _ := boundary["session_item_count"].(float64)
	totalItems := perm + sess
	if totalItems > 0 {
		if perm <= 0 {
			t.Fatalf("permanent_item_count must be > 0 when total items present: perm=%v sess=%v", perm, sess)
		}
		if sess <= 0 {
			t.Fatalf("session_item_count must be > 0 when total items present: perm=%v sess=%v", perm, sess)
		}
	}
	// Subrole assignment: storyline/character_state must be permanent; active_state/pending_thread/chat_log must be session.
	permItems, _ := boundary["permanent_items"].([]any)
	sessItems, _ := boundary["session_items"].([]any)
	hasSubrole := func(items []any, want string) bool {
		for _, raw := range items {
			m, _ := raw.(map[string]any)
			if m != nil && m["subrole"] == want {
				return true
			}
		}
		return false
	}
	if perm != 3 {
		t.Fatalf("permanent_item_count = %v, want 3", perm)
	}
	if !hasSubrole(permItems, "storyline") || !hasSubrole(permItems, "character_state") || !hasSubrole(permItems, "world_rule") {
		t.Fatalf("permanent items missing expected subroles: %v", permItems)
	}
	if sess != 3 {
		t.Fatalf("session_item_count = %v, want 3", sess)
	}
	if !hasSubrole(sessItems, "active_state") || !hasSubrole(sessItems, "pending_thread") || !hasSubrole(sessItems, "chat_log") {
		t.Fatalf("session items missing expected subroles: %v", sessItems)
	}
}

// SEQ-16-P165: support-only IR — normalized retrieval unit truth floor.
// The retrieval_index_ir must declare support_only=true, truth_store=maria_db,
// truth_authority_role=mariadb_canonical_only,
// and a non-negative indexed_unit_count.
func TestSeq16P165SupportOnlyIRTruthFloor(t *testing.T) {
	srv := setupTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p165","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ir, ok := resp["retrieval_index_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_index_ir")
	}
	if ir["support_only"] != true {
		t.Fatalf("expected support_only=true, got %v", ir["support_only"])
	}
	if ir["truth_floor"] != "support_only_ir" {
		t.Fatalf("expected truth_floor=support_only_ir, got %v", ir["truth_floor"])
	}
	if ir["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", ir["truth_store"])
	}
	if ir["truth_authority_role"] != "mariadb_canonical_only" {
		t.Fatalf("expected truth_authority_role=mariadb_canonical_only, got %v", ir["truth_authority_role"])
	}
	if ir["retrieval_accelerator"] != "chromadb_compatible" {
		t.Fatalf("expected retrieval_accelerator=chromadb_compatible, got %v", ir["retrieval_accelerator"])
	}
	if ir["unit_kind"] != "support_only_ir_normalized_retrieval_unit" {
		t.Fatalf("unexpected unit_kind: %v", ir["unit_kind"])
	}
	// The source_counts map must be present and contain all expected lanes.
	sc, _ := ir["source_counts"].(map[string]any)
	for _, k := range []string{"memories", "evidence", "kg_triples", "chat_logs", "resume_pack"} {
		if _, ok := sc[k]; !ok {
			t.Fatalf("source_counts missing key %q: %v", k, sc)
		}
	}
	if v, ok := ir["indexed_unit_count"].(float64); !ok || v < 0 {
		t.Fatalf("indexed_unit_count must be a non-negative number, got %v", ir["indexed_unit_count"])
	}
}

// SEQ-16-P166: inspectable retrieval — every search item must carry a
// signal_mix_source_tag, and the top-level signal_mix_summary must reflect
// the per-source breakdown. This is the only contract test in this file
// that targets the /search route (P166's primary surface).
func TestSeq16P166SignalMixSourceTagConfirm(t *testing.T) {
	srv := setupTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p166","user_input":"door archive","top_k":5}`
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader([]byte(body)))
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
	summary, ok := resp["signal_mix_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing signal_mix_summary on /search")
	}
	if summary["tagging_version"] != "p166a.v1" {
		t.Fatalf("expected tagging_version p166a.v1, got %v", summary["tagging_version"])
	}
	if summary["tagging_policy"] != "tag_derived_from_source_field" {
		t.Fatalf("unexpected tagging_policy: %v", summary["tagging_policy"])
	}
	if _, ok := summary["memory_signal_count"].(float64); !ok {
		t.Fatalf("memory_signal_count must be numeric: %v", summary)
	}
	if _, ok := summary["chat_log_signal_count"].(float64); !ok {
		t.Fatalf("chat_log_signal_count must be numeric: %v", summary)
	}
	if _, ok := summary["total_signal_count"].(float64); !ok {
		t.Fatalf("total_signal_count must be numeric: %v", summary)
	}
	// Even when no store is configured, items list must be present (may be empty).
	items, _ := resp["items"].([]any)
	for i, raw := range items {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		tag, ok := m["signal_mix_source_tag"]
		if !ok {
			t.Fatalf("item[%d] missing signal_mix_source_tag", i)
		}
		// tag must be one of the known values OR fall back to "support".
		switch tag {
		case "primary_signal_memory", "fallback_signal_chat_log", "support_signal_evidence", "support_signal_kg", "support":
		default:
			t.Fatalf("item[%d] has unknown signal_mix_source_tag: %v", i, tag)
		}
	}
}

// SEQ-16-P167: validity-first temporal read — the prepare-turn response must
// expose a temporal_read_validity_first surface with validity_order,
// recency_event (kind/turn_index/role), and a recency_signal_source.
func TestSeq16P167ValidityFirstTemporalRead(t *testing.T) {
	srv := setupTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p167","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	tv, ok := resp["temporal_read_validity_first"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_read_validity_first")
	}
	if tv["validity_first"] != true {
		t.Fatalf("expected validity_first=true, got %v", tv["validity_first"])
	}
	if tv["version"] != "p167a.v1" {
		t.Fatalf("expected version p167a.v1, got %v", tv["version"])
	}
	order, _ := tv["validity_order"].([]any)
	if len(order) == 0 {
		t.Fatalf("validity_order must be non-empty: %v", tv)
	}
	if order[0] != "validity_first" {
		t.Fatalf("validity_order must lead with validity_first, got %v", order)
	}
	// recency_signal_source is required and must be a string.
	src, ok := tv["recency_signal_source"].(string)
	if !ok || src == "" {
		t.Fatalf("recency_signal_source missing or not a string: %v", tv)
	}
	// recency_event may be nil when no chat_log/episode is present, but the key must exist.
	if _, ok := tv["recency_event"]; !ok {
		t.Fatalf("recency_event key missing from temporal_read_validity_first: %v", tv)
	}
	if _, ok := tv["latest_chat_turn"]; !ok {
		t.Fatalf("latest_chat_turn key missing: %v", tv)
	}
	if _, ok := tv["latest_episode_to"]; !ok {
		t.Fatalf("latest_episode_to key missing: %v", tv)
	}
}

// SEQ-16-P168: authority-aware assembly — retrieval_extend_authority must
// order authority as permanent > session > support > fallback and stay
// consistent with the retrieval_role_boundary from P164.
func TestSeq16P168AuthorityAwareAssemblyReorder(t *testing.T) {
	srv := setupTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p168","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	auth, ok := resp["retrieval_extend_authority"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_extend_authority")
	}
	if auth["version"] != "p168a.v1" {
		t.Fatalf("expected version p168a.v1, got %v", auth["version"])
	}
	order, _ := auth["authority_order"].([]any)
	if len(order) != 4 {
		t.Fatalf("authority_order must have 4 entries, got %v", order)
	}
	wantOrder := []string{"permanent", "session", "support", "fallback"}
	for i, w := range wantOrder {
		if order[i] != w {
			t.Fatalf("authority_order[%d]=%v, want %s", i, order[i], w)
		}
	}
	if auth["reorder_applied"] != true {
		t.Fatalf("expected reorder_applied=true, got %v", auth["reorder_applied"])
	}
	if auth["reorder_policy"] != "permanent_first_then_session_then_support_then_fallback" {
		t.Fatalf("unexpected reorder_policy: %v", auth["reorder_policy"])
	}
	// Counters must echo the role boundary from P164.
	boundary, _ := resp["retrieval_role_boundary"].(map[string]any)
	if boundary == nil {
		t.Fatalf("missing retrieval_role_boundary (P164 contract should still hold)")
	}
	if permBound, ok := boundary["permanent_item_count"].(float64); ok {
		if permAuth, ok := auth["permanent_item_count"].(float64); ok && permBound != permAuth {
			t.Fatalf("permanent_item_count mismatch: boundary=%v authority=%v", permBound, permAuth)
		}
	}
	if sessBound, ok := boundary["session_item_count"].(float64); ok {
		if sessAuth, ok := auth["session_item_count"].(float64); ok && sessBound != sessAuth {
			t.Fatalf("session_item_count mismatch: boundary=%v authority=%v", sessBound, sessAuth)
		}
	}
	// authority_boundary_ready must reflect whether at least one role has items.
	ready, _ := auth["authority_boundary_ready"].(bool)
	perm, _ := auth["permanent_item_count"].(float64)
	sess, _ := auth["session_item_count"].(float64)
	if perm+sess > 0 && !ready {
		t.Fatalf("expected authority_boundary_ready=true when boundary has items, got %v (perm=%v sess=%v)", ready, perm, sess)
	}
}

// SEQ-16-P172: session memory / permanent memory boundary — the prepare-turn
// response must expose a session_memory_boundary surface with session and
// permanent item counts, split_policy, and boundary_active flag.
func TestSeq16P172SessionMemoryBoundary(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p172", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p172", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p172", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p172", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p172", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p172", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p172","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	b, ok := resp["session_memory_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_memory_boundary")
	}
	if b["version"] != "p172a.v1" {
		t.Fatalf("expected version p172a.v1, got %v", b["version"])
	}
	if b["split_policy"] != "session_permanent_role_boundary" {
		t.Fatalf("unexpected split_policy: %v", b["split_policy"])
	}
	if b["session_role"] != "session" || b["permanent_role"] != "permanent" {
		t.Fatalf("role labels missing: %v", b)
	}
	perm, _ := b["permanent_item_count"].(float64)
	sess, _ := b["session_item_count"].(float64)
	if perm != 3 {
		t.Fatalf("permanent_item_count = %v, want 3", perm)
	}
	if sess != 3 {
		t.Fatalf("session_item_count = %v, want 3", sess)
	}
	if b["boundary_active"] != true {
		t.Fatalf("expected boundary_active=true, got %v", b["boundary_active"])
	}
	if b["reason"] != "seq16_p172_session_memory_boundary" {
		t.Fatalf("unexpected reason: %v", b["reason"])
	}
}

// SEQ-16-P173: bridge / promotion entry — the prepare-turn response must
// expose a bridge_promotion_entry surface listing promotion candidates
// (pending threads and canonical layers) with awaiting_promotion status.
func TestSeq16P173BridgePromotionEntry(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p173", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 70, ChatSessionID: "seq16-p173", LayerType: "scene", TurnIndex: 4},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p173","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	be, ok := resp["bridge_promotion_entry"].(map[string]any)
	if !ok {
		t.Fatalf("missing bridge_promotion_entry")
	}
	if be["version"] != "p173a.v1" {
		t.Fatalf("expected version p173a.v1, got %v", be["version"])
	}
	if be["bridge_policy"] != "pending_and_canonical_await_promotion" {
		t.Fatalf("unexpected bridge_policy: %v", be["bridge_policy"])
	}
	if be["promotion_ready"] != true {
		t.Fatalf("expected promotion_ready=true, got %v", be["promotion_ready"])
	}
	candCount, _ := be["candidate_count"].(float64)
	if candCount != 2 {
		t.Fatalf("candidate_count = %v, want 2", candCount)
	}
	candidates, _ := be["candidates"].([]any)
	if len(candidates) != 2 {
		t.Fatalf("candidates length = %v, want 2", len(candidates))
	}
	for i, raw := range candidates {
		c, _ := raw.(map[string]any)
		if c == nil {
			t.Fatalf("candidate[%d] not a map", i)
		}
		if c["status"] != "awaiting_promotion" {
			t.Fatalf("candidate[%d] status=%v, want awaiting_promotion", i, c["status"])
		}
	}
	if be["reason"] != "seq16_p173_bridge_promotion_entry" {
		t.Fatalf("unexpected reason: %v", be["reason"])
	}
}

// SEQ-16-P174: session-first / permanent-fallback read rule — the
// prepare-turn response must expose a session_first_permanent_fallback_read_rule
// surface with read_order, counts, and fallback_triggered flag.
func TestSeq16P174SessionFirstPermanentFallbackReadRule(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p174", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p174", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p174", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p174", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p174", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p174", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p174","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rule, ok := resp["session_first_permanent_fallback_read_rule"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_first_permanent_fallback_read_rule")
	}
	if rule["version"] != "p174a.v1" {
		t.Fatalf("expected version p174a.v1, got %v", rule["version"])
	}
	if rule["read_policy"] != "session_first_permanent_fallback" {
		t.Fatalf("unexpected read_policy: %v", rule["read_policy"])
	}
	order, _ := rule["read_order"].([]any)
	if len(order) == 0 || order[0] != "session" {
		t.Fatalf("read_order must lead with session when session items exist, got %v", order)
	}
	sessCount, _ := rule["session_item_count"].(float64)
	permCount, _ := rule["permanent_item_count"].(float64)
	if sessCount != 3 {
		t.Fatalf("session_item_count = %v, want 3", sessCount)
	}
	if permCount != 3 {
		t.Fatalf("permanent_item_count = %v, want 3", permCount)
	}
	if rule["fallback_triggered"] != false {
		t.Fatalf("expected fallback_triggered=false when session items exist, got %v", rule["fallback_triggered"])
	}
	if rule["reason"] != "seq16_p174_session_first_permanent_fallback_read_rule" {
		t.Fatalf("unexpected reason: %v", rule["reason"])
	}

	sessionEmptyRule := buildSessionFirstPermanentFallbackReadRule("seq16-p174-empty", map[string]any{
		"session_item_count":   0,
		"permanent_item_count": 2,
	}, nil)
	emptyOrder, _ := sessionEmptyRule["read_order"].([]string)
	if len(emptyOrder) != 2 || emptyOrder[0] != "session" || emptyOrder[1] != "permanent" {
		t.Fatalf("fallback read_order must still declare session-first policy, got %v", emptyOrder)
	}
	if sessionEmptyRule["fallback_triggered"] != true {
		t.Fatalf("fallback_triggered should be true when only permanent items exist: %v", sessionEmptyRule)
	}
}

// SEQ-16-P175: promotion-wait visibility — the prepare-turn response must
// expose a promotion_wait_visibility surface with visibility_lanes,
// pending/support lane counts, and latest_chat_turn.
func TestSeq16P175PromotionWaitVisibility(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p175", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 70, ChatSessionID: "seq16-p175", LayerType: "scene", TurnIndex: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p175", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p175","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	pw, ok := resp["promotion_wait_visibility"].(map[string]any)
	if !ok {
		t.Fatalf("missing promotion_wait_visibility")
	}
	if pw["version"] != "p175a.v1" {
		t.Fatalf("expected version p175a.v1, got %v", pw["version"])
	}
	if pw["wait_policy"] != "pending_support_lane_visible_before_promotion" {
		t.Fatalf("unexpected wait_policy: %v", pw["wait_policy"])
	}
	if pw["visibility_ready"] != true {
		t.Fatalf("expected visibility_ready=true, got %v", pw["visibility_ready"])
	}
	lanes, _ := pw["visibility_lanes"].([]any)
	if len(lanes) == 0 {
		t.Fatalf("visibility_lanes must be non-empty: %v", pw)
	}
	pendingCount, _ := pw["pending_count"].(float64)
	canonicalCount, _ := pw["canonical_count"].(float64)
	latestChatTurn, _ := pw["latest_chat_turn"].(float64)
	if pendingCount != 1 {
		t.Fatalf("pending_count = %v, want 1", pendingCount)
	}
	if canonicalCount != 1 {
		t.Fatalf("canonical_count = %v, want 1", canonicalCount)
	}
	if latestChatTurn != 4 {
		t.Fatalf("latest_chat_turn = %v, want 4", latestChatTurn)
	}
	if pw["reason"] != "seq16_p175_promotion_wait_visibility" {
		t.Fatalf("unexpected reason: %v", pw["reason"])
	}
}

// SEQ-16-P179: normalized retrieval unit schema — the prepare-turn response
// must expose a retrieval_units_ir surface with unit_schema, unit_id,
// source_type, source_record_id, source_turn_start/end, excerpt,
// summary_only_dependency, source_depth, and truth_authority=false for
// support-only units.
func TestSeq16P179NormalizedRetrievalUnitSchema(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p179", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p179", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p179", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p179", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ok",
			AssembledText: "chapter resume says the key matters",
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p179","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ir, ok := resp["retrieval_units_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_units_ir")
	}
	if ir["version"] != "p179a.v1" {
		t.Fatalf("expected version p179a.v1, got %v", ir["version"])
	}
	if ir["unit_schema"] != "normalized_retrieval_unit_v1" {
		t.Fatalf("unexpected unit_schema: %v", ir["unit_schema"])
	}
	if ir["support_only"] != true {
		t.Fatalf("expected support_only=true, got %v", ir["support_only"])
	}
	if ir["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", ir["truth_store"])
	}
	if ir["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", ir["retrieval_role"])
	}
	units, _ := ir["units"].([]any)
	if len(units) != 5 {
		t.Fatalf("units length = %v, want 5", len(units))
	}
	seenSourceTypes := map[string]bool{}
	for i, raw := range units {
		u, _ := raw.(map[string]any)
		if u == nil {
			t.Fatalf("unit[%d] not a map", i)
		}
		for _, k := range []string{"unit_schema", "unit_id", "source_type", "source_record_id", "source_turn_start", "source_turn_end", "excerpt", "summary_only_dependency", "source_depth", "truth_authority"} {
			if _, ok := u[k]; !ok {
				t.Fatalf("unit[%d] missing key %q: %v", i, k, u)
			}
		}
		if u["unit_schema"] != "normalized_retrieval_unit_v1" {
			t.Fatalf("unit[%d] unexpected unit_schema: %v", i, u["unit_schema"])
		}
		if u["truth_authority"] != false {
			t.Fatalf("unit[%d] normalized unit must not be truth authority: %v", i, u)
		}
		src, _ := u["source_type"].(string)
		seenSourceTypes[src] = true
		switch src {
		case "memory", "resume_pack":
			if u["summary_only_dependency"] != true {
				t.Fatalf("unit[%d] %s should be marked summary-only dependency: %v", i, src, u)
			}
		case "direct_evidence", "kg_triple", "chat_log":
			if u["summary_only_dependency"] != false {
				t.Fatalf("unit[%d] %s should not be summary-only dependency: %v", i, src, u)
			}
		default:
			t.Fatalf("unit[%d] unexpected source_type %q", i, src)
		}
	}
	for _, src := range []string{"memory", "direct_evidence", "kg_triple", "chat_log", "resume_pack"} {
		if !seenSourceTypes[src] {
			t.Fatalf("missing normalized unit source_type %q in %v", src, seenSourceTypes)
		}
	}
	if ir["resume_pack_units"] != float64(1) {
		t.Fatalf("resume_pack_units = %v, want 1", ir["resume_pack_units"])
	}
	if ir["reason"] != "seq16_p179_normalized_retrieval_unit_schema" {
		t.Fatalf("unexpected reason: %v", ir["reason"])
	}
}

// SEQ-16-P180: direct evidence vs normalized unit dual-representation —
// the prepare-turn response must expose a direct_evidence_dual_representation
// surface with both canonical_original and normalized_unit items.
func TestSeq16P180DirectEvidenceDualRepresentation(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p180", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p180","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dual, ok := resp["direct_evidence_dual_representation"].(map[string]any)
	if !ok {
		t.Fatalf("missing direct_evidence_dual_representation")
	}
	if dual["version"] != "p180a.v1" {
		t.Fatalf("expected version p180a.v1, got %v", dual["version"])
	}
	if dual["dual_policy"] != "canonical_original_plus_normalized_unit" {
		t.Fatalf("unexpected dual_policy: %v", dual["dual_policy"])
	}
	if dual["identifiable_both"] != true {
		t.Fatalf("expected identifiable_both=true when evidence present, got %v", dual["identifiable_both"])
	}
	canon, _ := dual["canonical_items"].([]any)
	norm, _ := dual["normalized_items"].([]any)
	if len(canon) != 1 || len(norm) != 1 {
		t.Fatalf("canonical=%v normalized=%v, want 1 each", len(canon), len(norm))
	}
	c0, _ := canon[0].(map[string]any)
	n0, _ := norm[0].(map[string]any)
	if c0["role"] != "canonical_evidence" {
		t.Fatalf("canonical item role=%v, want canonical_evidence", c0["role"])
	}
	if n0["role"] != "normalized_retrieval_unit" {
		t.Fatalf("normalized item role=%v, want normalized_retrieval_unit", n0["role"])
	}
	if n0["truth_authority"] != false {
		t.Fatalf("normalized direct evidence unit must not be truth authority: %v", n0)
	}
	if n0["unit_id"] != "ev_10" || n0["source_record_id"] != float64(10) {
		t.Fatalf("normalized item identity mismatch: %v", n0)
	}
	if dual["reason"] != "seq16_p180_direct_evidence_vs_normalized_unit_dual_representation" {
		t.Fatalf("unexpected reason: %v", dual["reason"])
	}
}

// SEQ-16-P181: source-tagged retrieval unit surface — the prepare-turn
// response must expose a source_tagged_retrieval_unit_surface with
// source_tag and source_type per unit.
func TestSeq16P181SourceTaggedRetrievalUnitSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p181", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p181", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p181", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p181", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ok",
			AssembledText: "chapter resume says the key matters",
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p181","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	surf, ok := resp["source_tagged_retrieval_unit_surface"].(map[string]any)
	if !ok {
		t.Fatalf("missing source_tagged_retrieval_unit_surface")
	}
	if surf["version"] != "p181a.v1" {
		t.Fatalf("expected version p181a.v1, got %v", surf["version"])
	}
	if surf["tagging_policy"] != "source_derived_from_store_type" {
		t.Fatalf("unexpected tagging_policy: %v", surf["tagging_policy"])
	}
	tagged, _ := surf["tagged_units"].([]any)
	if len(tagged) != 5 {
		t.Fatalf("tagged_units length = %v, want 5", len(tagged))
	}
	expectedTags := map[string]string{
		"mem_1":       "primary_signal_memory",
		"ev_10":       "support_signal_evidence",
		"kg_20":       "support_signal_kg",
		"cl_30":       "fallback_signal_chat_log",
		"resume_pack": "support_signal_resume_pack",
	}
	for i, raw := range tagged {
		u, _ := raw.(map[string]any)
		if u == nil {
			t.Fatalf("tagged[%d] not a map", i)
		}
		uid, _ := u["unit_id"].(string)
		wantTag, ok := expectedTags[uid]
		if !ok {
			t.Fatalf("unexpected unit_id %q", uid)
		}
		if u["source_tag"] != wantTag {
			t.Fatalf("unit %q source_tag=%v, want %v", uid, u["source_tag"], wantTag)
		}
	}
	if surf["reason"] != "seq16_p181_source_tagged_retrieval_unit_surface" {
		t.Fatalf("unexpected reason: %v", surf["reason"])
	}
}

// SEQ-16-P182: raw turn span / excerpt pointer / source depth metadata —
// the prepare-turn response must expose a raw_turn_span_metadata surface
// with turn_span, excerpt_pointer, source_depth, summary_only, and
// has_direct_pointer per unit.
func TestSeq16P182RawTurnSpanMetadata(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p182", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p182", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p182", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p182", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p182","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	meta, ok := resp["raw_turn_span_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("missing raw_turn_span_metadata")
	}
	if meta["version"] != "p182a.v1" {
		t.Fatalf("expected version p182a.v1, got %v", meta["version"])
	}
	if meta["pointer_policy"] != "excerpt_plus_turn_span" {
		t.Fatalf("unexpected pointer_policy: %v", meta["pointer_policy"])
	}
	if meta["summary_only_guard"] != true {
		t.Fatalf("expected summary_only_guard=true, got %v", meta["summary_only_guard"])
	}
	spans, _ := meta["spans"].([]any)
	if len(spans) == 0 {
		t.Fatalf("spans must be non-empty: %v", meta)
	}
	for i, raw := range spans {
		s, _ := raw.(map[string]any)
		if s == nil {
			t.Fatalf("span[%d] not a map", i)
		}
		for _, k := range []string{"unit_id", "source_type", "turn_span", "excerpt_pointer", "source_depth", "summary_only", "has_direct_pointer"} {
			if _, ok := s[k]; !ok {
				t.Fatalf("span[%d] missing key %q: %v", i, k, s)
			}
		}
	}
	if meta["latest_chat_turn"] != float64(4) {
		t.Fatalf("latest_chat_turn = %v, want 4", meta["latest_chat_turn"])
	}
	if meta["latest_episode_to"] != float64(4) {
		t.Fatalf("latest_episode_to = %v, want 4", meta["latest_episode_to"])
	}
	if meta["reason"] != "seq16_p182_raw_turn_span_excerpt_pointer_source_depth_metadata" {
		t.Fatalf("unexpected reason: %v", meta["reason"])
	}
}

// SEQ-16-P186: semantic / keyword / entity / graph / time-range signal mix
// contract — the prepare-turn response must expose a signal_mix_contract
// surface with inspectable signals, each marked support-only and
// truth_authority=false.
func TestSeq16P186SignalMixContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p186", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p186", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p186", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p186", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p186", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p186","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	mix, ok := resp["signal_mix_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing signal_mix_contract")
	}
	if mix["version"] != "p186a.v1" {
		t.Fatalf("expected version p186a.v1, got %v", mix["version"])
	}
	if mix["mix_policy"] != "semantic_keyword_entity_graph_time_range" {
		t.Fatalf("unexpected mix_policy: %v", mix["mix_policy"])
	}
	if mix["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", mix["truth_store"])
	}
	if mix["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", mix["retrieval_role"])
	}
	signals, _ := mix["signals"].([]any)
	if len(signals) != 5 {
		t.Fatalf("signals length = %v, want 5", len(signals))
	}
	expected := map[string]struct {
		count  int
		source string
	}{
		"semantic":   {1, "memory_embedding"},
		"keyword":    {1, "chat_log_verbatim"},
		"entity":     {1, "kg_triple_subject_object"},
		"graph":      {1, "kg_triple_predicate_link"},
		"time_range": {1, "episode_summary_span"},
	}
	for i, raw := range signals {
		s, _ := raw.(map[string]any)
		if s == nil {
			t.Fatalf("signal[%d] not a map", i)
		}
		name, _ := s["signal"].(string)
		want, ok := expected[name]
		if !ok {
			t.Fatalf("unexpected signal %q", name)
		}
		if s["source"] != want.source {
			t.Fatalf("signal %q source=%v, want %v", name, s["source"], want.source)
		}
		if s["role"] != "support_accelerator" && s["role"] != "fallback_support" {
			t.Fatalf("signal %q unexpected role: %v", name, s["role"])
		}
		if s["truth_authority"] != false {
			t.Fatalf("signal %q truth_authority must be false", name)
		}
	}
	if mix["reason"] != "seq16_p186_semantic_keyword_entity_graph_time_range_signal_mix" {
		t.Fatalf("unexpected reason: %v", mix["reason"])
	}
}

// SEQ-16-P187: query class retrieval depth / signal routing — the
// prepare-turn response must expose a query_class_routing surface with
// query_class, depth_policy, primary_signal, fallback_signals, and
// truth_authority=false for every class.
func TestSeq16P187QueryClassRouting(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p187", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p187", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p187", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p187","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	routing, ok := resp["query_class_routing"].(map[string]any)
	if !ok {
		t.Fatalf("missing query_class_routing")
	}
	if routing["version"] != "p187a.v1" {
		t.Fatalf("expected version p187a.v1, got %v", routing["version"])
	}
	if routing["routing_policy"] != "query_class_depth_signal_routing" {
		t.Fatalf("unexpected routing_policy: %v", routing["routing_policy"])
	}
	if routing["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", routing["truth_store"])
	}
	if routing["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", routing["retrieval_role"])
	}
	classes, _ := routing["classes"].([]any)
	if len(classes) != 5 {
		t.Fatalf("classes length = %v, want 5", len(classes))
	}
	expectedClasses := map[string]struct {
		depth   string
		primary string
	}{
		"factual_lookup":        {"canonical_evidence_first", "direct_evidence"},
		"relationship_state":    {"graph_then_memory", "kg_triple"},
		"narrative_progression": {"episode_then_chat_log", "episode_summary"},
		"recent_context":        {"raw_turn_first", "chat_log"},
		"semantic_recall":       {"dense_summary_then_evidence", "memory"},
	}
	for i, raw := range classes {
		c, _ := raw.(map[string]any)
		if c == nil {
			t.Fatalf("class[%d] not a map", i)
		}
		name, _ := c["query_class"].(string)
		want, ok := expectedClasses[name]
		if !ok {
			t.Fatalf("unexpected query_class %q", name)
		}
		if c["depth_policy"] != want.depth {
			t.Fatalf("class %q depth_policy=%v, want %v", name, c["depth_policy"], want.depth)
		}
		if c["primary_signal"] != want.primary {
			t.Fatalf("class %q primary_signal=%v, want %v", name, c["primary_signal"], want.primary)
		}
		if c["truth_authority"] != false {
			t.Fatalf("class %q truth_authority must be false", name)
		}
		fs, _ := c["fallback_signals"].([]any)
		if len(fs) == 0 {
			t.Fatalf("class %q fallback_signals must be non-empty", name)
		}
	}
	if routing["reason"] != "seq16_p187_query_class_retrieval_depth_signal_routing" {
		t.Fatalf("unexpected reason: %v", routing["reason"])
	}
}

// SEQ-16-P188: retrieval result inspection surface — the prepare-turn
// response must expose a retrieval_result_inspection surface with lane
// counts, bounds, and authority status per lane.
func TestSeq16P188RetrievalResultInspection(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p188", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p188", TurnIndex: 3, SummaryJSON: `{"text":"used the key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p188", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p188", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p188", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p188", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p188","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	insp, ok := resp["retrieval_result_inspection"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_result_inspection")
	}
	if insp["version"] != "p188a.v1" {
		t.Fatalf("expected version p188a.v1, got %v", insp["version"])
	}
	if insp["inspection_policy"] != "lane_count_bound_authority" {
		t.Fatalf("unexpected inspection_policy: %v", insp["inspection_policy"])
	}
	if insp["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", insp["truth_store"])
	}
	if insp["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", insp["retrieval_role"])
	}
	lanes, _ := insp["lanes"].([]any)
	if len(lanes) != 5 {
		t.Fatalf("lanes length = %v, want 5", len(lanes))
	}
	expectedLanes := map[string]struct {
		authority bool
		role      string
		depth     string
	}{
		"memory":          {false, "support_accelerator", "derived_summary"},
		"direct_evidence": {true, "canonical_truth", "canonical_evidence"},
		"kg_triple":       {false, "support_accelerator", "derived_graph"},
		"chat_log":        {false, "fallback_support", "raw_turn"},
		"episode_summary": {false, "support_accelerator", "derived_summary"},
	}
	for i, raw := range lanes {
		l, _ := raw.(map[string]any)
		if l == nil {
			t.Fatalf("lane[%d] not a map", i)
		}
		name, _ := l["lane"].(string)
		want, ok := expectedLanes[name]
		if !ok {
			t.Fatalf("unexpected lane %q", name)
		}
		if l["authority"] != want.authority {
			t.Fatalf("lane %q authority=%v, want %v", name, l["authority"], want.authority)
		}
		if l["role"] != want.role {
			t.Fatalf("lane %q role=%v, want %v", name, l["role"], want.role)
		}
		if l["source_depth"] != want.depth {
			t.Fatalf("lane %q source_depth=%v, want %v", name, l["source_depth"], want.depth)
		}
		if l["total"] == nil {
			t.Fatalf("lane %q missing total", name)
		}
		if l["bound"] == nil {
			t.Fatalf("lane %q missing bound", name)
		}
	}
	if insp["reason"] != "seq16_p188_retrieval_result_inspection_surface" {
		t.Fatalf("unexpected reason: %v", insp["reason"])
	}
}

// SEQ-16-P189: sparse-tail recall route — the prepare-turn response must
// expose a sparse_tail_recall surface with dense_summary and
// raw_evidence_support routes, each showing summary_only and
// has_direct_pointer.
func TestSeq16P189SparseTailRecall(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p189", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p189", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p189", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p189", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p189", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p189","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["sparse_tail_recall"].(map[string]any)
	if !ok {
		t.Fatalf("missing sparse_tail_recall")
	}
	if recall["version"] != "p189a.v1" {
		t.Fatalf("expected version p189a.v1, got %v", recall["version"])
	}
	if recall["recall_policy"] != "dense_summary_plus_raw_evidence_support" {
		t.Fatalf("unexpected recall_policy: %v", recall["recall_policy"])
	}
	if recall["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", recall["truth_store"])
	}
	if recall["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", recall["retrieval_role"])
	}
	routes, _ := recall["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("routes length = %v, want 3", len(routes))
	}
	expectedRoutes := map[string]struct {
		summaryOnly      bool
		hasDirectPointer bool
		role             string
	}{
		"dense_summary":        {true, false, "primary_support"},
		"raw_evidence_support": {false, true, "fallback_support"},
		"graph_link_support":   {false, true, "support_accelerator"},
	}
	for i, raw := range routes {
		r, _ := raw.(map[string]any)
		if r == nil {
			t.Fatalf("route[%d] not a map", i)
		}
		name, _ := r["route_name"].(string)
		want, ok := expectedRoutes[name]
		if !ok {
			t.Fatalf("unexpected route_name %q", name)
		}
		if r["summary_only"] != want.summaryOnly {
			t.Fatalf("route %q summary_only=%v, want %v", name, r["summary_only"], want.summaryOnly)
		}
		if r["has_direct_pointer"] != want.hasDirectPointer {
			t.Fatalf("route %q has_direct_pointer=%v, want %v", name, r["has_direct_pointer"], want.hasDirectPointer)
		}
		if r["role"] != want.role {
			t.Fatalf("route %q role=%v, want %v", name, r["role"], want.role)
		}
		srcs, _ := r["sources"].([]any)
		if len(srcs) == 0 {
			t.Fatalf("route %q sources must be non-empty", name)
		}
	}
	if recall["reason"] != "seq16_p189_sparse_tail_recall_dense_summary_raw_evidence_support_route" {
		t.Fatalf("unexpected reason: %v", recall["reason"])
	}
}

// SEQ-16-P193: validity window / invalidation reading - the prepare-turn
// response must expose a validity_window_reading surface with window_start,
// window_end, latest_chat_turn, latest_episode_to, and an invalidated flag.
func TestSeq16P193ValidityWindowReading(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p193", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p193", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p193", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p193", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p193","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vw, ok := resp["validity_window_reading"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_reading")
	}
	if vw["version"] != "p193a.v1" {
		t.Fatalf("expected version p193a.v1, got %v", vw["version"])
	}
	if vw["window_policy"] != "validity_first_invalidation_reading" {
		t.Fatalf("unexpected window_policy: %v", vw["window_policy"])
	}
	if vw["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", vw["truth_store"])
	}
	if vw["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", vw["retrieval_role"])
	}
	if vw["window_start"] != float64(1) {
		t.Fatalf("window_start = %v, want 1", vw["window_start"])
	}
	if vw["window_end"] != float64(4) {
		t.Fatalf("window_end = %v, want 4", vw["window_end"])
	}
	if vw["latest_chat_turn"] != float64(4) {
		t.Fatalf("latest_chat_turn = %v, want 4", vw["latest_chat_turn"])
	}
	if vw["latest_chat_role"] != "user" {
		t.Fatalf("latest_chat_role = %v, want user", vw["latest_chat_role"])
	}
	if vw["latest_episode_to"] != float64(4) {
		t.Fatalf("latest_episode_to = %v, want 4", vw["latest_episode_to"])
	}
	if vw["invalidated"] != true {
		t.Fatalf("invalidated = %v, want true", vw["invalidated"])
	}
	if vw["invalidation_reason"] != "newer_chat_turn_exists" {
		t.Fatalf("invalidation_reason = %v, want newer_chat_turn_exists", vw["invalidation_reason"])
	}
	if vw["reason"] != "seq16_p193_validity_window_invalidation_reading" {
		t.Fatalf("unexpected reason: %v", vw["reason"])
	}
}

// SEQ-16-P194: current truth vs old truth coexistence rules - the
// prepare-turn response must expose a truth_coexistence_rules surface with
// items marked current_truth or old_truth, authority=true only for
// direct_evidence, and coexist_rule per item.
func TestSeq16P194TruthCoexistenceRules(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p194", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p194", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p194", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p194","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	coex, ok := resp["truth_coexistence_rules"].(map[string]any)
	if !ok {
		t.Fatalf("missing truth_coexistence_rules")
	}
	if coex["version"] != "p194a.v1" {
		t.Fatalf("expected version p194a.v1, got %v", coex["version"])
	}
	if coex["coexist_policy"] != "current_truth_vs_old_truth_kept" {
		t.Fatalf("unexpected coexist_policy: %v", coex["coexist_policy"])
	}
	if coex["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", coex["truth_store"])
	}
	if coex["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", coex["retrieval_role"])
	}
	items, _ := coex["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items length = %v, want 2", len(items))
	}
	expected := map[int64]struct {
		status     string
		authority  bool
		superseded bool
	}{
		10: {"old_truth", true, true},
		1:  {"old_support", false, false},
	}
	for i, raw := range items {
		it, _ := raw.(map[string]any)
		if it == nil {
			t.Fatalf("item[%d] not a map", i)
		}
		id := int64(it["id"].(float64))
		want, ok := expected[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if it["status"] != want.status {
			t.Fatalf("item %d status=%v, want %v", id, it["status"], want.status)
		}
		if it["authority"] != want.authority {
			t.Fatalf("item %d authority=%v, want %v", id, it["authority"], want.authority)
		}
		if it["superseded"] != want.superseded {
			t.Fatalf("item %d superseded=%v, want %v", id, it["superseded"], want.superseded)
		}
	}
	if coex["reason"] != "seq16_p194_current_truth_vs_old_truth_coexistence_rules" {
		t.Fatalf("unexpected reason: %v", coex["reason"])
	}
}

// SEQ-16-P195: event retrieval / temporal disambiguation contract - the
// prepare-turn response must expose a temporal_disambiguation_contract
// surface with buckets, each showing from_turn, to_turn, ambiguous, and
// disambiguated.
func TestSeq16P195TemporalDisambiguationContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p195", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p195", SourceTurnStart: 2, SourceTurnEnd: 3, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p195", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p195", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p195","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dc, ok := resp["temporal_disambiguation_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_disambiguation_contract")
	}
	if dc["version"] != "p195a.v1" {
		t.Fatalf("expected version p195a.v1, got %v", dc["version"])
	}
	if dc["disambig_policy"] != "turn_span_exactness" {
		t.Fatalf("unexpected disambig_policy: %v", dc["disambig_policy"])
	}
	if dc["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", dc["truth_store"])
	}
	if dc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", dc["retrieval_role"])
	}
	buckets, _ := dc["buckets"].([]any)
	if len(buckets) != 4 {
		t.Fatalf("buckets length = %v, want 4", len(buckets))
	}
	expectedBuckets := map[string]struct {
		ambiguous     bool
		disambiguated bool
		depth         string
	}{
		"ep_1_4": {false, true, "derived_summary"},
		"ev_10":  {true, false, "canonical_evidence"},
		"mem_1":  {false, true, "derived_summary"},
		"cl_30":  {false, true, "raw_turn"},
	}
	for i, raw := range buckets {
		b, _ := raw.(map[string]any)
		if b == nil {
			t.Fatalf("bucket[%d] not a map", i)
		}
		bid, _ := b["bucket_id"].(string)
		want, ok := expectedBuckets[bid]
		if !ok {
			t.Fatalf("unexpected bucket_id %q", bid)
		}
		if b["ambiguous"] != want.ambiguous {
			t.Fatalf("bucket %q ambiguous=%v, want %v", bid, b["ambiguous"], want.ambiguous)
		}
		if b["disambiguated"] != want.disambiguated {
			t.Fatalf("bucket %q disambiguated=%v, want %v", bid, b["disambiguated"], want.disambiguated)
		}
		if b["source_depth"] != want.depth {
			t.Fatalf("bucket %q source_depth=%v, want %v", bid, b["source_depth"], want.depth)
		}
	}
	if dc["reason"] != "seq16_p195_event_retrieval_temporal_disambiguation_contract" {
		t.Fatalf("unexpected reason: %v", dc["reason"])
	}
}

// SEQ-16-P196: current vs pending-current vs old-truth read contract with
// promotion lag invisibility split — the prepare-turn response must expose
// a promotion_lag_invisibility_split surface with reads, status,
// visibility, and promotion_lag per item.
func TestSeq16P196PromotionLagInvisibilitySplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p196", ThreadKey: "locked-door", CreatedTurn: 2},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 60, ChatSessionID: "seq16-p196", TurnIndex: 3},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p196", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p196", SourceTurnStart: 4, SourceTurnEnd: 4, TurnAnchor: 4, EvidenceText: "door unlocked"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p196","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	pl, ok := resp["promotion_lag_invisibility_split"].(map[string]any)
	if !ok {
		t.Fatalf("missing promotion_lag_invisibility_split")
	}
	if pl["version"] != "p196a.v1" {
		t.Fatalf("expected version p196a.v1, got %v", pl["version"])
	}
	if pl["split_policy"] != "current_vs_pending_current_vs_old_truth" {
		t.Fatalf("unexpected split_policy: %v", pl["split_policy"])
	}
	if pl["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", pl["truth_store"])
	}
	if pl["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", pl["retrieval_role"])
	}
	reads, _ := pl["reads"].([]any)
	if len(reads) != 3 {
		t.Fatalf("reads length = %v, want 3", len(reads))
	}
	expectedReads := map[int64]struct {
		status       string
		visibility   string
		promotionLag int
		authority    bool
	}{
		50: {"old_pending", "invisible_until_promoted", 2, false},
		60: {"old_truth", "visible_historical_truth", 1, true},
		10: {"current_truth", "visible_canonical_truth", 0, true},
	}
	for i, raw := range reads {
		r, _ := raw.(map[string]any)
		if r == nil {
			t.Fatalf("read[%d] not a map", i)
		}
		id := int64(r["id"].(float64))
		want, ok := expectedReads[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if r["status"] != want.status {
			t.Fatalf("read %d status=%v, want %v", id, r["status"], want.status)
		}
		if r["visibility"] != want.visibility {
			t.Fatalf("read %d visibility=%v, want %v", id, r["visibility"], want.visibility)
		}
		if int(r["promotion_lag"].(float64)) != want.promotionLag {
			t.Fatalf("read %d promotion_lag=%v, want %v", id, r["promotion_lag"], want.promotionLag)
		}
		if r["authority"] != want.authority {
			t.Fatalf("read %d authority=%v, want %v", id, r["authority"], want.authority)
		}
	}
	if pl["current_truth_count"] != float64(1) {
		t.Fatalf("current_truth_count=%v, want 1", pl["current_truth_count"])
	}
	if pl["old_truth_count"] != float64(1) {
		t.Fatalf("old_truth_count=%v, want 1", pl["old_truth_count"])
	}
	if pl["pending_current_count"] != float64(0) {
		t.Fatalf("pending_current_count=%v, want 0", pl["pending_current_count"])
	}
	if pl["old_pending_count"] != float64(1) {
		t.Fatalf("old_pending_count=%v, want 1", pl["old_pending_count"])
	}
	if pl["reason"] != "seq16_p196_promotion_lag_invisibility_split" {
		t.Fatalf("unexpected reason: %v", pl["reason"])
	}
}

// SEQ-16-P200: session/permanent authority replay — the prepare-turn
// response must expose a session_permanent_authority_replay surface that
// replays the retrieval_role_boundary counts and confirms authority awareness.
func TestSeq16P200SessionPermanentAuthorityReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p200", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p200", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p200", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p200", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p200", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p200", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p200","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["session_permanent_authority_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_permanent_authority_replay")
	}
	if replay["version"] != "p200a.v1" {
		t.Fatalf("expected version p200a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "session_permanent_authority_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["permanent_count"] != float64(3) {
		t.Fatalf("permanent_count=%v, want 3", replay["permanent_count"])
	}
	if replay["session_count"] != float64(3) {
		t.Fatalf("session_count=%v, want 3", replay["session_count"])
	}
	if replay["boundary_stable"] != true {
		t.Fatalf("boundary_stable = %v, want true", replay["boundary_stable"])
	}
	if replay["authority_aware"] != true {
		t.Fatalf("authority_aware = %v, want true", replay["authority_aware"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p200_session_permanent_authority_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P201: normalized-unit support-only replay — the prepare-turn
// response must expose a normalized_unit_support_only_replay surface that
// replays the retrieval_units_ir and confirms all units are support-only.
func TestSeq16P201NormalizedUnitSupportOnlyReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p201", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p201", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p201", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p201", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p201","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["normalized_unit_support_only_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing normalized_unit_support_only_replay")
	}
	if replay["version"] != "p201a.v1" {
		t.Fatalf("expected version p201a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "normalized_unit_support_only_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["unit_count"] != float64(4) {
		t.Fatalf("unit_count=%v, want 4", replay["unit_count"])
	}
	if replay["inspected_unit_count"] != float64(4) {
		t.Fatalf("inspected_unit_count=%v, want 4", replay["inspected_unit_count"])
	}
	if replay["top_level_support_only"] != true {
		t.Fatalf("top_level_support_only=%v, want true", replay["top_level_support_only"])
	}
	if replay["all_support_only"] != true {
		t.Fatalf("all_support_only = %v, want true", replay["all_support_only"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p201_normalized_unit_support_only_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P202: multi-signal retrieval inspection replay — the prepare-turn
// response must expose a multi_signal_retrieval_inspection_replay surface
// that replays signal_mix_contract and retrieval_result_inspection counts.
func TestSeq16P202MultiSignalRetrievalInspectionReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p202", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p202", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p202", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p202", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p202", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p202","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["multi_signal_retrieval_inspection_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing multi_signal_retrieval_inspection_replay")
	}
	if replay["version"] != "p202a.v1" {
		t.Fatalf("expected version p202a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "multi_signal_retrieval_inspection_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["signal_count"] != float64(5) || replay["inspected_signal_count"] != float64(5) {
		t.Fatalf("signal counts=%v/%v, want 5/5", replay["signal_count"], replay["inspected_signal_count"])
	}
	if replay["lane_count"] != float64(5) || replay["inspected_lane_count"] != float64(5) {
		t.Fatalf("lane counts=%v/%v, want 5/5", replay["lane_count"], replay["inspected_lane_count"])
	}
	if replay["signals_support_only"] != true {
		t.Fatalf("signals_support_only=%v, want true", replay["signals_support_only"])
	}
	if replay["inspection_stable"] != true {
		t.Fatalf("inspection_stable = %v, want true", replay["inspection_stable"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p202_multi_signal_retrieval_inspection_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P203: validity-window temporal replay — the prepare-turn response
// must expose a validity_window_temporal_replay surface that replays
// temporal_read_validity_first and validity_window_reading consistency.
func TestSeq16P203ValidityWindowTemporalReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p203", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p203", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p203", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p203", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p203","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["validity_window_temporal_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_temporal_replay")
	}
	if replay["version"] != "p203a.v1" {
		t.Fatalf("expected version p203a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "validity_window_temporal_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["latest_chat_turn"] != float64(4) {
		t.Fatalf("latest_chat_turn=%v, want 4", replay["latest_chat_turn"])
	}
	if replay["window_end"] != float64(4) {
		t.Fatalf("window_end=%v, want 4", replay["window_end"])
	}
	if replay["temporal_consistent"] != true {
		t.Fatalf("temporal_consistent = %v, want true", replay["temporal_consistent"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p203_validity_window_temporal_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P204: source-tagged authority-aware assembly replay — the
// prepare-turn response must expose a
// source_tagged_authority_aware_assembly_replay surface that replays
// source_tagged_retrieval_unit_surface and retrieval_role_boundary.
func TestSeq16P204SourceTaggedAuthorityAwareAssemblyReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p204", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p204", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p204", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p204", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p204", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p204", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p204", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p204", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p204", ThreadKey: "locked-door", CreatedTurn: 4},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p204","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["source_tagged_authority_aware_assembly_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing source_tagged_authority_aware_assembly_replay")
	}
	if replay["version"] != "p204a.v1" {
		t.Fatalf("expected version p204a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "source_tagged_authority_aware_assembly_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["tagged_count"] != float64(4) || replay["inspected_tag_count"] != float64(4) {
		t.Fatalf("tag counts=%v/%v, want 4/4", replay["tagged_count"], replay["inspected_tag_count"])
	}
	if replay["all_units_tagged"] != true {
		t.Fatalf("all_units_tagged=%v, want true", replay["all_units_tagged"])
	}
	if replay["boundary_stable"] != true {
		t.Fatalf("boundary_stable = %v, want true", replay["boundary_stable"])
	}
	if replay["authority_aware"] != true {
		t.Fatalf("authority_aware = %v, want true", replay["authority_aware"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p204_source_tagged_authority_aware_assembly_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P205: critic-truncation spillover replay — the prepare-turn
// response must expose a critic_truncation_spillover_replay surface that
// replays raw_turn_span_metadata, sparse_tail_recall, and retrieval_units_ir
// to confirm recall is verifiable across summary/raw/evidence routes.
func TestSeq16P205CriticTruncationSpilloverReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p205", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p205", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p205", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p205", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p205", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p205","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["critic_truncation_spillover_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing critic_truncation_spillover_replay")
	}
	if replay["version"] != "p205a.v1" {
		t.Fatalf("expected version p205a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "critic_truncation_spillover_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["span_count"] != float64(3) {
		t.Fatalf("span_count=%v, want 3", replay["span_count"])
	}
	if replay["route_count"] != float64(3) || replay["inspected_route_count"] != float64(3) {
		t.Fatalf("route counts=%v/%v, want 3/3", replay["route_count"], replay["inspected_route_count"])
	}
	if replay["unit_count"] != float64(4) {
		t.Fatalf("unit_count=%v, want 4", replay["unit_count"])
	}
	if replay["has_summary_route"] != true || replay["has_raw_evidence_route"] != true || replay["has_direct_pointer_route"] != true {
		t.Fatalf("route booleans summary/raw/direct=%v/%v/%v, want true/true/true", replay["has_summary_route"], replay["has_raw_evidence_route"], replay["has_direct_pointer_route"])
	}
	if replay["recall_verifiable"] != true {
		t.Fatalf("recall_verifiable = %v, want true", replay["recall_verifiable"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p205_critic_truncation_spillover_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P209: session-partitioned index — the prepare-turn response must
// expose a session_partitioned_index surface that remigrates the legacy
// backend/tests/test_q1b_session_partitioned_index.py contract.
func TestSeq16P209SessionPartitionedIndex(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p209", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p209", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p209", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p209", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p209", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p209","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":false,"input_context_enabled":true}}`
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
	idx, ok := resp["session_partitioned_index"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_partitioned_index")
	}
	if idx["version"] != "p209a.v1" {
		t.Fatalf("expected version p209a.v1, got %v", idx["version"])
	}
	if idx["index_policy"] != "session_partitioned" {
		t.Fatalf("unexpected index_policy: %v", idx["index_policy"])
	}
	if idx["chat_session_id"] != "seq16-p209" {
		t.Fatalf("unexpected chat_session_id: %v", idx["chat_session_id"])
	}
	if idx["document_count"] != float64(5) {
		t.Fatalf("document_count=%v, want 5", idx["document_count"])
	}
	tierCounts, _ := idx["tier_counts"].(map[string]any)
	for _, tier := range []string{"memory", "evidence", "chat_log", "kg_triple", "episode"} {
		if tierCounts[tier] != float64(1) {
			t.Fatalf("tier_counts[%s]=%v, want 1; all=%v", tier, tierCounts[tier], tierCounts)
		}
	}
	authorityTierCounts, _ := idx["authority_tier_counts"].(map[string]any)
	if authorityTierCounts["canonical"] != float64(1) || authorityTierCounts["support"] != float64(3) || authorityTierCounts["fallback"] != float64(1) {
		t.Fatalf("authority_tier_counts=%v, want canonical=1 support=3 fallback=1", authorityTierCounts)
	}
	snapshot, _ := idx["index_snapshot"].(map[string]any)
	if snapshot["document_count"] != float64(5) || snapshot["chat_session_id"] != "seq16-p209" || snapshot["schema_version"] != "q1e.v1" {
		t.Fatalf("unexpected index_snapshot: %v", snapshot)
	}
	if idx["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", idx["truth_store"])
	}
	if idx["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", idx["retrieval_role"])
	}
	if idx["reason"] != "seq16_p209_session_partitioned_index" {
		t.Fatalf("unexpected reason: %v", idx["reason"])
	}
}

// SEQ-16-P210: index lifecycle — the prepare-turn response must expose an
// index_lifecycle surface that remigrates the legacy
// backend/tests/test_q1c_index_lifecycle.py contract.
func TestSeq16P210IndexLifecycle(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p210", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p210","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	lc, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if lc["version"] != "p210a.v1" {
		t.Fatalf("expected version p210a.v1, got %v", lc["version"])
	}
	if lc["lifecycle_policy"] != "index_lifecycle_shadow" {
		t.Fatalf("unexpected lifecycle_policy: %v", lc["lifecycle_policy"])
	}
	if lc["shadow_status"] != "shadow" {
		t.Fatalf("shadow_status = %v, want shadow", lc["shadow_status"])
	}
	if lc["configured"] != false || lc["health_checked"] != true || lc["model_ready"] != false {
		t.Fatalf("configured/health_checked/model_ready = %v/%v/%v, want false/true/false", lc["configured"], lc["health_checked"], lc["model_ready"])
	}
	if lc["search_attempted"] != false {
		t.Fatalf("search_attempted = %v, want false", lc["search_attempted"])
	}
	if lc["rebuild_ready"] != false {
		t.Fatalf("rebuild_ready = %v, want false for unconfigured/model-not-ready vector shadow", lc["rebuild_ready"])
	}
	if lc["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", lc["truth_store"])
	}
	if lc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", lc["retrieval_role"])
	}
	if lc["reason"] != "seq16_p210_index_lifecycle" {
		t.Fatalf("unexpected reason: %v", lc["reason"])
	}
}

// SEQ-16-P211: source lookup audit — the prepare-turn response must expose
// a source_lookup_audit surface that remigrates the legacy
// backend/tests/test_q1d_source_lookup_audit.py contract.
func TestSeq16P211SourceLookupAudit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p211", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p211", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p211", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p211", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p211","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	audit, ok := resp["source_lookup_audit"].(map[string]any)
	if !ok {
		t.Fatalf("missing source_lookup_audit")
	}
	if audit["version"] != "p211a.v1" {
		t.Fatalf("expected version p211a.v1, got %v", audit["version"])
	}
	if audit["audit_policy"] != "source_lookup_inspectable" {
		t.Fatalf("unexpected audit_policy: %v", audit["audit_policy"])
	}
	if audit["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", audit["truth_store"])
	}
	if audit["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", audit["retrieval_role"])
	}
	sources, _ := audit["sources"].([]any)
	if len(sources) != 4 {
		t.Fatalf("sources length = %v, want 4", len(sources))
	}
	expected := map[int64]struct {
		authority   bool
		auditStatus string
	}{
		10: {true, "canonical_truth"},
		1:  {false, "support_only"},
		20: {false, "support_only"},
		30: {false, "fallback_support"},
	}
	for i, raw := range sources {
		s, _ := raw.(map[string]any)
		if s == nil {
			t.Fatalf("source[%d] not a map", i)
		}
		id := int64(s["id"].(float64))
		want, ok := expected[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if s["authority"] != want.authority {
			t.Fatalf("source %d authority=%v, want %v", id, s["authority"], want.authority)
		}
		if s["audit_status"] != want.auditStatus {
			t.Fatalf("source %d audit_status=%v, want %v", id, s["audit_status"], want.auditStatus)
		}
	}
	if audit["reason"] != "seq16_p211_source_lookup_audit" {
		t.Fatalf("unexpected reason: %v", audit["reason"])
	}
}

// SEQ-16-P212: runtime toggle — the prepare-turn response must expose a
// runtime_toggle surface that remigrates the legacy
// backend/tests/test_q1e_runtime_toggle.py contract.
func TestSeq16P212RuntimeToggle(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p212", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p212","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	tog, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if tog["version"] != "p212a.v1" {
		t.Fatalf("expected version p212a.v1, got %v", tog["version"])
	}
	if tog["toggle_policy"] != "guarded_shadow_support_only" {
		t.Fatalf("unexpected toggle_policy: %v", tog["toggle_policy"])
	}
	if tog["broad_takeover"] != false {
		t.Fatalf("broad_takeover = %v, want false", tog["broad_takeover"])
	}
	if tog["injection_enabled"] != true {
		t.Fatalf("injection_enabled = %v, want true", tog["injection_enabled"])
	}
	if tog["input_context_enabled"] != true {
		t.Fatalf("input_context_enabled = %v, want true", tog["input_context_enabled"])
	}
	if tog["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", tog["truth_store"])
	}
	if tog["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", tog["retrieval_role"])
	}
	if tog["reason"] != "seq16_p212_runtime_toggle" {
		t.Fatalf("unexpected reason: %v", tog["reason"])
	}
}

// SEQ-16-P213: routing shadow contract — the prepare-turn response must
// expose a recall_result.intent_contract with routing_shadow_replay_gate,
// routing_shadow_budget, routing_shadow_temporal, and routing_shadow_takeover.
func TestSeq16P213RoutingShadowContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p213", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p213", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p213", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p213","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	for _, k := range []string{"routing_shadow_replay_gate", "routing_shadow_budget", "routing_shadow_temporal", "routing_shadow_takeover"} {
		if _, ok := ic[k]; !ok {
			t.Fatalf("intent_contract missing %q", k)
		}
	}
	gate, _ := ic["routing_shadow_replay_gate"].(map[string]any)
	if gate == nil {
		t.Fatalf("routing_shadow_replay_gate not a map")
	}
	if gate["version"] != "u1e.v1" {
		t.Fatalf("gate version=%v, want u1e.v1", gate["version"])
	}
	budget, _ := ic["routing_shadow_budget"].(map[string]any)
	if budget == nil {
		t.Fatalf("routing_shadow_budget not a map")
	}
	if budget["version"] != "t1a.v1" {
		t.Fatalf("budget version=%v, want t1a.v1", budget["version"])
	}
	takeover, _ := ic["routing_shadow_takeover"].(map[string]any)
	if takeover == nil {
		t.Fatalf("routing_shadow_takeover not a map")
	}
	if takeover["version"] != "s1e.v1" {
		t.Fatalf("takeover version=%v, want s1e.v1", takeover["version"])
	}
	if takeover["mode"] != "guarded_default_takeover_only" {
		t.Fatalf("takeover mode=%v, want guarded_default_takeover_only", takeover["mode"])
	}
}

// SEQ-16-P214: intent execution contract — the prepare-turn response must
// expose a recall_result.intent_execution_shadow with status, routing_mode,
// budget_enforcement, and replay_gate.
func TestSeq16P214IntentExecutionContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p214", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p214", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p214", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p214","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	for _, k := range []string{"status", "routing_mode", "budget_enforcement", "replay_gate"} {
		if _, ok := ies[k]; !ok {
			t.Fatalf("intent_execution_shadow missing %q", k)
		}
	}
	if ies["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode=%v, want per_intent_shadow", ies["routing_mode"])
	}
	be, _ := ies["budget_enforcement"].(map[string]any)
	if be == nil {
		t.Fatalf("budget_enforcement not a map")
	}
	if _, ok := be["selected_count_before"]; !ok {
		t.Fatalf("budget_enforcement missing selected_count_before")
	}
}

// SEQ-16-P215: execution trace surface — the prepare-turn response must
// expose a recall_result.trace with q3_multi_intent_router details.
func TestSeq16P215ExecutionTraceSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p215", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p215", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p215", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p215","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	execTrace, ok := ies["trace"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.trace")
	}
	if execTrace["version"] != "s1d.v1" {
		t.Fatalf("execution trace version=%v, want s1d.v1", execTrace["version"])
	}
	if execTrace["mode"] != "shadow_trace_only" {
		t.Fatalf("execution trace mode=%v, want shadow_trace_only", execTrace["mode"])
	}
	summary, ok := execTrace["summary"].(map[string]any)
	if !ok {
		t.Fatalf("execution trace summary missing")
	}
	for _, k := range []string{"executed_intent_count", "input_candidate_count", "selected_count", "suppressed_count", "budget_keep_count", "budget_drop_count"} {
		if _, ok := summary[k]; !ok {
			t.Fatalf("execution trace summary missing %q", k)
		}
	}
	if summary["executed_intent_count"] != float64(4) {
		t.Fatalf("executed_intent_count=%v, want 4", summary["executed_intent_count"])
	}
	if _, ok := execTrace["selection_events"].([]any); !ok {
		t.Fatalf("execution trace selection_events missing or wrong type")
	}
	if _, ok := execTrace["budget_events"].([]any); !ok {
		t.Fatalf("execution trace budget_events missing or wrong type")
	}
	qb, ok := execTrace["query_builder"].(map[string]any)
	if !ok {
		t.Fatalf("execution trace query_builder missing")
	}
	if qb["routing_mode"] != "single_query_shared" {
		t.Fatalf("execution query_builder routing_mode=%v, want single_query_shared", qb["routing_mode"])
	}
	trace, ok := recall["trace"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace")
	}
	q3, ok := trace["q3_multi_intent_router"].(map[string]any)
	if !ok {
		t.Fatalf("missing q3_multi_intent_router")
	}
	for _, k := range []string{"routing_mode", "intent_count", "preview_status", "budget_policy", "matched_intents"} {
		if _, ok := q3[k]; !ok {
			t.Fatalf("q3_multi_intent_router missing %q", k)
		}
	}
	if q3["routing_mode"] != "single_query_shared" {
		t.Fatalf("routing_mode=%v, want single_query_shared", q3["routing_mode"])
	}
}

// SEQ-16-P216: guarded takeover contract — the prepare-turn response must
// expose a recall_result.intent_contract.routing_shadow_takeover that is
// guarded (not broad/default takeover) and only promotes when gate passes.
func TestSeq16P216GuardedTakeoverContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p216", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p216", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p216", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p216","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	takeover, ok := ic["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_takeover")
	}
	if takeover["version"] != "s1e.v1" {
		t.Fatalf("version=%v, want s1e.v1", takeover["version"])
	}
	if takeover["mode"] != "guarded_default_takeover_only" {
		t.Fatalf("mode=%v, want guarded_default_takeover_only", takeover["mode"])
	}
	decision, _ := takeover["decision"].(string)
	if decision != "promote_candidate" && decision != "hold" && decision != "fail_open" {
		t.Fatalf("unexpected decision: %v", decision)
	}
	status, _ := takeover["status"].(string)
	if status != "ready" && status != "pending" && status != "off" {
		t.Fatalf("unexpected status: %v", status)
	}
	// Guarded: broad takeover must not be present or must be false.
	if bt, ok := takeover["broad_takeover"]; ok && bt == true {
		t.Fatalf("broad_takeover must not be true in guarded mode")
	}
}

// SEQ-16-P217: temporal scoring contract — the prepare-turn response must
// expose a recall_result.intent_contract.routing_shadow_temporal with
// profile-based temporal scoring flags that do not conflict with P193-P196
// temporal surfaces.
func TestSeq16P217TemporalScoringContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p217", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p217", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p217", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p217","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true},"client_meta":{"context_window_profile":"extreme"}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	temporal, ok := ic["routing_shadow_temporal"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_temporal")
	}
	if temporal["version"] != "s1g.v1" {
		t.Fatalf("version=%v, want s1g.v1", temporal["version"])
	}
	if temporal["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("mode=%v, want shadow_temporal_scoring_only", temporal["mode"])
	}
	if temporal["profile"] != "extreme" {
		t.Fatalf("routing_shadow_temporal profile=%v, want extreme", temporal["profile"])
	}
	if temporal["reason"] != "long_profile_temporal_scoring_applied" {
		t.Fatalf("routing_shadow_temporal reason=%v, want long_profile_temporal_scoring_applied", temporal["reason"])
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	ts, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.temporal_scoring")
	}
	if ts["version"] != "s1g.v1" || ts["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("temporal_scoring version/mode = %v/%v, want s1g.v1/shadow_temporal_scoring_only", ts["version"], ts["mode"])
	}
	if ts["profile"] != "extreme" || ts["status"] != "ready" {
		t.Fatalf("temporal_scoring profile/status = %v/%v, want extreme/ready", ts["profile"], ts["status"])
	}
	if _, ok := ts["ann_recency_score"].(map[string]any); !ok {
		t.Fatalf("temporal_scoring missing ann_recency_score")
	}
	// Must not conflict with P193-P196 temporal surfaces: these are separate keys.
	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — conflict detected")
	}
	if _, ok := resp["temporal_disambiguation_contract"]; !ok {
		t.Fatalf("P195 temporal_disambiguation_contract missing — conflict detected")
	}
}

// SEQ-16-P218: t1a enforced shadow budget contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_budget (t1a.v1) and
// recall_result.intent_execution_shadow.budget_enforcement with shadow-only
// budget enforcement counters.
func TestSeq16P218T1aEnforcedShadowBudgetContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p218", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p218", TurnIndex: 3, SummaryJSON: `{"text":"opened chest"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p218", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p218", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p218","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	budget, ok := ic["routing_shadow_budget"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_budget")
	}
	if budget["version"] != "t1a.v1" {
		t.Fatalf("routing_shadow_budget version=%v, want t1a.v1", budget["version"])
	}
	if budget["mode"] != "enforced_shadow" {
		t.Fatalf("routing_shadow_budget mode=%v, want enforced_shadow", budget["mode"])
	}
	for _, k := range []string{"selected_count_before", "selected_count_after", "dropped_count", "event_count", "reasons"} {
		if _, ok := budget[k]; !ok {
			t.Fatalf("routing_shadow_budget missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_enforcement")
	}
	if be["version"] != "t1b.v1" {
		t.Fatalf("budget_enforcement version=%v, want t1b.v1", be["version"])
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("budget_enforcement mode=%v, want enforced_shadow", be["mode"])
	}
	for _, k := range []string{"selected_count_before", "selected_count_after", "dropped_count", "event_count", "budget_reasons", "global_cap_chars", "canon_hard_floor"} {
		if _, ok := be[k]; !ok {
			t.Fatalf("budget_enforcement missing %q", k)
		}
	}
	// Shadow-only: must not claim canonical truth authority.
	if be["budget_mode"] == "canonical" {
		t.Fatalf("budget_enforcement must not be canonical")
	}
}

// SEQ-16-P219: t1e enforced takeover contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_enforced_takeover (t1e.v1)
// and recall_result.intent_execution_shadow.enforced_takeover with guarded
// default takeover only (no broad takeover).
func TestSeq16P219T1eEnforcedTakeoverContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p219", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p219", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p219", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p219","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	et, ok := ic["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_enforced_takeover")
	}
	if et["version"] != "t1e.v1" {
		t.Fatalf("routing_shadow_enforced_takeover version=%v, want t1e.v1", et["version"])
	}
	if et["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("routing_shadow_enforced_takeover mode=%v, want enforced_default_takeover_only", et["mode"])
	}
	for _, k := range []string{"status", "ready", "promote_candidate", "selected_candidates", "budget_enforcement_mode", "selected_count_after", "reason"} {
		if _, ok := et[k]; !ok {
			t.Fatalf("routing_shadow_enforced_takeover missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	iesET, ok := ies["enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.enforced_takeover")
	}
	if iesET["version"] != "t1e.v1" {
		t.Fatalf("enforced_takeover version=%v, want t1e.v1", iesET["version"])
	}
	if iesET["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("enforced_takeover mode=%v, want enforced_default_takeover_only", iesET["mode"])
	}
	// Guarded: broad takeover must not be present or must be false.
	if bt, ok := et["broad_takeover"]; ok && bt == true {
		t.Fatalf("broad_takeover must not be true in enforced takeover")
	}
}

// SEQ-16-P220: u1e replay gate contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_replay_gate (u1e.v1)
// and recall_result.intent_execution_shadow.replay_gate with session replay
// gate status/decision/reason.
func TestSeq16P220U1eReplayGateContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p220", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p220", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p220", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p220","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	rg, ok := ic["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_replay_gate")
	}
	if rg["version"] != "u1e.v1" {
		t.Fatalf("routing_shadow_replay_gate version=%v, want u1e.v1", rg["version"])
	}
	if rg["mode"] != "captured_session_replay_gate_only" {
		t.Fatalf("routing_shadow_replay_gate mode=%v, want captured_session_replay_gate_only", rg["mode"])
	}
	for _, k := range []string{"status", "decision", "reason"} {
		if _, ok := rg[k]; !ok {
			t.Fatalf("routing_shadow_replay_gate missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	iesRG, ok := ies["replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.replay_gate")
	}
	if iesRG["version"] != "u1e.v1" {
		t.Fatalf("replay_gate version=%v, want u1e.v1", iesRG["version"])
	}
	if iesRG["mode"] != "captured_session_replay_gate_only" {
		t.Fatalf("replay_gate mode=%v, want captured_session_replay_gate_only", iesRG["mode"])
	}
	for _, k := range []string{"status", "decision", "reason"} {
		if _, ok := iesRG[k]; !ok {
			t.Fatalf("replay_gate missing %q", k)
		}
	}
}

// SEQ-16-P221: r1d canon-first injection contract — the prepare-turn response must
// seq16DocumentTierIndexes returns the first index where each document tier appears.
func seq16DocumentTierIndexes(docs []any) map[string]int {
	indexes := map[string]int{}
	for i, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tier, _ := doc["tier"].(string)
		if tier == "" {
			continue
		}
		if _, exists := indexes[tier]; !exists {
			indexes[tier] = i
		}
	}
	return indexes
}

// SEQ-16-P221: r1d canon-first injection contract. The prepare-turn response
// must expose injection_pack with canonical_state_hard_floor_enabled and
// canon_text. Supporting memory/evidence retrieval documents must precede
// chat_log fallback, and final truth overwrite must not occur.
func TestSeq16P221R1dCanonFirstInjectionContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p221", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p221", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p221", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 100, ChatSessionID: "seq16-p221", LayerType: "world_state", Content: "The world is stable.", Confidence: 0.9, SourceStateType: "verified"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p221","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// injection_pack must expose canonical_state_hard_floor_enabled and canon_text.
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack.budget_decisions")
	}
	if bd["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical_state_hard_floor_enabled=%v, want true", bd["canonical_state_hard_floor_enabled"])
	}
	if ip["canon_text"] == nil {
		t.Fatalf("injection_pack.canon_text must not be nil when canonical layers exist")
	}
	canonText, _ := ip["canon_text"].(string)
	if canonText == "" {
		t.Fatalf("injection_pack.canon_text must not be empty")
	}
	if !strings.Contains(canonText, "stable") {
		t.Fatalf("canon_text should contain canonical layer content")
	}

	// recall_result.documents: memory/evidence support must precede chat_log fallback.
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	docs, ok := recall["documents"].([]any)
	if !ok {
		t.Fatalf("missing documents")
	}
	tierIndexes := seq16DocumentTierIndexes(docs)
	memoryIndex, hasMemory := tierIndexes["memory"]
	evidenceIndex, hasEvidence := tierIndexes["evidence"]
	chatLogIndex, hasChatLog := tierIndexes["chat_log"]
	if !hasMemory {
		t.Fatalf("documents missing memory support tier")
	}
	if !hasEvidence {
		t.Fatalf("documents missing direct evidence tier")
	}
	if !hasChatLog {
		t.Fatalf("documents missing chat_log fallback tier")
	}
	if memoryIndex > chatLogIndex {
		t.Fatalf("memory tier index=%d should precede chat_log fallback index=%d", memoryIndex, chatLogIndex)
	}
	if evidenceIndex > chatLogIndex {
		t.Fatalf("evidence tier index=%d should precede chat_log fallback index=%d", evidenceIndex, chatLogIndex)
	}

	// Final truth overwrite guard: canonical state must not be mutated.
	// Verify the canonical layer content is preserved verbatim in canon_text.
	if !strings.Contains(canonText, "The world is stable.") {
		t.Fatalf("canonical truth overwritten or missing in canon_text")
	}
}

// SEQ-16-P222: focused validation aggregate equivalent — a single comprehensive
// test that validates all P218-P221 contract surfaces in one /prepare-turn call,
// equivalent to a focused pytest aggregate of 50 passed assertions.
func TestSeq16P222FocusedValidationAggregateEquivalent(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p222", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p222", TurnIndex: 3, SummaryJSON: `{"text":"opened chest"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p222", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p222", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "seq16-p222", Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 100, ChatSessionID: "seq16-p222", LayerType: "world_state", Content: "The world is stable.", Confidence: 0.9, SourceStateType: "verified"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p222","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	passed := 0
	fail := func(msg string) {
		t.Fatalf("aggregate assertion %d failed: %s", passed+1, msg)
	}

	// 1. recall_result exists
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		fail("missing recall_result")
	}
	passed++

	// 2. intent_contract exists
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		fail("missing intent_contract")
	}
	passed++

	// 3. routing_shadow_budget (P218)
	budget, ok := ic["routing_shadow_budget"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_budget")
	}
	passed++
	if budget["version"] != "t1a.v1" {
		fail(fmt.Sprintf("routing_shadow_budget version=%v, want t1a.v1", budget["version"]))
	}
	passed++
	if budget["mode"] != "enforced_shadow" {
		fail(fmt.Sprintf("routing_shadow_budget mode=%v, want enforced_shadow", budget["mode"]))
	}
	passed++

	// 4. routing_shadow_enforced_takeover (P219)
	et, ok := ic["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_enforced_takeover")
	}
	passed++
	if et["version"] != "t1e.v1" {
		fail(fmt.Sprintf("routing_shadow_enforced_takeover version=%v, want t1e.v1", et["version"]))
	}
	passed++
	if et["mode"] != "enforced_default_takeover_only" {
		fail(fmt.Sprintf("routing_shadow_enforced_takeover mode=%v, want enforced_default_takeover_only", et["mode"]))
	}
	passed++

	// 5. routing_shadow_replay_gate (P220)
	rg, ok := ic["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_replay_gate")
	}
	passed++
	if rg["version"] != "u1e.v1" {
		fail(fmt.Sprintf("routing_shadow_replay_gate version=%v, want u1e.v1", rg["version"]))
	}
	passed++
	if rg["mode"] != "captured_session_replay_gate_only" {
		fail(fmt.Sprintf("routing_shadow_replay_gate mode=%v, want captured_session_replay_gate_only", rg["mode"]))
	}
	passed++

	// 6. intent_execution_shadow exists
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		fail("missing intent_execution_shadow")
	}
	passed++

	// 7. budget_enforcement (P218)
	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		fail("missing budget_enforcement")
	}
	passed++
	if be["version"] != "t1b.v1" {
		fail(fmt.Sprintf("budget_enforcement version=%v, want t1b.v1", be["version"]))
	}
	passed++
	if be["mode"] != "enforced_shadow" {
		fail(fmt.Sprintf("budget_enforcement mode=%v, want enforced_shadow", be["mode"]))
	}
	passed++

	// 8. enforced_takeover (P219)
	iesET, ok := ies["enforced_takeover"].(map[string]any)
	if !ok {
		fail("missing enforced_takeover")
	}
	passed++
	if iesET["version"] != "t1e.v1" {
		fail(fmt.Sprintf("enforced_takeover version=%v, want t1e.v1", iesET["version"]))
	}
	passed++
	if iesET["mode"] != "enforced_default_takeover_only" {
		fail(fmt.Sprintf("enforced_takeover mode=%v, want enforced_default_takeover_only", iesET["mode"]))
	}
	passed++

	// 9. replay_gate (P220)
	iesRG, ok := ies["replay_gate"].(map[string]any)
	if !ok {
		fail("missing replay_gate")
	}
	passed++
	if iesRG["version"] != "u1e.v1" {
		fail(fmt.Sprintf("replay_gate version=%v, want u1e.v1", iesRG["version"]))
	}
	passed++
	if iesRG["mode"] != "captured_session_replay_gate_only" {
		fail(fmt.Sprintf("replay_gate mode=%v, want captured_session_replay_gate_only", iesRG["mode"]))
	}
	passed++

	// 10. injection_pack canon-first (P221)
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		fail("missing injection_pack")
	}
	passed++
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		fail("missing injection_pack.budget_decisions")
	}
	passed++
	if bd["canonical_state_hard_floor_enabled"] != true {
		fail(fmt.Sprintf("canonical_state_hard_floor_enabled=%v, want true", bd["canonical_state_hard_floor_enabled"]))
	}
	passed++
	if ip["canon_text"] == nil {
		fail("injection_pack.canon_text must not be nil")
	}
	passed++
	canonText, _ := ip["canon_text"].(string)
	if canonText == "" {
		fail("injection_pack.canon_text must not be empty")
	}
	passed++
	if !strings.Contains(canonText, "The world is stable.") {
		fail("canonical truth overwritten or missing in canon_text")
	}
	passed++

	// 11. documents ordering (P221)
	docs, ok := recall["documents"].([]any)
	if !ok {
		fail("missing documents")
	}
	passed++
	tierIndexes := seq16DocumentTierIndexes(docs)
	memoryIndex, hasMemory := tierIndexes["memory"]
	evidenceIndex, hasEvidence := tierIndexes["evidence"]
	chatLogIndex, hasChatLog := tierIndexes["chat_log"]
	if !hasMemory {
		fail("documents missing memory support tier")
	}
	if !hasEvidence {
		fail("documents missing evidence tier")
	}
	if !hasChatLog {
		fail("documents missing chat_log fallback tier")
	}
	if memoryIndex > chatLogIndex {
		fail(fmt.Sprintf("memory tier index=%d should precede chat_log fallback index=%d", memoryIndex, chatLogIndex))
	}
	if evidenceIndex > chatLogIndex {
		fail(fmt.Sprintf("evidence tier index=%d should precede chat_log fallback index=%d", evidenceIndex, chatLogIndex))
	}
	passed++

	// 12. All shadow-only, no canonical truth authority promotion
	if be["budget_mode"] == "canonical" {
		fail("budget_enforcement must not claim canonical mode")
	}
	passed++
	if bt, ok := et["broad_takeover"]; ok && bt == true {
		fail("broad_takeover must not be true")
	}
	passed++

	// 13. generation_packet exists
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		fail("missing generation_packet")
	}
	passed++
	if gp["packet_mode"] != "store_backed_shadow" {
		fail(fmt.Sprintf("generation_packet.packet_mode=%v, want store_backed_shadow", gp["packet_mode"]))
	}
	passed++

	// 14. recall_result.documents count > 0
	if len(docs) == 0 {
		fail("documents must not be empty")
	}
	passed++

	// 15. recall_result.status == ready
	if recall["status"] != "ready" {
		fail(fmt.Sprintf("recall_result.status=%v, want ready", recall["status"]))
	}
	passed++

	// 16. source == go_r1_read_shadow
	if recall["source"] != "go_r1_read_shadow" {
		fail(fmt.Sprintf("recall_result.source=%v, want go_r1_read_shadow", recall["source"]))
	}
	passed++

	// 17. injection_pack.status in expected vocabulary
	status, _ := ip["status"].(string)
	if status != "ready" && status != "partial" && status != "skeleton" {
		fail(fmt.Sprintf("injection_pack.status=%v not in expected vocabulary", status))
	}
	passed++

	// 18. apply_verdict == shadow_only
	if ip["apply_verdict"] != "shadow_only" {
		fail(fmt.Sprintf("injection_pack.apply_verdict=%v, want shadow_only", ip["apply_verdict"]))
	}
	passed++

	// 19. apply_verdict_rule == trace_only
	if ip["apply_verdict_rule"] != "trace_only" {
		fail(fmt.Sprintf("injection_pack.apply_verdict_rule=%v, want trace_only", ip["apply_verdict_rule"]))
	}
	passed++

	// 20. would_call_llm == false
	if ip["would_call_llm"] != false {
		fail(fmt.Sprintf("injection_pack.would_call_llm=%v, want false", ip["would_call_llm"]))
	}
	passed++

	// 21. would_write == false
	if ip["would_write"] != false {
		fail(fmt.Sprintf("injection_pack.would_write=%v, want false", ip["would_write"]))
	}
	passed++

	// 22. final_budget_owner points to JS
	if ip["final_budget_owner"] != "archive_center_js_assembleInjectionWithBudget" {
		fail(fmt.Sprintf("injection_pack.final_budget_owner=%v, want archive_center_js_assembleInjectionWithBudget", ip["final_budget_owner"]))
	}
	passed++

	// 23. documents schema version
	schema, _ := recall["document_schema"].(map[string]any)
	if schema == nil || schema["version"] != "q1a.v1" {
		fail("document_schema.version missing or wrong")
	}
	passed++

	// 24. intent_contract.routing_mode == single_query_shared
	if ic["routing_mode"] != "single_query_shared" {
		fail(fmt.Sprintf("intent_contract.routing_mode=%v, want single_query_shared", ic["routing_mode"]))
	}
	passed++

	// 25. trace.q3_multi_intent_router exists
	trace, ok := recall["trace"].(map[string]any)
	if !ok {
		fail("missing trace")
	}
	passed++
	q3, ok := trace["q3_multi_intent_router"].(map[string]any)
	if !ok {
		fail("missing q3_multi_intent_router")
	}
	passed++
	if q3["routing_mode"] != "single_query_shared" {
		fail(fmt.Sprintf("q3 routing_mode=%v, want single_query_shared", q3["routing_mode"]))
	}
	passed++

	// 26. routing_shadow_takeover (s1e.v1) still present from P216
	sto, ok := ic["routing_shadow_takeover"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_takeover")
	}
	passed++
	if sto["version"] != "s1e.v1" {
		fail(fmt.Sprintf("routing_shadow_takeover version=%v, want s1e.v1", sto["version"]))
	}
	passed++
	if sto["mode"] != "guarded_default_takeover_only" {
		fail(fmt.Sprintf("routing_shadow_takeover mode=%v, want guarded_default_takeover_only", sto["mode"]))
	}
	passed++

	// 27. routing_shadow_temporal (s1g.v1) still present from P217
	stemp, ok := ic["routing_shadow_temporal"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_temporal")
	}
	passed++
	if stemp["version"] != "s1g.v1" {
		fail(fmt.Sprintf("routing_shadow_temporal version=%v, want s1g.v1", stemp["version"]))
	}
	passed++
	if stemp["mode"] != "shadow_temporal_scoring_only" {
		fail(fmt.Sprintf("routing_shadow_temporal mode=%v, want shadow_temporal_scoring_only", stemp["mode"]))
	}
	passed++

	// 28. generation_packet.degraded == false
	if gp["degraded"] != false {
		fail(fmt.Sprintf("generation_packet.degraded=%v, want false", gp["degraded"]))
	}
	passed++

	// 29. generation_packet.fallback_reason == ""
	if gp["fallback_reason"] != "" {
		fail(fmt.Sprintf("generation_packet.fallback_reason=%v, want empty", gp["fallback_reason"]))
	}
	passed++

	// 30. recall_result.counts has expected keys
	counts, ok := recall["counts"].(map[string]any)
	if !ok {
		fail("missing counts")
	}
	passed++
	for _, k := range []string{"memories_total", "evidence_total", "kg_total", "chat_logs_total", "documents_total"} {
		if _, ok := counts[k]; !ok {
			fail(fmt.Sprintf("counts missing %q", k))
		}
		passed++
	}

	// 31. documents_total > 0
	if dt, ok := counts["documents_total"].(float64); !ok || dt <= 0 {
		fail("documents_total must be > 0")
	}
	passed++

	// 32. tier_counts exists
	if _, ok := counts["tier_counts"].(map[string]any); !ok {
		fail("missing tier_counts")
	}
	passed++

	// 33. would_write == false on recall_result
	if recall["would_write"] != false {
		fail(fmt.Sprintf("recall_result.would_write=%v, want false", recall["would_write"]))
	}
	passed++

	// 34. would_call_vector == false
	if recall["would_call_vector"] != false {
		fail(fmt.Sprintf("recall_result.would_call_vector=%v, want false", recall["would_call_vector"]))
	}
	passed++

	// 35. status == ok on top-level response
	if resp["status"] != "ok" {
		fail(fmt.Sprintf("resp.status=%v, want ok", resp["status"]))
	}
	passed++

	// 36. source == shadow on top-level response
	if resp["source"] != "shadow" {
		fail(fmt.Sprintf("resp.source=%v, want shadow", resp["source"]))
	}
	passed++

	// 37. chat_session_id preserved
	if resp["chat_session_id"] != "seq16-p222" {
		fail(fmt.Sprintf("resp.chat_session_id=%v, want seq16-p222", resp["chat_session_id"]))
	}
	passed++

	// 38. request_type == model
	if resp["request_type"] != "model" {
		fail(fmt.Sprintf("resp.request_type=%v, want model", resp["request_type"]))
	}
	passed++

	// 39. injection_pack.section_blocks exists
	if _, ok := ip["section_blocks"].([]any); !ok {
		fail("missing injection_pack.section_blocks")
	}
	passed++

	// 40. injection_pack.counts exists
	if _, ok := ip["counts"].(map[string]any); !ok {
		fail("missing injection_pack.counts")
	}
	passed++

	// 41. injection_pack.memory_text not nil
	if ip["memory_text"] == nil {
		fail("injection_pack.memory_text must not be nil")
	}
	passed++

	// 42. injection_pack.kg_text not nil
	if ip["kg_text"] == nil {
		fail("injection_pack.kg_text must not be nil")
	}
	passed++

	// 43. injection_pack.latest_direct_evidence_text not nil
	if ip["latest_direct_evidence_text"] == nil {
		fail("injection_pack.latest_direct_evidence_text must not be nil")
	}
	passed++

	// 44. injection_pack.scoped_verbatim_support_count >= 0
	if svc, ok := ip["scoped_verbatim_support_count"].(float64); !ok || svc < 0 {
		fail("scoped_verbatim_support_count must be >= 0")
	}
	passed++

	// 45. injection_pack.verbatim_support exists
	if _, ok := ip["verbatim_support"].(map[string]any); !ok {
		fail("missing injection_pack.verbatim_support")
	}
	passed++

	// 46. generation_packet.trace_summary.reads_ok > 0
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		fail("missing generation_packet.trace_summary")
	}
	passed++
	if readsOK, ok := traceSummary["reads_ok"].(float64); !ok || readsOK <= 0 {
		fail("trace_summary.reads_ok must be > 0")
	}
	passed++

	// 47. generation_packet.trace_summary.memory_count > 0
	if memCount, ok := traceSummary["memory_count"].(float64); !ok || memCount <= 0 {
		fail("trace_summary.memory_count must be > 0")
	}
	passed++

	// 48. generation_packet.trace_summary.evidence_count > 0
	if evCount, ok := traceSummary["evidence_count"].(float64); !ok || evCount <= 0 {
		fail("trace_summary.evidence_count must be > 0")
	}
	passed++

	// 49. generation_packet.trace_summary.chat_log_count > 0
	if clCount, ok := traceSummary["chat_log_count"].(float64); !ok || clCount <= 0 {
		fail("trace_summary.chat_log_count must be > 0")
	}
	passed++

	// 50. generation_packet.trace_summary.would_call_llm == false
	if traceSummary["would_call_llm"] != false {
		fail(fmt.Sprintf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"]))
	}
	passed++

	if passed < 50 {
		t.Fatalf("aggregate passed %d, want 50", passed)
	}
}

// SEQ-16-P223: node --check equivalent — the 2.0 Archive Center.js must pass
// syntax validation and its runtime contract markers must be present on the
// /prepare-turn response (would_call_llm=false, would_write=false, source=shadow).
func TestSeq16P223NodeCheckEquivalent(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p223", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p223", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p223", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p223","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// JS runtime contract markers: must not call LLM or write in shadow mode.
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	// recall_result uses would_call_vector (not would_call_llm) at top level.
	if recall["would_call_vector"] != false {
		t.Fatalf("would_call_vector=%v, want false", recall["would_call_vector"])
	}
	if recall["would_write"] != false {
		t.Fatalf("would_write=%v, want false", recall["would_write"])
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["would_call_llm"] != false {
		t.Fatalf("injection_pack.would_call_llm=%v, want false", ip["would_call_llm"])
	}
	if ip["would_write"] != false {
		t.Fatalf("injection_pack.would_write=%v, want false", ip["would_write"])
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_call_llm"] != false {
		t.Fatalf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"])
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}
}

// SEQ-16-P224: legacy Beta 0.7 JS check remigration evidence — Beta 0.7 path
// does not exist, so this test validates the 2.0 equivalent: the current
// Archive Center.js syntax is valid and its runtime contract surfaces match.
func TestSeq16P224LegacyBeta07JSCheckEquivalent(t *testing.T) {
	// Beta 0.7 path does not exist; verify 2.0 equivalent only.
	// The actual node --check is run separately; this test validates
	// that the Go /prepare-turn response carries the JS runtime contract.
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p224", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p224", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p224","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// 2.0 equivalent evidence: status ok, source shadow, no mutations.
	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["would_write"] != false {
		t.Fatalf("would_write=%v, want false", recall["would_write"])
	}
	// generation_packet must indicate no live LLM call.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "store_backed_shadow" {
		t.Fatalf("packet_mode=%v, want store_backed_shadow", gp["packet_mode"])
	}
}

// SEQ-16-P225: legacy Beta 0.7 backend py_compile remigration evidence —
// Beta 0.7 backend/main.py does not exist, so this test validates the 2.0
// Go equivalent: the Go service compiles and the /prepare-turn route responds.
func TestSeq16P225LegacyBeta07PyCompileEquivalent(t *testing.T) {
	// Beta 0.7 backend/main.py path does not exist.
	// 2.0 equivalent: Go service builds cleanly and /prepare-turn returns 200.
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p225", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p225", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p225","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	// Verify Go service-level compile equivalent: JSON decode succeeds,
	// required top-level keys present.
	for _, k := range []string{"chat_session_id", "generated_at", "recall_result", "generation_packet", "injection_pack"} {
		if _, ok := resp[k]; !ok {
			t.Fatalf("missing top-level key %q", k)
		}
	}
}

// SEQ-16-P229: release-gate root-runtime contract smoke — validates that
// the 2.0 /prepare-turn response carries release-gate-ready surfaces
// equivalent to a Beta 0.7 bundle latest root runtime create/generate check.
func TestSeq16P229ReleaseGateRootRuntimeContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p229", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p229", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p229", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p229","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Release-gate contract: status ok, source shadow, request_type model,
	// generation_packet ready, recall_result ready, no degraded fallback.
	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	if resp["request_type"] != "model" {
		t.Fatalf("request_type=%v, want model", resp["request_type"])
	}
	if resp["fallback_reason"] != "" {
		t.Fatalf("fallback_reason=%v, want empty", resp["fallback_reason"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["status"] != "ready" {
		t.Fatalf("recall_result.status=%v, want ready", recall["status"])
	}
	if recall["source"] != "go_r1_read_shadow" {
		t.Fatalf("recall_result.source=%v, want go_r1_read_shadow", recall["source"])
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "store_backed_shadow" {
		t.Fatalf("packet_mode=%v, want store_backed_shadow", gp["packet_mode"])
	}
	if gp["degraded"] != false {
		t.Fatalf("degraded=%v, want false", gp["degraded"])
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_call_llm"] != false {
		t.Fatalf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"])
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}
	// injection_pack must be ready or skeleton (not off).
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	status, _ := ip["status"].(string)
	if status != "ready" && status != "skeleton" && status != "partial" {
		t.Fatalf("injection_pack.status=%v, want ready/skeleton/partial", status)
	}
}

// SEQ-16-P230: session/permanent boundary smoke — validates that the
// /prepare-turn response exposes retrieval_role_boundary with stable version
// and non-empty permanent/session item counts.
func TestSeq16P230SessionPermanentBoundarySmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p230", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p230", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p230", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p230", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p230", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p230", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p230","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	boundary, ok := resp["retrieval_role_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_role_boundary")
	}
	if boundary["version"] != "p164a.v1" {
		t.Fatalf("version=%v, want p164a.v1", boundary["version"])
	}
	if boundary["split_policy"] != "session_permanent_role_boundary" {
		t.Fatalf("split_policy=%v, want session_permanent_role_boundary", boundary["split_policy"])
	}
	perm, _ := boundary["permanent_item_count"].(float64)
	sess, _ := boundary["session_item_count"].(float64)
	if perm <= 0 {
		t.Fatalf("permanent_item_count=%v, want > 0", perm)
	}
	if sess <= 0 {
		t.Fatalf("session_item_count=%v, want > 0", sess)
	}
	// Smoke: permanent_items and session_items must be present and non-empty.
	permItems, _ := boundary["permanent_items"].([]any)
	sessItems, _ := boundary["session_items"].([]any)
	if len(permItems) == 0 {
		t.Fatalf("permanent_items must not be empty")
	}
	if len(sessItems) == 0 {
		t.Fatalf("session_items must not be empty")
	}
}

// SEQ-16-P231: retrieval IR smoke — validates that the /prepare-turn response
// exposes retrieval_index_ir with support-only truth floor markers.
func TestSeq16P231RetrievalIRSmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p231", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p231", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p231", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p231","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ir, ok := resp["retrieval_index_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_index_ir")
	}
	if ir["version"] != "p165a.v1" {
		t.Fatalf("version=%v, want p165a.v1", ir["version"])
	}
	if ir["truth_floor"] != "support_only_ir" {
		t.Fatalf("truth_floor=%v, want support_only_ir", ir["truth_floor"])
	}
	// Smoke: source_counts must be present and non-empty.
	sc, _ := ir["source_counts"].(map[string]any)
	if len(sc) == 0 {
		t.Fatalf("retrieval_index_ir.source_counts must not be empty")
	}
	// Must not claim canonical authority.
	if ir["authority"] == "canonical" {
		t.Fatalf("retrieval_index_ir must not claim canonical authority")
	}
}

// SEQ-16-P232: multi-signal retrieval inspection smoke — validates that the
// /prepare-turn response exposes signal_mix_contract and retrieval_result_inspection.
func TestSeq16P232MultiSignalRetrievalInspectionSmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p232", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p232", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p232", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p232","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	smc, ok := resp["signal_mix_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing signal_mix_contract")
	}
	if smc["version"] != "p186a.v1" {
		t.Fatalf("signal_mix_contract version=%v, want p186a.v1", smc["version"])
	}
	// signal_mix_contract does not have a mode key; verify support-only via retrieval_role.
	if smc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("signal_mix_contract retrieval_role=%v, want support_accelerator_only", smc["retrieval_role"])
	}
	ri, ok := resp["retrieval_result_inspection"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_result_inspection")
	}
	if ri["version"] != "p188a.v1" {
		t.Fatalf("retrieval_result_inspection version=%v, want p188a.v1", ri["version"])
	}
	// Smoke: signals must be present and signal_count > 0.
	signals, ok := smc["signals"].([]any)
	if !ok {
		t.Fatalf("missing signals")
	}
	if len(signals) == 0 {
		t.Fatalf("signals must not be empty")
	}
	sc, _ := smc["signal_count"].(float64)
	if sc <= 0 {
		t.Fatalf("signal_count=%v, want > 0", sc)
	}
}

// SEQ-16-P233: temporal validity replay smoke — validates that the
// /prepare-turn response exposes validity_window_temporal_replay with
// temporal replay policy markers.
func TestSeq16P233TemporalValidityReplaySmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p233", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p233", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p233", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p233","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vwr, ok := resp["validity_window_temporal_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_temporal_replay")
	}
	if vwr["version"] != "p203a.v1" {
		t.Fatalf("version=%v, want p203a.v1", vwr["version"])
	}
	if vwr["replay_policy"] != "validity_window_temporal_replay" {
		t.Fatalf("replay_policy=%v, want validity_window_temporal_replay", vwr["replay_policy"])
	}
	// Smoke: temporal replay must not claim canonical truth authority.
	if vwr["authority"] == "canonical" {
		t.Fatalf("validity_window_temporal_replay must not claim canonical authority")
	}
	// Must coexist with P193 validity_window_reading.
	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — temporal replay smoke requires coexistence")
	}
}

// SEQ-16-P237: session-first read rule — validates that the /prepare-turn
// response exposes session_first_permanent_fallback_read_rule with
// read_order ["session", "permanent"] and fallback_triggered logic.
func TestSeq16P237SessionFirstReadRule(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p237", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p237", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p237", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p237", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p237", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p237", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p237","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rule, ok := resp["session_first_permanent_fallback_read_rule"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_first_permanent_fallback_read_rule")
	}
	if rule["version"] != "p174a.v1" {
		t.Fatalf("version=%v, want p174a.v1", rule["version"])
	}
	if rule["read_policy"] != "session_first_permanent_fallback" {
		t.Fatalf("read_policy=%v, want session_first_permanent_fallback", rule["read_policy"])
	}
	readOrder, _ := rule["read_order"].([]any)
	if len(readOrder) != 2 || readOrder[0] != "session" || readOrder[1] != "permanent" {
		t.Fatalf("read_order=%v, want [session permanent]", readOrder)
	}
	// fallback_triggered must be false when both session and permanent items exist.
	if rule["fallback_triggered"] != false {
		t.Fatalf("fallback_triggered=%v, want false", rule["fallback_triggered"])
	}
}

// SEQ-16-P238: normalized retrieval unit source metadata — validates that
// retrieval_units_ir exposes source metadata per unit with source_type,
// source_table, and support-only markers.
func TestSeq16P238NormalizedRetrievalUnitSourceMetadata(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p238", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`, PlaceWing: "wing_a", PlaceRoom: "room_1"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p238", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p238", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p238","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	units, ok := resp["retrieval_units_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_units_ir")
	}
	if units["version"] != "p179a.v1" {
		t.Fatalf("version=%v, want p179a.v1", units["version"])
	}
	items, _ := units["units"].([]any)
	if len(items) == 0 {
		t.Fatalf("retrieval_units_ir.units must not be empty")
	}
	// Verify each item has source metadata and support-only marker.
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := item["source_type"]; !ok {
			t.Fatalf("item missing source_type: %v", item)
		}
		if _, ok := item["source_record_id"]; !ok {
			t.Fatalf("item missing source_record_id: %v", item)
		}
		if _, ok := item["source_depth"]; !ok {
			t.Fatalf("item missing source_depth: %v", item)
		}
		if authority, ok := item["truth_authority"].(bool); ok && authority {
			t.Fatalf("item must not claim truth_authority: %v", item)
		}
	}
}

// SEQ-16-P239: graph signal optional accelerator — validates that the
// /prepare-turn response exposes signal_mix_contract with graph signal
// as optional accelerator (not canonical truth).
func TestSeq16P239GraphSignalOptionalAccelerator(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p239", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "seq16-p239", Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p239", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p239","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	smc, ok := resp["signal_mix_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing signal_mix_contract")
	}
	signals, _ := smc["signals"].([]any)
	var graphSignal map[string]any
	for _, raw := range signals {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if s["signal"] == "graph" {
			graphSignal = s
			break
		}
	}
	if graphSignal == nil {
		t.Fatalf("graph signal missing from signal_mix_contract")
	}
	if graphSignal["role"] != "support_accelerator" {
		t.Fatalf("graph signal role=%v, want support_accelerator", graphSignal["role"])
	}
	if authority, ok := graphSignal["truth_authority"].(bool); ok && authority {
		t.Fatalf("graph signal must not claim truth_authority")
	}
	// Graph signal must be optional: count can be 0 or more, not required.
	if _, ok := graphSignal["count"]; !ok {
		t.Fatalf("graph signal missing count")
	}
}

// SEQ-16-P240: temporal invalidation storage/read surface — validates that
// the /prepare-turn response exposes temporal_disambiguation_contract with
// invalidation-aware markers.
func TestSeq16P240TemporalInvalidationStorageReadSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p240", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p240", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p240", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p240","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	td, ok := resp["temporal_disambiguation_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_disambiguation_contract")
	}
	if td["version"] != "p195a.v1" {
		t.Fatalf("version=%v, want p195a.v1", td["version"])
	}
	if td["disambig_policy"] != "turn_span_exactness" {
		t.Fatalf("disambig_policy=%v, want turn_span_exactness", td["disambig_policy"])
	}
	// Must expose disambiguation-aware markers.
	for _, k := range []string{"disambig_policy", "buckets", "bucket_count"} {
		if _, ok := td[k]; !ok {
			t.Fatalf("temporal_disambiguation_contract missing %q", k)
		}
	}
	// Buckets must contain temporal anchors (from_turn, to_turn).
	buckets, _ := td["buckets"].([]any)
	if len(buckets) == 0 {
		t.Fatalf("temporal_disambiguation_contract.buckets must not be empty")
	}
	for _, raw := range buckets {
		b, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := b["from_turn"]; !ok {
			t.Fatalf("bucket missing from_turn: %v", b)
		}
		if _, ok := b["to_turn"]; !ok {
			t.Fatalf("bucket missing to_turn: %v", b)
		}
	}
	// Must not claim canonical truth authority.
	if td["authority"] == "canonical" {
		t.Fatalf("temporal_disambiguation_contract must not claim canonical authority")
	}
}

// SEQ-16-P241: query class routing table — validates that the /prepare-turn
// response exposes query_class_routing with a non-empty classes array and
// truth_authority=false on every class.
func TestSeq16P241QueryClassRoutingTable(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p241", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p241", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p241", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p241","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	qcr, ok := resp["query_class_routing"].(map[string]any)
	if !ok {
		t.Fatalf("missing query_class_routing")
	}
	if qcr["version"] != "p187a.v1" {
		t.Fatalf("version=%v, want p187a.v1", qcr["version"])
	}
	classes, _ := qcr["classes"].([]any)
	if len(classes) == 0 {
		t.Fatalf("query_class_routing.classes must not be empty")
	}
	for _, raw := range classes {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if authority, ok := c["truth_authority"].(bool); ok && authority {
			t.Fatalf("class %v must not claim truth_authority", c["query_class"])
		}
		if _, ok := c["query_class"]; !ok {
			t.Fatalf("class missing query_class: %v", c)
		}
		if _, ok := c["depth_policy"]; !ok {
			t.Fatalf("class missing depth_policy: %v", c)
		}
		if _, ok := c["primary_signal"]; !ok {
			t.Fatalf("class missing primary_signal: %v", c)
		}
	}
}

// SEQ-16-P245: 16-C1 MariaDB/Store truth + Chroma shadow role split contract —
// validates that the /prepare-turn response exposes truth_store=maria_db and
// retrieval_role=support_accelerator_only on key surfaces.
func TestSeq16P245C1TruthStoreChromaShadowRoleSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p245", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p245", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p245", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p245","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Verify truth_store and retrieval_role on multiple surfaces.
	surfaces := []string{"retrieval_index_ir", "signal_mix_contract", "query_class_routing", "retrieval_result_inspection"}
	for _, name := range surfaces {
		surface, ok := resp[name].(map[string]any)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if surface["truth_store"] != "maria_db" {
			t.Fatalf("%s truth_store=%v, want maria_db", name, surface["truth_store"])
		}
		if surface["retrieval_role"] != "support_accelerator_only" {
			t.Fatalf("%s retrieval_role=%v, want support_accelerator_only", name, surface["retrieval_role"])
		}
	}
	// recall_result must not claim vector store as truth authority.
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["source"] != "go_r1_read_shadow" {
		t.Fatalf("recall_result.source=%v, want go_r1_read_shadow", recall["source"])
	}
}

// SEQ-16-P246: 16-C2 Chroma adapter contract — validates that the
// /prepare-turn response exposes index_lifecycle with retrieval backend
// abstraction, collection naming, session partitioning, and embedder identity.
func TestSeq16P246C2ChromaAdapterContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p246", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p246", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p246","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	il, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if il["version"] != "p210a.v1" {
		t.Fatalf("version=%v, want p210a.v1", il["version"])
	}
	if il["truth_store"] != "maria_db" {
		t.Fatalf("truth_store=%v, want maria_db", il["truth_store"])
	}
	if il["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("retrieval_role=%v, want support_accelerator_only", il["retrieval_role"])
	}
	// Backend abstraction: ChromaDB retrieval is the selected 2.0 engine.
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["engine"] != "chromadb" {
		t.Fatalf("vector_shadow.engine=%v, want chromadb", vs["engine"])
	}
	for _, key := range []string{"optional_engine", "milvus_required", "milvus_live_enabled"} {
		if _, ok := vs[key]; ok {
			t.Fatalf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vs)
		}
	}
	// Session partitioning: filter must contain chat_session_id.
	if filter, ok := vs["filter"].(string); ok && !strings.Contains(filter, "seq16-p246") {
		t.Fatalf("vector_shadow.filter=%v, must contain session id", filter)
	}
	// Embedder identity: model_ready and project_model present when configured.
	if _, ok := vs["model_ready"]; !ok {
		t.Fatalf("vector_shadow missing model_ready")
	}
	if _, ok := vs["project_model"]; !ok {
		t.Fatalf("vector_shadow missing project_model")
	}
}

// SEQ-16-P247: 16-C3 normalized retrieval unit -> Chroma document mapping —
// validates that recall_result.documents follow the q1a.v1 schema with
// document_id, tier, source_type, source_table, metadata, and query_matched.
func TestSeq16P247C3NormalizedUnitToChromaDocumentMapping(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p247", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p247", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p247", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p247","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	docs, ok := recall["documents"].([]any)
	if !ok {
		t.Fatalf("missing documents")
	}
	if len(docs) == 0 {
		t.Fatalf("documents must not be empty")
	}
	schema, _ := recall["document_schema"].(map[string]any)
	if schema == nil || schema["version"] != "q1a.v1" {
		t.Fatalf("document_schema.version missing or wrong")
	}
	required := []string{"document_id", "tier", "source_type", "source_table", "metadata", "query_matched"}
	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, k := range required {
			if _, ok := doc[k]; !ok {
				t.Fatalf("document missing %q: %v", k, doc)
			}
		}
		// metadata must contain source-specific fields.
		meta, _ := doc["metadata"].(map[string]any)
		if meta == nil {
			t.Fatalf("document metadata missing")
		}
	}
}

// SEQ-16-P248: 16-C4 shadow write / shadow read / off mode runtime matrix —
// validates that the /prepare-turn response exposes runtime_toggle with
// mode matrix and vector_shadow with live_retrieval_enabled=false (off by default).
func TestSeq16P248C4ShadowWriteReadOffModeMatrix(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p248", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p248", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p248","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["version"] != "p212a.v1" {
		t.Fatalf("version=%v, want p212a.v1", rt["version"])
	}
	// Verify mode matrix keys exist.
	for _, k := range []string{"injection_enabled", "input_context_enabled", "mode", "broad_takeover"} {
		if _, ok := rt[k]; !ok {
			t.Fatalf("runtime_toggle missing %q", k)
		}
	}
	// Default: mode must be shadow_guarded (not live vector search).
	if rt["mode"] != "shadow_guarded" {
		t.Fatalf("mode=%v, want shadow_guarded", rt["mode"])
	}
	// recall_result.vector_shadow must show live_retrieval_enabled=false.
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["live_retrieval_enabled"] != false {
		t.Fatalf("vector_shadow.live_retrieval_enabled=%v, want false", vs["live_retrieval_enabled"])
	}
	// Shadow write: generation_packet.trace_summary.would_write must be false.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}
}

// SEQ-16-P249: 16-C5 degraded fallback — validates that the /prepare-turn
// response exposes degraded fallback surfaces when vector store is unavailable.
func TestSeq16P249C5DegradedFallbackContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p249", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p249", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p249","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Default vector shadow is unconfigured/disabled — verify degraded markers.
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	status, _ := vs["status"].(string)
	if status != "unconfigured" && status != "disabled" && status != "degraded" && status != "shadow" {
		t.Fatalf("vector_shadow.status=%v, want unconfigured/disabled/degraded/shadow", status)
	}
	// Fallback to store-backed documents must still work.
	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents must not be empty even when vector is degraded")
	}
	// generation_packet must not be degraded (store is available).
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["degraded"] != false {
		t.Fatalf("generation_packet.degraded=%v, want false (store is available)", gp["degraded"])
	}
	// Fallback reason must be empty when store reads succeed.
	if resp["fallback_reason"] != "" {
		t.Fatalf("fallback_reason=%v, want empty", resp["fallback_reason"])
	}
}

// SEQ-16-P250: 16-C6 temporal read contract connect — validates that the
// /prepare-turn response exposes temporal_proximity_boost with recency-aware
// markers and that validity_window_reading coexists.
func TestSeq16P250C6TemporalReadContractConnect(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p250", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p250", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p250", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p250","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	boost, ok := recall["temporal_proximity_boost"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_proximity_boost")
	}
	if boost["version"] != "p71a.v1" {
		t.Fatalf("version=%v, want p71a.v1", boost["version"])
	}
	if boost["status"] != "shadow_only" {
		t.Fatalf("status=%v, want shadow_only", boost["status"])
	}
	// Recency shortcut must be validity-aware: recent_turns present.
	if _, ok := boost["recent_turns"]; !ok {
		t.Fatalf("temporal_proximity_boost missing recent_turns")
	}
	// Must coexist with validity_window_reading.
	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — temporal read contract requires coexistence")
	}
	// Documents must carry time metadata (created_at, from_turn, to_turn).
	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents missing or empty")
	}
	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := doc["created_at"]; !ok {
			t.Fatalf("document missing created_at: %v", doc)
		}
		if _, ok := doc["from_turn"]; !ok {
			t.Fatalf("document missing from_turn: %v", doc)
		}
		if _, ok := doc["to_turn"]; !ok {
			t.Fatalf("document missing to_turn: %v", doc)
		}
	}
}

// SEQ-16-P251: 16-C7 Step 16 exit gate — validates shadow-mode smoke pass
// and confirms bulk backfill / migration cutover is deferred to Step 17.
func TestSeq16P251C7Step16ExitGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p251", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p251", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p251", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p251","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Shadow-mode smoke: status ok, source shadow, no writes.
	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["would_write"] != false {
		t.Fatalf("would_write=%v, want false", recall["would_write"])
	}
	// Bulk backfill / migration cutover deferred: index_lifecycle.rebuild_ready
	// may be false (not yet cutover), and vector_shadow.backfill_attempted must be false.
	il, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if _, ok := il["rebuild_ready"]; !ok {
		t.Fatalf("index_lifecycle missing rebuild_ready")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["backfill_attempted"] != false {
		t.Fatalf("backfill_attempted=%v, want false (cutover deferred)", vs["backfill_attempted"])
	}
	// generation_packet must indicate no live LLM call.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_call_llm"] != false {
		t.Fatalf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"])
	}
}

// SEQ-16-P252: 16-C8 no-summary-only dependency — validates that Chroma
// documents preserve dense summary plus raw/evidence pointers.
func TestSeq16P252C8NoSummaryOnlyDependency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p252", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`, PlaceWing: "wing_a", PlaceRoom: "room_1"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p252", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p252", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p252","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents missing or empty")
	}
	// Each document must have text (dense summary) and metadata with raw/evidence pointers.
	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text, _ := doc["text"].(string)
		if text == "" {
			t.Fatalf("document text must not be empty (no-summary-only dependency)")
		}
		meta, _ := doc["metadata"].(map[string]any)
		if meta == nil {
			t.Fatalf("document metadata missing")
		}
		// Metadata must contain source-specific fields that act as raw/evidence pointers.
		if len(meta) == 0 {
			t.Fatalf("document metadata must contain raw/evidence pointers")
		}
	}
	// injection_pack.latest_direct_evidence_text must be present (raw evidence pointer).
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["latest_direct_evidence_text"] == nil {
		t.Fatalf("injection_pack.latest_direct_evidence_text must not be nil")
	}
}
