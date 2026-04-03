# Tasks Watcher

This project uses Tasks Watcher to track tasks across humans and AI agents.

## AI Agent Workflow

When starting work on a task, use the Tasks Watcher MCP tools:

1. **Create and start a task:**
   - Use `task_create` with `title`, `project_name` (use the current directory name or a meaningful project name), `assignee` (e.g., "claude-code"), and optionally `task_mode`

2. **Work on the task**

3. **Complete or fail:**
   - Use `task_complete` when done successfully
   - Use `task_fail` with a reason when the task cannot be completed

## Task Modes

Tasks support two ordering modes:
- **`task_mode: "sequential"`** — Children must complete in order. Child N cannot start until child N-1 reaches a terminal state (completed/failed/cancelled). If a sequential child fails, the parent task also fails.
- **`task_mode: "parallel"`** (default) — All children run independently. Parent auto-completes when all children complete.

Use `--task-mode sequential` in CLI or `task_mode: "sequential"` in MCP to create parent tasks with ordered subtasks.

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `task_create` | Create a task and auto-start it. Supports `task_mode: "sequential"` or `"parallel"` |
| `task_list` | List tasks (filter by project, status, assignee) |
| `task_show` | Show task details |
| `task_start` | Mark task as in_progress (blocked by incomplete dependencies or sequential order) |
| `task_complete` | Mark task as completed |
| `task_fail` | Mark task as failed with reason |
| `task_cancel` | Cancel a task |
| `project_list` | List all projects |
| `project_create` | Create a project |

## Quick Usage

When working on any feature or bug:
```
Use task_create: title="Fix auth bug", project_name="tasks-watcher", assignee="claude-code"
Use task_complete when done
```

For multi-step work with ordered subtasks:
```
Use task_create: title="Refactor auth module", task_mode="sequential", assignee="claude-code"
```

The dashboard is at http://localhost:4242
