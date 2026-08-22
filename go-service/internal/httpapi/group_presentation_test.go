package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type presentationTopologyStore struct {
	store.Store
	snapshot store.WorldlineTopologySnapshot
	err      error
	anchor   string
	limit    int
}

func (s *presentationTopologyStore) GetWorldlineTopologySnapshot(_ context.Context, anchorSessionID string, limit int) (store.WorldlineTopologySnapshot, error) {
	s.anchor = anchorSessionID
	s.limit = limit
	return s.snapshot, s.err
}

func TestBuildPresentationViewModelGroupsTimelineAndExplorerCounts(t *testing.T) {
	req := presentationViewModelRequest{
		Timeline: presentationTimelineInput{
			SelectedSessionID: "char_1_cid_a", CurrentSessionID: "char_1_cid_b", NowMS: 2000,
			Items: []map[string]any{
				{"type": "chat_log", "id": 1, "role": "user", "turn_index": 2, "content": "input", "chat_session_id": "char_1_cid_a"},
				{"type": "memory", "id": 2, "turn_index": 2, "summary_json": `{"summary":"remembered event"}`, "chat_session_id": "char_1_cid_a"},
			},
			PendingItems: []map[string]any{{"type": "pending_artifacts", "id": "expired", "turn_index": 3, "expires_at_ms": 1000}},
			Sessions:     []map[string]any{{"chat_session_id": "char_1_cid_a", "chat_logs_count": 2, "memories_count": 1}},
			Meta: map[string]any{
				"total_unpaged": 4,
				"worldline":     map[string]any{"contract_version": worldlineViewModelContract, "state": "confirmed", "current_session_id": "char_1_cid_a", "parent_session_id": "char_1_cid_parent", "fork_turn": 2, "reason": "official_branch_marker_validated"},
			},
		},
		Explorer: presentationExplorerInput{
			SelectedSessionID: "char_1_cid_a", ActiveChatSessionID: "char_1_cid_a", ActiveTab: "memories",
			Totals:     map[string]any{"chat_logs": 2, "memories": 1, "episodes": 1, "chapters": 1, "arcs": 1, "lorebook": 9},
			Trust:      map[string]any{"storylines": []any{1, 2}, "world_rules": []any{1}, "hooks": []any{1}},
			WorldGraph: map[string]any{"all_rules": []any{1, 2, 3, 4, 5}},
			Entities:   map[string]any{"characters": []any{1, 2}, "locations": []any{1, 2}, "items": []any{1, 2}},
		},
	}
	vm := buildPresentationViewModel(req)
	if vm.Timeline.Worldline.State != "confirmed" || vm.Timeline.Worldline.ForkTurn != 2 {
		t.Fatalf("timeline worldline=%v", vm.Timeline.Worldline)
	}
	if vm.Timeline.CurrentTurn != 2 {
		t.Fatalf("current turn=%d", vm.Timeline.CurrentTurn)
	}
	if vm.ContractVersion != presentationViewModelContractVersion || vm.Status != "ok" {
		t.Fatalf("contract=%+v", vm)
	}
	if len(vm.Timeline.Items) != 2 || len(vm.Timeline.Groups) != 1 {
		t.Fatalf("timeline=%+v", vm.Timeline)
	}
	group := vm.Timeline.Groups[0]
	if group.Key != "turn:2" || group.ItemCount != 2 || group.Preview != "remembered event" {
		t.Fatalf("group=%+v", group)
	}
	if vm.Timeline.Summary["total"] != 4 {
		t.Fatalf("summary=%+v", vm.Timeline.Summary)
	}
	if len(vm.Timeline.Sessions) != 1 || !vm.Timeline.Sessions[0].CanCopy {
		t.Fatalf("sessions=%+v", vm.Timeline.Sessions)
	}
	if vm.Explorer.SyncState != "current" || vm.Explorer.ActiveTab != "memories" {
		t.Fatalf("explorer=%+v", vm.Explorer)
	}
	counts := map[string]int{}
	for _, tab := range vm.Explorer.Tabs {
		counts[tab.Key] = tab.Count
	}
	if counts["episodes"] != 3 || counts["trust"] != 4 || counts["world"] != 5 || counts["entities"] != 6 {
		t.Fatalf("counts=%+v", counts)
	}
	if _, exists := counts["lorebook"]; exists {
		t.Fatalf("lorebook must be owned by Extensions, not Memory tabs: %+v", vm.Explorer.Tabs)
	}
}

