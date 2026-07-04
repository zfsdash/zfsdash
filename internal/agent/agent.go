package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	Token    string
	CloudURL string
	Interval time.Duration
}

type PoolStatus struct {
	Name   string `json:"name"`
	Health string `json:"health"`
	Size   string `json:"size"`
	Used   string `json:"used"`
}

type HeartbeatPayload struct {
	Token     string       `json:"token"`
	Hostname  string       `json:"hostname"`
	Timestamp time.Time    `json:"timestamp"`
	Pools     []PoolStatus `json:"pools"`
}

type Agent struct {
	config   Config
	client   *http.Client
	hostname string
}

func NewAgent(config Config) *Agent {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return &Agent{
		config:   config,
		hostname: hostname,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *Agent) Start(ctx context.Context) error {
	if a.config.Token == "" {
		return fmt.Errorf("agent token is required")
	}
	if a.config.CloudURL == "" {
		return fmt.Errorf("cloud URL is required")
	}
	if a.config.Interval <= 0 {
		a.config.Interval = 30 * time.Second
	}

	// Send immediate heartbeat
	if err := a.sendHeartbeat(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "initial heartbeat failed: %v\n", err)
	}

	ticker := time.NewTicker(a.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := a.sendHeartbeat(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
			}
		}
	}
}

func (a *Agent) collectPools(ctx context.Context) ([]PoolStatus, error) {
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "name,health,size,alloc")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("zpool list: %w", err)
	}

	var pools []PoolStatus
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pools = append(pools, PoolStatus{
			Name:   fields[0],
			Health: fields[1],
			Size:   fields[2],
			Used:   fields[3],
		})
	}
	return pools, nil
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	pools, err := a.collectPools(ctx)
	if err != nil {
		return fmt.Errorf("collect pools: %w", err)
	}

	payload := HeartbeatPayload{
		Token:     a.config.Token,
		Hostname:  a.hostname,
		Timestamp: time.Now().UTC(),
		Pools:     pools,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.CloudURL+"/api/agent/heartbeat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server rejected heartbeat: HTTP %d", resp.StatusCode)
	}
	return nil
}
