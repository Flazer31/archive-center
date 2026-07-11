package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) upsertMemoryVector(ctx context.Context, sid string, turnIndex int, mem *store.Memory, documentText string, embedding []float32, result *artifactSaveResult) {
	if result == nil {
		return
	}
	if len(embedding) == 0 {
		if result.VectorStatus == "not_requested" {
			result.VectorStatus = "missing_embedding"
		}
		return
	}
	if s.Vector == nil {
		result.VectorStatus = "vector_not_configured"
		return
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		result.VectorStatus = "vector_not_configured"
		return
	}
	sourceRowID := strconv.FormatInt(mem.ID, 10)
	if mem.ID <= 0 {
		sourceRowID = fmt.Sprintf("turn_%d_memory", turnIndex)
	}
	searchBuild := memorySearchTextBuild{}
	if mem != nil {
		searchBuild = memorySearchTextFromMemory(*mem)
	}
	documentText = strings.TrimSpace(documentText)
	if documentText == "" {
		documentText = strings.TrimSpace(searchBuild.Text)
	}
	languageMeta := map[string]string{}
	if mem != nil {
		languageMeta = memoryVectorLanguageMetadata(*mem)
	}
	doc := vector.VectorDocument{
		ID:                    fmt.Sprintf("memory:%s:%s", sid, sourceRowID),
		Embedding:             embedding,
		Tier:                  "memory",
		ChatSessionID:         sid,
		SourceTable:           "memories",
		SourceRowID:           sourceRowID,
		SchemaVersion:         "memory.v2",
		DocumentText:          documentText,
		SearchTextPolicy:      extractionFirstNonEmpty(languageMeta["search_text_policy"], languageMemorySearchPolicy),
		RawLanguage:           languageMeta["raw_language"],
		SummaryLanguage:       languageMeta["summary_language"],
		SessionOutputLanguage: languageMeta["session_output_language"],
		AliasCount:            searchBuild.AliasCount,
	}
	vectorStartedAt := time.Now()
	err := s.Vector.Upsert(ctx, sid, []vector.VectorDocument{doc})
	result.addTiming("vector_upsert", vectorStartedAt)
	if err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_upsert_failed")
		return
	}
	result.VectorsUpserted++
	result.VectorsMemoryUpserted++
	result.VectorStatus = "ok"
}

func (s *Server) upsertDerivedArtifactVector(ctx context.Context, sid string, turnIndex int, tier, sourceTable string, sourceRowID int64, schemaVersion, documentText string, embCfg completeTurnEmbeddingConfig, result *artifactSaveResult) {
	if result == nil {
		return
	}
	tier = strings.TrimSpace(tier)
	sourceTable = strings.TrimSpace(sourceTable)
	documentText = strings.TrimSpace(documentText)
	if tier == "" || sourceTable == "" || documentText == "" {
		return
	}
	if sourceRowID <= 0 {
		result.VectorStatus = "missing_source_row_id"
		result.Warnings = append(result.Warnings, "vector_"+tier+"_source_row_id_missing")
		return
	}
	if !embCfg.hasConfig() {
		if result.VectorStatus == "not_requested" {
			result.VectorStatus = "missing_embedding_config"
		}
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_config_missing")
		return
	}
	if s.Vector == nil || strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		result.VectorStatus = "vector_not_configured"
		return
	}
	embeddingStartedAt := time.Now()
	emb, _, err := callEmbedding(ctx, embCfg, documentText)
	result.addTiming("embedding", embeddingStartedAt)
	if err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_failed")
		return
	}
	embedding := parseFloat32JSONList(emb)
	if len(embedding) == 0 {
		result.VectorStatus = "empty_embedding"
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_empty")
		return
	}
	rowID := strconv.FormatInt(sourceRowID, 10)
	doc := vector.VectorDocument{
		ID:               fmt.Sprintf("%s:%s:%s", tier, sid, rowID),
		Embedding:        embedding,
		Tier:             tier,
		ChatSessionID:    sid,
		SourceTable:      sourceTable,
		SourceRowID:      rowID,
		SchemaVersion:    schemaVersion,
		DocumentText:     documentText,
		SearchTextPolicy: "derived_artifact_search_text.v1",
	}
	vectorStartedAt := time.Now()
	err = s.Vector.Upsert(ctx, sid, []vector.VectorDocument{doc})
	result.addTiming("vector_upsert", vectorStartedAt)
	if err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_"+tier+"_upsert_failed")
		return
	}
	result.VectorsUpserted++
	switch tier {
	case "evidence":
		result.VectorsEvidenceUpserted++
	case "world_rule":
		result.VectorsWorldRuleUpserted++
	}
	result.VectorStatus = "ok"
}

