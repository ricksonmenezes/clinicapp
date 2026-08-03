package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"testing"
)

var verifyTokenRe = regexp.MustCompile(`token=([a-f0-9-]+)`)

// ExtractToken pulls the token query param out of an email body's link.
func ExtractToken(t *testing.T, body string) string {
	t.Helper()
	match := verifyTokenRe.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("no token found in email body: %q", body)
	}
	return match[1]
}

// PostJSON sends a JSON-encoded POST via http.DefaultClient and decodes the
// JSON response into out (if out is non-nil). clientType is the
// X-Client-Type header value.
func PostJSON(t *testing.T, baseURL, path, clientType string, payload any, out any) *http.Response {
	t.Helper()
	return PostJSONClient(t, http.DefaultClient, baseURL, path, clientType, payload, out)
}

func PostJSONClient(t *testing.T, client *http.Client, baseURL, path, clientType string, payload any, out any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if clientType != "" {
		req.Header.Set("X-Client-Type", clientType)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	decodeIfPresent(t, resp, out)
	return resp
}

// PostJSONAuth sends a JSON-encoded POST with a bearer token via
// http.DefaultClient and decodes the JSON response into out (if non-nil).
func PostJSONAuth(t *testing.T, baseURL, path, clientType, bearerToken string, payload any, out any) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(mustJSON(t, payload)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if clientType != "" {
		req.Header.Set("X-Client-Type", clientType)
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	decodeIfPresent(t, resp, out)
	return resp
}

// PatchJSON sends a JSON-encoded PATCH via http.DefaultClient and decodes
// the JSON response into out (if out is non-nil).
func PatchJSON(t *testing.T, baseURL, path, clientType, bearerToken string, payload any, out any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, baseURL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if clientType != "" {
		req.Header.Set("X-Client-Type", clientType)
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	decodeIfPresent(t, resp, out)
	return resp
}

// LoginAdmin logs in as the bootstrap admin seeded by NewTestServer and
// returns its access token.
func LoginAdmin(t *testing.T, baseURL string) string {
	t.Helper()
	var loginResp map[string]any
	resp := PostJSON(t, baseURL, "/auth/login", "mobile", map[string]string{
		"email": TestAdminEmail, "password": TestAdminPassword,
	}, &loginResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login: want 200, got %d (%v)", resp.StatusCode, loginResp)
	}
	token, _ := loginResp["access_token"].(string)
	if token == "" {
		t.Fatalf("admin login: expected access_token, got %v", loginResp)
	}
	return token
}

// RegisterVerifiedPatient self-registers a patient account, extracts the
// verification token from fakeMailer, verifies it, and returns the new
// user's ID.
func RegisterVerifiedPatient(t *testing.T, baseURL string, fakeMailer *FakeMailer, email, password string) string {
	t.Helper()

	var registerResp map[string]any
	resp := PostJSON(t, baseURL, "/auth/register", "mobile", map[string]string{
		"email": email, "password": password,
	}, &registerResp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: want 201, got %d (%v)", resp.StatusCode, registerResp)
	}
	userID, _ := registerResp["id"].(string)
	if userID == "" {
		t.Fatalf("register: expected id in response, got %v", registerResp)
	}

	msg, ok := fakeMailer.LastTo(email)
	if !ok {
		t.Fatalf("no verification email sent to %s", email)
	}
	resp = Get(t, baseURL, "/auth/verify-email?token="+ExtractToken(t, msg.Body), "mobile", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify-email: want 200, got %d", resp.StatusCode)
	}

	return userID
}

// RegisterStaffUser creates a clinician/attendant/admin account via the
// admin-only /auth/register-staff endpoint and returns the new user's ID.
func RegisterStaffUser(t *testing.T, baseURL, adminToken, email, password, role string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/auth/register-staff", bytes.NewReader(mustJSON(t, map[string]string{
		"email": email, "password": password, "role": role,
	})))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Type", "mobile")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	var out map[string]any
	decodeIfPresent(t, resp, &out)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register-staff: want 201, got %d (%v)", resp.StatusCode, out)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("register-staff: expected id in response, got %v", out)
	}
	return id
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// Get sends a GET request via http.DefaultClient with an optional bearer
// token and X-Client-Type.
func Get(t *testing.T, baseURL, path, clientType, bearerToken string, out any) *http.Response {
	t.Helper()
	return GetClient(t, http.DefaultClient, baseURL, path, clientType, bearerToken, out)
}

func GetClient(t *testing.T, client *http.Client, baseURL, path, clientType, bearerToken string, out any) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if clientType != "" {
		req.Header.Set("X-Client-Type", clientType)
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	decodeIfPresent(t, resp, out)
	return resp
}

// NoRedirectClient returns an http.Client with a cookie jar that does not
// follow redirects, so tests can assert on 303 + Location directly.
func NoRedirectClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func decodeIfPresent(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}
