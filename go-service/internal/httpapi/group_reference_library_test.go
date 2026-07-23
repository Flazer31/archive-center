package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type referenceLibraryHTTPStore struct {
	store.Store
	store.ReferenceLibraryStore
	mu            sync.Mutex
	works         []store.ReferenceWork
	continuities  []store.ReferenceContinuity
	documents     []store.ReferenceDocument
	timeline      []store.ReferenceTimelineNode
	entities      []store.ReferenceEntity
	aliases       map[string][]store.ReferenceEntityAlias
	claims        []store.ReferenceClaim
	reviews       []referenceReviewCall
	deleteWorkErr error
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

func TestReferenceDocumentSourceViewsExposeLocalRetentionWithoutRawBody(t *testing.T) {
	views := referenceDocumentSourceViews([]store.ReferenceDocument{{
		DocumentID: "doc-1", SourceURI: "https://reference.example/wiki/page",
		ContentHash: "hash-1", RawRetention: "full", RawText: `<html><head><title>Archive Guide</title></head><body><h1>Fallback Heading</h1></body></html>`, ImportStatus: "pending",
	}})
	if len(views) != 1 || views[0]["raw_retention"] != "full" || views[0]["raw_text_length"] != len([]rune(`<html><head><title>Archive Guide</title></head><body><h1>Fallback Heading</h1></body></html>`)) {
		t.Fatalf("views=%#v", views)
	}
	if views[0]["document_title"] != "Archive Guide" {
		t.Fatalf("document_title=%#v", views[0]["document_title"])
	}
	if _, exposed := views[0]["raw_text"]; exposed {
		t.Fatalf("raw source body must remain in the local DB, not the library list response: %#v", views[0])
	}
}

func TestReferenceDocumentTitlePrefersProvenanceAndFallsBackToReadableURL(t *testing.T) {
	items := []store.ReferenceDocument{
		{ProvenanceJSON: `{"document_title":"  Stored   Document Title "}`, SourceURI: "https://reference.example/wiki/ignored"},
		{SourceURI: "https://reference.example/wiki/Readable_Page_Name"},
		{SourceURI: "https://reference.example/"},
	}
	want := []string{"Stored Document Title", "Readable Page Name", "reference.example"}
	for index, item := range items {
		if got := referenceDocumentTitle(item); got != want[index] {
			t.Fatalf("index %d: got %q want %q", index, got, want[index])
		}
	}
}

func (f *referenceLibraryHTTPStore) CreateReferenceWork(_ context.Context, item *store.ReferenceWork) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.works = append(f.works, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) GetReferenceWork(_ context.Context, workID string) (*store.ReferenceWork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, item := range f.works {
		if item.WorkID == workID {
			copy := item
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *referenceLibraryHTTPStore) ListReferenceWorks(_ context.Context, _ string, _ int) ([]store.ReferenceWork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.ReferenceWork(nil), f.works...), nil
}

func (f *referenceLibraryHTTPStore) UpdateReferenceWork(_ context.Context, item *store.ReferenceWork, expectedRevision int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.works {
		if f.works[i].WorkID != item.WorkID {
			continue
		}
		if f.works[i].Revision != expectedRevision {
			return store.ErrReferenceConflict
		}
		item.Revision = expectedRevision + 1
		f.works[i] = *item
		return nil
	}
	return store.ErrNotFound
}

func (f *referenceLibraryHTTPStore) DeleteReferenceWork(_ context.Context, workID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteWorkErr != nil {
		return f.deleteWorkErr
	}
	for i := range f.works {
		if f.works[i].WorkID == workID {
			f.works = append(f.works[:i], f.works[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
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

func (f *referenceLibraryHTTPStore) ListReferenceDocuments(_ context.Context, workID, continuityID, _ string) ([]store.ReferenceDocument, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceDocument{}
	for _, item := range f.documents {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) {
			out = append(out, item)
		}
	}
	return out, nil
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

func (f *referenceLibraryHTTPStore) ListReferenceTimelineNodes(_ context.Context, workID, continuityID, reviewStatus string) ([]store.ReferenceTimelineNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceTimelineNode{}
	for _, item := range f.timeline {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) && (reviewStatus == "" || item.ReviewStatus == reviewStatus) {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ordinal == out[j].Ordinal {
			return out[i].NodeKey < out[j].NodeKey
		}
		return out[i].Ordinal < out[j].Ordinal
	})
	return out, nil
}

func (f *referenceLibraryHTTPStore) NormalizeReferenceTimelineOrder(_ context.Context, workID, continuityID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	indexes := []int{}
	for i := range f.timeline {
		if f.timeline[i].WorkID == workID && f.timeline[i].ContinuityID == continuityID && f.timeline[i].ReviewStatus == "approved" {
			indexes = append(indexes, i)
		}
	}
	for order, index := range indexes {
		f.timeline[index].Ordinal = int64((order + 1) * 10)
	}
	return len(indexes), nil
}

func (f *referenceLibraryHTTPStore) ApplyReferenceTimelineOrder(_ context.Context, workID, continuityID string, orderedIDs []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	indexes := map[string]int{}
	for i := range f.timeline {
		if f.timeline[i].WorkID == workID && f.timeline[i].ContinuityID == continuityID && f.timeline[i].ReviewStatus == "approved" {
			indexes[f.timeline[i].NodeID] = i
		}
	}
	if len(indexes) != len(orderedIDs) {
		return 0, store.ErrReferenceConflict
	}
	for order, id := range orderedIDs {
		index, ok := indexes[id]
		if !ok {
			return 0, store.ErrReferenceConflict
		}
		f.timeline[index].Ordinal = int64((order + 1) * 10)
	}
	return len(orderedIDs), nil
}

func (f *referenceLibraryHTTPStore) UpsertReferenceEntity(_ context.Context, item *store.ReferenceEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entities = append(f.entities, *item)
	return nil
}

func (f *referenceLibraryHTTPStore) ListReferenceEntities(_ context.Context, workID, continuityID, reviewStatus string) ([]store.ReferenceEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceEntity{}
	for _, item := range f.entities {
		if item.WorkID == workID && (continuityID == "" || item.ContinuityID == continuityID) && (reviewStatus == "" || item.ReviewStatus == reviewStatus) {
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

func TestReferenceMetadataStringDoesNotTurnMissingValueIntoNilText(t *testing.T) {
	if got := referenceMetadataString(`{"origin_kind":"source_discovery"}`, "fact_key"); got != "" {
		t.Fatalf("missing fact_key=%q", got)
	}
	if got := referenceMetadataString(`{"fact_key":null}`, "fact_key"); got != "" {
		t.Fatalf("null fact_key=%q", got)
	}
	if got := referenceMetadataString(`{"fact_key":"character.role"}`, "fact_key"); got != "character.role" {
		t.Fatalf("fact_key=%q", got)
	}
}

func (f *referenceLibraryHTTPStore) UpdateReferenceLibraryItem(_ context.Context, item *store.ReferenceLibraryItemUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now().UTC()
	switch item.Kind {
	case "timeline":
		for i := range f.timeline {
			if f.timeline[i].WorkID == item.WorkID && f.timeline[i].NodeID == item.ID {
				f.timeline[i].NodeKey, f.timeline[i].Label = item.NodeKey, item.Label
				f.timeline[i].Ordinal, f.timeline[i].NodeKind, f.timeline[i].BranchKey = item.Ordinal, item.NodeKind, item.BranchKey
				f.timeline[i].ReviewStatus, f.timeline[i].ReviewSource, f.timeline[i].ReviewedAt = "approved", "user_edit", &now
				return nil
			}
		}
	case "entity":
		for i := range f.entities {
			if f.entities[i].WorkID == item.WorkID && f.entities[i].EntityID == item.ID {
				f.entities[i].EntityType, f.entities[i].CanonicalName, f.entities[i].DescriptionText = item.EntityType, item.CanonicalName, item.DescriptionText
				f.entities[i].ReviewStatus, f.entities[i].ReviewSource, f.entities[i].ReviewedAt = "approved", "user_edit", &now
				return nil
			}
		}
	case "claim":
		for i := range f.claims {
			if f.claims[i].WorkID == item.WorkID && f.claims[i].ClaimID == item.ID {
				f.claims[i].ClaimType, f.claims[i].ClaimText, f.claims[i].EvidenceExcerpt = item.ClaimType, item.ClaimText, item.EvidenceExcerpt
				f.claims[i].TemporalScope, f.claims[i].KnowledgeScope, f.claims[i].Confidence = item.TemporalScope, item.KnowledgeScope, item.Confidence
				f.claims[i].ReviewStatus, f.claims[i].ReviewSource, f.claims[i].ReviewedAt = "approved", "user_edit", &now
				return nil
			}
		}
	}
	return store.ErrNotFound
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

func TestReferenceWorkUpdateAndDeleteRoutes(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.works = append(fake.works, store.ReferenceWork{
		WorkID: "work-1", Title: "Old title", WorkType: "novel", DefaultLanguage: "ko", Status: "draft", Revision: 3,
	})
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	updated := referenceLibraryTestRequest(t, mux, http.MethodPatch, "/reference-works/work-1", map[string]any{
		"title": "New title", "work_type": "comic", "default_language": "ja", "expected_revision": 3,
	})
	work := updated["work"].(map[string]any)
	if work["title"] != "New title" || work["work_type"] != "comic" || work["default_language"] != "ja" || intFromAny(work["revision"], 0) != 4 {
		t.Fatalf("unexpected updated work: %#v", work)
	}

	deleted := referenceLibraryTestRequest(t, mux, http.MethodDelete, "/reference-works/work-1", nil)
	if deleted["deleted"] != true || len(fake.works) != 0 {
		t.Fatalf("work was not deleted: response=%#v works=%#v", deleted, fake.works)
	}
}

func TestReferenceWorkDeleteRoutePreservesDependencyGuard(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.works = append(fake.works, store.ReferenceWork{WorkID: "work-1", Title: "Linked", Revision: 1})
	fake.deleteWorkErr = store.ErrReferenceWorkInUse
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/reference-works/work-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "reference_work_in_use") || len(fake.works) != 1 {
		t.Fatalf("dependency guard response=%d %s works=%#v", rec.Code, rec.Body.String(), fake.works)
	}
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

func TestReferenceLibraryBrowseReturnsPendingItemsSeparately(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.works = append(fake.works, store.ReferenceWork{WorkID: "work-1", Title: "Neutral"})
	fake.entities = append(fake.entities, store.ReferenceEntity{EntityID: "entity-pending", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "character", CanonicalName: "Mina", ReviewStatus: "pending"})
	fake.claims = append(fake.claims, store.ReferenceClaim{ClaimID: "claim-pending", WorkID: "work-1", ContinuityID: "continuity-1", ClaimType: "event", ClaimText: "Mina arrives.", ReviewStatus: "pending"})
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	result := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	pending := result["pending"].(map[string]any)
	if result["count"] != float64(0) || pending["count"] != float64(2) || len(pending["entities"].([]any)) != 1 || len(pending["claims"].([]any)) != 1 {
		t.Fatalf("result=%#v", result)
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
	if request, ok := first["request"].(map[string]any); !ok || request["auto_review"] != false {
		close(release)
		t.Fatalf("reference extraction did not default to manual review: %#v", first)
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
			result := job["result"].(map[string]any)
			vectorIndex := result["vector_index"].(map[string]any)
			if vectorIndex["status"] != "skipped" || vectorIndex["reason"] != "manual_approval_required" {
				t.Fatalf("extraction claimed an automatic vector index: %#v", result)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reference extraction job did not complete")
}

func TestReferenceExtractorUsesEvidenceGroundedGeneralFactionRule(t *testing.T) {
	var systemPrompt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode extractor request: %v", err)
		} else if messages, ok := request["messages"].([]any); ok && len(messages) > 0 {
			message, _ := messages[0].(map[string]any)
			systemPrompt, _ = message["content"].(string)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"timeline\":[],\"entities\":[],\"claims\":[],\"warnings\":[]}"}}]}`))
	}))
	defer upstream.Close()

	_, err := callReferenceExtractor(context.Background(), completeTurnLLMConfig{
		APIKey: "test", Endpoint: upstream.URL, Model: "test", Provider: "openai", TimeoutMs: 30000,
	}, &store.ReferenceDocument{WorkID: "work-1", ContinuityID: "continuity-1", SourceURI: "source.txt"}, "A source chunk.", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(systemPrompt, "1930s hunters") || !containsAll(systemPrompt, "faction", "distinct in-world organization", "evidence_excerpt", "directly supports") {
		t.Fatalf("extractor faction rule = %q", systemPrompt)
	}
}

func TestSplitReferenceDocumentDoesNotCapChunkCount(t *testing.T) {
	raw := strings.Repeat("x", 65*16)
	chunks := splitReferenceDocument(raw, 16)
	if len(chunks) != 65 {
		t.Fatalf("chunks=%d", len(chunks))
	}
	if strings.Join(chunks, "") != raw {
		t.Fatal("chunking changed document content")
	}
}

func TestReferenceLLMCallsPreserveConfiguredGenerationValues(t *testing.T) {
	requests := []map[string]any{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, request)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer upstream.Close()
	cfg := completeTurnLLMConfig{
		APIKey: "test", Endpoint: upstream.URL, Model: "test", Provider: "openai", TimeoutMs: 30000,
		Temperature: 0.87, MaxTokens: 777, MaxCompletionTokens: 777,
	}
	if _, err := callReferenceExtractor(context.Background(), cfg, &store.ReferenceDocument{WorkID: "work-1", ContinuityID: "continuity-1", SourceURI: "source.txt"}, "A source chunk.", 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := callReferenceAutoReviewer(context.Background(), cfg, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := callReferenceTimelineChronology(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatalf("request count=%d", len(requests))
	}
	for i, request := range requests {
		if request["temperature"] != 0.87 || request["max_tokens"] != float64(777) {
			t.Fatalf("request[%d] changed configured generation values: %#v", i, request)
		}
	}
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
	counts, _, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, parsed, "", 0)
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

func TestReferenceExtractionStoresOnlyGroundedOriginalExcerpts(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	doc := &store.ReferenceDocument{DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1"}
	source := "Arin is the\nleader of Aster Unit.\nThe stage is quiet."
	parsed := map[string]any{"claims": []any{
		map[string]any{"claim_type": "character", "claim_text": "Arin leads Aster Unit.", "evidence_excerpt": "Arin is the leader of Aster Unit.", "temporal_scope": "timeless"},
		map[string]any{"claim_type": "event", "claim_text": "An invented event occurred.", "evidence_excerpt": "This sentence is not in the source.", "temporal_scope": "timeless"},
	}}
	counts, warnings, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, parsed, source, 2)
	if err != nil {
		t.Fatal(err)
	}
	if counts["claims"] != 2 || len(fake.claims) != 2 {
		t.Fatalf("claim extraction counts=%#v claims=%#v", counts, fake.claims)
	}
	if fake.claims[0].EvidenceExcerpt != "Arin is the\nleader of Aster Unit." {
		t.Fatalf("grounded excerpt did not preserve original text: %q", fake.claims[0].EvidenceExcerpt)
	}
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(fake.claims[0].MetadataJSON), &metadata); err != nil || metadata["evidence_grounded"] != true || metadata["chunk_index"] != float64(2) {
		t.Fatalf("grounded provenance missing: %#v err=%v", metadata, err)
	}
	if fake.claims[1].EvidenceExcerpt != "" || !strings.Contains(strings.Join(warnings, "\n"), "claim evidence excerpt was not found in source") {
		t.Fatalf("ungrounded excerpt was not rejected: claim=%#v warnings=%#v", fake.claims[1], warnings)
	}
}

func TestReferenceGroundedSourceExcerptRejectsParaphrase(t *testing.T) {
	if got := referenceGroundedSourceExcerpt("Bera guards the sealed gate.", "Bera protects a gate."); got != "" {
		t.Fatalf("paraphrase was accepted as original evidence: %q", got)
	}
}

func TestReferenceAutoReviewRecordsRecommendationsWithoutApprovingCandidates(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"decisions\":[{\"kind\":\"entity\",\"id\":\"entity-supported\",\"decision\":\"approved\",\"reason\":\"direct evidence\"},{\"kind\":\"entity\",\"id\":\"entity-rejected\",\"decision\":\"rejected\",\"reason\":\"production trivia\"},{\"kind\":\"entity\",\"id\":\"entity-ambiguous\",\"decision\":\"pending\",\"reason\":\"heading may not be an organization\"}]}"}}]}`))
	}))
	defer upstream.Close()

	fake := newReferenceLibraryHTTPStore()
	fake.entities = append(fake.entities,
		store.ReferenceEntity{EntityID: "entity-supported", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Aster Unit", EntityType: "faction", ReviewStatus: "pending", MetadataJSON: `{"evidence_excerpt":"direct source sentence"}`},
		store.ReferenceEntity{EntityID: "entity-rejected", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Actor Notes", EntityType: "other", ReviewStatus: "pending", MetadataJSON: `{"evidence_excerpt":"production trivia"}`},
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
			if fake.entities[0].ReviewStatus != "pending" || fake.entities[1].ReviewStatus != "pending" || fake.entities[2].ReviewStatus != "pending" {
				t.Fatalf("unexpected review statuses: %#v", fake.entities)
			}
			if fake.entities[0].ReviewSource != "machine_recommended_approved" || fake.entities[0].ReviewReason != "direct evidence" || fake.entities[0].ReviewedAt == nil {
				t.Fatalf("approval recommendation was not recorded: %#v", fake.entities[0])
			}
			if fake.entities[1].ReviewSource != "machine_recommended_rejected" || fake.entities[1].ReviewReason != "production trivia" || fake.entities[1].ReviewedAt == nil {
				t.Fatalf("rejection recommendation was not recorded: %#v", fake.entities[1])
			}
			if fake.entities[2].ReviewSource != "machine_recommended_pending" || fake.entities[2].ReviewReason == "" || fake.entities[2].ReviewedAt == nil {
				t.Fatalf("pending recommendation was not recorded: %#v", fake.entities[2])
			}
			result := job["result"].(map[string]any)
			if result["mode"] != "recommendation_only" || intFromAny(result["approved"], -1) != 0 || intFromAny(result["rejected"], -1) != 0 || intFromAny(result["recommended_approved"], 0) != 1 || intFromAny(result["recommended_rejected"], 0) != 1 || intFromAny(result["recommended_pending"], 0) != 1 || intFromAny(result["remaining_pending"], 0) != 3 {
				t.Fatalf("unexpected auto review result: %#v", result)
			}
			vectorIndex := result["vector_index"].(map[string]any)
			if vectorIndex["status"] != "skipped" || vectorIndex["reason"] != "manual_approval_required" {
				t.Fatalf("recommendation job claimed an automatic vector index: %#v", result)
			}
			listed := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/review-candidates?continuity_id=continuity-1&review_status=all", nil)
			summary := listed["summary"].(map[string]any)
			if intFromAny(summary["approved"], -1) != 0 || intFromAny(summary["rejected"], -1) != 0 || intFromAny(summary["pending"], 0) != 3 || intFromAny(summary["total"], 0) != 3 {
				t.Fatalf("unexpected review summary: %#v", summary)
			}
			library := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
			if intFromAny(library["count"], -1) != 0 || len(library["entities"].([]any)) != 0 {
				t.Fatalf("generated library exposed recommendation-only candidates: %#v", library)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reference auto review job did not complete")
}

func TestReferenceLibraryLegacyAutoReviewPreviewIsReadOnlyAndEvidenceAware(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.timeline = append(fake.timeline,
		store.ReferenceTimelineNode{NodeID: "legacy-timeline", WorkID: "work-1", ContinuityID: "continuity-1", Label: "Legacy Event", ReviewStatus: "approved", ReviewSource: "critic_auto", ReviewReason: "model approved", MetadataJSON: `{"evidence_grounded":true,"evidence_excerpt":"direct timeline evidence"}`},
	)
	fake.entities = append(fake.entities,
		store.ReferenceEntity{EntityID: "legacy-entity", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "Legacy Entity", EntityType: "character", ReviewStatus: "rejected", ReviewSource: "critic_auto", ReviewReason: "model rejected", MetadataJSON: `{"evidence_grounded":false,"evidence_excerpt":"unverified entity evidence"}`},
		store.ReferenceEntity{EntityID: "new-recommendation", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "New Recommendation", EntityType: "character", ReviewStatus: "pending", ReviewSource: "machine_recommended_approved", MetadataJSON: `{"evidence_grounded":true,"evidence_excerpt":"new evidence"}`},
		store.ReferenceEntity{EntityID: "user-approved", WorkID: "work-1", ContinuityID: "continuity-1", CanonicalName: "User Approved", EntityType: "character", ReviewStatus: "approved", ReviewSource: "manual", MetadataJSON: `{"evidence_grounded":true,"evidence_excerpt":"user evidence"}`},
	)
	fake.claims = append(fake.claims,
		store.ReferenceClaim{ClaimID: "legacy-claim", WorkID: "work-1", ContinuityID: "continuity-1", ClaimText: "Legacy claim without evidence", ClaimType: "event", ReviewStatus: "pending", ReviewSource: "critic_auto", ReviewReason: "model pending", MetadataJSON: `{}`},
	)
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	result := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	preview := result["legacy_auto_review_preview"].(map[string]any)
	if preview["contract_version"] != referenceLegacyAutoReviewPreviewContractVersion || preview["mode"] != "read_only" || preview["mutation_allowed"] != false || preview["truncated"] != false {
		t.Fatalf("legacy preview contract = %#v", preview)
	}
	summary := preview["summary"].(map[string]any)
	for key, want := range map[string]int{
		"total": 3, "approved": 1, "rejected": 1, "pending": 1,
		"potentially_active": 1, "evidence_grounded": 1,
		"evidence_present_unverified": 1, "evidence_missing": 1,
		"timeline": 1, "entity": 1, "claim": 1,
	} {
		if got := intFromAny(summary[key], -1); got != want {
			t.Fatalf("legacy preview summary %s=%d want=%d: %#v", key, got, want, summary)
		}
	}
	items := preview["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("legacy preview items = %#v", items)
	}
	evidenceStatuses := map[string]bool{}
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["review_source"] != "critic_auto" {
			t.Fatalf("non-legacy recommendation leaked into preview: %#v", item)
		}
		evidenceStatuses[item["evidence_status"].(string)] = true
	}
	if !evidenceStatuses["grounded"] || !evidenceStatuses["present_unverified"] || !evidenceStatuses["missing"] {
		t.Fatalf("legacy evidence classes = %#v", evidenceStatuses)
	}
	if intFromAny(result["count"], -1) != 2 {
		t.Fatalf("read-only preview changed approved library behavior: %#v", result)
	}
	if fake.timeline[0].ReviewStatus != "approved" || fake.entities[0].ReviewStatus != "rejected" || fake.claims[0].ReviewStatus != "pending" || len(fake.reviews) != 0 {
		t.Fatalf("legacy preview mutated review state: timeline=%#v entities=%#v claims=%#v reviews=%#v", fake.timeline, fake.entities, fake.claims, fake.reviews)
	}
}

func TestReferenceLegacyAutoReviewPreviewIncludesAllItems(t *testing.T) {
	const itemCount = 103
	entities := make([]store.ReferenceEntity, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		entities = append(entities, store.ReferenceEntity{EntityID: fmt.Sprintf("entity-%d", i), CanonicalName: fmt.Sprintf("Entity %d", i), ReviewStatus: "approved", ReviewSource: "critic_auto"})
	}
	preview := buildReferenceLegacyAutoReviewPreview("work-1", "continuity-1", nil, entities, nil)
	if preview["truncated"] != false || len(preview["items"].([]map[string]any)) != itemCount {
		t.Fatalf("complete legacy preview = %#v", preview)
	}
	summary := preview["summary"].(map[string]int)
	if summary["total"] != itemCount || summary["potentially_active"] != itemCount {
		t.Fatalf("complete legacy preview lost counts: %#v", summary)
	}
}

func TestReferenceLibraryManageEditExcludeAndRestore(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.documents = append(fake.documents, store.ReferenceDocument{DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1", RawText: "must not leak", ProvenanceJSON: `{"filename":"canon.txt"}`})
	fake.entities = append(fake.entities, store.ReferenceEntity{EntityID: "entity-1", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "character", CanonicalName: "Old Name", ReviewStatus: "approved", MetadataJSON: `{"document_id":"doc-1","evidence_excerpt":"source"}`})
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	listed := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	if intFromAny(listed["count"], 0) != 1 {
		t.Fatalf("approved item missing: %#v", listed)
	}
	documents := listed["documents"].([]any)
	if len(documents) != 1 || documents[0].(map[string]any)["raw_text"] != nil {
		t.Fatalf("source view leaked document text: %#v", documents)
	}

	referenceLibraryTestRequest(t, mux, http.MethodPatch, "/reference-works/work-1/library/entity/entity-1", map[string]any{"action": "edit", "entity_type": "location", "canonical_name": "Correct Name", "description_text": "Corrected"})
	if fake.entities[0].CanonicalName != "Correct Name" || fake.entities[0].EntityType != "location" || fake.entities[0].ReviewSource != "user_edit" {
		t.Fatalf("user edit was not locked: %#v", fake.entities[0])
	}

	referenceLibraryTestRequest(t, mux, http.MethodDelete, "/reference-works/work-1/library/entity/entity-1", nil)
	listed = referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	if intFromAny(listed["count"], 0) != 0 || intFromAny(listed["excluded"].(map[string]any)["count"], 0) != 1 {
		t.Fatalf("recoverable delete was not separated: %#v", listed)
	}

	referenceLibraryTestRequest(t, mux, http.MethodPatch, "/reference-works/work-1/library/entity/entity-1", map[string]any{"action": "restore"})
	listed = referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	if intFromAny(listed["count"], 0) != 1 || fake.entities[0].ReviewSource != "user_restore" {
		t.Fatalf("excluded item was not restored: %#v", listed)
	}
}

func TestReferenceClaimStableIDIgnoresChunkAndCandidateOrder(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	doc := &store.ReferenceDocument{DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1"}
	first := map[string]any{"claims": []any{map[string]any{"claim_type": "world_rule", "claim_text": "Magic requires a spoken name."}}}
	if _, _, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, first, "", 0); err != nil {
		t.Fatal(err)
	}
	second := map[string]any{"claims": []any{
		map[string]any{"claim_type": "event", "claim_text": "Another fact."},
		map[string]any{"claim_type": "world_rule", "claim_text": "Magic requires a spoken name."},
	}}
	if _, _, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, second, "", 4); err != nil {
		t.Fatal(err)
	}
	if fake.claims[0].ClaimID != fake.claims[2].ClaimID {
		t.Fatalf("same semantic claim received different IDs: %q != %q", fake.claims[0].ClaimID, fake.claims[2].ClaimID)
	}
}

func TestReferenceDocumentExtractionTextPreservesStructuredContent(t *testing.T) {
	raw := `<!doctype html><html><body><nav>Site navigation</nav><main><h1>Characters</h1><ul><li>Mina - captain</li><li>Rin - scout</li></ul><h2>Locations</h2><table><tr><th>Name</th><th>Type</th></tr><tr><td>Citadel</td><td>Fortress</td></tr></table></main><script>ignoreMe()</script></body></html>`
	got := referenceDocumentExtractionText(raw)
	for _, expected := range []string{"Characters", "Mina", "Rin", "Locations", "Citadel", "Fortress"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("structured extraction text omitted %q: %s", expected, got)
		}
	}
	for _, excluded := range []string{"Site navigation", "ignoreMe"} {
		if strings.Contains(got, excluded) {
			t.Fatalf("structured extraction text retained excluded page chrome %q: %s", excluded, got)
		}
	}
}

func TestReferenceExtractionReusesExistingCanonicalRecordIDs(t *testing.T) {
	fake := newReferenceLibraryHTTPStore()
	fake.timeline = append(fake.timeline, store.ReferenceTimelineNode{
		NodeID: "timeline-existing", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "arrival", ReviewStatus: "pending",
	})
	fake.entities = append(fake.entities, store.ReferenceEntity{
		EntityID: "entity-existing", WorkID: "work-1", ContinuityID: "continuity-1", EntityType: "character", CanonicalName: "Mina", ReviewStatus: "pending",
	})
	fake.claims = append(fake.claims, store.ReferenceClaim{
		ClaimID: "claim-existing", WorkID: "work-1", ContinuityID: "continuity-1", ClaimType: "world_rule", ClaimText: "Magic requires names.", ReviewStatus: "pending",
	})
	doc := &store.ReferenceDocument{DocumentID: "doc-2", WorkID: "work-1", ContinuityID: "continuity-1", SourceURI: "https://example.test/wiki"}
	parsed := map[string]any{
		"timeline": []any{map[string]any{"node_key": "arrival", "label": "Arrival"}},
		"entities": []any{map[string]any{"entity_type": "character", "canonical_name": "Mina"}},
		"claims":   []any{map[string]any{"claim_type": "world_rule", "claim_text": "Magic requires names."}},
	}
	if _, _, err := saveReferenceExtractionCandidates(context.Background(), fake, doc, parsed, "Mina arrived. Magic requires names.", 0); err != nil {
		t.Fatal(err)
	}
	if got := fake.timeline[len(fake.timeline)-1].NodeID; got != "timeline-existing" {
		t.Fatalf("timeline ID was not reused: %q", got)
	}
	if got := fake.entities[len(fake.entities)-1].EntityID; got != "entity-existing" {
		t.Fatalf("entity ID was not reused: %q", got)
	}
	if got := fake.claims[len(fake.claims)-1].ClaimID; got != "claim-existing" {
		t.Fatalf("claim ID was not reused: %q", got)
	}
}

func TestAnalyzeReferenceLibraryFlagsFactKeyConflict(t *testing.T) {
	claims := []store.ReferenceClaim{
		{ClaimID: "claim-1", ClaimType: "world_rule", ClaimText: "The gate is blue.", MetadataJSON: `{"fact_key":"gate_color"}`},
		{ClaimID: "claim-2", ClaimType: "world_rule", ClaimText: "The gate is red.", MetadataJSON: `{"fact_key":"gate_color"}`},
	}
	diagnostics := analyzeReferenceLibrary(nil, nil, claims)
	if intFromAny(diagnostics["conflict_count"], 0) != 1 {
		t.Fatalf("fact-key conflict was not detected: %#v", diagnostics)
	}
}

func TestReferenceTimelineNormalizeAssignsSpacedUniqueOrder(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"ordered_ids\":[\"node-1\",\"node-3\",\"node-2\"],\"unresolved_ids\":[],\"duplicate_groups\":[],\"conflict_groups\":[],\"notes\":[]}"}}]}`))
	}))
	defer upstream.Close()
	fake := newReferenceLibraryHTTPStore()
	fake.timeline = append(fake.timeline,
		store.ReferenceTimelineNode{NodeID: "node-1", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "a", Label: "1620", Ordinal: 0, ReviewStatus: "approved"},
		store.ReferenceTimelineNode{NodeID: "node-2", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "b", Label: "1900", Ordinal: 1, ReviewStatus: "approved"},
		store.ReferenceTimelineNode{NodeID: "node-3", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "c", Label: "1700", Ordinal: 1, ReviewStatus: "approved"},
	)
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	started := referenceLibraryTestRequest(t, mux, http.MethodPost, "/reference-works/work-1/library/timeline/normalize?continuity_id=continuity-1", map[string]any{
		"continuity_id": "continuity-1",
		"client_meta":   map[string]any{"critic": map[string]any{"provider": "openai", "api_key": "test", "endpoint": upstream.URL, "model": "test", "timeout_ms": 30000}},
	})
	jobID := started["job_id"].(string)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := srv.AdminJobs.get(jobID)
		if ok && job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, _ := srv.AdminJobs.get(jobID)
	if job["status"] != "completed" || fake.timeline[0].Ordinal != 10 || fake.timeline[2].Ordinal != 20 || fake.timeline[1].Ordinal != 30 {
		t.Fatalf("timeline chronology was not applied: job=%#v timeline=%#v", job, fake.timeline)
	}
	listed := referenceLibraryTestRequest(t, mux, http.MethodGet, "/reference-works/work-1/library?continuity_id=continuity-1", nil)
	timeline := listed["timeline"].([]any)
	wantLabels := []string{"1620", "1700", "1900"}
	for i, raw := range timeline {
		item := raw.(map[string]any)
		if intFromAny(item["display_order"], 0) != i+1 || item["label"] != wantLabels[i] {
			t.Fatalf("display order %d was not sequential: %#v", i, timeline)
		}
	}
}
