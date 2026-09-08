package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestPrepareTurnCharacterPrivateRecollectionLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-npc-loop", TurnIndex: 1, Role: "user", Content: "Siwoo enters the kitchen."},
			{ID: 2, ChatSessionID: "target-npc-loop", TurnIndex: 1, Role: "assistant", Content: "Chloe quietly watches him."},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  21,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-npc-loop",
				SourceTurn:          5,
				MemoryText:          "Chloe remembers from a previous loop that Siwoo avoided the broken bridge.",
				SecretGuard:         true,
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				TagsJSON:            `["loop","npc_private"]`,
				Importance10:        9,
				EmotionalWeight:     0.8,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-npc-loop","turn_index":2,"raw_user_input":"He asks Chloe whether they should avoid the broken bridge.","settings":{"max_injection_chars":1400,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Subjective Memories and Relationships]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("injection_text missing character private recollection: %q", injectionText)
	}
	if strings.Contains(injectionText, "support-only private recollection") {
		t.Fatalf("NPC private recollection leaked into persona lane: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Chloe remembers from a previous loop that Siwoo avoided the broken bridge.") {
		t.Fatalf("NPC private recollection omitted exact protected memory: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Protected NPC-private hint") {
		t.Fatalf("NPC private recollection missing protected hint wording: %q", injectionText)
	}
	inputContextText, _ := resp["input_context_text"].(string)
	if strings.Contains(inputContextText, "[Subjective Memories and Relationships]") || strings.Contains(inputContextText, "not player knowledge") {
		t.Fatalf("NPC private recollection bypassed its dedicated injection lane: %q", inputContextText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["character_private_recollection_active"] != true {
		t.Fatalf("character private lane inactive: %+v", ip)
	}
	policy, ok := ip["character_private_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection_policy missing: %+v", ip)
	}
	if policy["visible_to_player"] != false || policy["narrator_reveal_blocked"] != true || policy["canonical_write"] != false {
		t.Fatalf("character private policy must block reveal/write: %+v", policy)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["visible_to_player"] != false || surface["narrator_reveal_blocked"] != true {
		t.Fatalf("character private surface mismatch: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn character private recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestPrepareTurnReadsCandidatesThenFiltersUnmatchedPrivateMemory(t *testing.T) {
	fake := &turnRecordingStore{
		returnEntityOwners: []store.ProtagonistEntityMemoryOwner{
			{OwnerEntityKey: "mina", OwnerEntityName: "Mina"},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{ID: 1, OwnerEntityKey: "mina", OwnerEntityName: "Mina", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: "sess-owner-miss", MemoryText: "Mina's unrelated private memory."},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(`{"chat_session_id":"sess-owner-miss","turn_index":4,"raw_user_input":"Rowan checks the empty harbor.","settings":{"injection_enabled":true,"max_injection_chars":9000}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.entityMemoryReadCount != 1 {
		t.Fatalf("private memory candidates were skipped before semantic filtering: count=%d", fake.entityMemoryReadCount)
	}
	if strings.Contains(rec.Body.String(), "Mina's unrelated private memory") {
		t.Fatalf("unmatched private memory reached prepare-turn: %s", rec.Body.String())
	}
}

func TestPrepareTurnReadsPrivateMemoryForOwnerCarriedByPreviousEvent(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-owner-previous-event", TurnIndex: 3, SummaryJSON: `{"turn_summary":"Mina waits with Rowan at the harbor."}`},
		},
		returnEntityOwners: []store.ProtagonistEntityMemoryOwner{
			{OwnerEntityKey: "mina", OwnerEntityName: "Mina"},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{ID: 1, OwnerEntityKey: "mina", OwnerEntityName: "Mina", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: "sess-owner-previous-event", MemoryText: "Mina remembers Rowan's promise at the harbor."},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(`{"chat_session_id":"sess-owner-previous-event","turn_index":4,"raw_user_input":"Rowan checks the harbor.","settings":{"injection_enabled":true,"max_injection_chars":9000}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.entityMemoryReadCount != 1 {
		t.Fatalf("private memory read count=%d, want one read for owner carried by the relevant previous event", fake.entityMemoryReadCount)
	}
	if len(fake.entityMemoryFilters) != 1 || len(fake.entityMemoryFilters[0].OwnerEntityKeys) != 1 || fake.entityMemoryFilters[0].OwnerEntityKeys[0] != "mina" {
		t.Fatalf("owner filter=%#v, want Mina from the relevant previous event", fake.entityMemoryFilters)
	}
}

func TestPrepareTurnBatchesCurrentNPCMemoryOwnersIntoOneRead(t *testing.T) {
	fake := &turnRecordingStore{
		returnEntityOwners: []store.ProtagonistEntityMemoryOwner{
			{OwnerEntityKey: "mina", OwnerEntityName: "Mina"},
			{OwnerEntityKey: "rowan", OwnerEntityName: "Rowan"},
			{OwnerEntityKey: "juno", OwnerEntityName: "Juno"},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{ID: 1, OwnerEntityKey: "mina", OwnerEntityName: "Mina", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: "sess-owner-batch", MemoryText: "Mina remembers Rowan's promise."},
			{ID: 2, OwnerEntityKey: "rowan", OwnerEntityName: "Rowan", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: "sess-owner-batch", MemoryText: "Rowan worries about Mina."},
			{ID: 3, OwnerEntityKey: "juno", OwnerEntityName: "Juno", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: "sess-owner-batch", MemoryText: "Juno watches the harbor."},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-owner-batch","turn_index":4,"raw_user_input":"Mina asks Rowan about the promise.","settings":{"injection_enabled":true,"input_context_enabled":false,"max_injection_chars":9000,"top_k":2}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.entityMemoryReadCount != 1 {
		t.Fatalf("subjective memory read count=%d, want one batched read", fake.entityMemoryReadCount)
	}
	if len(fake.entityMemoryFilters) != 1 || len(fake.entityMemoryFilters[0].OwnerEntityKeys) != 2 {
		t.Fatalf("owner batch filter=%#v, want Mina and Rowan in one filter", fake.entityMemoryFilters)
	}
	keys := strings.Join(fake.entityMemoryFilters[0].OwnerEntityKeys, ",")
	if !strings.Contains(keys, "mina") || !strings.Contains(keys, "rowan") || strings.Contains(keys, "juno") {
		t.Fatalf("unexpected owner batch keys=%q", keys)
	}
}

func TestPrepareTurnHTTPPrioritizesEachDirectlyRecalledCharacterWithoutPromotingOffSceneState(t *testing.T) {
	const sid = "sess-direct-character-coverage"
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: sid, TurnIndex: 12, SummaryJSON: `{"turn_summary":"Ava and Ren shared an honest harbor conversation.","characters":["Ava","Ren"],"locations":["harbor"]}`, Importance: 0.8},
			{ID: 2, ChatSessionID: sid, TurnIndex: 15, SummaryJSON: `{"turn_summary":"Bella gave Ren medicine as a caring gift.","characters":["Bella","Ren"],"items":["medicine"]}`, Importance: 0.8},
			{ID: 3, ChatSessionID: sid, TurnIndex: 40, SummaryJSON: `{"turn_summary":"Cora sent Ren an honest letter about her feelings.","characters":["Cora","Ren"],"items":["letter"]}`, Importance: 0.8},
			{ID: 4, ChatSessionID: sid, TurnIndex: 49, SummaryJSON: `{"turn_summary":"Ren calibrated the lathe flywheel and steel axle.","characters":["Ren"],"items":["lathe","flywheel","steel axle"]}`, Importance: 0.99},
		},
		returnCharStates: []store.CharacterState{
			{CharacterName: "Ren", TurnIndex: 49, StatusJSON: `{"health":"tired","location":"workshop"}`},
			{CharacterName: "Ava", TurnIndex: 12, StatusJSON: `{"health":"well","location":"harbor"}`},
			{CharacterName: "Bella", TurnIndex: 15, StatusJSON: `{"health":"well","location":"clinic"}`},
			{CharacterName: "Cora", TurnIndex: 40, StatusJSON: `{"health":"well","location":"estate"}`},
		},
		returnActiveStates: []store.ActiveState{{
			ChatSessionID: sid,
			StateType:     "scene",
			TurnIndex:     49,
			Content:       `{"location":"workshop","present_entities":["Ren"],"items":["lathe","flywheel"]}`,
		}},
		returnEntityOwners: []store.ProtagonistEntityMemoryOwner{
			{OwnerEntityKey: "ava", OwnerEntityName: "Ava"},
			{OwnerEntityKey: "bella", OwnerEntityName: "Bella"},
			{OwnerEntityKey: "cora", OwnerEntityName: "Cora"},
			{OwnerEntityKey: "dax", OwnerEntityName: "Dax"},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{ID: 11, OwnerEntityKey: "ava", OwnerEntityName: "Ava", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: sid, SourceTurn: 12, MemoryText: "Ava privately remembers the honest harbor conversation.", Importance10: 7},
			{ID: 12, OwnerEntityKey: "bella", OwnerEntityName: "Bella", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: sid, SourceTurn: 15, MemoryText: "Bella privately remembers giving Ren medicine.", Importance10: 7},
			{ID: 13, OwnerEntityKey: "cora", OwnerEntityName: "Cora", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: sid, SourceTurn: 40, MemoryText: "Cora privately remembers sending the honest letter.", Importance10: 7},
			{ID: 14, OwnerEntityKey: "dax", OwnerEntityName: "Dax", OwnerEntityRole: "npc", OwnerVisibility: "owner_private", SourceChatSessionID: sid, SourceTurn: 48, MemoryText: "Dax privately remembers the steel axle.", Importance10: 10},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-direct-character-coverage",
		"turn_index":50,
		"raw_user_input":"Ren remembers Ava's honest harbor conversation, Bella's caring medicine gift, and Cora's honest letter about her feelings.",
		"settings":{"apply_mode":"shadow","max_injection_chars":9000,"injection_enabled":true,"input_context_enabled":false,"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pack := mapFromAny(resp["injection_pack"])
	if pack["final_budget_owner"] != "go_priority_memory_delivery_plan" {
		t.Fatalf("final budget owner=%v, want Go-owned priority delivery plan", pack["final_budget_owner"])
	}
	memoryText, _ := pack["memory_text"].(string)
	for _, want := range []string{"Ava", "Bella", "Cora"} {
		if !strings.Contains(memoryText, want) {
			t.Fatalf("directly recalled character %q lost in HTTP prepare-turn: %q", want, memoryText)
		}
	}
	for _, unwanted := range []string{"lathe", "flywheel", "steel axle"} {
		if strings.Contains(memoryText, unwanted) {
			t.Fatalf("unrelated technical memory %q filled direct-character budget: %q", unwanted, memoryText)
		}
	}
	characterText, _ := pack["character_text"].(string)
	if !strings.Contains(characterText, "Ren") {
		t.Fatalf("observed scene character state was lost: %q", characterText)
	}
	for _, offScene := range []string{"Ava", "Bella", "Cora"} {
		if strings.Contains(characterText, offScene) {
			t.Fatalf("off-scene recalled character %q was promoted to objective state: %q", offScene, characterText)
		}
	}
	plan := mapFromAny(pack["memory_delivery_plan"])
	finalText, _ := plan["final_text"].(string)
	for _, want := range []string{"Ava", "Bella", "Cora"} {
		if !strings.Contains(finalText, want) {
			t.Fatalf("directly recalled character %q was lost after final budget selection: %q", want, finalText)
		}
	}
	for _, unwanted := range []string{"lathe", "flywheel", "steel axle", "Dax"} {
		if strings.Contains(finalText, unwanted) {
			t.Fatalf("unrelated final delivery %q survived: %q", unwanted, finalText)
		}
	}
	if used := intFromAny(plan["used_chars"], -1); used < 0 || used > intFromAny(plan["delivery_cap_chars"], 0) {
		t.Fatalf("final delivery budget mismatch: %#v", plan)
	}
	corePriority := mapFromAny(plan["core_objective_memory"])
	if plan["contract_version"] != "memory_delivery_plan.v2" ||
		corePriority["contract_version"] != "core_priority_memory_delivery.v4" ||
		boolFromAny(plan["unused_k_transfer_between_groups"]) {
		t.Fatalf("independent priority-group/K contract mismatch: %#v", plan)
	}
	for _, rawGroup := range sliceFromAny(corePriority["quota_groups"]) {
		group := mapFromAny(rawGroup)
		if intFromAny(group["core_priority_target"], 0) != intFromAny(group["requested_max_items"], 0) || intFromAny(group["deferred_by_limit_count"], 0) != 0 {
			t.Fatalf("priority group retained the retired count ceiling: %#v", group)
		}
	}
	classText := map[string]string{}
	for _, rawClass := range sliceFromAny(plan["classes"]) {
		class := mapFromAny(rawClass)
		classText[stringFromMap(class, "key")] = stringFromMap(class, "text")
	}
	for _, want := range []string{"Ava", "Bella", "Cora"} {
		if !strings.Contains(classText["event_recent"], want) {
			t.Fatalf("event_recent final class lost %q: %q items=%#v", want, classText["event_recent"], plan["priority_items"])
		}
	}
	if strings.Contains(classText["subjective_relationship"], "Dax") {
		t.Fatalf("unmentioned private-memory owner entered final class: %q", classText["subjective_relationship"])
	}
	for _, offScene := range []string{"Ava", "Bella", "Cora"} {
		if strings.Contains(classText["character_objective"], offScene) {
			t.Fatalf("off-scene objective state %q entered final class: %q", offScene, classText["character_objective"])
		}
	}
}

func TestPrepareTurnCharacterPrivateRecollectionTreatsMisunderstandingAsInterpretation(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-private-conflict", TurnIndex: 2, Role: "user", Content: "Siwoo stays quiet beside Chloe."},
			{ID: 2, ChatSessionID: "target-private-conflict", TurnIndex: 2, Role: "assistant", Content: "Chloe watches his silence and hesitates."},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  41,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-conflict",
				SourceTurn:          9,
				MemoryText:          "misunderstanding: Chloe privately interpreted Siwoo's silence as rejection.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				TagsJSON:            `["npc_private","misunderstanding","conflict"]`,
				Importance10:        7,
				EmotionalWeight:     0.6,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-private-conflict","turn_index":3,"raw_user_input":"Chloe asks why Siwoo went quiet.","settings":{"max_injection_chars":2600,"max_input_context_chars":1400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Subjective Memories and Relationships]") || !strings.Contains(injectionText, "Private interpretation") {
		t.Fatalf("injection_text missing private interpretation lane: %q", injectionText)
	}
	if !strings.Contains(injectionText, "owning NPC's interpretation") || !strings.Contains(injectionText, "do not present it as objective fact") {
		t.Fatalf("injection_text missing interpretation-not-fact guard: %q", injectionText)
	}
	if strings.Contains(injectionText, "support-only private recollection") {
		t.Fatalf("NPC private recollection leaked into persona lane: %q", injectionText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	policy, ok := ip["character_private_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection_policy missing: %+v", ip)
	}
	if policy["interpretation_not_fact"] != true || policy["private_conflict_guard"] != true || policy["narrator_must_not_confirm_private_memory"] != true {
		t.Fatalf("character private policy missing PMC-20 guard fields: %+v", policy)
	}
	allowedExpression, _ := policy["allowed_expression"].([]any)
	hasMisunderstandingExpression := false
	for _, item := range allowedExpression {
		if item == "misunderstanding" {
			hasMisunderstandingExpression = true
			break
		}
	}
	if !hasMisunderstandingExpression {
		t.Fatalf("character private policy must allow misunderstanding as private expression: %+v", policy)
	}
	if policy["truth_authority"] != false || policy["canonical_write"] != false || policy["current_world_fact"] != false {
		t.Fatalf("character private policy must remain support-only: %+v", policy)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["interpretation_not_fact"] != true || surface["private_conflict_guard"] != true || surface["narrator_fact_reveal_blocked"] != true {
		t.Fatalf("character private surface missing PMC-20 guard fields: %+v", surface)
	}
	if surface["visible_to_player"] != false || surface["narrator_reveal_blocked"] != true {
		t.Fatalf("character private surface must block reveal: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn character private recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}

func TestPrepareTurnEntityRecollectionRelevanceFiltersUnrelatedNPCMemory(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-pmc19", TurnIndex: 3, Role: "user", Content: "Siwoo reaches the second exit corridor."},
			{ID: 2, ChatSessionID: "target-pmc19", TurnIndex: 3, Role: "assistant", Content: "Chloe stands beside him and studies the flickering sign."},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "target-pmc19", StateType: "scene", Content: `{"location":"second exit corridor","present_entities":["Siwoo","Chloe"]}`, TurnIndex: 3},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  31,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-pmc19",
				SourceTurn:          2,
				MemoryText:          "Chloe privately remembers that Siwoo noticed the exit sign changed.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
			{
				ID:                  32,
				OwnerEntityKey:      "saori",
				OwnerEntityName:     "Saori",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-pmc19",
				SourceTurn:          2,
				MemoryText:          "Saori privately remembers the kitchen confession from another scene.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              71,
				CapsuleID:       12,
				SourceTurn:      7,
				MemoryText:      "Saori remembers a private kitchen promise.",
				Importance10:    8,
				Portability:     "npc_private_recollection",
				TagsJSON:        `["owner_entity_key:saori","owner_entity_name:Saori","owner_entity_role:npc","owner_visibility:owner_private","source_chat_session_id:old-saori-session","npc_private"]`,
				InjectionPolicy: "support_only_npc_private_recollection",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-pmc19","turn_index":4,"raw_user_input":"Chloe quietly checks the second exit sign with Siwoo.","settings":{"max_injection_chars":1800,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Subjective Memories and Relationships]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("expected Chloe private recollection in injection text: %q", injectionText)
	}
	if strings.Contains(injectionText, "Saori") || strings.Contains(injectionText, "kitchen promise") || strings.Contains(injectionText, "kitchen confession") {
		t.Fatalf("unrelated Saori memory leaked into prepare-turn injection: %q", injectionText)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) {
		t.Fatalf("expected exactly one relevant private recollection, got %+v", surface)
	}
	relevance, ok := resp["entity_recollection_relevance"].(map[string]any)
	if !ok {
		t.Fatalf("entity_recollection_relevance surface missing")
	}
	if relevance["character_private_before_filter"] != float64(2) || relevance["character_private_after_filter"] != float64(1) || relevance["character_private_dropped_count"] != float64(1) {
		t.Fatalf("unexpected relevance counts: %+v", relevance)
	}
	if relevance["blocks_unrelated_session_memory"] != true || relevance["blocks_unrelated_entity_memory"] != true {
		t.Fatalf("relevance surface must expose unrelated memory guards: %+v", relevance)
	}
}

func TestPrepareTurnCharacterPrivateRecollectionBlocksStaleOwnerMention(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-private-stale", TurnIndex: 1, Role: "user", Content: "Ashley Silvertrail argues in the archive."},
			{ID: 2, ChatSessionID: "target-private-stale", TurnIndex: 1, Role: "assistant", Content: "Ashley keeps her doubts private."},
			{ID: 3, ChatSessionID: "target-private-stale", TurnIndex: 2, Role: "user", Content: "Niv and Ingrid move to the garden."},
			{ID: 4, ChatSessionID: "target-private-stale", TurnIndex: 2, Role: "assistant", Content: "Niv laughs while Ingrid answers quietly."},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "target-private-stale", StateType: "scene", Content: `{"location":"garden","present_entities":["Niv","Ingrid"]}`, TurnIndex: 2},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "target-private-stale", LayerType: "entity_state", Content: `{"characters":[{"name":"Ashley Silvertrail"},{"name":"Niv"},{"name":"Ingrid"}]}`, TurnIndex: 2},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  81,
				OwnerEntityKey:      "ashley_silvertrail",
				OwnerEntityName:     "Ashley Silvertrail",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-stale",
				SourceTurn:          1,
				MemoryText:          "Ashley privately distrusts the archive conversation.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        9,
			},
			{
				ID:                  82,
				OwnerEntityKey:      "niv",
				OwnerEntityName:     "Niv",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-stale",
				SourceTurn:          2,
				MemoryText:          "Niv privately enjoys Ingrid's dry humor.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        7,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-private-stale","turn_index":3,"raw_user_input":"Niv asks Ingrid what she thinks of the garden path.","settings":{"max_injection_chars":2200,"max_input_context_chars":1200,"injection_enabled":true,"input_context_enabled":true,"top_k":4}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if strings.Contains(injectionText, "owner Ashley") || strings.Contains(injectionText, "archive conversation") {
		t.Fatalf("stale off-scene owner leaked into private recollection: %q", injectionText)
	}
	if !strings.Contains(injectionText, "owner Niv") {
		t.Fatalf("current-scene private recollection missing: %q", injectionText)
	}
	relevance, ok := resp["entity_recollection_relevance"].(map[string]any)
	if !ok {
		t.Fatalf("entity_recollection_relevance surface missing")
	}
	if relevance["character_private_before_filter"] != float64(1) || relevance["character_private_after_filter"] != float64(1) {
		t.Fatalf("unexpected stale-owner relevance counts: %+v", relevance)
	}
	if relevance["character_private_gate"] != "owner_entity_must_match_current_user_input_or_observed_current_scene_entity" {
		t.Fatalf("unexpected private recollection gate: %+v", relevance)
	}
}

func TestPrepareTurnCharacterPrivateRecollectionAllowsMentionedOffscreenOwner(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-private-mentioned", TurnIndex: 4, Role: "user", Content: "Niv and Ingrid sit near the stairwell."},
			{ID: 2, ChatSessionID: "target-private-mentioned", TurnIndex: 4, Role: "assistant", Content: "Their voices carry farther than they realize."},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "target-private-mentioned", StateType: "scene", Content: `{"location":"stairwell","present_entities":["Niv","Ingrid"]}`, TurnIndex: 4},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  91,
				OwnerEntityKey:      "ashley_silvertrail",
				OwnerEntityName:     "Ashley Silvertrail",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-mentioned",
				SourceTurn:          3,
				MemoryText:          "Ashley privately fears Niv and Ingrid already distrust her.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-private-mentioned","turn_index":5,"raw_user_input":"Niv and Ingrid quietly complain about Ashley Silvertrail, unaware she may be nearby.","settings":{"max_injection_chars":2200,"max_input_context_chars":1200,"injection_enabled":true,"input_context_enabled":true,"top_k":4}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "owner Ashley Silvertrail") {
		t.Fatalf("explicitly mentioned offscreen owner should remain eligible: %q", injectionText)
	}
	relevance, ok := resp["entity_recollection_relevance"].(map[string]any)
	if !ok {
		t.Fatalf("entity_recollection_relevance surface missing")
	}
	if relevance["character_private_after_filter"] != float64(1) {
		t.Fatalf("mentioned owner should pass private relevance gate: %+v", relevance)
	}
}

func TestPrepareTurnCharacterPrivateRecollectionKeepsRelevantSameOwnerFill(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-private-cap", TurnIndex: 7, Role: "user", Content: "Chloe waits in the hallway."},
			{ID: 2, ChatSessionID: "target-private-cap", TurnIndex: 7, Role: "assistant", Content: "Chloe keeps her expression calm."},
		},
		returnEntityMemories: []store.ProtagonistEntityMemory{
			{
				ID:                  101,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-cap",
				SourceTurn:          5,
				MemoryText:          "Chloe privately remembers the first hallway warning.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        8,
			},
			{
				ID:                  102,
				OwnerEntityKey:      "chloe",
				OwnerEntityName:     "Chloe",
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: "target-private-cap",
				SourceTurn:          6,
				MemoryText:          "Chloe privately remembers the second hallway warning.",
				TargetRevealPolicy:  "owner_private_until_revealed",
				Portability:         "npc_private_recollection",
				Importance10:        7,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-private-cap","turn_index":8,"raw_user_input":"Chloe asks Siwoo to slow down after recalling the first and second hallway warnings.","settings":{"max_injection_chars":2200,"max_input_context_chars":1200,"injection_enabled":true,"input_context_enabled":true,"top_k":4}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "first hallway warning") || !strings.Contains(injectionText, "second hallway warning") {
		t.Fatalf("same-owner relevant recollection fill failed: %q", injectionText)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["count"] != float64(2) {
		t.Fatalf("expected both relevant private recollections within the lane budget, got %+v", surface)
	}
	relevance, ok := resp["entity_recollection_relevance"].(map[string]any)
	if !ok {
		t.Fatalf("entity_recollection_relevance surface missing")
	}
	if relevance["character_private_before_filter"] != float64(2) || relevance["character_private_after_filter"] != float64(2) || relevance["character_private_owner_cap"] != "removed_after_eligibility" {
		t.Fatalf("unexpected coverage-first relevance surface: %+v", relevance)
	}
	dropped, ok := relevance["dropped"].([]any)
	if !ok || len(dropped) != 0 {
		t.Fatalf("distinct relevant same-owner recollections were unexpectedly dropped: %+v", relevance)
	}
}

func TestPrepareTurnAttachedNPCPrivateCapsuleUsesCharacterPrivateLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-attached-npc", TurnIndex: 1, Role: "user", Content: "Siwoo enters the old classroom."},
			{ID: 2, ChatSessionID: "target-attached-npc", TurnIndex: 1, Role: "assistant", Content: "Chloe becomes unusually careful."},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              42,
				CapsuleID:       9,
				SourceTurn:      6,
				MemoryText:      "Chloe remembers from a previous loop that Siwoo should avoid the broken bridge.",
				Importance10:    8.5,
				EmotionalWeight: 0.7,
				Portability:     "npc_private_recollection",
				TagsJSON:        `["owner_entity_key:chloe","owner_entity_name:Chloe","owner_entity_role:npc","owner_visibility:owner_private","source_chat_session_id:old-chloe-session","target_reveal_policy:owner_private_until_revealed","npc_private","secret_guard"]`,
				EvidenceExcerpt: "avoid the broken bridge",
				InjectionPolicy: "support_only_npc_private_recollection",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-attached-npc","turn_index":2,"raw_user_input":"He asks Chloe if they should cross the bridge.","settings":{"max_injection_chars":1400,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Subjective Memories and Relationships]") || !strings.Contains(injectionText, "owner Chloe") {
		t.Fatalf("attached NPC capsule did not enter character private lane: %q", injectionText)
	}
	if strings.Contains(injectionText, "support-only private recollection") {
		t.Fatalf("attached NPC capsule leaked into persona lane: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Chloe remembers from a previous loop that Siwoo should avoid the broken bridge.") {
		t.Fatalf("attached NPC capsule omitted exact protected memory: %q", injectionText)
	}
	surface, ok := resp["character_private_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("character_private_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["visible_to_player"] != false {
		t.Fatalf("character private attached capsule surface mismatch: %+v", surface)
	}
	items, ok := surface["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("character private items missing: %+v", surface["items"])
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["owner_entity_key"] != "chloe" || first["owner_visibility"] != "owner_private" {
		t.Fatalf("attached NPC capsule owner metadata not restored: %+v", first)
	}
	personaSurface, ok := resp["persona_recollection"].(map[string]any)
	if !ok || personaSurface["status"] != "empty" || personaSurface["count"] != float64(0) {
		t.Fatalf("persona surface should remain empty for NPC-private capsule: %+v", personaSurface)
	}
}

func TestPrepareTurnEpisodeDenseAnchorsSurviveSummaryText(t *testing.T) {
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1a",
			FromTurn:                1,
			ToTurn:                  2,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               time.Unix(20, 0),
		},
	}
	assembly := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, nil, nil, episodes, nil, nil, nil, 5, 1200, "Alice opens the sealed gate.", "wide_context_700k", nil, nil, nil)
	if !strings.Contains(assembly.EpisodeText, "key_event=Alice opens the sealed gate") {
		t.Fatalf("episode_text missing key event anchor: %s", assembly.EpisodeText)
	}
	if !strings.Contains(assembly.EpisodeText, "rel=Alice trusts Bob") {
		t.Fatalf("episode_text missing relationship anchor: %s", assembly.EpisodeText)
	}
	if !strings.Contains(assembly.EpisodeText, "open_loop=sealed gate remains unresolved") {
		t.Fatalf("episode_text missing open-loop anchor: %s", assembly.EpisodeText)
	}
}

func TestPrepareTurnEpisodeDoesNotRepeatSummaryAsKeyEvent(t *testing.T) {
	episodes := []store.EpisodeSummary{{
		ID: 43, ChatSessionID: "sess-ds1b", FromTurn: 1, ToTurn: 5,
		SummaryText: "memory: Alice opens the sealed gate",
		KeyEvents:   `["memory: Alice opens the sealed gate"]`,
	}}
	assembly := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, nil, nil, episodes, nil, nil, nil, 5, 1200, "Alice opens the sealed gate.", "wide_context_700k", nil, nil, nil)
	if strings.Contains(assembly.EpisodeText, "key_event=memory: Alice opens the sealed gate") {
		t.Fatalf("episode summary repeated as key event: %s", assembly.EpisodeText)
	}
	if strings.Count(assembly.EpisodeText, "memory: Alice opens the sealed gate") != 1 {
		t.Fatalf("episode fact count=%d, want 1: %s", strings.Count(assembly.EpisodeText, "memory: Alice opens the sealed gate"), assembly.EpisodeText)
	}
}

func TestUnifiedRetrievalDocumentsEpisodeDenseAnchors(t *testing.T) {
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1a",
			FromTurn:                1,
			ToTurn:                  2,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               time.Unix(20, 0),
		},
	}
	docs := buildUnifiedRetrievalDocuments("sess-ds1a", nil, nil, nil, episodes, nil, nil)
	if len(docs) != 1 {
		t.Fatalf("docs = %d, want 1", len(docs))
	}
	text := extractionStringFromAny(docs[0]["text"])
	if !strings.Contains(text, "Alice opens the sealed gate") || !strings.Contains(text, "Alice trusts Bob") || !strings.Contains(text, "sealed gate remains unresolved") {
		t.Fatalf("episode retrieval document text missing dense anchors: %s", text)
	}
	meta, _ := docs[0]["metadata"].(map[string]any)
	if meta["key_events"] == "" || meta["relationship_changes_json"] == "" || meta["open_loops_json"] == "" {
		t.Fatalf("episode retrieval metadata missing dense anchors: %+v", meta)
	}
}

func TestUnifiedRetrievalDocumentsDenseSummaryMetadataSurfaces(t *testing.T) {
	now := time.Unix(30, 0)
	evidence := []store.DirectEvidence{
		{
			ID:              9,
			ChatSessionID:   "sess-ds1f-docs",
			EvidenceKind:    "relationship_world_promise",
			EvidenceText:    "Alice and Bob promise to obey the gate law.",
			SourceTurnStart: 10,
			SourceTurnEnd:   80,
			TurnAnchor:      40,
		},
	}
	episodes := []store.EpisodeSummary{
		{
			ID:                      42,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                10,
			ToTurn:                  20,
			SummaryText:             "A short episode summary.",
			KeyEvents:               `["Alice opens the sealed gate"]`,
			RelationshipChangesJSON: `["Alice trusts Bob"]`,
			OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
			CreatedAt:               now,
		},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID:                      50,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                21,
			ToTurn:                  40,
			ChapterIndex:            2,
			ChapterTitle:            "Gate Promise",
			SummaryText:             "The gate promise becomes explicit.",
			ResumeText:              "The gate promise remains unresolved.",
			RelationshipChangesJSON: `["Alice and Bob form an alliance"]`,
			WorldChangesJSON:        `["gate law changes the archive"]`,
			CallbackCandidatesJSON:  `["repay the promise"]`,
			CreatedAt:               &now,
		},
		Arc: &store.ArcSummary{
			ID:                     60,
			ChatSessionID:          "sess-ds1f-docs",
			FromTurn:               41,
			ToTurn:                 70,
			ArcName:                "Gate Arc",
			CoreConflict:           "The gate law binds the group.",
			ActivePromisesJSON:     `["protect the gate"]`,
			RelationshipPivotsJSON: `["Alice trusts Bob"]`,
			CreatedAt:              &now,
		},
		Saga: &store.SagaDigest{
			ID:                      70,
			ChatSessionID:           "sess-ds1f-docs",
			FromTurn:                1,
			ToTurn:                  80,
			EraLabel:                "Gate Era",
			SagaSummary:             "The gate law shapes the era.",
			PersistentFactsJSON:     `["gate law persists"]`,
			NeverDropCandidatesJSON: `["Alice and Bob promise remains"]`,
			CreatedAt:               &now,
		},
	}
	docs := buildUnifiedRetrievalDocuments("sess-ds1f-docs", nil, evidence, nil, episodes, resumePack, nil)
	byTier := map[string]map[string]any{}
	for _, doc := range docs {
		tier, _ := doc["tier"].(string)
		byTier[tier] = doc
	}
	for _, tier := range []string{"episode", "chapter", "arc", "saga"} {
		doc, ok := byTier[tier]
		if !ok {
			t.Fatalf("missing %s doc in %+v", tier, byTier)
		}
		meta, ok := doc["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("%s metadata missing: %+v", tier, doc)
		}
		if meta["dense_source_anchor_policy_version"] != denseSourceAnchorPolicyVersion {
			t.Fatalf("%s DS-1f source anchor missing: %+v", tier, meta)
		}
		if meta["dense_role_split_policy_version"] != denseRoleSplitPolicyVersion || meta["dense_structured_usage"] != "adjudication_retrieval" {
			t.Fatalf("%s DS-1h role split missing: %+v", tier, meta)
		}
		if meta["dense_retention_policy_version"] != denseRetentionPolicyVersion || meta["dense_retention_applied"] != true {
			t.Fatalf("%s DS-1g retention missing: %+v", tier, meta)
		}
		if meta["dense_direct_evidence_promotion_policy_version"] != denseEvidencePromotionPolicy || meta["dense_structured_precedence_applied"] != true {
			t.Fatalf("%s DS-1i promotion missing: %+v", tier, meta)
		}
	}
}

func TestPrepareTurnNarrativeGuideAutoModeBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-guide","turn_index":1,"raw_user_input":"If combat starts, immediately engage and defend.","settings":{"guide_mode":"auto","guide_strength":"strong","injection_enabled":false,"input_context_enabled":false}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pack, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack is not an object: %+v", resp)
	}
	if pack["guide_mode"] != "standard" {
		t.Fatalf("guide_mode = %v, want stable standard auto mode", pack["guide_mode"])
	}
	if pack["guide_strength"] != "strong" {
		t.Fatalf("guide_strength = %v, want strong", pack["guide_strength"])
	}
	focus := stringSliceFromAny(pack["guide_focus"])
	if len(focus) == 0 || focus[0] != "coherent continuity" {
		t.Fatalf("guide_focus = %#v, want standard continuity focus", focus)
	}
	for _, key := range []string{"guide_suffix", "director_overrides", "narrative_stance_bounds", "auto_advance_hint"} {
		if _, exists := pack[key]; exists {
			t.Fatalf("guide pack exposes story-control field %q: %#v", key, pack[key])
		}
	}
	for _, key := range []string{"autonomy_plan", "micro_beat_proposal", "scene_step_proposal", "combined_proposal"} {
		if _, exists := resp[key]; exists {
			t.Fatalf("prepare-turn exposes story planner %q: %#v", key, resp[key])
		}
	}
}

func TestNarrativeGuideFocusIsDirectionalOnly(t *testing.T) {
	for _, input := range []string{
		"The romantic mood deepens and the scene moves closer.",
		"전투가 시작되고 추격이 이어진다.",
		"恋愛と戦闘が同時に描かれる。",
	} {
		if got := resolveNarrativeGuideMode("auto", []map[string]any{{"role": "user", "content": input}}, input, input); got != "standard" {
			t.Fatalf("resolveNarrativeGuideMode(auto, %q) = %q, want language-invariant standard", input, got)
		}
	}
	focus := buildNarrativeGuideFocus("mature_soft")
	if len(focus) != 2 || focus[0] != "sensory atmosphere" {
		t.Fatalf("mature_soft guide focus mismatch: %#v", focus)
	}
	if focus := buildNarrativeGuideFocus("strict"); len(focus) != 0 {
		t.Fatalf("unknown guide mode should not create focus, got %#v", focus)
	}
}

func TestPrepareTurnCharacterBlockIncludesSpeechStyle(t *testing.T) {
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", Content: `{"present_entities":["Chloe"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{{
			ChatSessionID:     "sess-speech",
			CharacterName:     "Chloe",
			StatusJSON:        `{"mood":"guarded"}`,
			SpeechStyleJSON:   `{"default_tone":"dry","speech_notes":"short replies"}`,
			RelationshipsJSON: `{"Hero":{"affection":35}}`,
		}},
		nil, nil, nil,
		nil,
		nil,
		nil,
		3, 1000,
		"How does Chloe answer?",
		"default",
		nil,
		nil,
		nil,
		perspective,
	)
	if !strings.Contains(assembly.CharacterText, "speech_style") || !strings.Contains(assembly.CharacterText, "dry") || !strings.Contains(assembly.CharacterText, "short replies") {
		t.Fatalf("character block should include speech style guidance, got %q", assembly.CharacterText)
	}
	if !strings.Contains(assembly.Text, "[Characters]") || !strings.Contains(assembly.Text, "speech_style") {
		t.Fatalf("injection text should carry speech style inside character block, got %q", assembly.Text)
	}
}

