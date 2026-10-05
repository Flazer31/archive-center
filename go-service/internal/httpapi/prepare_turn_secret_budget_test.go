package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func secretBudgetCard32(owner, subject, claim string) string {
	return "- [turn 3] " + prepareTurnProtectedCardGuard + claim + "; owner=" + owner + "; subject=" + subject + "; known_by=Mira; unknown_to=Visitor; policy=owner_private_until_revealed"
}

func Test33SecretIndependentBudget(t *testing.T) {
	for _, mode := range []string{"auto", "custom"} {
		t.Run(mode, func(t *testing.T) {
			body, clock, character := bodyDeliveryFixture46()
			input := bodyAssemblyInput46(body, clock, character)
			input.MaxChars, input.BudgetMode = 800, mode
			input.Budgets = map[string]int{"character_objective": 800, "protected_secret": 50000}
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			beforeMain := stringFromMap(out.MemoryDeliveryPlan, "main_memory_text")
			beforeBody := stringFromMap(mapFromAny(out.MemoryDeliveryPlan["body_tracking_budget"]), "final_text")
			out.ProtectedMemoryText = secretBudgetCard32("Mira", "mask", strings.Repeat("hidden detail ", 100))
			plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, input.MaxChars, 5, mode, input.Budgets, prepareTurnMemorySelectionContext{Query: "Mira"})
			secret := mapFromAny(plan["protected_secret_budget"])
			if beforeMain == "" || beforeBody == "" || stringFromMap(plan, "main_memory_text") != beforeMain || stringFromMap(mapFromAny(plan["body_tracking_budget"]), "final_text") != beforeBody {
				t.Fatal("secret changed main/body allocation")
			}
			if intFromAny(secret["selected_count"], 0) != 1 || intFromAny(secret["used_chars"], 0) <= input.MaxChars || intFromAny(secret["cap_chars"], 0) != 4000 {
				t.Fatalf("secret did not use independent base: %v", secret)
			}
			stats := prepareTurnMemoryPayloadBudgetStats(plan, "")
			if stats.SelectedChars != len([]rune(beforeMain)) || stats.EffectiveCap != input.MaxChars {
				t.Fatalf("main charged secret/body: %+v", stats)
			}
			if intFromAny(secret["used_chars"], 0) != len([]rune(stringFromMap(secret, "final_text")))+2 {
				t.Fatal("secret separator missing")
			}
		})
	}
	caps, _ := prepareTurnPriorityDeliveryCaps(18000, "auto", nil)
	if caps["event_recent"] != 3500 || caps["protected_secret"] != 0 {
		t.Fatalf("ordinary denominator or secret reservation changed: %v", caps)
	}
	a, _ := prepareTurnPriorityDeliveryCaps(500, "custom", map[string]int{"event_recent": 400, "world_state": 200})
	b, _ := prepareTurnPriorityDeliveryCaps(500, "custom", map[string]int{"event_recent": 400, "world_state": 200, "protected_secret": 50000})
	if mustCompactJSON(a) != mustCompactJSON(b) {
		t.Fatal("legacy secret custom share consumed main")
	}
}

func Test33SecretSceneSelectionDoesNotFill(t *testing.T) {
	owner := secretBudgetCard32("Mask", "hidden role", "covert route")
	content := secretBudgetCard32("Rowan", "missing charter", "silver seal below tower")
	unrelated := secretBudgetCard32("Rowan", "winter orchard", "old apples stored")
	out := &prepareTurnInjectionAssembly{ProtectedMemoryText: strings.Join([]string{unrelated, content, owner}, "\n"), PriorityEntityAliases: map[string]any{"Mask": "Mira"}, Counts: map[string]any{"stored_active_scene_entities": []string{"Mira"}}}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 100000, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "silver seal"})
	budget := mapFromAny(plan["protected_secret_budget"])
	final := stringFromMap(budget, "final_text")
	if strings.Contains(final, "old apples") || !strings.Contains(final, "covert route") || !strings.Contains(final, "silver seal") || strings.Index(final, "silver seal") > strings.Index(final, "covert route") {
		t.Fatal("scene/owner/content selection incorrect", final)
	}
	if intFromAny(budget["excluded_count"], 0) != 1 || boolFromAny(budget["over_base"]) || intFromAny(budget["cap_chars"], 0) != 4000 {
		t.Fatal("sparse selection was filled or expanded", budget)
	}
	if strings.Count(final, strings.TrimSpace(prepareTurnProtectedCardGuard)) != 1 || !strings.Contains(final, "known_by=Mira; unknown_to=Visitor") {
		t.Fatal("shared guidance or stored boundary changed")
	}
	out.Counts = nil
	out.ProtectedMemoryText = unrelated
	plan = buildPrepareTurnPriorityMemoryDeliveryPlan(out, 100000, 1, "auto", nil, prepareTurnMemorySelectionContext{})
	if stringFromMap(plan, "final_text") != "" {
		t.Fatal("empty query neutral score became relevance")
	}
}

