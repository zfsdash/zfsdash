package web

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/zfsdash/zfsdash/internal/db"
	"github.com/zfsdash/zfsdash/internal/store"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

//go:embed static
var Static embed.FS

// CollectorMap maps host names to their collectors.
type CollectorMap map[string]zfs.Collector

// New creates the HTTP API router.
func New(st *store.Store, database *db.DB, collectors CollectorMap) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	h := &handler{st: st, db: database, cols: collectors}

	r.Get("/api/health", h.health)
	r.Get("/api/hosts", h.listHosts)
	r.Get("/api/hosts/{host}/pools", h.listPools)
	r.Get("/api/hosts/{host}/pools/{pool}/history", h.poolHistory)
	r.Get("/api/hosts/{host}/datasets", h.listDatasets)
	r.Get("/api/hosts/{host}/snapshots", h.listSnapshots)
	r.Post("/api/hosts/{host}/snapshots", h.createSnapshot)
	r.Delete("/api/hosts/{host}/snapshots/{snapshot}", h.deleteSnapshot)
	r.Get("/api/hosts/{host}/smart", h.listSMART)
	r.Post("/api/hosts/{host}/pools/{pool}/scrub", h.startScrub)
	r.Delete("/api/hosts/{host}/pools/{pool}/scrub", h.stopScrub)

	return r
}

type handler struct {
	st   *store.Store
	db   *db.DB
	cols CollectorMap
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

func (h *handler) listHosts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.st.ListHosts())
}

func (h *handler) listPools(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	e := h.st.GetHost(host)
	if e == nil {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, 200, e.Pools)
}

func (h *handler) poolHistory(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}
	history, err := h.db.GetPoolHistory(host, pool, limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, history)
}

func (h *handler) listDatasets(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	e := h.st.GetHost(host)
	if e == nil {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, 200, e.Datasets)
}

func (h *handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	e := h.st.GetHost(host)
	if e == nil {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, 200, e.Snapshots)
}

func (h *handler) createSnapshot(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	col, ok := h.cols[host]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	var req struct {
		Dataset string `json:"dataset"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if req.Dataset == "" || req.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "dataset and name required"})
		return
	}
	if err := col.CreateSnapshot(r.Context(), req.Dataset, req.Name); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *handler) deleteSnapshot(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	snap := chi.URLParam(r, "snapshot")
	col, ok := h.cols[host]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	if err := col.DeleteSnapshot(r.Context(), snap); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *handler) listSMART(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	e := h.st.GetHost(host)
	if e == nil {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, 200, e.SMARTData)
}

func (h *handler) startScrub(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	col, ok := h.cols[host]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	if err := col.StartScrub(r.Context(), pool); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (h *handler) stopScrub(w http.ResponseWriter, r *http.Request) {
	host := chi.URLParam(r, "host")
	pool := chi.URLParam(r, "pool")
	col, ok := h.cols[host]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "host not found"})
		return
	}
	if err := col.StopScrub(r.Context(), pool); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}
