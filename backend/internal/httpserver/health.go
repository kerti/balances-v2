package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

// appVersion is set via -ldflags -X at build time (Dockerfile's Go build
// stage), mirroring VITE_APP_VERSION on the SPA side (issue #75). "dev" for a
// local, non-ldflags build. Surfaced on /healthz so a deploy can assert the
// right version actually rolled out, not just that some health check passed
// (#355).
var appVersion = "dev"

type healthzResponse struct {
	OK      bool      `json:"ok"`
	DBTime  time.Time `json:"db_time"`
	Version string    `json:"version"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	var dbTime time.Time
	if err := s.pool.QueryRow(r.Context(), "SELECT now()").Scan(&dbTime); err != nil {
		http.Error(w, "db unreachable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthzResponse{OK: true, DBTime: dbTime, Version: appVersion})
}
