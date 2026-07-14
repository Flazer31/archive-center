package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type sessionMigrationPreviewStore struct {
	store.Store
	chatLogs           []store.ChatLog
	effectiveInputs    []store.EffectiveInput
	memories           []store.Memory
	evidence           []store.DirectEvidence
	triples            []store.KGTriple
	episodes           []store.EpisodeSummary
	subjective         []store.ProtagonistEntityMemory
	completeCalled     bool
	completeResult     *store.SessionMigrationCompleteResult
	completeRequest    store.SessionMigrationCompleteRequest
	vectorDocs         []store.SessionMigrationVectorDocument
	vectorStatusCalled bool
	vectorStatus       string
	vectorStatusCount  int
	vectorStatusErrors string
	lockCalled         bool
	lockReason         string
	lockResult         *store.SessionMigrationSourceLockResult
	activeLock         *store.SessionMigrationLock
	rollbackCalled     bool
	rollbackReason     string
	rollbackResult     *store.SessionMigrationRollbackResult
	cleanupPreview     *store.SessionMigrationCleanupPreview
	cleanupCalled      bool
	cleanupReason      string
	cleanupResult      *store.SessionMigrationCleanupResult
}

func (s *sessionMigrationPreviewStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	out := []store.ChatLog{}
	for _, item := range s.chatLogs {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.Memory, error) {
	out := []store.Memory{}
	for _, item := range s.memories {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.EffectiveInput, error) {
	out := []store.EffectiveInput{}
	for _, item := range s.effectiveInputs {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListEvidence(ctx context.Context, chatSessionID string) ([]store.DirectEvidence, error) {
	out := []store.DirectEvidence{}
	for _, item := range s.evidence {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]store.KGTriple, error) {
	out := []store.KGTriple{}
	for _, item := range s.triples {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	out := []store.EpisodeSummary{}
	for _, item := range s.episodes {
		if item.ChatSessionID == chatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) ListProtagonistEntityMemories(ctx context.Context, filter store.ProtagonistEntityMemoryFilter) ([]store.ProtagonistEntityMemory, error) {
	out := []store.ProtagonistEntityMemory{}
	for _, item := range s.subjective {
		if item.SourceChatSessionID == filter.SourceChatSessionID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) CreateProtagonistEntityMemory(ctx context.Context, item *store.ProtagonistEntityMemory) (*store.ProtagonistEntityMemory, error) {
	panic("CreateProtagonistEntityMemory must not be called by migrate-preview")
}

func (s *sessionMigrationPreviewStore) CompleteSessionMigration(ctx context.Context, req store.SessionMigrationCompleteRequest) (*store.SessionMigrationCompleteResult, error) {
	s.completeCalled = true
	s.completeRequest = req
	if s.completeResult != nil {
		return s.completeResult, nil
	}
	return &store.SessionMigrationCompleteResult{
		MigrationID:           77,
		Status:                "copied",
		SourceSessionID:       req.SourceSessionID,
		TargetSessionID:       req.TargetSessionID,
		Mode:                  req.Mode,
		Counts:                store.SessionMigrationArtifactCounts{ChatLogs: 1, Memories: 1, CanonicalTotal: 2, CanonicalAndSubjectiveTotal: 2},
		RowMapCount:           2,
		ChromaReindexRequired: true,
		ReadyForLive:          false,
	}, nil
}

func (s *sessionMigrationPreviewStore) ListSessionMigrationVectorDocuments(ctx context.Context, migrationID int64) ([]store.SessionMigrationVectorDocument, error) {
	out := []store.SessionMigrationVectorDocument{}
	for _, item := range s.vectorDocs {
		if item.MigrationID == migrationID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *sessionMigrationPreviewStore) UpdateSessionMigrationVectorStatus(ctx context.Context, migrationID int64, status string, reindexedCount int, errorsJSON string) error {
	s.vectorStatusCalled = true
	s.vectorStatus = status
	s.vectorStatusCount = reindexedCount
	s.vectorStatusErrors = errorsJSON
	return nil
}

func (s *sessionMigrationPreviewStore) LockSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*store.SessionMigrationSourceLockResult, error) {
	s.lockCalled = true
	s.lockReason = reason
	if s.lockResult != nil {
		return s.lockResult, nil
	}
	lock := store.SessionMigrationLock{
		MigrationID:     migrationID,
		SourceSessionID: "char_59_cid_source",
		TargetSessionID: "char_59_cid_target",
		Locked:          true,
		LockStatus:      "migrated_away",
		Reason:          reason,
		LockedAt:        time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
	}
	return &store.SessionMigrationSourceLockResult{
		MigrationID:     migrationID,
		SourceSessionID: lock.SourceSessionID,
		TargetSessionID: lock.TargetSessionID,
		Status:          "source_locked",
		Lock:            lock,
		ReadyForLive:    true,
	}, nil
}

func (s *sessionMigrationPreviewStore) GetSessionMigrationSourceLock(ctx context.Context, sourceSessionID string) (*store.SessionMigrationLock, error) {
	if s.activeLock != nil && s.activeLock.SourceSessionID == sourceSessionID && s.activeLock.Locked {
		return s.activeLock, nil
	}
	return nil, store.ErrNotFound
}

func (s *sessionMigrationPreviewStore) RollbackSessionMigration(ctx context.Context, migrationID int64, reason string) (*store.SessionMigrationRollbackResult, error) {
	s.rollbackCalled = true
	s.rollbackReason = reason
	if s.rollbackResult != nil {
		return s.rollbackResult, nil
	}
	return &store.SessionMigrationRollbackResult{
		MigrationID:     migrationID,
		SourceSessionID: "char_59_cid_source",
		TargetSessionID: "char_59_cid_target",
		Status:          "rolled_back",
		Counts:          store.SessionMigrationArtifactCounts{ChatLogs: 2, Memories: 1, Episodes: 1, CanonicalTotal: 4, CanonicalAndSubjectiveTotal: 4},
		RowMapCount:     4,
		SourceUnlocked:  true,
		ReadyForLive:    false,
	}, nil
}

func (s *sessionMigrationPreviewStore) PreviewSessionMigrationSourceCleanup(ctx context.Context, migrationID int64) (*store.SessionMigrationCleanupPreview, error) {
	if s.cleanupPreview != nil {
		return s.cleanupPreview, nil
	}
	return &store.SessionMigrationCleanupPreview{
		MigrationID:     migrationID,
		SourceSessionID: "char_59_cid_source",
		TargetSessionID: "char_59_cid_target",
		Status:          "source_locked",
		SourceLocked:    true,
		Counts:          store.SessionMigrationArtifactCounts{ChatLogs: 2, Memories: 1, CanonicalTotal: 3, CanonicalAndSubjectiveTotal: 3},
		ReadyForCleanup: true,
	}, nil
}

func (s *sessionMigrationPreviewStore) CleanupSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*store.SessionMigrationCleanupResult, error) {
	s.cleanupCalled = true
	s.cleanupReason = reason
	if s.cleanupResult != nil {
		return s.cleanupResult, nil
	}
	return &store.SessionMigrationCleanupResult{
		MigrationID:     migrationID,
		SourceSessionID: "char_59_cid_source",
		TargetSessionID: "char_59_cid_target",
		Status:          "source_cleaned",
		Counts:          store.SessionMigrationArtifactCounts{ChatLogs: 2, Memories: 1, CanonicalTotal: 3, CanonicalAndSubjectiveTotal: 3},
		SourceCleaned:   true,
		ReadyForLive:    true,
	}, nil
}

type sessionMigrationPreviewVector struct {
	counts            map[string]int
	err               error
	upsertCalled      bool
	upsertSessionID   string
	upsertDocs        []vector.VectorDocument
	deleteCalled      bool
	deleteSessionID   string
	deleteDocumentIDs []string
	rebuildCalled     bool
}

func (v *sessionMigrationPreviewVector) Search(ctx context.Context, sessionID string, embedding []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotFound
}

func (v *sessionMigrationPreviewVector) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	v.upsertCalled = true
	v.upsertSessionID = sessionID
	v.upsertDocs = append(v.upsertDocs, docs...)
	if v.counts == nil {
		v.counts = map[string]int{}
	}
	v.counts[sessionID] += len(docs)
	return nil
}

func (v *sessionMigrationPreviewVector) DeleteSession(ctx context.Context, sessionID string) error {
	v.deleteCalled = true
	v.deleteSessionID = sessionID
	return nil
}

func (v *sessionMigrationPreviewVector) DeleteDocuments(ctx context.Context, ids []string) error {
	v.deleteDocumentIDs = append(v.deleteDocumentIDs, ids...)
	return nil
}

func (v *sessionMigrationPreviewVector) Rebuild(ctx context.Context, sessionID string) error {
	v.rebuildCalled = true
	return nil
}

func (v *sessionMigrationPreviewVector) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok"}, nil
}

func (v *sessionMigrationPreviewVector) Count(ctx context.Context, sessionID string) (int, error) {
	if v.err != nil {
		return 0, v.err
	}
	return v.counts[sessionID], nil
}

func (v *sessionMigrationPreviewVector) Close(ctx context.Context) error { return nil }

func TestSessionMigratePreviewDryRunBlocksNonEmptyTarget(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_target"
	st := &sessionMigrationPreviewStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: sourceID},
			{ID: 2, ChatSessionID: sourceID},
			{ID: 3, ChatSessionID: targetID},
		},
		effectiveInputs: []store.EffectiveInput{
			{ID: 4, ChatSessionID: sourceID},
		},
		memories: []store.Memory{
			{ID: 10, ChatSessionID: sourceID},
		},
		evidence: []store.DirectEvidence{
			{ID: 20, ChatSessionID: sourceID},
		},
		triples: []store.KGTriple{
			{ID: 30, ChatSessionID: sourceID},
		},
		episodes: []store.EpisodeSummary{
			{ID: 40, ChatSessionID: sourceID},
		},
		subjective: []store.ProtagonistEntityMemory{
			{ID: 50, SourceChatSessionID: sourceID},
		},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sourceID: 6, targetID: 1}}
	resp := performSessionMigrationPreview(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
		"mode":              sessionMigrationModeCopyLock,
	})

	if resp.Status != "ok" || !resp.DryRun || !resp.ReadOnly {
		t.Fatalf("unexpected response status/dry-run: %+v", resp)
	}
	if resp.WriteAttempted || resp.VectorWriteAttempted || resp.LLMCallAttempted {
		t.Fatalf("preview attempted side effects: %+v", resp)
	}
	if vec.upsertCalled || vec.deleteCalled || vec.rebuildCalled {
		t.Fatalf("vector mutation called during preview")
	}
	if !resp.Blocked {
		t.Fatalf("expected non-empty target to block migration")
	}
	if !sessionMigrationContainsString(resp.BlockedReasons, "target_session_not_empty") {
		t.Fatalf("blocked_reasons = %#v, want target_session_not_empty", resp.BlockedReasons)
	}
	if !sessionMigrationContainsString(resp.BlockedReasons, "target_chroma_vectors_not_empty") {
		t.Fatalf("blocked_reasons = %#v, want target_chroma_vectors_not_empty", resp.BlockedReasons)
	}
	if resp.Counts.ChatLogs != 2 || resp.Counts.EffectiveInputs != 1 || resp.Counts.Memories != 1 || resp.Counts.DirectEvidence != 1 ||
		resp.Counts.KGTriples != 1 || resp.Counts.Episodes != 1 || resp.Counts.SubjectiveEntityMemories != 1 ||
		resp.Counts.ChromaVectors != 6 {
		t.Fatalf("source counts = %+v", resp.Counts)
	}
	if resp.TargetCounts.ChatLogs != 1 || resp.TargetCounts.ChromaVectors != 1 {
		t.Fatalf("target counts = %+v", resp.TargetCounts)
	}
}

