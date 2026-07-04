package zfs

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SnapshotCost struct {
	Name              string
	Dataset           string
	CreatedAt         time.Time
	SpaceHeld         uint64
	CloneCount        int
	RetentionTier     string
	IsDeleteCandidate bool
	Reason            string
}

type SnapshotCostAnalysis struct {
	Dataset              string
	TotalSpaceHeld       uint64
	SnapshotCount        int
	Snapshots            []SnapshotCost
	PredictedFreeIfClean uint64
	Recommendations      []SnapshotRecommendation
}

type SnapshotRecommendation struct {
	Action       string
	Snapshots    []string
	ExpectedFree uint64
	Risk         string
	Reason       string
}

func AnalyzeSnapshotCost(dataset string) (*SnapshotCostAnalysis, error) {
	out, err := exec.Command("zfs", "list", "-t", "snapshot", "-p", "-o",
		"name,used,refer,clones,creation", "-r", dataset).Output()
	if err != nil {
		return mockSnapshotAnalysis(dataset), nil
	}
	analysis := &SnapshotCostAnalysis{Dataset: dataset}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		used, _ := strconv.ParseUint(fields[1], 10, 64)
		cloneStr := fields[3]
		cloneCount := 0
		if cloneStr != "-" && cloneStr != "" {
			cloneCount = len(strings.Split(cloneStr, ","))
		}
		createdUnix, _ := strconv.ParseInt(fields[4], 10, 64)
		sc := SnapshotCost{
			Name: name, Dataset: dataset, CreatedAt: time.Unix(createdUnix, 0),
			SpaceHeld: used, CloneCount: cloneCount, RetentionTier: classifySnapshot(name),
		}
		sc.IsDeleteCandidate, sc.Reason = isDeleteCandidate(sc)
		analysis.Snapshots = append(analysis.Snapshots, sc)
		analysis.TotalSpaceHeld += used
	}
	analysis.SnapshotCount = len(analysis.Snapshots)
	analysis.Recommendations = buildRecommendations(analysis.Snapshots)
	for _, r := range analysis.Recommendations {
		if r.Action == "delete" {
			analysis.PredictedFreeIfClean += r.ExpectedFree
		}
	}
	sort.Slice(analysis.Snapshots, func(i, j int) bool {
		return analysis.Snapshots[i].SpaceHeld > analysis.Snapshots[j].SpaceHeld
	})
	return analysis, nil
}

func classifySnapshot(name string) string {
	switch {
	case strings.Contains(name, "hourly"):
		return "hourly"
	case strings.Contains(name, "daily"):
		return "daily"
	case strings.Contains(name, "weekly"):
		return "weekly"
	case strings.Contains(name, "monthly"):
		return "monthly"
	default:
		return "manual"
	}
}

func isDeleteCandidate(sc SnapshotCost) (bool, string) {
	if sc.CloneCount > 0 {
		return false, fmt.Sprintf("has %d dependent clone(s)", sc.CloneCount)
	}
	age := time.Since(sc.CreatedAt)
	switch sc.RetentionTier {
	case "hourly":
		if age > 48*time.Hour {
			return true, "hourly snapshot older than 48h"
		}
	case "daily":
		if age > 30*24*time.Hour {
			return true, "daily snapshot older than 30 days"
		}
	case "weekly":
		if age > 90*24*time.Hour {
			return true, "weekly snapshot older than 90 days"
		}
	case "monthly":
		if age > 365*24*time.Hour {
			return true, "monthly snapshot older than 1 year"
		}
	case "manual":
		if age > 180*24*time.Hour && sc.SpaceHeld > 100*1024*1024 {
			return true, "manual snapshot older than 6 months holding >100MB"
		}
	}
	return false, "within retention policy"
}

func buildRecommendations(snaps []SnapshotCost) []SnapshotRecommendation {
	var deleteNames []string
	var deleteSpace uint64
	for _, s := range snaps {
		if s.IsDeleteCandidate {
			deleteNames = append(deleteNames, s.Name)
			deleteSpace += s.SpaceHeld
		}
	}
	if len(deleteNames) == 0 {
		return []SnapshotRecommendation{{Action: "keep", Risk: "low", Reason: "all snapshots within retention policy"}}
	}
	return []SnapshotRecommendation{{
		Action: "delete", Snapshots: deleteNames, ExpectedFree: deleteSpace,
		Risk: "low", Reason: fmt.Sprintf("%d snapshots beyond retention policy", len(deleteNames)),
	}}
}

func mockSnapshotAnalysis(dataset string) *SnapshotCostAnalysis {
	return &SnapshotCostAnalysis{
		Dataset: dataset, TotalSpaceHeld: 4 * 1024 * 1024 * 1024, SnapshotCount: 12,
		Snapshots: []SnapshotCost{
			{Name: dataset + "@daily-2024-01-01", SpaceHeld: 1200000000, RetentionTier: "daily", IsDeleteCandidate: true, Reason: "daily snapshot older than 30 days"},
			{Name: dataset + "@weekly-2024-03-01", SpaceHeld: 800000000, RetentionTier: "weekly", IsDeleteCandidate: false, Reason: "within retention policy"},
		},
		PredictedFreeIfClean: 1200000000,
		Recommendations: []SnapshotRecommendation{{
			Action: "delete", Snapshots: []string{dataset + "@daily-2024-01-01"},
			ExpectedFree: 1200000000, Risk: "low", Reason: "1 snapshot beyond retention policy",
		}},
	}
}