func directEvidenceVectorDocumentText(ev store.DirectEvidence) string {
	parts := []string{}
	if kind := strings.TrimSpace(ev.EvidenceKind); kind != "" {
		parts = append(parts, "kind: "+kind)
	}
	if text := strings.TrimSpace(ev.EvidenceText); text != "" {
		parts = append(parts, text)
	}
	if ev.SourceTurnStart > 0 || ev.SourceTurnEnd > 0 || ev.TurnAnchor > 0 {
		parts = append(parts, fmt.Sprintf("turns: %d-%d anchor:%d", ev.SourceTurnStart, ev.SourceTurnEnd, ev.TurnAnchor))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func worldRuleVectorDocumentText(wr store.WorldRule) string {
	parts := []string{}
	for _, part := range []string{
		strings.TrimSpace(wr.Scope),
		strings.TrimSpace(wr.ScopeName),
		strings.TrimSpace(wr.Category),
		strings.TrimSpace(wr.Key),
		strings.TrimSpace(wr.ValueJSON),
	} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func callEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string) (string, string, error) {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "":
		return "", "", errors.New("embedding provider is required")
	case "ollama":
		return callOllamaEmbedding(ctx, cfg, input)
	case "gemini":
		return callGeminiEmbedding(ctx, cfg, input, false)
	case "vertex":
		return callGeminiEmbedding(ctx, cfg, input, true)
	case "voyageai":
		return callOpenAICompatibleEmbedding(ctx, cfg, input, normalizeVoyageEmbeddingEndpoint(cfg.Endpoint), true)
	case "openai", "custom":
		return callOpenAICompatibleEmbedding(ctx, cfg, input, normalizeEmbeddingEndpoint(cfg.Endpoint), false)
	default:
		return "", "", fmt.Errorf("unsupported embedding provider %q", provider)
	}
}

func callOllamaEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string) (string, string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint = strings.TrimSuffix(endpoint, "/v1")
	}
	if !strings.HasSuffix(endpoint, "/api/embed") {
		endpoint += "/api/embed"
	}
	payload, _ := json.Marshal(map[string]any{
		"model": cfg.Model,
		"input": input,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("embedding upstream returned %s", resp.Status)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", err
	}
	embedding := data["embedding"]
	if embedding == nil {
		rows := sliceFromAny(data["embeddings"])
		if len(rows) > 0 {
			embedding = rows[0]
		}
	}
	if embedding == nil {
		return "", "", errors.New("embedding_data_empty")
	}
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), cfg.Model, nil
}

func callOpenAICompatibleEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string, endpoint string, arrayInput bool) (string, string, error) {
	body := map[string]any{"model": cfg.Model, "input": input}
	if arrayInput {
		body["input"] = []string{input}
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("embedding upstream returned %s", resp.Status)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", err
	}
	rows := sliceFromAny(data["data"])
	if len(rows) == 0 {
		return "", "", errors.New("embedding_data_empty")
	}
	embedding := mapFromAny(rows[0])["embedding"]
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), extractionFirstNonEmpty(extractionStringFromAny(data["model"]), cfg.Model), nil
}

func callGeminiEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string, vertex bool) (string, string, error) {
	target := proxyNormalizeGeminiEndpoint(cfg.Endpoint, cfg.Model, "embedContent")
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if vertex {
		token, _, err := proxyGetVertexAccessToken(ctx, cfg.APIKey)
		if err != nil {
			return "", "", err
		}
		target = proxyNormalizeVertexEmbeddingEndpoint(cfg.Endpoint, cfg.Model)
		target, err = proxyResolveVertexProjectID(target, cfg.APIKey)
		if err != nil {
			return "", "", err
		}
		headers["Authorization"] = "Bearer " + token
	} else {
		headers["x-goog-api-key"] = cfg.APIKey
	}
	status, data, raw, err := proxyDoJSON(ctx, target, headers, map[string]any{
		"content": map[string]any{"parts": []map[string]any{{"text": input}}},
	})
	if err != nil {
		return "", "", err
	}
	if status < 200 || status >= 300 {
		detail := proxyErrorDetail(status, data, raw)
		if vertex {
			detail = proxyVertexEndpointErrorDetail(status, target, data, raw)
		}
		return "", "", fmt.Errorf("embedding upstream returned %s: %s", http.StatusText(status), detail)
	}
	embedding := mapFromAny(data["embedding"])["values"]
	if embedding == nil {
		return "", "", errors.New("embedding_data_empty")
	}
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), cfg.Model, nil
}

func normalizeEmbeddingEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(endpoint, "/embeddings") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return strings.TrimSuffix(endpoint, "/chat/completions") + "/embeddings"
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/embeddings"
	}
	return endpoint + "/embeddings"
}

func normalizeVoyageEmbeddingEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return "https://api.voyageai.com/v1/embeddings"
	}
	if strings.HasSuffix(endpoint, "/embeddings") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/embeddings"
	}
	return endpoint + "/embeddings"
}

func proxyNormalizeVertexEmbeddingEndpoint(endpoint, model string) string {
	base := proxyNormalizeVertexBaseEndpoint(endpoint)
	if strings.Contains(base, ":streamGenerateContent") {
		return strings.Replace(base, ":streamGenerateContent", ":embedContent", 1)
	}
	if strings.Contains(base, ":generateContent") {
		return strings.Replace(base, ":generateContent", ":embedContent", 1)
	}
	if strings.Contains(base, ":embedContent") {
		return base
	}
	return base + "/" + strings.TrimSpace(model) + ":embedContent"
}

func parseFloat32JSONList(raw string) []float32 {
	var values []any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	out := make([]float32, 0, len(values))
	for _, item := range values {
		switch v := item.(type) {
		case float64:
			out = append(out, float32(v))
		case float32:
			out = append(out, v)
		case int:
			out = append(out, float32(v))
		case json.Number:
			f, err := v.Float64()
			if err == nil {
				out = append(out, float32(f))
			}
		default:
			return nil
		}
	}
	return out
}
