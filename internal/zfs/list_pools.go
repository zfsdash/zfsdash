package zfs

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

type PoolInfo struct {
	Name      string
	Size      int64
	Allocated int64
	Free      int64
	Health    string
}

// ListPools returns basic info for all ZFS pools.
func ListPools() ([]PoolInfo, error) {
	out, err := exec.Command("zpool", "list", "-H", "-p", "-o", "name,size,allocated,free,health").Output()
	if err != nil {
		return nil, err
	}
	var pools []PoolInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		p := PoolInfo{Name: fields[0], Health: fields[4]}
		p.Size, _ = strconv.ParseInt(fields[1], 10, 64)
		p.Allocated, _ = strconv.ParseInt(fields[2], 10, 64)
		p.Free, _ = strconv.ParseInt(fields[3], 10, 64)
		pools = append(pools, p)
	}
	return pools, nil
}

// ListPoolsJSON returns pool list as JSON bytes (for API responses).
func ListPoolsJSON() ([]byte, error) {
	pools, err := ListPools()
	if err != nil {
		return json.Marshal([]PoolInfo{})
	}
	return json.Marshal(pools)
}
