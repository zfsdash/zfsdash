// Package auth implements user authentication and session management for ZFSdash.
//
// Session tokens are 32 random bytes encoded as a 64-character hex string.
// Only the SHA256 hash of the token is stored in the database; the plaintext
// token is returned to the caller once and never persisted.
//
// Passwords are hashed with bcrypt at cost 12.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zfsdash/zfsdash/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionDuration = 7 * 24 * time.Hour
	bcryptCost      = 12
)

// Service handles authentication and session lifecycle.
type Service struct {
	store *db.Store
}

// New creates a new auth Service backed by the given store.
func New(store *db.Store) *Service {
	return &Service{store: store}
}

// --- Password helpers ---

// HashPassword returns a bcrypt hash of password using cost 12.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// --- Token helpers ---

// generateToken creates a cryptographically random 64-char hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the hex-encoded SHA256 of token for database storage.
// The plaintext token is never stored; only this hash is persisted.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// --- User management ---

// CreateAdminUser creates the initial admin account. Returns an error if a user
// with the same username already exists.
func (s *Service) CreateAdminUser(username, email, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.CreateUser(uuid.New().String(), username, email, hash, true)
}

// CreateUser creates a non-admin user account.
func (s *Service) CreateUser(username, email, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.CreateUser(uuid.New().String(), username, email, hash, false)
}

// ListUsers returns all users in the database.
func (s *Service) ListUsers() ([]*db.User, error) {
	return s.store.ListUsers()
}

// DeleteUser removes the user with the given ID.
func (s *Service) DeleteUser(id string) error {
	return s.store.DeleteUser(id)
}

// --- Session management ---

// Login verifies username/password credentials and returns a plaintext session
// token on success. The token must be sent to the client as a cookie; its
// SHA256 hash is what is stored in the database.
func (s *Service) Login(username, password string) (string, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	if user == nil || !user.IsActive {
		return "", fmt.Errorf("invalid credentials")
	}
	if !CheckPassword(user.PasswordHash, password) {
		return "", fmt.Errorf("invalid credentials")
	}

	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(sessionDuration)

	if err := s.store.CreateSessionByHash(uuid.New().String(), user.ID, tokenHash, expiresAt); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	if err := s.store.TouchLastLogin(user.ID); err != nil {
		// Non-fatal: login succeeded even if we can't update last_login_at.
		fmt.Printf("[auth] touch last_login_at: %v\n", err)
	}

	return token, nil
}

// ValidateSession looks up the session for the given plaintext token and
// returns the associated user. Returns (nil, nil) for an unknown or expired
// token — the caller should treat this as "not authenticated".
func (s *Service) ValidateSession(token string) (*db.User, error) {
	if token == "" {
		return nil, nil
	}
	tokenHash := hashToken(token)
	sess, err := s.store.GetSessionByHash(tokenHash)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	// Slide the last_activity_at timestamp.
	_ = s.store.TouchSessionActivity(sess.ID)

	return s.store.GetUserByID(sess.UserID)
}

// Logout deletes the session identified by the plaintext token.
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	tokenHash := hashToken(token)
	return s.store.DeleteSessionByHash(tokenHash)
}
