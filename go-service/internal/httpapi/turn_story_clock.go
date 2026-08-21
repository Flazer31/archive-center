package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	storyClockContractVersion = "story_clock.v1"
	storyClockStatusKey       = "story_clock"
	storyClockOwnerScope      = "session"
	storyClockOwnerID         = "current"
)

var (
	storyClockObservationKinds = map[string]bool{
		"absolute": true, "partial": true, "relative": true, "bounded_range": true, "unknown": true,
	}
	storyClockSceneScopes = map[string]bool{
		"current": true, "flashback": true, "planned": true, "hypothetical": true,
	}
	storyClockPrecisions = map[string]bool{
		"exact": true, "partial": true, "bounded_range": true, "unknown": true,
	}
	storyClockTransitions = map[string]bool{
		"set": true, "advance": true, "correction": true, "reaffirm": true, "supersede": true, "retract": true,
	}
	storyClockRelativeUnits = map[string]bool{
		"second": true, "minute": true, "hour": true, "day": true, "week": true, "month": true, "year": true,
	}
)

func validateStoryClockProposal(value any) error {
	raw, ok := value.(map[string]any)
	if !ok {
		return errors.New("critic schema field story_clock must be an object")
	}
	if version := strings.TrimSpace(extractionStringFromAny(raw["version"])); version != storyClockContractVersion {
		return errors.New("critic schema field story_clock.version is invalid")
	}
	if strings.TrimSpace(extractionStringFromAny(raw["version"])) != storyClockContractVersion {
		return errors.New("critic schema field story_clock.version is invalid")
	}
	kind := strings.TrimSpace(extractionStringFromAny(raw["observation_kind"]))
	if !storyClockObservationKinds[kind] {
		return fmt.Errorf("critic schema field story_clock.observation_kind is invalid")
	}
	scope := strings.TrimSpace(extractionStringFromAny(raw["scene_scope"]))
	if !storyClockSceneScopes[scope] {
		return fmt.Errorf("critic schema field story_clock.scene_scope is invalid")
	}
	precision := strings.TrimSpace(extractionStringFromAny(raw["precision"]))
	if !storyClockPrecisions[precision] {
		return fmt.Errorf("critic schema field story_clock.precision is invalid")
	}
	if transition := strings.TrimSpace(extractionStringFromAny(raw["transition"])); transition != "" && !storyClockTransitions[transition] {
		return fmt.Errorf("critic schema field story_clock.transition is invalid")
	}
	if _, ok := raw["evidence_excerpt"].(string); !ok {
		return errors.New("critic schema field story_clock.evidence_excerpt must be a string")
	}
	for _, key := range []string{"absolute", "partial", "relative", "range", "sequence", "duration"} {
		if nested, exists := raw[key]; exists {
			if _, ok := nested.(map[string]any); !ok {
				return fmt.Errorf("critic schema field story_clock.%s must be an object", key)
			}
		}
	}
	primaryField := map[string]string{
		"absolute": "absolute", "partial": "partial", "relative": "relative", "bounded_range": "range",
	}[kind]
	for _, key := range []string{"absolute", "partial", "relative", "range"} {
		if nested := mapFromAny(raw[key]); len(nested) > 0 && key != primaryField {
			return fmt.Errorf("critic schema field story_clock.%s conflicts with observation_kind %s", key, kind)
		}
	}
	for key, allowed := range map[string]map[string]bool{
		"absolute": {"date": true, "time": true, "datetime": true},
		"partial":  {"daypart": true, "season": true},
		"relative": {"offset": true, "offset_min": true, "offset_max": true, "unit": true, "anchor": true},
		"range":    {"start": true, "end": true},
		"sequence": {"relation": true, "anchor": true, "index": true, "label": true},
		"duration": {"value": true, "min": true, "max": true, "unit": true, "approximate": true},
	} {
		if err := validateStoryClockObjectKeys(key, mapFromAny(raw[key]), allowed); err != nil {
			return err
		}
	}
	if err := validateStoryClockSequence(mapFromAny(raw["sequence"])); err != nil {
		return err
	}
	if err := validateStoryClockDuration(mapFromAny(raw["duration"])); err != nil {
		return err
	}
	switch kind {
	case "absolute":
		absolute := mapFromAny(raw["absolute"])
		if strings.TrimSpace(extractionStringFromAny(absolute["date"])) == "" &&
			strings.TrimSpace(extractionStringFromAny(absolute["datetime"])) == "" {
			return errors.New("critic schema field story_clock.absolute requires date or datetime")
		}
		if _, _, ok := parseStoryClockAbsolute(absolute); !ok {
			return errors.New("critic schema field story_clock.absolute must use a valid ISO date or RFC3339 datetime")
		}
		if precision != "exact" {
			return errors.New("critic schema field story_clock absolute observation requires exact precision")
		}
	case "partial":
		partial := mapFromAny(raw["partial"])
		if strings.TrimSpace(extractionStringFromAny(partial["daypart"])) == "" &&
			strings.TrimSpace(extractionStringFromAny(partial["season"])) == "" {
			return errors.New("critic schema field story_clock.partial requires daypart or season")
		}
		if precision != "partial" {
			return errors.New("critic schema field story_clock partial observation requires partial precision")
		}
	case "relative":
		relative := mapFromAny(raw["relative"])
		unit := strings.TrimSpace(extractionStringFromAny(relative["unit"]))
		if !storyClockRelativeUnits[unit] {
			return errors.New("critic schema field story_clock.relative.unit is invalid")
		}
		if strings.TrimSpace(extractionStringFromAny(relative["anchor"])) == "" {
			return errors.New("critic schema field story_clock.relative.anchor is required")
		}
		if offset, ok := storyClockNumeric(relative["offset"]); ok {
			if !storyClockFiniteIntegral(offset) {
				return errors.New("critic schema field story_clock.relative.offset must be a finite integer")
			}
			if precision != "exact" && precision != "unknown" {
				return errors.New("critic schema field story_clock exact relative observation requires exact or unknown precision")
			}
		} else {
			minimum, minOK := storyClockNumeric(relative["offset_min"])
			maximum, maxOK := storyClockNumeric(relative["offset_max"])
			if !minOK || !maxOK || !storyClockFiniteIntegral(minimum) || !storyClockFiniteIntegral(maximum) {
				return errors.New("critic schema field story_clock.relative requires numeric offset or offset_min")
			}
			if minimum > maximum {
				return errors.New("critic schema field story_clock.relative offset range is reversed")
			}
			if minimum == maximum && precision != "exact" && precision != "unknown" {
				return errors.New("critic schema field story_clock exact relative range requires exact or unknown precision")
			}
			if minimum != maximum && precision != "bounded_range" && precision != "unknown" {
				return errors.New("critic schema field story_clock ranged relative observation requires bounded_range or unknown precision")
			}
		}
	case "bounded_range":
		rng := mapFromAny(raw["range"])
		start := mapFromAny(rng["start"])
		end := mapFromAny(rng["end"])
		if len(start) == 0 || len(end) == 0 {
			return errors.New("critic schema field story_clock.range requires start and end objects")
		}
		startTime, _, startOK := parseStoryClockAbsolute(start)
		endTime, _, endOK := parseStoryClockAbsolute(end)
		if !startOK || !endOK {
			return errors.New("critic schema field story_clock.range bounds must use valid ISO dates or RFC3339 datetimes")
		}
		if endTime.Before(startTime) {
			return errors.New("critic schema field story_clock.range bounds are reversed")
		}
		if precision != "bounded_range" {
			return errors.New("critic schema field story_clock bounded_range observation requires bounded_range precision")
		}
	case "unknown":
		if precision != "unknown" {
			return errors.New("critic schema field story_clock unknown observation must keep unknown precision")
		}
	}
	return nil
}

