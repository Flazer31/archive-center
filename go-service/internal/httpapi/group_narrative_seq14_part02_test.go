package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSeq14P68StoryGuidancePrecedenceCleanup(t *testing.T) {
	guidance := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "act_1"},
		map[string]any{"pressure_level": "steady"},
	)

	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["guidance_may_override_user_input"] != false {
		t.Fatal("conflict_policy.guidance_may_override_user_input must be false")
	}
	if cp["explicit_user_correction_wins"] != true {
		t.Fatal("conflict_policy.explicit_user_correction_wins must be true")
	}
	if cp["current_user_input_wins"] != true {
		t.Fatal("conflict_policy.current_user_input_wins must be true")
	}
	if cp["on_conflict"] != "yield_to_current_user_input" {
		t.Fatalf("conflict_policy.on_conflict must be yield_to_current_user_input, got %v", cp["on_conflict"])
	}

	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("precedence.guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}

	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	expectedOrder := []string{
		"current_user_input",
		"explicit_user_correction",
		"hard_world_rule",
		"latest_direct_evidence",
		"canonical_truth_floor",
	}
	if len(hps) != len(expectedOrder) {
		t.Fatalf("higher_priority_sources length must be %d, got %d", len(expectedOrder), len(hps))
	}
	for i, want := range expectedOrder {
		if hps[i] != want {
			t.Fatalf("higher_priority_sources[%d] must be %q, got %q", i, want, hps[i])
		}
	}

	for _, src := range expectedOrder {
		if !seq14Contains(hps, src) {
			t.Fatalf("higher_priority_sources must include %q", src)
		}
	}

	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	if len(du) < 3 {
		t.Fatal("precedence.disallowed_usage must list at least 3 blocked override patterns")
	}
}

// TestSeq14P72WeakInputPlannerFocusSelection verifies that the story guidance
// surface exposes a conservative planner focus suitable for weak-input
// continuation: active tensions, focus characters, beat queue, and ending
// requirement are all present so that a weak/empty user input does not become
// a new truth authority.
func TestSeq14P72WeakInputPlannerFocusSelection(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "act_2",
		"narrative_goal":     "sustain betrayal tension",
		"active_tensions":    []string{"trust_fracture", "loyalty_test"},
		"next_beats":         []string{"quiet_probe", "indirect_confrontation"},
		"continuity_anchors": []string{"oath_from_ep3"},
		"focus_characters":   []string{"aria", "kael"},
	}
	director := map[string]any{
		"pressure_level":      "steady",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{"reveal_twist_early"},
		"execution_checklist": []string{"maintain_tone"},
		"world_guardrails":    []string{"no_magic_in_city"},
		"persona_guardrails":  []string{"aria_stays_calm"},
		"scene_mandate":       "",
	}

	guidance := buildStoryGuidanceSurface(storyPlan, director)
	sf := seq14SubMap(t, guidance, "story_frame")
	td := seq14SubMap(t, guidance, "turn_directives")

	seq14AssertStringSlice(t, sf, "live_tensions", 2)
	seq14AssertStringSlice(t, sf, "spotlight_characters", 2)
	seq14AssertStringSlice(t, sf, "beat_queue", 2)
	seq14AssertStringSlice(t, sf, "carry_threads", 1)

	ec := seq14SubMap(t, td, "execution_contract")
	if _, ok := ec["ending_requirement"].(string); !ok {
		t.Fatal("execution_contract.ending_requirement must be a string for weak-input focus")
	}
	if hs, ok := td["handoff_edge"].(string); !ok || hs == "" {
		t.Fatal("handoff_edge must be a non-empty continuation edge")
	}

	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatalf("weak-input planner focus requires allow_scene_jump=false, got %v", fm["allow_scene_jump"])
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("weak-input planner focus requires allow_forced_resolution=false, got %v", fm["allow_forced_resolution"])
	}

	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["current_user_input_wins"] != true {
		t.Fatal("conflict_policy.current_user_input_wins must be true")
	}
	if cp["guidance_may_override_user_input"] != false {
		t.Fatal("conflict_policy.guidance_may_override_user_input must be false")
	}
}

