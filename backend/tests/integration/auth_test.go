package integration

import (
	"net/http"
	"testing"
)

// TestAuthFlow_RegisterVerifyLoginProtected is the canonical M1 flow from
// PLAN.md's Testing strategy section: register -> FakeMailer captures the
// email -> extract token -> verify -> login -> hit a protected endpoint.
func TestAuthFlow_RegisterVerifyLoginProtected(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)

	email := "e2e@example.com"
	password := "correcthorsebattery"

	var registerResp map[string]any
	resp := PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": password,
	}, &registerResp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: want 201, got %d (%v)", resp.StatusCode, registerResp)
	}

	if got := fakeMailer.CountTo(email); got != 1 {
		t.Fatalf("want 1 verification email sent, got %d", got)
	}
	msg, _ := fakeMailer.LastTo(email)
	token := ExtractToken(t, msg.Body)

	var verifyResp map[string]any
	resp = Get(t, ts.URL, "/auth/verify-email?token="+token, "mobile", "", &verifyResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify-email: want 200, got %d (%v)", resp.StatusCode, verifyResp)
	}
	accessToken, _ := verifyResp["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("verify-email: expected access_token in response, got %v", verifyResp)
	}

	var loginResp map[string]any
	resp = PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": password,
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%v)", resp.StatusCode, loginResp)
	}
	loginAccessToken, _ := loginResp["access_token"].(string)
	if loginAccessToken == "" {
		t.Fatalf("login: expected access_token in response, got %v", loginResp)
	}

	if resp := Get(t, ts.URL, "/patients", "mobile", loginAccessToken, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("protected /patients with token: want 200, got %d", resp.StatusCode)
	}

	if resp := Get(t, ts.URL, "/patients", "mobile", "", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("protected /patients without token: want 401, got %d", resp.StatusCode)
	}
}

// TestAuthFlow_WebClientUsesCookiesAndRedirects exercises the same flow via
// X-Client-Type: web, asserting on redirects and httpOnly cookies instead
// of a JSON body, per the renderer's documented web/mobile contract.
func TestAuthFlow_WebClientUsesCookiesAndRedirects(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	client := NoRedirectClient(t)

	email := "webflow@example.com"
	password := "correcthorsebattery"

	resp := PostJSONClient(t, client, ts.URL, "/auth/register", "web", map[string]string{
		"email": email, "password": password,
	}, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("register: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/check-email" {
		t.Fatalf("register: want redirect to /check-email, got %q", loc)
	}

	msg, ok := fakeMailer.LastTo(email)
	if !ok {
		t.Fatalf("no verification email sent to %s", email)
	}
	token := ExtractToken(t, msg.Body)

	resp = GetClient(t, client, ts.URL, "/auth/verify-email?token="+token, "web", "", nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("verify-email: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/dashboard" {
		t.Fatalf("verify-email: want redirect to /dashboard, got %q", loc)
	}

	var gotAccess, gotRefresh bool
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "access_token":
			gotAccess = c.Value != "" && c.HttpOnly
		case "refresh_token":
			gotRefresh = c.Value != "" && c.HttpOnly
		}
	}
	if !gotAccess || !gotRefresh {
		t.Fatalf("verify-email: expected httpOnly access_token and refresh_token cookies, got %v", resp.Cookies())
	}

	if resp := GetClient(t, client, ts.URL, "/patients", "web", "", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("protected /patients via cookie: want 200, got %d", resp.StatusCode)
	}
}

func TestRegister_DuplicateEmailRejected(t *testing.T) {
	ts, _ := NewTestServer(t)
	email := "dup@example.com"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)

	resp := PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "anotherpassword",
	}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate register: want 409, got %d", resp.StatusCode)
	}
}

func TestLogin_WrongPasswordRejected(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	email := "wrongpass@example.com"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)
	msg, _ := fakeMailer.LastTo(email)
	Get(t, ts.URL, "/auth/verify-email?token="+ExtractToken(t, msg.Body), "mobile", "", nil)

	resp := PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": "totallywrongpassword",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password login: want 401, got %d", resp.StatusCode)
	}
}

func TestLogin_UnverifiedEmailRejected(t *testing.T) {
	ts, _ := NewTestServer(t)
	email := "unverified@example.com"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)

	resp := PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unverified login: want 403, got %d", resp.StatusCode)
	}
}

