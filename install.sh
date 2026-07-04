#!/usr/bin/env bash
set -euo pipefail

REPO="zfsdash/zfsdash"
VERSION="v0.1.0"
INSTALL_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"
DATA_DIR="/var/lib/zfsdash"
USER="zfsdash"

GREEN='\033[0;32m'; BLUE='\033[0;34m'; RED='\033[0;31m'; NC='\033[0m'
log()  { echo -e "${BLUE}==>${NC} $1"; }
ok()   { echo -e "${GREEN}✓${NC} $1"; }
err()  { echo -e "${RED}✗${NC} $1" >&2; exit 1; }

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       err "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|freebsd) ;;
  *) err "Unsupported OS: $OS (Linux and FreeBSD supported)" ;;
esac

BINARY="zfsdash-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"

log "Installing ZFSdash ${VERSION} for ${OS}/${ARCH}..."

# Check for root
if [ "$(id -u)" -ne 0 ]; then
  err "Please run as root: curl -fsSL https://zfsdash.com/install.sh | sudo bash"
fi

# Download binary
log "Downloading ${BINARY}..."
TMP=$(mktemp)
curl -fsSL "$DOWNLOAD_URL" -o "$TMP" || err "Download failed. Check https://github.com/${REPO}/releases"
chmod +x "$TMP"
mv "$TMP" "${INSTALL_DIR}/zfsdash"
ok "Binary installed to ${INSTALL_DIR}/zfsdash"

# Create system user
if ! id "$USER" &>/dev/null; then
  useradd --system --no-create-home --shell /bin/false "$USER" 2>/dev/null || true
fi

# Create data directory
mkdir -p "$DATA_DIR"
chown "$USER:$USER" "$DATA_DIR"
ok "Data directory: ${DATA_DIR}"

# Install systemd service (Linux only)
if [ "$OS" = "linux" ] && command -v systemctl &>/dev/null; then
  cat > "${SERVICE_DIR}/zfsdash.service" << 'EOF'
[Unit]
Description=ZFSdash — ZFS Management Dashboard
After=network.target

[Service]
Type=simple
User=zfsdash
ExecStart=/usr/local/bin/zfsdash -addr :8080 -data /var/lib/zfsdash
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/zfsdash
ProtectHome=yes

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable zfsdash
  systemctl restart zfsdash
  ok "systemd service installed and started"
fi

# Install rc.d script (FreeBSD only)
if [ "$OS" = "freebsd" ]; then
  cat > /usr/local/etc/rc.d/zfsdash << 'EOF'
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr
name="zfsdash"
rcvar="zfsdash_enable"
command="/usr/local/bin/zfsdash"
command_args="-addr :8080 -data /var/lib/zfsdash"
zfsdash_user="zfsdash"
load_rc_config $name
run_rc_command "$1"
EOF
  chmod +x /usr/local/etc/rc.d/zfsdash
  sysrc zfsdash_enable=YES 2>/dev/null || true
  service zfsdash start 2>/dev/null || true
  ok "rc.d script installed and started"
fi

echo
echo -e "${GREEN}ZFSdash ${VERSION} installed successfully!${NC}"
echo
echo "  Open: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo localhost):8080"
echo "  Complete the setup wizard to create your admin account."
echo
echo "  Logs:    journalctl -u zfsdash -f"
echo "  Restart: systemctl restart zfsdash"
echo "  Docs:    https://zfsdash.com"
echo
