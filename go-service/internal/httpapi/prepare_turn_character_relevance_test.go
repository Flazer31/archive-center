package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnCurrentCharacterRanksBeforeCharacterCap(t *testing.T) {
	states := []store.CharacterState{
		{CharacterName: "강한얼", StatusJSON: `{"emotion":"밤 기계 고민"}`, TurnIndex: 44},
		{CharacterName: "세종", StatusJSON: `{"emotion":"고민하는 지탱해"}`, TurnIndex: 43},
		{CharacterName: "민서현", StatusJSON: `{"emotion":"고민하는 지탱해"}`, TurnIndex: 42},
		{CharacterName: "윤기", StatusJSON: `{"emotion":"밤 기계 걱정"}`, TurnIndex: 41},
		{CharacterName: "솔희", StatusJSON: `{"emotion":"고민하는 지탱해"}`, TurnIndex: 40},
		{CharacterName: "윤슬아", StatusJSON: `{"emotion":"한얼에 대한 풋풋한 호감","location":"윤기 저택"}`, RelationshipsJSON: `{"강한얼":{"type":"호감","description":"혼인 제안 이후 서로를 알아가는 중"}}`, TurnIndex: 15},
	}
	raw := "밤 기계를 고민하는 한얼을 보던 윤기는 딸 슬아에게 한얼을 지탱해 달라고 말했다."
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", Content: `{"present_entities":["강한얼","윤기","윤슬아"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, states, nil, nil, nil, nil, nil, nil,
		5, 12000, raw, "default", nil, nil, nil, perspective,
	)
	if !strings.Contains(assembly.CharacterObjectiveText, "윤슬아") {
		t.Fatalf("current short-name character was capped before relevance ranking: %q", assembly.CharacterObjectiveText)
	}
	if !strings.Contains(assembly.CharacterRelationshipText, "윤슬아") || !strings.Contains(assembly.CharacterRelationshipText, "강한얼") {
		t.Fatalf("current character relationship was not delivered: %q", assembly.CharacterRelationshipText)
	}
	if got := intFromAny(assembly.Counts["character_state_candidate_capped"], 0); got != 0 {
		t.Fatalf("character_state_candidate_capped=%d, want 0 under independent candidate safety bound: %#v", got, assembly.Counts)
	}
	if assembly.Counts["character_state_relevance_before_cap"] != true || assembly.Counts["character_state_reviewed_identity_alias_priority"] != true {
		t.Fatalf("missing relevance-order trace: %#v", assembly.Counts)
	}
}

func TestPrepareTurnUnobservedSceneDoesNotPromoteDirectRecollectionsToObjectiveState(t *testing.T) {
	const rawInput = "소월, 슬아, 서현까지 떠올려보니 하나같이 자신에게 과분하다고 한얼은 생각했다."
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{
			{CharacterName: "이소월", StatusJSON: `{"emotion":"여유","location":"월하방"}`},
			{CharacterName: "윤슬아", StatusJSON: `{"emotion":"연정","location":"윤기의 사저"}`},
			{CharacterName: "민서현", StatusJSON: `{"emotion":"호감","location":"민정호 사저"}`},
		},
		nil, nil, nil, nil, nil, nil,
		5, 12000, rawInput, "default", nil, nil, nil,
	)
	if strings.TrimSpace(assembly.CharacterObjectiveText) != "" {
		t.Fatalf("unobserved scene promoted recalled characters to objective state: %q", assembly.CharacterObjectiveText)
	}
	if assembly.Counts["objective_entity_source"] != "unobserved_no_objective_state_delivery" {
		t.Fatalf("unobserved objective source was not exposed: %#v", assembly.Counts)
	}
}

func TestPrepareTurnUnreviewedShortSuffixIsNotAnAlias(t *testing.T) {
	for _, name := range []string{"김슬아", "윤슬아"} {
		if rank := prepareTurnDirectEntityMentionRank("슬아가 찾아왔다.", name, nil); rank != 0 {
			t.Fatalf("unreviewed suffix ranked %q as current: rank=%d", name, rank)
		}
	}
}

