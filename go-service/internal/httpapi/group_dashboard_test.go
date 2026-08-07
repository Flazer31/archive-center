package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestPersistenceDashboardDistinguishesAttemptedCommittedAndRollback(t *testing.T) {
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled: true,
		RuntimeState: map[string]any{
			"lastCompleteTurnStatus": map[string]any{
				"status":                "fail",
				"turnIndex":             2,
				"chatLogsSaved":         2,
				"derivedArtifactsSaved": 0,
				"rawStatus":             "ok",
				"derivedStatus":         "error",
				"vectorStatus":          "not_requested",
				"persistencePipeline": map[string]any{
					"raw": map[string]any{"status": "ok"},
					"derived": map[string]any{
						"status":         "error",
						"attempted":      19,
						"committed":      0,
						"rollback_state": "atomic_rollback",
						"error_diagnostics": []any{map[string]any{
							"operation": "CommitMemoryAdmission",
							"cause":     "data too long for column relationship_kind",
						}},
					},
					"vector": map[string]any{"status": "not_requested"},
				},
			},
		},
	})
	persistence := requireDashboardCard(t, vm, "persistence_lanes")
	derived := requireDashboardRow(t, persistence, "derived")
	if derived.Status != "fail" ||
		!strings.Contains(derived.Detail, "attempted:19") ||
		!strings.Contains(derived.Detail, "committed:0") ||
		!strings.Contains(derived.Detail, "transaction:atomic_rollback") ||
		!strings.Contains(derived.Detail, "CommitMemoryAdmission") {
		t.Fatalf("derived persistence row=%+v", derived)
	}
}

func TestBuildDashboardViewModelOwnsStatusAndLaneCalculation(t *testing.T) {
	req := dashboardViewModelRequest{
		PluginEnabled:            true,
		PrepareTurnEverContacted: true,
		FailedQueueDepth:         2,
		FirstTurnLight:           true,
		RuntimeState: map[string]any{
			"lastSupervisorStatus": map[string]any{"status": "ok", "time": "2026-07-11T01:02:03Z"},
			"lastSearchStatus":     map[string]any{"status": "unknown"},
			"prepareTurnStatus":    map[string]any{"status": "off"},
			"lastCompleteTurnStatus": map[string]any{
				"status":               "ok",
				"reason_code":          "idempotent_pair_replay",
				"turnIndex":            7,
				"detail":               "idempotent pair replay; duplicate save skipped",
				"chatLogsSaved":        2,
				"memoriesSaved":        1,
				"vectorUpserted":       3,
				"rawStatus":            "ok",
				"derivedStatus":        "ok",
				"vectorMemoryUpserted": 1,
			},
			"queuePersistence": map[string]any{
				"lastLoad": map[string]any{"status": "ok"},
				"lastSave": map[string]any{"status": "ok"},
			},
		},
		GuideModeState: map[string]any{"status": "ok", "detail": "standard / auto_inferred"},
	}

	vm := buildDashboardViewModel(req)
	if vm.ContractVersion != dashboardViewModelContractVersion || vm.Status != "ok" {
		t.Fatalf("unexpected contract: %+v", vm)
	}
	connection := requireDashboardCard(t, vm, "connection")
	if got := requireDashboardRow(t, connection, "supervisorHealthTest"); got.Status != "ok" || got.DetailCode != "supervisorOkByTurn" {
		t.Fatalf("supervisor row=%+v", got)
	}
	if got := requireDashboardRow(t, connection, "search"); got.Status != "skipped" || got.DetailCode != "firstTurnLight" {
		t.Fatalf("first-turn search row=%+v", got)
	}
	engine := requireDashboardCard(t, vm, "engine")
	if got := requireDashboardRow(t, engine, "turnEngine"); got.Status != "skipped" || got.DetailCode != "firstTurnLight" {
		t.Fatalf("first-turn engine row=%+v", got)
	}
	saveQueue := requireDashboardCard(t, vm, "save_queue")
	if findDashboardCard(vm, "current_queue") != nil {
		t.Fatalf("unscoped failed_queue_depth must not be shown as current-turn queue: %+v", vm.Cards)
	}
	historicalQueue := requireDashboardCard(t, vm, "historical_queue")
	if got := requireDashboardRow(t, historicalQueue, "queueHistory.transport_retry"); got.Status != "notice" || got.Detail != "2 unknown" || got.Scope != "unknown" {
		t.Fatalf("historical retry row=%+v", got)
	}
	if saveQueue.Summary.Warn != 0 {
		t.Fatalf("save queue must not inherit historical queue severity, summary=%+v", saveQueue.Summary)
	}
	persistence := requireDashboardCard(t, vm, "persistence_lanes")
	for _, label := range []string{"rawSave", "derived", "vectorUpsert"} {
		row := requireDashboardRow(t, persistence, label)
		if row.Status != "ok" || row.DetailCode != "noNewLaneNeeded" {
			t.Fatalf("persistence %s=%+v", label, row)
		}
	}
	complete := requireDashboardCard(t, vm, "complete_turn")
	if complete.Summary.OK != 1 || complete.Severity != "ok" {
		t.Fatalf("complete summary=%+v severity=%s", complete.Summary, complete.Severity)
	}
}

