package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	sourceDiscoveryMaxBytes           = 2 << 20
	sourceDiscoveryOperationTimeout   = 9 * time.Minute
	ollamaSourceSearchMaxRounds       = 3
	ollamaSourceSearchMaxToolCalls    = 3
	ollamaSourceSearchToolResultRunes = 12000
	SourceDiscoveryUserAgent          = "ArchiveCenter-SourceDiscovery/1.0"
	sourceCandidateExtractionContract = "source-candidate-extraction.v3"
)

func (s *Server) registerSourceDiscoveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /source-discovery/preview/v1", s.handleSourceDiscoveryPreviewV1)
	mux.HandleFunc("POST /source-discovery/jobs/v1", s.handleSourceDiscoveryCreateV1)
	mux.HandleFunc("GET /source-discovery/latest-job/v1", s.handleSourceDiscoveryLatestV1)
	mux.HandleFunc("GET /source-discovery/jobs/{job_id}/v1", s.handleSourceDiscoveryGetV1)
	mux.HandleFunc("POST /source-discovery/jobs/{job_id}/resume/v1", s.handleSourceDiscoveryResumeV1)
	mux.HandleFunc("POST /source-discovery/jobs/{job_id}/complete/v1", s.handleSourceDiscoveryCompleteV1)
	mux.HandleFunc("POST /source-discovery/jobs/{job_id}/admit/v1", s.handleSourceDiscoveryAdmitV1)
	mux.HandleFunc("POST /source-discovery/jobs/{job_id}/repair-structural-roster/v1", s.handleSourceDiscoveryStructuralRosterRepairV1)
}

func (s *Server) handleSourceDiscoveryStructuralRosterRepairV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	mutable, ok := discoveryStore.(store.SourceDiscoveryMutableStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_repair_unavailable", "Source Discovery job updates are unavailable")
		return
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || ref == nil {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_repair_unavailable", "Reference Library store is unavailable")
		return
	}
	jobID := strings.TrimSpace(r.PathValue("job_id"))
	job, err := mutable.GetSourceDiscoveryJob(r.Context(), jobID)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	input, err := sourceDiscoveryInputFromJob(job)
	if err != nil || strings.TrimSpace(input.WorkID) == "" || strings.TrimSpace(input.ContinuityID) == "" {
		writeError(w, http.StatusConflict, "source_discovery_repair_scope_missing", "The Source Discovery job is not bound to a reference work and continuity")
		return
	}
	kept, removed := removeSourceDiscoveryStructuralRosterCandidates(sliceMapFromAny(job.Result["discovered_candidates"]))
	for _, candidate := range removed {
		name := firstSourceCandidateValue(candidate, "canonical_name", "name")
		if name == "" || !sourceCandidateIsEntityLike(candidate) {
			continue
		}
		entityID := referenceStableID("source-discovery-entity", input.WorkID, input.ContinuityID, normalizeSourceCandidateValue(name))
		if err := ref.UpdateReferenceCandidateReview(r.Context(), input.WorkID, "entity", entityID, "rejected", "system_structural_roster_repair", "removed invalid deterministic structural roster candidate"); err != nil && !errors.Is(err, store.ErrNotFound) {
			writeReferenceStoreError(w, err)
			return
		}
	}
	job.Result["discovered_candidates"] = mapsToAny(kept)
	job.Result["structural_roster_repair"] = map[string]any{
		"contract": "source_discovery_structural_roster_repair.v1", "removed": len(removed), "remaining": len(kept), "repaired_at": time.Now().UTC(),
	}
	conflicts, uncertainties := sourceDiscoveryReconciliation(kept)
	job.Result["conflicts"] = conflicts
	job.Result["uncertainties"] = uncertainties
	job.Result["scope_distinct_groups"] = sourceDiscoveryScopeDistinctGroups(kept)
	job.Result["review_queue"] = map[string]any{"mode": "exception_only", "conflicts": conflicts, "uncertainties": uncertainties, "admission_eligible": false}
	job.Result["admission_preview"] = sourceDiscoveryAdmissionPreview(kept, conflicts, uncertainties)
	coverage := discoveryCoverageFromCandidates(normalizedDiscoveryDomains(input.RequestedDomains), kept, sliceFromAny(job.Result["observations"]), sliceMapFromAny(job.Result["coverage_delta"]), sliceFromAny(job.Result["exceptions"]), false, false)
	state := "insufficient_source_coverage"
	if len(kept) > 0 {
		state = "awaiting_exception_review"
	}
	updated, err := mutable.UpdateSourceDiscoveryJob(r.Context(), jobID, state, job.Result, coverage)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "repaired", "job_id": jobID, "removed": len(removed), "remaining": len(kept), "job": updated})
}

func removeSourceDiscoveryStructuralRosterCandidates(candidates []map[string]any) ([]map[string]any, []map[string]any) {
	kept := make([]map[string]any, 0, len(candidates))
	removed := []map[string]any{}
	for _, candidate := range candidates {
		if stringFromMap(candidate, "provenance") == "deterministic_structural_roster.v1" {
			removed = append(removed, candidate)
			continue
		}
		kept = append(kept, candidate)
	}
	return kept, removed
}

func (s *Server) handleSourceDiscoveryLatestV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	queryStore, ok := discoveryStore.(store.SourceDiscoveryQueryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_query_unavailable", "Source Discovery job lookup is unavailable")
		return
	}
	job, err := queryStore.FindLatestSourceDiscoveryJob(r.Context(), strings.TrimSpace(r.URL.Query().Get("work_id")), strings.TrimSpace(r.URL.Query().Get("continuity_id")))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "none"})
		return
	}
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleSourceDiscoveryPreviewV1(w http.ResponseWriter, r *http.Request) {
	var input store.SourceDiscoveryInput
	if !decodeReferenceJSON(w, r, &input) {
		return
	}
	var err error
	input, err = s.resolveSourceDiscoveryWorkIdentity(r.Context(), input)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	if err := validateSourceDiscoveryInput(input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "source_discovery_input_invalid", err.Error())
		return
	}
	domains := normalizedDiscoveryDomains(input.RequestedDomains)
	queries := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		queries = append(queries, map[string]any{"query": strings.TrimSpace(input.WorkQuery) + " " + domain, "domain": domain, "generated_by": "deterministic_scope_frontier"})
	}
	diagnostics := []map[string]any{}
	for _, source := range input.Sources {
		status := "ready_for_access_gate"
		if !source.PolicyConfirmed {
			status = "manual_source_policy_confirmation_required"
		}
		diagnostics = append(diagnostics, map[string]any{"url": source.URL, "source_type": source.SourceType, "status": status})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contract": "source_discovery_preview.v1", "db_write": false,
		"query_frontier": queries, "source_diagnostics": diagnostics,
		"search_provider_required":   len(input.Sources) == 0 && !s.sourceSearchLLMConfigured(),
		"search_provider_configured": s.sourceSearchLLMConfigured(),
	})
}

