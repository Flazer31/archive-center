package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type smokeReport struct {
	Status    string        `json:"status"`
	CheckedAt string        `json:"checked_at"`
	SessionID string        `json:"session_id"`
	Note      string        `json:"note"`
	Routes    []routeResult `json:"routes"`
	Error     string        `json:"error,omitempty"`
}

type routeResult struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}

func main() {
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	execute := flag.Bool("execute", false, "Actually run the canonical route smoke session against the Go server stack.")
	out := flag.String("out", "", "Path to write JSON report. Defaults to stdout.")
	sessionID := flag.String("session-id", fmt.Sprintf("canonical-route-smoke-%d", time.Now().UTC().UnixNano()), "Smoke session id.")
	flag.Parse()

	report := buildSmokeReport(*execute, *dsn, *sessionID)
	if report != nil {
		writeReport(report, *out)
		if report.Status == "failed" {
			os.Exit(2)
		}
		return
	}

	handler, closeFn, err := newMariaDBSmokeHandler(*dsn)
	if err != nil {
		writeReport(&smokeReport{
			Status:    "failed",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			SessionID: *sessionID,
			Note:      "R1 manual canonical route smoke; not an authority switch.",
			Error:     err.Error(),
		}, *out)
		os.Exit(1)
	}
	defer closeFn()

	report = runSmoke(handler, *sessionID)
	writeReport(report, *out)
	if report.Status != "ok" {
		os.Exit(1)
	}
}

