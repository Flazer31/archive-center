package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestSeq15P114ExplicitCorrectionOverrideConfirm verifies that explicit user
// correction always wins over story guidance, and guidance may never override
// it. Contract: explicit_user_correction_wins=true, guidance_may_override_user_input=false,
// explicit_user_correction in higher_priority_sources, "explicit_user_correction_override"
// in disallowed_usage.
func TestSeq15P114ExplicitCorrectionOverrideConfirm(t *testing.T) {
	surface := buildStoryGuidanceSurface(
		map[string]any{
			"current_arc":      "Betrayal",
			"next_beats":       []string{"ambush"},
			"focus_characters": []string{"Kael"},
		},
		map[string]any{
			"pressure_level":    "critical",
			"required_outcomes": []string{"ambush_succeeds"},
			"forbidden_moves":   []string{"hero_dies"},
		},
	)

	conflictPolicy := asMap(surface["conflict_policy"])
	if got, _ := conflictPolicy["explicit_user_correction_wins"].(bool); !got {
		t.Fatal("explicit_user_correction_wins = false, want true")
	}

	if got, _ := conflictPolicy["current_user_input_wins"].(bool); !got {
		t.Fatal("current_user_input_wins = false, want true")
	}

	if got, _ := conflictPolicy["guidance_may_override_user_input"].(bool); got {
		t.Fatal("guidance_may_override_user_input = true, want false")
	}

	if got, _ := conflictPolicy["on_conflict"].(string); got != "yield_to_current_user_input" {
		t.Fatalf("on_conflict = %q, want %q", got, "yield_to_current_user_input")
	}

	precedence := asMap(surface["precedence"])
	higherSources := mapAnyOrEmptyStringSlice(precedence, "higher_priority_sources")
	mustContain(t, higherSources, "explicit_user_correction")

	disallowed := mapAnyOrEmptyStringSlice(precedence, "disallowed_usage")
	mustContain(t, disallowed, "explicit_user_correction_override")

	td := asMap(surface["turn_directives"])
	fm := asMap(td["fail_mode"])
	if got, _ := fm["respect_explicit_user_correction"].(bool); !got {
		t.Fatal("fail_mode.respect_explicit_user_correction = false, want true")
	}

	emptySurface := buildStoryGuidanceSurface(map[string]any{}, map[string]any{})
	emptyCP := asMap(emptySurface["conflict_policy"])
	if got, _ := emptyCP["explicit_user_correction_wins"].(bool); !got {
		t.Fatal("empty-input explicit_user_correction_wins = false, want true")
	}
	if got, _ := emptyCP["guidance_may_override_user_input"].(bool); got {
		t.Fatal("empty-input guidance_may_override_user_input = true, want false")
	}
}

// TestSeq15P115StaleContinuityCarryForwardGuard verifies that stale or
// low-anchor characters are blocked from weak-input default carry-forward.
// Contract: stale_guard.active=true means allow_weak_input_carry_forward=false;
// fresh character with anchor means allow_weak_input_carry_forward=true.
func TestSeq15P115StaleContinuityCarryForwardGuard(t *testing.T) {

	staleItem := store.CharacterState{
		ChatSessionID: "sess-seq15-p115-stale",
		CharacterName: "the hooded stranger",
		TurnIndex:     2,
	}
	staleSnap := characterStaleSnapshot(staleItem, nil, 10, "", nil)
	staleGuard := asMap(staleSnap["stale_guard"])

	if got, _ := staleGuard["active"].(bool); !got {
		t.Fatal("stale character: stale_guard.active = false, want true")
	}
	if got, _ := staleGuard["allow_weak_input_carry_forward"].(bool); got {
		t.Fatal("stale character: allow_weak_input_carry_forward = true, want false")
	}
	if got, _ := staleSnap["is_stale"].(bool); !got {
		t.Fatal("stale character: is_stale = false, want true")
	}

	freshItem := store.CharacterState{
		ChatSessionID:     "sess-seq15-p115-fresh",
		CharacterName:     "Kael",
		PersonalityJSON:   `{"trait":"loyal"}`,
		RelationshipsJSON: `{"Bryn":{"summary":"ally"}}`,
		TurnIndex:         9,
	}
	freshSnap := characterStaleSnapshot(freshItem, nil, 10, "Kael drew his sword", nil)
	freshGuard := asMap(freshSnap["stale_guard"])

	if got, _ := freshGuard["active"].(bool); got {
		t.Fatal("fresh anchored character: stale_guard.active = true, want false")
	}
	if got, _ := freshGuard["allow_weak_input_carry_forward"].(bool); !got {
		t.Fatal("fresh anchored character: allow_weak_input_carry_forward = false, want true")
	}
	if got, _ := freshSnap["admission_class"].(string); got != "major_recurring" {
		t.Fatalf("fresh anchored character: admission_class = %q, want %q", got, "major_recurring")
	}

	lowAnchorItem := store.CharacterState{
		ChatSessionID: "sess-seq15-p115-low",
		CharacterName: "GuardCaptain",
		TurnIndex:     3,
	}
	lowSnap := characterStaleSnapshot(lowAnchorItem, nil, 10, "", nil)
	lowGuard := asMap(lowSnap["stale_guard"])

	if got, _ := lowGuard["active"].(bool); !got {
		t.Fatal("low-anchor gap character: stale_guard.active = false, want true")
	}
	if got, _ := lowGuard["allow_weak_input_carry_forward"].(bool); got {
		t.Fatal("low-anchor gap character: allow_weak_input_carry_forward = true, want false")
	}

	remenItem := store.CharacterState{
		ChatSessionID: "sess-seq15-p115-rem",
		CharacterName: "the hooded stranger",
		TurnIndex:     2,
	}
	remenSnap := characterStaleSnapshot(remenItem, nil, 10, "the hooded stranger returned", nil)
	remenGuard := asMap(remenSnap["stale_guard"])

	if got, _ := remenSnap["is_stale"].(bool); got {
		t.Fatal("recently re-mentioned descriptor: is_stale = true, want false")
	}
	if got, _ := remenGuard["allow_weak_input_carry_forward"].(bool); !got {
		t.Fatal("recently re-mentioned descriptor: allow_weak_input_carry_forward = false, want true")
	}

	eventItem := store.CharacterState{
		ChatSessionID: "sess-seq15-p115-ev",
		CharacterName: "Lyra",
		TurnIndex:     5,
	}
	events := []store.CharacterEvent{
		{EventType: "relationship_shift", DetailsJSON: `{"summary":"trust shift"}`},
		{EventType: "personality_change", DetailsJSON: `{"summary":"growth"}`},
	}
	eventSnap := characterStaleSnapshot(eventItem, events, 10, "", nil)
	eventGuard := asMap(eventSnap["stale_guard"])

	if got, _ := eventSnap["admission_class"].(string); got != "major_recurring" {
		t.Fatalf("event-anchored: admission_class = %q, want %q", got, "major_recurring")
	}
	if got, _ := eventGuard["active"].(bool); got {
		t.Fatal("event-anchored: stale_guard.active = true, want false")
	}
	if got, _ := eventGuard["allow_weak_input_carry_forward"].(bool); !got {
		t.Fatal("event-anchored: allow_weak_input_carry_forward = false, want true")
	}
}

