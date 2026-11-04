package app

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"
)

// signup creates a user through the public endpoints and returns the client's
// session plus its TOTP secret.
func signup(t *testing.T, httpServer string, username string) (*http.Client, string) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	status, start := requestJSON(t, client, http.MethodPost, httpServer+"/api/auth/signup/start", map[string]string{"username": username})
	if status != http.StatusOK {
		t.Fatalf("signup start %s: %d %v", username, status, start)
	}
	secret := start["secret"].(string)
	signupID := start["signup_id"].(string)
	now := time.Unix(1_800_000_000, 0)
	status, confirmed := requestJSON(t, client, http.MethodPost, httpServer+"/api/auth/signup/confirm", map[string]string{
		"signup_id": signupID,
		"code":      codeAt(t, secret, now),
	})
	if status != http.StatusCreated {
		t.Fatalf("signup confirm %s: %d %v", username, status, confirmed)
	}
	return client, secret
}

func TestSupportSaveAndViewerState(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	author, _ := signup(t, httpServer.URL, "yazar")
	reader, _ := signup(t, httpServer.URL, "okuyucu")

	status, created := requestJSON(t, author, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
		"category": "Genel",
		"title":    "Üzerine konuşmak istediğim bir konu",
		"body":     "Buraya seksen karakterden uzun bir metin yazıyorum ki doğrulama geçsin ve paylaşım oluşsun rahatça.",
	})
	if status != http.StatusCreated {
		t.Fatalf("create post: %d %v", status, created)
	}
	postID := created["post"].(map[string]any)["id"].(string)

	// Support toggles on and off.
	status, toggled := requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/support", nil)
	if status != http.StatusOK || toggled["supported"] != true || toggled["supports"].(float64) != 1 {
		t.Fatalf("support on: %d %v", status, toggled)
	}
	status, toggled = requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/support", nil)
	if status != http.StatusOK || toggled["supported"] != false || toggled["supports"].(float64) != 0 {
		t.Fatalf("support off: %d %v", status, toggled)
	}
	requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/support", nil)

	// Save toggles on and off.
	status, saved := requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/save", nil)
	if status != http.StatusOK || saved["saved"] != true {
		t.Fatalf("save on: %d %v", status, saved)
	}
	status, saved = requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/save", nil)
	if status != http.StatusOK || saved["saved"] != false {
		t.Fatalf("save off: %d %v", status, saved)
	}
	requestJSON(t, reader, http.MethodPost, httpServer.URL+"/api/posts/"+postID+"/save", nil)

	// Viewer sees their own state, the author does not.
	status, detail := requestJSON(t, reader, http.MethodGet, httpServer.URL+"/api/posts/"+postID, nil)
	if status != http.StatusOK || detail["supported"] != true || detail["saved"] != true || detail["mine"] != false {
		t.Fatalf("reader detail: %d %v", status, detail)
	}
	status, detail = requestJSON(t, author, http.MethodGet, httpServer.URL+"/api/posts/"+postID, nil)
	if status != http.StatusOK || detail["mine"] != true {
		t.Fatalf("author detail: %d %v", status, detail)
	}

	// Saved list and stats reflect the state.
	status, list := requestJSON(t, reader, http.MethodGet, httpServer.URL+"/api/me/saved", nil)
	if status != http.StatusOK || len(list["posts"].([]any)) != 1 {
		t.Fatalf("saved list: %d %v", status, list)
	}
	status, stats := requestJSON(t, author, http.MethodGet, httpServer.URL+"/api/me/stats", nil)
	if status != http.StatusOK {
		t.Fatalf("stats: %d", status)
	}
	numbers := stats["stats"].(map[string]any)
	if numbers["posts"].(float64) != 1 || numbers["supports_on_posts"].(float64) != 1 {
		t.Fatalf("unexpected stats: %v", numbers)
	}
}

