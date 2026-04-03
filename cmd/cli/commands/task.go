package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func TaskCommand() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
		Long:  "Create, list, update, and delete tasks. Each task is tagged with its source. Tasks can be marked as sequential (children must complete in order) or parallel (children run independently).",
		Example: `  tasks-watcher task create -t "Fix auth bug" -P high -p myproject
  tasks-watcher task create -t "Multi-step refactor" --task-mode sequential
  tasks-watcher task list -s pending
  tasks-watcher task start <task-id>
  tasks-watcher task complete <task-id>
  tasks-watcher task fail <task-id> -r "API not responding"
  tasks-watcher task show <task-id>
  tasks-watcher task delete <task-id>`,
	}

	taskCmd.AddCommand(
		taskCreateCmd(),
		taskListCmd(),
		taskStartCmd(),
		taskCompleteCmd(),
		taskFailCmd(),
		taskCancelCmd(),
		taskShowCmd(),
		taskDeleteCmd(),
		taskHeartbeatCmd(),
		TaskDepCommand(),
		TaskSubtaskCommand(),
	)

	return taskCmd
}

func apiRequest(method, path string, body interface{}) ([]byte, error) {
	serverURL, apiKey := resolveConfig()
	url := serverURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %v\nMake sure the server is running (`tasks-watcher-server`)", serverURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func taskCreateCmd() *cobra.Command {
	var project, title, description, priority, assignee, taskMode string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			body := map[string]interface{}{"title": title, "source": "cli"}
			if description != "" {
				body["description"] = description
			}
			if project != "" {
				body["project_name"] = project
			} else {
				// Auto-detect current git repo and associate project
				projID, repoPath, err := resolveProjectFromGit()
				if err != nil {
					return fmt.Errorf("failed to resolve project from git: %w", err)
				}
				if projID != "" {
					body["project_id"] = projID
					fmt.Printf("📁 Auto-linked to project: %s (%s)\n", repoPath, projID[:8])
				}
			}
			if priority != "" {
				body["priority"] = priority
			}
			if assignee != "" {
				body["assignee"] = assignee
			}
			if taskMode != "" {
				body["task_mode"] = taskMode
			}

			resp, err := apiRequest("POST", "/api/tasks", body)
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✓ Task created: %s [%s]\n", task["title"], task["id"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name (auto-detected from git repo if omitted)")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Task title (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Task description")
	cmd.Flags().StringVarP(&priority, "priority", "P", "medium", "Priority: low, medium, high, urgent")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.Flags().StringVar(&taskMode, "task-mode", "", "Task mode: sequential or parallel")
	cmd.MarkFlagRequired("title")
	return cmd
}

func taskListCmd() *cobra.Command {
	var project, status, assignee string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List tasks",
		Example: "  tasks-watcher task list\n  tasks-watcher task list -s in_progress\n  tasks-watcher task list -p <project-id>",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/tasks?"
			if project != "" {
				path += "project_id=" + project + "&"
			}
			if status != "" {
				path += "status=" + status + "&"
			}
			if assignee != "" {
				path += "assignee=" + assignee + "&"
			}

			resp, err := apiRequest("GET", path, nil)
			if err != nil {
				return err
			}
			var result map[string][]map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			tasks := result["tasks"]
			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			fmt.Printf("%-12s %-10s %-8s %-12s %s\n", "STATUS", "PRIORITY", "ASSIGNEE", "MODE/SOURCE", "TITLE")
			fmt.Println(strings.Repeat("-", 90))
			for _, t := range tasks {
				status := fmt.Sprintf("%s", t["status"])
				priority := fmt.Sprintf("%s", t["priority"])
				asgn := fmt.Sprintf("%s", t["assignee"])
				title := fmt.Sprintf("%s", t["title"])
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				modeSrc := ""
				if m, ok := t["task_mode"].(string); ok && m != "" {
					modeSrc = m
				}
				if s, ok := t["source"].(string); ok && s != "" && s != "manual" {
					if modeSrc != "" {
						modeSrc += " " + s
					} else {
						modeSrc = s
					}
				}
				fmt.Printf("%-12s %-10s %-8s %-12s %s\n", status, priority, asgn, modeSrc, title)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Filter by project ID")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Filter by assignee")
	return cmd
}

func taskStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <task-id>",
		Short: "Start a task (mark as in_progress)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("PATCH", "/api/tasks/"+args[0]+"/status", map[string]string{"status": "in_progress"})
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✓ Task started: %s\n", task["title"])
			return nil
		},
	}
}

func taskCompleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "complete <task-id>",
		Short: "Mark a task as completed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("PATCH", "/api/tasks/"+args[0]+"/status", map[string]string{"status": "completed"})
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✓ Task completed: %s\n", task["title"])
			return nil
		},
	}
}

func taskFailCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "fail <task-id>",
		Short: "Mark a task as failed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("PATCH", "/api/tasks/"+args[0]+"/status", map[string]string{
				"status": "failed",
				"reason": reason,
			})
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✗ Task failed: %s\n", task["title"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason for failure")
	return cmd
}

func taskCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <task-id>",
		Short: "Cancel a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("PATCH", "/api/tasks/"+args[0]+"/status", map[string]string{"status": "cancelled"})
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("○ Task cancelled: %s\n", task["title"])
			return nil
		},
	}
}

func taskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/tasks/"+args[0], nil)
			if err != nil {
				return err
			}
			var task map[string]interface{}
			if err := json.Unmarshal(resp, &task); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Header
			title := task["title"]
			fmt.Printf("=== %s ===\n", title)
			fmt.Printf("ID:        %s\n", task["id"])
			if p, ok := task["project_id"].(string); ok && p != "" {
				fmt.Printf("Project:   %s\n", p)
			}
			fmt.Printf("Status:    %s\n", task["status"])
			fmt.Printf("Priority:  %s\n", task["priority"])
			if a, ok := task["assignee"].(string); ok && a != "" {
				fmt.Printf("Assignee:  %s\n", a)
			}
			if s, ok := task["source"].(string); ok && s != "" {
				fmt.Printf("Source:    %s\n", s)
			}
			if m, ok := task["task_mode"].(string); ok && m != "" {
				fmt.Printf("Mode:      %s\n", m)
			}
			if desc, ok := task["description"].(string); ok && desc != "" {
				if len(desc) > 200 {
					desc = desc[:197] + "..."
				}
				fmt.Printf("Desc:      %s\n", desc)
			}
			if errMsg, ok := task["error_message"].(string); ok && errMsg != "" {
				fmt.Printf("Error:     %s\n", errMsg)
			}

			// Subtasks
			subResp, _ := apiRequest("GET", "/api/tasks/"+args[0]+"/subtasks", nil)
			var subResult map[string][]map[string]interface{}
			if json.Unmarshal(subResp, &subResult) == nil {
				subtasks := subResult["subtasks"]
				fmt.Printf("\nSubtasks (%d):\n", len(subtasks))
				if len(subtasks) == 0 {
					fmt.Println("  (none)")
				}
				for _, s := range subtasks {
					pos := 0
					if p, ok := s["position"].(float64); ok {
						pos = int(p)
					}
					fmt.Printf("  [%d] [%s] %s\n", pos, s["status"], s["title"])
				}
			}

			// Blockers
			blockResp, _ := apiRequest("GET", "/api/tasks/"+args[0]+"/dependencies", nil)
			var blockResult map[string][]map[string]interface{}
			if json.Unmarshal(blockResp, &blockResult) == nil {
				blockers := blockResult["blockers"]
				fmt.Printf("\nBlocked by (%d):\n", len(blockers))
				if len(blockers) == 0 {
					fmt.Println("  (none)")
				}
				for _, b := range blockers {
					fmt.Printf("  [%s] %s\n", b["status"], b["title"])
				}
			}

			// Dependents
			depResp, _ := apiRequest("GET", "/api/tasks/"+args[0]+"/dependents", nil)
			var depResult map[string][]map[string]interface{}
			if json.Unmarshal(depResp, &depResult) == nil {
				dependents := depResult["dependents"]
				fmt.Printf("\nBlocking (%d):\n", len(dependents))
				if len(dependents) == 0 {
					fmt.Println("  (none)")
				}
				for _, d := range dependents {
					fmt.Printf("  [%s] %s\n", d["status"], d["title"])
				}
			}

			return nil
		},
	}
}

func taskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiRequest("DELETE", "/api/tasks/"+args[0], nil)
			if err != nil {
				return err
			}
			id := args[0]
			if len(id) > 8 {
				id = id[:8]
			}
			fmt.Printf("✓ Task deleted: %s\n", id)
			return nil
		},
	}
}

func taskHeartbeatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "heartbeat <task-id>",
		Short: "Send a heartbeat to keep a task alive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiRequest("POST", "/api/tasks/"+args[0]+"/heartbeat", nil)
			return err
		},
	}
}

// detectGitRepo walks up from the current directory looking for .git
func detectGitRepo() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; dir != ""; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			abs, _ := filepath.Abs(dir)
			return abs
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return ""
}

// resolveProjectFromGit calls GET /projects/by-repo?repo_path=... to find or create
// the project for the current git repository.
func resolveProjectFromGit() (string, string, error) {
	repoPath := detectGitRepo()
	if repoPath == "" {
		return "", "", nil
	}
	// Use exec.Command for git rev-parse as a fallback / validation
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", repoPath, nil
	}
	repoPath = strings.TrimSpace(string(out))

	serverURL, apiKey := resolveConfig()
	url := serverURL + "/api/projects/by-repo?repo_path=" + repoPath
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", repoPath, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", repoPath, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", repoPath, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}
	var proj struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &proj); err != nil {
		return "", repoPath, fmt.Errorf("invalid response: %w", err)
	}
	return proj.ID, repoPath, nil
}
