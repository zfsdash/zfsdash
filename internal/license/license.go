package license

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var licenseKeyRegex = regexp.MustCompile(`^zfd_[a-f0-9]{32}$`)

var (
	ErrInvalidFormat    = errors.New("invalid license key format")
	ErrValidationFailed = errors.New("license validation failed")
)

type Plan string

const (
	PlanFree       Plan = "free"
	PlanCloud      Plan = "cloud"
	PlanEnterprise Plan = "enterprise"
)

type Info struct {
	Plan       Plan      `json:"plan"`
	Features   []string  `json:"features"`
	ValidUntil time.Time `json:"valid_until"`
	Valid      bool      `json:"valid"`
}

func (i Info) HasFeature(f string) bool {
	for _, feat := range i.Features {
		if feat == f {
			return true
		}
	}
	return false
}

type cache struct {
	info      Info
	expiresAt time.Time
}

type Manager struct {
	validationURL string
	mu            sync.RWMutex
	caches        map[string]cache
}

func NewManager(validationURL string) *Manager {
	return &Manager{
		validationURL: validationURL,
		caches:        make(map[string]cache),
	}
}

func (m *Manager) Validate(key string) (Info, error) {
	if key == "" {
		return Info{Plan: PlanFree, Valid: true, Features: []string{}}, nil
	}
	if !licenseKeyRegex.MatchString(key) {
		return Info{}, ErrInvalidFormat
	}
	m.mu.RLock()
	if c, ok := m.caches[key]; ok && time.Now().Before(c.expiresAt) {
		m.mu.RUnlock()
		return c.info, nil
	}
	m.mu.RUnlock()

	info, err := m.validateRemote(key)
	if err != nil {
		return Info{}, err
	}
	m.mu.Lock()
	m.caches[key] = cache{info: info, expiresAt: time.Now().Add(24 * time.Hour)}
	m.mu.Unlock()
	return info, nil
}

func (m *Manager) validateRemote(key string) (Info, error) {
	req, _ := http.NewRequest("GET", m.validationURL, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "ZFSdash/1.0")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info Info
	if err := json.Unmarshal(body, &info); err != nil {
		return Info{}, ErrValidationFailed
	}
	return info, nil
}
