package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Read the model-visible source dictionary, asserting every link is present.
// No production selection or rendering is replaced by this fixture reader.
func modelEvidenceForTest(t *testing.T, input map[string]any, raw any) map[string]any {
	t.Helper()
	item := mapFromAny(raw)
	out := map[string]any{}
	for k, v := range item {
		out[k] = v
	}
	if ref := extractionStringFromAny(item["source"]); ref != "" {
		source := mapFromAny(mapFromAny(input["source_catalog"])[ref])
		if len(source) == 0 {
			t.Fatalf("model received a dangling source reference %q", ref)
		}
		for k, v := range source {
			out[k] = v
		}
	}
	return out
}

func Test43SearchQuestionObjectsPreserveIndependentFields(t *testing.T) {
	r, err := parseMultiAgentRecommendation(`{"selected_ids":["F2","F1"],"search_requests":[{"question":"When was the key delivered?"},{"query":"Who received it?"},"Where is it now?"],"unresolved":["recipient's location"]}`)
	if err != nil || !r.formatRepaired || !reflect.DeepEqual(r.SearchRequests, []string{"When was the key delivered?", "Who received it?", "Where is it now?"}) || !reflect.DeepEqual(r.SelectedIDs, []string{"F2", "F1"}) || len(r.Unresolved) != 1 {
		t.Fatalf("observed question objects lost usable results: %+v %v", r, err)
	}
	r, err = parseMultiAgentRecommendation(`{"search_requests":[{"question":42},"valid later question",{"query":"another question"}],"selected_ids":["F2","F1"]}`)
	if err == nil || !reflect.DeepEqual(r.SearchRequests, []string{"valid later question", "another question"}) || !reflect.DeepEqual(r.SelectedIDs, []string{"F2", "F1"}) {
		t.Fatalf("one invalid question blocked independent entries: %+v %v", r, err)
	}
	r, err = parseMultiAgentRecommendation(`{"selected_ids":[{"question":"not a memory ID"},"F1"],"search_requests":[]}`)
	if err == nil || !reflect.DeepEqual(r.SelectedIDs, []string{"F1"}) {
		t.Fatalf("question decoding changed memory ID semantics: %+v %v", r, err)
	}
}

func Test43RoleOrderUsesOwningRecommendation(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "goal-b", Lane: "unresolved_goal"},
		{CanonicalFactID: "event-a", Lane: "event_recent"},
		{CanonicalFactID: "goal-a", Lane: "unresolved_goal"},
		{CanonicalFactID: "state-b", Lane: "world_state"},
		{CanonicalFactID: "event-b", Lane: "event_recent"},
		{CanonicalFactID: "state-a", Lane: "world_state"},
	}
	summaries := []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "summary-a"}, {SummaryID: "summary-b"}}
	selection := &multiAgentSelection{Roles: []multiAgentRoleResult{
		{Role: "world_state", Source: "go_default", Selection: multiAgentRecommendation{SelectedIDs: []string{"state-a", "state-b"}, SelectedSummaryIDs: []string{"summary-a", "summary-b"}}},
		{Role: "event_recent", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: []string{"event-b", "event-a", "goal-a"}, SelectedSummaryIDs: []string{"summary-b", "summary-a"}}},
		{Role: "unresolved_goal", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: []string{"goal-a", "goal-b"}}},
	}}
	multiAgentOrderCandidates(selection, facts, summaries)
	got := []string{}
	for _, f := range facts {
		got = append(got, f.CanonicalFactID)
	}
	if !reflect.DeepEqual(got, []string{"goal-a", "event-b", "goal-b", "state-b", "event-a", "state-a"}) || summaries[0].SummaryID != "summary-b" {
		t.Fatalf("another role changed the owner's order or Go baseline: %v %+v", got, summaries)
	}
}

