package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// TaskSubtaskCommand exposes subtask operations as `task subtask <subcmd>`.
func TaskSubtaskCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subtask",
		Short: "Manage task subtasks",
		Long:  "Create, link, and list subtasks. A parent task auto-completes when all children complete.",
	}
	cmd.AddCommand(
		taskSubtaskCreateCmd(),
		taskSubtaskLinkCmd(),
		taskSubtaskListCmd(),
		taskSubtaskRemoveCmd(),
	)
	return cmd
}

func taskSubtaskCreateCmd() *cobra.Command {
	var taskID, title, description, priority, assignee string
	c := &cobra.Command{
		Use:   "create --task-id <id> -t <title>",
		Short: "Create a subtask under a parent task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			if title == "" {
				return fmt.Errorf("-t <title> is required")
			}
			body := map[string]interface{}{"title": title}
			if description != "" {
				body["description"] = description
			}
			if priority != "" {
				body["priority"] = priority
			}
			if assignee != "" {
				body["assignee"] = assignee
			}
			resp, err := apiRequest("POST", "/api/tasks/"+taskID+"/subtasks", body)
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			if task, ok := result["task"].(map[string]interface{}); ok {
				fmt.Printf("✓ Subtask created: %s [%s]\n", task["title"], task["id"])
			}
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Parent task ID (required)")
	c.Flags().StringVarP(&title, "title", "t", "", "Subtask title (required)")
	c.Flags().StringVarP(&description, "description", "d", "", "Subtask description")
	c.Flags().StringVarP(&priority, "priority", "P", "", "Priority: low, medium, high, urgent")
	c.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	c.MarkFlagRequired("task-id")
	c.MarkFlagRequired("title")
	return c
}

func taskSubtaskLinkCmd() *cobra.Command {
	var taskID, childID string
	c := &cobra.Command{
		Use:   "link --task-id <id> --add <child-id>",
		Short: "Link an existing task as a subtask",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			if childID == "" {
				return fmt.Errorf("--add <child-id> is required")
			}
			resp, err := apiRequest("POST", "/api/tasks/"+taskID+"/subtasks", map[string]string{"child_id": childID})
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			if task, ok := result["task"].(map[string]interface{}); ok {
				fmt.Printf("✓ Linked subtask %s under %s\n", task["title"], taskID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Parent task ID (required)")
	c.Flags().StringVar(&childID, "add", "", "Existing task ID to link as subtask")
	c.MarkFlagRequired("task-id")
	c.MarkFlagRequired("add")
	return c
}

func taskSubtaskListCmd() *cobra.Command {
	var taskID string
	c := &cobra.Command{
		Use:   "list --task-id <id>",
		Short: "List subtasks of a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			resp, err := apiRequest("GET", "/api/tasks/"+taskID+"/subtasks", nil)
			if err != nil {
				return err
			}
			var result map[string][]map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			subtasks := result["subtasks"]

			parentResp, _ := apiRequest("GET", "/api/tasks/"+taskID, nil)
			var parent map[string]interface{}
			json.Unmarshal(parentResp, &parent)
			parentTitle, _ := parent["title"].(string)

			fmt.Printf("Parent: %s\n", parentTitle)
			fmt.Printf("Subtasks (%d):\n", len(subtasks))
			if len(subtasks) == 0 {
				fmt.Println("  (none)")
			}
			for _, s := range subtasks {
				fmt.Printf("  [%s] %s [%s]\n", s["status"], s["title"], s["id"])
			}
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Parent task ID (required)")
	c.MarkFlagRequired("task-id")
	return c
}

func taskSubtaskRemoveCmd() *cobra.Command {
	var taskID, childID string
	c := &cobra.Command{
		Use:   "remove --task-id <id> --remove <child-id>",
		Short: "Remove a subtask from its parent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			if childID == "" {
				return fmt.Errorf("--remove <child-id> is required")
			}
			_, err := apiRequest("DELETE", "/api/tasks/"+taskID+"/subtasks/"+childID, nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Removed subtask %s from %s\n", childID, taskID)
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Parent task ID (required)")
	c.Flags().StringVarP(&childID, "remove", "r", "", "Subtask ID to remove")
	c.MarkFlagRequired("task-id")
	c.MarkFlagRequired("remove")
	return c
}
