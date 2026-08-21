package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type perspectiveIdentityTestStore struct {
	*turnRecordingStore
	resolvedSurface   string
	resolvedID        string
	resolvedNamespace string
	resolveErr        error
}

func (s *perspectiveIdentityTestStore) ResolveUniqueActiveEntityIDBySurface(_ context.Context, _ string, surface string) (string, error) {
	s.resolvedSurface = surface
	return s.resolvedID, s.resolveErr
}

func (s *perspectiveIdentityTestStore) ResolveUniqueActiveEntityIdentityBySurface(_ context.Context, _ string, surface string) (store.ResolvedEntityIdentity, error) {
	s.resolvedSurface = surface
	return store.ResolvedEntityIdentity{
		StableEntityID: s.resolvedID, IdentityNamespace: s.resolvedNamespace,
	}, s.resolveErr
}

func TestPrepareTurnPerspectiveIdentityUsesExactSourceSurfaceAndFailsClosed(t *testing.T) {
	st := &perspectiveIdentityTestStore{
		turnRecordingStore: &turnRecordingStore{},
		resolvedID:         "entity-gloria",
	}
	resolved := resolvePrepareTurnPerspectiveIdentity(
		context.Background(), st, "session",
		map[string]any{"current_pov": "글로리아", "source": "client_meta"},
	)
	if got := extractionStringFromAny(resolved["current_pov_entity_id"]); got != "entity-gloria" ||
		extractionStringFromAny(resolved["identity_state"]) != "resolved" {
		t.Fatalf("unique POV surface did not resolve: %#v", resolved)
	}
	if st.resolvedSurface != comparableEntityKey("글로리아") {
		t.Fatalf("resolver surface=%q want exact identity normalization %q", st.resolvedSurface, comparableEntityKey("글로리아"))
	}

	st.resolveErr = store.ErrReviewedEntityIdentityAmbiguous
	ambiguous := resolvePrepareTurnPerspectiveIdentity(
		context.Background(), st, "session",
		map[string]any{"current_pov": "Alex", "current_pov_entity_id": "forged"},
	)
	if extractionStringFromAny(ambiguous["identity_state"]) != "needs_review" ||
		extractionStringFromAny(ambiguous["current_pov_entity_id"]) != "" {
		t.Fatalf("ambiguous POV did not fail closed: %#v", ambiguous)
	}

	st.resolveErr = errors.New("read failed")
	unobserved := resolvePrepareTurnPerspectiveIdentity(
		context.Background(), st, "session",
		map[string]any{"current_pov": "Narrator"},
	)
	if extractionStringFromAny(unobserved["identity_state"]) != "unobserved" ||
		extractionStringFromAny(unobserved["current_pov_entity_id"]) != "" {
		t.Fatalf("unobserved narrator gained identity: %#v", unobserved)
	}
}

func TestPrepareTurnPerspectiveUsesObservedRisuPersonaAsKnowledgeHolder(t *testing.T) {
	context := prepareTurnPerspectiveContextFromClientMeta(map[string]any{
		"risu_persona_observation": map[string]any{
			"contract_version":  "risu_persona_observation.v1",
			"observation_state": "observed",
			"source":            "chat.bindedPersona",
			"persona_id":        "persona-host-id",
			"persona_name":      "Rowan",
		},
	})
	if extractionStringFromAny(context["current_pov"]) != "Rowan" ||
		extractionStringFromAny(context["source"]) != "risu_persona_observation" ||
		extractionStringFromAny(context["mode"]) != "active_user_persona_knowledge_holder" {
		t.Fatalf("observed Risu persona was not mapped to a typed knowledge holder: %#v", context)
	}
	if extractionStringFromAny(context["current_pov_entity_id"]) != "" {
		t.Fatalf("host persona ID bypassed stable story identity resolution: %#v", context)
	}

	unobserved := prepareTurnPerspectiveContextFromClientMeta(map[string]any{
		"risu_persona_observation": map[string]any{
			"contract_version":  "risu_persona_observation.v1",
			"observation_state": "unobserved",
			"persona_name":      "Rowan",
		},
	})
	if len(unobserved) != 0 {
		t.Fatalf("unobserved Risu persona became a knowledge holder: %#v", unobserved)
	}
}

func TestCharacterPerspectivePacketSeparatesKnowledgeAndTargetedReveal(t *testing.T) {
	holderID := "holder-rowan"
	unit := func(id, holder, state, claim string) store.PreciseMemoryUnit {
		payload, _ := json.Marshal(map[string]any{
			"contract_version":           "perspective_memory.v1",
			"knowledge_holder_entity_id": holder,
			"epistemic_state":            state,
			"subject":                    id,
			"state_slot":                 "access",
			"claim":                      claim,
		})
		return store.PreciseMemoryUnit{
			UnitID: id, Kind: "observation", PayloadJSON: string(payload),
			EpistemicMode: state, AdmissionState: "committed",
			ReviewState: "source_observed", LifecycleState: "active",
			KnowledgeHolderEntityID: holder,
		}
	}
	units := []store.PreciseMemoryUnit{
		unit("known", holderID, "known", "open"),
		unit("suspected", holderID, "suspected", "trapped"),
		unit("misinformed", holderID, "misinformed", "empty"),
		unit("unknown", holderID, "unknown", "sealed"),
		unit("hidden", holderID, "hidden", "guarded"),
		unit("revealed", holderID, "revealed", "mapped"),
		unit("wrong-holder-secret", "holder-jules", "revealed", "contains the crown"),
	}
	packet, text := buildCharacterPerspectivePacket(
		units,
		map[string]any{"identity_state": "resolved", "current_pov_entity_id": holderID},
		10000,
	)
	for _, want := range []string{"known", "suspected", "misinformed", "revealed"} {
		if !strings.Contains(text, "- "+want+" |") {
			t.Fatalf("missing epistemic lane %q: %q", want, text)
		}
	}
	for _, forbidden := range []string{"sealed", "guarded", "contains the crown", "wrong-holder-secret"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("wrong/unknown knowledge leaked %q: %q", forbidden, text)
		}
	}
	serialized, _ := json.Marshal(packet)
	if strings.Contains(string(serialized), "wrong-holder-secret") ||
		strings.Contains(string(serialized), "contains the crown") {
		t.Fatalf("drop trace exposed IDs or raw private payload: %s", serialized)
	}
	dropped := mapFromAny(packet["dropped_counts"])
	if intFromAny(dropped["wrong_knowledge_holder"], 0) != 1 ||
		intFromAny(dropped["not_known_by_current_holder"], 0) != 2 {
		t.Fatalf("drop reasons=%#v", dropped)
	}
	finalPacket, finalText := finalizeCharacterPerspectivePacket(packet, text, text)
	if finalText == "" ||
		intFromAny(mapFromAny(finalPacket["selected_states"])["known"], 0) != 1 ||
		intFromAny(mapFromAny(finalPacket["selected_states"])["revealed"], 0) != 1 {
		t.Fatalf("final selected states do not reflect delivered items: %#v text=%q", finalPacket, finalText)
	}

	narratorPacket, narratorText := buildCharacterPerspectivePacket(
		units, map[string]any{"identity_state": "unobserved", "current_pov": "Narrator"}, 10000,
	)
	if narratorText != "" || intFromAny(mapFromAny(narratorPacket["dropped_counts"])["stable_pov_identity_required"], 0) != 1 {
		t.Fatalf("narrator isolation failed: packet=%#v text=%q", narratorPacket, narratorText)
	}
}