func TestPrepareTurnVectorHitsHydrateIntoMemoryLane(t *testing.T) {
	memories := []store.Memory{
		{
			ID:          1,
			TurnIndex:   1,
			SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine.","language_context":{"contract_version":"language_memory.v1","session_output_language":"en","raw_user_language":"ko","summary_language":"en","search_text_policy":"summary_plus_raw_plus_aliases"},"entities":[{"name":"Mina","aliases":["미나"]}],"archive_hint":{"wing":"North Wing"}}`,
			Evidence:    `{"evidence_excerpts":["RAW-KO: Mina hid the brass key."]}`,
			Importance:  5,
		},
		{
			ID:          2,
			TurnIndex:   9,
			SummaryJSON: `{"turn_summary":"Unrelated recent market conversation."}`,
			Importance:  9,
		},
	}
	vectorShadow := map[string]any{
		"memory_search_result": "ok",
		"search_result":        "ok",
		"search_results": []map[string]any{
			{
				"id":                      "memory:sess-vector:1",
				"source_table":            "memories",
				"source_row_id":           "1",
				"search_text_policy":      "summary_plus_raw_plus_aliases",
				"raw_language":            "ko",
				"summary_language":        "en",
				"session_output_language": "en",
				"alias_count":             3,
				"similarity":              0.86,
				"similarity_source":       "cosine_from_query_and_stored_embedding",
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "Where is the key?", 1, vectorShadow)
	if len(selection.VectorRelevant) != 1 {
		t.Fatalf("vector relevant count = %d, want 1; trace=%#v", len(selection.VectorRelevant), selection.Trace)
	}
	if selection.VectorRelevant[0].ID != 1 {
		t.Fatalf("vector relevant id = %d, want old semantic memory id 1", selection.VectorRelevant[0].ID)
	}
	if got := prepareTurnSelectedMemoryCount(selection); got != 1 {
		t.Fatalf("selected count = %d, want topK-limited 1", got)
	}
	if len(selection.Recent)+len(selection.Relevant)+len(selection.Deep) != 0 {
		t.Fatalf("vector-selected memory should consume topK slot without duplicate fallback lanes: %#v", selection)
	}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 2000, "Where is the key?", "default", nil, vectorShadow, nil)
	if !strings.Contains(assembly.MemoryText, "[vector_relevant, turn 1") || !strings.Contains(assembly.MemoryText, "Mina hid the brass key") {
		t.Fatalf("memory_text should expose vector_relevant lane and hydrated memory: %q", assembly.MemoryText)
	}
	for key, want := range map[string]int{
		"vector_memory_hit_count":                       1,
		"vector_memory_hydrated_count":                  1,
		"vector_memory_selected_count":                  1,
		"vector_memory_injected_count":                  1,
		"vector_memory_hit_language_context_count":      1,
		"vector_memory_hit_alias_indexed_count":         1,
		"vector_memory_hydrated_language_context_count": 1,
		"vector_memory_hydrated_alias_ready_count":      1,
		"vector_relevant_memory_count":                  1,
		"selected_memory_total_count":                   1,
	} {
		if got := intFromAny(assembly.Counts[key], 0); got != want {
			t.Fatalf("assembly.Counts[%s] = %d, want %d; counts=%#v", key, got, want, assembly.Counts)
		}
	}
	if got := stringFromMap(assembly.Counts, "vector_memory_search_text_policy"); got != "summary_plus_raw_plus_aliases" {
		t.Fatalf("vector_memory_search_text_policy = %q, counts=%#v", got, assembly.Counts)
	}
}

func TestPrepareTurnVectorReadyLeavesUnusedTopKEmptyInsteadOfInjectingUnrelatedRecent(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine."}`, Importance: 5},
		{ID: 2, TurnIndex: 9, SummaryJSON: `{"turn_summary":"Recent unrelated market conversation."}`, Importance: 9},
		{ID: 3, TurnIndex: 10, SummaryJSON: `{"turn_summary":"Another unrelated recent tail."}`, Importance: 8},
	}
	vectorShadow := map[string]any{
		"memory_search_result": "ok",
		"search_result":        "ok",
		"search_results": []map[string]any{
			{"id": "memory:sess-vector:1", "source_table": "memories", "source_row_id": "1", "similarity": 0.84, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "Where is the key?", 3, vectorShadow)
	if len(selection.VectorRelevant) != 1 {
		t.Fatalf("vector relevant count = %d, want 1", len(selection.VectorRelevant))
	}
	if len(selection.Relevant)+len(selection.Deep)+len(selection.Recent) != 0 {
		t.Fatalf("unrelated memories must not fill unused topK slots: %#v", selection)
	}
	if got := prepareTurnSelectedMemoryCount(selection); got != 1 {
		t.Fatalf("selected count = %d, want only the supported vector hit", got)
	}
	if selection.Trace["lexical_fill_enabled"] != true || selection.Trace["vector_recall_ready"] != true {
		t.Fatalf("vector-ready trace mismatch: %#v", selection.Trace)
	}
}

func TestPrepareTurnEventMemoryQueryDoesNotBorrowOtherLaneContext(t *testing.T) {
	ctx := buildPrepareTurnRecollectionContext(
		"current forge work",
		[]store.Memory{
			{TurnIndex: 4, SummaryJSON: `{"turn_summary":"older market event"}`},
			{TurnIndex: 5, SummaryJSON: `{"turn_summary":"previous stored forge work event"}`},
		},
		[]store.ActiveState{{TurnIndex: 5, Content: `{"location":"royal forge","status":"inspection ready","present_entities":["Mira"]}`}},
		nil,
		[]store.PendingThread{{Status: "open", Description: "complete the royal inspection"}},
	)
	query := prepareTurnEventMemoryQuery(ctx, []string{"Mira"})
	for _, wanted := range []string{"current forge work", "previous stored forge work event", "Mira"} {
		if !strings.Contains(query, wanted) {
			t.Fatalf("query missing %q: %q", wanted, query)
		}
	}
	for _, unwanted := range []string{"older market event", "previous assistant full output", "royal forge", "complete the royal inspection"} {
		if strings.Contains(query, unwanted) {
			t.Fatalf("event query borrowed another lane %q: %q", unwanted, query)
		}
	}
}

func TestPreviousAssistantRawStaysInInputContextButCannotActivateSupportRecall(t *testing.T) {
	const assistantOnlyAnchor = "obsolete-passport-token"
	assistantOutput := assistantOnlyAnchor + strings.Repeat(" x", (6718-len(assistantOnlyAnchor))/2)
	chatLogs := []store.ChatLog{{TurnIndex: 5, Role: "assistant", Content: assistantOutput}}

	inputContext, _ := buildInputContextText(chatLogs, 8000)
	if !strings.Contains(inputContext, assistantOnlyAnchor) {
		t.Fatalf("previous assistant raw missing from Input Context: %q", inputContext)
	}

	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 1, TurnIndex: 5, SummaryJSON: `{"turn_summary":"Mira completed the forge inspection."}`}},
		nil,
		nil,
		chatLogs,
		nil,
		[]store.WorldRule{{ID: 9, Key: assistantOnlyAnchor, ValueJSON: `{"rule":"unrelated old passport rule"}`}},
		[]store.CharacterState{{TurnIndex: 5, CharacterName: "Mira", StatusJSON: `{"location":"forge"}`}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		5,
		9000,
		"Mira rests after the forge inspection.",
		"default",
		nil,
		nil,
		nil,
	)
	if strings.Contains(assembly.WorldRulesText, assistantOnlyAnchor) || strings.Contains(assembly.Text, "unrelated old passport rule") {
		t.Fatalf("previous assistant raw activated unrelated support recall: %q", assembly.WorldRulesText)
	}
	if assembly.Counts["previous_assistant_raw_used_for_search"] != false || assembly.Counts["previous_assistant_raw_delivery_owner"] != "input_context_only" {
		t.Fatalf("previous assistant ownership trace mismatch: %#v", assembly.Counts)
	}
}

