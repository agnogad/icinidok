package app

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	limiter := NewLimiter(1, 2) // one token per minute, burst of two
	now := time.Unix(1_800_000_000, 0)
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("a") || !limiter.Allow("a") {
		t.Fatal("burst should allow two requests")
	}
	if limiter.Allow("a") {
		t.Fatal("third request should be limited")
	}
	if !limiter.Allow("b") {
		t.Fatal("different key must have its own bucket")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow("a") {
		t.Fatal("token should refill after a minute")
	}
}
