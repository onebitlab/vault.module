// File: cmd/export.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"vault.module/internal/actions"
	"vault.module/internal/audit"
	"vault.module/internal/colors"
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
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		filePath := args[0]

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %w", err),
				colors.Error,
			))
		}

		if len(v) == 0 {
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Vault '%s' is empty. Nothing to export.", config.Cfg.ActiveVault),
				colors.Info,
			))
			return nil
		}

		if !exportYes {
			if !askForConfirmation(colors.SafeColor(
				"WARNING: You are about to create an unencrypted copy of all secrets from the active vault. Are you sure?",
				colors.Warning,
			)) {
				fmt.Println(colors.SafeColor("Cancelled.", colors.Info))
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
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to generate JSON for export: %w", err),
				colors.Error,
			))
		}

		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to write to file '%s': %w", filePath, err),
				colors.Error,
			))
		}

		audit.Logger.Info("Plaintext export completed successfully", "destination_file", filePath)
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("All wallets (%d) from vault '%s' successfully exported to '%s'.", len(v), config.Cfg.ActiveVault, filePath),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportYes, "yes", false, "Skip confirmation prompt.")
}
