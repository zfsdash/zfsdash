package web

import (
	"encoding/json"
	"net/http"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// HealthHandler handles GET /api/pools/{name}/health
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}

	health, err := zfs.ParsePoolHealth(poolName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
