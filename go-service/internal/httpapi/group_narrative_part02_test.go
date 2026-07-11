package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCharacterAndEpisodeNotFoundPythonShape(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		store      *narrativeFakeStore
		wantDetail string
	}{
		{
			name:       "character",
			path:       "/characters/sess-1/Alice",
			store:      &narrativeFakeStore{characterNotFound: true},
			wantDetail: "character not found: Alice",
		},
		{
			name:       "episode",
			path:       "/episodes/detail/999999",
			store:      &narrativeFakeStore{episodeNotFound: true},
			wantDetail: "episode not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			srv := setupTestServer()
			srv.Store = tt.store
			srv.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "error" || resp["detail"] != tt.wantDetail {
				t.Fatalf("unexpected not-found response: %#v", resp)
			}
			if _, ok := resp["found"]; ok {
				t.Fatalf("unexpected found key in not-found response: %#v", resp)
			}
		})
	}
}

func TestNarrativeReadBehaviorMatchesPythonReferenceShape(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ID:               1,
				ChatSessionID:    "sess-1",
				Name:             "Gate pressure",
				Status:           "active",
				EvidenceCount:    1,
				LastEvidenceTurn: 5,
				LastTurn:         5,
			},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "location", ScopeName: "Archive", Category: "access", Key: "sealed"},
			{ID: 2, ChatSessionID: "sess-1", Scope: "root", Category: "physics", Key: "gravity"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 20, SummaryText: "Alice studies the sealed archive gate.", KeyEntities: "Alice"},
			{ID: 2, ChatSessionID: "sess-1", FromTurn: 21, ToTurn: 60, SummaryText: "The gate pressure rises.", KeyEvents: "pressure"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Chapter: &store.ChapterSummary{
				ID:           7,
				FromTurn:     1,
				ToTurn:       60,
				ChapterTitle: "Archive Gate",
				SummaryText:  "Alice studies the sealed archive gate.",
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("storylines include reference and stale fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/storylines/sess-1?current_turn=8", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["reference_turn"] != float64(8) {
			t.Fatalf("reference_turn = %v, want 8", resp["reference_turn"])
		}
		items := resp["storylines"].([]any)
		first := items[0].(map[string]any)
		if first["last_observed_turn"] != float64(5) || first["freshness_turn_gap"] != float64(3) {
			t.Fatalf("stale snapshot = %#v", first)
		}
		if first["is_stale"] != true || first["stale_reason"] != "low_evidence_gap" {
			t.Fatalf("stale fields = %#v", first)
		}
	})

	t.Run("world rules inherited expose Python-compatible rules and scope chain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/world-rules/sess-1/inherited?active_scope=location&scope_name=Archive", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["active_scope"] != "location" || resp["scope_name"] != "Archive" {
			t.Fatalf("scope fields = %#v", resp)
		}
		chain := resp["scope_chain"].([]any)
		if len(chain) != 4 || chain[0] != "location" || chain[1] != "region" || chain[2] != "root" || chain[3] != "session" {
			t.Fatalf("scope_chain = %#v", chain)
		}
		rules := resp["rules"].([]any)
		if len(rules) != 2 {
			t.Fatalf("rules len = %d, want 2", len(rules))
		}
	})

	t.Run("chapter dry run exposes interval preview fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chapters/dry-run", strings.NewReader(`{"chat_session_id":"sess-1","turn_index":60,"interval":60,"top_k":8}`))
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
		if resp["mode"] != "dry_run" || resp["triggered"] != true {
			t.Fatalf("dry-run fields = %#v", resp)
		}
		candidate := resp["candidate_range"].(map[string]any)
		if candidate["from_turn"] != float64(1) || candidate["to_turn"] != float64(60) {
			t.Fatalf("candidate_range = %#v", candidate)
		}
	})

	t.Run("search responses keep 0.8 aliases", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-1","query":"Alice","top_k":1}`))
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
		if resp["query"] != "Alice" || resp["count"] != float64(1) {
			t.Fatalf("search envelope = %#v", resp)
		}
		if _, ok := resp["episodes"].([]any); !ok {
			t.Fatalf("missing episodes alias: %#v", resp)
		}
	})
}

func TestStorylineRegistryWriteRoutes(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 7, ChatSessionID: "sess-1", Name: "Old Arc", Status: "active", EvidenceCount: 1, LastEvidenceTurn: 3, FirstTurn: 2, LastTurn: 3},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("patch storyline updates allowed fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"status":"paused","key_points_json":["beat","beat"],"ongoing_tensions_json":["answer","answer"],"confidence":0.75,"evidence_count":3,"last_evidence_turn":9}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if len(fake.storylinePatches) != 1 {
			t.Fatalf("patch calls = %d, want 1", len(fake.storylinePatches))
		}
		if fake.storylinePatches[0]["key_points_json"] != `["beat"]` {
			t.Fatalf("deduped key_points_json = %#v", fake.storylinePatches[0]["key_points_json"])
		}
		if fake.storylinePatches[0]["ongoing_tensions_json"] != `["answer"]` {
			t.Fatalf("deduped ongoing_tensions_json = %#v", fake.storylinePatches[0]["ongoing_tensions_json"])
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode patch response: %v", err)
		}
		if resp["confidence"] != float64(0.75) || resp["evidence_count"] != float64(3) || resp["last_evidence_turn"] != float64(9) {
			t.Fatalf("quality fields missing from patch response: %#v", resp)
		}
		updatedValues, _ := resp["updated_values"].(map[string]any)
		if updatedValues["confidence"] != float64(0.75) || updatedValues["last_evidence_turn"] != float64(9) {
			t.Fatalf("updated_values missing quality fields: %#v", updatedValues)
		}
	})

	t.Run("patch storyline rejects invalid quality fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"confidence":1.2}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("confidence status = %d, want 400: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"evidence_count":-1}`))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("evidence_count status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("trust patch updates flags", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7/trust", strings.NewReader(`{"pinned":true,"suppressed":false}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.storylineTrustPatch["pinned"] != true || fake.storylineTrustPatch["suppressed"] != false {
			t.Fatalf("trust patch = %#v", fake.storylineTrustPatch)
		}
	})

	t.Run("delete storyline uses live store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/storylines/7", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.deletedStorylineID != 7 {
			t.Fatalf("deleted id = %d, want 7", fake.deletedStorylineID)
		}
	})
}

