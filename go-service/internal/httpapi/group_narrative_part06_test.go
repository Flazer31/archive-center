package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestNarrativeControlGetExpiredForbiddenAccumulate(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-expired",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-expired", Name: "Arc A", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-expired", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	expired, _ := director["expired_forbidden"].([]any)
	found := false
	for _, e := range expired {
		if e == "Do not abruptly resolve: Risk A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected expired_forbidden to include previously forbidden risk, got %v", expired)
	}
	forbidden, _ := director["forbidden_moves"].([]any)
	for _, f := range forbidden {
		if f == "Do not abruptly resolve: Risk A" {
			t.Fatal("expected forbidden_moves to NOT include expired risk")
		}
	}
}

func TestNarrativeControlGetCompactHistorySummarizesResolvedWithoutActiveLeak(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "strong",
		"execution_checklist": []any{"keep visible beat"},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []string{"confession pressure", "promise debt"},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-compact",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-compact", Name: "Resolved Arc", Status: "resolved", CurrentContext: "Resolved arc full context should not stay active", Confidence: 0.9, EvidenceCount: 4, LastTurn: 10, LastEvidenceTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-compact", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	for _, item := range director["required_outcomes"].([]any) {
		if item == "Carry forward: Hook A" {
			t.Fatal("resolved hook leaked back into required_outcomes")
		}
	}
	for _, item := range director["forbidden_moves"].([]any) {
		if item == "Do not abruptly resolve: Risk A" {
			t.Fatal("expired risk leaked back into forbidden_moves")
		}
	}
	history, _ := resp["compact_history"].([]any)
	if !containsAnyStringSubstring(history, "Resolved: Carry forward: Hook A") ||
		!containsAnyStringSubstring(history, "Forbidden expired: Do not abruptly resolve: Risk A") ||
		!containsAnyStringSubstring(history, "Resolved arc: Resolved Arc resolved at turn 10") {
		t.Fatalf("compact_history missing resolved/expired continuity summaries: %+v", history)
	}
	meta, _ := resp["compact_history_meta"].(map[string]any)
	if meta["total_records"] == float64(0) {
		t.Fatalf("compact_history_meta total_records = 0: %+v", meta)
	}
	if avg, _ := meta["avg_emotional_weight"].(float64); avg <= 1.0 {
		t.Fatalf("avg_emotional_weight = %v, want > 1.0: %+v", avg, meta)
	}
	if strings.Contains(strings.Join(anySliceToStringSlice(history), "\n"), "Resolved arc full context should not stay active") {
		t.Fatalf("compact_history leaked full resolved context: %+v", history)
	}
}

func TestBuildNarrativeCompactHistoryEmotionWeightPriority(t *testing.T) {
	history, meta := buildNarrativeCompactHistory(
		map[string]any{"active_tensions": []string{"fear", "promise", "public pressure"}},
		map[string]any{"pressure_level": "strong", "resolved_outcomes": []string{}, "expired_forbidden": []string{}, "last_turn": 12},
		[]store.Storyline{
			{ChatSessionID: "sess-weight", Name: "Low Weight Arc", Status: "resolved", Confidence: 0.1, EvidenceCount: 1, LastTurn: 11},
			{ChatSessionID: "sess-weight", Name: "High Weight Arc", Status: "resolved", Confidence: 0.95, EvidenceCount: 8, LastTurn: 10},
		},
		nil,
	)
	if len(history) < 2 {
		t.Fatalf("history len = %d, want >= 2: %+v", len(history), history)
	}
	if !strings.Contains(history[0], "High Weight Arc") {
		t.Fatalf("emotion/importance weighting did not prioritize high-weight arc: %+v", history)
	}
	if meta["emotion_weight_strategy"] == "" {
		t.Fatalf("compact meta missing emotion weight strategy: %+v", meta)
	}
}

