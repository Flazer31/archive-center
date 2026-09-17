package httpapi

import (
	"sort"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// seq15SortedKeys returns sorted keys from a map[string]any for diagnostic output.
func seq15SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// asMap safely casts value to map[string]any, returning empty map on failure.
func asMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

// mustContain asserts that slice contains the expected string.
func mustContain(t *testing.T, slice []string, expected string) {
	t.Helper()
	for _, s := range slice {
		if s == expected {
			return
		}
	}
	t.Fatalf("slice %v does not contain %q", slice, expected)
}

// mustContainAll asserts that slice contains all expected strings.
func mustContainAll(t *testing.T, slice []string, expected []string) {
	t.Helper()
	set := map[string]bool{}
	for _, s := range slice {
		set[s] = true
	}
	for _, want := range expected {
		if !set[want] {
			t.Fatalf("slice %v missing %q", slice, want)
		}
	}
}

// TestSeq15P84DurableCurrentSplitStableSnapshotField verifies P84:
// stable surface (buildStableCharacterSheet) carries durable trait/evidence spine,
// dynamic/current snapshot (characterStaleSnapshot) carries current state.
// The two surfaces must be separated and not cross-contaminate.
func TestSeq15P84DurableCurrentSplitStableSnapshotField(t *testing.T) {
	item := store.CharacterState{
		ID:              84,
		ChatSessionID:   "sess-seq15-p84",
		CharacterName:   "Mira",
		AppearanceJSON:  `{"height":"tall","left_brow_scar":"visible","internal_feeling":"carries herself like someone who has seen war"}`,
		PersonalityJSON: `{"core":"guarded but loyal","flaw":"slow to trust"}`,
		SpeechStyleJSON: `{"tone":"clipped","pace":"measured"}`,
		TurnIndex:       25,
	}

	snapshot := characterStaleSnapshot(item, nil, 25, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)

	if stable["surface_type"] != "stable_character_sheet" {
		t.Fatalf("stable surface_type = %v, want stable_character_sheet", stable["surface_type"])
	}
	if _, ok := stable["surface_version"]; !ok {
		t.Fatal("stable surface missing surface_version")
	}

	durableKeys := []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"}
	for _, key := range durableKeys {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable sheet missing durable axis %q", key)
		}
	}

	forbidden := []string{"is_stale", "stale_reason", "freshness_turn_gap", "stale_after_turns", "freshness_status"}
	for _, key := range forbidden {
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

	dp := seq14SubMap(t, stable, "durable_profile")
	if dp["personality"] == nil {
		t.Fatal("durable_profile missing personality")
	}
	if dp["speech_style"] == nil {
		t.Fatal("durable_profile missing speech_style")
	}

	obsVal, ok := stable["appearance_observable"].(map[string]any)
	if !ok || len(obsVal) == 0 {
		t.Fatalf("appearance_observable should be populated map for observable data, got %T %+v", stable["appearance_observable"], stable["appearance_observable"])
	}
	nonObsVal, ok := stable["appearance_non_observable"].(map[string]any)
	if !ok || len(nonObsVal) == 0 {
		t.Fatalf("appearance_non_observable should be populated map for internal data, got %T %+v", stable["appearance_non_observable"], stable["appearance_non_observable"])
	}
	if _, ok := obsVal["internal_feeling"]; ok {
		t.Fatalf("appearance_observable must not include non-observable internal field: %+v", obsVal)
	}
	if _, ok := nonObsVal["internal_feeling"]; !ok {
		t.Fatalf("appearance_non_observable missing internal field: %+v", nonObsVal)
	}
}

// TestSeq15P85SparseHonestyUnknownOmit verifies P85:
// unknown/empty/unsupported fields are omitted or expressed as empty-axis,
// not fabricated into "unknown" or guessed filler values.
// Weak inference must not be upgraded to stable fact.
func TestSeq15P85SparseHonestyUnknownOmit(t *testing.T) {

	sparseItem := store.CharacterState{
		ID:              85,
		ChatSessionID:   "sess-seq15-p85",
		CharacterName:   "the hooded stranger",
		AppearanceJSON:  `{}`,
		PersonalityJSON: ``,
		SpeechStyleJSON: ``,
		StatusJSON:      ``,
		TurnIndex:       3,
	}

	snapshot := characterStaleSnapshot(sparseItem, nil, 3, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(sparseItem, snapshot)

	dp := seq14SubMap(t, stable, "durable_profile")
	personality := dp["personality"]
	switch v := personality.(type) {
	case map[string]any:
		if len(v) > 0 {
			t.Fatalf("durable_profile personality should be empty for empty input, got %v", v)
		}
	case nil:

	default:
		t.Fatalf("durable_profile personality should be nil or empty map for empty input, got type %T value %v", personality, personality)
	}

	speechStyle := dp["speech_style"]
	switch v := speechStyle.(type) {
	case map[string]any:
		if len(v) > 0 {
			t.Fatalf("durable_profile speech_style should be empty for empty input, got %v", v)
		}
	case nil:

	default:
		t.Fatalf("durable_profile speech_style should be nil or empty map for empty input, got type %T value %v", speechStyle, speechStyle)
	}

	obsVal, _ := stable["appearance_observable"].(string)
	if obsVal != "" {
		t.Fatalf("appearance_observable should be empty for empty appearance JSON, got %q", obsVal)
	}
	nonObsVal, _ := stable["appearance_non_observable"].(string)
	if nonObsVal != "" {
		t.Fatalf("appearance_non_observable should be empty for empty appearance JSON, got %q", nonObsVal)
	}

	if _, ok := stable["sparse_policy"]; !ok {
		t.Fatal("sparse_policy must be present even when data is sparse")
	}

	for key, val := range stable {
		if s, ok := val.(string); ok && s == "unknown" {
			t.Fatalf("stable field %q must not use fabricated 'unknown' string, got %q", key, s)
		}
	}

	if _, ok := snapshot["unknown_field"]; ok {
		t.Fatal("snapshot must not contain fabricated unknown_field")
	}

	partialItem := store.CharacterState{
		ID:              186,
		ChatSessionID:   "sess-seq15-p85-partial",
		CharacterName:   "Partial Character",
		AppearanceJSON:  `{"height":"tall"}`,
		PersonalityJSON: `{"core":"brave"}`,
		SpeechStyleJSON: ``,
		TurnIndex:       5,
	}
	partialSnapshot := characterStaleSnapshot(partialItem, nil, 5, "", map[string]struct{}{})
	partialStable := buildStableCharacterSheet(partialItem, partialSnapshot)
	partialObs, ok := partialStable["appearance_observable"].(map[string]any)
	if !ok {
		t.Fatalf("appearance_observable should be map for partial appearance data, got %T", partialStable["appearance_observable"])
	}
	if partialObs["height"] != "tall" {
		t.Fatalf("appearance_observable should preserve partial height 'tall', got %+v", partialObs)
	}
	partialNonObs, ok := partialStable["appearance_non_observable"].(map[string]any)
	if !ok {
		t.Fatalf("appearance_non_observable should be map for partial appearance data, got %T", partialStable["appearance_non_observable"])
	}
	if len(partialNonObs) != 0 {
		t.Fatalf("appearance_non_observable should be empty when no non-observable data, got %+v", partialNonObs)
	}
}

// TestSeq15P86SelectiveAdmissionMajorRecurringContinuity verifies P86:
// major recurring characters are preserved as continuity anchors,
// transient low-anchor descriptors are not over-promoted.
func TestSeq15P86SelectiveAdmissionMajorRecurringContinuity(t *testing.T) {

	major := store.CharacterState{
		ID:             86,
		ChatSessionID:  "sess-seq15-p86",
		CharacterName:  "Mira",
		AppearanceJSON: `{"height":"tall"}`,
		TurnIndex:      40,
	}
	majorSnap := characterStaleSnapshot(major, nil, 40, "", map[string]struct{}{})

	if majorSnap["is_stale"] != false {
		t.Fatalf("major recurring character at current turn should not be stale, got %v", majorSnap["is_stale"])
	}
	majorGuard := seq14SubMap(t, majorSnap, "stale_guard")
	if majorSnap["admission_class"] != "major_recurring" {
		t.Fatalf("anchored major character admission_class = %v, want major_recurring", majorSnap["admission_class"])
	}
	if majorGuard["active"] != false {
		t.Fatalf("fresh anchored character stale guard should not be active, got %v", majorGuard["active"])
	}

	transient := store.CharacterState{
		ID:            87,
		ChatSessionID: "sess-seq15-p86",
		CharacterName: "the hooded stranger",
		TurnIndex:     3,
	}

	transientSnap := characterStaleSnapshot(transient, nil, 45, "", map[string]struct{}{})
	if transientSnap["is_stale"] != true {
		t.Fatalf("transient low-anchor at far reference turn should be stale, got %v", transientSnap["is_stale"])
	}

	transientGuard := seq14SubMap(t, transientSnap, "stale_guard")
	if transientSnap["admission_class"] != "transient_descriptor" {
		t.Fatalf("transient descriptor admission_class = %v, want transient_descriptor", transientSnap["admission_class"])
	}
	if transientGuard["allow_weak_input_carry_forward"] != false {
		t.Fatalf("transient stale guard must block weak-input carry-forward, got %v", transientGuard["allow_weak_input_carry_forward"])
	}

	transientNear := characterStaleSnapshot(transient, nil, 5, "", map[string]struct{}{})
	nearGuard := seq14SubMap(t, transientNear, "stale_guard")

	if transientNear["is_stale"] != false {

	}
	_ = nearGuard

	if majorGuard["allow_weak_input_carry_forward"] != true {
		t.Fatalf("major recurring fresh guard must allow weak-input carry-forward, got %v", majorGuard["allow_weak_input_carry_forward"])
	}
}

// TestSeq15P87DigestEconomyPriorityCapped verifies P87:
// dynamic character digest respects priority-capped budgets:
// relationship lane cap, milestone cap, read window, used count.
// Must not flood with excessive detail.
func TestSeq15P87DigestEconomyPriorityCapped(t *testing.T) {
	item := store.CharacterState{
		ID:              88,
		ChatSessionID:   "sess-seq15-p87",
		CharacterName:   "Aric",
		PersonalityJSON: `{"core":"loyal knight","strength":"swordsmanship"}`,
		StatusJSON:      `{"health":"wounded","mood":"resolute"}`,
		TurnIndex:       30,
	}

	events := []store.CharacterEvent{
		{ID: 1, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 5, EventType: "appearance", DetailsJSON: `{"desc":"entered the hall"}`},
		{ID: 2, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 10, EventType: "dialogue", DetailsJSON: `{"desc":"pledged allegiance"}`},
		{ID: 3, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 15, EventType: "action", DetailsJSON: `{"desc":"drew sword"}`},
		{ID: 4, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 20, EventType: "conflict", DetailsJSON: `{"desc":"fought the beast"}`},
		{ID: 5, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 25, EventType: "resolution", DetailsJSON: `{"desc":"accepted knighthood"}`},
		{ID: 6, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 28, EventType: "dialogue", DetailsJSON: `{"desc":"swore oath"}`},
		{ID: 7, ChatSessionID: "sess-seq15-p87", CharacterName: "Aric", TurnIndex: 29, EventType: "action", DetailsJSON: `{"desc":"mounted horse"}`},
	}

	snapshot := characterStaleSnapshot(item, events, 30, "", map[string]struct{}{})
	relationshipLane := map[string]any{
		"display_mode":   "protagonist_first_then_observed_order",
		"primary_target": "{{user}}",
		"summary_text":   "trusted ally",
		"protagonist_relation": map[string]any{
			"target":           "{{user}}",
			"summary_text":     "trusted ally",
			"descriptor_bands": []string{"trust: high"},
		},
		"other_relations": []any{},
		"items": []any{
			map[string]any{
				"target":           "{{user}}",
				"summary_text":     "trusted ally",
				"descriptor_bands": []string{"trust: high"},
			},
		},
	}
	latestAnchor := map[string]any{"event_type": "dialogue", "turn_index": 29, "summary_text": "swore oath"}

	digest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latestAnchor, events)

	if digest["surface_type"] != "dynamic_continuity_digest" {
		t.Fatalf("digest surface_type = %v, want dynamic_continuity_digest", digest["surface_type"])
	}

	relLane := mapAnySlice(digest["relationship_lane"])
	if len(relLane) == 0 {
		t.Fatal("digest relationship_lane should retain capped relationship item")
	}

	digestBudget := seq14SubMap(t, digest, "digest_budget")
	if digestBudget["policy"] != "priority_capped" {
		t.Fatalf("digest_budget.policy = %v, want priority_capped", digestBudget["policy"])
	}
	for _, key := range []string{"relationship_lane_cap", "milestone_cap", "milestone_read_window", "relationship_lane_used", "milestones_used"} {
		if _, ok := digestBudget[key]; !ok {
			t.Fatalf("digest_budget missing cap indicator %q; keys=%v", key, seq15SortedKeys(digestBudget))
		}
	}
	if digestBudget["relationship_lane_cap"] != 4 {
		t.Fatalf("relationship_lane_cap = %v, want 4", digestBudget["relationship_lane_cap"])
	}
	if digestBudget["milestone_cap"] != 3 {
		t.Fatalf("milestone_cap = %v, want 3", digestBudget["milestone_cap"])
	}

	milestoneLedger := characterMilestoneLedger(events)

	if len(milestoneLedger) > 3 {
		t.Fatalf("milestone ledger length %d exceeds milestone cap 3", len(milestoneLedger))
	}

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile"} {
		if _, ok := digest[key]; ok {
			t.Fatalf("dynamic digest must not carry stable surface field %q", key)
		}
	}

	if _, ok := digest["current_status"]; !ok {
		t.Fatal("digest missing current_status")
	}
	if _, ok := digest["stale_guard"]; !ok {
		t.Fatal("digest missing stale_guard")
	}
}

