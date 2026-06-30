package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/compare"
)

var (
	pythonBase = flag.String("python-base", "http://127.0.0.1:8000", "Python 0.8 backend base URL")
	goBase     = flag.String("go-base", "http://127.0.0.1:28080", "Go shadow backend base URL")
	out        = flag.String("out", "", "Output file path (empty = stdout)")
	jsonOut    = flag.String("json-out", "", "JSON output file path (empty = none)")
	timeout    = flag.Duration("timeout", 30*time.Second, "Per-request timeout")
	maxDiffs   = flag.Int("max-diffs", 20, "Maximum diffs to report per endpoint")
	sessionID  = flag.String("session-id", "shadow-parity-fake-sid", "Session id used for session-scoped probes; non-default values enable data-backed explorer/KG probe query params.")
)

// probeDef defines a single probe to run.
type probeDef struct {
	method     string
	path       string
	body       []byte
	skipReason string
}

// buildProbes returns the full R1 read-only parity probe set.
// It excludes all R2 write/mutation routes and marks Go-only / 2.0-only
// endpoints with a skipReason.
func buildProbes(fakeSID string) []probeDef {
	dataBackedSession := fakeSID != "shadow-parity-fake-sid"
	explorerQuery := ""
	kgRecallGetQuery := ""
	sessionsComparePath := "/sessions/compare"
	if dataBackedSession {
		explorerQuery = "?chat_session_id=" + fakeSID
		kgRecallGetQuery = "?chat_session_id=" + fakeSID + "&limit=20&offset=0"
		sessionsComparePath = "/sessions/compare?session_ids=" + fakeSID + ",shadow-parity-secondary-sid&preview_limit=3"
	}
	return []probeDef{
		// ---- System / ops ----
		{method: "GET", path: "/health"},
		{method: "GET", path: "/ready", skipReason: "Go-only ops endpoint (not present in Python 0.8)"},
		{method: "GET", path: "/version", skipReason: "Go-only ops endpoint (not present in Python 0.8)"},
		{method: "GET", path: "/stats"},
		{method: "GET", path: "/wakeup"},
		{method: "GET", path: "/sessions"},
		{method: "GET", path: "/audit"},
		{method: "GET", path: "/chroma-shadow/preflight"},
		{method: "POST", path: "/search", body: []byte(`{"user_input":"parity probe","chat_session_id":"` + fakeSID + `","top_k":5}`)},

		// ---- Session-scoped reads ----
		{method: "GET", path: "/active-states/" + fakeSID},
		{method: "GET", path: "/retrieval-index/runtime-config"},
		{method: "GET", path: "/intent-routing/runtime-config"},
		{method: "GET", path: "/retrieval-index/" + fakeSID},
		{method: "GET", path: "/retrieval-index/" + fakeSID + "/source-row?document_id=memory:1"},
		{method: "GET", path: "/sessions/" + fakeSID},
		{method: "GET", path: "/sessions/" + fakeSID + "/export"},
		{method: "GET", path: "/sessions/" + fakeSID + "/guidance-snapshot"},
		{method: "GET", path: "/sessions/" + fakeSID + "/step7-health"},
		{method: "GET", path: "/sessions/" + fakeSID + "/resume-pack?continuity_trigger_mode=resume"},
		{method: "GET", path: sessionsComparePath},
		{method: "GET", path: "/session-state/" + fakeSID},
		{method: "GET", path: "/continuity-pack/" + fakeSID},
		{method: "GET", path: "/pending-threads/" + fakeSID},
		{method: "GET", path: "/canonical-state-layer/" + fakeSID},
		{method: "GET", path: "/narrative-control/" + fakeSID},
		{method: "GET", path: "/momentum-packet/" + fakeSID},
		{method: "GET", path: "/session/" + fakeSID},
		{method: "GET", path: "/long-session-health/" + fakeSID},

		// ---- Explorer routes ----
		{method: "GET", path: "/explorer/" + fakeSID},
		{method: "GET", path: "/explorer/chat_logs" + explorerQuery},
		{method: "GET", path: "/explorer/memories" + explorerQuery},
		{method: "GET", path: "/explorer/direct-evidence" + explorerQuery},
		{method: "GET", path: "/explorer/kg_triples" + explorerQuery},
		{method: "GET", path: "/explorer/chapter_summaries" + explorerQuery},

		// ---- Knowledge graph & world-model reads ----
		{method: "GET", path: "/kg/recall" + kgRecallGetQuery},
		{method: "POST", path: "/kg/recall", body: []byte(`{"chat_session_id":"` + fakeSID + `","entities":["parity","probe"],"limit":20,"current_turn":1}`)},
		{method: "GET", path: "/storylines/" + fakeSID},
		{method: "GET", path: "/world-rules/" + fakeSID},
		{method: "GET", path: "/world-rules/" + fakeSID + "/inherited"},
		{method: "GET", path: "/characters/" + fakeSID},
		{method: "GET", path: "/characters/" + fakeSID + "/test-character"},
		{method: "GET", path: "/characters/" + fakeSID + "/test-character/events"},
		{method: "GET", path: "/episodes/" + fakeSID},
		{method: "GET", path: "/episodes/detail/999999"},

		// ---- Chapters / episodes search ----
		{method: "POST", path: "/chapters/dry-run", body: []byte(`{"chat_session_id":"` + fakeSID + `","turn_index":1}`)},
		{method: "POST", path: "/chapters/search", body: []byte(`{"chat_session_id":"` + fakeSID + `","query":"parity probe","top_k":3}`)},
		{method: "POST", path: "/episodes/search", body: []byte(`{"chat_session_id":"` + fakeSID + `","query":"parity probe","top_k":3}`)},

		// ---- Metrics reads ----
		{method: "GET", path: "/metrics/lc1c/" + fakeSID},
		{method: "GET", path: "/metrics/lc1d/" + fakeSID},
		{method: "GET", path: "/metrics/lc1q/" + fakeSID},
		{method: "GET", path: "/metrics/lc1r/regression-corpus"},
		{method: "GET", path: "/metrics/lc1s/step17-bundle-closure"},
		{method: "GET", path: "/metrics/tm1d/" + fakeSID},

		// ---- Go-only / 2.0-only prompt filesystem ----
		{method: "GET", path: "/prompts", skipReason: "Go-only / 2.0-only prompt filesystem route (not present in Python 0.8)"},
		{method: "GET", path: "/prompts/", skipReason: "Go-only / 2.0-only prompt filesystem route (not present in Python 0.8)"},
	}
}

