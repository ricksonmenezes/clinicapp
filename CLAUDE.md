# clinicapp — Operating Guide

> This file is HOW to run, deploy, and test this repo.
> For WHAT clinicapp is and WHY decisions were made, read `PLAN.md`.
> For current status and what's done/pending, read `PROGRESS.md`.
> For gaps this plan left implicit that had to be resolved mid-build — and a checklist to avoid
> repeating them on the next project — read `ARCHITECTURE.md`.

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
| `PORT` | HTTP listen port | `8080` | `8081` (`8080` is taken by an unrelated service on the netcup box) |
| `DB_DSN` | Postgres connection string | `postgres://...` (local DB) | `postgres://clinicapp:...@localhost:5432/clinicapp_prod` (managed secret, in `/opt/clinicapp/.env` only) |
| `JWT_SECRET` | HMAC signing key for access tokens | generate locally | managed secret, in `/opt/clinicapp/.env` only |
| `JWT_EXPIRY_MINUTES` | Access token TTL | `15` | `15` |
| `REFRESH_TOKEN_EXPIRY_DAYS` | Refresh token TTL | `30` | `30` |
| `RESEND_API_KEY` | Resend API key | sandbox key from resend.com | same sandbox key reused for now (see §9) |
| `MAIL_FROM` | From address on outbound emails | `onboarding@resend.dev` (Resend sandbox address) | `noreply@clinic.ricksonmenezes.com` — verified sender domain in Resend (see §7) |
| `SMS_PROVIDER` | SMS provider name | `philsms` | `philsms` |
| `SMS_API_KEY` | PhilSMS API key | key from dashboard.philsms.com | managed secret |
| `SMS_SENDER_ID` | PhilSMS sender name on outbound SMS | `PhilSMS` (platform shared default) | `PhilSMS`, or the clinic's own registered sender ID once obtained |
| `BASE_URL` | Public base URL (used in email links) | `http://localhost:8080` | `https://<domain>` |
| `INVOICE_STORAGE_DIR` | Directory generated invoice PDFs are written to (relative to `backend/` unless absolute) | `data/invoices` | persistent path outside the deploy dir |
| `PRESCRIPTION_STORAGE_DIR` | Directory generated Rx PDFs are written to (relative to `backend/` unless absolute) | `data/prescriptions` | persistent path outside the deploy dir |
| `ADMIN_BOOTSTRAP_EMAIL` | Email for the auto-created first admin account (idempotent, runs on every boot) | optional, local testing only | set once, then unset |
| `ADMIN_BOOTSTRAP_PASSWORD` | Password for the bootstrap admin account | optional, local testing only | set once, then unset |

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
│       ├── booking/       ← customer-portal calendar availability + self-booking
│       ├── mailer/        ← Resend interface + implementation (api.resend.com/emails)
│       ├── sms/           ← SMS abstraction (PhilSMS implementation)
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
- **Migrations**: plain numbered SQL files in `migrations/`, applied at startup by the migration
  runner in `internal/store` (reads `../migrations` relative to the `backend/` working directory).
  A `schema_migrations` table tracks applied migrations.
- **Local setup**: create a local Postgres DB, set `DB_DSN` in `backend/.env`.

This machine has no Docker, so Postgres runs as a native Homebrew service instead of a container:
```bash
brew install postgresql@16
brew services start postgresql@16
createdb clinicapp_dev
createdb clinicapp_test   # used by backend/tests/integration
```

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

**Live**: `https://clinic.ricksonmenezes.com` — deployed to the netcup server (`ssh netcup`) as of
M9. Runs as systemd unit `clinicapp` under system user `clinicapp`, code at `/opt/clinicapp`.

**Port**: clinicapp listens on **`8081`**, not the `8080` shown in older docs/examples — the
netcup box already runs an unrelated service (`sanasalinin.service`) bound to `8080`. `PORT=8081`
is set in the server's `/opt/clinicapp/.env`. If you ever re-provision from scratch, check
`ss -tlnp` for port conflicts before picking a port, don't assume 8080 is free.

**SSH**: `ssh netcup` (`~/.ssh/config`, root@netcup server). Deployment target only — do not
develop or debug on it directly (see Hard Constraints below).