func validateStoryClockObjectKeys(field string, raw map[string]any, allowed map[string]bool) error {
	for key := range raw {
		if !allowed[key] {
			return fmt.Errorf("critic schema field story_clock.%s.%s is not allowed", field, key)
		}
	}
	return nil
}

func validateStoryClockSequence(raw map[string]any) error {
	for _, key := range []string{"relation", "anchor", "label"} {
		if value, exists := raw[key]; exists {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("critic schema field story_clock.sequence.%s must be a string", key)
			}
		}
	}
	if value, exists := raw["index"]; exists {
		number, ok := storyClockNumeric(value)
		if !ok || !storyClockFiniteIntegral(number) {
			return errors.New("critic schema field story_clock.sequence.index must be a finite integer")
		}
	}
	return nil
}

func validateStoryClockDuration(raw map[string]any) error {
	if len(raw) == 0 {
		return nil
	}
	unit := strings.TrimSpace(extractionStringFromAny(raw["unit"]))
	if !storyClockRelativeUnits[unit] {
		return errors.New("critic schema field story_clock.duration.unit is invalid")
	}
	hasNumeric := false
	values := map[string]float64{}
	for _, key := range []string{"value", "min", "max"} {
		if value, exists := raw[key]; exists {
			number, ok := storyClockNumeric(value)
			if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
				return fmt.Errorf("critic schema field story_clock.duration.%s must be a finite non-negative number", key)
			}
			values[key] = number
			hasNumeric = true
		}
	}
	if !hasNumeric {
		return errors.New("critic schema field story_clock.duration requires value or min/max")
	}
	if minimum, minOK := values["min"]; minOK {
		if maximum, maxOK := values["max"]; !maxOK || minimum > maximum {
			return errors.New("critic schema field story_clock.duration range is incomplete or reversed")
		}
	}
	if value, exists := raw["approximate"]; exists {
		if _, ok := value.(bool); !ok {
			return errors.New("critic schema field story_clock.duration.approximate must be a boolean")
		}
	}
	return nil
}

