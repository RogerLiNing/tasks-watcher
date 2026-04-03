package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogerrlee/tasks-watcher/pkg/mcp"
)

// Client wraps the Tasks Watcher HTTP API
type Client struct {
	baseURL   string
	apiKey    string
	httpClient *http.Client
}

// Task and Project types
type Task struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"project_id"`
	Title       string            `json:"title"`
	Description map[string]string `json:"description"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	Assignee    string            `json:"assignee"`
	TaskMode    string            `json:"task_mode"`
	ErrorMsg    string            `json:"error_message,omitempty"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
	CompletedAt int64             `json:"completed_at,omitempty"`
}

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoPath    string `json:"repo_path"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func New() (*Client, error) {
	// Resolve server URL
	serverURL := os.Getenv("TASKS_WATCHER_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:4242"
	}

	// Resolve API key
	apiKey := os.Getenv("TASKS_WATCHER_API_KEY")
	if apiKey == "" {
		home, _ := os.UserHomeDir()
		keyPath := filepath.Join(home, ".tasks-watcher", "api.key")
		if data, err := os.ReadFile(keyPath); err == nil {
			apiKey = strings.TrimSpace(string(data))
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found: set TASKS_WATCHER_API_KEY or ensure ~/.tasks-watcher/api.key exists")
	}

	return &Client{
		baseURL:    serverURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) do(method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("connection failed: %w (is the server running at %s?)", err, c.baseURL)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

// Task operations

func (c *Client) TaskCreate(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	title := str(args["title"], "")
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	body := map[string]interface{}{"title": title, "source": "claude-code"}

	// Resolve project: explicit project_name > auto-detect git repo > default
	if proj := str(args["project_name"], ""); proj != "" {
		body["project_name"] = proj
	} else if repoPath := detectGitRepo(); repoPath != "" {
		projectID, err := c.getOrCreateProjectByRepo(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve project from repo: %w", err)
		}
		body["project_id"] = projectID
	}

	if desc := str(args["description"], ""); desc != "" {
		body["description"] = desc
	}
	if pri := str(args["priority"], "medium"); pri != "" {
		body["priority"] = pri
	}
	if asgn := str(args["assignee"], ""); asgn != "" {
		body["assignee"] = asgn
	}
	if mode := str(args["task_mode"], ""); mode != "" {
		body["task_mode"] = mode
	}

	data, status, err := c.do("POST", "/api/tasks", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	// Auto-start the task
	c.do("PATCH", "/api/tasks/"+task.ID+"/status", map[string]string{"status": "in_progress"})

	link := fmt.Sprintf("http://localhost:4242")
	text := fmt.Sprintf("✅ Task created and started: [%s]\nTitle: %s\nStatus: in_progress\nPriority: %s\nID: %s\n\nView at: %s",
		task.ID[:8], task.Title, task.Priority, task.ID, link)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) TaskList(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	path := "/api/tasks?"
	if pid := str(args["project_id"], ""); pid != "" {
		path += "project_id=" + pid + "&"
	}
	if status := str(args["status"], ""); status != "" {
		path += "status=" + status + "&"
	}
	if asgn := str(args["assignee"], ""); asgn != "" {
		path += "assignee=" + asgn + "&"
	}

	data, status, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var result struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	if len(result.Tasks) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "No tasks found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d task(s):\n\n", len(result.Tasks)))
	sb.WriteString(fmt.Sprintf("%-12s %-10s %-10s %s\n", "STATUS", "PRIORITY", "ASSIGNEE", "TITLE"))
	sb.WriteString(strings.Repeat("-", 85) + "\n")
	for _, t := range result.Tasks {
		title := t.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		asgn := t.Assignee
		if asgn == "" {
			asgn = "—"
		}
		sb.WriteString(fmt.Sprintf("%-12s %-10s %-10s %s [%s]\n",
			t.Status, t.Priority, asgn, title, t.ID[:8]))
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: sb.String()}},
	}, nil
}

