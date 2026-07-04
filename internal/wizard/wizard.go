package wizard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Wizard struct {
	db *sql.DB
}

type StatusResponse struct {
	Complete bool   `json:"complete"`
	Step     string `json:"step"`
}

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type HostRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Mode     string `json:"mode"`   // local, ssh, truenas
	Username string `json:"username"`
	SSHKey   string `json:"ssh_key"`
	APIKey   string `json:"api_key"`
}

func New(db *sql.DB) *Wizard {
	return &Wizard{db: db}
}

func (w *Wizard) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		address TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT 'local',
		username TEXT,
		ssh_key TEXT,
		api_key TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS admin_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		email TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := w.db.Exec(schema)
	return err
}

func (w *Wizard) Status(rw http.ResponseWriter, r *http.Request) {
	if err := w.InitSchema(); err != nil {
		http.Error(rw, "Database error", http.StatusInternalServerError)
		return
	}

	var wizardComplete string
	err := w.db.QueryRow("SELECT value FROM settings WHERE key='wizard_complete'").Scan(&wizardComplete)
	if err == sql.ErrNoRows {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(StatusResponse{Complete: false, Step: "admin_setup"})
		return
	}
	if err != nil {
		http.Error(rw, "Database error", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(StatusResponse{Complete: wizardComplete == "true", Step: "complete"})
}

func (w *Wizard) Setup(rw http.ResponseWriter, r *http.Request) {
	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(rw, "Username and password required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(rw, "Password hashing failed", http.StatusInternalServerError)
		return
	}

	tx, err := w.db.Begin()
	if err != nil {
		http.Error(rw, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO admin_users (username, password_hash, email) VALUES (?, ?, ?)",
		req.Username, string(hash), req.Email)
	if err != nil {
		http.Error(rw, "Admin user creation failed (user may already exist)", http.StatusConflict)
		return
	}

	_, err = tx.Exec("INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES ('wizard_complete', 'true', ?)",
		time.Now())
	if err != nil {
		http.Error(rw, "Settings update failed", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(rw, "Transaction commit failed", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok", "message": "Admin account created"})
}

func (w *Wizard) AddHost(rw http.ResponseWriter, r *http.Request) {
	var req HostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Address == "" {
		http.Error(rw, "Name and address required", http.StatusBadRequest)
		return
	}
	if req.Mode == "" {
		req.Mode = "local"
	}

	_, err := w.db.Exec(
		"INSERT INTO hosts (name, address, mode, username, ssh_key, api_key) VALUES (?, ?, ?, ?, ?, ?)",
		req.Name, req.Address, req.Mode, req.Username, req.SSHKey, req.APIKey,
	)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Host creation failed: %v", err), http.StatusConflict)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok", "message": "Host added"})
}

func (w *Wizard) ListHosts(rw http.ResponseWriter, r *http.Request) {
	rows, err := w.db.Query("SELECT id, name, address, mode, username, created_at FROM hosts ORDER BY created_at ASC")
	if err != nil {
		http.Error(rw, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Host struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		Mode      string `json:"mode"`
		Username  string `json:"username"`
		CreatedAt string `json:"created_at"`
	}

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.Name, &h.Address, &h.Mode, &h.Username, &h.CreatedAt); err != nil {
			continue
		}
		hosts = append(hosts, h)
	}

	if hosts == nil {
		hosts = []Host{}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(hosts)
}
