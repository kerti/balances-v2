package auth

import (
	"sync"
	"time"
)

// loginLimiter is the soft online-guessing brake for local password login
// (ADR-0039). It is deliberately *not* a hard lockout: a household instance must
// never be able to lock itself (or a roommate) out by fat-fingering a password,
// so a key is only ever delayed, never permanently blocked. Failures grow the
// required wait exponentially up to a cap; a success clears the key.
//
// Keyed twice per attempt — once by client IP, once by the typed email — so
// neither a single IP hammering many emails nor many IPs hammering one email
// slips the brake. In-memory and mutex-guarded: no Redis dependency on an SBC,
// and a process restart simply resets the (transient) backoff state.
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	now     func() time.Time // injectable for tests

	base time.Duration // delay after the first failure
	cap  time.Duration // ceiling on the exponential growth
}

type limiterEntry struct {
	failures     int
	blockedUntil time.Time
}

// newLoginLimiter builds a limiter with the production backoff curve: the first
// failure costs nothing, then the wait doubles from base (1s) up to cap (15m).
func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		entries: make(map[string]*limiterEntry),
		now:     time.Now,
		base:    1 * time.Second,
		cap:     15 * time.Minute,
	}
}

// retryAfter returns how long the key must wait before another attempt is
// allowed; zero means allowed now. Callers check every key (IP and email) and
// deny if any is currently blocked.
func (l *loginLimiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return 0
	}
	if d := e.blockedUntil.Sub(l.now()); d > 0 {
		return d
	}
	return 0
}

// allowed reports whether none of the given keys is currently in backoff.
func (l *loginLimiter) allowed(keys ...string) bool {
	return l.maxRetryAfter(keys...) == 0
}

// maxRetryAfter returns the longest remaining backoff across the given keys;
// zero means every key is currently allowed.
func (l *loginLimiter) maxRetryAfter(keys ...string) time.Duration {
	var longest time.Duration
	for _, k := range keys {
		if d := l.retryAfter(k); d > longest {
			longest = d
		}
	}
	return longest
}

// recordFailure bumps the failure count for each key and extends its backoff
// window. The window after failure n is base * 2^(n-1), capped — so 1s, 2s, 4s,
// … 15m, and then a flat 15m forever after (never a hard lock).
func (l *loginLimiter) recordFailure(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, k := range keys {
		e := l.entries[k]
		if e == nil {
			e = &limiterEntry{}
			l.entries[k] = e
		}
		e.failures++
		wait := l.backoff(e.failures)
		e.blockedUntil = now.Add(wait)
	}
}

// recordSuccess clears the keys — a correct login wipes the slate so a returning
// member is never throttled by their own earlier typos.
func (l *loginLimiter) recordSuccess(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.entries, k)
	}
}

// backoff computes the wait for a given failure count: 0 on the first failure
// (gentle for a typo), then base doubling up to cap.
func (l *loginLimiter) backoff(failures int) time.Duration {
	if failures <= 1 {
		return 0
	}
	wait := l.base
	for i := 2; i < failures; i++ {
		wait *= 2
		if wait >= l.cap {
			return l.cap
		}
	}
	if wait > l.cap {
		return l.cap
	}
	return wait
}
