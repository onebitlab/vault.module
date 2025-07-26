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
	Use:                   "vault.module",
	Short:                 "A secure CLI manager for crypto keys with YubiKey support.",
	DisableAutoGenTag:     true,
	DisableSuggestions:    false,
	DisableFlagsInUseLine: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help if no subcommand is provided
		cmd.Help()
		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := audit.InitLogger(); err != nil {
			return fmt.Errorf("failed to initialize audit logger: %s", err.Error())
		}
		if err := config.LoadConfig(); err != nil {
			return fmt.Errorf("failed to load configuration: %s", err.Error())
		}
		if cmd.Use != "vault.module" {
			audit.Logger.Info("Command executed", slog.String("command", cmd.Use))
		}
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

	// Регистрация всех команд
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(deriveCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(vaultsCmd)

	// Регистрация подкоманд vaults
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsAddCmd)
	vaultsCmd.AddCommand(vaultsUseCmd)
	vaultsCmd.AddCommand(vaultsRemoveCmd)
}
