package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultChromaCollection = "archive_center_vectors"

type chromaStore struct {
	endpoint       string
	apiPath        string
	collectionName string
	client         *http.Client

	mu            sync.Mutex
	collectionRef string
}

type chromaCollection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewChromaStore creates a VectorStore backed by the ChromaDB HTTP API.
// ChromaDB is support-only in Archive Center 2.0; MariaDB remains canonical
// truth authority.
func NewChromaStore(endpoint, collectionName, apiPath string) (VectorStore, error) {
	return NewChromaStoreWithHTTPClient(endpoint, collectionName, apiPath, &http.Client{Timeout: 15 * time.Second})
}

func NewChromaStoreWithHTTPClient(endpoint, collectionName, apiPath string, client *http.Client) (VectorStore, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return nil, errors.New("chroma store: endpoint is required")
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("chroma store: invalid endpoint: %w", err)
	}
	collectionName = strings.TrimSpace(collectionName)
	if collectionName == "" {
		collectionName = defaultChromaCollection
	}
	apiPath = "/" + strings.Trim(strings.TrimSpace(apiPath), "/")
	if apiPath == "/" {
		apiPath = "/api/v2"
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &chromaStore{
		endpoint:       endpoint,
		apiPath:        apiPath,
		collectionName: collectionName,
		client:         client,
	}, nil
}

func (s *chromaStore) Search(ctx context.Context, sessionID string, vector []float32, limit int, filter string) ([]VectorDocument, error) {
	if len(vector) == 0 {
		return nil, ErrNotFound
	}
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	body := map[string]any{
		"query_embeddings": [][]float32{vector},
		"n_results":        limit,
		"include":          []string{"metadatas", "documents", "distances"},
	}
	if where := chromaWhere(sessionID, filter); len(where) > 0 {
		body["where"] = where
	}
	var out struct {
		IDs       [][]string         `json:"ids"`
		Documents [][]string         `json:"documents"`
		Metadatas [][]map[string]any `json:"metadatas"`
		Distances [][]float64        `json:"distances"`
	}
	if _, err := s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "query"), body, &out, http.StatusOK); err != nil {
		return nil, err
	}
	if len(out.IDs) == 0 || len(out.IDs[0]) == 0 {
		return nil, ErrNotFound
	}
	docs := make([]VectorDocument, 0, len(out.IDs[0]))
	for i, id := range out.IDs[0] {
		meta := map[string]any{}
		if len(out.Metadatas) > 0 && i < len(out.Metadatas[0]) && out.Metadatas[0][i] != nil {
			meta = out.Metadatas[0][i]
		}
		text := ""
		if len(out.Documents) > 0 && i < len(out.Documents[0]) {
			text = out.Documents[0][i]
		}
		docs = append(docs, vectorDocumentFromChroma(id, text, meta))
	}
	return docs, nil
}

