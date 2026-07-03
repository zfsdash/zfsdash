package store

import (
	"sync"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// HostEntry holds the latest collected state for a single host.
type HostEntry struct {
	Name        string          `json:"name"`
	LastUpdated time.Time       `json:"last_updated"`
	Error       string          `json:"error,omitempty"`
	PoolCount   int             `json:"pool_count"`
	Pools       []*zfs.Pool     `json:"-"`
	Datasets    []*zfs.Dataset  `json:"-"`
	Snapshots   []*zfs.Snapshot `json:"-"`
	SMARTData   []*zfs.SMARTData `json:"-"`
}

// Store is a thread-safe in-memory store for ZFS monitoring data.
type Store struct {
	mu    sync.RWMutex
	hosts map[string]*HostEntry
}

// New creates a new Store.
func New() *Store {
	return &Store{hosts: make(map[string]*HostEntry)}
}

func (s *Store) entry(name string) *HostEntry {
	if e, ok := s.hosts[name]; ok {
		return e
	}
	e := &HostEntry{Name: name}
	s.hosts[name] = e
	return e
}

// SetError records a collection error for a host.
func (s *Store) SetError(name, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(name)
	e.Error = errMsg
	e.LastUpdated = time.Now()
}

// SetPools updates the pool list for a host.
func (s *Store) SetPools(name string, pools []*zfs.Pool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(name)
	e.Pools = pools
	e.PoolCount = len(pools)
	e.LastUpdated = time.Now()
	e.Error = ""
}

// SetDatasets updates the dataset list for a host.
func (s *Store) SetDatasets(name string, ds []*zfs.Dataset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(name).Datasets = ds
}

// SetSnapshots updates the snapshot list for a host.
func (s *Store) SetSnapshots(name string, snaps []*zfs.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(name).Snapshots = snaps
}

// SetSMARTData updates SMART data for a host.
func (s *Store) SetSMARTData(name string, data []*zfs.SMARTData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(name).SMARTData = data
}

// GetHost returns the entry for a host (nil if not found).
func (s *Store) GetHost(name string) *HostEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hosts[name]
}

// ListHosts returns summary entries for all hosts.
func (s *Store) ListHosts() []*HostEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*HostEntry, 0, len(s.hosts))
	for _, e := range s.hosts {
		out = append(out, e)
	}
	return out
}