func (s *Server) handleSourceDiscoveryCreateV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	var req struct {
		store.SourceDiscoveryInput
		ClientMeta map[string]any `json:"client_meta,omitempty"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	input := req.SourceDiscoveryInput
	var err error
	input, err = s.resolveSourceDiscoveryWorkIdentity(r.Context(), input)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	if err := validateSourceDiscoveryInput(input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "source_discovery_input_invalid", err.Error())
		return
	}
	extractionConfig := s.sourceDiscoveryCriticConfig(req.ClientMeta)
	pipelineCtx, cancel := context.WithTimeout(r.Context(), sourceDiscoveryOperationTimeout)
	defer cancel()
	input, result, coverage, state := s.runSourceDiscoveryPipeline(pipelineCtx, extractionConfig, input)
	retainedDocuments := takeSourceDiscoveryRetainedDocuments(result)
	job, err := discoveryStore.SaveSourceDiscoveryJob(r.Context(), input, state, result, coverage)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	if strings.TrimSpace(input.WorkID) != "" && strings.TrimSpace(input.ContinuityID) != "" {
		if ref, ok := s.Store.(store.ReferenceLibraryStore); ok && ref != nil {
			counts, stageErr := stageSourceDiscoveryResult(r.Context(), ref, job.JobID, input.WorkID, input.ContinuityID, retainedDocuments, sliceMapFromAny(result["discovered_candidates"]))
			if stageErr != nil {
				job.Result["staging"] = map[string]any{"status": "failed", "error": stageErr.Error()}
			} else {
				job.Result["staging"] = map[string]any{"status": "completed", "review_status": "pending", "counts": counts}
			}
		}
	}
	if mutable, ok := discoveryStore.(store.SourceDiscoveryMutableStore); ok {
		updated, updateErr := mutable.UpdateSourceDiscoveryJob(r.Context(), job.JobID, job.State, job.Result, job.CoverageReport)
		if updateErr != nil {
			writeReferenceStoreError(w, updateErr)
			return
		}
		job = updated
	}
	writeJSON(w, http.StatusCreated, job)
}

func takeSourceDiscoveryRetainedDocuments(result map[string]any) []map[string]any {
	retainedDocuments := sliceMapFromAny(result["_retained_documents"])
	delete(result, "_retained_documents")
	return retainedDocuments
}

func (s *Server) resolveSourceDiscoveryWorkIdentity(ctx context.Context, input store.SourceDiscoveryInput) (store.SourceDiscoveryInput, error) {
	if strings.TrimSpace(input.WorkID) == "" {
		return input, nil
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || ref == nil {
		return input, errors.New("selected reference work is unavailable")
	}
	work, err := ref.GetReferenceWork(ctx, input.WorkID)
	if err != nil {
		return input, err
	}
	input.WorkTitle = strings.TrimSpace(work.Title)
	input.WorkType = strings.ToLower(strings.TrimSpace(work.WorkType))
	if input.WorkTitle != "" {
		input.WorkQuery = input.WorkTitle
	}
	return input, nil
}

func (s *Server) sourceDiscoveryCriticConfig(clientMeta map[string]any) completeTurnLLMConfig {
	return s.completeTurnExtractionConfig(clientMeta).Critic
}

type sourceDiscoveryPipelineDeps struct {
	search  func(context.Context, store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error)
	fetch   func(context.Context, store.SourceDiscoverySource) (map[string]any, []map[string]any, error)
	extract func(context.Context, completeTurnLLMConfig, store.SourceDiscoveryInput, map[string]any) ([]map[string]any, []map[string]any, map[string]any, error)
}

func (s *Server) runSourceDiscoveryPipeline(ctx context.Context, cfg completeTurnLLMConfig, input store.SourceDiscoveryInput) (store.SourceDiscoveryInput, map[string]any, map[string]any, string) {
	var search func(context.Context, store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error)
	if s.sourceSearchLLMConfigured() {
		search = s.discoverSourcesWithSearchLLM
	}
	return runSourceDiscoveryPipelineWith(ctx, cfg, input, sourceDiscoveryPipelineDeps{
		search:  search,
		fetch:   fetchDiscoverySource,
		extract: runSourceCandidateExtraction,
	})
}

func runSourceDiscoveryPipelineWith(ctx context.Context, cfg completeTurnLLMConfig, input store.SourceDiscoveryInput, deps sourceDiscoveryPipelineDeps) (store.SourceDiscoveryInput, map[string]any, map[string]any, string) {
	domains := normalizedDiscoveryDomains(input.RequestedDomains)
	result := map[string]any{
		"contract": "source-discovery-result.v1", "admission_status": "pending",
		"extraction_contract": sourceCandidateExtractionContract,
		"observations":        []any{}, "section_candidates": []any{}, "exceptions": []any{},
	}
	seenURLs, seenQueries := map[string]bool{}, map[string]bool{}
	frontier := []map[string]any{{"query": strings.TrimSpace(input.WorkQuery), "domain": "identity", "generated_by": "initial_work_query", "state": "pending"}}
	if len(input.Sources) > 0 {
		frontier = nil
	}
	pendingSources := append([]store.SourceDiscoverySource(nil), input.Sources...)
	allCandidates := []map[string]any{}
	coverageDeltas := []map[string]any{}
	searchRounds := []map[string]any{}
	processingInventory := []map[string]any{}
	allObservations, allSections, allExceptions := []any{}, []any{}, []any{}
	retainedDocuments := []map[string]any{}
	seenDocumentHashes := map[string]bool{}
	seenSectionSignatures := map[string]map[string]any{}
	duplicateSectionCount := 0
	extractionTraces := []map[string]any{}
	operationalLimit := false
	saturated := false
	extractionFailure := map[string]any{}
	processingIncomplete := false

	for round := 1; ; round++ {
		if err := ctx.Err(); err != nil {
			allExceptions = append(allExceptions, map[string]any{"code": "operational_timeout", "message": err.Error()})
			operationalLimit = true
			break
		}
		queriesUsed := []string{}
		if len(pendingSources) == 0 && len(frontier) > 0 && deps.search != nil {
			for _, item := range frontier {
				query := strings.TrimSpace(stringFromMap(item, "query"))
				queryKey := strings.ToLower(query)
				if query == "" || seenQueries[queryKey] {
					continue
				}
				seenQueries[queryKey] = true
				queriesUsed = append(queriesUsed, query)
				queryInput := input
				queryInput.WorkQuery = query
				queryInput.Sources = nil
				sources, diagnostics, err := deps.search(ctx, queryInput)
				if err != nil {
					allExceptions = append(allExceptions, map[string]any{"query": query, "code": "search_provider_failed", "message": err.Error()})
					continue
				}
				searchRounds = append(searchRounds, map[string]any{"round": round, "query": query, "diagnostics": diagnostics})
				for _, source := range sources {
					if key := canonicalDiscoveryURL(source.URL); key != "" && !seenURLs[key] {
						seenURLs[key] = true
						pendingSources = append(pendingSources, source)
					}
				}
			}
		}

		roundObservations, roundSections, roundExceptions := []any{}, []any{}, []any{}
		for _, source := range pendingSources {
			key := canonicalDiscoveryURL(source.URL)
			if key == "" {
				continue
			}
			seenURLs[key] = true
			observation, sections, err := deps.fetch(ctx, source)
			if err != nil {
				roundExceptions = append(roundExceptions, map[string]any{"url": source.URL, "code": discoveryFetchErrorCode(err), "message": err.Error()})
				continue
			}
			documentHash := stringFromMap(observation, "document_sha256")
			if rawBody := stringFromMap(observation, "_raw_body"); rawBody != "" && documentHash != "" {
				retainedDocuments = append(retainedDocuments, map[string]any{
					"document_sha256": documentHash, "raw_text": rawBody,
					"source_url":  firstNonEmpty(stringFromMap(observation, "final_url"), stringFromMap(observation, "requested_url")),
					"source_type": source.SourceType, "media_type": observation["media_type"], "retrieved_at": observation["retrieved_at"],
					"document_title": observation["document_title"],
				})
			}
			delete(observation, "_raw_body")
			duplicateDocument := documentHash != "" && seenDocumentHashes[documentHash]
			if documentHash != "" {
				seenDocumentHashes[documentHash] = true
			}
			if duplicateDocument {
				observation["extraction_status"] = "duplicate_content_hash"
			}
			roundObservations = append(roundObservations, observation)
			if duplicateDocument {
				processingInventory = append(processingInventory, map[string]any{
					"round": round, "source_url": source.URL, "document_sha256": documentHash,
					"discovered_sections": len(sections), "processing_status": "duplicate_content_hash_skipped",
				})
				continue
			}
			for _, section := range sections {
				section["source_url"] = source.URL
				section["source_type"] = source.SourceType
				section["document_sha256"] = observation["document_sha256"]
				section["review_state"] = "pending"
				signature := discoverySectionSignature(section)
				if representative := seenSectionSignatures[signature]; signature != "" && representative != nil {
					appendDiscoveryEquivalentEvidence(representative, section)
					duplicateSectionCount++
					continue
				}
				if signature != "" {
					seenSectionSignatures[signature] = section
				}
				roundSections = append(roundSections, section)
			}
			processingInventory = append(processingInventory, map[string]any{
				"round": round, "source_url": source.URL, "document_sha256": observation["document_sha256"],
				"discovered_sections": len(sections), "processing_status": "normalized_for_extraction",
			})
		}
		// Internal links remain in source provenance. Coverage-driven follow-up
		// queries decide which pages deserve another network and LLM pass.
		pendingSources = nil
		roundSections = prioritizeDiscoverySections(roundSections)
		allObservations = append(allObservations, roundObservations...)
		allSections = append(allSections, roundSections...)
		allExceptions = append(allExceptions, roundExceptions...)

		newCandidates := []map[string]any{}
		nextFrontier := []map[string]any{}
		if len(roundSections) > 0 && deps.extract != nil {
			roundResult := map[string]any{"section_candidates": roundSections}
			candidates, followUps, trace, err := deps.extract(ctx, cfg, input, roundResult)
			if err != nil {
				extractionFailure = sourceCandidateExtractionFailure(err)
				allExceptions = append(allExceptions, map[string]any{"round": round, "code": "candidate_extraction_failed", "message": err.Error()})
			} else {
				newCandidates = candidates
				nextFrontier = followUps
				extractionTraces = append(extractionTraces, trace)
				if boolFromAny(trace["processing_incomplete"]) || boolFromAny(trace["input_truncated"]) || int64FromMap(trace, "processed_sections", 0) < int64FromMap(trace, "discovered_sections", 0) {
					processingIncomplete = true
					allExceptions = append(allExceptions, map[string]any{"round": round, "code": "processing_inventory_incomplete", "message": "not every normalized section fit in the bounded extraction request"})
				}
			}
		}
		before := len(allCandidates)
		allCandidates, _ = reconcileSourceCandidates(allCandidates, newCandidates)
		enrichSourceCandidatesWithEquivalentEvidence(allCandidates, allSections)
		meaningfulGain := len(allCandidates) - before
		coverageDeltas = append(coverageDeltas, map[string]any{
			"round": round, "queries": queriesUsed, "new_sources": len(roundObservations),
			"failed_sources": len(roundExceptions), "new_sections": len(roundSections),
			"new_candidates": meaningfulGain, "duplicate_candidates": len(newCandidates) - meaningfulGain,
		})
		frontier = uniqueSourceDiscoveryFrontier(nextFrontier, seenQueries)
		if processingIncomplete {
			operationalLimit = true
			break
		}
		if len(roundObservations) == 0 && meaningfulGain == 0 && len(frontier) == 0 && len(pendingSources) == 0 {
			saturated = true
			break
		}
		if len(frontier) == 0 && meaningfulGain == 0 && len(pendingSources) == 0 {
			saturated = true
			break
		}
	}

	input.Sources = sourceDiscoverySourcesFromObservations(allObservations)
	result["observations"] = allObservations
	result["section_candidates"] = allSections
	result["exceptions"] = allExceptions
	result["discovered_candidates"] = allCandidates
	result["dynamic_query_frontier"] = frontier
	result["coverage_delta"] = coverageDeltas
	result["processing_inventory"] = processingInventory
	result["duplicate_analysis_sections"] = duplicateSectionCount
	result["extraction"] = summarizeSourceDiscoveryExtraction(extractionTraces, allSections)
	result["_retained_documents"] = mapsToAny(retainedDocuments)
	result["search_rounds"] = searchRounds
	result["search_llm"] = sourceDiscoverySearchSummary(searchRounds)
	conflicts, uncertainties := sourceDiscoveryReconciliation(allCandidates)
	result["conflicts"] = conflicts
	result["uncertainties"] = uncertainties
	result["source_lineage"] = sourceDiscoverySourceLineage(allObservations)
	result["scope_distinct_groups"] = sourceDiscoveryScopeDistinctGroups(allCandidates)
	result["review_queue"] = map[string]any{"mode": "exception_only", "conflicts": conflicts, "uncertainties": uncertainties, "admission_eligible": false}
	result["admission_preview"] = sourceDiscoveryAdmissionPreview(allCandidates, conflicts, uncertainties)
	if len(extractionFailure) > 0 {
		result["extraction"] = extractionFailure
	}
	coverage := discoveryCoverageFromCandidates(domains, allCandidates, allObservations, coverageDeltas, allExceptions, saturated, operationalLimit)
	state := "insufficient_source_coverage"
	result["termination_reason"] = "insufficient_source_coverage"
	if saturated && !processingIncomplete && len(allCandidates) > 0 && len(conflicts) == 0 && len(coverageMissingDomains(coverage)) == 0 {
		state = "ready_for_admission"
		result["termination_reason"] = "coverage_saturation"
	} else if processingIncomplete {
		result["termination_reason"] = "analysis_backlog_remaining"
	} else if operationalLimit {
		result["termination_reason"] = "operational_limit_reached"
	} else if len(allObservations) == 0 && deps.search == nil && len(input.Sources) == 0 {
		state = "insufficient_source_coverage"
		result["termination_reason"] = "search_provider_required"
	} else if len(allObservations) == 0 {
		state = "blocked_by_access_policy"
		result["termination_reason"] = "all_sources_blocked_or_failed"
	} else if len(conflicts) > 0 || len(uncertainties) > 0 {
		state = "awaiting_exception_review"
		result["termination_reason"] = "exception_review_required"
	}
	return input, result, coverage, state
}

func (s *Server) handleSourceDiscoveryGetV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	job, err := discoveryStore.GetSourceDiscoveryJob(r.Context(), strings.TrimSpace(r.PathValue("job_id")))
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleSourceDiscoveryResumeV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	mutable, ok := discoveryStore.(store.SourceDiscoveryMutableStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_resume_unavailable", "Source Discovery job updates are unavailable")
		return
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || ref == nil {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_resume_unavailable", "Reference Library store is unavailable")
		return
	}
	var req struct {
		ClientMeta map[string]any `json:"client_meta,omitempty"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	jobID := strings.TrimSpace(r.PathValue("job_id"))
	job, err := mutable.GetSourceDiscoveryJob(r.Context(), jobID)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	input, err := sourceDiscoveryInputFromJob(job)
	if err != nil || strings.TrimSpace(input.WorkID) == "" || strings.TrimSpace(input.ContinuityID) == "" {
		writeError(w, http.StatusUnprocessableEntity, "source_discovery_resume_invalid", "The discovery job is not bound to a reference work and continuity")
		return
	}
	updated, err := s.resumeSourceDiscoveryJob(r.Context(), mutable, ref, job, input, s.sourceDiscoveryCriticConfig(req.ClientMeta))
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleSourceDiscoveryCompleteV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	mutable, ok := discoveryStore.(store.SourceDiscoveryMutableStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_resume_unavailable", "Source Discovery job updates are unavailable")
		return
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || ref == nil {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_resume_unavailable", "Reference Library store is unavailable")
		return
	}
	var req struct {
		ClientMeta map[string]any `json:"client_meta,omitempty"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	jobID := strings.TrimSpace(r.PathValue("job_id"))
	job, err := mutable.GetSourceDiscoveryJob(r.Context(), jobID)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	input, err := sourceDiscoveryInputFromJob(job)
	if err != nil || strings.TrimSpace(input.WorkID) == "" || strings.TrimSpace(input.ContinuityID) == "" {
		writeError(w, http.StatusUnprocessableEntity, "source_discovery_resume_invalid", "The discovery job is not bound to a reference work and continuity")
		return
	}
	cfg := s.sourceDiscoveryCriticConfig(req.ClientMeta)
	if !cfg.hasConfig() {
		writeError(w, http.StatusBadRequest, "critic_config_missing", "critic provider, api key, endpoint, and model are required")
		return
	}
	if s.AdminJobs == nil {
		s.AdminJobs = newAdminJobManager()
	}
	analysisJob := s.AdminJobs.start("source_discovery_corpus_analysis", jobID, map[string]any{
		"source_job_id": jobID, "work_id": input.WorkID, "continuity_id": input.ContinuityID,
	}, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		return s.runSourceDiscoveryCorpusAnalysis(ctx, mutable, ref, jobID, input, cfg, progress)
	})
	writeJSON(w, http.StatusAccepted, analysisJob)
}

func (s *Server) resumeSourceDiscoveryJob(ctx context.Context, mutable store.SourceDiscoveryMutableStore, ref store.ReferenceLibraryStore, job *store.SourceDiscoveryJob, input store.SourceDiscoveryInput, cfg completeTurnLLMConfig) (*store.SourceDiscoveryJob, error) {
	jobID := strings.TrimSpace(job.JobID)
	sections := sliceFromAny(job.Result["section_candidates"])
	priorLLMCalls := int(int64FromMap(mapFromAny(job.Result["extraction"]), "llm_call_count", 0))
	offset, contractUpgrade := sourceDiscoveryResumeContractOffset(job.Result, len(sections))
	if contractUpgrade {
		job.Result["extraction_contract"] = sourceCandidateExtractionContract
		job.Result["reanalysis"] = map[string]any{
			"reason":         "previous extraction did not prove exhaustive section coverage",
			"source_refetch": false, "offset_reset": true,
		}
	}
	if offset >= len(sections) {
		return job, nil
	}
	var resumeDuplicates int
	sections, resumeDuplicates = deduplicateSourceDiscoveryResumeSections(sections, offset)
	job.Result["duplicate_analysis_sections"] = int64FromMap(job.Result, "duplicate_analysis_sections", 0) + int64(resumeDuplicates)
	prioritized := prioritizeDiscoverySections(append([]any(nil), sections[offset:]...))
	sections = append(append([]any(nil), sections[:offset]...), prioritized...)
	job.Result["section_candidates"] = sections
	pipelineCtx, cancel := context.WithTimeout(ctx, sourceDiscoveryOperationTimeout)
	defer cancel()
	newCandidates, followUps, trace, extractErr := runSourceCandidateExtraction(pipelineCtx, cfg, input, map[string]any{"section_candidates": sections[offset:]})
	if extractErr != nil {
		job.Result["extraction"] = sourceCandidateExtractionFailure(extractErr)
		return mutable.UpdateSourceDiscoveryJob(ctx, jobID, "insufficient_source_coverage", job.Result, job.CoverageReport)
	}
	allCandidates, _ := reconcileSourceCandidates(sliceMapFromAny(job.Result["discovered_candidates"]), newCandidates)
	enrichSourceCandidatesWithEquivalentEvidence(allCandidates, sections)
	job.Result["discovered_candidates"] = allCandidates
	processedNow := int(int64FromMap(trace, "processed_sections", 0))
	attemptedNow := int(int64FromMap(trace, "attempted_sections", int64(processedNow)))
	if attemptedNow < processedNow {
		attemptedNow = processedNow
	}
	if attemptedNow > len(sections)-offset {
		attemptedNow = len(sections) - offset
	}
	deferredNow := attemptedNow - processedNow
	processedTotal := offset + attemptedNow
	if processedTotal > len(sections) {
		processedTotal = len(sections)
	}
	trace["processed_sections"] = processedTotal
	trace["exhaustively_processed_sections"] = offset + processedNow
	trace["attempted_sections"] = processedTotal
	trace["discovered_sections"] = len(sections)
	trace["remaining_sections"] = len(sections) - processedTotal
	trace["processing_incomplete"] = processedTotal < len(sections)
	trace["aggregate"] = true
	trace["accepted_candidates"] = len(allCandidates)
	trace["llm_call_count"] = priorLLMCalls + int(int64FromMap(trace, "llm_call_count", 0))
	trace["processed_document_count"] = discoverySectionDocumentCount(sections[:processedTotal])
	trace["remaining_document_count"] = discoverySectionDocumentCount(sections[processedTotal:])
	deferred := sliceMapFromAny(job.Result["deferred_incomplete_sections"])
	if deferredNow > 0 {
		deferred = appendSourceDiscoveryDeferredSections(deferred, sections[offset+processedNow:offset+attemptedNow])
	}
	job.Result["deferred_incomplete_sections"] = mapsToAny(deferred)
	trace["deferred_incomplete_sections"] = len(deferred)
	job.Result["extraction"] = trace
	job.Result["dynamic_query_frontier"] = mapsToAny(uniqueSourceDiscoveryFrontier(append(sliceMapFromAny(job.Result["dynamic_query_frontier"]), followUps...), map[string]bool{}))
	conflicts, uncertainties := sourceDiscoveryReconciliation(allCandidates)
	job.Result["conflicts"] = conflicts
	job.Result["uncertainties"] = uncertainties
	job.Result["review_queue"] = map[string]any{"mode": "exception_only", "conflicts": conflicts, "uncertainties": uncertainties, "admission_eligible": false}
	job.Result["admission_preview"] = sourceDiscoveryAdmissionPreview(allCandidates, conflicts, uncertainties)
	operationalLimit := processedTotal < len(sections)
	coverage := discoveryCoverageFromCandidates(normalizedDiscoveryDomains(input.RequestedDomains), allCandidates, sliceFromAny(job.Result["observations"]), sliceMapFromAny(job.Result["coverage_delta"]), sliceFromAny(job.Result["exceptions"]), !operationalLimit, operationalLimit)
	state := "insufficient_source_coverage"
	job.Result["termination_reason"] = "analysis_backlog_remaining"
	if !operationalLimit {
		job.Result["termination_reason"] = "coverage_review_required"
		if len(conflicts) > 0 || len(uncertainties) > 0 {
			state = "awaiting_exception_review"
		} else if len(coverageMissingDomains(coverage)) == 0 && len(allCandidates) > 0 {
			state = "ready_for_admission"
			job.Result["termination_reason"] = "coverage_saturation"
		}
	}
	counts, stageErr := persistSourceDiscoveryCandidates(ctx, ref, jobID, input.WorkID, input.ContinuityID, allCandidates, false, nil)
	if stageErr != nil {
		job.Result["staging"] = map[string]any{"status": "failed", "error": stageErr.Error()}
	} else {
		job.Result["staging"] = map[string]any{"status": "completed", "review_status": "pending", "counts": counts}
	}
	updated, err := mutable.UpdateSourceDiscoveryJob(ctx, jobID, state, job.Result, coverage)
	if err == nil && len(deferred) == 0 {
		markSourceDiscoveryProcessedDocuments(ctx, ref, input.WorkID, input.ContinuityID, sections, processedTotal)
	}
	return updated, err
}

func appendSourceDiscoveryDeferredSections(existing []map[string]any, sections []any) []map[string]any {
	seen := map[string]bool{}
	for _, item := range existing {
		seen[sourceDiscoveryDeferredSectionKey(item)] = true
	}
	for _, raw := range sections {
		section := mapFromAny(raw)
		item := map[string]any{
			"source_url": stringFromMap(section, "source_url"), "document_sha256": stringFromMap(section, "document_sha256"),
			"source_type": stringFromMap(section, "source_type"), "locator": mapFromAny(section["locator"]),
			"heading_path": stringFromMap(section, "heading_path"), "section_kind": stringFromMap(section, "section_kind"),
		}
		key := sourceDiscoveryDeferredSectionKey(item)
		if key == "\x00{}" || seen[key] {
			continue
		}
		seen[key] = true
		existing = append(existing, item)
	}
	return existing
}

func sourceDiscoveryDeferredSectionKey(section map[string]any) string {
	locator, _ := json.Marshal(mapFromAny(section["locator"]))
	return strings.TrimSpace(stringFromMap(section, "document_sha256")) + "\x00" + string(locator)
}

func (s *Server) runSourceDiscoveryCorpusAnalysis(ctx context.Context, mutable store.SourceDiscoveryMutableStore, ref store.ReferenceLibraryStore, jobID string, input store.SourceDiscoveryInput, cfg completeTurnLLMConfig, progress adminJobProgressFunc) (map[string]any, error) {
	passes := 0
	processedStart := 0
	lastProcessed := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		job, err := mutable.GetSourceDiscoveryJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		sections := sliceFromAny(job.Result["section_candidates"])
		if len(sections) == 0 {
			sections, err = sourceDiscoverySectionsFromStoredDocuments(ctx, ref, input, job.Result)
			if err != nil {
				return nil, err
			}
			if len(sections) > 0 {
				job.Result["section_candidates"] = sections
				job.Result["extraction"] = summarizeSourceDiscoveryExtraction(nil, sections)
				job.Result["reanalysis"] = map[string]any{
					"reason": "stored source body was retained before HTML sections could be normalized", "source_refetch": false,
				}
				job, err = mutable.UpdateSourceDiscoveryJob(ctx, jobID, job.State, job.Result, job.CoverageReport)
				if err != nil {
					return nil, err
				}
			}
		}
		beforeProcessed, _ := sourceDiscoveryResumeContractOffset(job.Result, len(sections))
		beforeCandidates := len(sliceMapFromAny(job.Result["discovered_candidates"]))
		if passes == 0 {
			processedStart = beforeProcessed
			lastProcessed = beforeProcessed
		}
		if beforeProcessed >= len(sections) {
			return sourceDiscoveryCorpusResult(job, passes, processedStart, beforeProcessed, beforeCandidates, sourceDiscoveryCorpusTermination(job)), nil
		}
		updated, err := s.resumeSourceDiscoveryJob(ctx, mutable, ref, job, input, cfg)
		if err != nil {
			return nil, err
		}
		passes++
		extraction := mapFromAny(updated.Result["extraction"])
		processed := int(int64FromMap(extraction, "processed_sections", int64(beforeProcessed)))
		remaining := int(int64FromMap(extraction, "remaining_sections", int64(len(sections)-processed)))
		candidateCount := len(sliceMapFromAny(updated.Result["discovered_candidates"]))
		growth := candidateCount - beforeCandidates
		progress(map[string]any{
			"stage": "corpus_analysis", "processed": processed, "candidate_count": len(sections),
			"progress_percent": adminJobProgressPercent(processed, len(sections)), "remaining_sections": remaining,
			"analysis_passes": passes, "new_candidates": growth,
		})
		if remaining <= 0 {
			return sourceDiscoveryCorpusResult(updated, passes, processedStart, processed, candidateCount, sourceDiscoveryCorpusTermination(updated)), nil
		}
		if processed <= lastProcessed {
			return sourceDiscoveryCorpusResult(updated, passes, processedStart, processed, candidateCount, "operational_limit_reached"), nil
		}
		lastProcessed = processed
	}
}

func sourceDiscoverySectionsFromStoredDocuments(ctx context.Context, ref store.ReferenceLibraryStore, input store.SourceDiscoveryInput, result map[string]any) ([]any, error) {
	type observedSource struct {
		mediaType  string
		sourceURL  string
		sourceType string
	}
	observed := map[string]observedSource{}
	for _, raw := range sliceFromAny(result["observations"]) {
		item := mapFromAny(raw)
		hash := strings.TrimSpace(stringFromMap(item, "document_sha256"))
		if hash == "" {
			continue
		}
		observed[hash] = observedSource{
			mediaType: stringFromMap(item, "media_type"), sourceURL: firstNonEmpty(stringFromMap(item, "final_url"), stringFromMap(item, "requested_url")),
			sourceType: stringFromMap(item, "source_type"),
		}
	}
	if len(observed) == 0 {
		return nil, nil
	}
	documents, err := ref.ListReferenceDocuments(ctx, input.WorkID, input.ContinuityID, "")
	if err != nil {
		return nil, err
	}
	sections := []any{}
	seen := map[string]map[string]any{}
	for _, document := range documents {
		source, ok := observed[strings.TrimSpace(document.ContentHash)]
		if !ok || strings.TrimSpace(document.RawRetention) != "full" || strings.TrimSpace(document.RawText) == "" {
			continue
		}
		mediaType := strings.TrimSpace(source.mediaType)
		if mediaType == "" {
			mediaType = "text/plain"
		}
		for _, section := range discoverySections([]byte(document.RawText), mediaType) {
			section["source_url"] = firstNonEmpty(source.sourceURL, document.SourceURI)
			section["source_type"] = defaultReferenceString(source.sourceType, document.SourceType)
			section["document_sha256"] = document.ContentHash
			section["review_state"] = "pending"
			signature := discoverySectionSignature(section)
			if representative := seen[signature]; signature != "" && representative != nil {
				appendDiscoveryEquivalentEvidence(representative, section)
				continue
			}
			if signature != "" {
				seen[signature] = section
			}
			sections = append(sections, section)
		}
	}
	return prioritizeDiscoverySections(sections), nil
}

func sourceDiscoveryCorpusTermination(job *store.SourceDiscoveryJob) string {
	if len(sliceMapFromAny(job.Result["deferred_incomplete_sections"])) > 0 {
		return "all_sources_attempted_with_incomplete_sections"
	}
	return "coverage_saturation"
}

func sourceDiscoveryCorpusResult(job *store.SourceDiscoveryJob, passes, processedStart, processed, candidateCount int, termination string) map[string]any {
	sections := sliceFromAny(job.Result["section_candidates"])
	if processed < 0 {
		processed = 0
	}
	if processed > len(sections) {
		processed = len(sections)
	}
	if processedStart < 0 {
		processedStart = 0
	}
	if processedStart > processed {
		processedStart = processed
	}
	remaining := len(sections) - processed
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"status": "completed", "source_job_id": job.JobID, "analysis_passes": passes,
		"processed_sections": processed - processedStart, "total_processed_sections": processed,
		"remaining_sections": remaining, "candidate_count": candidateCount, "termination_reason": termination,
		"llm_call_count":               int64FromMap(mapFromAny(job.Result["extraction"]), "llm_call_count", 0),
		"deferred_incomplete_sections": len(sliceMapFromAny(job.Result["deferred_incomplete_sections"])),
		"remaining_document_count":     discoverySectionDocumentCount(sections[processed:]),
	}
}

func markSourceDiscoveryProcessedDocuments(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string, sections []any, processed int) {
	if processed < 0 {
		processed = 0
	}
	if processed > len(sections) {
		processed = len(sections)
	}
	completed := map[string]bool{}
	remaining := map[string]bool{}
	for _, raw := range sections[:processed] {
		if hash := strings.TrimSpace(stringFromMap(mapFromAny(raw), "document_sha256")); hash != "" {
			completed[hash] = true
		}
	}
	for _, raw := range sections[processed:] {
		if hash := strings.TrimSpace(stringFromMap(mapFromAny(raw), "document_sha256")); hash != "" {
			remaining[hash] = true
		}
	}
	documents, err := ref.ListReferenceDocuments(ctx, workID, continuityID, "")
	if err != nil {
		return
	}
	for _, document := range documents {
		if completed[document.ContentHash] && !remaining[document.ContentHash] {
			_ = ref.UpdateReferenceDocumentStatus(ctx, document.DocumentID, "parsed")
		}
	}
}

func sourceDiscoveryResumeContractOffset(result map[string]any, sectionCount int) (int, bool) {
	if strings.TrimSpace(stringFromMap(result, "extraction_contract")) != sourceCandidateExtractionContract {
		return 0, true
	}
	return sourceDiscoveryResumeOffset(result, sectionCount), false
}

func sourceDiscoveryInputFromJob(job *store.SourceDiscoveryJob) (store.SourceDiscoveryInput, error) {
	var input store.SourceDiscoveryInput
	if job == nil {
		return input, store.ErrInvalidReference
	}
	encoded, err := json.Marshal(job.Input)
	if err != nil {
		return input, err
	}
	if err := json.Unmarshal(encoded, &input); err != nil {
		return input, err
	}
	return input, nil
}

func sourceDiscoveryResumeOffset(result map[string]any, totalSections int) int {
	extraction := mapFromAny(result["extraction"])
	processed := int(int64FromMap(extraction, "processed_sections", 0))
	if !boolFromAny(extraction["aggregate"]) {
		discovered := int(int64FromMap(extraction, "discovered_sections", 0))
		if discovered > 0 && discovered < totalSections {
			processed += totalSections - discovered
		}
	}
	if processed < 0 {
		return 0
	}
	if processed > totalSections {
		return totalSections
	}
	return processed
}

func deduplicateSourceDiscoveryResumeSections(sections []any, processed int) ([]any, int) {
	if processed < 0 {
		processed = 0
	}
	if processed > len(sections) {
		processed = len(sections)
	}
	out := append([]any(nil), sections[:processed]...)
	seen := map[string]map[string]any{}
	for _, raw := range out {
		section := mapFromAny(raw)
		if signature := discoverySectionSignature(section); signature != "" {
			seen[signature] = section
		}
	}
	duplicates := 0
	for _, raw := range sections[processed:] {
		section := mapFromAny(raw)
		signature := discoverySectionSignature(section)
		if representative := seen[signature]; signature != "" && representative != nil {
			appendDiscoveryEquivalentEvidence(representative, section)
			duplicates++
			continue
		}
		if signature != "" {
			seen[signature] = section
		}
		out = append(out, raw)
	}
	return out, duplicates
}

func (s *Server) handleSourceDiscoveryAdmitV1(w http.ResponseWriter, r *http.Request) {
	discoveryStore, ok := s.sourceDiscoveryAuthorityStore(w)
	if !ok {
		return
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || ref == nil {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_admission_unavailable", "Reference Library store is unavailable")
		return
	}
	var req struct {
		WorkID       string         `json:"work_id"`
		ContinuityID string         `json:"continuity_id"`
		Confirm      bool           `json:"confirm_evidence_validated_batch"`
		ClientMeta   map[string]any `json:"client_meta,omitempty"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.WorkID) == "" || strings.TrimSpace(req.ContinuityID) == "" || !req.Confirm {
		writeError(w, http.StatusUnprocessableEntity, "source_discovery_admission_confirmation_required", "work_id, continuity_id, and explicit batch confirmation are required")
		return
	}
	job, err := discoveryStore.GetSourceDiscoveryJob(r.Context(), strings.TrimSpace(r.PathValue("job_id")))
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	preview := mapFromAny(job.Result["admission_preview"])
	eligible := sliceMapFromAny(preview["eligible_batch"])
	if len(eligible) == 0 {
		writeError(w, http.StatusConflict, "source_discovery_no_eligible_batch", "No evidence-validated candidates are eligible for batch admission")
		return
	}
	counts, err := admitSourceDiscoveryCandidates(r.Context(), ref, job.JobID, req.WorkID, req.ContinuityID, eligible)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	indexStatus, indexResult := s.refreshCanonPackReferenceIndex(r.Context(), &store.CanonPackInstall{WorkID: req.WorkID}, req.ClientMeta)
	writeJSON(w, http.StatusOK, map[string]any{"contract": "source_discovery_admission.v1", "job_id": job.JobID, "status": "admitted", "review_source": "evidence_validated_batch", "counts": counts, "index_status": indexStatus, "index_result": indexResult})
}

func admitSourceDiscoveryCandidates(ctx context.Context, ref store.ReferenceLibraryStore, jobID, workID, continuityID string, candidates []map[string]any) (map[string]int, error) {
	return persistSourceDiscoveryCandidates(ctx, ref, jobID, workID, continuityID, candidates, true, nil)
}

func stageSourceDiscoveryCandidates(ctx context.Context, ref store.ReferenceLibraryStore, jobID, workID, continuityID string, candidates []map[string]any) (map[string]int, error) {
	return persistSourceDiscoveryCandidates(ctx, ref, jobID, workID, continuityID, candidates, false, nil)
}

func stageSourceDiscoveryResult(ctx context.Context, ref store.ReferenceLibraryStore, jobID, workID, continuityID string, retainedDocuments, candidates []map[string]any) (map[string]int, error) {
	documentCounts, documentIDs, err := stageSourceDiscoveryDocuments(ctx, ref, jobID, workID, continuityID, retainedDocuments)
	if err != nil {
		return documentCounts, err
	}
	counts, err := persistSourceDiscoveryCandidates(ctx, ref, jobID, workID, continuityID, candidates, false, documentIDs)
	if counts == nil {
		counts = map[string]int{}
	}
	for _, key := range []string{"documents", "documents_retained", "documents_upgraded", "source_observations"} {
		counts[key] += documentCounts[key]
	}
	return counts, err
}

func stageSourceDiscoveryDocuments(ctx context.Context, ref store.ReferenceLibraryStore, jobID, workID, continuityID string, retainedDocuments []map[string]any) (map[string]int, map[string]string, error) {
	counts := map[string]int{"documents": 0, "source_observations": len(retainedDocuments)}
	documentIDs := map[string]string{}
	if err := validateSourceDiscoveryReferenceScope(ctx, ref, workID, continuityID); err != nil {
		return counts, documentIDs, err
	}
	type retainedGroup struct {
		rawText       string
		sourceType    string
		mediaType     string
		documentTitle string
		sourceURLs    []string
		retrievedAt   []string
	}
	groups := map[string]*retainedGroup{}
	for _, material := range retainedDocuments {
		hash := strings.TrimSpace(stringFromMap(material, "document_sha256"))
		rawText := stringFromMap(material, "raw_text")
		if hash == "" || rawText == "" {
			continue
		}
		group := groups[hash]
		if group == nil {
			group = &retainedGroup{
				rawText: rawText, sourceType: defaultReferenceString(stringFromMap(material, "source_type"), "community_wiki"),
				mediaType: stringFromMap(material, "media_type"), documentTitle: stringFromMap(material, "document_title"),
			}
			groups[hash] = group
		}
		group.sourceURLs = appendUniqueSourceDiscoveryString(group.sourceURLs, stringFromMap(material, "source_url"))
		group.retrievedAt = appendUniqueSourceDiscoveryString(group.retrievedAt, fmt.Sprint(material["retrieved_at"]))
	}
	counts["documents_retained"] = len(groups)
	for hash, group := range groups {
		documentID := referenceStableID("source-discovery-document", workID, continuityID, hash)
		documentIDs[hash] = documentID
		metadata, _ := json.Marshal(map[string]any{
			"origin_kind": "source_discovery", "job_id": jobID, "source_urls": group.sourceURLs,
			"retrieved_at": group.retrievedAt, "media_type": group.mediaType,
			"document_title":   group.documentTitle,
			"content_contract": "fetched_source_body.v1", "redistribution": "local_only_not_packaged",
		})
		existing, getErr := ref.GetReferenceDocument(ctx, documentID)
		item := &store.ReferenceDocument{
			DocumentID: documentID, WorkID: workID, ContinuityID: continuityID,
			SourceType: group.sourceType, SourceURI: firstSourceDiscoveryString(group.sourceURLs),
			ContentHash: hash, RawRetention: "full", RawText: group.rawText,
			ImportStatus: "pending", ProvenanceJSON: string(metadata),
		}
		if errors.Is(getErr, store.ErrNotFound) {
			if err := ref.SaveReferenceDocument(ctx, item); err != nil {
				return counts, documentIDs, err
			}
			counts["documents"]++
		} else if getErr != nil {
			return counts, documentIDs, getErr
		} else if existing.WorkID != workID || existing.ContinuityID != continuityID || existing.ContentHash != hash {
			return counts, documentIDs, store.ErrReferenceConflict
		} else if strings.TrimSpace(existing.RawRetention) != "full" || strings.TrimSpace(existing.RawText) == "" {
			if err := ref.UpdateReferenceDocumentSource(ctx, item); err != nil {
				return counts, documentIDs, err
			}
			counts["documents_upgraded"]++
		}
	}
	return counts, documentIDs, nil
}

func appendUniqueSourceDiscoveryString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstSourceDiscoveryString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func validateSourceDiscoveryReferenceScope(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string) error {
	if _, err := ref.GetReferenceWork(ctx, workID); err != nil {
		return err
	}
	continuities, err := ref.ListReferenceContinuities(ctx, workID)
	if err != nil {
		return err
	}
	foundContinuity := false
	for _, continuity := range continuities {
		if continuity.ContinuityID == continuityID {
			foundContinuity = true
			break
		}
	}
	if !foundContinuity {
		return store.ErrInvalidReference
	}
	return nil
}

func persistSourceDiscoveryCandidates(ctx context.Context, ref store.ReferenceLibraryStore, jobID, workID, continuityID string, candidates []map[string]any, approve bool, retainedDocumentIDs map[string]string) (map[string]int, error) {
	if err := validateSourceDiscoveryReferenceScope(ctx, ref, workID, continuityID); err != nil {
		return nil, err
	}
	counts := map[string]int{"documents": 0, "entities": 0, "aliases": 0, "claims": 0, "timeline": 0}
	importStatus := "pending"
	if approve {
		importStatus = "approved"
	}
	entityIDs := map[string]string{}
	for _, candidate := range candidates {
		if !sourceCandidateIsEntityLike(candidate) {
			continue
		}
		name := firstSourceCandidateValue(candidate, "canonical_name", "name")
		if normalized := normalizeSourceCandidateValue(name); normalized != "" {
			entityIDs[normalized] = referenceStableID("source-discovery-entity", workID, continuityID, normalized)
		}
	}
	for _, candidate := range candidates {
		evidenceSet := sliceMapFromAny(candidate["evidence_set"])
		if len(evidenceSet) == 0 {
			continue
		}
		primary := evidenceSet[0]
		hash := stringFromMap(primary, "document_sha256")
		sourceURL := stringFromMap(primary, "source_url")
		sourceType := stringFromMap(primary, "source_type")
		if hash == "" || sourceURL == "" {
			continue
		}
		if sourceType == "" {
			sourceType = "community_wiki"
		}
		documentID := strings.TrimSpace(retainedDocumentIDs[hash])
		rawRetention := "none"
		if documentID == "" {
			documentID = referenceStableID("source-discovery-document", workID, continuityID, hash)
		} else {
			rawRetention = "full"
		}
		existingDocument, getErr := ref.GetReferenceDocument(ctx, documentID)
		if getErr == nil && strings.TrimSpace(existingDocument.RawRetention) != "" {
			rawRetention = existingDocument.RawRetention
		}
		kind := strings.ToLower(stringFromMap(candidate, "kind"))
		metadataValue := map[string]any{"origin_kind": "source_discovery", "job_id": jobID, "source_url": sourceURL, "evidence_set": evidenceSet, "raw_retention": rawRetention, "uncertain": int64FromMap(candidate, "independent_source_count", 1) < 2}
		if provenance := firstSourceCandidateValue(candidate, "structural_provenance", "provenance"); strings.HasPrefix(provenance, "deterministic_structural_roster.") {
			metadataValue["structural_provenance"] = provenance
		}
		if kind == "relation" {
			subject := strings.TrimSpace(stringFromMap(candidate, "subject"))
			target := strings.TrimSpace(firstSourceCandidateValue(candidate, "target", "object"))
			metadataValue["relation"] = map[string]any{
				"subject": subject, "predicate": firstSourceCandidateValue(candidate, "relation", "predicate"), "target": target,
				"subject_entity_id": entityIDs[normalizeSourceCandidateValue(subject)], "target_entity_id": entityIDs[normalizeSourceCandidateValue(target)],
			}
		}
		metadata, _ := json.Marshal(metadataValue)
		if errors.Is(getErr, store.ErrNotFound) {
			if err := ref.SaveReferenceDocument(ctx, &store.ReferenceDocument{DocumentID: documentID, WorkID: workID, ContinuityID: continuityID, SourceType: sourceType, SourceURI: sourceURL, ContentHash: hash, RawRetention: "none", ImportStatus: importStatus, ProvenanceJSON: string(metadata)}); err != nil {
				return counts, err
			}
			counts["documents"]++
		} else if getErr != nil {
			return counts, getErr
		}
		if kind == "event" {
			label := firstSourceCandidateValue(candidate, "statement", "claim_text", "value", "label", "name")
			if label == "" {
				continue
			}
			branch := defaultReferenceString(stringFromMap(candidate, "branch"), "main")
			nodeKey := firstSourceCandidateValue(candidate, "timeline_key", "node_key")
			chronologyStatus := "explicit"
			if nodeKey == "" {
				nodeKey = referenceStableID("source-discovery-timeline-key", workID, continuityID, branch, normalizeSourceCandidateValue(label))
				chronologyStatus = "unknown"
			}
			ordinal := int64FromMap(candidate, "timeline_ordinal", int64FromMap(candidate, "ordinal", 0))
			if ordinal == 0 {
				chronologyStatus = "unknown"
			}
			timelineMetadata := map[string]any{}
			_ = json.Unmarshal(metadata, &timelineMetadata)
			timelineMetadata["chronology_status"] = chronologyStatus
			timelineMetadataJSON, _ := json.Marshal(timelineMetadata)
			nodeID := referenceStableID("source-discovery-timeline", workID, continuityID, branch, normalizeSourceCandidateValue(label))
			node := &store.ReferenceTimelineNode{
				NodeID: nodeID, WorkID: workID, ContinuityID: continuityID, NodeKey: nodeKey,
				Label: label, Ordinal: ordinal, BranchKey: branch, NodeKind: "event",
				MetadataJSON: string(timelineMetadataJSON), ReviewStatus: "pending",
			}
			if err := ref.UpsertReferenceTimelineNode(ctx, node); err != nil {
				return counts, err
			}
			if approve {
				if err := ref.UpdateReferenceCandidateReview(ctx, workID, "timeline", nodeID, "approved", "evidence_validated_batch", "independent evidence passed Source Discovery reconciliation"); err != nil {
					return counts, err
				}
			}
			counts["timeline"]++
			continue
		}
		name := firstSourceCandidateValue(candidate, "canonical_name", "name", "label", "subject")
		if kind == "entity" || kind == "character" || kind == "location" || kind == "item" || kind == "faction" {
			if name == "" {
				continue
			}
			entityType := kind
			if kind == "entity" {
				entityType = strings.ToLower(firstSourceCandidateValue(candidate, "entity_type", "type"))
				switch entityType {
				case "character", "location", "item", "faction":
				default:
					entityType = "other"
				}
			}
			entityID := referenceStableID("source-discovery-entity", workID, continuityID, normalizeSourceCandidateValue(name))
			entity := &store.ReferenceEntity{EntityID: entityID, WorkID: workID, ContinuityID: continuityID, EntityType: entityType, CanonicalName: name, DescriptionText: firstSourceCandidateValue(candidate, "description", "statement"), MetadataJSON: string(metadata), ReviewStatus: "pending"}
			if err := ref.UpsertReferenceEntity(ctx, entity); err != nil {
				return counts, err
			}
			for _, alias := range stringsFromAny(candidate["aliases"]) {
				alias = strings.TrimSpace(alias)
				if alias == "" || normalizeSourceCandidateValue(alias) == normalizeSourceCandidateValue(name) {
					continue
				}
				if err := ref.UpsertReferenceEntityAlias(ctx, &store.ReferenceEntityAlias{
					WorkID: workID, ContinuityID: continuityID, EntityID: entityID,
					AliasText: alias, NormalizedAlias: normalizeSourceCandidateValue(alias), LanguageCode: stringFromMap(candidate, "language"),
				}); err != nil {
					return counts, err
				}
				counts["aliases"]++
			}
			if approve {
				if err := ref.UpdateReferenceCandidateReview(ctx, workID, "entity", entityID, "approved", "evidence_validated_batch", "independent evidence passed Source Discovery reconciliation"); err != nil {
					return counts, err
				}
			}
			counts["entities"]++
			continue
		}
		text := firstSourceCandidateValue(candidate, "statement", "claim_text", "value")
		if text == "" {
			continue
		}
		claimType := defaultReferenceString(kind, "claim")
		if kind == "setting" {
			claimType = "world_rule"
		}
		claimID := referenceStableID("source-discovery-claim", workID, continuityID, kind, normalizeSourceCandidateValue(text))
		claim := &store.ReferenceClaim{ClaimID: claimID, WorkID: workID, ContinuityID: continuityID, DocumentID: documentID, ClaimType: claimType, ClaimText: text, EvidenceExcerpt: stringFromMap(primary, "evidence_excerpt"), TemporalScope: defaultReferenceString(stringFromMap(candidate, "time_scope"), "bounded"), BranchKey: defaultReferenceString(stringFromMap(candidate, "branch"), "main"), KnowledgeScope: "public_world", ReviewStatus: "pending", MetadataJSON: string(metadata)}
		if err := ref.UpsertReferenceClaim(ctx, claim); err != nil {
			return counts, err
		}
		if approve {
			if err := ref.UpdateReferenceCandidateReview(ctx, workID, "claim", claimID, "approved", "evidence_validated_batch", "independent evidence passed Source Discovery reconciliation"); err != nil {
				return counts, err
			}
		}
		counts["claims"]++
	}
	return counts, nil
}

func (s *Server) sourceDiscoveryAuthorityStore(w http.ResponseWriter) (store.SourceDiscoveryStore, bool) {
	if s.Cfg.StoreMode != config.StoreModeMariaDBAuthority {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_unavailable", "Source Discovery requires MariaDB authority mode")
		return nil, false
	}
	discoveryStore, ok := s.Store.(store.SourceDiscoveryStore)
	if !ok || discoveryStore == nil {
		writeError(w, http.StatusServiceUnavailable, "source_discovery_unavailable", "Source Discovery store is unavailable")
		return nil, false
	}
	return discoveryStore, true
}

type sourceSearchProviderResult struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	OriginalURL string `json:"-"`
}

