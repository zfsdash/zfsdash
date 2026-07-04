package zfs

import (
	"fmt"
	"os/exec"
	"strings"
)

// Dataset represents a ZFS dataset with its properties.
type Dataset struct {
	Name       string `json:"name"`
	Used       string `json:"used"`
	Avail      string `json:"avail"`
	Refer      string `json:"refer"`
	Mountpoint string `json:"mountpoint"`
}

// Snapshot represents a ZFS snapshot.
type Snapshot struct {
	Name    string `json:"name"`
	Used    string `json:"used"`
	Refer   string `json:"refer"`
	Created string `json:"created"`
}

// ParseDatasets lists all datasets under a pool using 'zfs list'.
func ParseDatasets(pool string) ([]Dataset, error) {
	cmd := exec.Command("zfs", "list", "-H", "-r",
		"-o", "name,used,avail,refer,mountpoint",
		"-t", "filesystem",
		pool)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list: %w", err)
	}

	var datasets []Dataset
	newline := string([]byte{10})
	for _, line := range strings.Split(string(out), newline) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		ds := Dataset{
			Name:       fields[0],
			Used:       fields[1],
			Avail:      fields[2],
			Refer:      fields[3],
			Mountpoint: fields[4],
		}
		datasets = append(datasets, ds)
	}
	return datasets, nil
}

// ParseSnapshots lists snapshots for a given dataset.
func ParseSnapshots(dataset string) ([]Snapshot, error) {
	cmd := exec.Command("zfs", "list", "-H", "-r",
		"-o", "name,used,refer,creation",
		"-t", "snapshot",
		dataset)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list snapshots: %w", err)
	}

	var snaps []Snapshot
	newline := string([]byte{10})
	for _, line := range strings.Split(string(out), newline) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// name is pool/dataset@snapname — extract just the snap name after @
		fullName := fields[0]
		snapName := fullName
		if idx := strings.LastIndex(fullName, "@"); idx >= 0 {
			snapName = fullName[idx+1:]
		}
		snap := Snapshot{
			Name:    snapName,
			Used:    fields[1],
			Refer:   fields[2],
			Created: strings.Join(fields[3:], " "),
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}
