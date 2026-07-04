package web

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func SetupRoutes(r chi.Router, h *Handler, staticFS fs.FS) {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes
	r.Get("/api/setup/state", h.handleSetupState)
	r.Post("/api/setup/admin", h.handleSetupAdmin)
	r.Post("/api/setup/host", h.handleSetupHost)
	r.Post("/api/auth/login", h.handleLogin)
	r.Get("/api/health", h.handleHealth)
	r.Get("/metrics", h.handleMetrics)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(h.requireAuth)

		// Pools
		r.Get("/api/pools", h.handleListPools)
		r.Get("/api/pools/{pool}/datasets", h.handleListDatasets)
		r.Get("/api/pools/{pool}/snapshots", h.handleListSnapshots)
		r.Post("/api/pools/{pool}/snapshots", h.handleCreateSnapshot)
		r.Delete("/api/pools/{pool}/snapshots/{snapshot}", h.handleDeleteSnapshot)
		r.Post("/api/pools/{pool}/scrub", h.handleStartScrub)
		r.Get("/api/pools/{pool}/scrub/stream", h.handleScrubStream)
		r.Get("/api/pools/{pool}/health", h.handlePoolHealth)
		r.Get("/api/pools/{pool}/forecast", h.handleForecast)
		r.Get("/api/pools/{pool}/iostat", h.handleIOStat)
		r.Get("/api/pools/{pool}/capacity-trend", h.handleCapacityTrend)
		r.Get("/api/pools/{pool}/vdev/{vdev}/health", h.handleVdevHealth)
		r.Get("/api/pools/{pool}/checksum-audit", h.handleChecksumAudit)

		// Vdevs
		r.Get("/api/vdev/health", h.handleAllVdevHealth)
		r.Post("/api/vdev/collect", h.handleVdevCollect)
		r.Post("/api/pools/{pool}/replace", h.handleDriveReplace)
		r.Get("/api/pools/{pool}/resilver/status", h.handleResilverStatus)

		// ARC
		r.Get("/api/arc", h.handleARC)
		r.Get("/api/arc/anomalies", h.handleARCAnomalies)

		// Alerts
		r.Get("/api/alerts", h.handleListAlerts)
		r.Post("/api/alerts", h.handleCreateAlert)
		r.Delete("/api/alerts/{id}", h.handleDeleteAlert)

		// Users (admin only)
		r.Get("/api/users", h.handleListUsers)
		r.Post("/api/users", h.handleCreateUser)
		r.Delete("/api/users/{id}", h.handleDeleteUser)

		// Hosts
		r.Get("/api/hosts", h.handleListHosts)
		r.Post("/api/hosts", h.handleAddHost)
		r.Delete("/api/hosts/{id}", h.handleDeleteHost)

		// Send/Receive
		r.Post("/api/replication/send", h.handleStartSend)
		r.Get("/api/replication/jobs", h.handleListSendJobs)
		r.Get("/api/replication/jobs/{id}", h.handleGetSendJob)
		r.Get("/api/replication/jobs/{id}/progress", h.handleSendJobProgress)
		r.Post("/api/replication/jobs/{id}/cancel", h.handleCancelSendJob)
		r.Get("/api/replication/estimate", h.handleEstimateSendSize)
	})

	// Static files
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	// DLQ — Silent failure detection
	r.Get("/api/dlq/events", h.handleDLQEvents)
	r.Post("/api/dlq/events/{id}/acknowledge", h.handleDLQAcknowledge)

	// Zvol health
	r.Get("/api/pools/{pool}/zvols", h.handleZvolHealth)
	r.Get("/api/pools/{pool}/capacity-risk", h.handleZvolCapacityRisk)
}
