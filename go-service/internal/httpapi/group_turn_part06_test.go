package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestPrepareTurnVectorHydrationUsesVectorIDFallbackAndFiltersNonMemory(t *testing.T) {
	memories := []store.Memory{
		{
			ID:          4,
			TurnIndex:   4,
			SummaryJSON: `{"turn_summary":"The shrine key was wrapped in red cloth."}`,
			Importance:  6,
		},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{
				"id":    "episode:sess-vector:99",
				"tier":  "episode",
				"score": 0.99,
			},
			{
				"id":                "memory:sess-vector:4",
				"tier":              "memory",
				"similarity":        0.84,
				"similarity_source": "cosine_from_query_and_stored_embedding",
			},
			{
				"id":                "memory:sess-vector:4",
				"tier":              "memory",
				"similarity":        0.84,
				"similarity_source": "cosine_from_query_and_stored_embedding",
			},
		},
	}

	selection := selectPrepareTurnMemoryLanesWithVector(memories, "red cloth key", 3, vectorShadow)
	if len(selection.VectorRelevant) != 1 || selection.VectorRelevant[0].ID != 4 {
		t.Fatalf("expected one hydrated memory from vector id fallback, got %#v", selection.VectorRelevant)
	}
	trace := mapFromAny(selection.Trace["vector_recall"])
	if got := intFromAny(trace["non_memory_count"], 0); got != 1 {
		t.Fatalf("non_memory_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["duplicate_count"], 0); got != 1 {
		t.Fatalf("duplicate_count = %d, want 1; trace=%#v", got, trace)
	}
	if got := intFromAny(trace["hydrated_count"], 0); got != 1 {
		t.Fatalf("hydrated_count = %d, want 1; trace=%#v", got, trace)
	}
}

func TestPrepareTurnProtectedGuardDiversityRefillsTopK(t *testing.T) {
	memories := []store.Memory{}
	vectorResults := []map[string]any{}
	for i := 1; i <= 7; i++ {
		memories = append(memories, store.Memory{
			ID:          int64(i),
			TurnIndex:   i,
			SummaryJSON: fmt.Sprintf(`{"turn_summary":"Repeated protected identity memory %d.","character_identity_accuracy":[{"surface_identity_name":"Lia","true_identity_name":"Gloria","canonical_entity_name":"Gloria","identity_kind":"cover_identity","same_entity":true,"reveal_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Gloria"]}}]}`, i),
			Importance:  0.9,
		})
		vectorResults = append(vectorResults, map[string]any{
			"id": fmt.Sprintf("memory:sess-diversity:%d", i), "source_table": "memories", "source_row_id": fmt.Sprint(i),
			"similarity": 0.95 - float64(i)/100, "similarity_source": "cosine_from_query_and_stored_embedding",
		})
	}
	for i := 8; i <= 19; i++ {
		memories = append(memories, store.Memory{
			ID: int64(i), TurnIndex: i,
			SummaryJSON: fmt.Sprintf(`{"turn_summary":"Actual event memory %d at the market."}`, i),
			Importance:  0.5,
		})
		vectorResults = append(vectorResults, map[string]any{
			"id": fmt.Sprintf("memory:sess-diversity:%d", i), "source_table": "memories", "source_row_id": fmt.Sprint(i),
			"similarity": 0.80 - float64(i)/1000, "similarity_source": "cosine_from_query_and_stored_embedding",
		})
	}
	vectorShadow := map[string]any{"search_result": "ok", "search_results": vectorResults}

	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 10, 12000, "Gloria returns to the market.", "default", nil, vectorShadow, nil, map[string]any{"current_pov": "Gloria", "source": "client_meta"})
	if got := strings.Count(assembly.MemoryText, "POV-scoped identity continuity:") + strings.Count(assembly.MemoryText, "Protected identity continuity:"); got != 1 {
		t.Fatalf("protected identity guard count = %d, want 1: %q", got, assembly.MemoryText)
	}
	if got := strings.Count(assembly.MemoryText, "Actual event memory"); got != 9 {
		t.Fatalf("actual vector event count = %d, want 9 plus one protected hit inside vector top_k 10: %q", got, assembly.MemoryText)
	}
	if got := intFromAny(assembly.Counts["selected_memory_total_count"], 0); got != 10 {
		t.Fatalf("selected memory count = %d, want 9 actual vector memories plus one guard: %#v", got, assembly.Counts)
	}
	if got := intFromAny(assembly.Counts["actual_memory_selected_count"], 0); got != 9 {
		t.Fatalf("actual memory count = %d, want 9: %#v", got, assembly.Counts)
	}
	policy := mapFromAny(assembly.Counts["memory_recall_lane_policy"])
	if got := intFromAny(policy["actual_memory_vector_selected"], 0); got != 9 {
		t.Fatalf("vector actual memory count = %d, want 9: %#v", got, policy)
	}
	if policy["protected_candidates_consume_actual_memory_target"] != false {
		t.Fatalf("protected candidates consumed actual-memory target: %#v", policy)
	}
}

