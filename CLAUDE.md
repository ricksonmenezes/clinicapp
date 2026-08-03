# clinicapp — Operating Guide

> This file is HOW to run, deploy, and test this repo.
> For WHAT clinicapp is and WHY decisions were made, read `PLAN.md`.
> For current status and what's done/pending, read `PROGRESS.md`.

---

## 1. What this repo is

A clinic scheduling and management platform: Go backend + HTMX backoffice + customer-facing
web portal. One backend serves both web (HTML fragments) and mobile (JSON) from the same
endpoints, distinguished by the `X-Client-Type` header. See `PLAN.md` for full scope.

---

## 2. Local dev startup

```bash
# 1. Copy env and fill in secrets
cp .env.example backend/.env

# 2. Start the backend
cd backend && go run ./cmd/server

# 3. Run backend unit tests
cd backend && go test ./...
```

The server starts on `localhost:8080` by default (`PORT` env overrides).
`GET /healthz` → `{"status":"ok"}` confirms it's running.

---

## 3. Environment variable reference

| Var | Meaning | Local | Prod |
|---|---|---|---|
| `APP_ENV` | `local` or `prod` | `local` | `prod` |
| `PORT` | HTTP listen port | `8080` | `8080` |
| `DB_DSN` | Postgres connection string | `postgres://...` (local DB) | managed secret |
| `JWT_SECRET` | HMAC signing key for access tokens | generate locally | managed secret |
| `JWT_EXPIRY_MINUTES` | Access token TTL | `15` | `15` |
| `REFRESH_TOKEN_EXPIRY_DAYS` | Refresh token TTL | `30` | `30` |
| `SMTP_HOST` | Outbound mail server host | mailtrap or similar | real SMTP |
| `SMTP_PORT` | SMTP port | `587` | `587` |
| `SMTP_USER` | SMTP username | local creds | managed secret |
| `SMTP_PASS` | SMTP password | local creds | managed secret |
| `SMTP_FROM` | From address on outbound emails | `noreply@clinic.local` | real address |
| `SMS_PROVIDER` | SMS provider name (TBD) | — | TBD |
| `SMS_API_KEY` | SMS provider API key | — | managed secret |
| `BASE_URL` | Public base URL (used in email links) | `http://localhost:8080` | `https://<domain>` |

Never commit `backend/.env` to git.

---

## 4. Project structure

```
clinicapp/
├── CLAUDE.md              ← this file
├── PLAN.md                ← product plan
├── PROGRESS.md            ← living milestone log
├── .env.example           ← template for backend/.env
├── .gitignore
│
├── backend/
│   ├── go.mod / go.sum
│   ├── .env               ← gitignored
│   ├── bin/               ← gitignored (built binary)
│   ├── cmd/server/main.go
│   └── internal/
│       ├── config/        ← env loading, profile switching
│       ├── middleware/    ← auth (JWT), client-type, rate limit, CORS
│       ├── renderer/      ← web (HTML fragment) vs mobile (JSON) response dispatch
│       ├── auth/          ← handlers, service, repository
│       ├── patient/
│       ├── consultant/
│       ├── attendant/
│       ├── service/
│       ├── session/
│       ├── invoice/
│       ├── prescription/
│       ├── mailer/        ← SMTP interface + implementation
│       ├── sms/           ← SMS interface (provider plugged in when confirmed)
│       └── store/         ← DB pool, migration runner
│
├── web/
│   ├── templates/         ← Go HTML templates (HTMX fragments + full pages)
│   └── static/            ← CSS, minimal JS, images
│
├── migrations/            ← numbered SQL files (001_create_users.sql, etc.)
├── deploy/
│   ├── clinicapp.service  ← systemd unit
│   └── Caddyfile
└── scripts/
    └── deploy.sh
```

---

## 5. Database

- **Engine**: PostgreSQL
- **Migrations**: plain numbered SQL files in `migrations/`, applied at startup via an embedded
  migration runner in `internal/store`. A `schema_migrations` table tracks applied migrations.
- **Local setup**: create a local Postgres DB, set `DB_DSN` in `backend/.env`.

To reset your local DB and re-apply all migrations from scratch:
```bash
dropdb clinicapp_dev && createdb clinicapp_dev
cd backend && go run ./cmd/server  # migrations auto-apply on boot
```

---

## 6. Client-type detection

All endpoints are shared between web and mobile. The backend reads `X-Client-Type`:

| Header value | Response format |
|---|---|
| `web` (or absent) | HTMX-compatible HTML fragment |
| `mobile` | JSON |

Auth tokens:
- **Web**: access token + refresh token in httpOnly cookies (set by the server).
- **Mobile**: tokens returned in the JSON response body; client stores them.

---

## 7. Deployment

**SSH**: `ssh <your-server>` (configure in `~/.ssh/config`).

