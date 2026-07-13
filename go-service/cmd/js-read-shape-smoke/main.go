package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

const defaultSessionID = "js-read-shape-smoke-session"

// smokeReport is the top-level JSON output.
type smokeReport struct {
	Status          string        `json:"status"`
	CheckedAt       string        `json:"checked_at"`
	SessionID       string        `json:"session_id"`
	Scope           string        `json:"scope"`
	Note            string        `json:"note"`
	Summary         smokeSummary  `json:"summary"`
	Routes          []shapeResult `json:"routes"`
	OpenGaps        []string      `json:"open_gaps"`
	ProductGateNote string        `json:"product_gate_note"`
}

type smokeSummary struct {
	Total             int            `json:"total"`
	Passed            int            `json:"passed"`
	Failed            int            `json:"failed"`
	JSONFailures      int            `json:"json_failures"`
	MissingFields     int            `json:"missing_fields"`
	TypeMismatches    int            `json:"type_mismatches"`
	NoRouteFailures   int            `json:"no_route_failures"`
	ServerErrors      int            `json:"server_errors"`
	StatusClassCounts map[string]int `json:"status_class_counts"`
}

// expectedKind maps a field name to a coarse JSON kind:
// "string", "number", "boolean", "array", "object", "null"
type expectedKind struct {
	Field string
	Kind  string
}

type shapeCase struct {
	ID            int
	Name          string
	Method        string
	Path          string
	Body          string
	ExpectedKinds []expectedKind
}

type shapeResult struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	HTTPStatus     int      `json:"http_status"`
	StatusClass    string   `json:"status_class"`
	JSONValid      bool     `json:"json_valid"`
	TopLevelFields []string `json:"top_level_fields,omitempty"`
	Passed         bool     `json:"passed"`
	Detail         string   `json:"detail,omitempty"`
}

func main() {
	out := flag.String("out", "", "Path to write the JSON report. Defaults to stdout.")
	sessionID := flag.String("session-id", defaultSessionID, "Smoke session id used for path/query/body fixtures.")
	flag.Parse()

	report := runSmoke(newRealSmokeHandler(), *sessionID)
	if err := writeReport(report, *out); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	if report.Status != "ok" {
		os.Exit(1)
	}
}

func newRealSmokeHandler() http.Handler {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	server := httpapi.NewServer(cfg)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	return mux
}

func runSmoke(handler http.Handler, sessionID string) smokeReport {
	cases := buildShapeCases(sessionID)
	report := smokeReport{
		Status:    "ok",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		SessionID: sessionID,
		Scope:     "R1 JS adapter read response shape smoke",
		Note: "Read response shape smoke. This proves the 0.8 JS adapter read routes return JSON-parseable responses " +
			"with stable top-level fields and expected primitive types in Go 2.0. " +
			"It does not prove behavior parity, MariaDB default cutover, ChromaDB live retrieval, or product readiness.",
		Routes: []shapeResult{},
		OpenGaps: []string{
			"Behavior parity remains open for R2-guarded write and mutation surfaces.",
			"MariaDB is not the live default Store in this smoke.",
			"Deep field-by-field parity against the Python 0.8 backend is not measured here.",
			"Nested object/array element shapes are not validated in this slice.",
		},
		ProductGateNote: "Product gates remain unchanged; this is R1 JS adapter read-shape evidence, not a green cutover gate.",
	}
	report.Summary.StatusClassCounts = map[string]int{}

	for _, tc := range cases {
		result := probeShape(handler, tc)
		report.Routes = append(report.Routes, result)
		report.Summary.Total++
		report.Summary.StatusClassCounts[result.StatusClass]++
		if result.Passed {
			report.Summary.Passed++
			continue
		}
		report.Summary.Failed++
		report.Status = "failed"
		switch result.Detail {
		case "no_route", "method_not_allowed":
			report.Summary.NoRouteFailures++
		case "json_parse_failure":
			report.Summary.JSONFailures++
		default:
			if strings.HasPrefix(result.Detail, "missing_field:") {
				report.Summary.MissingFields++
			} else if strings.HasPrefix(result.Detail, "type_mismatch:") {
				report.Summary.TypeMismatches++
			} else if strings.HasPrefix(result.Detail, "server_error:") {
				report.Summary.ServerErrors++
			}
		}
	}

	return report
}

