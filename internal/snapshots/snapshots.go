package snapshots

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type VdevState struct {
	Name           string `json:"name"`
	State          string `json:"state"`
	ReadErrors     int    `json:"read_errors"`
	WriteErrors    int    `json:"write_errors"`
	ChecksumErrors int    `json:"checksum_errors"`
}

type PoolSnapshot struct {
	ID             int64
	Timestamp      time.Time
	PoolName       string
	PoolHealth     string
	CapacityBytes  int64
	AllocatedBytes int64
	FreeBytes      int64
	Vdevs          []VdevState
	ReadErrors     int
	WriteErrors    int
	ChecksumErrors int
	ScrubState     string
}

type Alert struct {
	ID        int64
	Timestamp time.Time
	PoolName  string
	AlertType string
	Severity  string
	Message   string
	Runbook   string
	Resolved  bool
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS pool_snapshots (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp INTEGER NOT NULL, pool_name TEXT NOT NULL, pool_health TEXT NOT NULL, capacity_bytes INTEGER NOT NULL DEFAULT 0, allocated_bytes INTEGER NOT NULL DEFAULT 0, free_bytes INTEGER NOT NULL DEFAULT 0, vdevs_json TEXT NOT NULL DEFAULT "[]", read_errors INTEGER NOT NULL DEFAULT 0, write_errors INTEGER NOT NULL DEFAULT 0, checksum_errors INTEGER NOT NULL DEFAULT 0, scrub_state TEXT NOT NULL DEFAULT "none", UNIQUE(timestamp, pool_name))`)
	if err != nil {
		return err
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_snapshots_pool_time ON pool_snapshots(pool_name, timestamp DESC)`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS pool_alerts (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp INTEGER NOT NULL, pool_name TEXT NOT NULL, alert_type TEXT NOT NULL, severity TEXT NOT NULL, message TEXT NOT NULL, runbook TEXT NOT NULL, resolved INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		return err
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_alerts_pool_time ON pool_alerts(pool_name, timestamp DESC)`)
	return nil
}

func StoreSnapshot(db *sql.DB, snap *PoolSnapshot) error {
	vdevsJSON, _ := json.Marshal(snap.Vdevs)
	_, err := db.Exec(
		`INSERT OR REPLACE INTO pool_snapshots (timestamp, pool_name, pool_health, capacity_bytes, allocated_bytes, free_bytes, vdevs_json, read_errors, write_errors, checksum_errors, scrub_state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.Timestamp.Unix(), snap.PoolName, snap.PoolHealth,
		snap.CapacityBytes, snap.AllocatedBytes, snap.FreeBytes,
		string(vdevsJSON), snap.ReadErrors, snap.WriteErrors, snap.ChecksumErrors,
		snap.ScrubState,
	)
	return err
}

func GetRecentSnapshots(db *sql.DB, poolName string, limit int) ([]PoolSnapshot, error) {
	rows, err := db.Query(
		`SELECT id, timestamp, pool_name, pool_health, capacity_bytes, allocated_bytes, free_bytes, vdevs_json, read_errors, write_errors, checksum_errors, scrub_state FROM pool_snapshots WHERE pool_name = ? ORDER BY timestamp DESC LIMIT ?`,
		poolName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snaps []PoolSnapshot
	for rows.Next() {
		var s PoolSnapshot
		var ts int64
		var vdevsJSON string
		if err := rows.Scan(&s.ID, &ts, &s.PoolName, &s.PoolHealth, &s.CapacityBytes, &s.AllocatedBytes, &s.FreeBytes, &vdevsJSON, &s.ReadErrors, &s.WriteErrors, &s.ChecksumErrors, &s.ScrubState); err != nil {
			continue
		}
		s.Timestamp = time.Unix(ts, 0)
		json.Unmarshal([]byte(vdevsJSON), &s.Vdevs)
		snaps = append(snaps, s)
	}
	return snaps, nil
}

func GetAlerts(db *sql.DB, poolName string, resolved bool, limit int) ([]Alert, error) {
	resolvedInt := 0
	if resolved {
		resolvedInt = 1
	}
	rows, err := db.Query(
		`SELECT id, timestamp, pool_name, alert_type, severity, message, runbook, resolved FROM pool_alerts WHERE pool_name = ? AND resolved = ? ORDER BY timestamp DESC LIMIT ?`,
		poolName, resolvedInt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []Alert
	for rows.Next() {
		var a Alert
		var ts int64
		var resolvedInt int
		if err := rows.Scan(&a.ID, &ts, &a.PoolName, &a.AlertType, &a.Severity, &a.Message, &a.Runbook, &resolvedInt); err != nil {
			continue
		}
		a.Timestamp = time.Unix(ts, 0)
		a.Resolved = resolvedInt == 1
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func DetectAndStoreAlerts(db *sql.DB, current *PoolSnapshot) {
	rows, err := db.Query(
		`SELECT pool_health, vdevs_json, read_errors, write_errors, checksum_errors FROM pool_snapshots WHERE pool_name = ? AND timestamp < ? ORDER BY timestamp DESC LIMIT 1`,
		current.PoolName, current.Timestamp.Unix())
	if err != nil || rows == nil {
		return
	}
	defer rows.Close()
	if !rows.Next() {
		return
	}
	var prevHealth, prevVdevsJSON string
	var prevRead, prevWrite, prevChecksum int
	rows.Scan(&prevHealth, &prevVdevsJSON, &prevRead, &prevWrite, &prevChecksum)

	if prevHealth != current.PoolHealth && current.PoolHealth != "ONLINE" {
		severity := "warning"
		if current.PoolHealth == "FAULTED" || current.PoolHealth == "UNAVAIL" {
			severity = "critical"
		}
		runbook := fmt.Sprintf("Run: zpool status %s -- identify faulted devices and replace. After replacement: zpool clear %s", current.PoolName, current.PoolName)
		db.Exec(`INSERT INTO pool_alerts (timestamp, pool_name, alert_type, severity, message, runbook) VALUES (?, ?, ?, ?, ?, ?)`,
			current.Timestamp.Unix(), current.PoolName, "health_change", severity,
			fmt.Sprintf("Pool %s health changed: %s to %s", current.PoolName, prevHealth, current.PoolHealth),
			runbook)
		slog.Warn("pool health alert", "pool", current.PoolName, "from", prevHealth, "to", current.PoolHealth)
	}

	var prevVdevs []VdevState
	json.Unmarshal([]byte(prevVdevsJSON), &prevVdevs)
	prevMap := make(map[string]string)
	for _, v := range prevVdevs {
		prevMap[v.Name] = v.State
	}
	for _, v := range current.Vdevs {
		if prev, ok := prevMap[v.Name]; ok && prev != v.State && !strings.EqualFold(v.State, "online") {
			db.Exec(`INSERT INTO pool_alerts (timestamp, pool_name, alert_type, severity, message, runbook) VALUES (?, ?, ?, ?, ?, ?)`,
				current.Timestamp.Unix(), current.PoolName, "vdev_state_change", "critical",
				fmt.Sprintf("Vdev %s in pool %s: %s to %s", v.Name, current.PoolName, prev, v.State),
				fmt.Sprintf("Run: zpool status %s -- replace failed device. After: zpool replace %s <old> <new>", current.PoolName, current.PoolName))
		}
	}

	errorDelta := (current.ReadErrors - prevRead) + (current.WriteErrors - prevWrite) + (current.ChecksumErrors - prevChecksum)
	if errorDelta > 10 {
		db.Exec(`INSERT INTO pool_alerts (timestamp, pool_name, alert_type, severity, message, runbook) VALUES (?, ?, ?, ?, ?, ?)`,
			current.Timestamp.Unix(), current.PoolName, "error_spike", "warning",
			fmt.Sprintf("Pool %s: +%d errors in last interval (R:%d W:%d C:%d)", current.PoolName, errorDelta, current.ReadErrors, current.WriteErrors, current.ChecksumErrors),
			fmt.Sprintf("Run: zpool status %s -- check drive SMART data. Run scrub: zpool scrub %s", current.PoolName, current.PoolName))
	}
}
