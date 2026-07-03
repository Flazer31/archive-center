package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type adminReindexDerivedArtifactResult struct {
	CandidatesByTier map[string]int
	ProcessedByTier  map[string]int
	UpsertedByTier   map[string]int
	SkippedByTier    map[string]int
	FailedIDsByTier  map[string][]int64
	SkippedIDsByTier map[string][]int64
	Errors           []string
	Processed        int
	Upserted         int
	Skipped          int
}

func newAdminReindexDerivedArtifactResult() adminReindexDerivedArtifactResult {
	return adminReindexDerivedArtifactResult{
		CandidatesByTier: map[string]int{"evidence": 0, "world_rule": 0},
		ProcessedByTier:  map[string]int{"evidence": 0, "world_rule": 0},
		UpsertedByTier:   map[string]int{"evidence": 0, "world_rule": 0},
		SkippedByTier:    map[string]int{"evidence": 0, "world_rule": 0},
		FailedIDsByTier:  map[string][]int64{"evidence": []int64{}, "world_rule": []int64{}},
		SkippedIDsByTier: map[string][]int64{"evidence": []int64{}, "world_rule": []int64{}},
	}
}

func (r adminReindexDerivedArtifactResult) Summary() map[string]any {
	return map[string]any{
		"candidates_by_tier":  r.CandidatesByTier,
		"processed_by_tier":   r.ProcessedByTier,
		"upserted_by_tier":    r.UpsertedByTier,
		"skipped_by_tier":     r.SkippedByTier,
		"failed_ids_by_tier":  r.FailedIDsByTier,
		"skipped_ids_by_tier": r.SkippedIDsByTier,
		"processed":           r.Processed,
		"upserted":            r.Upserted,
		"skipped":             r.Skipped,
		"errors":              r.Errors,
		"policy":              "existing direct_evidence/world_rules rows are reindexed only when embedding settings are available",
	}
}

func (s *Server) adminReindexDerivedArtifacts(ctx context.Context, sid string, cfg completeTurnExtractionConfig, dryRun bool, maxItems int, evidence []store.DirectEvidence, worldRules []store.WorldRule) adminReindexDerivedArtifactResult {
	result := newAdminReindexDerivedArtifactResult()
	evidenceCandidates := []store.DirectEvidence{}
	for _, item := range evidence {
		if adminEvidenceVectorEligible(item) {
			evidenceCandidates = append(evidenceCandidates, item)
		}
	}
	worldRuleCandidates := []store.WorldRule{}
	for _, item := range worldRules {
		if adminWorldRuleVectorEligible(item) {
			worldRuleCandidates = append(worldRuleCandidates, item)
		}
	}
	result.CandidatesByTier["evidence"] = len(evidenceCandidates)
	result.CandidatesByTier["world_rule"] = len(worldRuleCandidates)
	if maxItems > 0 {
		if len(evidenceCandidates) > maxItems {
			evidenceCandidates = evidenceCandidates[:maxItems]
		}
		if len(worldRuleCandidates) > maxItems {
			worldRuleCandidates = worldRuleCandidates[:maxItems]
		}
	}
	if dryRun {
		return result
	}
	for _, item := range evidenceCandidates {
		result.Processed++
		result.ProcessedByTier["evidence"]++
		saveResult := artifactSaveResult{VectorStatus: "not_requested"}
		s.upsertDerivedArtifactVector(ctx, sid, maxInt(item.TurnAnchor, item.SourceTurnEnd), "evidence", "direct_evidence_records", item.ID, "direct_evidence.v1", directEvidenceVectorDocumentText(item), cfg.Embedder, &saveResult)
		if saveResult.VectorsEvidenceUpserted > 0 {
			result.Upserted += saveResult.VectorsEvidenceUpserted
			result.UpsertedByTier["evidence"] += saveResult.VectorsEvidenceUpserted
			continue
		}
		result.Skipped++
		result.SkippedByTier["evidence"]++
		if saveResult.VectorStatus != "" && saveResult.VectorStatus != "not_requested" && saveResult.VectorStatus != "ok" {
			result.Errors = append(result.Errors, fmt.Sprintf("evidence:%d vector: %s", item.ID, saveResult.VectorStatus))
			result.FailedIDsByTier["evidence"] = append(result.FailedIDsByTier["evidence"], item.ID)
		} else {
			result.SkippedIDsByTier["evidence"] = append(result.SkippedIDsByTier["evidence"], item.ID)
		}
	}
	for _, item := range worldRuleCandidates {
		result.Processed++
		result.ProcessedByTier["world_rule"]++
		saveResult := artifactSaveResult{VectorStatus: "not_requested"}
		s.upsertDerivedArtifactVector(ctx, sid, item.SourceTurn, "world_rule", "world_rules", item.ID, "world_rule.v1", worldRuleVectorDocumentText(item), cfg.Embedder, &saveResult)
		if saveResult.VectorsWorldRuleUpserted > 0 {
			result.Upserted += saveResult.VectorsWorldRuleUpserted
			result.UpsertedByTier["world_rule"] += saveResult.VectorsWorldRuleUpserted
			continue
		}
		result.Skipped++
		result.SkippedByTier["world_rule"]++
		if saveResult.VectorStatus != "" && saveResult.VectorStatus != "not_requested" && saveResult.VectorStatus != "ok" {
			result.Errors = append(result.Errors, fmt.Sprintf("world_rule:%d vector: %s", item.ID, saveResult.VectorStatus))
			result.FailedIDsByTier["world_rule"] = append(result.FailedIDsByTier["world_rule"], item.ID)
		} else {
			result.SkippedIDsByTier["world_rule"] = append(result.SkippedIDsByTier["world_rule"], item.ID)
		}
	}
	return result
}

