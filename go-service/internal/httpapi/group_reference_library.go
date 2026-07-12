package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const referenceDocumentMaxBytes = 4 << 20
const referenceDocumentMaxChunks = 64

type referenceWorkCreateRequest struct {
	Title           string         `json:"title"`
	WorkType        string         `json:"work_type"`
	DefaultLanguage string         `json:"default_language"`
	Metadata        map[string]any `json:"metadata"`
}

type referenceContinuityCreateRequest struct {
	ContinuityKey string         `json:"continuity_key"`
	Label         string         `json:"label"`
	Metadata      map[string]any `json:"metadata"`
}

type referenceDocumentCreateRequest struct {
	ContinuityID string         `json:"continuity_id"`
	SourceType   string         `json:"source_type"`
	SourceURI    string         `json:"source_uri"`
	Filename     string         `json:"filename"`
	Content      string         `json:"content"`
	Metadata     map[string]any `json:"metadata"`
}

type referenceExtractRequest struct {
	ClientMeta map[string]any `json:"client_meta"`
}

type referenceReviewRequest struct {
	Items []struct {
		Kind     string `json:"kind"`
		ID       string `json:"id"`
		Decision string `json:"decision"`
	} `json:"items"`
}

func (s *Server) registerReferenceLibraryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /reference-works", s.handleReferenceWorksList)
	mux.HandleFunc("POST /reference-works", s.handleReferenceWorkCreate)
	mux.HandleFunc("GET /reference-works/{work_id}/continuities", s.handleReferenceContinuitiesList)
	mux.HandleFunc("POST /reference-works/{work_id}/continuities", s.handleReferenceContinuityCreate)
	mux.HandleFunc("POST /reference-works/{work_id}/documents", s.handleReferenceDocumentCreate)
	mux.HandleFunc("POST /reference-works/{work_id}/documents/{document_id}/extract", s.handleReferenceDocumentExtract)
	mux.HandleFunc("GET /reference-works/{work_id}/review-candidates", s.handleReferenceReviewCandidates)
	mux.HandleFunc("POST /reference-works/{work_id}/review", s.handleReferenceReviewApply)
	mux.HandleFunc("GET /reference-jobs/{job_id}", s.handleReferenceJob)
}

func (s *Server) referenceLibraryStore(w http.ResponseWriter) (store.ReferenceLibraryStore, bool) {
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "reference_library_unavailable", "MariaDB authority reference library is not available")
		return nil, false
	}
	return ref, true
}

func (s *Server) handleReferenceWorksList(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	items, err := ref.ListReferenceWorks(r.Context(), strings.TrimSpace(r.URL.Query().Get("status")), 200)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "works": items})
}

func (s *Server) handleReferenceWorkCreate(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceWorkCreateRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeBadRequest(w, "title is required")
		return
	}
	workID, err := newReferenceID()
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	metadata, _ := json.Marshal(req.Metadata)
	item := &store.ReferenceWork{WorkID: workID, Title: strings.TrimSpace(req.Title), WorkType: strings.TrimSpace(req.WorkType), DefaultLanguage: strings.TrimSpace(req.DefaultLanguage), Status: "draft", MetadataJSON: string(metadata)}
	if err := ref.CreateReferenceWork(r.Context(), item); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "work": item})
}

func (s *Server) handleReferenceContinuitiesList(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	items, err := ref.ListReferenceContinuities(r.Context(), r.PathValue("work_id"))
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "continuities": items})
}

func (s *Server) handleReferenceContinuityCreate(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceContinuityCreateRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		writeBadRequest(w, "label is required")
		return
	}
	key := strings.TrimSpace(req.ContinuityKey)
	if key == "" {
		key = "main"
	}
	metadata, _ := json.Marshal(req.Metadata)
	item := &store.ReferenceContinuity{ContinuityID: referenceStableID("continuity", r.PathValue("work_id"), key), WorkID: r.PathValue("work_id"), ContinuityKey: key, Label: strings.TrimSpace(req.Label), Status: "active", MetadataJSON: string(metadata)}
	if err := ref.UpsertReferenceContinuity(r.Context(), item); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "continuity": item})
}

