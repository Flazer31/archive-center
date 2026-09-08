package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestAssistantTimelineRecoveryRegressionFixtureIsVersioned(t *testing.T) {
	raw, err := os.ReadFile("testdata/assistant_timeline_recovery_cases.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ContractVersion string           `json:"contract_version"`
		Cases           []map[string]any `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	caseIDs := map[string]bool{}
	for _, item := range fixture.Cases {
		caseIDs[stringFromMap(item, "id")] = true
	}
	if fixture.ContractVersion != "assistant_timeline_recovery_cases.v1" ||
		!caseIDs["deleted_tail_36_38_poisoned_client_anchor"] ||
		!caseIDs["deleted_user_inputs_keep_all_assistant_outputs"] {
		t.Fatalf("fixture=%+v ids=%+v", fixture, caseIDs)
	}
}

type durableRoutingBaselineStore struct {
	store.Store
	*rollbackRouteBindingFixture
	baseline      *store.SessionRoutingBaseline
	activeSources []store.MemorySourceRevision
}

type rollbackDecisionChatLogStore struct {
	store.Store
	*rollbackRouteBindingFixture
	logs             []store.ChatLog
	activeSources    []store.MemorySourceRevision
	baseline         *store.SessionRoutingBaseline
	latestTurnCalls  int
	listChatLogsFrom int
	listChatLogsTo   int
}

// rollbackDecisionExecutionStore exercises the real decision and rollback
// handlers against one store object. It keeps the exact active source evidence
// used by the decision owner and the actual RollbackStore calls recorded by the
// existing rollbackRecordingStore.
type rollbackDecisionExecutionStore struct {
	*rollbackRecordingStore
	*rollbackRouteBindingFixture
	logs          []store.ChatLog
	activeSources []store.MemorySourceRevision
}

type rollbackRoutedRecordingStore struct {
	*rollbackRecordingStore
	*rollbackRouteBindingFixture
}

const (
	rollbackTestStableCharacterID = "rollback-test-character"
	rollbackTestHostChatID        = "rollback-test-chat"
)

// rollbackRouteBindingFixture implements the production ResolveExisting store
// contract used by the rollback handlers. Tests may advance revision to model a
// route remap without replacing the handler or duplicating its policy.
type rollbackRouteBindingFixture struct {
	stableCharacterID string
	hostChatID        string
	canonicalSID      string
	revision          uint64
}

func newRollbackRouteBindingFixture(canonicalSID string) *rollbackRouteBindingFixture {
	return &rollbackRouteBindingFixture{
		stableCharacterID: rollbackTestStableCharacterID,
		hostChatID:        rollbackTestHostChatID,
		canonicalSID:      canonicalSID,
		revision:          1,
	}
}

func (f *rollbackRouteBindingFixture) BindSessionRoute(_ context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	if f == nil || req.Mode != store.SessionRouteBindingModeResolveExisting ||
		strings.TrimSpace(req.StableCharacterID) != f.stableCharacterID ||
		strings.TrimSpace(req.HostChatID) != f.hostChatID ||
		strings.TrimSpace(f.canonicalSID) == "" {
		return nil, store.ErrNotFound
	}
	return &store.SessionRouteBindingResult{
		Binding: store.SessionRouteBinding{
			ContractVersion:    store.SessionRouteBindingContractVersion,
			StableCharacterID:  f.stableCharacterID,
			HostChatID:         f.hostChatID,
			CanonicalSessionID: f.canonicalSID,
			BindingState:       "active",
			Revision:           f.revision,
		},
		ReadbackVerified: true,
	}, nil
}

func (s *durableRoutingBaselineStore) BindSessionRoute(ctx context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	return s.rollbackRouteBindingFixture.BindSessionRoute(ctx, req)
}

func (s *rollbackDecisionChatLogStore) BindSessionRoute(ctx context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	return s.rollbackRouteBindingFixture.BindSessionRoute(ctx, req)
}

func (s *rollbackDecisionExecutionStore) BindSessionRoute(ctx context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	return s.rollbackRouteBindingFixture.BindSessionRoute(ctx, req)
}

func (s *rollbackRoutedRecordingStore) BindSessionRoute(ctx context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	return s.rollbackRouteBindingFixture.BindSessionRoute(ctx, req)
}

func (s *rollbackDecisionExecutionStore) LatestSessionTurnIndex(context.Context, string) (int, error) {
	latest := 0
	for _, item := range s.logs {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	return latest, nil
}

func (s *rollbackDecisionExecutionStore) ListChatLogs(_ context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	result := make([]store.ChatLog, 0, len(s.logs))
	for _, item := range s.logs {
		if item.ChatSessionID != sid || (fromTurn > 0 && item.TurnIndex < fromTurn) || (toTurn > 0 && item.TurnIndex > toTurn) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *rollbackDecisionExecutionStore) ListActiveSourceRevisions(
	_ context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
) ([]store.MemorySourceRevision, error) {
	result := make([]store.MemorySourceRevision, 0, len(s.activeSources))
	for _, item := range s.activeSources {
		if item.ChatSessionID != chatSessionID || item.LifecycleState != "active" ||
			(fromTurn > 0 && item.TurnIndex < fromTurn) || (toTurn > 0 && item.TurnIndex > toTurn) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

type sessionIdentityRoutingStore struct {
	store.Store
	sessions []store.SessionSummary
	logs     map[string][]store.ChatLog
	baseline *store.SessionRoutingBaseline
}

type durableSessionIdentityBindingStore struct {
	store.Store
	bindings         map[string]string
	locks            map[string]string
	sources          map[string][]store.MemorySourceRevision
	history          map[string][]store.MemorySourceRevision
	lineage          []store.ForkLineageRecord
	baseline         *store.SessionRoutingBaseline
	fail             bool
	lastMode         string
	lineageListCalls int
}

func (s *durableSessionIdentityBindingStore) BindSessionRoute(_ context.Context, req store.SessionRouteBindingRequest) (*store.SessionRouteBindingResult, error) {
	if s.fail {
		return nil, context.DeadlineExceeded
	}
	if s.bindings == nil {
		s.bindings = map[string]string{}
	}
	s.lastMode = req.Mode
	key := req.StableCharacterID + "\x00" + req.HostChatID
	canonical, exists := s.bindings[key]
	if req.Mode == store.SessionRouteBindingModeResolveExisting && !exists {
		return nil, store.ErrNotFound
	}
	force := req.Mode == store.SessionRouteBindingModeManualAttach || req.Mode == store.SessionRouteBindingModeMigrationCommit
	if !exists || force {
		canonical = req.RequestedSessionID
	}
	redirected := false
	if target := s.locks[canonical]; target != "" {
		canonical = target
		redirected = true
	}
	if canonical == "" {
		return nil, store.ErrNotFound
	}
	s.bindings[key] = canonical
	return &store.SessionRouteBindingResult{
		Binding: store.SessionRouteBinding{
			ContractVersion:    store.SessionRouteBindingContractVersion,
			StableCharacterID:  req.StableCharacterID,
			HostChatID:         req.HostChatID,
			CanonicalSessionID: canonical,
			BindingState:       "active",
		},
		Created:              !exists,
		Updated:              exists && (force || redirected),
		ReadbackVerified:     true,
		LockedSourceRedirect: redirected,
	}, nil
}

func (s *durableSessionIdentityBindingStore) ListActiveSourceRevisions(_ context.Context, sid string, _, _ int) ([]store.MemorySourceRevision, error) {
	return append([]store.MemorySourceRevision(nil), s.sources[sid]...), nil
}

func (s *durableSessionIdentityBindingStore) ListSourceRevisions(_ context.Context, sid string, fromTurn, toTurn int) ([]store.MemorySourceRevision, error) {
	out := make([]store.MemorySourceRevision, 0, len(s.history[sid]))
	for _, source := range s.history[sid] {
		if fromTurn > 0 && source.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && source.TurnIndex > toTurn {
			continue
		}
		out = append(out, source)
	}
	return out, nil
}

func (s *durableSessionIdentityBindingStore) ListForkLineageRecords(_ context.Context, sid, _ string, limit int) ([]store.ForkLineageRecord, error) {
	s.lineageListCalls++
	out := make([]store.ForkLineageRecord, 0, len(s.lineage))
	for index := len(s.lineage) - 1; index >= 0; index-- {
		if s.lineage[index].ChatSessionID == sid {
			out = append(out, s.lineage[index])
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *durableSessionIdentityBindingStore) GetSessionRoutingBaseline(_ context.Context, targetSessionID string) (*store.SessionRoutingBaseline, error) {
	if s.baseline == nil || s.baseline.TargetSessionID != targetSessionID {
		return nil, store.ErrNotFound
	}
	copy := *s.baseline
	return &copy, nil
}

func TestSessionRoutingNormalObservationOmitsWorldlineAndDoesNotReadLineage(t *testing.T) {
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"stable\x00child-chat": "child-session"},
	}
	server := &Server{Store: st}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"child-session",
		"mode":"identity",
		"stable_character_id":"stable",
		"stable_character_id_state":"observed",
		"host_chat_id":"child-chat",
		"host_chat_id_state":"observed"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, exists := payload["worldline"]; exists || st.lineageListCalls != 0 {
		t.Fatalf("normal routing leaked/read worldline: exists=%t lineage_reads=%d body=%s", exists, st.lineageListCalls, rec.Body.String())
	}
}

func TestSessionRoutingAppliesWorldlineResolvedInSameRequest(t *testing.T) {
	route := func(t *testing.T, st *durableSessionIdentityBindingStore) sessionRoutingTurnResolutionResponse {
		t.Helper()
		server := &Server{Store: st}
		mux := http.NewServeMux()
		server.RegisterRoutes(mux)
		req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
			"chat_session_id":"child-session",
			"mode":"batch",
			"stable_character_id":"stable",
			"stable_character_id_state":"observed",
			"host_chat_id":"child-chat",
			"host_chat_id_state":"observed",
			"observations":[
				{"observation_index":0,"risu_user_message_index":0,"observed_pair_ordinal":1},
				{"observation_index":1,"risu_user_message_index":14,"observed_pair_ordinal":8},
				{"observation_index":2,"risu_user_message_index":16,"observed_pair_ordinal":9}
			],
			"worldline_observation":{
				"contract_version":"risu_worldline_observation.v2",
				"host_signal_source":"active_chat_pre_backfill",
				"branch_shape_contract":"risu_branchedfrom.v1",
				"observed_at_ms":1776000000100,
				"marker_state":"observed",
				"branch_marker":"{{specialcomment::branchedfrom::parent-chat::Parent::assistant-source::}}",
				"marker_index":2,
				"messages":[
					{"message_index":0,"role":"user","message_chat_id":"user-anchor","disabled":false},
					{"message_index":1,"role":"char","message_chat_id":"assistant-source","disabled":false},
					{"message_index":2,"role":"comment","message_chat_id":"","disabled":true}
				]
			}
		}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response sessionRoutingTurnResolutionResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	assertConfirmedBoundary := func(t *testing.T, response sessionRoutingTurnResolutionResponse) {
		t.Helper()
		if response.Worldline == nil || response.Worldline.State != "confirmed" ||
			response.Worldline.ParentSessionID != "parent-session" || response.Worldline.ForkTurn != 8 {
			t.Fatalf("worldline=%+v response=%+v", response.Worldline, response)
		}
		wantTurns := []int{1, 8, 9}
		wantResolutions := []string{"skip_pre_route_visible_pair", "skip_pre_route_visible_pair", "normal"}
		if len(response.ResolvedObservations) != len(wantTurns) {
			t.Fatalf("resolved observations=%+v", response.ResolvedObservations)
		}
		for index, item := range response.ResolvedObservations {
			if item.TurnIndex != wantTurns[index] || item.Resolution != wantResolutions[index] {
				t.Fatalf("resolved[%d]=%+v", index, item)
			}
		}
	}
	newStore := func() *durableSessionIdentityBindingStore {
		return &durableSessionIdentityBindingStore{
			Store: store.NewNoopStore(),
			bindings: map[string]string{
				"stable\x00child-chat":  "child-session",
				"stable\x00parent-chat": "parent-session",
			},
			sources: map[string][]store.MemorySourceRevision{},
			history: map[string][]store.MemorySourceRevision{},
		}
	}

	t.Run("first branch observation owns the boundary immediately", func(t *testing.T) {
		st := newStore()
		st.sources["parent-session"] = []store.MemorySourceRevision{
			activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "user-anchor", "assistant-source", 8),
		}
		assertConfirmedBoundary(t, route(t, st))
		if st.lineageListCalls != 1 {
			t.Fatalf("resolved worldline was read again: lineage_reads=%d", st.lineageListCalls)
		}
	})

	t.Run("historical recovery replaces unresolved ownership immediately", func(t *testing.T) {
		st := newStore()
		historical := activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "old-user-anchor", "assistant-source", 8)
		historical.LogicalTurnID = "canonical_turn_normalized"
		historical.LifecycleState = "superseded"
		st.history["parent-session"] = []store.MemorySourceRevision{historical}
		st.lineage = []store.ForkLineageRecord{{
			ContractVersion:  store.RisuWorldlineForkLineageContractVersion,
			LineageState:     "unresolved",
			ChatSessionID:    "child-session",
			IdempotencyKey:   "prior-unresolved-observation",
			DivergenceMarker: `{"reason":"parent_fork_source_history_unresolved"}`,
		}}
		assertConfirmedBoundary(t, route(t, st))
		if st.lineageListCalls != 1 {
			t.Fatalf("recovered worldline was read again: lineage_reads=%d", st.lineageListCalls)
		}
	})
}

func TestAutomaticActiveChatFullSweepUsesConfirmedWorldlineOwnership(t *testing.T) {
	routeObservation := func(t *testing.T, st *durableSessionIdentityBindingStore, userMessageIndex, observedPairOrdinal int, routingContext, baseline string) sessionRoutingTurnResolutionResponse {
		t.Helper()
		server := &Server{Store: st}
		mux := http.NewServeMux()
		server.RegisterRoutes(mux)
		body := fmt.Sprintf(`{
			"chat_session_id":"child-session",
			"mode":"pair",
			"stable_character_id":"stable",
			"stable_character_id_state":"observed",
			"host_chat_id":"child-chat",
			"host_chat_id_state":"observed",
			"risu_user_message_index":%d,
			"observed_pair_ordinal":%d,
			"routing_context":%q%s
		}`, userMessageIndex, observedPairOrdinal, routingContext, baseline)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response sessionRoutingTurnResolutionResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	routeWithContext := func(t *testing.T, st *durableSessionIdentityBindingStore, userMessageIndex int, routingContext, baseline string) sessionRoutingTurnResolutionResponse {
		t.Helper()
		return routeObservation(t, st, userMessageIndex, 0, routingContext, baseline)
	}
	route := func(t *testing.T, st *durableSessionIdentityBindingStore, userMessageIndex int, baseline string) sessionRoutingTurnResolutionResponse {
		t.Helper()
		return routeWithContext(t, st, userMessageIndex, automaticActiveChatFullSweep, baseline)
	}

	lineage := func(role string, forkTurn int, parent string) store.ForkLineageRecord {
		return store.ForkLineageRecord{
			ContractVersion:     store.RisuWorldlineForkLineageContractVersion,
			LineageState:        "confirmed",
			ChatSessionID:       "child-session",
			CopiedFromSessionID: parent,
			ForkTurn:            forkTurn,
			ForkSourceMessageID: "opaque-source",
			ForkSourceRole:      role,
			IdempotencyKey:      "opaque-key-" + parent,
		}
	}
	newStore := func(records ...store.ForkLineageRecord) *durableSessionIdentityBindingStore {
		return &durableSessionIdentityBindingStore{
			Store:    store.NewNoopStore(),
			bindings: map[string]string{"stable\x00child-chat": "child-session"},
			lineage:  records,
		}
	}

	t.Run("char source keeps the fork turn with the parent", func(t *testing.T) {
		st := newStore(lineage("char", 3, "parent-session"))
		inherited := route(t, st, 4, "")
		childOwned := route(t, st, 6, "")
		if inherited.Resolution != "skip_pre_route_visible_pair" || inherited.LocalTurnIndex != 3 || inherited.MinFromTurn != 4 {
			t.Fatalf("inherited=%+v", inherited)
		}
		if childOwned.Resolution != "normal" || childOwned.TurnIndex != 4 {
			t.Fatalf("child-owned=%+v", childOwned)
		}
	})

	t.Run("user source leaves the fork turn for the child", func(t *testing.T) {
		st := newStore(lineage("user", 3, "parent-session"))
		inherited := route(t, st, 2, "")
		childOwned := route(t, st, 4, "")
		if inherited.Resolution != "skip_pre_route_visible_pair" || inherited.LocalTurnIndex != 2 || inherited.MinFromTurn != 3 {
			t.Fatalf("inherited=%+v", inherited)
		}
		if childOwned.Resolution != "normal" || childOwned.TurnIndex != 3 {
			t.Fatalf("child-owned=%+v", childOwned)
		}
	})

	t.Run("first user source has no inherited completed turn", func(t *testing.T) {
		response := route(t, newStore(lineage("user", 1, "parent-session")), 0,
			`,"baseline":{"backend_turn_at_route":5,"local_pairs_at_route":2,"reason":"timeline_copy"}`)
		if response.Resolution != "normal" || response.TurnIndex != 1 || response.BaselineApplied {
			t.Fatalf("response=%+v", response)
		}
	})

	t.Run("legacy and conflicting rows are not ownership authority", func(t *testing.T) {
		legacy := lineage("", 3, "parent-session")
		legacy.ContractVersion = store.ForkLineageContractVersion
		legacyResponse := route(t, newStore(legacy), 0, "")
		if legacyResponse.Resolution != "worldline_ownership_unresolved" || legacyResponse.TurnIndex != 0 {
			t.Fatalf("legacy=%+v", legacyResponse)
		}

		first := lineage("char", 3, "parent-a")
		second := lineage("user", 3, "parent-b")
		conflictResponse := route(t, newStore(first, second), 0, "")
		if conflictResponse.Resolution != "worldline_ownership_unresolved" || conflictResponse.TurnIndex != 0 ||
			conflictResponse.Worldline == nil || conflictResponse.Worldline.State != "conflict" {
			t.Fatalf("conflict=%+v", conflictResponse)
		}
	})

	t.Run("client baseline cannot replay a char branch prefix or rebase its native tail", func(t *testing.T) {
		st := newStore(lineage("char", 8, "parent-session"))
		baseline := `,"baseline":{"backend_turn_at_route":9,"local_pairs_at_route":0,"reason":"timeline_attach"}`
		for _, pair := range []int{1, 2, 8} {
			response := route(t, st, (pair-1)*2, baseline)
			if response.Resolution != "skip_pre_route_visible_pair" || response.LocalTurnIndex != pair || response.BaselineApplied {
				t.Fatalf("pair=%d response=%+v", pair, response)
			}
		}
		for _, pair := range []int{9, 10} {
			response := route(t, st, (pair-1)*2, baseline)
			if response.Resolution != "normal" || response.TurnIndex != pair || response.BaselineApplied {
				t.Fatalf("pair=%d response=%+v", pair, response)
			}
		}
		if st.lineageListCalls != 5 {
			t.Fatalf("worldline ownership was not checked for every pair: lineage_reads=%d", st.lineageListCalls)
		}
	})

	t.Run("batch rebuild plan keeps branch ownership and native turns", func(t *testing.T) {
		st := newStore(lineage("char", 8, "parent-session"))
		for _, baseline := range []string{
			``,
			`,"baseline":{"backend_turn_at_route":9,"local_pairs_at_route":0,"reason":"timeline_attach"}`,
		} {
			server := &Server{Store: st}
			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			rec := httptest.NewRecorder()
			body := fmt.Sprintf(`{
			"chat_session_id":"child-session",
			"mode":"batch",
			"stable_character_id":"stable",
			"stable_character_id_state":"observed",
			"host_chat_id":"child-chat",
			"host_chat_id_state":"observed",
			"observations":[
				{"observation_index":0,"risu_user_message_index":0},
				{"observation_index":1,"risu_user_message_index":14},
				{"observation_index":2,"risu_user_message_index":16},
				{"observation_index":3,"risu_user_message_index":18}
			]%s
		}`, baseline)
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("baseline=%q status=%d body=%s", baseline, rec.Code, rec.Body.String())
			}
			var response sessionRoutingTurnResolutionResponse
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}
			wantTurns := []int{1, 8, 9, 10}
			wantResolutions := []string{"skip_pre_route_visible_pair", "skip_pre_route_visible_pair", "normal", "normal"}
			if len(response.ResolvedObservations) != len(wantTurns) {
				t.Fatalf("baseline=%q response=%+v", baseline, response)
			}
			for index, item := range response.ResolvedObservations {
				if item.TurnIndex != wantTurns[index] || item.Resolution != wantResolutions[index] {
					t.Fatalf("baseline=%q resolved[%d]=%+v", baseline, index, item)
				}
			}
		}
	})

	t.Run("nested branch ignores disabled marker offsets", func(t *testing.T) {
		st := newStore(lineage("char", 9, "parent-session"))
		response := routeObservation(t, st, 20, 10, "", "")
		if response.Resolution != "normal" || response.TurnIndex != 10 || response.LocalTurnIndex != 10 || response.LocalTurnSource != "observed_pair_ordinal" {
			t.Fatalf("response=%+v", response)
		}
	})

	t.Run("nested branch batch ignores disabled marker offsets", func(t *testing.T) {
		st := newStore(lineage("char", 9, "parent-session"))
		server := &Server{Store: st}
		mux := http.NewServeMux()
		server.RegisterRoutes(mux)
		rec := httptest.NewRecorder()
		body := `{
			"chat_session_id":"child-session",
			"mode":"batch",
			"stable_character_id":"stable",
			"stable_character_id_state":"observed",
			"host_chat_id":"child-chat",
			"host_chat_id_state":"observed",
			"observations":[
				{"observation_index":0,"risu_user_message_index":17,"observed_pair_ordinal":9},
				{"observation_index":1,"risu_user_message_index":20,"observed_pair_ordinal":10}
			]
		}`
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response sessionRoutingTurnResolutionResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if len(response.ResolvedObservations) != 2 {
			t.Fatalf("response=%+v", response)
		}
		inherited, childOwned := response.ResolvedObservations[0], response.ResolvedObservations[1]
		if inherited.Resolution != "skip_pre_route_visible_pair" || inherited.TurnIndex != 9 || inherited.Source != "observed_pair_ordinal" {
			t.Fatalf("inherited=%+v", inherited)
		}
		if childOwned.Resolution != "normal" || childOwned.TurnIndex != 10 || childOwned.LocalTurnIndex != 10 || childOwned.Source != "observed_pair_ordinal" {
			t.Fatalf("child-owned=%+v", childOwned)
		}
	})

	t.Run("client baseline preserves user-source child turns", func(t *testing.T) {
		st := newStore(lineage("user", 8, "parent-session"))
		baseline := `,"baseline":{"backend_turn_at_route":9,"local_pairs_at_route":0,"reason":"timeline_attach"}`
		for _, pair := range []int{1, 7} {
			response := route(t, st, (pair-1)*2, baseline)
			if response.Resolution != "skip_pre_route_visible_pair" || response.LocalTurnIndex != pair {
				t.Fatalf("pair=%d response=%+v", pair, response)
			}
		}
		for _, pair := range []int{8, 9} {
			response := route(t, st, (pair-1)*2, baseline)
			if response.Resolution != "normal" || response.TurnIndex != pair || response.BaselineApplied {
				t.Fatalf("pair=%d response=%+v", pair, response)
			}
		}
	})

	t.Run("non-automatic accepted final uses the same confirmed branch numbering", func(t *testing.T) {
		st := newStore(lineage("char", 8, "parent-session"))
		for _, baseline := range []string{
			``,
			`,"baseline":{"backend_turn_at_route":9,"local_pairs_at_route":0,"reason":"timeline_attach"}`,
		} {
			inherited := routeWithContext(t, st, 0, "", baseline)
			if inherited.Resolution != "skip_pre_route_visible_pair" || inherited.TurnIndex != 1 {
				t.Fatalf("baseline=%q inherited=%+v", baseline, inherited)
			}
			response := routeWithContext(t, st, 18, "", baseline)
			if response.Resolution != "normal" || response.TurnIndex != 10 || response.BaselineApplied {
				t.Fatalf("baseline=%q response=%+v", baseline, response)
			}
		}
	})

	t.Run("canonical tail alignment cannot bypass inherited ownership", func(t *testing.T) {
		st := newStore(lineage("char", 8, "parent-session"))
		req := sessionRoutingTurnResolutionRequest{
			ChatSessionID: "child-session", Mode: "pair", RisuUserMessageIndex: intPointer(0),
			Baseline:             &routingTurnBaseline{BackendTurnAtRoute: 9, Reason: "timeline_attach"},
			canonicalTailAligned: true,
		}
		response := calculateSessionRoutingTurnResolution(req)
		if response.BaselineApplied {
			t.Fatalf("fixture did not exercise canonical-tail early return: %+v", response)
		}
		worldline := currentWorldlineViewModel(context.Background(), st, req.ChatSessionID)
		response = applyAutomaticWorldlineBackfillBoundary(req, response, &worldline)
		if response.Resolution != "skip_pre_route_visible_pair" || response.TurnIndex != 1 {
			t.Fatalf("response=%+v", response)
		}
	})

	t.Run("durable migration baseline must agree with confirmed parent ownership", func(t *testing.T) {
		st := newStore(lineage("char", 8, "parent-session"))
		st.baseline = &store.SessionRoutingBaseline{
			MigrationID: 1, SourceSessionID: "parent-session", TargetSessionID: "child-session",
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 8,
		}
		baseline := `,"baseline":{"backend_turn_at_route":999,"local_pairs_at_route":0,"reason":"timeline_copy"}`
		inherited := route(t, st, 0, baseline)
		childOwned := route(t, st, 16, baseline)
		if inherited.Resolution != "skip_pre_route_visible_pair" || !inherited.BaselineApplied || inherited.ProtectedBeforeTurn != 8 {
			t.Fatalf("inherited=%+v", inherited)
		}
		if childOwned.Resolution != "normal" || childOwned.TurnIndex != 9 || !childOwned.BaselineApplied {
			t.Fatalf("child-owned=%+v", childOwned)
		}

		st.baseline.SourceSessionID = "different-parent"
		conflict := route(t, st, 16, baseline)
		if conflict.Resolution != "worldline_ownership_unresolved" || conflict.TurnIndex != 0 {
			t.Fatalf("conflict=%+v", conflict)
		}

		st.baseline.SourceSessionID = "parent-session"
		st.baseline.ImportedThroughTurn = 7
		conflict = route(t, st, 16, baseline)
		if conflict.Resolution != "worldline_ownership_unresolved" || conflict.TurnIndex != 0 {
			t.Fatalf("boundary conflict=%+v", conflict)
		}
	})

	t.Run("ordinary attach without lineage keeps its baseline", func(t *testing.T) {
		st := newStore()
		response := routeWithContext(t, st, 0, "", `,"baseline":{"backend_turn_at_route":9,"local_pairs_at_route":0,"reason":"timeline_attach"}`)
		if response.Resolution != "rebased" || response.TurnIndex != 10 || !response.BaselineApplied {
			t.Fatalf("response=%+v", response)
		}
	})
}

func (s *durableSessionIdentityBindingStore) SaveForkLineageRecord(_ context.Context, record store.ForkLineageRecord) (store.ForkLineageRecord, error) {
	for index := range s.lineage {
		if record.IdempotencyKey == "" ||
			s.lineage[index].ChatSessionID != record.ChatSessionID ||
			s.lineage[index].IdempotencyKey != record.IdempotencyKey {
			continue
		}
		if (s.lineage[index].LineageState == "confirmed" && s.lineage[index].ContractVersion == record.ContractVersion) ||
			s.lineage[index].ImportedAt.After(record.ImportedAt) {
			if s.lineage[index].LineageState == "confirmed" && record.LineageState == "confirmed" &&
				(s.lineage[index].InheritedItemsJSON == "" || s.lineage[index].InheritedItemsJSON == "[]") {
				s.lineage[index].InheritedItemsJSON = record.InheritedItemsJSON
			}
			return s.lineage[index], nil
		}
		record.ID = s.lineage[index].ID
		s.lineage[index] = record
		return record, nil
	}
	record.ID = int64(len(s.lineage) + 1)
	s.lineage = append(s.lineage, record)
	return record, nil
}

func (s *sessionIdentityRoutingStore) ListSessions(context.Context) ([]store.SessionSummary, error) {
	return s.sessions, nil
}

func (s *sessionIdentityRoutingStore) ListChatLogs(_ context.Context, sid string, _, _ int) ([]store.ChatLog, error) {
	return s.logs[sid], nil
}

func (s *sessionIdentityRoutingStore) GetSessionRoutingBaseline(context.Context, string) (*store.SessionRoutingBaseline, error) {
	return s.baseline, nil
}

func (s *rollbackDecisionChatLogStore) LatestSessionTurnIndex(context.Context, string) (int, error) {
	s.latestTurnCalls++
	latest := 0
	for _, item := range s.logs {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	return latest, nil
}

func (s *rollbackDecisionChatLogStore) ListChatLogs(_ context.Context, _ string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	s.listChatLogsFrom = fromTurn
	s.listChatLogsTo = toTurn
	result := make([]store.ChatLog, 0, len(s.logs))
	for _, item := range s.logs {
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *rollbackDecisionChatLogStore) GetSessionRoutingBaseline(context.Context, string) (*store.SessionRoutingBaseline, error) {
	return s.baseline, nil
}

func (s *rollbackDecisionChatLogStore) ListActiveSourceRevisions(
	_ context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
) ([]store.MemorySourceRevision, error) {
	result := make([]store.MemorySourceRevision, 0, len(s.activeSources))
	for _, item := range s.activeSources {
		if item.ChatSessionID != chatSessionID || item.LifecycleState != "active" {
			continue
		}
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *durableRoutingBaselineStore) GetSessionRoutingBaseline(context.Context, string) (*store.SessionRoutingBaseline, error) {
	return s.baseline, nil
}

func (s *durableRoutingBaselineStore) ListActiveSourceRevisions(
	_ context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
) ([]store.MemorySourceRevision, error) {
	result := make([]store.MemorySourceRevision, 0, len(s.activeSources))
	for _, item := range s.activeSources {
		if item.ChatSessionID != chatSessionID || item.LifecycleState != "active" ||
			(fromTurn > 0 && item.TurnIndex < fromTurn) || (toTurn > 0 && item.TurnIndex > toTurn) {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func activeSourceRevisionForUserAnchor(parentSessionID, parentHostChatID, userMessageID, assistantMessageID string, turn int) store.MemorySourceRevision {
	return store.MemorySourceRevision{
		LogicalTurnID: completeTurnLogicalTurnID(parentSessionID, completeTurnSourceObservation{
			HostChatID:             parentHostChatID,
			HostChatIDState:        "observed_before_request",
			UserMessageChatID:      userMessageID,
			UserMessageChatIDState: "observed_before_request",
		}),
		SourceMessageID: assistantMessageID,
		TurnIndex:       turn,
		LifecycleState:  "active",
	}
}

func TestResolveRisuWorldlineObservationConfirmsExactParentActiveSource(t *testing.T) {
	st := &durableSessionIdentityBindingStore{
		Store: store.NewNoopStore(),
		bindings: map[string]string{
			"character-stable\x00parent-chat": "parent-session",
		},
		sources: map[string][]store.MemorySourceRevision{
			"parent-session": {activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "user-anchor", "source-message", 7)},
		},
	}
	server := &Server{Store: st}
	req := sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "active_chat_pre_backfill",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_000,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::::source-message::}}",
			MarkerIndex:         4,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 2, Role: "user", MessageChatID: "user-anchor"},
				{MessageIndex: 3, Role: "char", MessageChatID: "source-message"},
				{MessageIndex: 4, Role: "comment", Disabled: true},
			},
		},
	}
	vm := server.resolveRisuWorldlineObservation(context.Background(), req, "child-session")
	if vm.State != "confirmed" || vm.ParentSessionID != "parent-session" || vm.ForkTurn != 7 ||
		vm.ForkSourceMessageID != "source-message" || vm.ForkSourceRole != "char" || vm.InheritedThroughTurn != 7 {
		t.Fatalf("worldline=%+v", vm)
	}
	if st.lastMode != store.SessionRouteBindingModeResolveExisting || len(st.lineage) != 1 ||
		st.lineage[0].ContractVersion != store.RisuWorldlineForkLineageContractVersion ||
		st.lineage[0].ForkSourceRole != "char" || st.lineage[0].InheritanceMode != "none" {
		t.Fatalf("route/store state mode=%q lineage=%+v", st.lastMode, st.lineage)
	}
}

func TestCompleteTurnSourceWriterFeedsExactWorldlineParentSource(t *testing.T) {
	const (
		opaqueUserMessageID   = "opaque-user-coordinate"
		opaqueSourceMessageID = "opaque-source-coordinate"
	)
	logicalTurnID := completeTurnLogicalTurnID("parent-session", completeTurnSourceObservation{
		HostChatID:             "parent-chat",
		HostChatIDState:        "observed_before_request",
		UserMessageChatID:      opaqueUserMessageID,
		UserMessageChatIDState: "observed_before_request",
	})
	source, err := completeTurnMemorySourceRevision(completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: "source-revision", LogicalTurnID: logicalTurnID,
		Observation: completeTurnSourceObservation{
			ObservedAtMS:           1_776_000_000_000,
			HostChatID:             "parent-chat",
			HostChatIDState:        "observed_before_request",
			UserMessageChatID:      opaqueUserMessageID,
			UserMessageChatIDState: "observed_before_request",
			MessageIndex:           -1,
			MessageChatIDState:     "not_exposed_by_risu_afterRequest",
			HashAlgorithm:          "djb2.v1",
		},
	}, "parent-session", 7, "neutral user", "neutral assistant", time.UnixMilli(1_776_000_000_000).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if source.SourceMessageID != "" || source.LogicalTurnID != logicalTurnID {
		t.Fatalf("normal v3 writer source=%+v", source)
	}
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources:  map[string][]store.MemorySourceRevision{"parent-session": {*source}},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_100,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::" + opaqueSourceMessageID + "::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 0, Role: "user", MessageChatID: opaqueUserMessageID},
				{MessageIndex: 1, Role: "char", MessageChatID: opaqueSourceMessageID},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		},
	}, "child-session")
	if vm.State != "confirmed" || vm.ParentSessionID != "parent-session" || vm.ForkTurn != 7 ||
		vm.ForkSourceMessageID != opaqueSourceMessageID || vm.ForkSourceRole != "char" || vm.InheritedThroughTurn != 7 {
		t.Fatalf("writer-to-resolver worldline=%+v", vm)
	}
}

func TestCompleteTurnSourceWriterFeedsExactUserAnchorWorldlineParentSource(t *testing.T) {
	const opaqueUserMessageID = "opaque-user-coordinate"
	logicalTurnID := completeTurnLogicalTurnID("parent-session", completeTurnSourceObservation{
		HostChatID:             "parent-chat",
		HostChatIDState:        "observed_before_request",
		UserMessageChatID:      opaqueUserMessageID,
		UserMessageChatIDState: "observed_before_request",
	})
	source, err := completeTurnMemorySourceRevision(completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: "source-revision", LogicalTurnID: logicalTurnID,
		Observation: completeTurnSourceObservation{
			ObservedAtMS:           1_776_000_000_000,
			HostChatID:             "parent-chat",
			HostChatIDState:        "observed_before_request",
			UserMessageChatID:      opaqueUserMessageID,
			UserMessageChatIDState: "observed_before_request",
			MessageIndex:           -1,
			MessageChatIDState:     "not_exposed_by_risu_afterRequest",
			HashAlgorithm:          "djb2.v1",
		},
	}, "parent-session", 7, "neutral user", "neutral assistant", time.UnixMilli(1_776_000_000_000).UTC())
	if err != nil {
		t.Fatal(err)
	}
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources:  map[string][]store.MemorySourceRevision{"parent-session": {*source}},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_100,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::" + opaqueUserMessageID + "::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 1, Role: "user", MessageChatID: opaqueUserMessageID},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		},
	}, "child-session")
	if vm.State != "confirmed" || vm.ParentSessionID != "parent-session" || vm.ForkTurn != 7 ||
		vm.ForkSourceMessageID != opaqueUserMessageID || vm.ForkSourceRole != "user" || vm.InheritedThroughTurn != 6 {
		t.Fatalf("writer-to-user-anchor resolver worldline=%+v source=%+v", vm, source)
	}
}

func TestResolveRisuWorldlineObservationRejectsWrongOrDuplicateUserAnchor(t *testing.T) {
	newObservation := func(userAnchor string) *risuWorldlineObservation {
		return &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_100,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::assistant-source::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 0, Role: "user", MessageChatID: userAnchor},
				{MessageIndex: 1, Role: "char", MessageChatID: "assistant-source"},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		}
	}
	t.Run("wrong anchor remains unresolved", func(t *testing.T) {
		st := &durableSessionIdentityBindingStore{
			Store:    store.NewNoopStore(),
			bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
			sources: map[string][]store.MemorySourceRevision{
				"parent-session": {activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "expected-anchor", "", 7)},
			},
		}
		vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
			StableCharacterID: "character-stable", HostChatID: "child-chat", HostChatIDState: "observed", WorldlineObservation: newObservation("wrong-anchor"),
		}, "child-session")
		if vm.State != "unresolved" || vm.Reason != "parent_fork_source_history_unresolved" ||
			len(st.lineage) != 1 || st.lineage[0].CopiedFromSessionID != "" {
			t.Fatalf("wrong anchor worldline=%+v lineage=%+v", vm, st.lineage)
		}
	})
	t.Run("duplicate active anchor conflicts", func(t *testing.T) {
		first := activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "user-anchor", "", 7)
		second := first
		second.TurnIndex = 8
		st := &durableSessionIdentityBindingStore{
			Store:    store.NewNoopStore(),
			bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
			sources:  map[string][]store.MemorySourceRevision{"parent-session": {first, second}},
		}
		vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
			StableCharacterID: "character-stable", HostChatID: "child-chat", HostChatIDState: "observed", WorldlineObservation: newObservation("user-anchor"),
		}, "child-session")
		if vm.State != "conflict" || vm.Reason != "parent_active_fork_source_conflict" ||
			len(st.lineage) != 1 || st.lineage[0].CopiedFromSessionID != "" {
			t.Fatalf("duplicate anchor worldline=%+v lineage=%+v", vm, st.lineage)
		}
	})
}

func TestResolveRisuWorldlineObservationStarterWithoutActiveSourceIsUnresolved(t *testing.T) {
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources:  map[string][]store.MemorySourceRevision{},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_100,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::starter-source::}}",
			MarkerIndex:         1,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 0, Role: "char", MessageChatID: "starter-source"},
				{MessageIndex: 1, Role: "comment", Disabled: true},
			},
		},
	}, "child-session")
	if vm.State != "unresolved" || vm.Reason != "fork_user_anchor_unresolved" || vm.ParentSessionID != "" || vm.ForkTurn != 0 ||
		len(st.lineage) != 1 || st.lineage[0].CopiedFromSessionID != "" {
		t.Fatalf("starter worldline=%+v lineage=%+v", vm, st.lineage)
	}
}

func TestResolveRisuWorldlineObservationRecoversUniqueHistoricalNormalizedSource(t *testing.T) {
	historical := activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "old-user", "source-message", 7)
	historical.LogicalTurnID = "canonical_turn_normalized"
	historical.LifecycleState = "superseded"
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources:  map[string][]store.MemorySourceRevision{"parent-session": {}},
		history:  map[string][]store.MemorySourceRevision{"parent-session": {historical}},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable", HostChatID: "child-chat", HostChatIDState: "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion: risuWorldlineObservationContract, HostSignalSource: "output",
			BranchShapeContract: risuBranchShapeContract, ObservedAtMS: 1_776_000_000_100,
			MarkerState: "observed", BranchMarker: "{{specialcomment::branchedfrom::parent-chat::Parent::source-message::}}", MarkerIndex: 2,
			Messages: []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "missing-user"}, {MessageIndex: 1, Role: "char", MessageChatID: "source-message"}, {MessageIndex: 2, Role: "comment", Disabled: true}},
		},
	}, "child-session")
	if vm.State != "confirmed" || vm.ParentSessionID != "parent-session" || vm.ForkTurn != 7 || vm.Reason != "official_branch_marker_historical_source_validated" {
		t.Fatalf("historical recovery=%+v", vm)
	}
	if historical.LifecycleState != "superseded" || len(st.lineage) != 1 || st.lineage[0].ForkTurn != 7 {
		t.Fatalf("historical revision mutated or lineage missing: source=%+v lineage=%+v", historical, st.lineage)
	}
}

func TestPrioritizedWorldlineSourceTurnsPrefersExactAssistantSourceID(t *testing.T) {
	sources := []store.MemorySourceRevision{
		{TurnIndex: 7, SourceMessageID: "assistant-source", LogicalTurnID: "normalized"},
		{TurnIndex: 9, SourceMessageID: "other-source", LogicalTurnID: "expected-user-logical-turn"},
	}
	turns := prioritizedWorldlineSourceTurns(sources, "expected-user-logical-turn", "char", "assistant-source")
	if fmt.Sprint(turns) != "[7]" {
		t.Fatalf("exact assistant source ID did not take precedence: %v", turns)
	}
}

func TestResolveRisuWorldlineObservationReportsAmbiguousHistoricalTurnsWithoutMutatingChild(t *testing.T) {
	first := activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "old-user", "source-message", 7)
	first.LogicalTurnID = "canonical_turn_a"
	first.LifecycleState = "superseded"
	second := first
	second.TurnIndex = 9
	second.SourceRevision = "other-revision"
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources:  map[string][]store.MemorySourceRevision{"parent-session": {}},
		history:  map[string][]store.MemorySourceRevision{"parent-session": {first, second}},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable", HostChatID: "child-chat", HostChatIDState: "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion: risuWorldlineObservationContract, HostSignalSource: "output",
			BranchShapeContract: risuBranchShapeContract, ObservedAtMS: 1_776_000_000_100,
			MarkerState: "observed", BranchMarker: "{{specialcomment::branchedfrom::parent-chat::Parent::source-message::}}", MarkerIndex: 2,
			Messages: []risuWorldlineMessageObservation{{MessageIndex: 0, Role: "user", MessageChatID: "missing-user"}, {MessageIndex: 1, Role: "char", MessageChatID: "source-message"}, {MessageIndex: 2, Role: "comment", Disabled: true}},
		},
	}, "child-session")
	if vm.State != "conflict" || vm.Reason != "parent_fork_source_history_ambiguous" || vm.CandidateParentID != "parent-session" || fmt.Sprint(vm.CandidateForkTurns) != "[7 9]" {
		t.Fatalf("ambiguous recovery=%+v", vm)
	}
	if len(st.lineage) != 1 || st.lineage[0].CopiedFromSessionID != "" {
		t.Fatalf("ambiguous recovery changed child lineage=%+v", st.lineage)
	}
}

func TestParseExactRisuBranchMarkerAllowsOpaqueParentName(t *testing.T) {
	parent, name, source, ok := parseExactRisuBranchMarker(
		"{{specialcomment::branchedfrom::parent-chat::Act 1::Quiet Room::source-message::}}",
	)
	if !ok || parent != "parent-chat" || name != "Act 1::Quiet Room" || source != "source-message" {
		t.Fatalf("parsed parent=%q name=%q source=%q ok=%v", parent, name, source, ok)
	}
}

func TestResolveRisuWorldlineObservationKeepsConfirmedAfterMarkerOrParentSourceDisappears(t *testing.T) {
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources: map[string][]store.MemorySourceRevision{
			"parent-session": {activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "user-anchor", "source-message", 7)},
		},
	}
	server := &Server{Store: st}
	observed := &risuWorldlineObservation{
		ContractVersion: risuWorldlineObservationContract, HostSignalSource: "output",
		BranchShapeContract: risuBranchShapeContract, ObservedAtMS: 1_776_000_000_000,
		MarkerState:  "observed",
		BranchMarker: "{{specialcomment::branchedfrom::parent-chat::Parent::source-message::}}",
		MarkerIndex:  2,
		Messages: []risuWorldlineMessageObservation{
			{MessageIndex: 0, Role: "user", MessageChatID: "user-anchor"},
			{MessageIndex: 1, Role: "char", MessageChatID: "source-message"},
			{MessageIndex: 2, Role: "comment", Disabled: true},
		},
	}
	first := server.resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable", HostChatID: "child-chat", HostChatIDState: "observed", WorldlineObservation: observed,
	}, "child-session")
	if first.State != "confirmed" {
		t.Fatalf("first=%+v", first)
	}
	st.sources["parent-session"] = nil
	absent := server.resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		HostChatID: "child-chat", WorldlineObservation: &risuWorldlineObservation{MarkerState: "absent"},
	}, "child-session")
	if absent.State != "confirmed" || absent.ForkTurn != 7 || len(st.lineage) != 1 {
		t.Fatalf("confirmed lineage lost after marker/source removal: vm=%+v lineage=%+v", absent, st.lineage)
	}
}

func TestResolveRisuWorldlineObservationMissingParentPersistsUnresolvedWithoutCreatingRoute(t *testing.T) {
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{}}
	server := &Server{Store: st}
	req := sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_000,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::missing-parent::Parent::source-message::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 0, Role: "user", MessageChatID: "user-anchor"},
				{MessageIndex: 1, Role: "char", MessageChatID: "source-message"},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		},
	}
	vm := server.resolveRisuWorldlineObservation(context.Background(), req, "child-session")
	if vm.State != "unresolved" || vm.Reason != "parent_route_unresolved" {
		t.Fatalf("worldline=%+v", vm)
	}
	if len(st.bindings) != 0 || st.lastMode != store.SessionRouteBindingModeResolveExisting {
		t.Fatalf("missing parent route mutated: mode=%q bindings=%v", st.lastMode, st.bindings)
	}
	if len(st.lineage) != 1 || st.lineage[0].LineageState != "unresolved" || st.lineage[0].CopiedFromSessionID != "" {
		t.Fatalf("unresolved lineage=%+v", st.lineage)
	}
}

func TestResolveRisuWorldlineObservationRequiresObservedChildHostChat(t *testing.T) {
	st := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"character-stable\x00parent-chat": "parent-session"},
		sources: map[string][]store.MemorySourceRevision{
			"parent-session": {activeSourceRevisionForUserAnchor("parent-session", "parent-chat", "user-anchor", "source-message", 7)},
		},
	}
	vm := (&Server{Store: st}).resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatIDState:   "unobserved",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "active_chat_pre_backfill",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_000,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::source-message::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 0, Role: "user", MessageChatID: "user-anchor"},
				{MessageIndex: 1, Role: "char", MessageChatID: "source-message"},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		},
	}, "child-session")
	if vm.State != "unresolved" || vm.Reason != "child_host_chat_unresolved" ||
		st.lastMode != "" || len(st.lineage) != 1 || st.lineage[0].CopiedFromSessionID != "" {
		t.Fatalf("missing child host worldline=%+v mode=%q lineage=%+v", vm, st.lastMode, st.lineage)
	}
}

func TestResolveRisuWorldlineObservationNonTurnSourceIsUnresolvedNotConflict(t *testing.T) {
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{}}
	server := &Server{Store: st}
	vm := server.resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "child-chat",
		HostChatIDState:   "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion:     risuWorldlineObservationContract,
			HostSignalSource:    "output",
			BranchShapeContract: risuBranchShapeContract,
			ObservedAtMS:        1_776_000_000_000,
			MarkerState:         "observed",
			BranchMarker:        "{{specialcomment::branchedfrom::parent-chat::Parent::source-message::}}",
			MarkerIndex:         2,
			Messages: []risuWorldlineMessageObservation{
				{MessageIndex: 1, Role: "comment", MessageChatID: "source-message"},
				{MessageIndex: 2, Role: "comment", Disabled: true},
			},
		},
	}, "child-session")
	if vm.State != "unresolved" || vm.Reason != "fork_source_message_unresolved" {
		t.Fatalf("worldline=%+v", vm)
	}
	if st.lastMode != "" || len(st.lineage) != 1 || st.lineage[0].LineageState != "unresolved" {
		t.Fatalf("non-turn source unexpectedly resolved parent route: mode=%q lineage=%+v", st.lastMode, st.lineage)
	}
}

func TestResolveRisuWorldlineObservationMalformedMarkerPersistsConflict(t *testing.T) {
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{}}
	server := &Server{Store: st}
	vm := server.resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		HostChatID:      "child-chat",
		HostChatIDState: "observed",
		WorldlineObservation: &risuWorldlineObservation{
			ContractVersion: risuWorldlineObservationContract, HostSignalSource: "output",
			BranchShapeContract: risuBranchShapeContract, ObservedAtMS: 1_776_000_000_000,
			MarkerState: "observed", BranchMarker: "{{specialcomment::branchedfrom::broken}}",
		},
	}, "child-session")
	if vm.State != "conflict" || vm.Reason != "official_branch_marker_malformed" {
		t.Fatalf("worldline=%+v", vm)
	}
	if len(st.lineage) != 1 || strings.Contains(st.lineage[0].DivergenceMarker, "specialcomment") {
		t.Fatalf("conflict lineage leaked raw marker: %+v", st.lineage)
	}
}

func TestResolveRisuWorldlineObservationAbsentIsNotApplicableAndDoesNotPersist(t *testing.T) {
	st := &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), bindings: map[string]string{}}
	server := &Server{Store: st}
	vm := server.resolveRisuWorldlineObservation(context.Background(), sessionRoutingTurnResolutionRequest{
		HostChatID:           "same-chat",
		WorldlineObservation: &risuWorldlineObservation{MarkerState: "absent"},
	}, "same-session")
	if vm.State != "not_applicable" || len(st.lineage) != 0 {
		t.Fatalf("worldline=%+v lineage=%+v", vm, st.lineage)
	}
}

func TestRollbackDecisionProtectsCopiedSessionBaseline(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "assistant_deleted_output_removed",
		CandidateFromTurn: 1, PreviousTurnIndex: 8, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 1, BackendLatestTurn: 8, DeletionObserved: true,
		Baseline: &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"},
	})
	if !resp.Allowed || resp.FromTurn != 8 || resp.ProtectedBeforeTurn != 7 || resp.MinFromTurn != 8 {
		t.Fatalf("decision=%+v", resp)
	}
}

func TestSessionRoutingHandlerUsesDurableCopiedBaselineWhenClientBaselineIsMissing(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	server := &Server{Store: &durableRoutingBaselineStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		baseline: &store.SessionRoutingBaseline{
			MigrationID: 42, SourceSessionID: "source", TargetSessionID: sid,
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 8,
		},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: sid, TurnIndex: 9, SourceMessageID: "assistant-9",
			AssistantContent: "deleted assistant", LifecycleState: "active",
		}},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"mode":"pair",
		"risu_user_message_index":0,
		"observed_pair_ordinal":1,
		"baseline":{"backend_turn_at_route":1,"local_pairs_at_route":0,"reason":"timeline_copy"}
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.BaselineApplied || response.TurnIndex != 9 || response.ProtectedBeforeTurn != 8 || response.MinFromTurn != 9 {
		t.Fatalf("durable copied baseline was not applied: %+v", response)
	}
}

func TestSessionRoutingIdentityKeepsExistingCIDWhenCharacterIndexChanges(t *testing.T) {
	const (
		hostChatID = "76eff0e5-8439-446f-a164-1271a7cc2089"
		existingID = "char_1_cid_" + hostChatID
		requested  = "char_0_cid_" + hostChatID
	)
	server := &Server{Store: &sessionIdentityRoutingStore{
		Store:    store.NewNoopStore(),
		sessions: []store.SessionSummary{{ChatSessionID: existingID, ChatLogsCount: 68}},
		logs:     map[string][]store.ChatLog{},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+requested+`",
		"mode":"identity",
		"host_chat_id":"`+hostChatID+`",
		"host_chat_id_state":"observed"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChatSessionID != existingID || response.IdentityResolution != "existing_host_chat_id" {
		t.Fatalf("same observed CID was split by character index: %+v", response)
	}
}

func TestSessionRoutingDurableBindingSurvivesIndexMoveAndReload(t *testing.T) {
	bindingStore := &durableSessionIdentityBindingStore{
		Store: store.NewNoopStore(),
	}
	server := &Server{Store: bindingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	post := func(requested string) sessionRoutingTurnResolutionResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
			"chat_session_id":"`+requested+`",
			"mode":"identity",
			"stable_character_id":"stable-character",
			"stable_character_id_state":"observed",
			"host_chat_id":"opaque-chat",
			"host_chat_id_state":"observed"
		}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var response sessionRoutingTurnResolutionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	created := post("char_4_cid_opaque-chat")
	if created.ChatSessionID != "char_4_cid_opaque-chat" || !created.BindingAcknowledged ||
		!created.BindingCreated || created.IdentityResolution != "durable_binding_created" {
		t.Fatalf("created binding response = %+v", created)
	}
	reloadedAfterIndexMove := post("char_1_cid_opaque-chat")
	if reloadedAfterIndexMove.ChatSessionID != "char_4_cid_opaque-chat" ||
		!reloadedAfterIndexMove.BindingAcknowledged ||
		reloadedAfterIndexMove.IdentityResolution != "durable_binding_existing" {
		t.Fatalf("index move/reload split durable binding: %+v", reloadedAfterIndexMove)
	}
}

func TestSessionRoutingDurableBindingKeepsDifferentStableCharactersSeparate(t *testing.T) {
	bindingStore := &durableSessionIdentityBindingStore{Store: store.NewNoopStore()}
	server := &Server{Store: bindingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	for _, fixture := range []struct {
		character string
		session   string
	}{
		{character: "stable-a", session: "session-a"},
		{character: "stable-b", session: "session-b"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
			"chat_session_id":"`+fixture.session+`",
			"mode":"identity",
			"stable_character_id":"`+fixture.character+`",
			"stable_character_id_state":"observed",
			"host_chat_id":"same-opaque-chat",
			"host_chat_id_state":"observed"
		}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var response sessionRoutingTurnResolutionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.ChatSessionID != fixture.session || !response.BindingAcknowledged {
			t.Fatalf("%s response = %+v", fixture.character, response)
		}
	}
}

func TestSessionRoutingDurableBindingRedirectsLockedSource(t *testing.T) {
	bindingStore := &durableSessionIdentityBindingStore{
		Store: store.NewNoopStore(),
		bindings: map[string]string{
			"stable-character\x00opaque-chat": "locked-source",
		},
		locks: map[string]string{"locked-source": "migration-target"},
	}
	server := &Server{Store: bindingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"locked-source",
		"mode":"identity",
		"stable_character_id":"stable-character",
		"stable_character_id_state":"observed",
		"host_chat_id":"opaque-chat",
		"host_chat_id_state":"observed"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ChatSessionID != "migration-target" || !response.LockedSourceRedirect ||
		!response.BindingAcknowledged || response.IdentityResolution != "durable_binding_locked_source_redirect" {
		t.Fatalf("locked source was not redirected: %+v", response)
	}
}

func TestSessionRoutingBindingFailureReturnsNoAcknowledgement(t *testing.T) {
	server := &Server{Store: &durableSessionIdentityBindingStore{Store: store.NewNoopStore(), fail: true}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"requested",
		"mode":"identity",
		"stable_character_id":"stable-character",
		"stable_character_id_state":"observed",
		"host_chat_id":"opaque-chat",
		"host_chat_id_state":"observed"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "error" || response.Code != "session_route_binding_failed" ||
		response.BindingAcknowledged || response.ChatSessionID != "requested" {
		t.Fatalf("binding failure was presented as success: %+v", response)
	}
}

func TestSessionRoutingManualAttachUsesExplicitBindingMode(t *testing.T) {
	bindingStore := &durableSessionIdentityBindingStore{
		Store:    store.NewNoopStore(),
		bindings: map[string]string{"stable-character\x00opaque-chat": "old-session"},
	}
	server := &Server{Store: bindingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"attached-session",
		"mode":"identity",
		"stable_character_id":"stable-character",
		"stable_character_id_state":"observed",
		"host_chat_id":"opaque-chat",
		"host_chat_id_state":"observed",
		"bind_requested_session":true,
		"binding_mode":"manual_attach"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ChatSessionID != "attached-session" || !response.BindingAcknowledged ||
		!response.BindingUpdated || bindingStore.lastMode != store.SessionRouteBindingModeManualAttach {
		t.Fatalf("manual attach binding response = %+v mode=%q", response, bindingStore.lastMode)
	}
}

func TestSessionRoutingLegacyIndexPinPromotesToDurableBinding(t *testing.T) {
	bindingStore := &durableSessionIdentityBindingStore{Store: store.NewNoopStore()}
	server := &Server{Store: bindingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"legacy-pinned-session",
		"mode":"identity",
		"stable_character_id":"stable-character",
		"stable_character_id_state":"observed",
		"host_chat_id":"opaque-chat",
		"host_chat_id_state":"observed",
		"binding_mode":"legacy_promotion"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ChatSessionID != "legacy-pinned-session" || !response.BindingAcknowledged ||
		!response.BindingCreated || bindingStore.lastMode != store.SessionRouteBindingModeLegacyPromotion {
		t.Fatalf("legacy promotion response = %+v mode=%q", response, bindingStore.lastMode)
	}
}

func TestSessionRoutingIdentityUsesObservedTailAcrossExistingIndexAliases(t *testing.T) {
	const hostChatID = "same-host-chat"
	oldID, currentID := "char_0_cid_"+hostChatID, "char_1_cid_"+hostChatID
	server := &Server{Store: &sessionIdentityRoutingStore{
		Store: store.NewNoopStore(),
		sessions: []store.SessionSummary{
			{ChatSessionID: oldID, ChatLogsCount: 8},
			{ChatSessionID: currentID, ChatLogsCount: 68},
		},
		logs: map[string][]store.ChatLog{
			oldID: {
				{ChatSessionID: oldID, TurnIndex: 4, Role: "user", Content: "old user"},
				{ChatSessionID: oldID, TurnIndex: 4, Role: "assistant", Content: "old assistant"},
			},
			currentID: {
				{ChatSessionID: currentID, TurnIndex: 51, Role: "user", Content: "current user"},
				{ChatSessionID: currentID, TurnIndex: 51, Role: "assistant", Content: "current assistant"},
			},
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+oldID+`",
		"mode":"identity",
		"host_chat_id":"`+hostChatID+`",
		"host_chat_id_state":"observed",
		"latest_user_hash":"`+prepareOR1CHash("current user")+`",
		"latest_assistant_hash":"`+prepareOR1CHash("current assistant")+`"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChatSessionID != currentID || response.IdentityResolution != "existing_host_chat_tail_match" {
		t.Fatalf("observed active tail did not select the matching session: %+v", response)
	}
}

func TestSessionRoutingIdentityDoesNotReuseMismatchedCIDTail(t *testing.T) {
	const hostChatID = "same-host-chat"
	existingID, requestedID := "char_1_cid_"+hostChatID, "char_0_cid_"+hostChatID
	server := &Server{Store: &sessionIdentityRoutingStore{
		Store:    store.NewNoopStore(),
		sessions: []store.SessionSummary{{ChatSessionID: existingID, ChatLogsCount: 68}},
		logs: map[string][]store.ChatLog{
			existingID: {
				{ChatSessionID: existingID, TurnIndex: 51, Role: "user", Content: "wrong old user"},
				{ChatSessionID: existingID, TurnIndex: 51, Role: "assistant", Content: "wrong old assistant"},
			},
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+requestedID+`",
		"mode":"identity",
		"host_chat_id":"`+hostChatID+`",
		"host_chat_id_state":"observed",
		"latest_user_hash":"`+prepareOR1CHash("actual turn 35 user")+`",
		"latest_assistant_hash":"`+prepareOR1CHash("actual turn 35 assistant")+`"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChatSessionID != requestedID || response.IdentityResolution != "existing_host_chat_tail_mismatch" {
		t.Fatalf("mismatched backend tail was reused: %+v", response)
	}
}

func TestSessionRoutingIdentityRejectsMatchingContentAtWrongTurnNumber(t *testing.T) {
	const hostChatID = "same-host-chat"
	existingID, requestedID := "char_1_cid_"+hostChatID, "char_0_cid_"+hostChatID
	server := &Server{Store: &sessionIdentityRoutingStore{
		Store:    store.NewNoopStore(),
		sessions: []store.SessionSummary{{ChatSessionID: existingID, ChatLogsCount: 68}},
		logs: map[string][]store.ChatLog{
			existingID: {
				{ChatSessionID: existingID, TurnIndex: 51, Role: "user", Content: "actual turn 35 user"},
				{ChatSessionID: existingID, TurnIndex: 51, Role: "assistant", Content: "actual turn 35 assistant"},
			},
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+requestedID+`",
		"mode":"identity",
		"host_chat_id":"`+hostChatID+`",
		"host_chat_id_state":"observed",
		"visible_completed_turns":35,
		"latest_user_hash":"`+prepareOR1CHash("actual turn 35 user")+`",
		"latest_assistant_hash":"`+prepareOR1CHash("actual turn 35 assistant")+`"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ChatSessionID != requestedID || response.IdentityResolution != "existing_host_chat_turn_mismatch" {
		t.Fatalf("matching content at backend turn 51 was mistaken for RisuAI turn 35: %+v", response)
	}
}

func TestSessionRoutingMatchingCanonicalTailDisablesLostCopyOffset(t *testing.T) {
	const (
		hostChatID = "copy-target"
		sid        = "char_1_cid_" + hostChatID
	)
	server := &Server{Store: &sessionIdentityRoutingStore{
		Store:    store.NewNoopStore(),
		sessions: []store.SessionSummary{{ChatSessionID: sid, ChatLogsCount: 38}},
		logs: map[string][]store.ChatLog{
			sid: {
				{ChatSessionID: sid, TurnIndex: 19, Role: "user", Content: "turn 19 user"},
				{ChatSessionID: sid, TurnIndex: 19, Role: "assistant", Content: "turn 19 assistant"},
			},
		},
		baseline: &store.SessionRoutingBaseline{
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 17,
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"mode":"pair",
		"host_chat_id":"`+hostChatID+`",
		"host_chat_id_state":"observed",
		"visible_completed_turns":19,
		"risu_user_message_index":38,
		"observed_pair_ordinal":20,
		"latest_user_hash":"`+prepareOR1CHash("turn 19 user")+`",
		"latest_assistant_hash":"`+prepareOR1CHash("turn 19 assistant")+`"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.TurnIndex != 20 || response.Resolution != "canonical_tail_aligned" || response.BaselineApplied {
		t.Fatalf("lost local copy baseline added 17 turns again: %+v", response)
	}
}

func TestRollbackDecisionHandlerUsesDurableCopiedBaselineWhenClientBaselineIsMissing(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	server := &Server{Store: &durableRoutingBaselineStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		baseline: &store.SessionRoutingBaseline{
			MigrationID: 42, SourceSessionID: "source", TargetSessionID: sid,
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 8,
		},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: sid, TurnIndex: 9, SourceMessageID: "assistant-9",
			AssistantContent: "deleted assistant", LifecycleState: "active",
		}},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"first_removed_turn":1,
		"visible_completed_turns":0,
		"backend_latest_turn":9,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || !response.BaselineApplied || response.FromTurn != 9 || response.ProtectedBeforeTurn != 8 || response.MinFromTurn != 9 {
		t.Fatalf("durable copied rollback baseline was not applied: %+v", response)
	}
	hud, ok := response.TurnWorkflowHUD.(map[string]any)
	if !ok || hud["display_mode"] != "notice" || hud["status"] != "running" ||
		hud["notice_code"] != "ASSISTANT_OUTPUT_DELETE_DETECTED" ||
		hud["title_key"] != "turn_hud.notice.delete_detected" {
		t.Fatalf("verified deletion did not return a detection HUD: %#v", response.TurnWorkflowHUD)
	}
}

func TestVerifiedTailDeleteUsesBackendTailWhenCopiedBaselineIsMissing(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "active_chat_tail_missing_from_runtime",
		CandidateFromTurn: 1, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9,
		DeletionObserved: true, LedgerVerified: true,
	})
	if !resp.Allowed || resp.FromTurn != 9 {
		t.Fatalf("missing-baseline verified tail decision=%+v", resp)
	}
}

func TestVerifiedTailDeleteRejectsImpossibleRemovedCount(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		CandidateFromTurn: 1, RemovedAssistantCount: 10,
		BackendLatestTurn: 9, DeletionObserved: true, LedgerVerified: true,
	})
	if resp.Allowed || resp.Reason != "ledger_removed_count_exceeds_backend_tail" {
		t.Fatalf("impossible removed count decision=%+v", resp)
	}
}

func TestVerifiedTailDeleteWithoutClientBaselineExecutesOnlyBackendTail(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recordingStore := &rollbackDecisionExecutionStore{
		rollbackRecordingStore:      &rollbackRecordingStore{Store: store.NewNoopStore()},
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs:                        []store.ChatLog{{ChatSessionID: sid, TurnIndex: 9, Role: "assistant", Content: "deleted assistant"}},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: sid, TurnIndex: 9, SourceMessageID: "assistant-9",
			AssistantContent: "deleted assistant", LifecycleState: "active",
		}},
	}
	server := &Server{Cfg: cfg, Store: recordingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	decisionReq := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"reason":"active_chat_tail_missing_from_runtime",
		"candidate_from_turn":1,
		"removed_assistant_count":1,
		"visible_completed_turns":0,
		"backend_latest_turn":9,
		"deletion_observed":true,
		"ledger_verified":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[]
	}`))
	decisionRec := httptest.NewRecorder()
	mux.ServeHTTP(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if !decision.Allowed || decision.FromTurn != 9 || decision.DecisionToken == "" {
		t.Fatalf("decision=%+v", decision)
	}

	rollbackReq := httptest.NewRequest(http.MethodDelete, "/rollback/9?chat_session_id="+sid+"&req_source=auto&decision_token="+decision.DecisionToken+"&assistant_observation_digest="+decision.AssistantObservationDigest, nil)
	rollbackRec := httptest.NewRecorder()
	mux.ServeHTTP(rollbackRec, rollbackReq)
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}
	if len(recordingStore.deletes) == 0 {
		t.Fatal("rollback did not execute store deletions")
	}
	for _, deletion := range recordingStore.deletes {
		if !strings.HasSuffix(deletion, ":"+sid+":9") {
			t.Fatalf("rollback escaped backend tail: %s", deletion)
		}
	}
}

func TestRollbackDecisionTokenKeepsOperationSourceThroughTerminalHUD(t *testing.T) {
	const sid = "char_1_cid_postprocessor_replace"
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recordingStore := &rollbackRoutedRecordingStore{
		rollbackRecordingStore:      &rollbackRecordingStore{Store: store.NewNoopStore()},
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
	}
	server := &Server{Cfg: cfg, Store: recordingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	decisionReq := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"postprocessor_final_replace",
		"candidate_from_turn":9,
		"backend_latest_turn":9,
		"deletion_observed":true,
		"lifecycle_action_observation":"superseded"
	}`))
	decisionRec := httptest.NewRecorder()
	mux.ServeHTTP(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if !decision.Allowed || decision.DecisionToken == "" || decision.FromTurn != 9 {
		t.Fatalf("decision=%+v", decision)
	}
	detectedHUD := mapFromAny(decision.TurnWorkflowHUD)
	wantRequestID := "rollback:" + sid + ":9:postprocessor_final_replace"
	if detectedHUD["request_id"] != wantRequestID || detectedHUD["status"] != "running" {
		t.Fatalf("detection HUD=%+v want request_id=%q", detectedHUD, wantRequestID)
	}

	// The query value is intentionally different. The consumed one-use token is
	// the canonical owner of this operation and must keep the original source.
	rollbackReq := httptest.NewRequest(
		http.MethodDelete,
		"/rollback/9?chat_session_id="+sid+"&req_source=manual&decision_token="+decision.DecisionToken+"&assistant_observation_digest="+decision.AssistantObservationDigest,
		nil,
	)
	rollbackRec := httptest.NewRecorder()
	mux.ServeHTTP(rollbackRec, rollbackReq)
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rollbackRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	plan := mapFromAny(response["rollback_plan"])
	if plan["req_source"] != "postprocessor_final_replace" || plan["decision_verified"] != true {
		t.Fatalf("rollback plan lost decision source: %+v", plan)
	}
	terminalHUD := mapFromAny(response["turn_workflow_hud"])
	if terminalHUD["request_id"] != wantRequestID || terminalHUD["status"] != "completed" ||
		terminalHUD["notice_code"] != "ASSISTANT_OUTPUT_DELETE_CONFIRMED" {
		t.Fatalf("terminal HUD did not complete the detected operation: %+v", terminalHUD)
	}
	if len(recordingStore.audits) == 0 || recordingStore.audits[len(recordingStore.audits)-1].Source != "postprocessor_final_replace" {
		t.Fatalf("rollback audit source=%+v", recordingStore.audits)
	}
}

func TestRollbackDecisionCarriesTypedSupersession(t *testing.T) {
	request := rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_replace", CandidateFromTurn: 4,
		BackendLatestTurn: 4, DeletionObserved: true,
		LifecycleActionObservation: store.LogicalTurnLifecycleSuperseded,
	}
	decision := calculateRollbackDecision(request)
	if !decision.Allowed || decision.LifecycleAction != store.LogicalTurnLifecycleSuperseded {
		t.Fatalf("supersession decision=%+v", decision)
	}
	ledger := newRollbackDecisionLedger()
	record := ledger.issue(rollbackDecisionRecord{SessionID: decision.ChatSessionID, FromTurn: decision.FromTurn, RequestSource: "adapter", LifecycleAction: decision.LifecycleAction})
	consumed, ok := ledger.consume(record.Token, decision.ChatSessionID, decision.FromTurn, "")
	if !ok || consumed.LifecycleAction != store.LogicalTurnLifecycleSuperseded {
		t.Fatalf("typed supersession was not preserved by decision token: %+v ok=%v", consumed, ok)
	}
}

func TestRollbackDecisionRejectsUnknownLifecycleAction(t *testing.T) {
	decision := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_replace", CandidateFromTurn: 4,
		BackendLatestTurn: 4, DeletionObserved: true,
		LifecycleActionObservation: "guess_from_prompt_text",
	})
	if decision.Allowed || decision.Reason != "lifecycle_action_observation_invalid" {
		t.Fatalf("unknown lifecycle action was not rejected: %+v", decision)
	}
}

func TestRollbackDecisionHandlerVerifiesIncompleteUserOnlyBackendTail(t *testing.T) {
	const sid = "char_1_cid_user_only_tail"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs:                        []store.ChatLog{{ChatSessionID: sid, TurnIndex: 9, Role: "user", Content: "saved input only"}},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":9,
		"removed_assistant_count":0,
		"removed_user_count":1,
		"removed_message_count":1,
		"visible_completed_turns":8,
		"backend_latest_turn":9,
		"deletion_observed":true,
		"incomplete_tail_candidate":true
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || response.FromTurn != 9 || response.DecisionToken == "" {
		t.Fatalf("user-only tail decision=%+v", response)
	}
	if decisionStore.latestTurnCalls != 1 || decisionStore.listChatLogsFrom != 9 || decisionStore.listChatLogsTo != 9 {
		t.Fatalf("latest calls=%d chat log range=%d..%d, want one latest lookup and turn 9 only", decisionStore.latestTurnCalls, decisionStore.listChatLogsFrom, decisionStore.listChatLogsTo)
	}
}

func TestRollbackDecisionHandlerResolvesMissingBackendLatestTurn(t *testing.T) {
	const sid = "char_1_cid_manual_delete"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 5, Role: "user", Content: "target"},
			{ChatSessionID: sid, TurnIndex: 6, Role: "user", Content: "u"},
			{ChatSessionID: sid, TurnIndex: 6, Role: "assistant", Content: "a"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"manual",
		"candidate_from_turn":5,
		"deletion_observed":true,
		"allow_manual_candidate":true,
		"lifecycle_action_observation":"deleted"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || response.FromTurn != 5 || response.DecisionToken == "" {
		t.Fatalf("manual delete decision=%+v", response)
	}
	if decisionStore.latestTurnCalls != 1 {
		t.Fatalf("latest turn calls=%d, want 1", decisionStore.latestTurnCalls)
	}
}

func TestRollbackDecisionHandlerRetainsCompletedTurnsWhenOnlyUserInputsDisappear(t *testing.T) {
	const sid = "char_1_cid_user_inputs_removed"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 1, Role: "user", Content: "u1"},
			{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "a1"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "user", Content: "u2"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "a2"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", AssistantContent: "a1", AssistantObservedContentHash: prepareOR1CHash("a1"), LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", AssistantContent: "a2", AssistantObservedContentHash: prepareOR1CHash("a2"), LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":2,
		"removed_assistant_count":1,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","content_hash":"`+prepareOR1CHash("a1")+`","message_index":0},
			{"message_id":"assistant-2","content_hash":"`+prepareOR1CHash("a2")+`","message_index":1}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Allowed || response.Reason != "assistant_output_not_removed" || response.DecisionToken != "" {
		t.Fatalf("user-only deletion authorized rollback: %+v", response)
	}
}

func TestRollbackDecisionHandlerDerivesEarliestDeletedAssistantFromActiveSources(t *testing.T) {
	const sid = "char_1_cid_assistant_removed"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "a1"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "a2"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", AssistantContent: "a1", AssistantObservedContentHash: prepareOR1CHash("a1"), LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", AssistantContent: "a2", AssistantObservedContentHash: prepareOR1CHash("a2"), LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"removed_assistant_count":0,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","content_hash":"`+prepareOR1CHash("a1")+`","message_index":0}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || response.FromTurn != 2 || response.Reason != "verified_delete_range" || response.DecisionToken == "" {
		t.Fatalf("assistant deletion was not derived from active sources: %+v", response)
	}
}

func TestObservedAssistantDeletionRunsRealDecisionThenCanonicalRollback(t *testing.T) {
	const sid = "char_1_cid_real_observed_delete"
	recorder := &rollbackRecordingStore{Store: store.NewNoopStore()}
	executionStore := &rollbackDecisionExecutionStore{
		rollbackRecordingStore:      recorder,
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "a1"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "a2"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", AssistantContent: "a1", AssistantObservedContentHash: prepareOR1CHash("a1"), LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", AssistantContent: "a2", AssistantObservedContentHash: prepareOR1CHash("a2"), LifecycleState: "active"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	server := &Server{Cfg: cfg, Store: executionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	decisionReq := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":0,
		"deletion_observed":true,
		"pending_output_guard":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","content_hash":"`+prepareOR1CHash("a1")+`","message_index":0,"disabled_state":"active","streaming_state":"not_streaming","final_state":"active_final"}
		]
	}`))
	decisionRec := httptest.NewRecorder()
	mux.ServeHTTP(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if !decision.Allowed || decision.FromTurn != 2 || decision.DecisionToken == "" {
		t.Fatalf("real decision did not identify deleted turn 2: %+v", decision)
	}
	if decision.ContractVersion != rollbackDecisionContractVersion || decision.RouteBindingRevision != executionStore.revision || decision.AssistantObservationDigest == "" {
		t.Fatalf("decision did not bind v2 route and assistant evidence: %+v", decision)
	}
	detectedHUD := mapFromAny(decision.TurnWorkflowHUD)
	wantRequestID := "rollback:" + sid + ":2:auto"
	if detectedHUD["request_id"] != wantRequestID || detectedHUD["status"] != "running" {
		t.Fatalf("decision HUD=%+v", detectedHUD)
	}

	rollbackReq := httptest.NewRequest(
		http.MethodDelete,
		"/rollback/2?chat_session_id="+sid+"&req_source=auto&decision_token="+decision.DecisionToken+"&assistant_observation_digest="+decision.AssistantObservationDigest,
		nil,
	)
	rollbackRec := httptest.NewRecorder()
	mux.ServeHTTP(rollbackRec, rollbackReq)
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rollbackRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode rollback: %v", err)
	}
	plan := mapFromAny(result["rollback_plan"])
	if plan["status"] != "executed" || plan["mutation_enabled"] != true || plan["decision_verified"] != true || plan["req_source"] != "auto" {
		t.Fatalf("canonical rollback mutation was not confirmed: %+v", plan)
	}
	if len(recorder.deletes) == 0 {
		t.Fatal("real rollback handler performed no canonical delete calls")
	}
	wantSuffix := ":" + sid + ":2"
	for _, deletion := range recorder.deletes {
		if !strings.HasSuffix(deletion, wantSuffix) {
			t.Fatalf("rollback widened beyond exact turn 2: %q (all=%+v)", deletion, recorder.deletes)
		}
	}
	terminalHUD := mapFromAny(result["turn_workflow_hud"])
	if terminalHUD["request_id"] != wantRequestID || terminalHUD["status"] != "completed" {
		t.Fatalf("terminal HUD did not complete the same operation: %+v", terminalHUD)
	}
	if len(recorder.audits) != 1 || recorder.audits[0].Source != "auto" || recorder.audits[0].TargetID != 2 {
		t.Fatalf("rollback audit=%+v", recorder.audits)
	}
}

func TestRollbackDecisionV2RejectsCrossRouteSessionWithoutMutation(t *testing.T) {
	const (
		requestedSID = "session-a"
		canonicalSID = "session-b"
	)
	recorder := &rollbackRecordingStore{Store: store.NewNoopStore()}
	executionStore := &rollbackDecisionExecutionStore{
		rollbackRecordingStore:      recorder,
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(canonicalSID),
		logs:                        []store.ChatLog{{ChatSessionID: canonicalSID, TurnIndex: 1, Role: "assistant", Content: "b output"}},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: canonicalSID, TurnIndex: 1, SourceMessageID: "b-assistant-1",
			AssistantContent: "b output", LifecycleState: "active",
		}},
	}
	server := &Server{Store: executionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+requestedSID+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || decision.Allowed || decision.Reason != "session_route_canonical_mismatch" || decision.DecisionToken != "" {
		t.Fatalf("cross-route decision=%+v status=%d", decision, rec.Code)
	}
	if len(recorder.deletes) != 0 || len(recorder.audits) != 0 {
		t.Fatalf("cross-route observation mutated session A: deletes=%+v audits=%+v", recorder.deletes, recorder.audits)
	}
}

func TestRollbackDecisionV2RejectsRouteRevisionChangeBeforeMutation(t *testing.T) {
	const sid = "session-route-revision"
	recorder := &rollbackRecordingStore{Store: store.NewNoopStore()}
	route := newRollbackRouteBindingFixture(sid)
	executionStore := &rollbackDecisionExecutionStore{
		rollbackRecordingStore:      recorder,
		rollbackRouteBindingFixture: route,
		logs:                        []store.ChatLog{{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "deleted output"}},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1",
			AssistantContent: "deleted output", LifecycleState: "active",
		}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	server := &Server{Cfg: cfg, Store: executionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	decisionReq := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[]
	}`))
	decisionRec := httptest.NewRecorder()
	mux.ServeHTTP(decisionRec, decisionReq)
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.DecisionToken == "" || decision.RouteBindingRevision != route.revision {
		t.Fatalf("decision=%+v", decision)
	}

	route.revision++
	rollbackReq := httptest.NewRequest(
		http.MethodDelete,
		"/rollback/1?chat_session_id="+sid+"&req_source=auto&decision_token="+decision.DecisionToken+"&assistant_observation_digest="+decision.AssistantObservationDigest,
		nil,
	)
	rollbackRec := httptest.NewRecorder()
	mux.ServeHTTP(rollbackRec, rollbackReq)
	var result map[string]any
	if err := json.Unmarshal(rollbackRec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if rollbackRec.Code != http.StatusConflict || result["code"] != "rollback_session_route_changed" {
		t.Fatalf("route change response=%d %+v", rollbackRec.Code, result)
	}
	if len(recorder.deletes) != 0 || len(recorder.audits) != 0 {
		t.Fatalf("route change mutated session: deletes=%+v audits=%+v", recorder.deletes, recorder.audits)
	}
}

func TestRollbackDecisionV2LegacyPendingOutputFlagDoesNotOverrideObservedSupersession(t *testing.T) {
	const sid = "session-pending-output"
	server := &Server{Store: &rollbackRoutedRecordingStore{
		rollbackRecordingStore:      &rollbackRecordingStore{Store: store.NewNoopStore()},
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"postprocessor_final_replace",
		"candidate_from_turn":1,
		"deletion_observed":true,
		"pending_output_guard":true,
		"lifecycle_action_observation":"superseded"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.FromTurn != 1 || decision.LifecycleAction != store.LogicalTurnLifecycleSuperseded || decision.DecisionToken == "" {
		t.Fatalf("legacy pending output flag overrode observed supersession: %+v", decision)
	}
}

func TestRollbackDecisionV2KeepsCompleteObservationsWhenAnotherIsIncomplete(t *testing.T) {
	const sid = "session-partial-observation"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "present output"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "deleted output"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", AssistantContent: "present output", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", AssistantContent: "deleted output", LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","message_index":0},
			{"message_index":-1,"disabled_state":"unknown"}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.FromTurn != decisionStore.activeSources[1].TurnIndex || decision.DecisionToken == "" {
		t.Fatalf("complete observation was discarded: %+v", decision)
	}
	if len(decision.IncompleteAssistantObservations) != 1 || decision.IncompleteAssistantObservations[0].ObservationIndex != 1 {
		t.Fatalf("incomplete observations=%+v", decision.IncompleteAssistantObservations)
	}
}

func TestRollbackDecisionHandlerIgnoresPoisonedCounterAnchorAndUsesDeletedSourceTurn(t *testing.T) {
	const sid = "char_1_cid_deleted_36_38"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
	}
	observations := make([]rollbackAssistantObservation, 0, 35)
	for turn := 1; turn <= 38; turn++ {
		content := fmt.Sprintf("assistant output %d", turn)
		decisionStore.logs = append(decisionStore.logs, store.ChatLog{
			ChatSessionID: sid,
			TurnIndex:     turn,
			Role:          "assistant",
			Content:       content,
		})
		decisionStore.activeSources = append(decisionStore.activeSources, store.MemorySourceRevision{
			ChatSessionID:                sid,
			TurnIndex:                    turn,
			SourceMessageID:              fmt.Sprintf("assistant-%d", turn),
			AssistantContent:             content,
			AssistantObservedContentHash: prepareOR1CHash(content),
			LifecycleState:               "active",
		})
		if turn <= 35 {
			observations = append(observations, rollbackAssistantObservation{
				MessageID:    fmt.Sprintf("assistant-%d", turn),
				ContentHash:  prepareOR1CHash(content),
				MessageIndex: turn - 1,
			})
		}
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	payload, err := json.Marshal(rollbackDecisionRequest{
		ChatSessionID:             sid,
		StableCharacterID:         rollbackTestStableCharacterID,
		StableCharacterIDState:    "observed",
		HostChatID:                rollbackTestHostChatID,
		HostChatIDState:           "observed",
		RequestSource:             "auto",
		CandidateFromTurn:         3,
		FirstRemovedTurn:          3,
		LedgerAnchorTurn:          3,
		RemovedAssistantCount:     3,
		DeletionObserved:          true,
		AssistantObservationScope: "full_active_chat",
		AssistantObservations:     observations,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(string(payload)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || response.FromTurn != 36 || response.RequestedFromTurn != 3 || response.DecisionToken == "" {
		t.Fatalf("poisoned client anchor was not replaced by exact source evidence: %+v", response)
	}
}

func TestRollbackDecisionTreatsDisabledObservedAssistantAsPresent(t *testing.T) {
	const sid = "char_1_cid_disabled_output_present"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs:                        []store.ChatLog{{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "still stored"}},
		activeSources: []store.MemorySourceRevision{{
			ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1",
			AssistantContent: "still stored", LifecycleState: "active",
		}},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":1,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","message_index":0,"disabled_state":"disabled"}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Allowed || response.Reason != "assistant_output_not_removed" {
		t.Fatalf("disabled but present assistant was treated as deleted: %+v", response)
	}
}

func TestRollbackDecisionHandlerBlocksMiddleAssistantGap(t *testing.T) {
	const sid = "char_1_cid_middle_assistant_removed"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "a1"},
			{ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "a2"},
			{ChatSessionID: sid, TurnIndex: 3, Role: "assistant", Content: "a3"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", AssistantContent: "a1", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", AssistantContent: "a2", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 3, SourceMessageID: "assistant-3", AssistantContent: "a3", LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":3,
		"removed_assistant_count":0,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-1","content_hash":"`+prepareOR1CHash("a1")+`","message_index":0},
			{"message_id":"assistant-3","content_hash":"`+prepareOR1CHash("a3")+`","message_index":1}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Allowed || response.FromTurn != 0 || response.DecisionToken != "" || response.Reason != "historical_revision_conflict" {
		t.Fatalf("middle assistant gap authorized a non-tail rollback: %+v", response)
	}
}

func TestRollbackDecisionHandlerBlocksHistoricalConflictBeforeMissingTail(t *testing.T) {
	const sid = "char_1_cid_historical_conflict_before_tail"
	decisionStore := &rollbackDecisionChatLogStore{
		Store:                       store.NewNoopStore(),
		rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid),
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 87, Role: "assistant", Content: "old-87"},
			{ChatSessionID: sid, TurnIndex: 88, Role: "assistant", Content: "a88"},
			{ChatSessionID: sid, TurnIndex: 89, Role: "assistant", Content: "a89"},
			{ChatSessionID: sid, TurnIndex: 90, Role: "assistant", Content: "a90"},
		},
		activeSources: []store.MemorySourceRevision{
			{ChatSessionID: sid, TurnIndex: 87, SourceMessageID: "assistant-old-87", AssistantContent: "old-87", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 88, SourceMessageID: "assistant-88", AssistantContent: "a88", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 89, SourceMessageID: "assistant-89", AssistantContent: "a89", LifecycleState: "active"},
			{ChatSessionID: sid, TurnIndex: 90, SourceMessageID: "assistant-90", AssistantContent: "a90", LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"stable_character_id":"`+rollbackTestStableCharacterID+`",
		"stable_character_id_state":"observed",
		"host_chat_id":"`+rollbackTestHostChatID+`",
		"host_chat_id_state":"observed",
		"request_source":"auto",
		"candidate_from_turn":90,
		"deletion_observed":true,
		"assistant_observation_scope":"full_active_chat",
		"assistant_observations":[
			{"message_id":"assistant-new-87","content_hash":"`+prepareOR1CHash("new-87")+`","message_index":173},
			{"message_id":"assistant-88","content_hash":"`+prepareOR1CHash("a88")+`","message_index":175},
			{"message_id":"assistant-89","content_hash":"`+prepareOR1CHash("a89")+`","message_index":177}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Allowed || response.FromTurn != 0 || response.DecisionToken != "" || response.Reason != "historical_revision_conflict" {
		t.Fatalf("historical conflict widened a missing-tail rollback: %+v", response)
	}
}

func TestSessionRoutingBatchResolvesAssistantObservationsAgainstActiveSources(t *testing.T) {
	const sid = "char_1_cid_normalize_user_missing"
	decisionStore := &rollbackDecisionChatLogStore{
		Store: store.NewNoopStore(),
		activeSources: []store.MemorySourceRevision{
			{ID: 999, SourceRevision: "foreign-rev-1", ChatSessionID: "another-session", TurnIndex: 1, SourceMessageID: "assistant-1", UserContent: "foreign user", AssistantContent: "stored assistant 1", LifecycleState: "active"},
			{ID: 1, SourceRevision: "rev-1", ChatSessionID: sid, TurnIndex: 1, SourceMessageID: "assistant-1", UserContent: "stored user 1", AssistantContent: "stored assistant 1", LifecycleState: "active"},
			{SourceRevision: "rev-2", ChatSessionID: sid, TurnIndex: 2, SourceMessageID: "assistant-2", UserContent: "stored user 2", AssistantContent: "stored assistant 2", LifecycleState: "active"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"mode":"batch",
		"observations":[
			{"observation_index":0,"risu_assistant_message_index":0,"assistant_message_id":"assistant-1","assistant_content_hash":"`+prepareOR1CHash("stored assistant 1")+`"},
			{"observation_index":1,"risu_assistant_message_index":1,"assistant_message_id":"assistant-2","assistant_content_hash":"`+prepareOR1CHash("stored assistant 2")+`"}
		]
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.ResolvedObservations) != 2 {
		t.Fatalf("resolved observations=%+v", response.ResolvedObservations)
	}
	for index, item := range response.ResolvedObservations {
		wantTurn := index + 1
		if item.TurnIndex != wantTurn || item.Resolution != "existing_turn_by_assistant_source" ||
			item.InputMode != "stored_pair_recovered" || item.UserInputState != "restored_from_source_revision" ||
			item.StoredUserContent != fmt.Sprintf("stored user %d", wantTurn) ||
			item.StoredAssistantContent != fmt.Sprintf("stored assistant %d", wantTurn) {
			t.Fatalf("resolved[%d]=%+v", index, item)
		}
	}
}

func TestSessionRoutingFullSweepRecoversStoredPairsAndAssistantOnlyOutputs(t *testing.T) {
	const sid = "char_1_cid_179_messages"
	decisionStore := &rollbackDecisionChatLogStore{Store: store.NewNoopStore()}
	observations := make([]routingTurnObservation, 0, 84)
	for turn := 1; turn <= 84; turn++ {
		content := fmt.Sprintf("assistant output %d", turn)
		observations = append(observations, routingTurnObservation{
			ObservationIndex:     turn - 1,
			ObservedPairOrdinal:  turn,
			AssistantMessageID:   fmt.Sprintf("assistant-%d", turn),
			AssistantContentHash: prepareOR1CHash(content),
			AssistantContent:     content,
			AssistantFinalState:  "active_final",
			AdjacentUserPresent:  false,
		})
		if turn <= 15 {
			decisionStore.activeSources = append(decisionStore.activeSources, store.MemorySourceRevision{
				SourceRevision:               fmt.Sprintf("rev-%d", turn),
				ChatSessionID:                sid,
				TurnIndex:                    turn,
				SourceMessageID:              fmt.Sprintf("assistant-%d", turn),
				UserContent:                  fmt.Sprintf("stored user %d", turn),
				AssistantContent:             content,
				AssistantObservedContentHash: prepareOR1CHash(content),
				LifecycleState:               "active",
			})
		}
	}
	req := sessionRoutingTurnResolutionRequest{
		ChatSessionID:  sid,
		Mode:           "batch",
		RoutingContext: automaticActiveChatFullSweep,
		Observations:   observations,
	}
	server := &Server{Store: decisionStore}
	response := server.applyAssistantSourceRoutingResolution(
		context.Background(),
		req,
		calculateSessionRoutingTurnResolution(req),
	)
	if len(response.ResolvedObservations) != 84 ||
		response.ObservationCounts["resolved"] != 84 ||
		response.ObservationCounts["stored_pair_recovered"] != 15 ||
		response.ObservationCounts["assistant_only"] != 69 ||
		response.ObservationCounts["unresolved"] != 0 {
		t.Fatalf("full sweep counts=%+v resolved=%d", response.ObservationCounts, len(response.ResolvedObservations))
	}
	for index, item := range response.ResolvedObservations {
		turn := index + 1
		if item.TurnIndex != turn {
			t.Fatalf("resolved[%d]=%+v, want turn %d", index, item, turn)
		}
		if turn <= 15 && (item.InputMode != "stored_pair_recovered" || item.StoredUserContent == "") {
			t.Fatalf("stored pair was not recovered at turn %d: %+v", turn, item)
		}
		if turn > 15 && (item.InputMode != "assistant_only" || item.StoredUserContent != "" || item.StoredAssistantContent == "") {
			t.Fatalf("assistant-only output was not retained at turn %d: %+v", turn, item)
		}
	}
}

func TestRollbackDecisionHandlerAllowsSequentialOwnedManualTailDeletesAcrossImportedBaseline(t *testing.T) {
	const sid = "char_1_cid_manual_sequential"
	decisionStore := &rollbackDecisionChatLogStore{
		Store: store.NewNoopStore(),
		baseline: &store.SessionRoutingBaseline{
			SourceSessionID:     "parent-session",
			TargetSessionID:     sid,
			ImportedThroughTurn: 14,
			Mode:                store.SessionMigrationModeCopyKeepSource,
		},
		logs: []store.ChatLog{
			{ChatSessionID: sid, TurnIndex: 14, Role: "user", Content: "u14"},
			{ChatSessionID: sid, TurnIndex: 14, Role: "assistant", Content: "a14"},
			{ChatSessionID: sid, TurnIndex: 15, Role: "user", Content: "u15"},
			{ChatSessionID: sid, TurnIndex: 15, Role: "assistant", Content: "a15"},
		},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	requestDecision := func(turn int) rollbackDecisionResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(fmt.Sprintf(`{
			"chat_session_id":%q,
			"request_source":"manual",
			"candidate_from_turn":%d,
			"deletion_observed":true,
			"allow_manual_candidate":true,
			"lifecycle_action_observation":"deleted"
		}`, sid, turn)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var response rollbackDecisionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode turn %d response: %v", turn, err)
		}
		return response
	}

	first := requestDecision(15)
	if !first.Allowed || first.FromTurn != 15 || first.BaselineApplied || first.DecisionToken == "" {
		t.Fatalf("manual turn 15 decision=%+v", first)
	}
	decisionStore.logs = decisionStore.logs[:2]
	second := requestDecision(14)
	if !second.Allowed || second.FromTurn != 14 || second.BaselineApplied || second.DecisionToken == "" {
		t.Fatalf("manual turn 14 decision=%+v", second)
	}
}

func TestRollbackDecisionHandlerBlocksManualTargetNotOwnedByCurrentSession(t *testing.T) {
	const sid = "char_1_cid_manual_not_owned"
	decisionStore := &rollbackDecisionChatLogStore{
		Store: store.NewNoopStore(),
		logs:  []store.ChatLog{{ChatSessionID: "parent-session", TurnIndex: 14, Role: "assistant", Content: "parent only"}},
	}
	server := &Server{Store: decisionStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"request_source":"manual",
		"candidate_from_turn":14,
		"deletion_observed":true,
		"allow_manual_candidate":true,
		"lifecycle_action_observation":"deleted"
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Allowed || response.Reason != "manual_target_not_owned" || response.DecisionToken != "" {
		t.Fatalf("non-owned manual target decision=%+v", response)
	}
}

func TestRollbackDecisionRejectsUnverifiedIncompleteTailCandidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		logs []store.ChatLog
	}{
		{name: "no backend rows"},
		{name: "assistant row exists", logs: []store.ChatLog{
			{ChatSessionID: "s", TurnIndex: 4, Role: "user", Content: "u"},
			{ChatSessionID: "s", TurnIndex: 4, Role: "assistant", Content: "a"},
		}},
		{name: "candidate is not actual backend tail", logs: []store.ChatLog{
			{ChatSessionID: "s", TurnIndex: 4, Role: "user", Content: "old incomplete"},
			{ChatSessionID: "s", TurnIndex: 5, Role: "user", Content: "newer"},
			{ChatSessionID: "s", TurnIndex: 5, Role: "assistant", Content: "newer answer"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := &Server{Store: &rollbackDecisionChatLogStore{
				Store:                       store.NewNoopStore(),
				rollbackRouteBindingFixture: newRollbackRouteBindingFixture("s"),
				logs:                        tc.logs,
			}}
			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
				"chat_session_id":"s",
				"stable_character_id":"`+rollbackTestStableCharacterID+`",
				"stable_character_id_state":"observed",
				"host_chat_id":"`+rollbackTestHostChatID+`",
				"host_chat_id_state":"observed",
				"request_source":"auto",
				"candidate_from_turn":4,
				"removed_user_count":1,
				"removed_message_count":1,
				"backend_latest_turn":4,
				"deletion_observed":true,
				"incomplete_tail_candidate":true
			}`))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			var response rollbackDecisionResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Allowed || response.Reason != "incomplete_tail_not_verified" || response.DecisionToken != "" {
				t.Fatalf("unverified incomplete tail decision=%+v", response)
			}
		})
	}
}

func TestRollbackDecisionHistoryTrimGuardBlocksVerifiedIncompleteTail(t *testing.T) {
	response := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "s", CandidateFromTurn: 4, BackendLatestTurn: 4,
		DeletionObserved: true, IncompleteTailCandidate: true, BackendIncompleteTailVerified: true,
		HistoryTrimGuard: true,
	})
	if response.Allowed || response.Reason != "history_trim_guard" {
		t.Fatalf("history trim decision=%+v", response)
	}
}

