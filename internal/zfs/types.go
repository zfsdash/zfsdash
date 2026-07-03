package zfs

import "time"

// Pool represents a ZFS pool.
type Pool struct {
	Name       string  `json:"name"`
	Size       uint64  `json:"size"`
	Alloc      uint64  `json:"alloc"`
	Free       uint64  `json:"free"`
	Health     string  `json:"health"`
	Capacity   float64 `json:"capacity"`
	Vdevs      []Vdev  `json:"vdevs"`
	Scrub      *Scrub  `json:"scrub,omitempty"`
}

// Vdev represents a virtual device in a pool.
type Vdev struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	State    string `json:"state"`
	Read     uint64 `json:"read_errors"`
	Write    uint64 `json:"write_errors"`
	Checksum uint64 `json:"checksum_errors"`
	Children []Vdev `json:"children,omitempty"`
}

// Scrub represents the last scrub status of a pool.
type Scrub struct {
	State     string    `json:"state"`
	Errors    uint64    `json:"errors"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Dataset represents a ZFS dataset or zvol.
type Dataset struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Used        uint64 `json:"used"`
	Avail       uint64 `json:"avail"`
	Refer       uint64 `json:"refer"`
	Mountpoint  string `json:"mountpoint"`
	Compression string `json:"compression"`
	CompressRatio float64 `json:"compress_ratio"`
	Encryption  string `json:"encryption"`
	Children    []Dataset `json:"children,omitempty"`
}

// Snapshot represents a ZFS snapshot.
type Snapshot struct {
	Name      string    `json:"name"`
	Used      uint64    `json:"used"`
	Refer     uint64    `json:"refer"`
	CreatedAt time.Time `json:"created_at"`
}

// Collector gathers ZFS data from a host.
type Collector interface {
	GetPools(ctx context.Context) ([]Pool, error)
	GetDatasets(ctx context.Context, pool string) ([]Dataset, error)
	GetSnapshots(ctx context.Context, dataset string) ([]Snapshot, error)
	CreateSnapshot(ctx context.Context, dataset, name string) error
	DestroySnapshot(ctx context.Context, snapshot string) error
	StartScrub(ctx context.Context, pool string) error
	Ping(ctx context.Context) error
}
