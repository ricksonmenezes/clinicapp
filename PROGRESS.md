# clinicapp — Progress Log

Living status doc. Update this as work happens so a killed/resumed session can pick up exactly
where it left off. See `PLAN.md` for the full product plan and `CLAUDE.md` for operating
instructions.

---

## Milestone checklist

- [x] **M0** — Repo skeleton: directory structure, `PLAN.md`, `CLAUDE.md`, `PROGRESS.md`,
      `.gitignore`, `.env.example`, `go.mod`. GitHub repo created and pushed.
- [x] **M1** — Auth module: DB migrations (users + token tables + user_providers stub), all
      auth endpoints (register, verify-email, resend-verification, login, refresh, logout,
      forgot-password, reset-password), mailer abstraction (SMTP), middleware (auth JWT,
      client-type, rate limit), renderer (web HTML fragment vs mobile JSON).
      Integration test harness scaffolded (testserver, testdb, FakeMailer, helpers).
      Full register → verify-email → login → protected-endpoint e2e test passing.
      `go test ./...` green with zero manual steps.
- [ ] **M2** — Patient & consultant management: patient profile CRUD, consultant profile CRUD,
      global commission config, per-service commission override, attendant profiles.
      Backoffice HTMX pages for each. Tests green.
- [ ] **M3** — Services & packages: service CRUD (price, requires_consultant flag), package/promo
      definition (N sessions, package price, principal consultant), patient package subscription.
      Backoffice pages. Tests green.
- [ ] **M4** — Session management: session recording (patient, service, consultant nullable,
      attendants), commission resolution logic (session override → service override → consultant
      default), commission snapshot storage, session history per patient and per consultant.
      Tests green.
- [ ] **M5** — Invoicing & PDF: invoice generation per session and per package, server-side PDF
      output, admin placeholder editor (clinic name, address, footer etc.), commission history
      view per consultant showing rate + resolution source per session. Tests green.
- [ ] **M6** — Prescriptions (Rx): Rx authoring UI for consultants, printable PDF output.
      Tests green.
- [ ] **M7** — Customer-facing portal: patient self-register/login (same auth backend), service
      browse, calendar slot availability view, booking flow, email confirmation on booking.
      Tests green.
- [ ] **M8** — SMS notifications: integrate SMS provider (TBD — awaiting user confirmation of
      provider), send SMS on booking confirmation.
- [ ] **M9** — Production deploy: server provisioned, systemd service installed, Caddy configured,
      Cloudflare orange proxy active (TLS), end-to-end booking flow verified on prod.
      `curl https://<domain>/healthz` → ok.

---

## Pending / deferred items

- SMS provider: user to confirm which provider (Twilio, Vonage, local Philippine SMS gateway, etc.)
  before M8 can start. Interface is abstracted in `internal/sms/` — implementation slots in when
  confirmed.
- Phase 2: Google / Apple OAuth (user_providers table scaffolded in M1, unused until Phase 2).
- Phase 2: mobile app (React Native or Flutter) consuming the same Go API.
- Phase 2: reporting / analytics dashboard.
- Phase 2: multi-clinic / multi-branch support.

---

## Decision log

- **2026-08-03** (M1): Auth module complete. `internal/config`, `internal/store` (migration
  runner against numbered SQL files in `migrations/`), `internal/mailer` (SMTP, Mailpit-
  compatible), `internal/renderer` (X-Client-Type dispatch), `internal/middleware`
  (client-type, JWT auth, per-IP rate limiting), `internal/auth` (register, verify-email,
  resend-verification, login, refresh, logout, forgot-password, reset-password), wired via
  `internal/server.NewRouter` (shared by `cmd/server/main.go` and the integration test
  harness). `backend/tests/integration/` scaffolded per the testing strategy below — 10 e2e
  tests plus unit tests for JWT and the migration statement splitter. `go test ./...` green
  (15 tests) with zero manual steps. Local dev environment: PostgreSQL 16 and Mailpit both
  installed via Homebrew (no Docker available on this machine) and running as background
  services; `clinicapp_dev` and `clinicapp_test` databases created locally. Pre-push hook
  (`scripts/install-hooks.sh`) and GitHub Actions CI (`.github/workflows/ci.yml`, Postgres +
  Mailpit service containers) both run `go test ./...` as a hard gate.
- **2026-08-03** (M1 planning): Testing strategy decided. All milestone verification automated
  via `go test ./...` — no manual curl steps. Integration test harness lives in
  `backend/tests/integration/`. FakeMailer (in-process) used for email flow tests; Mailpit
  (Docker) used for local dev SMTP inspection. Each milestone adds its own integration test
  file before being marked complete. Canonical flow: register → FakeMailer captures email →
  extract token → verify → login → hit protected endpoint → assert 200.
- **2026-08-03** (M0): Repo skeleton built — full directory structure per `PLAN.md`, `go.mod`
  (module `clinicapp/backend`, Go 1.21), minimal `cmd/server/main.go` with `GET /healthz`,
  `.gitignore`, `.env.example`. `internal/*` packages scaffolded with package-only `doc.go` files
  (no logic yet — that lands per-module starting M1). Pushed to
  `https://github.com/ricksonmenezes/clinicapp` (public).
- **2026-08-03** (planning): Tech stack confirmed as Go backend + HTMX frontend. One backend
  serves web and mobile via `X-Client-Type` header — same endpoints, different response renderer.
  No separate mobile API surface.
- **2026-08-03** (planning): Email chosen as the unique user identifier (lowercased on write).
  Phase 2 will add OAuth via the already-scaffolded `user_providers` table without schema breakage.
- **2026-08-03** (planning): JWT access tokens (15 min) + DB-backed refresh tokens (30 days).
  Web tokens in httpOnly cookies; mobile tokens in JSON body. Refresh token rotation on every
  use. Bulk revocation on password reset.
- **2026-08-03** (planning): Commission resolution hierarchy agreed: session-level override wins,
  then service-level override, then consultant global default. Rate at session time is always
  snapshotted — historical reports are immune to future config changes.
- **2026-08-03** (planning): Package commissions follow the performing consultant per session,
  not the principal consultant on the package record. The principal is informational only.
- **2026-08-03** (planning): Invoice template placeholders stored in DB, editable by admin.
  Server-side PDF generation (library choice deferred to M5 when implementation starts).
- **2026-08-03** (planning): SMS provider not yet confirmed by user. `internal/sms/` will be an
  interface from day one so M7 (customer portal) can be completed without it, and M8 slots the
  provider in when chosen.
