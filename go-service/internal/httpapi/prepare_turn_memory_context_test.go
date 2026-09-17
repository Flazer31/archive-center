package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func context45Assembly(t *testing.T, raw string, budget int, mode string) prepareTurnInjectionAssembly {
	t.Helper()
	p := priorityMemoryTestContext(5)
	p["_priority_memory_query"], p["_priority_memory_current_turn"] = "Mira uses the return cube at the archive", 31
	input := prepareTurnAssemblyInput{TopK: 5, MaxChars: budget, BudgetMode: mode, UserInput: "Mira uses the return cube at the archive", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(p),
		WorldRules:  []store.WorldRule{{ID: 601, ChatSessionID: "context45", Scope: "location", ScopeName: "archive", Key: "travel", ValueJSON: raw, SourceTurn: 20}},
		VectorTrace: map[string]any{"search_result": "ok", "search_results": []map[string]any{{"source_table": "world_rules", "source_row_id": "601", "tier": "world_rule", "similarity": .9}}}}
	if mode == "custom" {
		input.Budgets = map[string]int{"world_state": budget}
	}
	before, _ := json.Marshal(input)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("source input mutated")
	}
	assert45Budget(t, out.MemoryDeliveryPlan, budget)
	return out
}

func assert45Budget(t *testing.T, plan map[string]any, budget int) {
	t.Helper()
	final := extractionStringFromAny(plan["final_text"])
	if utf8.RuneCountInString(final) > budget {
		t.Fatalf("budget exceeded: %d > %d\n%s", utf8.RuneCountInString(final), budget, final)
	}
	for _, raw := range outputFidelityLineageSlice(plan["classes"]) {
		lane := mapFromAny(raw)
		if got, want := intFromAny(lane["used_chars"], 0), utf8.RuneCountInString(extractionStringFromAny(lane["text"])); got != want {
			t.Fatalf("lane %v accounted %d actual %d", lane["key"], got, want)
		}
	}
}

func Test45MinimumContextBeforeGoAndAISelection(t *testing.T) {
	raw := `[{"item":"return cube","giver":"Rowan","recipient":"Mira","function":"returns its holder to Rowan only outdoors","promise":"Mira promised to return the cube after the journey"}]`
	out := context45Assembly(t, raw, 18000, "auto")
	facts, summaries := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if strings.Contains(c.CompleteText, "function:") {
			if c.Minimum == nil || c.Relevance <= c.OriginalRelevance || c.FinalScore <= c.OriginalScore {
				t.Fatalf("context did not reach scoring: %+v", c)
			}
			if !strings.Contains(c.Minimum.Text, "return cube") || !strings.Contains(c.Minimum.Text, "only outdoors") || strings.Contains(c.Minimum.Text, "promised") {
				t.Fatalf("wrong minimum: %+v", c.Minimum)
			}
		}
		if strings.Contains(c.CompleteText, "giver:") && (c.Minimum == nil || !strings.Contains(c.Minimum.Text, "recipient: Mira")) {
			t.Fatal("transfer lost its direction")
		}
	}
	final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	for _, value := range []string{"item: return cube", "giver: Rowan", "recipient: Mira", "only outdoors", "after the journey"} {
		if strings.Count(final, value) != 1 {
			t.Fatalf("source constituent missing or repeated %q:\n%s", value, final)
		}
	}
	input := multiAgentInput("world_state", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 18000, 5, nil)
	for _, v := range outputFidelityLineageSlice(input["candidates"]) {
		item := mapFromAny(v)
		if strings.Contains(extractionStringFromAny(item["text"]), "function:") && (len(stringsFromAny(item["context_refs"])) < 2 || !strings.Contains(extractionStringFromAny(item["text"]), "only outdoors")) {
			t.Fatal("AI received an atomic fragment rather than the scored minimum")
		}
	}
	// Removing the new reading projection must not rename source facts.
	ids := map[string]bool{}
	for _, c := range facts {
		ids[c.CanonicalFactID] = true
	}
	control := out
	control.PriorityFactSeeds = append([]prepareTurnPriorityFactSeed(nil), out.PriorityFactSeeds...)
	for i := range control.PriorityFactSeeds {
		control.PriorityFactSeeds[i].Fact.Reading = nil
	}
	plain, _, _ := prepareTurnBuildPriorityCandidates(&control, "Mira uses the return cube at the archive", nil, 31, nil)
	for _, c := range plain {
		if !ids[c.CanonicalFactID] {
			t.Fatal("reading projection changed canonical fact IDs")
		}
	}

}

