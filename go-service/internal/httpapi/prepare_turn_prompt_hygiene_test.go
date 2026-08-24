package httpapi

import (
	"context"
	"errors"
	"fmt"
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
		"memory_search_result": "ok",
		"search_result":        "ok",
		"search_results": []map[string]any{
			{"source_table": "memories", "source_row_id": "5", "similarity": 0.99},
			{"source_table": "memories", "source_row_id": "4", "similarity": 0.98},
			{"source_table": "memories", "source_row_id": "1", "similarity": 0.90},
			{"source_table": "memories", "source_row_id": "2", "similarity": 0.89},
			{"source_table": "memories", "source_row_id": "3", "similarity": 0.88},
		},
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, activeStates)
	perspective[prepareTurnEntityIdentityAliasesContextKey] = map[string]any{"소월": "이소월", "슬아": "윤슬아", "서현": "민서현"}
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
		nil, nil, chatLogs,
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
		"world rules":     assembly.WorldRulesText,
		"canonical world": assembly.CanonWorldText,
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
	if assembly.CharacterRelationshipText != "" || assembly.CanonRelationshipText != "" {
		t.Fatalf("stale current-state relationship lane survived: character=%q canonical=%q",
			assembly.CharacterRelationshipText, assembly.CanonRelationshipText)
	}
	if strings.Contains(assembly.KGText, "Han-eol --demonstrated_to--> Bae") {
		t.Fatalf("single-endpoint historical KG edge survived without current relation evidence: %q", assembly.KGText)
	}
	if got := intFromAny(assembly.Counts["kg_single_endpoint_only_dropped"], 0); got != 1 {
		t.Fatalf("kg_single_endpoint_only_dropped=%d, want 1; counts=%#v", got, assembly.Counts)
	}
	if strings.Contains(assembly.CanonWorldText, "old workshop") || strings.Contains(assembly.CanonWorldText, "old bellows") {
		t.Fatalf("stale volatile world state survived: %q", assembly.CanonWorldText)
	}
	if boolFromAny(assembly.Counts["current_scene_state_is_current"]) {
		t.Fatalf("stale scene was marked current: %#v", assembly.Counts)
	}
}

func TestPrepareTurnVectorExactAndLexicalCandidatesCoexistWithoutRecentFill(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 38, SummaryJSON: `{"turn_summary":"Mira studies signal repairs around harbor before tonight."}`},
		{ID: 2, TurnIndex: 7, SummaryJSON: `{"turn_summary":"The harbor signal failed at dawn."}`},
		{ID: 3, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Tonight Mira checks gate machinery near harbor."}`},
		{ID: 4, TurnIndex: 99, SummaryJSON: `{"turn_summary":"Rowan waters roses while rain crosses the distant garden."}`},
	}
	vectorShadow := map[string]any{
		"memory_search_attempted": true,
		"memory_search_result":    "ok",
		"search_attempted":        true,
		"search_result":           "ok",
		"search_result_count":     1,
		"search_results": []map[string]any{
			{"source_table": "memories", "source_row_id": "1", "similarity": 0.8},
		},
	}
	selection := selectPrepareTurnMemoryLanesWithVector(
		memories,
		"Mira opens harbor signal gate tonight.",
		5,
		vectorShadow,
		[]string{"Mira"},
		nil,
	)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 1 {
		t.Fatalf("vector result was not retained: %#v", selection.VectorRelevant)
	}
	if len(selection.Relevant) != 2 || selection.Relevant[0].ID != 2 || selection.Relevant[1].ID != 3 {
		t.Fatalf("exact and distinct lexical results did not coexist with vector recall: %#v", selection.Relevant)
	}
	if boolFromAny(selection.Trace["general_lexical_refill_skipped_after_vector_success"]) ||
		!boolFromAny(selection.Trace["general_lexical_evaluated_with_vector_success"]) {
		t.Fatalf("lexical evaluation was still skipped after vector success: %#v", selection.Trace)
	}
	if got := intFromAny(selection.Trace["exact_phrase_candidate_count"], 0); got != 1 {
		t.Fatalf("exact phrase candidate count=%d, want 1: %#v", got, selection.Trace)
	}
	if got := intFromAny(selection.Trace["exact_phrase_selected_count"], 0); got != 1 {
		t.Fatalf("exact phrase selected count=%d, want 1: %#v", got, selection.Trace)
	}
	if got := intFromAny(selection.Trace["lexical_candidate_count"], 0); got != 2 {
		t.Fatalf("lexical candidate count=%d, want 2: %#v", got, selection.Trace)
	}
	if got := intFromAny(selection.Trace["lexical_selected_count"], 0); got != 1 {
		t.Fatalf("lexical selected count=%d, want 1 after vector dedupe: %#v", got, selection.Trace)
	}
	selectedIDs := map[int64]int{}
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, item := range lane {
			selectedIDs[item.ID]++
		}
	}
	if selectedIDs[1] != 1 {
		t.Fatalf("vector/lexical duplicate was repeated: counts=%#v selection=%#v", selectedIDs, selection)
	}
	if selectedIDs[4] != 0 || len(selection.Recent) != 0 {
		t.Fatalf("unrelated recent memory filled a query-time slot: counts=%#v recent=%#v", selectedIDs, selection.Recent)
	}
}

