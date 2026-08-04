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
- [x] **M10** — Reporting & analytics (Phase 2): four aggregate report endpoints (revenue,
      commission payouts, service popularity, booking volume), admin-only, JSON for mobile and a
      Chart.js-rendered fragment for web. Tests green.
- [x] **M11** — Patient portal UI: real HTML pages (`/`, `/register`, `/login`, `/check-email`,
      `/forgot-password`, `/reset-password`, `/dashboard`, `/book`) closing the gap where Module 8
      had only API/fragment endpoints and nothing mounted at `GET /` (a 404 since project
      inception, not an M9 deploy regression — flagged right after M9, filled in now). Tests green.
- [x] **M12** — Backoffice UI (staff-facing): real HTML pages under `/admin/*` for the admin/
      clinician-only capabilities that already exist as API/fragment endpoints from M2–M6 and M10
      (patient/consultant/attendant/service/package management, session recording, invoicing +
      template editor, prescriptions, reporting dashboards) but — same gap M11 closed for
      patients — never had a page shell, only curl/JSON-testable endpoints. Tests green.

---

## M12 scope (Backoffice UI — shipped, see decision log below for what changed during the build)

Mirrors M11's approach: new `internal/backoffice` package (`Templates` + `Handler`, no
repository — pure presentation over existing services), server-rendered Go templates under
`web/templates/admin/`, HTMX for GET-triggered fragments, the existing `web/static/js/app.js`
JSON-fetch helper reused as-is for POST forms (same redirect-vs-fragment response shape every
existing handler already returns — no backend changes expected). Role-gating on every page
matches its underlying API route exactly (no new authorization rules invented):

