package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type referenceVectorWithoutExact struct{ vector.VectorStore }

type failingReferenceCoverageStore struct{ *referenceBindingHTTPStore }

func (f *failingReferenceCoverageStore) ReplaceSessionReferenceCoverageSnapshot(context.Context, *store.SessionReferenceCoverageSnapshot, []store.SessionReferenceCoverageField) (bool, error) {
	return false, errors.New("coverage persistence unavailable")
}

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
	approvedCount := len(fake.timeline) + len(fake.claims)
	if vectorStore.exactQuery.Limit != approvedCount {
		t.Fatalf("query limit = %d, want approved source count %d", vectorStore.exactQuery.Limit, approvedCount)
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

func TestReferenceRecallSupplementInjectionKeepsSessionFactsHigherPriority(t *testing.T) {
	result := referenceRecallResult{Status: "ready", InjectionItems: []referenceInjectionItem{{WorkTitle: "Example", ReferenceKind: "claim", ReferenceMode: referenceModeSupplement, Text: "Canon fact"}}}
	formatted := formatReferenceRecallInjection(result, 500)
	text := formatted.Text
	if text == "" || !containsAll(text, "[Original Work Reference]", "Current user input and session-established facts override", "Canon fact") {
		t.Fatalf("injection = %q", text)
	}
	if formatted.IncludedCount != len(result.InjectionItems) {
		t.Fatalf("included count = %d, want %d", formatted.IncludedCount, len(result.InjectionItems))
	}
	if tiny := formatReferenceRecallInjection(result, 20); tiny.Text != "" || tiny.IncludedCount != 0 {
		t.Fatalf("tiny budget must not emit a partial header: %#v", tiny)
	}
}

func TestReferenceRecallPrimaryInjectionOverridesUnsupportedSessionInvention(t *testing.T) {
	result := referenceRecallResult{Status: "ready", InjectionItems: []referenceInjectionItem{{WorkTitle: "Example", ReferenceKind: "claim", ReferenceMode: referenceModePrimary, Text: "HUNTR/X consists of Rumi, Mira, and Zoey."}}}
	text := formatReferenceRecallInjection(result, 900).Text
	if !containsAll(text,
		"user-authored divergence override",
		"Approved primary canon overrides unsupported model-invented or session-derived claims",
		"Preserve session-original additions only when they do not conflict",
		"HUNTR/X consists of Rumi, Mira, and Zoey.",
	) {
		t.Fatalf("primary precedence missing: %q", text)
	}
	if strings.Contains(text, "session-established facts override this reference") {
		t.Fatalf("primary mode still lets generated session claims override canon: %q", text)
	}
}

func TestReferenceRecallDegradedFormatterIncludesItemsAndReportsExactCount(t *testing.T) {
	result := referenceRecallResult{Status: "degraded", InjectionItems: []referenceInjectionItem{
		{WorkTitle: "Example", ReferenceKind: "claim", ReferenceMode: referenceModePrimary, Text: "First fact."},
		{WorkTitle: "Example", ReferenceKind: "claim", ReferenceMode: referenceModePrimary, Text: "Second fact."},
	}}
	formatted := formatReferenceRecallInjection(result, 900)
	if formatted.IncludedCount != len(result.InjectionItems) || !containsAll(formatted.Text, "First fact.", "Second fact.") {
		t.Fatalf("degraded formatter = %#v", formatted)
	}
}

func TestReferenceRecallStatusDistinguishesFailedDegradedEmptyAndReady(t *testing.T) {
	newFixture := func(bindingCount int, vectorStore *referenceVectorTestStore) *Server {
		fake := newReferenceBindingHTTPStore()
		fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
		fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
		fake.claims = []store.ReferenceClaim{{ClaimID: "claim-1", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "The archive opens at dusk.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"}}
		for i := 0; i < bindingCount; i++ {
			fake.bindings = append(fake.bindings, store.SessionReferenceBinding{BindingID: fmt.Sprintf("binding-%d", i), ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", ReferenceMode: referenceModePrimary})
		}
		embeddingServer, _ := referenceVectorEmbeddingServer(t)
		t.Cleanup(embeddingServer.Close)
		return referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	}
	doc := vector.ExactQueryResult{Document: referenceRecallVectorDocument("claim", "claim-1"), ChromaRank: 1}

	missingInput := (&Server{Store: store.NewNoopStore()}).buildSessionReferenceRecall(context.Background(), "session-1", "", 1, nil)
	if missingInput.Status != "skipped" {
		t.Fatalf("missing query status = %q", missingInput.Status)
	}
	missingStore := (&Server{Store: store.NewNoopStore()}).buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if missingStore.Status != "failed" || !containsAll(strings.Join(missingStore.Warnings, "\n"), "reference_store_unavailable") {
		t.Fatalf("missing reference store = %#v", missingStore)
	}

	missingExactServer := newFixture(1, &referenceVectorTestStore{})
	missingExactServer.ReferenceVector = &referenceVectorWithoutExact{VectorStore: vector.NewFakeVectorStore()}
	missingExact := missingExactServer.buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if missingExact.Status != "failed" || !containsAll(strings.Join(missingExact.Warnings, "\n"), "reference_vector_exact_query_unavailable") {
		t.Fatalf("missing exact metadata querier = %#v", missingExact)
	}

	missingEmbeddingServer := newFixture(1, &referenceVectorTestStore{})
	missingEmbeddingServer.RuntimeConfig.EmbeddingAPIKey = ""
	missingEmbedding := missingEmbeddingServer.buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if missingEmbedding.Status != "failed" || !containsAll(strings.Join(missingEmbedding.Warnings, "\n"), "embedding_config_missing") {
		t.Fatalf("missing embedding config = %#v", missingEmbedding)
	}

	failed := newFixture(1, &referenceVectorTestStore{exactErr: errors.New("query unavailable")}).buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if failed.Status != "failed" {
		t.Fatalf("all query failures status = %q warnings=%#v", failed.Status, failed.Warnings)
	}

	degradedStore := &referenceVectorTestStore{exactResultsByCall: [][]vector.ExactQueryResult{{doc}, nil}, exactErrsByCall: []error{nil, errors.New("second query unavailable")}}
	degraded := newFixture(2, degradedStore).buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if degraded.Status != "degraded" || len(degraded.Selected) != 1 {
		t.Fatalf("partial query status=%q selected=%#v warnings=%#v", degraded.Status, degraded.Selected, degraded.Warnings)
	}

	empty := newFixture(1, &referenceVectorTestStore{}).buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if empty.Status != "empty" || len(empty.Selected) != 0 {
		t.Fatalf("successful empty query = %#v", empty)
	}

	ready := newFixture(1, &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{doc}}).buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if ready.Status != "ready" || len(ready.Selected) != 1 || len(ready.InjectionItems) > 1 {
		t.Fatalf("successful valid query = %#v", ready)
	}

	degradedCoverageServer := newFixture(1, &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{doc}})
	degradedCoverageServer.Store = &failingReferenceCoverageStore{referenceBindingHTTPStore: degradedCoverageServer.Store.(*referenceBindingHTTPStore)}
	degradedCoverage := degradedCoverageServer.buildSessionReferenceRecall(context.Background(), "session-1", "archive", 1, nil)
	if degradedCoverage.Status != "degraded" || degradedCoverage.CoverageShadow.FieldIndex.Status != "degraded" {
		t.Fatalf("coverage persistence failure status=%q field_index=%#v warnings=%#v", degradedCoverage.Status, degradedCoverage.CoverageShadow.FieldIndex, degradedCoverage.Warnings)
	}
}

func TestReferenceRecallReportsOpenFailureWithoutBreakingPrepareTurn(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1"}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	srv := referenceRecallTestServer(fake, &referenceVectorTestStore{}, embeddingServer.URL)
	srv.ReferenceVectorOpenError = errors.New("reference collection cannot open")

	recall := srv.buildSessionReferenceRecall(context.Background(), "session-1", "continue", 1, nil)
	if recall.Status != "failed" || !stringSliceContains(recall.Warnings, "reference_vector_open_failed") {
		t.Fatalf("reference open failure = %#v", recall)
	}
	srv.ReferenceVectorOpenError = nil
	srv.ReferenceVector = nil
	unavailable := srv.buildSessionReferenceRecall(context.Background(), "session-1", "continue", 1, nil)
	if unavailable.Status != "failed" || !stringSliceContains(unavailable.Warnings, "reference_vector_unavailable") {
		t.Fatalf("missing reference vector = %#v", unavailable)
	}
	srv.ReferenceVectorOpenError = errors.New("reference collection cannot open")

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	prepared := prepareTurnReferenceResponse(t, mux)
	preparedRecall := prepared["reference_recall"].(map[string]any)
	if preparedRecall["status"] != "failed" {
		t.Fatalf("prepare reference recall = %#v", preparedRecall)
	}
	if prepared["status"] != "ok" || prepared["injection_text"] != nil {
		t.Fatalf("reference failure changed main prepare result: %#v", prepared)
	}
	referenceInjection := prepared["reference_injection"].(map[string]any)
	if referenceInjection["applied"] != false || referenceInjection["injected_count"] != float64(0) {
		t.Fatalf("reference failure injected content: %#v", referenceInjection)
	}
}

func TestReferenceRecallPrimaryPreservesRequestedItemLimit(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
	fake.claims = []store.ReferenceClaim{
		{ClaimID: "claim-1", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "First fact.", TemporalScope: "timeless", BranchKey: "main", ReviewStatus: "approved"},
		{ClaimID: "claim-2", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Second fact.", TemporalScope: "timeless", BranchKey: "main", ReviewStatus: "approved"},
	}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", ReferenceMode: referenceModePrimary}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: referenceRecallVectorDocument("claim", "claim-1"), ChromaRank: 1},
		{Document: referenceRecallVectorDocument("claim", "claim-2"), ChromaRank: 2},
	}}
	result := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL).buildSessionReferenceRecall(context.Background(), "session-1", "fact", 1, nil)
	if len(result.Selected) != 1 || len(result.InjectionItems) > 1 {
		t.Fatalf("primary expanded requested limit: selected=%d injection=%d", len(result.Selected), len(result.InjectionItems))
	}
	if vectorStore.exactQuery.Limit != len(fake.timeline)+len(fake.claims) {
		t.Fatalf("query limit = %d, want approved ID count %d", vectorStore.exactQuery.Limit, len(fake.timeline)+len(fake.claims))
	}
}

