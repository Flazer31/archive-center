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
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", Content: `{"location":"forge","present_entities":["Mira"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil,
		[]store.CanonicalStateLayer{
			{ID: 1, LayerType: "entity_state", Content: `{"characters":["Mira","Juno"],"background":{"location":"forge"}}`, Confidence: 0.9},
			{ID: 2, LayerType: "entity_state", Content: `{"characters":[{"name":"Mira","emotion":"tense","location":"forge"}]}`, Confidence: 0.9},
		},
		nil, nil, nil, nil,
		5, 9000, "Mira waits tensely at the forge.", "default", nil, nil, nil, perspective,
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

func TestPrepareTurnSelectedEventEvidenceOutranksGenericAndSameTurnUnrelatedEvidence(t *testing.T) {
	const rawInput = "이소월은 강한얼과 맺은 상업 약조서를 바라보며 자신과 다른 그의 모습에 자신이 어떤 협력자가 될지 생각했다."
	memories := []store.Memory{{
		ID:        1,
		TurnIndex: 49,
		SummaryJSON: `{
			"turn_summary":"강한얼과 이소월은 상업 약조서를 체결한 뒤 천체 망원경도 점검했다.",
			"characters":["강한얼","이소월"],
			"items":["상업 약조서","천체 망원경"],
			"narrative_events":[
				{
					"summary":"토지 매입 및 수수료 조항의 상업 약조서를 체결했다.",
					"participants":["강한얼","이소월"],
					"evidence_excerpt":"강한얼과 이소월은 토지 매입과 사업 수수료를 정한 상업 약조서에 서명했다."
				},
				{
					"summary":"푸른 렌즈를 단 천체 망원경을 함께 점검했다.",
					"participants":["강한얼","이소월"],
					"evidence_excerpt":"강한얼과 이소월은 푸른 렌즈를 단 천체 망원경을 함께 점검했다."
				}
			]
		}`,
	}}
	evidence := []store.DirectEvidence{
		{ID: 101, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 토지 매입과 사업 수수료를 정한 상업 약조서에 서명했다."},
		{ID: 102, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 푸른 렌즈를 단 천체 망원경을 함께 점검했다."},
		{ID: 103, TurnAnchor: 60, EvidenceText: "민서현은 자신과 다른 그의 모습에 자신이 점점 더 빠져 들어간다는 것을 알았다."},
		{ID: 104, TurnAnchor: 25, EvidenceText: "그 계약의 토지 수수료 조항은 두 사람의 협력 관계를 지속시켰다."},
		{ID: 105, TurnAnchor: 49, EvidenceText: "강한얼과 이소월의 약조는 삭제된 근거다.", Tombstoned: true},
		{ID: 106, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 상업 약조서 뒤에 비밀 수수료 조항을 숨겨 두었다."},
	}
	private := []store.ProtagonistEntityMemory{{
		ID:                 901,
		OwnerEntityKey:     "sowol",
		OwnerEntityName:    "이소월",
		OwnerEntityRole:    "npc",
		OwnerVisibility:    "owner_private",
		SourceTurn:         49,
		MemoryText:         "이소월만 아는 비밀 수수료 조항",
		EvidenceExcerpt:    "강한얼과 이소월은 상업 약조서 뒤에 비밀 수수료 조항을 숨겨 두었다.",
		SecretGuard:        true,
		TargetRevealPolicy: "owner_private_until_revealed",
	}}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"source_table": "memories", "source_row_id": "1", "similarity": 0.93},
			{"source_table": "direct_evidence_records", "source_row_id": "101", "similarity": 0.91},
			{"source_table": "direct_evidence_records", "source_row_id": "104", "similarity": 0.83},
			{"source_table": "direct_evidence_records", "source_row_id": "105", "similarity": 0.99},
		},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories,
		nil,
		evidence,
		nil,
		nil,
		nil,
		[]store.CharacterState{{CharacterName: "강한얼"}, {CharacterName: "이소월"}, {CharacterName: "민서현"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		private,
		2,
		9000,
		rawInput,
		"default",
		nil,
		vectorShadow,
		nil,
	)
	const selected = "토지 매입과 사업 수수료를 정한 상업 약조서"
	if !strings.Contains(assembly.LatestDirectEvidenceText, selected) {
		t.Fatalf("selected event evidence did not own latest evidence: %q", assembly.LatestDirectEvidenceText)
	}
	for _, unwanted := range []string{"천체 망원경", "민서현은 자신과 다른", "삭제된 근거", "비밀 수수료"} {
		if strings.Contains(assembly.LatestDirectEvidenceText, unwanted) ||
			strings.Contains(assembly.DirectEvidenceText, unwanted) ||
			strings.Contains(assembly.ScopedVerbatimText, unwanted) {
			t.Fatalf("unrelated or private/tombstoned evidence %q survived: latest=%q direct=%q scoped=%q",
				unwanted, assembly.LatestDirectEvidenceText, assembly.DirectEvidenceText, assembly.ScopedVerbatimText)
		}
	}
	if !strings.Contains(assembly.DirectEvidenceText, "그 계약의 토지 수수료 조항") {
		t.Fatalf("unlinked semantic vector evidence was discarded instead of kept at lower rank: %q", assembly.DirectEvidenceText)
	}
	projection := mapFromAny(assembly.Counts["recall_link_projection"])
	if stringFromMap(projection, "version") != "recall_link_projection.v1" ||
		!boolFromAny(projection["materialized_candidates_only"]) ||
		boolFromAny(projection["same_source_alone_authority"]) {
		t.Fatalf("request-scoped recall-link contract missing: %#v", projection)
	}
	if got := intFromAny(projection["selected_count"], 0); got != 2 {
		t.Fatalf("selected evidence count=%d, want linked+semantic vector: %#v", got, projection)
	}
	if got := intFromAny(projection["deferred_count"], 0); got < 2 {
		t.Fatalf("same-turn unrelated and generic evidence were not deferred: %#v", projection)
	}
	if got := intFromAny(projection["excluded_count"], 0); got < 2 {
		t.Fatalf("tombstoned and owner-private evidence were not excluded: %#v", projection)
	}
	if got := intFromAny(assembly.Counts["top_k_memory_target"], 0); got != 2 {
		t.Fatalf("entity-event evidence linking changed topK: got=%d", got)
	}
	classes, _ := assembly.MemoryDeliveryPlan["classes"].([]map[string]any)
	directReserved := 0
	for _, item := range classes {
		if stringFromMap(item, "key") == "direct_evidence" {
			directReserved = intFromAny(item["configured_reserved_chars"], 0)
			break
		}
	}
	if directReserved != prepareTurnAutomaticMemoryBudgets(9000)["direct_evidence"] {
		t.Fatalf("entity-event evidence linking changed direct-evidence budget: got=%d", directReserved)
	}
}

func TestPrepareTurnRecallLinkKeepsMultipleCurrentEventsAndPrivateFence(t *testing.T) {
	memory := store.Memory{
		ID:        201,
		TurnIndex: 49,
		SummaryJSON: `{
			"characters":["강한얼","이소월"],
			"narrative_events":[
				{
					"summary":"토지 계약과 수수료 합의를 마쳤다.",
					"participants":["강한얼","이소월"],
					"evidence_excerpt":"강한얼과 이소월은 토지 계약과 수수료 합의에 서명했다."
				},
				{
					"summary":"망원경 점검과 렌즈 교정을 마쳤다.",
					"participants":["강한얼","이소월"],
					"evidence_excerpt":"강한얼과 이소월은 망원경 점검과 렌즈 교정을 함께 마쳤다."
				}
			]
		}`,
	}
	evidence := []store.DirectEvidence{
		{ID: 201, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 토지 계약과 수수료 합의에 서명했다."},
		{ID: 202, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 망원경 점검과 렌즈 교정을 함께 마쳤다."},
		{ID: 203, TurnAnchor: 49, EvidenceText: "그들은 앞으로도 서로를 믿고 움직였다."},
		{ID: 204, TurnAnchor: 49, EvidenceText: "강한얼과 이소월은 둘만 아는 비밀 보상에 합의했다."},
	}
	privateEvidence := []store.ProtagonistEntityMemory{{
		OwnerEntityName:    "이소월",
		OwnerVisibility:    "owner_private",
		SourceTurn:         49,
		EvidenceExcerpt:    "강한얼과 이소월은 둘만 아는 비밀 보상에 합의했다.",
		SecretGuard:        true,
		TargetRevealPolicy: "owner_private_until_revealed",
	}}
	selection := prepareTurnMemoryLaneSelection{VectorRelevant: []store.Memory{memory}}
	selected, trace := selectPrepareTurnDirectEvidence(
		evidence,
		[]store.DirectEvidence{evidence[2]},
		selection,
		"이소월과 강한얼은 토지 계약과 수수료 합의, 망원경 점검과 렌즈 교정을 함께 되돌아봤다.",
		[]string{"이소월", "강한얼"},
		privateEvidence,
		3,
	)
	if len(selected) != 3 {
		t.Fatalf("selected=%d, want two linked events plus vector/source support: %#v", len(selected), trace)
	}
	firstTwo := map[int64]bool{selected[0].Evidence.ID: true, selected[1].Evidence.ID: true}
	if !firstTwo[201] || !firstTwo[202] || selected[2].Evidence.ID != 203 {
		t.Fatalf("request-linked order changed: %#v", selected)
	}
	if got := intFromAny(trace["event_count"], 0); got != 2 {
		t.Fatalf("same-memory current events=%d, want 2: %#v", got, trace)
	}
	if got := intFromAny(trace["excluded_count"], 0); got != 1 {
		t.Fatalf("unfiltered private evidence fence did not exclude exact excerpt: %#v", trace)
	}

	limited, limitedTrace := selectPrepareTurnDirectEvidence(
		evidence,
		[]store.DirectEvidence{evidence[2]},
		selection,
		"이소월과 강한얼은 토지 계약과 수수료 합의, 망원경 점검과 렌즈 교정을 함께 되돌아봤다.",
		[]string{"이소월", "강한얼"},
		privateEvidence,
		2,
	)
	if len(limited) != 2 || intFromAny(limitedTrace["deferred_count"], 0) == 0 {
		t.Fatalf("support candidate limit was not applied after ranking: selected=%#v trace=%#v", limited, limitedTrace)
	}
}

func TestPrepareTurnAssemblyUsesPrivateFenceBeforeDeliveryFilter(t *testing.T) {
	const rawInput = "Alice and Bob inspect the sealed red wax letter."
	memory := store.Memory{
		ID:        301,
		TurnIndex: 5,
		SummaryJSON: `{
			"characters":["Alice","Bob"],
			"narrative_events":[{
				"summary":"Alice and Bob inspected the sealed red wax letter.",
				"participants":["Alice","Bob"],
				"evidence_excerpt":"The letter was sealed with red wax."
			}]
		}`,
	}
	evidence := []store.DirectEvidence{
		{ID: 301, TurnAnchor: 5, EvidenceText: "The letter was sealed with red wax."},
		{ID: 302, TurnAnchor: 5, EvidenceText: "Alice and Bob hid a private red wax bargain."},
	}
	privateFence := []store.ProtagonistEntityMemory{{
		ID:                 301,
		OwnerEntityKey:     "eve",
		OwnerEntityName:    "Eve",
		OwnerEntityRole:    "npc",
		OwnerVisibility:    "owner_private",
		SourceTurn:         5,
		MemoryText:         "Eve privately remembers a hidden bargain.",
		EvidenceExcerpt:    "Alice and Bob hid a private red wax bargain.",
		SecretGuard:        true,
		TargetRevealPolicy: "owner_private_until_revealed",
	}}
	deliveryPrivate := append([]store.ProtagonistEntityMemory(nil), privateFence...)
	filterPrepareTurnEntityRecollections(rawInput, []store.Memory{memory}, nil, nil, nil, nil, &deliveryPrivate)
	if len(deliveryPrivate) != 0 {
		t.Fatalf("test precondition failed: off-scene private owner survived delivery filter: %#v", deliveryPrivate)
	}

	assembly := buildPrepareTurnInjectionAssemblyWithBudget(
		[]store.Memory{memory},
		nil,
		evidence,
		nil,
		nil,
		nil,
		[]store.CharacterState{{CharacterName: "Alice"}, {CharacterName: "Bob"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		deliveryPrivate,
		privateFence,
		2,
		9000,
		rawInput,
		"default",
		nil,
		nil,
		nil,
		"auto",
		nil,
	)
	if !strings.Contains(assembly.DirectEvidenceText, "sealed with red wax") {
		t.Fatalf("public current evidence was lost: %q", assembly.DirectEvidenceText)
	}
	for _, surface := range []string{assembly.LatestDirectEvidenceText, assembly.DirectEvidenceText, assembly.ScopedVerbatimText, assembly.CharacterPrivateText} {
		if strings.Contains(surface, "private red wax bargain") {
			t.Fatalf("pre-filter private evidence escaped through final assembly: %q", surface)
		}
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

func TestPrepareTurnRecollectionDoesNotLetTechnicalSceneReactivateUnrelatedHistory(t *testing.T) {
	const rawInput = "소월, 슬아, 서현까지 떠올려보니 하나같이 예쁘고 참된 여성 같아 자신에게 과분하다고 한얼은 생각했다."
	memories := []store.Memory{
		{ID: 1, TurnIndex: 12, SummaryJSON: `{"turn_summary":"이소월은 강한얼과 술자리에서 속 깊은 대화를 나누었다.","characters":["이소월","강한얼"]}`, Importance: 0.8},
		{ID: 2, TurnIndex: 15, SummaryJSON: `{"turn_summary":"윤슬아는 강한얼의 건강을 걱정해 약재를 건넸다.","characters":["윤슬아","강한얼"]}`, Importance: 0.8},
		{ID: 3, TurnIndex: 40, SummaryJSON: `{"turn_summary":"민서현은 강한얼의 신념과 솔직함에 호감을 품었다.","characters":["민서현","강한얼"]}`, Importance: 0.8},
		{ID: 4, TurnIndex: 48, SummaryJSON: `{"turn_summary":"강한얼은 화승총의 격발 구조와 강철 가공법을 다시 계산했다.","characters":["강한얼"],"items":["화승총","강철"]}`, Importance: 0.9},
		{ID: 5, TurnIndex: 49, SummaryJSON: `{"turn_summary":"강한얼과 장영실은 연삭기 편심 축과 플라이휠 무게를 보정했다.","characters":["강한얼","장영실"],"items":["연삭기","플라이휠"]}`, Importance: 0.9},
	}
	activeStates := []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 49,
		Content:   `{"location":"서운관 공작소","present_entities":["강한얼","장영실"],"items":["연삭기","플라이휠"],"status":"기계 점검 완료"}`,
	}}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"source_table": "memories", "source_row_id": "5", "similarity": 0.99},
			{"source_table": "memories", "source_row_id": "4", "similarity": 0.98},
			{"source_table": "memories", "source_row_id": "1", "similarity": 0.90},
			{"source_table": "memories", "source_row_id": "2", "similarity": 0.89},
			{"source_table": "memories", "source_row_id": "3", "similarity": 0.88},
		},
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, activeStates)
	assembly := buildPrepareTurnInjectionAssembly(
		memories,
		nil,
		[]store.DirectEvidence{
			{EvidenceText: "강한얼과 장영실은 연삭기 편심 축과 플라이휠을 보정했다.", TurnAnchor: 49},
			{EvidenceText: "윤슬아는 강한얼의 건강을 걱정해 약재를 건넸다.", TurnAnchor: 15},
		},
		nil,
		nil,
		[]store.WorldRule{{Key: "연삭기 영점 보정", ValueJSON: `{"rule":"플라이휠은 납 무게로 보정한다."}`}},
		[]store.CharacterState{
			{CharacterName: "강한얼", TurnIndex: 49},
			{CharacterName: "이소월", TurnIndex: 12},
			{CharacterName: "윤슬아", TurnIndex: 15},
			{CharacterName: "민서현", TurnIndex: 40},
			{CharacterName: "장영실", TurnIndex: 49},
		},
		nil,
		[]store.CanonicalStateLayer{
			{LayerType: "world_state", Content: `{"rule":"연삭기 플라이휠 영점 보정"}`},
			{LayerType: "entity_state", Content: `{"events":{"main_plot":"장영실과 연삭기 편심 축을 보정했다"},"characters":["강한얼","장영실"]}`},
		},
		nil, nil, nil, nil,
		5, 12000, rawInput, "default", nil, vectorShadow, nil, perspective,
	)
	for _, want := range []string{"이소월", "윤슬아", "민서현"} {
		if !strings.Contains(assembly.ActualMemoryText, want) {
			t.Fatalf("explicitly recalled character event %q was omitted: %q", want, assembly.ActualMemoryText)
		}
	}
	for _, unwanted := range []string{"화승총", "강철 가공", "연삭기", "플라이휠", "장영실"} {
		if strings.Contains(assembly.ActualMemoryText, unwanted) {
			t.Fatalf("unrelated technical history %q was reactivated by stale scene context: %q", unwanted, assembly.ActualMemoryText)
		}
	}
	for _, unwanted := range []string{"연삭기", "플라이휠", "장영실"} {
		if strings.Contains(assembly.CanonEventText, unwanted) {
			t.Fatalf("stale canonical event %q bypassed the event-memory query: %q", unwanted, assembly.CanonEventText)
		}
	}
}

func TestPrepareTurnRelationshipRequestDoesNotReactivatePriorWorldState(t *testing.T) {
	const rawInput = "Mira pauses beside Rowan and waits for him to answer her."
	const unrelated = "turbine calibration"
	chatLogs := []store.ChatLog{{TurnIndex: 20, Role: "assistant", Content: "The turbine calibration procedure was reviewed in the old workshop."}}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{
		{StateType: "state_deltas", TurnIndex: 20, Content: `{"scene_state":{"location":"east library hall","present_entities":["Mira","Rowan"]},"relationship_changes":[{"summary":"turbine calibration remains active"}],"confidence":0.9,"verification":"direct turn evidence"}`},
		{StateType: "world_state", TurnIndex: 20, Content: `{"policy":"turbine calibration remains active"}`},
		{StateType: "entities", TurnIndex: 20, Content: `{"items":["turbine calibration gauge"],"locations":["east library hall"]}`},
	})
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 1, TurnIndex: 12, SummaryJSON: `{"turn_summary":"Mira and Rowan learned to trust each other after their first meeting.","characters":["Mira","Rowan"]}`}},
		nil,
		[]store.DirectEvidence{{ID: 1, TurnAnchor: 12, EvidenceText: "The turbine calibration procedure remains active in the old workshop."}},
		chatLogs,
		nil,
		[]store.WorldRule{
			{Scope: "session", Key: "turbine calibration procedure", ValueJSON: `{"rule":"keep using the old workshop gauge"}`},
			{Scope: "session", Key: "library etiquette for Mira and Rowan", ValueJSON: `{"rule":"wait for the other person to answer"}`},
			{Scope: "location", ScopeName: "library", Key: "library voices", ValueJSON: `{"rule":"speak softly"}`},
			{Scope: "location", ScopeName: "old workshop", Key: "old workshop restriction", ValueJSON: `{"rule":"wear a turbine calibration gauge"}`},
			{Key: "Mira turbine safety", ValueJSON: `{"rule":"inspect the turbine alone"}`},
			{Key: "narrative tense", ValueJSON: `{"rule":"keep the established tense"}`, Pinned: true},
			{Scope: "root", Key: "gravity", ValueJSON: `{"rule":"gravity always applies"}`},
			{Scope: "root", Key: "suppressed root", ValueJSON: `{"rule":"never deliver"}`, Pinned: true, Suppressed: true},
		},
		[]store.CharacterState{{CharacterName: "Mira", RelationshipsJSON: `{"relationships":[{"target":"Rowan","type":"trusted companion"}]}`}},
		nil,
		[]store.CanonicalStateLayer{
			{LayerType: "relationship_state", TurnIndex: 20, Content: `{"pair":["Mira","Rowan"],"bond_and_distance":"mutual trust"}`, Confidence: 0.9},
			{LayerType: "scene_state", TurnIndex: 20, Content: `{"scene_state":{"location":"east library hall","present_entities":["Mira","Rowan"]},"confidence":0.9}`, Confidence: 0.9},
			{LayerType: "world_state", TurnIndex: 20, Content: `{"policy":"turbine calibration remains active"}`, Confidence: 0.9},
			{LayerType: "entity_state", TurnIndex: 20, Content: `{"items":["turbine calibration gauge"],"locations":["east library hall"]}`, Confidence: 0.9},
		},
		nil, nil, nil, nil,
		5, 9000, rawInput, "default", nil, nil, nil, perspective,
	)
	if !strings.Contains(assembly.ActualMemoryText, "learned to trust") ||
		!strings.Contains(assembly.CanonRelationshipText, "mutual trust") {
		t.Fatalf("relationship support was lost: memory=%q canonical=%q", assembly.ActualMemoryText, assembly.CanonRelationshipText)
	}
	if !strings.Contains(assembly.RecentRawTurnText, unrelated) {
		t.Fatalf("prior technical chat must remain in Input Context only: %q", assembly.RecentRawTurnText)
	}
	for name, text := range map[string]string{
		"latest direct evidence": assembly.LatestDirectEvidenceText,
		"direct evidence":        assembly.DirectEvidenceText,
		"world rules":            assembly.WorldRulesText,
		"canonical world":        assembly.CanonWorldText,
	} {
		if strings.Contains(strings.ToLower(text), unrelated) {
			t.Fatalf("prior world state reactivated through %s: %q", name, text)
		}
	}
	for _, wanted := range []string{"library etiquette", "library voices", "narrative tense", "gravity"} {
		if !strings.Contains(assembly.WorldRulesText, wanted) {
			t.Fatalf("relevant or persistent world rule %q was lost: %q", wanted, assembly.WorldRulesText)
		}
	}
	for _, unwanted := range []string{"Mira turbine safety", "old workshop restriction", "suppressed root"} {
		if strings.Contains(assembly.WorldRulesText, unwanted) {
			t.Fatalf("irrelevant or suppressed world rule %q survived: %q", unwanted, assembly.WorldRulesText)
		}
	}
	ctx := buildPrepareTurnRecollectionContext(
		rawInput,
		nil,
		[]store.ActiveState{{StateType: "state_deltas", TurnIndex: 20, Content: `{"relationship_changes":[{"summary":"turbine calibration remains active"}]}`}},
		nil,
		nil,
		chatLogs,
	)
	if strings.TrimSpace(ctx.currentSceneStates) != "" {
		t.Fatalf("state_deltas without scene_state became scene relevance: %q", ctx.currentSceneStates)
	}
}

