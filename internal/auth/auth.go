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
	TokenLength     = 32
	SessionDuration = 7 * 24 * time.Hour
)

// Service manages authentication and sessions.
type Service struct {
	store *db.Store
}

// New creates a new auth Service.
func New(store *db.Store) *Service {
	return &Service{store: store}
}

// generateToken creates a cryptographically random session token.
func generateToken() (string, error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the SHA-256 hex hash of a plaintext token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Login authenticates a user by username+password and creates a session.
// Returns the plaintext session token (stored hashed in DB).
func (s *Service) Login(username, password string) (string, *db.User, error) {
	user, err := s.store.GetUserByUsername(username)
	if err != nil {
		return "", nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.IsActive {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	token, err := generateToken()
	if err != nil {
		return "", nil, err
	}
	sessID := uuid.New().String()
	if err := s.store.CreateSessionByHash(sessID, user.ID, hashToken(token), time.Now().Add(SessionDuration)); err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}
	_ = s.store.TouchLastLogin(user.ID)
	return token, user, nil
}

// ValidateSession validates a plaintext token and returns the session + user.
func (s *Service) ValidateSession(token string) (*db.Session, *db.User, error) {
	sess, err := s.store.GetSessionByHash(hashToken(token))
	if err != nil {
		return nil, nil, fmt.Errorf("get session: %w", err)
	}
	if sess == nil {
		return nil, nil, fmt.Errorf("session not found or expired")
	}
	user, err := s.store.GetUserByID(sess.UserID)
	if err != nil || user == nil {
		return nil, nil, fmt.Errorf("user not found")
	}
	_ = s.store.TouchSessionActivity(sess.ID)
	return sess, user, nil
}

// Logout invalidates a session by plaintext token.
func (s *Service) Logout(token string) error {
	return s.store.DeleteSessionByHash(hashToken(token))
}

// CreateAdminUser creates the first admin user.
func (s *Service) CreateAdminUser(username, email, password string) (*db.User, error) {
	existing, err := s.store.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("check existing: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	id := uuid.New().String()
	if err := s.store.CreateUser(id, username, email, string(hash), true); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return s.store.GetUserByID(id)
}
