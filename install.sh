#!/usr/bin/env bash
set -euo pipefail
REPO="zfsdash/zfsdash"; INSTALL_DIR="/usr/local/bin"
GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'
log() { echo -e "${BLUE}==>${NC} $1"; }; ok() { echo -e "${GREEN}✓${NC} $1"; }; die() { echo "ERROR: $1" >&2; exit 1; }
echo -e "\n  ${BLUE}ZFSdash${NC} — Open Source ZFS Management Dashboard\n"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"; ARCH="$(uname -m)"
case "$OS" in linux|freebsd) ;; *) die "Unsupported OS: $OS" ;; esac
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; *) die "Unsupported arch: $ARCH" ;; esac
log "Detected: $OS/$ARCH"
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
[ -z "$LATEST" ] && die "Could not determine latest version"; ok "Latest: $LATEST"
TMP=$(mktemp); trap 'rm -f $TMP' EXIT
curl -fsSL "https://github.com/$REPO/releases/download/$LATEST/zfsdash-${OS}-${ARCH}" -o "$TMP" || die "Download failed"
chmod +x "$TMP"
if [ -w "$INSTALL_DIR" ]; then mv "$TMP" "$INSTALL_DIR/zfsdash"; else sudo mv "$TMP" "$INSTALL_DIR/zfsdash"; fi
ok "Installed: $INSTALL_DIR/zfsdash"
if [ "$OS" = "linux" ]; then
  sudo tee /etc/systemd/system/zfsdash.service > /dev/null << 'UNIT'
[Unit]
Description=ZFSdash ZFS Management Dashboard
After=network.target
[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/zfsdash serve
Restart=on-failure
RestartSec=5
[Install]
WantedBy=multi-user.target
UNIT
  sudo systemctl daemon-reload; sudo systemctl enable zfsdash; sudo systemctl start zfsdash; ok "systemd service started"
elif [ "$OS" = "freebsd" ]; then
  sudo tee /usr/local/etc/rc.d/zfsdash > /dev/null << 'RCD'
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
. /etc/rc.subr
name="zfsdash"; rcvar="zfsdash_enable"
command="/usr/local/bin/zfsdash"; command_args="serve"
load_rc_config $name; run_rc_command "$1"
RCD
  sudo chmod +x /usr/local/etc/rc.d/zfsdash
  grep -q 'zfsdash_enable' /etc/rc.conf 2>/dev/null || echo 'zfsdash_enable="YES"' | sudo tee -a /etc/rc.conf
  sudo service zfsdash start; ok "rc.d service started"
fi
echo -e "\n  ${GREEN}ZFSdash is running!${NC}\n  Open: ${BLUE}http://localhost:8080/setup${NC}\n"
if [ -n "${DISPLAY:-}" ] || [ -n "${WAYLAND_DISPLAY:-}" ]; then
  for b in xdg-open open firefox chromium; do command -v "$b" &>/dev/null && "$b" "http://localhost:8080/setup" &>/dev/null & break; done
fi