func TestPrepareTurnLexicalRecallRequiresCurrentEvidenceAndDoesNotSpendCapacity(t *testing.T) {
	selection := selectPrepareTurnMemoryLanes([]store.Memory{
		{ID: 1, TurnIndex: 10, SummaryJSON: `{"turn_summary":"The current forge prepares royal inspection supplies.","entities":[{"name":"Han-eol","location":"forge"}]}`},
		{ID: 2, TurnIndex: 11, SummaryJSON: `{"turn_summary":"A previous political conversation happened elsewhere."}`},
		{ID: 3, TurnIndex: 12, SummaryJSON: `{"turn_summary":"An unrelated market visit happened previously."}`},
	}, "Han-eol checks the royal inspection supplies at the forge.", 5)
	if len(selection.Relevant) != 1 || selection.Relevant[0].ID != 1 {
		t.Fatalf("relevant=%#v, want only the current-scene memory", selection.Relevant)
	}
	if len(selection.Recent) != 0 {
		t.Fatalf("query-present recall filled unused capacity with recent rows: %#v", selection.Recent)
	}
	if got := intFromAny(selection.Trace["actual_memory_refill_gap"], -1); got != 0 {
		t.Fatalf("unused budget was reported as a fill target gap=%d; trace=%#v", got, selection.Trace)
	}
}

