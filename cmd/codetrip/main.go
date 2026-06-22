package main

import (
	"log/slog"
	"os"
)

func main() {
	// Default to warn level to suppress INFO logs; use --verbose to enable
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	flags := newCLIFlags()
	rootCmd := newRootCmd(flags)

	// Add MCP server sub-command
	rootCmd.AddCommand(newMCPCmd(flags))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}