func TestPendingThreadContinuityHookRoutes(t *testing.T) {
	fake := &narrativeFakeStore{
		pendingThreads: []store.PendingThread{
			{
				ID:               11,
				ChatSessionID:    "sess-1",
				ThreadKey:        "thread_rooftop_promise",
				Description:      "Mira answers the rooftop promise",
				Status:           "open",
				SourceTurn:       4,
				HookType:         "promise",
				HookMetadataJSON: `{"title":"Mira answers the rooftop promise","owner":"Nia","target":"Mira","confidence":0.82,"last_seen_turn":7}`,
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("continuity-hooks alias returns pending thread items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/continuity-hooks/sess-1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["fetched"] != true || resp["count"] != float64(1) {
			t.Fatalf("continuity hook envelope = %#v", resp)
		}
		items := resp["items"].([]any)
		first := items[0].(map[string]any)
		if first["title"] != "Mira answers the rooftop promise" || first["owner"] != "Nia" || first["target"] != "Mira" {
			t.Fatalf("metadata-derived fields missing: %#v", first)
		}
		if first["confidence"] != float64(0.82) || first["last_seen_turn"] != float64(7) {
			t.Fatalf("metadata-derived quality fields missing: %#v", first)
		}
	})

	t.Run("patch validates and forwards allowed fields", func(t *testing.T) {
		body := `{"status":"paused","thread_type":"open_question","title":"Ask Mira why she hesitated","owner":"Nia","target":"Mira","confidence":0.74,"details_json":{"reason":"follow-up"},"resolution_note":"waiting"}`
		req := httptest.NewRequest(http.MethodPatch, "/continuity-hooks/11", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if len(fake.pendingThreadPatches) != 1 {
			t.Fatalf("patch calls = %d, want 1", len(fake.pendingThreadPatches))
		}
		if fake.pendingThreadPatches[0]["thread_type"] != "open_question" || fake.pendingThreadPatches[0]["confidence"] != float64(0.74) {
			t.Fatalf("patch payload = %#v", fake.pendingThreadPatches[0])
		}
		if fake.pendingThreadPatches[0]["details_json"] != `{"reason":"follow-up"}` {
			t.Fatalf("details_json = %#v", fake.pendingThreadPatches[0]["details_json"])
		}
	})

	t.Run("patch rejects invalid thread type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/pending-threads/11", strings.NewReader(`{"thread_type":"misc"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("trust patch and delete use live store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/pending-threads/11/trust", strings.NewReader(`{"pinned":true,"suppressed":false}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("trust status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.pendingThreadTrust["pinned"] != true || fake.pendingThreadTrust["suppressed"] != false {
			t.Fatalf("trust payload = %#v", fake.pendingThreadTrust)
		}

		req = httptest.NewRequest(http.MethodDelete, "/pending-threads/11", nil)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.deletedPendingThread != 11 {
			t.Fatalf("deleted id = %d, want 11", fake.deletedPendingThread)
		}
	})
}

func TestSessionStateAggregateReadBuildsCoreFiveSections(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 9, CreatedAt: now},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 2, ChatSessionID: "sess-1", LayerType: "scene", Content: `{"mood":"tense"}`, TurnIndex: 9, CreatedAt: now},
		},
		storylines: []store.Storyline{
			{ID: 3, ChatSessionID: "sess-1", Name: "Rooftop Promise", Status: "active", CurrentContext: "Mira owes Nia an answer.", KeyPointsJSON: `["promise"]`, OngoingTensionsJSON: `["answer pending"]`, Confidence: 0.8, EvidenceCount: 2, LastEvidenceTurn: 8, FirstTurn: 4, LastTurn: 8, UpdatedAt: now},
			{ID: 4, ChatSessionID: "sess-1", Name: "Suppressed Arc", Status: "active", LastTurn: 9, Suppressed: true, UpdatedAt: now},
		},
		characterStates: []store.CharacterState{
			{ID: 5, ChatSessionID: "sess-1", CharacterName: "Mira", StatusJSON: `{"emotion":"conflicted"}`, TurnIndex: 8, CreatedAt: now, UpdatedAt: now},
		},
		worldRules: []store.WorldRule{
			{ID: 6, ChatSessionID: "sess-1", Scope: "session", Category: "promise", Key: "answers_need_followup", ValueJSON: `{"rule":"Do not drop promises."}`, SourceTurn: 7, UpdatedAt: now},
			{ID: 7, ChatSessionID: "sess-1", Scope: "session", Category: "hidden", Key: "suppressed", ValueJSON: `{}`, SourceTurn: 9, Suppressed: true, UpdatedAt: now},
		},
		pendingThreads: []store.PendingThread{
			{ID: 8, ChatSessionID: "sess-1", ThreadKey: "thread_rooftop", Description: "Mira answers Nia", Status: "open", SourceTurn: 6, LastSeenTurn: 9, HookType: "promise", HookMetadataJSON: `{"title":"Mira answers Nia","owner":"Nia","target":"Mira","confidence":0.9}`, UpdatedAt: now},
			{ID: 9, ChatSessionID: "sess-1", ThreadKey: "thread_suppressed", Description: "Hidden", Status: "open", SourceTurn: 8, Suppressed: true, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/session-state/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fake.aggregateReadCalls != 1 {
		t.Fatalf("aggregate snapshot calls = %d, want 1", fake.aggregateReadCalls)
	}
	if fake.listChatLogCalls != 0 {
		t.Fatalf("ListChatLogs fallback calls = %d, want 0 when aggregate snapshot provides recent logs", fake.listChatLogCalls)
	}
	if resp["snapshot_status"] != "ready" {
		t.Fatalf("snapshot_status = %v, want ready: %#v", resp["snapshot_status"], resp)
	}
	if len(resp["storylines"].([]any)) != 1 || len(resp["world_rules"].([]any)) != 1 || len(resp["pending_threads"].([]any)) != 1 {
		t.Fatalf("suppressed rows were not filtered: story=%#v world=%#v threads=%#v", resp["storylines"], resp["world_rules"], resp["pending_threads"])
	}
	if _, ok := resp["continuity_hooks"].([]any); !ok {
		t.Fatalf("continuity_hooks alias missing: %#v", resp)
	}
	meta := resp["section_meta"].(map[string]any)
	for _, key := range []string{"active_states", "storylines", "characters", "world_rules", "pending_threads", "continuity_hooks", "canonical_state_layer"} {
		m, ok := meta[key].(map[string]any)
		if !ok {
			t.Fatalf("meta %s missing: %#v", key, meta)
		}
		if m["ready"] != true || m["count"] != float64(1) {
			t.Fatalf("meta %s = %#v, want ready count=1", key, m)
		}
		if _, exists := m["last_turn"]; !exists {
			t.Fatalf("meta %s missing last_turn: %#v", key, m)
		}
		if _, exists := m["updated_at"]; !exists {
			t.Fatalf("meta %s missing updated_at: %#v", key, m)
		}
	}
}

func TestMomentumPacketBuildsStorylineHookRules(t *testing.T) {
	now := time.Date(2026, 5, 31, 11, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 14, CreatedAt: now},
		},
		storylines: []store.Storyline{
			{ID: 10, ChatSessionID: "sess-1", Name: "Confession Aftermath", Status: "active", CurrentContext: "Mira still owes Nia an answer.", KeyPointsJSON: `["rooftop hesitation","rooftop hesitation","answer promised"]`, OngoingTensionsJSON: `["answer the confession"]`, Confidence: 0.85, EvidenceCount: 4, FirstTurn: 4, LastTurn: 14, UpdatedAt: now},
			{ID: 12, ChatSessionID: "sess-1", Name: "Stairwell Echo", Status: "active", CurrentContext: "The same hesitation keeps returning.", KeyPointsJSON: `["rooftop hesitation","hand on railing"]`, OngoingTensionsJSON: `["admit the fear"]`, Confidence: 0.72, EvidenceCount: 3, FirstTurn: 5, LastTurn: 13, UpdatedAt: now},
			{ID: 11, ChatSessionID: "sess-1", Name: "Suppressed", Status: "active", KeyPointsJSON: `["hidden"]`, OngoingTensionsJSON: `["hidden"]`, Suppressed: true, LastTurn: 15, UpdatedAt: now},
		},
		pendingThreads: []store.PendingThread{
			{ID: 21, ChatSessionID: "sess-1", ThreadKey: "thread_old", Description: "Ask why Mira paused", Status: "open", SourceTurn: 6, LastSeenTurn: 6, Priority: 2, HookType: "open_question", HookMetadataJSON: `{"title":"Ask why Mira paused"}`, UpdatedAt: now},
			{ID: 22, ChatSessionID: "sess-1", ThreadKey: "thread_new", Description: "Follow the rooftop answer", Status: "paused", SourceTurn: 13, LastSeenTurn: 13, Priority: 1, HookType: "promise", HookMetadataJSON: `{"title":"Follow the rooftop answer"}`, UpdatedAt: now},
		},
		characterStates: []store.CharacterState{
			{ID: 31, ChatSessionID: "sess-1", CharacterName: "Mira", RelationshipsJSON: `{"Nia":{"summary":"Mira owes Nia a direct answer after the confession.","trust":0.73}}`, TurnIndex: 14, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/momentum-packet/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["packet_status"] != "ready" {
		t.Fatalf("packet_status = %v, want ready: %#v", resp["packet_status"], resp)
	}
	for _, key := range []string{"next_pressure", "payoff_candidates", "tension_to_reuse", "beats_to_avoid"} {
		items, ok := resp[key].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("%s missing generated items: %#v", key, resp[key])
		}
		first := items[0].(map[string]any)
		for _, field := range []string{"label", "source_type", "source_id", "source_name", "priority"} {
			if _, exists := first[field]; !exists {
				t.Fatalf("%s first item missing %s: %#v", key, field, first)
			}
		}
	}
	payoffItems, _ := resp["payoff_candidates"].([]any)
	sourceTypes := map[string]bool{}
	for _, raw := range payoffItems {
		item, _ := raw.(map[string]any)
		sourceTypes[fmt.Sprint(item["source_type"])] = true
	}
	for _, want := range []string{"storyline", "relationship", "pending_thread"} {
		if !sourceTypes[want] {
			t.Fatalf("payoff_candidates missing %s source type: %#v", want, payoffItems)
		}
	}
}

func TestStorylineRegistrySyncDryRunAndApply(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 3, ChatSessionID: "sess-1", Name: "Rooftop Promise", Status: "active", EvidenceCount: 1, LastEvidenceTurn: 2, FirstTurn: 1, LastTurn: 2},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	dryBody := `{"chat_session_id":"sess-1","mode":"dry_run","supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","key_points":["confession","confession"],"ongoing_tensions":["answer pending","answer pending"],"confidence":0.8,"evidence_count":2,"last_evidence_turn":5}]}}`
	req := httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(dryBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 0 {
		t.Fatalf("dry-run saved storylines = %d, want 0", len(fake.savedStorylines))
	}
	var dryResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dryResp); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if dryResp["mode"] != "dry_run" || dryResp["valid_count"] != float64(1) {
		t.Fatalf("dry-run response = %#v", dryResp)
	}
	candidates := dryResp["candidates"].([]any)
	candidate := candidates[0].(map[string]any)
	if candidate["key_points_json"] != `["confession"]` || candidate["ongoing_tensions_json"] != `["answer pending"]` {
		t.Fatalf("dry-run candidate did not normalize lists: %#v", candidate)
	}
	if candidate["confidence"] != float64(0.8) || candidate["evidence_count"] != float64(2) || candidate["last_evidence_turn"] != float64(5) {
		t.Fatalf("dry-run candidate missing quality fields: %#v", candidate)
	}

	applyBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":4,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"She waits for the answer.","key_points":["confession"],"ongoing_tensions":["answer pending"],"confidence":0.8}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(applyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 1 {
		t.Fatalf("apply saved storylines = %d, want 1", len(fake.savedStorylines))
	}
	saved := fake.savedStorylines[0]
	if saved.EvidenceCount != 2 || saved.LastEvidenceTurn != 4 || saved.FirstTurn != 1 || saved.LastTurn != 4 {
		t.Fatalf("saved storyline evidence/turns = %#v", saved)
	}
	var applyResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &applyResp); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	results := applyResp["results"].([]any)
	result := results[0].(map[string]any)
	if result["confidence"] != float64(0.8) || result["evidence_count"] != float64(2) || result["last_evidence_turn"] != float64(4) {
		t.Fatalf("apply result missing quality fields: %#v", result)
	}

	duplicateBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":4,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"She still waits.","last_evidence_turn":4}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(duplicateBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 2 {
		t.Fatalf("duplicate saved storylines = %d, want 2", len(fake.savedStorylines))
	}
	duplicateSaved := fake.savedStorylines[1]
	if duplicateSaved.EvidenceCount != 2 || duplicateSaved.LastEvidenceTurn != 4 {
		t.Fatalf("duplicate sync double-counted evidence: %#v", duplicateSaved)
	}

	explicitBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":6,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"The answer lands.","evidence_count":3,"last_evidence_turn":6}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(explicitBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explicit status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 3 {
		t.Fatalf("explicit saved storylines = %d, want 3", len(fake.savedStorylines))
	}
	explicitSaved := fake.savedStorylines[2]
	if explicitSaved.EvidenceCount != 5 || explicitSaved.LastEvidenceTurn != 6 {
		t.Fatalf("explicit quality fields were not applied as increment/current turn: %#v", explicitSaved)
	}
}

