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
				"claim-gate": {ClaimID: "claim-gate", DocumentID: "doc-1", ClaimText: "The gate opens only at night.", EvidenceExcerpt: "At midnight, the eastern gate opened.", MetadataJSON: `{"chunk_index":2,"evidence_grounded":true}`},
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

	items, summary := buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{scopes["binding-1"].binding}, scopes, selected, nil, fieldIndex, 4)
	if len(items) != 2 || summary.AppliedCount != 2 || summary.ChromaAppliedCount != 1 || summary.FieldIndexApplied != 1 {
		t.Fatalf("application result items=%#v summary=%#v", items, summary)
	}
	if items[0].SourceID != "claim-gate" || items[0].SelectionSource != "chroma_candidate" || items[0].ChromaRank == nil || *items[0].ChromaRank != 7 {
		t.Fatalf("raw Chroma provenance changed: %#v", items[0])
	}
	if items[0].Distance == nil || *items[0].Distance != distance || items[0].CosineSimilarity == nil || *items[0].CosineSimilarity != cosine {
		t.Fatalf("raw Chroma measurements changed: %#v", items[0])
	}
	if !items[0].SourceVerified || items[0].SourceDocumentID != "doc-1" || items[0].SourceChunkIndex == nil || *items[0].SourceChunkIndex != 2 || items[0].SourceExcerpt != "At midnight, the eastern gate opened." || items[0].ContentMode != "structured_plus_source" {
		t.Fatalf("structured and original source were not linked: %#v", items[0])
	}
	formatted := formatReferenceRecallInjection(referenceRecallResult{Status: "ready", InjectionItems: items[:1]}, 1000)
	if !containsAll(formatted.Text, "Structured: The gate opens only at night.", "Original excerpt: At midnight, the eastern gate opened.") || formatted.IncludedCount != 1 {
		t.Fatalf("combined source injection = %#v", formatted)
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
	items, summary := buildReferenceCoverageInjectionItems(nil, map[string]referenceRecallScope{"binding-1": scope}, []referenceRecallItem{{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "claim-1", ChromaRank: 1}}, nil, index, 3)
	if len(items) != 0 || summary.SkippedStatusCounts["covered"] != 1 {
		t.Fatalf("covered source was injected: items=%#v summary=%#v", items, summary)
	}
}

func TestReferenceCoverageApplicationDoesNotPresentUnverifiedExcerptAsOriginal(t *testing.T) {
	scope := referenceRecallScope{
		binding:  store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1"},
		work:     &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
		claims:   map[string]store.ReferenceClaim{"claim-1": {ClaimID: "claim-1", DocumentID: "doc-1", ClaimText: "Structured fact.", EvidenceExcerpt: "Unverified legacy excerpt."}},
		entities: map[string]store.ReferenceEntity{},
		nodes:    map[string]store.ReferenceTimelineNode{},
	}
	index := newReferenceCoverageFieldIndexSummary()
	index.NeededSourceItems = []referenceCoverageNeededSource{{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "claim-1", CoverageStatus: "missing", MissingFields: []string{"claim_text"}, Eligible: true}}
	items, _ := buildReferenceCoverageInjectionItems(nil, map[string]referenceRecallScope{"binding-1": scope}, nil, nil, index, 3)
	if len(items) != 1 || items[0].SourceVerified || items[0].SourceExcerpt != "" || items[0].ContentMode != "structured_only" {
		t.Fatalf("unverified excerpt was presented as original: %#v", items)
	}
}

