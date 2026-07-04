package agent

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Token     string
	ServerURL string
	Interval  time.Duration
}

type PoolSummary struct {
	Name      string  `json:"name"`
	Size      uint64  `json:"size"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Health    string  `json:"health"`
	UsedPct   float64 `json:"used_pct"`
	Timestamp int64   `json:"timestamp"`
}

type Collector interface {
	GetPools(ctx context.Context) ([]PoolSummary, error)
}

type Registration struct {
	AgentID string `json:"agent_id"`
	Secret  string `json:"secret"`
}

type Agent struct {
	cfg    Config
	db     *sql.DB
	reg    *Registration
	client *http.Client
	logger *slog.Logger
}

func New(cfg Config, db *sql.DB) *Agent {
	return &Agent{
		cfg:    cfg,
		db:     db,
		client: &http.Client{Timeout: 15 * time.Second},
		logger: slog.Default(),
	}
}

func (a *Agent) initSchema() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS agent_registration (
		agent_id TEXT NOT NULL,
		secret TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func (a *Agent) getOrRegister(ctx context.Context) (*Registration, error) {
	if err := a.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	var reg Registration
	err := a.db.QueryRowContext(ctx, "SELECT agent_id, secret FROM agent_registration LIMIT 1").
		Scan(&reg.AgentID, &reg.Secret)
	if err == nil {
		return &reg, nil
	}

	// Register with server
	body, _ := json.Marshal(map[string]string{"token": a.cfg.Token})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.ServerURL+"/api/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("register failed: HTTP %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}

	_, err = a.db.ExecContext(ctx, "INSERT INTO agent_registration (agent_id, secret) VALUES (?, ?)",
		reg.AgentID, reg.Secret)
	if err != nil {
		return nil, fmt.Errorf("save registration: %w", err)
	}

	a.logger.Info("agent registered", "agent_id", reg.AgentID)
	return &reg, nil
}

func (a *Agent) sendHeartbeat(ctx context.Context, pools []PoolSummary) error {
	payload := map[string]interface{}{
		"agent_id": a.reg.AgentID,
		"secret":   a.reg.Secret,
		"pools":    pools,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.ServerURL+"/api/agent/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("heartbeat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("heartbeat failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (a *Agent) Run(ctx context.Context, collect func(ctx context.Context) ([]PoolSummary, error)) error {
	var err error
	a.reg, err = a.getOrRegister(ctx)
	if err != nil {
		return fmt.Errorf("agent registration: %w", err)
	}

	if a.cfg.Interval == 0 {
		a.cfg.Interval = 60 * time.Second
	}

	a.logger.Info("agent running", "server", a.cfg.ServerURL, "interval", a.cfg.Interval)

	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pools, err := collect(ctx)
			if err != nil {
				a.logger.Warn("collect pools failed", "err", err)
				continue
			}
			if err := a.sendHeartbeat(ctx, pools); err != nil {
				a.logger.Warn("heartbeat failed", "err", err)
			} else {
				a.logger.Debug("heartbeat sent", "pools", len(pools))
			}
		}
	}
}

func DefaultServerURL() string {
	if u := os.Getenv("ZFSDASH_SERVER"); u != "" {
		return u
	}
	return "https://app.zfsdash.com"
}
