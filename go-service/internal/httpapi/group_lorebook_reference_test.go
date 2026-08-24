package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type lorebookReferenceHTTPStore struct {
	store.Store
	last              *store.LorebookReferenceSnapshot
	currentPage       *store.LorebookReferenceCurrentPage
	currentPageScope  store.LorebookReferenceScope
	latestSessionID   string
	currentPageLimit  int
	currentPageOffset int
}

func (f *lorebookReferenceHTTPStore) ApplyLorebookReferenceSnapshot(_ context.Context, item *store.LorebookReferenceSnapshot) (*store.LorebookReferenceSnapshotResult, error) {
	copy := *item
	copy.Entries = append([]store.LorebookReferenceEntryObservation(nil), item.Entries...)
	f.last = &copy
	return &store.LorebookReferenceSnapshotResult{
		ScopeID: 7, SnapshotID: item.SnapshotID, ObservationState: item.ObservationState,
		LifecycleAction: "current_projection_replaced", ObservedEntryCount: len(item.Entries), CurrentEntryCount: len(item.Entries),
	}, nil
}

func (f *lorebookReferenceHTTPStore) GetLorebookReferenceCurrent(context.Context, store.LorebookReferenceScope) (*store.LorebookReferenceCurrent, error) {
	return nil, store.ErrNotFound
}

func (f *lorebookReferenceHTTPStore) GetLorebookReferenceCurrentPage(_ context.Context, scope store.LorebookReferenceScope, limit, offset int) (*store.LorebookReferenceCurrentPage, error) {
	f.currentPageScope = scope
	f.currentPageLimit = limit
	f.currentPageOffset = offset
	if f.currentPage == nil {
		return nil, store.ErrNotFound
	}
	return f.currentPage, nil
}

func (f *lorebookReferenceHTTPStore) GetLorebookReferenceLatestSessionPage(_ context.Context, sessionID string, limit, offset int) (*store.LorebookReferenceCurrentPage, error) {
	f.latestSessionID = sessionID
	f.currentPageLimit = limit
	f.currentPageOffset = offset
	if f.currentPage == nil {
		return nil, store.ErrNotFound
	}
	return f.currentPage, nil
}

