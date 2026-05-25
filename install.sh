#!/bin/bash
set -euo pipefail

APP_NAME="clipboardsync"
BIN_NAME="clipboardsync"
PLIST_LABEL="com.clipboardsync"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
BIN_PATH="/usr/local/bin/${BIN_NAME}"
LOG_PATH="$HOME/Library/Logs/${APP_NAME}.log"
BIN_DIR="$(dirname "$BIN_PATH")"
VERSION="0.1.0"

# Detect current arch
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  GOARCH="amd64" ;;
  arm64)   GOARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Find the right binary in dist/
find_binary() {
  local pattern="${APP_NAME}_${VERSION}_darwin_${GOARCH}"
  local result
  result=$(compgen -G "dist/$pattern" 2>/dev/null || true)
  if [ -z "$result" ]; then
    echo "Binary not found matching: dist/$pattern"
    echo "Run './build.sh' first."
    exit 1
  fi
  echo "$result"
}

check_sudo() {
  if ! sudo -v &>/dev/null; then
    echo "This command requires sudo privileges."
    exit 1
  fi
}

usage() {
  cat <<EOF
Usage: $0 {install|uninstall|status|help}

Commands:
  install    Copy binary, create plist, load LaunchAgent
  uninstall  Unload LaunchAgent, remove plist and binary
  status     Show LaunchAgent status
  help       Show this help
EOF
}

cmd_install() {
  echo "==> Installing Clipboard Sync..."
  check_sudo

  local src
  src="$(find_binary)"
  echo "  Binary: $src"

  echo "  Copying to $BIN_PATH..."
  sudo mkdir -p "$BIN_DIR"
  sudo cp "$src" "$BIN_PATH"
  sudo chmod 755 "$BIN_PATH"

  mkdir -p "$(dirname "$PLIST_PATH")"

  echo "  Creating config..."
  CONFIG_DIR="$HOME/.config/clipboardsync"
  mkdir -p "$CONFIG_DIR"
  if [ ! -f "$CONFIG_DIR/trusted.json" ]; then
    echo '{"trusted_uuids":[],"devices":{}}' > "$CONFIG_DIR/trusted.json"
  fi
  echo "  Run 'clipboardsync trust' to configure trusted devices."

  echo "  Writing plist..."
  cat > "$PLIST_PATH" <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${PLIST_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BIN_PATH}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
  <key>StandardOutPath</key>
  <string>${LOG_PATH}</string>
  <key>StandardErrorPath</key>
  <string>${LOG_PATH}</string>
</dict>
</plist>
PLISTEOF

  echo "  Loading LaunchAgent..."
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"

  echo "==> Done! Logs: $LOG_PATH"
}

cmd_uninstall() {
  echo "==> Uninstalling Clipboard Sync..."
  check_sudo

  echo "  Unloading LaunchAgent..."
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true

  echo "  Removing plist..."
  rm -f "$PLIST_PATH"

  echo "  Removing binary..."
  sudo rm -f "$BIN_PATH"

  if [ -f "$LOG_PATH" ]; then
    read -r -p "  Remove log file? [y/N] " resp
    if [[ "$resp" =~ ^[yY] ]]; then
      rm -f "$LOG_PATH"
    fi
  fi

  echo "==> Done."
}

cmd_status() {
  echo "==> LaunchAgent status:"
  if launchctl print "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null; then
    :
  else
    echo "  Not loaded"
  fi
  echo ""
  if [ -f "$BIN_PATH" ]; then
    echo "  Binary: installed"
  else
    echo "  Binary: not found"
  fi
  if [ -f "$PLIST_PATH" ]; then
    echo "  Plist:  exists"
  else
    echo "  Plist:  not found"
  fi
}

case "${1:-help}" in
  install)   cmd_install ;;
  uninstall) cmd_uninstall ;;
  status)    cmd_status ;;
  help|*)    usage ;;
esac
