package httpapi

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestScheduleDateTimeStringsKeepKnownPrecision(t *testing.T) {
	const date = "2900-09-06"
	current := map[string]any{"absolute": map[string]any{"date": date, "time": "13:15"}}
	for _, tc := range []struct{ text, clock string }{
		{date + " 정오", "12:00"},
		{date + " noon", "12:00"},
		{date + " 正午", "12:00"},
		{date + " 자정", "00:00"},
		{date + " midnight", "00:00"},
		{date + " 12:00", "12:00"},
		{date + "T12:00:30", "12:00:30"},
		{date + " 12:00 PM", "12:00"},
		{date + " 12 AM", "00:00"},
		{date + " 오후 2:30", "14:30"},
		{date + " 午前 9:30", "09:30"},
		{"  " + date + "\t정오  ", "12:00"},
	} {
		for _, key := range []string{"due", "next_due"} {
			t.Run(key+"/"+tc.text, func(t *testing.T) {
				schedule := map[string]any{"kind": "recurring", key: tc.text, "last_fulfilled": map[string]any{"date": "2900-09-01"}, "recurrence": map[string]any{"unit": "day", "interval": 1}}
				if key == "next_due" {
					schedule["due"] = "2900-08-01"
				}
				before := mustCompactJSON(schedule)
				reading := buildCommitmentScheduleReading(map[string]any{"schedule": schedule}, current)
				expected := storyTimeRelation(map[string]any{"date": date, "time": tc.clock}, current)
				relation := mapFromAny(reading["due_relation"])
				if relation["relation"] != expected["relation"] || relation["precision"] != "instant" {
					t.Fatalf("stored time was not read at its explicit precision: %#v", reading)
				}
				for _, bound := range []string{"min", "max"} {
					got, ok := storyClockNumeric(mapFromAny(relation["elapsed_days"])[bound])
					want, _ := storyClockNumeric(mapFromAny(expected["elapsed_days"])[bound])
					if !ok || math.Abs(got-want) > 1e-9 {
						t.Fatalf("wrong elapsed time: got=%#v want=%#v", relation, expected)
					}
				}
				text := storyTimePromptSchedule(parseJSONMap(mustCompactJSON(reading)))
				if !strings.Contains(text, "due="+date+" "+tc.clock) {
					t.Fatalf("calculation and displayed date disagree: %s", text)
				}
				if mustCompactJSON(schedule) != before || reading[key] != tc.text || reading["next_due_estimate"] != nil || reading["outcome"] != nil || reading["status"] != nil {
					t.Fatalf("source changed or outcome/recurrence invented: %#v", reading)
				}
			})
		}
	}
}

func TestScheduleDateWithUncertainTimeKeepsDateAndWording(t *testing.T) {
	for _, suffix := range []string{"정오 무렵", "오후", "early morning", "正午ごろ", "25:90", "12:00 or 14:00"} {
		t.Run(suffix, func(t *testing.T) {
			raw := "2900-09-06 " + suffix
			reading := buildCommitmentScheduleReading(map[string]any{"due": raw}, map[string]any{"date": "2900-09-07", "time": "13:15"})
			relation := mapFromAny(reading["due_relation"])
			text := storyTimePromptSchedule(reading)
			if relation["precision"] != "day" || mapFromAny(relation["elapsed_days"])["min"] != float64(1) || !containsAll(text, "due=2900-09-06 (date only)", suffix) {
				t.Fatalf("lost known date/unknown-time wording: %s", text)
			}
			if reading["due"] != raw || reading["outcome"] != nil || strings.Contains(text, "due=2900-09-06 12:00") {
				t.Fatalf("invented exact time or outcome: %#v", reading)
			}
		})
	}
	for _, raw := range []string{"2900-02-30 정오", "내일 정오", "2900-09 정오", "2900-09-060 정오"} {
		if got := storyTimeScheduleDate(raw); len(got) != 0 {
			t.Errorf("invented date for %q: %#v", raw, got)
		}
	}
	stamp := "2900-09-06T12:00:00+09:00"
	reading := buildCommitmentScheduleReading(map[string]any{"due": stamp}, map[string]any{"datetime": "2900-09-06T04:15:00Z"})
	if !containsAll(storyTimePromptSchedule(reading), stamp, "1h 15m before reference") || reading["due"] != stamp {
		t.Fatalf("explicit timezone not preserved: %#v", reading)
	}
}

