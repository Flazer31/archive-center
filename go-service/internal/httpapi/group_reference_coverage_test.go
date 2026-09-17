package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestReferenceCoverageShadowClassifiesExactPartialMissingAndUnrelatedWithoutFiltering(t *testing.T) {
	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"arin": {EntityID: "arin", CanonicalName: "Arin", EntityType: "character", DescriptionText: "Aster Unit's leader and lead singer."},
		},
		claims: map[string]store.ReferenceClaim{
			"claim-role": {ClaimID: "claim-role", SubjectEntityID: "arin", ClaimType: "role", ClaimText: "Arin leads Aster Unit."},
		},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}

	entityItem := referenceRecallItem{ReferenceKind: "entity", SourceID: "arin", Text: "Arin: Aster Unit's leader and lead singer.", Eligible: true, Reason: "eligible", Metadata: map[string]any{"aliases": []string{"아린"}}}
	exact := applyReferenceCoverageShadow(entityItem, scope, "아린가 어떻게 대답해?", []map[string]any{
		{"role": "system", "content": "Arin: Aster Unit's leader and lead singer."},
		{"role": "user", "content": "아린가 어떻게 대답해?"},
	}, referenceCoverageSceneContext{})
	if !exact.Needed || exact.CoverageStatus != "covered" || exact.DecisionReason != "exact_reference_text_present" {
		t.Fatalf("exact coverage = %#v", exact)
	}

	partial := applyReferenceCoverageShadow(entityItem, scope, "아린가 어떻게 대답해?", []map[string]any{
		{"role": "system", "content": "이번 장면에는 아린가 등장한다."},
		{"role": "user", "content": "아린가 어떻게 대답해?"},
	}, referenceCoverageSceneContext{})
	if !partial.Needed || partial.CoverageStatus != "partial" || len(partial.MissingFields) != 1 || partial.MissingFields[0] != "description" {
		t.Fatalf("partial coverage = %#v", partial)
	}

	scope.sceneEntities["arin"] = true
	claimItem := referenceRecallItem{ReferenceKind: "claim", SourceID: "claim-role", Text: "Arin leads Aster Unit.", Eligible: true, Reason: "eligible"}
	missing := applyReferenceCoverageShadow(claimItem, scope, "Continue the current scene.", []map[string]any{
		{"role": "user", "content": "Continue the current scene."},
	}, referenceCoverageSceneContext{})
	if !missing.Needed || missing.CoverageStatus != "missing" || len(missing.MissingFields) != 1 || missing.MissingFields[0] != "role" {
		t.Fatalf("missing coverage = %#v", missing)
	}

	delete(scope.sceneEntities, "arin")
	unrelated := applyReferenceCoverageShadow(entityItem, scope, "문을 열고 복도로 나간다.", []map[string]any{
		{"role": "user", "content": "문을 열고 복도로 나간다."},
	}, referenceCoverageSceneContext{})
	if unrelated.Needed || unrelated.CoverageStatus != "not_applicable" || unrelated.DecisionReason != "no_current_scene_need_signal" {
		t.Fatalf("unrelated coverage = %#v", unrelated)
	}
}

func TestReferenceCoveragePrimaryModeChecksExistingLoreBeforeAddingChromaContext(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{ReferenceMode: referenceModePrimary},
		claims: map[string]store.ReferenceClaim{
			"claim-role": {ClaimID: "claim-role", ClaimType: "role", ClaimText: "Arin leads Aster Unit."},
		},
		entities:      map[string]store.ReferenceEntity{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}
	item := referenceRecallItem{ReferenceKind: "claim", SourceID: "claim-role", Text: "Arin leads Aster Unit.", Eligible: true, Reason: "eligible"}

	covered := applyReferenceCoverageShadow(item, scope, "Continue.", []map[string]any{
		{"role": "system", "content": "Arin leads Aster Unit."},
		{"role": "user", "content": "Continue."},
	}, referenceCoverageSceneContext{})
	if !covered.Needed || covered.CoverageStatus != "covered" || len(covered.NeededBy) != 1 || covered.NeededBy[0] != "primary_chroma_relevance" {
		t.Fatalf("primary mode did not suppress already supplied lore: %#v", covered)
	}

	missing := applyReferenceCoverageShadow(item, scope, "Continue.", []map[string]any{
		{"role": "system", "content": "The scene begins backstage."},
		{"role": "user", "content": "Continue."},
	}, referenceCoverageSceneContext{})
	if !missing.Needed || missing.CoverageStatus != "missing" || len(missing.NeededBy) != 1 || missing.NeededBy[0] != "primary_chroma_relevance" {
		t.Fatalf("primary mode did not expose missing canon context: %#v", missing)
	}
}

func TestReferenceCoverageShadowKeepsUnknownAndHardFilterReasonsExplicit(t *testing.T) {
	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"arin": {EntityID: "arin", CanonicalName: "Arin", EntityType: "character"},
		},
		claims:        map[string]store.ReferenceClaim{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}

	unknown := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "arin",
		Text:          "Arin",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Arin", nil, referenceCoverageSceneContext{})
	if !unknown.Needed || unknown.CoverageStatus != "unknown" || unknown.DecisionReason != "coverage_sources_unavailable" {
		t.Fatalf("unknown coverage = %#v", unknown)
	}

	future := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "timeline",
		SourceID:      "future",
		Text:          "Future reveal",
		Eligible:      false,
		Reason:        "future_timeline_node",
	}, scope, "Future reveal", []map[string]any{{"role": "user", "content": "Future reveal"}}, referenceCoverageSceneContext{})
	if future.Needed || future.CoverageStatus != "not_applicable" || future.DecisionReason != "future_timeline_node" {
		t.Fatalf("future coverage = %#v", future)
	}

	summary := summarizeReferenceCoverage([]referenceRecallItem{unknown}, []referenceRecallItem{future}, referenceCoverageSceneContext{}, newReferenceCoverageFieldIndexSummary())
	if summary.Mode != "shadow" || summary.InjectionFiltered || summary.EvaluatedCount != 2 || summary.StatusCounts["unknown"] != 1 || summary.StatusCounts["not_applicable"] != 1 {
		t.Fatalf("coverage summary = %#v", summary)
	}
}