// These are the delivery-boundary failures, rather than a test that merely
// selects every field and observes that all the original words remain.
func Test45RuleEachSelectablePartCarriesItsProposition(t *testing.T) {
	raw := `[{"rule":"Only members may open the archive door","scope":"location","scope_name":"archive","exception":"Never while the red lamp is on","evidence_excerpt":"Mira said: keep it shut."}]`
	out := context45Assembly(t, raw, 18000, "auto")
	facts, summaries := multiAgentCandidatePool(&out)
	input := multiAgentInput("world_state", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 18000, 5, nil)
	for _, v := range outputFidelityLineageSlice(input["candidates"]) {
		item := mapFromAny(v)
		text := extractionStringFromAny(item["text"])
		for _, required := range []string{"Only members may open the archive door", "archive", "Never while the red lamp is on"} {
			if !strings.Contains(text, required) {
				t.Errorf("selectable %v lacks proposition %q: %s", item["ref"], required, text)
			}
		}
	}
	for _, chosen := range facts {
		// Selecting only a qualifier must still produce the rule in the final
		// delivery, not merely in the diagnostic candidate pool.
		copyOut := out
		copyOut.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "world_state", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: []string{chosen.CanonicalFactID}}}}}
		plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&copyOut, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
		final := extractionStringFromAny(plan["final_text"])
		for _, required := range []string{"Only members may open the archive door", "Never while the red lamp is on"} {
			if !strings.Contains(final, required) {
				t.Errorf("single selection %s lost %s: %s", chosen.SourcePath, required, final)
			}
		}
	}
}

func Test45NestedRuleQualifierKeepsProposition(t *testing.T) {
	out := context45Assembly(t, `[{"rule":"Only members may open the archive door","exceptions":[{"when":"red lamp is on","effect":"keep the door closed"}]}]`, 18000, "auto")
	facts, summaries := multiAgentCandidatePool(&out)
	for _, c := range facts {
		copyOut := out
		copyOut.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "world_state", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}}}}}
		plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&copyOut, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
		for _, s := range []string{"Only members may open", "red lamp is on", "keep the door closed"} {
			if !strings.Contains(extractionStringFromAny(plan["final_text"]), s) {
				t.Fatalf("nested rule candidate %s lost %q", c.SourcePath, s)
			}
		}
	}
}

func Test45KGAbbreviationKeepsWholeRelation(t *testing.T) {
	p := priorityMemoryTestContext(5)
	query := "Mira guild_number No. 42"
	p["_priority_memory_query"] = query
	out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
		TopK: 5, MaxChars: 18000, UserInput: query, Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(p),
		Triples: []store.KGTriple{{ID: 71, ChatSessionID: "context45", Subject: "Mira", Predicate: "guild_number", Object: "No. 42", SourceTurn: 20}},
	})
	facts, summaries := multiAgentCandidatePool(&out)
	if len(facts) == 0 {
		t.Fatal("KG fixture did not reach production selection")
	}
	input := multiAgentInput("subjective_relationship", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 18000, 5, nil)
	for _, v := range outputFidelityLineageSlice(input["candidates"]) {
		text := extractionStringFromAny(mapFromAny(v)["text"])
		for _, required := range []string{"Mira", "guild_number", "No. 42"} {
			if !strings.Contains(text, required) {
				t.Errorf("relation fragment %q lacks %q", text, required)
			}
		}
	}
	final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	if strings.Count(final, "No. 42") != 1 {
		t.Errorf("relation not delivered once and intact: %s", final)
	}
}

func Test45SameSourceSummaryAndParsedVariantShareDelivery(t *testing.T) {
	text := "Mira approved the plan. Rowan prepared the journey. Guardians external_deployment: suspended | summary_language=ko | raw_language=ko"
	out := prepareTurnInjectionAssembly{}
	appendPrepareTurnPrioritySourceMetadata(&out, "event_recent", "memories", "required", "- "+text, "memories:71", 71, 20, .8, true, "public_projection", "", nil)
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira plan journey", CurrentTurn: 21})
	final := extractionStringFromAny(plan["final_text"])
	if strings.Count(final, "Mira approved the plan.") != 1 {
		t.Errorf("summary repeated as a punctuation variant: %s", final)
	}
	if !strings.Contains(final, "suspended") {
		t.Fatal("summary detail disappeared")
	}
	facts, summaries := multiAgentCandidatePool(&out)
	ids, summaryIDs := []string{}, []string{}
	for _, c := range facts {
		ids = append(ids, c.CanonicalFactID)
	}
	for _, c := range summaries {
		summaryIDs = append(summaryIDs, c.SummaryID)
	}
	out.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: ids, SelectedSummaryIDs: summaryIDs}}}}
	plan = finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
	final = extractionStringFromAny(plan["final_text"])
	if strings.Count(final, "Mira approved the plan.") != 1 || len(stringsFromAny(plan["selected_fact_ids"])) != len(ids) || len(stringsFromAny(plan["selected_turn_summary_ids"])) != len(summaryIDs) {
		t.Fatalf("AI choices must remain selected while their body is shared: %s", final)
	}
	// No fuzzy collapse of another source or later occurrence of similar text.
	other := prepareTurnInjectionAssembly{}
	for _, turn := range []int{20, 21} {
		appendPrepareTurnPrioritySourceMetadata(&other, "event_recent", "memories", "required", "- "+text, fmt.Sprintf("memories:%d", turn), turn, turn, .8, true, "public_projection", "", nil)
	}
	plan = buildPrepareTurnPriorityMemoryDeliveryPlan(&other, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira plan journey", CurrentTurn: 22})
	if strings.Count(extractionStringFromAny(plan["final_text"]), "Mira approved the plan.") != 2 {
		t.Fatal("independent source occurrences were merged")
	}
}

