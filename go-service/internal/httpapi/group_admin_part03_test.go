package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestAdminReindexBlocksChromaDimensionMismatchAtFirstVectorError(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"test-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-dim-mismatch", Scope: "world", Category: "rule", Key: "gate", ValueJSON: `{"value":"The gate remains sealed."}`, SourceTurn: 1},
		},
	}
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{upsertErr: fmt.Errorf("chroma collection dimension mismatch: current embedding dimension=1024; existing collection was created with a different embedding dimension")}

	resp, err := srv.runAdminReindexJob(context.Background(), "sess-dim-mismatch", map[string]any{
		"force": true,
		"client_meta": map[string]any{"embedding": map[string]any{
			"provider": "openai", "api_key": "key", "endpoint": embeddingServer.URL,
			"model": "test-model", "timeout_ms": 5000,
		}},
	}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if resp["status"] != "blocked" || resp["reason"] != "chroma_collection_dimension_mismatch" {
		t.Fatalf("response = %#v, want blocked chroma_collection_dimension_mismatch", resp)
	}
	if resp["stage"] != "collection_recreate_required" || resp["ui_action"] != "recreate_chromadb_collection_then_reindex" {
		t.Fatalf("dimension mismatch guidance missing: %#v", resp)
	}
	if resp["blocked_tier"] != "world_rule" || resp["blocked_row_id"] != int64(1) {
		t.Fatalf("blocked target mismatch: %#v", resp)
	}
}

func TestAdminReindexDerivedArtifactsEmitsTierProgress(t *testing.T) {
	srv := NewServer(config.Default())
	events := []map[string]any{}
	progress := adminReindexDerivedArtifactProgress{
		Total: 2,
		Progress: func(item map[string]any) {
			events = append(events, cloneMapAny(item))
		},
	}
	result := srv.adminReindexDerivedArtifacts(
		context.Background(),
		"sess-derived-progress",
		completeTurnExtractionConfig{},
		false,
		100,
		[]store.DirectEvidence{{ID: 10, ChatSessionID: "sess-derived-progress", EvidenceText: "The brass key opens the cellar.", SourceTurnEnd: 1}},
		[]store.WorldRule{{ID: 20, ChatSessionID: "sess-derived-progress", Scope: "location", ScopeName: "cellar", Category: "access", Key: "brass_key", ValueJSON: `{"value":"The cellar opens with a brass key."}`, SourceTurn: 1}},
		nil,
		nil,
		progress,
	)
	if result.Processed != 2 || result.Skipped != 2 {
		t.Fatalf("result = %+v, want processed/skipped 2", result)
	}
	if len(events) == 0 {
		t.Fatal("expected progress events")
	}
	seenEvidence := false
	seenWorldRule := false
	for _, event := range events {
		if event["stage"] != "derived_artifact_reindex" {
			continue
		}
		if event["tier"] == "evidence" && event["phase"] == "item_done" {
			seenEvidence = true
		}
		if event["tier"] == "world_rule" && event["phase"] == "item_done" {
			seenWorldRule = true
		}
	}
	if !seenEvidence || !seenWorldRule {
		t.Fatalf("missing tier progress: evidence=%v world_rule=%v events=%#v", seenEvidence, seenWorldRule, events)
	}
}

func TestAdminReindexForceDoesNotIncludePerspectiveScopedEvidence(t *testing.T) {
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnEvidence: []store.DirectEvidence{{
		ID:            10,
		ChatSessionID: "sess-perspective-evidence",
		EvidenceKind:  "perspective_scoped_turn_excerpt",
		EvidenceText:  "A private perspective excerpt must not enter general vectors.",
		SourceTurnEnd: 1,
	}}}
	srv.StoreOpenError = nil
	vec := &turnRecordingVectorStore{}
	srv.Vector = vec

	result, err := srv.runAdminReindexJob(
		context.Background(),
		"sess-perspective-evidence",
		map[string]any{"force": true},
		nil,
	)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if result["status"] != "ok" || result["force"] != true {
		t.Fatalf("result = %#v, want successful forced reindex", result)
	}
	derived, ok := result["derived_artifact_reindex"].(map[string]any)
	if !ok {
		t.Fatalf("derived_artifact_reindex = %T, want object", result["derived_artifact_reindex"])
	}
	candidates, ok := derived["candidates_by_tier"].(map[string]int)
	if !ok || candidates["evidence"] != 0 {
		t.Fatalf("perspective evidence candidates = %#v, want zero", derived["candidates_by_tier"])
	}
	integrity, ok := result["integrity_report"].(map[string]any)
	if !ok || integrity["canonical_evidence_vector_count"] != 0 {
		t.Fatalf("integrity report counted perspective evidence: %#v", result["integrity_report"])
	}
	if vec.upsertCalls != 0 {
		t.Fatalf("forced reindex upsert calls = %d, want zero", vec.upsertCalls)
	}
}

func TestAdminIntegrityDoesNotRequirePrivateOnlyMemoryProjection(t *testing.T) {
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Vector = &turnRecordingVectorStore{}
	privateOnly := store.Memory{
		ID:             9,
		ChatSessionID:  "sess-private-integrity",
		TurnIndex:      2,
		SummaryJSON:    `{"turn_summary":"Mira privately suspects the gate code.","belief_updates":[{"knowledge_holder":"Mira","belief":"the code is 17"}]}`,
		Embedding:      "[]",
		EmbeddingModel: "perspective_scoped_typed_delivery",
	}
	report := srv.adminReindexIntegrityReport(
		context.Background(), privateOnly.ChatSessionID,
		[]store.Memory{privateOnly}, nil, nil, "test-model",
	)
	if report["canonical_memory_count"] != 0 ||
		report["canonical_vector_candidate_count"] != 0 ||
		report["missing_embedding_count"] != 0 {
		t.Fatalf("private-only projection was counted as required: %#v", report)
	}
}

type adminPreciseInventoryStore struct {
	*turnRecordingStore
	units []store.PreciseMemoryUnit
	err   error
}

func (s *adminPreciseInventoryStore) ListGeneralVectorPreciseMemoryUnits(context.Context, string) ([]store.PreciseMemoryUnit, error) {
	return append([]store.PreciseMemoryUnit(nil), s.units...), s.err
}

func TestAdminIntegrityCountsEligiblePreciseMemoryWithoutReportingExtraVector(t *testing.T) {
	const sid = "sess-precise-integrity"
	publicUnit := store.PreciseMemoryUnit{
		ID: 41, UnitID: "public-unit", ChatSessionID: sid,
		AdmissionState: "committed", ReviewState: "source_observed",
		Visibility: "public", EpistemicMode: "direct", LifecycleState: "active",
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &adminPreciseInventoryStore{
		turnRecordingStore: &turnRecordingStore{
			returnKGTriples:   []store.KGTriple{{ID: 61, ChatSessionID: sid}},
			returnEpisodeSums: []store.EpisodeSummary{{ID: 71, ChatSessionID: sid}},
		},
		units: []store.PreciseMemoryUnit{publicUnit},
	}
	srv.Vector = &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "precise_memory:" + sid + ":public-unit", Tier: "precise_memory", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "public-unit"},
		{ID: "kg_triple:" + sid + ":61", Tier: "kg_triple", ChatSessionID: sid, SourceTable: "kg_triples", SourceRowID: "61"},
		{ID: "episode:" + sid + ":71", Tier: "episode", ChatSessionID: sid, SourceTable: "episode_summaries", SourceRowID: "71"},
	}}

	report := srv.adminReindexIntegrityReport(context.Background(), sid, nil, nil, nil, "")
	if report["canonical_precise_memory_vector_count"] != 1 ||
		report["canonical_vector_candidate_count"] != 1 ||
		report["extra_vector_count_estimate"] != 0 ||
		report["vector_count_matches_canonical"] != true {
		t.Fatalf("eligible precise integrity report=%#v", report)
	}
}