func TestPrepareTurnPrivateRecollectionAcceptsReviewedIdentityAlias(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "min_seohyeon", OwnerEntityName: "민서현", OwnerEntityRole: "npc", MemoryText: "민서현의 개인 기억"},
		{ID: 2, OwnerEntityKey: "yun_seula", OwnerEntityName: "윤슬아", OwnerEntityRole: "npc", MemoryText: "윤슬아는 한얼을 걱정하면서도 가까워지고 싶어 한다."},
	}
	trace := filterPrepareTurnEntityRecollectionsWithAliases(
		"윤기는 딸 슬아에게 한얼을 지탱해 달라고 말했다.",
		nil, nil, nil, nil, nil, &memories,
		map[string]any{"슬아": "윤슬아"},
	)
	if len(memories) != 1 || memories[0].OwnerEntityName != "윤슬아" {
		t.Fatalf("reviewed alias owner was not selected: %#v trace=%#v", memories, trace)
	}
	if got := intFromAny(trace["character_private_reviewed_identity_aliases"], 0); got < 1 {
		t.Fatalf("reviewed identity alias trace missing: %#v", trace)
	}
}

func TestPrepareTurnPrivateRecollectionUsesLatestAcceptedAssistantOnlyForEntityScope(t *testing.T) {
	private := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerEntityRole: "npc", MemoryText: "Mira remembers smiling at the silver cup."},
		{ID: 2, OwnerEntityKey: "rook", OwnerEntityName: "Rook", OwnerEntityRole: "npc", MemoryText: "Rook privately fears the distant storm."},
	}
	chatLogs := []store.ChatLog{
		{TurnIndex: 8, Role: "user", Content: "Continue."},
		{TurnIndex: 8, Role: "assistant", Content: "Mira smiled at the silver cup and lifted it."},
	}
	trace := filterPrepareTurnEntityRecollections(
		"She accepts it carefully.",
		nil, nil, nil, nil, nil, &private, chatLogs,
	)
	if len(private) != 1 || private[0].OwnerEntityName != "Mira" {
		t.Fatalf("latest accepted scene participant was not scoped precisely: memories=%#v trace=%#v", private, trace)
	}
	if intFromAny(trace["accepted_context_owner_count"], 0) != 1 {
		t.Fatalf("accepted assistant entity scope was not traced: %#v", trace)
	}
}

func TestPrepareTurnEntityRecollectionHasNoPreDeliveryCountCap(t *testing.T) {
	entries := make([]store.ProtagonistEntityMemory, 0, 81)
	for i := 0; i < 81; i++ {
		entries = append(entries, store.ProtagonistEntityMemory{
			ID:              int64(i + 1),
			OwnerEntityKey:  "ari",
			OwnerEntityName: "Ari",
			MemoryText:      fmt.Sprintf("recollection-%d", i),
		})
	}
	text := buildCharacterPrivateRecollectionText(entries, 100000)
	if !strings.Contains(text, "recollection-80") {
		t.Fatalf("eligible recollection after the former read window was lost: %s", text)
	}
}

func TestPrepareTurnDirectEntityMemoryOwnersUsesReviewedIdentityAlias(t *testing.T) {
	owners := []store.ProtagonistEntityMemoryOwner{
		{OwnerEntityKey: "first", OwnerEntityName: "첫인물"},
		{OwnerEntityKey: "yunseula", OwnerEntityName: "윤슬아"},
		{OwnerEntityKey: "third", OwnerEntityName: "셋인물"},
	}
	selected := prepareTurnDirectEntityMemoryOwnersWithAliases("아버지는 딸 슬아에게 말을 건넸다.", owners, map[string]any{"슬아": "윤슬아"})
	if len(selected) != 1 || selected[0].OwnerEntityKey != "yunseula" {
		t.Fatalf("selected = %#v, want only yunseula", selected)
	}
}