func Test45CompleteReadingReusePreservesVisibleContext(t *testing.T) {
	var l prepareTurnMemoryReadingLayout
	part := prepareTurnMemoryFormPart{Key: "relation", Text: "Mira --guild_number--> No. 42"}
	a := prepareTurnMemoryReadingRow{Order: 0, Group: "source-A", Header: "source A", Parts: []prepareTurnMemoryFormPart{part}}
	l.apply(l.preview(a))
	l.apply(l.preview(prepareTurnMemoryReadingRow{Order: 1, Plain: "- another independent event"}))
	a.Order = 2
	edit := l.preview(a)
	if edit.delta != 0 {
		t.Fatal("complete source body charged twice")
	}
	l.apply(edit)
	if strings.Count(strings.Join(l.texts(), "\n"), part.Text) != 1 {
		t.Fatal("non-adjacent source body repeated")
	}
	// An earlier deferred duplicate must not erase the existing source body.
	a.Order = -1
	l.apply(l.preview(a))
	if strings.Count(strings.Join(l.texts(), "\n"), part.Text) != 1 {
		t.Fatal("borrowed source body disappeared or repeated")
	}
	// Late identity support is additional delivered content, even if this
	// candidate's complete base reading was already represented by a summary.
	a.Order = 3
	a.Parts = append(a.Parts, prepareTurnMemoryFormPart{Key: "identity_metadata", Text: "identity_metadata: Mira also uses the name M"})
	l.apply(l.preview(a))
	final := strings.Join(l.texts(), "\n")
	if strings.Count(final, part.Text) != 1 || !strings.Contains(final, "Mira also uses the name M") {
		t.Fatal("shared reading dropped late identity support or repeated its base")
	}
}

func Test45MinimumBudgetConditionsAndSeparateRecords(t *testing.T) {
	for _, mode := range []string{"auto", "custom"} {
		t.Run(mode, func(t *testing.T) {
			raw := `[{"item":"return cube","function":"returns its holder to Rowan only outdoors","description":"` + strings.Repeat("decorative inscription ", 150) + `"},{"scope":"location","scope_name":"archive","rule":"Only members may open the door","exception":"Never while the red lamp is lit"}]`
			out := context45Assembly(t, raw, 650, mode)
			final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
			for _, s := range []string{"return cube", "only outdoors", "Only members may open the door", "Never while the red lamp is lit"} {
				if !strings.Contains(final, s) {
					t.Fatalf("lost minimum %q: %s", s, final)
				}
			}
			if strings.Contains(final, "decorative inscription") {
				t.Fatal("whole group displaced minimum details")
			}
			facts, _ := multiAgentCandidatePool(&out)
			for _, c := range facts {
				if c.Minimum != nil && strings.Contains(c.CompleteText, "function:") && strings.Contains(c.Minimum.Text, "red lamp") {
					t.Fatal("same stored row mixed independent records")
				}
			}
		})
	}
}

func Test45RecollectionExperienceEmotionAndEvidence(t *testing.T) {
	p := priorityMemoryTestContext(5)
	p["_priority_memory_query"] = "Mira recalls her key at the archive"
	p["_priority_memory_current_turn"] = 31
	entry := store.PersonaMemoryEntry{ID: 801, CapsuleID: 80, SourceMemoryType: "manual", SourceTurn: 20, MemoryText: "Mira recalls Rowan returning her brass key at the archive. Mira said she felt relieved.", EvidenceExcerpt: `Mira said: "Thank you for returning my key."`, Importance10: 8, Portability: "same_character_continuation", InjectionPolicy: "support_only"}
	out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{TopK: 5, MaxChars: 18000, BudgetMode: "auto", UserInput: "Mira recalls her key at the archive", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(p), PersonaEntries: []store.PersonaMemoryEntry{entry}})
	final := extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	for _, s := range []string{entry.MemoryText, entry.EvidenceExcerpt, "not current-world truth", "owner protagonist", "visibility protagonist_private"} {
		if !strings.Contains(final, s) {
			t.Fatalf("missing capsule context %q in %s", s, final)
		}
	}
	if strings.Count(final, entry.MemoryText) != 1 {
		t.Fatal("recollection duplicated for each sentence")
	}
	assert45Budget(t, out.MemoryDeliveryPlan, 18000)
	facts, _ := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if multiAgentPublicEvidence(c) {
			t.Fatal("capsule became public handoff evidence")
		}
	}
}

