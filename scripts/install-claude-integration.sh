#!/bin/bash
# Claude Code integration for Tasks Watcher
# This script sets up Claude Code to automatically track tasks

set -e

echo "Setting up Claude Code integration for Tasks Watcher..."

# Detect Claude Code commands directory
CLAUDE_DIR=""
if [ -d "$HOME/.claude/commands" ]; then
  CLAUDE_DIR="$HOME/.claude/commands"
elif [ -d "$(pwd)/.claude/commands" ]; then
  CLAUDE_DIR="$(pwd)/.claude/commands"
else
  echo "Could not find .claude/commands directory."
  read -p "Create at ~/.claude/commands? [y/N] " yn
  if [ "$yn" = "y" ]; then
    mkdir -p "$HOME/.claude/commands"
    CLAUDE_DIR="$HOME/.claude/commands"
  else
    echo "Skipping Claude Code integration."
    exit 0
  fi
fi

# Copy the tasks command hook
SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
if [ -f "$SCRIPT_DIR/claude-tasks-hook.md" ]; then
  cp "$SCRIPT_DIR/claude-tasks-hook.md" "$CLAUDE_DIR/tasks.md"
  echo "✓ Installed /tasks command to $CLAUDE_DIR/tasks.md"
fi

# Ensure API key env var is set
if [ -z "$TASKS_WATCHER_API_KEY" ]; then
  if [ -f "$HOME/.tasks-watcher/api.key" ]; then
    echo ""
    echo "Add this to your ~/.zshrc to auto-configure the CLI:"
    echo ""
    echo "  export TASKS_WATCHER_API_KEY=\$(cat ~/.tasks-watcher/api.key)"
    echo "  export TASKS_WATCHER_SERVER_URL=http://localhost:4242"
    echo ""
  fi
fi

echo ""
echo "Integration setup complete!"
echo ""
echo "Usage in Claude Code:"
echo "  /tasks create --project myproject --title 'Implement feature X'"
echo "  /tasks start <task-id>"
echo "  /tasks complete <task-id>"
echo ""
echo "Or use the CLI directly:"
echo "  tasks-watcher task create -p myproject -t 'My task'"
echo "  tasks-watcher task complete <task-id>"
