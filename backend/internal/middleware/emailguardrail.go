package middleware

import (
	"net/http"
	"sync"
	"time"
)

// emailGuardrailKey is the single bucket key every request shares, since the
// guardrail is account-wide (Resend's free-tier cap applies to the whole
// account), not per-IP like RateLimiter's normal usage.
const emailGuardrailKey = "global"

// EmailGuardrail throttles every email-sending endpoint (register,
// resend-verification, forgot-password, reset-password) account-wide.
// Resend's free tier caps outbound email at 100/day and 3,000/month
// regardless of which user or IP triggered the send, so per-IP limiting
// alone isn't enough to stay under it.
//
// Per-minute and per-hour limits reuse RateLimiter's fixed-window logic
// keyed on a single constant key. The daily cap works differently: once hit,
// sending stays blocked for a full rolling 24h from the request that hit the
// cap, not a reset at midnight.
type EmailGuardrail struct {
	perMinute *RateLimiter
	perHour   *RateLimiter
	now       func() time.Time

	dailyLimit  int
	dailyMu     sync.Mutex
	dailyCount  int
	cooldownEnd time.Time // zero if not currently in cooldown
}

func NewEmailGuardrail(perMinuteLimit, perHourLimit, perDayLimit int) *EmailGuardrail {
	return newEmailGuardrail(perMinuteLimit, perHourLimit, perDayLimit, time.Now)
}

func newEmailGuardrail(perMinuteLimit, perHourLimit, perDayLimit int, now func() time.Time) *EmailGuardrail {
	return &EmailGuardrail{
		perMinute:  newRateLimiter(perMinuteLimit, time.Minute, now),
		perHour:    newRateLimiter(perHourLimit, time.Hour, now),
		now:        now,
		dailyLimit: perDayLimit,
	}
}

func (g *EmailGuardrail) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *EmailGuardrail) allow() bool {
	if !g.perMinute.allow(emailGuardrailKey) {
		return false
	}
	if !g.perHour.allow(emailGuardrailKey) {
		return false
	}
	return g.allowDaily()
}

func (g *EmailGuardrail) allowDaily() bool {
	g.dailyMu.Lock()
	defer g.dailyMu.Unlock()

	now := g.now()
	if !g.cooldownEnd.IsZero() {
		if now.Before(g.cooldownEnd) {
			return false
		}
		// A full 24h has passed since the request that hit the cap — start fresh.
		g.dailyCount = 0
		g.cooldownEnd = time.Time{}
	}

	g.dailyCount++
	if g.dailyCount >= g.dailyLimit {
		g.cooldownEnd = now.Add(24 * time.Hour)
	}
	return true
}