func TestSessionMigratePreviewAllowsEmptyTargetDryRun(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_fresh"
	st := &sessionMigrationPreviewStore{
		chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: sourceID}},
		memories: []store.Memory{{ID: 2, ChatSessionID: sourceID}},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sourceID: 2}}
	resp := performSessionMigrationPreview(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
	})

	if resp.Blocked {
		t.Fatalf("empty target preview blocked: reasons=%#v warnings=%#v", resp.BlockedReasons, resp.Warnings)
	}
	if !resp.SourceExists || !resp.TargetEmpty {
		t.Fatalf("source_exists/target_empty mismatch: %+v", resp)
	}
	if resp.Mode != sessionMigrationModeCopyLock {
		t.Fatalf("mode = %q, want default %q", resp.Mode, sessionMigrationModeCopyLock)
	}
}

func TestSessionMigratePreviewAllowsTargetWithOnlyStarterTurnZero(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_fresh"
	st := &sessionMigrationPreviewStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: sourceID, TurnIndex: 0, Role: "assistant", Content: "source opening"},
			{ID: 2, ChatSessionID: targetID, TurnIndex: 0, Role: "assistant", Content: "temporary target opening"},
		},
		memories: []store.Memory{{ID: 3, ChatSessionID: sourceID}},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sourceID: 1}}
	resp := performSessionMigrationPreview(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
	})

	if resp.Blocked || !resp.TargetEmpty {
		t.Fatalf("starter-only target must remain migration-empty: %+v", resp)
	}
	if !resp.TargetCounts.ReplaceableStarterOnly {
		t.Fatalf("starter-only target was not identified: %+v", resp.TargetCounts)
	}
	if !sessionMigrationContainsString(resp.Warnings, "target_starter_turn_zero_will_be_replaced") {
		t.Fatalf("starter replacement warning missing: %#v", resp.Warnings)
	}
}

