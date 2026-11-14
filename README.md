# Backend Auth Service

A small, self-contained authentication API written in Go. It handles user registration, bcrypt password hashing, JWT login and role-based access control on top of SQLite. A browser-based API tester ships inside the binary, so you can try every endpoint without curl.

## Features

- **Signup and login.** Emails are normalised and validated, passwords must be 8–72 bytes, and duplicate emails are rejected case-insensitively.
- **bcrypt password hashing.** Logins for unknown emails take about as long as logins with a wrong password, so response time doesn't reveal which emails are registered.
- **JWT access tokens** (HS256). The signing key comes from the environment. Tokens expire after a configurable lifetime, and the server only accepts HS256 tokens that carry an expiry.
- **Role-based access control.** There are two roles, `user` and `admin`. Signup always creates a `user`. The only way to get an admin is the bootstrap account set through environment variables.
- **Defaults suitable for production:** request body size limits, server timeouts, graceful shutdown, structured logging, panic recovery, security headers (CSP, `nosniff`, frame denial), an opt-in CORS allow-list and a `/healthz` endpoint.
- **Embedded API tester** at `/`. It works on desktop and mobile, supports keyboard navigation, and escapes all server data before displaying it.

## Tech stack

| Concern          | Choice                                                     |
| ---------------- | ---------------------------------------------------------- |
| Language         | Go 1.26                                                    |
| HTTP             | `net/http` standard library (method-aware `ServeMux`)      |
| Database         | SQLite via `github.com/mattn/go-sqlite3` (cgo)             |
| Tokens           | `github.com/golang-jwt/jwt/v5`                             |
| Password hashing | `golang.org/x/crypto/bcrypt`                               |
| Logging          | `log/slog`                                                 |
| Lint / format    | golangci-lint v2 (gofmt, goimports, gosec, revive, …)      |

## Architecture

```
            ┌───────────────────────── internal/api ──────────────────────────┐
request ──▶ │ logRequests ─▶ securityHeaders ─▶ cors ─▶ ServeMux               │
            │                                         ├─ POST /signup          │
            │                                         ├─ POST /login           │
            │                                         ├─ GET  /profile  [auth] │
            │                                         ├─ GET  /users    [auth+admin]
            │                                         ├─ GET  /healthz         │
            │                                         └─ GET  /  (web/static)  │
            └───────────────┬───────────────────────────────┬─────────────────┘
                            │                               │
                 internal/auth                       internal/store
          (bcrypt, JWT TokenManager)              (SQLite users table)
```

- `main.go` loads config, opens the store, seeds the optional admin, starts the HTTP server and shuts it down cleanly on SIGINT/SIGTERM.
- Handlers depend on a small `UserStore` interface plus a `TokenManager`. Nothing is global, which keeps the handlers easy to test.

## Getting started

### Prerequisites

- Go 1.26+
- A C toolchain (gcc or clang), because `go-sqlite3` uses cgo

### Run locally

```bash
git clone https://github.com/jeevithg090/Task-Job.git
cd Task-Job

export JWT_SECRET="$(openssl rand -base64 48)"
export ADMIN_EMAIL=admin@example.com      # optional
export ADMIN_PASSWORD='change-me-please'  # required if ADMIN_EMAIL is set

go run .
```

Open <http://localhost:8080> to use the API tester.

### Environment variables

All configuration is read from the process environment. See [`.env.example`](.env.example).

| Variable               | Required | Default          | Description                                                                 |
| ---------------------- | -------- | ---------------- | --------------------------------------------------------------------------- |
| `JWT_SECRET`           | yes      | —                | HMAC signing key, at least 32 bytes. The server refuses to start without it. |
| `PORT`                 | no       | `8080`           | Listen port.                                                                |
| `DATABASE_PATH`        | no       | `./users.db`     | SQLite file path. The file is created if it doesn't exist.                  |
| `JWT_TTL`              | no       | `24h`            | Token lifetime as a Go duration (`30m`, `12h`, …).                           |
| `CORS_ALLOWED_ORIGINS` | no       | *(empty)*        | Comma-separated allowed origins. Empty means same-origin only; `*` allows any origin. |
| `ADMIN_EMAIL`          | no       | *(empty)*        | If set, an admin with this email is created on startup if it doesn't exist. |
| `ADMIN_PASSWORD`       | if admin | —                | Password for the bootstrap admin (8–72 bytes).                              |
| `ADMIN_NAME`           | no       | `Administrator`  | Display name for the bootstrap admin.                                       |