func anySliceToStringSlice(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func TestDirectorPatchUpdatesState(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-patch",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	patchBody := map[string]any{
		"scene_mandate":     "Patched mandate",
		"required_outcomes": []string{"Patched outcome"},
		"pressure_level":    "strong",
	}
	bodyJSON, _ := json.Marshal(patchBody)
	req := httptest.NewRequest(http.MethodPatch, "/narrative-control/sess-patch/director-patch", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["patched"] != true {
		t.Fatalf("patched = %v, want true", resp["patched"])
	}
	if resp["state_status"] != "user_patched" {
		t.Fatalf("state_status = %v, want user_patched", resp["state_status"])
	}

	director, _ := resp["director"].(map[string]any)
	if director["scene_mandate"] != "Patched mandate" {
		t.Fatalf("scene_mandate = %v, want Patched mandate", director["scene_mandate"])
	}

	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected saved guidance plan state after patch")
	}
	if fake.savedGuidancePlanState.StateStatus != "user_patched" {
		t.Fatalf("saved state_status = %v, want user_patched", fake.savedGuidancePlanState.StateStatus)
	}
	if fake.savedGuidancePlanState.LastTurn != 5 {
		t.Fatalf("saved last_turn = %d, want 5", fake.savedGuidancePlanState.LastTurn)
	}
}