func adminEvidenceVectorEligible(item store.DirectEvidence) bool {
	return item.ID > 0 &&
		!item.Tombstoned &&
		!item.RepairNeeded &&
		item.SupersededByID <= 0 &&
		strings.TrimSpace(item.EvidenceText) != ""
}

func adminWorldRuleVectorEligible(item store.WorldRule) bool {
	return item.ID > 0 &&
		!item.Suppressed &&
		strings.TrimSpace(worldRuleVectorDocumentText(item)) != ""
}

func derivedArtifactVectorDocumentID(tier, sid string, rowID int64) string {
	tier = strings.TrimSpace(tier)
	sid = strings.TrimSpace(sid)
	if tier == "" || sid == "" || rowID <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%s:%d", tier, sid, rowID)
}

func (s *Server) deleteDerivedArtifactVectorDocuments(ctx context.Context, sid, tier string, rowID int64) map[string]any {
	ids := []string{
		derivedArtifactVectorDocumentID(tier, sid, rowID),
		rollbackVectorDocumentLegacyAlias(tier, rowID),
	}
	return s.deleteVectorDocumentsBestEffort(ctx, ids)
}

func (s *Server) deleteVectorDocumentsBestEffort(ctx context.Context, ids []string) map[string]any {
	clean := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	cleanup := map[string]any{
		"attempted":    false,
		"ok":           true,
		"document_ids": clean,
		"deleted_ids":  0,
	}
	if len(clean) == 0 {
		cleanup["skipped_reason"] = "missing_vector_document_id"
		return cleanup
	}
	if s.Vector == nil {
		cleanup["skipped_reason"] = "vector_store_not_configured"
		return cleanup
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		cleanup["skipped_reason"] = "chromadb_endpoint_not_configured"
		return cleanup
	}
	deleter, ok := s.Vector.(vector.DocumentDeleter)
	if !ok {
		cleanup["ok"] = false
		cleanup["skipped_reason"] = "vector_store_does_not_support_document_delete"
		return cleanup
	}
	cleanup["attempted"] = true
	if err := deleter.DeleteDocuments(ctx, clean); err != nil {
		if errors.Is(err, vector.ErrNotEnabled) {
			cleanup["warning"] = "vector_store_not_enabled"
			return cleanup
		}
		cleanup["ok"] = false
		cleanup["error"] = err.Error()
		return cleanup
	}
	cleanup["deleted_ids"] = len(clean)
	return cleanup
}

