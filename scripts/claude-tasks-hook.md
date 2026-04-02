# /tasks — Task management command for Claude Code

Use this command to create and manage tasks in Tasks Watcher while you work.

## Quick Start

**Create a task:**
```
/tasks create --project <name> --title <title> [--priority low|medium|high|urgent]
```

**Mark task started/completed/failed:**
```
/tasks start <task-id>
/tasks complete <task-id>
/tasks fail <task-id> --reason <reason>
```

## Workflow

When starting a new feature or fix:
1. `/tasks create --project <my-project> --title <description>`
2. Copy the task ID from the output
3. `/tasks start <task-id>` to mark it in progress
4. Do your work...
5. `/tasks complete <task-id>` when done
6. Or `/tasks fail <task-id> --reason <error>` if it failed

## Examples

```
/tasks create --project myapp --title "Add user authentication" --priority high
/tasks start abc123
/tasks complete abc123
```

## Notes

- The CLI (`tasks-watcher`) must be in your PATH
- Set `TASKS_WATCHER_API_KEY` in your shell or use `tasks-watcher config show`
- For interactive mode, just run: `tasks-watcher task create`
