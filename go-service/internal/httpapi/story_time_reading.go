package httpapi

import (
	"math"
	"strings"
	"time"
)

// These readings are request-local calculations over admitted story coordinates.
// They never advance a clock, infer an event from its mention, or change a status.
func buildStoryTimeReading(sourceContext, currentClock map[string]any) map[string]any {
	if len(sourceContext) == 0 {
		return nil
	}
	out := storyClockJSONMap(sourceContext)
	out["clock_basis"] = "last_confirmed_story_clock"
	if clock := storyTimeReadingClock(currentClock); len(clock) > 0 {
		out["last_confirmed_story_clock"] = clock
	}
	observed := mapFromAny(sourceContext["observed_at"])
	out["observation_relation"] = storyTimeRelation(observed, currentClock)
	occurrence := mapFromAny(sourceContext["occurrence_time"])
	if len(occurrence) == 0 {
		if relative := mapFromAny(sourceContext["relative"]); len(relative) > 0 {
			occurrence = storyTimeRelative(observed, relative)
			if len(occurrence) > 0 {
				out["resolved_occurrence_time"] = occurrence
			}
		}
	}
	out["current_relation"] = storyTimeRelation(occurrence, currentClock)
	return out
}

// A due cue describes a date, not whether the obligation was performed. Explicit
// missed/refused/cancelled/impossible outcomes remain separate source facts.
func buildCommitmentScheduleReading(lifecycleDetails, currentClock map[string]any) map[string]any {
	schedule := mapFromAny(lifecycleDetails["schedule"])
	if len(schedule) == 0 {
		schedule = lifecycleDetails
	}
	kind := strings.TrimSpace(extractionStringFromAny(schedule["kind"]))
	if kind == "" && schedule["due"] == nil && schedule["next_due"] == nil && schedule["recurrence"] == nil && schedule["condition"] == nil {
		return nil
	}
	out := storyClockJSONMap(schedule)
	out["clock_basis"] = "last_confirmed_story_clock"
	if clock := storyTimeReadingClock(currentClock); len(clock) > 0 {
		out["last_confirmed_story_clock"] = clock
	}
	if out["outcome"] == nil && lifecycleDetails["outcome"] != nil {
		out["outcome"] = lifecycleDetails["outcome"]
	}
	transition := strings.TrimSpace(extractionStringFromAny(lifecycleDetails["lifecycle_transition"]))
	if transition != "" {
		out["lifecycle_transition"] = transition
	}
	due := mapFromAny(schedule["next_due"])
	if len(due) == 0 {
		due = mapFromAny(schedule["due"])
	}
	if kind == "recurring" && len(due) == 0 {
		recurrence := mapFromAny(schedule["recurrence"])
		anchor := mapFromAny(schedule["last_fulfilled"])
		if len(anchor) == 0 {
			anchor = mapFromAny(recurrence["anchor"])
		}
		if interval, ok := storyClockNumeric(recurrence["interval"]); ok && interval > 0 {
			if next := storyTimeRelative(anchor, map[string]any{"anchor": "source_observation", "offset": interval, "unit": recurrence["unit"]}); len(next) > 0 {
				due = next
				out["next_due_estimate"] = next
				out["next_due_basis"] = "one_interval_after_last_fulfilled_or_explicit_anchor"
			}
		}
	}
	relation := storyTimeRelation(due, currentClock)
	out["due_relation"] = relation
	if relation["relation"] == "past" {
		out["due_cue"] = "due_passed_outcome_unknown"
		if strings.TrimSpace(extractionStringFromAny(out["outcome"])) != "" {
			out["due_cue"] = "due_passed_with_explicit_outcome"
		}
		if narrativeLifecycleProjectionStatus(transition) == "resolved" || transition == "missed" || transition == "refused" || transition == "impossible" {
			out["due_cue"] = "due_passed_with_explicit_outcome"
		}
	}
	if kind == "conditional" {
		out["condition_evaluation"] = "source_only_not_inferred_from_date"
	}
	return out
}

type storyTimeSpan struct {
	minimum, maximum       float64
	dayMinimum, dayMaximum float64
	calendar               string
	precision              string
	dayBased               bool
	floatingTime           bool
	valid                  bool
}

func storyTimeReadingClock(currentClock map[string]any) map[string]any {
	clock := storyClockPromptProjection(storyTimeCoordinate(currentClock))
	for _, key := range []string{"date", "time", "datetime"} {
		if value, exists := currentClock[key]; exists {
			clock[key] = value
		}
	}
	return storyClockJSONMap(clock)
}

func storyTimeCoordinate(raw map[string]any) map[string]any {
	if clock := mapFromAny(raw["story_clock"]); len(clock) > 0 {
		return clock
	}
	return raw
}

