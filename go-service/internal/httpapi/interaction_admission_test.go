package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func testEntityScalarKG(semanticClass, subject, entityKind, predicate, object, scalarType, evidence string) map[string]any {
	return map[string]any{
		"subject":          subject,
		"predicate":        predicate,
		"object":           object,
		"evidence_excerpt": evidence,
	}
}

func TestInteractionAdmissionPreservesBroadDirectionalSemanticCandidates(t *testing.T) {
	source := `A said, "I love B."`
	raw := map[string]any{
		"relationship_observations": []any{
			map[string]any{
				"source_entity": "A", "target_entity": "B", "domain": "romantic",
				"source_entity_expression": "A", "target_entity_expression": "B", "domain_expression": "love",
				"observation": "I love B", "support_kind": "explicit_statement",
				"evidence_excerpt": source,
			},
			map[string]any{
				"source_entity": "B", "target_entity": "A", "domain": "romantic",
				"source_entity_expression": "B", "target_entity_expression": "A", "domain_expression": "loves",
				"observation": "B loves A", "support_kind": "explicit_statement",
				"evidence_excerpt": source,
			},
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["relationship_observations"])
	if len(items) != 2 {
		t.Fatalf("broad relationship collection count=%d, want 2: %#v", len(items), items)
	}
	committed := mapFromAny(items[0])
	review := mapFromAny(items[1])
	if stringFromMap(committed, "source_entity") != "A" ||
		stringFromMap(committed, "target_entity") != "B" ||
		stringFromMap(committed, "domain") != "romantic" ||
		stringFromMap(committed, "admission_state") != "committed" {
		t.Fatalf("source-bound relationship direction/domain changed: %#v", committed)
	}
	if stringFromMap(review, "source_entity") != "B" ||
		stringFromMap(review, "target_entity") != "A" ||
		stringFromMap(review, "admission_state") != "committed" {
		t.Fatalf("understandable reciprocal candidate was erased by exact-expression proof: %#v", review)
	}
}

func TestInteractionAdmissionKeepsHelpAtomicAndRetainsRelationshipCandidatesForReview(t *testing.T) {
	source := "A helped B lift the fallen beam. The beam is lifted."
	raw := map[string]any{
		"interaction_events": []any{map[string]any{
			"actor": "A", "actor_expression": "A", "counterpart": "B", "counterpart_expression": "B",
			"action": "helped B lift the fallen beam", "action_expression": "helped B lift the fallen beam",
			"evidence_excerpt": source,
		}},
		"relationship_observations": []any{
			map[string]any{
				"source_entity": "B", "target_entity": "A", "domain": "trust",
				"observation": "B trusts A", "support_kind": "explicit_observed_state",
				"evidence_excerpt": source,
			},
			map[string]any{
				"source_entity": "B", "target_entity": "A", "domain": "intimacy",
				"observation": "B feels close to A", "support_kind": "explicit_observed_state",
				"evidence_excerpt": source,
			},
		},
		"character_deltas": []any{map[string]any{
			"name":          "B",
			"relationships": map[string]any{"A": map[string]any{"trust": 1}},
			"events": []any{
				map[string]any{"type": "relationship_shift", "detail": "B trusts A"},
				map[string]any{"type": "action", "detail": "B stood up"},
			},
		}},
		"narrative_events": []any{
			map[string]any{"event_type": "relationship_shift", "summary": "B trusts A", "evidence_excerpt": source},
			map[string]any{"event_type": "action", "summary": "A lifted the beam", "evidence_excerpt": source},
		},
		"kg_triples": []any{
			map[string]any{"semantic_class": "relationship_observation", "subject": "B", "predicate": "trust", "object": "A"},
			testEntityScalarKG("state_fact", "beam", "item", "is", "lifted", "state", "The beam is lifted."),
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	if len(sliceFromAny(admitted["interaction_events"])) != 1 ||
		len(sliceFromAny(admitted["relationship_observations"])) != 2 {
		t.Fatalf("broad interaction collection lost candidates: %#v", admitted)
	}
	for _, rawRelationship := range sliceFromAny(admitted["relationship_observations"]) {
		if stringFromMap(mapFromAny(rawRelationship), "admission_state") != "committed" {
			t.Fatalf("understandable relationship candidate was blocked by exact-expression proof: %#v", rawRelationship)
		}
	}
	delta := mapFromAny(sliceFromAny(admitted["character_deltas"])[0])
	if _, exists := delta["relationships"]; !exists || len(sliceFromAny(delta["events"])) != 2 {
		t.Fatalf("raw character observations were removed during collection: %#v", delta)
	}
	if len(sliceFromAny(admitted["narrative_events"])) != 2 ||
		len(sliceFromAny(admitted["kg_triples"])) != 2 {
		t.Fatalf("raw extraction content was removed during interaction collection: %#v", admitted)
	}
}

func TestInteractionAdmissionDoesNotTurnCommandComplianceIntoLoyaltyOrConsent(t *testing.T) {
	source := "B carried out A's order."
	raw := map[string]any{
		"interaction_events": []any{map[string]any{
			"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
			"action": "carried out A's order", "action_expression": "carried out A's order",
			"evidence_excerpt": source,
		}},
		"relationship_observations": []any{map[string]any{
			"source_entity": "B", "target_entity": "A", "domain": "obedience",
			"observation": "B is loyal to A", "support_kind": "explicit_observed_state",
			"evidence_excerpt": source,
		}},
		"interaction_boundaries": []any{map[string]any{
			"actor": "B", "counterpart": "A", "action_scope": "all future orders",
			"decision": "allow", "evidence_excerpt": source,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	if len(sliceFromAny(admitted["interaction_events"])) != 1 ||
		len(sliceFromAny(admitted["relationship_observations"])) != 1 ||
		len(sliceFromAny(admitted["interaction_boundaries"])) != 1 {
		t.Fatalf("broad interaction collection lost candidates: %#v", admitted)
	}
	if stringFromMap(mapFromAny(sliceFromAny(admitted["relationship_observations"])[0]), "admission_state") != "committed" ||
		stringFromMap(mapFromAny(sliceFromAny(admitted["interaction_boundaries"])[0]), "admission_state") != "committed" {
		t.Fatalf("semantic observations were removed or their review metadata was lost: %#v", admitted)
	}
}

func TestInteractionBoundaryDefaultsToEventAndWithdrawalWinsSameScope(t *testing.T) {
	source := `B told A, "A may hold B's hand now." Later B told A, "A must not hold B's hand; I withdraw permission."`
	raw := map[string]any{
		"interaction_boundaries": []any{
			map[string]any{
				"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
				"action_scope": "hold B's hand", "action_scope_expression": "hold B's hand",
				"decision": "allow", "decision_expression": "may", "support_kind": "explicit_statement",
				"evidence_excerpt": `B told A, "A may hold B's hand now."`,
			},
			map[string]any{
				"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
				"action_scope": "hold B's hand", "action_scope_expression": "hold B's hand",
				"decision": "withdrawn", "decision_expression": "withdraw permission", "support_kind": "explicit_statement",
				"evidence_excerpt": `B told A, "A must not hold B's hand; I withdraw permission."`,
			},
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["interaction_boundaries"])
	if len(items) != 2 {
		t.Fatalf("same-turn boundary observations were not both collected: %#v", items)
	}
	for _, rawBoundary := range items {
		boundary := mapFromAny(rawBoundary)
		if stringFromMap(boundary, "admission_state") != "committed" ||
			stringFromMap(boundary, "effective_scope") != "event" ||
			stringFromMap(boundary, "visibility") != "public" {
			t.Fatalf("source-bound boundary observation was not preserved: %#v", boundary)
		}
	}
}

func TestInteractionVisibilityDefaultsPublicAndPreservesExplicitPrivate(t *testing.T) {
	publicEvidence := `A publicly said, "I trust B."`
	source := publicEvidence + ` B told A, "Do not touch the sealed letter."`
	raw := map[string]any{
		"relationship_observations": []any{map[string]any{
			"source_entity": "A", "target_entity": "B", "domain": "trust",
			"source_entity_expression": "A", "target_entity_expression": "B", "domain_expression": "trust",
			"observation": "I trust B", "support_kind": "explicit_statement",
			"visibility": "public", "evidence_excerpt": publicEvidence,
			"public_visibility_support": map[string]any{
				"contract_version": "public_visibility_support.v1",
				"support_kind":     "explicit_public_statement", "visibility_assertion": publicEvidence,
			},
		}},
		"interaction_boundaries": []any{map[string]any{
			"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
			"action_scope": "touch the sealed letter", "action_scope_expression": "touch the sealed letter",
			"decision": "refuse", "decision_expression": "Do not", "support_kind": "explicit_statement",
			"visibility":       "owner_private",
			"evidence_excerpt": `B told A, "Do not touch the sealed letter."`,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	relationship := mapFromAny(sliceFromAny(admitted["relationship_observations"])[0])
	boundary := mapFromAny(sliceFromAny(admitted["interaction_boundaries"])[0])
	if stringFromMap(relationship, "visibility") != "public" {
		t.Fatalf("explicit public relationship visibility was changed: %#v", relationship)
	}
	if stringFromMap(boundary, "visibility") != "owner_private" {
		t.Fatalf("explicit private boundary visibility was changed: %#v", boundary)
	}
}

func TestWhisperedRelationshipCannotGainPublicVisibilityFromShortExpression(t *testing.T) {
	evidence := `A whispered to B, "I trust you."`
	raw := map[string]any{
		"relationship_observations": []any{map[string]any{
			"source_entity": "A", "source_entity_expression": "A",
			"target_entity": "B", "target_entity_expression": "B",
			"domain": "trust", "domain_expression": "trust",
			"observation": "I trust you", "support_kind": "explicit_statement",
			"visibility": "public", "visibility_expression": "A", "evidence_excerpt": evidence,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, evidence, "")
	item := mapFromAny(sliceFromAny(admitted["relationship_observations"])[0])
	if stringFromMap(item, "visibility") != "public" || len(mapFromAny(item["public_visibility_support"])) != 0 {
		t.Fatalf("explicit public visibility was downgraded for missing proof metadata: %#v", item)
	}
}

func TestInteractionPublicVisibilityWithoutExactExpressionStaysPublic(t *testing.T) {
	source := `A said, "I trust B."`
	raw := map[string]any{
		"relationship_observations": []any{map[string]any{
			"source_entity": "A", "source_entity_expression": "A",
			"target_entity": "B", "target_entity_expression": "B",
			"domain": "trust", "domain_expression": "trust",
			"observation": `A said, "I trust B."`, "support_kind": "explicit_statement",
			"visibility": "public", "evidence_excerpt": source,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	item := mapFromAny(sliceFromAny(admitted["relationship_observations"])[0])
	if stringFromMap(item, "visibility") != "public" ||
		stringFromMap(item, "visibility_disposition") != "public_or_unspecified" {
		t.Fatalf("public visibility was downgraded for missing redundant proof: %#v", item)
	}
}

func TestUserAndRPProfileNamespacesStaySeparatedAndOutOfWorldLanes(t *testing.T) {
	userEvidence := "[OOC] Do not describe gore."
	rpEvidence := "Mira said that she dislikes rain."
	source := userEvidence + "\n" + rpEvidence
	raw := map[string]any{
		"turn_summary": "Mira said that she dislikes rain.",
		"user_interaction_profile": []any{map[string]any{
			"profile_key": "gore", "profile_key_expression": "gore",
			"value": "forbidden", "value_expression": "Do not describe gore", "evidence_excerpt": userEvidence,
		}},
		"rp_character_profile": []any{map[string]any{
			"character": "Mira", "profile_key": "weather preference",
			"character_expression": "Mira", "value": "dislikes rain", "value_expression": "dislikes rain", "evidence_excerpt": rpEvidence,
		}},
		"narrative_events": []any{map[string]any{
			"event_type": "dialogue", "summary": "The user forbade gore.",
			"evidence_excerpt": userEvidence,
		}},
		"kg_triples": []any{map[string]any{
			"subject": "user", "predicate": "forbids", "object": "gore",
			"evidence_excerpt": userEvidence,
		}},
	}
	admitted, trace := admitCriticInteractionLanes(raw, source, "")
	users := sliceFromAny(admitted["user_interaction_profile"])
	rp := sliceFromAny(admitted["rp_character_profile"])
	if len(users) != 1 || len(rp) != 1 ||
		stringFromMap(mapFromAny(users[0]), "namespace") != "user_interaction_profile" ||
		stringFromMap(mapFromAny(rp[0]), "namespace") != "rp_character_profile" ||
		stringFromMap(mapFromAny(rp[0]), "admission_state") != "review_required" {
		t.Fatalf("profile namespace mismatch: %#v", admitted)
	}
	if len(sliceFromAny(admitted["narrative_events"])) != 0 ||
		len(sliceFromAny(admitted["kg_triples"])) != 0 {
		t.Fatalf("user-profile evidence leaked into in-world lanes: %#v", admitted)
	}
	proposals := sliceFromAny(trace["user_profile_review_proposals"])
	if len(proposals) != 1 || stringFromMap(mapFromAny(proposals[0]), "review_state") != "ooc_class_unobserved" {
		t.Fatalf("unobserved user profile was not retained in admission trace: %#v", trace)
	}
	rpProposals := sliceFromAny(trace["rp_profile_review_proposals"])
	if len(rpProposals) != 1 || stringFromMap(mapFromAny(rpProposals[0]), "review_state") != "stable_in_world_identity_unverified" {
		t.Fatalf("unverified RP profile was not retained only for review: %#v", trace)
	}

	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	result := srv.saveCriticExtractionArtifacts(
		context.Background(), "profile-session", 1, admitted, source,
		completeTurnEmbeddingConfig{}, time.Unix(100, 0),
	)
	if len(fake.savedMemories) == 0 {
		t.Fatalf("expected ordinary RP turn memory save, result=%#v", result)
	}
	if strings.Contains(fake.savedMemories[0].SummaryJSON, "forbidden") {
		t.Fatalf("unobserved user profile persisted in generic memory: %s", fake.savedMemories[0].SummaryJSON)
	}
}

func TestInteractionAdmissionStoresSemanticActionsWithoutExactExpressions(t *testing.T) {
	source := "A waved to B."
	raw := map[string]any{
		"interaction_events": []any{
			map[string]any{
				"actor": "A", "counterpart": "B", "action": "waved to B",
				"evidence_excerpt": source,
			},
			map[string]any{
				"actor": "A", "actor_expression": "A", "counterpart": "B", "counterpart_expression": "B",
				"action": "kissed B", "action_expression": "waved to B", "evidence_excerpt": source,
			},
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["interaction_events"])
	if len(items) != 2 {
		t.Fatalf("structurally complete interaction candidates were not collected: %#v", admitted)
	}
	for _, rawItem := range items {
		if stringFromMap(mapFromAny(rawItem), "admission_state") != "committed" {
			t.Fatalf("source-occurring semantic action was not retained: %#v", rawItem)
		}
	}
}

func TestRelationshipSemanticCandidatesDoNotRequireExactExpressions(t *testing.T) {
	source := `A told B, "I love B."`
	raw := map[string]any{
		"relationship_observations": []any{
			map[string]any{
				"source_entity": "B", "source_entity_expression": "A",
				"target_entity": "A", "target_entity_expression": "B",
				"domain": "romantic", "domain_expression": "love",
				"observation": `A told B, "I love B."`, "support_kind": "explicit_statement",
				"evidence_excerpt": source,
			},
			map[string]any{
				"source_entity": "A", "source_entity_expression": "A",
				"target_entity": "B", "target_entity_expression": "B",
				"domain": "trust", "domain_expression": "loyalty",
				"observation": `A told B, "I love B."`, "support_kind": "explicit_statement",
				"evidence_excerpt": source,
			},
			map[string]any{
				"source_entity": "A", "source_entity_expression": "A",
				"target_entity": "B", "target_entity_expression": "B",
				"domain": "romantic", "domain_expression": "love",
				"observation": `A told B, "I love B."`, "support_kind": "explicit_statement",
				"evidence_excerpt": source,
			},
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["relationship_observations"])
	if len(items) != 3 {
		t.Fatalf("relationship collection lost structurally complete candidates: %#v", admitted)
	}
	committed := 0
	for _, rawItem := range items {
		if stringFromMap(mapFromAny(rawItem), "admission_state") == "committed" {
			committed++
		}
	}
	if committed != len(items) {
		t.Fatalf("semantic relationship candidates were exact-expression gated: committed=%d items=%#v", committed, items)
	}
}

func TestBoundarySemanticCandidatesDoNotRequireExactExpressions(t *testing.T) {
	source := `B told A, "Do not touch the seal."`
	raw := map[string]any{
		"interaction_boundaries": []any{
			map[string]any{
				"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
				"action_scope": "touch the seal", "action_scope_expression": "touch the seal",
				"decision": "allow", "decision_expression": "allow", "support_kind": "explicit_statement", "evidence_excerpt": source,
			},
			map[string]any{
				"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
				"action_scope": "all future contact", "action_scope_expression": "touch the seal",
				"decision": "refuse", "decision_expression": "Do not", "support_kind": "explicit_statement", "evidence_excerpt": source,
			},
			map[string]any{
				"actor": "B", "actor_expression": "B", "counterpart": "A", "counterpart_expression": "A",
				"action_scope": "touch the seal", "action_scope_expression": "touch the seal",
				"decision": "refuse", "decision_expression": "Do not", "support_kind": "explicit_statement", "evidence_excerpt": source,
			},
		},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["interaction_boundaries"])
	if len(items) != 3 {
		t.Fatalf("boundary collection lost structurally complete candidates: %#v", admitted)
	}
	committed := 0
	for _, rawItem := range items {
		if stringFromMap(mapFromAny(rawItem), "admission_state") == "committed" {
			committed++
		}
	}
	if committed != len(items) {
		t.Fatalf("semantic boundary candidates were exact-expression gated: committed=%d items=%#v", committed, items)
	}
}

func TestRPProfileUnboundValueIsRetainedForReview(t *testing.T) {
	source := "Mira dislikes rain."
	raw := map[string]any{
		"rp_character_profile": []any{map[string]any{
			"character": "Mira", "character_expression": "Mira", "profile_key": "weather preference",
			"value": "loves rain", "value_expression": "dislikes rain", "evidence_excerpt": source,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	profiles := sliceFromAny(admitted["rp_character_profile"])
	if len(profiles) != 1 || stringFromMap(mapFromAny(profiles[0]), "admission_state") != "review_required" {
		t.Fatalf("unbound RP profile candidate was removed or committed: %#v", admitted)
	}
}

func TestUserProfileEvidenceQuarantineDoesNotClassifyLiteralOOCText(t *testing.T) {
	literal := `Mira quoted the label "[OOC]" from the screen.`
	ordinary, _ := admitCriticInteractionLanes(map[string]any{
		"narrative_events": []any{map[string]any{"event_type": "dialogue", "evidence_excerpt": literal}},
	}, literal, "")
	if len(sliceFromAny(ordinary["narrative_events"])) != 1 {
		t.Fatalf("literal OOC text was classified without a typed proposal: %#v", ordinary)
	}

	ooc := "Do not narrate the user's private medical detail."
	raw := map[string]any{
		"user_interaction_profile": []any{map[string]any{
			"profile_key": "medical detail", "profile_key_expression": "medical detail",
			"value": "private", "value_expression": "private", "evidence_excerpt": ooc,
		}},
		"state_claims":      []any{map[string]any{"subject": "user", "evidence_excerpt": ooc}},
		"belief_updates":    []any{map[string]any{"perspective_owner": "Mira", "evidence_excerpt": ooc}},
		"world_rules":       []any{map[string]any{"key": "privacy", "evidence_excerpt": ooc}},
		"reversible_states": []any{map[string]any{"subject": "user", "evidence_excerpt": ooc}},
		"character_deltas":  []any{map[string]any{"name": "Mira", "evidence_excerpt": ooc}},
		"evidence_excerpts": []any{ooc},
	}
	admitted, trace := admitCriticInteractionLanes(raw, ooc, "")
	for _, lane := range []string{"state_claims", "belief_updates", "world_rules", "reversible_states", "character_deltas", "evidence_excerpts"} {
		if len(sliceFromAny(admitted[lane])) != 0 && len(stringsFromAny(admitted[lane])) != 0 {
			t.Fatalf("typed OOC profile evidence survived lane %s: %#v", lane, admitted[lane])
		}
	}
	if intFromAny(mapFromAny(trace["reasons"])["user_profile_shared_evidence_quarantined:state_claims"], 0) != 1 {
		t.Fatalf("OOC structural quarantine was not traced: %#v", trace)
	}
}

func TestUserProfileSemanticStorageDoesNotRequireExactExpressionsOrEvidence(t *testing.T) {
	admitted, _ := admitCriticInteractionLanes(map[string]any{
		"user_interaction_profile": []any{map[string]any{
			"profile_key": "graphic detail",
			"value":       "avoid",
		}},
	}, "unrelated source wording", "")
	profiles := sliceFromAny(admitted["user_interaction_profile"])
	if len(profiles) != 1 {
		t.Fatalf("understandable user-private semantic profile was erased: %#v", admitted)
	}
	profile := mapFromAny(profiles[0])
	if stringFromMap(profile, "namespace") != "user_interaction_profile" ||
		stringFromMap(profile, "visibility") != "user_private" ||
		stringFromMap(profile, "evidence_excerpt") != "" {
		t.Fatalf("user-private boundary or honest empty evidence changed: %#v", profile)
	}
}

func TestUserProfileEvidenceQuarantineCatchesContainingAndContainedSpans(t *testing.T) {
	full := "The user explicitly requests that graphic injury descriptions remain omitted."
	partial := "graphic injury descriptions remain omitted"
	raw := map[string]any{
		"user_interaction_profile": []any{map[string]any{
			"profile_key": "graphic injury", "profile_key_expression": "graphic injury",
			"value": "descriptions remain omitted", "value_expression": "descriptions remain omitted",
			"evidence_excerpt": full,
		}},
		"narrative_events":  []any{map[string]any{"event_type": "state", "evidence_excerpt": partial}},
		"state_claims":      []any{map[string]any{"subject": "world", "evidence_excerpt": full + " Always."}},
		"evidence_excerpts": []any{partial},
	}
	admitted, trace := admitCriticInteractionLanes(raw, full+" Always.", "")
	for _, lane := range []string{"narrative_events", "state_claims", "evidence_excerpts"} {
		if len(sliceFromAny(admitted[lane])) != 0 && len(stringsFromAny(admitted[lane])) != 0 {
			t.Fatalf("overlapping OOC evidence survived lane %s: %#v", lane, admitted[lane])
		}
	}
	if intFromAny(mapFromAny(trace["reasons"])["user_profile_shared_evidence_quarantined:narrative_events"], 0) != 1 {
		t.Fatalf("overlap quarantine was not traced: %#v", trace)
	}
}

func TestUnverifiedRPProfileIsArchivedForReviewWithoutCurrentProjection(t *testing.T) {
	evidence := "The user says Mira dislikes rain."
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}}},
		"rp_character_profile": []any{map[string]any{
			"character": "Mira", "character_expression": "Mira", "profile_key": "weather preference",
			"value": "dislikes rain", "value_expression": "dislikes rain", "evidence_excerpt": evidence,
		}},
	}
	admitted, trace := admitCriticInteractionLanes(raw, evidence, "")
	profiles := sliceFromAny(admitted["rp_character_profile"])
	candidates := interactionAdmissionPreciseMemoryCandidates(admitted)
	if len(profiles) != 1 || stringFromMap(mapFromAny(profiles[0]), "admission_state") != "review_required" ||
		len(candidates) != 1 || candidates[0].admissionState != "review_required" {
		t.Fatalf("unverified RP profile was removed or committed: admitted=%#v candidates=%#v", admitted, candidates)
	}
	if len(sliceFromAny(mapFromAny(admitted["entities"])["characters"])) != 1 {
		t.Fatalf("broad entity collection removed the RP character: %#v", admitted["entities"])
	}
	if len(sliceFromAny(trace["rp_profile_review_proposals"])) != 1 {
		t.Fatalf("unverified RP proposal was not retained for review: %#v", trace)
	}

}

func TestRPProfileCommitsOnlyWithMatchingStableInWorldIdentityProof(t *testing.T) {
	evidence := "Mira dislikes rain."
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{
			"name": "Mira", "stable_entity_id": "entity-mira", "identity_namespace": "session_npc",
		}}},
		"rp_character_profile": []any{map[string]any{
			"character": "Mira", "character_expression": "Mira", "profile_key": "weather preference",
			"value": "dislikes rain", "value_expression": "dislikes rain", "evidence_excerpt": evidence,
			"identity_proof": map[string]any{
				"contract_version": "in_world_identity_proof.v1", "stable_entity_id": "entity-mira",
				"identity_namespace": "session_npc", "character_expression": "Mira",
			},
		}},
	}
	identityStore := &perspectiveIdentityTestStore{
		turnRecordingStore: &turnRecordingStore{},
		resolvedID:         "entity-mira",
		resolvedNamespace:  "session_npc",
	}
	srv := NewServer(config.Default())
	srv.Store = identityStore
	trusted := srv.resolveTrustedRPCharacterIdentities(context.Background(), "rp-commit", raw)
	admitted, trace := admitCriticInteractionLanesWithTrustedIdentities(raw, evidence, "", trusted)
	profiles := sliceFromAny(admitted["rp_character_profile"])
	if len(profiles) != 1 || stringFromMap(mapFromAny(profiles[0]), "admission_state") != "committed" ||
		len(sliceFromAny(trace["rp_profile_review_proposals"])) != 0 {
		t.Fatalf("matching stable identity proof did not commit RP profile: admitted=%#v trace=%#v", admitted, trace)
	}
	if identityStore.resolvedSurface != "mira" {
		t.Fatalf("stable identity proof did not use backend surface resolution: %q", identityStore.resolvedSurface)
	}
}

func TestRPProfileWithMismatchedNamespaceRemainsReviewOnly(t *testing.T) {
	evidence := "Mira dislikes rain."
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}}},
		"rp_character_profile": []any{map[string]any{
			"character": "Mira", "character_expression": "Mira", "profile_key": "weather preference",
			"value": "dislikes rain", "value_expression": "dislikes rain", "evidence_excerpt": evidence,
			"identity_proof": map[string]any{
				"contract_version": inWorldIdentityProofContract,
				"stable_entity_id": "entity-mira", "identity_namespace": "session_player",
				"character_expression": "Mira",
			},
		}},
	}
	identityStore := &perspectiveIdentityTestStore{
		turnRecordingStore: &turnRecordingStore{},
		resolvedID:         "entity-mira",
		resolvedNamespace:  "session_npc",
	}
	srv := NewServer(config.Default())
	srv.Store = identityStore
	trusted := srv.resolveTrustedRPCharacterIdentities(context.Background(), "rp-namespace", raw)
	admitted, trace := admitCriticInteractionLanesWithTrustedIdentities(raw, evidence, "", trusted)
	profiles := sliceFromAny(admitted["rp_character_profile"])
	if len(profiles) != 1 || stringFromMap(mapFromAny(profiles[0]), "admission_state") != "review_required" ||
		len(sliceFromAny(trace["rp_profile_review_proposals"])) != 1 {
		t.Fatalf("critic namespace overrode database namespace: admitted=%#v trace=%#v", admitted, trace)
	}
}

func TestLegacyTypedRelationshipCandidatesRemainInBroadCollection(t *testing.T) {
	typed := func() map[string]any {
		return map[string]any{
			"semantic_class": "relationship_observation", "source_entity": "A", "target_entity": "B",
			"relation_domain": "trust", "evidence_excerpt": "A trusts B.",
		}
	}
	raw := map[string]any{
		"narrative_events":           []any{typed()},
		"state_claims":               []any{typed()},
		"belief_updates":             []any{typed()},
		"subjective_entity_memories": []any{typed()},
		"kg_triples":                 []any{typed()},
	}
	admitted, _ := admitCriticInteractionLanes(raw, "A trusts B.", "")
	for _, lane := range []string{"narrative_events", "state_claims", "belief_updates", "subjective_entity_memories"} {
		if len(sliceFromAny(admitted[lane])) != 1 {
			t.Fatalf("typed relationship candidate was removed from lane %s: %#v", lane, admitted[lane])
		}
	}
	if len(sliceFromAny(admitted["kg_triples"])) != 1 {
		t.Fatalf("KG relationship was filtered by a parallel content rule: %#v", admitted["kg_triples"])
	}
}

func TestSourceBoundGenericKGIsAdmitted(t *testing.T) {
	raw := map[string]any{
		"kg_triples": []any{
			map[string]any{"subject": "A", "predicate": "loves", "object": "B", "evidence_excerpt": "A loves B."},
			map[string]any{"subject": "A", "predicate": "is_loyal_to", "object": "B", "evidence_excerpt": "A is loyal to B."},
			testEntityScalarKG("state_fact", "beam", "item", "is", "lifted", "state", "The beam is lifted."),
		},
	}
	admitted, trace := admitCriticInteractionLanes(raw, "A loves B. A is loyal to B. The beam is lifted.", "")
	items := sliceFromAny(admitted["kg_triples"])
	if len(items) != 3 {
		t.Fatalf("source-bound KG was suppressed: admitted=%#v trace=%#v", admitted, trace)
	}
}

func TestKGAdmissionDoesNotApplyContentRules(t *testing.T) {
	evidence := "Ayoung described the concept as bedroom lingerie."
	raw := map[string]any{
		"kg_triples": []any{
			map[string]any{"subject": "Ayoung", "predicate": "described_as", "object": "bedroom lingerie", "evidence_excerpt": evidence},
			map[string]any{"subject": "Ayoung", "predicate": "described_as", "object": "uniform", "evidence_excerpt": "not in this turn"},
			map[string]any{"subject": "Ayoung", "predicate": "described_as", "evidence_excerpt": evidence},
		},
	}

	admitted, trace := admitCriticInteractionLanes(raw, evidence, "")
	items := sliceFromAny(admitted["kg_triples"])
	if len(items) != 3 {
		t.Fatalf("KG admission applied a content rule: admitted=%#v trace=%#v", admitted, trace)
	}
	if len(mapFromAny(trace["reasons"])) != 0 {
		t.Fatalf("KG admission produced content rejection reasons: %#v", trace)
	}
}

func TestOOCGuardUsesOnlyTypedHostObservation(t *testing.T) {
	if shouldApplyCompleteTurnOOCGuard(map[string]any{
		"risu_request_observation": map[string]any{
			"contract_version": "risu_request_observation.v1",
			"ooc_class_state":  "not_exposed",
			"ooc_class":        "ooc",
		},
	}) {
		t.Fatal("not_exposed OOC class activated the guard")
	}
	if !shouldApplyCompleteTurnOOCGuard(map[string]any{
		"risu_request_observation": map[string]any{
			"contract_version": "risu_request_observation.v1",
			"ooc_class_state":  "observed",
			"ooc_class":        "ooc",
		},
	}) {
		t.Fatal("future typed observed OOC class did not activate the guard")
	}
	if shouldApplyCompleteTurnOOCGuard(map[string]any{
		"risu_request_observation": map[string]any{
			"contract_version":   "risu_request_observation.v1",
			"ooc_class_state":    "not_exposed",
			"request_type":       `model containing a quoted "[OOC]" string`,
			"request_type_state": "observed",
		},
	}) {
		t.Fatal("quoted OOC text was treated as host classification")
	}
}

func TestExplicitSingleSceneRelationshipChangeIsAdmittedWithoutCountThreshold(t *testing.T) {
	source := "B loves A."
	raw := map[string]any{
		"relationship_observations": []any{map[string]any{
			"source_entity": "B", "target_entity": "A", "domain": "romantic",
			"source_entity_expression": "B", "target_entity_expression": "A", "domain_expression": "loves",
			"observation": "B loves A", "support_kind": "explicit_observed_state",
			"magnitude": "major", "duration": "ongoing",
			"evidence_excerpt": source,
		}},
	}
	admitted, _ := admitCriticInteractionLanes(raw, source, "")
	items := sliceFromAny(admitted["relationship_observations"])
	if len(items) != 1 {
		t.Fatalf("explicit single-scene relation was rejected: %#v", admitted)
	}
	relationship := mapFromAny(items[0])
	if extractionStringFromAny(relationship["magnitude"]) != "major" || extractionStringFromAny(relationship["duration"]) != "ongoing" {
		t.Fatalf("relationship archive lost the extracted magnitude or duration: %#v", relationship)
	}
	candidates := interactionAdmissionPreciseMemoryCandidates(admitted)
	if len(candidates) != 1 ||
		candidates[0].relationshipKey != "b->a/romantic" ||
		candidates[0].admissionState != "committed" ||
		candidates[0].reviewState != "source_observed" {
		t.Fatalf("directional precise candidate mismatch: %#v", candidates)
	}
	if candidates[0].payload["magnitude"] != "major" {
		t.Fatalf("semantic magnitude was erased without an exact expression: %#v", candidates[0].payload)
	}
	if candidates[0].payload["duration"] != "ongoing" {
		t.Fatalf("semantic duration was erased without an exact expression: %#v", candidates[0].payload)
	}
}
