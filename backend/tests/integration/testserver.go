package integration

import (
	"context"
	"net/http/httptest"
	"testing"

	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/server"
)

// TestAdminEmail / TestAdminPassword are the bootstrap admin credentials
// every test server is seeded with, so tests can log in as an admin without
// reaching into the DB directly.
const (
	TestAdminEmail    = "admin@clinic.local"
	TestAdminPassword = "admin-bootstrap-password"
)

// webTemplatesDir/webStaticDir are relative to this package's directory
// (backend/tests/integration/), same convention as testdb.go's migrationsDir.
const webTemplatesDir = "../../../web/templates"
const webStaticDir = "../../../web/static"

// NewTestServer spins up the full HTTP server (real test DB, injected
// FakeMailer and FakeSMSSender) behind an httptest.Server, and returns both
// fakes so tests can assert on outbound email/SMS. The server and its DB
// pool are closed automatically via t.Cleanup. A bootstrap admin
// (TestAdminEmail / TestAdminPassword) is seeded on every call.
func NewTestServer(t *testing.T) (*httptest.Server, *FakeMailer, *FakeSMSSender) {
	t.Helper()

	pool := SetupTestDB(t)
	fakeMailer := NewFakeMailer()
	fakeSMS := NewFakeSMSSender()

	cfg := &config.Config{
		AppEnv:                 "local",
		JWTSecret:              "test-secret-do-not-use-in-prod",
		JWTExpiryMinutes:       15,
		RefreshTokenExpiryDays: 30,
		BaseURL:                "http://localhost:8080",
		MailFrom:               "onboarding@resend.dev",
		InvoiceStorageDir:      t.TempDir(),
		PrescriptionStorageDir: t.TempDir(),
		AdminBootstrapEmail:    TestAdminEmail,
		AdminBootstrapPassword: TestAdminPassword,
	}

	if err := server.Bootstrap(context.Background(), pool, cfg); err != nil {
		t.Fatalf("bootstrap test admin: %v", err)
	}

	router, err := server.NewRouter(pool, cfg, fakeMailer, fakeSMS, webTemplatesDir, webStaticDir)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	return ts, fakeMailer, fakeSMS
}