func TestCharacterPerspectivePacketProjectsLatestEpistemicStatePerHolderSubjectAndSlot(t *testing.T) {
	holderID := "holder-rowan"
	unit := func(id, state, claim string, turn int) store.PreciseMemoryUnit {
		payload, _ := json.Marshal(map[string]any{
			"contract_version":           "perspective_memory.v1",
			"knowledge_holder_entity_id": holderID,
			"epistemic_state":            state,
			"subject":                    "vault",
			"subject_entity_id":          "entity-vault",
			"state_slot":                 "access",
			"claim":                      claim,
		})
		return store.PreciseMemoryUnit{
			UnitID: id, Kind: "observation", PayloadJSON: string(payload),
			EpistemicMode: state, AdmissionState: "committed",
			ReviewState: "source_observed", LifecycleState: "active",
			KnowledgeHolderEntityID: holderID,
			SourceTurnStart:         turn,
			SourceTurnEnd:           turn,
		}
	}
	perspective := map[string]any{
		"identity_state":        "resolved",
		"current_pov_entity_id": holderID,
	}

	packet, text := buildCharacterPerspectivePacket(
		[]store.PreciseMemoryUnit{
			unit("old-known", "known", "open", 1),
			unit("latest-unknown", "unknown", "open", 2),
		},
		perspective,
		10000,
	)
	if text != "" {
		t.Fatalf("latest unknown state did not suppress old known state: %q", text)
	}
	dropped := mapFromAny(packet["dropped_counts"])
	if intFromAny(dropped["superseded_epistemic_state"], 0) != 1 ||
		intFromAny(dropped["not_known_by_current_holder"], 0) != 1 {
		t.Fatalf("latest projection drop reasons=%#v", dropped)
	}

	_, correctedText := buildCharacterPerspectivePacket(
		[]store.PreciseMemoryUnit{
			unit("old-misinformed", "misinformed", "sealed", 3),
			unit("latest-known", "known", "open", 4),
		},
		perspective,
		10000,
	)
	if !strings.Contains(correctedText, "known | vault / access: open") ||
		strings.Contains(correctedText, "misinformed") ||
		strings.Contains(correctedText, "sealed") {
		t.Fatalf("latest correction was not projected alone: %q", correctedText)
	}

	conflictPacket, conflictText := buildCharacterPerspectivePacket(
		[]store.PreciseMemoryUnit{
			unit("same-turn-known", "known", "open", 5),
			unit("same-turn-suspected", "suspected", "trapped", 5),
		},
		perspective,
		10000,
	)
	if conflictText != "" ||
		intFromAny(mapFromAny(conflictPacket["dropped_counts"])["conflicting_latest_epistemic_state"], 0) != 2 {
		t.Fatalf("same-turn conflict did not fail closed: packet=%#v text=%q", conflictPacket, conflictText)
	}

	latestReview := unit("latest-review", "suspected", "trapped", 6)
	latestReview.AdmissionState = "review_required"
	latestReview.ReviewState = "needs_review"
	reviewPacket, reviewText := buildCharacterPerspectivePacket(
		[]store.PreciseMemoryUnit{
			unit("older-known", "known", "open", 5),
			latestReview,
		},
		perspective,
		10000,
	)
	reviewDrops := mapFromAny(reviewPacket["dropped_counts"])
	if !strings.Contains(reviewText, "suspected | vault / access: trapped") ||
		strings.Contains(reviewText, "known | vault / access: open") ||
		intFromAny(reviewDrops["superseded_epistemic_state"], 0) != 1 {
		t.Fatalf("review metadata erased or revived owned knowledge incorrectly: packet=%#v text=%q", reviewPacket, reviewText)
	}
	wrongOwnerPacket, wrongOwnerText := buildCharacterPerspectivePacket(
		[]store.PreciseMemoryUnit{latestReview},
		map[string]any{"identity_state": "resolved", "current_pov_entity_id": "holder-jules"},
		10000,
	)
	if wrongOwnerText != "" || intFromAny(mapFromAny(wrongOwnerPacket["dropped_counts"])["wrong_knowledge_holder"], 0) != 1 {
		t.Fatalf("review item escaped its exact holder: packet=%#v text=%q", wrongOwnerPacket, wrongOwnerText)
	}
}

