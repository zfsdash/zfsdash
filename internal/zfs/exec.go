package zfs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// binaryPath returns the correct path to a ZFS binary for the current OS.
func binaryPath(binary string) (string, error) {
	var paths []string
	switch runtime.GOOS {
	case "freebsd":
		paths = []string{"/sbin/" + binary}
	case "linux":
		paths = []string{
			"/usr/sbin/" + binary,
			"/sbin/" + binary,
		}
	default:
		paths = []string{
			"/usr/sbin/" + binary,
			"/sbin/" + binary,
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// Fall back to PATH
	if found, err := exec.LookPath(binary); err == nil {
		return found, nil
	}
	return "", fmt.Errorf("%s not found (install zfsutils-linux or enable ZFS)", binary)
}

func runZpool(ctx context.Context, args ...string) (string, error) {
	return runBinary(ctx, "zpool", args...)
}

func runZfs(ctx context.Context, args ...string) (string, error) {
	return runBinary(ctx, "zfs", args...)
}

func runBinary(ctx context.Context, binary string, args ...string) (string, error) {
	path, err := binaryPath(binary)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s %v failed: %s", binary, args, string(ee.Stderr))
		}
		return "", err
	}
	return string(out), nil
}
