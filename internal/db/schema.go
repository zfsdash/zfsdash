package db

// schema contains all CREATE TABLE / CREATE INDEX statements.
// All statements use IF NOT EXISTS so the schema is idempotent on every startup.
// The ALTER TABLE statements at the bottom are wrapped in BEGIN/COMMIT and
// silently ignored if the column already exists (SQLite returns an error that
// we swallow in Migrate).
const schema = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_login_at DATETIME
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  token_hash TEXT NOT NULL DEFAULT '',
  ip_address TEXT,
  user_agent TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL,
  last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE COLLATE NOCASE,
  type TEXT NOT NULL,
  hostname TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  password TEXT NOT NULL DEFAULT '',
  api_key TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  is_active BOOLEAN NOT NULL DEFAULT 1,
  last_health_check_at DATETIME,
  last_health_status TEXT NOT NULL DEFAULT '',
  last_health_error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pool_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id TEXT NOT NULL,
  pool_name TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  alloc_bytes INTEGER NOT NULL,
  free_bytes INTEGER NOT NULL,
  health TEXT NOT NULL,
  recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pool_history_host ON pool_history(host_id, pool_name, recorded_at);

CREATE TABLE IF NOT EXISTS scrub_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id TEXT NOT NULL,
  pool_name TEXT NOT NULL,
  status TEXT NOT NULL,
  errors INTEGER NOT NULL DEFAULT 0,
  started_at DATETIME,
  ended_at DATETIME,
  recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS alert_configs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  host_id TEXT,
  metric TEXT NOT NULL,
  threshold REAL,
  condition TEXT NOT NULL,
  action TEXT NOT NULL,
  action_target TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// Migrate runs the base schema and any incremental column additions.
// Column additions use a "try ALTER TABLE, ignore error" pattern because
// SQLite does not support IF NOT EXISTS on ALTER TABLE ADD COLUMN prior
// to version 3.37.0, and modernc.org/sqlite may not always be at that
// version in all environments.
func (s *Store) runMigrations() {
	// token_hash column — added when we switched from plain-ID sessions to
	// SHA256-hashed tokens. Safe to ignore if already present.
	s.db.Exec(`ALTER TABLE sessions ADD COLUMN token_hash TEXT NOT NULL DEFAULT ''`)

	// last_login_at — tracks when the user last authenticated.
	s.db.Exec(`ALTER TABLE users ADD COLUMN last_login_at DATETIME`)
}
