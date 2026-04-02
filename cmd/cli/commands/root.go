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
