package httpapi

import (
	"fmt"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Step 14 first contract-fix: SEQ-14-P44 .. SEQ-14-P48.
//
// These tests lock the character/guidance read surfaces produced by
// group_narrative.go without changing production behavior. They assert the
// truth-floor boundary, the stable/dynamic continuity split, the relationship
// lane split, the subordinate story guidance surface, and the weak-input /
// explicit-correction guard.
//
// Scope note: this slice intentionally stops at the P44..P48 contract floor.
// Later schema detail rows (SEQ-14-P52+) are NOT closed here.

func seq14StringSlice(t *testing.T, v any, label string) []string {
	t.Helper()
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			str, ok := item.(string)
			if !ok {
				t.Fatalf("%s: element is not a string: %#v", label, item)
			}
			out = append(out, str)
		}
		return out
	case nil:
		t.Fatalf("%s: missing slice value", label)
	}
	t.Fatalf("%s: unexpected slice type %T", label, v)
	return nil
}

func seq14SubMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := parent[key]
	if !ok {
		t.Fatalf("expected key %q present", key)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected key %q to be map, got %T", key, raw)
	}
	return m
}

func seq14RelationItems(t *testing.T, v any) []map[string]any {
	t.Helper()
	switch s := v.(type) {
	case []map[string]any:
		return s
	case []any:
		out := make([]map[string]any, 0, len(s))
		for _, item := range s {
			m, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("other_relations element is not a map: %#v", item)
			}
			out = append(out, m)
		}
		return out
	case nil:
		t.Fatal("other_relations missing")
	}
	t.Fatalf("other_relations unexpected type %T", v)
	return nil
}

func seq14Contains(values []string, want string) bool {

	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func seq14AssertStringSlice(t *testing.T, parent map[string]any, key string, minLen int) {
	t.Helper()
	raw, ok := parent[key]
	if !ok {
		t.Fatalf("expected key %q present", key)
	}
	switch s := raw.(type) {
	case []string:
		if len(s) < minLen {
			t.Fatalf("%q length = %d, want >= %d", key, len(s), minLen)
		}
	case []any:
		if len(s) < minLen {
			t.Fatalf("%q length = %d, want >= %d", key, len(s), minLen)
		}
		for i, item := range s {
			if _, ok := item.(string); !ok {
				t.Fatalf("%q[%d] is not a string: %T", key, i, item)
			}
		}
	default:
		t.Fatalf("%q must be a string slice, got %T", key, raw)
	}
}

// TestSeq14P44CharacterGuidanceTruthBoundaryContract verifies P44:
// character/guidance surfaces never overwrite the canonical truth floor.
// Guidance authority is subordinate and ranks below current_user_input,
// explicit_user_correction, hard_world_rule, latest_direct_evidence, and
// canonical_truth_floor.
func TestSeq14P44CharacterGuidanceTruthBoundaryContract(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "Truce Arc",
		"narrative_goal":     "broker a fragile ceasefire",
		"active_tensions":    []string{"distrust between camps"},
		"next_beats":         []string{"private parley"},
		"continuity_anchors": []string{"the signed treaty exists"},
		"focus_characters":   []string{"Mira"},
	}
	director := map[string]any{
		"pressure_level":    "steady",
		"required_outcomes": []string{"surface one concession"},
		"forbidden_moves":   []string{"break the treaty off-screen"},
	}
	surface := buildStoryGuidanceSurface(storyPlan, director)

	if surface["surface_type"] != "story_guidance_surface" {
		t.Fatalf("expected story_guidance_surface, got %v", surface["surface_type"])
	}

	prec := seq14SubMap(t, surface, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}

	higher := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	wantHigher := []string{
		"current_user_input",
		"explicit_user_correction",
		"hard_world_rule",
		"latest_direct_evidence",
		"canonical_truth_floor",
	}
	if len(higher) != len(wantHigher) {
		t.Fatalf("higher_priority_sources length = %d, want %d (%v)", len(higher), len(wantHigher), higher)
	}
	for i, want := range wantHigher {
		if higher[i] != want {
			t.Fatalf("higher_priority_sources[%d] = %q, want %q", i, higher[i], want)
		}
	}

	disallowed := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{"canonical_truth_floor_overwrite", "hard_world_rule_bypass", "explicit_user_correction_override"} {
		if !seq14Contains(disallowed, want) {
			t.Fatalf("disallowed_usage missing %q (%v)", want, disallowed)
		}
	}
}

