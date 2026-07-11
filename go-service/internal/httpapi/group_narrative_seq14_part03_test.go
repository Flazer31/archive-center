package httpapi

import (
	"fmt"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestSeq14P95RelationLaneDescriptorStructuredSlot verifies that the
// relationship_lane surface exposes descriptor_bands as a structured slot
// on each item, and that descriptor_summary and primary_descriptor_bands
// are present at the surface root.
func TestSeq14P95RelationLaneDescriptorStructuredSlot(t *testing.T) {
	item := store.CharacterState{
		ID:                95,
		ChatSessionID:     "sess-seq14-p95",
		CharacterName:     "Nyx",
		AppearanceJSON:    `{"core":"short stature"}`,
		PersonalityJSON:   `{"core":"cunning"}`,
		RelationshipsJSON: `{"user":{"summary":"wary respect","trust":"high","closeness":"moderate","tension":"low"}}`,
		TurnIndex:         4,
	}
	relLane := buildCharacterRelationshipLane(item)

	if relLane["surface_version"] != "rl14a.v1" {
		t.Fatalf("P95: surface_version = %v, want rl14a.v1", relLane["surface_version"])
	}
	if relLane["surface_type"] != "relationship_lane" {
		t.Fatalf("P95: surface_type = %v, want relationship_lane", relLane["surface_type"])
	}

	items := seq14RelationItems(t, relLane["items"])
	if len(items) == 0 {
		t.Fatal("P95: expected at least 1 item in relationship_lane")
	}
	foundBands := false
	for _, entry := range items {
		bands := seq14StringSlice(t, entry["descriptor_bands"], "descriptor_bands")
		if len(bands) > 0 {
			foundBands = true
			break
		}
	}
	if !foundBands {
		t.Fatal("P95: no item in relationship_lane has non-empty descriptor_bands")
	}

	seq14AssertStringSlice(t, relLane, "primary_descriptor_bands", 0)

	if _, ok := relLane["descriptor_summary"]; !ok {
		t.Fatal("P95: relationship_lane missing descriptor_summary")
	}

	if relLane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("P95: display_mode = %v", relLane["display_mode"])
	}

	for i, entry := range items {
		for _, key := range []string{"target", "summary_text", "state_snapshot", "descriptor_bands", "display_priority"} {
			if _, ok := entry[key]; !ok {
				t.Fatalf("P95: item[%d] missing key %q", i, key)
			}
		}
	}
}

// TestSeq14P96WeakInputBaselineFocusPriority verifies that the stale_guard
// correctly handles weak-input baselines: a character with anchor data should
// have allow_weak_input_carry_forward=true, while a character with no anchors
// and a large freshness gap should be stale.
func TestSeq14P96WeakInputBaselineFocusPriority(t *testing.T) {

	anchored := store.CharacterState{
		ID:              96,
		ChatSessionID:   "sess-seq14-p96a",
		CharacterName:   "Serin",
		AppearanceJSON:  `{"core":"bright eyes"}`,
		PersonalityJSON: `{"core":"loyal"}`,
		StatusJSON:      `{"location":"camp"}`,
		TurnIndex:       7,
	}
	snapshotA := characterStaleSnapshot(anchored, nil, 8, "", map[string]struct{}{})
	guardA := seq14SubMap(t, snapshotA, "stale_guard")
	if guardA["active"] == true {
		t.Fatal("P96-A: anchored character stale_guard.active must not be true")
	}
	if carryForward, ok := guardA["allow_weak_input_carry_forward"].(bool); !ok || !carryForward {
		t.Fatalf("P96-A: anchored character allow_weak_input_carry_forward = %v, want true", guardA["allow_weak_input_carry_forward"])
	}
	if snapshotA["is_stale"] == true {
		t.Fatal("P96-A: anchored character must not be stale")
	}

	transient := store.CharacterState{
		ID:            96,
		ChatSessionID: "sess-seq14-p96b",
		CharacterName: "unknown woman",
		TurnIndex:     2,
	}
	snapshotB := characterStaleSnapshot(transient, nil, 10, "", map[string]struct{}{})
	if snapshotB["is_stale"] != true {
		t.Fatal("P96-B: transient descriptor with no anchors and large gap must be stale")
	}
	guardB := seq14SubMap(t, snapshotB, "stale_guard")
	if carryForward, ok := guardB["allow_weak_input_carry_forward"].(bool); !ok || carryForward {
		t.Fatalf("P96-B: transient stale_guard allow_weak_input_carry_forward = %v, want false", guardB["allow_weak_input_carry_forward"])
	}

	if snapshotA["admission_class"] != "major_recurring" {
		t.Fatalf("P96-C: anchored character admission_class = %v, want major_recurring", snapshotA["admission_class"])
	}
	if snapshotB["admission_class"] != "transient_descriptor" {
		t.Fatalf("P96-C: transient descriptor admission_class = %v, want transient_descriptor", snapshotB["admission_class"])
	}
}

