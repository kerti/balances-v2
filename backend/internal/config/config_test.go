package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/config"
)

// All env keys Config reads. clearConfigEnv unsets every one (restoring on
// cleanup) so a developer's sourced .env doesn't leak ambient values into the
// defaults assertions. Empty-string is NOT equivalent to unset for caarlos0/env
// — envDefault only applies when the var is genuinely absent — so we Unsetenv
// rather than t.Setenv("").
var configEnvKeys = []string{
	"DATABASE_URL", "PORT", "LOG_FORMAT",
	"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "OIDC_ISSUER_URL",
	"AUTH_GOOGLE_ENABLED", "AUTH_LOCAL_ENABLED",
	"APP_URL", "OAUTH_REDIRECT_URL", "FRONTEND_URL", "BACKEND_URL",
	"SESSION_TTL", "COOKIE_SECURE",
	"EMAIL_ENABLED",
	"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD",
	"EMAIL_FROM_ADDRESS",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range configEnvKeys {
		if v, ok := os.LookupEnv(k); ok {
			_ = os.Unsetenv(k)
			k, v := k, v
			t.Cleanup(func() { _ = os.Setenv(k, v) })
		}
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL != "postgres://localhost/db" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", cfg.Port)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want default text", cfg.LogFormat)
	}
	if cfg.OIDCIssuerURL != "https://accounts.google.com" {
		t.Errorf("OIDCIssuerURL = %q", cfg.OIDCIssuerURL)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("SessionTTL = %v, want 720h", cfg.SessionTTL)
	}
	if cfg.CookieSecure {
		t.Errorf("CookieSecure = true, want default false")
	}
	if cfg.SMTPPort != 1025 {
		t.Errorf("SMTPPort = %d, want default 1025", cfg.SMTPPort)
	}
	if cfg.EmailFromAddress != "noreply@balances.local" {
		t.Errorf("EmailFromAddress = %q", cfg.EmailFromAddress)
	}
	// Mail is on by default (ADR-0037); a self-hoster opts out with EMAIL_ENABLED=false.
	if !cfg.EmailEnabled {
		t.Errorf("EmailEnabled = false, want default true")
	}
	// With neither APP_URL nor the individual URL vars set, the split-origin dev
	// defaults stand (Vite SPA on :5173, API on :8080).
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("FrontendURL = %q, want default http://localhost:5173", cfg.FrontendURL)
	}
	if cfg.BackendURL != "http://localhost:8080" {
		t.Errorf("BackendURL = %q, want default http://localhost:8080", cfg.BackendURL)
	}
	if cfg.OAuthRedirectURL != "http://localhost:8080/api/auth/google/callback" {
		t.Errorf("OAuthRedirectURL = %q, want default localhost callback", cfg.OAuthRedirectURL)
	}
	// Hosted posture by default (ADR-0039): Google on, local off.
	if !cfg.AuthGoogleEnabled {
		t.Errorf("AuthGoogleEnabled = false, want default true")
	}
	if cfg.AuthLocalEnabled {
		t.Errorf("AuthLocalEnabled = true, want default false")
	}
}

// TestLoad_FailsFastWhenNoProviderEnabled guards the boot check: a server with
// both identity providers disabled cannot sign anyone in, so Load errors rather
// than returning a config that boots a dead instance (ADR-0039).
func TestLoad_FailsFastWhenNoProviderEnabled(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("AUTH_GOOGLE_ENABLED", "false")
	t.Setenv("AUTH_LOCAL_ENABLED", "false")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load: expected an error when no auth provider is enabled")
	}
}

// TestLoad_LocalOnly is the minimal self-host posture: local on, Google off.
func TestLoad_LocalOnly(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("AUTH_GOOGLE_ENABLED", "false")
	t.Setenv("AUTH_LOCAL_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthGoogleEnabled || !cfg.AuthLocalEnabled {
		t.Errorf("local-only: got Google=%v Local=%v", cfg.AuthGoogleEnabled, cfg.AuthLocalEnabled)
	}
}

// TestLoad_AppURLDerivesOrigins: APP_URL alone collapses the three single-origin
// URLs to one operator-facing value — the post-login redirect (FrontendURL), the
// invite/welcome-email links (BackendURL), and the OAuth callback all resolve to
// that origin, with the callback suffix derived (no hand-typed path).
func TestLoad_AppURLDerivesOrigins(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_URL", "https://balances.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.FrontendURL != "https://balances.example.com" {
		t.Errorf("FrontendURL = %q, want APP_URL origin", cfg.FrontendURL)
	}
	if cfg.BackendURL != "https://balances.example.com" {
		t.Errorf("BackendURL = %q, want APP_URL origin", cfg.BackendURL)
	}
	if cfg.OAuthRedirectURL != "https://balances.example.com/api/auth/google/callback" {
		t.Errorf("OAuthRedirectURL = %q, want derived callback", cfg.OAuthRedirectURL)
	}
}

// TestLoad_AppURLTrailingSlash: a trailing slash on APP_URL must not double up
// when the callback suffix is appended.
func TestLoad_AppURLTrailingSlash(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_URL", "https://balances.example.com/")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OAuthRedirectURL != "https://balances.example.com/api/auth/google/callback" {
		t.Errorf("OAuthRedirectURL = %q, want single-slash derived callback", cfg.OAuthRedirectURL)
	}
	if cfg.FrontendURL != "https://balances.example.com" {
		t.Errorf("FrontendURL = %q, want trimmed origin", cfg.FrontendURL)
	}
}

// TestLoad_AppURLWithExplicitOverride: an explicitly-set individual URL still
// wins over the APP_URL-derived default, preserving the split-origin dev path
// and the Fly deployment's per-var configuration.
func TestLoad_AppURLWithExplicitOverride(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_URL", "https://balances.example.com")
	t.Setenv("OAUTH_REDIRECT_URL", "https://auth.example.com/api/auth/google/callback")
	t.Setenv("FRONTEND_URL", "https://app.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OAuthRedirectURL != "https://auth.example.com/api/auth/google/callback" {
		t.Errorf("OAuthRedirectURL = %q, want explicit override", cfg.OAuthRedirectURL)
	}
	if cfg.FrontendURL != "https://app.example.com" {
		t.Errorf("FrontendURL = %q, want explicit override", cfg.FrontendURL)
	}
	// BackendURL was not overridden, so it still derives from APP_URL.
	if cfg.BackendURL != "https://balances.example.com" {
		t.Errorf("BackendURL = %q, want APP_URL origin", cfg.BackendURL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("PORT", "9999")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("SESSION_TTL", "1h")
	t.Setenv("COOKIE_SECURE", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.SessionTTL != time.Hour {
		t.Errorf("SessionTTL = %v, want 1h", cfg.SessionTTL)
	}
	if !cfg.CookieSecure {
		t.Errorf("CookieSecure = false, want true")
	}
}

// TestLoad_EmailDisabled: EMAIL_ENABLED=false flips the gate that main reads to
// wire a no-op Mailer and skip SMTP construction (ADR-0037, self-host).
func TestLoad_EmailDisabled(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("EMAIL_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.EmailEnabled {
		t.Errorf("EmailEnabled = true, want false")
	}
}

func TestLoad_MissingRequiredDatabaseURL(t *testing.T) {
	clearConfigEnv(t) // unsets DATABASE_URL too

	if _, err := config.Load(); err == nil {
		t.Fatal("Load: want error when DATABASE_URL is unset, got nil")
	}
}

func TestLoad_InvalidValue(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("PORT", "not-a-number")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load: want error for non-integer PORT, got nil")
	}
}
