package vdev

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type VdevHealthSample struct {
	Timestamp    time.Time
	PoolName     string
	VdevName     string
	State        string
	ReadErrors   int64
	WriteErrors  int64
	CksumErrors  int64
	Latency99    int64
	Temp         int
	Reallocated  int64
	PendingSect  int64
	PowerOnHrs   int64
}

type HealthMetric struct {
	Timestamp      time.Time `json:"timestamp"`
	PoolName       string    `json:"pool"`
	VdevName       string    `json:"vdev"`
	ErrorRate      float64   `json:"error_rate"`
	LatencyTrend   float64   `json:"latency_trend_pct"`
	RiskScore      float64   `json:"risk_score"`
	Recommendation string    `json:"recommendation"`
	SmartIssues    []string  `json:"smart_issues,omitempty"`
	DaysToFailure  int       `json:"days_to_failure"`
}

type HealthCollector struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHealthCollector(db *sql.DB, log *slog.Logger) *HealthCollector {
	hc := &HealthCollector{db: db, log: log}
	hc.initSchema()
	return hc
}

func (hc *HealthCollector) initSchema() {
	const schema = `
	CREATE TABLE IF NOT EXISTS vdev_health_samples (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		pool_name TEXT NOT NULL,
		vdev_name TEXT NOT NULL,
		state TEXT,
		read_errors INTEGER DEFAULT 0,
		write_errors INTEGER DEFAULT 0,
		cksum_errors INTEGER DEFAULT 0,
		latency_99 INTEGER DEFAULT 0,
		temp INTEGER DEFAULT -1,
		reallocated INTEGER DEFAULT 0,
		pending_sect INTEGER DEFAULT 0,
		power_on_hrs INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_vdev_health_pool_vdev_time
		ON vdev_health_samples(pool_name, vdev_name, timestamp);
	`
	if _, err := hc.db.Exec(schema); err != nil {
		hc.log.Error("vdev schema init failed", "err", err)
	}
}

func (hc *HealthCollector) CollectAll(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "zpool", "status", "-p").Output()
	if err != nil {
		return fmt.Errorf("zpool status: %w", err)
	}
	samples := parseZpoolStatus(string(out))
	for _, s := range samples {
		s.Timestamp = time.Now()
		if err := hc.record(ctx, s); err != nil {
			hc.log.Warn("record sample", "vdev", s.VdevName, "err", err)
		}
	}
	return nil
}