func (s *Server) handleAdminVectorOrphanAudit(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAdminAuditBody(w, r)
	if !ok {
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(req["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	deleteOrphans := completeTurnBoolFromAny(req["delete_orphans"])
	if deleteOrphans && !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/vector-orphan-audit")
		return
	}
	report := s.adminVectorOrphanAudit(r.Context(), sid, deleteOrphans)
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_vector_orphan_audit",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin vector orphan audit requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"delete_orphans": deleteOrphans,
			"report":         report,
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	report["audit_written"] = true
	report["changed_at"] = now
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) adminVectorOrphanAudit(ctx context.Context, sid string, deleteOrphans bool) map[string]any {
	report := map[string]any{
		"status":                 "unavailable",
		"chat_session_id":        sid,
		"full_listing_available": false,
		"delete_orphans":         deleteOrphans,
	}
	if s.Vector == nil {
		report["reason"] = "vector_store_not_configured"
		return report
	}
	lister, ok := s.Vector.(vector.DocumentLister)
	if !ok {
		report["reason"] = "vector_store_does_not_support_document_listing"
		return report
	}
	canonicalIDs, canonicalPairs, canonicalCounts, err := s.adminCanonicalVectorReferences(ctx, sid)
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	docs, err := lister.ListDocuments(ctx, sid)
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	managed := 0
	orphans := []string{}
	ignored := []string{}
	okDocs := []string{}
	for _, doc := range docs {
		tier := adminManagedVectorTier(doc)
		if tier == "" {
			ignored = append(ignored, strings.TrimSpace(doc.ID))
			continue
		}
		managed++
		sourcePair := strings.TrimSpace(doc.SourceTable) + ":" + strings.TrimSpace(doc.SourceRowID)
		if canonicalIDs[strings.TrimSpace(doc.ID)] || canonicalPairs[sourcePair] {
			okDocs = append(okDocs, strings.TrimSpace(doc.ID))
			continue
		}
		orphans = append(orphans, strings.TrimSpace(doc.ID))
	}
	deleted := 0
	deleteStatus := "not_requested"
	if deleteOrphans {
		deleteStatus = "skipped"
		if len(orphans) == 0 {
			deleteStatus = "no_orphans"
		} else if deleter, ok := s.Vector.(vector.DocumentDeleter); ok {
			if err := deleter.DeleteDocuments(ctx, orphans); err != nil {
				deleteStatus = "error"
				report["delete_error"] = err.Error()
			} else {
				deleted = len(orphans)
				deleteStatus = "ok"
			}
		} else {
			report["delete_warning"] = "vector_store_does_not_support_document_delete"
		}
	}
	report["status"] = "ok"
	report["full_listing_available"] = true
	report["vector_document_count"] = len(docs)
	report["managed_document_count"] = managed
	report["canonical_counts"] = canonicalCounts
	report["canonical_document_reference_count"] = len(canonicalIDs)
	report["ok_document_count"] = len(okDocs)
	report["orphan_count"] = len(orphans)
	report["orphan_ids"] = orphans
	report["ignored_unmanaged_count"] = len(ignored)
	report["ignored_unmanaged_ids"] = ignored
	report["deleted_orphan_count"] = deleted
	report["delete_status"] = deleteStatus
	report["policy"] = "full ChromaDB session document list is compared with MariaDB canonical row references"
	return report
}

func (s *Server) adminCanonicalVectorReferences(ctx context.Context, sid string) (map[string]bool, map[string]bool, map[string]int, error) {
	ids := map[string]bool{}
	pairs := map[string]bool{}
	counts := map[string]int{
		"memories":                0,
		"direct_evidence_records": 0,
		"world_rules":             0,
		"kg_triples":              0,
		"episode_summaries":       0,
		"chapter_summaries":       0,
		"arc_summaries":           0,
		"saga_digests":            0,
	}
	add := func(table, tier string, rowID int64, docIDs ...string) {
		if rowID <= 0 {
			return
		}
		counts[table]++
		pairs[table+":"+strconv.FormatInt(rowID, 10)] = true
		for _, id := range docIDs {
			if id = strings.TrimSpace(id); id != "" {
				ids[id] = true
			}
		}
		ids[rollbackVectorDocumentAlias(tier, sid, rowID)] = true
		ids[rollbackVectorDocumentLegacyAlias(tier, rowID)] = true
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		return nil, nil, nil, err
	}
	for _, item := range memories {
		add("memories", "memory", item.ID, memoryVectorDocumentID(sid, item))
	}
	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		return nil, nil, nil, err
	}
	for _, item := range evidence {
		if adminEvidenceVectorEligible(item) {
			add("direct_evidence_records", "evidence", item.ID, derivedArtifactVectorDocumentID("evidence", sid, item.ID))
		}
	}
	worldRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		return nil, nil, nil, err
	}
	for _, item := range worldRules {
		if adminWorldRuleVectorEligible(item) {
			add("world_rules", "world_rule", item.ID, derivedArtifactVectorDocumentID("world_rule", sid, item.ID))
		}
	}
	kg, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		return nil, nil, nil, err
	}
	for _, item := range kg {
		add("kg_triples", "kg_triple", item.ID)
	}
	episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		return nil, nil, nil, err
	}
	for _, item := range episodes {
		add("episode_summaries", "episode", item.ID)
	}
	if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
		chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, nil, nil, err
		}
		for _, item := range chapters {
			add("chapter_summaries", "chapter", item.ID)
		}
	}
	if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
		arcs, err := arcStore.ListArcSummaries(ctx, sid, "", 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, nil, nil, err
		}
		for _, item := range arcs {
			add("arc_summaries", "arc", item.ID)
		}
	}
	if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
		sagas, err := sagaStore.ListSagaDigests(ctx, sid, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, nil, nil, err
		}
		for _, item := range sagas {
			add("saga_digests", "saga", item.ID)
		}
	}
	return ids, pairs, counts, nil
}

