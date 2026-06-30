// milvus-sdk-smoke is a guarded CLI smoke/executor for the Milvus SDK adapter.
//
// It does nothing unless -execute is provided. When executed against a real
// Milvus-compatible endpoint, it upserts 2 deterministic documents, searches
// with a deterministic query vector, and reports health.
//
// The report always contains milvus_live_enabled=false and
// live_retrieval_enabled=false because this is a manual smoke tool, not a
// live production lane.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type smokeReport struct {
	Status               string             `json:"status"`
	Executed             bool               `json:"executed"`
	Endpoint             string             `json:"endpoint,omitempty"`
	Collection           string             `json:"collection,omitempty"`
	Dimension            int                `json:"dimension,omitempty"`
	SessionID            string             `json:"session_id,omitempty"`
	QuerySet             string             `json:"query_set,omitempty"`
	QuerySetSourceMode   string             `json:"query_set_source_mode,omitempty"`
	MilvusLiveEnabled    bool               `json:"milvus_live_enabled"`
	LiveRetrievalEnabled bool               `json:"live_retrieval_enabled"`
	EnsureCollection     bool               `json:"ensure_collection"`
	CollectionEnsured    bool               `json:"collection_ensured,omitempty"`
	UpsertedCount        int                `json:"upserted_count,omitempty"`
	SearchResultCount    int                `json:"search_result_count,omitempty"`
	CountResult          int                `json:"count_result,omitempty"`
	TopIDs               []string           `json:"top_ids,omitempty"`
	Comparisons          []comparisonResult `json:"comparisons,omitempty"`
	HealthStatus         string             `json:"health_status,omitempty"`
	HealthModelReady     bool               `json:"health_model_ready,omitempty"`
	Errors               []string           `json:"errors,omitempty"`
	GeneratedAt          string             `json:"generated_at"`
}

type comparisonResult struct {
	QueryID    string   `json:"query_id"`
	SourceID   string   `json:"source_id"`
	TopIDs     []string `json:"top_ids"`
	Top1Match  bool     `json:"top1_match"`
	SelfFound  bool     `json:"self_found"`
	HitCount   int      `json:"hit_count"`
	FilterExpr string   `json:"filter_expr,omitempty"`
}

type querySet struct {
	SourceMode  string         `json:"source_mode"`
	ResultLimit int            `json:"result_limit"`
	Queries     []querySetItem `json:"queries"`
}

type querySetItem struct {
	QueryID         string    `json:"query_id"`
	SourceID        string    `json:"source_id"`
	ID              string    `json:"id"`
	Embedding       []float32 `json:"embedding"`
	Tier            string    `json:"tier"`
	ChatSessionID   string    `json:"chat_session_id"`
	SourceTable     string    `json:"source_table"`
	SourceRowID     string    `json:"source_row_id"`
	DocumentExcerpt string    `json:"document_excerpt"`
}

func main() {
	endpoint := flag.String("endpoint", os.Getenv("AC_MILVUS_ENDPOINT"), "Milvus endpoint. Defaults to AC_MILVUS_ENDPOINT.")
	execute := flag.Bool("execute", false, "Required to perform upsert/search against the endpoint.")
	ensureCollection := flag.Bool("ensure-collection", false, "Create the Archive Center Milvus collection if it is missing.")
	collection := flag.String("collection", "archive_center_vectors", "Collection name for reporting.")
	dim := flag.Int("dim", 4, "Vector dimension for synthetic docs.")
	querySetPath := flag.String("query-set", "", "Optional real vector query-set JSON to upsert/search instead of synthetic smoke docs.")
	outPath := flag.String("out", "", "Path to write JSON report. Defaults to stdout.")
	flag.Parse()

	report, exitCode := run(*endpoint, *execute, *ensureCollection, *collection, *dim, *querySetPath, vector.NewMilvusSDKStore)
	writeReport(report, *outPath)
	os.Exit(exitCode)
}

type collectionEnsurer interface {
	EnsureCollection(ctx context.Context, dimension int) error
}

