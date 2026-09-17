package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const referenceCoverageFieldIndexContractVersion = "coverage_field_index.v1"

type referenceCoverageFieldIndexBindingSummary struct {
	BindingID          string `json:"binding_id"`
	WorkID             string `json:"work_id"`
	ContinuityID       string `json:"continuity_id"`
	ContextHash        string `json:"context_hash"`
	InventoryHash      string `json:"inventory_hash"`
	SnapshotHash       string `json:"snapshot_hash"`
	SourceMessageCount int    `json:"source_message_count"`
	FieldCount         int    `json:"field_count"`
	CoveredFieldCount  int    `json:"covered_field_count"`
	SnapshotPersisted  bool   `json:"snapshot_persisted"`
	SnapshotReused     bool   `json:"snapshot_reused"`
}

type referenceCoverageNeededSource struct {
	BindingID         string   `json:"binding_id"`
	WorkID            string   `json:"work_id"`
	ContinuityID      string   `json:"continuity_id"`
	ReferenceKind     string   `json:"reference_kind"`
	SourceID          string   `json:"source_id"`
	NeededBy          []string `json:"needed_by"`
	CoverageStatus    string   `json:"coverage_status"`
	MissingFields     []string `json:"missing_fields"`
	MatchedLocations  []string `json:"matched_locations"`
	Eligible          bool     `json:"eligible"`
	EligibilityReason string   `json:"eligibility_reason"`
}

type referenceCoverageFieldIndexSummary struct {
	ContractVersion   string                                      `json:"contract_version"`
	Mode              string                                      `json:"mode"`
	Status            string                                      `json:"status"`
	BindingCount      int                                         `json:"binding_count"`
	InventoryFields   int                                         `json:"inventory_field_count"`
	EligibleFields    int                                         `json:"eligible_field_count"`
	BlockedFields     int                                         `json:"blocked_field_count"`
	CoveredFields     int                                         `json:"covered_field_count"`
	MissingFields     int                                         `json:"missing_field_count"`
	NeededSources     int                                         `json:"needed_source_count"`
	CoveredSources    int                                         `json:"covered_source_count"`
	PartialSources    int                                         `json:"partial_source_count"`
	MissingSources    int                                         `json:"missing_source_count"`
	BlockedSources    int                                         `json:"blocked_source_count"`
	SnapshotWrites    int                                         `json:"snapshot_write_count"`
	SnapshotReuses    int                                         `json:"snapshot_reuse_count"`
	Bindings          []referenceCoverageFieldIndexBindingSummary `json:"bindings"`
	NeededSourceItems []referenceCoverageNeededSource             `json:"needed_sources"`
	NeededTruncated   bool                                        `json:"needed_sources_truncated"`
}

func newReferenceCoverageFieldIndexSummary() referenceCoverageFieldIndexSummary {
	return referenceCoverageFieldIndexSummary{
		ContractVersion:   referenceCoverageFieldIndexContractVersion,
		Mode:              "shadow",
		Status:            "not_evaluated",
		Bindings:          []referenceCoverageFieldIndexBindingSummary{},
		NeededSourceItems: []referenceCoverageNeededSource{},
	}
}

