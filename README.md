<div align="center">

<img src="./assets/logo.svg" width="88" height="88" alt="içini dök." />

# içini dök.

**An anonymous support wall. Share what you can't say out loud.**

No email. No phone number. No password. Your whole account is a username and a
six-digit code from an authenticator app — and a place for the feelings that
have nowhere else to go.

[![License](https://img.shields.io/badge/license-MIT-0A0A0A?style=flat-square)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-0A0A0A?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16-0A0A0A?style=flat-square&logo=next.js&logoColor=white)](https://nextjs.org)
[![React](https://img.shields.io/badge/React-19-0A0A0A?style=flat-square&logo=react&logoColor=white)](https://react.dev)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-0A0A0A?style=flat-square&logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![Docker](https://img.shields.io/badge/Docker-one%20image-0A0A0A?style=flat-square&logo=docker&logoColor=white)](./Dockerfile)
[![2FA](https://img.shields.io/badge/2FA-TOTP-0A0A0A?style=flat-square)](#security)

[Features](#why-içini-dök) · [Screenshots](#screenshots) · [Quick start](#quick-start) · [API](#api) · [Security](#security) · [Layout](#repository-layout)

</div>

---

## Screenshots

> A support wall that starts empty — the posts below are seeded demo content for the shots only.

| Wall | Chat |
| --- | --- |
| ![Home wall](./assets/screenshots/home-desktop.png) | ![Conversation](./assets/screenshots/chat.png) |

| Dark mode | Create account (TOTP) | Mobile |
| --- | --- | --- |
| ![Dark mode](./assets/screenshots/home-desktop-dark.png) | ![Signup with QR](./assets/screenshots/signup-2fa.png) | ![Mobile wall](./assets/screenshots/home-mobile.png) |

## Why içini dök.

Most "anonymous" platforms still want your identity — an email here, a phone
number there, a password to leak later. This one asks for the least it possibly
can: a username and a second factor. Everything else is built to protect the
person behind the post.

- **Truly anonymous.** Username only. No email, phone, password or profile — nothing to leak, nothing to correlate.
- **Passwordless 2FA.** Signup shows a QR code, login requires a TOTP code or one of eight single-use recovery codes.
- **A quiet place to vent.** Post pseudonymously, browse by category, and offer anonymous support instead of advice.
- **Ephemeral by design.** Conversations last 24 hours. Messages are throttled and purged; nothing is kept "just in case".
- **Hardened by default.** Every write endpoint is rate limited, sessions are HttpOnly, and the whole thing runs as one non-root container.
- **One origin, zero CORS.** A static Next.js export and the Go API are served from the same port.

## Quick start

### Docker (recommended)

```bash
cp .env.example .env
docker compose up --build
# open http://localhost:8080
```

Behind HTTPS set `APP_ORIGIN=https://example.com` and `COOKIE_SECURE=true`.
Application data lives in the `icinidok-data` volume (`/data/icinidok.db`).

### Local development

Two terminals:

```bash
# 1) API on http://localhost:8080
cd services/api
go run ./cmd/server

# 2) Frontend on http://localhost:3000 (talks to the API on :8080)
cd apps/web
npm ci && npm run dev
```

Or use the `Makefile` shortcuts:

| Command | Description |
| --- | --- |
| `make dev-api` | Go API on `:8080` |
| `make dev-web` | Next.js on `:3000` |
| `make test` | Go test suite |
| `make docker` | Build the image |
| `make up` / `make down` | `docker compose up --build -d` / down |

## Configuration

All configuration is environment-driven (see `.env.example`):

| Variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | Listen address. |
| `DB_PATH` | `./data/icinidok.db` | SQLite database file. |
| `WEB_DIR` | `./web` | Directory with the exported frontend. |
| `APP_ORIGIN` | `http://localhost:8080,http://localhost:3000` | Comma-separated allowed origins. |
| `COOKIE_SECURE` | `false` | Set `true` when served over HTTPS. |
| `TRUST_PROXY` | `false` | Trust `X-Forwarded-*` and same-host `Origin` behind a proxy/tunnel. |
| `TOTP_ISSUER` | `içini dök.` | Issuer name shown in the authenticator app. |

## What's inside

The Go server serves the exported frontend **and** the JSON API on one origin,
which keeps cookie handling simple and removes CORS from the picture:

```
Browser ──► Go server (:8080)
             ├── /api/*   JSON API (SQLite)
             └── /*       static frontend (Next.js export)
```

- **Auth** — anonymous signup with a 15-minute pending window, TOTP verification, 8 hashed recovery codes, replay protection on the last accepted time step.
- **Wall** — pseudonymous posts with server-owned categories and sort options, support / save toggles, and per-user saved and posted lists.
- **Chat** — one conversation per post per person, expiry after 24h, a 10-second per-sender throttle, and a lock on close.
- **Notifications** — unread badges for messages, supports and chat activity, surfaced in the nav.
- **Meta-driven UI** — categories and wall sorts come from `GET /api/meta`; the frontend hardcodes no content.

### Posting and chat rules

| Rule | Value |
| --- | --- |
| Posts per rolling window | **4 / 24h** |
| Minimum gap between posts | **10 minutes** |
| Minimum gap between messages | **10 seconds** |
| Message lifetime | **24 hours**, then purged |
| Closed conversations | reject writes from both sides |

## API

| Action | Endpoint | Notes |
| --- | --- | --- |
| Start signup | `POST /api/auth/signup/start` | username → QR + TOTP secret (valid 15 min) |
| Confirm signup | `POST /api/auth/signup/confirm` | TOTP code → session + 8 recovery codes |
| Log in | `POST /api/auth/login` | username + TOTP or recovery code |
| Log out | `POST /api/auth/logout` | clears the session cookie |
| Session | `GET /api/auth/me` | current user or `401` |
| Wall | `GET /api/posts`, `GET /api/posts/{id}` | detail includes `mine` / `supported` / `saved` / `conversation_id` |
| Meta | `GET /api/meta` | category list + wall sort options |
| Create post | `POST /api/posts` | requires login |
| Support · Save | `POST /api/posts/{id}/support`, `POST /api/posts/{id}/save` | toggles |
| My data | `GET /api/me/saved`, `GET /api/me/posts`, `GET /api/me/stats` | saved, posted, counters |
| Rotate recovery codes | `POST /api/me/recovery-codes` | requires TOTP confirmation |
| Summary | `GET /api/summary` | unread message / notification counts |
| Conversations | `GET/POST /api/conversations` | list / start |
| Conversation | `GET /api/conversations/{id}` | `?after=` for incremental fetch |
| Send message | `POST /api/conversations/{id}/messages` | |
| Close conversation | `POST /api/conversations/{id}/close` | closed chats reject writes |
| Notifications | `GET /api/notifications`, `POST /api/notifications/read` | |

## Security

- HttpOnly, `SameSite=Lax` session cookies (marked `Secure` over HTTPS).
- TOTP replay protection — the last accepted time step is stored per user.
- Recovery codes are single-use and stored only as SHA-256 hashes.
- Per-endpoint token-bucket rate limiting on auth, signup, login, posts, messages, reads and polls, keyed by IP and/or user.
- `Origin` header validation, stricter when not behind a trusted proxy.
- Content-Security-Policy and standard hardening headers.
- Single-writer SQLite with WAL — no concurrent-write corruption.
- Non-root runtime user inside the container.

## Tech stack

| Layer | Choice |
| --- | --- |
| Frontend | Next.js 16 (App Router, static export), React 19 |
| Backend | Go 1.26, `net/http` standard library |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| 2FA | `github.com/pquerna/otp` (TOTP) |
| QR codes | `github.com/skip2/go-qrcode` |
| Packaging | Multi-stage Docker image, `docker compose` |

## Repository layout

```
icinidok/
├─ apps/web/                 Next.js static frontend (→ out/)
│  ├─ app/                   App Router routes (home, paylas, gelen, ben, …)
│  ├─ components/            Shared UI (Frame, HomeWall, GelenClient, …)
│  ├─ lib/                   API client, session and summary hooks
│  └─ public/                Self-hosted Outfit webfont + README screenshots
├─ services/api/             Go API + embedded SQLite + static file server
│  ├─ cmd/server/            Entrypoint
│  └─ internal/app/          config, store, auth, rate limiting, HTTP server, tests
├─ scripts/smoke.sh          Black-box end-to-end check
├─ Dockerfile                web build + API build + runtime
├─ docker-compose.yml
└─ Makefile
```

## Testing

```bash
cd services/api
go vet ./...
go test -race ./...

# end-to-end smoke test against a running server
sh ../../scripts/smoke.sh 8080
```

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| P0 | Monorepo, Go API, SQLite store, anonymous TOTP accounts | ✅ Done |
| P1 | Wall, categories, support / save, meta-driven UI | ✅ Done |
| P2 | Ephemeral chat, notifications, unread badges | ✅ Done |
| P3 | Rate limiting, security hardening, Docker packaging | ✅ Done |
| P4 | Chat throttle, 24h retention, closed-chat lock | ✅ Done |
| P5 | Screenshots, docs, public release | 🚧 In progress |

## Contributing

Issues and pull requests are welcome. Before opening a PR:

1. Keep the API and store changes covered by a Go test — the suite runs with `-race`.
2. Run `go vet ./...` and `go test ./...` inside `services/api`.
3. Keep all user-facing copy in Turkish; keep code and comments in English.
4. Never commit a database file, `.env`, or build output — `.gitignore` already covers them.

## License

Released under the [MIT License](./LICENSE).

<div align="center">

**[Quick start](#quick-start)** · **[API](#api)** · **[Security](#security)** · **[Screenshots](#screenshots)**

</div>
