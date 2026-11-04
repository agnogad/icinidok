package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/skip2/go-qrcode"
)

const (
	sessionCookie      = "icini_session"
	sessionTTL         = 30 * 24 * time.Hour
	signupTTL          = 15 * time.Minute
	conversationTTL    = 24 * time.Hour
	maxSignupAttempts  = 5
	maxRecoveryCodes   = 8
	maxRequestBody     = 64 << 10
	maxPostTitleRunes  = 120
	maxPostBodyRunes   = 4000
	maxPostExcerptRune = 240
	maxMessageRunes    = 1000
)

// postCategoryList is the ordered source of truth; postCategories validates
// membership. Keep them in sync.
var postCategoryList = []string{
	"Genel", "Paylaşmak istedim", "Aile", "İlişki", "İş & okul",
	"KPSS", "İş arıyorum", "Ev arıyorum", "Yalnızlık", "Sağlık", "Diğer",
}

var postCategories = func() map[string]bool {
	set := make(map[string]bool, len(postCategoryList))
	for _, category := range postCategoryList {
		set[category] = true
	}
	return set
}()

// postSorts drives the wall's sort chips; the client sends the key, the API
// only understands these values.
var postSorts = []map[string]string{
	{"key": "gundem", "label": "Gündem"},
	{"key": "yeni", "label": "Yeni"},
}

type Server struct {
	cfg     Config
	store   *Store
	now     func() time.Time
	authIP  *Limiter
	signIP  *Limiter
	login   *Limiter
	post10  *Limiter
	postDay *Limiter
	msgSlow *Limiter
	apiIP   *Limiter
	writeIP *Limiter
	pollIP  *Limiter
	handle  http.Handler
}

func New(cfg Config, store *Store) *Server {
	s := &Server{
		cfg:     cfg,
		store:   store,
		now:     time.Now,
		authIP:  NewLimiter(30, 30),
		signIP:  NewLimiter(10.0/60.0, 10),
		login:   NewLimiter(1.2, 6),
		post10:  NewLimiter(0.1, 1),
		postDay: NewLimiter(4.0/1440.0, 4),
		// One message every 10 seconds: burst 1, refill 6/min.
		msgSlow: NewLimiter(6, 1),
		apiIP:   NewLimiter(240, 240),
		writeIP: NewLimiter(30, 30),
		pollIP:  NewLimiter(60, 60),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("POST /api/auth/signup/start", s.handleSignupStart)
	mux.HandleFunc("POST /api/auth/signup/confirm", s.handleSignupConfirm)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)
	mux.HandleFunc("GET /api/posts", s.handleListPosts)
	mux.HandleFunc("POST /api/posts", s.handleCreatePost)
	mux.HandleFunc("GET /api/posts/{id}", s.handlePost)
	mux.HandleFunc("POST /api/posts/{id}/support", s.handleToggleSupport)
	mux.HandleFunc("POST /api/posts/{id}/save", s.handleToggleSave)
	mux.HandleFunc("GET /api/me/posts", s.handleMyPosts)
	mux.HandleFunc("GET /api/me/saved", s.handleSavedPosts)
	mux.HandleFunc("GET /api/me/stats", s.handleStats)
	mux.HandleFunc("POST /api/me/recovery-codes", s.handleRotateRecoveryCodes)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/conversations", s.handleListConversations)
	mux.HandleFunc("POST /api/conversations", s.handleStartConversation)
	mux.HandleFunc("GET /api/conversations/{id}", s.handleConversation)
	mux.HandleFunc("POST /api/conversations/{id}/messages", s.handleSendMessage)
	mux.HandleFunc("POST /api/conversations/{id}/close", s.handleCloseConversation)
	mux.HandleFunc("GET /api/notifications", s.handleNotifications)
	mux.HandleFunc("POST /api/notifications/read", s.handleMarkNotificationsRead)
	mux.HandleFunc("/", s.handleStatic)
	s.handle = s.middleware(mux)
	return s
}