func (s *Server) sourceSearchLLMConfigured() bool {
	cfg := s.sourceSearchPlannerLLMConfig()
	if strings.TrimSpace(cfg.APIKey) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai", "gemini", "claude", "ollama":
		return strings.TrimSpace(cfg.Model) != ""
	default:
		return false
	}
}

func (s *Server) discoverSourcesWithSearchLLM(ctx context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
	cfg := s.sourceSearchPlannerLLMConfig()
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	searchLocale := sourceDiscoveryTitleLocale(input)
	if !s.sourceSearchLLMConfigured() {
		return nil, map[string]any{"status": "failed"}, errors.New("configured LLM does not support a native web search contract")
	}
	promptInput, _ := json.Marshal(map[string]any{
		"work_query": input.WorkQuery, "work_title": input.WorkTitle, "work_type": input.WorkType, "original_title": input.OriginalTitle,
		"edition_hint": input.EditionHint, "search_locale": searchLocale,
		"allowed_source_types": input.AllowedSourceTypes,
	})
	promptParts := []string{
		"Search public wiki articles only about this fictional or published work.",
		"Treat work_title and work_type as the selected work identity. Exclude same-title works in a different medium.",
		"Prefer wiki articles with attributable citations over unattributed discussion.",
		"Exclude stores, account pages, forums, social posts, fiction uploads, coupon pages, product listings, event promotion pages, memes, jokes, and unsupported claims.",
		"Find source pages rather than answering from memory. Do not invent titles, aliases, facts, or URLs.",
	}
	if searchLocale != "" {
		promptParts = append(promptParts, "Prioritize wiki pages written in search_locale. Use other languages only as fallback when search_locale coverage is insufficient.")
	}
	promptParts = append(promptParts, "Input: "+string(promptInput))
	prompt := strings.Join(promptParts, " ")
	results, err := executeNativeSourceSearchLLM(ctx, cfg, prompt, sourceDiscoverySearchQuery(input))
	if err != nil {
		return nil, map[string]any{"status": "failed", "provider": provider}, err
	}
	if len(results) == 0 {
		diagnostics := map[string]any{
			"status": "completed_no_results", "provider": provider, "model": strings.TrimSpace(cfg.Model),
			"result_count": 0, "approved_source_count": 0, "unmatched_or_unapproved_count": 0,
			"native_web_search": true, "reason": "native_web_search_returned_no_cited_sources",
		}
		addOllamaSearchAgentDiagnostics(diagnostics, provider)
		return nil, diagnostics, nil
	}
	results = prioritizeSourceSearchResultsByLocale(results, searchLocale)
	activeResults, deferredLocaleResults := deferExplicitFallbackLocaleResults(results, searchLocale)
	results = activeResults
	filteredResults, excludedResults := excludeSourceSearchResults(results)
	rewrittenSources := sourceSearchRewriteDiagnostics(results)
	sources, unmatched := providerSourcesFromResults(input, filteredResults)
	diagnostics := map[string]any{
		"status": "completed", "provider": provider, "model": strings.TrimSpace(cfg.Model),
		"result_count": len(results), "approved_source_count": len(sources),
		"unmatched_or_unapproved_count": unmatched, "excluded_result_count": len(excludedResults),
		"excluded_results": excludedResults, "rewritten_source_count": len(rewrittenSources),
		"rewritten_sources": rewrittenSources, "native_web_search": true,
	}
	if len(deferredLocaleResults) > 0 {
		diagnostics["deferred_locale_result_count"] = len(deferredLocaleResults)
		diagnostics["deferred_locale_results"] = sourceSearchResultURLs(deferredLocaleResults)
	}
	if searchLocale != "" {
		diagnostics["search_locale"] = searchLocale
		diagnostics["search_locale_basis"] = "display_title_script"
	}
	addOllamaSearchAgentDiagnostics(diagnostics, provider)
	return sources, diagnostics, nil
}

