package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ── Setup / Wizard stubs ──────────────────────────────────────────────────────

func (h *Handler) handleSetupAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "admin created"})
}

func (h *Handler) handleSetupHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ── Pool stubs ────────────────────────────────────────────────────────────────

func (h *Handler) handlePools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"pools": []interface{}{}})
}

func (h *Handler) handleIOStat(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"pool": pool, "status": "no_data"})
}

func (h *Handler) handleCapacityTrend(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"pool": pool, "readings": []interface{}{}})
}

func (h *Handler) handleScrubStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

// func (h *Handler) handleFaultedDevices(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{"faulted": []interface{}{}})
// }

// func (h *Handler) handleDriveReplace(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]string{"status": "initiated"})
// }

// func (h *Handler) handleResilverStatus(w http.ResponseWriter, r *http.Request) {
// 	pool := chi.URLParam(r, "pool")
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{"pool": pool, "resilvering": false, "progress": 0})
// }

// ── Dataset / Snapshot stubs ──────────────────────────────────────────────────

func (h *Handler) handleDatasets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"datasets": []interface{}{}})
}

// func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{"snapshots": []interface{}{}})
// }

// ── ARC / Metrics stubs ───────────────────────────────────────────────────────

func (h *Handler) handleARC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hit_ratio": 0.0, "size_bytes": 0, "max_size_bytes": 0,
	})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte("# ZFSdash metrics\n# no pools configured\n"))
}

// ── Replication stubs ─────────────────────────────────────────────────────────

func (h *Handler) handleReplList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"jobs": []interface{}{}})
}

func (h *Handler) handleReplEstimate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"estimated_bytes": 0, "estimated_duration_seconds": 0})
}

func (h *Handler) handleReplStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *Handler) handleReplStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "jobID")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"job_id": id, "running": false})
}

func (h *Handler) handleReplStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
}

// ── Simulator stubs ───────────────────────────────────────────────────────────

func (h *Handler) handleSimulatorRebalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"estimated_duration_hours": 0,
		"data_movement_bytes": 0,
	})
}

// ── Alert stubs ───────────────────────────────────────────────────────────────

// func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{"alerts": []interface{}{}})
// }

// func (h *Handler) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusCreated)
// 	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
// }

// func (h *Handler) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
// 	w.WriteHeader(http.StatusNoContent)
// }

// ── User management stubs ─────────────────────────────────────────────────────

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": []interface{}{}})
}

func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func (h *Handler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// ── Host management stubs ─────────────────────────────────────────────────────

func (h *Handler) handleAddHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

func (h *Handler) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}


func (h *Handler) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write([]byte(`{"status":"created"}`))
}

func (h *Handler) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(204)
}