func TestStorylineQualityGateFiveTurnReplayEvidenceAndSelection(t *testing.T) {
	const sid = "sess-h2-five-turn"
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	var current *store.Storyline
	postSync := func(turn int) store.Storyline {
		if current != nil {
			fake.storylines = []store.Storyline{*current}
		}
		body := fmt.Sprintf(`{"chat_session_id":%q,"mode":"apply","turn_index":%d,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"Turn %d keeps the same promise active.","key_points":["shared vow","shared vow","turn %d evidence"],"ongoing_tensions":["answer pending","answer pending"],"confidence":0.8}]}}`, sid, turn, turn, turn)
		req := httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("sync turn %d status = %d, want 200: %s", turn, rec.Code, rec.Body.String())
		}
		if len(fake.savedStorylines) == 0 {
			t.Fatalf("sync turn %d saved no storyline", turn)
		}
		saved := fake.savedStorylines[len(fake.savedStorylines)-1]
		current = &saved
		return saved
	}

	for turn := 1; turn <= 5; turn++ {
		saved := postSync(turn)
		if saved.LastEvidenceTurn != turn || saved.LastTurn != turn {
			t.Fatalf("turn %d saved turns = %#v", turn, saved)
		}
	}
	if current == nil || current.EvidenceCount != 5 || current.LastEvidenceTurn != 5 {
		t.Fatalf("five-turn evidence accumulation = %#v, want evidence=5 last_evidence_turn=5", current)
	}

	duplicateBefore := *current
	duplicate := postSync(5)
	if duplicate.EvidenceCount != duplicateBefore.EvidenceCount || duplicate.LastEvidenceTurn != duplicateBefore.LastEvidenceTurn {
		t.Fatalf("duplicate same-turn sync double-counted evidence: before=%#v after=%#v", duplicateBefore, duplicate)
	}
	current = &duplicateBefore

	fake.storylines = []store.Storyline{
		*current,
		{
			ID:               99,
			ChatSessionID:    sid,
			Name:             "Stale High",
			Status:           "active",
			CurrentContext:   "stale high should not guide",
			Confidence:       0.99,
			EvidenceCount:    1,
			LastEvidenceTurn: 1,
			LastTurn:         5,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/storylines/"+sid, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("storylines status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var storyResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &storyResp); err != nil {
		t.Fatalf("decode storylines: %v", err)
	}
	rows, _ := storyResp["storylines"].([]any)
	if len(rows) != 2 {
		t.Fatalf("storylines rows = %#v, want fresh and stale", storyResp)
	}
	fresh := rows[0].(map[string]any)
	if fresh["evidence_count"] != float64(5) || fresh["last_evidence_turn"] != float64(5) || fresh["last_observed_turn"] != float64(5) || fresh["freshness_turn_gap"] != float64(0) {
		t.Fatalf("fresh storyline quality/freshness fields = %#v", fresh)
	}
	if strings.Count(fmt.Sprint(fresh["key_points_json"]), "shared vow") != 1 || strings.Count(fmt.Sprint(fresh["ongoing_tensions_json"]), "answer pending") != 1 {
		t.Fatalf("fresh storyline read path did not dedupe key/tension lists: %#v", fresh)
	}

	req = httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(fmt.Sprintf(`{"chat_session_id":%q,"context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`, sid)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("supervisor status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var supResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &supResp); err != nil {
		t.Fatalf("decode supervisor: %v", err)
	}
	pack := supResp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["reference_turn"] != float64(5) || selection["selected_count"] != float64(1) || selection["stale_selected_count"] != float64(0) || selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("storyline selection counts = %#v", selection)
	}
	contextText := extractionStringFromAny(pack["storylines_context"])
	if !strings.Contains(contextText, "Turn 5 keeps the same promise active.") || strings.Contains(contextText, "stale high should not guide") {
		t.Fatalf("storyline context did not select fresh-only row: %q", contextText)
	}
}

