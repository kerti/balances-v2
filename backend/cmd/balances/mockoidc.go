package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// mock-oidc is a tiny, self-contained OpenID Connect provider used only by the
// Playwright E2E suite (ADR-0024 option B). It exists so the E2E backend can run
// the *real* login flow — button → /auth/google/start → IdP authorize → callback
// → session — against a local fake instead of contacting accounts.google.com.
//
// It is NOT wired into serve and never ships in the app binary's request path:
// it's a separate subcommand the Makefile launches alongside the E2E backend,
// which points at it via OIDC_ISSUER_URL. The signing key is generated fresh on
// each boot; tokens live only as long as the process.
//
// Defaults match the values playwright.config.ts feeds the E2E backend; the
// MOCK_OIDC_* env vars exist so the Makefile can override without code edits.
const (
	mockOIDCDefaultAddr         = ":8090"
	mockOIDCDefaultIssuer       = "http://localhost:8090"
	mockOIDCDefaultClientID     = "e2e-client"
	mockOIDCDefaultClientSecret = "e2e-secret"
	mockOIDCKeyID               = "e2e-key-1"

	// mockOIDCSubCookie lets an E2E request a brand-new, never-seeded identity.
	// The default flow issues the seeded Alice (sub=e2e-alice), so every sign-in
	// takes the existing-user branch and can never reach the onboarding gate
	// (#274). When this cookie is set on the mock-oidc origin — the browser sends
	// it on the top-level redirect to /authorize — its value becomes the id_token
	// `sub` and the email/name are derived, guaranteeing GetUserByGoogleSub misses
	// so the callback falls through to the ADR-0038 gate. Test-only; never in the
	// app binary's request path (see the file header). onboarding.spec.ts drives it.
	mockOIDCSubCookie = "mock_oidc_sub"
)

// mockIdentity is the set of claims the mock issues in an id_token. The seeded
// Alice is the default; a cookie-selected fresh `sub` derives the rest.
type mockIdentity struct {
	sub, email, name, picture string
}

// identityFromRequest resolves which identity /authorize should mint a code for.
// A `mock_oidc_sub` cookie selects a brand-new identity (email/name derived from
// the sub so it is guaranteed unseeded); its absence yields the seeded Alice.
func identityFromRequest(r *http.Request) mockIdentity {
	if c, err := r.Cookie(mockOIDCSubCookie); err == nil && c.Value != "" {
		return mockIdentity{
			sub:     c.Value,
			email:   c.Value + "@e2e.example.com",
			name:    "E2E Newcomer",
			picture: e2eAlicePictureURL,
		}
	}
	return mockIdentity{
		sub:     e2eAliceGoogleSub,
		email:   e2eAliceEmail,
		name:    e2eAliceName,
		picture: e2eAlicePictureURL,
	}
}

type mockOIDC struct {
	issuer       string
	clientID     string
	clientSecret string
	priv         *rsa.PrivateKey
	signer       jose.Signer

	mu    sync.Mutex
	codes map[string]mockIdentity // outstanding single-use codes → the identity to mint
}

func seedE2EEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newMockOIDC() (*mockOIDC, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       jose.JSONWebKey{Key: priv, KeyID: mockOIDCKeyID, Algorithm: string(jose.RS256), Use: "sig"},
		},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return nil, fmt.Errorf("new signer: %w", err)
	}
	return &mockOIDC{
		issuer:       seedE2EEnv("MOCK_OIDC_ISSUER", mockOIDCDefaultIssuer),
		clientID:     seedE2EEnv("MOCK_OIDC_CLIENT_ID", mockOIDCDefaultClientID),
		clientSecret: seedE2EEnv("MOCK_OIDC_CLIENT_SECRET", mockOIDCDefaultClientSecret),
		priv:         priv,
		signer:       signer,
		codes:        make(map[string]mockIdentity),
	}, nil
}

