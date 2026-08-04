# clinicapp — Architecture Decision Log & Future-Project Checklist

> This file exists for one reason: across this build, `PLAN.md` and `CLAUDE.md` didn't specify
> everything up front. Real decisions got made mid-milestone, some by asking the user, some as a
> judgment call documented after the fact in `PROGRESS.md`'s decision log. This file extracts
> those into reusable categories — a checklist to fill in *before* starting the next project, so
> the same categories of question don't have to interrupt a build again. Each category below is
> written generically first (the checklist item), then grounded with what actually happened here
> (the concrete example) so it's not abstract advice.
>
> **Living document**: per `CLAUDE.md`'s Workflow constraints, any future architectural/technical
> design decision forced by something unclear in the plan (provider specifics, secrets handling,
> deploy-target infra, sending-domain readiness, implicit business logic, etc.) gets added here as
> a dated note when it happens — not just recorded in `PROGRESS.md`. `PROGRESS.md` is this
> project's "what happened and why"; this file is the reusable "what to decide up front next
> time," and it keeps growing as long as this project is under active development.

---

## 1. Third-party provider decisions

**Checklist**: for every external service (email, SMS, payments, storage...), pin down *before*
implementation starts: which provider, the exact developer-docs URL (verify it's publicly
fetchable, not gated behind a dashboard login), the auth scheme (API key vs OAuth vs bearer
token), and the free-tier limits if relevant.

**What happened here**:
- Mail provider changed mid-project from a planned SMTP/Mailpit setup to Resend, after the plan
  was already written and partially implemented — a real pivot, not a gap-fill.
- SMS provider (PhilSMS) was left an open placeholder in `PLAN.md` from the start ("SMS provider
  not yet confirmed") — correct to defer, but the docs URL the user first supplied
  (`dashboard.philsms.com/developers/docs`) 401'd on fetch because it required a logged-in
  dashboard session. Had to ask the user for a working alternative
  (`app.philsms.com/developers/documentation`) before the integration could be verified against a
  real spec rather than guessed.
- **Lesson**: when a plan defers a provider choice, also defer confidently — don't let the
  eventual implementation start until a *fetchable* docs link is confirmed, not just a provider
  name.

## 2. Production secrets & credentials strategy

**Checklist**: decide, before deploy day: who generates secrets (agent vs human), where the human
gets a durable copy (the deployed `.env` is not a backup), whether any existing prod credential
(e.g. a DB superuser) gets touched or a new scoped one gets created, and what the rotation story
is for each.

**What happened here**: asked the user whether to generate `JWT_SECRET` and the Postgres
password automatically or have them supply/paste values. Agreed to auto-generate both plus the
admin bootstrap password, print them once in-session for the user to store in their own password
manager, and create a *new scoped* Postgres role (`clinicapp`) rather than touch the actual
`postgres` superuser. None of this was in `CLAUDE.md` beforehand — it now is (§3, §7).

## 3. Production infrastructure specifics

**Checklist**: before assuming a "clean deploy," check what's *already* running on the target —
other sites/services sharing the box, ports already bound, an existing reverse-proxy config that
must be appended to rather than overwritten. Don't let the plan's example config (a single-site
Caddyfile, a default port) become the literal deployed config without checking.

**What happened here**: the netcup server already ran an unrelated service
(`sanasalinin.service`) bound to port `8080` — the port every doc and the `deploy/Caddyfile`
example assumed clinicapp would use. Found only by SSHing in and running `ss -tlnp` before
provisioning. Ended up on port `8081`, and the real Caddy config was *appended* to the box's
existing multi-site Caddyfile (with a timestamped backup first), not generated fresh from the
repo's template. This was discoverable in five minutes, but nothing in the plan flagged "check
for existing occupants of the target server" as a step.

## 4. Sending-domain / deliverability readiness

**Checklist**: for any transactional email or SMS that's blocking (i.e., the request fails if the
send fails), confirm the sending identity is *fully verified with the provider* before treating
that flow as "done" — not just "configured."

**What happened here**: `MAIL_FROM` was set to a real subdomain address
(`noreply@clinic.ricksonmenezes.com`) at the user's explicit choice over the safer sandbox
fallback, but that domain isn't verified in Resend yet. Every email-blocking endpoint
(register, forgot-password, booking confirmation) will fail on prod until that DNS/dashboard step
happens — a real, currently-open gap, tracked in `PROGRESS.md`'s pending items, not something to
silently assume is fine because the config value looks correct.

## 5. Ambiguous business logic left implicit in the plan

**Checklist**: a plan that specifies entities and an API table still leaves real gaps — capacity
model, authorship/ownership rules, fixed-vs-configurable operational parameters (hours, slot
size), what a partial/derived value should be computed from. Either specify these explicitly per
module, or explicitly mark them "implementer's judgment call, document the choice" so a
mid-build decision isn't mistaken for a missed requirement.

**What happened here** (all resolved as documented, unilateral judgment calls, not user-asked —
listed to show how often this category comes up): self-service patient-profile creation pattern
(M7), fixed clinic calendar hours/slot size with no admin config (M7), one-booking-per-slot
citywide capacity model in the absence of a consultant-service capability table (M7), commission
base amount for package-linked sessions (M4), Rx authorship restricted to the caller's own
consultant profile (M6), phone-number normalization for a previously free-text field (M8). Seven
separate milestones, seven separate "not specified in PLAN.md" judgment calls — each fine
individually, but collectively suggests the plan's API/schema tables need an explicit
"open questions per module" subsection next time, filled in before that module starts rather than
discovered while writing it.

## 6. Roadmap / phase boundaries

**Checklist**: when phases are described in prose across a chat message, convert to an explicit
enumerated list (`Phase N: ...`) and confirm it back before writing it into docs — informal phase
labels collide easily (the same label reused for two different scopes) in a way a numbered list
in the docs won't.

**What happened here**: a request to restructure the roadmap used "phase 2" twice for two
different scopes in the same message, right before explicit "phase 3"/"phase 4" lines. Required a
clarifying round-trip before `PLAN.md`/`PROGRESS.md` could be edited correctly — guessing wrong
here would have produced two internally-inconsistent planning docs future sessions would trust.

## 7. Security decisions that need explicit sign-off, even when "obviously correct"

**Checklist**: when implementation work incidentally closes a security gap (not the milestone's
stated goal), surface it and get explicit confirmation before bundling it into the milestone,
even if the fix is uncontroversial.

**What happened here**: `POST /auth/register` originally accepted an arbitrary `role` in the
request body — harmless until M2 added the first role-gated routes, at which point it became a
real privilege-escalation path. Fixed as part of M2, but only after flagging it and getting the
user's go-ahead first, rather than silently bundling a security-relevant behavior change into an
unrelated milestone's diff.

---

## How to use this on the next project

Before writing that project's `PLAN.md`, run down sections 1–7 above as a literal checklist:
name every third-party provider and confirm its docs URL loads unauthenticated; decide the
secrets-generation/handoff story; ask "what else already runs on the deploy target"; confirm
sending-domain verification is a tracked step, not an assumption; add an explicit "open
questions" list per module instead of leaving gaps implicit; write phase/roadmap structure as a
numbered list from the start; and flag security-relevant side effects for sign-off as soon as
they're spotted, not after the fact.
