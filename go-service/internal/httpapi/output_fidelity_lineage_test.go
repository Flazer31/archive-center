package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const (
	outputFidelity35BPrepareLineageContract      = "source_to_payload_lineage.v1"
	outputFidelity35BCompleteObservationContract = "source_to_final_lineage_observation.v1"
	outputFidelity35BCompleteLineageContract     = "source_to_final_lineage.v1"
	outputFidelity35BCorrelationID               = "ac-correlation-output-fidelity-35b"
	outputFidelity35BPrepareLineageID            = "prepare-lineage-output-fidelity-35b"
	outputFidelity35BPayloadPlanID               = "payload-plan-output-fidelity-35b"
)

type outputFidelity35BPrepareFixture struct {
	Response        map[string]any
	PriorSource     outputFidelitySource
	CurrentSource   outputFidelitySource
	MemorySourceRef string
	MemoryRowID     int64
}

func outputFidelityCaseByID(t *testing.T, corpus outputFidelityCorpus, caseID string) outputFidelityCase {
	t.Helper()
	for _, fixture := range corpus.Cases {
		if fixture.CaseID == caseID {
			return fixture
		}
	}
	t.Fatalf("output-fidelity case %q not found", caseID)
	return outputFidelityCase{}
}

func outputFidelitySourceByLifecycle(t *testing.T, fixture outputFidelityCase, kind, lifecycle string) outputFidelitySource {
	t.Helper()
	for _, source := range fixture.EvidenceSources {
		if source.SourceKind == kind && source.SourceLifecycle == lifecycle {
			return source
		}
	}
	t.Fatalf("case %q has no %s/%s source", fixture.CaseID, kind, lifecycle)
	return outputFidelitySource{}
}

func prepareOutputFidelity35BFixture(t *testing.T, sourceSpanObserved bool) outputFidelity35BPrepareFixture {
	t.Helper()
	corpus, _ := loadOutputFidelityCorpus(t)
	fixture := outputFidelityCaseByID(t, corpus, "ko_reencounter_supported_v1")
	prior := outputFidelitySourceByLifecycle(t, fixture, "assistant_output", "active_final")
	current := outputFidelitySourceByLifecycle(t, fixture, "user_input", "current_observation")

	const (
		sid         = "output-fidelity-35b-prepare"
		memoryRowID = int64(41)
		sourceTurn  = 4
	)
	memorySourceRef := fmt.Sprintf("memory:%s:%d", sid, memoryRowID)
	summaryJSON, err := json.Marshal(map[string]any{
		"turn_summary": prior.ReplayText,
	})
	if err != nil {
		t.Fatal(err)
	}

	fake := &turnRecordingStore{
		returnMemories: []store.Memory{{
			ID:            memoryRowID,
			ChatSessionID: sid,
			TurnIndex:     sourceTurn,
			SummaryJSON:   string(summaryJSON),
			Importance:    0.9,
		}},
	}
	if sourceSpanObserved {
		fake.returnChatLogs = []store.ChatLog{
			{ID: 1, ChatSessionID: sid, TurnIndex: sourceTurn, Role: "user", Content: "B와 주인공은 돌다리에서 다시 만나기로 했다."},
			{ID: 2, ChatSessionID: sid, TurnIndex: sourceTurn, Role: "assistant", Content: prior.ReplayText},
		}
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 1, ModelReady: true},
		searchResults: []vector.VectorDocument{{
			ID:                  memorySourceRef,
			Tier:                "memory",
			ChatSessionID:       sid,
			SourceTable:         "memories",
			SourceRowID:         fmt.Sprint(memoryRowID),
			Similarity:          0.95,
			SimilarityAvailable: true,
			SimilaritySource:    "cosine_from_query_and_stored_embedding",
		}},
	}
	srv.VectorOpenError = nil

	body, err := json.Marshal(map[string]any{
		"chat_session_id":             sid,
		"turn_index":                  21,
		"raw_user_input":              current.ReplayText,
		"narrative_support_max_chars": 6000,
		"client_meta": map[string]any{
			"chroma_query_vector":                   []float64{0.4, 0.6},
			"archive_center_request_correlation_id": outputFidelity35BCorrelationID,
		},
		"settings": map[string]any{
			"apply_mode":            "shadow",
			"max_injection_chars":   9000,
			"injection_enabled":     true,
			"input_context_enabled": false,
			"top_k":                 1,
			"guide_mode":            "standard",
			"guide_strength":        "medium",
			"narrative_stance":      "balanced",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, response := prepareTurnPerfRequest(t, srv, string(body))
	return outputFidelity35BPrepareFixture{
		Response:        response,
		PriorSource:     prior,
		CurrentSource:   current,
		MemorySourceRef: memorySourceRef,
		MemoryRowID:     memoryRowID,
	}
}

func outputFidelity35BDeliveredMemoryItem(t *testing.T, response map[string]any, rowID int64) map[string]any {
	t.Helper()
	pack := mapFromAny(response["injection_pack"])
	lineage := mapFromAny(pack["memory_delivery_lineage"])
	if lineage["contract_version"] != "memory_delivery_lineage.v1" {
		t.Fatalf("memory delivery lineage missing: %#v", lineage)
	}
	for _, raw := range sliceFromAny(lineage["items"]) {
		item := mapFromAny(raw)
		if int64(intFromAny(item["source_row_id"], 0)) == rowID && boolFromAny(item["delivered"]) {
			return item
		}
	}
	t.Fatalf("memory row %d was not delivered: %#v", rowID, lineage)
	return nil
}

func outputFidelity35BContainsSourceRef(value any, sourceRef string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "source_refs" && stringSliceContains(stringSliceFromAny(child), sourceRef) {
				return true
			}
			if outputFidelity35BContainsSourceRef(child, sourceRef) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if outputFidelity35BContainsSourceRef(child, sourceRef) {
				return true
			}
		}
	case []map[string]any:
		for _, child := range typed {
			if outputFidelity35BContainsSourceRef(child, sourceRef) {
				return true
			}
		}
	}
	return false
}

