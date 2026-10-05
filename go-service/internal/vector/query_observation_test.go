package vector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChromaQueryResponseMeasurement(t *testing.T) {
	for _, status := range []int{200, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := `{"ids":[["one"]],"documents":[["기억"]],"metadatas":[[{"chat_session_id":"s","tier":"memory"}]],"distances":[[0.1]],"embeddings":[[[1,0]]]}`
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/query") {
					w.WriteHeader(status)
					w.Write([]byte(body))
					return
				}
				if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/collections/measurement") {
					w.Write([]byte(`{"id":"collection","name":"measurement"}`))
					return
				}
				http.Error(w, "unexpected request", 400)
			}))
			defer ts.Close()
			st, err := NewChromaStore(ts.URL, "measurement", "/api/v2")
			if err != nil {
				t.Fatal(err)
			}
			calls, total, observedStatus := 0, 0, 0
			ctx := WithQueryResponseObserver(context.Background(), func(size, status int) { calls++; total += size; observedStatus = status })
			rows, err := st.Search(ctx, "s", []float32{1, 0}, 1, "")
			if (status == 200 && (err != nil || len(rows) != 1)) || (status == 500 && err == nil) {
				t.Fatalf("search semantics changed: %v %v", rows, err)
			}
			if calls != 1 || total != len(body) || observedStatus != status {
				t.Fatalf("wrong query-only wire observation: %d %d %d", calls, total, observedStatus)
			}
		})
	}
}
