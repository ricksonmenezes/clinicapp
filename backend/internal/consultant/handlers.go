package consultant

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"clinicapp/backend/internal/renderer"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type consultantRequest struct {
	UserID            string  `json:"user_id"`
	FullName          string  `json:"full_name"`
	DefaultCommission float64 `json:"default_commission"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req consultantRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	c, err := h.svc.Create(r.Context(), req.UserID, req.FullName, req.DefaultCommission)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   consultantJSON(c),
		HTML:   consultantHTML(c),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   consultantJSON(c),
		HTML:   consultantHTML(c),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	consultants, err := h.svc.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(consultants))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, c := range consultants {
		items = append(items, consultantJSON(c))
		htmlBuf.WriteString(consultantHTML(c))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"consultants": items},
		HTML:   htmlBuf.String(),
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req consultantRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	c, err := h.svc.Update(r.Context(), r.PathValue("id"), req.FullName, req.DefaultCommission)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   consultantJSON(c),
		HTML:   consultantHTML(c),
	})
}

func consultantJSON(c *Consultant) map[string]any {
	return map[string]any{
		"id":                 c.ID,
		"user_id":            c.UserID,
		"full_name":          c.FullName,
		"default_commission": c.DefaultCommission,
		"created_at":         c.CreatedAt,
		"updated_at":         c.UpdatedAt,
	}
}

func consultantHTML(c *Consultant) string {
	return fmt.Sprintf(`<li data-id="%s">%s (%.2f%%)</li>`, html.EscapeString(c.ID), html.EscapeString(c.FullName), c.DefaultCommission)
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
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidCommission), errors.Is(err, ErrInvalidRole):
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
