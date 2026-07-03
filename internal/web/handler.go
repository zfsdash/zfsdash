package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/zfsdash/zfsdash/internal/alerts"
	"github.com/zfsdash/zfsdash/internal/db"
	"github.com/zfsdash/zfsdash/internal/store"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	store      *store.Store
	db         *db.DB
	alertEngine *alerts.Engine
	collectors  map[string]zfs.Collector
	apiKey     string
}

// New creates a new Handler
func New(s *store.Store, database *db.DB, ae *alerts.Engine, collectors map[string]zfs.Collector, apiKey string) *Handler {
	return &Handler{
		store:       s,
		db:          database,
		alertEngine: ae,
		collectors:  collectors,
		apiKey:      apiKey,
	}
}

// Router returns the chi router
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/api/health", h.handleHealth)
	r.Get("/api/hosts", h.handleListHosts)

	r.Route("/api/hosts/{host}", func(r chi.Router) {
		r.Get("/pools", h.handleListPools)
		r.Get("/pools/{pool}/datasets", h.handleListDatasets)
		r.Get("/pools/{pool}/snapshots", h.handleListSnapshots)
		r.Post("/pools/{pool}/snapshots", h.handleCreateSnapshot)
		r.Delete("/pools/{pool}/snapshots/{snapshot}", h.handleDeleteSnapshot)
		r.Post("/pools/{pool}/scrub", h.handleStartScrub)
		r.Get("/pools/{pool}/history", h.handlePoolHistory)
		r.Get("/pools/{pool}/scrub-history", h.handleScrubHistory)
		r.Get("/smart", h.handleSMARTData)
	})

	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]string{"status": "ok", "version": "0.1.0"})
}

func (h *Handler) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts := h.store.GetAllHosts()
	type hostSummary struct {
		Name        string    `json:"name"`
		PoolCount   int       `json:"pool_count"`
		LastUpdated time.Time `json:"last_updated"`
		Error       string    `json:"error,omitempty"`
	}
	result := make([]hostSummary, 0, len(hosts))
	for _, hd := range hosts {
		result = append(result, hostSummary{
			Name:        hd.Name,
			PoolCount:   len(hd.Pools),
			LastUpdated: hd.LastUpdated,
			Error:       hd.Error,
		})
	}
	jsonOK(w, result)
}

func (h *Handler) handleListPools(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	pools, err := col.CollectPools(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Enrich with scrub status
	for _, p := range pools {
		if scrub, err := col.CollectScrubStatus(r.Context(), p.Name); err == nil {
			p.Scrub = scrub
		}
		if vdev, err := col.CollectVdevTree(r.Context(), p.Name); err == nil {
			p.VdevTree = vdev
		}
	}
	// Update store and record history
	hd := &store.HostData{
		Pools:     pools,
		Datasets:  make(map[string][]*zfs.Dataset),
		Snapshots: make(map[string][]*zfs.Snapshot),
	}
	h.store.SetHostData(hostName, hd)
	if h.db != nil {
		for _, p := range pools {
			if err := h.db.RecordPoolSnapshot(hostName, p); err != nil {
				log.Printf("[db] record pool snapshot: %v", err)
			}
		}
	}
	if h.alertEngine != nil {
		h.alertEngine.CheckPools(hostName, pools)
	}
	jsonOK(w, pools)
}

func (h *Handler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	datasets, err := col.CollectDatasets(r.Context(), poolName)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, datasets)
}

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	snapshots, err := col.CollectSnapshots(r.Context(), poolName)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, snapshots)
}

func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	var req struct {
		Dataset string `json:"dataset"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Dataset == "" {
		req.Dataset = poolName
	}
	if req.Name == "" {
		req.Name = time.Now().Format("20060102-150405")
	}
	if err := col.CreateSnapshot(context.Background(), req.Dataset, req.Name); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"snapshot": req.Dataset + "@" + req.Name})
}

func (h *Handler) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	snapshotName := chi.URLParam(r, "snapshot")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	if err := col.DestroySnapshot(context.Background(), snapshotName); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleStartScrub(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	if err := col.StartScrub(context.Background(), poolName); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "scrub started", "pool": poolName})
}

func (h *Handler) handlePoolHistory(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	if h.db == nil {
		jsonOK(w, []interface{}{})
		return
	}
	history, err := h.db.GetPoolHistory(hostName, poolName, 168) // 7 days at 1hr intervals
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, history)
}

func (h *Handler) handleScrubHistory(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	if h.db == nil {
		jsonOK(w, []interface{}{})
		return
	}
	history, err := h.db.GetScrubHistory(hostName, poolName, 20)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, history)
}

func (h *Handler) handleSMARTData(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	col, ok := h.collectors[hostName]
	if !ok {
		jsonError(w, "host not found", http.StatusNotFound)
		return
	}
	smartData, err := col.CollectSMARTData(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, smartData)
}

// --- helpers ---

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
