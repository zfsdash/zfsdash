package wizard

import "database/sql"

// SetupState holds the current wizard state.
type SetupState struct {
	Complete  bool   `json:"complete"`
	AdminSet  bool   `json:"admin_set"`
	HostCount int    `json:"host_count"`
}

// IsSetupComplete returns true if the initial setup wizard has been completed.
func IsSetupComplete(db *sql.DB) bool {
	var val string
	err := db.QueryRow(`SELECT value FROM config WHERE key = 'setup_complete'`).Scan(&val)
	if err != nil {
		return false
	}
	return val == "1"
}

// GetSetupState returns the current setup progress.
func GetSetupState(db *sql.DB) interface{} {
	state := &SetupState{}
	state.Complete = IsSetupComplete(db)

	// Check if an admin user exists.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err == nil {
		state.AdminSet = count > 0
	}

	// Check how many hosts are configured.
	if err := db.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&count); err == nil {
		state.HostCount = count
	}

	return state
}

// CompleteSetup marks the wizard as done.
func CompleteSetup(db *sql.DB) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO config (key, value) VALUES ('setup_complete', '1')`)
	return err
}
