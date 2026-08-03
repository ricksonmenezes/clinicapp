package service

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
	mgr *Manager
}

func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

type serviceRequest struct {
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	Price              float64 `json:"price"`
	RequiresConsultant *bool   `json:"requires_consultant"`
	Active             *bool   `json:"active"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	requiresConsultant := true
	if req.RequiresConsultant != nil {
		requiresConsultant = *req.RequiresConsultant
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	s, err := h.mgr.Create(r.Context(), req.Name, req.Description, req.Price, requiresConsultant, active)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   serviceJSON(s),
		HTML:   serviceHTML(s),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.mgr.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   serviceJSON(s),
		HTML:   serviceHTML(s),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	services, err := h.mgr.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(services))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, s := range services {
		items = append(items, serviceJSON(s))
		htmlBuf.WriteString(serviceHTML(s))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"services": items},
		HTML:   htmlBuf.String(),
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	requiresConsultant := true
	if req.RequiresConsultant != nil {
		requiresConsultant = *req.RequiresConsultant
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	s, err := h.mgr.Update(r.Context(), r.PathValue("id"), req.Name, req.Description, req.Price, requiresConsultant, active)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   serviceJSON(s),
		HTML:   serviceHTML(s),
	})
}

func serviceJSON(s *Service) map[string]any {
	return map[string]any{
		"id":                  s.ID,
		"name":                s.Name,
		"description":         s.Description,
		"price":               s.Price,
		"requires_consultant": s.RequiresConsultant,
		"active":              s.Active,
		"created_at":          s.CreatedAt,
		"updated_at":          s.UpdatedAt,
	}
}

func serviceHTML(s *Service) string {
	return fmt.Sprintf(`<li data-id="%s">%s ($%.2f)</li>`, html.EscapeString(s.ID), html.EscapeString(s.Name), s.Price)
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
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
