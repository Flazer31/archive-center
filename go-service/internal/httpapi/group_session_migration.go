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
	sessionMigrationCompleteVersion = "sc-mig-complete.v2"
	sessionMigrationReindexVersion  = "sc-mig-reindex.v2"
	sessionMigrationLockVersion     = "sc-mig-lock.v1"
	sessionMigrationRollbackVersion = "sc-mig-rollback.v1"
	sessionMigrationCleanupVersion  = "sc-mig-cleanup.v2"
	sessionMigrationModeCopyLock    = store.SessionMigrationModeCopyThenLockSource
	sessionMigrationModeCopyKeep    = store.SessionMigrationModeCopyKeepSource
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
	ReferenceBindings           int  `json:"reference_bindings"`
	ReferenceRuntimes           int  `json:"reference_runtimes"`
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
	Status                   string                        `json:"status"`
	ContractVersion          string                        `json:"contract_version"`
	WriteAttempted           bool                          `json:"write_attempted"`
	VectorWriteAttempted     bool                          `json:"vector_write_attempted"`
	LLMCallAttempted         bool                          `json:"llm_call_attempted"`
	MigrationID              int64                         `json:"migration_id"`
	MigrationStatus          string                        `json:"migration_status"`
	SourceSessionID          string                        `json:"source_session_id"`
	TargetSessionID          string                        `json:"target_session_id"`
	Mode                     string                        `json:"mode"`
	Counts                   sessionMigrationPreviewCounts `json:"counts"`
	RowMapCount              int                           `json:"row_map_count"`
	SourceLocked             bool                          `json:"source_locked"`
	ChromaReindexRequired    bool                          `json:"chroma_reindex_required"`
	ReadyForLive             bool                          `json:"ready_for_live"`
	TargetStarterReplaced    bool                          `json:"target_starter_replaced"`
	ManifestVersion          string                        `json:"manifest_version"`
	ManifestDirectTables     int                           `json:"manifest_direct_tables"`
	ManifestIndirectTables   int                           `json:"manifest_indirect_tables"`
	ManifestExecutorComplete bool                          `json:"manifest_executor_complete"`
	ManifestParityVerified   bool                          `json:"manifest_parity_verified"`
	ReleaseBlocked           bool                          `json:"release_blocked"`
	ReleaseBlockers          []string                      `json:"release_blockers"`
	Blocked                  bool                          `json:"blocked"`
	BlockedReasons           []string                      `json:"blocked_reasons"`
	Warnings                 []string                      `json:"warnings"`
	GeneratedAt              string                        `json:"generated_at"`
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
	manifestDirect, manifestIndirect, manifestImplemented := store.SessionMigrationManifestSummary()
	manifestTotal := manifestDirect + manifestIndirect
	manifestExecutorComplete := manifestImplemented == manifestTotal
	manifestBlockers := store.SessionMigrationManifestReleaseBlockers()
	migrationRequest := store.SessionMigrationCompleteRequest{
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Mode:            mode,
		OperatorNote:    strings.TrimSpace(req.OperatorNote),
	}
	migrationStore, migrationStoreAvailable := s.Store.(store.SessionMigrationStore)
	var resumeContext *store.SessionMigrationResumeContext
	if migrationStoreAvailable {
		var resumeErr error
		resumeContext, resumeErr = migrationStore.GetSessionMigrationResumeContext(r.Context(), migrationRequest)
		if errors.Is(resumeErr, store.ErrNotFound) {
			resumeContext = nil
			resumeErr = nil
		}
		if resumeErr != nil {
			writeInternalError(w, resumeErr.Error())
			return
		}
	}

	blockedReasons, warnings, sourceCounts, targetCounts, chroma, err := s.sessionMigrationValidate(r.Context(), sourceID, targetID, mode)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if resumeContext != nil {
		filtered := blockedReasons[:0]
		for _, reason := range blockedReasons {
			if reason == "target_session_not_empty" || reason == "target_chroma_vectors_not_empty" {
				continue
			}
			if reason == "source_session_has_no_archive_data" && resumeContext.Status == "source_cleaned" {
				continue
			}
			filtered = append(filtered, reason)
		}
		blockedReasons = filtered
	}
	if len(blockedReasons) > 0 {
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:                   "ok",
			ContractVersion:          sessionMigrationCompleteVersion,
			WriteAttempted:           false,
			VectorWriteAttempted:     false,
			LLMCallAttempted:         false,
			SourceSessionID:          sourceID,
			TargetSessionID:          targetID,
			Mode:                     mode,
			Counts:                   sourceCounts,
			ManifestVersion:          store.SessionMigrationManifestVersion,
			ManifestDirectTables:     manifestDirect,
			ManifestIndirectTables:   manifestIndirect,
			ManifestExecutorComplete: manifestExecutorComplete,
			ManifestParityVerified:   false,
			ReleaseBlocked:           true,
			ReleaseBlockers:          manifestBlockers,
			Blocked:                  true,
			BlockedReasons:           blockedReasons,
			Warnings:                 warnings,
			GeneratedAt:              time.Now().UTC().Format(time.RFC3339),
		})
		_ = targetCounts
		_ = chroma
		return
	}

	if !manifestExecutorComplete || len(manifestBlockers) > 0 {
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:                   "ok",
			ContractVersion:          sessionMigrationCompleteVersion,
			WriteAttempted:           false,
			VectorWriteAttempted:     false,
			LLMCallAttempted:         false,
			SourceSessionID:          sourceID,
			TargetSessionID:          targetID,
			Mode:                     mode,
			Counts:                   sourceCounts,
			ManifestVersion:          store.SessionMigrationManifestVersion,
			ManifestDirectTables:     manifestDirect,
			ManifestIndirectTables:   manifestIndirect,
			ManifestExecutorComplete: manifestExecutorComplete,
			ManifestParityVerified:   false,
			ReleaseBlocked:           true,
			ReleaseBlockers:          manifestBlockers,
			Blocked:                  true,
			BlockedReasons:           []string{"session_migration_manifest_executor_incomplete"},
			Warnings:                 warnings,
			GeneratedAt:              time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if !migrationStoreAvailable {
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:                   "ok",
			ContractVersion:          sessionMigrationCompleteVersion,
			WriteAttempted:           false,
			VectorWriteAttempted:     false,
			LLMCallAttempted:         false,
			SourceSessionID:          sourceID,
			TargetSessionID:          targetID,
			Mode:                     mode,
			Counts:                   sourceCounts,
			ManifestVersion:          store.SessionMigrationManifestVersion,
			ManifestDirectTables:     manifestDirect,
			ManifestIndirectTables:   manifestIndirect,
			ManifestExecutorComplete: manifestExecutorComplete,
			ManifestParityVerified:   false,
			ReleaseBlocked:           true,
			ReleaseBlockers:          manifestBlockers,
			Blocked:                  true,
			BlockedReasons:           []string{"session_migration_store_unavailable"},
			Warnings:                 append(warnings, "complete migration requires MariaDB authority store"),
			GeneratedAt:              time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	writeResumeBlocked := func(reasons []string) {
		migrationID := int64(0)
		migrationStatus := ""
		if resumeContext != nil {
			migrationID = resumeContext.MigrationID
			migrationStatus = resumeContext.Status
		}
		writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
			Status:                   "ok",
			ContractVersion:          sessionMigrationCompleteVersion,
			WriteAttempted:           false,
			VectorWriteAttempted:     false,
			LLMCallAttempted:         false,
			MigrationID:              migrationID,
			MigrationStatus:          migrationStatus,
			SourceSessionID:          sourceID,
			TargetSessionID:          targetID,
			Mode:                     mode,
			Counts:                   sourceCounts,
			ManifestVersion:          store.SessionMigrationManifestVersion,
			ManifestDirectTables:     manifestDirect,
			ManifestIndirectTables:   manifestIndirect,
			ManifestExecutorComplete: manifestExecutorComplete,
			ManifestParityVerified:   false,
			ReleaseBlocked:           true,
			ReleaseBlockers:          manifestBlockers,
			Blocked:                  true,
			BlockedReasons:           reasons,
			Warnings:                 warnings,
			GeneratedAt:              time.Now().UTC().Format(time.RFC3339),
		})
	}
	var result *store.SessionMigrationCompleteResult
	if resumeContext != nil && resumeContext.Status != "copied" {
		err = s.sessionMigrationWithExclusiveVectorFence(r.Context(), func(rawVector vector.VectorStore) error {
			parity, verifyErr := s.sessionMigrationRevalidateCurrentTargetWithVector(
				r.Context(), resumeContext.MigrationID, store.SessionMigrationProofOperationResume, rawVector,
			)
			if verifyErr != nil {
				return verifyErr
			}
			if parity == nil || !parity.Verified {
				return &store.SessionMigrationBlockerError{
					Code: "current_vector_id_drift", Phase: "resume",
				}
			}
			var completeErr error
			result, completeErr = migrationStore.CompleteSessionMigration(r.Context(), migrationRequest)
			return completeErr
		})
	} else {
		result, err = migrationStore.CompleteSessionMigration(r.Context(), migrationRequest)
	}
	if err != nil {
		var blocker *store.SessionMigrationBlockerError
		if errors.As(err, &blocker) {
			writeResumeBlocked([]string{blocker.Code})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	responseWarnings := append([]string{}, warnings...)
	if result.Status == "copied" {
		responseWarnings = append(
			responseWarnings,
			"copy_phase_only: target is not live-complete",
			store.SessionMigrationManifestParityUnverifiedReason,
			"chroma_reindex_pending: vector parity is necessary but not sufficient for source lock",
		)
	} else {
		responseWarnings = append(responseWarnings, "resume_current_state_revalidated")
	}

	writeJSON(w, http.StatusOK, sessionMigrationCompleteResponse{
		Status:                   "ok",
		ContractVersion:          sessionMigrationCompleteVersion,
		WriteAttempted:           true,
		VectorWriteAttempted:     false,
		LLMCallAttempted:         false,
		MigrationID:              result.MigrationID,
		MigrationStatus:          result.Status,
		SourceSessionID:          result.SourceSessionID,
		TargetSessionID:          result.TargetSessionID,
		Mode:                     result.Mode,
		Counts:                   sessionMigrationCountsFromStore(result.Counts),
		RowMapCount:              result.RowMapCount,
		SourceLocked:             result.SourceLocked,
		ChromaReindexRequired:    result.ChromaReindexRequired,
		ReadyForLive:             result.ReadyForLive,
		TargetStarterReplaced:    result.TargetStarterReplaced,
		ManifestVersion:          store.SessionMigrationManifestVersion,
		ManifestDirectTables:     manifestDirect,
		ManifestIndirectTables:   manifestIndirect,
		ManifestExecutorComplete: manifestExecutorComplete,
		ManifestParityVerified:   false,
		ReleaseBlocked:           true,
		ReleaseBlockers:          manifestBlockers,
		Blocked:                  false,
		BlockedReasons:           []string{},
		Warnings:                 responseWarnings,
		GeneratedAt:              time.Now().UTC().Format(time.RFC3339),
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
	EmbeddingCallAttempted  bool     `json:"embedding_call_attempted"`
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
	ManifestVersion         string   `json:"manifest_version"`
	ManifestParityVerified  bool     `json:"manifest_parity_verified"`
	ExpectedVectorIDs       int      `json:"expected_vector_ids"`
	ActualVectorIDs         int      `json:"actual_vector_ids"`
	MissingVectorIDs        []string `json:"missing_vector_ids"`
	UnexpectedVectorIDs     []string `json:"unexpected_vector_ids"`
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
		Status:                 "ok",
		ContractVersion:        sessionMigrationReindexVersion,
		MigrationID:            req.MigrationID,
		WriteAttempted:         false,
		VectorWriteAttempted:   false,
		LLMCallAttempted:       false,
		VerificationStatus:     "not_run",
		ManifestVersion:        store.SessionMigrationManifestVersion,
		ManifestParityVerified: false,
		GeneratedAt:            time.Now().UTC().Format(time.RFC3339),
	}

	manifestDirect, manifestIndirect, manifestImplemented := store.SessionMigrationManifestSummary()
	if manifestImplemented != manifestDirect+manifestIndirect {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_manifest_executor_incomplete")
		resp.Warnings = append(resp.Warnings, store.SessionMigrationManifestReleaseBlockers()...)
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
	parityStore, ok := s.Store.(store.SessionMigrationVectorParityStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_vector_parity_store_unavailable")
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
	parityContext, err := parityStore.GetSessionMigrationVectorParityContext(r.Context(), req.MigrationID)
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
	resp.TargetSessionID = parityContext.TargetSessionID
	resp.ExpectedVectorIDs = len(parityContext.ExpectedIDs)

	candidates, err := migrationVectorStore.ListSessionMigrationVectorDocuments(r.Context(), req.MigrationID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp.Candidates = len(candidates)
	expectedSet := make(map[string]bool, len(parityContext.ExpectedIDs))
	for _, id := range parityContext.ExpectedIDs {
		expectedSet[strings.TrimSpace(id)] = true
	}
	candidateSet := make(map[string]bool, len(candidates))
	targetSessionID := strings.TrimSpace(parityContext.TargetSessionID)
	for _, candidate := range candidates {
		candidateID := strings.TrimSpace(candidate.ID)
		if candidateID == "" || candidateSet[candidateID] {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "duplicate_or_empty_vector_candidate_id")
			continue
		}
		candidateSet[candidateID] = true
		if !expectedSet[candidateID] {
			resp.UnexpectedVectorIDs = append(resp.UnexpectedVectorIDs, candidateID)
		}
		if strings.TrimSpace(candidate.ChatSessionID) != targetSessionID {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "vector_candidate_target_session_mismatch")
		}
		if strings.TrimSpace(candidate.DocumentText) == "" {
			resp.Skipped++
			resp.SkippedIDs = append(resp.SkippedIDs, candidate.ID)
			continue
		}
	}
	for _, expectedID := range parityContext.ExpectedIDs {
		if !candidateSet[strings.TrimSpace(expectedID)] {
			resp.MissingVectorIDs = append(resp.MissingVectorIDs, expectedID)
		}
	}
	if resp.Blocked || resp.Skipped > 0 || len(resp.MissingVectorIDs) > 0 || len(resp.UnexpectedVectorIDs) > 0 || len(candidates) != len(parityContext.ExpectedIDs) {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_candidate_expected_id_mismatch")
		resp.VerificationStatus = "candidate_parity_failed"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	embeddingConfig := s.completeTurnExtractionConfig(map[string]any{}).Embedder
	needsEmbedding := usesVoyageContextualizedEmbedding(embeddingConfig)
	for _, candidate := range candidates {
		if len(parseFloat32JSONList(candidate.EmbeddingJSON)) == 0 {
			needsEmbedding = true
			break
		}
	}
	if needsEmbedding && !embeddingConfig.hasConfig() {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "embedding_config_unavailable")
		resp.VerificationStatus = "embedding_preflight_failed"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	contextualizedEmbeddings := map[string][]float32{}
	if usesVoyageContextualizedEmbedding(embeddingConfig) && len(candidates) > 0 {
		inputs := make([]string, 0, len(candidates))
		ids := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			inputs = append(inputs, candidate.DocumentText)
			ids = append(ids, candidate.ID)
		}
		resp.EmbeddingCallAttempted = true
		grouped, _, err := callDocumentEmbeddings(r.Context(), embeddingConfig, inputs)
		if err != nil {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "embedding_failed")
			resp.Errors = append(resp.Errors, err.Error())
			resp.VerificationStatus = "embedding_failed"
			writeJSON(w, http.StatusOK, resp)
			return
		}
		for i, id := range ids {
			contextualizedEmbeddings[id] = parseFloat32JSONList(grouped[i])
			if len(contextualizedEmbeddings[id]) == 0 {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, "embedding_result_invalid")
				resp.VerificationStatus = "embedding_failed"
				writeJSON(w, http.StatusOK, resp)
				return
			}
		}
	}
	docs := make([]vector.VectorDocument, 0, len(candidates))
	for _, candidate := range candidates {
		embedding := contextualizedEmbeddings[candidate.ID]
		if !usesVoyageContextualizedEmbedding(embeddingConfig) {
			embedding = parseFloat32JSONList(candidate.EmbeddingJSON)
		}
		if len(embedding) == 0 && !usesVoyageContextualizedEmbedding(embeddingConfig) {
			resp.EmbeddingCallAttempted = true
			embeddingJSON, _, err := callEmbedding(r.Context(), embeddingConfig, candidate.DocumentText)
			if err != nil {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, "embedding_failed")
				resp.Errors = append(resp.Errors, candidate.ID+": "+err.Error())
				resp.VerificationStatus = "embedding_failed"
				writeJSON(w, http.StatusOK, resp)
				return
			}
			embedding = parseFloat32JSONList(embeddingJSON)
			if len(embedding) == 0 {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, "embedding_result_invalid")
				resp.VerificationStatus = "embedding_failed"
				writeJSON(w, http.StatusOK, resp)
				return
			}
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

	before, err := s.Vector.Count(r.Context(), targetSessionID)
	if err != nil {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "target_chroma_count_unavailable")
		resp.Errors = append(resp.Errors, err.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.TargetVectorCountBefore = before
	if len(docs) > 0 {
		resp.VectorWriteAttempted = true
		if err := s.Vector.Upsert(r.Context(), targetSessionID, docs); err != nil {
			resp.Errors = append(resp.Errors, err.Error())
			resp.VerificationStatus = "upsert_failed"
			_ = migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindex_failed", 0, mustCompactJSON(resp.Errors))
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp.Upserted = len(docs)
	}
	after, err := s.Vector.Count(r.Context(), targetSessionID)
	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		resp.VerificationStatus = "verification_count_failed"
		_ = migrationVectorStore.UpdateSessionMigrationVectorStatus(r.Context(), req.MigrationID, "vector_reindex_unverified", resp.Upserted, mustCompactJSON(resp.Errors))
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.TargetVectorCountAfter = after
	lister, ok := s.Vector.(vector.DocumentLister)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_document_list_unavailable")
		resp.VerificationStatus = "exact_id_readback_unavailable"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	actualDocuments, err := lister.ListDocuments(r.Context(), targetSessionID)
	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		resp.VerificationStatus = "exact_id_readback_failed"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	actualIDs := make([]string, 0, len(actualDocuments))
	for _, document := range actualDocuments {
		actualIDs = append(actualIDs, document.ID)
	}
	parity, err := parityStore.VerifySessionMigrationVectorParity(
		r.Context(), req.MigrationID, store.SessionMigrationProofOperationSourceLock, actualIDs,
	)
	if err != nil {
		var blocker *store.SessionMigrationBlockerError
		if errors.As(err, &blocker) {
			resp.Blocked = true
			resp.VerificationStatus = blocker.Code
			resp.BlockedReasons = append(resp.BlockedReasons, blocker.Code)
			if parity != nil {
				resp.ActualVectorIDs = len(parity.ActualIDs)
				resp.MissingVectorIDs = append([]string(nil), parity.MissingIDs...)
				resp.UnexpectedVectorIDs = append([]string(nil), parity.UnexpectedIDs...)
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.WriteAttempted = true
	resp.ActualVectorIDs = len(parity.ActualIDs)
	resp.MissingVectorIDs = append([]string(nil), parity.MissingIDs...)
	resp.UnexpectedVectorIDs = append([]string(nil), parity.UnexpectedIDs...)
	resp.ManifestParityVerified = parity.Verified
	if parity.Verified {
		resp.VerificationStatus = "exact_id_parity_verified"
		resp.ReadyForSourceLock = true
		resp.ReadyForLive = false
	} else {
		resp.Blocked = true
		resp.VerificationStatus = "exact_id_parity_mismatch"
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_expected_id_parity_mismatch")
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

func (s *Server) sessionMigrationRevalidateCurrentTarget(
	ctx context.Context,
	migrationID int64,
	operation string,
) (*store.SessionMigrationVectorParityResult, error) {
	return s.sessionMigrationRevalidateCurrentTargetWithVector(ctx, migrationID, operation, s.Vector)
}

func (s *Server) sessionMigrationRevalidateCurrentTargetWithVector(
	ctx context.Context,
	migrationID int64,
	operation string,
	vectorStore vector.VectorStore,
) (*store.SessionMigrationVectorParityResult, error) {
	parityStore, ok := s.Store.(store.SessionMigrationVectorParityStore)
	if !ok {
		return nil, &store.SessionMigrationBlockerError{Code: "current_vector_parity_store_unavailable", Phase: "current_revalidation"}
	}
	if vectorStore == nil {
		return nil, &store.SessionMigrationBlockerError{Code: "current_vector_store_unavailable", Phase: "current_revalidation"}
	}
	lister, ok := vectorStore.(vector.DocumentLister)
	if !ok {
		return nil, &store.SessionMigrationBlockerError{Code: "current_vector_document_list_unavailable", Phase: "current_revalidation"}
	}
	parityContext, err := parityStore.GetSessionMigrationVectorParityContext(ctx, migrationID)
	if err != nil {
		return nil, err
	}
	documents, err := lister.ListDocuments(ctx, parityContext.TargetSessionID)
	if err != nil {
		return nil, err
	}
	actualIDs := make([]string, 0, len(documents))
	for _, document := range documents {
		actualIDs = append(actualIDs, document.ID)
	}
	return parityStore.VerifySessionMigrationVectorParity(ctx, migrationID, operation, actualIDs)
}

func (s *Server) sessionMigrationWithExclusiveVectorFence(
	ctx context.Context,
	fn func(vector.VectorStore) error,
) error {
	if s.Vector == nil {
		return &store.SessionMigrationBlockerError{Code: "current_vector_store_unavailable", Phase: "current_revalidation"}
	}
	fencer, ok := s.Vector.(vector.MutationFencer)
	if !ok {
		return &store.SessionMigrationBlockerError{Code: "current_vector_mutation_fence_unavailable", Phase: "current_revalidation"}
	}
	return fencer.WithExclusiveMutationFence(ctx, fn)
}

func sessionMigrationAppendTypedBlocker(blockedReasons *[]string, err error) bool {
	var blocker *store.SessionMigrationBlockerError
	if !errors.As(err, &blocker) {
		return false
	}
	*blockedReasons = append(*blockedReasons, blocker.Code)
	return true
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
	fenceStore, ok := s.Store.(store.SessionMigrationSourceLockFenceStore)
	if !ok {
		resp.Blocked = true
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_source_lock_fence_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	provisional, err := fenceStore.PrepareSessionMigrationSourceLock(
		r.Context(), req.MigrationID, req.Reason,
	)
	if err != nil {
		if sessionMigrationAppendTypedBlocker(&resp.BlockedReasons, err) {
			resp.Blocked = true
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp.WriteAttempted = true
	pendingFence := provisional != nil && provisional.LockStatus == "lock_pending_verification"
	if pendingFence && strings.TrimSpace(provisional.SourceSessionID) != "" {
		s.cancelCompleteTurnSourceWorkers(provisional.SourceSessionID, 1)
	}
	releasePendingFence := func(reason string) {
		if !pendingFence {
			return
		}
		if releaseErr := fenceStore.ReleaseSessionMigrationSourceLockFence(
			r.Context(), req.MigrationID, reason,
		); releaseErr != nil {
			resp.BlockedReasons = append(resp.BlockedReasons, "source_lock_fence_release_failed")
		}
	}
	var result *store.SessionMigrationSourceLockResult
	err = s.sessionMigrationWithExclusiveVectorFence(r.Context(), func(rawVector vector.VectorStore) error {
		parity, verifyErr := s.sessionMigrationRevalidateCurrentTargetWithVector(
			r.Context(), req.MigrationID, store.SessionMigrationProofOperationSourceLock, rawVector,
		)
		if verifyErr != nil {
			return verifyErr
		}
		if parity == nil || !parity.Verified {
			return &store.SessionMigrationBlockerError{
				Code: "current_vector_id_drift", Phase: "source_lock",
			}
		}
		var lockErr error
		result, lockErr = lockStore.LockSessionMigrationSource(r.Context(), req.MigrationID, req.Reason)
		return lockErr
	})
	if err != nil {
		releasePendingFence(err.Error())
		if errors.Is(err, store.ErrNotFound) {
			resp.Blocked = true
			resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
			writeJSON(w, http.StatusOK, resp)
			return
		}
		if sessionMigrationAppendTypedBlocker(&resp.BlockedReasons, err) {
			resp.Blocked = true
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
	CleanupPrepared      bool                          `json:"cleanup_prepared"`
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

type sessionMigrationCleanupPreparer interface {
	PrepareSessionMigrationSourceCleanup(ctx context.Context, migrationID int64, reason string) (*store.SessionMigrationCleanupPreview, error)
	MarkSessionMigrationSourceVectorCleanup(ctx context.Context, migrationID int64) error
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
	if resp.Blocked || resp.DryRun {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.Vector == nil {
		resp.Blocked = true
		resp.ReadyForCleanup = false
		resp.BlockedReasons = append(resp.BlockedReasons, "vector_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		resp.Blocked = true
		resp.ReadyForCleanup = false
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_endpoint_not_configured")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if s.VectorOpenError != nil {
		resp.Blocked = true
		resp.ReadyForCleanup = false
		resp.BlockedReasons = append(resp.BlockedReasons, "chroma_open_error")
		resp.Errors = append(resp.Errors, s.VectorOpenError.Error())
		writeJSON(w, http.StatusOK, resp)
		return
	}
	preparer, ok := recoveryStore.(sessionMigrationCleanupPreparer)
	if !ok {
		resp.Blocked = true
		resp.ReadyForCleanup = false
		resp.BlockedReasons = append(resp.BlockedReasons, "session_migration_cleanup_prepare_store_unavailable")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	fenceErr := s.sessionMigrationWithExclusiveVectorFence(r.Context(), func(rawVector vector.VectorStore) error {
		currentParity, err := s.sessionMigrationRevalidateCurrentTargetWithVector(
			r.Context(), req.MigrationID, store.SessionMigrationProofOperationCleanupPrepare, rawVector,
		)
		if err != nil {
			if sessionMigrationAppendTypedBlocker(&resp.BlockedReasons, err) {
				resp.Blocked = true
				resp.ReadyForCleanup = false
				writeJSON(w, http.StatusOK, resp)
				return nil
			}
			writeInternalError(w, err.Error())
			return nil
		}
		if currentParity == nil || !currentParity.Verified {
			resp.Blocked = true
			resp.ReadyForCleanup = false
			resp.BlockedReasons = append(resp.BlockedReasons, "current_vector_id_drift")
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		prepared, err := preparer.PrepareSessionMigrationSourceCleanup(
			r.Context(), req.MigrationID, strings.TrimSpace(req.Reason),
		)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
				writeJSON(w, http.StatusOK, resp)
				return nil
			}
			writeInternalError(w, err.Error())
			return nil
		}
		resp.MigrationStatus = prepared.Status
		resp.SourceSessionID = prepared.SourceSessionID
		resp.TargetSessionID = prepared.TargetSessionID
		resp.SourceLocked = prepared.SourceLocked
		resp.ReadyForCleanup = prepared.ReadyForCleanup
		resp.SourceRows = sessionMigrationCountsFromStore(prepared.Counts)
		resp.BlockedReasons = append(resp.BlockedReasons, prepared.BlockedReasons...)
		if !prepared.ReadyForCleanup || len(prepared.BlockedReasons) > 0 {
			resp.Blocked = true
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		resp.CleanupPrepared = true
		resp.VectorWriteAttempted = true
		if err := rawVector.DeleteSession(r.Context(), prepared.SourceSessionID); err != nil {
			resp.Errors = append(resp.Errors, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		if err := preparer.MarkSessionMigrationSourceVectorCleanup(r.Context(), req.MigrationID); err != nil {
			resp.Blocked = true
			resp.ReadyForCleanup = false
			resp.BlockedReasons = append(resp.BlockedReasons, "source_vector_cleanup_mark_failed_recovery_required")
			resp.Errors = append(resp.Errors, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		currentParity, err = s.sessionMigrationRevalidateCurrentTargetWithVector(
			r.Context(), req.MigrationID, store.SessionMigrationProofOperationCleanupFinalize, rawVector,
		)
		if err != nil {
			resp.Blocked = true
			resp.ReadyForCleanup = false
			if !sessionMigrationAppendTypedBlocker(&resp.BlockedReasons, err) {
				resp.BlockedReasons = append(resp.BlockedReasons, "current_state_revalidation_failed_recovery_required")
			}
			resp.Errors = append(resp.Errors, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		if currentParity == nil || !currentParity.Verified {
			resp.Blocked = true
			resp.ReadyForCleanup = false
			resp.BlockedReasons = append(resp.BlockedReasons, "current_vector_id_drift")
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		resp.WriteAttempted = true
		result, err := recoveryStore.CleanupSessionMigrationSource(r.Context(), req.MigrationID, strings.TrimSpace(req.Reason))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, "migration_not_found")
				writeJSON(w, http.StatusOK, resp)
				return nil
			}
			if strings.Contains(err.Error(), "blocked:") {
				resp.Blocked = true
				resp.BlockedReasons = append(resp.BlockedReasons, err.Error())
				writeJSON(w, http.StatusOK, resp)
				return nil
			}
			resp.Blocked = true
			resp.ReadyForCleanup = false
			resp.BlockedReasons = append(resp.BlockedReasons, "source_cleanup_finalize_failed_recovery_required")
			resp.Errors = append(resp.Errors, err.Error())
			writeJSON(w, http.StatusOK, resp)
			return nil
		}
		resp.MigrationStatus = result.Status
		resp.SourceSessionID = result.SourceSessionID
		resp.TargetSessionID = result.TargetSessionID
		resp.SourceRows = sessionMigrationCountsFromStore(result.Counts)
		resp.SourceCleaned = result.SourceCleaned
		resp.ReadyForLive = result.ReadyForLive
		writeJSON(w, http.StatusOK, resp)
		return nil
	})
	if fenceErr != nil {
		resp.Blocked = true
		resp.ReadyForCleanup = false
		if !sessionMigrationAppendTypedBlocker(&resp.BlockedReasons, fenceErr) {
			resp.BlockedReasons = append(resp.BlockedReasons, "current_vector_mutation_fence_failed")
			resp.Errors = append(resp.Errors, fenceErr.Error())
		}
		writeJSON(w, http.StatusOK, resp)
	}
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
		})
		if err != nil {
			return counts, warnings, sessionMigrationReadError("subjective_entity_memories", err)
		}
		counts.SubjectiveEntityMemories = len(subjective)
	}
	if referenceStore, ok := s.Store.(store.ReferenceLibraryStore); ok {
		bindings, err := referenceStore.ListSessionReferenceBindings(ctx, sessionID, false)
		if err != nil {
			return counts, warnings, sessionMigrationReadError("session_reference_bindings", err)
		}
		counts.ReferenceBindings = len(bindings)
		for _, binding := range bindings {
			if _, err := referenceStore.GetSessionReferenceRuntime(ctx, binding.BindingID); err == nil {
				counts.ReferenceRuntimes++
			} else if !errors.Is(err, store.ErrNotFound) {
				return counts, warnings, sessionMigrationReadError("session_reference_runtime", err)
			}
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
	if targetID != "" && ((targetCounts.CanonicalAndSubjectiveTotal > 0 && !targetCounts.ReplaceableStarterOnly) || targetCounts.ReferenceBindings > 0 || chroma.TargetVectors > 0) {
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
		ReferenceBindings:           in.ReferenceBindings,
		ReferenceRuntimes:           in.ReferenceRuntimes,
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
		ids := []string{strings.TrimSpace(doc.ID)}
		tier := strings.TrimSpace(doc.Tier)
		sourceRowID := strings.TrimSpace(doc.SourceRowID)
		if tier != "" && sourceRowID != "" {
			ids = append(ids, tier+":"+sourceRowID)
		}
		for _, id := range ids {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
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
