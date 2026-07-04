package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

func (h *Handler) handleVdevLatencyProfile(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	vdev := chi.URLParam(r, "vdev")
	if vdev == "" {
		vdev = "all"
	}
	profile, err := zfs.ProfileVdevLatency(pool, vdev, 30*time.Second, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

func (h *Handler) handleDatasetIOHeat(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	heats, err := zfs.GetDatasetIOHeat(pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"pool": pool, "datasets": heats, "count": len(heats)})
}

func (h *Handler) handleSnapshotCostAnalysis(w http.ResponseWriter, r *http.Request) {
	dataset := r.URL.Query().Get("dataset")
	if dataset == "" {
		http.Error(w, "dataset required", http.StatusBadRequest)
		return
	}
	analysis, err := zfs.AnalyzeSnapshotCost(dataset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}
