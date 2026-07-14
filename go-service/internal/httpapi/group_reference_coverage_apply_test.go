package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestReferenceCoverageApplicationInjectsOnlyPartialAndMissingWithoutSyntheticChromaScores(t *testing.T) {
	distance := 0.27
	cosine := 0.73
	scopes := map[string]referenceRecallScope{
		"binding-1": {
			binding: store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", Priority: 10},
			work:    &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
			entities: map[string]store.ReferenceEntity{
				"entity-mira": {EntityID: "entity-mira", CanonicalName: "Mira", DescriptionText: "Leader of HUNTR/X."},
			},
			claims: map[string]store.ReferenceClaim{
				"claim-gate": {ClaimID: "claim-gate", ClaimText: "The gate opens only at night."},
			},
			nodes: map[string]store.ReferenceTimelineNode{},
		},
	}
	selected := []referenceRecallItem{
		{BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "claim-gate", ChromaRank: 7, Distance: &distance, CosineSimilarity: &cosine},
		{BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1", ReferenceKind: "entity", SourceID: "not-needed", ChromaRank: 8},
	}
	fieldIndex := newReferenceCoverageFieldIndexSummary()
	fieldIndex.NeededSourceItems = []referenceCoverageNeededSource{
		{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "claim-gate", CoverageStatus: "partial", MissingFields: []string{"claim_text"}, NeededBy: []string{"explicit_user_subject_mention"}, Eligible: true},
		{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceKind: "entity", SourceID: "entity-mira", CoverageStatus: "missing", MissingFields: []string{"identity_name", "description_text"}, NeededBy: []string{"recent_completed_dialogue"}, Eligible: true},
		{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "covered", CoverageStatus: "covered", Eligible: true},
		{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "conflict", CoverageStatus: "conflict", Eligible: true},
		{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "unknown", CoverageStatus: "unknown", Eligible: true},
	}

	items, summary := buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{scopes["binding-1"].binding}, scopes, selected, fieldIndex, 4)
	if len(items) != 2 || summary.AppliedCount != 2 || summary.ChromaAppliedCount != 1 || summary.FieldIndexApplied != 1 {
		t.Fatalf("application result items=%#v summary=%#v", items, summary)
	}
	if items[0].SourceID != "claim-gate" || items[0].SelectionSource != "chroma_candidate" || items[0].ChromaRank == nil || *items[0].ChromaRank != 7 {
		t.Fatalf("raw Chroma provenance changed: %#v", items[0])
	}
	if items[0].Distance == nil || *items[0].Distance != distance || items[0].CosineSimilarity == nil || *items[0].CosineSimilarity != cosine {
		t.Fatalf("raw Chroma measurements changed: %#v", items[0])
	}
	if items[1].SourceID != "entity-mira" || items[1].SelectionSource != "coverage_field_index" || items[1].ChromaRank != nil || items[1].Distance != nil || items[1].CosineSimilarity != nil {
		t.Fatalf("field index item received a synthetic Chroma score: %#v", items[1])
	}
	if summary.SkippedStatusCounts["covered"] != 1 || summary.SkippedStatusCounts["conflict"] != 1 || summary.SkippedStatusCounts["unknown"] != 1 || summary.SkippedNoSceneNeed != 1 {
		t.Fatalf("blocked coverage statuses = %#v", summary)
	}
}

func TestReferenceCoverageApplicationDoesNotInjectCoveredSource(t *testing.T) {
	scope := referenceRecallScope{
		binding:  store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1"},
		work:     &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
		claims:   map[string]store.ReferenceClaim{"claim-1": {ClaimID: "claim-1", ClaimText: "Already present."}},
		entities: map[string]store.ReferenceEntity{},
		nodes:    map[string]store.ReferenceTimelineNode{},
	}
	index := newReferenceCoverageFieldIndexSummary()
	index.NeededSourceItems = []referenceCoverageNeededSource{{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "claim-1", CoverageStatus: "covered", Eligible: true}}
	items, summary := buildReferenceCoverageInjectionItems(nil, map[string]referenceRecallScope{"binding-1": scope}, []referenceRecallItem{{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "claim-1", ChromaRank: 1}}, index, 3)
	if len(items) != 0 || summary.SkippedStatusCounts["covered"] != 1 {
		t.Fatalf("covered source was injected: items=%#v summary=%#v", items, summary)
	}
}
