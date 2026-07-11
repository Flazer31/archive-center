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
	if err := store.Upsert(ctx, doc.ChatSessionID, []VectorDocument{doc}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	count, err := store.Count(ctx, doc.ChatSessionID)
	if err != nil || count != 1 {
		t.Fatalf("Count: count=%d err=%v", count, err)
	}
	results, err := store.Search(ctx, doc.ChatSessionID, doc.Embedding, 1, "tier == memory")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].ID != doc.ID || results[0].DocumentText != doc.DocumentText {
		t.Fatalf("unexpected search results: %+v", results)
	}
	if err := store.DeleteSession(ctx, doc.ChatSessionID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	count, err = store.Count(ctx, doc.ChatSessionID)
	if err != nil || count != 0 {
		t.Fatalf("Count after delete: count=%d err=%v", count, err)
	}
}