func TestCopiedSessionSevenPlusTwoDeletesOnlyTurnNine(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		PreviousTurnIndex: 9, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 1, BackendLatestTurn: 9,
		DeletionObserved: true, Baseline: baseline,
	})
	if !resp.Allowed || resp.FromTurn != 9 || resp.ProtectedBeforeTurn != 7 {
		t.Fatalf("7+2 single-tail-delete decision=%+v", resp)
	}
}

func TestCopiedSessionSevenPlusTwoDeletingTwoTurnsStartsAtEight(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		PreviousTurnIndex: 9, RemovedAssistantCount: 2,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9,
		DeletionObserved: true, Baseline: baseline,
	})
	if !resp.Allowed || resp.FromTurn != 8 || resp.MinFromTurn != 8 {
		t.Fatalf("7+2 two-tail-delete decision=%+v", resp)
	}
}

func TestRollbackDecisionBlocksHistoryTrimAndOutOfRange(t *testing.T) {
	trim := calculateRollbackDecision(rollbackDecisionRequest{ChatSessionID: "s", DeletionObserved: true, CandidateFromTurn: 3, HistoryTrimGuard: true})
	if trim.Allowed || trim.Reason != "history_trim_guard" {
		t.Fatalf("trim=%+v", trim)
	}
	out := calculateRollbackDecision(rollbackDecisionRequest{ChatSessionID: "s", DeletionObserved: true, CandidateFromTurn: 9, BackendLatestTurn: 8})
	if out.Allowed || out.Reason != "delete_anchor_after_backend_tail" {
		t.Fatalf("out=%+v", out)
	}
}

