package expansion

import (
	"fmt"
	"math"
	"strings"
)

type VdevType string

const (
	VdevMirror VdevType = "mirror"
	VdevRAIDZ1 VdevType = "raidz1"
	VdevRAIDZ2 VdevType = "raidz2"
	VdevRAIDZ3 VdevType = "raidz3"
	VdevSingle VdevType = "single"
)

type DriveSpec struct {
	Count    int
	SizeGB   float64
	VdevType VdevType
}

type ExpansionScenario struct {
	DriveSpec
	RawCapacityGB    float64
	UsableCapacityGB float64
	Overhead         float64
	FaultTolerance   int
	EstimatedRebuildH float64
	CapacityGainGB   float64
	Warnings         []string
	Recommendation   string
	Command          string
}

type PoolTopology struct {
	Name         string
	UsableGB     float64
	TotalGB      float64
	UsedGB       float64
	VdevType     VdevType
	VdevCount    int
	DrivePerVdev int
	DriveGB      float64
}

func Plan(pool PoolTopology, driveGB float64, driveCounts []int, vdevTypes []VdevType) []ExpansionScenario {
	var scenarios []ExpansionScenario
	for _, count := range driveCounts {
		for _, vtype := range vdevTypes {
			if !validConfig(vtype, count) {
				continue
			}
			s := calculate(pool, DriveSpec{count, driveGB, vtype})
			scenarios = append(scenarios, s)
		}
	}
	return scenarios
}

func validConfig(vtype VdevType, count int) bool {
	switch vtype {
	case VdevMirror:
		return count >= 2
	case VdevRAIDZ1:
		return count >= 3
	case VdevRAIDZ2:
		return count >= 4
	case VdevRAIDZ3:
		return count >= 5
	case VdevSingle:
		return count == 1
	}
	return false
}

func calculate(pool PoolTopology, spec DriveSpec) ExpansionScenario {
	s := ExpansionScenario{DriveSpec: spec}
	rawGB := float64(spec.Count) * spec.SizeGB
	switch spec.VdevType {
	case VdevMirror:
		s.UsableCapacityGB = rawGB / 2
		s.Overhead = 0.5
		s.FaultTolerance = spec.Count / 2
	case VdevRAIDZ1:
		s.UsableCapacityGB = float64(spec.Count-1) * spec.SizeGB
		s.Overhead = 1.0 / float64(spec.Count)
		s.FaultTolerance = 1
	case VdevRAIDZ2:
		s.UsableCapacityGB = float64(spec.Count-2) * spec.SizeGB
		s.Overhead = 2.0 / float64(spec.Count)
		s.FaultTolerance = 2
	case VdevRAIDZ3:
		s.UsableCapacityGB = float64(spec.Count-3) * spec.SizeGB
		s.Overhead = 3.0 / float64(spec.Count)
		s.FaultTolerance = 3
	case VdevSingle:
		s.UsableCapacityGB = rawGB
		s.FaultTolerance = 0
	}
	s.RawCapacityGB = rawGB
	s.CapacityGainGB = s.UsableCapacityGB
	s.EstimatedRebuildH = (spec.SizeGB / 100.0) * 1.5
	if spec.VdevType != pool.VdevType && pool.VdevType != "" {
		s.Warnings = append(s.Warnings, fmt.Sprintf("mixing vdev types (%s + %s)", pool.VdevType, spec.VdevType))
	}
	if spec.SizeGB != pool.DriveGB && pool.DriveGB > 0 {
		s.Warnings = append(s.Warnings, fmt.Sprintf("drive size mismatch: pool=%.0fGB new=%.0fGB", pool.DriveGB, spec.SizeGB))
	}
	if spec.VdevType == VdevSingle {
		s.Warnings = append(s.Warnings, "no redundancy — any drive failure = data loss")
	}
	score := scoreScenario(spec, s)
	if score >= 8 {
		s.Recommendation = "recommended"
	} else if score >= 5 {
		s.Recommendation = "acceptable"
	} else {
		s.Recommendation = "not_recommended"
	}
	drives := make([]string, spec.Count)
	for i := range drives {
		drives[i] = fmt.Sprintf("/dev/disk%d", i+1)
	}
	if spec.VdevType == VdevSingle {
		s.Command = fmt.Sprintf("zpool add %s %s", pool.Name, strings.Join(drives, " "))
	} else {
		s.Command = fmt.Sprintf("zpool add %s %s %s", pool.Name, spec.VdevType, strings.Join(drives, " "))
	}
	return s
}

func scoreScenario(spec DriveSpec, s ExpansionScenario) int {
	score := 5
	score += s.FaultTolerance * 2
	efficiency := s.UsableCapacityGB / (float64(spec.Count) * spec.SizeGB)
	score += int(math.Round(efficiency * 3))
	score -= len(s.Warnings)
	if spec.VdevType == VdevSingle {
		score -= 4
	}
	return score
}

func Validate(pool PoolTopology, spec DriveSpec) []string {
	var errors []string
	if !validConfig(spec.VdevType, spec.Count) {
		errors = append(errors, fmt.Sprintf("invalid: %s needs >= %d drives", spec.VdevType, minDrives(spec.VdevType)))
	}
	if pool.DriveGB > 0 && spec.SizeGB < pool.DriveGB*0.5 {
		errors = append(errors, fmt.Sprintf("new drives too small vs existing (%.0f vs %.0fGB)", spec.SizeGB, pool.DriveGB))
	}
	return errors
}

func minDrives(vtype VdevType) int {
	switch vtype {
	case VdevMirror:
		return 2
	case VdevRAIDZ1:
		return 3
	case VdevRAIDZ2:
		return 4
	case VdevRAIDZ3:
		return 5
	default:
		return 1
	}
}
