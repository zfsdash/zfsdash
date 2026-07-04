package zfs

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PoolHealth is the full health report for a pool
type PoolHealth struct {
	Pool         string        `json:"pool"`
	Status       PoolStatus    `json:"status"`
	Issues       []PoolIssue   `json:"issues"`
	ResiverState *ResiverState `json:"resilver_state,omitempty"`
	CheckedAt    time.Time     `json:"checked_at"`
	RawStatus    string        `json:"raw_status"`
}

// IssueSeverity represents the severity of a pool health issue
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityWarning  IssueSeverity = "warning"
	SeverityInfo     IssueSeverity = "info"
)

// PoolStatus represents the overall health status of a pool
type PoolStatus string

const (
	PoolStatusOnline   PoolStatus = "ONLINE"
	PoolStatusDegraded PoolStatus = "DEGRADED"
	PoolStatusFaulted  PoolStatus = "FAULTED"
	PoolStatusOffline  PoolStatus = "OFFLINE"
	PoolStatusUnavail  PoolStatus = "UNAVAIL"
	PoolStatusRemoved  PoolStatus = "REMOVED"
	PoolStatusUnknown  PoolStatus = "UNKNOWN"
)

// PoolIssue represents a single health issue with repair steps
type PoolIssue struct {
	ID             string        `json:"id"`
	Severity       IssueSeverity `json:"severity"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	RootCause      string        `json:"root_cause"`
	AffectedDevice string        `json:"affected_device,omitempty"`
	ErrorCount     int           `json:"error_count,omitempty"`
	RepairSteps    []RepairStep  `json:"repair_steps"`
	Consequences   string        `json:"consequences"`
}

// RepairStep is a single actionable step in the repair process
type RepairStep struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Risk        string `json:"risk"`
}

// ResiverState tracks an active resilver operation
type ResiverState struct {
	IsActive       bool          `json:"is_active"`
	Type           string        `json:"type"`
	PercentDone    float64       `json:"percent_done"`
	ETA            string        `json:"eta"`
	SafeToUnplug  bool          `json:"safe_to_unplug"`
	Warning        string        `json:"warning,omitempty"`
}

var (
	scanRe    = regexp.MustCompile(`scan:\s+(resilver|scrub)\s+in\s+progress`)
	pctRe     = regexp.MustCompile(`(\d+\.\d+)%\s+done`)
	etaRe     = regexp.MustCompile(`to\s+go`)
	vdevErrRe = regexp.MustCompile(`(\S+)\s+(FAULTED|DEGRADED|UNAVAIL|REMOVED|OFFLINE)\s+(\d+)\s+(\d+)\s+(\d+)`)
	stateRe   = regexp.MustCompile(`state:\s+(\w+)`)
)

// ParsePoolHealth runs 'zpool status <pool>' and returns structured health
func ParsePoolHealth(pool string) (*PoolHealth, error) {
	out, err := exec.Command("zpool", "status", "-v", pool).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}

	health := &PoolHealth{
		Pool:      pool,
		Status:    PoolStatusUnknown,
		CheckedAt: time.Now(),
		RawStatus: string(out),
	}

	health.parse(string(out))
	return health, nil
}

func (h *PoolHealth) parse(raw string) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var inScan bool
	var scanLines []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Pool state
		if m := stateRe.FindStringSubmatch(trimmed); m != nil {
			h.Status = PoolStatus(m[1])
		}

		// Scan/resilver tracking
		if scanRe.MatchString(trimmed) {
			inScan = true
		}
		if inScan {
			scanLines = append(scanLines, trimmed)
			if len(scanLines) > 5 {
				inScan = false
			}
		}

		// Faulted/degraded vdevs
		if m := vdevErrRe.FindStringSubmatch(trimmed); m != nil {
			device := m[1]
			state := m[2]
			readErr, _ := strconv.Atoi(m[3])
			writeErr, _ := strconv.Atoi(m[4])
			cksumErr, _ := strconv.Atoi(m[5])
			totalErr := readErr + writeErr + cksumErr

			h.Issues = append(h.Issues, buildVdevIssue(device, state, pool, totalErr))
		}
	}

	// Parse resilver state from scan lines
	if len(scanLines) > 0 {
		h.ResiverState = parseResiverState(strings.Join(scanLines, " "))
	}

	// Add generic DEGRADED issue if no specific vdev issue found
	if h.Status == PoolStatusDegraded && len(h.Issues) == 0 {
		h.Issues = append(h.Issues, PoolIssue{
			ID:          "pool-degraded",
			Severity:    SeverityCritical,
			Title:       "Pool is DEGRADED",
			Description: fmt.Sprintf("Pool '%s' is operating in a degraded state. Redundancy is reduced.", pool),
			RootCause:   "One or more vdevs are not fully operational.",
			Consequences: "If another vdev fails, the pool may become unavailable. Data loss risk is elevated.",
			RepairSteps: []RepairStep{
				{Order: 1, Description: "Check full vdev status", Command: fmt.Sprintf("zpool status -v %s", pool), Risk: "none"},
				{Order: 2, Description: "Check SMART data on all drives", Command: "smartctl -a /dev/sdX", Risk: "none"},
				{Order: 3, Description: "Start scrub to check for errors", Command: fmt.Sprintf("zpool scrub %s", pool), Risk: "low"},
			},
		})
	}
}

func buildVdevIssue(device, state, pool string, errCount int) PoolIssue {
	switch state {
	case "FAULTED":
		return PoolIssue{
			ID:             fmt.Sprintf("vdev-faulted-%s", device),
			Severity:       SeverityCritical,
			Title:          fmt.Sprintf("Drive %s is FAULTED", device),
			Description:    fmt.Sprintf("Device %s has been marked FAULTED by ZFS due to excessive errors (%d total).", device, errCount),
			RootCause:      "Hardware failure, bad sectors, or loose SATA/SAS cable.",
			AffectedDevice: device,
			ErrorCount:     errCount,
			Consequences:   "Pool redundancy is reduced. Replace this drive immediately to restore protection.",
			RepairSteps: []RepairStep{
				{Order: 1, Description: "Check SMART status of the faulted drive", Command: fmt.Sprintf("smartctl -a %s", device), Risk: "none"},
				{Order: 2, Description: "Get a replacement drive of equal or larger size", Command: "", Risk: "none"},
				{Order: 3, Description: "Offline the faulted device", Command: fmt.Sprintf("zpool offline %s %s", pool, device), Risk: "low"},
				{Order: 4, Description: "Replace the physical drive, then replace in ZFS", Command: fmt.Sprintf("zpool replace %s %s /dev/NEW_DEVICE", pool, device), Risk: "medium"},
				{Order: 5, Description: "Monitor resilver progress", Command: fmt.Sprintf("zpool status %s", pool), Risk: "none"},
			},
		}
	case "DEGRADED":
		return PoolIssue{
			ID:             fmt.Sprintf("vdev-degraded-%s", device),
			Severity:       SeverityWarning,
			Title:          fmt.Sprintf("Device %s is DEGRADED", device),
			Description:    fmt.Sprintf("Device %s is reporting errors (%d total) but is still functioning.", device, errCount),
			RootCause:      "Intermittent hardware errors — drive may be failing.",
			AffectedDevice: device,
			ErrorCount:     errCount,
			Consequences:   "Drive may fail soon. Back up your data and plan a replacement.",
			RepairSteps: []RepairStep{
				{Order: 1, Description: "Run a scrub to assess error extent", Command: fmt.Sprintf("zpool scrub %s", pool), Risk: "low"},
				{Order: 2, Description: "Check SMART data", Command: fmt.Sprintf("smartctl -a %s", device), Risk: "none"},
				{Order: 3, Description: "Clear errors after fixing (if hardware issue resolved)", Command: fmt.Sprintf("zpool clear %s", pool), Risk: "low"},
			},
		}
	case "UNAVAIL":
		return PoolIssue{
			ID:             fmt.Sprintf("vdev-unavail-%s", device),
			Severity:       SeverityCritical,
			Title:          fmt.Sprintf("Device %s is UNAVAILABLE", device),
			Description:    fmt.Sprintf("Device %s cannot be accessed by ZFS.", device),
			RootCause:      "Drive disconnected, failed controller, or loose cable.",
			AffectedDevice: device,
			Consequences:   "Pool may be offline or read-only. Check physical connections immediately.",
			RepairSteps: []RepairStep{
				{Order: 1, Description: "Check physical connection — reseat the SATA/SAS cable", Command: "", Risk: "none"},
				{Order: 2, Description: "Check if drive is visible to OS", Command: "lsblk", Risk: "none"},
				{Order: 3, Description: "If drive reappears, try to online it", Command: fmt.Sprintf("zpool online %s %s", pool, device), Risk: "low"},
			},
		}
	default:
		return PoolIssue{
			ID:          fmt.Sprintf("vdev-%s-%s", strings.ToLower(state), device),
			Severity:    SeverityWarning,
			Title:       fmt.Sprintf("Device %s is %s", device, state),
			Description: fmt.Sprintf("Device %s is in state %s.", device, state),
			RootCause:   "Unknown. Check zpool status for details.",
			RepairSteps: []RepairStep{
				{Order: 1, Description: "Get full pool status", Command: fmt.Sprintf("zpool status -v %s", pool), Risk: "none"},
			},
		}
	}
}

func parseResiverState(line string) *ResiverState {
	rs := &ResiverState{IsActive: true}

	if strings.Contains(line, "resilver") {
		rs.Type = "resilver"
	} else {
		rs.Type = "scrub"
	}

	if m := pctRe.FindStringSubmatch(line); m != nil {
		rs.PercentDone, _ = strconv.ParseFloat(m[1], 64)
	}

	// Extract ETA from line like "with 2h14m to go"
	parts := strings.Split(line, "to go")
	if len(parts) > 0 {
		before := parts[0]
		lastWith := strings.LastIndex(before, "with ")
		if lastWith >= 0 {
			rs.ETA = strings.TrimSpace(before[lastWith+5:])
		}
	}

	rs.SafeToUnplug = rs.PercentDone >= 100.0
	if !rs.SafeToUnplug && rs.Type == "resilver" {
		rs.Warning = fmt.Sprintf("Do NOT remove any drives — resilver is %.1f%% complete. Removing a drive now may cause data loss.", rs.PercentDone)
	}

	return rs
}