func outputFidelity35BFindLane(plan map[string]any, key string) map[string]any {
	for _, raw := range sliceFromAny(plan["lanes"]) {
		lane := mapFromAny(raw)
		if extractionStringFromAny(lane["key"]) == key {
			return lane
		}
	}
	return nil
}

func TestOutputFidelity35BPrepareTurnLinksEligibleCorpusSourceToPayload(t *testing.T) {
	fixture := prepareOutputFidelity35BFixture(t, true)
	delivered := outputFidelity35BDeliveredMemoryItem(t, fixture.Response, fixture.MemoryRowID)
	memoryLineage := mapFromAny(mapFromAny(fixture.Response["injection_pack"])["memory_delivery_lineage"])
	if got := intFromAny(memoryLineage["top_k_memory_target"], 0); got != 1 {
		t.Fatalf("bounded lineage lost the Go-owned top_k target: got %d lineage=%#v", got, memoryLineage)
	}
	if boolFromAny(memoryLineage["source_text_exposed"]) || delivered["final_text"] != nil {
		t.Fatalf("bounded lineage exposed delivered source text: %#v", delivered)
	}

	sourceSpan := mapFromAny(delivered["source_span"])
	if sourceSpan["status"] != "partially_observed" ||
		sourceSpan["source_kind"] != fixture.PriorSource.SourceKind ||
		sourceSpan["source_lifecycle"] != "unobserved" ||
		sourceSpan["semantic_outcome"] != "unobserved" ||
		sourceSpan["link_precision"] != "turn_only" ||
		intFromAny(sourceSpan["turn_index"], 0) != 4 ||
		sourceSpan["content_hash"] != prepareTurnTextHash(fixture.PriorSource.ReplayText) {
		t.Errorf("delivered memory did not preserve the observed assistant turn without inferring lifecycle: %#v", sourceSpan)
	}

	executionContract := mapFromAny(fixture.Response["response_execution_contract"])
	if executionContract["contract_version"] != "response_execution_contract.v1" {
		t.Fatalf("response execution contract missing: %#v", executionContract)
	}
	if !stringSliceContains(stringSliceFromAny(mapFromAny(executionContract["source_refs"])["all"]), fixture.MemorySourceRef) {
		t.Errorf("execution contract source_refs omit delivered memory %q: %#v", fixture.MemorySourceRef, executionContract["source_refs"])
	}
	if !outputFidelity35BContainsSourceRef(executionContract, fixture.MemorySourceRef) {
		t.Errorf("no execution item is linked to delivered memory %q", fixture.MemorySourceRef)
	}

	plan := mapFromAny(fixture.Response["payload_application_plan"])
	if plan["contract_version"] != "payload_application_plan.v1" {
		t.Fatalf("payload application plan missing: %#v", plan)
	}
	supportPacket := mapFromAny(mapFromAny(fixture.Response["supervisor_input_pack"])["support_packet"])
	supportedMemory := false
	for _, raw := range outputFidelityLineageSlice(supportPacket["delivered_memory"]) {
		item := mapFromAny(raw)
		if item["source_ref"] == fixture.MemorySourceRef &&
			strings.TrimSpace(extractionStringFromAny(item["final_text"])) != "" {
			supportedMemory = true
			break
		}
	}
	if !supportedMemory {
		t.Errorf("supervisor support packet omitted delivered safe memory %q: %#v", fixture.MemorySourceRef, supportPacket)
	}
	trace := mapFromAny(plan["guidance_application_trace"])
	if outputFidelity35BContainsSourceRef(trace["items"], fixture.MemorySourceRef) {
		t.Errorf("deterministic output guidance duplicated delivered memory %q: %#v", fixture.MemorySourceRef, trace["items"])
	}
	if trace["final_hash"] != prepareTurnTextHash(extractionStringFromAny(trace["final_text"])) {
		t.Errorf("guidance final hash does not match its exact text: %#v", trace)
	}
	outputLane := outputFidelity35BFindLane(plan, "output_guidance")
	if outputLane == nil {
		t.Fatal("payload plan has no output_guidance lane")
	}
	if outputLane["content_hash"] != prepareTurnTextHash(extractionStringFromAny(outputLane["text"])) {
		t.Errorf("output guidance lane hash mismatch: %#v", outputLane)
	}
	if boolFromAny(outputLane["applied"]) || outputFidelity35BContainsSourceRef(outputLane, fixture.MemorySourceRef) {
		t.Errorf("output guidance duplicated delivered memory without a supervisor proposal %q: %#v", fixture.MemorySourceRef, outputLane)
	}
	if plan["auxiliary_hash"] != prepareTurnTextHash(extractionStringFromAny(plan["auxiliary_text"])) {
		t.Errorf("final auxiliary payload hash mismatch: %#v", plan)
	}

	prepareLineage := mapFromAny(fixture.Response["source_to_payload_lineage"])
	replayed := prepareOutputFidelity35BFixture(t, true)
	replayedLineage := mapFromAny(replayed.Response["source_to_payload_lineage"])
	lineageID := extractionStringFromAny(prepareLineage["lineage_id"])
	payloadPlanID := extractionStringFromAny(prepareLineage["payload_plan_id"])
	if prepareLineage["contract_version"] != outputFidelity35BPrepareLineageContract ||
		prepareLineage["status"] != "observed" ||
		prepareLineage["archive_center_request_correlation_id"] != outputFidelity35BCorrelationID ||
		lineageID == "" ||
		payloadPlanID == "" ||
		!strings.HasPrefix(lineageID, "stl_") ||
		!strings.HasPrefix(payloadPlanID, "stp_") ||
		replayedLineage["lineage_id"] != lineageID ||
		replayedLineage["payload_plan_id"] != payloadPlanID ||
		plan["payload_plan_id"] != payloadPlanID ||
		trace["payload_plan_id"] != payloadPlanID ||
		trace["prepare_lineage_id"] != lineageID ||
		prepareLineage["official_risu_request_id_state"] != "official_risu_request_id_not_exposed" ||
		!outputFidelity35BContainsSourceRef(prepareLineage, fixture.MemorySourceRef) {
		t.Errorf("source-to-payload lineage is missing or detached: %#v", prepareLineage)
	}
}

