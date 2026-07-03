package zfs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// LocalCollector collects ZFS data by running zpool/zfs binaries on the local system.
type LocalCollector struct{}

// NewLocalCollector creates a collector that reads from the local system.
func NewLocalCollector() *LocalCollector { return &LocalCollector{} }

func (l *LocalCollector) run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", name, args, err, stderr.String())
	}
	return stdout.String(), nil
}

func (l *LocalCollector) GetPools(ctx context.Context) ([]*Pool, error) {
	out, err := l.run(ctx, "zpool", "list", "-H", "-p",
		"-o", "name,size,allocated,free,fragmentation,capacity,health")
	if err != nil {
		return nil, err
	}
	pools, err := ParsePoolList(out)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if sOut, serr := l.run(ctx, "zpool", "status", pool.Name); serr == nil {
			ParseZpoolStatus(sOut, pool)
		}
	}
	return pools, nil
}

func (l *LocalCollector) GetDatasets(ctx context.Context, pool string) ([]*Dataset, error) {
	args := []string{"list", "-H", "-p", "-t", "filesystem,volume",
		"-o", "name,type,used,avail,refer,logicalused,mounted,mountpoint,compression,ratio,dedup,quota,reservation,volsize,encryption"}
	if pool != "" {
		args = append(args, pool)
	}
	out, err := l.run(ctx, "zfs", args...)
	if err != nil {
		return nil, err
	}
	return ParseDatasetList(out)
}

func (l *LocalCollector) GetSnapshots(ctx context.Context, dataset string) ([]*Snapshot, error) {
	args := []string{"list", "-H", "-p", "-t", "snapshot", "-o", "name,used,refer"}
	if dataset != "" {
		args = append(args, dataset)
	}
	out, err := l.run(ctx, "zfs", args...)
	if err != nil {
		if strings.Contains(err.Error(), "no datasets available") {
			return nil, nil
		}
		return nil, err
	}
	return ParseSnapshotList(out)
}

func (l *LocalCollector) GetSMARTData(ctx context.Context) ([]*SMARTData, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "--scan")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, nil // smartctl not installed
	}
	var results []*SMARTData
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		if sd := querySMARTDevice(ctx, fields[0]); sd != nil {
			results = append(results, sd)
		}
	}
	return results, nil
}

func (l *LocalCollector) CreateSnapshot(ctx context.Context, dataset, snapName string) error {
	_, err := l.run(ctx, "zfs", "snapshot", dataset+"@"+snapName)
	return err
}

func (l *LocalCollector) DeleteSnapshot(ctx context.Context, fullName string) error {
	_, err := l.run(ctx, "zfs", "destroy", fullName)
	return err
}

func (l *LocalCollector) StartScrub(ctx context.Context, pool string) error {
	_, err := l.run(ctx, "zpool", "scrub", pool)
	return err
}

func (l *LocalCollector) StopScrub(ctx context.Context, pool string) error {
	_, err := l.run(ctx, "zpool", "scrub", "-s", pool)
	return err
}

func (l *LocalCollector) Close() error { return nil }

// querySMARTDevice runs smartctl on a single device and returns parsed SMART data.
func querySMARTDevice(ctx context.Context, dev string) *SMARTData {
	cmd := exec.CommandContext(ctx, "smartctl", "-A", "-H", "-i", dev)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()
	return parseSMARTOutput(dev, out.String())
}

// parseSMARTOutput parses smartctl -A -H -i output into SMARTData.
func parseSMARTOutput(dev, output string) *SMARTData {
	sd := &SMARTData{Device: dev, Health: "UNKNOWN", UpdatedAt: time.Now()}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "SMART overall-health") {
			if strings.Contains(line, "PASSED") {
				sd.Health = "PASSED"
			} else {
				sd.Health = "FAILED"
			}
		}
		if strings.HasPrefix(line, "Device Model:") {
			sd.Model = strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
		}
		if strings.HasPrefix(line, "Serial Number:") {
			sd.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		switch fields[1] {
		case "Temperature_Celsius", "Airflow_Temperature_Cel":
			v, _ := strconv.Atoi(fields[9])
			sd.Temperature = v
		case "Power_On_Hours":
			sd.PowerOnHours, _ = strconv.ParseUint(fields[9], 10, 64)
		case "Power_Cycle_Count":
			sd.PowerCycles, _ = strconv.ParseUint(fields[9], 10, 64)
		case "Reallocated_Sector_Ct", "Reallocated_Event_Count":
			sd.ReallocatedSectors, _ = strconv.ParseUint(fields[9], 10, 64)
		case "Current_Pending_Sector":
			sd.PendingSectors, _ = strconv.ParseUint(fields[9], 10, 64)
		case "Offline_Uncorrectable":
			sd.UncorrectableErrors, _ = strconv.ParseUint(fields[9], 10, 64)
		}
	}
	return sd
}