func TestDashboardViewModelRoute(t *testing.T) {
	body, err := json.Marshal(dashboardViewModelRequest{PluginEnabled: true, RuntimeState: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server := &Server{}
	server.handleDashboardViewModel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var vm dashboardViewModel
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
		t.Fatal(err)
	}
	if vm.ContractVersion != dashboardViewModelContractVersion || len(vm.Cards) < 3 {
		t.Fatalf("response=%+v", vm)
	}
}

func TestDashboardLegacyRuntimeStateUsesOnlyTypedStatusSeverityAndReason(t *testing.T) {
	body, err := json.Marshal(dashboardViewModelRequest{
		PluginEnabled: true,
		RuntimeState: map[string]any{
			"lastBridgeHealth": map[string]any{
				"detail": "native afterRequest missing; recovered from active chat",
			},
			"lastInjectionStatus": map[string]any{
				"severity": "warning",
				"detail":   "accepted (existing pair)",
			},
			"lastSaveStatus": map[string]any{
				"status": "warn",
				"detail": "waiting for RisuAI active chat confirmation",
			},
			"lastCompleteStatus": map[string]any{
				"status":      "warn",
				"reason_code": "pending_sync",
				"detail":      "arbitrary operator prose",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server := &Server{}
	server.handleDashboardViewModel(recorder, httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var vm dashboardViewModel
	if err := json.Unmarshal(recorder.Body.Bytes(), &vm); err != nil {
		t.Fatal(err)
	}

	connection := requireDashboardCard(t, vm, "connection")
	if got := requireDashboardRow(t, connection, "bridgeHealth"); got.Status != "unknown" || got.DetailCode != "" {
		t.Fatalf("missing typed fields must stay unknown/unobserved: %+v", got)
	}
	saveQueue := requireDashboardCard(t, vm, "save_queue")
	if got := requireDashboardRow(t, saveQueue, "injection"); got.Status != "warn" || got.DetailCode != "" {
		t.Fatalf("typed severity was not authoritative: %+v", got)
	}
	if got := requireDashboardRow(t, saveQueue, "save"); got.Status != "warn" || got.DetailCode != "" {
		t.Fatalf("misleading prose changed typed status: %+v", got)
	}
	if got := requireDashboardRow(t, saveQueue, "complete"); got.Status != "notice" || got.DetailCode != "pendingSync" {
		t.Fatalf("typed reason did not drive the stable dashboard disposition: %+v", got)
	}
}

func TestDashboardLegacyBackfillUsesTypedCountsForDetailCode(t *testing.T) {
	tests := []struct {
		name       string
		state      map[string]any
		wantStatus string
		wantCode   string
	}{
		{
			name: "nothing missing",
			state: map[string]any{
				"status":        "skipped",
				"detail":        "operator prose may change freely",
				"savedCount":    0,
				"existingCount": 1,
				"queuedCount":   0,
			},
			wantStatus: "ok",
			wantCode:   "noMissingBackfill",
		},
		{
			name: "manual rebuild queued",
			state: map[string]any{
				"status":      "warn",
				"reason_code": "recent_active_chat_rebuild_queued",
				"detail":      "localized prose is not an authority field",
				"savedCount":  0,
				"queuedCount": 1,
			},
			wantStatus: "notice",
			wantCode:   "activeChatRebuildQueued",
		},
		{
			name: "skipped backfill is not complete",
			state: map[string]any{
				"status":        "skipped",
				"savedCount":    0,
				"existingCount": 1,
				"queuedCount":   0,
				"skippedCount":  1,
			},
			wantStatus: "skipped",
			wantCode:   "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vm := buildDashboardViewModel(dashboardViewModelRequest{
				PluginEnabled: true,
				RuntimeState: map[string]any{
					"lastActiveChatBackfill": test.state,
				},
			})
			row := requireDashboardRow(t, requireDashboardCard(t, vm, "activity"), "activeChatBackfill")
			if row.Status != test.wantStatus || row.DetailCode != test.wantCode {
				t.Fatalf("typed backfill observation did not own row classification: %+v", row)
			}
		})
	}
}

func TestDashboardViewModelRouteIncludesLatestSessionWorkflow(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	server.TurnWorkflows.begin("request-latest", "session-latest", 3)
	server.TurnWorkflows.setHostTurn("request-latest", 4, true)
	body, err := json.Marshal(dashboardViewModelRequest{
		PluginEnabled: true, CurrentSessionID: "session-latest", RuntimeState: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleDashboardViewModel(recorder, httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var vm dashboardViewModel
	if err := json.Unmarshal(recorder.Body.Bytes(), &vm); err != nil {
		t.Fatal(err)
	}
	alignment := requireDashboardRow(t, requireDashboardCard(t, vm, "current_workflow"), "turnAlignment")
	if alignment.Status != "warn" || alignment.DetailCode != "host_turn_ahead_of_backend" {
		t.Fatalf("latest workflow alignment=%+v", alignment)
	}
}

func TestDashboardViewModelRoutePrefersExactCurrentWorkflowOverNewerOperation(t *testing.T) {
	server := &Server{TurnWorkflows: newTurnWorkflowHUDLedger()}
	server.TurnWorkflows.begin("request-current", "session-exact", 3)
	server.TurnWorkflows.setHostTurn("request-current", 4, true)
	server.turnWorkflowHUDOperationNotice(
		"operation-newer", "session-exact", 3, "completed", "notice",
		"turn_hud.notice.delete_confirmed", "turn_hud.notice.delete_confirmed_detail", "ASSISTANT_OUTPUT_DELETE_CONFIRMED",
	)
	body, err := json.Marshal(dashboardViewModelRequest{
		PluginEnabled: true, CurrentSessionID: "session-exact", CurrentWorkflowRequestID: "request-current", RuntimeState: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleDashboardViewModel(recorder, httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var vm dashboardViewModel
	if err := json.Unmarshal(recorder.Body.Bytes(), &vm); err != nil {
		t.Fatal(err)
	}
	alignment := requireDashboardRow(t, requireDashboardCard(t, vm, "current_workflow"), "turnAlignment")
	if alignment.DetailCode != "host_turn_ahead_of_backend" {
		t.Fatalf("dashboard substituted a newer operation for the exact current workflow: %+v", alignment)
	}
}

func TestDashboardViewModelRouteStableSourceLaneSeverity(t *testing.T) {
	tests := []struct {
		status   string
		severity string
	}{
		{status: "eligible", severity: "ok"},
		{status: "empty", severity: "neutral"},
		{status: "not_applicable", severity: "neutral"},
		{status: "degraded", severity: "warn"},
		{status: "deferred", severity: "notice"},
		{status: "failed", severity: "fail"},
		{status: "incompatible", severity: "fail"},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			body, err := json.Marshal(dashboardViewModelRequest{
				PluginEnabled:            true,
				CurrentSessionID:         "session-source-contract",
				PrepareTurnEverContacted: true,
				GuideModeState:           map[string]any{"status": "ok"},
				RuntimeState: map[string]any{
					"lastBridgeHealth":     map[string]any{"status": "ok"},
					"lastSupervisorWakeup": map[string]any{"status": "ok"},
					"lastSearchStatus":     map[string]any{"status": "ok"},
					"lastSupervisorStatus": map[string]any{"status": "ok"},
					"prepareTurnStatus":    map[string]any{"status": test.status, "detail": map[string]any{"reason_code": "fixture_reason"}},
					"lastInjectionStatus":  map[string]any{"status": "ok"},
					"lastSaveStatus":       map[string]any{"status": "ok"},
					"lastCompleteStatus":   map[string]any{"status": "ok"},
					"queuePersistence": map[string]any{
						"lastLoad": map[string]any{"status": "ok"},
						"lastSave": map[string]any{"status": "ok"},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			server := &Server{}
			mux := http.NewServeMux()
			server.registerDashboardRoutes(mux)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body)))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			var viewModel dashboardViewModel
			if err := json.Unmarshal(recorder.Body.Bytes(), &viewModel); err != nil {
				t.Fatal(err)
			}
			engine := requireDashboardCard(t, viewModel, "engine")
			row := requireDashboardRow(t, engine, "turnEngine")
			if row.Status != test.status {
				t.Fatalf("stable row status=%q, want %q", row.Status, test.status)
			}
			if test.severity != "neutral" && engine.Severity != test.severity {
				t.Fatalf("engine severity=%q, want %q: %+v", engine.Severity, test.severity, engine)
			}
			switch test.severity {
			case "fail":
				if engine.Summary.Fail == 0 || viewModel.Summary.Fail == 0 {
					t.Fatalf("%s must fail both card and global summary: card=%+v global=%+v", test.status, engine.Summary, viewModel.Summary)
				}
			case "warn":
				if engine.Summary.Warn == 0 || viewModel.Summary.Warn == 0 {
					t.Fatalf("%s must warn both card and global summary: card=%+v global=%+v", test.status, engine.Summary, viewModel.Summary)
				}
			case "notice":
				if engine.Summary.Notice == 0 || viewModel.Summary.Notice == 0 || engine.Summary.Fail != 0 || engine.Summary.Warn != 0 {
					t.Fatalf("%s must be advisory: card=%+v global=%+v", test.status, engine.Summary, viewModel.Summary)
				}
			case "neutral":
				if engine.Summary.Neutral == 0 || viewModel.Summary.Neutral == 0 || engine.Summary.Fail != 0 || engine.Summary.Warn != 0 {
					t.Fatalf("%s must remain neutral: card=%+v global=%+v", test.status, engine.Summary, viewModel.Summary)
				}
			default:
				if engine.Summary.Fail != 0 || engine.Summary.Warn != 0 {
					t.Fatalf("%s must remain non-error: %+v", test.status, engine.Summary)
				}
			}
		})
	}
}

func TestDashboardAdvisoryRuntimeStatesDoNotBecomeWarnings(t *testing.T) {
	req := dashboardViewModelRequest{
		PluginEnabled:            true,
		PrepareTurnEverContacted: true,
		FailedQueueDepth:         3,
		GuideModeState:           map[string]any{"status": "ok"},
		RuntimeState: map[string]any{
			"prepareTurnStatus": map[string]any{
				"status": "ok",
				"backendTiming": map[string]any{
					"total_ms":      12500,
					"slowest_ms":    12000,
					"slowest_stage": "vector_recall",
				},
			},
			"lastSaveStatus": map[string]any{
				"status":      "warn",
				"reason_code": "pending_sync",
				"detail":      "waiting for RisuAI active chat confirmation",
			},
			"lastCompleteStatus": map[string]any{"status": "ok"},
			"queuePersistence": map[string]any{
				"lastLoad": map[string]any{"status": "ok"},
				"lastSave": map[string]any{"status": "ok"},
			},
			"lastAutoRollback": map[string]any{
				"status":      "warn",
				"reason_code": "history_trim_protected",
				"detail":      "active chat tail is shorter than backend; possible /cut",
			},
			"lastStreamingAfterRequest": map[string]any{
				"status":      "warn",
				"reason_code": "native_after_request_active_chat_recovered",
				"detail":      "native afterRequest missing; recovered from active chat",
			},
			"lastRisuForkCopyCapture": map[string]any{
				"status":      "warn",
				"reason_code": "risu_fork_copy_observed",
				"detail":      "observed source-session -> target-session",
			},
			"lastRerollReplacement": map[string]any{
				"status":      "ok",
				"reason_code": "logical_turn_replaced",
				"detail":      "logical_turn_replaced",
				"turnIndex":   7,
			},
		},
	}

	vm := buildDashboardViewModel(req)
	if vm.Summary.Warn != 0 || vm.Summary.Fail != 0 {
		t.Fatalf("advisory runtime states must not raise warning/failure counts: %+v", vm.Summary)
	}
	if vm.Summary.Notice < 6 {
		t.Fatalf("expected advisory states in summary, got %+v", vm.Summary)
	}

	saveQueue := requireDashboardCard(t, vm, "save_queue")
	if got := requireDashboardRow(t, saveQueue, "save"); got.Status != "notice" || got.DetailCode != "pendingSync" {
		t.Fatalf("active-chat confirmation wait=%+v", got)
	}
	if findDashboardCard(vm, "current_queue") != nil {
		t.Fatalf("unscoped retry depth must not become a current queue card: %+v", vm.Cards)
	}
	if got := requireDashboardRow(t, requireDashboardCard(t, vm, "historical_queue"), "queueHistory.transport_retry"); got.Status != "notice" || got.Scope != "unknown" {
		t.Fatalf("historical retry queue=%+v", got)
	}

	activity := requireDashboardCard(t, vm, "activity")
	if got := requireDashboardRow(t, activity, "autoRollback"); got.Status != "notice" || got.DetailCode != "historyTrimProtected" {
		t.Fatalf("protected history trim=%+v", got)
	}
	if got := requireDashboardRow(t, activity, "streamingHook"); got.Status != "ok" || got.DetailCode != "streamingRecovered" {
		t.Fatalf("successful streaming recovery=%+v", got)
	}
	if got := requireDashboardRow(t, activity, "forkCopyCapture"); got.Status != "notice" || got.DetailCode != "forkCopyObserved" {
		t.Fatalf("fork copy observation=%+v", got)
	}
	if got := requireDashboardRow(t, activity, "rerollReplacement"); got.Status != "notice" || got.DetailCode != "rerollReplaced" || dashboardInt(got.TurnIndex) != 7 {
		t.Fatalf("confirmed reroll replacement=%+v", got)
	}

	timing := requireDashboardCard(t, vm, "backend_timing")
	if got := requireDashboardRow(t, timing, "prepareTiming"); got.Status != "notice" {
		t.Fatalf("slow timing must be informational, got %+v", got)
	}
}

func TestDashboardSeparatesCurrentAndHistoricalQueueObservations(t *testing.T) {
	workflow := turnWorkflowHUDViewModel{
		RequestID: "request-current", ChatSessionID: "session-current", BackendTurn: 8,
		TurnAlignment: turnWorkflowHUDTurnAlignment{HostTurn: 8, BackendTurn: 8, State: "aligned", ReasonCode: "host_backend_turn_aligned"},
		Facts:         []turnWorkflowHUDFact{{Key: "raw_persistence", Status: "ok", Severity: turnWorkflowHUDSeverityNormal}},
	}
	req := dashboardViewModelRequest{
		PluginEnabled:            true,
		CurrentSessionID:         "session-current",
		CurrentWorkflowRequestID: "request-current",
		WorkflowSnapshot:         &workflow,
		RuntimeState:             map[string]any{},
		QueueObservations: []dashboardQueueObservation{
			{QueueKind: "pending_confirmation", SessionID: "session-current", RequestID: "request-current", TurnIndex: 8, State: "terminal", ReasonCode: "pending_reconciliation_failed", TerminalAt: "2026-07-30T04:00:00Z"},
			{QueueKind: "transport_retry", SessionID: "session-current", RequestID: "request-old", TurnIndex: 8, State: "retryable", Attempts: 2, MaxAttempts: 4},
			{QueueKind: "maintenance", SessionID: "session-other", TurnIndex: 3, State: "queued", Count: 2},
		},
	}
	vm := buildDashboardViewModel(req)
	current := requireDashboardCard(t, vm, "current_queue")
	currentRow := requireDashboardRow(t, current, "queue.pending_confirmation")
	if currentRow.Scope != "current_request" || currentRow.Status != "fail" ||
		currentRow.DetailCode != "pending_reconciliation_failed" || currentRow.Time != "2026-07-30T04:00:00Z" {
		t.Fatalf("current queue row=%+v", currentRow)
	}
	historical := requireDashboardCard(t, vm, "historical_queue")
	if got := requireDashboardRow(t, historical, "queueHistory.transport_retry"); got.Scope != "current_session_history" || dashboardInt(got.ItemCount) != 1 {
		t.Fatalf("current-session history row=%+v", got)
	}
	if got := requireDashboardRow(t, historical, "queueHistory.maintenance"); got.Scope != "other_session" || dashboardInt(got.ItemCount) != 2 {
		t.Fatalf("other-session history row=%+v", got)
	}
}

func TestDashboardHistoricalQueuePreservesTerminalAndRetryableStates(t *testing.T) {
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled:    true,
		CurrentSessionID: "session-current",
		RuntimeState:     map[string]any{},
		QueueObservations: []dashboardQueueObservation{
			{QueueKind: "transport_retry", SessionID: "session-current", RequestID: "old-terminal", State: "terminal", ReasonCode: "retry_limit_reached", TerminalAt: "2026-07-30T04:05:00Z", Count: 2},
			{QueueKind: "transport_retry", SessionID: "session-current", RequestID: "old-retryable", State: "retryable", Count: 3},
		},
	})
	card := requireDashboardCard(t, vm, "historical_queue")
	var terminal, retryable *dashboardRow
	for index := range card.Rows {
		row := &card.Rows[index]
		switch row.DetailCode {
		case "retry_limit_reached":
			terminal = row
		case "historical_queue_retryable":
			retryable = row
		}
	}
	if terminal == nil || terminal.Status != "fail" || terminal.Detail != "2 terminal" ||
		terminal.Time != "2026-07-30T04:05:00Z" || dashboardInt(terminal.ItemCount) != 2 {
		t.Fatalf("historical terminal queue=%+v", terminal)
	}
	if retryable == nil || retryable.Status != "notice" || retryable.Detail != "3 retryable" || dashboardInt(retryable.ItemCount) != 3 {
		t.Fatalf("historical retryable queue=%+v", retryable)
	}
}

func TestDashboardPendingRecoveryPreservesTerminalReasonAndTime(t *testing.T) {
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled:    true,
		CurrentSessionID: "session-current",
		RuntimeState:     map[string]any{},
		QueueObservations: []dashboardQueueObservation{
			{
				QueueKind:  "pending_confirmation_recovery",
				SessionID:  "session-current",
				RequestID:  "old-recovery",
				State:      "terminal",
				ReasonCode: "pending_terminal_persistence_failed",
				TerminalAt: "2026-07-30T04:10:00Z",
			},
		},
	})
	row := requireDashboardRow(t, requireDashboardCard(t, vm, "historical_queue"), "queueHistory.pending_confirmation_recovery")
	if row.Status != "fail" || row.DetailCode != "pending_terminal_persistence_failed" ||
		row.Time != "2026-07-30T04:10:00Z" || row.Scope != "current_session_history" {
		t.Fatalf("pending recovery terminal row=%+v", row)
	}
}

func TestDashboardHistoricalQueueGroupsMatchingIncidentsAcrossTimestamps(t *testing.T) {
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled:    true,
		CurrentSessionID: "session-current",
		RuntimeState:     map[string]any{},
		QueueObservations: []dashboardQueueObservation{
			{QueueKind: "transport_retry", SessionID: "session-current", RequestID: "old-1", State: "terminal", ReasonCode: "pending_confirmation_persistence_failed", TerminalAt: "2026-07-30T04:05:00Z", Count: 20},
			{QueueKind: "transport_retry", SessionID: "session-current", RequestID: "old-2", State: "terminal", ReasonCode: "pending_confirmation_persistence_failed", TerminalAt: "2026-07-30T04:10:00Z", Count: 30},
		},
	})
	card := requireDashboardCard(t, vm, "historical_queue")
	matchingRows := 0
	for index := range card.Rows {
		row := &card.Rows[index]
		if row.DetailCode != "pending_confirmation_persistence_failed" {
			continue
		}
		matchingRows++
		if row.Status != "fail" || row.Detail != "50 terminal" || dashboardInt(row.ItemCount) != 50 || row.Time != "2026-07-30T04:10:00Z" {
			t.Fatalf("grouped historical queue row=%+v", row)
		}
	}
	if matchingRows != 1 {
		t.Fatalf("matching historical rows=%d, want 1", matchingRows)
	}
}

func TestDashboardMaintenanceWithoutRequestProvenanceNeverBecomesCurrent(t *testing.T) {
	workflow := turnWorkflowHUDViewModel{
		RequestID: "request-current", ChatSessionID: "session-current", BackendTurn: 8,
		TurnAlignment: turnWorkflowHUDTurnAlignment{HostTurn: 8, BackendTurn: 8, State: "aligned", ReasonCode: "host_backend_turn_aligned"},
	}
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled: true, CurrentSessionID: "session-current", CurrentWorkflowRequestID: "request-current",
		WorkflowSnapshot: &workflow, RuntimeState: map[string]any{},
		QueueObservations: []dashboardQueueObservation{
			{QueueKind: "maintenance", SessionID: "session-current", TurnIndex: 8, State: "queued", Count: 2},
		},
	})
	if findDashboardCard(vm, "current_queue") != nil {
		t.Fatalf("maintenance without exact request provenance became current: %+v", vm.Cards)
	}
	row := requireDashboardRow(t, requireDashboardCard(t, vm, "historical_queue"), "queueHistory.maintenance")
	if row.Scope != "current_session_history" || dashboardInt(row.ItemCount) != 2 {
		t.Fatalf("maintenance history row=%+v", row)
	}
}