func TestLorebookReferenceSnapshotUsesOfficialShapeAndPreservesObservedFalse(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	body := []byte(`{
		"contract_version":"lorebook_reference_snapshot.v1",
		"consent_state":"active",
		"observation_state":"observed",
		"complete_snapshot":true,
		"character_index":4,
		"chat_index":9,
		"enabled_module_ids":["module-b","module-a"],
		"enabled_modules_observed":true,
		"host_product":"pocketrisu",
		"future_optional_field":"ignored",
		"entries":[{
			"id":"entry-1","key":"Han-eol","secondkey":"exam","insertorder":0,
			"comment":"profile","content":"Han-eol will take the exam.","mode":"normal",
			"alwaysActive":false,"selective":false,"useRegex":false,"activationPercent":0,
			"bookVersion":3,"folder":"cast","extentions":{"risu_case_sensitive":false},
			"future_entry_field":"ignored"
		}]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/sessions/session-a/lorebook-reference/snapshots", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.last == nil || len(fake.last.Entries) != 1 {
		t.Fatalf("snapshot=%#v", fake.last)
	}
	entry := fake.last.Entries[0]
	if entry.AlwaysActive == nil || *entry.AlwaysActive || entry.ActivationPct == nil || *entry.ActivationPct != 0 {
		t.Fatalf("observed false/zero was not preserved: %#v", entry)
	}
	if fake.last.Scope.CharacterIndex == nil || *fake.last.Scope.CharacterIndex != 4 || fake.last.Scope.ChatIndex == nil || *fake.last.Scope.ChatIndex != 9 {
		t.Fatalf("scope=%#v", fake.last.Scope)
	}
	if entry.SourceKind != "current_host_aggregate" || entry.SourceIdentity != "" {
		t.Fatalf("unexposed source was inferred: %#v", entry)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response["status"] != "ok" {
		t.Fatalf("response=%s err=%v", recorder.Body.String(), err)
	}
}

func TestLorebookReferenceSnapshotToleratesOptionalBookVersionShapes(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	body := []byte(`{
		"contract_version":"lorebook_reference_snapshot.v1",
		"consent_state":"active",
		"observation_state":"observed",
		"complete_snapshot":true,
		"character_index":4,
		"chat_index":9,
		"enabled_module_ids":[],
		"enabled_modules_observed":true,
		"entries":[
			{"id":"numeric","content":"numeric","bookVersion":3},
			{"id":"numeric-string","content":"numeric string","bookVersion":"2"},
			{"id":"empty-string","content":"empty string","bookVersion":""},
			{"id":"invalid-string","content":"invalid string","bookVersion":"legacy"},
			{"id":"null","content":"null","bookVersion":null},
			{"id":"missing","content":"missing"}
		]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/sessions/session-a/lorebook-reference/snapshots", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.last == nil || len(fake.last.Entries) != 6 {
		t.Fatalf("snapshot=%#v", fake.last)
	}
	for index, want := range []int64{3, 2} {
		got := fake.last.Entries[index].BookVersion
		if got == nil || *got != want {
			t.Fatalf("entry[%d] bookVersion=%v want=%d", index, got, want)
		}
	}
	for _, index := range []int{2, 3, 4, 5} {
		if got := fake.last.Entries[index].BookVersion; got != nil {
			t.Fatalf("entry[%d] bookVersion=%v want=nil", index, *got)
		}
	}
}

func TestLorebookReferenceObservedActiveSnapshotMustBeComplete(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	body := []byte(`{"contract_version":"lorebook_reference_snapshot.v1","consent_state":"active","observation_state":"observed","complete_snapshot":false}`)
	request := httptest.NewRequest(http.MethodPost, "/sessions/session-a/lorebook-reference/snapshots", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || fake.last != nil {
		t.Fatalf("status=%d snapshot=%#v body=%s", recorder.Code, fake.last, recorder.Body.String())
	}
}

func TestLorebookReferenceRouteDoesNotFallBackToCanonicalStore(t *testing.T) {
	server := &Server{Store: store.NewNoopStore()}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/sessions/session-a/lorebook-reference/snapshots", bytes.NewReader([]byte(`{}`)))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestLorebookReferenceCurrentReturnsBoundedExactScopeProjection(t *testing.T) {
	characterIndex := int64(4)
	chatIndex := int64(9)
	fake := &lorebookReferenceHTTPStore{
		Store: store.NewNoopStore(),
		currentPage: &store.LorebookReferenceCurrentPage{
			ScopeID: 7,
			Entries: []store.LorebookReferenceEntryObservation{
				{EntryOrdinal: 20, Key: "Han-eol", Content: "stored lore"},
			},
			Total: 41, Limit: 20, Offset: 20,
		},
	}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	request := httptest.NewRequest(http.MethodGet,
		"/sessions/session-a/lorebook-reference/current?character_index=4&chat_index=9&enabled_modules_observed=true&enabled_module_id=module-b&enabled_module_id=module-a&limit=20&offset=20", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.currentPageLimit != 20 || fake.currentPageOffset != 20 || fake.currentPageScope.CharacterIndex == nil || *fake.currentPageScope.CharacterIndex != characterIndex || fake.currentPageScope.ChatIndex == nil || *fake.currentPageScope.ChatIndex != chatIndex {
		t.Fatalf("scope=%#v limit=%d offset=%d", fake.currentPageScope, fake.currentPageLimit, fake.currentPageOffset)
	}
	if !fake.currentPageScope.EnabledModulesObserved || len(fake.currentPageScope.EnabledModuleIDs) != 2 {
		t.Fatalf("module scope=%#v", fake.currentPageScope)
	}
	var response struct {
		Status          string                                    `json:"status"`
		ContractVersion string                                    `json:"contract_version"`
		Items           []store.LorebookReferenceEntryObservation `json:"items"`
		Total           int                                       `json:"total"`
		HasMore         bool                                      `json:"has_more"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || response.ContractVersion != store.LorebookReferenceCurrentViewV1 || len(response.Items) != 1 || response.Total != 41 || !response.HasMore {
		t.Fatalf("response=%+v", response)
	}
}

func TestLorebookReferenceCurrentRejectsUnboundedPage(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sessions/session-a/lorebook-reference/current?limit=101", nil))
	if recorder.Code != http.StatusBadRequest || fake.currentPageLimit != 0 {
		t.Fatalf("status=%d called_limit=%d body=%s", recorder.Code, fake.currentPageLimit, recorder.Body.String())
	}
}

func TestLorebookReferenceCurrentReturnsLatestStoredScopeForSelectedSession(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{
		Store: store.NewNoopStore(),
		currentPage: &store.LorebookReferenceCurrentPage{
			ScopeID: 12,
			Entries: []store.LorebookReferenceEntryObservation{
				{EntryOrdinal: 0, Key: "archive", Content: "selected session lore"},
			},
			Total: 1, Limit: 20, Offset: 0,
		},
	}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/sessions/session-b/lorebook-reference/current?scope_mode=latest_session&limit=20&offset=0", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.latestSessionID != "session-b" || fake.currentPageLimit != 20 || fake.currentPageOffset != 0 {
		t.Fatalf("latest_session=%q limit=%d offset=%d", fake.latestSessionID, fake.currentPageLimit, fake.currentPageOffset)
	}
	if fake.currentPageScope.ChatSessionID != "" {
		t.Fatalf("latest-session read must not enter exact-scope owner: %#v", fake.currentPageScope)
	}
}

func TestLorebookReferenceCurrentReturnsEmptyProjectionForSessionWithoutLorebook(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/sessions/session-empty/lorebook-reference/current?scope_mode=latest_session&limit=20&offset=0", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Status string                                    `json:"status"`
		Scope  store.LorebookReferenceScope              `json:"scope"`
		Items  []store.LorebookReferenceEntryObservation `json:"items"`
		Total  int                                       `json:"total"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || response.Scope.ChatSessionID != "session-empty" || response.Total != 0 || len(response.Items) != 0 {
		t.Fatalf("response=%+v body=%s", response, recorder.Body.String())
	}
}

func TestLorebookReferenceCurrentRejectsUnknownScopeMode(t *testing.T) {
	fake := &lorebookReferenceHTTPStore{Store: store.NewNoopStore()}
	server := &Server{Store: fake}
	mux := http.NewServeMux()
	server.registerLorebookReferenceRoutes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/sessions/session-a/lorebook-reference/current?scope_mode=guess", nil))
	if recorder.Code != http.StatusBadRequest || fake.latestSessionID != "" || fake.currentPageScope.ChatSessionID != "" {
		t.Fatalf("status=%d latest_session=%q exact_scope=%#v body=%s", recorder.Code, fake.latestSessionID, fake.currentPageScope, recorder.Body.String())
	}
}
