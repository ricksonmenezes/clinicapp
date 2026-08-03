package integration

import (
	"net/http"
	"testing"
)

func TestPatientFlow_CreateListGetUpdate(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	userID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "patient1@example.com", "correcthorsebattery")

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id":   userID,
		"full_name": "Jane Doe",
		"dob":       "1990-05-15",
		"phone":     "+63 900 000 0000",
		"notes":     "Prefers morning appointments",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create patient: want 201, got %d (%v)", resp.StatusCode, created)
	}
	patientID, _ := created["id"].(string)
	if patientID == "" {
		t.Fatalf("create patient: expected id in response, got %v", created)
	}
	if created["dob"] != "1990-05-15" {
		t.Fatalf("create patient: expected dob 1990-05-15, got %v", created["dob"])
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/patients", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list patients: want 200, got %d", resp.StatusCode)
	}
	patients, _ := listResp["patients"].([]any)
	if len(patients) != 1 {
		t.Fatalf("list patients: want 1, got %d", len(patients))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/patients/"+patientID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get patient: want 200, got %d", resp.StatusCode)
	}
	if getResp["full_name"] != "Jane Doe" {
		t.Fatalf("get patient: expected full_name Jane Doe, got %v", getResp["full_name"])
	}

	var updateResp map[string]any
	resp = PatchJSON(t, ts.URL, "/patients/"+patientID, "mobile", adminToken, map[string]any{
		"full_name": "Jane R. Doe",
		"phone":     "+63 911 111 1111",
	}, &updateResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update patient: want 200, got %d (%v)", resp.StatusCode, updateResp)
	}
	if updateResp["full_name"] != "Jane R. Doe" {
		t.Fatalf("update patient: expected updated full_name, got %v", updateResp["full_name"])
	}
}

func TestPatientCreate_RequiresPatientRoleUser(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	clinicianID := RegisterStaffUser(t, ts.URL, adminToken, "wrongrole@example.com", "correcthorsebattery", "clinician")
	_ = fakeMailer

	resp := PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id":   clinicianID,
		"full_name": "Should Fail",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create patient for clinician-role user: want 400, got %d", resp.StatusCode)
	}
}

func TestPatientCreate_UnknownUserRejected(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id":   "00000000-0000-0000-0000-000000000000",
		"full_name": "Nobody",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create patient for unknown user: want 400, got %d", resp.StatusCode)
	}
}

func TestPatientRoutes_NonAdminNonClinicianForbidden(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	userID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "onlypatient@example.com", "correcthorsebattery")

	var loginResp map[string]any
	PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": "onlypatient@example.com", "password": "correcthorsebattery",
	}, &loginResp)
	patientToken, _ := loginResp["access_token"].(string)

	if resp := Get(t, ts.URL, "/patients", "mobile", patientToken, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("list patients as patient role: want 403, got %d", resp.StatusCode)
	}

	// Sanity: admin can still create using this same patient's user id.
	resp := PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id":   userID,
		"full_name": "Only Patient",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin create patient: want 201, got %d", resp.StatusCode)
	}
}