func TestDashboardCurrentWorkflowUsesTypedFactsAndTurnAlignment(t *testing.T) {
	workflow := turnWorkflowHUDViewModel{
		RequestID: "request-mismatch", ChatSessionID: "session-current", BackendTurn: 7,
		TurnAlignment: turnWorkflowHUDTurnAlignment{HostTurn: 8, BackendTurn: 7, State: "host_ahead", ReasonCode: "host_turn_ahead_of_backend"},
		Facts: []turnWorkflowHUDFact{
			{Key: "host_observation", Status: "accepted", Disposition: "eligible", ReasonCode: "source_observation_eligible", Severity: turnWorkflowHUDSeverityNormal},
			{Key: "vector_index", Status: "vector_not_configured", Disposition: "dropped", ReasonCode: "vector_not_configured", Severity: turnWorkflowHUDSeverityWarning, Count: intValuePtr(0)},
		},
	}
	vm := buildDashboardViewModel(dashboardViewModelRequest{
		PluginEnabled: true, CurrentSessionID: "session-current", WorkflowSnapshot: &workflow, RuntimeState: map[string]any{},
	})
	card := requireDashboardCard(t, vm, "current_workflow")
	if got := requireDashboardRow(t, card, "turnAlignment"); got.Status != "warn" || got.DetailCode != "host_turn_ahead_of_backend" {
		t.Fatalf("alignment row=%+v", got)
	}
	if got := requireDashboardRow(t, card, "workflowFact.host_observation"); got.Status != "ok" || got.DetailCode != "source_observation_eligible" {
		t.Fatalf("host fact=%+v", got)
	}
	if got := requireDashboardRow(t, card, "workflowFact.vector_index"); got.Status != "warn" || got.DetailCode != "vector_not_configured" || dashboardInt(got.ItemCount) != 0 {
		t.Fatalf("vector fact=%+v", got)
	}
}