func newMariaDBSmokeHandler(dsn string) (http.Handler, func(), error) {
	maria, err := store.OpenMariaDB(dsn)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open mariadb store: %w", err)
	}
	closeFn := func() {
		if closer, ok := maria.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if pinger, ok := maria.(store.Pinger); ok {
		if err := pinger.Ping(ctx); err != nil {
			closeFn()
			return nil, func() {}, fmt.Errorf("ping mariadb store: %w", err)
		}
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.MariaDBDSN = dsn

	srv := &httpapi.Server{
		Cfg:     cfg,
		Started: time.Now().UTC(),
		Store:   maria,
		Vector:  vector.NewFakeVectorStore(),
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	return mux, closeFn, nil
}

func buildSmokeReport(execute bool, dsn, sessionID string) *smokeReport {
	now := time.Now().UTC().Format(time.RFC3339)
	if !execute {
		return &smokeReport{
			Status:    "guarded",
			CheckedAt: now,
			SessionID: sessionID,
			Note:      "R1 manual canonical route smoke; not an authority switch.",
		}
	}
	if strings.TrimSpace(dsn) == "" {
		return &smokeReport{
			Status:    "failed",
			CheckedAt: now,
			SessionID: sessionID,
			Note:      "R1 manual canonical route smoke; not an authority switch.",
			Error:     "missing dsn: provide -dsn or AC_MARIADB_DSN",
		}
	}
	return nil
}

func runSmoke(handler http.Handler, sessionID string) *smokeReport {
	turnIndex := 1
	report := &smokeReport{
		Status:    "ok",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		SessionID: sessionID,
		Note:      "R1 manual canonical route smoke; not an authority switch.",
		Routes:    []routeResult{},
	}

	posts := []struct {
		name string
		path string
		body string
	}{
		{"chat-logs", fmt.Sprintf("/canonical/%s/chat-logs", sessionID), fmt.Sprintf(`{"turn_index":%d,"role":"user","content":"smoke"}`, turnIndex)},
		{"effective-inputs", fmt.Sprintf("/canonical/%s/effective-inputs", sessionID), fmt.Sprintf(`{"turn_index":%d,"effective_input":"smoke"}`, turnIndex)},
		{"memories", fmt.Sprintf("/canonical/%s/memories", sessionID), fmt.Sprintf(`{"turn_index":%d,"summary_json":"{}","embedding":"[0.1]","embedding_model":"smoke","importance":0.5,"emotional_boost":0.1,"evidence":"{}","emotional_intensity":0.1,"narrative_significance":0.1,"place_wing":"w","place_room":"r"}`, turnIndex)},
		{"evidence", fmt.Sprintf("/canonical/%s/evidence", sessionID), `{"evidence_kind":"fact_event","evidence_text":"smoke","source_turn_start":1,"source_turn_end":1,"turn_anchor":1,"source_message_ids_json":"[]","source_hash":"h","archive_state":"committed","capture_stage":"smoke","capture_verification":"verified","committed_gate":"shadow","lineage_json":"{}"}`},
		{"kg-triples", fmt.Sprintf("/canonical/%s/kg-triples", sessionID), `{"subject":"s","predicate":"p","object":"o","valid_from":1,"source_turn":1}`},
		{"audit-logs", fmt.Sprintf("/canonical/%s/audit-logs", sessionID), `{"event_type":"smoke","target_type":"session","target_id":1,"summary":"smoke","details_json":"{}","source":"smoke"}`},
		{"critic-feedback", fmt.Sprintf("/canonical/%s/critic-feedback", sessionID), `{"target_type":"memory","target_id":1,"feedback_value":"accept","feedback_note":"smoke","source":"smoke"}`},
		{"character-events", fmt.Sprintf("/canonical/%s/character-events", sessionID), `{"character_name":"c","turn_index":1,"event_type":"smoke","details_json":"{}"}`},
	}

	for _, p := range posts {
		req := httptest.NewRequest(http.MethodPost, p.path, strings.NewReader(p.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		pass, detail := postPassed(rec)
		report.Routes = append(report.Routes, routeResult{
			Name:   p.name,
			Method: "POST",
			Path:   p.path,
			Status: rec.Code,
			Pass:   pass,
			Detail: detail,
		})
		if !pass {
			report.Status = "failed"
		}
	}

	gets := []struct {
		name string
		path string
	}{
		{"chat-logs", fmt.Sprintf("/canonical/%s/chat-logs?from_turn=1&to_turn=1", sessionID)},
		{"effective-inputs", fmt.Sprintf("/canonical/%s/effective-inputs?turn_index=1", sessionID)},
		{"memories", fmt.Sprintf("/canonical/%s/memories?from_turn=1&to_turn=1", sessionID)},
		{"evidence", fmt.Sprintf("/canonical/%s/evidence", sessionID)},
		{"kg-triples", fmt.Sprintf("/canonical/%s/kg-triples", sessionID)},
		{"audit-logs", fmt.Sprintf("/canonical/%s/audit-logs?event_type=smoke&limit=10", sessionID)},
		{"critic-feedback", fmt.Sprintf("/canonical/%s/critic-feedback?target_type=memory&target_id=1", sessionID)},
		{"character-events", fmt.Sprintf("/canonical/%s/character-events?character_name=c", sessionID)},
	}

	for _, g := range gets {
		req := httptest.NewRequest(http.MethodGet, g.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		pass, detail := getPassed(g.name, rec)
		report.Routes = append(report.Routes, routeResult{
			Name:   g.name,
			Method: "GET",
			Path:   g.path,
			Status: rec.Code,
			Pass:   pass,
			Detail: detail,
		})
		if !pass {
			report.Status = "failed"
		}
	}

	return report
}

func postPassed(rec *httptest.ResponseRecorder) (bool, string) {
	if rec.Code != http.StatusOK {
		return false, "status_not_ok"
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		return false, "invalid_json"
	}
	if body["saved"] != true {
		return false, "saved_not_true"
	}
	return true, ""
}

func getPassed(name string, rec *httptest.ResponseRecorder) (bool, string) {
	if rec.Code != http.StatusOK {
		return false, "status_not_ok"
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		return false, "invalid_json"
	}
	if name == "effective-inputs" {
		if body["found"] != true {
			return false, "found_not_true"
		}
		return true, ""
	}
	count, ok := body["count"].(float64)
	if !ok || count <= 0 {
		return false, "count_not_positive"
	}
	return true, ""
}

func writeReport(report *smokeReport, outPath string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
	}
}
