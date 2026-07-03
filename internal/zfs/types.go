package zfs

import "time"

// Pool represents a ZFS pool.
type Pool struct {
	Name        string       `json:"name"`
	State       string       `json:"state"` // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	Health      string       `json:"health"`
	Size        uint64       `json:"size"`
	Allocated   uint64       `json:"allocated"`
	Free        uint64       `json:"free"`
	Capacity    float64      `json:"capacity"` // 0-100
	ReadOps     uint64       `json:"read_ops"`
	WriteOps    uint64       `json:"write_ops"`
	ReadBytes   uint64       `json:"read_bytes"`
	WriteBytes  uint64       `json:"write_bytes"`
	Errors      uint64       `json:"errors"`
	ScrubStatus *ScrubStatus `json:"scrub_status,omitempty"`
	ScanStatus  *ScrubStatus `json:"scan_status,omitempty"`
	Vdevs       []Vdev       `json:"vdevs,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Vdev is a virtual device in a pool.
type Vdev struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Read     uint64 `json:"read"`
	Write    uint64 `json:"write"`
	Cksum    uint64 `json:"cksum"`
	Children []Vdev `json:"children,omitempty"`
}

// Dataset is a ZFS dataset, volume, or snapshot.
type Dataset struct {
	Name          string    `json:"name"`
	Pool          string    `json:"pool"`
	Type          string    `json:"type"`
	Used          uint64    `json:"used"`
	Available     uint64    `json:"available"`
	Referenced    uint64    `json:"referenced"`
	LogicalUsed   uint64    `json:"logical_used"`
	Mounted       bool      `json:"mounted"`
	MountPoint    string    `json:"mount_point"`
	Compression   string    `json:"compression"`
	CompressRatio float64   `json:"compress_ratio"`
	Dedup         bool      `json:"dedup"`
	Quota         uint64    `json:"quota"`
	Reservation   uint64    `json:"reservation"`
	VolSize       *uint64   `json:"vol_size,omitempty"`
	Encryption    string    `json:"encryption"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Snapshot is a ZFS snapshot.
type Snapshot struct {
	Name       string    `json:"name"`
	Dataset    string    `json:"dataset"`
	Pool       string    `json:"pool"`
	Used       uint64    `json:"used"`
	Referenced uint64    `json:"referenced"`
	CreatedAt  time.Time `json:"created_at"`
}

// SMARTData holds SMART health attributes for a physical drive.
type SMARTData struct {
	Device              string    `json:"device"`
	Model               string    `json:"model"`
	Serial              string    `json:"serial"`
	Health              string    `json:"health"`
	Temperature         int       `json:"temperature"`
	PowerOnHours        uint64    `json:"power_on_hours"`
	PowerCycles         uint64    `json:"power_cycles"`
	ReallocatedSectors  uint64    `json:"reallocated_sectors"`
	PendingSectors      uint64    `json:"pending_sectors"`
	UncorrectableErrors uint64    `json:"uncorrectable_errors"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ScrubStatus is the current scrub state of a pool.
type ScrubStatus struct {
	State         string     `json:"state"`
	Function      string     `json:"function"`
	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	Progress      float64    `json:"progress"`
	Examined      uint64     `json:"examined"`
	Total         uint64     `json:"total"`
	Errors        uint64     `json:"errors"`
	Duration      uint64     `json:"duration"`
	RemainingTime uint64     `json:"remaining_time"`
}

// ZFSData is the complete collected snapshot for one host.
type ZFSData struct {
	Pools       []Pool       `json:"pools"`
	Datasets    []*Dataset   `json:"datasets"`
	Snapshots   []*Snapshot  `json:"snapshots"`
	SMART       []*SMARTData `json:"smart,omitempty"`
	CollectedAt time.Time    `json:"collected_at"`
}

// Alert is a triggered monitoring alert.
type Alert struct {
	ID        string    `json:"id"`
	HostName  string    `json:"host_name"`
	PoolName  string    `json:"pool_name,omitempty"`
	Device    string    `json:"device,omitempty"`
	Severity  string    `json:"severity"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Triggered time.Time `json:"triggered"`
}