func Test45ContextLayoutInsertionMatchesActualCost(t *testing.T) {
	layout := &prepareTurnMemoryReadingLayout{}
	rows := []prepareTurnMemoryReadingRow{
		{Order: 1, Group: "A", Header: "A source", Parts: []prepareTurnMemoryFormPart{{Key: "name", Text: "name: Mira"}, {Key: "a", Text: "action A"}}},
		{Order: 5, Group: "A", Header: "A source", Parts: []prepareTurnMemoryFormPart{{Key: "name", Text: "name: Mira"}, {Key: "c", Text: "condition C"}}},
		{Order: 3, Group: "B", Header: "B source", Parts: []prepareTurnMemoryFormPart{{Key: "b", Text: "separate event B"}}},
		{Order: 2, Group: "A", Header: "A source", Parts: []prepareTurnMemoryFormPart{{Key: "name", Text: "name: Mira"}, {Key: "a", Text: "action A"}}},
		{Order: 4, Plain: "- independent summary"},
	}
	chars := 0
	for _, row := range rows {
		edit := layout.preview(row)
		chars += edit.delta
		layout.apply(edit)
		actual := 0
		for _, line := range layout.texts() {
			actual += 1 + utf8.RuneCountInString(line)
		}
		if chars != actual {
			t.Fatalf("incremental cost %d != %d", chars, actual)
		}
	}
	text := strings.Join(layout.texts(), "\n")
	if strings.Count(text, "A source") != 1 || strings.Count(text, "name: Mira") != 1 || strings.Count(text, "action A") != 1 || !strings.Contains(text, "condition C") || !strings.Contains(text, "separate event B") {
		t.Fatalf("nonadjacent source reuse lost or repeated constituents: %s", text)
	}
}

func expand45Shared(value any, records map[string]any) any {
	switch node := value.(type) {
	case map[string]any:
		if ref, ok := node["shared_record"].(string); ok && len(node) == 1 {
			return expand45Shared(records[ref], records)
		}
		out := map[string]any{}
		for k, v := range node {
			out[k] = expand45Shared(v, records)
		}
		return out
	case []any:
		out := make([]any, len(node))
		for i, v := range node {
			out[i] = expand45Shared(v, records)
		}
		return out
	default:
		return value
	}
}

func Test45SharedTextAcrossCandidateRefsAndRoleSources(t *testing.T) {
	for _, round := range []int{1, 2} {
		for _, count := range []int{1, 2, 5} {
			t.Run(fmt.Sprintf("round_%d_roles_%d", round, count), func(t *testing.T) {
				cfg := defaultMultiAgentSettings()
				roles := multiAgentRoles[:count]
				common := strings.Repeat("Only members may open the archive; never while the red lamp is lit. ", 8)
				calls, expected := []multiAgentCall{}, []map[string]any{}
				for i, role := range roles {
					cfg.Roles[role] = multiAgentRoleConfig{Provider: "custom", Endpoint: "http://127.0.0.1:1", Model: "fixture", APIKey: "fixture"}
					// Different aliases, times and holders must remain exact. The text
					// dictionary is shared prose, never a merged evidence record.
					input := map[string]any{"role": role, "current_input": "Open the archive", "candidates": []map[string]any{
						{"ref": "F1", "id": role + "-1", "source_ref": fmt.Sprintf("records:%d", i), "source_turn": 20 + i, "perspective_owner": role, "visibility": "owner_private", "text": common},
						{"ref": "F2", "id": role + "-2", "source_ref": fmt.Sprintf("records:%d", i), "source_turn": 20 + i, "perspective_owner": role, "visibility": "owner_private", "text": common},
						{"ref": "F3", "id": role + "-3", "text": "Distinct exception for " + role},
					}}
					var before map[string]any
					_ = json.Unmarshal([]byte(multiAgentModelInput(input, round)), &before)
					expected = append(expected, before)
					call, _ := (&Server{}).multiAgentProxyRequest(role, cfg, round, input, count > 1)
					calls = append(calls, call)
				}
				wire := calls[0].ModelInput
				if count > 1 {
					wire = multiAgentGroupedInput(calls, roles, cfg)
				}
				var packet map[string]any
				_ = json.Unmarshal([]byte(wire), &packet)
				if strings.Count(wire, common) != 1 {
					t.Fatalf("same reading still sent more than once: %d", strings.Count(wire, common))
				}
				for i := range roles {
					local := packet
					if count > 1 {
						local = map[string]any{}
						for k, v := range mapFromAny(packet["shared_input"]) {
							local[k] = v
						}
						for k, v := range mapFromAny(mapFromAny(outputFidelityLineageSlice(packet["roles"])[i])["input"]) {
							local[k] = v
						}
					}
					expanded := mapFromAny(expand45Shared(local, mapFromAny(packet["shared_records"])))
					delete(expanded, "shared_records")
					delete(expanded, "shared_records_format")
					if !reflect.DeepEqual(expanded, expected[i]) {
						t.Fatalf("%s lost input/order/provenance", roles[i])
					}
					selected := calls[i]
					selected.Raw = `{"selected_ids":["F2","F1","F3"]}`
					selected = finishMultiAgentCall(selected, 200, nil, "")
					want := []string{roles[i] + "-2", roles[i] + "-1", roles[i] + "-3"}
					if !reflect.DeepEqual(selected.Result.SelectedIDs, want) {
						t.Fatalf("selection IDs/order changed: %v", selected.Result.SelectedIDs)
					}
				}
			})
		}
	}
}