func TestPrepareTurnProtectedGuardSurvivesPronounContinuationFromRelevantPreviousEvent(t *testing.T) {
	protectedMina := store.Memory{
		SummaryJSON: `{
			"turn_summary":"Mina keeps the route private.",
			"protected_secrets":[{
				"owner":"Mina",
				"secret_kind":"mina_hidden_route",
				"secret_summary":"The route is private.",
				"disclosure_policy":"owner_private_until_revealed",
				"knowledge_scope":{"known_by":["Mina"]}
			}]
		}`,
	}
	protectedDax := store.Memory{
		SummaryJSON: `{
			"turn_summary":"Dax keeps another route private.",
			"protected_secrets":[{
				"owner":"Dax",
				"secret_kind":"dax_hidden_route",
				"secret_summary":"Another route is private.",
				"disclosure_policy":"owner_private_until_revealed",
				"knowledge_scope":{"known_by":["Dax"]}
			}]
		}`,
	}
	ctx := buildPrepareTurnRecollectionContext(
		"She hesitates before answering.",
		[]store.Memory{{
			TurnIndex:   10,
			SummaryJSON: `{"turn_summary":"Mina was asked about the route.","characters":["Mina"]}`,
		}},
		nil, nil, nil,
		[]store.ChatLog{{TurnIndex: 10, Role: "assistant", Content: "Mina pauses after the question."}},
	)
	if ctx.previousEventSummary != "" {
		t.Fatalf("pronoun-only input must not reactivate the prior event as ordinary recall: %q", ctx.previousEventSummary)
	}
	relevant, reason := prepareTurnProtectedMemoryRelevant(protectedMina, ctx, nil)
	if !relevant || reason != "previous_final_event_guard" {
		t.Fatalf("pronoun continuation lost its existing secret guard: relevant=%v reason=%q", relevant, reason)
	}
	if relevant, reason := prepareTurnProtectedMemoryRelevant(protectedDax, ctx, nil); relevant {
		t.Fatalf("unrelated prior owner received a secret guard: reason=%q", reason)
	}
	protectedMina.TurnIndex = 10
	protectedDax.TurnIndex = 9
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{protectedMina, protectedDax},
		nil, nil,
		[]store.ChatLog{{TurnIndex: 10, Role: "assistant", Content: "Mina pauses after the question."}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		2, 9000, "She hesitates before answering.", "default", nil, nil, nil,
	)
	if !strings.Contains(assembly.ProtectedMemoryText, "mina_hidden_route") {
		t.Fatalf("production assembly lost the previous-final secret guard: %q", assembly.ProtectedMemoryText)
	}
	if strings.Contains(assembly.ProtectedMemoryText, "dax_hidden_route") {
		t.Fatalf("production assembly admitted an unrelated prior-owner guard: %q", assembly.ProtectedMemoryText)
	}
}

