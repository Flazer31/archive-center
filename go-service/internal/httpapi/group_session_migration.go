package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const (
	sessionMigrationPreviewVersion  = "sc-mig-preview.v1"
	sessionMigrationCompleteVersion = "sc-mig-complete.v1"
	sessionMigrationReindexVersion  = "sc-mig-reindex.v1"
	sessionMigrationLockVersion     = "sc-mig-lock.v1"
	sessionMigrationRollbackVersion = "sc-mig-rollback.v1"
	sessionMigrationCleanupVersion  = "sc-mig-cleanup.v1"
	sessionMigrationModeCopyLock    = store.SessionMigrationModeCopyThenLockSource
	sessionMigrationModeCopyKeep    = store.SessionMigrationModeCopyKeepSource
	sessionMigrationSubjectiveCap   = 1000
)

type sessionMigrationPreviewRequest struct {
	SourceSessionID string `json:"source_session_id"`
	TargetSessionID string `json:"target_session_id"`
	Mode            string `json:"mode"`
}

type sessionMigrationPreviewCounts struct {
	ChatLogs                    int  `json:"chat_logs"`
	EffectiveInputs             int  `json:"effective_inputs"`
	Memories                    int  `json:"memories"`
	DirectEvidence              int  `json:"direct_evidence"`
	KGTriples                   int  `json:"kg_triples"`
	Episodes                    int  `json:"episodes"`
	SubjectiveEntityMemories    int  `json:"subjective_entity_memories"`
	ChromaVectors               int  `json:"chroma_vectors"`
	CanonicalTotal              int  `json:"canonical_total"`
	CanonicalAndSubjectiveTotal int  `json:"canonical_and_subjective_total"`
	ReplaceableStarterOnly      bool `json:"replaceable_starter_only"`
}

type sessionMigrationChromaPreview struct {
	Status              string   `json:"status"`
	SourceVectors       int      `json:"source_vectors"`
	TargetVectors       int      `json:"target_vectors"`
	CountAttempted      bool     `json:"count_attempted"`
	WriteAttempted      bool     `json:"write_attempted"`
	Errors              []string `json:"errors"`
	RequiredForComplete bool     `json:"required_for_complete"`
}

type sessionMigrationPreviewResponse struct {
	Status               string                        `json:"status"`
	ContractVersion      string                        `json:"contract_version"`
	DryRun               bool                          `json:"dry_run"`
	ReadOnly             bool                          `json:"read_only"`
	WriteAttempted       bool                          `json:"write_attempted"`
	VectorWriteAttempted bool                          `json:"vector_write_attempted"`
	LLMCallAttempted     bool                          `json:"llm_call_attempted"`
	SourceSessionID      string                        `json:"source_session_id"`
	TargetSessionID      string                        `json:"target_session_id"`
	Mode                 string                        `json:"mode"`
	SourceExists         bool                          `json:"source_exists"`
	TargetEmpty          bool                          `json:"target_empty"`
	Blocked              bool                          `json:"blocked"`
	BlockedReasons       []string                      `json:"blocked_reasons"`
	Warnings             []string                      `json:"warnings"`
	Counts               sessionMigrationPreviewCounts `json:"counts"`
	TargetCounts         sessionMigrationPreviewCounts `json:"target_counts"`
	Chroma               sessionMigrationChromaPreview `json:"chroma"`
	GeneratedAt          string                        `json:"generated_at"`
}

func (s *Server) registerSessionMigrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions/migrate-preview", s.handleSessionMigratePreview)
	mux.HandleFunc("POST /sessions/migrate-complete", s.handleSessionMigrateComplete)
	mux.HandleFunc("POST /sessions/migrate-reindex", s.handleSessionMigrateReindex)
	mux.HandleFunc("POST /sessions/migrate-lock-source", s.handleSessionMigrateLockSource)
	mux.HandleFunc("POST /sessions/migrate-rollback", s.handleSessionMigrateRollback)
	mux.HandleFunc("POST /sessions/migrate-cleanup-source", s.handleSessionMigrateCleanupSource)
}

