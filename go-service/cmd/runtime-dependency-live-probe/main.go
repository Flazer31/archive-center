// runtime-dependency-live-probe runs explicit, mutating dependency probes.
// Without -execute it only emits a guarded JSON report.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const runtimeDependencyProbeContract = "archive-center.runtime-dependency-live-probe.v1"

type runtimeDependencyProbeReport struct {
	Contract    string                             `json:"contract"`
	Status      string                             `json:"status"`
	Executed    bool                               `json:"executed"`
	GeneratedAt string                             `json:"generated_at"`
	Chroma      *vector.ChromaRoundTripProbeReport `json:"chroma,omitempty"`
	Errors      []string                           `json:"errors,omitempty"`
}

type chromaProbeRunner func(context.Context, vector.ChromaRoundTripProbeConfig) (*vector.ChromaRoundTripProbeReport, error)

func main() {
	report, exitCode := run(os.Args[1:], vector.RunChromaRoundTripProbe)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode live probe report: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	_, _ = os.Stdout.Write(data)
	os.Exit(exitCode)
}

func run(args []string, runner chromaProbeRunner) (*runtimeDependencyProbeReport, int) {
	report := &runtimeDependencyProbeReport{
		Contract:    runtimeDependencyProbeContract,
		Status:      "guarded",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	flags := flag.NewFlagSet("runtime-dependency-live-probe", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	execute := flags.Bool("execute", false, "Required to run the mutating live probe.")
	endpoint := flags.String("chroma-endpoint", os.Getenv("AC_CHROMA_ENDPOINT"), "ChromaDB endpoint. Defaults to AC_CHROMA_ENDPOINT.")
	apiPath := flags.String("chroma-api-path", firstNonEmpty(os.Getenv("AC_CHROMA_API_PATH"), "/api/v2"), "ChromaDB API path.")
	collectionPrefix := flags.String("collection-prefix", "archive_center_windows_probe", "Temporary ChromaDB collection prefix.")
	timeout := flags.Duration("timeout", 0, "Overall probe timeout (0 = no local deadline).")
	if err := flags.Parse(args); err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, "invalid arguments")
		return report, 2
	}
	if !*execute {
		report.Errors = append(report.Errors, "-execute is required before the live probe mutates ChromaDB")
		return report, 2
	}
	report.Executed = true
	if strings.TrimSpace(*endpoint) == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "missing ChromaDB endpoint: provide -chroma-endpoint or AC_CHROMA_ENDPOINT")
		return report, 2
	}
	if *timeout < 0 {
		report.Status = "failed"
		report.Errors = append(report.Errors, "timeout must not be negative")
		return report, 2
	}

	ctx := context.Background()
	cancel := func() {}
	if *timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
	}
	defer cancel()
	chromaReport, err := runner(ctx, vector.ChromaRoundTripProbeConfig{
		Endpoint:         *endpoint,
		APIPath:          *apiPath,
		CollectionPrefix: *collectionPrefix,
	})
	report.Chroma = chromaReport
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		return report, 1
	}
	report.Status = "ok"
	return report, 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