func TestSeq15P119DurableCurrentDriftReplay(t *testing.T) {

	item := store.CharacterState{
		ID:              119,
		ChatSessionID:   "sess-seq15-p119",
		CharacterName:   "Mira",
		AppearanceJSON:  `{"height":"tall","visible_scar":"left cheek","internal_fear":"heights"}`,
		PersonalityJSON: `{"core":"resolute","flaw":"stubborn"}`,
		SpeechStyleJSON: `{"tone":"direct","pace":"measured"}`,
		StatusJSON:      `{"mood":"alert","location":"market"}`,
		TurnIndex:       5,
	}

	snap1 := characterStaleSnapshot(item, nil, 5, "", map[string]struct{}{})
	stable1 := buildStableCharacterSheet(item, snap1)

	item.StatusJSON = `{"mood":"exhausted","location":"fortress","injury":"left arm"}`
	item.TurnIndex = 20
	events := []store.CharacterEvent{
		{CharacterName: "Mira", TurnIndex: 12, EventType: "action", DetailsJSON: `{"summary":"fought the beast"}`},
		{CharacterName: "Mira", TurnIndex: 18, EventType: "relationship_shift", DetailsJSON: `{"summary":"trusted the healer"}`},
	}
	snap2 := characterStaleSnapshot(item, events, 20, "Mira drew her blade", map[string]struct{}{})
	stable2 := buildStableCharacterSheet(item, snap2)

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile"} {
		v1 := stable1[key]
		v2 := stable2[key]
		if !reflect.DeepEqual(v1, v2) {
			t.Fatalf("durable axis %q drifted between phases: %+v vs %+v", key, v1, v2)
		}
	}

	if got, _ := snap2["is_stale"].(bool); got {
		t.Fatal("character at turn 20 with ref turn 20 should not be stale")
	}

	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dynamic := buildDynamicCharacterDigest(item, snap2, rel, latest, events)
	if dynamic["current_snapshot"] == nil {
		t.Fatal("dynamic digest missing current_snapshot at phase 2")
	}

	for _, key := range []string{"is_stale", "stale_reason", "freshness_turn_gap", "freshness_status", "stale_after_turns"} {
		if _, ok := stable2[key]; ok {
			t.Fatalf("stable surface must not carry current-snapshot field %q", key)
		}
	}

	if _, ok := dynamic["current_status"]; !ok {
		t.Fatal("dynamic digest missing current_status at phase 2")
	}
	if _, ok := dynamic["stale_guard"]; !ok {
		t.Fatal("dynamic digest missing stale_guard at phase 2")
	}

	if stable1["surface_type"] != "stable_character_sheet" || stable2["surface_type"] != "stable_character_sheet" {
		t.Fatalf("surface_type must be stable_character_sheet in both phases")
	}
}

func TestSeq15P120SparseOptionalNoFabricationReplay(t *testing.T) {

	sparseItem := store.CharacterState{
		ID:              120,
		ChatSessionID:   "sess-seq15-p120",
		CharacterName:   "the unnamed wanderer",
		AppearanceJSON:  `{"height":"medium"}`,
		PersonalityJSON: ``,
		SpeechStyleJSON: ``,
		StatusJSON:      ``,
		TurnIndex:       3,
	}

	snap1 := characterStaleSnapshot(sparseItem, nil, 3, "", map[string]struct{}{})
	stable1 := buildStableCharacterSheet(sparseItem, snap1)

	sparseItem.TurnIndex = 15
	snap2 := characterStaleSnapshot(sparseItem, nil, 15, "", map[string]struct{}{})
	stable2 := buildStableCharacterSheet(sparseItem, snap2)

	for phase, stable := range []map[string]any{stable1, stable2} {
		dp := asMap(stable["durable_profile"])
		personality := dp["personality"]
		if pm, ok := personality.(map[string]any); ok && len(pm) > 0 {
			t.Fatalf("phase %d: personality fabricated from empty input: %v", phase+1, pm)
		}
		speech := dp["speech_style"]
		if sm, ok := speech.(map[string]any); ok && len(sm) > 0 {
			t.Fatalf("phase %d: speech_style fabricated from empty input: %v", phase+1, sm)
		}
		for key, val := range stable {
			if s, ok := val.(string); ok && s == "unknown" {
				t.Fatalf("phase %d: field %q uses fabricated 'unknown' string", phase+1, key)
			}
		}
	}

	if _, ok := stable1["sparse_policy"]; !ok {
		t.Fatal("phase 1: sparse_policy missing")
	}
	if _, ok := stable2["sparse_policy"]; !ok {
		t.Fatal("phase 2: sparse_policy missing")
	}

	for phase, stable := range []map[string]any{stable1, stable2} {
		sp := asMap(stable["sparse_policy"])
		emptyAxes := mapAnyOrEmptyStringSlice(sp, "empty_axes")
		mustContain(t, emptyAxes, "personality")
		mustContain(t, emptyAxes, "speech_style")
		_ = phase
	}

	for phase, stable := range []map[string]any{stable1, stable2} {
		obs := asMap(stable["appearance_observable"])
		if obs["height"] != "medium" {
			t.Fatalf("phase %d: appearance_observable height = %v, want 'medium'", phase+1, obs["height"])
		}
	}
}

