package httpapi

import (
	"context"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestReferenceCoverageActiveContextKeepsUserRoleLorebookAndDropsConversation(t *testing.T) {
	scene := referenceCoverageSceneContext{Conversation: []referenceCoverageSceneSource{
		{Text: "Previous player line."},
		{Text: "Previous assistant line."},
	}}
	messages := []map[string]any{
		{"role": "system", "content": "Rumi leads HUNTR/X."},
		{"role": "user", "content": "Mira carries the Sain-geom."},
		{"role": "user", "content": "Previous player line."},
		{"role": "assistant", "content": "Previous assistant line."},
		{"role": "user", "content": "Continue."},
	}
	active := referenceCoverageActiveContextMessages("Continue.", messages, scene)
	if len(active) != 2 {
		t.Fatalf("active context = %#v, want system and user-role lorebook only", active)
	}
	if !containsAll(active[0].normalized+active[1].normalized, "rumi", "mira", "saingeom") {
		t.Fatalf("active context lost lorebook fields: %#v", active)
	}

	secondMessages := append([]map[string]any{}, messages[:4]...)
	secondMessages = append(secondMessages, map[string]any{"role": "user", "content": "Move on."})
	if got, want := referenceCoverageContextHash(referenceCoverageActiveContextMessages("Move on.", secondMessages, scene)), referenceCoverageContextHash(active); got != want {
		t.Fatalf("turn input changed stable context hash: got=%s want=%s", got, want)
	}
}

func TestReferenceCoverageInventoryUsesStructuredFieldsWithoutSyntheticScores(t *testing.T) {
	scope := referenceRecallScope{
		binding: store.SessionReferenceBinding{BindingID: "binding-1", WorkID: "work-1", ContinuityID: "continuity-1"},
		entities: map[string]store.ReferenceEntity{
			"entity-mira": {EntityID: "entity-mira", CanonicalName: "Mira", EntityType: "character", DescriptionText: "Leader of HUNTR/X", MetadataJSON: `{"weapon":"Sain-geom"}`},
		},
		aliases: map[string][]string{"entity-mira": {"미라"}},
		claims: map[string]store.ReferenceClaim{
			"claim-1": {ClaimID: "claim-1", ClaimType: "role", SubjectEntityID: "entity-mira", ClaimText: "Mira leads HUNTR/X.", TemporalScope: "timeless", KnowledgeScope: "public_world", BranchKey: "main", MetadataJSON: `{"source":"official profile"}`},
		},
		nodes: map[string]store.ReferenceTimelineNode{
			"node-1": {NodeID: "node-1", NodeKey: "first-hunt", Label: "The first hunt", Ordinal: 1, BranchKey: "main", MetadataJSON: `{"era":"early"}`},
		},
		branchKey: "main",
	}
	ordinal := int64(1)
	scope.currentOrdinal = &ordinal
	scope.revealOrdinal = &ordinal
	active := referenceCoverageMessages([]map[string]any{{"role": "system", "content": "미라 wields the Sain-geom."}})
	fields := referenceCoverageInventoryFields(scope, active)

	wantFields := map[string]bool{
		"entity:canonical_name":   false,
		"entity:alias":            false,
		"entity:metadata.weapon":  false,
		"claim:subject_entity_id": false,
		"claim:claim_type":        false,
		"claim:claim_text":        false,
		"claim:metadata.source":   false,
		"timeline:node_key":       false,
		"timeline:metadata.era":   false,
	}
	for _, field := range fields {
		key := field.ReferenceKind + ":" + field.FieldName
		if _, ok := wantFields[key]; ok {
			wantFields[key] = true
		}
		if field.FieldName == "alias" && field.FieldValue == "미라" && !field.PresentInContext {
			t.Fatalf("literal alias was not detected: %#v", field)
		}
		if field.FieldName == "canonical_name" && field.PresentInContext {
			t.Fatalf("alias must not mark the canonical_name field as literally present: %#v", field)
		}
		if field.FieldName == "metadata.weapon" && !field.PresentInContext {
			t.Fatalf("metadata leaf was not detected: %#v", field)
		}
	}
	for key, found := range wantFields {
		if !found {
			t.Fatalf("structured inventory field missing: %s; fields=%#v", key, fields)
		}
	}
	changed := append([]store.SessionReferenceCoverageField(nil), fields...)
	changed[0].FieldValue += "!"
	if referenceCoverageInventoryHash(changed) == referenceCoverageInventoryHash(fields) {
		t.Fatal("inventory hash ignored a raw field value change")
	}
}

func TestReferenceCoverageFieldIndexReusesSnapshotAndFindsNeededSourceOutsideChromaWindow(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "current", Label: "Current", Ordinal: 10, BranchKey: "main", ReviewStatus: "approved"}}
	fake.entities = []store.ReferenceEntity{
		{EntityID: "entity-rumi", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Rumi", EntityType: "character", DescriptionText: "A hunter.", ReviewStatus: "approved"},
		{EntityID: "entity-mira", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Mira", EntityType: "character", DescriptionText: "Leader of HUNTR/X.", ReviewStatus: "approved"},
	}
	fake.aliases["entity-mira"] = []store.ReferenceEntityAlias{{WorkID: "work-1", ContinuityID: "continuity-1", EntityID: "entity-mira", AliasText: "미라"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", RevealCeilingNodeID: "node-current"}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{{Document: referenceRecallVectorDocument("entity", "entity-rumi"), ChromaRank: 1, CosineSimilarity: 0.9, CosineAvailable: true}}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	scene := buildReferenceCoverageSceneContext([]store.ChatLog{
		{ID: 1, TurnIndex: 3, Role: "user", Content: "Mira enters the rehearsal room."},
		{ID: 2, TurnIndex: 3, Role: "assistant", Content: "Mira checks the team."},
	}, nil, nil, nil, 0)
	messages := []map[string]any{
		{"role": "system", "content": "Mira is present."},
		{"role": "user", "content": "Continue."},
	}

	first := srv.buildSessionReferenceRecallWithSceneContext(context.Background(), "session-1", "Continue.", 3, nil, messages, scene)
	if fake.coverageWrites != 1 {
		t.Fatalf("first snapshot writes=%d, want 1", fake.coverageWrites)
	}
	index := first.CoverageShadow.FieldIndex
	if index.ContractVersion != referenceCoverageFieldIndexContractVersion || index.SnapshotWrites != 1 || index.NeededSources == 0 {
		t.Fatalf("field index summary = %#v", index)
	}
	foundMira := false
	for _, item := range index.NeededSourceItems {
		if item.SourceID == "entity-mira" {
			foundMira = true
			if item.CoverageStatus != "partial" || !referenceCoverageTestContainsString(item.NeededBy, "recent_completed_dialogue") {
				t.Fatalf("Mira field coverage = %#v", item)
			}
		}
	}
	if !foundMira {
		t.Fatalf("needed source outside Chroma window was not indexed: %#v", index.NeededSourceItems)
	}
	if len(first.Selected) != 1 || first.Selected[0].SourceID != "entity-rumi" || first.Selected[0].ChromaRank != 1 {
		t.Fatalf("field index changed Chroma result: %#v", first.Selected)
	}
	if len(first.InjectionItems) != 1 || first.InjectionItems[0].SourceID != "entity-mira" || first.InjectionItems[0].SelectionSource != "coverage_field_index" || first.InjectionItems[0].ChromaRank != nil {
		t.Fatalf("needed source outside Chroma window was not applied without a synthetic rank: %#v", first.InjectionItems)
	}

	second := srv.buildSessionReferenceRecallWithSceneContext(context.Background(), "session-1", "Continue.", 3, nil, messages, scene)
	if fake.coverageWrites != 1 || second.CoverageShadow.FieldIndex.SnapshotReuses != 1 {
		t.Fatalf("identical context rewrote snapshot: writes=%d summary=%#v", fake.coverageWrites, second.CoverageShadow.FieldIndex)
	}

	changedMessages := []map[string]any{
		{"role": "system", "content": "Mira is present. Leader of HUNTR/X."},
		{"role": "user", "content": "Continue."},
	}
	third := srv.buildSessionReferenceRecallWithSceneContext(context.Background(), "session-1", "Continue.", 3, nil, changedMessages, scene)
	if fake.coverageWrites != 2 || third.CoverageShadow.FieldIndex.SnapshotWrites != 1 {
		t.Fatalf("changed lorebook did not replace snapshot: writes=%d summary=%#v", fake.coverageWrites, third.CoverageShadow.FieldIndex)
	}
}

func TestReferenceCoverageFieldIndexDoesNotRequireVectorOrEmbedding(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.entities = []store.ReferenceEntity{{EntityID: "entity-mira", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Mira", EntityType: "character", ReviewStatus: "approved"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1"}}
	srv := &Server{Store: fake}
	scene := referenceCoverageSceneContext{RecentDialogue: []referenceCoverageSceneSource{{Text: "Mira arrives."}}}
	result := srv.buildSessionReferenceRecallWithSceneContext(context.Background(), "session-1", "Continue.", 3, nil, []map[string]any{
		{"role": "system", "content": "Mira is a hunter."},
		{"role": "user", "content": "Continue."},
	}, scene)
	if result.CoverageShadow.FieldIndex.Status != "ready" || fake.coverageWrites != 1 {
		t.Fatalf("field index depended on vector runtime: %#v writes=%d", result.CoverageShadow.FieldIndex, fake.coverageWrites)
	}
	if !referenceCoverageTestContainsString(result.Warnings, "reference_vector_exact_query_unavailable") {
		t.Fatalf("expected vector warning without losing field index: %#v", result.Warnings)
	}
}

func referenceCoverageTestContainsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