func (s *Server) handleSessionMigratePreview(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	sourceID := strings.TrimSpace(req.SourceSessionID)
	targetID := strings.TrimSpace(req.TargetSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = sessionMigrationModeCopyLock
	}

	blockedReasons := []string{}
	warnings := []string{}
	if sourceID == "" {
		blockedReasons = append(blockedReasons, "source_session_id_required")
	}
	if targetID == "" {
		blockedReasons = append(blockedReasons, "target_session_id_required")
	}
	if sourceID != "" && targetID != "" && sourceID == targetID {
		blockedReasons = append(blockedReasons, "source_target_must_differ")
	}
	if !sessionMigrationModeSupported(mode) {
		blockedReasons = append(blockedReasons, "unsupported_mode")
		warnings = append(warnings, "supported_modes: "+sessionMigrationSupportedModesText())
	}

	sourceCounts, sourceWarnings, sourceErr := s.sessionMigrationPreviewCounts(r.Context(), sourceID)
	if sourceErr != nil {
		writeInternalError(w, sourceErr.Error())
		return
	}
	warnings = append(warnings, sourceWarnings...)
	targetCounts, targetWarnings, targetErr := s.sessionMigrationPreviewCounts(r.Context(), targetID)
	if targetErr != nil {
		writeInternalError(w, targetErr.Error())
		return
	}
	warnings = append(warnings, targetWarnings...)

	chroma := s.sessionMigrationPreviewChroma(r.Context(), sourceID, targetID)
	warnings = append(warnings, chroma.Errors...)
	sourceCounts.ChromaVectors = chroma.SourceVectors
	targetCounts.ChromaVectors = chroma.TargetVectors

	sourceExists := sourceCounts.CanonicalAndSubjectiveTotal > 0
	targetEmpty := (targetCounts.CanonicalAndSubjectiveTotal == 0 || targetCounts.ReplaceableStarterOnly) && chroma.TargetVectors == 0
	if sourceID != "" && !sourceExists {
		blockedReasons = append(blockedReasons, "source_session_has_no_archive_data")
	}
	if targetID != "" && !targetEmpty {
		blockedReasons = append(blockedReasons, "target_session_not_empty")
	}
	if chroma.TargetVectors > 0 {
		blockedReasons = append(blockedReasons, "target_chroma_vectors_not_empty")
	}
	if targetCounts.ReplaceableStarterOnly {
		warnings = append(warnings, "target_starter_turn_zero_will_be_replaced")
	}
	if sourceCounts.CanonicalAndSubjectiveTotal == 0 && chroma.SourceVectors > 0 {
		warnings = append(warnings, "source_vectors_exist_without_canonical_rows")
	}

	writeJSON(w, http.StatusOK, sessionMigrationPreviewResponse{
		Status:               "ok",
		ContractVersion:      sessionMigrationPreviewVersion,
		DryRun:               true,
		ReadOnly:             true,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		SourceSessionID:      sourceID,
		TargetSessionID:      targetID,
		Mode:                 mode,
		SourceExists:         sourceExists,
		TargetEmpty:          targetEmpty,
		Blocked:              len(blockedReasons) > 0,
		BlockedReasons:       blockedReasons,
		Warnings:             warnings,
		Counts:               sourceCounts,
		TargetCounts:         targetCounts,
		Chroma:               chroma,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	})
}

type sessionMigrationCompleteRequest struct {
	SourceSessionID string `json:"source_session_id"`
	TargetSessionID string `json:"target_session_id"`
	Mode            string `json:"mode"`
	OperatorNote    string `json:"operator_note"`
}

type sessionMigrationCompleteResponse struct {
	Status                string                        `json:"status"`
	ContractVersion       string                        `json:"contract_version"`
	WriteAttempted        bool                          `json:"write_attempted"`
	VectorWriteAttempted  bool                          `json:"vector_write_attempted"`
	LLMCallAttempted      bool                          `json:"llm_call_attempted"`
	MigrationID           int64                         `json:"migration_id"`
	MigrationStatus       string                        `json:"migration_status"`
	SourceSessionID       string                        `json:"source_session_id"`
	TargetSessionID       string                        `json:"target_session_id"`
	Mode                  string                        `json:"mode"`
	Counts                sessionMigrationPreviewCounts `json:"counts"`
	RowMapCount           int                           `json:"row_map_count"`
	SourceLocked          bool                          `json:"source_locked"`
	ChromaReindexRequired bool                          `json:"chroma_reindex_required"`
	ReadyForLive          bool                          `json:"ready_for_live"`
	TargetStarterReplaced bool                          `json:"target_starter_replaced"`
	Blocked               bool                          `json:"blocked"`
	BlockedReasons        []string                      `json:"blocked_reasons"`
	Warnings              []string                      `json:"warnings"`
	GeneratedAt           string                        `json:"generated_at"`
}

