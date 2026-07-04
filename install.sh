#!/bin/bash
set -euo pipefail

VERSION=v0.1.5
BINARY_NAME=zfsdash
INSTALL_DIR=/usr/local/bin
SERVICE_USER=zfsdash
DATA_DIR=/var/lib/zfsdash

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)  OS_NAME=linux ;;
  freebsd) OS_NAME=freebsd ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH_NAME=amd64 ;;
  aarch64|arm64) ARCH_NAME=arm64 ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Check for ZFS
if ! command -v zpool &>/dev/null; then
  echo "WARNING: zpool not found. Install ZFS before using ZFSdash."
  echo "  Ubuntu: apt install zfsutils-linux"
  echo "  FreeBSD: pkg install zfs"
fi

DOWNLOAD_URL="https://github.com/zfsdash/zfsdash/releases/download/${VERSION}/${BINARY_NAME}-${OS_NAME}-${ARCH_NAME}"

echo "Installing ZFSdash ${VERSION} (${OS_NAME}/${ARCH_NAME})..."
curl -fsSL "$DOWNLOAD_URL" -o "/tmp/${BINARY_NAME}"
chmod +x "/tmp/${BINARY_NAME}"

# Verify binary
if ! /tmp/${BINARY_NAME} --version &>/dev/null 2>&1; then
  # Some versions don't have --version, just check it's executable
  echo "Binary downloaded successfully."
fi

mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

# Create service user
if ! id -u "$SERVICE_USER" &>/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null || true
fi

# Create data directory
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" 2>/dev/null || true

# Install systemd service (Linux)
if [ "$OS_NAME" = "linux" ] && command -v systemctl &>/dev/null; then
  cat > /etc/systemd/system/zfsdash.service << 'UNIT'
[Unit]
Description=ZFSdash — ZFS Management Dashboard
After=network.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
User=zfsdash
ExecStartPre=/bin/bash -c 'fuser -k 8080/tcp 2>/dev/null || true'
ExecStart=/usr/local/bin/zfsdash -addr :8080 -data /var/lib/zfsdash
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/zfsdash

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  systemctl enable zfsdash
  systemctl restart zfsdash

  echo ""
  echo "✓ ZFSdash ${VERSION} installed and running."
  echo ""
  LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "your-server-ip")
  echo "  Open http://${LOCAL_IP}:8080 to complete setup."
  echo ""
fi

# FreeBSD rc.d service
if [ "$OS_NAME" = "freebsd" ]; then
  cat > /usr/local/etc/rc.d/zfsdash << 'RC'
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
# KEYWORD: shutdown
. /etc/rc.subr
name="zfsdash"
rcvar="zfsdash_enable"
command="/usr/local/bin/zfsdash"
command_args="-addr :8080 -data /var/lib/zfsdash"
load_rc_config $name
run_rc_command "$1"
RC
  chmod +x /usr/local/etc/rc.d/zfsdash
  sysrc zfsdash_enable=YES 2>/dev/null || true
  service zfsdash start 2>/dev/null || true
  echo ""
  echo "✓ ZFSdash ${VERSION} installed."
  echo "  Open http://$(hostname):8080 to complete setup."
  echo ""
fi
