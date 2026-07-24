package httpapi

import (
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
	if assembly.Counts["character_state_relevance_before_cap"] != true || assembly.Counts["character_state_unique_short_alias_priority"] != true {
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

func TestPrepareTurnShortCharacterAliasMustBeUnique(t *testing.T) {
	states := []store.CharacterState{
		{CharacterName: "김슬아"},
		{CharacterName: "윤슬아"},
	}
	aliases := prepareTurnObservedShortEntityAliases(states)
	if len(aliases) != 0 {
		t.Fatalf("ambiguous short alias was accepted: %#v", aliases)
	}
	for _, state := range states {
		if rank := prepareTurnDirectEntityMentionRank("슬아가 찾아왔다.", state.CharacterName, aliases); rank != 0 {
			t.Fatalf("ambiguous alias ranked %q as current: rank=%d", state.CharacterName, rank)
		}
	}
}

func TestPrepareTurnPrivateRecollectionAcceptsUniqueObservedShortName(t *testing.T) {
	memories := []store.ProtagonistEntityMemory{
		{ID: 1, OwnerEntityKey: "min_seohyeon", OwnerEntityName: "민서현", OwnerEntityRole: "npc", MemoryText: "민서현의 개인 기억"},
		{ID: 2, OwnerEntityKey: "yun_seula", OwnerEntityName: "윤슬아", OwnerEntityRole: "npc", MemoryText: "윤슬아는 한얼을 걱정하면서도 가까워지고 싶어 한다."},
	}
	trace := filterPrepareTurnEntityRecollections(
		"윤기는 딸 슬아에게 한얼을 지탱해 달라고 말했다.",
		nil, nil, nil, nil, nil, &memories,
	)
	if len(memories) != 1 || memories[0].OwnerEntityName != "윤슬아" {
		t.Fatalf("unique short-name owner was not selected: %#v trace=%#v", memories, trace)
	}
	if got := intFromAny(trace["character_private_unique_short_aliases"], 0); got < 1 {
		t.Fatalf("short alias trace missing: %#v", trace)
	}
}

func TestPrepareTurnEntityRecollectionReadsCandidatesBeforeDeliveryCap(t *testing.T) {
	if got := prepareTurnEntityRecollectionCandidateLimit(5); got != 80 {
		t.Fatalf("candidate limit=%d, want 80", got)
	}
	if got := prepareTurnEntityRecollectionCandidateLimit(30); got != 120 {
		t.Fatalf("bounded candidate limit=%d, want 120", got)
	}
}

func TestPrepareTurnDirectEntityMemoryOwnersUsesUniqueShortName(t *testing.T) {
	owners := []store.ProtagonistEntityMemoryOwner{
		{OwnerEntityKey: "first", OwnerEntityName: "첫인물"},
		{OwnerEntityKey: "yunseula", OwnerEntityName: "윤슬아"},
		{OwnerEntityKey: "third", OwnerEntityName: "셋인물"},
	}
	selected := prepareTurnDirectEntityMemoryOwners("아버지는 딸 슬아에게 말을 건넸다.", owners)
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
	filterPrepareTurnEntityRecollections("가나다가 찾아왔다.", nil, nil, nil, nil, nil, &items)
	if len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("selected = %#v, want durable high-emotion memory instead of newest row", items)
	}
}

func TestPrepareTurnExplicitRecollectionsDoNotBecomeOffSceneObjectiveState(t *testing.T) {
	const rawInput = "소월, 슬아, 서현까지 떠올려보니 하나같이 예쁘고 참된 여성 같아 자신에게 과분하다고 한얼은 생각했다."
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
	trace := filterPrepareTurnEntityRecollections(rawInput, nil, activeStates, nil, nil, nil, &privateMemories)
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

func TestPrepareTurnDirectWorldEpisodeAndCanonicalRequireCurrentSceneRelevance(t *testing.T) {
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil,
		[]store.DirectEvidence{
			{ID: 1, EvidenceText: "Mira used the brass key at the gate.", TurnAnchor: 9},
			{ID: 2, EvidenceText: "Juno left a passport at the harbor.", TurnAnchor: 10},
		},
		nil, nil,
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
	for _, text := range []string{assembly.LatestDirectEvidenceText, assembly.ScopedVerbatimText, assembly.WorldRulesText, assembly.CanonText, assembly.EpisodeText} {
		if strings.Contains(text, "Juno") || strings.Contains(text, "passport") || strings.Contains(text, "harbor") {
			t.Fatalf("unrelated latest/support material survived current-scene gate: %q", text)
		}
	}
	checks := map[string]string{
		"Mira used the brass key":       assembly.LatestDirectEvidenceText,
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
