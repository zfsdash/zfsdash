package collector

import (
	"database/sql"
	"time"
)

// CapacityReading records pool capacity at a point in time.
type CapacityReading struct {
	PoolName  string    `json:"pool_name"`
	Used      uint64    `json:"used"`
	Avail     uint64    `json:"avail"`
	Total     uint64    `json:"total"`
	Pct       float64   `json:"pct"`
	RecordedAt time.Time `json:"recorded_at"`
}

// CapacityTrend holds the last N readings for a pool.
type CapacityTrend struct {
	PoolName string            `json:"pool_name"`
	Readings []CapacityReading `json:"readings"`
	// DaysUntilFull estimates days until pool hits 80% based on linear trend.
	// -1 means cannot estimate (not enough data or not growing).
	DaysUntilFull int `json:"days_until_full"`
}

// RecordCapacity saves a capacity reading to SQLite.
func RecordCapacity(db *sql.DB, poolName string, used, avail uint64) error {
	total := used + avail
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100.0
	}

	_, err := db.Exec(`
		INSERT INTO capacity_history (pool_name, used, avail, total, pct, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, poolName, used, avail, total, pct, time.Now().UTC())
	return err
}

// GetCapacityTrend retrieves the last 30 days of readings for a pool and
// estimates days until the pool reaches 80% capacity.
func GetCapacityTrend(db *sql.DB, poolName string) (*CapacityTrend, error) {
	rows, err := db.Query(`
		SELECT pool_name, used, avail, total, pct, recorded_at
		FROM capacity_history
		WHERE pool_name = ?
		  AND recorded_at > datetime('now', '-30 days')
		ORDER BY recorded_at ASC
	`, poolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trend := &CapacityTrend{PoolName: poolName, DaysUntilFull: -1}

	for rows.Next() {
		var r CapacityReading
		var ts string
		if err := rows.Scan(&r.PoolName, &r.Used, &r.Avail, &r.Total, &r.Pct, &ts); err != nil {
			continue
		}
		r.RecordedAt, _ = time.Parse(time.RFC3339, ts)
		trend.Readings = append(trend.Readings, r)
	}

	// Estimate days until 80% full using linear regression on last N points
	if len(trend.Readings) >= 2 {
		first := trend.Readings[0]
		last := trend.Readings[len(trend.Readings)-1]

		duration := last.RecordedAt.Sub(first.RecordedAt).Hours() / 24.0
		if duration > 0 && last.Pct > first.Pct {
			growthPerDay := (last.Pct - first.Pct) / duration
			if growthPerDay > 0 {
				remaining := 80.0 - last.Pct
				if remaining > 0 {
					trend.DaysUntilFull = int(remaining / growthPerDay)
				} else {
					trend.DaysUntilFull = 0 // Already over 80%
				}
			}
		}
	}

	return trend, rows.Err()
}

// InitCapacitySchema creates the capacity_history table if it doesn't exist.
func InitCapacitySchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS capacity_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			pool_name   TEXT NOT NULL,
			used        INTEGER NOT NULL,
			avail       INTEGER NOT NULL,
			total       INTEGER NOT NULL,
			pct         REAL NOT NULL,
			recorded_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_capacity_pool_time
			ON capacity_history(pool_name, recorded_at);
	`)
	return err
}