// TestSeq14P97Step14PromptExplorerAuditSurfaceMarkers verifies that all
// surface_version and surface_type markers are present and correct across
// every Step-14 surface in a full character response item plus the
// story_guidance_surface, so that a prompt explorer can locate them by
// their canonical markers.
func TestSeq14P97Step14PromptExplorerAuditSurfaceMarkers(t *testing.T) {
	item := store.CharacterState{
		ID:                97,
		ChatSessionID:     "sess-seq14-p97",
		CharacterName:     "Vera",
		AppearanceJSON:    `{"core":"scar across cheek"}`,
		PersonalityJSON:   `{"core":"stoic"}`,
		StatusJSON:        `{"location":"outpost","emotion":"vigilant"}`,
		RelationshipsJSON: `{"user":{"summary":"quiet bond","trust":0.6}}`,
		SpeechStyleJSON:   `{"tone":"terse"}`,
		TurnIndex:         8,
	}
	events := []store.CharacterEvent{
		{ID: 300, ChatSessionID: "sess-seq14-p97", CharacterName: "Vera", EventType: "personality_change", TurnIndex: 7, DetailsJSON: `{"interaction":"shared watch duty"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 8, "", map[string]struct{}{})
	resp := characterResponseItem(item, snapshot, &events[0], events)

	stable, ok := resp["stable_character_sheet"].(map[string]any)
	if !ok {
		t.Fatal("P97: response missing stable_character_sheet")
	}
	if stable["surface_version"] != "cc14a.v1" {
		t.Fatalf("P97: stable surface_version = %v, want cc14a.v1", stable["surface_version"])
	}
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("P97: stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}

	dynamic, ok := resp["dynamic_continuity_digest"].(map[string]any)
	if !ok {
		t.Fatal("P97: response missing dynamic_continuity_digest")
	}
	if dynamic["surface_version"] != "cc14b.v1" {
		t.Fatalf("P97: dynamic surface_version = %v, want cc14b.v1", dynamic["surface_version"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("P97: dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}

	relLane, ok := resp["relationship_lane"].(map[string]any)
	if !ok {
		t.Fatal("P97: response missing relationship_lane map")
	}
	if relLane["surface_version"] != "rl14a.v1" {
		t.Fatalf("P97: relationship_lane surface_version = %v, want rl14a.v1", relLane["surface_version"])
	}
	if relLane["surface_type"] != "relationship_lane" {
		t.Fatalf("P97: relationship_lane surface_type = %v, want relationship_lane", relLane["surface_type"])
	}

	anchorMap, ok := resp["latest_interaction_anchor"].(map[string]any)
	if !ok {
		t.Fatal("P97: response missing latest_interaction_anchor map (event was provided)")
	}
	if anchorMap["surface_version"] != "rl14b.v1" {
		t.Fatalf("P97: latest_interaction_anchor surface_version = %v, want rl14b.v1", anchorMap["surface_version"])
	}
	if anchorMap["surface_type"] != "latest_interaction_anchor" {
		t.Fatalf("P97: latest_interaction_anchor surface_type = %v, want latest_interaction_anchor", anchorMap["surface_type"])
	}

	storyPlan := map[string]any{
		"current_arc":        "rising_arc",
		"narrative_goal":     "tension_build",
		"active_tensions":    []string{"mounting_dread"},
		"next_beats":         []string{"confrontation"},
		"continuity_anchors": []string{"oath"},
		"focus_characters":   []string{"Vera"},
	}
	director := map[string]any{
		"pressure_level":    "high",
		"required_outcomes": []string{},
		"forbidden_moves":   []string{},
	}
	guidance := buildStoryGuidanceSurface(storyPlan, director)
	if guidance["surface_version"] != "sg14a.v1" {
		t.Fatalf("P97: guidance surface_version = %v, want sg14a.v1", guidance["surface_version"])
	}
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("P97: guidance surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}

	for label, surface := range map[string]map[string]any{
		"stable":   stable,
		"dynamic":  dynamic,
		"rel_lane": relLane,
		"anchor":   anchorMap,
		"guidance": guidance,
	} {
		if _, ok := surface["status"]; !ok {
			t.Fatalf("P97: %s surface missing status field", label)
		}
	}

	auditVersions := map[string]string{
		"stable_character_sheet":    "cc14a.v1",
		"dynamic_continuity_digest": "cc14b.v1",
		"relationship_lane":         "rl14a.v1",
		"latest_interaction_anchor": "rl14b.v1",
		"story_guidance_surface":    "sg14a.v1",
	}
	actualVersions := map[string]string{
		"stable_character_sheet":    fmt.Sprintf("%v", stable["surface_version"]),
		"dynamic_continuity_digest": fmt.Sprintf("%v", dynamic["surface_version"]),
		"relationship_lane":         fmt.Sprintf("%v", relLane["surface_version"]),
		"latest_interaction_anchor": fmt.Sprintf("%v", anchorMap["surface_version"]),
		"story_guidance_surface":    fmt.Sprintf("%v", guidance["surface_version"]),
	}
	for name, want := range auditVersions {
		if actualVersions[name] != want {
			t.Fatalf("P97: audit %s version = %v, want %v", name, actualVersions[name], want)
		}
	}
}
