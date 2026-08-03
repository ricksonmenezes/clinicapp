package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

const philSMSAPIURL = "https://app.philsms.com/api/v3/sms/send"

// PhilSMSSender sends SMS via the PhilSMS HTTP API. SenderID is the
// registered brand name PhilSMS sends under — "PhilSMS" is the platform's
// own shared default, usable without registering a dedicated one; override
// via SMS_SENDER_ID once the clinic registers its own. APIURL defaults to
// the real PhilSMS endpoint when empty; tests override it to point at a
// local httptest server.
type PhilSMSSender struct {
	APIKey   string
	SenderID string
	APIURL   string
	Client   *http.Client
}

func NewPhilSMSSender(apiKey, senderID string) *PhilSMSSender {
	return &PhilSMSSender{APIKey: apiKey, SenderID: senderID, Client: http.DefaultClient}
}

type philSMSRequest struct {
	Recipient string `json:"recipient"`
	SenderID  string `json:"sender_id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
}

type philSMSResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (s *PhilSMSSender) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(philSMSRequest{
		Recipient: msg.To,
		SenderID:  s.SenderID,
		Type:      "plain",
		Message:   msg.Body,
	})
	if err != nil {
		return err
	}

	url := s.APIURL
	if url == "" {
		url = philSMSAPIURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("philsms: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		var parsed philSMSResponse
		if json.Unmarshal(respBody, &parsed) == nil && parsed.Message != "" {
			return fmt.Errorf("philsms: %s", parsed.Message)
		}
		return fmt.Errorf("philsms: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	var parsed philSMSResponse
	if json.Unmarshal(respBody, &parsed) == nil && parsed.Status == "error" {
		return fmt.Errorf("philsms: %s", parsed.Message)
	}

	return nil
}

var (
	phDigitsOnly = regexp.MustCompile(`[^0-9]`)
)

// NormalizePHPhone converts a free-text Philippine mobile number (as stored
// on patients.phone, which has no format validation at write time — see
// internal/patient) into the "63XXXXXXXXXX" form PhilSMS expects, with no
// leading "+". Accepts +639171234567, 639171234567, 09171234567, and bare
// 9171234567; anything else is ErrInvalidPhone rather than sent as-is,
// since a malformed recipient would otherwise fail silently at PhilSMS
// instead of surfacing a clear local error.
func NormalizePHPhone(raw string) (string, error) {
	digits := phDigitsOnly.ReplaceAllString(raw, "")

	switch {
	case len(digits) == 12 && digits[:2] == "63":
		return digits, nil
	case len(digits) == 11 && digits[0] == '0':
		return "63" + digits[1:], nil
	case len(digits) == 10 && digits[0] == '9':
		return "63" + digits, nil
	default:
		return "", ErrInvalidPhone
	}
}