func TestSupervisorStorylineSelectionExposesQualityTrace(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-e1f-supervisor", Name: "Fresh arc", Status: "active", CurrentContext: "Fresh arc should guide the next beat", KeyPointsJSON: `["fresh beat","fresh beat"," fresh beat "]`, OngoingTensionsJSON: `["answer pending","answer pending"]`, Confidence: 0.85, EvidenceCount: 3, LastEvidenceTurn: 10, LastTurn: 10},
			{ID: 2, ChatSessionID: "sess-e1f-supervisor", Name: "Stale arc", Status: "active", CurrentContext: "Stale arc should not repeat", KeyPointsJSON: `["stale beat","stale beat"]`, OngoingTensionsJSON: `["old tension","old tension"]`, Confidence: 0.95, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 1},
			{ID: 3, ChatSessionID: "sess-e1f-supervisor", Name: "Resolved arc", Status: "resolved", CurrentContext: "Resolved arc stays summary-only", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 6},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(`{"chat_session_id":"sess-e1f-supervisor","context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`))
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
	pack := resp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("storyline_selection = %#v, want one selected and one stale dropped", selection)
	}
	if selection["resolved_summary_count"] != float64(1) {
		t.Fatalf("resolved_summary_count = %v, want 1", selection["resolved_summary_count"])
	}
	contextText, _ := pack["storylines_context"].(string)
	if !strings.Contains(contextText, "Fresh arc") {
		t.Fatalf("storylines_context missing selected storyline: %q", contextText)
	}
	if strings.Contains(contextText, "Stale arc should not repeat") || strings.Contains(contextText, "stale beat") || strings.Contains(contextText, "Resolved arc stays summary-only") {
		t.Fatalf("storylines_context leaked stale/resolved full context: %q", contextText)
	}
	if !strings.Contains(contextText, "[Resolved Storylines Summary]") || !strings.Contains(contextText, "Resolved arc resolved at turn 6") {
		t.Fatalf("storylines_context missing resolved compressed summary: %q", contextText)
	}
	if strings.Count(contextText, "fresh beat") != 1 || strings.Count(contextText, "answer pending") != 1 {
		t.Fatalf("storylines_context did not dedupe key/tension fields: %q", contextText)
	}
	trace := resp["trace_summary"].(map[string]any)
	if trace["storyline_read_status"] != "ok" {
		t.Fatalf("storyline_read_status = %v, want ok", trace["storyline_read_status"])
	}
}