func adminManagedVectorTier(doc vector.VectorDocument) string {
	tier := strings.ToLower(strings.TrimSpace(doc.Tier))
	table := strings.ToLower(strings.TrimSpace(doc.SourceTable))
	id := strings.ToLower(strings.TrimSpace(doc.ID))
	switch {
	case tier == "memory" || table == "memories" || strings.HasPrefix(id, "memory:"):
		return "memory"
	case tier == "evidence" || table == "direct_evidence_records" || strings.HasPrefix(id, "evidence:"):
		return "evidence"
	case tier == "world_rule" || table == "world_rules" || strings.HasPrefix(id, "world_rule:"):
		return "world_rule"
	case tier == "kg_triple" || table == "kg_triples" || strings.HasPrefix(id, "kg_triple:"):
		return "kg_triple"
	case tier == "episode" || table == "episode_summaries" || strings.HasPrefix(id, "episode:"):
		return "episode"
	case tier == "chapter" || table == "chapter_summaries" || strings.HasPrefix(id, "chapter:"):
		return "chapter"
	case tier == "arc" || table == "arc_summaries" || strings.HasPrefix(id, "arc:"):
		return "arc"
	case tier == "saga" || table == "saga_digests" || strings.HasPrefix(id, "saga:"):
		return "saga"
	default:
		return ""
	}
}

