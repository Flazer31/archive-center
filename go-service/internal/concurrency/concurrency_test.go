package concurrency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_AllowThenBlock(t *testing.T) {
	rl := NewRateLimiter(2, 60)
	if !rl.IsAllowed() {
		t.Fatal("first request should be allowed")
	}
	if !rl.IsAllowed() {
		t.Fatal("second request should be allowed")
	}
	if rl.IsAllowed() {
		t.Fatal("third request should be blocked")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if !rl.IsAllowed() {
		t.Fatal("first request should be allowed")
	}
	if rl.IsAllowed() {
		t.Fatal("second request should be blocked in same window")
	}
	time.Sleep(1100 * time.Millisecond)
	if !rl.IsAllowed() {
		t.Fatal("request after window should be allowed")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, 60)
	if !rl.IsAllowed() {
		t.Fatal("first request should be allowed")
	}
	if rl.IsAllowed() {
		t.Fatal("second request should be blocked")
	}
	rl.Reset()
	if !rl.IsAllowed() {
		t.Fatal("request after reset should be allowed")
	}
}

func TestRateLimiter_State(t *testing.T) {
	rl := NewRateLimiter(3, 60)
	rl.IsAllowed()
	count, start := rl.State()
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}
	if start.IsZero() {
		t.Fatal("expected non-zero window start")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, 60)
	var wg sync.WaitGroup
	allowed := 0
	var mu sync.Mutex
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.IsAllowed() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 100 {
		t.Fatalf("expected exactly 100 allowed, got %d", allowed)
	}
}

func TestTokenCache_GetSetDelete(t *testing.T) {
	tc := NewTokenCache()
	if _, ok := tc.Get("k"); ok {
		t.Fatal("missing key should not be found")
	}
	tc.Set("k", &CachedToken{Token: "abc", ExpiresAt: time.Now().Add(time.Hour)})
	if tok, ok := tc.Get("k"); !ok || tok.Token != "abc" {
		t.Fatal("set token should be retrievable")
	}
	tc.Delete("k")
	if _, ok := tc.Get("k"); ok {
		t.Fatal("deleted key should not be found")
	}
}

func TestTokenCache_Expiry(t *testing.T) {
	tc := NewTokenCache()
	tc.Set("k", &CachedToken{Token: "expired", ExpiresAt: time.Now().Add(-time.Hour)})
	if _, ok := tc.Get("k"); ok {
		t.Fatal("expired token should not be returned")
	}
}

func TestTokenCache_GetOrFetch_Hit(t *testing.T) {
	tc := NewTokenCache()
	tc.Set("k", &CachedToken{Token: "cached", ExpiresAt: time.Now().Add(time.Hour)})
	called := false
	fetcher := func(ctx context.Context, key string) (*CachedToken, error) {
		called = true
		return nil, errors.New("should not be called")
	}
	tok, err := tc.GetOrFetch(context.Background(), "k", fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("fetcher should not be called on cache hit")
	}
	if tok.Token != "cached" {
		t.Fatalf("expected cached token, got %s", tok.Token)
	}
}

func TestTokenCache_GetOrFetch_Miss(t *testing.T) {
	tc := NewTokenCache()
	fetcher := func(ctx context.Context, key string) (*CachedToken, error) {
		return &CachedToken{Token: "new", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	tok, err := tc.GetOrFetch(context.Background(), "k", fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Token != "new" {
		t.Fatalf("expected new token, got %s", tok.Token)
	}
	// Second call should hit cache
	tok2, err := tc.GetOrFetch(context.Background(), "k", fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok2.Token != "new" {
		t.Fatalf("expected cached token on second call, got %s", tok2.Token)
	}
}

func TestTokenCache_GetOrFetch_Error(t *testing.T) {
	tc := NewTokenCache()
	fetcher := func(ctx context.Context, key string) (*CachedToken, error) {
		return nil, errors.New("fetch failed")
	}
	_, err := tc.GetOrFetch(context.Background(), "k", fetcher)
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
	if err.Error() != "fetch failed" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSessionCache_GetSetDelete(t *testing.T) {
	sc := NewSessionCache()
	if _, ok := sc.Get("s1"); ok {
		t.Fatal("missing session should not be found")
	}
	sc.Set("s1", "data", time.Hour)
	if d, ok := sc.Get("s1"); !ok || d != "data" {
		t.Fatal("set session should be retrievable")
	}
	sc.Delete("s1")
	if _, ok := sc.Get("s1"); ok {
		t.Fatal("deleted session should not be found")
	}
}

func TestSessionCache_TTLExpiry(t *testing.T) {
	sc := NewSessionCache()
	sc.Set("s1", "data", 50*time.Millisecond)
	if _, ok := sc.Get("s1"); !ok {
		t.Fatal("session should exist immediately after set")
	}
	time.Sleep(100 * time.Millisecond)
	if _, ok := sc.Get("s1"); ok {
		t.Fatal("session should be expired after TTL")
	}
}

func TestSessionCache_Len(t *testing.T) {
	sc := NewSessionCache()
	if sc.Len() != 0 {
		t.Fatalf("expected len 0, got %d", sc.Len())
	}
	sc.Set("s1", "a", time.Hour)
	sc.Set("s2", "b", time.Hour)
	if sc.Len() != 2 {
		t.Fatalf("expected len 2, got %d", sc.Len())
	}
	sc.Delete("s1")
	if sc.Len() != 1 {
		t.Fatalf("expected len 1, got %d", sc.Len())
	}
}

func TestSharedState_OperatorToken(t *testing.T) {
	ss := NewSharedState()
	if ss.GetOperatorToken() != "" {
		t.Fatal("default token should be empty")
	}
	ss.SetOperatorToken("tok123")
	if ss.GetOperatorToken() != "tok123" {
		t.Fatal("token round-trip failed")
	}
}

func TestSharedState_MariaDBReady(t *testing.T) {
	ss := NewSharedState()
	if ss.IsMariaDBReady() {
		t.Fatal("default MariaDB ready should be false")
	}
	ss.SetMariaDBReady(true)
	if !ss.IsMariaDBReady() {
		t.Fatal("MariaDB ready round-trip failed")
	}
}

func TestSharedState_MilvusReady(t *testing.T) {
	ss := NewSharedState()
	if ss.IsMilvusReady() {
		t.Fatal("default Milvus ready should be false")
	}
	ss.SetMilvusReady(true)
	if !ss.IsMilvusReady() {
		t.Fatal("Milvus ready round-trip failed")
	}
}

func TestSharedState_EmbeddingConfig(t *testing.T) {
	ss := NewSharedState()
	model, endpoint := ss.GetEmbeddingConfig()
	if model != "" || endpoint != "" {
		t.Fatal("default embedding config should be empty")
	}
	ss.SetEmbeddingConfig("model-a", "http://embed")
	m, e := ss.GetEmbeddingConfig()
	if m != "model-a" || e != "http://embed" {
		t.Fatal("embedding config round-trip failed")
	}
}
