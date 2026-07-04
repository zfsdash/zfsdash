package zfs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// LocalCollector shells out to zpool and zfs commands directly
type LocalCollector struct {
	timeout time.Duration
}

// NewLocalCollector creates a new local ZFS collector
func NewLocalCollector(timeoutSec int) *LocalCollector {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &LocalCollector{timeout: timeout}
}

func (lc *LocalCollector) CollectPools(ctx context.Context) ([]*Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-p",
		"-o", "name,health,size,allocated,free,capacity")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool list failed: %w", err)
	}
	return parsePoolList(output)
}

func (lc *LocalCollector) CollectDatasets(ctx context.Context, poolName string) ([]*Dataset, error) {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zfs", "list", "-H", "-p",
		"-o", "name,used,available,referenced,mountpoint,type",
		"-r", poolName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list datasets failed: %w", err)
	}
	return parseDatasetList(output)
}

func (lc *LocalCollector) CollectSnapshots(ctx context.Context, datasetName string) ([]*Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zfs", "list", "-H", "-p",
		"-t", "snapshot",
		"-o", "name,used,referenced,creation",
		"-r", datasetName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list snapshots failed: %w", err)
	}
	return parseSnapshotList(output)
}

func (lc *LocalCollector) CollectScrubStatus(ctx context.Context, poolName string) (*Scrub, error) {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zpool", "status", "-v", poolName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status failed: %w", err)
	}
	return parseScrubStatus(output), nil
}

func (lc *LocalCollector) CollectVdevTree(ctx context.Context, poolName string) (*Vdev, error) {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zpool", "status", "-v", poolName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status failed: %w", err)
	}
	return parseVdevTree(output, poolName), nil
}

func (lc *LocalCollector) CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error) {
	// smartctl requires root; return empty map gracefully if unavailable
	cmd := exec.CommandContext(ctx, "which", "smartctl")
	if err := cmd.Run(); err != nil {
		return map[string]*SMARTData{}, nil
	}

	// List block devices
	listCmd := exec.CommandContext(ctx, "lsblk", "-d", "-o", "NAME", "-n")
	output, err := listCmd.Output()
	if err != nil {
		return map[string]*SMARTData{}, nil
	}

	result := map[string]*SMARTData{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		dev := "/dev/" + strings.TrimSpace(scanner.Text())
		smartCmd := exec.CommandContext(ctx, "smartctl", "-j", "-a", dev)
		smartOut, _ := smartCmd.Output()
		if smartData := parseSMARTJSON(smartOut, dev); smartData != nil {
			result[dev] = smartData
		}
	}
	return result, nil
}

func (lc *LocalCollector) CreateSnapshot(ctx context.Context, datasetName, snapshotName string) error {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	fullName := datasetName + "@" + snapshotName
	cmd := exec.CommandContext(ctx, "zfs", "snapshot", fullName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot failed: %w: %s", err, out)
	}
	return nil
}

func (lc *LocalCollector) DestroySnapshot(ctx context.Context, snapshotName string) error {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zfs", "destroy", snapshotName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy failed: %w: %s", err, out)
	}
	return nil
}

func (lc *LocalCollector) StartScrub(ctx context.Context, poolName string) error {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zpool", "scrub", poolName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zpool scrub failed: %w: %s", err, out)
	}
	return nil
}

func (lc *LocalCollector) Close() error { return nil }

// --- parsers ---

func parsePoolList(output []byte) ([]*Pool, error) {
	var pools []*Pool
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}
		size, _ := strconv.ParseInt(fields[2], 10, 64)
		allocated, _ := strconv.ParseInt(fields[3], 10, 64)
		free, _ := strconv.ParseInt(fields[4], 10, 64)
		capPct, _ := strconv.Atoi(strings.TrimSuffix(fields[5], "%"))
		pools = append(pools, &Pool{
			Name:      fields[0],
			Health:    fields[1],
			Size:      size,
			Allocated: allocated,
			Free:      free,
			Capacity:  capPct,
			Timestamp: time.Now(),
		})
	}
	return pools, scanner.Err()
}