func TestPerspectiveScopedEvidenceCannotReenterThroughGeneralVectorHydration(t *testing.T) {
	memories := []store.Memory{{
		ID:        71,
		TurnIndex: 7,
		SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": "Mira privately told Rowan about the vault.",
			"belief_updates": []any{map[string]any{
				"perspective_owner": "Rowan",
				"subject":           "vault",
				"state_slot":        "access",
				"value":             "open",
				"evidence_excerpt":  "Mira privately told Rowan that the vault was open.",
			}},
		}),
	}}
	evidence := []store.DirectEvidence{
		{ID: 101, EvidenceText: "Mira privately told Rowan that the vault was open.", TurnAnchor: 7, SourceTurnStart: 7, SourceTurnEnd: 7},
		{ID: 102, EvidenceText: "The public bell rang at noon.", TurnAnchor: 6, SourceTurnStart: 6, SourceTurnEnd: 6},
		{ID: 103, EvidenceKind: "perspective_scoped_turn_excerpt", EvidenceText: "A tagged private excerpt.", TurnAnchor: 5, SourceTurnStart: 5, SourceTurnEnd: 5},
		{ID: 104, EvidenceText: "The public gate opened.", TurnAnchor: 7, SourceTurnStart: 7, SourceTurnEnd: 7},
	}
	safe, blockedIDs := filterPrepareTurnPerspectiveScopedEvidence(evidence, memories)
	if len(safe) != 2 || safe[0].ID != 102 || safe[1].ID != 104 ||
		!blockedIDs[101] || !blockedIDs[103] {
		t.Fatalf("perspective evidence filter mismatch: safe=%#v blocked=%#v", safe, blockedIDs)
	}

	vectorShadow := map[string]any{
		"memory_search_result": "ok",
		"search_result":        "ok",
		"search_results": []map[string]any{
			{"id": "evidence:session:101", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "101", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "evidence:session:102", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "102", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "evidence:session:103", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "103", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "evidence:session:104", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "104", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
	}
	hydrated := prepareTurnHydrateVectorArtifactHits(safe, nil, vectorShadow, 10, blockedIDs)
	if len(hydrated.Evidence) != 2 ||
		hydrated.Evidence[0].ID != 102 ||
		hydrated.Evidence[1].ID != 104 {
		t.Fatalf("protected evidence reentered vector hydration: %#v", hydrated.Evidence)
	}
	if intFromAny(hydrated.Trace["scope_filtered_count"], 0) != 2 ||
		intFromAny(hydrated.Trace["missing_count"], 0) != 0 {
		t.Fatalf("protected vector hit was not reported as scope-filtered: %#v", hydrated.Trace)
	}
}

func TestLegacyHolderScopedMemoryVectorCannotBypassTypedPerspectiveDelivery(t *testing.T) {
	memories := []store.Memory{
		{
			ID:        201,
			TurnIndex: 7,
			SummaryJSON: mustCompactJSON(map[string]any{
				"turn_summary": "Rowan privately believes the vault is open.",
				"belief_updates": []any{map[string]any{
					"perspective_owner": "Rowan",
					"subject":           "vault",
					"state_slot":        "access",
					"value":             "open",
				}},
			}),
		},
		{
			ID:          202,
			TurnIndex:   6,
			SummaryJSON: `{"turn_summary":"The public bell rang."}`,
		},
	}
	vectorShadow := map[string]any{
		"memory_search_result": "ok",
		"search_result":        "ok",
		"memory_search_results": []map[string]any{
			{"id": "memory:session:201", "tier": "memory", "source_table": "memories", "source_row_id": "201", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "memory:session:202", "tier": "memory", "source_table": "memories", "source_row_id": "202", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
		"search_results": []map[string]any{
			{"id": "memory:session:201", "tier": "memory", "source_table": "memories", "source_row_id": "201", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "memory:session:202", "tier": "memory", "source_table": "memories", "source_row_id": "202", "similarity": 0.9, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
	}
	hydrated := prepareTurnHydrateVectorMemoryHits(memories, vectorShadow, 10)
	if len(hydrated.Items) != 1 || hydrated.Items[0].ID != 202 {
		t.Fatalf("legacy holder-scoped vector bypassed typed delivery: %#v", hydrated.Items)
	}
	if intFromAny(hydrated.Trace["scope_filtered_count"], 0) != 1 {
		t.Fatalf("legacy holder-scoped vector was not scope-filtered: %#v", hydrated.Trace)
	}
}

func TestPrivateTypedBucketsDoNotBecomeStandalonePublicProjection(t *testing.T) {
	for _, key := range []string{
		"belief_updates",
		"protected_secrets",
		"character_identity_accuracy",
		"subjective_entity_memories",
	} {
		projection := buildPublicMemoryProjection(map[string]any{
			key: []any{map[string]any{"value": "private"}},
		}, "")
		if projection.Eligible {
			t.Fatalf("%s entered standalone public projection: %#v", key, projection)
		}
	}
	objective := buildPublicMemoryProjection(map[string]any{
		"turn_summary": "The bell is ringing.",
		"state_claims": []any{map[string]any{"value": "public"}},
	}, "")
	if !objective.Eligible {
		t.Fatalf("objective state was not publicly searchable: %#v", objective)
	}
}

func TestPublicMemoryProjectionKeepsObjectiveAndSourceObservedPublicInteractions(t *testing.T) {
	publicItem := func(evidence string, fields map[string]any) map[string]any {
		item := map[string]any{
			"admission_state":  "committed",
			"review_state":     "source_observed",
			"visibility":       "public",
			"evidence_excerpt": evidence,
			"public_visibility_support": map[string]any{
				"contract_version":     publicVisibilitySupportContract,
				"support_kind":         "explicit_public_observation",
				"visibility_assertion": evidence,
			},
		}
		for key, value := range fields {
			item[key] = value
		}
		return item
	}

	objective := buildPublicMemoryProjection(map[string]any{
		"turn_summary":      "The public bell rang at noon.",
		"evidence_excerpts": []any{"The public bell rang at noon."},
		"kg_triples": []any{map[string]any{
			"subject": "bell", "predicate": "rang_at", "object": "noon",
		}},
	}, "")
	if !objective.Eligible || !strings.Contains(objective.SearchText.Text, "public bell") {
		t.Fatalf("objective-only projection was not searchable: %#v", objective)
	}

	extraction := map[string]any{
		"turn_summary": "Mira publicly displayed several interaction traits.",
		"evidence_excerpts": []any{
			"Mira publicly said she trusted Rowan.",
			"Mira openly checked the gate twice.",
			"Mira openly refused the bribe.",
			`Mira publicly said, "Enough."`,
		},
		"relationship_observations": []any{publicItem("Mira publicly said she trusted Rowan.", map[string]any{
			"source_entity": "Mira", "target_entity": "Rowan", "domain": "trust", "observation": "trusts Rowan",
		})},
		"habit_observations": []any{publicItem("Mira openly checked the gate twice.", map[string]any{
			"subject_entity": "Mira", "behavior_key": "checks_gate", "observation_kind": "repeated_action",
		})},
		"character_profile_observations": []any{publicItem("Mira openly refused the bribe.", map[string]any{
			"subject_entity": "Mira", "trait_key": "values_honesty", "supported_expression": "refused the bribe",
		})},
		"voice_observations": []any{publicItem(`Mira publicly said, "Enough."`, map[string]any{
			"subject_entity": "Mira", "principle_key": "brief_imperatives", "utterance_expression": `"Enough."`,
		})},
	}
	projection := buildPublicMemoryProjection(extraction, "")
	if !projection.Eligible {
		t.Fatalf("public interaction projection was not eligible: %#v", projection)
	}
	for _, key := range []string{"relationship_observations", "habit_observations", "character_profile_observations", "voice_observations"} {
		if len(sliceFromAny(projection.Extraction[key])) != 1 {
			t.Fatalf("%s was not retained in public projection: %#v", key, projection.Extraction)
		}
	}
}

func TestPublicProjectionDoesNotRequirePositiveProofForInteractionOrVoiceMeaning(t *testing.T) {
	marker := strings.ToLower(t.Name())
	fixture := map[string]any{
		"interaction_events": []any{map[string]any{
			"actor": marker + ":actor", "action": marker + ":action",
		}},
		"relationship_observations": []any{map[string]any{
			"source_entity": marker + ":source", "target_entity": marker + ":target",
			"observation": marker + ":relationship",
		}},
		"habit_observations": []any{map[string]any{
			"subject_entity": marker + ":habit-subject", "behavior_key": marker + ":habit",
		}},
		"character_profile_observations": []any{map[string]any{
			"subject_entity": marker + ":profile-subject", "trait_key": marker + ":profile",
			"visibility": "public",
		}},
		"voice_observations": []any{map[string]any{
			"subject_entity": marker + ":voice-subject", "principle_key": marker + ":voice",
		}},
	}
	projection := buildPublicMemoryProjection(fixture, "")
	if !projection.Eligible {
		t.Fatalf("interaction/profile/voice meaning required absent positive-proof metadata: %#v", projection)
	}
	for _, key := range []string{
		"interaction_events", "relationship_observations", "habit_observations",
		"character_profile_observations", "voice_observations",
	} {
		if got := len(sliceFromAny(projection.Extraction[key])); got != 1 {
			t.Fatalf("%s missing without admission/review/evidence/support metadata: %#v", key, projection.Extraction)
		}
	}
	for _, want := range []string{marker + ":action", marker + ":relationship", marker + ":habit", marker + ":profile", marker + ":voice"} {
		if !strings.Contains(projection.SearchText.Text, want) {
			t.Fatalf("public projection search text missing %q: %q", want, projection.SearchText.Text)
		}
	}
	if !strings.Contains(projection.SearchText.Text, marker+":voice-subject | "+marker+":voice") {
		t.Fatalf("voice semantic summary lost its subject association: %q", projection.SearchText.Text)
	}
	fixture["relationship_observations"] = append(sliceFromAny(fixture["relationship_observations"]), map[string]any{
		"observation":     marker + ":review-relationship",
		"admission_state": "review_required", "review_state": "needs_review",
	})
	fixture["voice_observations"] = append(sliceFromAny(fixture["voice_observations"]), map[string]any{
		"principle_key":   marker + ":review-voice",
		"admission_state": "review_required", "review_state": "needs_review",
	})
	reviewed := buildPublicMemoryProjection(fixture, "")
	if len(sliceFromAny(reviewed.Extraction["relationship_observations"])) != 2 ||
		len(sliceFromAny(reviewed.Extraction["voice_observations"])) != 2 ||
		!strings.Contains(reviewed.SearchText.Text, marker+":review-relationship") ||
		!strings.Contains(reviewed.SearchText.Text, marker+":review-voice") {
		t.Fatalf("review metadata was incorrectly treated as privacy: %#v", reviewed)
	}

	fixture["voice_observations"] = append(sliceFromAny(fixture["voice_observations"]), map[string]any{
		"principle_key": marker + ":private-voice", "visibility": "owner_private",
	})
	blocked := buildPublicMemoryProjection(fixture, "")
	if strings.Contains(blocked.SearchText.Text, marker+":private-voice") || len(sliceFromAny(blocked.Extraction["voice_observations"])) != 2 {
		t.Fatalf("explicit private voice observation entered public projection: %#v", blocked)
	}
}

func TestPublicInteractionProjectionDoesNotTreatScopeKeyNamesAsPrivate(t *testing.T) {
	publicItem := func(evidence string) map[string]any {
		return map[string]any{
			"admission_state":  "committed",
			"review_state":     "source_observed",
			"visibility":       "public",
			"evidence_excerpt": evidence,
			"public_visibility_support": map[string]any{
				"contract_version":     publicVisibilitySupportContract,
				"support_kind":         "explicit_public_observation",
				"visibility_assertion": evidence,
			},
		}
	}

	allowed := publicItem("Mira openly waved at Rowan.")
	allowed["knowledge_holder"] = ""
	allowed["perspective_owner"] = ""
	allowed["secret_guard"] = false
	allowed["knowledge_scope"] = map[string]any{
		"known_by": []any{}, "unknown_to": []any{""}, "publicly_revealed": false,
	}
	projection := buildPublicMemoryProjection(map[string]any{
		"evidence_excerpts":  []any{"Mira openly waved at Rowan."},
		"interaction_events": []any{allowed},
	}, "")
	if !projection.Eligible || len(sliceFromAny(projection.Extraction["interaction_events"])) != 1 {
		t.Fatalf("empty scope metadata blocked a proven public interaction: %#v", projection)
	}

	for _, tc := range []struct {
		name  string
		key   string
		value any
	}{
		{name: "holder", key: "knowledge_holder", value: "Rowan"},
		{name: "scope", key: "knowledge_scope", value: map[string]any{"known_by": []any{"Rowan"}}},
		{name: "reveal policy", key: "reveal_policy", value: "explicit_reveal_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := publicItem("Mira openly waved at Rowan.")
			item[tc.key] = tc.value
			kept := buildPublicMemoryProjection(map[string]any{
				"evidence_excerpts":  []any{"Mira openly waved at Rowan."},
				"interaction_events": []any{item},
			}, "")
			if !kept.Eligible || len(sliceFromAny(kept.Extraction["interaction_events"])) != 1 {
				t.Fatalf("scope-shaped key name removed its enclosing public item: %#v", kept)
			}
		})
	}
	privateItem := publicItem("Mira openly waved at Rowan.")
	privateItem["secret_guard"] = true
	blocked := buildPublicMemoryProjection(map[string]any{
		"evidence_excerpts":  []any{"Mira openly waved at Rowan."},
		"interaction_events": []any{privateItem},
	}, "")
	if blocked.Eligible || len(sliceFromAny(blocked.Extraction["interaction_events"])) != 0 {
		t.Fatalf("explicit private guard entered public projection: %#v", blocked)
	}
}

func TestObjectiveProjectionKeepsUnscopedReviewItemsAlongsideLegacyAndCommittedItems(t *testing.T) {
	legacyEvidence := "The legacy bell rang."
	committedEvidence := "The committed gate opened."
	reviewEvidence := "The unverified cipher named Night Glass."
	projection := buildPublicMemoryProjection(map[string]any{
		"turn_summary":      "Two verified state changes and the unverified Night Glass cipher were recorded.",
		"evidence_excerpts": []any{legacyEvidence, committedEvidence, reviewEvidence},
		"state_claims": []any{
			map[string]any{
				"subject": "legacy_bell", "state_slot": "ringing", "value": true,
				"evidence_excerpt": legacyEvidence,
			},
			map[string]any{
				"subject": "gate", "state_slot": "open", "value": true,
				"evidence_excerpt": committedEvidence,
				"admission_state":  "committed", "review_state": "source_observed", "visibility": "public",
			},
			map[string]any{
				"subject": "cipher", "state_slot": "name", "value": "Night Glass",
				"evidence_excerpt": reviewEvidence,
				"admission_state":  "review_required", "review_state": "needs_review", "visibility": "public",
			},
		},
	}, "")
	if !projection.Eligible {
		t.Fatalf("objective projection unexpectedly became ineligible: %#v", projection)
	}
	if got := len(sliceFromAny(projection.Extraction["state_claims"])); got != 3 {
		t.Fatalf("objective projection kept %d items, want legacy+committed+review: %#v", got, projection.Extraction)
	}
	if !strings.Contains(projection.SearchText.Text, legacyEvidence) ||
		!strings.Contains(projection.SearchText.Text, committedEvidence) {
		t.Fatalf("admissible objective meaning was lost: %q", projection.SearchText.Text)
	}
	if !strings.Contains(projection.SearchText.Text, reviewEvidence) || !strings.Contains(projection.SearchText.Text, "Night Glass") {
		t.Fatalf("unscoped review item was treated as private: %q", projection.SearchText.Text)
	}
}

func TestMissingPrivateEvidenceDoesNotPoisonUnrelatedExactEvidence(t *testing.T) {
	extraction := map[string]any{
		"turn_summary": "The bell rang while Rowan learned something privately.",
		"evidence_excerpts": []any{
			"The public bell rang.",
			"Mira whispered that the vault was open.",
		},
		"belief_updates": []any{
			map[string]any{
				"perspective_owner": "Rowan", "subject": "vault", "state_slot": "access", "value": "open",
				"evidence_excerpt": "Mira whispered that the vault was open.",
			},
			map[string]any{
				"perspective_owner": "Rowan", "subject": "password", "state_slot": "value", "value": "missing-link",
			},
		},
		"state_claims": []any{map[string]any{
			"subject": "bell", "state_slot": "ringing", "value": true,
			"evidence_excerpt": "The public bell rang.",
		}},
	}
	projection := buildPublicMemoryProjection(extraction, "")
	if !projection.Eligible || !strings.Contains(projection.SearchText.Text, "bell ringing") {
		t.Fatalf("missing private evidence poisoned unrelated public evidence: %#v", projection)
	}
	if strings.Contains(projection.SearchText.Text, "vault") || strings.Contains(projection.SearchText.Text, "password") || strings.Contains(projection.SearchText.Text, "Rowan") {
		t.Fatalf("private holder material entered public projection: %q", projection.SearchText.Text)
	}
	if len(sliceFromAny(projection.Extraction["state_claims"])) != 1 {
		t.Fatalf("exact public objective item was lost from mixed projection: %#v", projection.Extraction)
	}

	memories := []store.Memory{{ID: 1, TurnIndex: 4, SummaryJSON: mustCompactJSON(extraction)}}
	evidence := []store.DirectEvidence{
		{ID: 1, EvidenceText: "The public bell rang.", SourceTurnStart: 4, SourceTurnEnd: 4},
		{ID: 2, EvidenceText: "Mira whispered that the vault was open.", SourceTurnStart: 4, SourceTurnEnd: 4},
	}
	safe, blocked := filterPrepareTurnPerspectiveScopedEvidence(evidence, memories)
	if len(safe) != 1 || safe[0].ID != 1 || !blocked[2] || blocked[1] {
		t.Fatalf("exact evidence scope mismatch: safe=%#v blocked=%#v", safe, blocked)
	}
}

func TestMixedProjectionDoesNotGloballySuppressEvidenceWhenPrivateLinkIsIncomplete(t *testing.T) {
	publicExcerpt := "The public bell rang."
	unlinkedPrivateExcerpt := "Rowan privately used the alias Night Orchid."
	projection := buildPublicMemoryProjection(map[string]any{
		"turn_summary":      "The bell rang while Rowan privately used an alias.",
		"evidence_excerpts": []any{publicExcerpt, unlinkedPrivateExcerpt},
		"state_claims": []any{map[string]any{
			"subject": "bell", "state_slot": "ringing", "value": true,
			"evidence_excerpt": publicExcerpt,
		}},
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan", "subject": "identity",
			"state_slot": "alias", "value": "Night Orchid",
		}},
	}, "")
	if !projection.Eligible || !strings.Contains(projection.SearchText.Text, "bell ringing") ||
		!strings.Contains(projection.SearchText.Text, publicExcerpt) ||
		!strings.Contains(projection.SearchText.Text, unlinkedPrivateExcerpt) {
		t.Fatalf("positive-linked public evidence was lost: %#v", projection)
	}
	serialized := mustCompactJSON(projection.Extraction)
	if strings.Contains(serialized, "belief_updates") || strings.Contains(serialized, "perspective_owner") {
		t.Fatalf("typed private bucket entered public projection: %s", serialized)
	}
}

func TestMixedProjectionKeepsGroundedEventDescriptionsWithoutRawPrivateSummary(t *testing.T) {
	marker := strings.ToLower(t.Name())
	publicEventEvidence := marker + ":public-event-evidence"
	publicDescriptionEvidence := marker + ":public-description-evidence"
	privateEvidence := marker + ":private-evidence"
	reviewEvidence := marker + ":review-evidence"
	publicEventText := marker + ":public-event"
	publicDescriptionText := marker + ":public-description"
	crossMatchedObjectiveValue := marker + ":cross-matched-objective-value"
	reviewValue := marker + ":review-value"
	extraction := map[string]any{
		"turn_summary": strings.Join([]string{publicEventText, publicDescriptionText, crossMatchedObjectiveValue, reviewValue}, " "),
		"evidence_excerpts": []any{
			publicEventEvidence,
			publicDescriptionEvidence,
			privateEvidence,
			reviewEvidence,
		},
		"narrative_events": []any{
			map[string]any{
				"event":            publicEventText,
				"evidence_excerpt": publicEventEvidence,
			},
			map[string]any{
				"description":      publicDescriptionText,
				"evidence_excerpt": publicDescriptionEvidence,
				"visibility":       "public",
			},
		},
		"state_claims": []any{
			map[string]any{
				"subject": marker + ":review-subject", "state_slot": "name", "value": reviewValue,
				"evidence_excerpt": reviewEvidence,
				"admission_state":  "review_required",
				"review_state":     "needs_review",
				"visibility":       "public",
			},
			map[string]any{
				"subject": marker + ":cross-matched-objective-subject", "state_slot": "origin", "value": crossMatchedObjectiveValue,
				"evidence_excerpt": privateEvidence,
			},
		},
		"kg_triples": []any{map[string]any{
			"subject": marker + ":kg-subject", "predicate": marker + ":kg-predicate", "object": marker + ":kg-object",
		}},
		"entities": map[string]any{"characters": []any{map[string]any{
			"name":            marker + ":entity",
			"identity_proof":  map[string]any{"kind": marker + ":proof"},
			"knowledge_scope": map[string]any{"known_by": []any{marker + ":observer"}},
		}}},
		"protected_secrets": []any{
			map[string]any{
				"owner": marker + ":owner", "secret_kind": marker + ":secret-kind",
				"secret_summary":   marker + ":private-secret-value",
				"evidence_excerpt": privateEvidence,
				"knowledge_scope":  map[string]any{"known_by": []any{marker + ":owner"}},
			},
		},
	}

	projection := buildPublicMemoryProjection(extraction, "")
	if !projection.Eligible {
		t.Fatalf("grounded public events were rejected with the mixed row: %#v", projection)
	}
	projectedSummary := memorySummaryFromParsed(projection.Extraction)
	for _, want := range []string{publicEventText, publicDescriptionText} {
		if !strings.Contains(projectedSummary, want) {
			t.Fatalf("projected event/description missing %q: %q", want, projectedSummary)
		}
	}
	serialized := mustCompactJSON(projection.Extraction)
	for _, forbidden := range []string{marker + ":private-secret-value"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("private or review canary entered public projection: %s", serialized)
		}
	}
	if got := len(sliceFromAny(projection.Extraction["narrative_events"])); got != len(sliceFromAny(extraction["narrative_events"])) {
		t.Fatalf("grounded public events=%d, want fixture events=%d", got, len(sliceFromAny(extraction["narrative_events"])))
	}
	if got := len(sliceFromAny(projection.Extraction["state_claims"])); got != 2 ||
		!strings.Contains(serialized, crossMatchedObjectiveValue) || !strings.Contains(serialized, reviewValue) {
		t.Fatalf("same-evidence or review objective meaning was removed: %#v", projection.Extraction["state_claims"])
	}
	for _, retained := range []string{marker + ":kg-subject", marker + ":entity", marker + ":proof", marker + ":observer"} {
		if !strings.Contains(serialized, retained) {
			t.Fatalf("private sibling caused general KG/entity metadata %q to disappear: %s", retained, serialized)
		}
	}
	if !strings.Contains(serialized, privateEvidence) {
		t.Fatalf("same-turn objective evidence was globally removed by a private typed item: %s", serialized)
	}

	projectedMemory, ok := publicMemoryFromCanonical(store.Memory{
		SummaryJSON: mustCompactJSON(extraction),
	})
	if !ok {
		t.Fatal("canonical mixed memory did not produce a public memory projection")
	}
	rendered := prepareTurnMemorySummary(projectedMemory)
	if strings.HasPrefix(rendered, "{") {
		t.Fatalf("prepare-turn rendered raw JSON instead of readable public events: %q", rendered)
	}
	for _, want := range []string{publicEventText, publicDescriptionText} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("prepare-turn readable summary missing %q: %q", want, rendered)
		}
	}
	for _, forbidden := range []string{marker + ":private-secret-value"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("prepare-turn readable summary leaked private or review canary: %q", rendered)
		}
	}
}

