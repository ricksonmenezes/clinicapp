package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

const resendAPIURL = "https://api.resend.com/emails"

// ResendMailer sends mail via the Resend HTTP API (api.resend.com/emails) —
// no SMTP layer, even in local dev. Local dev uses a Resend sandbox key and
// the sandbox From address (onboarding@resend.dev); prod uses a verified
// domain address.
type ResendMailer struct {
	APIKey string
	From   string
	Client *http.Client
}

func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{APIKey: apiKey, From: from, Client: http.DefaultClient}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (m *ResendMailer) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(resendRequest{
		From:    m.From,
		To:      []string{msg.To},
		Subject: msg.Subject,
		Text:    msg.Body,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := m.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: unexpected status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
