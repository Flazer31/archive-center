package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrimaryReferenceFoundationStillInjectsWhenVectorRecallIsUnavailable(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.entities = []store.ReferenceEntity{{EntityID: "group", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "faction", CanonicalName: "Aster", DescriptionText: "A three-member team.", ReviewStatus: "approved", MetadataJSON: `{"canon_role":"core_faction","canon_importance":"core"}`}}
	fake.claims = []store.ReferenceClaim{{ClaimID: "members", WorkID: "work-1", ContinuityID: "continuity-1", ClaimType: "relationship", SubjectEntityID: "group", ClaimText: "Aster consists of Rin, Mira, and Zoe.", TemporalScope: "timeless", KnowledgeScope: "public_world", ReviewStatus: "approved", MetadataJSON: `{"canon_role":"core_relationship","canon_importance":"core"}`}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: true, InjectionEnabled: true, ReferenceMode: referenceModePrimary}}

	srv := &Server{Store: fake}
	result := srv.buildSessionReferenceRecallWithSceneContext(context.Background(), "session-1", "Who are they?", 6, nil, nil, referenceCoverageSceneContext{})
	if result.Status != "ready" || len(result.FoundationItems) == 0 || len(result.InjectionItems) == 0 {
		t.Fatalf("foundation did not survive unavailable vector recall: %#v", result)
	}
	if !containsAll(formatReferenceRecallInjection(result, 1200), "[Canon Foundation]", "Aster consists of Rin, Mira, and Zoe.") {
		t.Fatalf("foundation injection missing: %#v", result.InjectionItems)
	}
}

func TestPrimaryReferenceFoundationSelectsCoreRelationsAndRulesWithoutSyntheticVectorScores(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{
			BindingID:     "binding-1",
			WorkID:        "work-1",
			ContinuityID:  "continuity-1",
			ReferenceMode: referenceModePrimary,
		},
		work: &store.ReferenceWork{WorkID: "work-1", Title: "Example"},
		entities: map[string]store.ReferenceEntity{
			"group": {EntityID: "group", EntityType: "faction", CanonicalName: "Aster", DescriptionText: "A three-member team.", MetadataJSON: `{"canon_role":"core_faction","canon_importance":"core"}`},
			"lead":  {EntityID: "lead", EntityType: "character", CanonicalName: "Rin", DescriptionText: "The team leader.", MetadataJSON: `{"canon_role":"main_cast","canon_importance":"core"}`},
		},
		claims: map[string]store.ReferenceClaim{
			"members": {ClaimID: "members", ClaimType: "relationship", SubjectEntityID: "group", ClaimText: "consists of Rin, Mira, and Zoe.", TemporalScope: "timeless", KnowledgeScope: "public_world", Confidence: 0.99, MetadataJSON: `{"canon_role":"core_relationship","canon_importance":"core"}`},
			"rule":    {ClaimID: "rule", ClaimType: "world_rule", SubjectEntityID: "group", ClaimText: "Aster protects the barrier through song.", TemporalScope: "timeless", KnowledgeScope: "public_world", Confidence: 0.95, MetadataJSON: `{"canon_role":"core_rule","canon_importance":"core"}`},
		},
		nodes:   map[string]store.ReferenceTimelineNode{},
		aliases: map[string][]string{},
	}

	items := buildPrimaryReferenceFoundationItems(map[string]referenceRecallScope{"binding-1": scope}, referenceCoverageSceneContext{RecentCompletedTurn: 0}, 6)
	if len(items) < 2 {
		t.Fatalf("foundation items = %#v", items)
	}
	joined := ""
	for _, item := range items {
		joined += "\n" + item.Text
		if item.SelectionSource != "primary_canon_foundation" || item.ChromaRank != nil || item.Distance != nil || item.CosineSimilarity != nil {
			t.Fatalf("foundation item received synthetic vector provenance: %#v", item)
		}
	}
	if !containsAll(joined, "Aster: consists of Rin, Mira, and Zoe.", "Aster protects the barrier through song.") {
		t.Fatalf("core canon missing from foundation: %q", joined)
	}
}

func TestPrimaryReferenceFoundationStopsAfterOpeningTurns(t *testing.T) {
	scope := referenceRecallScope{
		binding:  store.SessionReferenceBinding{BindingID: "binding-1", ReferenceMode: referenceModePrimary},
		entities: map[string]store.ReferenceEntity{"group": {EntityID: "group", EntityType: "faction", CanonicalName: "Aster", DescriptionText: "A team."}},
		claims:   map[string]store.ReferenceClaim{},
		nodes:    map[string]store.ReferenceTimelineNode{},
	}
	if items := buildPrimaryReferenceFoundationItems(map[string]referenceRecallScope{"binding-1": scope}, referenceCoverageSceneContext{RecentCompletedTurn: 2}, 6); len(items) != 0 {
		t.Fatalf("opening foundation remained active after turn 1: %#v", items)
	}
}

func TestReferenceInjectionSeparatesFoundationFromSceneRecall(t *testing.T) {
	foundation := referenceInjectionItem{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "foundation", WorkTitle: "Example", ReferenceMode: referenceModePrimary, Text: "Core canon.", SelectionSource: "primary_canon_foundation"}
	rank := 3
	distance := 0.27
	scene := referenceInjectionItem{BindingID: "binding-1", ReferenceKind: "claim", SourceID: "scene", WorkTitle: "Example", ReferenceMode: referenceModePrimary, Text: "Scene fact.", SelectionSource: "primary_chroma_candidate", ChromaRank: &rank, Distance: &distance}
	items := mergePrimaryReferenceFoundationItems([]referenceInjectionItem{foundation}, []referenceInjectionItem{foundation, scene}, 4)
	if len(items) != 2 || items[1].ChromaRank == nil || *items[1].ChromaRank != rank || items[1].Distance == nil || *items[1].Distance != distance {
		t.Fatalf("merged items changed scene recall provenance: %#v", items)
	}
	formatted := formatReferenceRecallInjection(referenceRecallResult{Status: "ready", InjectionItems: items}, 1200)
	if !containsAll(formatted, "[Canon Foundation]", "Core canon.", "[Scene Reference]", "Scene fact.") {
		t.Fatalf("reference sections = %q", formatted)
	}
	if strings.Index(formatted, "[Canon Foundation]") > strings.Index(formatted, "[Scene Reference]") {
		t.Fatalf("scene reference preceded canon foundation: %q", formatted)
	}
}

func TestReferenceExtractorCanonMetadataNormalization(t *testing.T) {
	if got := normalizeReferenceCanonRole(" MAIN_CAST "); got != "main_cast" {
		t.Fatalf("canon role = %q", got)
	}
	if got := normalizeReferenceCanonRole("invented_role"); got != "supporting" {
		t.Fatalf("unknown canon role = %q", got)
	}
	if got := normalizeReferenceCanonImportance("HIGH"); got != "high" {
		t.Fatalf("canon importance = %q", got)
	}
	if got := normalizeReferenceCanonImportance("urgent"); got != "normal" {
		t.Fatalf("unknown canon importance = %q", got)
	}
}