func Test43CompactNoteCatalogPreservesExactScope(t *testing.T) {
	selection := &multiAgentSelection{}
	planItems := []map[string]any{}
	for _, role := range []string{"event_recent", "subjective_relationship"} {
		r := multiAgentRoleResult{Role: role, Source: "ai", SelectionRound: 2, Selection: multiAgentRecommendation{Reasons: map[string]string{}, Unresolved: []string{"Open timing for " + role, "Open location for " + role}}}
		inputItems := []map[string]any{}
		for i := 0; i < 12; i++ {
			id := fmt.Sprintf("%s-%d", role, i)
			item := map[string]any{"canonical_fact_id": id, "selection_status": "selected", "source_table": "precise_memory_facts", "source_ref": "precise_memory_facts:" + id, "source_turn": i + 1, "visibility": "owner_private", "perspective_owner": fmt.Sprintf("Reader%d", i), "allowed_viewers": []string{fmt.Sprintf("Reader%d", i)}}
			if i == 0 {
				item["allowed_viewers"] = nil
			} else if i == 1 {
				delete(item, "allowed_viewers")
			}
			planItems = append(planItems, item)
			inputItems = append(inputItems, item)
			r.Selection.SelectedIDs = append(r.Selection.SelectedIDs, id)
			r.Selection.Reasons[id] = "Recorded detail and possible relevance: " + id
		}
		r.Calls = []multiAgentCall{{Round: 2, Input: map[string]any{"candidates": inputItems}}}
		selection.Roles = append(selection.Roles, r)
	}
	notes := buildPrepareTurnPreprocessingNotes(selection, map[string]any{"priority_items": planItems}, nil)
	text := extractionStringFromAny(notes["final_text"])
	marker := "Source scope catalog: "
	_, rest, found := strings.Cut(text, marker)
	if !found {
		t.Fatal("missing readable catalog")
	}
	line, _, _ := strings.Cut(rest, "\n")
	var compact map[string]map[string]any
	if err := json.Unmarshal([]byte(line), &compact); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"t": "source_table", "n": "source_turn", "v": "visibility", "o": "perspective_owner", "a": "allowed_viewers", "r": "source_refs"}
	expanded := map[string]any{}
	for ref, row := range compact {
		x := map[string]any{}
		for key, value := range row {
			full, ok := keys[key]
			if !ok {
				t.Fatalf("uncompacted or undocumented scope key: %s", key)
			}
			x[full] = value
		}
		expanded[ref] = x
	}
	want, _ := json.Marshal(notes["source_catalog"])
	got, _ := json.Marshal(expanded)
	if string(want) != string(got) || len(line) >= len(want) {
		t.Fatal("compaction changed scope, absence/null, provenance or failed to save space")
	}
	for _, role := range selection.Roles {
		for _, note := range role.Selection.Reasons {
			if strings.Count(text, note+"\n") != 1 && !strings.HasSuffix(text, note) {
				t.Fatalf("reason changed, repeated or dropped: %s", note)
			}
		}
		for _, note := range role.Selection.Unresolved {
			if strings.Count(text, note) != 1 {
				t.Fatalf("uncertainty changed, repeated or dropped: %s", note)
			}
		}
	}
	for _, raw := range outputFidelityLineageSlice(notes["items"]) {
		item := mapFromAny(raw)
		for _, ref := range stringsFromAny(item["scope_refs"]) {
			if compact[ref] == nil {
				t.Fatalf("note lost scope %s", ref)
			}
		}
	}
}

