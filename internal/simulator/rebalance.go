package simulator

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// VdevConfig represents a single vdev's configuration.
type VdevConfig struct {
	ID       string `json:"id"`
	Type     string `json:"type"`     // mirror, raidz, raidz2, raidz3, spare
	Capacity uint64 `json:"capacity"` // bytes
	Children int    `json:"children"` // number of disks
	Used     uint64 `json:"used"`     // bytes currently used
}

// PoolConfig represents the entire pool configuration.
type PoolConfig struct {
	Name      string       `json:"name"`
	Vdevs     []VdevConfig `json:"vdevs"`
	TotalSize uint64       `json:"total_size"`
	UsedSize  uint64       `json:"used_size"`
}

// RebalanceProposal describes proposed changes to the pool.
type RebalanceProposal struct {
	AddVdevs []VdevConfig `json:"add_vdevs"`
}

// RebalanceResult contains simulation results.
type RebalanceResult struct {
	BeforeConfig              PoolConfig       `json:"before_config"`
	AfterConfig               PoolConfig       `json:"after_config"`
	EstimatedDataMovement     uint64           `json:"estimated_data_movement_bytes"`
	DataMovementPercent       float64          `json:"data_movement_percent"`
	EstimatedDuration         time.Duration    `json:"estimated_duration"`
	DurationHuman             string           `json:"duration_human"`
	BeforeDistribution        VdevDistribution `json:"before_distribution"`
	AfterDistribution         VdevDistribution `json:"after_distribution"`
	CapacityUtilBefore        float64          `json:"capacity_util_before_percent"`
	CapacityUtilAfter         float64          `json:"capacity_util_after_percent"`
	ImprovedBy                float64          `json:"improved_by_percent"`
	Warnings                  []string         `json:"warnings"`
	Notes                     []string         `json:"notes"`
}

// VdevDistribution contains per-vdev capacity details.
type VdevDistribution struct {
	Vdevs         []VdevDistributionDetail `json:"vdevs"`
	AverageUsage  float64                  `json:"average_usage_percent"`
	MaxDeviation  float64                  `json:"max_deviation_percent"`
	WarnThreshold float64                  `json:"warn_threshold_percent"`
}

// VdevDistributionDetail is the per-vdev breakdown.
type VdevDistributionDetail struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	Capacity         uint64  `json:"capacity_bytes"`
	Used             uint64  `json:"used_bytes"`
	UsagePercent     float64 `json:"usage_percent"`
	DeviationPercent float64 `json:"deviation_from_average_percent"`
}

// SimulatorConfig controls rebalance simulation parameters.
type SimulatorConfig struct {
	ThroughputMBps          float64
	WarnUsageThreshold      float64
	WarnDataMovementPercent float64
}

// DefaultSimulatorConfig returns sensible defaults.
func DefaultSimulatorConfig() SimulatorConfig {
	return SimulatorConfig{
		ThroughputMBps:          50,
		WarnUsageThreshold:      80,
		WarnDataMovementPercent: 50,
	}
}

// Simulator performs rebalance simulations.
type Simulator struct {
	config SimulatorConfig
}

// NewSimulator creates a new simulator with default config.
func NewSimulator() *Simulator {
	return &Simulator{config: DefaultSimulatorConfig()}
}

// SimulateRebalance runs the rebalance simulation.
func (s *Simulator) SimulateRebalance(before PoolConfig, proposal RebalanceProposal) *RebalanceResult {
	result := &RebalanceResult{
		BeforeConfig: before,
		Warnings:     []string{},
		Notes:        []string{},
	}

	// Build after-rebalance pool config
	after := before
	after.Vdevs = make([]VdevConfig, len(before.Vdevs))
	copy(after.Vdevs, before.Vdevs)
	after.Vdevs = append(after.Vdevs, proposal.AddVdevs...)

	// Recalculate pool totals
	totalCapacity := uint64(0)
	for _, v := range after.Vdevs {
		totalCapacity += v.Capacity
	}
	after.TotalSize = totalCapacity
	result.AfterConfig = after

	// Calculate data movement
	dataMovement := s.calculateDataMovement(before, after)
	result.EstimatedDataMovement = dataMovement
	if before.UsedSize > 0 {
		result.DataMovementPercent = float64(dataMovement) / float64(before.UsedSize) * 100
	}
	if math.IsNaN(result.DataMovementPercent) || math.IsInf(result.DataMovementPercent, 0) {
		result.DataMovementPercent = 0
	}

	// Estimate duration
	duration := s.estimateDuration(dataMovement)
	result.EstimatedDuration = duration
	result.DurationHuman = formatDuration(duration)

	// Capacity utilization
	if before.TotalSize > 0 {
		result.CapacityUtilBefore = float64(before.UsedSize) / float64(before.TotalSize) * 100
	}
	if after.TotalSize > 0 {
		result.CapacityUtilAfter = float64(after.UsedSize) / float64(after.TotalSize) * 100
	}
	result.ImprovedBy = result.CapacityUtilBefore - result.CapacityUtilAfter

	// Per-vdev distributions
	result.BeforeDistribution = s.buildDistribution(before)
	result.AfterDistribution = s.buildAfterDistribution(after, before.UsedSize)

	// Warnings and notes
	s.validate(result)

	return result
}

