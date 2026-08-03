package patient

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"clinicapp/backend/internal/middleware"
	"clinicapp/backend/internal/renderer"
)

const dobLayout = "2006-01-02"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type patientRequest struct {
	UserID   string `json:"user_id"`
	FullName string `json:"full_name"`
	DOB      string `json:"dob"`
	Phone    string `json:"phone"`
	Notes    string `json:"notes"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req patientRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	dob, err := parseDOB(req.DOB)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	p, err := h.svc.Create(r.Context(), req.UserID, req.FullName, dob, optionalString(req.Phone), optionalString(req.Notes))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   patientJSON(p),
		HTML:   patientHTML(p),
	})
}

// CreateSelf lets a logged-in patient create their own profile — used by
// the customer portal registration flow. Unlike Create (admin-only,
// caller-supplied user_id), the profile is always created for the
// authenticated caller's own user id, never a body-supplied one, so a
// patient can't create a profile for someone else.
func (h *Handler) CreateSelf(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	var req patientRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	dob, err := parseDOB(req.DOB)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	p, err := h.svc.Create(r.Context(), claims.UserID, req.FullName, dob, optionalString(req.Phone), optionalString(req.Notes))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   patientJSON(p),
		HTML:   patientHTML(p),
	})
}

// GetSelf returns the authenticated patient's own profile.
func (h *Handler) GetSelf(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	p, err := h.svc.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   patientJSON(p),
		HTML:   patientHTML(p),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   patientJSON(p),
		HTML:   patientHTML(p),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	patients, err := h.svc.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(patients))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, p := range patients {
		items = append(items, patientJSON(p))
		htmlBuf.WriteString(patientHTML(p))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"patients": items},
		HTML:   htmlBuf.String(),
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req patientRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	dob, err := parseDOB(req.DOB)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	p, err := h.svc.Update(r.Context(), r.PathValue("id"), req.FullName, dob, optionalString(req.Phone), optionalString(req.Notes))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   patientJSON(p),
		HTML:   patientHTML(p),
	})
}

func patientJSON(p *Patient) map[string]any {
	out := map[string]any{
		"id":         p.ID,
		"user_id":    p.UserID,
		"full_name":  p.FullName,
		"created_at": p.CreatedAt,
		"updated_at": p.UpdatedAt,
	}
	if p.DOB != nil {
		out["dob"] = p.DOB.Format(dobLayout)
	}
	if p.Phone != nil {
		out["phone"] = *p.Phone
	}
	if p.Notes != nil {
		out["notes"] = *p.Notes
	}
	return out
}

func patientHTML(p *Patient) string {
	return fmt.Sprintf(`<li data-id="%s">%s</li>`, html.EscapeString(p.ID), html.EscapeString(p.FullName))
}

func parseDOB(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(dobLayout, s)
	if err != nil {
		return nil, ErrInvalidDOB
	}
	return &t, nil
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
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
		HTML:   fmt.Sprintf(`<p class="error">%s</p>`, html.EscapeString(err.Error())),
	})
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidDOB), errors.Is(err, ErrInvalidRole):
		return http.StatusBadRequest
	case errors.Is(err, ErrUserNotFound):
		return http.StatusBadRequest
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
