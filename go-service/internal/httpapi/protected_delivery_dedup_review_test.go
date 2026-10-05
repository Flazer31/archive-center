package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestProtectedPerspectiveStandaloneAndJoinedBudget(t *testing.T) {
	perspective := map[string]any{"current_pov": "Mira", "current_pov_entity_id": "holder-mira", "identity_state": "resolved"}
	_, known := buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{auditfixPerspectiveUnit("known-source", "known", "Mira keeps the AZURE_VAULT_731 access phrase.", 1)}, perspective, 5000)
	known = strings.Replace(known, "- ", "- "+prepareTurnProtectedCardGuard, 1)
	selection := prepareTurnMemorySelectionContext{Query: "Mira AZURE_VAULT_731"}
	control := &prepareTurnInjectionAssembly{ProtectedMemoryText: known, ProtectedSecretBudgetChars: 5000}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(control, 5000, 5, "auto", nil, selection)
	cost := intFromAny(mapFromAny(plan["protected_secret_budget"])["used_chars"], 0)
	if cost <= 0 {
		t.Fatal("production-rendered positive control missing")
	}
	for _, tc := range []struct {
		name                string
		cap                 int
		direct              string
		selected, separator int
	}{
		{"exact_standalone", cost, "", 1, 0},
		{"one_char_short", cost - 1, "", 0, 0},
		{"joined_separator_does_not_fit", cost, "[Latest Direct Evidence]\n- Mira visits the vault.", 0, 0},
		{"joined_separator_fits", cost + 2, "[Latest Direct Evidence]\n- Mira visits the vault.", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := &prepareTurnInjectionAssembly{ProtectedMemoryText: known, ProtectedSecretBudgetChars: tc.cap, DirectEvidenceText: tc.direct}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 5000, 5, "auto", nil, selection)
			budget := mapFromAny(plan["protected_secret_budget"])
			used := intFromAny(budget["used_chars"], 0)
			text := stringFromMap(budget, "final_text")
			if intFromAny(budget["selected_count"], 0) != tc.selected || intFromAny(budget["separator_chars"], 0) != tc.separator || used > tc.cap || used != len([]rune(text))+tc.separator {
				t.Fatalf("rendered budget mismatch: cost=%d budget=%v", cost, budget)
			}
		})
	}
}

func TestProtectedPerspectiveDeliveryIdentityControls(t *testing.T) {
	const claim = "Mira knows the sealed vault phrase."
	for _, name := range []string{"different_artifact", "different_revision", "distinct_boundaries", "public_source"} {
		t.Run(name, func(t *testing.T) {
			memory := auditfixSecret(1, 1, claim, "seal-A", []string{"Mira"})
			unit := auditfixPerspectiveUnit("identity-control", "known", claim, 1)
			payload := parseJSONMap(unit.PayloadJSON)
			payload["artifact_id"] = "seal-A"
			memories := []store.Memory{memory}
			ex := parseJSONMap(memory.SummaryJSON)
			wantProtectedOccurrences := 2
			switch name {
			case "different_artifact":
				payload["artifact_id"] = "seal-B"
			case "different_revision":
				ex["source_revision"] = "revision-A"
				unit.SourceRevision = "revision-B"
			case "distinct_boundaries":
				second := auditfixSecret(2, 1, claim, "seal-A", []string{"Mira"})
				secondMap := parseJSONMap(second.SummaryJSON)
				secret := mapFromAny(sliceFromAny(secondMap["protected_secrets"])[0])
				mapFromAny(secret["knowledge_scope"])["unknown_to"] = []string{"Courier"}
				second.SummaryJSON = mustCompactJSON(secondMap)
				memories = append(memories, second)
				wantProtectedOccurrences = 3
			case "public_source":
				secret := mapFromAny(sliceFromAny(ex["protected_secrets"])[0])
				secret["disclosure_policy"] = "public"
				mapFromAny(secret["knowledge_scope"])["publicly_revealed"] = true
				wantProtectedOccurrences = 1
			}
			memories[0].SummaryJSON = mustCompactJSON(ex)
			unit.PayloadJSON = mustCompactJSON(payload)
			pack := auditfixPreparePerspective(t, memories, []store.PreciseMemoryUnit{unit}, 5000, "Mira")
			budget := mapFromAny(mapFromAny(pack["memory_delivery_plan"])["protected_secret_budget"])
			text := stringFromMap(budget, "final_text")
			if strings.Count(text, claim) != wantProtectedOccurrences || !strings.Contains(text, "- known | Mira / plan: "+claim) || intFromAny(budget["same_source_duplicate_count"], 0) != 0 {
				t.Fatalf("independent identity/boundary suppressed: budget=%v", budget)
			}
			if _, exists := mapFromAny(pack["character_perspective_packet"])["_character_perspective_protected_units"]; exists {
				t.Fatal("request-only source metadata reached response")
			}
		})
	}
}
