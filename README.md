# Tasks Watcher

A local-first task management system for humans and AI agents. Track all your projects across Claude Code, Cursor, and other tools in one unified dashboard.

## Quick Start

### 1. Build

```bash
./scripts/build.sh
```

### 2. Start the server

```bash
tasks-watcher-server
# or
~/.tasks-watcher/bin/tasks-watcher-server
```

The server runs at `http://localhost:4242`. On first run, it generates an API key at `~/.tasks-watcher/api.key`.

### 3. Open the dashboard

Open `http://localhost:4242` in your browser and paste your API key.

## CLI Usage

Install CLI in PATH:
```bash
export PATH="$HOME/.tasks-watcher/bin:$PATH"
export TASKS_WATCHER_API_KEY=$(cat ~/.tasks-watcher/api.key)
export TASKS_WATCHER_SERVER_URL=http://localhost:4242
```

### Task management

```bash
# Create a task
tasks-watcher task create -t "Fix auth bug" -p myproject -P high -a claude-code

# List tasks
tasks-watcher task list
tasks-watcher task list -s pending

# Start working on a task
tasks-watcher task start <task-id>

# Mark as completed
tasks-watcher task complete <task-id>

# Mark as failed
tasks-watcher task fail <task-id> -r "API not responding"

# Show task details
tasks-watcher task show <task-id>
```

### Project management

```bash
# Create a project
tasks-watcher project create -n myproject -d "My cool project"

# List projects
tasks-watcher project list
```

## AI Agent Integration

### Claude Code

Run the integration script:
```bash
./scripts/install-claude-integration.sh
```

Then in Claude Code, use:
```
/tasks create --project myproject --title "Implement feature X"
/tasks start <task-id>
/tasks complete <task-id>
```

### Direct CLI in scripts

Add to your Claude Code `.env` or shell profile:
```bash
export TASKS_WATCHER_API_KEY=$(cat ~/.tasks-watcher/api.key)
export TASKS_WATCHER_SERVER_URL=http://localhost:4242
```

Then in your tasks:
```bash
# When starting a task
tasks-watcher task create -t "My task" -p myproject -a claude-code

# When done
tasks-watcher task complete <task-id>
```

## Architecture

```
tasks-watcher/
├── cmd/
│   ├── server/          # REST API + SSE server
│   └── cli/             # CLI tool
├── internal/
│   ├── db/              # SQLite database layer
│   ├── models/          # Domain models
│   ├── api/             # HTTP handlers
│   ├── notifications/   # Notification dispatcher (SSE, macOS, webhooks)
│   └── config/          # Config loading
├── web/                 # Svelte dashboard
└── migrations/          # SQLite schema
```

## Tech Stack

- **Backend**: Go + SQLite (WAL mode)
- **Frontend**: Svelte + Vite
- **Real-time**: Server-Sent Events (SSE)
- **CLI**: Cobra
- **Notifications**: macOS Notification Center via osascript

## Data Location

- Database: `~/.tasks-watcher/tasks.db`
- API Key: `~/.tasks-watcher/api.key`
- Binaries: `~/.tasks-watcher/bin/`

## API Reference

Base URL: `http://localhost:4242/api`

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| GET | /projects | List projects |
| POST | /projects | Create project |
| GET | /tasks | List tasks |
| POST | /tasks | Create task |
| PATCH | /tasks/:id/status | Update task status |
| DELETE | /tasks/:id | Delete task |
| GET | /notifications | List notifications |
| GET | /events | SSE stream |
| GET | /export | Export all data |
