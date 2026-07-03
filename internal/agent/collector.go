package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// LocalCollectFn builds a telemetry collect function backed by a
// zfs.LocalCollector. This is the bridge between the agent and the
// existing ZFS collection layer.
func LocalCollectFn(lc *zfs.LocalCollector) func(ctx context.Context) (*TelemetryPayload, error) {
	return func(ctx context.Context) (*TelemetryPayload, error) {
		payload := &TelemetryPayload{
			Timestamp: time.Now().UTC(),
		}

		// Collect pools.
		pools, err := lc.CollectPools(ctx)
		if err != nil {
			slog.Warn("agent: collect pools", "err", err)
			// Return empty payload rather than error — let the loop keep going.
			return payload, nil
		}

		for _, p := range pools {
			ps := PoolSummary{
				Name:     p.Name,
				Health:   p.Health,
				Used:     uint64(p.Allocated),
				Total:    uint64(p.Size),
				Free:     uint64(p.Free),
				Capacity: float64(p.Capacity),
			}

			// Scrub status.
			if scrub, err := lc.CollectScrubStatus(ctx, p.Name); err == nil && scrub != nil {
				ps.Scrub = &ScrubSummary{
					State:  scrub.State,
					Errors: scrub.Errors,
				}
			}

			// Datasets.
			datasets, err := lc.CollectDatasets(ctx, p.Name)
			if err != nil {
				slog.Warn("agent: collect datasets", "pool", p.Name, "err", err)
			} else {
				for _, d := range datasets {
					ps.Datasets = append(ps.Datasets, DatasetSummary{
						Name:  d.Name,
						Type:  d.Type,
						Used:  uint64(d.Used),
						Avail: uint64(d.Available),
						Refer: uint64(d.Referenced),
					})
				}
			}

			payload.Pools = append(payload.Pools, ps)
		}

		// SMART data.
		smartMap, err := lc.CollectSMARTData(ctx)
		if err != nil {
			slog.Warn("agent: collect smart", "err", err)
		} else {
			for _, sd := range smartMap {
				payload.SMART = append(payload.SMART, SMARTSummary{
					Device:             sd.Device,
					Health:             sd.HealthStatus,
					Temp:               sd.Temperature,
					ReallocatedSectors: sd.ReallocatedSectors,
				})
			}
		}

		return payload, nil
	}
}
