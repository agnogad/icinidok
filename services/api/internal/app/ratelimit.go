package app

import (
	"sync"
	"time"
)

const refillUnit = time.Millisecond

// Limiter is a small in-memory token bucket. Single-process state is enough for
// this deployment; a restart resets the buckets.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	perTick float64 // tokens produced per refillUnit
	burst   float64
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewLimiter(perMinute, burst float64) *Limiter {
	if perMinute <= 0 {
		perMinute = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		perTick: perMinute / float64(time.Minute/refillUnit),
		burst:   burst,
		now:     time.Now,
	}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	current, ok := l.buckets[key]
	if !ok {
		current = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = current
	}
	if elapsed := now.Sub(current.last); elapsed > 0 {
		current.tokens += float64(elapsed/refillUnit) * l.perTick
		if current.tokens > l.burst {
			current.tokens = l.burst
		}
		// Keep the sub-unit remainder: advance `last` only by the time
		// actually credited, instead of resetting the refill clock.
		current.last = current.last.Add(elapsed - elapsed%refillUnit)
		if current.tokens >= l.burst {
			current.last = now
		}
	}
	if current.tokens < 1 {
		return false
	}
	current.tokens--
	return true
}

func (l *Limiter) Sweep(olderThan time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-olderThan)
	for key, value := range l.buckets {
		if value.last.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
