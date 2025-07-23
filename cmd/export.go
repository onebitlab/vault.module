// File: cmd/export.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"vault.module/internal/actions"
	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var exportYes bool

var exportCmd = &cobra.Command{
	Use:   "export <FILE_PATH>",
	Short: "Exports all accounts from the active vault to an unencrypted JSON file.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		filePath := args[0]

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		if len(v) == 0 {
			fmt.Printf("ℹ️  Vault '%s' is empty. Nothing to export.\n", config.Cfg.ActiveVault)
			return nil
		}

		if !exportYes {
			if !askForConfirmation("⚠️ WARNING: You are about to create an unencrypted copy of all secrets from the active vault. Are you sure?") {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		audit.Logger.Error("Executing plaintext export of an entire vault",
			slog.String("command", "export"),
			slog.String("vault", config.Cfg.ActiveVault),
			slog.String("destination_file", filePath),
		)

		jsonData, err := actions.ExportVault(v)
		if err != nil {
			return fmt.Errorf("failed to generate JSON for export: %w", err)
		}

		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write to file '%s': %w", filePath, err)
		}

		audit.Logger.Info("Plaintext export completed successfully", "destination_file", filePath)
		fmt.Printf("✅ All wallets (%d) from vault '%s' successfully exported to '%s'.\n", len(v), config.Cfg.ActiveVault, filePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportYes, "yes", false, "Skip interactive confirmation.")
}