func TestDirectorPatchUserPatchedCacheProtection(t *testing.T) {
	cachedDirector := map[string]any{
		"scene_mandate":       "User patched mandate",
		"required_outcomes":   []string{"User outcome"},
		"forbidden_moves":     []string{},
		"pressure_level":      "strong",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(cachedDirector)
	planJSON, _ := json.Marshal(cachedPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-userpatched",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "user_patched",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-userpatched", Name: "Arc B", LastTurn: 20, FirstTurn: 6},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-userpatched", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "user_patched" {
		t.Fatalf("state_status = %v, want user_patched", resp["state_status"])
	}
	director, _ := resp["director"].(map[string]any)
	if director["scene_mandate"] != "User patched mandate" {
		t.Fatalf("scene_mandate = %v, want User patched mandate (cache protected)", director["scene_mandate"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("expected no upsert when user_patched cache is protected")
	}
}

func TestNarrativeControlGetDirectorFreshCacheKeepsCompactHistory(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "hold continuity",
		"active_tensions":    []string{"tension"},
		"next_beats":         []string{"next beat"},
		"continuity_anchors": []string{"anchor"},
		"last_plan_turn":     9,
		"state_status":       "ready",
	}
	cachedDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "steady",
		"execution_checklist": []string{"Deliver a visible beat."},
		"persona_guardrails":  []string{"[Chloe] speaks dry"},
		"world_guardrails":    []string{"World rule [gravity]: stable"},
		"focus_characters":    []string{"Chloe"},
		"last_turn":           9,
		"state_status":        "ready",
		"resolved_outcomes":   []string{"Carry forward: Old Hook"},
		"expired_forbidden":   []string{"Do not abruptly resolve: Old Risk"},
	}
	planJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDirector)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-k3-cache",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      9,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-k3-cache", Name: "Arc A", Status: "active", LastTurn: 9, FirstTurn: 1},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-cache", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	if !containsAnyStringValue(director["required_outcomes"].([]any), "Carry forward: Hook A") {
		t.Fatalf("required_outcomes not preserved: %+v", director["required_outcomes"])
	}
	if !containsAnyStringValue(director["forbidden_moves"].([]any), "Do not abruptly resolve: Risk A") {
		t.Fatalf("forbidden_moves not preserved: %+v", director["forbidden_moves"])
	}
	if !containsAnyStringValue(director["resolved_outcomes"].([]any), "Carry forward: Old Hook") {
		t.Fatalf("resolved_outcomes not preserved: %+v", director["resolved_outcomes"])
	}
	if !containsAnyStringValue(director["expired_forbidden"].([]any), "Do not abruptly resolve: Old Risk") {
		t.Fatalf("expired_forbidden not preserved: %+v", director["expired_forbidden"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("fresh director cache should not be rewritten")
	}
}

func TestDirectorPatchPreservesStoryPlanWarningsAndIgnoresUnknownFields(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Old mandate",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc": "Arc A",
		"next_beats":  []string{"do not lose this"},
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]string{"keep warning"})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-k3-patch-preserve",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      4,
			WarningsJSON:  string(warnJSON),
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"scene_mandate":"Patched","story_plan":{"current_arc":"malicious overwrite"},"unknown_field":"ignored"}`
	req := httptest.NewRequest(http.MethodPatch, "/narrative-control/sess-k3-patch-preserve/director-patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected saved guidance state")
	}
	if fake.savedGuidancePlanState.StoryPlanJSON != string(planJSON) {
		t.Fatalf("story plan was not preserved: %s", fake.savedGuidancePlanState.StoryPlanJSON)
	}
	if fake.savedGuidancePlanState.WarningsJSON != string(warnJSON) {
		t.Fatalf("warnings were not preserved: %s", fake.savedGuidancePlanState.WarningsJSON)
	}
	var savedDirector map[string]any
	if err := json.Unmarshal([]byte(fake.savedGuidancePlanState.DirectorJSON), &savedDirector); err != nil {
		t.Fatalf("saved director unmarshal: %v", err)
	}
	if savedDirector["story_plan"] != nil || savedDirector["unknown_field"] != nil {
		t.Fatalf("unknown fields leaked into director: %+v", savedDirector)
	}
	if savedDirector["scene_mandate"] != "Patched" {
		t.Fatalf("scene_mandate = %v, want Patched", savedDirector["scene_mandate"])
	}
}

func TestNarrativeControlGetDirectorPressureStrongFromPinnedHooks(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ChatSessionID: "sess-k3-pressure", Name: "Arc Pressure", Status: "active", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-k3-pressure", Title: "Hook A", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 10},
			{ChatSessionID: "sess-k3-pressure", Title: "Hook B", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 10},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-pressure", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	if director["pressure_level"] != "strong" {
		t.Fatalf("pressure_level = %v, want strong", director["pressure_level"])
	}
	required, _ := director["required_outcomes"].([]any)
	if len(required) < 2 {
		t.Fatalf("required_outcomes = %+v, want two pinned hook carry targets", required)
	}
}

func TestNarrativeControlGetDirectorGuardrailsBoundedAndPersonaFromCharacterState(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ChatSessionID:  "sess-k3-guardrails",
				Name:           "Arc A",
				Status:         "active",
				CurrentContext: "Chloe weighs the next move.",
				EntitiesJSON:   `["Chloe"]`,
				LastTurn:       12,
				FirstTurn:      1,
			},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-k3-guardrails", Title: "Hook A", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 12},
			{ChatSessionID: "sess-k3-guardrails", Title: "Risk A", HookType: "risk", Status: "open", LastSeenTurn: 11},
		},
		characterStates: []store.CharacterState{
			{
				ChatSessionID:   "sess-k3-guardrails",
				CharacterName:   "Chloe",
				SpeechStyleJSON: `{"default_tone":"dry","speech_notes":"short replies"}`,
				PersonalityJSON: `{"core_trait":"guarded"}`,
				TurnIndex:       12,
			},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "gravity", ValueJSON: `{"description":"gravity is stable"}`},
			{ID: 2, ChatSessionID: "sess-k3-guardrails", Category: "systems", Key: "oaths", ValueJSON: `{"description":"oaths bind public action"}`},
			{ID: 3, ChatSessionID: "sess-k3-guardrails", Category: "exists", Key: "tower", ValueJSON: `{"description":"the tower exists"}`},
			{ID: 4, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "rain", ValueJSON: `{"description":"rain muffles sound"}`},
			{ID: 5, ChatSessionID: "sess-k3-guardrails", Category: "hidden", Key: "secret", ValueJSON: `{"description":"must not carry"}`},
			{ID: 6, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "suppressed", ValueJSON: `{"description":"suppressed"}`, Suppressed: true},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-guardrails?current_user_input=walk+away", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, ok := resp["director"].(map[string]any)
	if !ok {
		t.Fatalf("director missing: %+v", resp)
	}
	required, _ := director["required_outcomes"].([]any)
	if !containsAnyStringValue(required, "Carry forward: Hook A") {
		t.Fatalf("required_outcomes = %+v, want Hook A carry-forward", required)
	}
	forbidden, _ := director["forbidden_moves"].([]any)
	if !containsAnyStringValue(forbidden, "Do not abruptly resolve: Risk A") {
		t.Fatalf("forbidden_moves = %+v, want Risk A guard", forbidden)
	}
	executionChecklist, _ := director["execution_checklist"].([]any)
	if len(executionChecklist) == 0 || len(executionChecklist) > 4 {
		t.Fatalf("execution_checklist len = %d, want 1..4: %+v", len(executionChecklist), executionChecklist)
	}
	worldGuardrails, _ := director["world_guardrails"].([]any)
	if len(worldGuardrails) == 0 || len(worldGuardrails) > 4 {
		t.Fatalf("world_guardrails len = %d, want 1..4: %+v", len(worldGuardrails), worldGuardrails)
	}
	if containsAnyStringSubstring(worldGuardrails, "secret") || containsAnyStringSubstring(worldGuardrails, "suppressed") {
		t.Fatalf("world_guardrails carried invalid/suppressed rule: %+v", worldGuardrails)
	}
	personaGuardrails, _ := director["persona_guardrails"].([]any)
	if !containsAnyStringSubstring(personaGuardrails, "Chloe") ||
		!containsAnyStringSubstring(personaGuardrails, "dry") ||
		!containsAnyStringSubstring(personaGuardrails, "guarded") {
		t.Fatalf("persona_guardrails = %+v, want Chloe speech/personality hints", personaGuardrails)
	}
	guidance, ok := resp["story_guidance"].(map[string]any)
	if !ok {
		t.Fatalf("story_guidance missing: %+v", resp)
	}
	conflictPolicy, _ := guidance["conflict_policy"].(map[string]any)
	if conflictPolicy["guidance_may_override_user_input"] != false {
		t.Fatalf("persona/world guidance can override user input: %+v", conflictPolicy)
	}
}

func containsAnyStringSubstring(items []any, needle string) bool {
	for _, item := range items {
		if strings.Contains(fmt.Sprint(item), needle) {
			return true
		}
	}
	return false
}

func TestSessionGuidanceSnapshotCachedActive(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Arc A", "narrative_goal": "goal A"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Continue"})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-gs-active",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-active/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "active" {
		t.Fatalf("state_status = %v, want active", resp["state_status"])
	}
	if resp["last_turn"] != float64(10) {
		t.Fatalf("last_turn = %v, want 10", resp["last_turn"])
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
	warnings, _ := resp["warnings"].([]any)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want empty", warnings)
	}
}

func TestSessionGuidanceSnapshotCachedEmpty(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": ""})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": ""})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-gs-empty",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "empty",
			LastTurn:      -1,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-empty/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "empty" {
		t.Fatalf("state_status = %v, want empty", resp["state_status"])
	}
	if resp["last_turn"] != float64(-1) {
		t.Fatalf("last_turn = %v, want -1", resp["last_turn"])
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, w := range warnings {
		if strings.Contains(fmt.Sprint(w), "rebuild will be triggered by next GET /narrative-control call") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rebuild warning in warnings: %v", warnings)
	}
}

func TestSessionGuidanceSnapshotNoStateDegrade(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-nostate/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "no_state" {
		t.Fatalf("state_status = %v, want no_state", resp["state_status"])
	}
	if resp["last_turn"] != float64(-1) {
		t.Fatalf("last_turn = %v, want -1", resp["last_turn"])
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok || len(plan) != 0 {
		t.Fatalf("story_plan = %v, want empty object", resp["story_plan"])
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, w := range warnings {
		if strings.Contains(fmt.Sprint(w), "No cached guidance plan state found") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected degrade warning in warnings: %v", warnings)
	}
}

func TestSessionExportIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Arc A"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Continue"})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-export-gs", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-export-gs",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      7,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-export-gs/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot is not an object: %v", resp["guidance_snapshot"])
	}
	if gs["state_status"] != "active" {
		t.Fatalf("guidance_snapshot.state_status = %v, want active", gs["state_status"])
	}
	if gs["last_turn"] != float64(7) {
		t.Fatalf("guidance_snapshot.last_turn = %v, want 7", gs["last_turn"])
	}
	plan, ok := gs["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("guidance_snapshot.story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
}

// TestSeq13P222ExportPackageLogicalEventDecisionMarkers verifies P222:
// Export endpoint returns a logical event package with portability contract,
// manual-first deferred copy detection, lineage surfaces, artifact exclusions,
// Chroma-compatible retrieval lane defaults, and selective rebuild handoff.
func TestSeq13P222ExportPackageLogicalEventDecisionMarkers(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-p222", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-p222/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["export_version"] != "1.1" {
		t.Fatalf("export_version = %v, want 1.1", resp["export_version"])
	}
	contract, ok := resp["portability_contract"].(map[string]any)
	if !ok {
		t.Fatalf("portability_contract missing")
	}
	if contract["package_mode"] != "logical_event_package" {
		t.Fatalf("package_mode = %v, want logical_event_package", contract["package_mode"])
	}
	if contract["db_snapshot_policy"] != "admin_full_profile_explicit_only" {
		t.Fatalf("db_snapshot_policy = %v, want admin_full_profile_explicit_only", contract["db_snapshot_policy"])
	}
	if contract["db_snapshot_default_included"] != false {
		t.Fatalf("db_snapshot_default_included = %v, want false", contract["db_snapshot_default_included"])
	}
	if contract["runtime_artifact_policy"] != "exclude_cache_temp_logs_downloads_git_runtime_proofs" {
		t.Fatalf("runtime_artifact_policy = %v", contract["runtime_artifact_policy"])
	}
	if contract["vector_artifact_policy"] != "exclude_from_default_package_rebuildable_retrieval_artifact" {
		t.Fatalf("vector_artifact_policy = %v", contract["vector_artifact_policy"])
	}
	if contract["canonical_truth_authority"] != "mariadb_store" {
		t.Fatalf("canonical_truth_authority = %v, want mariadb_store", contract["canonical_truth_authority"])
	}
	if contract["vector_retrieval_lane"] != "chromadb_only" {
		t.Fatalf("vector_retrieval_lane = %v, want chromadb_only", contract["vector_retrieval_lane"])
	}
	if contract["vector_engine_policy"] != "chromadb_only" {
		t.Fatalf("vector_engine_policy = %v, want chromadb_only", contract["vector_engine_policy"])
	}
	if _, ok := contract["milvus_lite_policy"]; ok {
		t.Fatalf("milvus_lite_policy should not be exposed in 2.0 runtime contract: %+v", contract)
	}
	if contract["manual_first"] != true {
		t.Fatalf("manual_first = %v, want true", contract["manual_first"])
	}
	if contract["auto_copy_detection"] != "deferred" {
		t.Fatalf("auto_copy_detection = %v, want deferred", contract["auto_copy_detection"])
	}
	if contract["session_origin"] != "sess-p222" {
		t.Fatalf("session_origin = %v, want sess-p222", contract["session_origin"])
	}
	portable, ok := contract["portable_units"].([]any)
	if !ok || len(portable) == 0 {
		t.Fatalf("portable_units missing or empty")
	}
	lineage, ok := contract["lineage_surfaces"].([]any)
	if !ok || len(lineage) == 0 {
		t.Fatalf("lineage_surfaces missing or empty")
	}
	handoff, ok := contract["rebuild_handoff"].(map[string]any)
	if !ok {
		t.Fatalf("rebuild_handoff missing")
	}
	if handoff["dirty_event_type"] != "backfill_import" {
		t.Fatalf("rebuild_handoff.dirty_event_type = %v, want backfill_import", handoff["dirty_event_type"])
	}
	if handoff["rebuild_mode"] != "selective" {
		t.Fatalf("rebuild_handoff.rebuild_mode = %v, want selective", handoff["rebuild_mode"])
	}
	if handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("rebuild_handoff.start_point = %v, want next_prepare_turn_fetch", handoff["start_point"])
	}
}

func TestSessionStateIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "State Arc"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Keep state aligned"})
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-state-gs", StateType: "scene_state", Content: "{}", TurnIndex: 3},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-state-gs",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      3,
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/session-state/sess-state-gs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot missing: %#v", resp)
	}
	if gs["state_status"] != "active" || gs["last_turn"] != float64(3) {
		t.Fatalf("guidance_snapshot mismatch: %#v", gs)
	}
	meta := resp["section_meta"].(map[string]any)
	gsMeta := meta["guidance_snapshot"].(map[string]any)
	if gsMeta["ready"] != true {
		t.Fatalf("guidance_snapshot meta not ready: %#v", gsMeta)
	}
}
