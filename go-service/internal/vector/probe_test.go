package vector

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type chromaProbeServerState struct {
	mu             sync.Mutex
	collectionName string
	collectionID   string
	documentID     string
	sessionID      string
	documentText   string
	deleted        bool
	reset          bool
	wrongSearchID  bool
	requests       []string
}

func newChromaProbeServer(t *testing.T, state *chromaProbeServerState) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.requests = append(state.requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		collectionBase := "/api/v2/tenants/default_tenant/databases/default_database/collections"
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, collectionBase+"/") && state.collectionID == "":
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == collectionBase:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create: %v", err)
				http.Error(w, "decode", http.StatusBadRequest)
				return
			}
			state.collectionName, _ = body["name"].(string)
			state.collectionID = "probe-collection-id"
			_, _ = w.Write([]byte(`{"id":"probe-collection-id","name":"` + state.collectionName + `"}`))
		case r.Method == http.MethodPost && r.URL.Path == collectionBase+"/"+state.collectionID+"/upsert":
			var body struct {
				IDs       []string         `json:"ids"`
				Documents []string         `json:"documents"`
				Metadatas []map[string]any `json:"metadatas"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode upsert: %v", err)
				http.Error(w, "decode", http.StatusBadRequest)
				return
			}
			state.documentID = body.IDs[0]
			state.documentText = body.Documents[0]
			state.sessionID, _ = body.Metadatas[0]["chat_session_id"].(string)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == collectionBase+"/"+state.collectionID+"/get":
			if state.deleted {
				_, _ = w.Write([]byte(`{"ids":[],"documents":[],"metadatas":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"ids":["` + state.documentID + `"],"documents":["` + state.documentText + `"],"metadatas":[{"chat_session_id":"` + state.sessionID + `","tier":"memory","source_table":"runtime_dependency_probe"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == collectionBase+"/"+state.collectionID+"/query":
			id := state.documentID
			if state.wrongSearchID {
				id = "wrong-document"
			}
			_, _ = w.Write([]byte(`{"ids":[["` + id + `"]],"documents":[["` + state.documentText + `"]],"metadatas":[[{"chat_session_id":"` + state.sessionID + `","tier":"memory"}]],"distances":[[0.0]],"embeddings":[[[0.125,0.5,0.875]]]}`))
		case r.Method == http.MethodPost && r.URL.Path == collectionBase+"/"+state.collectionID+"/delete":
			state.deleted = true
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodDelete && r.URL.Path == collectionBase+"/"+state.collectionName:
			state.reset = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
}

func TestRunChromaRoundTripProbeUsesProductionOperationsAndCleansCollection(t *testing.T) {
	state := &chromaProbeServerState{}
	server := newChromaProbeServer(t, state)
	defer server.Close()

	report, err := RunChromaRoundTripProbe(context.Background(), ChromaRoundTripProbeConfig{
		Endpoint: server.URL,
		APIPath:  "/api/v2",
		RunID:    "fixed-run",
	})
	if err != nil {
		t.Fatalf("RunChromaRoundTripProbe: %v report=%+v", err, report)
	}
	if report.Status != "ok" || !report.ExplicitMutation || report.CleanupStatus != "ok" {
		t.Fatalf("unexpected report: %+v", report)
	}
	wantStages := []string{"upsert", "get_readback", "search_readback", "count", "delete_document", "count_after_delete", "cleanup_collection"}
	if len(report.Stages) != len(wantStages) {
		t.Fatalf("stages=%+v", report.Stages)
	}
	for i, want := range wantStages {
		if report.Stages[i].Name != want || report.Stages[i].Status != "ok" {
			t.Fatalf("stage %d=%+v want name=%s status=ok", i, report.Stages[i], want)
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.deleted || !state.reset {
		t.Fatalf("cleanup state: deleted=%v reset=%v", state.deleted, state.reset)
	}
	joined := strings.Join(state.requests, "\n")
	for _, operation := range []string{"/upsert", "/get", "/query", "/delete"} {
		if !strings.Contains(joined, operation) {
			t.Fatalf("missing production operation %s in:\n%s", operation, joined)
		}
	}
}

func TestRunChromaRoundTripProbeSearchMismatchStillCleansCollection(t *testing.T) {
	state := &chromaProbeServerState{wrongSearchID: true}
	server := newChromaProbeServer(t, state)
	defer server.Close()

	report, err := RunChromaRoundTripProbe(context.Background(), ChromaRoundTripProbeConfig{
		Endpoint: server.URL,
		APIPath:  "/api/v2",
		RunID:    "negative-run",
	})
	if err == nil {
		t.Fatal("expected exact search mismatch")
	}
	var typed *ChromaRoundTripProbeError
	if !errors.As(err, &typed) || typed.Stage != "search_readback" || typed.Operation != "chroma.query_exact" {
		t.Fatalf("typed error=%#v", err)
	}
	if report.Status != "failed" || report.CleanupStatus != "ok" {
		t.Fatalf("unexpected failure report: %+v", report)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.reset {
		t.Fatal("temporary collection was not cleaned after failed search readback")
	}
}

func TestRunChromaRoundTripProbeRejectsMissingEndpointWithoutMutation(t *testing.T) {
	report, err := RunChromaRoundTripProbe(context.Background(), ChromaRoundTripProbeConfig{RunID: "guarded"})
	if err == nil {
		t.Fatal("expected missing endpoint failure")
	}
	if report.Status != "failed" || report.CleanupStatus != "not_created" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunChromaRoundTripProbeCleanupUsesParentContextWithoutSuppliedTimeout(t *testing.T) {
	state := &chromaProbeServerState{}
	server := newChromaProbeServer(t, state)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := RunChromaRoundTripProbe(ctx, ChromaRoundTripProbeConfig{
		Endpoint: server.URL,
		APIPath:  "/api/v2",
		RunID:    "cancelled-parent",
	})
	if err == nil {
		t.Fatal("expected cancelled parent context failure")
	}
	if report.CleanupStatus != "failed" {
		t.Fatalf("cleanup escaped cancelled parent without a caller timeout: %+v", report)
	}
	if len(report.Stages) < 2 || report.Stages[len(report.Stages)-1].Name != "cleanup_collection" ||
		report.Stages[len(report.Stages)-1].Status != "failed" {
		t.Fatalf("cleanup stage=%+v", report.Stages)
	}
}