func TestMemoryAdmissionIncompletePrivateLinkDoesNotSuppressGeneralEvidenceVectors(t *testing.T) {
	fake := &memoryAdmissionWorkerStore{}
	srv := &Server{Store: fake}
	srv.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	marker := strings.ToLower(t.Name())
	publicExcerpt := marker + ":public-evidence"
	unlinkedExcerpt := marker + ":unlinked-evidence"
	extraction := map[string]any{
		"turn_summary":      marker + ":blended-summary",
		"importance_score":  5,
		"evidence_excerpts": []any{publicExcerpt, unlinkedExcerpt},
		"state_claims": []any{map[string]any{
			"subject": marker + ":subject", "state_slot": marker + ":slot", "value": marker + ":value",
			"evidence_excerpt": publicExcerpt,
		}},
		"belief_updates": []any{map[string]any{
			"perspective_owner": marker + ":owner", "subject": marker + ":private-subject",
			"state_slot": marker + ":private-slot", "value": marker + ":private-value",
		}},
	}
	ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{
		ContractVersion: completeTurnSourceAcceptanceContract,
		Revision:        "revision-missing-private-evidence",
		LogicalTurnID:   "logical-turn-missing-private-evidence",
	})
	result := artifactSaveResult{}
	handled, _, _ := srv.commitAcceptedMemoryAdmission(
		ctx,
		"session-missing-private-evidence",
		12,
		extraction,
		publicExcerpt+" "+unlinkedExcerpt,
		extractionStringFromAny(extraction["turn_summary"]),
		"ignored canonical search text",
		memorySearchTextBuild{},
		completeTurnEmbeddingConfig{},
		"[]",
		"test-embedding",
		[]float32{0.1, 0.2},
		nil,
		nil,
		nil,
		time.Unix(1200, 0),
		&result,
	)
	if !handled || result.Errors != 0 || len(fake.admissions) != 1 {
		t.Fatalf("memory admission failed: handled=%t result=%+v admissions=%d", handled, result, len(fake.admissions))
	}
	admission := fake.admissions[0]
	if len(admission.Evidence) != 2 {
		t.Fatalf("canonical mixed evidence was lost: %#v", admission.Evidence)
	}
	wantVectors := 1 + len(memorySearchStringValues(extraction["evidence_excerpts"]))
	if len(admission.Vectors) != wantVectors || admission.Vectors[0].ArtifactType != "memory" {
		t.Fatalf("incomplete private link suppressed general evidence vectors: %#v", admission.Vectors)
	}
	serialized := mustCompactJSON(admission.Vectors)
	for _, retained := range []string{publicExcerpt, unlinkedExcerpt, marker + ":subject", marker + ":value"} {
		if !strings.Contains(serialized, retained) {
			t.Fatalf("general vector payload lost %q after an incomplete private link: %s", retained, serialized)
		}
	}
	if strings.Contains(serialized, marker+":private-subject") || strings.Contains(serialized, marker+":private-value") {
		t.Fatalf("typed private bucket entered general vector payload: %s", serialized)
	}
	if admission.Memory == nil ||
		!strings.Contains(admission.Memory.SummaryJSON, marker+":private-value") ||
		!strings.Contains(admission.Memory.Evidence, unlinkedExcerpt) {
		t.Fatalf("canonical private extraction/evidence was not preserved: %+v", admission.Memory)
	}
}

