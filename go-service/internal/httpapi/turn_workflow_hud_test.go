package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
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

func TestTurnWorkflowHUDDerivedPersistenceFailureKeepsCauseAndCommittedCount(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-derived", "session-a", 2)
	ledger.setPersistenceFacts("request-derived", "ok", 2, "error", 19, "not_requested", 0)
	ledger.failWithDetails(
		"request-derived",
		"DERIVED_PERSIST_FAILED",
		"turn_hud.error.derived_persist_failed",
		turnWorkflowStageDerivedPersist,
		true,
		[]turnWorkflowHUDDetail{
			{Key: "derived_attempted", Value: "19"},
			{Key: "derived_committed", Value: "0"},
			{Key: "transaction", Value: "atomic_rollback"},
			{Key: "reprocessing", Value: "queued"},
			{Key: "operation", Value: "CommitMemoryAdmission"},
			{Key: "cause", Value: "data too long for column relationship_kind"},
		},
	)
	ledger.setPersistenceFailureDetail(
		"request-derived",
		"derived_memory",
		"DERIVED_PERSIST_FAILED",
		"attempted=19 / committed=0 / transaction=atomic_rollback / CommitMemoryAdmission: data too long for column relationship_kind",
		0,
	)

	view, ok := ledger.snapshot("request-derived")
	if !ok || view.Error == nil || len(view.Error.Details) != 6 {
		t.Fatalf("failure view=%+v found=%t", view, ok)
	}
	if len(view.Error.RecoveryActions) != 1 ||
		view.Error.RecoveryActions[0].ID != turnWorkflowHUDRecoveryRetryDerivedTurn ||
		view.Error.RecoveryActions[0].Status != "available" {
		t.Fatalf("recovery actions=%+v", view.Error.RecoveryActions)
	}
	var derived turnWorkflowHUDFact
	for _, fact := range view.Facts {
		if fact.Key == "derived_memory" {
			derived = fact
			break
		}
	}
	if derived.Count == nil || *derived.Count != 0 ||
		derived.ReasonCode != "DERIVED_PERSIST_FAILED" ||
		!strings.Contains(derived.Detail, "committed=0") {
		t.Fatalf("derived fact=%+v", derived)
	}

	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled:    true,
		WorkflowSnapshot: &view,
	})
	workflow := requireDashboardCard(t, vm, "current_workflow")
	row := requireDashboardRow(t, workflow, "workflowFact.derived_memory")
	itemCount, itemCountOK := row.ItemCount.(*int)
	if !itemCountOK || itemCount == nil || *itemCount != 0 ||
		row.DetailCode != "DERIVED_PERSIST_FAILED" ||
		!strings.Contains(row.Detail, "attempted=19") ||
		!strings.Contains(row.Detail, "CommitMemoryAdmission") {
		t.Fatalf("dashboard derived row=%+v", row)
	}
	for _, label := range []string{"workflowFact.backend_processing", "workflowFact.finality"} {
		failureRow := requireDashboardRow(t, workflow, label)
		if failureRow.DetailCode != "DERIVED_PERSIST_FAILED" ||
			!strings.Contains(failureRow.Detail, "operation=CommitMemoryAdmission") ||
			!strings.Contains(failureRow.Detail, "cause=data too long") {
			t.Fatalf("dashboard failure row %s=%+v", label, failureRow)
		}
	}
}

