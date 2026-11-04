package app

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// The paylaş form's client-side rules must match the server exactly: title
// 10-120 chars and >=2 words, body 80-4000 chars and >=12 words. If they
// drift, a form that looks valid still fails on submit.
func TestCreatePostWordRules(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	client, _ := signup(t, httpServer.URL, "yazar")

	validBody := strings.Repeat("kelime ", 12) // 12 words, well over 80 chars
	cases := []struct {
		name   string
		title  string
		body   string
		status int
	}{
		{"valid", "iki kelimelik başlık", validBody, http.StatusCreated},
		{"single word title", "tekbaskelime", validBody, http.StatusBadRequest},
		{"short title", "kısa", validBody, http.StatusBadRequest},
		{"long char short words", "iki kelimelik başlık", strings.Repeat("a", 90), http.StatusBadRequest},
		{"toofew words", "iki kelimelik başlık", strings.Repeat("kelime ", 5) + strings.Repeat("x", 50), http.StatusBadRequest},
		{"short body", "iki kelimelik başlık", strings.Repeat("kelime ", 12)[:40], http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
				"category": "Genel",
				"title":    tc.title,
				"body":     tc.body,
			})
			if status != tc.status {
				t.Fatalf("got %d want %d (%v)", status, tc.status, body)
			}
			// Two throttles apply: 10 minutes between posts and 4 per rolling
			// 24h. Step past both so the next valid attempt is not 429'd.
			now = now.Add(6*time.Hour + time.Minute)
		})
	}
}
