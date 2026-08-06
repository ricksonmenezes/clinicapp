// Package server wires the full HTTP surface (middleware chain, auth
// endpoints) from injected dependencies, so both cmd/server/main.go and the
// integration test harness build the exact same router — the only
// difference being real Resend/PhilSMS senders vs. fakes.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"clinicapp/backend/internal/attendant"
	"clinicapp/backend/internal/auth"
	"clinicapp/backend/internal/backoffice"
	"clinicapp/backend/internal/booking"
	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/invoice"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/middleware"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/portal"
	"clinicapp/backend/internal/prescription"
	"clinicapp/backend/internal/promo"
	"clinicapp/backend/internal/report"
	"clinicapp/backend/internal/service"
	"clinicapp/backend/internal/session"
	"clinicapp/backend/internal/sms"
)

// Bootstrap creates the configured admin account if it doesn't exist yet.
// It's a no-op when ADMIN_BOOTSTRAP_EMAIL/PASSWORD aren't set, and safe to
// call on every startup — it only ever creates the account once. This is
// the only way to obtain the first admin, since RegisterStaff itself
// requires an authenticated admin.
func Bootstrap(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	if cfg.AdminBootstrapEmail == "" || cfg.AdminBootstrapPassword == "" {
		return nil
	}
	repo := auth.NewRepository(pool)
	svc := auth.NewService(repo, nil, auth.ServiceConfig{})
	return svc.EnsureBootstrapAdmin(ctx, cfg.AdminBootstrapEmail, cfg.AdminBootstrapPassword)
}