// runAt points every clock in the server at the caller's `now` variable so
// tests can advance time. The variable is captured by reference.
func (s *Server) runAt(now *time.Time) {
	s.now = func() time.Time { return *now }
	s.authIP.now = s.now
	s.signIP.now = s.now
	s.login.now = s.now
	s.post10.now = s.now
	s.postDay.now = s.now
	s.msgSlow.now = s.now
	s.apiIP.now = s.now
	s.writeIP.now = s.now
	s.pollIP.now = s.now
}

func (s *Server) Handler() http.Handler { return s.handle }

func (s *Server) Sweep() {
	s.authIP.Sweep(15 * time.Minute)
	s.signIP.Sweep(2 * time.Hour)
	s.login.Sweep(30 * time.Minute)
	s.post10.Sweep(2 * time.Hour)
	s.postDay.Sweep(48 * time.Hour)
	s.msgSlow.Sweep(30 * time.Minute)
	s.apiIP.Sweep(15 * time.Minute)
	s.writeIP.Sweep(30 * time.Minute)
	s.pollIP.Sweep(15 * time.Minute)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic: %v", recovered)
				writeErr(w, http.StatusInternalServerError, "Beklenmeyen bir hata oluştu.")
			}
		}()

		s.setSecurityHeaders(w)
		if s.handleCORS(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" && !s.allowedOrigin(origin) && !s.sameOriginHost(r) {
				writeErr(w, http.StatusForbidden, "Bu kaynak izinli değil.")
				return
			}
		}

		started := time.Now()
		status := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(status, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, status.status, time.Since(started).Round(time.Millisecond))
	})
}

func (s *Server) setSecurityHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "same-origin")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	connect := "'self'"
	if len(s.cfg.Origins) > 0 {
		connect += " " + strings.Join(s.cfg.Origins, " ")
	}
	header.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; font-src 'self'; connect-src "+connect+"; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	if s.cfg.CookieSecure {
		header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	if origin == "" {
		return false
	}
	if !s.allowedOrigin(origin) && !s.sameOriginHost(r) {
		if r.Method == http.MethodOptions {
			writeErr(w, http.StatusForbidden, "Bu kaynak izinli değil.")
			return true
		}
		return false
	}
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Credentials", "true")
	header.Set("Vary", "Origin")
	if r.Method == http.MethodOptions {
		header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Content-Type")
		header.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func (s *Server) allowedOrigin(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	for _, allowed := range s.cfg.Origins {
		if origin == allowed {
			return true
		}
	}
	return false
}

// sameOriginHost reports whether the Origin header host matches the request
// Host. Behind a tunnel the public hostname is unknown up front, so a
// same-origin request from the tunnel domain is accepted when proxy trust is
// on; this is safe because the browser only sends Origin on same-origin writes.
func (s *Server) sameOriginHost(r *http.Request) bool {
	if !s.cfg.TrustProxy {
		return false
	}
	origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
				return first
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("json response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeErr(w, http.StatusBadRequest, "Geçersiz istek.")
		return false
	}
	return true
}

func tooMany(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	writeErr(w, http.StatusTooManyRequests, "Çok fazla deneme. Bir süre sonra tekrar dene.")
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	writeErr(w, http.StatusInternalServerError, "Beklenmeyen bir hata oluştu.")
}

func (s *Server) setSession(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) sessionUser(r *http.Request) *User {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil
	}
	user, err := s.store.SessionUser(HashToken(cookie.Value), s.now())
	if err != nil {
		return nil
	}
	return user
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) *User {
	user := s.sessionUser(r)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "Bunun için giriş yapmalısın.")
		return nil
	}
	return user
}

type userJSON struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
	PostCount int    `json:"post_count"`
}

