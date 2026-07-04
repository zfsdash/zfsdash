package web

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes adds all API routes to the chi router.
// Called from handler.go after Handler is constructed.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Setup / wizard
	r.Get("/api/setup/state", h.handleSetupState)
	r.Post("/api/setup/admin", h.handleSetupAdmin)
	r.Post("/api/setup/host", h.handleSetupHost)

	// Pools
	r.Get("/api/pools", h.handlePools)
	r.Get("/api/pools/{pool}/health", h.handlePoolHealth)
	r.Get("/api/pools/{pool}/iostat", h.handleIOStat)
	r.Get("/api/pools/{pool}/capacity-trend", h.handleCapacityTrend)
	r.Get("/api/pools/{pool}/scrub/stream", h.handleScrubStream)
	r.Get("/api/pools/{pool}/faulted", h.handleFaultedDevices)
	r.Post("/api/pools/{pool}/replace", h.handleDriveReplace)
	r.Get("/api/pools/{pool}/resilver", h.handleResilverStatus)

	// Datasets + Snapshots
	r.Get("/api/datasets", h.handleDatasets)
	r.Get("/api/snapshots", h.handleSnapshots)
	r.Post("/api/snapshots", h.handleCreateSnapshot)
	r.Delete("/api/snapshots/{name}", h.handleDeleteSnapshot)

	// ARC + metrics
	r.Get("/api/arc", h.handleARC)
	r.Get("/metrics", h.handleMetrics)

	// Replication
	r.Get("/api/replication/jobs", h.handleReplList)
	r.Post("/api/replication/estimate", h.handleReplEstimate)
	r.Post("/api/replication/start", h.handleReplStart)
	r.Get("/api/replication/{jobID}", h.handleReplStatus)
	r.Get("/api/replication/{jobID}/stream", h.handleReplStream)

	// Simulator
	r.Post("/api/simulator/rebalance", h.handleSimulatorRebalance)

	// Alerts
	r.Get("/api/alerts", h.handleAlerts)
	r.Post("/api/alerts", h.handleCreateAlert)
	r.Delete("/api/alerts/{id}", h.handleDeleteAlert)

	// Auth
	r.Post("/api/auth/login", h.handleLogin)
	r.Post("/api/auth/logout", h.handleLogout)
	r.Get("/api/auth/me", h.handleMe)

	// Users (admin only)
	r.Get("/api/users", h.handleListUsers)
	r.Post("/api/users", h.handleCreateUser)
	r.Delete("/api/users/{id}", h.handleDeleteUser)

	// Hosts
	r.Get("/api/hosts", h.handleListHosts)
	r.Post("/api/hosts", h.handleAddHost)
	r.Delete("/api/hosts/{id}", h.handleDeleteHost)

	// Health check
	r.Get("/api/health", h.handleHealth)
	r.Get("/api/setup/state", h.handleSetupState)
}
