package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Long, mostly distinct queries exposed quadratic duplicate filtering in
// particle expansion. Keep the production function under a scaling benchmark;
// result checks also detect dropping the expansion or retaining duplicate stems.
// No wall-clock threshold is imposed on ordinary tests or user requests.
func BenchmarkMemoryRestorationRecallTerms(b *testing.B) {
	for _, count := range []int{128, 512, 2048} {
		b.Run(fmt.Sprintf("terms_%d", count), func(b *testing.B) {
			var input strings.Builder
			input.WriteString("x ") // Separate the shortest tier from the useful cues.
			want := make([]string, 0, count*3)
			for i := 0; i < count; i++ {
				stem := "기억단서" + string(rune('가'+i))
				first, second := stem+"을", stem+"은"
				input.WriteString(first + " " + second + " ")
				want = append(want, first, stem, second)
			}
			query := input.String()
			if got := prepareTurnDistinctiveRecallTerms(query); !slices.Equal(got, want) {
				b.Fatal("particle forms lost their first-seen order or shared stems were duplicated")
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(query)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				prepareTurnDistinctiveRecallTerms(query)
			}
		})
	}
}

// Isolated external-boundary fixtures. Production route, hydration, selection,
// budgets and final delivery-plan assembly are unchanged and run together.
type revalidationStore struct {
	*turnRecordingStore
	reads []string
}

