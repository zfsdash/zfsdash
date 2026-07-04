package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/zfsdash/zfsdash/internal/db"
)

// Service provides authentication operations backed by db.Store.
type Service struct {
	store *db.Store
}

// New creates a new auth Service. Alias for NewService.
func New(store *db.Store) *Service { return NewService(store) }

// NewService creates a new auth Service.
func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

// CreateAdminUser creates the first admin user during wizard setup.
func (s *Service) CreateAdminUser(username, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	id := uuid.New().String()
	return s.store.CreateUser(id, username, email, string(hash), true)
}

// Login authenticates by username+password and returns a plaintext session token.
func (s *Service) Login(username, password string) (string, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("db error: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if !user.IsActive {
		return "", fmt.Errorf("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	plaintext := hex.EncodeToString(b)
	tokenHash := hashToken(plaintext)
	sessID := uuid.New().String()
	expiry := time.Now().Add(7 * 24 * time.Hour)
	if err := s.store.CreateSessionByHash(sessID, user.ID, tokenHash, expiry); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	_ = s.store.TouchLastLogin(user.ID)
	return plaintext, nil
}

// ValidateSession looks up a session by plaintext token and returns the user.
// Returns (nil, nil) when no matching session exists.
func (s *Service) ValidateSession(token string) (*db.User, error) {
	tokenHash := hashToken(token)
	sess, err := s.store.GetSessionByHash(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}
	if sess == nil {
		return nil, nil
	}
	_ = s.store.TouchSessionActivity(sess.ID)
	user, err := s.store.GetUserByID(sess.UserID)
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}
	return user, nil
}

// Logout deletes the session identified by the plaintext token.
func (s *Service) Logout(token string) error {
	return s.store.DeleteSessionByHash(hashToken(token))
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
