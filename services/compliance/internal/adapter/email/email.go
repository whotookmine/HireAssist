// Package email is the Email Adapter: it sends notices over SMTP, or logs them when no server
// is configured.
package email

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// SMTP sends through one SMTP server. Locally that is Mailpit, which accepts anything and shows
// it in a web page.
type SMTP struct {
	Addr     string // host:port
	From     string
	Username string // optional; with a password, PLAIN authentication
	Password string
}

// Notify sends one plain-text message.
func (s SMTP) Notify(_ context.Context, to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("refusing a header with a line break")
	}
	from, err := mail.ParseAddress(s.From) // the envelope takes the bare address, the header the display name too
	if err != nil {
		return fmt.Errorf("SMTP_FROM %q is not an address: %w", s.From, err)
	}
	msg := strings.Join([]string{
		"From: " + s.From,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, strings.Split(s.Addr, ":")[0])
	}
	if err := smtp.SendMail(s.Addr, auth, from.Address, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("sending to %s: %w", to, err)
	}
	slog.Info("notice sent", "subject", subject)
	return nil
}

// Log writes notices to the log instead of sending them.
type Log struct{}

// Notify logs the notice. The recipient is logged, the body is not.
func (Log) Notify(_ context.Context, to, subject, _ string) error {
	slog.Info("notice not sent: no SMTP server configured", "to", to, "subject", subject)
	return nil
}
