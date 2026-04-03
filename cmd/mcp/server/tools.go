package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/rogerrlee/tasks-watcher/cmd/mcp/client"
	"github.com/rogerrlee/tasks-watcher/pkg/mcp"
)

type Tool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]SchemaProp  `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

type SchemaProp struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

func GetToolDefinitions() []Tool {
	return []Tool{
		// Task tools
		{
			Name:        "task_create",
			Description: "Create a new task in Tasks Watcher. Call this when starting a new feature, bug fix, or any work item. Tasks can have a task_mode of 'sequential' (children must complete in order) or 'parallel' (children run independently). Parent tasks auto-complete when all children complete.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskCreateProps(),
				Required:   []string{"title"},
			},
		},
		{
			Name:        "task_list",
			Description: "List all tasks, optionally filtered by project, status, or assignee.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskListProps(),
				Required:   []string{},
			},
		},
		{
			Name:        "task_show",
			Description: "Show detailed information about a specific task.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskShowProps(),
				Required:   []string{"task_id"},
			},
		},
		{
			Name:        "task_start",
			Description: "Mark a task as in_progress. Call this when you begin working on a task.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskIDProps("task_id", "ID of the task to start"),
				Required:   []string{"task_id"},
			},
		},
		{
			Name:        "task_complete",
			Description: "Mark a task as completed. Call this when you finish a task successfully.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskIDProps("task_id", "ID of the task to mark complete"),
				Required:   []string{"task_id"},
			},
		},
		{
			Name:        "task_fail",
			Description: "Mark a task as failed. Use this when a task encounters an error that prevents completion.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskFailProps(),
				Required:   []string{"task_id", "reason"},
			},
		},
		{
			Name:        "task_update",
			Description: "Update a task's title, description, priority, assignee, or task mode.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskUpdateProps(),
				Required:   []string{"task_id"},
			},
		},
		{
			Name:        "task_cancel",
			Description: "Cancel a task.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: taskIDProps("task_id", "ID of the task to cancel"),
				Required:   []string{"task_id"},
			},
		},
		// Project tools
		{
			Name:        "project_list",
			Description: "List all projects in Tasks Watcher.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]SchemaProp{},
				Required:   []string{},
			},
		},
		{
			Name:        "project_create",
			Description: "Create a new project in Tasks Watcher.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: projectCreateProps(),
				Required:   []string{"name"},
			},
		},
		{
			Name:        "project_update",
			Description: "Update a project's name or description. Use this when project scope changes or you want to document what the project is about.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: projectUpdateProps(),
				Required:   []string{"project_id"},
			},
		},
	}
}

func taskCreateProps() map[string]SchemaProp {
	return map[string]SchemaProp{
		"title":       {Type: "string", Description: "Task title (required)"},
		"description": {Type: "string", Description: "Task description"},
		"project_name": {Type: "string", Description: "Project name (will auto-create if not exists)"},
		"priority":    {Type: "string", Description: "Priority: low, medium, high, urgent", Enum: []string{"low", "medium", "high", "urgent"}, Default: "medium"},
		"assignee":    {Type: "string", Description: "Assignee name (e.g., claude-code, cursor, human)"},
		"task_mode":   {Type: "string", Description: "Task ordering mode: sequential (children must complete in order) or parallel (children run independently)", Enum: []string{"sequential", "parallel"}},
	}
}

func taskListProps() map[string]SchemaProp {
	return map[string]SchemaProp{
		"project_id": {Type: "string", Description: "Filter by project ID"},
		"status":    {Type: "string", Description: "Filter by status", Enum: []string{"pending", "in_progress", "completed", "failed", "cancelled"}},
		"assignee":  {Type: "string", Description: "Filter by assignee"},
	}
}

func taskShowProps() map[string]SchemaProp {
	return taskIDProps("task_id", "ID of the task to show")
}

func taskFailProps() map[string]SchemaProp {
	props := taskIDProps("task_id", "ID of the task to mark as failed")
	props["reason"] = SchemaProp{Type: "string", Description: "Reason for failure (required)"}
	return props
}

func taskUpdateProps() map[string]SchemaProp {
	return map[string]SchemaProp{
		"task_id":   {Type: "string", Description: "ID of the task to update (required)"},
		"title":     {Type: "string", Description: "New task title"},
		"description": {Type: "string", Description: "New task description"},
		"priority":  {Type: "string", Description: "New priority", Enum: []string{"low", "medium", "high", "urgent"}},
		"assignee":  {Type: "string", Description: "New assignee"},
		"task_mode": {Type: "string", Description: "New task mode", Enum: []string{"sequential", "parallel"}},
	}
}

func taskIDProps(idField, desc string) map[string]SchemaProp {
	return map[string]SchemaProp{
		idField: {Type: "string", Description: desc},
	}
}

func projectCreateProps() map[string]SchemaProp {
	return map[string]SchemaProp{
		"name":        {Type: "string", Description: "Project name (required)"},
		"description": {Type: "string", Description: "Project description"},
		"repo_path":   {Type: "string", Description: "Repository path"},
	}
}

func projectUpdateProps() map[string]SchemaProp {
	return map[string]SchemaProp{
		"project_id":  {Type: "string", Description: "Project ID to update (required)"},
		"name":        {Type: "string", Description: "New project name"},
		"description": {Type: "string", Description: "New project description (use this to document project goals, tech stack, or progress)"},
		"repo_path":   {Type: "string", Description: "Repository path"},
	}
}

// ExecuteTool runs the named tool with given arguments
func ExecuteTool(ctx context.Context, api *client.Client, name string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	switch name {
	case "task_create":
		return api.TaskCreate(args)
	case "task_list":
		return api.TaskList(args)
	case "task_show":
		return api.TaskShow(args)
	case "task_start":
		return api.TaskStart(args)
	case "task_complete":
		return api.TaskComplete(args)
	case "task_fail":
		return api.TaskFail(args)
	case "task_update":
		return api.TaskUpdate(args)
	case "task_cancel":
		return api.TaskCancel(args)
	case "project_list":
		return api.ProjectList(args)
	case "project_create":
		return api.ProjectCreate(args)
	case "project_update":
		return api.ProjectUpdate(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func str(v interface{}, def string) string {
	if v == nil {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return strings.TrimSpace(s)
}

func strPtr(v interface{}) *string {
	s := str(v, "")
	return &s
}
