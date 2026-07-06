package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// defaultSendTimeout bounds a Send call when the caller's context carries no
// deadline of its own — without it, a hung relay (dead TCP peer, one that
// accepts the connection but never responds) blocks the caller forever, since
// neither net.Dial nor the smtp.Client calls below time out on their own.
const defaultSendTimeout = 30 * time.Second

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

// Send mirrors smtp.SendMail's dial → EHLO → opportunistic STARTTLS → AUTH →
// MAIL/RCPT/DATA sequence, reimplemented on top of the lower-level Client so
// the connection can be dialed with ctx and bounded by a deadline —
// smtp.SendMail itself takes no context and never times out, so a hung relay
// would otherwise block the caller (welcome/invite/reset email, all sent
// inline from a request) indefinitely.
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
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
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}

	deadline := time.Now().Add(defaultSendTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: set deadline: %w", err)
	}

	// SetDeadline alone only bounds the worst case; if ctx is canceled sooner
	// (the request that triggered this send hung up), abort the conn right
	// away instead of idling until the deadline.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer client.Close() //nolint:errcheck // best-effort cleanup; Quit above already reported the real outcome

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}

	if m.cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("smtp: server doesn't support AUTH")
		}
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	// Envelope reverse-path is the bare address; the display-name form (if any)
	// lives only in the From: header built above (#192).
	if err := client.Mail(m.envelope); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp: rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close data: %w", err)
	}

	return client.Quit()
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