| Page(s) | Backing endpoint(s) | Role |
|---|---|---|
| `/admin` (dashboard/nav hub) | — | admin, clinician, attendant |
| `/admin/patients`, `/admin/patients/{id}` | `GET/POST /patients`, `GET/PATCH /patients/{id}` | admin+clinician read, admin write |
| `/admin/consultants` (+ commission config) | `GET/POST/PATCH /consultants(/{id})`, `/consultants/{id}/service-commissions`, `/consultants/{id}/commission-history` | admin only |
| `/admin/attendants` | `GET/POST/PATCH /attendants(/{id})` | admin only |
| `/admin/services` | `GET/POST /services` | any staff read, admin write |
| `/admin/packages`, `/admin/patient-packages` | `GET/POST /packages`, `/patient-packages` | admin write, admin+clinician read |
| `/admin/sessions` | `GET/POST /sessions`, `/sessions/{id}` | admin+clinician |
| `/admin/invoices`, `/admin/invoice-template` | `POST /invoices`, `/invoices/{id}/pdf`, placeholder endpoints | admin write, admin+clinician read PDF |
| `/admin/prescriptions` | `POST /prescriptions`, `/prescriptions/{id}/pdf` | clinician only |
| `/admin/reports` | `GET /reports/*` (hosts M10's existing Chart.js fragments) | admin only |
| `/admin/staff/new` | `POST /auth/register-staff` | admin only — the one admin capability with **zero UI today**, curl-only |

**Confirmed before starting the build** (per this project's standing "clarify vague/large scope
first" pattern from M10/M11):
1. **Shared `/login`, role-based redirect** — reuses the existing `/login` page and cookie/JWT
   mechanism as-is, no separate `/admin/login`. Only the post-login/home-page redirect branches by
   role (patient → `/dashboard`, staff → `/admin`). Requires adding a `Role` field to
   `portal.PageData` (currently only `Authenticated bool`) and updating `Home`'s
   redirect-if-authenticated branch.
2. **Built as one M12 pass**, not split into M12a/M12b — matches CLAUDE.md's default of building
   a whole milestone in one pass, committing incrementally as pieces land.

---

## Pending / deferred items

- **Phase 3** — mobile app (React Native or Flutter) consuming the same Go API.
- **Phase 4** — multi-clinic / multi-branch support.
- **Unscheduled** — Google / Apple OAuth (`user_providers` table scaffolded in M1, unused; not
  assigned to a phase — see `PLAN.md`).

---

## Decision log

- **2026-08-04** (M12): Backoffice UI complete — matches the scope table above exactly, plus one
  scope addition and one real bug found and fixed while building it.
  **New `internal/backoffice` package**, structurally identical to `internal/portal` (`Templates`
  + `Handler`, no repository). Role gating happens *inside* `backofficeHandler` (redirect to
  `/login` if unauthenticated, a rendered 403 page if the wrong role) rather than via
  `authRequired`/`RequireRole` at the router — same reasoning M11 used for `portal.Handler`: a
  page navigation should land somewhere useful, not return a bare 401/403 body. Every page's role
  gate matches its underlying API route's exactly (no new authorization rules invented) — this
  surfaced one real cross-role case worth calling out: `/admin/prescriptions` is `clinicianOnly`,
  so **admin itself gets a 403** there, same as the API — a dedicated test
  (`admin hits clinician-only prescriptions`) locks this in since it's easy to assume "admin can
  see everything."
  **One justified backend change**: `RegisterStaff`'s web-mode HTML fragment now includes
  `data-user-id="..."` (previously prose-only, no id) — needed so an admin creating a staff
  account via `/admin/staff/new` can carry that id into the consultant/attendant profile-creation
  forms that require it. Confirmed no test asserted the fragment's exact prior shape before
  changing it, same check M11 did before adding `data-scheduled-at` to the booking fragment.
  **Real bug found by manually testing the login flow, not by `go test`**: `POST /auth/login`'s
  web-mode redirect was hardcoded to `/dashboard` for every role — unaffected by M12's change to
  `portal.Handler.Home` (which only handles an *already-logged-in* visit to `/`). A freshly
  logged-in admin/clinician/attendant was landing on the patient dashboard, which then 403s on its
  patient-only `GET /patients/me` fragment. Fixed with a `homeRedirectFor(role)` helper mirroring
  `Home`'s split; `VerifyEmail` keeps its `/dashboard` literal since it's a patient-only flow
  (staff accounts are created active via `RegisterStaff`, no verification round-trip). Caught this
  specifically because CLAUDE.md's "test the golden path in a browser" guidance meant actually
  logging in as admin locally, not just hitting `/admin` with a pre-issued cookie — a scripted
  integration test alone wouldn't have exercised the real login redirect path for a staff role,
  since none of M12's own tests happened to check `Location` on a fresh login (fixed: added
  `TestLogin_RedirectsStaffToAdmin`).
  **Known, documented UX limitation (not fixed, scope judgment call)**: creating a
  patient/consultant/attendant profile from the backoffice requires pasting the raw `user_id` of
  an already-registered account — there's no `GET /users` lookup endpoint to browse/search by
  role, so admins must first create the account (via `/admin/staff/new`, which now at least
  surfaces the id per above) or, for patients, ask them to self-register and relay their id.
  Adding a users-listing endpoint was judged out of scope for a UI-only milestone (it's a new API
  surface, not a page wrapping an existing one) — flagged here rather than silently built or
  silently skipped, since it's a real day-to-day friction point if the clinic ever has walk-in
  patients registered by staff rather than self-registering.
  **`app.js` form serialization made type-aware** (M11's forms never exercised this — all
  text/date/email fields): checkboxes now serialize as real JSON booleans (`requires_consultant`,
  `active`), `<input type="number">` as JSON numbers (`price`, `default_commission`,
  `commission_override`, ...), `<input type="datetime-local">` converted to a full RFC3339 string
  via `new Date(...).toISOString()` (`scheduled_at` — the native `datetime-local` value has no
  seconds/timezone, which Go's `time.Time` JSON unmarshaling rejects), and blank *optional* fields
  (no `required` attribute) are omitted from the JSON body entirely rather than sent as `""`
  (matches how a JSON key's absence, not an empty string, is what makes Go's `*T` pointer/slice
  fields stay `nil`); required fields are guaranteed non-blank by native HTML5 validation before
  submit ever fires, so this doesn't weaken any existing validation.
  **New `web/static/js/admin.js`**: (1) clicking a `data-id` list item inside a
  `[data-admin-resource]` section fetches the full JSON record via `X-Client-Type: mobile` (the
  existing web-mode HTML fragments only carry an id and one or two display fields — enough for a
  list, not enough to pre-fill an edit form) and populates the section's `.admin-edit-form`
  fields, matched by `data-field`; (2) any `<select data-options-from="/services">` is populated
  from that endpoint's existing JSON list response instead of making the admin paste raw UUIDs for
  every foreign-key-shaped field (session's `patient_id`/`service_id`/`consultant_id`, a
  package's `service_id`, etc.) — `data-options-key`/`data-options-label` name the envelope key
  and display field; (3) the consultant per-service commission override sub-section uses htmx's
  own JS API (`htmx.ajax`) to reuse the existing `GET /consultants/{id}/service-commissions` HTML
  fragment for a dynamically-selected consultant id, rather than hand-rolling a client-side
  renderer, since `hx-get`'s URL is normally static.
  11 new integration tests in `backoffice_test.go` cover: unauthenticated redirect to `/login` on
  every `/admin/*` page, wrong role gets a rendered 403 (including the admin-vs-prescriptions case
  above), correct role renders 200 with expected content for admin/clinician/attendant, the
  register-staff fragment's `data-user-id`, and both halves of the staff-redirect fix
  (`portal.Handler.Home` and `POST /auth/login`). Manually verified beyond `go test` (no browser
  automation tool available in this session, so this was curl-driven, replicating the exact
  request shapes `app.js`/`admin.js` produce — JSON booleans/numbers/ISO datetimes,
  `X-Client-Type: mobile` fetches, dynamic PUT/PATCH URLs — rather than actually rendering and
  clicking through the pages in a real browser, unlike M11's Playwright-free-but-still-manual
  browser pass; noting this gap explicitly rather than claiming full UI verification): created a
  clinician and attendant staff account end-to-end (id surfaced in the fragment), created a
  consultant and attendant profile linked to those accounts, created a service, set a per-service
  commission override and confirmed its list fragment, recorded a session (confirming the
  datetime-local → RFC3339 conversion and commission-override resolution both worked), generated
  an invoice and downloaded its PDF, set an invoice template placeholder via the dynamic PUT URL,
  created a package and assigned it to a patient, authored a prescription as the clinician and
  downloaded its PDF, fetched a report's chart fragment, and confirmed the 403 forbidden page
  renders correctly for a clinician hitting an admin-only page. `go test ./...` green throughout.
- **2026-08-04** (Resend domain verified): `clinic.ricksonmenezes.com` is now a verified sending
  domain in Resend (user added it under Resend → Domains and added the DNS records to Cloudflare).
  Confirmed working end-to-end: registering a real patient on prod
  (`https://clinic.ricksonmenezes.com`) delivered the verification email from
  `noreply@clinic.ricksonmenezes.com`. This closes the last known gap from M9 — every
  email-blocking endpoint (`register`, `resend-verification`, `forgot-password`, `reset-password`,
  booking confirmation) is now expected to work on prod, not just locally. No code/config change
  needed on this end since `MAIL_FROM` was already set to this address in prod `.env` since M9 —
  the fix was entirely on the Resend/DNS side. Docs-only update: removed the corresponding
  "Pending / deferred items" bullet here and the "Outstanding" gap noted in `CLAUDE.md` §7.
- **2026-08-04** (M11): Patient portal UI complete — closes the `GET /` 404 flagged right after
  M9's deploy. **Scope confirmed with the user first**: full HTMX-style portal (real pages, not
  just a landing stub), reusing every existing API/fragment endpoint rather than building a
  second copy of any validation/authorization logic. New `internal/portal` package (`Templates`,
  `Handler`, no repository — pages are pure presentation). New `web/templates/*.html` (layout +
  8 pages) and `web/static/{css,js}` (real files replacing the `.gitkeep` placeholders that had
  sat empty since M0).
  **Per-page template sets, not one shared set**: `html/template` silently lets the last-parsed
  file's `{{define "content"}}` win when multiple files in one set define the same name, so each
  page is parsed as its own `layout.html` + `{name}.html` pair — 8 independent `*template.Template`
  values, not 8 files parsed together.
  **`GET /{$}` not `GET /`**: a bare `"/"` pattern is a subtree match in Go's `net/http.ServeMux`
  and silently catches every otherwise-unmatched path, which would have turned real 404s (typos,
  dead links) into a silently-served home page. `{$}` (Go 1.22+) matches only the exact root.
  Confirmed by a dedicated test (`TestPortalUnmatchedPath_Returns404NotHomePage`) — this was
  checked, not assumed.
  **No HTMX for POST forms**: HTMX's default form encoding is
  `application/x-www-form-urlencoded`, but every existing handler decodes `application/json`
  (`json.NewDecoder(r.Body).Decode`) — a real mismatch, not a style choice. Rather than touch 8
  already-tested handlers' body-decoding to also accept form-encoding (or lean on an HTMX
  extension whose redirect-interop behavior wasn't fully verifiable here), forms use a ~40-line
  vanilla-JS helper (`web/static/js/app.js`) that serializes to JSON and `fetch()`s the existing
  endpoint unchanged. Response handling is one rule for every form: `response.redirected` →
  `window.location = response.url` (real navigation, since `fetch` follows redirects
  transparently and exposes the final URL); not redirected → swap the response HTML into the
  form's `data-target` (covers both the success-fragment and error-fragment cases already
  returned by the existing handlers, uniformly). `GET`-triggered fragments (services list,
  availability, bookings list, own profile) use real HTMX (`hx-get`/`hx-trigger="load"`) — no
  request body, no encoding mismatch, HTMX's native strength.
  **Auth-state check duplicated, not reused, from `middleware.Auth`**: page routes need to
  *redirect* an unauthenticated visitor to `/login`; the JSON API's `middleware.Auth` returns a
  plain-text 401 by design (correct for an API, wrong for a page a browser just navigated to). A
  small `portal.Handler.authenticated` reads the same `access_token` cookie and calls the same
  `auth.ParseAccessToken`, just returning a bool instead of erroring — kept local to `internal/
  portal` rather than generalizing `middleware.Auth` itself, since the JSON API's 401 behavior is
  correct and shouldn't change.
  **One small existing-fragment change**: `booking.Handler.Availability`'s HTML fragment gained a
  `data-scheduled-at="..."` attribute per `<li>` (the RFC3339 timestamp was already in the text
  content, but not machine-readable) — needed so the booking page's JS can read which slot was
  clicked without parsing rendered text. Confirmed no existing test asserts the fragment's exact
  shape before making this change. `templatesDir`/`staticDir` are passed into `server.NewRouter`
  as parameters (now `(http.Handler, error)`, was `http.Handler`, since loading templates can fail)
  rather than hardcoded, matching `store.RunMigrations`' existing `dir` parameter — `cmd/server/
  main.go` and the integration test harness run from different working directories and need
  different relative paths to the same repo-root `web/` directory. No production/deploy changes
  needed beyond what already exists: `web/` is a normal tracked directory under the repo root, so
  the existing `git pull && systemctl restart` deploy one-liner picks it up like any other file.
  9 new integration tests in `portal_test.go` cover: unauthenticated `/` shows marketing content,
  authenticated `/` redirects to `/dashboard`, unauthenticated `/dashboard` and `/book` redirect
  to `/login`, authenticated versions render 200, every public form page renders its expected
  content, the reset-password token is echoed into its hidden field, `/static/` actually serves a
  file, and the `{$}`-vs-`/` 404 behavior above. Beyond `go test` (which can't execute client-side
  JS or render a real DOM): ran the local dev server and exercised the exact request shapes the
  browser JS produces — JSON POST with no `X-Client-Type` header, cookie-jar login, dashboard
  fragment loads, service/availability browsing (confirming the new `data-scheduled-at`
  attribute), a real booking creation, and logout — every response's redirect-vs-fragment shape
  matched what `app.js`/`book.js` assume. `go test ./...` green.
- **2026-08-04** (M10): Reporting & analytics complete — the first Phase 2 milestone. New
  `internal/report` package (`Repository`, `Service`, `Handler`, no new model/domain entity, no
  new migration — pure read-aggregation over `invoices`, `session_commission_snapshot`, and
  `sessions`, per `PLAN.md`'s note that Phase 2 needs "primarily new read/aggregate endpoints...
  not new domain entities"). **Scope confirmed with the user before starting** (`PLAN.md`'s Phase
  2 description was just "bookings, revenue, commission payouts, service popularity, etc." — see
  `ARCHITECTURE.md` §5/§6 on why vague scope gets clarified up front now instead of guessed): four
  reports — revenue over time, commission payouts per consultant, service popularity, booking
  volume/trends — delivered as endpoints whose web-mode HTML fragment renders an actual chart
  (Chart.js via CDN), not just a table; JSON for mobile as always.
  Routes: `GET /reports/revenue`, `/reports/commission-payouts`, `/reports/service-popularity`,
  `/reports/bookings`, all query-params `start`/`end` (RFC3339 or `YYYY-MM-DD`, default: last 30
  days) and `group_by=day|week|month` (revenue/bookings only, default `day`).
  **Role gating (not specified in PLAN.md — resolved by precedent, not asked)**: all four are
  admin-only, matching the existing `GET /consultants/{id}/commission-history` precedent from M5
  — aggregate financial/business reports are treated as more sensitive than the admin+clinician
  per-record reads (sessions, invoices, patient-packages), so clinicians don't get visibility into
  other consultants' payouts or clinic-wide revenue. **Chart rendering approach**: each report's
  web fragment is self-contained — a bare `<canvas>`, a `chart.js@4` CDN `<script>` tag, and an
  inline `<script>` that locates its own canvas via `document.currentScript.closest
  ('.report-chart')` rather than a unique element id, so multiple report widgets can safely land
  on the same page without id collisions. All dynamic values (labels, values, chart type, dataset
  label) are passed through `encoding/json.Marshal`, which HTML/JS-escapes `<`, `>`, `&` by
  default, before being embedded in the `<script>` block — deliberate XSS-safety choice given
  consultant/service names are user-supplied strings and this is the project's first fragment that
  embeds data inside inline JS rather than plain escaped HTML text. Revenue buckets by
  `invoices.issued_at` (when billed), booking volume/service popularity by `sessions.scheduled_at`
  (when the appointment is/was), commission payouts by `session_commission_snapshot.created_at`
  (when earned) — three different "which timestamp" choices per report, each the natural one for
  that report's question, confirmed by manually seeding real data through a local dev server run
  and inspecting the actual response (e.g. 5 invoices created back-to-back in one test session
  all land in a single revenue bucket, correctly, since they share one `issued_at` day regardless
  of their sessions' different `scheduled_at` dates). 9 new integration tests in `report_test.go`
  cover all four reports' aggregation correctness, date-range exclusion, `group_by` validation,
  `start >= end` rejection, admin-only gating (403 for clinician), and that the web fragment
  actually contains chart markup. Manually smoke-tested beyond `go test`: ran the local dev
  server, created real patient/consultant/service/session/invoice records, and fetched every
  report's web-mode fragment via curl to confirm the emitted HTML/JS is well-formed (`go test`
  alone can't confirm the client-side `Chart.js` bundle renders a correct chart — no real browser
  runs in the test suite, and no dashboard page exists yet for it to be visually mounted on, a
  pre-existing gap this milestone doesn't address, see M9's decision log entry on the `/` 404).
  `go test ./...` green.
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
