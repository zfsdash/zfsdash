package wizard

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zfsdash/zfsdash/internal/auth"
)

type State string

const (
	StatePending      State = "pending"
	StateAdminCreated State = "admin_created"
	StateHostAdded    State = "host_added"
	StateComplete     State = "complete"
)

type Wizard struct {
	db   *sql.DB
	auth *auth.Manager
}

func NewWizard(db *sql.DB, authMgr *auth.Manager) *Wizard {
	initSchema(db)
	return &Wizard{db: db, auth: authMgr}
}

func initSchema(db *sql.DB) {
	db.Exec(`CREATE TABLE IF NOT EXISTS setup_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		state TEXT NOT NULL DEFAULT 'pending',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`INSERT OR IGNORE INTO setup_state (id, state) VALUES (1, 'pending')`)
}

func (w *Wizard) GetState() State {
	var state State
	w.db.QueryRow(`SELECT state FROM setup_state WHERE id = 1`).Scan(&state)
	if state == "" {
		return StatePending
	}
	return state
}

func (w *Wizard) setState(s State) {
	w.db.Exec(`UPDATE setup_state SET state = ?, updated_at = ? WHERE id = 1`, s, time.Now())
}

func (w *Wizard) IsComplete() bool {
	return w.GetState() == StateComplete
}

func (w *Wizard) HandleStatus(rw http.ResponseWriter, r *http.Request) {
	state := w.GetState()
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"state":    state,
		"complete": state == StateComplete,
	})
}

func (w *Wizard) HandleCreateAdmin(rw http.ResponseWriter, r *http.Request) {
	if w.GetState() != StatePending {
		http.Error(rw, `{"error":"admin already created"}`, http.StatusConflict)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(rw, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(rw, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}
	user, err := w.auth.CreateUser(req.Email, req.Password)
	if err != nil {
		http.Error(rw, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	w.setState(StateAdminCreated)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "user": user})
}

func (w *Wizard) HandleAddHost(rw http.ResponseWriter, r *http.Request) {
	if w.GetState() != StateAdminCreated {
		http.Error(rw, `{"error":"create admin first"}`, http.StatusConflict)
		return
	}
	var req struct {
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Mode == "" {
		http.Error(rw, `{"error":"mode required (local, ssh, truenas)"}`, http.StatusBadRequest)
		return
	}
	w.setState(StateComplete)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{"ok": true, "state": StateComplete})
}

// Middleware redirects to /setup if wizard not complete
func (w *Wizard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !w.IsComplete() && !isSetupPath(r.URL.Path) {
			http.Redirect(rw, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

func isSetupPath(path string) bool {
	return path == "/setup" ||
		path == "/api/setup/status" ||
		path == "/api/setup/admin" ||
		path == "/api/setup/host" ||
		path == "/api/health"
}
