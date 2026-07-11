package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
