package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const referenceVectorSchemaVersion = "reference.v1"

type referenceVectorRequest struct {
	ContinuityID string         `json:"continuity_id"`
	Query        string         `json:"query"`
	Limit        int            `json:"limit"`
	ClientMeta   map[string]any `json:"client_meta"`
}

type referenceVectorMaterial struct {
	Kind     string
	ID       string
	Text     string
	Metadata map[string]any
}

type referenceVectorSearchResult struct {
	ChromaRank       int            `json:"chroma_rank"`
	DocumentID       string         `json:"document_id"`
	ReferenceKind    string         `json:"reference_kind"`
	SourceID         string         `json:"source_id"`
	DocumentText     string         `json:"document_text"`
	Distance         *float64       `json:"distance,omitempty"`
	CosineSimilarity *float64       `json:"cosine_similarity,omitempty"`
	Metadata         map[string]any `json:"metadata"`
}

func (s *Server) handleReferenceVectorReindex(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	if _, ok := s.referenceVectorStore(w); !ok {
		return
	}
	var req referenceVectorRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	workID := strings.TrimSpace(r.PathValue("work_id"))
	continuityID := strings.TrimSpace(req.ContinuityID)
	if err := validateReferenceVectorScope(r.Context(), ref, workID, continuityID); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	embedder := s.completeTurnExtractionConfig(req.ClientMeta).Embedder
	if !embedder.hasConfig() {
		writeError(w, http.StatusBadRequest, "embedding_config_missing", "embedding provider, api key, endpoint, and model are required")
		return
	}
	if s.AdminJobs == nil {
		s.AdminJobs = newAdminJobManager()
	}
	jobKey := strings.Join([]string{workID, continuityID, strings.ToLower(strings.TrimSpace(embedder.Provider)), strings.TrimSpace(embedder.Model)}, "\x00")
	job := s.AdminJobs.start("reference_vector_reindex", jobKey, map[string]any{
		"work_id":         workID,
		"continuity_id":   continuityID,
		"collection":      s.Cfg.ReferenceChromaCollection,
		"embedding_model": embedder.Model,
	}, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		return s.runReferenceVectorReindex(ctx, ref, workID, continuityID, embedder, progress)
	})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleReferenceVectorSearch(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	vectorStore, ok := s.referenceVectorStore(w)
	if !ok {
		return
	}
	querier, ok := vectorStore.(vector.ExactMetadataQuerier)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "reference_vector_exact_query_unavailable", "configured vector store does not expose exact Chroma query results")
		return
	}
	var req referenceVectorRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	workID := strings.TrimSpace(r.PathValue("work_id"))
	continuityID := strings.TrimSpace(req.ContinuityID)
	queryText := strings.TrimSpace(req.Query)
	if queryText == "" {
		writeBadRequest(w, "query is required")
		return
	}
	if err := validateReferenceVectorScope(r.Context(), ref, workID, continuityID); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	embedder := s.completeTurnExtractionConfig(req.ClientMeta).Embedder
	if !embedder.hasConfig() {
		writeError(w, http.StatusBadRequest, "embedding_config_missing", "embedding provider, api key, endpoint, and model are required")
		return
	}
	embeddingJSON, model, err := callEmbedding(r.Context(), embedder, queryText)
	if err != nil {
		writeError(w, http.StatusBadGateway, "reference_query_embedding_failed", err.Error())
		return
	}
	queryVector := parseFloat32JSONList(embeddingJSON)
	if len(queryVector) == 0 {
		writeError(w, http.StatusBadGateway, "reference_query_embedding_empty", "embedding provider returned an empty vector")
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}
	approvedIDs, err := loadApprovedReferenceVectorIDs(r.Context(), ref, workID, continuityID)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	if err := validateReferenceQueryEmbeddingSpace(r.Context(), vectorStore, workID, continuityID, approvedIDs, embedder.Provider, model); err != nil {
		writeError(w, http.StatusConflict, "reference_embedding_model_mismatch", err.Error())
		return
	}
	where := map[string]any{"$and": []map[string]any{
		{"work_id": workID},
		{"continuity_id": continuityID},
		{"review_status": "approved"},
	}}
	rawResults, err := querier.QueryExact(r.Context(), vector.ExactQuery{Embedding: queryVector, Limit: limit, Where: where})
	if errors.Is(err, vector.ErrNotFound) {
		writeJSON(w, http.StatusOK, referenceVectorSearchResponse(workID, continuityID, model, len(queryVector), nil, 0))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "reference_vector_query_failed", err.Error())
		return
	}
	results := make([]referenceVectorSearchResult, 0, len(rawResults))
	filtered := 0
	for _, raw := range rawResults {
		meta := raw.Document.Metadata
		if strings.TrimSpace(fmt.Sprint(meta["work_id"])) != workID ||
			strings.TrimSpace(fmt.Sprint(meta["continuity_id"])) != continuityID ||
			!strings.EqualFold(strings.TrimSpace(fmt.Sprint(meta["review_status"])), "approved") {
			filtered++
			continue
		}
		if !approvedIDs[raw.Document.ID] {
			filtered++
			continue
		}
		item := referenceVectorSearchResult{
			ChromaRank:    raw.ChromaRank,
			DocumentID:    raw.Document.ID,
			ReferenceKind: strings.TrimSpace(fmt.Sprint(meta["reference_kind"])),
			SourceID:      strings.TrimSpace(fmt.Sprint(meta["source_id"])),
			DocumentText:  raw.Document.DocumentText,
			Metadata:      meta,
		}
		if raw.DistanceAvailable {
			value := raw.Distance
			item.Distance = &value
		}
		if raw.CosineAvailable {
			value := raw.CosineSimilarity
			item.CosineSimilarity = &value
		}
		results = append(results, item)
	}
	writeJSON(w, http.StatusOK, referenceVectorSearchResponse(workID, continuityID, model, len(queryVector), results, filtered))
}

