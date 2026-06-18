package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/kerti/balances-v2/backend/internal/assets"
	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/fxrates"
	"github.com/kerti/balances-v2/backend/internal/httpserver"
	"github.com/kerti/balances-v2/backend/internal/income"
	"github.com/kerti/balances-v2/backend/internal/investments"
	"github.com/kerti/balances-v2/backend/internal/liabilities"
	"github.com/kerti/balances-v2/backend/internal/migrations"
	"github.com/kerti/balances-v2/backend/internal/receivables"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/reports"
	"github.com/kerti/balances-v2/backend/internal/tags"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := serveCmd(); err != nil {
			slog.Error("server stopped", "err", err)
			os.Exit(1)
		}
	case "migrate":
		if err := migrateCmd(os.Args[2:]); err != nil {
			slog.Error("migrate failed", "err", err)
			os.Exit(1)
		}
	case "seed-e2e":
		if err := seedE2ECmd(); err != nil {
			fmt.Fprintln(os.Stderr, "seed-e2e failed:", err)
			os.Exit(1)
		}
	case "mock-oidc":
		if err := mockOIDCCmd(); err != nil {
			fmt.Fprintln(os.Stderr, "mock-oidc failed:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: balances <command> [args]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  serve              run the HTTP server")
	fmt.Fprintln(os.Stderr, "  migrate up         apply all pending migrations")
	fmt.Fprintln(os.Stderr, "  migrate up-by-one  apply the next migration")
	fmt.Fprintln(os.Stderr, "  migrate down       roll back one migration")
	fmt.Fprintln(os.Stderr, "  migrate status     show migration status")
	fmt.Fprintln(os.Stderr, "  migrate version    show current revision")
	fmt.Fprintln(os.Stderr, "  seed-e2e           reset the balances_e2e DB with Playwright fixtures")
	fmt.Fprintln(os.Stderr, "  mock-oidc          run the E2E fake OIDC provider (ADR-0024)")
}

func serveCmd() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	slog.SetDefault(newLogger(cfg))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := applyMigrations(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	// EMAIL_ENABLED=false (ADR-0037, self-host) wires a no-op Mailer and skips
	// SMTP construction entirely, so the app boots with no SMTP config set.
	// Invitations fall back to the "copy invite link" UI affordance (the create
	// endpoint still returns the AcceptURL); welcome/restore mails no-op cleanly.
	var mailer email.Mailer
	if cfg.EmailEnabled {
		smtpMailer, err := email.NewSMTPMailer(email.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.EmailFromAddress,
		})
		if err != nil {
			return fmt.Errorf("mailer: %w", err)
		}
		mailer = smtpMailer
	} else {
		slog.Info("EMAIL_ENABLED=false: outbound email disabled; invitations rely on the copy-link fallback")
		mailer = email.NewNoopMailer()
	}

	authH, err := auth.New(ctx, queries, auth.Config{
		Google: auth.GoogleConfig{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.OAuthRedirectURL,
			IssuerURL:    cfg.OIDCIssuerURL,
		},
		SessionTTL:   cfg.SessionTTL,
		CookieSecure: cfg.CookieSecure,
		FrontendURL:  cfg.FrontendURL,
		BackendURL:   cfg.BackendURL,
		EmailFrom:    cfg.EmailFromAddress,
		Mailer:       mailer,
	})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	assetRepo := repo.NewAssetRepo(pool)
	assetsH := assets.New(assetRepo)

	liabilityRepo := repo.NewLiabilityRepo(pool)
	liabilitiesH := liabilities.New(liabilityRepo)

	receivableRepo := repo.NewReceivableRepo(pool)
	receivablesH := receivables.New(receivableRepo)

	investmentRepo := repo.NewInvestmentRepo(pool)
	investmentsH := investments.New(investmentRepo)

	incomeRepo := repo.NewIncomeRepo(pool)
	incomeH := income.New(incomeRepo)

	reportRepo := repo.NewMonthlyReportRepo(pool)
	reportsH := reports.New(reportRepo)

	fxRateRepo := repo.NewFxRateRepo(pool)
	fxRatesH := fxrates.New(fxRateRepo)

	tagRepo := repo.NewTagRepo(pool)
	tagsH := tags.New(tagRepo)

	srv := httpserver.New(pool, cfg, authH, assetsH, liabilitiesH, receivablesH, investmentsH, incomeH, reportsH, fxRatesH, tagsH)

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("server starting", "addr", httpSrv.Addr)

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func applyMigrations(ctx context.Context, dsn string) error {
	dbConn, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = dbConn.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	return goose.UpContext(ctx, dbConn, ".")
}