// TestSeq14P73WeakInputConservativeFailSafe verifies that the story guidance
// fail-safe is a conservative continuation: no forced scene jump, no forced
// resolution, no hard world rule bypass, and no canonical truth overwrite.
// The guidance surface must stay subordinate to truth and user authority.
func TestSeq14P73WeakInputConservativeFailSafe(t *testing.T) {
	guidance := buildStoryGuidanceSurface(
		map[string]any{
			"current_arc":     "act_1",
			"active_tensions": []string{"opening_tension"},
		},
		map[string]any{
			"pressure_level":     "steady",
			"world_guardrails":   []string{"no_time_travel"},
			"persona_guardrails": []string{"protagonist_no_kill"},
		},
	)
	td := seq14SubMap(t, guidance, "turn_directives")

	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatal("weak-input fail-safe must not allow scene jump")
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatal("weak-input fail-safe must not allow forced resolution")
	}
	if fm["respect_explicit_user_correction"] != true {
		t.Fatal("weak-input fail-safe must respect explicit user correction")
	}

	mode, _ := fm["mode"].(string)
	switch mode {
	case "conservative_continuation",
		"scene_continuation_without_scene_jump",
		"carry_forward_without_forcing_resolution",
		"pressure_continuation_without_resolution":

	default:
		t.Fatalf("fail_mode.mode must be a conservative continuation variant, got %q", mode)
	}

	prec := seq14SubMap(t, guidance, "precedence")
	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{"hard_world_rule_bypass", "canonical_truth_floor_overwrite"} {
		if !seq14Contains(du, want) {
			t.Fatalf("disallowed_usage must block %q to prevent weak-input authority leak, got %v", want, du)
		}
	}

	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}
}

// TestSeq14P74WeakInputExplicitCorrectionGuard verifies that explicit user
// correction always outranks weak-input steering in the guidance surface.
// conflict_policy, precedence, and fail_mode must all agree that guidance
// never overrides an explicit correction from the user.
func TestSeq14P74WeakInputExplicitCorrectionGuard(t *testing.T) {
	guidance := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "act_2", "active_tensions": []string{"rift"}},
		map[string]any{"pressure_level": "strong"},
	)

	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["explicit_user_correction_wins"] != true {
		t.Fatal("conflict_policy.explicit_user_correction_wins must be true")
	}
	if cp["guidance_may_override_user_input"] != false {
		t.Fatal("conflict_policy.guidance_may_override_user_input must be false")
	}
	if cp["on_conflict"] != "yield_to_current_user_input" {
		t.Fatalf("conflict_policy.on_conflict must be yield_to_current_user_input, got %v", cp["on_conflict"])
	}

	prec := seq14SubMap(t, guidance, "precedence")
	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	if !seq14Contains(hps, "explicit_user_correction") {
		t.Fatalf("higher_priority_sources must include explicit_user_correction, got %v", hps)
	}

	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	if !seq14Contains(du, "explicit_user_correction_override") {
		t.Fatalf("disallowed_usage must block explicit_user_correction_override, got %v", du)
	}

	td := seq14SubMap(t, guidance, "turn_directives")
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode.respect_explicit_user_correction must be true, got %v", fm["respect_explicit_user_correction"])
	}
}