func TestPrepareTurnDirectEntityMemoryOwnersRejectsAmbiguousShortName(t *testing.T) {
	owners := []store.ProtagonistEntityMemoryOwner{
		{OwnerEntityKey: "kimseula", OwnerEntityName: "김슬아"},
		{OwnerEntityKey: "yunseula", OwnerEntityName: "윤슬아"},
	}
	if selected := prepareTurnDirectEntityMemoryOwners("슬아가 찾아왔다.", owners); len(selected) != 0 {
		t.Fatalf("selected = %#v, want no ambiguous owner", selected)
	}
}

func TestPrepareTurnQualifiedOwnerTailMatchesOnlyWhenUnique(t *testing.T) {
	owners := []store.ProtagonistEntityMemoryOwner{
		{OwnerEntityKey: "min_seohyeon", OwnerEntityName: "예조판서 민정호의 딸 민서현"},
		{OwnerEntityKey: "yun_seula", OwnerEntityName: "윤슬아"},
	}
	identityAliases := map[string]any{"민서현": "예조판서 민정호의 딸 민서현"}
	selected := prepareTurnDirectEntityMemoryOwnersWithAliases("윤슬아 앞에 민서현이 나타났다.", owners, identityAliases)
	if len(selected) != 2 {
		t.Fatalf("qualified owner tail was not matched: %#v", selected)
	}

	ambiguous := []store.ProtagonistEntityMemoryOwner{
		{OwnerEntityKey: "first", OwnerEntityName: "첫 번째 기록의 민서현"},
		{OwnerEntityKey: "second", OwnerEntityName: "두 번째 기록의 민서현"},
	}
	if selected := prepareTurnDirectEntityMemoryOwners("민서현이 나타났다.", ambiguous); len(selected) != 0 {
		t.Fatalf("ambiguous qualified owner tail was accepted: %#v", selected)
	}

	private := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "min_seohyeon", OwnerEntityName: "예조판서 민정호의 딸 민서현", OwnerEntityRole: "npc", MemoryText: "민서현은 강한얼을 마음에 둔 사내로 여긴다."},
		{ID: 2, OwnerEntityKey: "yun_seula", OwnerEntityName: "윤슬아", OwnerEntityRole: "npc", MemoryText: "윤슬아는 강한얼에게 호감을 품고 있다."},
	}
	filterPrepareTurnEntityRecollectionsWithAliases("윤슬아 앞에 민서현이 나타나 강한얼에게 품은 호감과 마음에 둔 감정을 떠올렸다.", nil, nil, nil, nil, nil, &private, identityAliases)
	if len(private) != 2 {
		t.Fatalf("qualified owner memory was dropped after indexed read: %#v", private)
	}
}

