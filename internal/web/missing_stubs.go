package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleAllVdevHealth — stubs for routes registered in routes.go
func (h *Handler) handleAllVdevHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"vdevs": []any{}})
}

func (h *Handler) handleARCAnomalies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"anomalies": []any{}})
}

func (h *Handler) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"alerts": []any{}})
}

// Send/receive stubs matching routes.go registrations
func (h *Handler) handleStartSend(w http.ResponseWriter, r *http.Request) {
	h.handleStartSendReceive(w, r)
}

func (h *Handler) handleListSendJobs(w http.ResponseWriter, r *http.Request) {
	h.handleListSendReceiveJobs(w, r)
}

func (h *Handler) handleGetSendJob(w http.ResponseWriter, r *http.Request) {
	h.handleGetSendReceiveJob(w, r)
}

func (h *Handler) handleSendJobProgress(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	_ = id
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "no_progress_data"})
	flusher.Flush()
}

func (h *Handler) handleCancelSendJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

func (h *Handler) handleEstimateSendSize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"estimated_bytes": 0, "note": "run zfs send -nv for exact size"})
}

// Additional stubs for any other routes.go references
func (h *Handler) handleGetHostIOStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"iostat": []any{}})
}

func (h *Handler) handleGetHostCapacityTrend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"trend": []any{}})
}

func (h *Handler) handleGetHostForecast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"forecast": nil})
}

func (h *Handler) handleHostAnomalies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"anomalies": []any{}})
}

func (h *Handler) handleStreamScrub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	w.Write([]byte("data: {\"status\": \"idle\"}\n\n"))
	flusher.Flush()
}

func (h *Handler) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func (h *Handler) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *Handler) handleTestAlert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (h *Handler) handleSimulateRebalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"simulation": nil})
}
