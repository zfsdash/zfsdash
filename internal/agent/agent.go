// Package agent implements the ZFSdash agent mode.
// In agent mode the daemon registers with app.zfsdash.com and sends
// periodic telemetry (pool health, capacity, datasets, SMART data).
// The agent uses exponential backoff for all network calls and fails
// gracefully — if the server is unreachable, local operation continues.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	defaultServerURL = "https://app.zfsdash.com"
	defaultInterval  = 60 * time.Second
	maxBackoff       = 5 * time.Minute
	initialBackoff   = 5 * time.Second
	registerEndpoint = "/api/agent/register"
	telemetryEndpoint = "/api/agent/telemetry"
	httpTimeout      = 30 * time.Second
)

// PoolSummary is the per-pool payload sent in each telemetry report.
type PoolSummary struct {
	Name     string           `json:"name"`
	Health   string           `json:"health"`
	Used     uint64           `json:"used_bytes"`
	Total    uint64           `json:"total_bytes"`
	Free     uint64           `json:"free_bytes"`
	Capacity float64          `json:"capacity_pct"`
	Datasets []DatasetSummary `json:"datasets"`
	Scrub    *ScrubSummary    `json:"scrub,omitempty"`
}

// DatasetSummary is the per-dataset payload.
type DatasetSummary struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Used  uint64 `json:"used_bytes"`
	Avail uint64 `json:"avail_bytes"`
	Refer uint64 `json:"refer_bytes"`
}

// ScrubSummary is the scrub state for a pool.
type ScrubSummary struct {
	State  string `json:"state"`
	Errors uint64 `json:"errors"`
}

// SMARTSummary holds per-device SMART data.
type SMARTSummary struct {
	Device  string `json:"device"`
	Health  string `json:"health"`
	Temp    int    `json:"temp_celsius,omitempty"`
	ReallocatedSectors uint64 `json:"reallocated_sectors,omitempty"`
}

// TelemetryPayload is the full body sent to /api/agent/telemetry.
type TelemetryPayload struct {
	Hostname  string         `json:"hostname"`
	Platform  string         `json:"platform"`
	Version   string         `json:"version"`
	Pools     []PoolSummary  `json:"pools"`
	SMART     []SMARTSummary `json:"smart"`
	Timestamp time.Time      `json:"timestamp"`
}

