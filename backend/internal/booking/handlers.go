package booking

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
	"clinicapp/backend/internal/session"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Availability serves the calendar view: GET /availability?service_id=X&date=YYYY-MM-DD.
// Open to any authenticated role (not just patients), same as GET /services —
// staff may want to check the calendar too, and the data isn't sensitive.
func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	slots, err := h.svc.Availability(r.Context(), r.URL.Query().Get("service_id"), r.URL.Query().Get("date"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(slots))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, s := range slots {
		items = append(items, map[string]any{
			"scheduled_at": s.ScheduledAt,
			"available":    s.Available,
		})
		status := "booked"
		if s.Available {
			status = "available"
		}
		htmlBuf.WriteString(fmt.Sprintf(`<li data-available="%t">%s (%s)</li>`, s.Available, html.EscapeString(s.ScheduledAt.Format(time.RFC3339)), status))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"slots": items},
		HTML:   htmlBuf.String(),
	})
}

type createBookingRequest struct {
	ServiceID   string    `json:"service_id"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	var req createBookingRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	sess, err := h.svc.Book(r.Context(), claims.UserID, CreateBookingInput{
		ServiceID:   req.ServiceID,
		ScheduledAt: req.ScheduledAt,
	})
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusCreated,
		JSON:   bookingJSON(sess),
		HTML:   bookingHTML(sess),
	})
}

// List serves the authenticated patient's own booking history.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	bookings, err := h.svc.ListOwn(r.Context(), claims.UserID)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(bookings))
	var htmlBuf strings.Builder
	htmlBuf.WriteString("<ul>")
	for _, sess := range bookings {
		items = append(items, bookingJSON(sess))
		htmlBuf.WriteString(bookingHTML(sess))
	}
	htmlBuf.WriteString("</ul>")

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   map[string]any{"bookings": items},
		HTML:   htmlBuf.String(),
	})
}

// Get serves a single booking belonging to the authenticated patient — a
// booking id that exists but belongs to someone else 404s, same as any
// other id that never existed.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFrom(r.Context())
	if !ok {
		renderError(w, r, http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	sess, err := h.svc.GetOwn(r.Context(), claims.UserID, r.PathValue("id"))
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   bookingJSON(sess),
		HTML:   bookingHTML(sess),
	})
}

// bookingJSON is a patient-facing view of a session — it deliberately omits
// CommissionSnapshot (consultant/clinic revenue split), which is internal
// business data with no place in the customer portal, unlike
// session.Handler's own sessionJSON used by admin/clinician routes.
func bookingJSON(s *session.Session) map[string]any {
	out := map[string]any{
		"id":           s.ID,
		"service_id":   s.ServiceID,
		"scheduled_at": s.ScheduledAt,
		"created_at":   s.CreatedAt,
	}
	if s.ConsultantID != nil {
		out["consultant_id"] = *s.ConsultantID
	}
	return out
}

func bookingHTML(s *session.Session) string {
	return fmt.Sprintf(`<li data-id="%s">%s</li>`, html.EscapeString(s.ID), html.EscapeString(s.ScheduledAt.Format(time.RFC3339)))
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
	case errors.Is(err, ErrServiceNotFound),
		errors.Is(err, ErrServiceInactive),
		errors.Is(err, ErrInvalidDate),
		errors.Is(err, ErrInvalidSlot),
		errors.Is(err, ErrPatientProfileMissing):
		return http.StatusBadRequest
	case errors.Is(err, ErrSlotUnavailable):
		return http.StatusConflict
	case errors.Is(err, session.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
