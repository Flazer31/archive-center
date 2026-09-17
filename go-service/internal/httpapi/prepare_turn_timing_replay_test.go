package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"unicode/utf8"
)

// Opt-in replay of an operator-supplied preprocessing export. No source text or
// credentials are embedded here; no provider, database or host is contacted.
type timingReplayGroup struct {
	key      string
	roles    []string
	calls    []multiAgentCall
	settings multiAgentSettings
	wire     string
}

func loadTimingReplay(tb testing.TB) (*multiAgentSelection, []timingReplayGroup) {
	tb.Helper()
	path := os.Getenv("AC_TIMING_REPLAY_INPUT")
	if path == "" {
		tb.Skip("set AC_TIMING_REPLAY_INPUT to an existing preprocessing export")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		tb.Fatal("export has no JSON object")
	}
	var selection multiAgentSelection
	if err := json.NewDecoder(bytes.NewReader(raw[start:])).Decode(&selection); err != nil {
		tb.Fatal(err)
	}
	groups := []timingReplayGroup{}
	positions := map[string]int{}
	for _, role := range selection.Roles {
		for _, call := range role.Calls {
			// JSON export erases Go's []map type used by the live reference resolver.
			for _, key := range []string{"candidates", "turn_summaries", "lorebook_candidates", "related_evidence", "search_evidence"} {
				if value, ok := call.Input[key]; ok {
					items := []map[string]any{}
					for _, item := range outputFidelityLineageSlice(value) {
						items = append(items, mapFromAny(item))
					}
					call.Input[key] = items
				}
			}
			key := call.SharedRequestID
			if key == "" {
				key = fmt.Sprintf("round:%d:%s", call.Round, role.Role)
			}
			index, exists := positions[key]
			if !exists {
				index = len(groups)
				positions[key] = index
				groups = append(groups, timingReplayGroup{key: key, settings: defaultMultiAgentSettings()})
			}
			group := &groups[index]
			group.roles = append(group.roles, role.Role)
			if call.ModelInput != "" {
				group.wire = call.ModelInput
				var packet map[string]any
				if err := json.Unmarshal([]byte(call.ModelInput), &packet); err != nil {
					tb.Fatal(err)
				}
				for _, raw := range outputFidelityLineageSlice(packet["roles"]) {
					assignment := mapFromAny(raw)
					name := extractionStringFromAny(assignment["role"])
					cfg := group.settings.Roles[name]
					cfg.Prompt = extractionStringFromAny(assignment["prompt"])
					cfg.MaxTokens = int64(intFromAny(assignment["output_tokens"], 0))
					group.settings.Roles[name] = cfg
				}
			}
			call.ModelInput = multiAgentModelInput(call.Input, call.Round)
			group.calls = append(group.calls, call)
		}
	}
	if len(groups) == 0 {
		tb.Fatal("export has no calls")
	}
	return &selection, groups
}

func timingReplayWire(group timingReplayGroup) string {
	if len(group.calls) == 1 {
		return multiAgentShareReadingRecords(nil, []byte(multiAgentModelInput(group.calls[0].Input, group.calls[0].Round)))
	}
	return multiAgentGroupedInput(group.calls, group.roles, group.settings)
}

func Test45RecordedPreprocessingReplayParity(t *testing.T) {
	_, groups := loadTimingReplay(t)
	for _, group := range groups {
		t.Run(group.key, func(t *testing.T) {
			var got, want map[string]any
			wire := timingReplayWire(group)
			if err := json.Unmarshal([]byte(wire), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(group.wire), &want); err != nil {
				t.Fatal(err)
			}
			for _, packet := range []map[string]any{got, want} {
				expanded := mapFromAny(expand45Shared(packet, mapFromAny(packet["shared_records"])))
				for key := range packet {
					delete(packet, key)
				}
				for key, value := range expanded {
					packet[key] = value
				}
				delete(packet, "shared_records")
				delete(packet, "shared_records_format")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("expanded reading/refs/order differ from captured wire")
			}
			t.Logf("captured=%d chars compact=%d chars saved=%d; expanded input and recorded selections unchanged", utf8.RuneCountInString(group.wire), utf8.RuneCountInString(wire), utf8.RuneCountInString(group.wire)-utf8.RuneCountInString(wire))
			for _, call := range group.calls {
				parsed := finishMultiAgentCall(call, 200, nil, "")
				if !reflect.DeepEqual(parsed.Result.SelectedIDs, call.Result.SelectedIDs) || !reflect.DeepEqual(parsed.Result.SelectedSummaryIDs, call.Result.SelectedSummaryIDs) {
					t.Fatal("recorded response did not resolve to the same selections")
				}
			}
		})
	}
}

func Benchmark45RecordedPreprocessing(b *testing.B) {
	_, groups := loadTimingReplay(b)
	for _, group := range groups {
		b.Run(group.key+"/role_projection", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for _, call := range group.calls {
					_ = multiAgentModelInput(call.Input, call.Round)
				}
			}
		})
		b.Run(group.key+"/wire_assembly", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				_ = timingReplayWire(group)
			}
		})
		b.Run(group.key+"/response_processing", func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				for _, call := range group.calls {
					_ = finishMultiAgentCall(call, 200, nil, "")
				}
			}
		})
	}
}

// Canonical source rows are not present in the export. Initial candidate and
// final delivery timings therefore use the established synthetic source fixture.
func Benchmark45AssemblyStages(b *testing.B) {
	input := requestProjectionFixture(true)
	b.Run("initial_candidates_synthetic", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = buildPrepareTurnInjectionAssemblyWithBudget(input)
		}
	})
	base := buildPrepareTurnInjectionAssemblyWithBudget(input)
	b.Run("final_delivery_synthetic", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = finalizePrepareTurnPriorityMemoryDeliveryPlan(&base, input.MaxChars, 8, input.BudgetMode, input.Budgets, input.Perspective.Selection)
		}
	})
}