func validateReferenceQueryEmbeddingSpace(ctx context.Context, vectorStore vector.VectorStore, workID, continuityID string, approvedIDs map[string]bool, provider, model string) error {
	lister, ok := vectorStore.(vector.DocumentLister)
	if !ok {
		return errors.New("reference vector store cannot verify the indexed embedding model")
	}
	docs, err := lister.ListDocuments(ctx, workID)
	if err != nil {
		return err
	}
	wantProvider := strings.ToLower(strings.TrimSpace(provider))
	wantModel := strings.TrimSpace(model)
	for _, doc := range docs {
		meta := doc.Metadata
		if strings.TrimSpace(fmt.Sprint(meta["work_id"])) != workID || strings.TrimSpace(fmt.Sprint(meta["continuity_id"])) != continuityID || !approvedIDs[doc.ID] {
			continue
		}
		indexedProvider := strings.ToLower(strings.TrimSpace(fmt.Sprint(meta["embedding_provider"])))
		indexedModel := strings.TrimSpace(fmt.Sprint(meta["embedding_model"]))
		if indexedProvider == "<nil>" {
			indexedProvider = ""
		}
		if indexedModel == "<nil>" {
			indexedModel = ""
		}
		if indexedProvider == "" || indexedModel == "" {
			return fmt.Errorf("reference index has no embedding provider/model metadata; reindex before search")
		}
		if indexedProvider != wantProvider || !strings.EqualFold(indexedModel, wantModel) {
			return fmt.Errorf("reference index uses %s/%s but current search uses %s/%s; reindex with the current embedding settings", indexedProvider, indexedModel, wantProvider, wantModel)
		}
	}
	return nil
}

