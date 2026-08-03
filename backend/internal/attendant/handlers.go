package attendant

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

type attendantRequest struct {
	UserID   string `json:"user_id"`
	FullName string `json:"full_name"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req attendantRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	a, err := h.svc.Create(r.Context(), req.UserID, req.FullName)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   attendantJSON(a),
		HTML:   attendantHTML(a),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	a, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   attendantJSON(a),
		HTML:   attendantHTML(a),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	attendants, err := h.svc.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(attendants))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, a := range attendants {
		items = append(items, attendantJSON(a))
		htmlBuf.WriteString(attendantHTML(a))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"attendants": items},
		HTML:   htmlBuf.String(),
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req attendantRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	a, err := h.svc.Update(r.Context(), r.PathValue("id"), req.FullName)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   attendantJSON(a),
		HTML:   attendantHTML(a),
	})
}

func attendantJSON(a *Attendant) map[string]any {
	return map[string]any{
		"id":         a.ID,
		"user_id":    a.UserID,
		"full_name":  a.FullName,
		"created_at": a.CreatedAt,
		"updated_at": a.UpdatedAt,
	}
}

func attendantHTML(a *Attendant) string {
	return fmt.Sprintf(`<li data-id="%s">%s</li>`, html.EscapeString(a.ID), html.EscapeString(a.FullName))
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
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidRole):
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
