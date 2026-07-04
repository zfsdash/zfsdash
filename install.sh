#!/usr/bin/env bash
# ZFSdash installer
# Usage: curl -fsSL https://zfsdash.com/install.sh | sudo bash

set -euo pipefail

REPO="zfsdash/zfsdash"
INSTALL_DIR="/usr/local/bin"
SERVICE_USER="zfsdash"
CONFIG_DIR="/etc/zfsdash"
DATA_DIR="/var/lib/zfsdash"
BINARY="zfsdash"
SERVICE_NAME="zfsdash"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}[ZFSdash]${NC} $*"; }
success() { echo -e "${GREEN}[ZFSdash]${NC} $*"; }
warn()    { echo -e "${YELLOW}[ZFSdash]${NC} $*"; }
error()   { echo -e "${RED}[ZFSdash]${NC} $*" >&2; exit 1; }

# ─── Platform detection ───────────────────────────────────────────────────────

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64)        ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH. Supported: x86_64, aarch64/arm64." ;;
    esac

    case "$OS" in
        linux)   ;;
        freebsd) ;;
        *) error "Unsupported OS: $OS. Supported: linux, freebsd." ;;
    esac

    PLATFORM_SUFFIX="${OS}-${ARCH}"
    info "Detected platform: $OS/$ARCH"
}

# ─── Dependency check ─────────────────────────────────────────────────────────

check_deps() {
    local missing=()
    for cmd in curl tar; do
        command -v "$cmd" &>/dev/null || missing+=("$cmd")
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing[*]}. Install them first."
    fi
}

# ─── Root check ───────────────────────────────────────────────────────────────

check_root() {
    if [[ "$(id -u)" -ne 0 ]]; then
        error "This installer must be run as root. Try: sudo bash install.sh"
    fi
}

# ─── Resolve version ──────────────────────────────────────────────────────────

resolve_version() {
    local version="${ZFSDASH_VERSION:-latest}"

    if [[ "$version" == "latest" ]]; then
        info "Fetching latest release from GitHub..."
        local api_url="https://api.github.com/repos/${REPO}/releases/latest"
        version="$(curl -fsSL "$api_url" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
        if [[ -z "$version" ]]; then
            error "Could not determine latest version. Set ZFSDASH_VERSION to a specific tag, e.g. ZFSDASH_VERSION=v0.1.0"
        fi
    fi

    ZFSDASH_VERSION="$version"
    info "Installing ZFSdash ${ZFSDASH_VERSION}"
}

# ─── Download ─────────────────────────────────────────────────────────────────

download_binary() {
    local binary_name="${BINARY}-${PLATFORM_SUFFIX}"
    local download_url="https://github.com/${REPO}/releases/download/${ZFSDASH_VERSION}/${binary_name}"
    local tmp_dir
    tmp_dir="$(mktemp -d)"

    info "Downloading ${binary_name}..."
    if ! curl -fsSL --progress-bar -o "${tmp_dir}/${BINARY}" "$download_url"; then
        error "Download failed. URL: $download_url\nCheck that version ${ZFSDASH_VERSION} exists."
    fi

    # Verify checksum if SHA256SUMS is available
    local checksums_url="https://github.com/${REPO}/releases/download/${ZFSDASH_VERSION}/SHA256SUMS"
    if curl -fsSL -o "${tmp_dir}/SHA256SUMS" "$checksums_url" 2>/dev/null; then
        info "Verifying checksum..."
        cd "$tmp_dir"
        grep "${binary_name}" SHA256SUMS | sed "s/${binary_name}/${BINARY}/" | sha256sum -c - 2>/dev/null \
            || warn "Checksum verification failed — proceeding anyway (file may still be valid)"
        cd - >/dev/null
    fi

    chmod +x "${tmp_dir}/${BINARY}"
    mv "${tmp_dir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    rm -rf "$tmp_dir"

    success "Binary installed to ${INSTALL_DIR}/${BINARY}"
}

# ─── User and directories ─────────────────────────────────────────────────────

