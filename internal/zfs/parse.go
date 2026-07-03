package zfs

import (
	"bufio"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// parseUint64 converts a string to uint64, returning 0 for "-" or empty.
func parseUint64(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" || s == "none" {
		return 0
	}
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// parseFloat64 converts a string to float64.
func parseFloat64(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "x")
	if s == "-" || s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseBool interprets "yes"/"on" as true.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "yes" || s == "on" || s == "true" || s == "1"
}

// ParsePoolList parses the output of:
//
//	zpool list -H -p -o name,size,allocated,free,capacity,health[,guid]
//
// Note: 'state' is NOT a valid zpool list property. We infer state from health.
func ParsePoolList(output string) ([]*Pool, error) {
	var pools []*Pool
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}
		health := strings.TrimSpace(fields[5])
		pool := &Pool{
			Name:      strings.TrimSpace(fields[0]),
			State:     health, // state == health in ZFS terms
			Health:    health,
			Size:      parseUint64(fields[1]),
			Allocated: parseUint64(fields[2]),
			Free:      parseUint64(fields[3]),
			UpdatedAt: time.Now(),
		}
		// Capacity in fields[4] may be a raw integer (bytes used as pct) or percentage
		capStr := strings.TrimSuffix(strings.TrimSpace(fields[4]), "%")
		if v, err := strconv.ParseFloat(capStr, 64); err == nil {
			// -pH returns raw fractional percent (0-100)
			if v > 0 && v <= 1 {
				v *= 100 // some versions return 0.xx
			}
			pool.Capacity = v
		} else if pool.Size > 0 {
			pool.Capacity = math.Round(float64(pool.Allocated)/float64(pool.Size)*10000) / 100
		}
		pools = append(pools, pool)
	}
	return pools, scanner.Err()
}

// ParseDatasetList parses the output of:
//
//	zfs list -H -p -t filesystem,volume -o name,type,used,avail,refer,logicalused,mounted,mountpoint,compression,ratio,dedup,quota,reservation,volsize,encryption
func ParseDatasetList(output string) ([]*Dataset, error) {
	var datasets []*Dataset
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 10 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		pool := strings.SplitN(name, "/", 2)[0]
		if idx := strings.Index(pool, "@"); idx >= 0 {
			pool = pool[:idx]
		}
		ds := &Dataset{
			Name:        name,
			Pool:        pool,
			Type:        strings.TrimSpace(fields[1]),
			Used:        parseUint64(fields[2]),
			Available:   parseUint64(fields[3]),
			Referenced:  parseUint64(fields[4]),
			LogicalUsed: parseUint64(fields[5]),
			Mounted:     parseBool(fields[6]),
			MountPoint:  strings.TrimSpace(fields[7]),
			Compression: strings.TrimSpace(fields[8]),
			UpdatedAt:   time.Now(),
		}
		if len(fields) > 9 {
			ds.CompressRatio = parseFloat64(fields[9])
		}
		if len(fields) > 10 {
			ds.Dedup = parseBool(fields[10])
		}
		if len(fields) > 11 {
			ds.Quota = parseUint64(fields[11])
		}
		if len(fields) > 12 {
			ds.Reservation = parseUint64(fields[12])
		}
		if len(fields) > 13 && strings.TrimSpace(fields[13]) != "-" {
			v := parseUint64(fields[13])
			ds.VolSize = &v
		}
		if len(fields) > 14 {
			ds.Encryption = strings.TrimSpace(fields[14])
		}
		datasets = append(datasets, ds)
	}
	return datasets, scanner.Err()
}

// ParseSnapshotList parses snapshot lines from:
//
//	zfs list -H -p -t snapshot -o name,used,refer
func ParseSnapshotList(output string) ([]*Snapshot, error) {
	var snaps []*Snapshot
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		parts := strings.SplitN(name, "@", 2)
		if len(parts) != 2 {
			continue
		}
		dataset := parts[0]
		pool := strings.SplitN(dataset, "/", 2)[0]
		snap := &Snapshot{
			Name:       name,
			Dataset:    dataset,
			Pool:       pool,
			Used:       parseUint64(fields[1]),
			Referenced: parseUint64(fields[2]),
			CreatedAt:  time.Now(),
		}
		snaps = append(snaps, snap)
	}
	return snaps, scanner.Err()
}

// ParseZpoolStatus parses the output of `zpool status <pool>` and updates
// the ScrubStatus on the pool.
func ParseZpoolStatus(output string, pool *Pool) {
	if pool == nil {
		return
	}
	scrub := &ScrubStatus{State: "none", Function: "none"}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		// Extract state from status output
		if strings.HasPrefix(trimmed, "state:") {
			state := strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			if state != "" {
				pool.State = state
				pool.Health = state
			}
		}
		if !strings.HasPrefix(trimmed, "scan:") {
			continue
		}
		scanLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "scan:"))
		switch {
		case strings.Contains(scanLine, "scrub in progress"):
			scrub.State = "in_progress"
			scrub.Function = "scrub"
		case strings.Contains(scanLine, "resilver in progress"):
			scrub.State = "in_progress"
			scrub.Function = "resilver"
		case strings.Contains(scanLine, "scrub repaired"):
			scrub.State = "completed"
			scrub.Function = "scrub"
			if idx := strings.Index(scanLine, "with "); idx >= 0 {
				errStr := strings.Fields(scanLine[idx+5:])
				if len(errStr) > 0 {
					scrub.Errors, _ = strconv.ParseUint(errStr[0], 10, 64)
				}
			}
			if idx := strings.Index(scanLine, "on "); idx >= 0 {
				ts := strings.TrimSpace(scanLine[idx+3:])
				for _, layout := range []string{"Mon Jan  2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
					if t, err := time.Parse(layout, ts); err == nil {
						scrub.EndTime = &t
						break
					}
				}
			}
		case strings.Contains(scanLine, "none requested"):
			scrub.State = "none"
		}
	}
	pool.ScrubStatus = scrub
	pool.ScanStatus = scrub
}
