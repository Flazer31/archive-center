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

// SEQ-16.5-P115: optional anchor slot define — [Scene], [Entity], [Active Thread], [Chapter], [Saga]
// validates that input_context_text may contain optional anchor slots when
// activeStates, canonicalLayers, or episodeSums are present.
func TestSeq165P115OptionalAnchorSlot(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq165-p115", StateType: "scene", TurnIndex: 2, Content: "dark forest"},
			{ID: 2, ChatSessionID: "seq165-p115", StateType: "entity", TurnIndex: 3, Content: "mysterious stranger"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 10, ChatSessionID: "seq165-p115", LayerType: "world_state", TurnIndex: 1, Content: "raining"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 20, ChatSessionID: "seq165-p115", FromTurn: 1, ToTurn: 3, SummaryText: "entering the woods"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 25, ChatSessionID: "seq165-p115", ThreadKey: "north-path", CreatedTurn: 3, Status: "open"},
		},
		returnStorylines: []store.Storyline{
			{ID: 26, ChatSessionID: "seq165-p115", Name: "forest saga", LastTurn: 3, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p115", TurnIndex: 4, Role: "user", Content: "go north"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p115","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}

	if !strings.Contains(ict, "[Active States]") {
		t.Fatalf("input_context_text missing [Active States] optional slot")
	}
	if !strings.Contains(ict, "[Canonical State Layers]") {
		t.Fatalf("input_context_text missing [Canonical State Layers] optional slot")
	}
	if !strings.Contains(ict, "[Episode Summaries]") {
		t.Fatalf("input_context_text missing [Episode Summaries] optional slot")
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundActive := false
	foundCanon := false
	foundEpisode := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		switch kind {
		case "active_state":
			foundActive = true
		case "canonical_layer":
			foundCanon = true
		case "episode_summary":
			foundEpisode = true
		}
	}
	if !foundActive {
		t.Fatalf("continuity_pack.items missing active_state")
	}
	if !foundCanon {
		t.Fatalf("continuity_pack.items missing canonical_layer")
	}
	if !foundEpisode {
		t.Fatalf("continuity_pack.items missing episode_summary")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	optional, _ := gov["optional_slots"].([]any)
	for _, name := range []string{"Scene", "Entity", "Active Thread", "Chapter", "Saga"} {
		slot := seq165FindSlot(t, optional, name)
		if slot["marker"] != "["+name+"]" {
			t.Fatalf("%s marker mismatch: %v", name, slot)
		}
		if slot["selected"] != true {
			t.Fatalf("%s slot selected=%v, want true: %v", name, slot["selected"], slot)
		}
	}
}

// SEQ-16.5-P116: weak-input / temporal / resume / explicit-user-input slot promotion/demotion
// validates that trace_preview and generation_packet.trace_summary expose
// slot promotion/demotion decisions (e.g., storyline_selected_count vs dropped_count).
func TestSeq165P116SlotPromotionDemotion(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p116", Name: "main arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p116", Name: "old arc", LastTurn: 2, Status: "dormant"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p116", TurnIndex: 11, Role: "user", Content: "continue"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p116","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}
	if _, ok := ss["selected_count"]; !ok {
		t.Fatalf("storyline_selection missing selected_count")
	}
	if _, ok := ss["dropped_count"]; !ok {
		t.Fatalf("storyline_selection missing dropped_count")
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if _, ok := traceSummary["storyline_selected_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_selected_count")
	}
	if _, ok := traceSummary["storyline_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_dropped_count")
	}
	if _, ok := traceSummary["storyline_stale_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_stale_dropped_count")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	rules := seq165Map(t, gov, "promotion_demotion_rules")
	for _, key := range []string{"weak_input", "temporal_query", "resume", "explicit_user_input"} {
		if _, ok := rules[key]; !ok {
			t.Fatalf("promotion_demotion_rules missing %q", key)
		}
	}
}

