package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

const (
	defaultSessionID = "js-route-smoke-session"
	defaultJSONBody  = `{"chat_session_id":"js-route-smoke-session","turn_index":1,"user_input":"smoke","query":"smoke","force":false}`
)

type smokeReport struct {
	Status          string        `json:"status"`
	CheckedAt       string        `json:"checked_at"`
	SessionID       string        `json:"session_id"`
	Scope           string        `json:"scope"`
	Note            string        `json:"note"`
	Summary         smokeSummary  `json:"summary"`
	Routes          []routeResult `json:"routes"`
	OpenGaps        []string      `json:"open_gaps"`
	ProductGateNote string        `json:"product_gate_note"`
}

type smokeSummary struct {
	Total              int            `json:"total"`
	Passed             int            `json:"passed"`
	Failed             int            `json:"failed"`
	NoRouteFailures    int            `json:"no_route_failures"`
	JSONFailures       int            `json:"json_failures"`
	StatusClassCounts  map[string]int `json:"status_class_counts"`
	ImplementationTags map[string]int `json:"implementation_tags"`
	AssetClassCounts   map[string]int `json:"asset_class_counts"`
}

type routeCase struct {
	ID             int
	Name           string
	Method         string
	Path           string
	Body           string
	Tag            string
	ExpectedFields []string
	AllowNoBody    bool
}

type routeResult struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Implementation string   `json:"implementation"`
	AssetClass     string   `json:"asset_class"`
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
	cases := buildRouteCases(sessionID)
	report := smokeReport{
		Status:    "ok",
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		SessionID: sessionID,
		Scope:     "R1 JS adapter exact route variant smoke",
		Note: "Route surface liveness only. This proves the 0.8 JS adapter route variants do not hit 404/405/unhandled 500 in Go 2.0. " +
			"It does not prove behavior parity, MariaDB default cutover, ChromaDB endpoint-backed retrieval, or product readiness.",
		Routes: []routeResult{},
		OpenGaps: []string{
			"Behavior parity remains open for R2-guarded write and mutation surfaces.",
			"MariaDB is not the live default Store in this smoke.",
			"ChromaDB endpoint-backed retrieval is not exercised in this route liveness smoke.",
			"Supervisor, critic, and proxy routes do not call upstream providers in this smoke.",
			"Response body deep parity against the Python 0.8 backend is not measured here.",
		},
		ProductGateNote: "Product gates remain unchanged; this is R1 route surface evidence, not a green cutover gate.",
	}
	report.Summary.StatusClassCounts = map[string]int{}
	report.Summary.ImplementationTags = map[string]int{}
	report.Summary.AssetClassCounts = map[string]int{}

	for _, tc := range cases {
		result := probeRoute(handler, tc)
		report.Routes = append(report.Routes, result)
		report.Summary.Total++
		report.Summary.StatusClassCounts[result.StatusClass]++
		report.Summary.ImplementationTags[result.Implementation]++
		report.Summary.AssetClassCounts[result.AssetClass]++
		if result.Passed {
			report.Summary.Passed++
			continue
		}
		report.Summary.Failed++
		report.Status = "failed"
		if result.Detail == "no_route" {
			report.Summary.NoRouteFailures++
		}
		if result.Detail == "invalid_json" {
			report.Summary.JSONFailures++
		}
	}

	return report
}

func probeRoute(handler http.Handler, tc routeCase) routeResult {
	body := strings.TrimSpace(tc.Body)
	if body == "" && methodNeedsBody(tc.Method) {
		body = defaultJSONBody
	}
	req := httptest.NewRequest(tc.Method, tc.Path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	result := routeResult{
		ID:             tc.ID,
		Name:           tc.Name,
		Method:         tc.Method,
		Path:           tc.Path,
		Implementation: tc.Tag,
		AssetClass:     classifyRouteAsset(tc),
		HTTPStatus:     rec.Code,
		StatusClass:    classifyStatus(rec.Code),
	}

	if rec.Body.Len() == 0 {
		result.JSONValid = tc.AllowNoBody
	} else {
		var decoded map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			result.JSONValid = false
			result.Detail = "invalid_json"
		} else {
			result.JSONValid = true
			result.TopLevelFields = sortedKeys(decoded)
			if missing := missingFields(decoded, tc.ExpectedFields); len(missing) > 0 {
				result.Detail = "missing_expected_fields:" + strings.Join(missing, ",")
			}
		}
	}

	if !safeRouteStatus(rec.Code) {
		result.Detail = "no_route"
	}
	if result.Detail == "" && result.JSONValid {
		result.Passed = true
	}
	if result.Detail == "" && !result.JSONValid {
		result.Detail = "invalid_json"
	}
	return result
}