**Deploy one-liner** (after pushing to GitHub):
```bash
ssh netcup 'su clinicapp -s /bin/bash -c "cd /opt/clinicapp && git pull && cd backend && go build -o bin/clinicapp-server ./cmd/server" && systemctl restart clinicapp'
```
`git pull`/`go build` run as the `clinicapp` system user (which owns `/opt/clinicapp`), not as
root (the ssh user) — running them as root fails with git's "detected dubious ownership" check.
Only the final `systemctl restart` needs root.

Then verify:
```bash
curl https://clinic.ricksonmenezes.com/healthz
```

`scripts/deploy.sh netcup` wraps this one-liner.

**Caddy** (reverse proxy) — appended to the netcup box's existing `/etc/caddy/Caddyfile`
(which also serves unrelated sites on that box — never overwrite it wholesale):
```
http://clinic.ricksonmenezes.com {
    reverse_proxy localhost:8081
}
```
TLS is handled by Cloudflare's orange proxy — Caddy listens on port 80 only (explicit `http://`
scheme in the site address, matching the box's other sites, so Caddy never attempts ACME/its own
HTTPS). `deploy/Caddyfile` in this repo is a template/reference for a single-site box — the
actual live config lives only on the server and also has other, unrelated site blocks.

**systemd** (`deploy/clinicapp.service`):
- `User=clinicapp`
- `EnvironmentFile=/opt/clinicapp/.env`
- `Restart=on-failure`

`clinic.ricksonmenezes.com` is a verified sender domain in Resend (added under Resend → Domains,
DNS records added to Cloudflare, 2026-08-04) — confirmed working end-to-end via a real patient
registration on prod. All API routes, including every email-sending endpoint (`register`,
`resend-verification`, `forgot-password`, `reset-password`, booking confirmation), are live and
verified working.

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
- Runs `go test ./...`
- Fails the build and blocks merging if any test is red

Merges to main are blocked unless CI is green. This is the hard enforcement —
the pre-push hook is a convenience to catch failures before they hit CI.

---

## Hard Constraints

### Security
- **Never** commit `backend/.env` or any file containing real secrets to git.
- **Never** hardcode `JWT_SECRET`, `RESEND_API_KEY`, `SMS_API_KEY`, or any credential in source code.
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
- After any milestone is complete and `go test ./...` is green, push to `origin/main`
  immediately without asking, before starting the next milestone.
- If `git push` fails on auth, stop and ask — do not attempt to fix credentials silently.

### Workflow
- When asked to check/look at an image with no path given, look in
  `/Users/ricksonmenezes/Documents/Screenshots` — screenshots the user takes are saved there by
  default. Prefer the most recently modified file unless the name/context points to a different one.
- Default to building an entire milestone in one pass without pausing for per-piece review.
- Commit incrementally as pieces land and report back once the whole milestone is done.
- Only stop mid-milestone if genuinely blocked: missing decision, missing credential, or a failing test that needs a design call.
- If any single approach fails 3–5 times in a row, stop and report back: explain what was tried, what's failing, and ask whether a different approach or manual intervention is needed.
- Whenever an unclear point forces a real architectural/technical design decision — a third-party
  provider's exact endpoint or auth shape, a sending-domain choice, a secrets-handling approach, a
  port/infra conflict discovered on a deploy target, a capacity/authorship rule the plan left
  implicit, anything that took asking the user or a judgment call to resolve — record it as a
  dated technical note in `ARCHITECTURE.md`, not just in `PROGRESS.md`'s decision log. The point
  is for `ARCHITECTURE.md` to accumulate into a standing checklist so the *next* project's
  planning docs address these categories from the outset instead of surfacing as incremental
  interruptions mid-build. `PROGRESS.md` stays the project-specific "what happened and why";
  `ARCHITECTURE.md` is the reusable "what to decide up front next time."

### API design
- **Do not** add a separate mobile vs. web API surface. One endpoint, one handler, the `renderer` package dispatches the response format.
- **Do not** break the `X-Client-Type` contract — mobile clients depend on JSON responses.
- Commission rates: always snapshot the rate at session time. Never recalculate historical commissions from current config.

