package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	if got := requireDashboardRow(t, saveQueue, "retryQueue"); got.Status != "warn" || got.Detail != "2 pending" {
		t.Fatalf("retry row=%+v", got)
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