func Test43ModelInputReadingOrderAndSourcePacking(t *testing.T) {
	current := "The user establishes that Mira has no practical farming experience."
	facts := []prepareTurnPriorityMemoryCandidate{}
	for i := 0; i < 20; i++ {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: fmt.Sprintf("fact-%d", i), Lane: "character_objective", SourceTable: "character_states", SourceRef: "character_states:12", SourceTurn: 117, CompleteText: fmt.Sprintf("Recorded item %d <original>.", i), Visibility: "owner_private", PerspectiveOwner: "Mira", AllowedViewers: []string{"Mira"}})
	}
	cfg := defaultMultiAgentSettings()
	input := multiAgentInput("character_objective", facts, nil, dto.PrepareTurnRequest{RawUserInput: &current, Messages: []map[string]any{{"role": "user", "content": "Receive the tools."}, {"role": "assistant", "content": "The tools are delivered; the hour is unstated."}}}, cfg, 12000, 8, nil)
	input["previous_result"] = multiAgentRecommendation{SelectedIDs: []string{"fact-1", "fact-0"}, Reasons: map[string]string{"fact-1": "Earlier interpretation remains attributed."}}
	before, _ := json.Marshal(input)
	var wire string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		wire = extractionStringFromAny(mapFromAny(outputFidelityLineageSlice(body["messages"])[1])["content"])
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"selected_ids":["F2","F1"],"reasons":{"F2":"Earlier interpretation remains attributed."}}`}}}})
	}))
	defer provider.Close()
	c := cfg.Roles["character_objective"]
	c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
	cfg.Roles["character_objective"] = c
	call := (&Server{}).callMultiAgent(context.Background(), "character_objective", cfg, 2, input)
	if call.Error != "" || !reflect.DeepEqual(call.Result.SelectedIDs, []string{"fact-1", "fact-0"}) {
		t.Fatalf("selection/reference contract changed: %+v", call.Result)
	}
	if strings.Index(wire, `"current_input"`) > strings.Index(wire, `"candidates"`) || strings.Index(wire, `"recent_conversation"`) > strings.Index(wire, `"candidates"`) {
		t.Error("candidate mass precedes the current scene")
	}
	var packed map[string]any
	_ = json.Unmarshal([]byte(wire), &packed)
	if strings.Count(wire, "character_states:12") != 1 || packed["source_catalog"] == nil {
		t.Error("identical source metadata was repeated rather than shared")
	}
	catalog := mapFromAny(packed["source_catalog"])
	items := outputFidelityLineageSlice(packed["candidates"])
	if len(items) != len(facts) {
		t.Fatal("candidate count changed")
	}
	for i, raw := range items {
		item := mapFromAny(raw)
		source := mapFromAny(catalog[extractionStringFromAny(item["source"])])
		if item["text"] != facts[i].CompleteText || item["id"] != facts[i].CanonicalFactID || source["perspective_owner"] != "Mira" || !reflect.DeepEqual(stringsFromAny(source["allowed_viewers"]), []string{"Mira"}) {
			t.Error("source text, identity, order or private scope changed")
		}
	}
	if len([]rune(wire)) >= len([]rune(string(before))) {
		t.Error("source packing did not reduce this repeated-source input")
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("model presentation mutated canonical input")
	}
}

func Test43PreprocessingFactNotesHaveDistinctStableReferences(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "watch", Lane: "unresolved_goal", SourceRef: "pending_threads:1", SourceTable: "pending_threads", Visibility: "general", CompleteText: "The ruler will observe Mira's work."},
		{CanonicalFactID: "craft", Lane: "unresolved_goal", SourceRef: "pending_threads:1", SourceTable: "pending_threads", Visibility: "general", CompleteText: "The carpenter plans to make the axle."},
	}
	in := multiAgentInput("unresolved_goal", facts, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 2000, 8, nil)
	sel := &multiAgentSelection{Candidates: facts, Roles: []multiAgentRoleResult{{Role: "unresolved_goal", Source: "ai", SelectionRound: 1, Calls: []multiAgentCall{{Round: 1, Input: in}}, Selection: multiAgentRecommendation{SelectedIDs: []string{"watch", "craft"}, Reasons: map[string]string{"watch": "The carpenter's axle still needs checking.", "craft": "The earlier manufacturing plan."}}}}}
	assembly := prepareTurnInjectionAssembly{Preprocessing: sel}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 2000, 8, "auto", nil, nil)
	notes := buildPrepareTurnPreprocessingNotes(sel, plan, nil)
	memory, note := extractionStringFromAny(plan["final_text"]), extractionStringFromAny(notes["final_text"])
	for _, ref := range []string{"F1", "F2"} {
		if !strings.Contains(memory, "["+ref+"]") || !strings.Contains(note, "["+ref+"]") {
			t.Errorf("%s does not connect the individual evidence to its interpretation", ref)
		}
	}
	if !strings.Contains(note, sel.Roles[0].Selection.Reasons["watch"]) {
		t.Fatal("Go silently rewrote or rejected a mistaken AI interpretation")
	}
	if strings.Count(note, "pending_threads:1") != 1 {
		t.Error("same source metadata repeated in final notes")
	}
	linked := 0
	for _, item := range supervisorDeliveredContextItems(plan, nil, nil) {
		if strings.Contains(extractionStringFromAny(item["final_text"]), "[F") && item["source_ref"] != facts[0].SourceRef {
			t.Fatal("memory labels disconnected Publisher from the original source")
		}
		if strings.Contains(extractionStringFromAny(item["final_text"]), "[F") {
			linked++
		}
	}
	if linked != len(facts) {
		t.Fatal("Publisher never received both labeled facts")
	}
}

func Test43PackedSourcesKeepDistinctKnowledgeHolders(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{}
	for _, holder := range []string{"Mira", "Rook"} {
		for _, text := range []string{"Knows the disclosed identity.", "Remembers the disclosure."} {
			id := holder + text
			facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: "subjective_relationship", SourceRef: "private:shared", SourceTable: "protagonist_entity_memories", SourceTurn: 17, CompleteText: text, Visibility: "owner_private", PerspectiveOwner: holder, AllowedViewers: []string{holder}})
		}
	}
	in := multiAgentInput("subjective_relationship", facts, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 2000, 8, nil)
	var packed map[string]any
	_ = json.Unmarshal([]byte(multiAgentModelInput(in, 1)), &packed)
	if len(mapFromAny(packed["source_catalog"])) != 2 {
		t.Fatal("different knowledge holders were merged")
	}
	for i, raw := range outputFidelityLineageSlice(packed["candidates"]) {
		item := modelEvidenceForTest(t, packed, raw)
		if item["perspective_owner"] != facts[i].PerspectiveOwner || !reflect.DeepEqual(stringsFromAny(item["allowed_viewers"]), facts[i].AllowedViewers) {
			t.Fatal("holder or permitted readers changed")
		}
	}
}

func Test43CompleteSummaryRefReachesPublisher(t *testing.T) {
	summary := prepareTurnPriorityTurnSummaryCandidate{SummaryID: "summary", SourceRef: "memories:4", SourceTurn: 4, CompleteText: "Mira received the key and disclosed its purpose to Rook."}
	in := multiAgentInput("event_recent", nil, []prepareTurnPriorityTurnSummaryCandidate{summary}, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 2000, 8, nil)
	sel := &multiAgentSelection{Summaries: []prepareTurnPriorityTurnSummaryCandidate{summary}, Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", SelectionRound: 1, Calls: []multiAgentCall{{Round: 1, Input: in}}, Selection: multiAgentRecommendation{SelectedSummaryIDs: []string{summary.SummaryID}, Reasons: map[string]string{summary.SummaryID: "The completed disclosure explains Rook's knowledge."}}}}}
	assembly := prepareTurnInjectionAssembly{Preprocessing: sel}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 2000, 8, "auto", nil, nil)
	notes := buildPrepareTurnPreprocessingNotes(sel, plan, nil)
	if !strings.Contains(extractionStringFromAny(plan["final_text"]), "[S1]") || !strings.Contains(extractionStringFromAny(notes["final_text"]), "[S1]") {
		t.Fatal("summary and interpretation lost their shared ref")
	}
	found := false
	for _, item := range supervisorDeliveredContextItems(plan, nil, nil) {
		if strings.Contains(extractionStringFromAny(item["final_text"]), summary.CompleteText) {
			found = true
			if item["source_ref"] != summary.SourceRef {
				t.Fatal("Publisher lost the complete summary source")
			}
		}
	}
	if !found {
		t.Fatal("Publisher did not receive the complete summary")
	}
}

func Test43PreprocessingNotesFollowAcceptedRoundAndScope(t *testing.T) {
	for _, tc := range []struct {
		name, second, want, absent string
		fail                       bool
		ids                        []string
	}{
		{"supplement", `{"selected_ids":["F1"],"reasons":{"F1":"SECOND_NOTE"},"unresolved":["SECOND_QUESTION"]}`, "SECOND_NOTE", "FIRST_NOTE", false, []string{"fact-a"}},
		{"object_question_supplement", `{"selected_ids":["F1"],"reasons":{"F1":"SECOND_NOTE"},"search_requests":[{"query":"source time still unknown"}]}`, "SECOND_NOTE", "FIRST_NOTE", false, []string{"fact-a"}},
		{"failed_supplement", "", "FIRST_NOTE", "SECOND_NOTE", true, []string{"fact-b", "fact-a"}},
		{"partial_supplement_retains_first", `{"selected_ids":["F1"],"reasons":{"F1":"SECOND_NOTE"},"related_requests":false}`, "FIRST_NOTE", "SECOND_NOTE", false, []string{"fact-b", "fact-a"}},
		{"empty_final_go_selection", `{}`, "", "FIRST_NOTE", false, []string{"fact-a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				messages := outputFidelityLineageSlice(body["messages"])
				var input map[string]any
				_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input)
				answer := `{"selected_ids":["F2","F1"],"reasons":{"F2":"FIRST_NOTE_B","F1":"FIRST_NOTE_A"},"unresolved":["FIRST_QUESTION"],"search_requests":["source timing"]}`
				if input["previous_result"] != nil {
					if tc.fail {
						http.Error(w, "fixture failure", http.StatusBadRequest)
						return
					}
					answer = tc.second
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled = role == "subjective_relationship"
				c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
				cfg.Roles[role] = c
			}
			facts := []prepareTurnPriorityMemoryCandidate{
				{CanonicalFactID: "fact-a", Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", SourceRef: "private:a", CompleteText: "Original memory A", Visibility: "owner_private", PerspectiveOwner: "Mira", AllowedViewers: []string{"Mira"}},
				{CanonicalFactID: "fact-b", Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", SourceRef: "private:b", CompleteText: "Original memory B", Visibility: "owner_private", PerspectiveOwner: "Rook", AllowedViewers: []string{"Rook"}},
			}
			selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil,
				func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
					return nil, nil, nil
				})
			selection.BaselineIDs = map[string]bool{"fact-a": true}
			assembly := prepareTurnInjectionAssembly{Preprocessing: selection}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 2000, 5, "auto", nil, nil)
			if got := stringsFromAny(plan["selected_fact_ids"]); !reflect.DeepEqual(got, tc.ids) {
				t.Fatalf("existing selection/order changed: %v want %v", got, tc.ids)
			}
			notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
			text := extractionStringFromAny(notes["final_text"])
			if calls != 2 || strings.Contains(text, tc.absent) || (tc.want != "" && !strings.Contains(text, tc.want)) {
				t.Fatalf("wrong accepted interpretation: calls=%d text=%s", calls, text)
			}
			if tc.want == "" && text != "" {
				t.Fatal("Go baseline selection fabricated specialist interpretation")
			}
			if tc.want != "" && (!strings.Contains(text, "owner_private") || !strings.Contains(text, "Mira") || !strings.Contains(text, "allowed_viewers")) {
				t.Fatal("private source scope was lost")
			}
			if len(tc.ids) == 2 && strings.Index(text, "FIRST_NOTE_B") >= strings.Index(text, "FIRST_NOTE_A") {
				t.Fatal("interpretations changed AI recommendation order")
			}
			if strings.Contains(extractionStringFromAny(plan["final_text"]), "NOTE") || strings.Contains(text, "Original memory") {
				t.Fatal("source and interpretation were mixed")
			}
			for _, publisher := range []string{"disabled", "failed_open", "succeeded"} {
				payload := buildPrepareTurnPayloadApplicationPlan("user input", "", extractionStringFromAny(plan["final_text"]), "", true, true, 2000, 0, 0, nil, publisher, notes)
				attachPrepareTurnLorebookReferenceLane(payload, "Lore reference", 100, true, []string{"lore-ref"})
				if tc.want != "" && !strings.Contains(extractionStringFromAny(payload["auxiliary_text"]), tc.want) {
					t.Fatal("Publisher state removed specialist notes")
				}
				order := stringsFromAny(payload["lane_order"])
				lanes := outputFidelityLineageSlice(payload["lanes"])
				for i, raw := range lanes {
					if order[i] != extractionStringFromAny(mapFromAny(raw)["key"]) {
						t.Fatal("lore insertion lost the new lane order")
					}
				}
			}
		})
	}
	if buildPrepareTurnPreprocessingNotes(nil, nil, nil) != nil {
		t.Fatal("OFF created specialist notes")
	}
}

func Test43PreprocessingNotesKeepIndependentLoreRound(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		messages := outputFidelityLineageSlice(body["messages"])
		var input map[string]any
		if err := json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input); err != nil {
			t.Error(err)
			return
		}
		answer := `{"selected_ids":["F1"],"selected_lorebook_refs":["L1"],"reasons":{"F1":"FIRST_MEMORY","L1":"FIRST_LORE"},"search_requests":["source context"]}`
		if input["previous_result"] != nil {
			answer = `{"selected_ids":["F2"],"selected_lorebook_refs":["L2"],"reasons":{"F2":"SECOND_MEMORY","L2":"SECOND_LORE"},"related_requests":false}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = role == "world_state"
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
		cfg.Roles[role] = c
	}
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "fact-a", Lane: "world_state", SourceRef: "world:a", CompleteText: "Original world A", Visibility: "public"},
		{CanonicalFactID: "fact-b", Lane: "world_state", SourceRef: "world:b", CompleteText: "Original world B", Visibility: "public"},
	}
	loreContext := map[string]any{"lorebook_candidates": []map[string]any{{"id": "lore-a", "text": "Lore A"}, {"id": "lore-b", "text": "Lore B"}}, "lorebook_budget_chars": 100}
	selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, nil, loreContext)
	assembly := prepareTurnInjectionAssembly{Preprocessing: selection}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 2000, 5, "auto", nil, nil)
	lore := prepareTurnLorebookReferenceResult{delivered: []prepareTurnLorebookDeliveredItem{{SourceRefs: []string{"lore-b"}}}}
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, &lore)
	items := outputFidelityLineageSlice(notes["items"])
	text := extractionStringFromAny(notes["final_text"])
	if selection.AnalysisCalls != 2 || len(items) != 2 || !strings.Contains(text, "FIRST_MEMORY") || !strings.Contains(text, "SECOND_LORE") || strings.Contains(text, "SECOND_MEMORY") || strings.Contains(text, "FIRST_LORE") {
		t.Fatalf("mixed-round selection lost its matching interpretation: %s", text)
	}
	if intFromAny(mapFromAny(items[0])["round"], 0) != 1 || intFromAny(mapFromAny(items[1])["round"], 0) != 2 {
		t.Fatalf("independent memory/lore analysis round lost: %+v", items)
	}
}

