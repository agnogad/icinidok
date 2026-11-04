package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/hotp"
)

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	step := at.Unix() / totpPeriod
	code, err := hotp.GenerateCodeCustom(secret, uint64(step), hotp.ValidateOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func requestJSON(t *testing.T, client *http.Client, method, url string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	var decoded map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%s %s: invalid JSON %q: %v", method, url, raw, err)
		}
	}
	return response.StatusCode, decoded
}

func TestAuthAndPostsFlow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	server.runAt(&now)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Signup start returns a QR and a secret.
	status, start := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/start", map[string]string{"username": "Gece_Kirpi"})
	if status != http.StatusOK {
		t.Fatalf("signup start: %d %v", status, start)
	}
	secret, _ := start["secret"].(string)
	signupID, _ := start["signup_id"].(string)
	if secret == "" || signupID == "" {
		t.Fatalf("missing signup fields: %v", start)
	}
	if qr, _ := start["qr"].(string); !strings.HasPrefix(qr, "data:image/png;base64,") {
		t.Fatal("missing QR image")
	}

	// Confirm with the current TOTP code and receive recovery codes.
	status, confirmed := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/confirm", map[string]string{
		"signup_id": signupID,
		"code":      codeAt(t, secret, now),
	})
	if status != http.StatusCreated {
		t.Fatalf("signup confirm: %d %v", status, confirmed)
	}
	recovery, _ := confirmed["recovery_codes"].([]any)
	if len(recovery) != maxRecoveryCodes {
		t.Fatalf("recovery codes: %v", confirmed)
	}
	user, _ := confirmed["user"].(map[string]any)
	if user["username"] != "gece_kirpi" {
		t.Fatalf("username not normalized: %v", user)
	}

	// Session works.
	status, me := requestJSON(t, client, http.MethodGet, httpServer.URL+"/api/auth/me", nil)
	if status != http.StatusOK || me["user"] == nil {
		t.Fatalf("me: %d %v", status, me)
	}

	// Create a post.
	body := strings.Repeat("kelime ", 20)
	status, created := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/posts", map[string]string{
		"category": "Genel",
		"title":    "Kendimi yalnız hissediyorum",
		"body":     body,
	})
	if status != http.StatusCreated {
		t.Fatalf("create post: %d %v", status, created)
	}
	post, _ := created["post"].(map[string]any)
	postID, _ := post["id"].(string)
	if postID == "" || post["author"] != "gece_kirpi" {
		t.Fatalf("bad post: %v", post)
	}

	status, list := requestJSON(t, client, http.MethodGet, httpServer.URL+"/api/posts", nil)
	if status != http.StatusOK || len(list["posts"].([]any)) == 0 {
		t.Fatalf("list posts: %d %v", status, list)
	}
	status, detail := requestJSON(t, client, http.MethodGet, httpServer.URL+"/api/posts/"+postID, nil)
	if status != http.StatusOK || detail["post"].(map[string]any)["body"] != strings.TrimSpace(body) {
		t.Fatalf("detail: %d %v", status, detail)
	}

	// Logout invalidates the session.
	if status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/logout", nil); status != http.StatusOK {
		t.Fatalf("logout: %d", status)
	}
	if status, _ := requestJSON(t, client, http.MethodGet, httpServer.URL+"/api/auth/me", nil); status != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", status)
	}

	// Replaying the signup code must fail.
	status, _ = requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
		"username": "gece_kirpi",
		"code":     codeAt(t, secret, now),
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("replayed TOTP code accepted: %d", status)
	}

	// The next time step works.
	now = now.Add(30 * time.Second)
	status, _ = requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
		"username": "gece_kirpi",
		"code":     codeAt(t, secret, now),
	})
	if status != http.StatusOK {
		t.Fatalf("login: %d", status)
	}

	// A recovery code works once.
	if status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/logout", nil); status != http.StatusOK {
		t.Fatal("logout before recovery")
	}
	recoveryCode := recovery[0].(string)
	status, _ = requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
		"username": "gece_kirpi",
		"code":     recoveryCode,
	})
	if status != http.StatusOK {
		t.Fatalf("recovery login: %d", status)
	}
	requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/logout", nil)
	status, _ = requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
		"username": "gece_kirpi",
		"code":     recoveryCode,
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("reused recovery code accepted: %d", status)
	}
}

func TestSignupRejectsDuplicateUsername(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	client := &http.Client{}

	if _, err := store.CreateUser("ayse", "JBSWY3DPEHPK3PXP", time.Now()); err != nil {
		t.Fatal(err)
	}
	status, response := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/start", map[string]string{"username": "ayse"})
	if status != http.StatusConflict {
		t.Fatalf("duplicate username: %d %v", status, response)
	}
}
