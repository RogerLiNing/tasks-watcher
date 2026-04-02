---
name: tasks-watcher-cli
description: CLI workflow for tasks-watcher — how to use tasks-watcher task, project, and agents commands from the command line. Use this skill when the user wants to interact via terminal or when MCP is unavailable.
license: MIT
---

# Tasks Watcher CLI

Use the `tasks-watcher` CLI when the user wants terminal interaction or MCP is not configured.

## Installation

```bash
# npm (recommended)
npm install -g tasks-watcher

# or from source
go build -o $HOME/.local/bin/tasks-watcher ./cmd/cli
```

The server must be running: `tasks-watcher-server`

## Core Commands

### Task Commands

```bash
# Create a task (auto-starts it, sets source=cli)
tasks-watcher task create -t "Implement user auth" -P high -p myproject

# List tasks
tasks-watcher task list
tasks-watcher task list -s in_progress      # filter by status
tasks-watcher task list -p <project-id>      # filter by project

# Update status
tasks-watcher task start <task-id>
tasks-watcher task complete <task-id>
tasks-watcher task fail <task-id> -r "authentication logic broken"
tasks-watcher task cancel <task-id>

# Show task details
tasks-watcher task show <task-id>

# Delete a task
tasks-watcher task delete <task-id>
```

### Project Commands

```bash
tasks-watcher project create -n myproject -d "Backend API service"
tasks-watcher project list
tasks-watcher project delete <project-id>
```

### Agent Overview

```bash
# See what each agent is currently working on
tasks-watcher agents overview
```

Shows:
- 🤖 Claude Code agents and their active tasks
- 📎 Cursor agents and their active tasks
- 👤 Manual entries
- Per-agent stats: total, done, pending, failed

### Config

```bash
tasks-watcher config show    # show server URL and API key
tasks-watcher config api-key # print API key
```

## When to Use CLI vs MCP

| Scenario | Use |
|----------|-----|
| Claude Code session | MCP tools (native integration) |
| Cursor session | MCP tools |
| Terminal / shell | CLI (`tasks-watcher`) |
| Dashboard UI | http://localhost:4242 |
| Scripted automation | CLI + API |

## Task Status

```
pending → in_progress → completed
                      → failed → in_progress (retry)
                      → cancelled
```

Priority: `urgent` > `high` > `medium` > `low`

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Task title (required for create) |
| `--description` | `-d` | Task description |
| `--project` | `-p` | Project name (auto-creates if not exists) |
| `--priority` | `-P` | Priority: low, medium, high, urgent |
| `--assignee` | `-a` | Assignee name |
| `--status` | `-s` | Filter by status |
| `--reason` | `-r` | Reason (for fail command) |

## Examples

```bash
# Start working on a new feature
tasks-watcher task create -t "Add search filter" -P medium -p myproject -a claude-code
# → returns task ID

# Check what agents are doing right now
tasks-watcher agents overview

# Resume a failed task
tasks-watcher task start <task-id>

# Mark done with explanation
tasks-watcher task complete <task-id>
```