func probeShape(handler http.Handler, tc shapeCase) shapeResult {
	body := strings.TrimSpace(tc.Body)
	if body == "" && methodNeedsBody(tc.Method) {
		body = `{"chat_session_id":"` + defaultSessionID + `"}`
	}
	req := httptest.NewRequest(tc.Method, tc.Path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	result := shapeResult{
		ID:          tc.ID,
		Name:        tc.Name,
		Method:      tc.Method,
		Path:        tc.Path,
		HTTPStatus:  rec.Code,
		StatusClass: classifyStatus(rec.Code),
		Passed:      false,
	}

	if rec.Code == http.StatusNotFound && !isAllowedShapeHTTPStatus(tc, rec.Code) {
		result.Detail = "no_route"
		return result
	}
	if rec.Code == http.StatusMethodNotAllowed && !isAllowedShapeHTTPStatus(tc, rec.Code) {
		result.Detail = "method_not_allowed"
		return result
	}
	if rec.Code >= 500 && rec.Code < 600 {
		result.Detail = fmt.Sprintf("server_error:%d", rec.Code)
		return result
	}

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		result.Detail = "json_parse_failure"
		return result
	}
	result.JSONValid = true

	fields := make([]string, 0, len(parsed))
	for k := range parsed {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	result.TopLevelFields = fields

	for _, ek := range tc.ExpectedKinds {
		val, ok := parsed[ek.Field]
		if !ok {
			result.Detail = fmt.Sprintf("missing_field:%s", ek.Field)
			return result
		}
		actual := jsonKind(val)
		if actual != ek.Kind {
			result.Detail = fmt.Sprintf("type_mismatch:%s expected %s got %s", ek.Field, ek.Kind, actual)
			return result
		}
	}

	result.Passed = true
	return result
}

func methodNeedsBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}

func classifyStatus(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "other"
	}
}

// jsonKind returns the coarse JSON kind of a value decoded into an interface{}.
func jsonKind(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return reflect.TypeOf(v).Kind().String()
	}
}

func isAllowedShapeHTTPStatus(tc shapeCase, code int) bool {
	return tc.Name == "session-resume-pack" && code == http.StatusNotFound
}

func writeReport(report smokeReport, outPath string) error {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if outPath == "" {
		fmt.Println(string(b))
		return nil
	}
	return os.WriteFile(outPath, b, 0644)
}

