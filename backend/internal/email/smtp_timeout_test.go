package email

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// hungListener accepts one connection and then never speaks — no SMTP 220
// banner, nothing — modeling a relay that took the TCP handshake but hung
// (dead peer, firewall black hole). Before #358, Send had no dial/connection
// deadline and would block on this forever; these tests pin the two bounds
// that now apply.
func hungListener(t *testing.T) string {
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
		// Hold the connection open without writing the SMTP greeting.
		<-t.Context().Done()
		_ = conn.Close()
	}()
	return ln.Addr().String()
}

// A caller-supplied deadline shorter than defaultSendTimeout bounds Send to
// roughly that deadline, not the 30s fallback.
func TestSMTPMailer_SendHonorsContextDeadline(t *testing.T) {
	host, portStr, err := net.SplitHostPort(hungListener(t))
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	m, err := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "test@balances.local"})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = m.Send(ctx, Message{To: "recipient@example.com", Subject: "hi", Text: "hello"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want an error against a hung listener, got nil")
	}
	if elapsed > 5*time.Second {
		t.Errorf("Send took %s, want it bounded by the ~150ms context deadline, not defaultSendTimeout", elapsed)
	}
}

// Canceling ctx mid-send aborts the in-flight connection immediately rather
// than idling until SetDeadline's fixed instant.
func TestSMTPMailer_SendAbortsOnContextCancel(t *testing.T) {
	host, portStr, err := net.SplitHostPort(hungListener(t))
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	m, err := NewSMTPMailer(SMTPConfig{Host: host, Port: port, From: "test@balances.local"})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	err = m.Send(ctx, Message{To: "recipient@example.com", Subject: "hi", Text: "hello"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want an error after ctx cancellation, got nil")
	}
	if elapsed > 5*time.Second {
		t.Errorf("Send took %s, want it aborted shortly after the ~100ms cancel", elapsed)
	}
}
