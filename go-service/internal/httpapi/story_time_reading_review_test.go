package httpapi

import (
	"math"
	"testing"
	"time"
)

func Test46StoryTimeReadingRetainsInstantRangePrecision(t *testing.T) {
	start, end := "2024-01-01T10:00:00Z", "2024-01-01T12:00:00Z"
	event := map[string]any{"range": map[string]any{"start": map[string]any{"datetime": start}, "end": map[string]any{"datetime": end}}}
	for _, tc := range []struct {
		current, want string
	}{
		{"2024-01-01T09:00:00Z", "future"},
		{"2024-01-01T11:00:00Z", "overlapping_range"},
		{"2024-01-01T13:00:00Z", "past"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got := storyTimeRelation(event, map[string]any{"datetime": tc.current})
			if got["relation"] != tc.want || got["precision"] != "range" {
				t.Fatalf("bounded instants were reduced to a calendar date: %#v", got)
			}
			currentTime, _ := time.Parse(time.RFC3339, tc.current)
			startTime, _ := time.Parse(time.RFC3339, start)
			endTime, _ := time.Parse(time.RFC3339, end)
			span := mapFromAny(got["elapsed_days"])
			minimum, _ := storyClockNumeric(span["min"])
			maximum, _ := storyClockNumeric(span["max"])
			if math.Abs(minimum-currentTime.Sub(endTime).Hours()/24) > 1e-9 || math.Abs(maximum-currentTime.Sub(startTime).Hours()/24) > 1e-9 {
				t.Fatalf("range bounds lost hour precision: %#v", got)
			}
		})
	}
}

func Test46RecurringReadingPreservesExplicitDue(t *testing.T) {
	schedule := map[string]any{
		"kind": "recurring", "due": map[string]any{"date": "2024-02-10"},
		"last_fulfilled": map[string]any{"date": "2024-01-15"},
		"recurrence":     map[string]any{"unit": "month", "interval": 1},
	}
	before := mustCompactJSON(schedule)
	got := buildCommitmentScheduleReading(schedule, map[string]any{"date": "2024-02-12"})
	if mapFromAny(got["due_relation"])["relation"] != "past" || got["next_due_estimate"] != nil {
		t.Fatalf("explicit due was replaced by recurrence inference: %#v", got)
	}
	if mapFromAny(got["due"])["date"] != mapFromAny(schedule["due"])["date"] || mustCompactJSON(schedule) != before {
		t.Fatal("schedule reading changed the explicit source")
	}
	if got["status"] != nil || got["outcome"] != nil || got["due_cue"] != "due_passed_outcome_unknown" {
		t.Fatalf("date comparison invented an outcome: %#v", got)
	}
}

func Test46StoryTimeReadingRetainsFractionalTimestamp(t *testing.T) {
	start, end := "2024-01-01T10:00:00.1Z", "2024-01-01T10:00:00.9Z"
	got := storyTimeRelation(map[string]any{"datetime": start}, map[string]any{"datetime": end})
	if got["relation"] != "past" {
		t.Fatalf("distinct RFC3339 instants collapsed: %#v", got)
	}
	startTime, _ := time.Parse(time.RFC3339Nano, start)
	endTime, _ := time.Parse(time.RFC3339Nano, end)
	want := endTime.Sub(startTime).Hours() / 24
	span := mapFromAny(got["elapsed_days"])
	minimum, _ := storyClockNumeric(span["min"])
	maximum, _ := storyClockNumeric(span["max"])
	if math.Abs(minimum-want) > 1e-9 || math.Abs(maximum-want) > 1e-9 {
		t.Fatalf("fractional elapsed time lost: %#v", got)
	}
}

func Test46StoryTimeReadingTimezoneRangeNeverInvertsBounds(t *testing.T) {
	// The first instant precedes the second, although their explicit local dates
	// are reversed. A date-only reading must not emit an inverted elapsed span.
	event := map[string]any{"range": map[string]any{
		"start": map[string]any{"datetime": "2024-01-02T00:00:00+14:00"},
		"end":   map[string]any{"datetime": "2024-01-01T23:00:00-12:00"},
	}}
	got := storyTimeRelation(event, map[string]any{"date": "2024-01-02"})
	if got["relation"] == "unknown" {
		return // A common calendar-day basis is not supplied by these sources.
	}
	span := mapFromAny(got["elapsed_days"])
	minimum, minOK := span["min"].(float64)
	maximum, maxOK := span["max"].(float64)
	if !minOK || !maxOK || minimum > maximum {
		t.Fatalf("valid source instants produced an inverted elapsed range: %#v", got)
	}
}

func Test46StoryTimeReadingEquivalentZonedInstants(t *testing.T) {
	got := storyTimeRelation(map[string]any{"datetime": "2024-01-01T10:00:00Z"}, map[string]any{"datetime": "2024-01-01T19:00:00+09:00"})
	if got["relation"] != "same_instant" {
		t.Fatalf("equivalent explicit instants were treated as different local days: %#v", got)
	}
}

func Test46StoryTimeReadingDoesNotInventMissingTimezone(t *testing.T) {
	floating := map[string]any{"date": "2024-01-01", "time": "10:00:00"}
	zoned := map[string]any{"datetime": "2024-01-01T10:00:00+09:00"}
	for _, pair := range [][2]map[string]any{{floating, zoned}, {zoned, floating}} {
		got := storyTimeRelation(pair[0], pair[1])
		if got["relation"] != "unknown" || got["reason"] != "timezone_unknown" || got["elapsed_days"] != nil {
			t.Fatalf("timezone-less civil time was assigned a UTC offset: %#v", got)
		}
	}
	for _, tc := range []struct {
		current map[string]any
		want    string
	}{
		{map[string]any{"date": "2024-01-01", "time": "11:00:00"}, "past"},
		{map[string]any{"date": "2024-01-01"}, "same_day"},
	} {
		if got := storyTimeRelation(floating, tc.current); got["relation"] != tc.want {
			t.Fatalf("available common civil/date coordinates were discarded: %#v", got)
		}
	}
	mixedRange := map[string]any{"range": map[string]any{"start": floating, "end": zoned}}
	if got := storyTimeRelation(mixedRange, map[string]any{"datetime": "2024-01-01T12:00:00Z"}); got["relation"] != "unknown" {
		t.Fatalf("range with no common timezone basis became an exact interval: %#v", got)
	}
}