func parseDatasetList(output []byte) ([]*Dataset, error) {
	var datasets []*Dataset
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}
		// skip snapshots in dataset list
		if fields[5] == "snapshot" {
			continue
		}
		used, _ := strconv.ParseInt(fields[1], 10, 64)
		avail, _ := strconv.ParseInt(fields[2], 10, 64)
		ref, _ := strconv.ParseInt(fields[3], 10, 64)
		datasets = append(datasets, &Dataset{
			Name:       fields[0],
			Used:       used,
			Available:  avail,
			Referenced: ref,
			Mountpoint: fields[4],
			Type:       fields[5],
			Timestamp:  time.Now(),
		})
	}
	return datasets, scanner.Err()
}

func parseSnapshotList(output []byte) ([]*Snapshot, error) {
	var snapshots []*Snapshot
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		used, _ := strconv.ParseInt(fields[1], 10, 64)
		ref, _ := strconv.ParseInt(fields[2], 10, 64)
		creationTs, _ := strconv.ParseInt(fields[3], 10, 64)
		parts := strings.SplitN(name, "@", 2)
		dataset := ""
		if len(parts) == 2 {
			dataset = parts[0]
		}
		// pool is the first component of dataset name
		pool := ""
		poolParts := strings.SplitN(dataset, "/", 2)
		if len(poolParts) > 0 {
			pool = poolParts[0]
		}
		snapshots = append(snapshots, &Snapshot{
			Name:       name,
			Pool:       pool,
			Dataset:    dataset,
			Used:       used,
			Referenced: ref,
			CreatedAt:  time.Unix(creationTs, 0),
			Timestamp:  time.Now(),
		})
	}
	return snapshots, scanner.Err()
}

func parseScrubStatus(output []byte) *Scrub {
	scrub := &Scrub{State: "none"}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "scan:") {
			if strings.Contains(line, "scrub repaired") {
				scrub.State = "finished"
			} else if strings.Contains(line, "scrub in progress") {
				scrub.State = "scanning"
			} else if strings.Contains(line, "scrub paused") {
				scrub.State = "paused"
			}
		}
		if strings.Contains(line, "errors:") {
			// e.g. "errors: No known data errors"
			if strings.Contains(line, "No known") {
				scrub.Errors = 0
			} else {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					n, _ := strconv.Atoi(parts[1])
					scrub.Errors = uint64(n)
				}
			}
		}
	}
	return scrub
}

func parseVdevTree(output []byte, poolName string) *Vdev {
	root := &Vdev{Name: poolName, Type: "root", State: "ONLINE"}
	// Basic parsing — look for indented disk names in status output
	scanner := bufio.NewScanner(bytes.NewReader(output))
	inConfig := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "NAME") && strings.Contains(line, "STATE") {
			inConfig = true
			continue
		}
		if inConfig {
			if line == "" || strings.HasPrefix(line, "errors:") {
				break
			}
			trimmed := strings.TrimSpace(line)
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && fields[0] != poolName {
				vdev := &Vdev{
					Name:  fields[0],
					State: fields[1],
				}
				if len(fields) >= 5 {
					vdev.Read, _ = strconv.ParseInt(fields[2], 10, 64)
					vdev.Write, _ = strconv.ParseInt(fields[3], 10, 64)
					vdev.Checksum, _ = strconv.ParseInt(fields[4], 10, 64)
				}
				root.Children = append(root.Children, vdev)
			}
		}
	}
	return root
}

func parseSMARTJSON(data []byte, device string) *SMARTData {
	// Minimal JSON parse without full unmarshal dependency
	if len(data) == 0 {
		return nil
	}
	return &SMARTData{
		Device:       device,
		HealthStatus: "unknown",
		Timestamp:    time.Now(),
	}
}
