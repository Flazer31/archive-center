package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) SaveStoryline(ctx context.Context, s *Storyline) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	updatedAt := nonZeroTime(s.UpdatedAt)
	res, err := m.db.ExecContext(ctx, `
		UPDATE storylines
		SET status = ?, entities_json = ?, current_context = ?,
			key_points_json = ?, ongoing_tensions_json = ?, confidence = ?,
			evidence_count = ?, last_evidence_turn = ?,
			first_turn = CASE WHEN first_turn IS NULL OR first_turn = 0 THEN ? ELSE first_turn END,
			last_turn = ?, pinned = ?, suppressed = ?, user_corrected = ?,
			updated_at = ?
		WHERE chat_session_id = ? AND name = ?
	`, firstNonEmptyString(s.Status, "active"), nullableString(s.EntitiesJSON),
		nullableString(s.CurrentContext), nullableString(s.KeyPointsJSON), nullableString(s.OngoingTensionsJSON),
		s.Confidence, s.EvidenceCount, s.LastEvidenceTurn, s.FirstTurn, s.LastTurn,
		s.Pinned, s.Suppressed, s.UserCorrected, updatedAt, s.ChatSessionID, s.Name)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return nil
	}
	exists, err := m.storylineExists(ctx, s.ChatSessionID, s.Name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO storylines (
			chat_session_id, name, status, entities_json, current_context,
			key_points_json, ongoing_tensions_json, confidence, evidence_count,
			last_evidence_turn, first_turn, last_turn, pinned, suppressed,
			user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ChatSessionID, s.Name, firstNonEmptyString(s.Status, "active"), nullableString(s.EntitiesJSON),
		nullableString(s.CurrentContext), nullableString(s.KeyPointsJSON), nullableString(s.OngoingTensionsJSON),
		s.Confidence, s.EvidenceCount, s.LastEvidenceTurn, s.FirstTurn, s.LastTurn,
		s.Pinned, s.Suppressed, s.UserCorrected, nonZeroTime(s.CreatedAt), nonZeroTime(s.UpdatedAt))
	return err
}

func (m *mariadbStore) PatchStoryline(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	return m.patchStorylineFields(ctx, storylineID, updates, []string{
		"name", "status", "entities_json", "current_context", "key_points_json",
		"ongoing_tensions_json", "confidence", "evidence_count", "last_evidence_turn",
		"first_turn", "last_turn",
	})
}

func (m *mariadbStore) PatchStorylineTrust(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	return m.patchStorylineFields(ctx, storylineID, updates, []string{"pinned", "suppressed", "user_corrected"})
}

