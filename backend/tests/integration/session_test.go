package integration

import (
	"net/http"
	"testing"
)

// sessionFixture creates a verified patient, a clinician consultant, an
// attendant, and a service, returning their ids for use in session tests.
func sessionFixture(t *testing.T, baseURL string, fakeMailer *FakeMailer, adminToken string, suffix string, defaultCommission float64, requiresConsultant bool) (patientID, consultantID, attendantID, serviceID string) {
	t.Helper()

	patientUserID := RegisterVerifiedPatient(t, baseURL, fakeMailer, "patient-"+suffix+"@example.com", "correcthorsebattery")
	var patientProfile map[string]any
	PostJSONAuth(t, baseURL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": patientUserID, "full_name": "Patient " + suffix,
	}, &patientProfile)
	patientID, _ = patientProfile["id"].(string)

	consultantUserID := RegisterStaffUser(t, baseURL, adminToken, "consultant-"+suffix+"@example.com", "correcthorsebattery", "clinician")
	var consultantProfile map[string]any
	PostJSONAuth(t, baseURL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": consultantUserID, "full_name": "Dr. " + suffix, "default_commission": defaultCommission,
	}, &consultantProfile)
	consultantID, _ = consultantProfile["id"].(string)

	attendantUserID := RegisterStaffUser(t, baseURL, adminToken, "attendant-"+suffix+"@example.com", "correcthorsebattery", "attendant")
	var attendantProfile map[string]any
	PostJSONAuth(t, baseURL, "/attendants", "mobile", adminToken, map[string]any{
		"user_id": attendantUserID, "full_name": "Nurse " + suffix,
	}, &attendantProfile)
	attendantID, _ = attendantProfile["id"].(string)

	var svc map[string]any
	PostJSONAuth(t, baseURL, "/services", "mobile", adminToken, map[string]any{
		"name": "Service " + suffix, "price": 500, "requires_consultant": requiresConsultant,
	}, &svc)
	serviceID, _ = svc["id"].(string)

	return patientID, consultantID, attendantID, serviceID
}

func TestSessionCreate_ConsultantDefaultCommission(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, attendantID, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "default", 25, true)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":    patientID,
		"service_id":    serviceID,
		"consultant_id": consultantID,
		"scheduled_at":  "2026-08-10T10:00:00Z",
		"attendant_ids": []string{attendantID},
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: want 201, got %d (%v)", resp.StatusCode, created)
	}

	snap, _ := created["commission_snapshot"].(map[string]any)
	if snap == nil {
		t.Fatalf("create session: expected commission_snapshot, got %v", created)
	}
	if snap["resolution_source"] != "consultant_default" {
		t.Fatalf("expected consultant_default, got %v", snap["resolution_source"])
	}
	if snap["commission_rate"] != float64(25) {
		t.Fatalf("expected commission_rate 25, got %v", snap["commission_rate"])
	}
	if snap["consultant_amount"] != float64(125) {
		t.Fatalf("expected consultant_amount 125, got %v", snap["consultant_amount"])
	}
	if snap["clinic_amount"] != float64(375) {
		t.Fatalf("expected clinic_amount 375, got %v", snap["clinic_amount"])
	}

	attendantIDs, _ := created["attendant_ids"].([]any)
	if len(attendantIDs) != 1 || attendantIDs[0] != attendantID {
		t.Fatalf("expected attendant_ids [%s], got %v", attendantID, attendantIDs)
	}
}

func TestSessionCreate_ServiceOverrideCommission(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "svcoverride", 25, true)

	resp := PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "commission": 40,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set service commission: want 200, got %d", resp.StatusCode)
	}

	var created map[string]any
	resp = PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":    patientID,
		"service_id":    serviceID,
		"consultant_id": consultantID,
		"scheduled_at":  "2026-08-10T10:00:00Z",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: want 201, got %d (%v)", resp.StatusCode, created)
	}

	snap, _ := created["commission_snapshot"].(map[string]any)
	if snap["resolution_source"] != "service_override" {
		t.Fatalf("expected service_override, got %v", snap["resolution_source"])
	}
	if snap["commission_rate"] != float64(40) {
		t.Fatalf("expected commission_rate 40, got %v", snap["commission_rate"])
	}
}

func TestSessionCreate_SessionOverrideCommissionWinsOverService(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "sessoverride", 25, true)

	PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "commission": 40,
	}, nil)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":          patientID,
		"service_id":          serviceID,
		"consultant_id":       consultantID,
		"scheduled_at":        "2026-08-10T10:00:00Z",
		"commission_override": 60,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: want 201, got %d (%v)", resp.StatusCode, created)
	}

	snap, _ := created["commission_snapshot"].(map[string]any)
	if snap["resolution_source"] != "session_override" {
		t.Fatalf("expected session_override, got %v", snap["resolution_source"])
	}
	if snap["commission_rate"] != float64(60) {
		t.Fatalf("expected commission_rate 60, got %v", snap["commission_rate"])
	}
}

