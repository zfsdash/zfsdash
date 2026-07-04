package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/anomaly"
)

// anomalyMgr is the global anomaly manager.
var anomalyMgr *anomaly.Manager

func initAnomalyManager() {
	anomalyMgr = anomaly.NewManager()
}

// handleGetAnomalies returns all anomaly events across all pools.
// GET /api/anomalies
func (h *Handler) handleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	if anomalyMgr == nil {
		jsonResp(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}})
		return
	}
	events := anomalyMgr.AllEvents()
	jsonResp(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// handleGetPoolAnomalies returns anomaly events for a specific pool.
// GET /api/hosts/{host}/pools/{pool}/anomalies
func (h *Handler) handleGetPoolAnomalies(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	if anomalyMgr == nil {
		jsonResp(w, http.StatusOK, map[string]interface{}{"events": []interface{}{}})
		return
	}
	events := anomalyMgr.PoolEvents(pool)
	mean, stddev, samples := anomalyMgr.PoolStats(pool)
	jsonResp(w, http.StatusOK, map[string]interface{}{
		"pool":   pool,
		"events": events,
		"count":  len(events),
		"baseline": map[string]interface{}{
			"mean":    mean,
			"stddev":  stddev,
			"samples": samples,
		},
	})
}

// FeedAnomalyMetric feeds an ARC measurement into the anomaly detector.
func FeedAnomalyMetric(pool string, hitRatio float64, sizeBytes uint64) *anomaly.AnomalyEvent {
	if anomalyMgr == nil {
		return nil
	}
	return anomalyMgr.Feed(pool, hitRatio, sizeBytes)
}

func jsonResp(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