func migrateCmd(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	slog.SetDefault(newLogger(cfg))

	if len(args) == 0 {
		args = []string{"status"}
	}

	dbConn, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = dbConn.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	cmd := args[0]
	var rest []string
	if len(args) > 1 {
		rest = args[1:]
	}

	return goose.RunContext(context.Background(), cmd, dbConn, ".", rest...)
}

// e2eDatabaseName is the only database seed-e2e will touch. The seed
// truncates every application table, so a guard against running it anywhere
// else (notably the dev database) is load-bearing, not cosmetic. See ADR-0024.
const e2eDatabaseName = "balances_e2e"

// e2eSessionID is the fixed session cookie value Playwright injects via
// context.addCookies({name:'session', value:e2eSessionID}). It is deterministic
// rather than a random opaque token because it only ever exists in the
// balances_e2e database — never production — so global-setup can rely on a
// constant instead of parsing it back out. We still print it (see below) to
// honour the ADR-0024 contract.
const e2eSessionID = "e2e-session-alice"

// Alice's fixture identity is shared between seed-e2e (which inserts the user
// with this google_sub) and mock-oidc (which issues an id_token carrying it).
// They MUST agree: the login-flow E2E test signs in via mock-oidc and expects
// to land as the *seeded* Alice, which only happens if GetUserByGoogleSub finds
// a row with this exact sub.
const (
	e2eAliceGoogleSub = "e2e-alice"
	e2eAliceEmail     = "alice@example.com"
	e2eAliceName      = "Alice"
	// e2eAlicePictureURL points at the 1x1 PNG mock-oidc serves at /avatar.png
	// (see mockoidc.go). Mock-oidc includes this as the `picture` claim in the
	// id_token, so the real OAuth callback runs SetUserPicture against Alice and
	// the UserAvatar component can load a real image instead of falling back to
	// initials. Local URL so the browser can actually fetch it offline.
	e2eAlicePictureURL = "http://localhost:8090/avatar.png"
)

// seedE2ECmd resets the dedicated balances_e2e database to a known fixture:
// one household with two users (Alice + Bob) and an active session for Alice.
// It bypasses Google OAuth entirely — the resulting session is a real session
// row that SessionMiddleware will accept, so Playwright authenticates by
// injecting the cookie instead of driving the IdP. See ADR-0024.
func seedE2ECmd() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx := context.Background()

	// Apply migrations first so an empty balances_e2e self-populates,
	// mirroring serve's auto-migrate behaviour. goose up is idempotent.
	if err := applyMigrations(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	if name := pool.Config().ConnConfig.Database; name != e2eDatabaseName {
		return fmt.Errorf("refusing to seed: DATABASE_URL points at %q, expected %q", name, e2eDatabaseName)
	}

	if err := truncateAppTables(ctx, pool); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	q := db.New(pool)

	household, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       "E2E Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		return fmt.Errorf("create household: %w", err)
	}

	alice, err := q.CreateUser(ctx, db.CreateUserParams{
		HouseholdID: household.ID,
		DisplayName: e2eAliceName,
		Email:       e2eAliceEmail,
		GoogleSub:   e2eAliceGoogleSub,
		// Pin to en-GB so Playwright specs that assert against the English UI
		// don't drift if the runner's navigator.language ever changes. Specs
		// that need to exercise the ID locale should switch via the Settings
		// UI within the test rather than mutating the seed.
		Locale:    "en-GB",
		TimeZone:  "Asia/Jakarta",
		CreatedBy: nil,
	})
	if err != nil {
		return fmt.Errorf("create user alice: %w", err)
	}

	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		HouseholdID: household.ID,
		DisplayName: "Bob",
		Email:       "bob@example.com",
		GoogleSub:   "e2e-bob",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   nil,
	}); err != nil {
		return fmt.Errorf("create user bob: %w", err)
	}

	userAgent := "e2e"
	if _, err := q.CreateSession(ctx, db.CreateSessionParams{
		ID:        e2eSessionID,
		UserID:    alice.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(cfg.SessionTTL), Valid: true},
		UserAgent: &userAgent,
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	fmt.Fprintf(os.Stderr, "seeded %s: household=%s, users Alice (%s) + Bob, active session for Alice\n",
		e2eDatabaseName, household.ID, alice.ID)
	// Sole stdout line, machine-readable for Playwright global-setup.
	fmt.Printf("SESSION_ID=%s\n", e2eSessionID)
	return nil
}

// truncateAppTables empties every public table except goose's bookkeeping,
// resetting identity sequences. The table list comes from the catalog so new
// migrations are covered automatically. Mirrors testutil.truncateAll, kept
// separate because that helper is test-only (takes *testing.T).
func truncateAppTables(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, `"`+name+`"`)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}
	if len(tables) == 0 {
		return nil
	}

	stmt := "TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
