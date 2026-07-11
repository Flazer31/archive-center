package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	physicalConditionIngestContractVersion = "physical_condition_ingest.v1"
	physicalConditionStatusKey             = "physical_condition"
	entityConditionIngestContractVersion   = "entity_condition_ingest.v1"
	entityConditionStatusKey               = "entity_condition"
)

type conditionEffectLane struct {
	ExtractionKey     string
	StatusKey         string
	SchemaName        string
	Label             string
	OwnerScope        string
	ContractVersion   string
	SourceLabel       string
	EntityTypeDefault string
	CharacterOwner    bool
	Options           map[string]any
}

func (s *Server) savePhysicalConditionsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	s.saveConditionEffectsFromExtraction(ctx, sid, turnIndex, extraction, now, result, conditionEffectLane{
		ExtractionKey:   "physical_conditions",
		StatusKey:       physicalConditionStatusKey,
		SchemaName:      "physical_condition_status",
		Label:           "Physical condition",
		OwnerScope:      "character",
		ContractVersion: physicalConditionIngestContractVersion,
		SourceLabel:     "critic.physical_conditions",
		CharacterOwner:  true,
		Options: map[string]any{
			"condition_lane":                  true,
			"authority_mode":                  "archive_canonical",
			"projection_density":              "light",
			"duration_policy":                 "evidence_bound_no_default_duration",
			"severity_policy":                 "descriptive_no_numeric_scale",
			"runtime_status_override_allowed": true,
		},
	})
}

func (s *Server) saveEntityConditionsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	s.saveConditionEffectsFromExtraction(ctx, sid, turnIndex, extraction, now, result, conditionEffectLane{
		ExtractionKey:     "entity_conditions",
		StatusKey:         entityConditionStatusKey,
		SchemaName:        "entity_condition_status",
		Label:             "Entity condition",
		OwnerScope:        "entity",
		ContractVersion:   entityConditionIngestContractVersion,
		SourceLabel:       "critic.entity_conditions",
		EntityTypeDefault: "entity",
		Options: map[string]any{
			"condition_lane":                  true,
			"entity_condition_lane":           true,
			"authority_mode":                  "archive_canonical",
			"projection_density":              "light",
			"duration_policy":                 "evidence_bound_no_default_duration",
			"severity_policy":                 "descriptive_no_numeric_scale",
			"runtime_status_override_allowed": true,
		},
	})
}

