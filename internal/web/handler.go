// Package web provides the HTTP handler layer for ZFSdash.
package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/zfsdash/zfsdash/internal/alerts"
	"github.com/zfsdash/zfsdash/internal/auth"
	"github.com/zfsdash/zfsdash/internal/config"
	"github.com/zfsdash/zfsdash/internal/db"
	"github.com/zfsdash/zfsdash/internal/store"
	"github.com/zfsdash/zfsdash/internal/wizard"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

const (
	sessionCookieName = "zfsdash_session"
	sessionMaxAge     = 7 * 24 * 60 * 60
)

type contextKey int

const ctxKeyUser contextKey = iota

// Handler holds all dependencies for HTTP handlers.
type Handler struct {
	dbStore     *db.Store
	authSvc     *auth.Service
	alertEngine *alerts.Engine
	memStore    *store.Store
	collectors  map[string]zfs.Collector
	cfg         *config.Config
	version     string
}

// RegisterRoutes mounts all ZFSdash routes onto r.
func RegisterRoutes(
	r chi.Router,
	dbStore *db.Store,
	authSvc *auth.Service,
	cfg *config.Config,
	staticFS fs.FS,
) {
	h := &Handler{
		dbStore: dbStore,
		authSvc: authSvc,
		cfg:     cfg,
		version: "dev",
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(h.setupGuard)

	// Public routes
	r.Route("/api/setup", func(r chi.Router) {
		r.Get("/state", h.handleSetupState)
		r.Post("/init", h.handleSetupInit)
	})

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", h.handleLogin)
		r.Post("/logout", h.handleLogout)
		// MUST return 200 always — 401 triggers browser Basic Auth popup
		r.Get("/me", h.handleMe)
	})

	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		r.Handle("/setup", fileServer)
		r.Handle("/setup/*", fileServer)
		r.Handle("/*", fileServer)
	}

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(h.requireAuth)

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
	})
}

// ── Middleware ────────────────────────────────────────────────────────────────

func (h *Handler) setupGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/setup") ||
			strings.HasPrefix(path, "/api/auth") ||
			path == "/setup" ||
			strings.HasPrefix(path, "/setup/") {
			next.ServeHTTP(w, r)
			return
		}
		if h.dbStore != nil && !wizard.IsSetupComplete(h.dbStore.DB()) {
			if strings.HasPrefix(path, "/api/") {
				jsonError(w, "setup required", http.StatusServiceUnavailable)
				return
			}
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth validates the session cookie. Returns 403 (not 401) to avoid
// the browser Basic Auth credential popup.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := readSessionCookie(r)
		if token == "" {
			jsonError(w, "authentication required", http.StatusForbidden)
			return
		}
		_, user, err := h.authSvc.ValidateSession(token)
		if err != nil || user == nil {
			slog.Warn("session validation error", "err", err)
			jsonError(w, "authentication required", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── Setup handlers ────────────────────────────────────────────────────────────

func (h *Handler) handleSetupState(w http.ResponseWriter, r *http.Request) {
	state := wizard.GetSetupState(h.dbStore.DB(), h.version)
	jsonOK(w, state)
}

func (h *Handler) handleSetupInit(w http.ResponseWriter, r *http.Request) {
	firstRun, err := h.dbStore.IsFirstRun()
	if err != nil {
		jsonError(w, "database error", http.StatusInternalServerError)
		return
	}
	if !firstRun {
		jsonError(w, "setup already complete", http.StatusConflict)
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if _, err := h.authSvc.CreateAdminUser(req.Username, req.Email, req.Password); err != nil {
		slog.Error("create admin user", "err", err)
		jsonError(w, "failed to create admin user", http.StatusInternalServerError)
		return
	}

	// Auto-login after setup
	token, _, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		slog.Error("auto-login after setup", "err", err)
		jsonError(w, "account created but auto-login failed", http.StatusInternalServerError)
		return
	}

	writeSessionCookie(w, token)
	jsonOK(w, map[string]interface{}{
		"message": "Setup complete. Welcome to ZFSdash.",
		"token":   token,
	})
}

// ── Auth handlers ─────────────────────────────────────────────────────────────

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token, _, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		// 403 not 401 — 401 triggers browser Basic Auth popup
		jsonError(w, "invalid username or password", http.StatusForbidden)
		return
	}
	writeSessionCookie(w, token)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := readSessionCookie(r)
	if token != "" {
		if err := h.authSvc.Logout(token); err != nil {
			slog.Warn("logout error", "err", err)
		}
	}
	clearSessionCookie(w)
	jsonOK(w, map[string]string{"status": "ok"})
}

// handleMe returns current user or {"user": null}. ALWAYS returns HTTP 200.
func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	token := readSessionCookie(r)
	if token == "" {
		jsonOK(w, map[string]interface{}{"user": nil})
		return
	}
	_, user, err := h.authSvc.ValidateSession(token)
	if err != nil || user == nil {
		jsonOK(w, map[string]interface{}{"user": nil})
		return
	}
	jsonOK(w, map[string]interface{}{"user": safeUser(user)})
}

