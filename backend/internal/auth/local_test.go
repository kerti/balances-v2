package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/clientip"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// post issues a JSON POST carrying the given cookies and returns the recorder.
func (h *authHarness) post(t *testing.T, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func envelopeCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope (status %d): %v", rec.Code, err)
	}
	return env.Code
}

func TestAuthMethods_ReportsEnabledProviders(t *testing.T) {
	h := newAuthHarness(t)
	rec := h.doRaw(t, http.MethodGet, "/auth/methods", nil, nil)
	requireStatus(t, rec, http.StatusOK)
	got := decodeBody[authMethodsResponse](t, rec)
	if !got.Google || !got.Local {
		t.Errorf("methods: want both enabled, got %+v", got)
	}
	if got.DemoMode {
		t.Errorf("methods: want demo_mode false by default, got %+v", got)
	}
	if got.DemoEmail != "" || got.DemoPassword != "" {
		t.Errorf("methods: want no demo credentials when demo_mode is off, got %+v", got)
	}
}

// covers: INV-AUTH-33
func TestAuthMethods_FoundingOpenMirrorsConfig(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		h := newAuthHarness(t)
		h.h.foundingDisabled = disabled
		rec := h.doRaw(t, http.MethodGet, "/auth/methods", nil, nil)
		requireStatus(t, rec, http.StatusOK)
		got := decodeBody[authMethodsResponse](t, rec)
		if got.FoundingOpen == disabled {
			t.Errorf("foundingDisabled=%v: want founding_open=%v, got %v", disabled, !disabled, got.FoundingOpen)
		}
	}
}

// covers: INV-AUTH-27
func TestAuthMethods_DemoModeExposesSharedCredentials(t *testing.T) {
	h := newAuthHarness(t)
	h.h.demoMode = true
	h.h.demoEmail = "demo@balances.local"
	h.h.demoPassword = "BalancesDemo!2026"

	rec := h.doRaw(t, http.MethodGet, "/auth/methods", nil, nil)
	requireStatus(t, rec, http.StatusOK)
	got := decodeBody[authMethodsResponse](t, rec)
	if !got.DemoMode {
		t.Errorf("methods: want demo_mode true, got %+v", got)
	}
	if got.DemoEmail != "demo@balances.local" || got.DemoPassword != "BalancesDemo!2026" {
		t.Errorf("methods: want the shared demo credentials exposed, got %+v", got)
	}
}

// TestLocalFounder_RegisterThroughGateThenLogin is the founder tracer bullet
// (#280): register locally → commit the founder choice at the onboarding gate →
// a local User with no google_sub and a credential row exists → log back in.
//
// covers: INV-AUTH-15
func TestLocalFounder_RegisterThroughGateThenLogin(t *testing.T) {
	h := newAuthHarness(t)
	const email = "founder@example.com"
	const password = "a-decent-founder-passphrase"

	// 1. Register — writes no users row yet, only a handshake + cookie.
	reg := h.post(t, "/auth/local/register", map[string]string{
		"email": email, "password": password, "display_name": "Pat",
	})
	requireStatus(t, reg, http.StatusNoContent)
	hsCookie := findCookie(reg, onboardingCookieName)
	if hsCookie == nil {
		t.Fatal("register did not set the onboarding handshake cookie")
	}
	if _, err := h.q.GetUserByEmail(context.Background(), email); err == nil {
		t.Fatal("register created a users row before the gate commit")
	}

	// 2. Commit the founder choice at the gate.
	commit := h.post(t, "/onboarding/choice", map[string]any{"found": true}, hsCookie)
	requireStatus(t, commit, http.StatusNoContent)
	if findCookie(commit, sessionCookieName) == nil {
		t.Fatal("gate commit did not mint a session")
	}

	// 3. The User exists, is google_sub-less, and has a credential row.
	user, err := h.q.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("user not created at gate: %v", err)
	}
	if user.GoogleSub != nil {
		t.Errorf("local founder has a google_sub %v; want nil", *user.GoogleSub)
	}
	if _, err := h.q.GetLocalCredentialByUserID(context.Background(), user.ID); err != nil {
		t.Errorf("no local_credentials row for founder: %v", err)
	}

	// 4. Log back in with the same credentials.
	login := h.post(t, "/auth/local/login", map[string]string{"email": email, "password": password})
	requireStatus(t, login, http.StatusNoContent)
	if findCookie(login, sessionCookieName) == nil {
		t.Error("login did not mint a session")
	}
}