func (s *Server) saveConditionEffectsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult, lane conditionEffectLane) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	conditions := normalizePhysicalConditionItems(extraction[lane.ExtractionKey])
	if len(conditions) == 0 {
		return
	}
	registry, hasRegistry := s.Store.(store.StatusSchemaRegistryStore)
	lifecycle, hasLifecycle := s.Store.(store.StatusLifecycleStore)
	if !hasRegistry || !hasLifecycle {
		result.addSkipReason(lane.ExtractionKey, "status_schema_lifecycle_store_unavailable", map[string]any{"items": len(conditions)})
		return
	}
	definition, ok := s.ensureConditionStatusDefinition(ctx, sid, now, result, registry, lane.StatusKey, lane.SchemaName, lane.Label, lane.OwnerScope, lane.Options, lane.ExtractionKey)
	if !ok {
		return
	}
	for _, condition := range conditions {
		owner := conditionOwnerName(condition)
		if lane.CharacterOwner {
			owner = s.canonicalCharacterName(ctx, sid, owner)
		}
		if owner == "" || isPlaceholderKGPart(owner) {
			result.addSkipReason(lane.ExtractionKey, "missing_owner", condition)
			continue
		}
		evidence := physicalConditionEvidence(condition)
		if evidence == "" {
			result.addSkipReason(lane.ExtractionKey, "missing_evidence_excerpt", condition)
			continue
		}
		label := conditionLabel(condition)
		if label == "" {
			result.addSkipReason(lane.ExtractionKey, "missing_condition_label", condition)
			continue
		}
		effectKind := statusNormalizeEffectKind(stringFromMap(condition, "effect_kind"))
		if effectKind == "" {
			effectKind = "temporary_effect"
		}
		effectState := statusNormalizeEffectState(extractionFirstNonEmpty(stringFromMap(condition, "effect_state"), "active"))
		if effectState == "" {
			effectState = "active"
		}
		payload := physicalConditionPayload(condition, turnIndex)
		payload["contract_version"] = lane.ContractVersion
		if lane.EntityTypeDefault != "" {
			payload["entity_type"] = extractionFirstNonEmpty(stringFromMap(condition, "owner_entity_type"), stringFromMap(condition, "entity_type"), lane.EntityTypeDefault)
		}
		evidenceJSON := mustCompactJSON(map[string]any{
			"contract_version": lane.ContractVersion,
			"source":           lane.SourceLabel,
			"source_turn":      turnIndex,
			"evidence_excerpt": evidence,
			"authority_hint":   stringFromMap(condition, "authority_hint"),
		})
		result.trySave("SaveStatusEffect("+lane.StatusKey+")", func() error {
			_, err := lifecycle.SaveStatusEffect(ctx, store.StatusEffect{
				ChatSessionID:      sid,
				RegistryID:         definition.ID,
				StatusKey:          definition.StatusKey,
				OwnerScope:         definition.OwnerScope,
				OwnerID:            owner,
				EffectKind:         effectKind,
				EffectLabel:        label,
				EffectPayloadJSON:  mustCompactJSON(payload),
				EvidenceJSON:       evidenceJSON,
				SourceTurn:         turnIndex,
				StartClockJSON:     physicalConditionStartClockJSON(condition, turnIndex),
				DurationJSON:       physicalConditionDurationJSON(condition),
				ExpiresAtClockJSON: physicalConditionExpiresAtClockJSON(condition),
				EffectState:        effectState,
				CreatedAt:          now,
				UpdatedAt:          now,
			})
			return err
		}, result, func() {
			if lane.ExtractionKey == "physical_conditions" {
				result.PhysicalConditions++
			}
			if lane.ExtractionKey == "entity_conditions" {
				result.EntityConditions++
			}
			result.StatusEffects++
		})
	}
}

