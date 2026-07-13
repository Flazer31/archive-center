package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type longSessionRescanStore struct {
	*turnRecordingStore
}

func (f *longSessionRescanStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	out := make([]store.Memory, 0, len(f.returnMemories)+len(f.savedMemories))
	appendIfSelected := func(item store.Memory) {
		if sid != "" && item.ChatSessionID != sid {
			return
		}
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			return
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			return
		}
		out = append(out, item)
	}
	for _, item := range f.returnMemories {
		appendIfSelected(item)
	}
	for _, item := range f.savedMemories {
		if item != nil {
			appendIfSelected(*item)
		}
	}
	return out, nil
}

func (f *longSessionRescanStore) SaveMemory(ctx context.Context, item *store.Memory) error {
	copyItem := *item
	if copyItem.ID <= 0 {
		copyItem.ID = int64(len(f.returnMemories) + len(f.savedMemories) + 1)
	}
	f.savedMemories = append(f.savedMemories, &copyItem)
	return nil
}

func TestAdminRescanLongSessionResumesFromTurnZeroThrough120WithoutDuplicates(t *testing.T) {
	const sessionID = "sess-long-rescan"
	logs := make([]store.ChatLog, 0, 241)
	logs = append(logs, store.ChatLog{
		ChatSessionID: sessionID,
		TurnIndex:     0,
		Role:          "assistant",
		Content:       "The story opens before the first user turn.",
		CreatedAt:     time.Now(),
	})
	for turn := 1; turn <= 120; turn++ {
		logs = append(logs,
			store.ChatLog{ChatSessionID: sessionID, TurnIndex: turn, Role: "user", Content: fmt.Sprintf("user event %03d", turn), CreatedAt: time.Now()},
			store.ChatLog{ChatSessionID: sessionID, TurnIndex: turn, Role: "assistant", Content: fmt.Sprintf("assistant result %03d", turn), CreatedAt: time.Now()},
		)
	}
	fake := &longSessionRescanStore{turnRecordingStore: &turnRecordingStore{returnChatLogs: logs}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	criticCalls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		criticCalls++
		extractionBytes, _ := json.Marshal(map[string]any{
			"turn_summary":     fmt.Sprintf("long-session-memory-%03d", criticCalls),
			"importance_score": 6,
		})
		chatResp, _ := json.Marshal(map[string]any{
			"model":   "rescan-critic",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	updateBody := `{"criticApiKey":"sk-rescan","criticEndpoint":"https://api.example.com/v1","criticModel":"rescan-critic","criticProvider":"openai","criticTimeout":45}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	runRescan := func(maxItems int) map[string]any {
		t.Helper()
		body := fmt.Sprintf(`{"chat_session_id":%q,"max_items":%d,"client_meta":{"full_session_backfill":true}}`, sessionID, maxItems)
		req := httptest.NewRequest(http.MethodPost, "/admin/rescan", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode rescan response: %v", err)
		}
		return response
	}

	first := runRescan(53)
	if first["candidate_count"] != float64(53) || first["succeeded"] != float64(53) {
		t.Fatalf("first chunk mismatch: %#v", first)
	}
	assertProcessedTurnRange(t, first["processed_turns"], 0, 52)

	second := runRescan(1000)
	if second["candidate_count"] != float64(68) || second["succeeded"] != float64(68) {
		t.Fatalf("second chunk mismatch: %#v", second)
	}
	assertProcessedTurnRange(t, second["processed_turns"], 53, 120)

	third := runRescan(1000)
	if third["candidate_count"] != float64(0) || third["succeeded"] != float64(0) {
		t.Fatalf("completed session should have no duplicate candidates: %#v", third)
	}
	if len(fake.savedMemories) != 121 {
		t.Fatalf("saved memories = %d, want one for every turn 0..120", len(fake.savedMemories))
	}
	if criticCalls != 121 {
		t.Fatalf("critic calls = %d, want 121 without replay", criticCalls)
	}
	seen := make(map[int]int, 121)
	for _, item := range fake.savedMemories {
		seen[item.TurnIndex]++
	}
	for turn := 0; turn <= 120; turn++ {
		if seen[turn] != 1 {
			t.Fatalf("turn %d saved %d times, want exactly once", turn, seen[turn])
		}
	}
}

func TestSessionNormalizeRescanMetadataDoesNotReplaySuccessfulCriticCalls(t *testing.T) {
	const sessionID = "sess-normalize-resume"
	logs := []store.ChatLog{
		{ChatSessionID: sessionID, TurnIndex: 1, Role: "user", Content: "The traveler reaches the old gate.", CreatedAt: time.Now()},
		{ChatSessionID: sessionID, TurnIndex: 1, Role: "assistant", Content: "The guard refuses entry until dawn.", CreatedAt: time.Now()},
		{ChatSessionID: sessionID, TurnIndex: 2, Role: "user", Content: "The traveler waits beside the wall.", CreatedAt: time.Now()},
		{ChatSessionID: sessionID, TurnIndex: 2, Role: "assistant", Content: "At dawn the gate opens.", CreatedAt: time.Now()},
	}
	fake := &longSessionRescanStore{turnRecordingStore: &turnRecordingStore{returnChatLogs: logs}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	criticCalls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		criticCalls++
		extractionBytes, _ := json.Marshal(map[string]any{
			"turn_summary":     fmt.Sprintf("normalized-memory-%d", criticCalls),
			"importance_score": 6,
		})
		chatResp, _ := json.Marshal(map[string]any{
			"model":   "normalize-critic",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	meta := adminSessionNormalizeClientMeta(map[string]any{
		"critic": map[string]any{
			"api_key":  "sk-normalize",
			"endpoint": "https://api.example.com/v1",
			"model":    "normalize-critic",
			"provider": "openai",
		},
	})
	req := adminRescanRequest{ChatSessionID: sessionID, MaxItems: 1000, ClientMeta: meta}
	first, err := srv.runAdminRescan(context.Background(), sessionID, req)
	if err != nil {
		t.Fatalf("first normalize rescan: %v", err)
	}
	if first["candidate_count"] != 2 || first["succeeded"] != 2 {
		t.Fatalf("first normalize rescan mismatch: %#v", first)
	}
	second, err := srv.runAdminRescan(context.Background(), sessionID, req)
	if err != nil {
		t.Fatalf("second normalize rescan: %v", err)
	}
	if second["candidate_count"] != 0 || second["succeeded"] != 0 {
		t.Fatalf("second normalize rescan must skip completed turns: %#v", second)
	}
	if criticCalls != 2 {
		t.Fatalf("critic calls = %d, want 2 total with no replay on retry", criticCalls)
	}
}

func assertProcessedTurnRange(t *testing.T, value any, first, last int) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("processed_turns type = %T, want []any", value)
	}
	if len(items) != last-first+1 {
		t.Fatalf("processed_turns length = %d, want %d", len(items), last-first+1)
	}
	for i, item := range items {
		want := float64(first + i)
		if item != want {
			t.Fatalf("processed_turns[%d] = %v, want %.0f", i, item, want)
		}
	}
}
