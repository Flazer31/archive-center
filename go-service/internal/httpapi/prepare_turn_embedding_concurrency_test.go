package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
)

// The transport is controlled; query selection, provider serialization, timeout,
// vector provenance and assembly run through their production owners. No AI call.
func TestPrepareTurnHistoryEmbeddingConcurrency(t *testing.T) {
	for _, mode := range []string{"success", "history_failure", "primary_failure", "primary_empty", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.Default()
			cfg.ChromaEndpoint = "http://controlled-vector.invalid"
			srv := NewServer(cfg)
			srv.Vector = &restorationRepeatedHitVector{}
			srv.VectorOpenError = nil
			raw := "Recall the silver shield."
			req := dto.PrepareTurnRequest{ChatSessionID: "parallel-recall", RawUserInput: &raw,
				ClientMeta: map[string]any{"embedding": map[string]any{"provider": "custom", "endpoint": "https://controlled.invalid/v1", "api_key": "synthetic", "model": "fixture", "timeout_ms": 30000}}}
			for i := 0; i < 5; i++ {
				req.Messages = append(req.Messages, map[string]any{"role": "user", "content": fmt.Sprintf("Visit %d", i)}, map[string]any{"role": "assistant", "content": fmt.Sprintf("Room %d was inspected.", i)})
			}
			queries := prepareTurnRetrievalQueries(req, prepareTurnRecentConversationReferenceLimit(req.Settings))
			if len(queries) != 6 {
				t.Fatalf("default query coverage changed: %d", len(queries))
			}
			indices := map[string]int{}
			for i, q := range queries {
				indices[q.Text] = i
			}
			var calls, active, peak atomic.Int32
			var mu sync.Mutex
			seen := map[int]int{}
			started := make(chan struct{}, 6)
			release := make(chan struct{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					return nil, err
				}
				index, ok := indices[extractionStringFromAny(body["input"])]
				if !ok {
					return nil, fmt.Errorf("unexpected embedding input: %v", body["input"])
				}
				calls.Add(1)
				a := active.Add(1)
				defer active.Add(-1)
				for p := peak.Load(); a > p && !peak.CompareAndSwap(p, a); p = peak.Load() {
				}
				mu.Lock()
				seen[index]++
				mu.Unlock()
				if index == 0 {
					if mode == "primary_failure" {
						return nil, fmt.Errorf("primary unavailable")
					}
				} else {
					started <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
					// Return out of query order; provenance must retain original order.
					select {
					case <-time.After(time.Duration(6-index) * time.Millisecond):
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
					if mode == "history_failure" && index == 2 {
						return nil, fmt.Errorf("history unavailable")
					}
				}
				vectorJSON := fmt.Sprintf("[%d,0.2,0.3]", index+1)
				if mode == "primary_empty" && index == 0 {
					vectorJSON = "[]"
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"embedding":` + vectorJSON + `}]}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()
			done := make(chan map[string]any, 1)
			go func() { done <- srv.prepareTurnVectorShadow(ctx, req, 5) }()
			// Release on failure too, so no test goroutine can outlive the client.
			if mode != "primary_failure" && mode != "primary_empty" {
				for i := 0; i < 3; i++ {
					select {
					case <-started:
					case <-time.After(2 * time.Second):
						cancel()
						close(release)
						<-done
						t.Fatal("history embedding remained serial")
					}
				}
			}
			if mode == "cancel" {
				cancel()
			}
			close(release)
			var trace map[string]any
			select {
			case trace = <-done:
			case <-time.After(3 * time.Second):
				cancel()
				trace = <-done
				t.Fatal("embedding workers did not stop")
			}
			if mode == "primary_failure" || mode == "primary_empty" {
				if calls.Load() != 1 || intFromAny(trace["query_embedding_count"], 0) != 0 {
					t.Fatalf("primary failure spent history calls: %v", trace)
				}
				return
			}
			if calls.Load() != 6 || peak.Load() != 3 || active.Load() != 0 {
				t.Fatalf("calls=%d peak=%d active=%d", calls.Load(), peak.Load(), active.Load())
			}
			for i := range queries {
				if seen[i] != 1 {
					t.Fatalf("query %d calls=%d", i, seen[i])
				}
			}
			if mode == "cancel" {
				return
			}
			hits := prepareTurnVectorMemorySearchResultMaps(trace)
			if len(hits) != 1 {
				t.Fatalf("canonical hit count changed: %v", hits)
			}
			var got, want []string
			for _, observation := range sliceFromAny(hits[0]["recall_queries"]) {
				got = append(got, extractionStringFromAny(mapFromAny(observation)["query"]))
			}
			for i, q := range queries {
				if !(mode == "history_failure" && i == 2) {
					want = append(want, q.Text)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("query provenance shifted: got=%q want=%q", got, want)
			}
			if intFromAny(trace["query_embedding_count"], 0) != len(want) {
				t.Fatalf("lost embedding: %v", trace)
			}
			t.Logf("%s calls=%d peak=%d successful_vectors=%d ordered_provenance=true", mode, calls.Load(), peak.Load(), len(want))
		})
	}
}
