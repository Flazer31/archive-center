package httpapi

import (
	"crypto/sha256"
	"fmt"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"strings"
	"testing"
)

func TestAuditResidualNormalizedNoQueryAndGenuineMatches(t *testing.T) {
	card := "- [turn 1] " + prepareTurnProtectedCardGuard + "The courier knows the castle password.; owner=Rowan; subject=castle password; known_by=Rowan; unknown_to=Mira"
	run := func(query string, scene bool) map[string]any {
		out := &prepareTurnInjectionAssembly{ProtectedMemoryText: card}
		if scene {
			out.Counts = map[string]any{"stored_active_scene_entities": []string{"Rowan"}}
		}
		return buildPrepareTurnPriorityMemoryDeliveryPlan(out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: query})
	}
	for _, query := range []string{"", " ", "---", "___", "- -", "_ - _", "!?"} {
		t.Run("no_query_"+query, func(t *testing.T) {
			if got, want := prepareTurnAuthorityRelevanceScorer(query)(card), prepareTurnAuthorityRelevanceScorer("")(card); got != want {
				t.Fatalf("normalized no-query score=%g, want existing neutral=%g", got, want)
			}
			if mustCompactJSON(run(query, false)["protected_secret_budget"]) != mustCompactJSON(run("", false)["protected_secret_budget"]) {
				t.Fatal("normalized no-query selection differs from existing empty-query selection")
			}
			if got := mapFromAny(run(query, true)["protected_secret_budget"]); intFromAny(got["selected_count"], 0) != 1 {
				t.Fatal("genuine scene-owner match was lost", got)
			}
		})
	}
	for _, query := range []string{"castle password", "castle-password", "___castle password---", "Rowan"} {
		t.Run("match_"+query, func(t *testing.T) {
			plan := run(query, false)
			b := mapFromAny(plan["protected_secret_budget"])
			if intFromAny(b["selected_count"], 0) != 1 || !strings.Contains(stringFromMap(b, "final_text"), "unknown_to=Mira") {
				t.Fatal("genuine query/content/owner match lost its whole card or scope", b)
			}
			text := stringFromMap(b, "final_text")
			t.Logf("unchanged_query=%q sha256=%x chars=%d", query, sha256.Sum256([]byte(text)), len([]rune(text)))
		})
	}
}

func TestAuditResidualTitleIdentityBudgetAndSources(t *testing.T) {
	title, key := "Mira courier delivery", "mira-courier-delivery"
	ref := map[string]any{"title": title, "lifecycle_key": key, "description": strings.Repeat("Deliver the sealed letter before dawn. ", 80), "status": "open"}
	inputs := []map[string]any{{"id": 1, "turn_index": 1, "summary": title, "recorded_pending_threads": []any{ref}}}
	before := mustCompactJSON(inputs)
	for _, budget := range []int{0, 100, 200, 600, 4000} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, title, completeTurnCriticInputPolicy{AuxiliaryMaxChars: budget})
			rows := criticIdentityRowsForTest(t, ledger)
			indexCost := intFromAny(trace["identity_index_chars"], 0)
			if indexCost > budget || intFromAny(trace["auxiliary_selected_chars"], 0) > budget {
				t.Fatal("independent existing ceiling exceeded", trace)
			}
			if budget == 600 {
				if len(rows) != 1 || rows[0]["lifecycle_key"] != key || rows[0]["subject"] != title || intFromAny(rows[0]["source_turn"], 0) != 1 {
					t.Fatal("title-only key/title/source missing", ledger)
				}
				if len(criticReferenceCardsForTest(t, ledger)) != 0 || intFromAny(trace["auxiliary_selected_chars"], 0) != 0 {
					t.Fatal("identity index changed reference-card selection", trace)
				}
			}
			if budget == 4000 {
				cards := criticReferenceCardsForTest(t, ledger)
				if len(rows) != 0 || len(cards) != 1 || cards[0]["description"] != ref["description"] || cards[0]["title"] != title {
					t.Fatal("fitting reference card changed or duplicated", ledger)
				}
				t.Logf("unchanged_fitting_ledger_sha256=%x", sha256.Sum256([]byte(mustCompactJSON(ledger))))
			}
			t.Logf("budget=%d references=%d index_rows=%d index_chars=%d", budget, len(criticReferenceCardsForTest(t, ledger)), len(rows), indexCost)
		})
	}
	_, absent, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 600})
	if len(criticIdentityRowsForTest(t, absent)) != 0 {
		t.Fatal("partial title inferred a topic match", absent)
	}
	previous := []map[string]any{{"source": "previous_canonical_turn", "content": title}}
	_, ledger, _ := applyCompleteTurnCriticAuxiliaryBudget(previous, inputs, nil, nil, "A new scene", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 600})
	if rows := criticIdentityRowsForTest(t, ledger); len(rows) != 1 || rows[0]["lifecycle_key"] != key {
		t.Fatal("existing previous-turn mention lost title identity", ledger)
	}
	if mustCompactJSON(inputs) != before {
		t.Fatal("index changed stored reference inputs")
	}
	withSubject := cloneMapAny(ref)
	withSubject["subject"] = "Rowan"
	_, ledger, _ = applyCompleteTurnCriticAuxiliaryBudget(nil, []map[string]any{{"id": 1, "turn_index": 1, "recorded_pending_threads": []any{withSubject}}}, nil, nil, title, completeTurnCriticInputPolicy{AuxiliaryMaxChars: 600})
	if len(criticIdentityRowsForTest(t, ledger)) != 0 {
		t.Fatal("title replaced an explicitly stored subject", ledger)
	}
}

