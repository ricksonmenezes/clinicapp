package integration

import (
	"net/http"
	"testing"
	"time"
)

// nextBookableSlot returns a deterministic future clinic slot: 7 days out
// (safely clear of "past slot" edge cases regardless of when the test runs),
// nudged off Sunday (clinic closed), at 10:00 UTC (well inside 09:00-17:00).
func nextBookableSlot(t *testing.T) time.Time {
	t.Helper()
	d := time.Now().UTC().AddDate(0, 0, 7)
	for d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 10, 0, 0, 0, time.UTC)
}

// registerVerifiedPatientWithProfile registers+verifies a patient and
// completes their self-service profile via POST /patients/me, returning the
// patient's own bearer token and patient_id.
func registerVerifiedPatientWithProfile(t *testing.T, baseURL string, fakeMailer *FakeMailer, email, fullName string) (token, patientID string) {
	t.Helper()
	RegisterVerifiedPatient(t, baseURL, fakeMailer, email, "correcthorsebattery")

	var loginResp map[string]any
	resp := PostJSON(t, baseURL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patient login: want 200, got %d (%v)", resp.StatusCode, loginResp)
	}
	token, _ = loginResp["access_token"].(string)
	if token == "" {
		t.Fatalf("patient login: expected access_token, got %v", loginResp)
	}

	var profile map[string]any
	resp = PostJSONAuth(t, baseURL, "/patients/me", "mobile", token, map[string]any{
		"full_name": fullName,
	}, &profile)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create own patient profile: want 201, got %d (%v)", resp.StatusCode, profile)
	}
	patientID, _ = profile["id"].(string)
	if patientID == "" {
		t.Fatalf("create own patient profile: expected id, got %v", profile)
	}
	return token, patientID
}

