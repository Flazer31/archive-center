package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type bodyRepairTable struct {
	table, sessionColumn, identityColumns, vectorTier string
	clear                                             map[string]string
}

// Existing materializations keep their row identities and dependency links;
// only their readable content is cleared. Undo restores exact nullable fields.
// The backup lives solely in the existing explicit state-repair history.
var bodyRepairTables = []bodyRepairTable{
	{"memories", "chat_session_id", "turn_index", "memory", map[string]string{"summary_json": "{}", "evidence": "[]", "embedding": "[]"}},
	{"direct_evidence_records", "chat_session_id", "", "evidence", map[string]string{"evidence_text": "", "tombstoned": "1"}},
	{"precise_memory_units", "chat_session_id", "subject_entity_id,actor_entity_id,affected_entity_id,root_evidence_id", "precise_memory", map[string]string{"payload_json": "{}", "evidence_excerpt": "", "lifecycle_state": "invalidated"}},
	{"kg_triples", "chat_session_id", "subject,object", "", map[string]string{"subject": "", "predicate": "", "object": ""}},
	{"protagonist_entity_memories", "source_chat_session_id", "owner_entity_key,owner_entity_name,source_character_name", "", map[string]string{"memory_text": "", "evidence_excerpt": "", "tags_json": "[]"}},
	{"episode_summaries", "chat_session_id", "key_entities", "episode", map[string]string{"summary_text": "", "key_entities": "[]", "key_events": "[]", "open_loops_json": "[]", "relationship_changes_json": "[]", "embedding_vector": "[]"}},
	{"chapter_summaries", "chat_session_id", "", "chapter", map[string]string{"chapter_title": "", "summary_text": "", "open_loops_json": "[]", "relationship_changes_json": "[]", "world_changes_json": "[]", "callback_candidates_json": "[]", "resume_text": "", "embedding_vector": "[]"}},
	{"arc_summaries", "chat_session_id", "", "arc", map[string]string{"arc_name": "", "core_conflict": "", "key_turning_points_json": "[]", "active_promises_json": "[]", "unresolved_debts_json": "[]", "resolved_payoffs_json": "[]", "callback_candidates_json": "[]", "future_payoff_candidates_json": "[]", "irreversible_turns_json": "[]", "callback_debts_json": "[]", "relationship_pivots_json": "[]", "arc_resume_text": "", "embedding_vector": "[]"}},
	{"saga_digests", "chat_session_id", "", "saga", map[string]string{"saga_summary": "", "persistent_facts_json": "[]", "never_drop_candidates_json": "[]", "resume_pack_text": "", "embedding_vector": "[]"}},
	{"character_states", "chat_session_id", "character_name", "", map[string]string{"appearance_json": "{}", "status_json": "{}", "field_provenance_json": "{}"}},
	{"character_events", "chat_session_id", "character_name", "", map[string]string{"details_json": "{}"}},
	{"status_current_values", "chat_session_id", "owner_id,owner_label", "", map[string]string{"value_json": "{}"}},
	{"status_change_events", "chat_session_id", "owner_id", "", map[string]string{"previous_value_json": "{}", "new_value_json": "{}"}},
	{"pending_threads", "chat_session_id", "", "", map[string]string{"description": "", "hook_metadata_json": "{}", "suppressed": "1", "user_corrected": "1"}},
	{"active_states", "chat_session_id", "", "", map[string]string{"content": "{}"}},
	{"canonical_state_layers", "chat_session_id", "", "", map[string]string{"content": "{}"}},
}

var bodyRepairTopic = regexp.MustCompile(`(?i)pregnan|gestation|conception|conceiv|implantation|childbirth|gave birth|postpartum|menstrua|period_start|recovery_started|임신|잉태|수태|착상|출산|산후|월경|생리`)

