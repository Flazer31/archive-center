package store

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

const characterManualEditEvent = "manual_character_override"

// Preserve an existing explicit voice override before an older installation's
// first replacement. Other legacy fields have no retained edit values and must
// not be guessed from old snapshots or restored as operator instructions.
const preserveLegacyCharacterManualEditsSQL = `INSERT INTO character_events
	(chat_session_id, character_name, turn_index, event_type, details_json, created_at)
	SELECT state.chat_session_id, state.character_name, 0, 'manual_character_override',
	JSON_OBJECT('source', 'manual_patch', 'recorded_turn', state.turn_index,
	'edits', JSON_ARRAY(JSON_OBJECT('path', JSON_ARRAY('speech_style', 'manual_overrides'),
	'value', JSON_EXTRACT(state.speech_style_json, '$.manual_overrides')))), CURRENT_TIMESTAMP(3)
	FROM character_states state WHERE state.chat_session_id = ? AND state.turn_index >= ?
	AND JSON_CONTAINS_PATH(state.speech_style_json, 'one', '$.manual_overrides') = 1
	AND NOT EXISTS (SELECT 1 FROM character_states newer WHERE newer.chat_session_id = state.chat_session_id
	 AND newer.character_name = state.character_name AND (newer.turn_index > state.turn_index OR (newer.turn_index = state.turn_index AND newer.id > state.id)))
	AND NOT EXISTS (SELECT 1 FROM character_events edits WHERE edits.chat_session_id = state.chat_session_id
	 AND edits.character_name = state.character_name AND edits.event_type = 'manual_character_override')`

// Objects are edited by leaf. Voice principles use their named entries;
// other arrays remain whole values (positions are not durable identities).
type CharacterManualFieldEdit struct {
	Path      []string `json:"path"`
	Value     any      `json:"value,omitempty"`
	Remove    bool     `json:"remove,omitempty"`
	Principle string   `json:"principle,omitempty"`
}

func characterEditableValues(state CharacterState) map[string]any {
	out := map[string]any{}
	for key, raw := range map[string]string{
		"appearance": state.AppearanceJSON, "personality": state.PersonalityJSON,
		"status": state.StatusJSON, "relationships": state.RelationshipsJSON,
		"speech_style": state.SpeechStyleJSON,
	} {
		var value any
		if strings.TrimSpace(raw) == "" {
			value = map[string]any{}
		} else if json.Unmarshal([]byte(raw), &value) != nil {
			value = raw
		}
		out[key] = value
	}
	return out
}

// CharacterManualPatch records only what this UI save changed, not every
// automatic field carried by the full-character form.
func CharacterManualPatch(before, after CharacterState) []CharacterManualFieldEdit {
	var edits []CharacterManualFieldEdit
	var diff func([]string, any, any)
	diff = func(path []string, old, next any) {
		if reflect.DeepEqual(old, next) {
			return
		}
		if reflect.DeepEqual(path, []string{"speech_style", "principles"}) {
			oldList, oldOK := old.([]any)
			newList, newOK := next.([]any)
			if oldOK && newOK {
				for _, list := range [][]any{oldList, newList} {
					for _, value := range list {
						if characterVoicePrincipleKey(value) == "" {
							edits = append(edits, CharacterManualFieldEdit{Path: path, Value: next})
							return
						}
					}
				}
				used := map[int]bool{}
				for i, value := range newList {
					key := characterVoicePrincipleKey(value)
					match := -1
					for j, prior := range oldList {
						if !used[j] && key != "" && characterVoicePrincipleKey(prior) == key {
							match = j
							break
						}
					}
					if match < 0 && len(oldList) == len(newList) && !used[i] {
						match = i
					}
					if match >= 0 {
						used[match] = true
						if reflect.DeepEqual(oldList[match], value) {
							continue
						}
						if priorKey := characterVoicePrincipleKey(oldList[match]); priorKey != "" {
							key = priorKey
						}
					}
					if key == "" {
						break
					}
					edits = append(edits, CharacterManualFieldEdit{Path: path, Principle: key, Value: value})
				}
				for i, value := range oldList {
					if !used[i] {
						if key := characterVoicePrincipleKey(value); key != "" {
							edits = append(edits, CharacterManualFieldEdit{Path: path, Principle: key, Remove: true})
						}
					}
				}
				return
			}
		}
		oldMap, oldObject := old.(map[string]any)
		nextMap, nextObject := next.(map[string]any)
		if oldObject && nextObject {
			keys := map[string]bool{}
			for key := range oldMap {
				keys[key] = true
			}
			for key := range nextMap {
				keys[key] = true
			}
			ordered := make([]string, 0, len(keys))
			for key := range keys {
				ordered = append(ordered, key)
			}
			sort.Strings(ordered)
			for _, key := range ordered {
				child := append(append([]string{}, path...), key)
				nextValue, exists := nextMap[key]
				if !exists {
					edits = append(edits, CharacterManualFieldEdit{Path: child, Remove: true})
				} else if oldValue, existed := oldMap[key]; existed {
					diff(child, oldValue, nextValue)
				} else if reflect.DeepEqual(child, []string{"speech_style", "principles"}) {
					diff(child, []any{}, nextValue)
				} else if object, ok := nextValue.(map[string]any); ok && len(object) > 0 {
					diff(child, map[string]any{}, nextValue)
				} else {
					edits = append(edits, CharacterManualFieldEdit{Path: child, Value: nextValue})
				}
			}
			return
		}
		edits = append(edits, CharacterManualFieldEdit{Path: path, Value: next})
	}
	diff(nil, characterEditableValues(before), characterEditableValues(after))
	return edits
}

