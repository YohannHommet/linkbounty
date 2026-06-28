package handlers

import (
	"sync"
	"time"
)

// RateLimiter enforces a minimum interval between actions keyed by an
// arbitrary string (here, client IP). It evicts expired entries on every
// Allow call to bound memory growth.
type RateLimiter struct {
	mu     sync.Mutex
	last   map[string]time.Time
	window time.Duration
}

func NewRateLimiter(window time.Duration) *RateLimiter {
	return &RateLimiter{
		last:   make(map[string]time.Time),
		window: window,
	}
}

// Allow reports whether key may act now. When denied, it returns the time
// remaining until the next action is permitted. A successful Allow records
// the action time, so callers should only call it once per attempt.
func (rl *RateLimiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for k, t := range rl.last {
		if now.Sub(t) >= rl.window {
			delete(rl.last, k)
		}
	}

	if t, seen := rl.last[key]; seen {
		if elapsed := now.Sub(t); elapsed < rl.window {
			return false, rl.window - elapsed
		}
	}

	rl.last[key] = now
	return true, 0
}
