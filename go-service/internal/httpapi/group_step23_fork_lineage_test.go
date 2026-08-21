package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type forkLineageHTTPStore struct {
	store.Store
	records []store.ForkLineageRecord
	saved   store.ForkLineageRecord
}

func (f *forkLineageHTTPStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]store.ForkLineageRecord, error) {
	out := make([]store.ForkLineageRecord, 0, len(f.records))
	for _, record := range f.records {
		if record.ChatSessionID != chatSessionID {
			continue
		}
		if scopeID != "" && record.ScopeID != scopeID {
			continue
		}
		out = append(out, record)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *forkLineageHTTPStore) SaveForkLineageRecord(ctx context.Context, record store.ForkLineageRecord) (store.ForkLineageRecord, error) {
	record.ID = 88
	if record.ImportedAt.IsZero() {
		record.ImportedAt = time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC)
	}
	record.CreatedAt = record.ImportedAt
	record.UpdatedAt = record.ImportedAt
	f.saved = record
	f.records = append(f.records, record)
	return record, nil
}

func TestStep23ForkLineageListIsSupportOnlyManualMode(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &forkLineageHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.ForkLineageRecord{
			{
				ID:                  1,
				ChatSessionID:       "sess-fork",
				ScopeID:             "scope-child",
				ParentScopeID:       "scope-parent",
				CopiedFromSessionID: "sess-parent",
				ProvenanceSource:    "manual",
				InheritanceMode:     "conservative_import",
				InheritedItemsJSON:  `["consequence_records"]`,
				ImportedAt:          time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC),
			},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/step23/fork-lineage/sess-fork?scope_id=scope-child", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ForkLineageListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContractVersion != step23ForkLineageContractVersion || len(resp.Records) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.TruthBoundary.SupportOnly || resp.TruthBoundary.CanonicalTruthWriter || resp.TruthBoundary.SilentMergeBackAllowed || resp.TruthBoundary.HiddenOverwriteAllowed {
		t.Fatalf("truth boundary should prevent silent merge/overwrite: %+v", resp.TruthBoundary)
	}
	if !resp.TruthBoundary.AutomaticHookAvailable || resp.TruthBoundary.CloseoutMode != "manual_or_validated_official_risu_observation" {
		t.Fatalf("automatic observation boundary missing: %+v", resp.TruthBoundary)
	}
}

func TestStep23ForkLineageDeclareManualProvenance(t *testing.T) {
	fake := &forkLineageHTTPStore{Store: store.NewNoopStore()}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
		"chat_session_id":"sess-fork",
		"scope_id":"scope-child",
		"parent_scope_id":"scope-parent",
		"copied_from_session_id":"sess-parent",
		"imported_at":"2026-06-23T03:00:00Z",
		"divergence_marker":"{\"turn\":12}",
		"provenance_source":"manual",
		"inheritance_mode":"conservative_import",
		"inherited_items_json":"[\"consequence_records\",\"psychology_branches\"]"
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(body))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp step23ForkLineageDeclareResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Record.ID != 88 || fake.saved.ScopeID != "scope-child" || fake.saved.InheritanceMode != "conservative_import" {
		t.Fatalf("record was not saved as expected: resp=%+v saved=%+v", resp.Record, fake.saved)
	}
	if resp.TruthBoundary.CanonicalTruthWriter || resp.TruthBoundary.SilentMergeBackAllowed {
		t.Fatalf("fork lineage must not become canonical/merge writer: %+v", resp.TruthBoundary)
	}
}

