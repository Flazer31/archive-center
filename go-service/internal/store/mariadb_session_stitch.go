package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const SessionMigrationModeStitch = "stitch_keep_sources"

type SessionStitchRequest struct {
	SourceSessionIDs        []string            `json:"source_session_ids"`
	CurrentSessionID        string              `json:"current_session_id"`
	OperationID             string              `json:"operation_id"`
	RebuildPublicProjection func(string) string `json:"-"`
}

type SessionStitchSegment struct {
	SessionID   string `json:"session_id"`
	Ordinal     int    `json:"ordinal"`
	Offset      int    `json:"offset"`
	ThroughTurn int    `json:"through_turn"`
}

type SessionStitchResult struct {
	MigrationID              int64                  `json:"migration_id"`
	TargetSessionID          string                 `json:"target_session_id"`
	Segments                 []SessionStitchSegment `json:"segments"`
	CurrentOffset            int                    `json:"current_offset"`
	CurrentSourceSessionID   string                 `json:"current_source_session_id"`
	CurrentSourceSessionIDs  []string               `json:"current_source_session_ids"`
	CurrentInputGroupAliases map[string][]string    `json:"current_input_group_aliases,omitempty"`
	EntityIDMap              map[string]string      `json:"-"`
}

type SessionStitchStore interface {
	StitchSessions(context.Context, SessionStitchRequest) (*SessionStitchResult, error)
}

