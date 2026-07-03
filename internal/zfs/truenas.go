package zfs

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TrueNASCollector collects ZFS data from a TrueNAS instance via REST API.
type TrueNASCollector struct {
	baseURL string
	client  *http.Client
	apiKey  string
	username string
	password string
}

// NewTrueNASCollector creates a TrueNAS REST API collector.
func NewTrueNASCollector(cfg CollectorConfig) *TrueNASCollector {
	baseURL := cfg.Host
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &TrueNASCollector{
		baseURL:  baseURL,
		apiKey:   cfg.APIKey,
		username: cfg.Username,
		password: cfg.Password,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

func (t *TrueNASCollector) authHeader() string {
	if t.apiKey != "" {
		return "Bearer " + t.apiKey
	}
	if t.username != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(t.username + ":" + t.password))
		return "Basic " + creds
	}
	return ""
}

func (t *TrueNASCollector) get(ctx context.Context, path string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/api/v2.0"+path, nil)
	if err != nil {
		return err
	}
	if auth := t.authHeader(); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("truenas GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("truenas %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// TrueNAS API response types
type tnPool struct {
	Name   string `json:"name"`
	Health string `json:"health"`
	Status string `json:"status"`
	Size   int64  `json:"size"`
	Allocated int64 `json:"allocated"`
	Free   int64  `json:"free"`
}

type tnDataset struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Mountpoint string `json:"mountpoint"`
	Used       struct{ Value string } `json:"used"`
	Available  struct{ Value string } `json:"available"`
	Referenced struct{ Value string } `json:"referenced"`
}

type tnSnapshot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Dataset string `json:"dataset"`
	Used    struct{ Value string } `json:"used"`
	Referenced struct{ Value string } `json:"referenced"`
	CreateTXG string `json:"createtxg"`
}

func (t *TrueNASCollector) CollectPools(ctx context.Context) ([]*Pool, error) {
	var raw []tnPool
	if err := t.get(ctx, "/pool", &raw); err != nil {
		return nil, err
	}
	pools := make([]*Pool, 0, len(raw))
	for _, r := range raw {
		var cap int
		if r.Size > 0 {
			cap = int(r.Allocated * 100 / r.Size)
		}
		pools = append(pools, &Pool{
			Name:      r.Name,
			Health:    r.Health,
			Size:      r.Size,
			Allocated: r.Allocated,
			Free:      r.Free,
			Capacity:  cap,
			Timestamp: time.Now(),
		})
	}
	return pools, nil
}

func (t *TrueNASCollector) CollectDatasets(ctx context.Context, poolName string) ([]*Dataset, error) {
	var raw []tnDataset
	path := "/pool/dataset"
	if poolName != "" {
		path += "?pool=" + poolName
	}
	if err := t.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	datasets := make([]*Dataset, 0, len(raw))
	for _, r := range raw {
		datasets = append(datasets, &Dataset{
			Name:       r.Name,
			Type:       strings.ToLower(r.Type),
			Mountpoint: r.Mountpoint,
			Timestamp:  time.Now(),
		})
	}
	return datasets, nil
}

func (t *TrueNASCollector) CollectSnapshots(ctx context.Context, datasetName string) ([]*Snapshot, error) {
	var raw []tnSnapshot
	path := "/zfs/snapshot"
	if datasetName != "" {
		path += "?dataset=" + datasetName
	}
	if err := t.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	snaps := make([]*Snapshot, 0, len(raw))
	for _, r := range raw {
		pool := ""
		if parts := strings.SplitN(r.Dataset, "/", 2); len(parts) > 0 {
			pool = parts[0]
		}
		snaps = append(snaps, &Snapshot{
			Name:      r.Name,
			Pool:      pool,
			Dataset:   r.Dataset,
			Timestamp: time.Now(),
		})
	}
	return snaps, nil
}

func (t *TrueNASCollector) CollectScrubStatus(ctx context.Context, poolName string) (*Scrub, error) {
	// TrueNAS scrub status is embedded in pool detail
	var raw []tnPool
	if err := t.get(ctx, "/pool", &raw); err != nil {
		return nil, err
	}
	return &Scrub{State: "none"}, nil
}

func (t *TrueNASCollector) CollectVdevTree(ctx context.Context, poolName string) (*Vdev, error) {
	return &Vdev{Name: poolName, Type: "root", State: "ONLINE"}, nil
}

func (t *TrueNASCollector) CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error) {
	return map[string]*SMARTData{}, nil
}

func (t *TrueNASCollector) CreateSnapshot(ctx context.Context, datasetName, snapshotName string) error {
	return fmt.Errorf("TrueNAS snapshot creation not yet implemented")
}

func (t *TrueNASCollector) DestroySnapshot(ctx context.Context, snapshotName string) error {
	return fmt.Errorf("TrueNAS snapshot deletion not yet implemented")
}

func (t *TrueNASCollector) StartScrub(ctx context.Context, poolName string) error {
	return fmt.Errorf("TrueNAS scrub start not yet implemented")
}

func (t *TrueNASCollector) Close() error { return nil }