func TestSessionMigratePreviewStillBlocksStarterPlusAnyOtherRow(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_used"
	st := &sessionMigrationPreviewStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: sourceID, TurnIndex: 0, Role: "assistant", Content: "source opening"},
			{ID: 2, ChatSessionID: targetID, TurnIndex: 0, Role: "assistant", Content: "target opening"},
			{ID: 3, ChatSessionID: targetID, TurnIndex: 1, Role: "user", Content: "already used"},
		},
	}
	resp := performSessionMigrationPreview(t, st, &sessionMigrationPreviewVector{}, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
	})

	if !resp.Blocked || resp.TargetEmpty || resp.TargetCounts.ReplaceableStarterOnly {
		t.Fatalf("starter plus another row must stay protected: %+v", resp)
	}
	if !sessionMigrationContainsString(resp.BlockedReasons, "target_session_not_empty") {
		t.Fatalf("target_session_not_empty missing: %#v", resp.BlockedReasons)
	}
}

func TestSessionMigrateCompleteCopiesOnlyAfterEmptyTargetPreview(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_fresh"
	st := &sessionMigrationPreviewStore{
		chatLogs:        []store.ChatLog{{ID: 1, ChatSessionID: sourceID}},
		effectiveInputs: []store.EffectiveInput{{ID: 2, ChatSessionID: sourceID}},
		memories:        []store.Memory{{ID: 3, ChatSessionID: sourceID}},
		completeResult: &store.SessionMigrationCompleteResult{
			MigrationID:           99,
			Status:                "copied",
			SourceSessionID:       sourceID,
			TargetSessionID:       targetID,
			Mode:                  sessionMigrationModeCopyLock,
			Counts:                store.SessionMigrationArtifactCounts{ChatLogs: 1, EffectiveInputs: 1, Memories: 1, CanonicalTotal: 3, CanonicalAndSubjectiveTotal: 3},
			RowMapCount:           3,
			SourceLocked:          false,
			ReadyForLive:          false,
			ChromaReindexRequired: true,
		},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sourceID: 3}}
	resp := performSessionMigrationComplete(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
		"operator_note":     "fresh continuation",
	})

	if !st.completeCalled {
		t.Fatalf("expected CompleteSessionMigration to be called")
	}
	if st.completeRequest.OperatorNote != "fresh continuation" {
		t.Fatalf("operator note = %q", st.completeRequest.OperatorNote)
	}
	if resp.Blocked || !resp.WriteAttempted || resp.VectorWriteAttempted || resp.LLMCallAttempted {
		t.Fatalf("unexpected complete response flags: %+v", resp)
	}
	if resp.MigrationID != 99 || resp.MigrationStatus != "copied" || resp.RowMapCount != 3 {
		t.Fatalf("unexpected migration result: %+v", resp)
	}
	if !resp.ChromaReindexRequired || resp.ReadyForLive {
		t.Fatalf("complete should remain pending Chroma reindex: %+v", resp)
	}
}

