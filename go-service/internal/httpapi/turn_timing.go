package httpapi

import (
	"math"
	"time"
)

type backendTimingTrace struct {
	contractVersion string
	startedAt       time.Time
	stagesMS        map[string]float64
}

func newBackendTimingTrace(contractVersion string) *backendTimingTrace {
	return &backendTimingTrace{
		contractVersion: contractVersion,
		startedAt:       time.Now(),
		stagesMS:        map[string]float64{},
	}
}

func (t *backendTimingTrace) addElapsed(stage string, startedAt time.Time) {
	if t == nil || stage == "" || startedAt.IsZero() {
		return
	}
	t.addMilliseconds(stage, durationMilliseconds(time.Since(startedAt)))
}

func (t *backendTimingTrace) addMilliseconds(stage string, elapsedMS float64) {
	if t == nil || stage == "" || elapsedMS < 0 {
		return
	}
	if t.stagesMS == nil {
		t.stagesMS = map[string]float64{}
	}
	t.stagesMS[stage] = roundMilliseconds(t.stagesMS[stage] + elapsedMS)
}

func (t *backendTimingTrace) snapshot() map[string]any {
	if t == nil {
		return nil
	}
	stages := make(map[string]float64, len(t.stagesMS))
	slowestStage := ""
	slowestMS := 0.0
	for stage, elapsedMS := range t.stagesMS {
		stages[stage] = elapsedMS
		if elapsedMS > slowestMS {
			slowestStage = stage
			slowestMS = elapsedMS
		}
	}
	return map[string]any{
		"contract_version": t.contractVersion,
		"total_ms":         durationMilliseconds(time.Since(t.startedAt)),
		"stages_ms":        stages,
		"slowest_stage":    slowestStage,
		"slowest_ms":       roundMilliseconds(slowestMS),
	}
}

func durationMilliseconds(d time.Duration) float64 {
	return roundMilliseconds(float64(d) / float64(time.Millisecond))
}

func roundMilliseconds(value float64) float64 {
	return math.Round(value*1000) / 1000
}
