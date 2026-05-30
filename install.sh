#!/bin/bash
set -euo pipefail

APP_NAME="clipboardsync"
APP_PATH="/Applications/ClipboardSync.app/Contents/MacOS/clipboardsync"
PLIST_LABEL="com.clipboardsync"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
LOG_PATH="$HOME/Library/Logs/${APP_NAME}.log"
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

# Get shell config file (login shell config on macOS)
get_shell_config() {
  if [ -n "${ZSH_VERSION:-}" ]; then
    [ -f "$HOME/.zshrc" ] && echo "$HOME/.zshrc" || echo "$HOME/.zprofile"
  else
    [ -f "$HOME/.bash_profile" ] && echo "$HOME/.bash_profile" || echo "$HOME/.bashrc"
  fi
}

# Add to PATH
add_to_path() {
  local app_dir="/Applications/ClipboardSync.app/Contents/MacOS"

  # 创建符号链接到 /usr/local/bin (立即生效)
  sudo mkdir -p /usr/local/bin
  sudo ln -sf "${app_dir}/clipboardsync" /usr/local/bin/clipboardsync
  echo "  Symlinked to /usr/local/bin/clipboardsync"

  # 同时写入 shell config (新终端也生效)
  local config
  config="$(get_shell_config)"
  local export_line="export PATH=\"${app_dir}:\$PATH\""
  
  if [ -f "$config" ] && grep -q "$app_dir" "$config"; then
    echo "  PATH already configured in $config"
  else
    echo "" >> "$config"
    echo "# ClipboardSync" >> "$config"
    echo "$export_line" >> "$config"
    echo "  Added to PATH in $config"
  fi
}

# Remove from PATH
remove_from_path() {
  sudo rm -f /usr/local/bin/clipboardsync
  echo "  Removed /usr/local/bin/clipboardsync"

  local config
  config="$(get_shell_config)"
  local app_dir="/Applications/ClipboardSync.app/Contents/MacOS"
  
  if [ ! -f "$config" ]; then
    return
  fi
  
  sed -i.bak '/^# ClipboardSync$/d' "$config"
  sed -i.bak "\|${app_dir}|d" "$config"
  rm -f "${config}.bak"
  echo "  Removed from PATH in $config"
}

usage() {
  cat <<EOF
Usage: $0 {install|uninstall|status|help}

Commands:
  install    Copy app to /Applications, create plist, load LaunchAgent, add alias
  uninstall  Unload LaunchAgent, remove plist and alias
  status     Show LaunchAgent status
  help       Show this help
EOF
}

cmd_install() {
  echo "==> Installing Clipboard Sync..."

  local src
  src="$(find_binary)"
  echo "  Binary: $src"

  echo "  Copying app to /Applications..."
  sudo rm -rf /Applications/ClipboardSync.app
  sudo cp -R dist/ClipboardSync.app /Applications/

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
    <string>${APP_PATH}</string>
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

  echo "  Adding to PATH..."
  add_to_path

  echo "==> Done! Logs: $LOG_PATH"
  echo "  Loading shell config..."
  export PATH="/Applications/ClipboardSync.app/Contents/MacOS:$PATH"
  echo "  'clipboardsync' command is now available."
}

cmd_uninstall() {
  echo "==> Uninstalling Clipboard Sync..."

  echo "  Unloading LaunchAgent..."
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true

  echo "  Removing plist..."
  rm -f "$PLIST_PATH"

  echo "  Removing from PATH..."
  remove_from_path

  if [ -f "$LOG_PATH" ]; then
    read -r -p "  Remove log file? [y/N] " resp
    if [[ "$resp" =~ ^[yY] ]]; then
      rm -f "$LOG_PATH"
    fi
  fi

  echo "==> Done."
  echo "  Run 'source $(get_shell_config)' or restart terminal to apply changes."
}

cmd_status() {
  echo "==> LaunchAgent status:"
  if launchctl print "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null; then
    :
  else
    echo "  Not loaded"
  fi
  echo ""
  if [ -f "$APP_PATH" ]; then
    echo "  App: installed"
  else
    echo "  App: not found"
  fi
  if [ -f "$PLIST_PATH" ]; then
    echo "  Plist:  exists"
  else
    echo "  Plist:  not found"
  fi
  if grep -q "/Applications/ClipboardSync.app/Contents/MacOS" "$(get_shell_config)" 2>/dev/null; then
    echo "  PATH:   configured"
  else
    echo "  PATH:   not configured"
  fi
}

case "${1:-help}" in
  install)   cmd_install ;;
  uninstall) cmd_uninstall ;;
  status)    cmd_status ;;
  help|*)    usage ;;
esac
