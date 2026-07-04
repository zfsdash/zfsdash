package zfs

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SnapshotForensics struct {
	PoolName         string            `json:"pool_name"`
	TotalSnapshots   int               `json:"total_snapshots"`
	TotalSnapBytes   uint64            `json:"total_snap_bytes"`
	SnapPctOfPool    float64           `json:"snap_pct_of_pool"`
	TopSpaceHolders  []SnapshotDetail  `json:"top_space_holders"`
	DatasetBreakdown []DatasetSnapInfo `json:"dataset_breakdown"`
	OrphanedSnaps    []SnapshotDetail  `json:"orphaned_snapshots"`
	Alerts           []SnapAlert       `json:"alerts"`
	AnalyzedAt       time.Time         `json:"analyzed_at"`
}

type SnapshotDetail struct {
	Name             string    `json:"name"`
	Dataset          string    `json:"dataset"`
	SnapTag          string    `json:"snap_tag"`
	UsedBytes        uint64    `json:"used_bytes"`
	WrittenBytes     uint64    `json:"written_bytes"`
	ReferencedBytes  uint64    `json:"referenced_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	AgeHours         float64   `json:"age_hours"`
	IsOrphaned       bool      `json:"is_orphaned"`
}

type DatasetSnapInfo struct {
	Dataset          string  `json:"dataset"`
	DatasetUsed      uint64  `json:"dataset_used"`
	SnapshotUsed     uint64  `json:"snapshot_used"`
	SnapPctOfDataset float64 `json:"snap_pct_of_dataset"`
	SnapshotCount    int     `json:"snapshot_count"`
	OldestSnapDays   float64 `json:"oldest_snap_days"`
}

type SnapAlert struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

func GetSnapshotForensics(ctx context.Context, poolName string) (*SnapshotForensics, error) {
	sf := &SnapshotForensics{PoolName: poolName, AnalyzedAt: time.Now()}

	out, err := exec.CommandContext(ctx, "zfs", "list", "-Hrp",
		"-o", "name,used,written,refer,creation",
		"-t", "snapshot", poolName).Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list snapshots: %w", err)
	}

	datasetMap := make(map[string]*DatasetSnapInfo)
	var allSnaps []SnapshotDetail

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		parts := strings.SplitN(name, "@", 2)
		if len(parts) != 2 {
			continue
		}
		dataset := parts[0]
		snapTag := parts[1]

		used, _ := strconv.ParseUint(fields[1], 10, 64)
		written, _ := strconv.ParseUint(fields[2], 10, 64)
		refer, _ := strconv.ParseUint(fields[3], 10, 64)
		creationUnix, _ := strconv.ParseInt(fields[4], 10, 64)
		createdAt := time.Unix(creationUnix, 0)
		ageHours := time.Since(createdAt).Hours()

		snap := SnapshotDetail{
			Name:            name,
			Dataset:         dataset,
			SnapTag:         snapTag,
			UsedBytes:       used,
			WrittenBytes:    written,
			ReferencedBytes: refer,
			CreatedAt:       createdAt,
			AgeHours:        ageHours,
		}

		if ageHours > 90*24 &&
			!strings.Contains(snapTag, "daily") &&
			!strings.Contains(snapTag, "weekly") &&
			!strings.Contains(snapTag, "monthly") &&
			!strings.Contains(snapTag, "auto") &&
			!strings.Contains(snapTag, "sanoid") {
			snap.IsOrphaned = true
			sf.OrphanedSnaps = append(sf.OrphanedSnaps, snap)
		}

		allSnaps = append(allSnaps, snap)
		sf.TotalSnapBytes += used
		sf.TotalSnapshots++

		if _, ok := datasetMap[dataset]; !ok {
			datasetMap[dataset] = &DatasetSnapInfo{Dataset: dataset}
		}
		di := datasetMap[dataset]
		di.SnapshotUsed += used
		di.SnapshotCount++
		if ageHours/24 > di.OldestSnapDays {
			di.OldestSnapDays = ageHours / 24
		}
	}

	out2, _ := exec.CommandContext(ctx, "zfs", "list", "-Hrp",
		"-o", "name,used", "-t", "filesystem", poolName).Output()
	for _, line := range strings.Split(string(out2), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if di, ok := datasetMap[fields[0]]; ok {
			di.DatasetUsed, _ = strconv.ParseUint(fields[1], 10, 64)
			if di.DatasetUsed > 0 {
				di.SnapPctOfDataset = float64(di.SnapshotUsed) / float64(di.DatasetUsed) * 100
			}
		}
	}

	sort.Slice(allSnaps, func(i, j int) bool {
		return allSnaps[i].UsedBytes > allSnaps[j].UsedBytes
	})
	if len(allSnaps) > 20 {
		sf.TopSpaceHolders = allSnaps[:20]
	} else {
		sf.TopSpaceHolders = allSnaps
	}

	for _, di := range datasetMap {
		sf.DatasetBreakdown = append(sf.DatasetBreakdown, *di)
	}
	sort.Slice(sf.DatasetBreakdown, func(i, j int) bool {
		return sf.DatasetBreakdown[i].SnapshotUsed > sf.DatasetBreakdown[j].SnapshotUsed
	})

	poolOut, _ := exec.CommandContext(ctx, "zpool", "list", "-Hp", "-o", "size", poolName).Output()
	poolSize, _ := strconv.ParseUint(strings.TrimSpace(string(poolOut)), 10, 64)
	if poolSize > 0 {
		sf.SnapPctOfPool = float64(sf.TotalSnapBytes) / float64(poolSize) * 100
	}

	sf.Alerts = generateSnapAlerts(sf)
	return sf, nil
}

func generateSnapAlerts(sf *SnapshotForensics) []SnapAlert {
	var alerts []SnapAlert
	if sf.SnapPctOfPool > 30 {
		alerts = append(alerts, SnapAlert{
			Severity: "critical",
			Message:  fmt.Sprintf("Snapshots consuming %.1f%% of pool space", sf.SnapPctOfPool),
			Action:   "Review retention policies and destroy old snapshots",
		})
	} else if sf.SnapPctOfPool > 15 {
		alerts = append(alerts, SnapAlert{
			Severity: "warning",
			Message:  fmt.Sprintf("Snapshots consuming %.1f%% of pool space", sf.SnapPctOfPool),
			Action:   "Check snapshot retention policies",
		})
	}
	if len(sf.OrphanedSnaps) > 0 {
		alerts = append(alerts, SnapAlert{
			Severity: "warning",
			Message:  fmt.Sprintf("%d orphaned snapshots detected (>90 days, not in retention policy)", len(sf.OrphanedSnaps)),
			Action:   "Review and destroy orphaned snapshots",
		})
	}
	for _, di := range sf.DatasetBreakdown {
		if di.SnapPctOfDataset > 80 {
			alerts = append(alerts, SnapAlert{
				Severity: "warning",
				Message:  fmt.Sprintf("Dataset %s: snapshots using %.1f%% of dataset space", di.Dataset, di.SnapPctOfDataset),
				Action:   fmt.Sprintf("Reduce snapshot retention on %s", di.Dataset),
			})
		}
	}
	return alerts
}
