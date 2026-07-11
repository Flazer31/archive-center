package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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

	if _, ok := stable["durable_profile"]; !ok {
		t.Fatal("stable sheet missing durable_profile")
	}
	if _, leaked := stable["current_status"]; leaked {
		t.Fatal("stable sheet must not expose current_status (volatile axis)")
	}

	if _, ok := dynamic["current_status"]; !ok {
		t.Fatal("dynamic digest missing current_status")
	}
	if _, ok := dynamic["relationship_lane"]; !ok {
		t.Fatal("dynamic digest missing relationship_lane")
	}
	if _, ok := dynamic["latest_interaction_anchor"]; !ok {
		t.Fatal("dynamic digest missing latest_interaction_anchor")
	}

	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("sparse_policy.dynamic_redirects missing %q (%v)", want, redirects)
		}
	}

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

	for _, o := range others {
		if o["target"] == prot["target"] && o["summary_text"] == prot["summary_text"] {
			t.Fatalf("protagonist relation leaked into other_relations: %#v", o)
		}
	}

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

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable sheet missing durable axis %q", key)
		}
	}

	obsVal, _ := stable["appearance_observable"].(string)
	nonObsVal, _ := stable["appearance_non_observable"].(string)
	if obsVal == nonObsVal && obsVal != "" {
		t.Fatalf("appearance_observable and appearance_non_observable must be split surfaces")
	}

	if _, ok := stable["appearance_core"]; !ok {
		t.Fatalf("stable sheet missing appearance_core fallback")
	}

	dp := seq14SubMap(t, stable, "durable_profile")
	if _, ok := dp["personality"]; !ok {
		t.Fatal("durable_profile missing personality")
	}
	if _, ok := dp["speech_style"]; !ok {
		t.Fatal("durable_profile missing speech_style")
	}

	sparse := seq14SubMap(t, stable, "sparse_policy")
	if _, ok := sparse["dynamic_redirects"]; !ok {
		t.Fatal("sparse_policy missing dynamic_redirects")
	}

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

	cs := seq14SubMap(t, dynamic, "current_snapshot")
	statusMap := seq14SubMap(t, cs, "status")
	for _, key := range []string{"location", "emotion", "goal"} {
		if _, ok := statusMap[key]; !ok {
			t.Fatalf("current_snapshot.status missing %q", key)
		}
	}

	ledger, ok := dynamic["milestone_ledger"].([]any)
	if !ok || len(ledger) == 0 {
		t.Fatal("milestone_ledger must contain at least one milestone candidate")
	}
	first := ledger[0].(map[string]any)
	if first["event_type"] != "relationship_shift" {
		t.Fatalf("milestone_ledger[0].event_type = %v, want relationship_shift", first["event_type"])
	}

	rcs, _ := dynamic["recent_change_summary"].(string)
	if rcs == "" {
		t.Fatalf("recent_change_summary must carry a non-empty summary string, got %#v", dynamic["recent_change_summary"])
	}

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

	entry0, ok := rdl[0].(map[string]any)
	if !ok {
		t.Fatalf("relationship_descriptor_lane[0] must be a map, got %T", rdl[0])
	}
	if entry0["target"] == nil || entry0["target"] == "" {
		t.Fatalf("relationship_descriptor_lane[0] missing target, got %#v", entry0)
	}

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

	volatileKeys := []string{"current_status", "relationship_lane", "latest_interaction_anchor", "current_snapshot", "milestone_ledger", "recent_change_summary"}
	for _, key := range volatileKeys {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable sheet must NOT directly own volatile key %q", key)
		}
	}

	for _, key := range []string{"current_status", "relationship_lane", "latest_interaction_anchor", "current_snapshot"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("dynamic digest missing current-surface key %q", key)
		}
	}

	sparse := seq14SubMap(t, stable, "sparse_policy")
	redirects := seq14StringSlice(t, sparse["dynamic_redirects"], "dynamic_redirects")
	for _, want := range []string{"current_status", "relationship_lane", "latest_interaction_anchor"} {
		if !seq14Contains(redirects, want) {
			t.Fatalf("sparse_policy.dynamic_redirects missing %q (%v)", want, redirects)
		}
	}

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

	if len(ledger) == 0 {
		t.Fatal("milestone ledger must not be empty")
	}

	first := ledger[0].(map[string]any)
	if first["event_type"] != "relationship_shift" {
		t.Fatalf("ledger[0] event_type = %v, want relationship_shift", first["event_type"])
	}

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

	if statusIdx != -1 && persIdx >= statusIdx {
		t.Fatalf("personality_change (idx=%d) must come before status_change (idx=%d)", persIdx, statusIdx)
	}

	staleItem := store.CharacterState{
		ID:            56,
		ChatSessionID: "sess-seq14-p56",
		CharacterName: "Mira",
		TurnIndex:     12,
	}
	snap := characterStaleSnapshot(staleItem, events, 30, "", map[string]struct{}{})

	anchorTypes := seq14StringSlice(t, snap["continuity_anchor_types"], "continuity_anchor_types")
	if !seq14Contains(anchorTypes, "event_anchor") {
		t.Fatalf("lasting-impact events should produce event_anchor in continuity_anchor_types, got %v", anchorTypes)
	}

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

	if lane["surface_version"] != "rl14a.v1" {
		t.Fatalf("surface_version = %v, want rl14a.v1", lane["surface_version"])
	}
	if lane["surface_type"] != "relationship_lane" {
		t.Fatalf("surface_type = %v, want relationship_lane", lane["surface_type"])
	}
	if lane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("display_mode = %v, want protagonist_first_then_observed_order", lane["display_mode"])
	}

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

	others := seq14RelationItems(t, lane["other_relations"])
	for _, o := range others {
		if o["target"] == prot["target"] && o["summary_text"] == prot["summary_text"] {
			t.Fatalf("protagonist leaked into other_relations: %#v", o)
		}
	}

	items := seq14RelationItems(t, lane["items"])
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items (protagonist + others), got %d", len(items))
	}
	firstItem := items[0]
	if firstItem["target"] != prot["target"] {
		t.Fatalf("items[0].target = %v, want protagonist %v", firstItem["target"], prot["target"])
	}

	if prot["display_priority"] != 0 {
		t.Fatalf("protagonist display_priority = %v, want 0", prot["display_priority"])
	}

	for _, o := range others {
		if o["display_priority"] != 1 {
			t.Fatalf("other relation display_priority = %v, want 1", o["display_priority"])
		}
	}

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

	if anchorMap["surface_version"] != "rl14b.v1" {
		t.Fatalf("surface_version = %v, want rl14b.v1", anchorMap["surface_version"])
	}
	if anchorMap["surface_type"] != "latest_interaction_anchor" {
		t.Fatalf("surface_type = %v, want latest_interaction_anchor", anchorMap["surface_type"])
	}
	if anchorMap["status"] != "ready" {
		t.Fatalf("status = %v, want ready", anchorMap["status"])
	}

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

	if details["summary"] != "Mira defended the protagonist in council." {
		t.Fatalf("details.summary = %v", details["summary"])
	}

	nilAnchor := buildCharacterLatestInteractionAnchor(nil)
	if nilAnchor != nil {
		t.Fatalf("nil event must produce nil anchor, got %#v", nilAnchor)
	}

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

	prot, ok := dynamic["protagonist_relation"].(map[string]any)
	if !ok || prot == nil {
		t.Fatal("read precedence: protagonist_relation must be present at level 1")
	}
	if prot["target"] == nil || prot["summary_text"] == nil {
		t.Fatal("read precedence: protagonist_relation must expose target and summary_text")
	}

	otherRels := seq14RelationItems(t, dynamic["other_relations"])
	if len(otherRels) == 0 {
		t.Fatal("read precedence: other_relations must be present at level 2")
	}
	items := seq14RelationItems(t, dynamic["relationship_lane"])
	if len(items) == 0 {
		t.Fatal("read precedence: relationship_lane items must be present at level 2")
	}

	summaryText, _ := dynamic["relationship_summary_text"].(string)
	if summaryText == "" {
		t.Fatal("read precedence: relationship_summary_text must be present at level 3")
	}

	descSummary, _ := lane["descriptor_summary"].(string)

	_ = descSummary

	stable := buildStableCharacterSheet(item, snapshot)

	for _, relKey := range []string{"relationship_lane", "other_relations", "protagonist_relation", "relationship_summary_text", "relationship_descriptor_lane"} {
		if _, ok := stable[relKey]; ok {
			t.Fatalf("stable sheet must not contain relationship prose key %q", relKey)
		}
	}

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

	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile.personality must remain stable")
	}
}

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

	if td["stage_type"] != "turn_directives" {
		t.Fatalf("turn_directives.stage_type must be turn_directives, got %v", td["stage_type"])
	}

	if guidance["story_frame"] == nil || guidance["turn_directives"] == nil {
		t.Fatal("guidance surface must contain both story_frame and turn_directives")
	}

	for _, key := range []string{"scene_drive", "carry_targets", "blocked_routes", "tempo_band", "handoff_edge"} {
		if _, ok := sf[key]; ok {
			t.Fatalf("story_frame must NOT carry turn-level key %q", key)
		}
	}
}

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

	ec := seq14SubMap(t, td, "execution_contract")
	if ec["pacing_pressure"] != "critical" {
		t.Fatalf("execution_contract.pacing_pressure must be critical")
	}
	seq14AssertStringSlice(t, ec, "must_hit", 2)
	seq14AssertStringSlice(t, ec, "forbidden", 1)

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
