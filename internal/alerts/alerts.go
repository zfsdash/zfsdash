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

// Config holds alerting configuration.
type Config struct {
	Email   EmailConfig   `yaml:"email"`
	Webhook WebhookConfig `yaml:"webhook"`
	Rules   RulesConfig   `yaml:"rules"`
}

// EmailConfig holds SMTP settings.
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTP     string `yaml:"smtp"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
}

// WebhookConfig holds webhook settings.
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

// RulesConfig holds alert thresholds.
type RulesConfig struct {
	PoolCapacityWarn     float64 `yaml:"pool_capacity_warn"`
	PoolCapacityCrit     float64 `yaml:"pool_capacity_crit"`
	PoolDegradedEnabled  bool    `yaml:"pool_degraded_enabled"`
	SMARTFailedEnabled   bool    `yaml:"smart_failed_enabled"`
	CooldownMinutes      int     `yaml:"cooldown_minutes"`
}

// Manager handles alert sending with cooldown.
type Manager struct {
	cfg      Config
	mu       sync.Mutex
	cooldown map[string]time.Time
}

// New creates a new Manager.
func New(cfg Config) *Manager {
	if cfg.Rules.CooldownMinutes == 0 {
		cfg.Rules.CooldownMinutes = 60
	}
	if cfg.Rules.PoolCapacityWarn == 0 {
		cfg.Rules.PoolCapacityWarn = 80
	}
	if cfg.Rules.PoolCapacityCrit == 0 {
		cfg.Rules.PoolCapacityCrit = 90
	}
	return &Manager{
		cfg:      cfg,
		cooldown: make(map[string]time.Time),
	}
}

// CheckPools evaluates pool health and fires alerts.
func (m *Manager) CheckPools(host string, pools []*zfs.Pool) {
	for _, p := range pools {
		if m.cfg.Rules.PoolDegradedEnabled && p.Health != "ONLINE" && p.Health != "" {
			m.fire(fmt.Sprintf("%s:%s:health", host, p.Name),
				fmt.Sprintf("[ZFSdash] Pool %s on %s is %s", p.Name, host, p.Health),
				fmt.Sprintf("Pool %s on host %s has health status: %s", p.Name, host, p.Health))
		}
		if p.Capacity >= m.cfg.Rules.PoolCapacityCrit {
			m.fire(fmt.Sprintf("%s:%s:capacity:crit", host, p.Name),
				fmt.Sprintf("[ZFSdash] CRITICAL: Pool %s on %s is %.1f%% full", p.Name, host, p.Capacity),
				fmt.Sprintf("Pool %s on %s has reached %.1f%% capacity (threshold: %.1f%%)", p.Name, host, p.Capacity, m.cfg.Rules.PoolCapacityCrit))
		} else if p.Capacity >= m.cfg.Rules.PoolCapacityWarn {
			m.fire(fmt.Sprintf("%s:%s:capacity:warn", host, p.Name),
				fmt.Sprintf("[ZFSdash] WARNING: Pool %s on %s is %.1f%% full", p.Name, host, p.Capacity),
				fmt.Sprintf("Pool %s on %s has reached %.1f%% capacity (threshold: %.1f%%)", p.Name, host, p.Capacity, m.cfg.Rules.PoolCapacityWarn))
		}
	}
}

// CheckSMART evaluates SMART health and fires alerts.
func (m *Manager) CheckSMART(host string, data []*zfs.SMARTData) {
	if !m.cfg.Rules.SMARTFailedEnabled {
		return
	}
	for _, d := range data {
		if d.Health == "FAILED" {
			m.fire(fmt.Sprintf("%s:%s:smart", host, d.Device),
				fmt.Sprintf("[ZFSdash] SMART FAILED: %s on %s", d.Device, host),
				fmt.Sprintf("Device %s (%s serial %s) on %s has failed SMART health check", d.Device, d.Model, d.Serial, host))
		}
	}
}

func (m *Manager) fire(key, subject, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cooldownDur := time.Duration(m.cfg.Rules.CooldownMinutes) * time.Minute
	if last, ok := m.cooldown[key]; ok && time.Since(last) < cooldownDur {
		return
	}
	m.cooldown[key] = time.Now()
	if m.cfg.Email.Enabled {
		go m.sendEmail(subject, body)
	}
	if m.cfg.Webhook.Enabled {
		go m.sendWebhook(subject, body)
	}
}

func (m *Manager) sendEmail(subject, body string) {
	auth := smtp.PlainAuth("", m.cfg.Email.Username, m.cfg.Email.Password, m.cfg.Email.SMTP)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		m.cfg.Email.From, m.cfg.Email.To, subject, body)
	addr := fmt.Sprintf("%s:%d", m.cfg.Email.SMTP, m.cfg.Email.Port)
	if err := smtp.SendMail(addr, auth, m.cfg.Email.From, strings.Split(m.cfg.Email.To, ","), []byte(msg)); err != nil {
		log.Printf("[alerts] send email: %v", err)
	}
}

func (m *Manager) sendWebhook(subject, body string) {
	payload, _ := json.Marshal(map[string]string{"title": subject, "text": body})
	resp, err := http.Post(m.cfg.Webhook.URL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[alerts] webhook: %v", err)
		return
	}
	_ = resp.Body.Close()
}
