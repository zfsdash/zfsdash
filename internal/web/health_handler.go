package web

import (
	"encoding/json"
	"net/http"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

func (s *Handler) handlePoolHealth(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}

	health, err := zfs.ParsePoolHealth(poolName)
	if err != nil {
		http.Error(w, `{"error":"failed to parse pool health"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