func (s *Server) publicUser(user *User) userJSON {
	count, err := s.store.CountPostsByUser(user.ID)
	if err != nil {
		count = 0
	}
	return userJSON{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
		PostCount: count,
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type signupStartRequest struct {
	Username string `json:"username"`
}

func (s *Server) handleSignupStart(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.authIP.Allow("auth:"+ip) || !s.signIP.Allow("signup:"+ip) {
		tooMany(w)
		return
	}
	var req signupStartRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	username, err := NormalizeUsername(req.Username)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.UserByUsername(username); err == nil {
		writeErr(w, http.StatusConflict, "Bu kullanıcı adı zaten alınmış.")
		return
	} else if !errors.Is(err, ErrNotFound) {
		serverError(w, err)
		return
	}
	key, err := GenerateTOTP(s.cfg.Issuer, username)
	if err != nil {
		serverError(w, err)
		return
	}
	signupID, err := s.store.CreatePendingSignup(username, key.Secret(), s.now(), signupTTL)
	if err != nil {
		serverError(w, err)
		return
	}
	png, err := qrcode.Encode(key.URL(), qrcode.Medium, 320)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"signup_id":   signupID,
		"username":    username,
		"secret":      key.Secret(),
		"otpauth_url": key.URL(),
		"qr":          "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

type signupConfirmRequest struct {
	SignupID string `json:"signup_id"`
	Code     string `json:"code"`
}

func (s *Server) handleSignupConfirm(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.authIP.Allow("auth:" + ip) {
		tooMany(w)
		return
	}
	var req signupConfirmRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	now := s.now()
	// Own bucket per pending signup: an attacker cannot burn someone else's
	// attempt budget, and each signup gets its own brute-force ceiling.
	if !s.signIP.Allow("confirm:" + req.SignupID) {
		tooMany(w)
		return
	}
	pending, err := s.store.PendingSignup(req.SignupID)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusBadRequest, "Kayıt bulunamadı veya süresi doldu. Baştan dene.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if now.After(pending.ExpiresAt) {
		s.store.DeletePendingSignup(pending.ID)
		writeErr(w, http.StatusBadRequest, "Kayıt süresi doldu. Baştan dene.")
		return
	}
	if pending.Attempts >= maxSignupAttempts {
		s.store.DeletePendingSignup(pending.ID)
		tooMany(w)
		return
	}
	step, valid := VerifyTOTP(pending.Secret, req.Code, now)
	if !valid {
		attempts, err := s.store.IncrementPendingAttempts(pending.ID)
		if err == nil && attempts >= maxSignupAttempts {
			s.store.DeletePendingSignup(pending.ID)
			tooMany(w)
			return
		}
		writeErr(w, http.StatusBadRequest, "Kod geçersiz. Authenticator uygulamasındaki güncel kodu gir.")
		return
	}
	user, err := s.store.CreateUser(pending.Username, pending.Secret, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeErr(w, http.StatusConflict, "Bu kullanıcı adı zaten alınmış.")
			return
		}
		serverError(w, err)
		return
	}
	s.store.DeletePendingSignup(pending.ID)
	codes, hashes, err := GenerateRecoveryCodes(maxRecoveryCodes)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.CreateRecoveryCodes(user.ID, hashes); err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.SetLastTOTPStep(user.ID, step, now); err != nil {
		serverError(w, err)
		return
	}
	token, hash, err := NewSessionToken()
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.CreateSession(user.ID, hash, now, sessionTTL); err != nil {
		serverError(w, err)
		return
	}
	s.setSession(w, token, int(sessionTTL.Seconds()))
	writeJSON(w, http.StatusCreated, map[string]any{
		"user":           s.publicUser(user),
		"recovery_codes": codes,
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Code     string `json:"code"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.authIP.Allow("auth:" + ip) {
		tooMany(w)
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	username, err := NormalizeUsername(req.Username)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "Kullanıcı adı veya kod hatalı.")
		return
	}
	if !s.login.Allow("login:" + username) {
		tooMany(w)
		return
	}
	user, err := s.store.UserByUsername(username)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "Kullanıcı adı veya kod hatalı.")
		return
	}
	now := s.now()
	usedRecovery := false
	if normalized, isRecovery := NormalizeRecoveryCode(req.Code); isRecovery {
		ok, err := s.store.UseRecoveryCode(user.ID, HashRecoveryCode(normalized), now)
		if err != nil {
			serverError(w, err)
			return
		}
		usedRecovery = ok
	} else {
		step, valid := VerifyTOTP(user.TOTPSecret, req.Code, now)
		if valid && step > user.LastTOTPStep {
			if err := s.store.SetLastTOTPStep(user.ID, step, now); err != nil {
				serverError(w, err)
				return
			}
			usedRecovery = true
		}
	}
	if !usedRecovery {
		writeErr(w, http.StatusUnauthorized, "Kullanıcı adı veya kod hatalı.")
		return
	}
	if usedRecovery {
		s.store.TouchLogin(user.ID, now)
	}
	token, hash, err := NewSessionToken()
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.CreateSession(user.ID, hash, now, sessionTTL); err != nil {
		serverError(w, err)
		return
	}
	s.setSession(w, token, int(sessionTTL.Seconds()))
	writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(user)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		s.store.DeleteSession(HashToken(cookie.Value))
	}
	s.setSession(w, "", -1)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := s.sessionUser(r)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "Giriş yapılmamış.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(user)})
}

// handleMeta exposes the small, slowly-changing vocabulary the UI renders:
// post categories and wall sort options. Keeping it server-side removes the
// hardcoded copies from the frontend.
func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if !s.apiIP.Allow("api:" + s.clientIP(r)) {
		tooMany(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": postCategoryList,
		"sorts":      postSorts,
	})
}

func (s *Server) handleListPosts(w http.ResponseWriter, r *http.Request) {
	if !s.apiIP.Allow("api:" + s.clientIP(r)) {
		tooMany(w)
		return
	}
	posts, err := s.store.ListPosts(200)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

type createPostRequest struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.post10.Allow("post10:"+user.ID) || !s.postDay.Allow("postDay:"+user.ID) {
		tooMany(w)
		return
	}
	var req createPostRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	if !postCategories[req.Category] {
		writeErr(w, http.StatusBadRequest, "Kategori geçersiz.")
		return
	}
	if n := utf8.RuneCountInString(title); n < 10 || n > maxPostTitleRunes {
		writeErr(w, http.StatusBadRequest, "Başlık 10-120 karakter olmalı.")
		return
	}
	if len(strings.Fields(title)) < 2 {
		writeErr(w, http.StatusBadRequest, "Başlık en az iki kelime olmalı.")
		return
	}
	if n := utf8.RuneCountInString(body); n < 80 || n > maxPostBodyRunes {
		writeErr(w, http.StatusBadRequest, "Detay 80-4000 karakter olmalı.")
		return
	}
	if len(strings.Fields(body)) < 12 {
		writeErr(w, http.StatusBadRequest, "Detay en az 12 kelime olmalı.")
		return
	}
	post := &Post{
		ID:        NewID(),
		UserID:    user.ID,
		Author:    user.Username,
		Category:  req.Category,
		Title:     title,
		Excerpt:   truncateRunes(body, maxPostExcerptRune),
		Body:      body,
		Folder:    "dert",
		CreatedAt: s.now(),
	}
	if err := s.store.CreatePost(post); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"post": post})
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	if !s.apiIP.Allow("api:" + s.clientIP(r)) {
		tooMany(w)
		return
	}
	id := r.PathValue("id")
	if len(id) < 8 || len(id) > 64 {
		writeErr(w, http.StatusNotFound, "Paylaşım bulunamadı.")
		return
	}
	post, err := s.store.PostByID(id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Paylaşım bulunamadı.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	payload := map[string]any{"post": post, "mine": false, "supported": false, "saved": false, "conversation_id": ""}
	if viewer := s.sessionUser(r); viewer != nil {
		if state, err := s.store.ViewerState(viewer.ID); err == nil {
			for _, supported := range state.Supported {
				if supported == post.ID {
					payload["supported"] = true
					break
				}
			}
			for _, saved := range state.Saved {
				if saved == post.ID {
					payload["saved"] = true
					break
				}
			}
		}
		payload["mine"] = post.UserID == viewer.ID
		if conv, err := s.store.FindConversation(post.ID, post.UserID, viewer.ID); err == nil && conv != nil {
			payload["conversation_id"] = conv.ID
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

// ---------- supports, saves ----------

func (s *Server) handleToggleSupport(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.writeIP.Allow("write:" + user.ID) {
		tooMany(w)
		return
	}
	post, err := s.store.PostByID(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Paylaşım bulunamadı.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	on, count, err := s.store.ToggleSupport(post.ID, user.ID, s.now())
	if err != nil {
		serverError(w, err)
		return
	}
	if on && post.UserID != "" && post.UserID != user.ID {
		s.notify(post.UserID, "destek", fmt.Sprintf("@%s paylaşımına destek verdi.", user.Username), post.ID, "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"supported": on, "supports": count})
}

func (s *Server) handleToggleSave(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.writeIP.Allow("write:" + user.ID) {
		tooMany(w)
		return
	}
	post, err := s.store.PostByID(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Paylaşım bulunamadı.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	on, err := s.store.ToggleSave(post.ID, user.ID, s.now())
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": on})
}

func (s *Server) handleSavedPosts(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	posts, err := s.store.SavedPosts(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

// notify writes a notification, ignoring failures: a missing alert must never
// break the action that triggered it.
func (s *Server) notify(userID, kind, text, postID, conversationID string) {
	if userID == "" {
		return
	}
	err := s.store.CreateNotification(&Notification{
		ID:             NewID(),
		UserID:         userID,
		Kind:           kind,
		Text:           text,
		PostID:         postID,
		ConversationID: conversationID,
		CreatedAt:      s.now(),
	})
	if err != nil {
		log.Printf("notify: %v", err)
	}
}

// ---------- conversations ----------

type conversationJSON struct {
	ID          string    `json:"id"`
	PostID      string    `json:"post_id"`
	PostTitle   string    `json:"post_title"`
	PeerName    string    `json:"peer_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	Unread      int       `json:"unread"`
	OpenedAt    time.Time `json:"opened_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastMessage time.Time `json:"last_message_at"`
	Preview     string    `json:"preview"`
}

func (s *Server) conversationJSON(c Conversation, viewerID string) conversationJSON {
	item := conversationJSON{
		ID: c.ID, PostID: c.PostID, PostTitle: c.PostTitle,
		Status: c.Status, OpenedAt: c.OpenedAt, ExpiresAt: c.ExpiresAt, LastMessage: c.LastMessage,
		Role: "writer", PeerName: c.OwnerName, Unread: c.WriterUnread,
	}
	if c.OwnerID == viewerID {
		item.Role = "owner"
		item.PeerName = c.WriterName
		item.Unread = c.OwnerUnread
	}
	if last, err := s.store.LastMessage(c.ID, s.now().Add(-conversationTTL)); err == nil && last != nil {
		item.Preview = truncateRunes(last.Body, 90)
	}
	return item
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	list, err := s.store.ListConversations(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	items := make([]conversationJSON, 0, len(list))
	for _, c := range list {
		items = append(items, s.conversationJSON(c, user.ID))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (s *Server) handleStartConversation(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.writeIP.Allow("write:" + user.ID) {
		tooMany(w)
		return
	}
	var req struct {
		PostID  string `json:"post_id"`
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	body := strings.TrimSpace(req.Message)
	if n := utf8.RuneCountInString(body); n < 1 || n > maxMessageRunes {
		writeErr(w, http.StatusBadRequest, "Mesaj 1-1000 karakter olmalı.")
		return
	}
	post, err := s.store.PostByID(strings.TrimSpace(req.PostID))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Paylaşım bulunamadı.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if post.UserID == "" {
		writeErr(w, http.StatusBadRequest, "Bu paylaşımın sahibi artık yazamıyor.")
		return
	}
	if post.UserID == user.ID {
		writeErr(w, http.StatusBadRequest, "Kendi paylaşımına sohbet açamazsın.")
		return
	}
	now := s.now()
	if !s.msgSlow.Allow("msgslow:" + user.ID) {
		writeErr(w, http.StatusTooManyRequests, "Çok hızlı yazıyorsun. Birkaç saniye bekleyip tekrar dene.")
		return
	}
	conv, err := s.store.FindConversation(post.ID, post.UserID, user.ID)
	if errors.Is(err, ErrNotFound) {
		conv = &Conversation{
			ID: NewID(), PostID: post.ID, OwnerID: post.UserID, WriterID: user.ID,
			Status: "open", OpenedAt: now, ExpiresAt: now.Add(conversationTTL), LastMessage: now,
		}
		if err := s.store.CreateConversation(conv); err != nil {
			serverError(w, err)
			return
		}
		if err := s.store.IncrementChats(post.ID); err != nil {
			log.Printf("increment chats: %v", err)
		}
		// Reload so the joined usernames and title are filled in.
		if reloaded, err := s.store.ConversationByID(conv.ID); err == nil {
			conv = reloaded
		}
	} else if err != nil {
		serverError(w, err)
		return
	}
	if conv.Status == "closed" {
		writeErr(w, http.StatusForbidden, "Bu sohbet kapatıldı.")
		return
	}
	message, err := s.sendMessage(conv, user, body, now)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"conversation": s.conversationJSON(*conv, user.ID),
		"message":      message,
	})
}

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	conv, ok := s.ownedConversation(w, r, user)
	if !ok {
		return
	}
	after := r.URL.Query().Get("after")
	if after != "" && !s.pollIP.Allow("poll:"+user.ID) {
		tooMany(w)
		return
	}
	messages, err := s.store.Messages(conv.ID, after, 200, s.now().Add(-conversationTTL))
	if err != nil {
		serverError(w, err)
		return
	}
	if after == "" {
		if err := s.store.MarkConversationRead(conv.ID, user.ID); err != nil {
			log.Printf("mark read: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": s.conversationJSON(*conv, user.ID),
		"messages":     messages,
	})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.writeIP.Allow("write:" + user.ID) {
		tooMany(w)
		return
	}
	conv, ok := s.ownedConversation(w, r, user)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	body := strings.TrimSpace(req.Body)
	if n := utf8.RuneCountInString(body); n < 1 || n > maxMessageRunes {
		writeErr(w, http.StatusBadRequest, "Mesaj 1-1000 karakter olmalı.")
		return
	}
	if conv.Status == "closed" {
		writeErr(w, http.StatusForbidden, "Bu sohbet kapatıldı.")
		return
	}
	now := s.now()
	if !s.msgSlow.Allow("msgslow:" + user.ID) {
		writeErr(w, http.StatusTooManyRequests, "Çok hızlı yazıyorsun. Birkaç saniye bekleyip tekrar dene.")
		return
	}
	message, err := s.sendMessage(conv, user, body, now)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": message})
}

func (s *Server) handleCloseConversation(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	conv, ok := s.ownedConversation(w, r, user)
	if !ok {
		return
	}
	if err := s.store.SetConversationStatus(conv.ID, "closed"); err != nil {
		serverError(w, err)
		return
	}
	s.notify(s.peerID(*conv, user.ID), "sohbet", fmt.Sprintf("@%s sohbeti kapattı.", user.Username), conv.PostID, conv.ID)
	conv.Status = "closed"
	writeJSON(w, http.StatusOK, map[string]any{"conversation": s.conversationJSON(*conv, user.ID)})
}

// sendMessage stores the message, moves the conversation clock and alerts the
// other side.
func (s *Server) sendMessage(conv *Conversation, sender *User, body string, now time.Time) (*Message, error) {
	message := &Message{
		ID: NewID(), ConversationID: conv.ID, SenderID: sender.ID,
		SenderName: sender.Username, Body: body, CreatedAt: now,
	}
	if err := s.store.CreateMessage(message); err != nil {
		return nil, err
	}
	if err := s.store.TouchConversation(conv.ID, sender.ID, now); err != nil {
		return nil, err
	}
	preview := truncateRunes(body, 60)
	s.notify(s.peerID(*conv, sender.ID), "mesaj", fmt.Sprintf("@%s: %s", sender.Username, preview), conv.PostID, conv.ID)
	conv.LastMessage = now
	return message, nil
}

func (s *Server) peerID(conv Conversation, viewerID string) string {
	if conv.OwnerID == viewerID {
		return conv.WriterID
	}
	return conv.OwnerID
}

// ownedConversation loads a conversation and ensures the caller is a member.
func (s *Server) ownedConversation(w http.ResponseWriter, r *http.Request, user *User) (*Conversation, bool) {
	conv, err := s.store.ConversationByID(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Sohbet bulunamadı.")
		return nil, false
	}
	if err != nil {
		serverError(w, err)
		return nil, false
	}
	if conv.OwnerID != user.ID && conv.WriterID != user.ID {
		writeErr(w, http.StatusForbidden, "Bu sohbete erişimin yok.")
		return nil, false
	}
	return conv, true
}

// ---------- notifications ----------

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	items, err := s.store.Notifications(user.ID, 50)
	if err != nil {
		serverError(w, err)
		return
	}
	unread, err := s.store.UnreadNotifications(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": items, "unread": unread})
}

func (s *Server) handleMarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if err := s.store.MarkNotificationsRead(user.ID, s.now()); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	unreadMessages, err := s.store.UnreadCount(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	unreadNotifications, err := s.store.UnreadNotifications(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"unread_messages":      unreadMessages,
		"unread_notifications": unreadNotifications,
	})
}

func (s *Server) handleMyPosts(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	posts, err := s.store.UserPosts(user.ID, 50)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) handleRotateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	if !s.writeIP.Allow("write:" + user.ID) {
		tooMany(w)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.login.Allow("rotate:" + user.Username) {
		tooMany(w)
		return
	}
	valid := false
	if normalized, isRecovery := NormalizeRecoveryCode(req.Code); isRecovery {
		ok, err := s.store.UseRecoveryCode(user.ID, HashRecoveryCode(normalized), s.now())
		if err != nil {
			serverError(w, err)
			return
		}
		valid = ok
	} else if _, ok := VerifyTOTP(user.TOTPSecret, strings.TrimSpace(req.Code), s.now()); ok {
		valid = true
	}
	if !valid {
		writeErr(w, http.StatusUnauthorized, "Kod geçersiz. Authenticator kodunu dene.")
		return
	}
	codes, hashes, err := GenerateRecoveryCodes(maxRecoveryCodes)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.store.ReplaceRecoveryCodes(user.ID, hashes); err != nil {
		serverError(w, err)
		return
	}
	s.notify(user.ID, "guvenlik", "Yeni kurtarma kodları üretildi; eskiler geçersiz.", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	user := s.requireUser(w, r)
	if user == nil {
		return
	}
	stats, err := s.store.Stats(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	left, err := s.store.RecoveryCodesLeft(user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats, "recovery_codes_left": left})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "Bulunamadı.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeErr(w, http.StatusMethodNotAllowed, "Yöntem desteklenmiyor.")
		return
	}
	clean := path.Clean(r.URL.Path)
	if clean == "/dert" || strings.HasPrefix(clean, "/dert/") {
		s.serveFile(w, r, "dert/index.html")
		return
	}
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	full := filepath.Join(s.cfg.WebDir, filepath.FromSlash(rel))
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		s.setCacheHeaders(w, clean)
		http.ServeFile(w, r, full)
		return
	}
	index := filepath.Join(full, "index.html")
	if info, err := os.Stat(index); err == nil && !info.IsDir() {
		s.setCacheHeaders(w, clean)
		http.ServeFile(w, r, index)
		return
	}
	notFound := filepath.Join(s.cfg.WebDir, "404.html")
	if info, err := os.Stat(notFound); err == nil && !info.IsDir() {
		data, readErr := os.ReadFile(notFound)
		if readErr == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusNotFound)
			w.Write(data)
			return
		}
	}
	writeErr(w, http.StatusNotFound, "Bulunamadı.")
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, rel string) {
	full := filepath.Join(s.cfg.WebDir, filepath.FromSlash(rel))
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		writeErr(w, http.StatusNotFound, "Bulunamadı.")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, full)
}

func (s *Server) setCacheHeaders(w http.ResponseWriter, cleanPath string) {
	if strings.HasPrefix(cleanPath, "/_next/static/") || strings.HasPrefix(cleanPath, "/fonts/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}

func (s *Server) String() string {
	return fmt.Sprintf("icinidok(%s)", s.cfg.Addr)
}