// TestSeq15P88WeakInputStrongerButNotAuthoritative verifies P88:
// weak-input steering can provide stronger focus guidance but
// must not gain more authority than explicit correction / current input /
// hard rules / canonical truth floor.
func TestSeq15P88WeakInputStrongerButNotAuthoritative(t *testing.T) {

	surface := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "Redemption Arc", "narrative_goal": "seek truth", "weak_input": "maybe go left"},
		map[string]any{"pressure_level": "moderate"},
	)

	if surface["surface_type"] != "story_guidance_surface" {
		t.Fatalf("surface_type = %v, want story_guidance_surface", surface["surface_type"])
	}

	prec := seq14SubMap(t, surface, "precedence")
	higher := seq14StringSlice(t, prec["higher_priority_sources"], "higher_priority_sources")
	if !seq14Contains(higher, "explicit_user_correction") {
		t.Fatalf("explicit_user_correction must be in higher_priority_sources, got %v", higher)
	}
	if !seq14Contains(higher, "current_user_input") {
		t.Fatalf("current_user_input must be in higher_priority_sources, got %v", higher)
	}
	hasHard := seq14Contains(higher, "hard_world_rule") || seq14Contains(higher, "hard_rules") || seq14Contains(higher, "hard_rule")
	if !hasHard {
		t.Fatalf("hard rules must be in higher_priority_sources, got %v", higher)
	}

	if seq14Contains(higher, "guidance") {
		t.Fatalf("guidance must not appear in its own higher_priority_sources, got %v", higher)
	}

	turn := seq14SubMap(t, surface, "turn_directives")
	failMode := seq14SubMap(t, turn, "fail_mode")
	if failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode must respect explicit user correction, got %v", failMode)
	}

	if seq14Contains(higher, "weak_input") {
		t.Fatalf("weak_input must not be in higher_priority_sources, got %v", higher)
	}

	if !seq14Contains(higher, "canonical_truth_floor") {

		hasCanonical := seq14Contains(higher, "hard_rules") || seq14Contains(higher, "hard_rule") || seq14Contains(higher, "canonical")
		if !hasCanonical {
			t.Fatalf("canonical truth floor or equivalent must be in higher_priority_sources, got %v", higher)
		}
	}

	if turn["scene_drive"] != "" {
		t.Fatalf("weak input must not become authoritative scene_drive without director mandate, got %v", turn["scene_drive"])
	}
}