func (s *Server) buildReferenceCoverageFieldIndex(ctx context.Context, bindings []store.SessionReferenceBinding, scopes map[string]referenceRecallScope, query string, messages []map[string]any, sceneContext referenceCoverageSceneContext) (referenceCoverageFieldIndexSummary, []string) {
	summary := newReferenceCoverageFieldIndexSummary()
	warnings := []string{}
	persistenceFailures := 0
	activeContext := referenceCoverageActiveContextMessages(query, messages, sceneContext)
	contextHash := referenceCoverageContextHash(activeContext)
	coverageStore, canPersist := s.Store.(store.ReferenceCoverageStore)

	for _, binding := range bindings {
		scope, ok := scopes[binding.BindingID]
		if !ok {
			continue
		}
		fields := referenceCoverageInventoryFields(scope, activeContext)
		inventoryHash := referenceCoverageInventoryHash(fields)
		snapshotHash := referenceCoverageHash(referenceCoverageFieldIndexContractVersion, contextHash, inventoryHash)
		coveredCount := 0
		eligibleCount := 0
		for _, field := range fields {
			if field.Eligible {
				eligibleCount++
			}
			if field.Eligible && field.PresentInContext {
				coveredCount++
			}
		}
		statsJSON, _ := json.Marshal(map[string]any{
			"eligible_fields": referenceCoverageEligibleFieldCount(fields),
			"literal_match":   true,
			"semantic_score":  false,
		})
		snapshot := &store.SessionReferenceCoverageSnapshot{
			BindingID:          binding.BindingID,
			ContractVersion:    referenceCoverageFieldIndexContractVersion,
			ContextHash:        contextHash,
			InventoryHash:      inventoryHash,
			SnapshotHash:       snapshotHash,
			SourceMessageCount: len(activeContext),
			FieldCount:         len(fields),
			CoveredFieldCount:  coveredCount,
			StatsJSON:          string(statsJSON),
		}
		persisted, reused := false, false
		if canPersist {
			changed, err := coverageStore.ReplaceSessionReferenceCoverageSnapshot(ctx, snapshot, fields)
			if err != nil {
				warnings = append(warnings, "reference_coverage_snapshot_failed:"+binding.BindingID+": "+err.Error())
				persistenceFailures++
			} else {
				persisted = true
				reused = !changed
				if changed {
					summary.SnapshotWrites++
				} else {
					summary.SnapshotReuses++
				}
			}
		}
		summary.Bindings = append(summary.Bindings, referenceCoverageFieldIndexBindingSummary{
			BindingID:          binding.BindingID,
			WorkID:             binding.WorkID,
			ContinuityID:       binding.ContinuityID,
			ContextHash:        contextHash,
			InventoryHash:      inventoryHash,
			SnapshotHash:       snapshotHash,
			SourceMessageCount: len(activeContext),
			FieldCount:         len(fields),
			CoveredFieldCount:  coveredCount,
			SnapshotPersisted:  persisted,
			SnapshotReused:     reused,
		})
		summary.InventoryFields += len(fields)
		summary.EligibleFields += eligibleCount
		summary.BlockedFields += len(fields) - eligibleCount
		summary.CoveredFields += coveredCount
		summary.MissingFields += eligibleCount - coveredCount

		for _, needed := range referenceCoverageNeededSources(scope, fields, query, sceneContext) {
			summary.NeededSources++
			switch needed.CoverageStatus {
			case "covered":
				summary.CoveredSources++
			case "partial":
				summary.PartialSources++
			case "missing":
				summary.MissingSources++
			case "not_applicable":
				summary.BlockedSources++
			}
			if len(summary.NeededSourceItems) < 100 {
				summary.NeededSourceItems = append(summary.NeededSourceItems, needed)
			} else {
				summary.NeededTruncated = true
			}
		}
	}
	summary.BindingCount = len(summary.Bindings)
	if summary.BindingCount == 0 {
		summary.Status = "empty"
	} else if canPersist && persistenceFailures == 0 {
		summary.Status = "ready"
	} else if canPersist {
		summary.Status = "degraded"
	} else {
		summary.Status = "computed_not_persisted"
	}
	return summary, warnings
}

func referenceCoverageActiveContextMessages(query string, messages []map[string]any, sceneContext referenceCoverageSceneContext) []referenceCoverageMessage {
	excluded := map[string]bool{}
	if normalized := referenceCoverageNormalize(query); normalized != "" {
		excluded[normalized] = true
	}
	for _, source := range sceneContext.Conversation {
		if normalized := referenceCoverageNormalize(source.Text); normalized != "" {
			excluded[normalized] = true
		}
	}
	out := []referenceCoverageMessage{}
	seen := map[string]bool{}
	for _, message := range referenceCoverageMessages(messages) {
		if excluded[message.normalized] || seen[message.normalized] {
			continue
		}
		seen[message.normalized] = true
		out = append(out, message)
	}
	return out
}

func referenceCoverageContextHash(messages []referenceCoverageMessage) string {
	values := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.normalized != "" {
			values = append(values, message.normalized)
		}
	}
	sort.Strings(values)
	return referenceCoverageHash(values...)
}

