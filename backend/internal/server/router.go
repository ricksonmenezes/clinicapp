// Package server wires the full HTTP surface (middleware chain, auth
// endpoints) from injected dependencies, so both cmd/server/main.go and the
// integration test harness build the exact same router — the only
// difference being a real SMTP mailer vs. a FakeMailer.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"clinicapp/backend/internal/auth"
	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/middleware"
	"clinicapp/backend/internal/renderer"
)

func NewRouter(pool *pgxpool.Pool, cfg *config.Config, m mailer.Mailer) http.Handler {
	repo := auth.NewRepository(pool)
	svc := auth.NewService(repo, m, auth.ServiceConfig{
		JWTSecret:          cfg.JWTSecret,
		JWTExpiry:          time.Duration(cfg.JWTExpiryMinutes) * time.Minute,
		RefreshTokenExpiry: time.Duration(cfg.RefreshTokenExpiryDays) * 24 * time.Hour,
		BaseURL:            cfg.BaseURL,
	})
	authHandler := auth.NewHandler(svc, auth.HandlerConfig{
		SecureCookies:      cfg.AppEnv == "prod",
		RefreshTokenExpiry: time.Duration(cfg.RefreshTokenExpiryDays) * 24 * time.Hour,
	})

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
	mux.Handle("GET /patients", authRequired(http.HandlerFunc(placeholderPatientsHandler)))

	return middleware.ClientType(mux)
}

// placeholderPatientsHandler exists only to prove the auth middleware chain
// works end-to-end (see PLAN.md's M1 canonical test). Real patient CRUD
// lands in M2.
func placeholderPatientsHandler(w http.ResponseWriter, r *http.Request) {
	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"patients": []any{}},
		HTML:   "<ul></ul>",
	})
}

// healthzHandler always returns JSON, regardless of X-Client-Type — it's a
// plain infra health check, not part of the web/mobile response dispatch.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