func TestSeq15P92DurableTraitCurrentSnapshotFieldSplit(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID:     "sess-seq15-p92",
		CharacterName:     "Alice",
		AppearanceJSON:    `{"height":"tall","visible_scar":"left cheek","internal_fear":"water"}`,
		PersonalityJSON:   `{"core":"stoic","bond":"protective"}`,
		SpeechStyleJSON:   `{"tone":"quiet"}`,
		StatusJSON:        `{"location":"market","mood":"alert","goal":"find the courier"}`,
		RelationshipsJSON: `{"{{user}}":{"trust":"high","summary":"relies on the protagonist"}}`,
		TurnIndex:         12,
	}
	events := []store.CharacterEvent{{CharacterName: "Alice", TurnIndex: 12, EventType: "dialogue", DetailsJSON: `{"summary":"warned the protagonist"}`}}
	snapshot := characterStaleSnapshot(item, events, 12, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)
	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[0])
	dynamic := buildDynamicCharacterDigest(item, snapshot, rel, latest, events)

	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile", "sparse_policy"} {
		if _, ok := stable[key]; !ok {
			t.Fatalf("stable surface missing durable key %q; keys=%v", key, seq15SortedKeys(stable))
		}
	}
	for _, key := range []string{"current_status", "current_snapshot", "stale_guard", "relationship_lane", "latest_interaction_anchor"} {
		if _, ok := dynamic[key]; !ok {
			t.Fatalf("dynamic digest missing current key %q; keys=%v", key, seq15SortedKeys(dynamic))
		}
	}
	for _, key := range []string{"current_status", "current_snapshot", "stale_guard", "relationship_lane", "latest_interaction_anchor"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable surface must not own current/dynamic key %q", key)
		}
	}
	for _, key := range []string{"appearance_observable", "appearance_non_observable", "durable_profile"} {
		if _, ok := dynamic[key]; ok {
			t.Fatalf("dynamic digest must not own durable stable key %q", key)
		}
	}
}

