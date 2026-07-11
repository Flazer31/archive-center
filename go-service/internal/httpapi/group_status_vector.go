package httpapi

import (
	"context"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
	archvector "github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) indexStatusSchemaProposal(ctx context.Context, proposal store.StatusSchemaProposal, clientMeta map[string]any, explicitVector []float32) statusSchemaVectorIndex {
	docID := statusSchemaVectorDocumentID(proposal)
	sourceRowID := strconv.FormatInt(proposal.ID, 10)
	out := statusSchemaVectorIndex{
		Status:      "skipped",
		Attempted:   false,
		DocumentID:  docID,
		Tier:        "status_schema_proposal",
		SourceTable: "status_schema_proposals",
		SourceRowID: sourceRowID,
	}
	if proposal.ID <= 0 || strings.TrimSpace(proposal.ChatSessionID) == "" {
		out.Reason = "proposal_row_not_persisted"
		return out
	}
	if s.Vector == nil {
		out.Reason = "vector_not_configured"
		return out
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		out.Reason = "chroma_endpoint_not_configured"
		return out
	}
	documentText := statusSchemaVectorDocumentText(proposal)
	out.DocumentChars = len([]rune(documentText))
	embedding := append([]float32(nil), explicitVector...)
	if len(embedding) > 0 {
		out.EmbeddingMode = "request_vector"
	}
	if len(embedding) == 0 {
		for _, key := range []string{"status_schema_vector", "chroma_document_vector", "schema_embedding"} {
			if candidate := clientMetaFloat32Vector(clientMeta, key); len(candidate) > 0 {
				embedding = candidate
				out.EmbeddingMode = "client_meta:" + key
				break
			}
		}
	}
	if len(embedding) == 0 {
		cfg := s.completeTurnExtractionConfig(clientMeta).Embedder
		if !cfg.hasConfig() {
			out.Reason = "missing_embedding_config"
			return out
		}
		embeddingJSON, model, err := callEmbedding(ctx, cfg, documentText)
		if err != nil {
			out.Status = "failed"
			out.Attempted = true
			out.Reason = "embedding_error: " + err.Error()
			return out
		}
		embedding = parseFloat32JSONList(embeddingJSON)
		out.EmbeddingMode = "backend_embedding"
		out.EmbeddingModel = model
	}
	if len(embedding) == 0 {
		out.Reason = "empty_embedding"
		return out
	}
	out.Attempted = true
	doc := archvector.VectorDocument{
		ID:            docID,
		Embedding:     embedding,
		Tier:          "status_schema_proposal",
		ChatSessionID: proposal.ChatSessionID,
		SourceTable:   "status_schema_proposals",
		SourceRowID:   sourceRowID,
		SchemaVersion: statusSchemaContractVersion,
		DocumentText:  documentText,
	}
	if err := s.Vector.Upsert(ctx, proposal.ChatSessionID, []archvector.VectorDocument{doc}); err != nil {
		out.Status = "failed"
		out.Reason = "vector_upsert_error: " + err.Error()
		return out
	}
	out.Status = "ok"
	return out
}

