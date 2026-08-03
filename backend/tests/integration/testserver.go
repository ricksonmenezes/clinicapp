package integration

import (
	"net/http/httptest"
	"testing"

	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/server"
)

// NewTestServer spins up the full HTTP server (real test DB, injected
// FakeMailer) behind an httptest.Server, and returns the FakeMailer so
// tests can assert on outbound email. The server and its DB pool are
// closed automatically via t.Cleanup.
func NewTestServer(t *testing.T) (*httptest.Server, *FakeMailer) {
	t.Helper()

	pool := SetupTestDB(t)
	fakeMailer := NewFakeMailer()

	cfg := &config.Config{
		AppEnv:                 "local",
		JWTSecret:              "test-secret-do-not-use-in-prod",
		JWTExpiryMinutes:       15,
		RefreshTokenExpiryDays: 30,
		BaseURL:                "http://localhost:8080",
		SMTPFrom:               "noreply@clinic.local",
	}

	router := server.NewRouter(pool, cfg, fakeMailer)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	return ts, fakeMailer
}