func TestRollbackDecisionStillAllowsVerifiedIdleTailDeletion(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "session-1", RequestSource: "auto",
		CandidateFromTurn: 4, PreviousTurnIndex: 4,
		RemovedAssistantCount: 1, VisibleCompletedTurns: 3,
		BackendLatestTurn: 4, DeletionObserved: true, LedgerVerified: true,
	})
	if !resp.Allowed || resp.FromTurn != 4 {
		t.Fatalf("verified idle deletion should retain existing rollback behavior: %+v", resp)
	}
}

func TestSessionRoutingTurnResolutionPreservesOldPlusNewTurns(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_migrate"}
	pair := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", LocalTurnIndex: 2, Baseline: baseline})
	if pair.Resolution != "rebased" || pair.TurnIndex != 9 {
		t.Fatalf("pair=%+v", pair)
	}
	visible := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "visible_completed", VisibleCompletedTurns: 2, Baseline: baseline})
	if visible.CompletedTurns != 9 || visible.MinFromTurn != 8 {
		t.Fatalf("visible=%+v", visible)
	}
}

func TestSessionRoutingTurnResolutionDerivesTurnsFromRisuUserIndexes(t *testing.T) {
	for _, tc := range []struct {
		messageIndex int
		wantTurn     int
	}{
		{messageIndex: 0, wantTurn: 1},
		{messageIndex: 2, wantTurn: 2},
		{messageIndex: 4, wantTurn: 3},
		{messageIndex: 12, wantTurn: 7},
	} {
		messageIndex := tc.messageIndex
		got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
			Mode:                 "pair",
			RisuUserMessageIndex: &messageIndex,
			ObservedPairOrdinal:  99,
		})
		if got.TurnIndex != tc.wantTurn || got.LocalTurnIndex != tc.wantTurn || got.LocalTurnSource != "risu_user_message_index" {
			t.Fatalf("index=%d resolution=%+v", tc.messageIndex, got)
		}
	}

	nestedMarkerIndex := 20
	ordinary := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:                 "pair",
		RisuUserMessageIndex: &nestedMarkerIndex,
		ObservedPairOrdinal:  10,
	})
	if ordinary.TurnIndex != 11 || ordinary.LocalTurnIndex != 11 || ordinary.LocalTurnSource != "risu_user_message_index" {
		t.Fatalf("ordinary no-lineage raw-index contract changed: %+v", ordinary)
	}
}

