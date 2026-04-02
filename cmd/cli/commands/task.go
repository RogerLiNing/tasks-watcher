package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func TaskCommand() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
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
	)

	return taskCmd
}

func apiRequest(method, path string, body interface{}) ([]byte, error) {
	serverURL, apiKey := resolveConfig()
	url := serverURL + path

	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %v\nMake sure the server is running (`tasks-watcher-server`)", serverURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func taskCreateCmd() *cobra.Command {
	var project, title, description, priority, assignee string

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
			}
			if priority != "" {
				body["priority"] = priority
			}
			if assignee != "" {
				body["assignee"] = assignee
			}

			resp, err := apiRequest("POST", "/api/tasks", body)
			if err != nil {
				return err
			}
			var task map[string]interface{}
			json.Unmarshal(resp, &task)
			fmt.Printf("✓ Task created: %s [%s]\n", task["title"], task["id"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project name")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Task title (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Task description")
	cmd.Flags().StringVarP(&priority, "priority", "P", "medium", "Priority: low, medium, high, urgent")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.MarkFlagRequired("title")
	return cmd
}

func taskListCmd() *cobra.Command {
	var project, status, assignee string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
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
			json.Unmarshal(resp, &result)
			tasks := result["tasks"]
			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			fmt.Printf("%-12s %-10s %-8s %s\n", "STATUS", "PRIORITY", "ASSIGNEE", "TITLE")
			fmt.Println(strings.Repeat("-", 90))
			for _, t := range tasks {
				status := fmt.Sprintf("%s", t["status"])
				priority := fmt.Sprintf("%s", t["priority"])
				asgn := fmt.Sprintf("%s", t["assignee"])
				title := fmt.Sprintf("%s", t["title"])
				if len(title) > 45 {
					title = title[:42] + "..."
				}
				fmt.Printf("%-12s %-10s %-8s %s\n", status, priority, asgn, title)
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
			json.Unmarshal(resp, &task)
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
			json.Unmarshal(resp, &task)
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
			json.Unmarshal(resp, &task)
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
			json.Unmarshal(resp, &task)
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
			json.Unmarshal(resp, &task)
			fmt.Printf("Title:       %s\n", task["title"])
			fmt.Printf("ID:          %s\n", task["id"])
			fmt.Printf("Project ID:  %s\n", task["project_id"])
			fmt.Printf("Status:      %s\n", task["status"])
			fmt.Printf("Priority:    %s\n", task["priority"])
			fmt.Printf("Assignee:    %s\n", task["assignee"])
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