// Keep the selected display language and language-neutral sources in the active
// round. Explicitly different-language pages remain available as a fallback
// only when no preferred or neutral source was found.
func deferExplicitFallbackLocaleResults(results []sourceSearchProviderResult, preferred string) ([]sourceSearchProviderResult, []sourceSearchProviderResult) {
	preferred = strings.ToLower(strings.TrimSpace(preferred))
	if preferred == "" || len(results) == 0 {
		return results, nil
	}
	hasActiveTier := false
	for _, result := range results {
		detected := sourceSearchResultLocale(result)
		if detected == "" || detected == preferred {
			hasActiveTier = true
			break
		}
	}
	if !hasActiveTier {
		return results, nil
	}
	active := make([]sourceSearchProviderResult, 0, len(results))
	deferred := []sourceSearchProviderResult{}
	for _, result := range results {
		detected := sourceSearchResultLocale(result)
		if detected != "" && detected != preferred {
			deferred = append(deferred, result)
			continue
		}
		active = append(active, result)
	}
	return active, deferred
}

func sourceSearchResultURLs(results []sourceSearchProviderResult) []string {
	urls := make([]string, 0, len(results))
	for _, result := range results {
		if value := strings.TrimSpace(result.URL); value != "" {
			urls = append(urls, value)
		}
	}
	return urls
}

func sourceDiscoveryTitleLocale(input store.SourceDiscoveryInput) string {
	title := strings.TrimSpace(input.WorkTitle)
	if title == "" {
		title = strings.TrimSpace(input.WorkQuery)
	}
	hasHangul := false
	hasKana := false
	for _, r := range title {
		switch {
		case unicode.In(r, unicode.Hangul):
			hasHangul = true
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			hasKana = true
		}
	}
	if hasHangul == hasKana {
		return ""
	}
	if hasHangul {
		return "ko"
	}
	return "ja"
}

func prioritizeSourceSearchResultsByLocale(results []sourceSearchProviderResult, locale string) []sourceSearchProviderResult {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" || len(results) < 2 {
		return results
	}
	prioritized := append([]sourceSearchProviderResult(nil), results...)
	sort.SliceStable(prioritized, func(i, j int) bool {
		return sourceSearchResultLocaleRank(prioritized[i], locale) < sourceSearchResultLocaleRank(prioritized[j], locale)
	})
	return prioritized
}

func sourceSearchResultLocaleRank(result sourceSearchProviderResult, preferred string) int {
	detected := sourceSearchResultLocale(result)
	if detected == preferred {
		return 0
	}
	if detected == "" {
		return 1
	}
	return 2
}

func sourceSearchResultLocale(result sourceSearchProviderResult) string {
	if parsed, err := url.Parse(strings.TrimSpace(result.URL)); err == nil {
		labels := strings.Split(strings.ToLower(parsed.Hostname()), ".")
		for _, label := range labels {
			switch label {
			case "ko", "kr":
				return "ko"
			case "ja", "jp":
				return "ja"
			}
		}
		for _, label := range labels {
			if len(label) == 2 && label[0] >= 'a' && label[0] <= 'z' && label[1] >= 'a' && label[1] <= 'z' {
				switch label {
				case "ac", "co", "go", "ne", "or":
					continue
				}
				return label
			}
		}
	}
	return sourceDiscoveryTitleLocale(store.SourceDiscoveryInput{WorkTitle: result.Title})
}

func addOllamaSearchAgentDiagnostics(diagnostics map[string]any, provider string) {
	if diagnostics == nil || !strings.EqualFold(strings.TrimSpace(provider), "ollama") {
		return
	}
	diagnostics["search_agent"] = true
	diagnostics["agent_round_limit"] = ollamaSourceSearchMaxRounds
	diagnostics["agent_tool_call_limit"] = ollamaSourceSearchMaxToolCalls
}

func applySourceSearchTermination(result map[string]any, input store.SourceDiscoveryInput, diagnostics map[string]any) {
	if len(input.Sources) != 0 || result == nil {
		return
	}
	switch diagnostics["status"] {
	case "failed":
		result["termination_reason"] = "search_provider_failed"
	case "completed_no_results":
		result["termination_reason"] = "search_provider_no_cited_results"
	}
}

func sourceDiscoverySearchQuery(input store.SourceDiscoveryInput) string {
	query := strings.TrimSpace(input.WorkTitle)
	if query == "" {
		query = strings.TrimSpace(input.WorkQuery)
	} else if workQuery := strings.TrimSpace(input.WorkQuery); workQuery != "" && !strings.EqualFold(query, workQuery) {
		query += " " + workQuery
	}
	if original := strings.TrimSpace(input.OriginalTitle); original != "" && !strings.EqualFold(query, original) {
		query += " " + original
	}
	if workType := strings.TrimSpace(input.WorkType); workType != "" && !strings.EqualFold(workType, "other") && !strings.EqualFold(workType, "custom") {
		query += " " + workType
	}
	query += " wiki"
	return query
}

func executeNativeSourceSearchLLM(ctx context.Context, cfg completeTurnLLMConfig, prompt, searchQuery string) ([]sourceSearchProviderResult, error) {
	timeout := sourceDiscoveryLLMTimeout(cfg.TimeoutMs)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := validateNativeSourceSearchEndpoint(cfg.Provider, cfg.Endpoint); err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai":
		return executeOpenAINativeSourceSearch(ctx, cfg, prompt)
	case "gemini":
		return executeGeminiNativeSourceSearch(ctx, cfg, prompt)
	case "claude":
		return executeClaudeNativeSourceSearch(ctx, cfg, prompt)
	case "ollama":
		return executeOllamaSourceSearchAgent(ctx, cfg, prompt, searchQuery)
	default:
		return nil, fmt.Errorf("provider %q has no supported native web search contract", cfg.Provider)
	}
}

func sourceDiscoveryLLMTimeout(timeoutMs int64) time.Duration {
	if timeoutMs <= 0 {
		return 60 * time.Second
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func executeOllamaWebSearch(ctx context.Context, cfg completeTurnLLMConfig, query string, maxResults int64) ([]sourceSearchProviderResult, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if base == "" {
		base = "https://ollama.com"
	}
	target := base + "/api/web_search"
	if strings.HasSuffix(base, "/api") {
		target = base + "/web_search"
	} else if strings.HasSuffix(base, "/api/web_search") {
		target = base
	}
	status, data, raw, err := proxyDoJSON(ctx, target, map[string]string{
		"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + strings.TrimSpace(cfg.APIKey),
	}, map[string]any{"query": ollamaSourceSearchQuery(query), "max_results": ollamaMinInt64(10, maxInt64(1, maxResults))})
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 || data == nil {
		return nil, fmt.Errorf("Ollama web search returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
	}
	return parseOllamaWebSearchResults(data), nil
}

func executeOllamaSourceSearchAgent(ctx context.Context, cfg completeTurnLLMConfig, prompt, initialQuery string) ([]sourceSearchProviderResult, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if base == "" {
		base = "https://ollama.com"
	}
	chatTarget := base + "/api/chat"
	if strings.HasSuffix(base, "/api") {
		chatTarget = base + "/chat"
	}
	tool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "web_search",
			"description": "Search public wiki articles about the requested work.",
			"parameters": map[string]any{
				"type": "object", "required": []any{"query"},
				"properties": map[string]any{
					"query":       map[string]any{"type": "string", "description": "Focused web search query"},
					"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": 10},
				},
			},
		},
	}
	messages := []any{
		map[string]any{"role": "system", "content": strings.Join([]string{
			"You are a bounded source-discovery search planner.",
			"You must call web_search at least once and may use it for focused follow-up searches.",
			"Search for attributable wiki articles, not answers from memory.",
			"Return wiki article URLs only and prefer pages with citations.",
			"Exclude stores, account pages, forums, social posts, fiction uploads, coupon pages, product listings, event promotion pages, memes, jokes, and unsupported claims.",
			"Do not invent URLs or treat your final prose as evidence.",
		}, " ")},
		map[string]any{"role": "user", "content": prompt + " Initial search query: " + ollamaSourceSearchQuery(initialQuery)},
	}
	results := []sourceSearchProviderResult{}
	seenURLs := map[string]bool{}
	toolCallsUsed := 0
	for round := 0; round < ollamaSourceSearchMaxRounds && toolCallsUsed < ollamaSourceSearchMaxToolCalls; round++ {
		body := map[string]any{
			"model": cfg.Model, "messages": messages, "tools": []any{tool}, "stream": false,
			"think": ollamaSourceSearchThink(cfg.ReasoningEffort),
			"options": map[string]any{
				"temperature": cfg.Temperature,
				"num_predict": maxInt64(1, cfg.MaxTokens),
			},
		}
		status, data, raw, err := proxyDoJSON(ctx, chatTarget, map[string]string{
			"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + strings.TrimSpace(cfg.APIKey),
		}, body)
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 || data == nil {
			return nil, fmt.Errorf("Ollama search agent returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
		}
		message := mapFromAny(data["message"])
		toolCalls := sliceFromAny(message["tool_calls"])
		if len(toolCalls) == 0 {
			if toolCallsUsed == 0 {
				return nil, errors.New("Ollama search agent returned no web_search tool call")
			}
			break
		}
		messages = append(messages, message)
		for _, rawCall := range toolCalls {
			if toolCallsUsed >= ollamaSourceSearchMaxToolCalls {
				break
			}
			call := mapFromAny(rawCall)
			function := mapFromAny(call["function"])
			if !strings.EqualFold(strings.TrimSpace(stringFromMap(function, "name")), "web_search") {
				continue
			}
			arguments := ollamaSourceSearchArguments(function["arguments"])
			query := ollamaSourceSearchQuery(stringFromMap(arguments, "query"))
			if query == "" {
				query = ollamaSourceSearchQuery(initialQuery)
			}
			maxResults := int64FromMap(arguments, "max_results", 10)
			searchResults, err := executeOllamaWebSearch(ctx, cfg, query, maxResults)
			if err != nil {
				return nil, err
			}
			toolCallsUsed++
			for _, result := range searchResults {
				if result.URL == "" || seenURLs[result.URL] {
					continue
				}
				seenURLs[result.URL] = true
				results = append(results, result)
			}
			encoded, _ := json.Marshal(searchResults)
			messages = append(messages, map[string]any{
				"role": "tool", "tool_name": "web_search", "content": truncateRunes(string(encoded), ollamaSourceSearchToolResultRunes),
			})
		}
	}
	if toolCallsUsed == 0 {
		return nil, errors.New("Ollama search agent did not execute web_search")
	}
	return results, nil
}

func ollamaSourceSearchArguments(value any) map[string]any {
	if result := mapFromAny(value); len(result) > 0 {
		return result
	}
	if raw, ok := value.(string); ok {
		var result map[string]any
		if json.Unmarshal([]byte(raw), &result) == nil {
			return result
		}
	}
	return map[string]any{}
}

func ollamaSourceSearchQuery(value string) string {
	query := strings.TrimSpace(value)
	if query == "" {
		return ""
	}
	return truncateRunes(query, 500)
}

func ollamaSourceSearchThink(effort string) any {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal", "low":
		return "low"
	case "medium":
		return "medium"
	case "high", "xhigh", "max":
		return "high"
	case "enable":
		return true
	default:
		return false
	}
}

func ollamaMinInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func validateNativeSourceSearchEndpoint(provider, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("native web search endpoint must be a public HTTPS API endpoint")
	}
	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	if strings.Contains(path, "/chat/completions") {
		return fmt.Errorf("%s native web search requires an API base endpoint with a web-search tool, not a chat/completions endpoint", strings.TrimSpace(provider))
	}
	if strings.EqualFold(strings.TrimSpace(provider), "ollama") && (strings.HasSuffix(path, "/api/chat") || strings.HasSuffix(path, "/api/generate")) {
		return errors.New("Ollama web search requires the API base endpoint, such as https://ollama.com, not an /api/chat or /api/generate endpoint")
	}
	return nil
}

