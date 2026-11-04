# içini dök.

> An anonymous, passwordless support wall. Share what you can't say out loud.

A monorepo that serves a static Next.js frontend and a Go API from a single
origin. Accounts are fully anonymous — **no email, no phone number, no
password**. Identity is a username plus a TOTP second factor, and users are
issued one-time recovery codes at signup.

---

## Features

- **Anonymous accounts** — username only, enforced TOTP (2FA) on every login.
  No personal data is collected.
- **Recovery codes** — 8 single-use codes generated at signup, stored as
  SHA-256 hashes, so a lost authenticator is not a lost account.
- **Support wall** — pseudonymous posts with categories, "support" and "save"
  toggles, and server-owned sorting options.
- **Ephemeral 1:1 chat** — conversations opened from a post, auto-expiring
  after 24 hours. Messages are throttled to one per sender every 10 seconds
  and purged from the database after 24 hours.
- **Notifications** — unread badges for messages, supports and chat activity.
- **Rate limiting everywhere** — per-IP and per-user token buckets on every
  write endpoint.
- **Docker-first** — a single multi-stage image builds the frontend and the
  API and runs them behind one port.
- **Turkish UI** — all user-facing copy is in Turkish; the codebase and docs
  are in English.

## Architecture

```
apps/web        Next.js (static export → out/)
services/api    Go API + embedded SQLite + static file server
Dockerfile      multi-stage image: web build + API build + runtime
docker-compose.yml
Makefile
```

The Next.js app is exported to static HTML. In production the Go server serves
that directory **and** the JSON API on the same origin, which keeps session
cookie handling simple and avoids CORS entirely.

```
Browser ──► Go server (:8080)
             ├── /api/*   JSON API (SQLite)
             └── /*       static frontend (out/)
```

### Stack

| Layer     | Choice                                            |
| --------- | ------------------------------------------------- |
| Frontend  | Next.js 16 (App Router, `output: "export"`), React 19 |
| Backend   | Go 1.26, standard library `net/http`              |
| Database  | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| 2FA       | `github.com/pquerna/otp` (TOTP)                   |
| QR codes  | `github.com/skip2/go-qrcode`                      |

## Getting started

### With Docker (recommended)

```sh
cp .env.example .env
docker compose up --build
# open http://localhost:8080
```

Behind HTTPS, set `APP_ORIGIN=https://example.com` and `COOKIE_SECURE=true`.
Application data lives in the `icinidok-data` volume (`/data/icinidok.db`).

### Local development

Run two terminals:

```sh
# 1) API on http://localhost:8080
cd services/api
go run ./cmd/server

# 2) Frontend on http://localhost:3000 (talks to the API on :8080)
cd apps/web
npm ci
npm run dev
```

Or use the shortcuts in the `Makefile`:

```sh
make dev-api    # Go API on :8080
make dev-web    # Next.js on :3000
make test       # Go test suite
make docker     # build the image
make up         # docker compose up --build -d
make down       # docker compose down
```

## Configuration

All configuration is environment-driven (see `.env.example`):

| Variable        | Default                                          | Description                                             |
| --------------- | ------------------------------------------------ | ------------------------------------------------------- |
| `ADDR`          | `:8080`                                          | Listen address.                                          |
| `DB_PATH`       | `./data/icinidok.db`                             | SQLite database file.                                    |
| `WEB_DIR`       | `./web`                                          | Directory with the exported frontend.                    |
| `APP_ORIGIN`    | `http://localhost:8080,http://localhost:3000`    | Comma-separated allowed origins.                         |
| `COOKIE_SECURE` | `false`                                          | Set `true` when served over HTTPS.                       |
| `TRUST_PROXY`   | `false`                                          | Trust `X-Forwarded-*` and same-host `Origin` behind a proxy/tunnel. |
| `TOTP_ISSUER`   | `içini dök.`                                     | Issuer name shown in the authenticator app.              |

## API

| Action                | Endpoint                                | Notes                                                    |
| --------------------- | --------------------------------------- | -------------------------------------------------------- |
| Start signup          | `POST /api/auth/signup/start`           | username → QR + TOTP secret (valid 15 min)               |
| Confirm signup        | `POST /api/auth/signup/confirm`         | TOTP code → session + 8 recovery codes                   |
| Log in                | `POST /api/auth/login`                  | username + TOTP or recovery code                         |
| Log out               | `POST /api/auth/logout`                 | clears the session cookie                                |
| Session               | `GET /api/auth/me`                      | current user or 401                                      |
| Wall                  | `GET /api/posts`, `GET /api/posts/{id}` | detail includes `mine` / `supported` / `saved` / `conversation_id` |
| Meta                  | `GET /api/meta`                         | category list + wall sort options (frontend fetches these) |
| Create post           | `POST /api/posts`                       | requires login; 4 per day, ≥10 min apart                 |
| Support / unsupport   | `POST /api/posts/{id}/support`          | toggle, counter updates                                  |
| Save / unsave         | `POST /api/posts/{id}/save`             | toggle                                                   |
| Saved posts           | `GET /api/me/saved`                     |                                                          |
| My posts              | `GET /api/me/posts`                     |                                                          |
| Stats                 | `GET /api/me/stats`                     | counters + remaining recovery codes                      |
| Rotate recovery codes | `POST /api/me/recovery-codes`           | requires TOTP / recovery confirmation                    |
| Summary               | `GET /api/summary`                      | unread message and notification counts                   |
| Conversations         | `GET/POST /api/conversations`           | list / start                                             |
| Conversation          | `GET /api/conversations/{id}`           | `?after=` for incremental fetch                          |
| Send message          | `POST /api/conversations/{id}/messages` |                                                          |
| Close conversation    | `POST /api/conversations/{id}/close`    | closed chats reject writes                               |
| Notifications         | `GET /api/notifications`, `POST /api/notifications/read` |                                      |

### Chat rules

- At least **10 seconds between messages** per sender.
- Messages disappear from history and are purged **24 hours** after sending.
- Closing a conversation locks it for **both** participants.
- Posting: **4 posts per rolling 24 hours**, at least **10 minutes apart**.

## Security

- HttpOnly, `SameSite=Lax` session cookies (marked `Secure` over HTTPS).
- TOTP replay protection: the last accepted time step is stored per user.
- Recovery codes are single-use and stored only as SHA-256 hashes.
- Per-endpoint token-bucket rate limiting (auth, signup, login, posts,
  messages, reads, polls) keyed by IP and/or user.
- `Origin` header validation; stricter when not behind a trusted proxy.
- Content-Security-Policy and standard hardening headers.
- Single-writer SQLite with WAL — no concurrent-write corruption.
- Non-root runtime user inside the container.

## Testing

```sh
cd services/api
go vet ./...
go test -race ./...

# end-to-end smoke test against a running server
sh ../../scripts/smoke.sh 8080
```

## Project layout

```
apps/web/
  app/            routes (home, paylas, gelen, bildirimler, ben, hakkinda, …)
  components/     shared UI (Frame, HomeWall, GelenClient, …)
  lib/            API client, session and summary hooks
  public/fonts/   self-hosted Outfit webfont
services/api/
  cmd/server/     entrypoint
  internal/app/   config, store, auth, rate limiting, HTTP server, tests
scripts/smoke.sh  black-box end-to-end check
```

## License

[MIT](LICENSE)