func TestConversationFlow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	owner, _ := signup(t, httpServer.URL, "sahip")
	writer, _ := signup(t, httpServer.URL, "yazan")

	status, created := requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
		"category": "Yalnızlık",
		"title":    "Kimseye anlatamadığım bir şey var",
		"body":     "Buraya seksen karakterden uzun bir metin yazıyorum ki doğrulama geçsin ve paylaşım oluşsun rahatça.",
	})
	if status != http.StatusCreated {
		t.Fatalf("create post: %d %v", status, created)
	}
	postID := created["post"].(map[string]any)["id"].(string)

	// Owner cannot open a chat on their own post.
	status, _ = requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "kendime yazıyorum",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("self chat: %d", status)
	}

	// Writer opens the conversation with an opening message.
	status, started := requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "Seni dinliyorum, anlatmak ister misin?",
	})
	if status != http.StatusCreated {
		t.Fatalf("start conversation: %d %v", status, started)
	}
	now = now.Add(11 * time.Second)
	conv := started["conversation"].(map[string]any)
	convID := conv["id"].(string)
	if conv["role"] != "writer" || conv["peer_name"] != "sahip" {
		t.Fatalf("writer view: %v", conv)
	}

	// Starting again reuses the same conversation.
	status, again := requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "bir şey daha ekliyorum",
	})
	if status != http.StatusCreated || again["conversation"].(map[string]any)["id"] != convID {
		t.Fatalf("second start created new conversation: %v", again)
	}

	// The owner sees it with an unread badge and the writer's name.
	status, list := requestJSON(t, owner, http.MethodGet, httpServer.URL+"/api/conversations", nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d", status)
	}
	items := list["conversations"].([]any)
	if len(items) != 1 {
		t.Fatalf("owner conversations: %v", items)
	}
	ownerView := items[0].(map[string]any)
	if ownerView["role"] != "owner" || ownerView["peer_name"] != "yazan" || ownerView["unread"].(float64) != 2 {
		t.Fatalf("owner view: %v", ownerView)
	}

	// The owner replies; both sides keep their own role.
	status, sent := requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/messages", map[string]string{
		"body": "Teşekkür ederim, gerçekten iyi geldi.",
	})
	if status != http.StatusCreated {
		t.Fatalf("reply: %d %v", status, sent)
	}
	now = now.Add(11 * time.Second)

	// The writer polls incrementally and only gets the new message.
	status, page := requestJSON(t, writer, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	if status != http.StatusOK {
		t.Fatalf("history: %d", status)
	}
	history := page["messages"].([]any)
	if len(history) != 3 {
		t.Fatalf("history length: %d", len(history))
	}
	lastID := history[len(history)-1].(map[string]any)["id"].(string)
	requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/messages", map[string]string{"body": "Bir şey daha yazıyorum."})
	now = now.Add(11 * time.Second)
	status, delta := requestJSON(t, writer, http.MethodGet, httpServer.URL+"/api/conversations/"+convID+"?after="+lastID, nil)
	if status != http.StatusOK || len(delta["messages"].([]any)) != 1 {
		t.Fatalf("delta fetch: %d %v", status, delta)
	}

	// Opening the conversation clears the owner's badge.
	requestJSON(t, owner, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	status, summary := requestJSON(t, owner, http.MethodGet, httpServer.URL+"/api/summary", nil)
	if status != http.StatusOK || summary["unread_messages"].(float64) != 0 {
		t.Fatalf("summary after read: %d %v", status, summary)
	}

	// The writer got notifications, the owner got a support-free chat alert.
	status, notifications := requestJSON(t, owner, http.MethodGet, httpServer.URL+"/api/notifications", nil)
	if status != http.StatusOK || notifications["unread"].(float64) == 0 {
		t.Fatalf("owner notifications: %d %v", status, notifications)
	}
	requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/notifications/read", nil)
	status, notifications = requestJSON(t, owner, http.MethodGet, httpServer.URL+"/api/notifications", nil)
	if status != http.StatusOK || notifications["unread"].(float64) != 0 {
		t.Fatalf("notifications not cleared: %v", notifications)
	}

	// A stranger cannot read the conversation.
	stranger, _ := signup(t, httpServer.URL, "yabanci")
	status, _ = requestJSON(t, stranger, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	if status != http.StatusForbidden {
		t.Fatalf("stranger read conversation: %d", status)
	}

	// Closing marks it closed for both sides.
	status, closed := requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/close", nil)
	if status != http.StatusOK || closed["conversation"].(map[string]any)["status"] != "closed" {
		t.Fatalf("close: %d %v", status, closed)
	}

	// A closed conversation is locked: neither side can write into it.
	now = now.Add(11 * time.Second)
	status, _ = requestJSON(t, owner, http.MethodPost, httpServer.URL+"/api/conversations/"+convID+"/messages", map[string]string{"body": "Yeniden yazıyorum."})
	if status != http.StatusForbidden {
		t.Fatalf("write to closed conversation: %d", status)
	}
	status, _ = requestJSON(t, writer, http.MethodPost, httpServer.URL+"/api/conversations", map[string]string{
		"post_id": postID, "message": "tekrar deniyorum",
	})
	if status != http.StatusForbidden {
		t.Fatalf("restart closed conversation: %d", status)
	}
	status, page = requestJSON(t, writer, http.MethodGet, httpServer.URL+"/api/conversations/"+convID, nil)
	if status != http.StatusOK || page["conversation"].(map[string]any)["status"] != "closed" {
		t.Fatalf("closed state changed: %d %v", status, page)
	}
}

func TestRecoveryCodeRotation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)

	user, secret := signup(t, httpServer.URL, "kullanici")

	// Wrong code is rejected and no new codes are issued.
	status, _ := requestJSON(t, user, http.MethodPost, httpServer.URL+"/api/me/recovery-codes", map[string]string{"code": "000000"})
	if status != http.StatusUnauthorized {
		t.Fatalf("bad rotation code: %d", status)
	}

	// A valid TOTP code rotates the set.
	status, rotated := requestJSON(t, user, http.MethodPost, httpServer.URL+"/api/me/recovery-codes", map[string]string{
		"code": codeAt(t, secret, now),
	})
	if status != http.StatusOK || len(rotated["recovery_codes"].([]any)) != maxRecoveryCodes {
		t.Fatalf("rotation: %d %v", status, rotated)
	}
	status, stats := requestJSON(t, user, http.MethodGet, httpServer.URL+"/api/me/stats", nil)
	if status != http.StatusOK || stats["recovery_codes_left"].(float64) != maxRecoveryCodes {
		t.Fatalf("codes left: %v", stats)
	}

	// A code from the new set works; one from the old set does not.
	newCode := rotated["recovery_codes"].([]any)[0].(string)
	requestJSON(t, user, http.MethodPost, httpServer.URL+"/api/auth/logout", nil)
	status, _ = requestJSON(t, user, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
		"username": "kullanici", "code": newCode,
	})
	if status != http.StatusOK {
		t.Fatalf("new recovery code rejected: %d", status)
	}
}
