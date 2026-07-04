package zfs

import (
	"fmt"
	"os/exec"
	"strings"
)

// DriveReplacement wraps zpool replace workflow.

// ListFaultedDevices returns devices with FAULTED or UNAVAIL state in a pool.
func ListFaultedDevices(pool string) ([]string, error) {
	out, err := exec.Command("zpool", "status", "-P", pool).Output()
	if err != nil {
		return nil, fmt.Errorf("zpool status: %w", err)
	}
	var faulted []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			state := strings.ToUpper(fields[1])
			if (state == "FAULTED" || state == "UNAVAIL") && strings.HasPrefix(fields[0], "/") {
				faulted = append(faulted, fields[0])
			}
		}
	}
	return faulted, nil
}

// StartDriveReplace initiates zpool replace oldDev newDev.
// Pass newDev="" to initiate a self-heal (resilver in place).
func StartDriveReplace(pool, oldDev, newDev string) error {
	args := []string{"replace", pool, oldDev}
	if newDev != "" {
		args = append(args, newDev)
	}
	out, err := exec.Command("zpool", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("zpool replace: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Resilvering returns true if the pool is actively resilvering.
func Resilvering(pool string) (bool, string, error) {
	out, err := exec.Command("zpool", "status", pool).Output()
	if err != nil {
		return false, "", fmt.Errorf("zpool status: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "resilver in progress") || strings.Contains(l, "resilvered") {
			return strings.Contains(l, "in progress"), l, nil
		}
	}
	return false, "", nil
}
