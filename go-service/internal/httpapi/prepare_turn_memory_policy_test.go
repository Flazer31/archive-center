package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCoreObjectiveMemoryLimitAppliesAfterSelectionWithoutConsumingSupportLanes(t *testing.T) {
	const sessionID = "memory-policy"
	memories := []store.Memory{
		{
			ID:            1,
			ChatSessionID: sessionID,
			TurnIndex:     1,
			SummaryJSON: `{
				"turn_summary":"Mina keeps a cover identity.",
				"character_identity_accuracy":[{
					"surface_identity_name":"Lia",
					"true_identity_name":"Mina",
					"canonical_entity_name":"Mina",
					"identity_kind":"cover_identity",
					"same_entity":true,
					"reveal_policy":"owner_private_until_revealed",
					"knowledge_scope":{"known_by":["Mina"]}
				}]
			}`,
			Importance: 0.95,
		},
		{ID: 2, ChatSessionID: sessionID, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina archived the brass key."}`, Importance: 0.9},
		{ID: 3, ChatSessionID: sessionID, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Mina archived the silver bell."}`, Importance: 0.8},
		{ID: 4, ChatSessionID: sessionID, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Mina archived the blue ledger."}`, Importance: 0.7},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"id": "memory:" + sessionID + ":1", "source_table": "memories", "source_row_id": "1", "similarity": 0.95},
			{"id": "memory:" + sessionID + ":2", "source_table": "memories", "source_row_id": "2", "similarity": 0.90},
			{"id": "memory:" + sessionID + ":3", "source_table": "memories", "source_row_id": "3", "similarity": 0.85},
			{"id": "memory:" + sessionID + ":4", "source_table": "memories", "source_row_id": "4", "similarity": 0.80},
		},
	}
	perspective := map[string]any{
		"current_pov": "Mina",
		"source":      "client_meta",
		"_core_objective_memory_max_items_present": true,
		"_core_objective_memory_max_items":         2,
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories,
		nil,
		[]store.DirectEvidence{{
			ID: 10, ChatSessionID: sessionID, EvidenceKind: "turn_excerpt",
			EvidenceText: "The brass key is on the desk.", TurnAnchor: 4,
		}},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		3, 12000,
		"Mina reviews the archived brass key, silver bell, and blue ledger.",
		"default", nil, vectorShadow, nil, perspective,
	)

	core := mapFromAny(assembly.MemoryDeliveryPlan["core_objective_memory"])
	if core["status"] != "active" ||
		intFromAny(core["requested_max_items"], 0) != 2 ||
		intFromAny(core["eligible_distinct_count"], 0) != 3 ||
		intFromAny(core["delivered_count"], 0) != 2 ||
		intFromAny(core["deferred_by_limit_count"], 0) != 1 ||
		core["garbage_fill"] != false ||
		core["top_k_reinterpreted"] != false {
		t.Fatalf("core objective contract mismatch: %#v", core)
	}
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	for _, required := range []string{
		"Mina archived the brass key.",
		"Mina archived the silver bell.",
		"The brass key is on the desk.",
		"Protected Memory Guidance",
	} {
		if !strings.Contains(finalText, required) {
			t.Fatalf("final delivery missing %q: %q", required, finalText)
		}
	}
	if strings.Contains(finalText, "Mina archived the blue ledger.") {
		t.Fatalf("core limit did not defer the third objective memory: %q", finalText)
	}
	lineage := assembly.MemoryDeliveryLineage
	if got := intFromAny(lineage["top_k_memory_target"], 0); got != 3 {
		t.Fatalf("vector candidate top_k changed to %d, want 3", got)
	}
	foundCoreDeferred := false
	foundProtectedExempt := false
	for _, raw := range prepareTurnMemoryLineageSlice(lineage["items"]) {
		item := mapFromAny(raw)
		switch intFromAny(item["source_row_id"], 0) {
		case 1:
			foundProtectedExempt = item["core_objective_k_consumption"] == "item_count_exempt_protected_guard"
		case 4:
			foundCoreDeferred = item["reason_code"] == "core_objective_memory_max_items" &&
				item["core_objective_k_consumption"] == "deferred_by_core_objective_limit"
		}
	}
	if !foundCoreDeferred || !foundProtectedExempt {
		t.Fatalf("lineage did not expose counted/exempt outcomes: %#v", lineage["items"])
	}
}

func TestCoreObjectiveMemoryLimitAppliesAfterCrossLaneDeduplication(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		ActualMemoryText:         "Mina archived the brass key.\nMina archived the silver bell.",
		LatestDirectEvidenceText: "Mina archived the brass key.",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_core_objective_memory_max_items_present": true,
		"_core_objective_memory_max_items":         1,
	})
	core := mapFromAny(plan["core_objective_memory"])
	if intFromAny(core["eligible_distinct_count"], 0) != 1 ||
		intFromAny(core["delivered_count"], 0) != 1 ||
		intFromAny(core["deferred_by_limit_count"], 0) != 0 {
		t.Fatalf("cross-lane duplicate consumed the objective limit: %#v", core)
	}
	finalText := extractionStringFromAny(plan["final_text"])
	for _, required := range []string{"Mina archived the brass key.", "Mina archived the silver bell."} {
		if !strings.Contains(finalText, required) {
			t.Fatalf("deduplicated plan missing %q: %q", required, finalText)
		}
	}
	if strings.Count(finalText, "Mina archived the brass key.") != 1 {
		t.Fatalf("direct-evidence duplicate remained in final delivery: %q", finalText)
	}
}

