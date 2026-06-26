package auth

import (
	"testing"
	"time"
)

func newTestLimiter() (*loginLimiter, *time.Time) {
	now := time.Now()
	l := newLoginLimiter()
	l.now = func() time.Time { return now }
	return l, &now
}

func TestLimiter_FirstFailureNoDelay(t *testing.T) {
	l, _ := newTestLimiter()
	l.recordFailure("ip:1.2.3.4")
	// First failure is gentle (a typo) — still allowed immediately.
	if !l.allowed("ip:1.2.3.4") {
		t.Error("first failure should not block")
	}
}

func TestLimiter_BacksOffExponentially(t *testing.T) {
	l, now := newTestLimiter()
	key := "email:a@b.com"
	// Two failures → base (1s) wait.
	l.recordFailure(key)
	l.recordFailure(key)
	if l.allowed(key) {
		t.Fatal("second failure should impose a wait")
	}
	if d := l.maxRetryAfter(key); d <= 0 || d > time.Second {
		t.Errorf("retryAfter after 2 failures = %v, want (0,1s]", d)
	}
	// Advancing past the window re-allows.
	*now = now.Add(2 * time.Second)
	if !l.allowed(key) {
		t.Error("should be allowed again after the backoff window elapses")
	}
}

func TestLimiter_SuccessClears(t *testing.T) {
	l, _ := newTestLimiter()
	key := "email:a@b.com"
	l.recordFailure(key)
	l.recordFailure(key)
	l.recordFailure(key)
	l.recordSuccess(key)
	if !l.allowed(key) {
		t.Error("a successful login should clear the backoff")
	}
}

// TestLimiter_NoHardLockout asserts the cap is a ceiling, never an infinite
// block — even after many failures, waiting out the capped window re-allows.
func TestLimiter_NoHardLockout(t *testing.T) {
	l, now := newTestLimiter()
	key := "ip:9.9.9.9"
	for i := 0; i < 50; i++ {
		l.recordFailure(key)
	}
	if d := l.maxRetryAfter(key); d > l.cap {
		t.Errorf("backoff %v exceeded the cap %v — not a soft limit", d, l.cap)
	}
	*now = now.Add(l.cap + time.Second)
	if !l.allowed(key) {
		t.Error("after waiting out the capped window the key must be allowed (no hard lock)")
	}
}

func TestLimiter_KeysIndependent(t *testing.T) {
	l, _ := newTestLimiter()
	l.recordFailure("ip:1.1.1.1")
	l.recordFailure("ip:1.1.1.1")
	if !l.allowed("ip:2.2.2.2") {
		t.Error("backoff on one key must not affect a different key")
	}
}
