package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/compare"
)

var (
	pythonBase = flag.String("python-base", "http://127.0.0.1:8000", "Python 0.8 backend base URL")
	goBase     = flag.String("go-base", "http://127.0.0.1:28080", "Go shadow backend base URL")
	out        = flag.String("out", "", "Output file path (empty = stdout)")
	timeout    = flag.Duration("timeout", 30*time.Second, "Per-request timeout")
)

// probeDef defines a single probe to run.
type probeDef struct {
	method     string
	path       string
	body       []byte
	skipReason string
}

func main() {
	flag.Parse()

	fakeSID := "shadow-parity-fake-sid"

	probes := []probeDef{
		{method: "GET", path: "/health"},
		{method: "GET", path: "/ready", skipReason: "Go-only ops endpoint (not present in Python 0.8)"},
		{method: "GET", path: "/version", skipReason: "Go-only ops endpoint (not present in Python 0.8)"},
		{method: "GET", path: "/stats"},
		{method: "GET", path: "/sessions"},
		{method: "GET", path: "/audit"},
		{method: "GET", path: "/chroma-shadow/preflight"},
		{method: "POST", path: "/search", body: []byte(`{"user_input":"parity probe","chat_session_id":"` + fakeSID + `","top_k":5}`)},
		{method: "GET", path: "/active-states/" + fakeSID},
		{method: "GET", path: "/retrieval-index/" + fakeSID},
		{method: "GET", path: "/sessions/" + fakeSID},
		{method: "GET", path: "/sessions/" + fakeSID + "/resume-pack?continuity_trigger_mode=resume"},
		{method: "GET", path: "/explorer/" + fakeSID},
		{method: "GET", path: "/metrics/lc1c/" + fakeSID},
		{method: "GET", path: "/world-rules/" + fakeSID},
		{method: "GET", path: "/characters/" + fakeSID},
		{method: "GET", path: "/storylines/" + fakeSID},
		{method: "GET", path: "/episodes/" + fakeSID},
		{method: "GET", path: "/pending-threads/" + fakeSID},
		{method: "GET", path: "/narrative-control/" + fakeSID},
		{method: "GET", path: "/session-state/" + fakeSID},
		{method: "GET", path: "/canonical-state-layer/" + fakeSID},
		{method: "GET", path: "/continuity-pack/" + fakeSID},
		{method: "GET", path: "/momentum-packet/" + fakeSID},
		{method: "GET", path: "/session/" + fakeSID},
		{method: "GET", path: "/long-session-health/" + fakeSID},
		{method: "GET", path: "/prompts"},
		{method: "GET", path: "/prompts/"},
	}

	h := compare.NewHarness(*pythonBase, *goBase)
	h.HTTPClient.Timeout = *timeout

	ctx := context.Background()
	report := &compare.Report{
		Timestamp:   time.Now(),
		PythonBase:  *pythonBase,
		GoBase:      *goBase,
		Results:     make([]compare.Result, 0, len(probes)),
		SkipReasons: make(map[string]string),
	}

	for _, p := range probes {
		endpoint := p.method + " " + p.path
		if p.skipReason != "" {
			report.Results = append(report.Results, compare.Result{
				Endpoint: endpoint,
				Allowed:  true,
			})
			report.SkipReasons[endpoint] = p.skipReason
			continue
		}
		res := h.Probe(ctx, p.method, p.path, p.body)
		report.Results = append(report.Results, *res)
	}

	var w *os.File
	if *out == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	if err := report.WriteMarkdown(w); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
		os.Exit(1)
	}
}
