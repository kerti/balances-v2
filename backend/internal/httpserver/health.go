package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// appVersion is set via -ldflags -X at build time (Dockerfile's Go build
// stage), mirroring VITE_APP_VERSION on the SPA side (issue #75). "dev" for a
// local, non-ldflags build. Surfaced on /healthz so a deploy can assert the
// right version actually rolled out, not just that some health check passed
// (#355). Safe to bake at build time: identical across every environment a
// single build gets deployed to.
var appVersion = "dev"

type healthzResponse struct {
	OK        bool      `json:"ok"`
	DBTime    time.Time `json:"db_time"`
	Version   string    `json:"version"`
	DeployEnv string    `json:"deploy_env"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	var dbTime time.Time
	if err := s.pool.QueryRow(r.Context(), "SELECT now()").Scan(&dbTime); err != nil {
		http.Error(w, "db unreachable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// DEPLOY_ENV is read per-request (not baked at build time, unlike
	// appVersion above): it varies per deploy target for one build-once image
	// (#354), so it's set as a runtime env var (fly.toml --env / compose
	// environment) rather than a Docker build-arg.
	deployEnv := os.Getenv("DEPLOY_ENV")
	if deployEnv == "" {
		deployEnv = "local"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthzResponse{OK: true, DBTime: dbTime, Version: appVersion, DeployEnv: deployEnv})
}