type prepareTurnRetrievalFailureStore struct {
	*narrativeFakeStore
	memoryErr  error
	kgErr      error
	pendingErr error
}

func (s *prepareTurnRetrievalFailureStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.Memory, error) {
	if s.memoryErr != nil {
		return nil, s.memoryErr
	}
	return s.narrativeFakeStore.ListMemories(ctx, chatSessionID, fromTurn, toTurn)
}

func (s *prepareTurnRetrievalFailureStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]store.KGTriple, error) {
	if s.kgErr != nil {
		return nil, s.kgErr
	}
	return s.narrativeFakeStore.ListKGTriples(ctx, chatSessionID)
}

func (s *prepareTurnRetrievalFailureStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]store.PendingThread, error) {
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	return s.narrativeFakeStore.ListPendingThreads(ctx, chatSessionID, status)
}

func TestPrepareTurnProductionCountsReportRetrievalFailuresSeparately(t *testing.T) {
	base := &narrativeFakeStore{
		characterStates: []store.CharacterState{
			{CharacterName: "Mira", TurnIndex: 8, RelationshipsJSON: `{"Rowan":{"summary":"Mira maintains harbor trust with Rowan"}}`},
			{CharacterName: "Rowan", TurnIndex: 8},
		},
		activeStates: []store.ActiveState{{
			StateType: "scene", TurnIndex: 8,
			Content: `{"location":"harbor","present_entities":["Mira","Rowan"]}`,
		}},
		canonicalStateLayers: []store.CanonicalStateLayer{{
			LayerType: "relationship_state", TurnIndex: 8,
			Content:    `{"pair":["Mira","Rowan"],"bond_and_distance":"harbor trust"}`,
			Confidence: 0.9,
		}},
	}
	srv := setupTestServer()
	srv.Store = &prepareTurnRetrievalFailureStore{
		narrativeFakeStore: base,
		memoryErr:          errors.New("memory read unavailable"),
		kgErr:              errors.New("kg read unavailable"),
		pendingErr:         errors.New("pending thread read unavailable"),
	}
	srv.Vector = &fakeVectorStore{healthErr: errors.New("vector health unavailable")}

	_, response := prepareTurnPerfRequest(t, srv, `{
		"chat_session_id":"retrieval-method-status",
		"turn_index":9,
		"raw_user_input":"Mira asks Rowan whether their harbor trust remains intact.",
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"guide_strength":"none","injection_enabled":true,"input_context_enabled":false,"max_injection_chars":9000,"top_k":3}
	}`)
	pack := mapFromAny(response["injection_pack"])
	counts := mapFromAny(pack["counts"])
	methods := mapFromAny(counts["retrieval_methods"])
	if len(methods) != 5 {
		t.Fatalf("compact production retrieval method status missing: %#v", pack)
	}
	for _, name := range []string{"exact_phrase", "lexical"} {
		status := mapFromAny(methods[name])
		if status["status"] != "failed" || status["reason_code"] != "source_read_failed" {
			t.Fatalf("%s memory-read failure not separated: %#v", name, status)
		}
	}
	if status := mapFromAny(methods["vector"]); status["status"] != "failed" || status["reason_code"] != "readiness_or_embedding_failed" {
		t.Fatalf("vector failure not separated: %#v", status)
	}
	relationship := mapFromAny(methods["relationship"])
	if relationship["status"] != "partial" || !stringSliceContains(stringsFromAny(relationship["failed_sources"]), "kg") {
		t.Fatalf("relationship source failure not separated from retained results: %#v", relationship)
	}
	if intFromAny(relationship["selected_count"], 0) == 0 {
		t.Fatalf("successful relationship sources were erased by KG failure: %#v", relationship)
	}
	if status := mapFromAny(methods["unresolved_thread"]); status["status"] != "failed" || status["reason_code"] != "source_read_failed" {
		t.Fatalf("unresolved-thread failure not separated: %#v", status)
	}
	finalText := extractionStringFromAny(mapFromAny(pack["memory_delivery_plan"])["final_text"])
	if !strings.Contains(finalText, "harbor trust") {
		t.Fatalf("relationship delivery was erased by another retrieval failure: %q", finalText)
	}
}

