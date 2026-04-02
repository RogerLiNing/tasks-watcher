#!/bin/bash
# install.sh — downloads the tasks-watcher CLI binary from GitHub releases
set -e

BIN_DIR="${HOME}/.local/bin"
TMP_DIR="$(mktemp -d)"
OS="$(uname -s)"
ARCH="$(uname -m)"
REPO="RogerLiNing/tasks-watcher"

# Normalize OS and arch names to match GoReleaser output
case "$OS" in
  Darwin*) OS_NAME="darwin" ;;
  Linux*)  OS_NAME="linux" ;;
  *)       echo "Unsupported platform: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64)  ARCH_NAME="x86_64" ;;
  arm64)   ARCH_NAME="arm64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  *)       echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

echo "[tasks-watcher] Installing for $OS_NAME/$ARCH_NAME..."

# Determine tag
TAG="${npm_package_version:-}"
if [ -z "$TAG" ]; then
  TAG="latest"
fi

if [ "$TAG" = "latest" ]; then
  RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
else
  RELEASE_URL="https://api.github.com/repos/$REPO/releases/tags/$TAG"
fi

# Get version for naming
if [ "$TAG" = "latest" ]; then
  VERSION_INFO=$(curl -s "$RELEASE_URL")
  VERSION=$(echo "$VERSION_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])")
else
  VERSION="$TAG"
fi

# GoReleaser format: tasks-watcher_v{VERSION}_{OS}_{ARCH}.tar.gz
ASSET_NAME="tasks-watcher_${VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
DOWNLOAD_URL=$(curl -sf "$RELEASE_URL" | python3 -c "
import sys, json
try:
    r = json.load(sys.stdin)
    for a in r.get('assets', []):
        if '$ASSET_NAME' in a['name']:
            print(a['browser_download_url'])
            break
except:
    pass
")

if [ -n "$DOWNLOAD_URL" ]; then
  echo "[tasks-watcher] Downloading $ASSET_NAME..."
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/tasks-watcher.tar.gz"
  mkdir -p "$BIN_DIR"
  tar -xzf "$TMP_DIR/tasks-watcher.tar.gz" -C "$BIN_DIR"
  chmod +x "$BIN_DIR/tasks-watcher"
  echo "[tasks-watcher] Installed: $BIN_DIR/tasks-watcher"
  echo "[tasks-watcher] Add $BIN_DIR to your PATH if needed."
else
  echo "[tasks-watcher] Release not found. Build from source:"
  echo "  git clone https://github.com/$REPO && cd tasks-watcher"
  echo "  go build -o bin/tasks-watcher ./cmd/cli && mv bin/tasks-watcher ~/.local/bin/"
  exit 1
fi

rm -rf "$TMP_DIR"