func TestReferenceCoverageApplicationSeparatesSupplementPrimaryAndUnknownModes(t *testing.T) {
	distance := 0.19
	cosine := 0.81
	candidate := referenceRecallItem{
		BindingID:        "binding-1",
		WorkID:           "work-1",
		WorkTitle:        "Example",
		ContinuityID:     "continuity-1",
		ReferenceKind:    "claim",
		SourceID:         "claim-1",
		Text:             "Hunters seal demons through song.",
		ChromaRank:       2,
		Distance:         &distance,
		CosineSimilarity: &cosine,
		Eligible:         true,
		Reason:           "eligible",
		Needed:           true,
		NeededBy:         []string{"primary_chroma_relevance"},
		CoverageStatus:   "missing",
		DecisionReason:   "needed_claim_absent_from_coverage_sources",
	}
	makeScope := func(mode string) referenceRecallScope {
		return referenceRecallScope{
			binding:  store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceMode: mode},
			work:     &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
			claims:   map[string]store.ReferenceClaim{"claim-1": {ClaimID: "claim-1", ClaimText: candidate.Text}},
			entities: map[string]store.ReferenceEntity{},
			nodes:    map[string]store.ReferenceTimelineNode{},
		}
	}

	supplement := makeScope(referenceModeSupplement)
	items, summary := buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{supplement.binding}, map[string]referenceRecallScope{"binding-1": supplement}, []referenceRecallItem{candidate}, nil, newReferenceCoverageFieldIndexSummary(), 4)
	if len(items) != 0 || summary.SkippedNoSceneNeed != 1 {
		t.Fatalf("supplement mode injected unsupported scene context: items=%#v summary=%#v", items, summary)
	}

	primary := makeScope(referenceModePrimary)
	items, summary = buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{primary.binding}, map[string]referenceRecallScope{"binding-1": primary}, []referenceRecallItem{candidate}, nil, newReferenceCoverageFieldIndexSummary(), 4)
	if len(items) != 1 || items[0].ReferenceMode != referenceModePrimary || items[0].SelectionSource != "primary_chroma_candidate" || items[0].CoverageStatus != "primary_context" {
		t.Fatalf("primary mode did not use the real Chroma candidate: items=%#v summary=%#v", items, summary)
	}
	if items[0].ChromaRank == nil || *items[0].ChromaRank != 2 || items[0].Distance == nil || *items[0].Distance != distance || items[0].CosineSimilarity == nil || *items[0].CosineSimilarity != cosine {
		t.Fatalf("primary mode changed Chroma provenance: %#v", items[0])
	}
	formatted := formatReferenceRecallInjection(referenceRecallResult{Status: "ready", InjectionItems: items}, 1000)
	if !containsAll(formatted.Text, "user-selected primary canon source", candidate.Text) {
		t.Fatalf("primary mode instruction missing: %#v", formatted)
	}

	unknown := makeScope("invalid_mode")
	items, summary = buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{unknown.binding}, map[string]referenceRecallScope{"binding-1": unknown}, []referenceRecallItem{candidate}, nil, newReferenceCoverageFieldIndexSummary(), 4)
	if len(items) != 0 || summary.SkippedUnknownMode != 1 {
		t.Fatalf("unknown mode was not blocked: items=%#v summary=%#v", items, summary)
	}
}

func TestReferenceCoverageApplicationLetsRelationCompanionCompeteWithinTotalLimit(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1", ReferenceMode: referenceModePrimary},
		work:    &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
		claims: map[string]store.ReferenceClaim{
			"direct-1":    {ClaimID: "direct-1", ClaimText: "First direct fact."},
			"direct-2":    {ClaimID: "direct-2", ClaimText: "Second direct fact."},
			"companion-1": {ClaimID: "companion-1", ClaimText: "Related fact."},
		},
		entities: map[string]store.ReferenceEntity{},
		nodes:    map[string]store.ReferenceTimelineNode{},
	}
	selected := []referenceRecallItem{
		{BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "direct-1", Text: "First direct fact.", ChromaRank: 1, Eligible: true, CoverageStatus: "missing", NeededBy: []string{"primary_chroma_relevance"}, Reason: "eligible"},
		{BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "direct-2", Text: "Second direct fact.", ChromaRank: 2, Eligible: true, CoverageStatus: "missing", NeededBy: []string{"primary_chroma_relevance"}, Reason: "eligible"},
	}
	companions := []referenceRecallItem{
		{BindingID: "binding-1", WorkID: "work-1", WorkTitle: "Example", ContinuityID: "continuity-1", ReferenceKind: "claim", SourceID: "companion-1", Text: "Related fact.", Eligible: true, CoverageStatus: "missing"},
	}

	items, summary := buildReferenceCoverageInjectionItems([]store.SessionReferenceBinding{scope.binding}, map[string]referenceRecallScope{"binding-1": scope}, selected, companions, newReferenceCoverageFieldIndexSummary(), len(selected))
	if len(items) > len(selected) || len(items) != len(selected) {
		t.Fatalf("total limit not preserved: items=%#v", items)
	}
	if items[0].SourceID != "direct-1" || items[1].SourceID != "companion-1" || summary.RelationAppliedCount != 1 {
		t.Fatalf("relation companion did not compete after top direct candidate: items=%#v summary=%#v", items, summary)
	}
	if items[1].ChromaRank != nil || items[1].Distance != nil || items[1].CosineSimilarity != nil {
		t.Fatalf("relation companion received synthetic vector provenance: %#v", items[1])
	}
}

