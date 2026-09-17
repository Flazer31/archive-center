package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestAuthMiddlewareDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enforce = false
	srv := NewServer(cfg)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := srv.authMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !called {
		t.Error("next handler was not called when auth is disabled")
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enforce = true
	cfg.Auth.BearerToken = "valid-token"
	srv := NewServer(cfg)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called without token")
	})

	mw := srv.authMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enforce = true
	cfg.Auth.BearerToken = "valid-token"
	srv := NewServer(cfg)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called with invalid token")
	})

	mw := srv.authMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enforce = true
	cfg.Auth.BearerToken = "valid-token"
	srv := NewServer(cfg)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := srv.authMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !called {
		t.Error("next handler was not called with valid token")
	}
}