func referenceCoverageInventoryFields(scope referenceRecallScope, activeContext []referenceCoverageMessage) []store.SessionReferenceCoverageField {
	fields := []store.SessionReferenceCoverageField{}
	appendField := func(kind, sourceID, fieldName, fieldValue string, matchValues []string, eligible bool, eligibilityReason string) {
		fieldValue = strings.TrimSpace(fieldValue)
		matchValues = referenceCoverageUniqueStrings(append(matchValues, fieldValue))
		if fieldValue == "" || len(matchValues) == 0 {
			return
		}
		locations := referenceCoverageNameLocations(matchValues, activeContext).All
		matchJSON, _ := json.Marshal(matchValues)
		locationsJSON, _ := json.Marshal(locations)
		fieldKey := referenceCoverageHash(scope.binding.BindingID, kind, sourceID, fieldName, referenceCoverageNormalize(fieldValue))
		fields = append(fields, store.SessionReferenceCoverageField{
			BindingID:            scope.binding.BindingID,
			FieldKey:             fieldKey,
			WorkID:               scope.binding.WorkID,
			ContinuityID:         scope.binding.ContinuityID,
			ReferenceKind:        kind,
			SourceID:             sourceID,
			FieldName:            fieldName,
			FieldValue:           fieldValue,
			NormalizedValue:      referenceCoverageNormalize(fieldValue),
			MatchValuesJSON:      string(matchJSON),
			PresentInContext:     len(locations) > 0,
			MatchedLocationsJSON: string(locationsJSON),
			Eligible:             eligible,
			EligibilityReason:    eligibilityReason,
		})
	}

	for _, id := range referenceCoverageSortedEntityIDs(scope.entities) {
		item := scope.entities[id]
		appendField("entity", id, "canonical_name", item.CanonicalName, nil, true, "eligible")
		for _, alias := range scope.aliases[id] {
			appendField("entity", id, "alias", alias, nil, true, "eligible")
		}
		appendField("entity", id, "entity_type", item.EntityType, nil, true, "eligible")
		appendField("entity", id, "description_text", item.DescriptionText, nil, true, "eligible")
		referenceCoverageAppendMetadataFields("entity", id, item.MetadataJSON, true, "eligible", appendField)
	}
	for _, id := range referenceCoverageSortedClaimIDs(scope.claims) {
		item := scope.claims[id]
		eligible, reason := referenceRecallClaimEligible(scope, item)
		appendField("claim", id, "claim_type", item.ClaimType, nil, eligible, reason)
		if item.SubjectEntityID != "" {
			appendField("claim", id, "subject_entity_id", item.SubjectEntityID, referenceCoverageEntityNames(scope, item.SubjectEntityID), eligible, reason)
		}
		appendField("claim", id, "claim_text", item.ClaimText, nil, eligible, reason)
		referenceCoverageAppendMetadataFields("claim", id, item.MetadataJSON, eligible, reason, appendField)
	}
	for _, id := range referenceCoverageSortedNodeIDs(scope.nodes) {
		item := scope.nodes[id]
		eligible, reason := referenceRecallTimelineEligible(scope, item)
		appendField("timeline", id, "node_key", item.NodeKey, []string{item.Label}, eligible, reason)
		appendField("timeline", id, "label", item.Label, []string{item.NodeKey}, eligible, reason)
		appendField("timeline", id, "node_kind", item.NodeKind, nil, eligible, reason)
		referenceCoverageAppendMetadataFields("timeline", id, item.MetadataJSON, eligible, reason, appendField)
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].FieldKey < fields[j].FieldKey
	})
	return fields
}

func referenceCoverageAppendMetadataFields(kind, sourceID, raw string, eligible bool, reason string, appendField func(string, string, string, string, []string, bool, string)) {
	var value any
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &value) != nil {
		return
	}
	flat := map[string]string{}
	referenceCoverageFlattenMetadata(value, "metadata", 0, flat)
	keys := make([]string, 0, len(flat))
	for key := range flat {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 64 {
		keys = keys[:64]
	}
	for _, key := range keys {
		appendField(kind, sourceID, key, flat[key], nil, eligible, reason)
	}
}

func referenceCoverageFlattenMetadata(value any, path string, depth int, out map[string]string) {
	if depth > 4 || len(out) >= 64 {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			referenceCoverageFlattenMetadata(typed[key], path+"."+key, depth+1, out)
		}
	case []any:
		for index, item := range typed {
			if index >= 32 {
				break
			}
			referenceCoverageFlattenMetadata(item, fmt.Sprintf("%s[%d]", path, index), depth+1, out)
		}
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			out[path] = referenceCoverageBoundFieldValue(text)
		}
	case float64, bool:
		out[path] = fmt.Sprint(typed)
	}
}

func referenceCoverageBoundFieldValue(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 2000 {
		return value
	}
	return string(runes[:2000])
}

func referenceCoverageInventoryHash(fields []store.SessionReferenceCoverageField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, strings.Join([]string{
			field.FieldKey,
			field.FieldValue,
			field.MatchValuesJSON,
			fmt.Sprint(field.Eligible),
			field.EligibilityReason,
		}, "|"))
	}
	sort.Strings(parts)
	return referenceCoverageHash(parts...)
}

func referenceCoverageEligibleFieldCount(fields []store.SessionReferenceCoverageField) int {
	count := 0
	for _, field := range fields {
		if field.Eligible {
			count++
		}
	}
	return count
}