func normalizeStoryClockProposal(value any) map[string]any {
	raw := mapFromAny(value)
	if len(raw) == 0 || validateStoryClockProposal(raw) != nil {
		return nil
	}
	transition := strings.TrimSpace(extractionStringFromAny(raw["transition"]))
	if transition == "" {
		transition = "set"
	}
	out := map[string]any{
		"version":          storyClockContractVersion,
		"observation_kind": strings.TrimSpace(extractionStringFromAny(raw["observation_kind"])),
		"scene_scope":      strings.TrimSpace(extractionStringFromAny(raw["scene_scope"])),
		"precision":        strings.TrimSpace(extractionStringFromAny(raw["precision"])),
		"evidence_excerpt": strings.TrimSpace(extractionStringFromAny(raw["evidence_excerpt"])),
		"transition":       transition,
	}
	for _, key := range []string{"absolute", "partial", "relative", "range", "sequence", "duration"} {
		if nested := mapFromAny(raw[key]); len(nested) > 0 {
			out[key] = storyClockJSONMap(nested)
		}
	}
	return out
}

func storyClockJSONMap(raw map[string]any) map[string]any {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if json.Unmarshal(encoded, &out) != nil {
		return nil
	}
	return out
}

func storyClockNumeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		result, err := typed.Float64()
		return result, err == nil
	case string:
		result, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return result, err == nil
	default:
		return 0, false
	}
}

func storyClockFiniteIntegral(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return false
	}
	intLimit := math.Ldexp(1, strconv.IntSize-1)
	return value < intLimit && value >= -intLimit
}

func (s *Server) ensureStoryClockDefinition(ctx context.Context, sid string, now time.Time, result *artifactSaveResult) (store.StatusSchemaDefinition, bool) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		result.addSkipReason("story_clock", "status_schema_registry_unavailable", nil)
		return store.StatusSchemaDefinition{}, false
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, storyClockStatusKey, storyClockOwnerScope)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("story_clock", "status_schema_lookup_failed", err.Error())
		return store.StatusSchemaDefinition{}, false
	}
	result.Attempted++
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid,
		SchemaName:    "story_clock",
		StatusKey:     storyClockStatusKey,
		Label:         "Story clock",
		OwnerScope:    storyClockOwnerScope,
		ValueKind:     "object",
		OptionsJSON: mustCompactJSON(map[string]any{
			"contract_version":         storyClockContractVersion,
			"session_scoped":           true,
			"wall_clock_authority":     false,
			"turn_number_is_not_time":  true,
			"unknown_stays_unresolved": true,
		}),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil || len(definitions) == 0 {
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusSchemaDefinitions(story_clock): "+err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	result.StatusSchemaDefinitions++
	return definitions[0], true
}