func TestSessionMigrateCompleteCopyKeepSourcePassesMode(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_copy"
	st := &sessionMigrationPreviewStore{
		chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: sourceID}},
		memories: []store.Memory{{ID: 3, ChatSessionID: sourceID}},
		completeResult: &store.SessionMigrationCompleteResult{
			MigrationID:           100,
			Status:                "copied",
			SourceSessionID:       sourceID,
			TargetSessionID:       targetID,
			Mode:                  sessionMigrationModeCopyKeep,
			Counts:                store.SessionMigrationArtifactCounts{ChatLogs: 1, Memories: 1, CanonicalTotal: 2, CanonicalAndSubjectiveTotal: 2},
			RowMapCount:           2,
			SourceLocked:          false,
			ReadyForLive:          false,
			ChromaReindexRequired: true,
		},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sourceID: 2}}
	resp := performSessionMigrationComplete(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
		"mode":              sessionMigrationModeCopyKeep,
		"operator_note":     "manual copy",
	})

	if !st.completeCalled {
		t.Fatalf("expected CompleteSessionMigration to be called")
	}
	if st.completeRequest.Mode != sessionMigrationModeCopyKeep {
		t.Fatalf("mode = %q, want %q", st.completeRequest.Mode, sessionMigrationModeCopyKeep)
	}
	if st.completeRequest.OperatorNote != "manual copy" {
		t.Fatalf("operator note = %q", st.completeRequest.OperatorNote)
	}
	if resp.Blocked || !resp.WriteAttempted || resp.Mode != sessionMigrationModeCopyKeep || resp.SourceLocked {
		t.Fatalf("unexpected copy_keep_source complete response: %+v", resp)
	}
	if resp.MigrationID != 100 || resp.RowMapCount != 2 {
		t.Fatalf("unexpected migration result: %+v", resp)
	}
}

