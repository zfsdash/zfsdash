package zfs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ReplicationJob represents a configured ZFS send/receive replication job.
type ReplicationJob struct {
	ID              string     `json:"id"`
	SourcePool      string     `json:"source_pool"`
	SourceDataset   string     `json:"source_dataset"`
	DestinationHost string     `json:"destination_host"`
	DestinationPool string     `json:"destination_pool"`
	IntervalSeconds int        `json:"interval_seconds"`
	Compression     string     `json:"compression"`
	Incremental     bool       `json:"incremental"`
	LastRun         *time.Time `json:"last_run"`
	LastSuccess     *time.Time `json:"last_success"`
	LastError       string     `json:"last_error"`
	Enabled         bool       `json:"enabled"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ReplicationStatus is the runtime status of a replication job.
type ReplicationStatus struct {
	JobID     string     `json:"job_id"`
	Running   bool       `json:"running"`
	StartedAt *time.Time `json:"started_at"`
	BytesSent int64      `json:"bytes_sent"`
	Error     string     `json:"error"`
	Snapshot  string     `json:"snapshot"`
}

// VDev is a device in a ZFS pool vdev tree.
type VDev struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	State    string `json:"state"`
	Errors   uint64 `json:"errors"`
	Children []VDev `json:"children,omitempty"`
}

// PoolVDevTree is the full vdev topology of a pool.
type PoolVDevTree struct {
	PoolName string `json:"pool_name"`
	VDevs    []VDev `json:"vdevs"`
}

// ResiverStatus holds resilver progress for a pool.
type ResiverStatus struct {
	PoolName    string  `json:"pool_name"`
	Resilvering bool    `json:"resilvering"`
	Progress    float64 `json:"progress"`
	BytesDone   uint64  `json:"bytes_done"`
	BytesTotal  uint64  `json:"bytes_total"`
}

// GetPoolVDevTree returns the vdev topology for a pool.
func GetPoolVDevTree(ctx context.Context, poolName string) (*PoolVDevTree, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", "-L", poolName)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}
	tree := &PoolVDevTree{
		PoolName: poolName,
		VDevs:    parseVDevLines(string(out)),
	}
	return tree, nil
}

func parseVDevLines(output string) []VDev {
	var vdevs []VDev
	lines := strings.Split(output, "\n")
	inConfig := false
	for _, line := range lines {
		if strings.Contains(line, "config:") {
			inConfig = true
			continue
		}
		if !inConfig {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "NAME") || strings.HasPrefix(trimmed, "errors:") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		state := fields[1]
		switch state {
		case "ONLINE", "DEGRADED", "FAULTED", "OFFLINE", "UNAVAIL", "REMOVED":
			vdevs = append(vdevs, VDev{
				Name:  name,
				Path:  "/dev/" + name,
				State: state,
			})
		}
	}
	return vdevs
}

// ReplaceVDev runs zpool replace to substitute a failed device.
func ReplaceVDev(ctx context.Context, pool, oldDev, newDev string) error {
	cmd := exec.CommandContext(ctx, "zpool", "replace", pool, oldDev, newDev)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zpool replace: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GetResiverStatus returns live resilver progress for a pool.
func GetResiverStatus(ctx context.Context, poolName string) (*ResiverStatus, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}
	status := &ResiverStatus{PoolName: poolName}
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if strings.Contains(t, "resilver in progress") {
			status.Resilvering = true
		}
		if strings.Contains(t, "% complete") {
			var pct float64
			fmt.Sscanf(t, "%f%% complete", &pct)
			status.Progress = pct
		}
	}
	return status, nil
}

// SendReceive performs zfs send | zfs receive for a replication job.
func SendReceive(ctx context.Context, job *ReplicationJob, snapshot string) error {
	sendArgs := []string{"send"}
	if job.Incremental {
		sendArgs = append(sendArgs, "-i", job.SourceDataset+"@prev")
	}
	sendArgs = append(sendArgs, job.SourceDataset+"@"+snapshot)

	send := exec.CommandContext(ctx, "zfs", sendArgs...)
	var recv *exec.Cmd
	if job.DestinationHost != "" {
		recv = exec.CommandContext(ctx, "ssh", job.DestinationHost,
			"zfs receive -F "+job.DestinationPool)
	} else {
		recv = exec.CommandContext(ctx, "zfs", "receive", "-F", job.DestinationPool)
	}

	pipe, err := send.StdoutPipe()
	if err != nil {
		return err
	}
	recv.Stdin = pipe
	if err := send.Start(); err != nil {
		return fmt.Errorf("send start: %w", err)
	}
	if err := recv.Start(); err != nil {
		return fmt.Errorf("recv start: %w", err)
	}
	if err := send.Wait(); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return recv.Wait()
}
