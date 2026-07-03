package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/zfsdash/zfsdash/internal/zfs"
)

// Config holds alert configuration
type Config struct {
	Enabled          bool     `yaml:"enabled"`
	CapacityWarning  int      `yaml:"capacity_warning"`  // % threshold, default 80
	CapacityCritical int      `yaml:"capacity_critical"` // % threshold, default 90
	CooldownMinutes  int      `yaml:"cooldown_minutes"`  // default 60
	Email            *EmailConfig   `yaml:"email,omitempty"`
	Webhooks         []string `yaml:"webhooks,omitempty"`
}

// EmailConfig holds SMTP configuration
type EmailConfig struct {
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
}

// Alert represents a triggered alert
type Alert struct {
	Level   string    `json:"level"` // warning, critical
	Host    string    `json:"host"`
	Pool    string    `json:"pool"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// Engine manages alert state and delivery
type Engine struct {
	cfg      *Config
	mu       sync.Mutex
	cooldown map[string]time.Time // key -> last alert time
}

// New creates a new alert engine
func New(cfg *Config) *Engine {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.CapacityWarning == 0 {
		cfg.CapacityWarning = 80
	}
	if cfg.CapacityCritical == 0 {
		cfg.CapacityCritical = 90
	}
	if cfg.CooldownMinutes == 0 {
		cfg.CooldownMinutes = 60
	}
	return &Engine{
		cfg:      cfg,
		cooldown: make(map[string]time.Time),
	}
}

// CheckPools evaluates all pools and fires alerts as needed
func (e *Engine) CheckPools(host string, pools []*zfs.Pool) {
	if !e.cfg.Enabled {
		return
	}
	for _, p := range pools {
		e.checkPool(host, p)
	}
}

func (e *Engine) checkPool(host string, p *zfs.Pool) {
	var alerts []Alert

	// Health check
	if p.Health != "ONLINE" && p.Health != "" {
		alerts = append(alerts, Alert{
			Level:   "critical",
			Host:    host,
			Pool:    p.Name,
			Message: fmt.Sprintf("Pool %s health is %s", p.Name, p.Health),
			Time:    time.Now(),
		})
	}

	// Capacity checks
	if p.Capacity >= e.cfg.CapacityCritical {
		alerts = append(alerts, Alert{
			Level:   "critical",
			Host:    host,
			Pool:    p.Name,
			Message: fmt.Sprintf("Pool %s capacity critical: %d%%", p.Name, p.Capacity),
			Time:    time.Now(),
		})
	} else if p.Capacity >= e.cfg.CapacityWarning {
		alerts = append(alerts, Alert{
			Level:   "warning",
			Host:    host,
			Pool:    p.Name,
			Message: fmt.Sprintf("Pool %s capacity warning: %d%%", p.Name, p.Capacity),
			Time:    time.Now(),
		})
	}

	// Scrub errors
	if p.Scrub != nil && p.Scrub.Errors > 0 {
		alerts = append(alerts, Alert{
			Level:   "critical",
			Host:    host,
			Pool:    p.Name,
			Message: fmt.Sprintf("Pool %s scrub found %d errors", p.Name, p.Scrub.Errors),
			Time:    time.Now(),
		})
	}

	for _, alert := range alerts {
		e.dispatch(alert)
	}
}

func (e *Engine) dispatch(a Alert) {
	key := fmt.Sprintf("%s:%s:%s", a.Host, a.Pool, a.Level)
	e.mu.Lock()
	last, ok := e.cooldown[key]
	if ok && time.Since(last) < time.Duration(e.cfg.CooldownMinutes)*time.Minute {
		e.mu.Unlock()
		return
	}
	e.cooldown[key] = time.Now()
	e.mu.Unlock()

	log.Printf("[ALERT][%s] %s: %s", a.Level, a.Host, a.Message)

	if e.cfg.Email != nil {
		go e.sendEmail(a)
	}
	for _, webhookURL := range e.cfg.Webhooks {
		go e.sendWebhook(webhookURL, a)
	}
}

func (e *Engine) sendEmail(a Alert) {
	cfg := e.cfg.Email
	body := fmt.Sprintf("Subject: [ZFSdash][%s] %s\r\n\r\n%s\r\nHost: %s\r\nPool: %s\r\nTime: %s\r\n",
		strings.ToUpper(a.Level), a.Message, a.Message, a.Host, a.Pool, a.Time.Format(time.RFC3339))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	if err := smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(body)); err != nil {
		log.Printf("[alerts] email send failed: %v", err)
	}
}

func (e *Engine) sendWebhook(url string, a Alert) {
	payload, _ := json.Marshal(a)
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[alerts] webhook %s failed: %v", url, err)
		return
	}
	resp.Body.Close()
}