func (s *Server) indexStatusSchemaDefinition(ctx context.Context, definition store.StatusSchemaDefinition, clientMeta map[string]any, explicitVector []float32) statusSchemaVectorIndex {
	docID := statusSchemaDefinitionVectorDocumentID(definition)
	sourceRowID := strconv.FormatInt(definition.ID, 10)
	out := statusSchemaVectorIndex{
		Status:      "skipped",
		Attempted:   false,
		DocumentID:  docID,
		Tier:        "status_schema_definition",
		SourceTable: "status_schema_registry",
		SourceRowID: sourceRowID,
	}
	if definition.ID <= 0 || strings.TrimSpace(definition.ChatSessionID) == "" {
		out.Reason = "registry_row_not_persisted"
		return out
	}
	if s.Vector == nil {
		out.Reason = "vector_not_configured"
		return out
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		out.Reason = "chroma_endpoint_not_configured"
		return out
	}
	documentText := statusSchemaDefinitionVectorDocumentText(definition)
	out.DocumentChars = len([]rune(documentText))
	embedding := append([]float32(nil), explicitVector...)
	if len(embedding) > 0 {
		out.EmbeddingMode = "request_vector"
	}
	if len(embedding) == 0 {
		for _, key := range []string{"status_registry_vector", "status_schema_vector", "schema_embedding"} {
			if candidate := clientMetaFloat32Vector(clientMeta, key); len(candidate) > 0 {
				embedding = candidate
				out.EmbeddingMode = "client_meta:" + key
				break
			}
		}
	}
	if len(embedding) == 0 {
		cfg := s.completeTurnExtractionConfig(clientMeta).Embedder
		if !cfg.hasConfig() {
			out.Reason = "missing_embedding_config"
			return out
		}
		embeddingJSON, model, err := callEmbedding(ctx, cfg, documentText)
		if err != nil {
			out.Status = "failed"
			out.Attempted = true
			out.Reason = "embedding_error: " + err.Error()
			return out
		}
		embedding = parseFloat32JSONList(embeddingJSON)
		out.EmbeddingMode = "backend_embedding"
		out.EmbeddingModel = model
	}
	if len(embedding) == 0 {
		out.Reason = "empty_embedding"
		return out
	}
	out.Attempted = true
	doc := archvector.VectorDocument{
		ID:            docID,
		Embedding:     embedding,
		Tier:          "status_schema_definition",
		ChatSessionID: definition.ChatSessionID,
		SourceTable:   "status_schema_registry",
		SourceRowID:   sourceRowID,
		SchemaVersion: statusSchemaRegistryContractVersion,
		DocumentText:  documentText,
	}
	if err := s.Vector.Upsert(ctx, definition.ChatSessionID, []archvector.VectorDocument{doc}); err != nil {
		out.Status = "failed"
		out.Reason = "vector_upsert_error: " + err.Error()
		return out
	}
	out.Status = "ok"
	return out
}

func statusSchemaVectorDocumentID(proposal store.StatusSchemaProposal) string {
	if proposal.ID <= 0 {
		return ""
	}
	return "status_schema_proposal:" + strings.TrimSpace(proposal.ChatSessionID) + ":" + strconv.FormatInt(proposal.ID, 10)
}

func statusSchemaVectorDocumentText(proposal store.StatusSchemaProposal) string {
	parts := []string{
		"Archive Center status schema proposal",
		"schema_name: " + strings.TrimSpace(proposal.SchemaName),
		"ruleset_label: " + strings.TrimSpace(proposal.RulesetLabel),
		"input_channel: " + strings.TrimSpace(proposal.InputChannel),
		"proposal_state: " + strings.TrimSpace(proposal.ProposalState),
		"schema_json:",
		strings.TrimSpace(proposal.SchemaJSON),
	}
	if provenance := strings.TrimSpace(proposal.ProvenanceJSON); provenance != "" {
		parts = append(parts, "provenance_json:", provenance)
	}
	if note := strings.TrimSpace(proposal.ReviewNote); note != "" {
		parts = append(parts, "review_note:", note)
	}
	if reviewer := strings.TrimSpace(proposal.Reviewer); reviewer != "" {
		parts = append(parts, "reviewer: "+reviewer)
	}
	return strings.Join(parts, "\n")
}

func statusSchemaDefinitionVectorDocumentID(definition store.StatusSchemaDefinition) string {
	if definition.ID <= 0 {
		return ""
	}
	return "status_schema_definition:" + strings.TrimSpace(definition.ChatSessionID) + ":" + strconv.FormatInt(definition.ID, 10)
}

func statusSchemaDefinitionVectorDocumentText(definition store.StatusSchemaDefinition) string {
	parts := []string{
		"Archive Center status schema definition",
		"schema_name: " + strings.TrimSpace(definition.SchemaName),
		"ruleset_label: " + strings.TrimSpace(definition.RulesetLabel),
		"status_key: " + strings.TrimSpace(definition.StatusKey),
		"label: " + strings.TrimSpace(definition.Label),
		"owner_scope: " + strings.TrimSpace(definition.OwnerScope),
		"value_kind: " + strings.TrimSpace(definition.ValueKind),
		"registry_state: " + strings.TrimSpace(definition.RegistryState),
	}
	if definition.BoundsJSON != "" {
		parts = append(parts, "bounds_json:", definition.BoundsJSON)
	}
	if definition.OptionsJSON != "" {
		parts = append(parts, "options_json:", definition.OptionsJSON)
	}
	if definition.DefaultValueJSON != "" {
		parts = append(parts, "default_value_json:", definition.DefaultValueJSON)
	}
	return strings.Join(parts, "\n")
}