func executeOpenAINativeSourceSearch(ctx context.Context, cfg completeTurnLLMConfig, prompt string) ([]sourceSearchProviderResult, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	target := base + "/v1/responses"
	if strings.HasSuffix(base, "/v1") {
		target = base + "/responses"
	} else if strings.HasSuffix(base, "/responses") {
		target = base
	}
	body := map[string]any{
		"model": cfg.Model, "input": prompt, "tools": []any{map[string]any{"type": "web_search", "search_context_size": "low"}},
		"temperature": cfg.Temperature, "max_output_tokens": maxInt64(1, cfg.MaxTokens), "store": false,
	}
	if effort := strings.TrimSpace(cfg.ReasoningEffort); effort != "" && effort != "none" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	status, data, raw, err := proxyDoJSON(ctx, target, map[string]string{
		"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + strings.TrimSpace(cfg.APIKey),
	}, body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusBadRequest && proxyUnsupportedParameter(raw, data) {
		fallback := cloneMap(body)
		delete(fallback, "temperature")
		delete(fallback, "reasoning")
		status, data, raw, err = proxyDoJSON(ctx, target, map[string]string{
			"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + strings.TrimSpace(cfg.APIKey),
		}, fallback)
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status >= 300 || data == nil {
		return nil, fmt.Errorf("OpenAI web search returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
	}
	return parseOpenAINativeSourceSearchResults(data), nil
}

func executeGeminiNativeSourceSearch(ctx context.Context, cfg completeTurnLLMConfig, prompt string) ([]sourceSearchProviderResult, error) {
	target := proxyNormalizeGeminiEndpoint(cfg.Endpoint, cfg.Model, "generateContent")
	generation := map[string]any{"temperature": cfg.Temperature, "maxOutputTokens": maxInt64(1, cfg.MaxTokens)}
	if cfg.ReasoningBudgetTokens > 0 {
		generation["thinkingConfig"] = map[string]any{"thinkingBudget": cfg.ReasoningBudgetTokens, "includeThoughts": false}
	}
	body := map[string]any{
		"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}}}},
		"tools":    []any{map[string]any{"googleSearch": map[string]any{}}}, "generationConfig": generation,
	}
	status, data, raw, err := proxyDoJSON(ctx, target, map[string]string{
		"Content-Type": "application/json", "Accept": "application/json", "x-goog-api-key": strings.TrimSpace(cfg.APIKey),
	}, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 || data == nil {
		return nil, fmt.Errorf("Gemini grounded search returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
	}
	return parseGeminiNativeSourceSearchResults(data), nil
}

func executeClaudeNativeSourceSearch(ctx context.Context, cfg completeTurnLLMConfig, prompt string) ([]sourceSearchProviderResult, error) {
	target := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if target == "" {
		target = "https://api.anthropic.com"
	}
	if !strings.Contains(target, "/v1/") {
		target += "/v1/messages"
	}
	body := map[string]any{
		"model": cfg.Model, "max_tokens": maxInt64(1, cfg.MaxTokens), "temperature": cfg.Temperature,
		"messages": []any{map[string]any{"role": "user", "content": prompt}},
		"tools":    []any{map[string]any{"type": "web_search_20250305", "name": "web_search", "max_uses": 1}},
	}
	if cfg.ReasoningBudgetTokens >= 1024 {
		body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": cfg.ReasoningBudgetTokens}
	}
	status, data, raw, err := proxyDoJSON(ctx, target, map[string]string{
		"Content-Type": "application/json", "Accept": "application/json", "x-api-key": strings.TrimSpace(cfg.APIKey), "anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 || data == nil {
		return nil, fmt.Errorf("Claude web search returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
	}
	return parseClaudeNativeSourceSearchResults(data), nil
}

func appendSourceSearchResult(results []sourceSearchProviderResult, seen map[string]bool, rawURL, title string) []sourceSearchProviderResult {
	originalURL := strings.TrimSpace(rawURL)
	rawURL = normalizeSourceSearchResultURL(originalURL)
	if rawURL == "" || seen[rawURL] {
		return results
	}
	seen[rawURL] = true
	result := sourceSearchProviderResult{URL: rawURL, Title: strings.TrimSpace(title)}
	if originalURL != rawURL {
		result.OriginalURL = originalURL
	}
	return append(results, result)
}

func normalizeSourceSearchResultURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return strings.TrimSpace(rawURL)
	}
	parsed.Fragment = ""
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "namu.wiki" || host == "www.namu.wiki" || host == "m.namu.moe" || host == "www.namu.moe" {
		parsed.Host = "namu.moe"
	}
	query := parsed.Query()
	query.Del("uuid")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func parseOpenAINativeSourceSearchResults(data map[string]any) []sourceSearchProviderResult {
	results, seen := []sourceSearchProviderResult{}, map[string]bool{}
	for _, item := range anySlice(data["output"]) {
		entry, _ := item.(map[string]any)
		if action, ok := entry["action"].(map[string]any); ok {
			for _, source := range anySlice(action["sources"]) {
				candidate, _ := source.(map[string]any)
				results = appendSourceSearchResult(results, seen, extractionStringFromAny(candidate["url"]), extractionStringFromAny(candidate["title"]))
			}
		}
		for _, content := range anySlice(entry["content"]) {
			block, _ := content.(map[string]any)
			for _, annotation := range anySlice(block["annotations"]) {
				candidate, _ := annotation.(map[string]any)
				results = appendSourceSearchResult(results, seen, extractionStringFromAny(candidate["url"]), extractionStringFromAny(candidate["title"]))
			}
		}
	}
	return results
}

func parseGeminiNativeSourceSearchResults(data map[string]any) []sourceSearchProviderResult {
	results, seen := []sourceSearchProviderResult{}, map[string]bool{}
	for _, candidateValue := range anySlice(data["candidates"]) {
		candidate, _ := candidateValue.(map[string]any)
		metadata, _ := candidate["groundingMetadata"].(map[string]any)
		for _, chunkValue := range anySlice(metadata["groundingChunks"]) {
			chunk, _ := chunkValue.(map[string]any)
			web, _ := chunk["web"].(map[string]any)
			results = appendSourceSearchResult(results, seen, extractionStringFromAny(web["uri"]), extractionStringFromAny(web["title"]))
		}
	}
	return results
}

func parseClaudeNativeSourceSearchResults(data map[string]any) []sourceSearchProviderResult {
	results, seen := []sourceSearchProviderResult{}, map[string]bool{}
	for _, blockValue := range anySlice(data["content"]) {
		block, _ := blockValue.(map[string]any)
		for _, resultValue := range anySlice(block["content"]) {
			candidate, _ := resultValue.(map[string]any)
			if extractionStringFromAny(candidate["type"]) == "web_search_result" {
				results = appendSourceSearchResult(results, seen, extractionStringFromAny(candidate["url"]), extractionStringFromAny(candidate["title"]))
			}
		}
		for _, citationValue := range anySlice(block["citations"]) {
			candidate, _ := citationValue.(map[string]any)
			results = appendSourceSearchResult(results, seen, extractionStringFromAny(candidate["url"]), extractionStringFromAny(candidate["title"]))
		}
	}
	return results
}

func parseOllamaWebSearchResults(data map[string]any) []sourceSearchProviderResult {
	results, seen := []sourceSearchProviderResult{}, map[string]bool{}
	for _, value := range anySlice(data["results"]) {
		candidate, _ := value.(map[string]any)
		before := len(results)
		results = appendSourceSearchResult(results, seen, extractionStringFromAny(candidate["url"]), extractionStringFromAny(candidate["title"]))
		if len(results) > before {
			results[len(results)-1].Snippet = strings.TrimSpace(extractionStringFromAny(candidate["content"]))
		}
	}
	return results
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func providerSourcesFromResults(input store.SourceDiscoveryInput, results []sourceSearchProviderResult) ([]store.SourceDiscoverySource, int) {
	policies := map[string]store.SourceDiscoveryDomainPolicy{}
	for _, policy := range input.DomainPolicies {
		domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(policy.Domain)), ".")
		if domain != "" {
			policies[domain] = policy
		}
	}
	fallbackPolicy := store.SourceDiscoveryDomainPolicy{}
	if len(policies) == 0 {
		fallbackPolicy = store.SourceDiscoveryDomainPolicy{
			SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true,
		}
	}
	sources := []store.SourceDiscoverySource{}
	seen := map[string]bool{}
	unmatched := 0
	for _, result := range results {
		parsed, err := url.Parse(strings.TrimSpace(result.URL))
		if err != nil || parsed.Hostname() == "" {
			unmatched++
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
		var policy store.SourceDiscoveryDomainPolicy
		matched := fallbackPolicy.PolicyConfirmed
		if matched {
			policy = fallbackPolicy
		}
		for domain, candidate := range policies {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				policy, matched = candidate, true
				break
			}
		}
		if !matched || !policy.PolicyConfirmed || seen[result.URL] {
			unmatched++
			continue
		}
		seen[result.URL] = true
		sources = append(sources, store.SourceDiscoverySource{URL: result.URL, SourceType: policy.SourceType, AccessClass: policy.AccessClass, PolicyConfirmed: true})
	}
	return sources, unmatched
}

func sourceSearchRewriteDiagnostics(results []sourceSearchProviderResult) []map[string]any {
	rewritten := []map[string]any{}
	for _, result := range results {
		if result.OriginalURL == "" || result.OriginalURL == result.URL {
			continue
		}
		rewritten = append(rewritten, map[string]any{"from": result.OriginalURL, "to": result.URL, "reason": "url_canonicalized"})
	}
	return rewritten
}

func excludeSourceSearchResults(results []sourceSearchProviderResult) ([]sourceSearchProviderResult, []map[string]any) {
	filtered := make([]sourceSearchProviderResult, 0, len(results))
	excluded := []map[string]any{}
	for _, result := range results {
		reason := sourceSearchExclusionReason(result.URL)
		if reason == "" {
			filtered = append(filtered, result)
			continue
		}
		excluded = append(excluded, map[string]any{"url": strings.TrimSpace(result.URL), "reason": reason})
	}
	return filtered, excluded
}

func sourceSearchExclusionReason(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || parsed.Scheme != "https" || parsed.User != nil {
		return "invalid_or_non_https_source_url"
	}
	if !sourceSearchWikiURL(parsed) {
		return "not_wiki_source"
	}
	return ""
}

func sourceSearchWikiURL(parsed *url.URL) bool {
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	for _, label := range strings.Split(host, ".") {
		if strings.Contains(label, "wiki") {
			return true
		}
	}
	path := strings.ToLower(strings.TrimSpace(parsed.EscapedPath()))
	return path == "/wiki" || strings.HasPrefix(path, "/wiki/") || path == "/w" || strings.HasPrefix(path, "/w/")
}

func sourceCandidateExtractionFailure(err error) map[string]any {
	reason := "candidate extraction failed"
	if err != nil {
		reason = err.Error()
	}
	status := "failed"
	code := "extractor_request_failed"
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(reason), "context deadline exceeded") {
		return map[string]any{"status": status, "code": "candidate_extraction_timeout", "reason": reason}
	}
	switch reason {
	case "critic provider configuration is required for candidate extraction":
		status = "blocked"
		code = "extractor_config_missing"
	case "no fetched section candidates are available":
		status = "skipped"
		code = "no_fetched_sections"
	}
	return map[string]any{"status": status, "code": code, "reason": reason}
}

func sourceDiscoveryStructuralRosterCandidates(sections []any) []map[string]any {
	type rosterEntry struct {
		name    string
		section map[string]any
	}
	groups := map[string][]rosterEntry{}
	for _, raw := range sections {
		section := mapFromAny(raw)
		kind := strings.ToLower(strings.TrimSpace(stringFromMap(section, "section_kind")))
		if kind != "list_item" {
			continue
		}
		name := structuralRosterAnchorName(section)
		if name == "" || stringFromMap(section, "source_url") == "" || stringFromMap(section, "document_sha256") == "" || len(mapFromAny(section["locator"])) == 0 {
			continue
		}
		headingPath := strings.TrimSpace(stringFromMap(section, "heading_path"))
		groupPath := headingPath
		key := strings.Join([]string{stringFromMap(section, "document_sha256"), kind, groupPath}, "\x00")
		groups[key] = append(groups[key], rosterEntry{name: name, section: section})
	}

	candidates := []map[string]any{}
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	for _, key := range groupKeys {
		entries := groups[key]
		// A structural roster needs repeated linked sibling entries. A lone list
		// item or cell is not enough evidence that its anchor text is a name.
		if len(entries) < 2 {
			continue
		}
		for _, entry := range entries {
			section := entry.section
			candidate := map[string]any{
				"kind": "entity", "entity_type": "other", "canonical_name": entry.name,
				"work_identity_status": "ambiguous", "canon_scope_status": "ambiguous",
				"review_state": "pending", "admission_eligible": false,
				"provenance": "deterministic_structural_roster.v1",
				"source_url": stringFromMap(section, "source_url"), "source_type": stringFromMap(section, "source_type"),
				"document_sha256": stringFromMap(section, "document_sha256"), "locator": mapFromAny(section["locator"]),
				"evidence_excerpt":       stringFromMap(section, "excerpt"),
				"corroborating_evidence": mapsToAny(sliceMapFromAny(section["equivalent_evidence"])),
			}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func structuralRosterAnchorName(section map[string]any) string {
	anchors := sliceMapFromAny(section["anchors"])
	if len(anchors) != 1 {
		return ""
	}
	if !structuralRosterInternalArticleHref(stringFromMap(anchors[0], "href")) {
		return ""
	}
	return structuralRosterName(stringFromMap(anchors[0], "text"))
}

func structuralRosterInternalArticleHref(rawHref string) bool {
	rawHref = strings.TrimSpace(rawHref)
	if rawHref == "" || strings.HasPrefix(rawHref, "#") || strings.HasPrefix(rawHref, "//") {
		return false
	}
	parsed, err := url.Parse(rawHref)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Path == "" {
		return false
	}
	decodedPath, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return false
	}
	target := strings.TrimSpace(path.Base(strings.TrimRight(decodedPath, "/")))
	if target == "" || target == "." || target == ".." || strings.Contains(target, ":") {
		return false
	}
	lower := strings.ToLower(target)
	if lower == "wiki" || lower == "w" || lower == "index" || lower == "index.php" {
		return false
	}
	for _, extension := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".tif", ".tiff", ".ico", ".pdf", ".mp3", ".wav", ".ogg", ".mp4", ".webm", ".mov", ".avi"} {
		if strings.HasSuffix(lower, extension) {
			return false
		}
	}
	return true
}

func structuralRosterName(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || len([]rune(value)) > 160 || len(strings.Fields(value)) > 12 {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "://") || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func runSourceCandidateExtraction(ctx context.Context, cfg completeTurnLLMConfig, input store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
	sections := sliceFromAny(result["section_candidates"])
	if len(sections) == 0 {
		return nil, nil, nil, errors.New("no fetched section candidates are available")
	}
	structuralCandidates := sourceDiscoveryStructuralRosterCandidates(sections)
	allCandidates, _ := reconcileSourceCandidates(nil, structuralCandidates)
	if !cfg.hasConfig() {
		return allCandidates, nil, map[string]any{
			"status": "completed", "mode": "deterministic_structural_only",
			"accepted_candidates": len(allCandidates), "structural_roster_candidates": len(allCandidates),
			"discovered_sections": len(sections), "processed_sections": 0, "attempted_sections": 0,
			"remaining_sections": len(sections), "processing_incomplete": true, "llm_call_count": 0,
		}, nil
	}
	promptRuneBudget := sourceCandidateExtractionPromptRuneBudget(cfg)
	batches := sourceCandidateExtractionBatches(input, sections, promptRuneBudget)
	allFollowUps := []map[string]any{}
	processedSections := 0
	rejected := 0
	identityRejected := 0
	externalMetadataRejected := 0
	nonAtomicRejected := 0
	formatRetries := 0
	llmCallCount := 0
	attemptedSections := 0
	processingIncomplete := false
	batchError := ""
	var elapsedTotal time.Duration
	for _, batch := range batches {
		if err := ctx.Err(); err != nil {
			processingIncomplete = true
			batchError = err.Error()
			break
		}
		if deadline, ok := ctx.Deadline(); ok && processedSections > 0 {
			averageBatchDuration := elapsedTotal / time.Duration(processedSectionsBatchCount(processedSections, batches))
			if time.Until(deadline) <= averageBatchDuration+5*time.Second {
				processingIncomplete = true
				batchError = "operation time budget reserved before starting another extraction batch"
				break
			}
		}
		batchStarted := time.Now()
		llmCallCount++
		candidates, followUps, trace, err := runSourceCandidateExtractionBatch(ctx, cfg, input, batch)
		elapsedTotal += time.Since(batchStarted)
		if err != nil {
			if processedSections == 0 {
				return nil, nil, nil, err
			}
			processingIncomplete = true
			batchError = err.Error()
			break
		}
		attemptedSections += len(batch)
		allCandidates, _ = reconcileSourceCandidates(allCandidates, candidates)
		allFollowUps = append(allFollowUps, followUps...)
		processedInBatch := int(int64FromMap(trace, "processed_sections", int64(len(batch))))
		if processedInBatch < 0 {
			processedInBatch = 0
		}
		if processedInBatch > len(batch) {
			processedInBatch = len(batch)
		}
		processedSections += processedInBatch
		rejected += int(int64FromMap(trace, "rejected_unbound_candidates", 0))
		identityRejected += int(int64FromMap(trace, "rejected_work_identity_mismatch_candidates", 0))
		externalMetadataRejected += int(int64FromMap(trace, "rejected_external_metadata_candidates", 0))
		nonAtomicRejected += int(int64FromMap(trace, "rejected_non_atomic_candidates", 0))
		formatRetries += int(int64FromMap(trace, "format_retry_count", 0))
		if processedInBatch < len(batch) {
			processingIncomplete = true
			batchError = "extraction batch reported unexamined source sections"
			break
		}
	}
	allFollowUps = uniqueSourceDiscoveryFrontier(allFollowUps, map[string]bool{})
	trace := map[string]any{
		"status": "completed", "provider": cfg.Provider, "model": cfg.Model,
		"accepted_candidates": len(allCandidates), "rejected_unbound_candidates": rejected,
		"rejected_work_identity_mismatch_candidates": identityRejected,
		"rejected_external_metadata_candidates":      externalMetadataRejected,
		"rejected_non_atomic_candidates":             nonAtomicRejected,
		"format_retry_count":                         formatRetries, "discovered_sections": len(sections),
		"processed_sections": processedSections, "attempted_sections": attemptedSections, "input_truncated": false,
		"processing_incomplete": processingIncomplete, "batch_count": len(batches),
		"llm_call_count":               llmCallCount,
		"structural_roster_candidates": len(structuralCandidates),
		"prompt_rune_budget":           promptRuneBudget,
		"remaining_sections":           len(sections) - processedSections,
		"processed_document_count":     discoverySectionDocumentCount(sections[:processedSections]),
		"remaining_document_count":     discoverySectionDocumentCount(sections[processedSections:]),
	}
	if batchError != "" {
		trace["batch_error"] = batchError
	}
	return allCandidates, allFollowUps, trace, nil
}

func sourceCandidateExtractionPromptRuneBudget(cfg completeTurnLLMConfig) int {
	maxTokens := cfg.MaxTokens
	if cfg.MaxCompletionTokens > 0 {
		maxTokens = cfg.MaxCompletionTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	budget := int(maxTokens * 4)
	if budget < 8000 {
		return 8000
	}
	if budget > 24000 {
		return 24000
	}
	return budget
}

func processedSectionsBatchCount(processedSections int, batches [][]any) int {
	processedBatches := 0
	consumed := 0
	for _, batch := range batches {
		if consumed >= processedSections {
			break
		}
		consumed += len(batch)
		processedBatches++
	}
	if processedBatches == 0 {
		return 1
	}
	return processedBatches
}

func summarizeSourceDiscoveryExtraction(traces []map[string]any, sections []any) map[string]any {
	discoveredSections := len(sections)
	summary := map[string]any{
		"status": "skipped", "discovered_sections": discoveredSections, "processed_sections": 0,
		"remaining_sections": discoveredSections, "accepted_candidates": 0, "batch_count": 0,
		"rejected_unbound_candidates": 0, "rejected_work_identity_mismatch_candidates": 0,
		"rejected_external_metadata_candidates": 0, "rejected_non_atomic_candidates": 0,
		"format_retry_count": 0, "processing_incomplete": false,
	}
	if len(traces) == 0 {
		return summary
	}
	summary["status"] = "completed"
	for _, trace := range traces {
		for _, key := range []string{"processed_sections", "accepted_candidates", "batch_count", "rejected_unbound_candidates", "rejected_work_identity_mismatch_candidates", "rejected_external_metadata_candidates", "rejected_non_atomic_candidates", "format_retry_count"} {
			summary[key] = int64FromMap(summary, key, 0) + int64FromMap(trace, key, 0)
		}
		if boolFromAny(trace["processing_incomplete"]) {
			summary["processing_incomplete"] = true
		}
		if value := strings.TrimSpace(stringFromMap(trace, "batch_error")); value != "" {
			summary["batch_error"] = value
		}
		if summary["provider"] == nil {
			summary["provider"] = trace["provider"]
			summary["model"] = trace["model"]
		}
	}
	processed := int(int64FromMap(summary, "processed_sections", 0))
	if processed > discoveredSections {
		processed = discoveredSections
		summary["processed_sections"] = processed
	}
	remaining := discoveredSections - processed
	if remaining < 0 {
		remaining = 0
	}
	summary["remaining_sections"] = remaining
	summary["processing_incomplete"] = remaining > 0 || boolFromAny(summary["processing_incomplete"])
	summary["aggregate"] = true
	summary["processed_document_count"] = discoverySectionDocumentCount(sections[:processed])
	summary["remaining_document_count"] = discoverySectionDocumentCount(sections[processed:])
	return summary
}

func discoverySectionDocumentCount(sections []any) int {
	seen := map[string]bool{}
	for _, raw := range sections {
		if hash := strings.TrimSpace(stringFromMap(mapFromAny(raw), "document_sha256")); hash != "" {
			seen[hash] = true
		}
	}
	return len(seen)
}

func sourceCandidateExtractionBatches(input store.SourceDiscoveryInput, sections []any, maxPromptRunes int) [][]any {
	if maxPromptRunes <= 0 {
		return [][]any{sections}
	}
	batches := [][]any{}
	current := []any{}
	for _, section := range sections {
		trial := append(append([]any{}, current...), section)
		encoded, _ := json.Marshal(map[string]any{
			"work_query": input.WorkQuery, "work_title": input.WorkTitle, "work_type": input.WorkType, "original_title": input.OriginalTitle,
			"edition_hint": input.EditionHint, "language": input.Language, "sections": trial,
		})
		if len(current) > 0 && len([]rune(string(encoded))) > maxPromptRunes {
			batches = append(batches, current)
			current = []any{section}
			continue
		}
		current = trial
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func runSourceCandidateExtractionBatch(ctx context.Context, cfg completeTurnLLMConfig, input store.SourceDiscoveryInput, sections []any) ([]map[string]any, []map[string]any, map[string]any, error) {
	promptSections := sourceCandidatePromptSections(sections)
	promptInput, _ := json.Marshal(map[string]any{
		"work_query": input.WorkQuery, "work_title": input.WorkTitle, "work_type": input.WorkType, "original_title": input.OriginalTitle,
		"edition_hint": input.EditionHint, "language": input.Language, "sections": promptSections,
	})
	systemPrompt := strings.Join([]string{
		"You extract candidate facts from supplied source sections for an evidence ledger.",
		"The target identity is the supplied work_title and work_type. Ignore sections and candidates that clearly belong to a same-title work in a different medium.",
		"Every candidate must include work_identity_status as matched, ambiguous, or mismatch. Use mismatch for a clearly different work or medium and ambiguous when the evidence cannot decide.",
		"Extract in-world canon only: characters, places, items, factions, world rules, story events, and relations.",
		"Use kind character, location, item, or faction for named in-world entities. Use generic entity only when none of those types applies, and then include entity_type.",
		"For named entities, use canonical_name for the best-supported name and aliases only for names that the supplied evidence explicitly identifies as the same entity.",
		"Analyze the supplied sections as one structured document batch. Preserve heading, list, and table context across adjacent source_refs.",
		"Exhaustively enumerate every explicit atomic record; do not select representative examples from rosters, tables, lists, casts, organization charts, timelines, rule lists, or relation lists.",
		"Do not classify authors, illustrators, publishers, release dates, sales, adaptations, genres, product descriptions, or publication metadata as in-world canon.",
		"Every candidate must include canon_scope_status as in_world, ambiguous, or external_metadata.",
		"Write one concise atomic derived fact per candidate. Never copy a whole paragraph into a name, description, or statement.",
		"Return JSON only with records, follow_up_queries, examined_source_refs, and incomplete_source_refs. records must contain entities, world_rules, timeline_events, relations, and facts arrays.",
		"Each entity requires canonical_name, entity_type, and source_ref. entity_type is character, location, item, faction, or other. Description is optional and is never a name.",
		"Each world rule requires statement and source_ref. Each timeline event requires label and source_ref; chronology fields are optional and only evidence-based.",
		"Each relation requires subject, relation, target, and source_ref. Each remaining atomic fact requires statement and source_ref.",
		"Every record must use the source_ref of the supplied section that supports it. The backend attaches its URL, locator, and exact evidence text.",
		"Include a source_ref in examined_source_refs only after exhausting all explicit in-world information in that section. Put partially examined sections in incomplete_source_refs.",
		"For an event, include timeline_key and timeline_ordinal only when the supplied evidence explicitly establishes that chronology. Never invent an order.",
		"Do not fill gaps, resolve conflicts, or mark anything approved.",
	}, " ")
	promptText := string(promptInput)
	userPrompt := "Extract pending candidates and evidence-bound follow-up queries from this bounded input:\n" + promptText
	maxTokens := cfg.MaxTokens
	if cfg.MaxCompletionTokens > 0 {
		maxTokens = cfg.MaxCompletionTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	temperature := cfg.Temperature
	request := dto.ProxyPluginMainRequest{
		APIKey: &cfg.APIKey, Endpoint: &cfg.Endpoint, Model: &cfg.Model, Provider: &cfg.Provider,
		Messages:  []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens: &maxTokens, MaxCompletionTokens: &maxTokens, Temperature: &temperature, TimeoutMs: &cfg.TimeoutMs,
	}
	applyProxyOverridesFromLLMConfig(&request, cfg)
	content, err := callSourceCandidateExtractionLLM(ctx, cfg, request)
	if err != nil {
		return nil, nil, nil, err
	}
	parsed, err := parseJSONFromLLMContent(content)
	formatRetryCount := 0
	if err != nil {
		formatRetryCount = 1
		retryRequest := request
		retryRequest.Messages = append(append([]any{}, request.Messages...),
			map[string]any{"role": "assistant", "content": truncateRunes(content, 4000)},
			map[string]any{"role": "user", "content": "Your previous response was not valid JSON. Return exactly one JSON object with records, follow_up_queries, examined_source_refs, and incomplete_source_refs. Do not include prose or markdown fences."},
		)
		retryContent, retryErr := callSourceCandidateExtractionLLM(ctx, cfg, retryRequest)
		if retryErr != nil {
			return nil, nil, nil, retryErr
		}
		parsed, err = parseJSONFromLLMContent(retryContent)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("candidate_extraction_json_invalid_after_retry: %w", err)
	}
	sectionIndex := map[string]string{}
	sectionHashes := map[string]string{}
	sectionTypes := map[string]string{}
	sectionEquivalentEvidence := map[string][]map[string]any{}
	sectionsByRef := map[string]map[string]any{}
	for index, raw := range sections {
		section := mapFromAny(raw)
		sectionsByRef[fmt.Sprintf("s%d", index+1)] = section
		key := sourceCandidateEvidenceKey(stringFromMap(section, "source_url"), mapFromAny(section["locator"]))
		sectionIndex[key] = stringFromMap(section, "excerpt")
		sectionHashes[key] = stringFromMap(section, "document_sha256")
		sectionTypes[key] = stringFromMap(section, "source_type")
		sectionEquivalentEvidence[key] = sliceMapFromAny(section["equivalent_evidence"])
	}
	candidates := []map[string]any{}
	candidateRecords, typedRejected := sourceDiscoveryTypedCandidateRecords(parsed)
	rejected := typedRejected
	identityRejected := 0
	externalMetadataRejected := 0
	nonAtomicRejected := 0
	for _, raw := range candidateRecords {
		candidate := mapFromAny(raw)
		identityStatus := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "work_identity_status")))
		if identityStatus == "mismatch" {
			identityRejected++
			continue
		}
		if identityStatus != "matched" && identityStatus != "ambiguous" {
			identityStatus = "ambiguous"
		}
		canonScopeStatus := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "canon_scope_status")))
		if canonScopeStatus == "external_metadata" {
			externalMetadataRejected++
			continue
		}
		if canonScopeStatus != "in_world" && canonScopeStatus != "ambiguous" {
			canonScopeStatus = "ambiguous"
		}
		kind := strings.ToLower(stringFromMap(candidate, "kind"))
		switch kind {
		case "entity", "character", "location", "item", "faction", "setting", "event", "relation", "claim", "appearance":
		default:
			rejected++
			continue
		}
		if derived := sourceCandidateDerivedText(candidate); len([]rune(derived)) > 800 {
			nonAtomicRejected++
			continue
		}
		sourceURL := stringFromMap(candidate, "source_url")
		locator := mapFromAny(candidate["locator"])
		excerpt := strings.TrimSpace(stringFromMap(candidate, "evidence_excerpt"))
		if sourceRef := strings.TrimSpace(stringFromMap(candidate, "source_ref")); sourceRef != "" {
			section, exists := sectionsByRef[sourceRef]
			if !exists {
				rejected++
				continue
			}
			sourceURL = stringFromMap(section, "source_url")
			locator = mapFromAny(section["locator"])
			sectionText := strings.TrimSpace(stringFromMap(section, "excerpt"))
			if excerpt == "" || !strings.Contains(sectionText, excerpt) {
				excerpt = sectionText
			}
			candidate["source_url"] = sourceURL
			candidate["locator"] = locator
			candidate["evidence_excerpt"] = excerpt
		}
		evidenceKey := sourceCandidateEvidenceKey(sourceURL, locator)
		sectionText, ok := sectionIndex[evidenceKey]
		if !ok || excerpt == "" || !strings.Contains(sectionText, excerpt) {
			rejected++
			continue
		}
		candidate["kind"] = kind
		candidate["work_identity_status"] = identityStatus
		candidate["canon_scope_status"] = canonScopeStatus
		candidate["review_state"] = "pending"
		candidate["admission_eligible"] = false
		candidate["provenance"] = "model_derived_candidate"
		candidate["document_sha256"] = sectionHashes[evidenceKey]
		candidate["source_type"] = sectionTypes[evidenceKey]
		candidate["corroborating_evidence"] = mapsToAny(sectionEquivalentEvidence[evidenceKey])
		candidates = append(candidates, candidate)
	}
	examinedPrefix, coverageContractPresent := sourceCandidateExaminedPrefix(parsed, len(sections))
	followUps := []map[string]any{}
	for _, raw := range sliceFromAny(parsed["follow_up_queries"]) {
		query := ""
		domain := ""
		if item := mapFromAny(raw); len(item) > 0 {
			query = strings.TrimSpace(stringFromMap(item, "query"))
			domain = strings.TrimSpace(stringFromMap(item, "domain"))
		} else {
			query = strings.TrimSpace(fmt.Sprint(raw))
		}
		if query == "" || query == "<nil>" {
			continue
		}
		followUps = append(followUps, map[string]any{"query": query, "domain": domain, "generated_by": "model_candidate_frontier", "state": "pending"})
	}
	return candidates, followUps, map[string]any{
		"status": "completed", "provider": cfg.Provider, "model": cfg.Model,
		"accepted_candidates": len(candidates), "rejected_unbound_candidates": rejected,
		"rejected_work_identity_mismatch_candidates": identityRejected,
		"rejected_external_metadata_candidates":      externalMetadataRejected,
		"rejected_non_atomic_candidates":             nonAtomicRejected,
		"format_retry_count":                         formatRetryCount, "discovered_sections": len(sections),
		"processed_sections": examinedPrefix, "input_truncated": false,
		"coverage_contract_present": coverageContractPresent,
	}, nil
}

func sourceDiscoveryTypedCandidateRecords(parsed map[string]any) ([]any, int) {
	// Legacy generic records remain readable while v3 providers move to the
	// typed records object. The typed shape prevents descriptions and relation
	// prose from being interpreted as entity names.
	out := append([]any(nil), sliceFromAny(parsed["candidates"])...)
	records := mapFromAny(parsed["records"])
	rejected := 0
	for _, raw := range sliceFromAny(records["entities"]) {
		item := cloneMap(mapFromAny(raw))
		name := strings.TrimSpace(stringFromMap(item, "canonical_name"))
		if name == "" {
			rejected++
			continue
		}
		entityType := strings.ToLower(strings.TrimSpace(stringFromMap(item, "entity_type")))
		switch entityType {
		case "character", "location", "item", "faction":
			item["kind"] = entityType
		default:
			item["kind"] = "entity"
			item["entity_type"] = "other"
		}
		out = append(out, item)
	}
	for _, raw := range sliceFromAny(records["world_rules"]) {
		item := cloneMap(mapFromAny(raw))
		if strings.TrimSpace(stringFromMap(item, "statement")) == "" {
			rejected++
			continue
		}
		item["kind"] = "setting"
		out = append(out, item)
	}
	for _, raw := range sliceFromAny(records["timeline_events"]) {
		item := cloneMap(mapFromAny(raw))
		if strings.TrimSpace(stringFromMap(item, "label")) == "" {
			rejected++
			continue
		}
		item["kind"] = "event"
		out = append(out, item)
	}
	for _, raw := range sliceFromAny(records["relations"]) {
		item := cloneMap(mapFromAny(raw))
		subject := strings.TrimSpace(stringFromMap(item, "subject"))
		target := strings.TrimSpace(firstSourceCandidateValue(item, "target", "object"))
		relation := strings.TrimSpace(firstSourceCandidateValue(item, "relation", "predicate"))
		if subject == "" || target == "" || relation == "" {
			rejected++
			continue
		}
		item["kind"] = "relation"
		item["target"] = target
		item["statement"] = strings.Join([]string{subject, relation, target}, " ")
		out = append(out, item)
	}
	for _, raw := range sliceFromAny(records["facts"]) {
		item := cloneMap(mapFromAny(raw))
		if strings.TrimSpace(stringFromMap(item, "statement")) == "" {
			rejected++
			continue
		}
		item["kind"] = "claim"
		out = append(out, item)
	}
	return out, rejected
}

func sourceCandidatePromptSections(sections []any) []any {
	out := make([]any, 0, len(sections))
	for index, raw := range sections {
		section := mapFromAny(raw)
		item := map[string]any{
			"source_ref": fmt.Sprintf("s%d", index+1), "source_url": stringFromMap(section, "source_url"),
			"locator": mapFromAny(section["locator"]), "excerpt": stringFromMap(section, "excerpt"),
		}
		for _, key := range []string{"heading", "heading_path", "section_kind"} {
			if value, exists := section[key]; exists {
				item[key] = value
			}
		}
		out = append(out, item)
	}
	return out
}

func sourceCandidateExaminedPrefix(parsed map[string]any, sectionCount int) (int, bool) {
	_, examinedExists := parsed["examined_source_refs"]
	_, incompleteExists := parsed["incomplete_source_refs"]
	if !examinedExists || !incompleteExists {
		// Older providers remain readable, but diagnostics expose that they did
		// not prove exhaustive processing under the new extraction contract.
		return sectionCount, false
	}
	examined := map[string]bool{}
	for _, value := range stringsFromAny(parsed["examined_source_refs"]) {
		examined[strings.TrimSpace(value)] = true
	}
	incomplete := map[string]bool{}
	for _, value := range stringsFromAny(parsed["incomplete_source_refs"]) {
		incomplete[strings.TrimSpace(value)] = true
	}
	for index := 0; index < sectionCount; index++ {
		ref := fmt.Sprintf("s%d", index+1)
		if !examined[ref] || incomplete[ref] {
			return index, true
		}
	}
	return sectionCount, true
}

func sourceCandidateDerivedText(candidate map[string]any) string {
	return firstSourceCandidateValue(candidate, "name", "label", "statement", "claim_text", "value", "description", "subject")
}

func callSourceCandidateExtractionLLM(ctx context.Context, cfg completeTurnLLMConfig, request dto.ProxyPluginMainRequest) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(cfg.Provider), "ollama") {
		upstream, _, err := performProxyPluginMain(ctx, request)
		if err != nil {
			return "", err
		}
		content := chatCompletionText(upstream)
		if strings.TrimSpace(content) == "" {
			return "", errors.New("candidate extraction provider returned empty content")
		}
		return content, nil
	}
	target, err := ollamaNativeChatEndpoint(cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("Ollama candidate extraction endpoint is invalid: %w", err)
	}
	stringProperty := map[string]any{"type": "string"}
	recordProperties := map[string]any{
		"source_ref": stringProperty, "work_identity_status": stringProperty, "canon_scope_status": stringProperty,
		"canonical_name": stringProperty, "entity_type": stringProperty, "aliases": map[string]any{"type": "array", "items": stringProperty},
		"description": stringProperty, "statement": stringProperty, "label": stringProperty,
		"subject": stringProperty, "relation": stringProperty, "target": stringProperty,
		"timeline_key": stringProperty, "timeline_ordinal": map[string]any{"type": "integer"},
	}
	recordArray := func(required ...any) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": required, "properties": recordProperties}}
	}
	body := map[string]any{
		"model": cfg.Model, "messages": request.Messages, "stream": false,
		"think": ollamaSourceSearchThink(cfg.ReasoningEffort),
		"format": map[string]any{
			"type": "object", "required": []any{"records", "follow_up_queries", "examined_source_refs", "incomplete_source_refs"},
			"properties": map[string]any{
				"records": map[string]any{"type": "object", "required": []any{"entities", "world_rules", "timeline_events", "relations", "facts"}, "properties": map[string]any{
					"entities": recordArray("canonical_name", "entity_type", "source_ref"), "world_rules": recordArray("statement", "source_ref"),
					"timeline_events": recordArray("label", "source_ref"), "relations": recordArray("subject", "relation", "target", "source_ref"),
					"facts": recordArray("statement", "source_ref"),
				}},
				"follow_up_queries":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"examined_source_refs":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"incomplete_source_refs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
		"options": map[string]any{"temperature": cfg.Temperature, "num_predict": maxInt64(1, int64Value(request.MaxCompletionTokens, cfg.MaxTokens))},
	}
	status, data, raw, err := proxyDoJSON(ctx, target, map[string]string{
		"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + strings.TrimSpace(cfg.APIKey),
	}, body)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 || data == nil {
		return "", fmt.Errorf("Ollama candidate extraction returned HTTP %d: %s", status, scrubProxySecret(proxyErrorDetail(status, data, raw), cfg.APIKey))
	}
	content := stringFromMap(mapFromAny(data["message"]), "content")
	if content == "" {
		return "", errors.New("Ollama candidate extraction returned empty structured content")
	}
	return content, nil
}

func ollamaNativeChatEndpoint(endpoint string) (string, error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		raw = "https://ollama.com"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("an absolute Ollama HTTP(S) endpoint is required")
	}
	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/v1/chat/completions", "/chat/completions", "/api/generate", "/api/chat", "/v1", "/api"} {
		if strings.HasSuffix(strings.ToLower(path), suffix) {
			path = path[:len(path)-len(suffix)]
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + "/api/chat"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func sourceCandidateEvidenceKey(sourceURL string, locator map[string]any) string {
	encoded, _ := json.Marshal(locator)
	return strings.TrimSpace(sourceURL) + "\x00" + string(encoded)
}

func canonicalDiscoveryURL(rawURL string) string {
	normalized := normalizeSourceSearchResultURL(rawURL)
	parsed, err := url.Parse(strings.TrimSpace(normalized))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	parsed.Fragment = ""
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}

func uniqueSourceDiscoveryFrontier(items []map[string]any, seen map[string]bool) []map[string]any {
	out := []map[string]any{}
	queued := map[string]bool{}
	for _, item := range items {
		query := strings.TrimSpace(stringFromMap(item, "query"))
		key := strings.ToLower(query)
		if query == "" || seen[key] || queued[key] {
			continue
		}
		queued[key] = true
		clone := cloneMap(item)
		clone["state"] = "pending"
		out = append(out, clone)
	}
	return out
}

func sourceDiscoverySourcesFromObservations(values []any) []store.SourceDiscoverySource {
	out := []store.SourceDiscoverySource{}
	seen := map[string]bool{}
	for _, value := range values {
		item := mapFromAny(value)
		rawURL := stringFromMap(item, "final_url")
		if rawURL == "" {
			rawURL = stringFromMap(item, "requested_url")
		}
		key := canonicalDiscoveryURL(rawURL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		sourceType := stringFromMap(item, "source_type")
		if sourceType == "" {
			sourceType = "discovered_public"
		}
		accessClass := stringFromMap(item, "access_class")
		if accessClass == "" {
			accessClass = "public_web"
		}
		out = append(out, store.SourceDiscoverySource{URL: rawURL, SourceType: sourceType, AccessClass: accessClass, PolicyConfirmed: true})
	}
	return out
}

func sourceCandidateLogicalKey(candidate map[string]any) string {
	kind := strings.ToLower(stringFromMap(candidate, "kind"))
	identity := firstSourceCandidateValue(candidate, "canonical_name", "name", "label", "subject", "entity", "source")
	assertion := firstSourceCandidateValue(candidate, "statement", "claim_text", "value", "object", "target")
	scope := firstSourceCandidateValue(candidate, "edition", "edition_id", "continuity", "branch", "time_scope")
	if identity == "" {
		identity = assertion
	}
	return strings.Join([]string{kind, normalizeSourceCandidateValue(identity), normalizeSourceCandidateValue(assertion), normalizeSourceCandidateValue(scope)}, "\x00")
}

func sourceCandidateSubjectKey(candidate map[string]any) string {
	kind := strings.ToLower(stringFromMap(candidate, "kind"))
	identity := firstSourceCandidateValue(candidate, "canonical_name", "name", "label", "subject", "entity", "source")
	scope := firstSourceCandidateValue(candidate, "edition", "edition_id", "continuity", "branch", "time_scope")
	if identity == "" {
		return ""
	}
	return strings.Join([]string{kind, normalizeSourceCandidateValue(identity), normalizeSourceCandidateValue(scope)}, "\x00")
}

func firstSourceCandidateValue(candidate map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromMap(candidate, key); value != "" {
			return value
		}
	}
	return ""
}

func normalizeSourceCandidateValue(value string) string {
	return normalizeDiscoveryAnalysisText(value)
}

func sourceCandidateEvidence(candidate map[string]any) map[string]any {
	return map[string]any{
		"source_url": stringFromMap(candidate, "source_url"), "document_sha256": stringFromMap(candidate, "document_sha256"),
		"source_type": stringFromMap(candidate, "source_type"), "locator": mapFromAny(candidate["locator"]), "evidence_excerpt": stringFromMap(candidate, "evidence_excerpt"),
	}
}

func sourceEvidenceKey(evidence map[string]any) string {
	encoded, _ := json.Marshal(mapFromAny(evidence["locator"]))
	return strings.Join([]string{stringFromMap(evidence, "source_url"), stringFromMap(evidence, "document_sha256"), string(encoded), stringFromMap(evidence, "evidence_excerpt")}, "\x00")
}

func reconcileSourceCandidates(existing, incoming []map[string]any) ([]map[string]any, int) {
	out := append([]map[string]any(nil), existing...)
	index := map[string]int{}
	entityIndex := map[string]int{}
	for i, item := range out {
		index[sourceCandidateLogicalKey(item)] = i
		if key := sourceCandidateEntityIdentityKey(item); key != "" {
			entityIndex[key] = i
		}
	}
	duplicates := 0
	for _, raw := range incoming {
		candidate := cloneMap(raw)
		evidence := sourceCandidateEvidence(candidate)
		candidateEvidence := []map[string]any{evidence}
		candidateEvidence = append(candidateEvidence, sliceMapFromAny(candidate["corroborating_evidence"])...)
		delete(candidate, "corroborating_evidence")
		key := sourceCandidateLogicalKey(candidate)
		if key == "\x00\x00\x00" {
			continue
		}
		position, duplicate := index[key]
		entityIdentity := sourceCandidateEntityIdentityKey(candidate)
		if !duplicate && entityIdentity != "" {
			if entityPosition, ok := entityIndex[entityIdentity]; ok && sourceCandidateEntityKindsCompatible(out[entityPosition], candidate) {
				position, duplicate = entityPosition, true
			}
		}
		if duplicate {
			duplicates++
			current := out[position]
			if sourceCandidateIsGenericEntity(current) && !sourceCandidateIsGenericEntity(candidate) {
				priorProvenance := stringFromMap(current, "provenance")
				for field, value := range candidate {
					current[field] = value
				}
				if priorProvenance != "" {
					current["structural_provenance"] = priorProvenance
				}
			}
			set := sliceMapFromAny(current["evidence_set"])
			if len(set) == 0 {
				set = append(set, sourceCandidateEvidence(current))
			}
			seen := map[string]bool{}
			for _, item := range set {
				seen[sourceEvidenceKey(item)] = true
			}
			for _, item := range candidateEvidence {
				if !seen[sourceEvidenceKey(item)] {
					set = append(set, item)
					seen[sourceEvidenceKey(item)] = true
				}
			}
			current["evidence_set"] = mapsToAny(set)
			current["independent_source_count"] = sourceIndependentHostCount(set)
			index[sourceCandidateLogicalKey(current)] = position
			if entityIdentity != "" {
				entityIndex[entityIdentity] = position
			}
			continue
		}
		candidate["evidence_set"] = mapsToAny(candidateEvidence)
		candidate["independent_source_count"] = sourceIndependentHostCount(candidateEvidence)
		candidate["review_state"] = "pending"
		candidate["admission_eligible"] = false
		index[key] = len(out)
		if entityIdentity != "" {
			entityIndex[entityIdentity] = len(out)
		}
		out = append(out, candidate)
	}
	return out, duplicates
}

func sourceCandidateEntityIdentityKey(candidate map[string]any) string {
	if !sourceCandidateIsEntityLike(candidate) {
		return ""
	}
	name := normalizeSourceCandidateValue(firstSourceCandidateValue(candidate, "canonical_name", "name"))
	if name == "" {
		return ""
	}
	scope := normalizeSourceCandidateValue(firstSourceCandidateValue(candidate, "edition", "edition_id", "continuity", "branch", "time_scope"))
	return name + "\x00" + scope
}

func sourceCandidateIsEntityLike(candidate map[string]any) bool {
	switch strings.ToLower(stringFromMap(candidate, "kind")) {
	case "entity", "character", "location", "item", "faction":
		return true
	default:
		return false
	}
}

func sourceCandidateIsGenericEntity(candidate map[string]any) bool {
	return strings.EqualFold(stringFromMap(candidate, "kind"), "entity")
}

func sourceCandidateEntityKindsCompatible(left, right map[string]any) bool {
	return sourceCandidateIsGenericEntity(left) || sourceCandidateIsGenericEntity(right) || strings.EqualFold(stringFromMap(left, "kind"), stringFromMap(right, "kind"))
}

func enrichSourceCandidatesWithEquivalentEvidence(candidates []map[string]any, sections []any) {
	equivalents := map[string][]map[string]any{}
	for _, raw := range sections {
		section := mapFromAny(raw)
		key := sourceCandidateEvidenceKey(stringFromMap(section, "source_url"), mapFromAny(section["locator"]))
		if key == "\x00{}" {
			continue
		}
		equivalents[key] = sliceMapFromAny(section["equivalent_evidence"])
	}
	for _, candidate := range candidates {
		set := sliceMapFromAny(candidate["evidence_set"])
		if len(set) == 0 {
			set = append(set, sourceCandidateEvidence(candidate))
		}
		seen := map[string]bool{}
		for _, evidence := range set {
			seen[sourceEvidenceKey(evidence)] = true
		}
		for _, evidence := range append([]map[string]any(nil), set...) {
			key := sourceCandidateEvidenceKey(stringFromMap(evidence, "source_url"), mapFromAny(evidence["locator"]))
			for _, equivalent := range equivalents[key] {
				if !seen[sourceEvidenceKey(equivalent)] {
					set = append(set, equivalent)
					seen[sourceEvidenceKey(equivalent)] = true
				}
			}
		}
		candidate["evidence_set"] = mapsToAny(set)
		candidate["independent_source_count"] = sourceIndependentHostCount(set)
	}
}

func sliceMapFromAny(value any) []map[string]any {
	if values, ok := value.([]map[string]any); ok {
		return append([]map[string]any(nil), values...)
	}
	out := []map[string]any{}
	for _, raw := range sliceFromAny(value) {
		if item := mapFromAny(raw); len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func mapsToAny(values []map[string]any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func sourceIndependentHostCount(evidence []map[string]any) int {
	hashes := map[string]bool{}
	hosts := map[string]bool{}
	excerpts := map[string]bool{}
	count := 0
	for _, item := range evidence {
		hash := stringFromMap(item, "document_sha256")
		excerpt := normalizeSourceCandidateValue(stringFromMap(item, "evidence_excerpt"))
		host := ""
		if parsed, err := url.Parse(stringFromMap(item, "source_url")); err == nil {
			host = strings.ToLower(parsed.Hostname())
		}
		if (hash == "" || !hashes[hash]) && (host == "" || !hosts[host]) && (excerpt == "" || !excerpts[excerpt]) {
			count++
		}
		if hash != "" {
			hashes[hash] = true
		}
		if host != "" {
			hosts[host] = true
		}
		if excerpt != "" {
			excerpts[excerpt] = true
		}
	}
	return count
}

func sourceDiscoverySourceLineage(observations []any) []map[string]any {
	byHash := map[string][]string{}
	for _, raw := range observations {
		item := mapFromAny(raw)
		hash := stringFromMap(item, "document_sha256")
		rawURL := stringFromMap(item, "final_url")
		if hash != "" && rawURL != "" {
			byHash[hash] = append(byHash[hash], rawURL)
		}
	}
	out := []map[string]any{}
	for hash, urls := range byHash {
		if len(urls) > 1 {
			out = append(out, map[string]any{"document_sha256": hash, "relation": "mirror_or_repost", "independent_source_count": 1, "urls": urls})
		}
	}
	return out
}

func sourceDiscoveryScopeDistinctGroups(candidates []map[string]any) []map[string]any {
	groups := map[string]map[string]bool{}
	for _, candidate := range candidates {
		kind := strings.ToLower(stringFromMap(candidate, "kind"))
		identity := normalizeSourceCandidateValue(firstSourceCandidateValue(candidate, "name", "label", "subject", "entity", "source"))
		scope := normalizeSourceCandidateValue(firstSourceCandidateValue(candidate, "edition", "edition_id", "continuity", "branch", "time_scope"))
		if identity == "" || scope == "" {
			continue
		}
		key := kind + "\x00" + identity
		if groups[key] == nil {
			groups[key] = map[string]bool{}
		}
		groups[key][scope] = true
	}
	out := []map[string]any{}
	for key, scopes := range groups {
		if len(scopes) < 2 {
			continue
		}
		values := []string{}
		for scope := range scopes {
			values = append(values, scope)
		}
		sort.Strings(values)
		out = append(out, map[string]any{"identity_key": key, "classification": "scope_distinct_not_conflict", "scopes": values})
	}
	return out
}

func sourceDiscoveryReconciliation(candidates []map[string]any) ([]map[string]any, []map[string]any) {
	bySubject := map[string][]map[string]any{}
	uncertainties := []map[string]any{}
	for _, candidate := range candidates {
		if key := sourceCandidateSubjectKey(candidate); key != "" {
			bySubject[key] = append(bySubject[key], candidate)
		}
		if int64FromMap(candidate, "independent_source_count", 1) < 2 {
			uncertainties = append(uncertainties, map[string]any{"code": "single_source_candidate", "candidate_key": sourceCandidateLogicalKey(candidate), "kind": candidate["kind"], "label": firstSourceCandidateValue(candidate, "name", "label", "subject", "statement")})
		}
	}
	conflicts := []map[string]any{}
	for subject, items := range bySubject {
		assertions := map[string]bool{}
		for _, item := range items {
			if assertion := normalizeSourceCandidateValue(firstSourceCandidateValue(item, "statement", "claim_text", "value", "object", "target")); assertion != "" {
				assertions[assertion] = true
			}
		}
		if len(assertions) > 1 {
			conflicts = append(conflicts, map[string]any{"code": "direct_assertion_conflict", "subject_key": subject, "candidate_count": len(items), "candidates": mapsToAny(items)})
		}
	}
	return conflicts, uncertainties
}

func discoveryCoverageFromCandidates(domains []string, candidates []map[string]any, observations []any, deltas []map[string]any, exceptions []any, saturated, operationalLimit bool) map[string]any {
	counts := map[string]int{}
	for _, candidate := range candidates {
		switch strings.ToLower(stringFromMap(candidate, "kind")) {
		case "entity", "character":
			counts["entities"]++
		case "item":
			counts["items"]++
		case "location":
			counts["locations"]++
		case "faction":
			counts["factions"]++
		case "setting":
			counts["settings"]++
		case "event":
			counts["events"]++
		case "relation":
			counts["relations"]++
		case "claim":
			counts["identity"]++
		case "appearance":
			counts["visual_appearance"]++
		}
	}
	items := []map[string]any{}
	missing := []string{}
	for _, domain := range domains {
		status := "missing"
		if counts[domain] > 0 || (domain == "identity" && len(candidates) > 0) {
			status = "partial"
		}
		if status == "missing" {
			missing = append(missing, domain)
		}
		items = append(items, map[string]any{"domain": domain, "status": status, "candidate_count": counts[domain], "missing_topics": func() []string {
			if status == "missing" {
				return []string{domain + " evidence"}
			}
			return []string{}
		}()})
	}
	saturation := "insufficient_source_coverage"
	if saturated && len(missing) == 0 {
		saturation = "coverage_saturation"
	} else if operationalLimit {
		saturation = "operational_limit_reached"
	}
	return map[string]any{"contract": "source_discovery_coverage.v1", "saturation": saturation, "domains": items, "missing_domains": missing, "successful_sources": len(observations), "candidate_count": len(candidates), "coverage_delta": deltas, "exceptions": exceptions}
}

func coverageMissingDomains(coverage map[string]any) []string {
	return stringsFromAny(coverage["missing_domains"])
}

func sourceDiscoveryAdmissionPreview(candidates, conflicts, uncertainties []map[string]any) map[string]any {
	blocked := map[string]bool{}
	for _, conflict := range conflicts {
		for _, raw := range sliceFromAny(conflict["candidates"]) {
			blocked[sourceCandidateLogicalKey(mapFromAny(raw))] = true
		}
	}
	for _, uncertainty := range uncertainties {
		blocked[stringFromMap(uncertainty, "candidate_key")] = true
	}
	eligible := []any{}
	for _, candidate := range candidates {
		if !blocked[sourceCandidateLogicalKey(candidate)] && int64FromMap(candidate, "independent_source_count", 1) >= 2 {
			eligible = append(eligible, candidate)
		}
	}
	return map[string]any{
		"contract": "source_discovery_admission_preview.v1", "mode": "preview_only",
		"eligible_batch": eligible, "eligible_count": len(eligible),
		"exception_count": len(conflicts) + len(uncertainties), "requires_explicit_admission": true,
		"pack_candidate": map[string]any{"generation": "new", "review_status": "pending", "source_body_included": false, "candidate_count": len(eligible)},
	}
}

func sourceDiscoverySearchSummary(rounds []map[string]any) map[string]any {
	summary := map[string]any{"status": "completed", "round_count": len(rounds), "result_count": 0, "approved_source_count": 0, "excluded_result_count": 0, "rewritten_source_count": 0}
	for _, round := range rounds {
		diagnostics := mapFromAny(round["diagnostics"])
		for _, key := range []string{"result_count", "approved_source_count", "excluded_result_count", "rewritten_source_count"} {
			summary[key] = int64FromMap(summary, key, 0) + int64FromMap(diagnostics, key, 0)
		}
		if diagnostics["provider"] != nil {
			summary["provider"] = diagnostics["provider"]
		}
		if diagnostics["model"] != nil {
			summary["model"] = diagnostics["model"]
		}
	}
	if len(rounds) == 0 {
		summary["status"] = "not_run"
	}
	return summary
}

func validateSourceDiscoveryInput(input store.SourceDiscoveryInput) error {
	if strings.TrimSpace(input.WorkQuery) == "" {
		return errors.New("work_query is required")
	}
	allowed := map[string]bool{}
	for _, sourceType := range input.AllowedSourceTypes {
		sourceType = strings.TrimSpace(sourceType)
		switch sourceType {
		case "official_primary", "licensed_structured", "attributed_secondary", "community_wiki", "user_local":
			allowed[sourceType] = true
		default:
			return fmt.Errorf("unsupported allowed source type %q", sourceType)
		}
	}
	if len(allowed) == 0 {
		return errors.New("allowed_source_types is required")
	}
	for _, policy := range input.DomainPolicies {
		domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(policy.Domain)), ".")
		if domain == "" || strings.ContainsAny(domain, "/:@") {
			return errors.New("every domain policy requires a bare DNS domain")
		}
		if !allowed[strings.TrimSpace(policy.SourceType)] || strings.TrimSpace(policy.AccessClass) == "" {
			return errors.New("domain policy must use an approved source_type and access_class")
		}
	}
	for _, source := range input.Sources {
		if !allowed[strings.TrimSpace(source.SourceType)] {
			return fmt.Errorf("source type %q was not approved for this job", source.SourceType)
		}
		if strings.TrimSpace(source.AccessClass) == "" {
			return errors.New("every source requires access_class")
		}
	}
	return nil
}

func runSourceDiscovery(ctx context.Context, input store.SourceDiscoveryInput) (map[string]any, map[string]any, string) {
	domains := normalizedDiscoveryDomains(input.RequestedDomains)
	frontier := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		frontier = append(frontier, map[string]any{
			"query":  strings.TrimSpace(input.WorkQuery) + " " + domain,
			"domain": domain, "generated_by": "deterministic_scope_frontier",
		})
	}
	result := map[string]any{
		"contract": "source-discovery-result.v1", "query_frontier": frontier,
		"observations": []any{}, "section_candidates": []any{}, "exceptions": []any{},
		"admission_status": "pending",
	}
	if len(input.Sources) == 0 {
		result["termination_reason"] = "search_provider_required"
		coverage := discoveryCoverage(domains, 0, 0, "No approved source URL or configured search-provider result was supplied.")
		return result, coverage, "insufficient_source_coverage"
	}

	observations := []any{}
	sectionCandidates := []any{}
	exceptions := []any{}
	successful := 0
	for _, source := range input.Sources {
		if !source.PolicyConfirmed {
			exceptions = append(exceptions, map[string]any{"url": source.URL, "code": "manual_source_policy_confirmation_required"})
			continue
		}
		observation, sections, err := fetchDiscoverySource(ctx, source)
		if err != nil {
			exceptions = append(exceptions, map[string]any{"url": source.URL, "code": discoveryFetchErrorCode(err), "message": err.Error()})
			continue
		}
		delete(observation, "_raw_body")
		successful++
		observations = append(observations, observation)
		for _, section := range sections {
			section["source_url"] = source.URL
			section["source_type"] = source.SourceType
			section["review_state"] = "pending"
			sectionCandidates = append(sectionCandidates, section)
		}
	}
	result["observations"] = observations
	result["section_candidates"] = sectionCandidates
	result["exceptions"] = exceptions
	coverage := discoveryCoverage(domains, successful, len(sectionCandidates), "Fact extraction and independent-source reconciliation are still required.")
	if successful == 0 {
		result["termination_reason"] = "all_sources_blocked_or_failed"
		return result, coverage, "blocked_by_access_policy"
	}
	result["termination_reason"] = "insufficient_source_coverage"
	return result, coverage, "insufficient_source_coverage"
}

func fetchDiscoverySource(ctx context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
	parsed, err := validateDiscoveryURL(ctx, source.URL)
	if err != nil {
		return nil, nil, err
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           discoveryDialContext,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: 8 * time.Second,
		TLSHandshakeTimeout:   8 * time.Second,
		DisableKeepAlives:     true,
	}
	client := &http.Client{
		Transport: transport, Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("redirect limit exceeded")
			}
			_, err := validateDiscoveryURL(req.Context(), req.URL.String())
			return err
		},
	}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", SourceDiscoveryUserAgent)
	req.Header.Set("Accept", "text/html, application/json, text/plain;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, nil, errors.New("source content type is missing or invalid")
	}
	switch mediaType {
	case "text/html", "text/plain", "application/json", "application/ld+json":
	default:
		return nil, nil, fmt.Errorf("source MIME %q is not allowed", mediaType)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, sourceDiscoveryMaxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(body) > sourceDiscoveryMaxBytes {
		return nil, nil, errors.New("source response exceeds size limit")
	}
	if discoveryRobotsDenied(resp.Header, body, mediaType) {
		return nil, nil, errors.New("source access policy forbids automated analysis")
	}
	sum := sha256.Sum256(body)
	sections := discoverySections(body, mediaType)
	observation := map[string]any{
		"requested_url": parsed.String(), "final_url": resp.Request.URL.String(),
		"source_type": source.SourceType, "access_class": source.AccessClass,
		"policy_basis": "user_confirmed_public_source", "http_status": resp.StatusCode,
		"media_type": mediaType, "byte_length": len(body),
		"document_sha256": hex.EncodeToString(sum[:]), "retrieved_at": time.Now().UTC(),
		"raw_retention": "local_reference_document", "section_count": len(sections),
		"_raw_body": string(body),
	}
	if title := discoveryDocumentTitleFromSections(sections); title != "" {
		observation["document_title"] = title
	}
	profile := sourceDiscoveryProfileForURL(resp.Request.URL.String())
	observation["source_profile"] = profile
	if mediaType == "text/html" {
		observation["internal_source_links"] = discoveryHTMLInternalLinks(resp.Request.URL, body)
		visuals := []map[string]any{}
		visualExceptions := []map[string]any{}
		for _, asset := range discoveryHTMLImageAssets(resp.Request.URL, body) {
			visual, err := fetchDiscoveryImageObservation(ctx, stringFromMap(asset, "url"), stringFromMap(asset, "alt"))
			if err != nil {
				visualExceptions = append(visualExceptions, map[string]any{"url": asset["url"], "code": discoveryFetchErrorCode(err), "message": err.Error()})
				continue
			}
			visuals = append(visuals, visual)
		}
		observation["visual_observations"] = visuals
		observation["visual_exceptions"] = visualExceptions
	}
	return observation, sections, nil
}

func discoveryDocumentTitleFromSections(sections []map[string]any) string {
	for _, locatorType := range []string{"title", "h1"} {
		for _, section := range sections {
			locator := mapFromAny(section["locator"])
			if strings.EqualFold(stringFromMap(locator, "type"), locatorType) {
				if title := strings.Join(strings.Fields(stringFromMap(section, "excerpt")), " "); title != "" {
					return title
				}
			}
		}
	}
	return ""
}

func discoveryRobotsDenied(headers http.Header, body []byte, mediaType string) bool {
	policy := strings.ToLower(headers.Get("X-Robots-Tag"))
	if strings.Contains(policy, "noai") || strings.Contains(policy, "noindex") {
		return true
	}
	if mediaType != "text/html" {
		return false
	}
	head := strings.ToLower(string(body))
	if len(head) > 32768 {
		head = head[:32768]
	}
	return (strings.Contains(head, `name="robots"`) || strings.Contains(head, `name='robots'`)) && (strings.Contains(head, "noai") || strings.Contains(head, "noindex"))
}

func sourceDiscoveryProfileForURL(rawURL string) map[string]any {
	return map[string]any{"contract": "source-profile.v1", "profile_id": "generic_public_document", "profile_version": "1.0.0", "parser_strategy": "html_structural_v1", "review_state": "generic_policy", "raw_retention": "none", "executable_parser": false}
}

func discoveryHTMLInternalLinks(base *url.URL, body []byte) []string {
	if base == nil {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	out := []string{}
	seen := map[string]bool{}
	for len(out) < 20 {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "a") {
			continue
		}
		href := ""
		for _, attr := range start.Attr {
			if strings.EqualFold(attr.Name.Local, "href") {
				href = strings.TrimSpace(attr.Value)
				break
			}
		}
		ref, err := url.Parse(href)
		if err != nil || href == "" {
			continue
		}
		resolved := base.ResolveReference(ref)
		if !strings.EqualFold(strings.TrimSuffix(resolved.Hostname(), "."), strings.TrimSuffix(base.Hostname(), ".")) {
			continue
		}
		resolved.Fragment = ""
		key := canonicalDiscoveryURL(resolved.String())
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, resolved.String())
	}
	return out
}

func discoveryHTMLImageAssets(base *url.URL, body []byte) []map[string]any {
	if base == nil {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	out := []map[string]any{}
	seen := map[string]bool{}
	for len(out) < 3 {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "img") {
			continue
		}
		src, alt := "", ""
		for _, attr := range start.Attr {
			switch strings.ToLower(attr.Name.Local) {
			case "src":
				src = strings.TrimSpace(attr.Value)
			case "alt":
				alt = boundedDiscoveryText(attr.Value)
			}
		}
		ref, err := url.Parse(src)
		if err != nil || src == "" {
			continue
		}
		resolved := base.ResolveReference(ref)
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}
		key := canonicalDiscoveryURL(resolved.String())
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, map[string]any{"url": resolved.String(), "alt": alt})
	}
	return out
}

