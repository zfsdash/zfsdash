package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zfsdash/zfsdash/internal/vdev"
)

func (h *Handler) handleVdevHealth(w http.ResponseWriter, r *http.Request) {
	pool := chi.URLParam(r, "pool")
	vdevName := chi.URLParam(r, "vdev")
	window := 24 * time.Hour
	if wStr := r.URL.Query().Get("window"); wStr != "" {
		if d, err := time.ParseDuration(wStr); err == nil {
			window = d
		}
	}
	collector := vdev.NewHealthCollector(h.dbStore.DB(), slog.Default())
	metric, err := collector.Analyze(r.Context(), pool, vdevName, window)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metric)
}

func (h *Handler) handleVdevHealthAll(w http.ResponseWriter, r *http.Request) {
	collector := vdev.NewHealthCollector(h.dbStore.DB(), slog.Default())
	metrics, err := collector.AnalyzeAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if metrics == nil {
		metrics = []vdev.HealthMetric{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vdevs": metrics,
		"count": len(metrics),
	})
}

func (h *Handler) handleVdevCollect(w http.ResponseWriter, r *http.Request) {
	collector := vdev.NewHealthCollector(h.dbStore.DB(), slog.Default())
	if err := collector.CollectAll(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "collected"})
}
