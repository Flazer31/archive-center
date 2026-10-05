package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type privateEvidenceStitchCapture struct {
	store.Store
	request store.SessionStitchRequest
}

func (s *privateEvidenceStitchCapture) StitchSessions(_ context.Context, req store.SessionStitchRequest) (*store.SessionStitchResult, error) {
	s.request = req
	return nil, errors.New("synthetic stop after capturing callback")
}

func TestPrivateEvidenceReview2ProjectionCallbacks(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const private = "Behind the screen, I admitted that my former name was Neris Vale."
	const public = "The harbor beacon shines blue."
	const crossing = "my former name was Neris Vale. " + public
	ex := map[string]any{"protected_secrets": []any{map[string]any{"owner": "Ilan", "evidence_excerpt": private}}, "evidence_excerpts": []any{crossing, public}}
	stored := memoryAdmissionEvidenceJSON(ex, private+" "+public)
	control := buildPublicMemoryProjection(ex, stored)
	if !control.Eligible || strings.Contains(control.SearchText.Text, "Neris Vale") || !strings.Contains(control.SearchText.Text, public) {
		t.Fatal("stored-scope fixture is not a safe public projection")
	}
	st := &sessionMigrationPreviewStore{
		chatLogs:       []store.ChatLog{{ID: 1, ChatSessionID: "source"}},
		completeResult: &store.SessionMigrationCompleteResult{MigrationID: 1, Status: "copied", SourceSessionID: "source", TargetSessionID: "target", Mode: sessionMigrationModeCopyKeep},
	}
	performSessionMigrationComplete(t, st, &sessionMigrationPreviewVector{counts: map[string]int{}}, map[string]string{"source_session_id": "source", "target_session_id": "target", "mode": sessionMigrationModeCopyKeep})
	if !st.completeCalled || st.completeRequest.RebuildPublicProjection == nil {
		t.Fatal("production migration callback not captured")
	}
	stitch := &privateEvidenceStitchCapture{Store: store.NewNoopStore()}
	srv := &Server{Store: stitch}
	mux := http.NewServeMux()
	srv.registerSessionMigrationRoutes(mux)
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/sessions/stitch", strings.NewReader(`{"source_session_ids":["source"],"target_session_id":"target"}`)))
	if stitch.request.RebuildPublicProjection == nil {
		t.Fatal("production stitch callback not captured")
	}
	for lane, rebuild := range map[string]func(string, string) string{"migration": st.completeRequest.RebuildPublicProjection, "stitch": stitch.request.RebuildPublicProjection} {
		text := rebuild(mustCompactJSON(ex), stored)
		if strings.Contains(text, "Neris Vale") || !strings.Contains(text, public) {
			t.Errorf("%s callback lost available stored scope: %q", lane, text)
		}
	}
}
