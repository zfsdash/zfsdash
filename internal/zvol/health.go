package zvol

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// Zvol represents a ZFS volume (block device)
type Zvol struct {
	Name          string `json:"name"`
	Volsize       int64  `json:"volsize_bytes"`
	Used          int64  `json:"used_bytes"`
	Referenced    int64  `json:"referenced_bytes"`
	LogicalUsed   int64  `json:"logical_used_bytes"`
	Compression   string `json:"compression"`
	Refreservation int64 `json:"refreservation_bytes"` // -1 = none
	SnapshotCount int    `json:"snapshot_count"`
	SnapshotUsed  int64  `json:"snapshot_used_bytes"`
}

// HealthIssue represents a detected problem with a zvol
type HealthIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

// ZvolHealth is the full health report for a zvol
type ZvolHealth struct {
	Zvol   Zvol          `json:"zvol"`
	Issues []HealthIssue `json:"issues"`
	Status string        `json:"status"` // healthy, warning, critical
}

// PoolCapacityRisk reports over-provisioning risk for a pool
type PoolCapacityRisk struct {
	PoolName         string  `json:"pool_name"`
	PoolSizeBytes    int64   `json:"pool_size_bytes"`
	PoolFreeBytes    int64   `json:"pool_free_bytes"`
	TotalVolsize     int64   `json:"total_volsize_bytes"`
	TotalReserved    int64   `json:"total_reserved_bytes"`
	OverprovisionPct float64 `json:"overprovision_pct"`
	Risk             string  `json:"risk"` // low, medium, high, critical
}

// ListZvols returns all ZFS volumes in a pool
func ListZvols(pool string) ([]Zvol, error) {
	args := []string{"list", "-t", "volume", "-H", "-p",
		"-o", "name,volsize,used,refer,logicalused,compression,refreservation", "-r", pool}
	out, err := exec.Command("zfs", args...).Output()
	if err != nil {
		return nil, err
	}

	var zvols []Zvol
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 7 {
			continue
		}
		z := Zvol{
			Name:        fields[0],
			Compression: fields[5],
		}
		z.Volsize, _ = strconv.ParseInt(fields[1], 10, 64)
		z.Used, _ = strconv.ParseInt(fields[2], 10, 64)
		z.Referenced, _ = strconv.ParseInt(fields[3], 10, 64)
		z.LogicalUsed, _ = strconv.ParseInt(fields[4], 10, 64)
		if fields[6] == "none" {
			z.Refreservation = 0
		} else {
			z.Refreservation, _ = strconv.ParseInt(fields[6], 10, 64)
		}

		// Count snapshots
		snapOut, _ := exec.Command("zfs", "list", "-t", "snapshot", "-H", "-p",
			"-o", "name,used", "-r", fields[0]).Output()
		sc2 := bufio.NewScanner(strings.NewReader(string(snapOut)))
		for sc2.Scan() {
			sfields := strings.Fields(sc2.Text())
			if len(sfields) >= 2 {
				z.SnapshotCount++
				u, _ := strconv.ParseInt(sfields[1], 10, 64)
				z.SnapshotUsed += u
			}
		}

		zvols = append(zvols, z)
	}
	return zvols, nil
}

// CheckHealth runs health checks on a single zvol
func CheckHealth(z Zvol) ZvolHealth {
	h := ZvolHealth{Zvol: z, Status: "healthy"}

	// Check: refreservation not set (thin provisioning risk)
	if z.Refreservation == 0 && z.Volsize > 10*1024*1024*1024 { // > 10GB
		h.Issues = append(h.Issues, HealthIssue{
			Type:        "THIN_PROVISIONING_RISK",
			Severity:    "warning",
			Description: "refreservation=none — pool may be over-provisioned",
			Remediation: "Set refreservation: zfs set refreservation=" + humanBytes(z.Volsize) + " " + z.Name,
		})
	}

	// Check: snapshot space leak (snapshots using > 20% of volsize)
	if z.Volsize > 0 {
		snapPct := float64(z.SnapshotUsed) / float64(z.Volsize) * 100
		if snapPct > 20 {
			h.Issues = append(h.Issues, HealthIssue{
				Type:        "SNAPSHOT_SPACE_LEAK",
				Severity:    "warning",
				Description: "Snapshots consuming " + strconv.FormatFloat(snapPct, 'f', 1, 64) + "% of volsize",
				Remediation: "Review and prune old snapshots: zfs list -t snapshot -r " + z.Name,
			})
		}
	}

	// Check: usage near volsize
	if z.Volsize > 0 {
		usedPct := float64(z.Referenced) / float64(z.Volsize) * 100
		if usedPct > 90 {
			h.Issues = append(h.Issues, HealthIssue{
				Type:        "NEAR_CAPACITY",
				Severity:    "critical",
				Description: "Zvol is " + strconv.FormatFloat(usedPct, 'f', 1, 64) + "% full",
				Remediation: "Expand: zfs set volsize=" + humanBytes(z.Volsize*2) + " " + z.Name,
			})
		} else if usedPct > 75 {
			h.Issues = append(h.Issues, HealthIssue{
				Type:        "APPROACHING_CAPACITY",
				Severity:    "warning",
				Description: "Zvol is " + strconv.FormatFloat(usedPct, 'f', 1, 64) + "% full",
				Remediation: "Plan expansion: zfs set volsize=<new_size> " + z.Name,
			})
		}
	}

	// Determine overall status
	for _, issue := range h.Issues {
		if issue.Severity == "critical" {
			h.Status = "critical"
			break
		}
		if issue.Severity == "warning" && h.Status == "healthy" {
			h.Status = "warning"
		}
	}

	return h
}

// CheckPoolCapacityRisk checks for over-provisioning across all zvols in a pool
func CheckPoolCapacityRisk(pool string) (*PoolCapacityRisk, error) {
	// Get pool size/free
	out, err := exec.Command("zpool", "list", "-H", "-p", "-o",
		"name,size,free", pool).Output()
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 3 {
		return nil, nil
	}
	risk := &PoolCapacityRisk{PoolName: pool}
	risk.PoolSizeBytes, _ = strconv.ParseInt(fields[1], 10, 64)
	risk.PoolFreeBytes, _ = strconv.ParseInt(fields[2], 10, 64)

	// Sum all zvol volsizes
	zvols, err := ListZvols(pool)
	if err != nil {
		return risk, nil
	}
	for _, z := range zvols {
		risk.TotalVolsize += z.Volsize
		risk.TotalReserved += z.Refreservation
	}

	if risk.PoolSizeBytes > 0 {
		risk.OverprovisionPct = float64(risk.TotalVolsize) / float64(risk.PoolSizeBytes) * 100
	}

	switch {
	case risk.OverprovisionPct > 150:
		risk.Risk = "critical"
	case risk.OverprovisionPct > 110:
		risk.Risk = "high"
	case risk.OverprovisionPct > 90:
		risk.Risk = "medium"
	default:
		risk.Risk = "low"
	}

	return risk, nil
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatInt(b, 10) + "B"
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(b)/float64(div), 'f', 1, 64) +
		string("KMGTPE"[exp]) + "B"
}

// JSON marshal helper
func ToJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// GetLogger returns a default logger
func GetLogger() *slog.Logger {
	return slog.Default()
}
