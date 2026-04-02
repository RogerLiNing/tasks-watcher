package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func SubtaskCommand() *cobra.Command {
	stCmd := &cobra.Command{
		Use:   "subtask",
		Short: "Manage task subtasks",
		Long:  "Create, link, and list subtasks. A parent task auto-completes when all children complete.",
		Example: `  tasks-watcher task subtask <parent-id> -t "Design UI"
  tasks-watcher task subtask <parent-id> --add <child-id>
  tasks-watcher task subtask <parent-id> --list
  tasks-watcher task subtask <parent-id> --remove <child-id>`,
	}
	stCmd.AddCommand(
		subtaskCreateCmd(),
		subtaskLinkCmd(),
		subtaskListCmd(),
		subtaskRemoveCmd(),
	)
	return stCmd
}

func subtaskCreateCmd() *cobra.Command {
	var title, description, priority, assignee string
	cmd := &cobra.Command{
		Use:   "create <parent-id> -t <title>",
		Short: "Create a subtask under a parent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			resp, err := apiRequest("POST", "/api/tasks/"+args[0]+"/subtasks", body)
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
	cmd.Flags().StringVarP(&title, "title", "t", "", "Subtask title (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Subtask description")
	cmd.Flags().StringVarP(&priority, "priority", "P", "", "Priority: low, medium, high, urgent")
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "Assignee")
	cmd.MarkFlagRequired("title")
	return cmd
}

func subtaskLinkCmd() *cobra.Command {
	var childID string
	cmd := &cobra.Command{
		Use:   "link <parent-id> --add <child-id>",
		Short: "Link an existing task as a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if childID == "" {
				return fmt.Errorf("--add <child-id> is required")
			}
			resp, err := apiRequest("POST", "/api/tasks/"+args[0]+"/subtasks", map[string]string{"child_id": childID})
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			if task, ok := result["task"].(map[string]interface{}); ok {
				fmt.Printf("✓ Linked subtask %s under %s\n", task["title"], args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&childID, "add", "", "", "Existing task ID to link as subtask")
	return cmd
}

func subtaskListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <parent-id>",
		Short: "List subtasks of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			// List subtasks
			resp, err := apiRequest("GET", "/api/tasks/"+taskID+"/subtasks", nil)
			if err != nil {
				return err
			}
			var result map[string][]map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			subtasks := result["subtasks"]

			// Get parent info
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
	return cmd
}

func subtaskRemoveCmd() *cobra.Command {
	var childID string
	cmd := &cobra.Command{
		Use:   "remove <parent-id> --remove <child-id>",
		Short: "Remove a subtask from its parent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if childID == "" {
				return fmt.Errorf("--remove <child-id> is required")
			}
			_, err := apiRequest("DELETE", "/api/tasks/"+args[0]+"/subtasks/"+childID, nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Removed subtask %s from %s\n", childID, args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&childID, "remove", "r", "", "Subtask ID to remove")
	return cmd
}