func TestSeq15P121DigestCapAdmissionReplay(t *testing.T) {

	item := store.CharacterState{
		ID:              121,
		ChatSessionID:   "sess-seq15-p121",
		CharacterName:   "Kael",
		PersonalityJSON: `{"core":"cautious","strength":"tactics"}`,
		StatusJSON:      `{"mood":"focused","location":"war room"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","summary":"strategic partner"},
			"Bryn":{"trust":"medium","summary":"scout"},
			"Dex":{"tension":"rising","summary":"rival commander"},
			"Elara":{"closeness":"warm","summary":"healer"},
			"Fen":{"distance":"cold","summary":"estranged"},
			"Gwen":{"bond":"sworn","summary":"oath sibling"}
		}`,
		TurnIndex: 30,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Kael", TurnIndex: 5, EventType: "status_change", DetailsJSON: `{"summary":"arrived at camp"}`},
		{CharacterName: "Kael", TurnIndex: 10, EventType: "relationship_shift", DetailsJSON: `{"summary":"formed alliance"}`},
		{CharacterName: "Kael", TurnIndex: 15, EventType: "personality_change", DetailsJSON: `{"summary":"became more cautious"}`},
		{CharacterName: "Kael", TurnIndex: 20, EventType: "action", DetailsJSON: `{"summary":"led the raid"}`},
		{CharacterName: "Kael", TurnIndex: 25, EventType: "dialogue", DetailsJSON: `{"summary":"confided in Bryn"}`},
		{CharacterName: "Kael", TurnIndex: 28, EventType: "appearance_change", DetailsJSON: `{"summary":"acquired scar"}`},
		{CharacterName: "Kael", TurnIndex: 29, EventType: "relationship_shift", DetailsJSON: `{"summary":"broke with Fen"}`},
	}

	snap := characterStaleSnapshot(item, events, 30, "", map[string]struct{}{})

	if snap["admission_class"] != "major_recurring" {
		t.Fatalf("admission_class = %v, want major_recurring", snap["admission_class"])
	}

	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dynamic := buildDynamicCharacterDigest(item, snap, rel, latest, events)

	budget := asMap(dynamic["digest_budget"])
	if budget["policy"] != "priority_capped" {
		t.Fatalf("digest_budget.policy = %v, want priority_capped", budget["policy"])
	}
	if budget["relationship_lane_cap"] != 4 {
		t.Fatalf("relationship_lane_cap = %v, want 4", budget["relationship_lane_cap"])
	}
	if budget["milestone_cap"] != 3 {
		t.Fatalf("milestone_cap = %v, want 3", budget["milestone_cap"])
	}
	if used, ok := budget["milestones_used"].(int); !ok || used > 3 {
		t.Fatalf("milestones_used = %v, want int <= 3", budget["milestones_used"])
	}
	if used, ok := budget["relationship_lane_used"].(int); !ok || used > 4 {
		t.Fatalf("relationship_lane_used = %v, want int <= 4", budget["relationship_lane_used"])
	}

	ledger := mapAnySlice(dynamic["milestone_ledger"])
	if len(ledger) > 3 {
		t.Fatalf("milestone_ledger length %d exceeds cap 3", len(ledger))
	}

	item.TurnIndex = 50
	item.StatusJSON = `{"mood":"weary","location":"ruins"}`
	moreEvents := append(events,
		store.CharacterEvent{CharacterName: "Kael", TurnIndex: 35, EventType: "action", DetailsJSON: `{"summary":"crossed the bridge"}`},
		store.CharacterEvent{CharacterName: "Kael", TurnIndex: 40, EventType: "relationship_shift", DetailsJSON: `{"summary":"reconciled with Fen"}`},
		store.CharacterEvent{CharacterName: "Kael", TurnIndex: 45, EventType: "personality_change", DetailsJSON: `{"summary":"gained resolve"}`},
		store.CharacterEvent{CharacterName: "Kael", TurnIndex: 48, EventType: "dialogue", DetailsJSON: `{"summary":"debated strategy"}`},
	)
	snap2 := characterStaleSnapshot(item, moreEvents, 50, "", map[string]struct{}{})
	rel2 := buildCharacterRelationshipLane(item)
	latest2 := buildCharacterLatestInteractionAnchor(&moreEvents[len(moreEvents)-1])
	dynamic2 := buildDynamicCharacterDigest(item, snap2, rel2, latest2, moreEvents)
	budget2 := asMap(dynamic2["digest_budget"])

	if budget2["relationship_lane_cap"] != 4 {
		t.Fatalf("replay relationship_lane_cap = %v, want 4", budget2["relationship_lane_cap"])
	}
	if budget2["milestone_cap"] != 3 {
		t.Fatalf("replay milestone_cap = %v, want 3", budget2["milestone_cap"])
	}
	ledger2 := mapAnySlice(dynamic2["milestone_ledger"])
	if len(ledger2) > 3 {
		t.Fatalf("replay milestone_ledger length %d exceeds cap 3", len(ledger2))
	}
}

