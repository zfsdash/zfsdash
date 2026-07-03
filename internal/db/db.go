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

// DB returns the underlying *sql.DB (used by wizard and other sub-packages).
func (s *Store) DB() *sql.DB { return s.db }

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

// User is a database user record.
type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	IsAdmin      bool
	IsActive     bool
	CreatedAt    time.Time
	LastLoginAt  *time.Time
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

// ListUsers returns all users ordered by creation time.
func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, email, password_hash, is_admin, is_active, created_at FROM users ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser removes a user by ID (hard delete).
func (s *Store) DeleteUser(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// TouchLastLogin updates the last_login_at timestamp for a user.
func (s *Store) TouchLastLogin(userID string) error {
	_, err := s.db.Exec(
		`UPDATE users SET last_login_at = datetime('now') WHERE id = ?`,
		userID,
	)
	return err
}

// --- Session operations (SHA256-hashed token storage) ---

// Session represents a database session record.
type Session struct {
	ID             string
	UserID         string
	TokenHash      string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	LastActivityAt time.Time
}

// CreateSessionByHash inserts a new session using the pre-hashed token.
// The caller must pass hashToken(plaintextToken) as tokenHash.
func (s *Store) CreateSessionByHash(id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at) VALUES (?, ?, ?, ?)`,
		id, userID, tokenHash, expiresAt,
	)
	return err
}

// GetSessionByHash retrieves a valid (non-expired) session by its token hash.
// Returns (nil, nil) when no matching session is found.
func (s *Store) GetSessionByHash(tokenHash string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at, last_activity_at
		 FROM sessions
		 WHERE token_hash = ? AND expires_at > datetime('now')`,
		tokenHash,
	).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.ExpiresAt, &sess.CreatedAt, &sess.LastActivityAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

// DeleteSessionByHash removes a session by token hash (logout).
func (s *Store) DeleteSessionByHash(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// TouchSessionActivity updates last_activity_at for a session.
func (s *Store) TouchSessionActivity(sessionID string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET last_activity_at = datetime('now') WHERE id = ?`,
		sessionID,
	)
	return err
}

// DeleteExpiredSessions purges all expired sessions.
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}

// --- Host operations ---

// Host is a database host record.
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
		`SELECT id, name, type, hostname, port, username, password, api_key, description, is_active, created_at
		 FROM hosts WHERE is_active = 1 ORDER BY name`,
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
		`SELECT id, name, type, hostname, port, username, password, api_key, description, is_active, created_at
		 FROM hosts WHERE id = ?`,
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

// --- Pool history ---

// PoolSnapshot is a point-in-time pool metric record.
type PoolSnapshot struct {
	HostID     string    `json:"host_id"`
	PoolName   string    `json:"pool_name"`
	SizeBytes  int64     `json:"size_bytes"`
	AllocBytes int64     `json:"alloc_bytes"`
	FreeBytes  int64     `json:"free_bytes"`
	Health     string    `json:"health"`
	RecordedAt time.Time `json:"recorded_at"`
}

// GetPoolHistory returns up to limit recent pool snapshots.
func (s *Store) GetPoolHistory(hostID, poolName string, limit int) ([]PoolSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT host_id, pool_name, size_bytes, alloc_bytes, free_bytes, health, recorded_at
		 FROM pool_history
		 WHERE host_id = ? AND pool_name = ?
		 ORDER BY recorded_at DESC LIMIT ?`,
		hostID, poolName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PoolSnapshot
	for rows.Next() {
		var snap PoolSnapshot
		if err := rows.Scan(&snap.HostID, &snap.PoolName, &snap.SizeBytes, &snap.AllocBytes, &snap.FreeBytes, &snap.Health, &snap.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// --- Scrub history ---

// ScrubRecord is a single scrub history entry.
type ScrubRecord struct {
	HostID    string     `json:"host_id"`
	PoolName  string     `json:"pool_name"`
	Status    string     `json:"status"`
	Errors    int        `json:"errors"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// GetScrubHistory returns up to limit recent scrub records.
func (s *Store) GetScrubHistory(hostID, poolName string, limit int) ([]ScrubRecord, error) {
	rows, err := s.db.Query(
		`SELECT host_id, pool_name, status, errors, started_at, ended_at
		 FROM scrub_history
		 WHERE host_id = ? AND pool_name = ?
		 ORDER BY recorded_at DESC LIMIT ?`,
		hostID, poolName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScrubRecord
	for rows.Next() {
		var rec ScrubRecord
		if err := rows.Scan(&rec.HostID, &rec.PoolName, &rec.Status, &rec.Errors, &rec.StartedAt, &rec.EndedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
