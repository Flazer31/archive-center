package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Synthetic scaling changes stored rows/facts, not the production selection
// policy. The long query fixture is independent of any user's private record.
func preparationMeasurementFixture(turns int) prepareTurnAssemblyInput {
	input := preprocessingEfficiencyInput()
	input.Memories, input.ChatLogs = nil, nil
	input.Perspective.CharacterSeeds = nil
	input.Perspective.Selection.SemanticFacts = nil
	input.MaxChars = 36000
	for turn := 1; turn <= turns; turn++ {
		events := []any{}
		for j := 0; j < 20; j++ {
			text := fmt.Sprintf("Mira recorded archive parcel %d at station %d with Rowan.", j, turn)
			events = append(events, map[string]any{"event": text, "visibility": "public"})
			fact := prepareTurnPriorityMemoryFact{Text: fmt.Sprintf("미라는 기록단서%c를 확인하고 로완과 약속한 보관 순서 %d를 기억했다.", rune('가'+j), turn*20+j)}
			seed := prepareTurnPriorityFactSeed{Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", SourceRowID: turn*20 + j,
				SourceTurn: turn, SourceOccurrence: fmt.Sprintf("memory:%d:%d", turn, j), Visibility: "public", Fact: fact}
			if j%2 == 1 {
				seed.Visibility = "owner_private"
				seed.PerspectiveOwner = "Mira"
				seed.AllowedViewers = []string{"Mira"}
			}
			input.Perspective.CharacterSeeds = append(input.Perspective.CharacterSeeds, seed)
			if j%2 == 0 {
				input.Perspective.Selection.SemanticFacts = append(input.Perspective.Selection.SemanticFacts, prepareTurnPrioritySemanticFact{UnitID: fmt.Sprintf("unit-%d-%d", turn, j), SourceTurn: turn, Lane: seed.Lane, Fact: fact, Similarity: .8, SimilaritySource: "cosine", Visibility: "public"})
			}
		}
		summary := fmt.Sprintf("Mira checked archive station %d and Rowan kept the keys.", turn)
		data, _ := json.Marshal(map[string]any{"turn_summary": summary, "narrative_events": events})
		input.Memories = append(input.Memories, store.Memory{ID: int64(turn), ChatSessionID: "measurement", TurnIndex: turn, Importance: 7, SummaryJSON: string(data)})
		input.ChatLogs = append(input.ChatLogs, store.ChatLog{TurnIndex: turn, Role: "assistant", Content: summary})
	}
	input.Perspective.Selection.CurrentTurn = turns + 1
	return input
}

func measuredAssembly(input prepareTurnAssemblyInput, enabled bool) (prepareTurnInjectionAssembly, *prepareTurnMeasurement, time.Duration) {
	var m *prepareTurnMeasurement
	if enabled {
		m = newPrepareTurnMeasurement()
	}
	// Reflection is outside the measured interval and permits the identical test
	// to be compiled against a pre-instrumentation source overlay.
	field := reflect.ValueOf(&input).Elem().FieldByName("Measurement")
	if field.IsValid() {
		field.Set(reflect.ValueOf(m))
	}
	started := time.Now()
	input.Common = prepareTurnCommonAssemblySources(input)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	return out, m, time.Since(started)
}

func assemblyMeasurementIdentity(out *prepareTurnInjectionAssembly) string {
	// Include every source candidate field/score/order, summaries and final plan.
	data, _ := json.Marshal([]any{out.priorityCandidates, out.priorityTurnSummaries, out.MemoryDeliveryPlan})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestPreparationMeasurementPreservesAssembly(t *testing.T) {
	for _, turns := range []int{20, 120} {
		t.Run(fmt.Sprint(turns), func(t *testing.T) {
			before, _, _ := measuredAssembly(preparationMeasurementFixture(turns), false)
			after, m, _ := measuredAssembly(preparationMeasurementFixture(turns), true)
			if !reflect.DeepEqual(before.priorityCandidates, after.priorityCandidates) || !reflect.DeepEqual(before.priorityTurnSummaries, after.priorityTurnSummaries) || !reflect.DeepEqual(before.MemoryDeliveryPlan, after.MemoryDeliveryPlan) {
				t.Fatal("measurement changed candidate identity/score/order or delivery plan")
			}
			if m.counts["candidates.resolved"] == 0 || m.stages["assembly.semantic_link"] == nil || m.stages["assembly.tokenize"] == nil {
				t.Fatalf("missing measured stages: %v", m.snapshot())
			}
			root := m.stages["assembly.initial_candidates"]
			var exclusive time.Duration
			for name, stage := range m.stages {
				if name != "assembly.source_preparation" {
					exclusive += stage.exclusive
				}
				if stage.exclusive < 0 || stage.inclusive < stage.exclusive {
					t.Fatalf("invalid nested duration: %s", name)
				}
			}
			if exclusive != root.inclusive {
				t.Fatalf("exclusive stages must partition root: %s != %s", exclusive, root.inclusive)
			}
			t.Logf("turns=%d final_text_sha256=%v candidate_and_plan_sha256=%s", turns, after.MemoryDeliveryPlan["final_text_sha256"], assemblyMeasurementIdentity(&after))
		})
	}
}

// Opt-in measurement emits observations, not performance pass/fail thresholds.
func TestPreparationMeasurementSamples(t *testing.T) {
	if os.Getenv("AC_MEASURE_PREPARATION") != "1" {
		t.Skip("opt-in local timing run")
	}
	for _, turns := range []int{20, 120} {
		for run := 0; run < 5; run++ {
			for _, enabled := range []bool{run%2 == 0, run%2 != 0} {
				input := preparationMeasurementFixture(turns)
				out, m, elapsed := measuredAssembly(input, enabled)
				record := map[string]any{"kind": "assembly", "turns": turns, "run": run, "instrumented": enabled, "elapsed_ms": durationMilliseconds(elapsed), "final_text_sha256": out.MemoryDeliveryPlan["final_text_sha256"], "candidate_and_plan_sha256": assemblyMeasurementIdentity(&out), "measurement": m.snapshot()}
				data, _ := json.Marshal(record)
				t.Log(string(data))
			}
		}
	}
}

func BenchmarkPreparationMeasurement(b *testing.B) {
	for _, turns := range []int{20, 120} {
		b.Run(fmt.Sprint(turns), func(b *testing.B) {
			input := preparationMeasurementFixture(turns)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, _, _ := measuredAssembly(input, true)
				runtime.KeepAlive(out)
			}
		})
	}
}

func TestPreparationMeasuredReadPreservesError(t *testing.T) {
	want := fmt.Errorf("synthetic partial read")
	m := newPrepareTurnMeasurement()
	rows, err := prepareTurnMeasureRead(m, "db.synthetic", func() ([]store.Memory, error) { return []store.Memory{{SummaryJSON: "한글"}}, want })
	if err != want || len(rows) != 1 || m.counts["db.synthetic.text_bytes"] != int64(len("한글")) || m.counts["db.synthetic.errors"] != 1 {
		t.Fatal("read observation altered error/row/byte semantics")
	}
}