func TestSessionRoutingTurnResolutionUsesObservedOrdinalOnlyWhenIndexIsUnavailable(t *testing.T) {
	oddAssistantIndex := 5
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:                 "pair",
		RisuUserMessageIndex: &oddAssistantIndex,
		ObservedPairOrdinal:  4,
		LocalTurnIndex:       77,
	})
	if got.TurnIndex != 4 || got.LocalTurnIndex != 4 || got.LocalTurnSource != "observed_pair_ordinal" {
		t.Fatalf("ordinal fallback resolution=%+v", got)
	}

	legacy := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", LocalTurnIndex: 6})
	if legacy.ContractVersion != "session-routing.turn-resolution.v1" || legacy.TurnIndex != 6 || legacy.LocalTurnSource != "legacy_local_turn_index" {
		t.Fatalf("legacy compatibility resolution=%+v", legacy)
	}
}

func TestSessionRoutingVisibleCompletedUsesRawRisuIndex(t *testing.T) {
	messageIndex := 12
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:                  "visible_completed",
		RisuUserMessageIndex:  &messageIndex,
		ObservedPairOrdinal:   6,
		VisibleCompletedTurns: 99,
	})
	if got.CompletedTurns != 7 || got.LocalTurnIndex != 7 || got.LocalTurnSource != "risu_user_message_index" {
		t.Fatalf("visible completed resolution=%+v", got)
	}
}

