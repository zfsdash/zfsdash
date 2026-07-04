package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PoolIOStats represents I/O statistics for a ZFS pool.
type PoolIOStats struct {
	ReadOps    uint64 `json:"read_ops"`
	WriteOps   uint64 `json:"write_ops"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
}

// ReadPoolIOStats runs 'zpool iostat -Hp <pool>' and returns I/O statistics.
// Returns zero-valued struct if zpool command fails or pool not found.
func ReadPoolIOStats(pool string) *PoolIOStats {
	stats := &PoolIOStats{}

	if pool == "" {
		return stats
	}

	cmd := exec.Command("zpool", "iostat", "-Hp", pool)
	out, err := cmd.Output()
	if err != nil {
		return stats
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return stats
	}

	lines := strings.Split(line, "\n")
	if len(lines) == 0 {
		return stats
	}

	parts := strings.Fields(lines[0])
	// Expected: pool alloc free read_ops write_ops read_bytes write_bytes
	if len(parts) < 7 {
		return stats
	}

	if ro, err := parseSizeStr(parts[3]); err == nil {
		stats.ReadOps = ro
	}
	if wo, err := parseSizeStr(parts[4]); err == nil {
		stats.WriteOps = wo
	}
	if rb, err := parseSizeStr(parts[5]); err == nil {
		stats.ReadBytes = rb
	}
	if wb, err := parseSizeStr(parts[6]); err == nil {
		stats.WriteBytes = wb
	}

	return stats
}

// parseSizeStr converts a size string (with optional unit suffix) to uint64 bytes.
func parseSizeStr(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "-" {
		return 0, nil
	}

	if val, err := strconv.ParseUint(s, 10, 64); err == nil {
		return val, nil
	}

	if len(s) < 2 {
		return 0, fmt.Errorf("invalid size: %s", s)
	}

	lastChar := s[len(s)-1]
	numPart := s[:len(s)-1]

	val, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}

	multiplier := uint64(1)
	switch lastChar {
	case 'K', 'k':
		multiplier = 1024
	case 'M', 'm':
		multiplier = 1024 * 1024
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
	case 'T', 't':
		multiplier = 1024 * 1024 * 1024 * 1024
	case 'P', 'p':
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return strconv.ParseUint(s, 10, 64)
	}

	return uint64(val * float64(multiplier)), nil
}
