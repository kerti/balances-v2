package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Split-origin dev defaults: Vite serves the SPA on :5173 and proxies /api to
// the Go server on :8080. Used when neither APP_URL nor the individual URL var
// is set.
const (
	defaultFrontendURL      = "http://localhost:5173"
	defaultBackendURL       = "http://localhost:8080"
	defaultOAuthRedirectURL = "http://localhost:8080/api/auth/google/callback"
	oauthCallbackPath       = "/api/auth/google/callback"
)

type Config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`
	Port        int    `env:"PORT" envDefault:"8080"`
	LogFormat   string `env:"LOG_FORMAT" envDefault:"text"`

	// WebDir, when set, makes the server serve the built SPA from that directory
	// (single-origin production, ADR-0030) with an index.html fallback for
	// client-side routes. Unset in dev, where Vite serves the SPA and proxies /api.
	WebDir string `env:"WEB_DIR"`

	// ShutdownTimeout bounds how long graceful shutdown waits for in-flight
	// requests to drain before forcing the server closed. Adjustable so dev
	// restarts can use a short grace period; production keeps a longer one.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	OIDCIssuerURL      string `env:"OIDC_ISSUER_URL" envDefault:"https://accounts.google.com"`

	// AppURL is the operator-facing single-origin URL for a self-host deployment
	// (ADR-0037). When set, it supplies the default origin for FrontendURL and
	// BackendURL and derives OAuthRedirectURL as AppURL + the callback path —
	// removing the OAuth-redirect footgun (one origin, no hand-typed suffix). The
	// three individual vars below remain explicitly overridable, so the
	// split-origin dev path and the Fly deployment are unaffected. Derivation
	// happens in Load (see applyURLDefaults); these fields carry no envDefault so
	// an unset value stays empty and can be distinguished from an explicit one.
	AppURL           string        `env:"APP_URL"`
	OAuthRedirectURL string        `env:"OAUTH_REDIRECT_URL"`
	FrontendURL      string        `env:"FRONTEND_URL"`
	BackendURL       string        `env:"BACKEND_URL"`
	SessionTTL       time.Duration `env:"SESSION_TTL" envDefault:"720h"`
	CookieSecure     bool          `env:"COOKIE_SECURE" envDefault:"false"`

	// EmailEnabled gates all outbound transactional mail (ADR-0037). The default
	// is true, preserving current behaviour. A self-hoster who wants no mail
	// dependency sets EMAIL_ENABLED=false: main wires a no-op Mailer and skips
	// SMTP construction entirely, so the app boots and runs with no SMTP config.
	// The only mail with a hard dependency — invitations — falls back to the
	// "copy invite link" affordance on the invitation flow (the create endpoint
	// already returns the AcceptURL); welcome and restore mails silently no-op.
	EmailEnabled bool `env:"EMAIL_ENABLED" envDefault:"true"`

	SMTPHost         string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort         int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUsername     string `env:"SMTP_USERNAME"`
	SMTPPassword     string `env:"SMTP_PASSWORD"`
	EmailFromAddress string `env:"EMAIL_FROM_ADDRESS" envDefault:"noreply@balances.local"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("env parse: %w", err)
	}
	applyURLDefaults(&cfg)
	return &cfg, nil
}

// applyURLDefaults fills the single-origin URL fields. An explicitly-set
// FRONTEND_URL / BACKEND_URL / OAUTH_REDIRECT_URL always wins. Otherwise, when
// APP_URL is set, the missing ones derive from its origin (callback path
// appended); when APP_URL is unset too, the split-origin localhost dev defaults
// stand.
func applyURLDefaults(cfg *Config) {
	if origin := strings.TrimRight(cfg.AppURL, "/"); origin != "" {
		if cfg.FrontendURL == "" {
			cfg.FrontendURL = origin
		}
		if cfg.BackendURL == "" {
			cfg.BackendURL = origin
		}
		if cfg.OAuthRedirectURL == "" {
			cfg.OAuthRedirectURL = origin + oauthCallbackPath
		}
	}
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = defaultFrontendURL
	}
	if cfg.BackendURL == "" {
		cfg.BackendURL = defaultBackendURL
	}
	if cfg.OAuthRedirectURL == "" {
		cfg.OAuthRedirectURL = defaultOAuthRedirectURL
	}
}