func TestPrepareTurnStaleSceneCannotActivateRelationshipOrVolatileWorldLanes(t *testing.T) {
	rawInput := "Han-eol looks beyond the grinder and considers roads and maritime transport."
	chatLogs := []store.ChatLog{{TurnIndex: 39, Role: "assistant", Content: "Han-eol leaves the oil shop."}}
	activeStates := []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 38,
		Content:   `{"location":"old workshop","present_entities":["Han-eol","Jang","Bae"]}`,
	}}
	private := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "jang", OwnerEntityName: "Jang", OwnerEntityRole: "npc", MemoryText: "Jang admires Han-eol's old machine.", SourceTurn: 8},
		{ID: 2, OwnerEntityKey: "bae", OwnerEntityName: "Bae", OwnerEntityRole: "npc", MemoryText: "Bae remembers the old bellows.", SourceTurn: 4},
	}
	filterPrepareTurnEntityRecollections(rawInput, nil, activeStates, nil, nil, nil, &private, chatLogs)
	if len(private) != 0 {
		t.Fatalf("stale scene activated NPC recollections: %#v", private)
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, activeStates)
	assembly := buildPrepareTurnInjectionAssembly(
		nil,
		[]store.KGTriple{{Subject: "Han-eol", Predicate: "demonstrated_to", Object: "Bae"}},
		nil, chatLogs, nil, nil,
		[]store.CharacterState{
			{CharacterName: "Jang", RelationshipsJSON: `{"Han-eol":{"summary":"admires his old machine"}}`},
			{CharacterName: "Bae", RelationshipsJSON: `{"Han-eol":{"summary":"remembers the old bellows"}}`},
		},
		nil,
		[]store.CanonicalStateLayer{
			{LayerType: "relationship_state", TurnIndex: 38, Content: `{"pair":["Han-eol","Bae"],"bond_and_distance":"old workshop trust"}`, Confidence: 0.9},
			{LayerType: "scene_state", TurnIndex: 38, Content: `{"location":"old workshop"}`, Confidence: 0.9},
			{LayerType: "entity_state", TurnIndex: 38, Content: `{"items":["old bellows"],"locations":["old workshop"]}`, Confidence: 0.9},
		},
		nil, nil, nil, private,
		5, 9000, rawInput, "default", nil, nil, nil, perspective,
	)
	if assembly.CharacterRelationshipText != "" || assembly.CanonRelationshipText != "" || assembly.KGText != "" {
		t.Fatalf("stale relationship lane survived: character=%q canonical=%q kg=%q",
			assembly.CharacterRelationshipText, assembly.CanonRelationshipText, assembly.KGText)
	}
	if strings.Contains(assembly.CanonWorldText, "old workshop") || strings.Contains(assembly.CanonWorldText, "old bellows") {
		t.Fatalf("stale volatile world state survived: %q", assembly.CanonWorldText)
	}
	if boolFromAny(assembly.Counts["current_scene_state_is_current"]) {
		t.Fatalf("stale scene was marked current: %#v", assembly.Counts)
	}
}

