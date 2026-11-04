package app

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// One account may post at most 4 times per rolling 24h, with at least 10
// minutes between posts.
func TestPostDayLimit(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	client, _ := signup(t, httpServer.URL, "yazan")

	post := func() int {
		status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
			"category": "Genel",
			"title":    "iki kelimelik başlık",
			"body":     strings.Repeat("kelime ", 12),
		})
		return status
	}

	// Four posts accepted, 10+ minutes apart.
	for i := 0; i < 4; i++ {
		if status := post(); status != http.StatusCreated {
			t.Fatalf("post %d: got %d want 201", i+1, status)
		}
		now = now.Add(11 * time.Minute)
	}

	// Fifth within the same day is rejected even though 10 minutes passed.
	if status := post(); status != http.StatusTooManyRequests {
		t.Fatalf("5th post: got %d want 429", status)
	}

	// A day later the bucket has refilled enough for another post.
	now = now.Add(24 * time.Hour)
	if status := post(); status != http.StatusCreated {
		t.Fatalf("post after a day: got %d want 201", status)
	}
}

// Posts cannot be sent back to back: the second one within 10 minutes is
// rejected even when the daily quota is untouched.
func TestPostTenMinuteGap(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	client, _ := signup(t, httpServer.URL, "aceleci")

	send := func() int {
		status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
			"category": "Genel",
			"title":    "iki kelimelik başlık",
			"body":     strings.Repeat("kelime ", 12),
		})
		return status
	}

	if status := send(); status != http.StatusCreated {
		t.Fatalf("first: got %d want 201", status)
	}
	now = now.Add(5 * time.Minute)
	if status := send(); status != http.StatusTooManyRequests {
		t.Fatalf("early second: got %d want 429", status)
	}
	now = now.Add(6 * time.Minute) // 11 minutes after the first
	if status := send(); status != http.StatusCreated {
		t.Fatalf("second after gap: got %d want 201", status)
	}
}