func (s *Server) handleReferenceDocumentCreate(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceDocumentCreateRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	content := strings.TrimSpace(req.Content)
	if strings.TrimSpace(req.ContinuityID) == "" || content == "" {
		writeBadRequest(w, "continuity_id and content are required")
		return
	}
	if len([]byte(content)) > referenceDocumentMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "reference_document_too_large", "document exceeds 4 MiB")
		return
	}
	hashBytes := sha256.Sum256([]byte(content))
	contentHash := hex.EncodeToString(hashBytes[:])
	documentID := referenceStableID("document", r.PathValue("work_id"), req.ContinuityID, contentHash)
	if existing, err := ref.GetReferenceDocument(r.Context(), documentID); err == nil {
		if existing.WorkID != r.PathValue("work_id") || existing.ContinuityID != strings.TrimSpace(req.ContinuityID) {
			writeError(w, http.StatusConflict, "reference_document_scope_conflict", "document hash belongs to another reference scope")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "document": existing, "existing": true})
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeReferenceStoreError(w, err)
		return
	}
	provenance := map[string]any{"filename": strings.TrimSpace(req.Filename), "source_uri": strings.TrimSpace(req.SourceURI), "metadata": req.Metadata}
	provenanceJSON, _ := json.Marshal(provenance)
	item := &store.ReferenceDocument{DocumentID: documentID, WorkID: r.PathValue("work_id"), ContinuityID: strings.TrimSpace(req.ContinuityID), SourceType: defaultReferenceString(req.SourceType, "file"), SourceURI: strings.TrimSpace(req.SourceURI), ContentHash: contentHash, RawRetention: "full", RawText: content, ImportStatus: "pending", ProvenanceJSON: string(provenanceJSON)}
	if err := ref.SaveReferenceDocument(r.Context(), item); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "document": item})
}

func (s *Server) handleReferenceDocumentExtract(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceExtractRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	documentID := strings.TrimSpace(r.PathValue("document_id"))
	doc, err := ref.GetReferenceDocument(r.Context(), documentID)
	if err != nil || doc.WorkID != strings.TrimSpace(r.PathValue("work_id")) {
		if err == nil {
			err = store.ErrNotFound
		}
		writeReferenceStoreError(w, err)
		return
	}
	cfg := s.completeTurnExtractionConfig(req.ClientMeta).Critic
	if !cfg.hasConfig() {
		writeError(w, http.StatusBadRequest, "critic_config_missing", "critic provider, api key, endpoint, and model are required")
		return
	}
	if s.AdminJobs == nil {
		s.AdminJobs = newAdminJobManager()
	}
	job := s.AdminJobs.start("reference_extract", documentID, map[string]any{"work_id": doc.WorkID, "document_id": documentID, "continuity_id": doc.ContinuityID}, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		return s.runReferenceExtractionJob(ctx, ref, doc, cfg, progress)
	})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleReferenceReviewCandidates(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	workID := r.PathValue("work_id")
	continuityID := strings.TrimSpace(r.URL.Query().Get("continuity_id"))
	timeline, err := ref.ListReferenceTimelineNodes(r.Context(), workID, continuityID, "")
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	entities, err := ref.ListReferenceEntities(r.Context(), workID, continuityID, "")
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	claims, err := ref.ListReferenceClaims(r.Context(), workID, continuityID, "pending", "")
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	timeline = filterPendingTimeline(timeline)
	entities = filterPendingEntities(entities)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "timeline": timeline, "entities": entities, "claims": claims, "count": len(timeline) + len(entities) + len(claims)})
}

func (s *Server) handleReferenceReviewApply(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceReviewRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	updated := 0
	for _, item := range req.Items {
		if err := ref.UpdateReferenceCandidateReview(r.Context(), r.PathValue("work_id"), strings.TrimSpace(item.Kind), strings.TrimSpace(item.ID), strings.TrimSpace(item.Decision)); err != nil {
			writeReferenceStoreError(w, err)
			return
		}
		updated++
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
}

func (s *Server) handleReferenceJob(w http.ResponseWriter, r *http.Request) {
	if s.AdminJobs == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "not_found"})
		return
	}
	job, ok := s.AdminJobs.get(r.PathValue("job_id"))
	if !ok || fmt.Sprint(job["kind"]) != "reference_extract" {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func decodeReferenceJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, referenceDocumentMaxBytes+(1<<20))
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeBadRequest(w, err.Error())
		return false
	}
	return true
}

func writeReferenceStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "reference_not_found", err.Error())
	case errors.Is(err, store.ErrReferenceConflict):
		writeError(w, http.StatusConflict, "reference_conflict", err.Error())
	case errors.Is(err, store.ErrReferenceWorkInUse):
		writeError(w, http.StatusConflict, "reference_work_in_use", err.Error())
	case errors.Is(err, store.ErrInvalidReference):
		writeBadRequest(w, err.Error())
	default:
		writeInternalError(w, err.Error())
	}
}