// The request order is the author's story order. The active chat is always the
// last segment. Existing source rows are read in one transaction and never
// updated, locked for future use, or deleted by this operation.
func (m *mariadbStore) StitchSessions(ctx context.Context, req SessionStitchRequest) (*SessionStitchResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	current := strings.TrimSpace(req.CurrentSessionID)
	if current == "" || strings.TrimSpace(req.OperationID) == "" {
		return nil, errors.New("current_session_id and operation_id are required")
	}
	ids := []string{}
	seen := map[string]bool{current: true}
	for _, id := range req.SourceSessionIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("select at least one previous session")
	}
	ids = append(ids, current)
	target := "stitch_" + sessionMigrationStringHash("session-stitch.v1", req.OperationID, strings.Join(ids, "\x1f"))[:40]
	conn, err := m.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var acquired int
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 60)`, target).Scan(&acquired); err != nil {
		return nil, err
	}
	if acquired != 1 {
		return nil, errors.New("session stitch is still processing; retry this operation")
	}
	defer conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, target)
	// Use the existing migration ledger for retry and restart recovery.
	var migrationID int64
	var note string
	err = conn.QueryRowContext(ctx, `SELECT id, COALESCE(operator_note,'') FROM session_migrations WHERE target_session_id=? AND mode=? AND status NOT IN ('rolled_back','rollback_partial') ORDER BY id DESC LIMIT 1`, target, SessionMigrationModeStitch).Scan(&migrationID, &note)
	if err == nil {
		var result SessionStitchResult
		if err := json.Unmarshal([]byte(note), &result); err != nil {
			return nil, err
		}
		result.MigrationID, result.TargetSessionID = migrationID, target
		result.EntityIDMap, err = m.sessionStitchEntityMap(ctx, migrationID)
		return &result, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	manifest := SessionMigrationManifest()
	snapshots := make([]map[string][]sessionMigrationRow, len(ids))
	targetRows := map[string][]sessionMigrationRow{}
	for _, entry := range manifest {
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		if err := sessionMigrationValidateSchemaTx(ctx, tx, plan); err != nil {
			return nil, err
		}
		for i, id := range ids {
			if snapshots[i] == nil {
				snapshots[i] = map[string][]sessionMigrationRow{}
			}
			rows, err := sessionMigrationReadManifestRows(ctx, tx, entry, plan, id)
			if err != nil {
				return nil, err
			}
			snapshots[i][entry.Table] = rows
		}
		targetRows[entry.Table], err = sessionMigrationReadManifestRows(ctx, tx, entry, plan, target)
		if err != nil {
			return nil, err
		}
	}
	// A stitched current chat still displays only its own tail, not the earlier
	// segments already imported into it. Keep that boundary through another join.
	var previousNote string
	previous := SessionStitchResult{CurrentSourceSessionID: current}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(operator_note,'') FROM session_migrations WHERE target_session_id=? AND mode=? AND status NOT IN ('rolled_back','rollback_partial') ORDER BY id DESC LIMIT 1`, current, SessionMigrationModeStitch).Scan(&previousNote); err == nil {
		if err := json.Unmarshal([]byte(previousNote), &previous); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rows, segments, err := sessionStitchSnapshot(ids, snapshots, current, req.RebuildPublicProjection, previous.CurrentOffset)
	if err != nil {
		return nil, err
	}
	result := &SessionStitchResult{TargetSessionID: target, Segments: segments, CurrentOffset: segments[len(segments)-1].Offset + previous.CurrentOffset, CurrentSourceSessionID: previous.CurrentSourceSessionID}
	result.CurrentSourceSessionIDs = append([]string{current}, previous.CurrentSourceSessionIDs...)
	result.CurrentInputGroupAliases = sessionStitchInputGroupAliases(snapshots[len(snapshots)-1]["audit_logs"], previous.CurrentInputGroupAliases)
	encoded, _ := json.Marshal(result)
	execution, err := completeSessionMigrationSnapshotTx(ctx, tx, SessionMigrationCompleteRequest{
		SourceSessionID: current, TargetSessionID: target, Mode: SessionMigrationModeStitch, OperatorNote: string(encoded),
	}, rows, targetRows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result.MigrationID, result.EntityIDMap = execution.MigrationID, execution.EntityIDMap
	return result, nil
}

// Preserve the existing acceptance journal's group-member identities without
// copying its old-session lifecycle decisions or transient provider requests.
func sessionStitchInputGroupAliases(rows []sessionMigrationRow, prior map[string][]string) map[string][]string {
	result := map[string][]string{}
	appendIDs := func(key string, ids []string) {
		for _, id := range ids {
			found := false
			for _, old := range result[key] {
				if old == id {
					found = true
					break
				}
			}
			if id != "" && !found {
				result[key] = append(result[key], id)
			}
		}
	}
	for key, ids := range prior {
		appendIDs(key, ids)
	}
	for _, row := range rows {
		if row.Values["event_type"].Text != "source_acceptance_transition" {
			continue
		}
		var state struct {
			LogicalTurnID      string   `json:"logical_turn_id"`
			UserLogicalTurnIDs []string `json:"user_logical_turn_ids"`
		}
		if json.Unmarshal([]byte(row.Values["details_json"].Text), &state) == nil && state.LogicalTurnID != "" {
			appendIDs(state.LogicalTurnID, state.UserLogicalTurnIDs)
		}
	}
	return result
}

func (m *mariadbStore) sessionStitchEntityMap(ctx context.Context, id int64) (map[string]string, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT source_key, target_key FROM session_migration_artifact_row_map WHERE migration_id=? AND table_name='entity_identities' AND key_column_name='stable_entity_id'`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var a, b string
		if err := rows.Scan(&a, &b); err != nil {
			return nil, err
		}
		out[a] = b
	}
	return out, rows.Err()
}

// Explicit coordinate names only: duration, age, dates and prose are not turns.
func sessionStitchTurnColumn(key string) bool {
	switch key {
	case "turn_index", "source_turn", "source_turn_index", "source_turn_start", "source_turn_end", "turn_anchor", "first_seen_turn", "last_seen_turn", "first_turn", "last_turn", "last_evidence_turn", "created_turn", "resolved_turn", "last_verified_turn", "from_turn", "to_turn", "paid_turn", "cleared_turn", "valid_from_turn", "valid_to_turn", "candidate_source_turn":
		return true
	}
	return false
}

func sessionStitchJSON(raw string, offset int, replacements map[string]string) string {
	var value any
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return raw
	}
	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, item := range v {
				visit(item)
			}
		case map[string]any:
			for key, item := range v {
				if sessionStitchTurnColumn(key) {
					if n, ok := item.(json.Number); ok {
						if number, err := strconv.Atoi(n.String()); err == nil && number > 0 {
							v[key] = number + offset
						}
					}
				}
				if text, ok := item.(string); ok {
					switch key {
					case "entity_id", "character_id", "subject_entity_id", "actor_entity_id", "source_revision", "logical_turn_id", "source_logical_turn_id":
						if replacement, ok := replacements[text]; ok {
							v[key] = replacement
						}
					}
				}
				visit(item)
			}
		}
	}
	visit(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func sessionStitchSnapshot(ids []string, snapshots []map[string][]sessionMigrationRow, current string, rebuild func(string) string, currentImportedThrough ...int) (map[string][]sessionMigrationRow, []SessionStitchSegment, error) {
	out := map[string][]sessionMigrationRow{}
	segments := []SessionStitchSegment{}
	offset := 0
	for i, source := range snapshots {
		if i > 0 {
			for _, row := range source["chat_logs"] {
				if sessionMigrationCellInt(row.Values["turn_index"]) == 0 {
					offset++
					break
				}
			}
		}
		maxTurn := 0
		for table, rows := range source {
			plan, _ := SessionMigrationExecutionPlanFor(table)
			for _, row := range rows {
				for _, col := range plan.Columns {
					if sessionStitchTurnColumn(col) {
						if n := sessionMigrationCellInt(row.Values[col]); n > maxTurn {
							maxTurn = n
						}
					}
				}
			}
		}
		segments = append(segments, SessionStitchSegment{SessionID: ids[i], Ordinal: i + 1, Offset: offset, ThroughTurn: offset + maxTurn})
		// Retain old-copy public projection decisions without repairing originals.
		ops, err := sessionMigrationMemoryProjectionOperations(source["memory_vector_outbox"], source["memory_source_revisions"], ids[i])
		if err != nil {
			return nil, nil, err
		}
		active := sessionMigrationActiveSourceRevisions(source["memory_source_revisions"])
		byTurn := map[int]sessionMigrationRow{}
		for _, row := range source["memory_source_revisions"] {
			if _, ok := active[row.Values["source_revision"].Text]; ok {
				byTurn[sessionMigrationCellInt(row.Values["turn_index"])] = row
			}
		}
		for _, memory := range source["memories"] {
			id := "memory:" + ids[i] + ":" + memory.Values["id"].Text
			if _, ok := ops[id]; ok {
				continue
			}
			rev, ok := byTurn[sessionMigrationCellInt(memory.Values["turn_index"])]
			if !ok || rebuild == nil {
				continue
			}
			if err := sessionMigrationValidateAdmissionResult(rev); err != nil {
				return nil, nil, err
			}
			text := rebuild(rev.Values["derived_result_json"].Text)
			operation := "upsert"
			if strings.TrimSpace(text) == "" {
				operation = "delete"
			}
			ops[id] = sessionMigrationMemoryProjectionOperation{Operation: operation, DocumentText: text, SourceRevision: rev.Values["source_revision"].Text}
		}
		source["memory_vector_outbox"] = nil
		for _, memory := range source["memories"] {
			id := "memory:" + ids[i] + ":" + memory.Values["id"].Text
			op, ok := ops[id]
			if !ok {
				continue
			}
			docID := "memory:" + current + ":" + memory.Values["id"].Text
			document, _ := json.Marshal(map[string]any{"ID": docID, "Tier": "memory", "ChatSessionID": current, "SourceTable": "memories", "SourceRowID": memory.Values["id"].Text, "SchemaVersion": "memory.v2", "DocumentText": op.DocumentText, "Metadata": memoryVectorVerificationMetadata(op.SourceRevision, MemorySourceRevisionContract, MemoryPublicProjectionIndex, op.DocumentText)})
			r := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
			for k, v := range map[string]string{"id": memory.Values["id"].Text, "contract_version": MemoryVectorOutboxContract, "document_id": docID, "document_json": string(document), "operation": op.Operation, "source_revision": op.SourceRevision, "status": "completed"} {
				r.Values[k] = sessionMigrationCell{Valid: true, Text: v}
			}
			source["memory_vector_outbox"] = append(source["memory_vector_outbox"], r)
		}
		logical := map[string]string{}
		for _, r := range source["memory_source_revisions"] {
			id := r.Values["logical_turn_id"].Text
			olderCurrent := len(currentImportedThrough) > 0 && sessionMigrationCellInt(r.Values["turn_index"]) <= currentImportedThrough[0]
			if id != "" && (i < len(snapshots)-1 || olderCurrent) {
				logical[id] = "stitch-" + sessionMigrationStringHash(ids[i], id)
			}
		}
		for _, entry := range SessionMigrationManifest() {
			for _, original := range source[entry.Table] {
				if entry.Table == "memory_source_revisions" {
					if err := sessionMigrationValidateAdmissionResult(original); err != nil {
						return nil, nil, err
					}
				}
				row := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
				for col, cell := range original.Values {
					if sessionStitchTurnColumn(col) && cell.Valid {
						if n, err := strconv.Atoi(cell.Text); err == nil && n > 0 {
							cell.Text = strconv.Itoa(n + offset)
						}
					}
					if strings.HasSuffix(col, "_json") && col != "critic_input_snapshot_json" && cell.Valid {
						cell.Text = sessionStitchJSON(cell.Text, offset, logical)
					}
					if col == "logical_turn_id" || col == "source_logical_turn_id" {
						if v, ok := logical[cell.Text]; ok {
							cell.Text = v
						}
					}
					if col == "idempotency_key" && cell.Valid && cell.Text != "" {
						cell.Text = sessionMigrationStringHash(ids[i], cell.Text)
					}
					row.Values[col] = cell
				}
				if entry.Direct {
					row.Values[entry.SessionColumn] = sessionMigrationCell{Valid: true, Text: current}
				}
				if entry.Table == "memory_vector_outbox" {
					id := row.Values["document_id"]
					id.Text = strings.Replace(id.Text, "memory:"+ids[i]+":", "memory:"+current+":", 1)
					row.Values["document_id"] = id
				}
				if entry.Table == "memory_source_revisions" {
					hash, _, err := sessionMigrationRemappedAdmissionResult(row)
					if err != nil {
						return nil, nil, err
					}
					if hash != "" {
						row.Values["derived_result_hash"] = sessionMigrationCell{Valid: true, Text: hash}
					}
				}
				// A later chat's greeting is still source text, at its own boundary.
				if entry.Table == "chat_logs" && i > 0 && sessionMigrationCellInt(row.Values["turn_index"]) == 0 {
					row.Values["turn_index"] = sessionMigrationCell{Valid: true, Text: strconv.Itoa(offset)}
				}
				out[entry.Table] = append(out[entry.Table], row)
			}
		}
		offset += maxTurn
	}
	// These tables represent current values or unique configuration, not event
	// history. Later segments replace the same named slot; histories remain.
	aliases := map[string]map[string]string{}
	coalesce := func(table string, columns ...string) {
		plan, _ := SessionMigrationExecutionPlanFor(table)
		if len(plan.PrimaryKey) != 1 {
			return
		}
		pk := plan.PrimaryKey[0]
		last := map[string]int{}
		keys := make([]string, len(out[table]))
		for i, row := range out[table] {
			parts := []string{}
			for _, col := range columns {
				parts = append(parts, row.Values[col].Text)
			}
			key := strings.Join(parts, "\x1f")
			keys[i] = key
			last[key] = i
		}
		kept := []sessionMigrationRow{}
		aliases[table] = map[string]string{}
		for i, row := range out[table] {
			winner := out[table][last[keys[i]]]
			aliases[table][row.Values[pk].Text] = winner.Values[pk].Text
			if last[keys[i]] == i {
				kept = append(kept, row)
			}
		}
		out[table] = kept
	}
	coalesce("guidance_plan_states")
	coalesce("session_active_scopes")
	coalesce("lorebook_reference_session_locks")
	coalesce("persona_capsule_attachments", "capsule_id")
	coalesce("status_schema_registry", "schema_name", "status_key", "owner_scope")
	coalesce("session_reference_bindings", "work_id", "continuity_id")
	coalesce("entities", "entity_type", "name")
	coalesce("trust_states", "target_type", "target_name")
	coalesce("world_rules", "scope", "scope_name", "category", "key")
	coalesce("pending_threads", "thread_key")
	coalesce("storylines", "name")
	// Stable identities remain separate: matching display names are not identity evidence.
	remap := func() {
		for table, rows := range out {
			plan, _ := SessionMigrationExecutionPlanFor(table)
			for _, row := range rows {
				for _, fk := range plan.ForeignKeys {
					cell := row.Values[fk.Column]
					if next, ok := aliases[fk.ReferenceTable][cell.Text]; ok {
						cell.Text = next
						row.Values[fk.Column] = cell
					}
				}
				if plan.ParentColumn != "" {
					if next, ok := aliases["session_reference_bindings"][row.Values[plan.ParentColumn].Text]; ok {
						row.Values[plan.ParentColumn] = sessionMigrationCell{Valid: true, Text: next}
					}
				}
			}
		}
	}
	remap()
	out["session_fork_lineage"] = nil // Continuation order is recorded in the operation, not as a branch.
	coalesce("status_current_values", "registry_id", "owner_scope", "owner_id")
	coalesce("entity_identity_links", "source_entity_id", "target_entity_id", "link_kind")
	remap()
	// Latest settings for a shared work own its indirect runtime rows.
	for _, table := range []string{"session_reference_runtime", "session_reference_coverage_snapshots"} {
		coalesce(table, "binding_id")
	}
	// The migration owner regenerates reference coverage rather than copying it.
	for _, row := range out["memory_source_revisions"] {
		hash, _, err := sessionMigrationRemappedAdmissionResult(row)
		if err != nil {
			return nil, nil, err
		}
		if hash != "" {
			row.Values["derived_result_hash"] = sessionMigrationCell{Valid: true, Text: hash}
		}
	}
	return out, segments, nil
}

// Keep the selected order explicit in the durable operation note.
func sessionStitchCurrentOffset(raw string) (int, bool) {
	var result SessionStitchResult
	if json.Unmarshal([]byte(raw), &result) != nil || len(result.Segments) == 0 {
		return 0, false
	}
	return result.CurrentOffset, true
}

var _ SessionStitchStore = (*mariadbStore)(nil)
