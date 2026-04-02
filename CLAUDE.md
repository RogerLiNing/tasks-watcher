# Tasks Watcher

This project uses Tasks Watcher to track tasks across humans and AI agents.

## AI Agent Workflow

When starting work on a task, use the Tasks Watcher MCP tools:

1. **Create and start a task:**
   - Use `task_create` with title, project_name (use the current directory name or a meaningful project name), and assignee (e.g., "claude-code")

2. **Work on the task**

3. **Complete or fail:**
   - Use `task_complete` when done successfully
   - Use `task_fail` with a reason when the task cannot be completed

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `task_create` | Create a task and auto-start it |
| `task_list` | List tasks (filter by project, status, assignee) |
| `task_show` | Show task details |
| `task_start` | Mark task as in_progress |
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

The dashboard is at http://localhost:4242
