// Package server wires the full HTTP surface (middleware chain, auth
// endpoints) from injected dependencies, so both cmd/server/main.go and the
// integration test harness build the exact same router — the only
// difference being a real SMTP mailer vs. a FakeMailer.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"clinicapp/backend/internal/attendant"
	"clinicapp/backend/internal/auth"
	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/middleware"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/promo"
	"clinicapp/backend/internal/service"
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

func NewRouter(pool *pgxpool.Pool, cfg *config.Config, m mailer.Mailer) http.Handler {
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

	patientHandler := patient.NewHandler(patient.NewService(patientRepo, authRepo))
	consultantHandler := consultant.NewHandler(consultant.NewService(consultantRepo, authRepo, serviceRepo))
	attendantHandler := attendant.NewHandler(attendant.NewService(attendant.NewRepository(pool), authRepo))
	serviceHandler := service.NewHandler(service.NewManager(serviceRepo))

	packageRepo := promo.NewPackageRepository(pool)
	promoHandler := promo.NewHandler(
		promo.NewPackageManager(packageRepo, serviceRepo),
		promo.NewPatientPackageManager(promo.NewPatientPackageRepository(pool), packageRepo, patientRepo, consultantRepo),
	)

	registerLimiter := middleware.NewRateLimiter(3, time.Minute)
	loginLimiter := middleware.NewRateLimiter(5, time.Minute)
	resendLimiter := middleware.NewRateLimiter(3, time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)

	mux.Handle("POST /auth/register", registerLimiter.Middleware(http.HandlerFunc(authHandler.Register)))
	mux.HandleFunc("GET /auth/verify-email", authHandler.VerifyEmail)
	mux.Handle("POST /auth/resend-verification", resendLimiter.Middleware(http.HandlerFunc(authHandler.ResendVerification)))
	mux.Handle("POST /auth/login", loginLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /auth/reset-password", authHandler.ResetPassword)

	authRequired := middleware.Auth(cfg.JWTSecret)
	adminOnly := middleware.RequireRole(auth.RoleAdmin)
	adminOrClinician := middleware.RequireRole(auth.RoleAdmin, auth.RoleClinician)

	mux.Handle("POST /auth/register-staff", authRequired(adminOnly(http.HandlerFunc(authHandler.RegisterStaff))))

	mux.Handle("POST /patients", authRequired(adminOnly(http.HandlerFunc(patientHandler.Create))))
	mux.Handle("GET /patients", authRequired(adminOrClinician(http.HandlerFunc(patientHandler.List))))
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

	return middleware.ClientType(mux)
}

// healthzHandler always returns JSON, regardless of X-Client-Type — it's a
// plain infra health check, not part of the web/mobile response dispatch.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
