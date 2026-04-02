package main

import (
	"fmt"
	"os"

	"github.com/rogerrlee/tasks-watcher/cmd/cli/commands"
)

func main() {
	rootCmd := &commands.RootCommand{}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
