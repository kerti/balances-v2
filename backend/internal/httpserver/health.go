package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kerti/balances-v2/backend/internal/version"
)

type healthzResponse struct {
	OK        bool      `json:"ok"`
	DBTime    time.Time `json:"db_time"`
	Version   string    `json:"version"`
	DeployEnv string    `json:"deploy_env"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	var dbTime time.Time
	if err := s.pool.QueryRow(r.Context(), "SELECT now()").Scan(&dbTime); err != nil {
		slog.Error("healthz db unreachable", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// DEPLOY_ENV is read per-request (not baked at build time, unlike
	// version.Version): it varies per deploy target for one build-once image
	// (#354), so it's set as a runtime env var (fly.toml --env / compose
	// environment) rather than a Docker build-arg.
	deployEnv := os.Getenv("DEPLOY_ENV")
	if deployEnv == "" {
		deployEnv = "local"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthzResponse{OK: true, DBTime: dbTime, Version: version.Version, DeployEnv: deployEnv})
}
