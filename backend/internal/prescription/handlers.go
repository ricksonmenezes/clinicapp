package prescription

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	"clinicapp/backend/internal/middleware"
	"clinicapp/backend/internal/renderer"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createPrescriptionRequest struct {
	PatientID string  `json:"patient_id"`
	SessionID *string `json:"session_id"`
	Content   string  `json:"content"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	var req createPrescriptionRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	rx, err := h.svc.Create(r.Context(), claims.UserID, CreateInput{
		PatientID: req.PatientID,
		SessionID: req.SessionID,
		Content:   req.Content,
	})
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   prescriptionJSON(rx),
		HTML:   prescriptionHTML(rx),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rx, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   prescriptionJSON(rx),
		HTML:   prescriptionHTML(rx),
	})
}

// List serves Rx history — filterable by patient_id or consultant_id query
// params, same convention as GET /sessions.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	prescriptions, err := h.svc.List(r.Context(), ListFilter{
		PatientID:    r.URL.Query().Get("patient_id"),
		ConsultantID: r.URL.Query().Get("consultant_id"),
	})
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(prescriptions))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, rx := range prescriptions {
		items = append(items, prescriptionJSON(rx))
		htmlBuf.WriteString(prescriptionHTML(rx))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"prescriptions": items},
		HTML:   htmlBuf.String(),
	})
}

// DownloadPDF serves the raw PDF bytes regardless of X-Client-Type — a
// binary download isn't something the web/mobile renderer dispatch applies
// to, matching invoice.Handler.DownloadPDF.
func (h *Handler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	rx, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}
	if rx.PDFPath == nil {
		renderError(w, r, http.StatusNotFound, ErrNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="prescription-%s.pdf"`, rx.ID))
	http.ServeFile(w, r, *rx.PDFPath)
}

func prescriptionJSON(rx *Prescription) map[string]any {
	out := map[string]any{
		"id":            rx.ID,
		"consultant_id": rx.ConsultantID,
		"patient_id":    rx.PatientID,
		"content":       rx.Content,
		"issued_at":     rx.IssuedAt,
		"pdf_available": rx.PDFPath != nil,
	}
	if rx.SessionID != nil {
		out["session_id"] = *rx.SessionID
	}
	return out
}

func prescriptionHTML(rx *Prescription) string {
	return fmt.Sprintf(`<li data-id="%s">%s</li>`, html.EscapeString(rx.ID), rx.IssuedAt.Format("2006-01-02"))
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
	case errors.Is(err, ErrValidation),
		errors.Is(err, ErrConsultantProfileMissing),
		errors.Is(err, ErrPatientNotFound),
		errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrSessionPatientMismatch):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