func TestDashboardLaneStatusSeparatesQueuedWorkFromDegradation(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "queued", want: "notice"},
		{status: "pending", want: "notice"},
		{status: "delayed", want: "notice"},
		{status: "partial", want: "warn"},
		{status: "degraded", want: "warn"},
		{status: "fallback", want: "warn"},
		{status: "missing_suspected", want: "warn"},
		{status: "failed", want: "fail"},
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			if got := dashboardLaneStatus(test.status, 0); got != test.want {
				t.Fatalf("dashboardLaneStatus(%q)=%q, want %q", test.status, got, test.want)
			}
		})
	}
}

func TestDashboardRealWarningsRemainWarnings(t *testing.T) {
	for _, detail := range []string{
		"complete-turn accepted; critic_extract_failed",
		"rollback partial warning (turn 7): vector cleanup failed",
		"timeout waiting for native afterRequest/active assistant",
	} {
		if got := normalizeDashboardRuntimeStatus(map[string]any{"status": "warn", "detail": detail}); got != "warn" {
			t.Fatalf("real warning %q normalized to %q", detail, got)
		}
	}
}

func TestDashboardConfirmedTurnDeletionIsNotice(t *testing.T) {
	req := dashboardViewModelRequest{
		PluginEnabled: true,
		RuntimeState: map[string]any{
			"lastAutoRollback": map[string]any{
				"status":      "ok",
				"reason_code": "assistant_deleted_output_removed",
				"detail":      "turn 7+ rolled back (assistant_deleted_output_removed)",
				"turnIndex":   7,
			},
		},
	}
	vm := buildDashboardViewModel(req)
	row := requireDashboardRow(t, requireDashboardCard(t, vm, "activity"), "autoRollback")
	if row.Status != "notice" || row.DetailCode != "deletedTurnSynced" || dashboardInt(row.TurnIndex) != 7 {
		t.Fatalf("confirmed deletion=%+v", row)
	}
}

