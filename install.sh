#!/bin/bash
set -e

APP="clipboardsync"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/clipboardsync"
LOG_FILE="$HOME/Library/Logs/clipboardsync.log"
PLIST_NAME="com.clipboardsync"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_NAME}.plist"

show_help() {
  echo "Usage: ./install.sh [install|uninstall|status]"
  echo ""
  echo "Commands:"
  echo "  install    Install binary and LaunchAgent"
  echo "  uninstall  Remove binary, LaunchAgent and config"
  echo "  status     Show installation status"
}

install() {
  echo "==> Installing Clipboard Sync..."

  # Find binary
  local script_dir
  script_dir="$(cd "$(dirname "$0")" && pwd)"
  local binary="$script_dir/$APP"

  if [ ! -f "$binary" ]; then
    echo "Error: Binary not found at $binary"
    echo "Please build first: ./build.sh"
    exit 1
  fi

  # Install binary
  echo "  Installing binary to $INSTALL_DIR/$APP..."
  if [ -w "$INSTALL_DIR" ]; then
    if ! cp "$binary" "$INSTALL_DIR/$APP" 2>/dev/null; then
      echo "  Need sudo for $INSTALL_DIR..."
      sudo cp "$binary" "$INSTALL_DIR/$APP"
    fi
    chmod +x "$INSTALL_DIR/$APP"
  else
    echo "  Need sudo for $INSTALL_DIR..."
    sudo cp "$binary" "$INSTALL_DIR/$APP"
    sudo chmod +x "$INSTALL_DIR/$APP"
  fi

  # Create config directory
  echo "  Creating config directory..."
  mkdir -p "$CONFIG_DIR"

  # Create default config if not exists
  if [ ! -f "$CONFIG_DIR/trusted.json" ]; then
    echo '{"trusted_uuids":[],"devices":{}}' > "$CONFIG_DIR/trusted.json"
  fi

  # Create LaunchAgent plist
  echo "  Creating LaunchAgent..."
  mkdir -p "$HOME/Library/LaunchAgents"
  cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_NAME}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${APP}</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>${HOME}</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_FILE}</string>
    <key>StandardErrorPath</key>
    <string>${LOG_FILE}</string>
</dict>
</plist>
EOF

  # Load LaunchAgent
  echo "  Loading LaunchAgent..."
  launchctl load "$PLIST_PATH" 2>/dev/null || true

  echo "==> Installation complete!"
  echo "    Binary: $INSTALL_DIR/$APP"
  echo "    Config: $CONFIG_DIR/"
  echo "    Log: $LOG_FILE"
  echo ""
  echo "    The service is now running in background."
  echo "    It will auto-start on login."
  echo ""
  echo "    Commands:"
  echo "      clipboardsync alias show    # 查看别名"
  echo "      clipboardsync alias set X   # 设置别名"
  echo "      clipboardsync trust         # 管理信任设备"
  echo "      clipboardsync status        # 查看状态"
}

uninstall() {
  echo "==> Uninstalling Clipboard Sync..."

  # Stop LaunchAgent
  echo "  Stopping LaunchAgent..."
  launchctl unload "$PLIST_PATH" 2>/dev/null || true

  # Stop running processes
  echo "  Stopping running processes..."
  pkill -f "$APP" 2>/dev/null || true
  sleep 1

  # Remove LaunchAgent plist
  echo "  Removing LaunchAgent..."
  rm -f "$PLIST_PATH"

  # Remove binary
  echo "  Removing binary..."
  if [ -f "$INSTALL_DIR/$APP" ]; then
    if [ -w "$INSTALL_DIR" ]; then
      if ! rm -f "$INSTALL_DIR/$APP" 2>/dev/null; then
        echo "  Failed to remove binary. Trying with sudo..."
        sudo rm -f "$INSTALL_DIR/$APP"
      fi
    else
      sudo rm -f "$INSTALL_DIR/$APP"
    fi
  fi

  # Remove config
  echo "  Removing config directory..."
  if [ -d "$CONFIG_DIR" ]; then
    rm -rf "$CONFIG_DIR"
  fi

  # Remove log file
  echo "  Removing log file..."
  if [ -f "$LOG_FILE" ]; then
    rm -f "$LOG_FILE"
  fi

  echo "==> Uninstallation complete!"
}

status() {
  echo "==> Clipboard Sync Status:"
  echo ""

  # Check binary
  if [ -f "$INSTALL_DIR/$APP" ]; then
    echo "  Binary: installed ($INSTALL_DIR/$APP)"
  else
    echo "  Binary: not installed"
  fi

  # Check LaunchAgent
  if [ -f "$PLIST_PATH" ]; then
    echo "  LaunchAgent: installed"
    if launchctl list | grep -q "$PLIST_NAME"; then
      echo "  LaunchAgent: loaded"
    else
      echo "  LaunchAgent: not loaded"
    fi
  else
    echo "  LaunchAgent: not installed"
  fi

  # Check config
  if [ -d "$CONFIG_DIR" ]; then
    echo "  Config: exists ($CONFIG_DIR/)"
  else
    echo "  Config: not found"
  fi

  # Check running
  if pgrep -f "$APP" > /dev/null 2>&1; then
    local pid
    pid=$(pgrep -f "$APP")
    echo "  Status: running (PID: $pid)"
  else
    echo "  Status: not running"
  fi

  # Check alias
  local alias
  alias=$("$INSTALL_DIR/$APP" alias show 2>/dev/null | grep "设备别名:" | cut -d: -f2 | xargs)
  if [ -n "$alias" ]; then
    echo "  Alias: $alias"
  fi
}

# Main
case "${1:-help}" in
  install)
    install
    ;;
  uninstall)
    uninstall
    ;;
  status)
    status
    ;;
  help|*)
    show_help
    ;;
esac
