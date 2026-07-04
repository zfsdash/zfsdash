package wizard

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// State tracks setup wizard progress, persisted in SQLite.
type State struct {
	Complete  bool   `json:"complete"`
	Step      int    `json:"step"`
	AdminSet  bool   `json:"admin_set"`
	HostAdded bool   `json:"host_added"`
	Version   string `json:"version"`
}

// Wizard handles the first-run setup flow.
type Wizard struct {
	db      *sql.DB
	version string
}

func New(db *sql.DB, version string) *Wizard {
	return &Wizard{db: db, version: version}
}

// IsComplete returns true if setup has been completed.
func (w *Wizard) IsComplete() bool {
	var count int
	err := w.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// HandleStatus returns current wizard state.
func (w *Wizard) HandleStatus(rw http.ResponseWriter, r *http.Request) {
	complete := w.IsComplete()
	step := 1
	if complete {
		step = 3
	}

	var hostCount int
	_ = w.db.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&hostCount)
	if hostCount > 0 && complete {
		step = 3
	} else if complete {
		step = 2
	}

	state := State{
		Complete:  complete,
		Step:      step,
		AdminSet:  complete,
		HostAdded: hostCount > 0,
		Version:   w.version,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(state)
}

// HandleCreateAdmin creates the first admin user.
func (w *Wizard) HandleCreateAdmin(rw http.ResponseWriter, r *http.Request) {
	if w.IsComplete() {
		http.Error(rw, "setup already complete", http.StatusConflict)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Email == "" || len(req.Password) < 8 {
		http.Error(rw, "email required and password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	_, err = w.db.ExecContext(r.Context(),
		`INSERT INTO users (email, password_hash, role, created_at) VALUES (?, ?, 'admin', ?)`,
		req.Email, string(hash), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		slog.Error("create admin", "err", err)
		http.Error(rw, "failed to create admin user", http.StatusInternalServerError)
		return
	}

	slog.Info("admin user created", "email", req.Email)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "email": req.Email})
}

// HandleAddHost adds the first ZFS host.
func (w *Wizard) HandleAddHost(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Mode string `json:"mode"` // local, ssh, truenas
		Host string `json:"host"`
		User string `json:"user"`
		Key  string `json:"ssh_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = "localhost"
	}
	if req.Mode == "" {
		req.Mode = "local"
	}

	// Store SSH key to temp file if provided
	keyPath := ""
	if req.Key != "" {
		f, err := os.CreateTemp("", "zfsdash-key-*")
		if err == nil {
			f.WriteString(req.Key)
			f.Close()
			os.Chmod(f.Name(), 0600)
			keyPath = f.Name()
		}
	}

	_, err := w.db.ExecContext(r.Context(),
		`INSERT OR REPLACE INTO hosts (name, mode, host, user, key_path, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		req.Name, req.Mode, req.Host, req.User, keyPath, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		slog.Error("add host", "err", err)
		http.Error(rw, "failed to add host", http.StatusInternalServerError)
		return
	}

	slog.Info("host added", "name", req.Name, "mode", req.Mode)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "name": req.Name, "mode": req.Mode})
}
