package portal

import (
	"net/http"

	"clinicapp/backend/internal/auth"
)

type Handler struct {
	tmpl      *Templates
	jwtSecret string
}

func NewHandler(tmpl *Templates, jwtSecret string) *Handler {
	return &Handler{tmpl: tmpl, jwtSecret: jwtSecret}
}

// claims reads and validates the access_token cookie, returning the parsed
// claims (with Role) when present, or (nil, false) otherwise.
func (h *Handler) claims(r *http.Request) (*auth.AccessClaims, bool) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return nil, false
	}
	claims, err := auth.ParseAccessToken(h.jwtSecret, cookie.Value)
	if err != nil {
		return nil, false
	}
	return claims, true
}

// authenticated reports whether the request carries a valid access_token
// cookie. Used two ways: to vary every page's nav (logged-in links vs.
// login/register), and to gate Dashboard/BookPage — which redirect to
// /login on failure, unlike the JSON API's middleware.Auth (a plain-text
// 401), since a browser hitting these page URLs directly should land
// somewhere useful, not a bare error.
func (h *Handler) authenticated(r *http.Request) bool {
	_, ok := h.claims(r)
	return ok
}

// Home sends an already-logged-in visitor straight to the page that matters
// for their role, rather than showing the marketing page again: patients go
// to their booking dashboard, staff (clinician/attendant/admin) go to the
// backoffice.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	claims, authed := h.claims(r)
	if authed {
		if claims.Role == auth.RolePatient {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
		return
	}
	h.tmpl.Render(w, http.StatusOK, "home", PageData{Title: "Welcome", Authenticated: authed})
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.Render(w, http.StatusOK, "register", PageData{Title: "Register", Authenticated: h.authenticated(r)})
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.Render(w, http.StatusOK, "login", PageData{Title: "Log in", Authenticated: h.authenticated(r)})
}

func (h *Handler) CheckEmailPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.Render(w, http.StatusOK, "check-email", PageData{Title: "Check your email", Authenticated: h.authenticated(r)})
}

func (h *Handler) ForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.Render(w, http.StatusOK, "forgot-password", PageData{Title: "Forgot password", Authenticated: h.authenticated(r)})
}

func (h *Handler) ResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	h.tmpl.Render(w, http.StatusOK, "reset-password", PageData{
		Title:         "Reset password",
		Authenticated: h.authenticated(r),
		Token:         r.URL.Query().Get("token"),
	})
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if !h.authenticated(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, http.StatusOK, "dashboard", PageData{Title: "Dashboard", Authenticated: true})
}

func (h *Handler) BookPage(w http.ResponseWriter, r *http.Request) {
	if !h.authenticated(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, http.StatusOK, "book", PageData{Title: "Book a service", Authenticated: true})
}