func TestSessionCreate_AttendantOnlyServiceHasNoCommissionSnapshot(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, _, attendantID, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "attendantonly", 25, false)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":    patientID,
		"service_id":    serviceID,
		"scheduled_at":  "2026-08-10T10:00:00Z",
		"attendant_ids": []string{attendantID},
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if _, ok := created["commission_snapshot"]; ok {
		t.Fatalf("expected no commission_snapshot for attendant-only service, got %v", created["commission_snapshot"])
	}
	if _, ok := created["consultant_id"]; ok {
		t.Fatalf("expected no consultant_id, got %v", created["consultant_id"])
	}
}

func TestSessionCreate_RequiresConsultantWhenServiceDemandsIt(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, _, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "needconsult", 25, true)

	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":   patientID,
		"service_id":   serviceID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create session without consultant for requires_consultant service: want 400, got %d", resp.StatusCode)
	}
}

func TestSessionCreate_PackageSessionDecrementsRemainingAndBlocksWhenExhausted(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "package", 25, true)

	var pkg map[string]any
	PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "name": "2-pack", "session_count": 2, "price": 900,
	}, &pkg)
	packageID, _ := pkg["id"].(string)

	var subscribed map[string]any
	PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": patientID, "package_id": packageID,
	}, &subscribed)
	patientPackageID, _ := subscribed["id"].(string)

	for i := 0; i < 2; i++ {
		var created map[string]any
		resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
			"patient_id":         patientID,
			"service_id":         serviceID,
			"consultant_id":      consultantID,
			"patient_package_id": patientPackageID,
			"scheduled_at":       "2026-08-10T10:00:00Z",
		}, &created)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create package session %d: want 201, got %d (%v)", i, resp.StatusCode, created)
		}
	}

	var afterTwo map[string]any
	Get(t, ts.URL, "/patient-packages/"+patientPackageID, "mobile", adminToken, &afterTwo)
	if afterTwo["sessions_remaining"] != float64(0) {
		t.Fatalf("expected sessions_remaining 0 after 2 sessions, got %v", afterTwo["sessions_remaining"])
	}

	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":         patientID,
		"service_id":         serviceID,
		"consultant_id":      consultantID,
		"patient_package_id": patientPackageID,
		"scheduled_at":       "2026-08-10T10:00:00Z",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create session on exhausted package: want 400, got %d", resp.StatusCode)
	}
}

func TestSessionCreate_RejectsPatientPackageBelongingToAnotherPatient(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "mismatchowner", 25, true)
	otherPatientID, _, _, _ := sessionFixture(t, ts.URL, fakeMailer, adminToken, "mismatchother", 25, true)

	var pkg map[string]any
	PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "name": "1-pack", "session_count": 1, "price": 500,
	}, &pkg)
	packageID, _ := pkg["id"].(string)

	var subscribed map[string]any
	PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": otherPatientID, "package_id": packageID,
	}, &subscribed)
	patientPackageID, _ := subscribed["id"].(string)

	resp := PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id":         patientID,
		"service_id":         serviceID,
		"consultant_id":      consultantID,
		"patient_package_id": patientPackageID,
		"scheduled_at":       "2026-08-10T10:00:00Z",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create session with mismatched patient_package: want 400, got %d", resp.StatusCode)
	}
}

func TestSessionList_FiltersByPatientAndConsultant(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientA, consultantA, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "historya", 25, true)
	patientB, consultantB, _, _ := sessionFixture(t, ts.URL, fakeMailer, adminToken, "historyb", 25, true)

	PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id": patientA, "service_id": serviceID, "consultant_id": consultantA, "scheduled_at": "2026-08-10T10:00:00Z",
	}, nil)
	PostJSONAuth(t, ts.URL, "/sessions", "mobile", adminToken, map[string]any{
		"patient_id": patientB, "service_id": serviceID, "consultant_id": consultantB, "scheduled_at": "2026-08-11T10:00:00Z",
	}, nil)

	var byPatient map[string]any
	resp := Get(t, ts.URL, "/sessions?patient_id="+patientA, "mobile", adminToken, &byPatient)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list by patient: want 200, got %d", resp.StatusCode)
	}
	items, _ := byPatient["sessions"].([]any)
	if len(items) != 1 {
		t.Fatalf("list by patient: want 1, got %d (%v)", len(items), items)
	}

	var byConsultant map[string]any
	resp = Get(t, ts.URL, "/sessions?consultant_id="+consultantB, "mobile", adminToken, &byConsultant)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list by consultant: want 200, got %d", resp.StatusCode)
	}
	items, _ = byConsultant["sessions"].([]any)
	if len(items) != 1 {
		t.Fatalf("list by consultant: want 1, got %d (%v)", len(items), items)
	}

	var all map[string]any
	resp = Get(t, ts.URL, "/sessions", "mobile", adminToken, &all)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list all: want 200, got %d", resp.StatusCode)
	}
	items, _ = all["sessions"].([]any)
	if len(items) != 2 {
		t.Fatalf("list all: want 2, got %d (%v)", len(items), items)
	}
}
