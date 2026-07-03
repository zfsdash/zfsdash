// Package license validates ZFSdash license keys against app.zfsdash.com.
// Validation results are cached locally so the daemon works offline for up
// to TTL duration. If the server is unreachable AND there is no cache, the
// free tier is assumed (fail-open behaviour).
package license

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	validationEndpoint = "/api/license/validate"
	httpTimeout        = 15 * time.Second
	defaultTTL         = 24 * time.Hour
	defaultServerURL   = "https://app.zfsdash.com"
)

// keyPattern is the expected format: ZFS-XXXX-XXXX-XXXX-XXXX
var keyPattern = regexp.MustCompile(`^ZFS-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$`)

// Plan represents the resolved license plan and its features.
type Plan struct {
	Name      string    `json:"name"`       // free | enterprise
	Features  []string  `json:"features"`   // slack_alerts, pagerduty, rbac, ai_predictions, audit_log
	ExpiresAt time.Time `json:"expires_at"` // zero value means no expiry (perpetual)
	Validated time.Time `json:"validated"`  // when was this last validated online
}

// Has returns true if the plan includes the named feature.
func (p *Plan) Has(feature string) bool {
	for _, f := range p.Features {
		if strings.EqualFold(f, feature) {
			return true
		}
	}
	return false
}

// IsExpired returns true if the plan has a non-zero expiry that is in the past.
func (p *Plan) IsExpired() bool {
	if p.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(p.ExpiresAt)
}

// freePlan is the default plan used when no key is provided or when the
// server is unreachable and no cache exists.
var freePlan = &Plan{
	Name:      "free",
	Features:  []string{},
	Validated: time.Now(),
}

// cacheFile is the on-disk cache entry.
type cacheEntry struct {
	Key       string    `json:"key"`
	Plan      Plan      `json:"plan"`
	CachedAt  time.Time `json:"cached_at"`
}

// Validator checks license keys against app.zfsdash.com.
type Validator struct {
	ServerURL string
	CacheFile string        // default: ~/.zfsdash/license.cache
	TTL       time.Duration // default: 24h

	mu    sync.RWMutex
	cache *cacheEntry
	client *http.Client
}

// New creates a Validator with sensible defaults.
func New(cacheDir string) *Validator {
	return &Validator{
		ServerURL: defaultServerURL,
		CacheFile: filepath.Join(cacheDir, "license.cache"),
		TTL:       defaultTTL,
		client:    &http.Client{Timeout: httpTimeout},
	}
}

// Validate checks the license key and returns the resolved Plan.
//
// Priority order:
//  1. If key is empty — return free plan.
//  2. If key format is invalid — return error.
//  3. If cache exists, is for this key, and is fresh — return cached plan.
//  4. Call server to validate; on success update cache and return plan.
//  5. If server is unreachable but cache exists (possibly stale) — log warning, return cached plan.
//  6. If server is unreachable and no cache — fail open, return free plan.
func (v *Validator) Validate(key string) (*Plan, error) {
	if key == "" {
		return freePlan, nil
	}

	if !IsValidFormat(key) {
		return nil, fmt.Errorf("invalid license key format (expected ZFS-XXXX-XXXX-XXXX-XXXX)")
	}

	// Load cache if not already in memory.
	v.mu.RLock()
	cache := v.cache
	v.mu.RUnlock()

	if cache == nil {
		loaded := v.loadCache()
		if loaded != nil {
			v.mu.Lock()
			v.cache = loaded
			cache = loaded
			v.mu.Unlock()
		}
	}

	// Fresh cache hit for the same key.
	if cache != nil && cache.Key == key && time.Since(cache.CachedAt) < v.TTL {
		plan := cache.Plan
		return &plan, nil
	}

	// Validate online.
	plan, err := v.validateOnline(key)
	if err != nil {
		// Server unreachable: fail open.
		slog.Warn("license: server unreachable, using fallback",
			"err", err,
			"has_cache", cache != nil,
		)
		if cache != nil && cache.Key == key {
			// Return stale cache rather than downgrading to free.
			plan := cache.Plan
			return &plan, nil
		}
		// No cache at all — fail open to free tier.
		return freePlan, nil
	}

	// Success — update cache.
	entry := &cacheEntry{
		Key:      key,
		Plan:     *plan,
		CachedAt: time.Now(),
	}
	v.mu.Lock()
	v.cache = entry
	v.mu.Unlock()
	v.saveCache(entry)

	return plan, nil
}

// validateOnline calls POST /api/license/validate on the server.
func (v *Validator) validateOnline(key string) (*Plan, error) {
	body, err := json.Marshal(map[string]string{"key": key})
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Post(
		v.ServerURL+validationEndpoint,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("license key not found or invalid (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Plan Plan `json:"plan"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	result.Plan.Validated = time.Now()
	return &result.Plan, nil
}

// Invalidate clears the in-memory and on-disk cache.
func (v *Validator) Invalidate() {
	v.mu.Lock()
	v.cache = nil
	v.mu.Unlock()
	_ = os.Remove(v.CacheFile)
}

// IsValidFormat returns true if the key matches ZFS-XXXX-XXXX-XXXX-XXXX.
func IsValidFormat(key string) bool {
	return keyPattern.MatchString(strings.ToUpper(strings.TrimSpace(key)))
}

// loadCache reads the cache file from disk.
func (v *Validator) loadCache() *cacheEntry {
	data, err := os.ReadFile(v.CacheFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("license: read cache", "err", err)
		}
		return nil
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		slog.Warn("license: parse cache", "err", err)
		return nil
	}
	return &entry
}

// saveCache persists the cache entry to disk.
func (v *Validator) saveCache(entry *cacheEntry) {
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(v.CacheFile), 0700); err != nil {
		slog.Warn("license: mkdir cache dir", "err", err)
		return
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		slog.Warn("license: marshal cache", "err", err)
		return
	}
	if err := os.WriteFile(v.CacheFile, data, 0600); err != nil {
		slog.Warn("license: write cache", "err", err)
	}
}
