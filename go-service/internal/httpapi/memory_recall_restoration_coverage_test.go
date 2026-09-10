package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Compare the old five-result search with budget-sized candidate windows at
// fixed external ranks. This measures candidate/packing coverage, not an actual
// embedding model's recall or Chroma latency. Out-of-window cases stay visible.
func TestMemoryRestorationCandidateWindowCoverage(t *testing.T) {
	const sid = "window-coverage"
	const target = "The group practiced ribbon binding together every evening."
	for _, budget := range []int{2048, 9000, 18000} {
		for _, size := range []int{120, 700} {
			for _, rank := range []int{10, 40, 80, 160} {
				t.Run(fmt.Sprintf("budget%d/chars%d/rank%d", budget, size, rank), func(t *testing.T) {
					memories := make([]store.Memory, 180)
					hits := make([]map[string]any, len(memories))
					for i := range memories {
						text := fmt.Sprintf("Inventory ledger batch %d was filed. ", i+1)
						importance := .3
						if i+1 == rank {
							text, importance = target, 1
						}
						text += strings.Repeat(" Details were recorded.", maxInt((size-len(text))/23, 0))
						memories[i] = store.Memory{ID: int64(i + 1), ChatSessionID: sid, TurnIndex: i + 1, Importance: importance, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": text})}
						hits[i] = map[string]any{"id": fmt.Sprintf("memory:%s:%d", sid, i+1), "chat_session_id": sid, "source_table": "memories", "source_row_id": fmt.Sprint(i + 1), "tier": "memory", "similarity": .97 - float64(i)/10000, "similarity_source": "cosine"}
					}
					for _, limit := range []int{5, prepareTurnMemoryCandidateLimit(budget)} {
						returned := hits[:minInt(limit, len(hits))]
						shadow := map[string]any{"status": "ready", "search_result": "ok", "memory_search_result": "ok", "memory_search_results": returned}
						assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
							limit, budget, "Let us resume our usual exercise.", "default", nil, shadow, nil, priorityMemoryTestContext(5))
						candidate := false
						for _, seed := range assembly.PriorityFactSeeds {
							candidate = candidate || strings.Contains(seed.Fact.Text, target)
						}
						if candidate != (rank <= limit) {
							t.Errorf("rank %d limit %d: candidate=%t", rank, limit, candidate)
						}
						final := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
						delivered := strings.Contains(final, target)
						if candidate && !delivered {
							t.Errorf("important in-window evidence missing: rank=%d limit=%d", rank, limit)
						}
						if len([]rune(final)) > budget {
							t.Error("wider candidate retrieval exceeded final memory budget")
						}
						t.Logf("budget=%d source_chars~%d target_rank=%d requested=%d returned=%d candidate=%t delivered=%t final_chars=%d", budget, size, rank, limit, len(returned), candidate, delivered, len([]rune(final)))
					}
				})
			}
		}
	}
}

type restorationRepeatedHitVector struct{ vector.VectorStore }

func (v *restorationRepeatedHitVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", ModelReady: true}, nil
}
func (v *restorationRepeatedHitVector) Search(_ context.Context, sid string, q []float32, _ int, _ string) ([]vector.VectorDocument, error) {
	return []vector.VectorDocument{{ID: "memory:" + sid + ":1", ChatSessionID: sid, Tier: "memory", SourceTable: "memories", SourceRowID: "1", Similarity: float64(q[0]) / 10, SimilarityAvailable: true}}, nil
}