func (c *Client) TaskShow(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	id := str(args["task_id"], "")
	if id == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	data, status, err := c.do("GET", "/api/tasks/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	text := fmt.Sprintf("Task: %s\nID: %s\nStatus: %s\nPriority: %s\nAssignee: %s\nTaskMode: %s\nCreated: %d",
		task.Title, task.ID, task.Status, task.Priority, task.Assignee, task.TaskMode, task.CreatedAt)
	if len(task.Description) > 0 {
		text += fmt.Sprintf("\nDescription: %v", task.Description)
	}
	if task.ErrorMsg != "" {
		text += fmt.Sprintf("\nError: %s", task.ErrorMsg)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) TaskStart(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	return c.updateStatus(args, "in_progress", "")
}

func (c *Client) TaskComplete(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	return c.updateStatus(args, "completed", "")
}

func (c *Client) TaskFail(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	reason := str(args["reason"], "Unknown error")
	return c.updateStatus(args, "failed", reason)
}

func (c *Client) TaskUpdate(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	id := str(args["task_id"], "")
	if id == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	body := map[string]interface{}{}
	if title := str(args["title"], ""); title != "" {
		body["title"] = title
	}
	if desc := str(args["description"], ""); desc != "" {
		body["description"] = desc
	}
	if pri := str(args["priority"], ""); pri != "" {
		body["priority"] = pri
	}
	if asgn := str(args["assignee"], ""); asgn != "" {
		body["assignee"] = asgn
	}
	if mode := str(args["task_mode"], ""); mode != "" {
		body["task_mode"] = mode
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("no fields to update: provide title, description, priority, assignee, or task_mode")
	}

	data, status, err := c.do("PUT", "/api/tasks/"+id, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	text := fmt.Sprintf("✅ Task updated: [%s] %s\nStatus: %s\nPriority: %s",
		task.ID[:8], task.Title, task.Status, task.Priority)

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) TaskCancel(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	return c.updateStatus(args, "cancelled", "")
}

func (c *Client) TaskDelete(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	id := str(args["task_id"], "")
	if id == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	_, status, err := c.do("DELETE", "/api/tasks/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d)", status)
	}

	text := fmt.Sprintf("🗑 Task deleted: [%s]", id[:8])
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) updateStatus(args map[string]interface{}, status, reason string) (*mcp.ToolsCallResult, error) {
	id := str(args["task_id"], "")
	if id == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	body := map[string]string{"status": status}
	if reason != "" {
		body["reason"] = reason
	}

	data, statusCode, err := c.do("PATCH", "/api/tasks/"+id+"/status", body)
	if err != nil {
		return nil, err
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", statusCode, string(data))
	}

	var task Task
	json.Unmarshal(data, &task)

	icon := "✅"
	msg := status
	if status == "in_progress" {
		icon = "▶️"
		msg = "started"
	} else if status == "failed" {
		icon = "❌"
		msg = "failed"
	} else if status == "cancelled" {
		icon = "○"
		msg = "cancelled"
	}

	text := fmt.Sprintf("%s Task %s: [%s] %s", icon, msg, task.ID[:8], task.Title)
	if reason != "" {
		text += fmt.Sprintf("\nReason: %s", reason)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

// Project operations

func (c *Client) ProjectList(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	data, status, err := c.do("GET", "/api/projects", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var result struct {
		Projects []Project `json:"projects"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	if len(result.Projects) == 0 {
		return &mcp.ToolsCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "No projects found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d project(s):\n\n", len(result.Projects)))
	for _, p := range result.Projects {
		desc := p.Description
		if desc == "" {
			desc = "—"
		}
		sb.WriteString(fmt.Sprintf("📁 %s\n   ID: %s\n   %s\n\n", p.Name, p.ID, desc))
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: sb.String()}},
	}, nil
}

func (c *Client) ProjectCreate(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	name := str(args["name"], "")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	body := map[string]interface{}{"name": name}
	if desc := str(args["description"], ""); desc != "" {
		body["description"] = desc
	}
	if repo := str(args["repo_path"], ""); repo != "" {
		body["repo_path"] = repo
	}

	data, status, err := c.do("POST", "/api/projects", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var proj Project
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	text := fmt.Sprintf("📁 Project created: %s [%s]", proj.Name, proj.ID)
	if proj.Description != "" {
		text += fmt.Sprintf("\nDescription: %s", proj.Description)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) ProjectUpdate(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	id := str(args["project_id"], "")
	if id == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	body := map[string]interface{}{}
	if name := str(args["name"], ""); name != "" {
		body["name"] = name
	}
	if desc := str(args["description"], ""); desc != "" {
		body["description"] = desc
	}
	if repo := str(args["repo_path"], ""); repo != "" {
		body["repo_path"] = repo
	}

	data, status, err := c.do("PUT", "/api/projects/"+id, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", status, string(data))
	}

	var proj Project
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	text := fmt.Sprintf("📁 Project updated: %s [%s]", proj.Name, proj.ID)
	if proj.Description != "" {
		text += fmt.Sprintf("\nDescription: %s", proj.Description)
	}

	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

func (c *Client) ProjectDelete(args map[string]interface{}) (*mcp.ToolsCallResult, error) {
	id := str(args["project_id"], "")
	if id == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	_, status, err := c.do("DELETE", "/api/projects/"+id, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d)", status)
	}

	text := fmt.Sprintf("🗑 Project deleted: [%s]", id[:8])
	return &mcp.ToolsCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}, nil
}

// str helper
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

// detectGitRepo returns the absolute path of the current git repository root,
// or an empty string if not inside a git repo.
func detectGitRepo() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Walk up from current directory to find .git
	for dir := cwd; dir != ""; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return dir
			}
			return abs
		}
		// Stop at filesystem root
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return ""
}

// getOrCreateProjectByRepo calls GET /projects/by-repo?repo_path=... to find or create
// the project for the given repository path.
func (c *Client) getOrCreateProjectByRepo(repoPath string) (string, error) {
	url := c.baseURL + "/api/projects/by-repo?repo_path=" + repoPath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to look up project: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &proj); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	return proj.ID, nil
}
