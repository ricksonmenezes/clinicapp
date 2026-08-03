package sms

import (
	"context"
	"errors"
)

// Message is a single outbound SMS. To must already be in the recipient
// format the provider expects (see NormalizePHPhone) — Send does not
// reformat it.
type Message struct {
	To   string
	Body string
}

// Sender is the interface internal/booking depends on, so it can be swapped
// for a fake in tests the same way internal/mailer.Mailer is.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ErrInvalidPhone is returned by NormalizePHPhone when the input doesn't
// look like a Philippine mobile number in any of the common forms
// (+639171234567, 639171234567, 09171234567, 9171234567).
var ErrInvalidPhone = errors.New("not a valid Philippine mobile number")
