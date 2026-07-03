package zfs

import "context"

// Collector is the interface that all ZFS data sources (local, SSH, TrueNAS)
// must implement. It is defined here to avoid import cycles.
type Collector interface {
	CollectPools(ctx context.Context) ([]*Pool, error)
	CollectDatasets(ctx context.Context, pool string) ([]*Dataset, error)
	CollectSnapshots(ctx context.Context, dataset string) ([]*Snapshot, error)
	CollectScrubStatus(ctx context.Context, pool string) (*Scrub, error)
	CollectVdevTree(ctx context.Context, pool string) (*Vdev, error)
	CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error)
	CreateSnapshot(ctx context.Context, dataset, name string) error
	DestroySnapshot(ctx context.Context, snapshot string) error
	StartScrub(ctx context.Context, pool string) error
	Close() error
}