func TestPrepareTurnOneDirectNameDoesNotExpandUnrelatedRelationships(t *testing.T) {
	const rawInput = "비가 그친 저녁, 베라가 문을 열고 들어왔다."
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 41,
		Content:   `{"location":"객실","present_entities":["주인공","베라"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{
			{
				CharacterName:     "베라",
				RelationshipsJSON: `{"주인공":{"type":"trust","description":"둘이 함께 겪은 사건에서 생긴 신뢰"},"행인 C":{"type":"one_time_trade"}}`,
			},
			{CharacterName: "주인공"},
			{CharacterName: "행인 C"},
		},
		nil, nil, nil, nil, nil, nil,
		1, 9000, rawInput, "default", nil, nil, nil, perspective,
	)

	if !strings.Contains(assembly.CharacterRelationshipText, "둘이 함께 겪은 사건") {
		t.Fatalf("current-pair relationship was lost: %q", assembly.CharacterRelationshipText)
	}
	if strings.Contains(assembly.CharacterRelationshipText, "행인 C") ||
		strings.Contains(assembly.CharacterRelationshipText, "one_time_trade") {
		t.Fatalf("one direct name expanded an unrelated relationship: %q", assembly.CharacterRelationshipText)
	}
}

func TestPrepareTurnDirectReencounterDeliversOldCurrentPairEvent(t *testing.T) {
	const rawInput = "비가 그친 저녁, 베라가 문을 열고 다시 들어왔다."
	memories := []store.Memory{
		{
			ID:          1,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"베라와 주인공은 돌다리에서 서로를 구하고 신뢰하기 시작했다.","characters":["베라","주인공"],"locations":["돌다리"]}`,
			Importance:  0.7,
		},
		{
			ID:          2,
			TurnIndex:   39,
			SummaryJSON: `{"turn_summary":"베라는 시장에서 행인 C와 값을 흥정했다.","characters":["베라","행인 C"],"locations":["시장"]}`,
			Importance:  0.95,
		},
		{
			ID:          4,
			TurnIndex:   40,
			SummaryJSON: `{"turn_summary":"베라 홀로 창가에서 빗소리를 들었다.","characters":["베라"],"locations":["객실"]}`,
			Importance:  0.99,
		},
	}
	states := []store.CharacterState{
		{
			CharacterName:     "베라",
			RelationshipsJSON: `{"주인공":{"type":"trust","description":"돌다리 사건에서 서로를 구하며 생긴 신뢰"}}`,
		},
		{CharacterName: "주인공"},
		{
			CharacterName:     "행인 C",
			RelationshipsJSON: `{"베라":{"type":"one_time_trade"}}`,
		},
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 41,
		Content:   `{"location":"객실","present_entities":["주인공","베라"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, states, nil, nil, nil, nil, nil, nil,
		1, 9000, rawInput, "default", nil, nil, nil, perspective,
	)

	if !strings.Contains(assembly.ActualMemoryText, "돌다리에서 서로를 구하고 신뢰하기 시작했다") {
		t.Fatalf("old current-pair event was not delivered for the reencounter: %q", assembly.ActualMemoryText)
	}
	if !strings.Contains(assembly.CharacterRelationshipText, "돌다리 사건") {
		t.Fatalf("stored relationship state was not accompanied by its event evidence: %q", assembly.CharacterRelationshipText)
	}
	planText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(planText, "돌다리에서 서로를 구하고 신뢰하기 시작했다") {
		t.Fatalf("old current-pair event was lost from final memory delivery: %q", planText)
	}
	for _, unrelated := range []string{"행인 C", "값을 흥정", "홀로 창가"} {
		if strings.Contains(assembly.ActualMemoryText, unrelated) || strings.Contains(planText, unrelated) {
			t.Fatalf("non-pair memory %q was selected as reencounter evidence: actual=%q final=%q", unrelated, assembly.ActualMemoryText, planText)
		}
	}
}

func TestPrepareTurnVectorCurrentPairDoesNotAddSecondPairMemory(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 4, SummaryJSON: `{"turn_summary":"베라와 주인공의 오래된 사건","characters":["베라","주인공"]}`, Importance: 1},
		{ID: 2, TurnIndex: 40, SummaryJSON: `{"turn_summary":"베라와 주인공의 최근 사건","characters":["베라","주인공"]}`, Importance: 0.5},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{{
			"id": "memory:test:2", "tier": "memory", "similarity": 0.9,
			"similarity_source": "cosine_from_query_and_stored_embedding",
		}},
	}
	selection := selectPrepareTurnMemoryLanesWithVector(
		memories, "베라가 돌아왔다.", 3, vectorShadow, []string{"베라"}, []string{"주인공", "베라"},
	)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 2 ||
		prepareTurnSelectedMemoryCount(selection) != 1 {
		t.Fatalf("vector current-pair coverage added a semantic duplicate: %#v", selection)
	}
}

func TestPrepareTurnDirectReencounterDoesNotInventUnsupportedFamiliarity(t *testing.T) {
	const rawInput = "비가 그친 저녁, 베라가 문을 열고 들어왔다."
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 41,
		Content:   `{"location":"객실","present_entities":["주인공","베라"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{
			ID:          2,
			TurnIndex:   39,
			SummaryJSON: `{"turn_summary":"베라는 시장에서 행인 C와 값을 흥정했다.","characters":["베라","행인 C"],"locations":["시장"]}`,
			Importance:  0.95,
		}},
		nil, nil, nil, nil, nil,
		[]store.CharacterState{{CharacterName: "베라"}, {CharacterName: "주인공"}, {CharacterName: "행인 C"}},
		nil, nil, nil, nil, nil, nil,
		1, 9000, rawInput, "default", nil, nil, nil, perspective,
	)

	for _, unsupported := range []string{"행인 C", "값을 흥정"} {
		if strings.Contains(assembly.Text, unsupported) {
			t.Fatalf("unsupported familiarity or unrelated event was delivered %q: %q", unsupported, assembly.Text)
		}
	}
	if strings.TrimSpace(assembly.ActualMemoryText) != "" ||
		strings.TrimSpace(assembly.CharacterRelationshipText) != "" {
		t.Fatalf("no-support reencounter synthesized continuity: memory=%q relationship=%q", assembly.ActualMemoryText, assembly.CharacterRelationshipText)
	}
	classes, _ := assembly.MemoryDeliveryPlan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "subjective_relationship" && intFromAny(class["selected_count"], -1) != 0 {
			t.Fatalf("no-support reencounter synthesized subjective relationship items: %#v", class)
		}
	}
}