func loadApprovedReferenceVectorIDs(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string) (map[string]bool, error) {
	ids := map[string]bool{}
	timeline, err := ref.ListReferenceTimelineNodes(ctx, workID, continuityID, "approved")
	if err != nil {
		return nil, err
	}
	for _, item := range timeline {
		ids[referenceVectorDocumentID("timeline", item.NodeID)] = true
	}
	entities, err := ref.ListReferenceEntities(ctx, workID, continuityID, "approved")
	if err != nil {
		return nil, err
	}
	for _, item := range entities {
		ids[referenceVectorDocumentID("entity", item.EntityID)] = true
	}
	claims, err := ref.ListReferenceClaims(ctx, workID, continuityID, "approved", "")
	if err != nil {
		return nil, err
	}
	for _, item := range claims {
		ids[referenceVectorDocumentID("claim", item.ClaimID)] = true
	}
	return ids, nil
}

func (s *Server) handleReferenceVectorStatus(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	vectorStore, ok := s.referenceVectorStore(w)
	if !ok {
		return
	}
	workID := strings.TrimSpace(r.PathValue("work_id"))
	continuityID := strings.TrimSpace(r.URL.Query().Get("continuity_id"))
	if err := validateReferenceVectorScope(r.Context(), ref, workID, continuityID); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	lister, ok := vectorStore.(vector.DocumentLister)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "reference_vector_listing_unavailable", "configured vector store cannot verify indexed documents")
		return
	}
	docs, err := lister.ListDocuments(r.Context(), workID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "reference_vector_status_failed", err.Error())
		return
	}
	counts := map[string]int{"claim": 0, "entity": 0, "timeline": 0}
	models := map[string]bool{}
	approvedIDs, err := loadApprovedReferenceVectorIDs(r.Context(), ref, workID, continuityID)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	total := 0
	valid := 0
	stale := 0
	seenApproved := map[string]bool{}
	for _, doc := range docs {
		meta := doc.Metadata
		if strings.TrimSpace(fmt.Sprint(meta["work_id"])) != workID || strings.TrimSpace(fmt.Sprint(meta["continuity_id"])) != continuityID {
			continue
		}
		total++
		if !approvedIDs[doc.ID] {
			stale++
			continue
		}
		valid++
		seenApproved[doc.ID] = true
		counts[strings.TrimSpace(fmt.Sprint(meta["reference_kind"]))]++
		if model := strings.TrimSpace(fmt.Sprint(meta["embedding_model"])); model != "" {
			models[model] = true
		}
	}
	health, err := vectorStore.Health(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "reference_vector_health_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"work_id":           workID,
		"continuity_id":     continuityID,
		"collection":        health.Collection,
		"collection_status": health.Status,
		"indexed":           valid,
		"indexed_total":     total,
		"current_approved":  len(approvedIDs),
		"stale_indexed":     stale,
		"missing_approved":  len(approvedIDs) - len(seenApproved),
		"counts":            counts,
		"embedding_models":  sortedStringSet(models),
		"score_contract":    referenceVectorScoreContract(),
	})
}

func (s *Server) referenceVectorStore(w http.ResponseWriter) (vector.VectorStore, bool) {
	if !s.Cfg.ChromaEnabled || strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		writeError(w, http.StatusServiceUnavailable, "reference_chromadb_not_configured", "ChromaDB is required for the reference vector index")
		return nil, false
	}
	if s.ReferenceVectorOpenError != nil {
		writeError(w, http.StatusServiceUnavailable, "reference_chromadb_open_failed", s.ReferenceVectorOpenError.Error())
		return nil, false
	}
	if s.ReferenceVector == nil {
		writeError(w, http.StatusServiceUnavailable, "reference_chromadb_not_initialized", "reference vector store is not initialized")
		return nil, false
	}
	return s.ReferenceVector, true
}

