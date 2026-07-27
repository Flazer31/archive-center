package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestTurnWorkflowHUDBeginOrderAndSameSessionInvalidation(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	first := ledger.begin("request-1", "session-a", 7)
	if first == nil || first.Status != "running" || first.LogicalTurn != 7 {
		t.Fatalf("first workflow = %#v", first)
	}
	if len(first.Stages) != len(turnWorkflowHUDStageTemplates) {
		t.Fatalf("stage count = %d, want %d", len(first.Stages), len(turnWorkflowHUDStageTemplates))
	}
	for index, stage := range first.Stages {
		if stage.Ordinal != index+1 || stage.Total != len(first.Stages) || stage.Key != turnWorkflowHUDStageTemplates[index].Key {
			t.Fatalf("stage[%d] = %#v", index, stage)
		}
	}

	second := ledger.begin("request-2", "session-a", 7)
	if second == nil || second.Attempt != 2 {
		t.Fatalf("second workflow = %#v", second)
	}
	invalidated, ok := ledger.snapshot("request-1")
	if !ok || invalidated.Status != "invalidated" || invalidated.Severity != "warning" {
		t.Fatalf("invalidated workflow = %#v, found=%t", invalidated, ok)
	}
	if invalidated.CurrentStage == nil || invalidated.CurrentStage.Status != "invalidated" {
		t.Fatalf("invalidated current stage = %#v", invalidated.CurrentStage)
	}
}

func TestTurnWorkflowHUDAttemptIsScopedToLogicalTurnAndSupersedesCompletedAttempt(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	first := ledger.begin("request-first", "session-a", 0)
	if first == nil || first.Attempt != 1 || first.LogicalTurn != 0 {
		t.Fatalf("provisional first workflow = %#v", first)
	}
	ledger.setLogicalTurn("request-first", 7)
	ledger.complete("request-first")

	second := ledger.begin("request-second", "session-a", 0)
	ledger.setLogicalTurn("request-second", 7)
	secondView, ok := ledger.snapshot("request-second")
	if !ok || second == nil || secondView.Attempt != 2 || secondView.LogicalTurn != 7 {
		t.Fatalf("same-turn reroll workflow = %#v, found=%t", secondView, ok)
	}
	superseded, ok := ledger.snapshot("request-first")
	if !ok || superseded.Status != "invalidated" || superseded.CurrentStage == nil || superseded.CurrentStage.Status != "invalidated" {
		t.Fatalf("superseded completed workflow = %#v, found=%t", superseded, ok)
	}

	ledger.complete("request-second")
	third := ledger.begin("request-third", "session-a", 8)
	if third == nil || third.Attempt != 1 || third.LogicalTurn != 8 {
		t.Fatalf("new logical turn workflow = %#v", third)
	}
}

func TestTurnWorkflowHUDWaitSnapshotReturnsOnRevision(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	started := ledger.begin("request-wait", "session-wait", 1)
	if started == nil {
		t.Fatal("workflow did not start")
	}
	type result struct {
		view turnWorkflowHUDViewModel
		ok   bool
	}
	resultCh := make(chan result, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		view, ok := ledger.waitSnapshot(ctx, "request-wait", started.Revision, 900*time.Millisecond)
		resultCh <- result{view: view, ok: ok}
	}()
	time.Sleep(20 * time.Millisecond)
	ledger.startStage("request-wait", turnWorkflowStageRecall)
	select {
	case got := <-resultCh:
		if !got.ok || got.view.Revision <= started.Revision || got.view.CurrentStage == nil || got.view.CurrentStage.Key != turnWorkflowStageRecall {
			t.Fatalf("wait result = %#v, found=%t", got.view, got.ok)
		}
	case <-time.After(time.Second):
		t.Fatal("waitSnapshot did not wake after revision")
	}
}

