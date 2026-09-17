package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type referenceSourcePipelineStore struct {
	*referenceBindingHTTPStore
	jobMu sync.Mutex
	jobs  map[string]*store.SourceDiscoveryJob
}

func newReferenceSourcePipelineStore() *referenceSourcePipelineStore {
	return &referenceSourcePipelineStore{
		referenceBindingHTTPStore: newReferenceBindingHTTPStore(),
		jobs:                      map[string]*store.SourceDiscoveryJob{},
	}
}

func (f *referenceSourcePipelineStore) SaveSourceDiscoveryJob(_ context.Context, input store.SourceDiscoveryInput, state string, result, coverage map[string]any) (*store.SourceDiscoveryJob, error) {
	f.jobMu.Lock()
	defer f.jobMu.Unlock()
	rawInput, _ := json.Marshal(input)
	inputMap := map[string]any{}
	_ = json.Unmarshal(rawInput, &inputMap)
	now := time.Now().UTC()
	item := &store.SourceDiscoveryJob{
		Contract: store.SourceDiscoveryContract, JobID: "source-job-integration",
		State: state, Input: inputMap, Result: result, CoverageReport: coverage,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	f.jobs[item.JobID] = item
	return cloneSourceDiscoveryJobForTest(item), nil
}

func (f *referenceSourcePipelineStore) GetSourceDiscoveryJob(_ context.Context, jobID string) (*store.SourceDiscoveryJob, error) {
	f.jobMu.Lock()
	defer f.jobMu.Unlock()
	item := f.jobs[jobID]
	if item == nil {
		return nil, store.ErrNotFound
	}
	return cloneSourceDiscoveryJobForTest(item), nil
}

func (f *referenceSourcePipelineStore) UpdateSourceDiscoveryJob(_ context.Context, jobID, state string, result, coverage map[string]any) (*store.SourceDiscoveryJob, error) {
	f.jobMu.Lock()
	defer f.jobMu.Unlock()
	item := f.jobs[jobID]
	if item == nil {
		return nil, store.ErrNotFound
	}
	item.State = state
	item.Result = result
	item.CoverageReport = coverage
	item.Revision++
	item.UpdatedAt = time.Now().UTC()
	return cloneSourceDiscoveryJobForTest(item), nil
}

func (f *referenceSourcePipelineStore) FindLatestSourceDiscoveryJob(_ context.Context, workID, continuityID string) (*store.SourceDiscoveryJob, error) {
	f.jobMu.Lock()
	defer f.jobMu.Unlock()
	for _, item := range f.jobs {
		if (workID == "" || stringFromMap(item.Input, "work_id") == workID) &&
			(continuityID == "" || stringFromMap(item.Input, "continuity_id") == continuityID) {
			return cloneSourceDiscoveryJobForTest(item), nil
		}
	}
	return nil, store.ErrNotFound
}

func cloneSourceDiscoveryJobForTest(item *store.SourceDiscoveryJob) *store.SourceDiscoveryJob {
	if item == nil {
		return nil
	}
	copy := *item
	return &copy
}

func (f *referenceSourcePipelineStore) UpsertReferenceTimelineNode(_ context.Context, item *store.ReferenceTimelineNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range f.timeline {
		if f.timeline[index].WorkID == item.WorkID && f.timeline[index].NodeID == item.NodeID {
			f.timeline[index] = *item
			return nil
		}
	}
	f.timeline = append(f.timeline, *item)
	return nil
}

func (f *referenceSourcePipelineStore) UpsertReferenceEntity(_ context.Context, item *store.ReferenceEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range f.entities {
		if f.entities[index].WorkID == item.WorkID && f.entities[index].EntityID == item.EntityID {
			f.entities[index] = *item
			return nil
		}
	}
	f.entities = append(f.entities, *item)
	return nil
}

func (f *referenceSourcePipelineStore) UpsertReferenceClaim(_ context.Context, item *store.ReferenceClaim) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := range f.claims {
		if f.claims[index].WorkID == item.WorkID && f.claims[index].ClaimID == item.ClaimID {
			f.claims[index] = *item
			return nil
		}
	}
	f.claims = append(f.claims, *item)
	return nil
}

