# Tasks Watcher CLI

Task management CLI for humans and AI agents — unified view across Claude Code, Cursor, and manual entry.

## Quick Start

```bash
# npm (recommended)
npm install -g tasks-watcher

# or build from source
git clone https://github.com/RogerLiNing/tasks-watcher
cd tasks-watcher
go build -o bin/tasks-watcher ./cmd/cli
mv bin/tasks-watcher /usr/local/bin/

# ensure server is running
tasks-watcher-server

# create a task
tasks-watcher task create -t "Implement auth" -P high

# see what agents are doing
tasks-watcher agents overview
```

## Commands

```
tasks-watcher task [create|list|start|complete|fail|cancel|show|delete|heartbeat]
tasks-watcher project [create|list|delete]
tasks-watcher agents overview
tasks-watcher config [show|api-key]
```

## Task Lifecycle

```
pending → in_progress → completed
                      → failed → in_progress (retry)
                      → cancelled
```

## AI Agent Integration

The CLI is designed for AI agents (Claude Code, Cursor) to track their work.
When an AI creates a task, it's automatically tagged with its source:

- `claude-code` — tasks created by Claude Code via MCP
- `cursor` — tasks created by Cursor
- `cli` — tasks created via this CLI
- `manual` — tasks created via the dashboard

Use `tasks-watcher agents overview` to see what each agent is currently doing.

## Server

The CLI requires the tasks-watcher server running at `http://localhost:4242`.

Set `TASKS_WATCHER_SERVER_URL` to override, or `TASKS_WATCHER_API_KEY` to override the key.
Keys are stored at `~/.tasks-watcher/api.key`.