func TestScheduleDateStringReadingAndRendering(t *testing.T) {
	dueDate, referenceDate := "2900-08-19", "2900-08-22"
	dueTime, _ := time.Parse("2006-01-02", dueDate)
	referenceTime, _ := time.Parse("2006-01-02", referenceDate)
	wantDays := referenceTime.Sub(dueTime).Hours() / 24
	current := map[string]any{"absolute": map[string]any{"date": referenceDate, "time": "17:40"}}
	for _, field := range []string{"due", "next_due"} {
		for _, value := range []any{dueDate, " " + dueDate + " ", map[string]any{"date": dueDate}, map[string]any{"absolute": map[string]any{"date": dueDate}}} {
			t.Run(field+"/"+mustCompactJSON(value), func(t *testing.T) {
				schedule := map[string]any{"kind": "recurring", field: value,
					"last_fulfilled": map[string]any{"date": "2900-08-01"},
					"recurrence":     map[string]any{"unit": "day", "interval": 1}}
				if field == "next_due" {
					schedule["due"] = map[string]any{"date": "2900-07-01"}
				}
				source := map[string]any{"schedule": schedule, "lifecycle_transition": "set"}
				before := mustCompactJSON(source)
				reading := buildCommitmentScheduleReading(source, current)
				relation := mapFromAny(reading["due_relation"])
				span := mapFromAny(relation["elapsed_days"])
				if relation["relation"] != "past" || relation["precision"] != "day" || span["min"] != wantDays || span["max"] != wantDays {
					t.Fatalf("explicit date not read at its supplied precision: %#v", reading)
				}
				// Exercise the JSON handoff used by shared memory parts, not just date math.
				text := storyTimePromptSchedule(parseJSONMap(mustCompactJSON(reading)))
				if !strings.Contains(text, "due="+dueDate+" (date only)") || !strings.Contains(text, fmt.Sprintf("%g calendar days before reference", wantDays)) {
					t.Fatalf("date missing from rendered reading: %s", text)
				}
				if reading["next_due_estimate"] != nil || reading["outcome"] != nil || reading["status"] != nil || reading["lifecycle_transition"] != "set" || reading["due_cue"] != "due_passed_outcome_unknown" {
					t.Fatalf("explicit date triggered rescheduling or outcome inference: %#v", reading)
				}
				if mustCompactJSON(source) != before || mustCompactJSON(reading[field]) != mustCompactJSON(value) {
					t.Fatal("read-only projection rewrote retained source date")
				}
			})
		}
	}
}

func TestScheduleDateStringReachesGoAndPreprocessing(t *testing.T) {
	memory, current := lifecycle46Fixture("schedule-date-reading")
	_, clock := temporal46Fixture()
	payload := parseJSONMap(current.ValueJSON)
	payload["lifecycle_details"] = map[string]any{"schedule": map[string]any{"kind": "one_off", "due": "2026-03-08"}}
	current.ValueJSON = mustCompactJSON(payload)
	before := current.ValueJSON
	input := temporal46AssemblyInput(memory, clock)
	input.UserInput = "Mira remembers the compass voyage promise."
	input.Perspective.Selection.Query = input.UserInput
	input.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, summaries := multiAgentCandidatePool(&out)
	if len(summaries) != 1 || summaries[0].Minimum == nil {
		t.Fatal("schedule-bearing source disappeared")
	}
	for name, text := range map[string]string{
		"summary":       summaries[0].Minimum.Text,
		"go":            extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]),
		"preprocessing": mustCompactJSON(multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), input.MaxChars, 1, nil)),
	} {
		for _, want := range []string{"due=2026-03-08 (date only)", "7 calendar days before reference", "due_passed_with_explicit_outcome"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s omitted schedule reading %q", name, want)
			}
		}
	}
	if current.ValueJSON != before {
		t.Fatal("delivery rewrote lifecycle storage")
	}
	assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars)
}

func TestScheduleDateRemainsOptionalAndUncertain(t *testing.T) {
	for _, schedule := range []map[string]any{
		{"kind": "standing", "condition": "protect the companion"},
		{"kind": "conditional", "condition": "when the ship returns"},
		{"kind": "one_off", "due": ""},
		{"kind": "one_off", "due": "tomorrow"},
		{"kind": "one_off", "due": "2900-08"},
		{"kind": "one_off", "due": "2900-02-30"},
		{"kind": "one_off", "due": map[string]any{"absolute": map[string]any{"date": "2900-08-19"}, "calendar": map[string]any{"id": "lunar"}}},
	} {
		t.Run(mustCompactJSON(schedule), func(t *testing.T) {
			before := mustCompactJSON(schedule)
			reading := buildCommitmentScheduleReading(schedule, map[string]any{"date": "2900-08-22"})
			if mapFromAny(reading["due_relation"])["relation"] != "unknown" || reading["due_cue"] != nil || reading["outcome"] != nil || reading["status"] != nil {
				t.Fatalf("missing date/calendar basis became a date or outcome: %#v", reading)
			}
			if !strings.Contains(storyTimePromptSchedule(reading), "distance unknown") || mustCompactJSON(schedule) != before {
				t.Fatal("uncertainty or original source lost")
			}
			if _, supplied := schedule["due"]; !supplied {
				if _, added := reading["due"]; added {
					t.Fatal("optional due was fabricated")
				}
			}
		})
	}
}

