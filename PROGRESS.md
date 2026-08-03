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
- [x] **M3** — Services & packages: service CRUD (price, requires_consultant flag), package/promo
      definition (N sessions, package price, principal consultant), patient package subscription.
      Backoffice pages. Tests green.
- [x] **M4** — Session management: session recording (patient, service, consultant nullable,
      attendants), commission resolution logic (session override → service override → consultant
      default), commission snapshot storage, session history per patient and per consultant.
      Tests green.
- [x] **M5** — Invoicing & PDF: invoice generation per session and per package, server-side PDF
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
- SMS provider: user to confirm which provider (Twilio, Vonage, local Philippine SMS gateway, etc.)
  before M8 can start. Interface is abstracted in `internal/sms/` — implementation slots in when
  confirmed.
- Phase 2: Google / Apple OAuth (user_providers table scaffolded in M1, unused until Phase 2).
- Phase 2: mobile app (React Native or Flutter) consuming the same Go API.
- Phase 2: reporting / analytics dashboard.
- Phase 2: multi-clinic / multi-branch support.

---

## Decision log

- **2026-08-03** (M5): Invoicing & PDF complete. New `internal/invoice` package
  (`Invoice`/`TemplatePlaceholder` models, `Repository` + `PlaceholderRepository`, `Service`,
  `Handler`). Migrations `017`–`018` add `invoices` (with a DB `CHECK` enforcing exactly one of
  `session_id`/`package_id` — a real financial-record invariant, not just an API nicety) and
  `invoice_template_placeholders`. **PDF library decision (deferred from M3/PLAN.md to "when M5
  starts")**: chose `github.com/go-pdf/fpdf` — pure Go, no cgo, no external binary (rules out
  `wkhtmltopdf` via exec) and no headless-Chrome dependency (rules out `chromedp`), which matters
  because this machine and the netcup deploy target have no Docker and the project already avoids
  adding install-time system dependencies. `POST /invoices` takes only a `session_id` or
  `package_id` (a `patient_package` id) — never a caller-supplied `patient_id` or amount — and
  derives patient, line-item description, and total from the underlying record, so an invoice
  can't drift from what it's billing: session invoices total the service's price, package invoices
  total the package's price. New `INVOICE_STORAGE_DIR` env var (default `data/invoices`, relative
  to `backend/`) is where generated PDFs land; `pdf_path` is written to the DB in a second `UPDATE`
  after the DB assigns the invoice's id, since the on-disk filename is `{id}.pdf`. The JSON
  response never exposes the raw filesystem path — just a `pdf_available` boolean — with the
  actual bytes served through `GET /invoices/{id}/pdf`, which bypasses the web/mobile
  `renderer` dispatch entirely (raw `application/pdf`, not HTML/JSON) since a binary download
  isn't something X-Client-Type applies to. Commission history (CLAUDE.md's "must show
  `resolution_source` alongside the rate") landed as `GET /consultants/{id}/commission-history`,
  implemented in `internal/session` (where `CommissionSnapshot` already lives) but registered
  under the `/consultants/` path, same pattern as `consultant.Handler`'s own
  `/consultants/{id}/service-commissions`. 6 new integration tests in `invoice_test.go` cover
  session invoices, package invoices, the session/package exclusivity validation, unknown-FK
  rejection, placeholder upsert+list round-tripping, and commission history (including a manual
  PDF-render smoke check, not committed, confirming the output actually renders legibly). `go test
  ./...` green.