// TestSeq14P78StableDynamicNoOverwriteReplay verifies that the
// dynamic_continuity_digest (current_status, relationship_lane,
// latest_interaction_anchor, milestone_ledger, recent_change_summary) never
// overwrites stable_character_sheet or durable_profile content. Both surfaces
// must remain independent read lanes in the character response item.
func TestSeq14P78StableDynamicNoOverwriteReplay(t *testing.T) {
	item := store.CharacterState{
		ID:                78,
		ChatSessionID:     "sess-seq14-p78",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"appearance_observable":"tall, black hair","appearance_non_observable":"quiet presence"}`,
		PersonalityJSON:   `{"core":"guarded but loyal","flaw":"slow to trust"}`,
		SpeechStyleJSON:   `{"tone":"clipped"}`,
		StatusJSON:        `{"location":"bridge","emotion":"calm","goal":"stand watch"}`,
		RelationshipsJSON: `{"user":{"summary":"trusted ally","trust":0.9},"Rowan":{"summary":"uneasy rivalry","tension":0.6}}`,
		TurnIndex:         25,
	}
	snapshot := characterStaleSnapshot(item, nil, 25, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)

	events := []store.CharacterEvent{
		{
			ID:            780,
			ChatSessionID: "sess-seq14-p78",
			CharacterName: "Mira",
			EventType:     "relationship_shift",
			TurnIndex:     24,
			DetailsJSON:   `{"summary":"Mira chose to trust the protagonist.","before":{"trust":0.3},"after":{"trust":0.9}}`,
		},
	}
	anchor := buildCharacterLatestInteractionAnchor(&events[0])

	stable := buildStableCharacterSheet(item, snapshot)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, anchor, events)
	resp := characterResponseItem(item, snapshot, &events[0], events)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("stable surface_type = %v", stable["surface_type"])
	}

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable missing durable axis %q", key)
		}
	}

	volatileKeys := []string{
		"current_status", "current_snapshot",
		"relationship_lane", "relationship_descriptor_lane",
		"latest_interaction_anchor", "milestone_ledger",
		"recent_change_summary", "digest_budget",
		"other_relations", "protagonist_relation",
		"relationship_summary_text", "relationship_display_mode",
	}
	for _, key := range volatileKeys {
		if _, ok := stable[key]; ok {
			t.Fatalf("REPLAY: stable sheet must NOT contain volatile key %q", key)
		}
	}

	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("dynamic surface_type = %v", dynamic["surface_type"])
	}

	for _, key := range []string{
		"current_status", "current_snapshot",
		"relationship_lane", "relationship_descriptor_lane",
		"latest_interaction_anchor", "milestone_ledger",
		"recent_change_summary", "digest_budget",
	} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("dynamic missing current-surface key %q", key)
		}
	}

	durableKeys := []string{
		"appearance_observable", "appearance_non_observable",
		"appearance_core", "durable_profile", "sparse_policy",
	}
	for _, key := range durableKeys {
		if _, ok := dynamic[key]; ok {
			t.Fatalf("REPLAY: dynamic digest must NOT contain durable key %q", key)
		}
	}

	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain intact")
	}

	if _, ok := resp["stable_character_sheet"]; !ok {
		t.Fatal("response missing stable_character_sheet")
	}
	if _, ok := resp["dynamic_continuity_digest"]; !ok {
		t.Fatal("response missing dynamic_continuity_digest")
	}

	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("REPLAY: sparse_policy.dynamic_redirects missing %q", want)
		}
	}
}

// TestSeq14P79RelationLaneContinuityReplay verifies that relationship_lane
// and latest_interaction_anchor are connected within the same dynamic continuity
// digest read surface. Protagonist relation, other relations, and the latest
// interaction anchor must all reside in the dynamic digest, and none must leak
// into the stable sheet.
func TestSeq14P79RelationLaneContinuityReplay(t *testing.T) {
	item := store.CharacterState{
		ID:                79,
		ChatSessionID:     "sess-seq14-p79",
		CharacterName:     "Mira",
		PersonalityJSON:   `{"core":"steadfast"}`,
		StatusJSON:        `{"location":"bridge"}`,
		RelationshipsJSON: `{"user":{"summary":"trusted ally","trust":0.9},"Rowan":{"summary":"uneasy rivalry","tension":0.6},"Nia":{"summary":"old debt","trust":0.5}}`,
		TurnIndex:         30,
	}
	snapshot := characterStaleSnapshot(item, nil, 30, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)

	event := store.CharacterEvent{
		ID:            790,
		ChatSessionID: "sess-seq14-p79",
		CharacterName: "Mira",
		EventType:     "relationship_shift",
		TurnIndex:     29,
		DetailsJSON:   `{"summary":"Mira defended the protagonist in council.","before":{"trust":0.5},"after":{"trust":0.9}}`,
	}
	anchor := buildCharacterLatestInteractionAnchor(&event)

	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, anchor, []store.CharacterEvent{event})
	stable := buildStableCharacterSheet(item, snapshot)

	prot, ok := dynamic["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatal("REPLAY: dynamic digest missing protagonist_relation")
	}
	if prot["target"] == nil || prot["target"] == "" {
		t.Fatal("protagonist_relation must have target")
	}

	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("REPLAY: dynamic digest missing other_relations")
	}

	rlItems := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(rlItems) == 0 {
		t.Fatal("REPLAY: dynamic digest missing relationship_lane items")
	}

	anchorMap, ok := dynamic["latest_interaction_anchor"].(map[string]any)
	if !ok || anchorMap == nil {
		t.Fatal("REPLAY: dynamic digest missing latest_interaction_anchor")
	}
	if anchorMap["event_type"] != "relationship_shift" {
		t.Fatalf("anchor event_type = %v, want relationship_shift", anchorMap["event_type"])
	}
	if anchorMap["turn_index"] != 29 {
		t.Fatalf("anchor turn_index = %v, want 29", anchorMap["turn_index"])
	}

	if dynamic["relationship_display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("relationship_display_mode = %v", dynamic["relationship_display_mode"])
	}

	rdlRaw, ok := dynamic["relationship_descriptor_lane"]
	if !ok {
		t.Fatal("REPLAY: dynamic digest missing relationship_descriptor_lane")
	}
	rdl, ok := rdlRaw.([]any)
	if !ok || len(rdl) == 0 {
		t.Fatal("relationship_descriptor_lane must be a non-empty slice")
	}

	relationKeys := []string{
		"protagonist_relation", "other_relations",
		"relationship_lane", "relationship_descriptor_lane",
		"latest_interaction_anchor", "relationship_summary_text",
		"relationship_display_mode",
	}
	for _, key := range relationKeys {
		if _, ok := stable[key]; ok {
			t.Fatalf("REPLAY: stable sheet must NOT contain relation/anchor key %q", key)
		}
	}

	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain stable")
	}
}

// TestSeq14P80GuidanceNonLeakageReplay verifies that the story guidance surface
// (story_frame, turn_directives, execution_contract, fail_mode, conflict_policy,
// precedence) is guidance-only and never becomes a truth authority. It must not
// contain or claim to overwrite canonical_truth_floor, hard_world_rule, or
// latest_direct_evidence.
func TestSeq14P80GuidanceNonLeakageReplay(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "act_2_crisis",
		"narrative_goal":     "force a confrontation",
		"active_tensions":    []string{"hidden_betrayal", "trust_fracture"},
		"next_beats":         []string{"corner_suspect", "reveal_clue"},
		"continuity_anchors": []string{"letter_is_real"},
		"focus_characters":   []string{"Mira", "Kael"},
	}
	director := map[string]any{
		"pressure_level":      "critical",
		"scene_mandate":       "interrogation_scene",
		"required_outcomes":   []string{"expose_lie", "recover_artifact"},
		"forbidden_moves":     []string{"kill_protagonist", "destroy_building"},
		"execution_checklist": []string{"build_tension", "narrow_escape"},
		"world_guardrails":    []string{"no_magic_in_city"},
		"persona_guardrails":  []string{"aria_stays_calm"},
	}

	guidance := buildStoryGuidanceSurface(storyPlan, director)

	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("surface_type = %v", guidance["surface_type"])
	}

	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("REPLAY: guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}

	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	for _, src := range []string{"canonical_truth_floor", "hard_world_rule", "latest_direct_evidence"} {
		if !seq14Contains(hps, src) {
			t.Fatalf("REPLAY: higher_priority_sources must include %q, got %v", src, hps)
		}
	}

	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{"canonical_truth_floor_overwrite", "hard_world_rule_bypass"} {
		if !seq14Contains(du, want) {
			t.Fatalf("REPLAY: disallowed_usage must block %q, got %v", want, du)
		}
	}

	truthKeys := []string{
		"canonical_truth_floor", "hard_world_rule",
		"latest_direct_evidence", "current_user_input",
		"explicit_user_correction", "character_state",
		"stable_character_sheet", "dynamic_continuity_digest",
	}
	for _, key := range truthKeys {
		if _, ok := guidance[key]; ok {
			t.Fatalf("REPLAY: guidance surface must NOT contain truth-floor key %q", key)
		}
	}

	sf := seq14SubMap(t, guidance, "story_frame")
	td := seq14SubMap(t, guidance, "turn_directives")
	for _, sub := range []struct {
		name string
		m    map[string]any
	}{{"story_frame", sf}, {"turn_directives", td}} {
		for _, key := range truthKeys {
			if _, ok := sub.m[key]; ok {
				t.Fatalf("REPLAY: %s must NOT contain truth-floor key %q", sub.name, key)
			}
		}
	}

	ec := seq14SubMap(t, td, "execution_contract")
	if ec["must_hit"] == nil {
		t.Fatal("execution_contract.must_hit must be present")
	}
	if ec["forbidden"] == nil {
		t.Fatal("execution_contract.forbidden must be present")
	}

	for _, key := range []string{"truth_authority", "canonical_source", "overwrite_policy"} {
		if _, ok := ec[key]; ok {
			t.Fatalf("REPLAY: execution_contract must NOT contain truth authority key %q", key)
		}
	}

	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_scene_jump must be false")
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_forced_resolution must be false")
	}

	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["guidance_may_override_user_input"] != false {
		t.Fatalf("REPLAY: conflict_policy.guidance_may_override_user_input must be false")
	}
	if cp["explicit_user_correction_wins"] != true {
		t.Fatalf("REPLAY: conflict_policy.explicit_user_correction_wins must be true")
	}
}

// TestSeq14P81WeakInputNonOverreachReplay verifies that a weak-input replay
// allows only conservative continuation. Explicit user correction, hard world
// rule, and current user input must all outrank weak-input steering. No forced
// scene jump, forced resolution, or truth overwrite is allowed.
func TestSeq14P81WeakInputNonOverreachReplay(t *testing.T) {

	transient := store.CharacterState{
		ID:            81,
		ChatSessionID: "sess-seq14-p81",
		CharacterName: "the hooded stranger",
		TurnIndex:     5,
	}
	staleSnap := characterStaleSnapshot(transient, nil, 40, "", map[string]struct{}{})
	if staleSnap["is_stale"] != true {
		t.Fatalf("stale low-anchor transient must be stale")
	}
	staleGuard := seq14SubMap(t, staleSnap, "stale_guard")
	if staleGuard["allow_weak_input_carry_forward"] != false {
		t.Fatalf("REPLAY: stale transient must block weak-input carry-forward, got %v", staleGuard["allow_weak_input_carry_forward"])
	}

	anchored := store.CharacterState{
		ID:              82,
		ChatSessionID:   "sess-seq14-p81",
		CharacterName:   "Mira",
		PersonalityJSON: `{"core":"steadfast"}`,
		TurnIndex:       30,
	}
	freshSnap := characterStaleSnapshot(anchored, nil, 30, "", map[string]struct{}{})
	if freshSnap["is_stale"] != false {
		t.Fatalf("anchored fresh row must not be stale")
	}
	freshGuard := seq14SubMap(t, freshSnap, "stale_guard")
	if freshGuard["allow_weak_input_carry_forward"] != true {
		t.Fatalf("REPLAY: anchored fresh row must allow weak-input carry-forward")
	}

	guidance := buildStoryGuidanceSurface(
		map[string]any{
			"current_arc":        "act_2_drift",
			"narrative_goal":     "follow the lead",
			"active_tensions":    []string{"rift"},
			"next_beats":         []string{"quiet_probe"},
			"continuity_anchors": []string{"oath_remains"},
			"focus_characters":   []string{"Mira"},
		},
		map[string]any{
			"pressure_level":    "strong",
			"required_outcomes": []string{},
			"forbidden_moves":   []string{},
		},
	)

	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("REPLAY: guidance_authority must be subordinate under strong pressure, got %v", prec["guidance_authority"])
	}

	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	for _, src := range []string{"current_user_input", "explicit_user_correction", "hard_world_rule"} {
		if !seq14Contains(hps, src) {
			t.Fatalf("REPLAY: higher_priority_sources must include %q to outrank weak-input, got %v", src, hps)
		}
	}

	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["current_user_input_wins"] != true {
		t.Fatalf("REPLAY: current_user_input_wins must be true")
	}
	if cp["explicit_user_correction_wins"] != true {
		t.Fatalf("REPLAY: explicit_user_correction_wins must be true")
	}
	if cp["guidance_may_override_user_input"] != false {
		t.Fatalf("REPLAY: guidance_may_override_user_input must be false")
	}

	td := seq14SubMap(t, guidance, "turn_directives")
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_scene_jump must be false")
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_forced_resolution must be false")
	}
	if fm["respect_explicit_user_correction"] != true {
		t.Fatalf("REPLAY: fail_mode.respect_explicit_user_correction must be true")
	}

	mode, _ := fm["mode"].(string)
	switch mode {
	case "conservative_continuation",
		"scene_continuation_without_scene_jump",
		"carry_forward_without_forcing_resolution",
		"pressure_continuation_without_resolution":

	default:
		t.Fatalf("REPLAY: fail_mode.mode must be a conservative continuation variant, got %q", mode)
	}

	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{
		"canonical_truth_floor_overwrite",
		"hard_world_rule_bypass",
		"explicit_user_correction_override",
	} {
		if !seq14Contains(du, want) {
			t.Fatalf("REPLAY: disallowed_usage must block %q, got %v", want, du)
		}
	}

	stable := buildStableCharacterSheet(anchored, freshSnap)
	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "story_guidance"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("REPLAY: stable sheet must not be affected by weak-input guidance, found key %q", key)
		}
	}
}

// TestSeq14P86StableDynamicContinuitySurfaceSmoke is a smoke-level check that
// buildStableCharacterSheet and buildDynamicCharacterDigest produce distinct
// surfaces with correct surface_type labels and that characterResponseItem
// exposes both side by side.
func TestSeq14P86StableDynamicContinuitySurfaceSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:                86,
		ChatSessionID:     "sess-seq14-p86",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"appearance_observable":"tall, scar on left cheek","appearance_non_observable":"secretly anxious"}`,
		PersonalityJSON:   `{"core":"loyal, quiet"}`,
		StatusJSON:        `{"location":"bridge","emotion":"calm"}`,
		RelationshipsJSON: `{"user":{"summary":"trusts deeply","trust":0.8}}`,
		TurnIndex:         5,
	}
	snapshot := characterStaleSnapshot(item, nil, 5, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)

	stable := buildStableCharacterSheet(item, snapshot)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, nil, nil)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("SMOKE P86: stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("SMOKE P86: dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}

	if _, ok := stable["surface_version"]; !ok {
		t.Fatal("SMOKE P86: stable missing surface_version")
	}
	if _, ok := dynamic["surface_version"]; !ok {
		t.Fatal("SMOKE P86: dynamic missing surface_version")
	}

	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "recent_change_summary"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("SMOKE P86: stable must NOT carry volatile key %q", key)
		}
	}

	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("SMOKE P86: dynamic must carry volatile key %q", key)
		}
	}

	resp := characterResponseItem(item, snapshot, nil, nil)
	if _, ok := resp["stable_character_sheet"]; !ok {
		t.Fatal("SMOKE P86: response missing stable_character_sheet")
	}
	if _, ok := resp["dynamic_continuity_digest"]; !ok {
		t.Fatal("SMOKE P86: response missing dynamic_continuity_digest")
	}
}

