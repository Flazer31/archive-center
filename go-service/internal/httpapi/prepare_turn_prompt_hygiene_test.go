package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnDropsLegacySecretDuplicateOwnedByProtectedLane(t *testing.T) {
	sourceMemories := []store.Memory{{
		TurnIndex:   6,
		SummaryJSON: `{"protected_secrets":[{"owner":"Mira","summary":"Mira hides the brass key."}]}`,
	}}
	private := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerEntityRole: "npc", SecretGuard: true, SourceTurn: 6, MemoryText: "Mira hides the brass key."},
		{ID: 2, OwnerEntityKey: "juno", OwnerEntityName: "Juno", OwnerEntityRole: "npc", SourceTurn: 7, MemoryText: "Juno remembers the forge promise."},
	}
	trace := filterPrepareTurnEntityRecollections(
		"Mira and Juno meet at the forge.",
		sourceMemories, nil, nil, nil, nil, &private,
	)
	if len(private) != 1 || private[0].OwnerEntityKey != "juno" {
		t.Fatalf("protected-lane duplicate survived private lane: %#v trace=%#v", private, trace)
	}
}

func TestPrepareTurnProtectedLaneDoesNotDropDifferentPrivateMemoryFromSameTurn(t *testing.T) {
	identity := map[string]any{
		"canonical_entity_name": "Juno",
		"surface_identity_name": "The Courier",
		"same_entity":           true,
	}
	identityJSON := compactPrepareTurnJSON(identity)
	index := prepareTurnProtectedPrivateGuardIndex([]store.Memory{
		{
			TurnIndex:   6,
			SummaryJSON: `{"protected_secrets":[{"owner":"Mira","summary":"Mira hides the brass key."}]}`,
		},
		{
			TurnIndex:   7,
			SummaryJSON: `{"character_identity_accuracy":[` + identityJSON + `]}`,
		},
	})
	exactDuplicate := store.ProtagonistEntityMemory{
		OwnerEntityName: "Mira", SourceTurn: 6, MemoryText: "Mira hides the brass key.",
	}
	differentMemory := store.ProtagonistEntityMemory{
		OwnerEntityName: "Mira", SourceTurn: 6, MemoryText: "Mira privately remembers the forge promise.",
	}
	if !prepareTurnProtectedMemoryOwnsPrivateGuard(index, exactDuplicate) {
		t.Fatal("exact protected-lane duplicate was not recognized")
	}
	if prepareTurnProtectedMemoryOwnsPrivateGuard(index, differentMemory) {
		t.Fatal("different private memory from the same owner and turn was over-filtered")
	}
	identityDuplicate := store.ProtagonistEntityMemory{
		OwnerEntityName: "Juno", SourceTurn: 7, MemoryText: protectedIdentityGuardSummary(identity),
	}
	if !prepareTurnProtectedMemoryOwnsPrivateGuard(index, identityDuplicate) {
		t.Fatal("exact identity-guard duplicate was not recognized")
	}
}

func TestPrepareTurnCanonicalCharacterRosterDoesNotConsumeStateBudget(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil,
		[]store.CanonicalStateLayer{
			{ID: 1, LayerType: "entity_state", Content: `{"characters":["Mira","Juno"],"background":{"location":"forge"}}`, Confidence: 0.9},
			{ID: 2, LayerType: "entity_state", Content: `{"characters":[{"name":"Mira","emotion":"tense","location":"forge"}]}`, Confidence: 0.9},
		},
		nil, nil, nil, nil,
		5, 9000, "Mira waits tensely at the forge.", "default", nil, nil, nil,
	)
	if strings.Contains(assembly.CanonCharacterText, `["Mira","Juno"]`) {
		t.Fatalf("roster-only character array consumed state lane: %q", assembly.CanonCharacterText)
	}
	if !strings.Contains(assembly.CanonCharacterText, `"emotion":"tense"`) {
		t.Fatalf("detailed character state was lost: %q", assembly.CanonCharacterText)
	}
	if got := intFromAny(assembly.Counts["canonical_character_roster_only_dropped"], 0); got != 1 {
		t.Fatalf("roster drop count=%d, want 1: %#v", got, assembly.Counts)
	}
}

func TestPrepareTurnVectorEvidenceStillRequiresCurrentSceneRelevance(t *testing.T) {
	evidence := []store.DirectEvidence{
		{ID: 101, EvidenceText: "Mira aligned the brass wheel in the forge.", TurnAnchor: 12},
		{ID: 102, EvidenceText: "Juno mailed a passport from the harbor.", TurnAnchor: 4},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"id": "evidence:sess:101", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "101", "similarity": 0.87, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "evidence:sess:102", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "102", "similarity": 0.84, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, evidence, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Mira tests the brass wheel at the forge.", "default", nil, vectorShadow, nil,
	)
	if !strings.Contains(assembly.DirectEvidenceText, "brass wheel") {
		t.Fatalf("current vector evidence was lost: %q", assembly.DirectEvidenceText)
	}
	if strings.Contains(assembly.DirectEvidenceText, "passport") || strings.Contains(assembly.DirectEvidenceText, "harbor") {
		t.Fatalf("unrelated vector evidence survived: %q", assembly.DirectEvidenceText)
	}
	if got := intFromAny(assembly.Counts["direct_evidence_irrelevant_dropped"], 0); got != 1 {
		t.Fatalf("vector evidence drop count=%d, want 1", got)
	}
}

