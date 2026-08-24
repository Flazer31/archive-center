package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func combinedCriticPromptForTest(t *testing.T, userPrompt string) string {
	t.Helper()
	systemPrompt, source := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if source == "fallback_builtin" {
		t.Fatal("source critic_system.txt was not loaded")
	}
	return systemPrompt + "\n" + userPrompt
}

func criticWireJSONForTest(canonical map[string]any) string {
	raw, err := json.Marshal(canonical)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestCriticWireContractPreservesEveryCanonicalSurface(t *testing.T) {
	textSurfaces := []string{"evidence_excerpts", "prune_targets"}
	arraySurfaces := []string{
		"kg_triples", "character_deltas", "pending_threads", "speaker_attributions", "world_rules",
		"reversible_states", "physical_conditions", "entity_conditions", "narrative_events", "state_claims",
		"belief_updates", "subjective_entity_memories", "protected_secrets", "character_identity_accuracy",
		"persona_capsule_candidates", "interaction_events", "relationship_observations", "interaction_boundaries",
		"habit_observations", "character_profile_observations", "voice_observations", "user_interaction_profile",
		"rp_character_profile",
	}
	objectSurfaces := []string{"entities", "relationship_memory", "state_deltas", "world_rule_audit", "world_state", "archive_hint", "story_clock"}
	wire := map[string]any{
		"turn_summary":           "Mina kept the brass key.",
		"importance_score":       float64(7),
		"emotional_intensity":    float64(0.4),
		"narrative_significance": float64(0.8),
	}
	for _, surface := range textSurfaces {
		wire[surface] = []any{surface + " text"}
	}
	for _, surface := range arraySurfaces {
		wire[surface] = []any{map[string]any{"marker": surface}}
	}
	for _, surface := range objectSurfaces {
		wire[surface] = map[string]any{"marker": surface}
	}

	canonical, quarantine, err := validateCriticExtractionSchema(wire)
	if err != nil || quarantine != nil {
		t.Fatalf("wire contract conversion failed: err=%v quarantine=%#v", err, quarantine)
	}
	for _, surface := range textSurfaces {
		if values := stringsFromAny(canonical[surface]); len(values) != 1 || values[0] != surface+" text" {
			t.Fatalf("text surface %s was not preserved: %#v", surface, canonical[surface])
		}
	}
	for _, surface := range arraySurfaces {
		values := sliceFromAny(canonical[surface])
		if len(values) != 1 || stringFromMap(mapFromAny(values[0]), "marker") != surface {
			t.Fatalf("array surface %s was not preserved: %#v", surface, canonical[surface])
		}
	}
	for _, surface := range objectSurfaces {
		if stringFromMap(mapFromAny(canonical[surface]), "marker") != surface {
			t.Fatalf("object surface %s was not preserved: %#v", surface, canonical[surface])
		}
	}
	for _, field := range []string{"turn_summary", "importance_score", "emotional_intensity", "narrative_significance"} {
		if _, exists := canonical[field]; !exists {
			t.Fatalf("core field %s was not preserved: %#v", field, canonical)
		}
	}
}

func TestCriticSparseTopLevelWireRemovesGroupedOverheadWithoutDroppingItems(t *testing.T) {
	items := []any{}
	for index := 1; index <= 30; index++ {
		item := map[string]any{
			"subject":          fmt.Sprintf("entity-%d", index),
			"predicate":        "observed",
			"object":           fmt.Sprintf("fact-%d", index),
			"evidence_excerpt": fmt.Sprintf("evidence-%d", index),
		}
		items = append(items, item)
	}
	canonical := map[string]any{
		"turn_summary":     "Thirty durable facts were observed.",
		"importance_score": 6,
		"kg_triples":       items,
	}
	sparseJSON := criticWireJSONForTest(canonical)
	groupedJSON, err := json.Marshal(map[string]any{
		"contract_version": "critic_output.v1",
		"turn_summary":     canonical["turn_summary"],
		"importance_score": canonical["importance_score"],
		"records":          []any{map[string]any{"surface": "kg_triples", "items": items}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sparseJSON) >= len(groupedJSON) {
		t.Fatalf("sparse top-level wire did not remove grouped overhead: sparse=%d grouped=%d", len(sparseJSON), len(groupedJSON))
	}
	var wire map[string]any
	if err := json.Unmarshal([]byte(sparseJSON), &wire); err != nil {
		t.Fatal(err)
	}
	converted, quarantine, err := validateCriticExtractionSchema(wire)
	if err != nil || quarantine != nil || len(sliceFromAny(converted["kg_triples"])) != len(items) {
		t.Fatalf("sparse top-level wire changed item fidelity: err=%v quarantine=%#v converted=%#v", err, quarantine, converted)
	}
}

func TestCriticCanonicalContextUsesPreviousTurnAndRelevantMemorySources(t *testing.T) {
	longSource := "Mina hid the brass key in the lighthouse vault. " + strings.Repeat("source-detail-", 140)
	longSummary := "Mina and Rowan hid the brass key in the lighthouse vault. " + strings.Repeat("memory-detail-", 80)
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "critic-context", TurnIndex: 1, Role: "user", Content: longSource},
			{ChatSessionID: "critic-context", TurnIndex: 1, Role: "assistant", Content: "Rowan watched the lighthouse vault."},
			{ChatSessionID: "critic-context", TurnIndex: 2, Role: "user", Content: "Mina mapped the brass key route."},
			{ChatSessionID: "critic-context", TurnIndex: 2, Role: "assistant", Content: "Rowan checked the lighthouse vault."},
			{ChatSessionID: "critic-context", TurnIndex: 3, Role: "user", Content: "Rowan returned the brass key."},
			{ChatSessionID: "critic-context", TurnIndex: 3, Role: "assistant", Content: "Mina guarded the lighthouse vault."},
			{ChatSessionID: "critic-context", TurnIndex: 4, Role: "user", Content: "Mina carried the brass key."},
			{ChatSessionID: "critic-context", TurnIndex: 4, Role: "assistant", Content: "Rowan entered the lighthouse vault."},
			{ChatSessionID: "critic-context", TurnIndex: 99, Role: "user", Content: "previous user full text"},
			{ChatSessionID: "critic-context", TurnIndex: 99, Role: "assistant", Content: "previous assistant full text"},
			{ChatSessionID: "critic-context", TurnIndex: 100, Role: "user", Content: "current row must not repeat"},
		},
		returnMemories: []store.Memory{
			{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":` + strconv.Quote(longSummary) + `}`, Importance: 0.8},
			{ID: 2, TurnIndex: 98, SummaryJSON: `{"turn_summary":"Carol cooked mushroom soup in the village kitchen."}`, Importance: 0.9},
			{ID: 3, TurnIndex: 101, SummaryJSON: `{"turn_summary":"Future Mina retrieved the brass key."}`, Importance: 1},
			{ID: 4, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Private brass key identity","protected_secrets":[{"knowledge_scope":{"publicly_revealed":false}}]}`, Importance: 1},
			{ID: 5, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Mina and Rowan mapped the brass key route inside the lighthouse vault."}`, Importance: 0.7},
			{ID: 6, TurnIndex: 3, SummaryJSON: `{"turn_summary":"Rowan told Mina the brass key opens the lighthouse vault."}`, Importance: 0.7},
			{ID: 7, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Mina carried the brass key toward Rowan at the lighthouse vault."}`, Importance: 0.7},
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	contextMessages, memories, trace := srv.buildCompleteTurnCriticCanonicalContext(
		context.Background(), "critic-context", 100,
		"Mina asks Rowan to retrieve the brass key from the lighthouse vault.", 238,
	)
	if intFromAny(trace["host_messages_received"], 0) != 238 || intFromAny(trace["host_messages_used"], -1) != 0 {
		t.Fatalf("host context was not excluded: %#v", trace)
	}
	if len(memories) != 4 {
		t.Fatalf("relevant memory selection = %#v, want four relevant memories without a fixed item cap", memories)
	}
	longMemoryFound := false
	for _, memory := range memories {
		if intFromAny(memory["id"], 0) == 1 && stringFromMap(memory, "summary") == longSummary {
			longMemoryFound = true
		}
	}
	if !longMemoryFound {
		t.Fatalf("long relevant memory was changed or omitted: %#v", memories)
	}
	encoded, _ := json.Marshal(contextMessages)
	text := string(encoded)
	for _, expected := range []string{"previous user full text", "previous assistant full text", longSource, "Rowan watched the lighthouse vault"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("canonical context missing %q: %s", expected, text)
		}
	}
	for _, blocked := range []string{"current row must not repeat", "mushroom soup", "Future Mina", "Private brass key identity"} {
		if strings.Contains(text, blocked) {
			t.Fatalf("invalid context %q leaked: %s", blocked, text)
		}
	}
}

func TestCriticAuxiliaryBudgetKeepsPreviousTurnWholeAndSelectsOnlyRelevantDBSupport(t *testing.T) {
	previousUser := "previous-user-start " + strings.Repeat("previous-user-detail-", 120) + " previous-user-end"
	previousAssistant := "previous-assistant-start " + strings.Repeat("previous-assistant-detail-", 120) + " previous-assistant-end"
	contextMessages := []map[string]any{
		{"role": "user", "content": previousUser, "turn_index": 9, "source": "previous_canonical_turn"},
		{"role": "assistant", "content": previousAssistant, "turn_index": 9, "source": "previous_canonical_turn"},
		{"role": "user", "content": "Mina placed the brass key by the lighthouse vault.", "turn_index": 2, "source": "relevant_memory_source_turn"},
		{"role": "assistant", "content": "Rowan promised to guard the brass key.", "turn_index": 2, "source": "relevant_memory_source_turn"},
	}
	relevantMemories := []map[string]any{
		{"id": 21, "turn_index": 2, "summary": "Mina and Rowan hid the brass key in the lighthouse vault."},
	}
	ledger := map[string]any{
		"contract_version": "critic_archive_ledger.v1",
		"items": []any{
			map[string]any{"id": "ledger-related", "lane": "memory", "summary": "Mina and Rowan must recover the brass key from the lighthouse vault."},
			map[string]any{"id": "ledger-unrelated", "lane": "memory", "summary": "Carol cooked mushroom soup in the village kitchen."},
		},
	}
	rules := []map[string]any{
		{"scope": "scene", "scope_name": "lighthouse vault", "key": "vault-key-rule", "value": "The brass key opens the lighthouse vault."},
		{"scope": "scene", "scope_name": "village kitchen", "key": "soup-rule", "value": "Mushroom soup must simmer for one hour."},
		{"scope": "global", "key": "oversized-root-rule", "value": strings.Repeat("global archive law ", 200)},
	}
	policy := completeTurnCriticInputPolicy{AuxiliaryMaxChars: 800, ConfiguredChars: 800, Source: "test"}
	selectedContext, selectedLedger, trace := applyCompleteTurnCriticAuxiliaryBudget(
		contextMessages, relevantMemories, ledger, rules,
		"Mina asks Rowan to use the brass key at the lighthouse vault.", policy,
	)

	if len(selectedContext) < 2 || stringFromMap(selectedContext[0], "content") != previousUser || stringFromMap(selectedContext[1], "content") != previousAssistant {
		t.Fatalf("mandatory previous turn was changed or omitted: %#v", selectedContext)
	}
	if intFromAny(trace["auxiliary_selected_chars"], -1) > policy.AuxiliaryMaxChars {
		t.Fatalf("auxiliary selection exceeded the shared budget: %#v", trace)
	}
	if boolFromAny(trace["current_turn_bounded"]) || boolFromAny(trace["previous_turn_bounded"]) || intFromAny(trace["truncated_count"], -1) != 0 {
		t.Fatalf("current/previous turn or a partial DB item was truncated: %#v", trace)
	}
	encoded, _ := json.Marshal(selectedLedger)
	if strings.Contains(string(encoded), "mushroom soup") || strings.Contains(string(encoded), "soup-rule") {
		t.Fatalf("unrelated DB support reached the critic input: %s", encoded)
	}
	seenWorldRuleDecision := false
	seenUnrelatedDecision := false
	for _, key := range []string{"selected", "excluded"} {
		entries, _ := trace[key].([]map[string]any)
		for _, item := range entries {
			if stringFromMap(item, "kind") == "active_world_rule" {
				seenWorldRuleDecision = true
			}
			if stringFromMap(item, "id") == "ledger-unrelated" && stringFromMap(item, "reason") == "not_related_to_current_or_previous_turn" {
				seenUnrelatedDecision = true
			}
		}
	}
	if !seenWorldRuleDecision || !seenUnrelatedDecision {
		t.Fatalf("selected/excluded DB support decisions are incomplete: %#v", trace)
	}
}

func TestCriticInputPolicyUsesObservedUserBudgetWithoutAnItemCountCap(t *testing.T) {
	cfg := config.Default()
	cfg.CriticLedgerEnabled = true
	srv := NewServer(cfg)
	policy := srv.completeTurnCriticInputPolicy(map[string]any{
		"critic_input_budget_observation": map[string]any{
			"contract_version":        completeTurnCriticInputBudgetObservationContract,
			"max_input_context_chars": 975,
		},
	})
	if policy.ConfiguredChars != 975 || policy.Source != "risu_host_setting_observation" {
		t.Fatalf("observed Critic input budget was not used: %+v", policy)
	}
	if policy.AuxiliaryMaxChars != policy.ConfiguredChars+policy.LedgerChars {
		t.Fatalf("auxiliary budget does not combine the existing context and ledger budgets: %+v", policy)
	}
}

func TestCriticProviderPromptKeepsCurrentAndPreviousTurnsWholeAndExcludesHostHistory(t *testing.T) {
	currentUser := "current-user-start " + strings.Repeat("current-user-detail-", 180) + " current-user-end"
	currentAssistant := "current-assistant-start " + strings.Repeat("current-assistant-detail-", 180) + " current-assistant-end"
	previousUser := "previous-user-start " + strings.Repeat("previous-user-detail-", 100) + " previous-user-end"
	previousAssistant := "previous-assistant-start " + strings.Repeat("previous-assistant-detail-", 100) + " previous-assistant-end"
	fake := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "critic-provider-context", TurnIndex: 7, Role: "user", Content: previousUser},
		{ChatSessionID: "critic-provider-context", TurnIndex: 7, Role: "assistant", Content: previousAssistant},
	}}
	srv := NewServer(config.Default())
	srv.Store = fake

	oldClient := proxyHTTPClient
	providerSystemPrompt := ""
	providerUserPrompt := ""
	providerResponse := criticWireJSONForTest(map[string]any{
		"turn_summary":      "Mina and Rowan continue.",
		"importance_score":  5,
		"evidence_excerpts": []any{},
	})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		messages := sliceFromAny(request["messages"])
		if len(messages) >= 2 {
			providerSystemPrompt = stringFromMap(mapFromAny(messages[0]), "content")
			providerUserPrompt = stringFromMap(mapFromAny(messages[1]), "content")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"critic-test","choices":[{"message":{"content":%s}}],"usage":{"prompt_tokens":321,"completion_tokens":45,"total_tokens":366}}`, strconv.Quote(providerResponse)))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, trace, err := srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-provider-context", 8,
		currentUser, currentAssistant,
		[]map[string]any{{"role": "user", "content": "HOST_FULL_HISTORY_MUST_NOT_REACH_PROVIDER"}},
		nil,
		completeTurnLLMConfig{Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key", Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0)},
		true,
		completeTurnCriticInputPolicy{AuxiliaryMaxChars: 2_000, ConfiguredChars: 2_000, Source: "test"},
		completeTurnCriticInputReplay{},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{currentUser, currentAssistant, previousUser, previousAssistant} {
		if !strings.Contains(providerUserPrompt, expected) {
			t.Fatalf("provider prompt omitted or changed full turn text %q", expected[:20])
		}
	}
	if strings.Contains(providerUserPrompt, "HOST_FULL_HISTORY_MUST_NOT_REACH_PROVIDER") {
		t.Fatal("whole host chat history reached the critic provider")
	}
	if strings.Contains(providerUserPrompt, "<Deterministic_Preview_Pass_JSON>") || strings.Contains(providerUserPrompt, "recent_raw_preview") {
		t.Fatal("duplicated preview payload reached the main critic provider prompt")
	}
	if _, ok := trace["preview_pass"]; ok {
		t.Fatalf("removed preview pass remained in the critic trace: %#v", trace["preview_pass"])
	}
	if fake.listEvidenceCalls != 0 {
		t.Fatalf("critic prompt preparation performed %d unused full evidence reads", fake.listEvidenceCalls)
	}
	budgetTrace := mapFromAny(trace["input_budget"])
	if boolFromAny(budgetTrace["current_turn_bounded"]) || boolFromAny(budgetTrace["current_turn_content_changed"]) {
		t.Fatalf("current turn was reported as changed: %#v", budgetTrace)
	}
	if intFromAny(budgetTrace["current_turn_chars"], 0) != len([]rune(currentUser))+len([]rune(currentAssistant)) {
		t.Fatalf("current turn size trace mismatch: %#v", budgetTrace)
	}
	if intFromAny(budgetTrace["system_prompt_chars"], 0) != len([]rune(providerSystemPrompt)) ||
		intFromAny(budgetTrace["user_prompt_chars"], 0) != len([]rune(providerUserPrompt)) ||
		intFromAny(budgetTrace["final_prompt_chars"], 0) != len([]rune(providerSystemPrompt))+len([]rune(providerUserPrompt)) {
		t.Fatalf("final prompt size trace mismatch: %#v", budgetTrace)
	}
	callLedger := mapFromAny(trace["provider_call_budget_ledger"])
	if callLedger["contract_version"] != providerCallBudgetLedgerContractV1 || callLedger["owner"] != "go" ||
		callLedger["call_kind"] != "critic" || callLedger["status"] != "succeeded" || callLedger["failure_stage"] != "" {
		t.Fatalf("critic call ledger contract/status mismatch: %#v", callLedger)
	}
	if intFromAny(callLedger["current_turn_chars"], 0) != len([]rune(currentUser))+len([]rune(currentAssistant)) ||
		intFromAny(callLedger["auxiliary_memory_chars"], 0) <= 0 ||
		intFromAny(callLedger["original_work_reference_chars"], -1) != 0 || callLedger["original_work_reference_status"] != "not_in_call_contract" ||
		intFromAny(callLedger["lorebook_reference_chars"], -1) != 0 || callLedger["lorebook_reference_status"] != "not_in_call_contract" ||
		callLedger["json_schema_output_requirement_accounting"] != "embedded_in_system_prompt_not_separable" {
		t.Fatalf("critic call ledger lane accounting mismatch: %#v", callLedger)
	}
	if callLedger["provider_usage_status"] != "reported" || intFromAny(callLedger["input_tokens"], 0) != 321 || intFromAny(callLedger["output_tokens"], 0) != 45 {
		t.Fatalf("critic provider usage observation mismatch: %#v", callLedger)
	}
	selectionTrace := mapFromAny(trace["context_selection"])
	if intFromAny(selectionTrace["host_messages_received"], 0) != 1 || intFromAny(selectionTrace["host_messages_used"], -1) != 0 {
		t.Fatalf("host history exclusion trace mismatch: %#v", selectionTrace)
	}
}

func TestCriticProviderReasoningTransportKeepsSingleCompactCall(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		endpoint      string
		model         string
		preset        string
		effort        string
		budget        int64
		wantMaxTokens float64
	}{
		{
			name: "ollama deepseek low reasoning", provider: "ollama", endpoint: "http://127.0.0.1:11434/v1",
			model: "deepseek-v4-pro:0813-cloud", preset: "auto", effort: "low", budget: 64, wantMaxTokens: 576,
		},
		{
			name: "llm gateway luna low reasoning", provider: "llmgateway", endpoint: "https://api.llmgateway.io/v1",
			model: "gpt-5.6-luna", preset: "auto", effort: "low", budget: 64, wantMaxTokens: 512,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			callCount := 0
			requestBody := map[string]any{}
			providerResponse := criticWireJSONForTest(map[string]any{
				"turn_summary": "Mina kept the key.", "importance_score": 6,
				"kg_triples": []any{map[string]any{"subject": "Mina", "predicate": "kept", "object": "key"}},
			})
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				callCount++
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Fatal(err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(fmt.Sprintf(
						`{"model":"critic-test","choices":[{"finish_reason":"stop","message":{"content":%s}}]}`,
						strconv.Quote(providerResponse),
					))),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
			result, _, err := srv.runCompleteTurnCritic(
				context.Background(), "session", 1,
				"Mina found the key.", "Mina kept the key safe.", nil, nil,
				completeTurnLLMConfig{
					Provider: tt.provider, Endpoint: tt.endpoint, APIKey: "test-key", Model: tt.model,
					MaxTokens: 512, MaxCompletionTokens: 512, TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0),
					ReasoningPreset: tt.preset, ReasoningEffort: tt.effort, ReasoningBudgetTokens: tt.budget,
				},
			)
			if err != nil || callCount != 1 || stringFromMap(result, "turn_summary") != "Mina kept the key." {
				t.Fatalf("single compact critic call failed: err=%v calls=%d result=%#v", err, callCount, result)
			}
			if requestBody["reasoning_effort"] != tt.effort || requestBody["max_tokens"] != tt.wantMaxTokens {
				t.Fatalf("reasoning request mismatch: body=%#v", requestBody)
			}
			messages := sliceFromAny(requestBody["messages"])
			if len(messages) != 2 {
				t.Fatalf("critic message count=%d want=2", len(messages))
			}
			userPrompt := stringFromMap(mapFromAny(messages[1]), "content")
			if strings.Contains(userPrompt, "<Deterministic_Preview_Pass_JSON>") || strings.Contains(userPrompt, "recent_raw_preview") {
				t.Fatalf("compact critic prompt regained duplicated preview payload: %s", userPrompt)
			}
		})
	}
}

func TestCompleteTurnCriticLanguageUsesOnlyAssistantOutputObservation(t *testing.T) {
	tests := []struct {
		name       string
		observed   string
		want       string
		wantSource string
	}{
		{name: "korean output overrides japanese request", observed: "ko", want: "ko", wantSource: "current_assistant"},
		{name: "mixed output does not fall back", observed: "mixed", want: "auto", wantSource: "assistant_output_unknown"},
		{name: "unknown output does not fall back", observed: "unknown", want: "auto", wantSource: "assistant_output_unknown"},
		{name: "missing output does not fall back", observed: "", want: "auto", wantSource: "assistant_output_unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			languageContext := completeTurnCriticLanguageContextFromAssistantOutput(map[string]any{
				"session_output_language":   "ja",
				"summary_language":          "ja",
				"output_language_source":    "explicit_override",
				"assistant_output_language": tc.observed,
				"raw_user_language":         "ja",
				"ui_language":               "ja",
			})
			if got := extractionStringFromAny(languageContext["session_output_language"]); got != tc.want {
				t.Fatalf("session output language=%q, want %q: %#v", got, tc.want, languageContext)
			}
			if got := extractionStringFromAny(languageContext["summary_language"]); got != tc.want {
				t.Fatalf("summary language=%q, want %q: %#v", got, tc.want, languageContext)
			}
			if got := extractionStringFromAny(languageContext["output_language_source"]); got != tc.wantSource {
				t.Fatalf("output language source=%q, want %q: %#v", got, tc.wantSource, languageContext)
			}
		})
	}
}

func TestCriticReprocessingReplaysExactPersistedDynamicInput(t *testing.T) {
	fake := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "critic-replay", TurnIndex: 11, Role: "user", Content: "original previous user"},
		{ChatSessionID: "critic-replay", TurnIndex: 11, Role: "assistant", Content: "original previous assistant"},
	}}
	srv := NewServer(config.Default())
	srv.Store = fake

	oldClient := proxyHTTPClient
	providerPrompts := []string{}
	providerResponse := criticWireJSONForTest(map[string]any{
		"turn_summary":      "미나는 금고를 열었다.",
		"importance_score":  7,
		"evidence_excerpts": []any{},
	})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		messages := sliceFromAny(request["messages"])
		providerPrompts = append(providerPrompts, stringFromMap(mapFromAny(messages[1]), "content"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"critic-test","choices":[{"message":{"content":%s}}]}`, strconv.Quote(providerResponse)))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	cfg := completeTurnLLMConfig{
		Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
		Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0),
	}
	outputLanguage := map[string]any{"language": "ja", "source": "session"}
	languageContext := map[string]any{
		"session_output_language":   "ja",
		"summary_language":          "ja",
		"output_language_source":    "explicit_override",
		"assistant_output_language": "ko",
		"locked_for_turn":           true,
	}
	policy := completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4_000, ConfiguredChars: 4_000, Source: "test"}
	firstResult, firstTrace, err := srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-replay", 12,
		"현재 사용자 전체 문장", "현재 어시스턴트 전체 문장",
		[]map[string]any{{"role": "user", "content": "HOST_HISTORY_IGNORED"}},
		&outputLanguage, cfg, true, policy,
		completeTurnCriticInputReplay{SourceRevision: "source-replay"},
		languageContext,
	)
	if err != nil {
		t.Fatal(err)
	}
	resultLanguageContext := mapFromAny(firstResult["language_context"])
	resultWriteContract := mapFromAny(firstResult["memory_write_contract"])
	if stringFromMap(resultLanguageContext, "session_output_language") != "ko" ||
		stringFromMap(resultLanguageContext, "summary_language") != "ko" ||
		stringFromMap(resultWriteContract, "summary_language") != "ko" {
		t.Fatalf("critic result did not retain assistant-output-only memory language: result=%#v", firstResult)
	}
	saved, ok := fake.savedCriticInputSnapshots["source-replay"]
	if !ok || saved.JSON == "" || saved.Hash == "" {
		t.Fatalf("critic input snapshot was not persisted: %#v", fake.savedCriticInputSnapshots)
	}
	var snapshot completeTurnCriticInputSnapshot
	if err := json.Unmarshal([]byte(saved.JSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.UserInput != "현재 사용자 전체 문장" ||
		snapshot.AssistantContent != "현재 어시스턴트 전체 문장" ||
		stringFromMap(snapshot.LanguageContext, "session_output_language") != "ko" ||
		stringFromMap(snapshot.LanguageContext, "summary_language") != "ko" ||
		stringFromMap(snapshot.LanguageContext, "output_language_source") != "current_assistant" {
		t.Fatalf("snapshot lost current turn or language input: %#v", snapshot)
	}
	if strings.Contains(saved.JSON, "output_language_override") {
		t.Fatalf("critic snapshot retained an output-language override: %s", saved.JSON)
	}
	firstSnapshotTrace := mapFromAny(firstTrace["input_snapshot"])
	if stringFromMap(firstSnapshotTrace, "status") != "persisted" {
		t.Fatalf("first snapshot trace=%#v", firstSnapshotTrace)
	}
	legacySnapshot := map[string]any{}
	if err := json.Unmarshal([]byte(saved.JSON), &legacySnapshot); err != nil {
		t.Fatal(err)
	}
	legacySnapshot["output_language_override"] = map[string]any{"language": "ja", "source": "session"}
	legacyLanguageContext := mapFromAny(legacySnapshot["language_context"])
	legacyLanguageContext["session_output_language"] = "ja"
	legacyLanguageContext["summary_language"] = "ja"
	legacyLanguageContext["output_language_source"] = "explicit_override"
	legacySnapshot["language_context"] = legacyLanguageContext
	legacySnapshotJSON, err := json.Marshal(legacySnapshot)
	if err != nil {
		t.Fatal(err)
	}
	legacySnapshotHash := criticSystemPromptHash(string(legacySnapshotJSON))

	fake.returnChatLogs = []store.ChatLog{
		{ChatSessionID: "critic-replay", TurnIndex: 11, Role: "user", Content: "mutated previous user"},
		{ChatSessionID: "critic-replay", TurnIndex: 11, Role: "assistant", Content: "mutated previous assistant"},
	}
	mutatedLanguage := map[string]any{"session_output_language": "en", "summary_language": "en", "assistant_output_language": "en"}
	_, replayTrace, err := srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-replay", 12,
		"현재 사용자 전체 문장", "현재 어시스턴트 전체 문장",
		[]map[string]any{{"role": "user", "content": "MUTATED_HOST_HISTORY"}},
		nil, cfg, true,
		completeTurnCriticInputPolicy{AuxiliaryMaxChars: 1, ConfiguredChars: 1, Source: "mutated"},
		completeTurnCriticInputReplay{
			SourceRevision: "source-replay", SnapshotJSON: string(legacySnapshotJSON), SnapshotHash: legacySnapshotHash, Required: true,
		},
		mutatedLanguage,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(providerPrompts) != 2 || providerPrompts[0] != providerPrompts[1] {
		t.Fatalf("reprocessing prompt changed\nfirst=%q\nreplay=%q", providerPrompts[0], providerPrompts[1])
	}
	if strings.Contains(providerPrompts[1], "mutated previous") ||
		strings.Contains(providerPrompts[1], "MUTATED_HOST_HISTORY") ||
		strings.Contains(providerPrompts[1], `"summary_language":"en"`) ||
		strings.Contains(providerPrompts[1], "Output_Language_Override_JSON") ||
		!strings.Contains(providerPrompts[1], `"summary_language":"ko"`) {
		t.Fatalf("mutable session state leaked into replay prompt: %q", providerPrompts[1])
	}
	replayedSnapshotTrace := mapFromAny(replayTrace["input_snapshot"])
	if stringFromMap(replayedSnapshotTrace, "status") != "replayed" ||
		stringFromMap(replayedSnapshotTrace, "snapshot_hash") != legacySnapshotHash {
		t.Fatalf("replay snapshot trace=%#v", replayedSnapshotTrace)
	}

	legacySnapshot["archive_ledger"] = map[string]any{
		"language": map[string]any{
			"assistant_final_language": "ja",
			"source":                   "legacy_override",
			"override_applied":         true,
		},
	}
	staleLedgerSnapshotJSON, err := json.Marshal(legacySnapshot)
	if err != nil {
		t.Fatal(err)
	}
	staleLedgerSnapshotHash := criticSystemPromptHash(string(staleLedgerSnapshotJSON))
	_, _, err = srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-replay", 12,
		"현재 사용자 전체 문장", "현재 어시스턴트 전체 문장",
		nil, nil, cfg, true, policy,
		completeTurnCriticInputReplay{
			SourceRevision: "source-replay", SnapshotJSON: string(staleLedgerSnapshotJSON), SnapshotHash: staleLedgerSnapshotHash, Required: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(providerPrompts) != 3 ||
		strings.Contains(providerPrompts[2], `"assistant_final_language":"ja"`) ||
		!strings.Contains(providerPrompts[2], `"assistant_final_language":"ko"`) ||
		!strings.Contains(providerPrompts[2], `"source":"request_assistant_final_language"`) {
		t.Fatalf("legacy archive-ledger language was not canonicalized from assistant output: %q", providerPrompts)
	}

	snapshot.SystemPromptSHA256 = strings.Repeat("0", 64)
	promptChangedJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	_, changedPromptTrace, err := srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-replay", 12,
		"current user full text", "current assistant full text",
		nil, nil, cfg, true, policy,
		completeTurnCriticInputReplay{
			SourceRevision: "source-replay",
			SnapshotJSON:   string(promptChangedJSON),
			SnapshotHash:   criticSystemPromptHash(string(promptChangedJSON)),
			Required:       true,
		},
	)
	if err == nil || stringFromMap(criticPipelineErrorDetails(err), "code") != "CRITIC_INPUT_SNAPSHOT_INVALID" ||
		len(providerPrompts) != 3 || stringFromMap(mapFromAny(changedPromptTrace["input_snapshot"]), "status") != "invalid" {
		t.Fatalf("changed prompt contract was not rejected before provider call: err=%v trace=%#v calls=%d", err, changedPromptTrace, len(providerPrompts))
	}
}

func TestCriticSnapshotPersistenceFailureDoesNotBlockForegroundCritic(t *testing.T) {
	fake := &turnRecordingStore{criticInputSnapshotErr: errors.New("snapshot store unavailable")}
	srv := NewServer(config.Default())
	srv.Store = fake

	oldClient := proxyHTTPClient
	providerCalls := 0
	providerResponse := criticWireJSONForTest(map[string]any{
		"turn_summary":      "Mina opened the vault.",
		"importance_score":  7,
		"evidence_excerpts": []any{},
	})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		providerCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"critic-test","choices":[{"message":{"content":%s}}]}`, strconv.Quote(providerResponse)))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, trace, err := srv.runCompleteTurnCriticWithInputPolicy(
		context.Background(), "critic-snapshot-write-failure", 3,
		"Mina reaches the vault.", "Mina opens the vault.",
		nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0),
		},
		true,
		completeTurnCriticInputPolicy{AuxiliaryMaxChars: 2_000, ConfiguredChars: 2_000, Source: "test"},
		completeTurnCriticInputReplay{SourceRevision: "source-write-failure"},
	)
	if err != nil || providerCalls != 1 {
		t.Fatalf("foreground critic was blocked by snapshot persistence: err=%v calls=%d", err, providerCalls)
	}
	snapshotTrace := mapFromAny(trace["input_snapshot"])
	if stringFromMap(snapshotTrace, "status") != "persist_failed" ||
		stringFromMap(snapshotTrace, "reason") != "critic_input_snapshot_persist_failed" {
		t.Fatalf("snapshot failure trace=%#v", snapshotTrace)
	}
}

func TestCriticPipelineErrorClassificationPreservesStageAndHTTPStatus(t *testing.T) {
	timeoutErr := classifyCriticProviderError(context.DeadlineExceeded, http.StatusBadGateway)
	timeoutDetails := criticPipelineErrorDetails(timeoutErr)
	if timeoutDetails["code"] != "CRITIC_PROVIDER_TIMEOUT" ||
		timeoutDetails["stage"] != "provider_call" ||
		timeoutDetails["retryable"] != true ||
		timeoutDetails["http_status"] != http.StatusBadGateway {
		t.Fatalf("timeout details = %#v", timeoutDetails)
	}

	httpErr := classifyCriticProviderError(errors.New("unauthorized"), http.StatusUnauthorized)
	httpDetails := criticPipelineErrorDetails(httpErr)
	if httpDetails["code"] != "CRITIC_PROVIDER_HTTP_ERROR" ||
		httpDetails["stage"] != "provider_response" ||
		httpDetails["retryable"] != false ||
		httpDetails["http_status"] != http.StatusUnauthorized {
		t.Fatalf("HTTP details = %#v", httpDetails)
	}

	emptyDetails := criticPipelineErrorDetails(classifyCriticProviderError(
		&proxyEmptyContentError{Provider: "claude"}, http.StatusNoContent,
	))
	if emptyDetails["code"] != "CRITIC_EMPTY_RESPONSE" ||
		emptyDetails["stage"] != "provider_response" ||
		emptyDetails["retryable"] != true ||
		emptyDetails["http_status"] != http.StatusNoContent {
		t.Fatalf("empty response details = %#v", emptyDetails)
	}

	localDetails := criticPipelineErrorDetails(classifyCriticProviderError(
		&proxyLocalRequestError{Stage: "request_build", Cause: errors.New("conflict")},
		http.StatusBadRequest,
	))
	if localDetails["code"] != "CRITIC_REQUEST_BUILD_FAILED" ||
		localDetails["stage"] != "request_build" ||
		localDetails["retryable"] != false {
		t.Fatalf("local request details = %#v", localDetails)
	}
	if _, hasHTTPStatus := localDetails["http_status"]; hasHTTPStatus {
		t.Fatalf("local request error must not claim upstream HTTP status: %#v", localDetails)
	}

	trace := criticFailureTrace("test", completeTurnLLMConfig{
		Provider: "openai",
		Model:    "critic-test",
		APIKey:   "secret-key",
	}, http.StatusUnauthorized, httpErr, "provider rejected secret-key")
	if trace["http_status"] != http.StatusUnauthorized ||
		!strings.Contains(stringFromMap(trace, "raw_preview"), "[redacted]") ||
		strings.Contains(stringFromMap(trace, "raw_preview"), "secret-key") {
		t.Fatalf("failure trace = %#v", trace)
	}
}

func TestSanitizeContextMessagesUsesHostProvenanceInsteadOfProseKeywords(t *testing.T) {
	messages := []map[string]any{
		{
			"contract_version":  risuChatMessageObservationContract,
			"observation_state": "observed",
			"source_kind":       "active_chat_message",
			"role":              "user",
			"content":           "The character opens a book titled Persona and reads the rules aloud.",
		},
		{
			"contract_version":  risuChatMessageObservationContract,
			"observation_state": "observed",
			"source_kind":       "prompt_template",
			"role":              "user",
			"content":           "This is not an active chat message.",
		},
		{"role": "system", "content": "hidden prompt"},
	}

	got := sanitizeContextMessagesForCriticInput(messages)
	if len(got) != 1 || stringFromMap(got[0], "content") != messages[0]["content"] {
		t.Fatalf("structured context provenance mismatch: %#v", got)
	}
}

func TestCriticCurrentTurnInputIsNeverBounded(t *testing.T) {
	input := "current-turn-start " + strings.Repeat("complete-current-turn-", 1000) + " current-turn-end"
	if got := boundCompleteTurnCriticInput(input, 0); got != input {
		t.Fatalf("current turn changed: got=%d chars want=%d", len([]rune(got)), len([]rune(input)))
	}
}

/*
Obsolete fixed entity-reference admission contract retained only in history.

	func TestCriticEntityReferenceContractReplacesDescriptorWordList(t *testing.T) {
		turnText := "경비가 문을 열었고, Mira가 안으로 들어왔다."
		turnLocal := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "turn_local_descriptor",
			"name":               "경비",
			"name_expression":    "경비",
			"evidence_excerpt":   "경비가 문을 열었고",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(turnLocal, "경비", turnText); ok || reason != "turn_local_descriptor" {
			t.Fatalf("turn-local descriptor should not become canonical: ok=%v reason=%s", ok, reason)
		}

		stable := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "session_stable",
			"name":               "Mira",
			"name_expression":    "Mira",
			"evidence_excerpt":   "Mira가 안으로 들어왔다.",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(stable, "Mira", turnText); !ok || reason != "session_stable" {
			t.Fatalf("grounded stable entity was rejected: ok=%v reason=%s", ok, reason)
		}

		stableUnknownLanguage := map[string]any{
			"reference_contract": criticEntityReferenceContract,
			"reference_scope":    "session_stable",
			"name_expression":    "守門人甲",
			"evidence_excerpt":   "守門人甲留下了自己的名字。",
		}
		if ok, reason := criticEntityCanonicalWriteEligible(stableUnknownLanguage, "守門人甲", "鐘が鳴った。守門人甲留下了自己的名字。門が閉じた。"); !ok {
			t.Fatalf("typed stable entity should be language-neutral: reason=%s", reason)
		}
	}
*/
func TestCriticFailureTraceRedactsCredentialValuesNotEqualToConfiguredKey(t *testing.T) {
	raw := `Authorization: Bearer different-token-123 ` +
		`{"password":"hunter2","access_token":"other-access","api_key":"configured-key"}`
	trace := criticFailureTrace("test", completeTurnLLMConfig{
		Provider: "openai",
		Model:    "critic-test",
		APIKey:   "configured-key",
	}, http.StatusUnauthorized, errors.New(raw), raw)
	preview := stringFromMap(trace, "raw_preview")
	for _, secret := range []string{"different-token-123", "hunter2", "other-access", "configured-key"} {
		if strings.Contains(preview, secret) {
			t.Fatalf("credential %q leaked in preview: %s", secret, preview)
		}
	}
	if strings.Count(preview, "[redacted]") < 4 {
		t.Fatalf("expected credential values to be redacted: %s", preview)
	}
}

func TestCriticProviderFailureDoesNotHideASecondProviderCall(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	prompts := []string{}
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		messages := sliceFromAny(request["messages"])
		if len(messages) >= 2 {
			prompts = append(prompts, stringFromMap(mapFromAny(messages[1]), "content"))
		}
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"message":"provider failure marker"}}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(),
		"session",
		1,
		"Mina asks Rowan to be gentle.",
		"The intimate scene involved penetration.",
		nil,
		nil,
		completeTurnLLMConfig{
			Provider:    "openai",
			Endpoint:    "https://example.invalid/v1",
			APIKey:      "test-key",
			Model:       "critic-test",
			TimeoutMs:   30_000,
			RetryBudget: newLLMRetryBudget(1),
		},
	)
	if err == nil || callCount != 1 {
		t.Fatalf("error=%v calls=%d trace=%+v", err, callCount, trace)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "penetration") {
		t.Fatalf("critic request prompt_count=%d prompts=%#v", len(prompts), prompts)
	}
	if !strings.Contains(stringFromMap(trace, "raw_preview"), "provider failure marker") {
		t.Fatalf("provider failure preview was lost: %+v", trace)
	}
	if _, exists := trace["provider_retry"]; exists {
		t.Fatalf("hidden retry trace should not exist: %+v", trace)
	}
	if _, err := json.Marshal(trace); err != nil {
		t.Fatalf("retry failure trace is cyclic or unserializable: %v; trace=%+v", err, trace)
	}
}

func TestCriticProviderFailureRespectsZeroRetryBudget(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1,
		"Mina asks Rowan to be gentle.",
		"The intimate scene involved penetration.",
		nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(0),
		},
	)
	if err == nil || callCount != 1 {
		t.Fatalf("error=%v calls=%d trace=%+v", err, callCount, trace)
	}
}

func TestCriticCompleteJSONIsKeptEvenWhenProviderReportsTokenLimit(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	providerResponse := criticWireJSONForTest(map[string]any{
		"turn_summary":      "Mina kept the key.",
		"importance_score":  6,
		"evidence_excerpts": []any{"Mina found the key.", "Mina kept the key safe."},
		"kg_triples": []any{map[string]any{
			"subject": "Mina", "predicate": "kept", "object": "key",
		}},
	})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				fmt.Sprintf(`{"model":"critic-test","choices":[{"finish_reason":"length","message":{"content":%s}}],"usage":{"prompt_tokens":50,"completion_tokens":20,"total_tokens":70}}`, strconv.Quote(providerResponse)),
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	result, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1,
		"Mina found the key.", "Mina kept the key safe.", nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(2),
		},
	)
	if err != nil || callCount != 1 || stringFromMap(result, "turn_summary") != "Mina kept the key." {
		t.Fatalf("complete JSON was discarded: err=%v calls=%d result=%#v trace=%#v", err, callCount, result, trace)
	}
	metadata := mapFromAny(trace["provider_response"])
	if metadata["termination_kind"] != "length" || intFromAny(metadata["output_tokens"], 0) != 20 {
		t.Fatalf("provider response metadata=%#v", metadata)
	}
	observation := mapFromAny(trace["output_observation"])
	if observation["contract_version"] != "critic_output_observation.v1" ||
		intFromAny(observation["response_chars"], 0) != len([]rune(providerResponse)) ||
		intFromAny(observation["wire_field_count"], 0) != 4 ||
		intFromAny(observation["wire_item_count"], 0) != 3 ||
		intFromAny(observation["quarantined_item_count"], -1) != 0 {
		t.Fatalf("critic output observation=%#v", observation)
	}
}

func TestCriticMissingOptionalSurfacesKeepsIndependentSummaryWithoutSecondCall(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	providerResponse := `{"turn_summary":"Mina kept the key.","importance_score":6}`
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				fmt.Sprintf(`{"model":"critic-test","choices":[{"finish_reason":"stop","message":{"content":%s}}]}`, strconv.Quote(providerResponse)),
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	result, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1,
		"Mina found the key.", "Mina kept the key safe.", nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(2),
		},
	)
	if err != nil || callCount != 1 || stringFromMap(result, "turn_summary") != "Mina kept the key." || intFromAny(result["importance_score"], 0) != 6 {
		t.Fatalf("missing optional surfaces discarded the independent result or retried: err=%v calls=%d result=%#v trace=%#v", err, callCount, result, trace)
	}
	observation := mapFromAny(trace["output_observation"])
	if len(mapFromAny(trace["schema_quarantine"])) != 0 ||
		intFromAny(observation["wire_field_count"], 0) != 2 ||
		intFromAny(observation["quarantined_field_count"], -1) != 0 {
		t.Fatalf("missing-optional-surfaces observation=%#v quarantine=%#v", observation, trace["schema_quarantine"])
	}
}

func TestCriticEmptySafetyResponsePreservesProviderTerminationMetadata(t *testing.T) {
	oldClient := proxyHTTPClient
	callCount := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"model":"critic-test","choices":[{"finish_reason":"content_filter","message":{"content":""}}],"usage":{"prompt_tokens":50,"completion_tokens":0,"total_tokens":50}}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1, "Mina found the key.", "Mina kept the key safe.", nil, nil,
		completeTurnLLMConfig{
			Provider: "openai", Endpoint: "https://example.invalid/v1", APIKey: "test-key",
			Model: "critic-test", TimeoutMs: 30_000, RetryBudget: newLLMRetryBudget(2),
		},
	)
	if err == nil || callCount != 1 || stringFromMap(criticPipelineErrorDetails(err), "code") != "CRITIC_EMPTY_RESPONSE" {
		t.Fatalf("error=%v calls=%d trace=%#v", err, callCount, trace)
	}
	metadata := mapFromAny(trace["provider_response"])
	if metadata["termination_kind"] != "safety" || metadata["native_finish_reason"] != "content_filter" {
		t.Fatalf("provider safety metadata=%#v", metadata)
	}
}

func TestCriticExtractionSchemaRejectsParsedButInvalidPayload(t *testing.T) {
	for name, payload := range map[string]map[string]any{
		"empty":        {},
		"invalid_only": {"turn_summary": []any{"not", "text"}},
		"unknown_only": {"unrecognized": map[string]any{"value": "x"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := validateCriticExtractionSchema(payload); err == nil {
				t.Fatalf("payload should be rejected: %#v", payload)
			}
		})
	}
	if _, _, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary":      "Mina found the key.",
		"importance_score":  float64(7),
		"evidence_excerpts": []any{"Mina found the key."},
	}); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	for name, payload := range map[string]map[string]any{
		"missing_optional_surfaces":  {"turn_summary": "summary survives omitted optional surfaces"},
		"malformed_optional_surface": {"turn_summary": "summary survives malformed optional surface", "kg_triples": map[string]any{}},
	} {
		t.Run(name+"_keeps_independent_summary", func(t *testing.T) {
			sanitized, trace, err := validateCriticExtractionSchema(payload)
			if err != nil || strings.TrimSpace(stringFromMap(sanitized, "turn_summary")) == "" {
				t.Fatalf("one omitted or malformed optional surface discarded the independent summary: err=%v sanitized=%#v trace=%#v", err, sanitized, trace)
			}
			wantDropped := 0
			if name == "malformed_optional_surface" {
				wantDropped = 1
			}
			if intFromAny(trace["dropped_field_count"], 0) != wantDropped {
				t.Fatalf("optional-surface quarantine=%#v, want dropped=%d", trace, wantDropped)
			}
		})
	}
}

func TestCriticExtractionSchemaQuarantinesOnlyInvalidFieldsAndItems(t *testing.T) {
	sanitized, trace, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary":      "Mina found the key.",
		"importance_score":  "not-a-number",
		"evidence_excerpts": []any{"Mina found the key.", map[string]any{"quote": "invalid"}},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "found", "object": "key"}},
	})
	if err != nil {
		t.Fatalf("one malformed field or item discarded the valid extraction: %v", err)
	}
	if sanitized["turn_summary"] != "Mina found the key." || len(sliceFromAny(sanitized["kg_triples"])) != 1 {
		t.Fatalf("valid fields were lost: %#v", sanitized)
	}
	if _, exists := sanitized["importance_score"]; exists {
		t.Fatalf("invalid scalar field was retained: %#v", sanitized)
	}
	if excerpts := sliceFromAny(sanitized["evidence_excerpts"]); len(excerpts) != 1 || excerpts[0] != "Mina found the key." {
		t.Fatalf("invalid evidence item was not isolated: %#v", excerpts)
	}
	if intFromAny(trace["dropped_field_count"], 0) != 1 || intFromAny(trace["dropped_item_count"], 0) != 1 {
		t.Fatalf("quarantine trace=%#v", trace)
	}
	dropped := sliceFromAny(trace["dropped_items"])
	detail := mapFromAny(dropped[0])
	if intFromAny(detail["item_index"], -1) != 1 || stringFromMap(detail, "field") != "evidence_excerpts" {
		t.Fatalf("quarantine did not identify only the malformed sparse item: %#v", detail)
	}
}

func TestCriticOptionalStateErrorsDoNotDiscardIndependentKG(t *testing.T) {
	payload := map[string]any{
		"turn_summary":      "Mina entered the archive room.",
		"importance_score":  float64(6),
		"evidence_excerpts": []any{"Mina entered the archive room."},
		"story_clock":       "invalid optional state",
		"kg_triples": []any{map[string]any{
			"semantic_class": "location_fact", "subject": "Mina", "predicate": "entered_location",
			"predicate_expression": "entered", "object": "archive room",
			"subject_binding": map[string]any{
				"contract_version": "kg_endpoint_binding.v1", "endpoint_kind": "entity",
				"entity_kind": "character", "expression": "Mina",
			},
			"object_binding": map[string]any{
				"contract_version": "kg_endpoint_binding.v1", "endpoint_kind": "scalar",
				"scalar_type": "string", "expression": "archive room",
			},
			"evidence_excerpt": "Mina entered the archive room.",
		}},
		"reversible_states": []any{map[string]any{
			"version": reversibleStateContractVersion, "domain": "appearance", "transition": "set",
			"subject_name": "Mina", "state_slot": "appearance", "value": map[string]any{"text": "dusty"},
			"evidence_excerpt": "Mina entered the archive room.", "scene_scope": "current",
			"authority": "canonical_in_fiction", "assertion_kind": "literal", "polarity": "affirmative",
			"visibility": "public", "sensitivity": "ordinary",
		}},
	}
	sanitized, trace, err := validateCriticExtractionSchema(payload)
	if err != nil {
		t.Fatalf("optional state error rejected the full extraction: %v", err)
	}
	if intFromAny(trace["dropped_field_count"], 0) != 1 {
		t.Fatalf("invalid optional state was not isolated: %#v", trace)
	}
	extraction := normalizeCriticExtraction(sanitized)
	if len(sliceFromAny(extraction["kg_triples"])) != 1 {
		t.Fatalf("runtime extraction lost valid KG: %#v", extraction)
	}
	if _, exists := extraction["story_clock"]; exists || len(sliceFromAny(extraction["reversible_states"])) != 1 {
		t.Fatalf("open collection discarded the reversible observation or kept an empty story clock: %#v", extraction)
	}
}

func TestCriticProtectedCandidateCollectionKeepsStructurallyCompleteItems(t *testing.T) {
	source := "Masked Mina told Rowan that she was Mina. Mina told Rowan that she hid the brass key."
	payload := map[string]any{
		"turn_summary": "Mina kept a secret.",
		"protected_secrets": []any{
			map[string]any{
				"owner":             "Mina",
				"summary":           "Mina hid the brass key.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "she hid the brass key",
			},
			map[string]any{
				"owner":             "Mina",
				"summary":           "Invented secret.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "text absent from source",
			},
		},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name":    "Mina",
				"owner_entity_role":    "npc",
				"owner_visibility":     "owner_private",
				"memory_text":          "Mina hid the brass key and remembers it.",
				"target_reveal_policy": "owner_private_until_revealed",
				"evidence_excerpt":     "she hid the brass key",
			},
			map[string]any{
				"owner_entity_name": "Mina",
				"owner_entity_role": "npc",
				"memory_text":       "Mina remembers that she hid the brass key.",
				"evidence_excerpt":  "she hid the brass key",
			},
		},
		"character_identity_accuracy": []any{
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Masked Mina",
				"true_identity_name":    "Mina",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "Masked Mina told Rowan that she was Mina",
			},
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Unknown",
				"true_identity_name":    "Other",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "text absent from source",
			},
			map[string]any{
				"same_entity":           true,
				"surface_identity_name": "Masked Mina",
				"true_identity_name":    "Mina",
				"reveal_policy":         "owner_private_until_revealed",
				"evidence_excerpt":      "Masked Mina told Rowan",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 2 {
		t.Fatalf("protected secrets kept = %d, want 2: %#v", got, filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 2 {
		t.Fatalf("subjective memories kept = %d, want 2: %#v", got, filtered["subjective_entity_memories"])
	}
	defaulted := mapFromAny(sliceFromAny(filtered["subjective_entity_memories"])[1])
	if stringFromMap(defaulted, "target_reveal_policy") != "owner_private_until_revealed" {
		t.Fatalf("default-private policy was not normalized before quarantine: %#v", defaulted)
	}
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 3 {
		t.Fatalf("identity mappings kept = %d, want 3: %#v", got, filtered["character_identity_accuracy"])
	}
	if intFromAny(trace["candidate_count"], 0) != 7 ||
		intFromAny(trace["kept_count"], 0) != 7 ||
		intFromAny(trace["quarantined_count"], 0) != 0 {
		t.Fatalf("quarantine trace = %#v", trace)
	}
}

func TestCriticSubjectiveQuarantineDefaultsPrivatePolicyButKeepsExplicitPublicNPC(t *testing.T) {
	payload := map[string]any{
		"turn_summary": "Mina spoke to Rowan.",
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name": "Mina",
				"memory_text":       "Mina remembers that Mina spoke to Rowan.",
				"evidence_excerpt":  "Mina spoke to Rowan",
			},
			map[string]any{
				"owner_entity_name": "Rowan",
				"owner_entity_role": "npc",
				"owner_visibility":  "player_known",
				"memory_text":       "Rowan openly remembers the meeting.",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", "Mina spoke to Rowan.")
	items := sliceFromAny(filtered["subjective_entity_memories"])
	if len(items) != 2 {
		t.Fatalf("kept subjective memories = %d, want default-private and explicit-public items: %#v", len(items), items)
	}
	private := mapFromAny(items[0])
	if stringFromMap(private, "owner_visibility") != "owner_private" ||
		stringFromMap(private, "target_reveal_policy") != "owner_private_until_revealed" ||
		stringFromMap(private, "portability") != "npc_private_recollection" {
		t.Fatalf("default-private memory was not conservatively normalized: %#v", private)
	}
	public := mapFromAny(items[1])
	if stringFromMap(public, "owner_entity_name") != "Rowan" {
		t.Fatalf("unexpected public memory kept: %#v", public)
	}
	if intFromAny(trace["quarantined_count"], 0) != 0 {
		t.Fatalf("quarantine trace = %#v", trace)
	}

	normalized := normalizeSubjectiveEntityMemories(items)
	if len(normalized) != 2 {
		t.Fatalf("normalized public memories = %#v", normalized)
	}
	got := mapFromAny(normalized[1])
	if stringFromMap(got, "owner_visibility") != "player_known" ||
		stringFromMap(got, "target_reveal_policy") != "" ||
		stringFromMap(got, "portability") != "portable_subjective_entity_recollection" {
		t.Fatalf("explicit public NPC was promoted to private: %#v", got)
	}
}

func TestCriticSubjectiveCollectionPreservesStorySpecificRevealPolicy(t *testing.T) {
	payload := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina remembers opening the door.",
			"evidence_excerpt":     "Mina opened the door",
			"target_reveal_policy": "reveal_whenever_convenient",
		}},
	}
	filtered, _ := quarantineCriticProtectedCandidates(payload, "", "Mina opened the door.")
	items := sliceFromAny(filtered["subjective_entity_memories"])
	if len(items) != 1 || stringFromMap(mapFromAny(items[0]), "target_reveal_policy") != "reveal_whenever_convenient" {
		t.Fatalf("story-specific reveal policy was deleted during collection: %#v", filtered)
	}
}

func TestCriticPromptRequiresEvidenceEligibleSubjectiveCoverageAndAllowsValidZero(t *testing.T) {
	prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt("session", 3, "Mina opens the door.", "Rowan watches.", nil, nil))
	for _, required := range []string{
		"Before omitting this surface, inspect every named in-story entity",
		"Omitting `subjective_entity_memories` remains valid",
		"Each subjective memory needs an owner and memory text",
		"extract useful source-grounded in-story facts and relationships broadly",
		"A fact is not omitted merely because another typed lane also records it",
		"evidence_excerpts are durable citations, not transcript samples",
		"Speech-style examples belong in voice_observations",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic prompt missing subjective-memory contract %q", required)
		}
	}
	if strings.Contains(prompt, "Generic entity-to-entity KG edges are review-only") ||
		strings.Contains(prompt, "emit only source-bound entity-to-scalar") ||
		strings.Contains(prompt, "Every kg_triples item requires semantic_class") ||
		strings.Contains(prompt, "subject_binding/object_binding") ||
		strings.Contains(prompt, "Represent each record with subject, predicate, object") {
		t.Fatalf("critic prompt still contains the KG suppression policy")
	}
	for _, legacyWirePhrase := range []string{"Use `[]` only", "Empty arrays are valid", "Before returning this array empty", "top-level direct-evidence rows"} {
		if strings.Contains(prompt, legacyWirePhrase) {
			t.Fatalf("critic prompt still contains legacy wire guidance %q", legacyWirePhrase)
		}
	}
}

func TestCriticPromptJSONExamplesRemainParseableAfterDeduplication(t *testing.T) {
	systemPrompt, source := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if source == "fallback_builtin" {
		t.Fatal("source critic_system.txt was not loaded")
	}
	if chars := len([]rune(systemPrompt)); chars >= 16_000 {
		t.Fatalf("system critic prompt exceeded the stage-9 compact contract: chars=%d", chars)
	}
	if strings.Contains(systemPrompt, "Deterministic_Preview_Pass_JSON") {
		t.Fatal("system critic prompt still instructs the removed duplicate preview payload")
	}
	systemJSONSection := strings.Index(systemPrompt, "[Wire Output Contract]")
	if systemJSONSection < 0 {
		t.Fatal("system critic prompt is missing the JSON surface section")
	}
	userPrompt := buildCompleteTurnCriticPrompt(
		"session-json-contract", 7,
		"Mina found the brass key.",
		"Rowan nodded and followed.",
		nil, nil,
	)
	example, err := parseJSONFromLLMContent(systemPrompt[systemJSONSection:])
	if err != nil {
		t.Fatalf("system critic prompt JSON example is not parseable: %v", err)
	}
	canonical, _, err := validateCriticExtractionSchema(example)
	if err != nil {
		t.Fatalf("system critic prompt JSON example violates the critic schema: %v", err)
	}
	evidenceExcerpts := sliceFromAny(canonical["evidence_excerpts"])
	if len(evidenceExcerpts) != 1 || stringFromAny(evidenceExcerpts[0]) != "exact source excerpt" {
		t.Fatalf("system critic prompt does not demonstrate string-only evidence excerpts: %#v", evidenceExcerpts)
	}
	if _, hasRecords := example["records"]; hasRecords || len(sliceFromAny(example["kg_triples"])) != 1 || len(mapFromAny(example["entities"])) == 0 {
		t.Fatalf("system critic prompt does not demonstrate the sparse top-level wire contract: %#v", example)
	}
	for _, surface := range []string{
		"evidence_excerpts", "kg_triples", "entities", "world_rule_audit", "world_rules",
		"subjective_entity_memories", "protected_secrets", "character_identity_accuracy",
		"persona_capsule_candidates", "narrative_events", "state_claims", "belief_updates",
		"state_deltas", "character_deltas", "physical_conditions", "entity_conditions",
		"reversible_states", "pending_threads",
	} {
		if !strings.Contains(systemPrompt, surface) {
			t.Fatalf("system critic prompt lost supported surface %q", surface)
		}
	}
	for _, duplicate := range []string{"[Available JSON Surfaces]", "Use this JSON shape", "Sensitivity policy:", `"turn_summary":""`} {
		if strings.Contains(userPrompt, duplicate) {
			t.Fatalf("dynamic critic user prompt still duplicates static contract %q", duplicate)
		}
	}
	for _, dynamic := range []string{"<Latest_Turn>", "Mina found the brass key.", "<Recent_Context_JSON>", "<Critic_Archive_Ledger_JSON>", "<Language_Context_JSON>"} {
		if !strings.Contains(userPrompt, dynamic) {
			t.Fatalf("dynamic critic user prompt lost %q", dynamic)
		}
	}
	if strings.Contains(userPrompt, "Output_Language_Override_JSON") {
		t.Fatal("dynamic critic user prompt retained output-language override input")
	}
	if chars := len([]rune(userPrompt)); chars >= 2000 {
		t.Fatalf("dynamic critic user prompt regained a static contract: chars=%d", chars)
	}
}

func TestCriticCharacterDeltaNameContractPersistsState(t *testing.T) {
	systemPrompt, _ := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	systemJSONSection := strings.Index(systemPrompt, "[Wire Output Contract]")
	if systemJSONSection < 0 {
		t.Fatal("system critic prompt is missing the JSON surface section")
	}
	example, err := parseJSONFromLLMContent(systemPrompt[systemJSONSection:])
	if err != nil {
		t.Fatalf("system critic prompt JSON example is not parseable: %v", err)
	}
	_, _, err = validateCriticExtractionSchema(example)
	if err != nil {
		t.Fatalf("system critic prompt JSON example violates the critic schema: %v", err)
	}
	delta := map[string]any{"name": ""}
	delta["name"] = "Mina"
	delta["status"] = map[string]any{"emotion": "relieved"}

	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	result := srv.saveCriticExtractionArtifacts(
		context.Background(),
		"critic-character-name-contract",
		1,
		normalizeCriticExtraction(map[string]any{"character_deltas": []any{delta}}),
		"Mina looked relieved.",
		completeTurnEmbeddingConfig{},
		time.Unix(100, 0),
	)
	if result.CharacterStates != 1 || len(fake.savedCharacterStates) != 1 {
		t.Fatalf("named character delta was not persisted: result=%#v states=%#v", result, fake.savedCharacterStates)
	}
	if fake.savedCharacterStates[0].CharacterName != "Mina" {
		t.Fatalf("saved character name = %q, want Mina", fake.savedCharacterStates[0].CharacterName)
	}
	for _, reason := range result.SkipReasons {
		if stringFromMap(reason, "surface") == "character_deltas" && stringFromMap(reason, "reason") == "missing_name" {
			t.Fatalf("named character delta was discarded as missing_name: %#v", result.SkipReasons)
		}
	}
}

func TestCriticNormalizationCanonicalizesPendingThreadAliasesThroughPersistence(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"pending_threads": []any{map[string]any{
			"thread_type": "Open Questions",
			"title":       "Who opened the gate?",
			"confidence":  0.8,
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "pending-alias", 4, extraction, "Who opened the gate?", completeTurnEmbeddingConfig{}, time.Unix(400, 0))
	if result.PendingThreads != 1 || len(fake.savedPendingThreads) != 1 {
		t.Fatalf("canonical pending thread was not persisted: result=%#v saved=%#v", result, fake.savedPendingThreads)
	}
	if fake.savedPendingThreads[0].ThreadType != "open_question" || fake.savedPendingThreads[0].HookType != "open_question" {
		t.Fatalf("pending thread enum was not canonicalized: %#v", fake.savedPendingThreads[0])
	}
}

func TestCriticEnricherKeepsStoryClockEvidenceWithoutPromotingVoiceSamples(t *testing.T) {
	storyEvidence := "At dawn, the seventh bell rang."
	voiceEvidence := `Mina said, "Please wait here."`
	extraction := normalizeCriticExtraction(map[string]any{
		"evidence_excerpts": []any{"rewritten evidence not in source"},
		"story_clock": map[string]any{
			"version": "story_clock.v1", "observation_kind": "partial", "scene_scope": "current",
			"precision": "partial", "partial": map[string]any{"daypart": "dawn"},
			"evidence_excerpt": storyEvidence, "transition": "set",
		},
		"voice_observations": []any{
			map[string]any{"evidence_excerpt": voiceEvidence},
			map[string]any{"evidence_excerpt": "Mina politely asked everyone to wait."},
		},
	})
	enriched := enrichNormalizedCriticExtractionForFocusedRecall(extraction, "", storyEvidence+" "+voiceEvidence, 8)
	got := stringsFromAny(enriched["evidence_excerpts"])
	if len(got) != 1 || got[0] != storyEvidence {
		t.Fatalf("direct nested evidence = %#v", got)
	}
	voices := sliceFromAny(enriched["voice_observations"])
	if len(voices) != 2 || stringFromMap(mapFromAny(voices[0]), "evidence_excerpt") != voiceEvidence {
		t.Fatalf("voice observations lost their own evidence = %#v", voices)
	}
	if enriched["focused_recall_fallback"] != nil {
		t.Fatalf("exact nested evidence incorrectly triggered fallback: %#v", enriched["focused_recall_fallback"])
	}
}

func TestCriticBeliefTransferCreatesGroundedSubjectiveMemoryPerNamedListener(t *testing.T) {
	excerpt := "Mira told Rowan and Jules that Rowan is captain."
	normalized := normalizeCriticExtraction(map[string]any{
		"belief_updates": []any{map[string]any{
			"subject": "Rowan", "slot": "role", "claim": "captain",
			"speaker": "Mira", "listener_names": []any{"Rowan", "Jules"},
			"epistemic_state": "known", "acquisition_mode": "heard", "evidence": excerpt,
		}},
	})
	beliefs := sliceFromAny(normalized["belief_updates"])
	if len(beliefs) != 1 {
		t.Fatalf("normalized beliefs = %#v", beliefs)
	}
	belief := mapFromAny(beliefs[0])
	if stringFromMap(belief, "state_slot") != "role" || stringFromMap(belief, "value") != "captain" ||
		stringFromMap(belief, "speaker_name") != "Mira" || stringFromMap(belief, "evidence_excerpt") != excerpt {
		t.Fatalf("belief transfer aliases were not canonicalized: %#v", belief)
	}
	memories := sliceFromAny(normalized["subjective_entity_memories"])
	if len(memories) != 2 {
		t.Fatalf("listener subjective memories = %#v", memories)
	}
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	result := srv.saveCriticExtractionArtifacts(context.Background(), "belief-transfer", 6, normalized, "The room quieted. "+excerpt+" Then the bell rang.", completeTurnEmbeddingConfig{}, time.Unix(600, 0))
	if result.SubjectiveEntityMemories != 2 || len(fake.savedEntityMemories) != 2 {
		t.Fatalf("grounded listener memories were not persisted: result=%#v saved=%#v", result, fake.savedEntityMemories)
	}
	owners := map[string]bool{}
	for _, memory := range fake.savedEntityMemories {
		owners[memory.OwnerEntityName] = true
		if memory.EvidenceExcerpt != excerpt || memory.MemoryText != excerpt {
			t.Fatalf("listener memory was not exact-source grounded: %#v", memory)
		}
	}
	if !owners["Rowan"] || !owners["Jules"] {
		t.Fatalf("named listener coverage = %#v", owners)
	}
}

func TestCriticBeliefWithNamedKnowledgeHoldersCreatesSubjectiveMemoryWithoutSpeakerField(t *testing.T) {
	excerpt := "Rowan and Jules learned that Rowan is captain."
	normalized := normalizeCriticExtraction(map[string]any{
		"belief_updates": []any{map[string]any{
			"subject": "Rowan", "state_slot": "role", "value": "captain",
			"listener_names": []any{"Rowan", "Jules"}, "epistemic_state": "known",
			"evidence_excerpt": excerpt,
		}},
	})
	if got := len(sliceFromAny(normalized["belief_updates"])); got != 1 {
		t.Fatalf("source-grounded belief proposal was unexpectedly removed: %#v", normalized["belief_updates"])
	}
	if got := len(sliceFromAny(normalized["subjective_entity_memories"])); got != 2 {
		t.Fatalf("named knowledge holders did not receive subjective recollections: %#v", normalized["subjective_entity_memories"])
	}
}

func TestCriticObjectiveOnlyTurnDoesNotFabricateSubjectiveMemory(t *testing.T) {
	normalized := normalizeCriticExtraction(map[string]any{
		"state_claims": []any{map[string]any{
			"subject": "gate", "state_slot": "access", "value": "open",
			"evidence_excerpt": "The gate is open.",
		}},
	})
	if got := len(sliceFromAny(normalized["subjective_entity_memories"])); got != 0 {
		t.Fatalf("objective-only turn fabricated %d subjective memories: %#v", got, normalized["subjective_entity_memories"])
	}
}

func TestCriticWorldRulePersistenceOmitsExactUnchangedRepeat(t *testing.T) {
	fake := &turnRecordingStore{returnWorldRules: []store.WorldRule{{
		ChatSessionID: "world-repeat", Scope: "system", Category: "access",
		Key: "gate_requires_seal", ValueJSON: `"Gate access requires a seal."`, SourceTurn: 2,
	}}}
	srv := NewServer(config.Default())
	srv.Store = fake
	extraction := normalizeCriticExtraction(map[string]any{
		"world_rules": []any{map[string]any{
			"scope": "system", "category": "access", "key": "gate_requires_seal", "value": "Gate access requires a seal.",
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "world-repeat", 3, extraction, "The party approaches the gate.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.WorldRules != 0 || len(fake.savedWorldRules) != 0 {
		t.Fatalf("unchanged world rule was written again: result=%#v saved=%#v", result, fake.savedWorldRules)
	}
	found := false
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "world_rules" && skip["reason"] == "unchanged_existing_rule" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unchanged rule omission was not traced: %#v", result.SkipReasons)
	}
}

type criticWorldRuleReadFailingStore struct {
	*turnRecordingStore
}

func (s *criticWorldRuleReadFailingStore) ListWorldRules(context.Context, string) ([]store.WorldRule, error) {
	return nil, errors.New("world-rule read unavailable")
}

func TestCriticWorldRulePersistenceKeepsCandidateWhenExistingRulesCannotBeRead(t *testing.T) {
	base := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = &criticWorldRuleReadFailingStore{turnRecordingStore: base}
	extraction := normalizeCriticExtraction(map[string]any{
		"world_rules": []any{map[string]any{
			"scope": "system", "category": "access", "key": "gate_requires_seal", "value": "Gate access requires a seal.",
		}},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "world-read-failure", 3, extraction, "The gate requires a seal.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.WorldRules != 1 || len(base.savedWorldRules) != 1 {
		t.Fatalf("world rule candidate was dropped because duplicate lookup failed: result=%#v saved=%#v", result, base.savedWorldRules)
	}
	if !containsString(result.Warnings, "world_rule_existing_read_failed") {
		t.Fatalf("world-rule read failure was not surfaced: %#v", result.Warnings)
	}
	found := false
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "world_rules" && skip["reason"] == "existing_world_rules_read_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("world-rule read failure was not traced: %#v", result.SkipReasons)
	}
}

func TestCriticPromptReceivesActiveWorldRuleKeysWithoutSuppressedRows(t *testing.T) {
	fake := &turnRecordingStore{returnWorldRules: []store.WorldRule{
		{Scope: "system", Category: "access", Key: "gate_requires_seal", ValueJSON: `"Gate access requires a seal."`, SourceTurn: 2},
		{Scope: "system", Category: "access", Key: "obsolete_gate_rule", ValueJSON: `"obsolete"`, Suppressed: true},
	}}
	srv := NewServer(config.Default())
	srv.Store = fake
	active, trace := srv.buildCompleteTurnActiveWorldRuleInput(context.Background(), "world-prompt")
	if trace["status"] != "ok" || len(active) != 1 || stringFromMap(active[0], "key") != "gate_requires_seal" {
		t.Fatalf("active world-rule input = %#v trace=%#v", active, trace)
	}
	prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt("world-prompt", 3, "Approach the gate.", "The guard checks the seal.", nil, nil, map[string]any{"active_world_rules": active}))
	for _, expected := range []string{"gate_requires_seal", "reuse its exact scope, scope_name, category, and key", "omit that unchanged repeat"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("critic prompt missing active world-rule contract %q", expected)
		}
	}
	if strings.Contains(prompt, "obsolete_gate_rule") {
		t.Fatalf("suppressed world rule leaked into prompt: %s", prompt)
	}
}

func TestCriticPromptKeepsReversibleStateCollectionVocabularyOpen(t *testing.T) {
	prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt("session", 3, "Mina opens the door.", "Rowan watches.", nil, nil))
	for _, required := range []string{
		"reversible_states, physical_conditions, entity_conditions, state_deltas, and character_deltas may all preserve source-grounded continuity observations",
		"Saving broad observations is separate from deciding which value is current or injectable",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic prompt missing reversible-state sensitivity contract %q", required)
		}
	}
}

func TestCriticSystemPromptKeepsReversibleStateCollectionVocabularyOpen(t *testing.T) {
	prompt, source := readCriticSystemPrompt(filepath.Join("..", "..", "..", "prompts"))
	if source == "fallback_builtin" {
		t.Fatal("source critic_system.txt was not loaded")
	}
	for _, required := range []string{
		"Preserve the condition, subject, change, uncertainty, visibility, and sensitivity",
		"without forcing a fixed vocabulary at collection time",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("critic system prompt missing reversible-state sensitivity contract %q", required)
		}
	}
}

func TestCriticProtectedCollectionDoesNotUseContentSimilarityAsSaveGate(t *testing.T) {
	source := "Mina opened the garden door. Rowan watched from the hall."
	payload := map[string]any{
		"turn_summary": "Mina opened the door.",
		"protected_secrets": []any{
			map[string]any{
				"owner":             "Mina",
				"summary":           "Mina concealed a murder.",
				"disclosure_policy": "owner_private_until_revealed",
				"evidence_excerpt":  "Mina opened the garden door.",
			},
		},
		"subjective_entity_memories": []any{
			map[string]any{
				"owner_entity_name":    "Mina",
				"owner_entity_role":    "npc",
				"owner_visibility":     "owner_private",
				"memory_text":          "Mina remembers stealing the crown.",
				"target_reveal_policy": "owner_private_until_revealed",
				"evidence_excerpt":     "Mina opened the garden door.",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(payload, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 1 {
		t.Fatalf("structurally complete protected secret was dropped: %#v", filtered["protected_secrets"])
	}
	if got := len(sliceFromAny(filtered["subjective_entity_memories"])); got != 1 {
		t.Fatalf("structurally complete subjective memory was dropped: %#v", filtered["subjective_entity_memories"])
	}
	reasons := mapFromAny(trace["reasons"])
	if intFromAny(reasons["protected_secret_claim_unbound"], 0) != 0 ||
		intFromAny(reasons["protected_subjective_claim_unbound"], 0) != 0 {
		t.Fatalf("content-similarity save gate returned: %#v", trace)
	}
}

func TestPerspectiveClaimsCannotBeCopiedIntoObjectiveLanes(t *testing.T) {
	extraction := map[string]any{
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "vault is open",
			"evidence_excerpt":  "Mira privately told Rowan that the vault was open.",
		}},
		"narrative_events": []any{
			map[string]any{
				"event":            "Rowan knows the vault is open.",
				"evidence_excerpt": "Mira privately told Rowan that the vault was open.",
			},
			map[string]any{
				"event":            "The public bell rang.",
				"evidence_excerpt": "The public bell rang.",
			},
		},
		"state_claims": []any{
			map[string]any{
				"subject":          "vault",
				"state_slot":       "access",
				"value":            "vault is open",
				"evidence_excerpt": "Mira privately told Rowan that the vault was open.",
			},
		},
		"kg_triples": []any{
			map[string]any{"subject": "vault", "predicate": "status", "object": "open"},
			map[string]any{"subject": "bell", "predicate": "rang_at", "object": "noon"},
		},
	}
	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 3 {
		t.Fatalf("objective duplicate quarantine count=%d, want 3: %#v", quarantined, extraction)
	}
	events := sliceFromAny(extraction["narrative_events"])
	if len(events) != 1 || stringFromMap(mapFromAny(events[0]), "event") != "The public bell rang." {
		t.Fatalf("public objective event was not preserved: %#v", events)
	}
	if len(sliceFromAny(extraction["state_claims"])) != 0 {
		t.Fatalf("perspective state claim remained objective: %#v", extraction["state_claims"])
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "subject") != "bell" {
		t.Fatalf("public KG triple was not preserved: %#v", triples)
	}
}

func TestArchivedPrivateCandidateStillQuarantinesObjectiveDuplicate(t *testing.T) {
	source := "Mina opened the garden door."
	extraction := map[string]any{
		"turn_summary": "Mina opened the door.",
		"protected_secrets": []any{map[string]any{
			"owner":             "Mina",
			"summary":           "Mina concealed a murder.",
			"disclosure_policy": "owner_private_until_revealed",
			"evidence_excerpt":  "This excerpt is absent from the source.",
		}},
		"kg_triples": []any{
			map[string]any{
				"subject":   "Mina",
				"predicate": "concealed",
				"object":    "a killing",
			},
			map[string]any{
				"subject":   "garden door",
				"predicate": "state",
				"object":    "open",
			},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(extraction, "", source)
	if got := len(sliceFromAny(filtered["protected_secrets"])); got != 1 {
		t.Fatalf("structurally complete private candidate was deleted during collection: %#v", filtered["protected_secrets"])
	}
	triples := sliceFromAny(filtered["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "subject") != "garden door" {
		t.Fatalf("private objective duplicate was not quarantined: %#v", triples)
	}
	if intFromAny(trace["objective_lane_quarantined_count"], 0) != 1 {
		t.Fatalf("objective quarantine was not traced: %#v", trace)
	}
}

func TestPerspectiveObjectiveQuarantinePreservesUnrelatedPublicFactForSameOwner(t *testing.T) {
	extraction := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina secretly fears the magistrate.",
			"target_reveal_policy": "owner_private_until_revealed",
			"evidence_excerpt":     "Mina hid her fear from everyone.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "appointed_as", "object": "captain"},
			map[string]any{"subject": "Mina", "predicate": "secretly_fears", "object": "magistrate"},
		},
	}

	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 1 {
		t.Fatalf("objective quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "appointed_as" {
		t.Fatalf("unrelated public fact for the same owner was removed: %#v", triples)
	}
}

func TestPerspectiveObjectiveQuarantinePreservesPublicFactSharingOnlyClaimNoun(t *testing.T) {
	extraction := map[string]any{
		"subjective_entity_memories": []any{map[string]any{
			"owner_entity_name":    "Mina",
			"owner_visibility":     "owner_private",
			"memory_text":          "Mina fears becoming captain.",
			"target_reveal_policy": "owner_private_until_revealed",
			"evidence_excerpt":     "Mina privately feared becoming captain.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "appointed_as", "object": "captain"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 0 {
		t.Fatalf("public fact sharing only a noun was quarantined: %#v", extraction)
	}
	if got := len(sliceFromAny(extraction["kg_triples"])); got != 1 {
		t.Fatalf("public appointment fact was removed: %#v", extraction["kg_triples"])
	}
}

func TestPerspectiveObjectiveQuarantinePreservesObjectiveTruthOppositeBeliefValue(t *testing.T) {
	extraction := map[string]any{
		"belief_updates": []any{map[string]any{
			"perspective_owner": "Rowan",
			"subject":           "vault",
			"state_slot":        "access",
			"value":             "open",
			"epistemic_state":   "misinformed",
			"evidence_excerpt":  "Rowan wrongly believed the vault was open.",
		}},
		"state_claims": []any{
			map[string]any{
				"subject":          "vault",
				"state_slot":       "access",
				"value":            "closed",
				"evidence_excerpt": "The vault was closed.",
			},
			map[string]any{
				"subject":    "vault",
				"state_slot": "access",
				"value":      "open",
			},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("belief duplicate quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	states := sliceFromAny(extraction["state_claims"])
	if len(states) != 1 || stringFromMap(mapFromAny(states[0]), "value") != "closed" {
		t.Fatalf("objective truth opposite the character belief was removed: %#v", states)
	}
}

func TestPerspectiveObjectiveQuarantineCatchesIdentityAcrossBothKGEndpoints(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Shade admitted privately that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
			map[string]any{"subject": "Shade", "predicate": "entered", "object": "the hall"},
		},
	}

	quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction)
	if quarantined != 1 {
		t.Fatalf("identity objective quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "entered" {
		t.Fatalf("identity objective duplicate was not isolated precisely: %#v", triples)
	}
}

func TestPerspectiveObjectiveQuarantineUsesPrimaryIdentityPairNotEveryAlias(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"alias_name":            "Night",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Shade admitted privately that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("primary identity pair did not quarantine duplicate: %#v", extraction)
	}
}

func TestPerspectiveObjectiveQuarantineKeepsPubliclyRevealedIdentityObjective(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"transition":            "reveal",
			"knowledge_scope": map[string]any{
				"publicly_revealed": true,
			},
			"evidence_excerpt": "Shade publicly revealed that she was Alice.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 0 {
		t.Fatalf("publicly revealed identity was kept private: %#v", extraction)
	}
	if got := len(sliceFromAny(extraction["kg_triples"])); got != 1 {
		t.Fatalf("publicly revealed identity objective fact was removed: %#v", extraction["kg_triples"])
	}
}

func TestPerspectiveObjectiveQuarantineFailsClosedForSingleAnchorHiddenRoleWithoutEvidence(t *testing.T) {
	extraction := map[string]any{
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Mina",
			"true_identity_name":    "Mina",
			"same_entity":           true,
			"identity_kind":         "hidden_role",
			"true_role":             "spy",
			"reveal_policy":         "owner_private_until_revealed",
			"evidence_excerpt":      "Mina privately admitted that she served as a spy.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Mina", "predicate": "member_of", "object": "intelligence"},
			map[string]any{
				"subject":          "Mina",
				"predicate":        "entered",
				"object":           "the hall",
				"evidence_excerpt": "Mina entered the hall.",
			},
		},
	}

	if quarantined := quarantineCriticPerspectiveClaimsFromObjectiveLanes(extraction); quarantined != 1 {
		t.Fatalf("single-anchor hidden role quarantine count=%d, want 1: %#v", quarantined, extraction)
	}
	triples := sliceFromAny(extraction["kg_triples"])
	if len(triples) != 1 || stringFromMap(mapFromAny(triples[0]), "predicate") != "entered" {
		t.Fatalf("evidence-bound unrelated public fact was not preserved: %#v", triples)
	}
}

func TestArchivedUnboundPublicIdentityStillQuarantinesObjectiveDuplicate(t *testing.T) {
	source := "Shade entered the hall."
	extraction := map[string]any{
		"turn_summary": "Shade entered the hall.",
		"character_identity_accuracy": []any{map[string]any{
			"surface_identity_name": "Shade",
			"true_identity_name":    "Alice",
			"same_entity":           true,
			"reveal_policy":         "public_after_reveal",
			"transition":            "reveal",
			"knowledge_scope": map[string]any{
				"publicly_revealed": true,
			},
			"evidence_excerpt": "This public reveal is absent from the source.",
		}},
		"kg_triples": []any{
			map[string]any{"subject": "Shade", "predicate": "is_really", "object": "Alice"},
		},
	}

	filtered, trace := quarantineCriticProtectedCandidates(extraction, "", source)
	if got := len(sliceFromAny(filtered["character_identity_accuracy"])); got != 1 {
		t.Fatalf("structurally complete identity candidate was deleted during collection: %#v", filtered["character_identity_accuracy"])
	}
	if got := len(sliceFromAny(filtered["kg_triples"])); got != 0 {
		t.Fatalf("rejected false public identity leaked into objective KG: %#v", filtered["kg_triples"])
	}
	if intFromAny(trace["objective_lane_quarantined_count"], 0) != 1 {
		t.Fatalf("rejected public identity objective quarantine was not traced: %#v", trace)
	}
}