func TestPrepareTurnGeneralProjectionKeepsMixedPublicEvidenceAndCanonicalProtectedSource(t *testing.T) {
	publicExcerpt := "The public bell rang at noon."
	privateExcerpt := "Mira privately named herself the Silver Fox."
	mixed := store.Memory{
		ID:        31,
		TurnIndex: 8,
		SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary":      "The bell rang while Mira privately revealed her identity.",
			"evidence_excerpts": []any{publicExcerpt, privateExcerpt},
			"state_claims": []any{map[string]any{
				"subject": "bell", "state_slot": "ringing", "value": true,
				"evidence_excerpt": publicExcerpt,
			}},
			"protected_secrets": []any{map[string]any{
				"owner": "Mira", "secret_kind": "identity",
				"secret_summary":    "Mira is the Silver Fox.",
				"evidence_excerpt":  privateExcerpt,
				"disclosure_policy": "owner_private_until_revealed",
				"knowledge_scope":   map[string]any{"known_by": []any{"Mira"}},
			}},
		}),
	}
	privateOnly := store.Memory{
		ID:        32,
		TurnIndex: 9,
		SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary":      "Mira privately named the second vault key.",
			"evidence_excerpts": []any{"Mira privately named the second vault key."},
			"protected_secrets": []any{map[string]any{
				"owner": "Mira", "secret_kind": "vault_key",
				"secret_summary":    "The second vault key is Lark.",
				"evidence_excerpt":  "Mira privately named the second vault key.",
				"disclosure_policy": "owner_private_until_revealed",
				"knowledge_scope":   map[string]any{"known_by": []any{"Mira"}},
			}},
		}),
	}

	canonicalSummary := mixed.SummaryJSON
	if guard := prepareTurnProtectedMemoryGuard(mixed); !guard.Active {
		t.Fatal("canonical protected source did not retain its typed/protected guard")
	}
	general, trace := projectPrepareTurnGeneralMemories([]store.Memory{mixed, privateOnly})
	if len(general) != 1 || general[0].ID != mixed.ID {
		t.Fatalf("general projection did not keep only the mixed public item: %#v", general)
	}
	if intFromAny(trace["public_memory_projection_input_count"], 0) != 2 ||
		intFromAny(trace["public_memory_projection_output_count"], 0) != 1 {
		t.Fatalf("general projection trace mismatch: %#v", trace)
	}
	projectedJSON := general[0].SummaryJSON
	if !strings.Contains(projectedJSON, "bell") ||
		strings.Contains(projectedJSON, privateExcerpt) ||
		strings.Contains(projectedJSON, "Silver Fox") ||
		strings.Contains(projectedJSON, "Mira") ||
		strings.Contains(projectedJSON, "known_by") {
		t.Fatalf("mixed general projection leaked or lost scoped material: %s", projectedJSON)
	}
	if mixed.SummaryJSON != canonicalSummary || !prepareTurnProtectedMemoryGuard(mixed).Active {
		t.Fatalf("general projection mutated the canonical typed/protected source: %#v", mixed)
	}

	documents := buildUnifiedRetrievalDocuments("session-public-projection", general, nil, nil, nil, nil, nil)
	if len(documents) != 1 {
		t.Fatalf("safe retrieval document count=%d, want 1", len(documents))
	}
	documentJSON := mustCompactJSON(documents[0])
	if !strings.Contains(documentJSON, "bell") ||
		strings.Contains(documentJSON, privateExcerpt) ||
		strings.Contains(documentJSON, "Silver Fox") ||
		strings.Contains(documentJSON, "Mira") {
		t.Fatalf("safe retrieval document used canonical private text: %s", documentJSON)
	}

	privateChatText := "Mira privately named herself the Silver Fox."
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{mixed, privateOnly}, nil, nil,
		[]store.ChatLog{{ID: 91, TurnIndex: 9, Role: "assistant", Content: privateChatText}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "bell ringing", "default", documents, nil, nil,
		map[string]any{
			"_character_perspective_text":            "[Character Perspective]\n- Mira hesitates without revealing the reason.",
			"_character_perspective_candidate_count": 1,
		},
	)
	if !strings.Contains(assembly.ActualMemoryText, "bell ringing") {
		t.Fatalf("actual memory lane lost the mixed row public projection: %q", assembly.ActualMemoryText)
	}
	for _, privateText := range []string{privateExcerpt, "Silver Fox", "second vault key", privateChatText} {
		if strings.Contains(assembly.ActualMemoryText, privateText) || strings.Contains(assembly.FallbackText, privateText) {
			t.Fatalf("private text reentered general or fallback delivery: actual=%q fallback=%q", assembly.ActualMemoryText, assembly.FallbackText)
		}
	}
	if !strings.Contains(assembly.ProtectedMemoryText, "Mira hesitates without revealing the reason") {
		t.Fatalf("separate typed perspective lane was lost: %q", assembly.ProtectedMemoryText)
	}
	if intFromAny(assembly.Counts["public_memory_projection_input_count"], 0) != 2 ||
		intFromAny(assembly.Counts["public_memory_projection_output_count"], 0) != 1 ||
		boolFromAny(assembly.Counts["raw_chat_fallback_enabled"]) {
		t.Fatalf("assembly projection/fallback trace mismatch: %#v", assembly.Counts)
	}

	recall := buildRecallResult(
		"session-public-projection", "bell ringing", false,
		[]store.Memory{mixed, privateOnly}, nil, nil, nil,
		[]store.ChatLog{{ID: 91, TurnIndex: 9, Role: "assistant", Content: privateChatText}},
		nil, nil, nil, nil, nil, "default", 5, "bell ringing",
	)
	recallDeliveryJSON := mustCompactJSON(map[string]any{
		"items":        recall["items"],
		"search":       recall["search"],
		"documents":    recall["documents"],
		"recall_lanes": recall["recall_lanes"],
	})
	if !strings.Contains(recallDeliveryJSON, "bell ringing") || strings.Contains(recallDeliveryJSON, privateExcerpt) || strings.Contains(recallDeliveryJSON, "Silver Fox") || strings.Contains(recallDeliveryJSON, "second vault key") {
		t.Fatalf("recall delivery surfaces lost public projection or exposed private memory: %s", recallDeliveryJSON)
	}
}

