// File: cmd/import.go
package cmd

import (
	"fmt"
	"os"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var importFormat string
var importConflict string

var importCmd = &cobra.Command{
	Use:   "import <FILE_PATH>",
	Short: "Bulk imports accounts from a file into the active vault.",
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

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %w", err),
				colors.Error,
			))
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to read file '%s': %w", filePath, err),
				colors.Error,
			))
		}

		// Pass the vault type to the action to use the correct key manager.
		updatedVault, report, err := actions.ImportWallets(v, content, importFormat, importConflict, activeVault.Type)
		if err != nil {
			return err
		}

		if err := vault.SaveVault(activeVault, updatedVault); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %w", err),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(report, colors.Success))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVar(&importFormat, "format", constants.FormatJSON, "File format (json or key-value).")
	importCmd.Flags().StringVar(&importConflict, "on-conflict", constants.ConflictPolicySkip, "Behavior on conflict (skip, overwrite, fail).")
}