func TestOutputFidelity35BMissingSourceSpanRemainsUnobserved(t *testing.T) {
	fixture := prepareOutputFidelity35BFixture(t, false)
	delivered := outputFidelity35BDeliveredMemoryItem(t, fixture.Response, fixture.MemoryRowID)
	sourceSpan := mapFromAny(delivered["source_span"])
	if sourceSpan["status"] != "unobserved" ||
		sourceSpan["reason_code"] != "source_span_not_observed" ||
		sourceSpan["source_kind"] != "unobserved" ||
		sourceSpan["source_lifecycle"] != "unobserved" ||
		sourceSpan["semantic_outcome"] != "unobserved" {
		t.Fatalf("missing source span was not preserved as unobserved: %#v", sourceSpan)
	}
	if sourceSpan["source_kind"] == "assistant_output" ||
		sourceSpan["source_lifecycle"] == "active_final" ||
		sourceSpan["semantic_outcome"] == "fulfilled" ||
		sourceSpan["link_precision"] == "exact_message" {
		t.Fatalf("unobserved source span gained inferred authority or success: %#v", sourceSpan)
	}
}

func outputFidelity35BCompleteObservation(generationID, payloadGuidanceHash string) map[string]any {
	return map[string]any{
		"contract_version":                      outputFidelity35BCompleteObservationContract,
		"status":                                "ready",
		"archive_center_request_correlation_id": outputFidelity35BCorrelationID,
		"prepare_lineage_id":                    outputFidelity35BPrepareLineageID,
		"payload_plan_id":                       outputFidelity35BPayloadPlanID,
		"generation_id":                         generationID,
		"generation_id_state":                   "observed",
		"request_id_state":                      "official_risu_request_id_not_exposed",
		"payload_application_status":            "applied",
		"payload_observation_stage":             "archive_center_before_request_return",
		"final_provider_payload_state":          "not_exposed",
		"source_refs":                           []string{"memory:output-fidelity-35b-prepare:41"},
		"execution_item_refs":                   []string{"response_execution_contract.must_account[0]"},
		"payload_guidance_hash":                 payloadGuidanceHash,
		"hash_algorithm":                        "sha256_utf8.v1",
	}
}