func TestMemoryRestorationQueryProvenanceSurvivesFailedHistoryEmbedding(t *testing.T) {
	const current = "resume exercise"
	const recent = "user:\nsuccess\nassistant:\nolder continuity"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		input := extractionStringFromAny(body["input"])
		if strings.Contains(input, "failed continuity") {
			return nil, fmt.Errorf("fixture failed history embedding")
		}
		value := 8
		if input == recent {
			value = 9
		} else if input != current {
			t.Errorf("unexpected input %q", input)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"data":[{"embedding":[%d,0.2,0.3]}]}`, value)))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://offline-index.invalid"
	srv := NewServer(cfg)
	srv.Vector = &restorationRepeatedHitVector{}
	srv.VectorOpenError = nil
	currentInput := current
	req := dto.PrepareTurnRequest{ChatSessionID: "provenance", RawUserInput: &currentInput,
		Messages:   []map[string]any{{"role": "user", "content": "success"}, {"role": "assistant", "content": "older continuity"}, {"role": "user", "content": "failure"}, {"role": "assistant", "content": "failed continuity"}},
		ClientMeta: map[string]any{"embedding": map[string]any{"provider": "custom", "endpoint": "https://offline.example.test/v1", "api_key": "fixture", "model": "fixture", "timeout_ms": 30000}}}
	req.Settings.ApplyDefaults()
	two := 2
	req.Settings.RecentConversationReferenceCount = &two
	shadow := srv.prepareTurnVectorShadow(context.Background(), req, 5)
	if intFromAny(shadow["query_history_embedding_error_count"], 0) != 1 {
		t.Fatal("fixture did not exercise failed history embedding")
	}
	hits := prepareTurnVectorMemorySearchResultMaps(shadow)
	if len(hits) != 1 {
		t.Fatalf("same canonical hit was not merged: %d", len(hits))
	}
	observations := sliceFromAny(hits[0]["recall_queries"])
	if len(observations) != 2 || mapFromAny(observations[0])["query"] != current || mapFromAny(observations[1])["query"] != recent {
		t.Fatalf("query identity shifted after failed embedding: %v", observations)
	}
	memories := []store.Memory{{ID: 1, ChatSessionID: req.ChatSessionID, TurnIndex: 1, Importance: .9, SummaryJSON: `{"turn_summary":"The group practiced ribbon binding together."}`}}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 5, 9000, current, "default", nil, shadow, nil, priorityMemoryTestContext(5))
	seen := false
	for _, raw := range prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryPlan["priority_items"]) {
		item := mapFromAny(raw)
		if strings.Contains(extractionStringFromAny(item["complete_text"]), "ribbon binding") {
			seen = true
			if len(sliceFromAny(mapFromAny(item["score_lineage"])["recall_queries"])) != 2 {
				t.Error("final candidate lost per-query observations")
			}
		}
	}
	if !seen {
		t.Fatal("retrieved fact never reached final candidates")
	}
}

func TestMemoryRestorationFactsPrecedeSummaryRenderDeduplication(t *testing.T) {
	memories := []store.Memory{
		{ID: 1, ChatSessionID: "facts", TurnIndex: 1, Importance: .8, SummaryJSON: `{"turn_summary":"The courtyard meeting ended.","narrative_events":[{"event":"The brass key was entrusted to the caretaker.","visibility":"public"}]}`},
		{ID: 2, ChatSessionID: "facts", TurnIndex: 2, Importance: .9, SummaryJSON: `{"turn_summary":"The courtyard meeting ended.","narrative_events":[{"event":"The gate bell was repaired before dusk.","visibility":"public"}]}`},
	}
	hits := []map[string]any{}
	for _, m := range memories {
		hits = append(hits, map[string]any{"id": fmt.Sprintf("memory:facts:%d", m.ID), "chat_session_id": "facts", "source_table": "memories", "source_row_id": fmt.Sprint(m.ID), "tier": "memory", "similarity": .9})
	}
	shadow := map[string]any{"status": "ready", "search_result": "ok", "memory_search_result": "ok", "memory_search_results": hits}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, 1, 9000, "What was arranged?", "default", nil, shadow, nil, priorityMemoryTestContext(1))
	final := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	for _, phrase := range []string{"brass key", "gate bell"} {
		if !strings.Contains(final, phrase) {
			t.Errorf("distinct fact lost behind duplicate summary: %s", phrase)
		}
	}
}
