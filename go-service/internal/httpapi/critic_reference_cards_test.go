package httpapi

import (
	"path/filepath"
	"strings"
	"testing"
)

func criticReferenceCardsForTest(t *testing.T, ledger map[string]any) []map[string]any {
	t.Helper()
	table := mapFromAny(ledger["reference_cards"])
	fields := stringsFromAny(table["fields"])
	var out []map[string]any
	for _, raw := range sliceFromAny(table["rows"]) {
		row := sliceFromAny(raw)
		if len(row) != len(fields) {
			t.Fatal("reference card has mismatched columns")
		}
		card := map[string]any{}
		for i, field := range fields {
			if row[i] != nil {
				card[field] = row[i]
			}
		}
		out = append(out, card)
	}
	return out
}

func TestCriticReferenceCardsShareKeysAndKeepLatestCompleteObservation(t *testing.T) {
	memory := func(id, turn int, ref map[string]any) map[string]any {
		return map[string]any{"id": id, "turn_index": turn, "source": "mariadb_memory", "support_only": true,
			"summary": strings.Repeat("Workshop shield historical scenery. ", 300), "recorded_state_claims": []any{ref}}
	}
	old := memory(1, 1, map[string]any{"subject": "Workshop shield", "state_slot": "goal_status", "lifecycle_key": "shield", "value": "Collect workshop shield", "transition": "create"})
	latest := memory(2, 3, map[string]any{"subject": "Workshop shield", "state_slot": "goal_status", "lifecycle_key": "shield", "value": "Shield collected; certificate still missing", "transition": "advance"})
	latest["recorded_pending_threads"] = []any{map[string]any{"lifecycle_key": "shield", "title": "Workshop shield", "status": "open", "description": "Collect shield AND certificate before departure", "remaining_obligations": "Obtain certificate"}}
	other := memory(3, 2, map[string]any{"subject": "Workshop shield", "state_slot": "location", "value": "Workshop chest"})
	inputs := []map[string]any{old, other, latest}
	before := mustCompactJSON(inputs)
	_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Collect workshop shield and certificate before departure", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 1000})
	cards := criticReferenceCardsForTest(t, ledger)
	if len(cards) != 2 {
		t.Fatalf("wanted separate goal and location, with the goal once: %s", mustCompactJSON(ledger))
	}
	for _, card := range cards {
		if stringFromMap(card, "lifecycle_key") == "shield" {
			for key, want := range map[string]string{"value": "Shield collected; certificate still missing", "description": "Collect shield AND certificate before departure", "remaining_obligations": "Obtain certificate", "status": "open"} {
				if stringFromMap(card, key) != want {
					t.Errorf("card lost %s: %#v", key, card)
				}
			}
			if intFromAny(card["source_turn"], 0) != 3 || intFromAny(card["memory_id"], 0) != 2 {
				t.Fatal("latest wording lost its actual source")
			}
		}
	}
	if mustCompactJSON(inputs) != before || intFromAny(trace["auxiliary_selected_chars"], 0) > 1000 {
		t.Fatal("source mutation or budget excess")
	}
	// Whole-summary selection must not reintroduce repeated historical keys.
	_, large, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Collect workshop shield", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 100000})
	if len(criticReferenceCardsForTest(t, large)) != len(cards) {
		t.Fatal("summary selection duplicated reference identities")
	}
}

func TestCriticReferenceCardBudgetRollbackAndClosedWording(t *testing.T) {
	inputs := []map[string]any{}
	for turn, value := range []string{"Workshop shield awaits collection", "Workshop shield collected"} {
		inputs = append(inputs, map[string]any{"id": turn + 1, "turn_index": turn + 1, "recorded_state_claims": []any{map[string]any{"subject": "Workshop shield", "state_slot": "goal_status", "lifecycle_key": "shield", "value": value, "transition": []string{"create", "resolve"}[turn]}}})
	}
	_, full, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Workshop shield", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4000})
	card := criticReferenceCardsForTest(t, full)[0]
	if card["transition"] != "resolve" || card["value"] != "Workshop shield collected" {
		t.Fatal("older repeated goal reopened the reference")
	}
	price := intFromAny(trace["auxiliary_selected_chars"], 0)
	for _, budget := range []int{0, price - 1, price} {
		_, out, tr := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Workshop shield", completeTurnCriticInputPolicy{AuxiliaryMaxChars: budget})
		want := 0
		if budget == price {
			want = 1
		}
		if len(criticReferenceCardsForTest(t, out)) != want || intFromAny(tr["auxiliary_selected_chars"], 0) > budget {
			t.Fatalf("table overhead or rejected preview charged incorrectly at %d: %#v", budget, tr)
		}
	}
}

func TestCriticReferenceCardsKeepPerspectiveAndUnstructuredSummary(t *testing.T) {
	inputs := []map[string]any{
		{"id": 1, "turn_index": 1, "summary": "Mira arrived at the workshop."},
		{"id": 2, "turn_index": 2, "recorded_state_claims": []any{
			map[string]any{"subject": "Workshop shield", "state_slot": "location", "value": "Workshop chest", "claim_scope": "objective"},
			map[string]any{"subject": "Workshop shield", "state_slot": "location", "value": "Workshop roof", "claim_scope": "belief", "perspective_owner": "Mira"},
			map[string]any{"subject": "Workshop shield", "state_slot": "goal_status", "lifecycle_key": "shield", "value": "Collect workshop shield", "claim_scope": "objective"},
		}, "recorded_pending_threads": []any{map[string]any{"lifecycle_key": "shield", "description": "Collect workshop shield", "status": "open"}}},
	}
	_, ledger, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira arrived to collect the workshop shield", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4000})
	if len(criticReferenceCardsForTest(t, ledger)) != 3 {
		t.Fatalf("perspective merged or objective companion duplicated: %#v", ledger)
	}
	if !strings.Contains(mustCompactJSON(ledger), "Mira arrived at the workshop.") {
		t.Fatal("unstructured summary disappeared")
	}
}

func TestCriticPromptContiguousEvidenceAndReferenceCards(t *testing.T) {
	// The knowledge-scope additions were reverted after the 17-turn live
	// comparison lowered recorded boundaries; only the quote and card guidance stay.
	prompt, source := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if source == "fallback_builtin" {
		t.Fatal("source prompt not loaded")
	}
	for _, phrase := range []string{"one uninterrupted contiguous span", "Never insert ellipses, join separated fragments, change pronouns", "previous_canonical_turn", "reference_cards"} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("missing guidance %q", phrase)
		}
	}
}