**Deploy one-liner** (after pushing to GitHub):
```bash
ssh <server> 'cd /opt/clinicapp && git pull && cd backend && go build -o bin/clinicapp-server ./cmd/server && systemctl restart clinicapp'
```

Then verify:
```bash
curl https://<domain>/healthz
```

`scripts/deploy.sh` wraps this one-liner.

**Caddy** (reverse proxy):
```
<domain> {
    reverse_proxy localhost:8080
}
```
TLS is handled by Cloudflare's orange proxy — Caddy listens on port 80 only. Do not let Caddy
manage its own HTTPS (ACME) for this setup — Cloudflare handles TLS termination.

**systemd** (`deploy/clinicapp.service`):
- `User=clinicapp`
- `EnvironmentFile=/opt/clinicapp/.env`
- `Restart=on-failure`

---

## 8. Running tests

```bash
# Backend unit tests
cd backend && go test ./...

# (Future) integration / e2e tests — added when scaffolded
```

All tests must be green before any push to main.

### Guardrails — tests run automatically on every change

**Local (pre-push hook)**
Install once after cloning:
```bash
bash scripts/install-hooks.sh
```
This installs a `pre-push` git hook that runs `go test ./...` before every push.
A failing test aborts the push — nothing broken reaches GitHub.

**CI (GitHub Actions)**
Every push and pull request triggers `.github/workflows/ci.yml` which:
- Starts a Postgres service container (`clinicapp_test` DB, migrations applied)
- Starts a Mailpit service container (SMTP sink for email flow tests)
- Runs `go test ./...`
- Fails the build and blocks merging if any test is red

Merges to main are blocked unless CI is green. This is the hard enforcement —
the pre-push hook is a convenience to catch failures before they hit CI.

---

## Hard Constraints

### Security
- **Never** commit `backend/.env` or any file containing real secrets to git.
- **Never** hardcode `JWT_SECRET`, `SMTP_PASS`, `SMS_API_KEY`, or any credential in source code.
- **Never** log plaintext passwords or raw tokens anywhere.
- Refresh tokens are UUIDs stored in the DB — revoke them individually on logout and in bulk on password reset.
- Password hashing: bcrypt, cost 12 minimum.

### Server
- Do not SSH into the production server to develop or debug — it is a deployment target only.
- Never build or test on the prod server. Build locally, push to GitHub, deploy via the one-liner.

### Git
- Never make one giant commit at the end of a milestone — commit incrementally as logical units land.
- Commit immediately after each logical unit of work without asking; summarise what was committed in your response.
- Never push to main with failing tests.
- If `git push` fails on auth, stop and ask — do not attempt to fix credentials silently.

### Workflow
- Default to building an entire milestone in one pass without pausing for per-piece review.
- Commit incrementally as pieces land and report back once the whole milestone is done.
- Only stop mid-milestone if genuinely blocked: missing decision, missing credential, or a failing test that needs a design call.
- If any single approach fails 3–5 times in a row, stop and report back: explain what was tried, what's failing, and ask whether a different approach or manual intervention is needed.

### API design
- **Do not** add a separate mobile vs. web API surface. One endpoint, one handler, the `renderer` package dispatches the response format.
- **Do not** break the `X-Client-Type` contract — mobile clients depend on JSON responses.
- Commission rates: always snapshot the rate at session time. Never recalculate historical commissions from current config.

### PDF / invoices
- Invoice template placeholders live in the DB — never hardcode clinic name, address, or footer text in templates.
- Commission history must show the `resolution_source` (session override / service override / consultant default) alongside the rate.

### Testing
- Never mark a milestone complete unless `go test ./...` is green.
- Never push to main with a failing CI build.
- Every milestone must add its integration test file before the milestone is closed —
  tests are not a follow-up task, they ship with the code.
- Do not skip the pre-push hook (`--no-verify`) without explicit instruction.

---

## 9. Known gotchas

- **httpOnly cookies on mobile**: mobile clients cannot read httpOnly cookies set by the server.
  The `renderer` always returns tokens in the JSON body for `X-Client-Type: mobile` requests —
  do not set cookies in that path.
- **Refresh token rotation**: after a refresh, the old refresh token is revoked and a new one is
  issued. Mobile clients must update their stored token on every refresh response.
- **Email verification rate limit**: max 3 resend-verification emails per hour per email address.
  Enforce in middleware, not in the handler.
- **Commission resolution is per-session, not per-package**: even if a patient_package has a
  principal consultant, each session in that package independently resolves the commission for
  whoever actually performed it.
- **Postgres UUIDs**: use `gen_random_uuid()` (Postgres 13+) or the `uuid-ossp` extension.
  Set this as the default in migrations, not in Go code.
- **Migration order matters**: never edit a migration file after it has been applied to any
  environment. Add a new migration file instead.

---

## 10. Current phase

See `PROGRESS.md` for the live milestone checklist and pending items.
