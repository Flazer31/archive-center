package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestReferenceRecallNarratorOnlyGuidesNarrationWithoutBecomingCharacterKnowledge(t *testing.T) {
	scope := referenceRecallScope{branchKey: "main", nodes: map[string]store.ReferenceTimelineNode{}, sceneEntities: map[string]bool{}}
	narratorClaim := store.ReferenceClaim{BranchKey: "main", TemporalScope: "timeless", KnowledgeScope: "narrator_only"}
	if eligible, reason := referenceRecallClaimEligible(scope, narratorClaim); !eligible || reason != "eligible_narrator_only" {
		t.Fatalf("narrator-only claim eligible=%v reason=%s", eligible, reason)
	}
	privateClaim := store.ReferenceClaim{BranchKey: "main", TemporalScope: "timeless", KnowledgeScope: "entity_scoped", KnowerEntityIDs: []string{"member-1"}}
	if eligible, reason := referenceRecallClaimEligible(scope, privateClaim); eligible || reason != "knowledge_scope_not_in_scene" {
		t.Fatalf("entity-scoped claim leaked without a scene knower: eligible=%v reason=%s", eligible, reason)
	}
	if note := referenceKnowledgeScopeInstruction("narrator_only"); !strings.Contains(note, "do not grant this knowledge to characters") {
		t.Fatalf("narrator-only instruction missing: %q", note)
	}
}

func TestPrimaryReferenceRecallBundlesApprovedRelationshipWithoutFakeChromaScore(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceMode: referenceModePrimary},
		work:    &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
		entities: map[string]store.ReferenceEntity{
			"group-1": {EntityID: "group-1", CanonicalName: "HUNTR/X", EntityType: "faction"},
			"other-1": {EntityID: "other-1", CanonicalName: "Other Group", EntityType: "faction"},
		},
		aliases: map[string][]string{"group-1": {"Huntrix"}},
		claims: map[string]store.ReferenceClaim{
			"claim-seed": {
				ClaimID: "claim-seed", ClaimType: "relationship", SubjectEntityID: "group-1", ClaimText: "HUNTR/X members are Hunters.",
				TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "narrator_only", Confidence: 1,
			},
			"claim-members": {
				ClaimID: "claim-members", ClaimType: "relationship", SubjectEntityID: "group-1", ClaimText: "HUNTR/X consists of Rumi, Mira, and Zoey.",
				TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "narrator_only", Confidence: 1,
			},
			"claim-unrelated": {
				ClaimID: "claim-unrelated", ClaimType: "relationship", SubjectEntityID: "other-1", ClaimText: "Other Group consists of unrelated people.",
				TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", Confidence: 1,
			},
		},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}
	seed := referenceRecallItem{
		BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1",
		ReferenceKind: "claim", SourceID: "claim-seed", Text: "HUNTR/X members are Hunters.", ChromaRank: 1,
		Eligible: true, Reason: "eligible_narrator_only", Needed: true, NeededBy: []string{"primary_chroma_relevance"}, CoverageStatus: "missing",
		Metadata: map[string]any{"claim_type": "relationship", "subject_entity_id": "group-1", "knowledge_scope": "narrator_only"},
	}
	messages := []map[string]any{{"role": "system", "content": "The scene opens on a city street."}}
	companions := buildPrimaryReferenceRelationCompanions(map[string]referenceRecallScope{"binding-1": scope}, []referenceRecallItem{seed}, "Continue.", messages, referenceCoverageSceneContext{}, 4)
	if len(companions) != 1 || companions[0].SourceID != "claim-members" {
		t.Fatalf("relationship companions = %#v", companions)
	}

	items, summary := buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{scope.binding}, map[string]referenceRecallScope{"binding-1": scope}, []referenceRecallItem{seed}, companions, newReferenceCoverageFieldIndexSummary(), 4)
	if len(items) != 2 || summary.RelationAppliedCount != 1 {
		t.Fatalf("relationship injection items=%#v summary=%#v", items, summary)
	}
	related := items[1]
	if related.SelectionSource != "primary_relation_expansion" || related.ChromaRank != nil || related.Distance != nil || related.CosineSimilarity != nil {
		t.Fatalf("relation companion fabricated Chroma provenance: %#v", related)
	}
	formatted := formatReferenceRecallInjection(referenceRecallResult{Status: "ready", InjectionItems: items}, 4000)
	if !containsAll(formatted.Text, "preserve exact memberships", "HUNTR/X consists of Rumi, Mira, and Zoey") {
		t.Fatalf("relationship instruction or fact missing: %#v", formatted)
	}
}

func TestPrimaryReferenceRelationCompanionDoesNotRepeatCoveredLore(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceMode: referenceModePrimary},
		claims: map[string]store.ReferenceClaim{
			"seed":    {ClaimID: "seed", ClaimType: "relationship", SubjectEntityID: "group-1", ClaimText: "HUNTR/X are Hunters.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world"},
			"members": {ClaimID: "members", ClaimType: "relationship", SubjectEntityID: "group-1", ClaimText: "HUNTR/X consists of Rumi, Mira, and Zoey.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world"},
		},
		entities:      map[string]store.ReferenceEntity{},
		aliases:       map[string][]string{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}
	seed := referenceRecallItem{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "seed", Text: "HUNTR/X are Hunters.", Eligible: true, Metadata: map[string]any{"subject_entity_id": "group-1"}}
	messages := []map[string]any{{"role": "system", "content": "HUNTR/X consists of Rumi, Mira, and Zoey."}}
	companions := buildPrimaryReferenceRelationCompanions(map[string]referenceRecallScope{"binding-1": scope}, []referenceRecallItem{seed}, "Continue.", messages, referenceCoverageSceneContext{}, 4)
	if len(companions) != 0 {
		t.Fatalf("covered relationship was repeated: %#v", companions)
	}
}