func referenceStableID(namespace string, values ...string) string {
	sum := sha256.Sum256([]byte(namespace + "\x00" + strings.Join(values, "\x00")))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}

func newReferenceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32], nil
}

func defaultReferenceString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func filterPendingTimeline(items []store.ReferenceTimelineNode) []store.ReferenceTimelineNode {
	out := []store.ReferenceTimelineNode{}
	for _, item := range items {
		if item.ReviewStatus == "pending" {
			out = append(out, item)
		}
	}
	return out
}

func filterPendingEntities(items []store.ReferenceEntity) []store.ReferenceEntity {
	out := []store.ReferenceEntity{}
	for _, item := range items {
		if item.ReviewStatus == "pending" {
			out = append(out, item)
		}
	}
	return out
}

func callReferenceExtractor(ctx context.Context, cfg completeTurnLLMConfig, doc *store.ReferenceDocument, chunk string, chunkIndex, chunkTotal int) (map[string]any, error) {
	systemPrompt := `You extract reusable original-work reference data. Return one valid JSON object only. Never invent missing chronology. Unknown chronology must remain unknown, never timeless. All candidates remain pending review.`
	userPrompt := fmt.Sprintf(`Work ID: %s
Continuity ID: %s
Document: %s
Chunk: %d/%d

Return this shape:
{"timeline":[{"node_key":"","label":"","ordinal":0,"branch_key":"main","node_kind":"event","parent_node_key":"","evidence_excerpt":""}],"entities":[{"entity_type":"character|location|item|faction","canonical_name":"","description":"","aliases":[""],"evidence_excerpt":""}],"claims":[{"claim_type":"character|relationship|world_rule|event|item|location","subject":"","claim_text":"","evidence_excerpt":"","temporal_scope":"timeless|bounded|event","valid_from_node_key":"","valid_to_node_key":"","reveal_from_node_key":"","branch_key":"main","knowledge_scope":"public_world|entity_scoped|narrator_only","knowers":[""],"confidence":0.0}],"warnings":[""]}

Rules: factual concise summaries, no prose continuation, no ads/navigation, no markdown fences, no future-point guessing.

SOURCE:
%s`, doc.WorkID, doc.ContinuityID, doc.SourceURI, chunkIndex+1, chunkTotal, chunk)
	maxTokens := cfg.MaxTokens
	if maxTokens < 2400 {
		maxTokens = 2400
	}
	maxCompletion := cfg.MaxCompletionTokens
	if maxCompletion < 2400 {
		maxCompletion = maxTokens
	}
	temp := cfg.Temperature
	if temp > 0.3 {
		temp = 0.2
	}
	req := dto.ProxyPluginMainRequest{APIKey: &cfg.APIKey, Endpoint: &cfg.Endpoint, Model: &cfg.Model, Provider: &cfg.Provider, Messages: []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}}, MaxTokens: &maxTokens, MaxCompletionTokens: &maxCompletion, Temperature: &temp, TimeoutMs: &cfg.TimeoutMs}
	applyProxyOverridesFromLLMConfig(&req, cfg)
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return nil, err
	}
	return parseJSONFromLLMContent(chatCompletionText(upstream))
}

