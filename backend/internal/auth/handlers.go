package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"clinicapp/backend/internal/renderer"
)

type HandlerConfig struct {
	SecureCookies      bool
	RefreshTokenExpiry time.Duration
}

type Handler struct {
	svc *Service
	cfg HandlerConfig
}

func NewHandler(svc *Service, cfg HandlerConfig) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register is the public self-service signup path. It always creates a
// patient account — callers cannot self-assign an elevated role. Staff
// accounts (clinician/attendant/admin) are created via RegisterStaff by an
// already-authenticated admin.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	user, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON: map[string]any{
			"id":     user.ID,
			"email":  user.Email,
			"status": user.Status,
		},
		RedirectURL: "/check-email",
	})
}

type registerStaffRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// RegisterStaff creates a clinician/attendant/admin account. Only reachable
// by an authenticated admin (enforced by router middleware) — the account is
// active immediately, no email verification round-trip required.
func (h *Handler) RegisterStaff(w http.ResponseWriter, r *http.Request) {
	var req registerStaffRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	user, err := h.svc.RegisterStaff(r.Context(), req.Email, req.Password, req.Role)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON: map[string]any{
			"id":     user.ID,
			"email":  user.Email,
			"role":   user.Role,
			"status": user.Status,
		},
		HTML: fmt.Sprintf(`<p>Staff account created for %s.</p>`, user.Email),
	})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")

	user, pair, err := h.svc.VerifyEmail(r.Context(), token)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	h.respondWithTokens(w, r, http.StatusOK, user, pair, "/dashboard")
}

type resendVerificationRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req resendVerificationRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ResendVerification(r.Context(), req.Email); err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"status": "ok"},
		HTML:   "<p>If that email is registered and unverified, a new link is on its way.</p>",
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	user, pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	h.respondWithTokens(w, r, http.StatusOK, user, pair, "/dashboard")
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	token := h.refreshTokenFromRequest(r)
	if token == "" {
		renderError(w, r, http.StatusUnauthorized, ErrTokenInvalid)
		return
	}

	pair, err := h.svc.Refresh(r.Context(), token)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	if renderer.ClientTypeFrom(r.Context()) == renderer.ClientWeb {
		h.setAuthCookies(w, pair)
		renderer.Render(w, r, renderer.Response{Status: http.StatusOK, HTML: "<p>Session refreshed.</p>"})
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON: map[string]any{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"expires_in":    pair.ExpiresIn,
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := h.refreshTokenFromRequest(r)
	if token != "" {
		if err := h.svc.Logout(r.Context(), token); err != nil {
			renderError(w, r, http.StatusInternalServerError, err)
			return
		}
	}

	if renderer.ClientTypeFrom(r.Context()) == renderer.ClientWeb {
		h.clearAuthCookies(w)
	}

	renderer.Render(w, r, renderer.Response{
		Status:      http.StatusOK,
		JSON:        map[string]any{"status": "ok"},
		RedirectURL: "/login",
	})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ForgotPassword(r.Context(), req.Email); err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"status": "ok"},
		HTML:   "<p>If that email is registered, a reset link is on its way.</p>",
	})
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status:      http.StatusOK,
		JSON:        map[string]any{"status": "ok"},
		RedirectURL: "/login",
	})
}

func (h *Handler) respondWithTokens(w http.ResponseWriter, r *http.Request, status int, user *User, pair *TokenPair, webRedirect string) {
	if renderer.ClientTypeFrom(r.Context()) == renderer.ClientWeb {
		h.setAuthCookies(w, pair)
		renderer.Render(w, r, renderer.Response{Status: status, RedirectURL: webRedirect})
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: status,
		JSON: map[string]any{
			"user": map[string]any{
				"id":     user.ID,
				"email":  user.Email,
				"role":   user.Role,
				"status": user.Status,
			},
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"expires_in":    pair.ExpiresIn,
		},
	})
}

func (h *Handler) refreshTokenFromRequest(r *http.Request) string {
	if renderer.ClientTypeFrom(r.Context()) == renderer.ClientWeb {
		if cookie, err := r.Cookie("refresh_token"); err == nil {
			return cookie.Value
		}
		return ""
	}

	var req refreshRequest
	_ = decodeJSON(r, &req)
	return req.RefreshToken
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, pair *TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    pair.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   pair.ExpiresIn,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RefreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.RefreshTokenExpiry.Seconds()),
	})
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   h.cfg.SecureCookies,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func renderError(w http.ResponseWriter, r *http.Request, status int, err error) {
	renderer.Render(w, r, renderer.Response{
		Status: status,
		JSON:   map[string]any{"error": err.Error()},
		HTML:   fmt.Sprintf(`<p class="error">%s</p>`, err.Error()),
	})
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrEmailTaken):
		return http.StatusConflict
	case errors.Is(err, ErrWeakPassword):
		return http.StatusBadRequest
	case errors.Is(err, ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, ErrEmailNotVerified), errors.Is(err, ErrUserSuspended):
		return http.StatusForbidden
	case errors.Is(err, ErrTokenInvalid), errors.Is(err, ErrTokenExpired),
		errors.Is(err, ErrTokenUsed), errors.Is(err, ErrTokenRevoked):
		return http.StatusBadRequest
	case errors.Is(err, ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidRole):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
