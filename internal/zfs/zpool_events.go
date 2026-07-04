package zfs

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ZFSEvent struct {
	Timestamp time.Time         `json:"timestamp"`
	PoolName  string            `json:"pool_name"`
	EventType string            `json:"event_type"`
	EID       uint64            `json:"eid"`
	Class     string            `json:"class"`
	Subclass  string            `json:"subclass"`
	Data      map[string]string `json:"data"`
}

var (
	eventLineRe = regexp.MustCompile(`^(\w+ \d{1,2} \d{2}:\d{2}:\d{2}\.\d+)\s+(\S+)\s+\(([^)]+)\)\s+(.*)$`)
	kvRe        = regexp.MustCompile(`(\w+)=(\S+)`)
	eidRe       = regexp.MustCompile(`eid=(\d+)`)
	classRe     = regexp.MustCompile(`class=(\S+)`)
	subclassRe  = regexp.MustCompile(`subclass=(\S+)`)
)

func parseZFSEvent(line string) *ZFSEvent {
	m := eventLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	tsStr := m[1]
	poolName := m[2]
	eventType := m[3]
	kvStr := m[4]

	now := time.Now()
	ts, err := time.Parse("Jan 2 15:04:05.000000", tsStr)
	if err != nil {
		ts, _ = time.Parse("Jan 2 15:04:05", tsStr)
	}
	ts = ts.AddDate(now.Year(), 0, 0).UTC()
	if ts.After(now) {
		ts = ts.AddDate(-1, 0, 0)
	}

	ev := &ZFSEvent{
		Timestamp: ts,
		PoolName:  poolName,
		EventType: eventType,
		Data:      map[string]string{},
	}

	if m2 := eidRe.FindStringSubmatch(kvStr); m2 != nil {
		ev.EID, _ = strconv.ParseUint(m2[1], 10, 64)
	}
	if m2 := classRe.FindStringSubmatch(kvStr); m2 != nil {
		ev.Class = m2[1]
	}
	if m2 := subclassRe.FindStringSubmatch(kvStr); m2 != nil {
		ev.Subclass = m2[1]
	}
	for _, kv := range kvRe.FindAllStringSubmatch(kvStr, -1) {
		k := kv[1]
		if k != "eid" && k != "class" && k != "subclass" && k != "data_type" {
			ev.Data[k] = kv[2]
		}
	}
	return ev
}

// StreamEvents runs zpool events -v and sends parsed events to ch.
// Blocks until ctx is cancelled.
func StreamEvents(ctx context.Context, ch chan<- *ZFSEvent) error {
	cmd := exec.CommandContext(ctx, "zpool", "events", "-v")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Time") {
			continue
		}
		if ev := parseZFSEvent(line); ev != nil {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return cmd.Wait()
}