func run(endpoint string, execute bool, ensureCollection bool, collection string, dim int, querySetPath string, storeFactory func(string) (vector.VectorStore, error)) (*smokeReport, int) {
	report := &smokeReport{
		Status:               "ok",
		Executed:             execute,
		EnsureCollection:     ensureCollection,
		Endpoint:             endpoint,
		Collection:           collection,
		Dimension:            dim,
		MilvusLiveEnabled:    false,
		LiveRetrievalEnabled: false,
		SessionID:            "smoke-test-session",
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(querySetPath) != "" {
		report.QuerySet = querySetPath
	}

	if !execute {
		report.Status = "guarded"
		report.Errors = append(report.Errors, "-execute is required before connecting to Milvus")
		return report, 2
	}

	if endpoint == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "missing endpoint: provide -endpoint or AC_MILVUS_ENDPOINT")
		return report, 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := storeFactory(endpoint)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("connect: %v", err))
		return report, 1
	}
	defer store.Close(ctx)

	docs, searches, sourceMode, err := loadSmokePlan(querySetPath, report.SessionID, dim)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("query_set: %v", err))
		return report, 1
	}
	if sourceMode != "" {
		report.QuerySetSourceMode = sourceMode
	}
	if len(docs) > 0 && len(docs[0].Embedding) > 0 {
		report.Dimension = len(docs[0].Embedding)
	}

	if ensureCollection {
		ensurer, ok := store.(collectionEnsurer)
		if !ok {
			report.Status = "failed"
			report.Errors = append(report.Errors, "store does not support collection bootstrap")
			return report, 1
		}
		if err := ensurer.EnsureCollection(ctx, report.Dimension); err != nil {
			report.Status = "failed"
			report.Errors = append(report.Errors, fmt.Sprintf("ensure_collection: %v", err))
			return report, 1
		}
		report.CollectionEnsured = true
	}

	if err := store.Upsert(ctx, report.SessionID, docs); err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("upsert: %v", err))
		return report, 1
	}
	report.UpsertedCount = len(docs)

	for _, search := range searches {
		results, err := store.Search(ctx, search.SessionID, search.Vector, search.Limit, search.FilterExpr)
		if err != nil {
			report.Status = "failed"
			report.Errors = append(report.Errors, fmt.Sprintf("search %s: %v", search.QueryID, err))
			return report, 1
		}
		topIDs := make([]string, 0, len(results))
		for _, r := range results {
			topIDs = append(topIDs, r.ID)
		}
		if len(report.TopIDs) == 0 {
			report.TopIDs = append(report.TopIDs, topIDs...)
		}
		report.SearchResultCount += len(results)
		report.Comparisons = append(report.Comparisons, comparisonResult{
			QueryID:    search.QueryID,
			SourceID:   search.SourceID,
			TopIDs:     topIDs,
			Top1Match:  len(topIDs) > 0 && topIDs[0] == search.SourceID,
			SelfFound:  contains(topIDs, search.SourceID),
			HitCount:   len(topIDs),
			FilterExpr: search.FilterExpr,
		})
	}

	count, err := store.Count(ctx, "")
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("count: %v", err))
	} else {
		report.CountResult = count
	}

	health, err := store.Health(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("health: %v", err))
	} else {
		report.HealthStatus = health.Status
		report.HealthModelReady = health.ModelReady
	}

	return report, 0
}

type searchPlanItem struct {
	QueryID    string
	SourceID   string
	SessionID  string
	Vector     []float32
	Limit      int
	FilterExpr string
}

func loadSmokePlan(querySetPath string, defaultSessionID string, dim int) ([]vector.VectorDocument, []searchPlanItem, string, error) {
	if strings.TrimSpace(querySetPath) == "" {
		docs := makeSmokeDocs(defaultSessionID, dim)
		return docs, []searchPlanItem{{
			QueryID:   "synthetic-smoke",
			SourceID:  "smoke-doc-1",
			SessionID: defaultSessionID,
			Vector:    makeQueryVector(dim),
			Limit:     5,
		}}, "", nil
	}

	data, err := os.ReadFile(querySetPath)
	if err != nil {
		return nil, nil, "", err
	}
	var qs querySet
	if err := json.Unmarshal(data, &qs); err != nil {
		return nil, nil, "", err
	}
	limit := qs.ResultLimit
	if limit <= 0 {
		limit = 5
	}
	docs := make([]vector.VectorDocument, 0, len(qs.Queries))
	searches := make([]searchPlanItem, 0, len(qs.Queries))
	seen := map[string]bool{}
	for idx, item := range qs.Queries {
		sourceID := item.SourceID
		if sourceID == "" {
			sourceID = item.ID
		}
		if sourceID == "" || len(item.Embedding) == 0 {
			continue
		}
		if !seen[sourceID] {
			seen[sourceID] = true
			docs = append(docs, vector.VectorDocument{
				ID:            sourceID,
				Embedding:     item.Embedding,
				Tier:          item.Tier,
				ChatSessionID: item.ChatSessionID,
				SourceTable:   item.SourceTable,
				SourceRowID:   item.SourceRowID,
				SchemaVersion: "v1",
				DocumentText:  firstNonEmpty(item.DocumentExcerpt, sourceID),
			})
		}
		queryID := item.QueryID
		if queryID == "" {
			queryID = fmt.Sprintf("q%d", idx+1)
		}
		filterExpr := ""
		if item.ChatSessionID != "" {
			filterExpr = fmt.Sprintf("chat_session_id == %q", item.ChatSessionID)
		}
		searches = append(searches, searchPlanItem{
			QueryID:    queryID,
			SourceID:   sourceID,
			SessionID:  item.ChatSessionID,
			Vector:     item.Embedding,
			Limit:      limit,
			FilterExpr: filterExpr,
		})
	}
	if len(docs) == 0 {
		return nil, nil, qs.SourceMode, fmt.Errorf("query-set has no docs with embeddings")
	}
	if len(searches) == 0 {
		return nil, nil, qs.SourceMode, fmt.Errorf("query-set has no searches with embeddings")
	}
	return docs, searches, qs.SourceMode, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func makeSmokeDocs(sessionID string, dim int) []vector.VectorDocument {
	return []vector.VectorDocument{
		{
			ID:            "smoke-doc-1",
			Embedding:     makeUnitVector(dim, 0),
			Tier:          "smoke",
			ChatSessionID: sessionID,
			SourceTable:   "smoke",
			SourceRowID:   "r1",
			SchemaVersion: "v1",
			DocumentText:  "smoke document one",
		},
		{
			ID:            "smoke-doc-2",
			Embedding:     makeUnitVector(dim, 1),
			Tier:          "smoke",
			ChatSessionID: sessionID,
			SourceTable:   "smoke",
			SourceRowID:   "r2",
			SchemaVersion: "v1",
			DocumentText:  "smoke document two",
		},
	}
}

func makeUnitVector(dim, axis int) []float32 {
	v := make([]float32, dim)
	if axis < dim {
		v[axis] = 1.0
	}
	return v
}

func makeQueryVector(dim int) []float32 {
	return makeUnitVector(dim, 0)
}

func writeReport(report *smokeReport, outPath string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing report: %v\n", err)
	}
}
