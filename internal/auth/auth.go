package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSessionNotFound    = errors.New("session not found or expired")
	ErrUserExists         = errors.New("user already exists")
)

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) *Manager {
	if err := initSchema(db); err != nil {
		panic(err)
	}
	return &Manager{db: db}
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`)
	return err
}

func (m *Manager) CreateUser(email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := m.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrUserExists
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Email: email, CreatedAt: time.Now()}, nil
}

func (m *Manager) Login(email, password string) (string, error) {
	var id int64
	var hash string
	err := m.db.QueryRow(`SELECT id, password_hash FROM users WHERE email = ?`, email).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().Add(7 * 24 * time.Hour)
	_, err = m.db.Exec(`INSERT INTO sessions (user_id, token, expires_at) VALUES (?, ?, ?)`, id, token, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (m *Manager) ValidateSession(token string) (*User, error) {
	var u User
	var expiresAt time.Time
	err := m.db.QueryRow(`
		SELECT u.id, u.email, u.created_at, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?
	`, token).Scan(&u.ID, &u.Email, &u.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(expiresAt) {
		m.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil, ErrSessionNotFound
	}
	return &u, nil
}

func (m *Manager) Logout(token string) error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (m *Manager) UserCount() (int, error) {
	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// Middleware extracts session token from cookie or Authorization header
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie("zfsdash_session"); err == nil {
			token = c.Value
		} else if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, err := m.ValidateSession(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		r = r.WithContext(contextWithUser(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	token, err := m.Login(req.Email, req.Password)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "zfsdash_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("zfsdash_session"); err == nil {
		m.Logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "zfsdash_session", MaxAge: -1, Path: "/"})
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (m *Manager) HandleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user":null}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"user": user})
}
