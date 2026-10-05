package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestProtectedDuplicateDoesNotRemoveOnlyFittingCard(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const claim = "Mira keeps the AZURE_VAULT_731 access phrase."
	memory := auditfixSecret(1, 1, claim, "", []string{"Mira"})
	ex := parseJSONMap(memory.SummaryJSON)
	secret := mapFromAny(sliceFromAny(ex["protected_secrets"])[0])
	unknown := []string{}
	for i := 0; i < 60; i++ {
		unknown = append(unknown, fmt.Sprintf("Uninformed observer %d", i))
	}
	mapFromAny(secret["knowledge_scope"])["unknown_to"] = unknown
	memory.SummaryJSON = mustCompactJSON(ex)
	unit := auditfixPerspectiveUnit("known-source", "known", claim, 1)
	// Derive the exact fitting budget from the production renderer for the
	// known holder's smaller card, including the shared protected guidance.
	perspective := map[string]any{"current_pov": "Mira", "current_pov_entity_id": "holder-mira", "identity_state": "resolved"}
	_, known := buildCharacterPerspectivePacket([]store.PreciseMemoryUnit{unit}, perspective, 5000)
	if known == "" {
		t.Fatal("known positive control missing")
	}
	known = strings.Replace(known, "- ", "- "+prepareTurnProtectedCardGuard, 1)
	out := &prepareTurnInjectionAssembly{ProtectedMemoryText: known, ProtectedSecretBudgetChars: 5000}
	control := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 5000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira AZURE_VAULT_731"})
	cap := intFromAny(mapFromAny(control["protected_secret_budget"])["used_chars"], 0)
	if cap <= 0 {
		t.Fatalf("invalid fitting budget: %#v", control)
	}
	st := &auditfixPerspectiveStore{perspectiveIdentityTestStore: &perspectiveIdentityTestStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{memory}}, resolvedID: "holder-mira"}, units: []store.PreciseMemoryUnit{unit}, t: t}
	srv := NewServer(config.Default())
	srv.Store = st
	response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": "auditfix-synthetic", "turn_index": 4, "raw_user_input": "Mira AZURE_VAULT_731",
		"client_meta": map[string]any{"perspective_context": map[string]any{"current_pov": "Mira"}},
		"settings":    map[string]any{"max_injection_chars": 5000, "protected_secret_budget_chars": cap, "injection_enabled": true, "guide_strength": "none", "top_k": 5}})
	plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
	budget := mapFromAny(plan["protected_secret_budget"])
	text := stringFromMap(budget, "final_text")
	if strings.Count(text, "AZURE_VAULT_731") != 1 {
		t.Fatalf("dedup removed the only fitting known card: cap=%d budget=%v text=%s", cap, budget, text)
	}
	if intFromAny(budget["used_chars"], 0) > cap {
		t.Fatal("secret budget exceeded")
	}
}
