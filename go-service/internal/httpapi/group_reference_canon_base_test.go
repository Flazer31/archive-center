package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestPrimaryCanonBaseIncludesDynamicWorkAndMembershipWithinReferenceBudget(t *testing.T) {
	fake, vectorStore, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	response := preparePrimaryCanonBase(t, mux, true, 1200, intPointer(1200))
	base := response["primary_canon_base"].(map[string]any)
	if base["status"] != "ready" || base["configured_subbudget_chars"] != float64(1200) {
		t.Fatalf("base diagnostics = %#v", base)
	}
	if base["effective_subbudget_chars"] != float64(1200) || base["reference_total_cap_chars"] != float64(1200) || base["budget_scope"] != "within_reference_total" {
		t.Fatalf("base subbudget diagnostics = %#v", base)
	}
	policy := response["reference_injection"].(map[string]any)["budget_policy"].(map[string]any)
	if policy["mode"] != referenceModePrimary || policy["total_cap_chars"] != float64(1200) || policy["ratio_numerator"] != float64(1) || policy["ratio_denominator"] != float64(1) {
		t.Fatalf("primary reference budget policy = %#v", policy)
	}
	text, _ := base["text"].(string)
	if !containsAll(text, "[Primary Canon Base]", fake.works[0].Title, fake.continuities[0].Label, "North Watch", "includes Aster and Brin") {
		t.Fatalf("dynamic canon base = %q", text)
	}
	if strings.Contains(text, fake.bindings[0].WorkID) || strings.Contains(text, fake.bindings[0].ContinuityID) || strings.Contains(text, "entity-watch") || strings.Contains(text, "claim-members") {
		t.Fatalf("canon prompt leaked internal IDs: %q", text)
	}
	if int(base["used_chars"].(float64)) > int(base["effective_subbudget_chars"].(float64)) {
		t.Fatalf("base exceeded own budget: %#v", base)
	}
	sceneUsed := response["reference_injection"].(map[string]any)["scene_used_chars"].(float64)
	if policy["used_chars"].(float64) != base["used_chars"].(float64)+sceneUsed || policy["used_chars"].(float64) > policy["total_cap_chars"].(float64) {
		t.Fatalf("primary plus scene exceeded reference total: policy=%#v base=%#v", policy, base)
	}
	selected := base["selected_source_ids"].([]any)
	if len(selected) == 0 {
		t.Fatalf("selected source ids missing: %#v", base)
	}
	if base["search_status"] != "ready" {
		t.Fatalf("foundation search status = %#v", base)
	}
	queries := base["foundation_queries"].([]any)
	if len(queries) != 1 || !containsAll(queries[0].(string), "Continue the scene.", fake.works[0].Title, fake.continuities[0].Label, fake.bindings[0].ContinuityID) {
		t.Fatalf("foundation queries = %#v", queries)
	}
	approvedCount := len(fake.timeline) + len(fake.entities) + len(fake.claims)
	if vectorStore.exactQuery.Limit != approvedCount {
		t.Fatalf("foundation query limit = %d, want approved inventory %d", vectorStore.exactQuery.Limit, approvedCount)
	}
	combined, _ := response["injection_text"].(string)
	if !strings.HasPrefix(combined, "[Primary Canon Base]") || len([]rune(combined)) <= 1 {
		t.Fatalf("base borrowed main budget or was not applied: %q", combined)
	}
	pack := response["injection_pack"].(map[string]any)
	if pack["primary_canon_base_text"] != text || !strings.Contains(pack["injection_text"].(string), text) {
		t.Fatalf("base was not carried by injection pack: %#v", pack)
	}
	if recall := response["reference_recall"].(map[string]any); len(recall["injection_items"].([]any)) != 0 {
		t.Fatalf("canon base source ids were duplicated in scene injection: %#v", recall["injection_items"])
	}
}

