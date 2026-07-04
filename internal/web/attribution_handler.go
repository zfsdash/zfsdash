package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/attribution"
)

var globalAttributor = attribution.New()

func (h *Handler) handleAttributionVdev(w http.ResponseWriter, r *http.Request) {
	vdev := chi.URLParam(r, "vdev")
	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	samples, err := attribution.SampleDatasets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pool := r.URL.Query().Get("pool")
	var vdevSamples []attribution.VdevSample
	if pool != "" {
		vdevSamples = append(vdevSamples, attribution.VdevSample{VdevID: vdev, Pool: pool})
	}
	for _, s := range samples {
		globalAttributor.Ingest(s, vdevSamples)
	}
	top := globalAttributor.TopWorkloads(vdev, n)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vdev":          vdev,
		"top_workloads": top,
	})
}

func (h *Handler) handleAttributionPool(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	samples, _ := attribution.SampleDatasets()
	snap := globalAttributor.Snapshot()
	_ = samples
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pool":      pool,
		"timestamp": snap.Timestamp,
		"profiles":  snap.Profiles,
	})
}

func (h *Handler) handleAttributionSnapshot(w http.ResponseWriter, r *http.Request) {
	snap := globalAttributor.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}
