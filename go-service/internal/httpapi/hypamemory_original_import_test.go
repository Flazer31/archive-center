package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// The persistence boundary records actual writes and supports subsequent reads.
// Import identity and source preservation are exercised by the production API.
type hypaImportRecordingStore struct {
	*turnRecordingStore
	failText string
}

func (f *hypaImportRecordingStore) SaveMemory(ctx context.Context, m *store.Memory) error {
	if parseJSONMap(m.SummaryJSON)["turn_summary"] == f.failText {
		return fmt.Errorf("injected memory write failure")
	}
	m.ID = int64(len(f.returnMemories) + 1)
	f.savedMemories = append(f.savedMemories, m)
	f.returnMemories = append(f.returnMemories, *m)
	return nil
}

func (f *hypaImportRecordingStore) ListMemories(ctx context.Context, sid string, from, to int) ([]store.Memory, error) {
	rows := []store.Memory{}
	for _, m := range f.returnMemories {
		if m.ChatSessionID == sid && (from <= 0 || m.TurnIndex >= from) && (to <= 0 || m.TurnIndex <= to) {
			rows = append(rows, m)
		}
	}
	return rows, nil
}

func postHypaImportForTest(t *testing.T, srv *Server, sid string, summaries []map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{"chat_session_id": sid, "summaries": summaries})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.handleImportHypamemory(rec, httptest.NewRequest(http.MethodPost, "/import/hypamemory", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHypaOriginalImportOnePerSourceAndRepeat(t *testing.T) {
	const sid = "hypa-originals"
	fake := &hypaImportRecordingStore{turnRecordingStore: &turnRecordingStore{}}
	srv := &Server{Cfg: config.Default(), Store: fake}
	summaries := make([]map[string]any, 75)
	for i := range summaries {
		text := fmt.Sprintf("  기록 %d\n%s\n  ", i, strings.Repeat("The workshop received three copper parts. ", 40))
		summaries[i] = map[string]any{"text": text, "source_turn_index": -1}
	}
	// Equal summaries are distinct source occurrences, even on repeated imports.
	summaries[len(summaries)-1]["text"] = summaries[0]["text"]
	// Source text resembling a Critic payload must still be preserved verbatim.
	summaries[1]["text"] = " {\"turn_summary\":\"original\",\"world_rules\":[],\"evidence_excerpts\":[]} \n"
	result := postHypaImportForTest(t, srv, sid, summaries)
	if result["saved"] != float64(len(summaries)) || result["failed"] != float64(0) || len(fake.savedMemories) != len(summaries) {
		t.Fatalf("one source per memory failed: %+v writes=%d", result, len(fake.savedMemories))
	}
	used := map[int]bool{}
	for i, m := range fake.savedMemories {
		parsed := parseJSONMap(m.SummaryJSON)
		meta := mapFromAny(parsed["hypamemory_import"])
		if parsed["turn_summary"] != summaries[i]["text"] || meta["original_text"] != summaries[i]["text"] || meta["source_order"] != float64(i+1) {
			t.Fatalf("source %d rewritten or misbound: %+v", i, parsed)
		}
		if m.TurnIndex >= 0 || used[m.TurnIndex] {
			t.Fatalf("overlapping import number: %d", m.TurnIndex)
		}
		used[m.TurnIndex] = true
	}
	result = postHypaImportForTest(t, srv, sid, summaries)
	if result["saved"] != float64(0) || result["existing"] != float64(len(summaries)) || len(fake.savedMemories) != len(summaries) {
		t.Fatalf("repeat import duplicated originals: %+v writes=%d", result, len(fake.savedMemories))
	}
	// A different original at the same source position must not be discarded.
	replacement := []map[string]any{{"text": "A newly supplied source summary.", "source_turn_index": -1}}
	result = postHypaImportForTest(t, srv, sid, replacement)
	if result["saved"] != float64(len(replacement)) {
		t.Fatalf("colliding source discarded: %+v", result)
	}
}

func TestHypaImportSeparatesEmptyAndSaveFailure(t *testing.T) {
	fake := &hypaImportRecordingStore{turnRecordingStore: &turnRecordingStore{}, failText: "Cannot persist this source."}
	srv := &Server{Cfg: config.Default(), Store: fake}
	result := postHypaImportForTest(t, srv, "hypa-partial", []map[string]any{
		{"text": " \n "}, {"text": fake.failText}, {"text": "The next original still saves."},
	})
	if result["total"] != float64(3) || result["saved"] != float64(1) || result["failed"] != float64(1) || result["skipped"] != float64(1) || result["status"] != "partial_error" {
		t.Fatalf("misreported partial import: %+v", result)
	}
	items := result["items"].([]any)
	for i, status := range []string{"empty", "save_failed", "saved"} {
		item := items[i].(map[string]any)
		if item["index"] != float64(i+1) || item["status"] != status {
			t.Fatalf("wrong per-item result: %+v", item)
		}
	}
}

func TestHypaImportKeepsOriginalWhenCriticFails(t *testing.T) {
	fake := &hypaImportRecordingStore{turnRecordingStore: &turnRecordingStore{}}
	srv := NewServer(config.Default())
	srv.Store, srv.StoreOpenError = fake, nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	cfg := httptest.NewRecorder()
	mux.ServeHTTP(cfg, httptest.NewRequest(http.MethodPost, "/config/update", strings.NewReader(`{"criticApiKey":"test-key","criticEndpoint":"https://hypa-test.invalid/v1","criticModel":"critic-test","criticProvider":"openai","criticTimeout":30}`)))
	if cfg.Code != http.StatusOK {
		t.Fatalf("config: %s", cfg.Body.String())
	}
	previous := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.URL.Host != "hypa-test.invalid" || !strings.Contains(string(body), "The original lighthouse key is bronze.") {
			t.Fatalf("unexpected provider request: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"test rejected credentials"}}`))}, nil
	})}
	t.Cleanup(func() { proxyHTTPClient = previous })
	result := postHypaImportForTest(t, srv, "hypa-critic-failure", []map[string]any{{"text": "The original lighthouse key is bronze."}})
	if calls == 0 || result["saved"] != float64(1) || result["analysis_failed"] != float64(1) || result["failed"] != float64(0) || result["status"] != "partial_error" {
		t.Fatalf("analysis failure lost source: calls=%d result=%+v", calls, result)
	}
	if len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 || parseJSONMap(fake.savedMemories[0].SummaryJSON)["turn_summary"] != "The original lighthouse key is bronze." {
		t.Fatal("source was replaced or failed analysis invented artifacts")
	}
}

type hypaImportRangeStore struct {
	*hypaImportRecordingStore
	store.PrepareTurnRangeStore
}

func (f *hypaImportRangeStore) ListMemoriesRange(ctx context.Context, sid string, from, to int, ids []int64) ([]store.Memory, error) {
	rows, err := f.ListMemories(ctx, sid, 0, to)
	selected := []store.Memory{}
	for _, row := range rows {
		if row.TurnIndex < 0 || row.TurnIndex >= from {
			selected = append(selected, row)
		}
	}
	return selected, err
}

func TestHypaImportedMemoryVisibleAndScoped(t *testing.T) {
	const sid = "hypa-visible"
	fake := &hypaImportRecordingStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{
		{ID: 1, ChatSessionID: sid, TurnIndex: 10, SummaryJSON: `{"turn_summary":"Ordinary RP memory"}`},
		{ID: 2, ChatSessionID: sid, TurnIndex: -2, SummaryJSON: `{"turn_summary":"Old imported summary","hypamemory_import_score":{}}`},
		{ID: 3, ChatSessionID: "another-session", TurnIndex: -1, SummaryJSON: `{"turn_summary":"Foreign source"}`},
	}}}
	srv := &Server{Cfg: config.Default(), Store: fake}
	postHypaImportForTest(t, srv, sid, []map[string]any{{"text": "The original map shows the western pier."}})
	rec := httptest.NewRecorder()
	srv.handleExplorerMemories(rec, httptest.NewRequest(http.MethodGet, "/explorer/memories?chat_session_id="+sid+"&source=hypamemory&limit=1", nil))
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	items, _ := result["items"].([]any)
	if rec.Code != http.StatusOK || result["total"] != float64(2) || len(items) != 1 {
		t.Fatalf("import filter/pagination: %+v", result)
	}
	if mapFromAny(items[0])["hypamemory_import"] == nil {
		t.Fatalf("original unavailable in explorer: %+v", items)
	}
	segments := []prepareTurnHistorySegment{{SessionID: sid, FromTurn: 5, ToTurn: 10}}
	// Both range and ordinary store readers must retain negative imports while
	// keeping positive branch boundaries and session isolation.
	fake.returnMemories = append(fake.returnMemories,
		store.Memory{ID: 5, ChatSessionID: sid, TurnIndex: 4},
		store.Memory{ID: 6, ChatSessionID: sid, TurnIndex: 11})
	ranged := &hypaImportRangeStore{hypaImportRecordingStore: fake}
	rows, err := listPrepareTurnHistoryMemories(context.Background(), ranged, segments, nil)
	if err != nil || len(rows) != 3 {
		t.Fatalf("recall scope: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.ChatSessionID != sid || row.TurnIndex == 4 || row.TurnIndex == 11 {
			t.Fatalf("scope escaped: %+v", row)
		}
	}
	rows, err = listExplorerHistoryMemories(context.Background(), fake, segments)
	if err != nil || len(rows) != 3 {
		t.Fatalf("explorer scope: %+v %v", rows, err)
	}
}

func TestHypaOriginalMetadataDoesNotBypassPublicProjection(t *testing.T) {
	private := "Only the keeper knows the vault phrase."
	extraction := map[string]any{
		"turn_summary":      private,
		"hypamemory_import": map[string]any{"original_text": private, "critic_summary": private},
		"protected_secrets": []any{map[string]any{"content": private}},
	}
	projection := buildPublicMemoryProjection(extraction, "")
	if projection.Extraction["hypamemory_import"] != nil || strings.Contains(projection.SearchText.Text, private) {
		t.Fatalf("source metadata bypassed normal visibility projection: %+v", projection)
	}
	if mapFromAny(extraction["hypamemory_import"])["original_text"] != private {
		t.Fatal("canonical original was modified")
	}
}
