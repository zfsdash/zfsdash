package web

import (
	"encoding/json"
	"net/http"

	"github.com/zfsdash/zfsdash/internal/collector"
)

// HandleARCStats handles GET /api/arc
// Returns ARC cache statistics from /proc/spl/kstat/zfs/arcstats.
// Returns zeroed struct gracefully on non-Linux or ZFS not loaded.
func (s *Server) HandleARCStats(w http.ResponseWriter, r *http.Request) {
	stats := collector.ReadARCStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandlePoolIOStats handles GET /api/pools/{name}/iostat
// Returns real-time I/O stats for a named ZFS pool via zpool iostat.
func (s *Server) HandlePoolIOStats(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}
	stats := collector.ReadPoolIOStats(name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleCapacityTrend handles GET /api/pools/{name}/capacity-trend
// Returns 30-day capacity history and days-until-full estimate.
func (s *Server) HandleCapacityTrend(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}
	trend, err := collector.GetCapacityTrend(s.db, name)
	if err != nil {
		http.Error(w, `{"error":"failed to get trend"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trend)
}