func TestPrepareTurnProductionCountsRemainObservableWithoutAssembly(t *testing.T) {
	t.Run("store unavailable", func(t *testing.T) {
		srv := setupTestServer()
		srv.Store = nil
		srv.Vector = &fakeVectorStore{healthErr: errors.New("vector health unavailable")}

		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"retrieval-method-store-unavailable",
			"turn_index":1,
			"raw_user_input":"Mira checks the harbor signal.",
			"response_projection":"prepare_turn.production_compact.v1",
			"settings":{"guide_strength":"none","injection_enabled":true,"input_context_enabled":false,"max_injection_chars":9000,"top_k":3}
		}`)
		methods := mapFromAny(mapFromAny(mapFromAny(response["injection_pack"])["counts"])["retrieval_methods"])
		if len(methods) != 5 {
			t.Fatalf("store-unavailable retrieval status missing: %#v", response["injection_pack"])
		}
		for _, name := range []string{"exact_phrase", "lexical", "relationship", "unresolved_thread"} {
			status := mapFromAny(methods[name])
			if status["status"] != "unavailable" || status["reason_code"] != "store_unavailable" {
				t.Fatalf("%s store-unavailable status=%#v", name, status)
			}
		}
		if status := mapFromAny(methods["vector"]); status["status"] != "failed" || status["reason_code"] != "readiness_or_embedding_failed" {
			t.Fatalf("vector failure disappeared without assembly: %#v", status)
		}
	})

	t.Run("injection disabled", func(t *testing.T) {
		srv := setupTestServer()
		_, response := prepareTurnPerfRequest(t, srv, `{
			"chat_session_id":"retrieval-method-injection-disabled",
			"turn_index":1,
			"raw_user_input":"Mira checks the harbor signal.",
			"response_projection":"prepare_turn.production_compact.v1",
			"settings":{"guide_strength":"none","injection_enabled":false,"input_context_enabled":false,"max_injection_chars":9000,"top_k":3}
		}`)
		methods := mapFromAny(mapFromAny(mapFromAny(response["injection_pack"])["counts"])["retrieval_methods"])
		for _, name := range []string{"exact_phrase", "lexical", "relationship", "unresolved_thread"} {
			status := mapFromAny(methods[name])
			if status["status"] != "skipped" || status["reason_code"] != "injection_disabled" {
				t.Fatalf("%s injection-disabled status=%#v", name, status)
			}
		}
	})
}

func TestPrepareTurnVectorRetrievalStatusReportsSourceRevisionCheckFailure(t *testing.T) {
	shadow := map[string]any{
		"memory_search_result":       "ok",
		"memory_search_result_count": 2,
		"memory_source_revision_filter": map[string]any{
			"dropped_check_error": 1,
		},
	}
	partial := prepareTurnVectorRetrievalMethodStatus(shadow, 1)
	if partial["status"] != "partial" || partial["reason_code"] != "source_revision_check_failed" {
		t.Fatalf("retained hit hid source-revision check failure: %#v", partial)
	}
	failed := prepareTurnVectorRetrievalMethodStatus(shadow, 0)
	if failed["status"] != "failed" || failed["reason_code"] != "source_revision_check_failed" {
		t.Fatalf("dropped hits were reported as an empty search: %#v", failed)
	}
}

func TestPrepareTurnAggregateMemoryVectorUsesMemorySearchOwner(t *testing.T) {
	memories := []store.Memory{{
		ID:          41,
		TurnIndex:   4,
		SummaryJSON: `{"turn_summary":"aggregate event marker"}`,
	}}
	memoryHits := []map[string]any{{
		"id":                "memory:session:41",
		"source_table":      "memories",
		"source_row_id":     "41",
		"similarity":        prepareTurnMinCosineSimilarity + 0.1,
		"similarity_source": "cosine_from_query_and_stored_embedding",
	}}

	t.Run("memory success is not erased by broad failure", func(t *testing.T) {
		shadow := map[string]any{
			"search_attempted":           true,
			"search_result":              "error",
			"search_error":               "broad search failed",
			"search_results":             []map[string]any{},
			"memory_search_attempted":    true,
			"memory_search_result":       "ok",
			"memory_search_result_count": len(memoryHits),
			"memory_search_results":      memoryHits,
		}
		hydrated := prepareTurnHydrateVectorMemoryHits(memories, shadow, 1)
		if len(hydrated.Items) != 1 || hydrated.Items[0].ID != memories[0].ID {
			t.Fatalf("memory-owned hit was erased by broad status: %#v", hydrated)
		}
		if !prepareTurnVectorSearchAttempted(shadow) {
			t.Fatalf("memory search attempt was not reported: %#v", shadow)
		}
		method := prepareTurnVectorRetrievalMethodStatus(shadow, len(hydrated.Items))
		if method["status"] != "ready" || intFromAny(method["candidate_count"], 0) != len(memoryHits) {
			t.Fatalf("memory-owned retrieval status mismatch: %#v", method)
		}
	})

	t.Run("memory failure is not hidden by broad success", func(t *testing.T) {
		shadow := map[string]any{
			"search_attempted":           true,
			"search_result":              "ok",
			"search_result_count":        1,
			"search_results":             memoryHits,
			"memory_search_attempted":    true,
			"memory_search_result":       "error",
			"memory_search_error":        "memory search failed",
			"memory_search_result_count": 0,
			"memory_search_results":      []map[string]any{},
		}
		hydrated := prepareTurnHydrateVectorMemoryHits(memories, shadow, 1)
		if len(hydrated.Items) != 0 || hydrated.Trace["status"] != "skipped" || hydrated.Trace["reason"] != "error" {
			t.Fatalf("broad status hid memory search failure: %#v", hydrated)
		}
		method := prepareTurnVectorRetrievalMethodStatus(shadow, 0)
		if method["status"] != "failed" || method["reason_code"] != "search_failed" || intFromAny(method["candidate_count"], -1) != 0 {
			t.Fatalf("memory failure was reported from broad status: %#v", method)
		}
	})
}

func TestPrepareTurnSemanticVectorMemoriesReachEventDeliveryWithoutLexicalProof(t *testing.T) {
	sessionID := "session-" + strings.ToLower(t.Name())
	markers := []string{
		"amber_cascade_" + strings.ToLower(t.Name()),
		"silver_orbit_" + strings.ToLower(t.Name()),
	}
	wrongEntityMarker := "explicit_wrong_entity_" + strings.ToLower(t.Name())
	unrelatedRecentMarker := "unrelated_recent_" + strings.ToLower(t.Name())
	memories := []store.Memory{
		{ID: 51, ChatSessionID: sessionID, TurnIndex: 2, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": markers[0]})},
		{ID: 52, ChatSessionID: sessionID, TurnIndex: 3, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": markers[1]})},
		{ID: 53, ChatSessionID: sessionID, TurnIndex: 4, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": wrongEntityMarker, "characters": []any{"Entity Beta"}})},
		{ID: 54, ChatSessionID: sessionID, TurnIndex: 5, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": unrelatedRecentMarker})},
	}
	semanticHitScore := prepareTurnMinCosineSimilarity + (1-prepareTurnMinCosineSimilarity)/2
	hits := []map[string]any{
		{"id": "memory:" + sessionID + ":51", "source_table": "memories", "source_row_id": "51", "similarity": semanticHitScore, "similarity_source": "cosine_from_query_and_stored_embedding"},
		{"id": "memory:" + sessionID + ":52", "source_table": "memories", "source_row_id": "52", "similarity": semanticHitScore, "similarity_source": "cosine_from_query_and_stored_embedding"},
		{"id": "memory:" + sessionID + ":53", "source_table": "memories", "source_row_id": "53", "similarity": semanticHitScore, "similarity_source": "cosine_from_query_and_stored_embedding"},
	}
	vectorShadow := map[string]any{
		"memory_search_attempted":    true,
		"memory_search_result":       "ok",
		"memory_search_result_count": len(hits),
		"memory_search_results":      hits,
	}
	rawInput := "Entity Alpha asks about the violet horizon."
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil,
		[]store.CharacterState{{ChatSessionID: sessionID, CharacterName: "Entity Alpha", TurnIndex: 3}},
		nil, nil, nil, nil, nil, nil,
		len(hits), 9000, rawInput, "default", nil, vectorShadow, nil,
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	for _, marker := range markers {
		if !strings.Contains(assembly.ActualMemoryText, marker) || !strings.Contains(finalText, marker) {
			t.Fatalf("semantic vector memory did not reach Event delivery: marker=%q actual=%q final=%q trace=%#v", marker, assembly.ActualMemoryText, finalText, assembly.Counts)
		}
	}
	if strings.Contains(finalText, wrongEntityMarker) {
		t.Fatalf("explicit wrong-entity memory crossed the retained entity boundary: %q", finalText)
	}
	if strings.Contains(finalText, unrelatedRecentMarker) {
		t.Fatalf("unrelated recent memory filled an unused query-time slot: %q", finalText)
	}

	payloadPlan := buildPrepareTurnPayloadApplicationPlan(
		rawInput, "", finalText, "", true, false,
		9000, 0, 0, nil, "skipped",
	)
	longTermMemory := map[string]any(nil)
	for _, rawLane := range outputFidelityLineageSlice(payloadPlan["lanes"]) {
		lane := mapFromAny(rawLane)
		if extractionStringFromAny(lane["key"]) == "long_term_memory" {
			longTermMemory = lane
			break
		}
	}
	if !boolFromAny(longTermMemory["applied"]) || extractionStringFromAny(longTermMemory["text"]) != finalText {
		t.Fatalf("Event delivery did not reach the exact payload lane: lane=%#v final=%q", longTermMemory, finalText)
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

func TestPrepareTurnSeparatesObservedWorldStateAgeAndCollapsesOnlyExactSelectedRuleCopies(t *testing.T) {
	const sessionID = "sess-world-state-age"
	const exactRuleValue = "The dusk bells remain silent."
	const changedRuleValue = "The dusk bells once rang."
	const otherSessionValue = "The archive seal remains blue."
	const differentKeyValue = "Moon Hall closes at midnight."
	worldRules := []store.WorldRule{
		{
			ChatSessionID: sessionID,
			Scope:         "root",
			Category:      "custom",
			Key:           "dusk_bells",
			ValueJSON:     mustCompactJSON(exactRuleValue),
			Pinned:        true,
		},
		{
			ChatSessionID: sessionID,
			Scope:         "root",
			Category:      "custom",
			Key:           "archive_seal",
			ValueJSON:     mustCompactJSON(otherSessionValue),
			Pinned:        true,
		},
		{
			ChatSessionID: sessionID,
			Scope:         "root",
			Category:      "custom",
			Key:           "Moon_Hall_Hours",
			ValueJSON:     mustCompactJSON(differentKeyValue),
			Pinned:        true,
		},
	}
	canonical := []store.CanonicalStateLayer{
		{
			ChatSessionID: sessionID,
			LayerType:     "world_state",
			TurnIndex:     4,
			SourceTurn:    4,
			Content: mustCompactJSON(map[string]any{
				"current_location": "Moon Hall observatory chamber",
				"rules": []any{
					map[string]any{
						"scope": "root", "category": "custom", "key": "dusk_bells", "value": exactRuleValue,
					},
					map[string]any{
						"scope": "root", "category": "custom", "key": "moon_hall_hours", "value": differentKeyValue,
					},
				},
			}),
			Confidence: 0.9,
		},
		{
			ChatSessionID: sessionID,
			LayerType:     "world_state",
			TurnIndex:     3,
			SourceTurn:    3,
			Content: mustCompactJSON(map[string]any{
				"current_location": "Old Library",
				"rules": []any{map[string]any{
					"scope": "root", "category": "custom", "key": "dusk_bells", "value": changedRuleValue,
				}},
			}),
			Confidence: 0.9,
		},
		{
			ChatSessionID: "other-session",
			LayerType:     "world_state",
			TurnIndex:     2,
			SourceTurn:    2,
			Content: mustCompactJSON(map[string]any{
				"observation": "Unbound archive seal observation",
				"rules": []any{map[string]any{
					"scope": "root", "category": "custom", "key": "archive_seal", "value": otherSessionValue,
				}},
			}),
			Confidence: 0.9,
		},
		{
			ChatSessionID: sessionID, LayerType: "scene_state", TurnIndex: 4, SourceTurn: 4,
			Content: `{"location":"Moon Hall observatory chamber","present_entities":["Mira"]}`, Confidence: 0.9,
		},
		{
			ChatSessionID: sessionID, LayerType: "scene_state", TurnIndex: 3, SourceTurn: 3,
			Content: `{"location":"Old Library","present_entities":["Mira"]}`, Confidence: 0.9,
		},
		{
			ChatSessionID: sessionID, LayerType: "entity_state", TurnIndex: 4, SourceTurn: 4,
			Content: `{"characters":[{"name":"Mira","location":"Moon Hall observatory chamber"}]}`, Confidence: 0.9,
		},
		{
			ChatSessionID: sessionID, LayerType: "entity_state", TurnIndex: 3, SourceTurn: 3,
			Content: `{"characters":[{"name":"Mira","location":"Old Library"}]}`, Confidence: 0.9,
		},
	}
	originalRuleValues := make([]string, len(worldRules))
	for index := range worldRules {
		originalRuleValues[index] = worldRules[index].ValueJSON
	}
	originalCanonicalContent := make([]string, len(canonical))
	for index := range canonical {
		originalCanonicalContent[index] = canonical[index].Content
	}
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", TurnIndex: 4, Content: `{"location":"Moon Hall observatory chamber","present_entities":["Mira"]}`,
	}})

	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil,
		[]store.ChatLog{{ChatSessionID: sessionID, TurnIndex: 4, Role: "assistant", Content: "Mira entered the Moon Hall observatory chamber after leaving the Old Library."}},
		nil, worldRules, nil, nil, canonical,
		nil, nil, nil, nil,
		5, 12000,
		"Mira compares the Moon Hall observatory chamber with the Old Library, the dusk bells, and the archive seal observation.",
		"default", nil, nil, nil, perspective,
	)

	if !strings.Contains(assembly.CanonWorldText, "world_state [latest_observed turn=4]") ||
		!strings.Contains(assembly.CanonWorldText, "world_state [historical turn=3]") ||
		!strings.Contains(assembly.CanonWorldText, "Moon Hall") ||
		!strings.Contains(assembly.CanonWorldText, "Old Library") {
		t.Fatalf("latest and historical world states were not both labeled and retained: %q", assembly.CanonWorldText)
	}
	if !strings.Contains(assembly.CanonWorldText, "scene_state [latest_observed turn=4]") ||
		strings.Contains(assembly.CanonWorldText, "scene_state [historical turn=3]") {
		t.Fatalf("existing scene-state currentness changed unexpectedly: %q", assembly.CanonWorldText)
	}
	if !strings.Contains(assembly.CanonCharacterText, "Moon Hall") || strings.Contains(assembly.CanonCharacterText, "Old Library") {
		t.Fatalf("existing entity-state currentness changed unexpectedly: %q", assembly.CanonCharacterText)
	}
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if got := strings.Count(finalText, exactRuleValue); got != 1 {
		t.Fatalf("exact selected world-rule copy count=%d, want 1: %q", got, finalText)
	}
	if got := strings.Count(finalText, changedRuleValue); got != 1 {
		t.Fatalf("same-key different-value historical rule count=%d, want 1: %q", got, finalText)
	}
	if got := strings.Count(finalText, otherSessionValue); got != 2 {
		t.Fatalf("different-session rule copy count=%d, want 2 distinct sources retained: %q", got, finalText)
	}
	if got := strings.Count(finalText, differentKeyValue); got != 2 {
		t.Fatalf("different-key rule copy count=%d, want 2 distinct keys retained: %q", got, finalText)
	}
	for index := range worldRules {
		if worldRules[index].ValueJSON != originalRuleValues[index] {
			t.Fatalf("world-rule input mutated at %d: got %q want %q", index, worldRules[index].ValueJSON, originalRuleValues[index])
		}
	}
	for index := range canonical {
		if canonical[index].Content != originalCanonicalContent[index] {
			t.Fatalf("canonical input mutated at %d: got %q want %q", index, canonical[index].Content, originalCanonicalContent[index])
		}
	}
}

func TestPrepareTurnRecollectionContextDoesNotPreCutRelevantContextByCountOrFieldLength(t *testing.T) {
	memories := []store.Memory{}
	for index := 0; index < 4; index++ {
		memories = append(memories, store.Memory{
			TurnIndex:   20,
			SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": fmt.Sprintf("archive clue %d", index)}),
		})
	}
	longMemory := strings.Repeat("memory-padding ", 30) + "deep anchor memory-tail-marker"
	memories = append(memories, store.Memory{
		TurnIndex:   20,
		SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": longMemory}),
	})

	pending := []store.PendingThread{}
	for index := 0; index < 10; index++ {
		description := fmt.Sprintf("archive goal %d", index)
		if index == 9 {
			description += " " + strings.Repeat("goal-padding ", 30) + "goal-tail-marker"
		}
		pending = append(pending, store.PendingThread{Status: "open", Description: description})
	}
	longScene := strings.Repeat("scene-padding ", 40) + "state-tail-marker"
	active := []store.ActiveState{{
		StateType: "scene",
		TurnIndex: 20,
		Content: mustCompactJSON(map[string]any{
			"present_entities": []string{"Mira"},
			"description":      longScene,
		}),
	}}
	longAssistant := strings.Repeat("assistant-padding ", 35) + "assistant-tail-marker"
	ctx := buildPrepareTurnRecollectionContext(
		"archive clue deep anchor goal memory-tail-marker goal-tail-marker state-tail-marker",
		memories,
		active,
		nil,
		pending,
		[]store.ChatLog{{Role: "assistant", TurnIndex: 20, Content: longAssistant}},
	)

	if got := len(nonEmptyStrings(strings.Split(ctx.previousEventSummary, "\n"))); got != 5 {
		t.Fatalf("previous event context count = %d, want all 5 relevant summaries: %q", got, ctx.previousEventSummary)
	}
	for label, text := range map[string]string{
		"memory":    ctx.previousEventSummary,
		"goal":      ctx.unresolvedGoals,
		"scene":     ctx.currentSceneStates,
		"assistant": ctx.currentAssistantContext,
	} {
		marker := map[string]string{
			"memory": "memory-tail-marker", "goal": "goal-tail-marker",
			"scene": "state-tail-marker", "assistant": "assistant-tail-marker",
		}[label]
		if !strings.Contains(text, marker) || strings.Contains(text, "...") {
			t.Fatalf("%s relevance context was pre-truncated: %q", label, text)
		}
	}
	if got := strings.Count(ctx.unresolvedGoals, "archive goal "); got != 10 {
		t.Fatalf("unresolved goal context count = %d, want all 10 relevant goals: %q", got, ctx.unresolvedGoals)
	}
}
