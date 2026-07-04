package attribution

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IOProfile represents per-workload I/O accounting
type IOProfile struct {
	VdevID         string
	WorkloadTag    string
	OpType         string
	Count          int64
	TotalLatencyUs int64
	MaxLatencyUs   int64
	BytesTotal     int64
	LastUpdateUnix int64
}

func (p *IOProfile) AvgLatencyUs() float64 {
	if p.Count == 0 {
		return 0
	}
	return float64(p.TotalLatencyUs) / float64(p.Count)
}

// Attributor correlates dataset I/O with vdev performance
type Attributor struct {
	mu       sync.RWMutex
	profiles map[string]map[string]*IOProfile
	history  []*AttributionSnapshot
	maxHist  int
}

// AttributionSnapshot is a point-in-time snapshot
type AttributionSnapshot struct {
	Timestamp time.Time
	Profiles  []*IOProfile
}

// DatasetIOSample is raw I/O data for a dataset
type DatasetIOSample struct {
	Dataset    string
	Pool       string
	NRead      int64
	NWrite     int64
	ReadBytes  int64
	WriteBytes int64
	ReadTime   int64
	WriteTime  int64
	SampledAt  time.Time
}

// VdevSample is raw per-vdev data
type VdevSample struct {
	VdevID    string
	Pool      string
	ReadOps   int64
	WriteOps  int64
	ReadBW    int64
	WriteBW   int64
	SampledAt time.Time
}

// New creates a new Attributor
func New() *Attributor {
	return &Attributor{
		profiles: make(map[string]map[string]*IOProfile),
		maxHist:  288,
	}
}

// Ingest processes a dataset sample
func (a *Attributor) Ingest(ds DatasetIOSample, vdevs []VdevSample) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tag := ds.Dataset
	now := time.Now().Unix()

	for _, vdev := range vdevs {
		if vdev.Pool != ds.Pool {
			continue
		}
		if a.profiles[vdev.VdevID] == nil {
			a.profiles[vdev.VdevID] = make(map[string]*IOProfile)
		}
		for _, opType := range []string{"read", "write"} {
			key := tag + ":" + opType
			if _, ok := a.profiles[vdev.VdevID][key]; !ok {
				a.profiles[vdev.VdevID][key] = &IOProfile{
					VdevID:      vdev.VdevID,
					WorkloadTag: tag,
					OpType:      opType,
				}
			}
			p := a.profiles[vdev.VdevID][key]
			var count, bytes, rtime int64
			if opType == "read" {
				count, bytes, rtime = ds.NRead, ds.ReadBytes, ds.ReadTime
			} else {
				count, bytes, rtime = ds.NWrite, ds.WriteBytes, ds.WriteTime
			}
			p.Count += count
			p.BytesTotal += bytes
			if count > 0 && rtime > 0 {
				avgLat := rtime / count
				p.TotalLatencyUs += avgLat * count
				if avgLat > p.MaxLatencyUs {
					p.MaxLatencyUs = avgLat
				}
			}
			p.LastUpdateUnix = now
		}
	}
}

// Snapshot takes a point-in-time copy
func (a *Attributor) Snapshot() *AttributionSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	snap := &AttributionSnapshot{Timestamp: time.Now()}
	for _, workloads := range a.profiles {
		for _, p := range workloads {
			cp := *p
			snap.Profiles = append(snap.Profiles, &cp)
		}
	}
	a.history = append(a.history, snap)
	if len(a.history) > a.maxHist {
		a.history = a.history[1:]
	}
	return snap
}

// TopWorkloads returns top N workloads by bytes for a vdev
func (a *Attributor) TopWorkloads(vdevID string, n int) []*IOProfile {
	a.mu.RLock()
	defer a.mu.RUnlock()

	workloads := a.profiles[vdevID]
	profiles := make([]*IOProfile, 0, len(workloads))
	for _, p := range workloads {
		profiles = append(profiles, p)
	}
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			if profiles[j].BytesTotal > profiles[i].BytesTotal {
				profiles[i], profiles[j] = profiles[j], profiles[i]
			}
		}
	}
	if n > 0 && n < len(profiles) {
		return profiles[:n]
	}
	return profiles
}

// SampleDatasets reads /proc/spl/kstat/zfs/ for per-dataset I/O
func SampleDatasets() ([]DatasetIOSample, error) {
	const base = "/proc/spl/kstat/zfs"
	pools, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", base, err)
	}
	var samples []DatasetIOSample
	now := time.Now()
	for _, pool := range pools {
		if !pool.IsDir() {
			continue
		}
		poolDir := fmt.Sprintf("%s/%s", base, pool.Name())
		entries, err := os.ReadDir(poolDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), "objset-") {
				continue
			}
			data, err := os.ReadFile(poolDir + "/" + e.Name())
			if err != nil {
				continue
			}
			s := parseObjset(string(data), pool.Name(), now)
			if s != nil {
				samples = append(samples, *s)
			}
		}
	}
	return samples, nil
}

func parseObjset(raw, pool string, ts time.Time) *DatasetIOSample {
	s := &DatasetIOSample{Pool: pool, SampledAt: ts}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "dataset_name":
			s.Dataset = fields[2]
			continue
		}
		val, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "nread":
			s.NRead = val
		case "nwritten":
			s.NWrite = val
		case "reads":
			s.ReadBytes = val
		case "writes":
			s.WriteBytes = val
		case "read_time":
			s.ReadTime = val / 1000
		case "write_time":
			s.WriteTime = val / 1000
		}
	}
	if s.Dataset == "" {
		return nil
	}
	return s
}
