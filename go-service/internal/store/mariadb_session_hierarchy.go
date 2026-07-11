package store

import (
	"context"
	"database/sql"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT listed_sessions.chat_session_id,
		       listed_sessions.chat_logs_count,
		       listed_sessions.memories_count,
		       listed_sessions.kg_triples_count,
		       listed_sessions.last_activity
		FROM (
		SELECT sid.chat_session_id,
		       COALESCE(cl.chat_logs_count, 0) AS chat_logs_count,
		       COALESCE(mem.memories_count, 0) AS memories_count,
		       COALESCE(kg.kg_triples_count, 0) AS kg_triples_count,
		       CAST(NULLIF(GREATEST(
			       COALESCE(cl.last_activity, '1000-01-01 00:00:00'),
			       COALESCE(mem.last_activity, '1000-01-01 00:00:00'),
			       COALESCE(kg.last_activity, '1000-01-01 00:00:00')
		       ), '1000-01-01 00:00:00') AS DATETIME) AS last_activity
		FROM (
			SELECT chat_session_id FROM chat_logs
			UNION
			SELECT chat_session_id FROM memories
			UNION
			SELECT chat_session_id FROM kg_triples
		) sid
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS chat_logs_count, MAX(created_at) AS last_activity
			FROM chat_logs
			GROUP BY chat_session_id
		) cl ON cl.chat_session_id = sid.chat_session_id
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS memories_count, MAX(created_at) AS last_activity
			FROM memories
			GROUP BY chat_session_id
		) mem ON mem.chat_session_id = sid.chat_session_id
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS kg_triples_count, MAX(created_at) AS last_activity
			FROM kg_triples
			GROUP BY chat_session_id
		) kg ON kg.chat_session_id = sid.chat_session_id
		) listed_sessions
		ORDER BY listed_sessions.last_activity IS NULL ASC, listed_sessions.last_activity DESC, listed_sessions.chat_session_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionSummary
	for rows.Next() {
		var item SessionSummary
		var lastActivity sql.NullTime
		if err := rows.Scan(&item.ChatSessionID, &item.ChatLogsCount, &item.MemoriesCount, &item.KGTriplesCount, &lastActivity); err != nil {
			return nil, err
		}
		if lastActivity.Valid {
			item.LastActivity = lastActivity.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chapter, err := m.latestChapterSummary(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}
	arc, err := m.latestArcSummary(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}
	saga, err := m.latestSagaDigest(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}

	sources := []string{}
	parts := []string{}
	if saga != nil {
		sources = append(sources, "saga_digests")
		if saga.ResumePackText != "" {
			parts = append(parts, "Saga: "+saga.ResumePackText)
		} else if saga.SagaSummary != "" {
			parts = append(parts, "Saga: "+saga.SagaSummary)
		}
	}
	if arc != nil {
		sources = append(sources, "arc_summaries")
		if arc.ArcResumeText != "" {
			parts = append(parts, "Arc: "+arc.ArcResumeText)
		} else if arc.CoreConflict != "" {
			parts = append(parts, "Arc: "+arc.CoreConflict)
		}
	}
	if chapter != nil {
		sources = append(sources, "chapter_summaries")
		if chapter.ChapterTitle != "" {
			parts = append(parts, "Chapter: "+chapter.ChapterTitle)
		}
		if chapter.ResumeText != "" {
			parts = append(parts, "Resume: "+chapter.ResumeText)
		}
		if chapter.SummaryText != "" {
			parts = append(parts, "Summary: "+chapter.SummaryText)
		}
	}
	if len(sources) == 0 {
		return &ResumePack{
			PackStatus:    "empty",
			Trigger:       trigger,
			SourcesUsed:   []string{},
			LayerCount:    0,
			AssembledText: "",
			AssemblyNote:  "no hierarchy rows found",
		}, nil
	}
	return &ResumePack{
		PackStatus:    "ready",
		Trigger:       trigger,
		SourcesUsed:   sources,
		LayerCount:    len(sources),
		AssembledText: strings.Join(parts, "\n"),
		Saga:          saga,
		Arc:           arc,
		Chapter:       chapter,
		AssemblyNote:  "assembled from latest saga/arc/chapter rows",
	}, nil
}

func (m *mariadbStore) latestChapterSummary(ctx context.Context, chatSessionID string) (*ChapterSummary, error) {
	var item ChapterSummary
	var chapterTitle, summaryText, resumeText sql.NullString
	var chapterIndex sql.NullInt64
	var openLoopsJSON, relationshipChangesJSON, worldChangesJSON, callbackCandidatesJSON sql.NullString
	var embeddingVector, embeddingModel sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
		       open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
		       resume_text, embedding_vector, embedding_model, created_at
		FROM chapter_summaries
		WHERE chat_session_id = ?
		ORDER BY chapter_index DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &chapterIndex, &chapterTitle, &summaryText,
		&openLoopsJSON, &relationshipChangesJSON, &worldChangesJSON, &callbackCandidatesJSON,
		&resumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.ChapterIndex = intFromNull(chapterIndex)
	item.ChapterTitle = stringFromNull(chapterTitle)
	item.SummaryText = stringFromNull(summaryText)
	item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
	item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
	item.WorldChangesJSON = stringFromNull(worldChangesJSON)
	item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
	item.ResumeText = stringFromNull(resumeText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) latestArcSummary(ctx context.Context, chatSessionID string) (*ArcSummary, error) {
	var item ArcSummary
	var arcName, arcStatus, coreConflict, arcResumeText sql.NullString
	var keyTurningPointsJSON, activePromisesJSON, unresolvedDebtsJSON, resolvedPayoffsJSON sql.NullString
	var callbackCandidatesJSON, futurePayoffCandidatesJSON, irreversibleTurnsJSON, callbackDebtsJSON sql.NullString
	var relationshipPivotsJSON, embeddingVector, embeddingModel sql.NullString
	var arcIndex sql.NullInt64
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
		       key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
		       callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
		       relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model, created_at
		FROM arc_summaries
		WHERE chat_session_id = ?
		ORDER BY arc_index DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &arcIndex, &arcName, &arcStatus, &coreConflict,
		&keyTurningPointsJSON, &activePromisesJSON, &unresolvedDebtsJSON, &resolvedPayoffsJSON,
		&callbackCandidatesJSON, &futurePayoffCandidatesJSON, &irreversibleTurnsJSON, &callbackDebtsJSON,
		&relationshipPivotsJSON, &arcResumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.ArcIndex = intFromNull(arcIndex)
	item.ArcName = stringFromNull(arcName)
	item.ArcStatus = stringFromNull(arcStatus)
	item.CoreConflict = stringFromNull(coreConflict)
	item.KeyTurningPointsJSON = stringFromNull(keyTurningPointsJSON)
	item.ActivePromisesJSON = stringFromNull(activePromisesJSON)
	item.UnresolvedDebtsJSON = stringFromNull(unresolvedDebtsJSON)
	item.ResolvedPayoffsJSON = stringFromNull(resolvedPayoffsJSON)
	item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
	item.FuturePayoffCandidatesJSON = stringFromNull(futurePayoffCandidatesJSON)
	item.IrreversibleTurnsJSON = stringFromNull(irreversibleTurnsJSON)
	item.CallbackDebtsJSON = stringFromNull(callbackDebtsJSON)
	item.RelationshipPivotsJSON = stringFromNull(relationshipPivotsJSON)
	item.ArcResumeText = stringFromNull(arcResumeText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) latestSagaDigest(ctx context.Context, chatSessionID string) (*SagaDigest, error) {
	var item SagaDigest
	var eraLabel, sagaSummary, persistentFactsJSON, neverDropCandidatesJSON, resumePackText sql.NullString
	var embeddingVector, embeddingModel sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,
		       persistent_facts_json, never_drop_candidates_json, resume_pack_text,
		       embedding_vector, embedding_model, created_at
		FROM saga_digests
		WHERE chat_session_id = ?
		ORDER BY to_turn DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &eraLabel, &sagaSummary,
		&persistentFactsJSON, &neverDropCandidatesJSON, &resumePackText,
		&embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.EraLabel = stringFromNull(eraLabel)
	item.SagaSummary = stringFromNull(sagaSummary)
	item.PersistentFactsJSON = stringFromNull(persistentFactsJSON)
	item.NeverDropCandidatesJSON = stringFromNull(neverDropCandidatesJSON)
	item.ResumePackText = stringFromNull(resumePackText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) SaveChapterSummary(ctx context.Context, item *ChapterSummary) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO chapter_summaries (
			chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
			open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
			resume_text, embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, item.ChapterIndex, nullableString(item.ChapterTitle), item.SummaryText,
		nullableString(item.OpenLoopsJSON), nullableString(item.RelationshipChangesJSON), nullableString(item.WorldChangesJSON),
		nullableString(item.CallbackCandidatesJSON), nullableString(item.ResumeText), nullableString(item.EmbeddingVector),
		nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) SearchChapterSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ChapterSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
		       open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
		       resume_text, embedding_vector, embedding_model, created_at
		FROM chapter_summaries
		WHERE chat_session_id = ?
		  AND (? = '' OR chapter_title LIKE ? OR summary_text LIKE ? OR resume_text LIKE ?
		       OR open_loops_json LIKE ? OR relationship_changes_json LIKE ? OR world_changes_json LIKE ?
		       OR callback_candidates_json LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY chapter_index DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, like, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ChapterSummary{}
	for rows.Next() {
		var item ChapterSummary
		var chapterTitle, summaryText, resumeText sql.NullString
		var chapterIndex sql.NullInt64
		var openLoopsJSON, relationshipChangesJSON, worldChangesJSON, callbackCandidatesJSON sql.NullString
		var embeddingVector, embeddingModel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &chapterIndex, &chapterTitle, &summaryText,
			&openLoopsJSON, &relationshipChangesJSON, &worldChangesJSON, &callbackCandidatesJSON,
			&resumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ChapterIndex = intFromNull(chapterIndex)
		item.ChapterTitle = stringFromNull(chapterTitle)
		item.SummaryText = stringFromNull(summaryText)
		item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
		item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
		item.WorldChangesJSON = stringFromNull(worldChangesJSON)
		item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
		item.ResumeText = stringFromNull(resumeText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *mariadbStore) SaveArcSummary(ctx context.Context, chatSessionID string, item *ArcSummary) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO arc_summaries (
			chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
			key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
			callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
			relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, item.ArcIndex, nullableString(item.ArcName),
		firstNonEmptyString(item.ArcStatus, "active"), nullableString(item.CoreConflict),
		nullableString(item.KeyTurningPointsJSON), nullableString(item.ActivePromisesJSON), nullableString(item.UnresolvedDebtsJSON),
		nullableString(item.ResolvedPayoffsJSON), nullableString(item.CallbackCandidatesJSON), nullableString(item.FuturePayoffCandidatesJSON),
		nullableString(item.IrreversibleTurnsJSON), nullableString(item.CallbackDebtsJSON), nullableString(item.RelationshipPivotsJSON),
		nullableString(item.ArcResumeText), nullableString(item.EmbeddingVector), nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) GetLatestArcSummary(ctx context.Context, chatSessionID string) (*ArcSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return m.latestArcSummary(ctx, chatSessionID)
}

func (m *mariadbStore) ListArcSummaries(ctx context.Context, chatSessionID string, status string, limit int) ([]ArcSummary, error) {
	items, err := m.SearchArcSummaries(ctx, chatSessionID, "", 0, 0, limit)
	if err != nil || strings.TrimSpace(status) == "" {
		return items, err
	}
	filtered := make([]ArcSummary, 0, len(items))
	for _, item := range items {
		if item.ArcStatus == status {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (m *mariadbStore) SearchArcSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ArcSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
		       key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
		       callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
		       relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model, created_at
		FROM arc_summaries
		WHERE chat_session_id = ?
		  AND (? = '' OR arc_name LIKE ? OR core_conflict LIKE ? OR arc_resume_text LIKE ?
		       OR key_turning_points_json LIKE ? OR active_promises_json LIKE ? OR unresolved_debts_json LIKE ?
		       OR callback_candidates_json LIKE ? OR future_payoff_candidates_json LIKE ?
		       OR irreversible_turns_json LIKE ? OR callback_debts_json LIKE ? OR relationship_pivots_json LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY arc_index DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, like, like, like, like, like, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ArcSummary{}
	for rows.Next() {
		var item ArcSummary
		var arcName, arcStatus, coreConflict, arcResumeText sql.NullString
		var keyTurningPointsJSON, activePromisesJSON, unresolvedDebtsJSON, resolvedPayoffsJSON sql.NullString
		var callbackCandidatesJSON, futurePayoffCandidatesJSON, irreversibleTurnsJSON, callbackDebtsJSON sql.NullString
		var relationshipPivotsJSON, embeddingVector, embeddingModel sql.NullString
		var arcIndex sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &arcIndex, &arcName, &arcStatus, &coreConflict,
			&keyTurningPointsJSON, &activePromisesJSON, &unresolvedDebtsJSON, &resolvedPayoffsJSON,
			&callbackCandidatesJSON, &futurePayoffCandidatesJSON, &irreversibleTurnsJSON, &callbackDebtsJSON,
			&relationshipPivotsJSON, &arcResumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ArcIndex = intFromNull(arcIndex)
		item.ArcName = stringFromNull(arcName)
		item.ArcStatus = stringFromNull(arcStatus)
		item.CoreConflict = stringFromNull(coreConflict)
		item.KeyTurningPointsJSON = stringFromNull(keyTurningPointsJSON)
		item.ActivePromisesJSON = stringFromNull(activePromisesJSON)
		item.UnresolvedDebtsJSON = stringFromNull(unresolvedDebtsJSON)
		item.ResolvedPayoffsJSON = stringFromNull(resolvedPayoffsJSON)
		item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
		item.FuturePayoffCandidatesJSON = stringFromNull(futurePayoffCandidatesJSON)
		item.IrreversibleTurnsJSON = stringFromNull(irreversibleTurnsJSON)
		item.CallbackDebtsJSON = stringFromNull(callbackDebtsJSON)
		item.RelationshipPivotsJSON = stringFromNull(relationshipPivotsJSON)
		item.ArcResumeText = stringFromNull(arcResumeText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *mariadbStore) SaveSagaDigest(ctx context.Context, chatSessionID string, item *SagaDigest) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO saga_digests (
			chat_session_id, from_turn, to_turn, era_label, saga_summary,
			persistent_facts_json, never_drop_candidates_json, resume_pack_text,
			embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, nullableString(item.EraLabel), item.SagaSummary,
		nullableString(item.PersistentFactsJSON), nullableString(item.NeverDropCandidatesJSON), nullableString(item.ResumePackText),
		nullableString(item.EmbeddingVector), nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) GetLatestSagaDigest(ctx context.Context, chatSessionID string) (*SagaDigest, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return m.latestSagaDigest(ctx, chatSessionID)
}

func (m *mariadbStore) ListSagaDigests(ctx context.Context, chatSessionID string, limit int) ([]SagaDigest, error) {
	return m.SearchSagaDigests(ctx, chatSessionID, "", 0, 0, limit)
}

func (m *mariadbStore) SearchSagaDigests(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]SagaDigest, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,
		       persistent_facts_json, never_drop_candidates_json, resume_pack_text,
		       embedding_vector, embedding_model, created_at
		FROM saga_digests
		WHERE chat_session_id = ?
		  AND (? = '' OR era_label LIKE ? OR saga_summary LIKE ? OR resume_pack_text LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY to_turn DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []SagaDigest{}
	for rows.Next() {
		var item SagaDigest
		var eraLabel, sagaSummary, persistentFactsJSON, neverDropCandidatesJSON, resumePackText sql.NullString
		var embeddingVector, embeddingModel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &eraLabel, &sagaSummary,
			&persistentFactsJSON, &neverDropCandidatesJSON, &resumePackText,
			&embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.EraLabel = stringFromNull(eraLabel)
		item.SagaSummary = stringFromNull(sagaSummary)
		item.PersistentFactsJSON = stringFromNull(persistentFactsJSON)
		item.NeverDropCandidatesJSON = stringFromNull(neverDropCandidatesJSON)
		item.ResumePackText = stringFromNull(resumePackText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ---------------------------------------------------------------------------
// Narrative / State read methods (R1)
// ---------------------------------------------------------------------------
