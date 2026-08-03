package integration

import (
	"net/http"
	"testing"
)

func TestConsultantServiceCommission_SetListUpsert(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	userID := RegisterStaffUser(t, ts.URL, adminToken, "commissionconsultant@example.com", "correcthorsebattery", "clinician")
	var consultant map[string]any
	PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": userID, "full_name": "Dr. Override", "default_commission": 30,
	}, &consultant)
	consultantID, _ := consultant["id"].(string)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Facial", "price": 500,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	var setResp map[string]any
	resp := PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "commission": 45,
	}, &setResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set service commission: want 200, got %d (%v)", resp.StatusCode, setResp)
	}
	if setResp["commission"] != float64(45) {
		t.Fatalf("set service commission: expected 45, got %v", setResp["commission"])
	}

	// Upsert: setting again for the same consultant/service overwrites, not duplicates.
	resp = PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "commission": 50,
	}, &setResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-set service commission: want 200, got %d (%v)", resp.StatusCode, setResp)
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list service commissions: want 200, got %d", resp.StatusCode)
	}
	overrides, _ := listResp["service_commissions"].([]any)
	if len(overrides) != 1 {
		t.Fatalf("list service commissions: want 1 (upsert, not duplicate), got %d", len(overrides))
	}
	first, _ := overrides[0].(map[string]any)
	if first["commission"] != float64(50) {
		t.Fatalf("list service commissions: expected commission 50 after upsert, got %v", first["commission"])
	}
}

func TestConsultantServiceCommission_RejectsUnknownServiceOrInvalidRange(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	userID := RegisterStaffUser(t, ts.URL, adminToken, "commissionconsultant2@example.com", "correcthorsebattery", "clinician")
	var consultant map[string]any
	PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": userID, "full_name": "Dr. Bad Input", "default_commission": 30,
	}, &consultant)
	consultantID, _ := consultant["id"].(string)

	resp := PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": "00000000-0000-0000-0000-000000000000", "commission": 20,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("set commission for unknown service: want 400, got %d", resp.StatusCode)
	}

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Out Of Range Service", "price": 100,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	resp = PostJSONAuth(t, ts.URL, "/consultants/"+consultantID+"/service-commissions", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "commission": 150,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("set commission out of range: want 400, got %d", resp.StatusCode)
	}
}