func (m *mariadbStore) patchStorylineFields(ctx context.Context, storylineID int64, updates map[string]any, fieldOrder []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	updatedFields := make([]string, 0, len(updates))
	setParts := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for _, field := range fieldOrder {
		val, ok := updates[field]
		if !ok {
			continue
		}
		setParts = append(setParts, field+" = ?")
		args = append(args, val)
		updatedFields = append(updatedFields, field)
	}
	if len(setParts) == 0 {
		exists, err := m.storylineIDExists(ctx, storylineID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return updatedFields, nil
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, storylineID)
	res, err := m.db.ExecContext(ctx, "UPDATE storylines SET "+strings.Join(setParts, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return updatedFields, nil
	}
	exists, err := m.storylineIDExists(ctx, storylineID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return updatedFields, nil
}

func (m *mariadbStore) DeleteStoryline(ctx context.Context, storylineID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM storylines WHERE id = ?", storylineID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) storylineExists(ctx context.Context, chatSessionID, name string) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storylines WHERE chat_session_id = ? AND name = ?", chatSessionID, name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) storylineIDExists(ctx context.Context, storylineID int64) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storylines WHERE id = ?", storylineID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, name, status, entities_json, current_context, key_points_json,
			   ongoing_tensions_json, confidence, evidence_count, last_evidence_turn, first_turn, last_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM storylines
		WHERE chat_session_id = ?
		ORDER BY last_turn DESC, id DESC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Storyline
	for rows.Next() {
		var item Storyline
		var entitiesJSON, currentContext, keyPointsJSON, ongoingTensionsJSON sql.NullString
		var confidence sql.NullFloat64
		var evidenceCount, lastEvidenceTurn, firstTurn, lastTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Name, &item.Status,
			&entitiesJSON, &currentContext, &keyPointsJSON, &ongoingTensionsJSON,
			&confidence, &evidenceCount, &lastEvidenceTurn, &firstTurn, &lastTurn,
			&item.Pinned, &item.Suppressed, &item.UserCorrected, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.EntitiesJSON = stringFromNull(entitiesJSON)
		item.CurrentContext = stringFromNull(currentContext)
		item.KeyPointsJSON = stringFromNull(keyPointsJSON)
		item.OngoingTensionsJSON = stringFromNull(ongoingTensionsJSON)
		item.Confidence = float64FromNull(confidence)
		item.EvidenceCount = intFromNull(evidenceCount)
		item.LastEvidenceTurn = intFromNull(lastEvidenceTurn)
		item.FirstTurn = intFromNull(firstTurn)
		item.LastTurn = intFromNull(lastTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveWorldRule(ctx context.Context, w *WorldRule) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	scope := firstNonEmptyString(w.Scope, "root")
	scopeName := nullableString(w.ScopeName)
	category := firstNonEmptyString(w.Category, "custom")
	updatedAt := nonZeroTime(w.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE world_rules
		SET scope_name = ?,
			category = ?,
			value_json = COALESCE(?, value_json),
			genre = COALESCE(?, genre),
			source_turn = CASE WHEN ? > 0 THEN ? ELSE source_turn END,
			pinned = ?,
			suppressed = ?,
			user_corrected = ?,
			updated_at = ?
		WHERE chat_session_id = ? AND scope = ? AND `+"`key`"+` = ? AND scope_name <=> ?
		ORDER BY id DESC
		LIMIT 1
	`, scopeName, category, nullableString(w.ValueJSON), nullableString(w.Genre),
		w.SourceTurn, w.SourceTurn, w.Pinned, w.Suppressed, w.UserCorrected, updatedAt,
		w.ChatSessionID, scope, w.Key, scopeName)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		_ = m.db.QueryRowContext(ctx, `
			SELECT id
			FROM world_rules
			WHERE chat_session_id = ? AND scope = ? AND `+"`key`"+` = ? AND scope_name <=> ?
			ORDER BY id DESC
			LIMIT 1
		`, w.ChatSessionID, scope, w.Key, scopeName).Scan(&w.ID)
		return nil
	}
	res, err = m.db.ExecContext(ctx, `
		INSERT INTO world_rules (
			chat_session_id, scope, scope_name, category, `+"`key`"+`, value_json,
			genre, source_turn, pinned, suppressed, user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, w.ChatSessionID, scope, scopeName,
		category, w.Key, nullableString(w.ValueJSON),
		nullableString(w.Genre), w.SourceTurn, w.Pinned, w.Suppressed, w.UserCorrected,
		nonZeroTime(w.CreatedAt), updatedAt)
	if err == nil {
		if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
			w.ID = id
		}
	}
	return err
}

func (m *mariadbStore) PatchWorldRule(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	return m.patchWorldRuleFields(ctx, ruleID, updates, []string{"scope", "scope_name", "category", "key", "value_json", "genre", "source_turn"})
}

func (m *mariadbStore) PatchWorldRuleTrust(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	return m.patchWorldRuleFields(ctx, ruleID, updates, []string{"pinned", "suppressed", "user_corrected"})
}

func (m *mariadbStore) patchWorldRuleFields(ctx context.Context, ruleID int64, updates map[string]any, fieldOrder []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	updatedFields := make([]string, 0, len(updates))
	setParts := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for _, field := range fieldOrder {
		val, ok := updates[field]
		if !ok {
			continue
		}
		column := field
		if field == "key" {
			column = "`key`"
		}
		setParts = append(setParts, column+" = ?")
		args = append(args, val)
		updatedFields = append(updatedFields, field)
	}
	if len(setParts) == 0 {
		exists, err := m.worldRuleIDExists(ctx, ruleID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return updatedFields, nil
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, ruleID)
	res, err := m.db.ExecContext(ctx, "UPDATE world_rules SET "+strings.Join(setParts, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return updatedFields, nil
	}
	exists, err := m.worldRuleIDExists(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return updatedFields, nil
}

func (m *mariadbStore) DeleteWorldRule(ctx context.Context, ruleID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM world_rules WHERE id = ?", ruleID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) worldRuleIDExists(ctx context.Context, ruleID int64) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM world_rules WHERE id = ?", ruleID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, scope, scope_name, category, `+"`key`"+`, value_json, genre, source_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM world_rules
		WHERE chat_session_id = ?
		ORDER BY scope, category, `+"`key`"+`
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorldRule
	for rows.Next() {
		var item WorldRule
		var scopeName, genre sql.NullString
		var valueJSON sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Scope, &scopeName, &item.Category, &item.Key,
			&valueJSON, &genre, &sourceTurn, &item.Pinned, &item.Suppressed, &item.UserCorrected,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScopeName = stringFromNull(scopeName)
		item.ValueJSON = stringFromNull(valueJSON)
		item.Genre = stringFromNull(genre)
		item.SourceTurn = intFromNull(sourceTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	activeScope = strings.TrimSpace(activeScope)
	scopeName = strings.TrimSpace(scopeName)
	if activeScope == "" {
		if saved, err := m.GetActiveScope(ctx, chatSessionID); err == nil && saved != nil {
			activeScope = strings.TrimSpace(saved.ActiveScope)
			if scopeName == "" {
				scopeName = strings.TrimSpace(saved.ScopeName)
			}
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	chain := WorldRuleScopeChain(activeScope)
	chainOrder := make(map[string]int, len(chain))
	for i, scope := range chain {
		chainOrder[scope] = i
	}

	query := `
		SELECT id, chat_session_id, scope, scope_name, category, ` + "`key`" + `, value_json, genre, source_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM world_rules
		WHERE chat_session_id = ? AND suppressed = FALSE
		ORDER BY scope, category, ` + "`key`" + `
	`
	rows, err := m.db.QueryContext(ctx, query, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorldRule
	for rows.Next() {
		var item WorldRule
		var scopeNameNull, genre sql.NullString
		var valueJSON sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Scope, &scopeNameNull, &item.Category, &item.Key,
			&valueJSON, &genre, &sourceTurn, &item.Pinned, &item.Suppressed, &item.UserCorrected,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScopeName = stringFromNull(scopeNameNull)
		item.ValueJSON = stringFromNull(valueJSON)
		item.Genre = stringFromNull(genre)
		item.SourceTurn = intFromNull(sourceTurn)
		itemScope := NormalizeWorldRuleScope(item.Scope)
		if _, ok := chainOrder[itemScope]; !ok {
			continue
		}
		if itemScope == NormalizeWorldRuleScope(activeScope) {
			if scopeName != "" && strings.TrimSpace(item.ScopeName) != scopeName {
				continue
			}
			if scopeName == "" && strings.TrimSpace(item.ScopeName) != "" {
				continue
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := chainOrder[NormalizeWorldRuleScope(out[i].Scope)], chainOrder[NormalizeWorldRuleScope(out[j].Scope)]
		if left != right {
			return left < right
		}
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
