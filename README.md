# ZFSdash

<p align="center">
  <img src="https://zfsdash.com/logo.svg" alt="ZFSdash" width="120" />
</p>

<p align="center">
  <strong>A lightweight, self-hosted ZFS management dashboard written in Go.</strong><br />
  Monitor pools, datasets, and snapshots in real-time. Zero cloud. Zero accounts. Just ZFS.
</p>

<p align="center">
  <a href="https://github.com/zfsdash/zfsdash/releases"><img src="https://img.shields.io/github/v/release/zfsdash/zfsdash?color=blue" alt="Latest Release" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-green" alt="License" /></a>
  <a href="https://goreportcard.com/report/github.com/zfsdash/zfsdash"><img src="https://goreportcard.com/badge/github.com/zfsdash/zfsdash" alt="Go Report Card" /></a>
</p>

---

## Features

- **Pool health dashboard** — capacity, status, error counts at a glance
- **Dataset tree view** — hierarchical drill-down with used/available/compression ratio
- **Snapshot management** — create, browse, and destroy snapshots from the UI
- **Scrub control** — start scrubs, monitor progress, view history
- **SMART data** — disk health status per pool vdev
- **Alert engine** — email + webhook notifications with configurable thresholds
- **History tracking** — scrub runs and pool snapshots stored in SQLite
- **Three backend modes** — local, SSH, TrueNAS REST API
- **Multi-host** — manage multiple ZFS hosts from one dashboard
- **Single binary** — no runtime dependencies, embeds the UI

---

## Installation

### Via `go install` (Recommended)

Requires Go 1.21+.

```bash
go install github.com/zfsdash/zfsdash@latest
```

The binary lands at `$(go env GOPATH)/bin/zfsdash`. Make sure that's in your `$PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### Building from Source

```bash
git clone https://github.com/zfsdash/zfsdash.git
cd zfsdash
go build -o zfsdash .
./zfsdash -config config.yaml
```

### Docker

```bash
docker build -t zfsdash:latest .
docker run -d \
  --name zfsdash \
  -p 8080:8080 \
  -v /path/to/config.yaml:/etc/zfsdash/config.yaml:ro \
  zfsdash:latest
```

---

## Quick Start

### 1. Create a config file

**Local mode** — run ZFSdash directly on your ZFS host:

```yaml
# config.yaml
server:
  bind: "0.0.0.0:8080"

hosts:
  - name: local
    mode: local
```

Then open **http://localhost:8080** in your browser. Done.

### 2. Start ZFSdash

```bash
zfsdash -config config.yaml
```

---

## Configuration

Full example with all three modes:

```yaml
# config.yaml
server:
  bind: "0.0.0.0:8080"
  read_only: false  # set true to disable write operations

database:
  path: "/var/lib/zfsdash/zfsdash.db"  # SQLite history database

alerts:
  capacity_threshold: 85  # alert when pool used% exceeds this
  check_interval: "5m"
  email:
    enabled: false
    smtp_host: "smtp.example.com"
    smtp_port: 587
    from: "zfsdash@example.com"
    to: "admin@example.com"
    username: "zfsdash@example.com"
    password: "yourpassword"
  webhook:
    enabled: false
    url: "https://hooks.slack.com/services/..."
    cooldown: "1h"

hosts:
  # Mode 1: Local — ZFSdash runs ON the ZFS host
  - name: local
    mode: local

  # Mode 2: SSH — ZFSdash connects to a remote ZFS host
  - name: nas-01
    mode: ssh
    host: "192.168.1.100"
    port: 22
    user: "root"
    private_key: "/root/.ssh/id_ed25519"

  # Mode 3: TrueNAS REST API
  - name: truenas-main
    mode: truenas
    url: "https://truenas.local"
    api_key: "1-xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    verify_ssl: false
```

---

## Systemd Service

To run ZFSdash as a system service:

```bash
# Copy binary
sudo cp zfsdash /usr/local/bin/zfsdash

# Create config directory
sudo mkdir -p /etc/zfsdash
sudo cp config.yaml /etc/zfsdash/config.yaml

# Create data directory
sudo mkdir -p /var/lib/zfsdash

# Create service user
sudo useradd -r -s /bin/false zfsdash
sudo chown -R zfsdash:zfsdash /var/lib/zfsdash

# Install the service
sudo cp zfsdash.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zfsdash
```

**zfsdash.service:**

```ini
[Unit]
Description=ZFSdash — ZFS Management Dashboard
After=network.target

[Service]
Type=simple
User=zfsdash
ExecStart=/usr/local/bin/zfsdash -config /etc/zfsdash/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

---

## API

ZFSdash exposes a REST API at `/api/`:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/hosts` | List all configured hosts |
| `GET` | `/api/hosts/{host}/pools` | List pools on a host |
| `GET` | `/api/hosts/{host}/pools/{pool}/datasets` | List datasets |
| `GET` | `/api/hosts/{host}/pools/{pool}/snapshots` | List snapshots |
| `POST` | `/api/hosts/{host}/pools/{pool}/snapshots` | Create snapshot |
| `DELETE` | `/api/hosts/{host}/pools/{pool}/snapshots/{snap}` | Destroy snapshot |
| `POST` | `/api/hosts/{host}/pools/{pool}/scrub` | Start scrub |
| `GET` | `/api/hosts/{host}/pools/{pool}/history` | Scrub history |
| `GET` | `/api/hosts/{host}/smart` | SMART disk data |

---

## Enterprise

ZFSdash is free and open source under AGPL-3.0.

An **Enterprise edition** with AI-powered disk failure prediction, anomaly detection, Slack/PagerDuty integrations, RBAC, and SSO is coming soon.

→ [Learn more at zfsdash.com](https://zfsdash.com)

---

## Contributing

Pull requests welcome. Please open an issue first for major changes.

```bash
git clone https://github.com/zfsdash/zfsdash.git
cd zfsdash
go test ./...
go build .
```

---

## License

AGPL-3.0 — see [LICENSE](LICENSE) for details.

Sponsored by [ThoughtWave](https://thoughtwave.com).
