package chargeback

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DatasetUsage struct {
	Dataset         string    `json:"dataset"`
	Owner           string    `json:"owner"`
	LogicalUsedGB   float64   `json:"logical_used_gb"`
	PhysicalUsedGB  float64   `json:"physical_used_gb"`
	CompressRatio   float64   `json:"compress_ratio"`
	DeduprRatio     float64   `json:"dedup_ratio"`
	SnapshotUsedGB  float64   `json:"snapshot_used_gb"`
	ReadOpsTotal    int64     `json:"read_ops_total"`
	WriteOpsTotal   int64     `json:"write_ops_total"`
	ReadBytesTotal  int64     `json:"read_bytes_total"`
	WriteBytesTotal int64     `json:"write_bytes_total"`
	CollectedAt     time.Time `json:"collected_at"`
	Pool            string    `json:"pool"`
}

type ChargebackReport struct {
	Pool           string          `json:"pool"`
	PeriodStart    time.Time       `json:"period_start"`
	PeriodEnd      time.Time       `json:"period_end"`
	Datasets       []DatasetCharge `json:"datasets"`
	TotalChargeUSD float64         `json:"total_charge_usd"`
	GeneratedAt    time.Time       `json:"generated_at"`
}

type DatasetCharge struct {
	Dataset         string  `json:"dataset"`
	Owner           string  `json:"owner"`
	StorageGB       float64 `json:"storage_gb"`
	StorageCostUSD  float64 `json:"storage_cost_usd"`
	IOCostUSD       float64 `json:"io_cost_usd"`
	SnapshotCostUSD float64 `json:"snapshot_cost_usd"`
	DeduSavingsUSD  float64 `json:"dedup_savings_usd"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	PctOfPool       float64 `json:"pct_of_pool"`
}

type RateCard struct {
	StoragePerGBPerMonthUSD  float64 `json:"storage_per_gb_per_month_usd"`
	ReadPerMillionOpsUSD     float64 `json:"read_per_million_ops_usd"`
	WritePerMillionOpsUSD    float64 `json:"write_per_million_ops_usd"`
	SnapshotPerGBPerMonthUSD float64 `json:"snapshot_per_gb_per_month_usd"`
}

func DefaultRateCard() RateCard {
	return RateCard{
		StoragePerGBPerMonthUSD:  0.023,
		ReadPerMillionOpsUSD:     0.40,
		WritePerMillionOpsUSD:    5.00,
		SnapshotPerGBPerMonthUSD: 0.0125,
	}
}

type Manager struct {
	mu       sync.RWMutex
	history  []DatasetUsage
	rateCard RateCard
	maxAge   time.Duration
}

func NewManager() *Manager {
	return &Manager{
		rateCard: DefaultRateCard(),
		maxAge:   30 * 24 * time.Hour,
	}
}

func (m *Manager) Collect(ctx context.Context, pool string) ([]DatasetUsage, error) {
	cmd := exec.CommandContext(ctx, "zfs", "list", "-r", "-H", "-o",
		"name,used,usedbysnapshots,logicalused,compressratio,dedup,written", pool)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zfs list: %w", err)
	}
	var usages []DatasetUsage
	now := time.Now()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		name := fields[0]
		used := parseZFSSize(fields[1])
		snapUsed := parseZFSSize(fields[2])
		logUsed := parseZFSSize(fields[3])
		compRatio := parseRatio(fields[4])
		dedupRatio := parseRatio(fields[5])
		owner := getOwnerProperty(ctx, name)
		u := DatasetUsage{
			Dataset:        name,
			Owner:          owner,
			LogicalUsedGB:  float64(logUsed) / 1e9,
			PhysicalUsedGB: float64(used) / 1e9,
			CompressRatio:  compRatio,
			DeduprRatio:    dedupRatio,
			SnapshotUsedGB: float64(snapUsed) / 1e9,
			CollectedAt:    now,
			Pool:           pool,
		}
		usages = append(usages, u)
	}
	m.mu.Lock()
	m.history = append(m.history, usages...)
	cutoff := now.Add(-m.maxAge)
	filtered := m.history[:0]
	for _, u := range m.history {
		if u.CollectedAt.After(cutoff) {
			filtered = append(filtered, u)
		}
	}
	m.history = filtered
	m.mu.Unlock()
	return usages, nil
}

func (m *Manager) GenerateReport(pool string, start, end time.Time) ChargebackReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type agg struct {
		owner          string
		storageGBHours float64
		snapGBHours    float64
		readOps        int64
		writeOps       int64
		dedupRatio     float64
		samples        int
	}
	byDataset := map[string]*agg{}
	periodHours := end.Sub(start).Hours()
	for _, u := range m.history {
		if u.Pool != pool { continue }
		if u.CollectedAt.Before(start) || u.CollectedAt.After(end) { continue }
		a, ok := byDataset[u.Dataset]
		if !ok { a = &agg{owner: u.Owner}; byDataset[u.Dataset] = a }
		a.storageGBHours += u.PhysicalUsedGB
		a.snapGBHours += u.SnapshotUsedGB
		a.readOps += u.ReadOpsTotal
		a.writeOps += u.WriteOpsTotal
		a.dedupRatio += u.DeduprRatio
		a.samples++
	}
	var poolTotalGB float64
	for _, a := range byDataset {
		if a.samples > 0 { poolTotalGB += a.storageGBHours / float64(a.samples) }
	}
	monthFraction := periodHours / (30 * 24)
	var charges []DatasetCharge
	var totalCharge float64
	for dataset, a := range byDataset {
		if a.samples == 0 { continue }
		avgStorageGB := a.storageGBHours / float64(a.samples)
		avgSnapGB := a.snapGBHours / float64(a.samples)
		avgDedup := a.dedupRatio / float64(a.samples)
		storageCost := avgStorageGB * m.rateCard.StoragePerGBPerMonthUSD * monthFraction
		snapCost := avgSnapGB * m.rateCard.SnapshotPerGBPerMonthUSD * monthFraction
		ioCost := (float64(a.readOps)/1e6)*m.rateCard.ReadPerMillionOpsUSD + (float64(a.writeOps)/1e6)*m.rateCard.WritePerMillionOpsUSD
		var dedupSavings float64
		if avgDedup > 1.0 { savedGB := avgStorageGB * (1 - 1/avgDedup); dedupSavings = savedGB * m.rateCard.StoragePerGBPerMonthUSD * monthFraction }
		total := storageCost + snapCost + ioCost - dedupSavings
		pct := 0.0
		if poolTotalGB > 0 { pct = avgStorageGB / poolTotalGB * 100 }
		charges = append(charges, DatasetCharge{Dataset: dataset, Owner: a.owner, StorageGB: avgStorageGB, StorageCostUSD: storageCost, IOCostUSD: ioCost, SnapshotCostUSD: snapCost, DeduSavingsUSD: dedupSavings, TotalCostUSD: total, PctOfPool: pct})
		totalCharge += total
	}
	return ChargebackReport{Pool: pool, PeriodStart: start, PeriodEnd: end, Datasets: charges, TotalChargeUSD: totalCharge, GeneratedAt: time.Now()}
}

func (m *Manager) SetRateCard(rc RateCard) { m.mu.Lock(); m.rateCard = rc; m.mu.Unlock() }
func (m *Manager) GetRateCard() RateCard { m.mu.RLock(); defer m.mu.RUnlock(); return m.rateCard }

func getOwnerProperty(ctx context.Context, dataset string) string {
	cmd := exec.CommandContext(ctx, "zfs", "get", "-H", "-o", "value", "io.zfsdash:owner", dataset)
	out, err := cmd.Output()
	if err != nil { return "unassigned" }
	v := strings.TrimSpace(string(out))
	if v == "-" || v == "" { return "unassigned" }
	return v
}

func parseZFSSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" { return 0 }
	multipliers := map[byte]int64{'K': 1024, 'M': 1024*1024, 'G': 1024*1024*1024, 'T': 1024*1024*1024*1024}
	last := s[len(s)-1]
	if mul, ok := multipliers[last]; ok {
		v, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil { return 0 }
		return int64(v * float64(mul))
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseRatio(s string) float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "x")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

var _ = slog.Default