func TestStep23ForkLineageDeclareRejectsAutomaticHookAndSelfParent(t *testing.T) {
	srv := NewServer(config.Default())
	srv.Store = &forkLineageHTTPStore{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(`{"chat_session_id":"sess-fork","scope_id":"scope-child","parent_scope_id":"scope-parent","provenance_source":"automatic_hook","inherited_items_json":"[]"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("automatic hook status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "reserved for automatic host observation") {
		t.Fatalf("automatic hook rejection did not explain reserved owner: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/step23/fork-lineage", strings.NewReader(`{"chat_session_id":"sess-fork","scope_id":"same","parent_scope_id":"same","provenance_source":"manual","inherited_items_json":"[]"}`))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self parent status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBuildWorldlineTopologyTurnNodesNestedBranchesAndCopiedPrefixDedupe(t *testing.T) {
	snapshot := worldlineTopologyDiagramFixture()
	vm := buildWorldlineTopologyViewModel(snapshot, "branch-2-nested", "branch-1-nested")
	if vm.ContractVersion != "worldline_topology.viewmodel.v2" || vm.State != "ready" || vm.Truncated {
		t.Fatalf("topology envelope=%+v", vm)
	}
	if len(vm.Nodes) != 13 || len(vm.Edges) != 12 {
		t.Fatalf("turn graph nodes=%+v edges=%+v", vm.Nodes, vm.Edges)
	}
	for _, node := range vm.Nodes {
		if node.X != node.TurnIndex || node.TurnKey != "turn:"+node.TurnText {
			t.Fatalf("turn alignment mismatch: %+v", node)
		}
		if node.SessionID == "route-only" {
			t.Fatalf("disconnected route-only session leaked into the anchor component: %+v", node)
		}
		switch node.SessionID {
		case "branch-1":
			if node.TurnIndex <= 2 {
				t.Fatalf("copied user-boundary prefix duplicated: %+v", node)
			}
		case "branch-2", "branch-3":
			if node.TurnIndex <= 4 {
				t.Fatalf("copied char-boundary prefix duplicated: %+v", node)
			}
		}
	}
	rootTurn2 := worldlineTopologyNodeByTurn(t, vm, "root", 2)
	branch1Turn3 := worldlineTopologyNodeByTurn(t, vm, "branch-1", 3)
	rootTurn4 := worldlineTopologyNodeByTurn(t, vm, "root", 4)
	branch2Turn5 := worldlineTopologyNodeByTurn(t, vm, "branch-2", 5)
	if rootTurn2.Y != 0 || branch1Turn3.Y != -1 ||
		worldlineTopologyNodeByTurn(t, vm, "branch-1-nested", 4).Y != -2 ||
		branch2Turn5.Y != 1 ||
		worldlineTopologyNodeByTurn(t, vm, "branch-2-nested", 6).Y != 2 ||
		worldlineTopologyNodeByTurn(t, vm, "branch-3", 5).Y != -3 {
		t.Fatalf("deterministic outer lane placement failed: %+v", vm.Nodes)
	}
	if !worldlineTopologyHasEdge(vm, rootTurn2.NodeID, branch1Turn3.NodeID, "fork") ||
		!worldlineTopologyHasEdge(vm, rootTurn4.NodeID, branch2Turn5.NodeID, "fork") {
		t.Fatalf("user/char fork boundaries were not connected exactly: %+v", vm.Edges)
	}
	current := worldlineTopologyNodeByTurn(t, vm, "branch-2-nested", 6)
	selected := worldlineTopologyNodeByTurn(t, vm, "branch-1-nested", 4)
	if vm.CurrentNodeID != current.NodeID || vm.SelectedNodeID != selected.NodeID || !current.Current || current.Selected || !selected.Selected {
		t.Fatalf("current/selected tips current=%+v selected=%+v vm=%+v", current, selected, vm)
	}
	if len(vm.ActiveAncestorPath) != 6 || vm.ActiveAncestorPath[len(vm.ActiveAncestorPath)-1] != current.NodeID {
		t.Fatalf("active turn-node path=%v", vm.ActiveAncestorPath)
	}
	if vm.Bounds.MinX != 1 || vm.Bounds.MaxX != 6 || vm.Bounds.MinY != -3 || vm.Bounds.MaxY != 2 {
		t.Fatalf("logical bounds=%+v", vm.Bounds)
	}

	reversed := snapshot
	reversed.SessionIDs = reverseStrings(snapshot.SessionIDs)
	reversed.LineageRecords = reverseForkLineageRecords(snapshot.LineageRecords)
	reversed.CompletedTurns = reverseWorldlineCompletedTurns(snapshot.CompletedTurns)
	firstJSON, err := json.Marshal(vm)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(buildWorldlineTopologyViewModel(reversed, "branch-2-nested", "branch-1-nested"))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("topology output changed with input order:\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}

func TestBuildWorldlineTopologyNestedInheritedForkResolvesVisibleAncestorOwner(t *testing.T) {
	vm := buildWorldlineTopologyViewModel(store.WorldlineTopologySnapshot{
		AnchorSessionID: "nested",
		SessionIDs:      []string{"root", "branch", "nested"},
		LineageRecords: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("branch", "root", 4, "user", "branch-msg"),
			confirmedWorldlineTopologyRecord("nested", "branch", 3, "user", "nested-msg"),
		},
		CompletedTurns: append(
			worldlineCompletedTurns("root", 1, 2, 3, 4),
			append(worldlineCompletedTurns("branch", 1, 2, 3, 4), worldlineCompletedTurns("nested", 1, 2, 3, 4)...)...,
		),
	}, "nested", "nested")
	rootTurn2 := worldlineTopologyNodeByTurn(t, vm, "root", 2)
	nestedTurn3 := worldlineTopologyNodeByTurn(t, vm, "nested", 3)
	if vm.State != "ready" || !worldlineTopologyHasEdge(vm, rootTurn2.NodeID, nestedTurn3.NodeID, "fork") {
		t.Fatalf("inherited source did not resolve to visible root owner: %+v", vm)
	}
	if worldlineTopologyNodeCount(vm, "branch", 2) != 0 {
		t.Fatalf("inherited parent prefix was rendered as branch-owned")
	}
}

func TestBuildWorldlineTopologyMissingBoundaryAndGapsNeverInventEdges(t *testing.T) {
	t.Run("missing exact fork source", func(t *testing.T) {
		vm := buildWorldlineTopologyViewModel(store.WorldlineTopologySnapshot{
			AnchorSessionID: "child",
			SessionIDs:      []string{"root", "child"},
			LineageRecords: []store.ForkLineageRecord{
				confirmedWorldlineTopologyRecord("child", "root", 2, "char", "child-msg"),
			},
			CompletedTurns: append(worldlineCompletedTurns("root", 1), worldlineCompletedTurns("child", 1, 2, 3)...),
		}, "child", "child")
		if vm.State != "partial" || vm.Reason != "fork_source_turn_missing" || worldlineTopologyEdgeCount(vm, "fork") != 0 {
			t.Fatalf("missing source invented a fork: %+v", vm)
		}
	})

	t.Run("actual completed turn gap", func(t *testing.T) {
		vm := buildWorldlineTopologyViewModel(store.WorldlineTopologySnapshot{
			AnchorSessionID: "root",
			SessionIDs:      []string{"root"},
			CompletedTurns:  worldlineCompletedTurns("root", 1, 3, 4),
		}, "root", "root")
		turn1 := worldlineTopologyNodeByTurn(t, vm, "root", 1)
		turn3 := worldlineTopologyNodeByTurn(t, vm, "root", 3)
		turn4 := worldlineTopologyNodeByTurn(t, vm, "root", 4)
		if vm.State != "partial" || vm.Reason != "completed_turn_gap" ||
			worldlineTopologyHasEdge(vm, turn1.NodeID, turn3.NodeID, "continuation") ||
			!worldlineTopologyHasEdge(vm, turn3.NodeID, turn4.NodeID, "continuation") {
			t.Fatalf("gap handling=%+v", vm)
		}
	})

	t.Run("family truncation omits parent edge", func(t *testing.T) {
		vm := buildWorldlineTopologyViewModel(store.WorldlineTopologySnapshot{
			AnchorSessionID: "child",
			SessionIDs:      []string{"child"},
			LineageRecords: []store.ForkLineageRecord{
				confirmedWorldlineTopologyRecord("child", "omitted-parent", 2, "user", "child-msg"),
			},
			CompletedTurns: worldlineCompletedTurns("child", 1, 2),
			Truncated:      true,
		}, "child", "child")
		if vm.State != "partial" || vm.Reason != "family_limit_reached" || !vm.Truncated || len(vm.Edges) != 0 ||
			worldlineTopologyNodeCount(vm, "child", 1) != 0 || worldlineTopologyNodeCount(vm, "child", 2) != 1 {
			t.Fatalf("truncated family invented parent/prefix topology: %+v", vm)
		}
	})
}

func TestBuildWorldlineTopologyUnresolvedConflictCycleAndTruncationStayPartial(t *testing.T) {
	tests := []struct {
		name     string
		snapshot store.WorldlineTopologySnapshot
		reason   string
	}{
		{
			name: "conflict",
			snapshot: store.WorldlineTopologySnapshot{
				AnchorSessionID: "child",
				SessionIDs:      []string{"root-a", "root-b", "child"},
				LineageRecords: []store.ForkLineageRecord{
					confirmedWorldlineTopologyRecord("child", "root-a", 2, "user", "msg-a"),
					confirmedWorldlineTopologyRecord("child", "root-b", 2, "user", "msg-b"),
				},
				CompletedTurns: worldlineCompletedTurns("child", 1, 2, 3),
			},
			reason: "confirmed_v2_tuple_conflict",
		},
		{
			name: "unresolved",
			snapshot: func() store.WorldlineTopologySnapshot {
				record := confirmedWorldlineTopologyRecord("child", "root", 2, "user", "msg")
				record.LineageState = "unresolved"
				return store.WorldlineTopologySnapshot{
					AnchorSessionID: "child",
					SessionIDs:      []string{"root", "child"},
					LineageRecords:  []store.ForkLineageRecord{record},
					CompletedTurns:  worldlineCompletedTurns("child", 1, 2, 3),
				}
			}(),
			reason: "unresolved_v2_lineage",
		},
		{
			name: "legacy",
			snapshot: func() store.WorldlineTopologySnapshot {
				record := confirmedWorldlineTopologyRecord("child", "root", 2, "user", "msg")
				record.ContractVersion = store.ForkLineageContractVersion
				return store.WorldlineTopologySnapshot{
					AnchorSessionID: "child",
					SessionIDs:      []string{"root", "child"},
					LineageRecords:  []store.ForkLineageRecord{record},
					CompletedTurns:  worldlineCompletedTurns("child", 1, 2, 3),
				}
			}(),
			reason: "legacy_lineage_non_authoritative",
		},
		{
			name: "cycle",
			snapshot: store.WorldlineTopologySnapshot{
				AnchorSessionID: "a",
				SessionIDs:      []string{"a", "b"},
				LineageRecords: []store.ForkLineageRecord{
					confirmedWorldlineTopologyRecord("a", "b", 2, "user", "msg-a"),
					confirmedWorldlineTopologyRecord("b", "a", 3, "char", "msg-b"),
				},
				CompletedTurns: append(worldlineCompletedTurns("a", 3), worldlineCompletedTurns("b", 4)...),
			},
			reason: "confirmed_v2_cycle_cut",
		},
		{
			name: "turn truncation",
			snapshot: store.WorldlineTopologySnapshot{
				AnchorSessionID: "root",
				SessionIDs:      []string{"root"},
				CompletedTurns:  worldlineCompletedTurns("root", 1),
				TurnsTruncated:  true,
			},
			reason: "turn_limit_reached",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := buildWorldlineTopologyViewModel(tt.snapshot, tt.snapshot.AnchorSessionID, tt.snapshot.AnchorSessionID)
			if vm.State != "partial" || vm.Reason != tt.reason || len(vm.Edges) != 0 {
				t.Fatalf("unsafe topology=%+v", vm)
			}
			if tt.name != "turn truncation" && len(vm.Nodes) != 0 {
				t.Fatalf("unsafe ownership invented turn nodes: %+v", vm.Nodes)
			}
			if tt.name == "turn truncation" && !vm.Truncated {
				t.Fatalf("turn truncation flag missing: %+v", vm)
			}
		})
	}
}

func TestBuildWorldlineTopologyScopesLineageFailuresToAnchorComponent(t *testing.T) {
	unresolved := confirmedWorldlineTopologyRecord("unresolved", "root", 2, "user", "unresolved-msg")
	unresolved.LineageState = "unresolved"
	snapshot := store.WorldlineTopologySnapshot{
		AnchorSessionID: "root",
		SessionIDs:      []string{"root", "confirmed", "unresolved", "conflict", "other-root"},
		LineageRecords: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("confirmed", "root", 2, "user", "confirmed-msg"),
			unresolved,
			confirmedWorldlineTopologyRecord("conflict", "root", 2, "user", "conflict-a"),
			confirmedWorldlineTopologyRecord("conflict", "other-root", 2, "user", "conflict-b"),
		},
		CompletedTurns: append(
			worldlineCompletedTurns("root", 1),
			append(
				worldlineCompletedTurns("confirmed", 1, 2),
				append(worldlineCompletedTurns("unresolved", 1, 2), worldlineCompletedTurns("conflict", 1, 2)...)...,
			)...,
		),
	}

	clean := buildWorldlineTopologyViewModel(snapshot, "root", "root")
	if clean.State != "ready" || clean.Reason != "topology_ready" || len(clean.Nodes) != 2 || worldlineTopologyEdgeCount(clean, "fork") != 1 {
		t.Fatalf("disconnected lineage failures downgraded clean anchor component: %+v", clean)
	}
	for _, sessionID := range []string{"unresolved", "conflict", "other-root"} {
		for _, node := range clean.Nodes {
			if node.SessionID == sessionID {
				t.Fatalf("disconnected session %q leaked into clean component: %+v", sessionID, clean.Nodes)
			}
		}
	}

	for _, tt := range []struct {
		anchor string
		reason string
	}{
		{anchor: "unresolved", reason: "unresolved_v2_lineage"},
		{anchor: "conflict", reason: "confirmed_v2_tuple_conflict"},
	} {
		t.Run(tt.anchor, func(t *testing.T) {
			selected := snapshot
			selected.AnchorSessionID = tt.anchor
			vm := buildWorldlineTopologyViewModel(selected, tt.anchor, tt.anchor)
			if vm.State != "partial" || vm.Reason != tt.reason || len(vm.Nodes) != 0 || len(vm.Edges) != 0 {
				t.Fatalf("selected unsafe component was not partial: %+v", vm)
			}
		})
	}
}

func TestBuildWorldlineTopologyCycleCutParentReasonStaysWithSelectedChild(t *testing.T) {
	vm := buildWorldlineTopologyViewModel(store.WorldlineTopologySnapshot{
		AnchorSessionID: "child",
		SessionIDs:      []string{"a", "b", "child"},
		LineageRecords: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("a", "b", 2, "user", "a-msg"),
			confirmedWorldlineTopologyRecord("b", "a", 2, "char", "b-msg"),
			confirmedWorldlineTopologyRecord("child", "a", 3, "char", "child-msg"),
		},
		CompletedTurns: worldlineCompletedTurns("child", 4),
	}, "child", "child")
	if vm.State != "partial" || vm.Reason != "confirmed_v2_cycle_cut" || len(vm.Nodes) != 1 || len(vm.Edges) != 0 {
		t.Fatalf("cycle-cut parent reason was lost from selected child: %+v", vm)
	}
}

func worldlineTopologyDiagramFixture() store.WorldlineTopologySnapshot {
	completedTurns := worldlineCompletedTurns("root", 1, 2, 3, 4, 5, 6)
	for _, sessionTurns := range []struct {
		sessionID string
		turns     []int
	}{
		{"branch-1", []int{1, 2, 3, 4}},
		{"branch-1-nested", []int{1, 2, 3, 4}},
		{"branch-2", []int{1, 2, 3, 4, 5, 6}},
		{"branch-2-nested", []int{1, 2, 3, 4, 5, 6}},
		{"branch-3", []int{1, 2, 3, 4, 5}},
		{"route-only", []int{1, 2, 3}},
	} {
		completedTurns = append(completedTurns, worldlineCompletedTurns(sessionTurns.sessionID, sessionTurns.turns...)...)
	}
	return store.WorldlineTopologySnapshot{
		StableCharacterID: "stable-character",
		AnchorSessionID:   "branch-2-nested",
		SessionIDs: []string{
			"route-only", "branch-2", "root", "branch-1-nested", "branch-3", "branch-1", "branch-2-nested",
		},
		LineageRecords: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("branch-3", "root", 4, "char", "msg-branch-3"),
			confirmedWorldlineTopologyRecord("branch-1-nested", "branch-1", 4, "user", "msg-branch-1-nested"),
			confirmedWorldlineTopologyRecord("branch-2", "root", 4, "char", "msg-branch-2"),
			confirmedWorldlineTopologyRecord("branch-1", "root", 3, "user", "msg-branch-1"),
			confirmedWorldlineTopologyRecord("branch-2-nested", "branch-2", 5, "char", "msg-branch-2-nested"),
		},
		CompletedTurns: completedTurns,
	}
}

func confirmedWorldlineTopologyRecord(childSessionID, parentSessionID string, forkTurn int, sourceRole, sourceMessageID string) store.ForkLineageRecord {
	return store.ForkLineageRecord{
		ContractVersion:     store.RisuWorldlineForkLineageContractVersion,
		LineageState:        "confirmed",
		ChatSessionID:       childSessionID,
		CopiedFromSessionID: parentSessionID,
		ForkTurn:            forkTurn,
		ForkSourceMessageID: sourceMessageID,
		ForkSourceRole:      sourceRole,
		IdempotencyKey:      "risu-worldline:" + childSessionID + ":" + parentSessionID + ":" + sourceMessageID,
	}
}

func worldlineTopologyNodeByTurn(t *testing.T, vm worldlineTopologyViewModel, sessionID string, turnIndex int) worldlineTopologyNode {
	t.Helper()
	for _, node := range vm.Nodes {
		if node.SessionID == sessionID && node.TurnIndex == turnIndex {
			return node
		}
	}
	t.Fatalf("node %q turn %d missing from %+v", sessionID, turnIndex, vm.Nodes)
	return worldlineTopologyNode{}
}

func worldlineTopologyNodeCount(vm worldlineTopologyViewModel, sessionID string, turnIndex int) int {
	count := 0
	for _, node := range vm.Nodes {
		if node.SessionID == sessionID && node.TurnIndex == turnIndex {
			count++
		}
	}
	return count
}

func worldlineTopologyHasEdge(vm worldlineTopologyViewModel, parentNodeID, childNodeID, kind string) bool {
	for _, edge := range vm.Edges {
		if edge.ParentNodeID == parentNodeID && edge.ChildNodeID == childNodeID && edge.Kind == kind {
			return true
		}
	}
	return false
}

func worldlineTopologyEdgeCount(vm worldlineTopologyViewModel, kind string) int {
	count := 0
	for _, edge := range vm.Edges {
		if edge.Kind == kind {
			count++
		}
	}
	return count
}

func worldlineCompletedTurns(sessionID string, turnIndexes ...int) []store.WorldlineCompletedTurn {
	turns := make([]store.WorldlineCompletedTurn, 0, len(turnIndexes))
	for _, turnIndex := range turnIndexes {
		turns = append(turns, store.WorldlineCompletedTurn{ChatSessionID: sessionID, TurnIndex: turnIndex})
	}
	return turns
}

func reverseStrings(values []string) []string {
	out := append([]string(nil), values...)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}

func TestPrepareTurnHistoryScopeComposesConfirmedNestedWorldlineOwnership(t *testing.T) {
	fake := &forkLineageHTTPStore{
		Store: store.NewNoopStore(),
		records: []store.ForkLineageRecord{
			confirmedWorldlineTopologyRecord("branch-1", "root", 8, "char", "root-turn-8"),
			confirmedWorldlineTopologyRecord("branch-2", "branch-1", 9, "char", "branch-1-turn-9"),
		},
	}
	scope := resolvePrepareTurnHistoryScope(context.Background(), fake, "branch-2", 11)
	if scope.State != "ready" || scope.Reason != "confirmed_worldline_history_composed" {
		t.Fatalf("unexpected scope state: %+v", scope)
	}
	want := []prepareTurnHistorySegment{
		{SessionID: "root", FromTurn: 0, ToTurn: 8},
		{SessionID: "branch-1", FromTurn: 9, ToTurn: 9},
		{SessionID: "branch-2", FromTurn: 10, ToTurn: 10},
	}
	if !prepareTurnHistorySegmentsEqual(scope.Segments, want) {
		t.Fatalf("segments=%+v, want %+v", scope.Segments, want)
	}
}

func TestPrepareTurnHistoryScopeUsesUserForkBoundaryAndFailsClosedOnConflict(t *testing.T) {
	t.Run("user_fork_turn_one_has_no_inherited_parent_turn", func(t *testing.T) {
		fake := &forkLineageHTTPStore{
			Store: store.NewNoopStore(),
			records: []store.ForkLineageRecord{
				confirmedWorldlineTopologyRecord("child", "root", 1, "user", "root-user-1"),
			},
		}
		scope := resolvePrepareTurnHistoryScope(context.Background(), fake, "child", 2)
		want := []prepareTurnHistorySegment{{SessionID: "child", FromTurn: 1, ToTurn: 1}}
		if scope.State != "ready" || !prepareTurnHistorySegmentsEqual(scope.Segments, want) {
			t.Fatalf("scope=%+v, want segments %+v", scope, want)
		}
	})

	t.Run("conflicting_confirmed_tuples_do_not_guess_a_parent", func(t *testing.T) {
		left := confirmedWorldlineTopologyRecord("child", "root-a", 8, "char", "root-a-turn-8")
		right := confirmedWorldlineTopologyRecord("child", "root-b", 8, "char", "root-b-turn-8")
		fake := &forkLineageHTTPStore{Store: store.NewNoopStore(), records: []store.ForkLineageRecord{left, right}}
		scope := resolvePrepareTurnHistoryScope(context.Background(), fake, "child", 10)
		want := []prepareTurnHistorySegment{{SessionID: "child", FromTurn: 0, ToTurn: 9}}
		if scope.State != "partial" || scope.Reason != "confirmed_worldline_tuple_conflict" || !prepareTurnHistorySegmentsEqual(scope.Segments, want) {
			t.Fatalf("scope=%+v, want current-only conflict scope %+v", scope, want)
		}
	})
}

func prepareTurnHistorySegmentsEqual(left, right []prepareTurnHistorySegment) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func reverseForkLineageRecords(values []store.ForkLineageRecord) []store.ForkLineageRecord {
	out := append([]store.ForkLineageRecord(nil), values...)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}

func reverseWorldlineCompletedTurns(values []store.WorldlineCompletedTurn) []store.WorldlineCompletedTurn {
	out := append([]store.WorldlineCompletedTurn(nil), values...)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}