func (s *revalidationStore) ListMemories(_ context.Context, sid string, from, to int) ([]store.Memory, error) {
	s.reads = append(s.reads, fmt.Sprintf("memories:%s:%d:%d", sid, from, to))
	out := []store.Memory{}
	for _, m := range s.returnMemories {
		if m.ChatSessionID == sid && (from <= 0 || m.TurnIndex >= from) && (to <= 0 || m.TurnIndex <= to) {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *revalidationStore) SaveEvidence(_ context.Context, e *store.DirectEvidence) error {
	e.ID = int64(len(s.returnEvidence) + 1)
	s.savedEvidence = append(s.savedEvidence, e)
	s.returnEvidence = append(s.returnEvidence, *e)
	return nil
}

type revalidationSourceStore struct {
	*revalidationStore
	store.SourceRevisionStore
	sourceReads []string
	precise     []store.PreciseMemoryUnit
}

func (s *revalidationSourceStore) MemoryDerivationLifecycleEnabled() bool { return true }
func (s *revalidationSourceStore) GetSourceRevision(_ context.Context, sid, revision string) (*store.MemorySourceRevision, error) {
	s.sourceReads = append(s.sourceReads, sid+":"+revision)
	for _, m := range s.returnMemories {
		if m.ChatSessionID == sid && revision == fmt.Sprintf("offline-rev-%d", m.ID) {
			return &store.MemorySourceRevision{ChatSessionID: sid, SourceRevision: revision, TurnIndex: m.TurnIndex, LifecycleState: "active"}, nil
		}
	}
	return nil, store.ErrNotFound
}
func (s *revalidationSourceStore) IsSourceRevisionActive(ctx context.Context, sid, revision string) (bool, error) {
	_, err := s.GetSourceRevision(ctx, sid, revision)
	return err == nil, err
}
func (s *revalidationSourceStore) ListGeneralVectorPreciseMemoryUnits(_ context.Context, sid string) ([]store.PreciseMemoryUnit, error) {
	out := []store.PreciseMemoryUnit{}
	for _, u := range s.precise {
		if u.ChatSessionID == sid && store.PreciseMemoryGeneralVectorEligible(&u) {
			out = append(out, u)
		}
	}
	return out, nil
}

type revalidationAdmissionStore struct {
	*revalidationSourceStore
	admissions []*store.MemoryAdmission
	units      []store.PreciseMemoryUnit
}

func (s *revalidationAdmissionStore) MemoryAdmissionWritesEnabled() bool { return true }
func (s *revalidationAdmissionStore) CommitMemoryAdmission(_ context.Context, a *store.MemoryAdmission) (store.MemoryAdmissionResult, error) {
	s.admissions = append(s.admissions, a)
	if a.Memory != nil {
		nextID := int64(1)
		for _, m := range s.returnMemories {
			if m.ID >= nextID {
				nextID = m.ID + 1
			}
		}
		a.Memory.ID = nextID
		s.savedMemories = append(s.savedMemories, a.Memory)
		s.returnMemories = append(s.returnMemories, *a.Memory)
	}
	for _, e := range a.Evidence {
		e.ID = int64(len(s.returnEvidence) + 1)
		s.returnEvidence = append(s.returnEvidence, *e)
	}
	for _, u := range a.PreciseUnits {
		s.units = append(s.units, *u)
	}
	return store.MemoryAdmissionResult{MemoryInserted: a.Memory != nil, EvidenceInserted: len(a.Evidence), PreciseInserted: len(a.PreciseUnits), VectorOperations: len(a.Vectors), CommittedResultHash: a.ResultHash}, nil
}
func (s *revalidationAdmissionStore) ListGeneralVectorPreciseMemoryUnits(_ context.Context, sid string) ([]store.PreciseMemoryUnit, error) {
	out := []store.PreciseMemoryUnit{}
	for _, u := range s.units {
		if u.ChatSessionID == sid && store.PreciseMemoryGeneralVectorEligible(&u) {
			out = append(out, u)
		}
	}
	return out, nil
}

type revalidationVector struct {
	vector.VectorStore
	t            *testing.T
	sid          string
	weakerRecent bool
	precise      bool
	calls        []string
}

func (v *revalidationVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "isolated-revalidation", ModelReady: true, TotalCount: 6}, nil
}
func (v *revalidationVector) Search(_ context.Context, sid string, q []float32, k int, filter string) ([]vector.VectorDocument, error) {
	v.calls = append(v.calls, fmt.Sprintf("sid=%s q=%v k=%d filter=%s", sid, q, k, filter))
	if sid != v.sid || len(q) != 3 || (q[0] != 1 && q[0] != 2) || k < 1 {
		v.t.Errorf("unexpected vector request: %s", v.calls[len(v.calls)-1])
		return nil, fmt.Errorf("unexpected vector request")
	}
	preciseFilter := filter == `source_table == "precise_memory_units"`
	if filter != `tier == "memory"` && filter != fmt.Sprintf("chat_session_id == %q", sid) && !(v.precise && preciseFilter) {
		v.t.Errorf("unexpected filter: %s", filter)
		return nil, fmt.Errorf("unexpected filter")
	}
	preciseDoc := vector.VectorDocument{ID: "precise_memory:" + sid + ":old-habit", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "old-habit", SchemaVersion: store.PreciseMemoryUnitContract, Similarity: .9, SimilarityAvailable: true, SimilaritySource: "cosine", Metadata: map[string]any{"source_revision": "offline-rev-1"}}
	if preciseFilter {
		if q[0] == 1 {
			return []vector.VectorDocument{preciseDoc}, nil
		}
		return nil, vector.ErrNotFound
	}
	doc := func(id int, sim float64) vector.VectorDocument {
		return vector.VectorDocument{ID: fmt.Sprintf("memory:%s:%d", sid, id), ChatSessionID: sid, Tier: "memory", SourceTable: "memories", SourceRowID: fmt.Sprint(id), Similarity: sim, SimilarityAvailable: true, SimilaritySource: "cosine", Metadata: map[string]any{"source_revision": fmt.Sprintf("offline-rev-%d", id)}}
	}
	if q[0] == 1 {
		if v.precise && filter != `tier == "memory"` {
			return []vector.VectorDocument{preciseDoc, doc(1, .82)}, nil
		}
		return []vector.VectorDocument{doc(1, .82)}, nil
	}
	out := []vector.VectorDocument{}
	for id := 2; id <= 6 && len(out) < k; id++ {
		sim := .99 - float64(id)/100
		if v.weakerRecent {
			sim -= .4
		}
		out = append(out, doc(id, sim))
	}
	return out, nil
}
func revalidationHTTP(t *testing.T, srv *Server, body map[string]any) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}
func revalidationEmit(t *testing.T, v map[string]any) {
	t.Helper()
	b, _ := json.Marshal(v)
	t.Log("REVALIDATE " + string(b))
}

