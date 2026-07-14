package vector

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestChroma159V2Integration(t *testing.T) {
	endpoint := os.Getenv("AC_CHROMA_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("AC_CHROMA_INTEGRATION_ENDPOINT is not set")
	}

	collection := fmt.Sprintf("archive_center_ci_%d", time.Now().UnixNano())
	raw, err := NewChromaStore(endpoint, collection, "/api/v2")
	if err != nil {
		t.Fatalf("NewChromaStore: %v", err)
	}
	store := raw.(interface {
		VectorStore
		CollectionResetter
		ExactMetadataQuerier
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := store.ResetAll(cleanupCtx); err != nil {
			t.Errorf("ResetAll: %v", err)
		}
	}()

	health, err := store.Health(ctx)
	if err != nil || health.Status != "ok" {
		t.Fatalf("Health: snapshot=%+v err=%v", health, err)
	}
	doc := VectorDocument{
		ID:            "ci-memory-1",
		Embedding:     []float32{0.1, 0.2, 0.3},
		Tier:          "memory",
		ChatSessionID: "ci-session",
		SourceTable:   "memories",
		SourceRowID:   "1",
		SchemaVersion: "ci.v1",
		DocumentText:  "Archive Center ChromaDB 1.5.9 integration proof",
	}
	referenceA := VectorDocument{
		ID:            "ci-reference-a",
		Embedding:     []float32{1, 0, 0},
		Tier:          "reference_claim",
		ChatSessionID: doc.ChatSessionID,
		SourceTable:   "reference_claims",
		SourceRowID:   "claim-a",
		SchemaVersion: "reference.v1",
		DocumentText:  "red gate",
		Metadata:      map[string]any{"work_id": "ci-work", "continuity_id": "ci-main", "review_status": "approved"},
	}
	referenceB := VectorDocument{
		ID:            "ci-reference-b",
		Embedding:     []float32{0, 1, 0},
		Tier:          "reference_claim",
		ChatSessionID: doc.ChatSessionID,
		SourceTable:   "reference_claims",
		SourceRowID:   "claim-b",
		SchemaVersion: "reference.v1",
		DocumentText:  "blue gate",
		Metadata:      map[string]any{"work_id": "ci-work", "continuity_id": "ci-main", "review_status": "approved"},
	}
	if err := store.Upsert(ctx, doc.ChatSessionID, []VectorDocument{doc, referenceA, referenceB}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	count, err := store.Count(ctx, doc.ChatSessionID)
	if err != nil || count != 3 {
		t.Fatalf("Count: count=%d err=%v", count, err)
	}
	results, err := store.Search(ctx, doc.ChatSessionID, doc.Embedding, 1, "tier == memory")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].ID != doc.ID || results[0].DocumentText != doc.DocumentText {
		t.Fatalf("unexpected search results: %+v", results)
	}
	where := map[string]any{"$and": []map[string]any{{"work_id": "ci-work"}, {"continuity_id": "ci-main"}, {"review_status": "approved"}}}
	redResults, err := store.QueryExact(ctx, ExactQuery{Embedding: []float32{1, 0, 0}, Limit: 2, Where: where})
	if err != nil {
		t.Fatalf("QueryExact red: %v", err)
	}
	blueResults, err := store.QueryExact(ctx, ExactQuery{Embedding: []float32{0, 1, 0}, Limit: 2, Where: where})
	if err != nil {
		t.Fatalf("QueryExact blue: %v", err)
	}
	if len(redResults) != 2 || redResults[0].Document.ID != referenceA.ID || redResults[0].ChromaRank != 1 || !redResults[0].DistanceAvailable {
		t.Fatalf("unexpected red exact results: %+v", redResults)
	}
	if len(blueResults) != 2 || blueResults[0].Document.ID != referenceB.ID || blueResults[0].ChromaRank != 1 || !blueResults[0].DistanceAvailable {
		t.Fatalf("unexpected blue exact results: %+v", blueResults)
	}
	if redResults[0].Document.ID == blueResults[0].Document.ID {
		t.Fatalf("query-sensitive rank did not change: red=%s blue=%s", redResults[0].Document.ID, blueResults[0].Document.ID)
	}
	if err := store.DeleteSession(ctx, doc.ChatSessionID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	count, err = store.Count(ctx, doc.ChatSessionID)
	if err != nil || count != 0 {
		t.Fatalf("Count after delete: count=%d err=%v", count, err)
	}
}
