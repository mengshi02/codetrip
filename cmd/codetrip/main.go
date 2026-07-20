package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	flags := newCLIFlags()
	rootCmd := newRootCmd(flags)

	// Use PersistentPreRun to set log level after flag parsing but before any sub-command runs
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		level := slog.LevelWarn
		if flags.verbose {
			level = slog.LevelInfo
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
