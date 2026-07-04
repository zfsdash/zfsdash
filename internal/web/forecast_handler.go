package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zfsdash/zfsdash/internal/collector"
	"github.com/zfsdash/zfsdash/internal/forecast"
	"github.com/zfsdash/zfsdash/internal/snapshots"
)

func (s *Handler) handleForecast(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}

	trend, err := collector.GetCapacityTrend(s.dbStore.DB(), poolName)
	if err != nil || trend == nil || len(trend.Readings) < 3 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pool":  poolName,
			"error": "insufficient data -- need at least 3 capacity readings",
			"ready": false,
		})
		return
	}

	points := make([]forecast.CapacityPoint, len(trend.Readings))
	for i, rr := range trend.Readings {
		points[i] = forecast.CapacityPoint{
			Timestamp:  rr.RecordedAt,
			UsedBytes:  int64(rr.Used),
			TotalBytes: int64(rr.Total),
		}
	}

	lr := forecast.FitLinear(points)
	if lr == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pool":  poolName,
			"error": "insufficient data variance for forecast",
			"ready": false,
		})
		return
	}

	now := time.Now()
	lastReading := trend.Readings[len(trend.Readings)-1]
	totalBytes := int64(lastReading.Total)

	weekly := lr.Project(now, 7*24*time.Hour, 12, totalBytes)
	days80 := lr.DaysUntilFull(now, totalBytes, 80)
	days95 := lr.DaysUntilFull(now, totalBytes, 95)

	type ForecastPoint struct {
		Date          string  `json:"date"`
		PredictedUsed int64   `json:"predicted_used"`
		LowerBound    int64   `json:"lower_bound"`
		UpperBound    int64   `json:"upper_bound"`
		Confidence    float64 `json:"confidence"`
		PctUsed       float64 `json:"pct_used"`
	}

	pts := make([]ForecastPoint, len(weekly))
	for i, f := range weekly {
		pct := 0.0
		if totalBytes > 0 {
			pct = float64(f.PredictedUsed) / float64(totalBytes) * 100
		}
		pts[i] = ForecastPoint{
			Date:          f.Timestamp.Format("2006-01-02"),
			PredictedUsed: f.PredictedUsed,
			LowerBound:    f.LowerBound,
			UpperBound:    f.UpperBound,
			Confidence:    f.Confidence,
			PctUsed:       pct,
		}
	}

	resp := map[string]interface{}{
		"pool":         poolName,
		"ready":        true,
		"r_squared":    lr.RSquared,
		"growth_rate":  lr.Slope * 86400,
		"data_points":  lr.DataPoints,
		"history_days": int(lr.TimeRange.Hours() / 24),
		"total_bytes":  totalBytes,
		"forecast":     pts,
	}
	if days80 != nil {
		resp["days_until_80pct"] = int(*days80)
	}
	if days95 != nil {
		resp["days_until_95pct"] = int(*days95)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}
	snaps, err := snapshots.GetRecentSnapshots(s.dbStore.DB(), poolName, 288)
	if err != nil {
		http.Error(w, `{"error":"failed to query snapshots"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pool": poolName, "snapshots": snaps, "count": len(snaps),
	})
}

func (s *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	poolName := r.PathValue("name")
	if poolName == "" {
		http.Error(w, `{"error":"pool name required"}`, http.StatusBadRequest)
		return
	}
	resolved := r.URL.Query().Get("resolved") == "true"
	alerts, err := snapshots.GetAlerts(s.dbStore.DB(), poolName, resolved, 50)
	if err != nil {
		http.Error(w, `{"error":"failed to query alerts"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pool": poolName, "alerts": alerts, "count": len(alerts),
	})
}