func (m *mockOIDC) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", m.handleDiscovery)
	mux.HandleFunc("/jwks", m.handleJWKS)
	mux.HandleFunc("/authorize", m.handleAuthorize)
	mux.HandleFunc("/token", m.handleToken)
	// Serves the avatar referenced by the `picture` claim in signIDToken, so
	// the browser has a real image to load when the e2e suite drives the OAuth
	// flow. Tiny transparent 1x1 PNG — content doesn't matter, just that the
	// fetch succeeds and UserAvatar's onError doesn't fire.
	mux.HandleFunc("/avatar.png", m.handleAvatar)
	return mux
}

// onePixelPNG is a transparent 1x1 PNG, the smallest valid image the browser
// will accept without an onError. Decoded once at package init.
var onePixelPNG = mustDecodeBase64(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
)

func mustDecodeBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func (m *mockOIDC) handleAvatar(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(onePixelPNG)
}

func (m *mockOIDC) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                m.issuer,
		"authorization_endpoint":                m.issuer + "/authorize",
		"token_endpoint":                        m.issuer + "/token",
		"jwks_uri":                              m.issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "email", "profile"},
	})
}

func (m *mockOIDC) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &m.priv.PublicKey,
		KeyID:     mockOIDCKeyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// handleAuthorize approves immediately (no login form) and bounces the browser
// back to redirect_uri with a fresh code + the caller's state, mirroring a user
// who has just consented at the IdP.
func (m *mockOIDC) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != m.clientID {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}
	redirectURI := q.Get("redirect_uri")
	if _, err := url.ParseRequestURI(redirectURI); err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	code, err := randomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	m.mu.Lock()
	m.codes[code] = identityFromRequest(r)
	m.mu.Unlock()

	dest, _ := url.Parse(redirectURI)
	rq := dest.Query()
	rq.Set("code", code)
	if state := q.Get("state"); state != "" {
		rq.Set("state", state)
	}
	dest.RawQuery = rq.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}

// handleToken validates the client credentials (accepting either client_secret_basic
// or client_secret_post, so we don't depend on oauth2's auth-style autodetection)
// and the single-use code, then returns a signed id_token for the fixture user.
func (m *mockOIDC) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id, secret := r.PostFormValue("client_id"), r.PostFormValue("client_secret")
	if u, p, ok := r.BasicAuth(); ok {
		id, secret = u, p
	}
	if id != m.clientID || secret != m.clientSecret {
		http.Error(w, "invalid client credentials", http.StatusUnauthorized)
		return
	}

	code := r.PostFormValue("code")
	m.mu.Lock()
	identity, ok := m.codes[code]
	delete(m.codes, code)
	m.mu.Unlock()
	if !ok {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}

	idToken, err := m.signIDToken(identity)
	if err != nil {
		slog.Error("mock-oidc sign id_token", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"access_token": "mock-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

func (m *mockOIDC) signIDToken(id mockIdentity) (string, error) {
	now := time.Now()
	claims := map[string]any{
		"iss":            m.issuer,
		"aud":            m.clientID,
		"sub":            id.sub,
		"email":          id.email,
		"email_verified": true,
		"name":           id.name,
		// `picture` is the optional Google claim the OAuth callback persists via
		// SetUserPicture. Including it here lets picture.spec.ts assert the
		// backfill end-to-end: seeded Alice (picture_url NULL) → sign in →
		// avatar renders the image instead of initials.
		"picture": id.picture,
		"iat":     now.Unix(),
		"exp":     now.Add(time.Hour).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	obj, err := m.signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return obj.CompactSerialize()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// randomState is defined in the auth package for the real flow; mock-oidc needs
// its own copy since cmd/main can't import an unexported helper. Kept tiny.
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func mockOIDCCmd() error {
	m, err := newMockOIDC()
	if err != nil {
		return err
	}
	addr := seedE2EEnv("MOCK_OIDC_ADDR", mockOIDCDefaultAddr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := &http.Server{
		Addr:              addr,
		Handler:           m.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "mock-oidc listening on %s (issuer %s, client %s)\n", addr, m.issuer, m.clientID)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
