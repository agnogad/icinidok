package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

type User struct {
	ID           string
	Username     string
	TOTPSecret   string
	LastTOTPStep int64
	CreatedAt    time.Time
	LastLoginAt  time.Time
}

type PendingSignup struct {
	ID        string
	Username  string
	Secret    string
	Attempts  int
	ExpiresAt time.Time
}

type Post struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	Excerpt   string    `json:"excerpt"`
	Body      string    `json:"body,omitempty"`
	Author    string    `json:"author"`
	Folder    string    `json:"folder"`
	Supports  int       `json:"supports"`
	Chats     int       `json:"chats"`
	CreatedAt time.Time `json:"created_at"`
}

func OpenStore(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One writer keeps SQLite simple and predictable at this scale.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			totp_secret TEXT NOT NULL,
			last_totp_step INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_login_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS pending_signups (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			secret TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS recovery_codes (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			code_hash TEXT NOT NULL,
			used_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, code_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id)`,
		`CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
			author TEXT NOT NULL,
			category TEXT NOT NULL,
			title TEXT NOT NULL,
			excerpt TEXT NOT NULL,
			body TEXT NOT NULL,
			folder TEXT NOT NULL DEFAULT 'dert',
			supports INTEGER NOT NULL DEFAULT 0,
			chats INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS posts_created_idx ON posts(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS supports (
			post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (post_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS saved (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (user_id, post_id)
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
			owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			writer_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'open',
			owner_unread INTEGER NOT NULL DEFAULT 0,
			writer_unread INTEGER NOT NULL DEFAULT 0,
			opened_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			last_message_at INTEGER NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS conversations_pair_idx ON conversations(post_id, owner_id, writer_id)`,
		`CREATE INDEX IF NOT EXISTS conversations_owner_idx ON conversations(owner_id, last_message_at DESC)`,
		`CREATE INDEX IF NOT EXISTS conversations_writer_idx ON conversations(writer_id, last_message_at DESC)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			sender_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			body TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS messages_conversation_idx ON messages(conversation_id, created_at)`, `CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			text TEXT NOT NULL,
			post_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			read_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id, created_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

type recoverable interface {
	Scan(...any) error
}

type Conversation struct {
	ID           string
	PostID       string
	PostTitle    string
	OwnerID      string
	OwnerName    string
	WriterID     string
	WriterName   string
	Status       string
	OwnerUnread  int
	WriterUnread int
	OpenedAt     time.Time
	ExpiresAt    time.Time
	LastMessage  time.Time
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

type Notification struct {
	ID             string    `json:"id"`
	UserID         string    `json:"-"`
	Kind           string    `json:"kind"`
	Text           string    `json:"text"`
	PostID         string    `json:"post_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Read           bool      `json:"read"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreatePendingSignup(username, secret string, now time.Time, ttl time.Duration) (string, error) {
	if _, err := s.db.Exec(`DELETE FROM pending_signups WHERE username = ? OR expires_at <= ?`, username, now.Unix()); err != nil {
		return "", err
	}
	id := NewID()
	_, err := s.db.Exec(
		`INSERT INTO pending_signups (id, username, secret, attempts, created_at, expires_at) VALUES (?, ?, ?, 0, ?, ?)`,
		id, username, secret, now.Unix(), now.Add(ttl).Unix(),
	)
	return id, err
}

func (s *Store) PendingSignup(id string) (*PendingSignup, error) {
	var pending PendingSignup
	var expires int64
	err := s.db.QueryRow(
		`SELECT id, username, secret, attempts, expires_at FROM pending_signups WHERE id = ?`, id,
	).Scan(&pending.ID, &pending.Username, &pending.Secret, &pending.Attempts, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	pending.ExpiresAt = time.Unix(expires, 0)
	return &pending, nil
}

func (s *Store) IncrementPendingAttempts(id string) (int, error) {
	if _, err := s.db.Exec(`UPDATE pending_signups SET attempts = attempts + 1 WHERE id = ?`, id); err != nil {
		return 0, err
	}
	var attempts int
	err := s.db.QueryRow(`SELECT attempts FROM pending_signups WHERE id = ?`, id).Scan(&attempts)
	return attempts, err
}

func (s *Store) DeletePendingSignup(id string) error {
	_, err := s.db.Exec(`DELETE FROM pending_signups WHERE id = ?`, id)
	return err
}

func (s *Store) CreateUser(username, secret string, now time.Time) (*User, error) {
	user := &User{ID: NewID(), Username: username, TOTPSecret: secret, CreatedAt: now}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, totp_secret, last_totp_step, created_at, last_login_at) VALUES (?, ?, ?, 0, ?, ?)`,
		user.ID, user.Username, user.TOTPSecret, now.Unix(), now.Unix(),
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var user User
	var created, lastLogin int64
	if err := row.Scan(&user.ID, &user.Username, &user.TOTPSecret, &user.LastTOTPStep, &created, &lastLogin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	user.CreatedAt = time.Unix(created, 0)
	if lastLogin > 0 {
		user.LastLoginAt = time.Unix(lastLogin, 0)
	}
	return &user, nil
}

const userColumns = `id, username, totp_secret, last_totp_step, created_at, last_login_at`

func (s *Store) UserByUsername(username string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ? COLLATE NOCASE`, username))
}

func (s *Store) UserByID(id string) (*User, error) {
	return scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (s *Store) SetLastTOTPStep(userID string, step int64, now time.Time) error {
	_, err := s.db.Exec(`UPDATE users SET last_totp_step = ?, last_login_at = ? WHERE id = ?`, step, now.Unix(), userID)
	return err
}

func (s *Store) TouchLogin(userID string, now time.Time) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now.Unix(), userID)
	return err
}

func (s *Store) CreateSession(userID, tokenHash string, now time.Time, ttl time.Duration) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, now.Unix(), now.Add(ttl).Unix(),
	)
	return err
}

func (s *Store) SessionUser(tokenHash string, now time.Time) (*User, error) {
	user, err := scanUser(s.db.QueryRow(
		`SELECT u.id, u.username, u.totp_secret, u.last_totp_step, u.created_at, u.last_login_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.Unix(),
	))
	if errors.Is(err, ErrNotFound) {
		s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	}
	return user, err
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) CreateRecoveryCodes(userID string, hashes []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, hash := range hashes {
		if _, err := tx.Exec(`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UseRecoveryCode(userID, hash string, now time.Time) (bool, error) {
	result, err := s.db.Exec(
		`UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at = 0`,
		now.Unix(), userID, hash,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// ReplaceRecoveryCodes discards all old codes and installs a fresh set.
func (s *Store) ReplaceRecoveryCodes(userID string, hashes []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err := tx.Exec(`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecoveryCodesLeft(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at = 0`, userID).Scan(&count)
	return count, err
}

func (s *Store) CreatePost(post *Post) error {
	_, err := s.db.Exec(
		`INSERT INTO posts (id, user_id, author, category, title, excerpt, body, folder, supports, chats, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		post.ID, post.UserID, post.Author, post.Category, post.Title, post.Excerpt, post.Body, post.Folder, post.Supports, post.Chats, post.CreatedAt.Unix(),
	)
	return err
}

const postColumns = `id, COALESCE(user_id, ''), author, category, title, excerpt, body, folder, supports, chats, created_at`

func scanPost(row interface{ Scan(...any) error }) (*Post, error) {
	var post Post
	var created int64
	if err := row.Scan(&post.ID, &post.UserID, &post.Author, &post.Category, &post.Title, &post.Excerpt, &post.Body, &post.Folder, &post.Supports, &post.Chats, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	post.CreatedAt = time.Unix(created, 0)
	return &post, nil
}

func (s *Store) ListPosts(limit int) ([]Post, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT `+postColumns+` FROM posts ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0, limit)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		post.Body = ""
		posts = append(posts, *post)
	}
	return posts, rows.Err()
}

func (s *Store) PostByID(id string) (*Post, error) {
	return scanPost(s.db.QueryRow(`SELECT `+postColumns+` FROM posts WHERE id = ?`, id))
}

func (s *Store) CountPostsByUser(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE user_id = ?`, userID).Scan(&count)
	return count, err
}

// ---------- supports & saves ----------

// ToggleSupport adds or removes a support and keeps the cached counter in sync.
func (s *Store) ToggleSupport(postID, userID string, now time.Time) (bool, int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM supports WHERE post_id = ? AND user_id = ?`, postID, userID)
	if err != nil {
		return false, 0, err
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return false, 0, err
	}
	on := removed == 0
	if on {
		if _, err := tx.Exec(`INSERT INTO supports (post_id, user_id, created_at) VALUES (?, ?, ?)`, postID, userID, now.Unix()); err != nil {
			return false, 0, err
		}
		if _, err := tx.Exec(`UPDATE posts SET supports = supports + 1 WHERE id = ?`, postID); err != nil {
			return false, 0, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE posts SET supports = MAX(supports - 1, 0) WHERE id = ?`, postID); err != nil {
			return false, 0, err
		}
	}
	var count int
	if err := tx.QueryRow(`SELECT supports FROM posts WHERE id = ?`, postID).Scan(&count); err != nil {
		return false, 0, err
	}
	return on, count, tx.Commit()
}

func (s *Store) ToggleSave(postID, userID string, now time.Time) (bool, error) {
	result, err := s.db.Exec(`DELETE FROM saved WHERE user_id = ? AND post_id = ?`, userID, postID)
	if err != nil {
		return false, err
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if removed > 0 {
		return false, nil
	}
	_, err = s.db.Exec(`INSERT INTO saved (user_id, post_id, created_at) VALUES (?, ?, ?)`, userID, postID, now.Unix())
	return true, err
}

func (s *Store) SavedPosts(userID string) ([]Post, error) {
	rows, err := s.db.Query(
		`SELECT p.id, COALESCE(p.user_id, ''), p.author, p.category, p.title, p.excerpt, p.folder, p.supports, p.chats, p.created_at
		 FROM saved s JOIN posts p ON p.id = s.post_id
		 WHERE s.user_id = ? ORDER BY s.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		var post Post
		var created int64
		if err := rows.Scan(&post.ID, &post.UserID, &post.Author, &post.Category, &post.Title, &post.Excerpt, &post.Folder, &post.Supports, &post.Chats, &created); err != nil {
			return nil, err
		}
		post.CreatedAt = time.Unix(created, 0)
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// ViewerState reports which posts the viewer supported or saved.
type ViewerState struct {
	Supported []string `json:"supported"`
	Saved     []string `json:"saved"`
}

func (s *Store) ViewerState(userID string) (*ViewerState, error) {
	state := &ViewerState{Supported: []string{}, Saved: []string{}}
	rows, err := s.db.Query(`SELECT post_id FROM supports WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		state.Supported = append(state.Supported, id)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT post_id FROM saved WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		state.Saved = append(state.Saved, id)
	}
	return state, rows.Err()
}

// ---------- conversations ----------

const conversationSelect = `SELECT c.id, c.post_id, p.title, c.owner_id, owner.username, c.writer_id, writer.username,
		c.status, c.owner_unread, c.writer_unread, c.opened_at, c.expires_at, c.last_message_at
	FROM conversations c
	JOIN posts p ON p.id = c.post_id
	JOIN users owner ON owner.id = c.owner_id
	JOIN users writer ON writer.id = c.writer_id`

func scanConversation(row recoverable) (*Conversation, error) {
	var c Conversation
	var opened, expires, last int64
	err := row.Scan(&c.ID, &c.PostID, &c.PostTitle, &c.OwnerID, &c.OwnerName, &c.WriterID, &c.WriterName,
		&c.Status, &c.OwnerUnread, &c.WriterUnread, &opened, &expires, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.OpenedAt = time.Unix(opened, 0)
	c.ExpiresAt = time.Unix(expires, 0)
	c.LastMessage = time.Unix(last, 0)
	return &c, nil
}

// FindConversation returns the existing conversation for a post between the
// writer and the post owner, if any.
func (s *Store) FindConversation(postID, ownerID, writerID string) (*Conversation, error) {
	return scanConversation(s.db.QueryRow(
		conversationSelect+` WHERE c.post_id = ? AND c.owner_id = ? AND c.writer_id = ?`,
		postID, ownerID, writerID,
	))
}

func (s *Store) ConversationByID(id string) (*Conversation, error) {
	return scanConversation(s.db.QueryRow(conversationSelect+` WHERE c.id = ?`, id))
}

func (s *Store) CreateConversation(c *Conversation) error {
	_, err := s.db.Exec(
		`INSERT INTO conversations (id, post_id, owner_id, writer_id, status, owner_unread, writer_unread, opened_at, expires_at, last_message_at)
		 VALUES (?, ?, ?, ?, 'open', 0, 0, ?, ?, ?)`,
		c.ID, c.PostID, c.OwnerID, c.WriterID, c.OpenedAt.Unix(), c.ExpiresAt.Unix(), c.LastMessage.Unix(),
	)
	return err
}

// ListConversations returns every conversation the user takes part in.
func (s *Store) ListConversations(userID string) ([]Conversation, error) {
	rows, err := s.db.Query(
		conversationSelect+` WHERE c.owner_id = ? OR c.writer_id = ? ORDER BY c.last_message_at DESC LIMIT 100`,
		userID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Conversation{}
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

func (s *Store) ConversationsByPost(postID string) ([]Conversation, error) {
	rows, err := s.db.Query(conversationSelect+` WHERE c.post_id = ? ORDER BY c.last_message_at DESC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Conversation{}
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

func (s *Store) CreateMessage(m *Message) error {
	_, err := s.db.Exec(
		`INSERT INTO messages (id, conversation_id, sender_id, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.SenderID, m.Body, m.CreatedAt.Unix(),
	)
	return err
}

func scanMessage(row recoverable) (*Message, error) {
	var m Message
	var created int64
	err := row.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.CreatedAt = time.Unix(created, 0)
	return &m, nil
}

func (s *Store) Messages(conversationID, afterID string, limit int, since time.Time) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, m.created_at
		FROM messages m JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = ? AND m.created_at >= ?`
	args := []any{conversationID, since.Unix()}
	if afterID != "" {
		query += ` AND m.rowid > (SELECT rowid FROM messages WHERE id = ?)`
		args = append(args, afterID)
	}
	query += ` ORDER BY m.rowid ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}

// LastMessage returns the newest message inside the 24h window, if any.
func (s *Store) LastMessage(conversationID string, since time.Time) (*Message, error) {
	return scanMessage(s.db.QueryRow(
		`SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, m.created_at
		 FROM messages m JOIN users u ON u.id = m.sender_id
		 WHERE m.conversation_id = ? AND m.created_at >= ? ORDER BY m.rowid DESC LIMIT 1`,
		conversationID, since.Unix(),
	))
}

// TouchConversation updates the message clock and bumps the recipient's unread
// counter. Whichever side did not send the message gets the badge.
func (s *Store) TouchConversation(conversationID, senderID string, now time.Time) error {
	_, err := s.db.Exec(
		`UPDATE conversations
		 SET last_message_at = ?,
		     status = CASE WHEN status = 'closed' THEN 'open' ELSE status END,
		     owner_unread = CASE WHEN owner_id = ? THEN owner_unread ELSE owner_unread + 1 END,
		     writer_unread = CASE WHEN writer_id = ? THEN writer_unread ELSE writer_unread + 1 END
		 WHERE id = ?`,
		now.Unix(), senderID, senderID, conversationID,
	)
	return err
}

// MarkConversationRead clears the unread counter for whichever side the reader
// belongs to.
func (s *Store) MarkConversationRead(conversationID, readerID string) error {
	_, err := s.db.Exec(
		`UPDATE conversations
		 SET owner_unread = CASE WHEN owner_id = ? THEN 0 ELSE owner_unread END,
		     writer_unread = CASE WHEN writer_id = ? THEN 0 ELSE writer_unread END
		 WHERE id = ?`,
		readerID, readerID, conversationID,
	)
	return err
}

func (s *Store) SetConversationStatus(conversationID, status string) error {
	_, err := s.db.Exec(`UPDATE conversations SET status = ? WHERE id = ?`, status, conversationID)
	return err
}

func (s *Store) UnreadCount(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN owner_id = ? THEN owner_unread ELSE writer_unread END), 0)
		 FROM conversations WHERE owner_id = ? OR writer_id = ?`,
		userID, userID, userID,
	).Scan(&count)
	return count, err
}

// ---------- notifications ----------

func (s *Store) CreateNotification(n *Notification) error {
	_, err := s.db.Exec(
		`INSERT INTO notifications (id, user_id, kind, text, post_id, conversation_id, read_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		n.ID, n.UserID, n.Kind, n.Text, n.PostID, n.ConversationID, n.CreatedAt.Unix(),
	)
	return err
}

func (s *Store) Notifications(userID string, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, kind, text, post_id, conversation_id, read_at, created_at
		 FROM notifications WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Notification{}
	for rows.Next() {
		var n Notification
		var read, created int64
		if err := rows.Scan(&n.ID, &n.Kind, &n.Text, &n.PostID, &n.ConversationID, &read, &created); err != nil {
			return nil, err
		}
		n.Read = read > 0
		n.CreatedAt = time.Unix(created, 0)
		list = append(list, n)
	}
	return list, rows.Err()
}

func (s *Store) MarkNotificationsRead(userID string, now time.Time) error {
	_, err := s.db.Exec(`UPDATE notifications SET read_at = ? WHERE user_id = ? AND read_at = 0`, now.Unix(), userID)
	return err
}

func (s *Store) UnreadNotifications(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at = 0`, userID).Scan(&count)
	return count, err
}

// ---------- profile stats ----------

type Stats struct {
	Posts        int `json:"posts"`
	SupportsOn   int `json:"supports_on_posts"`
	GivenSupport int `json:"supports_given"`
	Chats        int `json:"chats"`
	Unread       int `json:"unread_messages"`
	Saved        int `json:"saved"`
}

func (s *Store) Stats(userID string) (*Stats, error) {
	stats := &Stats{}
	err := s.db.QueryRow(
		`SELECT
			(SELECT COUNT(*) FROM posts WHERE user_id = ?),
			(SELECT COALESCE(SUM(supports), 0) FROM posts WHERE user_id = ?),
			(SELECT COUNT(*) FROM supports WHERE user_id = ?),
			(SELECT COUNT(*) FROM conversations WHERE (owner_id = ? OR writer_id = ?) AND status = 'open'),
			(SELECT COUNT(*) FROM saved WHERE user_id = ?)`,
		userID, userID, userID, userID, userID, userID,
	).Scan(&stats.Posts, &stats.SupportsOn, &stats.GivenSupport, &stats.Chats, &stats.Saved)
	if err != nil {
		return nil, err
	}
	stats.Unread, err = s.UnreadCount(userID)
	return stats, err
}

func (s *Store) UserPosts(userID string, limit int) ([]Post, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT `+postColumns+` FROM posts WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, *post)
	}
	return posts, rows.Err()
}

func (s *Store) IncrementChats(postID string) error {
	_, err := s.db.Exec(`UPDATE posts SET chats = chats + 1 WHERE id = ?`, postID)
	return err
}

func (s *Store) Cleanup(now time.Time) error {
	statements := []string{
		`DELETE FROM sessions WHERE expires_at <= ` + strconv.FormatInt(now.Unix(), 10),
		`DELETE FROM pending_signups WHERE expires_at <= ` + strconv.FormatInt(now.Unix(), 10),
		// Chat history only lives for 24h.
		`DELETE FROM messages WHERE created_at <= ` + strconv.FormatInt(now.Add(-24*time.Hour).Unix(), 10),
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