### PDF / invoices
- Invoice template placeholders live in the DB — never hardcode clinic name, address, or footer text in templates.
- Commission history must show the `resolution_source` (session override / service override / consultant default) alongside the rate.

### Email sending guardrails (Resend free tier)
Resend's free tier caps outbound email at **100/day** and **3,000/month**, account-wide — this
ceiling is shared across every user and IP, not per-recipient like the existing per-email
resend-verification limit (§9). Enforce a **global** guardrail in middleware, applied to every
endpoint that sends an email (`register`, `resend-verification`, `forgot-password`,
`reset-password` — it sends a "password changed" confirmation email):
- More than **5** email-triggering requests in a rolling minute → stop servicing further requests.
- More than **20** in a rolling hour → stop servicing further requests.
- Hitting **100** in a day → stop servicing further requests entirely. Resume only once a full
  24 hours have elapsed since the 100th email was sent — a rolling cooldown keyed off that
  request, not a reset at midnight.
This is additive to, not a replacement for, the per-email-address resend-verification limit.

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
- **Role assignment is not public**: `POST /auth/register` always creates a `patient` account —
  it ignores/rejects any role in the request body. Elevated accounts (`clinician`, `attendant`,
  `admin`) can only be created via `POST /auth/register-staff`, which requires an authenticated
  `admin` bearer token. The very first admin comes from `ADMIN_BOOTSTRAP_EMAIL` /
  `ADMIN_BOOTSTRAP_PASSWORD` (idempotent — safe to leave set, only creates the account once).
- **Resend sender domain**: the `MAIL_FROM` address must use a domain you have verified
  in the Resend dashboard (resend.com → Domains). Using an unverified domain will cause
  all emails to silently fail. During local dev you can use the Resend test address
  `onboarding@resend.dev` (only delivers to your Resend account's own email).
- **Email verification testing locally**: the Resend sandbox never delivers to a real inbox —
  emails are visible only in the Resend dashboard (resend.com → Emails tab). To test the
  verify-email flow locally, copy the token directly from your DB or server logs and call
  `GET /auth/verify-email?token=<that-token>` manually (curl or browser). Do not set up
  Mailpit — it is no longer in the stack.
- **SMS provider (M8, implemented)**: PhilSMS. `internal/sms.PhilSMSSender` sends via
  `POST https://app.philsms.com/api/v3/sms/send` (the real endpoint per PhilSMS's developer
  docs — note this is `app.philsms.com`, not the `dashboard.philsms.com` host used for logging
  into the PhilSMS dashboard itself), `Authorization: Bearer <SMS_API_KEY>`, body
  `{recipient, sender_id, type: "plain", message}`. `patients.phone` is free text with no format
  validation at write time, so `sms.NormalizePHPhone` converts the common local forms
  (`+639...`, `639...`, `09...`, bare `9...`) to the `63XXXXXXXXXX` shape the API expects before
  sending. SMS confirmation on booking is best-effort — unlike the email confirmation, a missing
  phone number or a PhilSMS send failure is logged and does not fail the booking, since phone
  (unlike email) is optional and unverified.
- **Production deploy (M9, live)**: see §7 for the full picture. Two things not obvious from the
  code: (1) the netcup box hosts other, unrelated sites/services (a Hugo static site, and a
  separate `sanasalinin.service` already on port `8080`) — clinicapp runs on `8081` and its Caddy
  block was *appended* to the shared `/etc/caddy/Caddyfile`, never generated fresh from
  `deploy/Caddyfile`. (2) `MAIL_FROM`'s domain isn't verified in Resend yet, so every
  email-blocking endpoint 500s on prod until that's done — everything else is live and working.

---

## 10. Current phase

See `PROGRESS.md` for the live milestone checklist and pending items.

---

## 11. Resuming work ("proceed", "continue", etc.)

If asked to "proceed" or "continue" with no other context (including right after `/clear` or a
fresh session start), never ask what to proceed with — always check `PROGRESS.md` first for the
current milestone and pending items, then `PLAN.md` if `PROGRESS.md` is ambiguous, and start on
the next actionable item. Only ask a question if both files leave genuine ambiguity (e.g., two
equally-valid next milestones).