func (s *Server) handleAdminDedupeCleanup(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAdminAuditBody(w, r)
	if !ok {
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(req["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	apply := completeTurnBoolFromAny(req["apply"])
	if apply && !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/dedupe-cleanup")
		return
	}
	report := s.adminDedupeCleanup(r.Context(), sid, apply)
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_dedupe_cleanup",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin dedupe cleanup requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"apply":  apply,
			"report": report,
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	report["audit_written"] = true
	report["changed_at"] = now
	writeJSON(w, http.StatusOK, report)
}

type adminDedupeCandidate struct {
	ID     int64  `json:"id"`
	KeepID int64  `json:"keep_id"`
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

func (s *Server) adminDedupeCleanup(ctx context.Context, sid string, apply bool) map[string]any {
	report := map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"dry_run":         !apply,
		"apply":           apply,
		"policy":          "DB duplicate cleanup is explicit-apply only; default response is audit/dry-run",
	}
	memoryCandidates, memoryErr := s.adminMemoryDuplicateCandidates(ctx, sid)
	storylineCandidates, storylineErr := s.adminStorylineDuplicateCandidates(ctx, sid)
	worldRuleCandidates, worldRuleErr := s.adminWorldRuleDuplicateCandidates(ctx, sid)
	errorsOut := []string{}
	if memoryErr != nil {
		errorsOut = append(errorsOut, "memories: "+memoryErr.Error())
	}
	if storylineErr != nil {
		errorsOut = append(errorsOut, "storylines: "+storylineErr.Error())
	}
	if worldRuleErr != nil {
		errorsOut = append(errorsOut, "world_rules: "+worldRuleErr.Error())
	}
	deleted := map[string]int{"memories": 0, "storylines": 0, "world_rules": 0}
	vectorCleanup := []map[string]any{}
	if apply {
		if mutationStore, ok := s.Store.(store.ExplorerMutationStore); ok {
			for _, cand := range memoryCandidates {
				if err := mutationStore.DeleteMemoryByID(ctx, sid, cand.ID); err != nil {
					errorsOut = append(errorsOut, fmt.Sprintf("memory:%d delete: %s", cand.ID, err.Error()))
					continue
				}
				deleted["memories"]++
				vectorCleanup = append(vectorCleanup, s.deleteMemoryVectorDocument(ctx, sid, store.Memory{ID: cand.ID, ChatSessionID: sid}))
			}
		} else if len(memoryCandidates) > 0 {
			errorsOut = append(errorsOut, "memories: store does not support memory delete")
		}
		if storyDeleter, ok := s.Store.(interface {
			DeleteStoryline(context.Context, int64) error
		}); ok {
			for _, cand := range storylineCandidates {
				if err := storyDeleter.DeleteStoryline(ctx, cand.ID); err != nil {
					errorsOut = append(errorsOut, fmt.Sprintf("storyline:%d delete: %s", cand.ID, err.Error()))
					continue
				}
				deleted["storylines"]++
			}
		} else if len(storylineCandidates) > 0 {
			errorsOut = append(errorsOut, "storylines: store does not support storyline delete")
		}
		if worldDeleter, ok := s.Store.(interface {
			DeleteWorldRule(context.Context, int64) error
		}); ok {
			for _, cand := range worldRuleCandidates {
				if err := worldDeleter.DeleteWorldRule(ctx, cand.ID); err != nil {
					errorsOut = append(errorsOut, fmt.Sprintf("world_rule:%d delete: %s", cand.ID, err.Error()))
					continue
				}
				deleted["world_rules"]++
				vectorCleanup = append(vectorCleanup, s.deleteDerivedArtifactVectorDocuments(ctx, sid, "world_rule", cand.ID))
			}
		} else if len(worldRuleCandidates) > 0 {
			errorsOut = append(errorsOut, "world_rules: store does not support world rule delete")
		}
	}
	report["candidate_counts"] = map[string]int{
		"memories":    len(memoryCandidates),
		"storylines":  len(storylineCandidates),
		"world_rules": len(worldRuleCandidates),
	}
	report["delete_candidates"] = map[string]any{
		"memories":    memoryCandidates,
		"storylines":  storylineCandidates,
		"world_rules": worldRuleCandidates,
	}
	report["deleted_counts"] = deleted
	report["vector_cleanup"] = vectorCleanup
	report["errors"] = errorsOut
	if len(errorsOut) > 0 {
		report["status"] = "partial_error"
	}
	return report
}

func (s *Server) adminMemoryDuplicateCandidates(ctx context.Context, sid string) ([]adminDedupeCandidate, error) {
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			return nil, nil
		}
		return nil, err
	}
	groups := map[string][]store.Memory{}
	for _, item := range memories {
		key := collapseTextKey(prepareTurnMemorySummary(item))
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], item)
	}
	out := []adminDedupeCandidate{}
	for key, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Importance != group[j].Importance {
				return group[i].Importance > group[j].Importance
			}
			if group[i].TurnIndex != group[j].TurnIndex {
				return group[i].TurnIndex > group[j].TurnIndex
			}
			return group[i].ID > group[j].ID
		})
		keepID := group[0].ID
		for _, item := range group[1:] {
			out = append(out, adminDedupeCandidate{ID: item.ID, KeepID: keepID, Key: key, Reason: "same_normalized_memory_summary"})
		}
	}
	return out, nil
}