func TestSessionRoutingTurnResolutionBatchPreservesIndexGapsAndBaseline(t *testing.T) {
	indexes := []int{0, 4, 8}
	observations := make([]routingTurnObservation, 0, len(indexes))
	for position := range indexes {
		index := indexes[position]
		observations = append(observations, routingTurnObservation{
			ObservationIndex:     position,
			RisuUserMessageIndex: &index,
			ObservedPairOrdinal:  position + 1,
		})
	}
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:         "batch",
		Observations: observations,
		Baseline:     &routingTurnBaseline{BackendTurnAtRoute: 8, LocalPairsAtRoute: 0, Reason: "timeline_copy"},
	})
	if len(got.ResolvedObservations) != len(observations) {
		t.Fatalf("batch size=%d want=%d", len(got.ResolvedObservations), len(observations))
	}
	wantTurns := []int{9, 11, 13}
	for i, item := range got.ResolvedObservations {
		if item.ObservationIndex != i || item.TurnIndex != wantTurns[i] || item.Source != "risu_user_message_index" {
			t.Fatalf("batch[%d]=%+v wantTurn=%d", i, item, wantTurns[i])
		}
	}
}

func TestAllSessionRoutingModesProtectImportedTurnsOnFirstLocalDelete(t *testing.T) {
	for _, reason := range []string{"timeline_copy", "timeline_migrate", "timeline_attach"} {
		t.Run(reason, func(t *testing.T) {
			baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: reason}
			firstLocalTurn := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
				Mode: "pair", LocalTurnIndex: 1, Baseline: baseline,
			})
			if firstLocalTurn.TurnIndex != 8 || firstLocalTurn.ProtectedBeforeTurn != 7 || firstLocalTurn.MinFromTurn != 8 {
				t.Fatalf("first local turn must be 7+1, resolution=%+v", firstLocalTurn)
			}

			afterDelete := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
				Mode: "visible_completed", VisibleCompletedTurns: 0, Baseline: baseline,
			})
			if afterDelete.CompletedTurns != 7 || afterDelete.ProtectedBeforeTurn != 7 || afterDelete.MinFromTurn != 8 {
				t.Fatalf("empty local chat must retain imported seven turns, resolution=%+v", afterDelete)
			}

			decision := calculateRollbackDecision(rollbackDecisionRequest{
				ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "assistant_deleted_output_removed",
				CandidateFromTurn: 1, PreviousTurnIndex: 8, RemovedAssistantCount: 1,
				VisibleCompletedTurns: 0, BackendLatestTurn: 8, DeletionObserved: true, Baseline: baseline,
			})
			if !decision.Allowed || decision.FromTurn != 8 || decision.ProtectedBeforeTurn != 7 || decision.MinFromTurn != 8 {
				t.Fatalf("first local delete must remove only turn 8, decision=%+v", decision)
			}
		})
	}
}

