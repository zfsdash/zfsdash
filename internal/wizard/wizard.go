// Package wizard provides setup-state detection for the ZFSdash first-run wizard.
// It answers the question "has the application been configured yet?" so the HTTP
// layer can redirect new installations to the /setup flow.
package wizard

import (
	"database/sql"
)

// SetupState describes the current configuration state of the application.
type SetupState struct {
	// HasAdmin is true when at least one user exists in the database.
	HasAdmin bool `json:"has_admin"`

	// HasHosts is true when at least one host (local, SSH, or TrueNAS) has been
	// configured.
	HasHosts bool `json:"has_hosts"`

	// Version is the application version string, injected at startup.
	Version string `json:"version"`
}

// IsSetupComplete returns false when the users table is empty, meaning the
// first-run admin account has not yet been created.
//
// A nil or query error is treated as "not complete" so the wizard is shown
// rather than leaving the installation in an undefined state.
func IsSetupComplete(db *sql.DB) bool {
	if db == nil {
		return false
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// GetSetupState returns a full SetupState for the /api/setup/state endpoint.
// If any query fails the corresponding field defaults to false.
func GetSetupState(db *sql.DB, version string) SetupState {
	state := SetupState{Version: version}
	if db == nil {
		return state
	}

	// Admin check.
	var userCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err == nil {
		state.HasAdmin = userCount > 0
	}

	// Hosts check — only count active hosts.
	var hostCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE is_active = 1`).Scan(&hostCount); err == nil {
		state.HasHosts = hostCount > 0
	}

	return state
}
