// Package sms sends transactional SMS via PhilSMS (app.philsms.com), the
// provider confirmed for clinicapp (see CLAUDE.md §9 "Known gotchas"). It
// mirrors internal/mailer's Message/interface shape so callers (currently
// internal/booking) treat SMS the same way as email.
package sms
