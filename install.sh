#!/usr/bin/env bash
set -euo pipefail

# ZFSdash installer
# Usage: curl -fsSL https://zfsdash.com/install.sh | sudo bash

VERSION="latest"
GITHUB_REPO="zfsdash/zfsdash"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="zfsdash"
DATA_DIR="/var/lib/zfsdash"
PORT="8080"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[zfsdash]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[zfsdash]${NC} $*"; }
log_error() { echo -e "${RED}[zfsdash]${NC} $*" >&2; }

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$arch" in
    x86_64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) log_error "Unsupported arch: $arch"; exit 1 ;;
  esac
  case "$os" in
    linux)   OS="linux" ;;
    freebsd) OS="freebsd" ;;
    *) log_error "Unsupported OS: $os"; exit 1 ;;
  esac
  ARCH="$arch"
  PLATFORM="${OS}-${ARCH}"
  log_info "Detected: $PLATFORM"
}

get_latest_version() {
  if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -z "$VERSION" ] && { log_error "Could not fetch latest version"; exit 1; }
    log_info "Latest version: $VERSION"
  fi
}

download_binary() {
  local url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/zfsdash-${PLATFORM}"
  local tmp="/tmp/zfsdash-download"
  log_info "Downloading zfsdash ${VERSION}..."
  curl -fsSL "$url" -o "$tmp" || { log_error "Download failed: $url"; exit 1; }
  chmod +x "$tmp"
  mv "$tmp" "${INSTALL_DIR}/zfsdash"
  log_info "Installed to ${INSTALL_DIR}/zfsdash"
}

setup_dirs() {
  mkdir -p "$DATA_DIR"
}

install_systemd() {
  cat > /etc/systemd/system/zfsdash.service <<EOF
[Unit]
Description=ZFSdash - ZFS Management Dashboard
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/zfsdash --data-dir=${DATA_DIR} --port=${PORT}
Restart=on-failure
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now zfsdash
  log_info "systemd service installed and started"
}

install_rcd() {
  cat > /etc/rc.d/zfsdash <<EOF
#!/bin/sh
# PROVIDE: zfsdash
# REQUIRE: NETWORKING
. /etc/rc.subr
name="zfsdash"
rcvar="zfsdash_enable"
command="${INSTALL_DIR}/zfsdash"
command_args="--data-dir=${DATA_DIR} --port=${PORT}"
load_rc_config \$name
: \${zfsdash_enable:=no}
run_rc_command "\$1"
EOF
  chmod +x /etc/rc.d/zfsdash
  echo 'zfsdash_enable="YES"' >> /etc/rc.conf
  service zfsdash start
  log_info "rc.d service installed and started"
}

main() {
  [ "$(id -u)" -ne 0 ] && { log_error "Run as root (use sudo)"; exit 1; }
  log_info "Installing ZFSdash..."
  detect_platform
  get_latest_version
  setup_dirs
  download_binary
  case "$OS" in
    linux)   install_systemd ;;
    freebsd) install_rcd ;;
  esac
  echo ""
  echo -e "${GREEN}ZFSdash ${VERSION} installed!${NC}"
  echo -e "Open ${BLUE}http://localhost:${PORT}${NC} to complete setup"
  echo ""
}

main "$@"