func TestTurnWorkflowHUDCriticRecoveryRequiresDurableReprocessingQueue(t *testing.T) {
	for _, tc := range []struct {
		name        string
		details     []turnWorkflowHUDDetail
		retryable   bool
		wantActions int
	}{
		{
			name:        "durable queue",
			details:     []turnWorkflowHUDDetail{{Key: "reprocessing", Value: "queued"}},
			retryable:   false,
			wantActions: 1,
		},
		{
			name:        "no durable queue",
			details:     nil,
			retryable:   true,
			wantActions: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := newTurnWorkflowHUDLedger()
			ledger.begin("request-critic", "session-a", 1)
			ledger.failWithDetails(
				"request-critic",
				"CRITIC_JSON_PARSE_FAILED",
				"turn_hud.error.critic_llm_failed",
				turnWorkflowStageCriticLLM,
				tc.retryable,
				tc.details,
			)

			view, ok := ledger.snapshot("request-critic")
			if !ok || view.Error == nil {
				t.Fatalf("critic failure view=%+v found=%t", view, ok)
			}
			if got := len(view.Error.RecoveryActions); got != tc.wantActions {
				t.Fatalf("recovery actions=%+v, want count=%d", view.Error.RecoveryActions, tc.wantActions)
			}
		})
	}
}

type turnWorkflowHUDRecoveryStore struct {
	*adminDuplicateReprocessingStore
	reopenErr   error
	reopenCalls int
}

func (f *turnWorkflowHUDRecoveryStore) ReopenMemoryReprocessingJob(
	_ context.Context,
	_ string,
	chatSessionID string,
	sourceRevision string,
	_ time.Time,
) (bool, error) {
	f.reopenCalls++
	if chatSessionID != f.source.ChatSessionID || sourceRevision != f.source.SourceRevision {
		return false, errors.New("unexpected recovery source")
	}
	if f.reopenErr != nil {
		return false, f.reopenErr
	}
	return true, nil
}

