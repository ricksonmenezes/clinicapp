package integration

import (
	"net/http"
	"testing"
)

func TestServiceFlow_CreateListGetUpdate(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name":                "Deep Tissue Massage",
		"description":         "60 minute session",
		"price":               1200.50,
		"requires_consultant": true,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create service: want 201, got %d (%v)", resp.StatusCode, created)
	}
	serviceID, _ := created["id"].(string)
	if serviceID == "" {
		t.Fatalf("create service: expected id in response, got %v", created)
	}
	if created["price"] != 1200.5 {
		t.Fatalf("create service: expected price 1200.5, got %v", created["price"])
	}
	if created["active"] != true {
		t.Fatalf("create service: expected active to default true, got %v", created["active"])
	}

	var listResp map[string]any
	resp = Get(t, ts.URL, "/services", "mobile", adminToken, &listResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list services: want 200, got %d", resp.StatusCode)
	}
	services, _ := listResp["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("list services: want 1, got %d", len(services))
	}

	var getResp map[string]any
	resp = Get(t, ts.URL, "/services/"+serviceID, "mobile", adminToken, &getResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get service: want 200, got %d", resp.StatusCode)
	}
	if getResp["name"] != "Deep Tissue Massage" {
		t.Fatalf("get service: expected name Deep Tissue Massage, got %v", getResp["name"])
	}

	var updateResp map[string]any
	resp = PatchJSON(t, ts.URL, "/services/"+serviceID, "mobile", adminToken, map[string]any{
		"name":                "Deep Tissue Massage (90 min)",
		"price":               1600,
		"requires_consultant": true,
		"active":              false,
	}, &updateResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update service: want 200, got %d (%v)", resp.StatusCode, updateResp)
	}
	if updateResp["active"] != false {
		t.Fatalf("update service: expected active false, got %v", updateResp["active"])
	}
}

func TestServiceCreate_RejectsInvalidInput(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name":  "",
		"price": 10,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create service with empty name: want 400, got %d", resp.StatusCode)
	}

	resp = PostJSONAuth(t, ts.URL, "/services", "mobile", adminToken, map[string]any{
		"name":  "Negative Price Service",
		"price": -5,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create service with negative price: want 400, got %d", resp.StatusCode)
	}
}

func TestServiceCreate_ForbiddenForNonAdmin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	RegisterStaffUser(t, ts.URL, adminToken, "clinicianservices@example.com", "correcthorsebattery", "clinician")

	var loginResp map[string]any
	PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": "clinicianservices@example.com", "password": "correcthorsebattery",
	}, &loginResp)
	clinicianToken, _ := loginResp["access_token"].(string)

	if resp := PostJSONAuth(t, ts.URL, "/services", "mobile", clinicianToken, map[string]any{
		"name": "Should Fail", "price": 10,
	}, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("create service as clinician: want 403, got %d", resp.StatusCode)
	}

	// Reads are open to any authenticated role, unlike consultants/attendants.
	if resp := Get(t, ts.URL, "/services", "mobile", clinicianToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("list services as clinician: want 200, got %d", resp.StatusCode)
	}
}