func (s *Server) adminStorylineDuplicateCandidates(ctx context.Context, sid string) ([]adminDedupeCandidate, error) {
	items, err := s.Store.ListStorylines(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			return nil, nil
		}
		return nil, err
	}
	groups := map[string][]store.Storyline{}
	for _, item := range items {
		key := collapseTextKey(extractionFirstNonEmpty(item.Name, item.CurrentContext))
		detailKey := collapseTextKey(item.CurrentContext)
		if key == "" {
			key = detailKey
		}
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], item)
	}
	out := []adminDedupeCandidate{}
	for key, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Pinned != group[j].Pinned {
				return group[i].Pinned
			}
			if group[i].UserCorrected != group[j].UserCorrected {
				return group[i].UserCorrected
			}
			if group[i].Suppressed != group[j].Suppressed {
				return !group[i].Suppressed
			}
			if group[i].LastTurn != group[j].LastTurn {
				return group[i].LastTurn > group[j].LastTurn
			}
			if group[i].EvidenceCount != group[j].EvidenceCount {
				return group[i].EvidenceCount > group[j].EvidenceCount
			}
			return group[i].ID > group[j].ID
		})
		keepID := group[0].ID
		for _, item := range group[1:] {
			out = append(out, adminDedupeCandidate{ID: item.ID, KeepID: keepID, Key: key, Reason: "same_normalized_storyline_anchor"})
		}
	}
	return out, nil
}

func (s *Server) adminWorldRuleDuplicateCandidates(ctx context.Context, sid string) ([]adminDedupeCandidate, error) {
	items, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			return nil, nil
		}
		return nil, err
	}
	groups := map[string][]store.WorldRule{}
	for _, item := range items {
		key := strings.Join([]string{
			collapseTextKey(item.Scope),
			collapseTextKey(item.ScopeName),
			collapseTextKey(item.Category),
			collapseTextKey(item.Key),
			collapseTextKey(item.ValueJSON),
		}, "|")
		if strings.Trim(key, "|") == "" {
			continue
		}
		groups[key] = append(groups[key], item)
	}
	out := []adminDedupeCandidate{}
	for key, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Pinned != group[j].Pinned {
				return group[i].Pinned
			}
			if group[i].UserCorrected != group[j].UserCorrected {
				return group[i].UserCorrected
			}
			if group[i].Suppressed != group[j].Suppressed {
				return !group[i].Suppressed
			}
			if group[i].SourceTurn != group[j].SourceTurn {
				return group[i].SourceTurn > group[j].SourceTurn
			}
			return group[i].ID > group[j].ID
		})
		keepID := group[0].ID
		for _, item := range group[1:] {
			out = append(out, adminDedupeCandidate{ID: item.ID, KeepID: keepID, Key: key, Reason: "same_normalized_world_rule"})
		}
	}
	return out, nil
}