func validateReferenceVectorScope(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string) error {
	if workID == "" || continuityID == "" {
		return store.ErrInvalidReference
	}
	work, err := ref.GetReferenceWork(ctx, workID)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(work.Status), "disabled") {
		return store.ErrInvalidReference
	}
	continuities, err := ref.ListReferenceContinuities(ctx, workID)
	if err != nil {
		return err
	}
	for _, item := range continuities {
		if item.ContinuityID == continuityID && strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *Server) runReferenceVectorReindex(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string, embedder completeTurnEmbeddingConfig, progress adminJobProgressFunc) (map[string]any, error) {
	vectorStore := s.ReferenceVector
	if vectorStore == nil {
		return nil, vector.ErrNotEnabled
	}
	progress(map[string]any{"stage": "load_approved_material", "processed": 0, "candidate_count": 0, "progress_percent": 0, "work_id": workID, "continuity_id": continuityID})
	materials, err := loadReferenceVectorMaterials(ctx, ref, workID, continuityID)
	if err != nil {
		return nil, err
	}
	docs := make([]vector.VectorDocument, 0, len(materials))
	for i, material := range materials {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(map[string]any{"stage": "embed_approved_material", "processed": i, "candidate_count": len(materials), "progress_percent": adminJobProgressPercent(i, len(materials)), "reference_kind": material.Kind, "source_id": material.ID})
		embeddingJSON, model, embedErr := callEmbedding(ctx, embedder, material.Text)
		if embedErr != nil {
			return nil, fmt.Errorf("reference embedding failed for %s %s: %w", material.Kind, material.ID, embedErr)
		}
		embedding := parseFloat32JSONList(embeddingJSON)
		if len(embedding) == 0 {
			return nil, fmt.Errorf("reference embedding empty for %s %s", material.Kind, material.ID)
		}
		metadata := make(map[string]any, len(material.Metadata)+5)
		for key, value := range material.Metadata {
			metadata[key] = value
		}
		metadata["work_id"] = workID
		metadata["continuity_id"] = continuityID
		metadata["reference_kind"] = material.Kind
		metadata["source_id"] = material.ID
		metadata["review_status"] = "approved"
		metadata["embedding_provider"] = strings.ToLower(strings.TrimSpace(embedder.Provider))
		metadata["embedding_model"] = model
		docs = append(docs, vector.VectorDocument{
			ID:               referenceVectorDocumentID(material.Kind, material.ID),
			Embedding:        embedding,
			Tier:             "reference_" + material.Kind,
			ChatSessionID:    workID,
			SourceTable:      referenceVectorSourceTable(material.Kind),
			SourceRowID:      material.ID,
			SchemaVersion:    referenceVectorSchemaVersion,
			DocumentText:     material.Text,
			SearchTextPolicy: "approved_reference_material",
			Metadata:         metadata,
		})
	}

	progress(map[string]any{"stage": "upsert_reference_collection", "processed": len(docs), "candidate_count": len(docs), "progress_percent": 90, "collection": s.Cfg.ReferenceChromaCollection})
	if len(docs) > 0 {
		if err := vectorStore.Upsert(ctx, workID, docs); err != nil {
			return nil, err
		}
	}

	lister, listOK := vectorStore.(vector.DocumentLister)
	deleter, deleteOK := vectorStore.(vector.DocumentDeleter)
	if !listOK || !deleteOK {
		return nil, errors.New("reference vector store must support document listing and deletion")
	}
	existing, err := lister.ListDocuments(ctx, workID)
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{}
	for _, doc := range docs {
		expected[doc.ID] = true
	}
	stale := []string{}
	for _, doc := range existing {
		meta := doc.Metadata
		if strings.TrimSpace(fmt.Sprint(meta["work_id"])) != workID || strings.TrimSpace(fmt.Sprint(meta["continuity_id"])) != continuityID {
			continue
		}
		if !expected[doc.ID] {
			stale = append(stale, doc.ID)
		}
	}
	if len(stale) > 0 {
		progress(map[string]any{"stage": "delete_stale_reference_vectors", "processed": len(stale), "candidate_count": len(stale), "progress_percent": 95})
		if err := deleter.DeleteDocuments(ctx, stale); err != nil {
			return nil, err
		}
	}

	verifiedDocs, err := lister.ListDocuments(ctx, workID)
	if err != nil {
		return nil, err
	}
	verified := map[string]bool{}
	for _, doc := range verifiedDocs {
		meta := doc.Metadata
		if strings.TrimSpace(fmt.Sprint(meta["work_id"])) == workID && strings.TrimSpace(fmt.Sprint(meta["continuity_id"])) == continuityID {
			verified[doc.ID] = true
		}
	}
	missing := []string{}
	for id := range expected {
		if !verified[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return nil, fmt.Errorf("reference vector verification failed: %d expected documents missing", len(missing))
	}
	progress(map[string]any{"stage": "verified", "processed": len(expected), "candidate_count": len(expected), "progress_percent": 100, "stale_deleted": len(stale)})
	return map[string]any{
		"status":          "ok",
		"work_id":         workID,
		"continuity_id":   continuityID,
		"collection":      s.Cfg.ReferenceChromaCollection,
		"indexed":         len(expected),
		"stale_deleted":   len(stale),
		"embedding_model": embedder.Model,
		"schema_version":  referenceVectorSchemaVersion,
		"score_contract":  referenceVectorScoreContract(),
	}, nil
}

func (s *Server) runReferenceAutomaticVectorIndex(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string, embedder completeTurnEmbeddingConfig, progress adminJobProgressFunc) map[string]any {
	if !embedder.hasConfig() {
		return map[string]any{"status": "skipped", "reason": "embedding_config_missing"}
	}
	if s.ReferenceVectorOpenError != nil {
		return map[string]any{"status": "failed", "error": s.ReferenceVectorOpenError.Error()}
	}
	if s.ReferenceVector == nil {
		return map[string]any{"status": "failed", "error": "reference vector store is not initialized"}
	}
	result, err := s.runReferenceVectorReindex(ctx, ref, workID, continuityID, embedder, progress)
	if err != nil {
		return map[string]any{"status": "failed", "error": err.Error()}
	}
	result["status"] = "completed"
	result["trigger"] = "automatic_after_review"
	return result
}

func loadReferenceVectorMaterials(ctx context.Context, ref store.ReferenceLibraryStore, workID, continuityID string) ([]referenceVectorMaterial, error) {
	timeline, err := ref.ListReferenceTimelineNodes(ctx, workID, continuityID, "approved")
	if err != nil {
		return nil, err
	}
	entities, err := ref.ListReferenceEntities(ctx, workID, continuityID, "approved")
	if err != nil {
		return nil, err
	}
	claims, err := ref.ListReferenceClaims(ctx, workID, continuityID, "approved", "")
	if err != nil {
		return nil, err
	}
	materials := make([]referenceVectorMaterial, 0, len(timeline)+len(entities)+len(claims))
	entityNames := map[string][]string{}
	for _, item := range timeline {
		metadata := map[string]any{"branch_key": item.BranchKey, "node_kind": item.NodeKind, "ordinal": item.Ordinal}
		text := referenceVectorText("Timeline", item.Label, referenceMetadataText(item.MetadataJSON, "evidence_excerpt", "description", "summary"))
		if text != "" {
			materials = append(materials, referenceVectorMaterial{Kind: "timeline", ID: item.NodeID, Text: text, Metadata: metadata})
		}
	}
	for _, item := range entities {
		aliases, aliasErr := ref.ListReferenceEntityAliases(ctx, item.EntityID)
		if aliasErr != nil {
			return nil, aliasErr
		}
		aliasTexts := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			if value := strings.TrimSpace(alias.AliasText); value != "" && !strings.EqualFold(value, item.CanonicalName) {
				aliasTexts = append(aliasTexts, value)
			}
		}
		detail := strings.TrimSpace(item.DescriptionText)
		if len(aliasTexts) > 0 {
			detail = strings.TrimSpace(detail + "\nAliases: " + strings.Join(aliasTexts, ", "))
		}
		entityNames[item.EntityID] = append([]string{item.CanonicalName}, aliasTexts...)
		text := referenceVectorText(item.EntityType, item.CanonicalName, detail)
		if text != "" {
			materials = append(materials, referenceVectorMaterial{Kind: "entity", ID: item.EntityID, Text: text, Metadata: map[string]any{"entity_type": item.EntityType, "canonical_name": item.CanonicalName, "aliases": aliasTexts}})
		}
	}
	for _, item := range claims {
		detail := strings.TrimSpace(item.ClaimText)
		if evidence := strings.TrimSpace(item.EvidenceExcerpt); evidence != "" {
			detail += "\nEvidence: " + evidence
		}
		metadata := map[string]any{
			"claim_type":          item.ClaimType,
			"subject_entity_id":   item.SubjectEntityID,
			"temporal_scope":      item.TemporalScope,
			"valid_from_node_id":  item.ValidFromNodeID,
			"valid_to_node_id":    item.ValidToNodeID,
			"reveal_from_node_id": item.RevealFromNodeID,
			"branch_key":          item.BranchKey,
			"knowledge_scope":     item.KnowledgeScope,
			"confidence":          item.Confidence,
		}
		if names := entityNames[item.SubjectEntityID]; len(names) > 0 {
			metadata["subject_names"] = names
		}
		text := referenceVectorText("Claim", item.ClaimType, detail)
		if text != "" {
			materials = append(materials, referenceVectorMaterial{Kind: "claim", ID: item.ClaimID, Text: text, Metadata: metadata})
		}
	}
	sort.Slice(materials, func(i, j int) bool {
		if materials[i].Kind != materials[j].Kind {
			return materials[i].Kind < materials[j].Kind
		}
		return materials[i].ID < materials[j].ID
	})
	return materials, nil
}

