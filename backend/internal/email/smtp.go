package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
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
	// fromErr, when non-nil, marks the mailer as unable to send: cfg.From did
	// not yield a usable RFC5321 reverse-path. Send returns it instead of
	// shipping a bad envelope the relay silently 501s on (#195). Recorded here
	// rather than returned from the constructor so a malformed
	// EMAIL_FROM_ADDRESS can't crash boot — mail is best-effort.
	fromErr error
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
	// Strip CR/LF before the value reaches the From: header or the envelope: a
	// From carrying line breaks is either corruption (a secret with a stray
	// trailing newline — the common real case) or an SMTP header-injection
	// attempt smuggling extra headers into buildMultipartMessage's output.
	// Stripping rescues the trailing-newline case and neutralizes injection.
	cfg.From = stripLineBreaks(cfg.From)

	m := &SMTPMailer{cfg: cfg}
	addr, err := mail.ParseAddress(cfg.From)
	if err != nil {
		// Don't crash boot on a malformed EMAIL_FROM_ADDRESS — record the
		// failure so Send refuses (returns an error) rather than silently
		// shipping a bad reverse-path and letting the relay 501 it (#195).
		m.fromErr = fmt.Errorf("smtp: unusable from address %q: %w", cfg.From, err)
		slog.Error("smtp mailer: unparseable EMAIL_FROM_ADDRESS; email sends will be refused until it is fixed",
			"from", cfg.From, "err", err)
		return m, nil
	}
	m.envelope = addr.Address
	return m, nil
}

func (m *SMTPMailer) Send(_ context.Context, msg Message) error {
	if m.fromErr != nil {
		return m.fromErr
	}
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

// stripLineBreaks removes CR and LF so a From value can't inject extra SMTP
// headers into the message buildMultipartMessage emits, and so a secret with a
// stray trailing newline still parses to a clean address.
func stripLineBreaks(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

func randomBoundary() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "balances-" + base64.RawURLEncoding.EncodeToString(b), nil
}
