package integration

import (
	"net/http"
	"strings"
	"testing"
)

func TestUserSearch_FindsMatchingClinicianByPartialName(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := PostJSONAuth(t, ts.URL, "/auth/register-staff", "mobile", adminToken, map[string]string{
		"email": "jane.diaz@example.com", "password": "correcthorsebattery", "role": "clinician",
		"full_name": "Jane Diaz", "date_of_birth": "1990-05-14",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register-staff: want 201, got %d", resp.StatusCode)
	}

	var out map[string]any
	resp = Get(t, ts.URL, "/users?role=clinician&q=jane", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: want 200, got %d (%v)", resp.StatusCode, out)
	}
	users, _ := out["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 match, got %v", out)
	}
	u := users[0].(map[string]any)
	if u["full_name"] != "Jane Diaz" || u["date_of_birth"] != "1990-05-14" {
		t.Fatalf("expected Jane Diaz (1990-05-14), got %v", u)
	}
}

func TestUserSearch_RespectsRoleFilter(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	PostJSONAuth(t, ts.URL, "/auth/register-staff", "mobile", adminToken, map[string]string{
		"email": "attendantsmith@example.com", "password": "correcthorsebattery", "role": "attendant",
		"full_name": "Smith Attendant",
	}, nil)

	var out map[string]any
	resp := Get(t, ts.URL, "/users?role=clinician&q=smith", "mobile", adminToken, &out)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: want 200, got %d", resp.StatusCode)
	}
	users, _ := out["users"].([]any)
	if len(users) != 0 {
		t.Fatalf("expected role filter to exclude the attendant, got %v", out)
	}
}

func TestUserSearch_UnlinkedExcludesUsersWithExistingProfile(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	userID := RegisterStaffUser(t, ts.URL, adminToken, "linkedclinician@example.com", "correcthorsebattery", "clinician")
	resp := PostJSONAuth(t, ts.URL, "/consultants", "mobile", adminToken, map[string]any{
		"user_id": userID, "full_name": "Linked Clinician", "default_commission": 30,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create consultant profile: want 201, got %d", resp.StatusCode)
	}

	var linked map[string]any
	resp = Get(t, ts.URL, "/users?role=clinician&q=linked&unlinked=true", "mobile", adminToken, &linked)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: want 200, got %d", resp.StatusCode)
	}
	if users, _ := linked["users"].([]any); len(users) != 0 {
		t.Fatalf("expected already-linked clinician excluded from unlinked search, got %v", linked)
	}

	var unfiltered map[string]any
	resp = Get(t, ts.URL, "/users?role=clinician&q=linked", "mobile", adminToken, &unfiltered)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: want 200, got %d", resp.StatusCode)
	}
	if users, _ := unfiltered["users"].([]any); len(users) != 1 {
		t.Fatalf("expected the linked clinician to still appear without unlinked filter, got %v", unfiltered)
	}
}

func TestUserSearch_NonAdminForbidden(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	RegisterStaffUser(t, ts.URL, adminToken, "searchclinician@example.com", "correcthorsebattery", "clinician")

	loginResp := map[string]any{}
	resp := PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": "searchclinician@example.com", "password": "correcthorsebattery",
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clinician login: want 200, got %d", resp.StatusCode)
	}
	clinicianToken, _ := loginResp["access_token"].(string)

	resp = Get(t, ts.URL, "/users?role=clinician&q=a", "mobile", clinicianToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("clinician search: want 403, got %d", resp.StatusCode)
	}
}

func TestUserSearch_WebFragmentShowsDOBAndOmitsWhenAbsent(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	PostJSONAuth(t, ts.URL, "/auth/register-staff", "mobile", adminToken, map[string]string{
		"email": "withdob@example.com", "password": "correcthorsebattery", "role": "clinician",
		"full_name": "With Dob", "date_of_birth": "1985-01-01",
	}, nil)
	PostJSONAuth(t, ts.URL, "/auth/register-staff", "mobile", adminToken, map[string]string{
		"email": "nodob@example.com", "password": "correcthorsebattery", "role": "clinician",
		"full_name": "No Dob",
	}, nil)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/users?role=clinician&q=Dob", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Client-Type", "web")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	body := getBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search (web): want 200, got %d (%s)", resp.StatusCode, body)
	}
	if !strings.Contains(body, "With Dob (1985-01-01)") {
		t.Fatalf("expected DOB shown in parens, got %q", body)
	}
	if !strings.Contains(body, ">No Dob<") {
		t.Fatalf("expected no DOB suffix for a user without one, got %q", body)
	}
}

func TestRegisterStaff_FragmentIncludesNameAndUserID(t *testing.T) {
	// Regression guard: the fragment must show both the full name (in
	// prose) and the id (as a data attribute) — the id no longer needs to
	// be copied anywhere by hand since GET /users replaced that, but it's
	// still exposed for scripting/debugging.
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/auth/register-staff", strings.NewReader(`{"email":"fragmentcheck@example.com","password":"correcthorsebattery","role":"clinician","full_name":"Fragment Check"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Type", "web")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	body := getBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register-staff (web): want 201, got %d (%s)", resp.StatusCode, body)
	}
	if !strings.Contains(body, "data-user-id=") || !strings.Contains(body, "Fragment Check") {
		t.Fatalf("expected data-user-id and full name in fragment, got %q", body)
	}
}