func TestMEMCProtectedVectorDominanceRefillsOnlyEvidenceLinkedCanonicalMemories(t *testing.T) {
	memories := []store.Memory{}
	vectorResults := []map[string]any{}
	for i := 1; i <= 8; i++ {
		memories = append(memories, store.Memory{
			ID:        int64(i),
			TurnIndex: 20 + i,
			SummaryJSON: fmt.Sprintf(`{
				"turn_summary":"Gloria protected the Lia identity record %d.",
				"character_identity_accuracy":[{
					"surface_identity_name":"Lia","true_identity_name":"Gloria","canonical_entity_name":"Gloria",
					"identity_kind":"cover_identity","same_entity":true,
					"reveal_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Gloria"]}
				}]
			}`, i),
			Importance: 0.9,
		})
		vectorResults = append(vectorResults, map[string]any{
			"id": fmt.Sprintf("memory:sess-mem-c:%d", i), "source_table": "memories", "source_row_id": fmt.Sprint(i),
			"similarity": 0.98 - float64(i)/100, "similarity_source": "cosine_from_query_and_stored_embedding",
		})
	}
	memories = append(memories,
		store.Memory{ID: 9, TurnIndex: 5, SummaryJSON: `{"turn_summary":"Gloria가 시장조약에 붉은인장을 찍었다."}`, Importance: 0.6},
		store.Memory{ID: 10, TurnIndex: 90, SummaryJSON: `{"turn_summary":"미나는 해질녘 동문에 도착했다."}`, Importance: 0.5},
		store.Memory{ID: 11, TurnIndex: 91, SummaryJSON: `{"turn_summary":"미나는 문이 닫힌 뒤 기록관에 들어갔다."}`, Importance: 0.5},
		store.Memory{ID: 12, TurnIndex: 2, SummaryJSON: `{"turn_summary":"먼 옛날의 비 소식은 산맥 너머를 묘사했다."}`, Importance: 1.0},
	)
	vectorShadow := map[string]any{"search_result": "ok", "search_results": vectorResults}

	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		3, 12000, "Gloria가 시장조약의 붉은인장을 확인한다.", "default", nil, vectorShadow, nil,
		map[string]any{"current_pov": "Gloria", "source": "client_meta"},
	)
	if got := strings.Count(assembly.MemoryText, "identity continuity:"); got != 1 {
		t.Fatalf("protected identity guard count = %d, want 1: %q", got, assembly.MemoryText)
	}
	for _, required := range []string{
		"Gloria가 시장조약에 붉은인장을 찍었다.",
		"미나는 해질녘 동문에 도착했다.",
		"미나는 문이 닫힌 뒤 기록관에 들어갔다.",
	} {
		if strings.HasPrefix(required, "Gloria") && !strings.Contains(assembly.MemoryText, required) {
			t.Fatalf("canonical actual-memory refill lost %q: %q", required, assembly.MemoryText)
		}
	}
	if strings.Contains(assembly.MemoryText, "먼 옛날의 비 소식") {
		t.Fatalf("selection continued past the actual-memory target: %q", assembly.MemoryText)
	}
	if got := intFromAny(assembly.Counts["actual_memory_selected_count"], 0); got != 1 {
		t.Fatalf("actual memory count = %d, want only the evidence-linked memory: %#v", got, assembly.Counts)
	}
	policy := mapFromAny(assembly.Counts["memory_recall_lane_policy"])
	for key, want := range map[string]int{
		"actual_memory_vector_selected":          0,
		"actual_memory_relevant_refill_selected": 1,
		"actual_memory_recent_refill_selected":   0,
		"actual_memory_refill_selected":          1,
		"actual_memory_refill_gap":               0,
	} {
		if got := intFromAny(policy[key], -1); got != want {
			t.Fatalf("%s = %d, want %d: %#v", key, got, want, policy)
		}
	}
	if policy["protected_candidates_consume_actual_memory_target"] != false {
		t.Fatalf("protected candidate consumed actual-memory target: %#v", policy)
	}
}

func TestPrepareTurnMemoryLinesDeduplicateAfterProtectedRendering(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":"First wording.","protected_secrets":[{"secret_kind":"identity","disclosure_policy":"owner_private_until_revealed"}]}`},
		{ID: 2, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Second wording.","protected_secrets":[{"secret_kind":"identity","disclosure_policy":"owner_private_until_revealed"}]}`},
	}
	selection := prepareTurnMemoryLaneSelection{Relevant: memories, VectorScores: map[string]float64{}, RelevantScores: map[string]float64{}}
	lines, trace := prepareTurnMemoryLaneLines(selection, nil, nil)
	if len(lines) != 1 {
		t.Fatalf("final rendered lines = %d, want 1: %#v", len(lines), lines)
	}
	if got := intFromAny(trace["final_render_duplicate_count"], 0); got != 1 {
		t.Fatalf("final render duplicate count = %d, want 1: %#v", got, trace)
	}
}

