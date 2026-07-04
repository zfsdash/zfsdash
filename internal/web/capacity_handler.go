package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/capacity"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

func (h *Handler) handleCapacityPlan(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	if pool == "" {
		http.Error(w, "pool required", http.StatusBadRequest)
		return
	}
	analysis, err := capacity.AnalyzePool(r.Context(), pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}

func (h *Handler) handleSnapshotForensics(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	if pool == "" {
		http.Error(w, "pool required", http.StatusBadRequest)
		return
	}
	forensics, err := zfs.GetSnapshotForensics(r.Context(), pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(forensics)
}