// TestSeq14P87RelationLaneLatestInteractionReplaySmoke is a smoke-level check
// that relationship_lane and latest_interaction_anchor are connected inside
// the dynamic_continuity_digest and never leak into the stable_character_sheet.
func TestSeq14P87RelationLaneLatestInteractionReplaySmoke(t *testing.T) {
	item := store.CharacterState{
		ID:                87,
		ChatSessionID:     "sess-seq14-p87",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"core":"short, silver hair"}`,
		PersonalityJSON:   `{"core":"calm"}`,
		StatusJSON:        `{"location":"bridge"}`,
		RelationshipsJSON: `{"user":{"summary":"wary ally","trust":0.6},"Kael":{"summary":"rival","tension":0.7},"Lira":{"summary":"mentor","trust":0.8}}`,
		TurnIndex:         12,
	}
	snapshot := characterStaleSnapshot(item, nil, 12, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)

	events := []store.CharacterEvent{
		{
			ID:            870,
			ChatSessionID: "sess-seq14-p87",
			CharacterName: "Mira",
			EventType:     "relationship_shift",
			TurnIndex:     11,
			DetailsJSON:   `{"summary":"Kael challenged the protagonist publicly."}`,
		},
	}
	anchor := buildCharacterLatestInteractionAnchor(&events[0])
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, anchor, events)

	relItems := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(relItems) == 0 {
		t.Fatal("SMOKE P87: relationship_lane must have items")
	}

	if _, ok := dynamic["protagonist_relation"].(map[string]any); !ok {
		t.Fatal("SMOKE P87: dynamic missing protagonist_relation")
	}
	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("SMOKE P87: dynamic missing other_relations")
	}

	anchorMap, ok := dynamic["latest_interaction_anchor"].(map[string]any)
	if !ok {
		t.Fatalf("SMOKE P87: latest_interaction_anchor must be map, got %T", dynamic["latest_interaction_anchor"])
	}
	if anchorMap["event_type"] != "relationship_shift" {
		t.Fatalf("SMOKE P87: latest_interaction_anchor event_type = %v", anchorMap["event_type"])
	}

	stable := buildStableCharacterSheet(item, snapshot)
	for _, key := range []string{"relationship_lane", "latest_interaction_anchor", "current_status", "protagonist_relation"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("SMOKE P87: stable must NOT leak dynamic key %q", key)
		}
	}
}

// TestSeq14P88StoryGuidanceGuidanceOnlySurfaceSmoke is a smoke-level check
// that the story guidance surface (story_frame, turn_directives) remains
// guidance-only and never escalates to truth authority, forced resolution,
// or forced scene jump.
func TestSeq14P88StoryGuidanceGuidanceOnlySurfaceSmoke(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "betrayal_arc",
		"narrative_goal":     "deepen the betrayal tension",
		"active_tensions":    []string{"hidden_love", "trust_fracture"},
		"next_beats":         []string{"confront_suspect"},
		"continuity_anchors": []string{"oath_from_ep3"},
		"focus_characters":   []string{"Mira"},
	}
	director := map[string]any{
		"pressure_level":    "steady",
		"required_outcomes": []string{"expose_lie"},
		"forbidden_moves":   []string{"kill_protagonist"},
		"scene_mandate":     "interrogation_scene",
	}

	guidance := buildStoryGuidanceSurface(storyPlan, director)

	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("SMOKE P88: surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}

	sf := seq14SubMap(t, guidance, "story_frame")
	if sf["stage_type"] != "story_frame" {
		t.Fatalf("SMOKE P88: story_frame stage_type = %v", sf["stage_type"])
	}

	td := seq14SubMap(t, guidance, "turn_directives")
	if td["stage_type"] != "turn_directives" {
		t.Fatalf("SMOKE P88: turn_directives stage_type = %v", td["stage_type"])
	}

	for _, key := range []string{"scene_drive", "carry_targets", "blocked_routes", "tempo_band", "handoff_edge"} {
		if _, ok := sf[key]; ok {
			t.Fatalf("SMOKE P88: story_frame must NOT carry turn-level key %q", key)
		}
	}

	for _, key := range []string{"truth_floor", "canonical_truth", "world_rule_authority"} {
		if _, ok := td[key]; ok {
			t.Fatalf("SMOKE P88: turn_directives must NOT be truth authority, found key %q", key)
		}
	}

	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("SMOKE P88: fail_mode.allow_forced_resolution must be false, got %v", fm["allow_forced_resolution"])
	}
	if fm["allow_scene_jump"] != false {
		t.Fatalf("SMOKE P88: fail_mode.allow_scene_jump must be false, got %v", fm["allow_scene_jump"])
	}

	ec, ok := td["execution_contract"].(map[string]any)
	if !ok {
		t.Fatal("SMOKE P88: turn_directives missing execution_contract")
	}
	if ec["truth_authority"] == true || ec["canonical_override"] == true {
		t.Fatalf("SMOKE P88: execution_contract must NOT grant truth authority: %v", ec)
	}

	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("SMOKE P88: guidance_authority = %v, want subordinate", prec["guidance_authority"])
	}
}

// TestSeq14P89Step14NamingSchemaHelperConsistency verifies that the Step 14
// surface naming is consistent and uses local vocabulary without relying on
// runtime rename or external reference remapping. The canonical names must be:
//   - stable_character_sheet
//   - dynamic_continuity_digest
//   - story_guidance_surface
//   - story_frame
//   - turn_directives
//   - relationship_lane
func TestSeq14P89Step14NamingSchemaHelperConsistency(t *testing.T) {
	item := store.CharacterState{
		ID:                89,
		ChatSessionID:     "sess-seq14-p89",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"core":"tall, green eyes"}`,
		PersonalityJSON:   `{"core":"steadfast"}`,
		StatusJSON:        `{"location":"village"}`,
		RelationshipsJSON: `{"user":{"summary":"respects","trust":0.7}}`,
		TurnIndex:         3,
	}
	snapshot := characterStaleSnapshot(item, nil, 3, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)

	stable := buildStableCharacterSheet(item, snapshot)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, nil, nil)

	storyPlan := map[string]any{
		"current_arc":        "rising_arc",
		"narrative_goal":     "mid_build",
		"active_tensions":    []string{"opening_tension"},
		"next_beats":         []string{"quiet_probe"},
		"continuity_anchors": []string{"oath"},
		"focus_characters":   []string{"Mira"},
	}
	director := map[string]any{
		"pressure_level":    "steady",
		"required_outcomes": []string{},
		"forbidden_moves":   []string{},
	}
	guidance := buildStoryGuidanceSurface(storyPlan, director)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("NAMING P89: stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("NAMING P89: dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("NAMING P89: guidance surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}

	sf := seq14SubMap(t, guidance, "story_frame")
	td := seq14SubMap(t, guidance, "turn_directives")
	if sf["stage_type"] != "story_frame" {
		t.Fatalf("NAMING P89: story_frame stage_type = %v", sf["stage_type"])
	}
	if td["stage_type"] != "turn_directives" {
		t.Fatalf("NAMING P89: turn_directives stage_type = %v", td["stage_type"])
	}

	if _, ok := dynamic["relationship_lane"]; !ok {
		t.Fatal("NAMING P89: dynamic missing relationship_lane (canonical name)")
	}

	resp := characterResponseItem(item, snapshot, nil, nil)
	canonicalResponseKeys := []string{
		"stable_character_sheet",
		"dynamic_continuity_digest",
	}
	for _, key := range canonicalResponseKeys {
		if _, ok := resp[key]; !ok {
			t.Fatalf("NAMING P89: response missing canonical key %q", key)
		}
	}

	if _, ok := stable["surface_version"]; !ok {
		t.Fatal("NAMING P89: stable missing surface_version")
	}
	if _, ok := dynamic["surface_version"]; !ok {
		t.Fatal("NAMING P89: dynamic missing surface_version")
	}

	forbiddenRenameKeys := []string{"legacy_name", "alias_name", "remap_from", "rename_from"}
	for label, surface := range map[string]map[string]any{
		"stable":  stable,
		"dynamic": dynamic,
	} {
		for _, key := range forbiddenRenameKeys {
			if _, ok := surface[key]; ok {
				t.Fatalf("NAMING P89: %s must not contain runtime rename key %q", label, key)
			}
		}
	}
}

// TestSeq14P93StableCharacterSheetSurfaceFinalSmoke performs a final smoke test
// for the stable_character_sheet surface. Every expected key, axis label,
// version marker, and policy block must be present with the correct schema.
func TestSeq14P93StableCharacterSheetSurfaceFinalSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:              93,
		ChatSessionID:   "sess-seq14-p93",
		CharacterName:   "Elara",
		AppearanceJSON:  `{"core":"tall, silver hair","outfit":"dark cloak"}`,
		PersonalityJSON: `{"core":"calm","quirk":"dry humor"}`,
		StatusJSON:      `{"location":"harbor","mood":"focused"}`,
		SpeechStyleJSON: `{"tone":"measured","pace":"slow"}`,
		TurnIndex:       5,
	}
	snapshot := characterStaleSnapshot(item, nil, 5, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)

	if stable["surface_version"] != "cc14a.v1" {
		t.Fatalf("P93: surface_version = %v, want cc14a.v1", stable["surface_version"])
	}
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("P93: surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}

	status, ok := stable["status"].(string)
	if !ok || status == "" {
		t.Fatal("P93: missing or empty status")
	}

	filledAxes := seq14StringSlice(t, stable["filled_axes"], "filled_axes")
	for _, axis := range []string{"appearance", "personality", "speech_style"} {
		if !seq14Contains(filledAxes, axis) {
			t.Fatalf("P93: filled_axes missing %q", axis)
		}
	}

	for _, key := range []string{"appearance", "appearance_core", "appearance_observable", "appearance_non_observable", "appearance_snapshot_keys", "personality", "speech_style"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("P93: stable missing top-level key %q", key)
		}
	}

	dp := seq14SubMap(t, stable, "durable_profile")
	for _, key := range []string{"appearance", "personality", "speech_style"} {
		if _, ok := dp[key]; !ok {
			t.Fatalf("P93: durable_profile missing key %q", key)
		}
	}

	sp := seq14SubMap(t, stable, "sparse_policy")
	if sp["mode"] != "omit_unknown_fields" {
		t.Fatalf("P93: sparse_policy.mode = %v, want omit_unknown_fields", sp["mode"])
	}
	seq14AssertStringSlice(t, sp, "filled_axes", 1)
	seq14AssertStringSlice(t, sp, "empty_axes", 0)
	seq14AssertStringSlice(t, sp, "dynamic_redirects", 1)
	redirects := seq14StringSlice(t, sp["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("P93: dynamic_redirects missing %q", want)
		}
	}

	if stable["source_turn"] != snapshot["last_observed_turn"] {
		t.Fatalf("P93: source_turn = %v, snapshot last_observed_turn = %v", stable["source_turn"], snapshot["last_observed_turn"])
	}
}

