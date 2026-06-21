#!/usr/bin/env bash
# scripts/build.sh
set -euo pipefail

# Resolve script directory to make it independent of working directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$WORKSPACE_DIR/build"

mkdir -p "$BUILD_DIR"

echo "===================================================="
echo "          Plomvix Cross-Compilation Build           "
echo "===================================================="

# Clean existing build binaries
rm -f "$BUILD_DIR"/plomvix-linux-*

# Explicitly use Linux target OS
export GOOS="linux"

# 1. Compile for x86_64 / AMD64 (Ubuntu, standard Linux, WSL)
echo "--> Compiling statically-linked binary for Linux x86_64 (amd64)..."
CGO_ENABLED=0 GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -o "$BUILD_DIR/plomvix-linux-amd64" \
  "$WORKSPACE_DIR/cmd/plomvix/main.go"

# 2. Compile for ARM64 (Raspberry Pi, AWS Graviton, Apple Silicon VMs)
echo "--> Compiling statically-linked binary for Linux ARM64 (arm64)..."
CGO_ENABLED=0 GOARCH=arm64 go build \
  -ldflags="-s -w" \
  -o "$BUILD_DIR/plomvix-linux-arm64" \
  "$WORKSPACE_DIR/cmd/plomvix/main.go"

echo "===================================================="
echo "Build complete. Available binaries in $BUILD_DIR:"
echo "===================================================="
ls -lh "$BUILD_DIR"
echo "===================================================="
