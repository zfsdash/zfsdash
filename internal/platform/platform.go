package platform

import "runtime"

type OS string

const (
	Linux   OS = "linux"
	FreeBSD OS = "freebsd"
	Unknown OS = "unknown"
)

func Current() OS {
	switch runtime.GOOS {
	case "linux":
		return Linux
	case "freebsd":
		return FreeBSD
	default:
		return Unknown
	}
}

func ZpoolBin() string {
	if Current() == FreeBSD {
		return "/sbin/zpool"
	}
	return "/usr/sbin/zpool"
}

func ZfsBin() string {
	if Current() == FreeBSD {
		return "/sbin/zfs"
	}
	return "/usr/sbin/zfs"
}

func SmartctlBin() string {
	if Current() == FreeBSD {
		return "/usr/local/sbin/smartctl"
	}
	return "/usr/sbin/smartctl"
}

func IsFreeBSD() bool { return Current() == FreeBSD }
