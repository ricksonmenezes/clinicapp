package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"clinicapp/backend/internal/renderer"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type sessionRequest struct {
	PatientID          string     `json:"patient_id"`
	ServiceID          string     `json:"service_id"`
	PatientPackageID   *string    `json:"patient_package_id"`
	ScheduledAt        time.Time  `json:"scheduled_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	ConsultantID       *string    `json:"consultant_id"`
	Notes              *string    `json:"notes"`
	AttendantIDs       []string   `json:"attendant_ids"`
	CommissionOverride *float64   `json:"commission_override"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	sess, err := h.svc.Create(r.Context(), CreateInput{
		PatientID:          req.PatientID,
		ServiceID:          req.ServiceID,
		PatientPackageID:   req.PatientPackageID,
		ScheduledAt:        req.ScheduledAt,
		CompletedAt:        req.CompletedAt,
		ConsultantID:       req.ConsultantID,
		Notes:              req.Notes,
		AttendantIDs:       req.AttendantIDs,
		CommissionOverride: req.CommissionOverride,
	})
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   sessionJSON(sess),
		HTML:   sessionHTML(sess),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   sessionJSON(sess),
		HTML:   sessionHTML(sess),
	})
}

// List serves session history — filterable by patient_id or consultant_id
// (per PLAN.md Module 5's "session history per patient and per consultant"),
// with no query params it returns every session.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{
		PatientID:    r.URL.Query().Get("patient_id"),
		ConsultantID: r.URL.Query().Get("consultant_id"),
	}

	sessions, err := h.svc.List(r.Context(), filter)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, err)
		return
	}

	items := make([]any, 0, len(sessions))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, sess := range sessions {
		items = append(items, sessionJSON(sess))
		htmlBuf.WriteString(sessionHTML(sess))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"sessions": items},
		HTML:   htmlBuf.String(),
	})
}

func sessionJSON(s *Session) map[string]any {
	out := map[string]any{
		"id":            s.ID,
		"patient_id":    s.PatientID,
		"service_id":    s.ServiceID,
		"scheduled_at":  s.ScheduledAt,
		"attendant_ids": s.AttendantIDs,
		"created_at":    s.CreatedAt,
	}
	if s.PatientPackageID != nil {
		out["patient_package_id"] = *s.PatientPackageID
	}
	if s.CompletedAt != nil {
		out["completed_at"] = *s.CompletedAt
	}
	if s.ConsultantID != nil {
		out["consultant_id"] = *s.ConsultantID
	}
	if s.Notes != nil {
		out["notes"] = *s.Notes
	}
	if s.CommissionSnapshot != nil {
		snap := s.CommissionSnapshot
		out["commission_snapshot"] = map[string]any{
			"id":                snap.ID,
			"consultant_id":     snap.ConsultantID,
			"commission_rate":   snap.CommissionRate,
			"resolution_source": snap.ResolutionSource,
			"clinic_amount":     snap.ClinicAmount,
			"consultant_amount": snap.ConsultantAmount,
			"created_at":        snap.CreatedAt,
		}
	}
	return out
}

func sessionHTML(s *Session) string {
	commission := ""
	if s.CommissionSnapshot != nil {
		commission = fmt.Sprintf(" — commission %.2f%% (%s)", s.CommissionSnapshot.CommissionRate, html.EscapeString(s.CommissionSnapshot.ResolutionSource))
	}
	return fmt.Sprintf(`<li data-id="%s">%s%s</li>`, html.EscapeString(s.ID), html.EscapeString(s.ScheduledAt.Format(time.RFC3339)), commission)
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
		errors.Is(err, ErrConsultantRequired),
		errors.Is(err, ErrInvalidCommissionOverride),
		errors.Is(err, ErrPatientPackageMismatch),
		errors.Is(err, ErrNoSessionsRemaining),
		errors.Is(err, ErrPatientNotFound),
		errors.Is(err, ErrServiceNotFound),
		errors.Is(err, ErrConsultantNotFound),
		errors.Is(err, ErrAttendantNotFound),
		errors.Is(err, ErrPatientPackageNotFound):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
