package integration

import (
	"net/http"
	"testing"
)

func TestPackageFlow_CreateListGetUpdate(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Laser Hair Removal", "price": 800,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id":    serviceID,
		"name":          "6-Session Bundle",
		"session_count": 6,
		"price":         4000,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create package: want 201, got %d (%v)", resp.StatusCode, created)
	}
	packageID, _ := created["id"].(string)
	if packageID == "" {
		t.Fatalf("create package: expected id in response, got %v", created)
	}
	if created["session_count"] != float64(6) {
		t.Fatalf("create package: expected session_count 6, got %v", created["session_count"])
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/packages", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list packages: want 200, got %d", resp.StatusCode)
	}
	packages, _ := listResp["packages"].([]any)
	if len(packages) != 1 {
		t.Fatalf("list packages: want 1, got %d", len(packages))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/packages/"+packageID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get package: want 200, got %d", resp.StatusCode)
	}
	if getResp["name"] != "6-Session Bundle" {
		t.Fatalf("get package: expected name 6-Session Bundle, got %v", getResp["name"])
	}

	var updateResp map[string]any
	resp = PatchJSON(t, ts.URL, "/packages/"+packageID, "mobile", adminToken, map[string]any{
		"name":          "6-Session Bundle (Promo)",
		"session_count": 6,
		"price":         3600,
		"active":        false,
	}, &updateResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update package: want 200, got %d (%v)", resp.StatusCode, updateResp)
	}
	if updateResp["active"] != false {
		t.Fatalf("update package: expected active false, got %v", updateResp["active"])
	}
}

func TestPackageCreate_RejectsUnknownServiceOrInvalidInput(t *testing.T) {
	ts, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id":    "00000000-0000-0000-0000-000000000000",
		"name":          "Ghost Package",
		"session_count": 5,
		"price":         100,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create package for unknown service: want 400, got %d", resp.StatusCode)
	}

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Some Service", "price": 100,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	resp = PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id":    serviceID,
		"name":          "Zero Sessions",
		"session_count": 0,
		"price":         100,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create package with session_count 0: want 400, got %d", resp.StatusCode)
	}
}

func TestPatientPackageFlow_SubscribeSeedsSessionsRemaining(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	patientUserID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "packagepatient@example.com", "correcthorsebattery")
	var patientProfile map[string]any
	PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": patientUserID, "full_name": "Package Patient",
	}, &patientProfile)
	patientID, _ := patientProfile["id"].(string)

	consultantUserID := RegisterStaffUser(t, ts.URL, adminToken, "packageconsultant@example.com", "correcthorsebattery", "clinician")
	var consultantProfile map[string]any
	PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": consultantUserID, "full_name": "Dr. Principal", "default_commission": 25,
	}, &consultantProfile)
	consultantID, _ := consultantProfile["id"].(string)

	var svc map[string]any
	PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name": "Physio", "price": 500,
	}, &svc)
	serviceID, _ := svc["id"].(string)

	var pkg map[string]any
	PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "name": "Physio 10-pack", "session_count": 10, "price": 4500,
	}, &pkg)
	packageID, _ := pkg["id"].(string)

	var subscribed map[string]any
	resp := PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id":           patientID,
		"package_id":           packageID,
		"principal_consultant": consultantID,
	}, &subscribed)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("subscribe patient to package: want 201, got %d (%v)", resp.StatusCode, subscribed)
	}
	if subscribed["sessions_remaining"] != float64(10) {
		t.Fatalf("subscribe: expected sessions_remaining seeded to 10, got %v", subscribed["sessions_remaining"])
	}
	patientPackageID, _ := subscribed["id"].(string)
	if patientPackageID == "" {
		t.Fatalf("subscribe: expected id in response, got %v", subscribed)
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/patient-packages", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list patient-packages: want 200, got %d", resp.StatusCode)
	}
	items, _ := listResp["patient_packages"].([]any)
	if len(items) != 1 {
		t.Fatalf("list patient-packages: want 1, got %d", len(items))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/patient-packages/"+patientPackageID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get patient-package: want 200, got %d", resp.StatusCode)
	}
	if getResp["principal_consultant"] != consultantID {
		t.Fatalf("get patient-package: expected principal_consultant %v, got %v", consultantID, getResp["principal_consultant"])
	}
}

func TestPatientPackageSubscribe_RejectsUnknownPatientOrPackage(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	patientUserID := RegisterVerifiedPatient(t, ts.URL, fakeMailer, "badsubscribe@example.com", "correcthorsebattery")
	var patientProfile map[string]any
	PostJSONAuth(t, ts.URL, "/patients", "mobile", adminToken, map[string]any{
		"user_id": patientUserID, "full_name": "Bad Subscribe",
	}, &patientProfile)
	patientID, _ := patientProfile["id"].(string)

	resp := PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": patientID,
		"package_id": "00000000-0000-0000-0000-000000000000",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("subscribe with unknown package: want 400, got %d", resp.StatusCode)
	}

	resp = PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": "00000000-0000-0000-0000-000000000000",
		"package_id": "00000000-0000-0000-0000-000000000000",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("subscribe with unknown patient: want 400, got %d", resp.StatusCode)
	}
}