func TestSessionMigrateCompleteDoesNotWriteWhenPreviewBlocked(t *testing.T) {
	sessionID := "char_59_cid_same"
	st := &sessionMigrationPreviewStore{chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: sessionID}}}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sessionID: 1}}
	resp := performSessionMigrationComplete(t, st, vec, map[string]string{
		"source_session_id": sessionID,
		"target_session_id": sessionID,
	})

	if st.completeCalled {
		t.Fatalf("CompleteSessionMigration was called despite blocked preview")
	}
	if !resp.Blocked || resp.WriteAttempted {
		t.Fatalf("expected blocked no-write response: %+v", resp)
	}
	if !sessionMigrationContainsString(resp.BlockedReasons, "source_target_must_differ") {
		t.Fatalf("blocked_reasons = %#v", resp.BlockedReasons)
	}
}

func TestSessionMigrateReindexUpsertsTargetVectorsAndMarksLedger(t *testing.T) {
	targetID := "char_59_cid_target"
	sourceID := "char_59_cid_source"
	st := &sessionMigrationPreviewStore{
		vectorDocs: []store.SessionMigrationVectorDocument{
			{
				ID:                    "memory:" + targetID + ":101",
				MigrationID:           42,
				Tier:                  "memory",
				ChatSessionID:         targetID,
				SourceTable:           "memories",
				SourceRowID:           "101",
				SchemaVersion:         "memory.v1",
				DocumentText:          "remembered hallway event",
				EmbeddingJSON:         `[0.1,0.2,0.3]`,
				MigratedFromSessionID: sourceID,
			},
			{
				ID:                    "episode:" + targetID + ":201",
				MigrationID:           42,
				Tier:                  "episode",
				ChatSessionID:         targetID,
				SourceTable:           "episode_summaries",
				SourceRowID:           "201",
				SchemaVersion:         "episode.v1",
				DocumentText:          "episode summary",
				EmbeddingJSON:         `[0.4,0.5,0.6]`,
				MigratedFromSessionID: sourceID,
			},
			{
				ID:            "memory:" + targetID + ":empty",
				MigrationID:   42,
				ChatSessionID: targetID,
				EmbeddingJSON: `[]`,
				DocumentText:  "skip",
			},
		},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{targetID: 0}}
	resp := performSessionMigrationReindex(t, st, vec, map[string]any{"migration_id": float64(42)})

	if resp.Blocked || resp.VerificationStatus != "verified" || !resp.ReadyForSourceLock || resp.ReadyForLive {
		t.Fatalf("unexpected reindex response: %+v", resp)
	}
	if !vec.upsertCalled || vec.upsertSessionID != targetID || len(vec.upsertDocs) != 2 {
		t.Fatalf("upsert call mismatch: called=%v sid=%q docs=%d", vec.upsertCalled, vec.upsertSessionID, len(vec.upsertDocs))
	}
	if vec.upsertDocs[0].MigrationID != 42 || vec.upsertDocs[0].MigratedFromSessionID != sourceID {
		t.Fatalf("migration metadata missing from vector doc: %+v", vec.upsertDocs[0])
	}
	if !st.vectorStatusCalled || st.vectorStatus != "vector_reindexed" || st.vectorStatusCount != 2 || st.vectorStatusErrors != "[]" {
		t.Fatalf("ledger update mismatch: called=%v status=%q count=%d errors=%q", st.vectorStatusCalled, st.vectorStatus, st.vectorStatusCount, st.vectorStatusErrors)
	}
	if resp.Candidates != 3 || resp.Upserted != 2 || resp.Skipped != 1 {
		t.Fatalf("candidate/upsert/skip mismatch: %+v", resp)
	}
}

