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
	Health    string  `json:"health"`
	Size      uint64  `json:"size"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	UsedPct   float64 `json:"used_pct"`
	Timestamp int64   `json:"timestamp"`
}

type Agent struct {
	cfg    Config
	db     *sql.DB
	reg    struct{ AgentID, Secret string }
	client *http.Client
}

func New(cfg Config, db *sql.DB) *Agent {
	return &Agent{cfg: cfg, db: db, client: &http.Client{Timeout: 15 * time.Second}}
}

func (a *Agent) initSchema() {
	a.db.Exec(`CREATE TABLE IF NOT EXISTS agent_registration (agent_id TEXT, secret TEXT)`)
}

func (a *Agent) getOrRegister(ctx context.Context) error {
	a.initSchema()
	err := a.db.QueryRowContext(ctx, "SELECT agent_id, secret FROM agent_registration LIMIT 1").Scan(&a.reg.AgentID, &a.reg.Secret)
	if err == nil { return nil }
	body, _ := json.Marshal(map[string]string{"token": a.cfg.Token})
	req, _ := http.NewRequestWithContext(ctx, "POST", a.cfg.ServerURL+"/api/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil { return fmt.Errorf("register: %w", err) }
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&a.reg); err != nil { return fmt.Errorf("decode register: %w", err) }
	a.db.ExecContext(ctx, "INSERT INTO agent_registration VALUES (?,?)", a.reg.AgentID, a.reg.Secret)
	slog.Info("agent registered", "id", a.reg.AgentID)
	return nil
}

func (a *Agent) Run(ctx context.Context, collect func(context.Context) ([]PoolSummary, error)) error {
	if err := a.getOrRegister(ctx); err != nil { return err }
	if a.cfg.Interval == 0 { a.cfg.Interval = 60 * time.Second }
	slog.Info("agent running", "server", a.cfg.ServerURL, "interval", a.cfg.Interval)
	tick := time.NewTicker(a.cfg.Interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done(): return nil
		case <-tick.C:
			pools, err := collect(ctx)
			if err != nil { slog.Warn("collect failed", "err", err); continue }
			body, _ := json.Marshal(map[string]any{"agent_id": a.reg.AgentID, "secret": a.reg.Secret, "pools": pools})
			req, _ := http.NewRequestWithContext(ctx, "POST", a.cfg.ServerURL+"/api/agent/heartbeat", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := a.client.Do(req)
			if err != nil { slog.Warn("heartbeat failed", "err", err); continue }
			resp.Body.Close()
			slog.Debug("heartbeat sent", "pools", len(pools))
		}
	}
}

func DefaultServerURL() string {
	if u := os.Getenv("ZFSDASH_SERVER"); u != "" { return u }
	return "https://app.zfsdash.com"
}
