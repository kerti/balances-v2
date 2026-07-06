package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
)

// fakeSMTPServer starts a minimal scripted SMTP server on an ephemeral port
// and runs handle against the one connection it accepts. Used to provoke
// protocol-level error branches (e.g. a relay that never advertises AUTH) a
// real server is impractical to coax into in a unit test.
func fakeSMTPServer(t *testing.T, handle func(r *bufio.Reader, conn net.Conn)) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		handle(bufio.NewReader(conn), conn)
	}()

	h, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return h, p
}

// Nothing listening on the dialed address: Send must surface a wrapped dial
// error rather than hang or panic.
func TestSMTPMailer_SendDialError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	_ = ln.Close() // nothing listens on this port anymore

	m, err := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "test@balances.local"})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}

	err = m.Send(context.Background(), Message{To: "recipient@example.com", Subject: "hi", Text: "hello"})
	if err == nil {
		t.Fatal("want a dial error against a closed port, got nil")
	}
	if !strings.Contains(err.Error(), "smtp: dial") {
		t.Errorf("err = %q, want it wrapped with %q", err, "smtp: dial")
	}
}

// A malformed greeting (anything other than the 220 banner) must surface as a
// wrapped "new client" error rather than a confusing protocol panic.
func TestSMTPMailer_SendBadGreetingError(t *testing.T) {
	host, port := fakeSMTPServer(t, func(_ *bufio.Reader, conn net.Conn) {
		_, _ = fmt.Fprintf(conn, "500 not an smtp server\r\n")
	})

	m, err := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "test@balances.local"})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}

	err = m.Send(context.Background(), Message{To: "recipient@example.com", Subject: "hi", Text: "hello"})
	if err == nil {
		t.Fatal("want an error against a non-220 greeting, got nil")
	}
	if !strings.Contains(err.Error(), "smtp: new client") {
		t.Errorf("err = %q, want it wrapped with %q", err, "smtp: new client")
	}
}

// A relay whose EHLO capabilities omit AUTH must fail loudly when the mailer
// is configured with credentials — silently sending unauthenticated (or
// hanging) would either get the message rejected downstream or mask a
// misconfigured SMTP_USERNAME.
func TestSMTPMailer_SendErrorsWhenAuthUnsupported(t *testing.T) {
	host, port := fakeSMTPServer(t, func(r *bufio.Reader, conn net.Conn) {
		_, _ = fmt.Fprintf(conn, "220 fake.local ESMTP\r\n")
		line, err := r.ReadString('\n') // EHLO
		if err != nil || !strings.HasPrefix(strings.ToUpper(line), "EHLO") {
			return
		}
		_, _ = fmt.Fprintf(conn, "250-fake.local\r\n250 8BITMIME\r\n") // no AUTH advertised
	})

	m, err := NewSMTPMailer(SMTPConfig{
		Host:     host,
		Port:     port,
		From:     "test@balances.local",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}

	err = m.Send(context.Background(), Message{To: "recipient@example.com", Subject: "hi", Text: "hello"})
	if err == nil {
		t.Fatal("want an error when the server doesn't advertise AUTH, got nil")
	}
	if !strings.Contains(err.Error(), "doesn't support AUTH") {
		t.Errorf("err = %q, want it to mention unsupported AUTH", err)
	}
}
