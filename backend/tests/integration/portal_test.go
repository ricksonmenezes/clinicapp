package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// webLoginClient registers+verifies a patient, logs in with X-Client-Type:
// web (so the response sets access_token/refresh_token cookies in the
// client's jar), and returns the cookie-jar client for use against portal
// page routes. Redirects are not auto-followed, so callers can assert on
// Location headers directly.
func webLoginClient(t *testing.T, baseURL string, fakeMailer *FakeMailer, email, password string) *http.Client {
	t.Helper()
	RegisterVerifiedPatient(t, baseURL, fakeMailer, email, password)

	client := NoRedirectClient(t)
	resp := PostJSONClient(t, client, baseURL, "/auth/login", "web", map[string]string{
		"email": email, "password": password,
	}, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("web login: want 303, got %d", resp.StatusCode)
	}
	if len(resp.Cookies()) == 0 {
		t.Fatalf("web login: expected auth cookies to be set")
	}
	return client
}

func getBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestPortalHome_UnauthenticatedShowsMarketingPage(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get /: want 200, got %d", resp.StatusCode)
	}
	body := getBody(t, resp)
	if !strings.Contains(body, "Get started") || !strings.Contains(body, "clinicapp") {
		t.Fatalf("expected marketing content on /, got %q", body)
	}
}

func TestPortalHome_AuthenticatedRedirectsToDashboard(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	client := webLoginClient(t, ts.URL, fakeMailer, "portalhome@example.com", "correcthorsebattery")

	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("get / authenticated: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %q", loc)
	}
}

func TestPortalDashboard_UnauthenticatedRedirectsToLogin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	client := NoRedirectClient(t)

	resp, err := client.Get(ts.URL + "/dashboard")
	if err != nil {
		t.Fatalf("get /dashboard: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("get /dashboard unauthenticated: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestPortalBook_UnauthenticatedRedirectsToLogin(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	client := NoRedirectClient(t)

	resp, err := client.Get(ts.URL + "/book")
	if err != nil {
		t.Fatalf("get /book: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("get /book unauthenticated: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestPortalDashboardAndBook_AuthenticatedRenders200(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	client := webLoginClient(t, ts.URL, fakeMailer, "portaldash@example.com", "correcthorsebattery")

	resp, err := client.Get(ts.URL + "/dashboard")
	if err != nil {
		t.Fatalf("get /dashboard: %v", err)
	}
	body := getBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get /dashboard authenticated: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Your dashboard") {
		t.Fatalf("expected dashboard content, got %q", body)
	}

	resp, err = client.Get(ts.URL + "/book")
	if err != nil {
		t.Fatalf("get /book: %v", err)
	}
	body = getBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get /book authenticated: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Book a service") {
		t.Fatalf("expected booking page content, got %q", body)
	}
}

func TestPortalPublicPages_Render200(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	for path, want := range map[string]string{
		"/register":        "Create your account",
		"/login":           "Log in",
		"/check-email":     "Check your email",
		"/forgot-password": "Forgot your password",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body := getBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get %s: want 200, got %d", path, resp.StatusCode)
		}
		if !strings.Contains(body, want) {
			t.Fatalf("get %s: expected content %q, got %q", path, want, body)
		}
	}
}

func TestPortalResetPassword_EchoesTokenIntoHiddenField(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	resp, err := http.Get(ts.URL + "/reset-password?token=abc-123")
	if err != nil {
		t.Fatalf("get /reset-password: %v", err)
	}
	body := getBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get /reset-password: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, `value="abc-123"`) {
		t.Fatalf("expected token echoed into hidden field, got %q", body)
	}
}

func TestPortalStatic_ServesCSS(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	resp, err := http.Get(ts.URL + "/static/css/app.css")
	if err != nil {
		t.Fatalf("get /static/css/app.css: %v", err)
	}
	body := getBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get /static/css/app.css: want 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "font-family") {
		t.Fatalf("expected CSS content, got %q", body[:min(80, len(body))])
	}
}

func TestPortalUnmatchedPath_Returns404NotHomePage(t *testing.T) {
	ts, _, _ := NewTestServer(t)

	resp, err := http.Get(ts.URL + "/this-path-does-not-exist")
	if err != nil {
		t.Fatalf("get unmatched path: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unmatched path (not the home page catch-all), got %d", resp.StatusCode)
	}
}