func TestFormatStorylinesForSupervisorSkipsSelfEchoDetails(t *testing.T) {
	selection := storylineSupervisorSelection{
		Selected: []storylineSelectionEntry{
			{
				Item: store.Storyline{
					Name:                "루나의 고향 마을 방문 약속",
					Status:              "active",
					CurrentContext:      "루나의 고향 마을을 답사 경로에 포함시키기로 함",
					KeyPointsJSON:       `["루나의 고향 마을 방문 약속","서부 답사 전 루나에게 동선 확인"]`,
					OngoingTensionsJSON: `["루나의 고향 마을 방문 약속","방문 시점 조율 필요"]`,
					Confidence:          0.82,
					EvidenceCount:       3,
				},
				Confidence: 0.82,
			},
			{
				Item: store.Storyline{
					Name:                "점심 약속 (시우-루나)",
					Status:              "active",
					KeyPointsJSON:       `["점심 약속 (시우-루나)","약속 장소를 정해야 함"]`,
					OngoingTensionsJSON: `["약속 장소를 정해야 함"]`,
					Confidence:          0.7,
					EvidenceCount:       1,
				},
				Confidence: 0.7,
			},
		},
	}

	text := formatStorylinesForSupervisor(selection)
	if strings.Contains(text, "key_points: 루나의 고향 마을 방문 약속") || strings.Contains(text, "tensions: 루나의 고향 마을 방문 약속") {
		t.Fatalf("storylines_context repeated title-equivalent detail: %q", text)
	}
	if strings.Count(text, "점심 약속 (시우-루나)") != 1 {
		t.Fatalf("fallback name should appear only as the main storyline line: %q", text)
	}
	for _, want := range []string{"서부 답사 전 루나에게 동선 확인", "방문 시점 조율 필요", "약속 장소를 정해야 함"} {
		if !strings.Contains(text, want) {
			t.Fatalf("storylines_context dropped non-duplicate detail %q: %q", want, text)
		}
	}
}