func TestMEMBSamePersonSecretKindMergesOnceWithoutWeakeningProtection(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{
			"turn_summary":"Mina prepared the first concealed route.",
			"protected_secrets":[{
				"owner":"Mina","secret_kind":"hidden_plan","secret_summary":"first concealed route",
				"disclosure_policy":"owner_private_until_revealed",
				"knowledge_scope":{"known_by":["Mina"]}
			}]
		}`},
		{ID: 2, TurnIndex: 2, SummaryJSON: `{
			"turn_summary":"Mina revised the route with an advisor.",
			"protected_secrets":[{
				"owner":"Mina","secret_kind":"hidden_plan","secret_summary":"revised route details",
				"disclosure_policy":"explicit_user_reveal_required",
				"knowledge_scope":{"known_by":["Mina","Advisor"],"suspected_by":["Guard"]}
			}]
		}`},
		{ID: 3, TurnIndex: 3, SummaryJSON: `{
			"turn_summary":"Mina linked the route to a separate watch duty.",
			"protected_secrets":[
				{"owner":"Mina","secret_kind":"hidden_plan","secret_summary":"linked route details","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Mina"]}},
				{"owner":"Mina","secret_kind":"surveillance","secret_summary":"eastern watch timing","disclosure_policy":"current_session_confirmation_required","knowledge_scope":{"known_by":["Mina"]}}
			]
		}`},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 9000, "Mina reviews the hidden plan and surveillance duty.", "default", nil, nil, nil,
	)
	if got := strings.Count(assembly.MemoryText, "kind=hidden_plan"); got != 1 {
		t.Fatalf("same-person hidden_plan guard count = %d, want 1: %q", got, assembly.MemoryText)
	}
	if got := strings.Count(assembly.MemoryText, "kind=surveillance"); got != 1 {
		t.Fatalf("separate surveillance kind count = %d, want 1: %q", got, assembly.MemoryText)
	}
	for _, required := range []string{
		"owner_private_until_revealed",
		"explicit_user_reveal_required",
		"current_session_confirmation_required",
		"knowledge_scope=known:2 suspected:1",
		"Do not reveal, confess, or let unrelated characters discover it without current-scene evidence.",
	} {
		if !strings.Contains(assembly.MemoryText, required) {
			t.Fatalf("merged protection lost %q: %q", required, assembly.MemoryText)
		}
	}
	for _, leaked := range []string{"first concealed route", "revised route details", "linked route details", "eastern watch timing"} {
		if strings.Contains(assembly.MemoryText, leaked) {
			t.Fatalf("merged protection leaked secret source %q: %q", leaked, assembly.MemoryText)
		}
	}
	foundMergedHiddenPlan := false
	for _, raw := range sliceFromAny(assembly.MemoryDeliveryLineage["items"]) {
		item := mapFromAny(raw)
		if item["protected_coverage_key"] == "secret|mina|hidden_plan" {
			foundMergedHiddenPlan = true
			if got := intFromAny(item["merged_source_count"], 0); got != len(memories) {
				t.Fatalf("hidden_plan merged source count = %d, want %d: %#v", got, len(memories), item)
			}
		}
	}
	if !foundMergedHiddenPlan {
		t.Fatalf("hidden_plan coverage lineage missing: %#v", assembly.MemoryDeliveryLineage)
	}
}

func TestMEMBAmbiguousAliasDoesNotMergeDifferentPeople(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, TurnIndex: 1, SummaryJSON: `{"turn_summary":"Shade is Alice's cover.","character_identity_accuracy":[{"surface_identity_name":"Shade","true_identity_name":"Alice","canonical_entity_name":"Alice","identity_kind":"cover_identity","same_entity":true,"reveal_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Alice"]}}]}`},
		{ID: 2, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Shade is also recorded as Bob's cover.","character_identity_accuracy":[{"surface_identity_name":"Shade","true_identity_name":"Bob","canonical_entity_name":"Bob","identity_kind":"cover_identity","same_entity":true,"reveal_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Bob"]}}]}`},
		{ID: 3, TurnIndex: 3, SummaryJSON: `{"turn_summary":"First Shade has a hidden route.","protected_secrets":[{"owner":"Shade","secret_kind":"hidden_plan","secret_summary":"Alice route","disclosure_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Alice"]}}]}`},
		{ID: 4, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Second Shade has a different hidden route.","protected_secrets":[{"owner":"Shade","secret_kind":"hidden_plan","secret_summary":"Bob route","disclosure_policy":"explicit_user_reveal_required","knowledge_scope":{"known_by":["Bob"]}}]}`},
		{ID: 5, TurnIndex: 5, SummaryJSON: `{"turn_summary":"Shade crossed the northern bridge."}`},
		{ID: 6, TurnIndex: 6, SummaryJSON: `{"turn_summary":"Shade returned before dawn."}`},
	}
	vectorShadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{
			{"id": "memory:sess-mem-b:3", "source_table": "memories", "source_row_id": "3", "similarity": 0.91, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "memory:sess-mem-b:4", "source_table": "memories", "source_row_id": "4", "similarity": 0.89, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "memory:sess-mem-b:5", "source_table": "memories", "source_row_id": "5", "similarity": 0.82, "similarity_source": "cosine_from_query_and_stored_embedding"},
			{"id": "memory:sess-mem-b:6", "source_table": "memories", "source_row_id": "6", "similarity": 0.80, "similarity_source": "cosine_from_query_and_stored_embedding"},
		},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		2, 9000, "Shade considers the hidden plans.", "default", nil, vectorShadow, nil,
	)
	if got := strings.Count(assembly.MemoryText, "kind=hidden_plan"); got != 2 {
		t.Fatalf("ambiguous alias hidden plans were merged: count=%d text=%q", got, assembly.MemoryText)
	}
	for _, leaked := range []string{"Alice route", "Bob route"} {
		if strings.Contains(assembly.MemoryText, leaked) {
			t.Fatalf("ambiguous alias guard leaked secret source %q: %q", leaked, assembly.MemoryText)
		}
	}
	ambiguousGroups := 0
	for _, raw := range sliceFromAny(assembly.MemoryDeliveryLineage["items"]) {
		item := mapFromAny(raw)
		key := extractionStringFromAny(item["protected_coverage_key"])
		if strings.HasPrefix(key, "secret|ambiguous:shade:") && strings.HasSuffix(key, "|hidden_plan") {
			ambiguousGroups++
			if got := intFromAny(item["merged_source_count"], 0); got != 1 {
				t.Fatalf("ambiguous alias group merged sources: %#v", item)
			}
		}
	}
	if ambiguousGroups != 2 {
		t.Fatalf("ambiguous alias lineage groups = %d, want 2: %#v", ambiguousGroups, assembly.MemoryDeliveryLineage)
	}
}

