#!/bin/bash
set -e

APP="clipboardsync"
APP_NAME="ClipboardSync"
VERSION="0.1.0"

echo "Building $APP v$VERSION..."

mkdir -p dist

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o "dist/${APP}_${VERSION}_darwin_amd64" .
echo "  ✓ darwin/amd64"

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o "dist/${APP}_${VERSION}_darwin_arm64" .
echo "  ✓ darwin/arm64"

# Windows
GOOS=windows GOARCH=amd64 go build -o "dist/${APP}_${VERSION}_windows_amd64.exe" .
echo "  ✓ windows/amd64"

# ---- Package macOS .app bundle ----
echo ""
echo "Packaging macOS app..."

MACAPP="dist/${APP_NAME}.app"
MACOS_DIR="$MACAPP/Contents/MacOS"

rm -rf "$MACAPP"
mkdir -p "$MACOS_DIR"

# Universal binary via lipo
lipo -create -output "$MACOS_DIR/$APP" \
  "dist/${APP}_${VERSION}_darwin_amd64" \
  "dist/${APP}_${VERSION}_darwin_arm64"

# Info.plist: CFBundleExecutable points directly to the binary.
# LSUIElement=true hides from Dock.
cat > "$MACAPP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>${APP}</string>
  <key>CFBundleIdentifier</key>
  <string>com.${APP_NAME}</string>
  <key>CFBundleName</key>
  <string>${APP_NAME}</string>
  <key>CFBundleVersion</key>
  <string>${VERSION}</string>
  <key>CFBundleShortVersionString</key>
  <string>${VERSION}</string>
  <key>LSUIElement</key>
  <true/>
</dict>
</plist>
PLIST

echo "  ✓ ${APP_NAME}.app"

# ---- Package Windows zip ----
echo ""
echo "Packaging Windows installer..."

WIN_DIR="dist/${APP_NAME}_Windows_${VERSION}"
rm -rf "$WIN_DIR"
mkdir -p "$WIN_DIR"

cp "dist/${APP}_${VERSION}_windows_amd64.exe" "$WIN_DIR/${APP_NAME}.exe"
cp install.ps1 "$WIN_DIR/"

# Elevation batch file
cat > "$WIN_DIR/install.bat" <<BAT
@echo off
title ClipboardSync Installer
>nul 2>&1 "%SYSTEMROOT%\system32\cacls.exe" "%SYSTEMROOT%\system32\config\system"
if '%errorlevel%' NEQ '0' (
    echo Requesting administrator privileges...
    powershell Start-Process -FilePath "%~f0" -Verb runas
    exit /b
)
cd /d "%~dp0"
powershell -ExecutionPolicy Bypass -File install.ps1 -Command install
pause
BAT

cat > "$WIN_DIR/uninstall.bat" <<BAT
@echo off
title ClipboardSync Uninstaller
>nul 2>&1 "%SYSTEMROOT%\system32\cacls.exe" "%SYSTEMROOT%\system32\config\system"
if '%errorlevel%' NEQ '0' (
    echo Requesting administrator privileges...
    powershell Start-Process -FilePath "%~f0" -Verb runas
    exit /b
)
cd /d "%~dp0"
powershell -ExecutionPolicy Bypass -File install.ps1 -Command uninstall
pause
BAT

pushd dist >/dev/null
zip -r "${APP_NAME}_Windows_${VERSION}.zip" "${APP_NAME}_Windows_${VERSION}"
popd >/dev/null

echo "  ✓ ${APP_NAME}_Windows_${VERSION}.zip"

echo ""
echo "Done! Output in dist/:"
ls -lh dist/
