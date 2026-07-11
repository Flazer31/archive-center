package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	src, ok := tv["recency_signal_source"].(string)
	if !ok || src == "" {
		t.Fatalf("recency_signal_source missing or not a string: %v", tv)
	}

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