func buildShapeCases(sid string) []shapeCase {
	return []shapeCase{
		{1, "health", http.MethodGet, "/health", "", []expectedKind{{"status", "string"}}},
		{2, "ready", http.MethodGet, "/ready", "", []expectedKind{{"ready", "boolean"}, {"checks", "object"}}},
		{3, "version", http.MethodGet, "/version", "", []expectedKind{{"version", "string"}}},
		{4, "stats", http.MethodGet, "/stats", "", []expectedKind{{"status", "string"}, {"chat_logs", "number"}, {"memories", "number"}, {"kg_triples", "number"}}},
		{5, "wakeup", http.MethodGet, "/wakeup", "", []expectedKind{{"status", "string"}}},
		{6, "prompts-list", http.MethodGet, "/prompts", "", []expectedKind{{"status", "string"}, {"items", "array"}, {"count", "number"}}},
		{7, "prompts-get", http.MethodGet, "/prompts/supervisor_system.txt", "", []expectedKind{{"status", "string"}, {"prompt", "object"}}},
		{8, "search", http.MethodPost, "/search", fmt.Sprintf(`{"chat_session_id":%q,"query":"smoke"}`, sid), []expectedKind{{"items", "array"}, {"total_count", "number"}, {"memory_count", "number"}, {"has_fallback", "boolean"}, {"fallback_count", "number"}, {"injection_text", "string"}}},
		{9, "kg-recall-get", http.MethodGet, "/kg/recall?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"count", "number"}, {"total", "number"}, {"limit", "number"}, {"offset", "number"}, {"has_more", "boolean"}, {"legacy_compat", "boolean"}}},
		{10, "explorer-chat-logs", http.MethodGet, "/explorer/chat_logs?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"total", "number"}, {"has_more", "boolean"}}},
		{11, "explorer-memories", http.MethodGet, "/explorer/memories?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"total", "number"}, {"has_more", "boolean"}}},
		{12, "explorer-direct-evidence", http.MethodGet, "/explorer/direct-evidence?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"total", "number"}, {"has_more", "boolean"}}},
		{13, "explorer-kg-triples", http.MethodGet, "/explorer/kg_triples?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"total", "number"}, {"has_more", "boolean"}}},
		{14, "explorer-chapter-summaries", http.MethodGet, "/explorer/chapter_summaries?chat_session_id=" + sid, "", []expectedKind{{"status", "string"}, {"items", "array"}, {"total", "number"}, {"has_more", "boolean"}}},
		{15, "sessions", http.MethodGet, "/sessions", "", []expectedKind{{"status", "string"}, {"count", "number"}}},
		{16, "sessions-compare", http.MethodGet, "/sessions/compare?session_ids=" + sid + ",smoke-second-sid&limit=3", "", []expectedKind{{"status", "string"}, {"sessions", "object"}}},
		{17, "session-guidance-snapshot", http.MethodGet, "/sessions/" + sid + "/guidance-snapshot", "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"state_status", "string"}, {"last_turn", "number"}, {"story_plan", "object"}, {"director", "object"}, {"compact_records", "array"}, {"generated_at", "string"}, {"warnings", "array"}}},
		{18, "session-step7-health", http.MethodGet, "/sessions/" + sid + "/step7-health", "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"total_turns", "number"}, {"guidance_state", "object"}, {"drift_summary", "object"}, {"compaction_summary", "object"}, {"maintenance_summary", "object"}, {"regression_checks", "object"}, {"generated_at", "string"}, {"warnings", "array"}}},
		{19, "session-resume-pack", http.MethodGet, "/sessions/" + sid + "/resume-pack?continuity_trigger_mode=resume", "", []expectedKind{{"detail", "string"}}},
		{20, "session-state", http.MethodGet, "/session-state/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"snapshot_status", "string"}}},
		{21, "continuity-pack", http.MethodGet, "/continuity-pack/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"pack_status", "string"}, {"skeleton_only", "boolean"}}},
		{22, "pending-threads", http.MethodGet, "/pending-threads/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"hooks", "array"}, {"status_filter", "string"}}},
		{23, "active-states", http.MethodGet, "/active-states/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"states", "array"}}},
		{24, "canonical-state-layer", http.MethodGet, "/canonical-state-layer/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"layers", "array"}}},
		{25, "narrative-control", http.MethodGet, "/narrative-control/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"state_status", "string"}, {"story_plan", "object"}, {"director", "object"}, {"progression_ledger", "object"}, {"story_guidance", "object"}, {"generated_at", "string"}}},
		{26, "momentum-packet", http.MethodGet, "/momentum-packet/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"packet_status", "string"}, {"next_pressure", "array"}, {"payoff_candidates", "array"}, {"tension_to_reuse", "array"}, {"beats_to_avoid", "array"}, {"generated_at", "string"}}},
		{27, "storylines", http.MethodGet, "/storylines/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"storylines", "array"}}},
		{28, "world-rules", http.MethodGet, "/world-rules/" + sid, "", []expectedKind{{"status", "string"}, {"count", "number"}, {"items", "array"}}},
		{29, "world-rules-inherited", http.MethodGet, "/world-rules/" + sid + "/inherited", "", []expectedKind{{"status", "string"}, {"active_scope", "string"}, {"count", "number"}, {"rules", "array"}, {"scope_chain", "array"}}},
		{30, "characters", http.MethodGet, "/characters/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"characters", "array"}, {"omitted_count", "number"}}},
		{31, "episodes", http.MethodGet, "/episodes/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"count", "number"}, {"episodes", "array"}}},
		{32, "chapters-dry-run", http.MethodPost, "/chapters/dry-run", fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1}`, sid), []expectedKind{{"status", "string"}, {"mode", "string"}, {"chat_session_id", "string"}, {"turn_index", "number"}, {"interval", "number"}, {"triggered", "boolean"}, {"interval_check", "object"}, {"blocking_reasons", "array"}, {"warnings", "array"}, {"input_stats", "object"}, {"episode_inputs", "array"}}},
		{33, "chapters-search", http.MethodPost, "/chapters/search", fmt.Sprintf(`{"chat_session_id":%q,"query":"smoke"}`, sid), []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"query", "string"}, {"count", "number"}, {"chapters", "array"}}},
		{34, "episodes-search", http.MethodPost, "/episodes/search", fmt.Sprintf(`{"chat_session_id":%q,"query":"smoke"}`, sid), []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"query", "string"}, {"count", "number"}, {"episodes", "array"}}},
		{35, "metrics-lc1c", http.MethodGet, "/metrics/lc1c/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"memory_footprint", "object"}}},
		{36, "metrics-lc1d", http.MethodGet, "/metrics/lc1d/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"integrity_replay", "object"}}},
		{37, "metrics-lc1q", http.MethodGet, "/metrics/lc1q/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"freshness_lag_summary", "object"}}},
		{38, "metrics-lc1r", http.MethodGet, "/metrics/lc1r/regression-corpus", "", []expectedKind{{"status", "string"}, {"regression_corpus_manifest", "object"}}},
		{39, "metrics-lc1s", http.MethodGet, "/metrics/lc1s/step17-bundle-closure", "", []expectedKind{{"status", "string"}, {"step17_bundle_closure", "object"}}},
		{40, "metrics-tm1d", http.MethodGet, "/metrics/tm1d/" + sid, "", []expectedKind{{"status", "string"}, {"chat_session_id", "string"}, {"truth_maintenance_audit_replay", "object"}}},
	}
}