func (s *chromaStore) Upsert(ctx context.Context, sessionID string, docs []VectorDocument) error {
	if len(docs) == 0 {
		return nil
	}
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(docs))
	embeddings := make([][]float32, 0, len(docs))
	metadatas := make([]map[string]any, 0, len(docs))
	documents := make([]string, 0, len(docs))
	for i, doc := range docs {
		id := strings.TrimSpace(doc.ID)
		if id == "" {
			id = fmt.Sprintf("%s:%s:%d", doc.SourceTable, sessionID, i+1)
		}
		ids = append(ids, id)
		embeddings = append(embeddings, doc.Embedding)
		meta := map[string]any{
			"tier":            doc.Tier,
			"chat_session_id": firstNonEmpty(doc.ChatSessionID, sessionID),
			"source_table":    doc.SourceTable,
			"source_row_id":   doc.SourceRowID,
			"schema_version":  doc.SchemaVersion,
			"embedding_dim":   len(doc.Embedding),
		}
		if strings.TrimSpace(doc.SearchTextPolicy) != "" {
			meta["search_text_policy"] = strings.TrimSpace(doc.SearchTextPolicy)
		}
		if strings.TrimSpace(doc.RawLanguage) != "" {
			meta["raw_language"] = strings.TrimSpace(doc.RawLanguage)
		}
		if strings.TrimSpace(doc.SummaryLanguage) != "" {
			meta["summary_language"] = strings.TrimSpace(doc.SummaryLanguage)
		}
		if strings.TrimSpace(doc.SessionOutputLanguage) != "" {
			meta["session_output_language"] = strings.TrimSpace(doc.SessionOutputLanguage)
		}
		if doc.AliasCount > 0 {
			meta["alias_count"] = doc.AliasCount
		}
		if doc.MigrationID > 0 {
			meta["migration_id"] = strconv.FormatInt(doc.MigrationID, 10)
		}
		if strings.TrimSpace(doc.MigratedFromSessionID) != "" {
			meta["migrated_from_session_id"] = strings.TrimSpace(doc.MigratedFromSessionID)
		}
		metadatas = append(metadatas, meta)
		documents = append(documents, doc.DocumentText)
	}
	body := map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documents,
	}
	_, err = s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "upsert"), body, nil, http.StatusOK, http.StatusCreated)
	if err != nil {
		return chromaDimensionMismatchError(err, docs)
	}
	return nil
}

func (s *chromaStore) DeleteSession(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return err
	}
	_, err = s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "delete"), map[string]any{
		"where": map[string]any{"chat_session_id": sessionID},
	}, nil, http.StatusOK)
	return err
}

func (s *chromaStore) DeleteDocuments(ctx context.Context, ids []string) error {
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		if item := strings.TrimSpace(id); item != "" {
			clean = append(clean, item)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return err
	}
	_, err = s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "delete"), map[string]any{
		"ids": clean,
	}, nil, http.StatusOK)
	return err
}

func (s *chromaStore) ListDocuments(ctx context.Context, sessionID string) ([]VectorDocument, error) {
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"include": []string{"metadatas", "documents"},
	}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		body["where"] = map[string]any{"chat_session_id": sessionID}
	}
	var out struct {
		IDs       []string         `json:"ids"`
		Documents []string         `json:"documents"`
		Metadatas []map[string]any `json:"metadatas"`
	}
	if _, err := s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "get"), body, &out, http.StatusOK); err != nil {
		return nil, err
	}
	docs := make([]VectorDocument, 0, len(out.IDs))
	for i, id := range out.IDs {
		meta := map[string]any{}
		if i < len(out.Metadatas) && out.Metadatas[i] != nil {
			meta = out.Metadatas[i]
		}
		text := ""
		if i < len(out.Documents) {
			text = out.Documents[i]
		}
		docs = append(docs, vectorDocumentFromChroma(id, text, meta))
	}
	return docs, nil
}

func (s *chromaStore) ResetAll(ctx context.Context) error {
	s.mu.Lock()
	ref := strings.TrimSpace(s.collectionRef)
	s.collectionRef = ""
	s.mu.Unlock()
	if ref == "" {
		var found chromaCollection
		status, err := s.doJSON(ctx, http.MethodGet, s.collectionLookupPath(s.collectionName), nil, &found, http.StatusOK, http.StatusNotFound)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return nil
		}
		ref = strings.TrimSpace(found.ID)
		if ref == "" {
			ref = strings.TrimSpace(found.Name)
		}
		if ref == "" {
			ref = strings.TrimSpace(s.collectionName)
		}
	}
	status, err := s.doJSON(ctx, http.MethodDelete, s.collectionLookupPath(ref), nil, nil,
		http.StatusOK, http.StatusAccepted, http.StatusNoContent, http.StatusNotFound)
	if err == nil || status == http.StatusNotFound {
		return nil
	}
	if ref != strings.TrimSpace(s.collectionName) {
		status, retryErr := s.doJSON(ctx, http.MethodDelete, s.collectionLookupPath(s.collectionName), nil, nil,
			http.StatusOK, http.StatusAccepted, http.StatusNoContent, http.StatusNotFound)
		if retryErr == nil || status == http.StatusNotFound {
			return nil
		}
	}
	return err
}