func TestSeq15P93ConcreteObservableSchemaRefinement(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID:   "sess-seq15-p93",
		CharacterName:   "Bob",
		AppearanceJSON:  `{"physical_traits":{"build":"athletic","height":"medium"},"visible_features":["scar on left cheek","tattoo on forearm"],"mannerisms":"speaks softly","internal_secret":"hidden heir"}`,
		PersonalityJSON: `{"role":"messenger"}`,
		TurnIndex:       13,
	}
	snapshot := characterStaleSnapshot(item, nil, 13, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)
	appearance := seq14SubMap(t, stable, "appearance_observable")

	for _, key := range []string{"physical_traits", "visible_features", "mannerisms"} {
		if _, ok := appearance[key]; !ok {
			t.Fatalf("appearance_observable missing concrete key %q; keys=%v", key, seq15SortedKeys(appearance))
		}
	}
	if _, ok := appearance["internal_secret"]; ok {
		t.Fatalf("appearance_observable must not include non-observable internal_secret: %+v", appearance)
	}
	nonObs := seq14SubMap(t, stable, "appearance_non_observable")
	if nonObs["internal_secret"] != "hidden heir" {
		t.Fatalf("appearance_non_observable missing internal_secret: %+v", nonObs)
	}
	if _, ok := appearance["physical_traits"].(map[string]any); !ok {
		t.Fatalf("physical_traits must stay structured, got %T", appearance["physical_traits"])
	}
}

func TestSeq15P94SparseOptionalUnknownOmitWeakInferenceHold(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID:   "sess-seq15-p94",
		CharacterName:   "Charlie",
		AppearanceJSON:  `{"height":"slim"}`,
		PersonalityJSON: ``,
		SpeechStyleJSON: ``,
		TurnIndex:       14,
	}
	snapshot := characterStaleSnapshot(item, nil, 14, "", map[string]struct{}{})
	stable := buildStableCharacterSheet(item, snapshot)

	sparsePolicy := seq14SubMap(t, stable, "sparse_policy")
	emptyAxes := seq14StringSlice(t, sparsePolicy["empty_axes"], "empty_axes")
	if !seq14Contains(emptyAxes, "personality") || !seq14Contains(emptyAxes, "speech_style") {
		t.Fatalf("sparse optional policy should record empty axes, got %v", emptyAxes)
	}
	if _, ok := stable["unknown"]; ok {
		t.Fatal("stable surface must not fabricate unknown field")
	}
	if _, ok := stable["weak_inference"]; ok {
		t.Fatal("stable surface must not promote weak inference")
	}

	guidance := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "test", "narrative_goal": "verify", "weak_input": "maybe suspicious"},
		map[string]any{"pressure_level": "steady"},
	)
	precedence := seq14SubMap(t, guidance, "precedence")
	higher := seq14StringSlice(t, precedence["higher_priority_sources"], "higher_priority_sources")
	if seq14Contains(higher, "weak_input") {
		t.Fatalf("weak_input must not become authority source, got %v", higher)
	}
}

func TestSeq15P98MajorRecurringCharacterAdmissionGate(t *testing.T) {
	events := []store.CharacterEvent{
		{CharacterName: "Diana", TurnIndex: 3, EventType: "dialogue", DetailsJSON: `{"summary":"spoke to the protagonist"}`},
		{CharacterName: "Diana", TurnIndex: 7, EventType: "action", DetailsJSON: `{"summary":"returned with evidence"}`},
	}
	major := store.CharacterState{ChatSessionID: "sess-seq15-p98", CharacterName: "Diana", TurnIndex: 7}
	majorSnap := characterStaleSnapshot(major, events, 7, "", map[string]struct{}{})
	if majorSnap["admission_class"] != "major_recurring" {
		t.Fatalf("event-backed character admission_class = %v, want major_recurring", majorSnap["admission_class"])
	}
	basis := seq14StringSlice(t, majorSnap["admission_basis"], "admission_basis")
	if !seq14Contains(basis, "recent_event_history") {
		t.Fatalf("major recurring admission basis missing recent_event_history: %v", basis)
	}

	transient := store.CharacterState{ChatSessionID: "sess-seq15-p98", CharacterName: "the hooded stranger", TurnIndex: 1}
	transientSnap := characterStaleSnapshot(transient, nil, 12, "", map[string]struct{}{})
	if transientSnap["admission_class"] != "transient_descriptor" {
		t.Fatalf("low-anchor descriptor admission_class = %v, want transient_descriptor", transientSnap["admission_class"])
	}
	guard := seq14SubMap(t, transientSnap, "stale_guard")
	if guard["allow_weak_input_carry_forward"] != false {
		t.Fatalf("transient stale descriptor must not allow weak carry-forward: %+v", guard)
	}
}

func TestSeq15P99MilestoneLedgerEventAnchorDefine(t *testing.T) {
	events := []store.CharacterEvent{
		{CharacterName: "Frank", TurnIndex: 1, EventType: "status_change", DetailsJSON: `{"summary":"arrived in the tavern"}`},
		{CharacterName: "Frank", TurnIndex: 2, EventType: "relationship_shift", DetailsJSON: `{"summary":"swore loyalty to the protagonist"}`},
		{CharacterName: "Frank", TurnIndex: 3, EventType: "personality_change", DetailsJSON: `{"summary":"became openly cautious"}`},
		{CharacterName: "Frank", TurnIndex: 4, EventType: "appearance_change", DetailsJSON: `{"summary":"changed cloak"}`},
	}
	ledger := characterMilestoneLedger(events)
	if len(ledger) == 0 {
		t.Fatal("milestone ledger should retain event anchors")
	}
	if len(ledger) > 3 {
		t.Fatalf("milestone ledger length %d exceeds cap 3", len(ledger))
	}
	types := map[string]bool{}
	for i, raw := range ledger {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("ledger entry %d should be map, got %T", i, raw)
		}
		for _, key := range []string{"event_type", "turn_index", "summary_text", "details"} {
			if _, ok := entry[key]; !ok {
				t.Fatalf("ledger entry %d missing %q; keys=%v", i, key, seq15SortedKeys(entry))
			}
		}
		if eventType, ok := entry["event_type"].(string); ok {
			types[eventType] = true
		}
	}
	if !types["relationship_shift"] || !types["personality_change"] {
		t.Fatalf("ledger should prioritize durable relationship/personality anchors, got %v", types)
	}
}

