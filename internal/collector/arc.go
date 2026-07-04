package collector

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// ARCStats represents ARC (Adaptive Replacement Cache) statistics from ZFS.
type ARCStats struct {
	Size         uint64    `json:"size"`
	MaxSize      uint64    `json:"max_size"`
	Hits         uint64    `json:"hits"`
	Misses       uint64    `json:"misses"`
	HitRatio     float64   `json:"hit_ratio"`
	DataSize     uint64    `json:"data_size"`
	MetadataSize uint64    `json:"metadata_size"`
	CollectedAt  time.Time `json:"collected_at"`
}

// ReadARCStats reads ARC statistics from /proc/spl/kstat/zfs/arcstats.
// Returns zero-valued struct on non-Linux or if file is unavailable.
func ReadARCStats() *ARCStats {
	stats := &ARCStats{CollectedAt: time.Now()}

	f, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		// Graceful fallback: not on Linux, or ZFS not loaded
		return stats
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		// Format: name type value
		// Example: size 4 1234567890
		if len(parts) < 3 {
			continue
		}

		key := parts[0]
		val, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			continue
		}

		switch key {
		case "size":
			stats.Size = val
		case "c_max":
			stats.MaxSize = val
		case "hits":
			stats.Hits = val
		case "misses":
			stats.Misses = val
		case "data_size":
			stats.DataSize = val
		case "mru_metadata_size", "mfu_metadata_size":
			stats.MetadataSize += val
		}
	}

	// Calculate hit ratio
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRatio = float64(stats.Hits) / float64(total)
	}

	return stats
}