func (s *Simulator) calculateDataMovement(before, after PoolConfig) uint64 {
	if before.TotalSize == 0 || before.UsedSize == 0 {
		return 0
	}
	capacityRatio := float64(after.TotalSize) / float64(before.TotalSize)
	if capacityRatio <= 1.0 {
		return 0
	}
	// Fraction of data that moves to new vdevs
	newVdevFraction := 1.0 - (1.0 / capacityRatio)
	movement := uint64(float64(before.UsedSize) * newVdevFraction * 0.6) // 60% efficiency factor
	return movement
}

func (s *Simulator) estimateDuration(dataMovement uint64) time.Duration {
	if s.config.ThroughputMBps <= 0 {
		return 0
	}
	bytesPerSec := s.config.ThroughputMBps * 1024 * 1024
	seconds := float64(dataMovement) / bytesPerSec
	return time.Duration(seconds) * time.Second
}

func (s *Simulator) buildDistribution(pool PoolConfig) VdevDistribution {
	if len(pool.Vdevs) == 0 {
		return VdevDistribution{WarnThreshold: s.config.WarnUsageThreshold}
	}
	details := make([]VdevDistributionDetail, 0, len(pool.Vdevs))
	totalUsage := 0.0
	for _, v := range pool.Vdevs {
		usagePct := 0.0
		if v.Capacity > 0 {
			usagePct = float64(v.Used) / float64(v.Capacity) * 100
		}
		totalUsage += usagePct
		details = append(details, VdevDistributionDetail{
			ID:           v.ID,
			Type:         v.Type,
			Capacity:     v.Capacity,
			Used:         v.Used,
			UsagePercent: usagePct,
		})
	}
	avg := totalUsage / float64(len(details))
	maxDev := 0.0
	for i := range details {
		dev := math.Abs(details[i].UsagePercent - avg)
		details[i].DeviationPercent = dev
		if dev > maxDev {
			maxDev = dev
		}
	}
	sort.Slice(details, func(i, j int) bool {
		return details[i].UsagePercent > details[j].UsagePercent
	})
	return VdevDistribution{
		Vdevs:         details,
		AverageUsage:  avg,
		MaxDeviation:  maxDev,
		WarnThreshold: s.config.WarnUsageThreshold,
	}
}

func (s *Simulator) buildAfterDistribution(after PoolConfig, usedSize uint64) VdevDistribution {
	// Distribute used data proportionally across all vdevs
	if after.TotalSize == 0 || len(after.Vdevs) == 0 {
		return VdevDistribution{WarnThreshold: s.config.WarnUsageThreshold}
	}
	modified := make([]VdevConfig, len(after.Vdevs))
	copy(modified, after.Vdevs)
	for i := range modified {
		if after.TotalSize > 0 {
			fraction := float64(modified[i].Capacity) / float64(after.TotalSize)
			modified[i].Used = uint64(float64(usedSize) * fraction)
		}
	}
	pool := PoolConfig{Vdevs: modified, TotalSize: after.TotalSize, UsedSize: usedSize}
	return s.buildDistribution(pool)
}

func (s *Simulator) validate(result *RebalanceResult) {
	if result.CapacityUtilAfter > s.config.WarnUsageThreshold {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Pool will still be at %.1f%% capacity after rebalance — consider adding more storage",
				result.CapacityUtilAfter))
	}
	if result.DataMovementPercent > s.config.WarnDataMovementPercent {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("High data movement expected: %.1f%% of pool data will relocate — plan for degraded performance during rebalance",
				result.DataMovementPercent))
	}
	if result.EstimatedDuration > 24*time.Hour {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Estimated rebalance duration is %s — schedule during a maintenance window",
				result.DurationHuman))
	}
	result.Notes = append(result.Notes,
		"ZFS does not actively rebalance existing data — new writes will prefer less-full vdevs. This estimate models the ideal steady-state distribution.",
		"Throughput estimate assumes 50 MB/s sustained write speed on spinning disks. SSDs will be significantly faster.",
	)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "no data movement required"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if h < 24 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := h / 24
	hours := h % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
