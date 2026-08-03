package integration

import (
	"bytes"
	"net/http"
	"testing"
)

// clinicianFixture registers a verified patient, a clinician user with a
// consultant profile, and a service, then logs the clinician in — returning
// everything a prescription test needs including the clinician's own bearer
// token (prescriptions are authored as the caller, never a supplied
// consultant_id).
func clinicianFixture(t *testing.T, baseURL string, fakeMailer *FakeMailer, adminToken, suffix string) (patientID, consultantID, clinicianToken string) {
	t.Helper()

	patientUserID := RegisterVerifiedPatient(t, baseURL, fakeMailer, "rxpatient-"+suffix+"@example.com", "correcthorsebattery")
	var patientProfile map[string]any
	PostJSONAuth(t, baseURL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": patientUserID, "full_name": "Patient " + suffix,
	}, &patientProfile)
	patientID, _ = patientProfile["id"].(string)

	clinicianEmail := "rxclinician-" + suffix + "@example.com"
	clinicianPassword := "correcthorsebattery"
	clinicianUserID := RegisterStaffUser(t, baseURL, adminToken, clinicianEmail, clinicianPassword, "clinician")
	var consultantProfile map[string]any
	PostJSONAuth(t, baseURL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": clinicianUserID, "full_name": "Dr. " + suffix, "default_commission": 30,
	}, &consultantProfile)
	consultantID, _ = consultantProfile["id"].(string)

	var loginResp map[string]any
	resp := PostJSON(t, baseURL, "/auth/login", "mobile", map[string]string{
		"email": clinicianEmail, "password": clinicianPassword,
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clinician login: want 200, got %d (%v)", resp.StatusCode, loginResp)
	}
	clinicianToken, _ = loginResp["access_token"].(string)
	if clinicianToken == "" {
		t.Fatalf("clinician login: expected access_token, got %v", loginResp)
	}

	return patientID, consultantID, clinicianToken
}

func TestPrescriptionCreate_AsAuthoringClinician(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, clinicianToken := clinicianFixture(t, ts.URL, fakeMailer, adminToken, "author")

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianToken, map[string]any{
		"patient_id": patientID, "content": "Amoxicillin 500mg, 3x daily for 7 days",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create prescription: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["patient_id"] != patientID {
		t.Fatalf("expected patient_id %s, got %v", patientID, created["patient_id"])
	}
	if created["consultant_id"] != consultantID {
		t.Fatalf("expected consultant_id %s (the authenticated clinician's own profile), got %v", consultantID, created["consultant_id"])
	}
	if created["pdf_available"] != true {
		t.Fatalf("expected pdf_available true, got %v", created["pdf_available"])
	}
	rxID, _ := created["id"].(string)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/prescriptions/"+rxID+"/pdf", nil)
	if err != nil {
		t.Fatalf("build pdf request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+clinicianToken)
	req.Header.Set("X-Client-Type", "mobile")
	pdfResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download pdf: %v", err)
	}
	defer pdfResp.Body.Close()
	if pdfResp.StatusCode != http.StatusOK {
		t.Fatalf("download pdf: want 200, got %d", pdfResp.StatusCode)
	}
	if ct := pdfResp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %q", ct)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(pdfResp.Body)
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("expected PDF magic header, got %q", buf.Bytes()[:min(20, buf.Len())])
	}
}

func TestPrescriptionCreate_RejectsWithoutConsultantProfile(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	patientUserID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "rxpatient-noprofile@example.com", "correcthorsebattery")
	var patientProfile map[string]any
	PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": patientUserID, "full_name": "Patient NoProfile",
	}, &patientProfile)
	patientID, _ := patientProfile["id"].(string)

	// A clinician user with no consultant profile row — can happen if the
	// admin creates the login before filling in the profile.
	clinicianEmail := "rxclinician-noprofile@example.com"
	clinicianPassword := "correcthorsebattery"
	RegisterStaffUser(t, ts.URL, adminToken, clinicianEmail, clinicianPassword, "clinician")
	var loginResp map[string]any
	PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": clinicianEmail, "password": clinicianPassword,
	}, &loginResp)
	clinicianToken, _ := loginResp["access_token"].(string)

	resp := PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianToken, map[string]any{
		"patient_id": patientID, "content": "Rest and fluids",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create prescription without consultant profile: want 400, got %d", resp.StatusCode)
	}
}

func TestPrescriptionCreate_RejectsAdminCaller(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, _, _ := clinicianFixture(t, ts.URL, fakeMailer, adminToken, "adminreject")

	resp := PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", adminToken, map[string]any{
		"patient_id": patientID, "content": "Should be forbidden",
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("create prescription as admin: want 403, got %d", resp.StatusCode)
	}
}

func TestPrescriptionCreate_ValidatesSessionBelongsToPatient(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, clinicianToken := clinicianFixture(t, ts.URL, fakeMailer, adminToken, "sessioncheck")

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Consult", "price": 300, "requires_consultant": true,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)

	otherPatientUserID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "rxpatient-other@example.com", "correcthorsebattery")
	var otherPatientProfile map[string]any
	PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": otherPatientUserID, "full_name": "Other Patient",
	}, &otherPatientProfile)
	otherPatientID, _ := otherPatientProfile["id"].(string)

	resp := PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianToken, map[string]any{
		"patient_id": otherPatientID, "session_id": sessionID, "content": "Mismatched session",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create prescription with mismatched session/patient: want 400, got %d", resp.StatusCode)
	}

	var created map[string]any
	resp = PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianToken, map[string]any{
		"patient_id": patientID, "session_id": sessionID, "content": "Matching session",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create prescription with matching session: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["session_id"] != sessionID {
		t.Fatalf("expected session_id %s, got %v", sessionID, created["session_id"])
	}
}

func TestPrescriptionList_FiltersByPatientAndConsultant(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientAID, _, clinicianAToken := clinicianFixture(t, ts.URL, fakeMailer, adminToken, "lista")
	patientBID, consultantBID, clinicianBToken := clinicianFixture(t, ts.URL, fakeMailer, adminToken, "listb")

	PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianAToken, map[string]any{
		"patient_id": patientAID, "content": "Rx for A from A",
	}, nil)
	PostJSONAuth(t, ts.URL, "/prescriptions", "mobile", clinicianBToken, map[string]any{
		"patient_id": patientBID, "content": "Rx for B from B",
	}, nil)

	var byPatient map[string]any
	resp := Get(t, ts.URL, "/prescriptions?patient_id="+patientAID, "mobile", clinicianAToken, &byPatient)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list by patient: want 200, got %d", resp.StatusCode)
	}
	items, _ := byPatient["prescriptions"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 prescription for patient A, got %d (%v)", len(items), items)
	}

	var byConsultant map[string]any
	resp = Get(t, ts.URL, "/prescriptions?consultant_id="+consultantBID, "mobile", clinicianBToken, &byConsultant)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list by consultant: want 200, got %d", resp.StatusCode)
	}
	items, _ = byConsultant["prescriptions"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 prescription for consultant B, got %d (%v)", len(items), items)
	}
}
