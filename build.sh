#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$ROOT/build"
SERVER_DIR="$ROOT/server"
CLIENT_DIR="$ROOT/client"

echo "=== Relay Build Script ==="
echo "Output: $BUILD_DIR"
echo ""

# Clean & recreate build dir
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# --- Linux Server ---
echo "[1/2] Building relay-server (linux/amd64)..."
cd "$SERVER_DIR"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BUILD_DIR/relay-server" .
cp config.yaml "$BUILD_DIR/config.yaml"
echo "  -> relay-server"

# --- Windows Client ---
echo "[2/2] Building relay-client (windows/amd64)..."
cd "$CLIENT_DIR"
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w -H=windowsgui" -o "$BUILD_DIR/relay-client.exe" .
echo "  -> relay-client.exe"

echo ""
echo "=== Done ==="
ls -lh "$BUILD_DIR"