func TestPrepareTurnVectorSuccessDoesNotFillEventBudgetWithLexicalHistory(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 38, SummaryJSON: `{"turn_summary":"Han-eol repaired the grinder with whale oil.","items":["grinder","whale oil"]}`},
		{ID: 2, TurnIndex: 7, SummaryJSON: `{"turn_summary":"Han-eol showed an escapement at the old workshop.","items":["machine"]}`},
		{ID: 3, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Han-eol demonstrated old bellows machinery.","items":["machine"]}`},
	}
	vectorShadow := map[string]any{
		"search_attempted": true,
		"search_result":    "ok",
		"search_results": []map[string]any{
			{"source_table": "memories", "source_row_id": "1", "similarity": 0.8},
		},
	}
	selection := selectPrepareTurnMemoryLanesWithVector(
		memories,
		"Han-eol looks beyond the grinder and considers roads and maritime transport.",
		5,
		vectorShadow,
		[]string{"Han-eol"},
		nil,
	)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 1 {
		t.Fatalf("vector result was not retained: %#v", selection.VectorRelevant)
	}
	if len(selection.Relevant) != 0 {
		t.Fatalf("successful vector recall was padded with lexical history: %#v", selection.Relevant)
	}
	if !boolFromAny(selection.Trace["general_lexical_refill_skipped_after_vector_success"]) {
		t.Fatalf("vector-success no-fill decision missing: %#v", selection.Trace)
	}
}

