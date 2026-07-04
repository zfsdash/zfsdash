package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

var (
	srMu   sync.RWMutex
	srJobs = map[string]*zfs.SendReceiveJob{}
)

func (h *Handler) handleListSendReceiveJobs(w http.ResponseWriter, r *http.Request) {
	srMu.RLock()
	jobs := make([]*zfs.SendReceiveJob, 0, len(srJobs))
	for _, j := range srJobs {
		jobs = append(jobs, j)
	}
	srMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
}

func (h *Handler) handleStartSendReceive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset     string `json:"dataset"`
		Snapshot    string `json:"snapshot"`
		Destination string `json:"destination"`
		Incremental string `json:"incremental"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	job := &zfs.SendReceiveJob{
		ID:          uuid.New().String(),
		Dataset:     req.Dataset,
		Snapshot:    req.Snapshot,
		Destination: req.Destination,
		Incremental: req.Incremental,
		Status:      "running",
		Started:     time.Now(),
	}

	srMu.Lock()
	srJobs[job.ID] = job
	srMu.Unlock()

	go func() {
		err := zfs.SendSnapshot(context.Background(), req.Dataset, req.Snapshot, req.Incremental, req.Destination, func(n int64) {
			srMu.Lock()
			job.BytesSent = n
			srMu.Unlock()
		})
		now := time.Now()
		srMu.Lock()
		job.Finished = &now
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		} else {
			job.Status = "complete"
		}
		srMu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleGetSendReceiveJob(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	srMu.RLock()
	job, ok := srJobs[id]
	srMu.RUnlock()
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