func TestPrepareTurnNonCharacterPOVDoesNotAuthorizeProtectedMemory(t *testing.T) {
	memory := store.Memory{
		ID: 1, TurnIndex: 1,
		SummaryJSON: `{"turn_summary":"Gloria uses Lia as a protected cover identity.","character_identity_accuracy":[{"surface_identity_name":"Lia","true_identity_name":"Gloria","canonical_entity_name":"Gloria","identity_kind":"cover_identity","same_entity":true,"reveal_policy":"owner_private_until_revealed","knowledge_scope":{"known_by":["Gloria"]}}]}`,
	}
	assembly := buildPrepareTurnInjectionAssembly([]store.Memory{memory}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 3000, "Continue.", "default", nil, nil, nil, map[string]any{"current_pov": "of the most suffering character", "source": "client_meta"})
	if strings.Contains(assembly.MemoryText, "POV-scoped identity continuity") {
		t.Fatalf("non-character POV value authorized protected memory: %q", assembly.MemoryText)
	}
	if got := extractionStringFromAny(assembly.Counts["protected_perspective_ignored_reason"]); got != "current_pov_not_recognized_as_character" {
		t.Fatalf("ignored reason = %q, want current_pov_not_recognized_as_character: %#v", got, assembly.Counts)
	}
}

func TestPrepareTurnStorylineSelectionPreventsStaleAmplification(t *testing.T) {
	fake := &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-e1f", Name: "Fresh confrontation", Status: "active", CurrentContext: "Fresh confrontation escalates near the gate", Confidence: 0.82, EvidenceCount: 3, LastEvidenceTurn: 11, LastTurn: 11},
			{ID: 2, ChatSessionID: "sess-e1f", Name: "Old corridor rumor", Status: "active", CurrentContext: "Old corridor rumor repeats without evidence", Confidence: 0.9, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 1},
			{ID: 3, ChatSessionID: "sess-e1f", Name: "Resolved apology", Status: "resolved", CurrentContext: "Resolved apology should stay compressed", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 8, LastTurn: 8},
			{ID: 4, ChatSessionID: "sess-e1f", Name: "Suppressed detour", Status: "active", CurrentContext: "Suppressed detour must not enter prompt", Confidence: 1, EvidenceCount: 5, LastEvidenceTurn: 12, LastTurn: 12, Suppressed: true},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-e1f","turn_index":12,"raw_user_input":"Continue the fresh confrontation near the gate","settings":{"max_injection_chars":800,"injection_enabled":true,"input_context_enabled":false,"top_k":5}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	pack := resp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) {
		t.Fatalf("selected_count = %v, want 1: %#v", selection["selected_count"], selection)
	}
	if selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("stale_dropped_count = %v, want 1: %#v", selection["stale_dropped_count"], selection)
	}
	if selection["suppressed_count"] != float64(1) {
		t.Fatalf("suppressed_count = %v, want 1: %#v", selection["suppressed_count"], selection)
	}

	if _, exists := pack["storylines_context"]; exists {
		t.Fatalf("supervisor pack must not receive storyline prose as story guidance: %#v", pack["storylines_context"])
	}

	injectionPack := resp["injection_pack"].(map[string]any)
	storylineText, _ := injectionPack["storyline_text"].(string)
	if !strings.Contains(storylineText, "Fresh confrontation") {
		t.Fatalf("storyline_text missing fresh storyline: %q", storylineText)
	}
	if strings.Contains(storylineText, "Old corridor rumor") || strings.Contains(storylineText, "Suppressed detour") || strings.Contains(storylineText, "Resolved apology should stay compressed") {
		t.Fatalf("storyline_text contains stale/resolved/suppressed storyline: %q", storylineText)
	}
}

