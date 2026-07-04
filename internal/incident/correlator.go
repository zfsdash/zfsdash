package incident

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo        Severity = "info"
	SeverityWarning     Severity = "warning"
	SeverityDegradation Severity = "degradation"
	SeverityFailure     Severity = "failure"
)

type ZFSEvent struct {
	Time      time.Time
	Pool      string
	Vdev      string
	EventType string
	Message   string
	Count     int64
}

type CauseEvidence struct {
	EventType string
	Count     int
	Timespan  time.Duration
	Indicator string
}

type IncidentAction struct {
	Action      string
	Vdev        string
	Command     string
	Reason      string
	AutoExecute bool
}

type Incident struct {
	ID              string
	Pool            string
	RootCause       string
	Severity        Severity
	TimeRange       [2]time.Time
	Events          []ZFSEvent
	AffectedVdevs   []string
	Evidence        []CauseEvidence
	ConfidenceScore float64
	Actions         []IncidentAction
	Resolved        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Correlator struct {
	mu        sync.RWMutex
	incidents map[string]*Incident
	eventBuf  []ZFSEvent
	maxEvents int
}

func New() *Correlator {
	return &Correlator{
		incidents: make(map[string]*Incident),
		maxEvents: 1000,
	}
}

func (c *Correlator) Ingest(ev ZFSEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventBuf = append(c.eventBuf, ev)
	if len(c.eventBuf) > c.maxEvents {
		c.eventBuf = c.eventBuf[len(c.eventBuf)-c.maxEvents:]
	}
	c.correlate(ev)
}

func (c *Correlator) correlate(ev ZFSEvent) {
	window := 5 * time.Minute
	cutoff := ev.Time.Add(-window)
	var related []ZFSEvent
	for _, e := range c.eventBuf {
		if e.Time.After(cutoff) && e.Pool == ev.Pool {
			related = append(related, e)
		}
	}
	rootCause, confidence, evidence := classify(related)
	if confidence < 0.4 {
		return
	}
	incID := fmt.Sprintf("%s-%s", ev.Pool, rootCause)
	inc, exists := c.incidents[incID]
	if !exists {
		inc = &Incident{ID: incID, Pool: ev.Pool, CreatedAt: ev.Time, Severity: SeverityWarning}
		c.incidents[incID] = inc
		slog.Info("new incident", "id", incID, "root_cause", rootCause)
	}
	inc.RootCause = rootCause
	inc.ConfidenceScore = confidence
	inc.Evidence = evidence
	inc.Events = related
	inc.TimeRange = [2]time.Time{related[0].Time, related[len(related)-1].Time}
	inc.UpdatedAt = ev.Time
	inc.AffectedVdevs = uniqueVdevs(related)
	inc.Severity = severityFromCause(rootCause)
	inc.Actions = actionsForCause(rootCause, ev.Pool, inc.AffectedVdevs)
}

func classify(events []ZFSEvent) (string, float64, []CauseEvidence) {
	counts := map[string]int{}
	for _, e := range events {
		counts[e.EventType]++
	}
	var evidence []CauseEvidence
	span := time.Duration(0)
	if len(events) > 1 {
		span = events[len(events)-1].Time.Sub(events[0].Time)
	}
	checksums := counts["checksum_mismatch"]
	faulted := counts["vdev_faulted"]
	latency := counts["latency_spike"]
	scrubErr := counts["scrub_error"]
	if faulted > 0 && checksums > 5 {
		evidence = append(evidence, CauseEvidence{"vdev_faulted", faulted, span, "vdev marked FAULTED"})
		evidence = append(evidence, CauseEvidence{"checksum_mismatch", checksums, span, fmt.Sprintf("%d checksum errors", checksums)})
		return "drive_failure", 0.95, evidence
	}
	if checksums > 10 && latency > 2 {
		evidence = append(evidence, CauseEvidence{"checksum_mismatch", checksums, span, fmt.Sprintf("%d mismatches", checksums)})
		evidence = append(evidence, CauseEvidence{"latency_spike", latency, span, "sustained latency spikes"})
		return "sector_failure", 0.85, evidence
	}
	if scrubErr > 0 && checksums > 0 {
		evidence = append(evidence, CauseEvidence{"scrub_error", scrubErr, span, "scrub found errors"})
		return "silent_corruption", 0.75, evidence
	}
	if latency > 5 {
		evidence = append(evidence, CauseEvidence{"latency_spike", latency, span, fmt.Sprintf(">5 spikes in %s", span)})
		return "vdev_degraded", 0.60, evidence
	}
	return "unknown", 0.0, nil
}

func severityFromCause(cause string) Severity {
	switch cause {
	case "drive_failure":
		return SeverityFailure
	case "sector_failure", "silent_corruption":
		return SeverityDegradation
	default:
		return SeverityWarning
	}
}

func actionsForCause(cause, pool string, vdevs []string) []IncidentAction {
	switch cause {
	case "drive_failure":
		actions := []IncidentAction{}
		for _, v := range vdevs {
			actions = append(actions, IncidentAction{
				Action:  "replace_vdev",
				Vdev:    v,
				Command: fmt.Sprintf("zpool replace %s %s <new-device>", pool, v),
				Reason:  "drive FAULTED with checksum errors",
			})
		}
		actions = append(actions, IncidentAction{Action: "run_scrub", Command: fmt.Sprintf("zpool scrub %s", pool), Reason: "verify after replacement"})
		return actions
	case "sector_failure":
		return []IncidentAction{
			{Action: "run_scrub", Command: fmt.Sprintf("zpool scrub %s", pool), Reason: "identify sector error scope"},
		}
	case "silent_corruption":
		return []IncidentAction{
			{Action: "run_scrub", Command: fmt.Sprintf("zpool scrub %s", pool), Reason: "correct detected errors"},
		}
	default:
		return []IncidentAction{{Action: "investigate", Command: fmt.Sprintf("zpool status -v %s", pool)}}
	}
}

func uniqueVdevs(events []ZFSEvent) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range events {
		if e.Vdev != "" && !seen[e.Vdev] {
			seen[e.Vdev] = true
			out = append(out, e.Vdev)
		}
	}
	return out
}

func (c *Correlator) List() []*Incident {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*Incident, 0, len(c.incidents))
	for _, inc := range c.incidents {
		out = append(out, inc)
	}
	return out
}

func (c *Correlator) Resolve(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	inc, ok := c.incidents[id]
	if !ok {
		return false
	}
	inc.Resolved = true
	return true
}

func ParseZFSEventLine(line, pool string) (ZFSEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ZFSEvent{}, false
	}
	ev := ZFSEvent{Time: time.Now(), Pool: pool}
	switch {
	case strings.Contains(line, "checksum"):
		ev.EventType = "checksum_mismatch"
	case strings.Contains(line, "FAULTED") || strings.Contains(line, "faulted"):
		ev.EventType = "vdev_faulted"
	case strings.Contains(line, "latency"):
		ev.EventType = "latency_spike"
	case strings.Contains(line, "scrub") && strings.Contains(line, "error"):
		ev.EventType = "scrub_error"
	default:
		return ZFSEvent{}, false
	}
	ev.Message = line
	return ev, true
}