func (s *Server) ensureConditionStatusDefinition(ctx context.Context, sid string, now time.Time, result *artifactSaveResult, registry store.StatusSchemaRegistryStore, statusKey, schemaName, label, ownerScope string, options map[string]any, skipKey string) (store.StatusSchemaDefinition, bool) {
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		if result != nil {
			result.addSkipReason(skipKey, "status_schema_lookup_failed", err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	definition = store.StatusSchemaDefinition{
		ChatSessionID: sid,
		SchemaName:    schemaName,
		StatusKey:     statusKey,
		Label:         label,
		OwnerScope:    ownerScope,
		ValueKind:     "note",
		OptionsJSON:   mustCompactJSON(options),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	var saved []store.StatusSchemaDefinition
	if result == nil {
		var saveErr error
		saved, saveErr = registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{definition})
		return firstSavedPhysicalConditionDefinition(definition, saved), saveErr == nil
	}
	result.trySave("SaveStatusSchemaDefinitions("+statusKey+")", func() error {
		var saveErr error
		saved, saveErr = registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{definition})
		return saveErr
	}, result, func() { result.StatusSchemaDefinitions++ })
	if len(saved) == 0 {
		return store.StatusSchemaDefinition{}, false
	}
	return firstSavedPhysicalConditionDefinition(definition, saved), true
}

func firstSavedPhysicalConditionDefinition(fallback store.StatusSchemaDefinition, saved []store.StatusSchemaDefinition) store.StatusSchemaDefinition {
	if len(saved) == 0 {
		return fallback
	}
	return saved[0]
}

func normalizePhysicalConditionItems(raw any) []map[string]any {
	out := []map[string]any{}
	for _, item := range sliceFromAny(raw) {
		m := mapFromAny(item)
		if len(m) == 0 || !hasMeaningfulPayload(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func physicalConditionEvidence(item map[string]any) string {
	if text := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "evidence_excerpt"),
		stringFromMap(item, "evidence"),
		stringFromMap(item, "source_excerpt"),
	)); text != "" {
		return text
	}
	excerpts := stringsFromAny(item["evidence_excerpts"])
	if len(excerpts) == 0 {
		return ""
	}
	return excerpts[0]
}

func physicalConditionPayload(item map[string]any, turnIndex int) map[string]any {
	payload := map[string]any{
		"contract_version":          physicalConditionIngestContractVersion,
		"source_turn":               turnIndex,
		"condition":                 item,
		"duration_policy":           extractionFirstNonEmpty(stringFromMap(item, "duration_policy"), "unknown_until_updated"),
		"hardcoded_duration":        false,
		"numeric_severity_required": false,
	}
	if text := strings.TrimSpace(stringFromMap(item, "severity_text")); text != "" {
		payload["severity_text"] = text
	}
	if text := strings.TrimSpace(stringFromMap(item, "age_or_vulnerability_note")); text != "" {
		payload["age_or_vulnerability_note"] = text
	}
	if text := strings.TrimSpace(stringFromMap(item, "uncertainty_note")); text != "" {
		payload["uncertainty_note"] = text
	}
	return payload
}

func physicalConditionStartClockJSON(item map[string]any, turnIndex int) string {
	for _, key := range []string{"start_clock_json", "onset_story_clock_json", "story_clock_json"} {
		if raw := mapFromAny(item[key]); hasMeaningfulPayload(raw) {
			return mustCompactJSON(raw)
		}
	}
	return mustCompactJSON(map[string]any{
		"source_turn":      turnIndex,
		"precision":        "turn",
		"precision_label":  "turn_anchor",
		"calendar_unknown": true,
	})
}

func physicalConditionDurationJSON(item map[string]any) string {
	if raw := mapFromAny(item["duration_json"]); hasMeaningfulPayload(raw) {
		return mustCompactJSON(raw)
	}
	if raw := mapFromAny(item["expires_at_clock_json"]); hasMeaningfulPayload(raw) {
		return ""
	}
	return mustCompactJSON(map[string]any{
		"policy":             "unknown_until_updated",
		"reason":             "no_explicit_duration_in_evidence",
		"hardcoded_duration": false,
	})
}

func physicalConditionExpiresAtClockJSON(item map[string]any) string {
	if raw := mapFromAny(item["expires_at_clock_json"]); hasMeaningfulPayload(raw) {
		return mustCompactJSON(raw)
	}
	return ""
}

func conditionOwnerName(item map[string]any) string {
	return strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "owner_entity_name"),
		stringFromMap(item, "owner_entity_key"),
		stringFromMap(item, "owner_name"),
		stringFromMap(item, "entity_name"),
		stringFromMap(item, "character_name"),
		stringFromMap(item, "subject"),
		stringFromMap(item, "name"),
	))
}

func conditionLabel(item map[string]any) string {
	return strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "condition_label"),
		stringFromMap(item, "condition"),
		stringFromMap(item, "status_label"),
		stringFromMap(item, "summary"),
	))
}

func entityDescriptionWithConditions(base, name, entityType string, physicalConditions, entityConditions []map[string]any) string {
	base = strings.TrimSpace(base)
	nameKey := comparableEntityKey(name)
	if nameKey == "" {
		return base
	}
	var candidates []map[string]any
	if strings.EqualFold(strings.TrimSpace(entityType), "character") {
		candidates = physicalConditions
	} else {
		candidates = entityConditions
	}
	parts := []string{}
	for _, condition := range candidates {
		if comparableEntityKey(conditionOwnerName(condition)) != nameKey {
			continue
		}
		label := conditionLabel(condition)
		if label == "" {
			continue
		}
		if bodyArea := strings.TrimSpace(stringFromMap(condition, "body_area")); bodyArea != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(bodyArea)) {
			label = label + " (" + bodyArea + ")"
		}
		parts = append(parts, "condition: "+label)
	}
	if len(parts) == 0 {
		return base
	}
	extra := strings.Join(dedupeStrings(parts), "; ")
	if base == "" {
		return extra
	}
	if strings.Contains(base, extra) {
		return base
	}
	return base + " | " + extra
}

func comparableEntityKey(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}
