package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSourceDiscoveryRejectsPrivateAndSecretURLs(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/private",
		"http://localhost/private",
		"http://169.254.169.254/latest/meta-data",
		"https://example.com/page?api_key=secret",
		"https://user:pass@example.com/page",
	} {
		if _, err := validateDiscoveryURL(context.Background(), raw); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
}

func TestSourceDiscoveryLLMTimeoutPreservesConfiguredValue(t *testing.T) {
	if got := sourceDiscoveryLLMTimeout(180000); got != 180*time.Second {
		t.Fatalf("timeout=%v", got)
	}
	if got := sourceDiscoveryLLMTimeout(0); got != 60*time.Second {
		t.Fatalf("default timeout=%v", got)
	}
	failure := sourceCandidateExtractionFailure(fmt.Errorf("Post %q: %w", "https://provider.example/api/chat", context.DeadlineExceeded))
	if failure["code"] != "candidate_extraction_timeout" {
		t.Fatalf("failure=%#v", failure)
	}
	if sourceDiscoveryOperationTimeout != 9*time.Minute {
		t.Fatalf("operation timeout=%v", sourceDiscoveryOperationTimeout)
	}
}

func TestSourceDiscoveryUsesSelectedWorkIdentityAndType(t *testing.T) {
	ref := newReferenceLibraryHTTPStore()
	ref.works = append(ref.works, store.ReferenceWork{
		WorkID: "work-novel", Title: "Shared Title", WorkType: "novel",
	})
	srv := NewServer(config.Default())
	srv.Store = ref

	input, err := srv.resolveSourceDiscoveryWorkIdentity(context.Background(), store.SourceDiscoveryInput{
		WorkID: "work-novel", WorkQuery: "Different typed title",
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.WorkTitle != "Shared Title" || input.WorkQuery != "Shared Title" || input.WorkType != "novel" {
		t.Fatalf("resolved input=%#v", input)
	}
	if query := sourceDiscoverySearchQuery(input); query != "Shared Title novel wiki" {
		t.Fatalf("search query=%q", query)
	}
}

func TestSourceDiscoverySearchLocaleUsesDisplayTitleScriptOnly(t *testing.T) {
	tests := []struct {
		name  string
		input store.SourceDiscoveryInput
		want  string
	}{
		{name: "hangul display title", input: store.SourceDiscoveryInput{WorkTitle: "블루 아카이브", Language: "ja"}, want: "ko"},
		{name: "kana display title", input: store.SourceDiscoveryInput{WorkTitle: "オーバーロード", Language: "ko"}, want: "ja"},
		{name: "han only is ambiguous", input: store.SourceDiscoveryInput{WorkTitle: "王国"}, want: ""},
		{name: "latin is ambiguous", input: store.SourceDiscoveryInput{WorkTitle: "Overlord"}, want: ""},
		{name: "mixed hangul and kana is ambiguous", input: store.SourceDiscoveryInput{WorkTitle: "작품 アニメ"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceDiscoveryTitleLocale(tt.input); got != tt.want {
				t.Fatalf("locale=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestSourceDiscoveryPrioritizesDisplayTitleLocaleWithoutDroppingFallbacks(t *testing.T) {
	input := []sourceSearchProviderResult{
		{URL: "https://ja.wikipedia.org/wiki/example", Title: "日本語資料"},
		{URL: "https://reference.wiki/wiki/example", Title: "Reference"},
		{URL: "https://namu.moe/w/example", Title: "한국어 자료"},
		{URL: "https://ko.wikipedia.org/wiki/example", Title: "Example"},
		{URL: "https://reference.co.kr/wiki/example", Title: "Reference"},
	}
	got := prioritizeSourceSearchResultsByLocale(input, "ko")
	if len(got) != len(input) {
		t.Fatalf("prioritization dropped fallback sources: got=%d want=%d", len(got), len(input))
	}
	if got[0].URL != input[2].URL || got[1].URL != input[3].URL || got[2].URL != input[4].URL || got[3].URL != input[1].URL || got[4].URL != input[0].URL {
		t.Fatalf("unexpected locale priority: %#v", got)
	}
}

func TestSourceDiscoveryDefersExplicitFallbackLocaleWhenActiveTierExists(t *testing.T) {
	input := []sourceSearchProviderResult{
		{URL: "https://ko.wikipedia.org/wiki/example", Title: "한국어 자료"},
		{URL: "https://reference.wiki/wiki/example", Title: "Reference"},
		{URL: "https://ja.wikipedia.org/wiki/example", Title: "日本語資料"},
	}
	active, deferred := deferExplicitFallbackLocaleResults(input, "ko")
	if len(active) != 2 || len(deferred) != 1 || deferred[0].URL != input[2].URL {
		t.Fatalf("active=%#v deferred=%#v", active, deferred)
	}

	fallbackOnly := []sourceSearchProviderResult{{URL: "https://ja.wikipedia.org/wiki/example", Title: "日本語資料"}}
	active, deferred = deferExplicitFallbackLocaleResults(fallbackOnly, "ko")
	if len(active) != 1 || len(deferred) != 0 {
		t.Fatalf("fallback-only tier was dropped: active=%#v deferred=%#v", active, deferred)
	}
}

func TestSourceCandidateExtractionReceivesSelectedWorkType(t *testing.T) {
	var request map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": `{"candidates":[],"follow_up_queries":[]}`}}},
		})
	}))
	defer provider.Close()

	_, _, _, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000,
	}, store.SourceDiscoveryInput{WorkQuery: "Shared Title", WorkTitle: "Shared Title", WorkType: "novel"}, map[string]any{
		"section_candidates": []any{map[string]any{
			"source_url": "https://reference.example/wiki/shared", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Bounded evidence.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := sliceFromAny(request["messages"])
	if len(messages) < 2 {
		t.Fatalf("messages=%#v", messages)
	}
	joined := fmt.Sprint(messages)
	if !strings.Contains(joined, `"work_type":"novel"`) {
		t.Fatalf("selected work type missing from extraction request: %s", joined)
	}
}

func TestSourceCandidateExtractionUsesCompactReferencesAndExhaustionContract(t *testing.T) {
	var request map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		content := `{"candidates":[{"kind":"character","source_ref":"s1","canonical_name":"Alpha","work_identity_status":"matched","canon_scope_status":"in_world"}],"follow_up_queries":[],"examined_source_refs":["s1"],"incomplete_source_refs":[]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer provider.Close()

	sections := []any{map[string]any{
		"source_url": "https://reference.example/wiki/work", "source_type": "community_wiki",
		"document_sha256": "hash-1", "locator": map[string]any{"type": "li", "value": "1"},
		"excerpt": "Alpha is a named guardian.", "equivalent_evidence": []any{map[string]any{"source_url": "https://mirror.example/wiki/work"}},
	}}
	candidates, _, trace, err := runSourceCandidateExtractionBatch(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000, MaxTokens: 512,
	}, store.SourceDiscoveryInput{WorkQuery: "Work", WorkTitle: "Work", WorkType: "novel"}, sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || stringFromMap(candidates[0], "source_url") != "https://reference.example/wiki/work" || stringFromMap(candidates[0], "evidence_excerpt") != "Alpha is a named guardian." {
		t.Fatalf("candidates=%#v", candidates)
	}
	if int64FromMap(trace, "processed_sections", -1) != 1 || trace["coverage_contract_present"] != true {
		t.Fatalf("trace=%#v", trace)
	}
	joined := fmt.Sprint(request["messages"])
	if !strings.Contains(joined, `source_ref`) || strings.Contains(joined, `equivalent_evidence`) {
		t.Fatalf("request did not use compact source references: %s", joined)
	}
}

func TestSourceDiscoveryStructuralRosterSeedsAllEvidenceAndDedupes(t *testing.T) {
	section := func(kind, headingPath, name, locator string, anchored bool) any {
		item := map[string]any{
			"section_kind": kind, "heading_path": headingPath, "excerpt": name,
			"source_url": "https://reference.example/wiki/fixture", "source_type": "community_wiki", "document_sha256": "dense-hash",
			"locator": map[string]any{"type": kind, "value": locator},
		}
		if anchored {
			item["anchors"] = []any{map[string]any{"text": name, "href": "/wiki/" + url.PathEscape(name)}}
		}
		return item
	}
	sections := []any{
		section("heading", "Index > Arin", "Arin", "h1", false), section("heading", "Index > Bera", "Bera", "h2", false),
		section("list_item", "Index > Group", "Cato", "li1", true), section("list_item", "Index > Group", "Dara", "li2", true),
		section("table_cell", "Index > Registry", "Eren", "td1", true), section("table_cell", "Index > Registry", "Fara", "td2", true),
		section("table_cell", "Index > Registry", "1998", "td3", false),
	}
	first := sourceDiscoveryStructuralRosterCandidates(sections)
	if len(first) != 2 {
		t.Fatalf("seeded=%d want=2 candidates=%#v", len(first), first)
	}
	for _, candidate := range first {
		if stringFromMap(candidate, "canonical_name") == "" || stringFromMap(candidate, "document_sha256") != "dense-hash" || len(mapFromAny(candidate["locator"])) == 0 || stringFromMap(candidate, "evidence_excerpt") == "" {
			t.Fatalf("candidate is not evidence-bound: %#v", candidate)
		}
	}
	reconciled, _ := reconcileSourceCandidates(nil, first)
	reconciled, duplicates := reconcileSourceCandidates(reconciled, sourceDiscoveryStructuralRosterCandidates(sections))
	if len(reconciled) != len(first) || duplicates != len(first) {
		t.Fatalf("repeated analysis did not dedupe: candidates=%d duplicates=%d", len(reconciled), duplicates)
	}
	for _, candidate := range reconciled {
		if len(sliceFromAny(candidate["evidence_set"])) != 1 {
			t.Fatalf("duplicate evidence was retained: %#v", candidate)
		}
	}
}

func TestSourceDiscoveryHTMLRosterUsesAnchorNameAndFullSectionEvidence(t *testing.T) {
	sections := discoveryHTMLSections([]byte(`<main><h2>Roster</h2><ul><li><a href="/a">Arin</a> keeps the eastern gate.</li><li><a href="/b">Bera</a> maps the lower hall.</li></ul></main>`))
	for _, section := range sections {
		section["source_url"] = "https://reference.example/wiki/fixture"
		section["source_type"] = "community_wiki"
		section["document_sha256"] = "anchor-hash"
	}
	candidates := sourceDiscoveryStructuralRosterCandidates(mapsToAny(sections))
	if len(candidates) != 2 || stringFromMap(candidates[0], "canonical_name") != "Arin" || stringFromMap(candidates[1], "canonical_name") != "Bera" {
		t.Fatalf("anchor roster candidates=%#v sections=%#v", candidates, sections)
	}
	if stringFromMap(candidates[0], "evidence_excerpt") != "Arin keeps the eastern gate." {
		t.Fatalf("full section evidence was not retained: %#v", candidates[0])
	}
}

func TestSourceDiscoveryStructuralRosterRejectsMetadataLinks(t *testing.T) {
	body := []byte(`<main>
		<h2>Roster</h2><ul><li><a href="/wiki/Arin">Arin</a></li><li><a href="/w/Bera">Bera</a></li></ul>
		<h2>Metadata</h2><ul>
			<li><a href="/wiki/Category%3A2012">2012 novel</a></li>
			<li><a href="/wiki/poster.jpg">poster.jpg</a></li>
			<li><a href="https://publisher.example/work">Publisher</a></li>
			<li><a href="#cite-note-1">↑</a></li>
			<li><a href="/w/index.php?title=work&amp;action=edit">Edit</a></li>
		</ul>
		<table><tr><td><a href="/wiki/Publisher">Publisher</a></td><td><a href="/wiki/Broadcaster">Broadcaster</a></td></tr></table>
	</main>`)
	sections := discoveryHTMLSections(body)
	for _, section := range sections {
		section["source_url"] = "https://reference.example/wiki/fixture"
		section["source_type"] = "community_wiki"
		section["document_sha256"] = "metadata-hash"
	}
	candidates := sourceDiscoveryStructuralRosterCandidates(mapsToAny(sections))
	if len(candidates) != 2 || stringFromMap(candidates[0], "canonical_name") != "Arin" || stringFromMap(candidates[1], "canonical_name") != "Bera" {
		t.Fatalf("metadata links leaked into roster: %#v", candidates)
	}
	foundHref := false
	for _, section := range sections {
		for _, anchor := range sliceMapFromAny(section["anchors"]) {
			if stringFromMap(anchor, "text") == "Arin" && stringFromMap(anchor, "href") == "/wiki/Arin" {
				foundHref = true
			}
		}
	}
	if !foundHref {
		t.Fatalf("parser did not retain anchor href and text: %#v", sections)
	}
}

func TestSourceDiscoveryStructuralRosterRepairRemovesOnlyUnpromotedSeeds(t *testing.T) {
	candidates := []map[string]any{
		{"kind": "entity", "canonical_name": "Category label", "provenance": "deterministic_structural_roster.v1"},
		{"kind": "character", "canonical_name": "Arin", "provenance": "model_derived_candidate", "structural_provenance": "deterministic_structural_roster.v1"},
		{"kind": "setting", "statement": "The gate opens at dawn.", "provenance": "model_derived_candidate"},
	}
	kept, removed := removeSourceDiscoveryStructuralRosterCandidates(candidates)
	if len(removed) != 1 || stringFromMap(removed[0], "canonical_name") != "Category label" {
		t.Fatalf("removed=%#v", removed)
	}
	if len(kept) != 2 || stringFromMap(kept[0], "canonical_name") != "Arin" || stringFromMap(kept[1], "kind") != "setting" {
		t.Fatalf("kept=%#v", kept)
	}
}

func TestSourceDiscoveryTypedDenseProductionExtractionStagesAllCategoriesBounded(t *testing.T) {
	llmCalls := 0
	var request map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalls++
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		content := map[string]any{
			"records": map[string]any{
				"entities": []any{
					map[string]any{"canonical_name": "Arin", "entity_type": "character", "source_ref": "s1", "work_identity_status": "matched", "canon_scope_status": "in_world"},
					map[string]any{"canonical_name": "Bera", "entity_type": "location", "source_ref": "s2", "work_identity_status": "matched", "canon_scope_status": "in_world"},
				},
				"world_rules":     []any{map[string]any{"statement": "The gate opens at first light.", "source_ref": "s3", "work_identity_status": "matched", "canon_scope_status": "in_world"}},
				"timeline_events": []any{map[string]any{"label": "The eastern gate opens.", "source_ref": "s4", "work_identity_status": "matched", "canon_scope_status": "in_world"}},
				"relations":       []any{map[string]any{"subject": "Arin", "relation": "guards", "target": "Bera", "source_ref": "s5", "work_identity_status": "matched", "canon_scope_status": "in_world"}},
				"facts":           []any{map[string]any{"statement": "Arin carries a brass seal.", "source_ref": "s5", "work_identity_status": "matched", "canon_scope_status": "in_world"}},
			},
			"follow_up_queries": []any{}, "examined_source_refs": []any{"s1", "s2", "s3", "s4", "s5"}, "incomplete_source_refs": []any{},
		}
		encoded, _ := json.Marshal(content)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}}})
	}))
	defer provider.Close()

	makeSection := func(kind, excerpt, locator string) any {
		item := map[string]any{
			"section_kind": kind, "heading_path": "Guide > Roster", "excerpt": excerpt,
			"source_url": "https://reference.example/wiki/fixture", "source_type": "community_wiki", "document_sha256": "dense-hash",
			"locator": map[string]any{"type": kind, "value": locator},
		}
		if kind == "list_item" {
			item["anchors"] = []any{map[string]any{"text": excerpt, "href": "/wiki/" + url.PathEscape(excerpt)}}
		}
		return item
	}
	sections := []any{
		makeSection("list_item", "Arin", "1"), makeSection("list_item", "Bera", "2"),
		makeSection("prose", "The gate opens at first light.", "3"), makeSection("prose", "The eastern gate opens.", "4"),
		makeSection("prose", "Arin guards Bera and carries a brass seal.", "5"),
	}
	candidates, _, trace, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000, MaxTokens: 5000,
	}, store.SourceDiscoveryInput{WorkQuery: "Fixture", WorkTitle: "Fixture", WorkType: "novel"}, map[string]any{"section_candidates": sections})
	if err != nil {
		t.Fatal(err)
	}
	if llmCalls != 1 || int64FromMap(trace, "llm_call_count", 0) != 1 || int64FromMap(trace, "structural_roster_candidates", 0) != 2 {
		t.Fatalf("calls=%d trace=%#v", llmCalls, trace)
	}
	requestText := fmt.Sprint(request["messages"])
	if !strings.Contains(requestText, "world_rules") || !strings.Contains(requestText, "timeline_events") || !strings.Contains(requestText, "heading_path") {
		t.Fatalf("typed document context missing from request: %s", requestText)
	}
	kinds := map[string]int{}
	for _, candidate := range candidates {
		kinds[stringFromMap(candidate, "kind")]++
		if len(sliceFromAny(candidate["evidence_set"])) == 0 {
			t.Fatalf("candidate lacks evidence set: %#v", candidate)
		}
	}
	for _, kind := range []string{"character", "location", "setting", "event", "relation", "claim"} {
		if kinds[kind] == 0 {
			t.Fatalf("missing kind %q: %#v", kind, kinds)
		}
	}

	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	counts, err := persistSourceDiscoveryCandidates(context.Background(), fake, "job-dense", "work-1", "continuity-1", candidates, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if counts["entities"] != 2 || len(fake.entities) != 2 || counts["timeline"] != 1 || counts["claims"] != 3 || len(fake.timeline) != 1 || len(fake.claims) != 3 {
		t.Fatalf("staging counts=%#v entities=%#v timeline=%#v claims=%#v", counts, fake.entities, fake.timeline, fake.claims)
	}
	for _, entity := range fake.entities {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(entity.MetadataJSON), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["structural_provenance"] != "deterministic_structural_roster.v1" {
			t.Fatalf("structural provenance missing from persisted entity: %#v", metadata)
		}
	}
	var relationMetadata map[string]any
	for _, claim := range fake.claims {
		if claim.ClaimType == "relation" {
			if err := json.Unmarshal([]byte(claim.MetadataJSON), &relationMetadata); err != nil {
				t.Fatal(err)
			}
		}
	}
	relation := mapFromAny(relationMetadata["relation"])
	if relation["subject"] != "Arin" || relation["predicate"] != "guards" || relation["target"] != "Bera" || stringFromMap(relation, "subject_entity_id") == "" || stringFromMap(relation, "target_entity_id") == "" {
		t.Fatalf("relation metadata lost typed identity: %#v", relationMetadata)
	}
}

func TestSourceDiscoveryEntityPersistenceIDSurvivesGenericToTypedClassification(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	evidence := []any{map[string]any{
		"source_url": "https://reference.example/wiki/fixture", "source_type": "community_wiki", "document_sha256": "same-hash",
		"locator": map[string]any{"type": "li", "value": "1"}, "evidence_excerpt": "Arin",
	}}
	for _, candidate := range []map[string]any{
		{"kind": "entity", "entity_type": "other", "canonical_name": "Arin", "evidence_set": evidence},
		{"kind": "character", "canonical_name": "Arin", "evidence_set": evidence},
	} {
		if _, err := persistSourceDiscoveryCandidates(context.Background(), fake, "job", "work-1", "continuity-1", []map[string]any{candidate}, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(fake.entities) != 2 || fake.entities[0].EntityID != fake.entities[1].EntityID || fake.entities[1].EntityType != "character" {
		t.Fatalf("classification changed persistence identity: %#v", fake.entities)
	}
}

func TestSourceCandidateExaminedPrefixStopsAtFirstUnfinishedSection(t *testing.T) {
	parsed := map[string]any{
		"examined_source_refs":   []any{"s1", "s3"},
		"incomplete_source_refs": []any{"s2"},
	}
	processed, present := sourceCandidateExaminedPrefix(parsed, 3)
	if !present || processed != 1 {
		t.Fatalf("processed=%d present=%v", processed, present)
	}
	legacyProcessed, legacyPresent := sourceCandidateExaminedPrefix(map[string]any{}, 3)
	if legacyPresent || legacyProcessed != 3 {
		t.Fatalf("legacy processed=%d present=%v", legacyProcessed, legacyPresent)
	}
}

func TestSourceCandidateExtractionCountsAttemptedIncompleteSections(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		content := `{"candidates":[],"follow_up_queries":[],"examined_source_refs":[],"incomplete_source_refs":["s1","s2"]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer provider.Close()

	sections := []any{
		map[string]any{"source_url": "https://one.example/wiki", "document_sha256": "hash-1", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "First source section."},
		map[string]any{"source_url": "https://two.example/wiki", "document_sha256": "hash-2", "locator": map[string]any{"type": "p", "value": "2"}, "excerpt": "Second source section."},
	}
	_, _, trace, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000, MaxTokens: 512,
	}, store.SourceDiscoveryInput{WorkQuery: "Work", WorkTitle: "Work", WorkType: "novel"}, map[string]any{"section_candidates": sections})
	if err != nil {
		t.Fatal(err)
	}
	if int64FromMap(trace, "processed_sections", -1) != 0 || int64FromMap(trace, "attempted_sections", -1) != 2 {
		t.Fatalf("trace=%#v", trace)
	}
	if trace["processing_incomplete"] != true {
		t.Fatalf("trace=%#v", trace)
	}
}

func TestSourceDiscoveryDeferredSectionsRemainTraceableWithoutBlockingNextSource(t *testing.T) {
	sections := []any{
		map[string]any{"source_url": "https://one.example/wiki", "document_sha256": "hash-1", "locator": map[string]any{"type": "p", "value": "1"}, "heading_path": "Characters"},
		map[string]any{"source_url": "https://two.example/wiki", "document_sha256": "hash-2", "locator": map[string]any{"type": "li", "value": "2"}, "section_kind": "list_item"},
	}
	deferred := appendSourceDiscoveryDeferredSections(nil, sections)
	deferred = appendSourceDiscoveryDeferredSections(deferred, sections)
	if len(deferred) != 2 {
		t.Fatalf("deferred=%#v", deferred)
	}
	job := &store.SourceDiscoveryJob{Result: map[string]any{"deferred_incomplete_sections": mapsToAny(deferred)}}
	if got := sourceDiscoveryCorpusTermination(job); got != "all_sources_attempted_with_incomplete_sections" {
		t.Fatalf("termination=%q", got)
	}
}

func TestSourceCandidatePromptBudgetTracksOutputCapacity(t *testing.T) {
	if got := sourceCandidateExtractionPromptRuneBudget(completeTurnLLMConfig{MaxCompletionTokens: 1600}); got != 8000 {
		t.Fatalf("small budget=%d", got)
	}
	if got := sourceCandidateExtractionPromptRuneBudget(completeTurnLLMConfig{MaxCompletionTokens: 5000}); got != 20000 {
		t.Fatalf("medium budget=%d", got)
	}
	if got := sourceCandidateExtractionPromptRuneBudget(completeTurnLLMConfig{MaxCompletionTokens: 20000}); got != 24000 {
		t.Fatalf("bounded budget=%d", got)
	}
}

func TestSourceCandidateExtractionKeepsOnlyEvidenceBoundPendingCandidates(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		content := map[string]any{
			"candidates": []any{
				map[string]any{"kind": "entity", "name": "Mina", "work_identity_status": "matched", "source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."},
				map[string]any{"kind": "entity", "name": "Other Mina", "work_identity_status": "mismatch", "source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."},
				map[string]any{"kind": "entity", "name": "External Author", "canon_scope_status": "external_metadata", "source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."},
				map[string]any{"kind": "setting", "statement": strings.Repeat("long derived summary ", 60), "canon_scope_status": "in_world", "source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."},
				map[string]any{"kind": "claim", "statement": "Invented", "source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "This text was never present."},
			},
			"follow_up_queries": []any{map[string]any{"query": "Neutral Mina official", "domain": "entities"}},
		}
		encoded, _ := json.Marshal(content)
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "choices": []any{map[string]any{"message": map[string]any{"content": string(encoded)}}}})
	}))
	defer provider.Close()
	result := map[string]any{"section_candidates": []any{map[string]any{
		"source_url": "https://official.example/guide", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Mina is the archivist.",
	}}}
	candidates, followUps, trace, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000,
	}, store.SourceDiscoveryInput{WorkQuery: "Neutral"}, result)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0]["review_state"] != "pending" || candidates[0]["admission_eligible"] != false {
		t.Fatalf("candidates=%#v", candidates)
	}
	if len(followUps) != 1 || trace["rejected_unbound_candidates"] != 1 {
		t.Fatalf("followups=%#v trace=%#v", followUps, trace)
	}
	if trace["format_retry_count"] != 0 {
		t.Fatalf("unexpected format retry: %#v", trace)
	}
	if trace["rejected_work_identity_mismatch_candidates"] != 1 {
		t.Fatalf("identity mismatch was not rejected: %#v", trace)
	}
	if trace["rejected_external_metadata_candidates"] != 1 || trace["rejected_non_atomic_candidates"] != 1 {
		t.Fatalf("non-canon candidates were not rejected: %#v", trace)
	}
}