func classifyRouteAsset(tc routeCase) string {
	text := strings.ToLower(strings.Join([]string{tc.Name, tc.Path, tc.Tag}, " "))
	switch {
	case strings.Contains(text, "dry-run") || strings.Contains(text, "dry_run"):
		return "dry-run"
	case tc.Path == "/turns" || tc.Path == "/turns/complete" || strings.Contains(text, "legacy"):
		return "legacy"
	default:
		return "runtime-smoke"
	}
}

func safeRouteStatus(status int) bool {
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return false
	}
	if status >= http.StatusInternalServerError && status != http.StatusServiceUnavailable {
		return false
	}
	return status >= 200 && status < 600
}

func classifyStatus(status int) string {
	switch status {
	case http.StatusOK:
		return "200_ok"
	case http.StatusNoContent:
		return "204_no_content"
	case http.StatusBadRequest:
		return "400_bad_request"
	case http.StatusForbidden:
		return "403_forbidden"
	case http.StatusNotFound:
		return "404_not_found"
	case http.StatusMethodNotAllowed:
		return "405_method_not_allowed"
	case http.StatusServiceUnavailable:
		return "503_shadow_guard"
	default:
		if status >= 500 {
			return "5xx_unhandled"
		}
		if status >= 400 {
			return "4xx_other"
		}
		return fmt.Sprintf("%d_other", status)
	}
}

func methodNeedsBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		return true
	default:
		return false
	}
}