func serveOutputFidelity35BCompleteTurn(t *testing.T, server *Server, request dto.M4CompleteTurnRequest) map[string]any {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleCompleteTurn(recorder, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("complete-turn status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode complete-turn response: %v", err)
	}
	return response
}

func requireOutputFidelity35BUnattached(t *testing.T, response map[string]any, reason string) {
	t.Helper()
	lineage := mapFromAny(response["source_to_final_lineage"])
	if lineage["contract_version"] != outputFidelity35BCompleteLineageContract ||
		boolFromAny(lineage["attached"]) ||
		lineage["semantic_outcome"] != "unobserved" ||
		lineage["reason_code"] != reason {
		t.Fatalf("ambiguous final lineage was not explicitly left unattached: want reason=%q got=%#v", reason, lineage)
	}
}

func TestOutputFidelity35BCompleteTurnCorrelatesMatchingActiveFinal(t *testing.T) {
	const generationID = "generation-final-observed"
	finalOutput := "B는 익숙한 듯 고개를 끄덕이며 돌다리에서의 약속을 먼저 꺼냈다."
	request := completeTurnAnchoredAcceptanceTestRequest(
		"output-fidelity-35b-complete",
		21,
		"나는 약속한 돌다리에서 B의 이름을 불렀다.",
		finalOutput,
		2000,
		generationID,
		"not_streaming",
		40,
		41,
		42,
	)
	lineageObservation := outputFidelity35BCompleteObservation(
		generationID,
		prepareTurnTextHash("[Output Guidance]\nlinked guidance"),
	)
	lineageObservation["memory_injection_baseline_id"] = "mib_test"
	lineageObservation["surface_payload_application"] = []any{
		map[string]any{"surface": "memory", "status": "applied", "rendered_count": 2, "payload_character_count": 42, "displayed_effect": "unobserved"},
	}
	request.ClientMeta["source_to_final_lineage_observation"] = lineageObservation
	response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
	acceptance := mapFromAny(response["source_acceptance"])
	if !boolFromAny(acceptance["accepted"]) || acceptance["lifecycle"] != "active_final" {
		t.Fatalf("fixture did not reach accepted active final: %#v", acceptance)
	}

	lineage := mapFromAny(response["source_to_final_lineage"])
	finalObservation := mapFromAny(lineage["final_output"])
	if lineage["contract_version"] != outputFidelity35BCompleteLineageContract ||
		lineage["status"] != "attached" ||
		!boolFromAny(lineage["attached"]) ||
		lineage["archive_center_request_correlation_id"] != outputFidelity35BCorrelationID ||
		lineage["prepare_lineage_id"] != outputFidelity35BPrepareLineageID ||
		lineage["payload_plan_id"] != outputFidelity35BPayloadPlanID ||
		lineage["request_id_state"] != "official_risu_request_id_not_exposed" ||
		lineage["generation_id"] != generationID ||
		lineage["semantic_outcome"] != "unobserved" ||
		finalObservation["lifecycle"] != "active_final" ||
		finalObservation["content_hash"] != prepareOR1CHash(finalOutput) {
		t.Fatalf("accepted active final was not correlated without inventing semantic success: %#v", lineage)
	}
	if outputFidelity35BContainsSourceRef(lineage, "") {
		t.Fatal("lineage contains an empty source reference")
	}
	surfacePayload := outputFidelityLineageSlice(lineage["surface_payload_application"])
	if lineage["memory_injection_baseline_id"] != "mib_test" || len(surfacePayload) != 1 || mapFromAny(surfacePayload[0])["status"] != "applied" || mapFromAny(lineage["displayed_effect"])["status"] != "unobserved" {
		t.Fatalf("payload-applied/displayed-effect stages were conflated: %#v", lineage)
	}
}

