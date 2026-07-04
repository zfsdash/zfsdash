package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// in-memory job store (sufficient for MVP; jobs are short-lived)
var (
	replJobs   = map[string]*zfs.ReplicationJob{}
	replJobsMu sync.RWMutex
)

func newJobID() string {
	return fmt.Sprintf("repl-%d", time.Now().UnixNano())
}

// POST /api/replication/estimate
func (h *Handler) handleReplEstimate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source    string `json:"source"`
		FromSnap  string `json:"from_snap"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	bytes, err := zfs.EstimateReplication(req.Source, req.FromSnap, req.Recursive)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"bytes": bytes,
		"human": humanBytes(bytes),
	})
}

// POST /api/replication/start
func (h *Handler) handleReplStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source      string `json:"source"`
		Target      string `json:"target"`
		RemoteHost  string `json:"remote_host"`
		RemoteUser  string `json:"remote_user"`
		Incremental bool   `json:"incremental"`
		FromSnap    string `json:"from_snap"`
		Recursive   bool   `json:"recursive"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if req.Source == "" || req.Target == "" {
		http.Error(w, "source and target required", 400)
		return
	}

	job := &zfs.ReplicationJob{
		ID:          newJobID(),
		Source:      req.Source,
		Target:      req.Target,
		RemoteHost:  req.RemoteHost,
		RemoteUser:  req.RemoteUser,
		Incremental: req.Incremental,
		FromSnap:    req.FromSnap,
		Recursive:   req.Recursive,
		DryRun:      req.DryRun,
		Status:      "pending",
		Log:         []string{},
	}

	replJobsMu.Lock()
	replJobs[job.ID] = job
	replJobsMu.Unlock()

	// Run async
	go func() {
		progCh := make(chan string, 64)
		ctx := context.Background()
		_ = zfs.RunReplication(ctx, job, progCh)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

// GET /api/replication/{jobID}
func (h *Handler) handleReplStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	replJobsMu.RLock()
	job, ok := replJobs[jobID]
	replJobsMu.RUnlock()
	if !ok {
		http.Error(w, "job not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// GET /api/replication/{jobID}/stream  — SSE progress
func (h *Handler) handleReplStream(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	replJobsMu.RLock()
	job, ok := replJobs[jobID]
	replJobsMu.RUnlock()
	if !ok {
		http.Error(w, "job not found", 404)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastLen int
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			replJobsMu.RLock()
			newLines := job.Log[lastLen:]
			status := job.Status
			replJobsMu.RUnlock()

			for _, line := range newLines {
				fmt.Fprintf(w, "data: %s\n\n", line)
				lastLen++
			}

			if status == "done" || status == "error" {
				fmt.Fprintf(w, "data: {\"status\":\"%s\"}\n\n", status)
				flusher.Flush()
				return
			}
			flusher.Flush()
		}
	}
}

// GET /api/replication/jobs
func (h *Handler) handleReplList(w http.ResponseWriter, r *http.Request) {
	replJobsMu.RLock()
	jobs := make([]*zfs.ReplicationJob, 0, len(replJobs))
	for _, j := range replJobs {
		jobs = append(jobs, j)
	}
	replJobsMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
