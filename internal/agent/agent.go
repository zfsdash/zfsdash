package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// MetricsPayload is sent to app.zfsdash.com every 60 seconds.
type MetricsPayload struct {
	HostID    string        `json:"host_id"`
	Hostname  string        `json:"hostname"`
	Timestamp int64         `json:"timestamp"`
	Pools     []PoolMetrics `json:"pools"`
	Agent     AgentInfo     `json:"agent"`
	License   string        `json:"license_key"`
}

type PoolMetrics struct {
	Name      string  `json:"name"`
	Size      uint64  `json:"size"`
	Allocated uint64  `json:"allocated"`
	Free      uint64  `json:"free"`
	Capacity  float64 `json:"capacity"`
	Health    string  `json:"health"`
	Datasets  int     `json:"datasets"`
	Snapshots int     `json:"snapshots"`
	Errors    int     `json:"errors"`
}

type AgentInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Uptime  int64  `json:"uptime"`
}

type CommandResponse struct {
	Commands  []Command `json:"commands"`
	NextPoll  int       `json:"next_poll_seconds"`
}

type Command struct {
	ID     string                 `json:"id"`
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

type Agent struct {
	hostID       string
	hostname     string
	cloudURL     string
	licenseKey   string
	pollInterval time.Duration
	version      string
	startTime    time.Time
	client       *http.Client
	metricsFunc  func(ctx context.Context) ([]PoolMetrics, error)
	commandFunc  func(cmd Command) error
}

func New(
	hostID, hostname, cloudURL, licenseKey, version string,
	metricsFunc func(ctx context.Context) ([]PoolMetrics, error),
	commandFunc func(cmd Command) error,
) *Agent {
	return &Agent{
		hostID:       hostID,
		hostname:     hostname,
		cloudURL:     cloudURL,
		licenseKey:   licenseKey,
		pollInterval: 60 * time.Second,
		version:      version,
		startTime:    time.Now(),
		metricsFunc:  metricsFunc,
		commandFunc:  commandFunc,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *Agent) Start(ctx context.Context) error {
	slog.Info("agent starting", "host_id", a.hostID, "cloud_url", a.cloudURL)
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()
	// Send immediately on start
	if err := a.poll(ctx); err != nil {
		slog.Warn("initial poll failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.poll(ctx); err != nil {
				slog.Error("poll failed", "err", err)
			}
		}
	}
}

func (a *Agent) poll(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	pools, err := a.metricsFunc(ctx)
	if err != nil {
		return fmt.Errorf("collect metrics: %w", err)
	}

	hostname := a.hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	payload := MetricsPayload{
		HostID:    a.hostID,
		Hostname:  hostname,
		Timestamp: time.Now().Unix(),
		Pools:     pools,
		License:   a.licenseKey,
		Agent: AgentInfo{
			Version: a.version,
			OS:      "linux",
			Arch:    "amd64",
			Uptime:  int64(time.Since(a.startTime).Seconds()),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cloudURL+"/api/agent/heartbeat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.licenseKey)
	req.Header.Set("User-Agent", "zfsdash-agent/"+a.version)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	var cmdResp CommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
		return nil // non-fatal
	}

	if cmdResp.NextPoll > 0 && cmdResp.NextPoll <= 3600 {
		a.pollInterval = time.Duration(cmdResp.NextPoll) * time.Second
	}

	for _, cmd := range cmdResp.Commands {
		if err := a.commandFunc(cmd); err != nil {
			slog.Error("command failed", "id", cmd.ID, "action", cmd.Action, "err", err)
		} else {
			slog.Info("command executed", "id", cmd.ID, "action", cmd.Action)
		}
	}

	return nil
}