func TestMemoryAdmissionOmitsGeneralVectorsForPerspectiveScopedTurn(t *testing.T) {
	run := func(extraction map[string]any) *store.MemoryAdmission {
		fake := &memoryAdmissionWorkerStore{}
		srv := &Server{Store: fake}
		srv.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
		ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{
			ContractVersion: completeTurnSourceAcceptanceContract,
			Revision:        "revision-perspective-vector",
			LogicalTurnID:   "logical-turn-perspective-vector",
		})
		result := artifactSaveResult{}
		handled, _, _ := srv.commitAcceptedMemoryAdmission(
			ctx,
			"session-perspective-vector",
			7,
			extraction,
			"The public bell rang. Mira privately told Rowan the vault was open.",
			extractionStringFromAny(extraction["turn_summary"]),
			"search text",
			memorySearchTextBuild{},
			completeTurnEmbeddingConfig{},
			"[]",
			"test-embedding",
			[]float32{0.1, 0.2},
			nil,
			nil,
			nil,
			time.Unix(700, 0),
			&result,
		)
		if !handled || result.Errors != 0 || len(fake.admissions) != 1 {
			t.Fatalf("memory admission failed: handled=%t result=%+v admissions=%d", handled, result, len(fake.admissions))
		}
		return fake.admissions[0]
	}

	public := run(map[string]any{
		"turn_summary":     "The public bell rang.",
		"importance_score": 5,
		"evidence_excerpts": []any{
			"The public bell rang.",
		},
	})
	if len(public.Vectors) != 2 {
		t.Fatalf("public memory/evidence vectors=%d, want 2", len(public.Vectors))
	}

	private := run(map[string]any{
		"turn_summary":     "Mira privately told Rowan the vault was open.",
		"importance_score": 5,
		"evidence_excerpts": []any{
			"Mira privately told Rowan the vault was open.",
		},
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "open",
			"evidence_excerpt":  "Mira privately told Rowan the vault was open.",
		}},
	})
	if len(private.Vectors) != 0 {
		t.Fatalf("perspective-scoped turn entered general vectors: %#v", private.Vectors)
	}
	if len(private.Evidence) != 1 ||
		private.Evidence[0].EvidenceKind != "perspective_scoped_turn_excerpt" {
		t.Fatalf("perspective evidence was not tagged for fail-closed recall: %#v", private.Evidence)
	}
	if private.Memory == nil || private.Memory.EmbeddingModel != "" {
		t.Fatalf("private canonical memory stored a policy sentinel as embedding model: %+v", private.Memory)
	}

	mixed := run(map[string]any{
		"turn_summary":     "The public bell rang before Mira privately told Rowan about the vault.",
		"importance_score": 5,
		"evidence_excerpts": []any{
			"The public bell rang.",
			"Mira privately told Rowan the vault was open.",
		},
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "open",
			"evidence_excerpt":  "Mira privately told Rowan the vault was open.",
		}},
		"state_claims": []any{map[string]any{
			"subject":          "bell",
			"state_slot":       "ringing",
			"value":            true,
			"evidence_excerpt": "The public bell rang.",
		}},
	})
	if len(mixed.Vectors) != 2 ||
		mixed.Vectors[0].ArtifactType != "memory" ||
		!strings.Contains(mixed.Vectors[0].DocumentText, "bell ringing") ||
		strings.Contains(mixed.Vectors[0].DocumentText, "vault") ||
		strings.Contains(mixed.Vectors[0].DocumentText, "Mira") ||
		mixed.Vectors[1].ArtifactType != "evidence" ||
		mixed.Vectors[1].EvidenceText != "The public bell rang." {
		t.Fatalf("mixed turn did not retain only its public general projection: %#v", mixed.Vectors)
	}
	if len(mixed.Evidence) != 2 {
		t.Fatalf("mixed canonical evidence was lost: %#v", mixed.Evidence)
	}
}

