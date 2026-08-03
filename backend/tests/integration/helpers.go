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
