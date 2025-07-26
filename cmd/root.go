// File: cmd/root.go
package cmd

import (
	"fmt"
	"log/slog"

	"vault.module/internal/audit"
	"vault.module/internal/config"

	"github.com/spf13/cobra"
)

var programmaticMode bool

var rootCmd = &cobra.Command{
	Use:   "vault.module",
	Short: "A secure CLI manager for crypto keys with YubiKey support.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help if no subcommand is provided
		cmd.Help()
		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := audit.InitLogger(); err != nil {
			return fmt.Errorf("failed to initialize audit logger: %w", err)
		}
		if err := config.LoadConfig(); err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		if cmd.Use != "vault.module" {
			audit.Logger.Info("Command executed", slog.String("command", cmd.Use))
		}
		// ... (the rest of the function remains the same)
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {}
