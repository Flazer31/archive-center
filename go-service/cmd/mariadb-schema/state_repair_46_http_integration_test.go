package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func repair46HTTPJob(t *testing.T, routes http.Handler, request map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	routes.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/session-normalize", bytes.NewReader(body)))
	if response.Code != http.StatusAccepted {
		t.Fatalf("normalize: %d %s", response.Code, response.Body.String())
	}
	var job map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	route := fmt.Sprint(job["poll_route"])
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		response = httptest.NewRecorder()
		routes.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		switch job["status"] {
		case "completed":
			return job["result"].(map[string]any)
		case "partial_error", "failed":
			t.Fatalf("repair job failed: %s", response.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("repair job timed out: %#v", job)
	return nil
}

func TestStateRepair46HTTPMariaDBSummaryOnlyPreviewApplyUndoAndFieldHistory(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid = "repair-http-46"
	server := httpapi.NewServer(config.Config{})
	server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	server.Store = st
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	if _, err := db.Exec(`INSERT INTO chat_logs(chat_session_id,turn_index,role,content) VALUES (?,20,'user','Later scene')`, sid); err != nil {
		t.Fatal(err)
	}
	memory := archiveStore.Memory{ChatSessionID: sid, TurnIndex: 3, SummaryJSON: `{"summary":"The promised museum repair was completed."}`, Importance: .8}
	if err := st.(interface {
		SaveMemory(context.Context, *archiveStore.Memory) error
	}).SaveMemory(ctx, &memory); err != nil {
		t.Fatal(err)
	}
	memoryID := memory.ID
	before := archiveStore.PendingThread{ChatSessionID: sid, ThreadKey: "museum", Description: "Repair the museum", Status: "open", CreatedTurn: 1, SourceTurn: 1, Pinned: true, UserCorrected: true, HookType: "promise", HookMetadataJSON: `{"title":"Repair the museum","lifecycle_key":"museum","confidence":0.9}`, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)}
	if err := st.(interface {
		SavePendingThread(context.Context, *archiveStore.PendingThread) error
	}).SavePendingThread(ctx, &before); err != nil {
		t.Fatal(err)
	}
	storedBefore := state46Pending(t, st, sid)[0]
	request := map[string]any{"chat_session_id": sid, "dry_run": true, "state_repairs": []any{map[string]any{"operation_id": "summary-completion", "pending_thread_key": "museum", "replacement": map[string]any{"transition": "complete", "value": "The promised museum repair was completed."}, "source": map[string]any{"kind": "memory_summary", "id": memoryID, "turn": 3}}}}
	preview := repair46HTTPJob(t, routes, request)
	if preview["ai_calls"] != float64(0) || preview["reindex_requested"] != false {
		t.Fatalf("paid/side work: %#v", preview)
	}
	if got := state46Pending(t, st, sid)[0]; got.Status != "open" {
		t.Fatalf("preview wrote pending: %+v", got)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM status_change_events WHERE chat_session_id=?`, sid).Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview history writes: %d %v", count, err)
	}
	request["dry_run"] = false
	applied := repair46HTTPJob(t, routes, request)
	item := applied["items"].([]any)[0].(map[string]any)
	eventID := int64(item["event_id"].(float64))
	if got := state46Pending(t, st, sid)[0]; got.Status != "resolved" || got.CreatedTurn != storedBefore.CreatedTurn || got.ResolvedTurn != 3 {
		t.Fatalf("summary repair: %+v", got)
	}
	current, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(current) != 1 || current[0].SourceTurn != 3 || archiveStore.StatusCurrentObservationTurn(current[0]) != 20 {
		t.Fatalf("source vs recording: %+v %v", current, err)
	}
	if got := repair46HTTPJob(t, routes, request)["items"].([]any)[0].(map[string]any); got["replayed"] != true {
		t.Fatalf("repair replay: %#v", got)
	}
	repair46HTTPJob(t, routes, map[string]any{"chat_session_id": sid, "state_repairs": []any{map[string]any{"undo_event_id": eventID}}})
	restored := state46Pending(t, st, sid)[0]
	if restored.Status != storedBefore.Status || restored.SourceTurn != storedBefore.SourceTurn || restored.CreatedTurn != storedBefore.CreatedTurn || restored.Pinned != storedBefore.Pinned || restored.UserCorrected != storedBefore.UserCorrected || !restored.CreatedAt.Equal(storedBefore.CreatedAt) || !restored.UpdatedAt.Equal(storedBefore.UpdatedAt) {
		t.Fatalf("undo did not restore exact pending: before=%+v after=%+v", storedBefore, restored)
	}
	current, err = st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(current) != 0 {
		t.Fatalf("undo invented prior current: %+v %v", current, err)
	}
	// Older snapshots have no provenance. The repair pages >200 records and
	// recovers only the continuous observation origin, never an event date.
	for turn := 1; turn <= 205; turn++ {
		if _, err := db.Exec(`INSERT INTO character_states(chat_session_id,character_name,status_json,turn_index) VALUES (?,'Mira','{"rank":"captain"}',?)`, sid, turn); err != nil {
			t.Fatal(err)
		}
	}
	fieldRequest := map[string]any{"chat_session_id": sid, "character_provenance_repairs": []string{"Mira"}, "dry_run": true}
	fieldPreview := repair46HTTPJob(t, routes, fieldRequest)["items"].([]any)[0].(map[string]any)
	if fieldPreview["history_rows"] != float64(205) {
		t.Fatalf("history truncated: %#v", fieldPreview)
	}
	fieldRequest["dry_run"] = false
	fieldApplied := repair46HTTPJob(t, routes, fieldRequest)["items"].([]any)[0].(map[string]any)
	state, err := st.GetCharacterState(ctx, sid, "Mira")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(state.FieldProvenanceJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	field := metadata["fields"].(map[string]any)["/status/rank"].(map[string]any)
	if field["source_turn"] != float64(1) || field["occurrence_time"] != nil {
		t.Fatalf("field date invented/row-promoted: %#v", field)
	}
	repair46HTTPJob(t, routes, map[string]any{"chat_session_id": sid, "character_provenance_undo": []any{map[string]any{"character_name": "Mira", "event_id": fieldApplied["event_id"]}}})
	state, err = st.GetCharacterState(ctx, sid, "Mira")
	if err != nil || state.FieldProvenanceJSON != "" {
		t.Fatalf("field undo failed: %+v %v", state, err)
	}
}