func TestCharacterPerspectiveCandidateUsesProtectedMemoryDeliveryPlan(t *testing.T) {
	candidate := "[Character Perspective]\n- known | vault / access: open"
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		4, 4000, "Continue.", "default", nil, nil, nil,
		map[string]any{
			"current_pov":                            "Rowan",
			"current_pov_entity_id":                  "holder-rowan",
			"identity_state":                         "resolved",
			"_character_perspective_text":            candidate,
			"_character_perspective_candidate_count": 1,
		},
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(assembly.ProtectedMemoryText, "known | vault / access: open") ||
		!strings.Contains(finalText, "known | vault / access: open") {
		t.Fatalf("perspective candidate bypassed or missed protected memory delivery: protected=%q final=%q", assembly.ProtectedMemoryText, finalText)
	}
	if intFromAny(assembly.Counts["character_perspective_candidate_count"], 0) != 1 {
		t.Fatalf("candidate count missing: %#v", assembly.Counts)
	}
}

func TestPerspectiveMemoryAdmissionCreatesGroundedUnitPerListenerAndReplays(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	excerpt := "Mira told Rowan and Jules that the vault was open."
	content := "The room quieted. " + excerpt + " Everyone listened."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "Mira"},
			map[string]any{"name": "Rowan"},
			map[string]any{"name": "Jules"},
		}},
		"belief_updates": []any{map[string]any{
			"subject": "vault", "subject_type": "world", "state_slot": "access",
			"value": "open", "speaker_name": "Mira",
			"listener_names":  []any{"Rowan", "Jules"},
			"epistemic_state": "known", "acquisition_mode": "heard",
			"evidence_excerpt": excerpt,
		}},
	}
	save := func() artifactSaveResult {
		return srv.saveCriticExtractionArtifacts(
			acceptedPreciseMemoryContext("revision-listeners"), "session-listeners", 7,
			extraction, content, completeTurnEmbeddingConfig{}, time.Unix(700, 0),
		)
	}
	first := save()
	if first.PreciseMemoryUnits != 2 || len(fake.unitsByKey) != 2 {
		t.Fatalf("listener units=%d stored=%d skips=%#v errors=%#v", first.PreciseMemoryUnits, len(fake.unitsByKey), first.SkipReasons, first.ErrorDetails)
	}
	holderIDs := map[string]bool{}
	for _, unit := range fake.unitsByKey {
		if unit.ActorEntityID == "" || unit.KnowledgeHolderEntityID == "" ||
			unit.AdmissionState != "committed" || unit.ReviewState != "source_observed" {
			t.Fatalf("grounded speaker/listener link missing: %+v", unit)
		}
		if !strings.Contains(unit.PayloadJSON, `"contract_version":"perspective_memory.v1"`) ||
			!strings.Contains(unit.PayloadJSON, `"acquisition_mode":"heard"`) {
			t.Fatalf("normalized perspective payload missing: %s", unit.PayloadJSON)
		}
		if strings.Contains(unit.PayloadJSON, `"listener_names"`) ||
			strings.Contains(unit.PayloadJSON, `"listeners"`) ||
			strings.Contains(unit.PayloadJSON, `"knowledge_holders"`) {
			t.Fatalf("per-holder payload retained the multi-holder source list: %s", unit.PayloadJSON)
		}
		holderIDs[unit.KnowledgeHolderEntityID] = true
	}
	if len(holderIDs) != 2 {
		t.Fatalf("listeners collapsed into one holder: %#v", holderIDs)
	}
	replay := save()
	if replay.PreciseMemoryUnits != 0 || len(fake.unitsByKey) != 2 {
		t.Fatalf("replay was not idempotent: result=%+v stored=%d", replay, len(fake.unitsByKey))
	}
}

