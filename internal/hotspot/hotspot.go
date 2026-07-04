package hotspot

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChecksumEvent struct {
	VdevGUID   string    `json:"vdev_guid"`
	Pool       string    `json:"pool"`
	VdevPath   string    `json:"vdev_path"`
	ErrorType  string    `json:"error_type"`
	ErrorCount int64     `json:"error_count"`
	DetectedAt time.Time `json:"detected_at"`
}

type VdevRisk struct {
	Pool               string    `json:"pool"`
	VdevPath           string    `json:"vdev_path"`
	CurrentErrors      int64     `json:"current_errors"`
	ErrorRate7d        float64   `json:"error_rate_7d"`
	ErrorRate24h       float64   `json:"error_rate_24h"`
	Trend              string    `json:"trend"`
	RiskScore          int       `json:"risk_score"`
	PredictedFailHours *float64  `json:"predicted_fail_hours,omitempty"`
	Recommendation     string    `json:"recommendation"`
	LastUpdated        time.Time `json:"last_updated"`
}

type Hotspot struct {
	Pool         string    `json:"pool"`
	VdevPath     string    `json:"vdev_path"`
	ErrorsIn24h  int64     `json:"errors_in_24h"`
	ErrorsIn1h   int64     `json:"errors_in_1h"`
	Severity     string    `json:"severity"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	SuggestedCmd string    `json:"suggested_cmd"`
}

type Tracker struct {
	mu       sync.RWMutex
	events   []ChecksumEvent
	maxAge   time.Duration
	baseline map[string]int64
}

func NewTracker() *Tracker {
	return &Tracker{maxAge: 72 * time.Hour, baseline: map[string]int64{}}
}

func (t *Tracker) Collect(ctx context.Context) ([]ChecksumEvent, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}
	events := t.parseZpoolStatus(string(out))
	t.mu.Lock()
	now := time.Now()
	t.events = append(t.events, events...)
	cutoff := now.Add(-t.maxAge)
	filtered := t.events[:0]
	for _, e := range t.events {
		if e.DetectedAt.After(cutoff) { filtered = append(filtered, e) }
	}
	t.events = filtered
	t.mu.Unlock()
	return events, nil
}

func (t *Tracker) parseZpoolStatus(output string) []ChecksumEvent {
	var events []ChecksumEvent
	var currentPool string
	now := time.Now()
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "pool:") {
			currentPool = strings.TrimSpace(strings.TrimPrefix(trimmed, "pool:"))
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 { continue }
		if fields[0] == "NAME" || fields[0] == "config:" || fields[0] == "errors:" { continue }
		stateCandidates := []string{"ONLINE","DEGRADED","FAULTED","OFFLINE","UNAVAIL","REMOVED"}
		isVdev := false
		for _, s := range stateCandidates { if fields[1] == s { isVdev = true; break } }
		if !isVdev { continue }
		vdevPath := fields[0]
		cksumErrors, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil || cksumErrors == 0 { continue }
		key := currentPool + "/" + vdevPath
		t.mu.RLock()
		prev := t.baseline[key]
		t.mu.RUnlock()
		delta := cksumErrors - prev
		if delta > 0 {
			events = append(events, ChecksumEvent{Pool: currentPool, VdevPath: vdevPath, ErrorType: "cksum", ErrorCount: delta, DetectedAt: now})
		}
		t.mu.Lock()
		t.baseline[key] = cksumErrors
		t.mu.Unlock()
	}
	return events
}

func (t *Tracker) DetectHotspots() []Hotspot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	now := time.Now()
	type counts struct{ in1h, in24h int64; first, last time.Time; pool string }
	byVdev := map[string]*counts{}
	for _, e := range t.events {
		key := e.Pool + "/" + e.VdevPath
		c, ok := byVdev[key]
		if !ok { c = &counts{pool: e.Pool, first: e.DetectedAt, last: e.DetectedAt}; byVdev[key] = c }
		age := now.Sub(e.DetectedAt)
		if age <= time.Hour { c.in1h += e.ErrorCount }
		if age <= 24*time.Hour { c.in24h += e.ErrorCount }
		if e.DetectedAt.Before(c.first) { c.first = e.DetectedAt }
		if e.DetectedAt.After(c.last) { c.last = e.DetectedAt }
	}
	var hotspots []Hotspot
	for key, c := range byVdev {
		if c.in24h < 2 { continue }
		parts := strings.SplitN(key, "/", 2)
		vdev := parts[1]
		severity := "warning"
		if c.in1h >= 3 || c.in24h >= 10 { severity = "critical" }
		hotspots = append(hotspots, Hotspot{Pool: c.pool, VdevPath: vdev, ErrorsIn24h: c.in24h, ErrorsIn1h: c.in1h, Severity: severity, FirstSeen: c.first, LastSeen: c.last, SuggestedCmd: fmt.Sprintf("zpool replace %s %s", c.pool, vdev)})
	}
	sort.Slice(hotspots, func(i, j int) bool { return hotspots[i].ErrorsIn24h > hotspots[j].ErrorsIn24h })
	return hotspots
}

func (t *Tracker) PredictFailures() []VdevRisk {
	t.mu.RLock()
	defer t.mu.RUnlock()
	now := time.Now()
	type bucket struct{ pool string; errors []ChecksumEvent }
	byVdev := map[string]*bucket{}
	for _, e := range t.events {
		key := e.Pool + "/" + e.VdevPath
		b, ok := byVdev[key]
		if !ok { b = &bucket{pool: e.Pool}; byVdev[key] = b }
		b.errors = append(b.errors, e)
	}
	var risks []VdevRisk
	for key, b := range byVdev {
		parts := strings.SplitN(key, "/", 2)
		vdev := parts[1]
		var total7d, total24h int64
		for _, e := range b.errors {
			age := now.Sub(e.DetectedAt)
			if age <= 7*24*time.Hour { total7d += e.ErrorCount }
			if age <= 24*time.Hour { total24h += e.ErrorCount }
		}
		rate7d := float64(total7d) / 7.0
		rate24h := float64(total24h)
		trend := "stable"
		if rate24h > rate7d*1.5 { trend = "accelerating" } else if rate24h > rate7d { trend = "increasing" }
		riskScore := 0
		if total7d > 0 { riskScore = int(math.Min(100, float64(total7d)*5+float64(total24h)*10)) }
		rec := "healthy"
		var predictedHours *float64
		switch {
		case riskScore >= 80: rec = "replace_immediately"; h := 24.0; predictedHours = &h
		case riskScore >= 50: rec = "schedule_replacement"; h := float64(168 - riskScore); predictedHours = &h
		case riskScore >= 20: rec = "monitor_closely"
		default: rec = "healthy"
		}
		risks = append(risks, VdevRisk{Pool: b.pool, VdevPath: vdev, CurrentErrors: total7d, ErrorRate7d: rate7d, ErrorRate24h: rate24h, Trend: trend, RiskScore: riskScore, PredictedFailHours: predictedHours, Recommendation: rec, LastUpdated: now})
	}
	sort.Slice(risks, func(i, j int) bool { return risks[i].RiskScore > risks[j].RiskScore })
	return risks
}

var _ = slog.Default
