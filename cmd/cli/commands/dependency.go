package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// TaskDepCommand exposes dependency operations as `task dep <subcmd>`.
func TaskDepCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage task dependencies",
		Long:  "Add, remove, and list task blockers. A task cannot start until all its blockers are completed.",
	}
	cmd.AddCommand(
		taskDepAddCmd(),
		taskDepRemoveCmd(),
		taskDepListCmd(),
		taskDepCheckCmd(),
	)
	return cmd
}

func taskDepAddCmd() *cobra.Command {
	var taskID, blockerID string
	c := &cobra.Command{
		Use:   "add --task-id <id> --on <blocker-id>",
		Short: "Add a blocker to a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			if blockerID == "" {
				return fmt.Errorf("--on <blocker-id> is required")
			}
			resp, err := apiRequest("POST", "/api/tasks/"+taskID+"/dependencies", map[string]string{"blocker_id": blockerID})
			if err != nil {
				return err
			}
			var dep map[string]interface{}
			if err := json.Unmarshal(resp, &dep); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✓ Added blocker %s → %s\n", dep["blocker_id"], taskID)
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Task ID (required)")
	c.Flags().StringVar(&blockerID, "on", "", "Blocker task ID (required)")
	c.MarkFlagRequired("task-id")
	c.MarkFlagRequired("on")
	return c
}

func taskDepRemoveCmd() *cobra.Command {
	var taskID, blockerID string
	c := &cobra.Command{
		Use:   "remove --task-id <id> --remove <blocker-id>",
		Short: "Remove a blocker from a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			if blockerID == "" {
				return fmt.Errorf("--remove <blocker-id> is required")
			}
			_, err := apiRequest("DELETE", "/api/tasks/"+taskID+"/dependencies/"+blockerID, nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Removed blocker %s from %s\n", blockerID, taskID)
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Task ID (required)")
	c.Flags().StringVarP(&blockerID, "remove", "r", "", "Blocker task ID to remove")
	c.MarkFlagRequired("task-id")
	c.MarkFlagRequired("remove")
	return c
}

func taskDepListCmd() *cobra.Command {
	var taskID string
	c := &cobra.Command{
		Use:   "list --task-id <id>",
		Short: "List blockers and dependents of a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			resp, err := apiRequest("GET", "/api/tasks/"+taskID+"/dependencies", nil)
			if err != nil {
				return err
			}
			var blockResult map[string][]map[string]interface{}
			if err := json.Unmarshal(resp, &blockResult); err != nil {
				return fmt.Errorf("failed to parse blockers: %w", err)
			}
			blockers := blockResult["blockers"]

			respDep, err := apiRequest("GET", "/api/tasks/"+taskID+"/dependents", nil)
			if err != nil {
				return err
			}
			var depResult map[string][]map[string]interface{}
			if err := json.Unmarshal(respDep, &depResult); err != nil {
				return fmt.Errorf("failed to parse dependents: %w", err)
			}
			dependents := depResult["dependents"]

			fmt.Printf("Task: %s\n", taskID)
			fmt.Printf("Blocked by (%d):\n", len(blockers))
			if len(blockers) == 0 {
				fmt.Println("  (none)")
			}
			for _, b := range blockers {
				fmt.Printf("  [%s] %s\n", b["status"], b["title"])
			}
			fmt.Printf("Blocking (%d):\n", len(dependents))
			if len(dependents) == 0 {
				fmt.Println("  (none)")
			}
			for _, d := range dependents {
				fmt.Printf("  [%s] %s\n", d["status"], d["title"])
			}
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Task ID (required)")
	c.MarkFlagRequired("task-id")
	return c
}

func taskDepCheckCmd() *cobra.Command {
	var taskID string
	c := &cobra.Command{
		Use:   "can-start --task-id <id>",
		Short: "Check if a task can be started",
		RunE: func(cmd *cobra.Command, args []string) error {
			if taskID == "" {
				return fmt.Errorf("--task-id is required")
			}
			resp, err := apiRequest("GET", "/api/tasks/"+taskID+"/can-start", nil)
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			canStart := false
			if b, ok := result["can_start"].(bool); ok {
				canStart = b
			}
			if canStart {
				fmt.Printf("✓ Task %s can start\n", taskID)
			} else {
				fmt.Printf("✗ Task %s is blocked:\n", taskID)
				if blockers, ok := result["blockers"].([]interface{}); ok && len(blockers) > 0 {
					fmt.Println("  Blocked by incomplete tasks:")
					for _, b := range blockers {
						fmt.Printf("    - %s\n", b)
					}
				}
				if children, ok := result["child_titles"].([]interface{}); ok && len(children) > 0 {
					fmt.Println("  Has non-terminal subtasks:")
					for _, c := range children {
						fmt.Printf("    - %s\n", c)
					}
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&taskID, "task-id", "", "Task ID (required)")
	c.MarkFlagRequired("task-id")
	return c
}