func TestSessionMigrateReindexBlocksWithoutCandidates(t *testing.T) {
	st := &sessionMigrationPreviewStore{}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{}}
	resp := performSessionMigrationReindex(t, st, vec, map[string]any{"migration_id": float64(123)})

	if !resp.Blocked || !sessionMigrationContainsString(resp.BlockedReasons, "no_vector_candidates") {
		t.Fatalf("expected no_vector_candidates block: %+v", resp)
	}
	if vec.upsertCalled || st.vectorStatusCalled {
		t.Fatalf("unexpected side effect for no-vector-candidates")
	}
}

func TestSessionMigrateLockSourceWritesMigrationLock(t *testing.T) {
	st := &sessionMigrationPreviewStore{}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{}}
	resp := performSessionMigrationLockSource(t, st, vec, map[string]any{
		"migration_id": float64(42),
		"reason":       "operator confirmed target vectors",
	})

	if resp.Blocked || !resp.WriteAttempted || resp.VectorWriteAttempted || resp.LLMCallAttempted {
		t.Fatalf("unexpected lock response flags: %+v", resp)
	}
	if !st.lockCalled || st.lockReason != "operator confirmed target vectors" {
		t.Fatalf("lock store was not called correctly: called=%v reason=%q", st.lockCalled, st.lockReason)
	}
	if !resp.SourceLocked || !resp.ReadyForLive || resp.SourceSessionID != "char_59_cid_source" || resp.TargetSessionID != "char_59_cid_target" {
		t.Fatalf("lock response did not expose source/target readiness: %+v", resp)
	}
	if resp.Lock["lock_status"] != "migrated_away" {
		t.Fatalf("lock payload = %+v", resp.Lock)
	}
}

func TestSessionMigrateRollbackDeletesTargetVectorsAndRows(t *testing.T) {
	targetID := "char_59_cid_target"
	st := &sessionMigrationPreviewStore{
		vectorDocs: []store.SessionMigrationVectorDocument{
			{ID: "memory:" + targetID + ":101", MigrationID: 42},
			{ID: "episode:" + targetID + ":201", MigrationID: 42},
			{ID: "memory:" + targetID + ":101", MigrationID: 42},
		},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{targetID: 2}}
	resp := performSessionMigrationRollback(t, st, vec, map[string]any{
		"migration_id": float64(42),
		"reason":       "operator rollback during live test",
	})

	if resp.Blocked || !resp.WriteAttempted || !resp.VectorWriteAttempted || resp.LLMCallAttempted {
		t.Fatalf("unexpected rollback flags: %+v", resp)
	}
	if !st.rollbackCalled || st.rollbackReason != "operator rollback during live test" {
		t.Fatalf("rollback store call mismatch: called=%v reason=%q", st.rollbackCalled, st.rollbackReason)
	}
	if len(vec.deleteDocumentIDs) != 2 {
		t.Fatalf("DeleteDocuments ids = %#v, want two unique ids", vec.deleteDocumentIDs)
	}
	if !resp.RolledBack || !resp.SourceUnlocked || resp.TargetVectorDocumentsDeleted != 2 || resp.RowMapCount != 4 {
		t.Fatalf("rollback response mismatch: %+v", resp)
	}
}

