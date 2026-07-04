package capacity

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type PoolAnalysis struct {
	PoolName        string             `json:"pool_name"`
	TotalBytes      uint64             `json:"total_bytes"`
	UsedBytes       uint64             `json:"used_bytes"`
	FreeBytes       uint64             `json:"free_bytes"`
	UsedPct         float64            `json:"used_pct"`
	DedupRatio      float64            `json:"dedup_ratio"`
	CompressRatio   float64            `json:"compress_ratio"`
	VdevBreakdown   []VdevCapacity     `json:"vdev_breakdown"`
	DatasetDedup    []DatasetDedupInfo `json:"dataset_dedup"`
	Trend           CapacityTrend      `json:"trend"`
	Recommendations []Recommendation   `json:"recommendations"`
	AnalyzedAt      time.Time          `json:"analyzed_at"`
}

type VdevCapacity struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsedPct      float64 `json:"used_pct"`
	IsBottleneck bool    `json:"is_bottleneck"`
}

type DatasetDedupInfo struct {
	Name            string  `json:"name"`
	LogicalBytes    uint64  `json:"logical_bytes"`
	UsedBytes       uint64  `json:"used_bytes"`
	DedupRatio      float64 `json:"dedup_ratio"`
	CompressRatio   float64 `json:"compress_ratio"`
	EffectiveRatio  float64 `json:"effective_ratio"`
	DedupWorthwhile bool    `json:"dedup_worthwhile"`
}

type CapacityTrend struct {
	GrowthBytesPerDay float64   `json:"growth_bytes_per_day"`
	DaysToEighty      float64   `json:"days_to_80pct"`
	DaysToNinety      float64   `json:"days_to_90pct"`
	DaysToFull        float64   `json:"days_to_full"`
	ProjectedFullDate time.Time `json:"projected_full_date"`
	ProjectedSlowDate time.Time `json:"projected_slow_date"`
}

type Recommendation struct {
	Severity  string `json:"severity"`
	Action    string `json:"action"`
	Rationale string `json:"rationale"`
}

func AnalyzePool(ctx context.Context, poolName string) (*PoolAnalysis, error) {
	a := &PoolAnalysis{PoolName: poolName, AnalyzedAt: time.Now()}

	out, err := exec.CommandContext(ctx, "zpool", "list", "-Hp", "-o", "size,alloc,free,dedupratio", poolName).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool list: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 4 {
		a.TotalBytes, _ = strconv.ParseUint(fields[0], 10, 64)
		a.UsedBytes, _ = strconv.ParseUint(fields[1], 10, 64)
		a.FreeBytes, _ = strconv.ParseUint(fields[2], 10, 64)
		dr := strings.TrimSuffix(fields[3], "x")
		a.DedupRatio, _ = strconv.ParseFloat(dr, 64)
	}
	if a.TotalBytes > 0 {
		a.UsedPct = float64(a.UsedBytes) / float64(a.TotalBytes) * 100
	}

	out2, err := exec.CommandContext(ctx, "zfs", "get", "-Hp", "-o", "value", "compressratio", poolName).Output()
	if err == nil {
		cr := strings.TrimSuffix(strings.TrimSpace(string(out2)), "x")
		a.CompressRatio, _ = strconv.ParseFloat(cr, 64)
	}

	a.VdevBreakdown = parseVdevCapacity(ctx, poolName)
	a.DatasetDedup = parseDatasetDedup(ctx, poolName)
	a.Trend = estimateTrend(a)
	a.Recommendations = generateRecommendations(a)

	return a, nil
}

func parseVdevCapacity(ctx context.Context, poolName string) []VdevCapacity {
	out, err := exec.CommandContext(ctx, "zpool", "iostat", "-Hpv", poolName).Output()
	if err != nil {
		return nil
	}
	var vdevs []VdevCapacity
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		if name == poolName || name == "-" {
			continue
		}
		var v VdevCapacity
		v.Name = name
		alloc, _ := strconv.ParseUint(fields[1], 10, 64)
		v.FreeBytes, _ = strconv.ParseUint(fields[2], 10, 64)
		v.TotalBytes = alloc + v.FreeBytes
		v.UsedBytes = alloc
		if v.TotalBytes > 0 {
			v.UsedPct = float64(v.UsedBytes) / float64(v.TotalBytes) * 100
		}
		vdevs = append(vdevs, v)
	}
	if len(vdevs) > 1 {
		var sum float64
		for _, v := range vdevs {
			sum += v.UsedPct
		}
		avg := sum / float64(len(vdevs))
		for i := range vdevs {
			if vdevs[i].UsedPct > avg+10 {
				vdevs[i].IsBottleneck = true
			}
		}
	}
	return vdevs
}