func TestTurnWorkflowHUDStagesExposeBackendDurationStatusAndReason(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-stage-ledger", "session-stage-ledger", 2)
	ledger.startStage("request-stage-ledger", turnWorkflowStagePublisherLLM)

	ledger.mu.Lock()
	entry := ledger.entries["request-stage-ledger"]
	index := turnWorkflowHUDStageIndex(entry.view.Stages, turnWorkflowStagePublisherLLM)
	startedAt := time.Now().UTC().Add(-1500 * time.Millisecond)
	entry.view.Stages[index].StartedAt = timePtr(startedAt)
	ledger.mu.Unlock()

	ledger.finishStage("request-stage-ledger", turnWorkflowStagePublisherLLM, "skipped", "deferred_no_guide_support")
	ledger.complete("request-stage-ledger")

	view, ok := ledger.snapshot("request-stage-ledger")
	if !ok || len(view.Stages) != len(turnWorkflowHUDStageTemplates) {
		t.Fatalf("terminal stage ledger = %#v, found=%t", view.Stages, ok)
	}
	publisher := view.Stages[index]
	if publisher.Status != "skipped" || publisher.ReasonCode != "deferred_no_guide_support" || publisher.DurationMS < 1400 {
		t.Fatalf("publisher stage = %#v", publisher)
	}
	if completeIndex := turnWorkflowHUDStageIndex(view.Stages, turnWorkflowStageComplete); completeIndex < 0 ||
		view.Stages[completeIndex].Status != "succeeded" || view.Stages[completeIndex].DurationMS < 0 {
		t.Fatalf("complete stage = %#v", view.Stages)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal terminal stage ledger: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"duration_ms"`)) {
		t.Fatalf("terminal stage ledger omitted duration_ms: %s", encoded)
	}

	ledger.begin("request-stage-failure", "session-stage-failure", 3)
	ledger.startStage("request-stage-failure", turnWorkflowStageCriticLLM)
	ledger.mu.Lock()
	failureEntry := ledger.entries["request-stage-failure"]
	failureIndex := turnWorkflowHUDStageIndex(failureEntry.view.Stages, turnWorkflowStageCriticLLM)
	failureStartedAt := time.Now().UTC().Add(-800 * time.Millisecond)
	failureEntry.view.Stages[failureIndex].StartedAt = timePtr(failureStartedAt)
	ledger.mu.Unlock()
	ledger.fail("request-stage-failure", "CRITIC_LLM_FAILED", "turn_hud.error.critic_llm_failed", turnWorkflowStageCriticLLM, true)

	failed, ok := ledger.snapshot("request-stage-failure")
	if !ok {
		t.Fatal("failed workflow stage ledger was not retained")
	}
	critic := failed.Stages[failureIndex]
	if critic.Status != "failed" || critic.ReasonCode != "CRITIC_LLM_FAILED" || critic.DurationMS < 700 {
		t.Fatalf("failed critic stage = %#v", critic)
	}
}

func TestTurnWorkflowHUDCountsIncludeAllZerosAndTerminalSeverity(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-warning", "session-warning", 3)
	ledger.setCounts("request-warning", map[string]int{"raw_user": 1})
	ledger.addWarning("request-warning", "OPTIONAL_SKIPPED", "turn_hud.warning.optional_skipped", turnWorkflowStagePublisherLLM)
	ledger.complete("request-warning")
	view, ok := ledger.snapshot("request-warning")
	if !ok || view.Status != "completed_with_warning" || view.Severity != "warning" || view.Error != nil {
		t.Fatalf("warning terminal view = %#v, found=%t", view, ok)
	}
	if len(view.Counts) != len(turnWorkflowHUDCountTemplates) {
		t.Fatalf("count entries = %d, want %d", len(view.Counts), len(turnWorkflowHUDCountTemplates))
	}
	values := map[string]int{}
	for _, count := range view.Counts {
		values[count.Key] = count.Value
	}
	if values["raw_user"] != 1 || values["raw_assistant"] != 0 || values["direct_evidence"] != 0 || values["total_committed"] != 1 {
		t.Fatalf("zero-inclusive counts = %#v", values)
	}

	ledger.begin("request-error", "session-error", 4)
	ledger.fail("request-error", "CRITIC_LLM_FAILED", "turn_hud.error.critic_llm_failed", turnWorkflowStageCriticLLM, true)
	failed, ok := ledger.snapshot("request-error")
	if !ok || failed.Status != "failed" || failed.Severity != "error" || failed.Error == nil {
		t.Fatalf("error terminal view = %#v, found=%t", failed, ok)
	}
	if failed.Error.Code != "CRITIC_LLM_FAILED" || !failed.Error.Retryable {
		t.Fatalf("error payload = %#v", failed.Error)
	}
}

func TestTurnWorkflowHUDUnknownRouteAndTTLBound(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	request := httptest.NewRequest("GET", "/turn-workflow/status?request_id=missing&after_revision=0&wait_ms=0", nil)
	recorder := httptest.NewRecorder()
	mux := http.NewServeMux()
	server.registerTurnRoutes(mux)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "unknown" || payload["contract_version"] != turnWorkflowHUDContractVersion {
		t.Fatalf("unknown payload = %#v", payload)
	}

	ledger := newTurnWorkflowHUDLedger()
	ledger.ttl = time.Millisecond
	ledger.begin("expired", "session-expired", 1)
	ledger.complete("expired")
	time.Sleep(3 * time.Millisecond)
	if _, ok := ledger.snapshot("expired"); ok {
		t.Fatal("terminal workflow was not pruned after TTL")
	}
}

func TestResolvePrepareTurnWorkflowLogicalTurnUsesHostPositionBeforeText(t *testing.T) {
	stored := []store.ChatLog{
		{ChatSessionID: "session", TurnIndex: 1, Role: "user", Content: "repeat"},
		{ChatSessionID: "session", TurnIndex: 1, Role: "assistant", Content: "answer"},
	}

	newTurnRequest, newTurnDecision := turnWorkflowHUDPrepareFixture(
		"repeat",
		[]turnWorkflowHUDObservedMessage{
			{index: 0, role: "user", content: "repeat"},
			{index: 1, role: "assistant", content: "answer"},
			{index: 2, role: "user", content: "repeat"},
		},
		2,
	)
	if got := resolvePrepareTurnWorkflowLogicalTurn(newTurnRequest, newTurnDecision, stored); got != 2 {
		t.Fatalf("repeated identical new turn = %d, want 2", got)
	}

	rerollRequest, rerollDecision := turnWorkflowHUDPrepareFixture(
		"repeat",
		[]turnWorkflowHUDObservedMessage{{index: 0, role: "user", content: "repeat"}},
		0,
	)
	if got := resolvePrepareTurnWorkflowLogicalTurn(rerollRequest, rerollDecision, stored); got != 1 {
		t.Fatalf("reroll logical turn = %d, want 1", got)
	}

	freshRequest, freshDecision := turnWorkflowHUDPrepareFixture(
		"first",
		[]turnWorkflowHUDObservedMessage{{index: 0, role: "user", content: "first"}},
		0,
	)
	if got := resolvePrepareTurnWorkflowLogicalTurn(freshRequest, freshDecision, nil); got != 1 {
		t.Fatalf("fresh logical turn = %d, want 1", got)
	}
}

type turnWorkflowHUDObservedMessage struct {
	index   int
	role    string
	content string
}

func turnWorkflowHUDPrepareFixture(rawInput string, observed []turnWorkflowHUDObservedMessage, currentIndex int) (dto.PrepareTurnContractRequest, dto.PrepareTurnCurrentInputDecisionV1) {
	requestID := "request-fixture"
	request := dto.PrepareTurnContractRequest{
		PrepareTurnRequest: dto.PrepareTurnRequest{
			ChatSessionID: "session",
			RawUserInput:  &rawInput,
		},
		HostObservations: &dto.PrepareTurnHostObservationsV1{
			ContractVersion: prepareHostObservationsVersion,
			SessionID:       "session",
			RequestID:       requestID,
		},
	}
	for _, item := range observed {
		index := item.index
		role := item.role
		content := item.content
		request.HostObservations.ActiveChat = append(request.HostObservations.ActiveChat, dto.PrepareTurnMessageObservationV1{
			MessageIndex: &index,
			Role:         &role,
			RawContent:   &content,
		})
	}
	index := currentIndex
	decision := dto.PrepareTurnCurrentInputDecisionV1{
		Status: "eligible",
		Envelope: &dto.PrepareTurnMessageSourceEnvelopeV1{
			Identity: dto.PrepareTurnMessageSourceIdentityV1{MessageIndex: &index},
		},
	}
	return request, decision
}