func (s *Server) handleSessionMigrateComplete(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	sourceID := strings.TrimSpace(req.SourceSessionID)
	targetID := strings.TrimSpace(req.TargetSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = sessionMigrationModeCopyLock
	}

	blockedReasons, warnings, sourceCounts, targetCounts, chroma, err := s.sessionMigrationValidate(r.Context(), sourceID, targetID, mode)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if len(blockedReasons) > 0 {
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:               "ok",
			ContractVersion:      sessionMigrationCompleteVersion,
			WriteAttempted:       false,
			VectorWriteAttempted: false,
			LLMCallAttempted:     false,
			SourceSessionID:      sourceID,
			TargetSessionID:      targetID,
			Mode:                 mode,
			Counts:               sourceCounts,
			Blocked:              true,
			BlockedReasons:       blockedReasons,
			Warnings:             warnings,
			GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		})
		_ = targetCounts
		_ = chroma
		return
	}

	migrationStore, ok := s.Store.(store.SessionMigrationStore)
	if !ok {
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:               "ok",
			ContractVersion:      sessionMigrationCompleteVersion,
			WriteAttempted:       false,
			VectorWriteAttempted: false,
			LLMCallAttempted:     false,
			SourceSessionID:      sourceID,
			TargetSessionID:      targetID,
			Mode:                 mode,
			Counts:               sourceCounts,
			Blocked:              true,
			BlockedReasons:       []string{"session_migration_store_unavailable"},
			Warnings:             append(warnings, "complete migration requires MariaDB authority store"),
			GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	result, err := migrationStore.CompleteSessionMigration(r.Context(), store.SessionMigrationCompleteRequest{
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Mode:            mode,
		OperatorNote:    strings.TrimSpace(req.OperatorNote),
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
		Status:                "ok",
		ContractVersion:       sessionMigrationCompleteVersion,
		WriteAttempted:        true,
		VectorWriteAttempted:  false,
		LLMCallAttempted:      false,
		MigrationID:           result.MigrationID,
		MigrationStatus:       result.Status,
		SourceSessionID:       result.SourceSessionID,
		TargetSessionID:       result.TargetSessionID,
		Mode:                  result.Mode,
		Counts:                sessionMigrationCountsFromStore(result.Counts),
		RowMapCount:           result.RowMapCount,
		SourceLocked:          result.SourceLocked,
		ChromaReindexRequired: result.ChromaReindexRequired,
		ReadyForLive:          result.ReadyForLive,
		TargetStarterReplaced: result.TargetStarterReplaced,
		Blocked:               false,
		BlockedReasons:        []string{},
		Warnings:              append(warnings, "chroma_reindex_pending: run SC-MIG-5 before treating target as live-complete"),
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	})
}

type sessionMigrationReindexRequest struct {
	MigrationID int64 `json:"migration_id"`
}

type sessionMigrationReindexResponse struct {
	Status                  string   `json:"status"`
	ContractVersion         string   `json:"contract_version"`
	MigrationID             int64    `json:"migration_id"`
	WriteAttempted          bool     `json:"write_attempted"`
	VectorWriteAttempted    bool     `json:"vector_write_attempted"`
	LLMCallAttempted        bool     `json:"llm_call_attempted"`
	Candidates              int      `json:"candidates"`
	Upserted                int      `json:"upserted"`
	Skipped                 int      `json:"skipped"`
	SkippedIDs              []string `json:"skipped_ids"`
	TargetSessionID         string   `json:"target_session_id"`
	TargetVectorCountBefore int      `json:"target_vector_count_before"`
	TargetVectorCountAfter  int      `json:"target_vector_count_after"`
	VerificationStatus      string   `json:"verification_status"`
	ReadyForSourceLock      bool     `json:"ready_for_source_lock"`
	ReadyForLive            bool     `json:"ready_for_live"`
	Blocked                 bool     `json:"blocked"`
	BlockedReasons          []string `json:"blocked_reasons"`
	Warnings                []string `json:"warnings"`
	Errors                  []string `json:"errors"`
	GeneratedAt             string   `json:"generated_at"`
}

