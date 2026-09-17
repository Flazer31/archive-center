package httpapi

import (
	"testing"
	"time"
)

func requireBackendTimingStages(t *testing.T, response map[string]any, expectedStages ...string) {
	t.Helper()
	timing, ok := response["backend_timing"].(map[string]any)
	if !ok {
		t.Fatalf("backend_timing is not an object: %#v", response["backend_timing"])
	}
	if total, ok := timing["total_ms"].(float64); !ok || total < 0 {
		t.Fatalf("backend_timing.total_ms = %#v, want non-negative number", timing["total_ms"])
	}
	stages, ok := timing["stages_ms"].(map[string]any)
	if !ok {
		t.Fatalf("backend_timing.stages_ms is not an object: %#v", timing["stages_ms"])
	}
	for _, stage := range expectedStages {
		if value, ok := stages[stage].(float64); !ok || value < 0 {
			t.Fatalf("backend_timing.stages_ms[%q] = %#v, want non-negative number", stage, stages[stage])
		}
	}
}

func TestBackendTimingTraceAccumulatesAndReportsSlowestStage(t *testing.T) {
	trace := newBackendTimingTrace("test.timing.v1")
	trace.addMilliseconds("store_reads", 3.25)
	trace.addMilliseconds("store_reads", 1.75)
	trace.addElapsed("response_assembly", time.Now().Add(-time.Millisecond))

	snapshot := trace.snapshot()
	if snapshot["contract_version"] != "test.timing.v1" {
		t.Fatalf("contract_version = %v", snapshot["contract_version"])
	}
	stages := snapshot["stages_ms"].(map[string]float64)
	if stages["store_reads"] != 5 {
		t.Fatalf("store_reads = %v, want 5", stages["store_reads"])
	}
	if snapshot["slowest_stage"] != "store_reads" {
		t.Fatalf("slowest_stage = %v, want store_reads", snapshot["slowest_stage"])
	}
}
