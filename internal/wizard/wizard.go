package wizard

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zfsdash/zfsdash/internal/auth"
)

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS setup_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	return err
}

func IsComplete(db *sql.DB) bool {
	var value string
	err := db.QueryRow("SELECT value FROM setup_state WHERE key = 'completed'").Scan(&value)
	if err != nil {
		return false
	}
	return value == "true"
}

func MarkComplete(db *sql.DB) error {
	_, err := db.Exec("INSERT INTO setup_state (key, value) VALUES ('completed', 'true') ON CONFLICT(key) DO UPDATE SET value = 'true'")
	return err
}

type SetupStatusResponse struct {
	Completed    bool `json:"completed"`
	AdminCreated bool `json:"adminCreated"`
}

func SetupStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		completed := IsComplete(db)
		adminCreated := false
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err == nil && count > 0 {
			adminCreated = true
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SetupStatusResponse{Completed: completed, AdminCreated: adminCreated})
	}
}

type setupAdminReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type setupAdminResp struct {
	Token string `json:"token"`
}

func SetupAdminHandler(db *sql.DB, am *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if IsComplete(db) {
			http.Error(w, "Setup already complete", http.StatusConflict)
			return
		}
		var req setupAdminReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Email == "" || req.Password == "" {
			http.Error(w, "Email and password required", http.StatusBadRequest)
			return
		}
		_, err := am.CreateUser(req.Email, req.Password, "admin")
		if err != nil {
			http.Error(w, "Failed to create admin: "+err.Error(), http.StatusInternalServerError)
			return
		}
		token, err := am.AuthenticateUser(req.Email, req.Password)
		if err != nil {
			http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := MarkComplete(db); err != nil {
			http.Error(w, "Failed to mark setup complete: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(setupAdminResp{Token: token})
	}
}

func RequireSetup(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/setup/") {
			next.ServeHTTP(w, r)
			return
		}
		if !IsComplete(db) {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
