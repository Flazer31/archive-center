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
	return outputFidelity36FPrepareResponseWithBudgets(t, caseID, guideMode, guideStrength, supervisorEndpoint, withMemorySupport, injectionEnabled, 9000, narrativeSupportMaxChars)
}

func outputFidelity36FPrepareResponseWithBudgets(t *testing.T, caseID, guideMode, guideStrength, supervisorEndpoint string, withMemorySupport, injectionEnabled bool, maxInjectionChars, narrativeSupportMaxChars int) map[string]any {
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
			"max_injection_chars":   maxInjectionChars,
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

func outputFidelity36FFindLane(plan map[string]any, key string) map[string]any {
	for _, raw := range outputFidelityLineageSlice(plan["lanes"]) {
		lane := mapFromAny(raw)
		if extractionStringFromAny(lane["key"]) == key {
			return lane
		}
	}
	return nil
}

func TestOutputFidelity36FCurrentInputOnlyCallsExpressionSupervisor(t *testing.T) {
	var supervisorCalls atomic.Int64
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-supervisor","choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[],\"expression_hints\":[{\"kind\":\"response_focus\",\"text\":\"Keep the current request perceptible.\",\"source_refs\":[\"active:1\"]}]}}"}}]}`))
	}))
	defer supervisor.Close()

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
			hostEvidence := mapFromAny(response["host_context_reference_evidence"])
			if intFromAny(hostEvidence["selected_count"], 0) != 1 {
				t.Errorf("test did not exercise the official system-message shape: %#v", hostEvidence)
			}
			if profile.mode == "off" {
				if trace["eligibility"] != "off" ||
					intFromAny(trace["budget_chars"], -1) != 0 ||
					len(sliceFromAny(trace["items"])) != 0 ||
					lane == nil || boolFromAny(lane["applied"]) {
					t.Fatalf("guide OFF changed payload parity: trace=%#v lane=%#v", trace, lane)
				}
				return
			}
			eligibility := mapFromAny(trace["guide_eligibility"])
			lanes := mapFromAny(eligibility["lanes"])
			if trace["eligibility"] != "eligible" ||
				boolFromAny(mapFromAny(lanes["fidelity"])["eligible"]) ||
				!boolFromAny(mapFromAny(lanes["expression"])["eligible"]) ||
				trace["supervisor_call_status"] != "applied" {
				t.Fatalf("current-input-only expression lane was not applied: eligibility=%#v trace=%#v", eligibility, trace)
			}
			if lane == nil || !boolFromAny(lane["applied"]) ||
				!strings.Contains(extractionStringFromAny(lane["text"]), "Keep the current request perceptible.") {
				t.Fatalf("current-input-only expression was not delivered: %#v", lane)
			}
			support := mapFromAny(mapFromAny(response["supervisor_input_pack"])["support_packet"])
			currentSupport := mapFromAny(support["current_input"])
			if currentSupport["source_ref"] != "active:1" ||
				extractionStringFromAny(currentSupport["raw_text"]) == "" ||
				intFromAny(support["delivered_memory_count"], -1) != 0 {
				t.Fatalf("current-input-only support packet = %#v", support)
			}
		})
	}
	if got := supervisorCalls.Load(); got != 3 {
		t.Fatalf("current-input-only supervisor calls = %d, want 3 (OFF must skip)", got)
	}
}

func TestOutputFidelity36FFreshTurnZeroMemoryBudgetStillCallsExpressionSupervisor(t *testing.T) {
	var supervisorCalls atomic.Int64
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-supervisor","choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[],\"expression_hints\":[{\"kind\":\"response_focus\",\"text\":\"Keep the fresh-turn request perceptible.\",\"source_refs\":[\"active:1\"]}]}}"}}]}`))
	}))
	defer supervisor.Close()

	response := outputFidelity36FPrepareResponseWithBudgets(
		t,
		"ko_reencounter_no_support_v1",
		"standard",
		"weak",
		supervisor.URL,
		false,
		true,
		0,
		3000,
	)
	plan, trace := outputFidelity35CGuideTrace(t, response)
	eligibility := mapFromAny(trace["guide_eligibility"])
	lanes := mapFromAny(eligibility["lanes"])
	if trace["eligibility"] != "eligible" ||
		!boolFromAny(mapFromAny(lanes["expression"])["eligible"]) ||
		boolFromAny(mapFromAny(lanes["fidelity"])["eligible"]) ||
		trace["supervisor_call_status"] != "applied" ||
		supervisorCalls.Load() != 1 {
		t.Fatalf("fresh-turn expression guide was blocked by zero memory budget: eligibility=%#v trace=%#v calls=%d", eligibility, trace, supervisorCalls.Load())
	}
	outputLane := outputFidelity35BFindLane(plan, "output_guidance")
	if outputLane == nil || !boolFromAny(outputLane["applied"]) ||
		!strings.Contains(extractionStringFromAny(outputLane["text"]), "fresh-turn request") {
		t.Fatalf("fresh-turn expression guidance missing: %#v", outputLane)
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
		lanes := mapFromAny(eligibility["lanes"])
		fidelityLane := mapFromAny(lanes["fidelity"])
		expressionLane := mapFromAny(lanes["expression"])
		refs := stringSliceFromAny(fidelityLane["source_refs"])
		if !boolFromAny(fidelityLane["eligible"]) || !boolFromAny(expressionLane["eligible"]) ||
			len(refs) == 0 || !strings.HasPrefix(refs[0], "memory:") {
			t.Fatalf("%s has no delivered-memory fidelity lane: %#v", name, eligibility)
		}
		coverage := mapFromAny(eligibility["coverage"])
		roles := stringSliceFromAny(coverage["allowed_roles"])
		wantRoles := map[string][]string{
			"weak":   {"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not"},
			"medium": {"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not", "pacing", "scene_emphasis", "may_advance", "hold_allowed"},
			"strong": {"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not", "pacing", "scene_emphasis", "may_advance", "hold_allowed", "arc_anchor", "preferred_frontier", "reversible_option", "ending_edge"},
		}[profile.strength]
		if !reflect.DeepEqual(roles, wantRoles) {
			t.Errorf("%s roles = %#v, want %#v", name, roles, wantRoles)
		}
		itemKeys := []string{}
		for _, raw := range sliceFromAny(trace["items"]) {
			itemKeys = append(itemKeys, extractionStringFromAny(mapFromAny(raw)["key"]))
		}
		wantItemKeys := []string{}
		if !reflect.DeepEqual(itemKeys, wantItemKeys) {
			t.Errorf("%s actual guide items = %#v, want %#v", name, itemKeys, wantItemKeys)
		}
		if boolFromAny(outputLane["applied"]) || extractionStringFromAny(outputLane["text"]) != "" {
			t.Fatalf("%s injected duplicate deterministic fidelity guidance without a supervisor result: %#v", name, outputLane)
		}
		outputText := extractionStringFromAny(outputLane["text"])
		for _, forbidden := range []string{"scene_mandate=", "required_outcome=", "forbidden_move=", "pacing=", "ending_requirement=", "[Progression Choice Ledger]", "memory:"} {
			if strings.Contains(outputText, forbidden) {
				t.Errorf("%s injected story-composition directive %q: %q", name, forbidden, outputText)
			}
		}
		evaluation := mapFromAny(prepareLineage["guide_efficacy_evaluation"])
		if evaluation["status"] != "not_applied_no_guidance_item" ||
			boolFromAny(evaluation["automatic_text_judgement"]) ||
			boolFromAny(evaluation["body_difference_is_efficacy_evidence"]) ||
			len(sliceFromAny(evaluation["item_results"])) != 0 {
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
	if reflect.DeepEqual(observations["weak"].roles, observations["medium"].roles) ||
		reflect.DeepEqual(observations["medium"].roles, observations["strong"].roles) ||
		observations["weak"].outputHash != observations["medium"].outputHash ||
		observations["medium"].outputHash != observations["strong"].outputHash {
		t.Fatalf("strength coverage or no-result payload parity is wrong: %#v", observations)
	}
}

func TestOutputFidelity35CWeakGuideCallsConfiguredSupervisorOnce(t *testing.T) {
	var supervisorCalls atomic.Int64
	var capturedPrompt atomic.Value
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		supervisorCalls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode supervisor request: %v", err)
		} else {
			messages := sliceFromAny(body["messages"])
			if len(messages) > 0 {
				capturedPrompt.Store(extractionStringFromAny(mapFromAny(messages[len(messages)-1])["content"]))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"test-supervisor","choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[{\"text\":\"preserve the supported recollection\",\"source_refs\":[\"memory:output-fidelity-35c-prepare:51\"]}],\"expression_hints\":[{\"kind\":\"response_focus\",\"text\":\"keep the current request perceptible\",\"source_refs\":[\"active:1\"]}]}}"}}]}`))
	}))
	defer supervisor.Close()

	response := outputFidelity35CPrepareResponse(t, "ko_reencounter_supported_v1", "standard", "weak", supervisor.URL, true)
	_, trace := outputFidelity35CGuideTrace(t, response)
	if got := supervisorCalls.Load(); got != 1 || trace["supervisor_call_status"] != "applied" {
		t.Fatalf("configured weak supervisor calls=%d trace=%#v", got, trace)
	}
	support := mapFromAny(mapFromAny(response["supervisor_input_pack"])["support_packet"])
	delivered := anySliceFromAny(support["delivered_memory"])
	if len(delivered) != 1 {
		t.Fatalf("delivered support packet = %#v", support)
	}
	deliveredItem := mapFromAny(delivered[0])
	prompt, _ := capturedPrompt.Load().(string)
	var promptPayload map[string]any
	if err := json.Unmarshal([]byte(prompt), &promptPayload); err != nil {
		t.Fatalf("decode captured supervisor payload: %v\n%s", err, prompt)
	}
	if _, leaked := promptPayload["context_messages"]; leaked ||
		strings.Contains(prompt, "Stay in character and follow the current chat context.") {
		t.Fatalf("supervisor prompt leaked unbounded request context: %s", prompt)
	}
	promptSupport := mapFromAny(promptPayload["supervisor_support_packet"])
	promptDelivered := anySliceFromAny(promptSupport["delivered_memory"])
	if extractionStringFromAny(mapFromAny(promptSupport["current_input"])["raw_text"]) != extractionStringFromAny(mapFromAny(support["current_input"])["raw_text"]) ||
		len(promptDelivered) != 1 ||
		extractionStringFromAny(mapFromAny(promptDelivered[0])["final_text"]) != extractionStringFromAny(deliveredItem["final_text"]) ||
		extractionStringFromAny(mapFromAny(promptDelivered[0])["source_ref"]) != extractionStringFromAny(deliveredItem["source_ref"]) {
		t.Fatalf("supervisor prompt support packet mismatch: response=%#v prompt=%#v", support, promptSupport)
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

func TestOutputFidelity36FVisibleGuideOmitsRefsWhileTraceRetainsThem(t *testing.T) {
	const memoryRef = "memory:output-fidelity-35c:77"
	pack := supervisorBoundaryTestPack("weak")
	contractRefs := mapFromAny(mapFromAny(pack["response_execution_contract"])["source_refs"])
	contractRefs["memory"] = []string{memoryRef}
	contractRefs["all"] = []string{"input:latest", memoryRef}
	mapFromAny(pack["support_packet"])["delivered_memory"] = []map[string]any{{"source_ref": memoryRef, "final_text": "delivered safe memory"}}
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{map[string]any{"text": "Preserve the delivered recollection.", "source_refs": []any{memoryRef}}},
		},
	}, pack)
	plan := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 3000, supervisorSceneProposalGuidanceItems(result), "applied")
	lane := outputFidelity36FFindLane(plan, "output_guidance")
	visible := extractionStringFromAny(lane["text"])
	if !strings.Contains(visible, "Preserve the delivered recollection.") || strings.Contains(visible, memoryRef) {
		t.Fatalf("visible guide text leaked or omitted source support: %q", visible)
	}
	traceItems := outputFidelityLineageSlice(mapFromAny(plan["guidance_application_trace"])["items"])
	if len(traceItems) != 1 || !reflect.DeepEqual(stringSliceFromAny(mapFromAny(traceItems[0])["source_refs"]), []string{memoryRef}) {
		t.Fatalf("structured trace lost exact source ref: %#v", traceItems)
	}
}

func TestOutputFidelity36FOversizedGuidanceDoesNotBlockLaterSmallItem(t *testing.T) {
	items := []prepareTurnGuidanceItem{
		{Key: "oversized", Text: strings.Repeat("x", 200), SourceRefs: []string{"input:latest"}},
		{Key: "small", Text: "optional", SourceRefs: []string{"input:latest"}},
	}
	plan := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 20, items, "applied")
	trace := mapFromAny(plan["guidance_application_trace"])
	traceItems := outputFidelityLineageSlice(trace["items"])
	if intFromAny(trace["applied_count"], 0) != 1 ||
		intFromAny(trace["deferred_count"], 0) != 1 ||
		len(traceItems) != 2 ||
		mapFromAny(traceItems[0])["status"] != "deferred" ||
		mapFromAny(traceItems[1])["status"] != "applied" ||
		extractionStringFromAny(trace["final_text"]) != "optional" {
		t.Fatalf("item-by-item guidance packing failed: %#v", trace)
	}
}

func TestOutputFidelity36FManyHostRefsDoNotConsumeVisibleNarrativeBudget(t *testing.T) {
	hostRefs := make([]string, 0, 500)
	allRefs := []string{"input:latest", "memory:delivered"}
	for index := 0; index < 500; index++ {
		ref := fmt.Sprintf("system:host:%d", index)
		hostRefs = append(hostRefs, ref)
		allRefs = append(allRefs, ref)
	}
	pack := supervisorBoundaryTestPack("weak")
	contract := mapFromAny(pack["response_execution_contract"])
	contract["source_refs"] = map[string]any{
		"all":           allRefs,
		"current_input": []string{"input:latest"},
		"native_system": hostRefs,
		"memory":        []string{"memory:delivered"},
	}
	if ready, reason := supervisorExecutionContractReady(pack); !ready {
		t.Fatalf("many Host refs blocked supervisor readiness: %s", reason)
	}
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "response_focus", "text": "Keep the current request perceptible.", "source_refs": []any{"input:latest"}},
			},
		},
	}, pack)
	plan := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 3000, supervisorSceneProposalGuidanceItems(result), "applied")
	lane := outputFidelity36FFindLane(plan, "output_guidance")
	if !boolFromAny(lane["applied"]) || strings.Contains(extractionStringFromAny(lane["text"]), "system:host:") {
		t.Fatalf("Host ref repetition consumed or leaked into visible guide: %#v", lane)
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

func TestOutputFidelity36FZeroBudgetSkipsButPositiveBudgetDoesNotPreflightSkip(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		budget             int
		wantEligibility    string
		wantTraceItemCount int
		wantEvaluation     string
		wantCalls          int64
	}{
		{name: "zero", budget: 0, wantEligibility: "budget_disabled", wantTraceItemCount: 0, wantEvaluation: "not_applicable_budget_disabled", wantCalls: 0},
		{name: "positive_tiny", budget: 1, wantEligibility: "eligible", wantTraceItemCount: 1, wantEvaluation: "not_applied_no_guidance_item", wantCalls: 1},
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
			if got := supervisorCalls.Load(); got != testCase.wantCalls {
				t.Fatalf("budget %d supervisor calls = %d, want %d", testCase.budget, got, testCase.wantCalls)
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
