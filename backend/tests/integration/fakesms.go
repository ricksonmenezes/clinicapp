package integration

import (
	"context"
	"sync"

	"clinicapp/backend/internal/sms"
)

// FakeSMSSender implements sms.Sender in-process, capturing every send so
// tests can assert on outbound SMS without a real PhilSMS API call — same
// role as FakeMailer for mailer.Mailer.
type FakeSMSSender struct {
	mu   sync.Mutex
	Sent []sms.Message
}

func NewFakeSMSSender() *FakeSMSSender {
	return &FakeSMSSender{}
}

func (f *FakeSMSSender) Send(_ context.Context, msg sms.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sent = append(f.Sent, msg)
	return nil
}

// LastTo returns the most recent message sent to the given (already
// PhilSMS-normalized) number.
func (f *FakeSMSSender) LastTo(to string) (sms.Message, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.Sent) - 1; i >= 0; i-- {
		if f.Sent[i].To == to {
			return f.Sent[i], true
		}
	}
	return sms.Message{}, false
}