func Test46StoryTimeReadingSourceRelativeAndLongRecall(t *testing.T) {
	observed := map[string]any{"story_clock": map[string]any{"absolute": map[string]any{"date": "2020-02-28"}}}
	context := map[string]any{"observed_at": observed, "relative_expression": "내일", "relative": map[string]any{"anchor": "source_observation", "offset": 1, "unit": "day"}}
	original := mustCompactJSON(context)
	for _, current := range []string{"2020-02-29", "2020-03-02", "2020-05-02", "2024-03-01"} {
		t.Run(current, func(t *testing.T) {
			reading := buildStoryTimeReading(context, map[string]any{"absolute": map[string]any{"date": current}})
			if reading["relative_expression"] != "내일" || mapFromAny(reading["resolved_occurrence_time"])["date"] != "2020-02-29" {
				t.Fatalf("source expression or immutable anchor lost: %#v", reading)
			}
			relation := mapFromAny(reading["current_relation"])
			date, _ := time.Parse("2006-01-02", current)
			event, _ := time.Parse("2006-01-02", "2020-02-29")
			wantDays := date.Sub(event).Hours() / 24
			if mapFromAny(relation["elapsed_days"])["min"] != wantDays || mapFromAny(relation["elapsed_days"])["max"] != wantDays {
				t.Fatalf("elapsed date derived from wrong anchor: %#v", reading)
			}
		})
	}
	if mustCompactJSON(context) != original {
		t.Fatal("reading mutated persisted source")
	}
}

func Test46StoryTimeUnknownOccurrenceDoesNotBorrowMentionOrPCDate(t *testing.T) {
	for _, context := range []map[string]any{
		{"relative_expression": "yesterday"},
		{"observed_at": map[string]any{"date": "2020-01-01"}},
		{"relative_expression": "tomorrow", "observed_at": map[string]any{"date": "2020-01-01"}},
		{"relative": map[string]any{"offset": 1, "unit": "day", "anchor": "current_story_clock"}},
	} {
		reading := buildStoryTimeReading(context, map[string]any{"date": "2026-09-17", "turn_index": 500})
		if mapFromAny(reading["current_relation"])["relation"] != "unknown" {
			t.Fatalf("unknown event date invented: %#v", reading)
		}
	}
	reading := buildStoryTimeReading(map[string]any{"occurrence_time": map[string]any{"date": "2020-01-01"}}, map[string]any{"turn_index": 900})
	if mapFromAny(reading["current_relation"])["relation"] != "unknown" {
		t.Fatalf("turn count supplied a story date: %#v", reading)
	}
}

func Test46StoryTimeBoundsAndFictionalCalendar(t *testing.T) {
	calendar := func(id string, day int) map[string]any {
		return map[string]any{"calendar": map[string]any{"id": id, "day_index": day, "label": "Moon Feast"}}
	}
	cases := []struct {
		name           string
		event, current map[string]any
		relation       string
		min, max       float64
	}{
		{"bounded", map[string]any{"range": map[string]any{"start": map[string]any{"date": "2020-01-02"}, "end": map[string]any{"date": "2020-01-04"}}}, map[string]any{"date": "2020-01-10"}, "past", 6, 8},
		{"overlap", map[string]any{"range": map[string]any{"start": map[string]any{"date": "2020-01-02"}, "end": map[string]any{"date": "2020-01-04"}}}, map[string]any{"date": "2020-01-03"}, "overlapping_range", -1, 1},
		{"fantasy", calendar("lunar", 40), calendar("lunar", 50), "past", 10, 10},
		{"different_calendars", calendar("lunar", 40), calendar("solar", 50), "unknown", 0, 0},
		{"calendar_without_index", map[string]any{"calendar": map[string]any{"id": "lunar", "label": "Festival"}}, calendar("lunar", 50), "unknown", 0, 0},
		{"same_date_many_turns", map[string]any{"date": "2020-01-01", "source_turn": 1}, map[string]any{"date": "2020-01-01", "source_turn": 200}, "same_day", 0, 0},
		{"local_date_precision", map[string]any{"datetime": "2020-01-01T01:00:00+09:00"}, map[string]any{"date": "2020-01-01"}, "same_day", 0, 0},
		{"instant", map[string]any{"datetime": "2020-01-01T01:00:00Z"}, map[string]any{"datetime": "2020-01-01T13:00:00Z"}, "past", 0.5, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := storyTimeRelation(tc.event, tc.current)
			if got["relation"] != tc.relation {
				t.Fatalf("relation: %#v", got)
			}
			if tc.relation != "unknown" {
				span := mapFromAny(got["elapsed_days"])
				if span["min"] != tc.min || span["max"] != tc.max {
					t.Fatalf("bounds: %#v", got)
				}
			}
		})
	}
	if got := storyTimeRelative(calendar("lunar", 40), map[string]any{"anchor": "source_observation", "offset": 2, "unit": "month"}); got != nil {
		t.Fatalf("invented fantasy month length: %#v", got)
	}
}

