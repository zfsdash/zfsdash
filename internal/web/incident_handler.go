package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/incident"
)

var globalCorrelator = incident.New()

func (h *Handler) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	incidents := globalCorrelator.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"incidents": incidents})
}

func (h *Handler) handleResolveIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !globalCorrelator.Resolve(id) {
		http.Error(w, "incident not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

func (h *Handler) handleIngestEvent(w http.ResponseWriter, r *http.Request) {
	var ev incident.ZFSEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	globalCorrelator.Ingest(ev)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ingested"})
}