func TestOutputFidelity35BDoesNotAttachAmbiguousFinal(t *testing.T) {
	t.Run("streaming", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-streaming", 21, "user", "partial", 2000,
			"generation-streaming", "streaming", 40, 41, 42,
		)
		request.ClientMeta["source_to_final_lineage_observation"] = outputFidelity35BCompleteObservation(
			"generation-streaming",
			prepareTurnTextHash("streaming guidance"),
		)
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		if response["code"] != "source_acceptance_streaming_candidate" {
			t.Fatalf("streaming fixture did not exercise production rejection: %#v", response)
		}
		requireOutputFidelity35BUnattached(t, response, "source_acceptance_streaming_candidate")
	})

	t.Run("superseded", func(t *testing.T) {
		server := newCompleteTurnAcceptanceTestServer()
		newer := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-superseded", 21, "user", "newer", 3000,
			"generation-newer", "not_streaming", 40, 41, 42,
		)
		if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), newer); !decision.Accepted {
			t.Fatalf("newer fixture was not accepted: %+v", decision)
		}
		older := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-superseded", 21, "user", "older", 2000,
			"generation-older", "not_streaming", 40, 41, 42,
		)
		older.ClientMeta["source_to_final_lineage_observation"] = outputFidelity35BCompleteObservation(
			"generation-older",
			prepareTurnTextHash("older guidance"),
		)
		response := serveOutputFidelity35BCompleteTurn(t, server, older)
		if response["code"] != "source_acceptance_stale_or_superseded" {
			t.Fatalf("superseded fixture did not exercise production rejection: %#v", response)
		}
		requireOutputFidelity35BUnattached(t, response, "source_acceptance_stale_or_superseded")
	})

	t.Run("generation_mismatch", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-mismatch", 21, "user", "final", 2000,
			"generation-observed", "not_streaming", 40, 41, 42,
		)
		request.ClientMeta["source_to_final_lineage_observation"] = outputFidelity35BCompleteObservation(
			"generation-other",
			prepareTurnTextHash("mismatched guidance"),
		)
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		acceptance := mapFromAny(response["source_acceptance"])
		if !boolFromAny(acceptance["accepted"]) {
			t.Fatalf("generation mismatch fixture must isolate lineage from active-final acceptance: %#v", response)
		}
		requireOutputFidelity35BUnattached(t, response, "source_to_final_generation_mismatch")
	})

	t.Run("legacy_correlation_missing", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-legacy", 21, "user", "final", 2000,
			"generation-legacy", "not_streaming", 40, 41, 42,
		)
		delete(request.ClientMeta, "source_to_final_lineage_observation")
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		acceptance := mapFromAny(response["source_acceptance"])
		if !boolFromAny(acceptance["accepted"]) {
			t.Fatalf("legacy fixture must isolate missing correlation from active-final acceptance: %#v", response)
		}
		requireOutputFidelity35BUnattached(t, response, "source_to_final_correlation_missing")
	})

	t.Run("observation_status_missing", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-status-missing", 21, "user", "final", 2000,
			"generation-status-missing", "not_streaming", 40, 41, 42,
		)
		observation := outputFidelity35BCompleteObservation(
			"generation-status-missing",
			prepareTurnTextHash("status missing guidance"),
		)
		delete(observation, "status")
		request.ClientMeta["source_to_final_lineage_observation"] = observation
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		requireOutputFidelity35BUnattached(t, response, "source_to_final_observation_not_ready")
	})

	t.Run("payload_application_status_missing", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-payload-status-missing", 21, "user", "final", 2000,
			"generation-payload-status-missing", "not_streaming", 40, 41, 42,
		)
		observation := outputFidelity35BCompleteObservation(
			"generation-payload-status-missing",
			prepareTurnTextHash("payload status missing guidance"),
		)
		delete(observation, "payload_application_status")
		request.ClientMeta["source_to_final_lineage_observation"] = observation
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		requireOutputFidelity35BUnattached(t, response, "source_to_final_payload_application_unobserved")
	})

	t.Run("payload_observation_stage_missing", func(t *testing.T) {
		request := completeTurnAnchoredAcceptanceTestRequest(
			"output-fidelity-35b-stage-missing", 21, "user", "final", 2000,
			"generation-stage-missing", "not_streaming", 40, 41, 42,
		)
		observation := outputFidelity35BCompleteObservation(
			"generation-stage-missing",
			prepareTurnTextHash("stage missing guidance"),
		)
		delete(observation, "payload_observation_stage")
		request.ClientMeta["source_to_final_lineage_observation"] = observation
		response := serveOutputFidelity35BCompleteTurn(t, newCompleteTurnAcceptanceTestServer(), request)
		requireOutputFidelity35BUnattached(t, response, "source_to_final_payload_stage_unobserved")
	})
}