func Test43PendingGoalUsesConfiguredRecentContext(t *testing.T) {
	raw := "Mira starts the next job."
	goal := "Restore village pumping station before the dry season"
	threads := []store.PendingThread{
		{ID: 11, Description: goal, Status: "open", SourceTurn: 4},
		{ID: 12, Description: "Investigate mountain pass smuggling", Status: "open", SourceTurn: 2},
	}
	for _, withRecent := range []bool{false, true} {
		queries := []string{raw}
		if withRecent {
			queries = append(queries, "user: Let's restore village pumping station.\nassistant: The pump parts have arrived; restoration is the next job.")
		}
		perspective := map[string]any{"_priority_memory_enabled": true, "_priority_memory_query": raw, prepareTurnPriorityQuerySetContextKey: queries}
		assembly := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, nil, threads, nil, nil, nil, nil, nil, 5, 12000, raw, "default", nil, nil, nil, perspective)
		if strings.Contains(assembly.PendingThreadText, goal) != withRecent || strings.Contains(assembly.PendingThreadText, threads[1].Description) {
			t.Fatalf("configured recent context did not reach goal candidates: recent=%v text=%q", withRecent, assembly.PendingThreadText)
		}
		if withRecent {
			facts, _ := multiAgentCandidatePool(&assembly, perspective)
			input := multiAgentInput("unresolved_goal", facts, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 12000, 5, map[string]int{})
			items := input["candidates"].([]map[string]any)
			found := false
			for _, item := range items {
				if strings.Contains(extractionStringFromAny(item["text"]), goal) && item["source_table"] == "pending_threads" {
					found = true
				}
			}
			if !found {
				t.Fatalf("selected goal never reached the specialist input: %+v", input)
			}
		}
	}
}