func TestSupervisorStorylineManualBatchSyncShapeDropsStaleHigh(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-h2d-manual", Name: "Fresh Current", Status: "active", CurrentContext: "fresh current should guide", Confidence: 0.75, EvidenceCount: 2, LastEvidenceTurn: 10, LastTurn: 10},
			{ID: 2, ChatSessionID: "sess-h2d-manual", Name: "Stale High", Status: "active", CurrentContext: "stale high should not guide", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 10},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(`{"chat_session_id":"sess-h2d-manual","context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`))
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
	pack := resp["supervisor_input_pack"].(map[string]any)
	contextText := extractionStringFromAny(pack["storylines_context"])
	if !strings.Contains(contextText, "fresh current should guide") || strings.Contains(contextText, "stale high should not guide") {
		t.Fatalf("manual batch-sync shape leaked stale context: %q", contextText)
	}
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) || selection["stale_selected_count"] != float64(0) {
		t.Fatalf("storyline_selection counts = %#v", selection)
	}
	dropped, _ := selection["dropped"].([]any)
	if len(dropped) != 1 {
		t.Fatalf("dropped = %#v, want one stale row", dropped)
	}
	staleHigh, _ := dropped[0].(map[string]any)
	if staleHigh["name"] != "Stale High" || staleHigh["last_observed_turn"] != float64(1) || staleHigh["freshness_turn_gap"] != float64(9) || staleHigh["is_stale"] != true {
		t.Fatalf("stale high debug fields = %#v", staleHigh)
	}
	if staleHigh["stale_reason"] != "low_evidence_gap" {
		t.Fatalf("stale high reason = %#v", staleHigh["stale_reason"])
	}
}

func TestStorylineSelectionOrdersFreshRowsAndDropsStale(t *testing.T) {
	referenceTurn := 20
	items := []store.Storyline{
		{ID: 1, Name: "Gap two high confidence", Status: "active", CurrentContext: "gap two", Confidence: 0.99, EvidenceCount: 5, LastEvidenceTurn: 18, LastTurn: 20},
		{ID: 2, Name: "Gap one low confidence", Status: "active", CurrentContext: "gap one low", Confidence: 0.20, EvidenceCount: 1, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 3, Name: "Gap one high confidence", Status: "active", CurrentContext: "gap one high", Confidence: 0.80, EvidenceCount: 1, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 4, Name: "Gap one high evidence", Status: "active", CurrentContext: "gap one evidence", Confidence: 0.80, EvidenceCount: 6, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 5, Name: "Stale high confidence", Status: "active", CurrentContext: "stale should drop", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 20},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 4)
	got := storylineSelectionNames(selection.Selected)
	want := []string{"Gap one high evidence", "Gap one high confidence", "Gap one low confidence", "Gap two high confidence"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("selected order = %#v, want %#v", got, want)
	}
	dropped := storylineSelectionNames(selection.Dropped)
	if strings.Join(dropped, "|") != "Stale high confidence" {
		t.Fatalf("dropped = %#v, want stale row only", dropped)
	}

	summary := storylineSelectionSummary(selection)
	if summary["stale_dropped_count"] != 1 || summary["stale_selected_count"] != 0 || summary["fresh_rows_take_priority"] != true {
		t.Fatalf("selection summary = %#v", summary)
	}
	contextText := formatStorylinesForSupervisor(selection)
	if strings.Contains(contextText, "stale should drop") {
		t.Fatalf("stale storyline leaked into prompt context: %q", contextText)
	}
}

func TestStorylineSelectionSixActiveRowsDropsStaleAndLowPriority(t *testing.T) {
	referenceTurn := 12
	items := []store.Storyline{
		{ID: 1, Name: "Fresh high evidence", Status: "active", CurrentContext: "fresh high evidence", Confidence: 0.90, EvidenceCount: 5, LastEvidenceTurn: 12, LastTurn: 12},
		{ID: 2, Name: "Fresh high confidence", Status: "active", CurrentContext: "fresh high confidence", Confidence: 0.95, EvidenceCount: 2, LastEvidenceTurn: 11, LastTurn: 12},
		{ID: 3, Name: "Fresh medium", Status: "active", CurrentContext: "fresh medium", Confidence: 0.70, EvidenceCount: 2, LastEvidenceTurn: 10, LastTurn: 12},
		{ID: 4, Name: "Fresh low priority", Status: "active", CurrentContext: "fresh low priority should drop by limit", Confidence: 0.10, EvidenceCount: 1, LastEvidenceTurn: 10, LastTurn: 12},
		{ID: 5, Name: "Stale high confidence", Status: "active", CurrentContext: "stale high should drop", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 12},
		{ID: 6, Name: "Stale medium", Status: "active", CurrentContext: "stale medium should drop", Confidence: 0.50, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 12},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 3)
	if got := strings.Join(storylineSelectionNames(selection.Selected), "|"); got != "Fresh high evidence|Fresh high confidence|Fresh medium" {
		t.Fatalf("selected = %q, want top three fresh rows", got)
	}
	dropped := strings.Join(storylineSelectionNames(selection.Dropped), "|")
	for _, unwanted := range []string{"Stale high confidence", "Stale medium", "Fresh low priority"} {
		if !strings.Contains(dropped, unwanted) {
			t.Fatalf("dropped = %q, missing %q", dropped, unwanted)
		}
	}
	summary := storylineSelectionSummary(selection)
	if summary["selected_count"] != 3 || summary["dropped_count"] != 3 || summary["stale_dropped_count"] != 2 {
		t.Fatalf("six-active selection summary = %#v", summary)
	}
	if contextText := formatStorylinesForSupervisor(selection); strings.Contains(contextText, "stale high should drop") || strings.Contains(contextText, "fresh low priority should drop by limit") {
		t.Fatalf("dropped storyline leaked into supervisor context: %q", contextText)
	}
}

