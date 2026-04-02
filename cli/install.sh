#!/bin/bash
# install.sh — downloads or builds the tasks-watcher CLI binary
set -e

BIN_DIR="$(cd "$(dirname "$0")/bin" && pwd)"
TMP_DIR="$(mktemp -d)"
OS="$(uname -s)"
ARCH="$(uname -m)"
REPO="RogerLiNing/tasks-watcher"

# Determine binary name
case "$OS" in
  Darwin*) PLATFORM="darwin-$(uname -m)" ;;
  Linux*)  PLATFORM="linux-$(uname -m)" ;;
  *)       echo "Unsupported platform: $OS"; exit 1 ;;
esac

echo "[tasks-watcher] Installing for $PLATFORM..."

# Try GitHub releases first
TAG="${npm_package_version:-latest}"
if [ "$TAG" = "latest" ]; then
  RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
else
  RELEASE_URL="https://api.github.com/repos/$REPO/releases/tags/$TAG"
fi

ASSET_NAME="tasks-watcher-$PLATFORM.tar.gz"
DOWNLOAD_URL=$(curl -s "$RELEASE_URL" | python3 -c "import sys,json; r=json.load(sys.stdin); assets=r.get('assets',[]); [print(a['browser_download_url']) for a in assets if '$ASSET_NAME' in a['name']]" 2>/dev/null | head -1)

if [ -n "$DOWNLOAD_URL" ]; then
  echo "[tasks-watcher] Downloading from GitHub releases..."
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/tasks-watcher.tar.gz"
  tar -xzf "$TMP_DIR/tasks-watcher.tar.gz" -C "$BIN_DIR"
  chmod +x "$BIN_DIR/tasks-watcher"
  echo "[tasks-watcher] Installed: $BIN_DIR/tasks-watcher"
else
  echo "[tasks-watcher] No pre-built binary found. Build from source:"
  echo "  git clone https://github.com/$REPO && cd tasks-watcher"
  echo "  go build -o bin/tasks-watcher ./cmd/cli"
  echo "  mv bin/tasks-watcher ~/.local/bin/ # or add to PATH"
fi

rm -rf "$TMP_DIR"
