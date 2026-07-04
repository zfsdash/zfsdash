package license

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	validateURL = "https://app.zfsdash.com/api/v1/license/validate"
	cacheTTL    = 24 * time.Hour
)

type License struct {
	Tier        string    `json:"tier"`        // free, cloud, enterprise
	Features    []string  `json:"features"`
	Valid       bool      `json:"valid"`
	ExpiresAt   time.Time `json:"expires_at"`
	ValidatedAt time.Time `json:"validated_at"`
}

func (l *License) HasFeature(f string) bool {
	for _, feat := range l.Features {
		if feat == f {
			return true
		}
	}
	return false
}

func freeLicense() *License {
	return &License{
		Tier:        "free",
		Features:    []string{"local_hosts", "ssh_hosts", "truenas_hosts", "snapshots", "scrub", "alerts_basic"},
		Valid:       true,
		ExpiresAt:   time.Now().AddDate(10, 0, 0),
		ValidatedAt: time.Now(),
	}
}

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zfsdash")
}

func cachePath() string {
	return filepath.Join(cacheDir(), "license.json")
}

func loadCached() (*License, error) {
	f, err := os.Open(cachePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var l License
	if err := json.NewDecoder(f).Decode(&l); err != nil {
		return nil, err
	}
	if time.Since(l.ValidatedAt) > cacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	return &l, nil
}

func saveCached(l *License) {
	if err := os.MkdirAll(cacheDir(), 0700); err != nil {
		return
	}
	f, err := os.Create(cachePath())
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(l)
}

func Validate(ctx context.Context) *License {
	// Check env var first
	key := os.Getenv("ZFSDASH_LICENSE")
	if key == "" {
		// Check file
		keyFile := filepath.Join(cacheDir(), "license.key")
		data, err := os.ReadFile(keyFile)
		if err == nil {
			key = string(data)
		}
	}

	if key == "" {
		slog.Debug("no license key found, running free tier")
		return freeLicense()
	}

	// Try cache
	if cached, err := loadCached(); err == nil {
		slog.Debug("license loaded from cache", "tier", cached.Tier)
		return cached
	}

	// Validate remotely
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		slog.Warn("license validate request build failed", "err", err)
		return freeLicense()
	}
	req.Header.Set("X-License-Key", key)
	req.Header.Set("User-Agent", "zfsdash/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("license validation network error, falling back to free tier", "err", err)
		return freeLicense()
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		slog.Warn("invalid license key, running free tier")
		return freeLicense()
	}

	var l License
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		slog.Warn("decode license response failed", "err", err)
		return freeLicense()
	}

	l.ValidatedAt = time.Now()
	l.Valid = true
	saveCached(&l)

	slog.Info("license validated", "tier", l.Tier, "features", l.Features)
	return &l
}
