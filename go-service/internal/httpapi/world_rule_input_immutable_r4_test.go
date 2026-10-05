package httpapi

import "testing"

func TestWorldRuleInputImmutableR4(t *testing.T) {
	for _, bucket := range []string{"world_rules", "world_state.rules"} {
		t.Run(bucket, func(t *testing.T) {
			value := "Synthetic observatory shutters close at dusk."
			rule := map[string]any{"value": value, "scope": " Building ", "scope_name": "observatory", "category": "institution"}
			repeat := cloneMapAny(rule)
			other := map[string]any{"key": "gate_state", "value": "open", "scope": "global"}
			items := []any{rule, repeat, other}
			extraction := map[string]any{"turn_summary": "Synthetic fixture"}
			if bucket == "world_rules" {
				extraction["world_rules"] = items
			} else {
				extraction["world_state"] = map[string]any{"version": "world_state.v1", "rules": items}
			}
			before := mustCompactJSON(extraction)
			got := worldRuleItemsForSave(extraction)
			if after := mustCompactJSON(extraction); after != before {
				t.Errorf("accepted input mutated: before=%s after=%s", before, after)
			}
			if len(got) != 2 {
				t.Fatalf("valid rules/deduplication changed: %#v", got)
			}
			first, second := mapFromAny(got[0]), mapFromAny(got[1])
			if stringFromMap(first, "scope") != "location" || stringFromMap(first, "key") != stableKey("world_rule", value) || stringFromMap(first, "value") != value || stringFromMap(first, "scope_name") != "observatory" {
				t.Errorf("normalized rule projection changed: %#v", first)
			}
			if stringFromMap(second, "scope") != "root" || stringFromMap(second, "key") != "gate_state" || stringFromMap(second, "value") != "open" {
				t.Errorf("explicit valid rule changed: %#v", second)
			}
			first["scope"] = "session"
			second["value"] = "closed"
			if after := mustCompactJSON(extraction); after != before {
				t.Errorf("returned rule maps share accepted input ownership: before=%s after=%s", before, after)
			}
		})
	}
}