func fetchDiscoveryImageObservation(ctx context.Context, rawURL, alt string) (map[string]any, error) {
	parsed, err := validateDiscoveryURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: nil, DialContext: discoveryDialContext, ForceAttemptHTTP2: true, ResponseHeaderTimeout: 8 * time.Second, TLSHandshakeTimeout: 8 * time.Second, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit exceeded")
		}
		_, err := validateDiscoveryURL(req.Context(), req.URL.String())
		return err
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", SourceDiscoveryUserAgent)
	req.Header.Set("Accept", "image/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "image/") {
		return nil, errors.New("source MIME is not an image")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, sourceDiscoveryMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > sourceDiscoveryMaxBytes {
		return nil, errors.New("source response exceeds size limit")
	}
	sum := sha256.Sum256(body)
	return map[string]any{"source_url": rawURL, "final_url": resp.Request.URL.String(), "media_type": mediaType, "byte_length": len(body), "image_sha256": hex.EncodeToString(sum[:]), "alt_text": alt, "region_locator": map[string]any{"type": "whole_image"}, "raw_retention": "none", "redistribution": "not_included", "review_state": "pending", "admission_eligible": false}, nil
}

func validateDiscoveryURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("only absolute HTTP(S) source URLs are allowed")
	}
	if parsed.User != nil {
		return nil, errors.New("source URL credentials are forbidden")
	}
	port := parsed.Port()
	if port != "" && !((parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443")) {
		return nil, errors.New("source URL uses a disallowed port")
	}
	for key := range parsed.Query() {
		normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
		if strings.Contains(normalized, "token") || strings.Contains(normalized, "apikey") || strings.Contains(normalized, "password") || strings.Contains(normalized, "secret") {
			return nil, errors.New("source URL contains a secret-like query parameter")
		}
	}
	if err := validateDiscoveryHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	parsed.Fragment = ""
	return parsed, nil
}

