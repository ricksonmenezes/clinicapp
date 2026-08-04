package integration

import (
	"net/http"
	"strings"
	"testing"
)

// webLoginAsAdmin logs in as the bootstrap admin (seeded by NewTestServer)
// with X-Client-Type: web, returning a cookie-jar client for /admin/* page
// requests.
func webLoginAsAdmin(t *testing.T, baseURL string) *http.Client {
	t.Helper()
	client := NoRedirectClient(t)
	resp := PostJSONClient(t, client, baseURL, "/auth/login", "web", map[string]string{
		"email": TestAdminEmail, "password": TestAdminPassword,
	}, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin web login: want 303, got %d", resp.StatusCode)
	}
	return client
}

// webLoginAsStaff creates a clinician/attendant/admin account via the
// admin-only /auth/register-staff endpoint (active immediately, no
// verification round-trip) and logs it in with X-Client-Type: web.
func webLoginAsStaff(t *testing.T, baseURL, adminToken, email, password, role string) *http.Client {
	t.Helper()
	RegisterStaffUser(t, baseURL, adminToken, email, password, role)

	client := NoRedirectClient(t)
	resp := PostJSONClient(t, client, baseURL, "/auth/login", "web", map[string]string{
		"email": email, "password": password,
	}, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("staff web login (%s): want 303, got %d", role, resp.StatusCode)
	}
	return client
}

func TestBackofficePages_UnauthenticatedRedirectsToLogin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	client := NoRedirectClient(t)

	for _, path := range []string{
		"/admin", "/admin/patients", "/admin/consultants", "/admin/attendants",
		"/admin/services", "/admin/packages", "/admin/sessions", "/admin/invoices",
		"/admin/prescriptions", "/admin/reports", "/admin/staff/new",
	} {
		resp, err := client.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("get %s unauthenticated: want 303, got %d", path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Fatalf("get %s unauthenticated: expected redirect to /login, got %q", path, loc)
		}
	}
}

func TestBackofficePages_WrongRoleGetsForbidden(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	clinician := webLoginAsStaff(t, ts.URL, adminToken, "boclinician@example.com", "correcthorsebattery", "clinician")
	attendant := webLoginAsStaff(t, ts.URL, adminToken, "boattendant@example.com", "correcthorsebattery", "attendant")
	patientClient := webLoginClient(t, ts.URL, fakeMailer, "bopatient@example.com", "correcthorsebattery")

	cases := []struct {
		name   string
		client *http.Client
		path   string
	}{
		{"clinician hits admin-only consultants", clinician, "/admin/consultants"},
		{"clinician hits admin-only attendants", clinician, "/admin/attendants"},
		{"clinician hits admin-only reports", clinician, "/admin/reports"},
		{"clinician hits admin-only staff/new", clinician, "/admin/staff/new"},
		{"attendant hits admin-only consultants", attendant, "/admin/consultants"},
		{"attendant hits admin+clinician patients", attendant, "/admin/patients"},
		{"attendant hits clinician-only prescriptions", attendant, "/admin/prescriptions"},
		{"patient hits backoffice dashboard", patientClient, "/admin"},
		{"patient hits patients page", patientClient, "/admin/patients"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.client.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("get %s: %v", tc.path, err)
			}
			body := getBody(t, resp)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("get %s: want 403, got %d (%s)", tc.path, resp.StatusCode, body)
			}
			if !strings.Contains(body, "Forbidden") {
				t.Fatalf("get %s: expected forbidden page content, got %q", tc.path, body)
			}
		})
	}

	// Admin itself is also gated out of the clinician-only prescriptions page.
	t.Run("admin hits clinician-only prescriptions", func(t *testing.T) {
		admin := webLoginAsAdmin(t, ts.URL)
		resp, err := admin.Get(ts.URL + "/admin/prescriptions")
		if err != nil {
			t.Fatalf("get /admin/prescriptions: %v", err)
		}
		body := getBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("admin get /admin/prescriptions: want 403, got %d (%s)", resp.StatusCode, body)
		}
	})
}