func TestAstraAuditPunctuationDoesNotSelectSecrets(t *testing.T) {
	for _, query := range []string{"---", "___", "- -"} {
		t.Run(query, func(t *testing.T) {
			out := &prepareTurnInjectionAssembly{ProtectedMemoryText: "- [turn 1] " + prepareTurnProtectedCardGuard + "The courier knows the castle password.; owner=Rowan; subject=castle password; known_by=Rowan; unknown_to=Mira"}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 18000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: query})
			budget := mapFromAny(plan["protected_secret_budget"])
			t.Logf("relevance=%g normalized_query=%q selected=%v text=%s", prepareTurnAuthorityRelevanceScorer(query)(out.ProtectedMemoryText), normalizePrepareTurnEntityNeedle(query), budget["selected_count"], stringFromMap(plan, "final_text"))
			if intFromAny(budget["selected_count"], 0) > 0 {
				t.Error("punctuation-only query admitted unrelated secret")
			}
		})
	}
	t.Run("full_assembly", func(t *testing.T) {
		m := store.Memory{ID: 1, ChatSessionID: "audit-synthetic", TurnIndex: 1, SummaryJSON: `{"turn_summary":"The courier knows the castle password.","protected_secrets":[{"summary":"The courier knows the castle password.","disclosure_policy":"owner_private_until_revealed"}]}`}
		out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{Memories: []store.Memory{m}, TopK: 5, MaxChars: 18000, UserInput: "---", BudgetMode: "auto", Perspective: &prepareTurnAssemblyPerspective{Selection: prepareTurnMemorySelectionContext{PriorityEnabled: true, Query: "---", CurrentTurn: 2}}})
		b := mapFromAny(out.MemoryDeliveryPlan["protected_secret_budget"])
		t.Logf("assembly selected=%v final=%s", b["selected_count"], stringFromMap(b, "final_text"))
		if intFromAny(b["selected_count"], 0) > 0 {
			t.Error("full assembly admitted unrelated secret for punctuation-only query")
		}
	})
}

func TestAstraAuditPendingThreadIdentityWhenReferenceDoesNotFit(t *testing.T) {
	inputs := []map[string]any{{"id": 1, "turn_index": 1, "summary": "Mira courier delivery", "recorded_pending_threads": []any{map[string]any{"title": "Mira courier delivery", "lifecycle_key": "mira-courier-delivery", "description": strings.Repeat("Deliver the sealed letter before dawn. ", 80), "status": "open"}}}}
	_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira courier delivery", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 600})
	t.Logf("ledger=%s trace=%s", mustCompactJSON(ledger), mustCompactJSON(trace))
	if strings.Contains(mustCompactJSON(ledger), "mira-courier-delivery") {
		return
	}
	t.Error("schema-valid title-only pending identity is absent from both reference cards and identity index despite fitting as a compact key")
}
