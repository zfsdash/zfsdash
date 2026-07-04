package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

func (h *Handler) handleListReplicationJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"jobs": []zfs.ReplicationJob{}})
}

func (h *Handler) handleCreateReplicationJob(w http.ResponseWriter, r *http.Request) {
	var job zfs.ReplicationJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if job.SourcePool == "" || job.SourceDataset == "" {
		http.Error(w, "source_pool and source_dataset required", http.StatusBadRequest)
		return
	}
	job.ID = time.Now().Format("20060102150405")
	job.CreatedAt = time.Now()
	job.Enabled = true
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleReplicationJobStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(zfs.ReplicationStatus{JobID: id, Running: false})
}

func (h *Handler) handleRunReplicationJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "job_id": id})
}

func (h *Handler) handleGetPoolVDevs(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tree, err := zfs.GetPoolVDevTree(ctx, pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func (h *Handler) handleReplaceVDev(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	var req struct {
		OldDevice string `json:"old_device"`
		NewDevice string `json:"new_device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.OldDevice == "" || req.NewDevice == "" {
		http.Error(w, "old_device and new_device required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := zfs.ReplaceVDev(ctx, pool, req.OldDevice, req.NewDevice); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "replacing",
		"pool":    pool,
		"monitor": "/api/pools/" + pool + "/resilver/status",
	})
}

func (h *Handler) handleResiverStatus(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	status, err := zfs.GetResiverStatus(ctx, pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
