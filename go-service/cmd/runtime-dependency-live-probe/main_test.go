package main

import (
	"context"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestRunIsGuardedWithoutExecuteAndDoesNotInvokeProbe(t *testing.T) {
	called := false
	report, exitCode := run([]string{"-chroma-endpoint", "http://127.0.0.1:8000"}, func(context.Context, vector.ChromaRoundTripProbeConfig) (*vector.ChromaRoundTripProbeReport, error) {
		called = true
		return nil, nil
	})
	if exitCode != 2 || report.Status != "guarded" || report.Executed {
		t.Fatalf("unexpected report/code: code=%d report=%+v", exitCode, report)
	}
	if called {
		t.Fatal("probe was invoked without -execute")
	}
}

func TestRunExecuteCallsReusableChromaProbe(t *testing.T) {
	var got vector.ChromaRoundTripProbeConfig
	report, exitCode := run([]string{
		"-execute",
		"-chroma-endpoint", "http://localhost:8000",
		"-chroma-api-path", "/api/v2",
		"-collection-prefix", "windows_gate",
	}, func(_ context.Context, cfg vector.ChromaRoundTripProbeConfig) (*vector.ChromaRoundTripProbeReport, error) {
		got = cfg
		return &vector.ChromaRoundTripProbeReport{
			Contract:      vector.ChromaRoundTripProbeContract,
			Status:        "ok",
			CleanupStatus: "ok",
		}, nil
	})
	if exitCode != 0 || report.Status != "ok" || !report.Executed {
		t.Fatalf("unexpected report/code: code=%d report=%+v", exitCode, report)
	}
	if got.Endpoint != "http://localhost:8000" || got.APIPath != "/api/v2" || got.CollectionPrefix != "windows_gate" {
		t.Fatalf("probe config=%+v", got)
	}
}

func TestRunExecuteRequiresEndpoint(t *testing.T) {
	t.Setenv("AC_CHROMA_ENDPOINT", "")
	called := false
	report, exitCode := run([]string{"-execute", "-chroma-endpoint", ""}, func(context.Context, vector.ChromaRoundTripProbeConfig) (*vector.ChromaRoundTripProbeReport, error) {
		called = true
		return nil, nil
	})
	if exitCode != 2 || report.Status != "failed" || !report.Executed {
		t.Fatalf("unexpected report/code: code=%d report=%+v", exitCode, report)
	}
	if called {
		t.Fatal("probe was invoked without an endpoint")
	}
}
