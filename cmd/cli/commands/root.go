package commands

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type RootCommand struct {
	cmd *cobra.Command
}

func (r *RootCommand) Execute() error {
	r.cmd = &cobra.Command{
		Use:   "tasks-watcher",
		Short: "Task management CLI for humans and AI agents",
		Long: `Tasks Watcher — unified task management across Claude Code, Cursor, CLI, and manual entry.

All tasks are tagged with their source (claude-code, cursor, cli, manual),
so you can see what each tool is working on at a glance.

Start the server first: tasks-watcher-server
Then run any command. Use --help for details.

Examples:
  tasks-watcher task create -t "Implement auth" -P high
  tasks-watcher task list -s in_progress
  tasks-watcher agents overview
  tasks-watcher project create -n myproject`,
		Version: "1.0.0",
	}
	r.cmd.AddCommand(
		TaskCommand(),
		ProjectCommand(),
		AgentsCommand(),
		ConfigCommand(),
	)
	return r.cmd.Execute()
}

// Shared config resolution
var (
	serverURL = "http://localhost:4242"
	apiKey    string
)

func resolveConfig() (string, string) {
	if serverURLEnv := os.Getenv("TASKS_WATCHER_SERVER_URL"); serverURLEnv != "" {
		serverURL = serverURLEnv
	}
	if apiKeyEnv := os.Getenv("TASKS_WATCHER_API_KEY"); apiKeyEnv != "" {
		apiKey = apiKeyEnv
	}
	if apiKey == "" {
		home, _ := os.UserHomeDir()
		keyPath := filepath.Join(home, ".tasks-watcher", "api.key")
		if data, err := os.ReadFile(keyPath); err == nil {
			apiKey = string(data)
		}
	}
	return serverURL, apiKey
}