// TestSeq14P94DynamicDigestSurfaceFinalSmoke performs a final smoke test for
// the dynamic_continuity_digest surface. All expected keys, version markers,
// axis labels, admission metadata, budget block, and milestone ledger must be
// present and structurally correct.
func TestSeq14P94DynamicDigestSurfaceFinalSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:                94,
		ChatSessionID:     "sess-seq14-p94",
		CharacterName:     "Kael",
		AppearanceJSON:    `{"core":"lean build","outfit":"traveler gear"}`,
		PersonalityJSON:   `{"core":"resourceful"}`,
		StatusJSON:        `{"location":"market","emotion":"alert"}`,
		RelationshipsJSON: `{"user":{"summary":"trusting ally","trust":0.8}}`,
		TurnIndex:         6,
	}
	events := []store.CharacterEvent{
		{ID: 200, ChatSessionID: "sess-seq14-p94", CharacterName: "Kael", EventType: "relationship_shift", TurnIndex: 5, DetailsJSON: `{"interaction":"helped defend"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 6, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)
	latestEvent := events[0]
	latestAnchor := buildCharacterLatestInteractionAnchor(&latestEvent)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, latestAnchor, events)

	if dynamic["surface_version"] != "cc14b.v1" {
		t.Fatalf("P94: surface_version = %v, want cc14b.v1", dynamic["surface_version"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("P94: surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}

	if _, ok := dynamic["status"].(string); !ok {
		t.Fatal("P94: missing status string")
	}

	filledAxes := seq14StringSlice(t, dynamic["filled_axes"], "filled_axes")
	if len(filledAxes) == 0 {
		t.Fatal("P94: filled_axes must not be empty")
	}
	if !seq14Contains(filledAxes, "current_status") {
		t.Fatal("P94: filled_axes missing current_status")
	}

	if dynamic["admission_class"] == nil {
		t.Fatal("P94: missing admission_class")
	}
	if dynamic["admission_basis"] == nil {
		t.Fatal("P94: missing admission_basis")
	}

	sg := seq14SubMap(t, dynamic, "stale_guard")
	if _, ok := sg["active"]; !ok {
		t.Fatal("P94: stale_guard missing active")
	}

	if dynamic["current_status"] == nil {
		t.Fatal("P94: missing current_status")
	}
	if _, ok := dynamic["current_status_summary"].(string); !ok {
		t.Fatal("P94: missing current_status_summary")
	}

	if _, ok := dynamic["current_snapshot"].(map[string]any); !ok {
		t.Fatal("P94: missing current_snapshot map")
	}

	if _, ok := dynamic["appearance_snapshot"]; !ok {
		t.Fatal("P94: missing appearance_snapshot")
	}

	for _, key := range []string{"relationship_summary_text", "relationship_primary_target", "relationship_display_mode", "protagonist_relation", "relationship_lane", "other_relations", "relationship_descriptor_lane"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("P94: dynamic missing relationship field %q", key)
		}
	}

	if dynamic["latest_interaction_anchor"] == nil {
		t.Fatal("P94: missing latest_interaction_anchor")
	}

	if _, ok := dynamic["milestone_ledger"].([]any); !ok {
		t.Fatal("P94: milestone_ledger must be []any")
	}

	db := seq14SubMap(t, dynamic, "digest_budget")
	if db["policy"] != "priority_capped" {
		t.Fatalf("P94: digest_budget.policy = %v, want priority_capped", db["policy"])
	}
	for _, key := range []string{"relationship_lane_cap", "milestone_cap", "milestone_read_window", "milestone_selection_policy", "relationship_lane_used", "milestones_used"} {
		if _, ok := db[key]; !ok {
			t.Fatalf("P94: digest_budget missing key %q", key)
		}
	}

	if _, ok := dynamic["recent_change_summary"]; !ok {
		t.Fatal("P94: missing recent_change_summary")
	}

	if dynamic["source_turn"] == nil {
		t.Fatal("P94: missing source_turn")
	}
}
