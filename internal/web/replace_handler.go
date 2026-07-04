package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// GET /api/pools/{pool}/faulted
func (h *Handler) handleFaultedDevices(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	devices, err := zfs.ListFaultedDevices(pool)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pool":    pool,
		"faulted": devices,
		"count":   len(devices),
	})
}

// POST /api/pools/{pool}/replace
func (h *Handler) handleDriveReplace(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	var req struct {
		OldDev string `json:"old_dev"`
		NewDev string `json:"new_dev"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if req.OldDev == "" {
		http.Error(w, "old_dev required", 400)
		return
	}
	if err := zfs.StartDriveReplace(pool, req.OldDev, req.NewDev); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "resilvering",
		"pool":    pool,
		"old_dev": req.OldDev,
		"new_dev": req.NewDev,
	})
}

// GET /api/pools/{pool}/resilver
func (h *Handler) handleResilverStatus(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	active, detail, err := zfs.Resilvering(pool)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pool":       pool,
		"resilvering": active,
		"detail":     detail,
	})
}