func TestMemoryInjectionBaseline41SeparatesStagesAndOnlyReportsDuplicateCandidates(t *testing.T) {
	const sid = "baseline-41"
	assembly := prepareTurnInjectionAssembly{
		MemoryText:                "same fact",
		ActualMemoryText:          "same fact",
		DirectEvidenceText:        "same fact",
		KGText:                    "Alice knows Bob",
		WorldRulesText:            "doors stay locked",
		CharacterText:             "Alice is alert",
		CharacterRelationshipText: "Alice trusts Bob",
		PersonaText:               "I remember the garden",
		StorylineText:             "The manor mystery",
		PendingThreadText:         "Who sent the letter?",
		EpisodeText:               "They reached the manor",
		Blocks: []prepareTurnInjectionBlock{
			{Label: "memory", Count: 1, Text: "same fact"},
			{Label: "direct_evidence", Count: 1, Text: "same fact"},
			{Label: "kg", Count: 1, Text: "Alice knows Bob"},
			{Label: "world_rules", Count: 1, Text: "doors stay locked"},
			{Label: "character", Count: 1, Text: "Alice is alert"},
			{Label: "persona_recollection", Count: 1, Text: "I remember the garden"},
			{Label: "storyline", Count: 1, Text: "The manor mystery"},
			{Label: "pending_thread", Count: 1, Text: "Who sent the letter?"},
			{Label: "episode", Count: 1, Text: "They reached the manor"},
		},
		MemoryDeliveryPlan: map[string]any{
			"classes": []any{
				map[string]any{"text": "[Recent Events]\nsame fact\nThey reached the manor"},
				map[string]any{"text": "[Direct Evidence]\nsame fact"},
				map[string]any{"text": "[Subjective and Relationship]\nAlice knows Bob\nI remember the garden\nAlice trusts Bob"},
				map[string]any{"text": "[World State]\ndoors stay locked"},
				map[string]any{"text": "[Unresolved Goals]\nThe manor mystery\nWho sent the letter?"},
			},
		},
	}
	lineage := map[string]any{
		"final_delivered_count": 1,
		"items": []any{
			map[string]any{"source_ref": "memory:baseline-41:1", "selection_lane": "actual", "delivered": true},
			map[string]any{"source_ref": "memory:baseline-41:1", "selection_lane": "protected", "delivered": true},
		},
	}
	baseline := buildMemoryInjectionBaseline41(
		sid, "request-41", "same fact",
		[]store.Memory{{ID: 1, ChatSessionID: sid, SummaryJSON: `{"turn_summary":"same fact"}`}},
		[]store.DirectEvidence{{ID: 2, ChatSessionID: sid, EvidenceText: "same fact"}},
		[]store.KGTriple{{ID: 3, ChatSessionID: sid, Subject: "Alice", Predicate: "knows", Object: "Bob"}},
		[]store.WorldRule{{ID: 4, ChatSessionID: sid, Key: "doors", ValueJSON: `"locked"`}},
		[]store.CharacterState{{ID: 5, ChatSessionID: sid, CharacterName: "Alice", StatusJSON: `{"alert":true}`, RelationshipsJSON: `{"Bob":"trust"}`}},
		[]store.ActiveState{{ID: 6, ChatSessionID: sid, Content: "night"}},
		[]store.CanonicalStateLayer{{ID: 7, ChatSessionID: sid, Content: "manor"}},
		[]store.PersonaMemoryEntry{{ID: 8, MemoryText: "I remember the garden"}}, nil,
		[]store.Storyline{{ID: 9, ChatSessionID: sid, Name: "Manor", CurrentContext: "mystery"}},
		[]store.PendingThread{{ID: 10, ChatSessionID: sid, Description: "Who sent the letter?"}},
		[]store.EpisodeSummary{{ID: 11, ChatSessionID: sid, SummaryText: "They reached the manor"}},
		[]store.ChatLog{{ID: 12, ChatSessionID: sid, Role: "user", Content: "same fact"}},
		"[Reversible State]\nnight", assembly, lineage,
	)
	if baseline["contract_version"] != "memory_injection_baseline.v1" || baseline["policy_mode"] != "observation_only_4_1" || baseline["canonical_mutation"] != false || baseline["vector_mutation"] != false {
		t.Fatalf("baseline contract=%#v", baseline)
	}
	if surfaces := outputFidelityLineageSlice(baseline["surfaces"]); len(surfaces) != 9 {
		t.Fatalf("surface count=%d baseline=%#v", len(surfaces), baseline)
	} else if memorySurface := mapFromAny(surfaces[0]); intFromAny(memorySurface["selected_count"], 0) != 1 || intFromAny(memorySurface["rendered_count"], 0) != 1 || intFromAny(memorySurface["payload_character_count"], 0) == 0 {
		t.Fatalf("memory selected/rendered/payload counts=%#v", memorySurface)
	}
	if len(outputFidelityLineageSlice(baseline["exact_same_row_lane_duplicates"])) != 1 ||
		len(outputFidelityLineageSlice(baseline["semantic_duplicate_candidates"])) == 0 ||
		len(outputFidelityLineageSlice(baseline["current_input_or_recent_context_duplicate_candidates"])) < 2 {
		t.Fatalf("duplicate observations missing: %#v", baseline)
	}
	if mapFromAny(baseline["payload_application"])["status"] != "planned_unobserved" || mapFromAny(baseline["displayed_effect"])["status"] != "unobserved" {
		t.Fatalf("selected/rendered were conflated with live stages: %#v", baseline)
	}
}