func Test43LorebookLabelsStayWithExactReferences(t *testing.T) {
	r := newPrepareTurnLorebookReferenceResult(prepareTurnLorebookModeReferenceAssist)
	r.ScopeStatus, r.Status = "observed", "ready"
	r.candidates = []prepareTurnLorebookCandidate{
		{EntryRef: "lore-official", ContextOverlap: 1, Entry: store.LorebookReferenceEntryObservation{Comment: "The official", Content: "Original official biography."}},
		{EntryRef: "lore-craftsperson", ContextOverlap: 1, Entry: store.LorebookReferenceEntryObservation{Content: "### The craftsperson\nOriginal private biography."}},
	}
	lore := prepareTurnLorebookPreprocessingCandidates(r)
	input := multiAgentInput("world_state", nil, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 12000, 5, map[string]int{}, map[string]any{"lorebook_candidates": lore})
	items := input["lorebook_candidates"].([]map[string]any)
	for i, label := range []string{"L1 · The official", "L2 · The craftsperson"} {
		if items[i]["label"] != label || items[i]["id"] != r.candidates[i].EntryRef || items[i]["text"] != r.candidates[i].Entry.Content {
			t.Fatalf("name/reference/source mismatch: %+v", items[i])
		}
	}
}

func Test43MultiAgentInputKeepsWholeFactsAndSummariesWithinSharedCap(t *testing.T) {
	for _, summaryChars := range []int{10, 30, 60} {
		t.Run(fmt.Sprintf("summary_%d_chars", summaryChars), func(t *testing.T) {
			cfg := defaultMultiAgentSettings()
			cfg.CandidateChars = 80
			facts := make([]prepareTurnPriorityMemoryCandidate, 12)
			for i := range facts {
				facts[i] = prepareTurnPriorityMemoryCandidate{CanonicalFactID: fmt.Sprintf("fact-%d", i), Lane: "event_recent", CompleteText: strings.Repeat("사", 10), SourceTurn: i + 1}
			}
			summaries := []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "summary-completed-visit", CompleteText: strings.Repeat("요", summaryChars), SourceTurn: 12}}
			input := multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, cfg, 160, 3, map[string]int{"event_recent": 70})
			gotFacts := input["candidates"].([]map[string]any)
			gotSummaries := input["turn_summaries"].([]map[string]any)
			if len(gotFacts) == 0 || len(gotSummaries) != 1 {
				t.Fatalf("facts consumed the summary window: facts=%d summaries=%d", len(gotFacts), len(gotSummaries))
			}
			chars := len(gotFacts)*10 + summaryChars
			if input["input_candidate_chars"] != chars || chars > cfg.CandidateChars || cfg.CandidateChars-chars >= 10 {
				t.Fatalf("shared cap was exceeded or reusable space was lost: %+v", input)
			}
			for i, item := range gotFacts {
				if item["text"] != facts[i].CompleteText || item["id"] != facts[i].CanonicalFactID || item["source_turn"] != facts[i].SourceTurn {
					t.Fatal("candidate source order, text or source turn changed")
				}
			}
			if gotSummaries[0]["text"] != summaries[0].CompleteText || gotSummaries[0]["ref"] != "S1" || gotFacts[0]["ref"] != "F1" {
				t.Fatal("whole summary or typed reference missing")
			}
			counts := input["candidate_counts"].(map[string]int)
			if counts["facts_available"] != len(facts) || counts["facts_supplied"] != len(gotFacts) || counts["turn_summaries_supplied"] != 1 {
				t.Fatal("candidate availability was confused with supplied input")
			}
		})
	}
}