// NewRouter builds the full HTTP surface. templatesDir/staticDir locate the
// patient-portal page templates and their CSS/JS assets — passed in rather
// than hardcoded, same as store.RunMigrations' migrations dir, since
// cmd/server/main.go and the integration test harness run from different
// working directories and so need different relative paths to the same
// repo-root web/ directory.
func NewRouter(pool *pgxpool.Pool, cfg *config.Config, m mailer.Mailer, s sms.Sender, templatesDir, staticDir string) (http.Handler, error) {
	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, m, auth.ServiceConfig{
		JWTSecret:          cfg.JWTSecret,
		JWTExpiry:          time.Duration(cfg.JWTExpiryMinutes) * time.Minute,
		RefreshTokenExpiry: time.Duration(cfg.RefreshTokenExpiryDays) * 24 * time.Hour,
		BaseURL:            cfg.BaseURL,
	})
	authHandler := auth.NewHandler(authSvc, auth.HandlerConfig{
		SecureCookies:      cfg.AppEnv == "prod",
		RefreshTokenExpiry: time.Duration(cfg.RefreshTokenExpiryDays) * 24 * time.Hour,
	})

	serviceRepo := service.NewRepository(pool)
	patientRepo := patient.NewRepository(pool)
	consultantRepo := consultant.NewRepository(pool)

	attendantRepo := attendant.NewRepository(pool)

	patientHandler := patient.NewHandler(patient.NewService(patientRepo, authRepo))
	consultantHandler := consultant.NewHandler(consultant.NewService(consultantRepo, authRepo, serviceRepo))
	attendantHandler := attendant.NewHandler(attendant.NewService(attendantRepo, authRepo))
	serviceHandler := service.NewHandler(service.NewManager(serviceRepo))

	packageRepo := promo.NewPackageRepository(pool)
	patientPackageRepo := promo.NewPatientPackageRepository(pool)
	promoHandler := promo.NewHandler(
		promo.NewPackageManager(packageRepo, serviceRepo),
		promo.NewPatientPackageManager(patientPackageRepo, packageRepo, patientRepo, consultantRepo),
	)

	sessionRepo := session.NewRepository(pool)
	sessionSvc := session.NewService(
		sessionRepo, patientRepo, serviceRepo, consultantRepo, attendantRepo, patientPackageRepo,
	)
	sessionHandler := session.NewHandler(sessionSvc)

	placeholderRepo := invoice.NewPlaceholderRepository(pool)
	invoiceHandler := invoice.NewHandler(invoice.NewService(
		invoice.NewRepository(pool), placeholderRepo,
		sessionRepo, serviceRepo, packageRepo, patientPackageRepo, patientRepo,
		cfg.InvoiceStorageDir,
	))

	prescriptionHandler := prescription.NewHandler(prescription.NewService(
		prescription.NewRepository(pool), consultantRepo, patientRepo, sessionRepo, placeholderRepo,
		cfg.PrescriptionStorageDir,
	))

	bookingHandler := booking.NewHandler(booking.NewService(
		sessionSvc, sessionRepo, serviceRepo, consultantRepo, patientRepo, authRepo, m, s,
	))

	reportHandler := report.NewHandler(report.NewService(report.NewRepository(pool)))

	portalTemplates, err := portal.LoadTemplates(templatesDir)
	if err != nil {
		return nil, err
	}
	portalHandler := portal.NewHandler(portalTemplates, cfg.JWTSecret)

	backofficeTemplates, err := backoffice.LoadTemplates(filepath.Join(templatesDir, "admin"))
	if err != nil {
		return nil, err
	}
	backofficeHandler := backoffice.NewHandler(backofficeTemplates, cfg.JWTSecret)

	registerLimiter := middleware.NewRateLimiter(3, time.Minute)
	loginLimiter := middleware.NewRateLimiter(5, time.Minute)
	resendLimiter := middleware.NewRateLimiter(3, time.Hour)

	// Global, account-wide throttle shared by every email-sending endpoint —
	// see CLAUDE.md's "Email sending guardrails" for why this exists on top
	// of (not instead of) the per-IP limiters above: Resend's free tier caps
	// outbound email for the whole account, not per-IP or per-recipient.
	emailGuardrail := middleware.NewEmailGuardrail(5, 20, 100)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)

	mux.Handle("POST /auth/register", registerLimiter.Middleware(emailGuardrail.Middleware(http.HandlerFunc(authHandler.Register))))
	mux.HandleFunc("GET /auth/verify-email", authHandler.VerifyEmail)
	mux.Handle("POST /auth/resend-verification", resendLimiter.Middleware(emailGuardrail.Middleware(http.HandlerFunc(authHandler.ResendVerification))))
	mux.Handle("POST /auth/login", loginLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.Handle("POST /auth/forgot-password", emailGuardrail.Middleware(http.HandlerFunc(authHandler.ForgotPassword)))
	mux.Handle("POST /auth/reset-password", emailGuardrail.Middleware(http.HandlerFunc(authHandler.ResetPassword)))

	authRequired := middleware.Auth(cfg.JWTSecret)
	adminOnly := middleware.RequireRole(auth.RoleAdmin)
	adminOrClinician := middleware.RequireRole(auth.RoleAdmin, auth.RoleClinician)
	clinicianOnly := middleware.RequireRole(auth.RoleClinician)
	patientOnly := middleware.RequireRole(auth.RolePatient)

	mux.Handle("POST /auth/register-staff", authRequired(adminOnly(http.HandlerFunc(authHandler.RegisterStaff))))
	// Name-search lookup for staff accounts (GET /users?role=&q=&unlinked=),
	// admin-only same as register-staff — lets the backoffice find a
	// consultant/attendant by name instead of a raw user id (PLAN.md's
	// "Changes requested").
	mux.Handle("GET /users", authRequired(adminOnly(http.HandlerFunc(authHandler.Search))))

	mux.Handle("POST /patients", authRequired(adminOnly(http.HandlerFunc(patientHandler.Create))))
	mux.Handle("GET /patients", authRequired(adminOrClinician(http.HandlerFunc(patientHandler.List))))
	// /patients/me is the customer-portal self-service profile (patient-only,
	// resolved from the JWT subject) — registered before /patients/{id} so
	// Go's ServeMux picks the more specific literal match.
	mux.Handle("POST /patients/me", authRequired(patientOnly(http.HandlerFunc(patientHandler.CreateSelf))))
	mux.Handle("GET /patients/me", authRequired(patientOnly(http.HandlerFunc(patientHandler.GetSelf))))
	mux.Handle("GET /patients/{id}", authRequired(adminOrClinician(http.HandlerFunc(patientHandler.Get))))
	mux.Handle("PATCH /patients/{id}", authRequired(adminOnly(http.HandlerFunc(patientHandler.Update))))

	mux.Handle("POST /consultants", authRequired(adminOnly(http.HandlerFunc(consultantHandler.Create))))
	mux.Handle("GET /consultants", authRequired(adminOnly(http.HandlerFunc(consultantHandler.List))))
	mux.Handle("GET /consultants/{id}", authRequired(adminOnly(http.HandlerFunc(consultantHandler.Get))))
	mux.Handle("PATCH /consultants/{id}", authRequired(adminOnly(http.HandlerFunc(consultantHandler.Update))))
	mux.Handle("POST /consultants/{id}/service-commissions", authRequired(adminOnly(http.HandlerFunc(consultantHandler.SetServiceCommission))))
	mux.Handle("GET /consultants/{id}/service-commissions", authRequired(adminOnly(http.HandlerFunc(consultantHandler.ListServiceCommissions))))

	mux.Handle("POST /attendants", authRequired(adminOnly(http.HandlerFunc(attendantHandler.Create))))
	mux.Handle("GET /attendants", authRequired(adminOnly(http.HandlerFunc(attendantHandler.List))))
	mux.Handle("GET /attendants/{id}", authRequired(adminOnly(http.HandlerFunc(attendantHandler.Get))))
	mux.Handle("PATCH /attendants/{id}", authRequired(adminOnly(http.HandlerFunc(attendantHandler.Update))))

	// /services is readable by any authenticated role (patients browse in the
	// future customer portal) but writable by admin only, per PLAN.md's API table.
	mux.Handle("POST /services", authRequired(adminOnly(http.HandlerFunc(serviceHandler.Create))))
	mux.Handle("GET /services", authRequired(http.HandlerFunc(serviceHandler.List)))
	mux.Handle("GET /services/{id}", authRequired(http.HandlerFunc(serviceHandler.Get)))
	mux.Handle("PATCH /services/{id}", authRequired(adminOnly(http.HandlerFunc(serviceHandler.Update))))

	mux.Handle("POST /packages", authRequired(adminOnly(http.HandlerFunc(promoHandler.CreatePackage))))
	mux.Handle("GET /packages", authRequired(adminOnly(http.HandlerFunc(promoHandler.ListPackages))))
	mux.Handle("GET /packages/{id}", authRequired(adminOnly(http.HandlerFunc(promoHandler.GetPackage))))
	mux.Handle("PATCH /packages/{id}", authRequired(adminOnly(http.HandlerFunc(promoHandler.UpdatePackage))))

	mux.Handle("POST /patient-packages", authRequired(adminOnly(http.HandlerFunc(promoHandler.CreatePatientPackage))))
	mux.Handle("GET /patient-packages", authRequired(adminOrClinician(http.HandlerFunc(promoHandler.ListPatientPackages))))
	mux.Handle("GET /patient-packages/{id}", authRequired(adminOrClinician(http.HandlerFunc(promoHandler.GetPatientPackage))))

	mux.Handle("POST /sessions", authRequired(adminOrClinician(http.HandlerFunc(sessionHandler.Create))))
	mux.Handle("GET /sessions", authRequired(adminOrClinician(http.HandlerFunc(sessionHandler.List))))
	mux.Handle("GET /sessions/{id}", authRequired(adminOrClinician(http.HandlerFunc(sessionHandler.Get))))

	mux.Handle("GET /consultants/{id}/commission-history", authRequired(adminOnly(http.HandlerFunc(sessionHandler.CommissionHistory))))

	mux.Handle("POST /invoices", authRequired(adminOnly(http.HandlerFunc(invoiceHandler.Create))))
	mux.Handle("GET /invoices", authRequired(adminOrClinician(http.HandlerFunc(invoiceHandler.List))))
	mux.Handle("GET /invoices/{id}", authRequired(adminOrClinician(http.HandlerFunc(invoiceHandler.Get))))
	mux.Handle("GET /invoices/{id}/pdf", authRequired(adminOrClinician(http.HandlerFunc(invoiceHandler.DownloadPDF))))

	mux.Handle("GET /invoice-template-placeholders", authRequired(adminOnly(http.HandlerFunc(invoiceHandler.ListPlaceholders))))
	mux.Handle("PUT /invoice-template-placeholders/{key}", authRequired(adminOnly(http.HandlerFunc(invoiceHandler.SetPlaceholder))))

	// Prescriptions are clinician-only end to end (not admin/clinician like
	// invoices) — per PLAN.md's API table, Rx authorship and content are a
	// clinician's professional responsibility, not a front-office/billing
	// concern.
	mux.Handle("POST /prescriptions", authRequired(clinicianOnly(http.HandlerFunc(prescriptionHandler.Create))))
	mux.Handle("GET /prescriptions", authRequired(clinicianOnly(http.HandlerFunc(prescriptionHandler.List))))
	mux.Handle("GET /prescriptions/{id}", authRequired(clinicianOnly(http.HandlerFunc(prescriptionHandler.Get))))
	mux.Handle("GET /prescriptions/{id}/pdf", authRequired(clinicianOnly(http.HandlerFunc(prescriptionHandler.DownloadPDF))))

	// Customer-facing portal (PLAN.md Module 8). GET /availability is open to
	// any authenticated role, same as GET /services — it's non-sensitive
	// calendar data staff may also want to check. Booking creation sends a
	// confirmation email, so it goes through the same account-wide
	// emailGuardrail as every other email-triggering endpoint (CLAUDE.md's
	// "Email sending guardrails").
	mux.Handle("GET /availability", authRequired(http.HandlerFunc(bookingHandler.Availability)))
	mux.Handle("POST /bookings", authRequired(patientOnly(emailGuardrail.Middleware(http.HandlerFunc(bookingHandler.Create)))))
	mux.Handle("GET /bookings", authRequired(patientOnly(http.HandlerFunc(bookingHandler.List))))
	mux.Handle("GET /bookings/{id}", authRequired(patientOnly(http.HandlerFunc(bookingHandler.Get))))

	// Reporting & analytics (PLAN.md Phase 2). All admin-only, same precedent
	// as GET /consultants/{id}/commission-history — these are aggregate
	// financial/business reports (revenue, commission payouts), not
	// per-record operational data, so they don't get the admin+clinician
	// access sessions/invoices/patient-packages have.
	mux.Handle("GET /reports/revenue", authRequired(adminOnly(http.HandlerFunc(reportHandler.Revenue))))
	mux.Handle("GET /reports/commission-payouts", authRequired(adminOnly(http.HandlerFunc(reportHandler.CommissionPayouts))))
	mux.Handle("GET /reports/service-popularity", authRequired(adminOnly(http.HandlerFunc(reportHandler.ServicePopularity))))
	mux.Handle("GET /reports/bookings", authRequired(adminOnly(http.HandlerFunc(reportHandler.BookingVolume))))

	// Patient-facing portal pages (PLAN.md Module 8's "customer-facing web
	// portal" — HTML page shells around the API/fragment endpoints above,
	// not a second API surface). "/{$}" matches only the exact root path,
	// not every unmatched path, unlike a bare "/" pattern which Go's
	// ServeMux treats as a catch-all subtree.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	mux.HandleFunc("GET /{$}", portalHandler.Home)
	mux.HandleFunc("GET /register", portalHandler.RegisterPage)
	mux.HandleFunc("GET /login", portalHandler.LoginPage)
	mux.HandleFunc("GET /check-email", portalHandler.CheckEmailPage)
	mux.HandleFunc("GET /forgot-password", portalHandler.ForgotPasswordPage)
	mux.HandleFunc("GET /reset-password", portalHandler.ResetPasswordPage)
	mux.HandleFunc("GET /dashboard", portalHandler.Dashboard)
	mux.HandleFunc("GET /book", portalHandler.BookPage)

	// Backoffice (staff-facing) pages (M12) — same "page shell around the
	// existing API/fragment endpoints" philosophy as the patient portal
	// above, just for admin/clinician/attendant roles. Role gating happens
	// inside backofficeHandler itself (redirect to /login, or a rendered
	// 403 page), not via authRequired/RequireRole, since these are page
	// navigations, not JSON API calls.
	mux.HandleFunc("GET /admin", backofficeHandler.Dashboard)
	mux.HandleFunc("GET /admin/patients", backofficeHandler.Patients)
	mux.HandleFunc("GET /admin/consultants", backofficeHandler.Consultants)
	mux.HandleFunc("GET /admin/attendants", backofficeHandler.Attendants)
	mux.HandleFunc("GET /admin/services", backofficeHandler.Services)
	mux.HandleFunc("GET /admin/packages", backofficeHandler.Packages)
	mux.HandleFunc("GET /admin/sessions", backofficeHandler.Sessions)
	mux.HandleFunc("GET /admin/invoices", backofficeHandler.Invoices)
	mux.HandleFunc("GET /admin/prescriptions", backofficeHandler.Prescriptions)
	mux.HandleFunc("GET /admin/reports", backofficeHandler.Reports)
	mux.HandleFunc("GET /admin/staff/new", backofficeHandler.StaffNew)

	return middleware.ClientType(mux), nil
}

// healthzHandler always returns JSON, regardless of X-Client-Type — it's a
// plain infra health check, not part of the web/mobile response dispatch.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