func TestPrepareTurnSupportLanesDropUnrelatedRowsAndKeepLatestEpisodeAnchor(t *testing.T) {
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", Content: `{"location":"sealed gate","present_entities":["Alice"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		[]store.Memory{{ID: 1, TurnIndex: 20, SummaryJSON: `{"turn_summary":"Alice opens the sealed gate.","entities":[{"name":"Alice"}]}`}},
		[]store.KGTriple{
			{ID: 1, Subject: "Alice", Predicate: "carries", Object: "sealed key"},
			{ID: 2, Subject: "Bob", Predicate: "visits", Object: "market"},
			{ID: 3, Subject: "Alice", Predicate: "met", Object: "Rowan"},
		},
		nil, nil,
		[]store.Storyline{
			{ID: 1, Name: "Sealed Gate", CurrentContext: "Alice opens the sealed gate"},
			{ID: 2, Name: "Market Rumor", CurrentContext: "Bob repeats a market rumor"},
		},
		nil,
		[]store.CharacterState{
			{ID: 1, CharacterName: "Alice", StatusJSON: `{"location":"sealed gate"}`},
			{ID: 2, CharacterName: "Bob", StatusJSON: `{"location":"market"}`},
		},
		[]store.PendingThread{
			{ID: 1, ThreadKey: "sealed_gate", Description: "Alice must open the sealed gate", Status: "open", Owner: "Alice"},
			{ID: 2, ThreadKey: "market_rumor", Description: "Bob must verify the market rumor", Status: "open", Owner: "Bob"},
		},
		nil,
		[]store.EpisodeSummary{
			{ID: 1, FromTurn: 1, ToTurn: 5, SummaryText: "Alice discovered the sealed gate"},
			{ID: 2, FromTurn: 6, ToTurn: 10, SummaryText: "Bob traded cloth at the market"},
			{ID: 3, FromTurn: 11, ToTurn: 15, SummaryText: "The latest episode closes at night"},
		},
		nil, nil, nil,
		5, 12000, "Alice opens the sealed gate.", "default", nil, nil, nil, perspective,
	)
	for _, text := range []string{assembly.KGText, assembly.StorylineText, assembly.CharacterText, assembly.PendingThreadText, assembly.EpisodeText} {
		if strings.Contains(text, "Bob") || strings.Contains(text, "market") || strings.Contains(text, "Rowan") {
			t.Fatalf("unrelated support row survived relevance gate: %q", text)
		}
	}
	for _, wanted := range []string{"Alice --carries--> sealed key", "Alice opens the sealed gate", "Alice: state=", "Alice must open", "turns 1-5"} {
		if !strings.Contains(assembly.Text, wanted) {
			t.Fatalf("assembly missing supported continuity %q: %s", wanted, assembly.Text)
		}
	}
	relationshipClassFound := false
	for _, rawClass := range prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryPlan["classes"]) {
		class := mapFromAny(rawClass)
		if extractionStringFromAny(class["key"]) != "subjective_relationship" {
			continue
		}
		relationshipClassFound = true
		if intFromAny(class["eligible_count"], 0) != 1 || intFromAny(class["selected_count"], 0) != 1 || intFromAny(class["used_chars"], 0) <= 0 {
			t.Fatalf("relationship lane candidate/selected/final chars mismatch: %#v", class)
		}
	}
	if !relationshipClassFound {
		t.Fatalf("relationship lane diagnostics missing: %#v", assembly.MemoryDeliveryPlan)
	}
	for key, want := range map[string]int{
		"kg_irrelevant_dropped":              2,
		"kg_single_endpoint_only_dropped":    1,
		"storyline_irrelevant_dropped":       1,
		"character_state_irrelevant_dropped": 1,
		"pending_thread_irrelevant_dropped":  1,
		"episode_irrelevant_dropped":         2,
	} {
		if got := intFromAny(assembly.Counts[key], 0); got != want {
			t.Fatalf("%s=%d, want %d; counts=%#v", key, got, want, assembly.Counts)
		}
	}
}

