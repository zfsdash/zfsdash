package zfs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ReplicationJob describes a send/receive task.
type ReplicationJob struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`       // e.g. tank/data@snap1
	Target      string    `json:"target"`       // e.g. tank/data (on remote or local)
	RemoteHost  string    `json:"remote_host"`  // empty = local
	RemoteUser  string    `json:"remote_user"`  // default: root
	Incremental bool      `json:"incremental"`
	FromSnap    string    `json:"from_snap"`    // for incremental: previous snapshot
	Recursive   bool      `json:"recursive"`
	DryRun      bool      `json:"dry_run"`
	Status      string    `json:"status"`       // pending, running, done, error
	BytesEst    int64     `json:"bytes_est"`
	BytesSent   int64     `json:"bytes_sent"`
	Started     time.Time `json:"started"`
	Finished    time.Time `json:"finished"`
	Error       string    `json:"error,omitempty"`
	Log         []string  `json:"log"`
}

// EstimateReplication returns byte count for a send without actually sending.
func EstimateReplication(source, fromSnap string, recursive bool) (int64, error) {
	args := []string{"send", "-n", "-P"}
	if recursive {
		args = append(args, "-R")
	}
	if fromSnap != "" {
		args = append(args, "-i", fromSnap)
	}
	args = append(args, source)

	out, err := exec.Command("zfs", args...).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("zfs send -n: %w — %s", err, strings.TrimSpace(string(out)))
	}

	// Parse "size\t<bytes>" line from -P output
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "size") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var n int64
				fmt.Sscanf(parts[1], "%d", &n)
				return n, nil
			}
		}
	}
	return 0, nil
}

// RunReplication executes a zfs send | zfs recv pipeline.
// Progress lines are written to progressCh. Close ctx to cancel.
func RunReplication(ctx context.Context, job *ReplicationJob, progressCh chan<- string) error {
	job.Status = "running"
	job.Started = time.Now()

	// Build send args
	sendArgs := []string{"send", "-P"}
	if job.Recursive {
		sendArgs = append(sendArgs, "-R")
	}
	if job.DryRun {
		sendArgs = append(sendArgs, "-n")
	}
	if job.Incremental && job.FromSnap != "" {
		sendArgs = append(sendArgs, "-i", job.FromSnap)
	}
	sendArgs = append(sendArgs, job.Source)

	// Build recv args
	var recvCmd *exec.Cmd
	recvArgs := []string{"recv", "-u"}
	if job.Incremental {
		recvArgs = append(recvArgs, "-F")
	}
	recvArgs = append(recvArgs, job.Target)

	if job.RemoteHost != "" {
		user := job.RemoteUser
		if user == "" {
			user = "root"
		}
		sshArgs := []string{"-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes",
			user + "@" + job.RemoteHost, "zfs"}
		sshArgs = append(sshArgs, recvArgs...)
		recvCmd = exec.CommandContext(ctx, "ssh", sshArgs...)
	} else {
		recvCmd = exec.CommandContext(ctx, "zfs", recvArgs...)
	}

	sendCmd := exec.CommandContext(ctx, "zfs", sendArgs...)

	// Wire send stdout → recv stdin
	pipe, err := sendCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	recvCmd.Stdin = pipe

	// Capture send stderr for progress
	sendErrPipe, _ := sendCmd.StderrPipe()
	recvErrPipe, _ := recvCmd.StderrPipe()

	if err := sendCmd.Start(); err != nil {
		return fmt.Errorf("zfs send start: %w", err)
	}
	if err := recvCmd.Start(); err != nil {
		sendCmd.Process.Kill()
		return fmt.Errorf("zfs recv start: %w", err)
	}

	// Stream progress from send stderr
	go func() {
		scanner := bufio.NewScanner(io.MultiReader(sendErrPipe, recvErrPipe))
		for scanner.Scan() {
			line := scanner.Text()
			job.Log = append(job.Log, line)
			if progressCh != nil {
				select {
				case progressCh <- line:
				default:
				}
			}
		}
	}()

	if err := sendCmd.Wait(); err != nil {
		recvCmd.Process.Kill()
		job.Status = "error"
		job.Error = fmt.Sprintf("zfs send: %v", err)
		job.Finished = time.Now()
		return fmt.Errorf("zfs send: %w", err)
	}

	if err := recvCmd.Wait(); err != nil {
		job.Status = "error"
		job.Error = fmt.Sprintf("zfs recv: %v", err)
		job.Finished = time.Now()
		return fmt.Errorf("zfs recv: %w", err)
	}

	job.Status = "done"
	job.Finished = time.Now()
	slog.Info("replication complete", "source", job.Source, "target", job.Target,
		"duration", job.Finished.Sub(job.Started).Round(time.Second))
	return nil
}

// ListSnapshots returns snapshots for a dataset, sorted oldest→newest.
func ListSnapshotsForDataset(dataset string) ([]string, error) {
	out, err := exec.Command("zfs", "list", "-H", "-t", "snapshot",
		"-o", "name", "-s", "creation", "-r", dataset).Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list snapshots: %w", err)
	}
	var snaps []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			snaps = append(snaps, line)
		}
	}
	return snaps, nil
}

// LatestSharedSnapshot finds the most recent snapshot present on both source dataset
// and listed in targetSnaps (fetched separately from remote).
func LatestSharedSnapshot(sourceDataset string, targetSnaps []string) (string, error) {
	sourceSnaps, err := ListSnapshotsForDataset(sourceDataset)
	if err != nil {
		return "", err
	}
	targetSet := make(map[string]bool, len(targetSnaps))
	for _, s := range targetSnaps {
		parts := strings.SplitN(s, "@", 2)
		if len(parts) == 2 {
			targetSet[parts[1]] = true
		}
	}
	// Walk source newest→oldest
	for i := len(sourceSnaps) - 1; i >= 0; i-- {
		parts := strings.SplitN(sourceSnaps[i], "@", 2)
		if len(parts) == 2 && targetSet[parts[1]] {
			return sourceSnaps[i], nil
		}
	}
	return "", nil // no shared snapshot → full send needed
}