func referenceVectorText(kind, title, detail string) string {
	parts := []string{}
	if kind = strings.TrimSpace(kind); kind != "" {
		parts = append(parts, "["+kind+"]")
	}
	if title = strings.TrimSpace(title); title != "" {
		parts = append(parts, title)
	}
	if detail = strings.TrimSpace(detail); detail != "" {
		parts = append(parts, detail)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func referenceMetadataText(raw string, keys ...string) string {
	var metadata map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &metadata) != nil {
		return ""
	}
	parts := []string{}
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(metadata[key])); value != "" && value != "<nil>" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n")
}

func referenceVectorDocumentID(kind, sourceID string) string {
	return "reference_" + strings.TrimSpace(kind) + ":" + strings.TrimSpace(sourceID)
}

func referenceVectorSourceTable(kind string) string {
	switch strings.TrimSpace(kind) {
	case "claim":
		return "reference_claims"
	case "entity":
		return "reference_entities"
	case "timeline":
		return "reference_timeline_nodes"
	default:
		return "reference_library"
	}
}

func referenceVectorSearchResponse(workID, continuityID, model string, dimension int, results []referenceVectorSearchResult, filtered int) map[string]any {
	if results == nil {
		results = []referenceVectorSearchResult{}
	}
	return map[string]any{
		"status":              "ok",
		"work_id":             workID,
		"continuity_id":       continuityID,
		"embedding_model":     model,
		"query_embedding_dim": dimension,
		"results":             results,
		"count":               len(results),
		"filtered_mismatch":   filtered,
		"score_contract":      referenceVectorScoreContract(),
	}
}

func referenceVectorScoreContract() map[string]any {
	return map[string]any{
		"ranking":               "chromadb_response_order",
		"distance":              "raw_chromadb_distance",
		"cosine_similarity":     "computed_only_from_query_and_returned_stored_embedding",
		"normalized_similarity": "not_generated",
		"fixed_rank_scores":     false,
		"client_rerank":         false,
	}
}

func sortedStringSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