func Test33SecretConfiguredCeilingAndWholeCards(t *testing.T) {
	cards := []string{}
	for i := 0; i < 12; i++ {
		cards = append(cards, secretBudgetCard32("Mira", fmt.Sprint("artifact", i), fmt.Sprintf("hidden claim%d %s END%d", i, strings.Repeat("가", 710), i)))
	}
	for _, configured := range []int{0, 4000, 5000, 6000, 7351} {
		for _, count := range []int{1, len(cards)} {
			out := &prepareTurnInjectionAssembly{ProtectedSecretBudgetChars: configured, ProtectedMemoryText: strings.Join(cards[:count], "\n")}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 50, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "hidden"})
			b := mapFromAny(plan["protected_secret_budget"])
			text := stringFromMap(b, "final_text")
			want := configured
			if want <= 0 {
				want = prepareTurnProtectedBudgetBaseChars
			}
			if intFromAny(b["cap_chars"], 0) != want || len([]rune(text)) != intFromAny(b["used_chars"], 0) || len([]rune(text)) > want {
				t.Fatal("configured ceiling changed or exceeded", b)
			}
			if strings.Count(text, "END") != intFromAny(b["selected_count"], 0) || intFromAny(b["selected_count"], 0) == 0 {
				t.Fatal("whole-card delivery lost", b)
			}
			if count == len(cards) && intFromAny(b["budget_deferred_count"], 0) == 0 {
				t.Fatal("fixture did not exercise bounded selection")
			}
			if count == 1 && intFromAny(b["selected_count"], 0) != 1 {
				t.Fatal("sparse selection changed")
			}
		}
	}
}

func Test33SecretHTTPPayloadAndLedger(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	body, clock, character := bodyDeliveryFixture46()
	current := store.StatusCurrentValue{ID: 748, ChatSessionID: "body46", StatusKey: storyClockStatusKey, OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, SourceTurn: 20, ValueJSON: mustCompactJSON(clock)}
	secret := store.Memory{ID: 749, ChatSessionID: "body46", TurnIndex: 19, SummaryJSON: `{"turn_summary":"Mira keeps a disguise","protected_secrets":[{"summary":"Mira is the masked courier","owner":"Mira","secret_kind":"identity","subject":["Mira"],"knowledge_scope":{"known_by":["Mira"],"unknown_to":["Visitor"]},"disclosure_policy":"owner_private_until_revealed"}]}`}
	srv.Store = &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{returnCharStates: []store.CharacterState{character}, returnStatusCurrent: append(body.Values, current), returnMemories: []store.Memory{secret}}}
	if _, err := srv.saveBodyTrackingConfig("body46", body.Config); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := []byte(`{"chat_session_id":"body46","turn_index":21,"raw_user_input":"Mira checks her calendar in disguise.","settings":{"injection_enabled":true,"max_injection_chars":18000,"protected_secret_budget_chars":5000,"supervisor_enabled":false,"guide_strength":"none"}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(req)))
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	payload := mapFromAny(response["payload_application_plan"])
	memory := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
	budgets := mapFromAny(memory["protected_secret_budget"])
	lanes := map[string]map[string]any{}
	for _, raw := range outputFidelityLineageSlice(payload["lanes"]) {
		v := mapFromAny(raw)
		lanes[stringFromMap(v, "key")] = v
	}
	if !strings.Contains(stringFromMap(lanes["protected_secret"], "text"), "masked courier") || !strings.Contains(stringFromMap(lanes["protected_secret"], "text"), "unknown_to=Visitor") || strings.Contains(stringFromMap(lanes["long_term_memory"], "text"), "masked courier") {
		t.Fatal("secret missing, boundary lost, or double-injected")
	}
	if intFromAny(budgets["cap_chars"], 0) != 5000 || intFromAny(lanes["protected_secret"]["budget_chars"], 0) != intFromAny(budgets["cap_chars"], 0) || intFromAny(lanes["body_tracking"]["budget_chars"], 0) != 3000 {
		t.Fatal("payload budgets not separate")
	}
	if strings.Count(stringFromMap(payload, "auxiliary_text"), "masked courier") != 1 {
		t.Fatal("actual planned payload duplicates secret")
	}
	ledger := mapFromAny(payload["budget_ledger"])
	seen := false
	for _, raw := range outputFidelityLineageSlice(ledger["lanes"]) {
		v := mapFromAny(raw)
		if v["key"] == "protected_secret" {
			seen = true
			if intFromAny(v["final_delivery_chars"], 0) != len([]rune(stringFromMap(lanes["protected_secret"], "text"))) {
				t.Fatal("ledger text count mismatch")
			}
		}
	}
	if !seen || stringFromMap(budgets, "display_summary") == "" {
		t.Fatal("secret diagnostics absent")
	}
}