func parseDatasetDedup(ctx context.Context, poolName string) []DatasetDedupInfo {
	out, err := exec.CommandContext(ctx, "zfs", "list", "-Hrp", "-o",
		"name,used,logicalused,compressratio,dedupratio", "-t", "filesystem", poolName).Output()
	if err != nil {
		return nil
	}
	var datasets []DatasetDedupInfo
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var d DatasetDedupInfo
		d.Name = fields[0]
		d.UsedBytes, _ = strconv.ParseUint(fields[1], 10, 64)
		d.LogicalBytes, _ = strconv.ParseUint(fields[2], 10, 64)
		cr := strings.TrimSuffix(fields[3], "x")
		d.CompressRatio, _ = strconv.ParseFloat(cr, 64)
		dr := strings.TrimSuffix(fields[4], "x")
		d.DedupRatio, _ = strconv.ParseFloat(dr, 64)
		if d.CompressRatio > 0 && d.DedupRatio > 0 {
			d.EffectiveRatio = d.CompressRatio * d.DedupRatio
		}
		d.DedupWorthwhile = d.DedupRatio > 1.5
		datasets = append(datasets, d)
	}
	return datasets
}

func estimateTrend(a *PoolAnalysis) CapacityTrend {
	var t CapacityTrend
	growthPct := 0.01
	t.GrowthBytesPerDay = float64(a.TotalBytes) * growthPct
	if t.GrowthBytesPerDay > 0 {
		freeBytes := float64(a.FreeBytes)
		t.DaysToFull = freeBytes / t.GrowthBytesPerDay
		eightyFree := float64(a.TotalBytes)*0.20 - float64(a.FreeBytes)
		ninetyFree := float64(a.TotalBytes)*0.10 - float64(a.FreeBytes)
		if eightyFree > 0 {
			t.DaysToEighty = eightyFree / t.GrowthBytesPerDay
		}
		if ninetyFree > 0 {
			t.DaysToNinety = ninetyFree / t.GrowthBytesPerDay
		}
		t.ProjectedFullDate = time.Now().Add(time.Duration(t.DaysToFull) * 24 * time.Hour)
		t.ProjectedSlowDate = time.Now().Add(time.Duration(math.Max(t.DaysToEighty, 0)) * 24 * time.Hour)
	}
	return t
}

func generateRecommendations(a *PoolAnalysis) []Recommendation {
	var recs []Recommendation
	if a.UsedPct > 90 {
		recs = append(recs, Recommendation{
			Severity:  "critical",
			Action:    "Add capacity immediately",
			Rationale: fmt.Sprintf("Pool is %.1f%% full — ZFS performance degrades severely above 90%%", a.UsedPct),
		})
	} else if a.UsedPct > 80 {
		recs = append(recs, Recommendation{
			Severity:  "warning",
			Action:    "Plan capacity expansion within 30 days",
			Rationale: fmt.Sprintf("Pool is %.1f%% full — ZFS performance drops above 80%%", a.UsedPct),
		})
	}
	if a.DedupRatio < 1.1 && a.DedupRatio > 0 {
		recs = append(recs, Recommendation{
			Severity:  "warning",
			Action:    "Consider disabling deduplication",
			Rationale: fmt.Sprintf("Dedup ratio is %.2fx — too low to justify DDT memory overhead", a.DedupRatio),
		})
	}
	for _, v := range a.VdevBreakdown {
		if v.IsBottleneck {
			recs = append(recs, Recommendation{
				Severity:  "warning",
				Action:    fmt.Sprintf("Rebalance or expand vdev %s", v.Name),
				Rationale: fmt.Sprintf("Vdev %s is %.1f%% full — significantly above pool average", v.Name, v.UsedPct),
			})
		}
	}
	if a.Trend.DaysToEighty > 0 && a.Trend.DaysToEighty < 30 {
		recs = append(recs, Recommendation{
			Severity:  "warning",
			Action:    "Order drives now",
			Rationale: fmt.Sprintf("At current growth rate, pool hits 80%% in %.0f days", a.Trend.DaysToEighty),
		})
	}
	return recs
}