func validateDiscoveryHost(ctx context.Context, host string) error {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return errors.New("private or local source host is forbidden")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return errors.New("source host could not be resolved")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errors.New("source host resolves to a forbidden network")
		}
	}
	return nil
}

func discoveryDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := validateDiscoveryHost(ctx, host); err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("source host could not be resolved for connection")
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func discoverySections(body []byte, mediaType string) []map[string]any {
	if mediaType == "text/html" {
		return discoveryHTMLSections(body)
	}
	if mediaType == "application/json" || mediaType == "application/ld+json" {
		var value any
		if json.Unmarshal(body, &value) == nil {
			encoded, _ := json.Marshal(value)
			sections := []map[string]any{}
			for index, excerpt := range discoveryTextChunks(string(encoded), 800) {
				locator := "$"
				if index > 0 {
					locator = fmt.Sprintf("$.chunk.%d", index+1)
				}
				sections = append(sections, map[string]any{"locator": map[string]any{"type": "json_document", "value": locator}, "excerpt": excerpt})
			}
			return sections
		}
		return nil
	}
	sections := []map[string]any{}
	for index, paragraph := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n\n") {
		for chunkIndex, excerpt := range discoveryTextChunks(paragraph, 800) {
			locator := fmt.Sprint(index + 1)
			if chunkIndex > 0 {
				locator = fmt.Sprintf("%d.%d", index+1, chunkIndex+1)
			}
			sections = append(sections, map[string]any{"locator": map[string]any{"type": "paragraph", "value": locator}, "excerpt": excerpt})
		}
	}
	return sections
}