func (s *Server) handleSessionMigrateReindex(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationReindexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if req.MigrationID <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "migration_id must be positive")
		return
	}
	resp := sessionMigrationReindexResponse{
		Status:               "ok",
		ContractVersion:      sessionMigrationReindexVersion,
		MigrationID:          req.MigrationID,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		VerificationStatus:   "not_run",
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}

	migrationVectorStore, ok := s.Store.(store.SessionMigrationVectorStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_vector_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.Vector == nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_endpoint_not_configured")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.VectorOpenError != nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_open_error")
		resp.Errors = append(resp.Errors, s.VectorOpenError.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}

	candidates, err := migrationVectorStore.ListSessionMigrationVectorDocuments(r.Context(), req.MigrationID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp.Candidates = len(candidates)
	docs := make([]vector.VectorDocument, 0, len(candidates))
	targetSessionID := ""
	for _, candidate := range candidates {
		if targetSessionID == "" {
			targetSessionID = strings.TrimSpace(candidate.ChatSessionID)
		}
		embedding := parseFloat32JSONList(candidate.EmbeddingJSON)
		if len(embedding) == 0 {
			resp.Skipped++
			resp.SkippedIDs = append(resp.SkippedIDs, candidate.ID)
			continue
		}
		if strings.TrimSpace(candidate.DocumentText) == "" {
			resp.Skipped++
			resp.SkippedIDs = append(resp.SkippedIDs, candidate.ID)
			continue
		}
		docs = append(docs, vector.VectorDocument{
			ID:                    candidate.ID,
			Embedding:             embedding,
			Tier:                  candidate.Tier,
			ChatSessionID:         candidate.ChatSessionID,
			SourceTable:           candidate.SourceTable,
			SourceRowID:           candidate.SourceRowID,
			SchemaVersion:         candidate.SchemaVersion,
			DocumentText:          candidate.DocumentText,
			MigrationID:           candidate.MigrationID,
			MigratedFromSessionID: candidate.MigratedFromSessionID,
		})
	}
	resp.TargetSessionID = targetSessionID
	if len(docs) == 0 {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "no_vector_candidates")
		resp.VerificationStatus = "no_vector_candidates"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	before, err := s.Vector.Count(r.Context(), targetSessionID)
	if err != nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "target_chroma_count_unavailable")
		resp.Errors = append(resp.Errors, err.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.TargetVectorCountBefore = before
	resp.VectorWriteAttempted = true
	if err := s.Vector.Upsert(r.Context(), targetSessionID, docs); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		resp.VerificationStatus = "upsert_failed"
		_ = migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindex_failed", 0, mustCompactJSON(resp.Errors))
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Upserted = len(docs)
	after, err := s.Vector.Count(r.Context(), targetSessionID)
	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		resp.VerificationStatus = "verification_count_failed"
		_ = migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindex_unverified", resp.Upserted, mustCompactJSON(resp.Errors))
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.TargetVectorCountAfter = after
	expectedMinimum := before
	if resp.Upserted > expectedMinimum {
		expectedMinimum = resp.Upserted
	}
	if after >= expectedMinimum {
		resp.WriteAttempted = true
		resp.VerificationStatus = "verified"
		resp.ReadyForSourceLock = true
		resp.ReadyForLive = false
		if err := migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindexed", resp.Upserted, "[]"); err != nil {
			writeInternalError(w, err.Error())
			return
		}
	} else {
		resp.WriteAttempted = true
		resp.VerificationStatus = "insufficient_target_vectors"
		resp.Errors = append(resp.Errors, "target vector count is lower than expected migration verification minimum")
		if err := migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindex_unverified", resp.Upserted, mustCompactJSON(resp.Errors)); err != nil {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type sessionMigrationLockSourceRequest struct {
	MigrationID int64  `json:"migration_id"`
	Reason      string `json:"reason"`
}

type sessionMigrationLockSourceResponse struct {
	Status               string         `json:"status"`
	ContractVersion      string         `json:"contract_version"`
	MigrationID          int64          `json:"migration_id"`
	SourceSessionID      string         `json:"source_session_id"`
	TargetSessionID      string         `json:"target_session_id"`
	WriteAttempted       bool           `json:"write_attempted"`
	VectorWriteAttempted bool           `json:"vector_write_attempted"`
	LLMCallAttempted     bool           `json:"llm_call_attempted"`
	SourceLocked         bool           `json:"source_locked"`
	ReadyForLive         bool           `json:"ready_for_live"`
	Blocked              bool           `json:"blocked"`
	BlockedReasons       []string       `json:"blocked_reasons"`
	Warnings             []string       `json:"warnings"`
	Errors               []string       `json:"errors"`
	Lock                 map[string]any `json:"lock"`
	GeneratedAt          string         `json:"generated_at"`
}