func TestCopiedEightPlusOneRollbackDecisionExecutesOnlyTurnNine(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 8, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	decision := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: sid, RequestSource: "auto", Reason: "assistant_deleted_output_removed",
		CandidateFromTurn: 1, PreviousTurnIndex: 9, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9, DeletionObserved: true,
		Baseline: baseline,
	})
	if !decision.Allowed || decision.FromTurn != 9 || decision.ProtectedBeforeTurn != 8 || decision.MinFromTurn != 9 {
		t.Fatalf("8+1 rollback decision=%+v", decision)
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recordingStore := &rollbackRecordingStore{Store: store.NewNoopStore()}
	server := &Server{Cfg: cfg, Store: recordingStore}
	record := server.rollbackDecisionLedger().issue(rollbackDecisionRecord{SessionID: sid, FromTurn: decision.FromTurn, RequestSource: "auto", LifecycleAction: store.LogicalTurnLifecycleDeleted})
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodDelete, "/rollback/9?chat_session_id="+sid+"&req_source=auto&decision_token="+record.Token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(recordingStore.deletes) == 0 {
		t.Fatal("rollback handler did not execute store deletions")
	}
	for _, deletion := range recordingStore.deletes {
		if !strings.HasSuffix(deletion, ":"+sid+":9") {
			t.Fatalf("copied turn baseline was not preserved: %s", deletion)
		}
	}
}

