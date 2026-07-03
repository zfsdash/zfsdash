#!/bin/sh
set -e

# ZFSdash install script
# Usage: curl -fsSL https://zfsdash.com/install.sh | sudo bash

BINARY_BASE_URL="https://github.com/zfsdash/zfsdash/releases/latest/download"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/zfsdash"
SERVICE_NAME="zfsdash"

# ── Helpers ──────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m[zfsdash]\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m[zfsdash]\033[0m %s\n' "$*"; }
err()   { printf '\033[1;31m[zfsdash]\033[0m %s\n' "$*" >&2; exit 1; }

# ── OS detection ─────────────────────────────────────────────────────────────

OS="$(uname -s)"
case "$OS" in
  Linux)   OS_SLUG="linux" ;;
  FreeBSD) OS_SLUG="freebsd" ;;
  *)       err "Unsupported OS: $OS. ZFSdash supports Linux and FreeBSD." ;;
esac

# ── Architecture detection ────────────────────────────────────────────────────

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)   ARCH_SLUG="amd64" ;;
  aarch64|arm64)  ARCH_SLUG="arm64" ;;
  *)              err "Unsupported architecture: $ARCH. ZFSdash supports amd64 and arm64." ;;
esac

# FreeBSD arm64 not yet released
if [ "$OS_SLUG" = "freebsd" ] && [ "$ARCH_SLUG" = "arm64" ]; then
  err "FreeBSD arm64 is not yet supported. Please open an issue at https://github.com/zfsdash/zfsdash if you need it."
fi

BINARY_NAME="zfsdash-${OS_SLUG}-${ARCH_SLUG}"
DOWNLOAD_URL="${BINARY_BASE_URL}/${BINARY_NAME}"

info "Detected: ${OS} / ${ARCH_SLUG}"
info "Downloading: ${DOWNLOAD_URL}"

# ── Check for required tools ──────────────────────────────────────────────────

for cmd in curl chmod mkdir; do
  command -v "$cmd" >/dev/null 2>&1 || err "Required tool not found: $cmd"
done

# ── Download binary ───────────────────────────────────────────────────────────

TMP_BIN="$(mktemp)"
trap 'rm -f "$TMP_BIN"' EXIT INT TERM

curl -fsSL --retry 3 --retry-delay 2 -o "$TMP_BIN" "$DOWNLOAD_URL" \
  || err "Download failed. Check your internet connection and try again."

chmod 755 "$TMP_BIN"

# ── Install binary ────────────────────────────────────────────────────────────

info "Installing binary to ${INSTALL_DIR}/zfsdash"
mkdir -p "$INSTALL_DIR"
cp "$TMP_BIN" "${INSTALL_DIR}/zfsdash"

# ── Create data directory ────────────────────────────────────────────────────

mkdir -p "$DATA_DIR"

# ── Platform-specific service install ────────────────────────────────────────

case "$OS_SLUG" in

  linux)
    SYSTEMD_SERVICE_DIR="/etc/systemd/system"
    SERVICE_FILE="${SYSTEMD_SERVICE_DIR}/zfsdash.service"

    info "Installing systemd service to ${SERVICE_FILE}"
    mkdir -p "$SYSTEMD_SERVICE_DIR"

    cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=ZFSdash — ZFS Management Dashboard
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/zfsdash
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=zfsdash

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload

    # Enable and start (idempotent: --now is a no-op if already running)
    if systemctl is-active --quiet zfsdash; then
      info "Service already running — reloading binary"
      systemctl restart zfsdash
    else
      systemctl enable --now zfsdash
    fi
    ;;

  freebsd)
    RCD_DIR="/usr/local/etc/rc.d"
    RCD_FILE="${RCD_DIR}/zfsdash"

    info "Installing rc.d script to ${RCD_FILE}"
    mkdir -p "$RCD_DIR"

    cat > "$RCD_FILE" << 'EOF'
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="zfsdash"
rcvar="zfsdash_enable"
command="/usr/local/bin/zfsdash"
pidfile="/var/run/${name}.pid"

load_rc_config $name
: ${zfsdash_enable:="NO"}

run_rc_command "$1"
EOF

    chmod 755 "$RCD_FILE"

    # Enable in rc.conf if not already present
    if ! grep -q 'zfsdash_enable' /etc/rc.conf 2>/dev/null; then
      echo 'zfsdash_enable="YES"' >> /etc/rc.conf
    fi

    service zfsdash enable 2>/dev/null || true

    if service zfsdash status >/dev/null 2>&1; then
      info "Service already running — restarting"
      service zfsdash restart
    else
      service zfsdash start
    fi
    ;;
esac

# ── Done ─────────────────────────────────────────────────────────────────────

ok "ZFSdash installed! Open http://localhost:8080 in your browser to complete setup."
