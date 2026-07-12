package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type referenceLibraryHTTPStore struct {
	store.Store
	store.ReferenceLibraryStore
	mu           sync.Mutex
	works        []store.ReferenceWork
	continuities []store.ReferenceContinuity
	documents    []store.ReferenceDocument
	timeline     []store.ReferenceTimelineNode
	entities     []store.ReferenceEntity
	aliases      map[string][]store.ReferenceEntityAlias
	claims       []store.ReferenceClaim
	reviews      []referenceReviewCall
}

type referenceReviewCall struct {
	workID string
	kind   string
	id     string
	status string
	source string
	reason string
}

func newReferenceLibraryHTTPStore() *referenceLibraryHTTPStore {
	return &referenceLibraryHTTPStore{Store: store.NewNoopStore(), aliases: map[string][]store.ReferenceEntityAlias{}}
}

func (f *referenceLibraryHTTPStore) CreateReferenceWork(_ context.Context, item *store.ReferenceWork) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.works = append(f.works, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceWorks(_ context.Context, _ string, _ int) ([]store.ReferenceWork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.ReferenceWork(nil), f.works...), nil
}

func (f *referenceLibraryHTTPStore) UpsertReferenceContinuity(_ context.Context, item *store.ReferenceContinuity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.continuities = append(f.continuities, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceContinuities(_ context.Context, workID string) ([]store.ReferenceContinuity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceContinuity{}
	for _, item := range f.continuities {
		if item.WorkID == workID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *referenceLibraryHTTPStore) SaveReferenceDocument(_ context.Context, item *store.ReferenceDocument) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.documents = append(f.documents, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) GetReferenceDocument(_ context.Context, id string) (*store.ReferenceDocument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, item := range f.documents {
		if item.DocumentID == id {
			copy := item
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *referenceLibraryHTTPStore) UpdateReferenceDocumentStatus(_ context.Context, id, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.documents {
		if f.documents[i].DocumentID == id {
			f.documents[i].ImportStatus = status
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *referenceLibraryHTTPStore) UpsertReferenceTimelineNode(_ context.Context, item *store.ReferenceTimelineNode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.timeline = append(f.timeline, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceTimelineNodes(_ context.Context, workID, continuityID, _ string) ([]store.ReferenceTimelineNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceTimelineNode{}
	for _, item := range f.timeline {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *referenceLibraryHTTPStore) UpsertReferenceEntity(_ context.Context, item *store.ReferenceEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entities = append(f.entities, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceEntities(_ context.Context, workID, continuityID, _ string) ([]store.ReferenceEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceEntity{}
	for _, item := range f.entities {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *referenceLibraryHTTPStore) UpsertReferenceEntityAlias(_ context.Context, item *store.ReferenceEntityAlias) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aliases[item.EntityID] = append(f.aliases[item.EntityID], *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceEntityAliases(_ context.Context, entityID string) ([]store.ReferenceEntityAlias, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.ReferenceEntityAlias(nil), f.aliases[entityID]...), nil
}

func (f *referenceLibraryHTTPStore) UpsertReferenceClaim(_ context.Context, item *store.ReferenceClaim) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims = append(f.claims, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceClaims(_ context.Context, workID, continuityID, reviewStatus, _ string) ([]store.ReferenceClaim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceClaim{}
	for _, item := range f.claims {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) && (reviewStatus == "" || item.ReviewStatus == reviewStatus) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *referenceLibraryHTTPStore) ReplaceReferenceClaimKnowers(context.Context, string, []string) error {
	return nil
}

func (f *referenceLibraryHTTPStore) UpdateReferenceCandidateReview(_ context.Context, workID, kind, id, status, source, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reviews = append(f.reviews, referenceReviewCall{workID: workID, kind: kind, id: id, status: status, source: source, reason: reason})
	now := time.Now().UTC()
	switch kind {
	case "timeline":
		for i := range f.timeline {
			if f.timeline[i].WorkID == workID && f.timeline[i].NodeID == id {
				f.timeline[i].ReviewStatus = status
				f.timeline[i].ReviewSource = source
				f.timeline[i].ReviewReason = reason
				f.timeline[i].ReviewedAt = &now
			}
		}
	case "entity":
		for i := range f.entities {
			if f.entities[i].WorkID == workID && f.entities[i].EntityID == id {
				f.entities[i].ReviewStatus = status
				f.entities[i].ReviewSource = source
				f.entities[i].ReviewReason = reason
				f.entities[i].ReviewedAt = &now
			}
		}
	case "claim":
		for i := range f.claims {
			if f.claims[i].WorkID == workID && f.claims[i].ClaimID == id {
				f.claims[i].ReviewStatus = status
				f.claims[i].ReviewSource = source
				f.claims[i].ReviewReason = reason
				f.claims[i].ReviewedAt = &now
			}
		}
	}
	return nil
}

func referenceLibraryTestRequest(t *testing.T, mux http.Handler, method, path string, body any) map[string]any {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s returned %d: %s", method, path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReferenceLibraryFileToReviewRoutes(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	created := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works", map[string]any{"title": "Example", "work_type": "novel"})
	work := created["work"].(map[string]any)
	workID := work["work_id"].(string)
	if workID == "" {
		t.Fatal("work id was empty")
	}
	continuity := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/"+workID+"/continuities", map[string]any{"continuity_key": "main", "label": "Main"})
	continuityID := continuity["continuity"].(map[string]any)["continuity_id"].(string)
	document := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/"+workID+"/documents", map[string]any{"continuity_id": continuityID, "filename": "source.txt", "content": "A meets B."})
	if document["document"].(map[string]any)["import_status"] != "pending" {
		t.Fatalf("document was not pending: %#v", document)
	}

	fake.timeline = append(fake.timeline, store.ReferenceTimelineNode{NodeID: "node-1", WorkID: workID, ContinuityID: continuityID, NodeKey: "start", Label: "Start", ReviewStatus: "pending"})
	candidates := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/"+workID+"/review-candidates?continuity_id="+continuityID, nil)
	if candidates["count"].(float64) != 1 {
		t.Fatalf("unexpected candidate count: %#v", candidates)
	}
	referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/"+workID+"/review", map[string]any{"items": []any{map[string]any{"kind": "timeline", "id": "node-1", "decision": "approved"}}})
	if len(fake.reviews) != 1 || fake.reviews[0].workID != workID || fake.reviews[0].status != "approved" {
		t.Fatalf("review was not scoped to work: %#v", fake.reviews)
	}
}

func TestReferenceExtractionQueueReusesRunningDocumentJob(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"timeline\":[],\"entities\":[],\"claims\":[],\"warnings\":[]}"}}]}`))
	}))
	defer upstream.Close()

	fake := newReferenceLibraryHTTPStore()
	fake.documents = append(fake.documents, store.ReferenceDocument{DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1", RawText: "source"})
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{"client_meta": map[string]any{"critic": map[string]any{"provider": "openai", "api_key": "test", "endpoint": upstream.URL, "model": "test", "timeout_ms": 30000}}}
	first := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/documents/doc-1/extract", body)
	second := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/documents/doc-1/extract", body)
	if request, ok := first["request"].(map[string]any); !ok || request["auto_review"] != true {
		close(release)
		t.Fatalf("reference extraction did not default to automatic review: %#v", first)
	}
	if first["job_id"] != second["job_id"] || second["reused_running_job"] != true {
		close(release)
		t.Fatalf("running job was not reused: first=%#v second=%#v", first, second)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := srv.AdminJobs.get(first["job_id"].(string))
		if ok && job["status"] == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reference extraction job did not complete")
}

func TestReferenceExtractionLinksExistingAliasesAndTimeline(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.timeline = append(fake.timeline, store.ReferenceTimelineNode{NodeID: "start-id", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "start", ReviewStatus: "approved"})
	fake.entities = append(fake.entities, store.ReferenceEntity{EntityID: "alice-id", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Alice", ReviewStatus: "approved"})
	fake.aliases["alice-id"] = []store.ReferenceEntityAlias{{EntityID: "alice-id", AliasText: "A"}}
	doc := &store.ReferenceDocument{DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1"}
	parsed := map[string]any{
		"timeline": []any{map[string]any{"node_key": "after", "label": "After", "parent_node_key": "start"}},
		"claims":   []any{map[string]any{"claim_type": "character", "subject": "A", "claim_text": "A returns.", "temporal_scope": "bounded", "valid_from_node_key": "start"}},
	}
	counts, _, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, parsed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if counts["timeline"] != 1 || counts["claims"] != 1 {
		t.Fatalf("unexpected extraction counts: %#v", counts)
	}
	if got := fake.timeline[len(fake.timeline)-1].ParentNodeID; got != "start-id" {
		t.Fatalf("parent timeline was not linked: %q", got)
	}
	if got := fake.claims[len(fake.claims)-1]; got.SubjectEntityID != "alice-id" || got.ValidFromNodeID != "start-id" {
		t.Fatalf("claim links were not resolved: %#v", got)
	}
}

func TestReferenceAutoReviewApprovesSupportedAndLeavesAmbiguousPending(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"decisions\":[{\"kind\":\"entity\",\"id\":\"entity-supported\",\"decision\":\"approved\",\"reason\":\"direct evidence\"},{\"kind\":\"entity\",\"id\":\"entity-ambiguous\",\"decision\":\"pending\",\"reason\":\"heading may not be an organization\"}]}"}}]}`))
	}))
	defer upstream.Close()

	fake := newReferenceLibraryHTTPStore()
	fake.entities = append(fake.entities,
		store.ReferenceEntity{EntityID: "entity-supported", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "HUNTR/X", EntityType: "faction", ReviewStatus: "pending", MetadataJSON: `{"evidence_excerpt":"direct source sentence"}`},
		store.ReferenceEntity{EntityID: "entity-ambiguous", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "1930s Hunters", EntityType: "faction", ReviewStatus: "pending", MetadataJSON: `{"evidence_excerpt":"1930s heading"}`},
	)
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"continuity_id": "continuity-1",
		"client_meta":   map[string]any{"critic": map[string]any{"provider": "openai", "api_key": "test", "endpoint": upstream.URL, "model": "test", "timeout_ms": 30000}},
	}
	started := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/review/auto", body)
	jobID := started["job_id"].(string)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := srv.AdminJobs.get(jobID)
		if ok && job["status"] == "completed" {
			if fake.entities[0].ReviewStatus != "approved" || fake.entities[1].ReviewStatus != "pending" {
				t.Fatalf("unexpected review statuses: %#v", fake.entities)
			}
			if fake.entities[0].ReviewSource != "critic_auto" || fake.entities[0].ReviewReason != "direct evidence" || fake.entities[0].ReviewedAt == nil {
				t.Fatalf("automatic review audit was not recorded: %#v", fake.entities[0])
			}
			if fake.entities[1].ReviewSource != "critic_auto" || fake.entities[1].ReviewReason == "" || fake.entities[1].ReviewedAt == nil {
				t.Fatalf("pending review reason was not recorded: %#v", fake.entities[1])
			}
			result := job["result"].(map[string]any)
			if intFromAny(result["approved"], 0) != 1 || intFromAny(result["remaining_pending"], 0) != 1 {
				t.Fatalf("unexpected auto review result: %#v", result)
			}
			listed := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/review-candidates?continuity_id=continuity-1&review_status=all", nil)
			summary := listed["summary"].(map[string]any)
			if intFromAny(summary["approved"], 0) != 1 || intFromAny(summary["pending"], 0) != 1 || intFromAny(summary["total"], 0) != 2 {
				t.Fatalf("unexpected review summary: %#v", summary)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reference auto review job did not complete")
}