func TestAdminOrphanAuditKeepsEligiblePreciseAndDeletesIneligiblePrecise(t *testing.T) {
	const sid = "sess-precise-orphan"
	eligible := store.PreciseMemoryUnit{
		ID: 51, UnitID: "eligible-unit", ChatSessionID: sid,
		AdmissionState: "committed", ReviewState: "source_observed",
		Visibility: "public", EpistemicMode: "direct", LifecycleState: "active",
	}
	ineligible := store.PreciseMemoryUnit{
		ID: 52, UnitID: "private-unit", ChatSessionID: sid,
		AdmissionState: "committed", ReviewState: "source_observed",
		Visibility: "owner_private", KnowledgeHolderEntityID: "holder",
		EpistemicMode: "known", LifecycleState: "active",
	}
	srv := NewServer(config.Default())
	srv.Store = &adminPreciseInventoryStore{
		turnRecordingStore: &turnRecordingStore{},
		units:              []store.PreciseMemoryUnit{eligible, ineligible},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "precise_memory:" + sid + ":eligible-unit", Tier: "precise_memory", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "eligible-unit"},
		{ID: "precise_memory:" + sid + ":private-unit", Tier: "precise_memory", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "private-unit"},
	}}
	srv.Vector = vec

	report := srv.adminVectorOrphanAudit(context.Background(), sid, true)
	counts, ok := report["canonical_counts"].(map[string]int)
	if !ok || counts["precise_memory_units"] != 1 {
		t.Fatalf("canonical precise counts=%#v", report["canonical_counts"])
	}
	if report["orphan_count"] != 1 || report["deleted_orphan_count"] != 1 ||
		len(vec.docs) != 1 || vec.docs[0].ID != "precise_memory:"+sid+":eligible-unit" {
		t.Fatalf("precise orphan audit=%#v remaining=%#v", report, vec.docs)
	}
	if got := adminManagedVectorTier(vector.VectorDocument{ID: "precise_memory:" + sid + ":private-unit"}); got != "precise_memory" {
		t.Fatalf("managed precise tier=%q", got)
	}
}