func storyClockCurrentValue(values []store.StatusCurrentValue) store.StatusCurrentValue {
	var current store.StatusCurrentValue
	for _, item := range values {
		if item.StatusKey != storyClockStatusKey || item.OwnerScope != storyClockOwnerScope || item.OwnerID != storyClockOwnerID {
			continue
		}
		if current.ID == 0 || item.SourceTurn > current.SourceTurn ||
			(item.SourceTurn == current.SourceTurn && item.UpdatedAt.After(current.UpdatedAt)) {
			current = item
		}
	}
	return current
}

func storyClockSourceMetadata(ctx context.Context, sid string, turnIndex int, content string) (entityIdentitySourceContext, bool) {
	source := entityIdentitySourceFromContext(ctx, sid, turnIndex, content)
	return source, source.ContractVersion == completeTurnSourceAcceptanceContract &&
		strings.TrimSpace(source.Revision) != "" &&
		strings.TrimSpace(source.LogicalTurnID) != "" &&
		strings.TrimSpace(source.ContentHash) != ""
}

func storyClockMatchingEvidenceIDs(evidence []store.DirectEvidence, sid string, turnIndex int, excerpt string) []int64 {
	needle := normalizeArtifactDedupeText(excerpt)
	ids := []int64{}
	for _, item := range evidence {
		anchor := item.TurnAnchor
		if anchor == 0 {
			anchor = item.SourceTurnStart
		}
		if item.ID <= 0 ||
			needle == "" ||
			strings.TrimSpace(item.ChatSessionID) != strings.TrimSpace(sid) ||
			(turnIndex > 0 && anchor != turnIndex) ||
			item.ArchiveState != "verified_direct" ||
			item.CaptureVerification != "verified" ||
			item.CommittedGate != "auto_grounded_excerpt" ||
			item.RepairNeeded ||
			item.Tombstoned ||
			item.SupersededByID > 0 ||
			normalizeArtifactDedupeText(item.EvidenceText) != needle {
			continue
		}
		ids = append(ids, item.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func storyClockEvidencePayload(source entityIdentitySourceContext, turnIndex int, proposal map[string]any, evidenceIDs []int64) map[string]any {
	return map[string]any{
		"contract_version":     storyClockContractVersion,
		"source":               "critic.story_clock",
		"source_turn":          turnIndex,
		"source_revision":      source.Revision,
		"logical_turn_id":      source.LogicalTurnID,
		"source_message_id":    source.MessageID,
		"source_generation_id": source.GenerationID,
		"content_hash":         source.ContentHash,
		"direct_evidence_ids":  evidenceIDs,
		"evidence_excerpt":     proposal["evidence_excerpt"],
		"branch_scope":         "chat_session",
		"scene_scope":          proposal["scene_scope"],
		"transition":           proposal["transition"],
	}
}

func storyClockResolvedCurrent(proposal map[string]any, previous store.StatusCurrentValue) (map[string]any, string) {
	kind := strings.TrimSpace(extractionStringFromAny(proposal["observation_kind"]))
	if strings.TrimSpace(extractionStringFromAny(proposal["scene_scope"])) != "current" {
		return nil, "non_current_scene_scope"
	}
	if strings.TrimSpace(extractionStringFromAny(proposal["transition"])) == "retract" {
		retracted := storyClockJSONMap(proposal)
		retracted["observation_kind"] = "unknown"
		retracted["precision"] = "unknown"
		retracted["retracted_observation_kind"] = kind
		delete(retracted, "absolute")
		delete(retracted, "partial")
		delete(retracted, "range")
		delete(retracted, "relative")
		return retracted, "current_clock_retracted"
	}
	switch kind {
	case "absolute", "partial", "bounded_range":
		return storyClockJSONMap(proposal), ""
	case "relative":
		if strings.TrimSpace(extractionStringFromAny(proposal["precision"])) == "unknown" {
			return unresolvedRelativeStoryClock(proposal), "relative_precision_unknown"
		}
		if previous.ID == 0 || strings.TrimSpace(previous.ValueJSON) == "" {
			return unresolvedRelativeStoryClock(proposal), "relative_anchor_missing"
		}
		anchor := map[string]any{}
		if json.Unmarshal([]byte(previous.ValueJSON), &anchor) != nil {
			return unresolvedRelativeStoryClock(proposal), "relative_anchor_malformed"
		}
		resolved, ok := resolveRelativeStoryClock(anchor, proposal)
		if !ok {
			return unresolvedRelativeStoryClock(proposal), "relative_anchor_not_resolvable"
		}
		return resolved, ""
	case "unknown":
		return storyClockJSONMap(proposal), "current_clock_unknown"
	default:
		return nil, "unknown_observation"
	}
}

func unresolvedRelativeStoryClock(proposal map[string]any) map[string]any {
	out := storyClockJSONMap(proposal)
	out["source_precision"] = proposal["precision"]
	out["precision"] = "unknown"
	return out
}

func resolveRelativeStoryClock(anchor, proposal map[string]any) (map[string]any, bool) {
	if strings.TrimSpace(extractionStringFromAny(anchor["observation_kind"])) != "absolute" {
		return nil, false
	}
	absolute := mapFromAny(anchor["absolute"])
	anchorTime, layoutKind, ok := parseStoryClockAbsolute(absolute)
	if !ok {
		return nil, false
	}
	relative := mapFromAny(proposal["relative"])
	if strings.TrimSpace(extractionStringFromAny(relative["anchor"])) != "story_clock.current" {
		return nil, false
	}
	unit := strings.TrimSpace(extractionStringFromAny(relative["unit"]))
	offset, ok := storyClockNumeric(relative["offset"])
	if !ok {
		minimum, minOK := storyClockNumeric(relative["offset_min"])
		maximum, maxOK := storyClockNumeric(relative["offset_max"])
		if !minOK || !maxOK {
			return nil, false
		}
		if minimum != maximum {
			start, startOK := addStoryClockOffset(anchorTime, minimum, unit)
			end, endOK := addStoryClockOffset(anchorTime, maximum, unit)
			startAbsolute, startFormatOK := storyClockAbsoluteForTime(start, layoutKind, unit)
			endAbsolute, endFormatOK := storyClockAbsoluteForTime(end, layoutKind, unit)
			if !startOK || !endOK || !startFormatOK || !endFormatOK || end.Before(start) {
				return nil, false
			}
			out := storyClockJSONMap(proposal)
			out["observation_kind"] = "bounded_range"
			out["source_observation_kind"] = "relative"
			out["precision"] = "bounded_range"
			out["range"] = map[string]any{"start": startAbsolute, "end": endAbsolute}
			delete(out, "relative")
			out["resolved_from"] = map[string]any{
				"anchor_source_turn": previousSourceTurn(anchor),
				"anchor":             "story_clock.current",
				"relative":           relative,
			}
			return out, true
		}
		offset = minimum
	}
	resolvedTime, ok := addStoryClockOffset(anchorTime, offset, unit)
	if !ok {
		return nil, false
	}
	resolvedAbsolute, ok := storyClockAbsoluteForTime(resolvedTime, layoutKind, unit)
	if !ok {
		return nil, false
	}
	out := storyClockJSONMap(proposal)
	out["observation_kind"] = "absolute"
	out["source_observation_kind"] = "relative"
	out["precision"] = "exact"
	out["absolute"] = resolvedAbsolute
	delete(out, "relative")
	out["resolved_from"] = map[string]any{
		"anchor_source_turn": previousSourceTurn(anchor),
		"anchor":             "story_clock.current",
		"relative":           relative,
	}
	return out, true
}

func storyClockAbsoluteForTime(value time.Time, layoutKind, unit string) (map[string]any, bool) {
	absolute := map[string]any{}
	switch layoutKind {
	case "datetime":
		absolute["datetime"] = value.Format(time.RFC3339)
	case "date_time":
		absolute["date"] = value.Format("2006-01-02")
		absolute["time"] = value.Format("15:04:05")
	default:
		if unit == "second" || unit == "minute" || unit == "hour" {
			return nil, false
		}
		absolute["date"] = value.Format("2006-01-02")
	}
	return absolute, true
}

func previousSourceTurn(value map[string]any) int {
	return intFromAny(value["source_turn"], 0)
}

func parseStoryClockAbsolute(absolute map[string]any) (time.Time, string, bool) {
	if raw := strings.TrimSpace(extractionStringFromAny(absolute["datetime"])); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		return value, "datetime", err == nil
	}
	date := strings.TrimSpace(extractionStringFromAny(absolute["date"]))
	if date == "" {
		return time.Time{}, "", false
	}
	clock := strings.TrimSpace(extractionStringFromAny(absolute["time"]))
	if clock == "" {
		value, err := time.Parse("2006-01-02", date)
		return value, "date", err == nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if value, err := time.Parse(layout, date+"T"+clock); err == nil {
			return value, "date_time", true
		}
	}
	return time.Time{}, "", false
}

func addStoryClockOffset(anchor time.Time, offset float64, unit string) (time.Time, bool) {
	if !storyClockFiniteIntegral(offset) {
		return time.Time{}, false
	}
	whole := int(offset)
	var result time.Time
	switch unit {
	case "second":
		if whole > int(math.MaxInt64/int64(time.Second)) || whole < int(math.MinInt64/int64(time.Second)) {
			return time.Time{}, false
		}
		result = anchor.Add(time.Duration(whole) * time.Second)
	case "minute":
		if whole > int(math.MaxInt64/int64(time.Minute)) || whole < int(math.MinInt64/int64(time.Minute)) {
			return time.Time{}, false
		}
		result = anchor.Add(time.Duration(whole) * time.Minute)
	case "hour":
		if whole > int(math.MaxInt64/int64(time.Hour)) || whole < int(math.MinInt64/int64(time.Hour)) {
			return time.Time{}, false
		}
		result = anchor.Add(time.Duration(whole) * time.Hour)
	case "day":
		result = anchor.AddDate(0, 0, whole)
	case "week":
		if whole > int(^uint(0)>>1)/7 || whole < (-int(^uint(0)>>1)-1)/7 {
			return time.Time{}, false
		}
		result = anchor.AddDate(0, 0, whole*7)
	case "month":
		var ok bool
		result, ok = addStoryClockCalendarMonths(anchor, whole)
		if !ok {
			return time.Time{}, false
		}
	case "year":
		targetYear := int64(anchor.Year()) + int64(whole)
		if targetYear < 1 || targetYear > 9999 ||
			anchor.Day() > daysInStoryClockMonth(int(targetYear), anchor.Month()) {
			return time.Time{}, false
		}
		result = time.Date(
			int(targetYear), anchor.Month(), anchor.Day(),
			anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), anchor.Location(),
		)
	default:
		return time.Time{}, false
	}
	if result.Year() < 0 || result.Year() > 9999 {
		return time.Time{}, false
	}
	return result, true
}

func addStoryClockCalendarMonths(anchor time.Time, months int) (time.Time, bool) {
	monthIndex := int64(anchor.Year())*12 + int64(anchor.Month()-1) + int64(months)
	if monthIndex < 12 || monthIndex > int64(9999*12+11) {
		return time.Time{}, false
	}
	targetYear := int(monthIndex / 12)
	targetMonth := time.Month(monthIndex%12 + 1)
	if anchor.Day() > daysInStoryClockMonth(targetYear, targetMonth) {
		return time.Time{}, false
	}
	return time.Date(
		targetYear, targetMonth, anchor.Day(),
		anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), anchor.Location(),
	), true
}

func daysInStoryClockMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (s *Server) saveStoryClockFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, evidence []store.DirectEvidence, now time.Time, result *artifactSaveResult) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	proposal := normalizeStoryClockProposal(extraction["story_clock"])
	if len(proposal) == 0 {
		return
	}
	proposal["evidence_excerpt"] = sanitizeEvidenceExcerptForTurn(extractionStringFromAny(proposal["evidence_excerpt"]), content)
	source, accepted := storyClockSourceMetadata(ctx, sid, turnIndex, content)
	if !accepted {
		result.addSkipReason("story_clock", "accepted_source_required", nil)
		return
	}
	evidenceIDs := []int64{}
	if excerpt := strings.TrimSpace(extractionStringFromAny(proposal["evidence_excerpt"])); excerpt != "" {
		evidenceIDs = storyClockMatchingEvidenceIDs(evidence, sid, turnIndex, excerpt)
	}
	currentStore, currentOK := s.Store.(store.StatusCurrentValueStore)
	atomicStore, atomicOK := s.Store.(store.ReversibleStatusTransitionStore)
	if !currentOK || !atomicOK {
		result.addSkipReason("story_clock", "atomic_status_transition_unavailable", nil)
		return
	}
	definition, ok := s.ensureStoryClockDefinition(ctx, sid, now, result)
	if !ok {
		return
	}
	currentValues, err := currentStore.ListStatusCurrentValues(ctx, sid, storyClockOwnerScope, storyClockOwnerID, storyClockStatusKey, 1)
	if err != nil {
		result.addSkipReason("story_clock", "current_clock_read_failed", err.Error())
		return
	}
	previous := storyClockCurrentValue(currentValues)
	resolved, unresolvedReason := storyClockResolvedCurrent(proposal, previous)
	olderThanCurrent := previous.SourceTurn > turnIndex && turnIndex > 0
	if olderThanCurrent {
		resolved = nil
		unresolvedReason = "older_turn_historical_only"
		result.addSkipReason("story_clock", "older_turn_cannot_replace_current_clock", map[string]any{"current_turn": previous.SourceTurn, "incoming_turn": turnIndex})
	}
	valueForEvent := proposal
	eventState := "recorded"
	if len(resolved) > 0 {
		resolved["source_turn"] = turnIndex
		valueForEvent = resolved
	} else {
		switch unresolvedReason {
		case "non_current_scene_scope", "older_turn_historical_only":
			eventState = "recorded"
		default:
			eventState = "unresolved"
		}
	}
	newValueJSON := mustCompactJSON(valueForEvent)
	eventOwnerID := storyClockOwnerID
	if len(resolved) == 0 {
		eventOwnerID = "source:" + source.Revision
	}
	evidencePayload := storyClockEvidencePayload(source, turnIndex, proposal, evidenceIDs)
	sourceUnitID := "story_clock:" + source.Revision
	evidencePayload["source_unit_id"] = sourceUnitID
	evidencePayload["current_projection"] = len(resolved) > 0
	if unresolvedReason != "" {
		evidencePayload["resolution_status"] = unresolvedReason
	}
	transition := extractionStringFromAny(proposal["transition"])
	if len(resolved) > 0 && previous.ID > 0 {
		previousEvidence := map[string]any{}
		_ = json.Unmarshal([]byte(previous.EvidenceJSON), &previousEvidence)
		resolved["correction_lineage"] = map[string]any{
			"transition":               transition,
			"previous_status_value_id": previous.ID,
			"previous_source_turn":     previous.SourceTurn,
			"previous_source_revision": extractionStringFromAny(previousEvidence["source_revision"]),
		}
		newValueJSON = mustCompactJSON(resolved)
	}
	var currentValue *store.StatusCurrentValue
	if len(resolved) > 0 {
		currentValue = &store.StatusCurrentValue{
			ChatSessionID: sid,
			RegistryID:    definition.ID,
			StatusKey:     storyClockStatusKey,
			OwnerScope:    storyClockOwnerScope,
			OwnerID:       storyClockOwnerID,
			OwnerLabel:    "Current story clock",
			ValueKind:     "object",
			ValueJSON:     newValueJSON,
			EvidenceJSON:  mustCompactJSON(evidencePayload),
			SourceTurn:    turnIndex,
			WriteState:    "current",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
	}
	eventKind := "observation_recorded"
	if len(resolved) > 0 {
		eventKind = "set"
		if previous.ID > 0 {
			eventKind = "change"
		}
		if transition == "correction" {
			eventKind = "correction"
		} else if transition == "advance" {
			eventKind = "advance"
		} else if transition == "supersede" {
			eventKind = "supersede"
		}
	}
	if transition == "retract" {
		eventKind = "retract"
		eventState = "retracted"
	}
	result.Attempted++
	saved, err := atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
		SourceContract: source.ContractVersion,
		SourceRevision: source.Revision,
		SourceUnitID:   sourceUnitID,
		CurrentValue:   currentValue,
		Event: store.StatusChangeEvent{
			ChatSessionID:     sid,
			RegistryID:        definition.ID,
			StatusKey:         storyClockStatusKey,
			OwnerScope:        storyClockOwnerScope,
			OwnerID:           eventOwnerID,
			EventKind:         eventKind,
			PreviousValueJSON: previous.ValueJSON,
			NewValueJSON:      newValueJSON,
			EvidenceJSON:      mustCompactJSON(evidencePayload),
			SourceTurn:        turnIndex,
			StoryClockJSON:    newValueJSON,
			EventState:        eventState,
			CreatedAt:         now,
		},
	})
	if err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "ApplyReversibleStatusTransition(story_clock): "+err.Error())
		return
	}
	if saved.Replayed {
		result.addSkipReason("story_clock", "source_replay_idempotent", map[string]any{"source_revision": source.Revision})
		return
	}
	if currentValue != nil {
		result.NarrativeCurrentStates++
	}
	result.NarrativeStateEvents++
}

