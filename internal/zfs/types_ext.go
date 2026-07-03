package zfs

import "time"

// CollectorConfig holds configuration for creating a ZFS collector.
// Used by NewSSHCollector and NewTrueNASCollector.
type CollectorConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	SSHKey   string // path to private key file, or PEM contents
	APIKey   string // TrueNAS API key
	Timeout  int    // seconds; 0 means use default (30s)
}

// SMARTData holds the SMART health status for a single block device.
type SMARTData struct {
	Device             string    `json:"device"`
	HealthStatus       string    `json:"health_status"` // PASSED | FAILED | unknown
	Temperature        int       `json:"temperature_celsius,omitempty"`
	ReallocatedSectors uint64    `json:"reallocated_sectors,omitempty"`
	PowerOnHours       uint64    `json:"power_on_hours,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// Pool is the internal representation of a ZFS pool collected from any source.
// Note: local.go and truenas.go use this struct directly with these field names.
type Pool struct {
	Name      string  `json:"name"`
	Health    string  `json:"health"`
	Size      int64   `json:"size_bytes"`
	Allocated int64   `json:"allocated_bytes"`
	Free      int64   `json:"free_bytes"`
	Capacity  int     `json:"capacity_pct"`
	VdevTree  *Vdev   `json:"vdev_tree,omitempty"`
	Scrub     *Scrub  `json:"scrub,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Dataset is the internal representation of a ZFS dataset or zvol.
type Dataset struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"` // filesystem | volume | snapshot
	Used       int64     `json:"used_bytes"`
	Available  int64     `json:"available_bytes"`
	Referenced int64     `json:"referenced_bytes"`
	Mountpoint string    `json:"mountpoint,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Snapshot is the internal representation of a ZFS snapshot.
type Snapshot struct {
	Name       string    `json:"name"`
	Pool       string    `json:"pool"`
	Dataset    string    `json:"dataset"`
	Used       int64     `json:"used_bytes"`
	Referenced int64     `json:"referenced_bytes"`
	CreatedAt  time.Time `json:"created_at"`
	Timestamp  time.Time `json:"timestamp"`
}

// Vdev is a virtual device node in the ZFS pool vdev tree.
type Vdev struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	State    string  `json:"state"`
	Read     int64   `json:"read_errors"`
	Write    int64   `json:"write_errors"`
	Checksum int64   `json:"checksum_errors"`
	Children []*Vdev `json:"children,omitempty"`
}

// Scrub holds the most recent scrub status for a pool.
type Scrub struct {
	State  string `json:"state"`  // none | scanning | paused | finished
	Errors uint64 `json:"errors"`
}