func TestTurnWorkflowHUDRecoveryReopensOnlyTheFailedTurn(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	base.source.DerivedAdmissionState = "pending"
	base.source.DerivedAdmissionVersion = ""
	base.source.DerivedExtractorVersion = ""
	base.source.DerivedIndexVersion = ""
	base.source.DerivedResultHash = ""
	base.source.DerivedResultJSON = ""
	st := &turnWorkflowHUDRecoveryStore{adminDuplicateReprocessingStore: base}
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-recovery", base.source.ChatSessionID, base.source.TurnIndex)
	ledger.failWithDetails(
		"request-recovery",
		"DERIVED_PERSIST_FAILED",
		"turn_hud.error.derived_persist_failed",
		turnWorkflowStageDerivedPersist,
		true,
		[]turnWorkflowHUDDetail{{Key: "reprocessing", Value: "queued"}},
	)
	srv := &Server{
		Cfg:   config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store: st,
		RuntimeConfig: RuntimeConfig{
			Synced:           true,
			CriticProvider:   "openai",
			CriticAPIKey:     "critic-key",
			CriticEndpoint:   "https://example.invalid/v1/chat/completions",
			CriticModel:      "critic-model",
			CriticTimeoutSec: 30,
		},
		TurnWorkflows:    ledger,
		memoryWorkerWake: make(chan struct{}, 1),
	}
	body, _ := json.Marshal(turnWorkflowHUDRecoveryRequest{
		ContractVersion: turnWorkflowHUDRecoveryRequestContractVersion,
		RequestID:       "request-recovery",
		ActionID:        turnWorkflowHUDRecoveryRetryDerivedTurn,
	})
	req := httptest.NewRequest(http.MethodPost, "/turn-workflow/recovery", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleTurnWorkflowHUDRecovery(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if st.reopenCalls != 1 {
		t.Fatalf("reopen calls=%d", st.reopenCalls)
	}
	select {
	case <-srv.memoryWorkerWake:
	default:
		t.Fatal("recovery did not wake the memory worker")
	}
	view, ok := ledger.snapshot("request-recovery")
	if !ok || view.Error == nil || len(view.Error.RecoveryActions) != 1 ||
		view.Error.RecoveryActions[0].Status != "requested" {
		t.Fatalf("recovery view=%+v found=%t", view, ok)
	}
	if len(st.auditLogs) != 1 || st.auditLogs[0].TargetID != int64(base.source.TurnIndex) {
		t.Fatalf("recovery audit=%+v", st.auditLogs)
	}
}

func TestTurnWorkflowHUDRecoveryDoesNotInterruptAnActiveCriticRun(t *testing.T) {
	base := newAdminDuplicateReprocessingStore()
	base.source.DerivedAdmissionState = "pending"
	st := &turnWorkflowHUDRecoveryStore{
		adminDuplicateReprocessingStore: base,
		reopenErr:                       store.ErrMemoryReprocessingLeased,
	}
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-running-recovery", base.source.ChatSessionID, base.source.TurnIndex)
	ledger.failWithDetails(
		"request-running-recovery",
		"DERIVED_PERSIST_FAILED",
		"turn_hud.error.derived_persist_failed",
		turnWorkflowStageDerivedPersist,
		true,
		[]turnWorkflowHUDDetail{{Key: "reprocessing", Value: "queued"}},
	)
	srv := &Server{
		Cfg:   config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store: st,
		RuntimeConfig: RuntimeConfig{
			Synced:           true,
			CriticProvider:   "openai",
			CriticAPIKey:     "critic-key",
			CriticEndpoint:   "https://example.invalid/v1/chat/completions",
			CriticModel:      "critic-model",
			CriticTimeoutSec: 30,
		},
		TurnWorkflows: ledger,
	}
	body, _ := json.Marshal(turnWorkflowHUDRecoveryRequest{
		ContractVersion: turnWorkflowHUDRecoveryRequestContractVersion,
		RequestID:       "request-running-recovery",
		ActionID:        turnWorkflowHUDRecoveryRetryDerivedTurn,
	})
	rec := httptest.NewRecorder()
	srv.handleTurnWorkflowHUDRecovery(
		rec,
		httptest.NewRequest(http.MethodPost, "/turn-workflow/recovery", bytes.NewReader(body)),
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	view, _ := ledger.snapshot("request-running-recovery")
	if view.Error == nil || len(view.Error.RecoveryActions) != 1 ||
		view.Error.RecoveryActions[0].Status != "running" {
		t.Fatalf("active recovery view=%+v", view)
	}
}

func TestCompleteTurnPersistenceDiagnosticsRedactSecretsAndClassifyRollback(t *testing.T) {
	srv := &Server{RuntimeConfig: RuntimeConfig{CriticAPIKey: "critic-secret"}}
	diagnostics := srv.completeTurnPersistenceDiagnostics([]string{
		"CommitMemoryAdmission: Error 1406: data too long for column 'relationship_kind'; api_key=critic-secret",
	})
	if len(diagnostics) != 1 || diagnostics[0].Operation != "CommitMemoryAdmission" ||
		strings.Contains(diagnostics[0].Cause, "critic-secret") ||
		!strings.Contains(diagnostics[0].Cause, "[redacted]") {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	if got := completeTurnPersistenceRollbackState(diagnostics, 19, 1); got != "atomic_rollback" {
		t.Fatalf("rollback state=%q", got)
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
	supersededFacts := map[string]turnWorkflowHUDFact{}
	for _, fact := range superseded.Facts {
		supersededFacts[fact.Key] = fact
	}
	if supersededFacts["backend_processing"].Disposition != "dropped" ||
		supersededFacts["finality"].Disposition != "dropped" ||
		supersededFacts["finality"].Status != "invalidated" {
		t.Fatalf("superseded facts=%#v", supersededFacts)
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
	ledger.setCounts("request-warning", map[string]int{"raw_user": 1, "precise_memory": 3})
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
	if values["raw_user"] != 1 || values["raw_assistant"] != 0 || values["precise_memory"] != 3 || values["direct_evidence"] != 0 || values["total_committed"] != 4 {
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
	if failed.DismissalPolicy != turnWorkflowHUDDismissXOnly {
		t.Fatalf("error dismissal policy=%q", failed.DismissalPolicy)
	}
}

func TestTurnWorkflowHUDSeparatesKnowledgeGraphFromDirectionalRelationshipState(t *testing.T) {
	counts := turnWorkflowHUDCountsFromComplete(
		true, true,
		1,             // effective input
		1,             // memory
		0,             // precise memory
		2,             // direct evidence
		3,             // KG triples
		0,             // subjective memory
		0,             // world rules
		0, 0, 0, 0, 0, // character/state surfaces
		0, 0, // character events, storylines
		0, 0, // narrative current/events
		4, 5, // relationship current/events
		0, 0, 0, 0, 0, // pending, active, canonical, entities, trust
		0, 0, 0, 0, // identity surfaces and speaker attribution
		0, 0, // episode, vectors
	)
	if counts["knowledge_graph"] != 3 {
		t.Fatalf("knowledge_graph=%d, want KG triple count 3", counts["knowledge_graph"])
	}
	if counts["relationship_state"] != 9 {
		t.Fatalf("relationship_state=%d, want directional current+event count 9", counts["relationship_state"])
	}
	if _, legacy := counts["relationship_knowledge"]; legacy {
		t.Fatalf("ambiguous relationship_knowledge count must not remain: %#v", counts)
	}
}

func TestTurnWorkflowHUDTypedFactsTurnAlignmentAndPersistence(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	started := ledger.begin("request-facts", "session-facts", 7)
	if started == nil || started.ContractVersion != "turn_workflow_hud.v3" || started.Severity != turnWorkflowHUDSeverityNormal {
		t.Fatalf("started HUD=%#v", started)
	}
	ledger.setHostTurn("request-facts", 8, true)
	ledger.setFact("request-facts", turnWorkflowHUDFact{
		Key: "host_observation", Owner: "risu_host", Scope: "current_request",
		Status: "accepted", Disposition: "eligible", ReasonCode: "source_observation_eligible", Severity: turnWorkflowHUDSeverityNormal,
	})
	ledger.setFact("request-facts", turnWorkflowHUDFact{
		Key: "context_selection", Owner: "go_backend", Scope: "current_request",
		Status: "selected", Disposition: "selected", ReasonCode: "payload_plan_context_selected", Severity: turnWorkflowHUDSeverityNormal, Count: intValuePtr(240),
	})
	ledger.setPersistenceFacts("request-facts", "ok", 2, "empty", 0, "vector_not_configured", 0)
	ledger.addWarning("request-facts", "VECTOR_INDEX_SKIPPED", "turn_hud.warning.vector_index_skipped", turnWorkflowStageDerivedPersist)
	ledger.complete("request-facts")

	view, ok := ledger.snapshot("request-facts")
	if !ok {
		t.Fatal("typed HUD snapshot missing")
	}
	if view.TurnAlignment.State != "host_ahead" || view.TurnAlignment.HostTurn != 8 || view.TurnAlignment.BackendTurn != 7 {
		t.Fatalf("turn alignment=%#v", view.TurnAlignment)
	}
	facts := map[string]turnWorkflowHUDFact{}
	for _, fact := range view.Facts {
		facts[fact.Key] = fact
	}
	if facts["host_observation"].Disposition != "eligible" || facts["context_selection"].Disposition != "selected" || facts["context_selection"].Count == nil || *facts["context_selection"].Count != 240 {
		t.Fatalf("selection facts=%#v", facts)
	}
	if facts["raw_persistence"].Disposition != "delivered" || facts["derived_memory"].Disposition != "delivered" {
		t.Fatalf("persistence facts=%#v", facts)
	}
	if facts["vector_index"].Disposition != "dropped" || facts["vector_index"].Severity != turnWorkflowHUDSeverityWarning {
		t.Fatalf("vector fact=%#v", facts["vector_index"])
	}
	if facts["backend_processing"].Severity != turnWorkflowHUDSeverityNormal ||
		facts["finality"].Severity != turnWorkflowHUDSeverityNormal {
		t.Fatalf("successful backend/finality facts inherited an unrelated warning: %#v", facts)
	}
	if view.DismissalPolicy != turnWorkflowHUDDismissXOnly || view.Severity != turnWorkflowHUDSeverityWarning {
		t.Fatalf("warning presentation=%#v", view)
	}
}

func TestTurnWorkflowHUDCompletionClearsTransientAwaitingNotice(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-awaiting", "session-awaiting", 4)
	ledger.awaitFinal("request-awaiting")
	awaiting, ok := ledger.snapshot("request-awaiting")
	if !ok || awaiting.Severity != turnWorkflowHUDSeverityNotice {
		t.Fatalf("awaiting snapshot=%#v found=%t", awaiting, ok)
	}

	ledger.complete("request-awaiting")
	completed, ok := ledger.snapshot("request-awaiting")
	if !ok || completed.Status != "completed" || completed.Severity != turnWorkflowHUDSeverityNormal {
		t.Fatalf("completed snapshot=%#v found=%t", completed, ok)
	}
	facts := map[string]turnWorkflowHUDFact{}
	for _, fact := range completed.Facts {
		facts[fact.Key] = fact
	}
	if facts["backend_processing"].Severity != turnWorkflowHUDSeverityNormal ||
		facts["finality"].Severity != turnWorkflowHUDSeverityNormal {
		t.Fatalf("transient awaiting severity leaked into completed facts: %#v", facts)
	}
}

func TestTurnWorkflowHUDRepeatedIdenticalFactDoesNotCreateNewRevision(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-fact-idempotent", "session-fact-idempotent", 3)
	fact := turnWorkflowHUDFact{
		Key: "vector_index", Owner: "vector_store", Scope: "current_turn",
		Status: "queued", Disposition: "deferred",
		ReasonCode: "rollback_vector_retry_queued", Severity: turnWorkflowHUDSeverityNotice,
		Count: intValuePtr(1),
	}
	ledger.setFact("request-fact-idempotent", fact)
	first, ok := ledger.snapshot("request-fact-idempotent")
	if !ok {
		t.Fatal("first fact snapshot missing")
	}
	ledger.setFact("request-fact-idempotent", fact)
	repeated, ok := ledger.snapshot("request-fact-idempotent")
	if !ok {
		t.Fatal("repeated fact snapshot missing")
	}
	if repeated.Revision != first.Revision || !repeated.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("identical fact changed workflow revision: first=%#v repeated=%#v", first, repeated)
	}
}

func TestTurnWorkflowHUDOperationNoticePreservesBackendSeverity(t *testing.T) {
	confirmed := newTurnWorkflowHUDOperationNotice(
		"rollback:confirmed",
		"session-delete",
		9,
		"completed",
		"info",
		"turn_hud.notice.delete_confirmed",
		"turn_hud.notice.delete_confirmed_detail",
		"ASSISTANT_OUTPUT_DELETE_CONFIRMED",
	)
	if confirmed.DisplayMode != "notice" || confirmed.Status != "completed" || confirmed.Error != nil {
		t.Fatalf("confirmed delete notice = %#v", confirmed)
	}
	if confirmed.Severity != turnWorkflowHUDSeverityNotice || confirmed.DismissalPolicy != turnWorkflowHUDDismissCardOrX || confirmed.NoticeKind != "delete" {
		t.Fatalf("confirmed delete presentation = %#v", confirmed)
	}
	confirmedFacts := map[string]turnWorkflowHUDFact{}
	for _, fact := range confirmed.Facts {
		confirmedFacts[fact.Key] = fact
	}
	if confirmedFacts["host_observation"].Disposition != "eligible" ||
		confirmedFacts["finality"].Status != "deleted" ||
		confirmedFacts["finality"].Disposition != "dropped" {
		t.Fatalf("confirmed delete facts=%#v", confirmedFacts)
	}

	partial := newTurnWorkflowHUDOperationNotice(
		"rollback:partial",
		"session-delete",
		9,
		"failed",
		"error",
		"turn_hud.notice.delete_sync_failed",
		"turn_hud.error.delete_sync_partial",
		"ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL",
	)
	if partial.DisplayMode != "notice" || partial.Status != "failed" || partial.Severity != "error" || partial.Error == nil {
		t.Fatalf("partial delete notice = %#v", partial)
	}
	if partial.Error.Code != "ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL" || partial.Error.MessageKey != "turn_hud.error.delete_sync_partial" {
		t.Fatalf("partial delete error = %#v", partial.Error)
	}
	partialFacts := map[string]turnWorkflowHUDFact{}
	for _, fact := range partial.Facts {
		partialFacts[fact.Key] = fact
	}
	if partialFacts["finality"].Status != "deleted" ||
		partialFacts["finality"].Disposition != "dropped" ||
		partialFacts["finality"].Severity != turnWorkflowHUDSeverityError {
		t.Fatalf("partial delete facts=%#v", partialFacts)
	}
}

func TestTurnWorkflowHUDNoticeRouteAcceptsOnlyTypedOOCObservation(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	mux := http.NewServeMux()
	server.registerTurnRoutes(mux)
	body := []byte(`{"contract_version":"turn_workflow_notice_observation.v1","kind":"ooc_input_cancelled","request_id":"ooc-1","chat_session_id":"session-ooc","host_turn":5}`)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/turn-workflow/notice", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("notice status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var view turnWorkflowHUDViewModel
	if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.NoticeKind != "ooc" || view.Severity != turnWorkflowHUDSeverityNotice || view.DismissalPolicy != turnWorkflowHUDDismissCardOrX {
		t.Fatalf("OOC notice=%#v", view)
	}
	if view.TurnAlignment.HostTurn != 5 || view.TurnAlignment.State != "unobserved" || view.TurnAlignment.ReasonCode != "backend_turn_unobserved" {
		t.Fatalf("OOC turn alignment=%#v", view.TurnAlignment)
	}
	facts := map[string]turnWorkflowHUDFact{}
	for _, fact := range view.Facts {
		facts[fact.Key] = fact
	}
	if facts["host_observation"].Disposition != "dropped" ||
		facts["backend_processing"].Disposition != "dropped" ||
		facts["finality"].Status != "cancelled" ||
		facts["finality"].Disposition != "dropped" ||
		facts["raw_persistence"].Status != "skipped" ||
		facts["raw_persistence"].Disposition != "dropped" ||
		facts["derived_memory"].Status != "skipped" ||
		facts["vector_index"].Status != "not_requested" {
		t.Fatalf("OOC facts=%#v", facts)
	}
	if snapshot, ok := server.TurnWorkflows.latestSnapshotForSession("session-ooc"); !ok || snapshot.RequestID != "ooc-1" {
		t.Fatalf("recorded OOC snapshot=%#v found=%t", snapshot, ok)
	}
	repeated := httptest.NewRecorder()
	mux.ServeHTTP(repeated, httptest.NewRequest(http.MethodPost, "/turn-workflow/notice", bytes.NewReader(body)))
	if repeated.Code != http.StatusOK {
		t.Fatalf("repeated OOC status=%d body=%s", repeated.Code, repeated.Body.String())
	}
	var repeatedView turnWorkflowHUDViewModel
	if err := json.Unmarshal(repeated.Body.Bytes(), &repeatedView); err != nil {
		t.Fatal(err)
	}
	if len(server.TurnWorkflows.entries) != 1 || repeatedView.RequestID != view.RequestID ||
		repeatedView.Revision != view.Revision || !repeatedView.UpdatedAt.Equal(view.UpdatedAt) {
		t.Fatalf("repeated OOC created a second operation: first=%#v repeated=%#v entries=%d", view, repeatedView, len(server.TurnWorkflows.entries))
	}

	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/turn-workflow/notice", bytes.NewReader([]byte(`{"contract_version":"turn_workflow_notice_observation.v1","kind":"reroll","request_id":"bad","chat_session_id":"session-ooc"}`))))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unsupported notice kind status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestTurnWorkflowHUDCompleteWithNoticeIsTerminalAndPreservesWarnings(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("request-reroll", "session-reroll", 9)
	ledger.addWarning("request-reroll", "OPTIONAL_SKIPPED", "turn_hud.warning.optional_skipped", turnWorkflowStagePublisherLLM)
	ledger.completeWithNotice(
		"request-reroll",
		"turn_hud.notice.reroll_confirmed",
		"turn_hud.notice.reroll_confirmed_detail",
		"LOGICAL_TURN_REPLACED",
	)

	view, ok := ledger.snapshot("request-reroll")
	if !ok {
		t.Fatal("reroll notice was not retained")
	}
	if view.Status != "completed_with_warning" || view.Severity != "warning" || view.DisplayMode != "notice" {
		t.Fatalf("reroll notice terminal state = %#v", view)
	}
	if view.TitleKey != "turn_hud.notice.reroll_confirmed" ||
		view.MessageKey != "turn_hud.notice.reroll_confirmed_detail" ||
		view.NoticeCode != "LOGICAL_TURN_REPLACED" {
		t.Fatalf("reroll notice presentation = %#v", view)
	}
}

func TestTurnWorkflowHUDDuplicateReplayAndConflictUseDifferentSeverity(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	replayAny := server.completeTurnWorkflowHUDDuplicate(
		"duplicate-replay", "session-duplicate", 4,
		"duplicate_turn_replay", "DUPLICATE_TURN_REPLAY",
		"turn_hud.warning.duplicate_turn_replay", "turn_hud.notice.duplicate_existing_preserved",
	)
	replay, ok := replayAny.(turnWorkflowHUDViewModel)
	if !ok {
		t.Fatalf("duplicate replay HUD type=%T", replayAny)
	}
	if replay.NoticeKind != "duplicate" || replay.Severity != turnWorkflowHUDSeverityNotice || replay.DismissalPolicy != turnWorkflowHUDDismissCardOrX {
		t.Fatalf("duplicate replay HUD=%#v", replay)
	}
	replayFacts := map[string]turnWorkflowHUDFact{}
	for _, fact := range replay.Facts {
		replayFacts[fact.Key] = fact
	}
	if replayFacts["finality"].Status != "existing_preserved" ||
		replayFacts["finality"].Disposition != "dropped" ||
		replayFacts["raw_persistence"].Status != "existing" ||
		replayFacts["derived_memory"].Status != "existing" {
		t.Fatalf("duplicate replay facts=%#v", replayFacts)
	}

	conflictAny := server.completeTurnWorkflowHUDDuplicate(
		"duplicate-conflict", "session-duplicate", 5,
		"duplicate_turn_conflict", "DUPLICATE_TURN_CONFLICT",
		"turn_hud.warning.duplicate_turn_conflict", "turn_hud.notice.duplicate_conflict_preserved",
	)
	conflict, ok := conflictAny.(turnWorkflowHUDViewModel)
	if !ok {
		t.Fatalf("duplicate conflict HUD type=%T", conflictAny)
	}
	if conflict.NoticeKind != "duplicate" || conflict.Severity != turnWorkflowHUDSeverityWarning || conflict.DismissalPolicy != turnWorkflowHUDDismissXOnly {
		t.Fatalf("duplicate conflict HUD=%#v", conflict)
	}
	conflictFacts := map[string]turnWorkflowHUDFact{}
	for _, fact := range conflict.Facts {
		conflictFacts[fact.Key] = fact
	}
	if conflictFacts["finality"].Status != "existing_preserved" ||
		conflictFacts["finality"].Disposition != "dropped" ||
		conflictFacts["finality"].Severity != turnWorkflowHUDSeverityWarning {
		t.Fatalf("duplicate conflict facts=%#v", conflictFacts)
	}
}

func TestTurnWorkflowHUDUnknownRouteAndCapacityBound(t *testing.T) {
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
	ledger.maxEntries = 1
	ledger.begin("capacity-old", "session-capacity-old", 1)
	ledger.complete("capacity-old")
	ledger.begin("capacity-new", "session-capacity-new", 1)
	if _, ok := ledger.snapshot("capacity-old"); ok {
		t.Fatal("oldest terminal workflow was not evicted at capacity")
	}
	if _, ok := ledger.snapshot("capacity-new"); !ok {
		t.Fatal("new workflow was not retained at capacity")
	}
}

func TestTurnWorkflowHUDEventsPreserveRevisionOrderAndCloseOnTerminal(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	initial := server.TurnWorkflows.begin("events-order", "session-events", 3)
	if initial == nil {
		t.Fatal("begin returned nil")
	}
	server.TurnWorkflows.setHostTurn("events-order", 3, true)
	server.TurnWorkflows.setHostTurn("events-order", 4, true)
	server.TurnWorkflows.complete("events-order")

	request := httptest.NewRequest(
		"GET",
		"/turn-workflow/events?request_id=events-order&after_revision=0&wait_ms=0",
		nil,
	)
	recorder := httptest.NewRecorder()
	mux := http.NewServeMux()
	server.registerTurnRoutes(mux)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/x-ndjson" {
		t.Fatalf("content type = %q", contentType)
	}
	decoder := json.NewDecoder(recorder.Body)
	var revisions []int64
	var final turnWorkflowHUDViewModel
	for {
		var view turnWorkflowHUDViewModel
		if err := decoder.Decode(&view); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode event: %v", err)
		}
		revisions = append(revisions, view.Revision)
		final = view
	}
	if len(revisions) != 4 {
		t.Fatalf("revisions = %v, want four events", revisions)
	}
	for index, revision := range revisions {
		want := int64(index + 1)
		if revision != want {
			t.Fatalf("revisions = %v, want contiguous revision %d at index %d", revisions, want, index)
		}
	}
	if final.Status != "completed" {
		t.Fatalf("final status = %q, want completed", final.Status)
	}
}

func TestTurnWorkflowHUDEventsWaitForPrepareTurnRegistration(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result := make(chan []turnWorkflowHUDViewModel, 1)
	errs := make(chan error, 1)
	go func() {
		var views []turnWorkflowHUDViewModel
		_, err := ledger.streamSnapshots(ctx, "events-before-prepare", 0, func(view turnWorkflowHUDViewModel) error {
			views = append(views, view)
			return nil
		})
		if err != nil {
			errs <- err
			return
		}
		result <- views
	}()

	select {
	case views := <-result:
		t.Fatalf("stream ended before workflow registration: %#v", views)
	case err := <-errs:
		t.Fatalf("stream failed before workflow registration: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	ledger.begin("events-before-prepare", "session-events", 5)
	ledger.complete("events-before-prepare")

	select {
	case views := <-result:
		if len(views) != 2 {
			t.Fatalf("views = %#v, want initial and terminal revisions", views)
		}
		if views[0].Revision != 1 || views[0].Status != "running" {
			t.Fatalf("initial view = %#v", views[0])
		}
		if views[1].Revision != 2 || views[1].Status != "completed" {
			t.Fatalf("terminal view = %#v", views[1])
		}
	case err := <-errs:
		t.Fatalf("stream failed after workflow registration: %v", err)
	case <-ctx.Done():
		t.Fatal("stream did not observe workflow registration")
	}
}

func TestTurnWorkflowHUDEventsCloseOnRequestCancellation(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	initial := ledger.begin("events-cancel", "session-events", 4)
	if initial == nil {
		t.Fatal("begin returned nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	unexpected := make(chan struct{}, 1)
	go func() {
		_, err := ledger.streamSnapshots(ctx, "events-cancel", initial.Revision, func(turnWorkflowHUDViewModel) error {
			unexpected <- struct{}{}
			return nil
		})
		done <- err
	}()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("stream error = %v, want context canceled", err)
	}
	select {
	case <-unexpected:
		t.Fatal("unexpected event after current revision")
	default:
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
