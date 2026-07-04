# ZFSdash

**Open source ZFS management dashboard.** Single binary. Runs on Linux and FreeBSD. No agents, no containers, no dependencies.

[![CI](https://github.com/zfsdash/zfsdash/actions/workflows/ci.yml/badge.svg)](https://github.com/zfsdash/zfsdash/actions/workflows/ci.yml)
[![Release](https://github.com/zfsdash/zfsdash/actions/workflows/release.yml/badge.svg)](https://github.com/zfsdash/zfsdash/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/zfsdash/zfsdash)](https://goreportcard.com/report/github.com/zfsdash/zfsdash)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

---

## Quick Install

```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

Or install a specific version:

```bash
ZFSDASH_VERSION=v0.1.0 curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

## Manual Install

```bash
# Using go install
go install github.com/zfsdash/zfsdash@latest

# Or download a pre-built binary from GitHub Releases
# https://github.com/zfsdash/zfsdash/releases/latest
```

## First Run

```bash
# Start the daemon (runs on :8080 by default)
zfsdash

# Or specify custom address and data directory
zfsdash -addr :9090 -data /var/lib/zfsdash

# Print version
zfsdash -version
```

Open `http://localhost:8080` to complete the setup wizard.

---

## Modes

### Local

Manages ZFS on the host where ZFSdash runs. Requires ZFS utilities (`zpool`, `zfs`) on the same machine.

```
zfsdash
# → Setup wizard → choose "Local" → done
```

### SSH

Connects to a remote host over SSH and runs ZFS commands remotely. The remote host needs ZFS utilities installed — ZFSdash itself does not need to run there.

```
# Configure via the setup wizard or API:
POST /api/hosts
{
  "mode": "ssh",
  "host": "192.168.1.10",
  "port": 22,
  "user": "root",
  "private_key": "..."
}
```

### TrueNAS SCALE / CORE

Connects to a TrueNAS instance via its REST API. No SSH required.

```
POST /api/hosts
{
  "mode": "truenas",
  "host": "192.168.1.20",
  "api_key": "your-truenas-api-key"
}
```

### Agent (Cloud Dashboard)

ZFSdash can run in agent mode, phoning home to [app.zfsdash.com](https://app.zfsdash.com) for a cloud-hosted dashboard with multi-host support, alerting, and AI-powered predictions.

```bash
# Register and start agent mode
zfsdash agent --token YOUR_TOKEN

# Or set via environment
ZFSDASH_AGENT_TOKEN=YOUR_TOKEN zfsdash agent
```

The agent collects pool health, capacity, dataset stats, and SMART data every 60 seconds, sending telemetry to `app.zfsdash.com` over HTTPS. It uses exponential backoff for all network calls and continues local operation if the cloud is unreachable.

---

## Setup Wizard API

On first run, ZFSdash walks you through setup via the web UI. You can also drive it programmatically:

```bash
# Check setup status
GET /api/setup/status
# → {"needsSetup": true, "step": "admin"}

# Create first admin account
POST /api/setup/admin
{"email": "admin@example.com", "password": "..."}

# Add first host
POST /api/setup/host
{"mode": "local"}

# Mark setup complete
POST /api/setup/complete

# Check ZFS availability
GET /api/setup/check-zfs
# → {"available": true, "version": "2.2.2", "pools": 3}
```

---

## Configuration

ZFSdash is configured via command-line flags and environment variables:

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `-addr` | `ZFSDASH_ADDR` | `:8080` | HTTP listen address |
| `-data` | `ZFSDASH_DATA_DIR` | `/var/lib/zfsdash` | SQLite database directory |
| `-version` | — | — | Print version and exit |

When run as a non-root user, the data directory defaults to `~/.zfsdash`.

### systemd (Linux)

The installer creates `/etc/systemd/system/zfsdash.service`. Customize via `/etc/zfsdash/config.env`:

```env
ZFSDASH_ADDR=:8080
ZFSDASH_DATA_DIR=/var/lib/zfsdash
```

```bash
systemctl status zfsdash
journalctl -u zfsdash -f
```

### rc.d (FreeBSD)

The installer creates `/etc/rc.d/zfsdash`. Configure in `/etc/rc.conf`:

```sh
zfsdash_enable="YES"
zfsdash_addr=":8080"
zfsdash_data="/var/lib/zfsdash"
```

```bash
service zfsdash status
service zfsdash restart
```

---

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| Linux x86_64 | ✅ Supported | Ubuntu 20.04+, Debian 11+, RHEL 8+ |
| Linux arm64 | ✅ Supported | Raspberry Pi 4+, AWS Graviton |
| FreeBSD amd64 | ✅ Supported | FreeBSD 13.0+ |
| macOS | 🧪 Experimental | For development only (no production ZFS) |

---

## License

ZFSdash is open source under the [AGPL-3.0 license](LICENSE).

Enterprise features (Slack/PagerDuty alerts, RBAC, AI predictions, audit log) are available at [zfsdash.com](https://zfsdash.com) with a commercial license key.

### License Key Format

Enterprise keys use the format `ZFS-XXXX-XXXX-XXXX-XXXX`. The daemon validates keys against `app.zfsdash.com` once per 24 hours and caches the result locally — offline operation continues for up to 24 hours if the server is unreachable.

```bash
# Set via environment
ZFSDASH_LICENSE_KEY=ZFS-XXXX-XXXX-XXXX-XXXX zfsdash

# Or via the web UI: Settings → License
```

---

## Development

```bash
# Clone
git clone https://github.com/zfsdash/zfsdash
cd zfsdash

# Run tests
go test ./...

# Build
go build -o zfsdash .

# Cross-compile
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -o zfsdash-freebsd-amd64 .
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o zfsdash-linux-arm64 .

# Run locally (non-root, uses ~/.zfsdash as data dir)
./zfsdash -addr :8080
```

### Architecture

```
├── main.go                    # Entry point, HTTP server, signal handling
├── internal/
│   ├── agent/                 # Agent mode — telemetry to app.zfsdash.com
│   │   ├── agent.go           # Agent struct, Register(), Run(), backoff logic
│   │   └── collector.go       # Adapts zfs.LocalCollector for telemetry
│   ├── alerts/                # Alert rule evaluation and dispatch
│   ├── auth/                  # Session management, bcrypt password hashing
│   ├── config/                # Runtime configuration
│   ├── db/                    # SQLite schema, migrations, Store type
│   ├── license/               # License validation with offline cache
│   ├── platform/              # OS detection helpers
│   ├── store/                 # In-memory ZFS data cache
│   ├── web/                   # HTTP handlers, static file serving
│   ├── wizard/                # First-run setup state detection
│   └── zfs/                   # ZFS data collection (local, SSH, TrueNAS)
│       ├── local.go           # LocalCollector — shells out to zpool/zfs
│       ├── ssh.go             # SSHCollector — runs ZFS commands over SSH
│       ├── truenas.go         # TrueNASCollector — TrueNAS REST API client
│       ├── platform.go        # Platform-specific binary path resolution
│       └── types.go           # Pool, Dataset, SMART types
├── install.sh                 # One-liner installer (Linux + FreeBSD)
└── scripts/
    └── zfsdash.rc             # FreeBSD rc.d service script
```

### Contributing

Pull requests welcome. Run `go vet ./...` and `go test ./...` before submitting. The CI pipeline checks all three target platforms on every push.

---

## FAQ

**Q: Does ZFSdash require root?**  
A: For local mode, yes — `zpool` and `zfs` require root or `ZFS_ALLOW` delegation. For SSH mode, the configured SSH user needs ZFS access on the remote host. For TrueNAS, no root needed — just an API key.

**Q: Does it work with OpenZFS 2.x on Linux?**  
A: Yes. ZFSdash shells out to the installed `zpool`/`zfs` binaries and parses their output. It works with OpenZFS 2.0 through 2.2+.

**Q: Is there a Docker image?**  
A: Not yet — ZFSdash needs access to the host's ZFS devices, which is awkward in containers. The single binary + systemd service is the recommended deployment.

**Q: What database does it use?**  
A: SQLite (via `modernc.org/sqlite`, pure Go, no CGO). The database file lives in the data directory (`/var/lib/zfsdash/zfsdash.db`).

**Q: How do I upgrade?**  
A: Run the installer again — it downloads the latest binary and restarts the service:
```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```