func TestDashboardReferenceCardRequiresBinding(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	server := &Server{Store: fake}
	vm := requestDashboardViewModel(t, server, "session-1")
	if findDashboardCard(vm, "reference") != nil {
		t.Fatalf("reference card must be absent without a binding: %+v", vm.Cards)
	}
	if vm.Summary.Fail != 0 {
		t.Fatalf("unexpected summary without reference binding: %+v", vm.Summary)
	}
}

func TestDashboardReferenceCardFailsWhenVectorUnavailable(t *testing.T) {
	fake := dashboardReferenceBindingStore("session-1")
	server := &Server{Store: fake}
	vm := requestDashboardViewModel(t, server, "session-1")
	card := requireDashboardCard(t, vm, "reference")
	row := requireDashboardRow(t, card, "referenceVector")
	if row.Status != "fail" || row.DetailCode != "referenceVectorUnavailable" || row.ItemCount != float64(1) {
		t.Fatalf("reference row=%+v", row)
	}
}

func TestDashboardReferenceCardFailsWhenCapabilityMissing(t *testing.T) {
	fake := dashboardReferenceBindingStore("session-1")
	server := &Server{
		Store:           fake,
		ReferenceVector: &dashboardCoreVector{VectorStore: &referenceVectorTestStore{}},
	}
	vm := requestDashboardViewModel(t, server, "session-1")
	row := requireDashboardRow(t, requireDashboardCard(t, vm, "reference"), "referenceVector")
	if row.Status != "fail" || row.DetailCode != "referenceVectorExactQueryUnavailable" {
		t.Fatalf("reference row=%+v", row)
	}
}