func TestReferenceSourceDiscoveryPendingApprovalVectorAndPrepareTurnIntegration(t *testing.T) {
	fake := newReferenceSourcePipelineStore()
	vectorStore := &referenceVectorTestStore{}
	embeddingServer, embeddingCalls := referenceVectorEmbeddingServer(t)
	defer embeddingServer.Close()

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://chroma.test.invalid"
	srv := &Server{
		Cfg: cfg, Store: fake, ReferenceVector: vectorStore,
		AdminJobs: newAdminJobManager(),
	}
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.EmbeddingProvider = "openai"
	srv.RuntimeConfig.EmbeddingAPIKey = "embedding-test-key"
	srv.RuntimeConfig.EmbeddingEndpoint = embeddingServer.URL
	srv.RuntimeConfig.EmbeddingModel = "embed-reference"
	srv.RuntimeConfig.EmbeddingTimeoutSec = 5
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	created := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works", map[string]any{
		"title": "Archive Gate Chronicle", "work_type": "novel", "default_language": "en",
	})
	workID := stringFromMap(mapFromAny(created["work"]), "work_id")
	continuity := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/"+workID+"/continuities", map[string]any{
		"continuity_key": "main", "label": "Main continuity",
	})
	continuityID := stringFromMap(mapFromAny(continuity["continuity"]), "continuity_id")
	if workID == "" || continuityID == "" {
		t.Fatalf("selected reference scope was not created: work=%#v continuity=%#v", created, continuity)
	}

	preview := referenceLibraryTestRequest(t, mux, http.MethodPost, "/source-discovery/preview/v1", map[string]any{
		"work_id": workID, "continuity_id": continuityID,
		"work_query": "wrong caller title", "allowed_source_types": []string{"community_wiki"},
	})
	if strings.Contains(fmt.Sprint(preview["query_frontier"]), "wrong caller title") ||
		!strings.Contains(fmt.Sprint(preview["query_frontier"]), "Archive Gate Chronicle") {
		t.Fatalf("source discovery did not resolve the selected work identity: %#v", preview)
	}

	searchCalls := 0
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchCalls++
		if r.URL.Query().Get("q") != "Archive Gate Chronicle" {
			t.Errorf("unexpected search query: %q", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode([]store.SourceDiscoverySource{
			{URL: "https://canon-one.example/wiki/archive-gate", SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true},
			{URL: "https://canon-two.example/wiki/archive-gate", SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true},
		})
	}))
	defer searchServer.Close()

	extractionCalls := 0
	extractionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractionCalls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode extraction request: %v", err)
		}
		if !strings.Contains(fmt.Sprint(request["messages"]), `"work_title":"Archive Gate Chronicle"`) {
			t.Errorf("selected work identity did not reach extraction provider: %#v", request["messages"])
		}
		record := func(sourceRef string) map[string]any {
			return map[string]any{
				"records": map[string]any{
					"entities":    []any{},
					"world_rules": []any{},
					"timeline_events": []any{map[string]any{
						"label": "The archive gate opens", "branch": "main", "source_ref": sourceRef,
						"work_identity_status": "matched", "canon_scope_status": "in_world",
					}},
					"relations": []any{},
					"facts": []any{
						map[string]any{
							"statement": "The archive gate opens only at night.", "branch": "main", "source_ref": sourceRef,
							"work_identity_status": "matched", "canon_scope_status": "in_world",
						},
						map[string]any{
							"statement": "The west gate exists only in the alternate branch.", "branch": "alternate", "source_ref": sourceRef,
							"work_identity_status": "matched", "canon_scope_status": "in_world",
						},
					},
				},
				"follow_up_queries": []any{}, "examined_source_refs": []any{"s1", "s2"}, "incomplete_source_refs": []any{},
			}
		}
		first := record("s1")
		second := record("s2")
		firstRecords := mapFromAny(first["records"])
		secondRecords := mapFromAny(second["records"])
		firstRecords["timeline_events"] = append(sliceFromAny(firstRecords["timeline_events"]), sliceFromAny(secondRecords["timeline_events"])...)
		firstRecords["facts"] = append(sliceFromAny(firstRecords["facts"]), sliceFromAny(secondRecords["facts"])...)
		encoded, _ := json.Marshal(first)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}},
		})
	}))
	defer extractionServer.Close()

	searchBoundary := func(ctx context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchServer.URL+"?q="+url.QueryEscape(input.WorkQuery), nil)
		if err != nil {
			return nil, nil, err
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer response.Body.Close()
		var sources []store.SourceDiscoverySource
		if err := json.NewDecoder(response.Body).Decode(&sources); err != nil {
			return nil, nil, err
		}
		return sources, map[string]any{"status": "completed", "provider": "httptest-search", "result_count": len(sources), "approved_source_count": len(sources)}, nil
	}
	fetchBoundary := func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
		index := 1
		if strings.Contains(source.URL, "canon-two") {
			index = 2
		}
		excerpt := fmt.Sprintf("Independent source %d records the archive gate, its night rule, and the alternate west gate.", index)
		hash := fmt.Sprintf("independent-hash-%d", index)
		return map[string]any{
				"requested_url": source.URL, "final_url": source.URL, "source_type": source.SourceType,
				"document_sha256": hash, "_raw_body": excerpt, "media_type": "text/plain",
			}, []map[string]any{{
				"locator": map[string]any{"type": "paragraph", "value": "1"}, "excerpt": excerpt,
			}}, nil
	}
	input := store.SourceDiscoveryInput{
		WorkID: workID, ContinuityID: continuityID,
		WorkQuery: "Archive Gate Chronicle", WorkTitle: "Archive Gate Chronicle", WorkType: "novel",
		AllowedSourceTypes: []string{"community_wiki"},
	}
	resolvedInput, result, coverage, state := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "extraction-test-key", Endpoint: extractionServer.URL,
		Model: "extract-reference", TimeoutMs: 5000, MaxTokens: 2000, RetryBudget: newLLMRetryBudget(0),
	}, input, sourceDiscoveryPipelineDeps{
		search: searchBoundary, fetch: fetchBoundary, extract: runSourceCandidateExtraction,
	})
	if searchCalls != 1 || extractionCalls != 1 {
		t.Fatalf("external boundaries were not traversed exactly once: search=%d extraction=%d", searchCalls, extractionCalls)
	}
	candidates := sliceMapFromAny(result["discovered_candidates"])
	if len(candidates) != 3 {
		t.Fatalf("discovered candidates=%#v, want event and two claims", candidates)
	}
	for _, candidate := range candidates {
		if int64FromMap(candidate, "independent_source_count", 0) != 2 || candidate["review_state"] != "pending" {
			t.Fatalf("candidate was not reconciled as pending with two independent sources: %#v", candidate)
		}
	}

	retained := takeSourceDiscoveryRetainedDocuments(result)
	job, err := fake.SaveSourceDiscoveryJob(context.Background(), resolvedInput, state, result, coverage)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := stageSourceDiscoveryResult(context.Background(), fake, job.JobID, workID, continuityID, retained, candidates)
	if err != nil {
		t.Fatal(err)
	}
	result["staging"] = map[string]any{"status": "completed", "review_status": "pending", "counts": counts}
	if _, err := fake.UpdateSourceDiscoveryJob(context.Background(), job.JobID, state, result, coverage); err != nil {
		t.Fatal(err)
	}

	pending := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/"+workID+"/review-candidates?continuity_id="+continuityID, nil)
	if pending["count"] != float64(len(candidates)) || len(fake.reviews) != 0 || len(vectorStore.docs) != 0 {
		t.Fatalf("source discovery auto-approved or indexed pending candidates: pending=%#v reviews=%#v vectors=%#v", pending, fake.reviews, vectorStore.docs)
	}
	jobView := referenceLibraryTestRequest(t, mux, http.MethodGet, "/source-discovery/jobs/"+job.JobID+"/v1", nil)
	if mapFromAny(mapFromAny(jobView["result"])["admission_preview"])["requires_explicit_admission"] != true {
		t.Fatalf("source job lost explicit approval requirement: %#v", jobView)
	}

	privateClaimID := "claim-private-fixture"
	if err := fake.UpsertReferenceClaim(context.Background(), &store.ReferenceClaim{
		ClaimID: privateClaimID, WorkID: workID, ContinuityID: continuityID,
		ClaimType: "secret", ClaimText: "The sealed annex belongs to the absent keeper.",
		TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "character_private",
		KnowerEntityIDs: []string{"entity-absent-keeper"}, ReviewStatus: "approved",
	}); err != nil {
		t.Fatal(err)
	}

	rejected := performReferencePipelineRequest(t, mux, http.MethodPost, "/source-discovery/jobs/"+job.JobID+"/admit/v1", map[string]any{
		"work_id": workID, "continuity_id": continuityID,
	})
	if rejected.Code != http.StatusUnprocessableEntity || !strings.Contains(rejected.Body.String(), "source_discovery_admission_confirmation_required") {
		t.Fatalf("admission without explicit confirmation was accepted: status=%d body=%s", rejected.Code, rejected.Body.String())
	}
	admitted := referenceLibraryTestRequest(t, mux, http.MethodPost, "/source-discovery/jobs/"+job.JobID+"/admit/v1", map[string]any{
		"work_id": workID, "continuity_id": continuityID, "confirm_evidence_validated_batch": true,
	})
	if admitted["status"] != "admitted" || admitted["review_source"] != "evidence_validated_batch" || admitted["index_status"] != "current" {
		t.Fatalf("explicit admission did not approve and index the batch: %#v", admitted)
	}
	if len(fake.reviews) != len(candidates) || *embeddingCalls != len(vectorStore.docs) || len(vectorStore.docs) != len(candidates)+1 {
		t.Fatalf("approval/index counts diverged: reviews=%#v embedding_calls=%d docs=%#v", fake.reviews, *embeddingCalls, vectorStore.docs)
	}

	currentNodeID := referenceStableID("source-discovery-timeline", workID, continuityID, "main", normalizeSourceCandidateValue("The archive gate opens"))
	safeClaimID := referenceStableID("source-discovery-claim", workID, continuityID, "claim", normalizeSourceCandidateValue("The archive gate opens only at night."))
	alternateClaimID := referenceStableID("source-discovery-claim", workID, continuityID, "claim", normalizeSourceCandidateValue("The west gate exists only in the alternate branch."))
	vectorStore.exactResults = []vector.ExactQueryResult{
		{Document: referencePipelineVectorDocument(workID, continuityID, "claim", privateClaimID), ChromaRank: 1, CosineSimilarity: 0.98, CosineAvailable: true},
		{Document: referencePipelineVectorDocument(workID, continuityID, "claim", alternateClaimID), ChromaRank: 2, CosineSimilarity: 0.95, CosineAvailable: true},
		{Document: referencePipelineVectorDocument(workID, continuityID, "claim", safeClaimID), ChromaRank: 3, CosineSimilarity: 0.90, CosineAvailable: true},
	}
	searchResult := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/"+workID+"/vector/search", map[string]any{
		"continuity_id": continuityID, "query": "archive gate", "limit": 3,
	})
	if len(sliceFromAny(searchResult["results"])) != 3 {
		t.Fatalf("actual vector search route did not return exact Chroma candidates: %#v", searchResult)
	}

	binding := referenceLibraryTestRequest(t, mux, http.MethodPost, "/sessions/reference-pipeline-session/reference-bindings", map[string]any{
		"work_id": workID, "continuity_id": continuityID,
		"binding_role": "primary", "reference_mode": "primary", "anchor_mode": "manual",
		"current_node_id": currentNodeID, "reveal_ceiling_node_id": currentNodeID,
		"future_policy": "block", "expected_revision": 0,
	})
	if binding["action"] != "create" {
		t.Fatalf("reference binding was not created: %#v", binding)
	}

	prepare := referenceLibraryTestRequest(t, mux, http.MethodPost, "/prepare-turn", map[string]any{
		"chat_session_id": "reference-pipeline-session", "turn_index": 1,
		"raw_user_input": "What rule governs the archive gate?",
		"messages":       []map[string]any{{"role": "user", "content": "What rule governs the archive gate?"}},
		"settings": map[string]any{
			"injection_enabled": true, "reference_injection_enabled": true,
			"max_injection_chars": 1800, "reference_injection_budget_basis_chars": 1800,
			"reference_recall_limit": 3, "top_k": 0,
		},
	})
	injection := fmt.Sprint(prepare["injection_text"])
	if !strings.Contains(injection, "The archive gate opens only at night.") {
		t.Fatalf("approved source-discovery claim did not reach prepare-turn: %q", injection)
	}
	for _, forbidden := range []string{"The west gate exists only in the alternate branch.", "The sealed annex belongs to the absent keeper."} {
		if strings.Contains(injection, forbidden) {
			t.Fatalf("branch/private reference leaked through prepare-turn: %q", injection)
		}
	}
	recall := mapFromAny(prepare["reference_recall"])
	excludedReasons := map[string]bool{}
	for _, raw := range sliceMapFromAny(recall["excluded"]) {
		excludedReasons[stringFromMap(raw, "reason")] = true
	}
	if !excludedReasons["branch_mismatch"] || !excludedReasons["knowledge_scope_not_in_scene"] {
		t.Fatalf("prepare-turn did not expose branch/privacy exclusions: %#v", recall["excluded"])
	}
}

func performReferencePipelineRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func referencePipelineVectorDocument(workID, continuityID, kind, sourceID string) vector.VectorDocument {
	return vector.VectorDocument{
		ID: referenceVectorDocumentID(kind, sourceID), ChatSessionID: workID,
		DocumentText: sourceID,
		Metadata: map[string]any{
			"work_id": workID, "continuity_id": continuityID,
			"review_status": "approved", "reference_kind": kind, "source_id": sourceID,
			"embedding_provider": "openai", "embedding_model": "embed-reference",
		},
	}
}
