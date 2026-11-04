package app

import (
	"net/http"
	"testing"
	"time"
)

// Messages are throttled to one every 10 seconds per sender.
func TestMessageThrottle(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	owner, _ := signup(t, httpServer.URL, "sahip2")
	writer, _ := signup(t, httpServer.URL, "yazan2")

	status, created := requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
		"category": "Genel",
		"title":    "Mesaj hızı testi için başlık",
		"body":     "Buraya seksen karakterden uzun bir metin yazıyorum ki doğrulama geçsin ve paylaşım oluşsun rahatça.",
	})
	if status != http.StatusCreated {
		t.Fatalf("post: %d %v", status, created)
	}
	postID := created["post"].(map[string]any)["id"].(string)

	status, started := requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "ilk mesaj",
	})
	if status != http.StatusCreated {
		t.Fatalf("start: %d %v", status, started)
	}
	convID := started["conversation"].(map[string]any)["id"].(string)

	// A second message 5s later is rejected.
	now = now.Add(5 * time.Second)
	status, _ = requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/messages", map[string]string{"body": "beş saniye sonra"})
	if status != http.StatusTooManyRequests {
		t.Fatalf("fast second message: got %d want 429", status)
	}

	// After the full 10s window it goes through.
	now = now.Add(6 * time.Second)
	status, _ = requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/messages", map[string]string{"body": "onbir saniye sonra"})
	if status != http.StatusCreated {
		t.Fatalf("message after window: got %d want 201", status)
	}
}

// Messages older than 24h disappear from history and previews.
func TestMessagesExpireAfter24h(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	owner, _ := signup(t, httpServer.URL, "sahip3")
	writer, _ := signup(t, httpServer.URL, "yazan3")

	status, created := requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
		"category": "Genel",
		"title":    "Mesaj ömrü testi için başlık",
		"body":     "Buraya seksen karakterden uzun bir metin yazıyorum ki doğrulama geçsin ve paylaşım oluşsun rahatça.",
	})
	if status != http.StatusCreated {
		t.Fatalf("post: %d %v", status, created)
	}
	postID := created["post"].(map[string]any)["id"].(string)

	status, started := requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "yirmi dort saatten sonra kaybolmalı",
	})
	if status != http.StatusCreated {
		t.Fatalf("start: %d %v", status, started)
	}
	convID := started["conversation"].(map[string]any)["id"].(string)

	// Still visible one hour later.
	now = now.Add(time.Hour)
	status, page := requestJSON(t, writer, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	if status != http.StatusOK || len(page["messages"].([]any)) != 1 {
		t.Fatalf("within window: %d %v", status, page)
	}

	// Gone 25 hours after it was sent.
	now = now.Add(25 * time.Hour)
	status, page = requestJSON(t, writer, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	if status != http.StatusOK || len(page["messages"].([]any)) != 0 {
		t.Fatalf("past window: %d %v", status, page)
	}

	// The stored row is purged by cleanup.
	if err := server.store.Cleanup(now); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var count int
	if err := server.store.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, convID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("messages not purged: %d", count)
	}
}