// RegisterRequest is sent to /api/agent/register.
type RegisterRequest struct {
	Hostname string `json:"hostname"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
}

// RegisterResponse is the expected response from /api/agent/register.
type RegisterResponse struct {
	SecretToken string `json:"secret_token"`
	AgentID     string `json:"agent_id"`
	Message     string `json:"message,omitempty"`
}

// Collector is the interface the agent uses to gather ZFS data.
// This matches the methods on zfs.LocalCollector / SSHCollector.
type Collector interface {
	CollectPools(ctx context.Context) ([]PoolData, error)
	CollectDatasets(ctx context.Context, pool string) ([]DatasetData, error)
	CollectScrubStatus(ctx context.Context, pool string) (*ScrubData, error)
	CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error)
}

// PoolData is the minimal pool interface the agent needs.
type PoolData struct {
	Name      string
	Health    string
	Size      int64
	Allocated int64
	Free      int64
	Capacity  int
}

// DatasetData is the minimal dataset interface the agent needs.
type DatasetData struct {
	Name       string
	Type       string
	Used       int64
	Available  int64
	Referenced int64
}

// ScrubData is the minimal scrub interface the agent needs.
type ScrubData struct {
	State  string
	Errors uint64
}

// SMARTData is the minimal SMART interface the agent needs.
type SMARTData struct {
	Device             string
	HealthStatus       string
	Temperature        int
	ReallocatedSectors uint64
}

// Agent registers with app.zfsdash.com and sends periodic telemetry.
type Agent struct {
	ServerURL   string
	SecretToken string // obtained during Register(); can also be pre-set from config
	AgentID     string
	Hostname    string
	Platform    string
	Version     string
	collector   ZFSCollector
	interval    time.Duration
	httpClient  *http.Client
	mu          sync.Mutex
	registered  bool
}

// ZFSCollector is the subset of zfs.Collector methods needed by the agent.
// Using a separate interface avoids an import cycle with internal/zfs.
type ZFSCollector interface {
	CollectPoolsRaw(ctx context.Context) (interface{}, error)
}

// Config holds configuration for the agent.
type Config struct {
	ServerURL   string
	SecretToken string // pre-registered token; if empty, Register() is called
	Hostname    string // defaults to os.Hostname()
	Version     string
	Interval    time.Duration // defaults to 60s
}

// New creates an Agent. Call Register() then Run().
func New(cfg Config) (*Agent, error) {
	hostname := cfg.Hostname
	if hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("get hostname: %w", err)
		}
		hostname = h
	}

	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = defaultServerURL
	}

	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultInterval
	}

	platform := runtime.GOOS

	return &Agent{
		ServerURL:   serverURL,
		SecretToken: cfg.SecretToken,
		Hostname:    hostname,
		Platform:    platform,
		Version:     cfg.Version,
		interval:    interval,
		registered:  cfg.SecretToken != "",
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}, nil
}

// Register calls POST /api/agent/register and stores the returned secret token.
// It uses exponential backoff and will retry until ctx is cancelled.
func (a *Agent) Register(ctx context.Context) error {
	req := RegisterRequest{
		Hostname: a.Hostname,
		Platform: a.Platform,
		Version:  a.Version,
	}

	var resp RegisterResponse
	err := a.postWithBackoff(ctx, registerEndpoint, "", req, &resp)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	a.mu.Lock()
	a.SecretToken = resp.SecretToken
	a.AgentID = resp.AgentID
	a.registered = true
	a.mu.Unlock()

	slog.Info("agent registered", "agent_id", resp.AgentID, "hostname", a.Hostname)
	return nil
}

// Run starts the telemetry loop. It blocks until ctx is cancelled.
// collectFn is called each interval to gather pool data.
func (a *Agent) Run(ctx context.Context, collectFn func(ctx context.Context) (*TelemetryPayload, error)) {
	slog.Info("agent telemetry loop started", "interval", a.interval, "server", a.ServerURL)

	// Send immediately on start, then on interval.
	a.sendTelemetry(ctx, collectFn)

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent telemetry loop stopped")
			return
		case <-ticker.C:
			a.sendTelemetry(ctx, collectFn)
		}
	}
}

func (a *Agent) sendTelemetry(
	ctx context.Context,
	collectFn func(ctx context.Context) (*TelemetryPayload, error),
) {
	payload, err := collectFn(ctx)
	if err != nil {
		slog.Warn("agent: collect failed", "err", err)
		return
	}
	payload.Hostname = a.Hostname
	payload.Platform = a.Platform
	payload.Version = a.Version
	payload.Timestamp = time.Now().UTC()

	a.mu.Lock()
	token := a.SecretToken
	a.mu.Unlock()

	var result map[string]interface{}
	if err := a.postOnce(ctx, telemetryEndpoint, token, payload, &result); err != nil {
		slog.Warn("agent: telemetry send failed", "err", err)
		return
	}
	slog.Debug("agent: telemetry sent", "pools", len(payload.Pools))
}

// postWithBackoff posts to the given endpoint with exponential backoff until
// ctx is cancelled or the request succeeds.
func (a *Agent) postWithBackoff(ctx context.Context, endpoint, token string, body, out interface{}) error {
	backoff := initialBackoff
	attempt := 0

	for {
		err := a.postOnce(ctx, endpoint, token, body, out)
		if err == nil {
			return nil
		}

		attempt++
		slog.Warn("agent: request failed, will retry",
			"endpoint", endpoint,
			"attempt", attempt,
			"backoff", backoff,
			"err", err,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff: 5s, 10s, 20s, 40s … capped at maxBackoff.
		backoff = time.Duration(math.Min(
			float64(backoff)*2,
			float64(maxBackoff),
		))
	}
}

// postOnce performs a single POST and decodes the JSON response.
func (a *Agent) postOnce(ctx context.Context, endpoint, token string, body, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.ServerURL+endpoint,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("zfsdash-agent/%s (%s)", a.Version, a.Platform))

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// IsRegistered returns whether the agent has a valid secret token.
func (a *Agent) IsRegistered() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.registered
}

// Token returns the current secret token (may be empty if not registered).
func (a *Agent) Token() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.SecretToken
}