func missingFields(body map[string]any, fields []string) []string {
	missing := []string{}
	for _, field := range fields {
		if _, ok := body[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func sortedKeys(body map[string]any) []string {
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeReport(report smokeReport, outPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	data = append(data, '\n')
	if outPath == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return err
	}
	return nil
}

func buildRouteCases(sessionID string) []routeCase {
	sid := sessionID
	commonBody := fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"user_input":"smoke","query":"smoke","force":false}`, sid)
	turnBody := fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"user_content":"user smoke","assistant_content":"assistant smoke"}`, sid)
	repairBody := fmt.Sprintf(`{"chat_session_id":%q,"dry_run":true,"entries":[{"turn_index":1,"user_content":"user smoke","assistant_content":"assistant smoke","source":"smoke"}]}`, sid)
	prepareBody := fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"raw_user_input":"smoke","messages":[{"role":"user","content":"smoke"}],"settings":{"apply_mode":"shadow","guide_mode":"off"}}`, sid)
	completeBody := fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"user_content":"user smoke","assistant_content":"assistant smoke","raw_user_input":"smoke"}`, sid)
	searchBody := fmt.Sprintf(`{"chat_session_id":%q,"user_input":"smoke","top_k":3}`, sid)
	kgBody := fmt.Sprintf(`{"chat_session_id":%q,"query":"smoke","top_k":3}`, sid)
	trustBody := `{"pinned":false,"suppressed":false,"user_corrected":false}`
	patchBody := `{"name":"smoke","status":"active","summary":"smoke","value_json":"{}","current_context":"smoke"}`
	proxyBody := `{"endpoint":"https://localhost/v1/chat/completions","model":"smoke-model","messages":[{"role":"user","content":"smoke"}],"api_key":"redacted"}`
	supervisorBody := fmt.Sprintf(`{"chat_session_id":%q,"context_messages":[{"role":"user","content":"smoke"}],"guide_mode":"off","narrative_stance":"balanced"}`, sid)
	criticBody := fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"turn_content":"smoke","context":[{"role":"assistant","content":"smoke"}]}`, sid)
	chromaBody := fmt.Sprintf(`{"chat_session_id":%q,"sample_limit_per_tier":1,"batch_size_per_tier":1,"checkpoint":{},"operator_evidence":{},"retry_rows":[]}`, sid)

	return []routeCase{
		{1, "prepare-turn", http.MethodPost, "/prepare-turn", prepareBody, "R1-shadow", []string{"status"}, false},
		{2, "complete-turn", http.MethodPost, "/complete-turn", completeBody, "R1-shadow", []string{"status"}, false},
		{3, "effective-inputs", http.MethodPost, "/effective-inputs", fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"effective_input":"smoke"}`, sid), "R1-shadow", []string{"status"}, false},
		{4, "turns-complete", http.MethodPost, "/turns/complete", completeBody, "R2-guarded", []string{"status", "code"}, false},
		{5, "turns", http.MethodPost, "/turns", turnBody, "R2-guarded", []string{"status", "code"}, false},
		{6, "turns-repair-replay", http.MethodPost, "/turns/repair-replay", repairBody, "R2-plan", []string{"status", "repair_replay_plan"}, false},
		{7, "rollback", http.MethodDelete, fmt.Sprintf("/rollback/1?chat_session_id=%s&req_source=adapter", sid), "", "R2-plan", []string{"status", "rollback_plan"}, false},
		{8, "kg-recall-get", http.MethodGet, fmt.Sprintf("/kg/recall?chat_session_id=%s&query=smoke&top_k=3", sid), "", "R1-read", []string{"status"}, false},
		{9, "kg-recall-post", http.MethodPost, "/kg/recall", kgBody, "R1-read", []string{"status"}, false},
		{10, "search", http.MethodPost, "/search", searchBody, "R1-read", []string{"items"}, false},
		{11, "explorer-memories", http.MethodGet, fmt.Sprintf("/explorer/memories?chat_session_id=%s", sid), "", "R1-read", []string{"status"}, false},
		{12, "explorer-direct-evidence", http.MethodGet, fmt.Sprintf("/explorer/direct-evidence?chat_session_id=%s", sid), "", "R1-read", []string{"status"}, false},
		{13, "explorer-kg-triples", http.MethodGet, fmt.Sprintf("/explorer/kg_triples?chat_session_id=%s", sid), "", "R1-read", []string{"status"}, false},
		{14, "explorer-chat-logs", http.MethodGet, fmt.Sprintf("/explorer/chat_logs?chat_session_id=%s", sid), "", "R1-read", []string{"status"}, false},
		{15, "explorer-chapter-summaries", http.MethodGet, fmt.Sprintf("/explorer/chapter_summaries?chat_session_id=%s", sid), "", "R1-read", []string{"status"}, false},
		{16, "patch-memory", http.MethodPatch, "/explorer/memories/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{17, "patch-kg-triple", http.MethodPatch, "/explorer/kg_triples/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{18, "patch-evidence-review", http.MethodPatch, "/explorer/direct-evidence/1/review", `{"review_status":"verified","review_note":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{19, "patch-evidence-revalidate", http.MethodPatch, "/explorer/direct-evidence/1/revalidate", `{"reason":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{20, "patch-evidence-tombstone", http.MethodPatch, "/explorer/direct-evidence/1/tombstone", `{"tombstone_reason":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{21, "patch-evidence-supersede", http.MethodPatch, "/explorer/direct-evidence/1/supersede", `{"superseded_by":2,"supersede_reason":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{22, "delete-memory", http.MethodDelete, "/explorer/memories/1", "", "R2-guarded", []string{"status", "code"}, false},
		{23, "post-delete-memory", http.MethodPost, "/explorer/memories/1/delete", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{24, "delete-kg-triple", http.MethodDelete, "/explorer/kg_triples/1", "", "R2-guarded", []string{"status", "code"}, false},
		{25, "post-delete-kg-triple", http.MethodPost, "/explorer/kg_triples/1/delete", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{26, "regenerate-memory", http.MethodPost, "/explorer/memories/regenerate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{27, "storylines-get", http.MethodGet, "/storylines/" + sid, "", "R1-read", []string{"status"}, false},
		{28, "storylines-sync", http.MethodPost, "/storylines/sync", fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"mode":"dry_run","supervisor_result":{}}`, sid), "E-1-dry-run", []string{"status", "mode", "parsed_count"}, false},
		{29, "storyline-patch", http.MethodPatch, "/storylines/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{30, "storyline-trust", http.MethodPatch, "/storylines/1/trust", trustBody, "R2-guarded", []string{"status", "code"}, false},
		{31, "storyline-delete", http.MethodDelete, "/storylines/1", "", "R2-guarded", []string{"status", "code"}, false},
		{32, "world-rules-get", http.MethodGet, "/world-rules/" + sid, "", "R1-read", []string{"status"}, false},
		{33, "world-rules-inherited", http.MethodGet, "/world-rules/" + sid + "/inherited", "", "R1-read", []string{"status"}, false},
		{34, "world-rules-sync", http.MethodPost, "/world-rules/sync", fmt.Sprintf(`{"chat_session_id":%q,"turn_index":1,"supervisor_response":{"section_world":{"rules":[]}}}`, sid), "E-4-live-sync", []string{"status", "mode", "candidate_count"}, false},
		{35, "world-rule-patch", http.MethodPatch, "/world-rules/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{36, "world-rule-trust", http.MethodPatch, "/world-rules/1/trust", trustBody, "R2-guarded", []string{"status", "code"}, false},
		{37, "world-rule-delete", http.MethodDelete, "/world-rules/1", "", "R2-guarded", []string{"status", "code"}, false},
		{38, "session-state", http.MethodGet, "/session-state/" + sid, "", "R1-read", []string{"status"}, false},
		{39, "continuity-pack", http.MethodGet, "/continuity-pack/" + sid, "", "R1-read", []string{"status"}, false},
		{40, "pending-threads", http.MethodGet, "/pending-threads/" + sid + "?status_filter=open", "", "R1-read", []string{"status"}, false},
		{41, "pending-thread-patch", http.MethodPatch, "/pending-threads/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{42, "pending-thread-trust", http.MethodPatch, "/pending-threads/1/trust", trustBody, "R2-guarded", []string{"status", "code"}, false},
		{43, "pending-thread-delete", http.MethodDelete, "/pending-threads/1", "", "R2-guarded", []string{"status", "code"}, false},
		{44, "active-states", http.MethodGet, "/active-states/" + sid + "?state_type=scene", "", "R1-read", []string{"status"}, false},
		{45, "narrative-control", http.MethodGet, "/narrative-control/" + sid, "", "R1-read", []string{"status"}, false},
		{46, "director-patch", http.MethodPatch, "/narrative-control/" + sid + "/director-patch", `{"patch":{"pressure":"normal"}}`, "R2-guarded", []string{"status", "code"}, false},
		{47, "momentum-packet", http.MethodGet, "/momentum-packet/" + sid, "", "R1-read", []string{"status"}, false},
		{48, "characters", http.MethodGet, "/characters/" + sid, "", "R1-read", []string{"status"}, false},
		{49, "character-patch", http.MethodPatch, "/characters/" + sid + "/Chloe", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{50, "character-speech", http.MethodPatch, "/characters/" + sid + "/Chloe/speech", `{"speech_style":"direct"}`, "R2-guarded", []string{"status", "code"}, false},
		{52, "episodes", http.MethodGet, "/episodes/" + sid, "", "R1-read", []string{"status"}, false},
		{53, "episodes-generate", http.MethodPost, "/episodes/generate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{54, "episodes-search", http.MethodPost, "/episodes/search", searchBody, "R1-read", []string{"status"}, false},
		{55, "episodes-regenerate", http.MethodPost, "/episodes/regenerate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{56, "episodes-merge", http.MethodPost, "/episodes/merge", fmt.Sprintf(`{"chat_session_id":%q,"episode_ids":[1,2]}`, sid), "R2-guarded", []string{"status", "code"}, false},
		{57, "chapters-generate", http.MethodPost, "/chapters/generate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{58, "chapters-dry-run", http.MethodPost, "/chapters/dry-run", commonBody, "R1-read", []string{"status"}, false},
		{59, "chapters-search", http.MethodPost, "/chapters/search", searchBody, "R1-read", []string{"status"}, false},
		{60, "episode-patch", http.MethodPatch, "/episodes/1", patchBody, "R2-guarded", []string{"status", "code"}, false},
		{61, "episode-delete", http.MethodDelete, "/episodes/1", "", "R2-guarded", []string{"status", "code"}, false},
		{62, "arcs-generate", http.MethodPost, "/arcs/generate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{63, "sagas-generate", http.MethodPost, "/sagas/generate", commonBody, "R2-guarded", []string{"status", "code"}, false},
		{64, "admin-rescan", http.MethodPost, "/admin/rescan", fmt.Sprintf(`{"chat_session_id":%q,"max_items":1}`, sid), "R2-rescan", []string{"status", "candidate_count", "succeeded", "failed"}, false},
		{65, "admin-reindex", http.MethodPost, "/admin/reindex", fmt.Sprintf(`{"chat_session_id":%q,"max_items":1}`, sid), "RMG-23-audit-only", []string{"status", "audit_written", "reindex_executed"}, false},
		{66, "admin-session-migrate", http.MethodPost, "/admin/session-migrate", fmt.Sprintf(`{"source_session_id":%q,"target_session_id":"%s-target","dry_run":true}`, sid, sid), "R2-guarded", []string{"status", "code"}, false},
		{67, "proxy-plugin-main", http.MethodPost, "/proxy/plugin-main", proxyBody, "R1-shadow", []string{"status", "code"}, false},
		{68, "supervisor", http.MethodPost, "/supervisor", supervisorBody, "R1-shadow", []string{"status", "trace_summary"}, false},
		{69, "critic-test", http.MethodPost, "/critic/test", criticBody, "R1-shadow", []string{"status", "trace_summary"}, false},
		{70, "prompts-get", http.MethodGet, "/prompts/supervisor_system.txt", "", "R1-read", []string{"status"}, false},
		{71, "prompts-put", http.MethodPut, "/prompts/supervisor_system.txt", `{"content":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{72, "config-update", http.MethodPost, "/config/update", `{"operator":"smoke"}`, "R2-runtime-config", []string{"status", "updated", "source"}, false},
		{73, "health", http.MethodGet, "/health", "", "R1-read", []string{"status"}, false},
		{74, "stats", http.MethodGet, "/stats", "", "R1-read", []string{"status"}, false},
		{75, "wakeup", http.MethodGet, "/wakeup", "", "R1-read", []string{"status"}, false},
		{76, "sessions", http.MethodGet, "/sessions", "", "R1-read", []string{"status"}, false},
		{77, "session-guidance-snapshot", http.MethodGet, "/sessions/" + sid + "/guidance-snapshot", "", "R1-read", []string{"status"}, false},
		{78, "sessions-compare", http.MethodGet, "/sessions/compare?limit=3", "", "R1-read", []string{"status"}, false},
		{79, "feedback", http.MethodPost, "/feedback", `{"target_type":"turn","target_id":1,"feedback_value":"accept","feedback_note":"smoke"}`, "R2-guarded", []string{"status", "code"}, false},
		{80, "import-hypamemory", http.MethodPost, "/import/hypamemory", fmt.Sprintf(`{"chat_session_id":%q,"payload":{}}`, sid), "R2-guarded", []string{"status", "code"}, false},
		{81, "metrics-lc1q", http.MethodGet, "/metrics/lc1q/" + sid, "", "R1-read", []string{"status"}, false},
		{82, "metrics-lc1r", http.MethodGet, "/metrics/lc1r/regression-corpus", "", "R1-read", []string{"status"}, false},
		{83, "metrics-lc1s", http.MethodGet, "/metrics/lc1s/step17-bundle-closure", "", "R1-read", []string{"status"}, false},
		{84, "chroma-backfill-dry-run", http.MethodPost, "/chroma-shadow/backfill-dry-run", chromaBody, "R1-read", []string{"status"}, false},
		{85, "chroma-reembed-audit", http.MethodPost, "/chroma-shadow/reembed-audit", chromaBody, "R1-read", []string{"status"}, false},
		{86, "chroma-fallback-runbook", http.MethodPost, "/chroma-shadow/fallback-runbook", chromaBody, "R1-read", []string{"status"}, false},
		{87, "chroma-release-hygiene", http.MethodPost, "/chroma-shadow/release-hygiene", chromaBody, "R1-read", []string{"status"}, false},
		{88, "chroma-visibility-guard", http.MethodPost, "/chroma-shadow/visibility-guard", chromaBody, "R1-read", []string{"status"}, false},
		{89, "chroma-health-probe", http.MethodPost, "/chroma-shadow/health-probe", chromaBody, "R1-read", []string{"status"}, false},
		{90, "chroma-bootstrap", http.MethodPost, "/chroma-shadow/bootstrap", chromaBody, "R2-guarded", []string{"status", "code"}, false},
		{91, "chroma-backfill-batch", http.MethodPost, "/chroma-shadow/backfill-batch", chromaBody, "R2-guarded", []string{"status", "code"}, false},
		{92, "chroma-rebuild-drill", http.MethodPost, "/chroma-shadow/rebuild-drill", chromaBody, "R2-guarded", []string{"status", "code"}, false},
		{93, "chroma-adoption-gate", http.MethodPost, "/chroma-shadow/adoption-gate", chromaBody, "R2-guarded", []string{"status", "code"}, false},
		{96, "ready", http.MethodGet, "/ready", "", "R0-operational", []string{"ready", "checks"}, false},
		{97, "version", http.MethodGet, "/version", "", "R0-operational", []string{"version"}, false},
		{98, "prompts-list", http.MethodGet, "/prompts", "", "R1-read", []string{"status", "items"}, false},
		{99, "rollback-auto-reroll", http.MethodDelete, fmt.Sprintf("/rollback/2?chat_session_id=%s&req_source=auto_rollback", sid), "", "R2-plan", []string{"status", "rollback_plan"}, false},
		{100, "session-delete", http.MethodDelete, "/sessions/" + sid, "", "R2-plan", []string{"status", "deleted", "mutation_enabled"}, false},
	}
}