func Test45NonadjacentRuleConstituentsAndPreviewRollback(t *testing.T) {
	base := []prepareTurnMemoryFormPart{{Key: "scope", Text: "scope: archive"}, {Key: "rule", Text: "rule: entry is forbidden except to members"}}
	rows := []prepareTurnMemoryReadingRow{
		{Order: 0, Group: "source-A", Header: "turn 125 source A", Ref: "F41", Parts: append(append([]prepareTurnMemoryFormPart{}, base...), prepareTurnMemoryFormPart{Key: "key", Text: "key: entry_rule"})},
		{Order: 1, Group: "source-B", Header: "turn 126 source B", Parts: []prepareTurnMemoryFormPart{{Key: "rule", Text: "rule: emergency exit stays available"}}},
		{Order: 2, Group: "source-A", Header: "turn 125 source A", Ref: "F37", Parts: append(append([]prepareTurnMemoryFormPart{}, base...), prepareTurnMemoryFormPart{Key: "condition", Text: "condition: never while the red lamp is lit"})},
	}
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
		var layout prepareTurnMemoryReadingLayout
		cost := 0
		for _, i := range order {
			before := strings.Join(layout.texts(), "\n")
			_ = layout.preview(rows[i]) // An unaccepted reservation must change nothing.
			if strings.Join(layout.texts(), "\n") != before {
				t.Fatal("preview mutated delivery")
			}
			edit := layout.preview(rows[i])
			cost += edit.delta
			layout.apply(edit)
			actual := 0
			for _, text := range layout.texts() {
				actual += 1 + utf8.RuneCountInString(text)
			}
			if cost != actual {
				t.Fatalf("cost %d != actual %d", cost, actual)
			}
		}
		text := strings.Join(layout.texts(), "\n")
		for _, row := range rows {
			for _, part := range row.Parts {
				if strings.Count(text, part.Text) != 1 {
					t.Fatalf("missing/repeated %q in %s", part.Text, text)
				}
			}
		}
		// A changed value and another source's identical prose are not duplicates.
		changed := rows[0]
		changed.Order = 3
		changed.Parts = []prepareTurnMemoryFormPart{{Key: "rule", Text: "rule: later corrected source wording"}}
		layout.apply(layout.preview(changed))
		other := rows[0]
		other.Order, other.Group, other.Header = 4, "source-C-private", "turn 124 another holder"
		layout.apply(layout.preview(other))
		text = strings.Join(layout.texts(), "\n")
		if strings.Count(text, base[1].Text) != 2 || !strings.Contains(text, changed.Parts[0].Text) || !strings.Contains(text, other.Header) {
			t.Fatal("reuse merged independent source/value/holder")
		}
	}
}

func Test45FinalRuleReusePreservesSelectedEvidence(t *testing.T) {
	rule := "Only members may enter the archive; the prohibition also applies at night"
	raw := `[{"scope":"archive","scope_name":"entry authority","rule":"` + rule + `","key":"entry_rule","category":"training_authority"},{"scope":"courtyard","scope_name":"emergency exit","rule":"Keep the emergency exit available"}]`
	for _, mode := range []string{"off", "ai", "go_fallback"} {
		t.Run(mode, func(t *testing.T) {
			out := context45Assembly(t, raw, 18000, "auto")
			facts, summaries := multiAgentCandidatePool(&out)
			keys, category, other := "", "", ""
			for _, c := range facts {
				if strings.Contains(c.CompleteText, "key: entry_rule") {
					keys = c.CanonicalFactID
				}
				if strings.Contains(c.CompleteText, "category: training_authority") {
					category = c.CanonicalFactID
				}
				if strings.Contains(c.CompleteText, "rule: Keep the emergency exit available") {
					other = c.CanonicalFactID
				}
			}
			if keys == "" || category == "" || other == "" {
				t.Fatal("fixture did not produce distinct real field candidates")
			}
			ids := []string{keys, other, category}
			if mode != "off" {
				source := "ai"
				if mode == "go_fallback" {
					source = "go"
				}
				out.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "world_state", Source: source, Selection: multiAgentRecommendation{SelectedIDs: ids}}}}
				out.Preprocessing.captureBaseline(out.MemoryDeliveryPlan)
			}
			plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
			text := extractionStringFromAny(plan["final_text"])
			for _, value := range []string{rule, "key: entry_rule", "category: training_authority", "Keep the emergency exit available"} {
				if strings.Count(text, value) != 1 {
					t.Fatalf("%s lost/repeated %q: %s", mode, value, text)
				}
			}
			selected := map[string]bool{}
			for _, raw := range outputFidelityLineageSlice(plan["priority_items"]) {
				item := mapFromAny(raw)
				if item["selection_status"] == "selected" {
					selected[extractionStringFromAny(item["canonical_fact_id"])] = true
				}
			}
			for _, id := range ids {
				if !selected[id] {
					t.Fatalf("%s disappeared from selected evidence in %s", id, mode)
				}
			}
		})
	}
}