func splitReferenceDocument(raw string, maxRunes int) []string {
	runes := []rune(raw)
	if maxRunes <= 0 {
		maxRunes = 16000
	}
	chunks := []string{}
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

func (s *Server) runReferenceExtractionJob(ctx context.Context, ref store.ReferenceLibraryStore, doc *store.ReferenceDocument, cfg completeTurnLLMConfig, progress adminJobProgressFunc) (map[string]any, error) {
	if strings.TrimSpace(doc.RawText) == "" {
		return nil, errors.New("reference_document_raw_text_missing")
	}
	if err := ref.UpdateReferenceDocumentStatus(ctx, doc.DocumentID, "extracting"); err != nil {
		return nil, err
	}
	chunks := splitReferenceDocument(doc.RawText, 16000)
	if len(chunks) > referenceDocumentMaxChunks {
		_ = ref.UpdateReferenceDocumentStatus(ctx, doc.DocumentID, "failed")
		return nil, fmt.Errorf("reference_document_requires_split: %d chunks exceeds limit %d", len(chunks), referenceDocumentMaxChunks)
	}
	counts := map[string]int{"timeline": 0, "entities": 0, "claims": 0}
	warnings := []string{}
	succeeded := 0
	for i, chunk := range chunks {
		progress(map[string]any{"stage": "critic_extract", "processed": i, "candidate_count": len(chunks), "progress_percent": adminJobProgressPercent(i, len(chunks)), "chunk": i + 1})
		parsed, err := callReferenceExtractor(ctx, cfg, doc, chunk, i, len(chunks))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("chunk %d: %v", i+1, err))
			continue
		}
		chunkCounts, saveWarnings, err := saveReferenceExtractionCandidates(ctx, ref, doc, parsed, i)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("chunk %d save: %v", i+1, err))
			continue
		}
		for key, value := range chunkCounts {
			counts[key] += value
		}
		warnings = append(warnings, saveWarnings...)
		succeeded++
	}
	if succeeded == 0 {
		_ = ref.UpdateReferenceDocumentStatus(ctx, doc.DocumentID, "failed")
		return map[string]any{"counts": counts, "warnings": warnings}, errors.New("reference_extraction_all_chunks_failed")
	}
	status := "parsed"
	if len(warnings) > 0 {
		status = "parsed_with_warnings"
	}
	if err := ref.UpdateReferenceDocumentStatus(ctx, doc.DocumentID, status); err != nil {
		return nil, err
	}
	progress(map[string]any{"stage": "pending_review", "processed": len(chunks), "candidate_count": len(chunks), "progress_percent": 100})
	return map[string]any{"status": status, "document_id": doc.DocumentID, "counts": counts, "warnings": warnings, "pending_review": counts["timeline"] + counts["entities"] + counts["claims"]}, nil
}