setup_user_and_dirs() {
    # Create service user
    if ! id "$SERVICE_USER" &>/dev/null; then
        info "Creating service user: ${SERVICE_USER}"
        if [[ "$OS" == "freebsd" ]]; then
            pw useradd -n "$SERVICE_USER" -d /nonexistent -s /usr/sbin/nologin -c "ZFSdash daemon" || true
        else
            useradd --system --no-create-home --shell /usr/sbin/nologin \
                --comment "ZFSdash daemon" "$SERVICE_USER" || true
        fi
    fi

    # Create directories
    for dir in "$CONFIG_DIR" "$DATA_DIR"; do
        mkdir -p "$dir"
        chown "$SERVICE_USER:$SERVICE_USER" "$dir"
        chmod 750 "$dir"
    done

    # Default config if absent
    if [[ ! -f "${CONFIG_DIR}/config.env" ]]; then
        cat > "${CONFIG_DIR}/config.env" << CONFIG
# ZFSdash configuration
ZFSDASH_ADDR=:8080
ZFSDASH_DATA_DIR=${DATA_DIR}
CONFIG
        chown "${SERVICE_USER}:${SERVICE_USER}" "${CONFIG_DIR}/config.env"
        chmod 640 "${CONFIG_DIR}/config.env"
    fi
}

# ─── Service installation ─────────────────────────────────────────────────────

install_service_linux() {
    local unit_file="/etc/systemd/system/${SERVICE_NAME}.service"

    cat > "$unit_file" << UNIT
[Unit]
Description=ZFSdash — ZFS Management Dashboard
Documentation=https://zfsdash.com
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/${BINARY} -addr \${ZFSDASH_ADDR:-:8080} -data ${DATA_DIR}
EnvironmentFile=-${CONFIG_DIR}/config.env
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
ProtectHome=true

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    systemctl restart "$SERVICE_NAME"
    success "systemd service installed and started"
}

install_service_freebsd() {
    local rc_file="/etc/rc.d/${SERVICE_NAME}"

    cat > "$rc_file" << 'RCEOF'
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="zfsdash"
rcvar="zfsdash_enable"
desc="ZFSdash ZFS Management Dashboard"
command="/usr/local/bin/zfsdash"
command_args="-addr :8080 -data /var/lib/zfsdash"
pidfile="/var/run/${name}.pid"
start_precmd="${name}_prestart"

zfsdash_prestart() {
    if [ ! -d /var/lib/zfsdash ]; then
        install -d -o zfsdash -g zfsdash -m 750 /var/lib/zfsdash
    fi
}

load_rc_config "$name"
run_rc_command "$1"
RCEOF

    chmod +x "$rc_file"

    # Enable in rc.conf if not already
    if ! grep -q "zfsdash_enable" /etc/rc.conf 2>/dev/null; then
        echo 'zfsdash_enable="YES"' >> /etc/rc.conf
    fi

    service zfsdash start || true
    success "rc.d service installed and started"
}

install_service() {
    case "$OS" in
        linux)   install_service_linux ;;
        freebsd) install_service_freebsd ;;
    esac
}

# ─── ZFS availability check ───────────────────────────────────────────────────

check_zfs() {
    local zpool_cmd=""
    for p in /sbin/zpool /usr/sbin/zpool zpool; do
        if command -v "$p" &>/dev/null 2>&1; then
            zpool_cmd="$p"
            break
        fi
    done

    if [[ -z "$zpool_cmd" ]]; then
        warn "ZFS tools (zpool) not found. ZFSdash requires ZFS to be installed."
        warn "On Ubuntu/Debian: apt install zfsutils-linux"
        warn "On FreeBSD: zfs is built in — make sure zfs_enable=\"YES\" in /etc/rc.conf"
    else
        local pool_count
        pool_count="$("$zpool_cmd" list -H -o name 2>/dev/null | wc -l || echo 0)"
        info "ZFS detected: $pool_count pool(s) found"
    fi
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
    echo ""
    echo "╔═══════════════════════════════════╗"
    echo "║   ZFSdash Installer               ║"
    echo "║   https://zfsdash.com             ║"
    echo "╚═══════════════════════════════════╝"
    echo ""

    check_root
    check_deps
    detect_platform
    resolve_version
    download_binary
    check_zfs
    setup_user_and_dirs
    install_service

    echo ""
    success "ZFSdash ${ZFSDASH_VERSION} installed successfully!"
    echo ""
    echo "  Dashboard: http://$(hostname -f 2>/dev/null || hostname):8080"
    echo "  Config:    ${CONFIG_DIR}/config.env"
    echo "  Data:      ${DATA_DIR}"
    echo "  Logs:"
    if [[ "$OS" == "linux" ]]; then
        echo "    journalctl -u zfsdash -f"
    else
        echo "    tail -f /var/log/messages | grep zfsdash"
    fi
    echo ""
    echo "  Open the dashboard to complete setup:"
    echo "    http://$(hostname -f 2>/dev/null || hostname):8080"
    echo ""
}

main "$@"