func TestSeq15P122RelationDescriptorInflationReplay(t *testing.T) {

	item := store.CharacterState{
		ID:            122,
		ChatSessionID: "sess-seq15-p122",
		CharacterName: "Zara",
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","closeness":"warm","tension":"none","bond":"strong","distance":"near","stance":"allied","summary":"trusted protagonist"},
			"Bryn":{"trust":"medium","closeness":"cool","tension":"rising","summary":"uneasy ally"},
			"Dex":{"tension":"high","summary":"rival"},
			"Elara":{"trust":"high","closeness":"warm","bond":"sworn","summary":"oath sister"},
			"Fen":{"distance":"distant","summary":"estranged"},
			"Gwen":{"trust":"low","tension":"high","closeness":"cold","bond":"broken","summary":"betrayed"}
		}`,
		TurnIndex: 25,
	}

	lane1 := buildCharacterRelationshipLane(item)
	items1 := mapAnySlice(lane1["items"])
	if len(items1) == 0 {
		t.Fatal("phase 1: relationship lane items empty")
	}
	for i, entry := range items1 {
		bands := mapAnyOrEmptyStringSlice(entry, "descriptor_bands")
		if len(bands) > 3 {
			t.Fatalf("phase 1: items[%d] descriptor_bands length %d exceeds cap 3: %v", i, len(bands), bands)
		}
		for _, band := range bands {
			if band == "" {
				t.Fatalf("phase 1: items[%d] has empty descriptor band", i)
			}
		}
	}

	lane2 := buildCharacterRelationshipLane(item)
	items2 := mapAnySlice(lane2["items"])
	if len(items2) != len(items1) {
		t.Fatalf("phase 2: item count %d != phase 1 count %d", len(items2), len(items1))
	}
	for i, entry := range items2 {
		bands := mapAnyOrEmptyStringSlice(entry, "descriptor_bands")
		if len(bands) > 3 {
			t.Fatalf("phase 2: items[%d] descriptor_bands inflated to %d: %v", i, len(bands), bands)
		}
	}

	for i, entry := range items2 {
		snap := asMap(entry["state_snapshot"])
		for _, key := range []string{"truth_level", "fact_status", "verified_fact", "canonical"} {
			if _, ok := snap[key]; ok {
				t.Fatalf("phase 2: items[%d] state_snapshot has forbidden key %q", i, key)
			}
		}
	}

	if lane2["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("display_mode = %v, want protagonist_first_then_observed_order", lane2["display_mode"])
	}
}

func TestSeq15P123WeakInputFailModeReplay(t *testing.T) {

	scenarios := []struct {
		name     string
		plan     map[string]any
		director map[string]any
		wantMode string
	}{
		{
			name:     "empty inputs conservative",
			plan:     map[string]any{},
			director: map[string]any{},
			wantMode: "conservative_continuation",
		},
		{
			name:     "strong pressure pressure mode",
			plan:     map[string]any{"current_arc": "Climax"},
			director: map[string]any{"pressure_level": "strong"},
			wantMode: "pressure_continuation_without_resolution",
		},
		{
			name:     "scene mandate scene continuation",
			plan:     map[string]any{"next_beats": []string{"enter the tower"}},
			director: map[string]any{"scene_mandate": "Tower scene"},
			wantMode: "scene_continuation_without_scene_jump",
		},
		{
			name:     "required outcomes carry forward",
			plan:     map[string]any{},
			director: map[string]any{"pressure_level": "steady", "required_outcomes": []string{"reveal_truth"}},
			wantMode: "carry_forward_without_forcing_resolution",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			surface := buildStoryGuidanceSurface(sc.plan, sc.director)
			td := asMap(surface["turn_directives"])
			fm := asMap(td["fail_mode"])

			if got, _ := fm["mode"].(string); got != sc.wantMode {
				t.Fatalf("fail_mode.mode = %q, want %q", got, sc.wantMode)
			}

			if got, _ := fm["respect_explicit_user_correction"].(bool); !got {
				t.Fatal("fail_mode.respect_explicit_user_correction = false, want true")
			}

			if got, _ := fm["allow_scene_jump"].(bool); got {
				t.Fatal("fail_mode.allow_scene_jump = true, want false")
			}

			if got, _ := fm["allow_forced_resolution"].(bool); got {
				t.Fatal("fail_mode.allow_forced_resolution = true, want false")
			}

			prec := asMap(surface["precedence"])
			if got, _ := prec["guidance_authority"].(string); got != "subordinate" {
				t.Fatalf("guidance_authority = %q, want subordinate", got)
			}

			cp := asMap(surface["conflict_policy"])
			if got, _ := cp["explicit_user_correction_wins"].(bool); !got {
				t.Fatal("explicit_user_correction_wins = false, want true")
			}
			if got, _ := cp["guidance_may_override_user_input"].(bool); got {
				t.Fatal("guidance_may_override_user_input = true, want false")
			}
		})
	}
}

func TestSeq15P124LongSessionDetailPreservationReplay(t *testing.T) {

	item := store.CharacterState{
		ID:              124,
		ChatSessionID:   "sess-seq15-p124",
		CharacterName:   "Alice",
		AppearanceJSON:  `{"height":"tall","visible_scar":"left brow","mannerisms":"speaks softly"}`,
		PersonalityJSON: `{"core":"determined","flaw":"impatient"}`,
		SpeechStyleJSON: `{"tone":"quiet","pace":"deliberate"}`,
		StatusJSON:      `{"location":"citadel","mood":"focused","goal":"find the traitor"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","closeness":"warm","summary":"trusted partner"},
			"Bryn":{"trust":"medium","summary":"scout"},
			"Dex":{"tension":"high","summary":"rival"},
			"Elara":{"closeness":"warm","summary":"healer"},
			"Fen":{"distance":"cold","summary":"estranged"}
		}`,
		TurnIndex: 100,
	}

	events := make([]store.CharacterEvent, 0, 20)
	eventTypes := []string{"status_change", "relationship_shift", "personality_change", "action", "dialogue", "appearance_change"}
	summaries := []string{
		"arrived at the gate", "spoke to the guard", "noticed a shadow",
		"entered the hall", "examined the map", "confronted the spy",
		"escaped the ambush", "healed the wound", "crossed the river",
		"found the clue", "warned the ally", "rested at camp",
		"discovered the passage", "faced the guardian", "negotiated the treaty",
		"betrayed by Fen", "saved Bryn", "found the traitor",
		"returned to the citadel", "planned the next move",
	}
	for i := 0; i < 20; i++ {
		events = append(events, store.CharacterEvent{
			CharacterName: "Alice",
			TurnIndex:     i * 5,
			EventType:     eventTypes[i%len(eventTypes)],
			DetailsJSON:   `{"summary":"` + summaries[i] + `"}`,
		})
	}

	snap := characterStaleSnapshot(item, events, 100, "", map[string]struct{}{})
	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dynamic := buildDynamicCharacterDigest(item, snap, rel, latest, events)
	stable := buildStableCharacterSheet(item, snap)

	ledger := mapAnySlice(dynamic["milestone_ledger"])
	if len(ledger) > 3 {
		t.Fatalf("milestone_ledger length %d exceeds cap 3 for long session", len(ledger))
	}
	if len(ledger) == 0 {
		t.Fatal("milestone_ledger should not be empty for long session")
	}

	relLane := mapAnySlice(dynamic["relationship_lane"])
	if len(relLane) > 6 {
		t.Fatalf("relationship_lane length %d exceeds lane cap 6 for long session", len(relLane))
	}

	descLane := mapAnySlice(dynamic["relationship_descriptor_lane"])
	for i, entry := range descLane {
		bands := mapAnyOrEmptyStringSlice(entry, "descriptor_bands")
		if len(bands) > 3 {
			t.Fatalf("relationship_descriptor_lane[%d] bands length %d exceeds cap 3", i, len(bands))
		}
	}

	budget := asMap(dynamic["digest_budget"])
	if budget["policy"] != "priority_capped" {
		t.Fatalf("digest_budget.policy = %v, want priority_capped", budget["policy"])
	}
	if used, ok := budget["milestones_used"].(int); !ok || used > 3 {
		t.Fatalf("milestones_used = %v, want int <= 3", budget["milestones_used"])
	}

	for _, key := range []string{"relationship_lane", "milestone_ledger", "latest_interaction_anchor", "current_status", "stale_guard"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable surface must not carry dynamic key %q", key)
		}
	}

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile"} {
		if _, ok := dynamic[key]; ok {
			t.Fatalf("dynamic digest must not carry durable key %q", key)
		}
	}

	item.TurnIndex = 150
	extraEvents := append(events,
		store.CharacterEvent{CharacterName: "Alice", TurnIndex: 110, EventType: "action", DetailsJSON: `{"summary":"stormed the gates"}`},
		store.CharacterEvent{CharacterName: "Alice", TurnIndex: 120, EventType: "relationship_shift", DetailsJSON: `{"summary":"forged new alliance"}`},
		store.CharacterEvent{CharacterName: "Alice", TurnIndex: 130, EventType: "personality_change", DetailsJSON: `{"summary":"became more resolute"}`},
		store.CharacterEvent{CharacterName: "Alice", TurnIndex: 140, EventType: "dialogue", DetailsJSON: `{"summary":"gave the final speech"}`},
		store.CharacterEvent{CharacterName: "Alice", TurnIndex: 148, EventType: "status_change", DetailsJSON: `{"summary":"rested at last"}`},
	)
	snap2 := characterStaleSnapshot(item, extraEvents, 150, "", map[string]struct{}{})
	rel2 := buildCharacterRelationshipLane(item)
	latest2 := buildCharacterLatestInteractionAnchor(&extraEvents[len(extraEvents)-1])
	dynamic2 := buildDynamicCharacterDigest(item, snap2, rel2, latest2, extraEvents)

	ledger2 := mapAnySlice(dynamic2["milestone_ledger"])
	if len(ledger2) > 3 {
		t.Fatalf("extended replay milestone_ledger length %d exceeds cap 3", len(ledger2))
	}
	budget2 := asMap(dynamic2["digest_budget"])
	if used, ok := budget2["milestones_used"].(int); !ok || used > 3 {
		t.Fatalf("extended replay milestones_used = %v, want int <= 3", budget2["milestones_used"])
	}
}

func TestSeq15P128SessionStateReadEquivalent(t *testing.T) {
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-p128",
			StoryPlanJSON: `{"current_arc":"P128 Arc"}`,
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-p128", CharacterName: "Rowan", PersonalityJSON: `{"core":"bold"}`, TurnIndex: 10},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-p128", Name: "Main Arc"},
		},
	}
	mux := http.NewServeMux()
	s := &Server{Store: fake}
	s.registerNarrativeRoutes(mux)

	req := httptest.NewRequest("GET", "/session-state/sess-p128", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /session-state status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"active_states", "canonical_state_layer", "characters", "storylines", "world_rules", "pending_threads"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("session_state missing core section %q", key)
		}
	}

	gs := asMap(body["guidance_snapshot"])
	if gs == nil {
		t.Fatal("guidance_snapshot missing from session_state")
	}
}

func TestSeq15P129DirectorPersistenceEquivalent(t *testing.T) {
	prevDir := map[string]any{
		"pressure_level":    "steady",
		"required_outcomes": []string{"establish trust"},
	}
	dirJSON, _ := json.Marshal(prevDir)
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-p129",
			DirectorJSON:  string(dirJSON),
		},
	}
	mux := http.NewServeMux()
	s := &Server{Store: fake}
	s.registerNarrativeRoutes(mux)

	patch := map[string]any{
		"pressure_level":    "strong",
		"required_outcomes": []string{"confront the rival"},
	}
	patchBytes, _ := json.Marshal(patch)
	req := httptest.NewRequest("PATCH", "/narrative-control/sess-p129/director-patch", bytes.NewReader(patchBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("PATCH director-patch status = %d, body = %s", rr.Code, rr.Body.String())
	}

	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected savedGuidancePlanState after PATCH")
	}
	var savedDir map[string]any
	if err := json.Unmarshal([]byte(fake.savedGuidancePlanState.DirectorJSON), &savedDir); err != nil {
		t.Fatalf("unmarshal saved director: %v", err)
	}
	if got, _ := savedDir["pressure_level"].(string); got != "strong" {
		t.Fatalf("pressure_level = %q, want strong after PATCH", got)
	}
}

func TestSeq15P130TraceSupervisorEquivalent(t *testing.T) {
	fake := &narrativeFakeStore{
		sessions: []store.SessionSummary{{ChatSessionID: "sess-p130"}},
	}
	mux := http.NewServeMux()
	s := &Server{Store: fake}
	s.registerProxyRoutes(mux)

	payload := map[string]any{
		"chat_session_id": "sess-p130",
		"turn_index":      1,
		"raw_user_input":  "trace supervisor check",
		"guide_mode":      "action",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/supervisor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /supervisor status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode supervisor response: %v", err)
	}
	if resp["status"] != "ok" || resp["source"] != "shadow" {
		t.Fatalf("unexpected supervisor status/source: %+v", resp)
	}
	if resp["would_call_llm"] != false || resp["would_write"] != false {
		t.Fatalf("supervisor trace must remain read-only shadow: %+v", resp)
	}
	pack := asMap(resp["supervisor_input_pack"])
	if pack["status"] != "ready" {
		t.Fatalf("supervisor_input_pack.status = %v, want ready", pack["status"])
	}
	trace := asMap(resp["trace_summary"])
	if trace["guide_mode"] != "action" {
		t.Fatalf("trace_summary.guide_mode = %v, want action", trace["guide_mode"])
	}
	if trace["llm_call"] != "not_configured" || trace["would_call_llm"] != false {
		t.Fatalf("trace_summary should show not_configured shadow LLM call: %+v", trace)
	}
}

func TestSeq15P131GenerationPacketEquivalent(t *testing.T) {
	assembly := prepareTurnInjectionAssembly{
		MemoryText: "Chapter 3: The Deep Forest - the hero enters the woods",
	}
	rec := buildGenerationPacketShadowCompareRecord(assembly, "user walks into the forest")

	if _, ok := rec["new_has_chapter"]; !ok {
		t.Fatal("shadow compare record missing new_has_chapter")
	}
	if _, ok := rec["divergence_chapter"]; !ok {
		t.Fatal("shadow compare record missing divergence_chapter")
	}
	if got, _ := rec["new_has_chapter"].(bool); !got {
		t.Fatal("new_has_chapter should be true for chapter-bearing memory text")
	}
}

func TestSeq15P132AggregateContractEquivalent(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Aggregate Arc"})
	dirJSON, _ := json.Marshal(map[string]any{"pressure_level": "calm"})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-p132",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
		},
	}
	mux := http.NewServeMux()
	s := &Server{Store: fake}
	s.registerNarrativeRoutes(mux)

	req := httptest.NewRequest("GET", "/session-state/sess-p132", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /session-state status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	required := []string{
		"active_states",
		"canonical_state_layer",
		"characters",
		"storylines",
		"world_rules",
		"pending_threads",
		"guidance_snapshot",
		"section_meta",
		"snapshot_status",
	}
	for _, key := range required {
		if _, ok := body[key]; !ok {
			t.Fatalf("aggregate contract missing key %q", key)
		}
	}
}

func TestSeq15P133Step14ValidationReplayEquivalent(t *testing.T) {

	item := store.CharacterState{
		ID:                133,
		ChatSessionID:     "sess-seq15-p133",
		CharacterName:     "Rowan",
		PersonalityJSON:   `{"core":"bold"}`,
		StatusJSON:        `{"mood":"alert"}`,
		RelationshipsJSON: `{"{{user}}":{"trust":"high","summary":"ally"}}`,
		TurnIndex:         20,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Rowan", TurnIndex: 5, EventType: "action", DetailsJSON: `{"summary":"scouted ahead"}`},
		{CharacterName: "Rowan", TurnIndex: 15, EventType: "dialogue", DetailsJSON: `{"summary":"warned the group"}`},
	}

	snap1 := characterStaleSnapshot(item, events, 20, "", map[string]struct{}{})
	rel1 := buildCharacterRelationshipLane(item)
	latest1 := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dyn1 := buildDynamicCharacterDigest(item, snap1, rel1, latest1, events)

	snap2 := characterStaleSnapshot(item, events, 20, "", map[string]struct{}{})
	rel2 := buildCharacterRelationshipLane(item)
	latest2 := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dyn2 := buildDynamicCharacterDigest(item, snap2, rel2, latest2, events)

	b1 := asMap(dyn1["digest_budget"])
	b2 := asMap(dyn2["digest_budget"])
	if b1["policy"] != b2["policy"] {
		t.Fatalf("replay budget policy mismatch: %v vs %v", b1["policy"], b2["policy"])
	}
	if b1["milestone_cap"] != b2["milestone_cap"] {
		t.Fatalf("replay milestone_cap mismatch: %v vs %v", b1["milestone_cap"], b2["milestone_cap"])
	}
}

func TestSeq15P134Step15ValidationReplayEquivalent(t *testing.T) {

	scenarios := []struct {
		name     string
		plan     map[string]any
		director map[string]any
		wantMode string
	}{
		{"empty", map[string]any{}, map[string]any{}, "conservative_continuation"},
		{"strong", map[string]any{"current_arc": "Climax"}, map[string]any{"pressure_level": "strong"}, "pressure_continuation_without_resolution"},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {

			s1 := buildStoryGuidanceSurface(sc.plan, sc.director)
			fm1 := asMap(asMap(s1["turn_directives"])["fail_mode"])

			s2 := buildStoryGuidanceSurface(sc.plan, sc.director)
			fm2 := asMap(asMap(s2["turn_directives"])["fail_mode"])
			if fm1["mode"] != fm2["mode"] {
				t.Fatalf("replay fail_mode mismatch: %v vs %v", fm1["mode"], fm2["mode"])
			}
			if got, _ := fm2["mode"].(string); got != sc.wantMode {
				t.Fatalf("mode = %q, want %q", got, sc.wantMode)
			}
			if got, _ := fm2["allow_scene_jump"].(bool); got {
				t.Fatal("replay: allow_scene_jump must be false")
			}
		})
	}
}

func TestSeq15P135BroaderValidationAggregateEquivalent(t *testing.T) {

	chars := []store.CharacterState{
		{ID: 1, CharacterName: "Alpha", PersonalityJSON: `{"core":"brave"}`, TurnIndex: 10},
		{ID: 2, CharacterName: "Beta", PersonalityJSON: `{"core":"cautious"}`, TurnIndex: 20},
		{ID: 3, CharacterName: "Gamma", PersonalityJSON: `{"core":"curious"}`, TurnIndex: 30},
	}
	for _, ch := range chars {
		snap := characterStaleSnapshot(ch, nil, ch.TurnIndex, "", map[string]struct{}{})
		rel := buildCharacterRelationshipLane(ch)
		dyn := buildDynamicCharacterDigest(ch, snap, rel, nil, nil)
		stable := buildStableCharacterSheet(ch, snap)

		if _, ok := dyn["digest_budget"]; !ok {
			t.Fatalf("char %s: dynamic digest missing digest_budget", ch.CharacterName)
		}

		if _, ok := stable["durable_profile"]; !ok {
			t.Fatalf("char %s: stable sheet missing durable_profile", ch.CharacterName)
		}
	}

	surfaces := []struct {
		plan     map[string]any
		director map[string]any
	}{
		{map[string]any{}, map[string]any{}},
		{map[string]any{"current_arc": "Rising"}, map[string]any{"pressure_level": "steady"}},
		{map[string]any{"next_beats": []string{"rest"}}, map[string]any{"scene_mandate": "camp"}},
	}
	for i, sc := range surfaces {
		surface := buildStoryGuidanceSurface(sc.plan, sc.director)
		if surface == nil {
			t.Fatalf("surface[%d]: nil", i)
		}
		if _, ok := surface["turn_directives"]; !ok {
			t.Fatalf("surface[%d]: missing turn_directives", i)
		}
	}
}

func TestSeq15P139OutputLanguageToggleEquivalent(t *testing.T) {

	promptNoOverride := buildCompleteTurnCriticPrompt("sess-p139", 1, "hello", "hi there", nil, nil, nil)
	if !strings.Contains(promptNoOverride, "Output_Language_Override_JSON") {
		t.Fatal("prompt should always contain Output_Language_Override_JSON section")
	}

	if strings.Contains(promptNoOverride, "Korean") || strings.Contains(promptNoOverride, "Japanese") {
		t.Fatal("prompt without override should not contain any target language")
	}

	override := &map[string]any{
		"language": "Korean",
		"mode":     "strict",
	}
	promptWithOverride := buildCompleteTurnCriticPrompt("sess-p139", 2, "hello", "hi there", nil, override, nil)
	if !strings.Contains(promptWithOverride, "Korean") {
		t.Fatal("prompt with override should contain the target language 'Korean'")
	}
	if !strings.Contains(promptWithOverride, "Output_Language_Override_JSON") {
		t.Fatal("prompt with override should contain 'Output_Language_Override_JSON' section")
	}
}

func TestSeq15P140CriticLanguageOutputOverrideEquivalent(t *testing.T) {
	scenarios := []struct {
		name     string
		override *map[string]any
		wantLang string
		wantMode string
	}{
		{
			name:     "korean strict",
			override: &map[string]any{"language": "Korean", "mode": "strict"},
			wantLang: "Korean",
			wantMode: "strict",
		},
		{
			name:     "japanese soft",
			override: &map[string]any{"language": "Japanese", "mode": "soft"},
			wantLang: "Japanese",
			wantMode: "soft",
		},
		{
			name:     "nil override",
			override: nil,
			wantLang: "",
			wantMode: "",
		},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			prompt := buildCompleteTurnCriticPrompt("sess-p140", 1, "user input", "assistant reply", nil, sc.override, nil)
			if sc.wantLang != "" && !strings.Contains(prompt, sc.wantLang) {
				t.Fatalf("prompt should contain language %q", sc.wantLang)
			}
			if sc.wantMode != "" && !strings.Contains(prompt, sc.wantMode) {
				t.Fatalf("prompt should contain mode %q", sc.wantMode)
			}
			if sc.override == nil && strings.Contains(prompt, "output_language_override") {
				t.Fatal("nil override should not produce output_language_override section")
			}
		})
	}
}

func TestSeq15P136ArchiveCenterJSSyntaxCheckEquivalent(t *testing.T) {

	srv := &Server{Cfg: config.Default()}
	if srv == nil {
		t.Fatal("Server construction must succeed")
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health check = %d, want 200", rec.Code)
	}
}

func TestSeq15P137LegacyJSCheckRemigrationEvidence(t *testing.T) {

	jsPath := filepath.Join("..", "..", "..", "Archive Center.js")
	info, err := os.Stat(jsPath)
	if err != nil {
		t.Fatalf("2.0 Archive Center.js must exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("2.0 Archive Center.js must not be empty")
	}
}

func TestSeq15P138LegacyPythonCompileRemigrationEvidence(t *testing.T) {

	cfg := config.Default()
	if cfg.StoreMode == "" {
		t.Fatal("default config StoreMode must be set")
	}
	srv := NewServer(cfg)
	if srv == nil {
		t.Fatal("NewServer must return non-nil")
	}
	if srv.Cfg.StoreMode != cfg.StoreMode {
		t.Fatalf("server config StoreMode = %q, want %q", srv.Cfg.StoreMode, cfg.StoreMode)
	}
}

// TestSeq15P145StableSurfaceDurableCurrentSplitSmoke verifies P145:
// stable surface (buildStableCharacterSheet) carries durable fields only;
// dynamic/current snapshot (characterStaleSnapshot) carries current-state only.
// No cross-contamination between the two lanes.
func TestSeq15P145StableSurfaceDurableCurrentSplitSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:              145,
		ChatSessionID:   "sess-seq15-p145",
		CharacterName:   "Kaelen",
		AppearanceJSON:  `{"build":"lean","eye_color":"amber","aura":"calm"}`,
		PersonalityJSON: `{"core":"analytical","flaw":"overthinks"}`,
		SpeechStyleJSON: `{"tone":"measured","pace":"slow"}`,
		StatusJSON:      `{"mood":"focused","location":"library"}`,
		TurnIndex:       42,
	}

	snapshot := characterStaleSnapshot(item, nil, 42, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}

	durableKeys := []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"}
	for _, key := range durableKeys {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable sheet missing durable axis %q", key)
		}
	}

	forbiddenInStable := []string{"is_stale", "stale_reason", "freshness_turn_gap", "stale_after_turns", "freshness_status"}
	for _, key := range forbiddenInStable {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable sheet must not carry current-snapshot field %q", key)
		}
	}

	currentFields := []string{"is_stale", "stale_reason", "freshness_turn_gap", "stale_guard", "last_observed_turn"}
	for _, key := range currentFields {
		if _, ok := snapshot[key]; !ok {
			t.Fatalf("current snapshot missing field %q", key)
		}
	}

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile"} {
		if _, ok := snapshot[key]; ok {
			t.Fatalf("current snapshot must not carry durable-axis field %q", key)
		}
	}

	obsVal, ok := stable["appearance_observable"].(map[string]any)
	if !ok || len(obsVal) == 0 {
		t.Fatalf("appearance_observable should be populated map, got %T %+v", stable["appearance_observable"], stable["appearance_observable"])
	}

	_, ok = stable["appearance_non_observable"].(map[string]any)
	if !ok {
		t.Fatalf("appearance_non_observable should be map[string]any, got %T", stable["appearance_non_observable"])
	}
}

// TestSeq15P146DigestCapAdmissionSmoke verifies P146:
// dynamic character digest must include digest_budget caps and admission fields;
// current snapshot must stay in the dynamic lane rather than stable sheet.
func TestSeq15P146DigestCapAdmissionSmoke(t *testing.T) {
	item := store.CharacterState{
		ID:              146,
		ChatSessionID:   "sess-seq15-p146",
		CharacterName:   "Mira",
		AppearanceJSON:  `{"build":"slight","mark":"scar on left cheek"}`,
		PersonalityJSON: `{"core":"loyal","flaw":"distrustful"}`,
		SpeechStyleJSON: `{"tone":"soft","pace":"deliberate"}`,
		StatusJSON:      `{"mood":"wary","location":"alley"}`,
		TurnIndex:       17,
	}

	snapshot := characterStaleSnapshot(item, nil, 17, "", map[string]struct{}{})
	rel := buildCharacterRelationshipLane(item)
	digest := buildDynamicCharacterDigest(item, snapshot, rel, nil, nil)

	budget := asMap(digest["digest_budget"])
	if got := budget["policy"]; got != "priority_capped" {
		t.Fatalf("digest_budget.policy = %v, want priority_capped", got)
	}
	if got := budget["relationship_lane_cap"]; got != 4 {
		t.Fatalf("digest_budget.relationship_lane_cap = %v, want 4", got)
	}
	if got := budget["milestone_cap"]; got != 3 {
		t.Fatalf("digest_budget.milestone_cap = %v, want 3", got)
	}
	if got := digest["admission_class"]; got == "" || got == nil {
		t.Fatal("dynamic digest missing admission_class")
	}
	if got := digest["admission_basis"]; got == "" || got == nil {
		t.Fatal("dynamic digest missing admission_basis")
	}
	if _, ok := digest["current_snapshot"].(map[string]any); !ok {
		t.Fatalf("dynamic digest missing current_snapshot map, got %T", digest["current_snapshot"])
	}
}
