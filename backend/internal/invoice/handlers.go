package invoice

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

type createInvoiceRequest struct {
	SessionID *string `json:"session_id"`
	PackageID *string `json:"package_id"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	inv, err := h.svc.Create(r.Context(), CreateInput{SessionID: req.SessionID, PackageID: req.PackageID})
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   invoiceJSON(inv),
		HTML:   invoiceHTML(inv),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	inv, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   invoiceJSON(inv),
		HTML:   invoiceHTML(inv),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	invoices, err := h.svc.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(invoices))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, inv := range invoices {
		items = append(items, invoiceJSON(inv))
		htmlBuf.WriteString(invoiceHTML(inv))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"invoices": items},
		HTML:   htmlBuf.String(),
	})
}

// DownloadPDF serves the raw PDF bytes regardless of X-Client-Type — a
// binary download isn't something the web/mobile renderer dispatch applies
// to, unlike every other endpoint in this codebase.
func (h *Handler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	inv, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}
	if inv.PDFPath == nil {
		renderError(w, r, http.StatusNotFound, ErrNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="invoice-%s.pdf"`, inv.ID))
	http.ServeFile(w, r, *inv.PDFPath)
}

func (h *Handler) ListPlaceholders(w http.ResponseWriter, r *http.Request) {
	placeholders, err := h.svc.ListPlaceholders(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(placeholders))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, p := range placeholders {
		items = append(items, placeholderJSON(p))
		htmlBuf.WriteString(placeholderHTML(p))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"placeholders": items},
		HTML:   htmlBuf.String(),
	})
}

type placeholderRequest struct {
	Value string `json:"value"`
}

func (h *Handler) SetPlaceholder(w http.ResponseWriter, r *http.Request) {
	var req placeholderRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	p, err := h.svc.SetPlaceholder(r.Context(), r.PathValue("key"), req.Value)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   placeholderJSON(p),
		HTML:   placeholderHTML(p),
	})
}

func invoiceJSON(inv *Invoice) map[string]any {
	out := map[string]any{
		"id":            inv.ID,
		"patient_id":    inv.PatientID,
		"total":         inv.Total,
		"issued_at":     inv.IssuedAt,
		"pdf_available": inv.PDFPath != nil,
	}
	if inv.SessionID != nil {
		out["session_id"] = *inv.SessionID
	}
	if inv.PackageID != nil {
		out["package_id"] = *inv.PackageID
	}
	return out
}

func invoiceHTML(inv *Invoice) string {
	return fmt.Sprintf(`<li data-id="%s">$%.2f — %s</li>`, html.EscapeString(inv.ID), inv.Total, inv.IssuedAt.Format("2006-01-02"))
}

func placeholderJSON(p *TemplatePlaceholder) map[string]any {
	return map[string]any{
		"id":         p.ID,
		"key":        p.Key,
		"value":      p.Value,
		"updated_at": p.UpdatedAt,
	}
}

func placeholderHTML(p *TemplatePlaceholder) string {
	return fmt.Sprintf(`<li data-key="%s">%s</li>`, html.EscapeString(p.Key), html.EscapeString(p.Value))
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
		errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrPatientPackageNotFound),
		errors.Is(err, ErrPlaceholderKeyRequired):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