func TestPrimaryCanonBaseDoesNotApplyToSupplementOrMissingBinding(t *testing.T) {
	for _, test := range []struct {
		name        string
		mode        string
		dropBinding bool
		wantStatus  string
	}{
		{name: "supplement", mode: referenceModeSupplement, wantStatus: "not_applicable"},
		{name: "no binding", mode: referenceModePrimary, dropBinding: true, wantStatus: "empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake, _, _, srv := primaryCanonBaseFixtureServer(t, test.mode)
			if test.dropBinding {
				fake.bindings = nil
			}
			mux := http.NewServeMux()
			srv.RegisterRoutes(mux)
			response := preparePrimaryCanonBase(t, mux, true, 300, intPointer(1200))
			base := response["primary_canon_base"].(map[string]any)
			if base["status"] != test.wantStatus || base["text"] != nil {
				t.Fatalf("base = %#v", base)
			}
			if text, _ := response["injection_text"].(string); strings.Contains(text, "[Primary Canon Base]") {
				t.Fatalf("non-primary base changed injection: %q", text)
			}
		})
	}
}

func TestPrimaryCanonBaseRequiresExplicitBudgetAndInjectionEnablement(t *testing.T) {
	for _, test := range []struct {
		name       string
		enabled    bool
		budget     *int
		wantStatus string
	}{
		{name: "missing budget", enabled: true, budget: nil, wantStatus: "budget_missing"},
		{name: "zero budget", enabled: true, budget: intPointer(0), wantStatus: "disabled"},
		{name: "injection disabled", enabled: false, budget: intPointer(1200), wantStatus: "disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
			mux := http.NewServeMux()
			srv.RegisterRoutes(mux)
			response := preparePrimaryCanonBase(t, mux, test.enabled, 300, test.budget)
			base := response["primary_canon_base"].(map[string]any)
			if base["status"] != test.wantStatus || base["text"] != nil || base["used_chars"] != float64(0) {
				t.Fatalf("base = %#v", base)
			}
		})
	}
}

func TestPrimaryCanonBaseSupplementSkipsFoundationDependencies(t *testing.T) {
	_, vectorStore, embeddingCalls, srv := primaryCanonBaseFixtureServer(t, referenceModeSupplement)
	result := srv.buildPrimaryCanonBase(context.Background(), "session-1", "Continue the scene.", intPointer(1200), 1200, true, nil)
	if result.Status != "not_applicable" || result.Text != "" {
		t.Fatalf("supplement base = %#v", result)
	}
	if *embeddingCalls != 0 || len(vectorStore.exactQueries) != 0 {
		t.Fatalf("supplement invoked foundation dependencies: embedding=%d vector=%d", *embeddingCalls, len(vectorStore.exactQueries))
	}
}

func TestPrimaryCanonBaseSupplementPrepareDoesNotRunAdditionalPrimaryReads(t *testing.T) {
	fake, vectorStore, embeddingCalls, srv := primaryCanonBaseFixtureServer(t, referenceModeSupplement)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := preparePrimaryCanonBase(t, mux, true, 300, intPointer(1200))
	base := response["primary_canon_base"].(map[string]any)
	if base["status"] != "not_applicable" {
		t.Fatalf("supplement base = %#v", base)
	}
	policy := response["reference_injection"].(map[string]any)["budget_policy"].(map[string]any)
	if policy["mode"] != referenceModeSupplement || policy["total_cap_chars"] != float64(150) || policy["source"] != "supplement_reference_mode" {
		t.Fatalf("supplement reference budget policy = %#v", policy)
	}
	if fake.bindingListReads != 1 {
		t.Fatalf("supplement prepare binding reads = %d, want only reference recall read", fake.bindingListReads)
	}
	if *embeddingCalls != 1 || len(vectorStore.exactQueries) != 1 {
		t.Fatalf("supplement prepare foundation calls were not skipped: embedding=%d vector=%d", *embeddingCalls, len(vectorStore.exactQueries))
	}
}