## API

Every response body is JSON. Errors look like `{"error": "message"}`.

| Method | Path       | Auth          | Description                                   |
| ------ | ---------- | ------------- | --------------------------------------------- |
| POST   | `/signup`  | —             | Register a `user` account                     |
| POST   | `/login`   | —             | Exchange email + password for a JWT           |
| GET    | `/profile` | Bearer token  | The authenticated user's profile              |
| GET    | `/users`   | Bearer, admin | List all users                                |
| GET    | `/healthz` | —             | Liveness check that also pings the database   |

### Examples

```bash
# Sign up (201). Any "role" other than "user" is rejected with 403.
curl -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"password123"}'

# Log in (200)
curl -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"password123"}'
# → {"token":"<jwt>","token_type":"Bearer","expires_at":"2026-01-02T15:04:05Z"}

# Profile
curl http://localhost:8080/profile -H "Authorization: Bearer <jwt>"

# All users (admin token required, otherwise 403)
curl http://localhost:8080/users -H "Authorization: Bearer <admin-jwt>"
```

### Status codes

| Code | When                                                                  |
| ---- | --------------------------------------------------------------------- |
| 400  | Malformed JSON, body over 1 MiB, missing or invalid fields            |
| 401  | Bad credentials, or a missing, invalid or expired token               |
| 403  | Non-admin calling `/users`, or signup asking for a non-`user` role    |
| 404  | Profile requested for a user that no longer exists                    |
| 405  | Wrong HTTP method                                                     |
| 409  | Email already registered                                              |

## Development

```bash
make test    # go test -race ./...
make lint    # golangci-lint run
make fmt     # gofmt + goimports via golangci-lint
make build   # bin/backend-auth
make vuln    # govulncheck
```

Tests cover config parsing, password hashing and token validation (expired tokens, tampering, a wrong key, `alg: none`, algorithm swaps and missing expiry), the SQLite store, and the HTTP API end to end against a temporary database.

CI (`.github/workflows/ci.yml`) runs on pull requests and on pushes to `main`. It checks that `go.mod` is tidy, then runs the linter, the race-enabled tests, the build, `govulncheck` and a Docker image build.

## Deployment

The service is one binary that serves the API and the tester. Its only state is the SQLite file.

### Docker

```bash
docker build -t backend-auth .
docker run -p 8080:8080 \
  -e JWT_SECRET="$(openssl rand -base64 48)" \
  -e ADMIN_EMAIL=admin@example.com -e ADMIN_PASSWORD='change-me-please' \
  -v backend-auth-data:/data \
  backend-auth
```

The image runs as a non-root user on a distroless base and stores the database at `/data/users.db`. Mount a volume there to keep data across restarts.

### Production notes

- Generate a fresh `JWT_SECRET` per environment and keep it in your platform's secret store. Rotating it invalidates every issued token.
- Run behind a TLS-terminating reverse proxy or load balancer. Tokens are bearer credentials and must not travel over plain HTTP.
- SQLite runs in WAL mode with a busy timeout. That's fine for a single instance. If you need several replicas, move to a networked database.
- Point health checks at `GET /healthz`.

## Project structure

```
.
├── main.go                    # Wiring, HTTP server, graceful shutdown
├── internal/
│   ├── api/                   # Routing, middleware, handlers, validation, admin bootstrap
│   ├── auth/                  # bcrypt helpers and JWT TokenManager
│   ├── config/                # Environment-based configuration
│   └── store/                 # SQLite user repository
├── web/
│   ├── web.go                 # go:embed for the static tester
│   └── static/index.html      # Browser API tester
├── .github/workflows/ci.yml   # Lint, test, build, vuln scan, Docker build
├── .golangci.yml
├── .env.example
├── Dockerfile
└── Makefile
```