func Test45GroupedPartialSharingPreservesExactRolePackets(t *testing.T) {
	cfg := defaultMultiAgentSettings()
	roles := []string{"event_recent", "world_state"}
	calls := make([]multiAgentCall, 2)
	expected := make([]map[string]any, 2)
	common := strings.Repeat("Mira keeps the key; the archive exception still applies. ", 60)
	for i, role := range roles {
		input := map[string]any{"role": role, "current_input": "Open the archive", "candidates": []any{map[string]any{"ref": "F1", "source": "P1", "text": common}, map[string]any{"ref": "F2", "text": "independent " + role}}, "source_catalog": map[string]any{"P1": map[string]any{"r": fmt.Sprintf("source-%d", i), "n": 20 + i}}}
		calls[i].ModelInput = multiAgentModelInput(input, 2)
		_ = json.Unmarshal([]byte(calls[i].ModelInput), &expected[i])
	}
	wire := multiAgentGroupedInput(calls, roles, cfg)
	var packet map[string]any
	_ = json.Unmarshal([]byte(wire), &packet)
	records := mapFromAny(packet["shared_records"])
	if len(records) == 0 || strings.Count(wire, common) != 1 {
		t.Fatal("overlapping records were not shared")
	}
	for i, raw := range outputFidelityLineageSlice(packet["roles"]) {
		input := map[string]any{}
		for k, v := range mapFromAny(packet["shared_input"]) {
			input[k] = v
		}
		for k, v := range mapFromAny(mapFromAny(raw)["input"]) {
			input[k] = v
		}
		expanded := expand45Shared(input, records)
		if !reflect.DeepEqual(expanded, expected[i]) {
			t.Fatalf("role %s lost text/order/local provenance", roles[i])
		}
	}
	unsharedRoles := []any{}
	for i, raw := range outputFidelityLineageSlice(packet["roles"]) {
		assignment := map[string]any{}
		for k, v := range mapFromAny(raw) {
			assignment[k] = v
		}
		local := map[string]any{}
		for k, v := range expected[i] {
			if _, alreadyShared := mapFromAny(packet["shared_input"])[k]; !alreadyShared {
				local[k] = v
			}
		}
		assignment["input"] = local
		unsharedRoles = append(unsharedRoles, assignment)
	}
	before, _ := json.Marshal(map[string]any{"contract_version": "memory_preprocessing.group.v1", "shared_input": packet["shared_input"], "roles": unsharedRoles})
	if len(wire) >= len(before) {
		t.Fatal("sharing format consumed more than original packets")
	}
	t.Logf("same reading contents before=%d chars after=%d chars; exact role reconstruction passed", utf8.RuneCount(before), utf8.RuneCountInString(wire))
}

func Test45MinimumContextBothRoundsAndGoFallback(t *testing.T) {
	for _, mode := range []string{"normal", "none", "first_failure", "second_failure"} {
		t.Run(mode, func(t *testing.T) {
			out := context45Assembly(t, `[{"item":"return cube","function":"returns its holder to Rowan only outdoors"}]`, 18000, "auto")
			facts, summaries := multiAgentCandidatePool(&out)
			var chosen string
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				var input map[string]any
				_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(outputFidelityLineageSlice(body["messages"])[1])["content"])), &input)
				input = mapFromAny(expand45Shared(input, mapFromAny(input["shared_records"])))
				for _, v := range outputFidelityLineageSlice(input["candidates"]) {
					item := mapFromAny(v)
					if strings.Contains(extractionStringFromAny(item["text"]), "function:") {
						chosen = extractionStringFromAny(item["ref"])
						if !strings.Contains(extractionStringFromAny(item["text"]), "item: return cube") {
							t.Error("round lost minimum context")
						}
					}
				}
				if mode == "first_failure" || (mode == "second_failure" && calls == 2) {
					http.Error(w, "fixture failure", 400)
					return
				}
				result := map[string]any{}
				if mode != "none" {
					result["selected_ids"] = []string{chosen}
					if calls == 1 {
						result["search_requests"] = []string{"return cube operation"}
					}
				}
				b, _ := json.Marshal(result)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled = role == "world_state"
				c.UsePublisher = false
				c.Provider = "custom"
				c.Endpoint = provider.URL
				c.Model = "fixture"
				c.APIKey = "fixture-key"
				cfg.Roles[role] = c
			}
			out.Preprocessing = (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, 18000, 5, nil, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				return facts, summaries, map[string]any{"status": "ok"}
			})
			out.Preprocessing.captureBaseline(out.MemoryDeliveryPlan)
			plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
			final := extractionStringFromAny(plan["final_text"])
			for _, s := range []string{"item: return cube", "only outdoors"} {
				if !strings.Contains(final, s) {
					t.Fatalf("%s lost %s: %s", mode, s, final)
				}
			}
			if (mode == "normal" || mode == "second_failure") && calls != 2 {
				t.Fatalf("second round never exercised: calls=%d", calls)
			}
		})
	}
}