func TestSourceCandidateExtractionRetriesMalformedJSONOnce(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		content := "I could not format the result."
		if calls == 2 {
			content = `{"candidates":[{"kind":"entity","name":"Mina","source_url":"https://reference.example/entities/mina","locator":{"type":"p","value":"1"},"evidence_excerpt":"Mina is the archivist."}],"follow_up_queries":[]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer provider.Close()
	result := map[string]any{"section_candidates": []any{map[string]any{
		"source_url": "https://reference.example/entities/mina", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Mina is the archivist.",
	}}}
	candidates, _, trace, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "openai", APIKey: "fixture-key", Endpoint: provider.URL, Model: "fixture", TimeoutMs: 5000,
	}, store.SourceDiscoveryInput{WorkQuery: "Neutral"}, result)
	if err != nil || calls != 2 || len(candidates) != 1 || trace["format_retry_count"] != 1 {
		t.Fatalf("calls=%d candidates=%#v trace=%#v err=%v", calls, candidates, trace, err)
	}
}

func TestSourceDiscoveryCandidateExtractionUsesCommonCriticConfig(t *testing.T) {
	srv := NewServer(config.Default())
	srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider": "ollama",
		"sourceSearchPlannerApiKey":   "search-key",
		"sourceSearchPlannerEndpoint": "https://ollama.com",
		"sourceSearchPlannerModel":    "search-agent-model",
		"criticProvider":              "openai",
		"criticApiKey":                "critic-key",
		"criticEndpoint":              "https://critic.example/v1",
		"criticModel":                 "common-critic-model",
	})

	cfg := srv.sourceDiscoveryCriticConfig(nil)
	if cfg.Provider != "openai" || cfg.APIKey != "critic-key" || cfg.Endpoint != "https://critic.example/v1" || cfg.Model != "common-critic-model" {
		t.Fatalf("candidate extraction did not select common critic config: %#v", cfg)
	}
	if cfg.Model == "search-agent-model" || cfg.APIKey == "search-key" {
		t.Fatalf("candidate extraction reused source-search config: %#v", cfg)
	}
}

func TestSourceCandidateExtractionUsesOllamaNativeStructuredChat(t *testing.T) {
	var captured map[string]any
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("Ollama candidate extraction path=%q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer critic-key" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		content := `{"candidates":[{"kind":"entity","name":"Mina","source_ref":"s1","work_identity_status":"matched","canon_scope_status":"in_world"}],"follow_up_queries":[],"examined_source_refs":["s1"],"incomplete_source_refs":[]}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]any{"role": "assistant", "content": content}, "done": true})
	}))
	defer provider.Close()
	oldClient := proxyHTTPClient
	proxyHTTPClient = provider.Client()
	defer func() { proxyHTTPClient = oldClient }()

	result := map[string]any{"section_candidates": []any{map[string]any{
		"source_url": "https://reference.example/entities/mina", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Mina is the archivist.",
	}}}
	candidates, _, trace, err := runSourceCandidateExtraction(context.Background(), completeTurnLLMConfig{
		Provider: "ollama", APIKey: "critic-key", Endpoint: provider.URL + "/v1/chat/completions", Model: "critic-model", TimeoutMs: 5000,
		Temperature: 0.2, MaxTokens: 5000, ReasoningEffort: "none",
	}, store.SourceDiscoveryInput{WorkQuery: "Neutral"}, result)
	if err != nil || len(candidates) != 1 || trace["format_retry_count"] != 0 {
		t.Fatalf("candidates=%#v trace=%#v err=%v", candidates, trace, err)
	}
	if captured["model"] != "critic-model" || captured["think"] != false {
		t.Fatalf("Ollama model/reasoning mismatch: %#v", captured)
	}
	format := mapFromAny(captured["format"])
	if format["type"] != "object" || len(sliceFromAny(format["required"])) != 4 {
		t.Fatalf("Ollama structured format missing: %#v", captured)
	}
	options := mapFromAny(captured["options"])
	if options["temperature"] != float64(0.2) || options["num_predict"] != float64(5000) {
		t.Fatalf("Ollama extraction options mismatch: %#v", options)
	}
}