func TestPrepareTurnRejectsLowSimilarityVectorAndUsesQueryRelevantFallback(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":"Unrelated opening battle memory."}`, Importance: 0.9},
		{ID: 2, TurnIndex: 20, SummaryJSON: `{"turn_summary":"The party leaves the clinic and enters the shopping street."}`, Importance: 0.5},
	}
	vectorShadow := map[string]any{
		"memory_search_result": "ok",
		"search_result":        "ok",
		"search_results": []map[string]any{
			{
				"id":                "memory:sess-vector:1",
				"source_table":      "memories",
				"source_row_id":     "1",
				"similarity":        0.08,
				"similarity_source": "cosine_from_query_and_stored_embedding",
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "Leave the clinic and walk into the shopping street.", 1, vectorShadow)
	if len(selection.VectorRelevant) != 0 {
		t.Fatalf("low-similarity vector hit must not be injected: %#v", selection.VectorRelevant)
	}
	if len(selection.Relevant) != 1 || selection.Relevant[0].ID != 2 {
		t.Fatalf("query-relevant lexical memory should replace rejected vector hit: %#v", selection)
	}
	trace := mapFromAny(selection.Trace["vector_recall"])
	if got := intFromAny(trace["below_similarity_count"], 0); got != 1 {
		t.Fatalf("below_similarity_count = %d, want 1; trace=%#v", got, trace)
	}
}

func TestStep29RegressionPrepareTurnVectorIsInjectedNotShadowOnly(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-29-shadow-only", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Old semantic oath belongs in the next prompt."}`, Importance: 0.1},
			{ID: 2, ChatSessionID: "sess-29-shadow-only", TurnIndex: 20, SummaryJSON: `{"turn_summary":"Recent unrelated weather note."}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 2, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-29-shadow-only:1", Tier: "memory", ChatSessionID: "sess-29-shadow-only", SourceTable: "memories", SourceRowID: "1", DocumentText: "old semantic oath", Similarity: 0.83, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
		},
	}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-29-shadow-only",
		"turn_index":21,
		"raw_user_input":"Continue the old oath.",
		"client_meta":{"chroma_query_vector":[0.1,0.2]},
		"settings":{"max_injection_chars":1200,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack missing: %#v", resp["injection_pack"])
	}
	memoryText, _ := injectionPack["memory_text"].(string)
	if !strings.Contains(memoryText, "[vector_relevant, turn 1") || !strings.Contains(memoryText, "Old semantic oath") {
		t.Fatalf("memory_text did not inject hydrated vector memory: %q", memoryText)
	}
	if strings.Contains(memoryText, "Recent unrelated weather note") {
		t.Fatalf("recent unrelated memory should not replace vector topK slot: %q", memoryText)
	}
	counts := injectionPack["counts"].(map[string]any)
	for key, want := range map[string]float64{
		"vector_memory_hit_count":      1,
		"vector_memory_hydrated_count": 1,
		"vector_memory_selected_count": 1,
		"vector_memory_injected_count": 1,
		"selected_memory_total_count":  1,
	} {
		if got := counts[key]; got != want {
			t.Fatalf("injection_pack.counts[%s] = %v, want %v; counts=%#v", key, got, want, counts)
		}
	}
	recall := resp["recall_result"].(map[string]any)
	searchBundle := recall["search"].(map[string]any)
	if searchBundle["memory_count"] != float64(1) || searchBundle["fallback_count"] != float64(0) {
		t.Fatalf("recall_result.search should expose injected vector memory count, got %#v", searchBundle)
	}
	vectorShadow := recall["vector_shadow"].(map[string]any)
	if vectorShadow["search_result"] != "ok" {
		t.Fatalf("vector_shadow.search_result = %v, want ok", vectorShadow["search_result"])
	}
}

