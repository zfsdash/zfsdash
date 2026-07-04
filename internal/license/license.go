package license

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type License struct {
	Tier        string    `json:"tier"`
	Features    []string  `json:"features"`
	Valid        bool      `json:"valid"`
	ExpiresAt   time.Time `json:"expires_at"`
	ValidatedAt time.Time `json:"validated_at"`
}

func (l *License) HasFeature(f string) bool {
	for _, feat := range l.Features {
		if feat == f { return true }
	}
	return false
}

func free() *License {
	return &License{
		Tier:     "free",
		Features: []string{"local_hosts", "ssh_hosts", "truenas_hosts", "snapshots", "scrub", "basic_alerts"},
		Valid:    true, ExpiresAt: time.Now().AddDate(10, 0, 0), ValidatedAt: time.Now(),
	}
}

func cacheFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zfsdash", "license.json")
}

func Validate(ctx context.Context) *License {
	key := os.Getenv("ZFSDASH_LICENSE")
	if key == "" {
		home, _ := os.UserHomeDir()
		data, err := os.ReadFile(filepath.Join(home, ".zfsdash", "license.key"))
		if err == nil { key = string(data) }
	}
	if key == "" { return free() }

	// Try cache
	if f, err := os.Open(cacheFile()); err == nil {
		var l License
		if json.NewDecoder(f).Decode(&l) == nil && time.Since(l.ValidatedAt) < 24*time.Hour {
			f.Close(); return &l
		}
		f.Close()
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", "https://app.zfsdash.com/api/v1/license/validate", nil)
	req.Header.Set("X-License-Key", key)
	req.Header.Set("User-Agent", "zfsdash/1.0")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil { slog.Warn("license validation failed, free tier", "err", err); return free() }
	defer resp.Body.Close()
	if resp.StatusCode == 401 { slog.Warn("invalid license key"); return free() }

	var l License
	if json.NewDecoder(resp.Body).Decode(&l) != nil { return free() }
	l.ValidatedAt = time.Now(); l.Valid = true
	os.MkdirAll(filepath.Dir(cacheFile()), 0700)
	if f, err := os.Create(cacheFile()); err == nil { json.NewEncoder(f).Encode(l); f.Close() }
	slog.Info("license validated", "tier", l.Tier)
	return &l
}
