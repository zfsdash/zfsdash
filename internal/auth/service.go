package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Service handles user authentication.
type Service struct {
	db *sql.DB
}

// NewService creates a new auth Service backed by the given database.
func NewService(db *sql.DB) *Service {
	_ = initSchema(db)
	return &Service{db: db}
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	return err
}

// Register creates a new user and returns a session token.
func (s *Service) Register(email, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	res, err := s.db.Exec(`INSERT INTO users (email, password) VALUES (?, ?)`, email, string(hash))
	if err != nil {
		return "", fmt.Errorf("email already registered")
	}
	userID, _ := res.LastInsertId()
	return s.createSession(userID)
}

// Login authenticates a user and returns a session token.
func (s *Service) Login(email, password string) (string, error) {
	var userID int64
	var hash string
	err := s.db.QueryRow(`SELECT id, password FROM users WHERE email = ?`, email).Scan(&userID, &hash)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return "", fmt.Errorf("db error: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	return s.createSession(userID)
}

// ValidateToken checks a session token and returns the user ID.
func (s *Service) ValidateToken(token string) (int64, error) {
	var userID int64
	var expiresAt time.Time
	err := s.db.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("invalid token")
	}
	if err != nil {
		return 0, fmt.Errorf("db error: %w", err)
	}
	if time.Now().After(expiresAt) {
		return 0, fmt.Errorf("token expired")
	}
	return userID, nil
}

func (s *Service) createSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	_, err := s.db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expiry)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}