func Test45HTTPContextReachesPayloadAndPublisher(t *testing.T) {
	for _, preprocess := range []bool{false, true} {
		for _, publisher := range []bool{false, true} {
			t.Run(fmt.Sprintf("preprocess-%v/publisher-%v", preprocess, publisher), func(t *testing.T) {
				t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
				publisherCalls, agentCalls := 0, 0
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					encodedBody, _ := json.Marshal(body)
					for _, key := range []string{`"assembly_timing":`, `"preprocessing_stages_ms":`, `"timing_ms":`} {
						if strings.Contains(string(encodedBody), key) {
							t.Errorf("diagnostic timing leaked into provider input: %s", key)
						}
					}
					if _, ok := body["input"]; ok {
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{.2, .1}, "index": 0}}})
						return
					}
					var input map[string]any
					_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(outputFidelityLineageSlice(body["messages"])[1])["content"])), &input)
					if _, ok := input["supervisor_support_packet"]; ok {
						publisherCalls++
						encoded, _ := json.Marshal(input)
						for _, s := range []string{"return cube", "only outdoors"} {
							if !strings.Contains(string(encoded), s) {
								t.Errorf("Publisher lost %s", s)
							}
						}
						_, _ = w.Write([]byte(publisherV3OpenAIResponse("world_rules:601", "Use the cube's recorded operating condition.")))
						return
					}
					agentCalls++
					selected := []string{}
					for _, v := range outputFidelityLineageSlice(input["candidates"]) {
						item := mapFromAny(v)
						if strings.Contains(extractionStringFromAny(item["text"]), "function:") {
							selected = append(selected, extractionStringFromAny(item["ref"]))
							if !strings.Contains(extractionStringFromAny(item["text"]), "item: return cube") {
								t.Error("agent did not receive minimum context")
							}
						}
					}
					if len(selected) == 0 {
						t.Error("fixture did not reach real agent request")
					}
					result, _ := json.Marshal(map[string]any{"selected_ids": selected})
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(result)}}}})
				}))
				defer provider.Close()
				cfg := config.Default()
				cfg.PromptDir = filepath.Join("..", "..", "..", "prompts")
				cfg.StoreMode = config.StoreModeDualShadow
				cfg.ChromaEndpoint = "http://127.0.0.1:8000"
				cfg.Readiness.ChromaConfigured = true
				s := NewServer(cfg)
				s.Store = &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{
					returnMemories:   []store.Memory{{ID: 602, ChatSessionID: "context45-http", TurnIndex: 20, Importance: 1, SummaryJSON: `{"narrative_events":[{"event":"Mira received the return cube from Rowan at the archive.","visibility":"public"}]}`}},
					returnWorldRules: []store.WorldRule{{ID: 601, ChatSessionID: "context45-http", Scope: "location", ScopeName: "archive", Key: "travel", ValueJSON: `[{"item":"return cube","function":"returns its holder to Rowan only outdoors"}]`, SourceTurn: 20}},
				}}
				s.Vector = &priorityPrepareTurnVector{doc: vector.VectorDocument{ID: "rule-601", Tier: "world_rule", ChatSessionID: "context45-http", SourceTable: "world_rules", SourceRowID: "601", Similarity: .9, SimilarityAvailable: true, SimilaritySource: "cosine"}}
				s.RuntimeConfig.EmbeddingProvider, s.RuntimeConfig.EmbeddingEndpoint, s.RuntimeConfig.EmbeddingModel, s.RuntimeConfig.EmbeddingAPIKey = "custom", provider.URL, "fixture-embedding", "fixture-key"
				s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "fixture-publisher", "fixture-key"
				s.RuntimeConfig.EmbeddingTimeoutSec, s.RuntimeConfig.SupervisorTimeoutSec = 30, 30
				settings := defaultMultiAgentSettings()
				settings.Enabled = preprocess
				for role, c := range settings.Roles {
					c.Enabled = role == "world_state"
					c.UsePublisher = false
					c.Provider = "custom"
					c.Endpoint = provider.URL
					c.Model = "fixture"
					c.APIKey = "fixture-key"
					settings.Roles[role] = c
				}
				b, _ := json.Marshal(settings)
				rec := httptest.NewRecorder()
				s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(b)))
				if rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				mux := http.NewServeMux()
				s.RegisterRoutes(mux)
				request := map[string]any{"chat_session_id": "context45-http", "turn_index": 31, "raw_user_input": "Mira uses the return cube at the archive.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 6000, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}, "client_meta": map[string]any{"chroma_query_vector": []float64{.1, .2}}}
				b, _ = json.Marshal(request)
				rec = httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(b)))
				if rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				var response map[string]any
				_ = json.Unmarshal(rec.Body.Bytes(), &response)
				assemblyTiming := mapFromAny(mapFromAny(response["backend_timing"])["memory_assembly"])
				stageTimes := mapFromAny(assemblyTiming["stages_ms"])
				for _, key := range []string{"source_preparation", "initial_candidates", "other_assembly"} {
					if value, ok := stageTimes[key].(float64); !ok || value < 0 {
						t.Fatalf("missing measured assembly stage %s", key)
					}
				}
				if preprocess {
					stages := mapFromAny(assemblyTiming["preprocessing_stages_ms"])
					for _, key := range []string{"first_input_preparation", "first_round_wall", "second_input_preparation", "analysis_result_projection"} {
						if value, ok := stages[key].(float64); !ok || value < 0 {
							t.Fatalf("missing measured preprocessing stage %s", key)
						}
					}
				} else if assemblyTiming["preprocessing_stages_ms"] != nil {
					t.Fatal("disabled preprocessing reported analysis timing")
				}
				for _, value := range []any{mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])["final_text"], mapFromAny(response["injection_pack"])["injection_text"], response["payload_application_plan"]} {
					encoded, _ := json.Marshal(value)
					for _, key := range []string{`"assembly_timing":`, `"preprocessing_stages_ms":`, `"timing_ms":`} {
						if strings.Contains(string(encoded), key) {
							t.Fatalf("diagnostic timing leaked into final payload: %s", key)
						}
					}
					for _, s := range []string{"return cube", "only outdoors"} {
						if !strings.Contains(string(encoded), s) {
							t.Fatalf("production output lost %s: %s", s, string(encoded))
						}
					}
				}
				if (publisher && publisherCalls != 1) || (!publisher && publisherCalls != 0) || (preprocess && agentCalls != 1) || (!preprocess && agentCalls != 0) {
					t.Fatalf("unexpected call counts: publisher=%d agent=%d status=%v", publisherCalls, agentCalls, mapFromAny(response["supervisor_input_pack"])["guide_eligibility"])
				}
			})
		}
	}
}