func TestReferenceCoverageShadowIgnoresItsOwnPriorInjectionBlock(t *testing.T) {
	scope := referenceRecallScope{
		entities:      map[string]store.ReferenceEntity{"arin": {EntityID: "arin", CanonicalName: "Arin"}},
		claims:        map[string]store.ReferenceClaim{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{"arin": true},
	}
	item := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "arin",
		Text:          "Arin: Aster Unit's leader.",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Continue.", []map[string]any{
		{"role": "system", "content": "[Original Work Reference]\n- Arin: Aster Unit's leader."},
		{"role": "user", "content": "Continue."},
	}, referenceCoverageSceneContext{})
	if item.CoverageStatus != "missing" {
		t.Fatalf("prior Archive injection counted as active external coverage: %#v", item)
	}
}

func TestReferenceCoverageSceneContextUsesLatestCompletedTurnAndLatestLocation(t *testing.T) {
	context := buildReferenceCoverageSceneContext(
		[]store.ChatLog{
			{ID: 1, TurnIndex: 4, Role: "user", Content: "Arin enters the rehearsal room."},
			{ID: 2, TurnIndex: 4, Role: "assistant", Content: "Arin checks the stage marks."},
			{ID: 3, TurnIndex: 5, Role: "user", Content: "Continue."},
		},
		[]store.ActiveState{
			{ID: 10, StateType: "location", Content: "Old studio", TurnIndex: 2},
			{ID: 11, StateType: "scene", Content: `{"location":"Neon Bathhouse","mood":"quiet"}`, TurnIndex: 5},
		},
		[]store.CanonicalStateLayer{
			{ID: 20, LayerType: "scene_state", Content: `{"location":"Old studio"}`, LastVerifiedTurn: 3},
		},
		nil,
		8,
	)

	if context.RecentCompletedTurn != 4 || len(context.RecentDialogue) != 2 {
		t.Fatalf("recent completed dialogue = %#v", context)
	}
	if len(context.CurrentLocations) != 1 || context.CurrentLocations[0].Text != "Neon Bathhouse" {
		t.Fatalf("current locations = %#v", context.CurrentLocations)
	}

	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"arin": {EntityID: "arin", CanonicalName: "Arin", EntityType: "character"},
			"bath": {EntityID: "bath", CanonicalName: "Neon Bathhouse", EntityType: "location"},
		},
		claims:        map[string]store.ReferenceClaim{},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}

	arin := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "arin",
		Text:          "Arin: Aster Unit's leader.",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Continue.", []map[string]any{{"role": "user", "content": "Continue."}}, context)
	if !arin.Needed || !stringSliceContains(arin.NeededBy, "recent_completed_dialogue") || arin.CoverageStatus != "partial" {
		t.Fatalf("recent dialogue coverage = %#v", arin)
	}
	if len(arin.MatchedContextLocations) == 0 || len(arin.MatchedRequestLocations) != 0 {
		t.Fatalf("recent dialogue locations = %#v", arin)
	}

	bath := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "entity",
		SourceID:      "bath",
		Text:          "Neon Bathhouse: a hidden bathhouse beneath the city.",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Continue.", []map[string]any{{"role": "user", "content": "Continue."}}, context)
	if !bath.Needed || !stringSliceContains(bath.NeededBy, "current_location") || bath.CoverageStatus != "partial" {
		t.Fatalf("current location coverage = %#v", bath)
	}
}

func TestReferenceCoverageSceneContextUsesOnlyUnsuppressedActiveRules(t *testing.T) {
	context := buildReferenceCoverageSceneContext(
		nil,
		nil,
		nil,
		[]store.WorldRule{
			{ID: 30, Key: "demon_gate", ValueJSON: `{"rule":"Only marked gates open at night."}`},
			{ID: 31, Key: "suppressed_rule", ValueJSON: `{"rule":"Arin leads Aster Unit."}`, Suppressed: true},
		},
		8,
	)
	if len(context.ActiveRules) != 1 {
		t.Fatalf("active rules = %#v", context.ActiveRules)
	}

	scope := referenceRecallScope{
		entities: map[string]store.ReferenceEntity{
			"gate": {EntityID: "gate", CanonicalName: "marked gates", EntityType: "place"},
		},
		claims: map[string]store.ReferenceClaim{
			"gate-rule": {ClaimID: "gate-rule", SubjectEntityID: "gate", ClaimType: "access_rule", ClaimText: "Only marked gates open at night."},
		},
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}
	item := applyReferenceCoverageShadow(referenceRecallItem{
		ReferenceKind: "claim",
		SourceID:      "gate-rule",
		Text:          "Only marked gates open at night.",
		Eligible:      true,
		Reason:        "eligible",
	}, scope, "Continue.", []map[string]any{{"role": "user", "content": "Continue."}}, context)
	if !item.Needed || !stringSliceContains(item.NeededBy, "active_world_rule") || item.CoverageStatus != "covered" {
		t.Fatalf("active rule coverage = %#v", item)
	}
	if len(item.MatchedContextLocations) != 1 || len(item.MatchedRequestLocations) != 0 {
		t.Fatalf("active rule locations = %#v", item)
	}
}
