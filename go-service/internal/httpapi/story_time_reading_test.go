package httpapi

import (
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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