func mergeCharacterManualEdits(prior, patch []CharacterManualFieldEdit) []CharacterManualFieldEdit {
	out := append([]CharacterManualFieldEdit{}, prior...)
	for _, edit := range patch {
		kept := out[:0]
		for _, old := range out {
			if edit.Principle != "" && old.Principle != edit.Principle && reflect.DeepEqual(old.Path, edit.Path) {
				kept = append(kept, old)
				continue
			}
			if len(old.Path) >= len(edit.Path) && reflect.DeepEqual(old.Path[:len(edit.Path)], edit.Path) {
				continue
			}
			kept = append(kept, old)
		}
		out = append(kept, edit)
	}
	return out
}

func applyCharacterManualEdits(state *CharacterState, edits []CharacterManualFieldEdit) {
	if len(edits) == 0 {
		return
	}
	values := characterEditableValues(*state)
	metadata := DecodeCharacterFieldProvenance(state.FieldProvenanceJSON)
	for _, edit := range edits {
		if len(edit.Path) == 0 {
			continue
		}
		object := values
		for _, key := range edit.Path[:len(edit.Path)-1] {
			child, _ := object[key].(map[string]any)
			if child == nil {
				child = map[string]any{}
				object[key] = child
			}
			object = child
		}
		key := edit.Path[len(edit.Path)-1]
		if edit.Principle != "" {
			if _, exists := object["manual_overrides"]; !exists {
				object["manual_overrides"] = map[string]any{}
			}
			list, _ := object[key].([]any)
			result := []any{}
			nextKey := characterVoicePrincipleKey(edit.Value)
			for _, item := range list {
				k := characterVoicePrincipleKey(item)
				if k != edit.Principle && (nextKey == "" || k != nextKey) {
					result = append(result, item)
				}
			}
			if !edit.Remove {
				raw, _ := json.Marshal(edit.Value)
				var value any
				_ = json.Unmarshal(raw, &value)
				result = append(result, value)
			}
			object[key] = result
		} else if edit.Remove {
			delete(object, key)
		} else {
			// Later child edits must not mutate the saved cumulative patch.
			raw, _ := json.Marshal(edit.Value)
			var value any
			_ = json.Unmarshal(raw, &value)
			object[key] = value
		}
		path := ""
		for _, part := range edit.Path {
			path += "/" + strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
		}
		for prior := range metadata {
			if prior == path || strings.HasPrefix(prior, path+"/") {
				delete(metadata, prior)
			}
		}
		if !edit.Remove {
			metadata[path] = map[string]any{"authority": "manual_edit", "provenance_basis": "user_character_edit"}
		}
	}
	for key, target := range map[string]*string{
		"appearance": &state.AppearanceJSON, "personality": &state.PersonalityJSON,
		"status": &state.StatusJSON, "relationships": &state.RelationshipsJSON, "speech_style": &state.SpeechStyleJSON,
	} {
		// Do not rewrite untouched JSON (including empty legacy values).
		for _, edit := range edits {
			if len(edit.Path) > 0 && edit.Path[0] == key {
				raw, _ := json.Marshal(values[key])
				*target = string(raw)
				break
			}
		}
	}
	manualKeys := []string{}
	for _, edit := range edits {
		if edit.Principle != "" && !edit.Remove {
			manualKeys = append(manualKeys, characterVoicePrincipleKey(edit.Value))
		}
	}
	if len(manualKeys) > 0 {
		metadata["/speech_style/principles"] = map[string]any{"authority": "mixed", "manual_principle_keys": manualKeys}
	}
	raw, _ := json.Marshal(map[string]any{"contract_version": CharacterFieldProvenanceContract, "fields": metadata})
	state.FieldProvenanceJSON = string(raw)
	state.manualEdits = append([]CharacterManualFieldEdit{}, edits...)
}