func TestStorylineSelectionFallsBackToMostRecentStaleWhenNoFreshRows(t *testing.T) {
	referenceTurn := 35
	items := []store.Storyline{
		{ID: 1, Name: "Older stale", Status: "active", CurrentContext: "older stale fallback", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 35},
		{ID: 2, Name: "Recent stale", Status: "active", CurrentContext: "recent stale fallback", Confidence: 0.50, EvidenceCount: 1, LastEvidenceTurn: 31, LastTurn: 35},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 5)
	got := storylineSelectionNames(selection.Selected)
	if strings.Join(got, "|") != "Recent stale" {
		t.Fatalf("selected stale fallback = %#v, want most recent stale", got)
	}
	if len(selection.Dropped) != 1 || selection.Dropped[0].Item.Name != "Older stale" {
		t.Fatalf("dropped stale fallback = %#v", storylineSelectionNames(selection.Dropped))
	}
	summary := storylineSelectionSummary(selection)
	if summary["stale_selected_count"] != 1 || summary["stale_dropped_count"] != 1 {
		t.Fatalf("summary = %#v, want one stale selected and one stale dropped", summary)
	}
}

func TestStorylineSelectionDropsStaleRowsEvenWhenActiveCountFitsLimit(t *testing.T) {
	referenceTurn := 7
	items := []store.Storyline{
		{ID: 1, Name: "Fresh first", Status: "active", CurrentContext: "fresh one", Confidence: 0.60, EvidenceCount: 2, LastEvidenceTurn: 7, LastTurn: 7},
		{ID: 2, Name: "Fresh second", Status: "active", CurrentContext: "fresh two", Confidence: 0.55, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 7},
		{ID: 3, Name: "Stale high confidence", Status: "active", CurrentContext: "stale high should not guide", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 7},
		{ID: 4, Name: "Stale second", Status: "active", CurrentContext: "stale second should not guide", Confidence: 0.95, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 7},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 5)
	selected := storylineSelectionNames(selection.Selected)
	if strings.Join(selected, "|") != "Fresh first|Fresh second" {
		t.Fatalf("selected = %#v, want only fresh rows even though active count fits limit", selected)
	}
	dropped := storylineSelectionNames(selection.Dropped)
	if strings.Join(dropped, "|") != "Stale second|Stale high confidence" {
		t.Fatalf("dropped = %#v, want stale rows dropped", dropped)
	}
	summary := storylineSelectionSummary(selection)
	if summary["selected_count"] != 2 || summary["stale_selected_count"] != 0 || summary["stale_dropped_count"] != 2 {
		t.Fatalf("selection summary = %#v, want selected=2 stale_selected=0 stale_dropped=2", summary)
	}
	contextText := formatStorylinesForSupervisor(selection)
	if strings.Contains(contextText, "stale high should not guide") || strings.Contains(contextText, "stale second should not guide") {
		t.Fatalf("stale storyline leaked into prompt context: %q", contextText)
	}
}

func TestTrustControlsFilterAndPrioritizeNarrativeInjection(t *testing.T) {
	referenceTurn := 30
	storylines := []store.Storyline{
		{ID: 1, Name: "Pinned stale", Status: "active", CurrentContext: "pinned should stay in budget", Pinned: true, Confidence: 0.4, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 30},
		{ID: 2, Name: "Fresh normal", Status: "active", CurrentContext: "fresh should follow pinned", Confidence: 0.8, EvidenceCount: 3, LastEvidenceTurn: 30, LastTurn: 30},
		{ID: 3, Name: "Suppressed", Status: "active", CurrentContext: "suppressed must not guide", Suppressed: true, Confidence: 1, EvidenceCount: 8, LastEvidenceTurn: 30, LastTurn: 30},
	}
	selection := selectStorylinesForSupervisor(storylines, &referenceTurn, 2)
	if got := strings.Join(storylineSelectionNames(selection.Selected), "|"); got != "Pinned stale|Fresh normal" {
		t.Fatalf("selected = %q, want pinned first then fresh normal", got)
	}
	if len(selection.Suppressed) != 1 || selection.Suppressed[0].Item.Name != "Suppressed" {
		t.Fatalf("suppressed storylines = %#v", storylineSelectionNames(selection.Suppressed))
	}
	first := storylineSelectionEntryMap(selection.Selected[0])
	if first["pinned"] != true || first["suppressed"] != false {
		t.Fatalf("selected map missing trust flags: %#v", first)
	}
	if contextText := formatStorylinesForSupervisor(selection); strings.Contains(contextText, "suppressed must not guide") {
		t.Fatalf("suppressed storyline leaked into prompt context: %q", contextText)
	}

	worldRules := visibleSessionStateWorldRules([]store.WorldRule{
		{ID: 1, Scope: "session", Category: "plain", Key: "normal"},
		{ID: 2, Scope: "session", Category: "pinned", Key: "must_keep", Pinned: true},
		{ID: 3, Scope: "session", Category: "suppressed", Key: "must_drop", Suppressed: true},
	})
	if len(worldRules) != 2 || worldRules[0].ID != 2 || worldRules[1].ID == 3 {
		t.Fatalf("world rule trust filtering/order = %#v", worldRules)
	}

	hooks := continuityPendingThreads([]store.PendingThread{
		{ID: 1, Description: "normal hook", Status: "open", LastSeenTurn: 30},
		{ID: 2, Description: "pinned hook", Status: "open", LastSeenTurn: 1, Pinned: true},
		{ID: 3, Description: "suppressed hook", Status: "open", LastSeenTurn: 99, Suppressed: true},
	}, 0)
	if len(hooks) != 2 || hooks[0].ID != 2 || hooks[1].ID == 3 {
		t.Fatalf("pending thread trust filtering/order = %#v", hooks)
	}
}

