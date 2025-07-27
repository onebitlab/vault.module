// File: cmd/root.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"vault.module/internal/audit"
	"vault.module/internal/config"

	"github.com/spf13/cobra"
)

var programmaticMode bool

// checkDependencies checks for the availability of required external tools
func checkDependencies() error {
	// Check for age availability
	if _, err := exec.LookPath("age"); err != nil {
		return fmt.Errorf("age is not installed or not in PATH. Please install age: https://github.com/FiloSottile/age")
	}

	// Check for age-plugin-yubikey availability
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		return fmt.Errorf("age-plugin-yubikey is not installed or not in PATH. Please install age-plugin-yubikey: https://github.com/str4d/age-plugin-yubikey")
	}

	return nil
}

var rootCmd = &cobra.Command{
	Use:                   "vault.module",
	Short:                 "A secure CLI manager for crypto keys with YubiKey support.",
	DisableAutoGenTag:     true,
	DisableSuggestions:    false,
	DisableFlagsInUseLine: false,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help if no subcommand is provided
		cmd.Help()
		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Check dependencies only for commands that use them
		if cmd.Use != "vault.module" && cmd.Use != "help" {
			if err := checkDependencies(); err != nil {
				return err
			}
		}

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
	return rootCmd.Execute()
}

func init() {
	// Check if programmatic mode is enabled via environment variable
	if os.Getenv("VAULT_MODULE_PROGRAMMATIC") == "1" {
		programmaticMode = true
	}

	// Register all commands
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
	rootCmd.AddCommand(notesCmd)
	rootCmd.AddCommand(vaultsCmd)

	// Register vaults subcommands
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsAddCmd)
	vaultsCmd.AddCommand(vaultsUseCmd)
	vaultsCmd.AddCommand(vaultsDeleteCmd)
}
