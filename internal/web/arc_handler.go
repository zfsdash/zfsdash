package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zfsdash/zfsdash/internal/collector"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// HandleARCStats handles GET /api/arc
func (h *Handler) HandleARCStats(w http.ResponseWriter, r *http.Request) {
	stats := collector.ReadARCStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandlePoolIOStats handles GET /api/pools/{name}/iostat
func (h *Handler) HandlePoolIOStats(w http.ResponseWriter, r *http.Request) {
	poolName := pathSegment(r.URL.Path, "pools")
	if poolName == "" {
		http.Error(w, "pool name required", http.StatusBadRequest)
		return
	}
	stats, err := zfs.GetPoolIOStats(poolName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"pool": poolName, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleCapacityTrend handles GET /api/pools/{name}/capacity-trend
func (h *Handler) HandleCapacityTrend(w http.ResponseWriter, r *http.Request) {
	poolName := pathSegment(r.URL.Path, "pools")
	if poolName == "" {
		http.Error(w, "pool name required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"pool": poolName, "note": "capacity trend coming in v0.2"})
}

// pathSegment extracts the segment after a key in a URL path
func pathSegment(path, key string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == key && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