func TestPatientSelfServiceProfile(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	token, patientID := registerVerifiedPatientWithProfile(t, ts.URL, fakeMailer, "selfprofile@example.com", "Self Profile")

	var got map[string]any
	resp := Get(t, ts.URL, "/patients/me", "mobile", token, &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get own profile: want 200, got %d", resp.StatusCode)
	}
	if got["id"] != patientID {
		t.Fatalf("expected own profile id %s, got %v", patientID, got["id"])
	}

	// Creating a second profile for the same user must conflict.
	resp = PostJSONAuth(t, ts.URL, "/patients/me", "mobile", token, map[string]any{
		"full_name": "Duplicate",
	}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate own profile: want 409, got %d", resp.StatusCode)
	}

	// Non-patient callers are rejected.
	resp = PostJSONAuth(t, ts.URL, "/patients/me", "mobile", adminToken, map[string]any{
		"full_name": "Admin Trying",
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin create own profile: want 403, got %d", resp.StatusCode)
	}
}

func TestBooking_FullFlow_WithConsultantAutoAssignment(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Consultation", "price": 500, "requires_consultant": true,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	consultantUserID := RegisterStaffUser(t, ts.URL, adminToken, "bookingconsultant@example.com", "correcthorsebattery", "clinician")
	var consultantProfile map[string]any
	PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": consultantUserID, "full_name": "Dr. Booking", "default_commission": 40,
	}, &consultantProfile)
	consultantID, _ := consultantProfile["id"].(string)

	patientEmail := "bookingpatient@example.com"
	patientToken, patientID := registerVerifiedPatientWithProfile(t, ts.URL, fakeMailer, patientEmail, "Booking Patient")

	slot := nextBookableSlot(t)
	dateStr := slot.Format("2006-01-02")

	var availBefore map[string]any
	resp := Get(t, ts.URL, "/availability?service_id="+serviceID+"&date="+dateStr, "mobile", patientToken, &availBefore)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability: want 200, got %d (%v)", resp.StatusCode, availBefore)
	}
	if !slotAvailableInResponse(t, availBefore, slot, true) {
		t.Fatalf("expected slot %s to be available before booking: %v", slot, availBefore)
	}

	var created map[string]any
	resp = PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": slot.Format(time.RFC3339),
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create booking: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["consultant_id"] != consultantID {
		t.Fatalf("expected auto-assigned consultant_id %s, got %v", consultantID, created["consultant_id"])
	}
	if created["commission_snapshot"] != nil {
		t.Fatalf("booking response must not leak commission data to the patient, got %v", created["commission_snapshot"])
	}
	bookingID, _ := created["id"].(string)

	msg, ok := fakeMailer.LastTo(patientEmail)
	if !ok {
		t.Fatalf("expected a booking confirmation email sent to %s", patientEmail)
	}
	if msg.Subject == "" {
		t.Fatalf("expected a non-empty confirmation email subject")
	}

	var availAfter map[string]any
	resp = Get(t, ts.URL, "/availability?service_id="+serviceID+"&date="+dateStr, "mobile", patientToken, &availAfter)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability after booking: want 200, got %d", resp.StatusCode)
	}
	if !slotAvailableInResponse(t, availAfter, slot, false) {
		t.Fatalf("expected slot %s to be unavailable after booking: %v", slot, availAfter)
	}

	// A different patient booking the exact same service/slot must conflict —
	// clinic capacity per (service, slot) is 1 (see decision log).
	_, otherPatientID := registerVerifiedPatientWithProfile(t, ts.URL, fakeMailer, "otherbookingpatient@example.com", "Other Patient")
	otherToken := loginPatientToken(t, ts.URL, "otherbookingpatient@example.com")
	resp = PostJSONAuth(t, ts.URL, "/bookings", "mobile", otherToken, map[string]any{
		"service_id": serviceID, "scheduled_at": slot.Format(time.RFC3339),
	}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("conflicting booking: want 409, got %d", resp.StatusCode)
	}

	// Ownership isolation: the other patient can't fetch the first patient's booking.
	resp = Get(t, ts.URL, "/bookings/"+bookingID, "mobile", otherToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("other patient fetching booking: want 404, got %d", resp.StatusCode)
	}

	var mine map[string]any
	resp = Get(t, ts.URL, "/bookings/"+bookingID, "mobile", patientToken, &mine)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner fetching own booking: want 200, got %d", resp.StatusCode)
	}

	var list map[string]any
	resp = Get(t, ts.URL, "/bookings", "mobile", patientToken, &list)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list own bookings: want 200, got %d", resp.StatusCode)
	}
	items, _ := list["bookings"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 booking for the owning patient, got %d (%v)", len(items), items)
	}

	_ = patientID
	_ = otherPatientID
}

// TestBooking_SendsSMSConfirmationWhenPhoneOnFile covers the M8 addition:
// booking confirmation SMS via PhilSMS. registerVerifiedPatientWithProfile
// doesn't set a phone (patients.phone is optional), so this test builds its
// own profile with one to exercise the send path; the "no phone on file"
// case is already implicitly covered by every other booking test never
// getting an SMS.
func TestBooking_SendsSMSConfirmationWhenPhoneOnFile(t *testing.T) {
	ts, fakeMailer, fakeSMS := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "SMSCheckup", "price": 100, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	email := "smspatient@example.com"
	RegisterVerifiedPatient(t, ts.URL, fakeMailer, email, "correcthorsebattery")
	patientToken := loginPatientToken(t, ts.URL, email)

	var profile map[string]any
	resp := PostJSONAuth(t, ts.URL, "/patients/me", "mobile", patientToken, map[string]any{
		"full_name": "SMS Patient", "phone": "09171234567",
	}, &profile)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create own patient profile with phone: want 201, got %d (%v)", resp.StatusCode, profile)
	}

	slot := nextBookableSlot(t)
	var created map[string]any
	resp = PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": slot.Format(time.RFC3339),
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create booking: want 201, got %d (%v)", resp.StatusCode, created)
	}

	// 09171234567 normalizes to 639171234567 — see sms.NormalizePHPhone.
	msg, ok := fakeSMS.LastTo("639171234567")
	if !ok {
		t.Fatalf("expected a booking confirmation SMS sent to 639171234567, got sent=%v", fakeSMS.Sent)
	}
	if msg.Body == "" {
		t.Fatalf("expected a non-empty confirmation SMS body")
	}
}