func TestRefreshToken_RotatesAndRevokesOldToken(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	email := "refresh@example.com"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)
	msg, _ := fakeMailer.LastTo(email)
	var verifyResp map[string]any
	Get(t, ts.URL, "/auth/verify-email?token="+ExtractToken(t, msg.Body), "mobile", "", &verifyResp)
	oldRefresh := verifyResp["refresh_token"].(string)

	var refreshResp map[string]any
	resp := PostJSON(t, ts.URL, "/auth/refresh", "mobile", map[string]string{
		"refresh_token": oldRefresh,
	}, &refreshResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d (%v)", resp.StatusCode, refreshResp)
	}
	newRefresh, _ := refreshResp["refresh_token"].(string)
	if newRefresh == "" || newRefresh == oldRefresh {
		t.Fatalf("refresh: expected a new, different refresh_token, got %q", newRefresh)
	}

	resp = PostJSON(t, ts.URL, "/auth/refresh", "mobile", map[string]string{
		"refresh_token": oldRefresh,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reusing rotated refresh token: want 400, got %d", resp.StatusCode)
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	email := "logout@example.com"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": "correcthorsebattery",
	}, nil)
	msg, _ := fakeMailer.LastTo(email)
	var verifyResp map[string]any
	Get(t, ts.URL, "/auth/verify-email?token="+ExtractToken(t, msg.Body), "mobile", "", &verifyResp)
	refreshToken := verifyResp["refresh_token"].(string)

	resp := PostJSON(t, ts.URL, "/auth/logout", "mobile", map[string]string{
		"refresh_token": refreshToken,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: want 200, got %d", resp.StatusCode)
	}

	resp = PostJSON(t, ts.URL, "/auth/refresh", "mobile", map[string]string{
		"refresh_token": refreshToken,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("refresh after logout: want 400, got %d", resp.StatusCode)
	}
}

func TestForgotPassword_ResetFlow(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)
	email := "forgot@example.com"
	oldPassword := "correcthorsebattery"
	newPassword := "newcorrecthorsebattery"

	PostJSON(t, ts.URL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": oldPassword,
	}, nil)
	regMsg, _ := fakeMailer.LastTo(email)
	Get(t, ts.URL, "/auth/verify-email?token="+ExtractToken(t, regMsg.Body), "mobile", "", nil)

	resp := PostJSON(t, ts.URL, "/auth/forgot-password", "mobile", map[string]string{
		"email": email,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("forgot-password: want 200, got %d", resp.StatusCode)
	}

	resetMsg, ok := fakeMailer.LastTo(email)
	if !ok || resetMsg.Subject != "Reset your clinicapp password" {
		t.Fatalf("expected a password reset email, got %+v", resetMsg)
	}
	resetToken := ExtractToken(t, resetMsg.Body)

	resp = PostJSON(t, ts.URL, "/auth/reset-password", "mobile", map[string]string{
		"token": resetToken, "password": newPassword,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset-password: want 200, got %d", resp.StatusCode)
	}

	if resp := PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": oldPassword,
	}, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login with old password after reset: want 401, got %d", resp.StatusCode)
	}

	if resp := PostJSON(t, ts.URL, "/auth/login", "mobile", map[string]string{
		"email": email, "password": newPassword,
	}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("login with new password after reset: want 200, got %d", resp.StatusCode)
	}
}

func TestForgotPassword_UnknownEmailDoesNotLeak(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)

	resp := PostJSON(t, ts.URL, "/auth/forgot-password", "mobile", map[string]string{
		"email": "nobody-registered@example.com",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("forgot-password for unknown email: want 200, got %d", resp.StatusCode)
	}
	if got := fakeMailer.CountTo("nobody-registered@example.com"); got != 0 {
		t.Fatalf("want no email sent for unknown address, got %d", got)
	}
}

func TestResendVerification_NoEnumeration(t *testing.T) {
	ts, fakeMailer := NewTestServer(t)

	resp := PostJSON(t, ts.URL, "/auth/resend-verification", "mobile", map[string]string{
		"email": "not-a-real-user@example.com",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resend-verification for unknown email: want 200, got %d", resp.StatusCode)
	}
	if got := fakeMailer.CountTo("not-a-real-user@example.com"); got != 0 {
		t.Fatalf("want no email sent for unknown address, got %d", got)
	}
}