func TestPersonaRecollectionRequiresCurrentRequestOrAcceptedSceneRelevance(t *testing.T) {
	entries := []store.PersonaMemoryEntry{
		{ID: 1, MemoryText: "Chloe hid the brass key behind the cracked mirror.", Importance10: 8},
		{ID: 2, MemoryText: "A desert caravan crossed the southern dunes.", Importance10: 9},
		{ID: 3, MemoryText: "A violin concerto ended at the opera house.", TagsJSON: `["owner_entity_name:Chloe","raw_owner_entity_name:Chloe","owner_entity_role:npc","owner_visibility:owner_private","subjective_entity_memory"]`, Importance10: 10},
	}
	logs := []store.ChatLog{
		{TurnIndex: 4, Role: "assistant", Content: "Chloe pauses beside the cracked mirror."},
	}
	selected, trace := filterPrepareTurnPersonaRecollections(
		"Look around the room.", nil, nil, nil, nil, entries, logs,
	)
	if len(selected) != 1 || selected[0].ID != 1 {
		t.Fatalf("persona relevance selected %#v, want only the accepted-scene recollection; trace=%#v", selected, trace)
	}
	if intFromAny(trace["deferred_count"], 0) != 2 || trace["garbage_fill"] != false {
		t.Fatalf("persona relevance trace mismatch: %#v", trace)
	}

	koreanSelected, _ := filterPrepareTurnPersonaRecollections(
		"책상을 이상하게 느꼈다.", nil, nil, nil, nil,
		[]store.PersonaMemoryEntry{{ID: 3, MemoryText: "책상 안에 낡은 열쇠가 숨겨져 있었다."}},
	)
	if len(koreanSelected) != 1 {
		t.Fatalf("Korean inflection-aware relevance dropped a related recollection: %#v", koreanSelected)
	}
}

func TestHierarchyZoomSelectsOnlyTheNarrowestRelevantResolution(t *testing.T) {
	resume := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			FromTurn: 1, ToTurn: 10, ChapterTitle: "Bridge Operation",
			SummaryText: "Luka prepares the bridge demolition plan.",
		},
		Arc: &store.ArcSummary{
			FromTurn: 1, ToTurn: 30, ArcName: "River Campaign",
			CoreConflict: "The army must cross the river.",
		},
		Saga: &store.SagaDigest{
			FromTurn: 1, ToTurn: 80, EraLabel: "Northern War",
			SagaSummary: "The kingdom endures a northern invasion.",
		},
	}
	relevant := buildPrepareTurnHierarchyEscalation(
		resume, nil, prepareTurnMemoryLaneSelection{},
		"Continue Luka's bridge demolition plan.", "default",
	)
	if relevant.ChapterText == "" || relevant.ArcText != "" || relevant.SagaText != "" {
		t.Fatalf("hierarchy zoom did not keep the narrowest relevant level: %#v", relevant)
	}
	if mapFromAny(relevant.Trace)["selected_resolution"] != "chapter" {
		t.Fatalf("hierarchy selected resolution mismatch: %#v", relevant.Trace)
	}

	unrelated := buildPrepareTurnHierarchyEscalation(
		resume, nil, prepareTurnMemoryLaneSelection{},
		"Describe the tea cup on the desk.", "default",
	)
	if unrelated.ChapterText != "" || unrelated.ArcText != "" || unrelated.SagaText != "" ||
		mapFromAny(unrelated.Trace)["status"] != "no_support" {
		t.Fatalf("unrelated hierarchy was used as count filler: %#v", unrelated)
	}
}

func TestTurnWorkflowHUDMemorySelectionDoesNotExposeProtectedOrDeferredText(t *testing.T) {
	lineage := map[string]any{
		"status":              "mixed",
		"top_k_memory_target": 5,
		"items": []map[string]any{
			{
				"source_row_id": 1, "turn_index": 2, "selection_lane": "vector_relevant",
				"delivered": true, "reason_code": "selected_within_final_delivery_plan",
				"protected_guard": true, "final_text": "never expose the hidden route",
			},
			{
				"source_row_id": 2, "turn_index": 3, "selection_lane": "relevant",
				"delivered": false, "reason_code": "core_objective_memory_max_items",
				"protected_guard": false, "final_text": "deferred private candidate text",
			},
		},
	}
	plan := map[string]any{
		"core_objective_memory": map[string]any{
			"contract_version": "core_objective_memory_delivery.v1",
			"status":           "active", "requested_max_items": 2, "eligible_distinct_count": 3,
			"delivered_count": 2, "deferred_by_limit_count": 1, "deferred_by_budget_count": 0,
			"missing_to_limit": 0, "deferred_fact_keys": []string{"deferred private candidate text"},
			"garbage_fill": false, "top_k_reinterpreted": false,
		},
		"classes": []map[string]any{{
			"key": "event_recent", "eligible_count": 3, "selected_count": 2,
			"deferred_count": 1, "deduplicated_count": 0,
		}},
	}
	view := buildTurnWorkflowHUDMemorySelection(lineage, plan)
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal HUD memory selection: %v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{
		"never expose the hidden route",
		"deferred private candidate text",
		"deferred_fact_keys",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("HUD memory selection exposed protected/deferred text %q: %s", forbidden, text)
		}
	}
	if view.PrivateTextExposed || len(view.Lanes) != 1 ||
		view.DeliveredCount != 1 || view.DeferredCount != 1 ||
		view.ExclusionReasons["core_objective_memory_max_items"] != 1 {
		t.Fatalf("HUD memory selection facts mismatch: %#v", view)
	}
}
