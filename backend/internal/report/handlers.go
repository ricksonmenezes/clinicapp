package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"

	"clinicapp/backend/internal/renderer"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Revenue(w http.ResponseWriter, r *http.Request) {
	in, err := parseRangeQuery(r, true)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	points, start, end, err := h.svc.Revenue(r.Context(), in)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(points))
	labels := make([]string, 0, len(points))
	values := make([]float64, 0, len(points))
	for _, p := range points {
		items = append(items, map[string]any{"period": p.Period, "total": p.Total})
		labels = append(labels, p.Period.Format("2006-01-02"))
		values = append(values, p.Total)
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   rangeJSON(start, end, "group_by", in.GroupBy, "revenue", items),
		HTML:   chartHTML("Revenue", "line", "Revenue", labels, values),
	})
}

func (h *Handler) CommissionPayouts(w http.ResponseWriter, r *http.Request) {
	in, err := parseRangeQuery(r, false)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	payouts, start, end, err := h.svc.CommissionPayouts(r.Context(), in)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(payouts))
	labels := make([]string, 0, len(payouts))
	values := make([]float64, 0, len(payouts))
	for _, p := range payouts {
		items = append(items, map[string]any{
			"consultant_id":     p.ConsultantID,
			"consultant_name":   p.ConsultantName,
			"consultant_amount": p.ConsultantAmount,
			"clinic_amount":     p.ClinicAmount,
			"session_count":     p.SessionCount,
		})
		labels = append(labels, p.ConsultantName)
		values = append(values, p.ConsultantAmount)
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   rangeJSON(start, end, "", "", "commission_payouts", items),
		HTML:   chartHTML("Commission payouts", "bar", "Consultant amount", labels, values),
	})
}

func (h *Handler) ServicePopularity(w http.ResponseWriter, r *http.Request) {
	in, err := parseRangeQuery(r, false)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	popularity, start, end, err := h.svc.ServicePopularity(r.Context(), in)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(popularity))
	labels := make([]string, 0, len(popularity))
	values := make([]float64, 0, len(popularity))
	for _, p := range popularity {
		items = append(items, map[string]any{
			"service_id":    p.ServiceID,
			"service_name":  p.ServiceName,
			"session_count": p.SessionCount,
		})
		labels = append(labels, p.ServiceName)
		values = append(values, float64(p.SessionCount))
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   rangeJSON(start, end, "", "", "service_popularity", items),
		HTML:   chartHTML("Service popularity", "bar", "Sessions", labels, values),
	})
}

func (h *Handler) BookingVolume(w http.ResponseWriter, r *http.Request) {
	in, err := parseRangeQuery(r, true)
	if err != nil {
		renderError(w, r, http.StatusBadRequest, err)
		return
	}

	points, start, end, err := h.svc.BookingVolume(r.Context(), in)
	if err != nil {
		renderError(w, r, statusForError(err), err)
		return
	}

	items := make([]any, 0, len(points))
	labels := make([]string, 0, len(points))
	values := make([]float64, 0, len(points))
	for _, p := range points {
		items = append(items, map[string]any{"period": p.Period, "count": p.Count})
		labels = append(labels, p.Period.Format("2006-01-02"))
		values = append(values, float64(p.Count))
	}

	renderer.Render(w, r, renderer.Response{
		Status: http.StatusOK,
		JSON:   rangeJSON(start, end, "group_by", in.GroupBy, "bookings", items),
		HTML:   chartHTML("Bookings", "line", "Sessions", labels, values),
	})
}

// rangeJSON assembles the common start/end envelope every report response
// shares, plus the report-specific key/items and an optional extra
// key/value (group_by, when the report is a time series).
func rangeJSON(start, end time.Time, extraKey, extraVal, itemsKey string, items []any) map[string]any {
	out := map[string]any{
		"start":  start,
		"end":    end,
		itemsKey: items,
	}
	if extraKey != "" {
		out[extraKey] = extraVal
	}
	return out
}

// parseRangeQuery reads start/end/group_by query params. group_by is only
// meaningful for the two time-series reports (revenue, bookings) — the
// caller signals that with requiresGroupBy so an accidental group_by on the
// other two reports is silently ignored rather than erroring.
func parseRangeQuery(r *http.Request, allowGroupBy bool) (RangeInput, error) {
	var in RangeInput

	if v := r.URL.Query().Get("start"); v != "" {
		t, err := parseDateParam(v)
		if err != nil {
			return in, fmt.Errorf("invalid start: %w", err)
		}
		in.Start = &t
	}
	if v := r.URL.Query().Get("end"); v != "" {
		t, err := parseDateParam(v)
		if err != nil {
			return in, fmt.Errorf("invalid end: %w", err)
		}
		in.End = &t
	}
	if allowGroupBy {
		in.GroupBy = r.URL.Query().Get("group_by")
	}
	return in, nil
}

func parseDateParam(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD, got %q", v)
}

// chartHTML renders a self-contained fragment: a canvas plus the Chart.js
// CDN tag and an inline script that reads the immediately-preceding canvas
// via closest('.report-chart') — deliberately not relying on a unique
// element id, since more than one report widget can land on the same page.
// All dynamic values are passed through json.Marshal, which HTML-escapes
// <, >, & by default, so this is safe to embed directly in a <script> tag.
func chartHTML(title, chartType, datasetLabel string, labels []string, values []float64) string {
	data := struct {
		Labels []string  `json:"labels"`
		Values []float64 `json:"values"`
	}{Labels: labels, Values: values}
	if data.Labels == nil {
		data.Labels = []string{}
	}
	if data.Values == nil {
		data.Values = []float64{}
	}

	payload, _ := json.Marshal(data)
	chartTypeJSON, _ := json.Marshal(chartType)
	datasetLabelJSON, _ := json.Marshal(datasetLabel)

	return fmt.Sprintf(`<div class="report-chart">
<h3>%s</h3>
<canvas></canvas>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<script>
(function() {
  var data = %s;
  var canvas = document.currentScript.closest('.report-chart').querySelector('canvas');
  new Chart(canvas.getContext('2d'), {
    type: %s,
    data: { labels: data.labels, datasets: [{ label: %s, data: data.values, backgroundColor: 'rgba(54,162,235,0.5)', borderColor: 'rgba(54,162,235,1)', borderWidth: 1 }] },
    options: { responsive: true, scales: { y: { beginAtZero: true } } }
  });
})();
</script>
</div>`, html.EscapeString(title), payload, chartTypeJSON, datasetLabelJSON)
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
	case errors.Is(err, ErrInvalidDateRange), errors.Is(err, ErrInvalidGroupBy):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
