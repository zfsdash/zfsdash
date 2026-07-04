package license

import (
	"errors"
	"fmt"
	"time"
)

// Tier represents the license tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierCloud      Tier = "cloud"
	TierEnterprise Tier = "enterprise"
)

// License represents a parsed, validated license.
type License struct {
	Key       string
	Tier      Tier
	Email     string
	Features  []string
	ExpiresAt time.Time
	Valid     bool
}

// Features available per tier.
var tierFeatures = map[Tier][]string{
	TierFree: {
		"pools", "datasets", "snapshots", "scrub", "alerts_basic",
	},
	TierCloud: {
		"pools", "datasets", "snapshots", "scrub", "alerts_basic",
		"alerts_slack", "alerts_pagerduty", "multi_user", "cloud_sync",
	},
	TierEnterprise: {
		"pools", "datasets", "snapshots", "scrub", "alerts_basic",
		"alerts_slack", "alerts_pagerduty", "multi_user", "cloud_sync",
		"rbac", "sso", "audit_log", "ai_predictions", "support_sla",
	},
}

// Validate validates a license key.
// For now uses a simple prefix-based check; replace with JWT validation
// once the cloud app issues signed keys.
func Validate(key string) (*License, error) {
	if key == "" {
		return &License{Tier: TierFree, Valid: true, Features: tierFeatures[TierFree]}, nil
	}

	// Format: zd_<tier>_<token>
	// e.g. zd_cloud_abc123... or zd_ent_abc123...
	if len(key) < 8 || key[:3] != "zd_" {
		return nil, errors.New("invalid license key format")
	}

	var tier Tier
	switch {
	case len(key) > 9 && key[3:9] == "cloud_":
		tier = TierCloud
	case len(key) > 7 && key[3:7] == "ent_":
		tier = TierEnterprise
	default:
		return nil, fmt.Errorf("unknown license tier in key")
	}

	features, ok := tierFeatures[tier]
	if !ok {
		return nil, fmt.Errorf("unknown tier: %s", tier)
	}

	return &License{
		Key:      key,
		Tier:     tier,
		Features: features,
		Valid:    true,
		// Keys don't expire locally; cloud validates server-side
		ExpiresAt: time.Now().AddDate(1, 0, 0),
	}, nil
}

// HasFeature checks if a license includes a specific feature.
func (l *License) HasFeature(feature string) bool {
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// String returns a human-readable tier name.
func (l *License) String() string {
	switch l.Tier {
	case TierCloud:
		return "ZFSdash Cloud"
	case TierEnterprise:
		return "ZFSdash Enterprise"
	default:
		return "ZFSdash Free"
	}
}
