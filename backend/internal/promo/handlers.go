package promo

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
	packageMgr        *PackageManager
	patientPackageMgr *PatientPackageManager
}

func NewHandler(packageMgr *PackageManager, patientPackageMgr *PatientPackageManager) *Handler {
	return &Handler{packageMgr: packageMgr, patientPackageMgr: patientPackageMgr}
}

type packageRequest struct {
	ServiceID    string  `json:"service_id"`
	Name         string  `json:"name"`
	SessionCount int     `json:"session_count"`
	Price        float64 `json:"price"`
	Active       *bool   `json:"active"`
}

func (h *Handler) CreatePackage(w http.ResponseWriter, r *http.Request) {
	var req packageRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	p, err := h.packageMgr.Create(r.Context(), req.ServiceID, req.Name, req.SessionCount, req.Price, active)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   packageJSON(p),
		HTML:   packageHTML(p),
	})
}

func (h *Handler) GetPackage(w http.ResponseWriter, r *http.Request) {
	p, err := h.packageMgr.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   packageJSON(p),
		HTML:   packageHTML(p),
	})
}

func (h *Handler) ListPackages(w http.ResponseWriter, r *http.Request) {
	packages, err := h.packageMgr.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(packages))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, p := range packages {
		items = append(items, packageJSON(p))
		htmlBuf.WriteString(packageHTML(p))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"packages": items},
		HTML:   htmlBuf.String(),
	})
}

func (h *Handler) UpdatePackage(w http.ResponseWriter, r *http.Request) {
	var req packageRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	p, err := h.packageMgr.Update(r.Context(), r.PathValue("id"), req.Name, req.SessionCount, req.Price, active)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   packageJSON(p),
		HTML:   packageHTML(p),
	})
}

type patientPackageRequest struct {
	PatientID           string  `json:"patient_id"`
	PackageID           string  `json:"package_id"`
	PrincipalConsultant *string `json:"principal_consultant"`
}

func (h *Handler) CreatePatientPackage(w http.ResponseWriter, r *http.Request) {
	var req patientPackageRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	pp, err := h.patientPackageMgr.Subscribe(r.Context(), req.PatientID, req.PackageID, req.PrincipalConsultant)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   patientPackageJSON(pp),
		HTML:   patientPackageHTML(pp),
	})
}

func (h *Handler) GetPatientPackage(w http.ResponseWriter, r *http.Request) {
	pp, err := h.patientPackageMgr.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   patientPackageJSON(pp),
		HTML:   patientPackageHTML(pp),
	})
}

func (h *Handler) ListPatientPackages(w http.ResponseWriter, r *http.Request) {
	patientPackages, err := h.patientPackageMgr.List(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(patientPackages))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, pp := range patientPackages {
		items = append(items, patientPackageJSON(pp))
		htmlBuf.WriteString(patientPackageHTML(pp))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"patient_packages": items},
		HTML:   htmlBuf.String(),
	})
}

func packageJSON(p *Package) map[string]any {
	return map[string]any{
		"id":            p.ID,
		"service_id":    p.ServiceID,
		"name":          p.Name,
		"session_count": p.SessionCount,
		"price":         p.Price,
		"active":        p.Active,
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
	}
}

func packageHTML(p *Package) string {
	return fmt.Sprintf(`<li data-id="%s">%s (%d sessions, $%.2f)</li>`, html.EscapeString(p.ID), html.EscapeString(p.Name), p.SessionCount, p.Price)
}

func patientPackageJSON(pp *PatientPackage) map[string]any {
	out := map[string]any{
		"id":                 pp.ID,
		"patient_id":         pp.PatientID,
		"package_id":         pp.PackageID,
		"sessions_remaining": pp.SessionsRemaining,
		"purchased_at":       pp.PurchasedAt,
		"created_at":         pp.CreatedAt,
		"updated_at":         pp.UpdatedAt,
	}
	if pp.PrincipalConsultant != nil {
		out["principal_consultant"] = *pp.PrincipalConsultant
	}
	return out
}

func patientPackageHTML(pp *PatientPackage) string {
	return fmt.Sprintf(`<li data-id="%s">%d sessions remaining</li>`, html.EscapeString(pp.ID), pp.SessionsRemaining)
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
	case errors.Is(err, ErrServiceNotFound), errors.Is(err, ErrPatientNotFound), errors.Is(err, ErrConsultantNotFound), errors.Is(err, ErrInvalidPackageID):
		return http.StatusBadRequest
	case errors.Is(err, ErrPackageNotFound), errors.Is(err, ErrPatientPackageNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
