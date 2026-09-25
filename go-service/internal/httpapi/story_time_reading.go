package httpapi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Prompt rendering consumes the existing read-only calculation. The structured
// value remains available to diagnostics and relevance scoring, unchanged.
func storyTimePromptCoordinate(raw map[string]any) string {
	raw = storyTimeCoordinate(raw)
	if len(raw) == 0 || raw["precision"] == "unknown" || raw["observation_kind"] == "unknown" || raw["story_time"] == "unknown" {
		return "unknown"
	}
	if bounds := mapFromAny(raw["range"]); len(bounds) > 0 {
		return storyTimePromptCoordinate(mapFromAny(bounds["start"])) + " to " + storyTimePromptCoordinate(mapFromAny(bounds["end"]))
	}
	parts := []string{}
	if calendar := mapFromAny(raw["calendar"]); len(calendar) > 0 {
		parts = append(parts, "calendar "+mustCompactJSON(calendar))
	}
	absolute := mapFromAny(raw["absolute"])
	if len(absolute) == 0 {
		absolute = raw
	}
	if stamp := stringFromMap(absolute, "datetime"); stamp != "" {
		parts = append(parts, stamp)
	} else {
		date, clock := stringFromMap(absolute, "date"), stringFromMap(absolute, "time")
		if date != "" {
			if clock == "" {
				parts = append(parts, date+" (date only)")
			} else {
				parts = append(parts, date+" "+clock)
			}
		} else if clock != "" {
			parts = append(parts, clock+" (date unknown)")
		}
	}
	for _, key := range []string{"partial", "relative", "sequence", "duration"} {
		if value := mapFromAny(raw[key]); len(value) > 0 {
			parts = append(parts, key+" "+mustCompactJSON(value))
		}
	}
	if len(parts) == 0 {
		return "unknown"
	}
	if scope := stringFromMap(raw, "scene_scope"); scope != "" && scope != "current" {
		parts = append(parts, "scene="+scope)
	}
	if precision := stringFromMap(raw, "precision"); precision != "" && precision != "exact" {
		parts = append(parts, "precision="+precision)
	}
	return strings.Join(parts, "; ")
}