func TestPrepareTurnMemoryNameOnlyAnchorDoesNotFillEventLane(t *testing.T) {
	item := store.Memory{
		TurnIndex:   3,
		SummaryJSON: `{"turn_summary":"Mira discussed an old passport at the harbor.","entities":{"characters":[{"name":"Mira"}]}}`,
	}
	evidence := prepareTurnMemoryRecallEvidence("Mira calibrates the brass wheel at the forge.", item)
	if evidence.Eligible {
		t.Fatalf("character-name-only overlap filled event lane: %#v", evidence)
	}
}

func TestPrepareTurnLongSceneNeedsThreeTermsForNonVectorRefill(t *testing.T) {
	query := "Mira calibrates the brass wheel at the forge while the workshop crew prepares the demonstration and checks every bearing axle frame pedal weight balance surface tool material schedule guest entrance platform guard lamp document signal seat table door window floor ceiling"
	twoTerms := store.Memory{
		TurnIndex:   3,
		SummaryJSON: `{"turn_summary":"Mira discussed an old brass passport at the harbor."}`,
	}
	if got := prepareTurnMemoryRecallEvidence(query, twoTerms); got.Eligible {
		t.Fatalf("two incidental overlaps filled a long-scene event lane: %#v", got)
	}
	threeTerms := store.Memory{
		TurnIndex:   4,
		SummaryJSON: `{"turn_summary":"Mira calibrated the brass wheel before the demonstration."}`,
	}
	if got := prepareTurnMemoryRecallEvidence(query, threeTerms); !got.Eligible {
		t.Fatalf("three scene overlaps did not preserve relevant memory: %#v", got)
	}
}

func TestPrepareTurnPendingThreadNeedsDescriptionOverlapNotOwnerNameOnly(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil,
		[]store.PendingThread{{
			ThreadKey:   "old-promise",
			Owner:       "Mira",
			Status:      "open",
			Description: "Mira promised to reveal Juno's identity at the harbor.",
		}},
		nil, nil, nil, nil, nil,
		5, 9000, "Mira calibrates the brass wheel at the forge.", "default", nil, nil, nil,
	)
	if strings.TrimSpace(assembly.PendingThreadText) != "" {
		t.Fatalf("owner-name-only pending thread survived: %q", assembly.PendingThreadText)
	}
}

func TestPrepareTurnRelationshipSurfacesDoNotLeakOffSceneMarriageBundle(t *testing.T) {
	const rawInput = "한얼은 월하방에서 세종의 판단을 듣는다."
	perspective := prepareTurnPerspectiveWithNarrativeState(
		map[string]any{},
		nil,
		[]store.ActiveState{{
			StateType: "scene",
			TurnIndex: 51,
			Content:   `{"location":"월하방","present_entities":["강한얼","세종"],"status":"감시 중"}`,
		}},
	)
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil,
		[]store.WorldRule{{ID: 1, Key: "월하방 감시", ValueJSON: `{"rule":"월하방의 감시는 강한얼 주변에서 계속된다."}`}},
		[]store.CharacterState{
			{
				CharacterName: "강한얼",
				TurnIndex:     51,
				RelationshipsJSON: `{
				"민서현":{"summary":"오래된 혼사 제안이 아직 남아 있다"},
				"세종":{"summary":"세종이 현재 강한얼의 시연을 판단한다"},
				"legacy_bundle":"강한얼은 세종의 판단을 생각하면서 민서현의 오래된 혼사 제안도 떠올린다"
			}`,
			},
			{CharacterName: "민서현", TurnIndex: 40},
			{CharacterName: "세종", TurnIndex: 51},
		},
		nil,
		[]store.CanonicalStateLayer{
			{ID: 10, LayerType: "relationship_state", Content: `{"pair":["강한얼","민서현"],"target_name":"민서현","bond_and_distance":"오래된 혼사 제안이 아직 남아 있다"}`, TurnIndex: 40, Confidence: 0.9},
			{ID: 11, LayerType: "relationship_state", Content: `{"pair":["강한얼","세종"],"target_name":"세종","bond_and_distance":"세종이 현재 강한얼의 시연을 판단한다"}`, TurnIndex: 51, Confidence: 0.9},
			{ID: 12, LayerType: "relationship_state", Content: `강한얼은 세종의 판단을 생각하면서 민서현의 오래된 혼사 제안도 떠올린다.`, TurnIndex: 41, Confidence: 0.9},
		},
		nil, nil, nil, nil,
		5, 9000, rawInput, "default", nil, nil, nil, perspective,
	)

	relationshipText := assembly.CharacterRelationshipText + "\n" + assembly.CanonRelationshipText
	if strings.Contains(relationshipText, "민서현") || strings.Contains(relationshipText, "혼사") {
		t.Fatalf("off-scene marriage relationship leaked through a related bundle: %q", relationshipText)
	}
	if !strings.Contains(relationshipText, "세종") || !strings.Contains(relationshipText, "판단") {
		t.Fatalf("current-scene judgment relationship was lost: %q", relationshipText)
	}
	if !strings.Contains(assembly.WorldRulesText, "월하방 감시") {
		t.Fatalf("current-scene surveillance support was lost: %q", assembly.WorldRulesText)
	}
}
