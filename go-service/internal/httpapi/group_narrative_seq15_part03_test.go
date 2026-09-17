package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestSeq15P147RelationDescriptorLaneSmoke verifies P147:
// buildCharacterRelationshipLane produces a well-formed relationship descriptor
// surface with required fields and proper typing.
func TestSeq15P147RelationDescriptorLaneSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:                147,
		ChatSessionID:     "sess-seq15-p147",
		CharacterName:     "Elara",
		RelationshipsJSON: `{"{{user}}":{"trust":"high","closeness":"warm","summary":"trusted companion"},"Kael":{"trust":"medium","tension":"rising","summary":"uneasy ally"}}`,
		TurnIndex:         31,
	}

	lane := buildCharacterRelationshipLane(item)

	if lane == nil {
		t.Fatal("relationship lane must not be nil")
	}

	if lane["surface_type"] != "relationship_lane" {
		t.Fatalf("relationship lane surface_type = %v, want relationship_lane", lane["surface_type"])
	}

	if lane["summary_text"] == nil {
		t.Fatal("relationship lane missing summary_text")
	}

	items := mapAnySlice(lane["items"])
	if len(items) == 0 {
		t.Fatal("relationship lane items should not be empty for populated input")
	}

	firstItem := items[0]
	if firstItem["target"] != "{{user}}" {
		t.Fatalf("first item target = %v, want {{user}}", firstItem["target"])
	}
}

// TestSeq15P148WeakInputSteeringFailModeSmoke verifies P148:
// when story guidance surface receives weak/empty/minimal input,
// it must degrade gracefully and not crash or fabricate unsupported fields.
func TestSeq15P148WeakInputSteeringFailModeSmoke(t *testing.T) {

	surface1 := buildStoryGuidanceSurface(map[string]any{}, map[string]any{})
	if surface1 == nil {
		t.Fatal("buildStoryGuidanceSurface with empty inputs must not return nil")
	}
	if _, ok := surface1["turn_directives"]; !ok {
		t.Fatal("surface with empty inputs missing turn_directives")
	}
	td1 := asMap(surface1["turn_directives"])
	if td1["tempo_band"] != "steady" {
		t.Fatalf("empty-input tempo_band = %v, want steady", td1["tempo_band"])
	}
	fm1 := asMap(td1["fail_mode"])
	if fm1["mode"] != "conservative_continuation" {
		t.Fatalf("empty-input fail_mode.mode = %v, want conservative_continuation", fm1["mode"])
	}

	surface2 := buildStoryGuidanceSurface(nil, nil)
	if surface2 == nil {
		t.Fatal("buildStoryGuidanceSurface with nil inputs must not return nil")
	}
	if _, ok := surface2["turn_directives"]; !ok {
		t.Fatal("surface with nil inputs missing turn_directives")
	}
	td2 := asMap(surface2["turn_directives"])
	if td2["tempo_band"] != "steady" {
		t.Fatalf("nil-input tempo_band = %v, want steady", td2["tempo_band"])
	}

	minimalPlan := map[string]any{"narrative_goal": "hold current scene"}
	minimalDirector := map[string]any{"pressure_level": "steady"}
	surface3 := buildStoryGuidanceSurface(minimalPlan, minimalDirector)
	if surface3 == nil {
		t.Fatal("buildStoryGuidanceSurface with minimal inputs must not return nil")
	}
	if _, ok := surface3["turn_directives"]; !ok {
		t.Fatal("surface with minimal inputs missing turn_directives")
	}
	td3 := asMap(surface3["turn_directives"])
	if td3["tempo_band"] != "steady" {
		t.Fatalf("minimal-input tempo_band = %v, want steady", td3["tempo_band"])
	}
	fm3 := asMap(td3["fail_mode"])
	if fm3["allow_scene_jump"] != false {
		t.Fatal("minimal-input fail_mode.allow_scene_jump must be false")
	}
	if fm3["allow_forced_resolution"] != false {
		t.Fatal("minimal-input fail_mode.allow_forced_resolution must be false")
	}
	if fm3["respect_explicit_user_correction"] != true {
		t.Fatal("minimal-input fail_mode.respect_explicit_user_correction must be true")
	}

	precedence := asMap(surface3["precedence"])
	if precedence["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority = %v, want subordinate", precedence["guidance_authority"])
	}
	for _, forbidden := range []string{"truth_authority", "canonical_write", "current_fact_write"} {
		if _, ok := surface3[forbidden]; ok {
			t.Fatalf("surface contains forbidden authority key %q", forbidden)
		}
	}
}

