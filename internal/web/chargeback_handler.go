package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zfsdash/zfsdash/internal/chargeback"
)

var cbManager = chargeback.NewManager()

func (h *Handler) handleChargebackCollect(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	if pool == "" {
		pool = "tank"
	}
	usages, err := cbManager.Collect(r.Context(), pool)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"collected": len(usages),
		"pool":      pool,
		"usages":    usages,
	})
}

func (h *Handler) handleChargebackReport(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	if pool == "" {
		pool = "tank"
	}
	end := time.Now()
	start := end.AddDate(0, 0, -30)
	report := cbManager.GenerateReport(pool, start, end)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (h *Handler) handleChargebackRateCard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var rc chargeback.RateCard
		if err := json.NewDecoder(r.Body).Decode(&rc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cbManager.SetRateCard(rc)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cbManager.GetRateCard())
}