func Test43MultiAgentExactReferencesReachBothRounds(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "pmf_canonical_one", Lane: "event_recent", CompleteText: "A visit was planned."},
		{CanonicalFactID: "pmf_canonical_two", Lane: "event_recent", CompleteText: "The visit was completed."},
	}
	summaries := []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "pms_canonical_one", CompleteText: "The visit progressed from planning to completion."}}
	var firstRef string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(payload.Messages[1].Content), &input); err != nil {
			t.Error(err)
			return
		}
		byID := map[string]string{}
		for _, group := range []string{"candidates", "turn_summaries"} {
			for _, raw := range input[group].([]any) {
				item := raw.(map[string]any)
				byID[item["id"].(string)] = item["ref"].(string)
			}
		}
		var result multiAgentRecommendation
		if _, second := input["previous_result"]; !second {
			firstRef = byID[facts[1].CanonicalFactID]
			result.SelectedIDs = []string{firstRef}
			result.SelectedSummaryIDs = []string{byID[summaries[0].SummaryID]}
			result.SearchRequests = []string{"What followed the completed visit?"}
		} else {
			if byID[facts[1].CanonicalFactID] != firstRef || byID["pmf_new_source"] == firstRef {
				t.Error("short reference changed when supplemental candidates reordered")
			}
			previous := input["previous_result"].(map[string]any)
			if previous["selected_ids"].([]any)[0] != firstRef {
				t.Error("reviewed first recommendation lost its stable short reference")
			}
			result.SelectedIDs = []string{byID["pmf_new_source"], firstRef}
			result.SelectedSummaryIDs = []string{byID["pms_new_source"], summaries[0].SummaryID}
			result.Reasons = map[string]string{firstRef: "Preserve the completed visit as past context."}
		}
		body, _ := json.Marshal(result)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(body)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = role == "event_recent"
		c.Provider, c.Endpoint, c.APIKey, c.Model = "custom", provider.URL, "fixture-key", "fixture-model"
		cfg.Roles[role] = c
	}
	server := &Server{}
	result := server.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, 2000, 4, nil,
		func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
			return []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "pmf_new_source", Lane: "event_recent", CompleteText: "A new consequence followed."}}, []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "pms_new_source", CompleteText: "The completed visit led to a new consequence."}}, map[string]any{"status": "ready"}
		})
	got := result.role("event_recent")
	if result.AnalysisCalls != 2 || got.Source != "ai" || !reflect.DeepEqual(got.Selection.SelectedIDs, []string{"pmf_new_source", facts[1].CanonicalFactID}) || !reflect.DeepEqual(got.Selection.SelectedSummaryIDs, []string{"pms_new_source", summaries[0].SummaryID}) {
		t.Fatalf("exact reference mapping or selection order failed: %+v", got)
	}
	if got.Selection.Reasons[facts[1].CanonicalFactID] == "" || len(got.Unresolved) != 0 || !strings.Contains(got.Calls[0].Raw, firstRef) {
		t.Fatal("canonical reasons, raw result or resolution diagnostics changed")
	}
	partial, err := parseMultiAgentRecommendation(`{"selected_ids":["F1","pmf_canonical_typo"],"selected_summary_ids":["S1"],"unresolved":["unfinished`)
	if err == nil {
		t.Fatal("fixture must be a truncated response")
	}
	resolveMultiAgentReferences(&partial, got.Calls[0].Input)
	if !reflect.DeepEqual(partial.SelectedIDs, []string{facts[0].CanonicalFactID, "pmf_canonical_typo"}) || !reflect.DeepEqual(partial.SelectedSummaryIDs, []string{summaries[0].SummaryID}) {
		t.Fatal("partial selections were erased or a canonical typo was guessed")
	}
}

