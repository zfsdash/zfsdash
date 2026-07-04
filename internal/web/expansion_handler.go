package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zfsdash/zfsdash/internal/expansion"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

func (h *Handler) handleExpansionPlan(w http.ResponseWriter, r *http.Request) {
	poolName := r.URL.Query().Get("pool")
	driveSizeStr := r.URL.Query().Get("drive_size_gb")
	driveCountStr := r.URL.Query().Get("drive_count")

	driveGB := 8000.0
	if driveSizeStr != "" {
		if v, err := strconv.ParseFloat(driveSizeStr, 64); err == nil {
			driveGB = v
		}
	}
	driveCounts := []int{2, 3, 4, 6}
	if driveCountStr != "" {
		if v, err := strconv.Atoi(driveCountStr); err == nil {
			driveCounts = []int{v}
		}
	}

	topo := expansion.PoolTopology{Name: poolName}
	pools, err := zfs.ListPools()
	if err == nil {
		for _, p := range pools {
			if p.Name == poolName {
				topo.TotalGB = float64(p.Size) / 1e9
				topo.UsedGB = float64(p.Allocated) / 1e9
				topo.UsableGB = float64(p.Free)/1e9 + topo.UsedGB
				break
			}
		}
	}

	vdevTypes := []expansion.VdevType{expansion.VdevMirror, expansion.VdevRAIDZ1, expansion.VdevRAIDZ2}
	scenarios := expansion.Plan(topo, driveGB, driveCounts, vdevTypes)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"pool": topo, "scenarios": scenarios})
}

func (h *Handler) handleExpansionValidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool        string             `json:"pool"`
		VdevType    expansion.VdevType `json:"vdev_type"`
		DriveCount  int                `json:"drive_count"`
		DriveSizeGB float64            `json:"drive_size_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	spec := expansion.DriveSpec{Count: req.DriveCount, SizeGB: req.DriveSizeGB, VdevType: req.VdevType}
	topo := expansion.PoolTopology{Name: req.Pool}
	errs := expansion.Validate(topo, spec)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"valid": len(errs) == 0, "errors": errs})
}