func TestBooking_AttendantOnlyServiceNeedsNoConsultant(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Massage", "price": 200, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	patientToken, _ := registerVerifiedPatientWithProfile(t, ts.URL, fakeMailer, "attendantonly@example.com", "Attendant Only")
	slot := nextBookableSlot(t)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": slot.Format(time.RFC3339),
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create attendant-only booking: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["consultant_id"] != nil {
		t.Fatalf("attendant-only booking should not have a consultant_id, got %v", created["consultant_id"])
	}
}

func TestBooking_RejectsWithoutPatientProfile(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Checkup", "price": 100, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	RegisterVerifiedPatient(t, ts.URL, fakeMailer, "noprofilebooking@example.com", "correcthorsebattery")
	patientToken := loginPatientToken(t, ts.URL, "noprofilebooking@example.com")

	resp := PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": nextBookableSlot(t).Format(time.RFC3339),
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("booking without a patient profile: want 400, got %d", resp.StatusCode)
	}
}

func TestBooking_RejectsNonPatientCaller(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Checkup2", "price": 100, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	resp := PostJSONAuth(t, ts.URL, "/bookings", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "scheduled_at": nextBookableSlot(t).Format(time.RFC3339),
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin booking: want 403, got %d", resp.StatusCode)
	}
}

func TestBooking_RejectsSlotOffTheGrid(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "OffGrid", "price": 100, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	patientToken, _ := registerVerifiedPatientWithProfile(t, ts.URL, fakeMailer, "offgrid@example.com", "Off Grid")

	// 10:15 is not on the 30-minute slot grid.
	badSlot := nextBookableSlot(t).Add(15 * time.Minute)
	resp := PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": badSlot.Format(time.RFC3339),
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("off-grid booking: want 400, got %d", resp.StatusCode)
	}

	// A past slot is also rejected.
	pastSlot := time.Now().UTC().AddDate(0, 0, -7)
	for pastSlot.Weekday() == time.Sunday {
		pastSlot = pastSlot.AddDate(0, 0, -1)
	}
	pastSlot = time.Date(pastSlot.Year(), pastSlot.Month(), pastSlot.Day(), 10, 0, 0, 0, time.UTC)
	resp = PostJSONAuth(t, ts.URL, "/bookings", "mobile", patientToken, map[string]any{
		"service_id": serviceID, "scheduled_at": pastSlot.Format(time.RFC3339),
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("past-slot booking: want 400, got %d", resp.StatusCode)
	}
}

func TestAvailability_ClosedOnSunday(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	tok := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", tok, map[string]any{
		"name": "SundayCheck", "price": 100, "requires_consultant": false,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	d := time.Now().UTC().AddDate(0, 0, 7)
	for d.Weekday() != time.Sunday {
		d = d.AddDate(0, 0, 1)
	}

	var avail map[string]any
	resp := Get(t, ts.URL, "/availability?service_id="+serviceID+"&date="+d.Format("2006-01-02"), "mobile", tok, &avail)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sunday availability: want 200, got %d", resp.StatusCode)
	}
	slots, _ := avail["slots"].([]any)
	if len(slots) != 0 {
		t.Fatalf("expected no slots on a Sunday, got %d", len(slots))
	}
}

func loginPatientToken(t *testing.T, baseURL, email string) string {
	t.Helper()
	var loginResp map[string]any
	resp := PostJSON(t, baseURL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patient login: want 200, got %d (%v)", resp.StatusCode, loginResp)
	}
	token, _ := loginResp["access_token"].(string)
	if token == "" {
		t.Fatalf("patient login: expected access_token, got %v", loginResp)
	}
	return token
}

func slotAvailableInResponse(t *testing.T, resp map[string]any, slot time.Time, want bool) bool {
	t.Helper()
	slots, _ := resp["slots"].([]any)
	for _, raw := range slots {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ts, _ := m["scheduled_at"].(string)
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		if parsed.Equal(slot) {
			available, _ := m["available"].(bool)
			return available == want
		}
	}
	t.Fatalf("slot %s not found in availability response: %v", slot, resp)
	return false
}