func Test43MultiAgentGeneralPublicHandoffKeepsPrivateScope(t *testing.T) {
	const handoffReason = "Check which recorded operating conditions apply to these delivered tools."
	current := "Start repairs with the crew."
	recentLimit := 2
	req := dto.PrepareTurnRequest{
		RawUserInput: &current,
		Settings:     dto.PrepareTurnSettings{RecentConversationReferenceCount: &recentLimit},
		Messages: []map[string]any{
			{"role": "user", "content": "Visit the merchant."},
			{"role": "assistant", "content": "The merchant is away."},
			{"role": "user", "content": "Order one brace."},
			{"role": "assistant", "content": "One brace was ordered."},
			{"role": "user", "content": "Receive the delivery."},
			{"role": "assistant", "content": "One brace arrived; its precise arrival hour is unstated."},
			{"role": "user", "content": current},
		},
	}
	wantRecent := []string{
		"user:\nReceive the delivery.\nassistant:\nOne brace arrived; its precise arrival hour is unstated.",
		"user:\nOrder one brace.\nassistant:\nOne brace was ordered.",
	}
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "general-fact", Lane: "character_objective", SourceTable: "character_states", SourceRef: "character_states:12", Visibility: "general", CompleteText: "A public object is present.", SourceTurn: 3},
		{CanonicalFactID: "explicit-public-fact", Lane: "character_objective", Visibility: "public", CompleteText: "Another public object is present."},
		{CanonicalFactID: "private-fact", Lane: "character_objective", Visibility: "private", CompleteText: "Private fact."},
		{CanonicalFactID: "owned-fact", Lane: "character_objective", Visibility: "general", PerspectiveOwner: "Mira", CompleteText: "Owner context."},
		{CanonicalFactID: "viewer-fact", Lane: "character_objective", Visibility: "general", AllowedViewers: []string{"Mira"}, CompleteText: "Viewer context."},
		{CanonicalFactID: "subjective-fact", Lane: "subjective_relationship", Visibility: "general", CompleteText: "Subjective context."},
		{CanonicalFactID: "projection-owned", Lane: "event_recent", Visibility: "public_projection", PerspectiveOwner: "Mira", CompleteText: "Owned projection."},
		{CanonicalFactID: "projection-viewers", Lane: "event_recent", Visibility: "public_projection", AllowedViewers: []string{"Mira"}, CompleteText: "Viewer-scoped projection."},
		{CanonicalFactID: "projection-subjective", Lane: "subjective_relationship", Visibility: "public_projection", CompleteText: "Subjective projection."},
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages := body["messages"].([]any)
		var input map[string]any
		_ = json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &input)
		if input["current_input"] != current {
			t.Error("current input was replaced by old search context")
		}
		recent := anySliceFromAny(input["recent_conversation"])
		if len(recent) != recentLimit {
			t.Errorf("configured recent conversation count: got %d, want %d", len(recent), recentLimit)
		} else {
			for i, raw := range recent {
				if mapFromAny(raw)["Text"] != wantRecent[i] {
					t.Error("recent conversation lost the paired user input, completed response, quantity or uncertainty")
				}
			}
		}
		if !strings.Contains(extractionStringFromAny(mapFromAny(input["reference_format"])["source_turn"]), "snapshot") {
			t.Error("model input leaves snapshot update time indistinguishable from fact time")
		}
		if input["role"] == "character_objective" {
			items := input["candidates"].([]any)
			item := modelEvidenceForTest(t, input, items[0])
			if item["source_table"] != facts[0].SourceTable || item["source_ref"] != facts[0].SourceRef || item["text"] != facts[0].CompleteText || item["source_turn"] != float64(facts[0].SourceTurn) {
				t.Error("candidate lost its original source, text or snapshot turn")
			}
		}
		answer := multiAgentRecommendation{}
		_, second := input["previous_result"]
		if input["role"] == "character_objective" && !second {
			answer.RelatedRequests = []multiAgentRelatedRequest{{Role: "world_state", Refs: []string{"F1", "F2", "private-fact", "owned-fact", "viewer-fact", "subjective-fact", "projection-owned", "projection-viewers", "projection-subjective"}, Reason: handoffReason}}
		}
		if input["role"] == "world_state" && second {
			items := input["related_evidence"].([]any)
			if len(items) != 2 || items[0].(map[string]any)["id"] != "general-fact" || items[1].(map[string]any)["id"] != "explicit-public-fact" {
				t.Errorf("public general was lost or private scope broadened: %+v", items)
			}
			if modelEvidenceForTest(t, input, items[0])["source_turn"] != float64(3) {
				t.Error("handoff lost the public source time")
			}
			item := modelEvidenceForTest(t, input, items[0])
			if item["source_table"] != facts[0].SourceTable || item["source_ref"] != facts[0].SourceRef || item["text"] != facts[0].CompleteText {
				t.Error("cross-role handoff lost snapshot provenance or rewrote source text")
			}
			for _, raw := range items {
				item := mapFromAny(raw)
				if item["request_reason"] != handoffReason || item["from_role"] != "character_objective" {
					t.Error("recipient lost the public request purpose or its editor attribution")
				}
			}
		}
		encoded, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	cfg.SharedPrompt = "User-authored shared instructions."
	for role, c := range cfg.Roles {
		c.Enabled = role == "character_objective" || role == "world_state"
		c.Prompt = "User-authored role instructions."
		c.Provider, c.Endpoint, c.APIKey, c.Model = "custom", provider.URL, "fixture-key", "fixture-model"
		cfg.Roles[role] = c
	}
	searchCalls := 0
	search := func(query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		searchCalls++
		if query != handoffReason {
			t.Errorf("existing one-query handoff search changed: %q", query)
		}
		return nil, nil, nil
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, req, facts, nil, 2000, 4, nil, search)
	if searchCalls != 1 {
		t.Errorf("handoff changed existing search count: %d", searchCalls)
	}
	if result.AnalysisCalls != 4 || len(result.role("character_objective").Unresolved) != len(facts)-2 {
		t.Fatalf("existing cross-role scope behavior changed: %+v", result)
	}
}

