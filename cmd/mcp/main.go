package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rogerrlee/tasks-watcher/cmd/mcp/client"
	"github.com/rogerrlee/tasks-watcher/cmd/mcp/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tasks-watcher-mcp error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	apiClient, err := client.New()
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}
	defer apiClient.Close()

	// If --socket flag is given, run as a multi-session Unix socket server.
	// Otherwise, run as a classic stdio single-session server.
	for _, arg := range os.Args {
		if arg == "--socket" || arg == "-s" {
			return runSocketServer(apiClient)
		}
	}

	return runStdioServer(apiClient)
}

func runStdioServer(apiClient *client.Client) error {
	srv := server.New(apiClient)
	return srv.Serve(os.Stdin, os.Stdout)
}

func runSocketServer(apiClient *client.Client) error {
	socketPath := os.Getenv("TASKS_WATCHER_MCP_SOCKET")
	if socketPath == "" {
		// Default to a path in the tasks-watcher config dir
		home, _ := os.UserHomeDir()
		socketPath = home + "/.tasks-watcher/mcp.sock"
	}

	srv := server.NewMultiServer(apiClient)
	if err := srv.Listen(socketPath); err != nil {
		return fmt.Errorf("socket server: %w", err)
	}

	fmt.Fprintf(os.Stderr, "tasks-watcher-mcp listening on %s\n", socketPath)

	// Wait for interrupt signal to gracefully shut down
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	return srv.Close()
}