func TestPrepareTurnInputTransparencyRenderModelExposesSafeBlocksAndCounters(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{
				ID:            1,
				ChatSessionID: "sess-247a-render",
				TurnIndex:     7,
				SummaryJSON: `{
					"turn_summary":"Gloria privately inherited the sealed crest.",
					"protected_secrets":[{
						"secret_kind":"power_inheritance",
						"secret_summary":"Gloria privately inherited the sealed crest.",
						"disclosure_policy":"owner_private_until_revealed",
						"knowledge_scope":{"known_by":["Gloria"],"unknown_to":["Siwoo"]}
					}],
					"character_identity_accuracy":[{
						"surface_identity_name":"Lia",
						"true_identity_name":"Gloria",
						"canonical_entity_name":"Gloria",
						"identity_kind":"cover_identity",
						"same_entity":true,
						"reveal_policy":"owner_private_until_revealed",
						"knowledge_scope":{"known_by":["Gloria"],"unknown_to":["Siwoo"]}
					}]
				}`,
				Importance: 0.95,
			},
			{ID: 2, ChatSessionID: "sess-247a-render", TurnIndex: 8, SummaryJSON: `{"turn_summary":"Gloria continues the private scene carefully at the market."}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 2, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:sess-247a-render:1", Tier: "memory", ChatSessionID: "sess-247a-render", SourceTable: "memories", SourceRowID: "1", DocumentText: "sealed crest protected identity", Similarity: 0.88, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
		},
	}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-247a-render",
		"turn_index":9,
		"raw_user_input":"Continue Gloria's private scene carefully.",
		"client_meta":{"chroma_query_vector":[0.4,0.2]},
		"settings":{"apply_mode":"shadow","max_injection_chars":3000,"injection_enabled":true,"input_context_enabled":false,"top_k":1}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	model := mapFromAny(resp["input_transparency_model"])
	if model["contract_version"] != "input_transparency_render.v1" || model["read_only"] != true || model["llm_call_attempted"] != false {
		t.Fatalf("input_transparency_model contract mismatch: %#v", model)
	}
	if model["secret_display_policy"] != "counts_only_no_secret_text" {
		t.Fatalf("secret_display_policy = %v", model["secret_display_policy"])
	}
	counts := mapFromAny(model["counts"])
	for key, want := range map[string]int{
		"vector_found":                   1,
		"vector_hydrated":                0,
		"vector_injected":                0,
		"memory_injected":                2,
		"actual_memory_selected_count":   1,
		"protected_guard_selected_count": 1,
		"protected_secret_count":         1,
		"identity_accuracy_count":        1,
		"protected_memory_guarded_count": 1,
	} {
		if got := intFromAny(counts[key], 0); got != want {
			t.Fatalf("input_transparency_model.counts[%s] = %d, want %d; counts=%#v", key, got, want, counts)
		}
	}
	vectorRecall := mapFromAny(mapFromAny(counts["memory_recall_lane_policy"])["vector_recall"])
	if got := intFromAny(vectorRecall["scope_filtered_count"], 0); got != 1 {
		t.Fatalf("private vector hit scope-filtered count = %d, want 1; vector_recall=%#v", got, vectorRecall)
	}
	related := map[string]any(nil)
	protected := map[string]any(nil)
	for _, raw := range sliceFromAny(model["blocks"]) {
		block := mapFromAny(raw)
		if block["key"] == "related_memories" {
			related = block
		}
		if block["key"] == "protected_memory_guidance" {
			protected = block
		}
	}
	if related == nil || related["status"] != "included" {
		t.Fatalf("related_memories block missing or not included: %#v", model["blocks"])
	}
	if protected == nil || protected["status"] != "included" {
		t.Fatalf("protected_memory_guidance block missing or not included: %#v", model["blocks"])
	}
	protectedText := extractionStringFromAny(protected["text"])
	if !strings.Contains(protectedText, "Protected identity continuity") || !strings.Contains(protectedText, "kind=cover_identity") {
		t.Fatalf("protected guidance block should contain protected guard text, got %q", protectedText)
	}
	modelJSON := mustCompactJSON(model)
	for _, leaked := range []string{"Gloria privately inherited the sealed crest", "secret_summary", "true_identity_name", "surface_identity_name"} {
		if strings.Contains(modelJSON, leaked) {
			t.Fatalf("input_transparency_model leaked protected detail %q: %s", leaked, modelJSON)
		}
	}
	preview := mapFromAny(resp["effective_input_preview"])
	if preview["contract_version"] != "effective_input_preview.v1" || preview["payload_apply_mode"] != "shadow" || preview["raw_user_rewritten"] != false {
		t.Fatalf("effective_input_preview contract mismatch: %#v", preview)
	}
	if intFromAny(preview["auxiliary_context_chars"], 0) <= 0 {
		t.Fatalf("effective_input_preview auxiliary_context_chars not populated: %#v", preview)
	}
}

func TestPrepareTurnEffectiveInputPreviewReportsGoSelectedSource(t *testing.T) {
	preview := buildPrepareTurnEffectiveInputPreview(
		"sess-source-preview", 3, "실제 사용자 입력", "active_chat:4", "model", "shadow", "",
		true, false, false, false, "", prepareTurnInjectionAssembly{},
	)
	if preview["final_user_source"] != "active_chat:4" || preview["final_user_text"] != "실제 사용자 입력" {
		t.Fatalf("effective input preview did not preserve Go-selected source: %#v", preview)
	}
}

func TestMEMADeliveryLineageConnectsRowsVectorHitsAndFinalTopKConsumption(t *testing.T) {
	const sid = "sess-mem-a-lineage"
	identityJSON := func(summary string) string {
		return fmt.Sprintf(`{
			"turn_summary":%q,
			"character_identity_accuracy":[{
				"surface_identity_name":"Faust",
				"true_identity_name":"Mina",
				"canonical_entity_name":"Mina",
				"identity_kind":"cover_identity",
				"same_entity":true,
				"reveal_policy":"owner_private_until_revealed",
				"knowledge_scope":{"known_by":["Mina"]}
			}]
		}`, summary)
	}
	secretJSON := func(summary, kind string) string {
		return fmt.Sprintf(`{
			"turn_summary":%q,
			"protected_secrets":[{
				"owner":"Mina",
				"secret_kind":%q,
				"secret_summary":%q,
				"disclosure_policy":"owner_private_until_revealed",
				"knowledge_scope":{"known_by":["Mina"]}
			}]
		}`, summary, kind, summary)
	}
	fake := &turnRecordingStore{returnMemories: []store.Memory{
		{ID: 11, ChatSessionID: sid, TurnIndex: 11, SummaryJSON: identityJSON("Mina still uses Faust as a private cover identity."), Importance: 0.9},
		{ID: 9, ChatSessionID: sid, TurnIndex: 9, SummaryJSON: identityJSON("Faust remains Mina's protected cover identity."), Importance: 0.9},
		{ID: 2, ChatSessionID: sid, TurnIndex: 2, SummaryJSON: secretJSON("Mina keeps the evacuation route hidden.", "hidden_plan"), Importance: 0.8},
		{ID: 17, ChatSessionID: sid, TurnIndex: 17, SummaryJSON: secretJSON("Mina secretly watches the eastern gate.", "surveillance"), Importance: 0.8},
		{ID: 12, ChatSessionID: sid, TurnIndex: 12, SummaryJSON: secretJSON("Mina has an undisclosed council allegiance.", "hidden_allegiance"), Importance: 0.8},
		{ID: 31, ChatSessionID: sid, TurnIndex: 31, SummaryJSON: `{"turn_summary":"Mina secured the archive permit during the council hearing."}`, Importance: 0.7},
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 6, ModelReady: true},
		searchResults: []vector.VectorDocument{
			{ID: "memory:" + sid + ":11", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "11", Similarity: 0.91, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
			{ID: "memory:" + sid + ":9", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "9", Similarity: 0.89, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
			{ID: "memory:" + sid + ":2", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "2", Similarity: 0.87, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
			{ID: "memory:" + sid + ":17", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "17", Similarity: 0.84, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
			{ID: "memory:" + sid + ":12", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "12", Similarity: 0.81, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
			{ID: "memory:" + sid + ":31", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "31", Similarity: 0.76, SimilarityAvailable: true, SimilaritySource: "cosine_from_query_and_stored_embedding"},
		},
	}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{
		"chat_session_id":"sess-mem-a-lineage",
		"turn_index":32,
		"raw_user_input":"Mina reviews the hidden plan, surveillance, allegiance, identity, and council hearing.",
		"client_meta":{"chroma_query_vector":[0.3,0.6]},
		"settings":{"apply_mode":"shadow","max_injection_chars":9000,"injection_enabled":true,"input_context_enabled":false,"top_k":5}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pack := mapFromAny(resp["injection_pack"])
	lineage := mapFromAny(pack["memory_delivery_lineage"])
	if lineage["contract_version"] != "memory_delivery_lineage.v1" {
		t.Fatalf("lineage contract missing: %#v", lineage)
	}
	if lineage["status"] != "mixed" {
		t.Fatalf("lineage status = %v, want mixed objective memory plus protected guards: %#v", lineage["status"], lineage)
	}
	if got := intFromAny(lineage["top_k_memory_target"], 0); got != 5 {
		t.Fatalf("top_k target = %d, want 5", got)
	}
	if got := intFromAny(lineage["final_delivered_count"], 0); got != 6 {
		t.Fatalf("final delivered = %d, want all six distinct source occurrences within the global envelope; lineage=%#v", got, lineage)
	}
	if got := intFromAny(lineage["final_protected_guard_count"], 0); got != 5 {
		t.Fatalf("protected guard count = %d, want all five distinct protected source occurrences without an automatic class quota; lineage=%#v", got, lineage)
	}
	if got := intFromAny(lineage["final_actual_memory_count"], 0); got != 1 {
		t.Fatalf("actual memory count = %d, want 1; lineage=%#v", got, lineage)
	}
	if got := intFromAny(lineage["pre_render_protected_duplicate_count"], 0); got != 0 {
		t.Fatalf("pre-render protected duplicate count = %d, want 0 because the identity rows come from different turns; lineage=%#v", got, lineage)
	}
	duplicates := sliceFromAny(lineage["pre_render_protected_duplicates"])
	if len(duplicates) != 0 {
		t.Fatalf("different-turn protected identities must not be traced as duplicates: %#v", duplicates)
	}
	if issues := strings.Join(stringsFromAny(lineage["known_issue_codes"]), ","); issues != "" {
		t.Fatalf("resolved protected-slot issue remained in lineage: %#v", lineage["known_issue_codes"])
	}
	memoryText := extractionStringFromAny(pack["memory_text"])
	if !strings.Contains(memoryText, "Mina secured the archive permit during the council hearing") {
		t.Fatalf("actual event row must survive into final memory text: %q", memoryText)
	}
	for _, leaked := range []string{"evacuation route", "watches the eastern gate", "council allegiance"} {
		if strings.Contains(memoryText, leaked) {
			t.Fatalf("protected source detail leaked into final text (%s): %q", leaked, memoryText)
		}
	}
	deliveredRows := map[int]bool{}
	for _, raw := range sliceFromAny(lineage["items"]) {
		item := mapFromAny(raw)
		if boolFromAny(item["delivered"]) {
			deliveredRows[intFromAny(item["source_row_id"], 0)] = true
			if item["source_table"] != "memories" {
				t.Fatalf("delivered lineage item lost store/vector provenance: %#v", item)
			}
			if boolFromAny(item["protected_guard"]) == boolFromAny(item["vector_hit"]) {
				t.Fatalf("only the public objective row may retain vector-hit provenance: %#v", item)
			}
		}
	}
	for _, id := range []int{11, 9, 2, 17, 12, 31} {
		if !deliveredRows[id] {
			t.Fatalf("source row %d missing from final delivery lineage: %#v", id, lineage["items"])
		}
	}
}