func TestWorldGraphLiteActiveScopeAndInheritedRules(t *testing.T) {
	fake := &narrativeFakeStore{
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-world", Scope: "root", Category: "worldview", Key: "gravity", ValueJSON: `"stable"`, Pinned: true},
			{ID: 2, ChatSessionID: "sess-world", Scope: "region", Category: "weather", Key: "north_wind", ValueJSON: `"cold"`},
			{ID: 3, ChatSessionID: "sess-world", Scope: "location", ScopeName: "Archive", Category: "place", Key: "quiet_rule", ValueJSON: `"whisper"`},
			{ID: 4, ChatSessionID: "sess-world", Scope: "location", ScopeName: "Cellar", Category: "place", Key: "wrong_place", ValueJSON: `"exclude"`},
			{ID: 5, ChatSessionID: "sess-world", Scope: "root", Category: "hidden", Key: "suppressed", Suppressed: true},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/session/sess-world/active-scope", strings.NewReader(`{"active_scope":"location","scope_name":"Archive"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var scopeResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &scopeResp); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if scopeResp["active_scope"] != "location" || scopeResp["scope_name"] != "Archive" {
		t.Fatalf("active scope response = %#v", scopeResp)
	}
	chain := scopeResp["scope_chain"].([]any)
	if strings.Join([]string{fmt.Sprint(chain[0]), fmt.Sprint(chain[1]), fmt.Sprint(chain[2]), fmt.Sprint(chain[3])}, ">") != "location>region>root>session" {
		t.Fatalf("scope_chain = %#v, want location>region>root>session", chain)
	}

	req = httptest.NewRequest(http.MethodGet, "/world-rules/sess-world/inherited", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inherited status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode inherited: %v", err)
	}
	if resp["active_scope"] != "location" || resp["scope_name"] != "Archive" {
		t.Fatalf("inherited active scope = %#v", resp)
	}
	rules := resp["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3: %#v", len(rules), rules)
	}
	keys := []string{}
	inheritedByKey := map[string]bool{}
	for _, raw := range rules {
		item := raw.(map[string]any)
		key := fmt.Sprint(item["key"])
		keys = append(keys, key)
		inheritedByKey[key], _ = item["inherited"].(bool)
	}
	if strings.Join(keys, "|") != "quiet_rule|north_wind|gravity" {
		t.Fatalf("rule keys = %#v, want active location then region/root", keys)
	}
	if inheritedByKey["quiet_rule"] || !inheritedByKey["north_wind"] || !inheritedByKey["gravity"] {
		t.Fatalf("inherited flags = %#v", inheritedByKey)
	}
	for _, forbidden := range []string{"wrong_place", "suppressed"} {
		if strings.Contains(strings.Join(keys, "|"), forbidden) {
			t.Fatalf("forbidden rule leaked into inherited result: %s in %#v", forbidden, keys)
		}
	}
}

func TestStorylineStaleWindowClampAndLastEvidencePriority(t *testing.T) {
	referenceTurn := 20
	lowEvidence := store.Storyline{Status: "active", EvidenceCount: 0, LastEvidenceTurn: 1, LastTurn: 19}
	lowSnapshot := storylineStaleSnapshot(lowEvidence, &referenceTurn)
	if lowSnapshot["last_observed_turn"] != 1 || lowSnapshot["stale_after_turns"] != 3 || lowSnapshot["freshness_turn_gap"] != 19 || lowSnapshot["is_stale"] != true {
		t.Fatalf("low evidence snapshot = %#v", lowSnapshot)
	}
	if lowSnapshot["stale_reason"] != "low_evidence_gap" {
		t.Fatalf("low evidence stale_reason = %#v", lowSnapshot["stale_reason"])
	}

	highEvidence := store.Storyline{Status: "active", EvidenceCount: 20, LastEvidenceTurn: 15, LastTurn: 19}
	highSnapshot := storylineStaleSnapshot(highEvidence, &referenceTurn)
	if highSnapshot["last_observed_turn"] != 15 || highSnapshot["stale_after_turns"] != 8 || highSnapshot["freshness_turn_gap"] != 5 || highSnapshot["is_stale"] != false {
		t.Fatalf("high evidence snapshot = %#v", highSnapshot)
	}
}

func storylineSelectionNames(items []storylineSelectionEntry) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Item.Name)
	}
	return out
}
