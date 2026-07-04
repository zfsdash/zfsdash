package zfs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type SendReceiveJob struct {
	ID          string
	Pool        string
	Dataset     string
	Snapshot    string
	Destination string
	Incremental string // base snapshot for incremental
	Status      string // "running", "complete", "failed"
	BytesSent   int64
	Started     time.Time
	Finished    *time.Time
	Error       string
}

// SendSnapshot sends a ZFS snapshot to a destination dataset.
// If incremental is non-empty, performs incremental send from that snapshot.
func SendSnapshot(ctx context.Context, dataset, snapshot, incremental, destination string, progress func(int64)) error {
	var args []string
	if incremental != "" {
		args = []string{"send", "-i", incremental, dataset + "@" + snapshot}
	} else {
		args = []string{"send", dataset + "@" + snapshot}
	}

	sendCmd := exec.CommandContext(ctx, "zfs", args...)
	recvCmd := exec.CommandContext(ctx, "zfs", "recv", "-F", destination)

	pr, pw := io.Pipe()
	sendCmd.Stdout = pw
	recvCmd.Stdin = pr

	var sendErr, recvErr error

	go func() {
		sendErr = sendCmd.Run()
		pw.Close()
	}()

	recvErr = recvCmd.Run()
	pr.Close()

	if sendErr != nil {
		return fmt.Errorf("zfs send: %w", sendErr)
	}
	if recvErr != nil {
		return fmt.Errorf("zfs recv: %w", recvErr)
	}
	return nil
}

// GetResumeToken returns the resume token for an interrupted receive, if any.
func GetResumeToken(dataset string) (string, error) {
	out, err := exec.Command("zfs", "get", "-H", "-o", "value", "receive_resume_token", dataset).Output()
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(out))
	if tok == "none" || tok == "-" {
		return "", nil
	}
	return tok, nil
}

// ResumeReceive resumes an interrupted zfs receive using a resume token.
func ResumeReceive(ctx context.Context, token, source string) error {
	sendCmd := exec.CommandContext(ctx, "zfs", "send", "-t", token)
	recvCmd := exec.CommandContext(ctx, "zfs", "recv", "-s", source)

	pr, pw := io.Pipe()
	sendCmd.Stdout = pw
	recvCmd.Stdin = pr

	var sendErr error
	go func() {
		sendErr = sendCmd.Run()
		pw.Close()
	}()

	recvErr := recvCmd.Run()
	pr.Close()

	if sendErr != nil {
		return fmt.Errorf("zfs send -t: %w", sendErr)
	}
	return recvErr
}

// ParseSendProgress parses zfs send -v output to extract bytes sent.
func ParseSendProgress(line string) (int64, bool) {
	// Format: "sending dataset@snap: X bytes sent"
	if !strings.Contains(line, "bytes") {
		return 0, false
	}
	scanner := bufio.NewScanner(strings.NewReader(line))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		for i, f := range fields {
			if f == "bytes" && i > 0 {
				v, err := strconv.ParseInt(strings.TrimRight(fields[i-1], ","), 10, 64)
				if err == nil {
					return v, true
				}
			}
		}
	}
	return 0, false
}