// SEQ-16.5-P117: input context max slot / max char policy — short-and-sharp anchor lane preserve
// validates that generation_packet.trace_summary exposes max_input_context_chars
// and input_context_truncated flags.
func TestSeq165P117InputContextMaxSlotMaxCharPolicy(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p117", TurnIndex: 1, Role: "user", Content: strings.Repeat("a", 500)},
			{ID: 2, ChatSessionID: "seq165-p117", TurnIndex: 2, Role: "user", Content: strings.Repeat("b", 500)},
			{ID: 3, ChatSessionID: "seq165-p117", TurnIndex: 3, Role: "user", Content: strings.Repeat("c", 500)},
			{ID: 4, ChatSessionID: "seq165-p117", TurnIndex: 4, Role: "user", Content: strings.Repeat("d", 500)},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p117","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_input_context_chars":400}}`
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
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	maxChars, ok := traceSummary["max_input_context_chars"].(float64)
	if !ok || maxChars <= 0 {
		t.Fatalf("max_input_context_chars=%v, want > 0", traceSummary["max_input_context_chars"])
	}
	if maxChars != 400 {
		t.Fatalf("max_input_context_chars=%v, want 400", maxChars)
	}

	if traceSummary["input_context_truncated"] != true {
		t.Fatalf("input_context_truncated=%v, want true", traceSummary["input_context_truncated"])
	}

	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	policy := seq165Map(t, gov, "slot_policy")
	if policy["max_chars"] != float64(400) {
		t.Fatalf("slot_policy.max_chars=%v, want 400", policy["max_chars"])
	}
	if policy["input_context_truncated"] != true {
		t.Fatalf("slot_policy.input_context_truncated=%v, want true", policy["input_context_truncated"])
	}
	if policy["short_and_sharp_anchor_lane_preserve"] != true {
		t.Fatalf("slot_policy short_and_sharp_anchor_lane_preserve=%v, want true", policy["short_and_sharp_anchor_lane_preserve"])
	}
}

// SEQ-16.5-P118: helper injection anchor suppression — validates that
// injection_pack helper text does not contain anchor slot markers, and that
// input_context and injection are separate lanes.
func TestSeq165P118HelperInjectionAnchorSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p118", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p118", TurnIndex: 4, Role: "user", Content: "open door"},
		},
		returnResumePack: &store.ResumePack{
			Trigger: "resume", AssembledText: "Resume text here.",
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p118","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}

	injText, _ := ip["injection_text"].(string)
	for _, marker := range []string{"[Temporal Anchor]", "[Previous]", "[Scene]", "[Entity]", "[Active Thread]", "[Chapter]", "[Saga]", "[Resume Pack]", "[Direct Evidence]", "[Recent Chat]", "[Active States]", "[Canonical State Layers]", "[Episode Summaries]"} {
		if strings.Contains(injText, marker) {
			t.Fatalf("injection_text contains anchor marker %q (helper must be suppressed from anchors)", marker)
		}
	}

	if _, ok := resp["input_context_text"]; !ok {
		t.Fatalf("missing input_context_text (input context lane must be separate)")
	}

	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	suppression := seq165Map(t, gov, "helper_injection_anchor_suppression")
	if suppression["enabled"] != true {
		t.Fatalf("helper_injection_anchor_suppression.enabled=%v, want true", suppression["enabled"])
	}
	suppressed, _ := suppression["suppressed_markers"].([]any)
	seq165FindSlotLikeMarker := false
	for _, marker := range suppressed {
		if marker == "[Temporal Anchor]" {
			seq165FindSlotLikeMarker = true
		}
	}
	if !seq165FindSlotLikeMarker {
		t.Fatalf("suppressed_markers missing [Temporal Anchor]: %v", suppressed)
	}
}

