package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// DB wraps a SQLite database for ZFSdash history
type DB struct {
	sqlDB *sql.DB
}

// Open opens (or creates) the SQLite database at the given path
func Open(path string) (*DB, error) {
	if path == "" {
		path = "/var/lib/zfsdash/history.db"
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d := &DB{sqlDB: sqlDB}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) migrate() error {
	_, err := d.sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS pool_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host TEXT NOT NULL,
			pool_name TEXT NOT NULL,
			size INTEGER,
			allocated INTEGER,
			free INTEGER,
			capacity INTEGER,
			health TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS scrub_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			host TEXT NOT NULL,
			pool_name TEXT NOT NULL,
			start_time DATETIME,
			end_time DATETIME,
			duration_sec INTEGER,
			errors INTEGER,
			state TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_pool_snapshots_host_pool ON pool_snapshots(host, pool_name);
		CREATE INDEX IF NOT EXISTS idx_scrub_history_host_pool ON scrub_history(host, pool_name);
	`)
	return err
}

// RecordPoolSnapshot saves a pool state snapshot
func (d *DB) RecordPoolSnapshot(host string, p *zfs.Pool) error {
	_, err := d.sqlDB.Exec(
		`INSERT INTO pool_snapshots (host, pool_name, size, allocated, free, capacity, health) VALUES (?,?,?,?,?,?,?)`,
		host, p.Name, p.Size, p.Allocated, p.Free, p.Capacity, p.Health,
	)
	return err
}

// RecordScrub saves a scrub history entry
func (d *DB) RecordScrub(host, poolName string, s *zfs.Scrub) error {
	_, err := d.sqlDB.Exec(
		`INSERT INTO scrub_history (host, pool_name, start_time, end_time, duration_sec, errors, state) VALUES (?,?,?,?,?,?,?)`,
		host, poolName, s.StartTime, s.EndTime, s.DurationSec, s.Errors, s.State,
	)
	return err
}

// GetPoolHistory returns pool snapshots for a host+pool, newest first, limited to n rows
func (d *DB) GetPoolHistory(host, poolName string, n int) ([]*zfs.PoolSnapshot, error) {
	rows, err := d.sqlDB.Query(
		`SELECT id, pool_name, size, allocated, free, capacity, health, created_at
		 FROM pool_snapshots WHERE host=? AND pool_name=?
		 ORDER BY created_at DESC LIMIT ?`,
		host, poolName, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*zfs.PoolSnapshot
	for rows.Next() {
		s := &zfs.PoolSnapshot{}
		var createdAt string
		if err := rows.Scan(&s.ID, &s.PoolName, &s.Size, &s.Allocated, &s.Free, &s.Capacity, &s.Health, &createdAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetScrubHistory returns scrub history for a host+pool
func (d *DB) GetScrubHistory(host, poolName string, n int) ([]*zfs.ScrubHistory, error) {
	rows, err := d.sqlDB.Query(
		`SELECT id, pool_name, start_time, end_time, duration_sec, errors, state, created_at
		 FROM scrub_history WHERE host=? AND pool_name=?
		 ORDER BY created_at DESC LIMIT ?`,
		host, poolName, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*zfs.ScrubHistory
	for rows.Next() {
		s := &zfs.ScrubHistory{}
		var start, end, created string
		if err := rows.Scan(&s.ID, &s.PoolName, &start, &end, &s.Duration, &s.Errors, &s.State, &created); err != nil {
			return nil, err
		}
		s.StartTime, _ = time.Parse("2006-01-02 15:04:05", start)
		s.EndTime, _ = time.Parse("2006-01-02 15:04:05", end)
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		result = append(result, s)
	}
	return result, rows.Err()
}

// Close closes the database
func (d *DB) Close() error {
	return d.sqlDB.Close()
}
