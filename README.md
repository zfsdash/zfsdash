# ZFSdash

**Open source ZFS management dashboard. Single binary. No cloud required.**

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/zfsdash/zfsdash)](https://github.com/zfsdash/zfsdash/releases)

## Install

```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

Open **http://localhost:8080** — the setup wizard walks you through everything. No config files.

### Manual

```bash
# Linux amd64
curl -fsSL https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash-linux-amd64 \
  -o /usr/local/bin/zfsdash && chmod +x /usr/local/bin/zfsdash && zfsdash serve

# FreeBSD amd64
curl -fsSL https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash-freebsd-amd64 \
  -o /usr/local/bin/zfsdash && chmod +x /usr/local/bin/zfsdash && zfsdash serve
```

## Features

- **Pool health** — ONLINE/DEGRADED/FAULTED at a glance
- **Capacity tracking** — used/available with history
- **Dataset & snapshot management** — create, clone, rollback, destroy
- **Scrub scheduling** — start and track progress
- **SMART monitoring** — drive health, temperature, error counts
- **Alerting** — email and webhook for degraded pools and low capacity
- **Multi-host** — manage ZFS on many servers from one dashboard
- **Setup wizard** — no config files needed, configure everything in the browser

## Connection Modes

| Mode | How it works |
|------|-------------|
| **Local** | Runs `zpool`/`zfs` on the same machine |
| **SSH** | Connects via SSH to a remote host |
| **TrueNAS** | Uses the TrueNAS REST API |

## ZFSdash Cloud

Don't want to self-host? [app.zfsdash.com](https://app.zfsdash.com) hosts the dashboard for you.

Install the agent on your ZFS hosts:

```bash
ZFSDASH_TOKEN=your_token zfsdash agent
```

$19/month. Cancel anytime.

## Enterprise ($49/mo)

- Unlimited users + RBAC
- LDAP/SSO
- Slack, PagerDuty, Opsgenie alerts
- Audit log
- SLA support

[zfsdash.com/pricing](https://zfsdash.com/pricing)

## Build from Source

```bash
git clone https://github.com/zfsdash/zfsdash
cd zfsdash
go build -o zfsdash .
./zfsdash serve
```

Requires Go 1.22+.

## License

AGPL-3.0. See [LICENSE](LICENSE).
