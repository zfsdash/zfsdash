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

// TrueNASConfig holds connection settings for a TrueNAS Scale/Core host.
type TrueNASConfig struct {
	URL      string
	APIKey   string
	Username string
	Password string
	Insecure bool
}

// TrueNASCollector collects ZFS data from a TrueNAS host via its REST API.
type TrueNASCollector struct {
	baseURL string
	client  *http.Client
	headers map[string]string
}

// NewTrueNASCollector creates a collector that reads from a TrueNAS Scale/Core API.
func NewTrueNASCollector(cfg TrueNASConfig) (*TrueNASCollector, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure}, //nolint:gosec
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	} else if cfg.Username != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.Password))
		headers["Authorization"] = "Basic " + creds
	}
	return &TrueNASCollector{
		baseURL: strings.TrimRight(cfg.URL, "/") + "/api/v2.0",
		client:  &http.Client{Transport: transport, Timeout: 30 * time.Second},
		headers: headers,
	}, nil
}

func (t *TrueNASCollector) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+path, nil)
	if err != nil {
		return err
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TrueNAS API %s: %d %s", path, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (t *TrueNASCollector) post(ctx context.Context, path string, body interface{}) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TrueNAS API POST %s: %d %s", path, resp.StatusCode, string(body))
	}
	return nil
}

type tnPool struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Size      uint64 `json:"size"`
	Allocated uint64 `json:"allocated"`
	Free      uint64 `json:"free"`
	Healthy   bool   `json:"healthy"`
	Scan      struct {
		State    string `json:"state"`
		Function string `json:"function"`
		Errors   uint64 `json:"errors"`
	} `json:"scan"`
}

func (t *TrueNASCollector) GetPools(ctx context.Context) ([]*Pool, error) {
	var raw []tnPool
	if err := t.get(ctx, "/pool", &raw); err != nil {
		return nil, err
	}
	var pools []*Pool
	for _, r := range raw {
		p := &Pool{
			Name:      r.Name,
			State:     r.Status,
			Health:    r.Status,
			Size:      r.Size,
			Allocated: r.Allocated,
			Free:      r.Free,
			UpdatedAt: time.Now(),
		}
		if p.Size > 0 {
			p.Capacity = float64(p.Allocated) / float64(p.Size) * 100
		}
		if !r.Healthy {
			p.State = "DEGRADED"
		}
		p.ScrubStatus = &ScrubStatus{State: r.Scan.State, Function: r.Scan.Function, Errors: r.Scan.Errors}
		pools = append(pools, p)
	}
	return pools, nil
}

type tnDataset struct {
	Name       string `json:"name"`
	Pool       string `json:"pool"`
	Type       string `json:"type"`
	Mounted    bool   `json:"mounted"`
	Mountpoint string `json:"mountpoint"`
	Used          struct{ Parsed uint64  `json:"parsed"` } `json:"used"`
	Available     struct{ Parsed uint64  `json:"parsed"` } `json:"available"`
	Referenced    struct{ Parsed uint64  `json:"parsed"` } `json:"referenced"`
	Compression   struct{ Parsed string  `json:"parsed"` } `json:"compression"`
	Compressratio struct{ Parsed float64 `json:"parsed"` } `json:"compressratio"`
	Dedup         struct{ Parsed bool    `json:"parsed"` } `json:"dedup"`
	Quota         struct{ Parsed uint64  `json:"parsed"` } `json:"quota"`
}

func (t *TrueNASCollector) GetDatasets(ctx context.Context, pool string) ([]*Dataset, error) {
	path := "/pool/dataset"
	if pool != "" {
		path += "?pool=" + pool
	}
	var raw []tnDataset
	if err := t.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	var out []*Dataset
	for _, r := range raw {
		out = append(out, &Dataset{
			Name: r.Name, Pool: r.Pool, Type: strings.ToLower(r.Type),
			Mounted: r.Mounted, MountPoint: r.Mountpoint,
			Used: r.Used.Parsed, Available: r.Available.Parsed, Referenced: r.Referenced.Parsed,
			Compression: r.Compression.Parsed, CompressRatio: r.Compressratio.Parsed,
			Dedup: r.Dedup.Parsed, Quota: r.Quota.Parsed, UpdatedAt: time.Now(),
		})
	}
	return out, nil
}

type tnSnap struct {
	Name       string `json:"name"`
	Pool       string `json:"pool"`
	Used       struct{ Parsed uint64 `json:"parsed"` } `json:"used"`
	Referenced struct{ Parsed uint64 `json:"parsed"` } `json:"referenced"`
}

func (t *TrueNASCollector) GetSnapshots(ctx context.Context, dataset string) ([]*Snapshot, error) {
	path := "/zfs/snapshot"
	if dataset != "" {
		path += "?dataset=" + dataset
	}
	var raw []tnSnap
	if err := t.get(ctx, path, &raw); err != nil {
		return nil, err
	}
	var out []*Snapshot
	for _, r := range raw {
		parts := strings.SplitN(r.Name, "@", 2)
		ds := ""
		if len(parts) == 2 {
			ds = parts[0]
		}
		out = append(out, &Snapshot{Name: r.Name, Dataset: ds, Pool: r.Pool, Used: r.Used.Parsed, Referenced: r.Referenced.Parsed, CreatedAt: time.Now()})
	}
	return out, nil
}

func (t *TrueNASCollector) GetSMARTData(ctx context.Context) ([]*SMARTData, error) {
	var raw []struct {
		Name        string `json:"name"`
		Serial      string `json:"serial"`
		Model       string `json:"model"`
		Temperature int    `json:"temperature"`
	}
	if err := t.get(ctx, "/disk", &raw); err != nil {
		return nil, nil
	}
	var out []*SMARTData
	for _, r := range raw {
		out = append(out, &SMARTData{Device: r.Name, Model: r.Model, Serial: r.Serial, Health: "UNKNOWN", Temperature: r.Temperature, UpdatedAt: time.Now()})
	}
	return out, nil
}

func (t *TrueNASCollector) CreateSnapshot(ctx context.Context, dataset, snapName string) error {
	return t.post(ctx, "/zfs/snapshot", map[string]string{"dataset": dataset, "name": snapName})
}
func (t *TrueNASCollector) DeleteSnapshot(ctx context.Context, fullName string) error {
	return t.post(ctx, "/zfs/snapshot/id/"+fullName+"/delete", nil)
}
func (t *TrueNASCollector) StartScrub(ctx context.Context, pool string) error {
	return t.post(ctx, "/pool/scrub", map[string]interface{}{"name": pool, "threshold": 35})
}
func (t *TrueNASCollector) StopScrub(ctx context.Context, pool string) error {
	return t.post(ctx, "/pool/scrub", map[string]interface{}{"name": pool, "action": "STOP"})
}
func (t *TrueNASCollector) Close() error { return nil }