func saveReferenceExtractionCandidates(ctx context.Context, ref store.ReferenceLibraryStore, doc *store.ReferenceDocument, parsed map[string]any, chunkIndex int) (map[string]int, []string, error) {
	counts := map[string]int{"timeline": 0, "entities": 0, "claims": 0}
	warnings := append([]string{}, stringSliceFromAny(parsed["warnings"])...)
	timelineMap := map[string]string{}
	existingTimeline, err := ref.ListReferenceTimelineNodes(ctx, doc.WorkID, doc.ContinuityID, "")
	if err != nil {
		return counts, warnings, err
	}
	for _, node := range existingTimeline {
		timelineMap[strings.ToLower(strings.TrimSpace(node.NodeKey))] = node.NodeID
	}
	timelineCandidates := []map[string]any{}
	for _, raw := range sliceFromAny(parsed["timeline"]) {
		item := mapFromAny(raw)
		key := strings.TrimSpace(stringFromMap(item, "node_key"))
		label := strings.TrimSpace(stringFromMap(item, "label"))
		if key == "" || label == "" {
			continue
		}
		branch := defaultReferenceString(stringFromMap(item, "branch_key"), "main")
		nodeID := referenceStableID("timeline", doc.WorkID, doc.ContinuityID, branch, key)
		timelineMap[strings.ToLower(key)] = nodeID
		item["resolved_node_id"] = nodeID
		timelineCandidates = append(timelineCandidates, item)
	}
	for _, item := range timelineCandidates {
		key := strings.TrimSpace(stringFromMap(item, "node_key"))
		nodeID := stringFromMap(item, "resolved_node_id")
		parentID := timelineMap[strings.ToLower(strings.TrimSpace(stringFromMap(item, "parent_node_key")))]
		metadata := referenceCandidateMetadata(doc.DocumentID, chunkIndex, stringFromMap(item, "evidence_excerpt"))
		node := &store.ReferenceTimelineNode{NodeID: nodeID, WorkID: doc.WorkID, ContinuityID: doc.ContinuityID, NodeKey: key, Label: strings.TrimSpace(stringFromMap(item, "label")), Ordinal: int64(intFromAny(item["ordinal"], 0)), ParentNodeID: parentID, BranchKey: defaultReferenceString(stringFromMap(item, "branch_key"), "main"), NodeKind: defaultReferenceString(stringFromMap(item, "node_kind"), "event"), MetadataJSON: metadata, ReviewStatus: "pending"}
		if err := ref.UpsertReferenceTimelineNode(ctx, node); err != nil {
			return counts, warnings, err
		}
		counts["timeline"]++
	}
	entityMap := map[string]string{}
	existingEntities, err := ref.ListReferenceEntities(ctx, doc.WorkID, doc.ContinuityID, "")
	if err != nil {
		return counts, warnings, err
	}
	for _, entity := range existingEntities {
		entityMap[strings.ToLower(strings.TrimSpace(entity.CanonicalName))] = entity.EntityID
		aliases, aliasErr := ref.ListReferenceEntityAliases(ctx, entity.EntityID)
		if aliasErr != nil {
			return counts, warnings, aliasErr
		}
		for _, alias := range aliases {
			entityMap[strings.ToLower(strings.TrimSpace(alias.AliasText))] = entity.EntityID
		}
	}
	for _, raw := range sliceFromAny(parsed["entities"]) {
		item := mapFromAny(raw)
		name := strings.TrimSpace(stringFromMap(item, "canonical_name"))
		if name == "" {
			continue
		}
		entityID := referenceStableID("entity", doc.WorkID, doc.ContinuityID, strings.ToLower(name))
		entityMap[strings.ToLower(name)] = entityID
		metadata := referenceCandidateMetadata(doc.DocumentID, chunkIndex, stringFromMap(item, "evidence_excerpt"))
		entity := &store.ReferenceEntity{EntityID: entityID, WorkID: doc.WorkID, ContinuityID: doc.ContinuityID, EntityType: defaultReferenceString(stringFromMap(item, "entity_type"), "character"), CanonicalName: name, DescriptionText: stringFromMap(item, "description"), MetadataJSON: metadata, ReviewStatus: "pending"}
		if err := ref.UpsertReferenceEntity(ctx, entity); err != nil {
			return counts, warnings, err
		}
		for _, alias := range stringSliceFromAny(item["aliases"]) {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			_ = ref.UpsertReferenceEntityAlias(ctx, &store.ReferenceEntityAlias{WorkID: doc.WorkID, ContinuityID: doc.ContinuityID, EntityID: entityID, AliasText: alias, NormalizedAlias: strings.ToLower(alias)})
		}
		counts["entities"]++
	}
	for idx, raw := range sliceFromAny(parsed["claims"]) {
		item := mapFromAny(raw)
		text := strings.TrimSpace(stringFromMap(item, "claim_text"))
		if text == "" {
			continue
		}
		subjectName := strings.ToLower(strings.TrimSpace(stringFromMap(item, "subject")))
		claimID := referenceStableID("claim", doc.DocumentID, fmt.Sprint(chunkIndex), fmt.Sprint(idx), text)
		claim := &store.ReferenceClaim{ClaimID: claimID, WorkID: doc.WorkID, ContinuityID: doc.ContinuityID, DocumentID: doc.DocumentID, ClaimType: defaultReferenceString(stringFromMap(item, "claim_type"), "event"), SubjectEntityID: entityMap[subjectName], ClaimText: text, EvidenceExcerpt: truncateRunes(stringFromMap(item, "evidence_excerpt"), 800), TemporalScope: defaultReferenceString(stringFromMap(item, "temporal_scope"), "bounded"), ValidFromNodeID: timelineMap[strings.ToLower(stringFromMap(item, "valid_from_node_key"))], ValidToNodeID: timelineMap[strings.ToLower(stringFromMap(item, "valid_to_node_key"))], RevealFromNodeID: timelineMap[strings.ToLower(stringFromMap(item, "reveal_from_node_key"))], BranchKey: defaultReferenceString(stringFromMap(item, "branch_key"), "main"), KnowledgeScope: defaultReferenceString(stringFromMap(item, "knowledge_scope"), "public_world"), Confidence: floatFromAny(item["confidence"]), ReviewStatus: "pending", MetadataJSON: referenceCandidateMetadata(doc.DocumentID, chunkIndex, "")}
		if claim.TemporalScope != "timeless" && claim.ValidFromNodeID == "" {
			warnings = append(warnings, "claim chronology unresolved: "+truncateRunes(text, 100))
		}
		if err := ref.UpsertReferenceClaim(ctx, claim); err != nil {
			return counts, warnings, err
		}
		knowers := []string{}
		for _, name := range stringSliceFromAny(item["knowers"]) {
			if id := entityMap[strings.ToLower(strings.TrimSpace(name))]; id != "" {
				knowers = append(knowers, id)
			}
		}
		if err := ref.ReplaceReferenceClaimKnowers(ctx, claimID, knowers); err != nil {
			return counts, warnings, err
		}
		counts["claims"]++
	}
	return counts, warnings, nil
}

func referenceCandidateMetadata(documentID string, chunkIndex int, evidenceExcerpt string) string {
	metadata, _ := json.Marshal(map[string]any{
		"document_id":      strings.TrimSpace(documentID),
		"chunk_index":      chunkIndex,
		"evidence_excerpt": truncateRunes(strings.TrimSpace(evidenceExcerpt), 800),
	})
	return string(metadata)
}