func TestLocalRegister_RejectsWeakPassword(t *testing.T) {
	h := newAuthHarness(t)
	short := h.post(t, "/auth/local/register", map[string]string{
		"email": "x@example.com", "password": "short",
	})
	requireStatus(t, short, http.StatusBadRequest)
	if code := envelopeCode(t, short); code != string(httperr.CodeWeakPassword) {
		t.Errorf("short password code = %q, want WEAK_PASSWORD", code)
	}

	breached := h.post(t, "/auth/local/register", map[string]string{
		"email": "y@example.com", "password": "password123",
	})
	requireStatus(t, breached, http.StatusBadRequest)
	if code := envelopeCode(t, breached); code != string(httperr.CodeWeakPassword) {
		t.Errorf("breached password code = %q, want WEAK_PASSWORD", code)
	}
}

func TestLocalRegister_RejectsInvalidEmail(t *testing.T) {
	h := newAuthHarness(t)
	rec := h.post(t, "/auth/local/register", map[string]string{
		"email": "not-an-email", "password": "a-decent-passphrase-here",
	})
	requireStatus(t, rec, http.StatusBadRequest)
	if code := envelopeCode(t, rec); code != string(httperr.CodeValidation) {
		t.Errorf("invalid email code = %q, want VALIDATION", code)
	}
}

// TestLocalRegister_DerivesDisplayNameFromEmail: an omitted display name falls
// back to the email's local part, carried through the gate to the founder row.
func TestLocalRegister_DerivesDisplayNameFromEmail(t *testing.T) {
	h := newAuthHarness(t)
	reg := h.post(t, "/auth/local/register", map[string]string{
		"email": "solo@example.com", "password": "a-decent-founder-passphrase",
	})
	requireStatus(t, reg, http.StatusNoContent)
	hsCookie := findCookie(reg, onboardingCookieName)
	if hsCookie == nil {
		t.Fatal("no handshake cookie")
	}
	commit := h.post(t, "/onboarding/choice", map[string]any{"found": true}, hsCookie)
	requireStatus(t, commit, http.StatusNoContent)

	user, err := h.q.GetUserByEmail(context.Background(), "solo@example.com")
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if user.DisplayName != "solo" {
		t.Errorf("display name = %q, want derived local part %q", user.DisplayName, "solo")
	}
}

func TestLocalRegister_RejectsDuplicateEmail(t *testing.T) {
	h := newAuthHarness(t)
	// Seed a live (lowercase-email) account, then try to register the same email.
	h.seedLocalUser(t, "taken@example.com", "an-existing-passphrase")
	dup := h.post(t, "/auth/local/register", map[string]string{
		"email": "taken@example.com", "password": "a-decent-passphrase-here",
	})
	requireStatus(t, dup, http.StatusConflict)
	if code := envelopeCode(t, dup); code != string(httperr.CodeEmailTaken) {
		t.Errorf("duplicate email code = %q, want EMAIL_TAKEN", code)
	}
}

// seedLocalUser creates a live local user with a known password directly,
// bypassing the gate — for login-path tests that don't need the register flow.
func (h *authHarness) seedLocalUser(t *testing.T, email, password string) db.User {
	t.Helper()
	user, err := h.q.CreateLocalUser(context.Background(), db.CreateLocalUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Local",
		Email:       email,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("seed local user: %v", err)
	}
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := h.q.UpsertLocalCredential(context.Background(), db.UpsertLocalCredentialParams{
		UserID: user.ID, PasswordHash: hash,
	}); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	return user
}

