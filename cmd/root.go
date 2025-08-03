// File: cmd/root.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/errors"

	"github.com/spf13/cobra"
)

var programmaticMode bool

// checkDependencies checks for the availability and functionality of required external tools
func checkDependencies() error {
	// Check for age availability and basic functionality
	if _, err := exec.LookPath("age"); err != nil {
		return errors.NewDependencyError("age", "Please install age: https://github.com/FiloSottile/age")
	}
	
	// Test age basic functionality
	if err := testAgeCommand(); err != nil {
		return errors.NewDependencyError("age", "age command is not working properly").WithContext("test_error", err.Error())
	}

	// Check for age-plugin-yubikey availability
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		return errors.NewDependencyError("age-plugin-yubikey", "Please install age-plugin-yubikey: https://github.com/str4d/age-plugin-yubikey")
	}
	
	// Test age-plugin-yubikey basic functionality
	if err := testAgePluginYubikeyCommand(); err != nil {
		return errors.NewDependencyError("age-plugin-yubikey", "age-plugin-yubikey is not working properly").WithContext("test_error", err.Error())
	}

	return nil
}

// testAgeCommand tests if age command is working properly
func testAgeCommand() error {
	cmd := exec.Command("age", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run 'age --version': %v", err)
	}
	return nil
}

// testAgePluginYubikeyCommand tests if age-plugin-yubikey command is working properly
func testAgePluginYubikeyCommand() error {
	cmd := exec.Command("age-plugin-yubikey", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run 'age-plugin-yubikey --version': %v", err) 
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
			return errors.NewConfigLoadError("audit.log", err)
		}
		
		// Initialize error handler with audit logger
		if err := errors.InitWithAuditLogger(); err != nil {
			return err
		}
		
		if err := config.LoadConfig(); err != nil {
			return errors.NewConfigLoadError("config.json", err)
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
