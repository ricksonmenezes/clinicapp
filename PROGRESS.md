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
- [x] **M2** — Patient & consultant management: patient profile CRUD, consultant profile CRUD,
      global commission config, attendant profiles. Role-gated (`admin`/`clinician`) via new
      `middleware.RequireRole`. Per-service commission override deferred to M3 (needs the
      `services` table). `go test ./...` green.
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

- **Production domain already provisioned**: `clinic.ricksonmenezes.com` DNS already points at the
  netcup server. Still pending: adding the `reverse_proxy localhost:8080` block for this subdomain
  to Caddy on the netcup server itself (see `CLAUDE.md` §7). Deferred to M9 (production deploy);
  no code-side work needed until then.


- Per-service commission override (`consultant_service_commission` table): needs `services`
  (M3) for its `service_id` FK. Land it alongside M3's service CRUD migrations.
- SMS provider: user to confirm which provider (Twilio, Vonage, local Philippine SMS gateway, etc.)
  before M8 can start. Interface is abstracted in `internal/sms/` — implementation slots in when
  confirmed.
- Phase 2: Google / Apple OAuth (user_providers table scaffolded in M1, unused until Phase 2).
- Phase 2: mobile app (React Native or Flutter) consuming the same Go API.
- Phase 2: reporting / analytics dashboard.
- Phase 2: multi-clinic / multi-branch support.

---

## Decision log

- **2026-08-03** (M2): Patient & consultant management complete. New `internal/patient`,
  `internal/consultant`, `internal/attendant` packages, each following M1's
  models/repository/service/handlers layering exactly (raw SQL via pgx/v5, sentinel errors,
  `statusForError`, `renderer.Render` for the web/mobile dispatch). Each `Create` cross-checks
  the given `user_id` against `auth.Repository` and rejects if the user doesn't exist or doesn't
  hold the matching role (patient/clinician/attendant respectively) — `ErrInvalidRole`/
  `ErrUserNotFound`. Migrations `007`–`009` add `patients`, `consultants` (with
  `default_commission NUMERIC(5,2)`), `attendants`, each `user_id UUID UNIQUE REFERENCES
  users(id)`. New `middleware.RequireRole(roles...)` reads `middleware.ClaimsFrom` and 403s if
  the caller's role isn't in the allow-list; chained inside `middleware.Auth`. Routes:
  `POST/GET /patients`, `GET/PATCH /patients/{id}` (admin+clinician read, admin write),
  `POST/GET/PATCH /consultants(/{id})` and `/attendants(/{id})` (admin only), matching PLAN.md's
  API table. Web-mode responses render minimal XSS-safe HTML fragments (`html.EscapeString`) —
  no template engine introduced yet, matching M1's inline-string convention.
  **Per-service commission override deferred to M3**: `consultant_service_commission` needs a
  `service_id` FK to a `services` table that doesn't exist until M3, so only the consultant's
  global `default_commission` ships in M2; the override table lands with M3's service CRUD.
  **Security fix bundled with M2** (user-approved before starting): `POST /auth/register` used
  to accept an arbitrary `role` in the body — harmless while nothing checked roles, but M2 adds
  the first role-gated routes, making it a real privilege-escalation path. Fixed by: (1)
  `auth.Service.Register` now always creates `role=patient`, ignoring any role in the request;
  (2) new admin-only `POST /auth/register-staff` creates clinician/attendant/admin accounts,
  active immediately, no email verification round-trip; (3) `server.Bootstrap` (called from
  `main.go` and `testserver.go`) idempotently creates one admin from
  `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` env vars, solving the chicken-and-egg
  problem of needing an admin to create the first admin. M1's `TestAuthFlow_*` tests updated:
  a self-registered (patient-role) token now gets 403 on `/patients` instead of 200, since that
  route is no longer role-agnostic. 24 integration tests total, `go test ./...` green. Manually
  smoke-tested against local Postgres: bootstrap admin login → register-staff → consultant CRUD
  → HTML fragment rendering, all correct.
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
