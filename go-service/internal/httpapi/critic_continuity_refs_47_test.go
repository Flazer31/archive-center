package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test47CriticSelectedMemoryKeepsRecordedStateAndPromiseIdentity(t *testing.T) {
	const raw = `{"turn_summary":"Mira holds the brass compass and promises to return it.","state_claims":[{"subject":"Brass Compass","state_slot":"보관","value":"Mira","transition":"set","evidence_excerpt":"Mira holds the brass compass."}],"pending_threads":[{"title":"Return brass compass","lifecycle_key":"return-compass-1","status":"open","description":"Mira promises to return the brass compass.","evidence_excerpt":"Mira promises to return the brass compass."}]}`
	st := &turnRecordingStore{
		returnMemories: []store.Memory{{ID: 1, TurnIndex: 1, SummaryJSON: raw, Importance: .8}},
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "refs47", TurnIndex: 1, Role: "user", Content: "Mira takes the brass compass."},
			{ChatSessionID: "refs47", TurnIndex: 1, Role: "assistant", Content: "Mira holds the brass compass. Mira promises to return the brass compass."},
		},
	}
	s := NewServer(config.Default()) // Ledger switch remains off.
	s.Store = st
	_, memories, _ := s.buildCompleteTurnCriticCanonicalContext(context.Background(), "refs47", 2, "Mira returns the brass compass.", 0)
	if len(memories) != 1 {
		t.Fatalf("selected memory count=%d", len(memories))
	}
	state := sliceFromAny(memories[0]["recorded_state_claims"])
	threads := sliceFromAny(memories[0]["recorded_pending_threads"])
	if len(state) != 1 || stringFromMap(mapFromAny(state[0]), "state_slot") != "보관" || len(threads) != 1 || stringFromMap(mapFromAny(threads[0]), "lifecycle_key") != "return-compass-1" {
		t.Fatalf("identity was discarded before Critic could reuse it: %#v", memories)
	}
	if !boolFromAny(memories[0]["support_only"]) || intFromAny(memories[0]["turn_index"], 0) != 1 {
		t.Fatal("historical reference lost support-only source attribution")
	}
	if strings.Contains(mustCompactJSON(memories), "evidence_excerpt") {
		t.Fatal("identity support unnecessarily duplicated raw evidence")
	}
	if st.returnMemories[0].SummaryJSON != raw {
		t.Fatal("support projection mutated stored history")
	}
}

func TestCriticBudgetKeepsRelatedLifecycleReferencesWithoutWholeMemory(t *testing.T) {
	const shield = "silver-shield-receipt-1"
	const lighthouse = "harbor-lighthouse-restoration-1"
	memory := map[string]any{
		"source": "mariadb_memory", "id": 1, "turn_index": 1, "support_only": true,
		"summary": "미라는 은빛 방패 수령과 항구 등대 복구를 약속했다. " + strings.Repeat("대장간과 항구에서 나눈 대화의 상세 기록. ", 80),
		"recorded_state_claims": []any{
			map[string]any{"subject": "은빛 방패 수령", "state_slot": "goal_status", "lifecycle_key": shield, "transition": "create", "value": "미라는 완성된 은빛 방패를 내일 수령하기로 약속했다."},
			map[string]any{"subject": "항구 등대 복구", "state_slot": "goal_status", "lifecycle_key": lighthouse, "transition": "create", "value": "미라는 렌즈를 구하여 항구 등대의 불을 복구하기로 약속했다."},
		},
		"recorded_pending_threads": []any{
			map[string]any{"title": "은빛 방패 수령", "lifecycle_key": shield, "status": "open", "description": "완성된 방패를 수령한다."},
			map[string]any{"title": "항구 등대 복구", "lifecycle_key": lighthouse, "status": "open", "description": "렌즈를 구하여 등대를 복구한다."},
		},
	}
	original := mustCompactJSON(memory)
	ledger := map[string]any{"character_names": []any{
		map[string]any{"entity_id": "smith", "name": "대장장이", "aliases": []string{}, "identity_namespace": "session_npc"},
		map[string]any{"entity_id": "mira", "name": "미라", "aliases": []string{}, "identity_namespace": "session_npc"},
	}}
	query := "미라는 은빛 방패 수령을 마쳤다. 항구 등대 복구는 렌즈가 없어 아직 미완료다."
	for _, budget := range []int{0, 800, 10000} {
		_, selected, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, []map[string]any{memory}, ledger, nil, query,
			completeTurnCriticInputPolicy{AuxiliaryMaxChars: budget, ConfiguredChars: budget})
		if intFromAny(trace["auxiliary_selected_chars"], -1) > budget {
			t.Fatalf("budget %d exceeded: %#v", budget, trace)
		}
		cards := criticReferenceCardsForTest(t, selected)
		if budget == 0 {
			if len(cards) != 0 {
				t.Fatal("zero budget admitted references")
			}
			continue
		}
		if len(cards) != 2 {
			t.Fatalf("budget %d lost or duplicated a reference identity: %#v", budget, selected)
		}
		encoded := mustCompactJSON(selected)
		for _, key := range []string{shield, lighthouse} {
			if !strings.Contains(encoded, key) {
				t.Fatalf("budget %d discarded existing lifecycle key %s: %#v", budget, key, trace)
			}
		}
		for _, card := range cards {
			if intFromAny(card["source_turn"], 0) != 1 || intFromAny(card["memory_id"], 0) != 1 {
				t.Fatal("reference lost provenance")
			}
		}
		if budget == 800 && strings.Contains(encoded, "상세 기록") {
			t.Fatal("oversized body was smuggled into identity references")
		}
		if budget == 10000 {
			if strings.Contains(encoded, "상세 기록") {
				t.Fatal("a larger budget reintroduced the broad summary beside its cards")
			}
		}
	}
	if mustCompactJSON(memory) != original {
		t.Fatal("budget projection mutated the source memory")
	}
}

func TestCriticReferenceCandidatesKeepDistinctKeysAndRejectUnrelatedReferences(t *testing.T) {
	memory := map[string]any{
		"id": 42, "turn_index": 3, "source": "mariadb_memory", "support_only": true,
		"summary": strings.Repeat("Shield workshop notes and unrelated household details. ", 100),
		"recorded_pending_threads": []any{
			map[string]any{"title": "Collect shield", "lifecycle_key": "shield-for-mira", "status": "open"},
			map[string]any{"title": "Collect shield", "lifecycle_key": "shield-for-rowan", "status": "open"},
			map[string]any{"title": "Bake birthday cake", "lifecycle_key": "birthday-cake", "status": "open"},
		},
	}
	_, selected, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, []map[string]any{memory}, nil, nil,
		"Collect shield from the workshop", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 800, ConfiguredChars: 800})
	threads := criticReferenceCardsForTest(t, selected)
	if len(threads) != 2 {
		t.Fatalf("similar titles merged different identities, or unrelated reference was admitted: %#v", threads)
	}
	text := mustCompactJSON(threads)
	if !strings.Contains(text, "shield-for-mira") || !strings.Contains(text, "shield-for-rowan") || strings.Contains(text, "birthday-cake") {
		t.Fatalf("wrong identities selected: %s", text)
	}
	if intFromAny(trace["auxiliary_selected_chars"], 0) > 800 {
		t.Fatal("budget exceeded")
	}
}