- **2026-08-03** (M4): Session management complete. New `internal/session` package
  (`Session`/`CommissionSnapshot` models, `Repository`, `Service`, `Handler` — same layering as
  every prior module). Migrations `014`–`016` add `sessions`, `session_attendants`,
  `session_commission_snapshot`. Routes: `POST/GET /sessions`, `GET /sessions/{id}`, all
  admin+clinician per PLAN.md's API table; `GET /sessions` takes optional `patient_id`/
  `consultant_id` query params rather than dedicated sub-routes, which is how "session history
  per patient and per consultant" (PLAN.md Module 5) is served.
  `POST /sessions` validates patient/service/consultant/attendant ids and, for
  package-linked sessions, that the `patient_package` belongs to the same patient and still has
  `sessions_remaining > 0`; if the service's `requires_consultant` flag is set, omitting
  `consultant_id` is a 400. Commission resolution (session override → service override →
  consultant default) lives in `session.Service.resolveCommission`, backed by a new
  `consultant.Repository.GetServiceCommission` single-row lookup (vs. the existing `List`).
  Session-level override is a one-shot `commission_override` field on the create request, not a
  persisted config row — matching PLAN.md's schema, which has no session-override table, only the
  `resolution_source` enum value on the snapshot. **Commission base amount decision (not
  specified in PLAN.md)**: both standalone and package-linked sessions calculate
  `clinic_amount`/`consultant_amount` off the session's `service.price`; package purchases are not
  amortized per session, since PLAN.md only specifies that the *performing consultant* (not the
  package's principal) is who earns commission, and defines no per-session pricing model for
  packages. Revisit if M5 invoicing needs package revenue split differently.
  `session.Repository.Create` runs everything — session insert, attendant roster, commission
  snapshot, and (if package-linked) the `sessions_remaining` decrement — inside one transaction;
  the decrement uses a conditional `UPDATE ... WHERE sessions_remaining > 0` as the actual
  concurrency-safe guard, not just the service-layer pre-check. It reaches into `patient_packages`
  (owned by `internal/promo`) directly rather than introducing a cross-package transaction/querier
  abstraction for this one call site. 8 new integration tests in
  `tests/integration/session_test.go` cover all three resolution sources, attendant-only services
  (no consultant → no snapshot), the `requires_consultant` rejection, package exhaustion, patient/
  package ownership mismatch, and history filtering. `go test ./...` green.
- **2026-08-03** (Resend migration — implemented): The Resend switch and Mailpit removal
  described in the two entries below are now implemented, not just documented.
  `internal/mailer` replaced `SMTPMailer` with `ResendMailer` (`POST api.resend.com/emails`,
  stdlib `net/http` only, no new dependency). `internal/config` replaced `SMTPHost/Port/User/
  Pass/From` with `ResendAPIKey`/`MailFrom` (default `onboarding@resend.dev`). `cmd/server/main.go`
  and `.env.example`/`backend/.env` updated to match. `.github/workflows/ci.yml`'s unused Mailpit
  service container removed. New `internal/middleware.EmailGuardrail` enforces the account-wide
  5/min, 20/hour, 100/day throttle (with the rolling 24h cooldown once the daily cap is hit) on
  `register`, `resend-verification`, `forgot-password`, and `reset-password` — wired in
  `internal/server/router.go` alongside (not replacing) the existing per-IP `RateLimiter`s.
  `RateLimiter` and the new guardrail both take an injectable clock internally so the cooldown
  logic is unit-testable without real sleeps (`internal/middleware/emailguardrail_test.go`).
  New integration test (`email_guardrail_test.go`) proves the throttle is a single account-wide
  bucket shared across endpoints, not per-endpoint/per-IP. `go test ./...` green. **Still
  outstanding**: `RESEND_API_KEY` in `backend/.env` is blank — a real (sandbox is fine) key from
  resend.com must be added before `register`/etc. will actually succeed against the live Resend
  API in local dev; without it the mailer call returns a 401 and the request fails.
- **2026-08-03** (M1 planning): Mailpit dropped from the stack entirely. `internal/mailer` will
  call the Resend HTTP API directly (`api.resend.com/emails`) — no SMTP layer, not even for local
  dev. Local dev uses the Resend sandbox key; emails are inspectable in the Resend dashboard but
  never delivered to real inboxes. To test the verify-email flow locally, copy the token from
  DB/logs and hit `GET /auth/verify-email?token=<token>` manually. `SMTP_*` env vars removed;
  replaced with `RESEND_API_KEY` and `MAIL_FROM`. **Docs-only so far** — `.github/workflows/ci.yml`
  still starts a Mailpit service container (currently unused dead weight, since Go tests never
  hit it — they use FakeMailer in-process) and `internal/mailer/mailer.go` still implements
  `SMTPMailer`. Removing the CI service block and swapping in a `ResendMailer` are pending
  implementation follow-ups.
- **2026-08-03** (mail provider change): Mail provider switched to **Resend** (resend.com),
  superseding M1's original SMTP-based `internal/mailer` implementation. `internal/mailer/` will
  implement the Resend REST API (`POST https://api.resend.com/emails` with `Authorization: Bearer
  <RESEND_API_KEY>`) — no SMTP dependency. Sender domain must be verified in the Resend dashboard
  before transactional email works in prod (`onboarding@resend.dev` works for local dev testing
  only). Free tier caps: **100 emails/day, 3,000/month, account-wide**. New guardrail (see
  `CLAUDE.md`'s Hard Constraints → "Email sending guardrails"): a global, not per-IP, throttle on
  every email-sending endpoint (`register`, `resend-verification`, `forgot-password`,
  `reset-password`) — more than
  5/min or 20/hour rejects further requests; hitting the 100/day cap stops servicing new requests
  until a full 24 hours have passed since the 100th email of that window, rather than resetting at
  midnight. **This is a docs-only change so far** — `internal/mailer` still implements `SMTPMailer`
  (see `backend/internal/mailer/mailer.go`); swapping in a `ResendMailer` and adding the new global
  rate-limit middleware are implementation follow-ups, not yet done.
- **2026-08-03** (M3): Services & packages complete. New `internal/service` package (domain
  type `Service`; business-logic layer named `Manager` instead of the usual `Service`/`NewService`
  convention, since that name is already taken by the package's own entity type) with full CRUD —
  `GET /services` is open to any authenticated role (patients will browse in the future customer
  portal per PLAN.md Module 8), writes are admin-only. Migration `010` adds `services`.
  Per-service commission override (deferred from M2, since it needs `services`) lands as an
  extension of `internal/consultant`: `POST/GET /consultants/{id}/service-commissions`, upserting
  on the `UNIQUE(consultant_id, service_id)` constraint rather than exposing separate create/update
  endpoints, since the override is a single value being set, not a record with its own lifecycle.
  Migration `011` adds `consultant_service_commission`. New `internal/promo` package (named for
  PLAN.md's "Package / Promo" entity, since `package` is a reserved Go keyword and can't be a
  package/directory name) holds both `Package` (bundle definition, admin-only CRUD via
  `/packages`) and `PatientPackage` (a patient's subscription via `POST/GET /patient-packages`,
  admin write / admin+clinician read like `/patients`). Subscribing seeds `sessions_remaining`
  from the package's `session_count` at purchase time — later edits to the package definition
  don't retroactively change existing subscriptions. `principal_consultant` on a subscription is
  validated if given (must be an existing consultant) but is informational only, per PLAN.md;
  actual commission resolution follows the performing consultant per session, landing in M4.
  Migrations `012`/`013` add `packages`/`patient_packages`. All new tables registered in the
  integration test harness's truncate list. 9 new integration tests across
  `service_test.go`, `consultant_service_commission_test.go`, `promo_test.go`. `go test ./...`
  green.
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
