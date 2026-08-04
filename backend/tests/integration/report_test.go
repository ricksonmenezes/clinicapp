package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReportRevenue_SumsInvoiceTotalsInRange(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "revenue", 25, true)

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)
	PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{"session_id": sessionID}, nil)

	var out map[string]any
	resp := Get(t, ts.URL, "/reports/revenue?start=2026-08-01&end=2026-08-15&group_by=day", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revenue report: want 200, got %d (%v)", resp.StatusCode, out)
	}
	items, _ := out["revenue"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 revenue bucket, got %d (%v)", len(items), items)
	}
	point, _ := items[0].(map[string]any)
	if point["total"] != float64(500) {
		t.Fatalf("expected total 500, got %v", point["total"])
	}
	if out["group_by"] != "day" {
		t.Fatalf("expected group_by day, got %v", out["group_by"])
	}
}

func TestReportRevenue_OutsideRangeExcluded(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "revenueoutside", 25, true)

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)
	PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{"session_id": sessionID}, nil)

	var out map[string]any
	resp := Get(t, ts.URL, "/reports/revenue?start=2026-09-01&end=2026-09-15", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revenue report: want 200, got %d (%v)", resp.StatusCode, out)
	}
	items, _ := out["revenue"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected 0 revenue buckets outside range, got %d (%v)", len(items), items)
	}
}

func TestReportCommissionPayouts_SumsPerConsultant(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "payout", 25, true)

	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-11T10:00:00Z",
	})

	var out map[string]any
	resp := Get(t, ts.URL, "/reports/commission-payouts?start=2026-08-01&end=2026-08-15", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("commission payouts report: want 200, got %d (%v)", resp.StatusCode, out)
	}
	items, _ := out["commission_payouts"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 consultant payout row, got %d (%v)", len(items), items)
	}
	payout, _ := items[0].(map[string]any)
	if payout["consultant_id"] != consultantID {
		t.Fatalf("expected consultant_id %s, got %v", consultantID, payout["consultant_id"])
	}
	// default_commission 25% of two 500-price sessions = 125 each = 250 total.
	if payout["consultant_amount"] != float64(250) {
		t.Fatalf("expected consultant_amount 250, got %v", payout["consultant_amount"])
	}
	if payout["clinic_amount"] != float64(750) {
		t.Fatalf("expected clinic_amount 750, got %v", payout["clinic_amount"])
	}
	if payout["session_count"] != float64(2) {
		t.Fatalf("expected session_count 2, got %v", payout["session_count"])
	}
}

func TestReportServicePopularity_CountsSessionsPerService(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "popularity", 25, true)

	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-11T10:00:00Z",
	})

	var out map[string]any
	resp := Get(t, ts.URL, "/reports/service-popularity?start=2026-08-01&end=2026-08-15", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("service popularity report: want 200, got %d (%v)", resp.StatusCode, out)
	}
	items, _ := out["service_popularity"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 service row, got %d (%v)", len(items), items)
	}
	row, _ := items[0].(map[string]any)
	if row["service_id"] != serviceID {
		t.Fatalf("expected service_id %s, got %v", serviceID, row["service_id"])
	}
	if row["session_count"] != float64(2) {
		t.Fatalf("expected session_count 2, got %v", row["session_count"])
	}
}

func TestReportBookingVolume_CountsSessionsByPeriod(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "volume", 25, true)

	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T09:00:00Z",
	})
	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T14:00:00Z",
	})

	var out map[string]any
	resp := Get(t, ts.URL, "/reports/bookings?start=2026-08-01&end=2026-08-15&group_by=day", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("booking volume report: want 200, got %d (%v)", resp.StatusCode, out)
	}
	items, _ := out["bookings"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 day bucket, got %d (%v)", len(items), items)
	}
	point, _ := items[0].(map[string]any)
	if point["count"] != float64(2) {
		t.Fatalf("expected count 2, got %v", point["count"])
	}
}

func TestReports_RejectInvalidGroupBy(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := Get(t, ts.URL, "/reports/revenue?group_by=quarter", "mobile", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid group_by: want 400, got %d", resp.StatusCode)
	}
}

func TestReports_RejectStartAfterEnd(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := Get(t, ts.URL, "/reports/bookings?start=2026-08-15&end=2026-08-01", "mobile", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("start after end: want 400, got %d", resp.StatusCode)
	}
}

func TestReports_AdminOnly(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	clinicianUserID := RegisterStaffUser(t, ts.URL, adminToken, "clinician-reports@example.com", "correcthorsebattery", "clinician")
	_ = fakeMailer
	var loginResp map[string]any
	PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": "clinician-reports@example.com", "password": "correcthorsebattery",
	}, &loginResp)
	clinicianToken, _ := loginResp["access_token"].(string)
	if clinicianToken == "" {
		t.Fatalf("clinician login: expected access_token, got %v (user %s)", loginResp, clinicianUserID)
	}

	for _, path := range []string{
		"/reports/revenue", "/reports/commission-payouts", "/reports/service-popularity", "/reports/bookings",
	} {
		resp := Get(t, ts.URL, path, "mobile", clinicianToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s as clinician: want 403, got %d", path, resp.StatusCode)
		}
	}
}

func TestReportRevenue_WebFragmentIncludesChart(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "chart", 25, true)

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)
	PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{"session_id": sessionID}, nil)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/reports/revenue?start=2026-08-01&end=2026-08-15", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Client-Type", "web")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("web revenue report: want 200, got %d", resp.StatusCode)
	}
	body := new(strings.Builder)
	if _, err := io.Copy(body, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := body.String()
	if !strings.Contains(html, "<canvas>") || !strings.Contains(html, "chart.js") || !strings.Contains(html, "new Chart(") {
		t.Fatalf("expected chart markup in web fragment, got %q", html)
	}
}