func TestPresentationExplorerRehomesStaleLorebookTabToChatLogs(t *testing.T) {
	vm := buildPresentationExplorerModel(presentationExplorerInput{ActiveTab: "lorebook"})
	if vm.ActiveTab != "chat_logs" {
		t.Fatalf("active_tab=%q tabs=%+v", vm.ActiveTab, vm.Tabs)
	}
	for _, tab := range vm.Tabs {
		if tab.Key == "lorebook" {
			t.Fatalf("lorebook remained in Memory tabs: %+v", vm.Tabs)
		}
	}
}

func TestPresentationViewModelAttachesSelectedFamilyTopologyAndPreservesScalarWorldline(t *testing.T) {
	fake := &presentationTopologyStore{
		Store: store.NewNoopStore(),
		snapshot: store.WorldlineTopologySnapshot{
			StableCharacterID: "stable-character",
			AnchorSessionID:   "selected-session",
			SessionIDs:        []string{"current-session", "root-session", "selected-session"},
			LineageRecords: []store.ForkLineageRecord{
				confirmedWorldlineTopologyRecord("selected-session", "root-session", 3, "user", "msg-selected"),
				confirmedWorldlineTopologyRecord("current-session", "selected-session", 3, "char", "msg-current"),
			},
			CompletedTurns: append(
				worldlineCompletedTurns("root-session", 1, 2, 3),
				append(worldlineCompletedTurns("selected-session", 1, 2, 3), worldlineCompletedTurns("current-session", 1, 2, 3, 4)...)...,
			),
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"timeline":{
			"selected_session_id":"selected-session",
			"current_session_id":"current-session",
			"meta":{"worldline":{"contract_version":"worldline_view_model.v1","state":"confirmed","current_session_id":"selected-session","parent_session_id":"root-session","fork_turn":2,"reason":"official_branch_marker_validated"}}
		},
		"explorer":{}
	}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/presentation/view-model", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response presentationViewModelResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if fake.anchor != "selected-session" || fake.limit != worldlineTopologyFamilyLimit {
		t.Fatalf("snapshot query anchor=%q limit=%d", fake.anchor, fake.limit)
	}
	if response.Timeline.Worldline.State != "confirmed" || response.Timeline.Worldline.ParentSessionID != "root-session" {
		t.Fatalf("scalar worldline was not preserved: %+v", response.Timeline.Worldline)
	}
	topology := response.Timeline.WorldlineTopology
	if topology.ContractVersion != worldlineTopologyViewModelContract || topology.State != "ready" || topology.CurrentSessionID != "current-session" || topology.SelectedSessionID != "selected-session" {
		t.Fatalf("topology envelope=%+v", topology)
	}
	if len(topology.ActiveAncestorPath) != 4 || topology.ActiveAncestorPath[len(topology.ActiveAncestorPath)-1] != topology.CurrentNodeID {
		t.Fatalf("active node path=%v current=%q", topology.ActiveAncestorPath, topology.CurrentNodeID)
	}
	if node := worldlineTopologyNodeByTurn(t, topology, "current-session", 4); !node.Current || node.Selected || topology.CurrentNodeID != node.NodeID {
		t.Fatalf("current node=%+v", node)
	}
	if node := worldlineTopologyNodeByTurn(t, topology, "selected-session", 3); node.Current || !node.Selected || topology.SelectedNodeID != node.NodeID {
		t.Fatalf("selected node=%+v", node)
	}
}

func TestPresentationViewModelReturnsUnavailableTopologyWithoutInventingGraphOnReadError(t *testing.T) {
	fake := &presentationTopologyStore{Store: store.NewNoopStore(), err: errors.New("database read failed")}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/presentation/view-model", strings.NewReader(`{"timeline":{"selected_session_id":"selected-session","current_session_id":"current-session"},"explorer":{}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response presentationViewModelResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	topology := response.Timeline.WorldlineTopology
	if topology.State != "unavailable" || topology.Reason != "topology_snapshot_read_unavailable" || len(topology.Nodes) != 0 || len(topology.Edges) != 0 {
		t.Fatalf("read error invented topology: %+v", topology)
	}
}