func storyTimePromptDuration(days float64, dayOnly bool) string {
	if dayOnly {
		return strconv.FormatFloat(math.Abs(days), 'f', -1, 64) + " calendar days"
	}
	seconds := math.Round(math.Abs(days) * 86400)
	if seconds < 1 && days != 0 {
		return "<1s"
	}
	parts := []string{}
	for _, unit := range []struct {
		seconds float64
		label   string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}, {1, "s"}} {
		count := math.Floor(seconds / unit.seconds)
		if count > 0 {
			parts = append(parts, strconv.FormatFloat(count, 'f', 0, 64)+unit.label)
			seconds -= count * unit.seconds
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

func storyTimePromptRelation(relation map[string]any, coordinates ...map[string]any) string {
	kind := stringFromMap(relation, "relation")
	if kind == "same_day" {
		return "same calendar day; elapsed time unknown"
	}
	if kind == "same_instant" {
		return "same instant as reference"
	}
	span := mapFromAny(relation["elapsed_days"])
	minimum, minOK := storyClockNumeric(span["min"])
	maximum, maxOK := storyClockNumeric(span["max"])
	if kind == "unknown" || !minOK || !maxOK {
		reason := stringFromMap(relation, "reason")
		if reason == "" || reason == "date_or_source_anchor_unknown" {
			return "distance unknown"
		}
		return "distance unknown (" + reason + ")"
	}
	dayOnly := relation["precision"] == "day"
	for _, coordinate := range coordinates {
		dayOnly = dayOnly || storyTimeBounds(coordinate).dayBased
	}
	// Ranged readings may contain instants or dates. Keep bounds, not a midpoint.
	if kind == "past" || kind == "future" {
		lo, hi := minimum, maximum
		suffix := " before reference"
		if kind == "future" {
			lo, hi, suffix = -maximum, -minimum, " after reference"
		}
		text := storyTimePromptDuration(lo, dayOnly)
		if minimum != maximum {
			text += " to " + storyTimePromptDuration(hi, dayOnly)
		}
		return text + suffix
	}
	return "overlaps reference (" + storyTimePromptDuration(maximum, dayOnly) + " before to " + storyTimePromptDuration(minimum, dayOnly) + " after)"
}

func storyTimePromptReading(reading map[string]any) string {
	parts := []string{"reference=" + storyTimePromptCoordinate(mapFromAny(reading["last_confirmed_story_clock"]))}
	occurrence := mapFromAny(reading["occurrence_time"])
	if len(occurrence) == 0 {
		occurrence = mapFromAny(reading["resolved_occurrence_time"])
	}
	label := "event"
	if kind := stringFromMap(mapFromAny(reading["relative"]), "target_kind"); kind != "" {
		label += " (" + kind + ")"
	}
	clock := mapFromAny(reading["last_confirmed_story_clock"])
	parts = append(parts, label+"="+storyTimePromptCoordinate(occurrence)+"; "+storyTimePromptRelation(mapFromAny(reading["current_relation"]), occurrence, clock))
	if observed := mapFromAny(reading["observed_at"]); len(observed) > 0 {
		label := "recorded scene"
		if observed["resolution_source"] == "last_confirmed_clock" {
			label += " (carried clock)"
		}
		parts = append(parts, label+"="+storyTimePromptCoordinate(observed)+"; "+storyTimePromptRelation(mapFromAny(reading["observation_relation"]), observed, clock))
	}
	if expression := stringFromMap(reading, "relative_expression"); expression != "" {
		parts = append(parts, "original wording="+strconv.Quote(expression)+" (at source)")
	}
	if relative := mapFromAny(reading["relative"]); len(occurrence) == 0 && len(relative) > 0 {
		parts = append(parts, "unresolved source-relative="+mustCompactJSON(relative))
	}
	return "⏳ " + strings.Join(parts, " | ")
}

func storyTimePromptSchedule(reading map[string]any) string {
	due := mapFromAny(reading["next_due"])
	label := "due"
	if len(due) == 0 {
		due = mapFromAny(reading["due"])
	}
	if len(due) == 0 && len(mapFromAny(reading["next_due_estimate"])) > 0 {
		due, label = mapFromAny(reading["next_due_estimate"]), "estimated next due"
	}
	clock := mapFromAny(reading["last_confirmed_story_clock"])
	parts := []string{"reference=" + storyTimePromptCoordinate(clock), label + "=" + storyTimePromptCoordinate(due) + "; " + storyTimePromptRelation(mapFromAny(reading["due_relation"]), due, clock)}
	for _, key := range []string{"kind", "condition", "condition_evaluation", "recurrence", "last_fulfilled", "next_due_basis", "outcome", "lifecycle_transition", "due_cue"} {
		if value := reading[key]; value != nil {
			parts = append(parts, key+"="+prepareTurnPriorityScalarText(value))
		}
	}
	return "⏳ schedule | " + strings.Join(parts, " | ")
}

func storyTimePromptNote(clock map[string]any) string {
	coordinate := storyTimePromptCoordinate(clock)
	if coordinate == "unknown" {
		return ""
	}
	turn := ""
	if n := intFromAny(clock["source_turn"], 0); n > 0 {
		turn = fmt.Sprintf(" (turn %d)", n)
	}
	return "Last confirmed stored story clock (last accepted narration): " + coordinate + turn + ". ⏳ distances use this reference, not PC time or turn count. Recorded scene dates do not date the recalled event. Source yesterday/tomorrow is relative to its original scene. Use just-now/yesterday wording only when supported; unknown dates remain unknown. Explicit new user time movement directs the next scene."
}

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