// TestLocalLogin_NoEnumeration asserts a wrong password, an unknown email, and a
// credential-less (dormant/Google-only) user all return the same generic 401 —
// login never reveals whether an account exists.
//
// covers: INV-AUTH-17
func TestLocalLogin_NoEnumeration(t *testing.T) {
	h := newAuthHarness(t)
	h.seedLocalUser(t, "real@example.com", "the-real-passphrase")
	// A dormant user: has an identity (google_sub) but no local credential.
	if _, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Dormant",
		Email:       "dormant@example.com",
		GoogleSub:   "sub-dormant",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	}); err != nil {
		t.Fatalf("seed dormant user: %v", err)
	}

	cases := []struct{ name, email, password string }{
		{"wrong password", "real@example.com", "wrong-passphrase-xx"},
		{"unknown email", "ghost@example.com", "any-passphrase-here"},
		{"dormant credential-less user", "dormant@example.com", "any-passphrase-here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh limiter per case: this test asserts the failure *shape* is
			// identical across modes, not the cross-attempt backoff (covered
			// separately). All httptest requests share one client IP, so without a
			// reset the per-IP brake would trip mid-suite.
			h.h.limiter = newLoginLimiter()
			rec := h.post(t, "/auth/local/login", map[string]string{"email": tc.email, "password": tc.password})
			requireStatus(t, rec, http.StatusUnauthorized)
			if code := envelopeCode(t, rec); code != string(httperr.CodeInvalidCredentials) {
				t.Errorf("code = %q, want INVALID_CREDENTIALS", code)
			}
		})
	}
}

// TestLocalLogin_RateLimited drives repeated failures until the soft limiter
// trips and returns 429 with a Retry-After header.
//
// covers: INV-AUTH-17
func TestLocalLogin_RateLimited(t *testing.T) {
	h := newAuthHarness(t)
	h.seedLocalUser(t, "target@example.com", "the-correct-passphrase")

	var got429 bool
	for i := 0; i < 6; i++ {
		rec := h.post(t, "/auth/local/login", map[string]string{
			"email": "target@example.com", "password": "deliberately-wrong",
		})
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			if rec.Header().Get("Retry-After") == "" {
				t.Error("429 response missing Retry-After header")
			}
			if code := envelopeCode(t, rec); code != string(httperr.CodeTooManyAttempts) {
				t.Errorf("code = %q, want TOO_MANY_ATTEMPTS", code)
			}
			break
		}
	}
	if !got429 {
		t.Error("repeated login failures never tripped the rate limiter")
	}
}