func TestDashboardReferenceCardOKWhenRuntimeIsReady(t *testing.T) {
	fake := dashboardReferenceBindingStore("session-1")
	server := &Server{
		Store:           fake,
		ReferenceVector: &referenceVectorTestStore{},
		RuntimeConfig: RuntimeConfig{
			Synced:              true,
			EmbeddingProvider:   "openai",
			EmbeddingAPIKey:     "test-key",
			EmbeddingEndpoint:   "https://embedding.invalid/v1",
			EmbeddingModel:      "test-embedding",
			EmbeddingTimeoutSec: 30,
		},
	}
	vm := requestDashboardViewModel(t, server, "session-1")
	card := requireDashboardCard(t, vm, "reference")
	row := requireDashboardRow(t, card, "referenceVector")
	if row.Status != "ok" || row.DetailCode != "referenceReady" || card.Severity != "ok" {
		t.Fatalf("reference card=%+v row=%+v", card, row)
	}
}

type dashboardCoreVector struct {
	vector.VectorStore
}

func dashboardReferenceBindingStore(sessionID string) *referenceBindingHTTPStore {
	fake := newReferenceBindingHTTPStore()
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: sessionID}}
	return fake
}

func requestDashboardViewModel(t *testing.T, server *Server, sessionID string) dashboardViewModel {
	t.Helper()
	body, err := json.Marshal(dashboardViewModelRequest{PluginEnabled: true, CurrentSessionID: sessionID, RuntimeState: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/view-model", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.handleDashboardViewModel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var vm dashboardViewModel
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
		t.Fatal(err)
	}
	return vm
}

func findDashboardCard(vm dashboardViewModel, id string) *dashboardCard {
	for i := range vm.Cards {
		if vm.Cards[i].ID == id {
			return &vm.Cards[i]
		}
	}
	return nil
}

func requireDashboardCard(t *testing.T, vm dashboardViewModel, id string) dashboardCard {
	t.Helper()
	for _, card := range vm.Cards {
		if card.ID == id {
			return card
		}
	}
	t.Fatalf("dashboard card %q missing: %+v", id, vm.Cards)
	return dashboardCard{}
}

func requireDashboardRow(t *testing.T, card dashboardCard, label string) dashboardRow {
	t.Helper()
	for _, row := range card.Rows {
		if row.LabelKey == label {
			return row
		}
	}
	t.Fatalf("dashboard row %q missing: %+v", label, card.Rows)
	return dashboardRow{}
}
