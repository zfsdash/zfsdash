# ZFSdash

Lightweight, open-source ZFS management dashboard. Single binary, runs on Linux and FreeBSD. No Docker required.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![GitHub release](https://img.shields.io/github/v/release/zfsdash/zfsdash)](https://github.com/zfsdash/zfsdash/releases)

## Install

```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

Then open **http://localhost:8080** and complete the setup wizard.

## What It Does

- **Pool management** — health, capacity, scrub status, VDEV layout
- **Dataset management** — create, destroy, set properties (compression, quota, dedup)
- **Snapshot management** — create, clone, rollback, destroy
- **SMART monitoring** — drive health, temperature, reallocated sectors
- **Alerts** — email and webhook notifications on pool degradation
- **Multi-host** — manage local, SSH, and TrueNAS hosts from one UI

## Modes

### Local (default)
Runs on your ZFS machine. Zero config.
```yaml
hosts:
  - name: localhost
    mode: local
```

### SSH
Manage a remote host over SSH.
```yaml
hosts:
  - name: nas-01
    mode: ssh
    host: 192.168.1.10
    user: root
    key: /etc/zfsdash/id_ed25519
```

### TrueNAS
Connect via TrueNAS REST API.
```yaml
hosts:
  - name: truenas
    mode: truenas
    host: 192.168.1.20
    api_key: YOUR_API_KEY
```

## Platforms

| Platform | Supported |
|---|---|
| Linux (amd64) | ✅ |
| Linux (arm64) | ✅ |
| FreeBSD (amd64) | ✅ |
| TrueNAS CORE | ✅ (via FreeBSD binary) |
| TrueNAS SCALE | ✅ (via Linux binary) |

## Build From Source

```bash
git clone https://github.com/zfsdash/zfsdash
cd zfsdash
go build -o zfsdash .
./zfsdash serve
```

Requires Go 1.22+.

## License

AGPL-3.0. See [LICENSE](LICENSE).

Enterprise license with RBAC, SSO, Slack/PagerDuty alerts, and SLA support available at [zfsdash.com](https://zfsdash.com).
