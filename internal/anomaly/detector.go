package anomaly

import (
	"math"
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// ARCPoint is one ARC measurement.
type ARCPoint struct {
	Timestamp time.Time
	HitRatio  float64 // 0.0-1.0
	SizeBytes uint64
}

// AnomalyEvent fires when ARC hit ratio drops beyond threshold.
type AnomalyEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	Pool            string    `json:"pool"`
	CurrentRatio    float64   `json:"current_ratio"`
	BaselineMean    float64   `json:"baseline_mean"`
	BaselineStdDev  float64   `json:"baseline_stddev"`
	DeviationSigmas float64   `json:"deviation_sigmas"`
	Severity        Severity  `json:"severity"`
	Message         string    `json:"message"`
}

// Detector tracks a rolling window of ARC metrics and detects anomalies.
type Detector struct {
	mu           sync.RWMutex
	pool         string
	window       []ARCPoint
	windowSize   int
	sensitivity  float64
	baseline     baseline
	RecentEvents []AnomalyEvent
	maxEvents    int
}

type baseline struct {
	mean    float64
	stddev  float64
	samples int
}

// NewDetector creates a detector for one pool.
func NewDetector(pool string, windowSize int, sensitivity float64) *Detector {
	return &Detector{
		pool:        pool,
		windowSize:  windowSize,
		sensitivity: sensitivity,
		baseline:    baseline{mean: 0.85, stddev: 0.05},
		maxEvents:   200,
	}
}

// Record adds a new ARC measurement, updates baseline, and returns any anomaly.
func (d *Detector) Record(p ARCPoint) *AnomalyEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.window = append(d.window, p)
	if len(d.window) > d.windowSize {
		d.window = d.window[1:]
	}

	if len(d.window)%10 == 0 && len(d.window) >= 10 {
		d.recomputeBaseline()
	}

	if len(d.window) < 20 || d.baseline.stddev == 0 {
		return nil
	}

	threshold := d.baseline.mean - d.sensitivity*d.baseline.stddev
	if p.HitRatio >= threshold {
		return nil
	}

	sigmas := (d.baseline.mean - p.HitRatio) / d.baseline.stddev

	sev := SeverityWarning
	if sigmas >= 4.0 {
		sev = SeverityCritical
	}

	ev := AnomalyEvent{
		Timestamp:       p.Timestamp,
		Pool:            d.pool,
		CurrentRatio:    p.HitRatio,
		BaselineMean:    d.baseline.mean,
		BaselineStdDev:  d.baseline.stddev,
		DeviationSigmas: sigmas,
		Severity:        sev,
		Message:         "ARC hit ratio dropped below baseline",
	}

	d.RecentEvents = append(d.RecentEvents, ev)
	if len(d.RecentEvents) > d.maxEvents {
		d.RecentEvents = d.RecentEvents[len(d.RecentEvents)-d.maxEvents:]
	}

	return &ev
}

// Stats returns the current baseline for dashboard display.
func (d *Detector) Stats() (mean, stddev float64, samples int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.baseline.mean, d.baseline.stddev, d.baseline.samples
}

// Events returns a copy of recent anomaly events.
func (d *Detector) Events() []AnomalyEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]AnomalyEvent, len(d.RecentEvents))
	copy(out, d.RecentEvents)
	return out
}

func (d *Detector) recomputeBaseline() {
	n := float64(len(d.window))
	var sum, sumSq float64
	for _, p := range d.window {
		sum += p.HitRatio
		sumSq += p.HitRatio * p.HitRatio
	}
	mean := sum / n
	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}
	d.baseline = baseline{
		mean:    mean,
		stddev:  math.Sqrt(variance),
		samples: len(d.window),
	}
}
