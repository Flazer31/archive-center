package dto

import (
	"encoding/json"
	"testing"
)

func TestArcGenerateRequestUnmarshal(t *testing.T) {
	payload := `{"chat_session_id":"sess-123","force":true,"from_turn":5,"to_turn":10}`
	var req ArcGenerateRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.ChatSessionID == nil || *req.ChatSessionID != "sess-123" {
		t.Errorf("ChatSessionID mismatch")
	}
	if req.Force == nil || *req.Force != true {
		t.Errorf("Force mismatch")
	}
	if req.FromTurn == nil || *req.FromTurn != 5 {
		t.Errorf("FromTurn mismatch")
	}
	if req.ToTurn == nil || *req.ToTurn != 10 {
		t.Errorf("ToTurn mismatch")
	}
}

func TestArcGenerateRequestApplyDefaults(t *testing.T) {
	req := ArcGenerateRequest{}
	req.ApplyDefaults()
	if req.ChatSessionID == nil || *req.ChatSessionID != "" {
		t.Errorf("expected empty string default for ChatSessionID")
	}
	if req.Force == nil || *req.Force != false {
		t.Errorf("expected false default for Force")
	}
	if req.FromTurn == nil || *req.FromTurn != 0 {
		t.Errorf("expected 0 default for FromTurn")
	}
	if req.ToTurn == nil || *req.ToTurn != 0 {
		t.Errorf("expected 0 default for ToTurn")
	}
}

func TestArcGenerateRequestExplicitZeroPreserved(t *testing.T) {
	v := 0
	req := ArcGenerateRequest{FromTurn: &v}
	req.ApplyDefaults()
	if req.FromTurn == nil || *req.FromTurn != 0 {
		t.Errorf("explicit zero should be preserved")
	}
	if req.ToTurn == nil || *req.ToTurn != 0 {
		t.Errorf("ToTurn should have been defaulted to 0 because nil")
	}
}

func TestArcGenerateRequestOmitEmptyMarshal(t *testing.T) {
	req := ArcGenerateRequest{}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("re-unmarshal failed: %v", err)
	}
	// All optional fields should be omitted when nil
	if _, ok := m["chat_session_id"]; ok {
		t.Errorf("chat_session_id should be omitted")
	}
	if _, ok := m["force"]; ok {
		t.Errorf("force should be omitted")
	}
	if _, ok := m["from_turn"]; ok {
		t.Errorf("from_turn should be omitted")
	}
	if _, ok := m["to_turn"]; ok {
		t.Errorf("to_turn should be omitted")
	}
}

func TestChapterDryRunRequestDefaultInterval(t *testing.T) {
	req := ChapterDryRunRequest{}
	req.ApplyDefaults()
	if req.Interval == nil || *req.Interval != 60 {
		t.Errorf("expected default interval 60, got %v", req.Interval)
	}
}

func TestActiveScopeRequestRequiredField(t *testing.T) {
	payload := `{"active_scope":"global"}`
	var req ActiveScopeRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.ActiveScope != "global" {
		t.Errorf("ActiveScope mismatch")
	}
	// required field should remain present when marshaled
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("re-unmarshal failed: %v", err)
	}
	if _, ok := m["active_scope"]; !ok {
		t.Errorf("active_scope is required and must not be omitted")
	}
}

func TestCompileAllSchemas(t *testing.T) {
	// This test ensures all generated structs compile by instantiating each.
	_ = ActiveScopeRequest{}
	_ = ArcGenerateRequest{}
	_ = ChapterDryRunRequest{}
	_ = ChapterGenerateRequest{}
	_ = ChapterSearchRequest{}
	_ = ChatLogRepairEntryRequest{}
	_ = ChatLogRepairReplayRequest{}
	_ = ChromaShadowAdoptionGateRequest{}
	_ = ChromaShadowBackfillBatchRequest{}
	_ = ChromaShadowBackfillDryRunRequest{}
	_ = ChromaShadowFallbackRunbookRequest{}
	_ = ChromaShadowHealthProbeRequest{}
	_ = ChromaShadowRebuildDrillRequest{}
	_ = ChromaShadowReembedAuditRequest{}
	_ = ChromaShadowReleaseHygieneRequest{}
	_ = ChromaShadowVisibilityGuardRequest{}
	_ = CompleteTurnRequest{}
	_ = CriticTestRequest{}
	_ = DirectEvidenceRevalidateRequest{}
	_ = DirectorPatchRequest{}
	_ = EpisodeGenerateRequest{}
	_ = EpisodeMergeRequest{}
	_ = EpisodeSearchRequest{}
	_ = FeedbackRequest{}
	_ = HTTPValidationError{}
	_ = HypaImportRequest{}
	_ = HypaImportSummary{}
	_ = IntentRoutingRuntimeConfigRequest{}
	_ = KGRecallRequest{}
	_ = M4CompleteTurnRequest{}
	_ = M4CompleteTurnResponse{}
	_ = MaintenanceEnqueueRequest{}
	_ = MaintenanceEnqueueResponse{}
	_ = MaintenancePassRequest{}
	_ = PatchCharacterRequest{}
	_ = PatchDirectEvidenceReviewRequest{}
	_ = PatchDirectEvidenceSupersedeRequest{}
	_ = PatchDirectEvidenceTombstoneRequest{}
	_ = PatchEpisodeRequest{}
	_ = PatchKGTripleRequest{}
	_ = PatchMemoryRequest{}
	_ = PatchPendingThreadRequest{}
	_ = PatchSpeechStyleRequest{}
	_ = PatchStorylineRequest{}
	_ = PatchWorldRuleRequest{}
	_ = PrepareTurnRequest{}
	_ = PrepareTurnSettings{}
	_ = PromptUpdateRequest{}
	_ = ProxyPluginMainRequest{}
	_ = ReindexRequest{}
	_ = RescanRequest{}
	_ = RetrievalIndexRuntimeConfigRequest{}
	_ = SagaGenerateRequest{}
	_ = SaveEffectiveInputRequest{}
	_ = SaveTurnRequest{}
	_ = SearchRequest{}
	_ = SessionMigrateRequest{}
	_ = StorylineSyncRequest{}
	_ = SupervisorRequest{}
	_ = TrustControlRequest{}
	_ = ValidationError{}
	_ = WorldRuleSyncRequest{}
}