func TestOllamaNativeChatEndpointNormalizesSavedCriticEndpoints(t *testing.T) {
	for input, want := range map[string]string{
		"https://ollama.com":                                          "https://ollama.com/api/chat",
		"https://ollama.com/v1":                                       "https://ollama.com/api/chat",
		"https://ollama.com/v1/chat/completions":                      "https://ollama.com/api/chat",
		"https://ollama.com/api/chat":                                 "https://ollama.com/api/chat",
		"https://gateway.example/ollama/v1/chat/completions?legacy=1": "https://gateway.example/ollama/api/chat",
	} {
		got, err := ollamaNativeChatEndpoint(input)
		if err != nil || got != want {
			t.Fatalf("ollamaNativeChatEndpoint(%q)=%q, %v; want %q", input, got, err, want)
		}
	}
}

func TestSourceDiscoveryHTMLSectionsPreserveStructureAndBoundText(t *testing.T) {
	body := []byte(`<html><head><title>Fixture Guide</title><style>hidden</style></head><body><h1 id="people">People</h1><p>Mina is the archivist.</p><h2 id="guardians">Guardians</h2><ul><li>Alpha Floor Guardian</li></ul><script>secret()</script><p>Rowan guards the gate.</p></body></html>`)
	sections := discoverySections(body, "text/html")
	if title := discoveryDocumentTitleFromSections(sections); title != "Fixture Guide" {
		t.Fatalf("document title=%q", title)
	}
	if len(sections) != 6 {
		t.Fatalf("sections=%#v", sections)
	}
	joined := ""
	for _, section := range sections {
		joined += " " + section["excerpt"].(string)
	}
	if !strings.Contains(joined, "Fixture Guide") || !strings.Contains(joined, "Mina is the archivist") || strings.Contains(joined, "secret()") {
		t.Fatalf("unexpected structured extraction: %q", joined)
	}
	guardian := sections[4]
	if guardian["section_kind"] != "list_item" || guardian["heading_path"] != "People > Guardians" {
		t.Fatalf("guardian section lost heading context: %#v", guardian)
	}
	headingLocator := mapFromAny(sections[3]["locator"])
	if headingLocator["id"] != "guardians" {
		t.Fatalf("heading anchor was not retained: %#v", sections[3])
	}
}

func TestSourceDiscoveryHTMLInternalLinksStayOnSameSourceHost(t *testing.T) {
	base, _ := url.Parse("https://reference.example/work/neutral")
	body := []byte(`<a href="/entities/mina">Mina</a><a href="https://external.example/store">External</a><a href="https://second-reference.example/entities/arona#bio">Arona</a><a href="/entities/mina#duplicate">duplicate</a>`)
	links := discoveryHTMLInternalLinks(base, body)
	if !reflect.DeepEqual(links, []string{"https://reference.example/entities/mina"}) {
		t.Fatalf("links=%#v", links)
	}
	profile := sourceDiscoveryProfileForURL(base.String())
	if profile["contract"] != "source-profile.v1" || profile["profile_id"] != "generic_public_document" || profile["executable_parser"] != false {
		t.Fatalf("profile=%#v", profile)
	}
}

func TestSourceDiscoveryHonorsExplicitRobotsDenial(t *testing.T) {
	headers := http.Header{"X-Robots-Tag": []string{"noai, noindex"}}
	if !discoveryRobotsDenied(headers, []byte(`<html><body>content</body></html>`), "text/html") {
		t.Fatal("X-Robots-Tag noai must block automated analysis")
	}
	if !discoveryRobotsDenied(http.Header{}, []byte(`<meta name="robots" content="noindex"><p>content</p>`), "text/html") {
		t.Fatal("HTML robots noindex must block automated analysis")
	}
	if discoveryRobotsDenied(http.Header{}, []byte(`<p>public content</p>`), "text/html") {
		t.Fatal("public content was blocked")
	}
}

