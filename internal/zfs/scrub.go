package zfs

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zfsdash/zfsdash/internal/events"
)

var (
	scrubRunningRe = regexp.MustCompile(`scrub in progress`)
	scrubDoneRe    = regexp.MustCompile(`scrub repaired`)
	scrubPausedRe  = regexp.MustCompile(`scrub paused`)
	scrubPctRe     = regexp.MustCompile(`(\d+\.\d+)%\s+done`)
	scrubETARe     = regexp.MustCompile(`(\d+h\d+m|\d+m\d+s|\d+\.\d+\s+days?)\s+to go`)
	scrubSpeedRe   = regexp.MustCompile(`(\d+\.?\d*)([KMGT]?)\s+scanned`)
	scrubElapsedRe = regexp.MustCompile(`(\d+) days (\d+)h(\d+)m|(\d+)h(\d+)m`)
)

// ParseScrubStatus runs 'zpool status <pool>' and extracts scrub progress.
// Returns nil if no scrub is active.
func ParseScrubStatus(pool string) (*events.ScrubProgressEvent, error) {
	out, err := exec.Command("zpool", "status", pool).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}

	raw := string(out)

	var state string
	switch {
	case scrubRunningRe.MatchString(raw):
		state = "running"
	case scrubPausedRe.MatchString(raw):
		state = "paused"
	case scrubDoneRe.MatchString(raw):
		state = "finished"
	default:
		return nil, nil // no scrub active
	}

	ev := &events.ScrubProgressEvent{
		Pool:      pool,
		State:     state,
		Timestamp: time.Now(),
	}

	// Extract percent done
	if m := scrubPctRe.FindStringSubmatch(raw); m != nil {
		ev.ProgressPct, _ = strconv.ParseFloat(m[1], 64)
	}

	// Extract ETA string and convert to seconds
	if m := scrubETARe.FindStringSubmatch(raw); m != nil {
		ev.ETASeconds = parseETAToSeconds(m[1])
	}

	// Extract elapsed time
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "scan:") {
			if m := scrubElapsedRe.FindStringSubmatch(line); m != nil {
				if m[1] != "" {
					days, _ := strconv.ParseInt(m[1], 10, 64)
					hrs, _ := strconv.ParseInt(m[2], 10, 64)
					mins, _ := strconv.ParseInt(m[3], 10, 64)
					ev.ElapsedSeconds = days*86400 + hrs*3600 + mins*60
				} else if m[4] != "" {
					hrs, _ := strconv.ParseInt(m[4], 10, 64)
					mins, _ := strconv.ParseInt(m[5], 10, 64)
					ev.ElapsedSeconds = hrs*3600 + mins*60
				}
			}
		}
	}

	// Estimate bytes/sec from progress and elapsed
	if ev.ElapsedSeconds > 0 && ev.ProgressPct > 0 {
		// rough estimate: assume total pool size / 100 bytes per pct
		ev.BytesPerSecond = ev.ProgressPct * 1e9 / float64(ev.ElapsedSeconds) // estimate only
	}

	return ev, nil
}

func parseETAToSeconds(s string) int64 {
	s = strings.TrimSpace(s)
	// formats: "2h14m", "45m30s", "1.5 days"
	var total int64

	if strings.Contains(s, "day") {
		// "1.5 days"
		parts := strings.Fields(s)
		if len(parts) > 0 {
			f, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				total = int64(f * 86400)
			}
		}
		return total
	}

	// "2h14m" or "45m30s"
	hrRe := regexp.MustCompile(`(\d+)h`)
	minRe := regexp.MustCompile(`(\d+)m`)
	secRe := regexp.MustCompile(`(\d+)s`)

	if m := hrRe.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseInt(m[1], 10, 64)
		total += v * 3600
	}
	if m := minRe.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseInt(m[1], 10, 64)
		total += v * 60
	}
	if m := secRe.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseInt(m[1], 10, 64)
		total += v
	}
	return total
}