// TestSeq15P152StableCurrentSnapshotFieldBoundary verifies P152:
// current snapshot data belongs to dynamic/current surfaces, while the stable
// character sheet only points volatile readers toward the dynamic lane.
func TestSeq15P152StableCurrentSnapshotFieldBoundary(t *testing.T) {
	item := store.CharacterState{
		ID:              152,
		ChatSessionID:   "sess-seq15-p152",
		CharacterName:   "Riven",
		AppearanceJSON:  `{"height":"tall","outfit":"rain-dark coat","expression":"tired","internal_feeling":"hiding fear"}`,
		PersonalityJSON: `{"core":"protective","flaw":"secretive"}`,
		SpeechStyleJSON: `{"tone":"dry","pace":"quick"}`,
		StatusJSON:      `{"location":"north bridge","mood":"wary","goal":"stall the envoy"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","closeness":"guarded","summary":"trusted but tested ally"}
		}`,
		TurnIndex: 18,
	}

	snapshot := characterStaleSnapshot(item, nil, 18, "", map[string]struct{}{})
	relationshipLane := buildCharacterRelationshipLane(item)
	digest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, nil, nil)
	stable := buildStableCharacterSheet(item, snapshot)

	currentSnapshot := asMap(digest["current_snapshot"])
	if len(currentSnapshot) == 0 {
		t.Fatal("dynamic digest missing current_snapshot")
	}
	if _, ok := currentSnapshot["status"].(map[string]any); !ok {
		t.Fatalf("current_snapshot.status should be map, got %T", currentSnapshot["status"])
	}
	appearanceSnapshot := asMap(currentSnapshot["appearance"])
	for _, want := range []string{"outfit", "expression"} {
		if _, ok := appearanceSnapshot[want]; !ok {
			t.Fatalf("current_snapshot.appearance missing transient key %q: %+v", want, appearanceSnapshot)
		}
	}
	if _, ok := currentSnapshot["relationship_focus"].(map[string]any); !ok {
		t.Fatalf("current_snapshot.relationship_focus should be map, got %T", currentSnapshot["relationship_focus"])
	}

	for _, forbidden := range []string{"current_snapshot", "current_status", "relationship_focus", "relationship_lane", "latest_interaction_anchor", "stale_guard"} {
		if _, ok := stable[forbidden]; ok {
			t.Fatalf("stable sheet must not own current snapshot field %q", forbidden)
		}
	}
	stableRedirects := asStringSlice(asMap(stable["sparse_policy"])["dynamic_redirects"])
	mustContainAll(t, stableRedirects, []string{"current_status", "relationship_lane", "latest_interaction_anchor", "appearance_snapshot"})
}