// SEQ-16.5-P119: explicit user redirection stale arc anchor demotion — validates that
// resolved/dormant storylines do not dominate input_context or injection_pack,
// and that progression_ledger.lifecycle_model handles stale arcs.
func TestSeq165P119ExplicitUserRedirectionStaleArcAnchorDemotion(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p119", Name: "current arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p119", Name: "stale arc", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p119", TurnIndex: 11, Role: "user", Content: "let's move on"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p119","turn_index":1,"raw_user_input":"let's move on","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	lm, ok := pl["lifecycle_model"].(map[string]any)
	if !ok {
		t.Fatalf("missing lifecycle_model")
	}
	states, _ := lm["states"].([]any)
	hasResolved := false
	hasDormant := false
	for _, s := range states {
		if s == "resolved" {
			hasResolved = true
		}
		if s == "dormant" {
			hasDormant = true
		}
	}
	if !hasResolved {
		t.Fatalf("lifecycle_model.states missing 'resolved'")
	}
	if !hasDormant {
		t.Fatalf("lifecycle_model.states missing 'dormant'")
	}

	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}
	if ss["dropped_count"] == 0 {
		t.Fatalf("storyline_selection dropped_count=%v, want > 0 (stale arc should be dropped)", ss["dropped_count"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	redirection := seq165Map(t, gov, "explicit_user_redirection")
	if redirection["detected"] != true || redirection["stale_arc_demotes"] != true || redirection["current_user_input_wins"] != true {
		t.Fatalf("explicit_user_redirection mismatch: %v", redirection)
	}
	oldArcTrace, _ := gov["old_arc_keep_drop_trace"].([]any)
	foundStaleDrop := false
	for _, raw := range oldArcTrace {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["status"] == "resolved" && item["decision"] == "drop" && item["reason"] == "stale_or_resolved_arc_demoted" {
			foundStaleDrop = true
		}
	}
	if !foundStaleDrop {
		t.Fatalf("old_arc_keep_drop_trace missing resolved drop: %v", oldArcTrace)
	}
}

// SEQ-16.5-P123: helper budget need/risk breakdown trace schema — validates that
// injection_pack.budget_decisions exposes reason_counts and section_blocks
// show per-lane budget breakdown.
func TestSeq165P123HelperBudgetNeedRiskBreakdownTraceSchema(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p123", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p123", TurnIndex: 4, Role: "user", Content: "open door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p123","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}

	if _, ok := bd["reason_counts"]; !ok {
		t.Fatalf("budget_decisions missing reason_counts")
	}

	blocks, _ := ip["section_blocks"].([]any)
	if len(blocks) == 0 {
		t.Fatalf("section_blocks must not be empty")
	}
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := block["budget"]; !ok {
			t.Fatalf("section_block missing budget: %v", block)
		}
		if _, ok := block["chars"]; !ok {
			t.Fatalf("section_block missing chars: %v", block)
		}
	}
	helperTrace := seq165Map(t, resp, "helper_budget_governor_trace")
	if helperTrace["version"] != "seq16_5_helper_budget_trace.v1" {
		t.Fatalf("helper_budget_governor_trace version=%v", helperTrace["version"])
	}
	if helperTrace["truth_authority"] != false {
		t.Fatalf("helper_budget_governor_trace truth_authority=%v, want false", helperTrace["truth_authority"])
	}
	if _, ok := helperTrace["need_breakdown"].(map[string]any); !ok {
		t.Fatalf("helper_budget_governor_trace missing need_breakdown")
	}
	if _, ok := helperTrace["risk_breakdown"].(map[string]any); !ok {
		t.Fatalf("helper_budget_governor_trace missing risk_breakdown")
	}
}

// SEQ-16.5-P124: input anchor selected/dropped reason trace — validates that
// continuity_pack.items expose kind labels (selected) and that
// input_context_text construction implies slot selection.
func TestSeq165P124InputAnchorSelectedDroppedReasonTrace(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p124", TurnIndex: 3, Role: "user", Content: "hello"},
		},
		returnResumePack: &store.ResumePack{
			Trigger: "resume", AssembledText: "Resume summary.",
		},
		returnActiveStates: []store.ActiveState{
			{ID: 10, ChatSessionID: "seq165-p124", StateType: "scene", TurnIndex: 2, Content: "forest"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 20, ChatSessionID: "seq165-p124", LayerType: "world_state", TurnIndex: 1, Content: "rain"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 30, ChatSessionID: "seq165-p124", FromTurn: 1, ToTurn: 2, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p124","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("continuity_pack.items must not be empty")
	}

	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := item["kind"]; !ok {
			t.Fatalf("continuity_pack item missing kind: %v", item)
		}
	}

	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Fatalf("input_context_text missing [Recent Chat] (selected slot)")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	selected, _ := gov["selected_anchor_trace"].([]any)
	dropped, _ := gov["dropped_anchor_trace"].([]any)
	if len(selected) == 0 {
		t.Fatalf("selected_anchor_trace must not be empty")
	}
	if len(dropped) == 0 {
		t.Fatalf("dropped_anchor_trace must not be empty")
	}
	for _, raw := range append(selected, dropped...) {
		trace, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if trace["slot"] == "" || trace["reason"] == "" {
			t.Fatalf("anchor trace missing slot/reason: %v", trace)
		}
	}
}

