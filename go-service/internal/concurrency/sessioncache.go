package concurrency

import (
	"sync"
	"time"
)

// SessionValue is a generic session-scoped value with TTL.
type SessionValue struct {
	Data      any
	CreatedAt time.Time
	TTL       time.Duration
}

// SessionCache implements a session-scoped cache similar to 0.8's
// in-memory session state (e.g., guidance_plan_states caching).
type SessionCache struct {
	mu    sync.RWMutex
	items map[string]*SessionValue
}

// NewSessionCache creates a new session cache.
func NewSessionCache() *SessionCache {
	return &SessionCache{items: make(map[string]*SessionValue)}
}

// Get returns the value for sessionID if present and not expired.
func (sc *SessionCache) Get(sessionID string) (any, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	v, ok := sc.items[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(v.CreatedAt) > v.TTL {
		return nil, false
	}
	return v.Data, true
}

// Set stores a value for sessionID.
func (sc *SessionCache) Set(sessionID string, data any, ttl time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.items[sessionID] = &SessionValue{
		Data:      data,
		CreatedAt: time.Now(),
		TTL:       ttl,
	}
}

// Delete removes a session entry.
func (sc *SessionCache) Delete(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.items, sessionID)
}

// Len returns the number of items in the cache (including expired ones).
func (sc *SessionCache) Len() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.items)
}
