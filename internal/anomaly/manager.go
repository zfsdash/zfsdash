package anomaly

import (
	"sync"
	"time"
)

// Manager holds one Detector per pool.
type Manager struct {
	mu        sync.RWMutex
	detectors map[string]*Detector
}

// NewManager creates a manager.
func NewManager() *Manager {
	return &Manager{
		detectors: make(map[string]*Detector),
	}
}

// EnsurePool creates a detector for pool if it doesn't exist.
func (m *Manager) EnsurePool(pool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.detectors[pool]; !ok {
		m.detectors[pool] = NewDetector(pool, 40, 2.0)
	}
}

// Feed feeds an ARC measurement for a pool and returns any anomaly.
func (m *Manager) Feed(pool string, hitRatio float64, sizeBytes uint64) *AnomalyEvent {
	m.mu.RLock()
	d, ok := m.detectors[pool]
	m.mu.RUnlock()
	if !ok {
		m.EnsurePool(pool)
		m.mu.RLock()
		d = m.detectors[pool]
		m.mu.RUnlock()
	}
	return d.Record(ARCPoint{
		Timestamp: time.Now(),
		HitRatio:  hitRatio,
		SizeBytes: sizeBytes,
	})
}

// AllEvents returns all anomaly events across all pools, newest first.
func (m *Manager) AllEvents() []AnomalyEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []AnomalyEvent
	for _, d := range m.detectors {
		out = append(out, d.Events()...)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// PoolEvents returns anomaly events for a single pool.
func (m *Manager) PoolEvents(pool string) []AnomalyEvent {
	m.mu.RLock()
	d, ok := m.detectors[pool]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return d.Events()
}

// PoolStats returns baseline stats for a pool.
func (m *Manager) PoolStats(pool string) (mean, stddev float64, samples int) {
	m.mu.RLock()
	d, ok := m.detectors[pool]
	m.mu.RUnlock()
	if !ok {
		return 0, 0, 0
	}
	return d.Stats()
}
