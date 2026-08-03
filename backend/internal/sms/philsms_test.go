package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePHPhone(t *testing.T) {
	cases := map[string]string{
		"+639171234567":  "639171234567",
		"639171234567":   "639171234567",
		"09171234567":    "639171234567",
		"9171234567":     "639171234567",
		"0917-123-4567":  "639171234567",
		"(0917) 1234567": "639171234567",
	}
	for in, want := range cases {
		got, err := NormalizePHPhone(in)
		if err != nil {
			t.Errorf("NormalizePHPhone(%q): unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizePHPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePHPhone_Invalid(t *testing.T) {
	for _, in := range []string{"", "12345", "not a phone", "6591234567"} {
		if _, err := NormalizePHPhone(in); err != ErrInvalidPhone {
			t.Errorf("NormalizePHPhone(%q): got err %v, want ErrInvalidPhone", in, err)
		}
	}
}

func TestPhilSMSSender_Send(t *testing.T) {
	var gotReq philSMSRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing/wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(philSMSResponse{Status: "success", Message: "sent"})
	}))
	defer srv.Close()

	sender := &PhilSMSSender{APIKey: "test-key", SenderID: "PhilSMS", APIURL: srv.URL, Client: srv.Client()}

	if err := sender.Send(context.Background(), Message{To: "639171234567", Body: "hello"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotReq.Recipient != "639171234567" || gotReq.Message != "hello" || gotReq.SenderID != "PhilSMS" || gotReq.Type != "plain" {
		t.Errorf("unexpected request body: %+v", gotReq)
	}
}

func TestPhilSMSSender_Send_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(philSMSResponse{Status: "error", Message: "invalid recipient"})
	}))
	defer srv.Close()

	sender := &PhilSMSSender{APIKey: "test-key", SenderID: "PhilSMS", APIURL: srv.URL, Client: srv.Client()}

	err := sender.Send(context.Background(), Message{To: "639171234567", Body: "hello"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