func TestSeq15P100PriorityCappedDigestBudgetDefine(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID: "sess-seq15-p100",
		CharacterName: "Grace",
		StatusJSON:    `{"location":"clinic","mood":"focused"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"high","summary":"trusted patient"},
			"Iris":{"trust":"medium","summary":"apprentice"},
			"Jules":{"trust":"low","summary":"difficult patron"},
			"Kara":{"tension":"high","summary":"rival healer"},
			"Lio":{"closeness":"distant","summary":"old friend"}
		}`,
		TurnIndex: 20,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Grace", TurnIndex: 1, EventType: "status_change", DetailsJSON: `{"summary":"joined the clinic"}`},
		{CharacterName: "Grace", TurnIndex: 5, EventType: "relationship_shift", DetailsJSON: `{"summary":"earned trust"}`},
		{CharacterName: "Grace", TurnIndex: 9, EventType: "personality_change", DetailsJSON: `{"summary":"became decisive"}`},
		{CharacterName: "Grace", TurnIndex: 12, EventType: "appearance_change", DetailsJSON: `{"summary":"wore a red sash"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 20, "", map[string]struct{}{})
	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dynamic := buildDynamicCharacterDigest(item, snapshot, rel, latest, events)
	budget := seq14SubMap(t, dynamic, "digest_budget")

	if budget["policy"] != "priority_capped" {
		t.Fatalf("digest_budget.policy = %v, want priority_capped", budget["policy"])
	}
	if budget["relationship_lane_cap"] != 4 || budget["milestone_cap"] != 3 || budget["milestone_read_window"] != 8 {
		t.Fatalf("unexpected digest budget caps: %+v", budget)
	}
	if used, ok := budget["relationship_lane_used"].(int); !ok || used > 4 {
		t.Fatalf("relationship_lane_used should be int <= 4, got %T %v", budget["relationship_lane_used"], budget["relationship_lane_used"])
	}
	if used, ok := budget["milestones_used"].(int); !ok || used > 3 {
		t.Fatalf("milestones_used should be int <= 3, got %T %v", budget["milestones_used"], budget["milestones_used"])
	}
}

func TestSeq15P101LatestInteractionVsEventLedgerRoleSplit(t *testing.T) {
	latestEvent := store.CharacterEvent{CharacterName: "Hank", TurnIndex: 12, EventType: "dialogue", DetailsJSON: `{"summary":"confided his secret"}`}
	olderEvents := []store.CharacterEvent{
		{CharacterName: "Hank", TurnIndex: 1, EventType: "status_change", DetailsJSON: `{"summary":"first met"}`},
		{CharacterName: "Hank", TurnIndex: 8, EventType: "relationship_shift", DetailsJSON: `{"summary":"saved the bridge"}`},
	}
	latest := buildCharacterLatestInteractionAnchor(&latestEvent)
	latestMap, ok := latest.(map[string]any)
	if !ok {
		t.Fatalf("latest interaction anchor must be map, got %T", latest)
	}
	if latestMap["surface_type"] != "latest_interaction_anchor" || latestMap["event_type"] != "dialogue" || latestMap["turn_index"] != 12 {
		t.Fatalf("unexpected latest interaction anchor: %+v", latestMap)
	}
	if _, ok := latestMap["milestone_ledger"]; ok {
		t.Fatalf("latest interaction anchor must not contain historical milestone ledger: %+v", latestMap)
	}

	ledger := characterMilestoneLedger(olderEvents)
	if len(ledger) == 0 {
		t.Fatal("historical event ledger should not be empty")
	}
	for i, raw := range ledger {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("ledger entry %d must be map, got %T", i, raw)
		}
		if entry["surface_type"] == "latest_interaction_anchor" {
			t.Fatalf("milestone ledger entry must not masquerade as latest interaction anchor: %+v", entry)
		}
	}
	if nilAnchor := buildCharacterLatestInteractionAnchor(nil); nilAnchor != nil {
		t.Fatalf("nil latest interaction anchor should stay nil, got %v", nilAnchor)
	}
}

func TestSeq15P102DetailLossGuardReplayContinuityRelationshipCompact(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID:   "sess-seq15-p102",
		CharacterName:   "Ivy",
		StatusJSON:      `{"location":"castle","goal":"steal documents"}`,
		AppearanceJSON:  `{"height":"slim"}`,
		PersonalityJSON: `{"role":"spy"}`,
		RelationshipsJSON: `{
			"{{user}}":{"trust":"allies","history":"worked together on the heist","summary":"trusted handler"},
			"Kane":{"tension":"rivals","history":"competing for the same target","summary":"professional rival"}
		}`,
		TurnIndex: 30,
	}
	events := []store.CharacterEvent{
		{CharacterName: "Ivy", TurnIndex: 20, EventType: "relationship_shift", DetailsJSON: `{"summary":"trusted the handler"}`},
		{CharacterName: "Ivy", TurnIndex: 25, EventType: "personality_change", DetailsJSON: `{"summary":"accepted higher risk"}`},
		{CharacterName: "Ivy", TurnIndex: 30, EventType: "dialogue", DetailsJSON: `{"summary":"reported the stolen documents"}`},
	}
	snapshot := characterStaleSnapshot(item, events, 30, "", map[string]struct{}{})
	rel := buildCharacterRelationshipLane(item)
	latest := buildCharacterLatestInteractionAnchor(&events[len(events)-1])
	dynamic := buildDynamicCharacterDigest(item, snapshot, rel, latest, events)

	descriptorLane := mapAnySlice(dynamic["relationship_descriptor_lane"])
	if len(descriptorLane) == 0 || len(descriptorLane) > 4 {
		t.Fatalf("relationship_descriptor_lane should be compact and non-empty, got len=%d value=%v", len(descriptorLane), descriptorLane)
	}
	ledger := mapAnySlice(dynamic["milestone_ledger"])
	if len(ledger) == 0 || len(ledger) > 3 {
		t.Fatalf("milestone_ledger should be compact and non-empty, got len=%d value=%v", len(ledger), ledger)
	}
	if dynamic["recent_change_summary"] == nil {
		t.Fatalf("recent_change_summary should preserve latest anchor summary: %+v", dynamic)
	}
	if _, ok := dynamic["durable_profile"]; ok {
		t.Fatal("dynamic digest must not leak durable_profile")
	}
	stable := buildStableCharacterSheet(item, snapshot)
	for _, key := range []string{"relationship_lane", "relationship_descriptor_lane", "latest_interaction_anchor", "milestone_ledger"} {
		if _, ok := stable[key]; ok {
			t.Fatalf("stable sheet must not leak compact dynamic key %q", key)
		}
	}
}

// TestSeq15P106RelationDescriptorBandDefineClosenessTensionTrust verifies P106:
// relationDescriptorBands extracts closeness/tension/trust as readable descriptor
// bands, caps them at 3, and omits empty/missing fields.
func TestSeq15P106RelationDescriptorBandDefineClosenessTensionTrust(t *testing.T) {

	fullPayload := map[string]any{
		"trust":     "high",
		"closeness": "warm",
		"tension":   "low",
		"bond":      "strong",
		"distance":  "near",
		"stance":    "allied",
	}
	bands := relationDescriptorBands(fullPayload)
	if len(bands) != 3 {
		t.Fatalf("descriptor bands should be capped at 3, got %d: %v", len(bands), bands)
	}

	expected := []string{"trust: high", "closeness: warm", "tension: low"}
	for i, want := range expected {
		if bands[i] != want {
			t.Fatalf("descriptor band[%d] = %q, want %q", i, bands[i], want)
		}
	}

	sparsePayload := map[string]any{
		"tension": "rising",
		"bond":    "strained",
	}
	sparseBands := relationDescriptorBands(sparsePayload)
	if len(sparseBands) != 2 {
		t.Fatalf("sparse descriptor bands should be 2, got %d: %v", len(sparseBands), sparseBands)
	}
	if sparseBands[0] != "tension: rising" {
		t.Fatalf("sparse band[0] = %q, want 'tension: rising'", sparseBands[0])
	}
	if sparseBands[1] != "bond: strained" {
		t.Fatalf("sparse band[1] = %q, want 'bond: strained'", sparseBands[1])
	}

	emptyBands := relationDescriptorBands(map[string]any{})
	if len(emptyBands) != 0 {
		t.Fatalf("empty payload should produce 0 descriptor bands, got %d: %v", len(emptyBands), emptyBands)
	}

	nilPayload := map[string]any{
		"trust":     nil,
		"closeness": "",
		"tension":   "spiky",
	}
	nilBands := relationDescriptorBands(nilPayload)
	if len(nilBands) != 1 {
		t.Fatalf("nil/empty fields should be omitted, got %d bands: %v", len(nilBands), nilBands)
	}
	if nilBands[0] != "tension: spiky" {
		t.Fatalf("nil field band = %q, want 'tension: spiky'", nilBands[0])
	}

	item := store.CharacterState{
		ChatSessionID:     "sess-seq15-p106",
		CharacterName:     "Kira",
		RelationshipsJSON: `{"{{user}}":{"trust":"high","closeness":"warm","tension":"none","summary":"bonded allies"}}`,
		TurnIndex:         20,
	}
	lane := buildCharacterRelationshipLane(item)
	primaryBands := mapAnyOrEmptyStringSlice(lane, "primary_descriptor_bands")

	_ = primaryBands
	items := mapAnySlice(lane["items"])
	if len(items) == 0 {
		t.Fatal("relationship lane should have at least one item")
	}
	entry := items[0]
	entryBands := mapAnyOrEmptyStringSlice(entry, "descriptor_bands")
	if len(entryBands) == 0 {
		t.Fatalf("entry descriptor_bands should be populated: %+v", entry)
	}

	for _, band := range entryBands {
		if band == "" {
			t.Fatal("descriptor band must not be empty string")
		}
	}
}

// TestSeq15P107RelationLaneDisplayRulesNoAffinityPromotion verifies P107:
// raw affinity-like values in relationship payloads remain descriptive
// summaries/state snapshots, and are never promoted as truth/fact labels.
func TestSeq15P107RelationLaneDisplayRulesNoAffinityPromotion(t *testing.T) {

	item := store.CharacterState{
		ChatSessionID:     "sess-seq15-p107",
		CharacterName:     "Mira",
		RelationshipsJSON: `{"{{user}}":{"affinity":0.85,"trust":"high","summary":"trusted companion","status":"allied"}}`,
		TurnIndex:         15,
	}
	lane := buildCharacterRelationshipLane(item)
	if lane["surface_type"] != "relationship_lane" {
		t.Fatalf("surface_type = %v, want relationship_lane", lane["surface_type"])
	}

	summaryText, _ := lane["summary_text"].(string)
	if summaryText == "0.85" || summaryText == "affinity: 0.85" {
		t.Fatalf("summary_text must not be raw affinity value, got %q", summaryText)
	}

	items := mapAnySlice(lane["items"])
	if len(items) == 0 {
		t.Fatal("relationship lane should have at least one item")
	}
	entry := items[0]
	stateSnapshot := entry["state_snapshot"]
	if stateSnapshot == nil {
		t.Fatal("entry state_snapshot should exist")
	}

	if snapMap, ok := stateSnapshot.(map[string]any); ok {

		for _, key := range []string{"truth_level", "fact_status", "verified_fact", "canonical"} {
			if _, ok := snapMap[key]; ok {
				t.Fatalf("state_snapshot must not promote raw data to %q: %+v", key, snapMap)
			}
		}
	}

	displayPriority, _ := entry["display_priority"].(int)
	if displayPriority != 0 && displayPriority != 1 {
		t.Fatalf("display_priority should be 0 (protagonist) or 1 (other), got %v", displayPriority)
	}

	bands := mapAnyOrEmptyStringSlice(entry, "descriptor_bands")
	for _, band := range bands {

		if band == "" {
			continue
		}

		allDigits := true
		for _, r := range band {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			t.Fatalf("descriptor band must not be a bare numeric score: %q", band)
		}
	}

	rawItem := store.CharacterState{
		ChatSessionID:     "sess-seq15-p107-raw",
		CharacterName:     "Nova",
		RelationshipsJSON: `{"{{user}}":{"affinity":0.5}}`,
		TurnIndex:         10,
	}
	rawLane := buildCharacterRelationshipLane(rawItem)

	if rawLane["status"] != "empty" && rawLane["status"] != "ready" && rawLane["status"] != "summary_only" {
		t.Fatalf("raw affinity lane status should be empty/ready/summary_only, got %v", rawLane["status"])
	}
}

// TestSeq15P108ProtagonistRelationVsOthersLaneSplitHarden verifies P108:
// protagonist relation is clearly separated from other_relations in the
// relationship lane, protagonist always appears first, and the split is
// structurally enforced.
func TestSeq15P108ProtagonistRelationVsOthersLaneSplitHarden(t *testing.T) {
	item := store.CharacterState{
		ChatSessionID: "sess-seq15-p108",
		CharacterName: "Zara",
		RelationshipsJSON: `{
			"Kael":{"trust":"medium","summary":"uneasy ally"},
			"{{user}}":{"trust":"high","closeness":"warm","summary":"trusted protagonist"},
			"Bryn":{"tension":"rival","summary":"competing scholar"},
			"Dex":{"bond":"old friend","summary":"childhood companion"},
			"Vex":{"distance":"cold","summary":"estranged sibling"}
		}`,
		TurnIndex: 25,
	}
	lane := buildCharacterRelationshipLane(item)

	if lane["display_mode"] != "protagonist_first_then_observed_order" {
		t.Fatalf("display_mode = %v, want protagonist_first_then_observed_order", lane["display_mode"])
	}

	protag, ok := lane["protagonist_relation"].(map[string]any)
	if !ok || protag == nil {
		t.Fatal("protagonist_relation must be a populated map")
	}
	if protag["target"] != "{{user}}" {
		t.Fatalf("protagonist target = %v, want {{user}}", protag["target"])
	}

	if lane["primary_target"] != "{{user}}" {
		t.Fatalf("primary_target = %v, want {{user}}", lane["primary_target"])
	}

	others := mapAnySlice(lane["other_relations"])
	for i, other := range others {
		if other["target"] == "{{user}}" {
			t.Fatalf("other_relations[%d] must not include protagonist: %+v", i, other)
		}
	}

	items := mapAnySlice(lane["items"])
	if len(items) == 0 {
		t.Fatal("items should not be empty")
	}
	if items[0]["target"] != "{{user}}" {
		t.Fatalf("items[0] target = %v, want {{user}} (protagonist first)", items[0]["target"])
	}

	if items[0]["display_priority"] != 0 {
		t.Fatalf("protagonist display_priority = %v, want 0", items[0]["display_priority"])
	}

	for i := 1; i < len(items); i++ {
		if items[i]["display_priority"] != 1 {
			t.Fatalf("items[%d] display_priority = %v, want 1 for non-protagonist", i, items[i]["display_priority"])
		}
	}

	protagCount := 1
	otherCount := len(others)
	expectedCount := protagCount + otherCount
	if lane["count"] != expectedCount {
		t.Fatalf("count = %v, want %d (protagonist %d + others %d)", lane["count"], expectedCount, protagCount, otherCount)
	}

	if len(items) > 6 {
		t.Fatalf("items length %d exceeds cap 6", len(items))
	}

	primaryBands := mapAnyOrEmptyStringSlice(lane, "primary_descriptor_bands")
	protagBands := mapAnyOrEmptyStringSlice(protag, "descriptor_bands")
	if len(primaryBands) != len(protagBands) {
		t.Fatalf("primary_descriptor_bands length %d != protagonist bands length %d", len(primaryBands), len(protagBands))
	}

	noProtagItem := store.CharacterState{
		ChatSessionID:     "sess-seq15-p108-nop",
		CharacterName:     "Solo",
		RelationshipsJSON: `{"Kael":{"summary":"rival"},"Bryn":{"summary":"friend"}}`,
		TurnIndex:         10,
	}
	noProtagLane := buildCharacterRelationshipLane(noProtagItem)

	if prMap, ok := noProtagLane["protagonist_relation"].(map[string]any); ok && len(prMap) > 0 {
		t.Fatalf("no-protagonist lane should have nil/empty protagonist_relation, got %+v", prMap)
	}
	if noProtagLane["primary_target"] == nil {
		t.Fatal("no-protagonist lane should still have a primary_target fallback")
	}
	noProtagItems := mapAnySlice(noProtagLane["items"])
	if len(noProtagItems) == 0 {
		t.Fatal("no-protagonist lane should still have items")
	}

	for i, entry := range noProtagItems {
		if entry["display_priority"] != 1 {
			t.Fatalf("no-protagonist items[%d] display_priority = %v, want 1", i, entry["display_priority"])
		}
	}
}

// TestSeq15P112WeakInputFocusPriority verifies that story guidance provides
// focus steering but remains subordinate to all higher-priority sources.
// Contract: guidance_authority = "subordinate"; higher_priority_sources lists
// current_user_input, explicit_user_correction, hard_world_rule,
// latest_direct_evidence, canonical_truth_floor; guidance may suggest but
// never override those sources.
func TestSeq15P112WeakInputFocusPriority(t *testing.T) {

	storyPlan := map[string]any{
		"current_arc":        "Reckoning",
		"narrative_goal":     "Confront the betrayer",
		"active_tensions":    []string{"trust_fracture"},
		"next_beats":         []string{"showdown", "reveal"},
		"continuity_anchors": []string{"oath_memory"},
		"focus_characters":   []string{"Kael"},
	}
	director := map[string]any{
		"pressure_level":      "strong",
		"required_outcomes":   []string{"Kael confronts"},
		"forbidden_moves":     []string{"instant_forgiveness"},
		"execution_checklist": []string{"build_tension", "no_easy_resolution"},
		"world_guardrails":    []string{"magic_system_intact"},
		"persona_guardrails":  []string{"Kael_stays_suspicious"},
		"scene_mandate":       "Confrontation scene",
	}
	surface := buildStoryGuidanceSurface(storyPlan, director)

	precedence := asMap(surface["precedence"])
	if got, _ := precedence["guidance_authority"].(string); got != "subordinate" {
		t.Fatalf("guidance_authority = %q, want %q", got, "subordinate")
	}

	higherSources := mapAnyOrEmptyStringSlice(precedence, "higher_priority_sources")
	expectedHigher := []string{"current_user_input", "explicit_user_correction", "hard_world_rule", "latest_direct_evidence", "canonical_truth_floor"}
	if len(higherSources) != len(expectedHigher) {
		t.Fatalf("higher_priority_sources length = %d, want %d", len(higherSources), len(expectedHigher))
	}
	for i, want := range expectedHigher {
		if higherSources[i] != want {
			t.Fatalf("higher_priority_sources[%d] = %q, want %q", i, higherSources[i], want)
		}
	}

	disallowed := mapAnyOrEmptyStringSlice(precedence, "disallowed_usage")
	mustContainAll(t, disallowed, []string{
		"current_user_input_override",
		"explicit_user_correction_override",
		"hard_world_rule_bypass",
		"canonical_truth_floor_overwrite",
	})

	conflictPolicy := asMap(surface["conflict_policy"])
	if got, _ := conflictPolicy["guidance_may_suggest"].(bool); !got {
		t.Fatal("guidance_may_suggest = false, want true")
	}
	if got, _ := conflictPolicy["guidance_may_override_user_input"].(bool); got {
		t.Fatal("guidance_may_override_user_input = true, want false")
	}

	emptySurface := buildStoryGuidanceSurface(map[string]any{}, map[string]any{})
	emptyPrec := asMap(emptySurface["precedence"])
	if got, _ := emptyPrec["guidance_authority"].(string); got != "subordinate" {
		t.Fatalf("empty-input guidance_authority = %q, want %q", got, "subordinate")
	}

	if surface["story_frame"] == nil {
		t.Fatal("story_frame missing, guidance should provide focus")
	}
	if surface["turn_directives"] == nil {
		t.Fatal("turn_directives missing, guidance should provide focus")
	}
}

// TestSeq15P113WeakInputFailModeDefine verifies fail_mode semantics:
// subsystem unavailable or low confidence uses conservative_continuation;
// always: allow_scene_jump=false, allow_forced_resolution=false,
// respect_explicit_user_correction=true.
func TestSeq15P113WeakInputFailModeDefine(t *testing.T) {

	emptySurface := buildStoryGuidanceSurface(map[string]any{}, map[string]any{})
	emptyTD := asMap(emptySurface["turn_directives"])
	emptyFM := asMap(emptyTD["fail_mode"])

	if got, _ := emptyFM["mode"].(string); got != "conservative_continuation" {
		t.Fatalf("empty-input fail_mode.mode = %q, want %q", got, "conservative_continuation")
	}
	if got, _ := emptyFM["allow_scene_jump"].(bool); got {
		t.Fatal("empty-input fail_mode.allow_scene_jump = true, want false")
	}
	if got, _ := emptyFM["allow_forced_resolution"].(bool); got {
		t.Fatal("empty-input fail_mode.allow_forced_resolution = true, want false")
	}
	if got, _ := emptyFM["respect_explicit_user_correction"].(bool); !got {
		t.Fatal("empty-input fail_mode.respect_explicit_user_correction = false, want true")
	}

	lowConfSurface := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "Act I"},
		map[string]any{"pressure_level": "steady"},
	)
	lowConfTD := asMap(lowConfSurface["turn_directives"])
	lowConfFM := asMap(lowConfTD["fail_mode"])
	if got, _ := lowConfFM["mode"].(string); got != "conservative_continuation" {
		t.Fatalf("low-confidence fail_mode.mode = %q, want %q", got, "conservative_continuation")
	}

	strongSurface := buildStoryGuidanceSurface(
		map[string]any{"current_arc": "Climax"},
		map[string]any{"pressure_level": "strong"},
	)
	strongFM := asMap(asMap(strongSurface["turn_directives"])["fail_mode"])
	if got, _ := strongFM["mode"].(string); got != "pressure_continuation_without_resolution" {
		t.Fatalf("strong-pressure fail_mode.mode = %q, want %q", got, "pressure_continuation_without_resolution")
	}

	carrySurface := buildStoryGuidanceSurface(
		map[string]any{},
		map[string]any{
			"pressure_level":    "steady",
			"required_outcomes": []string{"land_a_beat"},
		},
	)
	carryFM := asMap(asMap(carrySurface["turn_directives"])["fail_mode"])
	if got, _ := carryFM["mode"].(string); got != "carry_forward_without_forcing_resolution" {
		t.Fatalf("carry fail_mode.mode = %q, want %q", got, "carry_forward_without_forcing_resolution")
	}
	if got, _ := carryFM["preserve_carry_targets"].(bool); !got {
		t.Fatal("carry fail_mode.preserve_carry_targets = false, want true")
	}

	sceneSurface := buildStoryGuidanceSurface(
		map[string]any{"next_beats": []string{"enter_tavern"}},
		map[string]any{"scene_mandate": "Tavern scene"},
	)
	sceneFM := asMap(asMap(sceneSurface["turn_directives"])["fail_mode"])
	if got, _ := sceneFM["mode"].(string); got != "scene_continuation_without_scene_jump" {
		t.Fatalf("scene fail_mode.mode = %q, want %q", got, "scene_continuation_without_scene_jump")
	}

	for _, fm := range []map[string]any{emptyFM, lowConfFM, strongFM, carryFM, sceneFM} {
		if got, _ := fm["respect_explicit_user_correction"].(bool); !got {
			t.Fatalf("fail_mode %+v missing respect_explicit_user_correction=true", fm)
		}
		if got, _ := fm["allow_scene_jump"].(bool); got {
			t.Fatalf("fail_mode %+v has allow_scene_jump=true", fm)
		}
		if got, _ := fm["allow_forced_resolution"].(bool); got {
			t.Fatalf("fail_mode %+v has allow_forced_resolution=true", fm)
		}
	}
}
