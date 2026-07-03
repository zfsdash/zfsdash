// Package zfs provides ZFS data collection for ZFSdash.
package zfs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// IsFreeBSD returns true when running on FreeBSD.
func IsFreeBSD() bool { return runtime.GOOS == "freebsd" }

// IsLinux returns true when running on Linux.
func IsLinux() bool { return runtime.GOOS == "linux" }

// DiscoverBlockDevices returns a list of block device paths suitable for
// SMART queries. On Linux it uses lsblk; on FreeBSD it uses geom disk list.
// Returns an empty slice (not an error) if no tool is available.
func DiscoverBlockDevices(ctx context.Context) []string {
	if IsFreeBSD() {
		return discoverBlockDevicesFreeBSD(ctx)
	}
	return discoverBlockDevicesLinux(ctx)
}

func discoverBlockDevicesLinux(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, "lsblk", "-d", "-o", "NAME", "-n")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var devs []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			devs = append(devs, "/dev/"+name)
		}
	}
	return devs
}

func discoverBlockDevicesFreeBSD(ctx context.Context) []string {
	// geom disk list prints "Geom name: daX" lines for each disk.
	cmd := exec.CommandContext(ctx, "geom", "disk", "list")
	out, err := cmd.Output()
	if err != nil {
		// Fall back: try camcontrol devlist
		return discoverBlockDevicesFreeBSDCam(ctx)
	}
	var devs []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Geom name:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "Geom name:"))
			if name != "" {
				devs = append(devs, "/dev/"+name)
			}
		}
	}
	return devs
}

func discoverBlockDevicesFreeBSDCam(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, "camcontrol", "devlist")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var devs []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// Lines look like: <VENDOR MODEL FIRMWARE> at scbus0 target 0 lun 0 (da0,pass0)
		if idx := strings.Index(line, "("); idx != -1 {
			inner := line[idx+1:]
			if end := strings.Index(inner, ")"); end != -1 {
				names := strings.Split(inner[:end], ",")
				if len(names) > 0 {
					dev := strings.TrimSpace(names[0])
					// Only add da/ada (real disk) devices, not pass devices.
					if strings.HasPrefix(dev, "da") || strings.HasPrefix(dev, "ada") {
						devs = append(devs, "/dev/"+dev)
					}
				}
			}
		}
	}
	return devs
}

// SystemInfo returns basic system information. On Linux it reads /proc;
// on FreeBSD it uses sysctl.
type SystemInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname,omitempty"`
	Kernel   string `json:"kernel,omitempty"`
}

// GetSystemInfo returns a SystemInfo struct for the current host.
func GetSystemInfo(ctx context.Context) SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	if IsFreeBSD() {
		info.Kernel = sysctlString(ctx, "kern.osrelease")
		info.Hostname = sysctlString(ctx, "kern.hostname")
	} else {
		// Linux: uname -r
		if out, err := exec.CommandContext(ctx, "uname", "-r").Output(); err == nil {
			info.Kernel = strings.TrimSpace(string(out))
		}
		if out, err := exec.CommandContext(ctx, "hostname").Output(); err == nil {
			info.Hostname = strings.TrimSpace(string(out))
		}
	}
	return info
}

// sysctlString reads a single string sysctl on FreeBSD.
func sysctlString(ctx context.Context, name string) string {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// smartctlPath returns the path to smartctl, searching common locations.
func smartctlPath() (string, error) {
	paths := []string{
		"/usr/local/sbin/smartctl", // FreeBSD pkg install smartmontools
		"/usr/sbin/smartctl",       // Some Linux distros
		"/sbin/smartctl",
	}
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return p, nil
		}
	}
	// Fall back to PATH.
	if found, err := exec.LookPath("smartctl"); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("smartctl not found (install smartmontools)")
}
