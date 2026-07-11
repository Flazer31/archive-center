package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) CreatePersonaMemoryCapsule(ctx context.Context, capsule *PersonaMemoryCapsule, entries []PersonaMemoryEntry) (*PersonaMemoryCapsule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	mode := strings.TrimSpace(capsule.Mode)
	if mode == "" {
		mode = "manual"
	}
	title := strings.TrimSpace(capsule.Title)
	if title == "" {
		title = "Persona Memory Capsule"
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO persona_memory_capsules
			(persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, capsule.PersonaKey, capsule.SourceChatSessionID, capsule.SourceCharacterName, title, mode, capsule.Summary, now, now)
	if err != nil {
		return nil, err
	}
	capsuleID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		portability := strings.TrimSpace(entry.Portability)
		if portability == "" {
			portability = "same_chat"
		}
		injectionPolicy := strings.TrimSpace(entry.InjectionPolicy)
		if injectionPolicy == "" {
			injectionPolicy = "support_only"
		}
		tagsJSON := strings.TrimSpace(entry.TagsJSON)
		if tagsJSON == "" {
			tagsJSON = "[]"
		}
		var sourceMemoryID any
		if entry.SourceMemoryID > 0 {
			sourceMemoryID = entry.SourceMemoryID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO persona_memory_entries
				(capsule_id, source_memory_type, source_memory_id, source_turn_index, memory_text, emotional_weight, importance_10, portability, tags_json, evidence_excerpt, injection_policy, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, capsuleID, nullableString(entry.SourceMemoryType), sourceMemoryID, entry.SourceTurn, entry.MemoryText, entry.EmotionalWeight, entry.Importance10, portability, tagsJSON, entry.EvidenceExcerpt, injectionPolicy, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	out := *capsule
	out.ID = capsuleID
	out.Title = title
	out.Mode = mode
	out.CreatedAt = now
	out.UpdatedAt = now
	return &out, nil
}

func (m *mariadbStore) ListPersonaMemoryCapsules(ctx context.Context, filter PersonaCapsuleFilter) ([]PersonaMemoryCapsule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at
		FROM persona_memory_capsules
		WHERE 1 = 1`
	args := []any{}
	if strings.TrimSpace(filter.PersonaKey) != "" {
		query += " AND persona_key = ?"
		args = append(args, strings.TrimSpace(filter.PersonaKey))
	}
	if strings.TrimSpace(filter.SourceChatSessionID) != "" {
		query += " AND source_chat_session_id = ?"
		args = append(args, strings.TrimSpace(filter.SourceChatSessionID))
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT 200"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryCapsule{}
	for rows.Next() {
		item, err := scanPersonaMemoryCapsule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetPersonaMemoryCapsule(ctx context.Context, capsuleID int64) (*PersonaMemoryCapsule, []PersonaMemoryEntry, error) {
	if err := m.ensureDB(); err != nil {
		return nil, nil, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT id, persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at
		FROM persona_memory_capsules
		WHERE id = ?
	`, capsuleID)
	capsule, err := scanPersonaMemoryCapsule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	entries, err := m.listPersonaMemoryEntriesByCapsule(ctx, capsuleID)
	if err != nil {
		return nil, nil, err
	}
	return &capsule, entries, nil
}

func (m *mariadbStore) DeletePersonaMemoryCapsule(ctx context.Context, capsuleID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM persona_memory_capsules WHERE id = ?", capsuleID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) AttachPersonaMemoryCapsule(ctx context.Context, attachment *PersonaCapsuleAttachment) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	mode := strings.TrimSpace(attachment.InjectionMode)
	if mode == "" {
		mode = "subtle_deja_vu"
	}
	now := time.Now().UTC()
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO persona_capsule_attachments
			(capsule_id, target_chat_session_id, injection_mode, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			injection_mode = VALUES(injection_mode),
			enabled = VALUES(enabled),
			updated_at = VALUES(updated_at)
	`, attachment.CapsuleID, attachment.TargetChatSessionID, mode, attachment.Enabled, now, now)
	return err
}

func (m *mariadbStore) DetachPersonaMemoryCapsule(ctx context.Context, capsuleID int64, targetChatSessionID string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM persona_capsule_attachments WHERE capsule_id = ? AND target_chat_session_id = ?", capsuleID, targetChatSessionID)
	return err
}

func (m *mariadbStore) ListPersonaCapsuleAttachments(ctx context.Context, targetChatSessionID string) ([]PersonaCapsuleAttachment, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, capsule_id, target_chat_session_id, injection_mode, enabled, created_at, updated_at
		FROM persona_capsule_attachments
		WHERE target_chat_session_id = ?
		ORDER BY updated_at DESC, id DESC
	`, targetChatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaCapsuleAttachment{}
	for rows.Next() {
		item, err := scanPersonaCapsuleAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListAttachedPersonaMemoryEntries(ctx context.Context, targetChatSessionID string, limit int) ([]PersonaMemoryEntry, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 80 {
		limit = 80
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT e.id, e.capsule_id, e.source_memory_type, e.source_memory_id,
		       COALESCE(p.source_turn_index, e.source_turn_index),
		       COALESCE(NULLIF(p.memory_text, ''), e.memory_text),
		       COALESCE(p.emotional_weight, e.emotional_weight),
		       COALESCE(p.importance_10, e.importance_10),
		       COALESCE(NULLIF(p.portability, ''), e.portability),
		       COALESCE(p.tags_json, e.tags_json),
		       COALESCE(NULLIF(p.evidence_excerpt, ''), e.evidence_excerpt),
		       e.injection_policy,
		       e.created_at
		FROM persona_memory_entries e
		INNER JOIN persona_capsule_attachments a ON a.capsule_id = e.capsule_id
		LEFT JOIN protagonist_entity_memories p
			ON e.source_memory_type = 'subjective_entity_memory'
			AND e.source_memory_id = p.id
		WHERE a.target_chat_session_id = ? AND a.enabled = TRUE
		ORDER BY COALESCE(p.importance_10, e.importance_10) DESC, e.id ASC
		LIMIT ?
	`, targetChatSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryEntry{}
	for rows.Next() {
		item, err := scanPersonaMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) listPersonaMemoryEntriesByCapsule(ctx context.Context, capsuleID int64) ([]PersonaMemoryEntry, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT e.id, e.capsule_id, e.source_memory_type, e.source_memory_id,
		       COALESCE(p.source_turn_index, e.source_turn_index),
		       COALESCE(NULLIF(p.memory_text, ''), e.memory_text),
		       COALESCE(p.emotional_weight, e.emotional_weight),
		       COALESCE(p.importance_10, e.importance_10),
		       COALESCE(NULLIF(p.portability, ''), e.portability),
		       COALESCE(p.tags_json, e.tags_json),
		       COALESCE(NULLIF(p.evidence_excerpt, ''), e.evidence_excerpt),
		       e.injection_policy,
		       e.created_at
		FROM persona_memory_entries e
		LEFT JOIN protagonist_entity_memories p
			ON e.source_memory_type = 'subjective_entity_memory'
			AND e.source_memory_id = p.id
		WHERE e.capsule_id = ?
		ORDER BY COALESCE(p.source_turn_index, e.source_turn_index) ASC, e.id ASC
	`, capsuleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryEntry{}
	for rows.Next() {
		item, err := scanPersonaMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) CreateProtagonistEntityMemory(ctx context.Context, item *ProtagonistEntityMemory) (*ProtagonistEntityMemory, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotEnabled
	}
	now := time.Now().UTC()
	portability := strings.TrimSpace(item.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	ownerEntityKey := strings.TrimSpace(item.OwnerEntityKey)
	if ownerEntityKey == "" {
		ownerEntityKey = strings.TrimSpace(item.PersonaEntityKey)
	}
	personaEntityKey := strings.TrimSpace(item.PersonaEntityKey)
	if personaEntityKey == "" {
		personaEntityKey = ownerEntityKey
	}
	ownerEntityName := strings.TrimSpace(item.OwnerEntityName)
	if ownerEntityName == "" {
		ownerEntityName = strings.TrimSpace(item.PersonaEntityName)
	}
	personaEntityName := strings.TrimSpace(item.PersonaEntityName)
	if personaEntityName == "" {
		personaEntityName = ownerEntityName
	}
	if ownerEntityName == "" {
		ownerEntityName = ownerEntityKey
	}
	if personaEntityName == "" {
		personaEntityName = personaEntityKey
	}
	ownerEntityRole := strings.TrimSpace(item.OwnerEntityRole)
	if ownerEntityRole == "" {
		ownerEntityRole = "protagonist"
	}
	ownerVisibility := strings.TrimSpace(item.OwnerVisibility)
	if ownerVisibility == "" {
		ownerVisibility = "player_known"
	}
	targetRevealPolicy := strings.TrimSpace(item.TargetRevealPolicy)
	if targetRevealPolicy == "" {
		targetRevealPolicy = "requires_explicit_attachment"
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO protagonist_entity_memories
			(persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
			 owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
			 source_turn_index, memory_text, evidence_excerpt, secret_guard, portability, tags_json,
			 target_reveal_policy, importance_10, emotional_weight, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, personaEntityKey, personaEntityName, ownerEntityKey, ownerEntityName, ownerEntityRole, ownerVisibility,
		strings.TrimSpace(item.SourceChatSessionID), strings.TrimSpace(item.SourceCharacterName),
		item.SourceTurn, strings.TrimSpace(item.MemoryText), strings.TrimSpace(item.EvidenceExcerpt),
		item.SecretGuard, portability, strings.TrimSpace(item.TagsJSON), targetRevealPolicy,
		item.Importance10, item.EmotionalWeight, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	out := *item
	out.ID = id
	out.PersonaEntityKey = personaEntityKey
	out.PersonaEntityName = personaEntityName
	out.OwnerEntityKey = ownerEntityKey
	out.OwnerEntityName = ownerEntityName
	out.OwnerEntityRole = ownerEntityRole
	out.OwnerVisibility = ownerVisibility
	out.Portability = portability
	out.TargetRevealPolicy = targetRevealPolicy
	out.CreatedAt = now
	out.UpdatedAt = now
	return &out, nil
}

func (m *mariadbStore) ListProtagonistEntityMemories(ctx context.Context, filter ProtagonistEntityMemoryFilter) ([]ProtagonistEntityMemory, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
		       owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
		       source_turn_index, memory_text, evidence_excerpt, secret_guard, portability, tags_json,
		       target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
		FROM protagonist_entity_memories
		WHERE 1 = 1`
	args := []any{}
	if strings.TrimSpace(filter.OwnerEntityKey) != "" {
		query += " AND (owner_entity_key = ? OR (owner_entity_key = '' AND persona_entity_key = ?))"
		key := strings.TrimSpace(filter.OwnerEntityKey)
		args = append(args, key, key)
	} else if strings.TrimSpace(filter.PersonaEntityKey) != "" {
		query += " AND (persona_entity_key = ? OR owner_entity_key = ?)"
		key := strings.TrimSpace(filter.PersonaEntityKey)
		args = append(args, key, key)
	}
	if strings.TrimSpace(filter.OwnerEntityRole) != "" {
		query += " AND owner_entity_role = ?"
		args = append(args, strings.TrimSpace(filter.OwnerEntityRole))
	}
	if strings.TrimSpace(filter.OwnerVisibility) != "" {
		query += " AND owner_visibility = ?"
		args = append(args, strings.TrimSpace(filter.OwnerVisibility))
	}
	if strings.TrimSpace(filter.SourceChatSessionID) != "" {
		query += " AND source_chat_session_id = ?"
		args = append(args, strings.TrimSpace(filter.SourceChatSessionID))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 80
	}
	if limit > 1000 {
		limit = 1000
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProtagonistEntityMemory{}
	for rows.Next() {
		item, err := scanProtagonistEntityMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) UpdateProtagonistEntityMemoryOwner(ctx context.Context, update ProtagonistEntityMemoryOwnerUpdate) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if update.ID <= 0 {
		return ErrNotFound
	}
	ownerKey := strings.TrimSpace(update.OwnerEntityKey)
	if ownerKey == "" {
		ownerKey = strings.TrimSpace(update.PersonaEntityKey)
	}
	personaKey := strings.TrimSpace(update.PersonaEntityKey)
	if personaKey == "" {
		personaKey = ownerKey
	}
	ownerName := strings.TrimSpace(update.OwnerEntityName)
	if ownerName == "" {
		ownerName = strings.TrimSpace(update.PersonaEntityName)
	}
	personaName := strings.TrimSpace(update.PersonaEntityName)
	if personaName == "" {
		personaName = ownerName
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if personaName == "" {
		personaName = personaKey
	}
	ownerRole := strings.TrimSpace(update.OwnerEntityRole)
	ownerVisibility := strings.TrimSpace(update.OwnerVisibility)
	res, err := m.db.ExecContext(ctx, `
		UPDATE protagonist_entity_memories
		SET persona_entity_key = ?,
		    persona_entity_name = ?,
		    owner_entity_key = ?,
		    owner_entity_name = ?,
		    owner_entity_role = CASE WHEN ? = '' THEN owner_entity_role ELSE ? END,
		    owner_visibility = CASE WHEN ? = '' THEN owner_visibility ELSE ? END,
		    tags_json = ?,
		    updated_at = ?
		WHERE id = ?
	`, personaKey, personaName, ownerKey, ownerName, ownerRole, ownerRole, ownerVisibility, ownerVisibility, strings.TrimSpace(update.TagsJSON), time.Now().UTC(), update.ID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) UpdateProtagonistEntityMemory(ctx context.Context, update ProtagonistEntityMemoryUpdate) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if update.ID <= 0 {
		return ErrNotFound
	}
	ownerKey := strings.TrimSpace(update.OwnerEntityKey)
	if ownerKey == "" {
		ownerKey = strings.TrimSpace(update.PersonaEntityKey)
	}
	personaKey := strings.TrimSpace(update.PersonaEntityKey)
	if personaKey == "" {
		personaKey = ownerKey
	}
	ownerName := strings.TrimSpace(update.OwnerEntityName)
	if ownerName == "" {
		ownerName = strings.TrimSpace(update.PersonaEntityName)
	}
	personaName := strings.TrimSpace(update.PersonaEntityName)
	if personaName == "" {
		personaName = ownerName
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if personaName == "" {
		personaName = personaKey
	}
	ownerRole := strings.TrimSpace(update.OwnerEntityRole)
	if ownerRole == "" {
		ownerRole = "protagonist"
	}
	ownerVisibility := strings.TrimSpace(update.OwnerVisibility)
	if ownerVisibility == "" {
		ownerVisibility = "player_known"
	}
	portability := strings.TrimSpace(update.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	targetRevealPolicy := strings.TrimSpace(update.TargetRevealPolicy)
	if targetRevealPolicy == "" {
		targetRevealPolicy = "requires_explicit_attachment"
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE protagonist_entity_memories
		SET persona_entity_key = ?,
		    persona_entity_name = ?,
		    owner_entity_key = ?,
		    owner_entity_name = ?,
		    owner_entity_role = ?,
		    owner_visibility = ?,
		    source_character_name = ?,
		    memory_text = ?,
		    evidence_excerpt = ?,
		    secret_guard = ?,
		    portability = ?,
		    tags_json = ?,
		    target_reveal_policy = ?,
		    importance_10 = ?,
		    emotional_weight = ?,
		    updated_at = ?
		WHERE id = ?
	`, personaKey, personaName, ownerKey, ownerName, ownerRole, ownerVisibility,
		strings.TrimSpace(update.SourceCharacterName), strings.TrimSpace(update.MemoryText), strings.TrimSpace(update.EvidenceExcerpt),
		update.SecretGuard, portability, strings.TrimSpace(update.TagsJSON), targetRevealPolicy,
		update.Importance10, update.EmotionalWeight, time.Now().UTC(), update.ID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) DeleteProtagonistEntityMemory(ctx context.Context, id int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if id <= 0 {
		return ErrNotFound
	}
	res, err := m.db.ExecContext(ctx, `DELETE FROM protagonist_entity_memories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

type personaMemoryCapsuleScanner interface {
	Scan(dest ...any) error
}

func scanPersonaMemoryCapsule(scanner personaMemoryCapsuleScanner) (PersonaMemoryCapsule, error) {
	var item PersonaMemoryCapsule
	var sourceCharacterName, summary sql.NullString
	if err := scanner.Scan(&item.ID, &item.PersonaKey, &item.SourceChatSessionID, &sourceCharacterName, &item.Title, &item.Mode, &summary, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	item.SourceCharacterName = stringFromNull(sourceCharacterName)
	item.Summary = stringFromNull(summary)
	return item, nil
}

func scanPersonaMemoryEntry(scanner personaMemoryCapsuleScanner) (PersonaMemoryEntry, error) {
	var item PersonaMemoryEntry
	var sourceMemoryType sql.NullString
	var sourceMemoryID, sourceTurn sql.NullInt64
	var emotionalWeight, importance10 sql.NullFloat64
	var tagsJSON, evidenceExcerpt sql.NullString
	if err := scanner.Scan(&item.ID, &item.CapsuleID, &sourceMemoryType, &sourceMemoryID, &sourceTurn, &item.MemoryText, &emotionalWeight, &importance10, &item.Portability, &tagsJSON, &evidenceExcerpt, &item.InjectionPolicy, &item.CreatedAt); err != nil {
		return item, err
	}
	item.SourceMemoryType = stringFromNull(sourceMemoryType)
	item.SourceMemoryID = int64FromNull(sourceMemoryID)
	item.SourceTurn = intFromNull(sourceTurn)
	item.EmotionalWeight = float64FromNull(emotionalWeight)
	item.Importance10 = float64FromNull(importance10)
	item.TagsJSON = stringFromNull(tagsJSON)
	item.EvidenceExcerpt = stringFromNull(evidenceExcerpt)
	return item, nil
}

func scanPersonaCapsuleAttachment(scanner personaMemoryCapsuleScanner) (PersonaCapsuleAttachment, error) {
	var item PersonaCapsuleAttachment
	if err := scanner.Scan(&item.ID, &item.CapsuleID, &item.TargetChatSessionID, &item.InjectionMode, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	return item, nil
}

func scanProtagonistEntityMemory(scanner personaMemoryCapsuleScanner) (ProtagonistEntityMemory, error) {
	var item ProtagonistEntityMemory
	var sourceTurn sql.NullInt64
	var ownerEntityKey, ownerEntityName, ownerEntityRole, ownerVisibility sql.NullString
	var sourceCharacterName, evidenceExcerpt, tagsJSON, targetRevealPolicy sql.NullString
	var importance10, emotionalWeight sql.NullFloat64
	if err := scanner.Scan(
		&item.ID, &item.PersonaEntityKey, &item.PersonaEntityName, &ownerEntityKey, &ownerEntityName,
		&ownerEntityRole, &ownerVisibility, &item.SourceChatSessionID, &sourceCharacterName,
		&sourceTurn, &item.MemoryText, &evidenceExcerpt, &item.SecretGuard, &item.Portability,
		&tagsJSON, &targetRevealPolicy, &importance10, &emotionalWeight, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return item, err
	}
	item.OwnerEntityKey = stringFromNull(ownerEntityKey)
	if item.OwnerEntityKey == "" {
		item.OwnerEntityKey = item.PersonaEntityKey
	}
	item.OwnerEntityName = stringFromNull(ownerEntityName)
	if item.OwnerEntityName == "" {
		item.OwnerEntityName = item.PersonaEntityName
	}
	item.OwnerEntityRole = stringFromNull(ownerEntityRole)
	if item.OwnerEntityRole == "" {
		item.OwnerEntityRole = "protagonist"
	}
	item.OwnerVisibility = stringFromNull(ownerVisibility)
	if item.OwnerVisibility == "" {
		item.OwnerVisibility = "player_known"
	}
	item.SourceCharacterName = stringFromNull(sourceCharacterName)
	item.SourceTurn = intFromNull(sourceTurn)
	item.EvidenceExcerpt = stringFromNull(evidenceExcerpt)
	item.TagsJSON = stringFromNull(tagsJSON)
	item.TargetRevealPolicy = stringFromNull(targetRevealPolicy)
	if item.TargetRevealPolicy == "" {
		item.TargetRevealPolicy = "requires_explicit_attachment"
	}
	item.Importance10 = float64FromNull(importance10)
	item.EmotionalWeight = float64FromNull(emotionalWeight)
	return item, nil
}
