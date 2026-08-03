package mailer

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

type SMTPMailer struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func NewSMTPMailer(host, port, user, pass, from string) *SMTPMailer {
	return &SMTPMailer{Host: host, Port: port, User: user, Pass: pass, From: from}
}

func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	addr := net.JoinHostPort(m.Host, m.Port)

	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Pass, m.Host)
	}

	return smtp.SendMail(addr, auth, m.From, []string{msg.To}, buildMessage(m.From, msg))
}

func buildMessage(from string, msg Message) []byte {
	header := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n",
		from, msg.To, msg.Subject,
	)
	return []byte(header + msg.Body)
}
