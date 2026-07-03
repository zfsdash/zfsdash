package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed data store.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at path.
func New(path string) (*Store, error) {
	conn, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1) // SQLite: single writer
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)
	return &Store{db: conn}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DB returns the underlying *sql.DB for use by sub-packages.
func (s *Store) DB() *sql.DB { return s.db }

// Migrate runs all schema migrations.
func (s *Store) Migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// IsFirstRun returns true if no users exist yet.
func (s *Store) IsFirstRun() (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// --- User operations ---

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	IsAdmin      bool
	IsActive     bool
	CreatedAt    time.Time
}

func (s *Store) CreateUser(id, username, email, passwordHash string, isAdmin bool) error {
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, is_admin) VALUES (?, ?, ?, ?, ?)`,
		id, username, email, passwordHash, isAdmin,
	)
	return err
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, is_admin, is_active, created_at FROM users WHERE username = ? COLLATE NOCASE`,
		username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (s *Store) GetUserByID(id string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, is_admin, is_active, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// --- Session operations ---

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s *Store) CreateSession(id, userID string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		id, userID, expiresAt,
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRow(
		`SELECT id, user_id, expires_at, created_at FROM sessions WHERE id = ? AND expires_at > datetime('now')`,
		id,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}

// --- Host operations ---

type Host struct {
	ID          string
	Name        string
	Type        string // local, ssh, truenas
	Hostname    string
	Port        int
	Username    string
	Password    string
	APIKey      string
	Description string
	IsActive    bool
	CreatedAt   time.Time
}

func (s *Store) CreateHost(h *Host) error {
	_, err := s.db.Exec(
		`INSERT INTO hosts (id, name, type, hostname, port, username, password, api_key, description, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.Type, h.Hostname, h.Port, h.Username, h.Password, h.APIKey, h.Description, h.IsActive,
	)
	return err
}

func (s *Store) ListHosts() ([]*Host, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, hostname, port, username, password, api_key, description, is_active, created_at FROM hosts WHERE is_active = 1 ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []*Host
	for rows.Next() {
		h := &Host{}
		if err := rows.Scan(&h.ID, &h.Name, &h.Type, &h.Hostname, &h.Port, &h.Username, &h.Password, &h.APIKey, &h.Description, &h.IsActive, &h.CreatedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *Store) GetHost(id string) (*Host, error) {
	h := &Host{}
	err := s.db.QueryRow(
		`SELECT id, name, type, hostname, port, username, password, api_key, description, is_active, created_at FROM hosts WHERE id = ?`,
		id,
	).Scan(&h.ID, &h.Name, &h.Type, &h.Hostname, &h.Port, &h.Username, &h.Password, &h.APIKey, &h.Description, &h.IsActive, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func (s *Store) DeleteHost(id string) error {
	_, err := s.db.Exec(`UPDATE hosts SET is_active = 0 WHERE id = ?`, id)
	return err
}
