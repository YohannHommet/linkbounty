package handlers

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsFirstThenBlocks(t *testing.T) {
	rl := NewRateLimiter(time.Minute)

	ok, _ := rl.Allow("1.2.3.4")
	if !ok {
		t.Fatal("first call should be allowed")
	}

	ok, retry := rl.Allow("1.2.3.4")
	if ok {
		t.Error("second call within window should be blocked")
	}
	if retry <= 0 || retry > time.Minute {
		t.Errorf("retryAfter = %v, want (0, 1m]", retry)
	}
}

func TestRateLimiterIsolatesKeys(t *testing.T) {
	rl := NewRateLimiter(time.Minute)

	if ok, _ := rl.Allow("a"); !ok {
		t.Error("first key should be allowed")
	}
	if ok, _ := rl.Allow("b"); !ok {
		t.Error("distinct key should not be affected by another key's limit")
	}
}

func TestRateLimiterReallowsAfterWindow(t *testing.T) {
	// Zero window means every call is outside the window → always allowed,
	// and exercises the eviction branch.
	rl := NewRateLimiter(0)

	if ok, _ := rl.Allow("x"); !ok {
		t.Error("first call should be allowed")
	}
	if ok, _ := rl.Allow("x"); !ok {
		t.Error("with a zero window the next call should be allowed again")
	}
}