// TestHandlersClientIP pins the wiring from Handlers.trustedProxies to the
// extractor (the decision table lives in internal/clientip): a zero-value
// Handlers ignores forwarding headers, a configured one honours them only from
// a trusted peer.
//
// covers: INV-AUTH-34
func TestHandlersClientIP(t *testing.T) {
	cases := []struct {
		name       string
		trusted    string
		remoteAddr string
		flyIP      string
		xff        string
		want       string
	}{
		{"unset: forged headers ignored", "", "203.0.113.9:1234", "198.51.100.7", "198.51.100.1, 198.51.100.2", "203.0.113.9"},
		{"untrusted peer: forged headers ignored", "10.0.0.0/8", "203.0.113.9:1234", "198.51.100.7", "198.51.100.1", "203.0.113.9"},
		{"trusted peer: XFF client", "10.0.0.0/8", "10.0.0.2:1234", "", "198.51.100.1", "198.51.100.1"},
		{"fly preset: Fly-Client-IP", "fly", "172.19.0.2:1234", "198.51.100.7", "", "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trusted, err := clientip.Parse(tc.trusted)
			if err != nil {
				t.Fatal(err)
			}
			h := &Handlers{trustedProxies: trusted}
			req := httptest.NewRequest(http.MethodPost, "/auth/local/login", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.flyIP != "" {
				req.Header.Set("Fly-Client-IP", tc.flyIP)
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := h.clientIP(req); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// failedLogin posts a wrong-password login as the given peer with the given
// forwarding headers and returns the response.
func (h *authHarness) failedLogin(t *testing.T, email, remoteAddr, flyIP, xff string) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(map[string]string{
		"email": email, "password": "deliberately-wrong",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/local/login", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if flyIP != "" {
		req.Header.Set("Fly-Client-IP", flyIP)
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// TestLocalLogin_ForgedHeadersCannotDodgeLimiter is the bypass this fix closes
// (#755): a caller reaching the app directly (an untrusted peer) who sends a
// fresh Fly-Client-IP / X-Forwarded-For on every attempt must still land in one
// bucket — their own — and trip the backoff. Each attempt targets a different
// email so only the per-IP bucket can be what trips.
//
// covers: INV-AUTH-34
func TestLocalLogin_ForgedHeadersCannotDodgeLimiter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trusted string
		forge   func(i int) (flyIP, xff string)
	}{
		{"nothing trusted, forged Fly-Client-IP", "", func(i int) (string, string) { return fmt.Sprintf("198.51.100.%d", i+1), "" }},
		{"nothing trusted, forged XFF", "", func(i int) (string, string) { return "", fmt.Sprintf("198.51.100.%d", i+1) }},
		{"proxy list, untrusted peer, forged XFF", "10.0.0.0/8", func(i int) (string, string) { return "", fmt.Sprintf("198.51.100.%d", i+1) }},
		{"fly preset, public peer, forged Fly-Client-IP", "fly", func(i int) (string, string) { return fmt.Sprintf("198.51.100.%d", i+1), "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthHarness(t)
			trusted, err := clientip.Parse(tc.trusted)
			if err != nil {
				t.Fatal(err)
			}
			h.h.trustedProxies = trusted

			got429 := false
			for i := 0; i < 12 && !got429; i++ {
				flyIP, xff := tc.forge(i)
				got429 = h.failedLogin(t, fmt.Sprintf("target%d@example.com", i), "203.0.113.9:4000", flyIP, xff).Code == http.StatusTooManyRequests
			}
			if !got429 {
				t.Error("forging a new client address per attempt dodged the per-IP limiter")
			}
		})
	}
}

// TestLocalLogin_RateLimitIsolationBehindTrustedProxy is the regression the
// original fix targeted (#363), now on TRUSTED_PROXIES: two different real
// clients arriving through the same trusted proxy (identical RemoteAddr) must
// key the limiter on their distinct forwarded address, not collapse onto one
// shared bucket — otherwise an attacker hammering one household's login from
// behind the proxy would also lock out every other household sharing it. Also
// covers the two-proxy chain, where the real client sits left of an inner hop.
//
// covers: INV-AUTH-17, INV-AUTH-34
func TestLocalLogin_RateLimitIsolationBehindTrustedProxy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trusted string
		remote  string
		fly     bool
		header  func(client string) string
	}{
		{"one proxy via XFF", "192.0.2.0/24", "192.0.2.100:443", false, func(c string) string { return c }},
		{"two proxies via XFF", "192.0.2.0/24,198.18.0.0/15", "192.0.2.100:443", false, func(c string) string { return c + ", 198.18.0.7" }},
		{"fly preset via Fly-Client-IP", "fly", "172.19.0.2:443", true, func(c string) string { return c }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthHarness(t)
			trusted, err := clientip.Parse(tc.trusted)
			if err != nil {
				t.Fatal(err)
			}
			h.h.trustedProxies = trusted
			login := func(email, client string) *httptest.ResponseRecorder {
				if tc.fly {
					return h.failedLogin(t, email, tc.remote, tc.header(client), "")
				}
				return h.failedLogin(t, email, tc.remote, "", tc.header(client))
			}

			// Attacker (203.0.113.1) trips their own IP bucket.
			var attackerGot429 bool
			for i := 0; i < 6; i++ {
				if login("attacker-target@example.com", "203.0.113.1").Code == http.StatusTooManyRequests {
					attackerGot429 = true
					break
				}
			}
			if !attackerGot429 {
				t.Fatal("attacker never tripped the rate limiter")
			}

			// A different real client behind the same proxy must still get a
			// normal 401 — not swept up in the attacker's IP-bucket backoff. A
			// different email keeps the per-email bucket out of the picture.
			rec := login("victim@example.com", "203.0.113.2")
			requireStatus(t, rec, http.StatusUnauthorized)
		})
	}
}
