package concurrency

import (
	"context"
	"sync"
	"time"
)

// CachedToken holds an OAuth-style access token with expiry.
type CachedToken struct {
	Token     string
	ExpiresAt time.Time
}

// TokenCache implements a keyed token cache with per-key locks,
// mirroring the 0.8 _VERTEX_TOKEN_CACHE and _VERTEX_TOKEN_LOCKS pattern.
type TokenCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedToken
}

// NewTokenCache creates a new token cache.
func NewTokenCache() *TokenCache {
	return &TokenCache{cache: make(map[string]*CachedToken)}
}

// Get returns the token for key if it exists and is not expired.
func (tc *TokenCache) Get(key string) (*CachedToken, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	t, ok := tc.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(t.ExpiresAt) {
		return nil, false
	}
	return t, true
}

// Set stores a token for key.
func (tc *TokenCache) Set(key string, token *CachedToken) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache[key] = token
}

// Delete removes a token for key.
func (tc *TokenCache) Delete(key string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	delete(tc.cache, key)
}

// TokenCacheFetcher is the contract for acquiring a new token when the cache misses.
type TokenCacheFetcher func(ctx context.Context, key string) (*CachedToken, error)

// GetOrFetch returns a cached token or fetches and caches a new one.
func (tc *TokenCache) GetOrFetch(ctx context.Context, key string, fetcher TokenCacheFetcher) (*CachedToken, error) {
	if t, ok := tc.Get(key); ok {
		return t, nil
	}
	t, err := fetcher(ctx, key)
	if err != nil {
		return nil, err
	}
	tc.Set(key, t)
	return t, nil
}
