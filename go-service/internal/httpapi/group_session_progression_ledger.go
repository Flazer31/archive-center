package httpapi

import (
	"fmt"
	"strings"
)

func buildNarrativeControlProgressionLedger(stateStatus string, director map[string]any, storyPlan map[string]any, lastTurn int) map[string]any {
	pressureLevel, _ := director["pressure_level"].(string)
	if pressureLevel == "" {
		pressureLevel = "light"
	}
	status := stateStatus
	if status == "" {
		status = "skeleton"
	}
	pendingBeats := []string{}
	if beats, ok := storyPlan["next_beats"].([]string); ok {
		for _, beat := range beats {
			b := strings.TrimSpace(beat)
			if b != "" && !containsString(pendingBeats, b) {
				pendingBeats = append(pendingBeats, b)
			}
		}
	}
	consumedBeats := []string{}
	if resolved, ok := director["resolved_outcomes"].([]string); ok {
		for _, item := range resolved {
			b := strings.TrimSpace(item)
			if b != "" && !containsString(consumedBeats, b) {
				consumedBeats = append(consumedBeats, b)
			}
		}
	}

	lastAdvancedTurn := any(nil)
	lastValidatedTurn := any(nil)
	ledgerStatus := "skeleton"
	if lastTurn > 0 || len(pendingBeats) > 0 || len(consumedBeats) > 0 {
		ledgerStatus = "tracking"
	}
	if lastTurn > 0 {
		lastAdvancedTurn = lastTurn
		if stateStatus == "ready" || stateStatus == "user_patched" {
			lastValidatedTurn = lastTurn
		}
	}

	pendingBeatsAny := make([]any, 0, len(pendingBeats))
	for _, b := range pendingBeats[:minInt(len(pendingBeats), 8)] {
		pendingBeatsAny = append(pendingBeatsAny, b)
	}
	consumedBeatsAny := make([]any, 0, len(consumedBeats))
	for _, b := range consumedBeats[:minInt(len(consumedBeats), 8)] {
		consumedBeatsAny = append(consumedBeatsAny, b)
	}

	consumedSet := map[string]bool{}
	for _, b := range consumedBeats {
		consumedSet[strings.ToLower(b)] = true
	}

	doNotResolveGuard := map[string]any{
		"status":                "active",
		"mode":                  "deterministic_no_llm",
		"min_turn_gap":          2,
		"protected_entry_types": []string{"unresolved_tension", "payoff"},
		"protected_sources":     []string{"story_plan.next_beats", "director.required_outcomes"},
		"long_horizon_tokens":   []string{"promise", "payoff", "callback", "\u003f\uC38C\uB0FD", "\u8E42\uB4ED\uAF51", "\u003f\uB6AF\uB2D4"},
	}

	lifecycleModel := map[string]any{
		"status":         "active",
		"states":         []string{"latent", "active", "escalating", "aftermath", "resolved", "dormant"},
		"pressure_scale": map[string]any{"min": 0, "max": 3},
		"decay_rules":    map[string]any{"latent": 5, "active": 4, "escalating": 3, "aftermath": 2, "resolved": 1, "dormant": 0},
		"mode":           "deterministic_no_llm",
	}

	unresolvedTensions := []any{}
	for _, beat := range pendingBeats {
		label := normalizeStoryLedgerLabel(beat)
		if label == "" {
			continue
		}
		anchor := buildLedgerAnchor(label, storyPlan, director)
		pressureScore, decayTurns := lifecycleProfileForState("latent", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "unresolved_tension",
			"label":              label,
			"source":             "story_plan.next_beats",
			"status":             "open",
			"lifecycle_state":    "latent",
			"pressure_score":     pressureScore,
			"decay_turns":        decayTurns,
			"deterministic":      true,
			"source_record_id":   nil,
			"source_message_ids": []any{},
			"affected_relations": anchor["affected_relations"],
			"affected_world":     anchor["affected_world"],
		}
		attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
		unresolvedTensions = append(unresolvedTensions, entry)
	}
	if requiredOutcomes, ok := director["required_outcomes"].([]string); ok {
		for _, item := range requiredOutcomes {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			if consumedSet[strings.ToLower(label)] {
				continue
			}
			dup := false
			for _, existing := range unresolvedTensions {
				if m, ok := existing.(map[string]any); ok && m["label"] == label {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "unresolved_tension",
				"label":              label,
				"source":             "director.required_outcomes",
				"status":             "open",
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
			unresolvedTensions = append(unresolvedTensions, entry)
		}
	}
	if len(unresolvedTensions) > 12 {
		unresolvedTensions = unresolvedTensions[:12]
	}

	consequences := []any{}
	if executionNotes, ok := storyPlan["execution_notes"].([]string); ok {
		for _, item := range executionNotes {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("escalating", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "story_plan.execution_notes",
				"status":             "pending",
				"lifecycle_state":    "escalating",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
			consequences = append(consequences, entry)
		}
	}
	if executionChecklist, ok := director["execution_checklist"].([]string); ok {
		for _, item := range executionChecklist {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			dup := false
			for _, existing := range consequences {
				if m, ok := existing.(map[string]any); ok && m["label"] == label {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			consequences = append(consequences, map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "director.execution_checklist",
				"status":             "pending",
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			})
		}
	}
	if len(consequences) > 12 {
		consequences = consequences[:12]
	}

	sceneDeltas := []any{}
	if sceneMandate, ok := director["scene_mandate"].(string); ok && strings.TrimSpace(sceneMandate) != "" {
		label := normalizeStoryLedgerLabel(sceneMandate)
		if label != "" {
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "scene_delta",
				"label":              label,
				"source":             "director.scene_mandate",
				"status":             "observed",
				"turn_hint":          lastTurn,
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			sceneDeltas = append(sceneDeltas, entry)
		}
	}
	if pressureLevel != "" {
		label := "pressure=" + pressureLevel
		anchor := buildLedgerAnchor(label, storyPlan, director)
		pressureScore, decayTurns := lifecycleProfileForState("escalating", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "scene_delta",
			"label":              label,
			"source":             "director.pressure_level",
			"status":             "observed",
			"turn_hint":          lastTurn,
			"lifecycle_state":    "escalating",
			"pressure_score":     pressureScore,
			"decay_turns":        decayTurns,
			"deterministic":      true,
			"source_record_id":   nil,
			"source_message_ids": []any{},
			"affected_relations": anchor["affected_relations"],
			"affected_world":     anchor["affected_world"],
		}
		sceneDeltas = append(sceneDeltas, entry)
	}
	if len(sceneDeltas) > 8 {
		sceneDeltas = sceneDeltas[:8]
	}

	worldPressure := buildWorldPressure(storyPlan, director, pendingBeats, consumedBeats, lastTurn)

	return map[string]any{
		"status":                               ledgerStatus,
		"last_advanced_turn":                   lastAdvancedTurn,
		"last_validated_turn":                  lastValidatedTurn,
		"consumed_beats":                       consumedBeatsAny,
		"pending_beats":                        pendingBeatsAny,
		"invalidation_reason":                  nil,
		"ledger_policy_version":                "lw1h.v1",
		"ledger_mode":                          "deterministic_no_llm",
		"unresolved_tensions":                  unresolvedTensions,
		"consequences":                         consequences,
		"payoffs":                              []any{},
		"scene_deltas":                         sceneDeltas,
		"world_pressure_policy_version":        "lw1d.v1",
		"world_pressure":                       worldPressure,
		"continuity_precedence_policy_version": "lw1e.v1",
		"supporting_precedence_guard": map[string]any{
			"status":                                   "supporting_only",
			"supporting_only":                          true,
			"cannot_override_current_user_input":       true,
			"cannot_override_verified_direct_evidence": true,
			"precedence_ceiling":                       "below_current_user_input_and_verified_direct_evidence",
			"allowed_usage":                            []string{"continuity_hint", "narrative_support"},
			"disallowed_usage":                         []string{"truth_overwrite", "canonical_override"},
		},
		"compatibility_policy_version": "lw1f.v1",
		"compatibility_contract": map[string]any{
			"status":           "compatible",
			"targets":          []string{"chapter_summary", "arc_summary", "continuity_pack"},
			"shape_mode":       "additive_non_breaking",
			"consumer_safe":    true,
			"adapter_required": false,
		},
		"lifecycle_policy_version":      "lw1g.v1",
		"lifecycle_model":               lifecycleModel,
		"do_not_resolve_policy_version": "lw1h.v1",
		"do_not_resolve_guard":          doNotResolveGuard,
	}
}

func normalizeStoryLedgerLabel(raw any) string {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	return text
}

func lifecycleProfileForState(state string, lifecycleModel map[string]any) (int, int) {
	pressureMap := map[string]int{"latent": 1, "active": 2, "escalating": 3, "aftermath": 1, "resolved": 0, "dormant": 0}
	pressure := pressureMap[state]
	if pressure == 0 && state != "resolved" && state != "dormant" {
		pressure = 1
	}
	decayRules, _ := lifecycleModel["decay_rules"].(map[string]any)
	decay := 2
	if dr, ok := decayRules[state]; ok {
		if di, ok := dr.(int); ok {
			decay = di
		}
	}
	return pressure, decay
}

func buildLedgerAnchor(label string, storyPlanData map[string]any, directorData map[string]any) map[string]any {
	return map[string]any{
		"source_record_id":   nil,
		"source_message_ids": []any{},
		"affected_relations": deriveAnchorRelations(label, storyPlanData, directorData),
		"affected_world":     deriveAnchorWorld(label, storyPlanData, directorData),
	}
}

func deriveAnchorRelations(label string, storyPlanData map[string]any, directorData map[string]any) []any {
	candidates := []string{}
	for _, raw := range asStringSlice(directorData["focus_characters"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	for _, raw := range asStringSlice(storyPlanData["focus_characters"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	lowered := strings.ToLower(label)
	matched := []string{}
	for _, name := range candidates {
		if strings.Contains(lowered, strings.ToLower(name)) {
			matched = append(matched, name)
		}
	}
	if len(matched) > 0 {
		if len(matched) > 4 {
			matched = matched[:4]
		}
		out := make([]any, len(matched))
		for i, m := range matched {
			out[i] = m
		}
		return out
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]any, len(candidates))
	for i, c := range candidates {
		out[i] = c
	}
	return out
}

func deriveAnchorWorld(label string, storyPlanData map[string]any, directorData map[string]any) []any {
	candidates := []string{}
	for _, raw := range asStringSlice(directorData["world_guardrails"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	for _, raw := range asStringSlice(storyPlanData["guardrails"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	currentArc := normalizeStoryLedgerLabel(storyPlanData["current_arc"])
	if currentArc != "" && !containsString(candidates, currentArc) {
		candidates = append(candidates, currentArc)
	}
	lowered := strings.ToLower(label)
	matched := []string{}
	for _, item := range candidates {
		if strings.Contains(lowered, strings.ToLower(item)) {
			matched = append(matched, item)
		}
	}
	if len(matched) > 0 {
		if len(matched) > 4 {
			matched = matched[:4]
		}
		out := make([]any, len(matched))
		for i, m := range matched {
			out[i] = m
		}
		return out
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]any, len(candidates))
	for i, c := range candidates {
		out[i] = c
	}
	return out
}

func buildWorldPressure(storyPlanData map[string]any, directorData map[string]any, pendingBeats []string, consumedBeats []string, lastTurn int) map[string]any {
	buckets := map[string][]map[string]any{
		"factions":          {},
		"regions":           {},
		"offscreen_threads": {},
		"public_pressure":   {},
	}
	appendBucket := func(bucket string, label string, source string, pressureState string) {
		if label == "" {
			return
		}
		for _, item := range buckets[bucket] {
			if item["label"] == label {
				return
			}
		}
		buckets[bucket] = append(buckets[bucket], map[string]any{
			"label":          label,
			"source":         source,
			"pressure_state": pressureState,
			"deterministic":  true,
		})
	}
	for _, raw := range asStringSlice(directorData["world_guardrails"]) {
		label := normalizeStoryLedgerLabel(raw)
		appendBucket(classifyWorldPressureBucket(label), label, "director.world_guardrails", "active")
	}
	for _, raw := range asStringSlice(storyPlanData["execution_notes"]) {
		label := normalizeStoryLedgerLabel(raw)
		appendBucket(classifyWorldPressureBucket(label), label, "story_plan.execution_notes", "escalating")
	}
	for _, beat := range pendingBeats {
		label := normalizeStoryLedgerLabel(beat)
		appendBucket("offscreen_threads", label, "story_plan.next_beats", "latent")
	}
	for _, beat := range consumedBeats {
		label := normalizeStoryLedgerLabel(beat)
		appendBucket("public_pressure", label, "director.resolved_outcomes", "aftermath")
	}
	return map[string]any{
		"status":            "structured_support",
		"factions":          mapSliceToAny(buckets["factions"])[:minInt(len(buckets["factions"]), 10)],
		"regions":           mapSliceToAny(buckets["regions"])[:minInt(len(buckets["regions"]), 10)],
		"offscreen_threads": mapSliceToAny(buckets["offscreen_threads"])[:minInt(len(buckets["offscreen_threads"]), 10)],
		"public_pressure":   mapSliceToAny(buckets["public_pressure"])[:minInt(len(buckets["public_pressure"]), 10)],
		"timeline": []any{
			map[string]any{
				"turn":              lastTurn,
				"marker":            "world_pressure_snapshot",
				"factions":          len(buckets["factions"]),
				"regions":           len(buckets["regions"]),
				"offscreen_threads": len(buckets["offscreen_threads"]),
				"public_pressure":   len(buckets["public_pressure"]),
			},
		},
	}
}

func isLongHorizonCandidate(label string, source string, guard map[string]any) bool {
	normalized := strings.TrimSpace(strings.ToLower(label))
	sourceKey := strings.TrimSpace(strings.ToLower(source))
	if normalized == "" {
		return false
	}
	tokens := []string{}
	if raw, ok := guard["long_horizon_tokens"].([]string); ok {
		for _, item := range raw {
			t := strings.TrimSpace(strings.ToLower(item))
			if t != "" {
				tokens = append(tokens, t)
			}
		}
	}
	for _, token := range tokens {
		if token != "" && strings.Contains(normalized, token) {
			return true
		}
	}
	protectedSources := map[string]bool{}
	if raw, ok := guard["protected_sources"].([]string); ok {
		for _, item := range raw {
			s := strings.TrimSpace(strings.ToLower(item))
			if s != "" {
				protectedSources[s] = true
			}
		}
	}
	return protectedSources[sourceKey]
}

func attachDoNotResolveFields(entry map[string]any, guard map[string]any, lastTurn int) {
	label := asString(entry["label"])
	source := asString(entry["source"])
	shouldProtect := isLongHorizonCandidate(label, source, guard)
	minTurnGap := 0
	if v, ok := guard["min_turn_gap"].(int); ok {
		minTurnGap = v
	}
	baseTurn := lastTurn
	if baseTurn < 0 {
		baseTurn = 0
	}
	if shouldProtect {
		entry["do_not_resolve_yet"] = true
		entry["resolve_guard_reason"] = "long_horizon_candidate"
		entry["resolve_earliest_turn"] = baseTurn + minTurnGap
	} else {
		entry["do_not_resolve_yet"] = false
		entry["resolve_guard_reason"] = nil
		entry["resolve_earliest_turn"] = nil
	}
}

func classifyWorldPressureBucket(label string) string {
	lowered := strings.ToLower(label)
	if strings.Contains(lowered, "faction") || strings.Contains(lowered, "guild") || strings.Contains(lowered, "clan") || strings.Contains(lowered, "house") || strings.Contains(lowered, "family") || strings.Contains(lowered, "group") {
		return "factions"
	}
	if strings.Contains(lowered, "region") || strings.Contains(lowered, "city") || strings.Contains(lowered, "village") || strings.Contains(lowered, "harbor") || strings.Contains(lowered, "capital") || strings.Contains(lowered, "area") || strings.Contains(lowered, "town") {
		return "regions"
	}
	if strings.Contains(lowered, "offscreen") || strings.Contains(lowered, "elsewhere") || strings.Contains(lowered, "distant") {
		return "offscreen_threads"
	}
	if strings.Contains(lowered, "public") || strings.Contains(lowered, "rumor") || strings.Contains(lowered, "panic") || strings.Contains(lowered, "trust") {
		return "public_pressure"
	}
	return "public_pressure"
}

func characterEventPriority(eventType string) int {
	switch eventType {
	case "relationship_shift":
		return 0
	case "personality_change":
		return 1
	case "appearance_change":
		return 2
	case "status_change":
		return 3
	default:
		return 4
	}
}

func buildStoryGuidanceSurface(storyPlan map[string]any, director map[string]any) map[string]any {
	pressureLevel := asString(director["pressure_level"])
	if pressureLevel == "" {
		pressureLevel = "steady"
	}
	currentArc := asString(storyPlan["current_arc"])
	narrativeGoal := asString(storyPlan["narrative_goal"])
	activeTensions := asStringSlice(storyPlan["active_tensions"])
	nextBeats := asStringSlice(storyPlan["next_beats"])
	anchors := asStringSlice(storyPlan["continuity_anchors"])
	focusCharacters := asStringSlice(storyPlan["focus_characters"])
	required := asStringSlice(director["required_outcomes"])
	forbidden := asStringSlice(director["forbidden_moves"])
	executionChecklist := asStringSlice(director["execution_checklist"])
	worldGuardrails := asStringSlice(director["world_guardrails"])
	personaGuardrails := asStringSlice(director["persona_guardrails"])
	sceneDrive := asString(director["scene_mandate"])

	ending := "End on a conservative continuation edge without forcing a hard scene jump."
	if pressureLevel == "strong" || pressureLevel == "critical" {
		ending = "End on a visible pressure beat without forcing a full resolution."
	} else if len(required) > 0 {
		ending = "Land at least one visible beat before ending, while keeping unresolved carry targets open."
	} else if len(nextBeats) > 0 || sceneDrive != "" {
		ending = "End on a clear continuation edge that preserves the active scene drive."
	}

	storyFrame := map[string]any{
		"stage_type":           "story_frame",
		"arc_focus":            currentArc,
		"narrative_drive":      narrativeGoal,
		"live_tensions":        nonNilSlice(activeTensions),
		"beat_queue":           nonNilSlice(nextBeats),
		"carry_threads":        nonNilSlice(anchors),
		"spotlight_characters": nonNilSlice(focusCharacters),
	}
	storyFrame["status"] = surfaceStatus(map[string]any{
		"arc_focus":       storyFrame["arc_focus"],
		"narrative_drive": storyFrame["narrative_drive"],
		"live_tensions":   storyFrame["live_tensions"],
		"beat_queue":      storyFrame["beat_queue"],
		"carry_threads":   storyFrame["carry_threads"],
	})

	turnDirectives := map[string]any{
		"stage_type":           "turn_directives",
		"scene_drive":          sceneDrive,
		"carry_targets":        nonNilSlice(required),
		"blocked_routes":       nonNilSlice(forbidden),
		"tempo_band":           pressureLevel,
		"handoff_edge":         ending,
		"turn_checklist":       nonNilSlice(limitStrings(executionChecklist, 4)),
		"voice_guardrails":     nonNilSlice(personaGuardrails),
		"setting_guardrails":   nonNilSlice(worldGuardrails),
		"spotlight_characters": nonNilSlice(focusCharacters),
	}
	turnDirectives["execution_contract"] = map[string]any{
		"must_hit":           limitStrings(required, 4),
		"forbidden":          limitStrings(forbidden, 4),
		"pacing_pressure":    pressureLevel,
		"ending_requirement": ending,
		"continuity_lock":    firstNonEmptyString(anchors),
	}
	failMode := "conservative_continuation"
	if pressureLevel == "strong" || pressureLevel == "critical" {
		failMode = "pressure_continuation_without_resolution"
	} else if len(required) > 0 {
		failMode = "carry_forward_without_forcing_resolution"
	} else if sceneDrive != "" || len(nextBeats) > 0 || narrativeGoal != "" {
		failMode = "scene_continuation_without_scene_jump"
	}
	turnDirectives["fail_mode"] = map[string]any{
		"mode":                             failMode,
		"allow_scene_jump":                 false,
		"allow_forced_resolution":          false,
		"respect_explicit_user_correction": true,
		"preserve_carry_targets":           len(required) > 0,
	}
	turnDirectives["status"] = surfaceStatus(map[string]any{
		"scene_drive":    turnDirectives["scene_drive"],
		"carry_targets":  turnDirectives["carry_targets"],
		"blocked_routes": turnDirectives["blocked_routes"],
		"handoff_edge":   turnDirectives["handoff_edge"],
	})

	statusInputs := map[string]any{
		"story_frame":     nil,
		"turn_directives": nil,
	}
	if storyFrame["status"] != "empty" {
		statusInputs["story_frame"] = storyFrame
	}
	if turnDirectives["status"] != "empty" {
		statusInputs["turn_directives"] = turnDirectives
	}
	status := surfaceStatus(statusInputs)

	return map[string]any{
		"surface_version": "sg14a.v1",
		"surface_type":    "story_guidance_surface",
		"status":          status,
		"story_frame":     storyFrame,
		"turn_directives": turnDirectives,
		"conflict_policy": map[string]any{
			"policy_version":                   "sg14a-conflict.v1",
			"current_user_input_wins":          true,
			"explicit_user_correction_wins":    true,
			"guidance_may_suggest":             true,
			"guidance_may_override_user_input": false,
			"on_conflict":                      "yield_to_current_user_input",
		},
		"precedence": map[string]any{
			"policy_version":          "sg14a.v1",
			"status":                  "fixed",
			"guidance_authority":      "subordinate",
			"higher_priority_sources": []string{"current_user_input", "explicit_user_correction", "hard_world_rule", "latest_direct_evidence", "canonical_truth_floor"},
			"disallowed_usage":        []string{"current_user_input_override", "explicit_user_correction_override", "hard_world_rule_bypass", "canonical_truth_floor_overwrite"},
			"precedence_note":         "Story guidance is a subordinate planning surface. Follow current user input, explicit user corrections, direct evidence, canonical truth, and hard world rules first.",
		},
	}
}

func firstNonEmptyString(items []string) string {
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
