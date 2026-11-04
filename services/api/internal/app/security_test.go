package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	return server, httpServer
}

func TestRecoveryCodeLoginIsRateLimited(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	server, httpServer := newTestServer(t)
	server.runAt(&now)
	_ = httpServer
	if _, err := server.store.CreateUser("ayse", "JBSWY3DPEHPK3PXP", now); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{}
	attempt := func() int {
		status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/login", map[string]string{
			"username": "ayse",
			"code":     "AAAA-AAAA",
		})
		return status
	}
	// Burst of 6 wrong recovery codes, then the bucket is empty.
	for i := 0; i < 6; i++ {
		if status := attempt(); status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, status)
		}
	}
	if status := attempt(); status != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit after burst, got %d", status)
	}
	// The extra request above drained the refilled token, so six minutes
	// later the bucket is still one token short of full.
	now = now.Add(10 * time.Minute)
	if status := attempt(); status != http.StatusUnauthorized {
		t.Fatalf("expected refill after waiting, got %d", status)
	}
}

func TestSignupConfirmIsAttemptLimited(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	client := &http.Client{}

	status, start := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/start", map[string]string{"username": "gece_kirpi"})
	if status != http.StatusOK {
		t.Fatalf("signup start: %d %v", status, start)
	}
	signupID := start["signup_id"].(string)
	for i := 0; i < maxSignupAttempts; i++ {
		status, _ := requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/confirm", map[string]string{
			"signup_id": signupID,
			"code":      "000000",
		})
		if i < maxSignupAttempts-1 && status != http.StatusBadRequest {
			t.Fatalf("attempt %d: got %d, want 400", i+1, status)
		}
	}
	// The pending signup is burned: even the real code cannot finish it.
	status, _ = requestJSON(t, client, http.MethodPost, httpServer.URL+"/api/auth/signup/confirm", map[string]string{
		"signup_id": signupID,
		"code":      "000000",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("burned signup still usable: %d", status)
	}
	if _, err := store.PendingSignup(signupID); err == nil {
		t.Fatal("pending signup should have been deleted")
	}
}

func TestAllowOriginBlocksUnknownOrigin(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	server := New(Config{Issuer: "test", WebDir: t.TempDir(), Origins: []string{"https://icinidok.com"}}, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)

	request, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/auth/login", nil)
	request.Header.Set("Origin", "https://kotu-site.example")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("unknown origin allowed: %d", response.StatusCode)
	}
}

// Behind the tunnel the public hostname is unknown, so a same-origin write from
// the request's own host is accepted when proxy trust is on; a different host
// is still refused, and trust-off keeps the strict check.
func TestSameOriginHostBehindTunnel(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	server := New(Config{Issuer: "test", WebDir: t.TempDir(), TrustProxy: true}, store)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	host := strings.TrimPrefix(httpServer.URL, "http://")

	// Same-origin write from the tunnel host reaches the handler (401 because
	// no session, not 403 because of the origin check).
	request, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/api/posts", nil)
	request.Header.Set("Origin", "http://"+host)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("same-origin write blocked: %d", response.StatusCode)
	}

	// A different origin is still refused.
	request, _ = http.NewRequest(http.MethodPost, httpServer.URL+"/api/posts", nil)
	request.Header.Set("Origin", "https://kotu-site.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin allowed: %d", response.StatusCode)
	}

	// With trust off the same-origin shortcut does not apply.
	strict := New(Config{Issuer: "test", WebDir: t.TempDir()}, store)
	strictServer := httptest.NewServer(strict.Handler())
	t.Cleanup(strictServer.Close)
	strictHost := strings.TrimPrefix(strictServer.URL, "http://")
	request, _ = http.NewRequest(http.MethodPost, strictServer.URL+"/api/posts", nil)
	request.Header.Set("Origin", "http://"+strictHost)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("trust-off origin check bypassed: %d", response.StatusCode)
	}
}
