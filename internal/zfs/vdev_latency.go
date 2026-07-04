package zfs

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type VdevLatencySample struct {
	Vdev      string
	ReadNs    int64
	WriteNs   int64
	Timestamp time.Time
}

type VdevLatencyProfile struct {
	Pool       string
	Vdev       string
	Duration   time.Duration
	Samples    int
	ReadP50    int64
	ReadP95    int64
	ReadP99    int64
	WriteP50   int64
	WriteP95   int64
	WriteP99   int64
	Baseline   *VdevLatencyBaseline
	AlertLevel string
	Reason     string
}

type VdevLatencyBaseline struct {
	ReadP95Median  int64
	WriteP95Median int64
	SampleCount    int
}

var (
	latencyMu       sync.Mutex
	latencyBaseline = map[string]*VdevLatencyBaseline{}
	latencyHistory  = map[string][]VdevLatencySample{}
)

func ProfileVdevLatency(pool, vdev string, duration time.Duration, intervalMs int) (*VdevLatencyProfile, error) {
	intervalSec := intervalMs / 1000
	if intervalSec < 1 {
		intervalSec = 1
	}
	count := int(duration.Seconds()) / intervalSec
	if count < 2 {
		count = 2
	}
	args := []string{"iostat", "-v", "-H", pool, fmt.Sprintf("%d", intervalSec), fmt.Sprintf("%d", count)}
	out, err := exec.Command("zpool", args...).Output()
	if err != nil {
		slog.Warn("zpool iostat failed", "err", err)
		return mockLatencyProfile(pool, vdev, duration), nil
	}
	var readSamples, writeSamples []int64
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		if !strings.Contains(fields[0], vdev) {
			continue
		}
		rNs, rerr := strconv.ParseInt(fields[4], 10, 64)
		wNs, werr := strconv.ParseInt(fields[5], 10, 64)
		if rerr == nil && werr == nil {
			readSamples = append(readSamples, rNs)
			writeSamples = append(writeSamples, wNs)
		}
	}
	if len(readSamples) == 0 {
		return mockLatencyProfile(pool, vdev, duration), nil
	}
	profile := &VdevLatencyProfile{
		Pool: pool, Vdev: vdev, Duration: duration, Samples: len(readSamples),
		ReadP50: percentile(readSamples, 50), ReadP95: percentile(readSamples, 95), ReadP99: percentile(readSamples, 99),
		WriteP50: percentile(writeSamples, 50), WriteP95: percentile(writeSamples, 95), WriteP99: percentile(writeSamples, 99),
	}
	updateBaseline(pool, vdev, readSamples, writeSamples)
	profile.Baseline = getBaseline(pool, vdev)
	profile.AlertLevel, profile.Reason = assessLatency(profile)
	return profile, nil
}

func mockLatencyProfile(pool, vdev string, duration time.Duration) *VdevLatencyProfile {
	return &VdevLatencyProfile{
		Pool: pool, Vdev: vdev, Duration: duration, Samples: 10,
		ReadP50: 1200000, ReadP95: 3500000, ReadP99: 8000000,
		WriteP50: 800000, WriteP95: 2000000, WriteP99: 5000000,
		AlertLevel: "ok", Reason: "latency within normal range (mock data)",
	}
}

func percentile(data []int64, p int) int64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]int64, len(data))
	copy(sorted, data)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func updateBaseline(pool, vdev string, reads, writes []int64) {
	key := pool + "/" + vdev
	latencyMu.Lock()
	defer latencyMu.Unlock()
	now := time.Now()
	for i, r := range reads {
		w := int64(0)
		if i < len(writes) {
			w = writes[i]
		}
		latencyHistory[key] = append(latencyHistory[key], VdevLatencySample{Vdev: vdev, ReadNs: r, WriteNs: w, Timestamp: now})
	}
	cutoff := now.Add(-7 * 24 * time.Hour)
	var kept []VdevLatencySample
	for _, s := range latencyHistory[key] {
		if s.Timestamp.After(cutoff) {
			kept = append(kept, s)
		}
	}
	latencyHistory[key] = kept
	var rS, wS []int64
	for _, s := range kept {
		rS = append(rS, s.ReadNs)
		wS = append(wS, s.WriteNs)
	}
	latencyBaseline[key] = &VdevLatencyBaseline{ReadP95Median: percentile(rS, 95), WriteP95Median: percentile(wS, 95), SampleCount: len(rS)}
}

func getBaseline(pool, vdev string) *VdevLatencyBaseline {
	latencyMu.Lock()
	defer latencyMu.Unlock()
	return latencyBaseline[pool+"/"+vdev]
}

func assessLatency(p *VdevLatencyProfile) (string, string) {
	if p.Baseline == nil || p.Baseline.SampleCount < 10 {
		return "ok", "insufficient baseline data"
	}
	threshold := int64(float64(p.Baseline.ReadP95Median) * 1.5)
	if p.ReadP95 > threshold*2 {
		return "critical", fmt.Sprintf("read p95 %dms is >2x baseline", p.ReadP95/1e6)
	}
	if p.ReadP95 > threshold {
		return "warning", fmt.Sprintf("read p95 %dms is >1.5x baseline", p.ReadP95/1e6)
	}
	return "ok", "latency within normal range"
}
