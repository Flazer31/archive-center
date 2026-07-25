package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func outputFidelity35CPrepareResponse(t *testing.T, caseID, guideMode, guideStrength, supervisorEndpoint string, withMemorySupport bool) map[string]any {
	return outputFidelity35CPrepareResponseWithOptions(t, caseID, guideMode, guideStrength, supervisorEndpoint, withMemorySupport, true, 6000)
}

func outputFidelity35CPrepareResponseWithInjection(t *testing.T, caseID, guideMode, guideStrength, supervisorEndpoint string, withMemorySupport, injectionEnabled bool) map[string]any {
	return outputFidelity35CPrepareResponseWithOptions(t, caseID, guideMode, guideStrength, supervisorEndpoint, withMemorySupport, injectionEnabled, 6000)
}

func outputFidelity35CPrepareResponseWithOptions(t *testing.T, caseID, guideMode, guideStrength, supervisorEndpoint string, withMemorySupport, injectionEnabled bool, narrativeSupportMaxChars int) map[string]any {
	t.Helper()
	corpus, _ := loadOutputFidelityCorpus(t)
	fixture := outputFidelityCaseByID(t, corpus, caseID)
	current := outputFidelitySourceByLifecycle(t, fixture, "user_input", "current_observation")
	const nativeSystemText = "Stay in character and follow the current chat context."

	const (
		sid         = "output-fidelity-35c-prepare"
		memoryRowID = int64(51)
		sourceTurn  = 4
	)
	fake := &turnRecordingStore{}
	vectorStore := &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 0, ModelReady: true},
	}
	if withMemorySupport {
		prior := outputFidelitySourceByLifecycle(t, fixture, "assistant_output", "active_final")
		summaryJSON, err := json.Marshal(map[string]any{"turn_summary": prior.ReplayText})
		if err != nil {
			t.Fatal(err)
		}
		fake.returnMemories = []store.Memory{{
			ID:            memoryRowID,
			ChatSessionID: sid,
			TurnIndex:     sourceTurn,
			SummaryJSON:   string(summaryJSON),
			Importance:    0.9,
		}}
		fake.returnChatLogs = []store.ChatLog{
			{ID: 1, ChatSessionID: sid, TurnIndex: sourceTurn, Role: "user", Content: "B와 주인공은 돌다리에서 다시 만나기로 했다."},
			{ID: 2, ChatSessionID: sid, TurnIndex: sourceTurn, Role: "assistant", Content: prior.ReplayText},
		}
		memorySourceRef := fmt.Sprintf("memory:%s:%d", sid, memoryRowID)
		vectorStore.healthSnapshot.TotalCount = 1
		vectorStore.searchResults = []vector.VectorDocument{{
			ID:                  memorySourceRef,
			Tier:                "memory",
			ChatSessionID:       sid,
			SourceTable:         "memories",
			SourceRowID:         fmt.Sprint(memoryRowID),
			Similarity:          0.95,
			SimilarityAvailable: true,
			SimilaritySource:    "cosine_from_query_and_stored_embedding",
		}}
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = vectorStore
	srv.VectorOpenError = nil
	if supervisorEndpoint != "" {
		srv.RuntimeConfig.SupervisorProvider = "openai"
		srv.RuntimeConfig.SupervisorAPIKey = "test-only-supervisor-key"
		srv.RuntimeConfig.SupervisorEndpoint = supervisorEndpoint
		srv.RuntimeConfig.SupervisorModel = "test-supervisor"
		srv.RuntimeConfig.SupervisorTimeoutSec = 2
	}

	body, err := json.Marshal(map[string]any{
		"chat_session_id":             sid,
		"turn_index":                  21,
		"raw_user_input":              current.ReplayText,
		"narrative_support_max_chars": narrativeSupportMaxChars,
		"messages": []map[string]any{
			{"role": "system", "content": nativeSystemText},
			{"role": "user", "content": current.ReplayText},
		},
		"source_observation": map[string]any{
			"contract_version":         prepareSourceObservationVersion,
			"session_id":               sid,
			"request_id":               "archive-center-host-observation-35c",
			"message_index":            1,
			"observed_role":            "user",
			"observed_source_path":     `["messages",1]`,
			"raw_input_hash":           prepareOR1CHash(current.ReplayText),
			"raw_input_hash_algorithm": "or1c_utf16_djb2.v1",
			"observable":               true,
			"evidence_state":           "observed",
		},
		"capability_observation": map[string]any{
			"contract_version": prepareCapabilityObservationVersion,
			"capabilities": map[string]string{
				"message_position":    "observed",
				"message_role":        "observed",
				"request_correlation": "observed",
				"session_identity":    "observed",
				"raw_input_hash":      "observed",
				"source_path":         "observed",
			},
		},
		"host_observations": map[string]any{
			"contract_version":          prepareHostObservationsVersion,
			"session_id":                sid,
			"request_id":                "archive-center-host-observation-35c",
			"request_type":              "model",
			"payload_writable":          true,
			"payload_observation_stage": "archive_center_before_request",
			"final_payload_observation": "not_exposed",
			"active_chat": []map[string]any{
				{
					"observation_ref": "active:1", "source_kind": "active_chat",
					"observation_stage": "active_chat_stored_message",
					"message_index":     1, "role": "user", "raw_content": current.ReplayText,
					"content_hash": prepareOR1CHash(current.ReplayText), "hash_algorithm": "or1c_utf16_djb2.v1",
					"evidence_state": "observed",
				},
			},
			"payload": []map[string]any{
				{
					"observation_ref": "payload:0", "source_kind": "before_request_payload",
					"message_index": 0, "role": "system", "raw_content": nativeSystemText,
					"content_hash": prepareOR1CHash(nativeSystemText), "hash_algorithm": "or1c_utf16_djb2.v1",
					"evidence_state": "observed",
				},
				{
					"observation_ref": "payload:1", "source_kind": "before_request_payload",
					"message_index": 1, "role": "user", "raw_content": current.ReplayText,
					"content_hash": prepareOR1CHash(current.ReplayText), "hash_algorithm": "or1c_utf16_djb2.v1",
					"evidence_state": "observed",
				},
			},
		},
		"client_meta": map[string]any{
			"chroma_query_vector":                   []float64{0.4, 0.6},
			"archive_center_request_correlation_id": "ac-correlation-output-fidelity-35c",
		},
		"settings": map[string]any{
			"apply_mode":            "shadow",
			"max_injection_chars":   9000,
			"injection_enabled":     injectionEnabled,
			"input_context_enabled": false,
			"top_k":                 1,
			"guide_mode":            guideMode,
			"guide_strength":        guideStrength,
			"narrative_stance":      "balanced",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder, response := prepareTurnPerfRequest(t, srv, string(body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("prepare-turn status = %d: %#v", recorder.Code, response)
	}
	return response
}

func outputFidelity35CGuideTrace(t *testing.T, response map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	plan := mapFromAny(response["payload_application_plan"])
	if plan["contract_version"] != "payload_application_plan.v1" {
		t.Fatalf("payload application plan missing: plan=%#v response=%#v", plan, response)
	}
	trace := mapFromAny(plan["guidance_application_trace"])
	if trace["contract_version"] != "guidance_application_trace.v1" {
		t.Fatalf("guidance trace missing: %#v", trace)
	}
	return plan, trace
}

func TestOutputFidelity35CNoSupportHasZeroGuideWork(t *testing.T) {
	var supervisorCalls atomic.Int64
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		http.Error(w, "no-support must not call supervisor", http.StatusInternalServerError)
	}))
	defer supervisor.Close()

	var baselineAuxiliaryHash any
	var baselineInputHash any
	for _, profile := range []struct {
		mode     string
		strength string
	}{
		{mode: "off", strength: "weak"},
		{mode: "standard", strength: "weak"},
		{mode: "standard", strength: "medium"},
		{mode: "standard", strength: "strong"},
	} {
		t.Run(profile.mode+"_"+profile.strength, func(t *testing.T) {
			response := outputFidelity35CPrepareResponse(t, "ko_reencounter_no_support_v1", profile.mode, profile.strength, supervisor.URL, false)
			plan, trace := outputFidelity35CGuideTrace(t, response)
			lane := outputFidelity35BFindLane(plan, "output_guidance")
			if trace["eligibility"] != map[bool]string{true: "off", false: "no_support"}[profile.mode == "off"] {
				t.Errorf("eligibility = %v", trace["eligibility"])
			}
			if intFromAny(trace["budget_chars"], -1) != 0 ||
				intFromAny(trace["used_chars"], -1) != 0 ||
				intFromAny(trace["applied_count"], -1) != 0 ||
				len(sliceFromAny(trace["items"])) != 0 {
				t.Errorf("no-support generated guide work: %#v", trace)
			}
			if lane == nil ||
				boolFromAny(lane["applied"]) ||
				extractionStringFromAny(lane["text"]) != "" ||
				len(stringSliceFromAny(lane["source_refs"])) != 0 {
				t.Errorf("no-support output-guidance lane is not empty: %#v", lane)
			}
			hostEvidence := mapFromAny(response["host_context_reference_evidence"])
			if intFromAny(hostEvidence["selected_count"], 0) != 1 {
				t.Errorf("test did not exercise the official system-message shape: %#v", hostEvidence)
			}
			evaluation := mapFromAny(plan["guide_efficacy_evaluation"])
			wantEvaluationStatus := map[bool]string{true: "not_applicable_guide_off", false: "not_applicable_no_support"}[profile.mode == "off"]
			if evaluation["contract_version"] != "guide_efficacy_evaluation.v1" ||
				evaluation["status"] != wantEvaluationStatus ||
				len(sliceFromAny(evaluation["item_results"])) != 0 ||
				boolFromAny(evaluation["automatic_text_judgement"]) ||
				boolFromAny(evaluation["body_difference_is_efficacy_evidence"]) {
				t.Errorf("no-support evaluation invented guide efficacy: %#v", evaluation)
			}
			if baselineAuxiliaryHash == nil {
				baselineAuxiliaryHash = plan["auxiliary_hash"]
				baselineInputHash = plan["input_context_hash"]
			} else if plan["auxiliary_hash"] != baselineAuxiliaryHash || plan["input_context_hash"] != baselineInputHash {
				t.Errorf("no-support profile forced a payload difference: plan=%#v", plan)
			}
		})
	}
	if got := supervisorCalls.Load(); got != 0 {
		t.Fatalf("no-support supervisor calls = %d, want 0", got)
	}
}

func TestOutputFidelity35CEligibleGuideMatrixUsesSameSourceSnapshot(t *testing.T) {
	type observation struct {
		sourceRefs     []string
		memoryLaneHash any
		roles          []string
		outputHash     any
	}
	observations := map[string]observation{}
	for _, profile := range []struct {
		mode     string
		strength string
	}{
		{mode: "off", strength: "weak"},
		{mode: "standard", strength: "weak"},
		{mode: "standard", strength: "medium"},
		{mode: "standard", strength: "strong"},
	} {
		name := map[bool]string{true: "off", false: profile.strength}[profile.mode == "off"]
		response := outputFidelity35CPrepareResponse(t, "ko_reencounter_supported_v1", profile.mode, profile.strength, "", true)
		plan, trace := outputFidelity35CGuideTrace(t, response)
		prepareLineage := mapFromAny(response["source_to_payload_lineage"])
		observedSourceRefs := stringSliceFromAny(prepareLineage["source_refs"])
		memoryLane := outputFidelity35BFindLane(plan, "long_term_memory")
		outputLane := outputFidelity35BFindLane(plan, "output_guidance")
		if memoryLane == nil || outputLane == nil {
			t.Fatalf("%s missing lanes: memory=%#v output=%#v", name, memoryLane, outputLane)
		}
		deliveryPlan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
		deliveredMemory := extractionStringFromAny(deliveryPlan["final_text"])
		if extractionStringFromAny(memoryLane["text"]) != deliveredMemory ||
			memoryLane["content_hash"] != prepareTurnTextHash(deliveredMemory) {
			t.Fatalf("%s actual memory lane diverged from memory delivery plan: lane=%#v plan=%#v", name, memoryLane, deliveryPlan)
		}
		if profile.mode == "off" {
			if trace["eligibility"] != "off" ||
				intFromAny(trace["budget_chars"], -1) != 0 ||
				boolFromAny(outputLane["applied"]) {
				t.Fatalf("eligible snapshot did not preserve guide OFF: trace=%#v output=%#v", trace, outputLane)
			}
			observations[name] = observation{
				sourceRefs:     observedSourceRefs,
				memoryLaneHash: memoryLane["content_hash"],
				outputHash:     outputLane["content_hash"],
			}
			continue
		}
		eligibility := mapFromAny(trace["guide_eligibility"])
		if trace["eligibility"] != "eligible" || eligibility["status"] != "eligible" {
			t.Fatalf("%s eligibility = %#v / %#v", name, trace["eligibility"], eligibility)
		}
		refs := stringSliceFromAny(eligibility["source_refs"])
		if len(refs) == 0 || !strings.HasPrefix(refs[0], "memory:") {
			t.Fatalf("%s has no source-backed guide eligibility: %#v", name, eligibility)
		}
		coverage := mapFromAny(eligibility["coverage"])
		roles := stringSliceFromAny(coverage["allowed_roles"])
		wantRoles := map[string][]string{
			"weak":   {"fidelity_warning"},
			"medium": {"fidelity_warning", "portrayal_note"},
			"strong": {"fidelity_warning", "portrayal_note"},
		}[profile.strength]
		if !reflect.DeepEqual(roles, wantRoles) {
			t.Errorf("%s roles = %#v, want %#v", name, roles, wantRoles)
		}
		itemKeys := []string{}
		for _, raw := range sliceFromAny(trace["items"]) {
			itemKeys = append(itemKeys, extractionStringFromAny(mapFromAny(raw)["key"]))
		}
		wantItemKeys := map[string][]string{
			"weak":   {"fidelity_preservation"},
			"medium": {"fidelity_preservation"},
			"strong": {"fidelity_preservation"},
		}[profile.strength]
		if !reflect.DeepEqual(itemKeys, wantItemKeys) {
			t.Errorf("%s actual guide items = %#v, want %#v", name, itemKeys, wantItemKeys)
		}
		if !boolFromAny(outputLane["applied"]) {
			t.Fatalf("%s missing applied output lane: %#v", name, outputLane)
		}
		outputText := extractionStringFromAny(outputLane["text"])
		for _, forbidden := range []string{"scene_mandate=", "required_outcome=", "forbidden_move=", "pacing=", "ending_requirement=", "[Progression Choice Ledger]"} {
			if strings.Contains(outputText, forbidden) {
				t.Errorf("%s injected story-composition directive %q: %q", name, forbidden, outputText)
			}
		}
		if !strings.Contains(outputText, "must_preserve=") ||
			strings.Contains(outputText, "must_respond=") ||
			strings.Contains(outputText, "[Progression Choice Ledger]") {
			t.Errorf("%s guide payload does not match its semantic coverage: %q", name, outputText)
		}
		evaluation := mapFromAny(prepareLineage["guide_efficacy_evaluation"])
		if evaluation["status"] != "awaiting_final_and_explicit_live_review" ||
			boolFromAny(evaluation["automatic_text_judgement"]) ||
			boolFromAny(evaluation["body_difference_is_efficacy_evidence"]) ||
			len(sliceFromAny(evaluation["item_results"])) == 0 {
			t.Errorf("%s efficacy evaluation is not fail-closed: %#v", name, evaluation)
		}
		for _, raw := range sliceFromAny(evaluation["item_results"]) {
			if mapFromAny(raw)["semantic_outcome"] != "unobserved" {
				t.Errorf("%s inferred item efficacy before live review: %#v", name, raw)
			}
		}
		observations[name] = observation{
			sourceRefs:     observedSourceRefs,
			memoryLaneHash: memoryLane["content_hash"],
			roles:          roles,
			outputHash:     outputLane["content_hash"],
		}
	}
	if !reflect.DeepEqual(observations["off"].sourceRefs, observations["weak"].sourceRefs) ||
		!reflect.DeepEqual(observations["weak"].sourceRefs, observations["medium"].sourceRefs) ||
		!reflect.DeepEqual(observations["medium"].sourceRefs, observations["strong"].sourceRefs) ||
		observations["off"].memoryLaneHash != observations["weak"].memoryLaneHash ||
		observations["weak"].memoryLaneHash != observations["medium"].memoryLaneHash ||
		observations["medium"].memoryLaneHash != observations["strong"].memoryLaneHash {
		t.Fatalf("guide matrix did not preserve the source snapshot: %#v", observations)
	}
	if observations["weak"].outputHash != observations["medium"].outputHash ||
		observations["medium"].outputHash != observations["strong"].outputHash {
		t.Fatalf("strength changed deterministic guide text without memory-backed supervisor output: %#v", observations)
	}
}

func TestOutputFidelity35CWeakGuideCallsConfiguredSupervisorOnce(t *testing.T) {
	var supervisorCalls atomic.Int64
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-supervisor","choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[{\"text\":\"preserve the supported recollection\",\"source_refs\":[\"memory:output-fidelity-35c-prepare:51\"]}],\"portrayal_notes\":[]}}"}}]}`))
	}))
	defer supervisor.Close()

	response := outputFidelity35CPrepareResponse(t, "ko_reencounter_supported_v1", "standard", "weak", supervisor.URL, true)
	_, trace := outputFidelity35CGuideTrace(t, response)
	if got := supervisorCalls.Load(); got != 1 || trace["supervisor_call_status"] != "applied" {
		t.Fatalf("configured weak supervisor calls=%d trace=%#v", got, trace)
	}
}

func TestResponseExecutionContractDoesNotControlStoryComposition(t *testing.T) {
	contract := buildResponseExecutionContract(
		nil, nil, nil, nil, nil, nil, prepareTurnInjectionAssembly{}, nil,
		dto.PrepareTurnCurrentInputDecisionV1{}, dto.PrepareTurnHostContextReferenceEvidenceV1{},
	)
	for _, key := range []string{"scene_mandate", "required_outcome", "forbidden_move", "pacing_pressure", "ending_requirement", "role_lens_consumption"} {
		if _, exists := contract[key]; exists {
			t.Fatalf("response execution contract controls story composition through %q: %#v", key, contract[key])
		}
	}
}

func TestOutputFidelity35CRendersMemoryExecutionItem(t *testing.T) {
	const memoryRef = "memory:output-fidelity-35c:77"
	contract := map[string]any{
		"active": true,
		"must_preserve": map[string]any{
			"items": []map[string]any{{
				"instruction": "Preserve the continuity facts carried by the delivered long-term-memory items.",
				"source_refs": []string{memoryRef},
			}},
		},
		"must_respond":    map[string]any{"items": []map[string]any{}},
		"must_account":    map[string]any{"items": []map[string]any{}},
		"must_not_assert": map[string]any{"items": []map[string]any{}},
		"source_refs": map[string]any{
			"memory": []string{memoryRef},
			"all":    []string{memoryRef},
		},
	}
	rendered := formatResponseExecutionFidelityGuidance(contract)
	if !strings.Contains(rendered, "delivered long-term-memory items") || !strings.Contains(rendered, memoryRef) {
		t.Fatalf("memory execution item was attributed in lineage but omitted from guide text: %q", rendered)
	}
}

func TestOutputFidelity35CInjectionDisabledHasZeroGuideWork(t *testing.T) {
	var supervisorCalls atomic.Int64
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		http.Error(w, "disabled injection must not call supervisor", http.StatusInternalServerError)
	}))
	defer supervisor.Close()

	response := outputFidelity35CPrepareResponseWithInjection(t, "ko_reencounter_supported_v1", "standard", "strong", supervisor.URL, true, false)
	plan, trace := outputFidelity35CGuideTrace(t, response)
	eligibility := mapFromAny(trace["guide_eligibility"])
	outputLane := outputFidelity35BFindLane(plan, "output_guidance")
	if trace["eligibility"] != "injection_disabled" ||
		eligibility["status"] != "injection_disabled" ||
		eligibility["reason_code"] != "payload_injection_disabled" {
		t.Fatalf("disabled injection eligibility = %#v / %#v", trace["eligibility"], eligibility)
	}
	if intFromAny(trace["budget_chars"], -1) != 0 ||
		intFromAny(trace["used_chars"], -1) != 0 ||
		intFromAny(trace["applied_count"], -1) != 0 ||
		len(sliceFromAny(trace["items"])) != 0 {
		t.Errorf("disabled injection generated guide work: %#v", trace)
	}
	if extractionStringFromAny(plan["auxiliary_text"]) != "" ||
		outputLane == nil ||
		boolFromAny(outputLane["applied"]) ||
		extractionStringFromAny(outputLane["text"]) != "" {
		t.Errorf("disabled injection exposed output guidance: plan=%#v lane=%#v", plan, outputLane)
	}
	evaluation := mapFromAny(plan["guide_efficacy_evaluation"])
	if evaluation["status"] != "not_applicable_injection_disabled" ||
		len(sliceFromAny(evaluation["item_results"])) != 0 {
		t.Errorf("disabled injection invented efficacy work: %#v", evaluation)
	}
	if got := supervisorCalls.Load(); got != 0 {
		t.Fatalf("disabled injection supervisor calls = %d, want 0", got)
	}
}

func TestOutputFidelity35CZeroOrInsufficientBudgetDoesNotCallSupervisor(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		budget             int
		wantEligibility    string
		wantTraceItemCount int
		wantEvaluation     string
	}{
		{name: "zero", budget: 0, wantEligibility: "budget_disabled", wantTraceItemCount: 0, wantEvaluation: "not_applicable_budget_disabled"},
		{name: "insufficient", budget: 1, wantEligibility: "eligible", wantTraceItemCount: 1, wantEvaluation: "not_applied_no_guidance_item"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var supervisorCalls atomic.Int64
			supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				supervisorCalls.Add(1)
				http.Error(w, "undeliverable guide must not call supervisor", http.StatusInternalServerError)
			}))
			defer supervisor.Close()

			response := outputFidelity35CPrepareResponseWithOptions(t, "ko_reencounter_supported_v1", "standard", "strong", supervisor.URL, true, true, testCase.budget)
			plan, trace := outputFidelity35CGuideTrace(t, response)
			outputLane := outputFidelity35BFindLane(plan, "output_guidance")
			if trace["eligibility"] != testCase.wantEligibility ||
				intFromAny(trace["used_chars"], -1) != 0 ||
				intFromAny(trace["applied_count"], -1) != 0 ||
				len(sliceFromAny(trace["items"])) != testCase.wantTraceItemCount {
				t.Errorf("budget %d generated deliverable guide work: %#v", testCase.budget, trace)
			}
			if outputLane == nil || boolFromAny(outputLane["applied"]) || extractionStringFromAny(outputLane["text"]) != "" {
				t.Errorf("budget %d exposed output guidance: %#v", testCase.budget, outputLane)
			}
			evaluation := mapFromAny(plan["guide_efficacy_evaluation"])
			if evaluation["status"] != testCase.wantEvaluation {
				t.Errorf("budget %d evaluation = %#v", testCase.budget, evaluation)
			}
			if got := supervisorCalls.Load(); got != 0 {
				t.Fatalf("budget %d supervisor calls = %d, want 0", testCase.budget, got)
			}
		})
	}
}

func TestOutputFidelity35CFinalTextDifferenceIsNotSemanticEvidence(t *testing.T) {
	build := func(generationID, finalText string) map[string]any {
		return buildSourceToFinalLineage(
			dto.M4CompleteTurnRequest{ClientMeta: map[string]any{
				"source_to_final_lineage_observation": outputFidelity35BCompleteObservation(generationID, prepareTurnTextHash("")),
			}},
			completeTurnSourceAcceptanceDecision{
				Enabled:  true,
				Accepted: true,
				Observation: completeTurnSourceObservation{
					GenerationID:        generationID,
					GenerationIDState:   "observed",
					ObservedContentHash: prepareOR1CHash(finalText),
					HashAlgorithm:       "or1c_utf16_djb2.v1",
				},
			},
		)
	}
	first := build("gen-35c-a", "first final text")
	second := build("gen-35c-b", "different final text")
	for index, lineage := range []map[string]any{first, second} {
		if lineage["semantic_outcome"] != "unobserved" ||
			mapFromAny(lineage["final_output"])["semantic_outcome"] != "unobserved" {
			t.Fatalf("lineage %d inferred efficacy from final text/hash: %#v", index, lineage)
		}
		evaluation := mapFromAny(lineage["guide_efficacy_evaluation"])
		if evaluation["status"] != "awaiting_explicit_live_review" ||
			evaluation["first_output_disposition"] != "unobserved" ||
			evaluation["reroll_reason"] != "unobserved" ||
			evaluation["active_final_display_observed"] != "unobserved" ||
			!boolFromAny(evaluation["active_final_message_observed"]) ||
			evaluation["guide_item_results_state"] != "unobserved_not_transmitted" ||
			len(sliceFromAny(evaluation["item_results"])) != 0 ||
			boolFromAny(evaluation["automatic_text_judgement"]) ||
			boolFromAny(evaluation["body_difference_is_efficacy_evidence"]) {
			t.Fatalf("lineage %d inferred adoption or efficacy from final text/hash: %#v", index, evaluation)
		}
		for _, raw := range sliceFromAny(lineage["item_results"]) {
			if mapFromAny(raw)["semantic_outcome"] != "unobserved" {
				t.Fatalf("lineage %d inferred item result: %#v", index, raw)
			}
		}
	}
}
