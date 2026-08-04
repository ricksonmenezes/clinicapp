package backoffice

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

// claims reads and validates the access_token cookie — same mechanism
// internal/portal uses, duplicated rather than shared for the same reason
// portal's own check is kept local instead of generalizing
// middleware.Auth: page routes need role-aware HTML responses (redirect,
// 403 page), not the JSON API's plain-text 401/403.
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

// authorize is the shared gate every page handler calls first: an
// unauthenticated visitor is redirected to /login (a browser navigating
// here directly should land somewhere useful, not a bare 401); an
// authenticated visitor whose role isn't in allowed gets a rendered 403
// page — mirroring middleware.RequireRole's role table exactly, per page,
// just as HTML instead of a JSON error body.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, allowed ...string) (*auth.AccessClaims, bool) {
	claims, ok := h.claims(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil, false
	}
	for _, role := range allowed {
		if claims.Role == role {
			return claims, true
		}
	}
	h.tmpl.Render(w, http.StatusForbidden, "forbidden", PageData{Title: "Forbidden", Role: claims.Role})
	return nil, false
}

var (
	anyStaff         = []string{auth.RoleAdmin, auth.RoleClinician, auth.RoleAttendant}
	adminOnly        = []string{auth.RoleAdmin}
	adminOrClinician = []string{auth.RoleAdmin, auth.RoleClinician}
	clinicianOnly    = []string{auth.RoleClinician}
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, anyStaff...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "dashboard", PageData{Title: "Backoffice", Role: claims.Role})
}

// Patients: admin+clinician can view (matches GET /patients); only admin
// sees the create/edit forms (matches POST/PATCH /patients being
// admin-only) — the role gate here is read access, form visibility is a
// template-level check on .Role.
func (h *Handler) Patients(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOrClinician...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "patients", PageData{Title: "Patients", Role: claims.Role})
}

func (h *Handler) Consultants(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOnly...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "consultants", PageData{Title: "Consultants", Role: claims.Role})
}

func (h *Handler) Attendants(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOnly...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "attendants", PageData{Title: "Attendants", Role: claims.Role})
}

// Services: GET /services is open to any authenticated role, but the
// backoffice section is a staff tool (patients use /book instead), so the
// page itself is gated to staff roles; write forms are admin-only,
// enforced both by the template (Role check) and, ultimately, the
// underlying API.
func (h *Handler) Services(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, anyStaff...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "services", PageData{Title: "Services", Role: claims.Role})
}

func (h *Handler) Packages(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOrClinician...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "packages", PageData{Title: "Packages", Role: claims.Role})
}

func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOrClinician...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "sessions", PageData{Title: "Sessions", Role: claims.Role})
}

func (h *Handler) Invoices(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOrClinician...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "invoices", PageData{Title: "Invoices", Role: claims.Role})
}

// Prescriptions: clinician-only end to end, same as the API — admin gets
// a 403 page here too, not just clinician-vs-nobody.
func (h *Handler) Prescriptions(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, clinicianOnly...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "prescriptions", PageData{Title: "Prescriptions", Role: claims.Role})
}

func (h *Handler) Reports(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOnly...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "reports", PageData{Title: "Reports", Role: claims.Role})
}

func (h *Handler) StaffNew(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, adminOnly...)
	if !ok {
		return
	}
	h.tmpl.Render(w, http.StatusOK, "staff-new", PageData{Title: "New staff account", Role: claims.Role})
}