func TestPerspectiveMemoryAmbiguousListenerPreservesRawButCannotDeliver(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	excerpt := "Alex told Alex that the vault was open."
	content := "At the doorway, " + excerpt + " Then the lights failed."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "Alex"},
			map[string]any{"name": "Alex"},
		}},
		"belief_updates": []any{map[string]any{
			"subject": "vault", "subject_type": "world", "state_slot": "access",
			"value": "open", "speaker_name": "Alex", "listener_names": []any{"Alex"},
			"epistemic_state": "known", "evidence_excerpt": excerpt,
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-ambiguous"), "session-ambiguous", 8,
		extraction, content, completeTurnEmbeddingConfig{}, time.Unix(800, 0),
	)
	if result.PreciseMemoryUnits != 1 || len(fake.unitsByKey) != 1 {
		t.Fatalf("ambiguous raw unit missing: result=%+v stored=%d", result, len(fake.unitsByKey))
	}
	unit := preciseMemoryUnitByKind(t, fake, "observation")
	if unit.AdmissionState != "review_required" || unit.ReviewState != "needs_review" ||
		unit.KnowledgeHolderEntityID != "" || unit.ActorEntityID != "" {
		t.Fatalf("ambiguous identities became active links: %+v", unit)
	}
	if !strings.Contains(unit.PayloadJSON, "Alex") {
		t.Fatalf("ambiguous raw proposal was not preserved: %s", unit.PayloadJSON)
	}
}

func TestProtectedSecretConflictingHolderStatesRequireReviewAndDoNotCopyScope(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	excerpt := "Mira hid the crown map from Rowan."
	content := "At midnight, " + excerpt + " The door closed."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "Mira"},
			map[string]any{"name": "Rowan"},
		}},
		"evidence_excerpts": []any{excerpt},
		"protected_secrets": []any{map[string]any{
			"secret_kind": "crown_map", "owner": "Mira",
			"subject": []any{"Mira"}, "summary": "Mira owns the crown map.",
			"evidence_excerpt": excerpt,
			"knowledge_scope": map[string]any{
				"known_by":   []any{"Rowan"},
				"unknown_to": []any{"Rowan"},
			},
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-conflicting-secret"),
		"session-conflicting-secret", 10, extraction, content,
		completeTurnEmbeddingConfig{}, time.Unix(1000, 0),
	)
	if result.PreciseMemoryUnits != 2 || len(fake.unitsByKey) != 2 {
		t.Fatalf("conflicting proposals were not preserved for review: result=%+v stored=%d", result, len(fake.unitsByKey))
	}
	for _, unit := range fake.unitsByKey {
		if unit.AdmissionState != "review_required" ||
			unit.ReviewState != "needs_review" ||
			unit.KnowledgeHolderEntityID == "" {
			t.Fatalf("conflicting holder state became deliverable: %+v", unit)
		}
		if strings.Contains(unit.PayloadJSON, `"knowledge_scope"`) {
			t.Fatalf("per-holder secret payload copied the full knowledge scope: %s", unit.PayloadJSON)
		}
	}
}

func TestProtectedSecretMisinformedScopeCannotExposeTrueSecretAsFalseBelief(t *testing.T) {
	candidates := protectedSecretPerspectiveMemoryCandidates(map[string]any{
		"protected_secrets": []any{map[string]any{
			"secret_kind": "crown_map", "owner": "Mira",
			"subject": []any{"Mira"}, "summary": "Mira owns the crown map.",
			"evidence_excerpt": "Rowan repeated a false story about Mira's map.",
			"knowledge_scope": map[string]any{
				"misinformed_by": []any{"Rowan"},
			},
		}},
	})
	if len(candidates) != 1 {
		t.Fatalf("misinformed proposal count=%d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.epistemicMode != "misinformed" ||
		candidate.admissionState != "review_required" ||
		candidate.reviewState != "needs_review" {
		t.Fatalf("true secret became deliverable misinformation: %+v", candidate)
	}
}

func TestPerspectiveMemoryTargetedRevealRequiresExplicitRevealTransition(t *testing.T) {
	for _, tc := range []struct {
		name       string
		transition string
		admission  string
		review     string
	}{
		{name: "direct reveal", transition: "reveal", admission: "committed", review: "source_observed"},
		{name: "missing reveal transition", admission: "review_required", review: "needs_review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newPreciseMemoryRecordingStore()
			srv := &Server{Store: fake}
			excerpt := "Mira revealed her vault map to Rowan."
			content := "At dawn, " + excerpt + " Then Rowan nodded."
			extraction := map[string]any{
				"entities": map[string]any{"characters": []any{
					map[string]any{"name": "Mira"},
					map[string]any{"name": "Rowan"},
				}},
				"evidence_excerpts": []any{excerpt},
				"protected_secrets": []any{map[string]any{
					"secret_kind": "vault_map", "owner": "Mira",
					"subject": []any{"Mira"}, "summary": "Mira owns the vault map.",
					"transition": tc.transition, "evidence_excerpt": excerpt,
					"knowledge_scope": map[string]any{"revealed_to": []any{"Rowan"}},
				}},
			}
			result := srv.saveCriticExtractionArtifacts(
				acceptedPreciseMemoryContext("revision-reveal-"+strings.ReplaceAll(tc.name, " ", "-")),
				"session-reveal", 9, extraction, content,
				completeTurnEmbeddingConfig{}, time.Unix(900, 0),
			)
			if result.PreciseMemoryUnits != 1 {
				t.Fatalf("reveal unit missing: result=%+v", result)
			}
			unit := preciseMemoryUnitByKind(t, fake, "observation")
			if unit.EpistemicMode != "revealed" ||
				unit.AdmissionState != tc.admission || unit.ReviewState != tc.review {
				t.Fatalf("reveal validation mismatch: %+v", unit)
			}
		})
	}
}
