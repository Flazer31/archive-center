package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyTrackingPortabilityRequest(t *testing.T, server *Server, method, path string, payload any) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	raw, _ := json.Marshal(payload)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestBodyTracking46ExportSettingsRestoreAndMissingDefault(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	server := &Server{Store: &canonicalFakeStore{}}
	if exported := bodyTrackingPortabilityRequest(t, server, http.MethodGet, "/sessions/empty/export", nil); exported["body_tracking_settings"] != nil {
		t.Fatal("missing optional config was invented during export")
	}
	path, _ := bodyTrackingSettingsPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("export created missing config file: %v", err)
	}
	character := defaultBodyCharacterConfig()
	character.EntityID, character.OriginEntityID, character.CharacterName = "character-one", "character-origin", "Mina"
	config := bodyTrackingConfig{CycleTrackingEnabled: true, Characters: []bodyCharacterConfig{character}, SimulationSeed: "stable-import-seed"}
	if err := server.restoreBodyTrackingConfig("original", config); err != nil {
		t.Fatal(err)
	}
	unrelated := bodyTrackingConfig{AutomaticPregnancyEnabled: false, Characters: []bodyCharacterConfig{}, SimulationSeed: "unrelated-seed"}
	if err := server.restoreBodyTrackingConfig("unrelated", unrelated); err != nil {
		t.Fatal(err)
	}
	exported := bodyTrackingPortabilityRequest(t, server, http.MethodGet, "/sessions/original/export", nil)
	snapshot := exported["body_tracking_settings"]
	if snapshot == nil {
		t.Fatal("existing settings missing from export wrapper")
	}
	bodyTrackingPortabilityRequest(t, server, http.MethodPut, "/config/body-tracking/imported", map[string]any{"restore_snapshot": snapshot})
	for sid, want := range map[string]bodyTrackingConfig{"original": config, "imported": config, "unrelated": unrelated} {
		got, found, err := server.storedBodyTrackingConfig(sid)
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Fatalf("settings roundtrip %s: got=%+v want=%+v found=%v err=%v", sid, got, want, found, err)
		}
	}
	missing, err := server.loadBodyTrackingConfig("still-missing")
	if err != nil || missing.CycleTrackingEnabled || missing.AutomaticPregnancyEnabled || missing.SimulationSeed != "" {
		t.Fatalf("missing config did not stay OFF: %+v err=%v", missing, err)
	}
}

func TestBodyTracking46BranchCopiesMappedConfigOnceAndReportsOptionalFailure(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	server := &Server{}
	character := defaultBodyCharacterConfig()
	character.EntityID, character.OriginEntityID = "old-character", "original-character"
	config := bodyTrackingConfig{CycleTrackingEnabled: true, Characters: []bodyCharacterConfig{character}, SimulationSeed: "branch-seed"}
	if err := server.restoreBodyTrackingConfig("source", config); err != nil {
		t.Fatal(err)
	}
	st := &sessionMigrationPreviewStore{chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: "source"}}, completeResult: &store.SessionMigrationCompleteResult{MigrationID: 46, Status: "copied", SourceSessionID: "source", TargetSessionID: "target", EntityIDMap: map[string]string{"old-character": "new-character"}}}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{}}
	request := map[string]string{"source_session_id": "source", "target_session_id": "target"}
	response := performSessionMigrationComplete(t, st, vec, request)
	if response.Blocked || !response.WriteAttempted {
		t.Fatalf("canonical branch blocked: %+v", response)
	}
	got, found, err := server.storedBodyTrackingConfig("target")
	if err != nil || !found || got.SimulationSeed != config.SimulationSeed || got.Characters[0].EntityID != "new-character" || got.Characters[0].OriginEntityID != "original-character" {
		t.Fatalf("branch setting identity/seed mismatch: %+v found=%v err=%v", got, found, err)
	}
	got.CycleTrackingEnabled = false
	if err := server.restoreBodyTrackingConfig("target", got); err != nil {
		t.Fatal(err)
	}
	st.resumeContext = &store.SessionMigrationResumeContext{MigrationID: 46, Status: "copied", SourceSessionID: "source", TargetSessionID: "target"}
	performSessionMigrationComplete(t, st, vec, request)
	resumed, _, _ := server.storedBodyTrackingConfig("target")
	if !reflect.DeepEqual(got, resumed) {
		t.Fatal("resuming branch copy reset target's edited settings")
	}
	path, _ := bodyTrackingSettingsPath()
	if err := os.WriteFile(path, []byte("invalid JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	st.resumeContext = nil
	response = performSessionMigrationComplete(t, st, vec, request)
	if response.Blocked || !response.WriteAttempted || !st.completeCalled {
		t.Fatalf("optional config failure became canonical migration gate: %+v", response)
	}
	if !strings.Contains(strings.Join(response.Warnings, "\n"), "body_tracking_settings_copy_failed") {
		t.Fatalf("optional settings failure undisclosed: %+v", response.Warnings)
	}
}
