package integration

import (
	"context"
	"sync"

	"clinicapp/backend/internal/mailer"
)

// FakeMailer implements mailer.Mailer in-process, capturing every send so
// tests can extract tokens from email bodies without a real SMTP hop.
type FakeMailer struct {
	mu   sync.Mutex
	Sent []mailer.Message
}

func NewFakeMailer() *FakeMailer {
	return &FakeMailer{}
}

func (f *FakeMailer) Send(_ context.Context, msg mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent = append(f.Sent, msg)
	return nil
}

// LastTo returns the most recent message sent to the given address.
func (f *FakeMailer) LastTo(to string) (mailer.Message, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.Sent) - 1; i >= 0; i-- {
		if f.Sent[i].To == to {
			return f.Sent[i], true
		}
	}
	return mailer.Message{}, false
}

// CountTo returns how many messages were sent to the given address.
func (f *FakeMailer) CountTo(to string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.Sent {
		if m.To == to {
			n++
		}
	}
	return n
}