func TestSessionMigrateCleanupSourceDryRunDoesNotDelete(t *testing.T) {
	st := &sessionMigrationPreviewStore{}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{"char_59_cid_source": 3}}
	resp := performSessionMigrationCleanupSource(t, st, vec, map[string]any{
		"migration_id": float64(42),
		"dry_run":      true,
	})

	if resp.Blocked || !resp.DryRun || resp.WriteAttempted || resp.VectorWriteAttempted || st.cleanupCalled || vec.deleteCalled {
		t.Fatalf("cleanup dry-run attempted side effects: resp=%+v cleanupCalled=%v deleteCalled=%v", resp, st.cleanupCalled, vec.deleteCalled)
	}
	if !resp.ReadyForCleanup || resp.SourceVectors != 3 || resp.SourceRows.ChatLogs != 2 {
		t.Fatalf("cleanup dry-run summary mismatch: %+v", resp)
	}
}

func TestSessionMigrateCleanupSourceConfirmDeletesSourceVectorsAndRows(t *testing.T) {
	st := &sessionMigrationPreviewStore{}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{"char_59_cid_source": 3}}
	resp := performSessionMigrationCleanupSource(t, st, vec, map[string]any{
		"migration_id":           float64(42),
		"confirm_source_cleanup": true,
		"reason":                 "source abandoned after verified target",
	})

	if resp.Blocked || !resp.WriteAttempted || !resp.VectorWriteAttempted || !resp.SourceCleaned || !resp.ReadyForLive {
		t.Fatalf("cleanup confirm response mismatch: %+v", resp)
	}
	if !st.cleanupCalled || st.cleanupReason != "source abandoned after verified target" {
		t.Fatalf("cleanup store call mismatch: called=%v reason=%q", st.cleanupCalled, st.cleanupReason)
	}
	if !vec.deleteCalled || vec.deleteSessionID != "char_59_cid_source" {
		t.Fatalf("source vector cleanup mismatch: called=%v sid=%q", vec.deleteCalled, vec.deleteSessionID)
	}
}

func TestSessionMigrationSourceLockExcludesPrepareSearchAndCompleteTurn(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_target"
	st := &sessionMigrationPreviewStore{
		activeLock: &store.SessionMigrationLock{
			MigrationID:     42,
			SourceSessionID: sourceID,
			TargetSessionID: targetID,
			Locked:          true,
			LockStatus:      "migrated_away",
			Reason:          "operator confirmed",
			LockedAt:        time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
		},
		memories: []store.Memory{{ID: 1, ChatSessionID: sourceID, SummaryJSON: `{"summary":"must not leak"}`}},
	}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{}}

	prepare := performSessionMigrationPrepareTurn(t, st, vec, `{"chat_session_id":"`+sourceID+`","turn_index":9,"raw_user_input":"continue"}`)
	if prepare["read_excluded"] != true || prepare["target_session_id"] != targetID || prepare["injection_text"] != "" {
		t.Fatalf("prepare-turn source lock did not exclude reads: %+v", prepare)
	}

	search := performSessionMigrationSearch(t, st, vec, `{"chat_session_id":"`+sourceID+`","user_input":"must","top_k":5}`)
	if search["read_excluded"] != true || search["target_session_id"] != targetID || search["memory_count"] != float64(0) {
		t.Fatalf("search source lock did not exclude reads: %+v", search)
	}

	complete := performSessionMigrationCompleteTurn(t, st, vec, `{"chat_session_id":"`+sourceID+`","turn_index":9,"user_input":"u","assistant_content":"a"}`)
	if complete["status"] != "blocked" || complete["save_ok"] != false || complete["save_error"] != "source_session_migrated_away" {
		t.Fatalf("complete-turn source lock did not block writes: %+v", complete)
	}
}

