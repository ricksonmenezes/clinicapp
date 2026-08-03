package integration

import (
	"fmt"
	"net/http"
	"testing"
)

// TestEmailGuardrail_GlobalAcrossEndpoints proves the guardrail is a single
// account-wide bucket shared by every email-sending endpoint, not a
// per-endpoint or per-IP limiter (those already exist separately). It
// exhausts the per-minute budget (5) via forgot-password — which has no
// per-IP limiter of its own — then shows register is blocked too, even
// though register's own per-IP quota (3/min) is still untouched.
func TestEmailGuardrail_GlobalAcrossEndpoints(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	for i := 0; i < 5; i++ {
		resp := PostJSON(t, ts.URL, "/auth/forgot-password", "mobile", map[string]string{
			"email": fmt.Sprintf("guardrail-%d@example.com", i),
		}, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("forgot-password request %d: want 200, got %d", i, resp.StatusCode)
		}
	}

	resp := PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": "shouldbeblocked@example.com", "password": "correcthorsebattery",
	}, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("register after global per-minute cap exhausted by forgot-password: want 429, got %d", resp.StatusCode)
	}
}
