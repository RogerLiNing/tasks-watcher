package main

import (
	"fmt"
	"os"

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

	srv := server.New(apiClient)
	return srv.Serve(os.Stdin, os.Stdout)
}
