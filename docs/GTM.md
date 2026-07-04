# ZFSdash Go-To-Market Strategy

## Positioning

**ZFSdash is the lightweight, open-source ZFS management dashboard for people who want TrueNAS features without the bloat, FreeBSD dependency, or licensing uncertainty.**

Single 18MB Go binary. No runtime dependencies. Runs on any Linux or FreeBSD system with ZFS. Browser-based setup wizard. AGPL-3.0.

---

## Show HN Post

**Title:** `ZFSdash – Open-source ZFS dashboard in a single Go binary (Linux + FreeBSD)`

**Body:**

I built ZFSdash because I wanted a TrueNAS-style dashboard for my ZFS pools on Linux without running an entire NAS operating system.

It's a single statically-compiled Go binary (18MB, zero runtime dependencies). Run it on any Linux or FreeBSD machine that has ZFS, open localhost:8080, complete the setup wizard, and you have a full ZFS management dashboard.

**Features:**
- Real-time pool health: disk status, SMART data, vdev layout, capacity
- Dataset management: create/clone/destroy, quotas, compression, hierarchy view
- Snapshot management: manual + automatic with retention policies
- Scrub scheduling and history
- Email + webhook alerts (Slack, PagerDuty on Enterprise tier)
- Multi-host: manage multiple ZFS systems from one dashboard (SSH + TrueNAS REST API)
- REST API: scriptable, Prometheus-compatible metrics
- Works on: Ubuntu, Debian, Rocky Linux, FreeBSD, TrueNAS CORE

**Install:**
```bash
curl -fsSL https://zfsdash.com/install.sh | sudo bash
```

AGPL-3.0. No telemetry unless you opt in. No accounts required for self-hosted.

Looking for feedback: what ZFS management features would make this your daily driver?

github.com/zfsdash/zfsdash

---

## Reddit Posts

### r/homelab
**Title:** I built an open-source ZFS dashboard — single binary, installs in 30 seconds

Tired of either raw CLI or spinning up a full TrueNAS install just to get a dashboard for my ZFS pools. Built ZFSdash — single Go binary, no dependencies, browser-based setup wizard.

Install: `curl -fsSL https://zfsdash.com/install.sh | sudo bash`
GitHub: github.com/zfsdash/zfsdash

Features: pool health, dataset/snapshot management, SMART data, alerts, multi-host (SSH + TrueNAS REST). AGPL-3.0.

Looking for homelab feedback — what’s missing for your setup?

### r/zfs
**Title:** ZFSdash — lightweight open-source ZFS management dashboard (single binary, Linux + FreeBSD)

### r/selfhosted
**Title:** ZFSdash — self-hosted ZFS dashboard, single Go binary, zero dependencies

### r/truenas
**Title:** Built a TrueNAS-alternative dashboard for people running ZFS on Linux

### r/DataHoarder
**Title:** Open-source ZFS dashboard with SMART monitoring, snapshot management, and capacity alerts

---

## Pricing Page Copy

### Free (Self-Hosted)
**$0 forever for self-hosted use**
- Unlimited ZFS hosts
- Pool, dataset, snapshot management
- SMART monitoring
- Email + webhook alerts
- REST API
- Community support
- AGPL-3.0 — modify and self-host freely

### Cloud — $19/mo
**ZFSdash hosted for you**
- Everything in Free
- No server required — we host it
- Lightweight agent on your ZFS host
- Slack + PagerDuty alerts
- Multi-user access
- 30-day metrics history
- Email support

### Enterprise — $49/mo
**For teams and production infrastructure**
- Everything in Cloud
- RBAC (role-based access control)
- SSO / LDAP integration
- AI disk failure prediction
- Audit log
- 1-year metrics history
- SLA-backed support

### AI Health Add-on — $29/mo
**Predict failures before they happen**
- SMART trend analysis
- Failure prediction windows (days/weeks out)
- Anomaly detection
- Requires Cloud or Enterprise

---

## SEO Keywords

**Primary:**
- zfs dashboard
- zfs web interface
- zfs management tool
- open source zfs gui
- truenas alternative linux

**Secondary:**
- zfs pool monitoring
- zfs snapshot management
- zfs smart monitoring
- zfs linux dashboard
- freebsd zfs gui
- openzfs dashboard
- self-hosted zfs management
- zfs performance monitoring

**Long-tail:**
- zfs dashboard linux ubuntu
- monitor zfs pools web browser
- zfs management without truenas
- open source alternative to truenas scale
- zfs dashboard single binary

---

## Launch Sequence — First 2 Weeks

### Day 1
- [ ] Publish v0.1.0 GitHub release with Linux amd64/arm64 + FreeBSD amd64 binaries
- [ ] Post Show HN
- [ ] Post to r/homelab
- [ ] Post to r/zfs

### Day 2
- [ ] Post to r/selfhosted
- [ ] Post to r/DataHoarder
- [ ] Post to r/truenas
- [ ] Respond to all HN + Reddit comments

### Day 3-5
- [ ] Ship fixes from community feedback
- [ ] Publish v0.1.1 patch release
- [ ] Post to X/Twitter thread
- [ ] Submit to awesome-selfhosted
- [ ] Submit to awesome-zfs (if exists)

### Week 2
- [ ] Launch app.zfsdash.com Cloud tier
- [ ] Email everyone who starred the repo
- [ ] Post "What we learned from 1000 installs" on HN
- [ ] Reach out to 5 homelab YouTubers for coverage
- [ ] Add to alternativeto.net as TrueNAS alternative

---

## Twitter/X Thread

1/ I spent the last few weeks building ZFSdash — an open-source ZFS management dashboard that runs as a single 18MB binary.

No Docker. No Node.js. No config files. Just: `curl -fsSL https://zfsdash.com/install.sh | sudo bash` and open localhost:8080.

2/ Why? TrueNAS is great but it’s an entire OS. If you’re already running ZFS on Linux or FreeBSD, you shouldn’t need to reinstall your OS just to get a web dashboard.

3/ Features: pool health • dataset management • snapshot browser • scrub history • SMART data • capacity alerts • multi-host via SSH • REST API

4/ Browser-based setup wizard — no config files. First launch opens the wizard, you add your ZFS host, done.

5/ AGPL-3.0. Self-hosted is always free. Cloud and Enterprise tiers for teams who want hosting, Slack alerts, RBAC, and AI disk failure prediction.

github.com/zfsdash/zfsdash