func TestPrepareTurnBundleKeepsEligibleMemoryWithoutChatLogFallback(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-fallback", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Only one brass key memory is available","items":["brass key"]}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-fallback", Subject: "Door", Predicate: "hides", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 100, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "user", Content: "I check the hallway."},
			{ID: 101, ChatSessionID: "sess-fallback", TurnIndex: 1, Role: "assistant", Content: "The hallway has a brass key under the rug."},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fallback","turn_index":2,"raw_user_input":"Use the brass key","settings":{"max_injection_chars":900,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	fallbackText, _ := injectionPack["fallback_text"].(string)
	if fallbackText != "" {
		t.Fatalf("fallback_text filled unused capacity despite an eligible memory: %q", fallbackText)
	}
	budget, ok := injectionPack["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}
	if budget["fallback_chat_log_included"] != false {
		t.Errorf("fallback_chat_log_included = %v, want false", budget["fallback_chat_log_included"])
	}
	if budget["fallback_reason"] != "memory_sufficient" {
		t.Errorf("fallback_reason = %v, want memory_sufficient", budget["fallback_reason"])
	}
	packCounts, ok := injectionPack["counts"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack.counts is not an object")
	}
	if packCounts["memory_count"] != float64(1) || packCounts["fallback_count"] != float64(0) {
		t.Errorf("injection_pack counts memory/fallback = %v/%v, want 1/0", packCounts["memory_count"], packCounts["fallback_count"])
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	items, ok := recall["items"].([]any)
	if !ok {
		t.Fatalf("recall_result.items is not an array")
	}
	foundMemoryItem := false
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m["source"] == "memory" && strings.Contains(fmt.Sprint(m["summary"]), "brass key") {
			foundMemoryItem = true
			break
		}
	}
	if !foundMemoryItem {
		t.Fatalf("recall_result.items missing eligible memory item: %#v", items)
	}
	recallCounts, ok := recall["counts"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.counts is not an object")
	}
	if recallCounts["memory_count"] != float64(1) || recallCounts["fallback_count"] != float64(0) {
		t.Errorf("recall_result counts memory/fallback = %v/%v, want 1/0", recallCounts["memory_count"], recallCounts["fallback_count"])
	}
}

