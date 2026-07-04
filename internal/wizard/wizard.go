package wizard

import "database/sql"

// SetupState describes the current wizard progress.
type SetupState struct {
	Complete  bool   `json:"complete"`
	AdminSet  bool   `json:"admin_set"`
	HostCount int    `json:"host_count"`
	Version   string `json:"version"`
}

// IsSetupComplete returns true if at least one admin user exists.
func IsSetupComplete(db *sql.DB) bool {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// GetSetupState returns the current wizard progress.
func GetSetupState(db *sql.DB, version string) interface{} {
	state := &SetupState{Version: version}
	state.Complete = IsSetupComplete(db)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err == nil {
		state.AdminSet = count > 0
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE is_active = 1`).Scan(&count); err == nil {
		state.HostCount = count
	}
	return state
}

// CompleteSetup is a no-op — setup completion is derived from user count.
func CompleteSetup(db *sql.DB) error { return nil }
