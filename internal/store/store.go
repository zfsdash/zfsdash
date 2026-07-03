package store

import (
	"sync"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// HostData holds all ZFS data for a single host
type HostData struct {
	Name        string
	Pools       []*zfs.Pool
	Datasets    map[string][]*zfs.Dataset  // pool -> datasets
	Snapshots   map[string][]*zfs.Snapshot // dataset -> snapshots
	SMARTData   map[string]*zfs.SMARTData
	LastUpdated time.Time
	Error       string
}

// Store is a thread-safe in-memory store for ZFS host data
type Store struct {
	mu    sync.RWMutex
	hosts map[string]*HostData
}

// New creates a new Store
func New() *Store {
	return &Store{
		hosts: make(map[string]*HostData),
	}
}

// SetHostData updates all data for a host atomically
func (s *Store) SetHostData(name string, data *HostData) {
	data.Name = name
	data.LastUpdated = time.Now()
	s.mu.Lock()
	s.hosts[name] = data
	s.mu.Unlock()
}

// SetHostError records an error for a host
func (s *Store) SetHostError(name, errMsg string) {
	s.mu.Lock()
	if existing, ok := s.hosts[name]; ok {
		existing.Error = errMsg
		existing.LastUpdated = time.Now()
	} else {
		s.hosts[name] = &HostData{
			Name:        name,
			Error:       errMsg,
			LastUpdated: time.Now(),
		}
	}
	s.mu.Unlock()
}

// GetHost returns data for a specific host
func (s *Store) GetHost(name string) (*HostData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.hosts[name]
	return d, ok
}

// GetAllHosts returns a summary of all hosts
func (s *Store) GetAllHosts() []*HostData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hosts := make([]*HostData, 0, len(s.hosts))
	for _, h := range s.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

// ListHostNames returns all known host names
func (s *Store) ListHostNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.hosts))
	for name := range s.hosts {
		names = append(names, name)
	}
	return names
}
