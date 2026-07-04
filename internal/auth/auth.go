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

const (
	TokenLength     = 32
	SessionDuration = 7 * 24 * time.Hour
	RoleAdmin       = "admin"
	RoleUser        = "user"
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

// Login authenticates a user and creates a session.
func (s *Service) Login(email, password string) (*db.Session, error) {
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	session := &db.Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(SessionDuration),
		CreatedAt: time.Now(),
	}
	if err := s.store.CreateSession(session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

// ValidateSession checks if a token is valid and not expired.
func (s *Service) ValidateSession(token string) (*db.Session, error) {
	session, err := s.store.GetSession(token)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.store.DeleteSession(token)
		return nil, fmt.Errorf("session expired")
	}
	return session, nil
}

// Logout deletes a session.
func (s *Service) Logout(token string) error {
	return s.store.DeleteSession(token)
}

// CreateAdminUser creates a new admin user.
func (s *Service) CreateAdminUser(email, name, password string) (*db.User, error) {
	existing, err := s.store.GetUserByEmail(email)
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
	user := &db.User{
		ID:           uuid.New().String(),
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         RoleAdmin,
		CreatedAt:    time.Now(),
	}
	if err := s.store.CreateUser(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}
