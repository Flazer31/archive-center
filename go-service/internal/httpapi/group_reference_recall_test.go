package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestReferenceRecallUsesExactChromaOrderThenHardFiltersTimelineBranchAndDisclosure(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{
		{NodeID: "node-start", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Start", Ordinal: 10, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 20, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-future", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Future", Ordinal: 30, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-alt", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Alternate", Ordinal: 15, BranchKey: "alternate", ReviewStatus: "approved"},
	}
	fake.claims = []store.ReferenceClaim{
		{ClaimID: "claim-safe", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Known now", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved", ValidFromNodeID: "node-start"},
		{ClaimID: "claim-spoiler", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Future reveal", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved", RevealFromNodeID: "node-future"},
		{ClaimID: "claim-alt", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Other branch", BranchKey: "alternate", KnowledgeScope: "public_world", ReviewStatus: "approved"},
	}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: true, CurrentNodeID: "node-current", RevealCeilingNodeID: "node-current", Priority: 10}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: referenceRecallVectorDocument("claim", "claim-spoiler"), ChromaRank: 1, Distance: 0.05, DistanceAvailable: true, CosineSimilarity: 0.95, CosineAvailable: true},
		{Document: referenceRecallVectorDocument("claim", "claim-alt"), ChromaRank: 2, Distance: 0.10, DistanceAvailable: true, CosineSimilarity: 0.90, CosineAvailable: true},
		{Document: referenceRecallVectorDocument("claim", "claim-safe"), ChromaRank: 3, Distance: 0.20, DistanceAvailable: true, CosineSimilarity: 0.80, CosineAvailable: true},
	}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)

	result := srv.buildSessionReferenceRecall(context.Background(), "session-1", "what is known", 3, nil)
	if result.Status != "ready" || len(result.Selected) != 1 || result.Selected[0].SourceID != "claim-safe" {
		t.Fatalf("selected = %#v status=%s", result.Selected, result.Status)
	}
	if result.Selected[0].ChromaRank != 3 || result.Selected[0].CosineSimilarity == nil || *result.Selected[0].CosineSimilarity != 0.80 {
		t.Fatalf("exact Chroma measurement changed: %#v", result.Selected[0])
	}
	if len(result.Excluded) != 2 || result.Excluded[0].Reason != "spoiler_above_reveal_ceiling" || result.Excluded[1].Reason != "branch_mismatch" {
		t.Fatalf("excluded = %#v", result.Excluded)
	}
	if vectorStore.exactQuery.Limit != 50 {
		t.Fatalf("query limit = %d, want expanded hard-filter candidate window 50", vectorStore.exactQuery.Limit)
	}
}

func TestReferenceRecallPrivateClaimRequiresCurrentSceneKnower(t *testing.T) {
	scope := referenceRecallScope{
		branchKey:     "main",
		nodes:         map[string]store.ReferenceTimelineNode{},
		sceneEntities: map[string]bool{},
	}
	claim := store.ReferenceClaim{BranchKey: "main", TemporalScope: "timeless", KnowledgeScope: "character_private", KnowerEntityIDs: []string{"npc-1"}}
	if eligible, reason := referenceRecallClaimEligible(scope, claim); eligible || reason != "knowledge_scope_not_in_scene" {
		t.Fatalf("without knower eligible=%v reason=%s", eligible, reason)
	}
	scope.sceneEntities["npc-1"] = true
	if eligible, reason := referenceRecallClaimEligible(scope, claim); !eligible || reason != "eligible_for_scene_knower" {
		t.Fatalf("with knower eligible=%v reason=%s", eligible, reason)
	}
}

func TestReferenceRecallInjectionIsSmallAndSessionFactsStayHigherPriority(t *testing.T) {
	result := referenceRecallResult{Status: "ready", Selected: []referenceRecallItem{{WorkTitle: "Example", ReferenceKind: "claim", Text: "Canon fact"}}}
	text := formatReferenceRecallInjection(result, 500)
	if text == "" || !containsAll(text, "[Original Work Reference]", "Current user input and session-established facts override", "Canon fact") {
		t.Fatalf("injection = %q", text)
	}
	if text := formatReferenceRecallInjection(result, 20); text != "" {
		t.Fatalf("tiny budget must not emit a partial header: %q", text)
	}
}

