package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type adminQueueStore struct {
	*narrativeFakeStore
	auditLogs []store.AuditLog
	err       error
	gotSID    string
	gotEvent  string
	gotLimit  int
}

func (f *adminQueueStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]store.AuditLog, error) {
	f.gotSID = chatSessionID
	f.gotEvent = eventType
	f.gotLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.auditLogs, nil
}

type adminEpisodeBackfillStore struct {
	*turnRecordingStore
	savedEpisodes    []store.EpisodeSummary
	chapterSummaries []store.ChapterSummary
	savedChapters    []store.ChapterSummary
	arcSummaries     []store.ArcSummary
	savedArcs        []store.ArcSummary
	sagaDigests      []store.SagaDigest
	savedSagas       []store.SagaDigest
}

func (f *adminEpisodeBackfillStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	items := append([]store.EpisodeSummary{}, f.returnEpisodeSums...)
	items = append(items, f.savedEpisodes...)
	out := []store.EpisodeSummary{}
	for _, item := range items {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if fromTurn > 0 && item.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.FromTurn > toTurn {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *adminEpisodeBackfillStore) SaveEpisodeSummary(ctx context.Context, item *store.EpisodeSummary) error {
	cp := *item
	if cp.ID <= 0 {
		cp.ID = int64(len(f.savedEpisodes) + 1)
	}
	f.savedEpisodes = append(f.savedEpisodes, cp)
	return nil
}

func (f *adminEpisodeBackfillStore) SaveChapterSummary(ctx context.Context, item *store.ChapterSummary) error {
	cp := *item
	if cp.ID <= 0 {
		cp.ID = int64(len(f.chapterSummaries) + len(f.savedChapters) + 1)
	}
	f.savedChapters = append(f.savedChapters, cp)
	return nil
}

func (f *adminEpisodeBackfillStore) SearchChapterSummaries(ctx context.Context, sid, query string, fromTurn, toTurn, limit int) ([]store.ChapterSummary, error) {
	items := append([]store.ChapterSummary{}, f.chapterSummaries...)
	items = append(items, f.savedChapters...)
	out := []store.ChapterSummary{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, item := range items {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if fromTurn > 0 && item.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.ChapterTitle+" "+item.SummaryText+" "+item.ResumeText), query) {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *adminEpisodeBackfillStore) SaveArcSummary(ctx context.Context, sid string, item *store.ArcSummary) error {
	cp := *item
	if cp.ChatSessionID == "" {
		cp.ChatSessionID = sid
	}
	if cp.ID <= 0 {
		cp.ID = int64(len(f.arcSummaries) + len(f.savedArcs) + 1)
	}
	f.savedArcs = append(f.savedArcs, cp)
	return nil
}

func (f *adminEpisodeBackfillStore) GetLatestArcSummary(ctx context.Context, sid string) (*store.ArcSummary, error) {
	items, _ := f.ListArcSummaries(ctx, sid, "", 1)
	if len(items) == 0 {
		return nil, store.ErrNotFound
	}
	return &items[0], nil
}

func (f *adminEpisodeBackfillStore) ListArcSummaries(ctx context.Context, sid string, status string, limit int) ([]store.ArcSummary, error) {
	return f.SearchArcSummaries(ctx, sid, status, 0, 0, limit)
}

func (f *adminEpisodeBackfillStore) SearchArcSummaries(ctx context.Context, sid, query string, fromTurn, toTurn, limit int) ([]store.ArcSummary, error) {
	items := append([]store.ArcSummary{}, f.arcSummaries...)
	items = append(items, f.savedArcs...)
	out := []store.ArcSummary{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, item := range items {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if fromTurn > 0 && item.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.ArcName+" "+item.ArcStatus+" "+item.CoreConflict+" "+item.ArcResumeText), query) {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *adminEpisodeBackfillStore) SaveSagaDigest(ctx context.Context, sid string, item *store.SagaDigest) error {
	cp := *item
	if cp.ChatSessionID == "" {
		cp.ChatSessionID = sid
	}
	if cp.ID <= 0 {
		cp.ID = int64(len(f.sagaDigests) + len(f.savedSagas) + 1)
	}
	f.savedSagas = append(f.savedSagas, cp)
	return nil
}

func (f *adminEpisodeBackfillStore) GetLatestSagaDigest(ctx context.Context, sid string) (*store.SagaDigest, error) {
	items, _ := f.ListSagaDigests(ctx, sid, 1)
	if len(items) == 0 {
		return nil, store.ErrNotFound
	}
	return &items[0], nil
}

func (f *adminEpisodeBackfillStore) ListSagaDigests(ctx context.Context, sid string, limit int) ([]store.SagaDigest, error) {
	return f.SearchSagaDigests(ctx, sid, "", 0, 0, limit)
}

func (f *adminEpisodeBackfillStore) SearchSagaDigests(ctx context.Context, sid, query string, fromTurn, toTurn, limit int) ([]store.SagaDigest, error) {
	items := append([]store.SagaDigest{}, f.sagaDigests...)
	items = append(items, f.savedSagas...)
	out := []store.SagaDigest{}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, item := range items {
		if sid != "" && item.ChatSessionID != sid {
			continue
		}
		if fromTurn > 0 && item.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.EraLabel+" "+item.SagaSummary+" "+item.ResumePackText), query) {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type adminRegeneratedArtifactStore struct {
	*adminEpisodeBackfillStore
}

func (f *adminRegeneratedArtifactStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	out := append([]store.ChatLog{}, f.returnChatLogs...)
	for _, item := range f.savedChatLogs {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func (f *adminRegeneratedArtifactStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	out := append([]store.Memory{}, f.returnMemories...)
	for _, item := range f.savedMemories {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func (f *adminRegeneratedArtifactStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	out := append([]store.DirectEvidence{}, f.returnEvidence...)
	for _, item := range f.savedEvidence {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func (f *adminRegeneratedArtifactStore) ListWorldRules(ctx context.Context, sid string) ([]store.WorldRule, error) {
	out := append([]store.WorldRule{}, f.returnWorldRules...)
	for _, item := range f.savedWorldRules {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func TestAdminRescanEpisodeBackfillOnlyCreatesEpisodesWithoutCriticConfig(t *testing.T) {
	fake := &adminEpisodeBackfillStore{
		turnRecordingStore: &turnRecordingStore{
			returnChatLogs: []store.ChatLog{
				{ID: 1, ChatSessionID: "sess-ep-only", TurnIndex: 1, Role: "user", Content: "Luka wakes at the base."},
				{ID: 2, ChatSessionID: "sess-ep-only", TurnIndex: 1, Role: "assistant", Content: "The room is cold and quiet."},
				{ID: 3, ChatSessionID: "sess-ep-only", TurnIndex: 2, Role: "user", Content: "Luka asks Hank for the plan."},
				{ID: 4, ChatSessionID: "sess-ep-only", TurnIndex: 2, Role: "assistant", Content: "Hank points to the bridge map."},
				{ID: 5, ChatSessionID: "sess-ep-only", TurnIndex: 3, Role: "user", Content: "Wren starts loading oxygen tanks."},
				{ID: 6, ChatSessionID: "sess-ep-only", TurnIndex: 3, Role: "assistant", Content: "The team moves with tense focus."},
				{ID: 7, ChatSessionID: "sess-ep-only", TurnIndex: 4, Role: "user", Content: "The demolition route is confirmed."},
				{ID: 8, ChatSessionID: "sess-ep-only", TurnIndex: 4, Role: "assistant", Content: "The bridge operation becomes the priority."},
				{ID: 9, ChatSessionID: "sess-ep-only", TurnIndex: 5, Role: "user", Content: "Hank assigns the first bridge team."},
				{ID: 10, ChatSessionID: "sess-ep-only", TurnIndex: 5, Role: "assistant", Content: "The route to the bridge is locked in."},
				{ID: 11, ChatSessionID: "sess-ep-only", TurnIndex: 6, Role: "user", Content: "Luka checks the oxygen tanks."},
				{ID: 12, ChatSessionID: "sess-ep-only", TurnIndex: 6, Role: "assistant", Content: "Wren confirms the loading crew is ready."},
				{ID: 13, ChatSessionID: "sess-ep-only", TurnIndex: 7, Role: "user", Content: "The convoy prepares to leave."},
				{ID: 14, ChatSessionID: "sess-ep-only", TurnIndex: 7, Role: "assistant", Content: "The base opens the south gate."},
				{ID: 15, ChatSessionID: "sess-ep-only", TurnIndex: 8, Role: "user", Content: "A scout reports movement near the bridge."},
				{ID: 16, ChatSessionID: "sess-ep-only", TurnIndex: 8, Role: "assistant", Content: "Hank orders radio silence."},
				{ID: 17, ChatSessionID: "sess-ep-only", TurnIndex: 9, Role: "user", Content: "Luka reviews the detonation timing."},
				{ID: 18, ChatSessionID: "sess-ep-only", TurnIndex: 9, Role: "assistant", Content: "The team synchronizes watches."},
				{ID: 19, ChatSessionID: "sess-ep-only", TurnIndex: 10, Role: "user", Content: "The convoy reaches the staging point."},
				{ID: 20, ChatSessionID: "sess-ep-only", TurnIndex: 10, Role: "assistant", Content: "Operation Ice Wedge enters execution posture."},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ep-only","max_items":1,"client_meta":{"episode_interval_turns":5,"episode_backfill_only":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 2 {
		t.Fatalf("savedEpisodes = %d, want 2", len(fake.savedEpisodes))
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["episode_backfill_only"] != true {
		t.Fatalf("episode_backfill_only not reflected: %+v", resp)
	}
	if resp["candidate_count"] != float64(0) || resp["failed"] != float64(0) {
		t.Fatalf("episode-only rescan should not run critic candidates: %+v", resp)
	}
	backfill, ok := resp["episode_backfill"].(map[string]any)
	if !ok {
		t.Fatalf("episode_backfill missing: %+v", resp)
	}
	if backfill["status"] != "ok" || backfill["generated"] != float64(2) || backfill["candidate"] != float64(2) {
		t.Fatalf("unexpected episode_backfill: %+v", backfill)
	}
}

func TestAdminRescanEpisodeBackfillDefaultIntervalCreatesClosedFiveTurnEpisode(t *testing.T) {
	fake := &adminEpisodeBackfillStore{
		turnRecordingStore: &turnRecordingStore{
			returnChatLogs: []store.ChatLog{
				{ID: 1, ChatSessionID: "sess-ep-default", TurnIndex: 1, Role: "user", Content: "The protagonist draws a companion for island survival."},
				{ID: 2, ChatSessionID: "sess-ep-default", TurnIndex: 1, Role: "assistant", Content: "A companion joins the beach camp."},
				{ID: 3, ChatSessionID: "sess-ep-default", TurnIndex: 2, Role: "user", Content: "The party enters the first dungeon."},
				{ID: 4, ChatSessionID: "sess-ep-default", TurnIndex: 2, Role: "assistant", Content: "The dungeon gate opens under the survival rules."},
				{ID: 5, ChatSessionID: "sess-ep-default", TurnIndex: 3, Role: "user", Content: "They gather points from the dungeon."},
				{ID: 6, ChatSessionID: "sess-ep-default", TurnIndex: 3, Role: "assistant", Content: "The point total increases after clearing a room."},
				{ID: 7, ChatSessionID: "sess-ep-default", TurnIndex: 4, Role: "user", Content: "The player buys a skill with points."},
				{ID: 8, ChatSessionID: "sess-ep-default", TurnIndex: 4, Role: "assistant", Content: "The new skill improves their survival odds."},
				{ID: 9, ChatSessionID: "sess-ep-default", TurnIndex: 5, Role: "user", Content: "They find an item and prepare for the next floor."},
				{ID: 10, ChatSessionID: "sess-ep-default", TurnIndex: 5, Role: "assistant", Content: "The item is added to the camp inventory."},
				{ID: 11, ChatSessionID: "sess-ep-default", TurnIndex: 6, Role: "user", Content: "The second floor appears."},
				{ID: 12, ChatSessionID: "sess-ep-default", TurnIndex: 6, Role: "assistant", Content: "The party hesitates at the stairwell."},
				{ID: 13, ChatSessionID: "sess-ep-default", TurnIndex: 7, Role: "user", Content: "They decide whether to push deeper."},
				{ID: 14, ChatSessionID: "sess-ep-default", TurnIndex: 7, Role: "assistant", Content: "The camp prepares for a risky choice."},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ep-default","client_meta":{"episode_backfill_only":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 1 {
		t.Fatalf("savedEpisodes = %d, want 1: %#v", len(fake.savedEpisodes), fake.savedEpisodes)
	}
	if fake.savedEpisodes[0].FromTurn != 1 || fake.savedEpisodes[0].ToTurn != 5 {
		t.Fatalf("episode range = %d-%d, want 1-5", fake.savedEpisodes[0].FromTurn, fake.savedEpisodes[0].ToTurn)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	backfill := resp["episode_backfill"].(map[string]any)
	if backfill["interval"] != float64(5) || backfill["generated"] != float64(1) || backfill["partial_skipped"] != float64(1) {
		t.Fatalf("default interval backfill mismatch: %+v", backfill)
	}
}

func TestAdminRescanEpisodeBackfillSkipsOpenPartialRange(t *testing.T) {
	fake := &adminEpisodeBackfillStore{
		turnRecordingStore: &turnRecordingStore{
			returnChatLogs: []store.ChatLog{
				{ID: 1, ChatSessionID: "sess-ep-partial", TurnIndex: 1, Role: "user", Content: "Luka wakes at the base."},
				{ID: 2, ChatSessionID: "sess-ep-partial", TurnIndex: 1, Role: "assistant", Content: "The room is cold and quiet."},
				{ID: 3, ChatSessionID: "sess-ep-partial", TurnIndex: 2, Role: "user", Content: "Luka asks Hank for the plan."},
				{ID: 4, ChatSessionID: "sess-ep-partial", TurnIndex: 2, Role: "assistant", Content: "Hank points to the bridge map."},
				{ID: 5, ChatSessionID: "sess-ep-partial", TurnIndex: 3, Role: "user", Content: "Wren starts loading oxygen tanks."},
				{ID: 6, ChatSessionID: "sess-ep-partial", TurnIndex: 3, Role: "assistant", Content: "The team moves with tense focus."},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ep-partial","client_meta":{"episode_interval_turns":5,"episode_backfill_only":true,"force_episode_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 0 {
		t.Fatalf("savedEpisodes = %d, want 0 for open partial range", len(fake.savedEpisodes))
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	backfill := resp["episode_backfill"].(map[string]any)
	if backfill["generated"] != float64(0) || backfill["partial_skipped"] != float64(1) {
		t.Fatalf("partial range was not skipped: %+v", backfill)
	}
}

func TestAdminRescanPromotesClosedHierarchyBackfill(t *testing.T) {
	logs := []store.ChatLog{}
	id := int64(1)
	for turn := 1; turn <= 20; turn++ {
		logs = append(logs,
			store.ChatLog{ID: id, ChatSessionID: "sess-hierarchy", TurnIndex: turn, Role: "user", Content: "Luka advances Operation Ice Wedge turn " + strconv.Itoa(turn) + "."},
			store.ChatLog{ID: id + 1, ChatSessionID: "sess-hierarchy", TurnIndex: turn, Role: "assistant", Content: "The bridge team records consequence " + strconv.Itoa(turn) + "."},
		)
		id += 2
	}
	fake := &adminEpisodeBackfillStore{
		turnRecordingStore: &turnRecordingStore{returnChatLogs: logs},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-hierarchy","client_meta":{"episode_interval_turns":5,"episode_backfill_only":true,"chapter_interval_turns":10,"arc_interval_turns":20,"saga_interval_turns":20}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 4 {
		t.Fatalf("saved episodes = %d, want 4", len(fake.savedEpisodes))
	}
	if len(fake.savedChapters) != 2 {
		t.Fatalf("saved chapters = %d, want 2: %#v", len(fake.savedChapters), fake.savedChapters)
	}
	if len(fake.savedArcs) != 1 {
		t.Fatalf("saved arcs = %d, want 1: %#v", len(fake.savedArcs), fake.savedArcs)
	}
	if len(fake.savedSagas) != 1 {
		t.Fatalf("saved sagas = %d, want 1: %#v", len(fake.savedSagas), fake.savedSagas)
	}
	if fake.savedChapters[0].FromTurn != 1 || fake.savedChapters[0].ToTurn != 10 || fake.savedChapters[1].FromTurn != 11 || fake.savedChapters[1].ToTurn != 20 {
		t.Fatalf("chapter ranges mismatch: %#v", fake.savedChapters)
	}
	if fake.savedArcs[0].FromTurn != 1 || fake.savedArcs[0].ToTurn != 20 || fake.savedSagas[0].FromTurn != 1 || fake.savedSagas[0].ToTurn != 20 {
		t.Fatalf("arc/saga ranges mismatch: arcs=%#v sagas=%#v", fake.savedArcs, fake.savedSagas)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	hierarchy := resp["hierarchy_backfill"].(map[string]any)
	chapter := hierarchy["chapter"].(map[string]any)
	arc := hierarchy["arc"].(map[string]any)
	saga := hierarchy["saga"].(map[string]any)
	if chapter["generated"] != float64(2) || arc["generated"] != float64(1) || saga["generated"] != float64(1) {
		t.Fatalf("hierarchy generated counts mismatch: %+v", hierarchy)
	}
	counts := resp["artifact_counts"].(map[string]any)
	if counts["chapter_summaries"] != float64(2) || counts["arc_summaries"] != float64(1) || counts["saga_digests"] != float64(1) {
		t.Fatalf("artifact hierarchy counts mismatch: %+v", counts)
	}
}

func TestAdminRescanBackfillsWorldRulesFromExistingMemories(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{
				ChatSessionID: "sess-world-backfill",
				TurnIndex:     4,
				SummaryJSON: `{
					"turn_summary":"The bridge operation confirms the ice wedge demolition rule.",
					"world_state":{
						"version":"world_state.v1",
						"rules":[{"scope":"session","scope_name":"Demolition Logic","category":"setting","key":"ice_wedge_effect","value":"Superheated steel can fracture when shocked with freezing river water."}]
					}
				}`,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-world-backfill","client_meta":{"force_world_rule_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("saved world rules = %d, want 1: %#v", len(fake.savedWorldRules), fake.savedWorldRules)
	}
	if got := fake.savedWorldRules[0]; got.Key != "ice_wedge_effect" || got.ScopeName != "Demolition Logic" || got.SourceTurn != 4 {
		t.Fatalf("world rule fields mismatch: %+v", got)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	backfill := resp["world_rule_backfill"].(map[string]any)
	if backfill["generated"] != float64(1) {
		t.Fatalf("world_rule_backfill generated mismatch: %+v", backfill)
	}
}

func TestAdminRescanBackfillsWorldRulesFromRawChatLogsWhenMemoriesMissRules(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-raw-world", TurnIndex: 1, Role: "user", Content: "리아에게 여신님과 사도 이야기를 해도 되는지 묻는다."},
			{ChatSessionID: "sess-raw-world", TurnIndex: 1, Role: "assistant", Content: "루나는 카시아 여신이 혼돈을 정리해 땅과 바다와 하늘과 인간을 만들었다고 설명한다."},
			{ChatSessionID: "sess-raw-world", TurnIndex: 2, Role: "user", Content: "이시우가 괴물과 사도가 무엇인지 묻는다."},
			{ChatSessionID: "sess-raw-world", TurnIndex: 2, Role: "assistant", Content: "루나는 여신이 인간 운명에는 개입하지 않지만, 괴물은 혼돈의 잔재라서 인간에게 괴물을 없애는 힘을 빌려주었고 그것이 사도라고 말한다."},
		},
		returnMemories: []store.Memory{
			{ChatSessionID: "sess-raw-world", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Luna starts explaining church lore."}`},
			{ChatSessionID: "sess-raw-world", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Luna explains apostles and monsters but no structured rules were extracted."}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake

	noRulesBytes, _ := json.Marshal(map[string]any{
		"audit":       map[string]any{"durable_rule_found": false, "reason": "per-turn extractor missed the lore"},
		"world_rules": []any{},
		"world_state": map[string]any{"version": "world_state.v1", "rules": []any{}},
	})
	rawRuleBytes, _ := json.Marshal(map[string]any{
		"audit": map[string]any{"durable_rule_found": true, "reason": "The transcript establishes cosmology and apostle doctrine."},
		"world_rules": []any{
			map[string]any{"scope": "session", "scope_name": "Cassia Doctrine", "category": "cosmology", "key": "apostles_borrow_divine_power_against_chaos_monsters", "value": "Apostles are humans or agents who borrow power granted by goddess Cassia to eliminate monsters, which are remnants of chaos rather than Cassia's creations."},
		},
		"world_state": map[string]any{
			"version": "world_state.v1",
			"rules": []any{
				map[string]any{"scope": "session", "scope_name": "Cassia Doctrine", "category": "cosmology", "key": "cassia_non_intervention_except_monsters", "value": "Goddess Cassia does not directly intervene in human fate, but grants humans power against monsters because monsters come from leftover chaos."},
			},
		},
	})
	noRulesResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(noRulesBytes)}}},
	})
	rawRuleResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(rawRuleBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		resp := noRulesResp
		if strings.Contains(payload, "Session-level world rule audit from raw chat logs") {
			resp = rawRuleResp
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(resp))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-raw-world","max_items":10,"client_meta":{"critic":{"api_key":"sk-rescan","endpoint":"https://api.example.com/v1","model":"rescan-critic","provider":"openai"},"derived_backfill_only":true,"force_derived_rebuild":true,"force_world_rule_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedWorldRules) != 2 {
		t.Fatalf("saved world rules = %d, want 2: %#v", len(fake.savedWorldRules), fake.savedWorldRules)
	}
	if fake.savedWorldRules[0].SourceTurn != 2 || fake.savedWorldRules[1].SourceTurn != 2 {
		t.Fatalf("raw audit source turns mismatch: %#v", fake.savedWorldRules)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	backfill := resp["world_rule_backfill"].(map[string]any)
	if backfill["generated"] != float64(2) {
		t.Fatalf("world_rule_backfill generated mismatch: %+v", backfill)
	}
	rawAudit := backfill["raw_chat_audit"].(map[string]any)
	if rawAudit["generated"] != float64(2) || rawAudit["audit_runs"] != float64(1) {
		t.Fatalf("raw audit backfill mismatch: %+v", rawAudit)
	}
}

func TestAdminRescanForceDerivedRebuildProcessesTurnsThatAlreadyHaveMemory(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-force-derived", TurnIndex: 1, Role: "user", Content: "Luka confirms the bridge can be cracked with ice shock."},
			{ChatSessionID: "sess-force-derived", TurnIndex: 1, Role: "assistant", Content: "Hank marks the ice wedge rule as operational doctrine."},
		},
		returnMemories: []store.Memory{
			{ChatSessionID: "sess-force-derived", TurnIndex: 1, SummaryJSON: `{"turn_summary":"old already indexed memory"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":     "Luka and Hank confirm the ice wedge bridge demolition rule.",
		"importance_score": 8,
		"evidence_excerpts": []any{
			"Hank marks the ice wedge rule as operational doctrine.",
		},
		"world_rules": []any{
			map[string]any{"scope": "session", "scope_name": "Demolition Logic", "category": "setting", "key": "ice_wedge_effect", "value": "Ice shock can crack heated bridge steel."},
		},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(chatResp))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	updateBody := `{"criticApiKey":"sk-rescan","criticEndpoint":"https://api.example.com/v1","criticModel":"rescan-critic","criticProvider":"openai","criticTimeout":45}`
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, want 200: %s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-force-derived","max_items":10,"turn_indices":[1],"client_meta":{"derived_backfill_only":true,"force_derived_rebuild":true,"force_world_rule_backfill":true,"force_episode_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["candidate_count"] != float64(1) || resp["succeeded"] != float64(1) {
		t.Fatalf("force derived rescan did not process existing-memory turn: %+v", resp)
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("expected one world rule from forced rescan, got %d", len(fake.savedWorldRules))
	}
}

func TestAdminRescanBackfillsEpisodesAfterRegeneratingMemories(t *testing.T) {
	fake := &adminRegeneratedArtifactStore{
		adminEpisodeBackfillStore: &adminEpisodeBackfillStore{
			turnRecordingStore: &turnRecordingStore{
				returnChatLogs: []store.ChatLog{
					{ChatSessionID: "sess-post-derived", TurnIndex: 1, Role: "user", Content: "Luka proposes heating the bridge beams."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 1, Role: "assistant", Content: "Hank accepts the demolition theory."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 2, Role: "user", Content: "Wren starts loading oxygen tanks."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 2, Role: "assistant", Content: "The bridge team commits to Operation Ice Wedge."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 3, Role: "user", Content: "Luka assigns the convoy route to the north bridge."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 3, Role: "assistant", Content: "Hank confirms the north bridge route as the first target."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 4, Role: "user", Content: "The oxygen tanks are split between two teams."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 4, Role: "assistant", Content: "Wren locks the supply split into the operation plan."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 5, Role: "user", Content: "The first detonation window is confirmed."},
					{ChatSessionID: "sess-post-derived", TurnIndex: 5, Role: "assistant", Content: "Operation Ice Wedge is ready to execute."},
				},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":     "The team confirms Operation Ice Wedge as a bridge demolition plan.",
		"importance_score": 8,
		"evidence_excerpts": []any{
			"Operation Ice Wedge",
		},
		"world_state": map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.9,
			"verification": "verified",
			"rules": []any{
				map[string]any{"scope": "session", "scope_name": "Demolition Logic", "category": "operation", "key": "operation_ice_wedge", "value": "The bridge team plans to fracture heated bridge beams with cold shock."},
			},
		},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(chatResp))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-post-derived","max_items":5,"turn_indices":[1,2,3,4,5],"client_meta":{"critic":{"api_key":"sk-rescan","endpoint":"https://api.example.com/v1","model":"rescan-critic","provider":"openai"},"episode_interval_turns":5,"derived_backfill_only":true,"force_derived_rebuild":true,"force_world_rule_backfill":true,"force_episode_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedMemories) != 5 {
		t.Fatalf("saved memories = %d, want 5", len(fake.savedMemories))
	}
	if len(fake.savedEpisodes) != 1 {
		t.Fatalf("saved episodes = %d, want 1: %#v", len(fake.savedEpisodes), fake.savedEpisodes)
	}
	if !strings.Contains(fake.savedEpisodes[0].SummaryText, "Operation Ice Wedge") {
		t.Fatalf("episode did not use regenerated memory artifacts: %q", fake.savedEpisodes[0].SummaryText)
	}
	if len(fake.savedWorldRules) == 0 {
		t.Fatalf("world rules were not saved")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	artifactCounts := resp["artifact_counts"].(map[string]any)
	if artifactCounts["episode_summaries"] != float64(1) {
		t.Fatalf("episode_summaries count mismatch: %+v", artifactCounts)
	}
	episodeBackfill := resp["episode_backfill"].(map[string]any)
	if episodeBackfill["generated"] != float64(1) || episodeBackfill["candidate"] != float64(1) {
		t.Fatalf("episode_backfill mismatch: %+v", episodeBackfill)
	}
}

func TestAdminRescanFullSessionBackfillDoesNotClampHierarchyToProcessedTurns(t *testing.T) {
	logs := []store.ChatLog{}
	id := int64(1)
	for turn := 1; turn <= 20; turn++ {
		logs = append(logs,
			store.ChatLog{ID: id, ChatSessionID: "sess-full-backfill", TurnIndex: turn, Role: "user", Content: "Luka advances the long session checkpoint " + strconv.Itoa(turn) + "."},
			store.ChatLog{ID: id + 1, ChatSessionID: "sess-full-backfill", TurnIndex: turn, Role: "assistant", Content: "The group records durable consequence " + strconv.Itoa(turn) + "."},
		)
		id += 2
	}
	fake := &adminRegeneratedArtifactStore{
		adminEpisodeBackfillStore: &adminEpisodeBackfillStore{
			turnRecordingStore: &turnRecordingStore{returnChatLogs: logs},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.StoreOpenError = nil
	srv.Store = fake

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":     "The long session checkpoint is rebuilt.",
		"importance_score": 6,
		"evidence_excerpts": []any{
			"The group records durable consequence 18.",
		},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "rescan-critic",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(chatResp))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-full-backfill","max_items":1,"turn_indices":[18],"client_meta":{"critic":{"api_key":"sk-rescan","endpoint":"https://api.example.com/v1","model":"rescan-critic","provider":"openai"},"episode_interval_turns":5,"chapter_interval_turns":10,"derived_backfill_only":true,"force_derived_rebuild":true,"force_episode_backfill":true,"force_hierarchy_backfill":true,"full_session_backfill":true}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rescan status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedMemories) != 1 {
		t.Fatalf("saved memories = %d, want only processed turn memory", len(fake.savedMemories))
	}
	if len(fake.savedEpisodes) != 4 {
		t.Fatalf("saved episodes = %d, want full closed session coverage 4: %#v", len(fake.savedEpisodes), fake.savedEpisodes)
	}
	if len(fake.savedChapters) != 2 {
		t.Fatalf("saved chapters = %d, want 2 full-session chapters: %#v", len(fake.savedChapters), fake.savedChapters)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	episodeBackfill := resp["episode_backfill"].(map[string]any)
	if episodeBackfill["generated"] != float64(4) || episodeBackfill["candidate"] != float64(4) {
		t.Fatalf("episode backfill was clamped to processed turn: %+v", episodeBackfill)
	}
	hierarchy := resp["hierarchy_backfill"].(map[string]any)
	chapter := hierarchy["chapter"].(map[string]any)
	if chapter["generated"] != float64(2) {
		t.Fatalf("chapter backfill was not promoted from full episode coverage: %+v", hierarchy)
	}
}

func TestEpisodeSummaryBackfillPrefersMemoryArtifactsOverRawChatMetadata(t *testing.T) {
	episode, trace := buildEpisodeSummaryForRangeWithArtifacts(
		"sess-episode-quality",
		1,
		2,
		[]store.ChatLog{
			{ChatSessionID: "sess-episode-quality", TurnIndex: 1, Role: "assistant", Content: "#### Chatindex: 216∮ [2025-12-31 (Wed)] <img=\"bg_hospital\"> raw metadata should not lead."},
		},
		[]store.Memory{
			{ChatSessionID: "sess-episode-quality", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Luka asks Jade and Brynn for help against excessive restraints."}`},
		},
		nil,
	)
	if !strings.Contains(episode.SummaryText, "Luka asks Jade and Brynn") {
		t.Fatalf("episode summary did not prefer memory artifact: %q", episode.SummaryText)
	}
	if strings.Contains(episode.KeyEntities, "Chatindex") || strings.Contains(episode.KeyEntities, "Wed") {
		t.Fatalf("episode key entities leaked metadata: %s", episode.KeyEntities)
	}
	if trace["input_memory_count"] != 1 {
		t.Fatalf("trace input_memory_count = %v, want 1", trace["input_memory_count"])
	}
}

func TestWorldRuleItemsDoesNotInferRulesFromTurnSummaryKeywords(t *testing.T) {
	items := worldRuleItemsForSave(map[string]any{
		"turn_summary": "The dormitory policy requires lights-out at 10 PM, and late students must clean the common room.",
	})
	if len(items) != 0 {
		t.Fatalf("world rules must come from critic-provided world_rules/world_state.rules, got %#v", items)
	}
}

func TestWorldRuleItemsAcceptsCriticJudgedWorldStateRule(t *testing.T) {
	items := worldRuleItemsForSave(map[string]any{
		"world_state": map[string]any{
			"rules": []any{map[string]any{
				"key":        "dormitory_lights_out",
				"value":      "The dormitory has lights-out at 10 PM, and late students must clean the common room.",
				"scope":      "location",
				"scope_name": "dormitory",
				"category":   "institution",
				"confidence": 0.84,
			}},
		},
	})
	if len(items) != 1 {
		t.Fatalf("critic world_state.rules items = %d, want 1: %#v", len(items), items)
	}
	rule := mapFromAny(items[0])
	if stringFromMap(rule, "scope") != "location" || stringFromMap(rule, "category") != "institution" {
		t.Fatalf("critic world rule metadata mismatch: %+v", rule)
	}
	if !strings.Contains(stringFromMap(rule, "value"), "lights-out") {
		t.Fatalf("critic world rule value mismatch: %+v", rule)
	}
}

func TestMaintenanceQueueStatusReadsAuditStore(t *testing.T) {
	fake := &adminQueueStore{
		narrativeFakeStore: &narrativeFakeStore{},
		auditLogs: []store.AuditLog{
			{ID: 1, ChatSessionID: "sess-1", EventType: "maintenance_enqueued", Summary: "queued"},
			{ID: 2, ChatSessionID: "sess-1", EventType: "maintenance_done", Summary: "done"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/maintenance/queue-status?chat_session_id=sess-1&event_type=maintenance_enqueued&limit=7", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.gotSID != "sess-1" || fake.gotEvent != "maintenance_enqueued" || fake.gotLimit != 7 {
		t.Fatalf("unexpected store args sid=%q event=%q limit=%d", fake.gotSID, fake.gotEvent, fake.gotLimit)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["source"] != "store_audit_shadow" {
		t.Fatalf("source = %v, want store_audit_shadow", resp["source"])
	}
	if resp["queue_depth"] != float64(2) {
		t.Fatalf("queue_depth = %v, want 2", resp["queue_depth"])
	}
	counts, ok := resp["status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("status_counts missing or wrong type: %T", resp["status_counts"])
	}
	if counts["maintenance_enqueued"] != float64(1) || counts["maintenance_done"] != float64(1) {
		t.Fatalf("unexpected status_counts: %#v", counts)
	}
}

type adminResetStore struct {
	*narrativeFakeStore
	called bool
	result store.AdminResetResult
	err    error
}

func (f *adminResetStore) ResetAll(ctx context.Context) (store.AdminResetResult, error) {
	f.called = true
	if f.err != nil {
		return store.AdminResetResult{}, f.err
	}
	return f.result, nil
}

type adminResetVector struct {
	vector.VectorStore
	called bool
	err    error
}

func (f *adminResetVector) ResetAll(ctx context.Context) error {
	f.called = true
	return f.err
}

func TestAdminDatabaseResetRequiresExactDebugConfirmation(t *testing.T) {
	fake := &adminResetStore{narrativeFakeStore: &narrativeFakeStore{}}
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/database-reset", strings.NewReader(`{"debug":true,"confirm":"wrong"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if fake.called {
		t.Fatal("ResetAll should not be called without the exact confirmation token")
	}
}

func TestAdminDatabaseResetClearsStoreAndConfiguredVector(t *testing.T) {
	resetVector := true
	fake := &adminResetStore{
		narrativeFakeStore: &narrativeFakeStore{},
		result:             store.AdminResetResult{TablesCleared: 26, RowsDeleted: 42},
	}
	vec := &adminResetVector{VectorStore: vector.NewFakeVectorStore()}
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv.Cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv.Store = fake
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"debug":true,"confirm":"` + adminDatabaseResetConfirm + `","reset_vector":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/database-reset", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !fake.called {
		t.Fatal("ResetAll was not called")
	}
	if resetVector && !vec.called {
		t.Fatal("vector ResetAll was not called")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["tables_cleared"] != float64(26) || resp["rows_deleted"] != float64(42) {
		t.Fatalf("unexpected reset counts: %+v", resp)
	}
	if resp["vector_reset_status"] != "ok" {
		t.Fatalf("vector_reset_status = %v, want ok", resp["vector_reset_status"])
	}
}

type sessionMigrationFakeStore struct {
	*memoryFakeStore
	chatLogsBySession  map[string][]store.ChatLog
	memoriesBySession  map[string][]store.Memory
	evidenceBySession  map[string][]store.DirectEvidence
	kgBySession        map[string][]store.KGTriple
	canonicalBySession map[string][]store.CanonicalStateLayer
	savedEvidence      []store.DirectEvidence
	patchedEvidence    []store.DirectEvidenceExplorerPatch
}

func (f *sessionMigrationFakeStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	return append([]store.ChatLog{}, f.chatLogsBySession[sid]...), nil
}

func (f *sessionMigrationFakeStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return append([]store.Memory{}, f.memoriesBySession[sid]...), nil
}

func (f *sessionMigrationFakeStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return append([]store.DirectEvidence{}, f.evidenceBySession[sid]...), nil
}

func (f *sessionMigrationFakeStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	cp := *e
	f.savedEvidence = append(f.savedEvidence, cp)
	f.evidenceBySession[cp.ChatSessionID] = append(f.evidenceBySession[cp.ChatSessionID], cp)
	return nil
}

func (f *sessionMigrationFakeStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return append([]store.KGTriple{}, f.kgBySession[sid]...), nil
}

func (f *sessionMigrationFakeStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]store.CanonicalStateLayer, error) {
	return append([]store.CanonicalStateLayer{}, f.canonicalBySession[sid]...), nil
}

func (f *sessionMigrationFakeStore) UpdateDirectEvidenceExplorerFields(ctx context.Context, sid string, recordID int64, patch store.DirectEvidenceExplorerPatch) error {
	f.patchedEvidence = append(f.patchedEvidence, patch)
	items := f.evidenceBySession[sid]
	for i := range items {
		if items[i].ID != recordID {
			continue
		}
		if patch.ArchiveState != nil {
			items[i].ArchiveState = *patch.ArchiveState
		}
		if patch.Tombstoned != nil {
			items[i].Tombstoned = *patch.Tombstoned
		}
		if patch.SupersededByID.Set {
			if patch.SupersededByID.Value == nil {
				items[i].SupersededByID = 0
			} else {
				items[i].SupersededByID = int64(*patch.SupersededByID.Value)
			}
		}
		f.evidenceBySession[sid] = items
		return nil
	}
	return store.ErrNotFound
}

func newSessionMigrationFakeStore() *sessionMigrationFakeStore {
	return &sessionMigrationFakeStore{
		memoryFakeStore:    &memoryFakeStore{},
		chatLogsBySession:  map[string][]store.ChatLog{},
		memoriesBySession:  map[string][]store.Memory{},
		evidenceBySession:  map[string][]store.DirectEvidence{},
		kgBySession:        map[string][]store.KGTriple{},
		canonicalBySession: map[string][]store.CanonicalStateLayer{},
	}
}

func seedSessionMigrationFakeStore() *sessionMigrationFakeStore {
	fake := newSessionMigrationFakeStore()
	fake.chatLogsBySession["source"] = []store.ChatLog{{ID: 1, ChatSessionID: "source", TurnIndex: 7, Role: "user", Content: "source line"}}
	fake.memoriesBySession["source"] = []store.Memory{{ID: 2, ChatSessionID: "source", TurnIndex: 7, SummaryJSON: `{"summary":"source"}`}}
	fake.evidenceBySession["source"] = []store.DirectEvidence{
		{ID: 100, ChatSessionID: "source", EvidenceKind: "fact_event", EvidenceText: "new fact", SourceTurnStart: 7, SourceTurnEnd: 7, TurnAnchor: 7, SourceHash: "sha256:new", LineageJSON: `{"source":"export"}`},
		{ID: 101, ChatSessionID: "source", EvidenceKind: "fact_event", EvidenceText: "old fact deleted", SourceTurnStart: 8, SourceTurnEnd: 8, TurnAnchor: 8, SourceHash: "sha256:dup", Tombstoned: true, SupersededByID: 100, LineageJSON: `{"source":"export"}`},
	}
	fake.evidenceBySession["target"] = []store.DirectEvidence{
		{ID: 201, ChatSessionID: "target", EvidenceKind: "fact_event", EvidenceText: "old fact live", SourceTurnStart: 6, SourceTurnEnd: 6, TurnAnchor: 6, SourceHash: "sha256:dup"},
	}
	fake.kgBySession["source"] = []store.KGTriple{{ID: 3, ChatSessionID: "source", Subject: "A", Predicate: "knows", Object: "B", SourceTurn: 7}}
	fake.canonicalBySession["source"] = []store.CanonicalStateLayer{{ID: 4, ChatSessionID: "source", LayerType: "relationship_state", SourceTurn: 7, SourceRecord: 100, LastVerifiedTurn: 8}}
	fake.canonicalBySession["target"] = []store.CanonicalStateLayer{{ID: 5, ChatSessionID: "target", LayerType: "relationship_state", SourceTurn: 7, SourceRecord: 201, LastVerifiedTurn: 8}}
	return fake
}

func TestSeq13P67P70P74P78P81SessionPortabilityDryRunGate(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"source_session_id":"source","target_session_id":"target","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["gate_status"] != "ready" || resp["apply_status"] != "dry_run_only" {
		t.Fatalf("unexpected migration dry-run state: %+v", resp)
	}
	if resp["package_policy_version"] != "sp1a.v1" || resp["ingest_gate_policy_version"] != "sp1b.v1" || resp["merge_policy_version"] != "sp1c.v1" {
		t.Fatalf("missing SP policy versions: %+v", resp)
	}
	if resp["manual_first"] != true || resp["operation_policy_version"] != "sp1e.v1" || resp["auto_copy_detection"] != "deferred" {
		t.Fatalf("manual-first portability contract missing: %+v", resp)
	}

	dedupe := resp["dedupe_report"].(map[string]any)
	if dedupe["direct_evidence_duplicate_source_hash"] != float64(1) || dedupe["canonical_layer_collisions"] != float64(1) {
		t.Fatalf("dedupe report mismatch: %+v", dedupe)
	}
	merge := resp["merge_report"].(map[string]any)
	if merge["tombstone_propagations"] != float64(1) || merge["supersede_propagations"] != float64(1) || merge["unresolved_superseded_blocks"] != float64(0) {
		t.Fatalf("merge report mismatch: %+v", merge)
	}
	handoff := resp["rebuild_handoff"].(map[string]any)
	if handoff["policy_version"] != "sp1d.v1" || handoff["dirty_event_type"] != "backfill_import" || handoff["rebuild_mode"] != "selective" || handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("selective rebuild handoff mismatch: %+v", handoff)
	}
	fields := resp["lineage_preserve_fields"].([]any)
	for _, needle := range []string{"source_hash", "source_turn", "session_origin", "tombstoned", "superseded_by_id"} {
		if !anySliceContains(fields, needle) {
			t.Fatalf("lineage preserve fields missing %q: %+v", needle, fields)
		}
	}
	if len(fake.savedEvidence) != 0 || len(fake.patchedEvidence) != 0 {
		t.Fatalf("dry-run should not write: saved=%d patched=%d", len(fake.savedEvidence), len(fake.patchedEvidence))
	}
}

func TestSeq13P70P74SessionPortabilityApplyRequiresReadyGate(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	blockedBody := `{"source_session_id":"source","target_session_id":"target","dry_run":false,"gate_status":"review"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(blockedBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("blocked status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var blocked map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &blocked); err != nil {
		t.Fatalf("decode blocked: %v", err)
	}
	if blocked["status"] != "blocked" || blocked["apply_status"] != "gate_not_ready" {
		t.Fatalf("apply should be blocked without ready gate: %+v", blocked)
	}
	if len(fake.savedEvidence) != 0 || len(fake.patchedEvidence) != 0 {
		t.Fatalf("blocked apply should not write: saved=%d patched=%d", len(fake.savedEvidence), len(fake.patchedEvidence))
	}

	readyBody := `{"source_session_id":"source","target_session_id":"target","dry_run":false,"gate_status":"ready"}`
	req = httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(readyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var ready map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ready); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if ready["status"] != "ok" || ready["apply_status"] != "applied" || ready["moved_rows"] != float64(1) || ready["merged_rows"] != float64(1) || ready["source_rows_remaining"] != float64(0) {
		t.Fatalf("ready apply summary mismatch: %+v", ready)
	}
	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].ChatSessionID != "target" || fake.savedEvidence[0].SourceHash != "sha256:new" {
		t.Fatalf("new source evidence was not copied into target: %+v", fake.savedEvidence)
	}
	var lineage map[string]any
	if err := json.Unmarshal([]byte(fake.savedEvidence[0].LineageJSON), &lineage); err != nil {
		t.Fatalf("decode saved lineage: %v", err)
	}
	if lineage["session_origin"] != "source" || lineage["import_policy_version"] != "sp1b.v1" {
		t.Fatalf("saved lineage did not preserve import origin: %+v", lineage)
	}
	if len(fake.patchedEvidence) != 1 || fake.patchedEvidence[0].Tombstoned == nil || *fake.patchedEvidence[0].Tombstoned != true {
		t.Fatalf("duplicate tombstone/supersede was not propagated to target: %+v", fake.patchedEvidence)
	}
}

// TestSeq13P223CopyAwareImportContractMarkers verifies P223:
// Session-migrate endpoint exposes copy-aware import contract with deferred
// auto-copy detection, manual-first policy, and lineage preserve fields.
func TestSeq13P223CopyAwareImportContractMarkers(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"source_session_id":"source","target_session_id":"target","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["manual_first"] != true {
		t.Fatalf("manual_first = %v, want true", resp["manual_first"])
	}
	if resp["auto_copy_detection"] != "deferred" {
		t.Fatalf("auto_copy_detection = %v, want deferred", resp["auto_copy_detection"])
	}
	policies, ok := resp["policy_versions"].([]any)
	if !ok || len(policies) == 0 {
		t.Fatalf("policy_versions missing or empty")
	}
	for _, want := range []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"} {
		found := false
		for _, p := range policies {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("policy_versions missing %q: %v", want, policies)
		}
	}
	if resp["package_policy_version"] != "sp1a.v1" {
		t.Fatalf("package_policy_version = %v, want sp1a.v1", resp["package_policy_version"])
	}
	if resp["ingest_gate_policy_version"] != "sp1b.v1" {
		t.Fatalf("ingest_gate_policy_version = %v, want sp1b.v1", resp["ingest_gate_policy_version"])
	}
	if resp["merge_policy_version"] != "sp1c.v1" {
		t.Fatalf("merge_policy_version = %v, want sp1c.v1", resp["merge_policy_version"])
	}
	fields, ok := resp["lineage_preserve_fields"].([]any)
	if !ok || len(fields) == 0 {
		t.Fatalf("lineage_preserve_fields missing or empty")
	}
	for _, needle := range []string{"source_hash", "source_turn", "session_origin", "tombstoned", "superseded_by_id"} {
		if !anySliceContains(fields, needle) {
			t.Fatalf("lineage_preserve_fields missing %q: %+v", needle, fields)
		}
	}
	if _, ok := resp["dedupe_report"]; !ok {
		t.Fatalf("dedupe_report missing")
	}
	if _, ok := resp["merge_report"]; !ok {
		t.Fatalf("merge_report missing")
	}
	handoff, ok := resp["rebuild_handoff"].(map[string]any)
	if !ok {
		t.Fatalf("rebuild_handoff missing")
	}
	if handoff["policy_version"] != "sp1d.v1" {
		t.Fatalf("rebuild_handoff.policy_version = %v, want sp1d.v1", handoff["policy_version"])
	}
	if handoff["dirty_event_type"] != "backfill_import" {
		t.Fatalf("rebuild_handoff.dirty_event_type = %v, want backfill_import", handoff["dirty_event_type"])
	}
	if handoff["rebuild_mode"] != "selective" {
		t.Fatalf("rebuild_handoff.rebuild_mode = %v, want selective", handoff["rebuild_mode"])
	}
	if handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("rebuild_handoff.start_point = %v, want next_prepare_turn_fetch", handoff["start_point"])
	}
}

func TestMaintenanceQueueStatusErrNotEnabledIsSafeEmpty(t *testing.T) {
	fake := &adminQueueStore{
		narrativeFakeStore: &narrativeFakeStore{},
		err:                store.ErrNotEnabled,
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/maintenance/queue-status?limit=999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["queue_depth"] != float64(0) || resp["audit_count"] != float64(0) {
		t.Fatalf("expected safe empty counts, got queue_depth=%v audit_count=%v", resp["queue_depth"], resp["audit_count"])
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["store_backed"] != false {
		t.Fatalf("trace_summary.store_backed = %#v, want false", trace)
	}
}

func TestMaintenanceEnqueueShadowWritesAuditAndTrace(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-maint","turn_index":12,"shadow_only":true,"assistant_response":"The scene continues.","recent_responses":["a","b"],"supervisor_result":{"directive":{"director":{"scene_mandate":"hold"}}}}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" || resp["queue_depth"] != float64(1) || resp["maintenance_pass_enabled"] != false {
		t.Fatalf("unexpected enqueue response: %+v", resp)
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["owner"] != "maintenance_shadow" || trace["non_blocking"] != true || trace["worker_enabled"] != false {
		t.Fatalf("trace_summary mismatch: %+v", trace)
	}
	refresh, ok := resp["refresh_output"].(map[string]any)
	if !ok || refresh["story_plan_refresh"] != "shadow_candidate" || refresh["director_refresh"] != "shadow_candidate" || refresh["writeback_enabled"] != false {
		t.Fatalf("refresh_output mismatch: %+v", refresh)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "maintenance_enqueued" || fake.auditLogs[0].ChatSessionID != "sess-maint" || fake.auditLogs[0].TargetID != 12 {
		t.Fatalf("expected maintenance audit, got %#v", fake.auditLogs)
	}
}

func TestMaintenanceEnqueueMalformedPayloadIsNonBlocking(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(`{"chat_session_id":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "skipped_malformed_payload" || resp["maintenance_pass_enabled"] != false {
		t.Fatalf("malformed payload should fail open: %+v", resp)
	}
	trace, _ := resp["trace_summary"].(map[string]any)
	if trace["non_blocking"] != true || trace["fallback"] != "malformed_payload" {
		t.Fatalf("malformed trace mismatch: %+v", trace)
	}
}

func TestMaintenancePassPathUsesSessionAndShadowOutput(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/maintenance-pass/sess-path", strings.NewReader(`{"turn_index":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["chat_session_id"] != "sess-path" || resp["action"] != "maintenance_pass" || resp["queue_depth"] != float64(1) {
		t.Fatalf("maintenance-pass response mismatch: %+v", resp)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].ChatSessionID != "sess-path" {
		t.Fatalf("expected path session audit, got %#v", fake.auditLogs)
	}
}

func TestMaintenanceEnqueueDriftSignalsAndCorrectionHints(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-drift",
	  "turn_index":13,
	  "assistant_response":"They abandon the market mystery and repeat stale refrain repeat stale refrain and erase the rooftop confession.",
	  "recent_responses":["repeat stale refrain opened the previous answer too"],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"rooftop confession"},
	      "director":{"forbidden_moves":["Do not erase the rooftop confession"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	signals, ok := resp["drift_signals"].([]any)
	if !ok || len(signals) < 2 {
		t.Fatalf("drift_signals = %+v, want multiple signals", resp["drift_signals"])
	}
	joinedSignals := strings.ToLower(strings.Join(anySliceToStringSlice(signals), "\n"))
	if !strings.Contains(joinedSignals, "forbidden_move_conflict") || !strings.Contains(joinedSignals, "pattern_repeat") {
		t.Fatalf("missing forbidden/pattern signals: %s", joinedSignals)
	}
	hints, ok := resp["correction_hints"].([]any)
	if !ok || len(hints) == 0 {
		t.Fatalf("correction_hints missing: %+v", resp["correction_hints"])
	}
	joinedHints := strings.ToLower(strings.Join(anySliceToStringSlice(hints), "\n"))
	if !strings.Contains(joinedHints, "may_override_current_user_input:false") && !strings.Contains(joinedHints, "map[") {
		t.Fatalf("correction hints should be subordinate maps: %s", joinedHints)
	}
	trace, _ := resp["trace_summary"].(map[string]any)
	traceSignals, _ := trace["drift_signals"].([]any)
	if len(traceSignals) != len(signals) {
		t.Fatalf("trace drift signals mismatch: trace=%+v resp=%+v", traceSignals, signals)
	}
}

func TestMaintenanceTM1bProvenanceConfidenceDriftAuditSurface(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-tm1b",
	  "turn_index":42,
	  "assistant_response":"They erase the archive promise and walk away from the archive promise.",
	  "recent_responses":["archive promise archive promise"],
	  "canonical_state_layers":[
	    {"layer_type":"relationship_state","last_verified_turn":39,"confidence":0.82},
	    {"layer_type":"scene_state","last_verified_turn":40,"confidence":0.33}
	  ],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"archive promise"},
	      "director":{"forbidden_moves":["erase the archive promise"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["maintenance_pass_enabled"] != false || resp["worker_enabled"] != false {
		t.Fatalf("maintenance should remain shadow-only: %+v", resp)
	}
	if resp["last_verified_turn"] != float64(42) {
		t.Fatalf("last_verified_turn = %v, want 42", resp["last_verified_turn"])
	}
	signals, ok := resp["drift_signals"].([]any)
	if !ok || len(signals) == 0 {
		t.Fatalf("drift_signals missing or empty: %T", resp["drift_signals"])
	}
	for _, s := range signals {
		sig, ok := s.(map[string]any)
		if !ok {
			t.Fatalf("drift signal wrong type: %T", s)
		}
		if sig["drift_type"] == "" {
			t.Fatalf("drift signal missing drift_type: %+v", sig)
		}
		if sig["canonical_name"] == "" {
			t.Fatalf("drift signal missing canonical_name: %+v", sig)
		}
		if sig["scene"] == "" {
			t.Fatalf("drift signal missing scene: %+v", sig)
		}
	}
	state, ok := resp["maintenance_pass_state"].(map[string]any)
	if !ok {
		t.Fatalf("maintenance_pass_state missing: %+v", resp)
	}
	if state["surface"] != "MaintenancePassState" || state["status"] != "shadow_only" || state["would_write"] != false {
		t.Fatalf("maintenance pass state authority mismatch: %+v", state)
	}
	if state["drift_detected"] != true || state["confidence_floor"] != float64(0.3) {
		t.Fatalf("drift/floor mismatch: %+v", state)
	}
	if !strings.Contains(state["drift_signals_json"].(string), "forbidden_move_conflict") {
		t.Fatalf("drift_signals_json missing forbidden conflict: %v", state["drift_signals_json"])
	}
	updates, ok := state["canonical_updates"].([]any)
	if !ok || len(updates) != 2 {
		t.Fatalf("canonical_updates missing: %+v", state["canonical_updates"])
	}
	first, ok := updates[0].(map[string]any)
	if !ok {
		t.Fatalf("first canonical update is not an object: %+v", updates[0])
	}
	if first["last_verified_turn"] != float64(39) || first["confidence"] != float64(0.82) || first["next_confidence"] != float64(0.67) {
		t.Fatalf("relationship provenance/degradation mismatch: %+v", first)
	}
	second, ok := updates[1].(map[string]any)
	if !ok {
		t.Fatalf("second canonical update is not an object: %+v", updates[1])
	}
	if second["confidence"] != float64(0.33) || second["next_confidence"] != float64(0.3) {
		t.Fatalf("confidence floor mismatch: %+v", second)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "drift_detected" {
		t.Fatalf("expected drift_detected audit, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["confidence_floor"] != float64(0.3) || details["maintenance_pass_state"] == nil {
		t.Fatalf("audit details missing TM-1b state: %+v", details)
	}
}

func TestMaintenanceTM1cMemoryImportanceFreshnessReweightingSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ChatSessionID: "sess-tm1c", TurnIndex: 10, Content: "The blue oath is mentioned again."},
			{ChatSessionID: "sess-tm1c", TurnIndex: 12, Content: "Blue oath promise returns in the current beat."},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1c", TurnIndex: 11, SummaryJSON: `{"turn_summary":"Blue oath promise remains active"}`, Importance: 0.5},
			{ID: 2, ChatSessionID: "sess-tm1c", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Solved gate mystery clue"}`, Importance: 0.7, EmotionalIntensity: 0.8},
			{ID: 3, ChatSessionID: "sess-tm1c", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Pinned vow callback"}`, Importance: 0.4, EmotionalIntensity: 0.9},
			{ID: 4, ChatSessionID: "sess-tm1c", TurnIndex: 1, SummaryJSON: `{"turn_summary":"User corrected anchor sacred key"}`, Importance: 0.3, EmotionalIntensity: 0.9},
		},
		storylines: []store.Storyline{
			{Name: "Gate mystery", Status: "resolved", CurrentContext: "Solved gate mystery clue"},
			{Name: "Pinned vow", Status: "active", CurrentContext: "Pinned vow callback", Pinned: true},
		},
		pendingThreads: []store.PendingThread{
			{ThreadKey: "gate-thread", Status: "resolved", Description: "Solved gate mystery clue"},
			{ThreadKey: "sacred-key", Status: "open", Description: "User corrected anchor sacred key", UserCorrected: true},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-tm1c",
	  "turn_index":12,
	  "assistant_response":"The blue oath is carried forward without contradiction."
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	state, ok := resp["memory_importance_reweighting"].(map[string]any)
	if !ok {
		t.Fatalf("memory_importance_reweighting missing: %+v", resp)
	}
	if state["policy_version"] != "tm1c.v1" || state["status"] != "shadow_only" || state["would_write"] != false {
		t.Fatalf("TM-1c authority mismatch: %+v", state)
	}
	if state["source"] != "store" || state["recent_source"] != "store" || state["importance_scale"] != "0..1" {
		t.Fatalf("TM-1c source/scale mismatch: %+v", state)
	}
	if state["updated_count"] != float64(2) || state["boosted_count"] != float64(1) || state["decayed_count"] != float64(1) || state["protected_count"] != float64(2) {
		t.Fatalf("TM-1c counters mismatch: %+v", state)
	}
	updates, ok := state["updates"].([]any)
	if !ok || len(updates) != 2 {
		t.Fatalf("TM-1c updates missing: %+v", state["updates"])
	}
	byID := map[float64]map[string]any{}
	for _, raw := range updates {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("TM-1c update wrong type: %+v", raw)
		}
		byID[item["memory_id"].(float64)] = item
	}
	boosted := byID[1]
	if boosted["next_importance"] != float64(0.56) || boosted["recent_rementioned"] != true {
		t.Fatalf("recent remention boost mismatch: %+v", boosted)
	}
	boostReasons := strings.Join(anySliceToStringSlice(boosted["reasons"].([]any)), ",")
	if !strings.Contains(boostReasons, "recent_remention_boost") {
		t.Fatalf("boost reasons missing recent_remention_boost: %s", boostReasons)
	}
	decayed := byID[2]
	if decayed["next_importance"] != float64(0.53) || decayed["age_gap"] != float64(10) {
		t.Fatalf("resolved/emotional decay mismatch: %+v", decayed)
	}
	decayReasons := strings.Join(anySliceToStringSlice(decayed["reasons"].([]any)), ",")
	for _, needle := range []string{"freshness_decay", "resolved_reference_decay", "emotional_decay"} {
		if !strings.Contains(decayReasons, needle) {
			t.Fatalf("decay reasons missing %s: %s", needle, decayReasons)
		}
	}
	if _, protectedPinnedUpdated := byID[3]; protectedPinnedUpdated {
		t.Fatalf("pinned protected memory should not decay: %+v", byID[3])
	}
	if _, protectedUserUpdated := byID[4]; protectedUserUpdated {
		t.Fatalf("user_corrected protected memory should not decay: %+v", byID[4])
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "importance_reevaluation" {
		t.Fatalf("expected importance_reevaluation audit, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["memory_importance_reweighting"] == nil {
		t.Fatalf("audit details missing TM-1c state: %+v", details)
	}
}

func TestMaintenanceEnqueueFalsePositiveSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-clean",
	  "turn_index":14,
	  "assistant_response":"The rooftop confession continues with a quiet pause.",
	  "recent_responses":["A different sentence."],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"rooftop confession"},
	      "director":{"forbidden_moves":["no"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if signals, _ := resp["drift_signals"].([]any); len(signals) != 0 {
		t.Fatalf("false-positive drift signals = %+v, want none", signals)
	}
	if hints, _ := resp["correction_hints"].([]any); len(hints) != 0 {
		t.Fatalf("false-positive correction hints = %+v, want none", hints)
	}
}
func TestMaintenanceTM1dAuditReplayDirtyMatrixSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1d", TurnIndex: 5, SummaryJSON: `{"turn_summary":"recent mention of blue oath"}`, Importance: 0.5},
		},
		storylines: []store.Storyline{
			{Name: "arc-x", Status: "resolved", CurrentContext: "resolved arc"},
		},
		pendingThreads: []store.PendingThread{
			{ThreadKey: "thread-y", Status: "resolved", Description: "done"},
		},
		chatLogs: []store.ChatLog{
			{ChatSessionID: "sess-tm1d", TurnIndex: 11, Content: "blue oath"},
			{ChatSessionID: "sess-tm1d", TurnIndex: 12, Content: "continues"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// 1) Drift request
	body1 := `{"chat_session_id":"sess-tm1d-drift","turn_index":5,"assistant_response":"They erase the archive promise and walk away from the archive promise.","recent_responses":["archive promise archive promise"],"canonical_state_layers":[{"layer_type":"relationship_state","last_verified_turn":3,"confidence":0.82},{"layer_type":"scene_state","last_verified_turn":4,"confidence":0.33}],"supervisor_result":{"directive":{"story_author":{"current_arc":"archive promise"},"director":{"forbidden_moves":["erase the archive promise"]}}}}`
	req1 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("drift status = %d, want 200: %s", rec1.Code, rec1.Body.String())
	}
	var resp1 map[string]any
	if err := json.Unmarshal(rec1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("drift decode: %v", err)
	}

	refresh1, _ := resp1["refresh_output"].(map[string]any)
	if refresh1 == nil {
		t.Fatalf("drift refresh_output missing")
	}
	dirtyMatrix1, _ := refresh1["or_phase_dirty_matrix"].(map[string]any)
	if dirtyMatrix1 == nil {
		t.Fatalf("drift or_phase_dirty_matrix missing in response")
	}
	if dirtyMatrix1["policy_version"] != "tm1d.v1" {
		t.Fatalf("drift policy_version = %v, want tm1d.v1", dirtyMatrix1["policy_version"])
	}
	if dirtyMatrix1["matrix_version"] != "or1h.tm1d.v1" {
		t.Fatalf("drift matrix_version = %v, want or1h.tm1d.v1", dirtyMatrix1["matrix_version"])
	}
	rows1, _ := dirtyMatrix1["rows"].([]any)
	if len(rows1) == 0 {
		t.Fatalf("drift rows empty")
	}
	firstRow1, ok := rows1[0].(map[string]any)
	if !ok {
		t.Fatalf("drift row wrong type")
	}
	if firstRow1["event_type"] != "drift_detected" {
		t.Fatalf("drift row event_type = %v, want drift_detected", firstRow1["event_type"])
	}
	if firstRow1["dirty_scope"] != "relationship_state" {
		t.Fatalf("drift row dirty_scope = %v, want relationship_state", firstRow1["dirty_scope"])
	}
	if dirtyMatrix1["row_count"] != float64(len(rows1)) {
		t.Fatalf("drift row_count = %v, want %d", dirtyMatrix1["row_count"], len(rows1))
	}
	targets1, _ := firstRow1["dirty_targets"].([]any)
	if len(targets1) == 0 {
		t.Fatalf("drift row dirty_targets empty")
	}
	replay1, _ := refresh1["replay_measurements"].(map[string]any)
	if replay1 == nil {
		t.Fatalf("drift replay_measurements missing")
	}
	if replay1["tm_drift_pass_count"] != float64(1) {
		t.Fatalf("drift tm_drift_pass_count = %v, want 1", replay1["tm_drift_pass_count"])
	}

	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "drift_detected" {
		t.Fatalf("expected drift_detected audit, got %#v", fake.auditLogs)
	}
	var auditDetails1 map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &auditDetails1); err != nil {
		t.Fatalf("drift audit details decode: %v", err)
	}
	auditMatrix1, _ := auditDetails1["or_phase_dirty_matrix"].(map[string]any)
	if auditMatrix1 == nil {
		t.Fatalf("drift audit or_phase_dirty_matrix missing")
	}
	if auditMatrix1["event_type"] != "drift_detected" {
		t.Fatalf("drift audit event_type mismatch")
	}

	// 2) Importance-only request
	fake.auditLogs = nil
	body2 := `{"chat_session_id":"sess-tm1d-imp","turn_index":12,"assistant_response":"The blue oath is carried forward without contradiction."}`
	req2 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("imp status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("imp decode: %v", err)
	}

	refresh2, _ := resp2["refresh_output"].(map[string]any)
	if refresh2 == nil {
		t.Fatalf("imp refresh_output missing")
	}
	dirtyMatrix2, _ := refresh2["or_phase_dirty_matrix"].(map[string]any)
	if dirtyMatrix2 == nil {
		t.Fatalf("imp or_phase_dirty_matrix missing")
	}
	rows2, _ := dirtyMatrix2["rows"].([]any)
	if len(rows2) == 0 {
		t.Fatalf("imp rows empty")
	}
	if dirtyMatrix2["row_count"] != float64(len(rows2)) {
		t.Fatalf("imp row_count = %v, want %d", dirtyMatrix2["row_count"], len(rows2))
	}
	hasBoost := false
	for _, raw := range rows2 {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if row["event_type"] != "importance_reevaluation" {
			t.Fatalf("imp row event_type = %v", row["event_type"])
		}
		if row["dirty_scope"] != "memory_importance" {
			t.Fatalf("imp row dirty_scope = %v, want memory_importance", row["dirty_scope"])
		}
		if row["delta_direction"] == "boost" {
			hasBoost = true
		}
	}
	if !hasBoost {
		t.Fatalf("imp expected at least one boost row")
	}
	replay2, _ := refresh2["replay_measurements"].(map[string]any)
	if replay2 == nil {
		t.Fatalf("imp replay_measurements missing")
	}
	if replay2["tm_importance_pass_count"] != float64(1) {
		t.Fatalf("imp tm_importance_pass_count = %v, want 1", replay2["tm_importance_pass_count"])
	}

	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "importance_reevaluation" {
		t.Fatalf("expected importance_reevaluation audit, got %#v", fake.auditLogs)
	}
	var auditDetails2 map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &auditDetails2); err != nil {
		t.Fatalf("imp audit details decode: %v", err)
	}
	auditMatrix2, _ := auditDetails2["or_phase_dirty_matrix"].(map[string]any)
	if auditMatrix2 == nil {
		t.Fatalf("imp audit or_phase_dirty_matrix missing")
	}
	if auditMatrix2["event_type"] != "importance_reevaluation" {
		t.Fatalf("imp audit event_type mismatch")
	}
}

func TestSeq123P79PlanningOnlyBridgeMarkers(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// 1. Readiness shows live cutover disabled = 12.5 entry decision is pending.
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200", rec.Code)
	}
	var r readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if r.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover = %q, want disabled", r.Checks["live_cutover"])
	}

	// 2. Maintenance enqueue must not activate runtime features (worker/pass disabled).
	body := `{"chat_session_id":"sess-p79","turn_index":1,"shadow_only":true,"assistant_response":"ok","recent_responses":["a"]}`
	req2 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("enqueue status = %d, want 200", rec2.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode enqueue: %v", err)
	}
	if resp["maintenance_pass_enabled"] != false {
		t.Errorf("maintenance_pass_enabled = %v, want false", resp["maintenance_pass_enabled"])
	}
	if resp["worker_enabled"] != false {
		t.Errorf("worker_enabled = %v, want false", resp["worker_enabled"])
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["non_blocking"] != true {
		t.Errorf("trace_summary non_blocking missing or false: %+v", trace)
	}

	// 3. Admin reindex must be shadow-guarded: runtime activation is not allowed in planning bridge.
	req3 := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-p79","dry_run":true,"allow_shadow_boundary":true}`))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusServiceUnavailable {
		t.Errorf("reindex status = %d, want 503 (shadow guard blocks runtime activation)", rec3.Code)
	}
}

func TestAdminReindexUpsertsExistingMemoryEmbeddings(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:             42,
				ChatSessionID:  "sess-reindex-live",
				TurnIndex:      7,
				SummaryJSON:    `{"summary":"Blue lantern oath persists."}`,
				Embedding:      `[0.1,0.2,0.3]`,
				EmbeddingModel: "test-embedding",
			},
		},
	}
	vec := &turnRecordingVectorStore{}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-reindex-live","dry_run":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["reindex_executed"] != true {
		t.Fatalf("reindex_executed = %v, want true", resp["reindex_executed"])
	}
	if resp["upserted"] != float64(1) {
		t.Fatalf("upserted = %v, want 1", resp["upserted"])
	}
	qv, ok := resp["quality_verification"].(map[string]any)
	if !ok {
		t.Fatalf("quality_verification missing: %#v", resp["quality_verification"])
	}
	if qv["status"] != "requires_before_after_report" || qv["before_after_required"] != true {
		t.Fatalf("quality_verification = %#v, want before/after report required", qv)
	}
	if len(vec.docs) != 1 {
		t.Fatalf("vector docs = %d, want 1", len(vec.docs))
	}
	doc := vec.docs[0]
	if doc.ID != "memory:sess-reindex-live:42" {
		t.Fatalf("doc ID = %q", doc.ID)
	}
	if doc.ChatSessionID != "sess-reindex-live" || doc.SourceTable != "memories" || doc.SourceRowID != "42" {
		t.Fatalf("doc provenance mismatch: %#v", doc)
	}
	if !strings.Contains(doc.DocumentText, "Blue lantern oath persists.") || !strings.Contains(doc.DocumentText, "[Canonical Summary]") {
		t.Fatalf("doc text = %q", doc.DocumentText)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "admin_reindex" {
		t.Fatalf("expected admin_reindex audit log, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["upserted"] != float64(1) {
		t.Fatalf("audit upserted = %v, want 1", details["upserted"])
	}
	integrity, ok := resp["integrity_report"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_report missing: %#v", resp["integrity_report"])
	}
	if integrity["vector_count_matches_canonical"] != true {
		t.Fatalf("vector_count_matches_canonical = %v, want true; integrity=%#v", integrity["vector_count_matches_canonical"], integrity)
	}
	if integrity["missing_vector_count_estimate"] != float64(0) {
		t.Fatalf("missing_vector_count_estimate = %v, want 0", integrity["missing_vector_count_estimate"])
	}
}

func TestAdminReindexIntegrityReportDetectsMissingVectorsAndModelMismatch(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.EmbedderModel = "embed-model"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-integrity", TurnIndex: 1, SummaryJSON: `{"summary":"Already indexed memory"}`, Embedding: `[0.1]`, EmbeddingModel: "embed-model"},
			{ID: 2, ChatSessionID: "sess-integrity", TurnIndex: 2, SummaryJSON: `{"summary":"Missing embedding memory"}`, Embedding: ``, EmbeddingModel: ""},
			{ID: 3, ChatSessionID: "sess-integrity", TurnIndex: 3, SummaryJSON: `{"summary":"Old model memory"}`, Embedding: `[0.3]`, EmbeddingModel: "old-model"},
		},
	}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 1, ProjectModel: "embed-model", ModelReady: true},
		countResult:    1,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-integrity","dry_run":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	integrity, ok := resp["integrity_report"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_report missing: %#v", resp["integrity_report"])
	}
	if integrity["status"] != "reindex_recommended" {
		t.Fatalf("integrity status = %v, want reindex_recommended; integrity=%#v", integrity["status"], integrity)
	}
	if integrity["canonical_memory_count"] != float64(3) || integrity["vector_count"] != float64(1) {
		t.Fatalf("count mismatch in report: %#v", integrity)
	}
	if integrity["missing_vector_count_estimate"] != float64(2) {
		t.Fatalf("missing_vector_count_estimate = %v, want 2", integrity["missing_vector_count_estimate"])
	}
	if integrity["missing_embedding_count"] != float64(1) {
		t.Fatalf("missing_embedding_count = %v, want 1", integrity["missing_embedding_count"])
	}
	if integrity["embedding_model_mismatch_count"] != float64(1) {
		t.Fatalf("embedding_model_mismatch_count = %v, want 1", integrity["embedding_model_mismatch_count"])
	}
	if integrity["reindex_recommended"] != true || integrity["reembed_recommended"] != true {
		t.Fatalf("recommendations missing: %#v", integrity)
	}
	reasons := fmt.Sprint(integrity["reindex_reasons"])
	for _, want := range []string{"vector_count_below_canonical_memory_count", "memory_rows_missing_embedding", "embedding_model_mismatch"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("reindex_reasons = %v, missing %q", reasons, want)
		}
	}
}

func TestAdminReindexBackgroundJobReportsProgress(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:             42,
				ChatSessionID:  "sess-reindex-bg",
				TurnIndex:      7,
				SummaryJSON:    `{"summary":"Blue lantern oath persists."}`,
				Embedding:      `[0.1,0.2,0.3]`,
				EmbeddingModel: "test-embedding",
			},
		},
	}
	vec := &turnRecordingVectorStore{}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-reindex-bg","dry_run":false,"background":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["processed"] != float64(1) || progress["upserted"] != float64(1) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	if progress["progress_percent"] != float64(100) {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["reindex_executed"] != true {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminRescanBackgroundJobReportsFailedTurns(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 2, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 3, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 4, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
		},
	}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(`{"chat_session_id":"sess-rescan-bg","max_items":222,"background":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["candidate_count"] != float64(2) || progress["failed_count"] != float64(2) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	failedTurns, ok := progress["failed_turns"].([]any)
	if !ok || len(failedTurns) != 2 {
		t.Fatalf("failed_turns mismatch: %#v", progress["failed_turns"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["failed"] != float64(2) {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminSessionNormalizeQueuesRedactedBackgroundJob(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-normalize-bg","max_items":25,"repair_entries":[{"turn_index":1,"user_content":"private user text","assistant_content":"private assistant text"}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-normalize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if start["kind"] != "session_normalize" {
		t.Fatalf("kind = %v, want session_normalize", start["kind"])
	}
	if start["job_id"] == "" {
		t.Fatalf("job_id missing: %#v", start)
	}
	request, ok := start["request"].(map[string]any)
	if !ok {
		t.Fatalf("request missing: %#v", start)
	}
	if request["repair_entry_count"] != float64(1) {
		t.Fatalf("repair_entry_count = %v, want 1", request["repair_entry_count"])
	}
	raw, _ := json.Marshal(request)
	if strings.Contains(string(raw), "private user text") || strings.Contains(string(raw), "private assistant text") {
		t.Fatalf("job request leaked raw content: %s", string(raw))
	}
	if request["destructive"] != false {
		t.Fatalf("destructive flag = %v, want false", request["destructive"])
	}
}