func TestPrimaryCanonBaseConfiguredCapIsBoundedByReferenceTotal(t *testing.T) {
	_, _, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	mainCap := 120
	configuredBaseCap := mainCap * 10
	response := preparePrimaryCanonBase(t, mux, true, mainCap, intPointer(configuredBaseCap))
	base := response["primary_canon_base"].(map[string]any)
	if base["configured_subbudget_chars"] != float64(configuredBaseCap) || base["effective_subbudget_chars"] != float64(mainCap) || base["reference_total_cap_chars"] != float64(mainCap) {
		t.Fatalf("bounded primary base = %#v", base)
	}
	if base["used_chars"].(float64) > base["effective_subbudget_chars"].(float64) {
		t.Fatalf("primary base exceeded reference subbudget: %#v", base)
	}
}

func TestPrimaryCanonBaseFirstTurnUsesConfiguredReferenceBudgetBasis(t *testing.T) {
	_, _, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"raw_user_input":  "Continue the scene.",
		"messages":        []map[string]any{{"role": "user", "content": "Continue the scene."}},
		"settings": map[string]any{
			"injection_enabled":                      false,
			"reference_injection_enabled":            true,
			"max_injection_chars":                    0,
			"reference_injection_budget_basis_chars": 1200,
			"primary_canon_base_max_chars":           300,
			"top_k":                                  4,
		},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare status=%d body=%s", rec.Code, rec.Body.String())
	}
	response := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	policy := response["reference_injection"].(map[string]any)["budget_policy"].(map[string]any)
	base := response["primary_canon_base"].(map[string]any)
	if policy["main_injection_cap_chars"] != float64(0) || policy["budget_basis_chars"] != float64(1200) || policy["total_cap_chars"] != float64(1200) {
		t.Fatalf("first-turn budget policy = %#v", policy)
	}
	if base["status"] != "ready" || base["text"] == nil {
		t.Fatalf("first-turn Canon Base = %#v", base)
	}
}

func TestPrimaryCanonBaseVectorFailureFallsBackToEligibleDirectCanon(t *testing.T) {
	_, vectorStore, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
	vectorStore.exactErr = errors.New("vector unavailable")
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := preparePrimaryCanonBase(t, mux, true, 300, intPointer(1200))
	if response["status"] != "ok" {
		t.Fatalf("main prepare failed: %#v", response)
	}
	base := response["primary_canon_base"].(map[string]any)
	text, _ := base["text"].(string)
	if base["status"] != "degraded" || base["search_status"] != "degraded" || !containsAll(text, "Example Chronicle", "North Watch includes Aster and Brin") {
		t.Fatalf("direct canon fallback = %#v", base)
	}
	if pack := response["injection_pack"].(map[string]any); pack["status"] != "skeleton" || pack["primary_canon_base_status"] != "degraded" {
		t.Fatalf("degraded base changed the main pack status: %#v", pack)
	}
	missing := base["missing_fields"].([]any)
	if !containsJSONValue(missing, "foundation_search") {
		t.Fatalf("foundation failure diagnostic missing: %#v", missing)
	}
}

func TestPrimaryCanonBaseIdentityOnlyIsUndercovered(t *testing.T) {
	fake, vectorStore, _, srv := primaryCanonBaseFixtureServer(t, referenceModePrimary)
	fake.entities = nil
	fake.aliases = map[string][]store.ReferenceEntityAlias{}
	fake.claims = nil
	vectorStore.exactResults = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := preparePrimaryCanonBase(t, mux, true, 300, intPointer(1200))
	base := response["primary_canon_base"].(map[string]any)
	if base["status"] != "undercovered" || len(base["selected_source_ids"].([]any)) != 0 {
		t.Fatalf("identity-only base = %#v", base)
	}
	if pack := response["injection_pack"].(map[string]any); pack["status"] != "skeleton" || pack["primary_canon_base_status"] != "undercovered" {
		t.Fatalf("undercovered base changed the main pack status: %#v", pack)
	}
}

func TestPrimaryCanonBaseDependencyFailureDoesNotFailPrepareTurn(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	response := preparePrimaryCanonBase(t, mux, true, 300, intPointer(1200))
	if response["status"] != "ok" {
		t.Fatalf("main prepare failed: %#v", response)
	}
	base := response["primary_canon_base"].(map[string]any)
	if base["status"] != "failed" || base["text"] != nil {
		t.Fatalf("base dependency status = %#v", base)
	}
}