// SEQ-16.5-P125: preview / dashboard / transparency surface — validates that
// trace_preview exists and exposes key preview fields for dashboard consumption.
func TestSeq165P125PreviewDashboardTransparencySurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p125", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p125", TurnIndex: 4, Role: "user", Content: "open door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p125","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}

	for _, k := range []string{"source", "evidence_counts", "section_summary", "supervisor_status", "critic_status"} {
		if _, ok := tp[k]; !ok {
			t.Fatalf("trace_preview missing %q", k)
		}
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	for _, k := range []string{"memory_count", "evidence_count", "chat_log_count", "max_injection_chars", "max_input_context_chars"} {
		if _, ok := traceSummary[k]; !ok {
			t.Fatalf("trace_summary missing %q", k)
		}
	}
	if _, ok := resp["input_anchor_governor"].(map[string]any); !ok {
		t.Fatalf("missing input_anchor_governor transparency surface")
	}
	if _, ok := resp["helper_budget_governor_trace"].(map[string]any); !ok {
		t.Fatalf("missing helper_budget_governor_trace transparency surface")
	}
}

// SEQ-16.5-P126: support lane truth lane wording/display guard — validates that
// retrieval_extend_authority and progression_ledger.supporting_precedence_guard
// use support-only wording and do not claim canonical truth.
func TestSeq165P126SupportLaneTruthWordingDisplayGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p126", Name: "arc", LastTurn: 4, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p126", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p126","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	rea, ok := resp["retrieval_extend_authority"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_extend_authority")
	}
	if rea["version"] != "p168a.v1" {
		t.Fatalf("retrieval_extend_authority version=%v, want p168a.v1", rea["version"])
	}

	order, _ := rea["authority_order"].([]any)
	foundSupport := false
	for _, o := range order {
		if o == "support" {
			foundSupport = true
		}
	}
	if !foundSupport {
		t.Fatalf("authority_order missing 'support': %v", order)
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	guard, ok := pl["supporting_precedence_guard"].(map[string]any)
	if !ok {
		t.Fatalf("missing supporting_precedence_guard")
	}
	if guard["supporting_only"] != true {
		t.Fatalf("supporting_precedence_guard supporting_only=%v, want true", guard["supporting_only"])
	}

	disallowed, _ := guard["disallowed_usage"].([]any)
	for _, d := range disallowed {
		if d == "truth_overwrite" || d == "canonical_override" {

		} else {
			continue
		}
	}
	foundTruthOverwrite := false
	foundCanonicalOverride := false
	for _, d := range disallowed {
		if d == "truth_overwrite" {
			foundTruthOverwrite = true
		}
		if d == "canonical_override" {
			foundCanonicalOverride = true
		}
	}
	if !foundTruthOverwrite {
		t.Fatalf("supporting_precedence_guard disallowed_usage missing truth_overwrite")
	}
	if !foundCanonicalOverride {
		t.Fatalf("supporting_precedence_guard disallowed_usage missing canonical_override")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	wording := seq165Map(t, gov, "support_lane_wording_guard")
	if wording["truth_lane_label_forbidden"] != true {
		t.Fatalf("support_lane_wording_guard truth_lane_label_forbidden=%v, want true", wording["truth_lane_label_forbidden"])
	}
	if wording["canonical_truth_wording_allowed"] != false {
		t.Fatalf("support_lane_wording_guard canonical_truth_wording_allowed=%v, want false", wording["canonical_truth_wording_allowed"])
	}
}

// SEQ-16.5-P127: old-arc keep/drop reason trace — validates that
// trace_preview.storyline_selection exposes keep/drop reasons for old arcs.
func TestSeq165P127OldArcKeepDropReasonTrace(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p127", Name: "active arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p127", Name: "stale arc", LastTurn: 2, Status: "dormant"},
			{ID: 3, ChatSessionID: "seq165-p127", Name: "resolved arc", LastTurn: 3, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p127", TurnIndex: 11, Role: "user", Content: "continue"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}

	if _, ok := ss["selected_count"]; !ok {
		t.Fatalf("storyline_selection missing selected_count")
	}
	if _, ok := ss["dropped_count"]; !ok {
		t.Fatalf("storyline_selection missing dropped_count")
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if _, ok := traceSummary["storyline_stale_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_stale_dropped_count")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	oldArcTrace, _ := gov["old_arc_keep_drop_trace"].([]any)
	kept := false
	dropped := false
	for _, raw := range oldArcTrace {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["decision"] == "keep" && item["reason"] == "active_arc_anchor" {
			kept = true
		}
		if item["decision"] == "drop" && item["reason"] == "stale_or_resolved_arc_demoted" {
			dropped = true
		}
	}
	if !kept || !dropped {
		t.Fatalf("old_arc_keep_drop_trace kept=%v dropped=%v trace=%v", kept, dropped, oldArcTrace)
	}
}

// SEQ-16.5-P131: weak-input continuity replay — validates that continuity_pack
// exposes resume_pack, episode_summary, and chat_log items for weak-input continuity.
func TestSeq165P131WeakInputContinuityReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnResumePack: &store.ResumePack{
			Trigger: "weak_input", AssembledText: "Weak input resume.",
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "seq165-p131", FromTurn: 1, ToTurn: 3, SummaryText: "early episode"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p131", TurnIndex: 4, Role: "user", Content: "hi"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p131","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundResume := false
	foundEpisode := false
	foundChat := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		switch kind {
		case "resume_pack":
			foundResume = true
		case "episode_summary":
			foundEpisode = true
		case "chat_log":
			foundChat = true
		}
	}
	if !foundResume {
		t.Fatalf("continuity_pack.items missing resume_pack")
	}
	if !foundEpisode {
		t.Fatalf("continuity_pack.items missing episode_summary")
	}
	if !foundChat {
		t.Fatalf("continuity_pack.items missing chat_log")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	mandatory, _ := gov["mandatory_slots"].([]any)
	if seq165FindSlot(t, mandatory, "Temporal Anchor")["selected"] != true {
		t.Fatalf("Temporal Anchor should be selected for weak-input continuity")
	}
	if seq165FindSlot(t, mandatory, "Previous")["selected"] != true {
		t.Fatalf("Previous should be selected for weak-input continuity")
	}
}

// SEQ-16.5-P132: temporal query replay — validates that temporal_read_validity_first,
// validity_window_reading, and temporal_disambiguation_contract surfaces are present.
func TestSeq165P132TemporalQueryReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p132", TurnIndex: 1, Role: "user", Content: "start"},
			{ID: 2, ChatSessionID: "seq165-p132", TurnIndex: 2, Role: "user", Content: "middle"},
			{ID: 3, ChatSessionID: "seq165-p132", TurnIndex: 3, Role: "user", Content: "end"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 10, ChatSessionID: "seq165-p132", FromTurn: 1, ToTurn: 3, SummaryText: "full episode"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	for _, key := range []string{"temporal_read_validity_first", "validity_window_reading", "temporal_disambiguation_contract"} {
		if _, ok := resp[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}

	trvf, ok := resp["temporal_read_validity_first"].(map[string]any)
	if !ok {
		t.Fatalf("temporal_read_validity_first not a map")
	}
	if _, ok := trvf["latest_chat_turn"]; !ok {
		t.Fatalf("temporal_read_validity_first missing latest_chat_turn")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	rules := seq165Map(t, gov, "promotion_demotion_rules")
	if !strings.Contains(rules["temporal_query"].(string), "temporal_anchor") {
		t.Fatalf("temporal_query rule does not promote temporal anchor: %v", rules["temporal_query"])
	}
}

// SEQ-16.5-P133: long-gap resume replay — validates that resumePack is present
// in both continuity_pack and input_context_text for long-gap resume scenarios.
func TestSeq165P133LongGapResumeReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnResumePack: &store.ResumePack{
			Trigger: "long_gap", AssembledText: "Long gap resume summary.",
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p133", TurnIndex: 1, Role: "user", Content: "old"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	if !strings.Contains(ict, "[Resume Pack]") {
		t.Fatalf("input_context_text missing [Resume Pack]")
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundResume := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["kind"] == "resume_pack" {
			foundResume = true
		}
	}
	if !foundResume {
		t.Fatalf("continuity_pack.items missing resume_pack")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	previous := seq165FindSlot(t, gov["mandatory_slots"].([]any), "Previous")
	if previous["selected"] != true || previous["reason"] != "resume_pack_available" {
		t.Fatalf("Previous slot mismatch for long-gap resume: %v", previous)
	}
}

// SEQ-16.5-P134: multi-entity / multi-thread replay — validates that
// session_state and continuity_pack expose multiple entities and threads.
func TestSeq165P134MultiEntityMultiThreadReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "seq165-p134", CharacterName: "Alice", TurnIndex: 1},
			{ID: 2, ChatSessionID: "seq165-p134", CharacterName: "Bob", TurnIndex: 2},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 10, ChatSessionID: "seq165-p134", ThreadKey: "thread-a", CreatedTurn: 1, Status: "open"},
			{ID: 11, ChatSessionID: "seq165-p134", ThreadKey: "thread-b", CreatedTurn: 2, Status: "open"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 20, ChatSessionID: "seq165-p134", StateType: "scene", TurnIndex: 1, Content: "forest"},
			{ID: 21, ChatSessionID: "seq165-p134", StateType: "entity", TurnIndex: 2, Content: "wolf"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 30, ChatSessionID: "seq165-p134", LayerType: "world_state", TurnIndex: 1, Content: "night"},
			{ID: 31, ChatSessionID: "seq165-p134", LayerType: "entity_state", TurnIndex: 2, Content: "angry"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 40, ChatSessionID: "seq165-p134", TurnIndex: 3, Role: "user", Content: "run"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p134","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_state")
	}
	sectionMeta, ok := ss["section_meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_state.section_meta")
	}
	if sectionMeta["character_count"].(float64) != 2 {
		t.Fatalf("character_count=%v, want 2", sectionMeta["character_count"])
	}
	if sectionMeta["pending_thread_count"].(float64) != 2 {
		t.Fatalf("pending_thread_count=%v, want 2", sectionMeta["pending_thread_count"])
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	activeCount := 0
	canonCount := 0
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		if kind == "active_state" {
			activeCount++
		}
		if kind == "canonical_layer" {
			canonCount++
		}
	}
	if activeCount < 2 {
		t.Fatalf("continuity_pack active_state items=%d, want >= 2", activeCount)
	}
	if canonCount < 2 {
		t.Fatalf("continuity_pack canonical_layer items=%d, want >= 2", canonCount)
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	optional := gov["optional_slots"].([]any)
	for _, name := range []string{"Scene", "Entity", "Active Thread"} {
		if seq165FindSlot(t, optional, name)["selected"] != true {
			t.Fatalf("%s should be selected for multi-entity/multi-thread replay", name)
		}
	}
}
