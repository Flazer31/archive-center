package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPublicScopeR3NamedAnnotationAuthority(t *testing.T) {
	scope := map[string]any{"known_by": []string{"Courier"}, "unknown_to": []string{"Outsider"}, "suspected_by": []string{"Observer"}, "misinformed_by": []string{"Watcher"}, "revealed_to": []string{"Recipient"}}
	t.Run("public_annotation_does_not_export_authority", func(t *testing.T) {
		item := map[string]any{"fact_id": "F42", "summary": "Courier checks the public cart", "visibility": " PUBLIC ", "knowledge_scope": scope}
		before := mustCompactJSON(item)
		seed := carryIntegrationR1Seed("pending_threads", 23, []string{"F42"})
		out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{seed}}
		reading := mustCompactJSON(seed.Fact.Reading)
		prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{{ID: 12, ChatSessionID: "integration-r1", SummaryJSON: mustCompactJSON(map[string]any{"events": []any{item}})}}})
		if len(prepareTurnOwnKnowledgeBoundary(item)) != 0 || len(out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries) != 0 || reading != mustCompactJSON(out.PriorityFactSeeds[0].Fact.Reading) {
			t.Fatal("public annotation became authority on its explicitly linked descendant")
		}
		if len(prepareTurnOwnKnowledgeBoundary(item, true)) != 1 {
			t.Fatal("existing classified authority lost scope")
		}
		protected := cloneMapAny(item)
		protected["secret_guard"] = true
		if len(prepareTurnOwnKnowledgeBoundary(protected)) != 1 {
			t.Fatal("existing explicit protected authority lost scope")
		}
		if before != mustCompactJSON(item) {
			t.Fatal("public source mutated")
		}
	})
	t.Run("public_historical_and_current_named_scopes_remain_distinct", func(t *testing.T) {
		_, prior := continuityR3Seed(t)
		item := parseJSONMap(mustCompactJSON(map[string]any{"subject": "Courier", "value": "Delivery accepted", "state_slot": "goal_status", "visibility": "public", "knowledge_scope": scope, "knowledge_boundaries": prior["knowledge_boundaries"]}))
		before := mustCompactJSON(item)
		boundaries := prepareTurnOwnKnowledgeBoundary(item)
		historical := sliceFromAny(item["knowledge_boundaries"])
		if len(boundaries) != len(historical)+1 {
			t.Fatal("historical boundary or separate current public scope lost")
		}
		for i, raw := range historical {
			if mustCompactJSON(boundaries[i]) != mustCompactJSON(raw) {
				t.Fatal("historical source scope rewritten")
			}
		}
		if mustCompactJSON(boundaries[len(historical)]["knowledge_scope"]) != mustCompactJSON(scope) {
			t.Fatal("current public names replaced by historical names")
		}
		facts := prepareTurnPriorityStructuredMemoryItemFacts("state_claims", item)
		if len(facts) == 0 || mustCompactJSON(facts[0].KnowledgeBoundaries) != mustCompactJSON(boundaries) {
			t.Fatal("structured source reader lost distinct boundaries")
		}
		continuityR3Delivery(t, store.PreciseMemoryUnit{UnitID: "synthetic-public-scope-r3", ChatSessionID: reviewR2SID, Kind: "state", Subtype: "goal_status", Visibility: "public", SourceTurnStart: 2, PayloadJSON: mustCompactJSON(item)}, boundaries)
		if before != mustCompactJSON(item) {
			t.Fatal("historical/current source mutated")
		}
	})
}