func TestReferenceRecallPreviewDistinguishesOmittedLimitFromExplicitZero(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
	fake.claims = []store.ReferenceClaim{
		{ClaimID: "claim-1", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "First fact.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"},
		{ClaimID: "claim-2", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Second fact.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"},
	}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", ReferenceMode: referenceModePrimary}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: referenceRecallVectorDocument("claim", "claim-1"), ChromaRank: 1},
		{Document: referenceRecallVectorDocument("claim", "claim-2"), ChromaRank: 2},
	}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	call := func(body map[string]any) map[string]any {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/sessions/session-1/reference-recall/preview", bytes.NewReader(raw))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
		}
		result := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	omitted := call(map[string]any{"query": "fact"})
	queryCountAfterOmitted := len(vectorStore.exactQueries)
	explicitZero := call(map[string]any{"query": "fact", "limit": 0})
	if len(omitted["selected"].([]any)) != len(vectorStore.exactResults) {
		t.Fatalf("omitted limit did not derive from approved inventory: %#v", omitted)
	}
	if len(explicitZero["selected"].([]any)) != 0 || len(explicitZero["injection_items"].([]any)) != 0 {
		t.Fatalf("explicit zero limit was replaced: %#v", explicitZero)
	}
	if len(vectorStore.exactQueries) != queryCountAfterOmitted {
		t.Fatalf("explicit zero limit performed a vector query: before=%d after=%d", queryCountAfterOmitted, len(vectorStore.exactQueries))
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
	policy := liveState["budget_policy"].(map[string]any)
	if policy["contract_version"] != referenceInjectionBudgetContractVersion || policy["mode"] != referenceModeSupplement || policy["total_cap_chars"] != float64(600) || policy["relationship_to_main"] != "additive_non_displacing" {
		t.Fatalf("supplement budget contract = %#v", policy)
	}
	pack := live["injection_pack"].(map[string]any)
	referenceText, _ := pack["reference_text"].(string)
	if policy["used_chars"] != float64(len([]rune(referenceText))) || policy["used_chars"].(float64) > policy["total_cap_chars"].(float64) {
		t.Fatalf("supplement budget usage = policy:%#v text:%q", policy, referenceText)
	}
	if memoryText, _ := pack["memory_text"].(string); strings.Contains(memoryText, "[Original Work Reference]") {
		t.Fatalf("reference entered the main memory lane: %q", memoryText)
	}
	if live["effective_user_input"] != "Open the gate" {
		t.Fatalf("reference recall rewrote user input: %#v", live["effective_user_input"])
	}
	coverage := live["reference_recall"].(map[string]any)["coverage_shadow"].(map[string]any)
	if coverage["mode"] != "applied" || coverage["injection_filtered"] != true || coverage["evaluated_count"] != float64(1) {
		t.Fatalf("coverage application = %#v", coverage)
	}
	selected := live["reference_recall"].(map[string]any)["selected"].([]any)
	selectedItem := selected[0].(map[string]any)
	if selectedItem["coverage_status"] != "partial" || selectedItem["needed"] != true {
		t.Fatalf("selected coverage = %#v", selectedItem)
	}
	injectionItems := live["reference_recall"].(map[string]any)["injection_items"].([]any)
	if len(injectionItems) != 1 || injectionItems[0].(map[string]any)["selection_source"] != "chroma_candidate" {
		t.Fatalf("coverage injection items = %#v", injectionItems)
	}

	firstTurn := prepareTurnReferenceFirstTurnResponse(t, mux)
	if firstTurn["injection_text"] != nil {
		t.Fatalf("disabled first-turn reference injection = %#v", firstTurn["injection_text"])
	}
	pack = firstTurn["injection_pack"].(map[string]any)
	if pack["reference_applied"] != false || pack["reference_selected_count"] != float64(0) || pack["reference_text"] != nil {
		t.Fatalf("first-turn reference pack = %#v", pack)
	}
	firstTurnState := firstTurn["reference_injection"].(map[string]any)
	firstTurnSelected := len(firstTurn["reference_recall"].(map[string]any)["injection_items"].([]any))
	if firstTurnState["enabled"] != false || firstTurnState["applied"] != false || firstTurnState["selected_count"] != float64(firstTurnSelected) || firstTurnState["injected_count"] != float64(0) {
		t.Fatalf("disabled first-turn reference state = %#v", firstTurnState)
	}

	zeroBudget := prepareTurnReferenceSettingsResponse(t, mux, true, 0)
	if zeroBudget["injection_text"] != nil {
		t.Fatalf("zero-budget reference injection = %#v", zeroBudget["injection_text"])
	}
	zeroBudgetPack := zeroBudget["injection_pack"].(map[string]any)
	zeroBudgetState := zeroBudget["reference_injection"].(map[string]any)
	zeroBudgetSelected := len(zeroBudget["reference_recall"].(map[string]any)["injection_items"].([]any))
	if zeroBudgetPack["reference_applied"] != false || zeroBudgetPack["reference_selected_count"] != float64(0) || zeroBudgetPack["reference_text"] != nil || zeroBudgetState["enabled"] != false || zeroBudgetState["selected_count"] != float64(zeroBudgetSelected) || zeroBudgetState["injected_count"] != float64(0) {
		t.Fatalf("zero-budget reference surfaces: pack=%#v state=%#v", zeroBudgetPack, zeroBudgetState)
	}

	covered := prepareTurnReferenceResponseWithSystem(t, mux, "The gate opens only at night.")
	if text, _ := covered["injection_text"].(string); strings.Contains(text, "[Original Work Reference]") {
		t.Fatalf("covered reference was injected again: %q", text)
	}
	coveredState := covered["reference_injection"].(map[string]any)
	if coveredState["applied"] != false || coveredState["selected_count"] != float64(0) {
		t.Fatalf("covered reference state = %#v", coveredState)
	}
	coveredRecall := covered["reference_recall"].(map[string]any)
	if len(coveredRecall["selected"].([]any)) != 1 || len(coveredRecall["injection_items"].([]any)) != 0 {
		t.Fatalf("raw and applied reference lists were not separated: %#v", coveredRecall)
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

func TestPrepareTurnReferenceCountsSelectedSeparatelyFromCharacterBudgetInclusion(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
	fake.claims = []store.ReferenceClaim{
		{ClaimID: "claim-short", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "A short relevant fact.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"},
		{ClaimID: "claim-long", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: strings.Repeat("A longer relevant fact. ", 60), TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"},
	}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", ReferenceMode: referenceModePrimary}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: referenceRecallVectorDocument("claim", "claim-short"), ChromaRank: 1},
		{Document: referenceRecallVectorDocument("claim", "claim-long"), ChromaRank: 2},
	}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	response := prepareTurnReferenceSettingsResponse(t, mux, true, 650)
	state := response["reference_injection"].(map[string]any)
	pack := response["injection_pack"].(map[string]any)
	if state["selected_count"] != float64(len(vectorStore.exactResults)) || state["injected_count"] != float64(1) {
		t.Fatalf("selected/injected counts = %#v", state)
	}
	if pack["reference_selected_count"] != state["injected_count"] {
		t.Fatalf("pack included count diverged from formatter count: pack=%#v state=%#v", pack, state)
	}
}

func TestPrepareTurnReferenceCoverageUsesStoreSceneSignals(t *testing.T) {
	fake := newReferenceBindingHTTPStore()
	fake.referenceLibraryHTTPStore.Store = &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ID: 1, ChatSessionID: "session-1", TurnIndex: 3, Role: "user", Content: "Rumi steps onto the stage."},
		{ID: 2, ChatSessionID: "session-1", TurnIndex: 3, Role: "assistant", Content: "Rumi checks the rehearsal marks."},
	}}
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Current", Ordinal: 10, BranchKey: "main", ReviewStatus: "approved"}}
	fake.entities = []store.ReferenceEntity{{EntityID: "entity-rumi", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Rumi", EntityType: "character", DescriptionText: "Huntrix's leader.", ReviewStatus: "approved"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1", CurrentNodeID: "node-current", Priority: 10}}
	embeddingServer, _ := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{{Document: referenceRecallVectorDocument("entity", "entity-rumi"), ChromaRank: 1, CosineSimilarity: 0.9, CosineAvailable: true}}}
	srv := referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      4,
		"raw_user_input":  "Continue.",
		"messages":        []map[string]any{{"role": "user", "content": "Continue."}},
		"settings": map[string]any{
			"injection_enabled":   true,
			"max_injection_chars": 1200,
			"top_k":               3,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn status=%d body=%s", rec.Code, rec.Body.String())
	}
	result := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	recall := result["reference_recall"].(map[string]any)
	coverage := recall["coverage_shadow"].(map[string]any)
	if coverage["contract_version"] != "coverage_shadow.v3" || coverage["mode"] != "applied" || coverage["injection_filtered"] != true {
		t.Fatalf("coverage contract = %#v", coverage)
	}
	fieldIndex := coverage["field_index"].(map[string]any)
	if fieldIndex["contract_version"] != "coverage_field_index.v1" || fieldIndex["status"] != "ready" {
		t.Fatalf("field index contract = %#v", fieldIndex)
	}
	sceneSignals := coverage["scene_signals"].(map[string]any)
	if sceneSignals["recent_completed_turn"] != float64(3) || sceneSignals["recent_dialogue_count"] != float64(2) {
		t.Fatalf("scene signals = %#v", sceneSignals)
	}
	selected := recall["selected"].([]any)[0].(map[string]any)
	if selected["coverage_status"] != "partial" || selected["needed"] != true || !anyStringSliceContains(selected["needed_by"], "recent_completed_dialogue") {
		t.Fatalf("store scene coverage = %#v", selected)
	}
	if len(selected["matched_request_locations"].([]any)) != 0 || len(selected["matched_context_locations"].([]any)) == 0 {
		t.Fatalf("store scene match locations = %#v", selected)
	}
}

func prepareTurnReferenceFirstTurnResponse(t *testing.T, handler http.Handler) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      1,
		"raw_user_input":  "Begin the story at the gate",
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

func prepareTurnReferenceSettingsResponse(t *testing.T, handler http.Handler, injectionEnabled bool, maxInjectionChars int) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      2,
		"raw_user_input":  "Open the gate",
		"messages":        []map[string]any{{"role": "user", "content": "Open the gate"}},
		"settings": map[string]any{
			"injection_enabled":   injectionEnabled,
			"max_injection_chars": maxInjectionChars,
			"top_k":               3,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare-turn settings status=%d body=%s", rec.Code, rec.Body.String())
	}
	result := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func prepareTurnReferenceResponse(t *testing.T, handler http.Handler) map[string]any {
	return prepareTurnReferenceResponseWithSystem(t, handler, "The gate is important to this scene.")
}

func prepareTurnReferenceResponseWithSystem(t *testing.T, handler http.Handler, systemText string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"turn_index":      2,
		"raw_user_input":  "Open the gate",
		"messages": []map[string]any{
			{"role": "system", "content": systemText},
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

func anyStringSliceContains(value any, needle string) bool {
	items, _ := value.([]any)
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
