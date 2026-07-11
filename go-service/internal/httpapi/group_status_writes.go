package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func statusCurrentValueFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusCurrentValueWriteRequest) (store.StatusSchemaDefinition, store.StatusCurrentValue, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("owner_id is required")
	}
	valueJSON, err := statusSchemaCompactRawJSON(req.ValueJSON, "value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	valueKind := statusSchemaNormalizeValueKind(definition.ValueKind)
	if valueKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("registered value_kind is invalid")
	}
	if err := statusCurrentValueMatchesKind(valueJSON, valueKind); err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	return definition, store.StatusCurrentValue{
		ChatSessionID: sid,
		RegistryID:    definition.ID,
		StatusKey:     definition.StatusKey,
		OwnerScope:    definition.OwnerScope,
		OwnerID:       ownerID,
		OwnerLabel:    strings.TrimSpace(req.OwnerLabel),
		ValueKind:     valueKind,
		ValueJSON:     valueJSON,
		EvidenceJSON:  evidenceJSON,
		SourceTurn:    req.SourceTurn,
		WriteState:    "current",
	}, nil
}

func statusChangeEventFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusChangeEventWriteRequest) (store.StatusSchemaDefinition, store.StatusChangeEvent, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("owner_id is required")
	}
	eventKind := statusNormalizeEventKind(req.EventKind)
	if eventKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("event_kind must be one of set, increase, decrease, clear, effect_applied, effect_expired, effect_cleared")
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	previousValueJSON, err := statusSchemaCompactOptionalRawJSON(req.PreviousValueJSON, "previous_value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	newValueJSON, err := statusSchemaCompactOptionalRawJSON(req.NewValueJSON, "new_value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	if eventKind != "clear" && eventKind != "effect_expired" && eventKind != "effect_cleared" && newValueJSON == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("new_value_json is required for this event_kind")
	}
	storyClockJSON, err := statusSchemaCompactOptionalRawJSON(req.StoryClockJSON, "story_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	if statusSchemaNormalizeValueKind(definition.ValueKind) == "derived" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("derived status events are projection-only")
	}
	return definition, store.StatusChangeEvent{
		ChatSessionID:     sid,
		RegistryID:        definition.ID,
		StatusValueID:     req.StatusValueID,
		StatusKey:         definition.StatusKey,
		OwnerScope:        definition.OwnerScope,
		OwnerID:           ownerID,
		EventKind:         eventKind,
		PreviousValueJSON: previousValueJSON,
		NewValueJSON:      newValueJSON,
		EvidenceJSON:      evidenceJSON,
		SourceTurn:        req.SourceTurn,
		StoryClockJSON:    storyClockJSON,
		EventState:        "recorded",
	}, nil
}

func statusEffectFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusEffectWriteRequest) (store.StatusSchemaDefinition, store.StatusEffect, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("owner_id is required")
	}
	effectKind := statusNormalizeEffectKind(req.EffectKind)
	if effectKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("effect_kind must be one of temporary_effect, buff, debuff, injury, cooldown")
	}
	state := statusNormalizeEffectState(firstNonEmptyStringLocal(req.EffectState, "active"))
	if state == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	startClockJSON, err := statusSchemaCompactJSONObject(req.StartClockJSON, "start_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	durationJSON, err := statusSchemaCompactOptionalJSONObject(req.DurationJSON, "duration_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	expiresJSON, err := statusSchemaCompactOptionalJSONObject(req.ExpiresAtClockJSON, "expires_at_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	if durationJSON == "" && expiresJSON == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("duration_json or expires_at_clock_json is required")
	}
	payloadJSON, err := statusSchemaCompactOptionalRawJSON(req.EffectPayloadJSON, "effect_payload_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	if statusSchemaNormalizeValueKind(definition.ValueKind) == "derived" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("derived status effects are projection-only")
	}
	return definition, store.StatusEffect{
		ChatSessionID:      sid,
		RegistryID:         definition.ID,
		StatusKey:          definition.StatusKey,
		OwnerScope:         definition.OwnerScope,
		OwnerID:            ownerID,
		EffectKind:         effectKind,
		EffectLabel:        strings.TrimSpace(req.EffectLabel),
		EffectPayloadJSON:  payloadJSON,
		EvidenceJSON:       evidenceJSON,
		SourceTurn:         req.SourceTurn,
		StartClockJSON:     startClockJSON,
		DurationJSON:       durationJSON,
		ExpiresAtClockJSON: expiresJSON,
		EffectState:        state,
	}, nil
}

func statusEffectStateUpdateFromRequest(req statusEffectStateRequest) (string, string, error) {
	state := statusNormalizeEffectState(req.EffectState)
	if state == "" {
		return "", "", errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	evidenceJSON, err := statusSchemaCompactOptionalRawJSON(req.ClearedEvidenceJSON, "cleared_evidence_json")
	if err != nil {
		return "", "", err
	}
	if (state == "expired" || state == "cleared") && evidenceJSON == "" {
		return "", "", errors.New("cleared_evidence_json is required for expired or cleared effect_state")
	}
	return state, evidenceJSON, nil
}

func statusCurrentValueMatchesKind(valueJSON, valueKind string) error {
	var value any
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return errors.New("value_json must be valid JSON")
	}
	switch valueKind {
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("boolean status value must be true or false")
		}
	case "tags":
		if _, ok := value.([]any); !ok {
			return errors.New("tags status value must be a JSON array")
		}
	case "note":
		if _, ok := value.(string); !ok {
			return errors.New("note status value must be a JSON string")
		}
	case "enum":
		if _, ok := value.(string); !ok {
			return errors.New("enum status value must be a JSON string")
		}
	case "derived":
		return errors.New("derived status values are projection-only and cannot be written directly")
	}
	return nil
}
