package zfs

import (
	"context"
	"time"
)

// Pool represents a ZFS storage pool
type Pool struct {
	Name      string    `json:"name"`
	Health    string    `json:"health"`
	Size      int64     `json:"size"`
	Allocated int64     `json:"allocated"`
	Free      int64     `json:"free"`
	Capacity  int       `json:"capacity"` // percentage 0-100
	Datasets  int       `json:"datasets"`
	Snapshots int       `json:"snapshots"`
	Scrub     *Scrub    `json:"scrub,omitempty"`
	VdevTree  *Vdev     `json:"vdev_tree,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Dataset represents a ZFS filesystem or volume
type Dataset struct {
	Name       string    `json:"name"`
	Used       int64     `json:"used"`
	Available  int64     `json:"available"`
	Referenced int64     `json:"referenced"`
	Mountpoint string    `json:"mountpoint"`
	Type       string    `json:"type"` // filesystem, volume, snapshot
	Timestamp  time.Time `json:"timestamp"`
}

// Snapshot represents a ZFS snapshot
type Snapshot struct {
	Name       string    `json:"name"`
	Pool       string    `json:"pool"`
	Dataset    string    `json:"dataset"`
	Used       int64     `json:"used"`
	Referenced int64     `json:"referenced"`
	CreatedAt  time.Time `json:"created_at"`
	Timestamp  time.Time `json:"timestamp"`
}

// Scrub represents ZFS pool scrub status
type Scrub struct {
	State       string    `json:"state"` // none, scanning, paused, finished
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Progress    int       `json:"progress"` // percentage 0-100
	Rate        string    `json:"rate"`
	Errors      int       `json:"errors"`
	DurationSec int64     `json:"duration_sec"`
}

// Vdev represents a ZFS virtual device in a pool
type Vdev struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"` // disk, mirror, raidz1, raidz2, raidz3
	State    string  `json:"state"`
	Read     int64   `json:"read_errors"`
	Write    int64   `json:"write_errors"`
	Checksum int64   `json:"checksum_errors"`
	Children []*Vdev `json:"children,omitempty"`
}

// SMARTData represents S.M.A.R.T. data for a disk
type SMARTData struct {
	Device             string    `json:"device"`
	ModelName          string    `json:"model_name"`
	SerialNumber       string    `json:"serial_number"`
	Temperature        int       `json:"temperature"`
	PowerOnHours       int       `json:"power_on_hours"`
	ReallocatedSectors int       `json:"reallocated_sectors"`
	PendingSectors     int       `json:"pending_sectors"`
	HealthStatus       string    `json:"health_status"` // passed, failed, unknown
	Timestamp          time.Time `json:"timestamp"`
}

// PoolSnapshot records pool state at a point in time for history
type PoolSnapshot struct {
	ID        int64     `json:"id"`
	PoolName  string    `json:"pool_name"`
	Size      int64     `json:"size"`
	Allocated int64     `json:"allocated"`
	Free      int64     `json:"free"`
	Capacity  int       `json:"capacity"`
	Health    string    `json:"health"`
	CreatedAt time.Time `json:"created_at"`
}

// ScrubHistory records historical scrub information
type ScrubHistory struct {
	ID        int64     `json:"id"`
	PoolName  string    `json:"pool_name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int64     `json:"duration_sec"`
	Errors    int       `json:"errors"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

// CollectorConfig holds configuration for a host
type CollectorConfig struct {
	Mode     string `yaml:"mode"` // local, ssh, truenas
	Hostname string `yaml:"hostname"`
	Host     string `yaml:"host,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	SSHKey   string `yaml:"ssh_key,omitempty"`
	Timeout  int    `yaml:"timeout,omitempty"`
}

// Collector interface defines ZFS data collection
type Collector interface {
	CollectPools(ctx context.Context) ([]*Pool, error)
	CollectDatasets(ctx context.Context, poolName string) ([]*Dataset, error)
	CollectSnapshots(ctx context.Context, datasetName string) ([]*Snapshot, error)
	CollectScrubStatus(ctx context.Context, poolName string) (*Scrub, error)
	CollectVdevTree(ctx context.Context, poolName string) (*Vdev, error)
	CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error)
	CreateSnapshot(ctx context.Context, datasetName, snapshotName string) error
	DestroySnapshot(ctx context.Context, snapshotName string) error
	StartScrub(ctx context.Context, poolName string) error
	Close() error
}
