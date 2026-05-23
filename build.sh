#!/bin/bash
set -e

APP="clipboardsync"
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

echo "Done! Binaries in dist/"
ls -lh dist/
