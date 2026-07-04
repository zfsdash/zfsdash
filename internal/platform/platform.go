package platform

import (
	"os"
	"runtime"
)

type Platform struct {
	OS           string
	ZpoolPath    string
	ZfsPath      string
	SmartctlPath string
	IsFreeBSD    bool
	IsLinux      bool
}

func Detect() Platform {
	p := Platform{
		OS: runtime.GOOS,
	}

	switch runtime.GOOS {
	case "freebsd":
		p.IsFreeBSD = true
		p.ZpoolPath = findBinary([]string{"/sbin/zpool", "/usr/local/sbin/zpool"})
		p.ZfsPath = findBinary([]string{"/sbin/zfs", "/usr/local/sbin/zfs"})
		p.SmartctlPath = findBinary([]string{"/usr/local/sbin/smartctl", "/usr/sbin/smartctl"})
	case "linux":
		p.IsLinux = true
		p.ZpoolPath = findBinary([]string{"/usr/sbin/zpool", "/usr/local/sbin/zpool"})
		p.ZfsPath = findBinary([]string{"/usr/sbin/zfs", "/usr/local/sbin/zfs"})
		p.SmartctlPath = findBinary([]string{"/usr/sbin/smartctl", "/usr/local/sbin/smartctl"})
	default:
		// Fallback: try common paths
		p.ZpoolPath = findBinary([]string{"/usr/sbin/zpool", "/sbin/zpool", "/usr/local/sbin/zpool"})
		p.ZfsPath = findBinary([]string{"/usr/sbin/zfs", "/sbin/zfs", "/usr/local/sbin/zfs"})
		p.SmartctlPath = findBinary([]string{"/usr/sbin/smartctl", "/usr/local/sbin/smartctl"})
	}

	return p
}

func findBinary(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}