// ── ZFS handlers ──────────────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]string{"status": "ok", "version": h.version})
}

func (h *Handler) handleListHosts(w http.ResponseWriter, r *http.Request) {
	if h.memStore == nil {
		jsonOK(w, []interface{}{})
		return
	}
	hosts := h.memStore.GetAllHosts()
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
	if h.collectors == nil {
		jsonOK(w, []interface{}{})
		return
	}
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
	for _, p := range pools {
		if scrub, err := col.CollectScrubStatus(r.Context(), p.Name); err == nil {
			p.Scrub = scrub
		}
		if vdev, err := col.CollectVdevTree(r.Context(), p.Name); err == nil {
			p.VdevTree = vdev
		}
	}
	if h.memStore != nil {
		h.memStore.SetHostData(hostName, &store.HostData{
			Pools:     pools,
			Datasets:  make(map[string][]*zfs.Dataset),
			Snapshots: make(map[string][]*zfs.Snapshot),
		})
	}
	if h.alertEngine != nil {
		h.alertEngine.CheckPools(hostName, pools)
	}
	jsonOK(w, pools)
}

func (h *Handler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	if h.collectors == nil {
		jsonOK(w, []interface{}{})
		return
	}
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
	if h.collectors == nil {
		jsonOK(w, []interface{}{})
		return
	}
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
	if h.collectors == nil {
		jsonError(w, "no collectors configured", http.StatusServiceUnavailable)
		return
	}
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
	if h.collectors == nil {
		jsonError(w, "no collectors configured", http.StatusServiceUnavailable)
		return
	}
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
	if h.collectors == nil {
		jsonError(w, "no collectors configured", http.StatusServiceUnavailable)
		return
	}
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
	if h.dbStore == nil {
		jsonOK(w, []interface{}{})
		return
	}
	history, err := h.dbStore.GetPoolHistory(hostName, poolName, 168)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if history == nil {
		history = []db.PoolSnapshot{}
	}
	jsonOK(w, history)
}

func (h *Handler) handleScrubHistory(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "host")
	poolName := chi.URLParam(r, "pool")
	if h.dbStore == nil {
		jsonOK(w, []interface{}{})
		return
	}
	history, err := h.dbStore.GetScrubHistory(hostName, poolName, 20)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if history == nil {
		history = []db.ScrubRecord{}
	}
	jsonOK(w, history)
}

func (h *Handler) handleSMARTData(w http.ResponseWriter, r *http.Request) {
	if h.collectors == nil {
		jsonOK(w, []interface{}{})
		return
	}
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

// ── Cookie helpers ────────────────────────────────────────────────────────────

func readSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func writeSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ── Response helpers ──────────────────────────────────────────────────────────

func safeUser(u *db.User) map[string]interface{} {
	if u == nil {
		return nil
	}
	return map[string]interface{}{
		"id":         u.ID,
		"username":   u.Username,
		"email":      u.Email,
		"is_admin":   u.IsAdmin,
		"created_at": u.CreatedAt,
	}
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[web] encode response: %v", err)
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		log.Printf("[web] encode error response: %v", err)
	}
}
