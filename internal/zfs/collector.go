package zfs

import "context"

// Collector is the interface every ZFS backend must implement.
type Collector interface {
	GetPools(ctx context.Context) ([]*Pool, error)
	GetDatasets(ctx context.Context, pool string) ([]*Dataset, error)
	GetSnapshots(ctx context.Context, dataset string) ([]*Snapshot, error)
	GetSMARTData(ctx context.Context) ([]*SMARTData, error)
	CreateSnapshot(ctx context.Context, dataset, snapName string) error
	DeleteSnapshot(ctx context.Context, fullName string) error
	StartScrub(ctx context.Context, pool string) error
	StopScrub(ctx context.Context, pool string) error
	Close() error
}
