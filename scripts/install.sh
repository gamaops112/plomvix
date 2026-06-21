#!/usr/bin/env bash
# scripts/install.sh
set -euo pipefail

# Make sure the script is run as root
if [ "$(id -u)" -ne 0 ]; then
  echo "Error: This script must be run as root (using sudo)." >&2
  exit 1
fi

echo "===================================================="
echo "          Plomvix Service Installer                 "
echo "===================================================="

# 1. Detect Architecture
ARCH=$(uname -m)
ARCH_SUFFIX=""
case "$ARCH" in
  x86_64|amd64)
    echo "Detected architecture: x86_64 / amd64"
    ARCH_SUFFIX="amd64"
    ;;
  aarch64|arm64)
    echo "Detected architecture: arm64 / aarch64"
    ARCH_SUFFIX="arm64"
    ;;
  *)
    echo "Error: Unsupported CPU architecture '$ARCH'." >&2
    exit 1
    ;;
esac

# 2. Retrieve Release Binary from GitHub
REPO="${GITHUB_REPO:-gamaops112/plomvix}"
VERSION="${VERSION:-latest}"

echo "Locating Plomvix release ($VERSION) from GitHub repository: $REPO..."

# Resolve Release Tag Name
if [ "$VERSION" = "latest" ]; then
  if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required to fetch latest release metadata from GitHub API." >&2
    exit 1
  fi
  TAG_NAME=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$TAG_NAME" ]; then
    echo "Error: Could not retrieve latest release tag from GitHub API for '$REPO'." >&2
    echo "Check network connection or ensure the repository has public releases." >&2
    exit 1
  fi
else
  TAG_NAME="$VERSION"
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$TAG_NAME/plomvix-linux-$ARCH_SUFFIX"
echo "Downloading binary from: $DOWNLOAD_URL"

# Download to temp path
TEMP_BINARY="/tmp/plomvix_download"
rm -f "$TEMP_BINARY"

if command -v curl >/dev/null 2>&1; then
  if ! curl -L -f -o "$TEMP_BINARY" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary from $DOWNLOAD_URL" >&2
    exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! wget -q -O "$TEMP_BINARY" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary from $DOWNLOAD_URL" >&2
    exit 1
  fi
else
  echo "Error: Neither curl nor wget is installed. Cannot download Plomvix." >&2
  exit 1
fi

chmod +x "$TEMP_BINARY"

# 3. Create System User and Group
echo "Creating 'plomvix' system user and group..."
if [ -f /etc/alpine-release ]; then
  # Alpine Linux (OpenRC)
  if ! getent group plomvix >/dev/null; then
    addgroup -S plomvix
  fi
  if ! getent passwd plomvix >/dev/null; then
    adduser -S -G plomvix -h /var/lib/plomvix -s /sbin/nologin plomvix
  fi
else
  # Debian/Ubuntu (Systemd, including WSL)
  if ! getent group plomvix >/dev/null; then
    groupadd -r plomvix
  fi
  if ! getent passwd plomvix >/dev/null; then
    useradd -r -g plomvix -d /var/lib/plomvix -s /usr/sbin/nologin -c "Plomvix Database Service Account" plomvix
  fi
fi

# 4. Create Directories & Set Permissions
echo "Configuring database system directories..."
mkdir -p /usr/local/bin
mkdir -p /etc/plomvix
mkdir -p /var/lib/plomvix/data/sql

# Set owner/permissions for runtime directories
chown -R plomvix:plomvix /var/lib/plomvix
chmod -R 770 /var/lib/plomvix

# 5. Install Binary
echo "Installing binary to /usr/local/bin/plomvix..."
mv "$TEMP_BINARY" /usr/local/bin/plomvix
chmod 755 /usr/local/bin/plomvix

# 6. Install Configuration File
if [ ! -f /etc/plomvix/config.toml ]; then
  echo "Installing default configuration to /etc/plomvix/config.toml..."
  if [ -f config.toml ]; then
    cp config.toml /etc/plomvix/config.toml
  elif [ -f config.example.toml ]; then
    cp config.example.toml /etc/plomvix/config.toml
  else
    # Fallback default configuration
    cat << 'EOF' > /etc/plomvix/config.toml
[server]
host = "127.0.0.1"
port = 5432
max_connections = 100
ssl_enabled = false
auth_type = "trust"

[logger]
level = "info"
format = "text"
output = "stdout"

[sql_engine]
data_dir = "data/sql"
max_mutation_rows = 1000
vacuum_workers = 2
vacuum_queue_size = 100

