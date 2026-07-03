package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database for ZFSdash history.
type DB struct {
	sql *sql.DB
}

// PoolHistory is a historical record of pool capacity.
type PoolHistory struct {
	ID        int64     `json:"id"`
	Host      string    `json:"host"`
	Pool      string    `json:"pool"`
	Capacity  float64   `json:"capacity"`
	Allocated uint64    `json:"allocated"`
	Size      uint64    `json:"size"`
	State     string    `json:"state"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Open opens (or creates) the SQLite database at path.
func Open(path string) (*DB, error) {
	if path == "" {
		path = "/var/lib/zfsdash/history.db"
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (d *DB) migrate() error {
	_, err := d.sql.Exec(`
		CREATE TABLE IF NOT EXISTS pool_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			host        TEXT NOT NULL,
			pool        TEXT NOT NULL,
			capacity    REAL NOT NULL,
			allocated   INTEGER NOT NULL,
			size        INTEGER NOT NULL,
			state       TEXT NOT NULL,
			recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_pool_history_host_pool ON pool_history(host, pool);
		CREATE INDEX IF NOT EXISTS idx_pool_history_recorded_at ON pool_history(recorded_at);
	`)
	return err
}

// RecordPoolHistory inserts a pool capacity snapshot.
func (d *DB) RecordPoolHistory(host, pool string, capacity float64, allocated, size uint64, state string) error {
	_, err := d.sql.Exec(
		`INSERT INTO pool_history (host, pool, capacity, allocated, size, state, recorded_at) VALUES (?,?,?,?,?,?,?)`,
		host, pool, capacity, allocated, size, state, time.Now(),
	)
	return err
}

// GetPoolHistory returns the last N records for a host+pool.
func (d *DB) GetPoolHistory(host, pool string, limit int) ([]*PoolHistory, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.sql.Query(
		`SELECT id, host, pool, capacity, allocated, size, state, recorded_at
		 FROM pool_history WHERE host=? AND pool=?
		 ORDER BY recorded_at DESC LIMIT ?`,
		host, pool, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PoolHistory
	for rows.Next() {
		r := &PoolHistory{}
		if err := rows.Scan(&r.ID, &r.Host, &r.Pool, &r.Capacity, &r.Allocated, &r.Size, &r.State, &r.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Close closes the database.
func (d *DB) Close() error {
	return d.sql.Close()
}