func TestReferenceCoverageApplicationPreservesTopDirectAcrossBindings(t *testing.T) {
	makeScope := func(bindingID string) referenceRecallScope {
		return referenceRecallScope{
			binding:  store.SessionReferenceBinding{BindingID: bindingID, WorkID: bindingID, ContinuityID: "main", ReferenceMode: referenceModePrimary},
			work:     &store.ReferenceWork{WorkID: bindingID, Title: bindingID},
			claims:   map[string]store.ReferenceClaim{"direct": {ClaimID: "direct", ClaimText: "Direct fact."}, "direct-2": {ClaimID: "direct-2", ClaimText: "Second direct fact."}, "companion-1": {ClaimID: "companion-1", ClaimText: "Related one."}, "companion-2": {ClaimID: "companion-2", ClaimText: "Related two."}},
			entities: map[string]store.ReferenceEntity{},
			nodes:    map[string]store.ReferenceTimelineNode{},
		}
	}
	first := makeScope("binding-a")
	second := makeScope("binding-b")
	direct := func(bindingID string) referenceRecallItem {
		return referenceRecallItem{BindingID: bindingID, WorkID: bindingID, ContinuityID: "main", ReferenceKind: "claim", SourceID: "direct", Text: "Direct fact.", Eligible: true, CoverageStatus: "missing", NeededBy: []string{"primary_chroma_relevance"}}
	}
	companions := []referenceRecallItem{
		{BindingID: "binding-a", WorkID: "binding-a", ContinuityID: "main", ReferenceKind: "claim", SourceID: "companion-1", Text: "Related one.", Eligible: true, CoverageStatus: "missing"},
		{BindingID: "binding-a", WorkID: "binding-a", ContinuityID: "main", ReferenceKind: "claim", SourceID: "companion-2", Text: "Related two.", Eligible: true, CoverageStatus: "missing"},
	}
	items, _ := buildReferenceCoverageInjectionItems(
		[]store.SessionReferenceBinding{first.binding, second.binding},
		map[string]referenceRecallScope{"binding-a": first, "binding-b": second},
		[]referenceRecallItem{direct("binding-a"), direct("binding-b")},
		companions,
		newReferenceCoverageFieldIndexSummary(),
		3,
	)
	if len(items) != 3 || items[0].BindingID != "binding-a" || items[1].BindingID != "binding-b" || items[2].SelectionSource != "primary_relation_expansion" {
		t.Fatalf("top direct candidate starved across bindings: %#v", items)
	}

	covered := direct("binding-b")
	covered.CoverageStatus = "covered"
	secondDirect := direct("binding-b")
	secondDirect.SourceID = "direct-2"
	secondDirect.Text = "Second direct fact."
	items, _ = buildReferenceCoverageInjectionItems(
		[]store.SessionReferenceBinding{first.binding, second.binding},
		map[string]referenceRecallScope{"binding-a": first, "binding-b": second},
		[]referenceRecallItem{direct("binding-a"), covered, secondDirect},
		companions,
		newReferenceCoverageFieldIndexSummary(),
		2,
	)
	if len(items) != 2 || items[0].BindingID != "binding-a" || items[1].BindingID != "binding-b" || items[1].SourceID != "direct-2" {
		t.Fatalf("first injectable direct candidate starved across bindings: %#v", items)
	}
}
