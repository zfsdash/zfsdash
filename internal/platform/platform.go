package platform

import (
	"os"
	"os/exec"
	"runtime"
)

// ServiceManager names the init system on the platform.
type ServiceManager string

const (
	ServiceManagerSystemd ServiceManager = "systemd"
	ServiceManagerRCD     ServiceManager = "rc.d"
	ServiceManagerLaunchd ServiceManager = "launchd"
)

// Platform holds detected runtime environment information.
type Platform struct {
	// OS is the detected operating system: "linux", "freebsd", or "darwin".
	OS string

	// ZpoolPath is the absolute path to the zpool binary.
	ZpoolPath string

	// ZFSPath is the absolute path to the zfs binary.
	ZFSPath string

	// SmartctlPath is the absolute path to smartctl. Empty if not found.
	SmartctlPath string

	// ServiceManager is the init/service manager for this platform.
	ServiceManager ServiceManager
}

// Detect returns a Platform struct describing the current runtime environment.
// Binary paths are probed in order: known platform path → PATH fallback → empty string.
// No error is returned; callers should check individual fields for empty strings when
// a binary is required.
func Detect() Platform {
	os := runtime.GOOS

	var (
		zpoolPath    string
		zfsPath      string
		smartctlPath string
		svcMgr       ServiceManager
	)

	switch os {
	case "linux":
		zpoolPath = resolveFirst("/sbin/zpool", "/usr/sbin/zpool", "zpool")
		zfsPath = resolveFirst("/sbin/zfs", "/usr/sbin/zfs", "zfs")
		smartctlPath = resolveFirst("/usr/sbin/smartctl", "/sbin/smartctl", "smartctl")
		svcMgr = ServiceManagerSystemd
	case "freebsd":
		zpoolPath = resolveFirst("/sbin/zpool", "zpool")
		zfsPath = resolveFirst("/sbin/zfs", "zfs")
		smartctlPath = resolveFirst("/usr/local/sbin/smartctl", "smartctl")
		svcMgr = ServiceManagerRCD
	case "darwin":
		zpoolPath = resolveFirst("/usr/local/sbin/zpool", "/opt/homebrew/sbin/zpool", "zpool")
		zfsPath = resolveFirst("/usr/local/sbin/zfs", "/opt/homebrew/sbin/zfs", "zfs")
		smartctlPath = resolveFirst("/usr/local/sbin/smartctl", "/opt/homebrew/sbin/smartctl", "smartctl")
		svcMgr = ServiceManagerLaunchd
	default:
		// Best-effort fallback for unknown platforms.
		zpoolPath = resolvePATH("zpool")
		zfsPath = resolvePATH("zfs")
		smartctlPath = resolvePATH("smartctl")
		svcMgr = ServiceManagerSystemd
	}

	return Platform{
		OS:             os,
		ZpoolPath:      zpoolPath,
		ZFSPath:        zfsPath,
		SmartctlPath:   smartctlPath,
		ServiceManager: svcMgr,
	}
}

// resolveFirst returns the first path that exists on disk, then falls through
// to exec.LookPath for the last element (which should be a bare binary name).
func resolveFirst(candidates ...string) string {
	for i, p := range candidates {
		// Last candidate: try PATH lookup.
		if i == len(candidates)-1 {
			return resolvePATH(p)
		}
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// resolvePATH looks up name in $PATH and returns the resolved path, or empty string.
func resolvePATH(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// fileExists returns true if the path exists and is a regular file (or symlink).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
