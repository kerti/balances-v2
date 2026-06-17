package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
)

// SMTPConfig configures a plain-SMTP Mailer. In dev this points at Mailpit
// (localhost:1025, no auth, no TLS). In production the same shape can target
// any SMTP relay; if/when Resend becomes the primary provider we'll add a
// separate ResendMailer alongside this one rather than retrofitting SMTP.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string // empty for Mailpit
	Password string // empty for Mailpit
	From     string
}

type SMTPMailer struct {
	cfg SMTPConfig
	// envelope is the bare RFC5321 address used as the SMTP reverse-path
	// (MAIL FROM). It is derived from cfg.From: a display-name From like
	// "Balances <noreply@example.com>" keeps that form in the From: header but
	// must be reduced to the bare "noreply@example.com" here, or the relay
	// rejects the envelope with 501 "Bad sender address syntax" (#192).
	envelope string
}

func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	if cfg.Host == "" {
		return nil, errors.New("smtp: host is required")
	}
	if cfg.Port == 0 {
		return nil, errors.New("smtp: port is required")
	}
	if cfg.From == "" {
		return nil, errors.New("smtp: from address is required")
	}
	// Degrade gracefully: if From doesn't parse (a malformed secret), fall back
	// to the raw value rather than failing construction — a bad EMAIL_FROM_ADDRESS
	// must never crash boot. The send may still 501, but that's no worse than today.
	envelope := cfg.From
	if addr, err := mail.ParseAddress(cfg.From); err == nil {
		envelope = addr.Address
	}
	return &SMTPMailer{cfg: cfg, envelope: envelope}, nil
}

func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	if msg.To == "" || msg.Subject == "" {
		return errors.New("smtp: to and subject are required")
	}
	if msg.HTML == "" && msg.Text == "" {
		return errors.New("smtp: at least one of HTML or Text is required")
	}

	body, err := buildMultipartMessage(m.cfg.From, msg)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	// Envelope reverse-path is the bare address; the display-name form (if any)
	// lives only in the From: header built above (#192).
	return smtp.SendMail(addr, auth, m.envelope, []string{msg.To}, body)
}

func buildMultipartMessage(from string, msg Message) ([]byte, error) {
	boundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	headers := []string{
		"From: " + from,
		"To: " + msg.To,
		"Subject: " + msg.Subject,
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="` + boundary + `"`,
	}
	for _, h := range headers {
		buf.WriteString(h)
		buf.WriteString("\r\n")
	}
	buf.WriteString("\r\n")

	if msg.Text != "" {
		writePart(&buf, boundary, "text/plain; charset=utf-8", msg.Text)
	}
	if msg.HTML != "" {
		writePart(&buf, boundary, "text/html; charset=utf-8", msg.HTML)
	}
	buf.WriteString("--")
	buf.WriteString(boundary)
	buf.WriteString("--\r\n")

	return buf.Bytes(), nil
}

func writePart(buf *bytes.Buffer, boundary, contentType, body string) {
	buf.WriteString("--")
	buf.WriteString(boundary)
	buf.WriteString("\r\n")
	buf.WriteString("Content-Type: ")
	buf.WriteString(contentType)
	buf.WriteString("\r\n\r\n")
	buf.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	buf.WriteString("\r\n")
}

func randomBoundary() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "balances-" + base64.RawURLEncoding.EncodeToString(b), nil
}