func TestMemoryRestorationHTTPQueryCompetition(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid = "offline-revalidation"
	type scenario struct {
		name, raw    string
		recent, weak bool
		k            int
		precise      bool
	}
	cases := []scenario{
		{"primary_only", "예전 운동을 다시 시작하자.", false, false, 5, false},
		{"recent_pair_stronger", "예전 운동을 다시 시작하자.", true, false, 5, false},
		{"recent_pair_weaker", "예전 운동을 다시 시작하자.", true, true, 5, false},
		{"recent_pair_larger_search", "예전 운동을 다시 시작하자.", true, false, 6, false},
		{"recent_pair_exact_word", "스트레칭을", true, false, 5, false},
		{"recent_pair_inflection", "스트레칭", true, false, 5, false},
		{"recent_pair_precise_rescue", "예전 운동을 다시 시작하자.", true, false, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recentUser := "도적 수사를 이어간다."
			recentAssistant := "마을에서 도적 수사를 이어가며 탐문한다."
			recentQuery := "user:\n" + recentUser + "\nassistant:\n" + recentAssistant
			embeds := []string{}
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://offline.example.test/v1/embeddings" || r.Method != http.MethodPost {
					t.Errorf("unexpected HTTP request %s %s", r.Method, r.URL)
					return nil, fmt.Errorf("unexpected HTTP request")
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					return nil, err
				}
				input := extractionStringFromAny(payload["input"])
				embeds = append(embeds, input)
				q := 1
				if input == recentQuery {
					q = 2
				} else if input != c.raw {
					t.Errorf("unexpected embedding input %q", input)
					return nil, fmt.Errorf("unexpected input")
				}
				return &http.Response{StatusCode: 200, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"offline-embed","data":[{"embedding":[%d,0.2,0.3]}]}`, q)))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()
			mems := []store.Memory{{ID: 1, ChatSessionID: sid, TurnIndex: 1, Importance: .7, SummaryJSON: `{"turn_summary":"매일 아침 스트레칭을 하며 허리와 어깨를 풀었다."}`}}
			for id := 2; id <= 6; id++ {
				mems = append(mems, store.Memory{ID: int64(id), ChatSessionID: sid, TurnIndex: 10 + id, Importance: .5, SummaryJSON: fmt.Sprintf(`{"turn_summary":"도적 수사 단서번호%d가 기록되었다."}`, id)})
			}
			st := &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{returnMemories: mems}}}
			if c.precise {
				st.precise = []store.PreciseMemoryUnit{{UnitID: "old-habit", ChatSessionID: sid, SourceTurnStart: 1, SourceTurnEnd: 1, SourceRevision: "offline-rev-1", Kind: "event", Subtype: "observed_event", PayloadJSON: `{"summary":"매일 아침 스트레칭을 하며 허리와 어깨를 풀었다."}`, Visibility: "public", EpistemicMode: "direct", AdmissionState: "committed", ReviewState: "source_observed", LifecycleState: "active"}}
			}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			cfg.ChromaEndpoint = "http://offline.example.test"
			cfg.Readiness.ChromaConfigured = true
			srv := NewServer(cfg)
			srv.Store = st
			srv.StoreOpenError = nil
			vec := &revalidationVector{t: t, sid: sid, weakerRecent: c.weak, precise: c.precise}
			srv.Vector = vec
			srv.VectorOpenError = nil
			messages := []map[string]any{}
			if c.recent {
				messages = append(messages, map[string]any{"role": "user", "content": recentUser}, map[string]any{"role": "assistant", "content": recentAssistant})
			}
			messages = append(messages, map[string]any{"role": "user", "content": c.raw})
			recentMessages := []map[string]any{}
			if c.recent {
				recentMessages = messages[:2]
			}
			response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": sid, "turn_index": 17, "raw_user_input": c.raw, "messages": messages, "recent_conversation_messages": recentMessages,
				"client_meta": map[string]any{"embedding": map[string]any{"api_key": "offline-key", "endpoint": "https://offline.example.test/v1", "model": "offline-embed", "provider": "openai", "timeout_ms": 30000}},
				"settings":    map[string]any{"max_injection_chars": 30000, "injection_enabled": true, "input_context_enabled": false, "top_k": c.k, "core_objective_memory_max_items": 5}})
			pack := mapFromAny(response["injection_pack"])
			plan := mapFromAny(pack["memory_delivery_plan"])
			final := extractionStringFromAny(plan["final_text"])
			selected := strings.Contains(final, "허리와 어깨")
			payloadJSON, _ := json.Marshal(response["payload_application_plan"])
			injectionText := extractionStringFromAny(pack["injection_text"])
			revalidationEmit(t, map[string]any{"case": c.name, "selected_old": selected, "injection_text_old": strings.Contains(injectionText, "허리와 어깨"), "payload_plan_present": response["payload_application_plan"] != nil, "payload_plan_old": strings.Contains(string(payloadJSON), "허리와 어깨"), "embeddings": embeds, "search_calls": vec.calls, "store_reads": st.reads, "source_reads": st.sourceReads, "final": final, "semantic_fact_vector_count": plan["semantic_fact_vector_count"], "relevance_query_source": plan["relevance_query_source"]})
			if len(embeds) == 0 || len(vec.calls) == 0 || len(st.reads) == 0 {
				t.Error("production path did not reach fixture boundaries")
			}
			if len(st.sourceReads) == 0 {
				t.Error("active source revision checks were bypassed")
			}
			if selected != strings.Contains(injectionText, "허리와 어깨") {
				t.Error("final memory and injection text disagree")
			}
			if response["payload_application_plan"] != nil && selected != strings.Contains(string(payloadJSON), "허리와 어깨") {
				t.Error("final memory and payload application plan disagree")
			}
			if !selected {
				t.Error("positive control: strongest/within-limit/precise semantic hit failed to reach HTTP delivery plan")
			}
		})
	}
}

func TestMemoryRestorationStoredMixedDelivery(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid = "offline-mixed-revalidation"
	for _, kind := range []string{"plain", "private_summary_only", "private_public_event", "private_public_excerpt"} {
		t.Run(kind, func(t *testing.T) {
			p := map[string]any{"turn_summary": "민우는 스트레칭을 하며 어깨를 풀었다.", "importance_score": 7}
			if kind != "plain" {
				p["subjective_entity_memories"] = []any{map[string]any{"owner_entity_name": "아인즈", "owner_entity_key": "ainz", "memory_text": "동료를 믿을지 혼자 고민했다.", "owner_visibility": "hidden"}}
			}
			if kind == "private_public_event" {
				p["narrative_events"] = []any{map[string]any{"event": "민우는 스트레칭을 하며 어깨를 풀었다.", "visibility": "public"}}
			}
			if kind == "private_public_excerpt" {
				p["evidence_excerpts"] = []any{"민우는 스트레칭을 하며 어깨를 풀었다."}
			}
			p = normalizeCriticExtraction(p)
			st := &revalidationStore{turnRecordingStore: &turnRecordingStore{}}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			srv := NewServer(cfg)
			srv.Store = st
			srv.StoreOpenError = nil
			result := srv.saveCriticExtractionArtifacts(context.Background(), sid, 1, p, "민우는 스트레칭을 하며 어깨를 풀었다. 아인즈는 동료를 믿을지 혼자 고민했다.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
			if result.Errors != 0 || len(st.savedMemories) != 1 {
				t.Fatalf("failed to persist fixture: %+v saved=%d", result, len(st.savedMemories))
			}
			m := *st.savedMemories[0]
			m.ID = 1
			st.returnMemories = []store.Memory{m}
			response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": sid, "turn_index": 2, "raw_user_input": "스트레칭을", "settings": map[string]any{"max_injection_chars": 30000, "injection_enabled": true, "input_context_enabled": false, "top_k": 5}})
			plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
			final := extractionStringFromAny(plan["final_text"])
			revalidationEmit(t, map[string]any{"case": kind, "canonical_public": strings.Contains(m.SummaryJSON, "어깨"), "private_normalized_count": len(sliceFromAny(p["subjective_entity_memories"])), "searchable_public": strings.Contains(memorySearchTextFromMemory(m).Text, "어깨"), "selected_public": strings.Contains(final, "어깨"), "selected_private": strings.Contains(final, "동료를 믿을지"), "embedding_status": result.EmbeddingStatus, "final": final})
			if kind != "private_summary_only" && (!strings.Contains(final, "어깨") || !strings.Contains(memorySearchTextFromMemory(m).Text, "어깨")) {
				t.Error("public source evidence missing from search or final memory")
			}
			if strings.Contains(final, "동료를 믿을지") {
				t.Error("unscoped private memory leaked")
			}
		})
	}
}

func TestMemoryRestorationAcceptedMixedDelivery(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid = "offline-accepted-revalidation"
	for _, kind := range []string{"plain", "private_summary_only", "private_public_event", "private_public_excerpt"} {
		t.Run(kind, func(t *testing.T) {
			content := "민우는 스트레칭을 하며 어깨를 풀었다. 아인즈는 동료를 믿을지 혼자 고민했다."
			p := map[string]any{"turn_summary": "민우는 스트레칭을 하며 어깨를 풀었다.", "importance_score": 7}
			if kind != "plain" {
				p["subjective_entity_memories"] = []any{map[string]any{"owner_entity_name": "아인즈", "owner_entity_key": "ainz", "memory_text": "동료를 믿을지 혼자 고민했다.", "owner_visibility": "hidden"}}
			}
			if kind == "private_public_event" {
				p["narrative_events"] = []any{map[string]any{"event": "민우는 스트레칭을 하며 어깨를 풀었다.", "visibility": "public", "evidence_excerpt": "민우는 스트레칭을 하며 어깨를 풀었다."}}
			}
			if kind == "private_public_excerpt" {
				p["evidence_excerpts"] = []any{"민우는 스트레칭을 하며 어깨를 풀었다."}
			}
			p = normalizeCriticExtraction(p)
			st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{}}}}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			srv := NewServer(cfg)
			srv.Store = st
			srv.StoreOpenError = nil
			ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-1"})
			result := srv.saveCriticExtractionArtifacts(ctx, sid, 1, p, content, completeTurnEmbeddingConfig{}, time.Unix(300, 0))
			if result.Errors != 0 || len(st.admissions) != 1 || len(st.returnMemories) != 1 {
				t.Fatalf("admission failed: %+v calls=%d memories=%d", result, len(st.admissions), len(st.returnMemories))
			}
			response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": sid, "turn_index": 2, "raw_user_input": "스트레칭을", "settings": map[string]any{"max_injection_chars": 30000, "injection_enabled": true, "input_context_enabled": false, "top_k": 5}})
			plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
			final := extractionStringFromAny(plan["final_text"])
			general, _ := st.ListGeneralVectorPreciseMemoryUnits(context.Background(), sid)
			revalidationEmit(t, map[string]any{"case": "accepted_" + kind, "canonical_public": strings.Contains(st.returnMemories[0].SummaryJSON, "어깨"), "searchable_public": strings.Contains(memorySearchTextFromMemory(st.returnMemories[0]).Text, "어깨"), "selected_public": strings.Contains(final, "어깨"), "selected_private": strings.Contains(final, "동료를 믿을지"), "admitted_evidence": len(st.returnEvidence), "admitted_units": len(st.units), "general_units": len(general), "final": final})
			if kind != "private_summary_only" && (!strings.Contains(final, "어깨") || !strings.Contains(memorySearchTextFromMemory(st.returnMemories[0]).Text, "어깨")) {
				t.Error("accepted public source evidence missing from search or final memory")
			}
			if strings.Contains(final, "동료를 믿을지") {
				t.Error("unscoped private memory leaked")
			}
		})
	}
}

func TestMemoryRestorationKeywordHTTP(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid = "offline-keyword-revalidation"
	for _, q := range []string{"스트레칭을", "스트레칭", "사건이 끝났으니 오랜만에 스트레칭을 다시 하자."} {
		st := &revalidationStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: sid, TurnIndex: 1, Importance: .7, SummaryJSON: `{"turn_summary":"민우는 매일 아침 스트레칭을 하며 허리와 어깨를 풀었다."}`},
			{ID: 2, ChatSessionID: sid, TurnIndex: 15, Importance: .7, SummaryJSON: `{"turn_summary":"도적 사건이 끝났고 마을에 평온이 돌아왔다."}`},
		}}}
		cfg := config.Default()
		cfg.StoreMode = config.StoreModeDualShadow
		srv := NewServer(cfg)
		srv.Store = st
		srv.StoreOpenError = nil
		response := revalidationHTTP(t, srv, map[string]any{"chat_session_id": sid, "turn_index": 16, "raw_user_input": q, "settings": map[string]any{"max_injection_chars": 30000, "injection_enabled": true, "input_context_enabled": false, "top_k": 5}})
		plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
		final := extractionStringFromAny(plan["final_text"])
		if !strings.Contains(final, "허리와 어깨") {
			t.Fatalf("explicit cue did not reach delivery: query=%q final=%q", q, final)
		}
		revalidationEmit(t, map[string]any{"case": "keyword_http", "query": q, "selected_old": strings.Contains(final, "허리와 어깨"), "final": final})
	}
}

func TestMemoryRestorationAcceptedSoftPrune(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	const sid = "offline-prune-revalidation"
	st := &revalidationAdmissionStore{revalidationSourceStore: &revalidationSourceStore{revalidationStore: &revalidationStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{
		{ID: 11, ChatSessionID: sid, TurnIndex: 1, Importance: .9, SummaryJSON: `{"turn_summary":"도적 사건은 끝났다. 민우는 매일 스트레칭을 한다."}`},
		{ID: 12, ChatSessionID: sid, TurnIndex: 1, Importance: .9, SummaryJSON: `{"turn_summary":"다른 기억"}`},
	}}}}}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = st
	srv.StoreOpenError = nil
	p := normalizeCriticExtraction(map[string]any{"turn_summary": "오늘 대화를 마무리했다.", "prune_targets": []any{"도적 사건"}})
	ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{ContractVersion: completeTurnSourceAcceptanceContract, Revision: "offline-rev-13"})
	result := srv.saveCriticExtractionArtifacts(ctx, sid, 16, p, "오늘 대화를 마무리했다.", completeTurnEmbeddingConfig{}, time.Unix(300, 0))
	if result.Errors != 0 || len(st.admissions) != 1 {
		t.Fatalf("accepted prune save failed: %+v", result)
	}
	revalidationEmit(t, map[string]any{"case": "accepted_soft_prune", "updates": st.updatedImportance, "canonical_habit_retained": strings.Contains(st.returnMemories[0].SummaryJSON, "스트레칭"), "admission_calls": len(st.admissions), "new_summary_mentions_target": strings.Contains(extractionStringFromAny(p["turn_summary"]), "도적 사건"), "new_row_id": st.admissions[0].Memory.ID, "new_row_json_mentions_target": strings.Contains(st.admissions[0].Memory.SummaryJSON, "도적 사건")})
	if _, ok := st.updatedImportance[12]; ok {
		t.Error("unrelated memory lowered")
	}
	if len(st.updatedImportance) != 0 {
		t.Fatalf("prune hint changed whole memory importance: %v", st.updatedImportance)
	}
}