func (s *Server) handleSessionMigrateLockSource(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationLockSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if req.MigrationID <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "migration_id must be positive")
		return
	}
	resp := sessionMigrationLockSourceResponse{
		Status:               "ok",
		ContractVersion:      sessionMigrationLockVersion,
		MigrationID:          req.MigrationID,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		SourceLocked:         false,
		ReadyForLive:         false,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	lockStore, ok := s.Store.(store.SessionMigrationSourceLockStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_source_lock_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	result, err := lockStore.LockSessionMigrationSource(r.Context(), req.MigrationID, req.Reason)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if strings.Contains(err.Error(), "blocked:") {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.WriteAttempted = true
	resp.MigrationID = result.MigrationID
	resp.SourceSessionID = result.SourceSessionID
	resp.TargetSessionID = result.TargetSessionID
	resp.SourceLocked = result.Lock.Locked
	resp.ReadyForLive = result.ReadyForLive
	resp.Lock = sessionMigrationLockPayload(&result.Lock)
	writeJSON(w, http.StatusOK, resp)
}

type sessionMigrationRollbackRequest struct {
	MigrationID int64  `json:"migration_id"`
	Reason      string `json:"reason"`
}

type sessionMigrationRollbackResponse struct {
	Status                       string                        `json:"status"`
	ContractVersion              string                        `json:"contract_version"`
	MigrationID                  int64                         `json:"migration_id"`
	MigrationStatus              string                        `json:"migration_status"`
	SourceSessionID              string                        `json:"source_session_id"`
	TargetSessionID              string                        `json:"target_session_id"`
	WriteAttempted               bool                          `json:"write_attempted"`
	VectorWriteAttempted         bool                          `json:"vector_write_attempted"`
	LLMCallAttempted             bool                          `json:"llm_call_attempted"`
	RolledBack                   bool                          `json:"rolled_back"`
	SourceUnlocked               bool                          `json:"source_unlocked"`
	ReadyForLive                 bool                          `json:"ready_for_live"`
	RowsDeleted                  sessionMigrationPreviewCounts `json:"rows_deleted"`
	RowMapCount                  int                           `json:"row_map_count"`
	TargetVectorDocumentsDeleted int                           `json:"target_vector_documents_deleted"`
	Blocked                      bool                          `json:"blocked"`
	BlockedReasons               []string                      `json:"blocked_reasons"`
	Warnings                     []string                      `json:"warnings"`
	Errors                       []string                      `json:"errors"`
	GeneratedAt                  string                        `json:"generated_at"`
}

func (s *Server) handleSessionMigrateRollback(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if req.MigrationID <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "migration_id must be positive")
		return
	}
	resp := sessionMigrationRollbackResponse{
		Status:               "ok",
		ContractVersion:      sessionMigrationRollbackVersion,
		MigrationID:          req.MigrationID,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	recoveryStore, ok := s.Store.(store.SessionMigrationRecoveryStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_recovery_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	migrationVectorStore, ok := s.Store.(store.SessionMigrationVectorStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_vector_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	vectorDocs, err := migrationVectorStore.ListSessionMigrationVectorDocuments(r.Context(), req.MigrationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	vectorIDs := sessionMigrationUniqueVectorIDs(vectorDocs)
	if len(vectorIDs) > 0 {
		if s.Vector == nil {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "vector_store_unavailable")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "chroma_endpoint_not_configured")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if s.VectorOpenError != nil {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "chroma_open_error")
			resp.Errors = append(resp.Errors, s.VectorOpenError.Error())
			writeJSON(w, http.StatusOK, resp)
			return
		}
		documentDeleter, ok := s.Vector.(vector.DocumentDeleter)
		if !ok {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "vector_document_delete_unavailable")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp.VectorWriteAttempted = true
		if err := documentDeleter.DeleteDocuments(r.Context(), vectorIDs); err != nil {
			resp.Errors = append(resp.Errors, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp.TargetVectorDocumentsDeleted = len(vectorIDs)
	}

	result, err := recoveryStore.RollbackSessionMigration(r.Context(), req.MigrationID, strings.TrimSpace(req.Reason))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if strings.Contains(err.Error(), "blocked:") {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.WriteAttempted = true
	resp.RolledBack = result.Status == "rolled_back"
	resp.MigrationStatus = result.Status
	resp.SourceSessionID = result.SourceSessionID
	resp.TargetSessionID = result.TargetSessionID
	resp.SourceUnlocked = result.SourceUnlocked
	resp.ReadyForLive = result.ReadyForLive
	resp.RowsDeleted = sessionMigrationCountsFromStore(result.Counts)
	resp.RowMapCount = result.RowMapCount
	writeJSON(w, http.StatusOK, resp)
}

type sessionMigrationCleanupSourceRequest struct {
	MigrationID          int64  `json:"migration_id"`
	Reason               string `json:"reason"`
	DryRun               bool   `json:"dry_run"`
	ConfirmSourceCleanup bool   `json:"confirm_source_cleanup"`
}

type sessionMigrationCleanupSourceResponse struct {
	Status               string                        `json:"status"`
	ContractVersion      string                        `json:"contract_version"`
	MigrationID          int64                         `json:"migration_id"`
	MigrationStatus      string                        `json:"migration_status"`
	SourceSessionID      string                        `json:"source_session_id"`
	TargetSessionID      string                        `json:"target_session_id"`
	DryRun               bool                          `json:"dry_run"`
	WriteAttempted       bool                          `json:"write_attempted"`
	VectorWriteAttempted bool                          `json:"vector_write_attempted"`
	LLMCallAttempted     bool                          `json:"llm_call_attempted"`
	SourceLocked         bool                          `json:"source_locked"`
	SourceCleaned        bool                          `json:"source_cleaned"`
	ReadyForCleanup      bool                          `json:"ready_for_cleanup"`
	ReadyForLive         bool                          `json:"ready_for_live"`
	SourceRows           sessionMigrationPreviewCounts `json:"source_rows"`
	SourceVectors        int                           `json:"source_vectors"`
	Blocked              bool                          `json:"blocked"`
	BlockedReasons       []string                      `json:"blocked_reasons"`
	Warnings             []string                      `json:"warnings"`
	Errors               []string                      `json:"errors"`
	GeneratedAt          string                        `json:"generated_at"`
}

func (s *Server) handleSessionMigrateCleanupSource(w http.ResponseWriter, r *http.Request) {
	var req sessionMigrationCleanupSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if req.MigrationID <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "migration_id must be positive")
		return
	}
	resp := sessionMigrationCleanupSourceResponse{
		Status:               "ok",
		ContractVersion:      sessionMigrationCleanupVersion,
		MigrationID:          req.MigrationID,
		DryRun:               req.DryRun || !req.ConfirmSourceCleanup,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	recoveryStore, ok := s.Store.(store.SessionMigrationRecoveryStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_recovery_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	preview, err := recoveryStore.PreviewSessionMigrationSourceCleanup(r.Context(), req.MigrationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.MigrationStatus = preview.Status
	resp.SourceSessionID = preview.SourceSessionID
	resp.TargetSessionID = preview.TargetSessionID
	resp.SourceLocked = preview.SourceLocked
	resp.ReadyForCleanup = preview.ReadyForCleanup
	resp.SourceRows = sessionMigrationCountsFromStore(preview.Counts)
	resp.Blocked = len(preview.BlockedReasons) > 0
	resp.BlockedReasons = append(resp.BlockedReasons, preview.BlockedReasons...)
	if s.Vector != nil && strings.TrimSpace(preview.SourceSessionID) != "" {
		count, err := s.Vector.Count(r.Context(), preview.SourceSessionID)
		if err != nil {
			resp.Warnings = append(resp.Warnings, "source_chroma_count_unavailable: "+err.Error())
		} else {
			resp.SourceVectors = count
		}
	}
	if resp.DryRun || resp.Blocked {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.Vector == nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_endpoint_not_configured")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.VectorOpenError != nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_open_error")
		resp.Errors = append(resp.Errors, s.VectorOpenError.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.VectorWriteAttempted = true
	if err := s.Vector.DeleteSession(r.Context(), preview.SourceSessionID); err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}
	result, err := recoveryStore.CleanupSessionMigrationSource(r.Context(), req.MigrationID, strings.TrimSpace(req.Reason))
	if err != nil {
		if strings.Contains(err.Error(), "blocked:") {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.WriteAttempted = true
	resp.MigrationStatus = result.Status
	resp.SourceCleaned = result.SourceCleaned
	resp.ReadyForLive = result.ReadyForLive
	resp.SourceRows = sessionMigrationCountsFromStore(result.Counts)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) sessionMigrationSourceLock(ctx context.Context, sessionID string) (*store.SessionMigrationLock, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || s.Store == nil {
		return nil, nil
	}
	lockStore, ok := s.Store.(store.SessionMigrationSourceLockStore)
	if !ok {
		return nil, nil
	}
	lock, err := lockStore.GetSessionMigrationSourceLock(ctx, sessionID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrNotEnabled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lock == nil || !lock.Locked {
		return nil, nil
	}
	return lock, nil
}

func sessionMigrationLockPayload(lock *store.SessionMigrationLock) map[string]any {
	if lock == nil {
		return nil
	}
	return map[string]any{
		"migration_id":      lock.MigrationID,
		"source_session_id": lock.SourceSessionID,
		"target_session_id": lock.TargetSessionID,
		"locked":            lock.Locked,
		"lock_status":       lock.LockStatus,
		"reason":            lock.Reason,
		"locked_at":         lock.LockedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Server) sessionMigrationPreviewCounts(ctx context.Context, sessionID string) (sessionMigrationPreviewCounts, []string, error) {
	counts := sessionMigrationPreviewCounts{}
	warnings := []string{}
	if strings.TrimSpace(sessionID) == "" || s.Store == nil {
		return counts, warnings, nil
	}

	chatLogs, err := s.Store.ListChatLogs(ctx, sessionID, 0, 0)
	if err != nil {
		return counts, warnings, sessionMigrationReadError("chat_logs", err)
	}
	counts.ChatLogs = len(chatLogs)

	if inputStore, ok := s.Store.(store.EffectiveInputListStore); ok {
		inputs, err := inputStore.ListEffectiveInputs(ctx, sessionID, 0, 0)
		if err != nil {
			return counts, warnings, sessionMigrationReadError("effective_input_logs", err)
		}
		counts.EffectiveInputs = len(inputs)
	}

	memories, err := s.Store.ListMemories(ctx, sessionID, 0, 0)
	if err != nil {
		return counts, warnings, sessionMigrationReadError("memories", err)
	}
	counts.Memories = len(memories)

	evidence, err := s.Store.ListEvidence(ctx, sessionID)
	if err != nil {
		return counts, warnings, sessionMigrationReadError("direct_evidence", err)
	}
	counts.DirectEvidence = len(evidence)

	triples, err := s.Store.ListKGTriples(ctx, sessionID)
	if err != nil {
		return counts, warnings, sessionMigrationReadError("kg_triples", err)
	}
	counts.KGTriples = len(triples)

	episodes, err := s.Store.ListEpisodeSummaries(ctx, sessionID, 100000, 0, 0)
	if err != nil {
		return counts, warnings, sessionMigrationReadError("episodes", err)
	}
	counts.Episodes = len(episodes)

	if subjectStore, ok := s.Store.(store.ProtagonistEntityMemoryStore); ok {
		subjective, err := subjectStore.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{
			SourceChatSessionID: sessionID,
			Limit:               sessionMigrationSubjectiveCap,
		})
		if err != nil {
			return counts, warnings, sessionMigrationReadError("subjective_entity_memories", err)
		}
		counts.SubjectiveEntityMemories = len(subjective)
		if len(subjective) >= sessionMigrationSubjectiveCap {
			warnings = append(warnings, "subjective_entity_memories_count_capped_at_1000")
		}
	}

	counts.CanonicalTotal = counts.ChatLogs + counts.EffectiveInputs + counts.Memories + counts.DirectEvidence + counts.KGTriples + counts.Episodes
	counts.CanonicalAndSubjectiveTotal = counts.CanonicalTotal + counts.SubjectiveEntityMemories
	if counts.CanonicalAndSubjectiveTotal == 1 && len(chatLogs) == 1 {
		row := chatLogs[0]
		counts.ReplaceableStarterOnly = row.TurnIndex == 0 && strings.EqualFold(strings.TrimSpace(row.Role), "assistant")
	}
	return counts, warnings, nil
}

func (s *Server) sessionMigrationValidate(ctx context.Context, sourceID, targetID, mode string) ([]string, []string, sessionMigrationPreviewCounts, sessionMigrationPreviewCounts, sessionMigrationChromaPreview, error) {
	blockedReasons := []string{}
	warnings := []string{}
	if sourceID == "" {
		blockedReasons = append(blockedReasons, "source_session_id_required")
	}
	if targetID == "" {
		blockedReasons = append(blockedReasons, "target_session_id_required")
	}
	if sourceID != "" && targetID != "" && sourceID == targetID {
		blockedReasons = append(blockedReasons, "source_target_must_differ")
	}
	if !sessionMigrationModeSupported(mode) {
		blockedReasons = append(blockedReasons, "unsupported_mode")
		warnings = append(warnings, "supported_modes: "+sessionMigrationSupportedModesText())
	}

	sourceCounts, sourceWarnings, sourceErr := s.sessionMigrationPreviewCounts(ctx, sourceID)
	if sourceErr != nil {
		return nil, nil, sessionMigrationPreviewCounts{}, sessionMigrationPreviewCounts{}, sessionMigrationChromaPreview{}, sourceErr
	}
	warnings = append(warnings, sourceWarnings...)
	targetCounts, targetWarnings, targetErr := s.sessionMigrationPreviewCounts(ctx, targetID)
	if targetErr != nil {
		return nil, nil, sessionMigrationPreviewCounts{}, sessionMigrationPreviewCounts{}, sessionMigrationChromaPreview{}, targetErr
	}
	warnings = append(warnings, targetWarnings...)

	chroma := s.sessionMigrationPreviewChroma(ctx, sourceID, targetID)
	warnings = append(warnings, chroma.Errors...)
	sourceCounts.ChromaVectors = chroma.SourceVectors
	targetCounts.ChromaVectors = chroma.TargetVectors

	if sourceID != "" && sourceCounts.CanonicalAndSubjectiveTotal == 0 {
		blockedReasons = append(blockedReasons, "source_session_has_no_archive_data")
	}
	if targetID != "" && ((targetCounts.CanonicalAndSubjectiveTotal > 0 && !targetCounts.ReplaceableStarterOnly) || chroma.TargetVectors > 0) {
		blockedReasons = append(blockedReasons, "target_session_not_empty")
	}
	if chroma.TargetVectors > 0 {
		blockedReasons = append(blockedReasons, "target_chroma_vectors_not_empty")
	}
	if targetCounts.ReplaceableStarterOnly {
		warnings = append(warnings, "target_starter_turn_zero_will_be_replaced")
	}
	if sourceCounts.CanonicalAndSubjectiveTotal == 0 && chroma.SourceVectors > 0 {
		warnings = append(warnings, "source_vectors_exist_without_canonical_rows")
	}
	return blockedReasons, warnings, sourceCounts, targetCounts, chroma, nil
}

func sessionMigrationCountsFromStore(in store.SessionMigrationArtifactCounts) sessionMigrationPreviewCounts {
	return sessionMigrationPreviewCounts{
		ChatLogs:                    in.ChatLogs,
		EffectiveInputs:             in.EffectiveInputs,
		Memories:                    in.Memories,
		DirectEvidence:              in.DirectEvidence,
		KGTriples:                   in.KGTriples,
		Episodes:                    in.Episodes,
		SubjectiveEntityMemories:    in.SubjectiveEntityMemories,
		CanonicalTotal:              in.CanonicalTotal,
		CanonicalAndSubjectiveTotal: in.CanonicalAndSubjectiveTotal,
		ReplaceableStarterOnly:      in.ReplaceableStarterOnly,
	}
}

func sessionMigrationModeSupported(mode string) bool {
	switch strings.TrimSpace(mode) {
	case sessionMigrationModeCopyLock, sessionMigrationModeCopyKeep:
		return true
	default:
		return false
	}
}

func sessionMigrationSupportedModesText() string {
	return sessionMigrationModeCopyLock + "," + sessionMigrationModeCopyKeep
}

func sessionMigrationUniqueVectorIDs(docs []store.SessionMigrationVectorDocument) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, doc := range docs {
		id := strings.TrimSpace(doc.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func sessionMigrationReadError(table string, err error) error {
	if errors.Is(err, store.ErrNotEnabled) {
		return nil
	}
	return errors.New(table + " read failed: " + err.Error())
}

func (s *Server) sessionMigrationPreviewChroma(ctx context.Context, sourceID, targetID string) sessionMigrationChromaPreview {
	out := sessionMigrationChromaPreview{
		Status:              "ok",
		CountAttempted:      true,
		WriteAttempted:      false,
		RequiredForComplete: true,
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		out.Status = "shadow"
		out.Errors = append(out.Errors, "chroma_endpoint_not_configured: complete migration will require ChromaDB")
	}
	if s.VectorOpenError != nil {
		out.Status = "unavailable"
		out.Errors = append(out.Errors, "chroma_open_error: "+s.VectorOpenError.Error())
	}
	if s.Vector == nil {
		out.Status = "unavailable"
		out.CountAttempted = false
		out.Errors = append(out.Errors, "chroma_count_unavailable: vector store is not configured")
		return out
	}
	if strings.TrimSpace(sourceID) != "" {
		count, err := s.Vector.Count(ctx, sourceID)
		if err != nil {
			out.Status = "unavailable"
			out.Errors = append(out.Errors, "source_chroma_count_unavailable: "+err.Error())
		} else {
			out.SourceVectors = count
		}
	}
	if strings.TrimSpace(targetID) != "" {
		count, err := s.Vector.Count(ctx, targetID)
		if err != nil {
			out.Status = "unavailable"
			out.Errors = append(out.Errors, "target_chroma_count_unavailable: "+err.Error())
		} else {
			out.TargetVectors = count
		}
	}
	return out
}