func storyTimeBounds(raw map[string]any) storyTimeSpan {
	raw = storyTimeCoordinate(raw)
	if raw["precision"] == "unknown" || raw["observation_kind"] == "unknown" {
		return storyTimeSpan{}
	}
	if bounds := mapFromAny(raw["range"]); len(bounds) > 0 {
		start, end := storyTimeBounds(mapFromAny(bounds["start"])), storyTimeBounds(mapFromAny(bounds["end"]))
		if start.floatingTime != end.floatingTime && !start.dayBased && !end.dayBased {
			return storyTimeSpan{}
		}
		if start.valid && end.valid && start.calendar == end.calendar && start.minimum <= end.maximum {
			return storyTimeSpan{minimum: start.minimum, maximum: end.maximum, dayMinimum: math.Min(start.dayMinimum, end.dayMinimum), dayMaximum: math.Max(start.dayMaximum, end.dayMaximum), calendar: start.calendar, precision: "range", dayBased: start.dayBased || end.dayBased, floatingTime: start.floatingTime, valid: true}
		}
		return storyTimeSpan{}
	}
	if calendar := mapFromAny(raw["calendar"]); len(calendar) > 0 {
		id := strings.TrimSpace(extractionStringFromAny(calendar["id"]))
		day, ok := storyClockNumeric(calendar["day_index"])
		if id != "" && ok && storyClockFiniteIntegral(day) {
			return storyTimeSpan{minimum: day, maximum: day, dayMinimum: day, dayMaximum: day, calendar: "calendar:" + id, precision: "day", dayBased: true, valid: true}
		}
		return storyTimeSpan{}
	}
	absolute := mapFromAny(raw["absolute"])
	if len(absolute) == 0 {
		absolute = raw
	}
	if value, kind, ok := parseStoryClockAbsolute(absolute); ok {
		precision := "instant"
		if kind == "date" {
			precision = "day"
		}
		day := float64(value.Unix())/86400 + float64(value.Nanosecond())/(86400*1e9)
		calendarDay := float64(time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC).Unix()) / 86400
		return storyTimeSpan{minimum: day, maximum: day, dayMinimum: calendarDay, dayMaximum: calendarDay, calendar: "gregorian", precision: precision, dayBased: kind == "date", floatingTime: kind == "date_time", valid: true}
	}
	return storyTimeSpan{}
}

func storyTimeRelation(event, current map[string]any) map[string]any {
	out := map[string]any{"relation": "unknown", "basis": "last_confirmed_story_clock"}
	a, b := storyTimeBounds(event), storyTimeBounds(current)
	if !a.valid || !b.valid {
		out["reason"] = "date_or_source_anchor_unknown"
		return out
	}
	if a.calendar != b.calendar {
		out["reason"] = "different_calendars"
		return out
	}
	if a.floatingTime != b.floatingTime && !a.dayBased && !b.dayBased {
		out["reason"] = "timezone_unknown"
		return out
	}
	// Day coordinates express calendar days, not a fabricated midnight instant.
	// Mixed timestamp/date precision therefore uses whole date coordinates.
	precision := "instant"
	if a.dayBased || b.dayBased {
		precision = "day"
		a.minimum, a.maximum = a.dayMinimum, a.dayMaximum
		b.minimum, b.maximum = b.dayMinimum, b.dayMaximum
	}
	minimum, maximum := b.minimum-a.maximum, b.maximum-a.minimum
	out["elapsed_days"] = map[string]any{"min": minimum, "max": maximum}
	out["precision"] = precision
	switch {
	case minimum > 0:
		out["relation"] = "past"
	case maximum < 0:
		out["relation"] = "future"
	case minimum == 0 && maximum == 0:
		out["relation"] = "same_day"
		if precision == "instant" {
			out["relation"] = "same_instant"
		}
	default:
		out["relation"] = "overlapping_range"
	}
	if a.precision == "range" || b.precision == "range" {
		out["precision"] = "range"
	}
	return out
}

func storyTimeRelative(observed, relative map[string]any) map[string]any {
	if relative["anchor"] != "source_observation" {
		return nil
	}
	minimum, ok := storyClockNumeric(relative["offset"])
	maximum := minimum
	if !ok {
		minimum, ok = storyClockNumeric(relative["offset_min"])
		var maxOK bool
		maximum, maxOK = storyClockNumeric(relative["offset_max"])
		ok = ok && maxOK
	}
	if !ok || !storyClockFiniteIntegral(minimum) || !storyClockFiniteIntegral(maximum) || minimum > maximum {
		return nil
	}
	unit := strings.TrimSpace(extractionStringFromAny(relative["unit"]))
	observed = storyTimeCoordinate(observed)
	if observed["precision"] == "unknown" || observed["observation_kind"] == "unknown" {
		return nil
	}
	if bounds := mapFromAny(observed["range"]); len(bounds) > 0 {
		start := storyTimeRelative(mapFromAny(bounds["start"]), map[string]any{"anchor": "source_observation", "offset": minimum, "unit": unit})
		end := storyTimeRelative(mapFromAny(bounds["end"]), map[string]any{"anchor": "source_observation", "offset": maximum, "unit": unit})
		if len(start) == 0 || len(end) == 0 {
			return nil
		}
		return map[string]any{"range": map[string]any{"start": start, "end": end}}
	}
	if calendar := mapFromAny(observed["calendar"]); len(calendar) > 0 {
		day, ok := storyClockNumeric(calendar["day_index"])
		if !ok || !storyClockFiniteIntegral(day) || extractionStringFromAny(calendar["id"]) == "" || unit != "day" {
			return nil
		}
		makeDay := func(offset float64) map[string]any {
			return map[string]any{"calendar": map[string]any{"id": calendar["id"], "day_index": day + offset}}
		}
		if minimum != maximum {
			return map[string]any{"range": map[string]any{"start": makeDay(minimum), "end": makeDay(maximum)}}
		}
		return makeDay(minimum)
	}
	absolute := mapFromAny(observed["absolute"])
	if len(absolute) == 0 {
		absolute = observed
	}
	value, kind, ok := parseStoryClockAbsolute(absolute)
	if !ok {
		return nil
	}
	shift := func(offset float64) map[string]any {
		result, ok := addStoryClockOffset(value, offset, unit)
		if !ok || result.Year() < 1 || result.Year() > 9999 {
			return nil
		}
		formatted, ok := storyClockAbsoluteForTime(result, kind, unit)
		if !ok {
			return nil
		}
		return formatted
	}
	start, end := shift(minimum), shift(maximum)
	if start == nil || end == nil {
		return nil
	}
	if minimum != maximum {
		return map[string]any{"range": map[string]any{"start": start, "end": end}}
	}
	return start
}
