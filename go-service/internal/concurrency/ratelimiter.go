package concurrency

import (
	"sync"
	"time"
)

// RateLimiter implements a fixed-window rate limiter similar to the 0.8
// _FixedWindowRateLimiter in backend/services/route_guard.py.
// It is safe for concurrent use.
type RateLimiter struct {
	limit         int
	windowSeconds int
	count         int
	windowStart   time.Time
	mu            sync.Mutex
}

// NewRateLimiter creates a fixed-window rate limiter.
func NewRateLimiter(limit int, windowSeconds int) *RateLimiter {
	return &RateLimiter{
		limit:         limit,
		windowSeconds: windowSeconds,
		windowStart:   time.Now(),
	}
}

// IsAllowed returns true if the request is within the current window limit.
func (r *RateLimiter) IsAllowed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.windowStart) >= time.Duration(r.windowSeconds)*time.Second {
		r.windowStart = now
		r.count = 1
		return true
	}
	r.count++
	return r.count <= r.limit
}

// Reset resets the limiter state.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count = 0
	r.windowStart = time.Time{}
}

// State returns a snapshot of the current window for diagnostics.
func (r *RateLimiter) State() (count int, windowStart time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count, r.windowStart
}