func restoreStoryClockCurrentAfterRollback(ctx context.Context, st store.Store, sid string) (int, error) {
	currentStore, currentOK := st.(store.StatusCurrentValueStore)
	eventLookup, lookupOK := st.(store.StatusChangeEventSourceLookupStore)
	if !currentOK || !lookupOK {
		return 0, nil
	}
	latest, err := eventLookup.GetLatestCurrentProjectionStatusChangeEvent(ctx, sid, storyClockStatusKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if latest.ID == 0 || strings.TrimSpace(latest.NewValueJSON) == "" {
		return 0, nil
	}
	_, err = currentStore.SaveStatusCurrentValue(ctx, store.StatusCurrentValue{
		ChatSessionID: sid,
		RegistryID:    latest.RegistryID,
		StatusKey:     storyClockStatusKey,
		OwnerScope:    storyClockOwnerScope,
		OwnerID:       storyClockOwnerID,
		OwnerLabel:    "Current story clock",
		ValueKind:     "object",
		ValueJSON:     latest.NewValueJSON,
		EvidenceJSON:  latest.EvidenceJSON,
		SourceTurn:    latest.SourceTurn,
		WriteState:    "current",
		CreatedAt:     latest.CreatedAt,
		UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func storyClockCurrentProjection(values []store.StatusCurrentValue) map[string]any {
	current := storyClockCurrentValue(values)
	if current.ID == 0 {
		return nil
	}
	payload := map[string]any{}
	if json.Unmarshal([]byte(current.ValueJSON), &payload) != nil {
		return nil
	}
	if validateStoryClockProposal(payload) != nil {
		return nil
	}
	payload["version"] = storyClockContractVersion
	payload["resolution_source"] = "status_current_values"
	payload["precision_label"] = extractionStringFromAny(payload["precision"])
	payload["turn_index"] = current.SourceTurn
	payload["status_value_id"] = current.ID
	payload["branch_scope"] = "chat_session"
	return payload
}