func TestAdminForceReplaysCanonicalPublicProjectionWithoutDirectChromaMutation(t *testing.T) {
	const sid = "sess-canonical-force"
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"test-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()
	st := newAdminCanonicalReplayTestStore(sid, 3, map[string]any{
		"turn_summary":      "The brass key remains on the public table.",
		"importance_score":  6,
		"evidence_excerpts": []any{"The brass key remains on the public table."},
	})
	st.memories = []store.Memory{{
		ID: 7, ChatSessionID: sid, TurnIndex: 3,
		SummaryJSON: `{"turn_summary":"The brass key remains on the public table."}`,
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	vec := &turnRecordingVectorStore{}
	srv.Vector = vec

	result, err := srv.runAdminReindexJob(
		context.Background(), sid, map[string]any{"force": true, "client_meta": map[string]any{
			"embedding": map[string]any{"provider": "openai", "api_key": "key", "endpoint": embeddingServer.URL, "model": "test-model", "timeout_ms": 5000},
		}}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result["canonical_replays_completed"] != 1 || result["vector_replays_queued"] != 1 ||
		result["upserted"] != 0 || vec.upsertCalls != 0 {
		t.Fatalf("canonical force crossed direct Chroma boundary: result=%#v upserts=%d", result, vec.upsertCalls)
	}
	if len(st.admissions) != 1 || st.admissions[0].IndexVersion != memoryAdmissionIndexVersion {
		t.Fatalf("admissions=%+v", st.admissions)
	}
	firstCount := len(st.admissions)
	if _, err := srv.runAdminReindexJob(
		context.Background(), sid, map[string]any{"force": true, "client_meta": map[string]any{
			"embedding": map[string]any{"provider": "openai", "api_key": "key", "endpoint": embeddingServer.URL, "model": "test-model", "timeout_ms": 5000},
		}}, nil,
	); err != nil {
		t.Fatal(err)
	}
	if len(st.admissions) != firstCount+1 || vec.upsertCalls != 0 {
		t.Fatalf("repeated force was not an idempotent canonical replay: admissions=%d upserts=%d", len(st.admissions), vec.upsertCalls)
	}
}

func TestAdminVoyageCanonicalReplayDoesNotResendRawChat(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()
	requests := [][]string{}
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		groups := sliceFromAny(request["inputs"])
		for _, group := range groups {
			chunks := []string{}
			for _, chunk := range sliceFromAny(group) {
				chunks = append(chunks, fmt.Sprint(chunk))
			}
			requests = append(requests, chunks)
		}
		rows := []map[string]any{{"index": 0, "data": []any{map[string]any{"index": 0, "embedding": []float64{0.1, 0.2}}}}}
		body, _ := json.Marshal(map[string]any{"data": rows, "model": "voyage-context-4"})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	const sid = "sess-admin-no-raw-chat"
	st := newAdminCanonicalReplayTestStore(sid, 1, map[string]any{
		"turn_summary":      "The public bell rang.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The public bell rang."},
	})
	st.source.UserContent = "RAW USER SECRET MUST NOT BE SENT"
	st.source.AssistantContent = "RAW ASSISTANT SECRET MUST NOT BE SENT"
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	srv.Vector = &turnRecordingVectorStore{}
	_, err := srv.runAdminReindexJob(context.Background(), sid, map[string]any{
		"force": true,
		"client_meta": map[string]any{"embedding": map[string]any{
			"provider": "voyageai", "api_key": "key", "endpoint": "https://api.voyageai.com/v1/embeddings",
			"model": "voyage-context-4", "timeout_ms": 5000,
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) == 0 {
		t.Fatal("canonical replay did not issue a public projection embedding request")
	}
	publicProjectionObserved := false
	for _, chunks := range requests {
		joined := strings.Join(chunks, "\n")
		if strings.Contains(joined, "RAW USER SECRET") || strings.Contains(joined, "RAW ASSISTANT SECRET") {
			t.Fatalf("admin Voyage received raw chat: %#v", requests)
		}
		if strings.Contains(joined, "The public bell rang.") {
			publicProjectionObserved = true
		}
	}
	if !publicProjectionObserved {
		t.Fatalf("canonical replay did not embed its public projection: %#v", requests)
	}
}

func TestAdminReindexReportsUnmatchedLegacyArtifactsInMixedCanonicalSession(t *testing.T) {
	const sid = "sess-mixed-canonical-legacy"
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"test-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()
	st := newAdminCanonicalReplayTestStore(sid, 1, map[string]any{
		"turn_summary":      "The public bell rang.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The public bell rang."},
	})
	st.memories = []store.Memory{
		{ID: 11, ChatSessionID: sid, TurnIndex: 1, SummaryJSON: `{"turn_summary":"The public bell rang."}`, Embedding: `[0.1]`, EmbeddingModel: "old-model"},
		{ID: 12, ChatSessionID: sid, TurnIndex: 2, SummaryJSON: `{"turn_summary":"Legacy row without a source owner."}`, Embedding: `[0.2]`, EmbeddingModel: "old-model"},
	}
	st.evidence = []store.DirectEvidence{{
		ID: 21, ChatSessionID: sid, EvidenceText: "Imported evidence on a covered turn.",
		TurnAnchor: 1, CaptureStage: "hypamemory_import",
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	vec := &turnRecordingVectorStore{}
	srv.Vector = vec
	result, err := srv.runAdminReindexJob(context.Background(), sid, map[string]any{
		"client_meta": map[string]any{"embedding": map[string]any{
			"provider": "openai", "api_key": "key", "endpoint": embeddingServer.URL,
			"model": "test-model", "timeout_ms": 5000,
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["canonical_replays_completed"] != 1 || vec.upsertCalls != 0 {
		t.Fatalf("result=%#v direct_upserts=%d", result, vec.upsertCalls)
	}
	failedTurns, ok := result["failed_turns"].([]int64)
	if !ok || len(failedTurns) != 2 ||
		!int64SliceContains(failedTurns, 1) || !int64SliceContains(failedTurns, 2) {
		t.Fatalf("failed_turns=%#v, want unmatched turn 2 and unsupported evidence turn 1", result["failed_turns"])
	}
	errorsOut, ok := result["errors"].([]string)
	joinedErrors := strings.Join(errorsOut, "\n")
	if !ok || !strings.Contains(joinedErrors, "memory:12 has no active committed source revision") ||
		!strings.Contains(joinedErrors, "evidence:21 has unsupported capture_stage") {
		t.Fatalf("errors=%#v, want explicit unmatched memory and unsupported evidence", result["errors"])
	}
}

func int64SliceContains(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestAdminCanonicalReplayBlocksBeforeMutationWithoutEmbeddingConfig(t *testing.T) {
	const sid = "sess-canonical-config-preflight"
	st := newAdminCanonicalReplayTestStore(sid, 1, map[string]any{
		"turn_summary":      "The public bell rang.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The public bell rang."},
	})
	st.memories = []store.Memory{{
		ID: 11, ChatSessionID: sid, TurnIndex: 1,
		SummaryJSON: `{"turn_summary":"The public bell rang."}`,
		Embedding:   `[0.1]`, EmbeddingModel: "legacy-model",
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	srv.Vector = &turnRecordingVectorStore{}
	result, err := srv.runAdminReindexJob(context.Background(), sid, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "blocked" || result["reason"] != "missing_embedding_config" ||
		result["stage"] != "embedding_config_preflight" || len(st.admissions) != 0 {
		t.Fatalf("result=%#v admissions=%d, want pre-mutation block", result, len(st.admissions))
	}
}

func TestAdminForcePrivateSentinelReconcilesDeleteWithoutEmbeddingConfig(t *testing.T) {
	const sid = "sess-private-sentinel-delete"
	st := newAdminCanonicalReplayTestStore(sid, 1, map[string]any{
		"turn_summary":     "Mira privately believes the gate code is seventeen.",
		"importance_score": 5,
		"belief_updates": []any{map[string]any{
			"knowledge_holder": "Mira",
			"belief":           "The gate code is seventeen.",
		}},
	})
	st.memories = []store.Memory{{
		ID: 11, ChatSessionID: sid, TurnIndex: 1,
		SummaryJSON:    `{"summary":"legacy perspective sentinel row"}`,
		Embedding:      `[]`,
		EmbeddingModel: "perspective_scoped_typed_delivery",
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	vec := &turnRecordingVectorStore{}
	srv.Vector = vec
	result, err := srv.runAdminReindexJob(
		context.Background(), sid, map[string]any{"force": true}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ok" || result["canonical_replays_completed"] != 1 ||
		result["vector_replays_queued"] != 1 || vec.upsertCalls != 0 || len(st.admissions) != 1 {
		t.Fatalf("result=%#v upserts=%d admissions=%d", result, vec.upsertCalls, len(st.admissions))
	}
	admission := st.admissions[0]
	if len(admission.Vectors) != 0 || admission.Memory == nil ||
		admission.Memory.EmbeddingModel != "" || admission.Memory.Embedding != "[]" {
		t.Fatalf("private replay created a general vector or fake model: %+v", admission)
	}
}

func TestAdminBoundedReindexReportsUnsupportedEvidenceOnCoveredSource(t *testing.T) {
	const sid = "sess-bounded-unsupported-evidence"
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"test-model","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer embeddingServer.Close()
	st := newAdminCanonicalReplayTestStore(sid, 7, map[string]any{
		"turn_summary":      "The public bell rang.",
		"importance_score":  5,
		"evidence_excerpts": []any{"The public bell rang."},
	})
	st.evidence = []store.DirectEvidence{{
		ID: 21, ChatSessionID: sid, EvidenceText: "Imported evidence on the covered turn.",
		TurnAnchor: 7, CaptureStage: "hypamemory_import",
	}}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = st
	srv.Vector = &turnRecordingVectorStore{}
	result, err := srv.runAdminReindexJob(context.Background(), sid, map[string]any{
		"max_items": 1,
		"client_meta": map[string]any{"embedding": map[string]any{
			"provider": "openai", "api_key": "key", "endpoint": embeddingServer.URL,
			"model": "test-model", "timeout_ms": 5000,
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	failedTurns, ok := result["failed_turns"].([]int64)
	if !ok || !int64SliceContains(failedTurns, 7) ||
		!strings.Contains(strings.Join(result["errors"].([]string), "\n"), "unsupported capture_stage") {
		t.Fatalf("bounded unsupported evidence was silently omitted: %#v", result)
	}
}

func TestAdminReindexVoyageSendsOnlyWorldRulesOutsideCanonicalAdmission(t *testing.T) {
	oldClient := proxyHTTPClient
	documents := [][]string{}
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/v1/contextualizedembeddings") {
			t.Fatalf("embedding path = %q", r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode embedding request: %v", err)
		}
		groups := sliceFromAny(request["inputs"])
		if len(groups) != 1 {
			t.Fatalf("Voyage request documents = %d, want one logical turn", len(groups))
		}
		chunksAny := sliceFromAny(groups[0])
		chunks := make([]string, 0, len(chunksAny))
		rows := make([]map[string]any, 0, len(chunksAny))
		for i, chunk := range chunksAny {
			chunks = append(chunks, fmt.Sprint(chunk))
			rows = append(rows, map[string]any{"index": i, "embedding": []float64{float64(i + 1), 0.5}})
		}
		documents = append(documents, chunks)
		body, _ := json.Marshal(map[string]any{
			"data":  []any{map[string]any{"index": 0, "data": rows}},
			"model": "voyage-context-4",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	const sid = "sess-voyage-turn-boundary"
	srv := NewServer(func() config.Config {
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeMariaDBShadow
		cfg.ChromaEndpoint = "http://127.0.0.1:8000"
		return cfg
	}())
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: sid, TurnIndex: 1, Role: "user", Content: "turn one user input"},
			{ID: 2, ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "turn one assistant output"},
			{ID: 3, ChatSessionID: sid, TurnIndex: 2, Role: "user", Content: "turn two user input"},
			{ID: 4, ChatSessionID: sid, TurnIndex: 2, Role: "assistant", Content: "turn two assistant output"},
		},
		returnMemories: []store.Memory{
			{ID: 11, ChatSessionID: sid, TurnIndex: 1, SummaryJSON: `{"summary":"turn one memory"}`},
			{ID: 12, ChatSessionID: sid, TurnIndex: 2, SummaryJSON: `{"summary":"turn two memory"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 21, ChatSessionID: sid, EvidenceText: "turn one evidence", SourceTurnEnd: 1},
			{ID: 22, ChatSessionID: sid, EvidenceText: "turn two evidence", SourceTurnEnd: 2},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 31, ChatSessionID: sid, Scope: "world", Category: "rule", Key: "turn_one", ValueJSON: `{"value":"turn one world rule"}`, SourceTurn: 1},
			{ID: 32, ChatSessionID: sid, Scope: "world", Category: "rule", Key: "turn_two", ValueJSON: `{"value":"turn two world rule"}`, SourceTurn: 2},
		},
	}
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{
		"chat_session_id":"sess-voyage-turn-boundary",
		"force":true,
		"client_meta":{"embedding":{
			"provider":"voyageai",
			"api_key":"test-key",
			"endpoint":"https://api.voyageai.com/v1/embeddings",
			"model":"voyage-context-4",
			"timeout_ms":5000
		}}
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(documents) != 2 {
		t.Fatalf("Voyage calls = %d, want one independent call per world rule: %#v response=%s", len(documents), documents, rec.Body.String())
	}
	for turnIndex, chunks := range documents {
		turn := turnIndex + 1
		joined := strings.Join(chunks, "\n")
		want := fmt.Sprintf("turn %s world rule", []string{"one", "two"}[turnIndex])
		if !strings.Contains(joined, want) {
			t.Fatalf("turn %d document missing %q: %#v", turn, want, chunks)
		}
		for _, forbidden := range []string{
			"user input", "assistant output", "memory", "evidence",
			"turn " + []string{"two", "one"}[turnIndex] + " world rule",
		} {
			if strings.Contains(joined, forbidden) {
				t.Fatalf("world-rule document leaked %q: %#v", forbidden, chunks)
			}
		}
	}
}

func TestAdminReindexVoyageContextKeepsUnanchoredArtifactsIndependent(t *testing.T) {
	documents := [][]string{}
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		groups := sliceFromAny(request["inputs"])
		if len(groups) != 1 {
			t.Fatalf("Voyage request documents=%d, want one independent document", len(groups))
		}
		chunksAny := sliceFromAny(groups[0])
		chunks := make([]string, len(chunksAny))
		rows := make([]map[string]any, len(chunksAny))
		for index, chunk := range chunksAny {
			chunks[index] = fmt.Sprint(chunk)
			rows[index] = map[string]any{"index": index, "embedding": []float64{float64(index + 1), 0.5}}
		}
		documents = append(documents, chunks)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"index": 0, "data": rows}}, "model": "voyage-context-4",
		})
	}))
	defer embeddingServer.Close()

	items := adminReindexContextualizedEmbeddingItems(
		nil,
		[]store.DirectEvidence{
			{ID: 1, EvidenceText: "unanchored evidence one"},
			{ID: 2, EvidenceText: "unanchored evidence two"},
		},
		[]store.WorldRule{
			{ID: 3, Scope: "world", Category: "rule", Key: "one", ValueJSON: `{"value":"unanchored rule one"}`},
			{ID: 4, Scope: "world", Category: "rule", Key: "two", ValueJSON: `{"value":"unanchored rule two"}`},
		},
		0,
		completeTurnEmbeddingConfig{Provider: "voyageai", APIKey: "key", Endpoint: embeddingServer.URL, Model: "voyage-context-4", TimeoutMs: 5000},
		true,
		true,
	)
	if _, _, err := callContextualizedEmbeddingItems(context.Background(), completeTurnEmbeddingConfig{
		Provider: "voyageai", APIKey: "key", Endpoint: embeddingServer.URL, Model: "voyage-context-4", TimeoutMs: 5000,
	}, nil, items); err != nil {
		t.Fatal(err)
	}
	if len(documents) != 4 {
		t.Fatalf("Voyage calls=%d, want one call per unanchored artifact: %#v", len(documents), documents)
	}
	for _, chunks := range documents {
		if len(chunks) != 1 {
			t.Fatalf("unanchored artifacts were combined: %#v", chunks)
		}
	}
}

func TestAdminReindexSkipsAlreadyCurrentIndexWithoutForce(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnMemories: []store.Memory{{
		ID:             7,
		ChatSessionID:  "sess-reindex-current",
		TurnIndex:      3,
		SummaryJSON:    `{"summary":"The gate is already indexed."}`,
		Embedding:      `[0.1,0.2,0.3]`,
		EmbeddingModel: "test-embedding",
	}}}
	srv.StoreOpenError = nil
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{{
		ID:            "memory:sess-reindex-current:7",
		Tier:          "memory",
		ChatSessionID: "sess-reindex-current",
		SourceTable:   "memories",
		SourceRowID:   "7",
	}}}
	srv.Vector = vec

	result, err := srv.runAdminReindexJob(context.Background(), "sess-reindex-current", map[string]any{"force": false}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if result["reason"] != "vector_index_already_current" || result["reindex_executed"] != false {
		t.Fatalf("already-current result mismatch: %#v", result)
	}
	if len(vec.docs) != 1 {
		t.Fatalf("already-current reindex must not upsert duplicates: %#v", vec.docs)
	}
}

func TestAdminReindexSkipsCurrentEmptyPublicIndexForPrivateOnlySession(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnMemories: []store.Memory{{
		ID:             9,
		ChatSessionID:  "sess-reindex-private-current",
		TurnIndex:      4,
		SummaryJSON:    `{"turn_summary":"Mira privately suspects the gate code.","belief_updates":[{"knowledge_holder":"Mira","belief":"the code is 17"}]}`,
		Embedding:      "[]",
		EmbeddingModel: "perspective_scoped_typed_delivery",
	}}}
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{}

	result, err := srv.runAdminReindexJob(context.Background(), "sess-reindex-private-current", map[string]any{"force": false}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if result["reason"] != "vector_index_already_current" || result["reindex_executed"] != false {
		t.Fatalf("private-only current result mismatch: %#v", result)
	}
	integrity, _ := result["integrity_report"].(map[string]any)
	if integrity["vector_index_current"] != true || integrity["index_usable_for_vector_first_read"] != false {
		t.Fatalf("private-only empty integrity mismatch: %#v", integrity)
	}
}

func TestAdminIntegrityCountsDuplicateManagedAliasesAsExtra(t *testing.T) {
	const sid = "sess-reindex-duplicate-alias"
	memory := store.Memory{
		ID: 7, ChatSessionID: sid, TurnIndex: 3,
		SummaryJSON: `{"summary":"The gate is indexed."}`,
		Embedding:   `[0.1,0.2,0.3]`, EmbeddingModel: "test-embedding",
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnMemories: []store.Memory{memory}}
	srv.Vector = &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:" + sid + ":7", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "7"},
		{ID: "memory:7", Tier: "memory", ChatSessionID: sid, SourceTable: "memories", SourceRowID: "7"},
	}}

	report := srv.adminReindexIntegrityReport(context.Background(), sid, []store.Memory{memory}, nil, nil, "test-embedding")
	if report["vector_index_current"] != false || report["extra_vector_count_estimate"] != 1 {
		t.Fatalf("duplicate managed alias integrity mismatch: %#v", report)
	}
}

func TestAdminIntegrityWrongIDWithCanonicalSourcePairIsOrphanAndMissing(t *testing.T) {
	const sid = "sess-reindex-wrong-id"
	memory := store.Memory{
		ID: 7, ChatSessionID: sid, TurnIndex: 3,
		SummaryJSON: `{"summary":"The gate is indexed."}`,
		Embedding:   `[0.1,0.2,0.3]`, EmbeddingModel: "test-embedding",
	}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{returnMemories: []store.Memory{memory}}
	srv.Vector = &turnRecordingVectorStore{docs: []vector.VectorDocument{{
		ID: "memory:" + sid + ":wrong", Tier: "memory", ChatSessionID: sid,
		SourceTable: "memories", SourceRowID: "7",
	}}}

	report := srv.adminReindexIntegrityReport(context.Background(), sid, []store.Memory{memory}, nil, nil, "test-embedding")
	if report["vector_index_current"] != false ||
		report["matched_canonical_vector_count"] != 0 ||
		report["managed_orphan_count"] != 1 ||
		report["missing_vector_count_estimate"] != 1 ||
		report["extra_vector_count_estimate"] != 1 {
		t.Fatalf("wrong-ID same-pair integrity mismatch: %#v", report)
	}
}

func TestAdminReindexIgnoresKGAndHierarchyOutsideForceOwnership(t *testing.T) {
	const sid = "sess-reindex-unowned-tiers"
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{
		returnKGTriples:   []store.KGTriple{{ID: 61, ChatSessionID: sid}},
		returnEpisodeSums: []store.EpisodeSummary{{ID: 71, ChatSessionID: sid}},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "kg_triple:" + sid + ":61", Tier: "kg_triple", ChatSessionID: sid, SourceTable: "kg_triples", SourceRowID: "61"},
		{ID: "episode:" + sid + ":71", Tier: "episode", ChatSessionID: sid, SourceTable: "episode_summaries", SourceRowID: "71"},
	}}
	srv.Vector = vec

	result, err := srv.runAdminReindexJob(context.Background(), sid, map[string]any{"force": false}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if result["reason"] != "vector_index_already_current" || result["reindex_executed"] != false {
		t.Fatalf("unowned tiers caused a reindex loop: %#v", result)
	}
	integrity, _ := result["integrity_report"].(map[string]any)
	if integrity["canonical_vector_candidate_count"] != 0 ||
		integrity["managed_vector_count"] != 0 ||
		integrity["vector_index_current"] != true {
		t.Fatalf("unowned tiers entered force inventory: %#v", integrity)
	}

	audit := srv.adminVectorOrphanAudit(context.Background(), sid, true)
	if audit["orphan_count"] != 0 || audit["deleted_orphan_count"] != 0 ||
		audit["ignored_unmanaged_count"] != 2 || len(vec.docs) != 2 {
		t.Fatalf("unowned tier documents were not preserved: report=%#v docs=%#v", audit, vec.docs)
	}
}

func TestAdminReindexZeroLimitProcessesWholeCandidateSet(t *testing.T) {
	const candidateCount = 250
	memories := make([]store.Memory, 0, candidateCount)
	for index := 1; index <= candidateCount; index++ {
		memories = append(memories, store.Memory{
			ID:             int64(index),
			ChatSessionID:  "sess-reindex-unlimited",
			TurnIndex:      index,
			SummaryJSON:    fmt.Sprintf(`{"summary":"memory %d"}`, index),
			Embedding:      `[0.1,0.2,0.3]`,
			EmbeddingModel: "test-embedding",
		})
	}
	srv := NewServer(config.Default())
	srv.Store = &turnRecordingStore{returnMemories: memories}
	srv.StoreOpenError = nil

	result, err := srv.runAdminReindexJob(
		context.Background(),
		"sess-reindex-unlimited",
		map[string]any{"max_items": 0, "dry_run": true},
		nil,
	)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if intFromAny(result["max_items"], -1) != 0 ||
		intFromAny(result["candidates"], 0) != candidateCount {
		t.Fatalf("result=%#v", result)
	}
}

func TestAdminVectorOrphanAuditDeletesFullListingOrphans(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{{ID: 1, ChatSessionID: "sess-orphan", TurnIndex: 1, SummaryJSON: `{"summary":"Kept memory"}`}},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:sess-orphan:1", Tier: "memory", ChatSessionID: "sess-orphan", SourceTable: "memories", SourceRowID: "1"},
		{ID: "evidence:sess-orphan:999", Tier: "evidence", ChatSessionID: "sess-orphan", SourceTable: "direct_evidence_records", SourceRowID: "999"},
	}}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/vector-orphan-audit", strings.NewReader(`{"chat_session_id":"sess-orphan","delete_orphans":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["full_listing_available"] != true || resp["orphan_count"] != float64(1) || resp["deleted_orphan_count"] != float64(1) {
		t.Fatalf("orphan audit response = %#v", resp)
	}
	if len(vec.docs) != 1 || vec.docs[0].ID != "memory:sess-orphan:1" {
		t.Fatalf("remaining vector docs = %#v", vec.docs)
	}
}

func TestAdminDedupeCleanupDryRunAndApply(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-dedupe", TurnIndex: 1, SummaryJSON: `{"summary":"Shared vow persists."}`, Importance: 3},
			{ID: 2, ChatSessionID: "sess-dedupe", TurnIndex: 2, SummaryJSON: `{"summary":"Shared vow persists."}`, Importance: 5},
		},
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "sess-dedupe", Name: "Bridge promise", CurrentContext: "The bridge promise remains open.", LastTurn: 1},
			{ID: 11, ChatSessionID: "sess-dedupe", Name: "Bridge promise", CurrentContext: "The bridge promise remains open.", LastTurn: 2},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "sess-dedupe", Scope: "location", ScopeName: "bridge", Category: "access", Key: "guarded_gate", ValueJSON: `{"value":"The gate is guarded."}`, SourceTurn: 1},
			{ID: 21, ChatSessionID: "sess-dedupe", Scope: "location", ScopeName: "bridge", Category: "access", Key: "guarded_gate", ValueJSON: `{"value":"The gate is guarded."}`, SourceTurn: 2},
		},
	}
	vec := &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:sess-dedupe:1", Tier: "memory", ChatSessionID: "sess-dedupe", SourceTable: "memories", SourceRowID: "1"},
		{ID: "memory:sess-dedupe:2", Tier: "memory", ChatSessionID: "sess-dedupe", SourceTable: "memories", SourceRowID: "2"},
		{ID: "world_rule:sess-dedupe:20", Tier: "world_rule", ChatSessionID: "sess-dedupe", SourceTable: "world_rules", SourceRowID: "20"},
		{ID: "world_rule:sess-dedupe:21", Tier: "world_rule", ChatSessionID: "sess-dedupe", SourceTable: "world_rules", SourceRowID: "21"},
	}}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/dedupe-cleanup", strings.NewReader(`{"chat_session_id":"sess-dedupe"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d: %s", rec.Code, rec.Body.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	counts, ok := preview["candidate_counts"].(map[string]any)
	if !ok || counts["memories"] != float64(1) || counts["storylines"] != float64(1) || counts["world_rules"] != float64(1) {
		t.Fatalf("candidate_counts = %#v", preview["candidate_counts"])
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/dedupe-cleanup", strings.NewReader(`{"chat_session_id":"sess-dedupe","apply":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d: %s", rec.Code, rec.Body.String())
	}
	var applied map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode applied: %v", err)
	}
	deleted, ok := applied["deleted_counts"].(map[string]any)
	if !ok || deleted["memories"] != float64(1) || deleted["storylines"] != float64(1) || deleted["world_rules"] != float64(1) {
		t.Fatalf("deleted_counts = %#v", applied["deleted_counts"])
	}
	if len(fake.returnMemories) != 1 || fake.returnMemories[0].ID != 2 {
		t.Fatalf("remaining memories = %#v", fake.returnMemories)
	}
	if len(fake.returnStorylines) != 1 || fake.returnStorylines[0].ID != 11 {
		t.Fatalf("remaining storylines = %#v", fake.returnStorylines)
	}
	if len(fake.returnWorldRules) != 1 || fake.returnWorldRules[0].ID != 21 {
		t.Fatalf("remaining world rules = %#v", fake.returnWorldRules)
	}
}

func TestAdminReindexBackgroundJobReportsProgress(t *testing.T) {
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"test-embedding","data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer embeddingServer.Close()
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := newAdminCanonicalReplayTestStore("sess-reindex-bg", 7, map[string]any{
		"turn_summary":      "Blue lantern oath persists.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Blue lantern oath persists."},
	})
	fake.memories = []store.Memory{{
		ID: 42, ChatSessionID: "sess-reindex-bg", TurnIndex: 7,
		SummaryJSON: `{"turn_summary":"Blue lantern oath persists."}`,
		Embedding:   `[0.1,0.2,0.3]`, EmbeddingModel: "old-model",
	}}
	vec := &turnRecordingVectorStore{}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(fmt.Sprintf(`{
		"chat_session_id":"sess-reindex-bg","force":true,"background":true,
		"client_meta":{"embedding":{"provider":"openai","api_key":"key",
		"endpoint":%q,"model":"test-embedding","timeout_ms":5000}}
	}`, embeddingServer.URL)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["processed"] != float64(1) || progress["upserted"] != float64(0) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	if progress["progress_percent"] != float64(100) {
		t.Fatalf("progress_percent = %v, want 100", progress["progress_percent"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["reindex_executed"] != true ||
		result["canonical_replays_completed"] != float64(1) ||
		result["vector_replays_queued"] != float64(1) ||
		result["vector_delivery_pending"] != true || vec.upsertCalls != 0 {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminRescanBackgroundJobReportsFailedTurns(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "user", Content: "turn one user"},
			{ID: 2, ChatSessionID: "sess-rescan-bg", TurnIndex: 1, Role: "assistant", Content: "turn one assistant"},
			{ID: 3, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "user", Content: "turn two user"},
			{ID: 4, ChatSessionID: "sess-rescan-bg", TurnIndex: 2, Role: "assistant", Content: "turn two assistant"},
		},
	}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/rescan", strings.NewReader(`{"chat_session_id":"sess-rescan-bg","max_items":222,"background":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	jobID, _ := start["job_id"].(string)
	if jobID == "" {
		t.Fatalf("job_id missing: %#v", start)
	}

	var job map[string]any
	for i := 0; i < 50; i++ {
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/admin/jobs/"+jobID, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("job status = %d: %s", rec.Code, rec.Body.String())
		}
		job = map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode job: %v", err)
		}
		if job["status"] == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job["status"] != "completed" {
		t.Fatalf("job did not complete: %#v", job)
	}
	progress, ok := job["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress missing: %#v", job)
	}
	if progress["candidate_count"] != float64(2) || progress["failed_count"] != float64(2) {
		t.Fatalf("progress mismatch: %#v", progress)
	}
	failedTurns, ok := progress["failed_turns"].([]any)
	if !ok || len(failedTurns) != 2 {
		t.Fatalf("failed_turns mismatch: %#v", progress["failed_turns"])
	}
	result, ok := job["result"].(map[string]any)
	if !ok || result["failed"] != float64(2) {
		t.Fatalf("result mismatch: %#v", job["result"])
	}
}

func TestAdminSessionNormalizeQueuesRedactedBackgroundJob(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	srv := NewServer(cfg)
	srv.Store = &memoryFakeStore{}
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-normalize-bg","max_items":25,"repair_entries":[{"turn_index":1,"user_content":"private user text","assistant_content":"private assistant text"}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-normalize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if start["kind"] != "session_normalize" {
		t.Fatalf("kind = %v, want session_normalize", start["kind"])
	}
	if start["job_id"] == "" {
		t.Fatalf("job_id missing: %#v", start)
	}
	request, ok := start["request"].(map[string]any)
	if !ok {
		t.Fatalf("request missing: %#v", start)
	}
	if request["repair_entry_count"] != float64(1) {
		t.Fatalf("repair_entry_count = %v, want 1", request["repair_entry_count"])
	}
	raw, _ := json.Marshal(request)
	if strings.Contains(string(raw), "private user text") || strings.Contains(string(raw), "private assistant text") {
		t.Fatalf("job request leaked raw content: %s", string(raw))
	}
	if request["destructive"] != false {
		t.Fatalf("destructive flag = %v, want false", request["destructive"])
	}
}

func TestAdminSessionNormalizeDefaultsToResumeExistingArtifacts(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{"source": "explorer_session_normalize"})
	for _, key := range []string{
		"force_derived_rebuild",
		"force_world_rule_backfill",
		"force_raw_world_rule_audit",
		"force_episode_backfill",
		"force_hierarchy_backfill",
	} {
		if boolFromAny(meta[key]) {
			t.Fatalf("%s must be opt-in for resumable session normalization: %#v", key, meta)
		}
	}
	if !boolFromAny(meta["resume_existing_artifacts"]) || !boolFromAny(meta["full_session_backfill"]) {
		t.Fatalf("resume/full-session markers missing: %#v", meta)
	}
	if adminSessionNormalizeForceReindex(adminSessionNormalizeRequest{}) {
		t.Fatal("force_reindex must default to false so completed vectors are not rebuilt on every retry")
	}
}

func TestAdminSessionNormalizePreservesExplicitForceOptions(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{
		"force_derived_rebuild":      true,
		"force_world_rule_backfill":  true,
		"force_raw_world_rule_audit": true,
		"force_episode_backfill":     true,
		"force_hierarchy_backfill":   true,
	})
	for _, key := range []string{
		"force_derived_rebuild",
		"force_world_rule_backfill",
		"force_raw_world_rule_audit",
		"force_episode_backfill",
		"force_hierarchy_backfill",
	} {
		if !boolFromAny(meta[key]) {
			t.Fatalf("explicit %s option was not preserved: %#v", key, meta)
		}
	}
	force := true
	resume := false
	if !adminSessionNormalizeForceReindex(adminSessionNormalizeRequest{ForceReindex: &force, ResumeExisting: &resume}) {
		t.Fatal("explicit force_reindex=true must remain available")
	}
}

func TestAdminSessionNormalizeHasNoHistoricalHostSourceSynthesisContract(t *testing.T) {
	meta := adminSessionNormalizeClientMeta(map[string]any{
		"source": "explorer_session_normalize",
	})
	if _, exists := meta["session_normalize_inline_reprocessing"]; exists {
		t.Fatalf("Session Normalize retained inline Critic control: %#v", meta)
	}
	request := adminSessionNormalizeJobRequest(
		"sess-normalize-canonical-only",
		adminSessionNormalizeRequest{},
		nil,
	)
	if _, exists := request["source_observation_count"]; exists {
		t.Fatalf("Session Normalize retained historical host-source metadata: %#v", request)
	}
}

type canonicalRawReplaySessionNormalizeStore struct {
	*memoryAdmissionWorkerStore
	sources map[string]*store.MemorySourceRevision
}

func (f *canonicalRawReplaySessionNormalizeStore) SaveCriticInputSnapshot(
	_ context.Context,
	_ string,
	revision string,
	snapshotJSON string,
	snapshotHash string,
	_ time.Time,
) error {
	source := f.sources[revision]
	if source == nil {
		return store.ErrNotFound
	}
	source.CriticInputSnapshotJSON = snapshotJSON
	source.CriticInputSnapshotHash = snapshotHash
	return nil
}

func (f *canonicalRawReplaySessionNormalizeStore) RegisterAcceptedSourceRevision(
	_ context.Context,
	source *store.MemorySourceRevision,
) (store.SourceRevisionRegistration, error) {
	if source == nil {
		return store.SourceRevisionRegistration{}, errors.New("source is required")
	}
	if f.sources == nil {
		f.sources = map[string]*store.MemorySourceRevision{}
	}
	if existing := f.sources[source.SourceRevision]; existing != nil {
		return store.SourceRevisionRegistration{Idempotent: true}, nil
	}
	copySource := *source
	f.sources[source.SourceRevision] = &copySource
	return store.SourceRevisionRegistration{Inserted: true}, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) GetSourceRevision(
	_ context.Context,
	_ string,
	sourceRevision string,
) (*store.MemorySourceRevision, error) {
	source := f.sources[sourceRevision]
	if source == nil {
		return nil, store.ErrNotFound
	}
	copySource := *source
	return &copySource, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) IsSourceRevisionActive(
	ctx context.Context,
	sid string,
	sourceRevision string,
) (bool, error) {
	source, err := f.GetSourceRevision(ctx, sid, sourceRevision)
	if err != nil {
		return false, err
	}
	return source.LifecycleState == "active", nil
}

func (f *canonicalRawReplaySessionNormalizeStore) ListActiveSourceRevisions(
	_ context.Context,
	sid string,
	fromTurn int,
	toTurn int,
) ([]store.MemorySourceRevision, error) {
	out := []store.MemorySourceRevision{}
	for _, source := range f.sources {
		if source == nil ||
			source.ChatSessionID != sid ||
			source.LifecycleState != "active" ||
			(fromTurn > 0 && source.TurnIndex < fromTurn) ||
			(toTurn > 0 && source.TurnIndex > toTurn) {
			continue
		}
		out = append(out, *source)
	}
	return out, nil
}

func (f *canonicalRawReplaySessionNormalizeStore) CommitMemoryAdmission(
	ctx context.Context,
	item *store.MemoryAdmission,
) (store.MemoryAdmissionResult, error) {
	result, err := f.memoryAdmissionWorkerStore.CommitMemoryAdmission(ctx, item)
	if err == nil && item != nil && item.Memory != nil {
		f.memories = append(f.memories, *item.Memory)
	}
	return result, err
}

func TestAdminSessionNormalizeReplaysCanonicalRawLogsThroughSharedDerivationOwner(t *testing.T) {
	const sid = "sess-normalize-canonical-raw-replay"
	logs := []store.ChatLog{
		{ChatSessionID: sid, TurnIndex: 1, Role: "user", Content: "The traveler reaches the gate."},
		{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "The guard refuses entry until dawn."},
	}
	fake := &canonicalRawReplaySessionNormalizeStore{
		memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{
			Store: store.NewNoopStore(),
			logs:  logs,
			memories: []store.Memory{{
				ChatSessionID: sid,
				TurnIndex:     1,
				SummaryJSON:   `{"turn_summary":"preexisting partial memory"}`,
			}},
		},
		sources: map[string]*store.MemorySourceRevision{},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	oldClient := proxyHTTPClient
	criticContent := criticWireJSONForTest(map[string]any{"turn_summary": "The guard refused entry until dawn.", "importance_score": 6})
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{
					"content": criticContent,
				},
			}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	request := adminSessionNormalizeRequest{
		SkipRepair:  true,
		SkipReindex: true,
		ClientMeta: map[string]any{
			"critic": map[string]any{
				"api_key":    "test-key",
				"endpoint":   "https://api.example.com/v1",
				"model":      "critic",
				"provider":   "openai",
				"timeout_ms": 45000,
			},
		},
	}
	result, err := srv.runAdminSessionNormalize(context.Background(), sid, request, nil)
	if err != nil {
		t.Fatalf("runAdminSessionNormalize: %v", err)
	}
	rescan := mapFromAny(result["rescan"])
	if result["status"] != "ok" ||
		intFromAny(rescan["candidate_count"], 0) != len(logs)/2 ||
		intFromAny(rescan["succeeded"], 0) != len(logs)/2 ||
		intFromAny(rescan["failed"], -1) != 0 ||
		len(fake.admissions) != len(logs)/2 ||
		len(fake.enqueuedJobs) != 0 {
		t.Fatalf(
			"status=%v rescan=%#v admissions=%d queued=%d",
			result["status"], rescan, len(fake.admissions), len(fake.enqueuedJobs),
		)
	}
	if _, exists := result["source_revisions"]; exists {
		t.Fatalf("removed historical source stage leaked into result: %#v", result)
	}
	if len(fake.sources) != len(logs)/2 {
		t.Fatalf("canonical replay sources=%d, want=%d", len(fake.sources), len(logs)/2)
	}
	for _, source := range fake.sources {
		if !strings.HasPrefix(source.SourceRevision, "sar_") ||
			!strings.HasPrefix(source.LogicalTurnID, "canonical_turn_") {
			t.Fatalf("canonical raw replay synthesized a live host source: %#v", source)
		}
	}
}

func TestAdminSessionNormalizeDefersReindexForQueuedCanonicalReplay(t *testing.T) {
	reasons := adminSessionNormalizeReindexDeferredReasons(map[string]any{
		"status":   "deferred",
		"deferred": 2,
		"queued":   2,
	})
	joined := strings.Join(reasons, ",")
	if !strings.Contains(joined, "rescan:deferred") ||
		!strings.Contains(joined, "rescan:pending_reprocessing") {
		t.Fatalf("queued canonical replay did not defer reindex: %#v", reasons)
	}
	if strings.Contains(joined, "source_revisions") {
		t.Fatalf("removed historical source stage still controls reindex: %#v", reasons)
	}
}

type blockingSessionNormalizeStore struct {
	*memoryFakeStore
	entered chan struct{}
}

func (f *blockingSessionNormalizeStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	if from == 1 && to == 1 {
		close(f.entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.memoryFakeStore.ListChatLogs(ctx, sid, from, to)
}

func TestAdminSessionNormalizeRawRepairReportsRealCandidateCountAndCompletes(t *testing.T) {
	userText := "The traveler reaches the old gate."
	assistantText := "The guard refuses entry until dawn."
	srv := NewServer(config.Default())
	srv.Store = &memoryFakeStore{}
	srv.StoreOpenError = nil

	updates := []map[string]any{}
	result, err := srv.runAdminSessionNormalize(context.Background(), "sess-normalize-complete", adminSessionNormalizeRequest{
		RepairEntries: []dto.ChatLogRepairEntryRequest{{
			TurnIndex:        1,
			UserContent:      &userText,
			AssistantContent: &assistantText,
		}},
		SkipRescan:  true,
		SkipReindex: true,
	}, func(progress map[string]any) {
		updates = append(updates, cloneMapAny(progress))
	})
	if err != nil {
		t.Fatalf("runAdminSessionNormalize: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("result = %#v, want ok", result)
	}
	sawRawRepair := false
	for _, update := range updates {
		if update["stage"] != "raw_repair_replay" {
			continue
		}
		sawRawRepair = true
		if got := intFromAny(update["candidate_count"], 0); got != 1 {
			t.Fatalf("raw repair candidate_count = %d, want 1: %#v", got, update)
		}
	}
	if !sawRawRepair {
		t.Fatalf("raw_repair_replay progress missing: %#v", updates)
	}
}

func TestAdminSessionNormalizeKeepsRawConflictForReviewWithoutDerivedReplay(t *testing.T) {
	dbUser := "database user"
	activeUser := "active chat user"
	missingAssistant := "verified missing assistant"
	fake := newRepairReplayMutableStore([]store.ChatLog{
		{ChatSessionID: "sess-normalize-conflict", TurnIndex: 2, Role: "user", Content: dbUser},
	})
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	result, err := srv.runAdminSessionNormalize(context.Background(), "sess-normalize-conflict", adminSessionNormalizeRequest{
		RepairEntries: []dto.ChatLogRepairEntryRequest{{
			TurnIndex:        2,
			UserContent:      &activeUser,
			AssistantContent: &missingAssistant,
		}},
		TurnIndices: []int{2},
		SkipReindex: true,
	}, nil)
	if err != nil {
		t.Fatalf("runAdminSessionNormalize: %v", err)
	}
	if !reflect.DeepEqual(intSliceFromAny(result["review_needed_turns"]), []int{2}) {
		t.Fatalf("review turns=%#v result=%#v", result["review_needed_turns"], result)
	}
	rescan := mapFromAny(result["rescan"])
	if stringFromMap(rescan, "reason") != "all_requested_turns_require_raw_review" ||
		intFromAny(rescan["candidate_count"], -1) != 0 {
		t.Fatalf("conflicting raw turn reached derived replay: %#v", rescan)
	}
	repair := mapFromAny(result["repair_replay"])
	if intFromAny(repair["total_conflict_role_count"], 0) != 1 ||
		intFromAny(repair["total_repaired_role_count"], 0) != 1 ||
		!reflect.DeepEqual(intSliceFromAny(repair["conflict_turns"]), []int{2}) {
		t.Fatalf("repair conflict result=%#v", repair)
	}
	if len(fake.savedChatLogs) != 1 || fake.savedChatLogs[0].Role != "assistant" || fake.savedChatLogs[0].Content != missingAssistant {
		t.Fatalf("independently verified assistant was not preserved: %#v", fake.savedChatLogs)
	}
}

func TestAdminSessionNormalizeTreatsAssistantOnlyAsProcessableAndUserOnlyAsReview(t *testing.T) {
	const sid = "sess-normalize-role-counts"
	fake := &memoryFakeStore{chatLogs: []store.ChatLog{
		{ChatSessionID: sid, TurnIndex: 1, Role: "assistant", Content: "assistant-only output"},
		{ChatSessionID: sid, TurnIndex: 2, Role: "user", Content: "user-only input"},
		{ChatSessionID: sid, TurnIndex: 3, Role: "user", Content: "complete input"},
		{ChatSessionID: sid, TurnIndex: 3, Role: "assistant", Content: "complete output"},
	}}
	srv := NewServer(config.Default())
	srv.Store = fake

	snapshot, warnings := srv.adminSessionNormalizeSnapshot(context.Background(), sid)
	if len(warnings) != 0 ||
		intFromAny(snapshot["raw_turns"], 0) != 3 ||
		intFromAny(snapshot["raw_complete_turns"], 0) != 1 ||
		intFromAny(snapshot["raw_assistant_only_turns"], 0) != 1 ||
		intFromAny(snapshot["raw_user_only_turns"], 0) != 1 ||
		intFromAny(snapshot["raw_processable_turns"], 0) != 2 {
		t.Fatalf("snapshot=%+v warnings=%+v", snapshot, warnings)
	}
	if got := adminSessionNormalizeConflictTurns(snapshot); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("review turns=%v, want only user-only turn 2", got)
	}
}

func TestAdminSessionNormalizeCancellationReachesBlockedRawChatQuery(t *testing.T) {
	userText := "The traveler reaches the old gate."
	assistantText := "The guard refuses entry until dawn."
	fake := &blockingSessionNormalizeStore{
		memoryFakeStore: &memoryFakeStore{},
		entered:         make(chan struct{}),
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := srv.runAdminSessionNormalize(ctx, "sess-normalize-cancel", adminSessionNormalizeRequest{
			RepairEntries: []dto.ChatLogRepairEntryRequest{{
				TurnIndex:        1,
				UserContent:      &userText,
				AssistantContent: &assistantText,
			}},
			SkipRescan:  true,
			SkipReindex: true,
		}, nil)
		resultCh <- err
	}()

	select {
	case <-fake.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("session normalize did not reach raw chat query")
	}
	cancel()
	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runAdminSessionNormalize error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session normalize did not stop after caller cancellation")
	}
}