func Test45ReadingFormsReusedAcrossIndependentQuestions(t *testing.T) {
	out := context45Assembly(t, `[{"item":"return cube","function":"teleports its holder","conditions":["only outdoors","never during a storm"],"item_1":"literal field, not an array position"}]`, 18000, "auto")
	facts, _ := multiAgentCandidatePool(&out)
	byID := map[string]*prepareTurnMemoryForm{}
	for _, c := range facts {
		byID[c.CanonicalFactID] = c.Minimum
	}
	count := len(out.preparation.readingForms)
	if count == 0 {
		t.Fatal("source forms not prepared")
	}
	for _, query := range []string{"storm", "outdoors", "holder", "literal field", "return cube"} {
		fresh, _, _ := prepareTurnBuildPriorityCandidates(&out, query, []string{query}, 31, nil)
		for _, c := range fresh {
			if c.Minimum != byID[c.CanonicalFactID] {
				t.Fatal("question rebuilt unchanged source form")
			}
			if c.SourcePath == "/0/function" && (!strings.Contains(c.Minimum.Text, "only outdoors") || !strings.Contains(c.Minimum.Text, "never during a storm")) {
				t.Fatal("nested conditions lost")
			}
			if strings.Contains(c.CompleteText, "literal field") && c.SourcePath != "/0/item_1" {
				t.Fatal("raw source path confused with display ordinal")
			}
		}
	}
	if len(out.preparation.readingForms) != count {
		t.Fatal("questions accumulated identical reading forms")
	}
}

func Test45ContextPriorityDoesNotResolveCurrentState(t *testing.T) {
	seeds := []prepareTurnPriorityFactSeed{}
	for i, value := range []string{"sleeping", "Mira returned to the archive"} {
		fact := prepareTurnPriorityMemoryFact{Text: value, FamilyKey: "Mira.state", ValueKey: value, Structured: true, SourcePath: "/state"}
		if i == 0 {
			fact.Reading = &prepareTurnMemoryContext{Path: "/", Label: "Mira", Parts: []prepareTurnMemoryPart{{Key: "/state", Value: value, FactTexts: []string{value}}, {Key: "/condition", Value: "Mira returned to the archive"}}}
		}
		seeds = append(seeds, prepareTurnPriorityFactSeed{Fact: fact, Lane: "character_objective", SourceTable: "character_states", SourceRowID: i + 1, SourceOccurrence: fmt.Sprint(i + 1), SourceTurn: 20, Importance: .5, ImportancePresent: true, ParentLineKey: value})
	}
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: seeds}
	current, previous, _ := prepareTurnBuildPriorityCandidates(&out, "Mira returned to the archive", nil, 31, nil)
	if len(current) != 1 || len(previous) != 1 || current[0].CompleteText != seeds[1].Fact.Text {
		t.Fatal("minimum-context priority replaced original state resolver")
	}
	if previous[0].FinalScore <= previous[0].OriginalScore {
		t.Fatal("negative control did not exercise changed context score")
	}
	seeds[0].SourceTurn = 21
	out.PriorityFactSeeds = seeds
	current, _, _ = prepareTurnBuildPriorityCandidates(&out, "Mira returned to the archive", nil, 31, nil)
	if current[0].SourceTurn != 21 {
		t.Fatal("source-time precedence changed")
	}
}
