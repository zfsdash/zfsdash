package wizard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Wizard struct{ db *sql.DB }

type StatusResponse struct {
	Complete bool   `json:"complete"`
	Step     string `json:"step"`
}

func New(db *sql.DB) *Wizard { return &Wizard{db: db} }

func (w *Wizard) InitSchema() error {
	_, err := w.db.Exec(`
	CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE IF NOT EXISTS hosts (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, address TEXT NOT NULL, mode TEXT NOT NULL DEFAULT 'local', username TEXT, ssh_key TEXT, api_key TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE IF NOT EXISTS admin_users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, email TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
	`)
	return err
}

func (w *Wizard) Status(rw http.ResponseWriter, r *http.Request) {
	w.InitSchema()
	var v string
	err := w.db.QueryRow("SELECT value FROM settings WHERE key='wizard_complete'").Scan(&v)
	rw.Header().Set("Content-Type", "application/json")
	if err == sql.ErrNoRows {
		json.NewEncoder(rw).Encode(StatusResponse{false, "admin_setup"})
		return
	}
	json.NewEncoder(rw).Encode(StatusResponse{v == "true", "complete"})
}

func (w *Wizard) Setup(rw http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password, Email string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Username == "" || req.Password == "" { http.Error(rw, "username and password required", 400); return }
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	tx, _ := w.db.Begin()
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT INTO admin_users (username, password_hash, email) VALUES (?,?,?)", req.Username, string(hash), req.Email); err != nil {
		http.Error(rw, "user already exists", 409); return
	}
	tx.Exec("INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES ('wizard_complete','true',?)", time.Now())
	tx.Commit()
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(201)
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (w *Wizard) AddHost(rw http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Address, Mode, Username, SSHKey, APIKey string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.Address == "" { http.Error(rw, "name and address required", 400); return }
	if req.Mode == "" { req.Mode = "local" }
	if _, err := w.db.Exec("INSERT INTO hosts (name,address,mode,username,ssh_key,api_key) VALUES (?,?,?,?,?,?)", req.Name, req.Address, req.Mode, req.Username, req.SSHKey, req.APIKey); err != nil {
		http.Error(rw, fmt.Sprintf("host creation failed: %v", err), 409); return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(201)
	json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
}

func (w *Wizard) ListHosts(rw http.ResponseWriter, r *http.Request) {
	rows, _ := w.db.Query("SELECT id, name, address, mode, username, created_at FROM hosts ORDER BY created_at ASC")
	defer rows.Close()
	type Host struct {
		ID int; Name, Address, Mode, Username, CreatedAt string
	}
	var hosts []Host
	for rows.Next() {
		var h Host
		rows.Scan(&h.ID, &h.Name, &h.Address, &h.Mode, &h.Username, &h.CreatedAt)
		hosts = append(hosts, h)
	}
	if hosts == nil { hosts = []Host{} }
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(hosts)
}
