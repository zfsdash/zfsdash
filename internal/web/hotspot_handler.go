package web

import (
	"encoding/json"
	"net/http"

	"github.com/zfsdash/zfsdash/internal/hotspot"
)

var hsTracker = hotspot.NewTracker()

func (h *Handler) handleHotspotCollect(w http.ResponseWriter, r *http.Request) {
	events, err := hsTracker.Collect(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"new_events": len(events),
		"events":     events,
	})
}

func (h *Handler) handleHotspots(w http.ResponseWriter, r *http.Request) {
	hotspots := hsTracker.DetectHotspots()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hotspots": hotspots,
		"count":    len(hotspots),
	})
}

func (h *Handler) handleVdevRisk(w http.ResponseWriter, r *http.Request) {
	risks := hsTracker.PredictFailures()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vdev_risks": risks,
		"count":      len(risks),
	})
}