func Test46RecurringAndConditionalScheduleKeepOutcomeSeparate(t *testing.T) {
	current := map[string]any{"date": "2024-04-15"}
	for _, outcome := range []string{"", "missed", "refused", "cancelled", "impossible", "fulfilled"} {
		schedule := map[string]any{"kind": "recurring", "last_fulfilled": map[string]any{"date": "2024-01-28"}, "recurrence": map[string]any{"unit": "month", "interval": 1}, "outcome": outcome}
		before := mustCompactJSON(schedule)
		got := buildCommitmentScheduleReading(map[string]any{"schedule": schedule}, current)
		if mapFromAny(got["next_due_estimate"])["date"] != "2024-02-28" || got["outcome"] != outcome {
			t.Fatalf("month math or outcome changed: %#v", got)
		}
		if outcome == "" && got["due_cue"] != "due_passed_outcome_unknown" {
			t.Fatalf("date alone marked missed: %#v", got)
		}
		if mustCompactJSON(schedule) != before {
			t.Fatal("schedule reading mutated source")
		}
	}
	explicit := buildCommitmentScheduleReading(map[string]any{"kind": "recurring", "next_due": map[string]any{"date": "2024-05-01"}, "last_fulfilled": map[string]any{"date": "2024-01-01"}, "recurrence": map[string]any{"unit": "day", "interval": 1}}, current)
	if explicit["next_due_estimate"] != nil || mapFromAny(explicit["due_relation"])["relation"] != "future" {
		t.Fatalf("explicit next occurrence overwritten: %#v", explicit)
	}
	conditional := buildCommitmentScheduleReading(map[string]any{"kind": "conditional", "condition": "when the ship returns", "due": map[string]any{"date": "2024-01-01"}}, current)
	if conditional["condition_evaluation"] != "source_only_not_inferred_from_date" || conditional["status"] != nil {
		t.Fatalf("condition inferred from elapsed date: %#v", conditional)
	}
	standing := buildCommitmentScheduleReading(map[string]any{"kind": "standing", "condition": "keep the gate closed"}, current)
	if mapFromAny(standing["due_relation"])["relation"] != "unknown" {
		t.Fatalf("standing duty expired: %#v", standing)
	}
	monthEnd := buildCommitmentScheduleReading(map[string]any{"kind": "recurring", "last_fulfilled": map[string]any{"date": "2024-01-31"}, "recurrence": map[string]any{"unit": "month", "interval": 1}}, current)
	if monthEnd["next_due_estimate"] != nil || mapFromAny(monthEnd["due_relation"])["relation"] != "unknown" {
		t.Fatalf("invented month-end scheduling convention: %#v", monthEnd)
	}
}

func Test46LegacyRelativeLedgerDoesNotBorrowCurrentAnchor(t *testing.T) {
	for _, content := range []string{"yesterday", `{"relative_label":"yesterday","offset_value_min":-1,"offset_unit":"day","precision":"exact"}`} {
		entry := normalizeRelationEntry(store.ActiveState{Content: content, TurnIndex: 3})
		if entry["anchor"] != "unknown" || entry["anchor_resolution_status"] != "carry_forward" || entry["relative_label"] != "yesterday" || entry["source_turn"] != 3 {
			t.Fatalf("legacy relative source moved to current clock: %#v", entry)
		}
	}
	explicit := normalizeRelationEntry(store.ActiveState{Content: `{"relative_label":"next day","anchor":"source_event_4","precision":"exact"}`, TurnIndex: 5})
	if explicit["anchor"] != "source_event_4" {
		t.Fatalf("explicit anchor replaced: %#v", explicit)
	}
}