func TestSourceDiscoveryImageInventoryIsBoundedAndKeepsNoRawBytes(t *testing.T) {
	base, _ := url.Parse("https://reference.example/work/neutral")
	assets := discoveryHTMLImageAssets(base, []byte(`<img src="/images/a.png" alt="Mina portrait"><img src="https://cdn.example/b.jpg"><img src="/images/a.png"><img src="/images/c.webp"><img src="/images/d.png">`))
	if len(assets) != 3 || assets[0]["url"] != "https://reference.example/images/a.png" || assets[0]["alt"] != "Mina portrait" {
		t.Fatalf("assets=%#v", assets)
	}
}

func TestSourceDiscoveryWithoutProviderOrSourcesIsExplicitlyInsufficient(t *testing.T) {
	result, coverage, state := runSourceDiscovery(context.Background(), store.SourceDiscoveryInput{
		WorkQuery: "Neutral Work", AllowedSourceTypes: []string{"official_primary"},
	})
	if state != "insufficient_source_coverage" || result["termination_reason"] != "search_provider_required" {
		t.Fatalf("state=%q result=%#v", state, result)
	}
	if coverage["saturation"] != "insufficient_source_coverage" {
		t.Fatalf("coverage=%#v", coverage)
	}
	if result["admission_status"] != "pending" {
		t.Fatalf("provider-free result was promoted: %#v", result)
	}
}

func TestSearchProviderResultsRequireApprovedDomainPolicy(t *testing.T) {
	input := store.SourceDiscoveryInput{DomainPolicies: []store.SourceDiscoveryDomainPolicy{{
		Domain: "reference.wiki", SourceType: "community_wiki", AccessClass: "open", PolicyConfirmed: true,
	}}}
	sources, unmatched := providerSourcesFromResults(input, []sourceSearchProviderResult{
		{URL: "https://reference.wiki/wiki/guide"},
		{URL: "https://sub.reference.wiki/wiki/characters"},
		{URL: "https://unapproved.example/wiki"},
	})
	if len(sources) != 2 || unmatched != 1 {
		t.Fatalf("sources=%#v unmatched=%d", sources, unmatched)
	}
	for _, source := range sources {
		if !source.PolicyConfirmed || source.SourceType != "community_wiki" {
			t.Fatalf("provider bypassed approved domain policy: %#v", source)
		}
	}
}

func TestSourceSearchLLMRuntimeConfigAndPreviewDoNotExposeSecret(t *testing.T) {
	srv := NewServer(config.Default())
	const secret = "source-search-llm-secret"
	updated := srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider": "openai", "sourceSearchPlannerApiKey": secret,
		"sourceSearchPlannerEndpoint": "https://api.openai.com/v1", "sourceSearchPlannerModel": "fixture",
	})
	if len(updated) != 4 {
		t.Fatalf("updated=%#v", updated)
	}
	if !srv.sourceSearchLLMConfigured() {
		t.Fatal("native web-search LLM should be configured")
	}
	trace, _ := json.Marshal(srv.runtimeConfigTrace())
	if strings.Contains(string(trace), secret) {
		t.Fatalf("runtime trace leaked source search API key: %s", trace)
	}

	req := httptest.NewRequest(http.MethodPost, "/source-discovery/preview/v1", strings.NewReader(`{
		"work_query":"Neutral Work","allowed_source_types":["official_primary"]
	}`))
	rec := httptest.NewRecorder()
	srv.handleSourceDiscoveryPreviewV1(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["search_provider_configured"] != true || body["search_provider_required"] != false {
		t.Fatalf("preview=%#v", body)
	}
}

func TestSourceSearchLLMUsesOpenAINativeWebSearchWithTemperatureAndNoReasoning(t *testing.T) {
	var captured map[string]any
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode search request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []any{
				map[string]any{"type": "web_search_call", "action": map[string]any{"sources": []any{map[string]any{"type": "url", "url": "https://reference.wiki/wiki/neutral", "title": "Neutral Reference"}}}},
			},
		})
	}))
	defer provider.Close()
	oldClient := proxyHTTPClient
	proxyHTTPClient = provider.Client()
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	const secret = "planner-secret"
	updated := srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider":              "openai",
		"sourceSearchPlannerApiKey":                secret,
		"sourceSearchPlannerEndpoint":              provider.URL,
		"sourceSearchPlannerModel":                 "fixture",
		"sourceSearchPlannerTimeout":               5,
		"sourceSearchPlannerTemperature":           0.1,
		"sourceSearchPlannerMaxCompletionTokens":   512,
		"sourceSearchPlannerReasoningPreset":       "auto",
		"sourceSearchPlannerReasoningEffort":       "none",
		"sourceSearchPlannerReasoningBudgetTokens": 0,
	})
	if len(updated) != 10 {
		t.Fatalf("updated=%#v", updated)
	}
	sources, trace, err := srv.discoverSourcesWithSearchLLM(context.Background(), store.SourceDiscoveryInput{
		WorkQuery: "Neutral Work", AllowedSourceTypes: []string{"community_wiki"},
	})
	if err != nil || len(sources) != 1 || sources[0].URL != "https://reference.wiki/wiki/neutral" || sources[0].SourceType != "community_wiki" {
		t.Fatalf("sources=%#v trace=%#v err=%v", sources, trace, err)
	}
	if trace["status"] != "completed" || trace["native_web_search"] != true {
		t.Fatalf("trace=%#v", trace)
	}
	if captured["temperature"] != 0.1 {
		t.Fatalf("planner temperature=%#v body=%#v", captured["temperature"], captured)
	}
	tools := anySlice(captured["tools"])
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "web_search" {
		t.Fatalf("native web search tool missing: %#v", captured)
	}
	if _, exists := captured["reasoning"]; exists {
		t.Fatalf("reasoning none must not send OpenAI reasoning config: %#v", captured)
	}
	traceJSON, _ := json.Marshal(srv.runtimeConfigTrace())
	if strings.Contains(string(traceJSON), secret) {
		t.Fatalf("runtime trace leaked planner API key: %s", traceJSON)
	}
}

func TestSourceSearchPreviewRequiresMissingRuntimeProvider(t *testing.T) {
	srv := NewServer(config.Default())
	req := httptest.NewRequest(http.MethodPost, "/source-discovery/preview/v1", strings.NewReader(`{
		"work_query":"Neutral Work","allowed_source_types":["official_primary"]
	}`))
	rec := httptest.NewRecorder()
	srv.handleSourceDiscoveryPreviewV1(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["search_provider_required"] != true || body["search_provider_configured"] != false {
		t.Fatalf("preview=%#v", body)
	}
}

func TestNativeSourceSearchParsersKeepOnlyCitedURLs(t *testing.T) {
	openAI := parseOpenAINativeSourceSearchResults(map[string]any{"output": []any{
		map[string]any{"type": "web_search_call", "action": map[string]any{"sources": []any{map[string]any{"url": "https://official.example/guide", "title": "Guide"}}}},
		map[string]any{"type": "message", "content": []any{map[string]any{"annotations": []any{map[string]any{"type": "url_citation", "url": "https://licensed.example/data", "title": "Data"}}}}},
	}})
	if len(openAI) != 2 {
		t.Fatalf("openai=%#v", openAI)
	}
	gemini := parseGeminiNativeSourceSearchResults(map[string]any{"candidates": []any{map[string]any{
		"groundingMetadata": map[string]any{"groundingChunks": []any{map[string]any{"web": map[string]any{"uri": "https://wiki.example/work", "title": "Wiki"}}}},
	}}})
	if len(gemini) != 1 || gemini[0].URL != "https://wiki.example/work" {
		t.Fatalf("gemini=%#v", gemini)
	}
	claude := parseClaudeNativeSourceSearchResults(map[string]any{"content": []any{map[string]any{
		"type": "web_search_tool_result", "content": []any{map[string]any{"type": "web_search_result", "url": "https://secondary.example/page", "title": "Page"}},
	}}})
	if len(claude) != 1 || claude[0].URL != "https://secondary.example/page" {
		t.Fatalf("claude=%#v", claude)
	}
}

func TestOllamaSourceSearchAgentUsesConfiguredModelAndBoundedToolLoop(t *testing.T) {
	requests := []map[string]any{}
	paths := []string{}
	authorizations := []string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		var captured map[string]any
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode Ollama request: %v", err)
		}
		requests = append(requests, captured)
		response := ""
		switch len(paths) {
		case 1:
			response = `{"message":{"role":"assistant","content":"","tool_calls":[{"type":"function","function":{"name":"web_search","arguments":{"query":"Neutral Work wiki characters","max_results":3}}}]},"done":true}`
		case 2:
			response = `{"results":[{"title":"Reference Guide","url":"https://reference.wiki/wiki/neutral","content":"Character and story guide"}]}`
		case 3:
			response = `{"message":{"role":"assistant","content":"Search complete."},"done":true}`
		default:
			t.Fatalf("unexpected Ollama call %d", len(paths))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(response)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider": "ollama", "sourceSearchPlannerApiKey": "ollama-key",
		"sourceSearchPlannerEndpoint": "https://ollama.com", "sourceSearchPlannerModel": "qwen3:4b",
		"sourceSearchPlannerTemperature": 0.25, "sourceSearchPlannerMaxCompletionTokens": 768,
		"sourceSearchPlannerReasoningEffort": "medium",
	})
	if !srv.sourceSearchLLMConfigured() {
		t.Fatal("Ollama search agent should be configured with a model")
	}
	sources, diagnostics, err := srv.discoverSourcesWithSearchLLM(context.Background(), store.SourceDiscoveryInput{
		WorkQuery: "Neutral Work", AllowedSourceTypes: []string{"community_wiki"},
	})
	if err != nil || len(sources) != 1 || diagnostics["status"] != "completed" {
		t.Fatalf("sources=%#v diagnostics=%#v err=%v", sources, diagnostics, err)
	}
	if !reflect.DeepEqual(paths, []string{"/api/chat", "/api/web_search", "/api/chat"}) {
		t.Fatalf("paths=%#v", paths)
	}
	for _, authorization := range authorizations {
		if authorization != "Bearer ollama-key" {
			t.Fatalf("authorizations=%#v", authorizations)
		}
	}
	first := requests[0]
	if first["model"] != "qwen3:4b" || first["think"] != "medium" || first["stream"] != false {
		t.Fatalf("chat request=%#v", first)
	}
	options := mapFromAny(first["options"])
	if options["temperature"] != 0.25 || options["num_predict"] != float64(768) {
		t.Fatalf("options=%#v", options)
	}
	tools := sliceFromAny(first["tools"])
	if len(tools) != 1 || stringFromMap(mapFromAny(mapFromAny(tools[0])["function"]), "name") != "web_search" {
		t.Fatalf("tools=%#v", tools)
	}
	searchRequest := requests[1]
	if searchRequest["max_results"] != float64(3) || stringFromMap(searchRequest, "query") != "Neutral Work wiki characters" {
		t.Fatalf("search request=%#v", searchRequest)
	}
	secondMessages := sliceFromAny(requests[2]["messages"])
	lastMessage := mapFromAny(secondMessages[len(secondMessages)-1])
	if lastMessage["role"] != "tool" || lastMessage["tool_name"] != "web_search" || !strings.Contains(stringFromMap(lastMessage, "content"), "reference.wiki/wiki/neutral") {
		t.Fatalf("messages=%#v", secondMessages)
	}
}

