package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func ProjectCommand() *cobra.Command {
	projCmd := &cobra.Command{
		Use:     "project",
		Short:   "Manage projects",
		Long:    "Create and list projects. Projects group tasks and provide repo context.",
		Example: `  tasks-watcher project create -n myproject -d "Backend API service"
  tasks-watcher project list`,
	}

	projCmd.AddCommand(
		projectCreateCmd(),
		projectListCmd(),
		projectDeleteCmd(),
	)

	return projCmd
}

func projectCreateCmd() *cobra.Command {
	var name, description, repoPath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			body := map[string]interface{}{"name": name}
			if description != "" {
				body["description"] = description
			}
			if repoPath != "" {
				body["repo_path"] = repoPath
			}
			resp, err := apiRequest("POST", "/api/projects", body)
			if err != nil {
				return err
			}
			var p map[string]interface{}
			json.Unmarshal(resp, &p)
			fmt.Printf("✓ Project created: %s [%s]\n", p["name"], p["id"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "Repository path")
	cmd.MarkFlagRequired("name")
	return cmd
}

func projectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/projects", nil)
			if err != nil {
				return err
			}
			var result map[string][]map[string]interface{}
			json.Unmarshal(resp, &result)
			projects := result["projects"]
			if len(projects) == 0 {
				fmt.Println("No projects found.")
				return nil
			}
			fmt.Printf("%-36s %s\n", "ID", "NAME")
			fmt.Println("────────────────────────────────────────────────────────────")
			for _, p := range projects {
				fmt.Printf("%-36s %s\n", p["id"], p["name"])
			}
			return nil
		},
	}
}

func projectDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project-id>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiRequest("DELETE", "/api/projects/"+args[0], nil)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Project deleted: %s\n", args[0][:8])
			return nil
		},
	}
}

func ConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration",
	}

	configCmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Show current configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				srv, key := resolveConfig()
				home, _ := os.UserHomeDir()
				keyPath := home + "/.tasks-watcher/api.key"
				fmt.Println("Tasks Watcher Configuration")
				fmt.Println("────────────────────────────")
				fmt.Printf("Server URL:  %s\n", srv)
				fmt.Printf("API Key:     %s\n", key)
				fmt.Printf("Key file:    %s\n", keyPath)
				if key == "" {
					fmt.Println("\n⚠ No API key found. Set TASKS_WATCHER_API_KEY or ensure ~/.tasks-watcher/api.key exists.")
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "api-key",
			Short: "Print API key",
			RunE: func(cmd *cobra.Command, args []string) error {
				_, key := resolveConfig()
				if key == "" {
					fmt.Println("No API key found.")
				} else {
					fmt.Println(key)
				}
				return nil
			},
		},
	)

	return configCmd
}
