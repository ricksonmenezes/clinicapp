package integration

import (
	"net/http"
	"testing"
)

func TestConsultantFlow_CreateListGetUpdate(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	userID := RegisterStaffUser(t, ts.URL, adminToken, "consultant1@example.com", "correcthorsebattery", "clinician")

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id":            userID,
		"full_name":          "Dr. Alex Cruz",
		"default_commission": 35.5,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create consultant: want 201, got %d (%v)", resp.StatusCode, created)
	}
	consultantID, _ := created["id"].(string)
	if consultantID == "" {
		t.Fatalf("create consultant: expected id in response, got %v", created)
	}
	if created["default_commission"] != 35.5 {
		t.Fatalf("create consultant: expected default_commission 35.5, got %v", created["default_commission"])
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/consultants", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list consultants: want 200, got %d", resp.StatusCode)
	}
	consultants, _ := listResp["consultants"].([]any)
	if len(consultants) != 1 {
		t.Fatalf("list consultants: want 1, got %d", len(consultants))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/consultants/"+consultantID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get consultant: want 200, got %d", resp.StatusCode)
	}
	if getResp["full_name"] != "Dr. Alex Cruz" {
		t.Fatalf("get consultant: expected full_name Dr. Alex Cruz, got %v", getResp["full_name"])
	}

	var updateResp map[string]any
	resp = PatchJSON(t, ts.URL, "/consultants/"+consultantID, "mobile", adminToken, map[string]any{
		"full_name":          "Dr. Alex M. Cruz",
		"default_commission": 40,
	}, &updateResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update consultant: want 200, got %d (%v)", resp.StatusCode, updateResp)
	}
	if updateResp["default_commission"] != float64(40) {
		t.Fatalf("update consultant: expected default_commission 40, got %v", updateResp["default_commission"])
	}
}

func TestConsultantCreate_RejectsOutOfRangeCommission(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	userID := RegisterStaffUser(t, ts.URL, adminToken, "consultant2@example.com", "correcthorsebattery", "clinician")

	resp := PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id":            userID,
		"full_name":          "Dr. Out Of Range",
		"default_commission": 150,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create consultant with commission > 100: want 400, got %d", resp.StatusCode)
	}
}

func TestConsultantCreate_RequiresClinicianRoleUser(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "notaclinician@example.com", "correcthorsebattery")

	resp := PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id":            patientID,
		"full_name":          "Should Fail",
		"default_commission": 10,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create consultant for patient-role user: want 400, got %d", resp.StatusCode)
	}
}

func TestConsultantRoutes_ClinicianForbidden(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	RegisterStaffUser(t, ts.URL, adminToken, "clinicianviewer@example.com", "correcthorsebattery", "clinician")

	var loginResp map[string]any
	PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": "clinicianviewer@example.com", "password": "correcthorsebattery",
	}, &loginResp)
	clinicianToken, _ := loginResp["access_token"].(string)

	// Unlike /patients, /consultants is admin-only per PLAN.md's API table —
	// a clinician gets 403 too.
	if resp := Get(t, ts.URL, "/consultants", "mobile", clinicianToken, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("list consultants as clinician role: want 403, got %d", resp.StatusCode)
	}
}
