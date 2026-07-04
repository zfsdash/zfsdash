package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/dlq"
	"github.com/zfsdash/zfsdash/internal/zvol"
)

var dlqManager *dlq.Manager

func initDLQ() {
	if dlqManager == nil {
		dlqManager = dlq.NewManager(nil)
	}
}

// handleDLQEvents returns all detected silent failure events
func (h *Handler) handleDLQEvents(w http.ResponseWriter, r *http.Request) {
	initDLQ()
	events := dlqManager.GetEvents()
	if events == nil {
		events = []*dlq.Event{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// handleDLQAcknowledge acknowledges a DLQ event
func (h *Handler) handleDLQAcknowledge(w http.ResponseWriter, r *http.Request) {
	initDLQ()
	id := chi.URLParam(r, "id")
	if dlqManager.Acknowledge(id) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
	} else {
		http.Error(w, "event not found", http.StatusNotFound)
	}
}

// handleZvolHealth returns health report for all zvols in a pool
func (h *Handler) handleZvolHealth(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	if pool == "" {
		pool = r.URL.Query().Get("pool")
	}
	if pool == "" {
		http.Error(w, "pool parameter required", http.StatusBadRequest)
		return
	}

	zvols, err := zvol.ListZvols(pool)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pool":   pool,
			"zvols":  []interface{}{},
			"error":  err.Error(),
		})
		return
	}

	var reports []zvol.ZvolHealth
	for _, z := range zvols {
		reports = append(reports, zvol.CheckHealth(z))
	}
	if reports == nil {
		reports = []zvol.ZvolHealth{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pool":   pool,
		"zvols":  reports,
		"count":  len(reports),
	})
}

// handleZvolCapacityRisk returns over-provisioning risk for a pool
func (h *Handler) handleZvolCapacityRisk(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	if pool == "" {
		pool = r.URL.Query().Get("pool")
	}
	if pool == "" {
		http.Error(w, "pool parameter required", http.StatusBadRequest)
		return
	}

	risk, err := zvol.CheckPoolCapacityRisk(pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if risk == nil {
		http.Error(w, "pool not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(risk)
}