func TestBackofficePages_CorrectRoleRenders200(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	admin := webLoginAsAdmin(t, ts.URL)
	clinician := webLoginAsStaff(t, ts.URL, adminToken, "bookclinician@example.com", "correcthorsebattery", "clinician")
	attendant := webLoginAsStaff(t, ts.URL, adminToken, "bookattendant@example.com", "correcthorsebattery", "attendant")

	adminPages := map[string]string{
		"/admin":             "Backoffice",
		"/admin/patients":    "Patients",
		"/admin/consultants": "Consultants",
		"/admin/attendants":  "Attendants",
		"/admin/services":    "Services",
		"/admin/packages":    "Packages",
		"/admin/sessions":    "Sessions",
		"/admin/invoices":    "Invoices",
		"/admin/reports":     "Reports",
		"/admin/staff/new":   "New staff account",
	}
	for path, want := range adminPages {
		resp, err := admin.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("admin get %s: %v", path, err)
		}
		body := getBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("admin get %s: want 200, got %d (%s)", path, resp.StatusCode, body)
		}
		if !strings.Contains(body, want) {
			t.Fatalf("admin get %s: expected content %q, got %q", path, want, body)
		}
	}

	clinicianPages := map[string]string{
		"/admin":               "Backoffice",
		"/admin/patients":      "Patients",
		"/admin/services":      "Services",
		"/admin/packages":      "Packages",
		"/admin/sessions":      "Sessions",
		"/admin/invoices":      "Invoices",
		"/admin/prescriptions": "Prescriptions",
	}
	for path, want := range clinicianPages {
		resp, err := clinician.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("clinician get %s: %v", path, err)
		}
		body := getBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("clinician get %s: want 200, got %d (%s)", path, resp.StatusCode, body)
		}
		if !strings.Contains(body, want) {
			t.Fatalf("clinician get %s: expected content %q, got %q", path, want, body)
		}
	}

	attendantPages := map[string]string{
		"/admin":          "Backoffice",
		"/admin/services": "Services",
	}
	for path, want := range attendantPages {
		resp, err := attendant.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("attendant get %s: %v", path, err)
		}
		body := getBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("attendant get %s: want 200, got %d (%s)", path, resp.StatusCode, body)
		}
		if !strings.Contains(body, want) {
			t.Fatalf("attendant get %s: expected content %q, got %q", path, want, body)
		}
	}
}

func TestBackofficeStaffCreation_SurfacesUserID(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	admin := webLoginAsAdmin(t, ts.URL)

	// The web-mode fragment (what the /admin/staff/new form's fetch() call
	// receives) must surface the created user's id in a data-user-id
	// attribute, not just prose, so admin.js/the admin can chain it into
	// creating a consultant/attendant/patient profile.
	req, reqErr := http.NewRequest(http.MethodPost, ts.URL+"/auth/register-staff", strings.NewReader(`{"email":"newstaffweb@example.com","password":"correcthorsebattery","role":"clinician"}`))
	if reqErr != nil {
		t.Fatalf("build request: %v", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")
	postResp, err := admin.Do(req)
	if err != nil {
		t.Fatalf("post register-staff: %v", err)
	}
	body := getBody(t, postResp)
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("register-staff (web): want 201, got %d (%s)", postResp.StatusCode, body)
	}
	if !strings.Contains(body, "data-user-id=") {
		t.Fatalf("expected data-user-id in web fragment, got %q", body)
	}
}

func TestPortalHome_StaffRedirectsToAdmin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	admin := webLoginAsAdmin(t, ts.URL)

	resp, err := admin.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("staff get /: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin" {
		t.Fatalf("staff get /: expected redirect to /admin, got %q", loc)
	}
}

// TestLogin_RedirectsStaffToAdmin locks in the fix alongside portal.Home's
// redirect: POST /auth/login's own web-mode redirect was previously
// hardcoded to /dashboard for every role, which sent a freshly-logged-in
// admin/clinician/attendant to the patient dashboard (which then 403s on
// its patient-only GET /patients/me fragment) instead of the backoffice.
func TestLogin_RedirectsStaffToAdmin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	client := NoRedirectClient(t)

	resp := PostJSONClient(t, client, ts.URL, "/auth/login", "web", map[string]string{
		"email": TestAdminEmail, "password": TestAdminPassword,
	}, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin login: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin" {
		t.Fatalf("admin login: expected redirect to /admin, got %q", loc)
	}
}