func characterVoicePrincipleKey(value any) string {
	object, _ := value.(map[string]any)
	key, _ := object["principle_key"].(string)
	return key
}

// Explicitly edited principles are operator guidance, not a new story quote.
// Return only the current edited entries, without source excerpts or audit IDs.
func CharacterManualVoicePrinciples(state CharacterState) []map[string]any {
	voice, _ := characterEditableValues(state)["speech_style"].(map[string]any)
	principles, _ := voice["principles"].([]any)
	keys := map[string]bool{}
	metadata := CharacterFieldProvenanceForPath(DecodeCharacterFieldProvenance(state.FieldProvenanceJSON), "/speech_style/principles")
	if values, ok := metadata["manual_principle_keys"].([]any); ok {
		for _, value := range values {
			if key, ok := value.(string); ok {
				keys[key] = true
			}
		}
	}
	out := []map[string]any{}
	for _, raw := range principles {
		if !keys[characterVoicePrincipleKey(raw)] {
			continue
		}
		entry, _ := raw.(map[string]any)
		clean := map[string]any{}
		for _, key := range []string{"principle_key", "trait_domain", "contexts", "counterparts", "state_modulations"} {
			if value, exists := entry[key]; exists {
				clean[key] = value
			}
		}
		out = append(out, clean)
	}
	return out
}

// Read the latest cumulative operator edits once per character/session read.
// Turn zero is settings scope: ordinary rollback removes derived snapshots,
// while explicit character/session deletion still removes these events.
func mariaApplyCharacterManualEdits(ctx context.Context, q mariaQueryer, sid, name string, states []CharacterState) ([]CharacterState, error) {
	rows, err := q.QueryContext(ctx, `SELECT e.character_name, e.details_json FROM character_events e
		WHERE e.chat_session_id = ? AND (? = '' OR e.character_name = ?)
		AND e.event_type = 'manual_character_override'
		AND e.id = (SELECT MAX(latest.id) FROM character_events latest
			WHERE latest.chat_session_id = e.chat_session_id AND latest.character_name = e.character_name
			AND latest.event_type = 'manual_character_override') ORDER BY e.id`, sid, name, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var character, raw string
		if err := rows.Scan(&character, &raw); err != nil {
			return nil, err
		}
		var record struct {
			Edits []CharacterManualFieldEdit `json:"edits"`
		}
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			return nil, err
		}
		if len(record.Edits) == 0 {
			continue
		}
		index := -1
		for i := range states {
			if strings.EqualFold(states[i].CharacterName, character) {
				index = i
				break
			}
		}
		if index < 0 {
			states = append(states, CharacterState{ChatSessionID: sid, CharacterName: character})
			index = len(states) - 1
		}
		applyCharacterManualEdits(&states[index], record.Edits)
	}
	return states, rows.Err()
}