func TestPrepareTurnDirectPairProtectedEventNeverBecomesActualMemory(t *testing.T) {
	const rawInput = "비가 그친 저녁, 베라가 문을 열고 들어왔다."
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 41,
		Content:   `{"location":"객실","present_entities":["주인공","베라"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{
			ID:        9,
			TurnIndex: 4,
			SummaryJSON: `{
				"turn_summary":"RAW_PAIR_SECRET 베라와 주인공의 숨겨진 맹세",
				"characters":["베라","주인공"],
				"relationship_changes":[{"pair":["베라","주인공"],"type":"secret_oath"}],
				"protected_secrets":[{
					"owner":"베라",
					"secret_kind":"hidden_oath",
					"secret_summary":"RAW_PAIR_SECRET",
					"disclosure_policy":"owner_private_until_revealed",
					"knowledge_scope":{"known_by":["베라"]}
				}]
			}`,
			Importance: 0.9,
		}},
		nil, nil, nil, nil, nil,
		[]store.CharacterState{{CharacterName: "베라"}, {CharacterName: "주인공"}},
		nil, nil, nil, nil, nil, nil,
		1, 9000, rawInput, "default", nil, nil, nil, perspective,
	)

	if strings.Contains(assembly.ActualMemoryText, "RAW_PAIR_SECRET") {
		t.Fatalf("protected current-pair memory bypassed into actual memory: %q", assembly.ActualMemoryText)
	}
	if strings.TrimSpace(assembly.ProtectedMemoryText) == "" ||
		strings.Contains(assembly.ProtectedMemoryText, "RAW_PAIR_SECRET") {
		t.Fatalf("protected pair memory was lost or exposed raw: %q", assembly.ProtectedMemoryText)
	}
}

func TestMergePrepareTurnEntityMemoriesKeepsDirectOwnerFirst(t *testing.T) {
	direct := []store.ProtagonistEntityMemory{{ID: 30, OwnerEntityName: "현재 인물"}}
	recent := []store.ProtagonistEntityMemory{{ID: 10, OwnerEntityName: "최근 인물"}, {ID: 30, OwnerEntityName: "현재 인물"}}
	merged := mergePrepareTurnEntityMemories(direct, recent)
	if len(merged) != 2 || merged[0].ID != 30 || merged[1].ID != 10 {
		t.Fatalf("merged = %#v, want direct owner first with duplicate removed", merged)
	}
}

func TestPrepareTurnPrivateRecollectionDoesNotLetRecencyOverrideDurableEmotion(t *testing.T) {
	items := []store.ProtagonistEntityMemory{
		{ID: 2, OwnerEntityKey: "owner", OwnerEntityName: "가나다", OwnerEntityRole: "npc", SourceTurn: 90, MemoryText: "최근의 평범한 관찰", Importance10: 5, EmotionalWeight: 0.1},
		{ID: 1, OwnerEntityKey: "owner", OwnerEntityName: "가나다", OwnerEntityRole: "npc", SourceTurn: 10, MemoryText: "오래된 핵심 관계 기억", Importance10: 8, EmotionalWeight: 0.9},
	}
	filterPrepareTurnEntityRecollections("가나다가 찾아와 최근의 평범한 관찰과 오래된 핵심 관계 기억을 함께 떠올렸다.", nil, nil, nil, nil, nil, &items)
	if len(items) != 2 || items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("selected = %#v, want durable high-emotion memory first and distinct relevant fill second", items)
	}
}

func TestPrepareTurnExplicitRecollectionsDoNotBecomeOffSceneObjectiveState(t *testing.T) {
	const rawInput = "소월과 나눈 술자리 대화, 슬아가 약재를 건넨 일, 서현이 자신의 신념과 솔직함을 바라보던 순간까지 떠올려보니 하나같이 예쁘고 참된 여성 같아 자신에게 과분하다고 한얼은 생각했다."
	activeStates := []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 51,
		Content:   `{"location":"한얼의 방","present_entities":["강한얼"],"status":"혼자 쉬는 중"}`,
	}}
	privateMemories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "lee_sowol", OwnerEntityName: "이소월", OwnerEntityRole: "npc", MemoryText: "이소월은 한얼과 나눈 술자리 대화를 흥미롭게 기억한다.", Importance10: 8},
		{ID: 2, OwnerEntityKey: "yun_seula", OwnerEntityName: "윤슬아", OwnerEntityRole: "npc", MemoryText: "윤슬아는 한얼을 걱정하며 약재를 건넨 일을 소중히 여긴다.", Importance10: 8},
		{ID: 3, OwnerEntityKey: "min_seohyeon", OwnerEntityName: "민서현", OwnerEntityRole: "npc", MemoryText: "민서현은 한얼의 신념과 솔직함에 호감을 느꼈다.", Importance10: 8},
		{ID: 4, OwnerEntityKey: "unrelated", OwnerEntityName: "배상문", OwnerEntityRole: "npc", MemoryText: "배상문은 연삭기 제작을 기억한다.", Importance10: 9},
	}
	identityAliases := map[string]any{"소월": "이소월", "슬아": "윤슬아", "서현": "민서현"}
	trace := filterPrepareTurnEntityRecollectionsWithAliases(rawInput, nil, activeStates, nil, nil, nil, &privateMemories, identityAliases)
	if len(privateMemories) != 3 {
		t.Fatalf("explicit recollection coverage = %d, want 3: memories=%#v trace=%#v", len(privateMemories), privateMemories, trace)
	}
	for _, want := range []string{"이소월", "윤슬아", "민서현"} {
		found := false
		for _, item := range privateMemories {
			if item.OwnerEntityName == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("explicit owner %q was omitted: memories=%#v trace=%#v", want, privateMemories, trace)
		}
	}

	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, activeStates)
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{
			{CharacterName: "강한얼", StatusJSON: `{"emotion":"휴식하며 회상 중","location":"한얼의 방"}`, TurnIndex: 51},
			{CharacterName: "이소월", StatusJSON: `{"emotion":"여유","location":"월하방"}`, TurnIndex: 49},
			{CharacterName: "윤슬아", StatusJSON: `{"emotion":"연정","location":"윤기의 사저"}`, TurnIndex: 45},
			{CharacterName: "민서현", StatusJSON: `{"emotion":"호감","location":"민정호 사저"}`, TurnIndex: 40},
		},
		nil, nil, nil, nil, nil, privateMemories,
		5, 12000, rawInput, "default", nil, nil, nil, perspective,
	)
	if !strings.Contains(assembly.CharacterObjectiveText, "강한얼") {
		t.Fatalf("actual scene character objective state was lost: %q", assembly.CharacterObjectiveText)
	}
	for _, offScene := range []string{"이소월", "윤슬아", "민서현"} {
		if strings.Contains(assembly.CharacterObjectiveText, offScene) {
			t.Fatalf("recalled off-scene character %q was promoted to objective current state: %q", offScene, assembly.CharacterObjectiveText)
		}
		if !strings.Contains(assembly.CharacterPrivateText, offScene) {
			t.Fatalf("recalled character %q lost subjective recollection: %q", offScene, assembly.CharacterPrivateText)
		}
	}
	if strings.Contains(assembly.CharacterPrivateText, "배상문") {
		t.Fatalf("unmentioned private owner leaked into recollection: %q", assembly.CharacterPrivateText)
	}
}

func TestPrepareTurnSelectedHistoricalMemoryCannotExpandOtherLanes(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 1, TurnIndex: 8, SummaryJSON: `{"turn_summary":"Mira opens the brass gate and remembers Juno's old passport."}`}},
		nil,
		nil,
		nil,
		nil,
		[]store.WorldRule{{ID: 11, Key: "passport law", ValueJSON: `{"rule":"Juno's passport requires a harbor seal"}`}},
		nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Mira opens the brass gate.", "default", nil, nil, nil,
	)
	if !strings.Contains(assembly.MemoryText, "Juno's old passport") {
		t.Fatalf("fixture memory was not selected: %q", assembly.MemoryText)
	}
	if strings.Contains(assembly.WorldRulesText, "passport") || strings.Contains(assembly.Text, "harbor seal") {
		t.Fatalf("selected historical memory expanded unrelated world relevance: %q", assembly.WorldRulesText)
	}
}

func TestPrepareTurnCharacterRelationshipsRequireCurrentCounterparty(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{{
			CharacterName: "Mira",
			StatusJSON:    `{"emotion":"calm"}`,
			RelationshipsJSON: `{
				"Juno":{"type":"trust","description":"Mira trusts Juno with the brass key."},
				"Rowan":{"type":"rivalry","description":"Mira resents Rowan over the harbor dispute."}
			}`,
			TurnIndex: 12,
		}},
		nil, nil, nil, nil, nil, nil,
		5, 9000, "Mira meets Juno beside the brass gate.", "default", nil, nil, nil,
	)
	if !strings.Contains(assembly.CharacterRelationshipText, "Juno") {
		t.Fatalf("current counterparty relationship was lost: %q", assembly.CharacterRelationshipText)
	}
	if strings.Contains(assembly.CharacterRelationshipText, "Rowan") || strings.Contains(assembly.CharacterRelationshipText, "harbor dispute") {
		t.Fatalf("absent counterparty relationship survived because the owner was current: %q", assembly.CharacterRelationshipText)
	}
	if got := intFromAny(assembly.Counts["character_relationship_irrelevant_dropped"], 0); got != 1 {
		t.Fatalf("character_relationship_irrelevant_dropped=%d, want 1", got)
	}
}

func TestPrepareTurnWorldEpisodeAndCanonicalRequireCurrentSceneRelevance(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil,
		[]store.WorldRule{
			{ID: 1, Key: "brass gate", ValueJSON: `{"rule":"The brass gate opens with Mira's key"}`},
			{ID: 2, Key: "harbor passport", ValueJSON: `{"rule":"A harbor seal is required"}`},
		},
		nil, nil,
		[]store.CanonicalStateLayer{
			{ID: 1, LayerType: "world_state", Content: `{"gate":"Mira holds the brass key"}`, Confidence: 0.9},
			{ID: 2, LayerType: "world_state", Content: `{"harbor":"Juno's passport is missing"}`, Confidence: 0.9},
		},
		[]store.EpisodeSummary{
			{ID: 1, FromTurn: 1, ToTurn: 3, SummaryText: "Mira found the brass gate key"},
			{ID: 2, FromTurn: 4, ToTurn: 6, SummaryText: "Juno searched the harbor for a passport"},
		},
		nil, nil, nil,
		5, 9000, "Mira turns the brass key at the gate.", "default", nil, nil, nil,
	)
	for _, text := range []string{assembly.WorldRulesText, assembly.CanonText, assembly.EpisodeText} {
		if strings.Contains(text, "Juno") || strings.Contains(text, "passport") || strings.Contains(text, "harbor") {
			t.Fatalf("unrelated latest/support material survived current-scene gate: %q", text)
		}
	}
	checks := map[string]string{
		"brass gate":                    assembly.WorldRulesText,
		"Mira holds the brass key":      assembly.CanonText,
		"Mira found the brass gate key": assembly.EpisodeText,
	}
	for wanted, text := range checks {
		if !strings.Contains(text, wanted) {
			t.Fatalf("relevant current-scene material %q was lost: %s", wanted, text)
		}
	}
}