func (s *chromaStore) Rebuild(ctx context.Context, sessionID string) error {
	return errors.New("chroma store: rebuild is orchestrated by the MariaDB backfill pipeline")
}

func (s *chromaStore) Health(ctx context.Context) (HealthSnapshot, error) {
	status := "ok"
	if _, err := s.doJSON(ctx, http.MethodGet, "/heartbeat", nil, nil, http.StatusOK); err != nil {
		status = "error"
		return HealthSnapshot{
			Status:          status,
			Collection:      s.collectionName,
			ModelReady:      false,
			PreflightIssues: []string{err.Error()},
		}, err
	}
	count, countErr := s.Count(ctx, "")
	issues := []string{}
	if countErr != nil {
		issues = append(issues, countErr.Error())
	}
	return HealthSnapshot{
		Status:          status,
		Collection:      s.collectionName,
		TotalCount:      count,
		ModelReady:      true,
		PreflightIssues: issues,
	}, countErr
}

func (s *chromaStore) Count(ctx context.Context, sessionID string) (int, error) {
	ref, err := s.ensureCollection(ctx)
	if err != nil {
		return 0, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		var out struct {
			IDs []string `json:"ids"`
		}
		_, err := s.doJSON(ctx, http.MethodPost, s.collectionOperationPath(ref, "get"), map[string]any{
			"where":   map[string]any{"chat_session_id": sessionID},
			"include": []string{},
		}, &out, http.StatusOK)
		return len(out.IDs), err
	}
	var raw any
	if _, err := s.doJSON(ctx, http.MethodGet, s.collectionOperationPath(ref, "count"), nil, &raw, http.StatusOK); err != nil {
		return 0, err
	}
	return intFromAny(raw), nil
}

func (s *chromaStore) Close(ctx context.Context) error { return nil }

func (s *chromaStore) ensureCollection(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.collectionRef != "" {
		return s.collectionRef, nil
	}
	var found chromaCollection
	status, err := s.doJSON(ctx, http.MethodGet, s.collectionLookupPath(s.collectionName), nil, &found, http.StatusOK, http.StatusNotFound)
	if err != nil {
		if isChromaMissingCollectionError(err) {
			status = http.StatusNotFound
		} else {
			return "", err
		}
	}
	if status == http.StatusNotFound {
		body := map[string]any{"name": s.collectionName}
		if s.usesV2API() {
			body["get_or_create"] = true
		}
		status, err = s.doJSON(ctx, http.MethodPost, s.collectionListPath(), body, &found, http.StatusOK, http.StatusCreated)
		if err != nil {
			return "", err
		}
		if status != http.StatusOK && status != http.StatusCreated {
			return "", fmt.Errorf("chroma store: create collection returned %d", status)
		}
	}
	ref := strings.TrimSpace(found.ID)
	if ref == "" {
		ref = strings.TrimSpace(found.Name)
	}
	if ref == "" {
		ref = s.collectionName
	}
	s.collectionRef = ref
	return ref, nil
}

func isChromaMissingCollectionError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "InvalidCollection") ||
		(strings.Contains(text, "returned 400") && strings.Contains(text, "does not exist"))
}

