// File: cmd/root.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"vault.module/internal/audit"
	"vault.module/internal/config"

	"github.com/spf13/cobra"
)

var programmaticMode bool

var rootCmd = &cobra.Command{
	Use:                    "vault.module",
	Short:                  "A secure CLI manager for crypto keys with YubiKey support.",
	DisableAutoGenTag:      true,
	DisableSuggestions:     false,
	DisableFlagsInUseLine:  false,
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
	// Отключаем автоматическую генерацию completion
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	return rootCmd.Execute()
}

func init() {
	// Check if programmatic mode is enabled via environment variable
	if os.Getenv("VAULT_MODULE_PROGRAMMATIC") == "1" {
		programmaticMode = true
	}
}