func bodyRepairColumns(spec bodyRepairTable) []string {
	keys := make([]string, 0, len(spec.clear))
	for key := range spec.clear {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func bodyRepairSpec(table string) (bodyRepairTable, error) {
	for _, spec := range bodyRepairTables {
		if spec.table == table {
			return spec, nil
		}
	}
	return bodyRepairTable{}, fmt.Errorf("unsupported repair artifact %q", table)
}

func (m *mariadbStore) ListBodyRepairArtifacts(ctx context.Context, sid, entityID, name string) ([]StateRepairArtifactChange, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	turns, evidenceIDs, err := m.bodyRepairSourceLinks(ctx, sid, entityID)
	if err != nil {
		return nil, err
	}
	out := []StateRepairArtifactChange{}
	for _, spec := range bodyRepairTables {
		columns := bodyRepairColumns(spec)
		selectColumns := strings.Join(columns, ",")
		if spec.identityColumns != "" {
			selectColumns += "," + spec.identityColumns
		}
		where := ""
		if spec.table == "status_current_values" || spec.table == "status_change_events" {
			where = " AND status_key NOT IN ('body_tracking','story_clock')"
		}
		rows, err := m.db.QueryContext(ctx, "SELECT id,"+selectColumns+" FROM "+spec.table+" WHERE "+spec.sessionColumn+" = ?"+where+" ORDER BY id", sid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			values := make([]sql.NullString, len(strings.Split(selectColumns, ",")))
			args := []any{&id}
			for i := range values {
				args = append(args, &values[i])
			}
			if err := rows.Scan(args...); err != nil {
				rows.Close()
				return nil, err
			}
			texts := []string{}
			for _, v := range values {
				texts = append(texts, v.String)
			}
			text := strings.Join(texts, "\n")
			// This selects reviewable memory units, not a new inference about the
			// character. Mixed-content units are shown in the delete preview.
			linked := spec.table == "direct_evidence_records" && evidenceIDs[id]
			if spec.table == "memories" {
				turn, _ := strconv.Atoi(values[len(values)-1].String)
				linked = turns[turn]
			}
			if spec.table == "precise_memory_units" {
				evidenceID, _ := strconv.ParseInt(values[len(values)-1].String, 10, 64)
				linked = evidenceIDs[evidenceID]
			}
			if !linked && (!bodyRepairTopic.MatchString(text) || !(entityID != "" && strings.Contains(text, entityID) || name != "" && strings.Contains(strings.ToLower(text), strings.ToLower(name)))) {
				continue
			}
			change := StateRepairArtifactChange{Table: spec.table, ID: id, Before: map[string]*string{}, After: map[string]*string{}}
			bound := false
			for _, value := range values[len(columns):] {
				if value.String == entityID || value.String == name {
					bound = true
				}
			}
			for i, key := range columns {
				v := spec.clear[key]
				if bodyRepairPartialTable(spec.table) {
					if !values[i].Valid {
						continue
					}
					v = bodyRepairPruneJSON(values[i].String, entityID, name, bound)
				}
				if values[i].Valid && values[i].String == v {
					continue
				}
				if values[i].Valid {
					before := values[i].String
					change.Before[key] = &before
				} else {
					change.Before[key] = nil
				}
				change.After[key] = &v
			}
			if len(change.After) > 0 {
				out = append(out, change)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	// Old and hierarchy vectors can predate the source outbox. Their explicit
	// repair uses the existing vector API and keeps its snapshot in this backup.
	for i := range out {
		spec, _ := bodyRepairSpec(out[i].Table)
		if spec.vectorTier == "" {
			continue
		}
		ids, err := stateRepairVectorIDs(ctx, m.db, sid, spec, out[i].ID)
		if err != nil {
			return nil, err
		}
		var count int
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id=? AND document_id=? AND operation='upsert'", sid, ids[0]).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			out[i].VectorIDs, out[i].VectorAfterJSON = ids, "[]"
		}
	}
	return out, nil
}

func (m *mariadbStore) bodyRepairSourceLinks(ctx context.Context, sid, entityID string) (map[int]bool, map[int64]bool, error) {
	turns, ids := map[int]bool{}, map[int64]bool{}
	rows, err := m.db.QueryContext(ctx, `SELECT e.source_turn,e.evidence_json FROM status_change_events e
		JOIN memory_source_revisions s ON s.chat_session_id=e.chat_session_id AND s.source_revision=JSON_UNQUOTE(JSON_EXTRACT(e.evidence_json,'$.source_revision'))
		WHERE e.chat_session_id=? AND e.owner_id=? AND e.status_key='body_tracking' AND s.lifecycle_state='active'`, sid, entityID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var turn sql.NullInt64
		var raw string
		if err := rows.Scan(&turn, &raw); err != nil {
			return nil, nil, err
		}
		if turn.Valid && turn.Int64 > 0 {
			turns[int(turn.Int64)] = true
		}
		var evidence struct {
			IDs []int64 `json:"direct_evidence_ids"`
		}
		if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
			return nil, nil, err
		}
		for _, id := range evidence.IDs {
			ids[id] = true
		}
	}
	return turns, ids, rows.Err()
}

func bodyRepairPartialTable(table string) bool {
	switch table {
	case "character_states", "character_events", "status_current_values", "status_change_events", "active_states", "canonical_state_layers":
		return true
	}
	return false
}

// Character/state snapshots contain unrelated fields in the same row. Remove
// only the body-related fields; do not erase identity, clothing or other status.
func bodyRepairPruneJSON(raw, entityID, name string, bound bool) string {
	var value any
	mentions := func(text string) bool {
		return entityID != "" && strings.Contains(text, entityID) || name != "" && strings.Contains(strings.ToLower(text), strings.ToLower(name))
	}
	if json.Unmarshal([]byte(raw), &value) != nil {
		if bound || mentions(raw) {
			return "{}"
		}
		return raw
	}
	removed := false
	var prune func(any, bool) (any, bool)
	prune = func(v any, owned bool) (any, bool) {
		switch item := v.(type) {
		case map[string]any:
			for _, key := range []string{"subject_entity_id", "character_id", "character_name", "subject_label", "subject", "name", "owner_id"} {
				if id, ok := item[key].(string); ok && id != "" {
					owned = id == entityID || id == name
					break // An explicit nested subject replaces inherited ownership.
				}
			}
			out := map[string]any{}
			for key, child := range item {
				if owned && bodyRepairTopic.MatchString(key) {
					removed = true
					continue
				}
				if cleaned, keep := prune(child, owned || key == entityID || key == name); keep {
					out[key] = cleaned
				}
			}
			return out, len(out) > 0
		case []any:
			out := []any{}
			for _, child := range item {
				if cleaned, keep := prune(child, owned); keep {
					out = append(out, cleaned)
				}
			}
			return out, len(out) > 0
		case string:
			if bodyRepairTopic.MatchString(item) && (owned || mentions(item)) {
				removed = true
				return nil, false
			}
			return item, true
		default:
			return item, true
		}
	}
	out, keep := prune(value, bound)
	if !removed {
		return raw
	}
	if !keep {
		return "{}"
	}
	data, _ := json.Marshal(out)
	return string(data)
}

// Rebase only the recorded JSON changes onto the row locked by this operation.
// The stored before/after pair already describes both delete and restore deltas.
func bodyRepairApplyJSONDelta(current, before, after string) string {
	var currentValue, beforeValue, afterValue any
	if json.Unmarshal([]byte(current), &currentValue) != nil || json.Unmarshal([]byte(before), &beforeValue) != nil || json.Unmarshal([]byte(after), &afterValue) != nil {
		return after
	}
	if reflect.DeepEqual(currentValue, beforeValue) {
		return after
	}
	if reflect.DeepEqual(beforeValue, afterValue) {
		return current
	}
	empty := func(value any) any {
		switch value.(type) {
		case map[string]any:
			return map[string]any{}
		case []any:
			return []any{}
		}
		return nil
	}
	// Arrays keep their current order and unrelated additions. Named entries are
	// matched by their existing identity; anonymous objects retain positional edits.
	identity := func(value any) string {
		item, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		for _, key := range []string{"subject_entity_id", "character_id", "character_name", "subject_label", "subject", "name", "owner_id", "id"} {
			if value, exists := item[key]; exists && value != nil && value != "" {
				raw, _ := json.Marshal(value)
				return key + ":" + string(raw)
			}
		}
		return ""
	}
	match := func(values []any, target any, used map[int]bool) int {
		id := identity(target)
		for i, value := range values {
			if !used[i] && (id != "" && identity(value) == id || reflect.DeepEqual(value, target)) {
				return i
			}
		}
		return -1
	}
	var apply func(any, any, any) any
	apply = func(current, before, after any) any {
		if reflect.DeepEqual(before, after) {
			return current
		}
		oldArray, oldList := before.([]any)
		newArray, newList := after.([]any)
		if oldList && newList {
			out, _ := current.([]any)
			out = append([]any{}, out...)
			usedOld, usedCurrent := map[int]bool{}, map[int]bool{}
			removed := map[int]bool{}
			for nextIndex, next := range newArray {
				oldIndex := match(oldArray, next, usedOld)
				if oldIndex < 0 && len(oldArray) == len(newArray) && identity(next) == "" {
					if _, object := next.(map[string]any); object && !usedOld[nextIndex] {
						oldIndex = nextIndex
					}
				}
				if oldIndex < 0 {
					out = append(out, next)
					continue
				}
				usedOld[oldIndex] = true
				old := oldArray[oldIndex]
				currentIndex := match(out, old, usedCurrent)
				if currentIndex < 0 && identity(old) == "" && oldIndex < len(out) {
					if _, object := old.(map[string]any); object && !usedCurrent[oldIndex] {
						currentIndex = oldIndex
					}
				}
				if currentIndex >= 0 {
					usedCurrent[currentIndex] = true
					out[currentIndex] = apply(out[currentIndex], old, next)
				} else if !reflect.DeepEqual(old, next) {
					out = append(out, next)
				}
			}
			for i, old := range oldArray {
				if !usedOld[i] {
					if currentIndex := match(out, old, usedCurrent); currentIndex >= 0 {
						removed[currentIndex], usedCurrent[currentIndex] = true, true
					}
				}
			}
			result := []any{}
			for i, value := range out {
				if !removed[i] {
					result = append(result, value)
				}
			}
			return result
		}
		oldMap, oldObject := before.(map[string]any)
		newMap, newObject := after.(map[string]any)
		if oldObject && newObject {
			out, _ := current.(map[string]any)
			if out == nil {
				out = map[string]any{}
			}
			for key, old := range oldMap {
				if next, exists := newMap[key]; exists {
					if !reflect.DeepEqual(old, next) {
						out[key] = apply(out[key], old, next)
					}
				} else {
					// A container removed by deletion can have new unrelated children now.
					if blank := empty(old); blank != nil {
						retained := apply(out[key], old, blank)
						if !reflect.DeepEqual(retained, blank) {
							out[key] = retained
							continue
						}
					}
					delete(out, key)
				}
			}
			for key, next := range newMap {
				if _, exists := oldMap[key]; !exists {
					out[key] = apply(out[key], empty(next), next)
				}
			}
			return out
		}
		return after
	}
	result := apply(currentValue, beforeValue, afterValue)
	raw, _ := json.Marshal(result)
	return string(raw)
}

func applyStateRepairArtifactsTx(ctx context.Context, tx *sql.Tx, sid, operation string, changes []StateRepairArtifactChange, now time.Time) ([]StateRepairArtifactChange, error) {
	result := make([]StateRepairArtifactChange, 0, len(changes))
	for _, change := range changes {
		spec, err := bodyRepairSpec(change.Table)
		if err != nil {
			return nil, err
		}
		columns := bodyRepairColumns(spec)
		values := make([]sql.NullString, len(columns))
		args := []any{}
		for i := range values {
			args = append(args, &values[i])
		}
		err = tx.QueryRowContext(ctx, "SELECT "+strings.Join(columns, ",")+" FROM "+spec.table+" WHERE "+spec.sessionColumn+" = ? AND id = ? FOR UPDATE", sid, change.ID).Scan(args...)
		if err == sql.ErrNoRows {
			continue
		} // Existing source deletion owns removed rows.
		if err != nil {
			return nil, err
		}
		plannedBefore := change.Before
		change.Before = map[string]*string{}
		set, params := []string{}, []any{}
		for i, key := range columns {
			if _, exists := change.After[key]; !exists {
				continue
			}
			if values[i].Valid {
				v := values[i].String
				change.Before[key] = &v
			} else {
				change.Before[key] = nil
			}
			if value, exists := change.After[key]; exists {
				if bodyRepairPartialTable(spec.table) && values[i].Valid && plannedBefore[key] != nil && value != nil {
					merged := bodyRepairApplyJSONDelta(values[i].String, *plannedBefore[key], *value)
					value = &merged
					change.After[key] = value
				}
				set = append(set, key+" = ?")
				params = append(params, value)
			}
		}
		if len(set) == 0 {
			continue
		}
		params = append(params, sid, change.ID)
		if _, err := tx.ExecContext(ctx, "UPDATE "+spec.table+" SET "+strings.Join(set, ",")+" WHERE "+spec.sessionColumn+" = ? AND id = ?", params...); err != nil {
			return nil, err
		}
		if spec.vectorTier != "" {
			if len(change.VectorIDs) > 0 {
				change.VectorIDs, err = stateRepairVectorIDs(ctx, tx, sid, spec, change.ID)
				if err != nil {
					return nil, err
				}
			}
			if err := repairArtifactVectorTx(ctx, tx, sid, operation, spec, change, now); err != nil {
				return nil, err
			}
		}
		result = append(result, change)
	}
	return result, nil
}

func repairArtifactVectorTx(ctx context.Context, tx *sql.Tx, sid, operation string, spec bodyRepairTable, change StateRepairArtifactChange, now time.Time) error {
	ids, err := stateRepairVectorIDs(ctx, tx, sid, spec, change.ID)
	if err != nil {
		return err
	}
	documentID := ids[0]
	var revision, state string
	var document sql.NullString
	var ready bool
	err = tx.QueryRowContext(ctx, `SELECT o.source_revision, s.lifecycle_state, o.document_json, o.embedding_ready
		FROM memory_vector_outbox o JOIN memory_source_revisions s ON s.chat_session_id=o.chat_session_id AND s.source_revision=o.source_revision
		WHERE o.chat_session_id=? AND o.document_id=? AND o.operation='upsert' ORDER BY o.id DESC LIMIT 1 FOR UPDATE`, sid, documentID).Scan(&revision, &state, &document, &ready)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	cleared := true
	for key, value := range change.After {
		if value != nil && *value != spec.clear[key] {
			cleared = false
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memory_vector_outbox SET status='stale_rejected', lease_owner=NULL, lease_until=NULL, retry_after=NULL, last_error='explicit_body_data_repair', updated_at=? WHERE chat_session_id=? AND document_id=? AND status IN ('pending','retryable','needs_embedding','leased')`, now, sid, documentID); err != nil {
		return err
	}
	item := &MemoryVectorOutboxItem{OperationKey: memoryVectorOperationKey("repair:"+operation, sid, revision, documentID), ChatSessionID: sid, SourceRevision: revision, DocumentID: documentID, Status: "pending", CreatedAt: now, UpdatedAt: now, RequiredSourceState: "active", EmbeddingReady: ready}
	if state != "active" {
		item.RequiredSourceState = "inactive"
	}
	if cleared {
		item.Operation, item.DocumentJSON, item.EmbeddingReady = "delete", memoryVectorDeleteAuditJSON("explicit_body_data_repair"), true
	} else {
		item.Operation, item.DocumentJSON = "upsert", document.String
	}
	_, err = enqueueMemoryVectorOperation(ctx, tx, item)
	return err
}

func stateRepairVectorIDs(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, sid string, spec bodyRepairTable, id int64) ([]string, error) {
	key := strconv.FormatInt(id, 10)
	if spec.table == "precise_memory_units" {
		if err := db.QueryRowContext(ctx, "SELECT unit_id FROM precise_memory_units WHERE chat_session_id=? AND id=?", sid, id).Scan(&key); err != nil {
			return nil, err
		}
	}
	return []string{spec.vectorTier + ":" + sid + ":" + key, spec.vectorTier + ":" + key}, nil
}

func stateRepairArtifactsEvidence(raw string, changes []StateRepairArtifactChange) string {
	var evidence map[string]any
	_ = json.Unmarshal([]byte(raw), &evidence)
	if evidence == nil {
		evidence = map[string]any{}
	}
	evidence["repair_artifacts"] = changes
	data, _ := json.Marshal(evidence)
	return string(data)
}
