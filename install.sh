#!/usr/bin/env bash
set -euo pipefail

VERSION="latest"
AGENT_MODE=false
TOKEN=""

for arg in "$@"; do
  case $arg in
    --agent) AGENT_MODE=true ;;
    --token=*) TOKEN="${arg#*=}" ;;
    --version=*) VERSION="${arg#*=}" ;;
  esac
done

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [ "$OS" = "freebsd" ]; then
  PLATFORM="freebsd-amd64"
elif [ "$OS" = "linux" ]; then
  PLATFORM="linux-${ARCH}"
else
  echo "Unsupported OS: $OS"; exit 1
fi

BINARY_URL="https://github.com/zfsdash/zfsdash/releases/${VERSION}/download/zfsdash-${PLATFORM}"
INSTALL_DIR="/usr/local/bin"

echo "==> Installing ZFSdash ($PLATFORM)"
curl -fsSL "$BINARY_URL" -o /tmp/zfsdash
chmod +x /tmp/zfsdash
mv /tmp/zfsdash "$INSTALL_DIR/zfsdash"

if [ "$AGENT_MODE" = true ]; then
  if [ -z "$TOKEN" ]; then
    echo "Error: --token is required in agent mode"
    exit 1
  fi

  echo "==> Installing ZFSdash agent service"
  CLOUD_URL="https://app.zfsdash.com"

  if [ "$OS" = "linux" ] && command -v systemctl &>/dev/null; then
    cat > /etc/systemd/system/zfsdash-agent.service <<EOF
[Unit]
Description=ZFSdash Agent
After=network.target

[Service]
ExecStart=$INSTALL_DIR/zfsdash agent --token=${TOKEN} --cloud-url=${CLOUD_URL}
Restart=always
RestartSec=30
Environment=HOME=/root

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now zfsdash-agent
    echo "==> Agent started (systemd)"

  elif [ "$OS" = "freebsd" ]; then
    cat > /etc/rc.d/zfsdash_agent <<EOF
#!/bin/sh
# PROVIDE: zfsdash_agent
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="zfsdash_agent"
rcvar="zfsdash_agent_enable"
command="$INSTALL_DIR/zfsdash"
command_args="agent --token=${TOKEN} --cloud-url=${CLOUD_URL}"
pidfile="/var/run/zfsdash_agent.pid"
start_precmd="zfsdash_prestart"

zfsdash_prestart() {
  export HOME=/root
}

load_rc_config \$name
run_rc_command "\$1"
EOF
    chmod +x /etc/rc.d/zfsdash_agent
    sysrc zfsdash_agent_enable=YES
    service zfsdash_agent start
    echo "==> Agent started (rc.d)"
  fi
else
  # Standalone dashboard mode
  CONFIG_DIR="/etc/zfsdash"
  mkdir -p "$CONFIG_DIR"

  if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cat > "$CONFIG_DIR/config.yaml" <<EOF
listen: :8080
database: /var/lib/zfsdash/zfsdash.db
hosts:
  - name: localhost
    mode: local
EOF
    mkdir -p /var/lib/zfsdash
    echo "==> Config written to $CONFIG_DIR/config.yaml"
  fi

  if [ "$OS" = "linux" ] && command -v systemctl &>/dev/null; then
    cat > /etc/systemd/system/zfsdash.service <<EOF
[Unit]
Description=ZFSdash
After=network.target

[Service]
ExecStart=$INSTALL_DIR/zfsdash serve --config /etc/zfsdash/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now zfsdash
  fi

  echo ""
  echo "==> ZFSdash installed!"
  echo "    Open http://localhost:8080 to get started"
  echo "    Config: $CONFIG_DIR/config.yaml"
fi
