package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func DependencyCommand() *cobra.Command {
	depCmd := &cobra.Command{
		Use:   "depend",
		Short: "Manage task dependencies",
		Long:  "Add, remove, and list task blockers. A task cannot start until all its blockers are completed.",
		Example: `  tasks-watcher task depend <task-id> --on <blocker-id>
  tasks-watcher task depend <task-id> --list
  tasks-watcher task depend <task-id> --remove <blocker-id>
  tasks-watcher task depend <task-id> --can-start`,
	}
	depCmd.AddCommand(
		dependAddCmd(),
		dependRemoveCmd(),
		dependListCmd(),
		dependCheckCmd(),
	)
	return depCmd
}

func dependAddCmd() *cobra.Command {
	var blockerID string
	cmd := &cobra.Command{
		Use:   "add <task-id> --on <blocker-id>",
		Short: "Add a blocker to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if blockerID == "" {
				return fmt.Errorf("--on <blocker-id> is required")
			}
			resp, err := apiRequest("POST", "/api/tasks/"+args[0]+"/dependencies", map[string]string{"blocker_id": blockerID})
			if err != nil {
				return err
			}
			var dep map[string]interface{}
			if err := json.Unmarshal(resp, &dep); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Printf("✓ Added blocker %s → %s\n", dep["blocker_id"], args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&blockerID, "on", "", "", "Blocker task ID (required)")
	return cmd
}

func dependRemoveCmd() *cobra.Command {
	var blockerID string
	cmd := &cobra.Command{
		Use:   "remove <task-id> --remove <blocker-id>",
		Short: "Remove a blocker from a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if blockerID == "" {
				return fmt.Errorf("--remove <blocker-id> is required")
			}
			_, err := apiRequest("DELETE", "/api/tasks/"+args[0]+"/dependencies/"+blockerID, nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Removed blocker %s from %s\n", blockerID, args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&blockerID, "remove", "r", "", "Blocker task ID to remove")
	return cmd
}

func dependListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <task-id>",
		Short: "List blockers and dependents of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			// List blockers
			resp, err := apiRequest("GET", "/api/tasks/"+taskID+"/dependencies", nil)
			if err != nil {
				return err
			}
			var blockResult map[string][]map[string]interface{}
			if err := json.Unmarshal(resp, &blockResult); err != nil {
				return fmt.Errorf("failed to parse blockers: %w", err)
			}
			blockers := blockResult["blockers"]

			// List dependents
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
	return cmd
}

func dependCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "can-start <task-id>",
		Short: "Check if a task can be started",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/tasks/"+args[0]+"/can-start", nil)
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			canStart, _ := result["can_start"].(bool)
			if canStart {
				fmt.Printf("✓ Task %s can start\n", args[0])
			} else {
				fmt.Printf("✗ Task %s is blocked:\n", args[0])
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
	return cmd
}