func Test43MultiAgentLoreAssessmentIsIndependentAndRetained(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		failFirst           bool
		failSecond          bool
		want                *[]string
	}{
		{name: "omitted", first: `{}`, want: nil},
		{name: "null_unassessed", first: `{"selected_lorebook_refs":null}`, want: nil},
		{name: "failed_first", failFirst: true, want: nil},
		{name: "explicit_none", first: `{"selected_lorebook_refs":[]}`, want: &[]string{}},
		{name: "selected_order", first: `{"selected_lorebook_refs":["L2","L1"]}`, want: &[]string{"lore-b", "lore-a"}},
		{name: "failed_supplement", first: `{"selected_lorebook_refs":["L2"],"search_requests":["scene context"]}`, failSecond: true, want: &[]string{"lore-b"}},
		{name: "unrelated_field_error", first: `{"reasons":false,"selected_lorebook_refs":["L2"]}`, want: &[]string{"lore-b"}},
		{name: "unassessed_supplement", first: `{"selected_lorebook_refs":["L2"],"search_requests":["scene context"]}`, second: `{}`, want: &[]string{"lore-b"}},
		{name: "empty_final", first: `{"selected_lorebook_refs":["L2"],"search_requests":["scene context"]}`, second: `{"selected_lorebook_refs":[]}`, want: &[]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				messages := body["messages"].([]any)
				var input map[string]any
				_ = json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &input)
				if _, leaked := input["scope"].(map[string]any)["lorebook_candidates"]; leaked {
					t.Error("lore candidates were copied into shared scope")
				}
				if input["role"] != "world_state" {
					if _, leaked := input["lorebook_candidates"]; leaked {
						t.Error("lore assessment reached a different specialist")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{}`}}}})
					return
				}
				if len(input["lorebook_candidates"].([]any)) != 2 || input["budgets"].(map[string]any)["lorebook_delivery_chars"] != float64(100) {
					t.Error("world specialist did not receive whole lore candidates and their delivery budget")
				}
				answer := tc.first
				if _, second := input["previous_result"]; second {
					if tc.failSecond {
						http.Error(w, "fixture provider failure", http.StatusBadRequest)
						return
					}
					answer = tc.second
				} else if tc.failFirst {
					http.Error(w, "fixture provider failure", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled = role == "world_state" || role == "event_recent"
				c.Provider, c.Endpoint, c.APIKey, c.Model = "custom", provider.URL, "fixture-key", "fixture-model"
				cfg.Roles[role] = c
			}
			lore := []map[string]any{{"id": "lore-a", "text": "A setting rule."}, {"id": "lore-b", "text": "A useful object."}}
			result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 4, nil, nil, map[string]any{"session": "fixture", "lorebook_candidates": lore, "lorebook_budget_chars": 100})
			if !reflect.DeepEqual(result.LorebookRefs, tc.want) || !reflect.DeepEqual(result.role("world_state").Selection.SelectedLorebookRefs, tc.want) {
				t.Fatalf("lore assessment changed: got=%v want=%v", result.LorebookRefs, tc.want)
			}
			if result.role("world_state").Source != "go_default" {
				t.Fatal("lore assessment replaced the empty canonical-memory selection")
			}
			if _, mutated := lore[0]["ref"]; mutated {
				t.Fatal("request input mutated the caller's lore data")
			}
		})
	}
}

func Test43MultiAgentConfigurationDiagnosticIsSeparateFromEmptyCandidates(t *testing.T) {
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = role == "unresolved_goal"
		c.Provider, c.Model = "custom", "configured-model"
		cfg.Roles[role] = c
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 4, nil, nil)
	call := result.role("unresolved_goal").Calls[0]
	if call.Error == "" || call.Dispatched || result.AnalysisCalls != 0 || result.AnalysisAttempts != 1 || !reflect.DeepEqual(call.MissingConfigurationFields, []string{"endpoint", "api_key"}) {
		t.Fatalf("existing configuration failure lacks actionable field names: %+v", call)
	}
	counts := call.Input["candidate_counts"].(map[string]int)
	if counts["facts_available"] != 0 || counts["facts_supplied"] != 0 || call.Input["omitted_candidate_count"] != 0 {
		t.Fatal("empty source candidates were confused with omitted input or connection failure")
	}
}

func Test43MultiAgentDispatchDiagnosticDistinguishesLocalBuildAndRemoteError(t *testing.T) {
	requests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// The same words in a remote response are not a local configuration error.
		http.Error(w, `{"error":{"message":"provider / endpoint / api_key / model is required"}}`, http.StatusBadRequest)
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	role := cfg.Roles["world_state"]
	role.Provider, role.Endpoint, role.Model, role.APIKey = "custom", provider.URL, "fixture-model", "fixture-key"
	role.LLMGatewayServiceTier = "invalid-tier-fixture"
	cfg.Roles["world_state"] = role
	server := &Server{}
	local := server.callMultiAgent(context.Background(), "world_state", cfg, 1, map[string]any{})
	if local.Error == "" || local.Dispatched || requests != 0 {
		t.Fatalf("local request-build failure was counted as a dispatched call: %+v requests=%d", local, requests)
	}
	role.LLMGatewayServiceTier = ""
	cfg.Roles["world_state"] = role
	remote := server.callMultiAgent(context.Background(), "world_state", cfg, 1, map[string]any{})
	if remote.Error == "" || !remote.Dispatched || requests != 1 {
		t.Fatalf("remote provider failure was misclassified by its message: %+v requests=%d", remote, requests)
	}
}
