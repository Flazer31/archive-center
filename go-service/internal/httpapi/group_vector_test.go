package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMilvusShadowRoutesRemovedFromActiveRuntime(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	for _, path := range []string{
		"/milvus-shadow/backfill-compare",
		"/milvus-shadow/bounded-live-read-drill",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"chat_session_id":"sess-123"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}
