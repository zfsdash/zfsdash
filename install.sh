#!/usr/bin/env bash
set -euo pipefail

VERSION="${ZFSDASH_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
SERVICE_USER="zfsdash"
DATA_DIR="/var/lib/zfsdash"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[ZFSdash]${NC} $*"; }
success() { echo -e "${GREEN}[ZFSdash]${NC} $*"; }
warn()    { echo -e "${YELLOW}[ZFSdash]${NC} $*"; }
error()   { echo -e "${RED}[ZFSdash]${NC} $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || error "Run as root: sudo bash install.sh"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac
case "$OS" in
  linux)   PLATFORM="linux" ;;
  freebsd) PLATFORM="freebsd" ;;
  *) error "Unsupported OS: $OS" ;;
esac

info "Detected: ${PLATFORM}/${ARCH}"

if ! command -v zpool &>/dev/null; then
  warn "zpool not found. ZFSdash will work but local ZFS pools won't be available."
else
  success "ZFS found"
fi

if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/zfsdash/zfsdash/releases/latest/download/zfsdash_${PLATFORM}_${ARCH}"
else
  URL="https://github.com/zfsdash/zfsdash/releases/download/${VERSION}/zfsdash_${PLATFORM}_${ARCH}"
fi

info "Downloading ZFSdash..."
curl -fsSL "$URL" -o /tmp/zfsdash || error "Download failed. See https://github.com/zfsdash/zfsdash/releases"
chmod +x /tmp/zfsdash
install -o root -g root -m 0755 /tmp/zfsdash "${INSTALL_DIR}/zfsdash"
rm -f /tmp/zfsdash
success "Installed to ${INSTALL_DIR}/zfsdash"

if ! id "$SERVICE_USER" &>/dev/null; then
  if [ "$PLATFORM" = "freebsd" ]; then
    pw useradd -n "$SERVICE_USER" -d "$DATA_DIR" -s /usr/sbin/nologin -c "ZFSdash" 2>/dev/null || true
  else
    useradd --system --home-dir "$DATA_DIR" --no-create-home --shell /usr/sbin/nologin --comment "ZFSdash" "$SERVICE_USER" 2>/dev/null || true
  fi
fi

mkdir -p "$DATA_DIR"
chown "$SERVICE_USER" "$DATA_DIR" 2>/dev/null || true
chmod 750 "$DATA_DIR"

if [ "$PLATFORM" = "freebsd" ]; then
  cat > /etc/rc.d/zfsdash <<EOF
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
. /etc/rc.subr
name="zfsdash"
rcvar="zfsdash_enable"
command="${INSTALL_DIR}/zfsdash"
command_args="--data ${DATA_DIR} --listen 0.0.0.0:8080"
zfsdash_user="${SERVICE_USER}"
load_rc_config \$name
run_rc_command "\$1"
EOF
  chmod +x /etc/rc.d/zfsdash
  sysrc zfsdash_enable=YES
  service zfsdash start
else
  cat > /etc/systemd/system/zfsdash.service <<EOF
[Unit]
Description=ZFSdash - ZFS Management Dashboard
After=network.target
Documentation=https://zfsdash.com

[Service]
Type=simple
User=$SERVICE_USER
ExecStart=${INSTALL_DIR}/zfsdash --data ${DATA_DIR} --listen 0.0.0.0:8080
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable zfsdash
  systemctl restart zfsdash
fi

success "ZFSdash installed and running!"
echo ""
echo "  Open: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'SERVER_IP'):8080"
echo "  Complete setup in the browser wizard."
echo "  Docs: https://zfsdash.com/docs"
echo ""
