package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type referenceBindingHTTPStore struct {
	*referenceLibraryHTTPStore
	bindings         []store.SessionReferenceBinding
	runtimes         map[string]store.SessionReferenceRuntime
	coverage         map[string]store.SessionReferenceCoverageSnapshot
	coverageFields   map[string][]store.SessionReferenceCoverageField
	coverageWrites   int
	bindingListReads int
}

func newReferenceBindingHTTPStore() *referenceBindingHTTPStore {
	return &referenceBindingHTTPStore{
		referenceLibraryHTTPStore: newReferenceLibraryHTTPStore(),
		runtimes:                  map[string]store.SessionReferenceRuntime{},
		coverage:                  map[string]store.SessionReferenceCoverageSnapshot{},
		coverageFields:            map[string][]store.SessionReferenceCoverageField{},
	}
}

func (f *referenceBindingHTTPStore) GetReferenceWork(_ context.Context, workID string) (*store.ReferenceWork, error) {
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

func (f *referenceBindingHTTPStore) UpsertSessionReferenceBinding(_ context.Context, item *store.SessionReferenceBinding, expectedRevision int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if expectedRevision == 0 {
		for _, existing := range f.bindings {
			if existing.ChatSessionID == item.ChatSessionID && existing.WorkID == item.WorkID && existing.ContinuityID == item.ContinuityID {
				return store.ErrReferenceConflict
			}
		}
		copy := *item
		copy.Revision = 1
		f.bindings = append(f.bindings, copy)
		return nil
	}
	for i := range f.bindings {
		if f.bindings[i].BindingID == item.BindingID && f.bindings[i].ChatSessionID == item.ChatSessionID {
			if f.bindings[i].Revision != expectedRevision {
				return store.ErrReferenceConflict
			}
			copy := *item
			copy.Revision = expectedRevision + 1
			f.bindings[i] = copy
			return nil
		}
	}
	return store.ErrReferenceConflict
}

func (f *referenceBindingHTTPStore) ListSessionReferenceBindings(_ context.Context, sid string, enabledOnly bool) ([]store.SessionReferenceBinding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindingListReads++
	out := []store.SessionReferenceBinding{}
	for _, item := range f.bindings {
		if item.ChatSessionID == sid && (!enabledOnly || item.Enabled) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *referenceBindingHTTPStore) DeleteSessionReferenceBinding(_ context.Context, sid, bindingID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.bindings {
		if f.bindings[i].ChatSessionID == sid && f.bindings[i].BindingID == bindingID {
			f.bindings = append(f.bindings[:i], f.bindings[i+1:]...)
			delete(f.runtimes, bindingID)
			delete(f.coverage, bindingID)
			delete(f.coverageFields, bindingID)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *referenceBindingHTTPStore) UpsertSessionReferenceRuntime(_ context.Context, item *store.SessionReferenceRuntime, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimes[item.BindingID] = *item
	return nil
}

func (f *referenceBindingHTTPStore) GetSessionReferenceRuntime(_ context.Context, bindingID string) (*store.SessionReferenceRuntime, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.runtimes[bindingID]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (f *referenceBindingHTTPStore) ListReferenceEntityAliasesByScope(_ context.Context, workID, continuityID string) ([]store.ReferenceEntityAlias, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.ReferenceEntityAlias{}
	for _, aliases := range f.aliases {
		for _, item := range aliases {
			if item.WorkID == workID && item.ContinuityID == continuityID {
				out = append(out, item)
			}
		}
	}
	return out, nil
}

func (f *referenceBindingHTTPStore) ReplaceSessionReferenceCoverageSnapshot(_ context.Context, snapshot *store.SessionReferenceCoverageSnapshot, fields []store.SessionReferenceCoverageField) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.coverage[snapshot.BindingID]; ok && existing.SnapshotHash == snapshot.SnapshotHash {
		return false, nil
	}
	copy := *snapshot
	f.coverage[snapshot.BindingID] = copy
	f.coverageFields[snapshot.BindingID] = append([]store.SessionReferenceCoverageField(nil), fields...)
	f.coverageWrites++
	return true, nil
}

func newReferenceBindingTestMux() (*referenceBindingHTTPStore, http.Handler) {
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-1", Title: "Example", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1", Label: "Main", Status: "active"}}
	fake.timeline = []store.ReferenceTimelineNode{
		{NodeID: "node-start", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "start", Label: "Start", Ordinal: 10, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-middle", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "middle", Label: "Middle", Ordinal: 20, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-future", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "future", Label: "Future", Ordinal: 30, BranchKey: "main", ReviewStatus: "approved"},
		{NodeID: "node-alt", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "alt", Label: "Alternate", Ordinal: 20, BranchKey: "alternate", ReviewStatus: "approved"},
		{NodeID: "node-pending", WorkID: "work-1", ContinuityID: "continuity-1", NodeKey: "pending", Label: "Pending", Ordinal: 40, BranchKey: "main", ReviewStatus: "pending"},
	}
	srv := &Server{Cfg: config.Config{}, Store: fake, AdminJobs: newAdminJobManager()}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	return fake, mux
}

func referenceBindingHTTPCall(t *testing.T, mux http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()
	raw := ""
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		raw = string(encoded)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	out := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s %s response: %v body=%q", method, path, err, rec.Body.String())
	}
	return rec.Code, out
}

func validReferenceBindingBody() map[string]any {
	return map[string]any{
		"work_id":                "work-1",
		"continuity_id":          "continuity-1",
		"binding_role":           "primary",
		"reference_mode":         "primary",
		"anchor_mode":            "manual",
		"current_node_id":        "node-start",
		"reveal_ceiling_node_id": "node-middle",
		"future_policy":          "block",
		"priority":               0,
		"expected_revision":      0,
	}
}

func TestReferenceBindingPreviewCRUDAndUnlinkPreservesWork(t *testing.T) {
	fake, mux := newReferenceBindingTestMux()
	body := validReferenceBindingBody()
	code, preview := referenceBindingHTTPCall(t, mux, http.MethodPost, "/sessions/session-1/reference-bindings/preview", body)
	if code != http.StatusOK || preview["valid"] != true || preview["action"] != "create" {
		t.Fatalf("unexpected preview: code=%d body=%#v", code, preview)
	}
	if len(fake.bindings) != 0 {
		t.Fatalf("preview mutated bindings: %#v", fake.bindings)
	}

	code, created := referenceBindingHTTPCall(t, mux, http.MethodPost, "/sessions/session-1/reference-bindings", body)
	if code != http.StatusOK || created["action"] != "create" {
		t.Fatalf("unexpected create: code=%d body=%#v", code, created)
	}
	binding := created["binding"].(map[string]any)
	bindingID := binding["binding_id"].(string)
	if binding["revision"] != float64(1) {
		t.Fatalf("create revision=%v", binding["revision"])
	}
	if binding["enabled"] != true || binding["injection_enabled"] != true {
		t.Fatalf("linked binding must be active without extra toggles: %#v", binding)
	}
	if binding["reference_mode"] != "primary" {
		t.Fatalf("reference mode was not persisted: %#v", binding)
	}

	code, listed := referenceBindingHTTPCall(t, mux, http.MethodGet, "/sessions/session-1/reference-bindings", nil)
	if code != http.StatusOK || len(listed["bindings"].([]any)) != 1 {
		t.Fatalf("unexpected list: code=%d body=%#v", code, listed)
	}

	body["current_node_id"] = "node-middle"
	body["reveal_ceiling_node_id"] = "node-future"
	body["expected_revision"] = 1
	code, updated := referenceBindingHTTPCall(t, mux, http.MethodPatch, "/sessions/session-1/reference-bindings/"+bindingID, body)
	if code != http.StatusOK || updated["action"] != "update" {
		t.Fatalf("unexpected update: code=%d body=%#v", code, updated)
	}
	if updated["binding"].(map[string]any)["revision"] != float64(2) {
		t.Fatalf("update revision=%v", updated["binding"].(map[string]any)["revision"])
	}

	code, deleted := referenceBindingHTTPCall(t, mux, http.MethodDelete, "/sessions/session-1/reference-bindings/"+bindingID+"?expected_revision=2", nil)
	if code != http.StatusOK || deleted["action"] != "unlinked" {
		t.Fatalf("unexpected delete: code=%d body=%#v", code, deleted)
	}
	if len(fake.bindings) != 0 || len(fake.works) != 1 || len(fake.timeline) != 5 {
		t.Fatalf("unlink changed reusable reference data: bindings=%d works=%d timeline=%d", len(fake.bindings), len(fake.works), len(fake.timeline))
	}
}

func TestReferenceBindingPreviewBlocksInvalidScopes(t *testing.T) {
	_, mux := newReferenceBindingTestMux()
	tests := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{name: "work", edit: func(v map[string]any) { v["work_id"] = "missing" }, want: "work_not_found"},
		{name: "continuity", edit: func(v map[string]any) { v["continuity_id"] = "missing" }, want: "continuity_not_in_work"},
		{name: "reference mode", edit: func(v map[string]any) { v["reference_mode"] = "automatic_guess" }, want: "reference_mode_invalid"},
		{name: "unknown node", edit: func(v map[string]any) { v["current_node_id"] = "missing" }, want: "current_node_not_approved"},
		{name: "pending node", edit: func(v map[string]any) { v["current_node_id"] = "node-pending" }, want: "current_node_not_approved"},
		{name: "cross branch", edit: func(v map[string]any) { v["reveal_ceiling_node_id"] = "node-alt" }, want: "selected_nodes_cross_branch"},
		{name: "reveal before current", edit: func(v map[string]any) {
			v["current_node_id"] = "node-middle"
			v["reveal_ceiling_node_id"] = "node-start"
		}, want: "reveal_ceiling_before_current"},
		{name: "divergence after current", edit: func(v map[string]any) { v["divergence_node_id"] = "node-future" }, want: "divergence_after_current"},
		{name: "divergence without current", edit: func(v map[string]any) { v["current_node_id"] = ""; v["divergence_node_id"] = "node-start" }, want: "divergence_requires_current_node"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := validReferenceBindingBody()
			tc.edit(body)
			code, result := referenceBindingHTTPCall(t, mux, http.MethodPost, "/sessions/session-1/reference-bindings/preview", body)
			if code != http.StatusOK || result["valid"] != false {
				t.Fatalf("preview was not blocked: code=%d body=%#v", code, result)
			}
			found := false
			for _, value := range result["blocked_reasons"].([]any) {
				if value == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("blocked reasons %#v do not contain %q", result["blocked_reasons"], tc.want)
			}
		})
	}
}

func TestReferenceBindingApplyRequiresCurrentRevision(t *testing.T) {
	_, mux := newReferenceBindingTestMux()
	body := validReferenceBindingBody()
	code, _ := referenceBindingHTTPCall(t, mux, http.MethodPost, "/sessions/session-1/reference-bindings", body)
	if code != http.StatusOK {
		t.Fatalf("create failed: %d", code)
	}
	body["expected_revision"] = 99
	code, result := referenceBindingHTTPCall(t, mux, http.MethodPost, "/sessions/session-1/reference-bindings", body)
	if code != http.StatusConflict || result["code"] != "reference_binding_revision_conflict" {
		t.Fatalf("revision conflict was not enforced: code=%d body=%#v", code, result)
	}
}
