package zfs

import (
	"fmt"
	"os/exec"
	"strings"
)

// PoolStatus represents the health state of a ZFS pool.
type PoolStatus string

const (
	PoolStatusOnline   PoolStatus = "ONLINE"
	PoolStatusDegraded PoolStatus = "DEGRADED"
	PoolStatusFaulted  PoolStatus = "FAULTED"
	PoolStatusOffline  PoolStatus = "OFFLINE"
	PoolStatusRemoved  PoolStatus = "REMOVED"
	PoolStatusUnavail  PoolStatus = "UNAVAIL"
)

// VdevIssue describes a problem with a specific vdev.
type VdevIssue struct {
	Device  string `json:"device"`
	State   string `json:"state"`
	Pool    string `json:"pool"`
	ErrRead uint64 `json:"err_read"`
	ErrWrite uint64 `json:"err_write"`
	ErrCksum uint64 `json:"err_cksum"`
}

// RepairStep is a suggested action to repair a pool issue.
type RepairStep struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Risk        string `json:"risk"` // none, low, medium, high
}

// PoolHealth contains the full health assessment of a pool.
type PoolHealth struct {
	Pool        string       `json:"pool"`
	Status      PoolStatus   `json:"status"`
	Issues      []VdevIssue  `json:"issues"`
	RepairSteps []RepairStep `json:"repair_steps"`
	RawStatus   string       `json:"raw_status"`
	ScrubStatus string       `json:"scrub_status"`
}

// ParsePoolHealth runs zpool status and returns a structured health report.
func ParsePoolHealth(poolName string) (*PoolHealth, error) {
	out, err := exec.Command("zpool", "status", "-v", poolName).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}

	h := &PoolHealth{
		Pool:      poolName,
		Status:    PoolStatusOnline,
		RawStatus: string(out),
	}
	h.parse(string(out))
	return h, nil
}

// parse extracts health details from zpool status output.
func (h *PoolHealth) parse(raw string) {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Overall h.Pool state
		if strings.HasPrefix(trimmed, "state:") {
			state := strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			h.Status = PoolStatus(state)
			continue
		}

		// Scan/scrub line
		if strings.HasPrefix(trimmed, "scan:") {
			h.ScrubStatus = strings.TrimSpace(strings.TrimPrefix(trimmed, "scan:"))
			continue
		}

		// Vdev lines: "  NAME   STATE   READ  WRITE  CKSUM"
		fields := strings.Fields(trimmed)
		if len(fields) >= 5 {
			device := fields[0]
			state := strings.ToUpper(fields[1])
			if state == "FAULTED" || state == "UNAVAIL" || state == "DEGRADED" || state == "REMOVED" {
				var errRead, errWrite, errCksum uint64
				fmt.Sscanf(fields[2], "%d", &errRead)
				fmt.Sscanf(fields[3], "%d", &errWrite)
				fmt.Sscanf(fields[4], "%d", &errCksum)
				totalErr := errRead + errWrite + errCksum
				h.Issues = append(h.Issues, buildVdevIssue(device, state, h.Pool, totalErr))
			}
		}
	}

	// Build repair steps based on status
	h.buildRepairSteps()
}

func (h *PoolHealth) buildRepairSteps() {
	switch h.Status {
	case PoolStatusFaulted:
		h.RepairSteps = []RepairStep{
			{Order: 1, Description: "Check full vdev status", Command: fmt.Sprintf("zpool status -v %s", h.Pool), Risk: "none"},
			{Order: 2, Description: "Clear transient errors", Command: fmt.Sprintf("zpool clear %s", h.Pool), Risk: "low"},
			{Order: 3, Description: "Replace failed drive", Command: fmt.Sprintf("zpool replace %s <old-dev> <new-dev>", h.Pool), Risk: "medium"},
			{Order: 4, Description: "Import pool if exported", Command: fmt.Sprintf("zpool import %s", h.Pool), Risk: "low"},
		}
	case PoolStatusDegraded:
		h.RepairSteps = []RepairStep{
			{Order: 1, Description: "Check full vdev status", Command: fmt.Sprintf("zpool status -v %s", h.Pool), Risk: "none"},
			{Order: 2, Description: "Identify failed device from issues list above", Command: "", Risk: "none"},
			{Order: 3, Description: "Start scrub to check for errors", Command: fmt.Sprintf("zpool scrub %s", h.Pool), Risk: "low"},
			{Order: 4, Description: "Replace degraded drive", Command: fmt.Sprintf("zpool replace %s <old-dev> <new-dev>", h.Pool), Risk: "medium"},
		}
	case PoolStatusOnline:
		if len(h.Issues) > 0 {
			h.RepairSteps = []RepairStep{
				{Order: 1, Description: "Run scrub to validate checksums", Command: fmt.Sprintf("zpool scrub %s", h.Pool), Risk: "low"},
				{Order: 2, Description: "Clear error counters after scrub", Command: fmt.Sprintf("zpool clear %s", h.Pool), Risk: "low"},
			}
		}
	}
}

func buildVdevIssue(device, state, pool string, errCount uint64) VdevIssue {
	return VdevIssue{
		Device:   device,
		State:    state,
		Pool:     pool,
		ErrRead:  errCount / 3,
		ErrWrite: errCount / 3,
		ErrCksum: errCount - (errCount/3)*2,
	}
}