[storage]
db_path = "data/plomvix.db"
wal_path = "data/plomvix.wal"
cache_size_mb = 64
sync_writes = true
max_open_files = 256
EOF
  fi
  chown plomvix:plomvix /etc/plomvix/config.toml
  chmod 640 /etc/plomvix/config.toml
else
  echo "Existing config file detected at /etc/plomvix/config.toml. Skipping install."
fi

# 7. Init System Detection & Installation
echo "Detecting init system..."

# Check for Systemd
if [ -d /run/systemd/system ] || [ -f /sbin/init ] && /sbin/init --version 2>&1 | grep -q "systemd"; then
  echo "Detected: Systemd (Ubuntu / WSL / generic Linux)"
  
  # Write Systemd service file
  echo "Writing systemd unit service file..."
  cat << 'EOF' > /etc/systemd/system/plomvix.service
[Unit]
Description=Plomvix High-Performance Database Server
After=network.target

[Service]
Type=simple
User=plomvix
Group=plomvix
ExecStart=/usr/local/bin/plomvix -config /etc/plomvix/config.toml
WorkingDirectory=/var/lib/plomvix
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

  chmod 644 /etc/systemd/system/plomvix.service
  
  echo "Reloading systemd, enabling, and starting service..."
  systemctl daemon-reload
  systemctl enable plomvix
  systemctl start plomvix
  
  echo "Plomvix service successfully configured and started under systemd!"
  echo "Run 'systemctl status plomvix' to inspect status."

# Check for OpenRC (Alpine Linux)
elif [ -x /sbin/openrc-run ] || [ -f /etc/alpine-release ]; then
  echo "Detected: OpenRC (Alpine Linux)"
  
  # Write OpenRC init service script
  echo "Writing OpenRC service script..."
  cat << 'EOF' > /etc/init.d/plomvix
#!/sbin/openrc-run

description="Plomvix Database Server"
command="/usr/local/bin/plomvix"
command_args="-config /etc/plomvix/config.toml"
command_background="yes"
directory="/var/lib/plomvix"
pidfile="/run/plomvix.pid"
start_stop_daemon_args="--user plomvix:plomvix"

depend() {
        need net
        after firewall
}

start_pre() {
        checkpath --directory --owner plomvix:plomvix --mode 0770 /var/lib/plomvix
}
EOF

  chmod 755 /etc/init.d/plomvix
  
  echo "Enabling and starting service under OpenRC..."
  rc-update add plomvix default
  rc-service plomvix start
  
  echo "Plomvix service successfully configured and started under OpenRC!"

# WSL Systemd fallback
elif grep -qi microsoft /proc/version; then
  echo "Detected: WSL Environment without systemd enabled."
  echo "WARNING: Modern WSL Ubuntu supports Systemd, but it is currently disabled."
  echo "TIP: You can enable it by adding this to '/etc/wsl.conf' inside WSL and restarting:"
  echo ""
  echo "    [boot]"
  echo "    systemd=true"
  echo ""
  echo "Installing SysVinit/background runner script fallback..."
  
  # Create a basic background startup helper script
  cat << 'EOF' > /usr/local/bin/plomvix-service
#!/bin/bash
case "$1" in
  start)
    if pgrep -x "plomvix" > /dev/null; then
      echo "Plomvix is already running."
    else
      echo "Starting Plomvix in background..."
      cd /var/lib/plomvix
      sudo -u plomvix /usr/local/bin/plomvix -config /etc/plomvix/config.toml > /var/log/plomvix.log 2>&1 &
      echo "Started."
    fi
    ;;
  stop)
    echo "Stopping Plomvix..."
    pkill -x "plomvix" || true
    echo "Stopped."
    ;;
  status)
    if pgrep -x "plomvix" > /dev/null; then
      echo "Plomvix is running."
    else
      echo "Plomvix is stopped."
    fi
    ;;
  *)
    echo "Usage: plomvix-service {start|stop|status}"
    exit 1
    ;;
esac
EOF
  chmod 755 /usr/local/bin/plomvix-service
  
  # Start the service in the background
  /usr/local/bin/plomvix-service start
  echo "Plomvix successfully started in background!"
  echo "Control it using: plomvix-service {start|stop|status}"
else
  echo "Error: Unknown init system. Binary installed to /usr/local/bin/plomvix." >&2
  exit 1
fi

echo "===================================================="
echo "          Plomvix Installation Complete!            "
echo "===================================================="