func primaryCanonBaseFixtureServer(t *testing.T, mode string) (*referenceBindingHTTPStore, *referenceVectorTestStore, *int, *Server) {
	t.Helper()
	fake := newReferenceBindingHTTPStore()
	fake.works = []store.ReferenceWork{{WorkID: "work-example", Title: "Example Chronicle", Status: "ready"}}
	fake.continuities = []store.ReferenceContinuity{{ContinuityID: "continuity-main", WorkID: "work-example", Label: "Main Continuity", Status: "ready"}}
	fake.timeline = []store.ReferenceTimelineNode{{NodeID: "node-current", WorkID: "work-example", ContinuityID: "continuity-main", Label: "Current", Ordinal: 1, BranchKey: "main", ReviewStatus: "approved"}}
	fake.entities = []store.ReferenceEntity{
		{EntityID: "entity-watch", WorkID: "work-example", ContinuityID: "continuity-main", CanonicalName: "North Watch", EntityType: "faction", DescriptionText: "A city guard organization.", ReviewStatus: "approved"},
		{EntityID: "entity-aster", WorkID: "work-example", ContinuityID: "continuity-main", CanonicalName: "Aster", EntityType: "character", ReviewStatus: "approved"},
		{EntityID: "entity-brin", WorkID: "work-example", ContinuityID: "continuity-main", CanonicalName: "Brin", EntityType: "character", ReviewStatus: "approved"},
	}
	fake.aliases["entity-watch"] = []store.ReferenceEntityAlias{{WorkID: "work-example", ContinuityID: "continuity-main", EntityID: "entity-watch", AliasText: "The Watch"}}
	fake.claims = []store.ReferenceClaim{{ClaimID: "claim-members", WorkID: "work-example", ContinuityID: "continuity-main", SubjectEntityID: "entity-watch", ClaimType: "relationship", ClaimText: "North Watch includes Aster and Brin.", TemporalScope: "timeless", BranchKey: "main", KnowledgeScope: "public_world", ReviewStatus: "approved"}}
	fake.bindings = []store.SessionReferenceBinding{{BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-example", ContinuityID: "continuity-main", ReferenceMode: mode, CurrentNodeID: "node-current", RevealCeilingNodeID: "node-current"}}
	embeddingServer, embeddingCalls := referenceVectorEmbeddingServer(t)
	t.Cleanup(embeddingServer.Close)
	vectorStore := &referenceVectorTestStore{exactResults: []vector.ExactQueryResult{
		{Document: primaryCanonBaseVectorDocument("entity", "entity-watch"), ChromaRank: 1},
		{Document: primaryCanonBaseVectorDocument("claim", "claim-members"), ChromaRank: 2},
	}}
	return fake, vectorStore, embeddingCalls, referenceRecallTestServer(fake, vectorStore, embeddingServer.URL)
}

func primaryCanonBaseVectorDocument(kind, sourceID string) vector.VectorDocument {
	doc := referenceRecallVectorDocument(kind, sourceID)
	doc.Metadata["work_id"] = "work-example"
	doc.Metadata["continuity_id"] = "continuity-main"
	return doc
}

func preparePrimaryCanonBase(t *testing.T, handler http.Handler, injectionEnabled bool, mainBudget int, baseBudget *int) map[string]any {
	t.Helper()
	settings := map[string]any{
		"injection_enabled":   injectionEnabled,
		"max_injection_chars": mainBudget,
		"top_k":               4,
	}
	if baseBudget != nil {
		settings["primary_canon_base_max_chars"] = *baseBudget
	}
	body, _ := json.Marshal(map[string]any{
		"chat_session_id": "session-1",
		"raw_user_input":  "Continue the scene.",
		"messages":        []map[string]any{{"role": "user", "content": "Continue the scene."}},
		"settings":        settings,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("prepare status=%d body=%s", rec.Code, rec.Body.String())
	}
	response := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func intPointer(value int) *int { return &value }

func containsJSONValue(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
