package middleware

import (
	"testing"
	"time"
)

func TestEmailGuardrail_DailyCapAndRollingCooldown(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	g := newEmailGuardrail(1000, 1000, 3, clock)

	for i := 1; i <= 3; i++ {
		if !g.allow() {
			t.Fatalf("request %d: want allowed (under daily cap), got blocked", i)
		}
	}

	if g.allow() {
		t.Fatal("request 4: want blocked (daily cap of 3 reached), got allowed")
	}

	now = now.Add(23*time.Hour + 59*time.Minute)
	if g.allow() {
		t.Fatal("request just under 24h since the cap-hitting request: want still blocked, got allowed")
	}

	now = now.Add(2 * time.Minute)
	if !g.allow() {
		t.Fatal("request just over 24h since the cap-hitting request: want allowed (fresh window), got blocked")
	}
}

func TestEmailGuardrail_PerMinuteLimit(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	g := newEmailGuardrail(2, 1000, 1000, clock)

	if !g.allow() || !g.allow() {
		t.Fatal("first two requests within the per-minute limit: want allowed")
	}
	if g.allow() {
		t.Fatal("third request within the same minute: want blocked")
	}

	now = now.Add(time.Minute + time.Second)
	if !g.allow() {
		t.Fatal("request after the minute window rolls over: want allowed")
	}
}
