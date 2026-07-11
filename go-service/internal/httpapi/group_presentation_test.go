package httpapi

import "testing"

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
			Meta:         map[string]any{"total_unpaged": 4},
		},
		Explorer: presentationExplorerInput{
			SelectedSessionID: "char_1_cid_a", ActiveChatSessionID: "char_1_cid_a", ActiveTab: "memories",
			Totals:     map[string]any{"chat_logs": 2, "memories": 1, "episodes": 1, "chapters": 1, "arcs": 1},
			Trust:      map[string]any{"storylines": []any{1, 2}, "world_rules": []any{1}, "hooks": []any{1}},
			WorldGraph: map[string]any{"all_rules": []any{1, 2, 3, 4, 5}},
			Entities:   map[string]any{"characters": []any{1, 2}, "locations": []any{1, 2}, "items": []any{1, 2}},
		},
	}
	vm := buildPresentationViewModel(req)
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
}