func TestRollbackDecisionTokenIsOneUseAndBoundToRange(t *testing.T) {
	ledger := newRollbackDecisionLedger()
	record := ledger.issue(rollbackDecisionRecord{SessionID: "s", FromTurn: 4, RequestSource: "auto", LifecycleAction: store.LogicalTurnLifecycleDeleted})
	if _, ok := ledger.consume(record.Token, "s", 5, ""); ok {
		t.Fatal("token accepted wrong turn")
	}
	record = ledger.issue(rollbackDecisionRecord{SessionID: "s", FromTurn: 4, RequestSource: "auto", LifecycleAction: store.LogicalTurnLifecycleDeleted})
	if _, ok := ledger.consume(record.Token, "s", 4, ""); !ok {
		t.Fatal("token rejected matching decision")
	}
	if _, ok := ledger.consume(record.Token, "s", 4, ""); ok {
		t.Fatal("token reused")
	}
}

func TestRollbackHandlerKeepsOneUseWrongSessionAndWrongTurnTokenRules(t *testing.T) {
	const (
		sid      = "token-session"
		fromTurn = 4
		digest   = "assistant-digest"
	)
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recorder := &rollbackRecordingStore{Store: store.NewNoopStore()}
	server := &Server{Cfg: cfg, Store: recorder}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	issue := func() rollbackDecisionRecord {
		return server.rollbackDecisionLedger().issue(rollbackDecisionRecord{
			SessionID: sid, FromTurn: fromTurn, RequestSource: "manual",
			LifecycleAction:            store.LogicalTurnLifecycleDeleted,
			AssistantObservationDigest: digest,
		})
	}
	request := func(record rollbackDecisionRecord, requestSID string, requestTurn int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/rollback/%d?chat_session_id=%s&req_source=manual&decision_token=%s&assistant_observation_digest=%s", requestTurn, requestSID, record.Token, digest),
			nil,
		)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	if rec := request(issue(), sid, fromTurn+1); rec.Code != http.StatusConflict {
		t.Fatalf("wrong turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := request(issue(), "other-session", fromTurn); rec.Code != http.StatusConflict {
		t.Fatalf("wrong session status=%d body=%s", rec.Code, rec.Body.String())
	}
	record := issue()
	if rec := request(record, sid, fromTurn); rec.Code != http.StatusOK {
		t.Fatalf("matching token status=%d body=%s", rec.Code, rec.Body.String())
	}
	deleteCount := len(recorder.deletes)
	if deleteCount == 0 {
		t.Fatal("matching token did not execute real rollback handler")
	}
	if rec := request(record, sid, fromTurn); rec.Code != http.StatusConflict {
		t.Fatalf("reused token status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(recorder.deletes) != deleteCount {
		t.Fatalf("reused token mutated store: before=%d after=%d", deleteCount, len(recorder.deletes))
	}
}

func TestRollbackDecisionLedgerEvictsOldestTokenAtCapacity(t *testing.T) {
	ledger := newRollbackDecisionLedger()
	first := ledger.issue(rollbackDecisionRecord{SessionID: "s", FromTurn: 4, RequestSource: "auto", LifecycleAction: store.LogicalTurnLifecycleDeleted})
	var latest rollbackDecisionRecord
	for index := 1; index <= rollbackDecisionMax; index++ {
		latest = ledger.issue(rollbackDecisionRecord{SessionID: "s", FromTurn: 4 + index, RequestSource: "auto", LifecycleAction: store.LogicalTurnLifecycleDeleted})
	}
	if _, ok := ledger.consume(first.Token, "s", 4, ""); ok {
		t.Fatal("oldest rollback token survived capacity eviction")
	}
	if _, ok := ledger.consume(latest.Token, latest.SessionID, latest.FromTurn, ""); !ok {
		t.Fatal("latest rollback token was not retained")
	}
}

func TestRollbackHandlerRejectsInvalidDecisionTokenBeforeMutation(t *testing.T) {
	server := &Server{RollbackDecisions: newRollbackDecisionLedger()}
	req := httptest.NewRequest(http.MethodDelete, "/rollback/4?chat_session_id=s&req_source=auto&decision_token=invalid", nil)
	req.SetPathValue("turn_index", "4")
	rec := httptest.NewRecorder()
	server.handleRollback(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
