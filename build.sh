#!/bin/bash
set -e

APP="clipboardsync"
APP_NAME="ClipboardSync"
VERSION="1.1.0"

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

# ---- Package macOS zip ----
echo ""
echo "Packaging macOS installer..."

MAC_DIR="dist/${APP_NAME}_macOS_${VERSION}"
rm -rf "$MAC_DIR"
mkdir -p "$MAC_DIR"

# Create universal binary via lipo
lipo -create -output "$MAC_DIR/$APP" \
  "dist/${APP}_${VERSION}_darwin_amd64" \
  "dist/${APP}_${VERSION}_darwin_arm64"

# Copy install script
cp install.sh "$MAC_DIR/"

# Create README
cat > "$MAC_DIR/README.txt" <<EOF
ClipboardSync v${VERSION} for macOS

Installation:
  ./install.sh install

Uninstallation:
  ./install.sh uninstall

Status:
  ./install.sh status
EOF

# Create zip
pushd dist >/dev/null
zip -r "${APP_NAME}_macOS_${VERSION}.zip" "${APP_NAME}_macOS_${VERSION}"
popd >/dev/null

echo "  ✓ ${APP_NAME}_macOS_${VERSION}.zip"

# ---- Package Windows zip ----
echo ""
echo "Packaging Windows installer..."

WIN_DIR="dist/${APP_NAME}_Windows_${VERSION}"
rm -rf "$WIN_DIR"
mkdir -p "$WIN_DIR"

cp "dist/${APP}_${VERSION}_windows_amd64.exe" "$WIN_DIR/${APP_NAME}.exe"
cp install.ps1 "$WIN_DIR/"

# Create install batch file
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

# Create uninstall batch file
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

# Create README
cat > "$WIN_DIR/README.txt" <<EOF
ClipboardSync v${VERSION} for Windows

Installation:
  Right-click install.bat and select "Run as administrator"

Uninstallation:
  Right-click uninstall.bat and select "Run as administrator"

Status:
  Run: .\install.ps1 -Command status
EOF

pushd dist >/dev/null
zip -r "${APP_NAME}_Windows_${VERSION}.zip" "${APP_NAME}_Windows_${VERSION}"
popd >/dev/null

echo "  ✓ ${APP_NAME}_Windows_${VERSION}.zip"

# Clean up intermediate files
echo ""
echo "Cleaning up..."
rm -f "dist/${APP}_${VERSION}_darwin_amd64"
rm -f "dist/${APP}_${VERSION}_darwin_arm64"
rm -f "dist/${APP}_${VERSION}_windows_amd64.exe"
rm -rf "dist/${APP_NAME}_macOS_${VERSION}"
rm -rf "dist/${APP_NAME}_Windows_${VERSION}"

echo ""
echo "Done! Output in dist/:"
ls -lh dist/*.zip
