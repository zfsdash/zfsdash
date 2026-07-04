package dlq

import (
	"bufio"
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FailureClass categorizes ZFS silent failure types
type FailureClass string

const (
	ClassChecksumError     FailureClass = "checksum_error"
	ClassReadError         FailureClass = "read_error"
	ClassWriteError        FailureClass = "write_error"
	ClassOrphanedSnapshot  FailureClass = "orphaned_snapshot"
	ClassSilentCorruption  FailureClass = "silent_corruption"
	ClassVdevDegrade       FailureClass = "vdev_degrade"
	ClassTXGTimeout        FailureClass = "txg_timeout"
	ClassARCEviction       FailureClass = "arc_eviction_storm"
)

// Event represents a detected silent failure
type Event struct {
	ID          string       `json:"id"`
	PoolName    string       `json:"pool_name"`
	FailureClass FailureClass `json:"failure_class"`
	Severity    string       `json:"severity"`
	Message     string       `json:"message"`
	RawLine     string       `json:"raw_line,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
	DetectedAt  time.Time    `json:"detected_at"`
	Remediation string       `json:"remediation"`
	Status      string       `json:"status"` // detected, acknowledged, resolved
}

// Manager coordinates DLQ detection and storage
type Manager struct {
	mu      sync.RWMutex
	events  []*Event
	maxSize int
	logger  *slog.Logger
	pools   []string
}

func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		events:  make([]*Event, 0, 200),
		maxSize: 200,
		logger:  logger,
	}
}

// Watch starts background polling for ZFS kernel events
func (m *Manager) Watch(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run immediately on start
	m.collect()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

func (m *Manager) collect() {
	pools := m.listPools()
	for _, pool := range pools {
		m.checkPoolErrors(pool)
	}
	m.checkKernelLog()
	m.checkARCStats()
}

func (m *Manager) listPools() []string {
	out, err := exec.Command("zpool", "list", "-H", "-o", "name").Output()
	if err != nil {
		return nil
	}
	var pools []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name != "" {
			pools = append(pools, name)
		}
	}
	return pools
}

func (m *Manager) checkPoolErrors(pool string) {
	out, err := exec.Command("zpool", "status", "-v", pool).Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	var inErrors bool
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "errors:") {
			if strings.Contains(line, "No known data errors") {
				return
			}
			inErrors = true
			continue
		}
		if inErrors && line != "" {
			m.addEvent(&Event{
				ID:           generateID(),
				PoolName:     pool,
				FailureClass: ClassChecksumError,
				Severity:     "critical",
				Message:      "Pool reports data errors: " + line,
				RawLine:      line,
				Timestamp:    time.Now(),
				DetectedAt:   time.Now(),
				Remediation:  "Run: zpool scrub " + pool + " && zpool status -v " + pool,
				Status:       "detected",
			})
		}
	}

	// Check vdev error counters
	for _, line := range lines {
		fields := strings.Fields(line)
		// vdev lines: NAME STATE READ WRITE CKSUM
		if len(fields) >= 5 {
			read, write, cksum := fields[2], fields[3], fields[4]
			hasErr := (read != "0" && read != "READ") ||
				(write != "0" && write != "WRITE") ||
				(cksum != "0" && cksum != "CKSUM")
			if hasErr {
				vdev := fields[0]
				m.addEvent(&Event{
					ID:           generateID(),
					PoolName:     pool,
					FailureClass: ClassReadError,
					Severity:     "high",
					Message:      "Vdev error counters non-zero: " + vdev + " read=" + read + " write=" + write + " cksum=" + cksum,
					RawLine:      line,
					Timestamp:    time.Now(),
					DetectedAt:   time.Now(),
					Remediation:  "Run: zpool replace " + pool + " " + vdev,
					Status:       "detected",
				})
			}
		}
	}
}

func (m *Manager) checkKernelLog() {
	// Check /proc/spl/kstat/zfs/dbgmsg for ZFS kernel errors
	out, err := exec.Command("sh", "-c",
		"cat /proc/spl/kstat/zfs/dbgmsg 2>/dev/null | tail -50").Output()
	if err != nil {
		return
	}

	errorPatterns := []struct {
		pattern string
		class   FailureClass
		sev     string
		fix     string
	}{
		{"txg_sync", ClassTXGTimeout, "high", "Check pool I/O performance and vdev health"},
		{"vdev_disk_io_done", ClassVdevDegrade, "critical", "Check vdev hardware immediately"},
		{"arc_buf_destroy", ClassARCEviction, "medium", "Monitor ARC pressure, consider adding RAM"},
		{"zio_done: zio", ClassSilentCorruption, "critical", "Run zpool scrub immediately"},
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		for _, p := range errorPatterns {
			if strings.Contains(line, p.pattern) && strings.Contains(strings.ToLower(line), "error") {
				m.addEvent(&Event{
					ID:           generateID(),
					PoolName:     "system",
					FailureClass: p.class,
					Severity:     p.sev,
					Message:      "ZFS kernel log: " + strings.TrimSpace(line),
					RawLine:      line,
					Timestamp:    time.Now(),
					DetectedAt:   time.Now(),
					Remediation:  p.fix,
					Status:       "detected",
				})
				break
			}
		}
	}
}

func (m *Manager) checkARCStats() {
	out, err := exec.Command("sh", "-c",
		"cat /proc/spl/kstat/zfs/arcstats 2>/dev/null | grep -E 'misses|demand_data_misses|prefetch_data_misses'").Output()
	if err != nil {
		return
	}

	var misses, hits int64
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		var val int64
		for _, c := range fields[2] {
			if c >= '0' && c <= '9' {
				val = val*10 + int64(c-'0')
			}
		}
		if strings.HasPrefix(fields[0], "misses") {
			misses = val
		}
		if strings.HasPrefix(fields[0], "hits") {
			hits = val
		}
	}

	if hits+misses > 10000 {
		ratio := float64(hits) / float64(hits+misses)
		if ratio < 0.50 {
			m.addEvent(&Event{
				ID:           generateID(),
				PoolName:     "system",
				FailureClass: ClassARCEviction,
				Severity:     "high",
				Message:      "ARC hit ratio critically low — possible memory pressure or corruption thrashing",
				Timestamp:    time.Now(),
				DetectedAt:   time.Now(),
				Remediation:  "Check RAM availability, reduce ARC min size, inspect for checksum errors",
				Status:       "detected",
			})
		}
	}
}

func (m *Manager) addEvent(e *Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Dedup by message + pool within last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, existing := range m.events {
		if existing.PoolName == e.PoolName &&
			existing.Message == e.Message &&
			existing.DetectedAt.After(cutoff) {
			return
		}
	}
	if len(m.events) >= m.maxSize {
		m.events = m.events[1:]
	}
	m.events = append(m.events, e)
}

// GetEvents returns all events, newest first
func (m *Manager) GetEvents() []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Event, len(m.events))
	for i, e := range m.events {
		out[len(m.events)-1-i] = e
	}
	return out
}

// Acknowledge marks an event as acknowledged
func (m *Manager) Acknowledge(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.events {
		if e.ID == id {
			e.Status = "acknowledged"
			return true
		}
	}
	return false
}

func generateID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%(16+int64(i))]
	}
	return string(b)
}