// TestSeq15P153DigestBudgetPerFieldCharacterBudget verifies P153:
// compactness is expressed as explicit per-field digest budget caps and used
// counts, not as an opaque whole-character budget.
func TestSeq15P153DigestBudgetPerFieldCharacterBudget(t *testing.T) {
	item := store.CharacterState{
		ID:            153,
		ChatSessionID: "sess-seq15-p153",
		CharacterName: "Maia",
		StatusJSON:    `{"location":"archive vault","mood":"alert"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","summary":"field partner"},
			"Aren":{"trust":"medium","summary":"old friend"},
			"Bryn":{"tension":"rising","summary":"rival"},
			"Cato":{"closeness":"warm","summary":"student"},
			"Dena":{"distance":"far","summary":"distant sibling"},
			"Eris":{"stance":"hostile","summary":"enemy agent"}
		}`,
		TurnIndex: 40,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Maia", TurnIndex: 12, EventType: "status_change", DetailsJSON: `{"summary":"opened the vault"}`},
		{CharacterName: "Maia", TurnIndex: 18, EventType: "relationship_shift", DetailsJSON: `{"summary":"trusted the player with the cipher"}`},
		{CharacterName: "Maia", TurnIndex: 26, EventType: "personality_change", DetailsJSON: `{"summary":"became less cautious"}`},
		{CharacterName: "Maia", TurnIndex: 34, EventType: "appearance_change", DetailsJSON: `{"summary":"changed disguise"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 40, "", map[string]struct{}{})
	relationshipLane := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	digest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latest, events)

	if _, ok := digest["character_budget"]; ok {
		t.Fatal("dynamic digest should not expose opaque character_budget")
	}
	budget := asMap(digest["digest_budget"])
	expected := map[string]any{
		"policy":                     "priority_capped",
		"relationship_lane_cap":      4,
		"milestone_cap":              3,
		"milestone_read_window":      8,
		"milestone_selection_policy": "latest_plus_priority_events",
	}
	for key, want := range expected {
		if got := budget[key]; got != want {
			t.Fatalf("digest_budget[%q] = %v, want %v", key, got, want)
		}
	}
	if used, ok := budget["relationship_lane_used"].(int); !ok || used > 4 {
		t.Fatalf("relationship_lane_used = %v, want int <= 4", budget["relationship_lane_used"])
	}
	if used, ok := budget["milestones_used"].(int); !ok || used > 3 {
		t.Fatalf("milestones_used = %v, want int <= 3", budget["milestones_used"])
	}
	if len(mapAnySlice(digest["relationship_descriptor_lane"])) > 4 {
		t.Fatal("relationship_descriptor_lane exceeds digest relationship cap")
	}
	if len(mapAnySlice(digest["milestone_ledger"])) > 3 {
		t.Fatal("milestone_ledger exceeds digest milestone cap")
	}
}

// TestSeq15P154RelationDescriptorBandGranularity verifies P154:
// descriptor bands stay readable and coarse-grained, prioritizing trust,
// closeness, and tension while capping output size.
func TestSeq15P154RelationDescriptorBandGranularity(t *testing.T) {
	item := store.CharacterState{
		ID:            154,
		ChatSessionID: "sess-seq15-p154",
		CharacterName: "Selene",
		RelationshipsJSON: `{
			"{{user}}":{
				"trust":"high",
				"closeness":"warm",
				"tension":"low",
				"bond":"oath-bound",
				"distance":"near",
				"stance":"protective",
				"summary":"trusted protagonist anchor"
			},
			"Orin":{"trust":"low","tension":"high","summary":"dangerous rival"}
		}`,
		TurnIndex: 22,
	}
	lane := buildCharacterRelationshipLane(item)
	protagonist := asMap(lane["protagonist_relation"])
	bands := mapAnyOrEmptyStringSlice(protagonist, "descriptor_bands")

	if len(bands) != 3 {
		t.Fatalf("descriptor_bands length = %d, want cap of 3: %v", len(bands), bands)
	}
	want := []string{"trust: high", "closeness: warm", "tension: low"}
	for i, expected := range want {
		if bands[i] != expected {
			t.Fatalf("descriptor_bands[%d] = %q, want %q; all=%v", i, bands[i], expected, bands)
		}
	}
	for _, forbidden := range []string{"bond: oath-bound", "distance: near", "stance: protective"} {
		for _, band := range bands {
			if band == forbidden {
				t.Fatalf("descriptor bands should not include overflow band %q", forbidden)
			}
		}
	}
	if summary := lane["descriptor_summary"]; !strings.Contains(asString(summary), "trust: high") {
		t.Fatalf("descriptor_summary = %v, want primary readable band summary", summary)
	}
}

// TestSeq15P155MilestoneLedgerNarrativeSummaryRoleSplit verifies P155:
// milestone ledger remains a compact event ledger, while the latest narrative
// summary is represented separately through latest_interaction/recent_change.
func TestSeq15P155MilestoneLedgerNarrativeSummaryRoleSplit(t *testing.T) {
	item := store.CharacterState{
		ID:            155,
		ChatSessionID: "sess-seq15-p155",
		CharacterName: "Ilya",
		StatusJSON:    `{"location":"harbor","goal":"protect witness"}`,
		TurnIndex:     60,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Ilya", TurnIndex: 44, EventType: "relationship_shift", DetailsJSON: `{"summary":"swore to protect the witness","detail":"lasting relationship promise"}`},
		{CharacterName: "Ilya", TurnIndex: 48, EventType: "status_change", DetailsJSON: `{"summary":"moved to the harbor"}`},
		{CharacterName: "Ilya", TurnIndex: 52, EventType: "personality_change", DetailsJSON: `{"summary":"became willing to trust help"}`},
		{CharacterName: "Ilya", TurnIndex: 58, EventType: "dialogue", DetailsJSON: `{"summary":"warned that the ship was watched"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 60, "", map[string]struct{}{})
	relationshipLane := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	digest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latest, events)

	ledger := mapAnySlice(digest["milestone_ledger"])
	if len(ledger) == 0 || len(ledger) > 3 {
		t.Fatalf("milestone_ledger length = %d, want 1..3", len(ledger))
	}
	for i, entry := range ledger {
		for _, key := range []string{"event_type", "turn_index", "summary_text", "details"} {
			if _, ok := entry[key]; !ok {
				t.Fatalf("milestone_ledger[%d] missing %q: %+v", i, key, entry)
			}
		}
		for _, forbidden := range []string{"narrative_summary", "recent_change_summary", "latest_interaction_anchor"} {
			if _, ok := entry[forbidden]; ok {
				t.Fatalf("milestone_ledger[%d] must not own summary surface %q", i, forbidden)
			}
		}
	}
	if got := digest["recent_change_summary"]; got != "warned that the ship was watched" {
		t.Fatalf("recent_change_summary = %v, want latest interaction summary", got)
	}
	latestMap := asMap(digest["latest_interaction_anchor"])
	if latestMap["event_type"] != "dialogue" {
		t.Fatalf("latest_interaction_anchor.event_type = %v, want dialogue", latestMap["event_type"])
	}
}

// TestSeq15P156WeakInputFocusPriorityTensionFirstCharacterSecond verifies P156:
// weak-input focus is tension/scene-continuity first and character spotlight
// second; character focus alone must not escalate into forced steering.
func TestSeq15P156WeakInputFocusPriorityTensionFirstCharacterSecond(t *testing.T) {
	tensionFirst := buildStoryGuidanceSurface(
		map[string]any{
			"active_tensions":  []string{"the treaty may collapse", "the witness is missing"},
			"focus_characters": []string{"Ilya", "Selene"},
			"next_beats":       []string{"keep pressure on the harbor standoff"},
		},
		map[string]any{"pressure_level": "steady"},
	)
	frame := asMap(tensionFirst["story_frame"])
	liveTensions := mapAnyOrEmptyStringSlice(frame, "live_tensions")
	spotlight := mapAnyOrEmptyStringSlice(frame, "spotlight_characters")
	mustContain(t, liveTensions, "the treaty may collapse")
	mustContain(t, spotlight, "Ilya")
	if frame["status"] == "empty" {
		t.Fatalf("tension-first story_frame.status = %v, want non-empty", frame["status"])
	}
	td := asMap(tensionFirst["turn_directives"])
	if td["tempo_band"] != "steady" {
		t.Fatalf("tempo_band = %v, want steady", td["tempo_band"])
	}
	fm := asMap(td["fail_mode"])
	if fm["mode"] != "scene_continuation_without_scene_jump" {
		t.Fatalf("tension-first fail_mode.mode = %v, want scene_continuation_without_scene_jump", fm["mode"])
	}

	characterOnly := buildStoryGuidanceSurface(
		map[string]any{"focus_characters": []string{"Ilya"}},
		map[string]any{"pressure_level": "steady"},
	)
	characterOnlyTD := asMap(characterOnly["turn_directives"])
	characterOnlyFM := asMap(characterOnlyTD["fail_mode"])
	if characterOnlyFM["mode"] != "conservative_continuation" {
		t.Fatalf("character-only weak input fail_mode.mode = %v, want conservative_continuation", characterOnlyFM["mode"])
	}
	if characterOnlyFM["allow_scene_jump"] != false || characterOnlyFM["allow_forced_resolution"] != false {
		t.Fatalf("character-only weak input must not force steering: %+v", characterOnlyFM)
	}
	if characterOnlyTD["tempo_band"] != "steady" {
		t.Fatalf("character-only tempo_band = %v, want steady", characterOnlyTD["tempo_band"])
	}
}

func TestSeq1415AuditCharactersRouteExposesClosedCharacterSurfaces(t *testing.T) {
	fake := &narrativeFakeStore{
		characterStates: []store.CharacterState{
			{
				ID:              141501,
				ChatSessionID:   "sess-seq1415-audit-char",
				CharacterName:   "AuditRiven",
				AppearanceJSON:  `{"height":"tall","outfit":"travel coat","internal_feeling":"hiding fear"}`,
				PersonalityJSON: `{"core":"protective","flaw":"secretive"}`,
				SpeechStyleJSON: `{"tone":"dry","pace":"quick"}`,
				StatusJSON:      `{"location":"north bridge","mood":"wary","goal":"stall the envoy"}`,
				RelationshipsJSON: `{
					"{{user}}":{"trust":"high","closeness":"guarded","tension":"low","summary":"trusted but tested ally"},
					"Orin":{"trust":"low","tension":"high","summary":"dangerous rival"}
				}`,
				TurnIndex: 18,
			},
		},
		characterEvents: []store.CharacterEvent{
			{
				ID:            141502,
				ChatSessionID: "sess-seq1415-audit-char",
				CharacterName: "AuditRiven",
				TurnIndex:     19,
				EventType:     "dialogue",
				DetailsJSON:   `{"summary":"warned the player not to trust Orin"}`,
			},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-seq1415-audit-char", TurnIndex: 20, Role: "assistant", Content: "AuditRiven keeps watch on the bridge."},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/characters/sess-seq1415-audit-char", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /characters status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode characters response: %v", err)
	}
	items := mapAnySlice(resp["characters"])
	if len(items) != 1 {
		t.Fatalf("characters length = %d, want 1; resp=%+v", len(items), resp)
	}
	item := items[0]

	stable := asMap(item["stable_character_sheet"])
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("route stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	for _, forbidden := range []string{"current_status", "current_snapshot", "relationship_lane", "latest_interaction_anchor"} {
		if _, ok := stable[forbidden]; ok {
			t.Fatalf("route stable sheet leaked current/dynamic key %q", forbidden)
		}
	}
	stableRedirects := asStringSlice(asMap(stable["sparse_policy"])["dynamic_redirects"])
	mustContainAll(t, stableRedirects, []string{"current_status", "relationship_lane", "latest_interaction_anchor", "appearance_snapshot"})

	dynamic := asMap(item["dynamic_continuity_digest"])
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("route dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}
	if _, ok := dynamic["current_snapshot"].(map[string]any); !ok {
		t.Fatalf("route dynamic digest missing current_snapshot map, got %T", dynamic["current_snapshot"])
	}
	budget := asMap(dynamic["digest_budget"])
	if budget["relationship_lane_cap"] != float64(4) && budget["relationship_lane_cap"] != 4 {
		t.Fatalf("route digest relationship_lane_cap = %v, want 4", budget["relationship_lane_cap"])
	}
	if budget["milestone_cap"] != float64(3) && budget["milestone_cap"] != 3 {
		t.Fatalf("route digest milestone_cap = %v, want 3", budget["milestone_cap"])
	}

	relationshipLane := asMap(item["relationship_lane"])
	if relationshipLane["surface_type"] != "relationship_lane" {
		t.Fatalf("route relationship_lane surface_type = %v, want relationship_lane", relationshipLane["surface_type"])
	}
	primaryBands := asStringSlice(relationshipLane["primary_descriptor_bands"])
	mustContainAll(t, primaryBands, []string{"trust: high", "closeness: guarded", "tension: low"})

	anchor := asMap(item["latest_interaction_anchor"])
	if anchor["surface_type"] != "latest_interaction_anchor" {
		t.Fatalf("route latest_interaction_anchor surface_type = %v, want latest_interaction_anchor", anchor["surface_type"])
	}
	if anchor["summary_text"] != "warned the player not to trust Orin" {
		t.Fatalf("route latest_interaction_anchor summary = %v", anchor["summary_text"])
	}
}

func TestSeq1415AuditNarrativeControlRoutePreservesCachedGuidanceArrays(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Harbor Treaty",
		"narrative_goal":     "hold the fragile negotiation",
		"active_tensions":    []any{"the treaty may collapse", "the witness is missing"},
		"next_beats":         []any{"keep pressure on the harbor standoff"},
		"continuity_anchors": []any{"protect the witness"},
		"focus_characters":   []any{"Ilya", "Selene"},
		"last_plan_turn":     float64(30),
		"state_status":       "ready",
	}
	cachedDirector := map[string]any{
		"scene_mandate":       "Keep the harbor negotiation in frame",
		"required_outcomes":   []any{"preserve the witness thread"},
		"forbidden_moves":     []any{"resolve the treaty offscreen"},
		"pressure_level":      "steady",
		"execution_checklist": []any{"show the witness risk"},
		"persona_guardrails":  []any{"do not flatten Ilya into a generic guard"},
		"world_guardrails":    []any{"harbor curfew remains active"},
		"focus_characters":    []any{"Ilya", "Selene"},
		"last_turn":           float64(30),
		"state_status":        "ready",
	}
	planJSON, _ := json.Marshal(cachedPlan)
	directorJSON, _ := json.Marshal(cachedDirector)
	warningsJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-seq1415-audit-guidance",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(directorJSON),
			StateStatus:   "ready",
			LastTurn:      30,
			WarningsJSON:  string(warningsJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-seq1415-audit-guidance", Name: "Harbor Treaty", LastTurn: 30, FirstTurn: 10, CurrentContext: "harbor negotiation"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-seq1415-audit-guidance", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /narrative-control status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode narrative-control response: %v", err)
	}
	guidance := asMap(resp["story_guidance"])
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("story_guidance surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}
	frame := asMap(guidance["story_frame"])
	mustContainAll(t, asStringSlice(frame["live_tensions"]), []string{"the treaty may collapse", "the witness is missing"})
	mustContainAll(t, asStringSlice(frame["spotlight_characters"]), []string{"Ilya", "Selene"})
	mustContain(t, asStringSlice(frame["beat_queue"]), "keep pressure on the harbor standoff")

	directives := asMap(guidance["turn_directives"])
	if directives["tempo_band"] != "steady" {
		t.Fatalf("tempo_band = %v, want steady", directives["tempo_band"])
	}
	mustContain(t, asStringSlice(directives["carry_targets"]), "preserve the witness thread")
	mustContain(t, asStringSlice(directives["blocked_routes"]), "resolve the treaty offscreen")
	mustContain(t, asStringSlice(directives["turn_checklist"]), "show the witness risk")
	mustContain(t, asStringSlice(directives["voice_guardrails"]), "do not flatten Ilya into a generic guard")
	mustContain(t, asStringSlice(directives["setting_guardrails"]), "harbor curfew remains active")
	failMode := asMap(directives["fail_mode"])
	if failMode["allow_scene_jump"] != false || failMode["allow_forced_resolution"] != false || failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("route fail_mode must remain conservative/subordinate: %+v", failMode)
	}
	precedence := asMap(guidance["precedence"])
	if precedence["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority = %v, want subordinate", precedence["guidance_authority"])
	}
}