func referenceCoverageNeededSources(scope referenceRecallScope, fields []store.SessionReferenceCoverageField, query string, sceneContext referenceCoverageSceneContext) []referenceCoverageNeededSource {
	bySource := map[string][]store.SessionReferenceCoverageField{}
	for _, field := range fields {
		key := field.ReferenceKind + ":" + field.SourceID
		bySource[key] = append(bySource[key], field)
	}
	keys := make([]string, 0, len(bySource))
	for key := range bySource {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []referenceCoverageNeededSource{}
	for _, key := range keys {
		group := bySource[key]
		if len(group) == 0 {
			continue
		}
		item := referenceCoverageSyntheticItem(scope, group[0].ReferenceKind, group[0].SourceID)
		neededBy := referenceCoverageNeededBy(item, scope, query, sceneContext)
		if len(neededBy) == 0 {
			continue
		}
		out = append(out, referenceCoverageSummarizeNeededSource(scope, group, neededBy))
	}
	return out
}

func referenceCoverageSyntheticItem(scope referenceRecallScope, kind, sourceID string) referenceRecallItem {
	item := referenceRecallItem{BindingID: scope.binding.BindingID, WorkID: scope.binding.WorkID, ContinuityID: scope.binding.ContinuityID, ReferenceKind: kind, SourceID: sourceID, Metadata: map[string]any{}}
	switch kind {
	case "entity":
		if value, ok := scope.entities[sourceID]; ok {
			item.Text = strings.TrimSpace(value.CanonicalName + ": " + value.DescriptionText)
		}
	case "claim":
		if value, ok := scope.claims[sourceID]; ok {
			item.Text = value.ClaimText
		}
	case "timeline":
		if value, ok := scope.nodes[sourceID]; ok {
			item.Text = value.Label
		}
	}
	return item
}

func referenceCoverageSummarizeNeededSource(scope referenceRecallScope, fields []store.SessionReferenceCoverageField, neededBy []string) referenceCoverageNeededSource {
	first := fields[0]
	result := referenceCoverageNeededSource{
		BindingID:         first.BindingID,
		WorkID:            first.WorkID,
		ContinuityID:      first.ContinuityID,
		ReferenceKind:     first.ReferenceKind,
		SourceID:          first.SourceID,
		NeededBy:          append([]string{}, neededBy...),
		Eligible:          true,
		EligibilityReason: "eligible",
		MatchedLocations:  []string{},
		MissingFields:     []string{},
	}
	anyPresent := false
	locations := []string{}
	for _, field := range fields {
		if !field.Eligible {
			result.Eligible = false
			result.EligibilityReason = field.EligibilityReason
		}
		if field.PresentInContext {
			anyPresent = true
			locations = append(locations, referenceCoverageJSONStrings(field.MatchedLocationsJSON)...)
		}
	}
	result.MatchedLocations = referenceCoverageUniqueStrings(locations)
	if !result.Eligible {
		result.CoverageStatus = "not_applicable"
		return result
	}
	if first.ReferenceKind == "entity" {
		identityPresent := referenceCoverageFieldNamePresent(fields, "canonical_name") || referenceCoverageFieldNamePresent(fields, "alias")
		descriptionRequired := referenceCoverageFieldExists(fields, "description_text")
		descriptionPresent := !descriptionRequired || referenceCoverageFieldNamePresent(fields, "description_text")
		if !identityPresent {
			result.MissingFields = append(result.MissingFields, "identity_name")
		}
		if !descriptionPresent {
			result.MissingFields = append(result.MissingFields, "description_text")
		}
		switch {
		case identityPresent && descriptionPresent:
			result.CoverageStatus = "covered"
		case anyPresent:
			result.CoverageStatus = "partial"
		default:
			result.CoverageStatus = "missing"
		}
		return result
	}
	required := referenceCoverageRequiredFieldNames(first.ReferenceKind, fields)
	coveredRequired := 0
	for _, name := range required {
		if referenceCoverageFieldNamePresent(fields, name) {
			coveredRequired++
		} else {
			result.MissingFields = append(result.MissingFields, name)
		}
	}
	switch {
	case len(required) > 0 && coveredRequired == len(required):
		result.CoverageStatus = "covered"
	case anyPresent:
		result.CoverageStatus = "partial"
	default:
		result.CoverageStatus = "missing"
	}
	return result
}

func referenceCoverageRequiredFieldNames(kind string, fields []store.SessionReferenceCoverageField) []string {
	switch kind {
	case "claim":
		return []string{"claim_text"}
	case "timeline":
		if referenceCoverageFieldExists(fields, "label") {
			return []string{"label"}
		}
		return []string{"node_key"}
	default:
		return nil
	}
}

func referenceCoverageFieldExists(fields []store.SessionReferenceCoverageField, name string) bool {
	for _, field := range fields {
		if field.FieldName == name {
			return true
		}
	}
	return false
}

func referenceCoverageFieldNamePresent(fields []store.SessionReferenceCoverageField, name string) bool {
	for _, field := range fields {
		if field.FieldName == name && field.PresentInContext {
			return true
		}
	}
	return false
}

func referenceCoverageJSONStrings(raw string) []string {
	values := []string{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func referenceCoverageHash(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(digest[:])
}

func referenceCoverageSortedEntityIDs(values map[string]store.ReferenceEntity) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func referenceCoverageSortedClaimIDs(values map[string]store.ReferenceClaim) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func referenceCoverageSortedNodeIDs(values map[string]store.ReferenceTimelineNode) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
