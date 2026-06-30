package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// TestTimelineEmptyStore returns 200 with empty items and meta.
func TestTimelineEmptyStore(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-empty", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is not an object: %T", resp["meta"])
	}
	if meta["session_id"] != "sess-empty" {
		t.Errorf("meta.session_id = %v, want sess-empty", meta["session_id"])
	}
	if meta["read_only"] != true {
		t.Errorf("meta.read_only = %v, want true", meta["read_only"])
	}
}

// TestTimelinePopulatedStore returns merged items newest-first with correct meta.
func TestTimelinePopulatedStore(t *testing.T) {
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-a", TurnIndex: 5, Role: "user", Content: "hello", CreatedAt: time.Now()},
			{ID: 2, ChatSessionID: "sess-a", TurnIndex: 6, Role: "assistant", Content: "world", CreatedAt: time.Now()},
		},
		memories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-a", TurnIndex: 5, SummaryJSON: `{"summary":"memory at 5"}`, Importance: 0.8, CreatedAt: time.Now()},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 20, ChatSessionID: "sess-a", EvidenceKind: "fact", EvidenceText: "fact text", TurnAnchor: 4, ArchiveState: "pending_capture", CaptureStage: "critic_extract", CreatedAt: time.Now()},
		},
		kgTriples: []store.KGTriple{
			{ID: 30, ChatSessionID: "sess-a", Subject: "Alice", Predicate: "knows", Object: "Bob", SourceTurn: 3, CreatedAt: time.Now()},
		},
		episodes: []store.EpisodeSummary{
			{ID: 40, ChatSessionID: "sess-a", FromTurn: 1, ToTurn: 2, SummaryText: "episode one", CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-a&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 6 {
		t.Errorf("items count = %d, want 6", len(items))
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is not an object: %T", resp["meta"])
	}
	sc, ok := meta["source_counts"].(map[string]any)
	if !ok {
		t.Fatalf("source_counts is not an object: %T", meta["source_counts"])
	}
	if sc["chat_logs"] != float64(2) {
		t.Errorf("source_counts.chat_logs = %v, want 2", sc["chat_logs"])
	}
	if sc["memories"] != float64(1) {
		t.Errorf("source_counts.memories = %v, want 1", sc["memories"])
	}
	if sc["evidence"] != float64(1) {
		t.Errorf("source_counts.evidence = %v, want 1", sc["evidence"])
	}
	if sc["kg_triples"] != float64(1) {
		t.Errorf("source_counts.kg_triples = %v, want 1", sc["kg_triples"])
	}
	if sc["episodes"] != float64(1) {
		t.Errorf("source_counts.episodes = %v, want 1", sc["episodes"])
	}

	// Verify newest-first ordering: assistant turn 6 should be first
	first := items[0].(map[string]any)
	if first["type"] != "chat_log" || first["turn_index"] != float64(6) {
		t.Errorf("first item type=%v turn_index=%v, want chat_log/6", first["type"], first["turn_index"])
	}
}

func TestTimelineDoesNotMergeDifferentSessions(t *testing.T) {
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-one", TurnIndex: 1, Role: "user", Content: "session one", CreatedAt: time.Now()},
			{ID: 2, ChatSessionID: "sess-two", TurnIndex: 1, Role: "user", Content: "session two", CreatedAt: time.Now()},
		},
		memories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-two", TurnIndex: 1, SummaryJSON: `{"summary":"belongs to session two"}`, CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-one&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items count = %d, want only selected session item", len(items))
	}
	item := items[0].(map[string]any)
	if item["chat_session_id"] != "sess-one" {
		t.Fatalf("chat_session_id = %v, want sess-one", item["chat_session_id"])
	}
	if item["preview"] != "session one" {
		t.Fatalf("preview = %v, want session one", item["preview"])
	}
}

// TestTimelineBeforeTurnPagination excludes items at or after beforeTurn.
func TestTimelineBeforeTurnPagination(t *testing.T) {
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-b", TurnIndex: 10, Role: "user", Content: "ten", CreatedAt: time.Now()},
			{ID: 2, ChatSessionID: "sess-b", TurnIndex: 5, Role: "user", Content: "five", CreatedAt: time.Now()},
			{ID: 3, ChatSessionID: "sess-b", TurnIndex: 3, Role: "user", Content: "three", CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-b&beforeTurn=6&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	meta, ok := resp["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta is not an object: %T", resp["meta"])
	}
	if meta["next_before_turn"] != float64(0) {
		t.Errorf("next_before_turn = %v, want 0", meta["next_before_turn"])
	}
}

func TestTimelineNextBeforeTurnUsesLastReturnedTurn(t *testing.T) {
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-page", TurnIndex: 10, Role: "user", Content: "ten", CreatedAt: time.Now()},
			{ID: 2, ChatSessionID: "sess-page", TurnIndex: 5, Role: "user", Content: "five", CreatedAt: time.Now()},
			{ID: 3, ChatSessionID: "sess-page", TurnIndex: 3, Role: "user", Content: "three", CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-page&limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	meta := resp["meta"].(map[string]any)
	if meta["next_before_turn"] != float64(5) {
		t.Fatalf("next_before_turn = %v, want last returned turn 5", meta["next_before_turn"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/timeline?sessionId=sess-page&beforeTurn=5&limit=2", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	items := resp2["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("second page items = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["turn_index"] != float64(3) {
		t.Errorf("second page turn_index = %v, want 3", item["turn_index"])
	}
}

// TestTimelineItemFound returns detail for an existing memory.
func TestTimelineItemFound(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 101, ChatSessionID: "sess-c", TurnIndex: 2, SummaryJSON: `{"s":"found"}`, Importance: 0.9, CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline-item?sessionId=sess-c&type=memory&id=101", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	item, ok := resp["item"].(map[string]any)
	if !ok {
		t.Fatalf("item is not an object: %T", resp["item"])
	}
	if item["id"] != float64(101) {
		t.Errorf("item.id = %v, want 101", item["id"])
	}
	if item["type"] != "memory" {
		t.Errorf("item.type = %v, want memory", item["type"])
	}
}

func TestTimelineItemRejectsOtherSessionRow(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 101, ChatSessionID: "sess-other", TurnIndex: 2, SummaryJSON: `{"s":"other"}`, Importance: 0.9, CreatedAt: time.Now()},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline-item?sessionId=sess-current&type=memory&id=101", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "not_found" {
		t.Errorf("status = %v, want not_found", resp["status"])
	}
}

// TestTimelineItemNotFound returns 404 shape for missing ID.
func TestTimelineItemNotFound(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline-item?sessionId=sess-d&type=memory&id=999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "not_found" {
		t.Errorf("status = %v, want not_found", resp["status"])
	}
	if resp["code"] != "not_found" {
		t.Errorf("code = %v, want not_found", resp["code"])
	}
}

// TestTimelineItemMissingParams returns 400.
func TestTimelineItemMissingParams(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/timeline-item?sessionId=sess-e", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
