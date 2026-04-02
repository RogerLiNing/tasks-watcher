#!/bin/bash
set -e

cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

echo "Building Tasks Watcher..."

# Build Svelte frontend
echo "Building frontend..."
cd web
npm install
npm run build
cd ..

# Build Go server
echo "Building server..."
go build -o tasks-watcher-server ./cmd/server

# Build Go CLI
echo "Building CLI..."
go build -o tasks-watcher ./cmd/cli

# Build MCP server
echo "Building MCP server..."
go build -o tasks-watcher-mcp ./cmd/mcp

# Install to ~/.tasks-watcher/bin/
INSTALL_DIR="$HOME/.tasks-watcher/bin"
mkdir -p "$INSTALL_DIR"
cp tasks-watcher-server "$INSTALL_DIR/"
cp tasks-watcher "$INSTALL_DIR/"
cp tasks-watcher-mcp "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/tasks-watcher-server"
chmod +x "$INSTALL_DIR/tasks-watcher"
chmod +x "$INSTALL_DIR/tasks-watcher-mcp"

echo ""
echo "Build complete!"
echo "  Server:   ./tasks-watcher-server"
echo "  CLI:      ./tasks-watcher"
echo "  Installed: $INSTALL_DIR/"
echo ""
echo "To run the server:"
echo "  ./tasks-watcher-server"
echo "  or: ~/.tasks-watcher/bin/tasks-watcher-server"
echo ""
echo "To install CLI in PATH, add to ~/.zshrc:"
echo "  export PATH=\"\$HOME/.tasks-watcher/bin:\$PATH\""
echo ""
echo "Your API key is at: ~/.tasks-watcher/api.key"
echo "Set TASKS_WATCHER_API_KEY=\$(cat ~/.tasks-watcher/api.key) in your shell."