func TestPrepareTurnTopKPrioritizesRelevantMemoryOverRecentTail(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, SummaryJSON: `{"turn_summary":"brass key old one"}`},
			{ID: 2, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, SummaryJSON: `{"turn_summary":"brass key old two"}`},
			{ID: 3, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, SummaryJSON: `{"turn_summary":"brass key old three"}`},
			{ID: 4, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, SummaryJSON: `{"turn_summary":"recent unrelated four"}`},
			{ID: 5, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, SummaryJSON: `{"turn_summary":"recent unrelated five"}`},
			{ID: 6, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, SummaryJSON: `{"turn_summary":"recent unrelated six"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 102, ChatSessionID: "sess-recall-lanes", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 201, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 202, ChatSessionID: "sess-recall-lanes", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
			{ID: 301, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "user", Content: "turn three user"},
			{ID: 302, ChatSessionID: "sess-recall-lanes", TurnIndex: 3, Role: "assistant", Content: "turn three assistant"},
			{ID: 401, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "user", Content: "turn four user"},
			{ID: 402, ChatSessionID: "sess-recall-lanes", TurnIndex: 4, Role: "assistant", Content: "turn four assistant"},
			{ID: 501, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "user", Content: "turn five user"},
			{ID: 502, ChatSessionID: "sess-recall-lanes", TurnIndex: 5, Role: "assistant", Content: "turn five assistant"},
			{ID: 601, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "user", Content: "turn six user"},
			{ID: 602, ChatSessionID: "sess-recall-lanes", TurnIndex: 6, Role: "assistant", Content: "turn six assistant"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-recall-lanes","turn_index":7,"raw_user_input":"brass key","settings":{"max_injection_chars":2000,"injection_enabled":true,"input_context_enabled":false,"top_k":3}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.lastEpisodeLimit != 64 {
		t.Fatalf("episode summary candidate read limit = %d, want independent safety bound 64", fake.lastEpisodeLimit)
	}
	if fake.lastPersonaLimit != 256 {
		t.Fatalf("persona recollection candidate read limit = %d, want independent safety bound 256", fake.lastPersonaLimit)
	}
	if fake.lastEntityMemoryLimit != 256 {
		t.Fatalf("character-private recollection candidate read limit = %d, want independent safety bound 256", fake.lastEntityMemoryLimit)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	injectionPack := resp["injection_pack"].(map[string]any)
	memoryText, _ := injectionPack["memory_text"].(string)
	for _, want := range []string{"brass key old three", "brass key old two", "brass key old one"} {
		if !strings.Contains(memoryText, want) {
			t.Fatalf("memory_text missing %q: %s", want, memoryText)
		}
	}
	for _, unwanted := range []string{"recent unrelated six", "recent unrelated five", "recent unrelated four"} {
		if strings.Contains(memoryText, unwanted) {
			t.Fatalf("unrelated recent memory was selected ahead of relevant memory %q: %s", unwanted, memoryText)
		}
	}

	recentRawTurnText, _ := injectionPack["recent_raw_turn_text"].(string)
	for _, want := range []string{"turn six user", "turn six assistant"} {
		if !strings.Contains(recentRawTurnText, want) {
			t.Fatalf("recent_raw_turn_text missing %q: %s", want, recentRawTurnText)
		}
	}
	for _, unwanted := range []string{"turn three user", "turn four user", "turn five user"} {
		if strings.Contains(recentRawTurnText, unwanted) {
			t.Fatalf("recent_raw_turn_text exceeded the previous logical turn with %q: %s", unwanted, recentRawTurnText)
		}
	}

	counts := injectionPack["counts"].(map[string]any)
	if counts["top_k_memory_target"] != float64(3) || counts["recent_memory_bound"] != float64(0) || counts["relevant_memory_bound"] != float64(3) {
		t.Fatalf("topK/recent counts mismatch: %+v", counts)
	}

	recall := resp["recall_result"].(map[string]any)
	searchBundle := recall["search"].(map[string]any)
	if searchBundle["memory_count"] != float64(3) || searchBundle["fallback_count"] != float64(0) {
		t.Fatalf("recall_result.search counts mismatch: %+v", searchBundle)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	recent := recallLanes["recent"].(map[string]any)
	if recent["count"] != float64(0) {
		t.Fatalf("recent lane count = %v, want 0 when relevant lane fills topK: %+v", recent["count"], recent)
	}
	relevant := recallLanes["relevant"].(map[string]any)
	items := relevant["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("relevant lane item count = %d, want 3: %+v", len(items), items)
	}
	wantTurns := []float64{3, 2, 1}
	for i, item := range items {
		row := item.(map[string]any)
		if row["turn_index"] != wantTurns[i] {
			t.Fatalf("relevant lane item %d turn_index = %v, want %v: %+v", i, row["turn_index"], wantTurns[i], items)
		}
	}
	trace := recall["trace"].(map[string]any)
	laneTrace := trace["r2_recall_lanes"].(map[string]any)
	if laneTrace["top_k_memory_target"] != float64(3) || laneTrace["recent_memory_count"] != float64(0) || laneTrace["relevant_memory_count"] != float64(3) {
		t.Fatalf("trace topK/recent mismatch: %+v", laneTrace)
	}
}

func TestRecentPrepareTurnRawTurnKeepsOnlyPreviousLogicalTurn(t *testing.T) {
	logs := []store.ChatLog{}
	for turn := 1; turn <= 10; turn++ {
		logs = append(logs,
			store.ChatLog{TurnIndex: turn, Role: "user", Content: fmt.Sprintf("turn %02d user", turn)},
			store.ChatLog{TurnIndex: turn, Role: "assistant", Content: fmt.Sprintf("turn %02d assistant", turn)},
		)
	}

	text := recentPrepareTurnRawTurn(logs)
	for _, want := range []string{"turn 10 user", "turn 10 assistant"} {
		if !strings.Contains(text, want) {
			t.Fatalf("recent raw turn text missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{"turn 01 user", "turn 08 user", "turn 09 assistant"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("historical raw chat leaked into previous-turn surface %q: %s", unwanted, text)
		}
	}
}

func TestPrepareTurnRelationshipRecallRejectsSingleEndpointHistory(t *testing.T) {
	query := "Alice and Eve inspect the sealed gate."
	if ok, reason := prepareTurnKGRecallEligible(query, store.KGTriple{Subject: "Alice", Predicate: "met", Object: "Rowan"}); ok || reason != "single_endpoint_only" {
		t.Fatalf("single-endpoint historical edge accepted: ok=%v reason=%q", ok, reason)
	}
	if ok, reason := prepareTurnKGRecallEligible(query, store.KGTriple{Subject: "Alice", Predicate: "trusts", Object: "Eve"}); !ok || reason != "both_endpoints_current" {
		t.Fatalf("current two-endpoint edge rejected: ok=%v reason=%q", ok, reason)
	}
	if ok, reason := prepareTurnKGRecallEligible(query, store.KGTriple{Subject: "Alice", Predicate: "carries", Object: "sealed key"}); !ok || reason != "endpoint_plus_relation_evidence" {
		t.Fatalf("endpoint plus event evidence rejected: ok=%v reason=%q", ok, reason)
	}

	raw := `{"Eve":{"trust":74},"Rowan":{"trust":20}}`
	text, dropped := prepareTurnRelevantRelationshipSurface(raw, "Alice", query, []string{"Alice", "Eve"}, []string{"Alice", "Eve", "Rowan"})
	if !strings.Contains(text, "Eve") || strings.Contains(text, "Rowan") {
		t.Fatalf("relationship surface did not retain only the current counterpart: %q", text)
	}
	if dropped != 1 {
		t.Fatalf("relationship dropped=%d, want 1", dropped)
	}
}

func TestPrepareTurnRecallLanesUseRawFallbackWhenVectorIndexNotReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, SummaryJSON: `{"turn_summary":"old memory one"}`},
			{ID: 2, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, SummaryJSON: `{"turn_summary":"old memory two"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 101, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "user", Content: "first raw turn"},
			{ID: 102, ChatSessionID: "sess-vector-degrade", TurnIndex: 1, Role: "assistant", Content: "first assistant raw turn"},
			{ID: 201, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "user", Content: "second raw turn"},
			{ID: 202, ChatSessionID: "sess-vector-degrade", TurnIndex: 2, Role: "assistant", Content: "second assistant raw turn"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-vector-degrade","turn_index":3,"raw_user_input":"Continue","settings":{"injection_enabled":true,"input_context_enabled":false,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall := resp["recall_result"].(map[string]any)
	readiness := recall["vector_readiness"].(map[string]any)
	if readiness["status"] != "embedding_model_not_ready" || readiness["fallback_recommended"] != true || readiness["reindex_recommended"] != true {
		t.Fatalf("vector readiness mismatch: %+v", readiness)
	}
	recallLanes := recall["recall_lanes"].(map[string]any)
	rawFallback := recallLanes["raw_fallback"].(map[string]any)
	if rawFallback["active"] != true || rawFallback["count"] != float64(4) {
		t.Fatalf("raw_fallback lane mismatch: %+v", rawFallback)
	}
}

func TestPrepareTurnChromaRecallReadUsesClientMetaVector(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["would_call_vector"] != true {
		t.Errorf("recall_result.would_call_vector = %v, want true", recall["would_call_vector"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["recall_read_drill_enabled"] != true {
		t.Errorf("recall_read_drill_enabled = %v, want true", vectorShadow["recall_read_drill_enabled"])
	}
	if vectorShadow["engine"] != "chromadb" {
		t.Errorf("engine = %v, want chromadb", vectorShadow["engine"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["search_result"] != "not_found" {
		t.Errorf("search_result = %v, want not_found", vectorShadow["search_result"])
	}
	if vectorShadow["query_vector_dim"] != float64(2) {
		t.Errorf("query_vector_dim = %v, want 2", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["live_retrieval_enabled"] != false {
		t.Errorf("live_retrieval_enabled = %v, want false", vectorShadow["live_retrieval_enabled"])
	}
}

func TestPrepareTurnChromaEndpointMarksR2Source(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vector.NewFakeVectorStore()
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"chroma_query_vector":[0.1,0.2],
			"chroma_filter":"chat_session_id == \"sess-prep\""
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if recall["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("recall_result.source = %v, want go_r2_chromadb_product_read", recall["source"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["product_read_enabled"] != true {
		t.Errorf("product_read_enabled = %v, want true", vectorShadow["product_read_enabled"])
	}
	if vectorShadow["live_retrieval_enabled"] != true {
		t.Errorf("live_retrieval_enabled = %v, want true", vectorShadow["live_retrieval_enabled"])
	}
	if vectorShadow["chromadb_live_enabled"] != true {
		t.Errorf("chromadb_live_enabled = %v, want true", vectorShadow["chromadb_live_enabled"])
	}
}

func TestPrepareTurnChromaRecallAutoEmbedsRawUserInput(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.test/v1/embeddings" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer embed-key" {
			t.Fatalf("Authorization = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "Find the relevant memory") {
			t.Fatalf("embedding request did not include raw user input: %s", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &turnRecordingVectorStore{}
	srv.VectorOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-prep",
		"turn_index":1,
		"raw_user_input":"Find the relevant memory",
		"client_meta":{
			"embedding":{
				"api_key":"embed-key",
				"endpoint":"https://api.example.test/v1",
				"model":"embed-model",
				"provider":"openai"
			}
		},
		"settings":{"top_k":2}
	}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("vector_shadow missing or not object")
	}
	if vectorShadow["query_embedding_status"] != "ok" {
		t.Errorf("query_embedding_status = %v, want ok", vectorShadow["query_embedding_status"])
	}
	if vectorShadow["query_vector_key"] != "server_query_embedding" {
		t.Errorf("query_vector_key = %v, want server_query_embedding", vectorShadow["query_vector_key"])
	}
	if vectorShadow["query_vector_dim"] != float64(3) {
		t.Errorf("query_vector_dim = %v, want 3", vectorShadow["query_vector_dim"])
	}
	if vectorShadow["search_attempted"] != true {
		t.Errorf("search_attempted = %v, want true", vectorShadow["search_attempted"])
	}
	if vectorShadow["source"] != "go_r2_chromadb_product_read" {
		t.Errorf("source = %v, want go_r2_chromadb_product_read", vectorShadow["source"])
	}
}

func TestPrepareTurnCharCapsAndTruncation(t *testing.T) {

	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-cap", TurnIndex: 1, SummaryJSON: `{"turn_summary":"test calibration ` + strings.Repeat("x", 300) + `","entities":[{"name":"test"}]}`, Importance: 0.5},
			{ID: 2, ChatSessionID: "sess-cap", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test calibration ` + strings.Repeat("y", 300) + `","entities":[{"name":"test"}]}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-cap", Subject: "A", Predicate: "B", Object: "C"},
		},
		returnEvidence:        []store.DirectEvidence{},
		returnChatLogs:        []store.ChatLog{},
		returnResumePack:      nil,
		returnStorylines:      nil,
		returnWorldRules:      nil,
		returnCharStates:      nil,
		returnPendingThreads:  nil,
		returnActiveStates:    nil,
		returnCanonicalLayers: nil,
		returnEpisodeSums:     nil,
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-cap","turn_index":3,"raw_user_input":"test calibration","settings":{"max_injection_chars":50,"max_input_context_chars":50,"injection_enabled":true,"input_context_enabled":true,"top_k":10}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	injection, _ := gp["injection_text"].(string)
	if len(injection) > 50 {
		t.Errorf("injection_text length %d exceeds cap 50", len(injection))
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["injection_truncated"] != true {
		t.Errorf("injection_truncated = %v, want true (injection was truncated)", trace["injection_truncated"])
	}
}

// prepareTurnNotEnabledStore returns ErrNotEnabled for all narrative/read methods.
type prepareTurnNotEnabledStore struct {
	memoryFakeStore
}

func (n *prepareTurnNotEnabledStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListStorylines(ctx context.Context, sid string) ([]store.Storyline, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListWorldRules(ctx context.Context, sid string) ([]store.WorldRule, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListCharacterStates(ctx context.Context, sid string) ([]store.CharacterState, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListPendingThreads(ctx context.Context, sid, status string) ([]store.PendingThread, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListActiveStates(ctx context.Context, sid, stateType string) ([]store.ActiveState, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]store.CanonicalStateLayer, error) {
	return nil, store.ErrNotEnabled
}

func (n *prepareTurnNotEnabledStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, store.ErrNotEnabled
}

func TestPrepareTurnErrNotEnabledSafeFallback(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ne","turn_index":1,"raw_user_input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Errorf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Errorf("packet_mode = %v, want off", gp["packet_mode"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "degraded" {
		t.Errorf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["source"] != "go_r1_read_shadow" {
		t.Errorf("recall_result.source = %v, want go_r1_read_shadow", rr["source"])
	}
	if rr["would_write"] != false {
		t.Errorf("would_write = %v, want false", rr["would_write"])
	}
	vs, ok := rr["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow missing")
	}
	if vs["status"] != "shadow" {
		t.Errorf("vector_shadow.status = %v, want shadow", vs["status"])
	}
	if vs["health_checked"] != true {
		t.Errorf("vector_shadow.health_checked = %v, want true", vs["health_checked"])
	}
	if vs["search_attempted"] != false {
		t.Errorf("vector_shadow.search_attempted = %v, want false", vs["search_attempted"])
	}

	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "degraded" {
		t.Errorf("session_state.snapshot_status = %v, want degraded", ss["snapshot_status"])
	}
	ssMeta, _ := ss["section_meta"].(map[string]any)
	if ssMeta["storyline_count"] != float64(0) {
		t.Errorf("session_state.section_meta.storyline_count = %v, want 0", ssMeta["storyline_count"])
	}

	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "skeleton" {
		t.Errorf("narrative_control.state_status = %v, want skeleton", nc["state_status"])
	}
	if nc["storyline_count"] != float64(0) {
		t.Errorf("narrative_control.storyline_count = %v, want 0", nc["storyline_count"])
	}
	if nc["would_call_llm"] != false {
		t.Errorf("narrative_control.would_call_llm = %v, want false", nc["would_call_llm"])
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "degraded" {
		t.Errorf("continuity_pack.status = %v, want degraded", cp["status"])
	}
	if cp["resume_pack_present"] != false {
		t.Errorf("continuity_pack.resume_pack_present = %v, want false", cp["resume_pack_present"])
	}
	cpItems, _ := cp["items"].([]any)
	if len(cpItems) != 0 {
		t.Errorf("continuity_pack.items len = %d, want 0", len(cpItems))
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}
}

func TestPrepareTurnRespectsDisabledFlags(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-flag", TurnIndex: 1, SummaryJSON: `{"turn_summary":"summary"}`, Importance: 0.5},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-flag", Subject: "A", Predicate: "B", Object: "C"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-flag","turn_index":2,"raw_user_input":"test","settings":{"injection_enabled":false,"input_context_enabled":false}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["injection_text"] != nil {
		t.Errorf("injection_text = %v, want nil when disabled", resp["injection_text"])
	}
	if resp["input_context_text"] != nil {
		t.Errorf("input_context_text = %v, want nil when disabled", resp["input_context_text"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing or not object")
	}
	if rr["status"] != "ready" {
		t.Errorf("recall_result.status = %v, want ready", rr["status"])
	}
	if rr["query_preview"] != "test" {
		t.Errorf("query_preview = %v, want test", rr["query_preview"])
	}
	if _, ok := rr["counts"]; !ok {
		t.Errorf("recall_result.counts missing")
	}
	if _, ok := rr["vector_shadow"]; !ok {
		t.Errorf("recall_result.vector_shadow missing")
	}
}

// TestPrepareTurnDegradedFailOpenPreservesTruthFloor proves that a degraded store
// (timeout/failure simulation) does not block the main turn path and does not
// overwrite canonical state. It verifies the response still returns 200 OK,
// injection_text is present, progression_ledger has would_write=false, and
// trace_preview contains fallback indicators. (SEQ-12-P33 RMG-01)
func TestPrepareTurnDegradedFailOpenPreservesTruthFloor(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &prepareTurnNotEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-p33","turn_index":1,"raw_user_input":"hello","settings":{"max_injection_chars":500,"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (main turn must not be blocked)", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet missing")
	}
	if gp["degraded"] != true {
		t.Fatalf("degraded = %v, want true", gp["degraded"])
	}
	if gp["packet_mode"] != "off" {
		t.Fatalf("packet_mode = %v, want off", gp["packet_mode"])
	}

	if _, ok := resp["injection_text"]; !ok {
		t.Fatalf("injection_text key must be present to preserve turn contract")
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger missing")
	}
	if pl["would_write"] != false {
		t.Fatalf("progression_ledger.would_write = %v, want false (truth floor must not be overwritten)", pl["would_write"])
	}
	if pl["status"] != "degraded" {
		t.Fatalf("progression_ledger.status = %v, want degraded", pl["status"])
	}

	tracePreview, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview missing")
	}
	if tracePreview["would_write"] != false {
		t.Fatalf("trace_preview.would_write = %v, want false", tracePreview["would_write"])
	}
	if tracePreview["would_call_llm"] != false {
		t.Fatalf("trace_preview.would_call_llm = %v, want false", tracePreview["would_call_llm"])
	}

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result missing")
	}
	if rr["status"] != "degraded" {
		t.Fatalf("recall_result.status = %v, want degraded", rr["status"])
	}
	if rr["would_write"] != false {
		t.Fatalf("recall_result.would_write = %v, want false", rr["would_write"])
	}
}
