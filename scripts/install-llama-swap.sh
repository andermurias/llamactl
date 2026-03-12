#!/usr/bin/env bash
# Download and install the llama-swap binary for macOS ARM64 (Apple Silicon).
# Project: https://github.com/mostlygeek/llama-swap

set -euo pipefail

INSTALL_DIR="/opt/homebrew/bin"
BINARY="$INSTALL_DIR/llama-swap"
REPO="mostlygeek/llama-swap"

echo "=== Installing llama-swap ==="

# Resolve latest release tag via GitHub API
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')

echo "Latest version: $LATEST"

URL="https://github.com/$REPO/releases/download/$LATEST/llama-swap-darwin-arm64"

echo "Downloading from: $URL"
curl -fsSL "$URL" -o "$BINARY"
chmod +x "$BINARY"

echo ""
echo "✅ llama-swap installed at $BINARY"
llama-swap --version 2>/dev/null || llama-swap -version 2>/dev/null || echo "(version flag not supported — binary is present)"
