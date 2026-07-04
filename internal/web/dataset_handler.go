package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// handleListDatasets handles GET /api/hosts/{host}/pools/{pool}/datasets
func (h *Handler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")

	if host == "" || pool == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "host and pool required"})
		return
	}

	datasets, err := zfs.ParseDatasets(pool)
	if err != nil {
		slog.Error("list datasets", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host":     host,
		"pool":     pool,
		"datasets": datasets,
	})
}

// handleListSnapshots handles GET /api/hosts/{host}/pools/{pool}/datasets/{dataset}/snapshots
func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	dataset := chi.URLParam(r, "dataset")

	fullDataset := pool + "/" + dataset
	snaps, err := zfs.ParseSnapshots(fullDataset)
	if err != nil {
		slog.Error("list snapshots", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host":      host,
		"pool":      pool,
		"dataset":   dataset,
		"snapshots": snaps,
	})
}

// handleCreateSnapshot handles POST /api/hosts/{host}/pools/{pool}/datasets/{dataset}/snapshot
func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	dataset := chi.URLParam(r, "dataset")

	var req struct {
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SnapshotName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "snapshot_name required"})
		return
	}

	if strings.ContainsAny(req.SnapshotName, "/ \t") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid snapshot_name"})
		return
	}

	fullPath := fmt.Sprintf("%s/%s@%s", pool, dataset, req.SnapshotName)
	cmd := exec.Command("zfs", "snapshot", fullPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Error("create snapshot", "path", fullPath, "err", err, "out", string(out))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": string(out)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host":     host,
		"pool":     pool,
		"dataset":  dataset,
		"snapshot": req.SnapshotName,
		"created":  true,
	})
}

// handleDestroySnapshot handles DELETE /api/hosts/{host}/pools/{pool}/datasets/{dataset}/snapshots/{snapshot}
func (h *Handler) handleDestroySnapshot(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	dataset := chi.URLParam(r, "dataset")
	snapshotName := chi.URLParam(r, "snapshot")

	if snapshotName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "snapshot name required"})
		return
	}

	fullPath := fmt.Sprintf("%s/%s@%s", pool, dataset, snapshotName)
	cmd := exec.Command("zfs", "destroy", fullPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Error("destroy snapshot", "path", fullPath, "err", err, "out", string(out))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": string(out)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host":     host,
		"pool":     pool,
		"dataset":  dataset,
		"snapshot": snapshotName,
		"deleted":  true,
	})
}