func TestOllamaSourceSearchAgentRequiresModelAndToolCall(t *testing.T) {
	srv := NewServer(config.Default())
	srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider": "ollama", "sourceSearchPlannerApiKey": "ollama-key",
		"sourceSearchPlannerEndpoint": "https://ollama.com", "sourceSearchPlannerModel": "",
	})
	if srv.sourceSearchLLMConfigured() {
		t.Fatal("Ollama search agent must require a model")
	}

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":{"role":"assistant","content":"I answered from memory."},"done":true}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	_, err := executeOllamaSourceSearchAgent(context.Background(), completeTurnLLMConfig{
		Provider: "ollama", APIKey: "ollama-key", Endpoint: "https://ollama.com", Model: "qwen3:4b", TimeoutMs: 5000,
	}, "Find sources.", "Neutral Work")
	if err == nil || !strings.Contains(err.Error(), "no web_search tool call") {
		t.Fatalf("err=%v", err)
	}
}

func TestOllamaSourceSearchAgentBoundsToolCalls(t *testing.T) {
	searchCalls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/api/chat":
			body = `{"message":{"role":"assistant","tool_calls":[
				{"function":{"name":"web_search","arguments":{"query":"one"}}},
				{"function":{"name":"web_search","arguments":{"query":"two"}}},
				{"function":{"name":"web_search","arguments":{"query":"three"}}},
				{"function":{"name":"web_search","arguments":{"query":"four"}}}
			]},"done":true}`
		case "/api/web_search":
			searchCalls++
			body = `{"results":[{"title":"Repeated","url":"https://official.example/repeated","content":"Evidence"}]}`
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	results, err := executeOllamaSourceSearchAgent(context.Background(), completeTurnLLMConfig{
		Provider: "ollama", APIKey: "ollama-key", Endpoint: "https://ollama.com", Model: "qwen3:4b", TimeoutMs: 5000,
	}, "Find sources.", "Neutral Work")
	if err != nil {
		t.Fatal(err)
	}
	if searchCalls != ollamaSourceSearchMaxToolCalls || len(results) != 1 {
		t.Fatalf("searchCalls=%d results=%#v", searchCalls, results)
	}
}

func TestSourceSearchKeepsOnlyWikiArticleURLs(t *testing.T) {
	results := []sourceSearchProviderResult{
		{URL: "https://reference.wiki/wiki/work", Title: "Wiki host"},
		{URL: "https://reference.example/wiki/work", Title: "Wiki path"},
		{URL: "https://reference.example/w/work", Title: "Short wiki path"},
		{URL: "https://store.example/apps/work", Title: "Store"},
		{URL: "http://insecure.example/page", Title: "Insecure"},
		{URL: "https://user:secret@example.com/page", Title: "Credentials"},
	}
	filtered, excluded := excludeSourceSearchResults(results)
	if len(filtered) != 3 || filtered[0].URL != "https://reference.wiki/wiki/work" || filtered[2].URL != "https://reference.example/w/work" {
		t.Fatalf("filtered=%#v", filtered)
	}
	if len(excluded) != 3 || excluded[0]["reason"] != "not_wiki_source" {
		t.Fatalf("excluded=%#v", excluded)
	}
	for i, item := range excluded[1:] {
		if item["reason"] != "invalid_or_non_https_source_url" {
			t.Fatalf("excluded[%d]=%#v", i+1, item)
		}
	}
}