func TestPrepareTurnReferenceRecallAppliesWhenSessionIsLinked(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 10, BranchKey: "main", ReviewStatus: "approved"}}
	fake.entities = []store.ReferenceEntity{{EntityID: "entity-gate", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "gate", EntityType: "place", ReviewStatus: "approved"}}
	fake.claims = []store.ReferenceClaim{{ClaimID: "claim-safe", WorkID: "work-1", ContinuityID: "continuity-1", SubjectEntityID: "entity-gate", ClaimType: "access_rule", ClaimText: "The gate opens only at night.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", Enabled: false, InjectionEnabled: false, CurrentNodeID: "node-current"}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{{Document: referenceRecallVectorDocument("claim", "claim-safe"), ChromaRank: 1, CosineSimilarity: 0.9, CosineAvailable: true}}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	live := prepareTurnReferenceResponse(t, mux)
	if text, _ := live["injection_text"].(string); !containsAll(text, "[Original Work Reference]", "The gate opens only at night") {
		t.Fatalf("live reference injection missing: %q", text)
	}
	liveState := live["reference_injection"].(map[string]any)
	if liveState["applied"] != true || liveState["mode"] != "live" {
		t.Fatalf("live state = %#v", liveState)
	}
	if live["effective_user_input"] != "Open the gate" {
		t.Fatalf("reference recall rewrote user input: %#v", live["effective_user_input"])
	}
	coverage := live["reference_recall"].(map[string]any)["coverage_shadow"].(map[string]any)
	if coverage["mode"] != "shadow" || coverage["injection_filtered"] != false || coverage["evaluated_count"] != float64(1) {
		t.Fatalf("coverage shadow = %#v", coverage)
	}
	selected := live["reference_recall"].(map[string]any)["selected"].([]any)
	selectedItem := selected[0].(map[string]any)
	if selectedItem["coverage_status"] != "covered" || selectedItem["needed"] != true {
		t.Fatalf("selected coverage = %#v", selectedItem)
	}

	firstTurn := prepareTurnReferenceFirstTurnResponse(t, mux)
	if text, _ := firstTurn["injection_text"].(string); !containsAll(text, "[Original Work Reference]", "The gate opens only at night") {
		t.Fatalf("first-turn reference injection missing: %q", text)
	}
	pack := firstTurn["injection_pack"].(map[string]any)
	if pack["reference_applied"] != true || pack["reference_selected_count"] != float64(1) {
		t.Fatalf("first-turn reference pack = %#v", pack)
	}

	fake.bindings = nil
	unlinked := prepareTurnReferenceResponse(t, mux)
	if text, _ := unlinked["injection_text"].(string); strings.Contains(text, "[Original Work Reference]") {
		t.Fatalf("unlinked session still received reference injection: %q", text)
	}
	unlinkedState := unlinked["reference_injection"].(map[string]any)
	if unlinkedState["applied"] != false {
		t.Fatalf("unlinked state = %#v", unlinkedState)
	}
}

func prepareTurnReferenceFirstTurnResponse(t *testing.T, handler http.Handler) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      1,
		"raw_user_input":  "Begin the story",
		"settings": map[string]any{
			"injection_enabled":   false,
			"max_injection_chars": 0,
			"top_k":               0,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first-turn prepare status=%d body=%s", rec.Code, rec.Body.String())
	}
	result := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func prepareTurnReferenceResponse(t *testing.T, handler http.Handler) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      2,
		"raw_user_input":  "Open the gate",
		"messages": []map[string]any{
			{"role": "system", "content": "The gate opens only at night."},
			{"role": "user", "content": "Open the gate"},
		},
		"settings": map[string]any{
			"injection_enabled":   true,
			"max_injection_chars": 1200,
			"top_k":               3,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	result := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func referenceRecallVectorDocument(kind, sourceID string) vector.VectorDocument {
	return vector.VectorDocument{
		ID:           referenceVectorDocumentID(kind, sourceID),
		DocumentText: sourceID,
		Metadata: map[string]any{
			"work_id":            "work-1",
			"continuity_id":      "continuity-1",
			"review_status":      "approved",
			"reference_kind":     kind,
			"source_id":          sourceID,
			"embedding_model":    "embed-reference",
			"embedding_provider": "openai",
		},
	}
}

func referenceRecallTestServer(fake *referenceBindingHTTPStore, vectorStore *referenceVectorTestStore, embeddingEndpoint string) *Server {
	cfg := config.Default()
	cfg.ChromaEnabled = true
	srv := &Server{Cfg: cfg, Store: fake, ReferenceVector: vectorStore}
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingEndpoint
	srv.RuntimeConfig.EmbeddingModel = "embed-reference"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 5
	return srv
}

func containsAll(text string, values ...string) bool {
	for _, value := range values {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}