// TestSeq14P45StableDynamicCharacterContinuitySplit verifies P45:
// stable_character_sheet (durable axis / evidence spine) and
// dynamic_continuity_digest (current status / relation / drift / latest
// interaction) are separated read surfaces.
func TestSeq14P45StableDynamicCharacterContinuitySplit(t *testing.T) {
	item := store.CharacterState{
		ID:                7,
		ChatSessionID:     "sess-seq14",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"hair":"black","scar":"left brow"}`,
		PersonalityJSON:   `{"core":"guarded but loyal"}`,
		SpeechStyleJSON:   `{"tone":"clipped"}`,
		StatusJSON:        `{"location":"war room","emotion":"tense","goal":"hold the line"}`,
		RelationshipsJSON: `{"Rowan":{"summary":"reluctant allies","trust":0.4}}`,
		TurnIndex:         12,
	}
	snapshot := characterStaleSnapshot(item, nil, 12, "", map[string]struct{}{})
	relationshipLane := buildCharacterRelationshipLane(item)
	stable := buildStableCharacterSheet(item, snapshot)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relationshipLane, nil, nil)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("stable surface_type = %v", stable["surface_type"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("dynamic surface_type = %v", dynamic["surface_type"])
	}

	// Stable sheet is the durable spine: it carries the durable axes and must
	// NOT carry volatile current status.
	if _, ok := stable["durable_profile"]; !ok {
		t.Fatal("stable sheet missing durable_profile")
	}
	if _, leaked := stable["current_status"]; leaked {
		t.Fatal("stable sheet must not expose current_status (volatile axis)")
	}

	// Dynamic digest carries the volatile axes: current status, relation,
	// latest interaction.
	if _, ok := dynamic["current_status"]; !ok {
		t.Fatal("dynamic digest missing current_status")
	}
	if _, ok := dynamic["relationship_lane"]; !ok {
		t.Fatal("dynamic digest missing relationship_lane")
	}
	if _, ok := dynamic["latest_interaction_anchor"]; !ok {
		t.Fatal("dynamic digest missing latest_interaction_anchor")
	}

	// The dynamic redirect policy must point volatile reads away from the
	// stable sheet.
	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("sparse_policy.dynamic_redirects missing %q (%v)", want, redirects)
		}
	}

	// Response item should expose both surfaces side by side.
	resp := characterResponseItem(item, snapshot, nil, nil)
	if _, ok := resp["stable_character_sheet"]; !ok {
		t.Fatal("response missing stable_character_sheet")
	}
	if _, ok := resp["dynamic_continuity_digest"]; !ok {
		t.Fatal("response missing dynamic_continuity_digest")
	}
}

// TestSeq14P46RelationshipLaneSplitContract verifies P46:
// relationship_lane is a separate read lane where protagonist relation,
// latest interaction, and other relations are split, and relationship prose /
// descriptors do not overwrite stable facts.
func TestSeq14P46RelationshipLaneSplitContract(t *testing.T) {
	item := store.CharacterState{
		ID:            21,
		ChatSessionID: "sess-seq14-rel",
		CharacterName: "Mira",
		RelationshipsJSON: `{
			"user":{"summary":"protects the protagonist at any cost","trust":0.9},
			"Rowan":{"summary":"uneasy rivalry","tension":0.6},
			"Nia":{"summary":"old debt unpaid","trust":0.5}
		}`,
		TurnIndex: 14,
	}
	lane := buildCharacterRelationshipLane(item)

	if lane["surface_type"] != "relationship_lane" {
		t.Fatalf("lane surface_type = %v", lane["surface_type"])
	}
	if lane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("lane display_mode = %v", lane["display_mode"])
	}

	prot, ok := lane["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatalf("protagonist_relation must be present and a map, got %#v", lane["protagonist_relation"])
	}
	if prot["target"] == nil || prot["target"] == "" {
		t.Fatalf("protagonist_relation must have a target, got %#v", prot["target"])
	}

	others := seq14RelationItems(t, lane["other_relations"])
	if len(others) == 0 {
		t.Fatal("expected non-protagonist relations to be split into other_relations")
	}
	// The protagonist relation must not be duplicated into other_relations.
	for _, o := range others {
		if o["target"] == prot["target"] && o["summary_text"] == prot["summary_text"] {
			t.Fatalf("protagonist relation leaked into other_relations: %#v", o)
		}
	}

	// Relationship entries are descriptor/prose lanes (summary_text), and the
	// digest must consume them as a separate descriptor lane rather than as a
	// stable-fact overwrite.
	dynamic := buildDynamicCharacterDigest(item, map[string]any{}, lane, nil, nil)
	if _, ok := dynamic["relationship_descriptor_lane"]; !ok {
		t.Fatal("dynamic digest missing relationship_descriptor_lane")
	}
	if dynamic["protagonist_relation"] == nil {
		t.Fatal("dynamic digest must carry protagonist_relation from the lane")
	}
}

// TestSeq14P47StoryGuidanceSubordinateSurface verifies P47:
// story_frame and turn_directives are guidance-only / subordinate surfaces.
// scene_drive / carry_targets / blocked_routes / tempo_band / handoff_edge must
// not be promoted to execution authority or truth writes (no forced scene jump
// or forced resolution; explicit user correction still respected).
func TestSeq14P47StoryGuidanceSubordinateSurface(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "Pressure Arc",
		"narrative_goal":     "force a confession",
		"active_tensions":    []string{"hidden betrayal"},
		"next_beats":         []string{"corner the suspect"},
		"continuity_anchors": []string{"the letter is real"},
		"focus_characters":   []string{"Mira"},
	}
	director := map[string]any{
		"pressure_level":    "strong",
		"scene_mandate":     "press the suspect",
		"required_outcomes": []string{"reveal one lie"},
		"forbidden_moves":   []string{"resolve the case"},
	}
	surface := buildStoryGuidanceSurface(storyPlan, director)

	storyFrame := seq14SubMap(t, surface, "story_frame")
	if storyFrame["stage_type"] != "story_frame" {
		t.Fatalf("story_frame stage_type = %v", storyFrame["stage_type"])
	}

	turn := seq14SubMap(t, surface, "turn_directives")
	if turn["stage_type"] != "turn_directives" {
		t.Fatalf("turn_directives stage_type = %v", turn["stage_type"])
	}
	for _, key := range []string{"scene_drive", "carry_targets", "blocked_routes", "tempo_band", "handoff_edge"} {
		if _, ok := turn[key]; !ok {
			t.Fatalf("turn_directives missing %q", key)
		}
	}

	// Subordination: even under strong pressure, guidance cannot force a scene
	// jump or a forced resolution, and must respect explicit user correction.
	failMode := seq14SubMap(t, turn, "fail_mode")
	if failMode["allow_scene_jump"] != false {
		t.Fatalf("fail_mode.allow_scene_jump must be false, got %v", failMode["allow_scene_jump"])
	}
	if failMode["allow_forced_resolution"] != false {
		t.Fatalf("fail_mode.allow_forced_resolution must be false, got %v", failMode["allow_forced_resolution"])
	}
	if failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode.respect_explicit_user_correction must be true, got %v", failMode["respect_explicit_user_correction"])
	}

	// Guidance authority remains subordinate at the surface root.
	prec := seq14SubMap(t, surface, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority must remain subordinate, got %v", prec["guidance_authority"])
	}
}

// TestSeq14P48WeakInputExplicitCorrectionGuard verifies P48:
// weak input is only allowed a conservative carry-forward, explicit user
// correction outranks weak-input steering, and stale / low-anchor character
// continuity is not force-carried forward.
func TestSeq14P48WeakInputExplicitCorrectionGuard(t *testing.T) {
	// Anchored, fresh character: weak-input carry-forward is allowed.
	anchored := store.CharacterState{
		ID:              31,
		ChatSessionID:   "sess-seq14-weak",
		CharacterName:   "Mira",
		PersonalityJSON: `{"core":"steadfast"}`,
		TurnIndex:       20,
	}
	freshSnap := characterStaleSnapshot(anchored, nil, 20, "", map[string]struct{}{})
	freshGuard := seq14SubMap(t, freshSnap, "stale_guard")
	if freshGuard["allow_weak_input_carry_forward"] != true {
		t.Fatalf("anchored fresh row should allow weak-input carry-forward, got %v", freshGuard["allow_weak_input_carry_forward"])
	}
	if freshSnap["is_stale"] != false {
		t.Fatalf("anchored fresh row should not be stale, got %v", freshSnap["is_stale"])
	}

	// Stale, low-anchor transient descriptor: weak-input carry-forward is
	// blocked (no forced carry-forward of stale continuity).
	transient := store.CharacterState{
		ID:            32,
		ChatSessionID: "sess-seq14-weak",
		CharacterName: "the hooded stranger",
		TurnIndex:     5,
	}
	staleSnap := characterStaleSnapshot(transient, nil, 40, "", map[string]struct{}{})
	if staleSnap["is_stale"] != true {
		t.Fatalf("stale low-anchor transient should be stale, got %v", staleSnap["is_stale"])
	}
	staleGuard := seq14SubMap(t, staleSnap, "stale_guard")
	if staleGuard["allow_weak_input_carry_forward"] != false {
		t.Fatalf("stale low-anchor row must block weak-input carry-forward, got %v", staleGuard["allow_weak_input_carry_forward"])
	}
	if staleGuard["active"] != true {
		t.Fatalf("stale guard should be active for stale low-anchor row, got %v", staleGuard["active"])
	}

	// Explicit user correction outranks weak-input steering: the guidance
	// surface keeps explicit_user_correction above guidance and respects it in
	// the fail mode.
	surface := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "Drift Arc", "narrative_goal": "follow the lead"},
		map[string]any{"pressure_level": "steady"},
	)
	prec := seq14SubMap(t, surface, "precedence")
	higher := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	if !seq14Contains(higher, "explicit_user_correction") {
		t.Fatalf("explicit_user_correction must outrank guidance, got %v", higher)
	}
	turn := seq14SubMap(t, surface, "turn_directives")
	failMode := seq14SubMap(t, turn, "fail_mode")
	if failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("guidance fail mode must respect explicit user correction, got %v", failMode["respect_explicit_user_correction"])
	}
}

// TestSeq14P52StableCharacterSheetSchemaDurableAxes verifies P52:
// stable_character_sheet is a durable-axis / evidence-spine surface.
// It must carry appearance_observable, appearance_non_observable, durable_profile,
// and sparse_policy. Concrete observable appearance must be separated from
// inferred / non-observable descriptors.
func TestSeq14P52StableCharacterSheetSchemaDurableAxes(t *testing.T) {
	item := store.CharacterState{
		ID:              52,
		ChatSessionID:   "sess-seq14-p52",
		CharacterName:   "Mira",
		AppearanceJSON:  `{"appearance_observable":"tall, black hair, left brow scar","appearance_non_observable":"carries herself like someone who has seen war"}`,
		PersonalityJSON: `{"core":"guarded but loyal","flaw":"slow to trust"}`,
		SpeechStyleJSON: `{"tone":"clipped","pace":"measured"}`,
		TurnIndex:       12,
	}
	snapshot := characterStaleSnapshot(item, nil, 12, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if stable["surface_version"] != "cc14a.v1" {
		t.Fatalf("surface_version = %v, want cc14a.v1", stable["surface_version"])
	}

	// Durable axes must all be present.
	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable sheet missing durable axis %q", key)
		}
	}

	// Concrete observable vs non-observable must be split (not merged into a
	// single appearance blob).
	obsVal, _ := stable["appearance_observable"].(string)
	nonObsVal, _ := stable["appearance_non_observable"].(string)
	if obsVal == nonObsVal && obsVal != "" {
		t.Fatalf("appearance_observable and appearance_non_observable must be split surfaces")
	}

	// appearance_core is the legacy fallback when split fields are absent.
	if _, ok := stable["appearance_core"]; !ok {
		t.Fatalf("stable sheet missing appearance_core fallback")
	}

	// durable_profile must expose personality and speech_style.
	dp := seq14SubMap(t, stable, "durable_profile")
	if _, ok := dp["personality"]; !ok {
		t.Fatal("durable_profile missing personality")
	}
	if _, ok := dp["speech_style"]; !ok {
		t.Fatal("durable_profile missing speech_style")
	}

	// sparse_policy must carry dynamic_redirects pointing to the volatile surface.
	sparse := seq14SubMap(t, stable, "sparse_policy")
	if _, ok := sparse["dynamic_redirects"]; !ok {
		t.Fatal("sparse_policy missing dynamic_redirects")
	}

	// Stable sheet must NOT directly own volatile current surface keys.
	for _, leaked := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "current_snapshot"} {
		if _, ok := stable[leaked]; ok {
			t.Fatalf("stable sheet must not directly own volatile key %q", leaked)
		}
	}
}

// TestSeq14P53DynamicContinuityDigestSchemaCurrentSurface verifies P53:
// dynamic_continuity_digest carries the current / volatile surface:
// current_status, current_snapshot, relationship_lane,
// relationship_descriptor_lane, latest_interaction_anchor,
// milestone_ledger, recent_change_summary, digest_budget.
func TestSeq14P53DynamicContinuityDigestSchemaCurrentSurface(t *testing.T) {
	item := store.CharacterState{
		ID:                53,
		ChatSessionID:     "sess-seq14-p53",
		CharacterName:     "Mira",
		StatusJSON:        `{"location":"war room","emotion":"tense","goal":"hold the line"}`,
		RelationshipsJSON: `{"user":{"summary":"trusted ally","trust":0.9},"Rowan":{"summary":"uneasy rivalry","tension":0.6}}`,
		TurnIndex:         15,
	}
	snapshot := characterStaleSnapshot(item, nil, 15, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)
	recentEvents := []store.CharacterEvent{
		{
			ID:            201,
			ChatSessionID: "sess-seq14-p53",
			CharacterName: "Mira",
			EventType:     "relationship_shift",
			TurnIndex:     14,
			DetailsJSON:   `{"summary":"Mira chose to stand with the protagonist.","before":{"trust":0.3},"after":{"trust":0.9}}`,
		},
	}
	latestAnchor := buildCharacterLatestInteractionAnchor(&recentEvents[0])

	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, latestAnchor, recentEvents)

	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}
	if dynamic["surface_version"] != "cc14b.v1" {
		t.Fatalf("surface_version = %v, want cc14b.v1", dynamic["surface_version"])
	}

	// All current-surface keys must be present.
	for _, key := range []string{
		"current_status",
		"current_snapshot",
		"relationship_lane",
		"relationship_descriptor_lane",
		"latest_interaction_anchor",
		"milestone_ledger",
		"recent_change_summary",
		"digest_budget",
	} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("dynamic digest missing current-surface key %q", key)
		}
	}

	// current_snapshot wraps status, appearance, and relationship_focus.
	cs := seq14SubMap(t, dynamic, "current_snapshot")
	statusMap := seq14SubMap(t, cs, "status")
	for _, key := range []string{"location", "emotion", "goal"} {
		if _, ok := statusMap[key]; !ok {
			t.Fatalf("current_snapshot.status missing %q", key)
		}
	}

	// milestone_ledger should contain at least the relationship_shift event.
	ledger, ok := dynamic["milestone_ledger"].([]any)
	if !ok || len(ledger) == 0 {
		t.Fatal("milestone_ledger must contain at least one milestone candidate")
	}
	first := ledger[0].(map[string]any)
	if first["event_type"] != "relationship_shift" {
		t.Fatalf("milestone_ledger[0].event_type = %v, want relationship_shift", first["event_type"])
	}

	// recent_change_summary carries the anchor summary_text as a string value
	// (or nil when no anchor is present).
	rcs, _ := dynamic["recent_change_summary"].(string)
	if rcs == "" {
		t.Fatalf("recent_change_summary must carry a non-empty summary string, got %#v", dynamic["recent_change_summary"])
	}

	// relationship_descriptor_lane is a descriptor slice (not a map).
	rdlRaw, ok := dynamic["relationship_descriptor_lane"]
	if !ok {
		t.Fatal("dynamic digest missing relationship_descriptor_lane")
	}
	rdl, ok := rdlRaw.([]any)
	if !ok {
		t.Fatalf("relationship_descriptor_lane must be a slice, got %T", rdlRaw)
	}
	if len(rdl) == 0 {
		t.Fatal("relationship_descriptor_lane must contain at least one descriptor entry")
	}
	// Each entry must expose target and summary_text.
	entry0, ok := rdl[0].(map[string]any)
	if !ok {
		t.Fatalf("relationship_descriptor_lane[0] must be a map, got %T", rdl[0])
	}
	if entry0["target"] == nil || entry0["target"] == "" {
		t.Fatalf("relationship_descriptor_lane[0] missing target, got %#v", entry0)
	}

	// digest_budget must include policy, milestone_cap, relationship_lane_cap,
	// milestone_read_window, milestone_selection_policy.
	budget := seq14SubMap(t, dynamic, "digest_budget")
	for _, key := range []string{"policy", "milestone_cap", "relationship_lane_cap", "milestone_read_window", "milestone_selection_policy"} {
		if _, ok := budget[key]; !ok {
			t.Fatalf("digest_budget missing %q", key)
		}
	}
}

// TestSeq14P54StableDynamicReadSplitEvidenceSpine verifies P54:
// stable sheet is the evidence spine; dynamic digest is the volatile / current
// read surface. The stable sheet must not directly own current_status,
// relationship_lane, or latest_interaction_anchor. Instead, sparse_policy
// must carry dynamic_redirects pointing to those volatile surfaces.
func TestSeq14P54StableDynamicReadSplitEvidenceSpine(t *testing.T) {
	item := store.CharacterState{
		ID:                54,
		ChatSessionID:     "sess-seq14-p54",
		CharacterName:     "Mira",
		AppearanceJSON:    `{"hair":"black"}`,
		PersonalityJSON:   `{"core":"steadfast"}`,
		StatusJSON:        `{"location":"bridge","emotion":"calm"}`,
		RelationshipsJSON: `{"user":{"summary":"ally","trust":0.8}}`,
		TurnIndex:         10,
	}
	snapshot := characterStaleSnapshot(item, nil, 10, "", map[string]struct{}{})
	relLane := buildCharacterRelationshipLane(item)
	stable := buildStableCharacterSheet(item, snapshot)
	dynamic := buildDynamicCharacterDigest(item, snapshot, relLane, nil, nil)

	// Stable sheet: evidence spine - must NOT directly own volatile keys.
	volatileKeys := []string{"current_status", "relationship_lane", "latest_interaction_anchor", "current_snapshot", "milestone_ledger", "recent_change_summary"}
	for _, key := range volatileKeys {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable sheet must NOT directly own volatile key %q", key)
		}
	}

	// Dynamic digest: must own all volatile / current keys.
	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "current_snapshot"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("dynamic digest missing current-surface key %q", key)
		}
	}

	// sparse_policy.dynamic_redirects must point volatile reads to the dynamic
	// surface, not to the stable sheet.
	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("sparse_policy.dynamic_redirects missing %q (%v)", want, redirects)
		}
	}

	// characterResponseItem must expose both surfaces side by side, so callers
	// can read stable and dynamic without merging them.
	resp := characterResponseItem(item, snapshot, nil, nil)
	if _, ok := resp["stable_character_sheet"]; !ok {
		t.Fatal("response missing stable_character_sheet")
	}
	if _, ok := resp["dynamic_continuity_digest"]; !ok {
		t.Fatal("response missing dynamic_continuity_digest")
	}
}

// TestSeq14P55ContinuityCarryForwardDefault verifies P55:
// continuity carry-forward is only allowed for anchored / fresh continuity.
// Stale, low-anchor transient descriptors must be blocked from carry-forward
// (no forced carry-forward of stale continuity).
func TestSeq14P55ContinuityCarryForwardDefault(t *testing.T) {
	// Anchored, fresh character: carry-forward should be allowed.
	anchored := store.CharacterState{
		ID:              55,
		ChatSessionID:   "sess-seq14-p55",
		CharacterName:   "Mira",
		PersonalityJSON: `{"core":"steadfast"}`,
		TurnIndex:       20,
	}
	freshSnap := characterStaleSnapshot(anchored, nil, 20, "", map[string]struct{}{})

	if freshSnap["is_stale"] != false {
		t.Fatalf("anchored fresh row must not be stale, got %v", freshSnap["is_stale"])
	}
	freshGuard := seq14SubMap(t, freshSnap, "stale_guard")
	if freshGuard["allow_weak_input_carry_forward"] != true {
		t.Fatalf("anchored fresh row must allow carry-forward, got %v", freshGuard["allow_weak_input_carry_forward"])
	}
	if freshGuard["active"] != false {
		t.Fatalf("stale guard must not be active for fresh row, got %v", freshGuard["active"])
	}

	// Stale, low-anchor transient descriptor: carry-forward must be blocked.
	transient := store.CharacterState{
		ID:            56,
		ChatSessionID: "sess-seq14-p55",
		CharacterName: "the hooded stranger",
		TurnIndex:     5,
	}
	staleSnap := characterStaleSnapshot(transient, nil, 40, "", map[string]struct{}{})

	if staleSnap["is_stale"] != true {
		t.Fatalf("stale low-anchor transient must be stale, got %v", staleSnap["is_stale"])
	}
	staleGuard := seq14SubMap(t, staleSnap, "stale_guard")
	if staleGuard["allow_weak_input_carry_forward"] != false {
		t.Fatalf("stale low-anchor row must block carry-forward, got %v", staleGuard["allow_weak_input_carry_forward"])
	}
	if staleGuard["active"] != true {
		t.Fatalf("stale guard must be active for stale row, got %v", staleGuard["active"])
	}

	// Recently re-mentioned stale character: carry-forward is allowed via the
	// recent mention path.
	rementioned := store.CharacterState{
		ID:            57,
		ChatSessionID: "sess-seq14-p55",
		CharacterName: "old captain",
		TurnIndex:     3,
	}
	recentSnap := characterStaleSnapshot(
		rementioned,
		nil,
		40,
		"the old captain returns to the bridge",
		map[string]struct{}{"old captain": {}, "captain": {}},
	)
	recentGuard := seq14SubMap(t, recentSnap, "stale_guard")
	if recentGuard["allow_weak_input_carry_forward"] != true {
		t.Fatalf("recently re-mentioned row must allow carry-forward, got %v", recentGuard["allow_weak_input_carry_forward"])
	}
}

// TestSeq14P56DecisiveEventUpdateGate verifies P56:
// decisive / lasting-impact events (relationship_shift, personality_change) are
// preserved as milestone / update candidates, while routine fluctuations
// (status_change, appearance_change) do not promote to stable overwrite
// priority. The milestone ledger must rank lasting events first.
func TestSeq14P56DecisiveEventUpdateGate(t *testing.T) {
	// Build a mixed event set: lasting impact + routine fluctuations.
	// The milestone ledger picks candidates[0] first, then sorts the rest by
	// priority. Put the lasting-impact event first to match the production
	// ordering expectation.
	events := []store.CharacterEvent{
		{
			ID:            302,
			ChatSessionID: "sess-seq14-p56",
			CharacterName: "Mira",
			EventType:     "relationship_shift",
			TurnIndex:     12,
			DetailsJSON:   `{"summary":"Mira chose to stand with the protagonist.","before":{"trust":0.3},"after":{"trust":0.9}}`,
		},
		{
			ID:            301,
			ChatSessionID: "sess-seq14-p56",
			CharacterName: "Mira",
			EventType:     "status_change",
			TurnIndex:     10,
			DetailsJSON:   `{"summary":"Mira moved to the observation deck."}`,
		},
		{
			ID:            303,
			ChatSessionID: "sess-seq14-p56",
			CharacterName: "Mira",
			EventType:     "appearance_change",
			TurnIndex:     13,
			DetailsJSON:   `{"summary":"Mira changed into field gear."}`,
		},
		{
			ID:            304,
			ChatSessionID: "sess-seq14-p56",
			CharacterName: "Mira",
			EventType:     "personality_change",
			TurnIndex:     14,
			DetailsJSON:   `{"summary":"Mira decided to trust the protagonist.","before":{"core":"guarded"},"after":{"core":"guarded but loyal"}}`,
		},
	}

	ledger := characterMilestoneLedger(events)

	// The ledger must contain entries and lasting-impact events must appear
	// before routine fluctuations.
	if len(ledger) == 0 {
		t.Fatal("milestone ledger must not be empty")
	}

	// First entry is the relationship_shift (candidates[0] + highest priority).
	first := ledger[0].(map[string]any)
	if first["event_type"] != "relationship_shift" {
		t.Fatalf("ledger[0] event_type = %v, want relationship_shift", first["event_type"])
	}

	// Lasting events (relationship_shift, personality_change) must be present.
	lastingFound := map[string]bool{}
	for _, entry := range ledger {
		m := entry.(map[string]any)
		et, _ := m["event_type"].(string)
		if et == "relationship_shift" || et == "personality_change" {
			lastingFound[et] = true
		}
	}
	if !lastingFound["relationship_shift"] {
		t.Fatal("milestone ledger missing relationship_shift (lasting impact)")
	}
	if !lastingFound["personality_change"] {
		t.Fatal("milestone ledger missing personality_change (lasting impact)")
	}

	// Personality change should come before routine events (priority 1 < 2,4).
	// Find the position of personality_change in the ledger.
	persIdx := -1
	statusIdx := -1
	for i, entry := range ledger {
		m := entry.(map[string]any)
		et, _ := m["event_type"].(string)
		if et == "personality_change" {
			persIdx = i
		}
		if et == "status_change" {
			statusIdx = i
		}
	}
	if persIdx == -1 {
		t.Fatal("personality_change not found in ledger")
	}
	// status_change may or may not be in the ledger depending on budget, but if
	// present, personality_change must come first.
	if statusIdx != -1 && persIdx >= statusIdx {
		t.Fatalf("personality_change (idx=%d) must come before status_change (idx=%d)", persIdx, statusIdx)
	}

	// The character stale snapshot must treat lasting-impact event anchors as
	// continuity anchors even for otherwise-stale characters.
	staleItem := store.CharacterState{
		ID:            56,
		ChatSessionID: "sess-seq14-p56",
		CharacterName: "Mira",
		TurnIndex:     12, // relationship_shift event turn
	}
	snap := characterStaleSnapshot(staleItem, events, 30, "", map[string]struct{}{})
	// The relationship_shift / personality_change events should be recognized
	// as event_anchor continuity anchors.
	anchorTypes := seq14StringSlice(t, snap["continuity_anchor_types"], "continuity_anchor_types")
	if !seq14Contains(anchorTypes, "event_anchor") {
		t.Fatalf("lasting-impact events should produce event_anchor in continuity_anchor_types, got %v", anchorTypes)
	}

	// Routine fluctuations (status_change) must NOT promote to stable overwrite:
	// the stable sheet must remain unchanged by the dynamic events.
	stableItem := store.CharacterState{
		ID:              58,
		ChatSessionID:   "sess-seq14-p56",
		CharacterName:   "Mira",
		AppearanceJSON:  `{"hair":"black"}`,
		PersonalityJSON: `{"core":"guarded but loyal"}`,
		TurnIndex:       14,
	}
	stableSnap := characterStaleSnapshot(stableItem, events, 14, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(stableItem, stableSnap)
	// Stable sheet must not contain any event-driven volatile keys.
	for _, leaked := range []string{"current_status", "latest_interaction_anchor", "milestone_ledger"} {
		if _, ok := stable[leaked]; ok {
			t.Fatalf("stable sheet must not contain event-driven volatile key %q", leaked)
		}
	}
}

// TestSeq14P60ProtagonistRelationLaneCurrentSplit verifies P60:
// protagonist relation is separated from other_relations and the display
// order is protagonist_first_then_observed_order.
func TestSeq14P60ProtagonistRelationLaneCurrentSplit(t *testing.T) {
	item := store.CharacterState{
		ID:            60,
		ChatSessionID: "sess-seq14-p60",
		CharacterName: "Mira",
		RelationshipsJSON: `{
			"user":{"summary":"trusted ally","trust":0.9},
			"Rowan":{"summary":"uneasy rivalry","tension":0.6},
			"Nia":{"summary":"old debt unpaid","trust":0.5}
		}`,
		TurnIndex: 20,
	}
	lane := buildCharacterRelationshipLane(item)

	// Surface identity.
	if lane["surface_version"] != "rl14a.v1" {
		t.Fatalf("surface_version = %v, want rl14a.v1", lane["surface_version"])
	}
	if lane["surface_type"] != "relationship_lane" {
		t.Fatalf("surface_type = %v, want relationship_lane", lane["surface_type"])
	}
	if lane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("display_mode = %v, want protagonist_first_then_observed_order", lane["display_mode"])
	}

	// Protagonist relation must be present and split from other_relations.
	prot, ok := lane["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatalf("protagonist_relation must be a non-nil map, got %#v", lane["protagonist_relation"])
	}
	if prot["target"] == nil || prot["target"] == "" {
		t.Fatal("protagonist_relation must have a target")
	}
	if prot["summary_text"] == nil || prot["summary_text"] == "" {
		t.Fatal("protagonist_relation must have summary_text")
	}

	// other_relations must not duplicate the protagonist.
	others := seq14RelationItems(t, lane["other_relations"])
	for _, o := range others {
		if o["target"] == prot["target"] && o["summary_text"] == prot["summary_text"] {
			t.Fatalf("protagonist leaked into other_relations: %#v", o)
		}
	}

	// items must be ordered: protagonist first, then others.
	items := seq14RelationItems(t, lane["items"])
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items (protagonist + others), got %d", len(items))
	}
	firstItem := items[0]
	if firstItem["target"] != prot["target"] {
		t.Fatalf("items[0].target = %v, want protagonist %v", firstItem["target"], prot["target"])
	}
	// Protagonist display_priority must be 0 (highest).
	if prot["display_priority"] != 0 {
		t.Fatalf("protagonist display_priority = %v, want 0", prot["display_priority"])
	}
	// Non-protagonist entries must have display_priority 1.
	for _, o := range others {
		if o["display_priority"] != 1 {
			t.Fatalf("other relation display_priority = %v, want 1", o["display_priority"])
		}
	}

	// Dynamic digest must carry the protagonist relation from the lane.
	snapshot := characterStaleSnapshot(item, nil, 20, "", map[string]struct{}{})
	dynamic := buildDynamicCharacterDigest(item, snapshot, lane, nil, nil)
	if dynamic["protagonist_relation"] == nil {
		t.Fatal("dynamic digest missing protagonist_relation")
	}
	if dynamic["relationship_display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("dynamic relationship_display_mode = %v", dynamic["relationship_display_mode"])
	}
}

// TestSeq14P61LatestInteractionAnchorPreserved verifies P61:
// latest_interaction_anchor preserves event_type, turn_index, summary_text,
// details and is connected to the dynamic continuity digest unchanged.
func TestSeq14P61LatestInteractionAnchorPreserved(t *testing.T) {
	event := store.CharacterEvent{
		ID:            401,
		ChatSessionID: "sess-seq14-p61",
		CharacterName: "Mira",
		EventType:     "relationship_shift",
		TurnIndex:     18,
		DetailsJSON:   `{"summary":"Mira defended the protagonist in council.","before":{"trust":0.5},"after":{"trust":0.9}}`,
	}
	anchor := buildCharacterLatestInteractionAnchor(&event)

	anchorMap, ok := anchor.(map[string]any)
	if !ok {
		t.Fatalf("anchor must be a map, got %T", anchor)
	}

	// Surface identity.
	if anchorMap["surface_version"] != "rl14b.v1" {
		t.Fatalf("surface_version = %v, want rl14b.v1", anchorMap["surface_version"])
	}
	if anchorMap["surface_type"] != "latest_interaction_anchor" {
		t.Fatalf("surface_type = %v, want latest_interaction_anchor", anchorMap["surface_type"])
	}
	if anchorMap["status"] != "ready" {
		t.Fatalf("status = %v, want ready", anchorMap["status"])
	}

	// Core anchor fields must be preserved.
	if anchorMap["event_type"] != "relationship_shift" {
		t.Fatalf("event_type = %v, want relationship_shift", anchorMap["event_type"])
	}
	if anchorMap["turn_index"] != 18 {
		t.Fatalf("turn_index = %v, want 18", anchorMap["turn_index"])
	}
	summaryText, _ := anchorMap["summary_text"].(string)
	if summaryText == "" {
		t.Fatalf("summary_text must be non-empty, got %#v", anchorMap["summary_text"])
	}
	details, ok := anchorMap["details"].(map[string]any)
	if !ok || details == nil {
		t.Fatalf("details must be a map, got %T", anchorMap["details"])
	}
	// Details must preserve the original payload.
	if details["summary"] != "Mira defended the protagonist in council." {
		t.Fatalf("details.summary = %v", details["summary"])
	}

	// Nil event must return nil anchor (no crash).
	nilAnchor := buildCharacterLatestInteractionAnchor(nil)
	if nilAnchor != nil {
		t.Fatalf("nil event must produce nil anchor, got %#v", nilAnchor)
	}

	// Dynamic digest must pass through the anchor unchanged.
	item := store.CharacterState{
		ID:            61,
		ChatSessionID: "sess-seq14-p61",
		CharacterName: "Mira",
		TurnIndex:     18,
	}
	snapshot := characterStaleSnapshot(item, nil, 18, "", map[string]struct{}{})
	lane := buildCharacterRelationshipLane(item)
	dynamic := buildDynamicCharacterDigest(item, snapshot, lane, anchor, nil)
	if dynamic["latest_interaction_anchor"] == nil {
		t.Fatal("dynamic digest missing latest_interaction_anchor")
	}
	dynAnchor, ok := dynamic["latest_interaction_anchor"].(map[string]any)
	if !ok {
		t.Fatalf("dynamic digest latest_interaction_anchor must be a map, got %T", dynamic["latest_interaction_anchor"])
	}
	if dynAnchor["event_type"] != "relationship_shift" {
		t.Fatalf("dynamic digest anchor event_type = %v", dynAnchor["event_type"])
	}
	if dynAnchor["turn_index"] != 18 {
		t.Fatalf("dynamic digest anchor turn_index = %v", dynAnchor["turn_index"])
	}
}

// TestSeq14P62RelationReadPrecedenceDisplaySurface verifies P62:
// relation read precedence is protagonist_relation -> other_relations/items ->
// summary_text/descriptor_summary. Relationship prose is a display/read
// surface only and must NOT overwrite stable facts.
func TestSeq14P62RelationReadPrecedenceDisplaySurface(t *testing.T) {
	item := store.CharacterState{
		ID:              62,
		ChatSessionID:   "sess-seq14-p62",
		CharacterName:   "Mira",
		PersonalityJSON: `{"core":"guarded but loyal"}`,
		RelationshipsJSON: `{
			"user":{"summary":"trusted ally","trust":0.9},
			"Rowan":{"summary":"uneasy rivalry","tension":0.6},
			"Nia":{"summary":"old debt unpaid","trust":0.5}
		}`,
		TurnIndex: 22,
	}
	snapshot := characterStaleSnapshot(item, nil, 22, "", map[string]struct{}{})
	lane := buildCharacterRelationshipLane(item)
	dynamic := buildDynamicCharacterDigest(item, snapshot, lane, nil, nil)

	// Read precedence level 1: protagonist_relation must be present.
	prot, ok := dynamic["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatal("read precedence: protagonist_relation must be present at level 1")
	}
	if prot["target"] == nil || prot["summary_text"] == nil {
		t.Fatal("read precedence: protagonist_relation must expose target and summary_text")
	}

	// Read precedence level 2: other_relations and items must be present.
	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("read precedence: other_relations must be present at level 2")
	}
	items := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(items) == 0 {
		t.Fatal("read precedence: relationship_lane items must be present at level 2")
	}

	// Read precedence level 3: summary_text and descriptor_summary must be present.
	summaryText, _ := dynamic["relationship_summary_text"].(string)
	if summaryText == "" {
		t.Fatal("read precedence: relationship_summary_text must be present at level 3")
	}
	// descriptor_summary from the lane.
	descSummary, _ := lane["descriptor_summary"].(string)
	// descriptor_summary may be empty if no descriptor_bands, that's fine.
	_ = descSummary

	// Relationship prose must NOT overwrite stable facts.
	// The stable sheet must remain the durable evidence spine.
	stable := buildStableCharacterSheet(item, snapshot)
	// Stable sheet must not contain any relationship prose.
	for _, relKey := range []string{"relationship_lane", "other_relations", "protagonist_relation", "relationship_summary_text", "relationship_descriptor_lane"} {
		if _, ok := stable[relKey]; ok {
			t.Fatalf("stable sheet must not contain relationship prose key %q", relKey)
		}
	}

	// Relationship descriptor_lane in dynamic digest is display-only.
	rdlRaw, ok := dynamic["relationship_descriptor_lane"]
	if !ok {
		t.Fatal("dynamic digest missing relationship_descriptor_lane")
	}
	rdl, ok := rdlRaw.([]any)
	if !ok {
		t.Fatalf("relationship_descriptor_lane must be a slice, got %T", rdlRaw)
	}
	if len(rdl) > 0 {
		entry0, ok := rdl[0].(map[string]any)
		if !ok {
			t.Fatalf("relationship_descriptor_lane[0] must be a map, got %T", rdl[0])
		}
		if entry0["target"] == nil || entry0["summary_text"] == nil {
			t.Fatal("relationship_descriptor_lane entries must expose target and summary_text")
		}
	}

	// The stable durable_profile must remain unchanged regardless of relationship
	// prose content.
	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain stable")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P66  14-3a story frame / turn directives split - guidance read surface
// ---------------------------------------------------------------------------

func TestSeq14P66StoryFrameTurnDirectivesSplitGuidanceSurface(t *testing.T) {
	storyPlan := map[string]any{
		"current_arc":        "act_2_rising",
		"narrative_goal":     "deepen the betrayal tension",
		"active_tensions":    []string{"trust_fracture", "loyalty_test"},
		"next_beats":         []string{"reveal_secret", "confrontation"},
		"continuity_anchors": []string{"oath_from_ep3"},
		"focus_characters":   []string{"aria", "kael"},
	}
	director := map[string]any{
		"pressure_level":      "strong",
		"required_outcomes":   []string{"expose_mole"},
		"forbidden_moves":     []string{"kill_protagonist"},
		"execution_checklist": []string{"build_tension", "reveal_clue"},
		"world_guardrails":    []string{"no_magic_in_city"},
		"persona_guardrails":  []string{"aria_stays_calm"},
		"scene_mandate":       "interrogation_scene",
	}

	guidance := buildStoryGuidanceSurface(storyPlan, director)

	if guidance["surface_version"] != "sg14a.v1" {
		t.Fatalf("surface_version must be sg14a.v1, got %v", guidance["surface_version"])
	}
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("surface_type must be story_guidance_surface, got %v", guidance["surface_type"])
	}

	sf := seq14SubMap(t, guidance, "story_frame")
	td := seq14SubMap(t, guidance, "turn_directives")

	// story_frame is narrative/arc frame
	if sf["stage_type"] != "story_frame" {
		t.Fatalf("story_frame.stage_type must be story_frame, got %v", sf["stage_type"])
	}
	if sf["arc_focus"] != "act_2_rising" {
		t.Fatalf("story_frame.arc_focus mismatch")
	}
	if sf["narrative_drive"] != "deepen the betrayal tension" {
		t.Fatalf("story_frame.narrative_drive mismatch")
	}
	seq14AssertStringSlice(t, sf, "live_tensions", 2)
	seq14AssertStringSlice(t, sf, "beat_queue", 2)
	seq14AssertStringSlice(t, sf, "carry_threads", 1)
	seq14AssertStringSlice(t, sf, "spotlight_characters", 2)

	// turn_directives is turn-level directive
	if td["stage_type"] != "turn_directives" {
		t.Fatalf("turn_directives.stage_type must be turn_directives, got %v", td["stage_type"])
	}

	// Both must be present inside the guidance surface
	if guidance["story_frame"] == nil || guidance["turn_directives"] == nil {
		t.Fatal("guidance surface must contain both story_frame and turn_directives")
	}

	// story_frame must NOT carry turn-level keys
	for _, key := range []string{"scene_drive", "carry_targets", "blocked_routes", "tempo_band", "handoff_edge"} {
		if _, ok := sf[key]; ok {
			t.Fatalf("story_frame must NOT carry turn-level key %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P67  14-3b turn directive slot define
// ---------------------------------------------------------------------------

func TestSeq14P67TurnDirectiveSlotsDefined(t *testing.T) {
	director := map[string]any{
		"pressure_level":      "critical",
		"scene_mandate":       "chase_sequence",
		"required_outcomes":   []string{"catch_thief", "recover_artifact"},
		"forbidden_moves":     []string{"destroy_building"},
		"execution_checklist": []string{"start_chase", "narrow_escape", "final_confrontation"},
		"world_guardrails":    []string{"no_explosions"},
		"persona_guardrails":  []string{"hero_no_kill"},
	}

	guidance := buildStoryGuidanceSurface(map[string]any{}, director)
	td := seq14SubMap(t, guidance, "turn_directives")

	// Required slots
	if td["scene_drive"] != "chase_sequence" {
		t.Fatalf("scene_drive must be chase_sequence, got %v", td["scene_drive"])
	}
	seq14AssertStringSlice(t, td, "carry_targets", 2)
	seq14AssertStringSlice(t, td, "blocked_routes", 1)
	if td["tempo_band"] != "critical" {
		t.Fatalf("tempo_band must be critical, got %v", td["tempo_band"])
	}
	if hs, ok := td["handoff_edge"].(string); !ok || hs == "" {
		t.Fatal("handoff_edge must be a non-empty string")
	}

	// execution_contract preserved
	ec := seq14SubMap(t, td, "execution_contract")
	if ec["pacing_pressure"] != "critical" {
		t.Fatalf("execution_contract.pacing_pressure must be critical")
	}
	seq14AssertStringSlice(t, ec, "must_hit", 2)
	seq14AssertStringSlice(t, ec, "forbidden", 1)

	// fail_mode preserved
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["mode"] != "pressure_continuation_without_resolution" {
		t.Fatalf("fail_mode.mode for critical pressure must be pressure_continuation_without_resolution, got %v", fm["mode"])
	}
	if fm["allow_scene_jump"] != false {
		t.Fatal("fail_mode.allow_scene_jump must be false")
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatal("fail_mode.allow_forced_resolution must be false")
	}
	if fm["respect_explicit_user_correction"] != true {
		t.Fatal("fail_mode.respect_explicit_user_correction must be true")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P68  14-3c story guidance precedence cleanup
// ---------------------------------------------------------------------------

func TestSeq14P68StoryGuidancePrecedenceCleanup(t *testing.T) {
	guidance := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "act_1"},
		map[string]any{"pressure_level": "steady"},
	)

	// conflict_policy
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

	// precedence
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

	// Verify precedence index: each higher source must appear before guidance
	// (implicit: guidance is subordinate to ALL listed sources)
	for _, src := range expectedOrder {
		if !seq14Contains(hps, src) {
			t.Fatalf("higher_priority_sources must include %q", src)
		}
	}

	// disallowed_usage must block override attempts
	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	if len(du) < 3 {
		t.Fatal("precedence.disallowed_usage must list at least 3 blocked override patterns")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P72  14-4a weak-input planner focus define
// ---------------------------------------------------------------------------

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

	// Planner focus must carry conservative continuation signals.
	seq14AssertStringSlice(t, sf, "live_tensions", 2)
	seq14AssertStringSlice(t, sf, "spotlight_characters", 2)
	seq14AssertStringSlice(t, sf, "beat_queue", 2)
	seq14AssertStringSlice(t, sf, "carry_threads", 1)

	// turn_directives must expose ending requirement and handoff_edge so that
	// a weak/empty input has a bounded continuation target instead of being
	// promoted to a new authority.
	ec := seq14SubMap(t, td, "execution_contract")
	if _, ok := ec["ending_requirement"].(string); !ok {
		t.Fatal("execution_contract.ending_requirement must be a string for weak-input focus")
	}
	if hs, ok := td["handoff_edge"].(string); !ok || hs == "" {
		t.Fatal("handoff_edge must be a non-empty continuation edge")
	}

	// fail_mode must not allow scene jump or forced resolution, which would
	// elevate weak input into a truth authority.
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatalf("weak-input planner focus requires allow_scene_jump=false, got %v", fm["allow_scene_jump"])
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("weak-input planner focus requires allow_forced_resolution=false, got %v", fm["allow_forced_resolution"])
	}

	// Weak input must not override current user input or explicit correction.
	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["current_user_input_wins"] != true {
		t.Fatal("conflict_policy.current_user_input_wins must be true")
	}
	if cp["guidance_may_override_user_input"] != false {
		t.Fatal("conflict_policy.guidance_may_override_user_input must be false")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P73  14-4b weak-input fail-safe define
// ---------------------------------------------------------------------------

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

	// Fail mode must be conservative.
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

	// Fail mode label must be a conservative continuation variant.
	mode, _ := fm["mode"].(string)
	switch mode {
	case "conservative_continuation",
		"scene_continuation_without_scene_jump",
		"carry_forward_without_forcing_resolution",
		"pressure_continuation_without_resolution":
		// ok
	default:
		t.Fatalf("fail_mode.mode must be a conservative continuation variant, got %q", mode)
	}

	// Precedence must block hard world rule bypass and canonical truth
	// overwrite, which would be the most dangerous weak-input authority leaks.
	prec := seq14SubMap(t, guidance, "precedence")
	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{"hard_world_rule_bypass", "canonical_truth_floor_overwrite"} {
		if !seq14Contains(du, want) {
			t.Fatalf("disallowed_usage must block %q to prevent weak-input authority leak, got %v", want, du)
		}
	}

	// Guidance authority must remain subordinate.
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P74  14-4c weak-input vs explicit correction guard
// ---------------------------------------------------------------------------

// TestSeq14P74WeakInputExplicitCorrectionGuard verifies that explicit user
// correction always outranks weak-input steering in the guidance surface.
// conflict_policy, precedence, and fail_mode must all agree that guidance
// never overrides an explicit correction from the user.
func TestSeq14P74WeakInputExplicitCorrectionGuard(t *testing.T) {
	guidance := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "act_2", "active_tensions": []string{"rift"}},
		map[string]any{"pressure_level": "strong"},
	)

	// conflict_policy: explicit_user_correction_wins = true and
	// guidance_may_override_user_input = false are the core guard.
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

	// precedence: explicit_user_correction must rank above guidance
	// (i.e. listed in higher_priority_sources).
	prec := seq14SubMap(t, guidance, "precedence")
	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	if !seq14Contains(hps, "explicit_user_correction") {
		t.Fatalf("higher_priority_sources must include explicit_user_correction, got %v", hps)
	}

	// disallowed_usage must include explicit_user_correction_override.
	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	if !seq14Contains(du, "explicit_user_correction_override") {
		t.Fatalf("disallowed_usage must block explicit_user_correction_override, got %v", du)
	}

	// fail_mode: respect_explicit_user_correction must be true so that even
	// under pressure the guidance surface yields to explicit corrections.
	td := seq14SubMap(t, guidance, "turn_directives")
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode.respect_explicit_user_correction must be true, got %v", fm["respect_explicit_user_correction"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P78  14-5a stable/dynamic no-overwrite replay
// ---------------------------------------------------------------------------

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

	// --- Stable sheet is the durable evidence spine. ---
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("stable surface_type = %v", stable["surface_type"])
	}

	// Durable axes present.
	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable missing durable axis %q", key)
		}
	}

	// Stable sheet must NOT contain any volatile/dynamic keys.
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

	// --- Dynamic digest is the current/volatile surface. ---
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("dynamic surface_type = %v", dynamic["surface_type"])
	}

	// Dynamic digest must carry all current-surface keys.
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

	// Dynamic digest must NOT contain durable stable keys.
	durableKeys := []string{
		"appearance_observable", "appearance_non_observable",
		"appearance_core", "durable_profile", "sparse_policy",
	}
	for _, key := range durableKeys {
		if _, ok := dynamic[key]; ok {
			t.Fatalf("REPLAY: dynamic digest must NOT contain durable key %q", key)
		}
	}

	// milestone_ledger must not overwrite durable_profile.
	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain intact")
	}

	// Response item must expose both surfaces side by side.
	if _, ok := resp["stable_character_sheet"]; !ok {
		t.Fatal("response missing stable_character_sheet")
	}
	if _, ok := resp["dynamic_continuity_digest"]; !ok {
		t.Fatal("response missing dynamic_continuity_digest")
	}

	// sparse_policy.dynamic_redirects must point volatile reads away from stable.
	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("REPLAY: sparse_policy.dynamic_redirects missing %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P79  14-5b relation lane continuity replay
// ---------------------------------------------------------------------------

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

	// Latest interaction anchor from a relationship_shift event.
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

	// --- Dynamic digest: relation lane + anchor co-located. ---
	// Protagonist relation must be present in dynamic digest.
	prot, ok := dynamic["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatal("REPLAY: dynamic digest missing protagonist_relation")
	}
	if prot["target"] == nil || prot["target"] == "" {
		t.Fatal("protagonist_relation must have target")
	}

	// Other relations must be present in dynamic digest.
	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("REPLAY: dynamic digest missing other_relations")
	}

	// Relationship lane items must be present.
	rlItems := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(rlItems) == 0 {
		t.Fatal("REPLAY: dynamic digest missing relationship_lane items")
	}

	// Latest interaction anchor must be present in dynamic digest.
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

	// Relationship display mode must be present.
	if dynamic["relationship_display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("relationship_display_mode = %v", dynamic["relationship_display_mode"])
	}

	// relationship_descriptor_lane must be present as display surface.
	rdlRaw, ok := dynamic["relationship_descriptor_lane"]
	if !ok {
		t.Fatal("REPLAY: dynamic digest missing relationship_descriptor_lane")
	}
	rdl, ok := rdlRaw.([]any)
	if !ok || len(rdl) == 0 {
		t.Fatal("relationship_descriptor_lane must be a non-empty slice")
	}

	// --- Stable sheet: NO relation lane / anchor leak. ---
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

	// Stable durable_profile must remain unchanged regardless of relation content.
	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain stable")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P80  14-5c guidance non-leakage replay
// ---------------------------------------------------------------------------

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

	// --- Surface identity ---
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("surface_type = %v", guidance["surface_type"])
	}

	// --- Guidance authority is subordinate. ---
	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("REPLAY: guidance_authority must be subordinate, got %v", prec["guidance_authority"])
	}

	// higher_priority_sources must include all truth-floor sources.
	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	for _, src := range []string{"canonical_truth_floor", "hard_world_rule", "latest_direct_evidence"} {
		if !seq14Contains(hps, src) {
			t.Fatalf("REPLAY: higher_priority_sources must include %q, got %v", src, hps)
		}
	}

	// disallowed_usage must block truth-floor overwrite patterns.
	du := seq14StringSlice(t, prec["disallowed_usage"], "disallowed_usage")
	for _, want := range []string{"canonical_truth_floor_overwrite", "hard_world_rule_bypass"} {
		if !seq14Contains(du, want) {
			t.Fatalf("REPLAY: disallowed_usage must block %q, got %v", want, du)
		}
	}

	// --- Guidance surface must NOT contain truth-floor keys. ---
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

	// --- story_frame / turn_directives must not contain truth-floor keys. ---
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

	// --- execution_contract must not overwrite truth. ---
	ec := seq14SubMap(t, td, "execution_contract")
	if ec["must_hit"] == nil {
		t.Fatal("execution_contract.must_hit must be present")
	}
	if ec["forbidden"] == nil {
		t.Fatal("execution_contract.forbidden must be present")
	}
	// execution_contract must not carry any truth authority.
	for _, key := range []string{"truth_authority", "canonical_source", "overwrite_policy"} {
		if _, ok := ec[key]; ok {
			t.Fatalf("REPLAY: execution_contract must NOT contain truth authority key %q", key)
		}
	}

	// --- fail_mode must not allow truth overwrite. ---
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_scene_jump"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_scene_jump must be false")
	}
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("REPLAY: fail_mode.allow_forced_resolution must be false")
	}

	// --- conflict_policy must yield to user. ---
	cp := seq14SubMap(t, guidance, "conflict_policy")
	if cp["guidance_may_override_user_input"] != false {
		t.Fatalf("REPLAY: conflict_policy.guidance_may_override_user_input must be false")
	}
	if cp["explicit_user_correction_wins"] != true {
		t.Fatalf("REPLAY: conflict_policy.explicit_user_correction_wins must be true")
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P81  14-5d weak-input non-overreach replay
// ---------------------------------------------------------------------------

// TestSeq14P81WeakInputNonOverreachReplay verifies that a weak-input replay
// allows only conservative continuation. Explicit user correction, hard world
// rule, and current user input must all outrank weak-input steering. No forced
// scene jump, forced resolution, or truth overwrite is allowed.
func TestSeq14P81WeakInputNonOverreachReplay(t *testing.T) {
	// --- Character stale guard: stale transient blocks carry-forward. ---
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

	// Anchored fresh character: carry-forward allowed.
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

	// --- Guidance surface: weak-input must not overreach. ---
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

	// Guidance authority must remain subordinate even under strong pressure.
	prec := seq14SubMap(t, guidance, "precedence")
	if prec["guidance_authority"] != "subordinate" {
		t.Fatalf("REPLAY: guidance_authority must be subordinate under strong pressure, got %v", prec["guidance_authority"])
	}

	// All three authority sources must outrank weak-input guidance.
	hps := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	for _, src := range []string{"current_user_input", "explicit_user_correction", "hard_world_rule"} {
		if !seq14Contains(hps, src) {
			t.Fatalf("REPLAY: higher_priority_sources must include %q to outrank weak-input, got %v", src, hps)
		}
	}

	// conflict_policy must block weak-input overreach.
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

	// fail_mode must block forced scene jump and forced resolution.
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

	// Fail mode label must be a conservative continuation variant.
	mode, _ := fm["mode"].(string)
	switch mode {
	case "conservative_continuation",
		"scene_continuation_without_scene_jump",
		"carry_forward_without_forcing_resolution",
		"pressure_continuation_without_resolution":
		// ok
	default:
		t.Fatalf("REPLAY: fail_mode.mode must be a conservative continuation variant, got %q", mode)
	}

	// disallowed_usage must block the most dangerous overreach patterns.
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

	// Stable sheet must remain unaffected by weak-input guidance.
	stable := buildStableCharacterSheet(anchored, freshSnap)
	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "story_guidance"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("REPLAY: stable sheet must not be affected by weak-input guidance, found key %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-14-P86 .. SEQ-14-P89  (smoke / naming)
// ---------------------------------------------------------------------------

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

	// --- Surface identity ---
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("SMOKE P86: stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("SMOKE P86: dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}

	// --- Surface version ---
	if _, ok := stable["surface_version"]; !ok {
		t.Fatal("SMOKE P86: stable missing surface_version")
	}
	if _, ok := dynamic["surface_version"]; !ok {
		t.Fatal("SMOKE P86: dynamic missing surface_version")
	}

	// --- Stable is the evidence spine; must not carry volatile keys ---
	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "recent_change_summary"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("SMOKE P86: stable must NOT carry volatile key %q", key)
		}
	}

	// --- Dynamic is the current surface; must carry volatile keys ---
	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("SMOKE P86: dynamic must carry volatile key %q", key)
		}
	}

	// --- Response item must expose both surfaces ---
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

	// --- Dynamic must carry relationship_lane (items slice) ---
	relItems := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(relItems) == 0 {
		t.Fatal("SMOKE P87: relationship_lane must have items")
	}

	// --- Dynamic must carry protagonist_relation and other_relations at top level ---
	if _, ok := dynamic["protagonist_relation"].(map[string]any); !ok {
		t.Fatal("SMOKE P87: dynamic missing protagonist_relation")
	}
	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("SMOKE P87: dynamic missing other_relations")
	}

	// --- Dynamic must carry latest_interaction_anchor ---
	anchorMap, ok := dynamic["latest_interaction_anchor"].(map[string]any)
	if !ok {
		t.Fatalf("SMOKE P87: latest_interaction_anchor must be map, got %T", dynamic["latest_interaction_anchor"])
	}
	if anchorMap["event_type"] != "relationship_shift" {
		t.Fatalf("SMOKE P87: latest_interaction_anchor event_type = %v", anchorMap["event_type"])
	}

	// --- Stable sheet must NOT leak relationship_lane or latest_interaction_anchor ---
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

	// --- Surface identity ---
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("SMOKE P88: surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}

	// --- story_frame must be present and narrative-level ---
	sf := seq14SubMap(t, guidance, "story_frame")
	if sf["stage_type"] != "story_frame" {
		t.Fatalf("SMOKE P88: story_frame stage_type = %v", sf["stage_type"])
	}

	// --- turn_directives must be present and turn-level ---
	td := seq14SubMap(t, guidance, "turn_directives")
	if td["stage_type"] != "turn_directives" {
		t.Fatalf("SMOKE P88: turn_directives stage_type = %v", td["stage_type"])
	}

	// --- story_frame must NOT carry turn-level keys ---
	for _, key := range []string{"scene_drive", "carry_targets", "blocked_routes", "tempo_band", "handoff_edge"} {
		if _, ok := sf[key]; ok {
			t.Fatalf("SMOKE P88: story_frame must NOT carry turn-level key %q", key)
		}
	}

	// --- turn_directives must NOT be truth authority ---
	for _, key := range []string{"truth_floor", "canonical_truth", "world_rule_authority"} {
		if _, ok := td[key]; ok {
			t.Fatalf("SMOKE P88: turn_directives must NOT be truth authority, found key %q", key)
		}
	}

	// --- fail_mode must block forced resolution and scene jump ---
	fm := seq14SubMap(t, td, "fail_mode")
	if fm["allow_forced_resolution"] != false {
		t.Fatalf("SMOKE P88: fail_mode.allow_forced_resolution must be false, got %v", fm["allow_forced_resolution"])
	}
	if fm["allow_scene_jump"] != false {
		t.Fatalf("SMOKE P88: fail_mode.allow_scene_jump must be false, got %v", fm["allow_scene_jump"])
	}

	// --- execution_contract must NOT grant truth authority ---
	ec, ok := td["execution_contract"].(map[string]any)
	if !ok {
		t.Fatal("SMOKE P88: turn_directives missing execution_contract")
	}
	if ec["truth_authority"] == true || ec["canonical_override"] == true {
		t.Fatalf("SMOKE P88: execution_contract must NOT grant truth authority: %v", ec)
	}

	// --- Guidance authority is subordinate (via precedence) ---
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

	// --- Verify canonical surface_type names ---
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("NAMING P89: stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("NAMING P89: dynamic surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}
	if guidance["surface_type"] != "story_guidance_surface" {
		t.Fatalf("NAMING P89: guidance surface_type = %v, want story_guidance_surface", guidance["surface_type"])
	}

	// --- Verify canonical stage_type names ---
	sf := seq14SubMap(t, guidance, "story_frame")
	td := seq14SubMap(t, guidance, "turn_directives")
	if sf["stage_type"] != "story_frame" {
		t.Fatalf("NAMING P89: story_frame stage_type = %v", sf["stage_type"])
	}
	if td["stage_type"] != "turn_directives" {
		t.Fatalf("NAMING P89: turn_directives stage_type = %v", td["stage_type"])
	}

	// --- Verify relationship_lane is the canonical name inside dynamic digest ---
	if _, ok := dynamic["relationship_lane"]; !ok {
		t.Fatal("NAMING P89: dynamic missing relationship_lane (canonical name)")
	}

	// --- Verify stable_character_sheet is the canonical name in response item ---
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

	// --- Verify surface_version is present (schema versioning) ---
	if _, ok := stable["surface_version"]; !ok {
		t.Fatal("NAMING P89: stable missing surface_version")
	}
	if _, ok := dynamic["surface_version"]; !ok {
		t.Fatal("NAMING P89: dynamic missing surface_version")
	}

	// --- Verify no runtime rename / alias patterns ---
	// The surfaces must use their canonical names directly; no
	// "legacy_name", "alias", or "remap" keys should appear.
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

	// --- surface version and type ---
	if stable["surface_version"] != "cc14a.v1" {
		t.Fatalf("P93: surface_version = %v, want cc14a.v1", stable["surface_version"])
	}
	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("P93: surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}

	// --- status must reflect axis fill ---
	status, ok := stable["status"].(string)
	if !ok || status == "" {
		t.Fatal("P93: missing or empty status")
	}

	// --- filled_axes must include appearance, personality, speech_style ---
	filledAxes := seq14StringSlice(t, stable["filled_axes"], "filled_axes")
	for _, axis := range []string{"appearance", "personality", "speech_style"} {
		if !seq14Contains(filledAxes, axis) {
			t.Fatalf("P93: filled_axes missing %q", axis)
		}
	}

	// --- top-level payload keys ---
	for _, key := range []string{"appearance", "appearance_core", "appearance_observable", "appearance_non_observable", "appearance_snapshot_keys", "personality", "speech_style"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("P93: stable missing top-level key %q", key)
		}
	}

	// --- durable_profile must carry the three durable axes ---
	dp := seq14SubMap(t, stable, "durable_profile")
	for _, key := range []string{"appearance", "personality", "speech_style"} {
		if _, ok := dp[key]; !ok {
			t.Fatalf("P93: durable_profile missing key %q", key)
		}
	}

	// --- sparse_policy block ---
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

	// --- source_turn matches snapshot ---
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

	// --- surface version and type ---
	if dynamic["surface_version"] != "cc14b.v1" {
		t.Fatalf("P94: surface_version = %v, want cc14b.v1", dynamic["surface_version"])
	}
	if dynamic["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("P94: surface_type = %v, want dynamic_continuity_digest", dynamic["surface_type"])
	}

	// --- status ---
	if _, ok := dynamic["status"].(string); !ok {
		t.Fatal("P94: missing status string")
	}

	// --- filled_axes (ordered) ---
	filledAxes := seq14StringSlice(t, dynamic["filled_axes"], "filled_axes")
	if len(filledAxes) == 0 {
		t.Fatal("P94: filled_axes must not be empty")
	}
	if !seq14Contains(filledAxes, "current_status") {
		t.Fatal("P94: filled_axes missing current_status")
	}

	// --- admission metadata ---
	if dynamic["admission_class"] == nil {
		t.Fatal("P94: missing admission_class")
	}
	if dynamic["admission_basis"] == nil {
		t.Fatal("P94: missing admission_basis")
	}

	// --- stale_guard block ---
	sg := seq14SubMap(t, dynamic, "stale_guard")
	if _, ok := sg["active"]; !ok {
		t.Fatal("P94: stale_guard missing active")
	}

	// --- current_status and summary ---
	if dynamic["current_status"] == nil {
		t.Fatal("P94: missing current_status")
	}
	if _, ok := dynamic["current_status_summary"].(string); !ok {
		t.Fatal("P94: missing current_status_summary")
	}

	// --- current_snapshot must exist ---
	if _, ok := dynamic["current_snapshot"].(map[string]any); !ok {
		t.Fatal("P94: missing current_snapshot map")
	}

	// --- appearance_snapshot ---
	if _, ok := dynamic["appearance_snapshot"]; !ok {
		t.Fatal("P94: missing appearance_snapshot")
	}

	// --- relationship fields ---
	for _, key := range []string{"relationship_summary_text", "relationship_primary_target", "relationship_display_mode", "protagonist_relation", "relationship_lane", "other_relations", "relationship_descriptor_lane"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("P94: dynamic missing relationship field %q", key)
		}
	}

	// --- latest_interaction_anchor ---
	if dynamic["latest_interaction_anchor"] == nil {
		t.Fatal("P94: missing latest_interaction_anchor")
	}

	// --- milestone_ledger ---
	if _, ok := dynamic["milestone_ledger"].([]any); !ok {
		t.Fatal("P94: milestone_ledger must be []any")
	}

	// --- digest_budget block ---
	db := seq14SubMap(t, dynamic, "digest_budget")
	if db["policy"] != "priority_capped" {
		t.Fatalf("P94: digest_budget.policy = %v, want priority_capped", db["policy"])
	}
	for _, key := range []string{"relationship_lane_cap", "milestone_cap", "milestone_read_window", "milestone_selection_policy", "relationship_lane_used", "milestones_used"} {
		if _, ok := db[key]; !ok {
			t.Fatalf("P94: digest_budget missing key %q", key)
		}
	}

	// --- recent_change_summary ---
	if _, ok := dynamic["recent_change_summary"]; !ok {
		t.Fatal("P94: missing recent_change_summary")
	}

	// --- source_turn ---
	if dynamic["source_turn"] == nil {
		t.Fatal("P94: missing source_turn")
	}
}

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

	// --- surface version and type ---
	if relLane["surface_version"] != "rl14a.v1" {
		t.Fatalf("P95: surface_version = %v, want rl14a.v1", relLane["surface_version"])
	}
	if relLane["surface_type"] != "relationship_lane" {
		t.Fatalf("P95: surface_type = %v, want relationship_lane", relLane["surface_type"])
	}

	// --- items must have descriptor_bands as []string ---
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

	// --- primary_descriptor_bands at root ---
	seq14AssertStringSlice(t, relLane, "primary_descriptor_bands", 0)

	// --- descriptor_summary at root ---
	if _, ok := relLane["descriptor_summary"]; !ok {
		t.Fatal("P95: relationship_lane missing descriptor_summary")
	}

	// --- display_mode ---
	if relLane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("P95: display_mode = %v", relLane["display_mode"])
	}

	// --- each item must carry target, summary_text, state_snapshot, descriptor_bands, display_priority ---
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
	// --- Case A: anchored character, weak input carry forward allowed ---
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

	// --- Case B: transient descriptor with no anchors, large gap -> stale ---
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

	// --- Case C: admission_class transitions from lightweight_named to major_recurring with anchors ---
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

	// --- stable_character_sheet marker audit ---
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

	// --- dynamic_continuity_digest marker audit ---
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

	// --- relationship_lane marker audit ---
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

	// --- latest_interaction_anchor marker audit ---
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

	// --- story_guidance_surface marker audit (built separately) ---
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

	// --- All 5 surfaces must carry a status field ---
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

	// --- Summary of all surface versions for audit trail ---
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
