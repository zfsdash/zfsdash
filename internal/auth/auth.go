package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zfsdash/zfsdash/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 7 * 24 * time.Hour
const bcryptCost = 12

// Service handles authentication.
type Service struct {
	store *db.Store
}

// New creates a new auth service.
func New(store *db.Store) *Service {
	return &Service{store: store}
}

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateAdminUser creates the initial admin user.
func (s *Service) CreateAdminUser(username, email, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.CreateUser(uuid.New().String(), username, email, hash, true)
}

// Login verifies credentials and returns a session token.
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

	expiresAt := time.Now().Add(sessionDuration)
	if err := s.store.CreateSession(token, user.ID, expiresAt); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// ValidateSession returns the user for a valid session token.
func (s *Service) ValidateSession(token string) (*db.User, error) {
	if token == "" {
		return nil, nil
	}
	sess, err := s.store.GetSession(token)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	return s.store.GetUserByID(sess.UserID)
}

// Logout deletes a session.
func (s *Service) Logout(token string) error {
	return s.store.DeleteSession(token)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