func main() {
	flag.Parse()
	if err := run(*pythonBase, *goBase, *out, *jsonOut, *timeout, *maxDiffs, *sessionID); err != nil {
		log.Fatal(err)
	}
}

func run(pythonBase, goBase, out, jsonOut string, timeout time.Duration, maxDiffs int, sessionID string) error {
	fakeSID := strings.TrimSpace(sessionID)
	if fakeSID == "" {
		fakeSID = "shadow-parity-fake-sid"
	}
	probes := buildProbes(fakeSID)

	if hasR2Path(probes) {
		return fmt.Errorf("probe set contains R2 write/mutation routes; aborting for safety")
	}

	h := compare.NewHarness(pythonBase, goBase)
	h.HTTPClient.Timeout = timeout

	ctx := context.Background()
	report := &compare.ValueReport{
		Timestamp:   time.Now(),
		PythonBase:  pythonBase,
		GoBase:      goBase,
		MaxDiffs:    maxDiffs,
		Results:     make([]compare.ValueResult, 0, len(probes)),
		SkipReasons: make(map[string]string),
	}

	for _, p := range probes {
		endpoint := p.method + " " + p.path
		if p.skipReason != "" {
			report.Results = append(report.Results, compare.ValueResult{
				Endpoint: endpoint,
				Allowed:  true,
			})
			report.SkipReasons[endpoint] = p.skipReason
			continue
		}
		res := h.ProbeValue(ctx, p.method, p.path, p.body, maxDiffs)
		report.Results = append(report.Results, *res)
	}

	// Markdown output
	var mdWriter *os.File
	if out == "" {
		mdWriter = os.Stdout
	} else {
		f, err := os.Create(out)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		mdWriter = f
	}
	if err := report.WriteMarkdown(mdWriter); err != nil {
		return fmt.Errorf("failed to write markdown report: %w", err)
	}

	// JSON output
	if jsonOut != "" {
		jf, err := os.Create(jsonOut)
		if err != nil {
			return fmt.Errorf("failed to create json output file: %w", err)
		}
		defer jf.Close()
		if err := report.WriteJSON(jf); err != nil {
			return fmt.Errorf("failed to write json report: %w", err)
		}
	}

	return nil
}

// hasR2Path returns true if the probe set contains any known R2
// write/mutation route.
func hasR2Path(probes []probeDef) bool {
	r2Prefixes := []string{
		"POST /complete-turn",
		"POST /prepare-turn",
		"POST /turns",
		"PATCH /session/",
		"POST /config/update",
		"DELETE /rollback",
		"POST /proxy/plugin-main",
	}
	for _, p := range probes {
		endpoint := p.method + " " + p.path
		for _, prefix := range r2Prefixes {
			if strings.HasPrefix(endpoint, prefix) {
				return true
			}
		}
	}
	return false
}
