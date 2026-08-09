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
	if reviewText != "" ||
		intFromAny(reviewDrops["superseded_epistemic_state"], 0) != 1 ||
		intFromAny(reviewDrops["latest_epistemic_state_requires_review"], 0) != 1 {
		t.Fatalf("latest review blocker allowed old knowledge to revive: packet=%#v text=%q", reviewPacket, reviewText)
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
		"search_result": "ok",
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
		"search_result": "ok",
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

func TestMemoryAdmissionDetectsPerspectiveScopedContent(t *testing.T) {
	for _, key := range []string{
		"belief_updates",
		"protected_secrets",
		"character_identity_accuracy",
		"subjective_entity_memories",
	} {
		if !memoryAdmissionHasPerspectiveScopedContent(map[string]any{
			key: []any{map[string]any{"value": "private"}},
		}) {
			t.Fatalf("%s did not require typed perspective delivery", key)
		}
	}
	if memoryAdmissionHasPerspectiveScopedContent(map[string]any{
		"state_claims": []any{map[string]any{"value": "public"}},
	}) {
		t.Fatal("objective state was incorrectly classified as perspective scoped")
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
		}},
	})
	if len(private.Vectors) != 0 {
		t.Fatalf("perspective-scoped turn entered general vectors: %#v", private.Vectors)
	}
	if len(private.Evidence) != 1 ||
		private.Evidence[0].EvidenceKind != "perspective_scoped_turn_excerpt" {
		t.Fatalf("perspective evidence was not tagged for fail-closed recall: %#v", private.Evidence)
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
	})
	if len(mixed.Vectors) != 1 ||
		mixed.Vectors[0].ArtifactType != "evidence" ||
		mixed.Vectors[0].EvidenceText != "The public bell rang." {
		t.Fatalf("mixed turn did not retain only public evidence vector: %#v", mixed.Vectors)
	}
	if len(mixed.Evidence) != 2 ||
		mixed.Evidence[0].EvidenceKind != "turn_excerpt" ||
		mixed.Evidence[1].EvidenceKind != "perspective_scoped_turn_excerpt" {
		t.Fatalf("mixed evidence classification mismatch: %#v", mixed.Evidence)
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
