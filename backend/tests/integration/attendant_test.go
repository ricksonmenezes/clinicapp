package integration

import (
	"net/http"
	"testing"
)

func TestAttendantFlow_CreateListGetUpdate(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	userID := RegisterStaffUser(t, ts.URL, adminToken, "attendant1@example.com", "correcthorsebattery", "attendant")

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/attendants", "mobile", adminToken, map[string]any{
		"user_id":   userID,
		"full_name": "Sam Rivera",
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create attendant: want 201, got %d (%v)", resp.StatusCode, created)
	}
	attendantID, _ := created["id"].(string)
	if attendantID == "" {
		t.Fatalf("create attendant: expected id in response, got %v", created)
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/attendants", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list attendants: want 200, got %d", resp.StatusCode)
	}
	attendants, _ := listResp["attendants"].([]any)
	if len(attendants) != 1 {
		t.Fatalf("list attendants: want 1, got %d", len(attendants))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/attendants/"+attendantID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get attendant: want 200, got %d", resp.StatusCode)
	}
	if getResp["full_name"] != "Sam Rivera" {
		t.Fatalf("get attendant: expected full_name Sam Rivera, got %v", getResp["full_name"])
	}

	var updateResp map[string]any
	resp = PatchJSON(t, ts.URL, "/attendants/"+attendantID, "mobile", adminToken, map[string]any{
		"full_name": "Samantha Rivera",
	}, &updateResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update attendant: want 200, got %d (%v)", resp.StatusCode, updateResp)
	}
	if updateResp["full_name"] != "Samantha Rivera" {
		t.Fatalf("update attendant: expected updated full_name, got %v", updateResp["full_name"])
	}
}

func TestAttendantCreate_RequiresAttendantRoleUser(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	clinicianID := RegisterStaffUser(t, ts.URL, adminToken, "notanattendant@example.com", "correcthorsebattery", "clinician")

	resp := PostJSONAuth(t, ts.URL, "/attendants", "mobile", adminToken, map[string]any{
		"user_id":   clinicianID,
		"full_name": "Should Fail",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create attendant for clinician-role user: want 400, got %d", resp.StatusCode)
	}
}

func TestAttendantRoutes_RequireAuth(t *testing.T) {
	ts, _ := NewTestServer(t)

	if resp := Get(t, ts.URL, "/attendants", "mobile", "", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("list attendants without token: want 401, got %d", resp.StatusCode)
	}
}