func TestSourceSearchCanonicalizationOnlyRemovesFragmentsAndDeduplicates(t *testing.T) {
	results := []sourceSearchProviderResult{}
	seen := map[string]bool{}
	first := "https://reference.example/work?edition=first#section"
	results = appendSourceSearchResult(results, seen, first, "First")
	results = appendSourceSearchResult(results, seen, "https://reference.example/work?edition=first#other", "Duplicate")

	if len(results) != 1 {
		t.Fatalf("results=%#v", results)
	}
	wantURL := "https://reference.example/work?edition=first"
	if results[0].URL != wantURL || results[0].OriginalURL != first {
		t.Fatalf("result=%#v wantURL=%q", results[0], wantURL)
	}
	diagnostics := sourceSearchRewriteDiagnostics(results)
	if len(diagnostics) != 1 || diagnostics[0]["from"] != first || diagnostics[0]["to"] != wantURL || diagnostics[0]["reason"] != "url_canonicalized" {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestSourceCandidateExtractionFailureClassifiesBlockedAndFailed(t *testing.T) {
	missing := sourceCandidateExtractionFailure(errors.New("critic provider configuration is required for candidate extraction"))
	if missing["status"] != "blocked" || missing["code"] != "extractor_config_missing" {
		t.Fatalf("missing=%#v", missing)
	}
	failed := sourceCandidateExtractionFailure(errors.New("upstream unavailable"))
	if failed["status"] != "failed" || failed["code"] != "extractor_request_failed" {
		t.Fatalf("failed=%#v", failed)
	}
}

func TestNativeSourceSearchEndpointRejectsChatCompletionsPath(t *testing.T) {
	err := validateNativeSourceSearchEndpoint("openai", "https://example.com/v1/chat/completions")
	if err == nil || !strings.Contains(err.Error(), "chat/completions") {
		t.Fatalf("chat completions endpoint error=%v", err)
	}
	if err := validateNativeSourceSearchEndpoint("openai", "https://api.openai.com/v1"); err != nil {
		t.Fatalf("OpenAI API base endpoint rejected: %v", err)
	}
	if err := validateNativeSourceSearchEndpoint("ollama", "https://ollama.com/api/chat"); err == nil || !strings.Contains(err.Error(), "API base endpoint") {
		t.Fatalf("Ollama chat endpoint error=%v", err)
	}
}

func TestSourceSearchNoCitedResultsHasDistinctTermination(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"output":[{"type":"message","content":[{"type":"output_text","text":"No cited sources"}]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	srv := NewServer(config.Default())
	srv.updateRuntimeConfig(map[string]any{
		"sourceSearchPlannerProvider": "openai", "sourceSearchPlannerApiKey": "fixture-key",
		"sourceSearchPlannerEndpoint": "https://provider.example/v1", "sourceSearchPlannerModel": "fixture",
	})
	input := store.SourceDiscoveryInput{WorkQuery: "Neutral Work", AllowedSourceTypes: []string{"official_primary"}}
	sources, diagnostics, err := srv.discoverSourcesWithSearchLLM(context.Background(), input)
	if err != nil || len(sources) != 0 || diagnostics["status"] != "completed_no_results" {
		t.Fatalf("sources=%#v diagnostics=%#v err=%v", sources, diagnostics, err)
	}
	result := map[string]any{"termination_reason": "search_provider_required"}
	applySourceSearchTermination(result, input, diagnostics)
	if result["termination_reason"] != "search_provider_no_cited_results" {
		t.Fatalf("result=%#v", result)
	}
}

func TestSearchProviderWikiResultsUseWikiSourceTypeWithoutDomainPolicy(t *testing.T) {
	results := []sourceSearchProviderResult{{URL: "https://reference-one.wiki/wiki/work"}, {URL: "https://reference-two.example/wiki/work"}}
	sources, unmatched := providerSourcesFromResults(store.SourceDiscoveryInput{
		AllowedSourceTypes: []string{"official_primary"},
	}, results)
	if len(sources) != 2 || unmatched != 0 {
		t.Fatalf("sources=%#v unmatched=%d", sources, unmatched)
	}
	for _, source := range sources {
		if source.SourceType != "community_wiki" || source.AccessClass != "public_web" || !source.PolicyConfirmed {
			t.Fatalf("unexpected pending acquisition classification: %#v", source)
		}
	}
}

func TestSourceDiscoveryPipelineDoesNotTreatMirroredFollowUpAsIndependent(t *testing.T) {
	searchCalls := []string{}
	fetchCalls := []string{}
	extractCalls := 0
	deps := sourceDiscoveryPipelineDeps{
		search: func(_ context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
			searchCalls = append(searchCalls, input.WorkQuery)
			url := "https://reference-one.example/entities/mina"
			if len(searchCalls) == 2 {
				url = "https://reference-two.example/entities/mina"
			}
			return []store.SourceDiscoverySource{{URL: url, SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true}}, map[string]any{"status": "completed", "result_count": 1, "approved_source_count": 1}, nil
		},
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			fetchCalls = append(fetchCalls, source.URL)
			hash := "hash-alpha"
			if strings.Contains(source.URL, "reference-two") {
				hash = "hash-reference-two"
			}
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": hash}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Mina is the archivist."}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			extractCalls++
			section := mapFromAny(sliceFromAny(result["section_candidates"])[0])
			candidate := map[string]any{"kind": "entity", "name": "Mina", "source_url": section["source_url"], "document_sha256": section["document_sha256"], "locator": section["locator"], "evidence_excerpt": section["excerpt"], "review_state": "pending", "admission_eligible": false}
			followUps := []map[string]any{}
			if extractCalls == 1 {
				followUps = []map[string]any{{"query": "Neutral Mina wiki", "domain": "entities"}}
			}
			return []map[string]any{candidate}, followUps, map[string]any{"status": "completed"}, nil
		},
	}
	_, result, coverage, state := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{}, store.SourceDiscoveryInput{
		WorkQuery: "Neutral", AllowedSourceTypes: []string{"community_wiki"}, RequestedDomains: []string{"identity", "entities"},
	}, deps)
	if !reflect.DeepEqual(searchCalls, []string{"Neutral", "Neutral Mina wiki"}) || len(fetchCalls) != 2 || extractCalls != 1 {
		t.Fatalf("search=%#v fetch=%#v extract=%d", searchCalls, fetchCalls, extractCalls)
	}
	candidateMaps := sliceMapFromAny(result["discovered_candidates"])
	candidates := mapsToAny(candidateMaps)
	if len(candidates) != 1 {
		t.Fatalf("candidates=%#v result=%#v", candidates, result)
	}
	merged := mapFromAny(candidates[0])
	if merged["independent_source_count"] != 1 || len(sliceFromAny(merged["evidence_set"])) != 2 {
		t.Fatalf("merged=%#v", merged)
	}
	if result["termination_reason"] != "coverage_saturation" || state != "ready_for_admission" || coverage["saturation"] != "coverage_saturation" {
		t.Fatalf("state=%q result=%#v coverage=%#v", state, result, coverage)
	}
	deltas := sliceMapFromAny(result["coverage_delta"])
	if len(deltas) < 2 || deltas[1]["new_sections"] != 0 || result["duplicate_analysis_sections"] != 1 {
		t.Fatalf("deltas=%#v", deltas)
	}
}

func TestSourceDiscoveryReconciliationPreservesDirectConflicts(t *testing.T) {
	candidates := []map[string]any{
		{"kind": "claim", "subject": "Mina role", "statement": "Mina is a teacher.", "independent_source_count": 1},
		{"kind": "claim", "subject": "Mina role", "statement": "Mina is a student.", "independent_source_count": 1},
	}
	conflicts, uncertainties := sourceDiscoveryReconciliation(candidates)
	if len(conflicts) != 1 || len(uncertainties) != 2 || conflicts[0]["code"] != "direct_assertion_conflict" {
		t.Fatalf("conflicts=%#v uncertainties=%#v", conflicts, uncertainties)
	}
}

func TestSourceDiscoveryMirrorHashDoesNotIncreaseIndependentEvidence(t *testing.T) {
	evidence := []map[string]any{
		{"source_url": "https://reference-one.example/entities/mina", "document_sha256": "same-hash"},
		{"source_url": "https://mirror.example/wiki/Mina", "document_sha256": "same-hash"},
	}
	if count := sourceIndependentHostCount(evidence); count != 1 {
		t.Fatalf("independent count=%d", count)
	}
	lineage := sourceDiscoverySourceLineage([]any{
		map[string]any{"final_url": evidence[0]["source_url"], "document_sha256": "same-hash"},
		map[string]any{"final_url": evidence[1]["source_url"], "document_sha256": "same-hash"},
	})
	if len(lineage) != 1 || lineage[0]["relation"] != "mirror_or_repost" {
		t.Fatalf("lineage=%#v", lineage)
	}
}

func TestSourceDiscoveryMirroredExcerptDoesNotBecomeIndependentByChangingHost(t *testing.T) {
	evidence := []map[string]any{
		{"source_url": "https://one.example/wiki", "document_sha256": "hash-one", "evidence_excerpt": "The same copied sentence."},
		{"source_url": "https://two.example/wiki", "document_sha256": "hash-two", "evidence_excerpt": "  The SAME copied sentence. "},
		{"source_url": "https://three.example/wiki", "document_sha256": "hash-three", "evidence_excerpt": "An independently worded account of the fact."},
	}
	if count := sourceIndependentHostCount(evidence); count != 2 {
		t.Fatalf("independent count=%d, want 2", count)
	}
}

type sourceDiscoveryAdmissionFake struct {
	store.ReferenceLibraryStore
	documents map[string]*store.ReferenceDocument
	timeline  []*store.ReferenceTimelineNode
	entities  []*store.ReferenceEntity
	aliases   []*store.ReferenceEntityAlias
	claims    []*store.ReferenceClaim
	reviews   []string
}

func (f *sourceDiscoveryAdmissionFake) UpsertReferenceTimelineNode(_ context.Context, item *store.ReferenceTimelineNode) error {
	f.timeline = append(f.timeline, item)
	return nil
}

func (f *sourceDiscoveryAdmissionFake) GetReferenceWork(context.Context, string) (*store.ReferenceWork, error) {
	return &store.ReferenceWork{WorkID: "work-1", Title: "Neutral"}, nil
}
func (f *sourceDiscoveryAdmissionFake) ListReferenceContinuities(context.Context, string) ([]store.ReferenceContinuity, error) {
	return []store.ReferenceContinuity{{ContinuityID: "continuity-1", WorkID: "work-1"}}, nil
}
func (f *sourceDiscoveryAdmissionFake) GetReferenceDocument(_ context.Context, id string) (*store.ReferenceDocument, error) {
	if item := f.documents[id]; item != nil {
		return item, nil
	}
	return nil, store.ErrNotFound
}
func (f *sourceDiscoveryAdmissionFake) SaveReferenceDocument(_ context.Context, item *store.ReferenceDocument) error {
	if f.documents == nil {
		f.documents = map[string]*store.ReferenceDocument{}
	}
	f.documents[item.DocumentID] = item
	return nil
}
func (f *sourceDiscoveryAdmissionFake) UpdateReferenceDocumentSource(_ context.Context, item *store.ReferenceDocument) error {
	if f.documents == nil || f.documents[item.DocumentID] == nil {
		return store.ErrNotFound
	}
	copy := *item
	copy.ImportStatus = f.documents[item.DocumentID].ImportStatus
	f.documents[item.DocumentID] = &copy
	return nil
}
func (f *sourceDiscoveryAdmissionFake) UpsertReferenceEntity(_ context.Context, item *store.ReferenceEntity) error {
	f.entities = append(f.entities, item)
	return nil
}
func (f *sourceDiscoveryAdmissionFake) UpsertReferenceEntityAlias(_ context.Context, item *store.ReferenceEntityAlias) error {
	f.aliases = append(f.aliases, item)
	return nil
}
func (f *sourceDiscoveryAdmissionFake) UpsertReferenceClaim(_ context.Context, item *store.ReferenceClaim) error {
	f.claims = append(f.claims, item)
	return nil
}
func (f *sourceDiscoveryAdmissionFake) UpdateReferenceCandidateReview(_ context.Context, _ string, kind, id, status, source, _ string) error {
	f.reviews = append(f.reviews, strings.Join([]string{kind, id, status, source}, ":"))
	return nil
}

func TestSourceDiscoveryAdmissionWritesOnlyEligibleEvidenceBatch(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	evidence := []any{map[string]any{"source_url": "https://reference.example/entities/mina", "source_type": "discovered_public", "document_sha256": "hash-1", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."}}
	candidates := []map[string]any{
		{"kind": "entity", "name": "Mina", "evidence_excerpt": "Mina is the archivist.", "evidence_set": evidence, "independent_source_count": 2},
		{"kind": "claim", "subject": "Mina", "statement": "Mina is the archivist.", "evidence_set": evidence, "independent_source_count": 2},
	}
	counts, err := admitSourceDiscoveryCandidates(context.Background(), fake, "job-1", "work-1", "continuity-1", candidates)
	if err != nil || counts["documents"] != 1 || counts["entities"] != 1 || counts["claims"] != 1 || len(fake.reviews) != 2 {
		t.Fatalf("counts=%#v reviews=%#v err=%v", counts, fake.reviews, err)
	}
	for _, review := range fake.reviews {
		if !strings.Contains(review, ":approved:evidence_validated_batch") {
			t.Fatalf("review=%q", review)
		}
	}
	for _, document := range fake.documents {
		if document.SourceType != "discovered_public" {
			t.Fatalf("admission replaced source type: %#v", document)
		}
	}
}

func TestSourceDiscoveryEventPersistsAsTimelineWithoutInventedOrder(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	evidence := []any{map[string]any{
		"source_url": "https://reference.example/wiki/event", "source_type": "community_wiki",
		"document_sha256": "event-hash", "locator": map[string]any{"type": "p", "value": "1"},
		"evidence_excerpt": "The gate opened after the service ended.",
	}}
	candidates := []map[string]any{{
		"kind": "event", "statement": "The gate opened after the service ended.",
		"evidence_set": evidence, "independent_source_count": 2,
	}}
	counts, err := admitSourceDiscoveryCandidates(context.Background(), fake, "job-event", "work-1", "continuity-1", candidates)
	if err != nil {
		t.Fatal(err)
	}
	if counts["timeline"] != 1 || counts["claims"] != 0 || len(fake.timeline) != 1 {
		t.Fatalf("counts=%#v timeline=%#v claims=%#v", counts, fake.timeline, fake.claims)
	}
	node := fake.timeline[0]
	if node.Ordinal != 0 || node.NodeKind != "event" || node.ReviewStatus != "pending" {
		t.Fatalf("node=%#v", node)
	}
	if !strings.Contains(node.MetadataJSON, `"chronology_status":"unknown"`) {
		t.Fatalf("metadata=%s", node.MetadataJSON)
	}
	if len(fake.reviews) != 1 || !strings.HasPrefix(fake.reviews[0], "timeline:") {
		t.Fatalf("reviews=%#v", fake.reviews)
	}
}

func TestSourceDiscoveryStagesEvidenceBoundCandidatesWithoutApprovingThem(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	evidence := []any{map[string]any{"source_url": "https://reference.wiki/wiki/mina", "source_type": "community_wiki", "document_sha256": "hash-stage", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina is the archivist."}}
	candidates := []map[string]any{
		{"kind": "entity", "name": "Mina", "evidence_excerpt": "Mina is the archivist.", "evidence_set": evidence, "independent_source_count": 1},
		{"kind": "claim", "subject": "Mina", "statement": "Mina is the archivist.", "evidence_set": evidence, "independent_source_count": 1},
	}
	counts, err := stageSourceDiscoveryCandidates(context.Background(), fake, "job-stage", "work-1", "continuity-1", candidates)
	if err != nil || counts["documents"] != 1 || counts["entities"] != 1 || counts["claims"] != 1 {
		t.Fatalf("counts=%#v err=%v", counts, err)
	}
	if len(fake.reviews) != 0 || fake.entities[0].ReviewStatus != "pending" || fake.claims[0].ReviewStatus != "pending" {
		t.Fatalf("reviews=%#v entities=%#v claims=%#v", fake.reviews, fake.entities, fake.claims)
	}
	for _, document := range fake.documents {
		if document.ImportStatus != "pending" || document.SourceType != "community_wiki" {
			t.Fatalf("document=%#v", document)
		}
	}
}

func TestSourceDiscoveryStagesFetchedBodiesOnceAndLinksDerivedFacts(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	rawBody := "<html><body><p>The Great Archive is an in-world location.</p></body></html>"
	hashBytes := sha256.Sum256([]byte(rawBody))
	hash := hex.EncodeToString(hashBytes[:])
	retained := []map[string]any{
		{"document_sha256": hash, "raw_text": rawBody, "source_url": "https://one.example/wiki/archive", "source_type": "community_wiki", "media_type": "text/html"},
		{"document_sha256": hash, "raw_text": rawBody, "source_url": "https://mirror.example/wiki/archive", "source_type": "community_wiki", "media_type": "text/html"},
	}
	evidence := []any{map[string]any{"source_url": "https://one.example/wiki/archive", "source_type": "community_wiki", "document_sha256": hash, "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "The Great Archive is an in-world location."}}
	candidates := []map[string]any{
		{"kind": "location", "name": "The Great Archive", "evidence_set": evidence, "independent_source_count": 1},
		{"kind": "claim", "statement": "The Great Archive is an in-world location.", "evidence_set": evidence, "independent_source_count": 1},
	}

	counts, err := stageSourceDiscoveryResult(context.Background(), fake, "job-retain", "work-1", "continuity-1", retained, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if counts["documents"] != 1 || counts["source_observations"] != 2 || counts["entities"] != 1 || counts["claims"] != 1 {
		t.Fatalf("counts=%#v", counts)
	}
	if len(fake.documents) != 1 || len(fake.entities) != 1 || len(fake.claims) != 1 {
		t.Fatalf("documents=%d entities=%d claims=%d", len(fake.documents), len(fake.entities), len(fake.claims))
	}
	for _, document := range fake.documents {
		if document.RawRetention != "full" || document.RawText != rawBody || document.ContentHash != hash || document.ImportStatus != "pending" {
			t.Fatalf("retained document=%#v", document)
		}
		if !strings.Contains(document.ProvenanceJSON, "https://one.example/wiki/archive") || !strings.Contains(document.ProvenanceJSON, "https://mirror.example/wiki/archive") || !strings.Contains(document.ProvenanceJSON, "local_only_not_packaged") {
			t.Fatalf("provenance=%s", document.ProvenanceJSON)
		}
		if fake.claims[0].DocumentID != document.DocumentID {
			t.Fatalf("claim document_id=%q retained document_id=%q", fake.claims[0].DocumentID, document.DocumentID)
		}
	}
}

func TestSourceDiscoveryUpgradesExistingMetadataOnlyDocumentWithFetchedBody(t *testing.T) {
	rawBody := "<html><body><p>Retained canon source.</p></body></html>"
	hashBytes := sha256.Sum256([]byte(rawBody))
	hash := hex.EncodeToString(hashBytes[:])
	documentID := referenceStableID("source-discovery-document", "work-1", "continuity-1", hash)
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{
		documentID: {
			DocumentID: documentID, WorkID: "work-1", ContinuityID: "continuity-1", ContentHash: hash,
			SourceType: "community_wiki", SourceURI: "https://reference.example/wiki", RawRetention: "none", ImportStatus: "pending",
		},
	}}
	counts, _, err := stageSourceDiscoveryDocuments(context.Background(), fake, "job-upgrade", "work-1", "continuity-1", []map[string]any{{
		"document_sha256": hash, "raw_text": rawBody, "source_url": "https://reference.example/wiki", "source_type": "community_wiki", "media_type": "text/html",
	}})
	if err != nil {
		t.Fatal(err)
	}
	document := fake.documents[documentID]
	if counts["documents"] != 0 || counts["documents_upgraded"] != 1 || counts["documents_retained"] != 1 {
		t.Fatalf("counts=%#v", counts)
	}
	if document.RawRetention != "full" || document.RawText != rawBody || document.ImportStatus != "pending" {
		t.Fatalf("upgraded document=%#v", document)
	}
}

func TestSourceDiscoveryRetainsFetchedBodyWhenExtractionReturnsNoCandidates(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	rawBody := "<html><body><p>Fetched but no candidate accepted.</p></body></html>"
	hashBytes := sha256.Sum256([]byte(rawBody))
	hash := hex.EncodeToString(hashBytes[:])
	counts, err := stageSourceDiscoveryResult(context.Background(), fake, "job-empty", "work-1", "continuity-1", []map[string]any{{
		"document_sha256": hash, "raw_text": rawBody, "source_url": "https://reference.example/wiki/empty", "source_type": "community_wiki", "media_type": "text/html",
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if counts["documents_retained"] != 1 || counts["documents"] != 1 || len(fake.documents) != 1 {
		t.Fatalf("counts=%#v documents=%#v", counts, fake.documents)
	}
	for _, document := range fake.documents {
		if document.RawRetention != "full" || document.RawText != rawBody || document.ImportStatus != "pending" {
			t.Fatalf("retained document=%#v", document)
		}
	}
}

func TestSourceDiscoveryDeduplicatesFetchedBodiesBeforeExtraction(t *testing.T) {
	extractedSections := 0
	sources := []store.SourceDiscoverySource{
		{URL: "https://one.example/wiki/archive", SourceType: "community_wiki", PolicyConfirmed: true},
		{URL: "https://mirror.example/wiki/archive", SourceType: "community_wiki", PolicyConfirmed: true},
	}
	deps := sourceDiscoveryPipelineDeps{
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": "same-body", "_raw_body": "same raw body"}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "same fact"}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			extractedSections += len(sliceFromAny(result["section_candidates"]))
			return nil, nil, map[string]any{"status": "completed", "discovered_sections": 1, "processed_sections": 1}, nil
		},
	}
	_, result, _, _ := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{}, store.SourceDiscoveryInput{WorkQuery: "Neutral", Sources: sources}, deps)
	if extractedSections != 1 || len(sliceFromAny(result["observations"])) != 2 {
		t.Fatalf("extracted=%d observations=%d", extractedSections, len(sliceFromAny(result["observations"])))
	}
	retained := takeSourceDiscoveryRetainedDocuments(result)
	if len(retained) != 2 || result["_retained_documents"] != nil {
		t.Fatalf("retained=%#v public_result=%#v", retained, result)
	}
}

func TestSourceDiscoveryCanonicalizesNamuMirrorAndUUID(t *testing.T) {
	got := normalizeSourceSearchResultURL("https://www.namu.wiki/w/Neutral?uuid=duplicate")
	if got != "https://namu.moe/w/Neutral" {
		t.Fatalf("normalized URL=%q", got)
	}
	if canonicalDiscoveryURL("https://m.namu.moe/w/Neutral?uuid=another") != got {
		t.Fatalf("canonical URL mismatch")
	}
}

func TestSourceDiscoveryOperationalLimitIsNotCoverageSaturation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	searchCalls := 0
	deps := sourceDiscoveryPipelineDeps{
		search: func(_ context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
			searchCalls++
			if searchCalls == 6 {
				cancel()
			}
			return []store.SourceDiscoverySource{{URL: fmt.Sprintf("https://reference.example/pages/%d", searchCalls), SourceType: "discovered_public", AccessClass: "public_web", PolicyConfirmed: true}}, map[string]any{"status": "completed", "result_count": 1}, nil
		},
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			excerpt := fmt.Sprintf("New evidence %d", searchCalls)
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": fmt.Sprintf("hash-%d", searchCalls)}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": excerpt}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			section := mapFromAny(sliceFromAny(result["section_candidates"])[0])
			candidate := map[string]any{"kind": "claim", "subject": fmt.Sprintf("Subject %d", searchCalls), "statement": fmt.Sprintf("Statement %d", searchCalls), "source_url": section["source_url"], "document_sha256": section["document_sha256"], "locator": section["locator"], "evidence_excerpt": section["excerpt"]}
			return []map[string]any{candidate}, []map[string]any{{"query": fmt.Sprintf("follow up %d", searchCalls), "domain": "identity"}}, map[string]any{"status": "completed", "discovered_sections": 1, "processed_sections": 1}, nil
		},
	}
	_, result, coverage, _ := runSourceDiscoveryPipelineWith(ctx, completeTurnLLMConfig{}, store.SourceDiscoveryInput{WorkQuery: "Neutral", AllowedSourceTypes: []string{"community_wiki"}, RequestedDomains: []string{"identity"}}, deps)
	if result["termination_reason"] != "operational_limit_reached" || coverage["saturation"] != "operational_limit_reached" || searchCalls != 6 {
		t.Fatalf("calls=%d result=%#v coverage=%#v", searchCalls, result, coverage)
	}
}

func TestSourceDiscoveryDoesNotCapSourceCount(t *testing.T) {
	sources := make([]store.SourceDiscoverySource, 0, 15)
	for index := 0; index < 15; index++ {
		sources = append(sources, store.SourceDiscoverySource{URL: fmt.Sprintf("https://reference.example/pages/%d", index), SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true})
	}
	if err := validateSourceDiscoveryInput(store.SourceDiscoveryInput{WorkQuery: "Neutral", AllowedSourceTypes: []string{"community_wiki"}, Sources: sources}); err != nil {
		t.Fatalf("source count was rejected: %v", err)
	}
	fetched := 0
	deps := sourceDiscoveryPipelineDeps{
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			fetched++
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": fmt.Sprintf("hash-%d", fetched)}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Evidence"}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			sections := sliceFromAny(result["section_candidates"])
			return nil, nil, map[string]any{"status": "completed", "discovered_sections": len(sections), "processed_sections": len(sections)}, nil
		},
	}
	_, result, _, _ := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{}, store.SourceDiscoveryInput{WorkQuery: "Neutral", AllowedSourceTypes: []string{"community_wiki"}, Sources: sources}, deps)
	if fetched != len(sources) || len(sliceFromAny(result["observations"])) != len(sources) {
		t.Fatalf("fetched=%d observations=%d sources=%d", fetched, len(sliceFromAny(result["observations"])), len(sources))
	}
}

func TestSourceDiscoverySectionParsingAndExtractionBatchingDoNotDropItems(t *testing.T) {
	var html strings.Builder
	for index := 0; index < 220; index++ {
		fmt.Fprintf(&html, "<p>Section %d has evidence text.</p>", index)
	}
	sections := discoveryHTMLSections([]byte(html.String()))
	if len(sections) != 220 {
		t.Fatalf("parsed sections=%d", len(sections))
	}
	items := make([]any, 0, len(sections))
	for _, section := range sections {
		items = append(items, section)
	}
	batches := sourceCandidateExtractionBatches(store.SourceDiscoveryInput{WorkQuery: "Neutral"}, items, 2000)
	processed := 0
	for _, batch := range batches {
		processed += len(batch)
	}
	if len(batches) < 2 || processed != len(items) {
		t.Fatalf("batches=%d processed=%d sections=%d", len(batches), processed, len(items))
	}
	longText := strings.Repeat("가", 1800)
	textSections := discoverySections([]byte(longText), "text/plain")
	var reconstructed strings.Builder
	for _, section := range textSections {
		reconstructed.WriteString(stringFromMap(section, "excerpt"))
	}
	if reconstructed.String() != longText || len(textSections) != 3 {
		t.Fatalf("long document was not preserved: sections=%d runes=%d", len(textSections), len([]rune(reconstructed.String())))
	}
}

func TestSourceDiscoveryHTMLAnalysisRemovesBoilerplateAndRepeatedSections(t *testing.T) {
	body := []byte(`<html><body class="skin-vector-search-vue main-menu-enabled">
		<nav><p>Navigation should not be analyzed.</p></nav>
		<div class="sidebar"><p>Sidebar should not be analyzed.</p></div>
		<main><h1>Archive City</h1><p>The archive belongs to Mina.</p><p>The archive belongs to Mina.</p></main>
		<footer><p>Footer should not be analyzed.</p></footer>
	</body></html>`)
	sections := discoveryHTMLSections(body)
	if len(sections) != 2 {
		t.Fatalf("sections=%#v", sections)
	}
	joined := fmt.Sprint(sections)
	if strings.Contains(joined, "Navigation") || strings.Contains(joined, "Sidebar") || strings.Contains(joined, "Footer") {
		t.Fatalf("boilerplate leaked into analysis: %s", joined)
	}
}

func TestSourceDiscoverySearchQueryRequestsWikiResults(t *testing.T) {
	query := sourceDiscoverySearchQuery(store.SourceDiscoveryInput{WorkTitle: "Overlord", WorkType: "novel"})
	if query != "Overlord novel wiki" {
		t.Fatalf("query=%q", query)
	}
}

func TestSourceDiscoveryRebuildsMissingSectionsFromRetainedSourceBody(t *testing.T) {
	ref := newReferenceLibraryHTTPStore()
	ref.documents = append(ref.documents, store.ReferenceDocument{
		DocumentID: "doc-1", WorkID: "work-1", ContinuityID: "continuity-1", SourceType: "community_wiki",
		SourceURI: "https://reference.example/wiki/work", ContentHash: "hash-1", RawRetention: "full",
		RawText: `<html><body class="skin-vector-search-vue"><main><h1>Archive City</h1><p>Mina guards the archive.</p></main></body></html>`,
	})
	sections, err := sourceDiscoverySectionsFromStoredDocuments(context.Background(), ref, store.SourceDiscoveryInput{
		WorkID: "work-1", ContinuityID: "continuity-1",
	}, map[string]any{"observations": []any{map[string]any{
		"document_sha256": "hash-1", "media_type": "text/html", "final_url": "https://reference.example/wiki/work", "source_type": "community_wiki",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 || stringFromMap(mapFromAny(sections[0]), "document_sha256") != "hash-1" {
		t.Fatalf("sections=%#v", sections)
	}
}

func TestSourceDiscoveryInternalLinksRemainProvenanceUntilCoverageRequestsThem(t *testing.T) {
	searchCalls := 0
	fetchCalls := 0
	deps := sourceDiscoveryPipelineDeps{
		search: func(_ context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
			searchCalls++
			return []store.SourceDiscoverySource{{URL: "https://reference.example/wiki/root", SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true}}, map[string]any{"status": "completed", "result_count": 1}, nil
		},
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			fetchCalls++
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": "root", "internal_source_links": []string{"https://reference.example/wiki/a", "https://reference.example/wiki/b"}}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Root evidence"}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, result map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			sections := sliceFromAny(result["section_candidates"])
			return nil, nil, map[string]any{"status": "completed", "discovered_sections": len(sections), "processed_sections": len(sections)}, nil
		},
	}
	_, result, _, _ := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{}, store.SourceDiscoveryInput{WorkQuery: "Neutral"}, deps)
	if searchCalls != 1 || fetchCalls != 1 || len(sliceFromAny(result["observations"])) != 1 {
		t.Fatalf("search=%d fetch=%d result=%#v", searchCalls, fetchCalls, result)
	}
}

func TestSourceDiscoveryIncompleteExtractionDoesNotExpandSearchFrontier(t *testing.T) {
	searchCalls := 0
	deps := sourceDiscoveryPipelineDeps{
		search: func(_ context.Context, input store.SourceDiscoveryInput) ([]store.SourceDiscoverySource, map[string]any, error) {
			searchCalls++
			return []store.SourceDiscoverySource{{URL: "https://reference.example/wiki/root", SourceType: "community_wiki", AccessClass: "public_web", PolicyConfirmed: true}}, map[string]any{"status": "completed", "result_count": 1}, nil
		},
		fetch: func(_ context.Context, source store.SourceDiscoverySource) (map[string]any, []map[string]any, error) {
			return map[string]any{"requested_url": source.URL, "final_url": source.URL, "document_sha256": "root"}, []map[string]any{{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Root evidence"}, {"locator": map[string]any{"type": "p", "value": "2"}, "excerpt": "More evidence"}}, nil
		},
		extract: func(_ context.Context, _ completeTurnLLMConfig, _ store.SourceDiscoveryInput, _ map[string]any) ([]map[string]any, []map[string]any, map[string]any, error) {
			return nil, []map[string]any{{"query": "must not run yet", "domain": "entities"}}, map[string]any{"status": "completed", "discovered_sections": 2, "processed_sections": 1, "processing_incomplete": true}, nil
		},
	}
	_, result, coverage, _ := runSourceDiscoveryPipelineWith(context.Background(), completeTurnLLMConfig{}, store.SourceDiscoveryInput{WorkQuery: "Neutral"}, deps)
	if searchCalls != 1 || result["termination_reason"] != "analysis_backlog_remaining" || coverage["saturation"] != "operational_limit_reached" {
		t.Fatalf("search=%d result=%#v coverage=%#v", searchCalls, result, coverage)
	}
}

func TestSourceDiscoveryPersistsCharacterAndItemKindsAsTypedEntities(t *testing.T) {
	fake := &sourceDiscoveryAdmissionFake{documents: map[string]*store.ReferenceDocument{}}
	evidence := []any{map[string]any{"source_url": "https://reference.example/wiki/root", "source_type": "community_wiki", "document_sha256": "hash", "locator": map[string]any{"type": "p", "value": "1"}, "evidence_excerpt": "Mina carries the archive key."}}
	candidates := []map[string]any{
		{"kind": "character", "canonical_name": "Mina", "aliases": []string{"Archivist Mina"}, "evidence_set": evidence},
		{"kind": "item", "name": "Archive Key", "evidence_set": evidence},
	}
	counts, err := persistSourceDiscoveryCandidates(context.Background(), fake, "job", "work-1", "continuity-1", candidates, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if counts["entities"] != 2 || counts["aliases"] != 1 || len(fake.entities) != 2 || len(fake.aliases) != 1 || fake.entities[0].EntityType != "character" || fake.entities[1].EntityType != "item" {
		t.Fatalf("counts=%#v entities=%#v aliases=%#v", counts, fake.entities, fake.aliases)
	}
}

func TestSourceDiscoveryPrioritizesLocallyRareSectionsWithoutDroppingAny(t *testing.T) {
	sections := []any{
		map[string]any{"locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "common common introduction with many unrelated unique prose tokens that must not outrank structure"},
		map[string]any{"locator": map[string]any{"type": "h2", "value": "1"}, "section_kind": "heading", "excerpt": "rare archive covenant"},
		map[string]any{"locator": map[string]any{"type": "p", "value": "2"}, "excerpt": "common common overview"},
		map[string]any{"locator": map[string]any{"type": "li", "value": "1"}, "section_kind": "list_item", "excerpt": "Mina"},
	}
	prioritized := prioritizeDiscoverySections(sections)
	if len(prioritized) != len(sections) || stringFromMap(mapFromAny(prioritized[0]), "excerpt") != "rare archive covenant" || stringFromMap(mapFromAny(prioritized[1]), "excerpt") != "Mina" {
		t.Fatalf("prioritized=%#v", prioritized)
	}
}

func TestSourceDiscoveryResumeOffsetSupportsLegacyAndAggregateJobs(t *testing.T) {
	legacy := map[string]any{"extraction": map[string]any{"discovered_sections": 80, "processed_sections": 20}}
	if got := sourceDiscoveryResumeOffset(legacy, 100); got != 40 {
		t.Fatalf("legacy resume offset=%d", got)
	}
	aggregate := map[string]any{"extraction": map[string]any{"aggregate": true, "discovered_sections": 100, "processed_sections": 40}}
	if got := sourceDiscoveryResumeOffset(aggregate, 100); got != 40 {
		t.Fatalf("aggregate resume offset=%d", got)
	}
}

func TestSourceDiscoveryResumeResetsUnprovenExtractionContractOnce(t *testing.T) {
	legacy := map[string]any{"extraction": map[string]any{"processed_sections": 40, "aggregate": true}}
	if got, upgraded := sourceDiscoveryResumeContractOffset(legacy, 100); got != 0 || !upgraded {
		t.Fatalf("legacy contract offset=%d upgraded=%v", got, upgraded)
	}
	current := map[string]any{
		"extraction_contract": sourceCandidateExtractionContract,
		"extraction":          map[string]any{"processed_sections": 40, "aggregate": true},
	}
	if got, upgraded := sourceDiscoveryResumeContractOffset(current, 100); got != 40 || upgraded {
		t.Fatalf("current contract offset=%d upgraded=%v", got, upgraded)
	}
}

func TestSourceDiscoveryResumeDeduplicatesOnlyTheUnprocessedBacklog(t *testing.T) {
	sections := []any{
		map[string]any{"source_url": "https://one.example/a", "document_sha256": "one", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Already processed fact."},
		map[string]any{"source_url": "https://two.example/a", "document_sha256": "two", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "Already processed fact!"},
		map[string]any{"source_url": "https://two.example/b", "document_sha256": "two", "locator": map[string]any{"type": "p", "value": "2"}, "excerpt": "New backlog fact."},
		map[string]any{"source_url": "https://three.example/b", "document_sha256": "three", "locator": map[string]any{"type": "p", "value": "1"}, "excerpt": "New backlog fact!"},
	}
	deduped, duplicates := deduplicateSourceDiscoveryResumeSections(sections, 1)
	if len(deduped) != 2 || duplicates != 2 {
		t.Fatalf("deduped=%#v duplicates=%d", deduped, duplicates)
	}
	if len(sliceFromAny(mapFromAny(deduped[0])["equivalent_evidence"])) != 1 || len(sliceFromAny(mapFromAny(deduped[1])["equivalent_evidence"])) != 1 {
		t.Fatalf("equivalent evidence was not retained: %#v", deduped)
	}
}

func TestSourceDiscoveryCorpusResultReportsRemainingBacklog(t *testing.T) {
	job := &store.SourceDiscoveryJob{JobID: "source-job-1", Result: map[string]any{"section_candidates": []any{
		map[string]any{"document_sha256": "doc-a"},
		map[string]any{"document_sha256": "doc-b"},
		map[string]any{"document_sha256": "doc-b"},
	}}}
	result := sourceDiscoveryCorpusResult(job, 2, 0, 1, 12, "coverage_saturation_no_growth")
	if intFromAny(result["processed_sections"], -1) != 1 || intFromAny(result["remaining_sections"], -1) != 2 {
		t.Fatalf("result=%#v", result)
	}
	if intFromAny(result["remaining_document_count"], -1) != 1 || result["termination_reason"] != "coverage_saturation_no_growth" {
		t.Fatalf("result=%#v", result)
	}
}

func TestMarkSourceDiscoveryProcessedDocumentsOnlyMarksExhaustedSources(t *testing.T) {
	ref := newReferenceLibraryHTTPStore()
	ref.documents = append(ref.documents,
		store.ReferenceDocument{DocumentID: "doc-a", WorkID: "work-1", ContinuityID: "continuity-1", ContentHash: "hash-a", ImportStatus: "pending"},
		store.ReferenceDocument{DocumentID: "doc-b", WorkID: "work-1", ContinuityID: "continuity-1", ContentHash: "hash-b", ImportStatus: "pending"},
	)
	sections := []any{
		map[string]any{"document_sha256": "hash-a"},
		map[string]any{"document_sha256": "hash-b"},
		map[string]any{"document_sha256": "hash-b"},
	}
	markSourceDiscoveryProcessedDocuments(context.Background(), ref, "work-1", "continuity-1", sections, 2)
	if ref.documents[0].ImportStatus != "parsed" {
		t.Fatalf("completed source status=%q", ref.documents[0].ImportStatus)
	}
	if ref.documents[1].ImportStatus != "pending" {
		t.Fatalf("partially processed source status=%q", ref.documents[1].ImportStatus)
	}
}
