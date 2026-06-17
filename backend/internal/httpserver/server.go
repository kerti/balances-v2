package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/assets"
	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/backup"
	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/fxrates"
	"github.com/kerti/balances-v2/backend/internal/income"
	"github.com/kerti/balances-v2/backend/internal/investments"
	"github.com/kerti/balances-v2/backend/internal/liabilities"
	"github.com/kerti/balances-v2/backend/internal/receivables"
	"github.com/kerti/balances-v2/backend/internal/reports"
	"github.com/kerti/balances-v2/backend/internal/tags"
)

type Server struct {
	pool         *pgxpool.Pool
	cfg          *config.Config
	authH        *auth.Handlers
	assetsH      *assets.Handlers
	liabilitiesH *liabilities.Handlers
	receivablesH *receivables.Handlers
	investmentsH *investments.Handlers
	incomeH      *income.Handlers
	reportsH     *reports.Handlers
	fxRatesH     *fxrates.Handlers
	tagsH        *tags.Handlers
	backupH      *backup.Handlers
	router       chi.Router
}

func New(
	pool *pgxpool.Pool,
	cfg *config.Config,
	authH *auth.Handlers,
	assetsH *assets.Handlers,
	liabilitiesH *liabilities.Handlers,
	receivablesH *receivables.Handlers,
	investmentsH *investments.Handlers,
	incomeH *income.Handlers,
	reportsH *reports.Handlers,
	fxRatesH *fxrates.Handlers,
	tagsH *tags.Handlers,
) *Server {
	s := &Server{
		pool:         pool,
		cfg:          cfg,
		authH:        authH,
		assetsH:      assetsH,
		liabilitiesH: liabilitiesH,
		receivablesH: receivablesH,
		investmentsH: investmentsH,
		incomeH:      incomeH,
		reportsH:     reportsH,
		fxRatesH:     fxRatesH,
		tagsH:        tagsH,
		// Backup reads across every table from the shared pool; it needs the pool,
		// the instance URL (stamped into the envelope), and the auth handler to
		// re-issue the caller's session after a restore wipes it and to send the
		// best-effort post-restore notifications (#176).
		backupH: backup.New(pool, cfg.BackendURL, authH, authH),
	}
	s.router = s.buildRouter()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// No middleware.RealIP: it trusts X-Forwarded-For / X-Real-IP, which any
	// client can spoof when no trusted proxy sits in front (our case — see
	// docker-compose.yml). chi deprecated it for this reason (GHSA-3fxj-6jh8-hvhx).
	// If we ever deploy behind a known proxy, add a trusted-CIDR-aware extractor.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.authH.SessionMiddleware)

	r.Get("/healthz", s.handleHealthz)

	r.Route("/api", func(r chi.Router) {
		s.authH.Mount(r)
		s.assetsH.Mount(r)
		s.liabilitiesH.Mount(r)
		s.receivablesH.Mount(r)
		s.investmentsH.Mount(r)
		s.incomeH.Mount(r)
		s.reportsH.Mount(r)
		s.fxRatesH.Mount(r)
		s.tagsH.Mount(r)
		s.backupH.Mount(r)
	})

	// Single-origin production: serve the built SPA from WEB_DIR alongside /api
	// (ADR-0030). /api and /healthz are matched above, so this catch-all only sees
	// frontend paths. Unset in dev (Vite serves the SPA and proxies /api here).
	if s.cfg.WebDir != "" {
		r.Handle("/*", spaHandler(s.cfg.WebDir))
	}

	return r
}

// spaHandler serves static files from dir, falling back to index.html for any
// path without a matching file so client-side routes (ADR-0025) resolve on a
// refresh or deep link. The within-dir prefix check guards against path
// traversal; misses fall through to index.html rather than leaking the error.
func spaHandler(dir string) http.HandlerFunc {
	root := filepath.Clean(dir)
	fileServer := http.FileServer(http.Dir(root))
	index := filepath.Join(root, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		full := filepath.Join(root, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(full); err == nil && !info.IsDir() &&
			strings.HasPrefix(full, root+string(os.PathSeparator)) {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	}
}