func TestPrepareTurnOpenGoalCannotSelfActivateThroughSceneState(t *testing.T) {
	const goal = "Restore the observatory clock"
	ctx := buildPrepareTurnRecollectionContext(
		"Mira asks Rowan whether their trust has changed.",
		nil,
		[]store.ActiveState{{
			StateType: "scene_state",
			TurnIndex: 20,
			Content:   `{"scene_state":{"location":"library","present_entities":["Mira","Rowan"]},"unresolved_threads":{"opened":["Restore the observatory clock"]}}`,
		}},
		nil,
		[]store.PendingThread{{
			Title:       goal,
			Description: goal,
			Status:      "open",
			SourceTurn:  20,
		}},
	)
	if strings.Contains(ctx.currentSceneStates, goal) || strings.Contains(ctx.currentSceneStates, "unresolved_threads") {
		t.Fatalf("open goal leaked into the scene/world relevance query: %q", ctx.currentSceneStates)
	}
	if strings.Contains(ctx.unresolvedGoals, goal) {
		t.Fatalf("open goal selected itself without current-request support: %q", ctx.unresolvedGoals)
	}
}

func TestPrepareTurnRelevantOpenGoalIsDeliveredOnceOutsideWorldState(t *testing.T) {
	const goal = "Restore the observatory clock"
	const rawInput = "Mira postpones the observatory clock repair and speaks with Rowan in the library."
	pending := []store.PendingThread{{
		Title:       goal,
		Description: goal,
		Status:      "open",
		SourceTurn:  20,
	}}
	canonical := []store.CanonicalStateLayer{{
		LayerType: "scene_state",
		TurnIndex: 20,
		Content:   `{"scene_state":{"location":"library","present_entities":["Mira","Rowan"]},"unresolved_threads":{"opened":["Restore the observatory clock"]}}`,
	}}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, pending, canonical,
		nil, nil, nil, nil,
		5, 9000, rawInput, "default", nil, nil, nil,
	)
	if !strings.Contains(assembly.PendingThreadText, goal) {
		t.Fatalf("request-relevant open goal was not delivered in its owner lane: %q", assembly.PendingThreadText)
	}
	if strings.Contains(assembly.CanonWorldText, goal) || strings.Contains(assembly.CanonWorldText, "unresolved_threads") {
		t.Fatalf("open goal leaked into world-state delivery: %q", assembly.CanonWorldText)
	}
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if strings.Count(finalText, goal) != 1 {
		t.Fatalf("goal should be delivered once, got %d copies: %q", strings.Count(finalText, goal), finalText)
	}
}
