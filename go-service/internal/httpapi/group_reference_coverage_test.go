package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestReferenceCoverageShadowClassifiesExactPartialMissingAndUnrelatedWithoutFiltering(t *testing.T) {
	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"rumi": {EntityID: "rumi", CanonicalName: "Rumi", EntityType: "character", DescriptionText: "Huntrix's leader and lead singer."},
		},
		claims: map[string]store.ReferenceClaim{
			"claim-role": {ClaimID: "claim-role", SubjectEntityID: "rumi", ClaimType: "role", ClaimText: "Rumi leads Huntrix."},
		},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}

	entityItem := referenceRecallItem{ReferenceKind: "entity", SourceID: "rumi", Text: "Rumi: Huntrix's leader and lead singer.", Eligible: true, Reason: "eligible", Metadata: map[string]any{"aliases": []string{"루미"}}}
	exact := applyReferenceCoverageShadow(entityItem, scope, "루미가 어떻게 대답해?", []map[string]any{
		{"role": "system", "content": "Rumi: Huntrix's leader and lead singer."},
		{"role": "user", "content": "루미가 어떻게 대답해?"},
	})
	if !exact.Needed || exact.CoverageStatus != "covered" || exact.DecisionReason != "exact_reference_text_present" {
		t.Fatalf("exact coverage = %#v", exact)
	}

	partial := applyReferenceCoverageShadow(entityItem, scope, "루미가 어떻게 대답해?", []map[string]any{
		{"role": "system", "content": "이번 장면에는 루미가 등장한다."},
		{"role": "user", "content": "루미가 어떻게 대답해?"},
	})
	if !partial.Needed || partial.CoverageStatus != "partial" || len(partial.MissingFields) != 1 || partial.MissingFields[0] != "description" {
		t.Fatalf("partial coverage = %#v", partial)
	}

	scope.sceneEntities["rumi"] = true
	claimItem := referenceRecallItem{ReferenceKind: "claim", SourceID: "claim-role", Text: "Rumi leads Huntrix.", Eligible: true, Reason: "eligible"}
	missing := applyReferenceCoverageShadow(claimItem, scope, "Continue the current scene.", []map[string]any{
		{"role": "user", "content": "Continue the current scene."},
	})
	if !missing.Needed || missing.CoverageStatus != "missing" || len(missing.MissingFields) != 1 || missing.MissingFields[0] != "role" {
		t.Fatalf("missing coverage = %#v", missing)
	}

	delete(scope.sceneEntities, "rumi")
	unrelated := applyReferenceCoverageShadow(entityItem, scope, "문을 열고 복도로 나간다.", []map[string]any{
		{"role": "user", "content": "문을 열고 복도로 나간다."},
	})
	if unrelated.Needed || unrelated.CoverageStatus != "not_applicable" || unrelated.DecisionReason != "no_current_scene_need_signal" {
		t.Fatalf("unrelated coverage = %#v", unrelated)
	}
}

func TestReferenceCoverageShadowKeepsUnknownAndHardFilterReasonsExplicit(t *testing.T) {
	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"rumi": {EntityID: "rumi", CanonicalName: "Rumi", EntityType: "character"},
		},
		claims:        map[string]store.ReferenceClaim{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}

	unknown := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "rumi",
		Text:          "Rumi",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Rumi", nil)
	if !unknown.Needed || unknown.CoverageStatus != "unknown" || unknown.DecisionReason != "active_request_messages_unavailable" {
		t.Fatalf("unknown coverage = %#v", unknown)
	}

	future := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "timeline",
		SourceID:      "future",
		Text:          "Future reveal",
		Eligible:      false,
		Reason:        "future_timeline_node",
	}, scope, "Future reveal", []map[string]any{{"role": "user", "content": "Future reveal"}})
	if future.Needed || future.CoverageStatus != "not_applicable" || future.DecisionReason != "future_timeline_node" {
		t.Fatalf("future coverage = %#v", future)
	}

	summary := summarizeReferenceCoverage([]referenceRecallItem{unknown}, []referenceRecallItem{future})
	if summary.Mode != "shadow" || summary.InjectionFiltered || summary.EvaluatedCount != 2 || summary.StatusCounts["unknown"] != 1 || summary.StatusCounts["not_applicable"] != 1 {
		t.Fatalf("coverage summary = %#v", summary)
	}
}

func TestReferenceCoverageShadowIgnoresItsOwnPriorInjectionBlock(t *testing.T) {
	scope := referenceRecallScope{
		entities:      map[string]store.ReferenceEntity{"rumi": {EntityID: "rumi", CanonicalName: "Rumi"}},
		claims:        map[string]store.ReferenceClaim{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{"rumi": true},
	}
	item := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "rumi",
		Text:          "Rumi: Huntrix's leader.",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Continue.", []map[string]any{
		{"role": "system", "content": "[Original Work Reference]\n- Rumi: Huntrix's leader."},
		{"role": "user", "content": "Continue."},
	})
	if item.CoverageStatus != "missing" {
		t.Fatalf("prior Archive injection counted as active external coverage: %#v", item)
	}
}
