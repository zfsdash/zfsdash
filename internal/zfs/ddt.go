package zfs

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type DDTStats struct {
	Pool          string  `json:"pool"`
	EntryCount    int64   `json:"entry_count"`
	DupRefs       int64   `json:"dup_refs"`
	UniqueRefs    int64   `json:"unique_refs"`
	MemoryEstMB   float64 `json:"memory_est_mb"`
	DedupRatio    float64 `json:"dedup_ratio"`
	Viable        bool    `json:"viable"`
	Recommendation string `json:"recommendation"`
}

var (
	ddtEntryRe = regexp.MustCompile(`DDT-sha256-zap-duplicate:\s+entries\s+=\s+(\d+)`)
	ddtUniqueRe = regexp.MustCompile(`DDT-sha256-zap-unique:\s+entries\s+=\s+(\d+)`)
)

func GetDDTStats(pool string) (*DDTStats, error) {
	out, err := exec.Command("zdb", "-D", pool).Output()
	if err != nil {
		return &DDTStats{Pool: pool, Recommendation: "zdb unavailable or no DDT"}, nil
	}
	s := string(out)
	stats := &DDTStats{Pool: pool}

	if m := ddtEntryRe.FindStringSubmatch(s); m != nil {
		stats.DupRefs, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if m := ddtUniqueRe.FindStringSubmatch(s); m != nil {
		stats.UniqueRefs, _ = strconv.ParseInt(m[1], 10, 64)
	}
	stats.EntryCount = stats.DupRefs + stats.UniqueRefs

	// ~320 bytes per DDT entry in ARC
	stats.MemoryEstMB = float64(stats.EntryCount) * 320.0 / 1024.0 / 1024.0

	// Get dedup ratio from zpool list
	dzOut, _ := exec.Command("zpool", "list", "-H", "-o", "dedupratio", pool).Output()
	ratioStr := strings.TrimSuffix(strings.TrimSpace(string(dzOut)), "x")
	stats.DedupRatio, _ = strconv.ParseFloat(ratioStr, 64)

	// Viability: ratio > 2.0 AND memory < 1GB
	stats.Viable = stats.DedupRatio >= 2.0 && stats.MemoryEstMB < 1024.0
	if stats.DedupRatio < 1.5 {
		stats.Recommendation = fmt.Sprintf("Dedup ratio %.1fx is too low — cost exceeds benefit. Consider disabling.", stats.DedupRatio)
	} else if stats.MemoryEstMB > 1024 {
		stats.Recommendation = fmt.Sprintf("DDT using ~%.0fMB of ARC. Consider adding RAM or disabling dedup on cold datasets.", stats.MemoryEstMB)
	} else {
		stats.Recommendation = fmt.Sprintf("Dedup is viable — ratio %.1fx, DDT size ~%.0fMB.", stats.DedupRatio, stats.MemoryEstMB)
	}
	return stats, nil
}
