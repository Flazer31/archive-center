package store

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

const CharacterFieldProvenanceContract = "character_field_provenance.v1"

// DecodeCharacterFieldProvenance returns per-field evidence without inventing
// missing time, perspective, or source coordinates. Additional explicit metadata
// stays intact so occurrence/effective/learned times remain separate dimensions.
func DecodeCharacterFieldProvenance(raw string) map[string]map[string]any {
	var envelope struct {
		Fields map[string]map[string]any `json:"fields"`
	}
	if json.Unmarshal([]byte(raw), &envelope) != nil || envelope.Fields == nil {
		return map[string]map[string]any{}
	}
	return envelope.Fields
}

// CharacterFieldProvenanceForPath resolves a JSON pointer. Arrays have one
// provenance entry for the entire value, never an identity for their indexes.
func CharacterFieldProvenanceForPath(fields map[string]map[string]any, pointer string) map[string]any {
	for pointer != "" {
		if metadata, found := fields[pointer]; found {
			return metadata
		}
		index := strings.LastIndex(pointer, "/")
		if index < 0 {
			break
		}
		pointer = pointer[:index]
	}
	return nil
}

// CharacterStateFieldValues uses the public category names and RFC 6901 escape
// rules. Objects are independent leaves; arrays remain whole values because an
// index does not identify the same person or possession across snapshots.
func CharacterStateFieldValues(state CharacterState) map[string]any {
	fields := map[string]any{}
	var visit func(string, any)
	visit = func(path string, value any) {
		if object, ok := value.(map[string]any); ok && len(object) > 0 {
			for key, child := range object {
				escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
				visit(path+"/"+escaped, child)
			}
			return
		}
		fields[path] = value
	}
	for category, raw := range map[string]string{
		"appearance": state.AppearanceJSON, "personality": state.PersonalityJSON,
		"status": state.StatusJSON, "relationships": state.RelationshipsJSON,
		"speech_style": state.SpeechStyleJSON,
	} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = raw
		}
		visit("/"+category, value)
	}
	return fields
}

// MergeCharacterStateFieldProvenance runs after the existing category merge.
// source_turn dates observation, not occurrence. recorded_turn dates the initial
// stored field version in source_session_id; the row's TurnIndex separately
// dates the current cumulative snapshot. Unchanged legacy values remain unknown.
func MergeCharacterStateFieldProvenance(previous *CharacterState, next CharacterState) string {
	priorValues, priorMetadata := map[string]any{}, map[string]map[string]any{}
	if previous != nil {
		priorValues = CharacterStateFieldValues(*previous)
		priorMetadata = DecodeCharacterFieldProvenance(previous.FieldProvenanceJSON)
	}
	explicit := DecodeCharacterFieldProvenance(next.FieldProvenanceJSON)
	fields := map[string]map[string]any{}
	for path, value := range CharacterStateFieldValues(next) {
		metadata := map[string]any{}
		priorValue, existed := priorValues[path]
		if existed && reflect.DeepEqual(priorValue, value) {
			for key, item := range CharacterFieldProvenanceForPath(priorMetadata, path) {
				metadata[key] = item
			}
		} else {
			if next.TurnIndex > 0 {
				metadata["source_turn"], metadata["recorded_turn"] = next.TurnIndex, next.TurnIndex
			}
			if next.ChatSessionID != "" {
				metadata["source_session_id"] = next.ChatSessionID
			}
		}
		for key, item := range CharacterFieldProvenanceForPath(explicit, path) {
			metadata[key] = item
		}
		fields[path] = metadata
	}
	raw, _ := json.Marshal(map[string]any{"contract_version": CharacterFieldProvenanceContract, "fields": fields})
	return string(raw)
}

// BuildCharacterFieldProvenanceFromHistory repairs the latest supplied snapshot
// from the continuous history of each unchanged leaf. Callers provide complete
// history. The final (highest ID) row represents each turn; discarded same-turn
// rows are not evidence for a field's earlier origin. This derives observation
// time only and never rewrites historical rows or invents occurrence dates.
func BuildCharacterFieldProvenanceFromHistory(history []CharacterState) string {
	if len(history) == 0 {
		return ""
	}
	ordered := append([]CharacterState(nil), history...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].TurnIndex == ordered[j].TurnIndex {
			return ordered[i].ID > ordered[j].ID
		}
		return ordered[i].TurnIndex > ordered[j].TurnIndex
	})
	turns := ordered[:0]
	for _, state := range ordered {
		if len(turns) == 0 || turns[len(turns)-1].TurnIndex != state.TurnIndex {
			turns = append(turns, state)
		}
	}
	current := turns[0]
	fields := map[string]map[string]any{}
	currentMetadata := DecodeCharacterFieldProvenance(current.FieldProvenanceJSON)
	priorValues := make([]map[string]any, len(turns))
	priorMetadata := make([]map[string]map[string]any, len(turns))
	for i, state := range turns {
		priorValues[i] = CharacterStateFieldValues(state)
		priorMetadata[i] = DecodeCharacterFieldProvenance(state.FieldProvenanceJSON)
	}
	for path, value := range CharacterStateFieldValues(current) {
		metadata := map[string]any{}
		for key, item := range CharacterFieldProvenanceForPath(currentMetadata, path) {
			metadata[key] = item
		}
		oldest := current
		for i, prior := range turns[1:] {
			priorValue, exists := priorValues[i+1][path]
			if !exists || !reflect.DeepEqual(value, priorValue) {
				break
			}
			oldest = prior
			for key, item := range CharacterFieldProvenanceForPath(priorMetadata[i+1], path) {
				if _, exists := metadata[key]; !exists {
					metadata[key] = item
				}
			}
		}
		if _, known := metadata["source_turn"]; !known && oldest.TurnIndex > 0 {
			metadata["source_turn"] = oldest.TurnIndex
			metadata["provenance_basis"] = "earliest_continuous_snapshot"
		}
		if _, known := metadata["recorded_turn"]; !known && oldest.TurnIndex > 0 {
			metadata["recorded_turn"] = oldest.TurnIndex
		}
		if _, known := metadata["source_session_id"]; !known && oldest.ChatSessionID != "" {
			metadata["source_session_id"] = oldest.ChatSessionID
		}
		fields[path] = metadata
	}
	raw, _ := json.Marshal(map[string]any{"contract_version": CharacterFieldProvenanceContract, "fields": fields})
	return string(raw)
}
