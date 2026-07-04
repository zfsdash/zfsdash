# ZFSdash

**Open source ZFS management dashboard. Single Go binary. No Docker. No config files.**

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/zfsdash/zfsdash)](https://github.com/zfsdash/zfsdash/releases)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20FreeBSD-lightgrey)](#)

![ZFSdash Dashboard](https://zfsdash.com/screenshot.png)

## Install

```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

Then open `http://your-server:8080` — a setup wizard walks you through the rest. No config file required.

### Direct Download

| Platform | Binary |
|---|---|
| Linux amd64 | [zfsdash-linux-amd64](https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash-linux-amd64) |
| Linux arm64 | [zfsdash-linux-arm64](https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash-linux-arm64) |
| FreeBSD amd64 | [zfsdash-freebsd-amd64](https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash-freebsd-amd64) |

## Features

- **Pool health monitoring** — ONLINE / DEGRADED / FAULTED status, capacity bars, error counts
- **Dataset tree** — used/avail/refer columns, compression ratio, mountpoint
- **Snapshot management** — create, clone, destroy, rollback
- **Scrub control** — start/stop scrubs, full scrub history
- **SMART data** — per-drive temperature, reallocated sectors, power-on hours
- **Alerts** — email + webhook, configurable thresholds, cooldown periods
- **Multi-host** — local `zpool`, SSH to remote hosts, TrueNAS REST API
- **Setup wizard** — first-run browser wizard, no config files needed
- **SQLite** — local history, no external database required

## How It Works

```
zfsdash binary
  ├── Embedded web UI (Next.js static export)
  ├── ZFS collectors (local / SSH / TrueNAS REST)
  ├── SQLite database (history, users, config)
  ├── Alert engine (email + webhook)
  └── Setup wizard (first-run only)
```

One binary. One port. Zero external dependencies.

## Comparison

| Feature | ZFSdash | TrueNAS SCALE | Proxmox + ZFS | Grafana + ZFS plugin |
|---|---|---|---|---|
| Single binary install | ✅ | ❌ Full OS | ❌ Full OS | ❌ Multiple services |
| Works on existing Linux | ✅ | ❌ Replaces OS | ❌ Replaces OS | ✅ |
| Works on existing FreeBSD | ✅ | ✅ (CORE only) | ❌ | ✅ |
| No Docker required | ✅ | ❌ | ❌ | ❌ |
| Browser setup wizard | ✅ | ✅ | ❌ | ❌ |
| Pool health dashboard | ✅ | ✅ | ✅ | ✅ |
| Snapshot management | ✅ | ✅ | ✅ | ❌ |
| SMART data | ✅ | ✅ | ✅ | ✅ |
| Scrub scheduling | ✅ | ✅ | ❌ | ❌ |
| SSH to remote hosts | ✅ | ❌ | ❌ | ✅ |
| TrueNAS API support | ✅ | — | ❌ | ❌ |
| Open source | ✅ AGPL | ✅ BSL | ✅ AGPL | ✅ Apache |
| Binary size | 12 MB | ~5 GB | ~2 GB | ~500 MB |
| Memory usage | ~3 MB | ~4 GB | ~1 GB | ~500 MB |

## Configuration Modes

### Local (default)
Monitors ZFS pools on the host where ZFSdash is running.

### SSH
Connects to remote Linux/FreeBSD hosts via SSH. No agent required on the remote host.

```yaml
hosts:
  - name: storage-box
    mode: ssh
    host: 192.168.1.100
    user: root
    key: /home/admin/.ssh/id_ed25519
```

### TrueNAS REST API
Connects to TrueNAS SCALE or CORE via the REST API.

```yaml
hosts:
  - name: my-truenas
    mode: truenas
    url: https://truenas.local
    api_key: 1-abcdef123456
```

## Alerts

Configure email or webhook alerts:

```yaml
alerts:
  - name: pool-usage-critical
    condition: pool_used_percent > 85
    webhook: https://hooks.slack.com/services/...
    cooldown: 24h
  - name: pool-degraded
    condition: pool_health != ONLINE
    email: admin@example.com
```

## Roadmap (v0.2)

- [ ] Capacity forecast ("full in X days")
- [ ] One-click snapshot clone to temp dataset
- [ ] Scrub scheduling UI
- [ ] Dataset I/O stats (read/write MB/s)
- [ ] Prometheus `/metrics` endpoint
- [ ] FreeBSD rc.d service file

See [open issues](https://github.com/zfsdash/zfsdash/issues) to contribute.

## Cloud

Manage multiple ZFS hosts from a single hosted dashboard at [app.zfsdash.com](https://app.zfsdash.com).

## License

[AGPL-3.0](LICENSE) — free for self-hosted use. [Enterprise license](https://app.zfsdash.com/billing) available for commercial deployments requiring SSO, RBAC, and SLA-backed support.

## Contributing

PRs welcome. Check [open issues](https://github.com/zfsdash/zfsdash/issues) for good first contributions.

```bash
git clone https://github.com/zfsdash/zfsdash
cd zfsdash
go build ./...
go test ./...
```