func chromaDimensionMismatchError(err error, docs []VectorDocument) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if !strings.Contains(text, "expecting embedding with dimension") || !strings.Contains(text, "got") {
		return err
	}
	dim := 0
	for _, doc := range docs {
		if len(doc.Embedding) > 0 {
			dim = len(doc.Embedding)
			break
		}
	}
	if dim > 0 {
		return fmt.Errorf("chroma collection dimension mismatch: current embedding dimension=%d; existing collection was created with a different embedding dimension. Recreate the ChromaDB collection or keep the previous embedding model. Original error: %w", dim, err)
	}
	return fmt.Errorf("chroma collection dimension mismatch: existing collection was created with a different embedding dimension. Recreate the ChromaDB collection or keep the previous embedding model. Original error: %w", err)
}

func (s *chromaStore) usesV2API() bool {
	return strings.HasPrefix(strings.TrimRight(s.apiPath, "/"), "/api/v2")
}

func (s *chromaStore) collectionListPath() string {
	if s.usesV2API() {
		return "/tenants/default_tenant/databases/default_database/collections"
	}
	return "/collections"
}

func (s *chromaStore) collectionLookupPath(collectionName string) string {
	if s.usesV2API() {
		return s.collectionListPath() + "/" + url.PathEscape(collectionName)
	}
	return "/collections/" + url.PathEscape(collectionName)
}

func (s *chromaStore) collectionOperationPath(collectionRef string, operation string) string {
	if s.usesV2API() {
		return s.collectionListPath() + "/" + url.PathEscape(collectionRef) + "/" + strings.Trim(operation, "/")
	}
	return "/collections/" + url.PathEscape(collectionRef) + "/" + strings.Trim(operation, "/")
}

func (s *chromaStore) doJSON(ctx context.Context, method string, path string, body any, out any, okStatuses ...int) (int, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.url(path), reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("chroma store: %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if !statusAllowed(resp.StatusCode, okStatuses) {
		return resp.StatusCode, fmt.Errorf("chroma store: %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("chroma store: decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

func (s *chromaStore) url(path string) string {
	return s.endpoint + s.apiPath + "/" + strings.TrimLeft(path, "/")
}

func statusAllowed(status int, allowed []int) bool {
	for _, item := range allowed {
		if status == item {
			return true
		}
	}
	return false
}

func chromaWhere(sessionID string, filter string) map[string]any {
	clauses := []map[string]any{}
	if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
		clauses = append(clauses, map[string]any{"chat_session_id": sessionID})
	}
	if tier := tierFromFilter(filter); tier != "" {
		clauses = append(clauses, map[string]any{"tier": tier})
	}
	switch len(clauses) {
	case 0:
		return nil
	case 1:
		return clauses[0]
	default:
		return map[string]any{"$and": clauses}
	}
}

func tierFromFilter(filter string) string {
	lower := strings.ToLower(filter)
	for _, tier := range []string{"memory", "episode", "chapter", "arc", "saga", "evidence"} {
		if strings.Contains(lower, "tier") && strings.Contains(lower, tier) {
			return tier
		}
	}
	return ""
}

func vectorDocumentFromChroma(id string, text string, meta map[string]any) VectorDocument {
	return VectorDocument{
		ID:                    id,
		Tier:                  stringFromAny(meta["tier"]),
		ChatSessionID:         stringFromAny(meta["chat_session_id"]),
		SourceTable:           stringFromAny(meta["source_table"]),
		SourceRowID:           stringFromAny(meta["source_row_id"]),
		SchemaVersion:         stringFromAny(meta["schema_version"]),
		DocumentText:          text,
		SearchTextPolicy:      stringFromAny(meta["search_text_policy"]),
		RawLanguage:           stringFromAny(meta["raw_language"]),
		SummaryLanguage:       stringFromAny(meta["summary_language"]),
		SessionOutputLanguage: stringFromAny(meta["session_output_language"]),
		AliasCount:            intFromAny(meta["alias_count"]),
		MigrationID:           int64FromAny(meta["migration_id"]),
		MigratedFromSessionID: stringFromAny(meta["migrated_from_session_id"]),
	}
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case map[string]any:
		return intFromAny(t["count"])
	default:
		return 0
	}
}

func int64FromAny(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