func (hc *HealthCollector) record(ctx context.Context, s VdevHealthSample) error {
	_, err := hc.db.ExecContext(ctx, `
		INSERT INTO vdev_health_samples
		(timestamp,pool_name,vdev_name,state,read_errors,write_errors,cksum_errors,latency_99,temp,reallocated,pending_sect,power_on_hrs)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.Timestamp, s.PoolName, s.VdevName, s.State,
		s.ReadErrors, s.WriteErrors, s.CksumErrors, s.Latency99,
		s.Temp, s.Reallocated, s.PendingSect, s.PowerOnHrs,
	)
	return err
}

func (hc *HealthCollector) Analyze(ctx context.Context, pool, vdev string, window time.Duration) (*HealthMetric, error) {
	cutoff := time.Now().Add(-window)
	rows, err := hc.db.QueryContext(ctx, `
		SELECT timestamp, state, read_errors, write_errors, cksum_errors, latency_99, temp, reallocated, pending_sect
		FROM vdev_health_samples
		WHERE pool_name=? AND vdev_name=? AND timestamp>=?
		ORDER BY timestamp ASC`, pool, vdev, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []VdevHealthSample
	for rows.Next() {
		var s VdevHealthSample
		var ts string
		rows.Scan(&ts, &s.State, &s.ReadErrors, &s.WriteErrors, &s.CksumErrors,
			&s.Latency99, &s.Temp, &s.Reallocated, &s.PendingSect)
		s.Timestamp, _ = time.Parse(time.RFC3339, ts)
		samples = append(samples, s)
	}
	return computeMetric(pool, vdev, samples), nil
}

func (hc *HealthCollector) AnalyzeAll(ctx context.Context) ([]HealthMetric, error) {
	rows, err := hc.db.QueryContext(ctx, `
		SELECT DISTINCT pool_name, vdev_name FROM vdev_health_samples
		WHERE timestamp >= datetime('now', '-24 hours')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HealthMetric
	for rows.Next() {
		var pool, vdev string
		rows.Scan(&pool, &vdev)
		m, err := hc.Analyze(ctx, pool, vdev, 24*time.Hour)
		if err == nil && m != nil {
			results = append(results, *m)
		}
	}
	return results, nil
}

func computeMetric(pool, vdev string, samples []VdevHealthSample) *HealthMetric {
	m := &HealthMetric{
		Timestamp: time.Now(),
		PoolName:  pool,
		VdevName:  vdev,
	}
	if len(samples) == 0 {
		m.Recommendation = "insufficient_data"
		return m
	}
	last := samples[len(samples)-1]
	m.ErrorRate = float64(last.ReadErrors + last.WriteErrors + last.CksumErrors)
	if len(samples) >= 4 {
		q1 := avgLatency(samples[:len(samples)/4])
		q4 := avgLatency(samples[3*len(samples)/4:])
		if q1 > 0 {
			m.LatencyTrend = (q4 - q1) / q1 * 100
		}
	}
	score := 0.0
	if last.CksumErrors > 0 {
		score += math.Min(float64(last.CksumErrors)*10, 40)
	}
	if last.ReadErrors > 0 || last.WriteErrors > 0 {
		score += math.Min(float64(last.ReadErrors+last.WriteErrors)*5, 30)
	}
	if m.LatencyTrend > 50 {
		score += 20
	} else if m.LatencyTrend > 20 {
		score += 10
	}
	if last.Reallocated > 0 {
		score += math.Min(float64(last.Reallocated)*2, 20)
		m.SmartIssues = append(m.SmartIssues, fmt.Sprintf("reallocated_sectors=%d", last.Reallocated))
	}
	if last.PendingSect > 0 {
		score += math.Min(float64(last.PendingSect)*5, 20)
		m.SmartIssues = append(m.SmartIssues, fmt.Sprintf("pending_sectors=%d", last.PendingSect))
	}
	if last.State != "ONLINE" && last.State != "" {
		score += 30
	}
	m.RiskScore = math.Min(score, 100)
	switch {
	case m.RiskScore >= 70:
		m.Recommendation = "replace"
		m.DaysToFailure = estimateDTF(samples)
	case m.RiskScore >= 40:
		m.Recommendation = "investigate"
		m.DaysToFailure = estimateDTF(samples)
	default:
		m.Recommendation = "healthy"
	}
	return m
}

func avgLatency(samples []VdevHealthSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s.Latency99)
	}
	return sum / float64(len(samples))
}

func estimateDTF(samples []VdevHealthSample) int {
	if len(samples) < 2 {
		return -1
	}
	n := float64(len(samples))
	var sumX, sumY, sumXY, sumX2 float64
	for i, s := range samples {
		x := float64(i)
		y := float64(s.ReadErrors + s.WriteErrors + s.CksumErrors)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return -1
	}
	slope := (n*sumXY - sumX*sumY) / denom
	if slope <= 0 {
		return -1
	}
	last := float64(samples[len(samples)-1].ReadErrors + samples[len(samples)-1].WriteErrors + samples[len(samples)-1].CksumErrors)
	remaining := 100.0 - last
	if remaining <= 0 {
		return 1
	}
	stepsLeft := remaining / slope
	if len(samples) >= 2 {
		interval := samples[1].Timestamp.Sub(samples[0].Timestamp)
		return int(time.Duration(stepsLeft) * interval / (24 * time.Hour))
	}
	return -1
}

func parseZpoolStatus(output string) []VdevHealthSample {
	var samples []VdevHealthSample
	var currentPool string
	newline := string([]byte{10})
	for _, line := range strings.Split(output, newline) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pool:") {
			currentPool = strings.TrimSpace(strings.TrimPrefix(line, "pool:"))
			continue
		}
		if currentPool == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			state := fields[1]
			if state == "ONLINE" || state == "DEGRADED" || state == "OFFLINE" || state == "FAULTED" || state == "REMOVED" {
				re, _ := strconv.ParseInt(fields[2], 10, 64)
				we, _ := strconv.ParseInt(fields[3], 10, 64)
				ce, _ := strconv.ParseInt(fields[4], 10, 64)
				samples = append(samples, VdevHealthSample{
					PoolName:    currentPool,
					VdevName:    fields[0],
					State:       state,
					ReadErrors:  re,
					WriteErrors: we,
					CksumErrors: ce,
				})
			}
		}
	}
	return samples
}