func TestSessionMigratePreviewReportsSemanticBlockers(t *testing.T) {
	sessionID := "char_59_cid_same"
	st := &sessionMigrationPreviewStore{chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: sessionID}}}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{sessionID: 1}}
	resp := performSessionMigrationPreview(t, st, vec, map[string]string{
		"source_session_id": sessionID,
		"target_session_id": sessionID,
		"mode":              "blind_rewrite",
	})

	if !resp.Blocked {
		t.Fatalf("expected same-session unsupported mode request to be blocked")
	}
	for _, reason := range []string{"source_target_must_differ", "unsupported_mode", "target_session_not_empty", "target_chroma_vectors_not_empty"} {
		if !sessionMigrationContainsString(resp.BlockedReasons, reason) {
			t.Fatalf("blocked_reasons = %#v, want %s", resp.BlockedReasons, reason)
		}
	}
}

func TestSessionMigratePreviewReportsChromaCountWarningWithoutWrite(t *testing.T) {
	sourceID := "char_59_cid_source"
	targetID := "char_59_cid_target"
	st := &sessionMigrationPreviewStore{chatLogs: []store.ChatLog{{ID: 1, ChatSessionID: sourceID}}}
	vec := &sessionMigrationPreviewVector{counts: map[string]int{}, err: errors.New("count unavailable")}
	resp := performSessionMigrationPreview(t, st, vec, map[string]string{
		"source_session_id": sourceID,
		"target_session_id": targetID,
	})

	if resp.Chroma.Status != "unavailable" || len(resp.Chroma.Errors) == 0 {
		t.Fatalf("chroma status/errors = %+v", resp.Chroma)
	}
	if resp.VectorWriteAttempted || vec.upsertCalled || vec.deleteCalled || vec.rebuildCalled {
		t.Fatalf("preview attempted vector write despite count warning")
	}
}

func performSessionMigrationPreview(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]string) sessionMigrationPreviewResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: st, Vector: vec}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-preview", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationPreviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationComplete(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]string) sessionMigrationCompleteResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: st, Vector: vec}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-complete", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationCompleteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationReindex(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]any) sessionMigrationReindexResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Cfg:    config.Config{ChromaEndpoint: "http://127.0.0.1:8000"},
		Store:  st,
		Vector: vec,
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-reindex", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationReindexResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationLockSource(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]any) sessionMigrationLockSourceResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: st, Vector: vec}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-lock-source", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationLockSourceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationRollback(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]any) sessionMigrationRollbackResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Cfg:    config.Config{ChromaEndpoint: "http://127.0.0.1:8000"},
		Store:  st,
		Vector: vec,
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-rollback", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationRollbackResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationCleanupSource(t *testing.T, st store.Store, vec vector.VectorStore, payload map[string]any) sessionMigrationCleanupSourceResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Cfg:    config.Config{ChromaEndpoint: "http://127.0.0.1:8000"},
		Store:  st,
		Vector: vec,
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/sessions/migrate-cleanup-source", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sessionMigrationCleanupSourceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func performSessionMigrationPrepareTurn(t *testing.T, st store.Store, vec vector.VectorStore, body string) map[string]any {
	t.Helper()
	return performSessionMigrationJSONRoute(t, st, vec, http.MethodPost, "/prepare-turn", body)
}

func performSessionMigrationSearch(t *testing.T, st store.Store, vec vector.VectorStore, body string) map[string]any {
	t.Helper()
	return performSessionMigrationJSONRoute(t, st, vec, http.MethodPost, "/search", body)
}

func performSessionMigrationCompleteTurn(t *testing.T, st store.Store, vec vector.VectorStore, body string) map[string]any {
	t.Helper()
	return performSessionMigrationJSONRoute(t, st, vec, http.MethodPost, "/complete-turn", body)
}

func performSessionMigrationJSONRoute(t *testing.T, st store.Store, vec vector.VectorStore, method, path, body string) map[string]any {
	t.Helper()
	srv := &Server{Store: st, Vector: vec}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func sessionMigrationContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
