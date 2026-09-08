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
		"memory_search_result": "ok",
		"search_result":        "ok",
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

func TestCoreObjectiveMemoryLimitDoesNotInferCrossLaneDuplicatesFromText(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		ActualMemoryText:         "Mina archived the brass key.\nMina archived the silver bell.",
		LatestDirectEvidenceText: "Mina archived the brass key.",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_core_objective_memory_max_items_present": true,
		"_core_objective_memory_max_items":         1,
	})
	core := mapFromAny(plan["core_objective_memory"])
	if intFromAny(core["eligible_distinct_count"], 0) != 2 ||
		intFromAny(core["delivered_count"], 0) != 1 ||
		intFromAny(core["deferred_by_limit_count"], 0) != 1 {
		t.Fatalf("coordinate-missing objective rows were merged by text before the explicit limit: %#v", core)
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mina archived the brass key.") || strings.Contains(finalText, "Mina archived the silver bell.") {
		t.Fatalf("explicit objective limit did not preserve first-row ordering after distinct treatment: %q", finalText)
	}
	if strings.Count(finalText, "Mina archived the brass key.") != 2 {
		t.Fatalf("coordinate-missing direct evidence was text-deduplicated: %q", finalText)
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
			ID: 1, ChatSessionID: "hierarchy-test",
			FromTurn: 1, ToTurn: 10, ChapterTitle: "Bridge Operation",
			SummaryText: "Luka prepares the bridge demolition plan.",
		},
		Arc: &store.ArcSummary{
			ID: 2, ChatSessionID: "hierarchy-test",
			FromTurn: 1, ToTurn: 30, ArcName: "River Campaign",
			CoreConflict: "The army must cross the river.",
		},
		Saga: &store.SagaDigest{
			ID: 3, ChatSessionID: "hierarchy-test",
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

func TestMemorySelectionRowsStayDistinctAndOnlyTheSameRowIsRenderedOnceAcrossLanes(t *testing.T) {
	first := store.Memory{ID: 1, ChatSessionID: "render-only-dedupe", TurnIndex: 4, SummaryJSON: `{"turn_summary":"same wording"}`}
	second := store.Memory{ID: 2, ChatSessionID: "render-only-dedupe", TurnIndex: 4, SummaryJSON: `{"turn_summary":"same wording"}`}
	selection := prepareTurnMemoryLaneSelection{
		Relevant: []store.Memory{first, second},
		Recent:   []store.Memory{first},
		Trace:    map[string]any{},
	}
	lines, trace := prepareTurnMemoryLaneLines(selection, nil, nil)
	if len(selection.Relevant) != 2 || len(selection.Recent) != 1 {
		t.Fatalf("rendering changed selected rows: relevant=%d recent=%d", len(selection.Relevant), len(selection.Recent))
	}
	if len(lines) != 2 {
		t.Fatalf("different DB rows with the same text were collapsed: %#v", lines)
	}
	if intFromAny(trace["final_render_duplicate_count"], 0) != 1 ||
		extractionStringFromAny(trace["final_render_dedup_scope"]) != "same_stored_memory_row_repeated_across_recall_lanes" {
		t.Fatalf("same-row render dedupe trace mismatch: %#v", trace)
	}
}

func TestMemoryExactOccurrenceKeyPreservesObservedOptionalSourcePointers(t *testing.T) {
	const sid = "exact-source-pointers"
	memory := func(id int64, extra string) store.Memory {
		return store.Memory{
			ID: id, ChatSessionID: sid, TurnIndex: 9,
			SummaryJSON: `{"turn_summary":"projection","source":{"source_revision":"rev-9","precise_memory_unit_id":"unit-9","content_hash":"hash-9","source_turn_start":9,"source_turn_end":9` + extra + `}}`,
		}
	}
	base := memory(1, "")
	baseReplay := memory(2, "")
	withOccurrence := memory(3, `,"source_occurrence_id":"occurrence-a"`)
	otherOccurrence := memory(4, `,"source_occurrence_id":"occurrence-b"`)
	withGeneration := memory(5, `,"generation_id":"generation-a"`)
	otherGeneration := memory(6, `,"generation_id":"generation-b"`)
	withMessage := memory(7, `,"source_message_id":"message-a"`)
	otherMessage := memory(8, `,"source_message_id":"message-b"`)
	baseKey := prepareTurnMemorySourceOccurrenceKey(base)
	if baseKey == "" || baseKey != prepareTurnMemorySourceOccurrenceKey(baseReplay) {
		t.Fatalf("identical observed source pointers did not produce the same exact key: %q", baseKey)
	}
	for _, pair := range [][2]store.Memory{
		{base, withOccurrence}, {withOccurrence, otherOccurrence},
		{base, withGeneration}, {withGeneration, otherGeneration},
		{base, withMessage}, {withMessage, otherMessage},
	} {
		if prepareTurnMemorySourceOccurrenceKey(pair[0]) == prepareTurnMemorySourceOccurrenceKey(pair[1]) {
			t.Fatalf("one-sided or mismatched optional source pointer was ignored: left=%s right=%s", pair[0].SummaryJSON, pair[1].SummaryJSON)
		}
	}
}

func TestMemoryExactOccurrenceKeyKeepsConflictingStructuredPointersRowDistinct(t *testing.T) {
	const sid = "exact-source-conflict"
	conflicting := func(id int64) store.Memory {
		return store.Memory{
			ID: id, ChatSessionID: sid, TurnIndex: 11,
			SummaryJSON: `{"turn_summary":"projection","source_revision":"outer-revision","source":{"source_revision":"inner-revision","precise_memory_unit_id":"unit-11","content_hash":"hash-11","source_turn_start":11,"source_turn_end":11}}`,
		}
	}
	firstKey := prepareTurnMemorySourceOccurrenceKey(conflicting(1))
	secondKey := prepareTurnMemorySourceOccurrenceKey(conflicting(2))
	if !strings.HasPrefix(firstKey, "memory-row:") || !strings.HasPrefix(secondKey, "memory-row:") || firstKey == secondKey {
		t.Fatalf("conflicting structured pointers did not fall back to distinct rows: first=%q second=%q", firstKey, secondKey)
	}
	aliasHash := store.Memory{
		ID: 3, ChatSessionID: sid, TurnIndex: 11,
		SummaryJSON: `{"turn_summary":"projection","source":{"source_revision":"revision-11","precise_memory_unit_id":"unit-11","content_hash":"hash-11","source_content_hash":"hash-11","source_turn_start":11,"source_turn_end":11}}`,
	}
	if key := prepareTurnMemorySourceOccurrenceKey(aliasHash); key == "" || strings.HasPrefix(key, "memory-row:") {
		t.Fatalf("equal explicit content-hash aliases were treated as conflicting: %q", key)
	}
	indexOnly := store.Memory{
		ID: 4, ChatSessionID: sid, TurnIndex: 11,
		SummaryJSON: `{"turn_summary":"projection","source":{"source_revision":"revision-11","precise_memory_unit_id":"unit-11","content_hash":"hash-11","source_turn_index":11}}`,
	}
	explicitRange := store.Memory{
		ID: 5, ChatSessionID: sid, TurnIndex: 11,
		SummaryJSON: `{"turn_summary":"projection","source":{"source_revision":"revision-11","precise_memory_unit_id":"unit-11","content_hash":"hash-11","source_turn_start":11,"source_turn_end":11}}`,
	}
	if left, right := prepareTurnMemorySourceOccurrenceKey(indexOnly), prepareTurnMemorySourceOccurrenceKey(explicitRange); left == "" || left != right {
		t.Fatalf("source_turn_index alias did not match the equivalent explicit range: index=%q range=%q", left, right)
	}
}

func TestPrepareTurnDoesNotCollapseDistinctDirectEvidenceRows(t *testing.T) {
	evidence := []store.DirectEvidence{
		{ID: 1, ChatSessionID: "evidence-preservation", EvidenceKind: "turn_excerpt", EvidenceText: "Mira opened the sealed gate.", SourceTurnStart: 7, SourceTurnEnd: 7, TurnAnchor: 7},
		{ID: 2, ChatSessionID: "evidence-preservation", EvidenceKind: "turn_excerpt", EvidenceText: "Mira broke the seal before opening the gate.", SourceTurnStart: 7, SourceTurnEnd: 7, TurnAnchor: 7},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, evidence, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		3, 9000, "Mira asks about the sealed gate.", "default", nil, nil, nil,
	)
	if got := intFromAny(assembly.Counts["evidence_count"], 0); got != len(evidence) {
		t.Fatalf("distinct evidence rows were collapsed before delivery: got=%d want=%d counts=%#v", got, len(evidence), assembly.Counts)
	}
	if _, exists := assembly.Counts["direct_evidence_delivery_dedupe"]; exists {
		t.Fatalf("selection-stage direct-evidence dedupe trace still exists: %#v", assembly.Counts["direct_evidence_delivery_dedupe"])
	}
}

func TestProtectedDeliveryGroupingKeepsSameKindArtifactsAtDifferentOrdinals(t *testing.T) {
	const source = `"source":{"source_revision":"rev-protected","precise_memory_unit_id":"unit-protected","content_hash":"hash-protected","source_turn_start":7,"source_turn_end":7}`
	item := store.Memory{
		ID: 71, ChatSessionID: "protected-artifact-ordinal", TurnIndex: 7,
		SummaryJSON: `{"turn_summary":"two distinct route secrets",` + source + `,"protected_secrets":[` +
			`{"owner":"Mira","secret_kind":"route","secret_summary":"north route","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mira"]}},` +
			`{"owner":"Mira","secret_kind":"route","secret_summary":"south route","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mira"]}}]}`,
	}
	groups, _ := buildPrepareTurnProtectedDeliveryGroups(prepareTurnMemoryLaneSelection{Relevant: []store.Memory{item}})
	count := 0
	for _, grouped := range groups {
		count += len(grouped)
	}
	if count != 2 {
		t.Fatalf("same-kind protected artifacts at separate array ordinals were merged: %#v", groups)
	}
	lines, _ := prepareTurnMemoryLaneLines(prepareTurnMemoryLaneSelection{Relevant: []store.Memory{item}}, nil, nil)
	if len(lines) != 2 {
		t.Fatalf("protected artifact render lost a distinct ordinal while preserving private text concealment: %#v", lines)
	}
}

func TestProtectedDeliveryGroupingRequiresTheSameOccurrenceFieldAndValue(t *testing.T) {
	const source = `"source":{"source_revision":"rev-protected-exact","precise_memory_unit_id":"unit-protected-exact","content_hash":"hash-protected-exact","source_turn_start":7,"source_turn_end":7}`
	memory := func(id int64, summary string) store.Memory {
		return store.Memory{
			ID: id, ChatSessionID: "protected-exact-artifact", TurnIndex: 7,
			SummaryJSON: `{"turn_summary":"projection",` + source + `,"protected_secrets":[` +
				`{"artifact_id":"route-secret","owner":"Mira","secret_kind":"route","secret_summary":"` + summary + `","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mira"]}}]}`,
		}
	}
	items := []store.Memory{memory(1, "north route"), memory(2, "north route"), memory(3, "south route")}
	groups, _ := buildPrepareTurnProtectedDeliveryGroups(prepareTurnMemoryLaneSelection{Relevant: items})
	count := 0
	for _, grouped := range groups {
		count += len(grouped)
	}
	if count != 2 {
		t.Fatalf("exact artifact duplicate was not grouped once or different value was lost: groups=%#v", groups)
	}
}
func TestProtectedDeliveryGroupingKeepsDifferentSourceCoordinates(t *testing.T) {
	items := []store.Memory{
		{ID: 1, ChatSessionID: "protected-dedupe", TurnIndex: 4, SummaryJSON: `{"turn_summary":"first","protected_secrets":[{"owner":"Mira","secret_kind":"route","secret_summary":"hidden route","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mira"]}}]}`},
		{ID: 2, ChatSessionID: "protected-dedupe", TurnIndex: 5, SummaryJSON: `{"turn_summary":"second","protected_secrets":[{"owner":"Mira","secret_kind":"route","secret_summary":"hidden route","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mira"]}}]}`},
	}
	groups, _ := buildPrepareTurnProtectedDeliveryGroups(prepareTurnMemoryLaneSelection{Relevant: items})
	count := 0
	for _, group := range groups {
		count += len(group)
	}
	if count != len(items) {
		t.Fatalf("different-turn protected memories were merged: %#v", groups)
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