func discoveryHTMLSections(body []byte) []map[string]any {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	allowed := map[string]bool{"title": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "p": true, "li": true, "td": true, "th": true, "figcaption": true}
	ignored := map[string]bool{
		"script": true, "style": true, "noscript": true, "svg": true, "template": true,
		"nav": true, "footer": true, "aside": true, "form": true, "button": true,
		"select": true, "option": true, "dialog": true,
	}
	sections := []map[string]any{}
	stack := []string{}
	activeTag := ""
	activeID := ""
	var text strings.Builder
	var anchorText strings.Builder
	anchorHref := ""
	anchorDepth := 0
	activeAnchors := []map[string]any{}
	counts := map[string]int{}
	ignoreDepth := 0
	seenSections := map[string]bool{}
	headings := make([]string, 6)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			tag := strings.ToLower(value.Name.Local)
			ignoredContainer := ignored[tag] || discoveryHTMLBoilerplateContainer(value)
			stackTag := tag
			if ignoredContainer {
				stackTag += "\x00ignored"
			}
			stack = append(stack, stackTag)
			if ignoredContainer {
				ignoreDepth++
			}
			if ignoreDepth == 0 && activeTag != "" {
				if anchorDepth > 0 {
					anchorDepth++
				} else if tag == "a" {
					anchorDepth = 1
					anchorText.Reset()
					anchorHref = ""
					for _, attr := range value.Attr {
						if strings.EqualFold(attr.Name.Local, "href") {
							anchorHref = strings.TrimSpace(attr.Value)
							break
						}
					}
				}
			}
			if ignoreDepth == 0 && allowed[tag] && activeTag == "" {
				activeTag = tag
				activeID = ""
				for _, attr := range value.Attr {
					if strings.EqualFold(attr.Name.Local, "id") {
						activeID = strings.TrimSpace(attr.Value)
						break
					}
				}
				text.Reset()
				activeAnchors = nil
			}
		case xml.CharData:
			if ignoreDepth == 0 && activeTag != "" {
				text.Write(value)
				text.WriteByte(' ')
				if anchorDepth > 0 {
					anchorText.Write(value)
					anchorText.WriteByte(' ')
				}
			}
		case xml.EndElement:
			tag := strings.ToLower(value.Name.Local)
			if ignoreDepth == 0 && anchorDepth > 0 {
				anchorDepth--
				if anchorDepth == 0 {
					if anchor := strings.Join(strings.Fields(anchorText.String()), " "); anchor != "" {
						activeAnchors = append(activeAnchors, map[string]any{"text": anchor, "href": anchorHref})
					}
					anchorText.Reset()
					anchorHref = ""
				}
			}
			if ignoreDepth == 0 && activeTag == tag {
				counts[tag]++
				for chunkIndex, excerpt := range discoveryTextChunks(text.String(), 800) {
					signature := normalizeDiscoveryAnalysisText(excerpt)
					if signature == "" || seenSections[signature] {
						continue
					}
					seenSections[signature] = true
					locator := fmt.Sprint(counts[tag])
					if chunkIndex > 0 {
						locator = fmt.Sprintf("%d.%d", counts[tag], chunkIndex+1)
					}
					if level := discoveryHTMLHeadingLevel(tag); level > 0 {
						headings[level-1] = excerpt
						for index := level; index < len(headings); index++ {
							headings[index] = ""
						}
					}
					locatorValue := map[string]any{"type": tag, "value": locator}
					if activeID != "" {
						locatorValue["id"] = activeID
					}
					section := map[string]any{
						"locator": locatorValue, "excerpt": excerpt,
						"section_kind": discoveryHTMLSectionKind(tag),
					}
					if headingPath := discoveryHTMLHeadingPath(headings); headingPath != "" {
						section["heading_path"] = headingPath
					}
					if len(activeAnchors) > 0 {
						section["anchors"] = mapsToAny(activeAnchors)
					}
					sections = append(sections, section)
				}
				activeTag = ""
				activeID = ""
				text.Reset()
			}
			if (ignored[tag] || discoveryHTMLBoilerplateEnd(stack, tag)) && ignoreDepth > 0 {
				ignoreDepth--
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return sections
}

func discoveryHTMLHeadingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	return 0
}

func discoveryHTMLHeadingPath(headings []string) string {
	path := make([]string, 0, len(headings))
	for _, heading := range headings {
		if value := strings.TrimSpace(heading); value != "" {
			path = append(path, value)
		}
	}
	return strings.Join(path, " > ")
}

func discoveryHTMLSectionKind(tag string) string {
	if discoveryHTMLHeadingLevel(tag) > 0 || tag == "title" {
		return "heading"
	}
	switch tag {
	case "li":
		return "list_item"
	case "td", "th":
		return "table_cell"
	case "figcaption":
		return "caption"
	default:
		return "prose"
	}
}

func discoveryHTMLBoilerplateContainer(element xml.StartElement) bool {
	// Theme classes on document roots often contain UI words such as
	// "search" or "menu" even though the element contains the article body.
	// Only descendants may be classified as boilerplate from CSS metadata.
	switch strings.ToLower(element.Name.Local) {
	case "html", "body", "main", "article", "section":
		return false
	}
	for _, attr := range element.Attr {
		name := strings.ToLower(attr.Name.Local)
		if name != "class" && name != "id" && name != "role" {
			continue
		}
		for _, token := range strings.FieldsFunc(strings.ToLower(attr.Value), func(r rune) bool {
			return !(unicode.IsLetter(r) || unicode.IsNumber(r))
		}) {
			switch token {
			case "navigation", "navbar", "menu", "menubar", "sidebar", "footer", "toolbar", "breadcrumb", "breadcrumbs", "pagination", "search", "login", "signup", "share", "comments", "comment", "advertisement", "ads":
				return true
			}
		}
	}
	return false
}

// The XML token stream does not repeat start-element attributes on close. The
// stack marker records whether the matching start element opened an ignored
// container so nested boilerplate is closed at the correct depth.
func discoveryHTMLBoilerplateEnd(stack []string, tag string) bool {
	if len(stack) == 0 {
		return false
	}
	return strings.TrimSuffix(stack[len(stack)-1], "\x00ignored") == tag && strings.HasSuffix(stack[len(stack)-1], "\x00ignored")
}

func boundedDiscoveryText(value string) string {
	chunks := discoveryTextChunks(value, 800)
	if len(chunks) == 0 {
		return ""
	}
	return chunks[0]
}

func discoverySectionSignature(section map[string]any) string {
	text := normalizeDiscoveryAnalysisText(stringFromMap(section, "excerpt"))
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func normalizeDiscoveryAnalysisText(value string) string {
	var normalized strings.Builder
	spacePending := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if spacePending && normalized.Len() > 0 {
				normalized.WriteByte(' ')
			}
			normalized.WriteRune(r)
			spacePending = false
			continue
		}
		spacePending = true
	}
	return normalized.String()
}

func appendDiscoveryEquivalentEvidence(representative, duplicate map[string]any) {
	evidence := map[string]any{
		"source_url": stringFromMap(duplicate, "source_url"), "source_type": stringFromMap(duplicate, "source_type"),
		"document_sha256": stringFromMap(duplicate, "document_sha256"), "locator": mapFromAny(duplicate["locator"]),
		"evidence_excerpt": stringFromMap(duplicate, "excerpt"),
	}
	items := sliceMapFromAny(representative["equivalent_evidence"])
	key := sourceEvidenceKey(evidence)
	for _, item := range items {
		if sourceEvidenceKey(item) == key {
			return
		}
	}
	representative["equivalent_evidence"] = mapsToAny(append(items, evidence))
}

func prioritizeDiscoverySections(sections []any) []any {
	if len(sections) < 2 {
		return sections
	}
	tokenFrequency := map[string]int{}
	sectionTokens := make([][]string, len(sections))
	for index, raw := range sections {
		text := normalizeDiscoveryAnalysisText(stringFromMap(mapFromAny(raw), "excerpt"))
		seen := map[string]bool{}
		for _, token := range strings.Fields(text) {
			if len([]rune(token)) < 2 || seen[token] {
				continue
			}
			seen[token] = true
			sectionTokens[index] = append(sectionTokens[index], token)
			tokenFrequency[token]++
		}
	}
	type rankedSection struct {
		value any
		score float64
		index int
	}
	ranked := make([]rankedSection, 0, len(sections))
	for index, raw := range sections {
		item := mapFromAny(raw)
		score := 0.0
		for _, token := range sectionTokens[index] {
			score += 1.0 / float64(tokenFrequency[token])
		}
		if len(sectionTokens[index]) > 0 {
			score /= float64(len(sectionTokens[index]))
		}
		locatorType := strings.ToLower(stringFromMap(mapFromAny(item["locator"]), "type"))
		sectionKind := strings.ToLower(stringFromMap(item, "section_kind"))
		switch {
		case sectionKind == "heading" || (strings.HasPrefix(locatorType, "h") && len(locatorType) == 2):
			score += 8
		case sectionKind == "list_item" || sectionKind == "table_cell" || locatorType == "li" || locatorType == "td" || locatorType == "th":
			score += 5
		case sectionKind == "caption" || locatorType == "figcaption":
			score += 2
		}
		ranked = append(ranked, rankedSection{value: raw, score: score, index: index})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]any, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.value)
	}
	return out
}

func discoveryTextChunks(value string, maxRunes int) []string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return nil
	}
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return []string{value}
	}
	chunks := make([]string, 0, (len(runes)+maxRunes-1)/maxRunes)
	for len(runes) > 0 {
		n := maxRunes
		if len(runes) < n {
			n = len(runes)
		}
		chunks = append(chunks, string(runes[:n]))
		runes = runes[n:]
	}
	return chunks
}

func normalizedDiscoveryDomains(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		result = []string{"identity", "entities", "locations", "items", "factions", "settings", "events", "relations"}
	}
	sort.Strings(result)
	return result
}

func discoveryCoverage(domains []string, successfulSources, sectionCount int, note string) map[string]any {
	items := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		status := "missing"
		if successfulSources > 0 && domain == "identity" {
			status = "partial"
		}
		items = append(items, map[string]any{"domain": domain, "status": status, "missing_topics": []string{"fact extraction and evidence reconciliation"}})
	}
	return map[string]any{
		"contract": "source_discovery_coverage.v1", "saturation": "insufficient_source_coverage",
		"successful_sources": successfulSources, "section_candidates": sectionCount,
		"domains": items, "note": note,
	}
}

func discoveryFetchErrorCode(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "forbidden") || strings.Contains(message, "disallowed") || strings.Contains(message, "credentials") || strings.Contains(message, "secret") {
		return "blocked_by_access_policy"
	}
	if strings.Contains(message, "mime") || strings.Contains(message, "content type") {
		return "unsupported_content_type"
	}
	if strings.Contains(message, "size limit") {
		return "response_too_large"
	}
	return "fetch_failed"
}
