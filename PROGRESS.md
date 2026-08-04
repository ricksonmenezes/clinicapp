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
- [x] **M6** — Prescriptions (Rx): Rx authoring UI for consultants, printable PDF output.
      Tests green.
- [x] **M7** — Customer-facing portal: patient self-register/login (same auth backend), service
      browse, calendar slot availability view, booking flow, email confirmation on booking.
      Tests green.
- [x] **M8** — SMS notifications: integrate PhilSMS, send SMS on booking confirmation.
      Tests green.
- [x] **M9** — Production deploy: server provisioned, systemd service installed, Caddy configured,
      Cloudflare orange proxy active (TLS). `curl https://clinic.ricksonmenezes.com/healthz` → ok.
      Auth/DB verified end-to-end on prod. Email-dependent flows (registration, booking
      confirmation) pending Resend domain verification — see decision log and pending items.

---

## Pending / deferred items

- **Verify `clinic.ricksonmenezes.com` as a Resend sending domain**: add the domain under Resend →
  Domains and add the DNS records it provides to Cloudflare. Until this is done, every
  email-blocking endpoint on prod (`register`, `resend-verification`, `forgot-password`,
  `reset-password`, booking confirmation) returns an error, since all of them fail the whole
  request if the mailer call fails (SMS confirmation is unaffected — it's best-effort). This is
  the only known gap between "deployed" and "fully usable by real patients."
- **Phase 2** — reporting / analytics dashboard (see `PLAN.md`'s "Phase 2 scope (deferred)").
- **Phase 3** — mobile app (React Native or Flutter) consuming the same Go API.
- **Phase 4** — multi-clinic / multi-branch support.
- **Unscheduled** — Google / Apple OAuth (`user_providers` table scaffolded in M1, unused; not
  assigned to a phase — see `PLAN.md`).

---

## Decision log

- **2026-08-04** (roadmap re-scoped): The old flat "Phase 2 (deferred)" bucket — OAuth, mobile
  app, reporting/analytics, multi-clinic — is now split into distinct phases in `PLAN.md`:
  **Phase 2** = reporting/analytics dashboard; **Phase 3** = mobile app; **Phase 4** =
  multi-clinic/multi-branch support; **OAuth** moved to an "Unscheduled" bucket (doesn't fit
  cleanly into any of the three above, not currently prioritized). **Does not touch the M0–M9
  milestone checklist or any already-shipped scope** — the already-complete patient
  register/verify-email/login/book flow (M1 + M7 + M8) and the already-complete admin
  service/consultant/commission/pricing management (M2 + M3) both stay documented exactly as
  before; the new phase list is additive, describing only what's not yet built. Docs-only change
  across `PLAN.md` and this file — no code changed.
- **2026-08-04** (M9): Production deploy complete. Live at `https://clinic.ricksonmenezes.com`.
  **Server state discovered before provisioning (not previously documented)**: the netcup box
  (`ssh netcup`, Debian 13) already runs Go 1.26.5 and Caddy 2.6.2 for unrelated sites (a Hugo
  static site at `ricksonmenezes.com`, and a separate `sanasalinin.service` already bound to
  `8080`) — PostgreSQL was not installed. **Port decision**: since `8080` was already taken,
  clinicapp runs on **`8081`** (`PORT=8081` in prod `.env`), not the `8080` used everywhere in
  local dev/docs — if `deploy/Caddyfile`'s port reference is ever reused elsewhere, check `ss
  -tlnp` first rather than assuming `8080` is free. **Postgres**: created a scoped `clinicapp`
  role + `clinicapp_prod` database (generated password) rather than reusing/changing the actual
  `postgres` superuser — least-privilege, and avoids touching a credential that predates this
  project. **Caddy**: the live `/etc/caddy/Caddyfile` on the box already serves other, unrelated
  sites — a new `http://clinic.ricksonmenezes.com { reverse_proxy localhost:8081 }` block was
  *appended* (with a timestamped backup taken first), never overwritten wholesale; `deploy/
  Caddyfile` in this repo remains a single-site reference template, not what's actually live.
  System user `clinicapp` created (`--system --shell /usr/sbin/nologin`), repo cloned to
  `/opt/clinicapp` from the public GitHub remote, binary built there directly (no cross-compile,
  matching `scripts/deploy.sh`'s existing assumption). `INVOICE_STORAGE_DIR`/
  `PRESCRIPTION_STORAGE_DIR` point at `/var/lib/clinicapp/{invoices,prescriptions}` — outside
  `/opt/clinicapp` per the env table's "persistent path outside the deploy dir" guidance, so a
  future `git pull`-based redeploy can never touch generated PDFs. **Secrets handling**:
  `JWT_SECRET`, the Postgres password, and the `ADMIN_BOOTSTRAP_PASSWORD` were freshly generated
  (not reused from local dev) and shown to the user once in-session to save in their own password
  manager — they're not recorded anywhere in this repo. `RESEND_API_KEY` and `SMS_API_KEY` *were*
  reused from local dev, per explicit user choice (same providers/accounts regardless of which
  server calls them). `ADMIN_BOOTSTRAP_EMAIL=ricksonmenezes@gmail.com`. **`MAIL_FROM` decision**:
  the user chose `noreply@clinic.ricksonmenezes.com` over the Resend sandbox address for prod, but
  that domain is not yet verified in Resend — see "Pending / deferred items" above; every
  email-blocking endpoint will fail until that verification step happens. **Verification
  performed**: `curl https://clinic.ricksonmenezes.com/healthz` → `{"status":"ok"}` through
  Cloudflare; admin bootstrap login → JWT issued; authenticated `GET /services` round-tripped
  through Postgres — confirming the full Cloudflare → Caddy → systemd → app → DB path works.
  Deliberately did **not** exercise `POST /auth/register` or the booking flow against prod, since
  both are known to fail pending the Resend domain verification above — running them now would
  just reproduce a known, already-documented gap, not surface new information.
- **2026-08-03** (M8): SMS notifications complete. New `internal/sms` package (`Message`,
  `Sender` interface, `PhilSMSSender`) — same shape as `internal/mailer`'s `Message`/`Mailer`,
  so `internal/booking` treats SMS the same way it already treats email. The docs URL from the
  "M8 unblocked" entry below (`dashboard.philsms.com/developers/docs`) required a logged-in
  dashboard session and returned 401 to an unauthenticated fetch; the user supplied the working
  public docs URL (`app.philsms.com/developers/documentation`) instead, which gave the real
  spec: `POST https://app.philsms.com/api/v3/sms/send`, `Authorization: Bearer <SMS_API_KEY>`,
  `Accept`/`Content-Type: application/json`, body `{recipient, sender_id, type: "plain",
  message}` — note the send endpoint's host (`app.philsms.com`) differs from the dashboard's
  host (`dashboard.philsms.com`). **New `SMS_SENDER_ID` env var (not previously in
  `.env.example`, user-directed)**: defaults to `"PhilSMS"`, the platform's own shared sender
  name usable without registering a dedicated one — set per user instruction mid-implementation.
  **Phone normalization (not specified in PLAN.md)**: `patients.phone` (`internal/patient`) has
  always been unvalidated free text; `sms.NormalizePHPhone` converts the common local forms
  (`+639171234567`, `639171234567`, `09171234567`, `9171234567`) to the `63XXXXXXXXXX` shape
  PhilSMS's `recipient` field expects, rejecting anything else as `ErrInvalidPhone` rather than
  forwarding a malformed number to the API. **SMS confirmation is best-effort, unlike email**:
  `booking.Service.Book` already fails the whole booking if the email confirmation send fails
  (established in M7); SMS instead only logs on failure (missing/invalid phone, or a PhilSMS API
  error) and does not fail the booking, since phone — unlike email, the verified account
  identifier — is optional and never format-checked at write time. `server.NewRouter` and
  `booking.NewService` both gained an `sms.Sender` parameter; `cmd/server/main.go` wires the real
  `PhilSMSSender`, the integration harness wires a new `FakeSMSSender` (mirrors `FakeMailer`) —
  every existing integration test file's `NewTestServer(t)` call site was updated for the added
  return value. New `philsms_test.go` unit-tests `NormalizePHPhone` (valid/invalid formats) and
  `PhilSMSSender.Send` (request shape, auth header, success/error response handling) against a
  local `httptest` server. New integration test `TestBooking_SendsSMSConfirmationWhenPhoneOnFile`
  covers the send path end-to-end; every other booking test implicitly covers the no-phone
  skip path (none of them set a phone, none trigger an SMS). `go test ./...` green.
- **2026-08-03** (M8 unblocked): SMS provider confirmed as **PhilSMS**
  (dashboard.philsms.com). User added `SMS_PROVIDER=philsms` and `SMS_API_KEY=<redacted>` to
  `backend/.env`. API base URL supplied: `https://dashboard.philsms.com/api/v3/` (PhilSMS's
  dashboard labels this the "OAuth 2.0 API Endpoint"). Developer docs:
  `https://dashboard.philsms.com/developers/docs`. `internal/sms` is still an empty stub
  (`doc.go` only) — no implementation yet. Exact request/auth shape (bearer token vs. an actual
  OAuth2 client-credentials exchange) needs confirming against PhilSMS's API docs when M8
  implementation starts, same as M5 deferred the PDF library choice and M1 deferred Resend's
  request format to when each milestone actually began. Docs-only change — `PLAN.md`,
  `PROGRESS.md`, `CLAUDE.md` updated to reflect the confirmed provider; no code changed.
- **2026-08-03** (M7): Customer-facing portal complete. Patient self-registration/login reuses
  `POST /auth/register` + `/auth/login` as-is (both were already patient-role/client-agnostic).
  **Self-service profile (not specified in PLAN.md)**: registering only creates the `users` row —
  someone still has to create the `patients` profile row. Rather than auto-creating it inside
  `auth.Service.Register` (would invert package layering — `auth` would have to depend on
  `patient`), added `POST/GET /patients/me`, patient-role-only, resolving `user_id` from the JWT
  claims via a new `patient.Repository.GetByUserID` — same self-authorship pattern as M6's Rx
  consultant resolution. New `internal/booking` package implements availability + booking as a
  thin layer over `session.Service`, so a patient-initiated booking gets the exact same
  patient/service/consultant validation and commission-snapshot pipeline as a backoffice-created
  session — no parallel booking-specific business logic. **Clinic calendar (not specified in
  PLAN.md)**: fixed package constants, not DB/admin-configurable — Mon–Sat 09:00–17:00 UTC,
  30-minute slots, closed Sunday. Nothing in PLAN.md scopes admin-editable hours, so this avoids
  a second admin settings surface alongside the invoice placeholder table; revisit if a real
  schedule is ever needed. **Slot capacity (not specified in PLAN.md)**: the schema has no
  consultant-service capability mapping (which consultants can perform which service), so rather
  than invent one, clinic capacity per (service, slot) is 1 — at most one booking of a given
  service per slot, city-wide. For services with `requires_consultant`, `Book` additionally
  auto-assigns the first consultant with no session (of any service) at that exact time — the
  patient never picks a consultant, matching PLAN.md's booking flow verbatim ("choose service ->
  choose date/time -> confirm"). Availability is recomputed inside `Book` itself, not trusted
  from a prior `GET /availability` call, to catch staleness between viewing the calendar and
  confirming — this is a best-effort check-then-insert, not a DB constraint (a `UNIQUE(service_id,
  scheduled_at)` constraint was considered and rejected: it would also constrain admin-created
  backoffice sessions, breaking the existing, legitimate case of two different consultants seeing
  two different patients for the same service at the same time). New
  `session.Repository.ListOccupancyInRange` returns a lightweight service/consultant/time
  projection for these checks without paying `List`/`GetByID`'s attendant/commission-snapshot
  hydrate cost. Booking confirmation email is sent through the existing `mailer.Mailer`; per
  CLAUDE.md's "Email sending guardrails" (scoped to "every endpoint that sends an email"),
  `POST /bookings` is wrapped in the same account-wide `emailGuardrail` middleware as
  register/forgot-password/reset-password. **Patient-facing responses never expose
  `CommissionSnapshot`** (consultant/clinic revenue split) — `booking.Handler` has its own
  `bookingJSON`, deliberately narrower than `session.Handler`'s admin/clinician-facing
  `sessionJSON`. 9 new integration tests in `booking_test.go` cover: self-service profile
  create/get/duplicate-conflict/role-rejection, full booking flow with consultant
  auto-assignment (availability before/after, confirmation email, commission data absent from the
  response), same-slot conflict from a second patient (409), ownership isolation on
  `GET /bookings/{id}` and `GET /bookings`, attendant-only services needing no consultant, missing
  patient profile (400), non-patient caller (403), off-grid and past-time slots (400), and Sunday
  returning zero slots rather than an error. `go test ./...` green.
- **2026-08-03** (M6): Prescriptions (Rx) complete. New `internal/prescription` package
  (`Prescription` model, `Repository`, `Service`, `Handler`, `pdf.go`) — same layering as
  `internal/invoice`. Migration `019` adds `prescriptions` (`session_id` nullable FK, `consultant_id`
  and `patient_id` required). **Authorship decision (not fully specified in PLAN.md)**: `POST
  /prescriptions` never accepts a caller-supplied `consultant_id` — it's always resolved from the
  authenticated caller's own consultant profile via a new `consultant.Repository.GetByUserID`, so a
  logged-in clinician can only issue an Rx under their own name, never impersonate another
  clinician. A clinician user with no consultant profile row gets `ErrConsultantProfileMissing`
  (400). If `session_id` is given, it must belong to the same `patient_id` (`ErrSessionPatientMismatch`,
  400) — same cross-ownership validation style as M4's patient_package checks. Routes
  (`POST /prescriptions`, `GET /prescriptions`, `GET /prescriptions/{id}`,
  `GET /prescriptions/{id}/pdf`) are **clinician-only end to end**, not admin+clinician like
  invoices — per PLAN.md's API table, Rx authorship and content are a clinician's professional
  responsibility, not a front-office/billing concern, so admins get 403. PDF letterhead
  (clinic name/address/footer) reuses the existing `invoice_template_placeholders` table via
  `internal/invoice`'s already-exported `PlaceholderRepository`/`Placeholder*` constants rather than
  adding a second admin-editable branding config — it's the same clinic identity, not
  invoice-specific data, and CLAUDE.md's "never hardcode clinic name/address/footer" isn't scoped to
  just invoices. New `PRESCRIPTION_STORAGE_DIR` env var (default `data/prescriptions`), same
  relative-path convention as `INVOICE_STORAGE_DIR`. `GET /prescriptions/{id}/pdf` bypasses the
  web/mobile `renderer` dispatch like the invoice PDF route, for the same reason (binary download).
  5 new integration tests in `prescription_test.go` cover: authoring as the caller's own consultant
  profile (verified by asserting the returned `consultant_id`), rejecting a clinician with no
  consultant profile, rejecting an admin caller (403), session/patient mismatch validation (both the
  rejection and the matching-session success case), and `patient_id`/`consultant_id` list filtering.
  `go test ./...` green.
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
