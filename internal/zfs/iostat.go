package zfs

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// IOStats holds per-pool I/O statistics.
type IOStats struct {
	Pool       string  `json:"pool"`
	ReadOps    float64 `json:"read_ops_per_sec"`
	WriteOps   float64 `json:"write_ops_per_sec"`
	ReadBytes  float64 `json:"read_bytes_per_sec"`
	WriteBytes float64 `json:"write_bytes_per_sec"`
}

// GetPoolIOStats runs zpool iostat for a single pool and returns stats.
func GetPoolIOStats(pool string) (*IOStats, error) {
	out, err := exec.Command("zpool", "iostat", "-Hp", pool, "1", "1").Output()
	if err != nil {
		return &IOStats{Pool: pool}, nil
	}
	stats := &IOStats{Pool: pool}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 7 && fields[0] == pool {
			stats.ReadOps, _ = strconv.ParseFloat(fields[3], 64)
			stats.WriteOps, _ = strconv.ParseFloat(fields[4], 64)
			stats.ReadBytes, _ = strconv.ParseFloat(fields[5], 64)
			stats.WriteBytes, _ = strconv.ParseFloat(fields[6], 64)
			break
		}
	}
	return stats, nil
}

// GetScrubStatusJSON returns JSON bytes describing the current scrub state.
func GetScrubStatusJSON(pool string) ([]byte, error) {
	out, err := exec.Command("zpool", "status", "-v", pool).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "scan:") {
			return json.Marshal(map[string]string{"pool": pool, "scan": strings.TrimPrefix(line, "scan: ")})
		}
	}
	return json.Marshal(map[string]string{"pool": pool, "scan": "none"})
}
